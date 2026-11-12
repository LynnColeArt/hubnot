package main

import (
	"context"
	"encoding/json"
	c "hubnot/internal/contribution"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type proposalPublicationEnvelope struct {
	OK    bool
	Data  *proposalPublicationReceipt
	Error *proposalPublicationError
}

func TestProposalPublicationOperation(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := inReplicationTestRepository(t)
	identity, _, err := createIdentity("Publication")
	if err != nil {
		t.Fatal(err)
	}
	writeTestPolicy(t, root, PolicyDocument{
		Version: policyVersion, Maintainers: []string{identity.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
	})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-qm", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "update-ref", "refs/heads/main", base)
	env := operationalTestEnvironment(t.TempDir())
	command := func(env []string, args ...string) (*exec.Cmd, context.CancelFunc) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		cmd := exec.CommandContext(ctx, binary, append([]string{"proposal"}, args...)...)
		cmd.Dir = root
		cmd.Env = env
		return cmd, cancel
	}
	invoke := func(args ...string) proposalPublicationEnvelope {
		t.Helper()
		cmd, cancel := command(env, args...)
		defer cancel()
		out, err := cmd.Output()
		return parseProposalPublicationEnvelope(t, out, err)
	}
	publish := func(actor, op, title string) []string {
		return []string{"publish", "--base", base, "--base-ref", "refs/heads/main", "--head", head,
			"--actor", actor, "--operation", op, "--title", title, "--body", "Exact\nbody", "--json"}
	}
	lookup := func(op string) proposalPublicationEnvelope {
		return invoke("operation", "--actor", identity.Actor, "--operation", op, "--json")
	}
	requireFailure := func(value proposalPublicationEnvelope, code string) {
		t.Helper()
		if value.OK || value.Error == nil || value.Error.Code != code {
			t.Fatalf("wanted %s, got %+v", code, value)
		}
	}
	requireFailure(lookup("missing"), "not_found")
	requireFailure(invoke(publish(strings.Repeat("0", 64), "first", "Exact title")...), "actor_mismatch")
	if events, err := collectEvents(); err != nil || len(events) != 0 {
		t.Fatalf("refusal changed history: %d events, %v", len(events), err)
	}

	// Two independent processes capture and submit the same intent.
	args := publish(identity.Actor, "first", "Exact title")
	first, cancelFirst := command(env, args...)
	defer cancelFirst()
	second, cancelSecond := command(env, args...)
	defer cancelSecond()
	type processResult struct {
		out []byte
		err error
	}
	finished := make(chan processResult, 2)
	for _, cmd := range []*exec.Cmd{first, second} {
		go func(cmd *exec.Cmd) {
			out, err := cmd.Output()
			finished <- processResult{out, err}
		}(cmd)
	}
	a, b := <-finished, <-finished
	one := parseProposalPublicationEnvelope(t, a.out, a.err)
	two := parseProposalPublicationEnvelope(t, b.out, b.err)
	if !one.OK || !two.OK || one.Data == nil || two.Data == nil || one.Data.ID != two.Data.ID ||
		one.Data.Replayed == two.Data.Replayed || !one.Data.Ready || !two.Data.Ready {
		t.Fatalf("concurrent operations did not return one publication and one replay: %s / %s", a.out, b.out)
	}
	proposal := one.Data
	if proposal.Base != base || proposal.Head != head || proposal.BaseRef != "refs/heads/main" ||
		proposal.Actor != identity.Actor || proposal.Title != "Exact title" || proposal.Body != "Exact\nbody" {
		t.Fatalf("unexpected receipt: %+v", proposal)
	}
	assertRefValue(t, proposalRef(proposal.ID), head)
	if events, err := collectEvents(); err != nil || len(events) != 1 {
		t.Fatalf("concurrent replay changed history: %d events, %v", len(events), err)
	}
	requireFailure(invoke(publish(identity.Actor, "first", "Different title")...), "operation_conflict")

	// A signed event can outlive its code ref. Lookup is read-only, then a
	// replay repairs that precise ref without revisiting the old base.
	mustGit(t, "update-ref", "-d", proposalRef(proposal.ID))
	mustGit(t, "update-ref", "refs/heads/main", head)
	missingCode := lookup("first")
	if !missingCode.OK || missingCode.Data == nil || missingCode.Data.ID != proposal.ID || missingCode.Data.CodeReady || missingCode.Data.Ready {
		t.Fatalf("lookup could not recover the signed receipt: %+v", missingCode)
	}
	if _, exists, err := refValue(proposalRef(proposal.ID)); err != nil || exists {
		t.Fatalf("lookup wrote the code ref: exists=%v, err=%v", exists, err)
	}
	repaired := invoke(args...)
	if !repaired.OK || repaired.Data == nil || !repaired.Data.Replayed || !repaired.Data.Ready || repaired.Data.ID != proposal.ID {
		t.Fatalf("replay did not repair the original proposal: %+v", repaired)
	}
	assertRefValue(t, proposalRef(proposal.ID), head)
	requireFailure(invoke(publish(identity.Actor, "stale", "Stale base")...), "stale_base")
	requireFailure(lookup("stale"), "not_found")
	if events, err := collectEvents(); err != nil || len(events) != 1 {
		t.Fatalf("replay or stale refusal changed history: %d events, %v", len(events), err)
	}

	// Move the branch from inside the Git invocation, after all preflight
	// checks and just before the native ref transaction.
	mustGit(t, "update-ref", "refs/heads/main", base)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	script := `#!/bin/sh
case " $* " in
  *" update-ref --no-deref --stdin "*) "$HN_TEST_REAL_GIT" update-ref refs/heads/main "$HN_TEST_NEW_BASE" || exit $? ;;
esac
exec "$HN_TEST_REAL_GIT" "$@"
`
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	shimEnv := append(append([]string{}, env...), "PATH="+shim+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HN_TEST_REAL_GIT="+git, "HN_TEST_NEW_BASE="+head)
	cmd, cancel := command(shimEnv, publish(identity.Actor, "stale-race", "Raced base")...)
	defer cancel()
	out, err := cmd.Output()
	requireFailure(parseProposalPublicationEnvelope(t, out, err), "stale_base")
	requireFailure(lookup("stale-race"), "not_found")
	if events, err := collectEvents(); err != nil || len(events) != 1 {
		t.Fatalf("stale transaction changed signed history: %d events, %v", len(events), err)
	}

	// Kill the CLI after Git commits both refs, before it can return a
	// receipt. A fresh lookup and replay must find exactly the same event.
	mustGit(t, "update-ref", "refs/heads/main", base)
	script = `#!/bin/sh
case " $* " in
  *" update-ref --no-deref --stdin "*)
    "$HN_TEST_REAL_GIT" "$@" || exit $?
    kill -KILL "$PPID"
    exit 0 ;;
esac
exec "$HN_TEST_REAL_GIT" "$@"
`
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	lost := publish(identity.Actor, "lost-response", "Interrupted publication")
	cmd, cancelLost := command(shimEnv, lost...)
	defer cancelLost()
	out, err = cmd.Output()
	if err == nil || len(out) != 0 {
		t.Fatalf("expected lost receipt, got %q, %v", out, err)
	}
	observed := lookup("lost-response")
	if !observed.OK || observed.Data == nil || !observed.Data.Ready {
		t.Fatalf("committed operation lost with caller: %+v", observed)
	}
	replay := invoke(lost...)
	if !replay.OK || replay.Data == nil || !replay.Data.Replayed || replay.Data.ID != observed.Data.ID {
		t.Fatalf("interrupted operation did not replay: %+v", replay)
	}
	if events, err := collectEvents(); err != nil || len(events) != 2 {
		t.Fatalf("interrupted replay changed signed history: %d events, %v", len(events), err)
	}
}

func TestProposalPublicationContributionPolicy(t *testing.T) {
	binary := buildOperationalBinary(t)
	for _, allowed := range []bool{true, false} {
		name := "allowed"
		if !allowed {
			name = "denied"
		}
		t.Run(name, func(t *testing.T) {
			rules := []c.Rule{{Operation: c.IssueCreate, Automatic: true}}
			if allowed {
				rules = append(rules, c.Rule{Operation: c.ProposalCreate, Automatic: true})
			}
			repo, actor, _ := contributionEnabledRepository(t, binary, "publication-"+name, rules)
			base := contributionGit(t, repo, "rev-parse", "refs/heads/main")
			contributionGit(t, repo, "switch", "-qc", "candidate")
			contributionGit(t, repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "Candidate")
			head := contributionGit(t, repo, "rev-parse", "HEAD")
			before := contributionGit(t, repo, "for-each-ref", "--format=%(objectname)", actorRef(actor))
			args := []string{"proposal", "publish", "--base", base, "--base-ref", "refs/heads/main", "--head", head,
				"--actor", actor, "--operation", "governed", "--title", "Governed candidate", "--json"}
			invoke := func(args ...string) proposalPublicationEnvelope {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, binary, args...)
				cmd.Dir, cmd.Env = repo, operationalTestEnvironment(t.TempDir())
				out, err := cmd.Output()
				return parseProposalPublicationEnvelope(t, out, err)
			}
			first := invoke(args...)
			if !allowed {
				if first.OK || first.Error == nil || first.Error.Code != "denied" ||
					contributionGit(t, repo, "for-each-ref", "--format=%(objectname)", actorRef(actor)) != before {
					t.Fatalf("policy refusal did not preserve history: %+v", first)
				}
				return
			}
			if !first.OK || first.Data == nil || !first.Data.Ready {
				t.Fatalf("governed publication was not admitted: %+v %+v", first, first.Error)
			}
			after := contributionGit(t, repo, "for-each-ref", "--format=%(objectname)", actorRef(actor))
			replay := invoke(args...)
			if !replay.OK || replay.Data == nil || replay.Data.ID != first.Data.ID || !replay.Data.Replayed ||
				contributionGit(t, repo, "for-each-ref", "--format=%(objectname)", actorRef(actor)) != after {
				t.Fatalf("governed replay changed history: %+v %+v", replay, replay.Error)
			}
		})
	}
}

func parseProposalPublicationEnvelope(t *testing.T, out []byte, err error) proposalPublicationEnvelope {
	t.Helper()
	var value proposalPublicationEnvelope
	if decodeErr := json.Unmarshal(out, &value); decodeErr != nil || value.OK != (err == nil) {
		t.Fatalf("invalid publication result %s: subprocess %v, decode %v", out, err, decodeErr)
	}
	return value
}
