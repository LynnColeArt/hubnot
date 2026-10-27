package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"hubnot/internal/migration"
)

// One compiled public journey combines native governance, independent audits,
// preserved foreign bytes, saved plans, archives and ordinary Git replication.
// Provider API recovery has its own public fixture journey in migration_acceptance_test.go.
func TestPortableCollaborationIntegratedPublicVariants(t *testing.T) {
	for _, tags := range []string{"", "nogithub", "noforge"} {
		t.Run("tags="+tags, func(t *testing.T) {
			binary := buildMigrationCLIBinary(t, tags)
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			path := t.TempDir()
			if err = os.Symlink(git, filepath.Join(path, "git")); err != nil {
				t.Fatal(err)
			}
			// A deliberately closed process environment: no ambient tokens, Go, Python,
			// provider CLIs, user Git configuration, credential helpers or services.
			env := []string{"PATH=" + path, "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="}
			call := func(repo string, ok bool, args ...string) string {
				return portablePublic(t, binary, repo, env, ok, args...)
			}
			audit := func(repo string, ok bool, args ...string) json.RawMessage {
				out := call(repo, ok, append([]string{"audit"}, append(args, "--json")...)...)
				var v struct {
					Schema string
					OK     bool
					Data   json.RawMessage
				}
				if err := json.NewDecoder(strings.NewReader(out)).Decode(&v); err != nil || v.Schema != "hn.audit/1" || v.OK != ok {
					t.Fatalf("audit envelope: %s %v", out, err)
				}
				return v.Data
			}
			source := migrationCLIRepository(t)
			configureOperationalGit(t, source, "Portable owner")
			call(source, true, "init", "--name", "Portable owner")
			actor := operationalLabel(t, call(source, true, "identity", "public"), "Actor:")
			actorRef := "refs/hn/actors/" + actor
			before := portableRefs(t, source)
			audit(source, false, "request", "--at", "HEAD", "--rule", "observe")
			if before != portableRefs(t, source) {
				t.Fatal("disabled audit changed accepted facts")
			}
			if _, err := os.Stat(filepath.Join(source, ".hn/policy.json")); !os.IsNotExist(err) {
				t.Fatal("init installed policy")
			}
			call(source, true, "issue", "open", "--body", "Native work survives", "Original signed work")
			policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{"check": {RequiredResults: 1, TrustedRunners: []string{actor}}}, Audits: &AuditPolicy{Scope: "portable", RuleOwners: []string{actor}, Requesters: []string{actor}, Executors: []string{actor}}}
			raw, _ := json.Marshal(policy)
			policyPublicFile(t, source, ".hn/policy.json", raw)
			policyPublicFile(t, source, ".hn/pipelines/check.json", []byte(`{"version":"hn.pipeline/0","steps":[{"name":"check","command":"git","args":["--version"]}]}`))
			// Git itself supplies an explicit test command; its alias uses Git's own
			// shell support and shell builtins, with no other executable on PATH.
			definition := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "observe", Command: "git", Args: []string{"-c", `alias.observe=!printf '%s\n' 'HN_AUDIT_FINDING {"key":"Missing-Test","title":"Add coverage","body":"Observed on old code"}'; exit 1`, "observe"}}}}
			raw, _ = json.Marshal(definition)
			policyPublicFile(t, source, ".hn/pipelines/observe.json", raw)
			runOperationalGit(t, source, "add", ".hn")
			runOperationalGit(t, source, "commit", "-qm", "Explicit native and independent audit policy")
			base := runOperationalGit(t, source, "rev-parse", "HEAD")
			runOperationalGit(t, source, "switch", "-qc", "candidate")
			policyPublicFile(t, source, "work.txt", []byte("candidate code\n"))
			runOperationalGit(t, source, "add", "work.txt")
			runOperationalGit(t, source, "commit", "-qm", "Candidate")
			head := runOperationalGit(t, source, "rev-parse", "HEAD")
			call(source, true, "proposal", "open", "--base", base, "--head", head, "Native proposal")
			proposal := latestOperationalEvent(t, source, actorRef).ID
			runOperationalGit(t, source, "switch", "-q", "main")
			status := call(source, true, "proposal", "status", proposal)
			audit(source, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
			audit(source, true, "rule", "authorize", "observe", "--source", "pipeline", "--at", base)
			request := auditPublicID(t, audit(source, true, "request", "--at", base, "--rule", "observe", "--operation", "old-code"))
			before = portableRefs(t, source)
			audit(source, false, "execute", request, "--backend", "host")
			if before != portableRefs(t, source) {
				t.Fatal("unsafe-host refusal published a result")
			}
			result := auditPublicID(t, audit(source, true, "execute", request, "--backend", "host", "--allow-unsafe-host-execution"))
			shown := string(audit(source, true, "show", result))
			if !strings.Contains(shown, `"outcome":"failed"`) || !strings.Contains(shown, base) || !strings.Contains(shown, `"key":"missing-test"`) {
				t.Fatal(shown)
			}
			if status != call(source, true, "proposal", "status", proposal) {
				t.Fatal("audit altered required-check readiness")
			}
			before = portableRefs(t, source)
			call(source, false, "decide", proposal, "--accept")
			call(source, false, "merge", proposal)
			call(source, false, "run", "show", result)
			if before != portableRefs(t, source) || runOperationalGit(t, source, "rev-parse", "HEAD") != base {
				t.Fatal("audit supplied required-check/merge authority")
			}
			// Real native CI remains required and sufficient despite the failed audit.
			call(source, true, "run", "request", proposal, "check")
			run := latestOperationalEvent(t, source, actorRef).ID
			call(source, true, "run", "execute", run, "--backend", "host", "--allow-unsafe-host-execution")
			call(source, true, "decide", proposal, "--accept")
			call(source, true, "merge", proposal)
			if runOperationalGit(t, source, "merge-base", "--is-ancestor", head, "HEAD") != "" {
				t.Fatal("native merge failed")
			}
			// Auditing already merged/old code needs no proposal, and does not open work.
			oldIssues := portableData(t, call(source, true, "issue", "list", "--json"))
			next := auditPublicID(t, audit(source, true, "execute", request, "--backend", "host", "--allow-unsafe-host-execution"))
			if !bytes.Equal(oldIssues, portableData(t, call(source, true, "issue", "list", "--json"))) {
				t.Fatal("execute implicitly opened follow-up work")
			}
			first := audit(source, true, "follow-up", next, "--operation", "triage")
			again := audit(source, true, "follow-up", next, "--operation", "triage")

			var links struct {
				Associations []struct {
					IssueID string `json:"issue_id"`
				}
			}
			if err := json.Unmarshal(first, &links); err != nil || len(links.Associations) != 1 {
				t.Fatalf("follow-up %s %v", first, err)
			}
			followup := links.Associations[0].IssueID
			if !strings.Contains(string(again), followup) {
				t.Fatal("follow-up retry changed association")
			}
			call(source, true, "gh", "issue", "edit", followup, "--body", "Operator triage is preserved")
			before = portableRefs(t, source)
			audit(source, true, "follow-up", result)
			if before != portableRefs(t, source) || !strings.Contains(call(source, true, "issue", "show", followup, "--json"), "Operator triage is preserved") {
				t.Fatal("stable finding dedup rewrote work")
			}

			directory := t.TempDir()
			foreign, blobs := migrationForeignSnapshot()
			foreignFile := filepath.Join(directory, "foreign.hn")
			file, err := os.Create(foreignFile)
			if err != nil {
				t.Fatal(err)
			}
			err = migration.WriteArchive(context.Background(), file, foreign, blobs, migration.DefaultLimits())
			closeErr := file.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			foreignPlan := filepath.Join(directory, "foreign-import")
			call(directory, true, "migrate", "plan", "--source-kind", "archive", "--source-repository", foreignFile, "--destination-kind", "local", "--destination-repository", source, "--session", foreignPlan, "--decision", "local/governance=preserve")
			call(directory, true, "migrate", "apply", "--session", foreignPlan, "--confirm-apply")
			var foreignImports []string
			if err := json.Unmarshal([]byte(call(directory, true, "migrate", "imports", "--repository", source)), &foreignImports); err != nil || len(foreignImports) != 1 {
				t.Fatal(foreignImports, err)
			}
			archive := filepath.Join(directory, "combined.hn")
			session := filepath.Join(directory, "export")
			call(directory, true, "migrate", "plan", "--source-kind", "imported", "--source-repository", source, "--snapshot", foreignImports[0], "--source-git", source, "--ref", "refs/heads/main", "--ref", actorRef, "--ref", "refs/hn/proposals/"+strings.TrimPrefix(proposal, "sha256:"), "--destination-kind", "archive", "--destination-repository", archive, "--session", session)
			if _, err := os.Stat(archive); !os.IsNotExist(err) {
				t.Fatal("planning published archive")
			}
			sourceOffline := source + "-offline"
			if err := os.Rename(source, sourceOffline); err != nil {
				t.Fatal(err)
			}
			call(directory, false, "migrate", "apply", "--session", session)
			if _, err := os.Stat(archive); !os.IsNotExist(err) {
				t.Fatal("unconfirmed apply published archive")
			}
			call(directory, true, "migrate", "apply", "--session", session, "--confirm-apply")
			for digest, expected := range blobs {
				if got := call(directory, true, "migrate", "inspect", "--archive", archive, "--blob", digest); got != string(expected) {
					t.Fatal("foreign byte preservation mismatch")
				}
			}
			destination := migrationCLIRepository(t)
			policyPublicFile(t, destination, ".hn/policy.json", []byte("destination policy sentinel\n"))
			sentinel, _ := os.ReadFile(filepath.Join(destination, ".hn/policy.json"))
			before = portableRefs(t, destination)
			plan := filepath.Join(directory, "import")
			args := []string{"migrate", "plan", "--source-kind", "archive", "--source-repository", archive, "--destination-kind", "local", "--destination-repository", destination, "--session", plan}
			call(directory, false, args...)
			if before != portableRefs(t, destination) {
				t.Fatal("missing governance choice changed accepted refs")
			}
			call(directory, true, append(args, "--decision", "local/governance=preserve")...)
			call(directory, true, "migrate", "apply", "--session", plan, "--confirm-apply")
			gotPolicy, _ := os.ReadFile(filepath.Join(destination, ".hn/policy.json"))
			if !bytes.Equal(sentinel, gotPolicy) {
				t.Fatal("import replaced destination policy")
			}
			// Exact event payload, signature and attachment objects survive admission.
			commits := strings.Fields(runOperationalGit(t, sourceOffline, "rev-list", actorRef))
			for _, commit := range commits {
				for _, name := range strings.Fields(runOperationalGit(t, sourceOffline, "ls-tree", "--name-only", commit)) {
					a := portableGitBytes(t, sourceOffline, "show", commit+":"+name)
					b := portableGitBytes(t, destination, "show", commit+":"+name)
					if !bytes.Equal(a, b) {
						t.Fatalf("signed bytes changed: %s:%s", commit, name)
					}
				}
			}
			audit(destination, true, "show", next)
			audit(destination, true, "logs", next)
			before = portableRefs(t, destination)
			audit(destination, false, "authority", "show")
			audit(destination, false, "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
			call(destination, false, "identity", "public")
			if before != portableRefs(t, destination) {
				t.Fatal("keyless imported history gained authority")
			}
			imports := call(directory, true, "migrate", "imports", "--repository", destination)
			var digests []string
			if err := json.Unmarshal([]byte(imports), &digests); err != nil || len(digests) != 1 {
				t.Fatal(imports, err)
			}
			inspection := call(directory, true, "migrate", "inspect", "--repository", destination, "--snapshot", digests[0])
			if !strings.Contains(inspection, "source commenter") || !strings.Contains(inspection, "Source-only conversation") {
				t.Fatal("attributed foreign records lost")
			}
			for digest, expected := range blobs {
				if got := call(directory, true, "migrate", "inspect", "--repository", destination, "--snapshot", digests[0], "--blob", digest); got != string(expected) {
					t.Fatal("offline foreign bytes changed")
				}
			}
			// Hostile public import refuses before any accepted visibility or file effect.
			hostile := filepath.Join(directory, "hostile.hn")
			var bad bytes.Buffer
			tw := tar.NewWriter(&bad)
			if err := tw.WriteHeader(&tar.Header{Name: "../escaped", Mode: 0600, Size: 1}); err != nil {
				t.Fatal(err)
			}
			_, _ = tw.Write([]byte("x"))
			_ = tw.Close()
			policyPublicFile(t, directory, "hostile.hn", bad.Bytes())
			snapshot := policyPublicSnapshot(t, destination)
			call(directory, false, "migrate", "plan", "--source-kind", "archive", "--source-repository", hostile, "--destination-kind", "local", "--destination-repository", destination, "--session", filepath.Join(directory, "rejected"), "--decision", "local/governance=preserve")
			if !reflect.DeepEqual(snapshot, policyPublicSnapshot(t, destination)) {
				t.Fatal("hostile archive changed destination")
			}
			if _, err := os.Stat(filepath.Join(directory, "escaped")); !os.IsNotExist(err) {
				t.Fatal("archive traversal escaped")
			}
			// Continue using a deliberately new local signer; historical actors stay observations.
			call(destination, true, "init", "--name", "Local continuation")
			audit(destination, false, "authority", "show")
			own := operationalLabel(t, call(destination, true, "identity", "public"), "Actor:")
			if own == actor {
				t.Fatal("import installed source signer")
			}
			call(destination, true, "issue", "open", "Continue independently")
			if runOperationalGit(t, destination, "for-each-ref", "--format=%(refname)", "refs/hn/actors/") != "refs/hn/actors/"+own {
				t.Fatal("source history became signing-owned")
			}
			// Installing policy and adopting an anchor are separate deliberate local actions.
			configureOperationalGit(t, destination, "Local continuation")
			policy.Maintainers = []string{own}
			policy.Audits = &AuditPolicy{Scope: "local-only", RuleOwners: []string{own}, Requesters: []string{own}, Executors: []string{own}}
			policy.Pipelines = map[string]PipelinePolicy{}
			localPolicy, _ := json.Marshal(policy)
			policyPublicFile(t, destination, ".hn/policy.json", localPolicy)
			policyPublicFile(t, destination, ".hn/pipelines/observe.json", raw)
			runOperationalGit(t, destination, "add", ".hn")
			runOperationalGit(t, destination, "commit", "-qm", "Explicit local audit policy")
			audit(destination, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
			audit(destination, true, "rule", "authorize", "observe", "--source", "pipeline", "--at", "HEAD")
			localRequest := auditPublicID(t, audit(destination, true, "request", "--at", "HEAD", "--rule", "observe"))
			audit(destination, true, "execute", localRequest, "--backend", "host", "--allow-unsafe-host-execution")
			// Ordinary Git replication carries audit relationships without private anchor/key.
			remote := filepath.Join(directory, "remote.git")
			runOperationalGit(t, sourceOffline, "init", "--bare", "-q", remote)
			runOperationalGit(t, sourceOffline, "remote", "add", "origin", remote)
			call(sourceOffline, true, "sync", "origin")
			reader := migrationCLIRepository(t)
			runOperationalGit(t, reader, "remote", "add", "origin", remote)
			call(reader, true, "replication", "select", "origin", "--actor", actor, "--proposal", proposal)
			call(reader, true, "sync", "origin")
			if !strings.Contains(string(audit(reader, true, "show", next)), next) || !strings.Contains(call(reader, true, "issue", "show", followup, "--json"), request) {
				t.Fatal("ordinary Git lost audit/follow-up relationships")
			}
			audit(reader, true, "logs", next)
			before = portableRefs(t, reader)
			audit(reader, false, "authority", "show")
			audit(reader, false, "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
			if before != portableRefs(t, reader) {
				t.Fatal("replication installed authority")
			}
		})
	}
}

func portablePublic(t *testing.T, binary, directory string, env []string, ok bool, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir, cmd.Env = directory, env
	out, err := cmd.CombinedOutput()
	if evidence := os.Getenv("HN_PORTABLE_TEST_TRANSCRIPTS"); evidence != "" {
		f, e := os.CreateTemp(evidence, "public-*.log")
		if e != nil {
			t.Fatal(e)
		}
		_, e = f.WriteString("test: " + t.Name() + "\nargv: " + strings.Join(args, " ") + "\noutput:\n" + string(out) + "\n")
		ce := f.Close()
		if e != nil || ce != nil {
			t.Fatal(e, ce)
		}
	}
	if (err == nil) != ok || ctx.Err() != nil {
		t.Fatalf("hn %v: %v wantOK=%v\n%s", args, err, ok, out)
	}
	return string(out)
}
func portableRefs(t *testing.T, repo string) string {
	t.Helper()
	return runOperationalGit(t, repo, "for-each-ref", "--format=%(refname) %(objectname)")
}
func portableGitBytes(t *testing.T, repo string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = operationalTestEnvironment(t.TempDir())
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func portableData(t *testing.T, output string) json.RawMessage {
	t.Helper()
	var v struct{ Data json.RawMessage }
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		t.Fatal(err)
	}
	return v.Data
}
