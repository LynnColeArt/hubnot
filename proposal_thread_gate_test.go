package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProposalMergeRequiresResolvedReviewThreads(t *testing.T) {
	for _, required := range []bool{false, true} {
		name := "optional"
		if required {
			name = "required"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			mustGit(t, "init", "-q", "-b", "main")
			mustGit(t, "config", "user.name", "Test")
			mustGit(t, "config", "user.email", "test@hn.invalid")
			identity, _, err := createIdentity("Reviewer")
			if err != nil {
				t.Fatal(err)
			}
			writeTestPolicy(t, root, PolicyDocument{
				Version: policyVersion, Maintainers: []string{identity.Actor},
				Proposals: ProposalPolicy{RequiredAccepts: 1, RequireResolvedReviewThreads: required},
				Pipelines: map[string]PipelinePolicy{},
			})
			mustGit(t, "add", ".hn/policy.json")
			mustGit(t, "commit", "-qm", "base")
			base := mustGitText(t, "rev-parse", "HEAD")
			mustGit(t, "switch", "-qc", "feature")
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("review this\n"), 0600); err != nil {
				t.Fatal(err)
			}
			mustGit(t, "add", "README.md")
			mustGit(t, "commit", "-qm", "feature")
			head := mustGitText(t, "rev-parse", "HEAD")
			mustGit(t, "switch", "-q", "main")
			appendSigned := func(kind string, fill func(*Event)) *StoredEvent {
				t.Helper()
				event, err := nextEvent(identity, kind)
				if err != nil {
					t.Fatal(err)
				}
				fill(&event)
				stored, err := appendEvent(event, identity)
				if err != nil {
					t.Fatal(err)
				}
				return stored
			}
			proposal := appendSigned("proposal.open", func(e *Event) {
				e.Title, e.Base, e.Head = "Thread gate", base, head
			})
			if err := createProposalRef(proposal.ID, head); err != nil {
				t.Fatal(err)
			}
			anchor := proposalInlineAnchor{Path: "README.md", Side: "RIGHT", Line: 1}
			thread := appendSigned("proposal.inline", func(e *Event) {
				e.Subject, e.Body = proposal.ID, "Please check this line"
				a := anchor
				a.Operation = eventID([]byte(e.Body))
				e.Inline = &a
			})
			_, _, digest, err := loadPolicy(base)
			if err != nil {
				t.Fatal(err)
			}
			if err := cmdDecide([]string{proposal.ID, "--accept"}); err != nil {
				t.Fatal(err)
			}
			check := func(blocked bool) {
				t.Helper()
				events, err := collectGovernedEvents()
				if err != nil {
					t.Fatal(err)
				}
				evaluation, err := evaluateProposal(proposal, events)
				if err != nil {
					t.Fatal(err)
				}
				if evaluation.PolicyDigest != digest || !evaluation.Ready || !evaluation.Accepted ||
					(len(evaluation.BlockingReviewThreads) != 0) != blocked {
					t.Fatalf("unexpected evaluation: %+v", evaluation)
				}
				projection := consumerProposalProjection(evaluation, events)
				merge := projection["observed_preconditions"].(map[string]any)["merge"].(map[string]any)
				if merge["met"] != !blocked {
					t.Fatalf("unexpected consumer merge readiness: %v", merge)
				}
				if blocked && evaluation.BlockingReviewThreads[0].ID != thread.ID {
					t.Fatalf("unexpected blocking thread: %+v", evaluation.BlockingReviewThreads)
				}
				if got := unresolvedReviewThreads(events, eventID([]byte("other revision"))); len(got) != 0 {
					t.Fatalf("threads leaked into another revision: %+v", got)
				}
			}
			change := func(state, operation string) {
				appendSigned("proposal.inline", func(e *Event) {
					e.Subject = proposal.ID
					a := anchor
					a.ReplyTo, a.State, a.Operation = thread.ID, state, eventID([]byte(operation))
					e.Inline = &a
				})
			}
			check(required)
			if required {
				assertBlocked := func() {
					t.Helper()
					err := cmdMerge([]string{proposal.ID})
					if err == nil || !strings.Contains(err.Error(), "resolve 1 review thread") {
						t.Fatalf("expected thread blocker, got %v", err)
					}
					if got := mustGitText(t, "rev-parse", "HEAD"); got != base {
						t.Fatalf("blocked merge changed target to %s", got)
					}
				}
				assertBlocked()
				change("resolved", "resolve")
				check(false)
				change("open", "reopen")
				check(true)
				assertBlocked()
				change("resolved", "resolve again")
				check(false)
			}
			if !required {
				// An already-open proposal cannot bypass a newly enabled target policy.
				policy, raw, _, err := loadPolicy(base)
				if err != nil {
					t.Fatal(err)
				}
				policy.Proposals.RequireResolvedReviewThreads = true
				writeTestPolicy(t, root, policy)
				mustGit(t, "commit", "-qam", "Require review thread resolution")
				if err := cmdMerge([]string{proposal.ID}); err == nil || !strings.Contains(err.Error(), "target review thread requirement") {
					t.Fatalf("expected target policy blocker, got %v", err)
				}
				// Restoring the original policy allows the open-thread proposal to merge.
				if err := os.WriteFile(filepath.Join(root, ".hn/policy.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
				mustGit(t, "commit", "-qam", "Restore policy")
			}
			if err := cmdMerge([]string{proposal.ID}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
