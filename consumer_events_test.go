package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func consumerJSONProcess(t *testing.T, cli *betaContributionCLI, repo string, ok bool, args ...string) map[string]json.RawMessage {
	t.Helper()
	cmd := exec.Command(cli.binary, args...)
	cmd.Dir = repo
	cmd.Env = cli.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var envelope struct {
		OK    bool
		Data  map[string]json.RawMessage
		Error *struct{ Code string }
	}
	if (err == nil) != ok || json.Unmarshal(out, &envelope) != nil || envelope.OK != ok {
		t.Fatalf("%v: %v stdout=%s stderr=%s", args, err, out, stderr.String())
	}
	if !ok {
		if envelope.Error == nil {
			t.Fatalf("missing JSON error: %s", out)
		}
		return map[string]json.RawMessage{"code": json.RawMessage(fmt.Sprintf("%q", envelope.Error.Code))}
	}
	if envelope.Error != nil || envelope.Data == nil {
		t.Fatalf("invalid JSON success: %s", out)
	}
	return envelope.Data
}

func TestConsumerEventAndLogCLI(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	repo := cli.repository()
	cli.hn(repo, true, "init", "--name", "Reader")
	cli.hn(repo, true, "issue", "open", "First issue")
	actor := cli.actor(repo)
	id := cli.latest(repo, actor)
	payload := cli.git(repo, "show", "refs/hn/actors/"+actor+":event.json")
	var event struct{ Kind, ActorName, Title string }
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("%s  %-13s  %-12s  %s\n", strings.TrimPrefix(id, "sha256:")[:12], event.Kind, event.ActorName, event.Title)
	if got := cli.hn(repo, true, "log"); got != expected {
		t.Fatalf("legacy log bytes=%q want=%q", got, expected)
	}
	before := consumerFiles(t, repo)
	logged := consumerJSONProcess(t, cli, repo, true, "log", "--json")
	var rows []struct {
		ID    string
		Event json.RawMessage
	}
	if err := json.Unmarshal(logged["events"], &rows); err != nil || len(rows) != 1 || rows[0].ID != id || !strings.Contains(string(rows[0].Event), event.Title) || string(logged["history_scope"]) != `"raw_signed_history"` {
		t.Fatalf("full raw log: %v %v", err, logged)
	}
	for _, action := range []string{"show", "resolve"} {
		full := consumerJSONProcess(t, cli, repo, true, "event", action, id, "--json")
		short := consumerJSONProcess(t, cli, repo, true, "event", action, strings.TrimPrefix(id, "sha256:")[:12], "--json")
		if string(full["id"]) != fmt.Sprintf("%q", id) || !reflect.DeepEqual(full, short) {
			t.Fatalf("prefix roundtrip: %v %v", full, short)
		}
	}
	if cli.hn(repo, true, "event", "resolve", strings.TrimPrefix(id, "sha256:")[:12]) != id+"\n" {
		t.Fatal("default resolver did not return full ID")
	}
	if !strings.Contains(cli.hn(repo, true, "event", "show", id), id) {
		t.Fatal("default event show omitted full ID")
	}
	if !reflect.DeepEqual(before, consumerFiles(t, repo)) {
		t.Fatal("read changed private or signed bytes")
	}
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"event", "show", "bad", "--json"}, "invalid_input"},
		{[]string{"event", "resolve", strings.Repeat("a", 12), "--json"}, "not_found"},
		{[]string{"event", "show", id, "--unknown", "--json"}, "invalid_input"},
		{[]string{"log", "--unknown", "--json"}, "invalid_input"},
	} {
		failure := consumerJSONProcess(t, cli, repo, false, tc.args...)
		if string(failure["code"]) != fmt.Sprintf("%q", tc.code) {
			t.Fatalf("error %v: %v", tc.args, failure)
		}
	}
	consumerJSONProcess(t, cli, t.TempDir(), false, "event", "show", id, "--json")
	// Convert fixture's signer to legacy-only without invoking a migrating command.
	gitdir := cli.git(repo, "rev-parse", "--absolute-git-dir")
	key, err := os.ReadFile(filepath.Join(gitdir, "hn", "identities", actor+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]json.RawMessage
	if err = json.Unmarshal(key, &record); err != nil {
		t.Fatal(err)
	}
	delete(record, "version")
	delete(record, "lifecycle")
	legacy, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(filepath.Join(gitdir, "hn", "identities")); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(gitdir, "hn", "active")); err != nil {
		t.Fatal(err)
	}
	betaContributionWrite(t, filepath.Join(gitdir, "hn", "identity.json"), legacy, 0600)
	before = consumerFiles(t, repo)
	consumerJSONProcess(t, cli, repo, true, "event", "show", id, "--json")
	consumerJSONProcess(t, cli, repo, true, "log", "--json")
	if !reflect.DeepEqual(before, consumerFiles(t, repo)) {
		t.Fatal("read migrated legacy identity")
	}
}

func TestConsumerEventResolverAmbiguity(t *testing.T) {
	// Deterministic resolver fixture, not forged accepted history or a hash search.
	prefix := "0123456789ab"
	rows := []StoredEvent{{ID: "sha256:" + prefix + strings.Repeat("0", 52)}, {ID: "sha256:" + prefix + strings.Repeat("1", 52)}}
	matched, err := resolveEvent(rows, prefix)
	code, ok := err.(interface{ NativeCode() string })
	if matched != nil || !ok || code.NativeCode() != "ambiguous_id" {
		t.Fatalf("ambiguous resolver must refuse distinctly: %v %v", matched, err)
	}
	out, replyErr := captureTestOutput(t, func() error { return consumerOutput(nil, err) })
	var envelope struct {
		OK    bool
		Data  any
		Error struct {
			Origin, Code, Recovery string
			Operation              any
		}
	}
	if replyErr == nil || json.Unmarshal([]byte(out), &envelope) != nil || envelope.OK || envelope.Data != nil || envelope.Error.Origin != "native" || envelope.Error.Code != "ambiguous_id" || envelope.Error.Recovery != "do_not_retry" || envelope.Error.Operation != nil {
		t.Fatalf("ambiguous resolver envelope: %v %s", replyErr, out)
	}
	if matched, err = resolveEvent(rows, rows[0].ID); err != nil || matched.ID != rows[0].ID {
		t.Fatalf("full identity resolves: %v %v", matched, err)
	}
}

func TestConsumerEventModerationCLI(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	repo := cli.repository()
	cli.hn(repo, true, "init", "--name", "Moderator")
	actor := cli.actor(repo)
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1, AllowAuthorApproval: true}, Pipelines: map[string]PipelinePolicy{"proof": {RequiredResults: 1, TrustedRunners: []string{actor}}}}
	raw, _ := json.Marshal(policy)
	betaContributionWrite(t, filepath.Join(repo, ".hn/policy.json"), raw, 0600)
	betaContributionWrite(t, filepath.Join(repo, ".hn/pipelines/proof.json"), []byte(`{"version":"hn.pipeline/0","steps":[{"name":"Git version","command":"git","args":["--version"]}]}`), 0600)
	cli.commit(repo, "Policy")
	base := cli.git(repo, "rev-parse", "HEAD")
	cli.commit(repo, "Candidate")
	head := cli.git(repo, "rev-parse", "HEAD")
	candidates := []string{}
	evidence := [][]string{}
	for _, title := range []string{"Admitted", "Unadmitted"} {
		cli.hn(repo, true, "proposal", "open", "--base", base, "--head", head, title)
		id := cli.latest(repo, actor)
		candidates = append(candidates, id)
		cli.hn(repo, true, "review", id, "--approve")
		review := cli.latest(repo, actor)
		cli.hn(repo, true, "run", "request", id, "proof")
		request := cli.latest(repo, actor)
		cli.hn(repo, true, "run", "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
		result := cli.latest(repo, actor)
		evidence = append(evidence, []string{review, request, result})
	}
	cli.hn(repo, true, "proposal", "revise", candidates[0], "--base", base, "--head", head, "--body", "Visible successor")
	successor := cli.latest(repo, actor)
	policy.Contributions = &c.Policy{Project: c.Digest([]byte(t.Name())), Moderators: []string{actor}, Baseline: []string{candidates[0], successor}, Limits: betaContributionLimits(), Rules: []c.Rule{{Operation: c.ProposalCreate}}}
	sort.Strings(policy.Contributions.Baseline)
	raw, _ = json.Marshal(policy)
	betaContributionWrite(t, filepath.Join(repo, ".hn/policy.json"), raw, 0600)
	cli.commit(repo, "Contribution authority")
	cli.hn(repo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	for _, id := range append([]string{candidates[0]}, evidence[0]...) {
		consumerJSONProcess(t, cli, repo, true, "event", "show", id, "--json")
	}
	for _, id := range append([]string{candidates[1]}, evidence[1]...) {
		if failure := consumerJSONProcess(t, cli, repo, false, "event", "show", id, "--json"); string(failure["code"]) != `"not_found"` {
			t.Fatalf("moderation must not reveal event: %v", failure)
		}
	}
	cli.decide(repo, "hide", "--revision", candidates[0], "absent", "hide-candidate")
	for _, id := range append([]string{candidates[0]}, evidence[0]...) {
		if failure := consumerJSONProcess(t, cli, repo, false, "event", "show", id, "--json"); string(failure["code"]) != `"not_found"` {
			t.Fatalf("moderation must not reveal event: %v", failure)
		}
	}
	// A visible successor remains readable when its predecessor is hidden:
	// the immutable Subject is lineage, not an evidence authorization parent.
	cli.hn(repo, true, "proposal", "show", successor)
	consumerJSONProcess(t, cli, repo, true, "event", "show", successor, "--json")
	rawLog := consumerJSONProcess(t, cli, repo, true, "log", "--json")
	for _, id := range append(evidence[0], evidence[1]...) {
		if !strings.Contains(string(rawLog["events"]), id) {
			t.Fatalf("raw history lost immutable evidence: %s", id)
		}
	}
	if !strings.HasPrefix(cli.hn(repo, true, "log"), "Raw signed history (includes unadmitted and hidden facts):\n") {
		t.Fatal("default raw banner changed")
	}
}
