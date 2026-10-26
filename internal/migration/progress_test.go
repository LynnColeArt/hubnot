package migration

import "testing"

func TestProgressRequiresExactPreparedBytesAndConfirmedPredecessors(t *testing.T) {
	p := testPlan()
	raw, _ := CanonicalPlan(p)
	pd := Digest(raw)
	w, e := BindWrite(pd, p.Intents[0], nil)
	if e != nil {
		t.Fatal(e)
	}
	v := Progress{Schema: "hn.migration-progress/1", PlanDigest: pd, Actor: "actor", Entries: []ProgressEntry{{IntentID: "issue", State: "inflight", Write: w, Receipts: []Receipt{}}}}
	if _, e = CanonicalProgress(v, p); e != nil {
		t.Fatal(e)
	}
	v.Entries[0].Write.Payload = []byte(`{}`)
	v.Entries[0].Write.PayloadDigest = Digest([]byte(`{}`))
	if _, e = CanonicalProgress(v, p); e == nil {
		t.Fatal("rewritten payload accepted")
	}
	v.Entries[0].Write = w
	v.Entries[0].State = "confirmed"
	if _, e = CanonicalProgress(v, p); e == nil {
		t.Fatal("confirmed without receipt")
	}
	r := Receipt{PlanDigest: pd, IntentID: "issue", Kind: "issue", Action: "create", Destination: p.Destination, RemoteID: "47", Revision: "rev1", PayloadDigest: w.PayloadDigest}
	v.Entries[0].Receipts = []Receipt{r}
	cw, e := BindWrite(pd, p.Intents[1], map[string]Receipt{"issue": r})
	if e != nil {
		t.Fatal(e)
	}
	v.Entries = append(v.Entries, ProgressEntry{IntentID: "comment", State: "prepared", Write: cw, Receipts: []Receipt{}})
	if _, e = CanonicalProgress(v, p); e != nil {
		t.Fatal(e)
	}
	v.Entries[0].Receipts[0].RemoteID = "48"
	if _, e = CanonicalProgress(v, p); e == nil {
		t.Fatal("comment binding changed on resume")
	}
}
func TestReconciliationRequiresExactPositiveClosedReceipt(t *testing.T) {
	p := testPlan()
	raw, _ := CanonicalPlan(p)
	b, _ := BindWrite(Digest(raw), p.Intents[0], nil)
	r := Receipt{PlanDigest: Digest(raw), IntentID: "issue", Kind: "issue", Action: "create", Destination: p.Destination, RemoteID: "47", Revision: "r1", PayloadDigest: b.PayloadDigest}
	v := Reconciliation{Status: "exact", Receipt: &r, Reason: "positive"}
	encoded, e := CanonicalReconciliation(v, p, p.Intents[0], b)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = DecodeReconciliation(encoded, p, p.Intents[0], b); e != nil {
		t.Fatal(e)
	}
	r.PayloadDigest = Digest(nil)
	if _, e = CanonicalReconciliation(v, p, p.Intents[0], b); e == nil {
		t.Fatal("wrong payload receipt accepted")
	}
	for _, raw := range []string{`{"status":"exact","reason":""}`, `{"status":"no_proof","reason":"","receipt":null}`, `{"status":"no_proof","reason":"","extra":true}`} {
		if _, e = DecodeReconciliation([]byte(raw), p, p.Intents[0], b); e == nil {
			t.Fatal("invalid reconciliation", raw)
		}
	}
}
