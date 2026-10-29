package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requesterFixture builds a repository whose base policy lists requesters
// for pipeline "test", opens a proposal, approves it, and has requester sign
// a run.request that a trusted runner passes. It returns the proposal.
func requesterFixture(t *testing.T, requesters func(maintainer, agent *Identity) []string, requester func(maintainer, agent, outsider *Identity) *Identity) StoredEvent {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	maintainer := testIdentity(t, "Maintainer")
	agent := testIdentity(t, "Orchestrator Agent")
	outsider := testIdentity(t, "Outsider")
	reviewer := testIdentity(t, "Reviewer")
	runner := testIdentity(t, "Runner")
	author := testIdentity(t, "Author")
	mustGit(t, "init", "-q", "-b", "main")
	mustGit(t, "config", "user.name", "Test")
	mustGit(t, "config", "user.email", "test@hn.invalid")
	writeTestPolicy(t, root, PolicyDocument{
		Version:     policyVersion,
		Maintainers: []string{maintainer.Actor},
		Proposals:   ProposalPolicy{RequiredApprovals: 1, RequiredAccepts: 1, TrustedReviewers: []string{reviewer.Actor}},
		Pipelines: map[string]PipelinePolicy{
			"test": {RequiredResults: 1, TrustedRunners: []string{runner.Actor}, Requesters: requesters(maintainer, agent)},
		},
	})
	if err := os.MkdirAll(filepath.Join(root, ".hn", "pipelines"), 0o755); err != nil {
		t.Fatal(err)
	}
	pipeline := []byte("{\"version\":\"hn.pipeline/0\",\"steps\":[{\"name\":\"Test\",\"command\":\"true\"}]}\n")
	if err := os.WriteFile(filepath.Join(root, ".hn", "pipelines", "test.json"), pipeline, 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-q", "-m", "base policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "change.txt")
	mustGit(t, "commit", "-q", "-m", "change")
	head := mustGitText(t, "rev-parse", "HEAD")

	opened := newEvent(author, "proposal.open", 1, "")
	opened.Title = "Ordinary change"
	opened.Base = base
	opened.Head = head
	proposal, err := appendEvent(opened, author)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	review := newEvent(reviewer, "review.submit", 1, "")
	review.Subject = proposal.ID
	review.Verdict = "approve"
	if _, err := appendEvent(review, reviewer); err != nil {
		t.Fatal(err)
	}
	signer := requester(maintainer, agent, outsider)
	requestEvent, err := nextEvent(signer, "run.request")
	if err != nil {
		t.Fatal(err)
	}
	_, _, definition, err := loadPipeline(head, "test")
	if err != nil {
		t.Fatal(err)
	}
	requestEvent.Subject = proposal.ID
	requestEvent.Pipeline = "test"
	requestEvent.Definition = definition
	requestEvent.Commit = head
	request, err := appendEvent(requestEvent, signer)
	if err != nil {
		t.Fatal(err)
	}
	log := []byte("tests passed\n")
	result := newEvent(runner, "run.result", 1, "")
	result.Subject = request.ID
	result.Pipeline = "test"
	result.Definition = definition
	result.Commit = head
	result.Outcome = "passed"
	result.DurationMS = 1
	result.Log = eventID(log)
	result.Backend = "sandbox"
	result.Platform = "test/test"
	result.Runner = "hn/test"
	if _, err := appendEventWithAttachments(result, runner, map[string][]byte{"log.txt": log}); err != nil {
		t.Fatal(err)
	}
	return *proposal
}

func requesterPasses(t *testing.T, proposal StoredEvent) int {
	t.Helper()
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	current, err := resolveEvent(events, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := evaluateProposal(current, events)
	if err != nil {
		t.Fatal(err)
	}
	for _, pipeline := range evaluation.Pipelines {
		if pipeline.Name == "test" {
			return len(pipeline.Passed)
		}
	}
	t.Fatal("pipeline test was not evaluated")
	return 0
}

func TestPipelineRequestersCountRunsFromListedNonMaintainers(t *testing.T) {
	listAgent := func(_, agent *Identity) []string { return []string{agent.Actor} }
	noList := func(_, _ *Identity) []string { return nil }
	byMaintainer := func(maintainer, _, _ *Identity) *Identity { return maintainer }
	byAgent := func(_, agent, _ *Identity) *Identity { return agent }
	byOutsider := func(_, _, outsider *Identity) *Identity { return outsider }
	cases := []struct {
		name       string
		requesters func(maintainer, agent *Identity) []string
		requester  func(maintainer, agent, outsider *Identity) *Identity
		want       int
	}{
		{"listed agent counts", listAgent, byAgent, 1},
		{"maintainer still counts with a list", listAgent, byMaintainer, 1},
		{"unlisted actor does not count", listAgent, byOutsider, 0},
		{"no list keeps maintainer-only", noList, byAgent, 0},
		{"no list maintainer counts", noList, byMaintainer, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposal := requesterFixture(t, tc.requesters, tc.requester)
			if got := requesterPasses(t, proposal); got != tc.want {
				t.Fatalf("passing results counted = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPipelineRequestersValidation(t *testing.T) {
	actor := testIdentity(t, "Actor")
	base := func(requesters []string) PolicyDocument {
		return PolicyDocument{
			Version:     policyVersion,
			Maintainers: []string{actor.Actor},
			Proposals:   ProposalPolicy{RequiredApprovals: 0, RequiredAccepts: 1},
			Pipelines:   map[string]PipelinePolicy{"test": {RequiredResults: 1, TrustedRunners: []string{actor.Actor}, Requesters: requesters}},
		}
	}
	if err := validatePolicy(base(nil)); err != nil {
		t.Fatalf("absent requesters rejected: %v", err)
	}
	if err := validatePolicy(base([]string{actor.Actor})); err != nil {
		t.Fatalf("valid requesters rejected: %v", err)
	}
	for name, requesters := range map[string][]string{
		"empty":     {},
		"invalid":   {"not-an-actor"},
		"duplicate": {actor.Actor, actor.Actor},
	} {
		err := validatePolicy(base(requesters))
		if err == nil || !strings.Contains(err.Error(), "requesters") {
			t.Fatalf("%s requesters accepted or unnamed: %v", name, err)
		}
	}
	parsed, _, err := parsePolicyBytes([]byte(`{"version":"hn.policy/0","maintainers":["` + actor.Actor + `"],"proposals":{"requiredApprovals":0,"requiredAccepts":1,"trustedReviewers":[],"allowAuthorApproval":false},"pipelines":{"test":{"requiredResults":1,"trustedRunners":["` + actor.Actor + `"],"requesters":["` + actor.Actor + `"]}}}`))
	if err != nil || len(parsed.Pipelines["test"].Requesters) != 1 {
		t.Fatalf("policy with requesters did not parse: %+v %v", parsed, err)
	}
}
