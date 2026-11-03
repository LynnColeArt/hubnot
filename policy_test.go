package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMemoryPolicyLegacyAndQualification(t *testing.T) {
	alice := deterministicMemoryIdentity()
	legacy := PolicyDocument{
		Version: policyVersion, Maintainers: []string{alice.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	parsed, digest, err := parsePolicyBytes(encoded)
	if err != nil || parsed.Memory != nil || digest != eventID(encoded) {
		t.Fatalf("legacy parse = %#v, %q, %v", parsed, digest, err)
	}
	if got := classifyMemoryTrust(nil, alice.Actor, memoryKindDecision); got != memoryTrustPolicyMissing {
		t.Fatalf("legacy trust = %q", got)
	}

	policy := &MemoryPolicy{TrustedActors: []string{alice.Actor}, TrustedKinds: []string{memoryKindDecision}}
	for _, test := range []struct {
		actor string
		kind  string
		want  string
	}{
		{alice.Actor, memoryKindDecision, memoryTrustQualified},
		{strings.Repeat("a", 64), memoryKindDecision, memoryTrustActorUntrusted},
		{alice.Actor, memoryKindAssumption, memoryTrustKindUntrusted},
		{strings.Repeat("a", 64), memoryKindAssumption, memoryTrustActorUntrusted},
	} {
		if got := classifyMemoryTrust(policy, test.actor, test.kind); got != test.want {
			t.Errorf("classifyMemoryTrust(%q, %q) = %q, want %q", test.actor, test.kind, got, test.want)
		}
	}
}

func TestMemoryPolicyValidation(t *testing.T) {
	alice := deterministicMemoryIdentity()
	base := PolicyDocument{
		Version: policyVersion, Maintainers: []string{alice.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
		Memory: &MemoryPolicy{TrustedActors: []string{alice.Actor}, TrustedKinds: []string{memoryKindDecision}},
	}
	if err := validatePolicy(base); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		edit func(*PolicyDocument)
		want string
	}{
		{"invalid actor", func(p *PolicyDocument) { p.Memory.TrustedActors = []string{"short"} }, "memory trustedActors contains invalid actor"},
		{"duplicate actor", func(p *PolicyDocument) { p.Memory.TrustedActors = []string{alice.Actor, alice.Actor} }, "memory trustedActors contains duplicate actor"},
		{"invalid kind", func(p *PolicyDocument) { p.Memory.TrustedKinds = []string{"wish"} }, "memory trustedKinds contains invalid kind"},
		{"duplicate kind", func(p *PolicyDocument) { p.Memory.TrustedKinds = []string{memoryKindDecision, memoryKindDecision} }, "memory trustedKinds contains duplicate kind"},
		{"unsorted actors", func(p *PolicyDocument) { p.Memory.TrustedActors = []string{strings.Repeat("f", 64), alice.Actor} }, "memory trustedActors must be sorted"},
		{"unsorted kinds", func(p *PolicyDocument) { p.Memory.TrustedKinds = []string{memoryKindObservation, memoryKindDecision} }, "memory trustedKinds must be sorted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := base
			memory := *base.Memory
			memory.TrustedActors = append([]string(nil), base.Memory.TrustedActors...)
			memory.TrustedKinds = append([]string(nil), base.Memory.TrustedKinds...)
			policy.Memory = &memory
			test.edit(&policy)
			if err := validatePolicy(policy); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestMemoryPolicyStrictJSONAndExactBytes(t *testing.T) {
	alice := deterministicMemoryIdentity()
	policy := PolicyDocument{
		Version: policyVersion, Maintainers: []string{alice.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
		Memory: &MemoryPolicy{TrustedActors: []string{}, TrustedKinds: []string{}},
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, digest, err := parsePolicyBytes(encoded); err != nil || digest != eventID(encoded) {
		t.Fatalf("exact byte digest = %q, %v", digest, err)
	}
	unknown := bytes.Replace(encoded, []byte(`"trustedKinds":[]`), []byte(`"trustedKinds":[],"surprise":true`), 1)
	if _, _, err := parsePolicyBytes(unknown); err == nil {
		t.Fatal("accepted unknown memory policy field")
	}
	if _, _, err := parsePolicyBytes(append(encoded, []byte(` {}`)...)); err == nil {
		t.Fatal("accepted trailing policy JSON")
	}
}

func TestPolicyEvidenceDecisionAndMerge(t *testing.T) {
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	alice := testIdentity(t, "Alice")
	reviewer := testIdentity(t, "Reviewer")
	runner := testIdentity(t, "Runner")
	mustGit(t, "init", "-q", "-b", "main")
	mustGit(t, "config", "user.name", "Test")
	mustGit(t, "config", "user.email", "test@hn.invalid")
	writeTestPolicy(t, root, PolicyDocument{
		Version:     policyVersion,
		Maintainers: []string{alice.Actor},
		Proposals: ProposalPolicy{
			RequiredApprovals:   1,
			RequiredAccepts:     1,
			TrustedReviewers:    []string{reviewer.Actor},
			AllowAuthorApproval: false,
		},
		Pipelines: map[string]PipelinePolicy{
			"test": {RequiredResults: 1, TrustedRunners: []string{runner.Actor}},
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

	// The proposed head changes policy, but this proposal must continue using
	// the policy from its signed base commit.
	writeTestPolicy(t, root, PolicyDocument{
		Version:     policyVersion,
		Maintainers: []string{alice.Actor},
		Proposals: ProposalPolicy{
			RequiredApprovals:   0,
			RequiredAccepts:     1,
			AllowAuthorApproval: true,
		},
		Pipelines: map[string]PipelinePolicy{},
	})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-q", "-m", "proposed policy change")
	head := mustGitText(t, "rev-parse", "HEAD")

	proposalEvent := newEvent(alice, "proposal.open", 1, "")
	proposalEvent.Title = "Governed change"
	proposalEvent.Base = base
	proposalEvent.Head = head
	proposal, err := appendEvent(proposalEvent, alice)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := evaluateProposal(proposal, events)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Ready || evaluation.Policy.Proposals.RequiredApprovals != 1 {
		t.Fatal("proposal did not use the immutable base policy")
	}

	reviewEvent := newEvent(reviewer, "review.submit", 1, "")
	reviewEvent.Subject = proposal.ID
	reviewEvent.Verdict = "approve"
	if _, err := appendEvent(reviewEvent, reviewer); err != nil {
		t.Fatal(err)
	}
	requestEvent, err := nextEvent(alice, "run.request")
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
	request, err := appendEvent(requestEvent, alice)
	if err != nil {
		t.Fatal(err)
	}
	log := []byte("tests passed\n")
	resultEvent := newEvent(runner, "run.result", 1, "")
	resultEvent.Subject = request.ID
	resultEvent.Pipeline = "test"
	resultEvent.Definition = definition
	resultEvent.Commit = head
	resultEvent.Outcome = "passed"
	resultEvent.DurationMS = 1
	resultEvent.Log = eventID(log)
	resultEvent.Backend = "sandbox"
	resultEvent.Platform = "test/test"
	resultEvent.Runner = "hn/test"
	if _, err := appendEventWithAttachments(resultEvent, runner, map[string][]byte{"log.txt": log}); err != nil {
		t.Fatal(err)
	}

	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	proposal, err = resolveEvent(events, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err = evaluateProposal(proposal, events)
	if err != nil {
		t.Fatal(err)
	}
	if !evaluation.Ready || len(evaluation.Evidence) != 2 {
		t.Fatalf("evaluation ready=%v evidence=%d, want true and 2", evaluation.Ready, len(evaluation.Evidence))
	}

	decisionEvent, err := nextEvent(alice, "proposal.decision")
	if err != nil {
		t.Fatal(err)
	}
	decisionEvent.Subject = proposal.ID
	decisionEvent.Policy = evaluation.PolicyDigest
	decisionEvent.Verdict = "accept"
	decisionEvent.Evidence = evaluation.Evidence
	decision, err := appendEvent(decisionEvent, alice)
	if err != nil {
		t.Fatal(err)
	}
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	proposal, _ = resolveEvent(events, proposal.ID)
	evaluation, err = evaluateProposal(proposal, events)
	if err != nil {
		t.Fatal(err)
	}
	if !evaluation.Accepted {
		t.Fatal("valid signed decision did not accept proposal")
	}

	mergeEvent, err := nextEvent(alice, "proposal.merged")
	if err != nil {
		t.Fatal(err)
	}
	mergeEvent.Subject = proposal.ID
	mergeEvent.Policy = evaluation.PolicyDigest
	mergeEvent.Head = head
	mergeEvent.Commit = head
	mergeEvent.Evidence = []string{decision.ID}
	if _, err := appendEvent(mergeEvent, alice); err != nil {
		t.Fatal(err)
	}
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	proposal, _ = resolveEvent(events, proposal.ID)
	evaluation, err = evaluateProposal(proposal, events)
	if err != nil {
		t.Fatal(err)
	}
	if !evaluation.Merged || proposalStatus(evaluation) != "merged" {
		t.Fatal("valid merge evidence did not produce merged status")
	}
}

// Exercise the existing proposal-evaluation contract with real Git commits and
// signed requests/results. The shared evaluator's shape is deliberately absent
// from this test so extraction cannot make its own implementation the oracle.
func TestPolicyPipelineEvidenceAcrossRequests(t *testing.T) {
	root := inReplicationTestRepository(t)
	maintainer := testIdentity(t, "Maintainer")
	requester := testIdentity(t, "Requester")
	outsider := testIdentity(t, "Outsider")
	runnerA := testIdentity(t, "Runner A")
	runnerB := testIdentity(t, "Runner B")
	stranger := testIdentity(t, "Untrusted runner")
	writeTestPolicy(t, root, PolicyDocument{
		Version: policyVersion, Maintainers: []string{maintainer.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1},
		Pipelines: map[string]PipelinePolicy{
			"test": {RequiredResults: 2, TrustedRunners: []string{runnerA.Actor, runnerB.Actor}, Requesters: []string{requester.Actor}},
		},
	})
	writeTestPipeline(t, root)
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-qm", "Base policy and pipeline")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-qm", "Feature")
	head := mustGitText(t, "rev-parse", "HEAD")
	proposal := appendPolicyTestProposal(t, maintainer, base, head, "Evaluate pipeline evidence")
	otherProposal := appendPolicyTestProposal(t, maintainer, base, head, "Independent candidate")
	_, _, definition, err := loadPipeline(head, "test")
	if err != nil {
		t.Fatal(err)
	}
	request := func(actor *Identity, candidate *StoredEvent, pipeline, digest string) *StoredEvent {
		t.Helper()
		event, err := nextEvent(actor, "run.request")
		if err != nil {
			t.Fatal(err)
		}
		event.Subject, event.Pipeline, event.Definition, event.Commit = candidate.ID, pipeline, digest, head
		stored, err := appendEvent(event, actor)
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	result := func(actor *Identity, request *StoredEvent, outcome string) *StoredEvent {
		t.Helper()
		event, err := nextEvent(actor, "run.result")
		if err != nil {
			t.Fatal(err)
		}
		log := []byte(outcome + "\n")
		event.Subject, event.Pipeline, event.Definition, event.Commit = request.ID, request.Event.Pipeline, request.Event.Definition, head
		event.Outcome, event.Log, event.Backend, event.Platform, event.Runner = outcome, eventID(log), "sandbox", "test/test", "hn/test"
		if outcome == "failed" {
			event.ExitCode = 1
		}
		stored, err := appendEventWithAttachments(event, actor, map[string][]byte{"log.txt": log})
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	check := func(ready bool, passed, failed []*StoredEvent) {
		t.Helper()
		events, err := collectEvents()
		if err != nil {
			t.Fatal(err)
		}
		evaluation, err := evaluateProposal(proposal, events)
		if err != nil {
			t.Fatal(err)
		}
		if len(evaluation.Pipelines) != 1 {
			t.Fatalf("required pipelines = %d, want 1", len(evaluation.Pipelines))
		}
		pipeline := evaluation.Pipelines[0]
		if evaluation.Ready != ready || pipeline.Name != "test" || pipeline.Required != 2 || len(pipeline.Passed) != len(passed) || len(pipeline.Failed) != len(failed) {
			t.Fatalf("ready=%t pipeline=%+v; want ready=%t, %d passes and %d failures", evaluation.Ready, pipeline, ready, len(passed), len(failed))
		}
		for _, pair := range []struct {
			actual []StoredEvent
			want   []*StoredEvent
		}{{pipeline.Passed, passed}, {pipeline.Failed, failed}} {
			for i, want := range pair.want {
				if pair.actual[i].ID != want.ID {
					t.Fatalf("pipeline result %d = %s, want %s", i, pair.actual[i].ID, want.ID)
				}
			}
		}
		if len(evaluation.Evidence) != len(passed) {
			t.Fatalf("qualifying evidence = %v, want exactly the passing results", evaluation.Evidence)
		}
		for _, want := range passed {
			if !slices.Contains(evaluation.Evidence, want.ID) {
				t.Fatalf("qualifying evidence omitted passing result %s", want.ID)
			}
		}
	}

	first := request(maintainer, proposal, "test", definition)
	second := request(requester, proposal, "test", definition)
	result(runnerA, first, "passed")
	latestA := result(runnerA, second, "passed")
	check(false, []*StoredEvent{latestA}, nil) // Two requests do not make one runner count twice.
	passedB := result(runnerB, second, "passed")
	check(true, []*StoredEvent{latestA, passedB}, nil)

	result(stranger, first, "passed")
	result(runnerB, request(outsider, proposal, "test", definition), "failed")
	result(runnerB, request(maintainer, proposal, "test", eventID([]byte("other definition"))), "failed")
	result(runnerB, request(maintainer, proposal, "other", definition), "failed")
	result(runnerB, request(maintainer, otherProposal, "test", definition), "failed")
	check(true, []*StoredEvent{latestA, passedB}, nil) // Later ineligible claims do not replace qualifying evidence.

	failedA := result(runnerA, first, "failed")
	check(false, []*StoredEvent{passedB}, []*StoredEvent{failedA})
	passedA := result(runnerA, second, "passed")
	check(true, []*StoredEvent{passedB, passedA}, nil)
}

func writeTestPolicy(t *testing.T, root string, policy PolicyDocument) {
	t.Helper()
	encoded, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Join(root, ".hn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hn", "policy.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}
