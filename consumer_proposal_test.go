package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type consumerProposalRead struct {
	Schema, ID, Base, Head, Policy, Status string
	Readiness                              bool
	CurrentReviews                         map[string]struct {
		ID, Actor, Verdict      string
		ActorName               string `json:"actor_name"`
		Sequence                uint64
		PolicyEligible          bool `json:"policy_eligible"`
		ProtectedPolicyEligible bool `json:"protected_policy_eligible"`
	} `json:"current_reviews"`
	Requirements struct {
		Missing []string
		Reviews struct {
			Required, Approvals, Deficit int
			EligibleActors               []string `json:"eligible_actors"`
		}
		Pipelines       []json.RawMessage
		ProtectedChecks *struct {
			Required, Approvals, Deficit int
			ChangedPaths                 []string `json:"changed_paths"`
		} `json:"protected_checks"`
	}
	Evidence []string
	Lineage  struct {
		Superseded bool
		Successors []string
	}
	ObservedPreconditions struct {
		LineageTerminal bool `json:"lineage_terminal"`
		Acceptance      struct{ Met bool }
	} `json:"observed_preconditions"`
	Disagreements struct {
		ObservedChangeRequests  int `json:"observed_change_requests"`
		QualifiedChangeRequests int `json:"qualified_change_requests"`
	}
}

func consumerReadProposal(t *testing.T, cli *betaContributionCLI, repo, command, id string) consumerProposalRead {
	t.Helper()
	var result struct {
		OK    bool
		Data  consumerProposalRead
		Error any
	}
	out := cli.hn(repo, true, "proposal", command, id, "--json")
	if err := json.Unmarshal([]byte(out), &result); err != nil || !result.OK || result.Error != nil {
		t.Fatalf("proposal JSON: %v %s", err, out)
	}
	return result.Data
}

func TestConsumerProposalProjectionCLI(t *testing.T) {
	binary := runnerAcceptanceBinary(t, true)
	cli := newBetaContributionCLI(t, binary)
	repo := cli.repository()
	cli.hn(repo, true, "init", "--name", "Author")
	author := cli.actor(repo)
	actorDirs := []string{}
	actors := []string{}
	for i := 0; i < 3; i++ {
		d := filepath.Join(t.TempDir(), fmt.Sprintf("reviewer%d", i))
		cli.git(repo, "worktree", "add", "--quiet", "--detach", d, "HEAD")
		cli.hn(d, true, "init", "--name", "Same name")
		actorDirs = append(actorDirs, d)
		actors = append(actors, cli.actor(d))
	}
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{author}, Proposals: ProposalPolicy{RequiredAccepts: 1, RequiredApprovals: 2, TrustedReviewers: actors[:2]}, Pipelines: map[string]PipelinePolicy{"proof": {RequiredResults: 1, TrustedRunners: []string{author}}}, ProtectedChecks: &ProtectedChecksPolicy{Paths: []string{"protected.txt"}, TrustedReviewers: actors[:2], RequiredApprovals: 1}}
	raw, _ := json.Marshal(policy)
	betaContributionWrite(t, filepath.Join(repo, ".hn/policy.json"), raw, 0600)
	betaContributionWrite(t, filepath.Join(repo, ".hn/pipelines/proof.json"), []byte(`{"version":"hn.pipeline/0","steps":[{"name":"Git version","command":"git","args":["--version"]}]}`), 0600)
	cli.commit(repo, "Policy")
	base := cli.git(repo, "rev-parse", "HEAD")
	betaContributionWrite(t, filepath.Join(repo, "protected.txt"), []byte("candidate\n"), 0600)
	cli.commit(repo, "Candidate")
	head := cli.git(repo, "rev-parse", "HEAD")
	cli.hn(repo, true, "proposal", "open", "--base", base, "--head", head, "Candidate")
	id := cli.latest(repo, author)
	// Both legacy reads remain text and retain the exact defining lines.
	textStatus := cli.hn(repo, true, "proposal", "status", id)
	textShow := cli.hn(repo, true, "proposal", "show", id)
	var signed struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(cli.git(repo, "show", "refs/hn/actors/"+author+":event.json")), &signed); err != nil {
		t.Fatal(err)
	}
	expectedShow := fmt.Sprintf("%s  Candidate\nProposed by Author at %s\nBase: %s\nHead: %s\nCode: available and matched\n", strings.TrimPrefix(id, "sha256:")[:12], signed.Timestamp, base, head)
	policyDigest := fmt.Sprintf("%x", sha256.Sum256(raw))
	expectedStatus := fmt.Sprintf("Proposal: %s  Candidate\nPolicy:   %s from base %s\nCode:     ✓ signed head available and matched\nReviews:  0/2 trusted approvals\nPipeline: proof 0/1 trusted passes\nProtected checks: enabled; 1 changed path(s); 0/1 designated approvals\n  %q\nDecision: 0/1 accepts; 0 rejects\nStatus:   blocked\n", strings.TrimPrefix(id, "sha256:")[:12], policyDigest[:12], base[:12], "protected.txt")
	if textShow != expectedShow || textStatus != expectedStatus {
		t.Fatalf("legacy default byte mismatch: show=%q want=%q status=%q want=%q", textShow, expectedShow, textStatus, expectedStatus)
	}

	if !strings.Contains(textStatus, "Reviews:  0/2 trusted approvals\n") || !strings.Contains(textShow, "Base: "+base+"\nHead: "+head+"\n") {
		t.Fatalf("legacy text missing: %s %s", textStatus, textShow)
	}
	before := consumerFiles(t, repo)
	empty := consumerReadProposal(t, cli, repo, "status", id)
	shown := consumerReadProposal(t, cli, repo, "show", id)
	if empty.ID != id || empty.Base != base || empty.Head != head || !validEventID(empty.Policy) || empty.Schema != "hn.consumer/1" || empty.Readiness || len(empty.CurrentReviews) != 0 || empty.Requirements.Reviews.Deficit != 2 || empty.Requirements.ProtectedChecks == nil || empty.Requirements.ProtectedChecks.Deficit != 1 || len(empty.Requirements.Pipelines) != 1 || !reflect.DeepEqual(empty, shown) {
		t.Fatalf("missing requirements/full IDs: %+v / %+v", empty, shown)
	}
	if !reflect.DeepEqual(before, consumerFiles(t, repo)) {
		t.Fatal("reads changed repository/private bytes")
	}
	if cli.hn(repo, true, "proposal", "status", id) != textStatus || cli.hn(repo, true, "proposal", "show", id) != textShow {
		t.Fatal("JSON reads changed default text")
	}
	cli.hn(repo, true, "run", "request", id, "proof")
	request := cli.latest(repo, author)
	cli.hn(repo, true, "run", "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
	result := cli.latest(repo, author)
	cli.hn(actorDirs[0], true, "review", id, "--approve")
	a1 := cli.latest(repo, actors[0])
	cli.hn(actorDirs[1], true, "review", id, "--request-changes")
	b1 := cli.latest(repo, actors[1])
	cli.hn(actorDirs[2], true, "review", id, "--request-changes")
	u1 := cli.latest(repo, actors[2])
	observed := consumerReadProposal(t, cli, repo, "status", id)
	if len(observed.CurrentReviews) != 3 || observed.CurrentReviews[actors[0]].ID != a1 || observed.CurrentReviews[actors[1]].ID != b1 || observed.CurrentReviews[actors[2]].ID != u1 || observed.CurrentReviews[actors[2]].PolicyEligible || !observed.CurrentReviews[actors[0]].PolicyEligible || observed.Disagreements.ObservedChangeRequests != 2 || observed.Disagreements.QualifiedChangeRequests != 1 || observed.Requirements.Reviews.Deficit != 1 {
		t.Fatalf("attribution/qualifications: %+v", observed)
	}
	if observed.CurrentReviews[actors[0]].ActorName != "Same name" || observed.CurrentReviews[actors[1]].ActorName != "Same name" {
		t.Fatalf("display-name collision fixture ineffective: %+v", observed.CurrentReviews)
	}
	cli.hn(actorDirs[1], true, "review", id, "--approve")
	b2 := cli.latest(repo, actors[1])
	ready := consumerReadProposal(t, cli, repo, "show", id)
	foundResult := false
	for _, evidence := range ready.Evidence {
		if evidence == result {
			foundResult = true
		}
	}
	if !foundResult || !strings.Contains(string(ready.Requirements.Pipelines[0]), result) {
		t.Fatalf("missing exact pipeline result: %+v", ready)
	}
	if !ready.Readiness || ready.CurrentReviews[actors[1]].ID != b2 || ready.CurrentReviews[actors[1]].Sequence <= observed.CurrentReviews[actors[1]].Sequence || ready.Requirements.Reviews.Deficit != 0 || ready.Requirements.ProtectedChecks.Deficit != 0 || !ready.ObservedPreconditions.Acceptance.Met {
		t.Fatalf("latest approval readiness: %+v", ready)
	}
	cli.hn(repo, true, "proposal", "revise", id, "--base", base, "--head", head, "--body", "Successor")
	successor := cli.latest(repo, author)
	// A late approval on the predecessor must stay attached to that exact candidate.
	cli.hn(actorDirs[0], true, "review", id, "--approve")
	late := cli.latest(repo, actors[0])
	old := consumerReadProposal(t, cli, repo, "status", id)
	next := consumerReadProposal(t, cli, repo, "status", successor)
	if !old.Readiness || !old.Lineage.Superseded || old.Status != "superseded" || old.ObservedPreconditions.LineageTerminal || old.ObservedPreconditions.Acceptance.Met || old.CurrentReviews[actors[0]].ID != late || len(next.CurrentReviews) != 0 || next.Requirements.Reviews.Deficit != 2 || next.Readiness {
		t.Fatalf("exact candidate lineage/late verdict: old=%+v next=%+v", old, next)
	}
	// Signing state stays untouched by normal keyless reads.
	gitdir := cli.git(repo, "rev-parse", "--absolute-git-dir")
	active := filepath.Join(gitdir, "hn", "active")
	saved, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(active); err != nil {
		t.Fatal(err)
	}
	consumerReadProposal(t, cli, repo, "status", id)
	if _, err = os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("read recreated active signer: %v", err)
	}
	if err = os.WriteFile(active, saved, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestConsumerProposalErrorsCLI(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	repo := cli.repository()
	// Poisoned pre-existing lock proves invalid arguments are rejected before
	// lock acquisition, while valid grammar still reports its outer refusal.
	dir := filepath.Join(repo, ".git", "hn", "contribution")
	if err := os.MkdirAll(filepath.Join(dir, "operation.lock"), 0700); err != nil {
		t.Fatal(err)
	}
	betaContributionWrite(t, filepath.Join(dir, "authority.json"), []byte("PRIVATE_BAD_AUTHORITY"), 0600)
	id := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		dir  string
		args []string
		code string
	}{
		{repo, []string{"proposal", "status", id, "--unknown", "--json"}, "invalid_input"},
		{repo, []string{"proposal", "show", id, "--json"}, "repository_error"},
		{t.TempDir(), []string{"proposal", "status", id, "--json"}, "repository_error"},
	} {
		before := consumerFiles(t, tc.dir)
		cmd := exec.Command(cli.binary, tc.args...)
		cmd.Dir = tc.dir
		cmd.Env = cli.env
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		var env struct {
			OK    bool
			Data  any
			Error struct {
				Origin, Code, Message, Recovery string
				Operation                       any
			}
		}
		if err == nil || json.Unmarshal(out, &env) != nil || env.OK || env.Data != nil || env.Error.Code != tc.code || env.Error.Origin != "native" || env.Error.Operation != nil || env.Error.Recovery != "do_not_retry" || strings.Contains(string(out), "PRIVATE_BAD_AUTHORITY") {
			t.Fatalf("outer CLI %v: %v stdout=%s stderr=%s", tc.args, err, out, stderr.String())
		}
		if !reflect.DeepEqual(before, consumerFiles(t, tc.dir)) {
			t.Fatalf("refusal changed state: %v", tc.args)
		}
	}
	// The literal value --json is not an output-mode request.
	out := cli.hn(t.TempDir(), false, "proposal", "show", id, "--", "--json")
	if strings.Contains(out, `"ok"`) {
		t.Fatalf("literal selected JSON: %s", out)
	}
}
