package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSuccessorPayloadBridgeFrozen(t *testing.T) {
	contexts := fixtureContexts(t)
	seen := map[string]bool{}
	for _, name := range []string{"asset-revise-a", "asset-resolve-ab", "asset-convert-v2", "asset-resolve-mixed"} {
		m := contexts[name]
		envelope := m["artifact"].(map[string]any)["envelope"].(map[string]any)
		declaration := m["type"].(map[string]any)["envelope"].(map[string]any)["payload"]
		typ, e := ValidateType(context.Background(), contextBytes(t, declaration.(map[string]any)))
		if e != nil {
			t.Fatal(e)
		}
		parents, _ := ids(envelope["parents"])
		raw := contextBytes(t, envelope["payload"].(map[string]any))
		kind := text(envelope, "kind")
		if e = typ.ValidateSuccessor(context.Background(), kind, raw, parents); e != nil {
			t.Fatal(name, e)
		}
		seen[kind] = true
	}
	// Independent literal family mirror: this wrapper supports exactly successors.
	if len(seen) != 3 || !seen["record.revise"] || !seen["record.resolve"] || !seen["record.convert"] {
		t.Fatal(seen)
	}
}
func TestSuccessorPayloadBridgeRefusalsAndBounds(t *testing.T) {
	m := fixtureContexts(t)["asset-revise-a"]
	envelope := m["artifact"].(map[string]any)["envelope"].(map[string]any)
	declaration := m["type"].(map[string]any)["envelope"].(map[string]any)["payload"].(map[string]any)
	typ, e := ValidateType(context.Background(), contextBytes(t, declaration))
	if e != nil {
		t.Fatal(e)
	}
	raw := contextBytes(t, envelope["payload"].(map[string]any))
	var badSchema map[string]any
	if e := json.Unmarshal(raw, &badSchema); e != nil {
		t.Fatal(e)
	}
	badSchema["fields"] = map[string]any{"unrecognized": true}
	badSchemaBytes, e := json.Marshal(badSchema)
	if e != nil {
		t.Fatal(e)
	}
	parents, _ := ids(envelope["parents"])
	for _, tc := range []struct {
		name, kind, code string
		raw              []byte
		parents          []string
	}{
		{"unsupported", "record.open", "invalid_input", raw, nil},
		{"unknown", "record.future", "invalid_input", raw, parents},
		{"type publication", "type.publish", "invalid_input", raw, parents},
		{"convert empty safe", "record.convert", "invalid_input", raw, nil},
		{"revise empty", "record.revise", "invalid_input", raw, nil},
		{"resolve empty", "record.resolve", "invalid_input", raw, nil},
		{"parent syntax", "record.resolve", "invalid_input", raw, []string{"no"}},
		{"parent duplicate", "record.resolve", "invalid_input", raw, []string{parents[0], parents[0]}},
		{"duplicate JSON", "record.revise", "invalid_input", []byte(`{"fields":{},"fields":{}}`), parents},
		{"unicode", "record.revise", "invalid_input", []byte(`{"content":"\ud800"}`), parents},
		{"wrong schema values", "record.revise", "invalid_input", badSchemaBytes, parents},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := typ.ValidateSuccessor(context.Background(), tc.kind, tc.raw, tc.parents)
			errorCode(t, e, tc.code)
		})
	}
	errorCode(t, (TypeDeclaration{}).ValidateSuccessor(context.Background(), "record.revise", raw, parents), "invalid_input")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	errorCode(t, typ.ValidateSuccessor(ctx, "record.revise", raw, parents), "cancelled")
	// Parent array exact at-bound and first-over through the production bridge.
	many := make([]string, 1024)
	for i := range many {
		many[i] = fmt.Sprintf("sha256:%064x", i)
	}
	if e := typ.ValidateSuccessor(context.Background(), "record.resolve", raw, many); e != nil {
		t.Fatal("1024 parents", e)
	}
	many = append(many, fmt.Sprintf("sha256:%064x", 1024))
	errorCode(t, typ.ValidateSuccessor(context.Background(), "record.resolve", raw, many), "resource_limit")
	// Exact complete payload byte budget, without changing schema semantics.
	var p map[string]any
	_ = json.Unmarshal(raw, &p)
	p["content"] = ""
	b, _ := json.Marshal(p)
	p["content"] = strings.Repeat("x", MaxEnvelopeBytes-len(b))
	b, _ = json.Marshal(p)
	if len(b) != MaxEnvelopeBytes {
		t.Fatal(len(b))
	}
	if e := typ.ValidateSuccessor(context.Background(), "record.revise", b, parents); e != nil {
		t.Fatal("payload at bound", e)
	}
	p["content"] = p["content"].(string) + "x"
	b, _ = json.Marshal(p)
	errorCode(t, typ.ValidateSuccessor(context.Background(), "record.revise", b, parents), "resource_limit")
}

func errorCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestSuccessorPayloadBridgeCollectionBounds(t *testing.T) {
	m := fixtureContexts(t)["list-open"]
	envelope := m["artifact"].(map[string]any)["envelope"].(map[string]any)
	p := envelope["payload"].(map[string]any)
	declaration := m["type"].(map[string]any)["envelope"].(map[string]any)["payload"].(map[string]any)
	typ, e := ValidateType(context.Background(), contextBytes(t, declaration))
	if e != nil {
		t.Fatal(e)
	}
	parent := "sha256:" + strings.Repeat("a", 64)
	members := make([]any, 1024)
	for i := range members {
		members[i] = map[string]any{"member_id": fmt.Sprintf("entry-%04d", 1024-i), "target": parent, "target_mode": "revision", "required": false}
	}
	p["members"] = members
	b := contextBytes(t, p)
	if e = typ.ValidateSuccessor(context.Background(), "record.resolve", b, []string{parent}); e != nil {
		t.Fatal("1024 ordered members", e)
	}
	p["members"] = append(members, map[string]any{"member_id": "one-over", "target": parent, "target_mode": "revision", "required": false})
	errorCode(t, typ.ValidateSuccessor(context.Background(), "record.resolve", contextBytes(t, p), []string{parent}), "resource_limit")
	p["members"] = members
	relationships := make([]any, 256)
	for i := range relationships {
		relationships[i] = map[string]any{"relation": "example.test/link", "target": fmt.Sprintf("sha256:%064x", i), "target_mode": "revision", "required": false}
	}
	p["relationships"] = relationships
	if e = typ.ValidateSuccessor(context.Background(), "record.resolve", contextBytes(t, p), []string{parent}); e != nil {
		t.Fatal("256 relationships", e)
	}
	p["relationships"] = append(relationships, map[string]any{"relation": "example.test/link", "target": fmt.Sprintf("sha256:%064x", 256), "target_mode": "revision", "required": false})
	errorCode(t, typ.ValidateSuccessor(context.Background(), "record.resolve", contextBytes(t, p), []string{parent}), "resource_limit")
}
