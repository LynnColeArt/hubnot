package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type trustIDFixture struct {
	root     string
	identity *Identity
	base     string
	head     string
	issue    *StoredEvent
	proposal *StoredEvent
	request  *StoredEvent
}

func setupTrustIDFixture(t *testing.T) trustIDFixture {
	t.Helper()
	root := inIdentityTestRepository(t)
	identity, _, err := createIdentity("Trust Test")
	if err != nil {
		t.Fatal(err)
	}
	writeTestPolicy(t, root, PolicyDocument{
		Version:     policyVersion,
		Maintainers: []string{identity.Actor},
		Proposals: ProposalPolicy{
			RequiredApprovals:   1,
			RequiredAccepts:     1,
			TrustedReviewers:    []string{identity.Actor},
			AllowAuthorApproval: true,
		},
		Pipelines: map[string]PipelinePolicy{},
	})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-q", "-m", "trust policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "-c", "candidate")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")

	issueEvent, err := nextEvent(identity, "issue.open")
	if err != nil {
		t.Fatal(err)
	}
	issueEvent.Title = "Full IDs"
	issue, err := appendEvent(issueEvent, identity)
	if err != nil {
		t.Fatal(err)
	}
	proposalEvent, err := nextEvent(identity, "proposal.open")
	if err != nil {
		t.Fatal(err)
	}
	proposalEvent.Title = "Full IDs"
	proposalEvent.Base = base
	proposalEvent.Head = head
	proposal, err := appendEvent(proposalEvent, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	requestEvent, err := nextEvent(identity, "run.request")
	if err != nil {
		t.Fatal(err)
	}
	requestEvent.Subject = proposal.ID
	requestEvent.Pipeline = "check"
	requestEvent.Definition = "sha256:" + strings.Repeat("d", 64)
	requestEvent.Commit = head
	request, err := appendEvent(requestEvent, identity)
	if err != nil {
		t.Fatal(err)
	}
	return trustIDFixture{root: root, identity: identity, base: base, head: head, issue: issue, proposal: proposal, request: request}
}

func TestTrustBearingCommandsRejectUnambiguousPrefixes(t *testing.T) {
	tests := []struct {
		name string
		run  func(trustIDFixture) error
	}{
		{"issue-comment", func(f trustIDFixture) error { return cmdIssueComment([]string{shortID(f.issue.ID), "comment"}) }},
		{"proposal-revise", func(f trustIDFixture) error {
			return cmdProposalRevise([]string{shortID(f.proposal.ID), "--base", f.base, "--head", f.head})
		}},
		{"proposal-status-with-selection", func(f trustIDFixture) error {
			if err := saveReplicationSelection(ReplicationSelection{
				Version: replicationSelectionVersion, Remote: "origin", Proposals: []string{f.proposal.ID}, Budgets: defaultReplicationBudgets(),
			}); err != nil {
				return err
			}
			return cmdProposalStatus(shortID(f.proposal.ID))
		}},
		{"review", func(f trustIDFixture) error { return cmdReview([]string{shortID(f.proposal.ID), "--approve"}) }},
		{"run-request", func(f trustIDFixture) error { return cmdRunRequest([]string{shortID(f.proposal.ID), "check"}) }},
		{"run-execute", func(f trustIDFixture) error {
			return cmdRunExecute([]string{shortID(f.request.ID), "--backend", "host", "--allow-unsafe-host-execution"})
		}},
		{"decision", func(f trustIDFixture) error { return cmdDecide([]string{shortID(f.proposal.ID), "--accept"}) }},
		{"merge", func(f trustIDFixture) error { return cmdMerge([]string{shortID(f.proposal.ID)}) }},
		{"identity-accept", func(f trustIDFixture) error { return cmdIdentityAccept([]string{shortID(f.proposal.ID)}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := setupTrustIDFixture(t)
			err := test.run(fixture)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "full") {
				t.Fatalf("prefix command error = %v, want full event ID rejection", err)
			}
		})
	}
}

func TestTrustBearingFullIDsAndDisplayPrefixesRemainDistinct(t *testing.T) {
	fixture := setupTrustIDFixture(t)
	if _, err := captureTestOutput(t, func() error { return cmdProposalStatus(fixture.proposal.ID) }); err != nil {
		t.Fatalf("full proposal status ID: %v", err)
	}
	if _, err := captureTestOutput(t, func() error { return cmdIssueComment([]string{fixture.issue.ID, "full ID comment"}) }); err != nil {
		t.Fatalf("full issue comment ID: %v", err)
	}
	if _, err := captureTestOutput(t, func() error { return cmdProposalShow(shortID(fixture.proposal.ID)) }); err != nil {
		t.Fatalf("display-only proposal prefix: %v", err)
	}
	if _, err := captureTestOutput(t, func() error { return cmdIssueShow(shortID(fixture.issue.ID)) }); err != nil {
		t.Fatalf("display-only issue prefix: %v", err)
	}
}

func TestDecisionRecoveryHintUsesExecutableFullProposalID(t *testing.T) {
	fixture := setupTrustIDFixture(t)
	err := cmdDecide([]string{fixture.proposal.ID, "--accept"})
	if err == nil {
		t.Fatal("not-ready decision unexpectedly succeeded")
	}
	const commandPrefix = "hn proposal status "
	start := strings.Index(err.Error(), commandPrefix)
	if start < 0 {
		t.Fatalf("decision recovery diagnostic lacks status command: %v", err)
	}
	suggested := strings.Trim(strings.Fields(err.Error()[start+len(commandPrefix):])[0], "'\"")
	if suggested != fixture.proposal.ID {
		t.Fatalf("decision recovery suggested %q, want full proposal ID %q", suggested, fixture.proposal.ID)
	}
	if _, statusErr := captureTestOutput(t, func() error { return cmdProposalStatus(suggested) }); statusErr != nil {
		t.Fatalf("suggested recovery command was not executable: %v", statusErr)
	}
}

type mergeRepairFixture struct {
	root       string
	maintainer *Identity
	reviewer   *Identity
	proposal   *StoredEvent
	request    *StoredEvent
	decision   *StoredEvent
}

func setupMergeRepairFixture(t *testing.T) mergeRepairFixture {
	t.Helper()
	root := inIdentityTestRepository(t)
	maintainer, _, err := createIdentity("Maintainer")
	if err != nil {
		t.Fatal(err)
	}
	reviewer := testIdentity(t, "Reviewer Runner")
	writeTestPolicy(t, root, PolicyDocument{
		Version:     policyVersion,
		Maintainers: []string{maintainer.Actor},
		Proposals: ProposalPolicy{
			RequiredApprovals: 1,
			RequiredAccepts:   1,
			TrustedReviewers:  []string{reviewer.Actor},
		},
		Pipelines: map[string]PipelinePolicy{
			"check": {RequiredResults: 1, TrustedRunners: []string{reviewer.Actor}},
		},
	})
	if err := os.MkdirAll(filepath.Join(root, ".hn", "pipelines"), 0o755); err != nil {
		t.Fatal(err)
	}
	pipeline := []byte("{\"version\":\"hn.pipeline/0\",\"steps\":[{\"name\":\"Check\",\"command\":\"true\"}]}\n")
	if err := os.WriteFile(filepath.Join(root, ".hn", "pipelines", "check.json"), pipeline, 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-q", "-m", "repair policy and pipeline")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "-c", "candidate")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "repair candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")

	proposalEvent, err := nextEvent(maintainer, "proposal.open")
	if err != nil {
		t.Fatal(err)
	}
	proposalEvent.Title = "Repair missing merge event"
	proposalEvent.Base = base
	proposalEvent.Head = head
	proposal, err := appendEvent(proposalEvent, maintainer)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	review := appendRevisionReview(t, reviewer, proposal.ID)
	_, _, definition, err := loadPipeline(head, "check")
	if err != nil {
		t.Fatal(err)
	}
	requestEvent, err := nextEvent(maintainer, "run.request")
	if err != nil {
		t.Fatal(err)
	}
	requestEvent.Subject = proposal.ID
	requestEvent.Pipeline = "check"
	requestEvent.Definition = definition
	requestEvent.Commit = head
	request, err := appendEvent(requestEvent, maintainer)
	if err != nil {
		t.Fatal(err)
	}
	result := appendRevisionRunResult(t, reviewer, request)
	_, _, policyDigest, err := loadPolicy(base)
	if err != nil {
		t.Fatal(err)
	}
	decision := appendRevisionDecision(t, maintainer, proposal.ID, policyDigest, []string{review.ID, result.ID})
	return mergeRepairFixture{root: root, maintainer: maintainer, reviewer: reviewer, proposal: proposal, request: request, decision: decision}
}

func crashAfterCodeMerge(t *testing.T, fixture mergeRepairFixture) string {
	t.Helper()
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	lock := filepath.Join(gitDir, "refs", "hn", "actors", fixture.maintainer.Actor+".lock")
	if err := os.WriteFile(lock, []byte("inject append failure\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := cmdMerge([]string{fixture.proposal.ID})
	if err == nil || !strings.Contains(err.Error(), "recording failed") {
		t.Fatalf("merge crash = %v, want post-merge recording failure", err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	mergeCommit := mustGitText(t, "rev-parse", "HEAD")
	contained, missing, err := exactCommitAncestor(fixture.proposal.Event.Head, mergeCommit)
	if err != nil || missing != "" || !contained {
		t.Fatalf("candidate was not merged before recording failure: contained=%t missing=%s err=%v", contained, missing, err)
	}
	return mergeCommit
}

func TestMergeRetryRepairsMissingEventAndIsIdempotent(t *testing.T) {
	fixture := setupMergeRepairFixture(t)
	mergeCommit := crashAfterCodeMerge(t, fixture)
	output, err := captureTestOutput(t, func() error { return cmdMerge([]string{fixture.proposal.ID}) })
	if err != nil {
		t.Fatalf("repair retry: %v", err)
	}
	if !strings.Contains(output, "Recorded missing merge event") {
		t.Fatalf("repair output did not identify reconciliation:\n%s", output)
	}
	if got := mustGitText(t, "rev-parse", "HEAD"); got != mergeCommit {
		t.Fatalf("repair reran or advanced the code merge: HEAD=%s want=%s", got, mergeCommit)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	merges := mergeEventsForProposal(events, fixture.proposal.ID)
	if len(merges) != 1 || merges[0].Event.Commit != mergeCommit || merges[0].Event.Head != fixture.proposal.Event.Head ||
		merges[0].Event.Policy != fixture.decision.Event.Policy || len(merges[0].Event.Evidence) != 1 || merges[0].Event.Evidence[0] != fixture.decision.ID {
		t.Fatalf("repaired merge facts = %#v", merges)
	}
	if err := cmdMerge([]string{fixture.proposal.ID}); err == nil || !strings.Contains(err.Error(), "already recorded as merged") {
		t.Fatalf("idempotent retry = %v", err)
	}
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(mergeEventsForProposal(events, fixture.proposal.ID)); got != 1 {
		t.Fatalf("idempotent retry created %d merge facts", got)
	}
}

func TestShallowMergeRetryRepairsMissingEventWithAppendCAS(t *testing.T) {
	fixture := setupMergeRepairFixture(t)
	mergeCommit := crashAfterCodeMerge(t, fixture)
	shallowPath := mustGitText(t, "rev-parse", "--git-path", "shallow")
	if err := os.WriteFile(shallowPath, []byte(fixture.proposal.Event.Base+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shallow, err := repositoryIsShallow()
	if err != nil || !shallow {
		t.Fatalf("fixture is not shallow: shallow=%t err=%v", shallow, err)
	}

	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	lock := filepath.Join(gitDir, "refs", "hn", "actors", fixture.maintainer.Actor+".lock")
	if err := os.WriteFile(lock, []byte("inject reconciliation append failure\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockedErr := cmdMerge([]string{fixture.proposal.ID})
	if lockedErr == nil || !strings.Contains(lockedErr.Error(), "merge event repair") {
		t.Fatalf("shallow repair with append lock = %v, want retryable repair failure", lockedErr)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if got := mustGitText(t, "rev-parse", "HEAD"); got != mergeCommit {
		t.Fatalf("failed shallow repair advanced HEAD to %s, want %s", got, mergeCommit)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(mergeEventsForProposal(events, fixture.proposal.ID)); got != 0 {
		t.Fatalf("failed shallow repair published %d merge facts", got)
	}

	output, err := captureTestOutput(t, func() error { return cmdMerge([]string{fixture.proposal.ID}) })
	if err != nil {
		t.Fatalf("shallow repair retry: %v", err)
	}
	if !strings.Contains(output, "Recorded missing merge event") {
		t.Fatalf("shallow repair output did not identify reconciliation:\n%s", output)
	}
	if got := mustGitText(t, "rev-parse", "HEAD"); got != mergeCommit {
		t.Fatalf("shallow repair reran or advanced the code merge: HEAD=%s want=%s", got, mergeCommit)
	}
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	merges := mergeEventsForProposal(events, fixture.proposal.ID)
	if len(merges) != 1 || merges[0].Event.Commit != mergeCommit || merges[0].Event.Head != fixture.proposal.Event.Head ||
		merges[0].Event.Policy != fixture.decision.Event.Policy || len(merges[0].Event.Evidence) != 1 || merges[0].Event.Evidence[0] != fixture.decision.ID {
		t.Fatalf("shallow repaired merge facts = %#v", merges)
	}
	if err := cmdMerge([]string{fixture.proposal.ID}); err == nil || !strings.Contains(err.Error(), "already recorded as merged") {
		t.Fatalf("idempotent shallow retry = %v", err)
	}
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(mergeEventsForProposal(events, fixture.proposal.ID)); got != 1 {
		t.Fatalf("idempotent shallow retry created %d merge facts", got)
	}
}

func TestMergeRetryFailsClosedWhenCurrentEvidenceChanged(t *testing.T) {
	for _, invalidate := range []struct {
		name string
		run  func(*testing.T, mergeRepairFixture)
	}{
		{"approval", func(t *testing.T, fixture mergeRepairFixture) {
			event, err := nextEvent(fixture.reviewer, "review.submit")
			if err != nil {
				t.Fatal(err)
			}
			event.Subject = fixture.proposal.ID
			event.Verdict = "request-changes"
			if _, err := appendEvent(event, fixture.reviewer); err != nil {
				t.Fatal(err)
			}
		}},
		{"ci", func(t *testing.T, fixture mergeRepairFixture) {
			log := []byte("current run failed\n")
			event, err := nextEvent(fixture.reviewer, "run.result")
			if err != nil {
				t.Fatal(err)
			}
			event.Subject = fixture.request.ID
			event.Pipeline = fixture.request.Event.Pipeline
			event.Definition = fixture.request.Event.Definition
			event.Commit = fixture.request.Event.Commit
			event.Outcome = "failed"
			event.ExitCode = 1
			event.DurationMS = 1
			event.Log = eventID(log)
			event.Backend = "host"
			event.Platform = "test/test"
			event.Runner = "hn/test"
			if _, err := appendEventWithAttachments(event, fixture.reviewer, map[string][]byte{"log.txt": log}); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(invalidate.name, func(t *testing.T) {
			fixture := setupMergeRepairFixture(t)
			crashAfterCodeMerge(t, fixture)
			invalidate.run(t, fixture)
			err := cmdMerge([]string{fixture.proposal.ID})
			if err == nil || !strings.Contains(err.Error(), "current approval and CI evidence") {
				t.Fatalf("stale-evidence repair = %v", err)
			}
			events, collectErr := collectEvents()
			if collectErr != nil {
				t.Fatal(collectErr)
			}
			if got := len(mergeEventsForProposal(events, fixture.proposal.ID)); got != 0 {
				t.Fatalf("stale-evidence repair created %d merge facts", got)
			}
		})
	}
}

func mergeEventsForProposal(events []StoredEvent, proposalID string) []StoredEvent {
	merges := make([]StoredEvent, 0)
	for _, stored := range events {
		if stored.Event.Kind == "proposal.merged" && stored.Event.Subject == proposalID {
			merges = append(merges, stored)
		}
	}
	return merges
}

func TestBubblewrapDiscoveryIgnoresHostilePATH(t *testing.T) {
	hostileDirectory := t.TempDir()
	hostile := filepath.Join(hostileDirectory, "bwrap")
	if err := os.WriteFile(hostile, []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", hostileDirectory)
	backend := newBubblewrapBackend()
	if backend.binary == hostile {
		t.Fatalf("sandbox guard resolved through hostile PATH: %s", backend.binary)
	}
	if strings.Contains(sandboxPath(), hostileDirectory) {
		t.Fatalf("sandbox PATH contains hostile directory: %s", sandboxPath())
	}
	if backend.binary != "" && !filepath.IsAbs(backend.binary) {
		t.Fatalf("sandbox guard path is not absolute: %s", backend.binary)
	}
}

func TestShallowRecoverySelectionErrorHasCurrentSemantics(t *testing.T) {
	message := errShallowRecoverySelectionRequired.Error()
	if strings.Contains(message, "WP05") || strings.Contains(message, "not available") {
		t.Fatalf("stale shallow recovery sentinel: %q", message)
	}
}

func TestMergeRetryMaintenanceRepairsOnlyConfirmedTarget(t *testing.T) {
	fixture := setupMergeRepairFixture(t)
	branch := mustGitText(t, "branch", "--show-current")
	base := mustGitText(t, "rev-parse", "HEAD")
	merged := crashAfterCodeMerge(t, fixture)
	run := func(tip string) (any, error) {
		return maintainProposal([]string{fixture.proposal.ID, "--action=merge", "--actor=" + fixture.maintainer.Actor,
			"--branch=" + branch, "--commit=" + tip, "--json"})
	}
	if _, err := run(merged); err == nil {
		t.Fatal("repair accepted an unrelated confirmed tip")
	}
	output, err := captureTestOutput(t, func() error {
		_, err := run(base)
		return err
	})
	if err != nil || !strings.Contains(output, merged) {
		t.Fatalf("repair output %q, error %v", output, err)
	}
	retry, err := run(base)
	if err != nil || retry == nil {
		t.Fatalf("retry receipt %v, error %v", retry, err)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if count := len(mergeEventsForProposal(events, fixture.proposal.ID)); count != 1 {
		t.Fatalf("expected one merge event, got %d", count)
	}
}
