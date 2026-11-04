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

// This fast-tier journey executes only a tiny fixture action. It must never
// invoke the project's own fast pipeline recursively. Expectations below come
// from the public runner contract, not scheduler or withdrawal internals.
func TestRunnerEfficiencyPublicJourney(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, true))
	owner := cli.repository()
	cli.git(owner, "config", "maintenance.auto", "false")
	cli.git(owner, "config", "gc.auto", "0")
	cli.hn(owner, true, "init", "--name", "Requester")
	requester := cli.actor(owner)
	if _, err := os.Stat(filepath.Join(owner, ".hn/policy.json")); !os.IsNotExist(err) {
		t.Fatal("init activated policy")
	}
	remote := filepath.Join(t.TempDir(), "project.git")
	cli.git(owner, "clone", "--quiet", "--bare", owner, remote)
	cli.git(owner, "remote", "add", "origin", remote)
	workers := []string{}
	actors := []string{}
	for _, name := range []string{"Runner A", "Runner B"} {
		repo := filepath.Join(t.TempDir(), "runner")
		cli.git(owner, "clone", "--quiet", remote, repo)
		cli.git(repo, "config", "maintenance.auto", "false")
		cli.git(repo, "config", "gc.auto", "0")
		cli.git(repo, "config", "user.name", name)
		cli.git(repo, "config", "user.email", "runner@example.invalid")
		cli.hn(repo, true, "init", "--name", name)
		workers = append(workers, repo)
		actors = append(actors, cli.actor(repo))
	}
	if actors[0] == actors[1] || actors[0] == requester || actors[1] == requester {
		t.Fatal("journey requires three distinct actors")
	}
	marker := filepath.Join(t.TempDir(), "backend-marker")
	betaContributionWrite(t, marker, nil, 0600)
	// Fixture input uses the documented wire schema, independent of production
	// policy/evidence evaluation and withdrawal construction helpers.
	policy := map[string]any{"version": "hn.policy/0", "maintainers": []string{requester}, "proposals": map[string]any{"requiredAccepts": 1}, "pipelines": map[string]any{"proof": map[string]any{"requiredResults": 2, "trustedRunners": actors}}}
	raw, _ := json.Marshal(policy)
	betaContributionWrite(t, filepath.Join(owner, ".hn/policy.json"), raw, 0644)
	pipeline := map[string]any{"version": "hn.pipeline/0", "steps": []any{map[string]any{"name": "Record exact execution", "command": "./.hn/actions/proof", "args": []string{marker}}}}
	raw, _ = json.Marshal(pipeline)
	definition := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	betaContributionWrite(t, filepath.Join(owner, ".hn/pipelines/proof.json"), raw, 0644)
	betaContributionWrite(t, filepath.Join(owner, ".hn/actions/proof"), []byte("#!/bin/sh\nprintf 'executed\\n' >> \"$1\"\n"), 0755)
	cli.commit(owner, "Explicit native policy and tiny pipeline")
	base := cli.git(owner, "rev-parse", "HEAD")
	cli.git(owner, "push", "--quiet", "origin", "main")
	cli.git(owner, "switch", "--quiet", "-c", "candidate")
	cli.commit(owner, "Candidate")
	head := cli.git(owner, "rev-parse", "HEAD")
	cli.hn(owner, true, "proposal", "open", "--base", base, "--head", head, "Runner efficiency acceptance")
	proposal, _ := runnerAcceptanceHead(t, cli, owner, requester)
	candidateHeads := map[string]string{proposal: head}
	request := func(candidate string) string {
		t.Helper()
		id := publicRunRequestID(t, cli.hn(owner, true, "run", "request", candidate, "proof"))
		actual, payload := runnerAcceptanceHead(t, cli, owner, requester)
		var fact struct{ Kind, Subject, Pipeline, Definition, Commit string }
		if err := json.Unmarshal(payload, &fact); err != nil {
			t.Fatal(err)
		}
		if actual != id || fact.Kind != "run.request" || fact.Subject != candidate || fact.Pipeline != "proof" || fact.Definition != definition || fact.Commit != candidateHeads[candidate] {
			t.Fatalf("request did not bind exact independently observed tuple: %s", payload)
		}
		if publicRunRequestID(t, cli.hn(owner, true, "run", "show", id)) != id {
			t.Fatal("show lost full Request identity")
		}
		return id
	}
	syncAll := func() {
		t.Helper()
		cli.hn(owner, true, "sync", "origin")
		for _, worker := range workers {
			cli.hn(worker, true, "sync", "origin")
		}
		cli.hn(owner, true, "sync", "origin")
		for _, worker := range workers {
			cli.hn(worker, true, "sync", "origin")
		}
	}
	verifyResult := func(worker, id, commit string) {
		t.Helper()
		actor := cli.actor(worker)
		_, payload := runnerAcceptanceHead(t, cli, worker, actor)
		var fact struct {
			Kind, Subject, Pipeline, Definition, Commit, Outcome, Log string
			ExitCode                                                  int
		}
		if err := json.Unmarshal(payload, &fact); err != nil {
			t.Fatal(err)
		}
		if fact.Kind != "run.result" || fact.Subject != id || fact.Pipeline != "proof" || fact.Definition != definition || fact.Commit != commit || fact.Outcome != "passed" || fact.ExitCode != 0 {
			t.Fatalf("result did not bind exact independently observed tuple: %s", payload)
		}
		resultCommit := cli.git(worker, "rev-parse", "refs/hn/actors/"+actor)
		log := cli.process(worker, "git", true, "show", resultCommit+":log.txt")
		if fact.Log != fmt.Sprintf("sha256:%x", sha256.Sum256(log)) {
			t.Fatal("signed result digest differs from exact log bytes")
		}
	}
	execute := func(worker, id, commit string) {
		t.Helper()
		before, _ := os.ReadFile(marker)
		cli.hn(worker, true, "run", "execute", id, "--backend", "host", "--allow-unsafe-host-execution")
		after, _ := os.ReadFile(marker)
		if string(after) != string(before)+"executed\n" {
			t.Fatalf("eligible pipeline effect: before=%q after=%q", before, after)
		}
		verifyResult(worker, id, commit)
	}
	// Stable sync first, then snapshot private state, all Git refs/objects, remote
	// refs and marker before each no-effect probe. Nothing is reset for controls.
	unchanged := func(repo string, ok bool, args ...string) string {
		t.Helper()
		before := extensionPrivateSnapshot(t, repo)
		refs := cli.git(repo, "for-each-ref", "--format=%(refname) %(objectname)")
		objects := cli.git(repo, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)")
		remoteRefs := cli.git(owner, "--git-dir="+remote, "for-each-ref", "--format=%(refname) %(objectname)")
		prior, _ := os.ReadFile(marker)
		output := cli.hn(repo, ok, args...)
		after, _ := os.ReadFile(marker)
		afterPrivate := extensionPrivateSnapshot(t, repo)
		if len(args) > 0 && args[0] == "runner" {
			// runner once always performs normal sync, whose local transaction
			// receipts are permitted bookkeeping, not signer/backend effects.
			for _, snapshot := range []map[string]string{before, afterPrivate} {
				for path := range snapshot {
					if strings.HasPrefix(path, "/.git/hn/replication/transactions/") && strings.HasSuffix(path, ".json") {
						delete(snapshot, path)
					}
				}
			}
		}
		if !reflect.DeepEqual(before, afterPrivate) || refs != cli.git(repo, "for-each-ref", "--format=%(refname) %(objectname)") || objects != cli.git(repo, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)") || remoteRefs != cli.git(owner, "--git-dir="+remote, "for-each-ref", "--format=%(refname) %(objectname)") || string(prior) != string(after) {
			t.Fatalf("no-effect probe changed state: %q", args)
		}
		return output
	}
	automatic := func(worker string, reasons ...string) {
		t.Helper()
		output := unchanged(worker, true, "runner", "once", "--accept-pipeline", "proof", "--accept-actor", requester, "--backend", "host", "--allow-unsafe-host-execution")
		betaContributionContains(t, output, reasons...)
	}
	first, second := request(proposal), request(proposal)
	syncAll()
	execute(workers[0], first, head)
	execute(workers[1], second, head)
	syncAll()
	fresh := request(proposal)
	syncAll()
	// Runner B has no own result on first/fresh; qualifying evidence from TWO
	// trusted runners across exact requests is what suppresses these starts.
	automatic(workers[1], "pipeline satisfied by 2 qualifying trusted results")
	execute(workers[1], fresh, head) // explicit new intent remains executable
	syncAll()
	cli.commit(owner, "Revised candidate")
	revisedHead := cli.git(owner, "rev-parse", "HEAD")
	revisionOutput := cli.hn(owner, true, "proposal", "revise", proposal, "--base", base, "--head", revisedHead)
	revision, _ := runnerAcceptanceHead(t, cli, owner, requester)
	candidateHeads[revision] = revisedHead
	betaContributionContains(t, revisionOutput, revision, proposal)
	queued := request(revision)
	syncAll()
	betaContributionContains(t, unchanged(workers[1], false, "run", "withdraw", queued, "--reason", "owner only"), "original requester")
	receipt := betaContributionJSON[withdrawalCLIReceipt](t, cli.hn(owner, true, "run", "withdraw", queued, "--reason", "  superseded queue intent  ", "--json"))
	if receipt.RequestID != queued || receipt.Replayed || receipt.Publication != "explicit-sync" {
		t.Fatal("withdrawal receipt identity/publication")
	}
	withdrawalCommit := cli.git(owner, "rev-parse", "refs/hn/actors/"+requester)
	signedWithdrawal := cli.process(owner, "git", true, "show", withdrawalCommit+":event.json")
	signedSignature := cli.process(owner, "git", true, "show", withdrawalCommit+":signature")
	replay := betaContributionJSON[withdrawalCLIReceipt](t, unchanged(owner, true, "run", "withdraw", queued, "--reason", "superseded queue intent", "--json"))
	if !replay.Replayed || replay.WithdrawalID != receipt.WithdrawalID {
		t.Fatal("normalized retry appended intent")
	}
	betaContributionContains(t, unchanged(owner, false, "run", "withdraw", queued, "--reason", "different intent"), "different reason")
	syncAll()
	betaContributionContains(t, cli.hn(workers[1], true, "run", "show", queued), "Status: withdrawn", queued, receipt.WithdrawalID)
	betaContributionContains(t, unchanged(workers[1], false, "run", "execute", queued, "--backend", "host", "--allow-unsafe-host-execution"), "withdrawn", receipt.WithdrawalID)
	// This revision has no results, is current and unmerged: withdrawal cannot
	// be masked by own-result, obsolete-lineage or satisfied-threshold guards.
	automatic(workers[1], "superseded", "withdrawn")
	control := request(revision)
	syncAll()
	execute(workers[0], control, revisedHead)
	execute(workers[1], control, revisedHead)
	request(revision) // untouched request exposes merged guard instead of own-result filtering
	syncAll()
	cli.git(owner, "switch", "--quiet", "main")
	cli.git(owner, "config", "user.name", "Requester")
	cli.git(owner, "config", "user.email", "requester@example.invalid")
	cli.hn(owner, true, "decide", revision, "--accept")
	cli.hn(owner, true, "merge", revision)
	syncAll()
	// This is combined-state observation (merged and threshold satisfied).
	// TestRunnerEfficiencyLineage supplies the isolated empty-policy proof.
	automatic(workers[1], "lineage closed", "merged")
	// Independent useful proposal after every merged-lineage assertion proves
	// a real automatic start still works from the same accumulated state.
	cli.git(owner, "switch", "--quiet", "-c", "next")
	nextBase := cli.git(owner, "rev-parse", "HEAD")
	cli.commit(owner, "Next candidate")
	cli.hn(owner, true, "proposal", "open", "--base", nextBase, "--head", "HEAD", "Next useful work")
	next, _ := runnerAcceptanceHead(t, cli, owner, requester)
	nextHead := cli.git(owner, "rev-parse", "HEAD")
	candidateHeads[next] = nextHead
	nextRequest := request(next)
	syncAll()
	before, _ := os.ReadFile(marker)
	cli.hn(workers[1], true, "runner", "once", "--accept-pipeline", "proof", "--accept-actor", requester, "--backend", "host", "--allow-unsafe-host-execution")
	after, _ := os.ReadFile(marker)
	if string(after) != string(before)+"executed\n" {
		t.Fatal("automatic positive control did not execute exactly one tiny action")
	}
	verifyResult(workers[1], nextRequest, nextHead)
	syncAll()
	// Explicitly select all three accepted actor histories and proposal refs: an archive must
	// refuse omitted cross-actor decision dependencies rather than invent facts.
	// Preserve every selected event/signature as exact immutable Git bytes.
	historyRefs := []string{"refs/hn/actors/" + requester, "refs/hn/remotes/origin/actors/" + actors[0], "refs/hn/remotes/origin/actors/" + actors[1]}
	signedHistory := map[string][]byte{}
	for _, ref := range historyRefs {
		for _, commit := range strings.Fields(cli.git(owner, "rev-list", ref)) {
			for _, name := range []string{"event.json", "signature"} {
				key := commit + ":" + name
				signedHistory[key] = cli.process(owner, "git", true, "show", key)
			}
		}
	}
	archive := filepath.Join(t.TempDir(), "project.hn")
	exportSession := filepath.Join(t.TempDir(), "export")
	exportArgs := []string{"migrate", "plan", "--source-kind", "local", "--source-repository", owner, "--ref", "refs/heads/main"}
	for _, candidate := range []string{proposal, revision, next} {
		exportArgs = append(exportArgs, "--ref", "refs/hn/proposals/"+strings.TrimPrefix(candidate, "sha256:"))
	}
	for _, ref := range historyRefs {
		exportArgs = append(exportArgs, "--ref", ref)
	}
	exportArgs = append(exportArgs, "--destination-kind", "archive", "--destination-repository", archive, "--session", exportSession)
	cli.hn(owner, true, exportArgs...)
	cli.hn(owner, true, "migrate", "apply", "--session", exportSession, "--confirm-apply")
	destination := cli.repository()
	destinationHead := cli.git(destination, "rev-parse", "HEAD")
	importSession := filepath.Join(t.TempDir(), "import")
	cli.hn(owner, true, "migrate", "plan", "--source-kind", "archive", "--source-repository", archive, "--destination-kind", "local", "--destination-repository", destination, "--decision", "local/governance=preserve", "--session", importSession)
	cli.hn(owner, true, "migrate", "apply", "--session", importSession, "--confirm-apply")
	if cli.git(destination, "rev-parse", "HEAD") != destinationHead {
		t.Fatal("archive adopted code checkout")
	}
	for key, want := range signedHistory {
		if got := cli.process(destination, "git", true, "show", key); !bytes.Equal(got, want) {
			t.Fatalf("archive changed signed history %s", key)
		}
	}
	if !bytes.Equal(cli.process(destination, "git", true, "show", withdrawalCommit+":event.json"), signedWithdrawal) || !bytes.Equal(cli.process(destination, "git", true, "show", withdrawalCommit+":signature"), signedSignature) {
		t.Fatal("archive changed signed withdrawal bytes")
	}
	betaContributionContains(t, cli.hn(destination, true, "run", "show", queued), "Status: withdrawn", queued, receipt.WithdrawalID)
	betaContributionContains(t, cli.hn(destination, true, "proposal", "status", revision), "merged")
	if _, err := os.Stat(filepath.Join(destination, ".git/hn/active")); !os.IsNotExist(err) {
		t.Fatal("archive installed signer")
	}
	if _, err := os.Stat(filepath.Join(destination, ".hn/policy.json")); !os.IsNotExist(err) {
		t.Fatal("archive activated policy")
	}
	cli.hn(destination, true, "init", "--name", "New local author")
	local := cli.actor(destination)
	if local == requester || local == actors[0] || local == actors[1] {
		t.Fatal("archive imported signing authority")
	}
	betaContributionContains(t, unchanged(destination, false, "decide", next, "--accept"), "maintainer")
	cli.hn(destination, true, "issue", "open", "New authorized local work after archive")
	betaContributionContains(t, cli.hn(destination, true, "issue", "list"), "New authorized local work after archive")
	t.Logf("requester=%s runners=%q first=%s second=%s fresh=%s revision=%s queued=%s withdrawal=%s", requester, actors, first, second, fresh, revision, queued, receipt.WithdrawalID)
}

// Independent immutable Git observation, used only to identify/compare signed
// facts where the proposal CLI prints an abbreviated identifier.
func runnerAcceptanceHead(t *testing.T, cli *betaContributionCLI, repo, actor string) (string, []byte) {
	t.Helper()
	raw := cli.process(repo, "git", true, "show", "refs/hn/actors/"+actor+":event.json")
	return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), raw
}

// Keep the fixed moderated legacy-identity refusal in bounded routine CI without
// selecting the complete withdrawal matrix. Critical validation still executes
// the existing public helper in both actual executable variants.
func TestRunnerEfficiencyModeratedLegacyRefusal(t *testing.T) {
	cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, false))
	runWithdrawalModeration(cli)
}

func runnerAcceptanceBinary(t *testing.T, native bool) string {
	t.Helper()
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	gitDir, err := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
	if err == nil {
		t.Setenv("GIT_DIR", strings.TrimSpace(string(gitDir)))
		t.Setenv("GIT_WORK_TREE", source)
	} else {
		t.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -buildvcs=false"))
		t.Log("metadata-free source snapshot: child compilation uses -buildvcs=false")
	}
	// Choose the child's actual tags explicitly: parent go test tags do not
	// propagate to go build. Native-only keeps this representative check small.
	binary := betaContributionBinary(t, native)
	tags := ""
	if native {
		tags = "nogithub,noforge"
	}
	t.Logf("source=%s; explicit child tags=%q; compiled source, not release certification", source, tags)
	return binary
}
