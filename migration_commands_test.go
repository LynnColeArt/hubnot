package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hubnot/internal/migration"
)

func TestMigrationForeignInputsRequireExplicitProfile(t *testing.T) {
	for _, side := range []string{"source", "destination"} {
		c := migration.ProviderConfig{Locator: migration.Locator{Kind: "unconfigured", Endpoint: "https://unused.invalid", Repository: "owner/repo"}}
		if err := migrationCLIProfile(side, c); err == nil || !strings.Contains(err.Error(), "--"+side+"-api-version") {
			t.Fatalf("missing profile: %v", err)
		}
		c.APIVersion = "explicit"
		if err := migrationCLIProfile(side, c); err == nil || !strings.Contains(err.Error(), "--"+side+"-credential-mode") {
			t.Fatalf("missing mode: %v", err)
		}
	}
}

func buildMigrationCLIBinary(t *testing.T, tags string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "hn")
	args := []string{"build", "-o", binary}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, ".")
	cmd := exec.Command("go", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", tags, err, output)
	}
	return binary
}

func migrationCLIProcess(t *testing.T, binary, directory, input string, environment map[string]string, wantOK bool, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, append([]string{"migrate"}, args...)...)
	cmd.Dir = directory
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = operationalTestEnvironmentWith(t.TempDir(), environment)
	output, err := cmd.CombinedOutput()
	// Optional evidence records real process I/O, never credential values or HOME.
	if evidence := os.Getenv("HN_MIGRATION_TEST_TRANSCRIPTS"); evidence != "" {
		file, writeErr := os.CreateTemp(evidence, "process-*.log")
		if writeErr != nil {
			t.Fatal(writeErr)
		}
		_, writeErr = fmt.Fprintf(file, "test: %s\nargv: %q\nstdin:\n%s\nexit: %v\ncombined output:\n%s", t.Name(), append([]string{"migrate"}, args...), input, err, output)
		closeErr := file.Close()
		if writeErr != nil {
			t.Fatal(writeErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if (err == nil) != wantOK {
		t.Fatalf("hn migrate %v: err=%v wantOK=%v\n%s", args, err, wantOK, output)
	}
	return string(output)
}

func migrationCLIPreview(t *testing.T, output string) struct {
	Intents  []migration.Intent  `json:"intents"`
	Findings []migration.Finding `json:"findings"`
} {
	t.Helper()
	var v struct {
		Intents  []migration.Intent  `json:"intents"`
		Findings []migration.Finding `json:"findings"`
	}
	marker := "Preview (read only; decisions apply to these findings):\n"
	index := strings.Index(output, marker)
	if index < 0 {
		t.Fatalf("missing preview:\n%s", output)
	}
	if err := json.NewDecoder(strings.NewReader(output[index+len(marker):])).Decode(&v); err != nil {
		t.Fatalf("decode preview: %v\n%s", err, output)
	}
	return v
}

func migrationCLIPlanWithDecisions(t *testing.T, binary, directory string, environment map[string]string, args ...string) string {
	t.Helper()
	output := migrationCLIProcess(t, binary, directory, "", environment, false, append([]string{"plan"}, args...)...)
	preview := migrationCLIPreview(t, output)
	for _, f := range preview.Findings {
		if f.Status != "mapped" && f.Status != "preserved" {
			args = append(args, "--decision", f.ID+"=preserve")
		}
	}
	return migrationCLIProcess(t, binary, directory, "", environment, true, append([]string{"plan"}, args...)...)
}

func migrationCLIRepository(t *testing.T) string {
	t.Helper()
	path := t.TempDir()
	for _, args := range [][]string{{"init", "--quiet", "--initial-branch=main"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		cmd.Env = operationalTestEnvironment(t.TempDir())
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git fixture: %v %s", e, out)
		}
	}
	return path
}

func TestMigrationCLINativeOfflineBuildVariants(t *testing.T) {
	for _, tags := range []string{"", "nogithub", "noforge"} {
		t.Run("tags="+tags, func(t *testing.T) {
			binary := buildMigrationCLIBinary(t, tags)
			directory := t.TempDir()
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			gitOnly := t.TempDir()
			if err = os.Symlink(git, filepath.Join(gitOnly, "git")); err != nil {
				t.Fatal(err)
			}
			env := map[string]string{"PATH": gitOnly}
			caps := migrationCLIProcess(t, binary, directory, "", env, true, "capabilities")
			kinds := strings.Fields(caps)
			contains := func(s string) bool {
				for _, v := range kinds {
					if v == s {
						return true
					}
				}
				return false
			}
			if tags == "noforge" {
				for _, kind := range []string{"github", "gitlab", "forgejo", "gitea", "bitbucket", "azure"} {
					if contains(kind) {
						t.Fatalf("excluded provider %s advertised: %s", kind, caps)
					}
				}
			} else if tags == "nogithub" && (contains("github") || !contains("gitlab")) {
				t.Fatalf("nogithub semantics changed: %s", caps)
			}
			source := migrationCLIRepository(t)
			for _, args := range [][]string{{"init", "--name", "Source operator"}, {"issue", "open", "--body", "Exact signed native work", "--json", "Original native issue"}} {
				cmd := exec.Command(binary, args...)
				cmd.Dir = source
				cmd.Env = operationalTestEnvironmentWith(t.TempDir(), env)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("source native %v: %v %s", args, err, output)
				}
			}
			refCommand := exec.Command("git", "-C", source, "for-each-ref", "--format=%(refname)", "refs/hn/actors/")
			actorRefs, err := refCommand.Output()
			if err != nil || len(strings.Fields(string(actorRefs))) == 0 {
				t.Fatalf("native actor refs missing: %s %v", actorRefs, err)
			}
			archive := filepath.Join(directory, "project.hn")
			session := filepath.Join(directory, "export")
			exportArgs := []string{"plan", "--source-kind", "local", "--source-repository", source, "--ref", "refs/heads/main", "--destination-kind", "archive", "--destination-repository", archive, "--session", session}
			for _, ref := range strings.Fields(string(actorRefs)) {
				exportArgs = append(exportArgs, "--ref", ref)
			}
			migrationCLIProcess(t, binary, directory, "", env, true, exportArgs...)
			if _, err = os.Stat(archive); !os.IsNotExist(err) {
				t.Fatal("plan wrote archive")
			}
			migrationCLIProcess(t, binary, directory, "", env, true, "apply", "--session", session, "--confirm-apply")
			inspection := migrationCLIProcess(t, binary, directory, "", env, true, "inspect", "--archive", archive)
			if !strings.Contains(inspection, `"source_ref": "refs/heads/main"`) {
				t.Fatal("code ref missing from archive")
			}
			var exported struct {
				Snapshot migration.Snapshot `json:"snapshot"`
			}
			if err = json.Unmarshal([]byte(inspection), &exported); err != nil {
				t.Fatal(err)
			}
			for _, ref := range strings.Fields(string(actorRefs)) {
				cmd := exec.Command("git", "-C", source, "rev-parse", ref)
				oid, err := cmd.Output()
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, r := range exported.Snapshot.NativeRefs {
					if r.SourceRef == ref && r.OID == strings.TrimSpace(string(oid)) {
						found = true
					}
				}
				if !found {
					t.Fatal("exact signed actor head missing from public archive inspection")
				}
			}
			destination := migrationCLIRepository(t)
			importSession := filepath.Join(directory, "import")
			migrationCLIPlanWithDecisions(t, binary, directory, env, "--source-kind", "archive", "--source-repository", archive, "--destination-kind", "local", "--destination-repository", destination, "--session", importSession)
			migrationCLIProcess(t, binary, directory, "", env, true, "apply", "--session", importSession, "--confirm-apply")
			list := migrationCLIProcess(t, binary, directory, "", env, true, "imports", "--repository", destination)
			var imports []string
			if err = json.Unmarshal([]byte(list), &imports); err != nil || len(imports) != 1 {
				t.Fatalf("imports %s: %v", list, err)
			}
			migrationCLIProcess(t, binary, directory, "", env, true, "inspect", "--repository", destination, "--snapshot", imports[0])
			refsCmd := exec.Command("git", "-C", destination, "for-each-ref", "--format=%(objectname)", "refs/hn/")
			importedOIDs, err := refsCmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range exported.Snapshot.NativeRefs {
				if strings.HasPrefix(r.SourceRef, "refs/hn/actors/") && !strings.Contains(string(importedOIDs), r.OID) {
					t.Fatal("signed actor bytes changed during import")
				}
			}
			continued := filepath.Join(directory, "continued.hn")
			migrationCLIProcess(t, binary, directory, "", env, true, "plan", "--source-kind", "imported", "--source-repository", destination, "--snapshot", imports[0], "--source-git", destination, "--ref", "refs/heads/main", "--destination-kind", "archive", "--destination-repository", continued, "--session", filepath.Join(directory, "continue"))
			migrationCLIProcess(t, binary, directory, "", env, true, "apply", "--session", filepath.Join(directory, "continue"), "--confirm-apply")
			// Native authoring remains usable after import with only Git on PATH.
			for _, args := range [][]string{{"init", "--name", "Offline operator"}, {"issue", "open", "--body", "offline work", "--json", "Continue locally"}, {"issue", "list", "--json"}} {
				cmd := exec.Command(binary, args...)
				cmd.Dir = destination
				cmd.Env = operationalTestEnvironmentWith(t.TempDir(), env)
				out, e := cmd.CombinedOutput()
				if e != nil {
					t.Fatalf("native %v: %v %s", args, e, out)
				}
			}
			if tags == "" {
				guidedArchive := filepath.Join(directory, "guided.hn")
				guidedSession := filepath.Join(directory, "guided")
				transcript := fmt.Sprintf("local\n%s\narchive\n%s\n%s\n-\nrefs/heads/main\ndone\nAPPLY\n", source, guidedArchive, guidedSession)
				guided := migrationCLIProcess(t, binary, directory, transcript, env, true)
				for _, prompt := range []string{"source kind", "destination kind", "Resource kinds", "Full Git refs", "Mapping KEY=VALUE", "Saved plan:", "Type APPLY"} {
					if !strings.Contains(guided, prompt) {
						t.Fatalf("missing guided question %q", prompt)
					}
				}
				if _, err := os.Stat(guidedArchive); err != nil {
					t.Fatal("fully guided archive was not explicitly applied")
				}
			}
		})
	}
}

func TestMigrationCLIHostileArchiveIsReadOnlyRefusal(t *testing.T) {
	binary := buildMigrationCLIBinary(t, "noforge")
	directory := t.TempDir()
	path := filepath.Join(directory, "hostile.hn")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := tar.NewWriter(f)
	if err = w.WriteHeader(&tar.Header{Name: "../escaped", Mode: 0600, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	migrationCLIProcess(t, binary, directory, "", nil, false, "inspect", "--archive", path)
	if _, err = os.Stat(filepath.Join(directory, "escaped")); !os.IsNotExist(err) {
		t.Fatal("hostile archive escaped its boundary")
	}
}

func TestMigrationDescriptorSealsExactNonsecretProfile(t *testing.T) {
	ctx := context.Background()
	s, blobs := migrationForeignSnapshot()
	path := filepath.Join(t.TempDir(), "out.hn")
	d := ArchiveMigrationDestination{Path: path}
	sel := migration.Selection{Destination: d.Locator()}
	session, err := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), s, blobs, sel, d, migration.DefaultLimits(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := migration.ProviderConfig{Locator: d.Locator(), Token: "must-never-be-sealed"}
	if err = saveMigrationCLI(session, c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(session.Directory, "cli.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(c.Token)) {
		t.Fatal("credential persisted")
	}
	got, err := loadMigrationCLI(session)
	if err != nil || got.Locator != c.Locator {
		t.Fatalf("descriptor=%+v err=%v", got, err)
	}
	session.Plan.Destination.Repository += "changed"
	if _, err = loadMigrationCLI(session); err == nil {
		t.Fatal("descriptor allowed a changed plan")
	}
}

func TestMigrationNoninteractiveAndGuidedMissingInputAreActionable(t *testing.T) {
	var out bytes.Buffer
	err := runMigrationCLI(context.Background(), []string{"plan"}, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "--source-kind") {
		t.Fatalf("missing input error: %v", err)
	}
	err = runMigrationCLI(context.Background(), []string{"plan", "--guided"}, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("guided EOF: %v", err)
	}
}
