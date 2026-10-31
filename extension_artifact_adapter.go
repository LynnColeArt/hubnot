package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
		data, err = inspectArtifactOperation(ctx, artifactText(p, "type_id"), artifactText(p, "actor"), artifactText(p, "operation"))
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
	if kind == "record.open" || kind == "type.publish" {
		return writeArtifact(ctx, kind, actor, operation, raw)
	}
	return writeArtifactSuccessor(ctx, kind, actor, operation, raw)
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
		item, err := readArtifactOpaque(ctx, artifactText(p, "opaque_id"))
		if err != nil {
			return nil, err
		}
		// Opaque identity is a public quarantine identity, never an accepted ID/cut.
		repository, _, err := artifactRepositoryID()
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(map[string]any{"repository_id": repository, "opaque_id": p["opaque_id"]})
		cut := eventID(raw)
		start, err := extensionArtifactOffset(call, cut, 1)
		if err != nil {
			return nil, err
		}
		return extensionArtifactPageAt(call, cut, []any{item}, start), nil
	}
	h, err := loadArtifactHistory(ctx)
	if err != nil {
		return nil, err
	}
	if p["cursor"] != nil {
		if _, err = extensionArtifactOffset(call, h.Cut, artifactReadEntries+1); err != nil {
			return nil, err
		}
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
	start, err := extensionArtifactOffset(call, h.Cut, len(selected))
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
	return extensionArtifactPageAt(call, h.Cut, items, start), nil
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
	start, err := extensionArtifactOffset(call, h.Cut, len(ids))
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
	page := extensionArtifactPage{Items: []any{}, SourceCut: h.Cut}
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
		raw, _ := json.Marshal(extensionArtifactCursor{1, extensionArtifactQuery(call), h.Cut, end})
		token := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &token
	}
	return page, nil
}
