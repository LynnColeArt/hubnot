package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	c "hubnot/internal/contribution"
	"io"
	"os"
	"sort"
	"strings"

	"hubnot/internal/extension"
)

// Registration is inert. Native commands never inspect extension state.
func extensionNativeHostError(code, message, recovery string) error {
	return &extension.Error{Origin: "host", Code: code, Message: message, Recovery: recovery}
}
func extensionNativeError(err error, params map[string]any) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return extensionNativeHostError("cancelled", "native request cancelled before publication", "do_not_retry")
	}
	var public *extension.Error
	if errors.As(err, &public) {
		return public
	}
	code, message := "repository_error", "native repository operation failed"
	var command *issueCommandError
	var read *IssueReadError
	if errors.As(err, &command) {
		code, message = command.Code, command.Message
	} else if errors.As(err, &read) {
		code = read.Code
		message = "native accepted history could not be verified"
	}
	switch code {
	case "actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision":
	default:
		code, message = "repository_error", "native repository operation failed"
	}
	recovery := "do_not_retry"
	switch code {
	case "concurrent_update", "repository_error":
		recovery = "retry_identical"
	case "admission_pending":
		recovery = "inspect_operation"
	case "stale_cursor", "invalid_cursor":
		recovery = "rescan_required"
	}
	// Repository diagnostics can include private filesystem paths or Git stderr.
	// Validation may quote input values; no input dump is part of this boundary.
	switch code {
	case "repository_error":
		message = "native repository operation failed"
	case "invalid_history":
		message = "native accepted history could not be verified"
	case "invalid_input":
		message = "native input is invalid"
	case "admission_pending":
		message = "native admission is pending; inspect the original operation"
	case "admission_refused":
		message = "native contribution policy refused the operation"
	}
	if len(message) > 4096 || strings.ContainsAny(message, "/\\\r\n") {
		message = "native operation refused; inspect local native diagnostics"
	}
	result := &extension.Error{Origin: "native", Code: code, Message: message, Recovery: recovery}
	if recovery == "inspect_operation" && params["actor"] != nil {
		result.Operation = map[string]any{"protocol": "hn/0", "actor": params["actor"], "operation": params["operation"]}
	}
	return result
}
func extensionNativeDispatch(ctx context.Context, call extension.Invocation) (any, error) {
	if err := extension.AuthorizeRequest(call); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, extensionNativeHostError("cancelled", "request cancelled before native dispatch", "do_not_retry")
	}
	repo, _, err := artifactRepositoryID()
	if err != nil {
		return nil, extensionNativeError(err, call.Request.Params)
	}
	if repo != call.Binding.RepositoryID {
		return nil, extensionNativeHostError("denied_access", "repository binding changed", "do_not_retry")
	}
	p := call.Request.Params
	family, _ := p["family"].(string)
	operation := "list"
	switch call.Request.Method {
	case "resource.read":
		if p["id"] != nil {
			operation = "show"
		}
	case "resource.operation":
		operation = "operation"
	case "resource.mutate":
		operation = p["intent"].(string)
	}
	if family == "" {
		family = "identity"
		operation = "public"
	}
	unlock, err := contributionCommandLock([]string{family, operation})
	if err != nil {
		return nil, extensionNativeHostError("repository_error", "native repository operation is busy or unavailable", "retry_identical")
	}
	defer unlock()
	if ctx.Err() != nil {
		return nil, extensionNativeHostError("cancelled", "request cancelled before native dispatch", "do_not_retry")
	}
	var result any
	switch call.Request.Method {
	case "identity.public":
		result, err = extensionNativeIdentity()
	case "resource.read":
		result, err = extensionNativeRead(ctx, call)
	case "resource.mutate":
		result, err = extensionNativeMutate(ctx, call)
	case "resource.operation":
		result, err = extensionNativeOperation(ctx, call)
	default:
		err = extensionNativeHostError("unsupported_capability", "native method is unsupported", "do_not_retry")
	}
	return result, extensionNativeError(err, p)
}
func extensionNativeIdentity() (any, error) {
	identity, err := inspectIdentity()
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{"initialized": false, "actor": nil, "name": nil, "public_key": nil}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"initialized": true, "actor": identity.Actor, "name": identity.Name, "public_key": identity.PublicKey}, nil
}

type extensionNativeView struct {
	Protocol string `json:"protocol"`
	Family   string `json:"family"`
	ID       string `json:"id"`
	Creator  string `json:"creator"`
	Heads    []any  `json:"heads"`
	Conflict bool   `json:"conflict"`
	State    any    `json:"state"`
	Comments []any  `json:"comments"`
}

// These are the exact verified inputs consumed by the current single-family
// Cut. They belong to this captured observation; composition must copy arrays
// and verify matching repository/source/authority inputs before unioning them.
// Keeping the preimage beside its hash avoids reconstructing authority or
// visibility from raw history or from a digest in later current callers.
type extensionNativeCutInputs struct {
	RepositoryID    string
	Refs            []extensionNativeRef
	Selection       extensionNativeReadSelection
	AuthorityCommit string
	PolicyDigest    string
	AdmittedIDs     []string
	VisibleIDs      []string
	RetainedRefs    []extensionNativeRef
}
type extensionNativeReadSelection struct {
	Families []string `json:"families"`
	Types    []string `json:"types"`
}
type extensionNativeProjection struct {
	cutInputs extensionNativeCutInputs
	events    []StoredEvent
	views     []extensionNativeView
	cut       string
	authority *contributionAuthority
	visible   map[string]bool
}
type extensionNativeRef struct {
	OID string `json:"oid"`
	Ref string `json:"ref"`
}

func extensionNativeFamily(event Event) string {
	if isIssueKind(event.Kind) {
		return "issue"
	}
	return resourceFamily(event.Kind)
}
func extensionNativeProjectionFor(ctx context.Context, family string) (*extensionNativeProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	events, snapshot, err := collectIssueEvents()
	if err != nil {
		return nil, err
	}
	projection := &extensionNativeProjection{events: events, views: []extensionNativeView{}, visible: map[string]bool{}}
	view, authority, err := contributionView(events)
	if err != nil {
		return nil, issueFailure("invalid_history", err)
	}
	projection.authority = authority
	admitted, visible := []string{}, []string{}
	for _, s := range events {
		if view == nil || view.Visible[s.ID] {
			projection.visible[s.ID] = true
		}
		if extensionNativeFamily(s.Event) != family {
			continue
		}
		if view == nil || view.Admitted[s.ID] {
			admitted = append(admitted, s.ID)
		}
		if view == nil || view.Visible[s.ID] {
			visible = append(visible, s.ID)
			projection.visible[s.ID] = true
		}
	}
	if family == "issue" {
		catalog, e := buildGovernedIssueCatalog(events)
		if e != nil {
			return nil, issueFailure("invalid_history", e)
		}
		for _, id := range catalog.IDs {
			v := catalog.Issues[id]
			out := extensionNativeView{Protocol: "hn/0", Family: family, ID: id, Creator: v.Creator, Heads: []any{}, Conflict: v.Conflict, Comments: []any{}}
			if v.State != nil {
				out.State = extensionNativeState(family, v.State)
			}
			sort.Slice(v.Heads, func(i, j int) bool { return v.Heads[i].ID < v.Heads[j].ID })
			for _, h := range v.Heads {
				out.Heads = append(out.Heads, map[string]any{"id": h.ID, "actor": h.Actor, "state": extensionNativeState(family, h.State)})
			}
			sort.Slice(v.Comments, func(i, j int) bool { return v.Comments[i].ID < v.Comments[j].ID })
			for _, c := range v.Comments {
				out.Comments = append(out.Comments, map[string]any{"id": c.ID, "actor": c.Event.Actor, "body": c.Event.Body})
			}
			projection.views = append(projection.views, out)
		}
	} else {
		catalog, e := buildGovernedResourceCatalog(events)
		if e != nil {
			return nil, issueFailure("invalid_history", e)
		}
		for _, id := range catalog.IDs {
			v := catalog.Resources[id]
			if v.Family != family {
				continue
			}
			out := extensionNativeView{Protocol: "hn/0", Family: family, ID: id, Creator: v.Creator, Heads: []any{}, Conflict: v.Conflict, Comments: []any{}}
			if !v.Conflict && len(v.State) > 0 {
				out.State = extensionNativeState(family, v.State)
			}
			sort.Slice(v.Heads, func(i, j int) bool { return v.Heads[i].ID < v.Heads[j].ID })
			for _, h := range v.Heads {
				out.Heads = append(out.Heads, map[string]any{"id": h.ID, "actor": h.Actor, "state": extensionNativeState(family, h.State)})
			}
			projection.views = append(projection.views, out)
		}
	}
	sort.Slice(projection.views, func(i, j int) bool { return projection.views[i].ID < projection.views[j].ID })
	refs, retained := []extensionNativeRef{}, []extensionNativeRef{}
	refArgs := []string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes"}
	if authority != nil && authority.Policy.Contributions != nil {
		refArgs = append(refArgs, "refs/hn/contributions/"+authority.Policy.Contributions.Project[7:]+"/")
	}
	witnesses := 0
	err = issueGitLines(nil, func(line string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		witnesses++
		if witnesses > defaultIssueReadLimits().refs+c.MaxGraphRecords {
			return issueReadFailure("resource_limit", errors.New("native source witnesses exceed verified selection bound"))
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !validGitOID(fields[1]) {
			return issueReadFailure("invalid_history", fmt.Errorf("invalid accepted ref"))
		}
		ref := fields[0]
		row := extensionNativeRef{Ref: ref, OID: fields[1]}
		actor := strings.TrimPrefix(ref, "refs/hn/actors/")
		_, remoteActor, acceptedRemote := parseAcceptedActorRef(ref)
		if strings.HasPrefix(ref, "refs/hn/actors/") || acceptedRemote {
			if acceptedRemote {
				actor = remoteActor
			}
			head, e := readStoredEvent(fields[1])
			if e != nil || head.Event.Actor != actor {
				return issueReadFailure("invalid_history", errors.New("native accepted source head is not a verified actor event"))
			}
			refs = append(refs, row)
			if acceptedRemote {
				retained = append(retained, row)
			}
		} else if authority != nil && authority.Policy.Contributions != nil && strings.HasPrefix(ref, "refs/hn/contributions/"+authority.Policy.Contributions.Project[7:]+"/") {
			retained = append(retained, row)
		}
		return nil
	}, refArgs...)
	if err != nil {
		return nil, err
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Ref < refs[j].Ref })
	sort.Slice(retained, func(i, j int) bool { return retained[i].Ref < retained[j].Ref })
	// A concurrent raw Git/native writer is outside the contribution lock. Detect
	// changes across projection and Cut capture instead of mixing two observations.
	_, current, err := issueReadRefs(defaultIssueReadLimits())
	if err != nil {
		return nil, err
	}
	current, err = contributionReadSnapshot(current)
	if err != nil {
		return nil, err
	}
	if current != snapshot {
		return nil, issueFailure("concurrent_update", fmt.Errorf("accepted history changed during native observation"))
	}
	finalAuthority, err := loadContributionAuthority()
	if err != nil {
		return nil, issueFailure("invalid_history", err)
	}
	if (authority == nil) != (finalAuthority == nil) || authority != nil && (authority.Commit != finalAuthority.Commit || authority.PolicyDigest != finalAuthority.PolicyDigest || authority.Digest != finalAuthority.Digest) {
		return nil, issueFailure("concurrent_update", errors.New("native authority changed during observation"))
	}
	// Admission and visibility are deterministic for the unchanged verified events,
	// selected authority and manifest refs. Recheck receiver-local retention too;
	// a private coverage loss is not a successful empty projection.
	if view != nil {
		if err := contributionRequireRetention(finalAuthority, view); err != nil {
			return nil, issueFailure("invalid_history", err)
		}
	}
	_, finalSnapshot, err := issueReadRefs(defaultIssueReadLimits())
	if err != nil {
		return nil, err
	}
	finalSnapshot, err = contributionReadSnapshot(finalSnapshot)
	if err != nil {
		return nil, err
	}
	if finalSnapshot != snapshot {
		return nil, issueFailure("concurrent_update", errors.New("native source witnesses changed during observation"))
	}
	repo, _, err := artifactRepositoryID()
	if err != nil {
		return nil, err
	}
	sort.Strings(admitted)
	sort.Strings(visible)
	projection.cutInputs = extensionNativeCutInputs{RepositoryID: repo, Refs: refs, Selection: extensionNativeReadSelection{Families: []string{family}, Types: []string{}}, AdmittedIDs: admitted, VisibleIDs: visible, RetainedRefs: retained}
	input := &projection.cutInputs
	if authority != nil {
		input.AuthorityCommit, input.PolicyDigest = authority.Commit, authority.PolicyDigest
	}
	viewRaw, _ := json.Marshal(map[string]any{"admitted_ids": input.AdmittedIDs, "visible_ids": input.VisibleIDs, "retained_refs": input.RetainedRefs})
	raw, _ := json.Marshal(map[string]any{"repository_id": input.RepositoryID, "refs": input.Refs, "selection": input.Selection, "authority_commit": input.AuthorityCommit, "policy_digest": input.PolicyDigest, "view_digest": eventID(viewRaw)})
	projection.cut = eventID(raw)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return projection, nil
}

// Native project-item storage omits optional empty target fields. The typed
// host draft target requires its complete title/body pair even when body is empty.
func extensionNativeState(family string, state any) any {
	if family != "project-item" {
		return state
	}
	raw, _ := json.Marshal(state)
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return state
	}
	if target, ok := value["target"].(map[string]any); ok && target["kind"] == "draft" {
		if _, exists := target["body"]; !exists {
			target["body"] = ""
		}
	}
	return value
}

type extensionNativeCursor struct {
	Cut, Family, ID, Repository, Registration string
	Epoch                                     string
	Offset                                    int
}

func extensionNativeRead(ctx context.Context, call extension.Invocation) (any, error) {
	p := call.Request.Params
	family := p["family"].(string)
	projection, err := extensionNativeProjectionFor(ctx, family)
	if err != nil {
		return nil, err
	}
	id, _ := p["id"].(string)
	items := []extensionNativeView{}
	for _, v := range projection.views {
		if id == "" || id == v.ID {
			items = append(items, v)
		}
	}
	if id != "" && len(items) == 0 {
		return nil, issueFailure("not_found", fmt.Errorf("resource not found in requested family"))
	}
	cursor := extensionNativeCursor{Cut: projection.cut, Family: family, ID: id, Repository: call.Binding.RepositoryID, Registration: call.Binding.RegistrationID, Epoch: call.Binding.Epoch}
	offset := 0
	if token, ok := p["cursor"].(string); ok {
		raw, e := base64.RawURLEncoding.DecodeString(token)
		var prior extensionNativeCursor
		if e != nil || json.Unmarshal(raw, &prior) != nil {
			return nil, issueFailure("invalid_cursor", fmt.Errorf("invalid native cursor"))
		}
		offset = prior.Offset
		prior.Offset = 0
		if prior != cursor || offset < 0 || offset > len(items) {
			return nil, issueFailure("stale_cursor", fmt.Errorf("native cursor no longer matches the selected view"))
		}
	}
	limit := int(p["limit"].(float64))
	end := min(offset+limit, len(items))
	var next any
	if end < len(items) {
		cursor.Offset = end
		raw, _ := json.Marshal(cursor)
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return map[string]any{"items": items[offset:end], "source_cut": projection.cut, "next_cursor": next}, nil
}
func extensionNativeMutationResult(projection *extensionNativeProjection, family, id, event string, replayed bool, admission string) map[string]any {
	var admitted any
	if admission != "" {
		admitted = admission
	}
	conflict, heads := false, 0
	for _, v := range projection.views {
		if v.ID == id {
			conflict, heads = v.Conflict, len(v.Heads)
			break
		}
	}
	return map[string]any{"protocol": "hn/0", "family": family, "type_id": nil, "resource_id": id, "event_id": event, "replayed": replayed, "conflict": conflict, "head_count": heads, "admission": admitted, "source_cut": projection.cut}
}
func extensionNativeMutate(ctx context.Context, call extension.Invocation) (any, error) {
	p := call.Request.Params
	family := p["family"].(string)
	// This reader does not migrate or initialize identity. It runs after grants,
	// binding and lock, before the writer captures/signs/reserves anything.
	identity, err := inspectIdentity()
	if err != nil {
		return nil, issueFailure("repository_error", err)
	}
	actor := p["actor"].(string)
	if identity.Actor != actor {
		message := "active signing actor differs from expected actor"
		if family == "issue" {
			message = "active signing actor differs from the expected actor; repair the binding"
		}
		return nil, issueFailure("actor_mismatch", errors.New(message))
	}
	if err = ctx.Err(); err != nil {
		return nil, extensionNativeHostError("cancelled", "request cancelled before native dispatch", "do_not_retry")
	}
	expected := []string{}
	for _, v := range p["expected"].([]any) {
		expected = append(expected, v.(string))
	}
	id, _ := p["id"].(string)
	intent := p["intent"].(string)
	key := p["operation"].(string)
	resultID, signedID := "", ""
	replayed := false
	admission := ""
	if family == "issue" {
		options := issueOptions{command: intent, id: id, expected: expected, actor: actor, operation: key, body: p["comment"].(string)}
		if p["state"] != nil {
			raw, _ := json.Marshal(p["state"])
			var state IssueState
			if err = json.Unmarshal(raw, &state); err != nil {
				return nil, issueFailure("invalid_input", err)
			}
			options.state = &state
		}
		value, e := executeIssueMutationContext(ctx, options)
		if e != nil {
			return nil, e
		}
		r := value.(*issueMutationResult)
		resultID, signedID, replayed, admission = r.IssueID, r.EventID, r.Replayed, r.Admission
	} else {
		raw, _ := json.Marshal(p["state"])
		r, e := MutateResourceContext(ctx, ResourceMutation{Family: family, Intent: intent, ID: id, Expected: expected, State: raw, Actor: actor, Operation: key})
		if e != nil {
			return nil, e
		}
		resultID, signedID, replayed, admission = r.ResourceID, r.EventID, r.Replayed, r.Admission
	}
	projection, err := extensionNativeProjectionFor(ctx, family)
	if err != nil {
		return nil, &extension.Error{Origin: "host", Code: "uncertain_operation", Message: "native publication requires operation inspection", Recovery: "inspect_operation", Operation: map[string]any{"protocol": "hn/0", "actor": actor, "operation": key}}
	}
	return extensionNativeMutationResult(projection, family, resultID, signedID, replayed, admission), nil
}
func extensionNativeOperation(ctx context.Context, call extension.Invocation) (any, error) {
	p := call.Request.Params
	family, actor, key := p["family"].(string), p["actor"].(string), p["operation"].(string)
	projection, err := extensionNativeProjectionFor(ctx, family)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"status": "verified_absent", "protocol": "hn/0", "actor": actor, "operation": key, "family": family, "type_id": nil, "request": nil, "resource_id": nil, "event_id": nil, "result": nil, "source_cut": projection.cut, "reason": nil}
	prior, err := findResourceOperation(projection.events, actor, key)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		// Hide/pending admission cannot reveal the raw resource/event/request ID.
		if !projection.visible[prior.ID] {
			result["status"], result["reason"] = "uncertain", "visibility_restricted"
			return result, nil
		}
		if extensionNativeFamily(prior.Event) != family {
			return nil, issueFailure("operation_conflict", errors.New("operation belongs to another resource kind"))
		}
		id := resourceRootID(*prior)
		request := ""
		if family == "issue" {
			r, e := observeIssueOperation(projection.events, actor, key)
			if e != nil {
				return nil, e
			}
			request = r.Request
			id = r.IssueID
		} else {
			r, e := ObserveResourceOperation(projection.events, actor, key, family)
			if e != nil {
				return nil, e
			}
			request = r.Request
		}
		if request == "" {
			result["status"], result["reason"] = "uncertain", "publication_unconfirmed"
			return result, nil
		}
		visibleRoot := false
		for _, v := range projection.views {
			if v.ID == id {
				visibleRoot = true
				break
			}
		}
		if !visibleRoot {
			result["status"], result["reason"] = "uncertain", "visibility_restricted"
			return result, nil
		}
		admission, e := contributionAdmission(projection.events, prior.ID)
		if e != nil {
			return nil, e
		}
		result["status"], result["request"], result["resource_id"], result["event_id"] = "found", request, id, prior.ID
		result["result"] = extensionNativeMutationResult(projection, family, id, prior.ID, true, admission)
		return result, nil
	}
	// A fully verified accepted walk is necessary but not sufficient for absence:
	// inspect existing local reservations without creating an exchange or retrying.
	directory, e := contributionSharedDirectory(false)
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	if e == nil {
		dir, e := os.Open(directory)
		if e != nil {
			return nil, e
		}
		entries, e := dir.ReadDir(c.MaxGraphRecords + 1)
		dir.Close()
		if e != nil && !errors.Is(e, io.EOF) {
			return nil, e
		}
		if len(entries) > c.MaxGraphRecords {
			return nil, issueFailure("resource_limit", errors.New("native local pending selection exceeds read bound"))
		}
		selected := ""
		if a := projection.authority; a != nil && a.Policy.Contributions != nil {
			selected = "inbox-" + a.Policy.Contributions.Project[7:]
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "inbox-") && entry.Name() != selected {
				result["status"], result["reason"] = "uncertain", "incomplete_selection"
				return result, nil
			}
		}
	}
	if a := projection.authority; a != nil && a.Policy.Contributions != nil {
		x, e := contributionExistingExchange(a)
		if e != nil {
			return nil, issueFailure("invalid_history", e)
		}
		j, e := x.load()
		if e != nil {
			return nil, issueFailure("invalid_history", e)
		}
		for _, attempt := range j.Attempts {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !strings.HasPrefix(attempt.Intent, "native-") {
				continue
			}
			plan, e := contributionLoadLocalPlan(a, x, attempt.Intent)
			if e != nil {
				return nil, issueFailure("invalid_history", e)
			}
			if plan == nil {
				result["status"], result["reason"] = "uncertain", "private_intent_pending"
				return result, nil
			}
			event, _, e := verifyEvent(plan.descriptor.NativePayload, plan.descriptor.NativeSignature)
			if e != nil {
				return nil, issueFailure("invalid_history", e)
			}
			if event.Actor == actor && event.Operation == key {
				result["status"], result["reason"] = "uncertain", "private_intent_pending"
				return result, nil
			}
		}
	}
	return result, nil
}
