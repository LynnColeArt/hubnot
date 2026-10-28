package contribution_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	c "hubnot/internal/contribution"
)

func digest(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}
func key(s byte) ed25519.PrivateKey {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = s
	}
	return ed25519.NewKeyFromSeed(seed)
}
func signer(s byte) string { return c.Actor(key(s).Public().(ed25519.PublicKey)) }
func content(s string) c.Content {
	id := digest(s)
	return c.Content{Root: id, Revision: id, Author: signer(1), Parents: []string{}, References: []string{}}
}
func policy() *c.Policy {
	return &c.Policy{Project: digest("project"), Moderators: []string{signer(2)}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}, {Operation: c.IssueReply, Automatic: true}, {Operation: c.IssueEdit, Automatic: true}, {Operation: c.ProposalCreate}}, Baseline: []string{}, Limits: c.DefaultLimits()}
}
func authority() c.Authority {
	return c.Authority{Ref: c.AuthorityRef{Commit: "0123456789012345678901234567890123456789", Digest: digest("policy")}, Policy: policy(), Maintainers: []string{signer(2)}}
}
func submission(t *testing.T, intent string, body c.Content, op c.Operation) c.Verified {
	t.Helper()
	r := c.Record{Version: c.Version, Kind: c.SubmissionKind, Project: policy().Project, Actor: signer(1), PublicKey: c.PublicKey(key(1)), Intent: intent, Submission: &c.Submission{Operation: op, Content: body}}
	return signed(t, r, 1)
}
func signed(t *testing.T, r c.Record, k byte) c.Verified {
	t.Helper()
	e, err := c.Sign(r, key(k))
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.Verify(e.Payload, e.Signature)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

type native map[string]c.Fact

func (n native) Fact(id string) (c.Fact, error) {
	f, ok := n[id]
	if !ok {
		return c.Fact{}, c.ErrMissingDependency
	}
	return f, nil
}

type auths map[string]c.Authority

func (a auths) Established(id string, r c.AuthorityRef) (bool, error) { return false, nil }
func (a auths) Historical(r c.AuthorityRef) (c.Authority, error) {
	v, ok := a[r.Digest]
	if !ok || v.Ref != r {
		return c.Authority{}, c.ErrAuthority
	}
	return v, nil
}
func receipt(t *testing.T, s c.Verified, automatic bool) c.Verified {
	t.Helper()
	r := c.Record{Version: c.Version, Kind: c.ReceiptKind, Project: policy().Project, Actor: signer(2), PublicKey: c.PublicKey(key(2)), Intent: "receipt-" + s.ID()[7:15], Receipt: &c.Receipt{Submission: s.ID(), Authority: authority().Ref, Artifact: digest("file"), Accounting: digest("ledger"), Window: "first", Cost: c.Cost{WireBytes: 1000, DecodedBytes: 1000, Objects: 2, Events: 1, Depth: 1, Milliseconds: 1000}, Automatic: automatic, Evaluated: []string{}}}
	return signed(t, r, 2)
}
func decision(t *testing.T, action c.Action, target c.Target, parents []string, intent string) c.Verified {
	t.Helper()
	return signed(t, c.Record{Version: c.Version, Kind: c.DecisionKind, Project: policy().Project, Actor: signer(2), PublicKey: c.PublicKey(key(2)), Intent: intent, Decision: &c.Decision{Action: action, Target: target, Authority: authority().Ref, Parents: parents}}, 2)
}
func reduce(t *testing.T, records []c.Verified, facts native) c.View {
	t.Helper()
	a := authority()
	v, err := c.Reduce(a, records, facts, auths{a.Ref.Digest: a})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRawIsNotAdmissionAndReceiptIsExact(t *testing.T) {
	body := content("issue")
	s := submission(t, "new", body, c.IssueCreate)
	facts := native{body.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: body}}
	v := reduce(t, []c.Verified{s}, facts)
	if v.Admitted[body.Revision] {
		t.Fatal("raw signed request became admitted")
	}
	v = reduce(t, []c.Verified{s, receipt(t, s, true)}, facts)
	if !v.Admitted[body.Revision] || !v.Visible[body.Revision] {
		t.Fatalf("automatic receipt did not admit: %#v", v)
	}
	changed := facts[body.Revision]
	changed.Content.Root = digest("substitution")
	facts[body.Revision] = changed
	bad, err := c.Reduce(authority(), []c.Verified{s, receipt(t, s, true)}, facts, auths{authority().Ref.Digest: authority()})
	if err != nil || bad.Admitted[body.Revision] || len(bad.Invalid) == 0 {
		t.Fatal("native content substitution accepted")
	}
}
func TestConcurrentModerationAndExplicitJoin(t *testing.T) {
	b := content("issue")
	s := submission(t, "new", b, c.IssueCreate)
	r := receipt(t, s, true)
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}}
	target := c.Target{Kind: c.RevisionTarget, ID: b.Revision}
	hide := decision(t, c.Hide, target, nil, "hide")
	restore := decision(t, c.Restore, target, nil, "restore")
	records := []c.Verified{s, r, hide, restore}
	v := reduce(t, records, facts)
	if v.Visible[b.Revision] || len(v.Conflicts) != 1 {
		t.Fatalf("conflict silently selected winner: %#v", v)
	}
	parents := []string{hide.ID(), restore.ID()}
	c.SortIDs(parents)
	join := decision(t, c.Restore, target, parents, "join")
	v = reduce(t, append(records, join), facts)
	if !v.Visible[b.Revision] || len(v.Conflicts) != 0 {
		t.Fatalf("explicit resolution failed: %#v", v)
	}
	reversed := []c.Verified{join, restore, r, hide, s}
	w := reduce(t, reversed, facts)
	if !w.Visible[b.Revision] {
		t.Fatal("arrival order changed reduction")
	}
}
func TestAdmittedHeadsExcludeRawSuccessorsAndKeepHiddenFacts(t *testing.T) {
	b := content("root")
	edit := b
	edit.Revision = digest("edit")
	edit.Parents = []string{b.Revision}
	s := submission(t, "create", b, c.IssueCreate)
	e := submission(t, "edit", edit, c.IssueEdit)
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}, edit.Revision: {Actor: signer(1), Operation: c.IssueEdit, Content: edit}}
	v := reduce(t, []c.Verified{s, receipt(t, s, true), e}, facts)
	if len(v.Heads[b.Root]) != 1 || v.Heads[b.Root][0] != b.Revision {
		t.Fatal("raw successor replaced admitted head")
	}
	h := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, nil, "hide")
	v = reduce(t, []c.Verified{s, receipt(t, s, true), e, h}, facts)
	if !v.Admitted[b.Revision] || v.Visible[b.Revision] || v.Admitted[edit.Revision] {
		t.Fatal("hiding corrupted admission or admitted successor")
	}
}
func TestIntentConflictAndExactReplay(t *testing.T) {
	b := content("one")
	s := submission(t, "same", b, c.IssueCreate)
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}}
	r := receipt(t, s, true)
	v := reduce(t, []c.Verified{s, s, r, r}, facts)
	if !v.Visible[b.Revision] || len(v.Conflicts) != 0 {
		t.Fatal("exact replay was not idempotent")
	}
	b2 := content("two")
	s2 := submission(t, "same", b2, c.IssueCreate)
	facts[b2.Revision] = c.Fact{Actor: signer(1), Operation: c.IssueCreate, Content: b2}
	v = reduce(t, []c.Verified{s, r, s2, receipt(t, s2, true)}, facts)
	if v.Admitted[b.Revision] || v.Admitted[b2.Revision] || len(v.Conflicts) == 0 {
		t.Fatal("conflicting intent admitted")
	}
}

type establishedAuth struct {
	current     c.Authority
	historical  c.Authority
	established map[string]bool
}

func (a establishedAuth) Historical(r c.AuthorityRef) (c.Authority, error) {
	for _, v := range []c.Authority{a.current, a.historical} {
		if v.Ref == r {
			return v, nil
		}
	}
	return c.Authority{}, c.ErrAuthority
}
func (a establishedAuth) Established(id string, r c.AuthorityRef) (bool, error) {
	return a.established[id], nil
}
func TestOldPolicyNeedsExactEstablishedCheckpoint(t *testing.T) {
	b := content("issue")
	s := submission(t, "new", b, c.IssueCreate)
	r := receipt(t, s, true)
	h := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, nil, "hide")
	old := authority()
	now := authority()
	now.Ref.Digest = digest("new policy")
	now.Policy.Moderators = []string{signer(3)}
	now.Maintainers = []string{signer(3)}
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}}
	source := establishedAuth{now, old, map[string]bool{r.ID(): true}}
	v, err := c.Reduce(now, []c.Verified{s, r, h}, facts, source)
	if err != nil || !v.Visible[b.Revision] || len(v.Invalid) != 1 {
		t.Fatalf("new old-policy signature became effective: %#v %v", v, err)
	}
	source.established[h.ID()] = true
	v, err = c.Reduce(now, []c.Verified{s, r, h}, facts, source)
	if err != nil || v.Visible[b.Revision] || !v.Admitted[b.Revision] {
		t.Fatalf("legitimate historical decision lost: %#v %v", v, err)
	}
	now.Policy.RevokedModerators = []string{signer(2)}
	source.current = now
	v, err = c.Reduce(now, []c.Verified{s, r, h}, facts, source)
	if err != nil || len(v.Revoked) != 2 || v.Admitted[b.Revision] {
		t.Fatalf("explicit revocation ineffective: %#v %v", v, err)
	}
	now.Policy.RevokedModerators = nil
	source.current = now
	v, err = c.Reduce(now, []c.Verified{s, r, h}, facts, source)
	if err != nil || !v.Admitted[b.Revision] || v.Visible[b.Revision] {
		t.Fatal("reversible revocation rewrote history")
	}
}
func TestManualAdmissionNeedsIntakeAndPermission(t *testing.T) {
	b := content("proposal")
	s := submission(t, "proposal", b, c.ProposalCreate)
	d := decision(t, c.Admit, c.Target{Kind: c.SubmissionTarget, ID: s.ID()}, nil, "admit")
	facts := native{b.Revision: {Actor: signer(1), Operation: c.ProposalCreate, Content: b}}
	v := reduce(t, []c.Verified{s, d}, facts)
	if v.Admitted[b.Revision] || len(v.Invalid) == 0 {
		t.Fatal("manual admission bypassed intake")
	}
	v = reduce(t, []c.Verified{s, receipt(t, s, false), d}, facts)
	if !v.Visible[b.Revision] {
		t.Fatal("manual proposal admission failed")
	}
	v = reduce(t, []c.Verified{s, receipt(t, s, true)}, facts)
	if v.Admitted[b.Revision] || len(v.Invalid) == 0 {
		t.Fatal("manual rule silently became automatic")
	}
}
func TestHostileDecisionCannotSuppressUnrelatedContent(t *testing.T) {
	b := content("good")
	s := submission(t, "good", b, c.IssueCreate)
	r := receipt(t, s, true)
	bad := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, []string{digest("missing-parent")}, "bad")
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}}
	v := reduce(t, []c.Verified{s, r, bad}, facts)
	if !v.Visible[b.Revision] || len(v.Invalid) != 1 || v.Invalid[0].ID != bad.ID() {
		t.Fatalf("hostile graph suppressed legitimate state: %#v", v)
	}
}
func TestAuthorBlockAndHiddenReplyDependencyAreReversible(t *testing.T) {
	b := content("discussion")
	s := submission(t, "discussion", b, c.IssueCreate)
	reply := content("reply")
	reply.References = []string{b.Revision}
	rs := submission(t, "reply", reply, c.IssueReply)
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}, reply.Revision: {Actor: signer(1), Operation: c.IssueReply, Content: reply}}
	base := []c.Verified{s, receipt(t, s, true), rs, receipt(t, rs, true)}
	v := reduce(t, base, facts)
	if !v.Visible[reply.Revision] {
		t.Fatal("automatic reply refused")
	}
	h := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, nil, "hide")
	v = reduce(t, append(base, h), facts)
	if v.Visible[b.Revision] || v.Visible[reply.Revision] || !v.Admitted[reply.Revision] {
		t.Fatal("hidden dependency leaked reply or destroyed history")
	}
	restore := decision(t, c.Restore, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, []string{h.ID()}, "restore")
	v = reduce(t, append(append(base, h), restore), facts)
	if !v.Visible[reply.Revision] {
		t.Fatal("restoring dependency lost reply")
	}
	block := decision(t, c.Block, c.Target{Kind: c.ActorTarget, ID: signer(1)}, nil, "block")
	v = reduce(t, append(base, block), facts)
	if v.Visible[b.Revision] || v.Visible[reply.Revision] {
		t.Fatal("blocked author remained visible")
	}
	unblock := decision(t, c.Unblock, c.Target{Kind: c.ActorTarget, ID: signer(1)}, []string{block.ID()}, "unblock")
	v = reduce(t, append(append(base, block), unblock), facts)
	if !v.Visible[b.Revision] || !v.Visible[reply.Revision] {
		t.Fatal("unblock did not restore retained facts")
	}
}
func TestFiniteBaselineRequiresExactDependencyClosure(t *testing.T) {
	b := content("old")
	edit := b
	edit.Revision = digest("old-edit")
	edit.Parents = []string{b.Revision}
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}, edit.Revision: {Actor: signer(1), Operation: c.IssueEdit, Content: edit}}
	a := authority()
	a.Policy.Baseline = []string{edit.Revision}
	source := auths{a.Ref.Digest: a}
	if _, err := c.Reduce(a, nil, facts, source); err == nil {
		t.Fatal("baseline implicitly admitted unlisted raw dependency")
	}
	a.Policy.Baseline = []string{b.Revision, edit.Revision}
	c.SortIDs(a.Policy.Baseline)
	v, err := c.Reduce(a, nil, facts, source)
	if err != nil || !v.Visible[edit.Revision] || len(v.Heads[b.Root]) != 1 || v.Heads[b.Root][0] != edit.Revision {
		t.Fatal("finite exact baseline failed")
	}
	// A later arbitrary raw import is not retroactively part of the baseline.
	later := content("later")
	facts[later.Revision] = c.Fact{Actor: signer(1), Operation: c.IssueCreate, Content: later}
	v, err = c.Reduce(a, nil, facts, source)
	if err != nil || v.Visible[later.Revision] {
		t.Fatal("raw import became baseline")
	}
}
func TestEvaluatedBlockedIntakeCannotBecomeAdmissionAfterUnblock(t *testing.T) {
	b := content("issue")
	s := submission(t, "new", b, c.IssueCreate)
	block := decision(t, c.Block, c.Target{Kind: c.ActorTarget, ID: signer(1)}, nil, "block")
	unblock := decision(t, c.Unblock, c.Target{Kind: c.ActorTarget, ID: signer(1)}, []string{block.ID()}, "unblock")
	rr := receipt(t, s, true).Record()
	rr.Receipt.Evaluated = []string{block.ID()}
	r := signed(t, rr, 2)
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}}
	v := reduce(t, []c.Verified{s, r, block, unblock}, facts)
	if v.Admitted[b.Revision] || len(v.Invalid) == 0 {
		t.Fatal("receipt explicitly evaluated a block but admitted after unblock")
	}
}

func TestUnknownModeratorAndCrossTargetParentCannotAuthorize(t *testing.T) {
	b := content("issue")
	s := submission(t, "new", b, c.IssueCreate)
	r := receipt(t, s, true)
	bad := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, nil, "hide").Record()
	bad.Actor = signer(3)
	bad.PublicKey = c.PublicKey(key(3))
	forged := signed(t, bad, 3)
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}}
	v := reduce(t, []c.Verified{s, r, forged}, facts)
	if !v.Visible[b.Revision] || len(v.Invalid) != 1 {
		t.Fatal("valid unknown-key signature became moderation authority")
	}
	block := decision(t, c.Block, c.Target{Kind: c.ActorTarget, ID: signer(1)}, nil, "block")
	cross := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, []string{block.ID()}, "cross")
	v = reduce(t, []c.Verified{s, r, block, cross}, facts)
	if len(v.Invalid) != 1 || v.Invalid[0].ID != cross.ID() {
		t.Fatal("cross-target parent accepted")
	}
}
func TestDepthBudgetAndRejectedAdmissionAreObservable(t *testing.T) {
	b := content("root")
	s := submission(t, "new", b, c.IssueCreate)
	r := receipt(t, s, false)
	reject := decision(t, c.Reject, c.Target{Kind: c.SubmissionTarget, ID: s.ID()}, nil, "reject")
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}}
	v := reduce(t, []c.Verified{s, r, reject}, facts)
	if v.Visible[b.Revision] || len(v.Pending) != 1 {
		t.Fatal("rejected content visible or lost pending inspection")
	}
	a := authority()
	a.Policy.Limits.Depth = 1
	hide := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, nil, "hide")
	restore := decision(t, c.Restore, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, []string{hide.ID()}, "restore")
	hideAgain := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, []string{restore.ID()}, "again")
	// Explicit native model adapters remain bounded by configured graph depth.
	v, err := c.Reduce(a, []c.Verified{s, r, hide, restore, hideAgain}, facts, auths{a.Ref.Digest: a})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range v.Invalid {
		if d.ID == hideAgain.ID() {
			found = true
		}
	}
	if !found {
		t.Fatal("cached predecessor bypassed decision graph depth limit")
	}
}

func TestBlockedBaselineReferenceDoesNotRejectUnrelatedProjection(t *testing.T) {
	b := content("old")
	other := content("other")
	other.Author = signer(3)
	other.References = []string{b.Revision}
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}, other.Revision: {Actor: signer(3), Operation: c.IssueCreate, Content: other}}
	a := authority()
	a.Policy.Baseline = []string{b.Revision, other.Revision}
	c.SortIDs(a.Policy.Baseline)
	block := decision(t, c.Block, c.Target{Kind: c.ActorTarget, ID: signer(1)}, nil, "block")
	v, err := c.Reduce(a, []c.Verified{block}, facts, auths{a.Ref.Digest: a})
	if err != nil || v.Visible[b.Revision] || v.Visible[other.Revision] {
		t.Fatalf("moderation poisoned structurally valid baseline: %v %#v", err, v)
	}
}
func TestReceiptEvidenceRefusesHiddenParentButPreservesPriorEdit(t *testing.T) {
	b := content("root")
	edit := b
	edit.Revision = digest("edit")
	edit.Parents = []string{b.Revision}
	s := submission(t, "root", b, c.IssueCreate)
	e := submission(t, "edit", edit, c.IssueEdit)
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}, edit.Revision: {Actor: signer(1), Operation: c.IssueEdit, Content: edit}}
	base := []c.Verified{s, receipt(t, s, true)}
	v := reduce(t, base, facts)
	heads, err := c.IntakeEvidence(v, e)
	if err != nil || len(heads) != 0 {
		t.Fatal("visible parent could not authorize intake")
	}
	h := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, nil, "hide")
	v = reduce(t, append(base, h), facts)
	if _, err = c.IntakeEvidence(v, e); err == nil {
		t.Fatal("hidden parent authorized new edit")
	}
	// A genuine receipt made before hide remains admission of the exact edit.
	er := receipt(t, e, true)
	v = reduce(t, append(append(base, er, e), h), facts)
	if !v.Admitted[edit.Revision] || !v.Visible[edit.Revision] {
		t.Fatal("later hiding rewrote prior admitted edit")
	}
}

func TestRevocationCannotResurrectReceiptThatEvaluatedHiddenParent(t *testing.T) {
	b := content("root")
	edit := b
	edit.Revision = digest("edit")
	edit.Parents = []string{b.Revision}
	s := submission(t, "edit", edit, c.IssueEdit)
	old := authority()
	old.Policy.Moderators = []string{signer(2), signer(3)}
	c.SortIDs(old.Policy.Moderators)
	hide := decision(t, c.Hide, c.Target{Kind: c.RevisionTarget, ID: b.Revision}, nil, "hide")
	rr := receipt(t, s, true).Record()
	rr.Actor = signer(3)
	rr.PublicKey = c.PublicKey(key(3))
	rr.Receipt.Evaluated = []string{hide.ID()}
	r := signed(t, rr, 3)
	now := authority()
	now.Ref.Digest = digest("new")
	now.Policy.Baseline = []string{b.Revision}
	now.Policy.Moderators = []string{signer(3)}
	now.Policy.RevokedModerators = []string{signer(2)}
	now.Maintainers = []string{signer(3)}
	source := establishedAuth{now, old, map[string]bool{hide.ID(): true, r.ID(): true}}
	facts := native{b.Revision: {Actor: signer(1), Operation: c.IssueCreate, Content: b}, edit.Revision: {Actor: signer(1), Operation: c.IssueEdit, Content: edit}}
	v, err := c.Reduce(now, []c.Verified{s, r, hide}, facts, source)
	if err != nil || !v.Visible[b.Revision] || v.Admitted[edit.Revision] || len(v.Invalid) == 0 {
		t.Fatalf("current revocation resurrected forbidden historical intake: %v %#v", err, v)
	}
}
