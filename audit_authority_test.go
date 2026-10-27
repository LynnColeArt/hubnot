package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditFixture(t *testing.T) (string, *Identity, string) {
	t.Helper()
	root := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "init", "-q", "-b", "main")
	mustGit(t, "config", "user.name", "Audit test")
	mustGit(t, "config", "user.email", "audit@hn.invalid")
	id, _, err := createIdentity("Owner")
	if err != nil {
		t.Fatal(err)
	}
	p := PolicyDocument{Version: policyVersion, Maintainers: []string{id.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Audits: &AuditPolicy{Scope: "project", RuleOwners: []string{id.Actor}, Requesters: []string{id.Actor}, Executors: []string{id.Actor}}}
	writeTestPolicy(t, root, p)
	raw := []byte(`{"version":"hn.pipeline/0","steps":[{"name":"check","command":"true"}]}`)
	if err := os.MkdirAll(filepath.Join(root, ".hn", "pipelines"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hn", "pipelines", "check.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-qm", "authority")
	return root, id, mustGitText(t, "rev-parse", "HEAD")
}

func TestAuditAuthorityExplicitAnchorAndWithdrawal(t *testing.T) {
	root, id, at := auditFixture(t)
	if _, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "check", Source: AuditSource{Kind: "pipeline", Commit: at, Path: ".hn/pipelines/check.json"}}, id); err == nil {
		t.Fatal("authoring without explicit anchor")
	}
	a, err := AuditAuthoritySet("refs/heads/main", "absent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuditAuthoritySet("refs/heads/main", "absent"); err == nil {
		t.Fatal("stale expected anchor")
	}
	if _, err := AuditAuthoritySet(at, a.Digest); err == nil {
		t.Fatal("accepted commit selector")
	}
	auth, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "check", Source: AuditSource{Kind: "pipeline", Commit: at, Path: ".hn/pipelines/check.json"}}, id)
	if err != nil {
		t.Fatal(err)
	}
	req, err := AuditRequest(at, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "commit", "--allow-empty", "-qm", "code only")
	if _, err := ValidateAuditExecution(req.ID, id.Actor, ""); err != nil {
		t.Fatal("identical policy code advance:", err)
	}
	p, _, _, err := loadPolicy("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	p.Audits = nil
	writeTestPolicy(t, root, p)
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "withdraw")
	if _, err := ValidateAuditExecution(req.ID, id.Actor, at); err == nil {
		t.Fatal("stale policy assertion revived authority")
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal("withdrawal erased historical records:", err)
	}
	if len(events) != 2 || auth.ID == "" {
		t.Fatal("history missing")
	}
}

func TestAuditAuthorityUnsafePrivatePaths(t *testing.T) {
	_, _, _ = auditFixture(t)
	gitdir, err := requireGitRepository()
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(gitdir, "hn", "audit")
	if err := os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), directory); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditAuthoritySet("refs/heads/main", "absent"); err == nil {
		t.Fatal("symlink authority directory")
	}
}

func TestAuditPolicyStrictAndDisabledDefault(t *testing.T) {
	_, id, _ := auditFixture(t)
	p := PolicyDocument{Version: policyVersion, Maintainers: []string{id.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}}
	raw, _ := json.Marshal(p)
	parsed, _, err := parsePolicyBytes(raw)
	if err != nil || parsed.Audits != nil {
		t.Fatal("old policy compatibility", err)
	}
	p.Audits = &AuditPolicy{Scope: "project", RuleOwners: []string{strings.Repeat("a", 64)}, Requesters: []string{id.Actor}, Executors: []string{id.Actor}}
	if err := validatePolicy(p); err != nil {
		t.Fatal(err)
	}
	p.Audits.Executors = nil
	if err := validatePolicy(p); err == nil {
		t.Fatal("empty role list")
	}
}
