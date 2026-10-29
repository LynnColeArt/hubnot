package main

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func contributionCLI(t *testing.T, binary, dir string, ok bool, args ...string) string {
	t.Helper()
	return contributionCLIEnvironment(t, binary, dir, nil, ok, args...)
}
func contributionCLIEnvironment(t *testing.T, binary, dir string, environment map[string]string, ok bool, args ...string) string {
	t.Helper()
	if binary == os.Getenv("HN_CONTRIBUTION_TEST_BINARY") {
		if err := contributionVerifyCandidate(binary, os.Getenv("HN_CONTRIBUTION_TEST_BINARY_SHA256")); err != nil {
			t.Fatal(err)
		}
	}
	p := exec.Command(binary, args...)
	p.Dir = dir
	p.Env = operationalTestEnvironmentWith(t.TempDir(), environment)
	out, err := p.CombinedOutput()
	if evidence := os.Getenv("HN_CONTRIBUTION_TEST_TRANSCRIPTS"); evidence != "" {
		file, e := os.CreateTemp(evidence, "cli-*.log")
		if e != nil {
			t.Fatal(e)
		}
		_, e = fmt.Fprintf(file, "test: %s\nbinary: %s\ncwd: %s\nargv: %q\nexit: %v\noutput:\n%s", t.Name(), binary, dir, args, err, out)
		closeErr := file.Close()
		if e != nil {
			t.Fatal(e)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if (err == nil) != ok {
		t.Fatalf("hn %v: %v\n%s", args, err, out)
	}
	return string(out)
}
func contributionGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	p := exec.Command("git", args...)
	p.Dir = dir
	p.Env = operationalTestEnvironment(t.TempDir())
	out, e := p.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, out)
	}
	return strings.TrimSpace(string(out))
}
func TestContributionCLIAutomaticAndScopedModeration(t *testing.T) {
	binary := contributionCLIBinary(t)
	owner := migrationCLIRepository(t)
	stranger := migrationCLIRepository(t)
	contributionCLI(t, binary, owner, true, "init", "--name", "Maintainer")
	var identity struct {
		Data struct {
			Actor string `json:"actor"`
		} `json:"data"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "identity", "show", "--json")), &identity); e != nil {
		t.Fatal(e)
	}
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{identity.Data.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("public journey")), Moderators: []string{identity.Data.Actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}, {Operation: c.IssueReply, Automatic: true}, {Operation: c.IssueEdit, Automatic: true}, {Operation: c.ProposalCreate}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policy.Contributions.Limits.WireBytes = 128 << 10
	policy.Contributions.Limits.DecodedBytes = 1 << 20
	policy.Contributions.Limits.Objects = 1024
	policy.Contributions.Limits.Events = 256
	raw, _ := json.Marshal(policy)
	if e := os.MkdirAll(filepath.Join(owner, ".hn"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(owner, ".hn/policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	contributionGit(t, owner, "add", ".hn/policy.json")
	contributionGit(t, owner, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Explicit project policy")
	contributionCLI(t, binary, owner, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	contributionCLI(t, binary, stranger, true, "init", "--name", "Stranger")
	output := contributionCLI(t, binary, stranger, true, "issue", "open", "--json", "Stranger issue")
	var issue struct {
		Data struct {
			EventID string `json:"event_id"`
		} `json:"data"`
	}
	if e := json.Unmarshal([]byte(output), &issue); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "issue.hnc")
	prepared := contributionCLI(t, binary, stranger, true, "contribution", "prepare", "--project", policy.Contributions.Project, "--revision", issue.Data.EventID, "--output", file, "--json")
	var artifact struct {
		Digest string `json:"digest"`
	}
	if e := json.Unmarshal([]byte(prepared), &artifact); e != nil {
		t.Fatal(e)
	}
	contributionCLI(t, binary, owner, true, "contribution", "receive", "--file", file, "--digest", artifact.Digest, "--operation", "issue-001", "--ref", actorRefFromCLI(t, binary, stranger), "--json")
	shown := contributionCLI(t, binary, owner, true, "issue", "list", "--json")
	if !strings.Contains(shown, "Stranger issue") {
		t.Fatal(shown)
	}
	beforeForeign := contributionGit(t, owner, "show-ref")
	foreign := contributionCLI(t, binary, owner, false, "issue", "close", issue.Data.EventID, "--expect", issue.Data.EventID, "--operation", "foreign-edit", "--json")
	if !strings.Contains(foreign, "original author required") {
		t.Fatal(foreign)
	}
	if beforeForeign != contributionGit(t, owner, "show-ref") {
		t.Fatal("foreign edit wrote native facts before permission refusal")
	}
	blockedActor := strings.TrimPrefix(actorRefFromCLI(t, binary, stranger), "refs/hn/actors/")
	blocked := contributionCLI(t, binary, owner, true, "contribution", "decide", "block", "--actor", blockedActor, "--expect-heads", "absent", "--json")
	var blockedDecision struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal([]byte(blocked), &blockedDecision); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("blocked author remained visible")
	}
	contributionCLI(t, binary, owner, true, "contribution", "decide", "unblock", "--actor", blockedActor, "--expect-heads", blockedDecision.ID, "--json")
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("unblock lost original receipt")
	}
	// A valid foreign signature cannot grant itself project moderation authority.
	keyRaw, e := os.ReadFile(filepath.Join(stranger, ".git/hn/identities/"+blockedActor+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var strangerIdentity Identity
	if e = json.Unmarshal(keyRaw, &strangerIdentity); e != nil {
		t.Fatal(e)
	}
	key, e := strangerIdentity.privateKey()
	if e != nil {
		t.Fatal(e)
	}
	unauthorized, e := c.Sign(c.Record{Version: c.Version, Kind: c.DecisionKind, Project: policy.Contributions.Project, Actor: blockedActor, PublicKey: strangerIdentity.PublicKey, Intent: "foreign-hide", Decision: &c.Decision{Action: c.Hide, Target: c.Target{Kind: c.RevisionTarget, ID: issue.Data.EventID}, Authority: c.AuthorityRef{Commit: contributionGit(t, owner, "rev-parse", "main"), Digest: c.Digest(raw)}, Parents: []string{}}}, key)
	if e != nil {
		t.Fatal(e)
	}
	hostile, digest := contributionRewriteArtifact(t, file, []c.Envelope{unauthorized})
	contributionCLI(t, binary, owner, true, "contribution", "receive", "--file", hostile, "--digest", digest, "--operation", "foreign-hide-intake", "--ref", actorRefFromCLI(t, binary, stranger), "--json")
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("foreign moderation changed shared visibility")
	}
	diagnostics := contributionCLI(t, binary, owner, true, "contribution", "pending", "--json")
	if !strings.Contains(diagnostics, c.Digest(unauthorized.Payload)) || !strings.Contains(diagnostics, "lacks evaluated moderator authority") {
		t.Fatal(diagnostics)
	}

	hide := contributionCLI(t, binary, owner, true, "contribution", "decide", "hide", "--revision", issue.Data.EventID, "--expect-heads", "absent", "--json")
	if strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("hidden body leaked")
	}
	if !strings.Contains(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--raw", issue.Data.EventID, "--json"), "Stranger issue") {
		t.Fatal("raw history lost")
	}
	var decision struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal([]byte(hide), &decision); e != nil {
		t.Fatal(e)
	}
	contributionCLI(t, binary, owner, true, "contribution", "decide", "restore", "--revision", issue.Data.EventID, "--expect-heads", decision.ID, "--json")
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("restore missing")
	}

	// Keep an effective old hide through the governed checkpoint transition.
	var restored struct {
		ID string `json:"id"`
	}
	beforeHide := contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")
	var allBefore struct {
		Records []struct {
			ID     string   `json:"id"`
			Record c.Record `json:"record"`
		} `json:"records"`
	}
	if e := json.Unmarshal([]byte(beforeHide), &allBefore); e != nil {
		t.Fatal(e)
	}
	for _, row := range allBefore.Records {
		if row.Record.Decision != nil && row.Record.Decision.Action == c.Restore {
			restored.ID = row.ID
		}
	}
	activeHideOutput := contributionCLI(t, binary, owner, true, "contribution", "decide", "hide", "--revision", issue.Data.EventID, "--expect-heads", restored.ID, "--operation", "hide-retained", "--json")
	var activeHide struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal([]byte(activeHideOutput), &activeHide); e != nil {
		t.Fatal(e)
	}
	refsBeforeRetry := contributionGit(t, owner, "show-ref")
	retry := contributionCLI(t, binary, owner, true, "contribution", "decide", "hide", "--revision", issue.Data.EventID, "--expect-heads", restored.ID, "--operation", "hide-retained", "--json")
	if !strings.Contains(retry, activeHide.ID) || !strings.Contains(retry, `"replayed":true`) {
		t.Fatal(retry)
	}
	if refsBeforeRetry != contributionGit(t, owner, "show-ref") {
		t.Fatal("exact decision replay changed refs")
	}
	contributionCLI(t, binary, owner, false, "contribution", "decide", "restore", "--revision", issue.Data.EventID, "--expect-heads", restored.ID, "--operation", "hide-retained", "--json")
	// A current native governance amendment checkpoints only already received exact
	// project records. Its proposal remains separately pending until manual admit.
	var inspected struct {
		Records []struct {
			ID     string   `json:"id"`
			Record c.Record `json:"record"`
		} `json:"records"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")), &inspected); e != nil {
		t.Fatal(e)
	}
	for _, row := range inspected.Records {
		if row.Record.Actor != identity.Data.Actor {
			continue
		}
		var authority c.AuthorityRef
		if row.Record.Receipt != nil {
			authority = row.Record.Receipt.Authority
		} else if row.Record.Decision != nil {
			authority = row.Record.Decision.Authority
		} else {
			continue
		}
		policy.ContributionCheckpoints = append(policy.ContributionCheckpoints, ContributionCheckpoint{ID: row.ID, Authority: authority})
	}
	sort.Slice(policy.ContributionCheckpoints, func(i, j int) bool {
		return policy.ContributionCheckpoints[i].ID < policy.ContributionCheckpoints[j].ID
	})
	base := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionGit(t, owner, "switch", "-qc", "checkpoint-amendment")
	raw, _ = json.Marshal(policy)
	if e := os.WriteFile(filepath.Join(owner, ".hn/policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	contributionGit(t, owner, "add", ".hn/policy.json")
	contributionGit(t, owner, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Ratified exact contribution checkpoint")
	head := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionCLI(t, binary, owner, true, "proposal", "open", "--base", base, "--head", head, "Ratify checkpoints")
	inspected.Records = nil
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")), &inspected); e != nil {
		t.Fatal(e)
	}
	proposalID, submissionID := "", ""
	for _, row := range inspected.Records {
		if row.Record.Submission != nil && row.Record.Submission.Operation == c.ProposalCreate {
			proposalID = row.Record.Submission.Content.Revision
			submissionID = row.ID
		}
	}
	if proposalID == "" {
		t.Fatal("native proposal did not enter bounded pending intake")
	}
	contributionCLI(t, binary, owner, false, "merge", proposalID)
	contributionCLI(t, binary, owner, true, "contribution", "decide", "admit", "--submission", submissionID, "--expect-heads", "absent", "--json")
	contributionCLI(t, binary, owner, true, "decide", proposalID, "--accept")
	proposalHidden := contributionCLI(t, binary, owner, true, "contribution", "decide", "hide", "--revision", proposalID, "--expect-heads", "absent", "--json")
	var proposalHide struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal([]byte(proposalHidden), &proposalHide); e != nil {
		t.Fatal(e)
	}
	refused := contributionCLI(t, binary, owner, false, "merge", proposalID)
	if !strings.Contains(refused, "not admitted and visible") {
		t.Fatal("native-evidence-ready hidden proposal did not hit live admission gate: " + refused)
	}
	contributionCLI(t, binary, owner, true, "contribution", "decide", "restore", "--revision", proposalID, "--expect-heads", proposalHide.ID, "--json")

	contributionGit(t, owner, "switch", "-q", "main")
	contributionGit(t, owner, "config", "user.name", "Fixture")
	contributionGit(t, owner, "config", "user.email", "fixture@example.invalid")
	contributionCLI(t, binary, owner, true, "merge", proposalID)
	if strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("authorized policy successor lost effective checkpointed hide")
	}
	archive := filepath.Join(t.TempDir(), "contributions.hnc")
	var backup struct {
		Digest string   `json:"digest"`
		Refs   []string `json:"refs"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "archive", "--output", archive, "--json")), &backup); e != nil {
		t.Fatal(e)
	}
	observer := filepath.Join(t.TempDir(), "observer")
	contributionGit(t, filepath.Dir(observer), "clone", "--quiet", "--no-hardlinks", owner, observer)
	restoreArgs := []string{"contribution", "restore", "--file", archive, "--digest", backup.Digest, "--operation", "observer-restore", "--json"}
	for _, ref := range backup.Refs {
		restoreArgs = append(restoreArgs, "--ref", ref)
	}
	contributionCLI(t, binary, observer, true, restoreArgs...)
	contributionCLI(t, binary, observer, false, "contribution", "authority", "show", "--json")
	if strings.Contains(contributionCLI(t, binary, observer, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("raw restore published content before explicit authority selection")
	}
	// Git ancestry and copied checkpoint JSON alone cannot ratify checkpoint data.
	contributionCLI(t, binary, observer, false, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--read-only")
	contributionCLI(t, binary, observer, true, "replication", "select", "origin", "--actor", identity.Data.Actor, "--proposal", proposalID)
	contributionCLI(t, binary, observer, true, "sync", "origin")
	contributionCLI(t, binary, observer, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--read-only")
	if strings.Contains(contributionCLI(t, binary, observer, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("keyless observer lost effective old hide")
	}

	var reconstructed struct {
		View struct {
			Admitted      map[string]bool `json:"admitted"`
			Visible       map[string]bool `json:"visible"`
			DecisionHeads []struct {
				Target c.Target `json:"target"`
				Heads  []string `json:"heads"`
			} `json:"decisionHeads"`
			Invalid []c.Diagnostic `json:"invalid"`
		} `json:"view"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, observer, true, "contribution", "inspect", "--json")), &reconstructed); e != nil {
		t.Fatal(e)
	}
	if !reconstructed.View.Admitted[issue.Data.EventID] || reconstructed.View.Visible[issue.Data.EventID] {
		t.Fatal("old receipt or effective moderation failed reconstruction")
	}
	matched := false
	for _, heads := range reconstructed.View.DecisionHeads {
		if heads.Target == (c.Target{Kind: c.RevisionTarget, ID: issue.Data.EventID}) && len(heads.Heads) == 1 && heads.Heads[0] == activeHide.ID {
			matched = true
		}
	}
	if !matched {
		t.Fatal("exact old hide head missing")
	}
	for _, diagnostic := range reconstructed.View.Invalid {
		for _, cp := range policy.ContributionCheckpoints {
			if diagnostic.ID == cp.ID {
				t.Fatalf("ratified checkpoint invalid: %+v", diagnostic)
			}
		}
	}
	contributionCLI(t, binary, observer, false, "identity", "show", "--json")
	contributionCLI(t, binary, observer, false, "contribution", "decide", "hide", "--revision", issue.Data.EventID, "--expect-heads", decision.ID, "--json")
}
func actorRefFromCLI(t *testing.T, binary, dir string) string {
	t.Helper()
	var v struct {
		Data struct {
			Actor string `json:"actor"`
		} `json:"data"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, dir, true, "identity", "show", "--json")), &v); e != nil {
		t.Fatal(e)
	}
	return actorRef(v.Data.Actor)
}

func TestContributionCLIBaselineGrowthRequiresLocalReservation(t *testing.T) {
	binary := contributionCLIBinary(t)
	owner := migrationCLIRepository(t)
	contributionCLI(t, binary, owner, true, "init", "--name", "Maintainer")
	actor := strings.TrimPrefix(actorRefFromCLI(t, binary, owner), "refs/hn/actors/")
	original := contributionCLI(t, binary, owner, true, "issue", "open", "--operation", "historical", "--json", "Explicit historical baseline")
	var native struct {
		Data struct {
			EventID string `json:"event_id"`
		} `json:"data"`
	}
	if e := json.Unmarshal([]byte(original), &native); e != nil {
		t.Fatal(e)
	}
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("baseline growth project")), Moderators: []string{actor}, Rules: []c.Rule{{Operation: c.ProposalCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policy.Contributions.Limits.WireBytes = 128 << 10
	policy.Contributions.Limits.DecodedBytes = 1 << 20
	policy.Contributions.Limits.Objects = 1024
	policy.Contributions.Limits.Events = 256
	writePolicy := func() {
		t.Helper()
		raw, _ := json.Marshal(policy)
		if e := os.MkdirAll(filepath.Join(owner, ".hn"), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(owner, ".hn/policy.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		contributionGit(t, owner, "add", ".hn/policy.json")
		contributionGit(t, owner, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Explicit policy amendment")
	}
	writePolicy()
	contributionCLI(t, binary, owner, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	if strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Explicit historical baseline") {
		t.Fatal("raw import became admitted on read")
	}
	base := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionGit(t, owner, "switch", "-qc", "baseline-amendment")
	policy.Contributions.Baseline = []string{native.Data.EventID}
	writePolicy()
	head := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionCLI(t, binary, owner, true, "proposal", "open", "--base", base, "--head", head, "Add finite baseline")
	var records struct {
		Records []struct {
			Record c.Record `json:"record"`
		} `json:"records"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")), &records); e != nil {
		t.Fatal(e)
	}
	proposal := ""
	for _, row := range records.Records {
		if row.Record.Submission != nil {
			proposal = row.Record.Submission.Content.Revision
		}
	}
	if proposal == "" {
		t.Fatal("automatic proposal intake missing")
	}
	contributionCLI(t, binary, owner, true, "decide", proposal, "--accept")
	contributionGit(t, owner, "switch", "-q", "main")
	contributionGit(t, owner, "config", "user.name", "Fixture")
	contributionGit(t, owner, "config", "user.email", "fixture@example.invalid")
	contributionCLI(t, binary, owner, true, "merge", proposal)
	ledgerPath := filepath.Join(owner, ".git/hn/contribution/inbox-"+policy.Contributions.Project[7:]+"/journal.json")
	before, e := os.ReadFile(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	refsBefore := contributionGit(t, owner, "show-ref")
	refused := contributionCLI(t, binary, owner, false, "issue", "list", "--json")
	if !strings.Contains(refused, "lacks durable local reservation") {
		t.Fatal(refused)
	}
	after, e := os.ReadFile(ledgerPath)
	if e != nil || string(before) != string(after) || refsBefore != contributionGit(t, owner, "show-ref") {
		t.Fatal("read mutated baseline accounting or native refs")
	}
	var authority struct {
		Digest string `json:"policyDigest"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "authority", "show", "--json")), &authority); e != nil {
		t.Fatal(e)
	}
	contributionCLI(t, binary, owner, true, "contribution", "baseline", "--expect-policy", authority.Digest)
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Explicit historical baseline") {
		t.Fatal("charged finite baseline not admitted")
	}
	charged, e := os.ReadFile(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	if string(charged) == string(before) {
		t.Fatal("baseline activation was not charged")
	}
	contributionCLI(t, binary, owner, true, "contribution", "baseline", "--expect-policy", authority.Digest)
	replayed, e := os.ReadFile(ledgerPath)
	if e != nil || string(charged) != string(replayed) {
		t.Fatal("baseline exact replay changed charge")
	}

	// A second amendment includes the maintainer's original signed native decision
	// and merge ancestors. Host-only intake must preserve and validate all of them.
	secondBase := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionGit(t, owner, "switch", "-qc", "second-amendment")
	policy.Contributions.Rules = append(policy.Contributions.Rules, c.Rule{Operation: c.IssueCreate, Automatic: true})
	writePolicy()
	secondHead := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionCLI(t, binary, owner, true, "proposal", "open", "--base", secondBase, "--head", secondHead, "Second maintainer amendment")
	records.Records = nil
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")), &records); e != nil {
		t.Fatal(e)
	}
	secondProposal := ""
	for _, row := range records.Records {
		if row.Record.Submission != nil && row.Record.Submission.Content.Revision != proposal {
			secondProposal = row.Record.Submission.Content.Revision
		}
	}
	if secondProposal == "" {
		t.Fatal("second native amendment failed full-history local intake")
	}
	contributionCLI(t, binary, owner, true, "decide", secondProposal, "--accept")
	contributionGit(t, owner, "switch", "-q", "main")
	contributionCLI(t, binary, owner, true, "merge", secondProposal)
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Explicit historical baseline") {
		t.Fatal("same finite baseline lost its retained charge across a policy successor")
	}
	refusedArchive := contributionCLI(t, binary, owner, false, "contribution", "archive", "--output", filepath.Join(t.TempDir(), "unsupported.hnc"))
	if !strings.Contains(refusedArchive, "unsupported signed history") {
		t.Fatal(refusedArchive)
	}
	// The explicit trusted public Git backup carries exact native/code/contribution
	// refs, separate from keys/ledger/selection. It is not ordinary hn sync support.
	wantedRefs := strings.Fields(contributionGit(t, owner, "for-each-ref", "--format=%(refname)", "refs/hn/actors/", "refs/hn/proposals/", "refs/hn/contributions/"))
	bundle := filepath.Join(t.TempDir(), "trusted-native.bundle")
	bundleArgs := append([]string{"bundle", "create", bundle, "refs/heads/main"}, wantedRefs...)
	contributionGit(t, owner, bundleArgs...)
	fresh := filepath.Join(t.TempDir(), "keyless")
	contributionGit(t, filepath.Dir(fresh), "clone", "--quiet", "--branch", "main", bundle, fresh)
	fetchArgs := []string{"fetch", "--quiet", bundle}
	for _, ref := range wantedRefs {
		target := ref
		if strings.HasPrefix(ref, "refs/hn/actors/") {
			target = "refs/hn/remotes/recovery/actors/" + strings.TrimPrefix(ref, "refs/hn/actors/")
		} else if strings.HasPrefix(ref, "refs/hn/proposals/") {
			target = "refs/hn/remotes/recovery/proposals/" + strings.TrimPrefix(ref, "refs/hn/proposals/")
		}
		fetchArgs = append(fetchArgs, ref+":"+target)
	}
	contributionGit(t, fresh, fetchArgs...)
	contributionCLI(t, binary, fresh, false, "contribution", "inspect", "--json")
	contributionCLI(t, binary, fresh, false, "identity", "show", "--json")
	contributionCLI(t, binary, fresh, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--read-only")
	var expected, recovered struct {
		View json.RawMessage `json:"view"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")), &expected); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, fresh, true, "contribution", "inspect", "--json")), &recovered); e != nil {
		t.Fatal(e)
	}
	if string(expected.View) != string(recovered.View) {
		t.Fatalf("explicit trusted Git restore changed governed state: %s / %s", expected.View, recovered.View)
	}
	for _, ref := range wantedRefs {
		if strings.HasPrefix(ref, "refs/hn/contributions/") {
			if contributionGit(t, owner, "rev-parse", ref) != contributionGit(t, fresh, "rev-parse", ref) {
				t.Fatal("original contribution manifest commit changed on raw Git restore")
			}
		}
	}
	lookup := contributionCLI(t, binary, owner, true, "issue", "operation", "--actor", actor, "--operation", "historical", "--json")
	if !strings.Contains(lookup, native.Data.EventID) {
		t.Fatal("baseline history lookup missing")
	}
}

func TestContributionCLILocalCapacityBeforeNativeWritesAndRawCannotSelectTrust(t *testing.T) {
	binary := contributionCLIBinary(t)
	owner := migrationCLIRepository(t)
	contributionCLI(t, binary, owner, true, "init", "--name", "Maintainer")
	actor := strings.TrimPrefix(actorRefFromCLI(t, binary, owner), "refs/hn/actors/")
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("local capacity")), Moderators: []string{actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policy.Contributions.Limits.WireBytes = 128 << 10
	policy.Contributions.Limits.DecodedBytes = 1 << 20
	policy.Contributions.Limits.Objects = 1024
	policy.Contributions.Limits.Events = 256
	policy.Contributions.Limits.Window.Requests = 2
	policy.Contributions.Limits.Retained.Requests = 2
	raw, _ := json.Marshal(policy)
	if e := os.MkdirAll(filepath.Join(owner, ".hn"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(owner, ".hn/policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	contributionGit(t, owner, "add", ".hn/policy.json")
	contributionGit(t, owner, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Bounded local policy")
	contributionCLI(t, binary, owner, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	opened := contributionCLI(t, binary, owner, true, "issue", "open", "--operation", "native-one", "--json", "Charged native issue")
	var issue struct {
		Data struct {
			EventID   string `json:"event_id"`
			Admission string `json:"admission"`
		} `json:"data"`
	}
	if e := json.Unmarshal([]byte(opened), &issue); e != nil {
		t.Fatal(e)
	}
	if issue.Data.Admission != "admitted" {
		t.Fatal(opened)
	}
	var intake struct {
		Attempt    ContributionIntake          `json:"attempt"`
		Descriptor contributionLocalDescriptor `json:"descriptor"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "intake", "--operation", "native-"+issue.Data.EventID[7:], "--json")), &intake); e != nil {
		t.Fatal(e)
	}
	if intake.Descriptor.Version != "hn.contribution-local/1" || intake.Descriptor.Revision != issue.Data.EventID || intake.Descriptor.Actor != actor {
		t.Fatal("local source attribution missing")
	}
	root := filepath.Join(owner, ".git/hn/contribution/inbox-"+policy.Contributions.Project[7:])
	descriptor := filepath.Join(root, "local-"+intake.Attempt.Artifact[7:]+".json")
	objects := contributionGit(t, owner, "count-objects", "-v")
	refs := contributionGit(t, owner, "show-ref")
	// The public parser cannot interpret an internal descriptor as trusted intake,
	// including an exact digest that already has a host-local reservation.
	aliased := contributionCLI(t, binary, owner, false, "contribution", "receive", "--file", descriptor, "--digest", intake.Attempt.Artifact, "--operation", "alias-local", "--ref", "refs/heads/main", "--json")
	if !strings.Contains(aliased, "cannot select public artifact") {
		t.Fatal(aliased)
	}
	descriptorBytes, e := os.ReadFile(descriptor)
	if e != nil {
		t.Fatal(e)
	}
	modified := append(descriptorBytes, '\n')
	rawDescriptor := filepath.Join(t.TempDir(), "raw.json")
	if e := os.WriteFile(rawDescriptor, modified, 0600); e != nil {
		t.Fatal(e)
	}
	rejected := contributionCLI(t, binary, owner, false, "contribution", "receive", "--file", rawDescriptor, "--digest", c.Digest(modified), "--operation", "raw-descriptor", "--ref", "refs/heads/main", "--json")
	if !strings.Contains(rejected, "framing") {
		t.Fatal(rejected)
	}
	ledgerPath := filepath.Join(root, "journal.json")
	ledger, e := os.ReadFile(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 4; i++ {
		output := contributionCLI(t, binary, owner, false, "issue", "open", "--operation", fmt.Sprintf("over-budget-%d", i), "--json", "Must not append")
		if !strings.Contains(output, "budget exhausted") {
			t.Fatal(output)
		}
	}
	if objects != contributionGit(t, owner, "count-objects", "-v") || refs != contributionGit(t, owner, "show-ref") {
		t.Fatal("exhausted local writer retained uncharged native objects/refs")
	}
	after, e := os.ReadFile(ledgerPath)
	if e != nil || string(ledger) != string(after) {
		t.Fatal("exhaustion wrote additional accounting metadata")
	}
	replay := contributionCLI(t, binary, owner, true, "issue", "open", "--operation", "native-one", "--json", "Charged native issue")
	if !strings.Contains(replay, issue.Data.EventID) || !strings.Contains(replay, `"replayed":true`) {
		t.Fatal(replay)
	}
	after, e = os.ReadFile(ledgerPath)
	if e != nil || string(ledger) != string(after) {
		t.Fatal("exact native replay spent new capacity")
	}
}

// Hostile envelopes are spliced into the real exported wire container while
// preserving its complete independently authenticated native archive bytes.
func contributionRewriteArtifact(t *testing.T, path string, envs []c.Envelope) (string, string) {
	t.Helper()
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	start := len(contributionArtifactMagic)
	length := int(binary.BigEndian.Uint64(raw[start : start+8]))
	var manifest contributionManifest
	if e = json.Unmarshal(raw[start+8:start+8+length], &manifest); e != nil {
		t.Fatal(e)
	}
	manifest.Envelopes = nil
	envs = append([]c.Envelope{}, envs...)
	sort.Slice(envs, func(i, j int) bool { return c.Digest(envs[i].Payload) < c.Digest(envs[j].Payload) })
	for _, env := range envs {
		manifest.Envelopes = append(manifest.Envelopes, contributionWireEnvelope{Payload: env.Payload, Signature: env.Signature})
	}
	encoded, e := json.Marshal(manifest)
	if e != nil {
		t.Fatal(e)
	}
	result := append([]byte{}, raw[:start]...)
	result = binary.BigEndian.AppendUint64(result, uint64(len(encoded)))
	result = append(result, encoded...)
	result = append(result, raw[start+8+length:]...)
	output := filepath.Join(t.TempDir(), "hostile.hnc")
	if e = os.WriteFile(output, result, 0600); e != nil {
		t.Fatal(e)
	}
	return output, c.Digest(result)
}

func contributionEnabledRepository(t *testing.T, binary, name string, rules []c.Rule, extraModerators ...string) (string, string, PolicyDocument) {
	t.Helper()
	repo := migrationCLIRepository(t)
	contributionCLI(t, binary, repo, true, "init", "--name", "Maintainer")
	actor := strings.TrimPrefix(actorRefFromCLI(t, binary, repo), "refs/hn/actors/")
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte(name)), Moderators: []string{actor}, Rules: rules, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policy.Contributions.Moderators = append(policy.Contributions.Moderators, extraModerators...)
	sort.Strings(policy.Contributions.Moderators)
	policy.Contributions.Limits.WireBytes = 128 << 10
	policy.Contributions.Limits.DecodedBytes = 1 << 20
	policy.Contributions.Limits.Objects = 1024
	policy.Contributions.Limits.Events = 256
	raw, _ := json.Marshal(policy)
	if e := os.MkdirAll(filepath.Join(repo, ".hn"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(repo, ".hn/policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	contributionGit(t, repo, "add", ".hn/policy.json")
	contributionGit(t, repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Explicit contribution policy")
	contributionCLI(t, binary, repo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	return repo, actor, policy
}
func contributionEventID(t *testing.T, output string) string {
	t.Helper()
	var result struct {
		Data struct {
			EventID string `json:"event_id"`
		} `json:"data"`
	}
	if e := json.Unmarshal([]byte(output), &result); e != nil {
		t.Fatal(e)
	}
	if !validEventID(result.Data.EventID) {
		t.Fatal(output)
	}
	return result.Data.EventID
}
func contributionDecisionID(t *testing.T, output string) string {
	t.Helper()
	var result struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal([]byte(output), &result); e != nil {
		t.Fatal(e)
	}
	if !validEventID(result.ID) {
		t.Fatal(output)
	}
	return result.ID
}
func contributionStateFile(t *testing.T, state any) string {
	t.Helper()
	raw, e := json.Marshal(state)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func TestContributionCLIGovernedSpecializedResourcesAndHiddenDependencies(t *testing.T) {
	binary := contributionCLIBinary(t)
	rules := []c.Rule{}
	for _, op := range []c.Operation{c.ProjectCreate, c.ProjectEdit, c.ProjectItemCreate, c.ProjectItemEdit, c.DiscussionCreate, c.DiscussionReply, c.DiscussionEdit, c.WorkflowCreate, c.WorkflowEdit} {
		rules = append(rules, c.Rule{Operation: op, Automatic: true})
	}
	repo, actor, _ := contributionEnabledRepository(t, binary, "specialized resources", rules)
	project := contributionEventID(t, contributionCLI(t, binary, repo, true, "project", "open", "--title", "Original project", "--operation", "project-open", "--json"))
	item := contributionEventID(t, contributionCLI(t, binary, repo, true, "project-item", "add", "--project", project, "--draft-title", "Pinned item", "--operation", "item-open", "--json"))
	projectHead := contributionEventID(t, contributionCLI(t, binary, repo, true, "project", "revise", project, "--expect", project, "--title", "Current project", "--operation", "project-revise", "--json"))
	hiddenProject := contributionDecisionID(t, contributionCLI(t, binary, repo, true, "contribution", "decide", "hide", "--revision", project, "--expect-heads", "absent", "--json"))
	if !strings.Contains(contributionCLI(t, binary, repo, true, "project", "show", project, "--json"), "Current project") {
		t.Fatal("hidden predecessor erased admitted current project")
	}
	if strings.Contains(contributionCLI(t, binary, repo, true, "project-item", "list", "--json"), "Pinned item") {
		t.Fatal("exact hidden schema revision leaked through item view")
	}
	lookup := contributionCLI(t, binary, repo, true, "project-item", "operation", "--actor", actor, "--operation", "item-open", "--json")
	if !strings.Contains(lookup, item) || strings.Contains(lookup, "Pinned item") {
		t.Fatal("hidden operation did not preserve exact result with redaction: " + lookup)
	}
	replay := contributionCLI(t, binary, repo, true, "project-item", "add", "--project", project, "--draft-title", "Pinned item", "--operation", "item-open", "--json")
	if !strings.Contains(replay, item) || !strings.Contains(replay, `"replayed":true`) {
		t.Fatal(replay)
	}
	contributionCLI(t, binary, repo, true, "contribution", "decide", "restore", "--revision", project, "--expect-heads", hiddenProject, "--json")
	if !strings.Contains(contributionCLI(t, binary, repo, true, "project-item", "show", item, "--json"), "Pinned item") {
		t.Fatal("restored schema did not restore item visibility")
	}
	if !strings.Contains(contributionCLI(t, binary, repo, true, "project", "heads", project, "--json"), projectHead) {
		t.Fatal("restored predecessor resurrected obsolete project head")
	}
	thread := contributionEventID(t, contributionCLI(t, binary, repo, true, "discussion", "open", "--title", "Question", "--category", "general", "--operation", "thread", "--json"))
	reply := contributionEventID(t, contributionCLI(t, binary, repo, true, "discussion", "reply", "add", thread, "--body", "Exact accepted answer", "--operation", "answer", "--json"))
	answered := contributionEventID(t, contributionCLI(t, binary, repo, true, "discussion", "answer", thread, "--expect", thread, "--reply", reply, "--operation", "accept-answer", "--json"))
	if !strings.Contains(contributionCLI(t, binary, repo, true, "discussion", "show", thread, "--json"), "Exact accepted answer") {
		t.Fatal("valid derived answer/reply root cycle was suppressed")
	}
	hideAnswer := contributionDecisionID(t, contributionCLI(t, binary, repo, true, "contribution", "decide", "hide", "--revision", reply, "--expect-heads", "absent", "--json"))
	contributionCLI(t, binary, repo, false, "discussion", "show", thread, "--json")
	before := contributionGit(t, repo, "show-ref")
	contributionCLI(t, binary, repo, false, "discussion", "reply", "add", thread, "--body", "Old opening cannot authorize hidden newer state", "--operation", "hidden-parent", "--json")
	if before != contributionGit(t, repo, "show-ref") {
		t.Fatal("hidden current root admitted a reply via old opening")
	}
	if !strings.Contains(contributionCLI(t, binary, repo, true, "contribution", "inspect", "--raw", reply, "--json"), "Exact accepted answer") {
		t.Fatal("raw hidden reply lost")
	}
	contributionCLI(t, binary, repo, true, "contribution", "decide", "restore", "--revision", reply, "--expect-heads", hideAnswer, "--json")
	if !strings.Contains(contributionCLI(t, binary, repo, true, "discussion", "heads", thread, "--json"), answered) {
		t.Fatal("answer restore changed admitted lineage")
	}
	sentinel := filepath.Join(repo, "executed")
	state := WorkflowState{Schema: "hn.workflow/1", Name: "fixture", Title: "Native fixture workflow", Enabled: true, Pipeline: &PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "proof", Command: "touch " + sentinel}}}}
	workflow := contributionEventID(t, contributionCLI(t, binary, repo, true, "workflow", "open", "--input", contributionStateFile(t, state), "--operation", "workflow-open", "--json"))
	contributionCLI(t, binary, repo, true, "workflow", "materialize", workflow, "--json")
	if _, e := os.Stat(sentinel); !os.IsNotExist(e) {
		t.Fatal("admission/materialization executed native pipeline")
	}
	pipelinePath := filepath.Join(repo, ".hn/pipelines/fixture.json")
	if e := os.Remove(pipelinePath); e != nil {
		t.Fatal(e)
	}
	contributionCLI(t, binary, repo, true, "contribution", "decide", "hide", "--revision", workflow, "--expect-heads", "absent", "--json")
	contributionCLI(t, binary, repo, false, "workflow", "materialize", workflow, "--json")
	if _, e := os.Stat(pipelinePath); !os.IsNotExist(e) {
		t.Fatal("hidden workflow materialized through specialized selector")
	}
	contributionCLI(t, binary, repo, false, "runner", "once", "--accept-pipeline", "fixture", "--accept-actor", actor)
	if _, e := os.Stat(filepath.Join(repo, ".git/hn/audit/authority.json")); !os.IsNotExist(e) {
		t.Fatal("contribution activation adopted independent audit authority")
	}
}

func TestContributionCLIDisabledPreservesNativeBehavior(t *testing.T) {
	binary := contributionCLIBinary(t)
	repo := migrationCLIRepository(t)
	contributionCLI(t, binary, repo, true, "init", "--name", "Default native author")
	id := contributionEventID(t, contributionCLI(t, binary, repo, true, "issue", "open", "--operation", "legacy-open", "--json", "Legacy native issue"))
	contributionCLI(t, binary, repo, true, "issue", "close", id, "--expect", id, "--json")
	if _, e := os.Stat(filepath.Join(repo, ".git/hn/contribution")); !os.IsNotExist(e) {
		t.Fatal("absent policy created contribution state")
	}
	if refs := contributionGit(t, repo, "for-each-ref", "--format=%(refname)", "refs/hn/contributions/"); refs != "" {
		t.Fatal("absent policy emitted contribution records")
	}
	if strings.Contains(contributionCLI(t, binary, repo, true, "log"), "unadmitted") {
		t.Fatal("absent configuration changed raw log format")
	}
}

func TestContributionCLIWorktreesShareAccountingAndLockKeepLocalAuthority(t *testing.T) {
	binary := contributionCLIBinary(t)
	repo := migrationCLIRepository(t)
	contributionCLI(t, binary, repo, true, "init", "--name", "First maintainer")
	first := strings.TrimPrefix(actorRefFromCLI(t, binary, repo), "refs/hn/actors/")
	linked := filepath.Join(t.TempDir(), "linked")
	contributionGit(t, repo, "worktree", "add", "--quiet", "-b", "linked", linked, "main")
	contributionCLI(t, binary, linked, true, "init", "--name", "Second maintainer")
	second := strings.TrimPrefix(actorRefFromCLI(t, binary, linked), "refs/hn/actors/")
	contributionCLI(t, binary, linked, true, "issue", "open", "--operation", "default-off-linked", "--json", "Raw linked default-off issue")
	private := contributionGit(t, linked, "rev-parse", "--absolute-git-dir")
	if _, e := os.Stat(filepath.Join(private, "hn/contribution")); !os.IsNotExist(e) {
		t.Fatal("default-off linked worktree created contribution state")
	}
	maintainers := []string{first, second}
	sort.Strings(maintainers)
	policy := PolicyDocument{Version: policyVersion, Maintainers: maintainers, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("shared worktree project")), Moderators: maintainers, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policy.Contributions.Limits.WireBytes = 128 << 10
	policy.Contributions.Limits.DecodedBytes = 1 << 20
	policy.Contributions.Limits.Objects = 1024
	policy.Contributions.Limits.Events = 256
	policy.Contributions.Limits.Window.Requests = 2
	policy.Contributions.Limits.Retained.Requests = 2
	raw, _ := json.Marshal(policy)
	if e := os.MkdirAll(filepath.Join(repo, ".hn"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(repo, ".hn/policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	contributionGit(t, repo, "add", ".hn/policy.json")
	contributionGit(t, repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Explicit shared project policy")
	contributionCLI(t, binary, repo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	contributionCLI(t, binary, linked, false, "contribution", "authority", "show", "--json")
	contributionCLI(t, binary, linked, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	common := contributionGit(t, repo, "rev-parse", "--absolute-git-dir")
	lock, e := os.OpenFile(filepath.Join(common, "hn/contribution/operation.lock"), os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		t.Fatal(e)
	}
	locked := contributionCLI(t, binary, linked, false, "issue", "open", "--json", "Lock must be common")
	syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	lock.Close()
	if !strings.Contains(locked, "contribution operation busy") {
		t.Fatal(locked)
	}
	contributionCLI(t, binary, repo, true, "issue", "open", "--operation", "shared-one", "--json", "First shared admitted issue")
	contributionCLI(t, binary, linked, true, "issue", "open", "--operation", "shared-two", "--json", "Second shared admitted issue")
	ledgerPath := filepath.Join(common, "hn/contribution/inbox-"+policy.Contributions.Project[7:]+"/journal.json")
	before, e := os.ReadFile(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	objects := contributionGit(t, repo, "count-objects", "-v")
	for _, cwd := range []string{repo, linked} {
		refused := contributionCLI(t, binary, cwd, false, "issue", "open", "--json", "Shared capacity exhausted")
		if !strings.Contains(refused, "budget exhausted") {
			t.Fatal(refused)
		}
		shown := contributionCLI(t, binary, cwd, true, "issue", "list", "--json")
		if !strings.Contains(shown, "First shared admitted issue") || !strings.Contains(shown, "Second shared admitted issue") || strings.Contains(shown, "Raw linked default-off issue") {
			t.Fatal(shown)
		}
	}
	after, e := os.ReadFile(ledgerPath)
	if e != nil || string(before) != string(after) || objects != contributionGit(t, repo, "count-objects", "-v") {
		t.Fatal("linked worktree reset shared retention accounting")
	}
	if _, e := os.Stat(filepath.Join(private, "hn/contribution/inbox-"+policy.Contributions.Project[7:])); !os.IsNotExist(e) {
		t.Fatal("linked worktree received a separate project budget")
	}
	anchorPath := filepath.Join(private, "hn/contribution/authority.json")
	saved, e := os.ReadFile(anchorPath)
	if e != nil {
		t.Fatal(e)
	}
	foreign, e := os.ReadFile(filepath.Join(common, "hn/contribution/authority.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(anchorPath, foreign, 0600); e != nil {
		t.Fatal(e)
	}
	refused := contributionCLI(t, binary, linked, false, "contribution", "authority", "show", "--json")
	if !strings.Contains(refused, "another worktree context") {
		t.Fatal(refused)
	}
	if e = os.WriteFile(anchorPath, saved, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestContributionCLIConflictsRevocationAndManufacturedHistoricalDecisions(t *testing.T) {
	binary := contributionCLIBinary(t)
	moderatorRepo := migrationCLIRepository(t)
	contributionCLI(t, binary, moderatorRepo, true, "init", "--name", "Scoped moderator")
	moderator := strings.TrimPrefix(actorRefFromCLI(t, binary, moderatorRepo), "refs/hn/actors/")
	owner, actor, policy := contributionEnabledRepository(t, binary, "revocation project", []c.Rule{{Operation: c.IssueCreate, Automatic: true}, {Operation: c.ProposalCreate, Automatic: true}}, moderator)
	issue := contributionEventID(t, contributionCLI(t, binary, owner, true, "issue", "open", "--operation", "subject", "--json", "Scoped moderation subject"))
	prepared := filepath.Join(t.TempDir(), "subject.hnc")
	contributionCLI(t, binary, owner, true, "contribution", "prepare", "--revision", issue, "--output", prepared, "--json")
	var authority struct {
		Commit string `json:"commit"`
		Digest string `json:"policyDigest"`
	}
	if e := json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "authority", "show", "--json")), &authority); e != nil {
		t.Fatal(e)
	}
	old := c.AuthorityRef{Commit: authority.Commit, Digest: authority.Digest}
	rawKey, e := os.ReadFile(filepath.Join(moderatorRepo, ".git/hn/identities/"+moderator+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var identity Identity
	if e = json.Unmarshal(rawKey, &identity); e != nil {
		t.Fatal(e)
	}
	key, e := identity.privateKey()
	if e != nil {
		t.Fatal(e)
	}
	decision := func(action c.Action, intent string, parents []string) c.Envelope {
		t.Helper()
		env, e := c.Sign(c.Record{Version: c.Version, Kind: c.DecisionKind, Project: policy.Contributions.Project, Actor: moderator, PublicKey: identity.PublicKey, Intent: intent, Decision: &c.Decision{Action: action, Target: c.Target{Kind: c.RevisionTarget, ID: issue}, Authority: old, Parents: parents}}, key)
		if e != nil {
			t.Fatal(e)
		}
		return env
	}
	deliver := func(operation string, envelopes ...c.Envelope) {
		t.Helper()
		file, digest := contributionRewriteArtifact(t, prepared, envelopes)
		contributionCLI(t, binary, owner, true, "contribution", "receive", "--file", file, "--digest", digest, "--operation", operation, "--ref", actorRef(actor), "--json")
	}
	hide, restore := decision(c.Hide, "concurrent-hide", []string{}), decision(c.Restore, "concurrent-restore", []string{})
	deliver("concurrent-controls", hide, restore)
	hideID, restoreID := c.Digest(hide.Payload), c.Digest(restore.Payload)
	if strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Scoped moderation subject") {
		t.Fatal("concurrent moderation selected a visibility winner")
	}
	pending := contributionCLI(t, binary, owner, true, "contribution", "pending", "--json")
	if !strings.Contains(pending, hideID) || !strings.Contains(pending, restoreID) || !strings.Contains(pending, "conflicts") {
		t.Fatal(pending)
	}
	contributionCLI(t, binary, owner, false, "contribution", "decide", "restore", "--revision", issue, "--expect-heads", "absent", "--json")
	parents := []string{hideID, restoreID}
	sort.Strings(parents)
	joined := contributionDecisionID(t, contributionCLI(t, binary, owner, true, "contribution", "decide", "restore", "--revision", issue, "--expect-heads", strings.Join(parents, ","), "--operation", "join-heads", "--json"))
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Scoped moderation subject") {
		t.Fatal("authorized join failed to resolve conflict")
	}
	effective := decision(c.Hide, "established-moderator-hide", []string{joined})
	deliver("effective-old-hide", effective)
	effectiveID := c.Digest(effective.Payload)
	if strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Scoped moderation subject") {
		t.Fatal("scoped moderator could not hide subject")
	}
	var inspection struct {
		Records []struct {
			ID     string   `json:"id"`
			Record c.Record `json:"record"`
		} `json:"records"`
	}
	if e = json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")), &inspection); e != nil {
		t.Fatal(e)
	}
	for _, row := range inspection.Records {
		if row.Record.Decision != nil {
			policy.ContributionCheckpoints = append(policy.ContributionCheckpoints, ContributionCheckpoint{ID: row.ID, Authority: row.Record.Decision.Authority})
		} else if row.Record.Receipt != nil {
			policy.ContributionCheckpoints = append(policy.ContributionCheckpoints, ContributionCheckpoint{ID: row.ID, Authority: row.Record.Receipt.Authority})
		}
	}
	sort.Slice(policy.ContributionCheckpoints, func(i, j int) bool {
		return policy.ContributionCheckpoints[i].ID < policy.ContributionCheckpoints[j].ID
	})
	policy.Contributions.Moderators = []string{actor}
	policy.Contributions.RevokedModerators = []string{moderator}
	base := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionGit(t, owner, "switch", "-qc", "revoke-moderator")
	raw, _ := json.Marshal(policy)
	if e = os.WriteFile(filepath.Join(owner, ".hn/policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	contributionGit(t, owner, "add", ".hn/policy.json")
	contributionGit(t, owner, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Governed exact moderator revocation")
	head := contributionGit(t, owner, "rev-parse", "HEAD")
	contributionCLI(t, binary, owner, true, "proposal", "open", "--base", base, "--head", head, "Revoke scoped moderator")
	inspection.Records = nil
	if e = json.Unmarshal([]byte(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--json")), &inspection); e != nil {
		t.Fatal(e)
	}
	proposal := ""
	for _, row := range inspection.Records {
		if row.Record.Submission != nil && row.Record.Submission.Operation == c.ProposalCreate {
			proposal = row.Record.Submission.Content.Revision
		}
	}
	if proposal == "" {
		t.Fatal("revocation native proposal missing")
	}
	contributionCLI(t, binary, owner, true, "decide", proposal, "--accept")
	contributionGit(t, owner, "switch", "-q", "main")
	contributionGit(t, owner, "config", "user.name", "Fixture")
	contributionGit(t, owner, "config", "user.email", "fixture@example.invalid")
	contributionCLI(t, binary, owner, true, "merge", proposal)
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Scoped moderation subject") {
		t.Fatal("revoked descendant erased surviving authorized restore head")
	}
	view := contributionCLI(t, binary, owner, true, "contribution", "pending", "--json")
	if !strings.Contains(view, effectiveID) || !strings.Contains(view, "revoked") {
		t.Fatal("historical revocation was not inspectable")
	}
	manufactured := decision(c.Hide, "manufactured-after-removal", []string{effectiveID})
	deliver("manufactured-old-policy", manufactured)
	view = contributionCLI(t, binary, owner, true, "contribution", "pending", "--json")
	if !strings.Contains(view, c.Digest(manufactured.Payload)) || !strings.Contains(view, "requires established authority checkpoint") {
		t.Fatal(view)
	}
	if !strings.Contains(contributionCLI(t, binary, owner, true, "issue", "list", "--json"), "Scoped moderation subject") {
		t.Fatal("old credentials manufactured current authority")
	}
	if !strings.Contains(contributionCLI(t, binary, owner, true, "contribution", "inspect", "--raw", effectiveID, "--json"), moderator) {
		t.Fatal("revocation erased original signed moderation history")
	}
}

func TestContributionCLIRawRecoveryChargesLocalRetentionAndGuardsNewRefs(t *testing.T) {
	binary := contributionCLIBinary(t)
	source := migrationCLIRepository(t)
	contributionCLI(t, binary, source, true, "init", "--name", "Source maintainer")
	actor := strings.TrimPrefix(actorRefFromCLI(t, binary, source), "refs/hn/actors/")
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("raw receiver retention")), Moderators: []string{actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policy.Contributions.Limits.WireBytes = 128 << 10
	policy.Contributions.Limits.DecodedBytes = 1 << 20
	policy.Contributions.Limits.Objects = 1024
	policy.Contributions.Limits.Events = 256
	policy.Contributions.Limits.Window.Requests = 2
	policy.Contributions.Limits.Retained.Requests = 2
	raw, _ := json.Marshal(policy)
	if e := os.MkdirAll(filepath.Join(source, ".hn"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(source, ".hn/policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	contributionGit(t, source, "add", ".hn/policy.json")
	contributionGit(t, source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Finite receiver retention")
	contributionCLI(t, binary, source, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	issue := contributionEventID(t, contributionCLI(t, binary, source, true, "issue", "open", "--operation", "original", "--json", "Portable receipt needs local charge"))
	ref := "refs/hn/contributions/" + policy.Contributions.Project[7:] + "/" + actor
	bundle := filepath.Join(t.TempDir(), "public.bundle")
	contributionGit(t, source, "bundle", "create", bundle, "refs/heads/main", actorRef(actor), ref)
	reader := filepath.Join(t.TempDir(), "reader")
	contributionGit(t, filepath.Dir(reader), "clone", "--quiet", "--branch", "main", bundle, reader)
	contributionGit(t, reader, "fetch", "--quiet", bundle, actorRef(actor)+":refs/hn/remotes/recovery/actors/"+actor, ref+":"+ref)
	contributionCLI(t, binary, reader, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--read-only")
	ledgerPath := filepath.Join(reader, ".git/hn/contribution/inbox-"+policy.Contributions.Project[7:]+"/journal.json")
	ledger, e := os.ReadFile(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	var journal contributionJournal
	if e = json.Unmarshal(ledger, &journal); e != nil {
		t.Fatal(e)
	}
	if journal.Ledger.RetainedUsed.Requests != 1 || journal.Ledger.RetainedUsed.Bytes == 0 {
		t.Fatal("foreign signed receipt supplied receiver-local capacity")
	}
	if !strings.Contains(contributionCLI(t, binary, reader, true, "issue", "list", "--json"), "Portable receipt needs local charge") {
		t.Fatal("charged raw restore failed governed reconstruction")
	}
	keyRaw, e := os.ReadFile(filepath.Join(source, ".git/hn/identities/"+actor+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var identity Identity
	if e = json.Unmarshal(keyRaw, &identity); e != nil {
		t.Fatal(e)
	}
	key, e := identity.privateKey()
	if e != nil {
		t.Fatal(e)
	}
	authority := c.AuthorityRef{Commit: contributionGit(t, source, "rev-parse", "main"), Digest: c.Digest(raw)}
	// These are deliberate external Git writes, outside the host storage boundary.
	// They retain valid signatures but cannot spend this receiver's local budget.
	rawWrite := func(action c.Action, intent string, parents []string) string {
		t.Helper()
		env, e := c.Sign(c.Record{Version: c.Version, Kind: c.DecisionKind, Project: policy.Contributions.Project, Actor: actor, PublicKey: identity.PublicKey, Intent: intent, Decision: &c.Decision{Action: action, Target: c.Target{Kind: c.RevisionTarget, ID: issue}, Authority: authority, Parents: parents}}, key)
		if e != nil {
			t.Fatal(e)
		}
		old := contributionGit(t, source, "rev-parse", ref)
		head, e := prepareContributionManifest(context.Background(), filepath.Join(source, ".git"), policy.Contributions.Project, actor, old, []c.Envelope{env}, policy.Contributions.Limits)
		if e != nil {
			t.Fatal(e)
		}
		contributionGit(t, source, "update-ref", ref, head, old)
		contributionGit(t, reader, "fetch", "--quiet", source, ref+":"+ref)
		return c.Digest(env.Payload)
	}
	hidden := rawWrite(c.Hide, "raw-hide", []string{})
	refused := contributionCLI(t, binary, reader, false, "issue", "list", "--json")
	if !strings.Contains(refused, "not locally registered") {
		t.Fatal(refused)
	}
	after, e := os.ReadFile(ledgerPath)
	if e != nil || string(ledger) != string(after) {
		t.Fatal("read silently registered raw additions")
	}
	contributionCLI(t, binary, reader, true, "contribution", "retained", "register", "--json")
	if strings.Contains(contributionCLI(t, binary, reader, true, "issue", "list", "--json"), "Portable receipt needs local charge") {
		t.Fatal("registered valid raw hide did not take effect")
	}
	charged, e := os.ReadFile(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	rawWrite(c.Restore, "raw-restore", []string{hidden})
	contributionCLI(t, binary, reader, false, "issue", "list", "--json")
	failed := contributionCLI(t, binary, reader, false, "contribution", "retained", "register", "--json")
	if !strings.Contains(failed, "budget exhausted") {
		t.Fatal(failed)
	}
	after, e = os.ReadFile(ledgerPath)
	if e != nil || string(charged) != string(after) {
		t.Fatal("raw registration reset exhausted receiver capacity")
	}
	contributionCLI(t, binary, reader, false, "identity", "show", "--json")
}

func contributionVerifyCandidate(path, digest string) error {
	if !filepath.IsAbs(path) || !validEventID(digest) {
		return fmt.Errorf("candidate binary requires absolute path and exact sha256 digest")
	}
	info, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return fmt.Errorf("candidate binary must be a regular executable")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if c.Digest(raw) != digest {
		return fmt.Errorf("candidate binary digest changed")
	}
	return nil
}
func contributionCLIBinary(t *testing.T) string {
	t.Helper()
	candidate, digest := os.Getenv("HN_CONTRIBUTION_TEST_BINARY"), os.Getenv("HN_CONTRIBUTION_TEST_BINARY_SHA256")
	if candidate != "" || digest != "" {
		if e := contributionVerifyCandidate(candidate, digest); e != nil {
			t.Fatal(e)
		}
		t.Logf("unchanged injected CLI: %s %s", candidate, digest)
		return candidate
	}
	return buildMigrationCLIBinary(t, "")
}
func TestContributionInjectedBinaryRequiresExactImmutableSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate")
	if e := os.WriteFile(path, []byte("candidate bytes"), 0700); e != nil {
		t.Fatal(e)
	}
	digest := c.Digest([]byte("candidate bytes"))
	if e := contributionVerifyCandidate(path, digest); e != nil {
		t.Fatal(e)
	}
	for _, input := range [][2]string{{"relative", digest}, {path, ""}, {path, c.Digest([]byte("different"))}} {
		if contributionVerifyCandidate(input[0], input[1]) == nil {
			t.Fatal("unsafe candidate selector accepted")
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if contributionVerifyCandidate(link, digest) == nil {
		t.Fatal("linked candidate accepted")
	}
	if e := os.WriteFile(path, []byte("changed bytes"), 0700); e != nil {
		t.Fatal(e)
	}
	if contributionVerifyCandidate(path, digest) == nil {
		t.Fatal("changed executable retained original binding")
	}
}

func TestContributionInjectedBinarySelectionRejectsIncompletePair(t *testing.T) {
	if os.Getenv("HN_CONTRIBUTION_TEST_SELECTION_PROBE") == "1" {
		contributionCLIBinary(t)
		t.Fatal("incomplete explicit candidate selection unexpectedly returned")
	}
	for _, input := range []struct{ name, path, digest string }{
		{"digest only", "", c.Digest([]byte("candidate bytes"))},
		{"path only", filepath.Join(t.TempDir(), "candidate"), ""},
	} {
		t.Run(input.name, func(t *testing.T) {
			probe := exec.Command(os.Args[0], "-test.run=^TestContributionInjectedBinarySelectionRejectsIncompletePair$")
			// A source-build fallback must fail differently from candidate validation.
			probe.Env = operationalTestEnvironmentWith(t.TempDir(), map[string]string{
				"PATH": t.TempDir(), "HN_CONTRIBUTION_TEST_SELECTION_PROBE": "1",
				"HN_CONTRIBUTION_TEST_BINARY": input.path, "HN_CONTRIBUTION_TEST_BINARY_SHA256": input.digest,
			})
			output, e := probe.CombinedOutput()
			if e == nil || !strings.Contains(string(output), "candidate binary requires absolute path and exact sha256 digest") {
				t.Fatalf("explicit incomplete selection did not fail before fallback: %v\n%s", e, output)
			}
		})
	}
}

func TestContributionCLIHiddenWorkflowKeepsRawPublicationWitness(t *testing.T) {
	// Provider proof is deliberately exercised with the standard adapter binary.
	// Native-only qualification selects the independent native journey tests.
	binary := buildMigrationCLIBinary(t, "")
	repo, _, _ := contributionEnabledRepository(t, binary, "hidden publication witness", []c.Rule{{Operation: c.WorkflowCreate, Automatic: true}})
	state := WorkflowState{Schema: "hn.workflow/1", Name: "ci", Title: "Published native workflow", Enabled: true, Pipeline: &PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "proof", Command: "true"}}}}
	workflow := contributionEventID(t, contributionCLI(t, binary, repo, true, "workflow", "open", "--input", contributionStateFile(t, state), "--operation", "published", "--json"))
	var mu sync.Mutex
	content := []byte("old workflow\n")
	writes := 0
	blob := func(data []byte) string {
		return fmt.Sprintf("%x", sha1.Sum(append([]byte(fmt.Sprintf("blob %d%c", len(data), 0)), data...)))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		hash := blob(content)
		commit := strings.Repeat("a", 40)
		switch {
		case r.URL.Path == "/repos/a/b/commits":
			json.NewEncoder(w).Encode([]any{map[string]any{"sha": commit, "commit": map[string]any{"tree": map[string]any{"sha": strings.Repeat("b", 40)}}}})
		case r.URL.Path == "/repos/a/b/actions/workflows":
			json.NewEncoder(w).Encode(map[string]any{"total_count": 1, "workflows": []any{map[string]any{"id": 1, "node_id": "W_1", "name": "CI", "path": ".github/workflows/ci.yml", "state": "active", "created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-02T03:04:05Z"}}})
		case strings.Contains(r.URL.Path, "/git/trees/"):
			name, mode, kind, sha := ".github", "040000", "tree", strings.Repeat("c", 40)
			if strings.HasSuffix(r.URL.Path, strings.Repeat("c", 40)) {
				name, sha = "workflows", strings.Repeat("d", 40)
			}
			if strings.HasSuffix(r.URL.Path, strings.Repeat("d", 40)) {
				name, mode, kind, sha = "ci.yml", "100644", "blob", hash
			}
			json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": name, "mode": mode, "type": kind, "sha": sha}}})
		case r.URL.Path == "/repos/a/b/contents/.github/workflows/ci.yml" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"type": "file", "path": ".github/workflows/ci.yml", "sha": hash, "size": len(content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(content)})
		case r.URL.Path == "/repos/a/b/contents/.github/workflows/ci.yml" && r.Method == "PUT":
			var input map[string]string
			if e := json.NewDecoder(r.Body).Decode(&input); e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			if input["sha"] != hash || input["branch"] != "main" {
				w.WriteHeader(409)
				return
			}
			next, e := base64.StdEncoding.DecodeString(input["content"])
			if e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			content = next
			writes++
			json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"path": ".github/workflows/ci.yml", "sha": blob(content)}, "commit": map[string]any{"sha": commit}})
		default:
			t.Errorf("unexpected workflow fixture request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	call := func(ok bool, args ...string) string {
		t.Helper()
		args = append(args, "--provider", "github", "--resource", "workflow", "--project", "a/b", "--endpoint", server.URL)
		return contributionCLIEnvironment(t, binary, repo, map[string]string{"HN_GITHUB_TOKEN": "test-only-token"}, ok, append([]string{"transfer"}, args...)...)
	}
	call(true, "export", workflow, "--map", "format=actions", "--map", "runner=ubuntu-24.04", "--map", "trigger=workflow_dispatch", "--map", "checkout=none", "--map", "runtime=python3-posix", "--map", "branch=main", "--map", "path=.github/workflows/ci.yml", "--map", "expected-sha="+blob([]byte("old workflow\n")))
	mu.Lock()
	published := append([]byte{}, content...)
	count := writes
	mu.Unlock()
	if count != 1 || !strings.Contains(string(published), "# hn-workflow/1 ") {
		t.Fatal("public export did not produce an exact publication")
	}
	positive := call(false, "import", "--dry-run")
	if !strings.Contains(positive, "existing signed native workflow") {
		t.Fatal("publication positive control failed: " + positive)
	}
	contributionCLI(t, binary, repo, true, "contribution", "decide", "hide", "--revision", workflow, "--expect-heads", "absent", "--json")
	before := contributionGit(t, repo, "show-ref")
	after := call(false, "import", "--dry-run")
	if !strings.Contains(after, "existing signed native workflow") {
		t.Fatal("hidden raw publication witness was lost: " + after)
	}
	if before != contributionGit(t, repo, "show-ref") {
		t.Fatal("publication dry-run changed native refs")
	}
	if strings.Contains(contributionCLI(t, binary, repo, true, "workflow", "list", "--json"), "Published native workflow") {
		t.Fatal("raw publication proof leaked into governed query")
	}
	if !strings.Contains(contributionCLI(t, binary, repo, true, "contribution", "inspect", "--raw", workflow, "--json"), "Published native workflow") {
		t.Fatal("original signed publication disappeared")
	}
}
