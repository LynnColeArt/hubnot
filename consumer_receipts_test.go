package main

import (
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func consumerReceiptFixture(t *testing.T, cli *betaContributionCLI) (repo, actor, id, base, head string, policy PolicyDocument) {
	t.Helper()
	repo = cli.repository()
	cli.hn(repo, true, "init", "--name", "Maintainer")
	actor = cli.actor(repo)
	policy = PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1, RequiredApprovals: 1, TrustedReviewers: []string{actor}, AllowAuthorApproval: true}, Pipelines: map[string]PipelinePolicy{}}
	raw, _ := json.Marshal(policy)
	betaContributionWrite(t, filepath.Join(repo, ".hn/policy.json"), raw, 0600)
	cli.commit(repo, "Policy")
	base = cli.git(repo, "rev-parse", "HEAD")
	cli.commit(repo, "Candidate")
	head = cli.git(repo, "rev-parse", "HEAD")
	cli.hn(repo, true, "proposal", "open", "--base", base, "--head", head, "Receipt candidate")
	id = cli.latest(repo, actor)
	return
}
func consumerReceiptID(t *testing.T, data map[string]json.RawMessage) string {
	t.Helper()
	var id string
	if json.Unmarshal(data["id"], &id) != nil || !validEventID(id) || string(data["schema"]) != `"hn.consumer/1"` {
		t.Fatalf("missing full receipt ID/schema: %v", data)
	}
	return id
}
func consumerLegacyFixture(t *testing.T, cli *betaContributionCLI, repo, actor string) {
	t.Helper()
	gitdir := cli.git(repo, "rev-parse", "--absolute-git-dir")
	raw, err := os.ReadFile(filepath.Join(gitdir, "hn", "identities", actor+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]json.RawMessage
	if json.Unmarshal(raw, &record) != nil {
		t.Fatal("identity fixture")
	}
	delete(record, "version")
	delete(record, "lifecycle")
	legacy, _ := json.Marshal(record)
	if err = os.RemoveAll(filepath.Join(gitdir, "hn", "identities")); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(gitdir, "hn", "active")); err != nil {
		t.Fatal(err)
	}
	betaContributionWrite(t, filepath.Join(gitdir, "hn", "identity.json"), legacy, 0600)
}

func TestConsumerReceiptsCLI(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	repo, actor, id, _, _, _ := consumerReceiptFixture(t, cli)
	approval := consumerJSONProcess(t, cli, repo, true, "review", id, "--approve", "--json")
	reviewID := consumerReceiptID(t, approval)
	if reviewID != cli.latest(repo, actor) || string(approval["subject"]) != fmt.Sprintf("%q", id) || string(approval["actor"]) != fmt.Sprintf("%q", actor) || string(approval["verdict"]) != `"approve"` {
		t.Fatalf("review receipt not exact created event: %v", approval)
	}
	status := consumerReadProposal(t, cli, repo, "status", id)
	if !reflect.DeepEqual(status.Evidence, []string{reviewID}) {
		t.Fatalf("proposal status evidence mismatch: %+v", status)
	}
	decision := consumerJSONProcess(t, cli, repo, true, "decide", id, "--accept", "--json")
	decisionID := consumerReceiptID(t, decision)
	var evidence []string
	var policy string
	_ = json.Unmarshal(decision["evidence"], &evidence)
	_ = json.Unmarshal(decision["policy"], &policy)
	if decisionID != cli.latest(repo, actor) || !reflect.DeepEqual(evidence, []string{reviewID}) || policy != status.Policy || !validEventID(policy) {
		t.Fatalf("decision receipt binding: %v", decision)
	}
	for _, e := range evidence {
		row := consumerJSONProcess(t, cli, repo, true, "event", "show", e, "--json")
		if string(row["id"]) != fmt.Sprintf("%q", e) {
			t.Fatal("evidence roundtrip")
		}
	}
	// --json consumed as body text must preserve the original text output mode.
	out := cli.hn(repo, true, "review", id, "--request-changes", "--body", "--json")
	latest := cli.latest(repo, actor)
	expected := fmt.Sprintf("Recorded request-changes review %s on proposal %s\n", strings.TrimPrefix(latest, "sha256:")[:12], strings.TrimPrefix(id, "sha256:")[:12])
	if out != expected {
		t.Fatalf("default review bytes: %q want %q", out, expected)
	}
	newer := consumerReadProposal(t, cli, repo, "status", id)
	if newer.Readiness || len(newer.Evidence) != 0 {
		t.Fatalf("newer evidence should differ: %+v", newer)
	}
	stored := consumerJSONProcess(t, cli, repo, true, "event", "show", decisionID, "--json")
	var signed struct {
		Policy   string
		Evidence []string
	}
	if json.Unmarshal(stored["event"], &signed) != nil || signed.Policy != policy || !reflect.DeepEqual(signed.Evidence, evidence) {
		t.Fatalf("older decision changed with current evidence: %v", stored)
	}
	changes := consumerJSONProcess(t, cli, repo, true, "review", id, "--request-changes", "--body", "reason", "--json")
	if string(changes["verdict"]) != `"request-changes"` || consumerReceiptID(t, changes) != cli.latest(repo, actor) {
		t.Fatalf("changes receipt: %v", changes)
	}
	out = cli.hn(repo, true, "review", id, "--approve")
	latest = cli.latest(repo, actor)
	expected = fmt.Sprintf("Recorded approve review %s on proposal %s\n", strings.TrimPrefix(latest, "sha256:")[:12], strings.TrimPrefix(id, "sha256:")[:12])
	if out != expected {
		t.Fatalf("default approve bytes %q want%q", out, expected)
	}
	out = cli.hn(repo, true, "decide", id, "--reject", "--body=--json")
	latest = cli.latest(repo, actor)
	expected = fmt.Sprintf("Recorded reject decision %s on proposal %s under policy %s\n", strings.TrimPrefix(latest, "sha256:")[:12], strings.TrimPrefix(id, "sha256:")[:12], strings.TrimPrefix(policy, "sha256:")[:12])
	if out != expected {
		t.Fatalf("default reject bytes %q want%q", out, expected)
	}
	rejected := consumerJSONProcess(t, cli, repo, true, "decide", id, "--reject", "--body", "rejected", "--json")
	if string(rejected["verdict"]) != `"reject"` || string(rejected["evidence"]) != "[]" || consumerReceiptID(t, rejected) != cli.latest(repo, actor) {
		t.Fatalf("reject receipt: %v", rejected)
	}
	out = cli.hn(repo, true, "decide", id, "--accept")
	latest = cli.latest(repo, actor)
	expected = fmt.Sprintf("Recorded accept decision %s on proposal %s under policy %s\nEvidence: 1 signed event(s)\n", strings.TrimPrefix(latest, "sha256:")[:12], strings.TrimPrefix(id, "sha256:")[:12], strings.TrimPrefix(policy, "sha256:")[:12])
	if out != expected {
		t.Fatalf("default accept bytes %q want%q", out, expected)
	}
}

func TestConsumerReceiptEarlyRefusalsCLI(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	repo, actor, id, base, head, policy := consumerReceiptFixture(t, cli)
	cli.hn(repo, true, "proposal", "revise", id, "--base", base, "--head", head)
	successor := cli.latest(repo, actor)
	consumerLegacyFixture(t, cli, repo, actor)
	for _, machine := range []bool{false, true} {
		for _, args := range [][]string{{"review", strings.TrimPrefix(id, "sha256:")[:12], "--approve"}, {"decide", strings.TrimPrefix(id, "sha256:")[:12], "--accept"}, {"review", id, "--approve", "--unknown"}, {"decide", id, "--accept", "--unknown"}, {"decide", id, "--accept"}, {"review", successor, "--approve", "--json=bogus"}, {"decide", successor, "--reject", "--body", "invalid boolean", "--json=bogus"}} {
			if machine {
				args = append(args, "--json")
			}
			before := consumerFiles(t, repo)
			if machine {
				consumerJSONProcess(t, cli, repo, false, args...)
			} else {
				cli.hn(repo, false, args...)
			}
			if !reflect.DeepEqual(before, consumerFiles(t, repo)) {
				t.Fatalf("early refusal changed legacy/private/public state: %v", args)
			}
		}
	}
	// A literal after -- is neither a machine request nor an accepted extra value.
	before := consumerFiles(t, repo)
	out := cli.hn(repo, false, "review", successor, "--approve", "--", "--json")
	if strings.Contains(out, `"ok"`) || !reflect.DeepEqual(before, consumerFiles(t, repo)) {
		t.Fatalf("literal mode/refusal effects: %s", out)
	}
	// Eligible control proves the untouched legacy key is usable; only success may
	// now migrate/sign. Complete all early-refusal snapshots before this control.
	cli.hn(repo, true, "review", successor, "--approve")
	policy.Contributions = &c.Policy{Project: c.Digest([]byte(t.Name())), Moderators: []string{actor}, Baseline: []string{id, successor}, Limits: betaContributionLimits(), Rules: []c.Rule{{Operation: c.ProposalCreate}}}
	sort.Strings(policy.Contributions.Baseline)
	raw, _ := json.Marshal(policy)
	betaContributionWrite(t, filepath.Join(repo, ".hn/policy.json"), raw, 0600)
	cli.commit(repo, "Authority")
	cli.hn(repo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	cli.decide(repo, "hide", "--revision", successor, "absent", "hide-successor")
	consumerLegacyFixture(t, cli, repo, actor)
	for _, machine := range []bool{false, true} {
		for _, args := range [][]string{{"review", successor, "--approve"}, {"decide", successor, "--accept"}} {
			if machine {
				args = append(args, "--json")
			}
			before := consumerFiles(t, repo)
			if machine {
				consumerJSONProcess(t, cli, repo, false, args...)
			} else {
				cli.hn(repo, false, args...)
			}
			if !reflect.DeepEqual(before, consumerFiles(t, repo)) {
				t.Fatalf("hidden refusal changed legacy/private/public state: %v", args)
			}
		}
	}
}

func TestConsumerReceiptLaterRefusalPreservesWriterBehavior(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	for _, machine := range []bool{false, true} {
		repo, actor, id, _, _, _ := consumerReceiptFixture(t, cli)
		consumerLegacyFixture(t, cli, repo, actor)
		refs := cli.git(repo, "show-ref")
		args := []string{"decide", id, "--accept"}
		if machine {
			args = append(args, "--json")
			consumerJSONProcess(t, cli, repo, false, args...)
		} else {
			cli.hn(repo, false, args...)
		}
		if cli.git(repo, "show-ref") != refs {
			t.Fatal("not-ready refusal appended signed facts")
		}
		gitdir := cli.git(repo, "rev-parse", "--absolute-git-dir")
		if _, err := os.Stat(filepath.Join(gitdir, "hn", "identities", actor+".json")); err != nil {
			t.Fatalf("existing writer signer migration changed: %v", err)
		}
	}
}

func TestConsumerOutputGrammarCLI(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	outside := t.TempDir()
	for _, args := range [][]string{{"version", "--json", "--json=false"}, {"version", "value", "--json"}, {"--version", "--", "--json"}} {
		if out := cli.hn(outside, true, args...); out != version+"\n" {
			t.Fatalf("registered grammar did not preserve text: %v %q", args, out)
		}
	}
	repo, actor, id, _, _, _ := consumerReceiptFixture(t, cli)
	// The writer's original unknown-flag usage text must not acquire a new -json
	// entry when no machine option is selected.
	for _, command := range []string{"review", "decide"} {
		args := []string{command, id, "--unknown"}
		out := cli.hn(repo, false, args...)
		if strings.Contains(out, "\n  -json") {
			t.Fatalf("default unknown-flag usage changed: %s", out)
		}
	}
	for _, args := range [][]string{{"review", id, "--approve", "--json", "--json=false"}, {"decide", id, "--reject", "--body", "last false", "--json", "--json=false"}} {
		out := cli.hn(repo, true, args...)
		if strings.Contains(out, `"ok"`) || !strings.HasPrefix(out, "Recorded ") {
			t.Fatalf("last false mode: %v %s", args, out)
		}
	}
	explicit := consumerJSONProcess(t, cli, repo, true, "review", id, "--approve", "--body", "--json", "--json")
	created := consumerReceiptID(t, explicit)
	if created != cli.latest(repo, actor) {
		t.Fatal("explicit machine receipt missing")
	}
	row := consumerJSONProcess(t, cli, repo, true, "event", "show", created, "--json")
	var signed struct{ Body string }
	if json.Unmarshal(row["event"], &signed) != nil || signed.Body != "--json" {
		t.Fatalf("body value was reclassified as mode flag: %v", row)
	}
	before := consumerFiles(t, repo)
	out := cli.hn(repo, false, "decide", id, "--reject", "--body", "literal", "--", "--json")
	if strings.Contains(out, `"ok"`) || !reflect.DeepEqual(before, consumerFiles(t, repo)) {
		t.Fatalf("decide literal selected mode/effect: %s", out)
	}
}
