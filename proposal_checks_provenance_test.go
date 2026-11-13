package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticCheckRequiresTargetFirstParentProvenance(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	mustGit(t, "init", "-q", "-b", "main")
	mustGit(t, "config", "user.name", "Fixture")
	mustGit(t, "config", "user.email", "fixture@example.invalid")
	actor := deterministicMemoryIdentity().Actor
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}}
	writeTestPolicy(t, repo, policy)
	if err := os.MkdirAll(filepath.Join(".hn", "pipelines"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(proposalChecksPath, proposalChecksConfig{proposalChecksVersion, map[string][]string{"test": {"proposal.open"}}})
	write(".hn/pipelines/test.json", PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "Test", Command: "true"}}})
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-qm", "active target")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-qc", "candidate")
	mustGit(t, "commit", "--allow-empty", "-qm", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-qc", "side", base)
	mustGit(t, "commit", "--allow-empty", "-qm", "side")
	side := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")
	mustGit(t, "merge", "--no-ff", "-qm", "merge side", "side")
	id := eventID([]byte("proposal"))
	candidate := StoredEvent{ID: id, Event: Event{Kind: "proposal.open", Base: base, Head: head}}
	for _, tc := range []struct {
		source, target string
		eligible       bool
	}{
		{base, "refs/heads/main", true},
		{side, "refs/heads/main", false},
		{side, "refs/heads/side", false},
	} {
		_, digest, err := automaticConfigAt(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		_, _, definition, err := loadPipeline(tc.source, "test")
		if err != nil {
			t.Fatal(err)
		}
		request := Event{Actor: actor, Subject: id, Commit: head, Pipeline: "test", Definition: definition,
			Automatic: &automaticRunIntent{Version: proposalChecksVersion, Target: tc.target, Source: tc.source, Configuration: digest, Check: eventID([]byte("check")), Attempt: eventID([]byte("attempt"))}}
		if got := automaticDefinition(request, candidate, policy); got != tc.eligible {
			t.Errorf("source %s target %s eligibility = %v, want %v", tc.source, tc.target, got, tc.eligible)
		}
	}
}
