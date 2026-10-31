package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestStrictPublicFrames(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, `{"a":"\ud800"}`, `{"a":1.0}`, `{"a":9007199254740992}`, `{} {}`, `[]`, "{\"a\":\"\xff\"}"} {
		if _, e := decodeObject([]byte(raw)); e == nil {
			t.Errorf("accepted hostile public JSON %q", raw)
		}
	}
	if _, e := decodeObject([]byte(`{"a":"雪 e\u0301","b":9007199254740991}`)); e != nil {
		t.Fatal(e)
	}
	for _, n := range []uint32{0, frameBytes + 1} {
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], n)
		if _, e := readFrame(bytes.NewReader(prefix[:])); e == nil {
			t.Fatalf("accepted invalid length %d", n)
		}
	}
	// Equality and first-over, including payload, never silently truncate.
	for _, n := range []int{frameBytes, frameBytes + 1} {
		raw := []byte(`{"s":"` + strings.Repeat("x", n-8) + `"}`)
		var buf bytes.Buffer
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
		buf.Write(prefix[:])
		buf.Write(raw)
		_, e := readFrame(&buf)
		if (e == nil) != (n == frameBytes) {
			t.Fatalf("frame boundary %d: %v", n, e)
		}
	}
}

func typedFixtures(t *testing.T) map[string]map[string]any {
	t.Helper()
	const id = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const actor = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	mutation := `{"protocol":"hn-artifact/1","family":null,"type_id":"example.charter/directive","resource_id":"$ID","event_id":"$ID","replayed":false,"conflict":false,"head_count":1,"admission":null,"source_cut":"$ID"}`
	payload := `{"shape":"document","type_id":"example.charter/directive","type_revision":"$ID","title":"Example","content":"exact data","fields":{},"members":[],"relationships":[],"conversion_sources":[]}`
	signed := `{"id":"$ID","envelope":{"protocol":"hn-artifact/1","kind":"type.publish","actor":"$ACTOR","public_key":"$PUBLIC","stream":"$ID","sequence":1,"previous":"","timestamp":"2026-10-31T00:00:00Z","operation":"type-1","request":"$ID","record":"","parents":[],"payload":{"type_id":"example.charter/directive","schema_version":1,"shape":"document","schema":{"language":"hn.schema/1","root":{"type":"object","properties":{},"required":[],"additionalProperties":false}}}},"signature":"$SIGNATURE","validation":"verified","diagnostics":[]}`
	view := `{"protocol":"hn-artifact/1","id":"$ID","type_id":"example.charter/directive","shape":"document","heads":[{"id":"$ID","actor":"$ACTOR","type_revision":"$ID","payload":` + payload + `}],"conflict":false,"state":` + payload + `,"diagnostics":[]}`
	literals := map[string]string{
		"identity.public":       `{"initialized":true,"actor":"$ACTOR","name":"Example author","public_key":"$PUBLIC"}`,
		"artifact.mutate":       mutation,
		"artifact.type.publish": mutation,
		"artifact.operation":    `{"status":"found","protocol":"hn-artifact/1","actor":"$ACTOR","operation":"open-1","family":null,"type_id":"example.charter/directive","request":"$ID","resource_id":"$ID","event_id":"$ID","result":` + mutation + `,"source_cut":"$ID","reason":null}`,
		"artifact.type.read":    `{"items":[` + signed + `],"source_cut":"$ID","next_cursor":null}`,
		"artifact.read":         `{"items":[` + signed + `],"source_cut":"$ID","next_cursor":null}`,
		"artifact.search":       `{"items":[` + view + `],"source_cut":"$ID","next_cursor":null}`,
	}
	results := map[string]map[string]any{}
	for method, literal := range literals {
		raw := strings.NewReplacer("$ID", id, "$ACTOR", actor, "$PUBLIC", strings.Repeat("A", 43), "$SIGNATURE", strings.Repeat("A", 86)).Replace(literal)
		data, e := decodeObject([]byte(raw))
		if e != nil {
			t.Fatal(e)
		}
		results[method] = map[string]any{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": true, "data": data, "error": nil}
	}
	return results
}

func wireCopy(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	m, e := decodeObject(raw)
	if e != nil {
		t.Fatal(e)
	}
	return m
}

// Exercise the real client call, so these controls also run against the
// pre-correction client for a meaningful behavioral RED once execution is granted.
func testReply(response map[string]any, method string, params map[string]any, expected string) error {
	if expected != "charter-1" {
		panic("fixture must target the first client request")
	}
	var input, output bytes.Buffer
	if e := writeFrame(&input, response); e != nil {
		return e
	}
	c := &client{in: &input, out: &output}
	_, e := c.call(method, params)
	return e
}

func TestMethodTypedResponsesBeforeNextRequest(t *testing.T) {
	for method, response := range typedFixtures(t) {
		t.Run(method, func(t *testing.T) {
			params := map[string]any{"mode": "inspect", "opaque": false}
			if e := testReply(response, method, params, "charter-1"); e != nil {
				t.Fatal(e)
			}
			bad := wireCopy(t, response)
			bad["data"].(map[string]any)["unknown"] = true
			if testReply(bad, method, params, "charter-1") == nil {
				t.Fatal("accepted unknown typed success field")
			}
			for _, bad := range []map[string]any{
				{"protocol": "hn.extension/1", "request_id": "wrong", "ok": true, "data": response["data"], "error": nil},
				{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": "true", "data": response["data"], "error": nil},
				{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": true, "data": nil, "error": nil},
				{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": true, "data": response["data"], "error": map[string]any{}},
				{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": false, "data": nil, "error": map[string]any{"origin": "host", "code": "denied_access", "message": 17, "recovery": "do_not_retry", "operation": nil}},
			} {
				if testReply(bad, method, params, "charter-1") == nil {
					t.Fatal("accepted malformed response", bad)
				}
			}
		})
	}
	fixtures := typedFixtures(t)
	for _, test := range []struct {
		method, field string
		bad           any
	}{{"identity.public", "initialized", "yes"}, {"identity.public", "public_key", "not-base64"}, {"artifact.mutate", "replayed", nil}, {"artifact.mutate", "head_count", true}, {"artifact.operation", "status", "absent"}, {"artifact.type.read", "items", nil}, {"artifact.search", "next_cursor", false}} {
		response := wireCopy(t, fixtures[test.method])
		response["data"].(map[string]any)[test.field] = test.bad
		if testReply(response, test.method, map[string]any{"mode": "inspect", "opaque": false}, "charter-1") == nil {
			t.Fatal("accepted wrong typed primitive", test)
		}
	}
	for _, method := range []string{"artifact.read", "artifact.type.read", "artifact.search"} {
		response := wireCopy(t, fixtures[method])
		row := response["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
		row["unknown"] = true
		if testReply(response, method, map[string]any{"mode": "inspect", "opaque": false}, "charter-1") == nil {
			t.Fatal("accepted unknown nested row", method)
		}
	}
}

type oneByteReader struct {
	raw      []byte
	consumed int
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.raw) == 0 {
		return 0, io.EOF
	}
	p[0] = r.raw[0]
	r.raw = r.raw[1:]
	r.consumed++
	return 1, nil
}

type orderedRequestWriter struct {
	bytes.Buffer
	response                 *oneByteReader
	firstResponseBytes       int
	requests, completedBytes int
}

func (w *orderedRequestWriter) Write(p []byte) (int, error) {
	if w.requests > 0 && w.response.consumed < w.firstResponseBytes {
		return 0, io.ErrUnexpectedEOF
	}
	n, e := w.Buffer.Write(p)
	for {
		raw := w.Buffer.Bytes()[w.completedBytes:]
		if len(raw) < 4 {
			break
		}
		size := 4 + int(binary.BigEndian.Uint32(raw[:4]))
		if len(raw) < size {
			break
		}
		w.requests++
		w.completedBytes += size
	}
	return n, e
}

func TestClientConsumesFullResponseAndStopsOnMalformedTypedData(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		response := typedFixtures(t)["identity.public"]
		if malformed {
			response["data"].(map[string]any)["initialized"] = "true"
		}
		var frames bytes.Buffer
		if e := writeFrame(&frames, response); e != nil {
			t.Fatal(e)
		}
		first := frames.Len()
		response = typedFixtures(t)["identity.public"]
		response["request_id"] = "charter-2"
		if e := writeFrame(&frames, response); e != nil {
			t.Fatal(e)
		}
		in := &oneByteReader{raw: frames.Bytes()}
		out := &orderedRequestWriter{response: in, firstResponseBytes: first}
		c := &client{in: in, out: out}
		_, e := c.call("identity.public", map[string]any{})
		if malformed {
			if e == nil || out.requests != 1 || in.consumed != first {
				t.Fatal("malformed complete response allowed progress", e, out.requests, in.consumed)
			}
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.call("identity.public", map[string]any{}); e != nil || out.requests != 2 {
			t.Fatal("valid fully consumed response did not permit next request", e, out.requests)
		}
	}
	response := typedFixtures(t)["identity.public"]
	var frame bytes.Buffer
	_ = writeFrame(&frame, response)
	for _, n := range []int{1, 3, frame.Len() - 1} {
		in := bytes.NewReader(frame.Bytes()[:n])
		var out bytes.Buffer
		c := &client{in: in, out: &out}
		if _, e := c.call("identity.public", map[string]any{}); e == nil {
			t.Fatal("accepted incomplete response prefix/body", n)
		}
	}
}

func TestResponseErrorVocabulary(t *testing.T) {
	want := map[string]string{
		"host":     "denied_access incompatible_version unavailable_module unsupported_capability resource_limit cancelled uncertain_operation checkpoint_invalidated verification_failure invalid_input not_found repository_error",
		"native":   "actor_mismatch admission_pending admission_refused ambiguous_id concurrent_update invalid_cursor invalid_history invalid_input issue_conflict not_found operation_conflict permission_denied repository_error resource_conflict resource_limit stale_cursor stale_revision",
		"artifact": "invalid_input resource_limit verification_failure invalid_history incomplete_dependency unsupported_schema actor_stream_conflict operation_conflict stale_revision resource_conflict concurrent_update not_found repository_error cancelled uncertain_operation",
	}
	if !reflect.DeepEqual(responseErrorCodes, want) {
		t.Fatal("closed contract error inventories drifted", responseErrorCodes)
	}
	for origin, codes := range want {
		for _, code := range strings.Fields(codes) {
			response := map[string]any{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": false, "data": nil, "error": map[string]any{"origin": origin, "code": code, "message": "explicit business refusal", "recovery": "do_not_retry", "operation": nil}}
			if e := testReply(response, "artifact.mutate", map[string]any{}, "charter-1"); e != nil {
				t.Fatal(origin, code, e)
			}
		}
	}
	for _, failure := range []map[string]any{
		{"origin": "artifact", "code": "target_conflict", "message": "diagnostic is not an error code", "recovery": "do_not_retry", "operation": nil},
		{"origin": "host", "code": "denied_access", "message": "refusal", "recovery": "automatically_retry", "operation": nil},
		{"origin": "native", "code": "unsupported_capability", "message": "wrong-origin code", "recovery": "do_not_retry", "operation": nil},
		{"origin": "host", "code": "denied_access", "message": "refusal", "recovery": "do_not_retry", "operation": map[string]any{"protocol": "hn-artifact/1", "actor": true, "operation": "key"}},
	} {
		response := map[string]any{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": false, "data": nil, "error": failure}
		if testReply(response, "artifact.mutate", map[string]any{}, "charter-1") == nil {
			t.Fatal("accepted malformed business refusal", failure)
		}
	}
}

func TestNestedTypedReadAndOperationVariants(t *testing.T) {
	fixtures := typedFixtures(t)
	for _, mutate := range []func(map[string]any){
		func(row map[string]any) { row["signature"] = "bad-base64" },
		func(row map[string]any) { row["envelope"].(map[string]any)["extra"] = true },
		func(row map[string]any) {
			row["envelope"].(map[string]any)["payload"].(map[string]any)["schema"].(map[string]any)["root"].(map[string]any)["required"] = nil
		},
	} {
		response := wireCopy(t, fixtures["artifact.read"])
		row := response["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
		mutate(row)
		if testReply(response, "artifact.read", map[string]any{"mode": "inspect", "opaque": false}, "charter-1") == nil {
			t.Fatal("accepted malformed nested signed artifact", row)
		}
	}
	for _, status := range []string{"verified_absent", "uncertain"} {
		response := wireCopy(t, fixtures["artifact.operation"])
		data := response["data"].(map[string]any)
		for _, field := range []string{"request", "resource_id", "event_id", "result"} {
			data[field] = nil
		}
		data["status"] = status
		if status == "uncertain" {
			data["reason"] = "publication_unconfirmed"
		}
		if e := testReply(response, "artifact.operation", map[string]any{}, "charter-1"); e != nil {
			t.Fatal(status, e)
		}
		data["reason"] = nil
		if status == "uncertain" && testReply(response, "artifact.operation", map[string]any{}, "charter-1") == nil {
			t.Fatal("accepted unexplained uncertain operation")
		}
	}
	response := wireCopy(t, fixtures["artifact.search"])
	if e := testReply(response, "artifact.read", map[string]any{"mode": "heads", "opaque": false}, "charter-1"); e != nil {
		t.Fatal(e)
	}
	if testReply(response, "artifact.read", map[string]any{"mode": "inspect", "opaque": false}, "charter-1") == nil {
		t.Fatal("accepted a current view as an exact signed revision")
	}
	for _, conflict := range []bool{false, true} {
		row := response["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
		row["conflict"] = conflict
		if conflict {
			row["state"] = nil
		}
		if e := testReply(response, "artifact.search", map[string]any{}, "charter-1"); e != nil {
			t.Fatal(e)
		}
	}
}

func TestHelloBindingAndClosedLimits(t *testing.T) {
	valid := `{"protocol":"hn.extension/1","hello":{"session_id":"00000000000000000000000000000001","repository_id":"sha256:` + strings.Repeat("a", 64) + `","module_id":"example.charter/model","module_version":"1","registration_id":"sha256:` + strings.Repeat("b", 64) + `"},"limits":{"frame_bytes":8388608,"read_timeout_ms":30000,"mutation_timeout_ms":240000,"page_max":200,"one_outstanding":true}}`
	if e := acceptHello([]byte(valid)); e != nil {
		t.Fatal(e)
	}
	if e := checkPinnedBinding([]byte(valid), map[string]any{"expected_repository_id": "sha256:" + strings.Repeat("a", 64), "expected_registration_id": "sha256:" + strings.Repeat("b", 64)}); e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"expected_repository_id", "expected_registration_id"} {
		if e := checkPinnedBinding([]byte(valid), map[string]any{field: "sha256:" + strings.Repeat("c", 64)}); e == nil || e.Error() != "operator-pinned host binding mismatch" {
			t.Fatal("wrong binding accepted", field, e)
		}
	}
	for _, raw := range []string{strings.Replace(valid, "example.charter/model", "other.example/model", 1), strings.Replace(valid, "8388608", "8388609", 1), strings.Replace(valid, `"one_outstanding":true`, `"one_outstanding":false`, 1), strings.Replace(valid, `"protocol":`, `"unknown":1,"protocol":`, 1), strings.Replace(valid, strings.Repeat("a", 64), "bad", 1)} {
		if acceptHello([]byte(raw)) == nil {
			t.Errorf("accepted incorrect binding/limit: %s", raw)
		}
	}
}

func TestLocalInputBound(t *testing.T) {
	for _, n := range []int{inputBytes, inputBytes + 1} {
		raw := `{"directive_text":"` + strings.Repeat("x", n-21) + `"}`
		if len(raw) != n {
			t.Fatalf("bad fixture %d != %d", len(raw), n)
		}
		_, e := readInput(strings.NewReader(raw))
		if (e == nil) != (n == inputBytes) {
			t.Fatalf("input boundary %d: %v", n, e)
		}
	}
}

func TestExampleMethodInventoryAndNestingBound(t *testing.T) {
	if !reflect.DeepEqual(methods, []string{"identity.public", "artifact.type.publish", "artifact.type.read", "artifact.mutate", "artifact.read", "artifact.search", "artifact.operation"}) {
		t.Fatal("example advertised unused or missing methods", methods)
	}
	for _, n := range []int{64, 65} {
		raw := strings.Repeat(`{"x":`, n) + `0` + strings.Repeat(`}`, n)
		_, e := decodeObject([]byte(raw))
		if (e == nil) != (n == 64) {
			t.Fatalf("nesting %d: %v", n, e)
		}
	}
}
