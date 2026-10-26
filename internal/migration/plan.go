package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

type Decision struct {
	FindingID string          `json:"finding_id"`
	Action    string          `json:"action"`
	Mapping   json.RawMessage `json:"mapping"`
}
type Intent struct {
	ID               string          `json:"id"`
	Action           string          `json:"action"`
	Kind             string          `json:"kind"`
	Sources          []Key           `json:"sources"`
	Payload          json.RawMessage `json:"payload"`
	ExpectedID       string          `json:"expected_id"`
	ExpectedRevision string          `json:"expected_revision"`
	DependsOn        []string        `json:"depends_on"`
}
type ReceiptRef struct {
	IntentID string `json:"intent_id"`
	Field    string `json:"field"`
}
type RemoteRef struct {
	ID      string      `json:"id,omitempty"`
	Receipt *ReceiptRef `json:"receipt,omitempty"`
}
type IssueWrite struct {
	Schema    string     `json:"schema"`
	Target    *RemoteRef `json:"target,omitempty"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	Type      string     `json:"type"`
	Labels    []string   `json:"labels"`
	Assignees []string   `json:"assignees"`
}
type CommentWrite struct {
	Schema string     `json:"schema"`
	Parent RemoteRef  `json:"parent"`
	Target *RemoteRef `json:"target,omitempty"`
	Body   string     `json:"body"`
}
type BoundWrite struct {
	PlanDigest    string          `json:"plan_digest"`
	IntentID      string          `json:"intent_id"`
	PayloadDigest string          `json:"payload_digest"`
	Payload       json.RawMessage `json:"payload"`
}
type Receipt struct {
	PlanDigest    string  `json:"plan_digest"`
	IntentID      string  `json:"intent_id"`
	Kind          string  `json:"kind"`
	Action        string  `json:"action"`
	Destination   Locator `json:"destination"`
	RemoteID      string  `json:"remote_id"`
	RemoteKey     string  `json:"remote_key"`
	Revision      string  `json:"revision"`
	PayloadDigest string  `json:"payload_digest"`
}
type Reconciliation struct {
	Status  string   `json:"status"`
	Receipt *Receipt `json:"receipt,omitempty"`
	Reason  string   `json:"reason"`
}
type Plan struct {
	Schema         string     `json:"schema"`
	SnapshotDigest string     `json:"snapshot_digest"`
	Destination    Locator    `json:"destination"`
	Limits         Limits     `json:"limits"`
	Decisions      []Decision `json:"decisions"`
	Intents        []Intent   `json:"intents"`
	Findings       []Finding  `json:"findings"`
}
type ApplyContext interface {
	OpenBlob(context.Context, string) (io.ReadCloser, Blob, error)
	Dependency(string) (Receipt, bool)
	BoundWrite() BoundWrite
}
type Destination interface {
	Capabilities(context.Context) ([]Capability, error)
	Plan(context.Context, Snapshot, Selection, Limits) ([]Intent, []Finding, error)
	Apply(context.Context, Intent, ApplyContext) (Receipt, error)
	Reconcile(context.Context, Intent, ApplyContext) (Reconciliation, error)
}

// ProjectWrite is the closed local/archive preservation intent. Refs are explicit
// destination observations, never imported local signing streams or authority.
type RefUpdate struct {
	Ref         string `json:"ref"`
	OID         string `json:"oid"`
	ExpectedOld string `json:"expected_old"`
}
type ProjectWrite struct {
	Schema         string      `json:"schema"`
	SnapshotDigest string      `json:"snapshot_digest"`
	Refs           []RefUpdate `json:"refs"`
	Governance     string      `json:"governance"`
	Baseline       string      `json:"baseline"`
}

// GitWrite is shared transport for selected ordinary code refs. Remote is an
// explicit absolute local path or credential-free HTTPS URL, never a helper.
type GitWrite struct {
	Schema         string      `json:"schema"`
	SnapshotDigest string      `json:"snapshot_digest"`
	Remote         string      `json:"remote"`
	Refs           []RefUpdate `json:"refs"`
}

func ValidGitRemote(remote string) bool {
	if !text(remote, 4096, true) || strings.ContainsAny(remote, "\r\n") {
		return false
	}
	if filepath.IsAbs(remote) {
		return !strings.Contains(remote, "\x00")
	}
	return strings.HasPrefix(remote, "https://") && validURL(remote)
}
func closed(raw []byte, required, optional []string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if e := StrictJSON(raw, &m); e != nil {
		return nil, e
	}
	if m == nil {
		return nil, fmt.Errorf("expected object")
	}
	allow := map[string]bool{}
	for _, k := range append(append([]string{}, required...), optional...) {
		allow[k] = true
	}
	for k := range m {
		if !allow[k] {
			return nil, fmt.Errorf("unknown field %q", k)
		}
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("missing field %q", k)
		}
	}
	return m, nil
}
func validateRef(raw []byte, deps []string, resolved bool) error {
	m, e := closed(raw, nil, []string{"id", "receipt"})
	if e != nil {
		return e
	}
	if len(m) != 1 {
		return fmt.Errorf("remote reference needs exactly one present branch")
	}
	if b, ok := m["id"]; ok {
		var id string
		if e = json.Unmarshal(b, &id); e != nil || !text(id, 4096, true) {
			return fmt.Errorf("invalid remote ID")
		}
		return nil
	}
	if resolved {
		return fmt.Errorf("unresolved receipt reference")
	}
	var r ReceiptRef
	if e = StrictJSON(m["receipt"], &r); e != nil {
		return e
	}
	if !contains(deps, r.IntentID) || (r.Field != "remote_id" && r.Field != "remote_key" && r.Field != "revision") {
		return fmt.Errorf("undeclared receipt reference")
	}
	return nil
}
func contains(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}

// CanonicalWrite is the sole presence-aware closed write validator. The shared
// StrictJSON map pass supplies duplicate/null/Unicode/depth protection.
func CanonicalWrite(in Intent, resolved bool) (json.RawMessage, error) {
	if in.Action != "create" && in.Action != "update" && in.Action != "preserve" {
		return nil, fmt.Errorf("invalid intent action")
	}
	switch in.Kind {
	case "issue", "work_item":
		m, e := closed(in.Payload, []string{"schema", "title", "body", "state", "type", "labels", "assignees"}, []string{"target"})
		if e != nil {
			return nil, e
		}
		if e = writeTarget(m, in, resolved); e != nil {
			return nil, e
		}
		var w IssueWrite
		if e = json.Unmarshal(in.Payload, &w); e != nil {
			return nil, e
		}
		if w.Schema != "hn.migration.issue-write/1" {
			return nil, fmt.Errorf("invalid issue write schema")
		}
		c := IssueContent{"hn.migration.issue/1", w.Title, w.Body, w.State, w.Type, w.Labels, w.Assignees}
		b, _ := json.Marshal(c)
		if _, e = CanonicalContent(in.Kind, b); e != nil {
			return nil, e
		}
		return json.Marshal(w)
	case "comment":
		m, e := closed(in.Payload, []string{"schema", "parent", "body"}, []string{"target"})
		if e != nil {
			return nil, e
		}
		if e = writeTarget(m, in, resolved); e != nil {
			return nil, e
		}
		if e = validateRef(m["parent"], in.DependsOn, resolved); e != nil {
			return nil, e
		}
		var w CommentWrite
		if e = json.Unmarshal(in.Payload, &w); e != nil {
			return nil, e
		}
		if w.Schema != "hn.migration.comment-write/1" || !text(w.Body, 65536, true) {
			return nil, fmt.Errorf("invalid comment write")
		}
		return json.Marshal(w)
	case "git_refs":
		var w GitWrite
		if e := StrictJSON(in.Payload, &w); e != nil {
			return nil, e
		}
		if in.Action != "preserve" || w.Schema != "hn.migration.git-write/1" || !ValidDigest(w.SnapshotDigest) || !ValidGitRemote(w.Remote) || len(w.Refs) == 0 {
			return nil, fmt.Errorf("invalid Git transport intent")
		}
		seen := map[string]bool{}
		for _, r := range w.Refs {
			if !ValidRef(r.Ref) || (!strings.HasPrefix(r.Ref, "refs/heads/") && !strings.HasPrefix(r.Ref, "refs/tags/")) || !ValidOID(r.OID) || (r.ExpectedOld != "" && !ValidOID(r.ExpectedOld)) || seen[r.Ref] {
				return nil, fmt.Errorf("invalid Git ref transport guard")
			}
			seen[r.Ref] = true
		}
		return json.Marshal(w)
	case "project":
		var w ProjectWrite
		if e := StrictJSON(in.Payload, &w); e != nil {
			return nil, e
		}
		if in.Action != "preserve" || w.Schema != "hn.migration.project-write/1" || !ValidDigest(w.SnapshotDigest) || w.Governance != "preserve" || !ValidDigest(w.Baseline) {
			return nil, fmt.Errorf("invalid project preservation intent")
		}
		seen := map[string]bool{}
		for _, r := range w.Refs {
			if !ValidRef(r.Ref) || !ValidOID(r.OID) || (r.ExpectedOld != "" && !ValidOID(r.ExpectedOld)) || seen[r.Ref] {
				return nil, fmt.Errorf("invalid destination ref guard")
			}
			seen[r.Ref] = true
		}
		return json.Marshal(w)
	default:
		return nil, fmt.Errorf("unsupported intent kind %q", in.Kind)
	}
}
func writeTarget(m map[string]json.RawMessage, in Intent, resolved bool) error {
	r, ok := m["target"]
	if in.Action == "create" {
		if ok {
			return fmt.Errorf("create target must be absent")
		}
		return nil
	}
	if in.Action != "update" || !ok {
		return fmt.Errorf("update target required")
	}
	return validateRef(r, in.DependsOn, resolved)
}

func CanonicalPlan(p Plan) ([]byte, error) {
	if p.Schema != "hn.migration-plan/1" || !ValidDigest(p.SnapshotDigest) || !text(p.Destination.Kind, 128, true) || !validURL(p.Destination.Endpoint) || !text(p.Destination.Repository, 4096, true) {
		return nil, fmt.Errorf("invalid plan binding")
	}
	for _, v := range []string{p.Destination.Organization, p.Destination.Project} {
		if !text(v, 1024, false) {
			return nil, fmt.Errorf("invalid destination selector")
		}
	}
	if e := p.Limits.Validate(); e != nil {
		return nil, e
	}
	if len(p.Intents) > p.Limits.Records+p.Limits.Refs+1 || len(p.Findings) > p.Limits.Records*8+p.Limits.Refs*8+32 {
		return nil, fmt.Errorf("plan count budget exceeded")
	}
	p.Decisions = append([]Decision{}, p.Decisions...)
	p.Intents = append([]Intent{}, p.Intents...)
	p.Findings = append([]Finding{}, p.Findings...)
	decisions := map[string]bool{}
	findings := map[string]bool{}
	for _, f := range p.Findings {
		if findings[f.ID] || !text(f.ID, 1024, true) || !text(f.Message, 65536, true) {
			return nil, fmt.Errorf("invalid plan finding")
		}
		findings[f.ID] = true
		switch f.Status {
		case "mapped", "preserved", "requires_mapping", "unsupported", "missing_dependency", "identity_conflict":
		default:
			return nil, fmt.Errorf("invalid finding status")
		}
	}
	for i, d := range p.Decisions {
		if !findings[d.FindingID] || decisions[d.FindingID] {
			return nil, fmt.Errorf("unknown/duplicate decision")
		}
		switch d.Action {
		case "acknowledge", "map", "preserve":
		default:
			return nil, fmt.Errorf("invalid decision action")
		}
		var m map[string]json.RawMessage
		if e := StrictJSON(d.Mapping, &m); e != nil {
			return nil, e
		}
		p.Decisions[i].Mapping, _ = json.Marshal(m)
		decisions[d.FindingID] = true
	}
	for _, f := range p.Findings {
		if f.Status != "mapped" && f.Status != "preserved" && !decisions[f.ID] {
			return nil, fmt.Errorf("explicit decision required for %s", f.ID)
		}
	}
	seen := map[string]bool{}
	for i, in := range p.Intents {
		if !text(in.ID, 1024, true) || seen[in.ID] || !text(in.ExpectedID, 4096, false) || !text(in.ExpectedRevision, 4096, false) {
			return nil, fmt.Errorf("invalid intent identity")
		}
		deps := map[string]bool{}
		for _, d := range in.DependsOn {
			if !seen[d] || deps[d] {
				return nil, fmt.Errorf("dependencies must be unique confirmed predecessors")
			}
			deps[d] = true
		}
		for _, k := range in.Sources {
			if !validKey(k) {
				return nil, fmt.Errorf("invalid source key")
			}
		}
		b, e := CanonicalWrite(in, false)
		if e != nil {
			return nil, e
		}
		in.Payload = b
		in.DependsOn = append([]string{}, in.DependsOn...)
		in.Sources = append([]Key{}, in.Sources...)
		p.Intents[i] = in
		seen[in.ID] = true
	}
	sort.Slice(p.Decisions, func(i, j int) bool { return p.Decisions[i].FindingID < p.Decisions[j].FindingID })
	b, e := json.Marshal(p)
	if int64(len(b)) > p.Limits.ManifestBytes {
		return nil, fmt.Errorf("plan byte budget exceeded")
	}
	return b, e
}
func NewPlan(ctx context.Context, s Snapshot, sel Selection, d Destination, l Limits, decisions []Decision) (Plan, error) {
	raw, e := CanonicalSnapshot(s, l)
	if e != nil {
		return Plan{}, e
	}
	_ = json.Unmarshal(raw, &s)
	ins, fs, e := d.Plan(ctx, s, sel, l)
	if e != nil {
		return Plan{}, e
	}
	p := Plan{"hn.migration-plan/1", Digest(raw), sel.Destination, l, decisions, ins, append(append([]Finding{}, s.Findings...), fs...)}
	b, e := CanonicalPlan(p)
	if e != nil {
		return Plan{}, e
	}
	_ = json.Unmarshal(b, &p)
	return p, nil
}

// BindWrite resolves references from confirmed immutable predecessor values only.
func BindWrite(planDigest string, in Intent, receipts map[string]Receipt) (BoundWrite, error) {
	b, e := CanonicalWrite(in, false)
	if e != nil {
		return BoundWrite{}, e
	}
	resolve := func(r *RemoteRef) error {
		if r == nil || r.Receipt == nil {
			return nil
		}
		v, ok := receipts[r.Receipt.IntentID]
		if !ok || !contains(in.DependsOn, r.Receipt.IntentID) || v.PlanDigest != planDigest {
			return fmt.Errorf("unconfirmed dependency")
		}
		switch r.Receipt.Field {
		case "remote_id":
			r.ID = v.RemoteID
		case "remote_key":
			r.ID = v.RemoteKey
		case "revision":
			r.ID = v.Revision
		}
		r.Receipt = nil
		if !text(r.ID, 4096, true) {
			return fmt.Errorf("empty receipt field")
		}
		return nil
	}
	switch in.Kind {
	case "issue", "work_item":
		var w IssueWrite
		_ = json.Unmarshal(b, &w)
		if e = resolve(w.Target); e != nil {
			return BoundWrite{}, e
		}
		b, _ = json.Marshal(w)
	case "comment":
		var w CommentWrite
		_ = json.Unmarshal(b, &w)
		if e = resolve(&w.Parent); e != nil {
			return BoundWrite{}, e
		}
		if e = resolve(w.Target); e != nil {
			return BoundWrite{}, e
		}
		b, _ = json.Marshal(w)
	}
	in.Payload = b
	b, e = CanonicalWrite(in, true)
	if e != nil {
		return BoundWrite{}, e
	}
	return BoundWrite{planDigest, in.ID, Digest(b), b}, nil
}
func ValidateReceipt(r Receipt, p Plan, in Intent, b BoundWrite) error {
	raw, e := CanonicalPlan(p)
	if e != nil {
		return e
	}
	if r.PlanDigest != Digest(raw) || r.IntentID != in.ID || r.Kind != in.Kind || r.Action != in.Action || r.Destination != p.Destination || r.PayloadDigest != b.PayloadDigest || !text(r.RemoteID, 4096, true) || !text(r.RemoteKey, 4096, false) || !text(r.Revision, 4096, true) {
		return fmt.Errorf("receipt does not prove exact bound write")
	}
	if in.ExpectedID != "" && r.RemoteID != in.ExpectedID {
		return fmt.Errorf("receipt identity mismatch")
	}
	return nil
}

// CanonicalReconciliation closes the optional receipt branch without changing
// StrictJSON's required-field semantics for snapshot/source records.
func CanonicalReconciliation(r Reconciliation, p Plan, in Intent, b BoundWrite) ([]byte, error) {
	if !text(r.Reason, 65536, false) {
		return nil, fmt.Errorf("invalid reconciliation reason")
	}
	switch r.Status {
	case "exact":
		if r.Receipt == nil {
			return nil, fmt.Errorf("exact reconciliation needs receipt")
		}
		if e := ValidateReceipt(*r.Receipt, p, in, b); e != nil {
			return nil, e
		}
	case "ambiguous", "no_proof":
		if r.Receipt != nil {
			return nil, fmt.Errorf("unconfirmed reconciliation cannot carry receipt")
		}
	default:
		return nil, fmt.Errorf("invalid reconciliation status")
	}
	return json.Marshal(r)
}
func DecodeReconciliation(raw []byte, p Plan, in Intent, b BoundWrite) (Reconciliation, error) {
	var r Reconciliation
	m, e := closed(raw, []string{"status", "reason"}, []string{"receipt"})
	if e != nil {
		return r, e
	}
	if v, ok := m["receipt"]; ok {
		var receipt Receipt
		if e = StrictJSON(v, &receipt); e != nil {
			return r, e
		}
	}
	if e = json.Unmarshal(raw, &r); e != nil {
		return r, e
	}
	_, e = CanonicalReconciliation(r, p, in, b)
	return r, e
}
