package contribution

import (
	"fmt"
	"math"
)

type Operation string

const (
	Prepare           Operation = "prepare"
	Transport         Operation = "transport"
	Moderate          Operation = "moderate"
	IssueCreate       Operation = "issue.create"
	IssueReply        Operation = "issue.reply"
	IssueEdit         Operation = "issue.edit"
	DiscussionCreate  Operation = "discussion.create"
	DiscussionReply   Operation = "discussion.reply"
	DiscussionEdit    Operation = "discussion.edit"
	ProposalCreate    Operation = "proposal.create"
	ProposalRevise    Operation = "proposal.revise"
	ProjectCreate     Operation = "project.create"
	ProjectEdit       Operation = "project.edit"
	WorkflowCreate    Operation = "workflow.create"
	WorkflowEdit      Operation = "workflow.edit"
	NativeReview      Operation = "native.review"
	NativeExecute     Operation = "native.execute"
	NativeAccept      Operation = "native.accept"
	NativeMerge       Operation = "native.merge"
	ProjectItemCreate Operation = "project-item.create"
	ProjectItemEdit   Operation = "project-item.edit"
)

func contentOperation(op Operation) bool {
	switch op {
	case IssueCreate, IssueReply, IssueEdit, DiscussionCreate, DiscussionReply, DiscussionEdit, ProposalCreate, ProposalRevise, ProjectCreate, ProjectEdit, ProjectItemCreate, ProjectItemEdit, WorkflowCreate, WorkflowEdit:
		return true
	}
	return false
}
func authorOperation(op Operation) bool {
	switch op {
	case IssueEdit, DiscussionEdit, ProposalRevise, ProjectEdit, ProjectItemEdit, WorkflowEdit:
		return true
	}
	return false
}

type Rule struct {
	Operation        Operation `json:"operation"`
	Automatic        bool      `json:"automatic"`
	Actors           []string  `json:"actors,omitempty"`
	AllowMaintainers bool      `json:"allowMaintainers,omitempty"`
}
type Policy struct {
	Project           string   `json:"project"`
	Moderators        []string `json:"moderators"`
	RevokedModerators []string `json:"revokedModerators,omitempty"`
	Rules             []Rule   `json:"rules"`
	Baseline          []string `json:"baseline"`
	Limits            Limits   `json:"limits"`
}

// Authority contains independently verified native policy and native maintainer
// roles. An imported envelope must never populate this trusted adapter result.
type Authority struct {
	Ref         AuthorityRef
	Policy      *Policy
	Maintainers []string
}
type Request struct {
	Operation            Operation
	Actor                string
	Author               string
	Verified             bool
	DependenciesAdmitted bool
	Receipt              bool
	Manual               bool
	Moderator            string
}
type Permission struct {
	Allowed                 bool
	Automatic               bool
	Legacy                  bool
	RequiresNativeAuthority bool
	Reason                  string
}

func listed(s string, ss []string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
func (p Policy) Validate() error {
	if !validID(p.Project) || len(p.Moderators) == 0 || !validActors(p.Moderators, MaxLinks) || !validActors(p.RevokedModerators, MaxLinks) || !validIDs(p.Baseline, MaxGraphRecords) || len(p.Rules) > 32 {
		return fmt.Errorf("invalid project policy")
	}
	seen := map[Operation]bool{}
	for _, r := range p.Rules {
		if !contentOperation(r.Operation) || seen[r.Operation] || !validActors(r.Actors, MaxLinks) {
			return fmt.Errorf("invalid or duplicate operation rule")
		}
		seen[r.Operation] = true
	}
	return p.Limits.Validate()
}
func (a Authority) validate() error {
	if a.Policy == nil {
		return fmt.Errorf("contributions disabled")
	}
	if err := a.Ref.validate(); err != nil {
		return err
	}
	if len(a.Maintainers) == 0 || !validActors(a.Maintainers, MaxLinks) {
		return ErrAuthority
	}
	return a.Policy.Validate()
}
func Evaluate(a Authority, r Request) (Permission, error) {
	deny := func(why string) (Permission, error) { return Permission{Reason: why}, nil }
	if !validActor(r.Actor) {
		return deny("invalid actor")
	}
	if r.Operation == Prepare {
		return Permission{Allowed: true, Reason: "local preparation grants no authority"}, nil
	}
	if r.Operation == Transport {
		if !r.Verified {
			return deny("native transport authentication required")
		}
		return Permission{Allowed: true}, nil
	}
	if !contentOperation(r.Operation) && r.Operation != Moderate && r.Operation != NativeReview && r.Operation != NativeExecute && r.Operation != NativeAccept && r.Operation != NativeMerge {
		return deny("unsupported operation")
	}
	if a.Policy == nil {
		if r.Operation == Moderate {
			return deny("contributions disabled")
		}
		return Permission{Allowed: true, Legacy: true, RequiresNativeAuthority: !contentOperation(r.Operation)}, nil
	}
	if err := a.validate(); err != nil {
		return Permission{}, err
	}
	if r.Operation == NativeReview || r.Operation == NativeExecute || r.Operation == NativeAccept || r.Operation == NativeMerge {
		return Permission{RequiresNativeAuthority: true, Reason: "existing native authority and evidence required"}, nil
	}
	if r.Operation == Moderate && listed(r.Actor, a.Policy.RevokedModerators) {
		return deny("actor explicitly revoked")
	}
	if r.Operation == Moderate {
		if !listed(r.Actor, a.Policy.Moderators) {
			return deny("current moderator required")
		}
		return Permission{Allowed: true}, nil
	}
	if !r.Verified || !r.DependenciesAdmitted || !r.Receipt {
		return deny("authenticated content, admitted dependencies and durable intake receipt required")
	}
	if r.Manual && (!listed(r.Moderator, a.Policy.Moderators) || listed(r.Moderator, a.Policy.RevokedModerators)) {
		return deny("evaluated moderator required for manual admission")
	}
	for _, rule := range a.Policy.Rules {
		if rule.Operation != r.Operation {
			continue
		}
		maintainer := rule.AllowMaintainers && listed(r.Actor, a.Maintainers)
		if len(rule.Actors) > 0 && !listed(r.Actor, rule.Actors) && !maintainer {
			return deny("operation actor not permitted")
		}
		if authorOperation(r.Operation) && r.Actor != r.Author && !maintainer {
			return deny("original author required")
		}
		if !rule.Automatic && !r.Manual {
			return Permission{Reason: "project moderator admission required"}, nil
		}
		return Permission{Allowed: true, Automatic: rule.Automatic && !r.Manual}, nil
	}
	return deny("no explicit operation rule")
}

// Cost is reserved before any input is opened. Bytes include all envelope,
// archive, predecessor and metadata material; reservation is conservative.
type Cost struct {
	WireBytes    int64 `json:"wireBytes"`
	DecodedBytes int64 `json:"decodedBytes"`
	Objects      int64 `json:"objects"`
	Events       int64 `json:"events"`
	Depth        int64 `json:"depth"`
	Milliseconds int64 `json:"milliseconds"`
}
type Usage struct {
	Bytes    int64 `json:"bytes"`
	Objects  int64 `json:"objects"`
	Events   int64 `json:"events"`
	Requests int64 `json:"requests"`
}
type Limits struct {
	WireBytes    int64 `json:"wireBytes"`
	DecodedBytes int64 `json:"decodedBytes"`
	Objects      int64 `json:"objects"`
	Events       int64 `json:"events"`
	Depth        int64 `json:"depth"`
	Milliseconds int64 `json:"milliseconds"`
	Window       Usage `json:"window"`
	Retained     Usage `json:"retained"`
}

func DefaultLimits() Limits {
	return Limits{WireBytes: 16 << 20, DecodedBytes: 32 << 20, Objects: 4096, Events: 2048, Depth: 256, Milliseconds: 30000, Window: Usage{Bytes: 256 << 20, Objects: 32768, Events: 16384, Requests: 256}, Retained: Usage{Bytes: 1 << 30, Objects: 131072, Events: 65536, Requests: 1024}}
}
func (u Usage) positive() bool { return u.Bytes > 0 && u.Objects > 0 && u.Events > 0 && u.Requests > 0 }
func (u Usage) nonnegative() bool {
	return u.Bytes >= 0 && u.Objects >= 0 && u.Events >= 0 && u.Requests >= 0
}
func (u Usage) within(max Usage) bool {
	return u.nonnegative() && u.Bytes <= max.Bytes && u.Objects <= max.Objects && u.Events <= max.Events && u.Requests <= max.Requests
}
func (c Cost) validate() error {
	if c.WireBytes <= 0 || c.DecodedBytes <= 0 || c.Objects <= 0 || c.Events <= 0 || c.Depth <= 0 || c.Milliseconds <= 0 || c.WireBytes > math.MaxInt64-c.DecodedBytes-LedgerEntryBytes {
		return fmt.Errorf("invalid finite request cost")
	}
	return nil
}
func (l Limits) Validate() error {
	if err := (Cost{l.WireBytes, l.DecodedBytes, l.Objects, l.Events, l.Depth, l.Milliseconds}).validate(); err != nil {
		return err
	}
	if !l.Window.positive() || !l.Retained.positive() {
		return fmt.Errorf("project budgets must be positive finite limits")
	}
	return nil
}
func (c Cost) within(l Limits) bool {
	return c.validate() == nil && c.WireBytes <= l.WireBytes && c.DecodedBytes <= l.DecodedBytes && c.Objects <= l.Objects && c.Events <= l.Events && c.Depth <= l.Depth && c.Milliseconds <= l.Milliseconds
}
func addUsage(a, b Usage) (Usage, error) {
	if !a.nonnegative() || !b.nonnegative() || a.Bytes > math.MaxInt64-b.Bytes || a.Objects > math.MaxInt64-b.Objects || a.Events > math.MaxInt64-b.Events || a.Requests > math.MaxInt64-b.Requests {
		return Usage{}, fmt.Errorf("accounting overflow")
	}
	return Usage{a.Bytes + b.Bytes, a.Objects + b.Objects, a.Events + b.Events, a.Requests + b.Requests}, nil
}

const LedgerEntryBytes int64 = 4096

type Reservation struct {
	Intent   string `json:"intent"`
	Artifact string `json:"artifact"`
	Cost     Cost   `json:"cost"`
	Window   string `json:"window"`
}
type Ledger struct {
	PriorWindows []string      `json:"priorWindows,omitempty"`
	Project      string        `json:"project"`
	Window       string        `json:"window"`
	WindowUsed   Usage         `json:"windowUsed"`
	RetainedUsed Usage         `json:"retainedUsed"`
	Reservations []Reservation `json:"reservations"`
}

// Reserve returns a pure replacement. Persist and lock it BEFORE opening input;
// failure/cancellation, admission, moderation and disable never undo this charge.
func Reserve(l Ledger, p Policy, intent, artifact string, cost Cost) (Ledger, bool, error) {
	fail := func(err error) (Ledger, bool, error) { return l, false, err }
	if err := p.Validate(); err != nil {
		return fail(err)
	}
	if l.Project != p.Project || !boundedText(l.Window, 128) || !boundedText(intent, 128) || !validID(artifact) || l.validate() != nil {
		return fail(fmt.Errorf("invalid reservation or project scope"))
	}
	// Bound replay metadata too: it is retained data, never a free unbounded map.
	if len(l.Reservations) > MaxGraphRecords {
		return fail(fmt.Errorf("retained reservation limit"))
	}
	for _, r := range l.Reservations {
		if r.Intent == intent {
			if r.Artifact != artifact || r.Cost != cost {
				return fail(fmt.Errorf("conflicting operation intent"))
			}
			return cloneLedger(l), true, nil
		}
	}
	if !cost.within(p.Limits) {
		return fail(fmt.Errorf("request budget exceeded"))
	}
	delta := Usage{cost.WireBytes + cost.DecodedBytes + LedgerEntryBytes, cost.Objects, cost.Events, 1}
	window, e := addUsage(l.WindowUsed, delta)
	if e != nil {
		return fail(e)
	}
	retained, e := addUsage(l.RetainedUsed, delta)
	if e != nil {
		return fail(e)
	}
	if !window.within(p.Limits.Window) || !retained.within(p.Limits.Retained) || len(l.Reservations) >= MaxGraphRecords {
		return fail(fmt.Errorf("project intake budget exhausted"))
	}
	next := cloneLedger(l)
	next.WindowUsed = window
	next.RetainedUsed = retained
	next.Reservations = append(next.Reservations, Reservation{intent, artifact, cost, l.Window})
	return next, false, nil
}
func cloneLedger(l Ledger) Ledger {
	l.Reservations = append([]Reservation(nil), l.Reservations...)
	l.PriorWindows = append([]string(nil), l.PriorWindows...)
	return l
}

// Rollover changes only the explicitly named window. Retention and replay history
// survive. The host must verify current native maintainer authority and persist CAS.
func Rollover(l Ledger, p Policy, expected, next string, authorized bool) (Ledger, error) {
	if err := p.Validate(); err != nil {
		return l, err
	}
	if !authorized || l.Project != p.Project || l.Window != expected || !boundedText(next, 128) || next == expected || listed(next, l.PriorWindows) || l.validate() != nil {
		return l, ErrAuthority
	}
	hasReservation := false
	for _, r := range l.Reservations {
		if r.Window == l.Window {
			hasReservation = true
			break
		}
	}
	if !hasReservation || len(l.PriorWindows) >= MaxGraphRecords {
		return l, fmt.Errorf("cannot roll over an empty window")
	}
	l = cloneLedger(l)
	l.PriorWindows = append(l.PriorWindows, l.Window)
	l.Window = next
	l.WindowUsed = Usage{}
	return l, nil
}

// validate detects truncated/reset counters rather than granting capacity after
// a corrupt journal. Retention is monotone; reclamation is deliberately absent.
func (l Ledger) validate() error {
	if !validID(l.Project) || !boundedText(l.Window, 128) || len(l.Reservations) > MaxGraphRecords || len(l.PriorWindows) > MaxGraphRecords || len(l.PriorWindows) > len(l.Reservations) {
		return fmt.Errorf("invalid ledger")
	}
	windows := map[string]bool{l.Window: true}
	for _, window := range l.PriorWindows {
		if !boundedText(window, 128) || windows[window] {
			return fmt.Errorf("invalid window history")
		}
		windows[window] = true
	}
	retained, window := Usage{}, Usage{}
	intents := map[string]bool{}
	for _, r := range l.Reservations {
		if !boundedText(r.Intent, 128) || !validID(r.Artifact) || r.Cost.validate() != nil || !windows[r.Window] || intents[r.Intent] {
			return fmt.Errorf("invalid retained reservation")
		}
		intents[r.Intent] = true
		delta := Usage{r.Cost.WireBytes + r.Cost.DecodedBytes + LedgerEntryBytes, r.Cost.Objects, r.Cost.Events, 1}
		var err error
		retained, err = addUsage(retained, delta)
		if err != nil {
			return err
		}
		if r.Window == l.Window {
			window, err = addUsage(window, delta)
			if err != nil {
				return err
			}
		}
	}
	if retained != l.RetainedUsed || window != l.WindowUsed {
		return fmt.Errorf("ledger counters differ from retained reservations")
	}
	return nil
}
