package migration

import (
	"encoding/json"
	"fmt"
)

type ProgressEntry struct {
	IntentID string     `json:"intent_id"`
	State    string     `json:"state"`
	Write    BoundWrite `json:"write"`
	Receipts []Receipt  `json:"receipts"`
}
type Progress struct {
	Schema     string          `json:"schema"`
	PlanDigest string          `json:"plan_digest"`
	Actor      string          `json:"actor"`
	Entries    []ProgressEntry `json:"entries"`
}

func CanonicalProgress(p Progress, plan Plan) ([]byte, error) {
	raw, e := CanonicalPlan(plan)
	if e != nil {
		return nil, e
	}
	if p.Schema != "hn.migration-progress/1" || p.PlanDigest != Digest(raw) || !text(p.Actor, 4096, false) || len(p.Entries) > len(plan.Intents) {
		return nil, fmt.Errorf("progress binding mismatch")
	}
	confirmed := map[string]Receipt{}
	for i, v := range p.Entries {
		in := plan.Intents[i]
		if v.IntentID != in.ID || v.Write.PlanDigest != p.PlanDigest || v.Write.IntentID != in.ID || v.Write.PayloadDigest != Digest(v.Write.Payload) {
			return nil, fmt.Errorf("bound progress mismatch")
		}
		expected, e := BindWrite(p.PlanDigest, in, confirmed)
		if e != nil || string(expected.Payload) != string(v.Write.Payload) || expected.PayloadDigest != v.Write.PayloadDigest {
			return nil, fmt.Errorf("persisted payload differs from confirmed dependencies")
		}
		switch v.State {
		case "prepared", "inflight":
			if len(v.Receipts) != 0 || i != len(p.Entries)-1 {
				return nil, fmt.Errorf("unconfirmed progress must be last")
			}
		case "confirmed":
			if len(v.Receipts) != 1 {
				return nil, fmt.Errorf("confirmed progress needs one receipt")
			}
			if e = ValidateReceipt(v.Receipts[0], plan, in, v.Write); e != nil {
				return nil, e
			}
			confirmed[in.ID] = v.Receipts[0]
		default:
			return nil, fmt.Errorf("invalid progress state")
		}
	}
	p.Entries = append([]ProgressEntry{}, p.Entries...)
	return json.Marshal(p)
}
