package migration

import (
	"encoding/json"
	"strings"
	"testing"
)

func issueIntent() Intent {
	return Intent{ID: "issue", Action: "create", Kind: "issue", Sources: []Key{}, Payload: json.RawMessage(`{"schema":"hn.migration.issue-write/1","title":"Title","body":"","state":"open","type":"","labels":[],"assignees":[]}`), DependsOn: []string{}}
}
func commentIntent(parent string) Intent {
	return Intent{ID: "comment", Action: "create", Kind: "comment", Sources: []Key{}, Payload: json.RawMessage(`{"schema":"hn.migration.comment-write/1","parent":` + parent + `,"body":"Body"}`), DependsOn: []string{"issue"}}
}
func TestCanonicalWritePresenceAndClosedSchemas(t *testing.T) {
	for _, parent := range []string{`{"id":"47"}`, `{"receipt":{"intent_id":"issue","field":"remote_id"}}`} {
		if _, e := CanonicalWrite(commentIntent(parent), false); e != nil {
			t.Fatal(e)
		}
	}
	for _, parent := range []string{`{}`, `{"id":"","receipt":{"intent_id":"issue","field":"remote_id"}}`, `{"id":"x","receipt":{"intent_id":"issue","field":"remote_id"}}`, `{"receipt":{"intent_id":"other","field":"remote_id"}}`, `{"receipt":{"intent_id":"issue","field":"url"}}`, `{"receipt":{"intent_id":"issue","field":"remote_id","extra":true}}`, `{"ID":"x"}`, `{"id":null}`, `{"id":"x","id":"y"}`, `{"id":"\ud800"}`} {
		if _, e := CanonicalWrite(commentIntent(parent), false); e == nil {
			t.Fatal("accepted", parent)
		}
	}
	in := issueIntent()
	if _, e := CanonicalWrite(in, false); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{strings.Replace(string(in.Payload), `"title":"Title",`, "", 1), strings.Replace(string(in.Payload), `"title":"Title"`, `"title":"Title","target":{}`, 1), strings.Replace(string(in.Payload), `"labels":[]`, `"labels":null`, 1), strings.Replace(string(in.Payload), `"state":"open"`, `"State":"open"`, 1)} {
		bad := in
		bad.Payload = []byte(raw)
		if _, e := CanonicalWrite(bad, false); e == nil {
			t.Fatal("accepted", raw)
		}
	}
	in.Action = "update"
	if _, e := CanonicalWrite(in, false); e == nil {
		t.Fatal("update missing target")
	}
	in.Payload = []byte(strings.Replace(string(in.Payload), `"title":"Title"`, `"title":"Title","target":{"id":"47"}`, 1))
	if _, e := CanonicalWrite(in, true); e != nil {
		t.Fatal(e)
	}
	// Keep all required source fields strict, including empty-valued type.
	if _, e := CanonicalContent("issue", []byte(`{"schema":"hn.migration.issue/1","title":"t","body":"","state":"open","labels":[],"assignees":[]}`)); e == nil {
		t.Fatal("source missing type accepted")
	}
}
func testPlan() Plan {
	return Plan{Schema: "hn.migration-plan/1", SnapshotDigest: Digest([]byte("snapshot")), Destination: Locator{Kind: "fixture", Repository: "project"}, Limits: DefaultLimits(), Decisions: []Decision{}, Intents: []Intent{issueIntent(), commentIntent(`{"receipt":{"intent_id":"issue","field":"remote_id"}}`)}, Findings: []Finding{}}
}
func TestPlanLossDecisionsAndDependencyBinding(t *testing.T) {
	p := testPlan()
	p.Findings = []Finding{{ID: "loss", Status: "unsupported", Message: "not retrieved"}}
	if _, e := CanonicalPlan(p); e == nil {
		t.Fatal("loss without decision")
	}
	p.Decisions = []Decision{{FindingID: "loss", Action: "acknowledge", Mapping: json.RawMessage(`{}`)}}
	raw, e := CanonicalPlan(p)
	if e != nil {
		t.Fatal(e)
	}
	pd := Digest(raw)
	r := Receipt{PlanDigest: pd, IntentID: "issue", RemoteID: "4711"}
	if _, e = BindWrite(pd, p.Intents[1], nil); e == nil {
		t.Fatal("unconfirmed parent")
	}
	b, e := BindWrite(pd, p.Intents[1], map[string]Receipt{"issue": r})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b.Payload), `"parent":{"id":"4711"}`) || strings.Contains(string(b.Payload), "receipt") {
		t.Fatal(string(b.Payload))
	}
	r.PlanDigest = Digest(nil)
	if _, e = BindWrite(pd, p.Intents[1], map[string]Receipt{"issue": r}); e == nil {
		t.Fatal("cross-plan receipt")
	}
	p.Intents[0].DependsOn = []string{"comment"}
	if _, e = CanonicalPlan(p); e == nil {
		t.Fatal("cycle accepted")
	}
}
