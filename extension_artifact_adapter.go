package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"sort"
	"strings"

	"hubnot/internal/artifact"
	"hubnot/internal/extension"
)

// Static registration is inert on every ordinary native/artifact command path.
func init() {
	for _, method := range []string{"artifact.read", "artifact.search", "artifact.type.read", "artifact.mutate", "artifact.type.publish", "artifact.operation"} {
		extensionHostMethods[method] = extensionArtifactDispatch
	}
}

func extensionArtifactScope(call extension.Invocation, typeID, action string) error {
	return extension.Authorize(call.Registration, call.Enablement, call.Binding, extension.Access{Method: call.Request.Method, Selector: "artifact:" + typeID, Action: action}, false)
}

// Resolve broad/id-only selection only within explicit applicable read grants.
func extensionArtifactReadGrant(call extension.Invocation) error {
	for _, grant := range call.Enablement.Grants {
		if !strings.HasPrefix(grant.Selector, "artifact:") || grant.Selector == "artifact:opaque" {
			continue
		}
		probe := strings.TrimPrefix(grant.Selector, "artifact:")
		if strings.HasSuffix(probe, "/*") {
			probe = strings.TrimSuffix(probe, "*") + "probe"
		}
		if extensionArtifactScope(call, probe, "read") == nil {
			return nil
		}
	}
	return frameInputError("denied_access", "artifact selection requires a declared read grant")
}
func extensionArtifactError(err error, call extension.Invocation) error {
	if err == nil {
		return nil
	}
	var host *extension.Error
	if errors.As(err, &host) {
		return host
	}
	origin, code, message := "artifact", "repository_error", "artifact request failed"
	var input *artifact.Error
	var storage *artifactStorageError
	if errors.As(err, &input) {
		code, message = input.Code, input.Message
	}
	if errors.As(err, &storage) {
		origin, code, message = storage.Origin, storage.Code, storage.Message
	}
	recovery := "do_not_retry"
	switch code {
	case "concurrent_update":
		recovery = "retry_identical"
	case "checkpoint_invalidated":
		recovery = "rescan_required"
	case "uncertain_operation":
		recovery = "inspect_operation"
	}
	result := &extension.Error{Origin: origin, Code: code, Message: message, Recovery: recovery}
	if code == "uncertain_operation" {
		result.Operation = map[string]any{"protocol": "hn-artifact/1", "actor": call.Request.Params["actor"], "operation": call.Request.Params["operation"]}
	}
	return result
}
func extensionArtifactDispatch(ctx context.Context, call extension.Invocation) (data any, err error) {
	defer func() { err = extensionArtifactError(err, call) }()
	// The transport has checked closed typed params. Check the current binding
	// before even reading data; unresolved ID scopes are checked after resolution.
	if err = extension.Authorize(call.Registration, call.Enablement, call.Binding, extension.Access{Method: "capabilities.list"}, false); err != nil {
		return nil, err
	}
	p := call.Request.Params
	switch call.Request.Method {
	case "artifact.mutate", "artifact.type.publish", "artifact.operation":
		if err = extension.AuthorizeRequest(call); err != nil {
			return nil, err
		}
	case "artifact.read":
		if p["opaque"] == true || p["type_id"] != nil {
			if err = extension.AuthorizeRequest(call); err != nil {
				return nil, err
			}
		}
	case "artifact.type.read", "artifact.search":
		if p["type_id"] != nil {
			if err = extension.AuthorizeRequest(call); err != nil {
				return nil, err
			}
		}
	}
	if (call.Request.Method == "artifact.read" && p["opaque"] != true || call.Request.Method == "artifact.type.read" || call.Request.Method == "artifact.search") && p["type_id"] == nil {
		if err = extensionArtifactReadGrant(call); err != nil {
			return nil, err
		}
	}
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact request cancelled")
	}
	switch call.Request.Method {
	case "artifact.mutate", "artifact.type.publish":
		data, err = extensionArtifactMutation(ctx, call)
	case "artifact.operation":
		data, err = extensionArtifactOperation(ctx, call)
	default:
		data, err = extensionArtifactRead(ctx, call)
	}
	if err != nil {
		return nil, err
	}
	// Include correlation/envelope overhead, never truncate full states or raw
	// Base64. A single oversized object needs explicit diagnostic file export.
	encoded, e := json.Marshal(map[string]any{"protocol": "hn.extension/1", "request_id": call.Request.ID, "ok": true, "data": data, "error": nil})
	if e != nil {
		return nil, artifactFailure("repository_error", "cannot encode complete artifact response")
	}
	if len(encoded) > extension.FrameLimit {
		return nil, artifactFailure("resource_limit", "complete artifact response exceeds 8 MiB; reduce page limit or use explicit generic CLI/file export for an oversized full object")
	}
	return data, nil
}

func extensionArtifactMutation(ctx context.Context, call extension.Invocation) (artifactMutationResult, error) {
	var zero artifactMutationResult
	p := call.Request.Params
	kind := artifactText(p, "kind")
	if call.Request.Method == "artifact.type.publish" {
		kind = "type.publish"
	}
	// Authorization and selected contribution checks precede signer/intent use.
	if err := artifactMutationPolicy(); err != nil {
		return zero, err
	}
	input := map[string]any{"parents": p["parents"], "payload": p["payload"]}
	if kind != "type.publish" {
		input["record"] = p["record"]
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return zero, artifactFailure("invalid_input", "invalid mutation input")
	}
	h, err := loadArtifactHistory(ctx)
	if err != nil {
		return zero, err
	}
	action := "type.publish"
	switch kind {
	case "record.open":
		action = "create"
	case "record.revise", "record.convert":
		action = "revise"
	case "record.resolve":
		action = "resolve"
	}
	// Scope the actual opening and every selected parent, including a rejected
	// cross-type conversion. Target dependencies may be verified internally.
	ids := artifactIDs(p["parents"])
	if record := artifactText(p, "record"); record != "" {
		ids = append(ids, record)
	}
	for _, id := range ids {
		if s := h.ByID[id]; s != nil {
			if err = extensionArtifactScope(call, artifactText(artifactPayload(s), "type_id"), action); err != nil {
				return zero, err
			}
		}
	}
	actor, operation := artifactText(p, "actor"), artifactText(p, "operation")
	var result artifactMutationResult
	if kind == "record.open" || kind == "type.publish" {
		result, err = writeArtifact(ctx, kind, actor, operation, raw)
	} else {
		result, err = writeArtifactSuccessor(ctx, kind, actor, operation, raw)
	}
	if err != nil {
		return zero, err
	}
	// The shared writer owns immutable publication/replay. Reobserve its exact
	// signed event through that same catalog before binding the host type Cut.
	h, err = loadArtifactHistory(ctx)
	if err != nil {
		return zero, artifactFailure("uncertain_operation", "published artifact outcome requires operation inspection")
	}
	c, err := buildArtifactCatalog(ctx, h)
	if err != nil {
		return zero, artifactFailure("uncertain_operation", "published artifact view requires operation inspection")
	}
	published := h.ByID[result.EventID]
	if published == nil || published.verified.Actor() != actor || artifactText(published.Envelope, "operation") != operation {
		return zero, artifactFailure("uncertain_operation", "published artifact witness requires operation inspection")
	}
	result = artifactResult(h, published, result.Replayed)
	result.SourceCut, err = extensionArtifactProjection(ctx, call, c, []string{result.TypeID})
	if err == nil {
		err = extensionArtifactStable(ctx, call, h)
	}
	if err != nil {
		return zero, artifactFailure("uncertain_operation", "published artifact projection requires operation inspection")
	}
	return result, nil
}

type extensionArtifactPage struct {
	Items      []any   `json:"items"`
	SourceCut  string  `json:"source_cut"`
	NextCursor *string `json:"next_cursor"`
}
type extensionArtifactCursor struct {
	Version int    `json:"version"`
	Query   string `json:"query"`
	Cut     string `json:"cut"`
	Offset  int    `json:"offset"`
}

// A cursor names exact selection/query/grants and a verified immutable source
// cut. Page size may change. It carries neither authority nor hidden payloads.
func extensionArtifactQuery(call extension.Invocation) string {
	params := map[string]any{}
	for k, v := range call.Request.Params {
		if k != "limit" && k != "cursor" {
			params[k] = v
		}
	}
	raw, _ := json.Marshal(map[string]any{"method": call.Request.Method, "params": params, "binding": call.Binding, "grants": call.Enablement.Grants})
	return eventID(raw)
}
func extensionArtifactOffset(call extension.Invocation, cut string, count int) (int, error) {
	text := artifactText(call.Request.Params, "cursor")
	if text == "" {
		if call.Request.Params["cursor"] != nil {
			return 0, artifactListCheckpoint()
		}
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(text)
	var cursor extensionArtifactCursor
	if err != nil || json.Unmarshal(raw, &cursor) != nil {
		return 0, artifactListCheckpoint()
	}
	canonical, _ := json.Marshal(cursor)
	if !bytes.Equal(raw, canonical) || base64.RawURLEncoding.EncodeToString(raw) != text || cursor.Version != 1 || cursor.Query != extensionArtifactQuery(call) || cursor.Cut != cut || cursor.Offset <= 0 || cursor.Offset >= count {
		return 0, artifactListCheckpoint()
	}
	return cursor.Offset, nil
}
func extensionArtifactPageAt(call extension.Invocation, cut string, items []any, start int) extensionArtifactPage {
	limit := int(call.Request.Params["limit"].(float64))
	end := start + int(limit)
	if end > len(items) {
		end = len(items)
	}
	page := extensionArtifactPage{Items: items[start:end], SourceCut: cut}
	if end < len(items) {
		raw, _ := json.Marshal(extensionArtifactCursor{1, extensionArtifactQuery(call), cut, end})
		token := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &token
	}
	return page
}
func extensionArtifactSigned(call extension.Invocation, s *storedArtifact) (any, error) {
	if err := extensionArtifactScope(call, artifactText(artifactPayload(s), "type_id"), "read"); err != nil {
		return nil, err
	}
	return s.signedValue(), nil
}
func extensionArtifactView(ctx context.Context, call extension.Invocation, c *artifactCatalog, id string) (artifactRecordView, error) {
	view, err := c.view(ctx, id)
	if err != nil {
		return view, err
	}
	if err = extensionArtifactScope(call, view.TypeID, "read"); err != nil {
		return artifactRecordView{}, err
	}
	for _, head := range view.Heads {
		if err = extensionArtifactScope(call, artifactText(head.Payload, "type_id"), "read"); err != nil {
			return artifactRecordView{}, err
		}
	}
	return view, nil
}
func extensionArtifactRead(ctx context.Context, call extension.Invocation) (any, error) {
	p := call.Request.Params
	if p["opaque"] == true {
		if p["cursor"] != nil {
			return nil, artifactListCheckpoint()
		}
		item, err := readArtifactOpaque(ctx, artifactText(p, "opaque_id"))
		if err != nil {
			return nil, err
		}
		// This exact one-object API never issues a continuation. Its Cut has no
		// accepted typed membership; immutable opaque_id remains the query identity.
		cut, err := extensionArtifactCut(call.Binding.RepositoryID, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		if err = extensionArtifactBinding(ctx, call); err != nil {
			return nil, err
		}
		return extensionArtifactPage{Items: []any{item}, SourceCut: cut}, nil
	}
	h, err := loadArtifactHistory(ctx)
	if err != nil {
		return nil, err
	}
	c, err := buildArtifactCatalog(ctx, h)
	if err != nil {
		return nil, err
	}
	if call.Request.Method == "artifact.search" {
		return extensionArtifactSearch(ctx, call, c)
	}
	selected := []string{}
	id, typeID := artifactText(p, "id"), artifactText(p, "type_id")
	mode := artifactText(p, "mode")
	if id != "" {
		s := h.ByID[id]
		if s == nil {
			if p["cursor"] != nil {
				return nil, artifactListCheckpoint()
			}
			return nil, artifactFailure("not_found", "artifact not found in selected accepted history")
		}
		actual := artifactText(artifactPayload(s), "type_id")
		if err = extensionArtifactScope(call, actual, "read"); err != nil {
			return nil, err
		}
		if typeID != "" && typeID != actual {
			return nil, artifactFailure("invalid_input", "artifact ID and type selector disagree")
		}
		typeID = actual
		if call.Request.Method == "artifact.type.read" {
			if s.verified.Kind() != "type.publish" {
				return nil, artifactFailure("invalid_input", "type inspection requires a type declaration")
			}
			selected = append(selected, id)
		} else if mode == "inspect" {
			selected = append(selected, id)
		} else {
			logical, e := c.logical(id)
			if e != nil {
				return nil, e
			}
			if mode == "history" {
				selected = append(selected, c.records[logical]...)
			} else {
				selected = append(selected, logical)
			}
		}
	} else {
		if err = extensionArtifactScope(call, typeID, "read"); err != nil {
			return nil, err
		}
		if call.Request.Method == "artifact.type.read" || mode == "inspect" {
			for key, s := range h.ByID {
				if artifactText(artifactPayload(s), "type_id") == typeID && (mode == "inspect" || s.verified.Kind() == "type.publish") {
					selected = append(selected, key)
				}
			}
		} else {
			for key := range c.records {
				if artifactText(artifactPayload(h.ByID[key]), "type_id") == typeID {
					selected = append(selected, key)
				}
			}
		}
	}
	sort.Strings(selected)
	cut, err := extensionArtifactProjection(ctx, call, c, []string{typeID})
	if err != nil {
		return nil, err
	}
	start, err := extensionArtifactOffset(call, cut, len(selected))
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, len(selected))
	for _, key := range selected {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact read cancelled")
		}
		var item any
		if call.Request.Method == "artifact.type.read" || mode == "inspect" || mode == "history" {
			item, err = extensionArtifactSigned(call, h.ByID[key])
		} else {
			item, err = extensionArtifactView(ctx, call, c, key)
		}
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = extensionArtifactStable(ctx, call, h); err != nil {
		return nil, err
	}
	return extensionArtifactPageAt(call, cut, items, start), nil
}

func extensionArtifactSearch(ctx context.Context, call extension.Invocation, c *artifactCatalog) (any, error) {
	p, h := call.Request.Params, c.history
	typeID, shape := artifactText(p, "type_id"), artifactText(p, "shape")
	text := artifactText(p, "literal")
	var regex *memoryGrepPattern
	if p["regex"] != nil {
		text = artifactText(p, "regex")
		compiled, err := compileArtifactSearchPattern(text)
		if err != nil {
			return nil, err
		}
		regex = &compiled
	}
	// Explicitly filter by actual read grants, type and shape before charging work.
	// Even an empty selection requires an applicable declared read grant.
	ids := []string{}
	for id := range c.records {
		payload := artifactPayload(h.ByID[id])
		actual := artifactText(payload, "type_id")
		if (typeID == "" || actual == typeID) && (shape == "" || artifactText(payload, "shape") == shape) && extensionArtifactScope(call, actual, "read") == nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	types := []string{}
	if typeID != "" {
		types = append(types, typeID)
	} else {
		for _, item := range h.ByID {
			payload := artifactPayload(item)
			actual := artifactText(payload, "type_id")
			if (shape == "" || artifactText(payload, "shape") == shape) && extensionArtifactScope(call, actual, "read") == nil {
				types = append(types, actual)
			}
		}
	}
	cut, err := extensionArtifactProjection(ctx, call, c, types)
	if err != nil {
		return nil, err
	}
	start, err := extensionArtifactOffset(call, cut, len(ids))
	if err != nil {
		return nil, err
	}
	views := make([]artifactRecordView, 0, len(ids))
	fields := make([][]string, 0, len(ids))
	used := uint64(0)
	for _, id := range ids {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact search cancelled")
		}
		view, e := extensionArtifactView(ctx, call, c, id)
		if e != nil {
			return nil, e
		}
		searchable, e := artifactSearchFields(view)
		if e != nil {
			return nil, e
		}
		if regex != nil {
			for _, field := range searchable {
				used, e = artifactSearchAddWork(used, regex.instructions, len(field))
				if e != nil {
					return nil, e
				}
			}
		}
		views = append(views, view)
		fields = append(fields, searchable)
	}
	limit := int(p["limit"].(float64))
	page := extensionArtifactPage{Items: []any{}, SourceCut: cut}
	end := start
	for end < len(ids) {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact search cancelled")
		}
		matched := artifactSearchMatches(fields[end], text, regex)
		if matched && len(page.Items) == int(limit) {
			break
		}
		if matched {
			page.Items = append(page.Items, views[end])
		}
		end++
	}
	if end < len(ids) {
		raw, _ := json.Marshal(extensionArtifactCursor{1, extensionArtifactQuery(call), cut, end})
		token := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &token
	}
	if err = extensionArtifactStable(ctx, call, h); err != nil {
		return nil, err
	}
	return page, nil
}

// Cut inputs contain only verified protocol identifiers, not arbitrary text.
// Canonical JSON here is C: sorted object keys, UTF-8 identifiers, finite arrays.
// Reusing the envelope input parser would impose its unrelated 1 MiB cap on the
// already bounded complete history fingerprint, so encode these known values.
func extensionArtifactCut(repository string, sources []artifactSource, types, ids []string) (string, error) {
	types = append([]string{}, types...)
	sort.Strings(types)
	types = slices.Compact(types)
	ids = append([]string{}, ids...)
	sort.Strings(ids)
	ids = slices.Compact(ids)
	sources = append([]artifactSource{}, sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Ref < sources[j].Ref })
	refs := []map[string]any{}
	for i, source := range sources {
		if i > 0 && source.Ref == sources[i-1].Ref {
			if source.OID != sources[i-1].OID {
				return "", artifactFailure("invalid_history", "contradictory accepted source witness")
			}
			continue
		}
		refs = append(refs, map[string]any{"ref": source.Ref, "oid": source.OID})
	}
	view, err := json.Marshal(map[string]any{"admitted_ids": ids, "visible_ids": ids, "retained_refs": []artifactSource{}})
	if err != nil {
		return "", err
	}
	cut, err := json.Marshal(map[string]any{"repository_id": repository, "refs": refs, "selection": map[string]any{"families": []string{}, "types": types}, "authority_commit": "", "policy_digest": "", "view_digest": eventID(view)})
	if err != nil {
		return "", err
	}
	return eventID(cut), nil
}

// The complete effective type projection is independent of endpoint/page/match.
// Shared history/context, catalog and diagnostics establish usable custom data;
// fingerprint membership never authorizes exporting a dependency body.
func extensionArtifactProjection(ctx context.Context, call extension.Invocation, c *artifactCatalog, types []string) (string, error) {
	selected := map[string]bool{}
	for _, typeID := range types {
		selected[typeID] = true
	}
	ids, roots := []string{}, []string{}
	for id, item := range c.history.ByID {
		if ctx.Err() != nil {
			return "", artifactFailure("cancelled", "artifact projection cancelled")
		}
		if item.verified.Kind() == "type.publish" && selected[artifactText(artifactPayload(item), "type_id")] {
			ids = append(ids, id)
		}
	}
	for logical, revisions := range c.records {
		if selected[artifactText(artifactPayload(c.history.ByID[logical]), "type_id")] {
			ids = append(ids, revisions...)
			roots = append(roots, c.heads[logical]...)
		}
	}
	if _, err := c.diagnostics(ctx, roots); err != nil {
		return "", err
	}
	return extensionArtifactCut(call.Binding.RepositoryID, c.history.Sources, types, ids)
}

// Use the existing physical repository, private registry and enablement readers.
// Exact epoch/registration/grant equality is rechecked after projection, not
// inferred from an earlier successful request or a Cut fingerprint.
func extensionArtifactBinding(ctx context.Context, call extension.Invocation) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact projection cancelled")
	}
	current, err := extensionRegisteredModule(call.Registration.Manifest.ModuleID)
	if err != nil {
		return err
	}
	store, err := extensionRepository(true)
	if err != nil {
		return err
	}
	entry, found, err := store.Entry(current)
	if err != nil {
		return err
	}
	if !found || store.RepositoryID() != call.Binding.RepositoryID || !reflect.DeepEqual(current, call.Registration) || !reflect.DeepEqual(entry, call.Enablement) {
		return frameInputError("denied_access", "artifact session binding or grants changed")
	}
	if err = extension.Authorize(current, entry, call.Binding, extension.Access{Method: "capabilities.list"}, false); err != nil {
		return err
	}
	status, _, err := current.Availability()
	if err != nil {
		return err
	}
	if status != "registered" {
		return frameInputError("unavailable_module", "registered executable is missing or changed")
	}
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact projection cancelled")
	}
	return nil
}
func extensionArtifactStable(ctx context.Context, call extension.Invocation, h *artifactHistory) error {
	after, err := artifactSources(ctx, h.gitDir)
	if err != nil {
		return err
	}
	if !slices.Equal(h.Sources, after) {
		if call.Request.Params["cursor"] != nil {
			return artifactListCheckpoint()
		}
		return artifactFailure("concurrent_update", "accepted artifact sources changed during projection")
	}
	return extensionArtifactBinding(ctx, call)
}

func extensionArtifactOperation(ctx context.Context, call extension.Invocation) (artifactOperationResult, error) {
	p := call.Request.Params
	result, err := inspectArtifactOperation(ctx, artifactText(p, "type_id"), artifactText(p, "actor"), artifactText(p, "operation"))
	if err != nil {
		return result, err
	}
	h, err := loadArtifactHistory(ctx)
	if err != nil {
		return artifactOperationResult{}, err
	}
	// Preserve the shared lookup's complete public/private-pending proof. Never
	// re-label a lookup using a later, different accepted-source observation.
	if h.Cut != result.SourceCut {
		return artifactOperationResult{}, artifactFailure("concurrent_update", "artifact operation observation changed during projection")
	}
	c, err := buildArtifactCatalog(ctx, h)
	if err != nil {
		return artifactOperationResult{}, err
	}
	cut, err := extensionArtifactProjection(ctx, call, c, []string{result.TypeID})
	if err != nil {
		return artifactOperationResult{}, err
	}
	if err = extensionArtifactStable(ctx, call, h); err != nil {
		return artifactOperationResult{}, err
	}
	result.SourceCut = cut
	if result.Result != nil {
		result.Result.SourceCut = cut
	}
	return result, nil
}
