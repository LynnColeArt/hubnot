package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Qualification instrumentation only; neither strace nor these provisional
// registry paths are a production dependency or a frozen host layout. WP24 must
// bind the detector to the actual registration and enablement paths from WP02.
var extensionBaselineTrace = flag.String("hn-baseline-trace", "", "explicit strace executable for local forbidden-effect qualification")
var extensionBaselineRegistry = flag.String("hn-baseline-registry", ".git/hn/extensions/registry.json", "repository-relative registry trap path (qualification only)")

// A detector reports its cause so a deliberately violating control cannot earn
// credit merely by failing for an unrelated reason.
func extensionBaselineViolation(trace, registry, marker string, before, after map[string]string) string {
	if strings.Contains(trace, `openat(`) {
		for _, line := range strings.Split(trace, "\n") {
			if strings.Contains(line, "openat(") && strings.Contains(line, registry) {
				return "registry-read"
			}
		}
	}
	for _, line := range strings.Split(trace, "\n") {
		if strings.Contains(line, "connect(") {
			return "provider-connect"
		}
		if strings.Contains(line, "execve(") && strings.Contains(line, marker) {
			return "module-start"
		}
	}
	if !reflect.DeepEqual(before, after) {
		return "extension-state-write"
	}
	return ""
}

// The child is a real external process used only by explicit test controls.
func TestExtensionBaselineControlProcess(t *testing.T) {
	mode := os.Getenv("HN_BASELINE_CONTROL")
	if mode == "" {
		return
	}
	var err error
	switch mode {
	case "registry-read":
		_, err = os.ReadFile(os.Getenv("HN_BASELINE_REGISTRY"))
	case "module-start":
		child := exec.Command(os.Getenv("HN_BASELINE_MARKER"), "-test.run=^TestExtensionBaselineControlProcess$")
		child.Env = append(os.Environ(), "HN_BASELINE_CONTROL=extension-state-write")
		err = child.Run()
	case "extension-state-write":
		err = os.WriteFile(os.Getenv("HN_BASELINE_STATE"), []byte("deliberate forbidden effect"), 0600)
	case "provider-connect":
		var conn net.Conn
		conn, err = net.DialTimeout("tcp", os.Getenv("HN_BASELINE_PROVIDER"), time.Second)
		if err == nil {
			err = conn.Close()
		}
	default:
		err = fmt.Errorf("unknown control %s", mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func extensionBaselineTraceRun(t *testing.T, env []string, repo, program string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	trace := filepath.Join(t.TempDir(), "effects.trace")
	command, argv := program, args
	if *extensionBaselineTrace != "" {
		command = *extensionBaselineTrace
		argv = append([]string{"-f", "-qq", "-s", "4096", "-e", "trace=openat,execve,connect", "-o", trace, program}, args...)
	}
	cmd := exec.CommandContext(ctx, command, argv...)
	cmd.Dir, cmd.Env = repo, env
	out, err := cmd.CombinedOutput()
	if err != nil || ctx.Err() != nil {
		t.Fatalf("public command %q: %v, deadline=%v\n%s", args, err, ctx.Err(), out)
	}
	if *extensionBaselineTrace == "" {
		return ""
	}
	raw, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestExtensionDefaultBaselineForbiddenEffects(t *testing.T) {
	for _, variant := range []struct{ name, tags string }{{"standard", ""}, {"native-only", "nogithub,noforge"}} {
		t.Run(variant.name, func(t *testing.T) {
			f := newBetaContributionCLI(t, buildMigrationCLIBinary(t, variant.tags))
			repo := f.repository()
			f.hn(repo, true, "init", "--name", "Native baseline")
			issue := betaRecoveryID(t, f.hn(repo, true, "issue", "open", "--operation", "baseline-issue", "--json", "Native baseline issue"))
			registry := filepath.Join(repo, *extensionBaselineRegistry)
			betaContributionWrite(t, registry, []byte("{ deliberately malformed inert registry\n"), 0600)
			stateDir := filepath.Dir(registry)
			marker := filepath.Join(t.TempDir(), "forbidden-module")
			testProgram, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(testProgram, marker); err != nil {
				t.Fatal(err)
			}
			provider, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer provider.Close()
			state := filepath.Join(stateDir, "forbidden-marker")
			env := append(append([]string{}, f.env...), "HN_BASELINE_REGISTRY="+registry, "HN_BASELINE_MARKER="+marker, "HN_BASELINE_STATE="+state, "HN_BASELINE_PROVIDER="+provider.Addr().String())
			before := policyPublicSnapshot(t, stateDir)
			for _, args := range [][]string{
				{"issue", "show", issue, "--json"}, {"issue", "list", "--json"},
				{"project", "list", "--json"}, {"discussion", "list", "--json"}, {"workflow", "list", "--json"},
				{"memory", "recall", "--at", "HEAD", "--include-untrusted", "--json"},
			} {
				trace := extensionBaselineTraceRun(t, env, repo, f.binary, args...)
				if cause := extensionBaselineViolation(trace, registry, marker, before, policyPublicSnapshot(t, stateDir)); cause != "" {
					t.Fatalf("native %q: forbidden effect %s", args, cause)
				}
			}
			for _, cause := range []string{"registry-read", "module-start", "provider-connect", "extension-state-write"} {
				if *extensionBaselineTrace == "" && cause != "extension-state-write" {
					continue
				}
				_ = os.Remove(state)
				initial := policyPublicSnapshot(t, stateDir)
				controlEnv := append(append([]string{}, env...), "HN_BASELINE_CONTROL="+cause)
				trace := extensionBaselineTraceRun(t, controlEnv, repo, testProgram, "-test.run=^TestExtensionBaselineControlProcess$")
				if got := extensionBaselineViolation(trace, registry, marker, initial, policyPublicSnapshot(t, stateDir)); got != cause {
					t.Fatalf("deliberate %s detector = %q", cause, got)
				}
				t.Logf("detector rejected deliberate %s for exact cause", cause)
			}
			if *extensionBaselineTrace == "" {
				t.Log("registry/start/connect tracing not requested; explicit qualification invocation required")
			}
		})
	}
}

// This extends the released-writer reader check to the combined native tags.
// Existing beta tests additionally exercise malformed signed historical inputs,
// continuation, import/export and recovery; those assertions remain unchanged.
func TestExtensionDefaultBaselineHistorical(t *testing.T) {
	for _, variant := range []struct{ name, tags string }{{"standard", ""}, {"native-only", "nogithub,noforge"}} {
		t.Run(variant.name, func(t *testing.T) {
			f := newBetaContributionCLI(t, buildMigrationCLIBinary(t, variant.tags))
			for _, version := range []string{"0.0.1-dev", "0.0.2-dev", "0.0.3-dev"} {
				t.Run(version, func(t *testing.T) {
					dir, err := filepath.Abs(filepath.Join("testdata", "compatibility", version))
					if err != nil {
						t.Fatal(err)
					}
					var corpus betaHistoricalCorpus
					var expected map[string]any
					betaReadJSON(t, filepath.Join(dir, "provenance.json"), &corpus)
					betaReadJSON(t, filepath.Join(dir, "observations.json"), &expected)
					parent := t.TempDir()
					repo := filepath.Join(parent, "reader")
					f.git(parent, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "clone", "-q", filepath.Join(dir, "public.bundle"), repo)
					f.git(repo, "fetch", "--no-write-fetch-head", filepath.Join(dir, "public.bundle"), "+refs/hn/*:refs/hn/*")
					before := betaHistoricalObjects(t, repo, f.env, corpus.Refs)
					for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow", "memory"} {
						var got any
						if err := json.Unmarshal([]byte(f.hn(repo, true, family, "show", corpus.IDs[family], "--json")), &got); err != nil {
							t.Fatal(err)
						}
						betaSemanticSubset(t, "", expected[family], got)
					}
					betaAssertHistoricalRefs(t, repo, f.env, corpus.Refs)
					if !bytes.Equal(before, betaHistoricalObjects(t, repo, f.env, corpus.Refs)) {
						t.Fatal("public reads changed historical bytes/signatures/IDs")
					}
					f.hn(repo, false, "identity", "show", "--json")
					for _, name := range []string{"identity.json", "identities", "active", "rotation.json"} {
						if _, err := os.Stat(filepath.Join(repo, ".git", "hn", name)); !os.IsNotExist(err) {
							t.Fatalf("keyless reads created %s: %v", name, err)
						}
					}
				})
			}
		})
	}
}
