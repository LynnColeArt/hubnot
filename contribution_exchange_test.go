package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	c "hubnot/internal/contribution"
)

func exchangePolicy() c.Policy {
	return c.Policy{Project: c.Digest([]byte("exchange project")), Moderators: []string{c.Actor(bytes.Repeat([]byte{1}, 32))}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}
}
func exchangeFixture(t *testing.T) (string, *StoredEvent, c.Envelope, c.Policy, []string) {
	t.Helper()
	repo := inReplicationTestRepository(t)
	identity := testIdentity(t, "Stranger with no upstream permission")
	fact := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "Exact local issue" })
	p := exchangePolicy()
	key, e := identity.privateKey()
	if e != nil {
		t.Fatal(e)
	}
	envelope, e := c.Sign(c.Record{Version: c.Version, Kind: c.SubmissionKind, Project: p.Project, Actor: identity.Actor, PublicKey: identity.PublicKey, Intent: "issue-one", Submission: &c.Submission{Operation: c.IssueCreate, Content: c.Content{Root: fact.ID, Revision: fact.ID, Author: identity.Actor, Parents: []string{}, References: []c.Reference{}}}}, key)
	if e != nil {
		t.Fatal(e)
	}
	return repo, fact, envelope, p, []string{actorRef(identity.Actor)}
}
func TestContributionExchangeRegularFilePendingReplayAndKeylessRestore(t *testing.T) {
	repo, fact, envelope, p, refs := exchangeFixture(t)
	ctx := context.Background()
	artifact := filepath.Join(t.TempDir(), "issue.hnc")
	digest, e := ExportContributionArtifact(ctx, filepath.Join(repo, ".git"), p.Project, refs, []c.Envelope{envelope}, artifact, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := gitOutput("show-ref")
	store, e := OpenContributionExchange(filepath.Join(t.TempDir(), "inbox"), p, "initial")
	if e != nil {
		t.Fatal(e)
	}
	got, e := store.Receive(ctx, artifact, digest, "receive-one", refs)
	if e != nil || got.Status != "pending" {
		t.Fatalf("pending: %#v %v", got, e)
	}
	pending, e := store.Pending()
	if e != nil || len(pending) != 1 {
		t.Fatalf("pending list: %#v %v", pending, e)
	}
	verified, e := store.Inspect(ctx, "receive-one")
	if e != nil {
		t.Fatal(e)
	}
	defer verified.Close()
	if len(verified.Records) != 1 || !bytes.Equal(verified.Records[0].Payload(), envelope.Payload) || !bytes.Equal(verified.Records[0].Signature(), envelope.Signature) {
		t.Fatal("outer signature bytes changed")
	}
	native, e := verified.Native.Fact(fact.ID)
	if e != nil || native.Content.Revision != fact.ID || native.Actor != fact.Event.Actor {
		t.Fatalf("native: %#v %v", native, e)
	}
	raw := verified.Events[0]
	if !bytes.Equal(raw.Payload, fact.Payload) || !bytes.Equal(raw.Signature, fact.Signature) {
		t.Fatal("native signed bytes changed")
	}
	usage := got.Reserved
	if e = os.Remove(artifact); e != nil {
		t.Fatal(e)
	}
	replay, e := store.Receive(ctx, artifact, digest, "receive-one", refs)
	if e != nil || !replay.Replayed || replay.Reserved != usage {
		t.Fatalf("exact replay performed I/O or changed reservation: %#v %v", replay, e)
	}
	if _, e = store.Receive(ctx, artifact, c.Digest([]byte("different")), "receive-one", refs); e == nil {
		t.Fatal("changed intent accepted")
	}
	lower := p
	lower.Limits.WireBytes = 1
	lower.Limits.DecodedBytes = 1
	lower.Limits.Objects = 1
	lower.Limits.Events = 1
	lower.Limits.Depth = 1
	lower.Limits.Milliseconds = 1
	lowerStore, e := OpenContributionExchange(store.root, lower, "unchanged")
	if e != nil {
		t.Fatal(e)
	}
	if got, e := lowerStore.Receive(ctx, "/not-opened", digest, "receive-one", refs); e != nil || !got.Replayed || got.Status != "pending" {
		t.Fatal("lower policy lost exact completed receipt", e)
	}
	backup := filepath.Join(t.TempDir(), "backup.hnc")
	if e = store.Export(ctx, "receive-one", backup); e != nil {
		t.Fatal(e)
	}
	observer, e := OpenContributionExchange(filepath.Join(t.TempDir(), "keyless"), p, "observer")
	if e != nil {
		t.Fatal(e)
	}
	restored, e := observer.Receive(ctx, backup, digest, "restore-one", refs)
	if e != nil || restored.Status != "pending" {
		t.Fatalf("keyless restore %#v %v", restored, e)
	}
	after, _ := gitOutput("show-ref")
	if !bytes.Equal(before, after) {
		t.Fatal("intake changed accepted refs or branches")
	}
}

func exchangeArtifact(t *testing.T) (string, []byte, string, c.Policy, []string) {
	t.Helper()
	repo, _, env, p, refs := exchangeFixture(t)
	path := filepath.Join(t.TempDir(), "source.hnc")
	digest, e := ExportContributionArtifact(context.Background(), filepath.Join(repo, ".git"), p.Project, refs, []c.Envelope{env}, path, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return path, raw, digest, p, refs
}
func exchangeOpen(t *testing.T, p c.Policy) *ContributionExchange {
	t.Helper()
	x, e := OpenContributionExchange(filepath.Join(t.TempDir(), "inbox"), p, "first")
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func exchangeSmall(p c.Policy, wire int) c.Policy {
	p.Limits.WireBytes = int64(wire)
	p.Limits.DecodedBytes = 128 << 10
	p.Limits.Objects = 128
	p.Limits.Events = 32
	p.Limits.Depth = 32
	p.Limits.Milliseconds = 5000
	return p
}
func TestContributionExchangeRefusalChargesBeforeOpeningAndDoesNotConsumeExtraWire(t *testing.T) {
	path, raw, digest, p, refs := exchangeArtifact(t)
	p = exchangeSmall(p, len(raw))
	ctx := context.Background()
	x := exchangeOpen(t, p)
	reads := 0
	x.readObserved = func(n int) { reads += n }
	got, e := x.Receive(ctx, path, digest, "exact-wire", refs)
	if e != nil || got.Status != "pending" || reads != len(raw) || got.WireBytes != int64(len(raw)) {
		t.Fatalf("exact cap positive control %#v reads=%d %v", got, reads, e)
	}
	huge := filepath.Join(t.TempDir(), "huge")
	if e = os.WriteFile(huge, append(raw, 1), 0600); e != nil {
		t.Fatal(e)
	}
	reads = 0
	if _, e = x.Receive(ctx, huge, c.Digest(append(raw, 1)), "oversized", refs); e == nil || reads != 0 {
		t.Fatalf("oversized source read %d bytes: %v", reads, e)
	}
	truncated := filepath.Join(t.TempDir(), "truncated")
	cut := raw[:len(raw)/2]
	if e = os.WriteFile(truncated, cut, 0600); e != nil {
		t.Fatal(e)
	}
	reads = 0
	if _, e = x.Receive(ctx, truncated, c.Digest(cut), "truncated", refs); e == nil || reads != len(cut) {
		t.Fatalf("truncated: reads=%d err=%v", reads, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	reads = 0
	if _, e = x.Receive(canceled, path, c.Digest([]byte("distinct canceled attempt")), "canceled", refs); e == nil || reads != 0 {
		t.Fatalf("canceled source read %d bytes: %v", reads, e)
	}
	ledger, attempts, e := x.Accounting()
	if e != nil || len(attempts) != 4 || ledger.RetainedUsed.Requests != 4 {
		t.Fatalf("failed attempts escaped charges: %#v %#v %v", ledger, attempts, e)
	}
	if attempts[1].Status != "failed" || attempts[2].Status != "failed" || attempts[3].Status != "failed" {
		t.Fatal("failed metadata missing")
	}
	if pending, e := x.Pending(); e != nil || len(pending) != 1 {
		t.Fatal("refusal poisoned unrelated pending work", e)
	}
}
func TestContributionExchangeSpecialFilesSelectionsAndSignedMismatch(t *testing.T) {
	path, raw, digest, p, refs := exchangeArtifact(t)
	p = exchangeSmall(p, len(raw)+1024)
	ctx := context.Background()
	x := exchangeOpen(t, p)
	linked := filepath.Join(t.TempDir(), "link")
	if e := os.Symlink(path, linked); e != nil {
		t.Fatal(e)
	}
	for i, path := range []string{linked, filepath.Dir(path), "https://invalid.example/issue.hnc"} {
		if _, e := x.Receive(ctx, path, digest, fmt.Sprintf("bad-path-%d", i), refs); e == nil {
			t.Fatalf("nonregular source accepted: %s", path)
		}
	}
	fifo := filepath.Join(t.TempDir(), "fifo")
	if e := syscall.Mkfifo(fifo, 0600); e != nil {
		t.Fatal(e)
	}
	started := time.Now()
	if _, e := x.Receive(ctx, fifo, digest, "fifo", refs); e == nil || time.Since(started) > time.Second {
		t.Fatal("FIFO was opened or blocked", e)
	}
	wrong := []string{"refs/heads/main"}
	if _, e := x.Receive(ctx, path, digest, "unselected", wrong); e == nil {
		t.Fatal("unselected native ref accepted")
	}
	var m contributionManifest
	offset := len(contributionArtifactMagic)
	n := int(binary.BigEndian.Uint64(raw[offset : offset+8]))
	start := offset + 8
	if e := json.Unmarshal(raw[start:start+n], &m); e != nil {
		t.Fatal(e)
	}
	// Correct envelope signature but false native revision must fail the binding.
	identity := testIdentity(t, "Different signer")
	key, e := identity.privateKey()
	if e != nil {
		t.Fatal(e)
	}
	v, e := c.Verify(m.Envelopes[0].Payload, m.Envelopes[0].Signature)
	if e != nil {
		t.Fatal(e)
	}
	r := v.Record()
	r.Actor = identity.Actor
	r.PublicKey = identity.PublicKey
	r.Submission.Content.Author = identity.Actor
	env, e := c.Sign(r, key)
	if e != nil {
		t.Fatal(e)
	}
	m.Envelopes[0] = contributionWireEnvelope{env.Payload, env.Signature}
	manifest, _ := json.Marshal(m)
	altered := append([]byte{}, raw[:offset]...)
	var size bytes.Buffer
	_ = binary.Write(&size, binary.BigEndian, uint64(len(manifest)))
	altered = append(altered, size.Bytes()...)
	altered = append(altered, manifest...)
	altered = append(altered, raw[start+n:]...)
	forged := filepath.Join(t.TempDir(), "false-binding")
	if e = os.WriteFile(forged, altered, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = x.Receive(ctx, forged, c.Digest(altered), "signed-mismatch", refs); e == nil {
		t.Fatal("signed envelope impersonated authenticated native author")
	}
}
func TestContributionExchangeAggregateManyKeysAndWindowDoesNotRefund(t *testing.T) {
	repo, first, env, p, refs := exchangeFixture(t)
	ctx := context.Background()
	p = exchangeSmall(p, 64<<10)
	cost := contributionReservation(p.Limits)
	charge := cost.WireBytes + cost.DecodedBytes + c.LedgerEntryBytes
	p.Limits.Window = c.Usage{Bytes: charge * 2, Objects: cost.Objects * 2, Events: cost.Events * 2, Requests: 2}
	p.Limits.Retained = p.Limits.Window
	x := exchangeOpen(t, p)
	for i := 0; i < 3; i++ {
		identity := testIdentity(t, fmt.Sprintf("Sybil %d", i))
		fact := first
		submission := env
		selected := refs
		if i > 0 {
			fact = appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "Many keys" })
			key, _ := identity.privateKey()
			var e error
			submission, e = c.Sign(c.Record{Version: c.Version, Kind: c.SubmissionKind, Project: p.Project, Actor: identity.Actor, PublicKey: identity.PublicKey, Intent: "same-per-key", Submission: &c.Submission{Operation: c.IssueCreate, Content: c.Content{Root: fact.ID, Revision: fact.ID, Author: identity.Actor, Parents: []string{}, References: []c.Reference{}}}}, key)
			if e != nil {
				t.Fatal(e)
			}
			selected = []string{actorRef(identity.Actor)}
		}
		path := filepath.Join(t.TempDir(), "input.hnc")
		digest, e := ExportContributionArtifact(ctx, filepath.Join(repo, ".git"), p.Project, selected, []c.Envelope{submission}, path, p.Limits)
		if e != nil {
			t.Fatal(e)
		}
		reads := 0
		x.readObserved = func(n int) { reads += n }
		_, e = x.Receive(ctx, path, digest, fmt.Sprintf("many-key-%d", i), selected)
		if i < 2 && e != nil {
			t.Fatal(e)
		}
		if i == 2 && (e == nil || reads != 0) {
			t.Fatalf("many-key aggregate bypass/read: %d %v", reads, e)
		}
	}
	before, _, e := x.Accounting()
	if e != nil {
		t.Fatal(e)
	}
	if e = x.Rollover("wrong", "second", true); e == nil {
		t.Fatal("stale rollover accepted")
	}
	if e = x.Rollover("first", "second", false); e == nil {
		t.Fatal("unauthorized rollover accepted")
	}
	if e = x.Rollover("first", "second", true); e != nil {
		t.Fatal(e)
	}
	after, _, e := x.Accounting()
	if e != nil || after.RetainedUsed != before.RetainedUsed {
		t.Fatal("rollover refunded immutable retention", e)
	}
	if _, e = x.Receive(ctx, "/not-opened", c.Digest([]byte("new")), "after-rollover", refs); e == nil {
		t.Fatal("rollover bypassed retained saturation")
	}
	reopened, e := OpenContributionExchange(x.root, p, "new-window-must-not-reset")
	if e != nil {
		t.Fatal(e)
	}
	ledger, _, e := reopened.Accounting()
	if e != nil || ledger.RetainedUsed != before.RetainedUsed || ledger.Window != "second" {
		t.Fatal("restart reset accounting", e)
	}
}
func TestContributionExchangeInterruptedDurableArtifactAndUnknownJournal(t *testing.T) {
	path, raw, digest, p, refs := exchangeArtifact(t)
	p = exchangeSmall(p, len(raw)+1)
	ctx := context.Background()
	x := exchangeOpen(t, p)
	x.fault = func(step string) error {
		if step == "artifact-durable" {
			return errors.New("interrupted after fsync")
		}
		return nil
	}
	got, e := x.Receive(ctx, path, digest, "interrupted", refs)
	if e == nil || got.Status != "uncertain" {
		t.Fatalf("interruption %#v %v", got, e)
	}
	ledger, _, e := x.Accounting()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	x.fault = nil
	recovered, e := x.Recover(ctx, "interrupted")
	if e != nil || recovered.Status != "pending" {
		t.Fatalf("recovery %#v %v", recovered, e)
	}
	after, _, e := x.Accounting()
	if e != nil || after.RetainedUsed != ledger.RetainedUsed {
		t.Fatal("recovery charged or refunded", e)
	}
	reserved := exchangeOpen(t, p)
	reserved.fault = func(step string) error { return errors.New("crash before open") }
	if _, e = reserved.Receive(ctx, "/absent", digest, "no-artifact", refs); e == nil {
		t.Fatal("fault not reached")
	}
	reserved.fault = nil
	if _, e = reserved.Recover(ctx, "no-artifact"); e == nil {
		t.Fatal("missing artifact guessed complete")
	}
	journal := filepath.Join(x.root, "journal.json")
	saved, e := os.ReadFile(journal)
	if e != nil {
		t.Fatal(e)
	}
	bad := bytes.Replace(saved, []byte(contributionJournalVersion), []byte("hn.contribution-intake/999"), 1)
	if e = os.WriteFile(journal, bad, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = OpenContributionExchange(x.root, p, "fresh"); e == nil {
		t.Fatal("unknown journal reset")
	}
	if e = os.Remove(journal); e != nil {
		t.Fatal(e)
	}
	if _, e = OpenContributionExchange(x.root, p, "fresh"); e == nil {
		t.Fatal("missing journal reset")
	}
}
func TestContributionExchangeNativeObjectDepthCountsAndCompressedMaterialBeforeGit(t *testing.T) {
	_, raw, _, p, refs := exchangeArtifact(t)
	ctx := context.Background()
	for name, mutate := range map[string]func(*c.Limits){"decoded": func(l *c.Limits) { l.DecodedBytes = 64 }, "objects": func(l *c.Limits) { l.Objects = 2 }, "events": func(l *c.Limits) { l.Events = 1 }, "depth": func(l *c.Limits) { l.Depth = 1 }} {
		t.Run(name, func(t *testing.T) {
			l := p.Limits
			mutate(&l)
			if a, e := readContributionArtifact(ctx, bytes.NewReader(raw), p.Project, refs, l); e == nil {
				a.Close()
				t.Fatal("native budget ignored")
			}
		})
	}
	// An opaque compressed pack named by a tree must be rejected without invoking
	// Git, zlib or audit readers. Invalid object hashes are intentional: preflight
	// must discover the forbidden shape before any object importer is reached.
	oid := strings.Repeat("0", 40)
	tree := append([]byte("100644 audit.pack\x00"), make([]byte, 20)...)
	nested := []byte(projectGitSchema + oid + " tree " + strconv.Itoa(len(tree)) + "\n")
	nested = append(nested, tree...)
	nested = append(nested, '\n')
	t.Setenv("PATH", t.TempDir())
	if _, _, e := contributionObjectPreflight(ctx, nested, p.Limits); e == nil || !strings.Contains(e.Error(), "compressed audit") {
		t.Fatal("audit material reached expensive decoder", e)
	}
}

func exchangeEnvelope(t *testing.T, p c.Policy, identity *Identity, operation c.Operation, content c.Content, intent string) c.Envelope {
	t.Helper()
	key, e := identity.privateKey()
	if e != nil {
		t.Fatal(e)
	}
	env, e := c.Sign(c.Record{Version: c.Version, Kind: c.SubmissionKind, Project: p.Project, Actor: identity.Actor, PublicKey: identity.PublicKey, Intent: intent, Submission: &c.Submission{Operation: operation, Content: content}}, key)
	if e != nil {
		t.Fatal(e)
	}
	return env
}
func TestContributionExchangeDiscussionProposalAndTypedNativeReferences(t *testing.T) {
	repo := inReplicationTestRepository(t)
	identity := testIdentity(t, "Native author")
	p := exchangePolicy()
	ctx := context.Background()
	base, e := gitText("rev-parse", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "untrusted code never executed")
	head, e := gitText("rev-parse", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	proposal := appendIdentityTestEvent(t, identity, "proposal.open", func(e *Event) { e.Title = "Proposed code"; e.Base = base; e.Head = head })
	if e = createProposalRef(proposal.ID, head); e != nil {
		t.Fatal(e)
	}
	discussion := appendIdentityTestEvent(t, identity, "discussion.open", func(e *Event) {
		e.Intent = "open"
		e.Body = string(resourceState(t, ResourceDiscussion, DiscussionState{Schema: "hn.discussion/1", Title: "Question", Category: "help", Status: "open"}))
	})
	reply := appendIdentityTestEvent(t, identity, "discussion-reply.open", func(e *Event) {
		e.Intent = "open"
		e.Body = string(resourceState(t, ResourceDiscussionReply, DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: discussion.ID, Body: "Exact answer"}))
	})
	answer := appendIdentityTestEvent(t, identity, "discussion.revise", func(e *Event) {
		e.Intent = "revise"
		e.Subject = discussion.ID
		e.Parents = []string{discussion.ID}
		e.Body = string(resourceState(t, ResourceDiscussion, DiscussionState{Schema: "hn.discussion/1", Title: "Question", Category: "help", Status: "open", AcceptedReplyRevision: reply.ID}))
	})
	issue := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "Issue state" })
	comment := appendIdentityTestEvent(t, identity, "issue.comment", func(e *Event) { e.Subject = issue.ID; e.Body = "Independent comment root" })
	content := func(id string) c.Content {
		return c.Content{Root: id, Revision: id, Author: identity.Actor, Parents: []string{}, References: []c.Reference{}}
	}
	replyContent := content(reply.ID)
	replyContent.References = []c.Reference{{Kind: c.RootReference, ID: discussion.ID}}
	envs := []c.Envelope{exchangeEnvelope(t, p, identity, c.ProposalCreate, content(proposal.ID), "proposal"), exchangeEnvelope(t, p, identity, c.DiscussionReply, replyContent, "reply")}
	refs := []string{actorRef(identity.Actor), proposalRef(proposal.ID), "refs/heads/main"}
	path := filepath.Join(t.TempDir(), "native.hnc")
	digest, e := ExportContributionArtifact(ctx, filepath.Join(repo, ".git"), p.Project, refs, envs, path, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	x := exchangeOpen(t, p)
	if _, e = x.Receive(ctx, path, digest, "native", refs); e != nil {
		t.Fatal(e)
	}
	a, e := x.Inspect(ctx, "native")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	got, e := a.Native.Fact(reply.ID)
	if e != nil || got.Content.Root != reply.ID || len(got.Content.References) != 1 || got.Content.References[0] != (c.Reference{Kind: c.RootReference, ID: discussion.ID}) {
		t.Fatal("reply root reference classification", got, e)
	}
	got, e = a.Native.Fact(answer.ID)
	if e != nil || got.Content.Root != discussion.ID || len(got.Content.Parents) != 1 || got.Content.References[0] != (c.Reference{Kind: c.RevisionReference, ID: reply.ID}) {
		t.Fatal("answer exact pin classification", got, e)
	}
	got, e = a.Native.Fact(comment.ID)
	if e != nil || got.Content.Root != comment.ID || got.Operation != c.IssueReply || len(got.Content.Parents) != 0 || got.Content.References[0] != (c.Reference{Kind: c.RootReference, ID: issue.ID}) {
		t.Fatal("legacy comment consumed issue state", got, e)
	}
	code, e := projectGit(ctx, a.Quarantine.GitDir, nil, 256, "rev-parse", head+"^{commit}")
	if e != nil || strings.TrimSpace(string(code)) != head {
		t.Fatal("proposal code lost", e)
	}
	// A missing proposal selection must refuse, rather than obtain it from an
	// ambient source ref or reinterpret a proposal as independent event bytes.
	if _, e = ExportContributionArtifact(ctx, filepath.Join(repo, ".git"), p.Project, []string{actorRef(identity.Actor)}, envs, filepath.Join(t.TempDir(), "incomplete"), p.Limits); e == nil {
		t.Fatal("omitted proposal code selection accepted")
	}
}
func TestContributionExchangeManifestRefsCASAndExplicitBackup(t *testing.T) {
	repo := inReplicationTestRepository(t)
	identity := testIdentity(t, "Manifest author")
	fact := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "Retained" })
	p := exchangePolicy()
	refs := []string{actorRef(identity.Actor)}
	content := c.Content{Root: fact.ID, Revision: fact.ID, Author: identity.Actor, Parents: []string{}, References: []c.Reference{}}
	env := exchangeEnvelope(t, p, identity, c.IssueCreate, content, "one")
	ctx := context.Background()
	dir := filepath.Join(repo, ".git")
	actor := identity.Actor
	x := exchangeOpen(t, p)
	receive := func(intent string, envelope c.Envelope) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "artifact")
		digest, e := ExportContributionArtifact(ctx, dir, p.Project, refs, []c.Envelope{envelope}, path, p.Limits)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = x.Receive(ctx, path, digest, intent, refs); e != nil {
			t.Fatal(e)
		}
	}
	receive("first", env)
	head, e := x.RetainActor(ctx, dir, "first", actor, "absent")
	if e != nil {
		t.Fatal(e)
	}
	records, readHead, e := ReadContributionManifest(ctx, dir, p.Project, actor, p.Limits)
	if e != nil || head != readHead || len(records) != 1 || !bytes.Equal(records[0].Payload(), env.Payload) || !bytes.Equal(records[0].Signature(), env.Signature) {
		t.Fatalf("raw ref reconstruction %s %s %#v %v", head, readHead, records, e)
	}
	replay, e := x.RetainActor(ctx, dir, "first", actor, "absent")
	if e != nil || replay != head {
		t.Fatal("raw retention exact replay promoted twice", e)
	}
	receive("second", exchangeEnvelope(t, p, identity, c.IssueCreate, content, "two"))
	if _, e = x.RetainActor(ctx, dir, "second", actor, "absent"); e == nil {
		t.Fatal("stale absence CAS accepted")
	}
	x.fault = func(step string) error {
		if step == "manifest-ref-updated" {
			return errors.New("crash after raw ref CAS")
		}
		return nil
	}
	if _, e = x.RetainActor(ctx, dir, "second", actor, head); e == nil {
		t.Fatal("ref fault not reached")
	}
	x.fault = nil
	other, e := x.RetainActor(ctx, dir, "second", actor, head)
	if e != nil || other == head {
		t.Fatal("exact planned ref recovery failed", e)
	}
	if _, e = x.RetainActor(ctx, dir, "second", actor, "absent"); e == nil {
		t.Fatal("changed expected-old replay accepted")
	}
	path := filepath.Join(t.TempDir(), "explicit-backup.hnc")
	digest, e := ExportContributionRefs(ctx, dir, p.Project, refs, []string{actor}, path, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	observer := exchangeOpen(t, p)
	if _, e = observer.Receive(ctx, path, digest, "restore", refs); e != nil {
		t.Fatal(e)
	}
	a, e := observer.Inspect(ctx, "restore")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if len(a.Records) != 2 {
		t.Fatal("explicit outer backup lost manifest records")
	}
}

func TestContributionExchangeDeadlineKillsSubprocessAndPreservesBudget(t *testing.T) {
	path, raw, digest, p, refs := exchangeArtifact(t)
	p = exchangeSmall(p, len(raw)+1)
	p.Limits.Milliseconds = 25
	x := exchangeOpen(t, p)
	commandDir := t.TempDir()
	wrapper := filepath.Join(commandDir, "git")
	if e := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec /bin/sleep 10\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", commandDir)
	start := time.Now()
	if _, e := x.Receive(context.Background(), path, digest, "deadline", refs); e == nil || time.Since(start) > time.Second {
		t.Fatalf("subprocess deadline was ineffective: %v elapsed=%v", e, time.Since(start))
	}
	ledger, attempts, e := x.Accounting()
	if e != nil || ledger.RetainedUsed.Requests != 1 || attempts[0].Status != "failed" {
		t.Fatal("timeout budget disappeared", e)
	}
}

func TestContributionExchangeSignedIntentConflictsAndExactArtifactDedup(t *testing.T) {
	repo := inReplicationTestRepository(t)
	identity := testIdentity(t, "One real key")
	p := exchangePolicy()
	ctx := context.Background()
	x := exchangeOpen(t, p)
	refs := []string{actorRef(identity.Actor)}
	receive := func(label string) (ContributionIntake, error) {
		fact := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = label })
		env := exchangeEnvelope(t, p, identity, c.IssueCreate, c.Content{Root: fact.ID, Revision: fact.ID, Author: identity.Actor, Parents: []string{}, References: []c.Reference{}}, "same-signed-intent")
		path := filepath.Join(t.TempDir(), "input")
		digest, e := ExportContributionArtifact(ctx, filepath.Join(repo, ".git"), p.Project, refs, []c.Envelope{env}, path, p.Limits)
		if e != nil {
			t.Fatal(e)
		}
		return x.Receive(ctx, path, digest, label, refs)
	}
	first, e := receive("first")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = receive("changed"); e == nil || !strings.Contains(e.Error(), "conflicting signed") {
		t.Fatal("same signed intent selected arrival-order winner", e)
	}
	before, attempts, e := x.Accounting()
	if e != nil || len(attempts) != 2 || !strings.Contains(attempts[1].Failure, "conflicting") {
		t.Fatal("conflict not inspectable", e)
	}
	replay, e := x.Receive(ctx, "/source-must-not-open", first.Artifact, "another-delivery-operation", refs)
	if e == nil || !replay.Replayed || replay.Intent != "first" || replay.Status != "conflict" {
		t.Fatal("exact conflicted artifact lost its durable refusal", e)
	}
	after, _, e := x.Accounting()
	if e != nil || after.RetainedUsed != before.RetainedUsed {
		t.Fatal("duplicate artifact consumed a second reservation", e)
	}
	lower := p
	lower.Limits.WireBytes = 1
	lower.Limits.DecodedBytes = 1
	lower.Limits.Objects = 1
	lower.Limits.Events = 1
	lower.Limits.Depth = 1
	lower.Limits.Milliseconds = 1
	reopened, e := OpenContributionExchange(x.root, lower, "must-not-change-window")
	if e != nil {
		t.Fatal(e)
	}
	if got, e := reopened.Receive(ctx, "/absent", first.Artifact, "first", refs); e == nil || !got.Replayed || got.Status != "conflict" {
		t.Fatal("lower policy lost prior exact conflict receipt", e)
	}
}
func TestContributionExchangeForeignReceiptDoesNotSpendReceiverCapacity(t *testing.T) {
	repo, fact, env, p, refs := exchangeFixture(t)
	ctx := context.Background()
	foreign := testIdentity(t, "Foreign intake signer")
	key, e := foreign.privateKey()
	if e != nil {
		t.Fatal(e)
	}
	submission, e := c.Verify(env.Payload, env.Signature)
	if e != nil {
		t.Fatal(e)
	}
	receipt, e := c.Sign(c.Record{Version: c.Version, Kind: c.ReceiptKind, Project: p.Project, Actor: foreign.Actor, PublicKey: foreign.PublicKey, Intent: "foreign-receipt", Receipt: &c.Receipt{Submission: submission.ID(), Authority: c.AuthorityRef{Commit: strings.Repeat("1", 40), Digest: c.Digest([]byte("foreign policy"))}, Artifact: c.Digest([]byte("foreign artifact")), Accounting: c.Digest([]byte("foreign accounting")), Window: "foreign-window", Cost: c.Cost{WireBytes: 1, DecodedBytes: 1, Objects: 1, Events: 1, Depth: 1, Milliseconds: 1}, Automatic: true, Evaluated: []string{}}}, key)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "receipt.hnc")
	digest, e := ExportContributionArtifact(ctx, filepath.Join(repo, ".git"), p.Project, refs, []c.Envelope{env, receipt}, path, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	x := exchangeOpen(t, p)
	got, e := x.Receive(ctx, path, digest, "local", refs)
	if e != nil {
		t.Fatal(e)
	}
	ledger, _, e := x.Accounting()
	if e != nil || ledger.Window != "first" || got.Reserved != contributionReservation(p.Limits) || ledger.RetainedUsed.Bytes <= 8192 {
		t.Fatal("foreign receipt replaced receiver-local measurement", e)
	}
	a, e := x.Inspect(ctx, "local")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if len(a.Records) != 2 {
		t.Fatal("foreign receipt raw bytes lost")
	}
	native, e := a.Native.Fact(fact.ID)
	if e != nil || native.Actor != fact.Event.Actor {
		t.Fatal(e)
	}
}
func TestContributionExchangeUnsupportedNativePredecessorPreservesFullChain(t *testing.T) {
	repo, fact, env, p, refs := exchangeFixture(t)
	ctx := context.Background()
	author := testIdentity(t, "CI ancestor author")
	head, e := gitText("rev-parse", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	appendIdentityTestEvent(t, author, "issue.open", func(e *Event) { e.Title = "Before unsupported CI" })
	appendIdentityTestEvent(t, author, "run.request", func(e *Event) {
		e.Subject = fact.ID
		e.Pipeline = "test"
		e.Definition = c.Digest([]byte("pipeline"))
		e.Commit = head
	})
	refs = append(refs, actorRef(author.Actor))
	before, _ := gitOutput("show-ref")
	path := filepath.Join(t.TempDir(), "must-not-publish")
	if _, e = ExportContributionArtifact(ctx, filepath.Join(repo, ".git"), p.Project, refs, []c.Envelope{env}, path, p.Limits); e == nil || !strings.Contains(e.Error(), "first public intake profile refuses native predecessor run.request") {
		t.Fatal("unsupported complete native chain was weakened/truncated", e)
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("unsupported profile published output")
	}
	after, _ := gitOutput("show-ref")
	if !bytes.Equal(before, after) {
		t.Fatal("unsupported history was rewritten to fit")
	}
}

func TestContributionExchangeRetainedDiskBoundLockAndPrivatePaths(t *testing.T) {
	path, raw, digest, p, refs := exchangeArtifact(t)
	p = exchangeSmall(p, len(raw))
	x := exchangeOpen(t, p)
	ctx := context.Background()
	if _, e := x.Receive(ctx, path, digest, "kept", refs); e != nil {
		t.Fatal(e)
	}
	unlock, e := x.lock()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = x.Receive(ctx, "/never-opened", digest, "concurrent", refs); e == nil {
		unlock()
		t.Fatal("concurrent operation ignored project lock")
	}
	unlock()
	ledger, attempts, e := x.Accounting()
	if e != nil {
		t.Fatal(e)
	}
	var retained int64
	entries, e := os.ReadDir(x.root)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		info, e := entry.Info()
		if e != nil {
			t.Fatal(e)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatal("unsafe or unexpected durable state", info.Name(), info.Mode())
		}
		retained += info.Size()
	}
	if retained > ledger.RetainedUsed.Bytes || len(attempts) != 1 || attempts[0].WireBytes != int64(len(raw)) {
		t.Fatalf("retained disk/read budget mismatch: retained=%d ledger=%#v attempts=%#v", retained, ledger, attempts)
	}
	linked := filepath.Join(t.TempDir(), "linked-private-state")
	if e = os.Symlink(x.root, linked); e != nil {
		t.Fatal(e)
	}
	if _, e = OpenContributionExchange(linked, p, "reset"); e == nil {
		t.Fatal("symlinked project ledger accepted")
	}
	rawJournal, e := os.ReadFile(filepath.Join(x.root, "journal.json"))
	if e != nil {
		t.Fatal(e)
	}
	damaged := bytes.Replace(rawJournal, []byte(`"requests":1`), []byte(`"requests":0`), 1)
	if bytes.Equal(damaged, rawJournal) {
		t.Fatal("corruption test did not mutate actual counters")
	}
	if e = os.WriteFile(filepath.Join(x.root, "journal.json"), damaged, 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e = x.Accounting(); e == nil {
		t.Fatal("corrupted counter granted fresh capacity")
	}
}

func TestContributionExchangePreJournalManifestRetryHasNoNewImmutableObjects(t *testing.T) {
	repo, _, env, p, refs := exchangeFixture(t)
	ctx := context.Background()
	dir := filepath.Join(repo, ".git")
	v, e := c.Verify(env.Payload, env.Signature)
	if e != nil {
		t.Fatal(e)
	}
	actor := v.Record().Actor
	path := filepath.Join(t.TempDir(), "artifact")
	digest, e := ExportContributionArtifact(ctx, dir, p.Project, refs, []c.Envelope{env}, path, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	x := exchangeOpen(t, p)
	if _, e = x.Receive(ctx, path, digest, "objects-before-plan", refs); e != nil {
		t.Fatal(e)
	}
	x.fault = func(step string) error {
		if step == "manifest-objects-durable" {
			return errors.New("journal intent not yet durable")
		}
		return nil
	}
	if _, e = x.RetainActor(ctx, dir, "objects-before-plan", actor, "absent"); e == nil {
		t.Fatal("fault not reached")
	}
	ref, e := contributionRef(p.Project, actor)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = gitOutput("show-ref", "--verify", ref); e == nil {
		t.Fatal("ref changed before durable plan")
	}
	before, e := gitOutput("count-objects", "-v")
	if e != nil {
		t.Fatal(e)
	}
	// Repeat the same fault: immutable blobs/tree/commit must remain exact. A
	// timestamp-based commit would create another retained object across a second.
	first, e := prepareContributionManifest(ctx, dir, p.Project, actor, "absent", []c.Envelope{env}, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(1100 * time.Millisecond) // Cross a Git commit timestamp tick.
	second, e := prepareContributionManifest(ctx, dir, p.Project, actor, "absent", []c.Envelope{env}, p.Limits)
	if e != nil || first != second {
		t.Fatal("raw retention commit is nondeterministic", e)
	}
	after, e := gitOutput("count-objects", "-v")
	if e != nil || !bytes.Equal(before, after) {
		t.Fatalf("retry accumulated immutable objects: before=%s after=%s err=%v", before, after, e)
	}
	x.fault = nil
	head, e := x.RetainActor(ctx, dir, "objects-before-plan", actor, "absent")
	if e != nil || head != first {
		t.Fatal("positive deterministic recovery failed", e)
	}
}

func TestContributionExchangeInterruptedConflictRemainsRawAndPortable(t *testing.T) {
	repo := inReplicationTestRepository(t)
	identity := testIdentity(t, "Conflicting actor")
	p := exchangePolicy()
	ctx := context.Background()
	dir := filepath.Join(repo, ".git")
	refs := []string{actorRef(identity.Actor)}
	x := exchangeOpen(t, p)
	artifact := func(label string) (string, string) {
		t.Helper()
		fact := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = label })
		env := exchangeEnvelope(t, p, identity, c.IssueCreate, c.Content{Root: fact.ID, Revision: fact.ID, Author: identity.Actor, Parents: []string{}, References: []c.Reference{}}, "immutable-conflicting-intent")
		path := filepath.Join(t.TempDir(), "artifact")
		digest, e := ExportContributionArtifact(ctx, dir, p.Project, refs, []c.Envelope{env}, path, p.Limits)
		if e != nil {
			t.Fatal(e)
		}
		return path, digest
	}
	first, digest1 := artifact("first exact payload")
	x.fault = func(step string) error {
		if step == "artifact-durable" {
			return errors.New("intake interrupted")
		}
		return nil
	}
	if got, e := x.Receive(ctx, first, digest1, "first", refs); e == nil || got.Status != "uncertain" {
		t.Fatal("interruption control failed", got, e)
	}
	x.fault = nil
	second, digest2 := artifact("changed exact payload")
	if got, e := x.Receive(ctx, second, digest2, "second", refs); e == nil || got.Status != "conflict" {
		t.Fatal("interrupted input was omitted from signed-intent conflict checks", got, e)
	}
	if got, e := x.Recover(ctx, "first"); e == nil || got.Status != "conflict" {
		t.Fatal("recovery turned a known conflict into eligible intake", got, e)
	}
	pending, e := x.Pending()
	if e != nil || len(pending) != 2 || pending[0].Status != "conflict" || pending[1].Status != "conflict" {
		t.Fatal("both raw conflict sides are not inspectable", pending, e)
	}
	before, _, e := x.Accounting()
	if e != nil {
		t.Fatal(e)
	}
	for _, intent := range []string{"first", "second"} {
		a, e := x.Inspect(ctx, intent)
		if e != nil {
			t.Fatal(e)
		}
		if a.Intake == nil || a.Intake.Status != "conflict" {
			t.Fatal("inspected conflict lost local provenance")
		}
		if len(a.Records) != 1 {
			t.Fatal("original conflict envelope lost")
		}
		a.Close()
	}
	// Raw manifests carry both immutable signatures for canonical projection to
	// diagnose. Raw retention is not an eligible admission/portable receipt.
	h1, e := x.RetainActor(ctx, dir, "first", identity.Actor, "absent")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = x.RetainActor(ctx, dir, "second", identity.Actor, h1); e != nil {
		t.Fatal(e)
	}
	records, _, e := ReadContributionManifest(ctx, dir, p.Project, identity.Actor, p.Limits)
	if e != nil || len(records) != 2 || records[0].Record().Intent != records[1].Record().Intent || records[0].ID() == records[1].ID() {
		t.Fatal("raw conflict graph was lost", e)
	}
	backup := filepath.Join(t.TempDir(), "conflict-backup")
	digest, e := ExportContributionRefs(ctx, dir, p.Project, refs, []string{identity.Actor}, backup, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	observer := exchangeOpen(t, p)
	if got, e := observer.Receive(ctx, backup, digest, "restore", refs); e == nil || got.Status != "conflict" {
		t.Fatal("backup selected an arrival-order winner", got, e)
	}
	a, e := observer.Inspect(ctx, "restore")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if len(a.Records) != 2 {
		t.Fatal("portable conflict evidence incomplete")
	}
	// A conflict on another actor/intent cannot poison unrelated pending input.
	other := testIdentity(t, "Unrelated author")
	fact := appendIdentityTestEvent(t, other, "issue.open", func(e *Event) { e.Title = "Unrelated valid pending" })
	env := exchangeEnvelope(t, p, other, c.IssueCreate, c.Content{Root: fact.ID, Revision: fact.ID, Author: other.Actor, Parents: []string{}, References: []c.Reference{}}, "different-intent")
	path := filepath.Join(t.TempDir(), "unrelated")
	selected := []string{actorRef(other.Actor)}
	digest, e = ExportContributionArtifact(ctx, dir, p.Project, selected, []c.Envelope{env}, path, p.Limits)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := observer.Receive(ctx, path, digest, "unrelated", selected); e != nil || got.Status != "pending" {
		t.Fatal("unrelated pending poisoned by another intent conflict", got, e)
	}
	after, _, e := x.Accounting()
	if e != nil || after.RetainedUsed != before.RetainedUsed {
		t.Fatal("raw conflict retention/recovery bypassed its existing charge", e)
	}
}
