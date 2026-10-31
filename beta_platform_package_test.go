package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"hubnot/internal/migration"
	"hubnot/internal/release"
)

var betaPackageBundle = flag.String("hn-package-bundle", "", "immutable candidate asset directory; independently verify all archives")

func betaFileDigest(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// This verifies transported bytes independently of the packager, before using
// either executable. The manifest is provenance, never authority to publish.
func TestBetaPlatformCandidateBundle(t *testing.T) {
	if *betaPackageBundle == "" {
		if *portablePackageDirectory != "" {
			t.Fatal("candidate package qualification requires -hn-package-bundle")
		}
		t.Log("candidate bundle verification requires explicit package inputs; source suite is separate evidence")
		return
	}
	var manifest release.Provenance
	betaReadJSON(t, filepath.Join(*betaPackageBundle, "PROVENANCE.json"), &manifest)
	head := strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD^{tree}"))
	if manifest.Schema != "hn.release/1" || manifest.SourceCommit != head || manifest.SourceTree != tree || manifest.Version != *portablePackageVersion || manifest.Compiler != *portablePackageCompiler || len(manifest.Assets) != 9 {
		t.Fatalf("candidate manifest does not bind the requested source/tree/version/compiler/full matrix: %+v", manifest)
	}
	raw, err := os.ReadFile(filepath.Join(*betaPackageBundle, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	checksums := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || filepath.Base(fields[1]) != fields[1] || checksums[fields[1]] != "" {
			t.Fatal("invalid/duplicate checksum entry", line)
		}
		checksums[fields[1]] = fields[0]
		if got := betaFileDigest(t, filepath.Join(*betaPackageBundle, fields[1])); got != fields[0] {
			t.Fatal("transported checksum mismatch", fields[1], got)
		}
	}
	if len(checksums) != 10 || checksums["PROVENANCE.json"] == "" {
		t.Fatal("checksums must cover nine archives and provenance")
	}
	seen := map[string]bool{}
	for _, asset := range manifest.Assets {
		if seen[asset.Name] || checksums[asset.Name] != asset.SHA256 {
			t.Fatal("archive manifest/checksum disagreement", asset.Name)
		}
		seen[asset.Name] = true
		archive := filepath.Join(*betaPackageBundle, asset.Name)
		file, err := os.Open(archive)
		if err != nil {
			t.Fatal(err)
		}
		gz, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		reader := tar.NewReader(gz)
		var build *release.BuildProvenance
		executable := ""
		members := map[string]bool{}
		root := strings.TrimSuffix(asset.Name, ".tar.gz") + "/"
		for {
			h, e := reader.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			if !strings.HasPrefix(h.Name, root) || strings.Contains(h.Name, "\\") || strings.Contains(h.Name, "../") || members[h.Name] || h.Typeflag != tar.TypeReg {
				t.Fatal("unsafe/duplicate/nonregular archive member", h.Name)
			}
			members[h.Name] = true
			data, e := io.ReadAll(reader)
			if e != nil {
				t.Fatal(e)
			}
			switch strings.TrimPrefix(h.Name, root) {
			case "hn":
				executable = fmt.Sprintf("%x", sha256.Sum256(data))
				if h.Mode != 0755 {
					t.Fatal("application is not executable")
				}
			case "BUILD.json":
				build = new(release.BuildProvenance)
				if e = json.Unmarshal(data, build); e != nil {
					t.Fatal(e)
				}
			}
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if asset.Build == nil {
			if asset.Name != "hubnot_"+manifest.Version+"_source.tar.gz" || build != nil || executable != "" || !members[root+"go.mod"] {
				t.Fatal("invalid source archive")
			}
		} else {
			if !reflect.DeepEqual(build, asset.Build) || build.SourceCommit != head || build.SourceTree != tree || build.Compiler != manifest.Compiler || build.Version != manifest.Version || build.ExecutableSHA256 != executable {
				t.Fatal("archive BUILD/executable provenance disagreement", asset.Name)
			}
			suffix := ""
			if build.Variant == "native-only" {
				suffix = "_native-only"
			} else if build.Variant != "standard" {
				t.Fatal("unknown variant")
			}
			expected := "hubnot_" + manifest.Version + "_" + strings.ReplaceAll(build.Target, "/", "_") + suffix + ".tar.gz"
			if expected != asset.Name {
				t.Fatal("archive name and target disagree")
			}
			switch build.Target {
			case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64":
			default:
				t.Fatal("unknown target")
			}
		}
		t.Logf("transported archive=%s sha256=%s build=%+v", asset.Name, asset.SHA256, build)
	}
	for _, tags := range []string{"", "noforge"} {
		portableJourneyBinary(t, tags)
	}
}

// A malformed explicit selection must refuse before a source build, even when
// the normal builder is the caller. A trap Go executable detects fallback.
func TestBetaPlatformInvalidPackageNeverBuilds(t *testing.T) {
	if os.Getenv("HN_BETA_PACKAGE_SELECTION_PROBE") == "1" {
		buildMigrationCLIBinary(t, "")
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	application, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD^{tree}"))
	for _, mode := range []string{"missing", "malformed", "source", "tree", "compiler", "target", "variant", "version", "digest"} {
		t.Run(mode, func(t *testing.T) {
			root, path := t.TempDir(), t.TempDir()
			marker := filepath.Join(root, "unexpected-source-build")
			if err := os.Symlink(git, filepath.Join(path, "git")); err != nil {
				t.Fatal(err)
			}
			trap := "#!/bin/sh\nprintf called > '" + marker + "'\nexit 99\n"
			if err := os.WriteFile(filepath.Join(path, "go"), []byte(trap), 0755); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "hubnot_invalid_"+runtime.GOOS+"_"+runtime.GOARCH)
			if mode != "missing" {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "hn"), application, 0755); err != nil {
					t.Fatal(err)
				}
				build := release.BuildProvenance{Schema: "hn.release-build/1", SourceCommit: head, SourceTree: tree, Compiler: runtime.Version(), Target: runtime.GOOS + "/" + runtime.GOARCH, Variant: "standard", Version: "invalid", ExecutableSHA256: fmt.Sprintf("%x", sha256.Sum256(application))}
				switch mode {
				case "source":
					build.SourceCommit = strings.Repeat("0", 40)
				case "tree":
					build.SourceTree = strings.Repeat("0", 40)
				case "compiler":
					build.Compiler = "go0.0.0"
				case "target":
					build.Target = "invalid/target"
				case "variant":
					build.Variant = "native-only"
				case "version":
					build.Version = "wrong"
				case "digest":
					build.ExecutableSHA256 = strings.Repeat("0", 64)
				}
				raw, err := json.Marshal(build)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "malformed" {
					raw = []byte("{")
				}
				if err := os.WriteFile(filepath.Join(dir, "BUILD.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(self, "-test.run=^TestBetaPlatformInvalidPackageNeverBuilds$", "-test.v", "-hn-package-dir", root, "-hn-package-version", "invalid")
			cmd.Env = append(operationalTestEnvironment(t.TempDir()), "PATH="+path, "HN_BETA_PACKAGE_SELECTION_PROBE=1")
			output, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte("--- FAIL:")) {
				t.Fatalf("invalid explicit package did not fail: %v\n%s", err, output)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("invalid explicit package attempted a source build", err)
			}
		})
	}
}

// Public install, memory-regex and migration paths use only the selected
// package. Child PATH contains hn and Git, with no compiler/provider tools.
func TestBetaPlatformPackageInstallAndRuntime(t *testing.T) {
	if *portablePackageDirectory == "" {
		t.Log("exact package journeys require explicit package inputs")
		return
	}
	for _, tags := range []string{"", "noforge"} {
		t.Run("tags="+tags, func(t *testing.T) {
			binary := portableJourneyBinary(t, tags)
			raw, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			f := newBetaContributionCLI(t, binary)
			install := t.TempDir()
			installed := filepath.Join(install, "hn")
			if err = os.WriteFile(installed, raw, 0755); err != nil {
				t.Fatal(err)
			}
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(git, filepath.Join(install, "git")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", install)
			selected, err := exec.LookPath("hn")
			if err != nil || selected != installed {
				t.Fatalf("PATH install: %s %v", selected, err)
			}
			f.binary = selected
			f.env[0] = "PATH=" + install
			if betaFileDigest(t, installed) != fmt.Sprintf("%x", sha256.Sum256(raw)) {
				t.Fatal("installed bytes changed")
			}
			betaContributionContains(t, f.hn(t.TempDir(), true, "version"), *portablePackageVersion)
			repo := f.repository()
			f.hn(repo, true, "init", "--name", "Package memory operator")
			identity := operationalLabel(t, f.hn(repo, true, "identity", "public"), "Actor:")
			writeOperationalMemoryPolicy(t, repo, identity, []string{identity})
			f.git(repo, "add", ".hn/policy.json")
			f.git(repo, "-c", "user.name=Package fixture", "-c", "user.email=package@hn.invalid", "commit", "-qm", "Explicit memory policy")
			f.hn(repo, true, "memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "exact", "--content", "Package Needle-42 世界", "--json")
			output := f.hn(repo, true, "memory", "grep", "--json", "Needle-4[23]")
			var result MemoryRecallEnvelopeV0
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			if result.Returned != 1 || result.Memories[0].Data.Content != "Package Needle-42 世界" || result.Warning != memoryRecallWarning {
				t.Fatal("actual package memory grep lost match/content/warning", output)
			}
			if failure := f.hn(repo, false, "memory", "grep", "--json", "["); !strings.Contains(failure, "invalid grep pattern") {
				t.Fatal("invalid regex did not reach pattern validation", failure)
			}
			// Preserve literal aliases; do not physicalize paths or inject TMPDIR.
			parents := []string{t.TempDir()}
			if runtime.GOOS == "darwin" {
				parents = []string{"/tmp", "/var/tmp"}
			}
			for _, parent := range parents {
				t.Run(parent, func(t *testing.T) {
					fixture := *f
					fixture.t = t
					f := &fixture
					root, err := os.MkdirTemp(parent, "hn-beta-package-")
					if err != nil {
						t.Fatal(err)
					}
					defer os.RemoveAll(root)
					snapshot, blobs := migrationForeignSnapshot()
					archive := filepath.Join(root, "source.hn")
					file, err := os.Create(archive)
					if err != nil {
						t.Fatal(err)
					}
					err = migration.WriteArchive(context.Background(), file, snapshot, blobs, migration.DefaultLimits())
					closeErr := file.Close()
					if err != nil || closeErr != nil {
						t.Fatal(err, closeErr)
					}
					local := filepath.Join(root, "local")
					f.git(root, "init", "--quiet", "--initial-branch=main", local)
					f.git(local, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "fixture")
					// Keep migration children in the same fully closed environment.
					call := func(ok bool, args ...string) string { return f.hn(root, ok, append([]string{"migrate"}, args...)...) }
					plan := func(args ...string) {
						output := call(false, append([]string{"plan"}, args...)...)
						for _, finding := range migrationCLIPreview(t, output).Findings {
							if finding.Status != "mapped" && finding.Status != "preserved" {
								args = append(args, "--decision", finding.ID+"=preserve")
							}
						}
						call(true, append([]string{"plan"}, args...)...)
					}
					session := filepath.Join(root, "session")
					plan("--source-kind", "archive", "--source-repository", archive, "--destination-kind", "local", "--destination-repository", local, "--session", session)
					call(true, "apply", "--session", session, "--confirm-apply")
					call(true, "inspect", "--session", session)
					imports := call(true, "imports", "--repository", local)
					var digests []string
					if err = json.Unmarshal([]byte(imports), &digests); err != nil || len(digests) != 1 {
						t.Fatal("missing imported snapshot", imports, err)
					}
					exported := filepath.Join(root, "continued.hn")
					exportPlan := filepath.Join(root, "export")
					call(true, "plan", "--source-kind", "imported", "--source-repository", local, "--snapshot", digests[0], "--destination-kind", "archive", "--destination-repository", exported, "--session", exportPlan)
					call(true, "apply", "--session", exportPlan, "--confirm-apply")
					view := call(true, "inspect", "--archive", exported)
					if !strings.Contains(view, "Foreign issue") {
						t.Fatal("package alias/temp round trip lost foreign record", view)
					}
					t.Logf("actual package migration literal parent=%s; child TMPDIR absent; binary sha256=%s", parent, betaFileDigest(t, installed))
				})
			}
		})
	}
}
