package contribution_test

import (
	c "hubnot/internal/contribution"
	"testing"
)

func TestOperationPermissionsRemainSeparate(t *testing.T) {
	a := authority()
	base := c.Request{Operation: c.IssueCreate, Actor: signer(1), Author: signer(1), Verified: true, DependenciesAdmitted: true, Receipt: true}
	got, err := c.Evaluate(a, base)
	if err != nil || !got.Allowed || !got.Automatic {
		t.Fatalf("unknown author denied explicit automatic rule: %#v %v", got, err)
	}
	for _, op := range []c.Operation{c.NativeReview, c.NativeExecute, c.NativeAccept, c.NativeMerge} {
		r := base
		r.Operation = op
		got, err = c.Evaluate(a, r)
		if err != nil || got.Allowed || !got.RequiresNativeAuthority {
			t.Fatalf("admission granted native authority for %s", op)
		}
	}
	for _, field := range []string{"signature", "dependencies", "receipt", "author"} {
		r := base
		switch field {
		case "signature":
			r.Verified = false
		case "dependencies":
			r.DependenciesAdmitted = false
		case "receipt":
			r.Receipt = false
		case "author":
			r.Operation = c.IssueEdit
			r.Author = signer(3)
		}
		got, err = c.Evaluate(a, r)
		if err != nil || got.Allowed {
			t.Fatalf("missing %s authorized: %#v %v", field, got, err)
		}
	}
	a.Policy = nil
	got, err = c.Evaluate(a, base)
	if err != nil || !got.Allowed || !got.Legacy {
		t.Fatal("absent optional configuration changed legacy behavior")
	}
	base.Operation = c.Moderate
	got, err = c.Evaluate(a, base)
	if err != nil || got.Allowed {
		t.Fatal("absent policy granted moderation")
	}
	base.Operation = c.Prepare
	got, err = c.Evaluate(c.Authority{}, base)
	if err != nil || !got.Allowed {
		t.Fatal("stranger cannot prepare locally")
	}
}
func TestReservationsAreProjectWideDurableAndBounded(t *testing.T) {
	p := policy()
	cost := c.Cost{WireBytes: 100, DecodedBytes: 100, Objects: 2, Events: 1, Depth: 1, Milliseconds: 1}
	p.Limits.Window = c.Usage{Bytes: 8592, Objects: 4, Events: 2, Requests: 2}
	p.Limits.Retained = p.Limits.Window
	l := c.Ledger{Project: p.Project, Window: "first"}
	next, replay, err := c.Reserve(l, *p, "one", digest("one"), cost)
	if err != nil || replay || next.RetainedUsed.Bytes != 4296 {
		t.Fatalf("reservation: %#v %v", next, err)
	}
	same, replay, err := c.Reserve(next, *p, "one", digest("one"), cost)
	if err != nil || !replay || same.WindowUsed != next.WindowUsed {
		t.Fatal("exact retry charged twice")
	}
	if _, _, err = c.Reserve(next, *p, "one", digest("changed"), cost); err == nil {
		t.Fatal("changed intent replay accepted")
	}
	full, _, err := c.Reserve(next, *p, "another-key", digest("two"), cost)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = c.Reserve(full, *p, "third-key", digest("three"), cost); err == nil {
		t.Fatal("many keys bypassed project budget")
	}
	rolled, err := c.Rollover(full, *p, "first", "second", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = c.Reserve(rolled, *p, "third", digest("three"), cost); err == nil {
		t.Fatal("window rollover made retained objects free")
	}
	if _, err = c.Rollover(full, *p, "wrong", "second", true); err == nil {
		t.Fatal("stale rollover accepted")
	}
	if _, err = c.Rollover(full, *p, "first", "second", false); err == nil {
		t.Fatal("unauthorized rollover accepted")
	}
}
func TestInvalidPoliciesAndCostRefuse(t *testing.T) {
	for _, mutate := range []func(*c.Policy){func(p *c.Policy) { p.Project = "main" }, func(p *c.Policy) { p.Moderators = nil }, func(p *c.Policy) { p.Rules = append(p.Rules, p.Rules[0]) }, func(p *c.Policy) { p.Limits.WireBytes = 0 }, func(p *c.Policy) { p.Rules[0].Operation = c.NativeMerge }, func(p *c.Policy) { p.Baseline = []string{digest("a"), digest("a")} }} {
		p := policy()
		mutate(p)
		if err := p.Validate(); err == nil {
			t.Fatalf("invalid policy accepted: %#v", p)
		}
	}
	p := policy()
	l := c.Ledger{Project: p.Project, Window: "first"}
	if _, _, err := c.Reserve(l, *p, "one", digest("one"), c.Cost{WireBytes: p.Limits.WireBytes + 1}); err == nil {
		t.Fatal("oversize reservation accepted")
	}
}

func TestLedgerRejectsCorruptionAndChargesMetadata(t *testing.T) {
	p := policy()
	cost := c.Cost{WireBytes: 100, DecodedBytes: 100, Objects: 2, Events: 1, Depth: 1, Milliseconds: 1}
	l := c.Ledger{Project: p.Project, Window: "first"}
	next, _, err := c.Reserve(l, *p, "one", digest("one"), cost)
	if err != nil {
		t.Fatal(err)
	}
	if next.RetainedUsed.Bytes < 200+4096 {
		t.Fatal("retained journal metadata was free")
	}
	next.RetainedUsed = c.Usage{}
	if _, _, err := c.Reserve(next, *p, "two", digest("two"), cost); err == nil {
		t.Fatal("corrupted accounting counters accepted")
	}
}

func TestExplicitMaintainerEditsAndManualActorAreBounded(t *testing.T) {
	a := authority()
	req := c.Request{Operation: c.IssueEdit, Actor: signer(2), Author: signer(1), Verified: true, DependenciesAdmitted: true, Receipt: true}
	got, err := c.Evaluate(a, req)
	if err != nil || got.Allowed {
		t.Fatal("maintainer implicitly received author edit")
	}
	a.Policy.Rules[2].AllowMaintainers = true
	got, err = c.Evaluate(a, req)
	if err != nil || !got.Allowed {
		t.Fatal("explicit maintainer edit refused")
	}
	req.Operation = c.ProposalCreate
	req.Actor = signer(1)
	req.Manual = true
	got, err = c.Evaluate(a, req)
	if err != nil || got.Allowed {
		t.Fatal("manual boolean supplied moderator authority")
	}
	req.Moderator = signer(2)
	got, err = c.Evaluate(a, req)
	if err != nil || !got.Allowed || got.Automatic {
		t.Fatal("explicit moderator manual admission refused")
	}
}
func TestWindowNamesCannotResetCapacityAndReplaySurvivesLowerLimits(t *testing.T) {
	p := policy()
	cost := c.Cost{WireBytes: 100, DecodedBytes: 100, Objects: 2, Events: 1, Depth: 1, Milliseconds: 1}
	l := c.Ledger{Project: p.Project, Window: "first"}
	if _, err := c.Rollover(l, *p, "first", "second", true); err == nil {
		t.Fatal("empty rollover could accumulate free metadata")
	}
	l, _, err := c.Reserve(l, *p, "one", digest("one"), cost)
	if err != nil {
		t.Fatal(err)
	}
	l, err = c.Rollover(l, *p, "first", "second", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Rollover(l, *p, "second", "first", true); err == nil {
		t.Fatal("old window name reused")
	}
	p.Limits.WireBytes = 1
	after, replay, err := c.Reserve(l, *p, "one", digest("one"), cost)
	if err != nil || !replay || after.RetainedUsed != l.RetainedUsed {
		t.Fatal("existing exact receipt was lost when limits lowered")
	}
	if _, _, err = c.Reserve(l, *p, "two", digest("two"), cost); err == nil {
		t.Fatal("new intake bypassed lower limits")
	}
}
