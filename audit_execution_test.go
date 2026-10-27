package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type auditTestBackend struct{ run func() error }

func (auditTestBackend) Name() string     { return "host" }
func (auditTestBackend) Available() error { return nil }
func (b auditTestBackend) RunStep(context.Context, string, PipelineStep, []string, io.Writer) error {
	return b.run()
}

func TestAuditExecutionRefusesTransportAndMidRunAuthorityWithdrawal(t *testing.T) {
	root, id, at := auditFixture(t)
	auditAuthorizeFixture(t, id, at)
	request, err := AuditRequest(at, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err = executeAudit(context.Background(), request.ID, "", id, auditTestBackend{run: func() error { return errors.New("transport unavailable") }}); err == nil || !strings.Contains(err.Error(), "transport") {
		t.Fatal(err)
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("transport error published a result")
	}
	_, err = executeAudit(context.Background(), request.ID, "", id, auditTestBackend{run: func() error {
		p, _, _, err := loadPolicy("HEAD")
		if err != nil {
			return err
		}
		p.Audits = nil
		writeTestPolicy(t, root, p)
		mustGit(t, "add", ".hn/policy.json")
		mustGit(t, "commit", "-qm", "withdraw during run")
		return nil
	}})
	if err == nil {
		t.Fatal("published after authority withdrawal")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("withdrawal refusal published a result")
	}
}

func TestAuditFindingOutputBoundsAndStableFallback(t *testing.T) {
	failure := executionResult{Outcome: "failed", Log: []byte("unstructured failure")}
	findings, err := auditFindings(failure)
	if err != nil || len(findings) != 1 || findings[0].Key != "rule-failure" {
		t.Fatal(findings, err)
	}
	if got, _ := auditFindings(executionResult{Outcome: "passed"}); len(got) != 0 {
		t.Fatal("pass invented findings")
	}
	for _, line := range []string{
		`{"key":"x","title":"x","unknown":true}`,
		`{"key":"x","key":"y","title":"x"}`,
		`{"key":"x","title":"x","path":"../escape"}`,
		`{"key":"x","title":"x","line":1}`,
		`{"key":"x","title":"x","body":"` + strings.Repeat("a", (16<<10)+1) + `"}`,
		`{"key":"X","title":"x"}` + "\nHN_AUDIT_FINDING " + `{"key":"x","title":"collision"}`,
	} {
		if _, err := auditFindings(executionResult{Outcome: "failed", Log: []byte("HN_AUDIT_FINDING " + line)}); err == nil {
			t.Fatalf("accepted invalid finding %s", line[:min(100, len(line))])
		}
	}
	lines := ""
	for i := 0; i < 129; i++ {
		raw, _ := json.Marshal(AuditFinding{Key: strings.Repeat("x", i+1), Title: "Finding"})
		lines += "HN_AUDIT_FINDING " + string(raw) + "\n"
	}
	if _, err := auditFindings(executionResult{Outcome: "failed", Log: []byte(lines)}); err == nil {
		t.Fatal("accepted finding overflow")
	}
}

func TestAuditPublicFailingUnstructuredPassAndInvalidOutput(t *testing.T) {
	binary := buildOperationalBinary(t)
	root, _, _ := auditFixture(t)
	auditPublic(t, binary, root, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	var previous string
	for _, scenario := range []struct {
		name, command string
		ok            bool
		want          string
	}{
		{"failure", "printf 'unstructured failure'; exit 4", true, `"key":"rule-failure"`},
		{"pass", "true", true, `"outcome":"passed"`},
		{"malformed", `printf '%s\n' 'HN_AUDIT_FINDING {"key":"x","title":"x","unknown":true}'`, false, ""},
		{"log-cap", "head -c 1100000 /dev/zero | tr '\\000' x; exit 2", true, "[log truncated by Hubnot]"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			definition := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "controlled", Command: "sh", Args: []string{"-c", scenario.command}}}}
			raw, _ := json.Marshal(definition)
			if err := os.WriteFile(filepath.Join(root, ".hn/pipelines/check.json"), raw, 0644); err != nil {
				t.Fatal(err)
			}
			mustGit(t, "add", ".hn")
			mustGit(t, "commit", "-qm", scenario.name)
			args := []string{"rule", "authorize", "check", "--source", "pipeline", "--at", "HEAD", "--json"}
			if previous != "" {
				args = append(args, "--expect", previous)
			}
			previous = auditPublicID(t, auditPublic(t, binary, root, true, args...))
			request := auditPublicID(t, auditPublic(t, binary, root, true, "request", "--at", "HEAD", "--rule", "check", "--json"))
			before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
			raw = auditPublic(t, binary, root, scenario.ok, "execute", request, "--backend", "host", "--allow-unsafe-host-execution", "--json")
			if !scenario.ok {
				if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
					t.Fatal("bad output published")
				}
				return
			}
			result := auditPublicID(t, raw)
			content := string(auditPublic(t, binary, root, true, "show", result, "--json"))
			if scenario.name == "log-cap" {
				content = string(auditPublic(t, binary, root, true, "logs", result, "--json"))
			}
			if !strings.Contains(content, scenario.want) {
				t.Fatal(content[:min(500, len(content))])
			}
		})
	}
}

// The subprocess holds the real production Git guard, allowing its parent to
// exercise process termination without installing a product testing hook.
func TestAuditRequestGuardChild(t *testing.T) {
	root := os.Getenv("HN_AUDIT_TEST_GUARD_ROOT")
	if root == "" {
		return
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	release, err := lockAuditRequestOperation(os.Getenv("HN_AUDIT_TEST_GUARD_ACTOR"), "terminated")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	fmt.Println("guard-ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
}
func TestAuditRequestGuardClientTerminationReleasesPreparedLock(t *testing.T) {
	root, id, _ := auditFixture(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestAuditRequestGuardChild$")
	cmd.Env = append(os.Environ(), "HN_AUDIT_TEST_GUARD_ROOT="+root, "HN_AUDIT_TEST_GUARD_ACTOR="+id.Actor, "TMPDIR="+t.TempDir())
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "guard-ready\n" {
		t.Fatal(line, err)
	}
	if _, err := lockAuditRequestOperation(id.Actor, "terminated"); err == nil {
		t.Fatal("guard was not exclusive")
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	deadline := time.Now().Add(5 * time.Second)
	for {
		release, err := lockAuditRequestOperation(id.Actor, "terminated")
		if err == nil {
			release()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client termination retained prepared lock", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := mustGitText(t, "for-each-ref", "--format=%(refname)", "refs/hn/local"); got != "" {
		t.Fatal("private ref committed", got)
	}
}
