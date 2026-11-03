package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func frameForTest(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, uint32(len(raw)))
	return append(out, raw...)
}
func helloForTest() map[string]any {
	return map[string]any{"protocol": "hn.extension/1", "hello": map[string]any{"session_id": strings.Repeat("1", 32), "repository_id": "sha256:" + strings.Repeat("a", 64), "module_id": "example.inventory/publisher", "module_version": "1", "registration_id": "sha256:" + strings.Repeat("b", 64)}, "limits": map[string]any{"frame_bytes": 8388608, "read_timeout_ms": 30000, "mutation_timeout_ms": 240000, "page_max": 200, "one_outstanding": true}}
}
func TestInventoryClientUsesIdentityAndPublicContract(t *testing.T) {
	incoming := frameForTest(t, helloForTest())
	incoming = append(incoming, frameForTest(t, map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "ok": true, "data": identityDataForTest(), "error": nil})...)
	incoming = append(incoming, frameForTest(t, map[string]any{"protocol": "hn.extension/1", "request_id": "inventory", "ok": true, "data": mutationDataForTest(), "error": nil})...)
	var wire, result bytes.Buffer
	err := run(bytes.NewReader(incoming), &wire, &result, Command{Action: "type", Model: "item", SchemaVersion: 1, Operation: "type-item", Parents: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(wire.Bytes())
	var ready map[string]any
	if err = readFrame(reader, &ready); err != nil || len(ready) != 2 || ready["ready"] != true {
		t.Fatal(ready, err)
	}
	var identity, request map[string]any
	if err = readFrame(reader, &identity); err != nil || identity["method"] != "identity.public" {
		t.Fatal(identity, err)
	}
	if err = readFrame(reader, &request); err != nil || request["method"] != "artifact.type.publish" {
		t.Fatal(request, err)
	}
	params := request["params"].(map[string]any)
	payload := params["payload"].(map[string]any)
	if params["actor"] != identityDataForTest()["actor"] || params["operation"] != "type-item" || payload["type_id"] != "example.inventory/item" || payload["shape"] != "document" {
		t.Fatal(request)
	}
	if _, err = reader.ReadByte(); err != io.EOF {
		t.Fatal("extra request", err)
	}
	if !strings.Contains(result.String(), `"event_id"`) {
		t.Fatal(result.String())
	}
}
func TestInventoryRejectsInvalidHelloBeforeReady(t *testing.T) {
	for _, change := range []func(map[string]any){func(h map[string]any) { h["protocol"] = "wrong" }, func(h map[string]any) { h["hello"].(map[string]any)["module_id"] = "other/module" }, func(h map[string]any) { h["hello"].(map[string]any)["repository_id"] = "short" }, func(h map[string]any) { h["limits"].(map[string]any)["one_outstanding"] = false }, func(h map[string]any) { h["extra"] = true }} {
		h := helloForTest()
		change(h)
		var wire, result bytes.Buffer
		if err := run(bytes.NewReader(frameForTest(t, h)), &wire, &result, Command{Action: "type", Model: "item", Operation: "type", SchemaVersion: 1}); err == nil || wire.Len() != 0 {
			t.Fatal("invalid hello accepted", h, err)
		}
	}
}
func TestInventoryFramingBoundsAndStrictJSON(t *testing.T) {
	for _, size := range []uint32{0, 8388609} {
		raw := make([]byte, 4)
		binary.BigEndian.PutUint32(raw, size)
		var out map[string]any
		if readFrame(bytes.NewReader(raw), &out) == nil {
			t.Fatal("frame bound", size)
		}
	}
	// Frozen transport boundary: exact 8MiB JSON object accepted, one byte over refused.
	raw := []byte(`{"value":"` + strings.Repeat("x", 8388608-len(`{"value":""}`)) + `"}`)
	prefix := make([]byte, 4)
	binary.BigEndian.PutUint32(prefix, uint32(len(raw)))
	var out map[string]any
	if err := readFrame(bytes.NewReader(append(prefix, raw...)), &out); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"x":1,"x":2}`, `{"x":1} {}`, `{"x":"\ud800"}`, "{\"x\":\"\xff\"}", `[]`} {
		prefix := make([]byte, 4)
		binary.BigEndian.PutUint32(prefix, uint32(len(raw)))
		if readFrame(bytes.NewReader(append(prefix, []byte(raw)...)), &out) == nil {
			t.Fatalf("accepted invalid JSON %q", raw)
		}
	}
}

func TestInventoryCommandsStayWithinExampleModels(t *testing.T) {
	for _, model := range []string{"item", "stock", "maintenance"} {
		for _, action := range []string{"type", "open", "revise", "resolve", "convert"} {
			c := Command{Model: model, Action: action, SchemaVersion: 1}
			method, params, err := c.request(strings.Repeat("c", 64))
			if err != nil {
				t.Fatal(err)
			}
			if method == "" || params == nil {
				t.Fatal("missing command", model, action)
			}
		}
	}
	for _, c := range []Command{{Model: "other", Action: "type", SchemaVersion: 1}, {Model: "item", Action: "activate"}, {Model: "item", Action: "inspect"}, {Model: "item", Action: "search"}, {Model: "item", Action: "type", SchemaVersion: 3}} {
		if _, _, err := c.request(strings.Repeat("c", 64)); err == nil {
			t.Fatal("unsupported local command accepted", c)
		}
	}
	for _, raw := range []string{`{"action":"type","model":"item","schema_version":1,"repository_id":"forged"}`, `{"action":"type","action":"open"}`} {
		var c Command
		if decodeObject([]byte(raw), &c) == nil {
			t.Fatal("unknown/duplicate command accepted", raw)
		}
	}
}

func identityDataForTest() map[string]any {
	public := make([]byte, 32)
	sum := sha256.Sum256(public)
	return map[string]any{"initialized": true, "actor": hex.EncodeToString(sum[:]), "name": "A", "public_key": base64.RawStdEncoding.EncodeToString(public)}
}
func mutationDataForTest() map[string]any {
	return map[string]any{"protocol": "hn-artifact/1", "family": nil, "type_id": "example.inventory/item", "resource_id": "sha256:" + strings.Repeat("d", 64), "event_id": "sha256:" + strings.Repeat("d", 64), "replayed": false, "conflict": false, "head_count": 1, "admission": nil, "source_cut": "sha256:" + strings.Repeat("e", 64)}
}
func responseForTest(id string, data any) map[string]any {
	return map[string]any{"protocol": "hn.extension/1", "request_id": id, "ok": true, "data": data, "error": nil}
}
func sentMethodsForTest(t *testing.T, raw []byte) []string {
	t.Helper()
	reader := bytes.NewReader(raw)
	var ready map[string]any
	if err := readFrame(reader, &ready); err != nil || len(ready) != 2 || ready["ready"] != true {
		t.Fatal("ready", ready, err)
	}
	methods := []string{}
	for reader.Len() > 0 {
		var request map[string]any
		if err := readFrame(reader, &request); err != nil {
			t.Fatal(err)
		}
		method, ok := request["method"].(string)
		if !ok {
			t.Fatal(request)
		}
		methods = append(methods, method)
	}
	return methods
}
func TestInventoryTypedIdentityFaultsStopBeforePublication(t *testing.T) {
	faults := map[string]func(map[string]any){
		"missing-ok": func(v map[string]any) { delete(v, "ok") }, "null-ok": func(v map[string]any) { v["ok"] = nil }, "wrong-ok-type": func(v map[string]any) { v["ok"] = "true" },
		"missing-id": func(v map[string]any) { delete(v, "request_id") }, "wrong-correlation": func(v map[string]any) { v["request_id"] = "other" }, "numeric-id": func(v map[string]any) { v["request_id"] = 1 },
		"case-envelope": func(v map[string]any) { delete(v, "protocol"); v["Protocol"] = "hn.extension/1" }, "unknown-envelope": func(v map[string]any) { v["authority"] = true },
		"null-success": func(v map[string]any) { v["data"] = nil }, "success-with-error": func(v map[string]any) { v["error"] = map[string]any{} }, "terminal-success": func(v map[string]any) { v["request_id"] = nil },
		"missing-identity-field": func(v map[string]any) { delete(v["data"].(map[string]any), "initialized") }, "null-initialized": func(v map[string]any) { v["data"].(map[string]any)["initialized"] = nil },
		"wrong-name": func(v map[string]any) { v["data"].(map[string]any)["name"] = false }, "bad-key": func(v map[string]any) { v["data"].(map[string]any)["public_key"] = "public" },
		"mismatched-key": func(v map[string]any) { v["data"].(map[string]any)["actor"] = strings.Repeat("c", 64) }, "extra-identity-field": func(v map[string]any) { v["data"].(map[string]any)["policy"] = true },
		"false-with-fields": func(v map[string]any) { v["data"].(map[string]any)["initialized"] = false },
	}
	for name, change := range faults {
		t.Run(name, func(t *testing.T) {
			v := responseForTest("identity", identityDataForTest())
			change(v)
			incoming := append(frameForTest(t, helloForTest()), frameForTest(t, v)...)
			incoming = append(incoming, frameForTest(t, responseForTest("inventory", mutationDataForTest()))...)
			var wire, result bytes.Buffer
			if err := run(bytes.NewReader(incoming), &wire, &result, Command{Action: "type", Model: "item", SchemaVersion: 1, Operation: "never-publish"}); err == nil {
				t.Fatal("accepted malformed identity response", v)
			}
			if !reflect.DeepEqual(sentMethodsForTest(t, wire.Bytes()), []string{"identity.public"}) || result.Len() != 0 {
				t.Fatal("malformed response gained a publication/output", wire.String(), result.String())
			}
		})
	}
}
func TestInventoryTypedMutationFaultsDoNotBecomeResults(t *testing.T) {
	for name, change := range map[string]func(map[string]any){"missing-field": func(d map[string]any) { delete(d, "replayed") }, "null-boolean": func(d map[string]any) { d["conflict"] = nil }, "wrong-count": func(d map[string]any) { d["head_count"] = "1" }, "negative-count": func(d map[string]any) { d["head_count"] = -1 }, "foreign-protocol": func(d map[string]any) { d["protocol"] = "hn/0" }, "foreign-type": func(d map[string]any) { d["type_id"] = "other/item" }, "bad-id": func(d map[string]any) { d["event_id"] = "short" }, "wrong-family": func(d map[string]any) { d["family"] = "issue" }, "wrong-admission": func(d map[string]any) { d["admission"] = "admitted" }, "extra": func(d map[string]any) { d["private"] = true }, "inconsistent-heads": func(d map[string]any) { d["head_count"] = 2 }} {
		t.Run(name, func(t *testing.T) {
			data := mutationDataForTest()
			change(data)
			incoming := append(frameForTest(t, helloForTest()), frameForTest(t, responseForTest("identity", identityDataForTest()))...)
			incoming = append(incoming, frameForTest(t, responseForTest("inventory", data))...)
			var wire, result bytes.Buffer
			if err := run(bytes.NewReader(incoming), &wire, &result, Command{Action: "type", Model: "item", SchemaVersion: 1, Operation: "type"}); err == nil || result.Len() != 0 {
				t.Fatal("malformed mutation accepted", data, err)
			}
			if !reflect.DeepEqual(sentMethodsForTest(t, wire.Bytes()), []string{"identity.public", "artifact.type.publish"}) {
				t.Fatal("extra request after malformed response")
			}
		})
	}
}
func TestInventoryDefinitiveAndTerminalErrorsPreserveOriginalValues(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		tuple := map[string]any{"protocol": "hn-artifact/1", "actor": identityDataForTest()["actor"], "operation": "original-immutable-intent"}
		failure := map[string]any{"origin": "artifact", "code": "uncertain_operation", "message": "inspect original operation", "recovery": "inspect_operation", "operation": tuple}
		v := map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "ok": false, "data": nil, "error": failure}
		if terminal {
			v["request_id"] = nil
		}
		incoming := append(frameForTest(t, helloForTest()), frameForTest(t, v)...)
		var wire, result bytes.Buffer
		if err := run(bytes.NewReader(incoming), &wire, &result, Command{Action: "type", Model: "item", SchemaVersion: 1, Operation: "never-publish"}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(sentMethodsForTest(t, wire.Bytes()), []string{"identity.public"}) {
			t.Fatal("error response did not stop session")
		}
		var got map[string]any
		if json.Unmarshal(result.Bytes(), &got) != nil || !reflect.DeepEqual(got["error"], failure) || terminal && got["request_id"] != nil {
			t.Fatal("original failure/tuple changed", result.String())
		}
	}
}
func TestInventoryResponseErrorInventoriesAndRefusals(t *testing.T) {
	expected := map[string][]string{
		"host":     {"denied_access", "incompatible_version", "unavailable_module", "unsupported_capability", "resource_limit", "cancelled", "uncertain_operation", "checkpoint_invalidated", "verification_failure", "invalid_input", "not_found", "repository_error"},
		"native":   {"actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision"},
		"artifact": {"invalid_input", "resource_limit", "verification_failure", "invalid_history", "incomplete_dependency", "unsupported_schema", "actor_stream_conflict", "operation_conflict", "stale_revision", "resource_conflict", "concurrent_update", "not_found", "repository_error", "cancelled", "uncertain_operation"},
	}
	if len(responseErrorCodes) != len(expected) {
		t.Fatal("origin inventory changed")
	}
	for origin, want := range expected {
		got := append([]string{}, responseErrorCodes[origin]...)
		want = append([]string{}, want...)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("closed error inventory drift", origin, got, want)
		}
	}
	for name, change := range map[string]func(map[string]any){"unknown-origin": func(v map[string]any) { v["origin"] = "invented" }, "unknown-code": func(v map[string]any) { v["code"] = "invented" }, "unknown-recovery": func(v map[string]any) { v["recovery"] = "retry_changed" }, "missing": func(v map[string]any) { delete(v, "operation") }, "extra": func(v map[string]any) { v["authority"] = true }, "wrong-message": func(v map[string]any) { v["message"] = false }, "oversize-message": func(v map[string]any) { v["message"] = strings.Repeat("x", 4097) }, "wrong-tuple": func(v map[string]any) {
		v["operation"] = map[string]any{"protocol": "other/1", "actor": strings.Repeat("c", 64), "operation": "original"}
	}} {
		t.Run(name, func(t *testing.T) {
			failure := map[string]any{"origin": "host", "code": "denied_access", "message": "action is not granted", "recovery": "do_not_retry", "operation": nil}
			change(failure)
			v := map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "ok": false, "data": nil, "error": failure}
			raw, _ := json.Marshal(v)
			var parsed map[string]any
			if err := decodeObject(raw, &parsed); err != nil {
				t.Fatal(err)
			}
			if _, err := checkedResponse(parsed, "identity", "identity.public", map[string]any{}); err == nil {
				t.Fatal("malformed error accepted", v)
			}
		})
	}
}

func TestInventoryPartialAndUninitializedIdentityCannotPublish(t *testing.T) {
	complete := frameForTest(t, responseForTest("identity", identityDataForTest()))
	inputs := [][]byte{complete[:3], complete[:len(complete)-1], frameForTest(t, responseForTest("identity", map[string]any{"initialized": false, "actor": nil, "name": nil, "public_key": nil}))}
	for _, response := range inputs {
		incoming := append(frameForTest(t, helloForTest()), response...)
		var wire, result bytes.Buffer
		if err := run(bytes.NewReader(incoming), &wire, &result, Command{Action: "type", Model: "item", SchemaVersion: 1, Operation: "never-publish"}); err == nil || result.Len() != 0 {
			t.Fatal("incomplete/uninitialized identity accepted", err)
		}
		if !reflect.DeepEqual(sentMethodsForTest(t, wire.Bytes()), []string{"identity.public"}) {
			t.Fatal("publication before complete initialized identity")
		}
	}
}
func TestInventoryErrorValueBounds(t *testing.T) {
	for _, origin := range []string{"host", "native", "artifact"} {
		for _, code := range responseErrorCodes[origin] {
			if !validError(map[string]any{"origin": origin, "code": code, "message": "literal allowed code", "recovery": "do_not_retry", "operation": nil}) {
				t.Fatal("closed literal error rejected", origin, code)
			}
		}
	}
	tuple := map[string]any{"protocol": "hn-artifact/1", "actor": strings.Repeat("c", 64), "operation": strings.Repeat("k", 128)}
	failure := map[string]any{"origin": "artifact", "code": "uncertain_operation", "message": strings.Repeat("m", 4096), "recovery": "inspect_operation", "operation": tuple}
	if !validError(failure) {
		t.Fatal("at-bound error rejected")
	}
	failure["message"] = strings.Repeat("m", 4097)
	if validError(failure) {
		t.Fatal("oversize message accepted")
	}
	failure["message"] = "safe"
	tuple["operation"] = strings.Repeat("k", 129)
	if validError(failure) {
		t.Fatal("oversize operation accepted")
	}
	tuple["operation"] = "original"
	tuple["extra"] = true
	if validError(failure) {
		t.Fatal("extra tuple authority accepted")
	}
}
