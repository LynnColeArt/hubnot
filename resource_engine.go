package main

import (
	"container/heap"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
)

type ResourceHead struct {
	ID    string          `json:"id"`
	Actor string          `json:"actor"`
	State json.RawMessage `json:"state"`
}
type ResourceView struct {
	ID       string          `json:"id"`
	Family   string          `json:"family"`
	Creator  string          `json:"creator"`
	Heads    []ResourceHead  `json:"heads"`
	Conflict bool            `json:"conflict"`
	State    json.RawMessage `json:"state"`
	History  []StoredEvent   `json:"-"`
}
type ResourceCatalog struct {
	Resources map[string]*ResourceView
	IDs       []string
}
type ResourceMutation struct {
	Family    string
	Intent    string
	ID        string
	Expected  []string
	State     json.RawMessage
	Actor     string
	Operation string
}
type ResourceMutationResult struct {
	Admission  string `json:"admission,omitempty"`
	ResourceID string `json:"resource_id"`
	EventID    string `json:"event_id"`
	Replayed   bool   `json:"replayed"`
	Conflict   bool   `json:"conflict"`
	HeadCount  int    `json:"head_count"`
}
type ResourceOperationResult struct {
	Admission  string          `json:"admission,omitempty"`
	ResourceID string          `json:"resource_id"`
	EventID    string          `json:"event_id"`
	Actor      string          `json:"actor"`
	Operation  string          `json:"operation"`
	Kind       string          `json:"kind"`
	Intent     string          `json:"intent"`
	Parents    []string        `json:"parents"`
	Request    string          `json:"request"`
	State      json.RawMessage `json:"state"`
}

func resourceRootID(stored StoredEvent) string {
	if !isResourceKind(stored.Event.Kind) {
		return ""
	}
	if strings.HasSuffix(stored.Event.Kind, ".open") {
		return stored.ID
	}
	return stored.Event.Subject
}

// resourceReferences is shared by the accepted reader, quarantine and shallow
// recovery. Every exact native dependency must enter this closure.
func resourceReferences(e Event) []string {
	family := resourceFamily(e.Kind)
	if family == "" {
		return nil
	}
	refs := append([]string{}, e.Parents...)
	if e.Subject != "" {
		refs = append(refs, e.Subject)
	}
	switch family {
	case ResourceProjectItem:
		var s ProjectItemState
		if json.Unmarshal([]byte(e.Body), &s) == nil {
			refs = append(refs, s.ProjectID, s.ProjectRevision)
			if s.Target.ID != "" {
				refs = append(refs, s.Target.ID)
			}
		}
	case ResourceDiscussion:
		var s DiscussionState
		if json.Unmarshal([]byte(e.Body), &s) == nil && s.AcceptedReplyRevision != "" {
			refs = append(refs, s.AcceptedReplyRevision)
		}
	case ResourceDiscussionReply:
		var s DiscussionReplyState
		if json.Unmarshal([]byte(e.Body), &s) == nil {
			refs = append(refs, s.DiscussionID)
			if s.ReplyTo != "" {
				refs = append(refs, s.ReplyTo)
			}
		}
	}
	return sortedUniqueStrings(refs...)
}

// resourceDependencyMissing distinguishes recoverable exact-history omissions
// found while walking nested containers from present but invalid relationships.
type resourceDependencyMissing struct {
	ID     string
	Family string
}

func (e *resourceDependencyMissing) Error() string {
	return fmt.Sprintf("resource requires unavailable %s %s", e.Family, e.ID)
}

func validateResourceReferences(stored StoredEvent, byID map[string]StoredEvent) error {
	e := stored.Event
	family := resourceFamily(e.Kind)
	if family == "" {
		return nil
	}
	rootID := resourceRootID(stored)
	require := func(id, family string, root bool) (StoredEvent, error) {
		v, ok := byID[id]
		if !ok {
			return StoredEvent{}, &resourceDependencyMissing{ID: id, Family: family}
		}
		if resourceFamily(v.Event.Kind) != family || root && !strings.HasSuffix(v.Event.Kind, ".open") {
			return StoredEvent{}, fmt.Errorf("resource %s requires available %s %s", stored.ID, family, id)
		}
		return v, nil
	}
	if strings.HasSuffix(e.Kind, ".revise") {
		if _, err := require(e.Subject, family, true); err != nil {
			return err
		}
		for _, p := range e.Parents {
			v, err := require(p, family, false)
			if err != nil {
				return err
			}
			if resourceRootID(v) != e.Subject {
				return fmt.Errorf("resource revision parent belongs to another aggregate")
			}
		}
	}
	switch family {
	case ResourceProjectItem:
		var s ProjectItemState
		_ = json.Unmarshal([]byte(e.Body), &s)
		if _, err := require(s.ProjectID, ResourceProject, true); err != nil {
			return err
		}
		schema, err := require(s.ProjectRevision, ResourceProject, false)
		if err != nil {
			return err
		}
		if resourceRootID(schema) != s.ProjectID {
			return fmt.Errorf("item schema revision belongs to another project")
		}
		var project ProjectState
		_ = json.Unmarshal([]byte(schema.Event.Body), &project)
		fields := map[string]ProjectField{}
		for _, f := range project.Fields {
			fields[f.ID] = f
		}
		for id, v := range s.Values {
			f, ok := fields[id]
			if !ok || f.Type != v.Type {
				return fmt.Errorf("item field %q does not match pinned project schema", id)
			}
			if v.Type == "single_select" {
				found := false
				for _, o := range f.Options {
					if o.ID == v.OptionID {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("item option does not exist in pinned project schema")
				}
			}
		}
		if rootID != stored.ID {
			var original ProjectItemState
			_ = json.Unmarshal([]byte(byID[rootID].Event.Body), &original)
			if original.ProjectID != s.ProjectID {
				return fmt.Errorf("item revision cannot change project container")
			}
		}
		if s.Target.ID != "" {
			target, ok := byID[s.Target.ID]
			if !ok {
				return fmt.Errorf("item target is unavailable")
			}
			valid := s.Target.Kind == "issue" && target.Event.Kind == "issue.open" || s.Target.Kind == "proposal" && isProposalKind(target.Event.Kind) || s.Target.Kind == "discussion" && target.Event.Kind == "discussion.open"
			if !valid {
				return fmt.Errorf("item target has wrong native type")
			}
		}
	case ResourceDiscussion:
		var s DiscussionState
		_ = json.Unmarshal([]byte(e.Body), &s)
		if s.AcceptedReplyRevision != "" {
			reply, err := require(s.AcceptedReplyRevision, ResourceDiscussionReply, false)
			if err != nil {
				return err
			}
			var value DiscussionReplyState
			_ = json.Unmarshal([]byte(reply.Event.Body), &value)
			if value.DiscussionID != rootID {
				return fmt.Errorf("accepted answer belongs to another discussion")
			}
		}
	case ResourceDiscussionReply:
		var s DiscussionReplyState
		_ = json.Unmarshal([]byte(e.Body), &s)
		if _, err := require(s.DiscussionID, ResourceDiscussion, true); err != nil {
			return err
		}
		if rootID != stored.ID {
			var original DiscussionReplyState
			_ = json.Unmarshal([]byte(byID[rootID].Event.Body), &original)
			if original.DiscussionID != s.DiscussionID || original.ReplyTo != s.ReplyTo {
				return fmt.Errorf("reply revision cannot change thread or nesting parent")
			}
		}
		visited := map[string]bool{rootID: true}
		parent := s.ReplyTo
		depth := 1
		for parent != "" {
			if visited[parent] {
				return fmt.Errorf("reply nesting cycle")
			}
			visited[parent] = true
			depth++
			if depth > 8 {
				return fmt.Errorf("reply nesting exceeds 8")
			}
			v, err := require(parent, ResourceDiscussionReply, true)
			if err != nil {
				return err
			}
			var ancestor DiscussionReplyState
			_ = json.Unmarshal([]byte(v.Event.Body), &ancestor)
			if ancestor.DiscussionID != s.DiscussionID {
				return fmt.Errorf("reply nesting crosses discussion threads")
			}
			parent = ancestor.ReplyTo
		}
	}
	return nil
}
func validateResourceRelationships(events []StoredEvent) ([]StoredEvent, error) {
	byID := map[string]StoredEvent{}
	for _, s := range events {
		byID[s.ID] = s
	}
	degrees := map[string]int{}
	children := map[string][]string{}
	for _, s := range events {
		if !isResourceKind(s.Event.Kind) {
			continue
		}
		if err := validateResourceEvent(s.Event); err != nil {
			return nil, fmt.Errorf("resource fact %s: %w", s.ID, err)
		}
		if err := validateResourceReferences(s, byID); err != nil {
			return nil, err
		}
		if _, exists := degrees[s.ID]; exists {
			continue
		}
		degrees[s.ID] = len(s.Event.Parents)
		for _, p := range s.Event.Parents {
			children[p] = append(children[p], s.ID)
		}
	}
	ready := &issueIDHeap{}
	heap.Init(ready)
	for id, n := range degrees {
		if n == 0 {
			heap.Push(ready, id)
		}
	}
	ordered := []StoredEvent{}
	for ready.Len() > 0 {
		id := heap.Pop(ready).(string)
		ordered = append(ordered, byID[id])
		for _, child := range children[id] {
			degrees[child]--
			if degrees[child] == 0 {
				heap.Push(ready, child)
			}
		}
	}
	if len(ordered) != len(degrees) {
		return nil, fmt.Errorf("resource revision lineage contains a cycle")
	}
	return ordered, nil
}

// BuildResourceCatalog accepts already authenticated facts. Revisions consume
// explicit parents; concurrent maximal vertices remain visible without a winner.
func BuildResourceCatalog(events []StoredEvent) (*ResourceCatalog, error) {
	ordered, err := validateResourceRelationships(events)
	if err != nil {
		return nil, err
	}
	cat := &ResourceCatalog{Resources: map[string]*ResourceView{}, IDs: []string{}}
	consumed := map[string]bool{}
	for _, s := range ordered {
		for _, p := range s.Event.Parents {
			consumed[p] = true
		}
	}
	for _, s := range ordered {
		id := resourceRootID(s)
		if strings.HasSuffix(s.Event.Kind, ".open") {
			cat.Resources[id] = &ResourceView{ID: id, Family: resourceFamily(s.Event.Kind), Creator: s.Event.Actor, Heads: []ResourceHead{}, History: []StoredEvent{}}
			cat.IDs = append(cat.IDs, id)
		}
		view := cat.Resources[id]
		if view == nil {
			return nil, fmt.Errorf("resource root is unavailable")
		}
		view.History = append(view.History, s)
		if !consumed[s.ID] {
			canonical, err := CanonicalResourceState(view.Family, []byte(s.Event.Body))
			if err != nil {
				return nil, err
			}
			view.Heads = append(view.Heads, ResourceHead{s.ID, s.Event.Actor, canonical})
		}
	}
	sort.Strings(cat.IDs)
	for _, view := range cat.Resources {
		sort.Slice(view.Heads, func(i, j int) bool { return view.Heads[i].ID < view.Heads[j].ID })
		view.Conflict = len(view.Heads) > 1
		if len(view.Heads) == 1 {
			view.State = append(json.RawMessage{}, view.Heads[0].State...)
		}
	}
	return cat, nil
}
func findResourceOperation(events []StoredEvent, actor, key string) (*StoredEvent, error) {
	var prior *StoredEvent
	for i := range events {
		s := &events[i]
		if s.Event.Actor != actor || s.Event.Operation != key {
			continue
		}
		if prior != nil && prior.ID != s.ID {
			return nil, issueFailure("operation_conflict", errors.New("actor operation key has multiple signed records"))
		}
		prior = s
	}
	return prior, nil
}
func ObserveResourceOperation(events []StoredEvent, actor, key, family string) (*ResourceOperationResult, error) {
	prior, err := findResourceOperation(events, actor, key)
	if err != nil {
		return nil, err
	}
	if prior == nil {
		return nil, issueFailure("not_found", errors.New("actor operation was not found"))
	}
	if resourceFamily(prior.Event.Kind) != family {
		return nil, issueFailure("operation_conflict", errors.New("operation belongs to another resource kind"))
	}
	e := prior.Event
	return &ResourceOperationResult{ResourceID: resourceRootID(*prior), EventID: prior.ID, Actor: e.Actor, Operation: e.Operation, Kind: e.Kind, Intent: e.Intent, Parents: append([]string{}, e.Parents...), Request: e.Request, State: json.RawMessage(e.Body)}, nil
}
func MutateResource(input ResourceMutation) (*ResourceMutationResult, error) {
	if !validResourceFamily(input.Family) {
		return nil, issueFailure("invalid_input", errors.New("unsupported resource family"))
	}
	identity, err := loadIdentity()
	if err != nil {
		return nil, issueFailure("repository_error", err)
	}
	if input.Actor != "" && input.Actor != identity.Actor {
		return nil, issueFailure("actor_mismatch", errors.New("active signing actor differs from expected actor"))
	}
	kind := input.Family + ".revise"
	if input.Intent == "open" {
		kind = input.Family + ".open"
	}
	captured, err := nextEvent(identity, kind)
	if err != nil {
		return nil, err
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		return nil, err
	}
	return applyResourceMutation(input, identity, captured, events)
}
func applyResourceMutation(input ResourceMutation, identity *Identity, captured Event, events []StoredEvent) (*ResourceMutationResult, error) {
	if input.Actor != "" && input.Actor != identity.Actor {
		return nil, issueFailure("actor_mismatch", errors.New("active signing actor differs from expected actor"))
	}
	state, err := CanonicalResourceState(input.Family, input.State)
	if err != nil {
		return nil, issueFailure("invalid_input", err)
	}
	event := captured
	event.Subject = input.ID
	event.Parents = append([]string{}, input.Expected...)
	sort.Strings(event.Parents)
	event.Intent = input.Intent
	event.Operation = input.Operation
	event.Body = string(state)
	if event.Operation != "" {
		event.Request, err = ResourceRequestDigest(event)
		if err != nil {
			return nil, issueFailure("invalid_input", err)
		}
	}
	if err = validateResourceEvent(event); err != nil {
		return nil, issueFailure("invalid_input", err)
	}
	cat, err := buildGovernedResourceCatalog(events)
	if err != nil {
		return nil, issueFailure("invalid_history", err)
	}
	if input.Operation != "" {
		prior, err := findResourceOperation(events, identity.Actor, input.Operation)
		if err != nil {
			return nil, err
		}
		if prior != nil {
			if prior.Event.Kind != event.Kind || prior.Event.Request != event.Request {
				return nil, issueFailure("operation_conflict", errors.New("operation key was already used for different kind, intent, content or expectations"))
			}
			recorded, err := ResourceRequestDigest(prior.Event)
			if err != nil || recorded != prior.Event.Request {
				return nil, issueFailure("invalid_history", errors.New("stored resource operation digest mismatch"))
			}
			if err := contributionRetryNative(prior); err != nil {
				return nil, issueFailure("admission_pending", err)
			}
			view := cat.Resources[resourceRootID(*prior)]
			admission, err := contributionAdmission(events, prior.ID)
			if err != nil {
				return nil, err
			}
			result := &ResourceMutationResult{ResourceID: resourceRootID(*prior), EventID: prior.ID, Replayed: true, Admission: admission}
			if view != nil {
				result.Conflict = view.Conflict
				result.HeadCount = len(view.Heads)
			}
			return result, nil
		}
	}
	if input.Intent != "open" {
		view := cat.Resources[input.ID]
		if view == nil || view.Family != input.Family {
			return nil, issueFailure("not_found", errors.New("resource not found in requested family"))
		}
		if input.Intent == "revise" && view.Conflict {
			return nil, issueFailure("resource_conflict", errors.New("resource has concurrent heads; inspect heads and resolve explicitly"))
		}
		heads := []string{}
		for _, h := range view.Heads {
			heads = append(heads, h.ID)
		}
		if !equalIssueIDs(heads, event.Parents) {
			return nil, issueFailure("stale_revision", errors.New("expected revisions must exactly match every current head"))
		}
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	candidate := StoredEvent{ID: eventID(payload), Event: event}
	after, err := BuildResourceCatalog(append(append([]StoredEvent{}, events...), candidate))
	if err != nil {
		return nil, issueFailure("invalid_input", err)
	}
	if err := contributionAuthorizeCandidate(event, events); err != nil {
		return nil, issueFailure("permission_denied", err)
	}
	localPlan, err := contributionBeforeNative(event, identity, events)
	if err != nil {
		return nil, issueFailure("admission_refused", err)
	}
	stored, err := appendEvent(event, identity)
	if err != nil {
		next, nextErr := nextEvent(identity, event.Kind)
		if nextErr == nil && (next.Previous != captured.Previous || next.Sequence != captured.Sequence) {
			return nil, issueFailure("concurrent_update", errors.New("actor history changed; retry immutable operation and expectations"))
		}
		return nil, issueFailure("repository_error", err)
	}
	if err := contributionFinishNative(stored, localPlan); err != nil {
		return nil, issueFailure("admission_pending", err)
	}
	afterEvents := append(append([]StoredEvent{}, events...), *stored)
	if localPlan != nil {
		after, err = buildGovernedResourceCatalog(afterEvents)
		if err != nil {
			return nil, err
		}
	}
	view := after.Resources[resourceRootID(*stored)]
	admission, err := contributionAdmission(afterEvents, stored.ID)
	if err != nil {
		return nil, err
	}
	result := &ResourceMutationResult{ResourceID: resourceRootID(*stored), EventID: stored.ID, Admission: admission}
	if view != nil && (admission == "" || admission == "admitted") {
		result.Conflict = view.Conflict
		result.HeadCount = len(view.Heads)
	}
	return result, nil
}

// NativeResourceCommands is looked up when dispatching, avoiding init-order
// replacement races. Family command files register their specialization here.
var NativeResourceCommands = map[string]func([]string) error{}

func init() {
	for _, family := range []string{ResourceProject, ResourceProjectItem, ResourceDiscussion, ResourceDiscussionReply, ResourceWorkflow} {
		family := family
		localCommands[family] = func(args []string) error {
			if command := NativeResourceCommands[family]; command != nil {
				return command(args)
			}
			return CmdResource(family, args)
		}
	}
}

type resourcePage struct {
	Items      any    `json:"items"`
	Snapshot   string `json:"snapshot"`
	NextCursor string `json:"next_cursor,omitempty"`
}
type resourceCursor struct {
	Family   string `json:"family"`
	Command  string `json:"command"`
	ID       string `json:"id"`
	Snapshot string `json:"snapshot"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

// CmdResource provides the full-state baseline retained by family convenience
// commands. Every list/history/heads page binds a stable accepted-ref snapshot.
func CmdResource(family string, args []string) error {
	if !validResourceFamily(family) {
		return fmt.Errorf("unsupported resource family")
	}
	if len(args) == 0 || args[0] == "help" {
		fmt.Printf("hn %s open --input FILE|- [--actor ACTOR] [--operation KEY] [--json]\nhn %s revise|resolve FULL_ID --expect FULL_REV ... --input FILE|- [--actor ACTOR] [--operation KEY] [--json]\nhn %s list|show|heads|history [ID] [--limit 1..200] [--cursor TOKEN] [--json]\nhn %s operation --actor ACTOR --operation KEY [--json]\n", family, family, family, family)
		return nil
	}
	command := args[0]
	switch command {
	case "open", "revise", "resolve", "list", "show", "heads", "history", "operation":
	default:
		return usageError("unsupported resource command")
	}
	flags := flag.NewFlagSet(family+" "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var path, actor, key, cursor string
	var machine bool
	limit := 50
	expected := issueExpected{}
	flags.StringVar(&path, "input", "", "")
	flags.StringVar(&actor, "actor", "", "")
	flags.StringVar(&key, "operation", "", "")
	flags.StringVar(&cursor, "cursor", "", "")
	flags.BoolVar(&machine, "json", false, "")
	flags.IntVar(&limit, "limit", 50, "")
	flags.Var(&expected, "expect", "")
	// Go flag stops at the first positional argument; normalize explicit ID first.
	rest := append([]string{}, args[1:]...)
	id := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		id = rest[0]
		rest = rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("unexpected positional arguments")
	}
	mutation := command == "open" || command == "revise" || command == "resolve"
	var data any
	var resultErr error
	if mutation {
		if path == "" || cursor != "" || flags.Lookup("limit").Value.String() != "50" {
			return usageError("resource mutation requires --input and no pagination")
		}
		if command == "open" && id != "" || command != "open" && !validEventID(id) {
			return usageError("resource revisions require a full aggregate ID")
		}
		var reader io.Reader = os.Stdin
		var file *os.File
		if path != "-" {
			var err error
			file, err = os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			reader = file
		}
		raw, err := io.ReadAll(io.LimitReader(reader, int64(resourcePayloadLimit(family)+1)))
		if err != nil {
			return err
		}
		data, resultErr = MutateResource(ResourceMutation{family, command, id, expected, raw, actor, key})
	} else {
		if path != "" || len(expected) != 0 {
			return usageError("resource queries do not accept input or expectations")
		}
		data, resultErr = queryResource(family, command, id, actor, key, limit, cursor)
	}
	if machine {
		envelope := map[string]any{"ok": resultErr == nil, "data": data}
		if resultErr != nil {
			code := "resource_error"
			var typed *issueCommandError
			if errors.As(resultErr, &typed) {
				code = typed.Code
			}
			envelope["error"] = map[string]string{"code": code, "message": resultErr.Error()}
		}
		if err := json.NewEncoder(os.Stdout).Encode(envelope); err != nil {
			return err
		}
	} else if resultErr == nil {
		raw, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(raw))
	}
	return resultErr
}
func queryResource(family, command, id, actor, key string, limit int, cursor string) (any, error) {
	if limit < 1 || limit > 200 {
		return nil, issueFailure("invalid_input", errors.New("page limit must be 1..200"))
	}
	if command != "operation" && (actor != "" || key != "") {
		return nil, usageError("actor/operation selectors require operation query")
	}
	events, snapshot, err := collectIssueEvents()
	if err != nil {
		return nil, err
	}
	if command == "operation" {
		if id != "" || !validActorFingerprint(actor) || key == "" || cursor != "" {
			return nil, usageError("operation requires explicit full actor and operation key")
		}
		result, err := ObserveResourceOperation(events, actor, key, family)
		if err != nil {
			return nil, err
		}
		admission, err := contributionAdmission(events, result.EventID)
		if err != nil {
			return nil, err
		}
		result.Admission = admission
		if admission != "" && admission != "admitted" {
			result.State = nil
		}
		return result, nil
	}
	cat, err := buildGovernedResourceCatalog(events)
	if err != nil {
		return nil, err
	}
	if command == "list" && id != "" {
		return nil, usageError("list does not accept resource ID")
	}
	var view *ResourceView
	if command != "list" {
		for _, root := range cat.IDs {
			if cat.Resources[root].Family == family && strings.HasPrefix(strings.TrimPrefix(root, "sha256:"), strings.TrimPrefix(id, "sha256:")) {
				if view != nil {
					return nil, issueFailure("ambiguous_id", errors.New("resource prefix is ambiguous"))
				}
				view = cat.Resources[root]
			}
		}
		if view == nil || len(strings.TrimPrefix(id, "sha256:")) < 8 {
			return nil, issueFailure("not_found", errors.New("resource not found"))
		}
		id = view.ID
	}
	if command == "show" {
		if cursor != "" {
			return nil, usageError("show does not accept cursor")
		}
		return view, nil
	}
	offset := 0
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || len(raw) > 4096 {
			return nil, issueFailure("invalid_cursor", errors.New("invalid resource cursor"))
		}
		var c resourceCursor
		if err = strictResourceJSON(raw, &c, 4096); err != nil || c.Family != family || c.Command != command || c.ID != id || c.Limit != limit || c.Offset < 0 {
			return nil, issueFailure("invalid_cursor", errors.New("cursor does not match query"))
		}
		if c.Snapshot != snapshot {
			return nil, issueFailure("stale_cursor", errors.New("accepted resource snapshot changed"))
		}
		offset = c.Offset
	}
	items := []any{}
	switch command {
	case "list":
		for _, root := range cat.IDs {
			v := cat.Resources[root]
			if v.Family == family {
				items = append(items, v)
			}
		}
	case "heads":
		for _, head := range view.Heads {
			items = append(items, head)
		}
	case "history":
		for _, s := range view.History {
			items = append(items, map[string]any{"id": s.ID, "actor": s.Event.Actor, "kind": s.Event.Kind, "parents": append([]string{}, s.Event.Parents...), "intent": s.Event.Intent, "operation": s.Event.Operation, "state": json.RawMessage(s.Event.Body)})
		}
	}
	if offset > len(items) {
		return nil, issueFailure("invalid_cursor", errors.New("cursor offset exceeds result"))
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := resourcePage{Items: items[offset:end], Snapshot: snapshot}
	if end < len(items) {
		raw, _ := json.Marshal(resourceCursor{family, command, id, snapshot, end, limit})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}

// ResourceStateJSON helps typed callers use exactly the same strict contract.
func ResourceStateJSON(family string, state any) (json.RawMessage, error) {
	if err := resourceInputStrings(reflect.ValueOf(state), 0); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return CanonicalResourceState(family, raw)
}
