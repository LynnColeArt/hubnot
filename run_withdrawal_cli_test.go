package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	c "hubnot/internal/contribution"
)

func TestRunWithdrawalCLI(t *testing.T) {
	// Go's VCS discovery can select the enclosing primary repository for a lane.
	// Reuse the established source-binding pattern without changing CLI fixture
	// environments: only compilation receives the registered source Git context.
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	git := exec.Command("git", "rev-parse", "--absolute-git-dir")
	gitDir, err := git.Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", strings.TrimSpace(string(gitDir)))
	t.Setenv("GIT_WORK_TREE", source)

	for _, variant := range []struct{ name, tags string }{{"standard", ""}, {"native-only", "nogithub,noforge"}} {
		t.Run(variant.name, func(t *testing.T) {
			binary := betaContributionBinary(t, variant.tags != "")
			t.Logf("actual CLI variant=%s explicit build tags=%q", variant.name, variant.tags)
			t.Run("discovery", func(t *testing.T) { runWithdrawalDiscovery(newBetaContributionCLI(t, binary)) })
			t.Run("withdraw-retry-sync", func(t *testing.T) { runWithdrawalPublicJourney(newBetaContributionCLI(t, binary)) })
			t.Run("historical-and-fresh-intent", func(t *testing.T) { runWithdrawalFreshIntent(newBetaContributionCLI(t, binary)) })
			t.Run("moderation-and-contention", func(t *testing.T) { runWithdrawalModeration(newBetaContributionCLI(t, binary)) })
			t.Run("legacy-refusal-retry", func(t *testing.T) { runWithdrawalLegacyRetry(newBetaContributionCLI(t, binary)) })
		})
	}
}
func runWithdrawalDiscovery(cli *betaContributionCLI) {
	t := cli.t
	outside := t.TempDir()
	before := extensionPrivateSnapshot(t, outside)
	marker := filepath.Join(t.TempDir(), "git-marker")
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "git"), []byte("#!/bin/sh\nprintf 'git invoked' >> \"$HN_DISCOVERY_MARKER\"\nexit 88\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cli.env = append(cli.env, "PATH="+path, "HN_DISCOVERY_MARKER="+marker)
	help := cli.hn(outside, true, "run", "help")
	betaContributionContains(t, help, "run withdraw", "--reason", "future", "sync", "older")
	raw := cli.hn(outside, true, "run", "capabilities", "--json")
	var capability struct {
		Schema          string `json:"schema"`
		GrantsAuthority bool   `json:"grants_authority"`
		Withdrawal      struct {
			EventKind             string `json:"event_kind"`
			Scope                 string `json:"scope"`
			CancelsRunning        bool   `json:"cancels_running"`
			RequesterOnly         bool   `json:"requester_only"`
			ReasonMaxBytes        int    `json:"reason_max_bytes"`
			OlderReadersMayReject bool   `json:"older_readers_may_reject"`
		} `json:"withdrawal"`
	}
	if err := json.Unmarshal([]byte(raw), &capability); err != nil {
		t.Fatal(err)
	}
	if capability.Schema != "hn.run-capabilities/1" || capability.GrantsAuthority || capability.Withdrawal.EventKind != "run.withdraw" || capability.Withdrawal.Scope != "future-starts" || capability.Withdrawal.CancelsRunning || !capability.Withdrawal.RequesterOnly || capability.Withdrawal.ReasonMaxBytes != 4096 || !capability.Withdrawal.OlderReadersMayReject {
		t.Fatalf("wrong static capability semantics: %s", raw)
	}
	if after := extensionPrivateSnapshot(t, outside); !reflect.DeepEqual(before, after) {
		t.Fatal("static discovery created native state")
	}
	got, err := os.ReadFile(marker)
	if err != nil || strings.TrimSpace(string(got)) != "" {
		t.Fatalf("static discovery invoked Git: %q %v", got, err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("static discovery created files outside repository")
	}
}

// Independent public wire mirror; changing production receipt field names must
// not silently update the contract assertions.
type withdrawalCLIReceipt struct {
	Schema       string `json:"schema"`
	RequestID    string `json:"request_id"`
	WithdrawalID string `json:"withdrawal_id"`
	Replayed     bool   `json:"replayed"`
	Publication  string `json:"publication"`
}

type withdrawalCLIFixture struct {
	cli                                                        *betaContributionCLI
	repo, remote, marker, actor, base, head, proposal, request string
}

func newWithdrawalCLI(cli *betaContributionCLI, governed bool) *withdrawalCLIFixture {
	t := cli.t
	f := &withdrawalCLIFixture{cli: cli, repo: cli.repository()}
	cli.git(f.repo, "config", "maintenance.auto", "false")
	cli.git(f.repo, "config", "gc.auto", "0")
	cli.hn(f.repo, true, "init", "--name", "Original requester")
	f.actor = cli.actor(f.repo)
	f.marker = filepath.Join(t.TempDir(), "backend-marker")
	betaContributionWrite(t, f.marker, nil, 0600)
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{f.actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}}
	if governed {
		policy.Pipelines["proof"] = PipelinePolicy{RequiredResults: 1, TrustedRunners: []string{f.actor}}
	}
	writeTestPolicy(t, f.repo, policy)
	pipeline := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "Record backend effect", Command: "./.hn/actions/proof", Args: []string{f.marker}}}}
	raw, err := json.Marshal(pipeline)
	if err != nil {
		t.Fatal(err)
	}
	betaContributionWrite(t, filepath.Join(f.repo, ".hn/pipelines/proof.json"), raw, 0644)
	betaContributionWrite(t, filepath.Join(f.repo, ".hn/actions/proof"), []byte("#!/bin/sh\nprintf 'executed\\n' >> \"$1\"\n"), 0755)
	cli.commit(f.repo, "Native policy and action")
	f.base = cli.git(f.repo, "rev-parse", "HEAD")
	cli.commit(f.repo, "Candidate")
	f.head = cli.git(f.repo, "rev-parse", "HEAD")
	cli.hn(f.repo, true, "proposal", "open", "--base", f.base, "--head", f.head, "Withdrawal CLI candidate")
	f.proposal, _ = f.actorHead(f.repo, f.actor, "proposal.open")
	f.request = f.newRequest()
	f.remote = filepath.Join(t.TempDir(), "project.git")
	cli.git(f.repo, "clone", "--quiet", "--bare", f.repo, f.remote)
	cli.git(f.repo, "remote", "add", "origin", f.remote)
	return f
}
func (f *withdrawalCLIFixture) actorHead(repo, actor, kind string) (string, string) {
	f.cli.t.Helper()
	raw := f.cli.git(repo, "show", "refs/hn/actors/"+actor+":event.json")
	var event Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		f.cli.t.Fatal(err)
	}
	if event.Kind != kind {
		f.cli.t.Fatalf("head kind=%s, want%s", event.Kind, kind)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(raw))), raw
}
func publicRunRequestID(t *testing.T, output string) string {
	t.Helper()
	var full string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Request: ") {
			if full != "" {
				t.Fatal("duplicate full request identity")
			}
			full = strings.TrimPrefix(line, "Request: ")
		}
	}
	if !validEventID(full) {
		t.Fatalf("public output omitted a usable full request ID before withdrawal: %s", output)
	}
	return full
}
func (f *withdrawalCLIFixture) newRequest() string {
	f.cli.t.Helper()
	output := f.cli.hn(f.repo, true, "run", "request", f.proposal, "proof")
	id := publicRunRequestID(f.cli.t, output)
	// A user can also resolve a known prefix through the existing read command;
	// the pending request must expose the full writer identifier before withdrawal.
	shown := f.cli.hn(f.repo, true, "run", "show", shortID(id))
	if fromShow := publicRunRequestID(f.cli.t, shown); fromShow != id {
		f.cli.t.Fatal("pending show changed full request identity")
	}
	actual, _ := f.actorHead(f.repo, f.actor, "run.request")
	if id != actual {
		f.cli.t.Fatal("public request identifier differs from actual signed event")
	}
	return id
}
func (f *withdrawalCLIFixture) cloneWorker() string {
	f.cli.t.Helper()
	f.cli.hn(f.repo, true, "sync", "origin")
	repo := filepath.Join(f.cli.t.TempDir(), "worker")
	f.cli.git(f.repo, "clone", "--quiet", f.remote, repo)
	f.cli.git(repo, "config", "maintenance.auto", "false")
	f.cli.git(repo, "config", "gc.auto", "0")
	f.cli.hn(repo, true, "init", "--name", "Independent worker")
	f.cli.hn(repo, true, "sync", "origin")
	f.cli.hn(repo, true, "sync", "origin")
	return repo
}
func (f *withdrawalCLIFixture) unchanged(repo string, ok bool, args ...string) string {
	f.cli.t.Helper()
	before := extensionPrivateSnapshot(f.cli.t, repo)
	marker, err := os.ReadFile(f.marker)
	if err != nil {
		f.cli.t.Fatal(err)
	}
	output := f.cli.hn(repo, ok, args...)
	if after := extensionPrivateSnapshot(f.cli.t, repo); !reflect.DeepEqual(before, after) {
		f.cli.t.Fatalf("%q changed repository/objects/signer", args)
	}
	if after, err := os.ReadFile(f.marker); err != nil || !reflect.DeepEqual(marker, after) {
		f.cli.t.Fatalf("%q changed backend marker", args)
	}
	return output
}
func (f *withdrawalCLIFixture) execute(repo, request string) {
	f.cli.t.Helper()
	before, err := os.ReadFile(f.marker)
	if err != nil {
		f.cli.t.Fatal(err)
	}
	f.cli.hn(repo, true, "run", "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
	after, err := os.ReadFile(f.marker)
	if err != nil || len(after) <= len(before) || !strings.HasPrefix(string(after), string(before)) {
		f.cli.t.Fatal("eligible actual pipeline did not append marker")
	}
}
func (f *withdrawalCLIFixture) withdraw(reason string) withdrawalCLIReceipt {
	f.cli.t.Helper()
	raw := f.cli.hn(f.repo, true, "run", "withdraw", f.request, "--reason", reason, "--json")
	receipt := betaContributionJSON[withdrawalCLIReceipt](f.cli.t, raw)
	if receipt.Schema != "hn.run-withdrawal/1" || receipt.RequestID != f.request || !validEventID(receipt.WithdrawalID) || receipt.Replayed || receipt.Publication != "explicit-sync" {
		f.cli.t.Fatalf("bad withdrawal receipt: %s", raw)
	}
	id, payload := f.actorHead(f.repo, f.actor, "run.withdraw")
	var event Event
	_ = json.Unmarshal([]byte(payload), &event)
	if id != receipt.WithdrawalID || event.Subject != f.request || event.Body != strings.TrimSpace(reason) {
		f.cli.t.Fatal("receipt does not bind actual signed withdrawal")
	}
	return receipt
}
func runWithdrawalPublicJourney(cli *betaContributionCLI) {
	t := cli.t
	f := newWithdrawalCLI(cli, false)
	for _, args := range [][]string{{f.request}, {f.request, "--reason", ""}, {f.request, "--reason", " \t\n"}, {f.request, "--reason", strings.Repeat("é", 2049)}, {f.request, "--reason", string([]byte{0xff})}, {shortID(f.request), "--reason", "bad"}, {f.proposal, "--reason", "wrong kind"}, {"sha256:" + strings.Repeat("f", 64), "--reason", "missing"}, {f.request, "--reason", "valid", "extra"}, {f.request, "--unexpected", "flag"}} {
		f.unchanged(f.repo, false, append([]string{"run", "withdraw"}, args...)...)
	}
	worker := f.cloneWorker()
	betaContributionContains(t, f.unchanged(worker, false, "run", "withdraw", f.request, "--reason", "moved elsewhere"), "original requester")
	remoteBefore := cli.git(f.repo, "--git-dir="+f.remote, "for-each-ref", "--format=%(refname) %(objectname)")
	receipt := f.withdraw(" \n moved elsewhere \t")
	if remoteAfter := cli.git(f.repo, "--git-dir="+f.remote, "for-each-ref", "--format=%(refname) %(objectname)"); remoteAfter != remoteBefore {
		t.Fatal("withdrawal published without explicit sync")
	}
	// Advance the requester chain, then retry in new real CLI processes.
	cli.hn(f.repo, true, "issue", "open", "--json", "Same-actor continuation")
	retry := betaContributionJSON[withdrawalCLIReceipt](t, f.unchanged(f.repo, true, "run", "withdraw", f.request, "--reason", "moved elsewhere", "--json"))
	if !retry.Replayed || retry.WithdrawalID != receipt.WithdrawalID || retry.RequestID != f.request {
		t.Fatal("restart retry signed new intention")
	}
	betaContributionContains(t, f.unchanged(f.repo, true, "run", "withdraw", f.request, "--reason", " moved elsewhere "), receipt.WithdrawalID, f.request, "sync REMOTE")
	betaContributionContains(t, f.unchanged(f.repo, false, "run", "withdraw", f.request, "--reason", "changed reason"), "different reason")
	betaContributionContains(t, f.unchanged(f.repo, true, "run", "list"), "withdrawn", receipt.WithdrawalID, f.request)
	shown := f.unchanged(f.repo, true, "run", "show", f.request)
	betaContributionContains(t, shown, "Status: withdrawn", receipt.WithdrawalID, f.request, "moved elsewhere")
	if publicRunRequestID(t, shown) != f.request {
		t.Fatal("withdrawn show changed or duplicated request identity")
	}
	cli.hn(f.repo, true, "sync", "origin")
	cli.hn(worker, true, "sync", "origin")
	cli.hn(worker, true, "sync", "origin")
	betaContributionContains(t, f.unchanged(worker, false, "run", "withdraw", f.request, "--reason", "moved elsewhere"), "original requester")
	betaContributionContains(t, f.unchanged(worker, false, "run", "execute", f.request, "--backend", "host", "--allow-unsafe-host-execution", "--rerun"), "withdrawn", receipt.WithdrawalID)
	before := cli.git(worker, "for-each-ref", "--format=%(refname) %(objectname)")
	objects := cli.git(worker, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)")
	betaContributionContains(t, cli.hn(worker, true, "runner", "once", "--accept-pipeline", "proof", "--accept-actor", f.actor, "--backend", "host", "--allow-unsafe-host-execution"), "withdrawn")
	if cli.git(worker, "for-each-ref", "--format=%(refname) %(objectname)") != before || cli.git(worker, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)") != objects {
		t.Fatal("automatic withdrawal skip appended signed facts/objects")
	}
	raw, _ := os.ReadFile(f.marker)
	if len(raw) != 0 {
		t.Fatal("refusal/automatic skip executed backend")
	}
	// Fresh explicit intent is the positive control after every no-effect check;
	// no marker or event state has been reset.
	fresh := f.newRequest()
	cli.hn(f.repo, true, "sync", "origin")
	cli.hn(worker, true, "sync", "origin")
	f.execute(worker, fresh)
}
func runWithdrawalFreshIntent(cli *betaContributionCLI) {
	t := cli.t
	f := newWithdrawalCLI(cli, true)
	f.execute(f.repo, f.request)
	resultID, resultPayload := f.actorHead(f.repo, f.actor, "run.result")
	resultCommit := cli.git(f.repo, "rev-parse", actorRef(f.actor))
	signature := cli.git(f.repo, "show", resultCommit+":signature")
	receipt := f.withdraw("No additional starts")
	fresh := f.newRequest()
	worker := f.cloneWorker()
	show := f.unchanged(f.repo, true, "run", "show", f.request)
	betaContributionContains(t, show, "withdrawn", receipt.WithdrawalID, "passed", shortID(resultID))
	before := cli.git(worker, "for-each-ref", "--format=%(refname) %(objectname)")
	marker, _ := os.ReadFile(f.marker)
	betaContributionContains(t, cli.hn(worker, true, "runner", "once", "--accept-pipeline", "proof", "--accept-actor", f.actor, "--backend", "host", "--allow-unsafe-host-execution"), "pipeline satisfied")
	if after, _ := os.ReadFile(f.marker); !reflect.DeepEqual(marker, after) || cli.git(worker, "for-each-ref", "--format=%(refname) %(objectname)") != before {
		t.Fatal("qualified automatic request executed")
	}
	f.execute(worker, fresh)
	if cli.git(f.repo, "show", resultCommit+":event.json") != resultPayload || cli.git(f.repo, "show", resultCommit+":signature") != signature {
		t.Fatal("withdrawal reinterpreted historical signed result")
	}
}
func runWithdrawalLegacyRetry(cli *betaContributionCLI) {
	t := cli.t
	f := newWithdrawalCLI(cli, false)
	receipt := f.withdraw("Legacy retry")
	path := filepath.Join(f.repo, ".git/hn/identities", f.actor+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record struct{ Actor, Name, PublicKey, PrivateKey string }
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	legacy, _ := json.Marshal(Identity{Actor: record.Actor, Name: record.Name, PublicKey: record.PublicKey, PrivateKey: record.PrivateKey})
	betaContributionWrite(t, filepath.Join(f.repo, ".git/hn/identity.json"), legacy, 0600)
	if err := os.RemoveAll(filepath.Join(f.repo, ".git/hn/identities")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.repo, ".git/hn/active")); err != nil {
		t.Fatal(err)
	}
	betaContributionContains(t, f.unchanged(f.repo, false, "run", "withdraw", f.request, "--reason", "changed"), "different reason")
	retry := betaContributionJSON[withdrawalCLIReceipt](t, f.unchanged(f.repo, true, "run", "withdraw", f.request, "--reason", "Legacy retry", "--json"))
	if !retry.Replayed || retry.WithdrawalID != receipt.WithdrawalID {
		t.Fatal("legacy replay changed withdrawal")
	}
	betaContributionContains(t, f.unchanged(f.repo, false, "run", "execute", f.request, "--backend", "host", "--allow-unsafe-host-execution", "--rerun"), "withdrawn")
	fresh := f.newRequest()
	f.execute(f.repo, fresh)
}
func runWithdrawalModeration(cli *betaContributionCLI) {
	t := cli.t
	f := newWithdrawalCLI(cli, false)
	// The requester is a native maintainer but is deliberately not a moderator.
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{f.actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte(t.Name())), Moderators: []string{strings.Repeat("b", 64)}, Baseline: []string{f.proposal}, Limits: betaContributionLimits(), Rules: []c.Rule{{Operation: c.ProposalCreate}}}}
	writeTestPolicy(t, f.repo, policy)
	cli.commit(f.repo, "Optional contribution authority")
	cli.hn(f.repo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	authorityPath := filepath.Join(f.repo, ".git/hn/contribution/authority.json")
	authority, err := os.ReadFile(authorityPath)
	if err != nil {
		t.Fatal(err)
	}
	// The exact static forms work with the operation lock held AND an unreadable
	// authority document, proving neither mutation gate is consulted.
	lockPath := filepath.Join(f.repo, ".git/hn/contribution/operation.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	betaContributionWrite(t, authorityPath, []byte("malformed authority"), 0600)
	f.unchanged(f.repo, true, "run", "help")
	f.unchanged(f.repo, true, "run", "capabilities", "--json")
	betaContributionContains(t, cli.hn(f.repo, false, "run", "withdraw", f.request, "--reason", "locked"), "busy")
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	betaContributionWrite(t, authorityPath, authority, 0600)
	betaContributionContains(t, f.unchanged(f.repo, false, "runner", "once", "--accept-pipeline", "proof", "--accept-actor", f.actor, "--backend", "host", "--allow-unsafe-host-execution"), "runner is unsupported")
	betaContributionContains(t, f.unchanged(f.repo, false, "run", "list"), "raw history")
	receipt := f.withdraw("Requester-only cancellation, no moderation grant")
	betaContributionContains(t, f.unchanged(f.repo, true, "run", "show", f.request), "withdrawn", receipt.WithdrawalID)
}
