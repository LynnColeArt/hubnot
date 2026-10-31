package extension

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// An invalid pattern makes compilation observable without depending on compiler
// allocation sizes. Creating an unused optional rule must not compile it.
func TestProtocolPatternCompilationIsDeferred(t *testing.T) {
	var rule wireRule
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("unused protocol rule compiled its pattern: %v", p)
			}
		}()
		rule = matches("[")
	}()
	defer func() {
		if recover() == nil {
			t.Fatal("first use did not compile the invalid pattern")
		}
	}()
	_ = rule("first use")
}

// Run this test alone in a fresh race-enabled test process so these globals
// receive their first use concurrently, before any typed fixture warms them.
func TestProtocolPatternsConcurrentFirstUse(t *testing.T) {
	cases := []struct {
		name string
		rule wireRule
		good string
		bad  []any
	}{
		{"id", idRule, testID, []any{strings.TrimSuffix(testID, "a"), testID + "a", strings.ToUpper(testID), "\n" + testID, nil}},
		{"actor", actorRule, testActor, []any{testActor[:63], testActor + "a", strings.ToUpper(testActor), testActor + "\n", nil}},
		{"session", sessionIDRule, strings.Repeat("a", 32), []any{strings.Repeat("a", 31), strings.Repeat("a", 33), strings.Repeat("A", 32), nil}},
		{"request", requestIDRule, strings.Repeat("~", 128), []any{"", strings.Repeat("a", 129), "space here", "é", nil}},
		{"version", moduleVersionRule, strings.Repeat("!", 64), []any{"", strings.Repeat("a", 65), "space here", "é", nil}},
		{"decimal", decimalRule, "-" + strings.Repeat("9", 61) + ".1", []any{"-0", "01", "1.0", "+1", "1e2", strings.Repeat("9", 65), "1\n", nil}},
		{"native identifier", nativeIdentifier, strings.Repeat("a", 64), []any{"", ".", "..", strings.Repeat("a", 65), "a/b", "a\n", nil}},
		{"field", fieldNameRule, "_" + strings.Repeat("a", 63), []any{"", "1a", strings.Repeat("a", 65), "a/b", "a\n", nil}},
		{"member", matches(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`), "a" + strings.Repeat("-", 127), []any{"", "_a", strings.Repeat("a", 129), "a/b", "a\n", nil}},
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 32 {
		for _, tc := range cases {
			workers.Go(func() {
				<-start
				for range 8 {
					if err := tc.rule(tc.good); err != nil {
						t.Errorf("%s valid value: %v", tc.name, err)
					}
					for _, bad := range tc.bad {
						requireCode(t, tc.rule(bad), "invalid_input")
					}
				}
			})
		}
	}
	close(start)
	workers.Wait()
}

const testID = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testActor = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func obj(pairs ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return m
}
func encode(v any) []byte { b, _ := json.Marshal(v); return b }
func recordFixture() map[string]any {
	return obj("shape", "document", "type_id", "example.test/item", "type_revision", testID, "title", "item", "content", "", "fields", obj(), "members", []any{}, "relationships", []any{}, "conversion_sources", []any{})
}
func typeFixture() map[string]any {
	return obj("type_id", "example.test/item", "schema_version", 1, "shape", "document", "schema", obj("language", "hn.schema/1", "root", obj("type", "object", "properties", obj(), "required", []any{}, "additionalProperties", false)))
}
func nativeFixture(family string) map[string]any {
	switch family {
	case "issue":
		return obj("title", "ticket", "body", "", "status", "open", "criteria", []any{}, "labels", []any{}, "assignees", []any{}, "relations", []any{}, "metadata", obj())
	case "project":
		return obj("schema", "hn.project/1", "title", "project", "body", "", "archived", false, "fields", []any{obj("id", "status", "name", "Status", "type", "single_select", "options", []any{obj("id", "open", "name", "Open")})}, "views", []any{obj("id", "board", "name", "Board", "layout", "board", "group_by_field", "status")})
	case "project-item":
		return obj("schema", "hn.project-item/1", "project_id", testID, "project_revision", testID, "target", obj("kind", "issue", "id", testID), "archived", false, "position", "0", "values", obj("status", obj("type", "single_select", "option_id", "open")))
	case "discussion":
		return obj("schema", "hn.discussion/1", "title", "discussion", "body", "", "category", "general", "status", "open")
	case "discussion-reply":
		return obj("schema", "hn.discussion-reply/1", "discussion_id", testID, "body", "reply")
	default:
		return obj("schema", "hn.workflow/1", "name", "check", "title", "check", "description", "", "enabled", true, "pipeline", obj("version", "hn.pipeline/0", "steps", []any{obj("name", "check", "command", "go", "args", []any{"test", "./..."}, "timeoutSeconds", 30)}), "source_documents", []any{obj("format", "text", "content", "description", "digest", testID)})
	}
}
func paramFixtures() map[string]map[string]any {
	selection := obj("families", []any{"issue"}, "types", []any{})
	return map[string]map[string]any{
		"capabilities.list": obj(), "identity.public": obj(),
		"resource.read":         obj("family", "issue", "id", nil, "limit", 20, "cursor", nil),
		"resource.mutate":       obj("family", "issue", "intent", "open", "id", nil, "expected", []any{}, "actor", testActor, "operation", "key", "state", nativeFixture("issue"), "comment", ""),
		"resource.operation":    obj("family", "issue", "actor", testActor, "operation", "key"),
		"artifact.read":         obj("id", testID, "opaque_id", nil, "type_id", nil, "limit", 20, "cursor", nil, "mode", "view", "opaque", false),
		"artifact.mutate":       obj("kind", "record.open", "record", nil, "parents", []any{}, "payload", recordFixture(), "actor", testActor, "operation", "key"),
		"artifact.operation":    obj("type_id", "example.test/item", "actor", testActor, "operation", "key"),
		"artifact.search":       obj("type_id", nil, "shape", nil, "literal", "item", "regex", nil, "limit", 20, "cursor", nil),
		"artifact.type.read":    obj("id", testID, "type_id", nil, "limit", 20, "cursor", nil),
		"artifact.type.publish": obj("parents", []any{}, "payload", typeFixture(), "actor", testActor, "operation", "key"),
		"changes.capture":       obj("selection", selection, "limit", 20),
		"changes.page":          obj("capture", "capture", "cursor", "cursor", "limit", 20),
		"changes.since":         obj("checkpoint", "checkpoint", "selection", selection, "limit", 20),
	}
}
func mutationFixture() map[string]any {
	return obj("protocol", "hn/0", "family", "issue", "type_id", nil, "resource_id", testID, "event_id", testID, "replayed", false, "conflict", false, "head_count", 1, "admission", nil, "source_cut", testID)
}

func TestProtocolMutationVisibleHeadCount(t *testing.T) {
	for _, method := range []string{"resource.mutate", "resource.operation", "artifact.mutate", "artifact.operation", "artifact.type.publish"} {
		t.Run(method, func(t *testing.T) {
			inspect := func(result map[string]any) error {
				data := result
				if strings.HasSuffix(method, ".operation") {
					data = obj("status", "found", "protocol", result["protocol"], "actor", testActor, "operation", "key", "family", result["family"], "type_id", result["type_id"], "request", testID, "resource_id", testID, "event_id", testID, "result", result, "source_cut", testID, "reason", nil)
				}
				response := obj("protocol", "hn.extension/1", "request_id", "r", "ok", true, "data", data, "error", nil)
				_, err := InspectMessage(encode(response), "response", "r", method)
				return err
			}
			result := mutationFixture()
			if strings.HasPrefix(method, "resource.") {
				// Pending and hidden admission can leave the governed native view
				// with no visible heads, including in a recovered acknowledgement.
				for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"} {
					result["family"] = family
					for _, admission := range []string{"pending", "hidden"} {
						result["head_count"], result["admission"] = 0, admission
						if err := inspect(result); err != nil {
							t.Errorf("%s %s with zero visible heads: %v", family, admission, err)
						}
					}
				}
				result["family"], result["admission"] = "issue", "admitted"
			} else {
				result["protocol"], result["family"], result["type_id"] = "hn-artifact/1", nil, "example.test/item"
			}
			// Native visible mutations and custom publication retain their actual
			// one-head result; the exact finite JSON integer upper bound is unchanged.
			for _, count := range []any{1, json.Number("9007199254740991")} {
				result["head_count"] = count
				if err := inspect(result); err != nil {
					t.Errorf("valid head_count %v: %v", count, err)
				}
			}
			for _, count := range []any{-1, json.Number("0.5"), json.Number("9007199254740992"), nil, "0", true} {
				result["head_count"] = count
				requireCode(t, inspect(result), "invalid_input")
			}
			delete(result, "head_count")
			requireCode(t, inspect(result), "invalid_input")
			result["head_count"] = 0
			for key, bad := range map[string]any{"protocol": "invented", "family": "invented", "type_id": "invalid", "admission": "invented", "resource_id": nil, "event_id": nil, "source_cut": nil, "replayed": nil, "conflict": nil, "extra": true} {
				original, present := result[key]
				result[key] = bad
				requireCode(t, inspect(result), "invalid_input")
				if present {
					result[key] = original
				} else {
					delete(result, key)
				}
			}
			if strings.HasPrefix(method, "resource.") {
				result["family"] = nil
				requireCode(t, inspect(result), "invalid_input")
				result["family"], result["type_id"] = "issue", "example.test/item"
				requireCode(t, inspect(result), "invalid_input")
			} else {
				result["admission"] = "pending"
				requireCode(t, inspect(result), "invalid_input")
				result["admission"], result["type_id"] = nil, nil
				requireCode(t, inspect(result), "invalid_input")
				result["type_id"], result["family"] = "example.test/item", "issue"
				requireCode(t, inspect(result), "invalid_input")
			}
		})
	}
}

func resultFixtures() map[string]map[string]any {
	page := obj("items", []any{}, "source_cut", testID, "next_cursor", nil)
	change := obj("capture", "capture", "snapshot_id", testID, "selection", obj("families", []any{"issue"}, "types", []any{}), "selection_digest", testID, "source_cut", testID, "mode", "replace", "items", []any{}, "next_cursor", nil, "complete", true, "checkpoint", "checkpoint", "expires_at", "2026-10-30T12:00:00Z")
	capabilities := []any{}
	for _, method := range []string{"capabilities.list", "identity.public", "resource.read", "resource.mutate", "resource.operation", "artifact.read", "artifact.mutate", "artifact.operation", "artifact.search", "artifact.type.read", "artifact.type.publish", "changes.capture", "changes.page", "changes.since"} {
		capabilities = append(capabilities, obj("method", method, "declared", true, "granted", false, "enabled", true, "compatible", true, "availability", "observed", "recovery", "do_not_retry"))
	}
	unsupported := []any{}
	for _, surface := range strings.Fields("memory.mutate proposal review decide merge policy.mutate identity.mutate run runner audit automerge sync.apply migration.apply") {
		unsupported = append(unsupported, obj("surface", surface, "status", "unsupported", "recovery", "do_not_retry"))
	}
	return map[string]map[string]any{
		"capabilities.list": obj("session", obj("session_id", strings.Repeat("a", 32), "module_id", "example.test/tool", "module_version", "1", "registration_id", testID, "repository_id", testID, "protocol", "hn.extension/1"), "capabilities", capabilities, "unsupported", unsupported),
		"identity.public":   obj("initialized", false, "actor", nil, "name", nil, "public_key", nil),
		"resource.read":     page, "artifact.read": page, "artifact.search": page, "artifact.type.read": page,
		"resource.mutate": mutationFixture(), "artifact.mutate": mutationFixture(), "artifact.type.publish": mutationFixture(),
		"resource.operation": obj("status", "found", "protocol", "hn/0", "actor", testActor, "operation", "key", "family", "issue", "type_id", nil, "request", testID, "resource_id", testID, "event_id", testID, "result", mutationFixture(), "source_cut", testID, "reason", nil),
		"artifact.operation": obj("status", "verified_absent", "protocol", "hn-artifact/1", "actor", testActor, "operation", "key", "family", nil, "type_id", "example.test/item", "request", nil, "resource_id", nil, "event_id", nil, "result", nil, "source_cut", testID, "reason", nil),
		"changes.capture":    change, "changes.page": change, "changes.since": change,
	}
}

func changeRevisionFixture(count int) (map[string]any, []any) {
	ids := make([]any, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("sha256:%064x", i+1)
	}
	comments := make([]any, 0, count-1)
	for _, id := range ids[1:] {
		comments = append(comments, obj("id", id, "actor", testActor, "body", "comment"))
	}
	state := nativeFixture("issue")
	view := obj("protocol", "hn/0", "family", "issue", "id", ids[0], "creator", testActor, "heads", []any{obj("id", ids[0], "actor", testActor, "state", state)}, "conflict", false, "state", state, "comments", comments)
	entry := obj("protocol", "hn/0", "family", "issue", "type_id", nil, "resource_id", ids[0], "revision_ids", ids, "value", view)
	data := resultFixtures()["changes.capture"]
	data["items"] = []any{entry}
	return obj("protocol", "hn.extension/1", "request_id", "r", "ok", true, "data", data, "error", nil), ids
}

func TestProtocolChangeRevisionIdentities(t *testing.T) {
	for _, method := range []string{"changes.capture", "changes.page", "changes.since"} {
		t.Run(method, func(t *testing.T) {
			for _, count := range []int{1024, 1025} {
				t.Run(fmt.Sprint(count), func(t *testing.T) {
					response, ids := changeRevisionFixture(count)
					raw := encode(response)
					if len(raw) >= FrameLimit {
						t.Fatal("revision fixture must fit the published frame bound")
					}
					if _, err := InspectMessage(raw, "response", "r", method); err != nil {
						t.Fatalf("complete issue view with %d revision identities (%d bytes): %v", count, len(raw), err)
					}
					last := ids[count-1]
					for control, bad := range map[string]any{"malformed": "sha256:invalid", "unsorted": "sha256:" + strings.Repeat("0", 64), "duplicate": ids[count-2], "wrong_type": nil} {
						t.Run(control, func(t *testing.T) {
							ids[count-1] = bad
							_, err := InspectMessage(encode(response), "response", "r", method)
							requireCode(t, err, "invalid_input")
							ids[count-1] = last
						})
					}
				})
			}
		})
	}
	// Complete change responses retain the same byte bound even when their
	// revision list legitimately exceeds the artifact parent-array limit.
	t.Run("frame_bound", func(t *testing.T) {
		response, _ := changeRevisionFixture(1025)
		raw := encode(response)
		raw = append(raw, bytes.Repeat([]byte(" "), FrameLimit-len(raw))...)
		if _, err := InspectMessage(raw, "response", "r", "changes.capture"); err != nil {
			t.Fatal(err)
		}
		_, err := InspectMessage(append(raw, ' '), "response", "r", "changes.capture")
		requireCode(t, err, "resource_limit")
	})
}

func TestProtocolArtifactIDArrayBoundsUnchanged(t *testing.T) {
	for _, method := range []string{"artifact.mutate", "artifact.type.publish"} {
		t.Run(method, func(t *testing.T) {
			for _, count := range []int{1024, 1025} {
				_, ids := changeRevisionFixture(count)
				params := paramFixtures()[method]
				params["parents"] = ids
				request := obj("protocol", "hn.extension/1", "request_id", "r", "method", method, "params", params)
				_, err := InspectMessage(encode(request), "request", "", "")
				if count == 1024 {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					requireCode(t, err, "resource_limit")
				}
			}
		})
	}
}

func TestProtocolAllTypedMethods(t *testing.T) {
	for method, params := range paramFixtures() {
		t.Run(method, func(t *testing.T) {
			request := obj("protocol", "hn.extension/1", "request_id", "r1", "method", method, "params", params)
			got, err := InspectMessage(encode(request), "request", "", "")
			if err != nil || got.Method == nil || *got.Method != method {
				t.Fatalf("request: %v", err)
			}
			params["repository"] = "private input must never be echoed"
			_, err = InspectMessage(encode(request), "request", "", "")
			requireCode(t, err, "invalid_input")
			delete(params, "repository")
			for name, original := range params {
				params[name] = []any{}
				if _, alreadyArray := original.([]any); !alreadyArray {
					_, err = InspectMessage(encode(request), "request", "", "")
					requireCode(t, err, "invalid_input")
				}
				params[name] = original
			}
		})
	}
	for method, data := range resultFixtures() {
		t.Run(method+" response", func(t *testing.T) {
			response := obj("protocol", "hn.extension/1", "request_id", "r1", "ok", true, "data", data, "error", nil)
			_, err := InspectMessage(encode(response), "response", "r1", method)
			if err != nil {
				t.Fatal(err)
			}
			data["unexpected"] = true
			_, err = InspectMessage(encode(response), "response", "r1", method)
			requireCode(t, err, "invalid_input")
			delete(data, "unexpected")
			_, err = InspectMessage(encode(response), "response", "wrong", method)
			requireCode(t, err, "invalid_input")
		})
	}
}
func TestProtocolNestedTypedShapes(t *testing.T) {
	for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"} {
		t.Run(family, func(t *testing.T) {
			params := paramFixtures()["resource.mutate"]
			params["family"] = family
			params["state"] = nativeFixture(family)
			request := obj("protocol", "hn.extension/1", "request_id", "r", "method", "resource.mutate", "params", params)
			if _, err := InspectMessage(encode(request), "request", "", ""); err != nil {
				t.Fatal(err)
			}
			state := params["state"].(map[string]any)
			for key, original := range state {
				state[key] = nil
				_, err := InspectMessage(encode(request), "request", "", "")
				requireCode(t, err, "invalid_input")
				state[key] = original
			}
			state["extra"] = false
			_, err := InspectMessage(encode(request), "request", "", "")
			requireCode(t, err, "invalid_input")
		})
	}
	envelope := obj("protocol", "hn-artifact/1", "kind", "record.open", "actor", testActor, "public_key", base64.RawStdEncoding.EncodeToString(make([]byte, 32)), "stream", testID, "sequence", 1, "previous", "", "timestamp", "2026-10-30T12:00:00Z", "operation", "key", "request", testID, "record", "", "parents", []any{}, "payload", recordFixture())
	signed := obj("id", testID, "envelope", envelope, "signature", base64.RawStdEncoding.EncodeToString(make([]byte, 64)), "validation", "verified", "diagnostics", []any{})
	response := obj("protocol", "hn.extension/1", "request_id", "r", "ok", true, "data", obj("items", []any{signed}, "source_cut", testID, "next_cursor", nil), "error", nil)
	if _, err := InspectMessage(encode(response), "response", "r", "artifact.type.read"); err != nil {
		t.Fatal(err)
	}
	envelope["payload"].(map[string]any)["members"] = nil
	_, err := InspectMessage(encode(response), "response", "r", "artifact.type.read")
	requireCode(t, err, "invalid_input")
}
func TestProtocolStrictJSONAndCorrelation(t *testing.T) {
	ready := `{"protocol":"hn.extension/1","ready":true}`
	for _, raw := range []string{`null`, `[]`, `true`, `{}`, `{"protocol":null,"ready":true}`, `{"protocol":"hn.extension/1","ready":null}`, `{"protocol":"hn.extension/1","ready":true,"ready":true}`, `{"protocol":"hn.extension/1","ready":true,"x":"\ud800"}`, `{"protocol":"hn.extension/1","ready":true} {}`, strings.Replace(ready, "true", "1e0", 1)} {
		_, err := InspectMessage([]byte(raw), "ready", "", "")
		requireCode(t, err, "invalid_input")
	}
	_, err := InspectMessage(append([]byte(ready[:len(ready)-1]), 255, '}'), "ready", "", "")
	requireCode(t, err, "invalid_input")
	_, err = InspectMessage([]byte(strings.Replace(ready, "hn.extension/1", "hn.extension/2", 1)), "ready", "", "")
	requireCode(t, err, "incompatible_version")
	for _, id := range []string{"!", strings.Repeat("z", 128)} {
		request := obj("protocol", "hn.extension/1", "request_id", id, "method", "identity.public", "params", obj())
		if _, err := InspectMessage(encode(request), "request", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []any{"", strings.Repeat("z", 129), "a b", "é", nil, []any{}, true} {
		request := obj("protocol", "hn.extension/1", "request_id", id, "method", "identity.public", "params", obj())
		_, err = InspectMessage(encode(request), "request", "", "")
		requireCode(t, err, "invalid_input")
	}
	request := obj("protocol", "hn.extension/1", "request_id", "r", "method", "runner", "params", obj())
	_, err = InspectMessage(encode(request), "request", "", "")
	requireCode(t, err, "unsupported_capability")
	for _, key := range []string{"expected", "method"} {
		expected, method := "", ""
		if key == "expected" {
			expected = "r"
		} else {
			method = "identity.public"
		}
		_, err = InspectMessage([]byte(ready), "ready", expected, method)
		requireCode(t, err, "invalid_input")
	}
	cancel := []byte(`{"protocol":"hn.extension/1","cancel":"r"}`)
	if _, err = InspectMessage(cancel, "cancel", "r", ""); err != nil {
		t.Fatal(err)
	}
	_, err = InspectMessage(cancel, "cancel", "other", "")
	requireCode(t, err, "invalid_input")
	failureResponse := obj("protocol", "hn.extension/1", "request_id", nil, "ok", false, "data", nil, "error", obj("origin", "native", "code", "stale_revision", "message", "expected revision is no longer the current head", "recovery", "do_not_retry", "operation", nil))
	if _, err = InspectMessage(encode(failureResponse), "response", "", ""); err != nil {
		t.Fatal(err)
	}
	failureResponse["ok"] = true
	_, err = InspectMessage(encode(failureResponse), "response", "", "")
	requireCode(t, err, "invalid_input")
	failureResponse["ok"] = false
	failureResponse["error"].(map[string]any)["code"] = "invented"
	_, err = InspectMessage(encode(failureResponse), "response", "", "")
	requireCode(t, err, "invalid_input")
	raw := `{"protocol":"hn.extension/1","request_id":"r","method":"resource.read","params":{"family":"issue","id":null,"limit":1,"limit":2,"cursor":null}}`
	_, err = InspectMessage([]byte(raw), "request", "", "")
	requireCode(t, err, "invalid_input")
}
func TestProtocolPayloadAtFrameLimit(t *testing.T) {
	raw := []byte(`{"protocol":"hn.extension/1","ready":true}`)
	raw = append(raw, bytes.Repeat([]byte(" "), FrameLimit-len(raw))...)
	if _, err := InspectMessage(raw, "ready", "", ""); err != nil {
		t.Fatal(err)
	}
	raw = append(raw, ' ')
	_, err := InspectMessage(raw, "ready", "", "")
	requireCode(t, err, "resource_limit")
}

func TestProtocolPublishedValueBounds(t *testing.T) {
	request := obj("protocol", "hn.extension/1", "request_id", "r", "method", "resource.read", "params", paramFixtures()["resource.read"])
	params := request["params"].(map[string]any)
	for _, n := range []int{1, 200} {
		params["limit"] = n
		if _, err := InspectMessage(encode(request), "request", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	params["limit"] = 201
	_, err := InspectMessage(encode(request), "request", "", "")
	requireCode(t, err, "resource_limit")
	params["limit"] = 0
	_, err = InspectMessage(encode(request), "request", "", "")
	requireCode(t, err, "invalid_input")
	params["limit"] = 1
	for _, n := range []int{1, 240000} {
		request["timeout_ms"] = n
		if _, err := InspectMessage(encode(request), "request", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	request["timeout_ms"] = 240001
	_, err = InspectMessage(encode(request), "request", "", "")
	requireCode(t, err, "invalid_input")
	delete(request, "timeout_ms")
	params["cursor"] = strings.Repeat("c", 4096)
	if _, err := InspectMessage(encode(request), "request", "", ""); err != nil {
		t.Fatal(err)
	}
	params["cursor"] = strings.Repeat("c", 4097)
	_, err = InspectMessage(encode(request), "request", "", "")
	requireCode(t, err, "resource_limit")
	response := obj("protocol", "hn.extension/1", "request_id", "r", "ok", false, "data", nil, "error", obj("origin", "host", "code", "denied_access", "message", strings.Repeat("m", 4096), "recovery", "do_not_retry", "operation", obj("protocol", "hn/0", "actor", testActor, "operation", "key")))
	if _, err := InspectMessage(encode(response), "response", "r", ""); err != nil {
		t.Fatal(err)
	}
	response["error"].(map[string]any)["message"] = strings.Repeat("m", 4097)
	_, err = InspectMessage(encode(response), "response", "r", "")
	requireCode(t, err, "resource_limit")
	params = paramFixtures()["artifact.mutate"]
	request = obj("protocol", "hn.extension/1", "request_id", "r", "method", "artifact.mutate", "params", params)
	payload := params["payload"].(map[string]any)
	payload["title"] = strings.Repeat("t", 1024)
	if _, err := InspectMessage(encode(request), "request", "", ""); err != nil {
		t.Fatal(err)
	}
	payload["title"] = strings.Repeat("t", 1025)
	_, err = InspectMessage(encode(request), "request", "", "")
	requireCode(t, err, "invalid_input")
}

// These are independent literal copies of the closed inventories frozen in
// host-v1 §§2/4/6/7.1 and artifact-v1 §§1/2/3, not values derived from validators.
func frozenProtocolInventories() map[string][]string {
	return map[string][]string{
		"methods":                 {"capabilities.list", "identity.public", "resource.read", "resource.mutate", "resource.operation", "artifact.read", "artifact.mutate", "artifact.operation", "artifact.search", "artifact.type.read", "artifact.type.publish", "changes.capture", "changes.page", "changes.since"},
		"unsupported":             {"memory.mutate", "proposal", "review", "decide", "merge", "policy.mutate", "identity.mutate", "run", "runner", "audit", "automerge", "sync.apply", "migration.apply"},
		"message_kinds":           {"hello", "ready", "request", "response", "cancel"},
		"families":                {"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"},
		"shapes":                  {"document", "list", "ticket"},
		"operation_protocols":     {"hn/0", "hn-artifact/1"},
		"recoveries":              {"do_not_retry", "retry_identical", "inspect_operation", "rescan_required"},
		"artifact_read_modes":     {"view", "heads", "history", "inspect"},
		"artifact_mutation_kinds": {"record.open", "record.revise", "record.resolve", "record.convert"},
		"native_intents":          {"open", "revise", "resolve"},
		"issue_intents":           {"open", "revise", "resolve", "comment"},
		"error_origins":           {"host", "native", "artifact"},
		"host_error_codes":        {"denied_access", "incompatible_version", "unavailable_module", "unsupported_capability", "resource_limit", "cancelled", "uncertain_operation", "checkpoint_invalidated", "verification_failure", "invalid_input", "not_found", "repository_error"},
		"native_error_codes":      {"actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision"},
		"artifact_error_codes":    {"invalid_input", "resource_limit", "verification_failure", "invalid_history", "incomplete_dependency", "unsupported_schema", "actor_stream_conflict", "operation_conflict", "stale_revision", "resource_conflict", "concurrent_update", "not_found", "repository_error", "cancelled", "uncertain_operation"},
		"diagnostic_codes":        {"incomplete_dependency", "unsupported_schema", "target_conflict", "missing_optional", "invalid_history", "opaque_unverified"},
		"native_admission":        {"admitted", "pending", "hidden"},
		"operation_statuses":      {"found", "verified_absent", "uncertain"},
		"operation_reasons":       {"incomplete_selection", "unavailable_history", "private_intent_pending", "visibility_restricted", "publication_unconfirmed"},
		"availability":            {"observed", "not_probed", "unavailable"},
		"signed_validation":       {"verified", "unsupported", "incomplete"},
		"artifact_envelope_kinds": {"type.publish", "record.open", "record.revise", "record.resolve", "record.convert"},
		"target_modes":            {"logical", "revision"},
		"schema_node_types":       {"object", "array", "string", "integer", "boolean", "null"},
		"native_statuses":         {"open", "closed"},
		"native_relation_kinds":   {"blocks", "related", "parent"},
		"project_value_types":     {"text", "number", "date", "single_select"},
		"project_layouts":         {"table", "board"},
		"project_target_kinds":    {"issue", "proposal", "discussion", "draft", "external"},
	}
}

func protocolInventoryDifference(actual, expected []string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("inventory count got %d, want %d", len(actual), len(expected))
	}
	seen := map[string]bool{}
	for _, value := range actual {
		if seen[value] {
			return fmt.Errorf("duplicate inventory member %q", value)
		}
		seen[value] = true
	}
	got, want := append([]string(nil), actual...), append([]string(nil), expected...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("inventory differs: got %q, want %q", got, want)
	}
	return nil
}

func runtimeProtocolInventories() map[string][]string {
	actual := make(map[string][]string, len(protocolInventories)+1)
	for name, values := range protocolInventories {
		actual[name] = values
	}
	// The existing WP13 method map remains the source used by requests and the
	// capabilities result. Inspect its real accessor, not the fixture map.
	actual["methods"] = methodNames()
	return actual
}
func TestProtocolClosedInventories(t *testing.T) {
	actual, expected := runtimeProtocolInventories(), frozenProtocolInventories()
	gotNames, wantNames := []string{}, []string{}
	for name := range actual {
		gotNames = append(gotNames, name)
	}
	for name := range expected {
		wantNames = append(wantNames, name)
	}
	if err := protocolInventoryDifference(gotNames, wantNames); err != nil {
		t.Fatalf("closed inventory names: %v", err)
	}
	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			if err := protocolInventoryDifference(actual[name], want); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := protocolInventoryDifference(unsupportedNames(), expected["unsupported"]); err != nil {
		t.Fatalf("actual unsupported result inventory: %v", err)
	}
}
func TestProtocolInventoryMirrorsDetectDrift(t *testing.T) {
	for name, actual := range runtimeProtocolInventories() {
		t.Run(name, func(t *testing.T) {
			expected := frozenProtocolInventories()[name]
			// Each alteration acts on the actual private runtime inventory, copied only
			// to avoid changing process-wide validators while checking the mirror guard.
			extra := append(append([]string(nil), actual...), "unexpected.contract.member")
			missing := append([]string(nil), actual[1:]...)
			renamed := append([]string(nil), actual...)
			renamed[0] = "renamed.contract.member"
			duplicate := append([]string(nil), actual...)
			duplicate[len(duplicate)-1] = duplicate[0]
			for control, values := range map[string][]string{"extra": extra, "missing": missing, "renamed": renamed, "duplicate": duplicate} {
				if err := protocolInventoryDifference(values, expected); err == nil {
					t.Fatalf("%s inventory drift escaped the exact mirror", control)
				}
			}
		})
	}
}
