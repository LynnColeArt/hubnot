//go:build !nogithub && !noforge

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/gitlabapi"
	"hubnot/internal/migration"
)

func migrationCLISeedGitLab(f *gitlabMigrationFixture) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "Portable issue", Body: "Original issue body", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}, Author: gitlabapi.User{ID: 3, Username: "source-author"}, CreatedAt: "2026-01-02T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}
	f.Notes = []gitlabapi.Note{{ID: 9001, Body: "Original conversation", NoteableID: 7001, Author: gitlabapi.User{ID: 4, Username: "source-commenter"}, CreatedAt: "2026-01-03T00:00:00Z", UpdatedAt: "2026-01-03T00:00:00Z"}}
}

func migrationCLIProviderArgs(source *gitlabMigrationFixture, dest *githubMigrationFixture, git, remote, session string) []string {
	args := []string{"--source-kind", "gitlab", "--source-endpoint", source.Config().Locator.Endpoint, "--source-repository", "group/nested/repo", "--source-api-version", "v4", "--source-credential-mode", "private-token", "--destination-kind", "github", "--destination-endpoint", dest.server.URL, "--destination-repository", "a/b", "--destination-api-version", "2022-11-28", "--destination-credential-mode", "bearer", "--source-token-env", "MIGRATION_SOURCE_TOKEN", "--destination-token-env", "MIGRATION_DEST_TOKEN", "--session", session, "--map", "state:opened=open", "--map", "type:issue=", "--map", "refs/heads/main=refs/heads/main", "--ref", "refs/heads/main", "--source-git", git, "--git-remote", remote, "--source-workflow-ref", strings.Repeat("a", 40)}
	for _, kind := range []string{"issue", "comment", "change_request", "review", "discussion", "reply", "inline_comment", "workflow"} {
		args = append(args, "--kind", kind)
	}
	return args
}
func migrationCLIProviderEnvironment() map[string]string {
	return map[string]string{"MIGRATION_SOURCE_TOKEN": "supplied-secret", "MIGRATION_DEST_TOKEN": "supplied-secret"}
}
func migrationCLIAssertNoWrites(t *testing.T, source *gitlabMigrationFixture, dest *githubMigrationFixture) {
	t.Helper()
	source.Mu.Lock()
	sw := source.IssueWrites + source.NoteWrites
	source.Mu.Unlock()
	dest.mu.Lock()
	dw := dest.issueWrites + dest.commentWrites
	dest.mu.Unlock()
	if sw+dw != 0 {
		t.Fatalf("unexpected source=%d destination=%d writes", sw, dw)
	}
}

func TestMigrationCLIGitLabToGitHubGuidedResumeAndOffline(t *testing.T) {
	binary := buildMigrationCLIBinary(t, "")
	source := newGitLabMigrationFixture(t)
	migrationCLISeedGitLab(source)
	dest := newGithubMigrationFixture(t)
	directory := t.TempDir()
	code := migrationCLIRepository(t)
	remote := migrationBare(t)
	session := filepath.Join(directory, "guided")
	env := migrationCLIProviderEnvironment()
	args := migrationCLIProviderArgs(source, dest, code, remote, session)
	previewOut := migrationCLIProcess(t, binary, directory, "", env, false, append([]string{"plan"}, args...)...)
	preview := migrationCLIPreview(t, previewOut)
	migrationCLIAssertNoWrites(t, source, dest)
	if _, err := os.Stat(session); !os.IsNotExist(err) {
		t.Fatal("missing decisions saved a usable session")
	}
	if len(preview.Intents) != 3 || preview.Intents[0].Kind != "issue" || preview.Intents[1].Kind != "comment" || preview.Intents[2].Kind != "git_refs" {
		t.Fatalf("not a real code/issue/comment migration: %+v", preview.Intents)
	}
	if len(preview.Intents[1].DependsOn) != 1 || preview.Intents[1].DependsOn[0] != preview.Intents[0].ID || !strings.Contains(string(preview.Intents[1].Payload), `"receipt"`) {
		t.Fatal("comment does not depend on saved issue receipt")
	}
	decisions := ""
	for _, f := range preview.Findings {
		if f.Status != "mapped" && f.Status != "preserved" {
			decisions += "preserve\n"
		}
	}
	if decisions == "" {
		t.Fatal("acceptance did not exercise unsupported retrieved content")
	}
	// Explicit selections still use wizard decisions, with no default consent.
	guidedArgs := append([]string{"plan", "--guided"}, args...)
	beforeDecisions := "-\n-\n-\n-\n" // explicitly absent organization/project selectors
	eofDecision := migrationCLIProcess(t, binary, directory, beforeDecisions, env, false, guidedArgs...)
	if !strings.Contains(eofDecision, "EOF") {
		t.Fatal("EOF at a loss decision was not refused")
	}
	migrationCLIAssertNoWrites(t, source, dest)
	eofFinal := migrationCLIProcess(t, binary, directory, beforeDecisions+decisions, env, false, guidedArgs...)
	if !strings.Contains(eofFinal, "EOF") {
		t.Fatal("EOF at final confirmation was not refused")
	}
	migrationCLIAssertNoWrites(t, source, dest)
	inspection := migrationCLIProcess(t, binary, directory, "", nil, true, "inspect", "--session", session)
	var saved struct {
		Plan     migration.Plan     `json:"plan"`
		Snapshot migration.Snapshot `json:"snapshot"`
	}
	if err := json.Unmarshal([]byte(inspection), &saved); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection, "Original conversation") || !strings.Contains(inspection, "source-commenter") {
		t.Fatal("attributed source communication not inspectable")
	}
	kinds := map[string]bool{}
	for _, r := range saved.Snapshot.Records {
		kinds[r.Key.Kind] = true
	}
	for _, kind := range []string{"change_request", "review", "workflow"} {
		if !kinds[kind] {
			t.Fatalf("retrieved %s not preserved", kind)
		}
	}
	if strings.Contains(previewOut+eofFinal+inspection, "supplied-secret") {
		t.Fatal("credential printed")
	}
	for _, name := range []string{"plan.json", "snapshot.json", "cli.json", "progress.json"} {
		raw, err := os.ReadFile(filepath.Join(session, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "supplied-secret") {
			t.Fatal("credential persisted in " + name)
		}
	}
	noConfirm := migrationCLIProcess(t, binary, directory, "", env, false, "apply", "--session", session)
	if !strings.Contains(noConfirm, "--confirm-apply") {
		t.Fatal("missing explicit apply guidance")
	}
	migrationCLIAssertNoWrites(t, source, dest)
	// Once saved, neither source API nor source checkout is a resume dependency.
	source.Server.Close()
	headCmd := exec.Command("git", "-C", code, "rev-parse", "HEAD")
	head, err := headCmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(code, code+"-unavailable"); err != nil {
		t.Fatal(err)
	}
	dest.mu.Lock()
	dest.loseComment = true
	dest.mu.Unlock()
	failed := migrationCLIProcess(t, binary, directory, "", env, false, "apply", "--session", session, "--confirm-apply", "--destination-token-env", "MIGRATION_DEST_TOKEN")
	if !strings.Contains(failed, "resume reconciles only") {
		t.Fatalf("not an uncertain comment failure:\n%s", failed)
	}
	// Inspect the public saved artifact, without changing progress or inventing a receipt.
	raw, err := os.ReadFile(filepath.Join(session, "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		Data migration.Progress `json:"data"`
	}
	if err = json.Unmarshal(raw, &checkpoint); err != nil {
		t.Fatal(err)
	}
	entries := checkpoint.Data.Entries
	if len(entries) != 2 || entries[0].State != "confirmed" || entries[1].State != "inflight" || len(entries[0].Receipts) != 1 {
		t.Fatalf("unexpected real progress: %+v", entries)
	}
	var bound migration.CommentWrite
	if err = json.Unmarshal(entries[1].Write.Payload, &bound); err != nil {
		t.Fatal(err)
	}
	if bound.Parent.Receipt != nil || bound.Parent.ID != entries[0].Receipts[0].RemoteKey {
		t.Fatal("saved comment did not bind the confirmed parent receipt")
	}
	migrationCLIProcess(t, binary, directory, "", env, true, "resume", "--session", session, "--confirm-apply", "--destination-token-env", "MIGRATION_DEST_TOKEN")
	dest.mu.Lock()
	if dest.issueWrites != 1 || dest.commentWrites != 1 || len(dest.comments) != 1 || dest.issue.Title != "Portable issue" || dest.issue.State != "open" || !strings.HasPrefix(dest.issue.Body, "Original issue body") || !strings.HasPrefix(dest.comments[0].Body, "Original conversation") {
		t.Fatalf("remote content or exactly-once recovery mismatch: issue=%+v comments=%+v writes=%d/%d", dest.issue, dest.comments, dest.issueWrites, dest.commentWrites)
	}
	dest.mu.Unlock()
	oidCmd := exec.Command("git", "--git-dir="+remote, "rev-parse", "refs/heads/main")
	oid, err := oidCmd.Output()
	if err != nil || string(oid) != string(head) {
		t.Fatalf("Git remote OID=%s want=%s err=%v", oid, head, err)
	}
	dest.server.Close()
	// All API clients are absent; saved bytes, imported records and current code remain usable.
	offline := buildMigrationCLIBinary(t, "noforge")
	migrationCLIProcess(t, offline, directory, "", nil, true, "inspect", "--session", session)
	local := migrationCLIRepository(t)
	localPlan := filepath.Join(directory, "local")
	migrationCLIPlanWithDecisions(t, offline, directory, nil, "--source-kind", "saved", "--source-repository", session, "--destination-kind", "local", "--destination-repository", local, "--session", localPlan)
	migrationCLIProcess(t, offline, directory, "", nil, true, "apply", "--session", localPlan, "--confirm-apply")
	imports := migrationCLIProcess(t, offline, directory, "", nil, true, "imports", "--repository", local)
	var digests []string
	if err = json.Unmarshal([]byte(imports), &digests); err != nil || len(digests) != 1 {
		t.Fatalf("imports=%s err=%v", imports, err)
	}
	offlineInspection := migrationCLIProcess(t, offline, directory, "", nil, true, "inspect", "--repository", local, "--snapshot", digests[0])
	if !strings.Contains(offlineInspection, "Original conversation") {
		t.Fatal("offline imported conversation lost")
	}
	archive := filepath.Join(directory, "continued.hn")
	archivePlan := filepath.Join(directory, "continued")
	migrationCLIPlanWithDecisions(t, offline, directory, nil, "--source-kind", "imported", "--source-repository", local, "--snapshot", digests[0], "--source-git", local, "--ref", "refs/heads/main", "--destination-kind", "archive", "--destination-repository", archive, "--session", archivePlan)
	migrationCLIProcess(t, offline, directory, "", nil, true, "apply", "--session", archivePlan, "--confirm-apply")
	archiveView := migrationCLIProcess(t, offline, directory, "", nil, true, "inspect", "--archive", archive)
	if !strings.Contains(archiveView, "Original conversation") {
		t.Fatal("imported-record continuation lost communication")
	}
}

func TestMigrationCLIPublicRefusalsPreserveRemoteState(t *testing.T) {
	binary := buildMigrationCLIBinary(t, "")
	for _, attack := range []string{"no_proof", "ambiguous", "stale_git", "hostile_git", "stale_repository", "tampered_profile", "missing_descriptor"} {
		t.Run(attack, func(t *testing.T) {
			source := newGitLabMigrationFixture(t)
			migrationCLISeedGitLab(source)
			dest := newGithubMigrationFixture(t)
			directory := t.TempDir()
			code := migrationCLIRepository(t)
			remote := migrationBare(t)
			session := filepath.Join(directory, "plan")
			env := migrationCLIProviderEnvironment()
			args := migrationCLIProviderArgs(source, dest, code, remote, session)
			migrationCLIPlanWithDecisions(t, binary, directory, env, args...)
			migrationCLIAssertNoWrites(t, source, dest)
			switch attack {
			case "tampered_profile":
				path := filepath.Join(session, "cli.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				modified := strings.Replace(string(raw), "2022-11-28", "2099-01-01", 1)
				if modified == string(raw) {
					t.Fatal("exact explicit profile missing from saved descriptor")
				}
				if err = os.WriteFile(path, []byte(modified), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing_descriptor":
				if err := os.Remove(filepath.Join(session, "cli.json")); err != nil {
					t.Fatal(err)
				}
			case "hostile_git":
				if err := os.Rename(remote, remote+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(remote+"-original", remote); err != nil {
					t.Fatal(err)
				}
			case "stale_git":
				cmd := exec.Command("git", "-C", code, "push", "--quiet", remote, "refs/heads/main:refs/heads/main")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("stale fixture: %v %s", err, out)
				}
			case "stale_repository":
				dest.mu.Lock()
				dest.repoID++
				dest.mu.Unlock()
			default:
				dest.mu.Lock()
				dest.loseIssue = true
				dest.mu.Unlock()
			}
			migrationCLIProcess(t, binary, directory, "", env, false, "apply", "--session", session, "--confirm-apply", "--destination-token-env", "MIGRATION_DEST_TOKEN")
			if attack == "no_proof" || attack == "ambiguous" {
				dest.mu.Lock()
				dest.hideIssue = attack == "no_proof"
				dest.duplicate = attack == "ambiguous"
				dest.mu.Unlock()
				output := migrationCLIProcess(t, binary, directory, "", env, false, "resume", "--session", session, "--confirm-apply", "--destination-token-env", "MIGRATION_DEST_TOKEN")
				if !strings.Contains(output, "manual recovery required") {
					t.Fatalf("unsafe uncertain recovery:\n%s", output)
				}
				dest.mu.Lock()
				if dest.issueWrites != 1 || dest.commentWrites != 0 {
					t.Fatal("uncertain create was duplicated")
				}
				dest.mu.Unlock()
			} else {
				migrationCLIAssertNoWrites(t, source, dest)
			}
		})
	}
}

func TestMigrationCLISelectedProfileCapabilitiesDoNotGuessCredentials(t *testing.T) {
	binary := buildMigrationCLIBinary(t, "")
	directory := t.TempDir()
	args := []string{"capabilities", "--source-kind", "github", "--source-endpoint", "https://unused.invalid", "--source-repository", "owner/repo", "--source-token-env", "ABSENT_SELECTED_TOKEN"}
	output := migrationCLIProcess(t, binary, directory, "", nil, false, args...)
	if !strings.Contains(output, "--source-api-version") {
		t.Fatal("missing explicit profile was not actionable")
	}
	args = append(args, "--source-api-version", "2022-11-28", "--source-credential-mode", "bearer")
	migrationCLIProcess(t, binary, directory, "", nil, true, args...)
	bitbucket := []string{"capabilities", "--destination-kind", "bitbucket", "--destination-endpoint", "https://api.bitbucket.org/2.0", "--destination-repository", "workspace/repo", "--destination-api-version", "2.0", "--destination-credential-mode", "api_token_basic", "--destination-token-env", "SELECTED_BITBUCKET_TOKEN"}
	output = migrationCLIProcess(t, binary, directory, "", nil, false, bitbucket...)
	if !strings.Contains(output, "email:api-token") {
		t.Fatalf("missing genuine basic credential was not reported: %s", output)
	}
	output = migrationCLIProcess(t, binary, directory, "", map[string]string{"SELECTED_BITBUCKET_TOKEN": "operator@example.invalid:actual-supplied-token"}, true, bitbucket...)
	if strings.Contains(output, "operator@example.invalid") || strings.Contains(output, "actual-supplied-token") {
		t.Fatal("credential leaked into capabilities")
	}
}

func TestMigrationCLILossyGitLabQuickActionRefusedBeforeWrites(t *testing.T) {
	binary := buildMigrationCLIBinary(t, "")
	source := newGitLabMigrationFixture(t)
	migrationCLISeedGitLab(source)
	dest := newGitLabMigrationFixture(t)
	source.Mu.Lock()
	source.Issue.Body = "Illustration:\n```\n /label ~secret\n```"
	source.Mu.Unlock()
	directory := t.TempDir()
	session := filepath.Join(directory, "lossy")
	args := []string{"plan", "--source-kind", "gitlab", "--source-endpoint", source.Config().Locator.Endpoint, "--source-repository", "group/nested/repo", "--source-api-version", "v4", "--source-credential-mode", "private-token", "--destination-kind", "gitlab", "--destination-endpoint", dest.Config().Locator.Endpoint, "--destination-repository", "group/nested/repo", "--destination-api-version", "v4", "--destination-credential-mode", "private-token", "--kind", "issue", "--kind", "comment", "--source-token-env", "MIGRATION_SOURCE_TOKEN", "--destination-token-env", "MIGRATION_DEST_TOKEN", "--session", session}
	output := migrationCLIProcess(t, binary, directory, "", migrationCLIProviderEnvironment(), false, args...)
	preview := migrationCLIPreview(t, output)
	if len(preview.Intents) != 0 || !strings.Contains(output, "slash-command lines are refused") {
		t.Fatalf("quick action was not a visible preflight limitation:\n%s", output)
	}
	source.Mu.Lock()
	sourceWrites := source.IssueWrites + source.NoteWrites
	source.Mu.Unlock()
	dest.Mu.Lock()
	destWrites := dest.IssueWrites + dest.NoteWrites
	dest.Mu.Unlock()
	if sourceWrites+destWrites != 0 {
		t.Fatal("lossy preview wrote to a provider")
	}
	if _, err := os.Stat(session); !os.IsNotExist(err) {
		t.Fatal("unanswered lossy preview saved a usable plan")
	}
}
