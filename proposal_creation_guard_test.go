package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProposalCandidateAncestryBeforeSigning(t *testing.T) {
	root := inReplicationTestRepository(t)
	identity, _, err := createIdentity("Proposal guard")
	if err != nil {
		t.Fatal(err)
	}
	writeTestPolicy(t, root, PolicyDocument{
		Version: policyVersion, Maintainers: []string{identity.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
	})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "base policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-qm", "feature")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "--detach", "-q", base)
	mustGit(t, "commit", "--allow-empty", "-qm", "new main base")
	otherBase := mustGitText(t, "rev-parse", "HEAD")
	// Reuse the byte/mode snapshot fixture to include objects, refs and signer
	// files, not just an event count that could miss an orphan signed append.
	state := &artifactCLIFixture{t: t, repo: root, home: t.TempDir()}
	checkRefusal := func(args ...string) {
		t.Helper()
		before := state.snapshot()
		_, err := captureTestOutput(t, func() error { return cmdProposal(args) })
		if err == nil || !strings.Contains(err.Error(), "proposal head "+head+" does not descend from base "+otherBase) {
			t.Fatalf("wrong ancestry refusal: %v", err)
		}
		if !reflect.DeepEqual(before, state.snapshot()) {
			t.Fatal("refused proposal changed repository/private state")
		}
	}
	checkRefusal("open", "--base", otherBase, "--head", head, "Invalid opening")
	// Positive control: the valid candidate is still created by this author.
	if _, err := captureTestOutput(t, func() error {
		return cmdProposalOpen([]string{"--base", base, "--head", head, "Valid opening"})
	}); err != nil {
		t.Fatal(err)
	}
	events, err := collectEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("valid author history: %d events, %v", len(events), err)
	}
	checkRefusal("revise", events[0].ID, "--base", otherBase, "--head", head)
	if _, err := captureTestOutput(t, func() error {
		return cmdProposalRevise([]string{events[0].ID, "--base", base, "--head", otherBase})
	}); err != nil {
		t.Fatalf("valid revision refused: %v", err)
	}
	// Exact parent objects prove ancestry even when Git's ordinary revision
	// walker stops at a shallow marker. No fetch or global unshallow is needed.
	if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(head+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := guardCandidateCreationDependencies("proposal open", base, head); err != nil {
		t.Fatalf("complete exact ancestry behind shallow marker refused: %v", err)
	}
}

func TestProposalCandidateMissingParentBeforeSigning(t *testing.T) {
	root := inReplicationTestRepository(t)
	identity, _, err := createIdentity("Missing parent guard")
	if err != nil {
		t.Fatal(err)
	}
	writeTestPolicy(t, root, PolicyDocument{
		Version: policyVersion, Maintainers: []string{identity.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
	})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "base policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-qm", "intermediate")
	middle := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-qm", "head")
	head := mustGitText(t, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(head+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".git", "objects", middle[:2], middle[2:])); err != nil {
		t.Fatal(err)
	}
	state := &artifactCLIFixture{t: t, repo: root, home: t.TempDir()}
	before := state.snapshot()
	_, err = captureTestOutput(t, func() error {
		return cmdProposalOpen([]string{"--base", base, "--head", head, "Missing ancestor"})
	})
	if err == nil || !strings.Contains(err.Error(), middle) || strings.Contains(err.Error(), "does not descend") {
		t.Fatalf("missing parent was not diagnosed separately: %v", err)
	}
	gap, readErr := loadShallowGapRecord()
	if readErr != nil || gap == nil || gap.Gap == nil || gap.Gap.MissingID != middle || gap.Scope == nil || gap.Scope.Base != base || gap.Scope.Head != head {
		t.Fatalf("missing-parent recovery diagnostic lost exact context: %+v, %v", gap, readErr)
	}
	after := state.snapshot()
	// The existing shallow-recovery contract deliberately records this exact
	// private diagnostic. Every other byte/mode, including objects/refs and
	// signer state, must remain identical.
	diagnostic := filepath.Join(root, ".git", "hn", "shallow-gap.json")
	if before[diagnostic] != "" || after[diagnostic] == "" {
		t.Fatal("expected exactly the newly recorded shallow gap diagnostic")
	}
	delete(after, diagnostic)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("missing ancestry changed state beyond its recovery diagnostic")
	}
}
