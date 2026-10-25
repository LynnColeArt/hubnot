package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func protectionPolicy(maintainer, reviewer string) PolicyDocument {
	return PolicyDocument{Version: policyVersion, Maintainers: []string{maintainer}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, ProtectedChecks: &ProtectedChecksPolicy{Paths: []string{"lint.conf", "scripts/"}, TrustedReviewers: []string{reviewer}, RequiredApprovals: 1}}
}

func TestProtectedPolicyIndependentOptInsAndValidation(t *testing.T) {
	base := protectionPolicy(policyActorA, policyActorB)
	base.ProtectedChecks = nil
	encoded, _ := json.Marshal(base)
	parsed, _, err := parsePolicyBytes(encoded)
	if err != nil || parsed.ProtectedChecks != nil || parsed.Automerge != nil {
		t.Fatalf("legacy optionality: %+v %v", parsed, err)
	}
	for _, change := range compareOptionalPolicies(base, base) {
		if strings.Contains(change.Value, "added (0)") {
			t.Fatalf("disabled options reported as newly configured: %+v", change)
		}
	}
	base.Automerge = &AutomergePolicy{Rules: []AutomergeRule{{Name: "docs", TargetBranch: "main", Actors: []string{policyActorA}, Authors: []string{policyActorC}, Paths: []string{"docs/"}}}}
	encoded, _ = json.Marshal(base)
	parsed, _, err = parsePolicyBytes(encoded)
	if err != nil || parsed.ProtectedChecks != nil || parsed.Automerge == nil {
		t.Fatalf("independent auto: %+v %v", parsed, err)
	}
	base.ProtectedChecks = protectionPolicy(policyActorA, policyActorB).ProtectedChecks
	if err := validatePolicy(base); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"", "/etc/lint", "../lint", "a/../lint", "a//b", "a\\b", "./lint", "a/*", "a?[x]", "a\nb", ".", "a/.git/config", ".git/config"} {
		t.Run(p, func(t *testing.T) {
			pcy := protectionPolicy(policyActorA, policyActorB)
			pcy.ProtectedChecks.Paths = []string{p}
			if err := validatePolicy(pcy); err == nil {
				t.Fatalf("accepted unsafe literal %q", p)
			}
		})
	}
	for _, edit := range []func(*PolicyDocument){
		func(p *PolicyDocument) { p.ProtectedChecks.RequiredApprovals = 0 },
		func(p *PolicyDocument) { p.ProtectedChecks.RequiredApprovals = 2 },
		func(p *PolicyDocument) { p.ProtectedChecks.TrustedReviewers = []string{policyActorB, policyActorB} },
		func(p *PolicyDocument) { p.ProtectedChecks.Paths = make([]string, 65) },
		func(p *PolicyDocument) { p.ProtectedChecks.Paths = []string{strings.Repeat("a", 4097)} },
		func(p *PolicyDocument) {
			p.Automerge = &AutomergePolicy{Rules: []AutomergeRule{{Name: "docs", TargetBranch: "main", Actors: []string{policyActorC}, Authors: []string{policyActorB}, Paths: []string{"docs/"}}}}
		},
		func(p *PolicyDocument) {
			p.Automerge = &AutomergePolicy{Rules: []AutomergeRule{{Name: "docs", TargetBranch: "../main", Actors: []string{policyActorA}, Authors: []string{policyActorB}, Paths: []string{"docs/"}}}}
		},
	} {
		pcy := protectionPolicy(policyActorA, policyActorB)
		edit(&pcy)
		if err := validatePolicy(pcy); err == nil {
			t.Fatal("accepted malformed optional policy")
		}
	}
}

func protectionFixture(t *testing.T, enabled bool) (string, *Identity, *Identity, *Identity, *StoredEvent) {
	t.Helper()
	root := inIdentityTestRepository(t)
	maintainer, _, err := createIdentity("Maintainer")
	if err != nil {
		t.Fatal(err)
	}
	author, reviewer := testIdentity(t, "Agent"), testIdentity(t, "Designated reviewer")
	policy := protectionPolicy(maintainer.Actor, reviewer.Actor)
	policy.ProtectedChecks.TrustedReviewers = append(policy.ProtectedChecks.TrustedReviewers, author.Actor)
	if !enabled {
		policy.ProtectedChecks = nil
	}
	writeTestPolicy(t, root, policy)
	if err := os.WriteFile("lint.conf", []byte("strict=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn/policy.json", "lint.conf")
	mustGit(t, "commit", "-q", "-m", "policy and lint")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "-c", "candidate")
	if err := os.WriteFile("lint.conf", []byte("strict=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "lint.conf")
	mustGit(t, "commit", "-q", "-m", "weaken lint")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")
	e := newEvent(author, "proposal.open", 1, "")
	e.Title = "Change lint"
	e.Base = base
	e.Head = head
	proposal, err := appendEvent(e, author)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	return root, maintainer, author, reviewer, proposal
}

func TestProtectedAcceptanceEvidenceAndWithdrawal(t *testing.T) {
	_, maintainer, author, reviewer, proposal := protectionFixture(t, true)
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	ev, err := evaluateProposal(proposal, events)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Ready || !reflect.DeepEqual(ev.ProtectedChecks.ChangedPaths, []string{"lint.conf"}) {
		t.Fatalf("unreviewed protected change: %+v", ev)
	}
	// Ordinary approval and even author approval cannot supply designated trust.
	appendRevisionReview(t, author, proposal.ID)
	ordinary := testIdentity(t, "Ordinary reviewer")
	appendRevisionReview(t, ordinary, proposal.ID)
	if err := cmdDecide([]string{proposal.ID, "--accept"}); err == nil {
		t.Fatal("accepted protected change without designated approval")
	}
	review := appendRevisionReview(t, reviewer, proposal.ID)
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	ev, err = evaluateProposal(proposal, events)
	if err != nil || !ev.Ready {
		t.Fatalf("designated approval not counted: %+v %v", ev, err)
	}
	if !stringListed(review.ID, ev.Evidence) {
		t.Fatalf("designated evidence absent: %v", ev.Evidence)
	}
	if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
		t.Fatal(err)
	}
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]StoredEvent{}
	for _, e := range events {
		byID[e.ID] = e
	}
	var decision StoredEvent
	for _, e := range events {
		if e.Event.Kind == "proposal.decision" {
			decision = e
		}
	}
	if !stringListed(review.ID, decision.Event.Evidence) {
		t.Fatal("acceptance omitted designated evidence")
	}
	gitDir, _ := requireGitRepository()
	forged := decision
	forged.Event.Evidence = nil
	if err := validateDecisionEvent(forged, *proposal, byID); err == nil {
		t.Fatal("local admission accepted missing protected evidence")
	}
	if err := validateReplicationDecisionAt(gitDir, gitDir, forged, *proposal, byID); err == nil {
		t.Fatal("replication admitted missing protected evidence")
	}
	// Historical exact evidence stays valid after a current withdrawal, but no new merge may use it.
	withdrawal, err := nextEvent(reviewer, "review.submit")
	if err != nil {
		t.Fatal(err)
	}
	withdrawal.Subject = proposal.ID
	withdrawal.Verdict = "request-changes"
	if _, err := appendEvent(withdrawal, reviewer); err != nil {
		t.Fatal(err)
	}
	events, err = collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDecisionEvent(decision, *proposal, byID); err != nil {
		t.Fatalf("rewrote historical trust: %v", err)
	}
	before := mustGitText(t, "rev-parse", "HEAD")
	if err := cmdMerge([]string{proposal.ID}); err == nil {
		t.Fatal("merged after designated withdrawal")
	}
	if got := mustGitText(t, "rev-parse", "HEAD"); got != before {
		t.Fatal("ref advanced on protected refusal")
	}
	_ = maintainer
}

func TestProtectedLegacyManualMergeAndOldBaseGate(t *testing.T) {
	t.Run("disabled preserves legacy", func(t *testing.T) {
		_, _, _, _, proposal := protectionFixture(t, false)
		if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
			t.Fatal(err)
		}
		if err := cmdMerge([]string{proposal.ID}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("new old base proposal cannot bypass target", func(t *testing.T) {
		root, maintainer, _, reviewer, proposal := protectionFixture(t, false)
		if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
			t.Fatal(err)
		}
		writeTestPolicy(t, root, protectionPolicy(maintainer.Actor, reviewer.Actor))
		mustGit(t, "add", ".hn/policy.json")
		mustGit(t, "commit", "-q", "-m", "opt in")
		before := mustGitText(t, "rev-parse", "HEAD")
		if err := cmdMerge([]string{proposal.ID}); err == nil || !strings.Contains(err.Error(), "revise") {
			t.Fatalf("old base gate: %v", err)
		}
		if got := mustGitText(t, "rev-parse", "HEAD"); got != before {
			t.Fatal("stale candidate advanced target")
		}
	})
}

func TestProtectedLiteralTreeChanges(t *testing.T) {
	root := inIdentityTestRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("scripts/lint.sh", []byte("strict"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("other", []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "scripts", "other")
	mustGit(t, "commit", "-q", "-m", "base")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "mv", "scripts/lint.sh", "moved.sh")
	mustGit(t, "commit", "-q", "-m", "move")
	head := mustGitText(t, "rev-parse", "HEAD")
	changed, err := changedPathsAt("", base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(changed, []string{"moved.sh", "scripts/lint.sh"}) {
		t.Fatalf("rename bypass: %q", changed)
	}
	if !protectedPathMatches("scripts/", "scripts/lint.sh") || !protectedPathMatches("scripts/", "scripts") || protectedPathMatches("scripts/", "scripts-other/x") {
		t.Fatal("literal directory scope")
	}
	if !protectedPathMatches("scripts/lint.sh", "scripts") {
		t.Fatal("ancestor file replacement was not protected")
	}
	// NUL delimiters keep hostile Git names intact.
	if err := os.WriteFile("scripts\nname", []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "scripts\nname")
	mustGit(t, "commit", "-q", "-m", "odd path")
	changed, err = changedPathsAt("", head, mustGitText(t, "rev-parse", "HEAD"))
	if err != nil || !reflect.DeepEqual(changed, []string{"scripts\nname"}) {
		t.Fatalf("NUL paths: %q %v", changed, err)
	}
}

func TestProtectedRequiredPipelineAndPolicyAmendment(t *testing.T) {
	t.Run("required pipeline no-op needs designated review", func(t *testing.T) {
		root := inIdentityTestRepository(t)
		maintainer, _, err := createIdentity("Maintainer")
		if err != nil {
			t.Fatal(err)
		}
		author, reviewer, runner := testIdentity(t, "Agent"), testIdentity(t, "Designated"), testIdentity(t, "Runner")
		policy := protectionPolicy(maintainer.Actor, reviewer.Actor)
		policy.ProtectedChecks.Paths = []string{}
		policy.Pipelines = map[string]PipelinePolicy{"lint": {RequiredResults: 1, TrustedRunners: []string{runner.Actor}}}
		writeTestPolicy(t, root, policy)
		if err := os.MkdirAll(".hn/pipelines", 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(".hn/pipelines/lint.json", []byte(`{"version":"hn.pipeline/0","steps":[{"name":"Lint","command":"lint"}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		mustGit(t, "add", ".hn")
		mustGit(t, "commit", "-q", "-m", "protected pipeline")
		base := mustGitText(t, "rev-parse", "HEAD")
		if err := os.WriteFile(".hn/pipelines/lint.json", []byte(`{"version":"hn.pipeline/0","steps":[{"name":"No-op","command":"true"}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		mustGit(t, "add", ".hn")
		mustGit(t, "commit", "-q", "-m", "weaken pipeline")
		head := mustGitText(t, "rev-parse", "HEAD")
		e := newEvent(author, "proposal.open", 1, "")
		e.Title = "No-op lint"
		e.Base = base
		e.Head = head
		proposal, err := appendEvent(e, author)
		if err != nil {
			t.Fatal(err)
		}
		if err := createProposalRef(proposal.ID, head); err != nil {
			t.Fatal(err)
		}
		_, _, definition, err := loadPipeline(head, "lint")
		if err != nil {
			t.Fatal(err)
		}
		req, err := nextEvent(maintainer, "run.request")
		if err != nil {
			t.Fatal(err)
		}
		req.Subject = proposal.ID
		req.Pipeline = "lint"
		req.Definition = definition
		req.Commit = head
		request, err := appendEvent(req, maintainer)
		if err != nil {
			t.Fatal(err)
		}
		appendRevisionRunResult(t, runner, request)
		events, err := collectEvents()
		if err != nil {
			t.Fatal(err)
		}
		evaluation, err := evaluateProposal(proposal, events)
		if err != nil {
			t.Fatal(err)
		}
		if evaluation.Ready || !reflect.DeepEqual(evaluation.ProtectedChecks.ChangedPaths, []string{".hn/pipelines/lint.json"}) {
			t.Fatalf("passing no-op bypassed protection: %+v", evaluation)
		}
		if err := cmdDecide([]string{proposal.ID, "--accept"}); err == nil {
			t.Fatal("accepted passing no-op lint")
		}
		appendRevisionReview(t, reviewer, proposal.ID)
		if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("candidate cannot remove its own protection", func(t *testing.T) {
		root, maintainer, _, _, proposal := protectionFixture(t, true)
		mustGit(t, "switch", "-q", "candidate")
		p := protectionPolicy(maintainer.Actor, policyActorB)
		p.ProtectedChecks = nil
		writeTestPolicy(t, root, p)
		mustGit(t, "add", ".hn/policy.json")
		mustGit(t, "commit", "-q", "-m", "remove protection")
		head := mustGitText(t, "rev-parse", "HEAD")
		author := testIdentity(t, "Policy author")
		e := newEvent(author, "proposal.open", 1, "")
		e.Title = "Remove gate"
		e.Base = proposal.Event.Base
		e.Head = head
		next, err := appendEvent(e, author)
		if err != nil {
			t.Fatal(err)
		}
		if err := createProposalRef(next.ID, head); err != nil {
			t.Fatal(err)
		}
		mustGit(t, "switch", "-q", "main")
		events, err := collectEvents()
		if err != nil {
			t.Fatal(err)
		}
		ev, err := evaluateProposal(next, events)
		if err != nil {
			t.Fatal(err)
		}
		if ev.Ready || !stringListed(".hn/policy.json", ev.ProtectedChecks.ChangedPaths) {
			t.Fatalf("candidate removed own policy gate: %+v", ev)
		}
	})
}

func TestProtectedModeAndAncestorChanges(t *testing.T) {
	inIdentityTestRepository(t)
	if err := os.MkdirAll("config/lint", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config/lint/settings", []byte("strict"), 0600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "config")
	mustGit(t, "commit", "-q", "-m", "base")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "update-index", "--chmod=+x", "config/lint/settings")
	mustGit(t, "commit", "-q", "-m", "mode")
	mode := mustGitText(t, "rev-parse", "HEAD")
	changed, err := changedPathsAt("", base, mode)
	if err != nil || !reflect.DeepEqual(changed, []string{"config/lint/settings"}) {
		t.Fatalf("mode invisible: %q %v", changed, err)
	}
	if err := os.RemoveAll("config"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config", []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "config")
	mustGit(t, "commit", "-q", "-m", "replace ancestor")
	changed, err = changedPathsAt("", mode, mustGitText(t, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	p := protectionPolicy(policyActorA, policyActorB)
	p.ProtectedChecks.Paths = []string{"config/lint/settings"}
	if paths := changedProtectedPaths(p, changed); !stringListed("config", paths) {
		t.Fatalf("ancestor replacement bypassed protection: %q", paths)
	}
}

func TestProtectedTargetCaseInsensitiveOptInAndInspection(t *testing.T) {
	root, maintainer, _, reviewer, proposal := protectionFixture(t, false)
	if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
		t.Fatal(err)
	}
	policy := protectionPolicy(maintainer.Actor, reviewer.Actor)
	encoded, _ := json.Marshal(policy)
	encoded = []byte(strings.Replace(string(encoded), `"protectedChecks"`, `"PROTECTEDCHECKS"`, 1))
	commitPolicyBytes(t, root, encoded, "alternate-case opt-in")
	if err := cmdMerge([]string{proposal.ID}); err == nil || !strings.Contains(err.Error(), "revise") {
		t.Fatalf("alternate-case gate bypass: %v", err)
	}
	show, err := captureTestOutput(t, func() error { return cmdPolicyShow(nil) })
	if err != nil {
		t.Fatal(err)
	}
	assertPolicyOutputContains(t, show, "Protected checks: enabled", reviewer.Actor, "lint.conf", ".hn/policy.json", "Automerge: disabled")
	check, err := captureTestOutput(t, func() error { return cmdPolicyCheck([]string{"--base", proposal.Event.Base, "--head", "HEAD"}) })
	if err != nil {
		t.Fatal(err)
	}
	assertPolicyOutputContains(t, check, "protected checks enabled: changed (false -> true)", "protected paths added:", "lint.conf", reviewer.Actor)
}

func TestProtectedChangedPathsOutputBound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell wrapper test")
	}
	fake := t.TempDir()
	script := filepath.Join(fake, "git")
	for _, test := range []struct{ name, body string }{
		{"stdout", "#!/bin/sh\nexec head -c 8388609 /dev/zero\n"},
		{"stderr", "#!/bin/sh\nhead -c 65537 /dev/zero >&2\nexit 1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(script, []byte(test.body), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
			if _, err := changedPathsAt("", strings.Repeat("1", 40), strings.Repeat("2", 40)); err == nil || !strings.Contains(err.Error(), "bound") {
				t.Fatalf("oversized Git output not refused: %v", err)
			}
		})
	}
}

func TestProtectedAcceptedProposalReplicatesAndMerges(t *testing.T) {
	root, _, _, reviewer, proposal := protectionFixture(t, true)
	appendRevisionReview(t, reviewer, proposal.ID)
	if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
		t.Fatal(err)
	}
	remote := filepath.Join(t.TempDir(), "project.git")
	replica := filepath.Join(t.TempDir(), "replica")
	mustGit(t, "init", "--bare", "-q", "-b", "main", remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", "main")
	if err := cmdSync(nil); err != nil {
		t.Fatal(err)
	}
	// Force pack negotiation: local clone hardlinks can make unadvertised proposal
	// objects appear present before sync and hide isolation/fallback defects.
	mustGit(t, "clone", "-q", "--no-local", remote, replica)
	mustGit(t, "-C", replica, "cat-file", "-e", proposal.Event.Base)
	if _, err := gitOutput("-C", replica, "cat-file", "-e", proposal.Event.Head); err == nil {
		t.Fatal("fixture leaked proposal code before sync")
	}
	if err := os.Chdir(replica); err != nil {
		t.Fatal(err)
	}
	if err := cmdSync(nil); err != nil {
		t.Fatal(err)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	fetched, err := resolveFullEvent(events, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := evaluateProposal(fetched, events)
	if err != nil {
		t.Fatal(err)
	}
	if !evaluation.Ready || !evaluation.Accepted || len(evaluation.ProtectedChecks.Approvals) != 1 {
		t.Fatalf("protected proposal lost admissible evidence in replica: %+v", evaluation)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	if err := cmdMerge([]string{proposal.ID}); err != nil {
		t.Fatal(err)
	}
	parents := strings.Fields(mustGitText(t, "show", "-s", "--format=%P", "HEAD"))
	if len(parents) != 2 || parents[1] != proposal.Event.Head {
		t.Fatalf("native merge parent binding: %q", parents)
	}
}

func TestProtectedTargetBaselineRequiresRevision(t *testing.T) {
	for _, test := range []struct {
		name                string
		targetChange, block bool
	}{
		{name: "protected target tightening hidden by old-base reversion", targetChange: true, block: true},
		{name: "unprotected target advancement remains eligible", targetChange: true},
		{name: "target exactly signed base remains eligible"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, maintainer, author, _, old := protectionFixture(t, true)
			base := old.Event.Base
			original, err := os.ReadFile("lint.conf")
			if err != nil {
				t.Fatal(err)
			}
			if test.targetChange {
				targetPath, targetBody := "notes.txt", []byte("Target documentation\n")
				if test.block {
					targetPath, targetBody = "lint.conf", []byte("strict=true\nadditional=true\n")
				}
				if err := os.WriteFile(targetPath, targetBody, 0600); err != nil {
					t.Fatal(err)
				}
				mustGit(t, "add", targetPath)
				mustGit(t, "commit", "-q", "-m", "advance target without policy change")
			}
			target := mustGitText(t, "rev-parse", "HEAD")
			basePolicy := mustGitText(t, "show", base+":.hn/policy.json")
			if got := mustGitText(t, "show", target+":.hn/policy.json"); got != basePolicy {
				t.Fatal("fixture changed policy instead of protected baseline")
			}
			mustGit(t, "switch", "-q", "-c", "stale-base-candidate")
			if err := os.WriteFile("lint.conf", original, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("docs.txt", []byte("Candidate documentation\n"), 0600); err != nil {
				t.Fatal(err)
			}
			mustGit(t, "add", "lint.conf", "docs.txt")
			mustGit(t, "commit", "-q", "-m", "candidate based on old policy baseline")
			head := mustGitText(t, "rev-parse", "HEAD")
			mustGit(t, "switch", "-q", "main")
			event, err := nextEvent(author, "proposal.open")
			if err != nil {
				t.Fatal(err)
			}
			event.Title = "Old base candidate"
			event.Base = base
			event.Head = head
			proposal, err := appendEvent(event, author)
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
			if !evaluation.Ready || len(evaluation.ProtectedChecks.ChangedPaths) != 0 || len(evaluation.ProtectedChecks.Approvals) != 0 {
				t.Fatalf("fixture requires an unprotected signed diff with zero designated reviews: %+v", evaluation)
			}
			if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
				t.Fatal(err)
			}
			actorBefore := mustGitText(t, "rev-parse", operationalActorRef(maintainer.Actor))
			mergeErr := cmdMerge([]string{proposal.ID})
			if test.block {
				if mergeErr == nil || !strings.Contains(mergeErr.Error(), "revise") {
					t.Fatalf("hidden protected baseline reversion was not refused: %v", mergeErr)
				}
				if got := mustGitText(t, "rev-parse", "HEAD"); got != target {
					t.Fatal("target advanced on protected baseline refusal")
				}
				if got := mustGitText(t, "rev-parse", operationalActorRef(maintainer.Actor)); got != actorBefore {
					t.Fatal("refusal appended a merge event")
				}
				actual, err := os.ReadFile("lint.conf")
				if err != nil {
					t.Fatal(err)
				}
				if string(actual) != "strict=true\nadditional=true\n" {
					t.Fatalf("target lint tightening was reverted: %q", actual)
				}
			} else {
				if mergeErr != nil {
					t.Fatalf("eligible baseline movement was overblocked: %v", mergeErr)
				}
				parents := strings.Fields(mustGitText(t, "show", "-s", "--format=%P", "HEAD"))
				if !reflect.DeepEqual(parents, []string{target, head}) {
					t.Fatalf("wrong eligible merge parents: %q", parents)
				}
			}
		})
	}
}
