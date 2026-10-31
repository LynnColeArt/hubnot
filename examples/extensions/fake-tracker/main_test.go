package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestPrivateAndWireBounds(t *testing.T) {
	for _, limit := range []int{inputBytes, frameBytes, privateBytes} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			at := []byte(`{}` + strings.Repeat(" ", limit-2))
			raw, err := readBounded(bytes.NewReader(at), limit)
			if err != nil || len(raw) != limit {
				t.Fatal("at-bound refused", limit, err)
			}
			if _, err = readBounded(bytes.NewReader(append(at, ' ')), limit); err == nil || err.Error() != "resource_limit" {
				t.Fatal("one-over accepted", limit, err)
			}
			path := filepath.Join(t.TempDir(), "input.json")
			if err = os.WriteFile(path, at, 0600); err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err = readPrivate(path, limit, &value); err != nil {
				t.Fatal("private at-bound", err)
			}
			if err = os.WriteFile(path, append(at, ' '), 0600); err != nil {
				t.Fatal(err)
			}
			if err = readPrivate(path, limit, &value); err == nil || err.Error() != "resource_limit" {
				t.Fatal("private one-over", err)
			}
			atValue := map[string]any{"x": strings.Repeat("a", limit-8)}
			if err = writePrivate(path, atValue, limit); err != nil {
				t.Fatal("private transaction at-bound", err)
			}
			before, _ := os.ReadFile(path)
			if len(before) != limit {
				t.Fatal("transaction equality bytes", len(before), limit)
			}
			atValue["x"] = strings.Repeat("a", limit-7)
			if err = writePrivate(path, atValue, limit); err == nil || err.Error() != "resource_limit" {
				t.Fatal("transaction first-over", err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("first-over changed private transaction")
			}

		})
	}
	// A valid exactly 8MiB JSON frame and its first-over prefix are tested at the
	// consumer's actual frame decoder; allocation refuses on the prefix alone.
	at := []byte(`{"x":"` + strings.Repeat("a", frameBytes-8) + `"}`)
	if len(at) != frameBytes {
		t.Fatal(len(at))
	}
	var wire bytes.Buffer
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(at)))
	wire.Write(prefix[:])
	wire.Write(at)
	value, err := readFrame(&wire)
	if err != nil || len(value["x"].(string)) != frameBytes-8 {
		t.Fatal("frame equality", err)
	}
	binary.BigEndian.PutUint32(prefix[:], frameBytes+1)
	if _, err = readFrame(bytes.NewReader(prefix[:])); err == nil || err.Error() != "resource_limit" {
		t.Fatal("over prefix", err)
	}
	binary.BigEndian.PutUint32(prefix[:], 0)
	if _, err = readFrame(bytes.NewReader(prefix[:])); err == nil || err.Error() != "invalid_input" {
		t.Fatal("zero frame", err)
	}
	if err = writeFrame(io.Discard, map[string]any{"x": strings.Repeat("a", frameBytes-8)}); err != nil {
		t.Fatal("write equality", err)
	}
	if err = writeFrame(io.Discard, map[string]any{"x": strings.Repeat("a", frameBytes-7)}); err == nil || err.Error() != "resource_limit" {
		t.Fatal("write over", err)
	}
}
func TestPrivateTransactionAndBinding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal")
	binding := map[string]any{"repository_id": "sha256:repo", "registration_id": "sha256:registration", "module_id": "example.tracker/tool"}
	c := config{API: "http://127.0.0.1", Token: "secret", Connection: "connection", Source: "source", Journal: path}
	j, err := loadJournal(c, binding)
	if err != nil {
		t.Fatal(err)
	}
	j.Native["issue"] = &nativeIntent{Params: map[string]any{"actor": "actor", "operation": "key", "expected": []any{"parent"}, "state": map[string]any{"title": "original"}}}
	if err = writePrivate(path, j, privateBytes); err != nil {
		t.Fatal(err)
	}
	again, err := loadJournal(c, binding)
	if err != nil || !reflect.DeepEqual(again, j) {
		t.Fatal("original durable request", err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0600 {
		t.Fatal("public journal permission")
	}
	binding["repository_id"] = "sha256:other"
	if _, err = loadJournal(c, binding); err == nil || err.Error() != "connector_binding_changed" {
		t.Fatal("cross-repo journal", err)
	}
	before, _ := os.ReadFile(path)
	if err = writePrivate(path, map[string]any{"value": strings.Repeat("x", privateBytes)}, privateBytes); err == nil {
		t.Fatal("oversized transaction accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed transaction changed original")
	}
	lock, err := lockJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if other, err := lockJournal(path); err == nil {
		other.Close()
		t.Fatal("parallel connector acquired same journal")
	}
}
func TestFakeAPIReceiptsVersionConflictAndRealResponseLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.json")
	service := &fakeService{path: path, token: "secret", state: serviceState{Item: externalItem{ID: "42", Version: 1}, Receipts: map[string]receipt{}}}
	writes := func() int { service.mu.Lock(); defer service.mu.Unlock(); return service.state.Writes }
	server := httptest.NewServer(service)
	defer server.Close()
	x := &client{c: config{API: server.URL, Token: "secret"}, http: server.Client()}
	request := externalRequest{Key: "key", Expected: 1, Source: "source", Resource: "resource", Revisions: []string{"revision"}, Title: "once", LoseAck: true}
	if err := x.api("POST", "/writes", request, new(receipt)); err == nil || err.Error() != "external_uncertain" {
		t.Fatal("real socket loss", err)
	}
	var persisted serviceState
	if err := readPrivate(path, privateBytes, &persisted); err != nil || persisted.Writes != 1 {
		t.Fatal("lost ack before effect", err)
	}
	var found receipt
	if err := x.api("GET", "/writes/key", nil, &found); err != nil || !reflect.DeepEqual(found.Request, request) || found.Version != 2 {
		t.Fatal("private API reconciliation", err)
	}
	var replay receipt
	if err := x.api("POST", "/writes", request, &replay); err != nil || replay.Version != found.Version || writes() != 1 {
		t.Fatal("external exact replay", err)
	}
	request.Title = "changed"
	if err := x.api("POST", "/writes", request, new(receipt)); err == nil || err.Error() != "external_conflict" {
		t.Fatal("changed external replay", err)
	}
	request.Key = "new"
	if err := x.api("POST", "/writes", request, new(receipt)); err == nil || err.Error() != "external_conflict" {
		t.Fatal("stale actual version", err)
	}
	if writes() != 1 {
		t.Fatal("conflict caused extra write")
	}
	x.c.Token = "ungranted"
	if err := x.api(http.MethodGet, "/item", nil, new(externalItem)); err == nil {
		t.Fatal("missing private API credential")
	}
	// Actual bounded HTTP decoder rejects one-over before any state effect.
	req, _ := http.NewRequest("POST", server.URL+"/writes", strings.NewReader(strings.Repeat(" ", frameBytes+1)))
	req.Header.Set("Authorization", "Bearer secret")
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge || writes() != 1 {
		t.Fatal("HTTP one-over effect", response.StatusCode)
	}
	raw, _ := json.Marshal(externalRequest{Key: "at", Expected: 2, Revisions: []string{}, Title: "at"})
	padded := append(raw, bytes.Repeat([]byte(" "), frameBytes-len(raw))...)
	req, _ = http.NewRequest("POST", server.URL+"/writes", bytes.NewReader(padded))
	req.Header.Set("Authorization", "Bearer secret")
	response, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || writes() != 2 {
		t.Fatal("HTTP at-bound", response.StatusCode)
	}
}

func TestHTTPResponseConsumptionBoundary(t *testing.T) {
	item := externalItem{ID: "42", Version: 1, Title: "valid fake item", Comments: []externalComment{{ID: "c1", Body: "comment"}}}
	raw, _ := json.Marshal(item)
	item.Body = strings.Repeat("a", frameBytes-len(raw))
	at, _ := json.Marshal(item)
	if len(at) != frameBytes {
		t.Fatal("HTTP response at-bound setup", len(at))
	}
	for _, over := range []bool{false, true} {
		t.Run(strconv.FormatBool(over), func(t *testing.T) {
			body := at
			if over {
				body = append(append([]byte{}, at...), ' ')
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}))
			defer server.Close()
			x := &client{c: config{API: server.URL, Token: "private"}, http: server.Client()}
			var result externalItem
			err := x.api("GET", "/item", nil, &result)
			if over {
				if err == nil || err.Error() != "resource_limit" {
					t.Fatal("HTTP response one-over", err)
				}
			} else if err != nil || result.ID != "42" || result.Body != item.Body {
				t.Fatal("HTTP response at-bound", err)
			}
		})
	}
}

func TestUnsupportedInitialCommentSetRefusesBeforeNativeIntent(t *testing.T) {
	for _, count := range []int{0, 2} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			item := externalItem{ID: "42", Version: 1, Title: "initial item", Comments: make([]externalComment, count)}
			for i := range item.Comments {
				item.Comments[i] = externalComment{ID: strconv.Itoa(i), Body: "comment"}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(item) }))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "journal.json")
			x := &client{c: config{API: server.URL, Token: "private", Journal: path}, j: &journal{Native: map[string]*nativeIntent{}}, http: server.Client()}
			if err := x.importItem(false); err == nil || err.Error() != "invalid_input" {
				t.Fatal("unsupported collection cause", err)
			}
			if len(x.j.Native) != 0 || x.j.InitialItem != nil {
				t.Fatal("unsupported comment set prepared native intent")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("unsupported comment set wrote a journal")
			}
		})
	}
}

func TestFakeAPIConcurrentVersionCAS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.json")
	service := &fakeService{path: path, token: "private", state: serviceState{Item: externalItem{ID: "42", Version: 1}, Receipts: map[string]receipt{}}}
	server := httptest.NewServer(service)
	defer server.Close()
	type outcome struct {
		title string
		err   error
	}
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan outcome, 2)
	for _, title := range []string{"first independent writer", "second independent writer"} {
		go func() {
			x := &client{c: config{API: server.URL, Token: "private"}, http: server.Client()}
			ready <- struct{}{}
			<-start
			request := externalRequest{Key: title, Expected: 1, Source: "source", Resource: "42", Revisions: []string{"revision"}, Title: title}
			results <- outcome{title: title, err: x.api("POST", "/writes", request, new(receipt))}
		}()
	}
	<-ready
	<-ready
	close(start)
	winner := ""
	conflicts := 0
	for range 2 {
		result := <-results
		if result.err == nil {
			if winner != "" {
				t.Fatal("two independent stale writers succeeded")
			}
			winner = result.title
		} else if result.err.Error() == "external_conflict" {
			conflicts++
		} else {
			t.Fatal("wrong concurrent refusal", result.err)
		}
	}
	if winner == "" || conflicts != 1 {
		t.Fatal("expected one current-version write and one real conflict", winner, conflicts)
	}
	var persisted serviceState
	if err := readPrivate(path, privateBytes, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Writes != 1 || persisted.Item.Version != 2 || persisted.Item.Title != winner || len(persisted.Receipts) != 1 {
		t.Fatal("concurrent conflict changed external state or receipts")
	}
	for _, receipt := range persisted.Receipts {
		if receipt.Request.Title != winner || receipt.Request.Expected != 1 || receipt.Version != 2 {
			t.Fatal("receipt lost exact winning intent")
		}
	}
}

func testHostHello() map[string]any {
	return map[string]any{"protocol": protocol,
		"hello":  map[string]any{"session_id": strings.Repeat("1", 32), "module_id": "example.tracker/tool", "module_version": "1", "repository_id": "sha256:" + strings.Repeat("a", 64), "registration_id": "sha256:" + strings.Repeat("b", 64)},
		"limits": map[string]any{"frame_bytes": frameBytes, "read_timeout_ms": 30000, "mutation_timeout_ms": 240000, "page_max": 200, "one_outstanding": true}}
}
func testHostID() string { return "sha256:" + strings.Repeat("a", 64) }
func testHostState() map[string]any {
	return map[string]any{"title": "native title", "body": "native body", "status": "open", "criteria": []any{}, "labels": []any{}, "assignees": []any{}, "relations": []any{}, "metadata": map[string]any{}}
}
func testHostView() map[string]any {
	return map[string]any{"protocol": "hn/0", "family": "issue", "id": testHostID(), "creator": strings.Repeat("a", 64), "heads": []any{map[string]any{"id": testHostID(), "actor": strings.Repeat("a", 64), "state": testHostState()}}, "conflict": false, "state": testHostState(), "comments": []any{}}
}
func testHostMutation() map[string]any {
	return map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": testHostID(), "event_id": testHostID(), "replayed": false, "conflict": false, "head_count": 1, "admission": nil, "source_cut": testHostID()}
}
func testHostPage() map[string]any {
	return map[string]any{"capture": "opaque capture", "snapshot_id": testHostID(), "selection": map[string]any{"families": []any{"issue"}, "types": []any{}}, "selection_digest": testHostID(), "source_cut": testHostID(), "mode": "replace", "items": []any{map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": testHostID(), "revision_ids": []any{testHostID()}, "value": testHostView()}}, "next_cursor": nil, "complete": true, "checkpoint": "opaque checkpoint", "expires_at": "2026-10-31T23:59:59Z"}
}
func testHostRefusal() map[string]any {
	return map[string]any{"origin": "host", "code": "denied_access", "message": "action is not granted", "recovery": "do_not_retry", "operation": nil}
}

// Round-trip helpers produce actual decoded wire primitive types, not Go-only
// int values that a JSON response can never expose to the client.
func testHostResponse(t *testing.T, data any) map[string]any {
	t.Helper()
	var wire bytes.Buffer
	if err := writeFrame(&wire, map[string]any{"protocol": protocol, "request_id": "tracker-1", "ok": true, "data": data, "error": nil}); err != nil {
		t.Fatal(err)
	}
	response, err := readFrame(&wire)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
func TestClientRejectsMalformedResponsesAndStopsSession(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alter func(map[string]any)
	}{
		{"wrong correlation", func(r map[string]any) { r["request_id"] = "tracker-2" }},
		{"null success correlation", func(r map[string]any) { r["request_id"] = nil }},
		{"missing correlation", func(r map[string]any) { delete(r, "request_id") }},
		{"unknown envelope field", func(r map[string]any) { r["extra"] = true }},
		{"wrong protocol", func(r map[string]any) { r["protocol"] = "hn.extension/2" }},
		{"wrong ok type", func(r map[string]any) { r["ok"] = "true" }},
		{"success plus error", func(r map[string]any) { r["error"] = testHostRefusal() }},
		{"failure plus data", func(r map[string]any) { r["ok"] = false; r["error"] = testHostRefusal() }},
		{"null data", func(r map[string]any) { r["data"] = nil }},
		{"unknown success field", func(r map[string]any) { r["data"].(map[string]any)["extra"] = true }},
		{"wrong nested primitive", func(r map[string]any) {
			r["data"].(map[string]any)["items"].([]any)[0].(map[string]any)["state"].(map[string]any)["body"] = 7
		}},
		{"null nested array", func(r map[string]any) {
			r["data"].(map[string]any)["items"].([]any)[0].(map[string]any)["comments"] = nil
		}},
		{"missing required nested field", func(r map[string]any) {
			delete(r["data"].(map[string]any)["items"].([]any)[0].(map[string]any)["heads"].([]any)[0].(map[string]any), "actor")
		}},
		{"terminal null ID failure", func(r map[string]any) {
			r["request_id"] = nil
			r["ok"] = false
			r["data"] = nil
			r["error"] = testHostRefusal()
		}},
		{"unknown error code", func(r map[string]any) {
			r["ok"] = false
			r["data"] = nil
			e := testHostRefusal()
			e["code"] = "invented"
			r["error"] = e
		}},
		{"wrong origin for code", func(r map[string]any) {
			r["ok"] = false
			r["data"] = nil
			e := testHostRefusal()
			e["origin"] = "native"
			r["error"] = e
		}},
		{"unknown nested error field", func(r map[string]any) {
			r["ok"] = false
			r["data"] = nil
			e := testHostRefusal()
			e["secret"] = "untrusted"
			r["error"] = e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			good := testHostResponse(t, map[string]any{"items": []any{testHostView()}, "source_cut": testHostID(), "next_cursor": nil})
			var input, output bytes.Buffer
			tc.alter(good)
			if err := writeFrame(&input, good); err != nil {
				t.Fatal(err)
			}
			x := &client{c: config{Report: filepath.Join(t.TempDir(), "report")}, input: &input, output: &output}
			_, err := x.call("resource.read", map[string]any{"family": "issue"})
			want := "invalid_input"
			if tc.name == "terminal null ID failure" {
				want = "denied_access"
			}
			if err == nil || err.Error() != want {
				t.Fatal("malformed/terminal response was accepted or lost exact cause", err)
			}
			before := append([]byte(nil), output.Bytes()...)
			_, again := x.call("identity.public", map[string]any{})
			if again == nil || again.Error() != want || !bytes.Equal(before, output.Bytes()) || x.sequence != 1 {
				t.Fatal("stopped session emitted another request", again, x.sequence)
			}
		})
	}
}
func TestClientConsumesFullResponseAndHandlesOrdinaryRefusal(t *testing.T) {
	good := testHostResponse(t, map[string]any{"initialized": false, "actor": nil, "name": nil, "public_key": nil})
	var input, output bytes.Buffer
	refusal := map[string]any{"protocol": protocol, "request_id": "tracker-1", "ok": false, "data": nil, "error": testHostRefusal()}
	if err := writeFrame(&input, refusal); err != nil {
		t.Fatal(err)
	}
	good["request_id"] = "tracker-2"
	if err := writeFrame(&input, good); err != nil {
		t.Fatal(err)
	}
	x := &client{c: config{Report: filepath.Join(t.TempDir(), "report")}, input: &input, output: &output}
	if _, err := x.ok("identity.public", map[string]any{}); err == nil || err.Error() != "denied_access" {
		t.Fatal("lost known ordinary refusal", err)
	}
	if _, err := x.ok("identity.public", map[string]any{}); err != nil || input.Len() != 0 {
		t.Fatal("valid correlated next response was not fully consumed", err, input.Len())
	}
	first, err := readFrame(&output)
	if err != nil || first["request_id"] != "tracker-1" {
		t.Fatal(first, err)
	}
	second, err := readFrame(&output)
	if err != nil || second["request_id"] != "tracker-2" || output.Len() != 0 {
		t.Fatal(second, err)
	}
}
func TestClientSuccessShapesHaveLivePositiveAndNegativeControls(t *testing.T) {
	operation := map[string]any{"status": "found", "protocol": "hn/0", "actor": strings.Repeat("a", 64), "operation": "original", "family": "issue", "type_id": nil, "request": testHostID(), "resource_id": testHostID(), "event_id": testHostID(), "result": testHostMutation(), "source_cut": testHostID(), "reason": nil}
	params := map[string]any{"actor": strings.Repeat("a", 64), "operation": "original", "family": "issue", "id": nil}
	for _, tc := range []struct {
		method  string
		data    map[string]any
		corrupt func(map[string]any)
	}{
		{"resource.mutate", testHostMutation(), func(v map[string]any) { v["replayed"] = "false" }},
		{"resource.operation", operation, func(v map[string]any) { v["operation"] = "different" }},
		{"changes.capture", testHostPage(), func(v map[string]any) { v["complete"] = false }},
		{"changes.page", testHostPage(), func(v map[string]any) { v["checkpoint"] = nil }},
		{"changes.since", testHostPage(), func(v map[string]any) { v["items"].([]any)[0].(map[string]any)["revision_ids"] = []any{"not-an-id"} }},
	} {
		t.Run(tc.method, func(t *testing.T) {
			r := testHostResponse(t, tc.data)
			if err := validateResponse(r, "tracker-1", tc.method, params, nil); err != nil {
				t.Fatal("valid typed control refused", err)
			}
			tc.corrupt(r["data"].(map[string]any))
			if err := validateResponse(r, "tracker-1", tc.method, params, nil); err == nil || err.Error() != "invalid_input" {
				t.Fatal("invalid typed control accepted", err)
			}
		})
	}
}
func TestClientWireJSONDomainControls(t *testing.T) {
	for _, raw := range []string{`{"x":1,"x":2}`, `{"nested":{"x":1,"x":2}}`, `{"x":"\ud800"}`, `{"x":"\udc00"}`, `{"x":-0}`, `{"x":1e0}`, `{"x":1.0}`, `{"x":9007199254740992}`, "{\"x\":\"\xff\"}"} {
		var wire bytes.Buffer
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
		wire.Write(prefix[:])
		wire.WriteString(raw)
		if _, err := readFrame(&wire); err == nil || err.Error() != "invalid_input" {
			t.Fatalf("invalid JSON domain accepted %q: %v", raw, err)
		}
	}
	for _, raw := range []string{`{"x":"\ud83d\ude00"}`, `{"x":"\\ud800"}`, `{"x":9007199254740991}`, `{"x":-9007199254740991}`} {
		if !validWireJSON([]byte(raw)) {
			t.Fatalf("valid domain refused %q", raw)
		}
	}
}

func TestClientIdentityCapabilitiesAndHelloControls(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	sum := sha256.Sum256(key)
	identity := map[string]any{"initialized": true, "actor": hex.EncodeToString(sum[:]), "name": "Actual native signer", "public_key": base64.RawStdEncoding.EncodeToString(key)}
	r := testHostResponse(t, identity)
	if err := validateResponse(r, "tracker-1", "identity.public", nil, nil); err != nil {
		t.Fatal("valid initialized public identity", err)
	}
	r["data"].(map[string]any)["actor"] = strings.Repeat("0", 64)
	if err := validateResponse(r, "tracker-1", "identity.public", nil, nil); err == nil {
		t.Fatal("actor/public-key mismatch accepted")
	}
	helloResponse := testHostResponse(t, testHostHello())
	hello := helloResponse["data"].(map[string]any)
	if !validHello(hello) {
		t.Fatal("valid startup hello refused")
	}
	binding := hello["hello"].(map[string]any)
	session := map[string]any{"protocol": protocol}
	for k, v := range binding {
		session[k] = v
	}
	capabilities := []any{}
	for _, method := range strings.Fields("capabilities.list identity.public resource.read resource.mutate resource.operation artifact.read artifact.mutate artifact.operation artifact.search artifact.type.read artifact.type.publish changes.capture changes.page changes.since") {
		capabilities = append(capabilities, map[string]any{"method": method, "declared": false, "granted": false, "enabled": true, "compatible": true, "availability": "observed", "recovery": "do_not_retry"})
	}
	unsupported := []any{}
	for _, surface := range strings.Fields("memory.mutate proposal review decide merge policy.mutate identity.mutate run runner audit automerge sync.apply migration.apply") {
		unsupported = append(unsupported, map[string]any{"surface": surface, "status": "unsupported", "recovery": "do_not_retry"})
	}
	data := map[string]any{"session": session, "capabilities": capabilities, "unsupported": unsupported}
	r = testHostResponse(t, data)
	if err := validateResponse(r, "tracker-1", "capabilities.list", nil, hello); err != nil {
		t.Fatal("full valid inventory refused", err)
	}
	r["data"].(map[string]any)["capabilities"].([]any)[0].(map[string]any)["method"] = "identity.public"
	if err := validateResponse(r, "tracker-1", "capabilities.list", nil, hello); err == nil {
		t.Fatal("duplicate/missing capability inventory accepted")
	}
	r = testHostResponse(t, data)
	r["data"].(map[string]any)["session"].(map[string]any)["repository_id"] = "sha256:" + strings.Repeat("c", 64)
	if err := validateResponse(r, "tracker-1", "capabilities.list", nil, hello); err == nil {
		t.Fatal("different startup repository accepted")
	}
	binding["extra"] = true
	if validHello(hello) {
		t.Fatal("unknown startup field accepted")
	}
}
func TestClientOpaqueTokenAndErrorMessageBounds(t *testing.T) {
	// Inherited public wire bounds: no narrower connector token/message limit.
	for _, tc := range []struct {
		name   string
		build  func(int) map[string]any
		method string
	}{
		{"checkpoint", func(n int) map[string]any {
			p := testHostPage()
			p["checkpoint"] = strings.Repeat("t", n)
			return testHostResponse(t, p)
		}, "changes.capture"},
		{"message", func(n int) map[string]any {
			r := testHostResponse(t, nil)
			r["ok"] = false
			e := testHostRefusal()
			e["message"] = strings.Repeat("m", n)
			r["error"] = e
			return r
		}, "identity.public"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateResponse(tc.build(4096), "tracker-1", tc.method, nil, nil); err != nil {
				t.Fatal("public equality bound refused", err)
			}
			if err := validateResponse(tc.build(4097), "tracker-1", tc.method, nil, nil); err == nil || err.Error() != "invalid_input" {
				t.Fatal("first-over public bound accepted", err)
			}
		})
	}
	// Native issue heads/comments and their combined revision list have no new
	// connector cap of 200/1024/1025. Only the public frame bounds this response.
	p := testHostPage()
	entry := p["items"].([]any)[0].(map[string]any)
	view := entry["value"].(map[string]any)
	ids := []any{testHostID()}
	comments := []any{}
	for i := 0; i < 1026; i++ {
		id := "sha256:" + fmt.Sprintf("%064x", i)
		ids = append(ids, id)
		comments = append(comments, map[string]any{"id": id, "actor": strings.Repeat("a", 64), "body": "comment"})
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].(string) < ids[j].(string) })
	entry["revision_ids"], view["comments"] = ids, comments
	if err := validateResponse(testHostResponse(t, p), "tracker-1", "changes.capture", nil, nil); err != nil {
		t.Fatal("legal complete revision set artificially capped", err)
	}
}

func TestClientClosedLookupStatusesAndCorrelationBounds(t *testing.T) {
	params := map[string]any{"actor": strings.Repeat("a", 64), "operation": "original", "family": "issue"}
	for _, status := range []string{"verified_absent", "uncertain"} {
		data := map[string]any{"status": status, "protocol": "hn/0", "actor": params["actor"], "operation": "original", "family": "issue", "type_id": nil, "request": nil, "resource_id": nil, "event_id": nil, "result": nil, "source_cut": testHostID(), "reason": nil}
		if status == "uncertain" {
			data["reason"] = "private_intent_pending"
		}
		r := testHostResponse(t, data)
		if err := validateResponse(r, "tracker-1", "resource.operation", params, nil); err != nil {
			t.Fatal("valid lookup status refused", status, err)
		}
		r["data"].(map[string]any)["result"] = testHostMutation()
		if err := validateResponse(r, "tracker-1", "resource.operation", params, nil); err == nil {
			t.Fatal("lookup invented result", status)
		}
	}
	r := testHostResponse(t, map[string]any{"initialized": false, "actor": nil, "name": nil, "public_key": nil})
	for _, size := range []int{128, 129} {
		id := strings.Repeat("r", size)
		r["request_id"] = id
		err := validateResponse(r, id, "identity.public", nil, nil)
		if size == 128 && err != nil || size == 129 && err == nil {
			t.Fatal("request ID equality/one-over mismatch", size, err)
		}
	}
	for _, size := range []int{200, 201} {
		p := testHostPage()
		items := make([]any, size)
		for i := range items {
			items[i] = map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": testHostID(), "revision_ids": []any{testHostID()}, "value": testHostView()}
		}
		p["items"] = items
		err := validateResponse(testHostResponse(t, p), "tracker-1", "changes.capture", nil, nil)
		if size == 200 && err != nil || size == 201 && err == nil {
			t.Fatal("page equality/one-over mismatch", size, err)
		}
	}
}

func TestClientClosedErrorInventoryMatchesPublicContract(t *testing.T) {
	// Independent literal mirror of host-v1 section 6: size and every member,
	// including shared code strings under their separate origins.
	expected := map[string][]string{
		"host":     {"denied_access", "incompatible_version", "unavailable_module", "unsupported_capability", "resource_limit", "cancelled", "uncertain_operation", "checkpoint_invalidated", "verification_failure", "invalid_input", "not_found", "repository_error"},
		"native":   {"actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision"},
		"artifact": {"invalid_input", "resource_limit", "verification_failure", "invalid_history", "incomplete_dependency", "unsupported_schema", "actor_stream_conflict", "operation_conflict", "stale_revision", "resource_conflict", "concurrent_update", "not_found", "repository_error", "cancelled", "uncertain_operation"},
	}
	if len(wireErrorCodes) != len(expected) {
		t.Fatal("closed error origins drifted")
	}
	for origin, want := range expected {
		actual := strings.Fields(wireErrorCodes[origin])
		sort.Strings(actual)
		sort.Strings(want)
		if !reflect.DeepEqual(actual, want) {
			t.Fatal("closed public error inventory drifted", origin, actual, want)
		}
		for _, code := range want {
			e := map[string]any{"origin": origin, "code": code, "message": "public refusal", "recovery": "do_not_retry", "operation": nil}
			if !wireError(e) {
				t.Fatal("known code not accepted under exact origin", origin, code)
			}
		}
	}
	for _, recovery := range []string{"do_not_retry", "retry_identical", "inspect_operation", "rescan_required"} {
		if !wireRecovery(recovery) {
			t.Fatal("closed recovery member missing", recovery)
		}
	}
	if wireRecovery("retry_everything") {
		t.Fatal("invented recovery accepted")
	}
	for _, origin := range []string{"host", "native", "artifact", "connector"} {
		e := map[string]any{"origin": origin, "code": "invented_code", "message": "public refusal", "recovery": "do_not_retry", "operation": nil}
		if wireError(e) {
			t.Fatal("unknown origin/code accepted", origin)
		}
	}
}

type trackerRoundTripFunc func(*http.Request) (*http.Response, error)

func (f trackerRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPendingUpdateSettlesBeforeFreshSourceOrNewOperation(t *testing.T) {
	for _, status := range []string{"found", "verified_absent", "uncertain", "recorded-result-before-baseline"} {
		t.Run(status, func(t *testing.T) {
			dir := t.TempDir()
			actor := strings.Repeat("a", 64)
			params := mutation(actor, "original-update-key", "revise", testHostID(), []string{testHostID()}, testHostState(), "")
			intent := &nativeIntent{Params: params}
			if status == "recorded-result-before-baseline" {
				intent.Result = testHostResponse(t, testHostMutation())["data"].(map[string]any)
			}
			j := &journal{Actor: actor, Native: map[string]*nativeIntent{"issue": {Result: map[string]any{"resource_id": testHostID()}}, "update-2": intent}, Baseline: map[string][]string{}, ExternalVersion: 1, NativeHead: testHostID()}
			lookup := map[string]any{"status": status, "protocol": "hn/0", "actor": actor, "operation": "original-update-key", "family": "issue", "type_id": nil, "request": nil, "resource_id": nil, "event_id": nil, "result": nil, "source_cut": testHostID(), "reason": nil}
			if status == "found" || status == "recorded-result-before-baseline" {
				lookup["status"] = "found"
				lookup["request"] = testHostID()
				lookup["resource_id"] = testHostID()
				lookup["event_id"] = testHostID()
				lookup["result"] = testHostMutation()
			}
			if status == "uncertain" {
				lookup["reason"] = "private_intent_pending"
			}
			var input, output bytes.Buffer
			if err := writeFrame(&input, map[string]any{"protocol": protocol, "request_id": "tracker-1", "ok": true, "data": lookup, "error": nil}); err != nil {
				t.Fatal(err)
			}
			if err := writeFrame(&input, map[string]any{"protocol": protocol, "request_id": "tracker-2", "ok": true, "data": testHostMutation(), "error": nil}); err != nil {
				t.Fatal(err)
			}
			reads := 0
			x := &client{c: config{Journal: filepath.Join(dir, "journal"), Report: filepath.Join(dir, "report")}, j: j, input: &input, output: &output, http: &http.Client{Transport: trackerRoundTripFunc(func(*http.Request) (*http.Response, error) {
				reads++
				return nil, fmt.Errorf("fresh source forbidden before pending settlement")
			})}}
			err := x.importItem(true)
			if reads != 0 || len(j.Native) != 2 {
				t.Fatal("pending update read fresh source or created a new operation", reads, j.Native)
			}
			first, frameErr := readFrame(&output)
			if frameErr != nil || first["method"] != "resource.operation" || first["params"].(map[string]any)["operation"] != "original-update-key" {
				t.Fatal("original operation was not looked up first", first, frameErr)
			}
			if status == "uncertain" {
				if err == nil || err.Error() != "native_uncertain" || output.Len() != 0 || intent.BaselineSaved || j.ExternalVersion != 1 {
					t.Fatal("uncertainty became absence/success or another writer", err)
				}
				return
			}
			if err != nil || !intent.BaselineSaved || j.ExternalVersion != 2 {
				t.Fatal("original update result/baseline not settled", err, j)
			}
			second, frameErr := readFrame(&output)
			if frameErr != nil || second["method"] != "resource.mutate" || output.Len() != 0 {
				t.Fatal("original exact replay missing", second, frameErr)
			}
			want, _ := json.Marshal(params)
			got, _ := json.Marshal(second["params"])
			if !bytes.Equal(want, got) {
				t.Fatal("recovery regenerated original actor/key/parents/content")
			}
		})
	}
}
