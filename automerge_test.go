package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

type automergeTestRepo struct {
	root                           string
	actor, author, reviewer, other *Identity
	proposal                       *StoredEvent
	policy                         PolicyDocument
}

func newAutomergeTestRepo(t *testing.T, edit func(*PolicyDocument), changes map[string]string) automergeTestRepo {
	t.Helper()
	root := inIdentityTestRepository(t)
	actor, _, err := createIdentity("Merge maintainer")
	if err != nil {
		t.Fatal(err)
	}
	author, reviewer, other := testIdentity(t, "Author"), testIdentity(t, "Reviewer"), testIdentity(t, "Other maintainer")
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor.Actor, other.Actor}, Proposals: ProposalPolicy{RequiredApprovals: 1, RequiredAccepts: 1, TrustedReviewers: []string{reviewer.Actor}}, Pipelines: map[string]PipelinePolicy{}, Automerge: &AutomergePolicy{Rules: []AutomergeRule{{Name: "docs", TargetBranch: "main", Actors: []string{actor.Actor}, Authors: []string{author.Actor}, Paths: []string{"docs/"}}}}}
	if edit != nil {
		edit(&policy)
	}
	writeTestPolicy(t, root, policy)
	for p, content := range map[string]string{".gitignore": "cache/\ndocs/ignored.md\n", "docs/existing.md": "existing\n"} {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name := range policy.Pipelines {
		if err := os.MkdirAll(".hn/pipelines", 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(".hn/pipelines/"+name+".json", []byte(`{"version":"hn.pipeline/0","steps":[{"name":"Check","command":"true"}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		mustGit(t, "add", ".hn/pipelines/"+name+".json")
	}
	mustGit(t, "add", ".hn/policy.json", ".gitignore", "docs/existing.md")
	mustGit(t, "commit", "-qm", "native policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-qc", "candidate")
	for p, content := range changes {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		mustGit(t, "add", "-f", "--", p)
	}
	mustGit(t, "commit", "--allow-empty", "-qm", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")
	event := newEvent(author, "proposal.open", 1, "")
	event.Title = "Documentation"
	event.Base = base
	event.Head = head
	proposal, err := appendEvent(event, author)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	appendRevisionReview(t, reviewer, proposal.ID)
	if len(policy.Pipelines) == 0 {
		if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { automergeProcessHook = nil; automergeRefAckTimeout = 30 * time.Second })
	return automergeTestRepo{root, actor, author, reviewer, other, proposal, policy}
}

func autoSnapshot(t *testing.T) (string, []byte, string) {
	t.Helper()
	head := mustGitText(t, "rev-parse", "HEAD")
	indexPath := mustGitText(t, "rev-parse", "--git-path", "index")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	refs := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)")
	return head, index, refs
}

func TestAutomergePreviewKeylessAndExplicitNativeMerge(t *testing.T) {
	f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "hello\n"})
	// Force a stat refresh opportunity; preview must not rewrite the index.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes("docs/existing.md", future, future); err != nil {
		t.Fatal(err)
	}
	before, index, refs := autoSnapshot(t)
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepathForIdentityRecord(paths, f.actor.Actor)
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	out, err := captureTestOutput(t, func() error { return run([]string{"automerge", f.proposal.ID, "--rule", "docs"}) })
	if err != nil || !strings.Contains(out, "Eligible") || !strings.Contains(out, f.proposal.ID) || !strings.Contains(out, f.proposal.Event.Base) {
		t.Fatalf("preview: %s %v", out, err)
	}
	after, afterIndex, afterRefs := autoSnapshot(t)
	if before != after || !bytes.Equal(index, afterIndex) || refs != afterRefs {
		t.Fatal("preview mutated ref/index state")
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("preview recreated signing key: %v", err)
	}
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err != nil {
		t.Fatal(err)
	}
	merged := mustGitText(t, "rev-parse", "HEAD")
	if merged == before {
		t.Fatal("execution did not advance target")
	}
	parents := strings.Fields(mustGitText(t, "show", "-s", "--format=%P", merged))
	if !reflect.DeepEqual(parents, []string{f.proposal.Event.Base, f.proposal.Event.Head}) {
		t.Fatalf("parents=%v", parents)
	}
	if got := mustGitText(t, "status", "--porcelain"); got != "" {
		t.Fatalf("dirty after merge: %s", got)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, e := range events {
		if e.Event.Kind == "proposal.merged" {
			count++
			if e.Event.Commit != merged || e.Event.Actor != f.actor.Actor || e.Event.Subject != f.proposal.ID || len(e.Event.Evidence) != 1 {
				t.Fatalf("bad merge fact: %+v", e.Event)
			}
		}
	}
	if count != 1 {
		t.Fatalf("merge facts=%d", count)
	}
	if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
		t.Fatal("repeat merged again")
	}
}

func TestAutomergeDisabledAndRuleBlockers(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		edit       func(*PolicyDocument)
		changes    map[string]string
	}{
		{"disabled", "disabled", func(p *PolicyDocument) { p.Automerge = nil }, map[string]string{"docs/safe.md": "x"}},
		{"author", "author", func(p *PolicyDocument) { p.Automerge.Rules[0].Authors = []string{p.Maintainers[0]} }, map[string]string{"docs/safe.md": "x"}},
		{"outside path", "allow", nil, map[string]string{"outside": "x"}},
		{"empty diff", "no changed", nil, nil},
		{"protected", "protected", func(p *PolicyDocument) {
			p.ProtectedChecks = &ProtectedChecksPolicy{Paths: []string{"docs/"}, TrustedReviewers: p.Proposals.TrustedReviewers, RequiredApprovals: 1}
		}, map[string]string{"docs/safe.md": "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAutomergeTestRepo(t, tc.edit, tc.changes)
			before, index, refs := autoSnapshot(t)
			out, err := captureTestOutput(t, func() error { return cmdAutomerge([]string{f.proposal.ID, "--rule", "docs"}) })
			if err == nil || !strings.Contains(strings.ToLower(out+err.Error()), tc.want) {
				t.Fatalf("blocker: %s %v", out, err)
			}
			after, newIndex, newRefs := autoSnapshot(t)
			if before != after || refs != newRefs || !bytes.Equal(index, newIndex) {
				t.Fatal("refusal mutated state")
			}
		})
	}
}

func TestAutomergeWithdrawnEvidenceAndUnapprovedRule(t *testing.T) {
	t.Run("withdraw review", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
		e, _ := nextEvent(f.reviewer, "review.submit")
		e.Subject = f.proposal.ID
		e.Verdict = "request-changes"
		if _, err := appendEvent(e, f.reviewer); err != nil {
			t.Fatal(err)
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("withdrawn approval merged")
		}
	})
	t.Run("wrong rule", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "other"}); err == nil {
			t.Fatal("unknown rule authorized")
		}
	})
	t.Run("non rule maintainer", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
		writeActiveTestIdentity(t, f.other)
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("unlisted maintainer merged")
		}
	})
	t.Run("signing identity changes", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
		automergeProcessHook = func(step string) error {
			if step == "before-native-merge" {
				writeActiveTestIdentity(t, f.other)
			}
			return nil
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil || !strings.Contains(err.Error(), "identity") {
			t.Fatalf("signer race: %v", err)
		}
		if got := mustGitText(t, "rev-parse", "HEAD"); got != f.proposal.Event.Base {
			t.Fatal("wrong signer advanced ref")
		}
	})
}

func TestAutomergeExpectedTargetLeaseAndCurrentEvidence(t *testing.T) {
	for _, step := range []string{"before-native-merge", "before-prepare"} {
		t.Run(step, func(t *testing.T) {
			f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
			moved := ""
			automergeProcessHook = func(actual string) error {
				if actual == step {
					tree := mustGitText(t, "rev-parse", f.proposal.Event.Base+"^{tree}")
					raw, err := gitInput([]byte("concurrent target\n"), nil, "commit-tree", tree, "-p", f.proposal.Event.Base)
					if err != nil {
						return err
					}
					moved = strings.TrimSpace(string(raw))
					mustGit(t, "update-ref", "refs/heads/main", moved, f.proposal.Event.Base)
				}
				return nil
			}
			if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
				t.Fatal("advanced stale target")
			}
			if got := mustGitText(t, "rev-parse", "HEAD"); got != moved {
				t.Fatalf("rewound concurrent target: %s != %s", got, moved)
			}
			if _, err := os.Stat("docs/safe.md"); !os.IsNotExist(err) {
				t.Fatal("lease refusal changed checkout")
			}
		})
	}
	t.Run("withdraw under lease", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
		automergeProcessHook = func(step string) error {
			if step == "lease-prepared" {
				e, _ := nextEvent(f.reviewer, "review.submit")
				e.Subject = f.proposal.ID
				e.Verdict = "request-changes"
				_, err := appendEvent(e, f.reviewer)
				return err
			}
			return nil
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("late withdrawal merged")
		}
		if got := mustGitText(t, "rev-parse", "HEAD"); got != f.proposal.Event.Base {
			t.Fatal("withdrawal advanced target")
		}
		requireAutomergeNoLocks(t)
	})
}

func TestAutomergeWorkspacePreservationAndManualRepair(t *testing.T) {
	t.Run("ignored collision", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/ignored.md": "candidate"})
		if err := os.WriteFile("docs/ignored.md", []byte("user ignored"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("ignored collision overwritten")
		}
		raw, _ := os.ReadFile("docs/ignored.md")
		if string(raw) != "user ignored" {
			t.Fatal("user content lost")
		}
		if got := mustGitText(t, "rev-parse", "HEAD"); got != f.proposal.Event.Base {
			t.Fatal("collision advanced target")
		}
	})
	t.Run("unrelated ignored cache", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "candidate"})
		os.MkdirAll("cache", 0700)
		os.WriteFile("cache/user", []byte("keep"), 0600)
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile("cache/user")
		if string(raw) != "keep" {
			t.Fatal("unrelated ignored file lost")
		}
	})
	t.Run("late dirty tracked", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/existing.md": "candidate"})
		automergeProcessHook = func(step string) error {
			if step == "lease-prepared" {
				return os.WriteFile("docs/existing.md", []byte("user edit"), 0600)
			}
			return nil
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("late dirty file overwritten")
		}
		raw, _ := os.ReadFile("docs/existing.md")
		if string(raw) != "user edit" {
			t.Fatal("dirty file lost")
		}
	})
	t.Run("post commit missing event repair", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "candidate"})
		automergeProcessHook = func(step string) error {
			if step == "merge-committed" {
				return os.ErrPermission
			}
			return nil
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("injected event boundary did not fail")
		}
		merged := mustGitText(t, "rev-parse", "HEAD")
		if merged == f.proposal.Event.Base {
			t.Fatal("commit not created")
		}
		automergeProcessHook = nil
		if err := cmdMerge([]string{f.proposal.ID}); err != nil {
			t.Fatal(err)
		}
		if got := mustGitText(t, "rev-parse", "HEAD"); got != merged {
			t.Fatal("repair created another commit")
		}
	})
}

func TestAutomergeAllowlistDoesNotAuthorizeAncestorReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink requires extra Windows privilege")
	}
	f := newAutomergeTestRepo(t, func(p *PolicyDocument) { p.Automerge.Rules[0].Paths = []string{"docs/existing.md"} }, map[string]string{"docs/existing.md": "x"})
	// Replace this candidate with a fresh candidate containing a directory symlink.
	mustGit(t, "switch", "-q", "candidate")
	mustGit(t, "rm", "-r", "docs")
	if err := os.Symlink("elsewhere", "docs"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "docs")
	mustGit(t, "commit", "-qm", "replace ancestor")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")
	e, _ := nextEvent(f.author, "proposal.open")
	e.Title = "Ancestor replacement"
	e.Base = f.proposal.Event.Base
	e.Head = head
	p, err := appendEvent(e, f.author)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(p.ID, head); err != nil {
		t.Fatal(err)
	}
	appendRevisionReview(t, f.reviewer, p.ID)
	if err := cmdDecide([]string{p.ID, "--accept"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdAutomerge([]string{p.ID, "--rule", "docs", "--execute"}); err == nil {
		t.Fatal("descendant allowlist authorized ancestor change")
	}
}

func TestAutomergeRefusesPromisorBeforeObjectReads(t *testing.T) {
	f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
	mustGit(t, "config", "remote.origin.promisor", "true")
	out, err := captureTestOutput(t, func() error { return cmdAutomerge([]string{f.proposal.ID, "--rule", "docs"}) })
	if err == nil || !strings.Contains(strings.ToLower(out+err.Error()), "promisor") {
		t.Fatalf("promisor refusal: %s %v", out, err)
	}
}

func requireAutomergeNoLocks(t *testing.T) {
	t.Helper()
	for _, name := range []string{"HEAD.lock", "refs/heads/main.lock", "index.lock"} {
		p := mustGitText(t, "rev-parse", "--git-path", name)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("refusal left lock %s: %v", p, err)
		}
	}
}

func TestAutomergeFullBranchIdentityAndQuotedPaths(t *testing.T) {
	t.Run("same named tag cannot grant wrong branch", func(t *testing.T) {
		f := newAutomergeTestRepo(t, func(p *PolicyDocument) { p.Automerge.Rules[0].TargetBranch = "heads/main" }, map[string]string{"docs/safe.md": "x"})
		mustGit(t, "tag", "main", f.proposal.Event.Base)
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("ambiguous short ref authorized wrong target")
		}
		if got := mustGitText(t, "rev-parse", "refs/heads/main"); got != f.proposal.Event.Base {
			t.Fatal("wrong target advanced")
		}
	})
	t.Run("same named tag retains exact main lease", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
		mustGit(t, "tag", "main", f.proposal.Event.Base)
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err != nil {
			t.Fatal(err)
		}
		if got := mustGitText(t, "rev-parse", "refs/heads/main"); got == f.proposal.Event.Base {
			t.Fatal("main did not advance")
		}
		if _, err := gitOutput("show-ref", "--verify", "refs/heads/heads/main"); err == nil {
			t.Fatal("created shadow target branch")
		}
	})
	t.Run("newline path is quoted", func(t *testing.T) {
		name := "docs/file\nEligible: injected.md"
		f := newAutomergeTestRepo(t, nil, map[string]string{name: "x"})
		out, err := captureTestOutput(t, func() error { return cmdAutomerge([]string{f.proposal.ID, "--rule", "docs"}) })
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, `docs/file\nEligible: injected.md`) || strings.Contains(out, "\nEligible: injected.md") {
			t.Fatalf("path forged preview line: %q", out)
		}
	})
}

func TestAutomergeCurrentFailureAndRejectionVeto(t *testing.T) {
	t.Run("failure despite passing quorum", func(t *testing.T) {
		f := newAutomergeTestRepo(t, func(p *PolicyDocument) {
			p.Pipelines["check"] = PipelinePolicy{RequiredResults: 1, TrustedRunners: []string{p.Proposals.TrustedReviewers[0], p.Maintainers[1]}}
		}, map[string]string{"docs/safe.md": "x"})
		if err := cmdRunRequest([]string{f.proposal.ID, "check"}); err != nil {
			t.Fatal(err)
		}
		events, err := collectEvents()
		if err != nil {
			t.Fatal(err)
		}
		requests := runRequests(events)
		if len(requests) != 1 {
			t.Fatal("request absent")
		}
		request := requests[0]
		appendRevisionRunResult(t, f.reviewer, &request)
		log := []byte("failed\n")
		event, _ := nextEvent(f.other, "run.result")
		event.Subject = request.ID
		event.Pipeline = request.Event.Pipeline
		event.Definition = request.Event.Definition
		event.Commit = request.Event.Commit
		event.Outcome = "failed"
		event.ExitCode = 1
		event.Log = eventID(log)
		event.Backend = "host"
		event.Platform = "test/test"
		event.Runner = "hn/test"
		if _, err := appendEventWithAttachments(event, f.other, map[string][]byte{"log.txt": log}); err != nil {
			t.Fatal(err)
		}
		// Legacy requiredResults=1 remains satisfiable; automerge adds its own veto.
		if err := cmdDecide([]string{f.proposal.ID, "--accept"}); err != nil {
			t.Fatal(err)
		}
		out, err := captureTestOutput(t, func() error { return cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}) })
		if err == nil || !strings.Contains(out+err.Error(), "current trusted failure") {
			t.Fatalf("failure did not veto: %s %v", out, err)
		}
	})
	t.Run("maintainer rejection", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
		if err := cmdDecide([]string{f.proposal.ID, "--reject", "--body", "pause"}); err != nil {
			t.Fatal(err)
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("current rejection did not veto")
		}
	})
}

func TestAutomergeLateAcceptanceSnapshotAndLeaseCleanup(t *testing.T) {
	f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
	var second *StoredEvent
	automergeProcessHook = func(step string) error {
		if step == "lease-prepared" {
			events, err := collectEvents()
			if err != nil {
				return err
			}
			evaluation, err := evaluateProposal(f.proposal, events)
			if err != nil {
				return err
			}
			second = appendRevisionDecision(t, f.other, f.proposal.ID, evaluation.PolicyDigest, evaluation.Evidence)
		}
		return nil
	}
	if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err != nil {
		t.Fatal(err)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Event.Kind == "proposal.merged" && !stringListed(second.ID, e.Event.Evidence) {
			t.Fatalf("merge omitted acceptance checked under lease: %v", e.Event.Evidence)
		}
	}
	requireAutomergeNoLocks(t)
}

func TestAutomergeRefLeaseHasNoWholeOperationDeadline(t *testing.T) {
	f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
	automergeRefAckTimeout = time.Second
	txn, err := prepareAutomergeRef("refs/heads/main", f.proposal.Event.Head, f.proposal.Event.Base)
	if err != nil {
		t.Fatal(err)
	}
	defer txn.close()
	time.Sleep(1100 * time.Millisecond)
	if _, err := gitOutput("update-ref", "refs/heads/main", f.proposal.Event.Head, f.proposal.Event.Base); err == nil {
		t.Fatal("lease expired while caller was inspecting state")
	}
	txn.close()
	requireAutomergeNoLocks(t)
	if got := mustGitText(t, "rev-parse", "HEAD"); got != f.proposal.Event.Base {
		t.Fatal("aborted lease advanced target")
	}
}

func installAutomergeGitWrapper(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell test shim")
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HN_TEST_AUTO_REAL_GIT", real)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestAutomergePromisorAndShallowPreflightHaveNoObjectReads(t *testing.T) {
	for _, mode := range []string{"promisor", "shallow"} {
		t.Run(mode, func(t *testing.T) {
			f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
			if mode == "promisor" {
				mustGit(t, "config", "remote.origin.promisor", "true")
			} else {
				if err := os.WriteFile(filepath.Join(f.root, ".git", "shallow"), []byte(f.proposal.Event.Base+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			log := filepath.Join(t.TempDir(), "git-calls")
			t.Setenv("HN_TEST_AUTO_LOG", log)
			installAutomergeGitWrapper(t, "printf '%s\\n' \"$*\" >> \"$HN_TEST_AUTO_LOG\"\nexec \"$HN_TEST_AUTO_REAL_GIT\" \"$@\"\n")
			if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs"}); err == nil {
				t.Fatal("unsupported repository was not refused")
			}
			raw, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			calls := strings.Split(strings.TrimSpace(string(raw)), "\n")
			for _, call := range calls {
				if !strings.Contains(call, " config --null --get-regexp ") && !strings.Contains(call, " rev-parse --is-shallow-repository") {
					t.Fatalf("object reader before refusal: %s", call)
				}
			}
			if mode == "promisor" && len(calls) != 1 {
				t.Fatalf("promisor read beyond config: %q", raw)
			}
		})
	}
}

func TestAutomergeTransactionEarlyExitAndAckTimeout(t *testing.T) {
	for _, mode := range []string{"early exit", "ack timeout", "oversized ack"} {
		t.Run(mode, func(t *testing.T) {
			f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
			script := "exit 0\n"
			if mode == "ack timeout" {
				script = "sleep 1\nexit 0\n"
			}
			if mode == "oversized ack" {
				script = "printf '%2048s\\n' x\nexit 0\n"
			}
			installAutomergeGitWrapper(t, script)
			automergeRefAckTimeout = 20 * time.Millisecond
			start := time.Now()
			if txn, err := prepareAutomergeRef("refs/heads/main", f.proposal.Event.Head, f.proposal.Event.Base); err == nil {
				txn.close()
				t.Fatal("invalid transaction process succeeded")
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("transaction response was not bounded")
			}
		})
	}
}

func TestAutomergeAuthorChangeRequestDoesNotDisappearWithAuthorApprovalDisabled(t *testing.T) {
	f := newAutomergeTestRepo(t, func(p *PolicyDocument) {
		p.Proposals.TrustedReviewers = append(p.Proposals.TrustedReviewers, p.Automerge.Rules[0].Authors[0])
	}, map[string]string{"docs/safe.md": "x"})
	event, _ := nextEvent(f.author, "review.submit")
	event.Subject = f.proposal.ID
	event.Verdict = "request-changes"
	if _, err := appendEvent(event, f.author); err != nil {
		t.Fatal(err)
	}
	out, err := captureTestOutput(t, func() error { return cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}) })
	if err == nil || !strings.Contains(out+err.Error(), "current trusted change request") {
		t.Fatalf("author's trusted change request ignored: %s %v", out, err)
	}
}

func TestAutomergePolicyAmendmentAlwaysRequiresManualMerge(t *testing.T) {
	f := newAutomergeTestRepo(t, func(p *PolicyDocument) { p.Automerge.Rules[0].Paths = []string{"docs/", ".hn/"} }, map[string]string{"docs/safe.md": "x"})
	mustGit(t, "switch", "-q", "candidate")
	f.policy.Proposals.AllowAuthorApproval = true
	writeTestPolicy(t, f.root, f.policy)
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "change policy")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")
	event, _ := nextEvent(f.author, "proposal.open")
	event.Title = "Policy change"
	event.Base = f.proposal.Event.Base
	event.Head = head
	proposal, err := appendEvent(event, f.author)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	appendRevisionReview(t, f.reviewer, proposal.ID)
	if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
		t.Fatal(err)
	}
	out, err := captureTestOutput(t, func() error { return cmdAutomerge([]string{proposal.ID, "--rule", "docs", "--execute"}) })
	if err == nil || !strings.Contains(out+err.Error(), "policy amendments require manual merge") {
		t.Fatalf("policy amendment authorized itself: %s %v", out, err)
	}
}

func TestAutomergeIgnoredAncestorAndLateUntrackedCollision(t *testing.T) {
	t.Run("ignored directory replaced by allowed file", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/ignored.md": "candidate file"})
		if err := os.MkdirAll("docs/ignored.md", 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("docs/ignored.md/user", []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("ignored descendant collision overwritten")
		}
		raw, _ := os.ReadFile("docs/ignored.md/user")
		if string(raw) != "keep" {
			t.Fatal("ignored descendant lost")
		}
		requireAutomergeNoLocks(t)
	})
	t.Run("untracked file created under lease", func(t *testing.T) {
		f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "candidate"})
		automergeProcessHook = func(step string) error {
			if step == "lease-prepared" {
				return os.WriteFile("docs/safe.md", []byte("user file"), 0600)
			}
			return nil
		}
		if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
			t.Fatal("late untracked file overwritten")
		}
		raw, _ := os.ReadFile("docs/safe.md")
		if string(raw) != "user file" {
			t.Fatal("untracked content lost")
		}
		requireAutomergeNoLocks(t)
	})
}

func TestAutomergeCommitOutcomeObservationAndSafeRollback(t *testing.T) {
	for _, mode := range []string{"aborted commit", "lost acknowledgement", "rollback preserves edit"} {
		t.Run(mode, func(t *testing.T) {
			f := newAutomergeTestRepo(t, nil, map[string]string{"docs/existing.md": "candidate"})
			before, _, refs := autoSnapshot(t)
			automergeRefAckTimeout = time.Second
			script := `case "$*" in
 *"update-ref --stdin"*)
  while IFS= read -r line; do
   if [ "$line" = commit ]; then
    if [ "$HN_TEST_AUTO_EDIT" = yes ]; then printf 'late user edit' > docs/existing.md; fi
    printf 'abort\n'
   else printf '%s\n' "$line"; fi
  done | "$HN_TEST_AUTO_REAL_GIT" "$@"
  ;;
 *) exec "$HN_TEST_AUTO_REAL_GIT" "$@" ;;
esac
`
			if mode == "lost acknowledgement" {
				script = `case "$*" in
 *"update-ref --stdin"*)
  "$HN_TEST_AUTO_REAL_GIT" "$@" | while IFS= read -r line; do
   if [ "$line" != 'commit: ok' ]; then printf '%s\n' "$line"; fi
  done
  ;;
 *) exec "$HN_TEST_AUTO_REAL_GIT" "$@" ;;
esac
`
			}
			if mode == "rollback preserves edit" {
				t.Setenv("HN_TEST_AUTO_EDIT", "yes")
			}
			installAutomergeGitWrapper(t, script)
			err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"})
			if mode == "lost acknowledgement" {
				if err != nil {
					t.Fatalf("actual committed ref not recognized: %v", err)
				}
				events, err := collectEvents()
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, e := range events {
					if e.Event.Kind == "proposal.merged" {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("confirmed actual merge facts=%d", count)
				}
			} else {
				if err == nil {
					t.Fatal("aborted ref transaction succeeded")
				}
				after, _, afterRefs := autoSnapshot(t)
				if before != after || refs != afterRefs {
					t.Fatal("aborted transaction changed refs")
				}
				raw, _ := os.ReadFile("docs/existing.md")
				if mode == "rollback preserves edit" {
					if string(raw) != "late user edit" {
						t.Fatal("rollback lost user edit")
					}
				} else if string(raw) != "existing\n" || mustGitText(t, "diff", "--cached", "--name-only") != "" {
					t.Fatal("safe rollback failed to restore checkout/index content")
				}
			}
			requireAutomergeNoLocks(t)
		})
	}
}

func TestAutomergeIgnoredCollisionLookup(t *testing.T) {
	for _, tc := range []struct {
		changed []string
		ignored string
		want    bool
	}{
		{[]string{"a", "a-z"}, "a/x", true},
		{[]string{"a-z", "a/x"}, "a", true},
		{[]string{"a-z", "a/x"}, "a-z", true},
		{[]string{"docs/safe.md", "docs/z"}, "docs/cache/user", false},
		{[]string{"docs/safe.md"}, "doc", false},
		{[]string{"docs/safe.md"}, "docs/safe.md.extra", false},
	} {
		sort.Strings(tc.changed)
		if got := automergeIgnoredPathCollision(tc.changed, tc.ignored); got != tc.want {
			t.Fatalf("changed %q ignored %q = %t", tc.changed, tc.ignored, got)
		}
	}
	changed := make([]string, 12000)
	for i := range changed {
		changed[i] = fmt.Sprintf("changed/dir-%05d/file", i)
	}
	sort.Strings(changed)
	for i := range changed {
		p := fmt.Sprintf("cache/dir-%05d/user", i)
		if automergeIgnoredPathCollision(changed, p) {
			t.Fatalf("unrelated ignored path blocked: %q", p)
		}
	}
}

func TestAutomergePreservesUnfinishedGitOperation(t *testing.T) {
	f := newAutomergeTestRepo(t, nil, map[string]string{"docs/safe.md": "x"})
	path := mustGitText(t, "rev-parse", "--git-path", "MERGE_HEAD")
	state := []byte(f.proposal.Event.Head + "\n")
	if err := os.WriteFile(path, state, 0600); err != nil {
		t.Fatal(err)
	}
	before, index, refs := autoSnapshot(t)
	if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil || !strings.Contains(err.Error(), "unfinished Git operation") {
		t.Fatalf("unfinished merge bypassed: %v", err)
	}
	after, afterIndex, afterRefs := autoSnapshot(t)
	raw, _ := os.ReadFile(path)
	if before != after || refs != afterRefs || !bytes.Equal(index, afterIndex) || !bytes.Equal(state, raw) {
		t.Fatal("operation refusal modified Git state")
	}
}

func TestAutomergeSubdirectoryDoesNotMissIgnoredCollision(t *testing.T) {
	f := newAutomergeTestRepo(t, nil, map[string]string{"docs/ignored.md": "candidate"})
	if err := os.MkdirAll("cache/subdir", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("docs/ignored.md", []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("cache/subdir"); err != nil {
		t.Fatal(err)
	}
	if err := cmdAutomerge([]string{f.proposal.ID, "--rule", "docs", "--execute"}); err == nil {
		t.Fatal("subdirectory missed ignored collision")
	}
	raw, _ := os.ReadFile(filepath.Join(f.root, "docs/ignored.md"))
	if string(raw) != "keep" {
		t.Fatal("ignored content lost")
	}
}
