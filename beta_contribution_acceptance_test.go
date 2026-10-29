package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	c "hubnot/internal/contribution"
)

// These journeys use the selected executable, including in explicit package
// mode. An outer go test tag does not choose the tags of a compiled child.
func betaContributionBinary(t *testing.T, native bool) string {
	t.Helper()
	var binary string
	if native && *portablePackageDirectory == "" {
		binary = buildMigrationCLIBinary(t, "nogithub,noforge")
	} else if native {
		binary = portableJourneyBinary(t, "noforge")
	} else {
		binary = portableJourneyBinary(t, "")
	}
	info, err := exec.Command("go", "version", "-m", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("selected executable build information: %v\n%s", err, info)
	}
	t.Logf("selected executable build information:\n%s", info)
	return binary
}

type betaContributionCLI struct {
	t      *testing.T
	binary string
	digest [32]byte
	env    []string
}

func newBetaContributionCLI(t *testing.T, binary string) *betaContributionCLI {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	if err = os.Symlink(git, filepath.Join(path, "git")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	f := &betaContributionCLI{t: t, binary: binary, digest: sha256.Sum256(raw), env: []string{
		"PATH=" + path, "HOME=" + t.TempDir(), "LANG=C", "LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=",
	}}
	t.Logf("actual public executable=%s sha256:%x; closed environment, Git-only PATH", binary, f.digest)
	return f
}

func (f *betaContributionCLI) process(dir, program string, ok bool, args ...string) []byte {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	p := exec.CommandContext(ctx, program, args...)
	p.Dir, p.Env = dir, f.env
	out, err := p.CombinedOutput()
	if evidence := os.Getenv("HN_CONTRIBUTION_TEST_TRANSCRIPTS"); evidence != "" {
		file, e := os.CreateTemp(evidence, "beta-cli-*.log")
		if e != nil {
			f.t.Fatal(e)
		}
		_, e = fmt.Fprintf(file, "test: %s\nprogram: %s\nsha256: %x\ncwd: %s\nargv: %q\nexit: %v\noutput:\n%s", f.t.Name(), program, f.digest, dir, args, err, out)
		closeErr := file.Close()
		if e != nil || closeErr != nil {
			f.t.Fatalf("transcript: %v %v", e, closeErr)
		}
	}
	if ctx.Err() != nil || (err == nil) != ok {
		f.t.Fatalf("%s %q: %v, deadline=%v\n%s", program, args, err, ctx.Err(), out)
	}
	return out
}

func (f *betaContributionCLI) hn(dir string, ok bool, args ...string) string {
	f.t.Helper()
	raw, err := os.ReadFile(f.binary)
	if err != nil || sha256.Sum256(raw) != f.digest {
		f.t.Fatalf("selected executable changed: %v", err)
	}
	return string(f.process(dir, f.binary, ok, args...))
}

func (f *betaContributionCLI) git(dir string, args ...string) string {
	f.t.Helper()
	return strings.TrimSpace(string(f.process(dir, "git", true, args...)))
}

func betaContributionJSON[T any](t *testing.T, output string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		t.Fatalf("JSON: %v\n%s", err, output)
	}
	return v
}

func betaContributionContains(t *testing.T, output string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(output, needle) {
			t.Fatalf("missing %q in:\n%s", needle, output)
		}
	}
}

func (f *betaContributionCLI) repository() string {
	f.t.Helper()
	repo := f.t.TempDir()
	f.git(repo, "init", "--quiet", "--initial-branch=main")
	f.commit(repo, "Initial repository")
	return repo
}

func (f *betaContributionCLI) commit(repo, message string) {
	f.t.Helper()
	f.git(repo, "add", ".")
	f.git(repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "--allow-empty", "-m", message)
}

func (f *betaContributionCLI) actor(repo string) string {
	f.t.Helper()
	var result struct {
		Data struct {
			Actor string `json:"actor"`
		} `json:"data"`
	}
	result = betaContributionJSON[struct {
		Data struct {
			Actor string `json:"actor"`
		} `json:"data"`
	}](f.t, f.hn(repo, true, "identity", "show", "--json"))
	if result.Data.Actor == "" {
		f.t.Fatal("missing public actor")
	}
	return result.Data.Actor
}

func betaContributionWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func (f *betaContributionCLI) enabled(name string, limits c.Limits, nativeChecks bool, moderators ...string) (string, string, PolicyDocument) {
	f.t.Helper()
	repo := f.repository()
	f.hn(repo, true, "init", "--name", "Project maintainer")
	actor := f.actor(repo)
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
		Contributions: &c.Policy{Project: c.Digest([]byte(name)), Moderators: []string{actor}, Baseline: []string{}, Limits: limits,
			Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}, {Operation: c.IssueReply, Automatic: true}, {Operation: c.IssueEdit, Automatic: true},
				{Operation: c.DiscussionCreate, Automatic: true}, {Operation: c.DiscussionReply, Automatic: true}, {Operation: c.DiscussionEdit, Automatic: true}, {Operation: c.ProposalCreate}}}}
	policy.Maintainers = append(policy.Maintainers, moderators...)
	policy.Contributions.Moderators = append(policy.Contributions.Moderators, moderators...)
	sort.Strings(policy.Maintainers)
	sort.Strings(policy.Contributions.Moderators)
	if nativeChecks {
		policy.Proposals.RequiredApprovals = 1
		policy.Proposals.TrustedReviewers = []string{actor}
		policy.Pipelines["proof"] = PipelinePolicy{RequiredResults: 1, TrustedRunners: []string{actor}}
		betaContributionWrite(f.t, filepath.Join(repo, ".hn/pipelines/proof.json"), []byte(`{"version":"hn.pipeline/0","steps":[{"name":"Must require explicit execution","command":"git","args":["--version"]}]}`), 0600)
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		f.t.Fatal(err)
	}
	betaContributionWrite(f.t, filepath.Join(repo, ".hn/policy.json"), raw, 0600)
	f.commit(repo, "Explicit contribution policy")
	f.hn(repo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	f.hn(repo, true, "policy", "show", "main")
	f.hn(repo, true, "policy", "check", "--base", "main", "--file", ".hn/policy.json")
	f.hn(repo, true, "policy", "check", "--base", "main", "--head", "main")
	return repo, actor, policy
}

func betaContributionLimits() c.Limits {
	l := c.DefaultLimits()
	l.WireBytes, l.DecodedBytes, l.Objects, l.Events = 128<<10, 1<<20, 1024, 256
	return l
}

type betaContributionArtifact struct {
	Digest, Submission, Revision string
	Refs                         []string
	File                         string
}

func (f *betaContributionCLI) prepare(repo, project, revision string) betaContributionArtifact {
	f.t.Helper()
	file := filepath.Join(f.t.TempDir(), "contribution.hnc")
	a := betaContributionJSON[betaContributionArtifact](f.t, f.hn(repo, true, "contribution", "prepare", "--project", project, "--revision", revision, "--output", file, "--json"))
	a.File = file
	if a.Digest == "" || a.Submission == "" || len(a.Refs) == 0 {
		f.t.Fatalf("incomplete handoff: %+v", a)
	}
	return a
}

func (f *betaContributionCLI) receive(repo, operation string, a betaContributionArtifact, ok bool) string {
	f.t.Helper()
	args := []string{"contribution", "receive", "--file", a.File, "--digest", a.Digest, "--operation", operation, "--json"}
	for _, ref := range a.Refs {
		args = append(args, "--ref", ref)
	}
	return f.hn(repo, ok, args...)
}

func (f *betaContributionCLI) latest(repo, actor string) string {
	f.t.Helper()
	payload := f.process(repo, "git", true, "show", "refs/hn/actors/"+actor+":event.json")
	return fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
}

func (f *betaContributionCLI) raw(repo, revision string) string {
	f.t.Helper()
	return f.hn(repo, true, "contribution", "inspect", "--raw", revision, "--json")
}

func (f *betaContributionCLI) decide(repo, action, targetFlag, target, parents, operation string) string {
	f.t.Helper()
	return contributionDecisionID(f.t, f.hn(repo, true, "contribution", "decide", action, targetFlag, target, "--expect-heads", parents, "--operation", operation, "--json"))
}

func TestBetaContributionPublicVariants(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "standard"
		if native {
			name = "native-only"
		}
		t.Run(name, func(t *testing.T) {
			binary := betaContributionBinary(t, native)
			t.Run("contributor-maintainer-keyless-observer", func(t *testing.T) {
				betaContributionPublicJourney(newBetaContributionCLI(t, binary))
			})
			t.Run("signed-spam-and-durable-aggregate-accounting", func(t *testing.T) {
				betaContributionPublicAbuse(newBetaContributionCLI(t, binary))
			})
			t.Run("concurrent-moderation-revocation-and-disable", func(t *testing.T) {
				betaContributionPublicModeration(newBetaContributionCLI(t, binary))
			})
			t.Run("interrupted-status-with-source-fault-fixture", func(t *testing.T) {
				betaContributionInterruptedStatus(newBetaContributionCLI(t, binary))
			})
		})
	}
}

// The failure is injected through the source adapter. Only the selected actual
// CLI performs retry/reconciliation/publication; the test never calls Recover.
// This is source-assisted interruption setup, not process-kill evidence.
func betaContributionInterruptedStatus(f *betaContributionCLI) {
	for _, mode := range []string{"receive", "restore-selected", "restore-unselected"} {
		for _, condition := range []string{"durable", "missing-retained", "corrupt-retained"} {
			f.t.Run(mode+"/"+condition, func(t *testing.T) {
				betaContributionRecoveryCase(newBetaContributionCLI(t, f.binary), mode, condition)
			})
		}
	}
}

type betaContributionIntakeSnapshot struct {
	Attempt ContributionIntake `json:"attempt"`
	Ledger  c.Ledger           `json:"ledger"`
}

func betaContributionRecoveryCase(f *betaContributionCLI, mode, condition string) {
	t := f.t
	owner, maintainer, policy := f.enabled(t.Name(), betaContributionLimits(), false)
	receiver := owner
	if mode == "restore-unselected" {
		receiver = filepath.Join(t.TempDir(), "keyless-receiver")
		f.git(owner, "clone", "--quiet", owner, receiver)
		// An explicit contribution command creates safe private control paths,
		// while reporting that no authority or identity has been selected.
		f.hn(receiver, false, "contribution", "authority", "show", "--json")
	}
	contributor := f.repository()
	f.hn(contributor, true, "init", "--name", "Interrupted contributor")
	issue := contributionEventID(t, f.hn(contributor, true, "issue", "open", "--json", "Durable interrupted public issue"))
	original := f.raw(contributor, issue)
	a := f.prepare(contributor, policy.Contributions.Project, issue)
	x, err := OpenContributionExchange(filepath.Join(receiver, ".git/hn/contribution/inbox-"+policy.Contributions.Project[7:]), *policy.Contributions, "initial")
	if err != nil {
		t.Fatal(err)
	}
	step := "artifact-durable"
	if condition == "missing-retained" {
		step = "reserved"
	}
	x.fault = func(at string) error {
		if at == step {
			return fmt.Errorf("source fixture interrupted at %s", step)
		}
		return nil
	}
	attempt, err := x.Receive(context.Background(), a.File, a.Digest, "interrupted-public", a.Refs)
	if err == nil || condition != "missing-retained" && (attempt.Status != "uncertain" || attempt.WireBytes <= 0) || condition == "missing-retained" && attempt.Status != "reserved" {
		t.Fatalf("valid interruption not reached: %+v %v", attempt, err)
	}
	if condition == "corrupt-retained" {
		betaContributionWrite(t, x.artifactPath(a.Digest), []byte("corrupted receiver-retained artifact"), 0600)
	}
	if err = os.Remove(a.File); err != nil {
		t.Fatal(err)
	}
	// Inspect accounting through the public CLI for selected authority. A raw
	// unselected restore intentionally has no public authority-dependent reader;
	// its source snapshot remains explicitly labeled as setup/accounting evidence.
	before, _, err := x.Accounting()
	if err != nil {
		t.Fatal(err)
	}
	if mode != "restore-unselected" {
		public := betaContributionJSON[betaContributionIntakeSnapshot](t, f.hn(receiver, true, "contribution", "intake", "--operation", "interrupted-public", "--json"))
		if public.Attempt.Artifact != a.Digest || public.Attempt.Reserved != attempt.Reserved || !reflect.DeepEqual(public.Ledger, before) {
			t.Fatal("public pre-recovery intake identity/accounting differs from injected setup")
		}
		before = public.Ledger
	}
	refs := f.git(receiver, "show-ref")
	verb := "receive"
	if mode != "receive" {
		verb = "restore"
	}
	args := []string{"contribution", verb, "--file", a.File, "--digest", a.Digest, "--operation", "interrupted-public", "--json"}
	for _, ref := range a.Refs {
		args = append(args, "--ref", ref)
	}
	if condition != "durable" {
		betaContributionContains(t, f.hn(receiver, false, args...), "intake remains uncertain")
		after, _, e := x.Accounting()
		if e != nil || !reflect.DeepEqual(before, after) || refs != f.git(receiver, "show-ref") {
			t.Fatal("unrecoverable input changed reservation or promoted facts", e)
		}
		if strings.Contains(f.hn(receiver, true, "issue", "list", "--json"), "Durable interrupted public issue") {
			t.Fatal("missing/corrupt retained bytes became shared content")
		}
		return
	}
	output := f.hn(receiver, true, args...)
	betaContributionContains(t, output, `"status":"pending"`)
	after, attempts, err := x.Accounting()
	if err != nil {
		t.Fatal(err)
	}
	if mode != "restore-unselected" {
		public := betaContributionJSON[betaContributionIntakeSnapshot](t, f.hn(receiver, true, "contribution", "intake", "--operation", "interrupted-public", "--json"))
		if public.Attempt.Status != "pending" || public.Attempt.Artifact != a.Digest || !reflect.DeepEqual(public.Ledger, after) {
			t.Fatal("public recovered identity/accounting differs from retained receiver state")
		}
		after = public.Ledger
	}
	originalReservations := 0
	for _, reservation := range after.Reservations {
		if reservation.Intent == "interrupted-public" {
			originalReservations++
			if reservation.Cost != attempt.Reserved || reservation.Artifact != a.Digest {
				t.Fatal("recovery reinterpreted original charged input")
			}
		}
	}
	if originalReservations != 1 {
		t.Fatalf("original input charged %d times", originalReservations)
	}
	found := false
	for _, retained := range attempts {
		if retained.Intent == "interrupted-public" {
			found = true
			if retained.Status != "pending" || retained.Reserved != attempt.Reserved || retained.Artifact != a.Digest || !reflect.DeepEqual(retained.Refs, a.Refs) {
				t.Fatalf("recovered identity/reservation changed: %+v", retained)
			}
		}
	}
	if !found {
		t.Fatal("original intake identity disappeared")
	}
	if mode == "receive" {
		// Publishing the legitimate signed receipt is a separate charged control.
		if after.RetainedUsed.Requests != before.RetainedUsed.Requests+1 {
			t.Fatal("missing/double charged receipt control")
		}
		betaContributionContains(t, f.hn(receiver, true, "issue", "list", "--json"), "Durable interrupted public issue")
		if f.raw(receiver, issue) != original || f.actor(receiver) != maintainer {
			t.Fatal("recovery changed original native bytes/identity")
		}
	} else {
		if after.RetainedUsed != before.RetainedUsed {
			t.Fatal("raw restore added a receipt charge or refunded input")
		}
		if strings.Contains(f.hn(receiver, true, "issue", "list", "--json"), "Durable interrupted public issue") {
			t.Fatal("raw restore manufactured admission")
		}
		if mode == "restore-selected" {
			betaContributionContains(t, f.hn(receiver, true, "contribution", "pending", "--json"), a.Submission)
			if f.raw(receiver, issue) != original {
				t.Fatal("raw selected restore changed signed bytes")
			}
		} else {
			betaContributionContains(t, output, `"authorityAdopted":false`, `"nativePublished":false`)
			f.hn(receiver, false, "identity", "show", "--json")
			f.hn(receiver, false, "contribution", "authority", "show", "--json")
		}
	}
	recoveredRefs := f.git(receiver, "show-ref")
	f.hn(receiver, true, args...)
	replayed, _, err := x.Accounting()
	if err != nil || !reflect.DeepEqual(after, replayed) || recoveredRefs != f.git(receiver, "show-ref") {
		t.Fatal("exact recovered replay charged or rewrote history", err)
	}
	t.Logf("source-assisted %s interruption; actual CLI recovered from immutable retained bytes after foreign-file removal; original reservation once, total retained=%+v", mode, after.RetainedUsed)
}

func betaContributionPublicJourney(f *betaContributionCLI) {
	t := f.t
	owner, maintainer, policy := f.enabled(t.Name(), betaContributionLimits(), true)
	// Ordinary collaboration without a selected authority creates no gate or
	// contribution records, even when a cloned branch contains optional policy.
	upstream := filepath.Join(t.TempDir(), "upstream.git")
	f.git(owner, "clone", "--quiet", "--bare", owner, upstream)
	betaContributionWrite(t, filepath.Join(upstream, "hooks/pre-receive"), []byte("#!/bin/sh\necho 'WP13 upstream publication denied by configured receive policy' >&2\nexit 1\n"), 0700)
	contributor := filepath.Join(t.TempDir(), "contributor")
	f.git(owner, "clone", "--quiet", upstream, contributor)
	f.hn(contributor, true, "init", "--name", "Unknown contributor")
	actor := f.actor(contributor)
	if actor == maintainer {
		t.Fatal("contributor inherited maintainer actor")
	}
	issue := contributionEventID(t, f.hn(contributor, true, "issue", "open", "--operation", "ordinary-issue", "--json", "Stranger issue"))
	if _, err := os.Stat(filepath.Join(contributor, ".git/hn/contribution")); !os.IsNotExist(err) {
		t.Fatal("default-off collaboration created contribution state", err)
	}
	f.hn(contributor, false, "contribution", "authority", "show", "--json")
	betaContributionContains(t, f.hn(contributor, false, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json"), "requires current native maintainer")
	f.hn(contributor, false, "contribution", "decide", "hide", "--revision", issue, "--expect-heads", "absent", "--json")
	issueRaw := f.raw(contributor, issue)
	issueArtifact := f.prepare(contributor, policy.Contributions.Project, issue)
	f.receive(owner, "issue-intake", issueArtifact, true)
	betaContributionContains(t, f.hn(owner, true, "issue", "list", "--json"), "Stranger issue")
	if issueRaw != f.raw(owner, issue) {
		t.Fatal("intake changed original issue payload/signature")
	}
	// Exact retry works without reopening the source, and changed intent refuses.
	if err := os.Remove(issueArtifact.File); err != nil {
		t.Fatal(err)
	}
	replayRefs := f.git(owner, "show-ref")
	replayLedger := f.hn(owner, true, "contribution", "intake", "--operation", "issue-intake", "--json")
	f.receive(owner, "issue-intake", issueArtifact, true)
	changed := issueArtifact
	changed.Digest = c.Digest([]byte("changed bytes"))
	betaContributionContains(t, f.receive(owner, "issue-intake", changed, false), "conflicting operation intent")
	if replayRefs != f.git(owner, "show-ref") || replayLedger != f.hn(owner, true, "contribution", "intake", "--operation", "issue-intake", "--json") {
		t.Fatal("exact replay/changed intent mutated refs or ledger")
	}
	before := f.git(owner, "show-ref")
	betaContributionContains(t, f.hn(owner, false, "issue", "close", issue, "--expect", issue, "--operation", "foreign-edit", "--json"), "original author required")
	if before != f.git(owner, "show-ref") {
		t.Fatal("foreign edit changed refs")
	}
	reply := contributionEventID(t, f.hn(contributor, true, "issue", "comment", issue, "--body", "Stranger issue reply", "--json"))
	f.receive(owner, "issue-reply-intake", f.prepare(contributor, policy.Contributions.Project, reply), true)
	betaContributionContains(t, f.hn(owner, true, "issue", "history", issue, "--kind", "comments", "--json"), "Stranger issue reply")
	thread := contributionEventID(t, f.hn(contributor, true, "discussion", "open", "--title", "Stranger discussion", "--body", "Question", "--category", "help", "--json"))
	f.receive(owner, "discussion-intake", f.prepare(contributor, policy.Contributions.Project, thread), true)
	answer := contributionEventID(t, f.hn(contributor, true, "discussion", "reply", "add", thread, "--body", "Stranger discussion reply", "--json"))
	f.receive(owner, "discussion-reply-intake", f.prepare(contributor, policy.Contributions.Project, answer), true)
	betaContributionContains(t, f.hn(owner, true, "discussion", "replies", thread, "--json"), "Stranger discussion reply")
	f.git(contributor, "switch", "--quiet", "-c", "patch")
	betaContributionWrite(t, filepath.Join(contributor, "contributed.txt"), []byte("Original contributor bytes\n"), 0600)
	f.commit(contributor, "Contributor code")
	head := f.git(contributor, "rev-parse", "HEAD")
	f.git(contributor, "cat-file", "-e", head+"^{commit}")
	push := string(f.process(contributor, "git", false, "push", "origin", "HEAD:refs/heads/contributor-patch"))
	betaContributionContains(t, push, "WP13 upstream publication denied by configured receive policy", "pre-receive hook declined")
	if f.git(owner, "--git-dir="+upstream, "for-each-ref", "--format=%(refname)", "refs/heads/contributor-patch") != "" {
		t.Fatal("denied upstream branch appeared")
	}
	// The identical valid source/ref can be published when the receive hook is
	// deliberately absent on a separate disposable control repository.
	control := filepath.Join(t.TempDir(), "writable-control.git")
	f.git(owner, "clone", "--quiet", "--bare", upstream, control)
	f.git(contributor, "push", control, "HEAD:refs/heads/contributor-patch")
	if f.git(owner, "--git-dir="+control, "rev-parse", "refs/heads/contributor-patch") != head {
		t.Fatal("positive push control failed to publish exact source object")
	}
	base := f.git(owner, "rev-parse", "main")
	f.hn(contributor, true, "proposal", "open", "--base", base, "--head", head, "Stranger code proposal")
	proposal := f.latest(contributor, actor)
	proposalArtifact := f.prepare(contributor, policy.Contributions.Project, proposal)
	f.receive(owner, "proposal-intake", proposalArtifact, true)
	betaContributionContains(t, f.hn(owner, true, "contribution", "pending", "--json"), proposalArtifact.Submission)
	// Pending facts are excluded from the operative native catalog.
	betaContributionContains(t, f.hn(owner, false, "merge", proposal), "not found")
	f.decide(owner, "admit", "--submission", proposalArtifact.Submission, "absent", "admit-proposal")
	betaContributionContains(t, f.hn(owner, true, "proposal", "list"), "Stranger code proposal")
	// Admission supplies neither review, required CI results, nor acceptance.
	f.hn(owner, false, "merge", proposal)
	betaContributionContains(t, f.hn(owner, true, "proposal", "status", proposal), "0/1 trusted approvals", "proof 0/1 trusted passes")
	betaContributionContains(t, f.hn(owner, false, "decide", proposal, "--accept"), "not ready")
	f.hn(owner, true, "review", proposal, "--approve", "--body", "Exact proposal reviewed")
	betaContributionContains(t, f.hn(owner, true, "proposal", "status", proposal), "1/1 trusted approvals", "proof 0/1 trusted passes")
	betaContributionContains(t, f.hn(owner, false, "decide", proposal, "--accept"), "not ready")
	f.hn(owner, true, "run", "request", proposal, "proof")
	request := f.latest(owner, maintainer)
	refsBeforeExecution := f.git(owner, "show-ref")
	betaContributionContains(t, f.hn(owner, false, "run", "execute", request, "--backend", "host"), "allow-unsafe-host-execution")
	if refsBeforeExecution != f.git(owner, "show-ref") {
		t.Fatal("host execution refusal appended evidence")
	}
	betaContributionContains(t, f.hn(owner, false, "runner", "once"), "unsupported under enabled contribution policy")
	betaContributionContains(t, f.hn(owner, false, "run", "list"), "raw history")
	// Project moderation changes visibility while exact signed history survives.
	hide := f.decide(owner, "hide", "--revision", issue, "absent", "hide-issue")
	if strings.Contains(f.hn(owner, true, "issue", "list", "--json"), "Stranger issue") {
		t.Fatal("hidden issue remained visible")
	}
	if f.raw(owner, issue) != issueRaw {
		t.Fatal("hide changed signed bytes")
	}
	f.decide(owner, "restore", "--revision", issue, hide, "restore-issue")
	betaContributionContains(t, f.hn(owner, true, "issue", "history", issue, "--kind", "comments", "--json"), "Stranger issue reply")
	block := f.decide(owner, "block", "--actor", actor, "absent", "block-author")
	if strings.Contains(f.hn(owner, true, "discussion", "list", "--json"), "Stranger discussion") {
		t.Fatal("blocked author remained visible")
	}
	f.decide(owner, "unblock", "--actor", actor, block, "unblock-author")
	finalHide := f.decide(owner, "hide", "--revision", answer, "absent", "hide-answer")
	// A mixed trusted backup explicitly lists its native and contribution refs.
	// It is not an assertion that the restricted public outer profile can carry
	// native review/run history or that ordinary hn sync copies contribution refs.
	refs := strings.Fields(f.git(owner, "for-each-ref", "--format=%(refname)", "refs/heads/main", "refs/hn"))
	bundle := filepath.Join(t.TempDir(), "selected-public.bundle")
	f.git(owner, append([]string{"bundle", "create", bundle}, refs...)...)
	observer := filepath.Join(t.TempDir(), "observer")
	f.git(owner, "clone", "--quiet", "--branch", "main", bundle, observer)
	for _, ref := range refs {
		if strings.HasPrefix(ref, "refs/hn/") {
			f.git(observer, "fetch", "--quiet", bundle, ref+":"+ref)
		}
	}
	// Compare every retained original contribution envelope through explicit raw
	// public inspection, in addition to selected native payload/signature pairs.
	var records struct {
		Records []struct {
			ID string `json:"id"`
		} `json:"records"`
	}
	records = betaContributionJSON[struct {
		Records []struct {
			ID string `json:"id"`
		} `json:"records"`
	}](t, f.hn(owner, true, "contribution", "inspect", "--json"))
	for _, record := range records.Records {
		if f.raw(owner, record.ID) != f.raw(observer, record.ID) {
			t.Fatalf("original contribution envelope changed: %s", record.ID)
		}
	}
	f.hn(observer, false, "identity", "show", "--json")
	f.hn(observer, false, "contribution", "authority", "show", "--json")
	f.hn(observer, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--read-only", "--json")
	betaContributionContains(t, f.hn(observer, true, "issue", "history", issue, "--kind", "comments", "--json"), "Stranger issue reply")
	if strings.Contains(f.hn(observer, true, "discussion", "replies", thread, "--json"), "Stranger discussion reply") {
		t.Fatal("keyless reconstruction lost exact scoped hide")
	}
	betaContributionContains(t, f.hn(observer, true, "contribution", "pending", "--json"), finalHide)
	for _, revision := range []string{issue, reply, thread, answer, proposal} {
		if f.raw(contributor, revision) != f.raw(owner, revision) || f.raw(owner, revision) != f.raw(observer, revision) {
			t.Fatalf("signed payload/signature changed in public recovery: %s", revision)
		}
	}
	for _, ref := range refs {
		if strings.HasPrefix(ref, "refs/hn/") && f.git(owner, "rev-parse", ref) != f.git(observer, "rev-parse", ref) {
			t.Fatalf("selected historical ref changed: %s", ref)
		}
	}
	f.hn(observer, false, "contribution", "decide", "restore", "--revision", answer, "--expect-heads", finalHide, "--json")
	f.hn(observer, false, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	f.hn(observer, false, "identity", "show", "--json")
	if _, err := os.Stat(filepath.Join(observer, ".git/hn/identities")); !os.IsNotExist(err) {
		t.Fatal("public recovery installed private identities", err)
	}
	for _, optional := range []string{"protectedChecks", "automerge", "audits"} {
		if strings.Contains(f.git(observer, "show", "main:.hn/policy.json"), `"`+optional+`"`) {
			t.Fatalf("contribution journey enabled %s", optional)
		}
	}
}

func (f *betaContributionCLI) archive(repo string) betaContributionArtifact {
	f.t.Helper()
	file := filepath.Join(f.t.TempDir(), "contributions.hnc")
	a := betaContributionJSON[betaContributionArtifact](f.t, f.hn(repo, true, "contribution", "archive", "--output", file, "--json"))
	a.File = file
	return a
}

func betaContributionPublicModeration(f *betaContributionCLI) {
	t := f.t
	moderatorRepo := f.repository()
	f.hn(moderatorRepo, true, "init", "--name", "Second project moderator")
	moderator := f.actor(moderatorRepo)
	owner, _, policy := f.enabled(t.Name(), betaContributionLimits(), false, moderator)
	issue := contributionEventID(t, f.hn(owner, true, "issue", "open", "--operation", "subject", "--json", "Concurrent moderation subject"))
	// A second legitimate project signer receives the same pre-decision snapshot.
	initial := f.archive(owner)
	f.git(moderatorRepo, "fetch", "--quiet", owner, "main")
	f.git(moderatorRepo, "reset", "--quiet", "--hard", "FETCH_HEAD")
	args := []string{"contribution", "restore", "--file", initial.File, "--digest", initial.Digest, "--operation", "moderator-snapshot", "--json"}
	for _, ref := range initial.Refs {
		args = append(args, "--ref", ref)
	}
	f.hn(moderatorRepo, true, args...)
	f.hn(moderatorRepo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	issueRaw := f.raw(owner, issue)
	hide := f.decide(owner, "hide", "--revision", issue, "absent", "concurrent-hide")
	restore := f.decide(moderatorRepo, "restore", "--revision", issue, "absent", "concurrent-restore")
	f.receive(owner, "concurrent-decisions", f.archive(moderatorRepo), true)
	pending := f.hn(owner, true, "contribution", "pending", "--json")
	betaContributionContains(t, pending, "conflicts", hide, restore)
	if strings.Contains(f.hn(owner, true, "issue", "list", "--json"), "Concurrent moderation subject") {
		t.Fatal("conflicting decisions silently chose a winner")
	}
	betaContributionContains(t, f.hn(owner, false, "contribution", "decide", "restore", "--revision", issue, "--expect-heads", "absent", "--operation", "stale-head", "--json"), "moderation heads changed")
	parents := []string{hide, restore}
	sort.Strings(parents)
	joined := f.decide(owner, "restore", "--revision", issue, strings.Join(parents, ","), "join-decisions")
	betaContributionContains(t, f.hn(owner, true, "issue", "list", "--json"), "Concurrent moderation subject")
	refs := f.git(owner, "show-ref")
	if repeated := f.decide(owner, "restore", "--revision", issue, strings.Join(parents, ","), "join-decisions"); repeated != joined || refs != f.git(owner, "show-ref") {
		t.Fatal("exact decision replay changed immutable history")
	}
	betaContributionContains(t, f.hn(owner, false, "contribution", "decide", "hide", "--revision", issue, "--expect-heads", strings.Join(parents, ","), "--operation", "join-decisions", "--json"), "conflicting decision operation intent")
	// Checkpoint only exact established facts through normal native governance,
	// then revoke the other moderator. Copied JSON alone is never ratification.
	checkpoint := func() {
		inspection := betaContributionJSON[struct {
			Records []struct {
				ID     string   `json:"id"`
				Record c.Record `json:"record"`
			} `json:"records"`
		}](t, f.hn(owner, true, "contribution", "inspect", "--json"))
		policy.ContributionCheckpoints = nil
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
	}
	amend := func(branch string) {
		t.Helper()
		base := f.git(owner, "rev-parse", "main")
		f.git(owner, "switch", "--quiet", "-c", branch, "main")
		raw, err := json.Marshal(policy)
		if err != nil {
			t.Fatal(err)
		}
		betaContributionWrite(t, filepath.Join(owner, ".hn/policy.json"), raw, 0600)
		f.commit(owner, branch)
		f.hn(owner, true, "proposal", "open", "--base", base, "--head", f.git(owner, "rev-parse", "HEAD"), branch)
		proposal := f.latest(owner, f.actor(owner))
		inspection := betaContributionJSON[struct {
			Records []struct {
				ID     string   `json:"id"`
				Record c.Record `json:"record"`
			} `json:"records"`
		}](t, f.hn(owner, true, "contribution", "inspect", "--json"))
		submission := ""
		for _, row := range inspection.Records {
			if row.Record.Submission != nil && row.Record.Submission.Content.Revision == proposal {
				submission = row.ID
			}
		}
		if submission == "" {
			t.Fatal("policy proposal submission missing")
		}
		f.decide(owner, "admit", "--submission", submission, "absent", "admit-"+branch)
		f.hn(owner, true, "decide", proposal, "--accept")
		f.git(owner, "switch", "--quiet", "main")
		f.git(owner, "config", "user.name", "Fixture")
		f.git(owner, "config", "user.email", "fixture@example.invalid")
		f.hn(owner, true, "merge", proposal)
	}
	checkpoint()
	policy.Contributions.Moderators = []string{f.actor(owner)}
	policy.Contributions.RevokedModerators = []string{moderator}
	amend("revoke-project-moderator")
	betaContributionContains(t, f.hn(owner, true, "contribution", "pending", "--json"), "revoked", restore)
	// A revoked original decision stays byte-exact and inspectable.
	if f.raw(owner, issue) != issueRaw {
		t.Fatal("revocation changed original content")
	}
	betaContributionContains(t, f.raw(owner, restore), moderator)
	checkpoint()
	policy.Contributions = nil
	amend("disable-contributions")
	betaContributionContains(t, f.hn(owner, true, "issue", "list", "--json"), "Concurrent moderation subject")
	betaContributionContains(t, f.hn(owner, true, "contribution", "authority", "show", "--json"), `"enabled":false`)
	if f.raw(owner, issue) != issueRaw {
		t.Fatal("disable changed signed native history")
	}
}

func betaContributionPublicAbuse(f *betaContributionCLI) {
	t := f.t
	l := betaContributionLimits()
	// Public automatic intake also retains a signed receipt-control operation.
	l.Window.Requests, l.Retained.Requests = 6, 6
	owner, _, policy := f.enabled(t.Name(), l, false)
	var first betaContributionArtifact
	var firstIssue string
	for i := 0; i < 3; i++ {
		repo := f.repository()
		f.hn(repo, true, "init", "--name", fmt.Sprintf("Independent spam key %d", i))
		issue := contributionEventID(t, f.hn(repo, true, "issue", "open", "--json", fmt.Sprintf("Signed message %d", i)))
		a := f.prepare(repo, policy.Contributions.Project, issue)
		f.receive(owner, fmt.Sprintf("key-%d", i), a, true)
		pending := betaContributionJSON[struct {
			Pending []string `json:"pending"`
		}](t, f.hn(owner, true, "contribution", "pending", "--json"))
		if len(pending.Pending) != 0 {
			t.Fatal("automatic intake unexpectedly needs manual admission")
		}
		intake := betaContributionJSON[struct {
			Attempt ContributionIntake `json:"attempt"`
			Ledger  c.Ledger           `json:"ledger"`
		}](t, f.hn(owner, true, "contribution", "intake", "--operation", fmt.Sprintf("key-%d", i), "--json"))
		if intake.Attempt.WireBytes <= 0 || intake.Attempt.DecodedBytes <= 0 || intake.Attempt.Objects <= 0 || intake.Attempt.Events <= 0 || intake.Ledger.RetainedUsed.Requests != int64(2*(i+1)) || intake.Ledger.RetainedUsed.Bytes <= 0 {
			t.Fatalf("unmeasured/unaccounted automatic intake: %+v", intake)
		}
		if i == 0 {
			first, firstIssue = a, issue
		}
	}
	before := f.git(owner, "show-ref")
	// The next independently signed, otherwise valid submission is refused too;
	// the absent-path control below proves the same refusal precedes source I/O.
	fourth := f.repository()
	f.hn(fourth, true, "init", "--name", "Fourth independent spam key")
	fourthIssue := contributionEventID(t, f.hn(fourth, true, "issue", "open", "--json", "Signed over-budget message"))
	fourthArtifact := f.prepare(fourth, policy.Contributions.Project, fourthIssue)
	betaContributionContains(t, f.receive(owner, "fourth-key", fourthArtifact, false), "budget exhausted")
	// Exhaustion must precede source opening, including after queue draining.
	missing := first
	missing.File = filepath.Join(t.TempDir(), "does-not-exist.hnc")
	missing.Digest = c.Digest([]byte("new signed spam"))
	betaContributionContains(t, f.receive(owner, "exhausted-new-key", missing, false), "budget exhausted")
	f.receive(owner, "key-0", first, true)
	if before != f.git(owner, "show-ref") {
		t.Fatal("saturated retry/refusal changed accepted refs")
	}
	f.hn(owner, true, "contribution", "rollover", "--expect", "initial", "--window", "second")
	betaContributionContains(t, f.receive(owner, "after-rollover", missing, false), "budget exhausted")
	betaContributionContains(t, f.hn(owner, true, "issue", "show", firstIssue, "--json"), "Signed message 0")
	// A separate bounded receiver proves malformed/oversized and out-of-selection
	// files never become shared facts and failed attempts remain charged.
	bounded, _, _ := f.enabled(t.Name(), betaContributionLimits(), false)
	refsBefore := f.git(bounded, "show-ref")
	oversize := filepath.Join(t.TempDir(), "oversize.hnc")
	betaContributionWrite(t, oversize, bytes.Repeat([]byte{'x'}, int(l.WireBytes)+1), 0600)
	bad := first
	bad.File, bad.Digest = oversize, c.Digest(bytes.Repeat([]byte{'x'}, int(l.WireBytes)+1))
	betaContributionContains(t, f.receive(bounded, "oversize", bad, false), "source exceeds reserved wire bytes")
	truncated := filepath.Join(t.TempDir(), "truncated.hnc")
	betaContributionWrite(t, truncated, []byte("hn.contribution-artifact/1\n"), 0600)
	bad.File, bad.Digest = truncated, c.Digest([]byte("hn.contribution-artifact/1\n"))
	betaContributionContains(t, f.receive(bounded, "truncated", bad, false), "unexpected EOF")
	bad = first
	bad.Refs = []string{"refs/heads/main"}
	betaContributionContains(t, f.receive(bounded, "unselected", bad, false), "unselected project or refs")
	if refsBefore != f.git(bounded, "show-ref") {
		t.Fatal("hostile intake changed shared refs")
	}
	for i, op := range []string{"oversize", "truncated", "unselected"} {
		got := betaContributionJSON[struct {
			Attempt ContributionIntake `json:"attempt"`
			Ledger  c.Ledger           `json:"ledger"`
		}](t, f.hn(bounded, true, "contribution", "intake", "--operation", op, "--json"))
		if got.Attempt.Status != "failed" || got.Ledger.RetainedUsed.Requests != 3 || got.Attempt.Reserved.WireBytes != l.WireBytes {
			t.Fatalf("failed charge %d: %+v", i, got)
		}
	}
}

// Source-adapter evidence supplements the compiled CLI journeys: instrumentation
// observes actual bytes read and deterministic interruption/cancellation. It is
// deliberately not counted as package runtime or human pilot evidence.
func TestBetaContributionMeasuredInterruptionAndCancellation(t *testing.T) {
	file, raw, digest, p, refs := exchangeArtifact(t)
	p = exchangeSmall(p, len(raw)+1)
	x := exchangeOpen(t, p)
	reads := 0
	x.readObserved = func(n int) { reads += n }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := x.Receive(ctx, file, digest, "canceled", refs); err == nil || reads != 0 {
		t.Fatalf("canceled input read %d bytes: %v", reads, err)
	}
	before, _, err := x.Accounting()
	if err != nil {
		t.Fatal(err)
	}
	// A regular file beyond the configured maximum must refuse without reading
	// even one content byte, while preserving its failed reservation.
	oversize := filepath.Join(t.TempDir(), "cap-plus-one.hnc")
	oversizeRaw := append(append([]byte{}, raw...), 0, 0)
	betaContributionWrite(t, oversize, oversizeRaw, 0600)
	if _, err = x.Receive(context.Background(), oversize, c.Digest(oversizeRaw), "oversize", refs); err == nil || reads != 0 {
		t.Fatalf("oversize source read=%d err=%v", reads, err)
	}
	before, _, err = x.Accounting()
	if err != nil || before.RetainedUsed.Requests != 2 {
		t.Fatal("failed oversize/cancel charges lost", err)
	}
	x.fault = func(step string) error {
		if step == "artifact-durable" {
			return fmt.Errorf("deterministic interruption after durable artifact")
		}
		return nil
	}
	attempt, err := x.Receive(context.Background(), file, digest, "interrupted", refs)
	if err == nil || attempt.Status != "uncertain" || reads != len(raw) {
		t.Fatalf("interruption reads=%d want=%d attempt=%+v err=%v", reads, len(raw), attempt, err)
	}
	charged, _, err := x.Accounting()
	if err != nil || charged.RetainedUsed.Requests != before.RetainedUsed.Requests+1 {
		t.Fatal("uncertain attempt lost reservation", err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	x.fault = nil
	recovered, err := x.Recover(context.Background(), "interrupted")
	if err != nil || recovered.Status != "pending" {
		t.Fatalf("durable retry %+v: %v", recovered, err)
	}
	after, _, err := x.Accounting()
	if err != nil || !reflect.DeepEqual(charged, after) {
		t.Fatal("recovery charged or refunded retained reservation", err)
	}
	t.Logf("source adapter: canceled reads=0; oversize reads=0; interrupted reads=%d; measured wire=%d decoded=%d objects=%d events=%d; retained=%+v", reads, recovered.WireBytes, recovered.DecodedBytes, recovered.Objects, recovered.Events, after.RetainedUsed)
}
