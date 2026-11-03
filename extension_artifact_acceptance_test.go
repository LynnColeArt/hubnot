package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	c "hubnot/internal/contribution"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The registered consumer speaks raw frames and writes only its own observation.
// It imports no host protocol client or production request/result types.
func TestExtensionArtifactChild(t *testing.T) {
	if os.Getenv("HN_WP18_FIXTURE_CONTROLS") == "1" {
		extensionArtifactFixtureControls(t)
		return
	}
	for i, arg := range os.Args {
		if arg == "--wp18-gate" {
			extensionArtifactGateMain(os.Args[i+1:])
			os.Exit(90)
		}
	}
	if os.Getenv("HN_WP18_CHILD") != "1" {
		return
	}
	at := 0
	for i, arg := range os.Args {
		if arg == "--" {
			at = i + 1
			break
		}
	}
	if at == 0 || len(os.Args) != at+2 {
		os.Exit(80)
	}
	raw, e := os.ReadFile(os.Args[at])
	if e != nil {
		os.Exit(81)
	}
	var requests []map[string]any
	if json.Unmarshal(raw, &requests) != nil {
		os.Exit(82)
	}
	_, _ = os.Stderr.WriteString("PRIVATE-WP18 bounded module diagnostics")
	hello := hostRead(os.Stdin)
	hostWrite(map[string]any{"protocol": "hn.extension/1", "ready": true})
	if os.WriteFile(os.Args[at+1]+".ready", []byte("ready"), 0600) != nil {
		os.Exit(84)
	}
	for {
		if _, e := os.Stat(os.Args[at+1] + ".go"); e == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	responses := []any{}
	for _, request := range requests {
		if request["request_id"] == "interrupted" && os.Getenv("HN_WP18_RECOVERY_DIR") != "" {
			extensionArtifactRecoveryPeer(request, os.Getenv("HN_WP18_RECOVERY_DIR"))
			os.Exit(86)
		}
		sent := time.Now()
		hostWrite(request)
		if request["request_id"] == "interrupted" {
			observation, _ := json.Marshal(map[string]any{"request_id": "interrupted", "send_utc": sent.UTC().Format(time.RFC3339Nano), "frame_written_after_ns": time.Since(sent).Nanoseconds(), "timeout_ms": request["timeout_ms"], "clock_boundary": "immediately before hostWrite; observation written after frame, before waiting; diagnostic I/O adds overhead"})
			if len(observation) > 1024 || os.WriteFile(os.Args[at+1]+".timing", observation, 0600) != nil {
				os.Exit(85)
			}
		}
		responses = append(responses, hostRead(os.Stdin))
	}

	raw, _ = json.Marshal(map[string]any{"hello": hello, "responses": responses})
	if os.WriteFile(os.Args[at+1], raw, 0600) != nil {
		os.Exit(83)
	}
	os.Exit(0)
}

type extensionArtifactFixture struct {
	*artifactCLIFixture
	input, output, module string
	expectedExternalRef   string
}

func extensionArtifactCLI(t *testing.T, grants ...string) *extensionArtifactFixture {
	t.Helper()
	binary := os.Getenv("HN_WP18_BINARY")
	if binary == "" {
		binary = extensionTestBinary(t)
	}
	t.Setenv("HN_WP05_BINARY", binary)
	f := &extensionArtifactFixture{artifactCLIFixture: artifactCLI(t), module: "example.adapters/custom"}
	if e := os.MkdirAll(f.home, 0700); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{filepath.Join(f.repo, ".hn", "connector-private"), filepath.Join(f.repo, ".git", "hn", "artifacts", "private", "journal")} {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte("PRIVATE-WP18 secret key credential recovery descriptor"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	f.overrides = map[string]string{"HN_WP18_CHILD": "1"}
	f.input = filepath.Join(f.home, "requests.json")
	f.output = filepath.Join(f.home, "responses.json")
	manifest := filepath.Join(f.home, "manifest.json")
	raw := []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.adapters/custom","module_version":"1","host_protocol":"hn.extension/1","capabilities":["capabilities.list","artifact.read","artifact.search","artifact.type.read","artifact.type.publish","artifact.mutate","artifact.operation"]}`)
	if e := os.WriteFile(manifest, raw, 0600); e != nil {
		t.Fatal(e)
	}
	child, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	f.call("", "", "extension", "register", "--manifest", manifest, "--executable", child, "--arg", "-test.run=^TestExtensionArtifactChild$", "--arg", "--", "--arg", f.input, "--arg", f.output, "--json")
	args := []string{"extension", "enable", f.module}
	for _, grant := range grants {
		args = append(args, "--grant", grant)
	}
	args = append(args, "--json")
	f.call("", "", args...)
	binaryBytes, e := os.ReadFile(binary)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("supplied host SHA256 %x", sha256.Sum256(binaryBytes))
	return f
}
func (f *extensionArtifactFixture) request(method string, params map[string]any, code string) map[string]any {
	f.t.Helper()
	raw, _ := json.Marshal([]any{map[string]any{"protocol": "hn.extension/1", "request_id": "raw-artifact", "method": method, "params": params}})
	if e := os.WriteFile(f.input, raw, 0600); e != nil {
		f.t.Fatal(e)
	}
	os.Remove(f.output + ".ready")
	os.Remove(f.output + ".go")
	cmd := exec.Command(f.binary, "extension", "run", f.module, "--json")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironmentWith(f.home, f.overrides)
	var cli bytes.Buffer
	cmd.Stdout = &cli
	cmd.Stderr = &cli
	if e := cmd.Start(); e != nil {
		f.t.Fatal(e)
	}
	defer func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, e := os.Stat(f.output + ".ready"); e == nil {
			break
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("consumer did not handshake: %s", cli.String())
		}
		time.Sleep(time.Millisecond)
	}
	before := f.protectedSnapshot()
	if e := os.WriteFile(f.output+".go", []byte("go"), 0600); e != nil {
		f.t.Fatal(e)
	}
	runErr := cmd.Wait()
	if runErr != nil {
		f.t.Fatalf("host run: %v %s", runErr, cli.String())
	}
	if code != "" || method != "artifact.mutate" && method != "artifact.type.publish" {
		if after := f.protectedSnapshot(); !reflect.DeepEqual(before, after) {
			for path, value := range after {
				if before[path] != value {
					f.t.Log("changed:", path)
				}
			}
			for path := range before {
				if _, ok := after[path]; !ok {
					f.t.Log("deleted:", path)
				}
			}
			f.t.Fatal("request changed signing/private intent/journal/public object/ref state")
		}
	}
	if bytes.Contains(cli.Bytes(), []byte("PRIVATE-WP18")) {
		f.t.Fatal("private module stderr leaked into host CLI output")
	}
	raw, e := os.ReadFile(f.output)
	if e != nil {
		f.t.Fatal(e)
	}
	var observed map[string]any
	if json.Unmarshal(raw, &observed) != nil {
		f.t.Fatalf("consumer JSON %s", raw)
	}
	response := observed["responses"].([]any)[0].(map[string]any)
	if code == "" {
		if response["ok"] != true {
			f.t.Fatalf("%s want success, got %s", method, raw)
		}
	} else {
		failure, _ := response["error"].(map[string]any)
		if response["ok"] != false || response["data"] != nil || failure["code"] != code {
			f.t.Fatalf("%s want %s, got %s", method, code, raw)
		}
	}
	for _, marker := range []string{"semantic_request", "expected_ref", "PRIVATE-WP18", "executable_digest", "registration_instance"} {
		if bytes.Contains(raw, []byte(marker)) {
			f.t.Fatalf("private marker %q in response", marker)
		}
	}
	return response
}
func TestExtensionArtifactTypeReadRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read")
	id := revisionEvent(f.publish(artifactStoreType, "declaration", ""))
	p := map[string]any{"id": id, "type_id": "example.test/note", "limit": 200, "cursor": nil}
	result := f.request("artifact.type.read", p, "")["data"].(map[string]any)
	if result["items"].([]any)[0].(map[string]any)["id"] != id {
		t.Fatal(result)
	}
}

func (f *extensionArtifactFixture) protectedSnapshot() map[string]string {
	values := f.snapshot()
	for path := range values {
		if strings.HasPrefix(path, f.output) || path == f.input || path == f.expectedExternalRef || strings.Contains(path, "/extensions/session-") || strings.Contains(path, "/extensions/owners-") {
			delete(values, path)
		}
	}
	return values
}

func extensionArtifactReadParams(id any, typeID any, mode string) map[string]any {
	return map[string]any{"id": id, "opaque_id": nil, "type_id": typeID, "limit": 200, "cursor": nil, "mode": mode, "opaque": false}
}
func extensionArtifactSearchParams(typeID any, shape any, literal any, regex any) map[string]any {
	return map[string]any{"type_id": typeID, "shape": shape, "literal": literal, "regex": regex, "limit": 200, "cursor": nil}
}
func extensionArtifactMutationParams(f *extensionArtifactFixture, kind string, record any, parents []string, payload map[string]any, key string) map[string]any {
	return map[string]any{"kind": kind, "record": record, "parents": parents, "payload": payload, "actor": f.actor, "operation": key}
}
func extensionArtifactEvent(response map[string]any) string {
	return response["data"].(map[string]any)["event_id"].(string)
}
func extensionArtifactItems(response map[string]any) []any {
	return response["data"].(map[string]any)["items"].([]any)
}

func TestExtensionArtifactModelsRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/*=read,create,revise,resolve,type.publish", "artifact:inventory.test/*=read,create,revise,resolve,type.publish")
	inventory := f.request("capabilities.list", map[string]any{}, "")["data"].(map[string]any)
	methods := []string{}
	for _, v := range inventory["capabilities"].([]any) {
		row := v.(map[string]any)
		methods = append(methods, row["method"].(string))
		if row["availability"] != "observed" {
			t.Fatal(row)
		}
	}
	want := []string{"artifact.mutate", "artifact.operation", "artifact.read", "artifact.search", "artifact.type.publish", "artifact.type.read", "capabilities.list", "changes.capture", "changes.page", "changes.since", "identity.public", "resource.mutate", "resource.operation", "resource.read"}
	if !reflect.DeepEqual(methods, want) {
		t.Fatal(methods)
	}
	unsupported := []string{}
	for _, v := range inventory["unsupported"].([]any) {
		row := v.(map[string]any)
		if row["status"] != "unsupported" || row["recovery"] != "do_not_retry" {
			t.Fatal(row)
		}
		unsupported = append(unsupported, row["surface"].(string))
	}
	if !reflect.DeepEqual(unsupported, []string{"memory.mutate", "proposal", "review", "decide", "merge", "policy.mutate", "identity.mutate", "run", "runner", "audit", "automerge", "sync.apply", "migration.apply"}) {
		t.Fatal(unsupported)
	}
	for _, tc := range []struct{ name, shape string }{{"example.test/note", "document"}, {"example.test/list", "list"}, {"example.test/ticket", "ticket"}, {"inventory.test/part", "document"}} {
		t.Run(tc.name, func(t *testing.T) {
			var declaration map[string]any
			raw := strings.ReplaceAll(strings.ReplaceAll(artifactStoreType, "example.test/note", tc.name), `"shape":"document"`, `"shape":"`+tc.shape+`"`)
			if json.Unmarshal([]byte(raw), &declaration) != nil {
				t.Fatal("declaration")
			}
			publish := map[string]any{"parents": declaration["parents"], "payload": declaration["payload"], "actor": f.actor, "operation": tc.name + "-type"}
			typ := extensionArtifactEvent(f.request("artifact.type.publish", publish, ""))
			repeated := f.request("artifact.type.publish", publish, "")["data"].(map[string]any)
			if repeated["event_id"] != typ || repeated["replayed"] != true {
				t.Fatal(repeated)
			}
			p := revisionPayload(typ, "雪 e\u0301\n$(touch /must-not-run) <script>inert</script> needle")
			p["type_id"], p["shape"] = tc.name, tc.shape
			params := extensionArtifactMutationParams(f, "record.open", nil, []string{}, p, tc.name+"-open")
			id := extensionArtifactEvent(f.request("artifact.mutate", params, ""))
			again := f.request("artifact.mutate", params, "")["data"].(map[string]any)
			if again["event_id"] != id || again["replayed"] != true {
				t.Fatal(again)
			}
			for _, mode := range []string{"view", "heads", "history", "inspect"} {
				items := extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(id, tc.name, mode), ""))
				if len(items) != 1 || items[0].(map[string]any)["id"] != id {
					t.Fatal(mode, items)
				}
			}
			listed := extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(nil, tc.name, "view"), ""))
			if len(listed) != 1 || listed[0].(map[string]any)["state"].(map[string]any)["content"] != p["content"] {
				t.Fatal(listed)
			}
			for _, search := range []map[string]any{extensionArtifactSearchParams(tc.name, tc.shape, "needle", nil), extensionArtifactSearchParams(tc.name, tc.shape, nil, "(?m)needle$")} {
				items := extensionArtifactItems(f.request("artifact.search", search, ""))
				if len(items) != 1 || items[0].(map[string]any)["id"] != id {
					t.Fatal(items)
				}
			}
			typeItems := extensionArtifactItems(f.request("artifact.type.read", map[string]any{"id": typ, "type_id": tc.name, "limit": 200, "cursor": nil}, ""))
			if len(typeItems) != 1 || typeItems[0].(map[string]any)["id"] != typ {
				t.Fatal(typeItems)
			}
			lookup := map[string]any{"type_id": tc.name, "actor": f.actor, "operation": tc.name + "-open"}
			operation := f.request("artifact.operation", lookup, "")["data"].(map[string]any)
			if operation["status"] != "found" || operation["event_id"] != id || operation["protocol"] != "hn-artifact/1" {
				t.Fatal(operation)
			}
			lookup["operation"] = "absent-key"
			absent := f.request("artifact.operation", lookup, "")["data"].(map[string]any)
			if absent["status"] != "verified_absent" || absent["event_id"] != nil || absent["reason"] != nil {
				t.Fatal(absent)
			}
			changed := revisionPayload(typ, "changed replay")
			changed["type_id"], changed["shape"] = tc.name, tc.shape
			f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, changed, tc.name+"-open"), "operation_conflict")
			next := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", id, []string{id}, changed, tc.name+"-revise"), ""))
			f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", id, []string{id}, changed, tc.name+"-stale"), "stale_revision")
			// A singleton resolve is a valid explicit resolution of every current head.
			resolved := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.resolve", id, []string{next}, changed, tc.name+"-resolve"), ""))
			newer := map[string]any{}
			json.Unmarshal([]byte(raw), &newer)
			newer["payload"].(map[string]any)["schema_version"] = 2
			publish2 := map[string]any{"parents": []string{typ}, "payload": newer["payload"], "actor": f.actor, "operation": tc.name + "-type2"}
			typ2 := extensionArtifactEvent(f.request("artifact.type.publish", publish2, ""))
			converted := revisionPayload(typ2, "converted")
			converted["type_id"], converted["shape"], converted["conversion_sources"] = tc.name, tc.shape, []string{resolved}
			f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.convert", id, []string{resolved}, converted, tc.name+"-convert"), "")
			history := extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(id, nil, "history"), ""))
			if len(history) != 4 {
				t.Fatal(history)
			}
		})
	}
}

func TestExtensionArtifactGrantRefusalsRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read,create", "artifact:inventory.test/part=read,revise")
	typ := revisionEvent(f.publish(artifactStoreType, "native-type", ""))
	secretType := revisionEvent(f.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", "private.test/hidden"), "hidden-type", ""))
	hidden := revisionPayload(secretType, "UNGRANTED-SAME-STREAM-BODY")
	hidden["type_id"] = "private.test/hidden"
	hiddenID := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, hidden), "hidden-open", ""))
	p := revisionPayload(typ, "allowed")
	id := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, p, "granted-create"), ""))
	for _, kind := range []string{"record.revise", "record.resolve", "record.convert"} {
		f.request("artifact.mutate", extensionArtifactMutationParams(f, kind, id, []string{id}, p, "deny-"+kind), "denied_access")
	}
	f.request("artifact.type.publish", map[string]any{"parents": []string{}, "payload": map[string]any{"type_id": "example.test/note", "schema_version": 1, "shape": "document", "schema": map[string]any{"language": "hn.schema/1", "root": map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}, "additionalProperties": false}}}, "actor": f.actor, "operation": "deny-publish"}, "denied_access")
	for _, mode := range []string{"view", "heads", "history", "inspect"} {
		result := f.request("artifact.read", extensionArtifactReadParams(id, nil, mode), "")
		raw, _ := json.Marshal(result)
		if bytes.Contains(raw, []byte("UNGRANTED-SAME-STREAM-BODY")) {
			t.Fatal("ungranted prefix body exported", string(raw))
		}
		// An authorized immutable envelope must retain its signed previous ID, even
		// when that previous artifact's body is outside the read grant. Reject body
		// export or an extra selected item, not its legitimate provenance reference.
		for _, value := range extensionArtifactItems(result) {
			item := value.(map[string]any)
			if item["id"] == hiddenID {
				t.Fatal("ungranted prefix item exported", item)
			}
		}
		f.request("artifact.read", extensionArtifactReadParams(hiddenID, nil, mode), "denied_access")
	}
	f.request("artifact.type.read", map[string]any{"id": secretType, "type_id": nil, "limit": 200, "cursor": nil}, "denied_access")
	f.request("artifact.search", extensionArtifactSearchParams("private.test/hidden", nil, "", nil), "denied_access")
	selected := extensionArtifactItems(f.request("artifact.search", extensionArtifactSearchParams(nil, nil, "", nil), ""))
	if len(selected) != 1 || selected[0].(map[string]any)["id"] != id {
		t.Fatal(selected)
	}
	f.request("artifact.operation", map[string]any{"type_id": "example.test/note", "actor": f.actor, "operation": "hidden-open"}, "operation_conflict")
	f.request("artifact.read", extensionArtifactReadParams(id, "inventory.test/part", "view"), "invalid_input")
	// A granted destination cannot launder an ungranted source namespace.
	destinationType := revisionEvent(f.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", "inventory.test/part"), "destination-type", ""))
	conversion := revisionPayload(destinationType, "invalid cross-type")
	conversion["type_id"] = "inventory.test/part"
	conversion["conversion_sources"] = []string{hiddenID}
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.convert", hiddenID, []string{hiddenID}, conversion, "cross-source"), "denied_access")
	for _, bad := range []struct {
		name    string
		payload map[string]any
		actor   string
		code    string
	}{
		{"missing-schema", revisionPayload("sha256:"+strings.Repeat("d", 64), "x"), f.actor, "incomplete_dependency"},
		{"wrong-actor", p, strings.Repeat("b", 64), "actor_mismatch"},
	} {
		params := extensionArtifactMutationParams(f, "record.open", nil, []string{}, bad.payload, bad.name)
		params["actor"] = bad.actor
		f.request("artifact.mutate", params, bad.code)
	}
}

func TestExtensionArtifactSearchBoundsAndCursorRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/*=read,create,revise")
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	pattern := strings.Repeat("a", 4094) // exactly 4096 compiled instructions
	overhead := 2*71 + len("example.test/note") + len("document") + len(f.actor) + len(schema) + len("Revision data") + len("{}")
	content := pattern + strings.Repeat("b", 16384-overhead-len(pattern))
	id := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, content)), "work-at", ""))
	q := extensionArtifactSearchParams("example.test/note", "document", nil, pattern)
	q["limit"] = 1
	if len(extensionArtifactItems(f.request("artifact.search", q, ""))) != 1 {
		t.Fatal("at-bound search missing")
	}
	f.request("artifact.search", extensionArtifactSearchParams("example.test/note", nil, nil, strings.Repeat("😀", 1024)), "")
	f.request("artifact.search", extensionArtifactSearchParams("example.test/note", nil, nil, strings.Repeat("a", 4095)), "resource_limit")
	f.request("artifact.search", extensionArtifactSearchParams("example.test/note", nil, nil, "["), "invalid_input")
	revisionCall(f.artifactCLIFixture, "revise", revisionInput(t, id, []string{id}, revisionPayload(schema, content+"b")), "work-over", "")
	f.request("artifact.search", q, "resource_limit")
	// An unauthorized or differently shaped expensive selection must not be charged.
	for _, tc := range []struct{ name, shape string }{{"ungranted.test/hidden", "document"}, {"example.test/ticket", "ticket"}} {
		typ := revisionEvent(f.publish(strings.ReplaceAll(strings.ReplaceAll(artifactStoreType, "example.test/note", tc.name), `"shape":"document"`, `"shape":"`+tc.shape+`"`), tc.name+"-type", ""))
		p := revisionPayload(typ, strings.Repeat("b", 20000))
		p["type_id"], p["shape"] = tc.name, tc.shape
		revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, p), tc.name+"-large", "")
	}
	// Restrict to another authorized exact type, then prove cursor invalidation
	// precedes charging newly excessive selected work on that same continuation.
	otherType := "example.test/small"
	typ := revisionEvent(f.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", otherType), "small-type", ""))
	p := revisionPayload(typ, pattern)
	p["type_id"] = otherType
	for _, key := range []string{"small-1", "small-2"} {
		revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, p), key, "")
	}
	search := extensionArtifactSearchParams(otherType, "document", nil, pattern)
	search["limit"] = 1
	first := f.request("artifact.search", search, "")["data"].(map[string]any)
	cursor := first["next_cursor"]
	if cursor == nil {
		t.Fatal(first)
	}
	search["cursor"], search["limit"] = cursor, 200
	if len(extensionArtifactItems(f.request("artifact.search", search, ""))) != 1 {
		t.Fatal("varying page limit skipped a match")
	}
	search["shape"] = "list"
	f.request("artifact.search", search, "checkpoint_invalidated")
	search["shape"] = "document"
	p["content"] = strings.Repeat("b", 20000)
	revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, p), "small-now-expensive", "")
	f.request("artifact.search", search, "checkpoint_invalidated")
	search["cursor"] = nil
	f.request("artifact.search", search, "resource_limit")
}

func TestExtensionArtifactFullResponseAtAndOverRawCLI(t *testing.T) {
	// Each signed envelope is legal and below 1 MiB; the whole host response,
	// including request ID and envelope, is exactly 8 MiB or one byte over.
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprint(extra), func(t *testing.T) {
			f := extensionArtifactCLI(t, "artifact:example.test/note=read")
			typ := revisionEvent(f.publish(artifactStoreType, "type", ""))
			items := []any{}
			typeItem := extensionArtifactItems(f.request("artifact.type.read", map[string]any{"id": typ, "type_id": nil, "limit": 200, "cursor": nil}, ""))[0]
			items = append(items, typeItem)
			var sample map[string]any
			for i := 0; i < 8; i++ {
				id := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, strings.Repeat("x", 940000))), fmt.Sprintf("bound-%d", i), ""))
				sample = extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(id, nil, "inspect"), ""))[0].(map[string]any)
				items = append(items, sample)
			}
			encoded, _ := json.Marshal(sample)
			var prospective map[string]any
			json.Unmarshal(encoded, &prospective)
			envelope := prospective["envelope"].(map[string]any)
			envelope["sequence"] = 10
			envelope["payload"].(map[string]any)["content"] = ""
			// Every remaining variable identity has the same fixed encoded width.
			items = append(items, prospective)
			predicted := map[string]any{"protocol": "hn.extension/1", "request_id": "raw-artifact", "ok": true, "data": map[string]any{"items": items, "source_cut": strings.Repeat("x", 71), "next_cursor": nil}, "error": nil}
			raw, _ := json.Marshal(predicted)
			content := strings.Repeat("x", 8388608-len(raw)+extra)
			last := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, content)), "bound-8", ""))
			_ = last
			code := ""
			if extra == 1 {
				code = "resource_limit"
			}
			response := f.request("artifact.read", extensionArtifactReadParams(nil, "example.test/note", "inspect"), code)
			if extra == 0 {
				actual, _ := json.Marshal(response)
				if len(actual) != 8388608 || len(extensionArtifactItems(response)) != 10 {
					t.Fatalf("actual full frame %d, want 8388608", len(actual))
				}
			}
			// The same complete object remains obtainable through a narrower selection.
			if len(extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(last, nil, "inspect"), ""))) != 1 {
				t.Fatal("small explicit read failed")
			}
		})
	}
}

func TestExtensionArtifactOpaqueRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:opaque=raw.inspect", "artifact:example.test/note=read")
	typ := revisionEvent(f.publish(artifactStoreType, "type", ""))
	smallCuts, smallIDs, smallBodies := []string{}, []string{}, []string{}
	for _, test := range []struct {
		size int
		fill byte
	}{{32, 'x'}, {32, 'y'}, {8388608, 'x'}} {
		n := test.size
		body := bytes.Repeat([]byte{test.fill}, n)
		object := exchangeObject("blob", body)
		bundle, _ := json.Marshal(exchangeBundle(object))
		imported := f.call(string(bundle), "", "artifact", "import", "--input", "-", "--preserve-opaque", "--json")["data"].(map[string]any)
		opaqueID := fmt.Sprintf("sha256:%x", sha256.Sum256(append([]byte("blob\x00"), body...)))
		raw, _ := json.Marshal(imported)
		if !bytes.Contains(raw, []byte(opaqueID)) {
			t.Fatal("preservation identity", string(raw))
		}
		p := extensionArtifactReadParams(nil, nil, "inspect")
		p["opaque"], p["opaque_id"] = true, opaqueID
		code := ""
		if n == 8388608 {
			code = "resource_limit"
		}
		response := f.request("artifact.read", p, code)
		if n == 32 {
			data := response["data"].(map[string]any)
			item := extensionArtifactItems(response)[0].(map[string]any)
			if item["id"] != nil || item["envelope"] != nil || item["signature"] != nil || item["validation"] != "opaque" || item["raw"].(map[string]any)["data"] != object.Data || data["next_cursor"] != nil {
				t.Fatal(item)
			}
			smallCuts = append(smallCuts, data["source_cut"].(string))
			smallIDs = append(smallIDs, opaqueID)
			smallBodies = append(smallBodies, object.Data)
			p["cursor"] = "forged-unissued"
			f.request("artifact.read", p, "checkpoint_invalidated")
			p["cursor"] = ""
			f.request("artifact.read", p, "checkpoint_invalidated")
			fabricated, _ := json.Marshal(map[string]any{"version": 1, "query": "sha256:" + strings.Repeat("0", 64), "cut": data["source_cut"], "offset": 0})
			p["cursor"] = base64.RawURLEncoding.EncodeToString(fabricated)
			f.request("artifact.read", p, "checkpoint_invalidated")
		}
	}
	if len(smallCuts) != 2 || smallCuts[0] != smallCuts[1] || smallCuts[0] != extensionArtifactExpectedCut(t, f, nil, nil, false) || smallIDs[0] == smallIDs[1] || smallBodies[0] == smallBodies[1] {
		t.Fatal("opaque Cut confused empty projection with content identity", smallCuts, smallIDs)
	}
	// An accepted signed identity and an arbitrary ODB object do not select the
	// explicit public quarantine index.
	p := extensionArtifactReadParams(nil, nil, "inspect")
	p["opaque"], p["opaque_id"] = true, typ
	f.request("artifact.read", p, "not_found")
	p["opaque_id"] = "sha256:" + strings.Repeat("d", 64)
	f.request("artifact.read", p, "not_found")
	// A prior Cut/result never bypasses current explicit index membership.
	if err := os.Remove(filepath.Join(f.repo, ".git", "hn", "artifact-opaque", "public.json")); err != nil {
		t.Fatal(err)
	}
	p["opaque_id"] = smallIDs[0]
	f.request("artifact.read", p, "not_found")
}

func TestExtensionArtifactInterruptedRecoveryRawCLI(t *testing.T) {
	t.Run("dispatch-reader-deadline", extensionArtifactReaderDeadline)
	for _, stage := range []string{"prepared", "lost-ack"} {
		t.Run(stage, func(t *testing.T) {
			f := extensionArtifactCLI(t, "artifact:example.test/note=read,create,revise")
			typ := revisionEvent(f.publish(artifactStoreType, "type", ""))
			id := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "opening")), "opening", ""))
			payload := revisionPayload(typ, "immutable host revision")
			params := extensionArtifactMutationParams(f, "record.revise", id, []string{id}, payload, "interrupted")
			beforeRef := f.git("rev-parse", artifactRef(f.actor))
			observed := extensionArtifactRecoveryRun(t, f, stage, params, 20000, beforeRef)
			if observed.failure != nil {
				t.Fatal(observed.failure)
			}
			if (f.git("rev-parse", artifactRef(f.actor)) != beforeRef) != (stage == "lost-ack") {
				t.Fatal("publication stage differed")
			}
			lookup := map[string]any{"type_id": "example.test/note", "actor": f.actor, "operation": "interrupted"}
			result := f.request("artifact.operation", lookup, "")["data"].(map[string]any)
			if stage == "prepared" {
				if result["status"] != "uncertain" || result["reason"] != "private_intent_pending" || result["result"] != nil {
					t.Fatal(result)
				}
			} else {
				if result["status"] != "found" || result["event_id"] == nil || observed.candidate != f.git("rev-parse", artifactRef(f.actor)) || result["event_id"] != extensionArtifactPublishedEvent(t, f, observed.candidate) {
					t.Fatal(result)
				}
			}
			// A changed exact immutable intent refuses even before a successful retry.
			changed := revisionPayload(typ, "changed host retry")
			f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", id, []string{id}, changed, "interrupted"), "operation_conflict")
			replay := f.request("artifact.mutate", params, "")["data"].(map[string]any)
			if stage == "lost-ack" && (replay["replayed"] != true || replay["event_id"] != result["event_id"]) {
				t.Fatal(replay, result)
			}
			// Lookup pins the original actor after rotation; it never initializes or
			// substitutes the active identity, and old-actor writes refuse.
			rotate := exec.Command(f.binary, "identity", "rotate", "--name", "Replacement")
			rotate.Dir = f.repo
			rotate.Env = operationalTestEnvironment(f.home)
			if raw, e := rotate.CombinedOutput(); e != nil {
				t.Fatalf("rotate %v %s", e, raw)
			}
			found := f.request("artifact.operation", lookup, "")["data"].(map[string]any)
			if found["status"] != "found" || found["actor"] != f.actor || found["event_id"] != replay["event_id"] {
				t.Fatal(found)
			}
			f.request("artifact.mutate", params, "actor_mismatch")
			for _, name := range []string{"active", "identity.json"} {
				if e := os.Remove(filepath.Join(f.repo, ".git", "hn", name)); e != nil && !os.IsNotExist(e) {
					t.Fatal(e)
				}
			}
			f.request("artifact.operation", lookup, "")
			f.request("artifact.read", extensionArtifactReadParams(id, nil, "view"), "")
			f.request("artifact.type.read", map[string]any{"id": typ, "type_id": nil, "limit": 200, "cursor": nil}, "")
			f.request("artifact.search", extensionArtifactSearchParams("example.test/note", nil, "immutable", nil), "")
			f.request("artifact.mutate", params, "repository_error")
		})
	}
}

// These finite rendezvous records contain public argv/identities and clocks only.
// They live outside the protected repository/home and never carry stdin or env.
type extensionArtifactGate struct {
	Stage    string `json:"stage"`
	Token    string `json:"token"`
	Git      string `json:"git"`
	Lexical  string `json:"lexical"`
	Physical string `json:"physical"`
	Ref      string `json:"ref"`
	Old      string `json:"old"`
}
type extensionArtifactStage struct {
	Stage     string   `json:"stage"`
	Token     string   `json:"token"`
	Argv      []string `json:"argv"`
	PID       int      `json:"pid"`
	NS        int64    `json:"unix_ns"`
	GitPID    int      `json:"git_pid"`
	GitExit   *int     `json:"git_exit"`
	GitWaitNS int64    `json:"git_wait_ns"`
}

// Each fixed record has one owner; existing final or temporary files are errors.
func extensionArtifactAtomic(path string, value any) error {
	return extensionArtifactAtomicUsing(path, value, (*os.File).Write, (*os.File).Close, os.Rename)
}

// The narrow I/O seam permits deterministic write/close/rename failure controls.
func extensionArtifactAtomicUsing(path string, value any, write func(*os.File, []byte) (int, error), closeFile func(*os.File) error, rename func(string, string) error) error {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 6144 {
		return fmt.Errorf("fixture record bound: %v", err)
	}
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("duplicate fixture record")
	}
	file, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(path + ".tmp")
	n, writeErr := write(file, raw)
	closeErr := closeFile(file)
	if writeErr != nil || n != len(raw) || closeErr != nil {
		return fmt.Errorf("fixture write/close: %v/%v", writeErr, closeErr)
	}
	return rename(path+".tmp", path)
}
func extensionArtifactReadRecord(path string, value any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, 6145))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(raw) > 6144 {
		return fmt.Errorf("fixture read bound/close")
	}
	// Reject duplicate JSON keys as well as unknown fields and trailing records.
	var keys map[string]json.RawMessage
	if err = json.Unmarshal(raw, &keys); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if _, err = decoder.Token(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, e := decoder.Token()
		if e != nil {
			return e
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return fmt.Errorf("duplicate fixture key")
		}
		seen[name] = true
		var skip json.RawMessage
		if e = decoder.Decode(&skip); e != nil {
			return e
		}
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err = decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return fmt.Errorf("extra fixture record")
	}
	return nil
}
func extensionArtifactEvidenceBound(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	total := int64(0)
	allowed := map[string]bool{"config.json": true, "git": true, "claim": true, "gate.json": true, "request.json": true, "witness.json": true, "peer.json": true, "cancel.json": true, "term.json": true, "failure.json": true}
	for _, file := range files {
		if !allowed[file.Name()] {
			return fmt.Errorf("unexpected/incomplete fixture evidence")
		}
		info, err := file.Info()
		if err != nil {
			return err
		}
		if info.IsDir() && file.Name() == "claim" {
			continue
		}
		if !info.Mode().IsRegular() || (file.Name() != "git" && info.Mode().Perm() != 0600) {
			return fmt.Errorf("fixture evidence mode")
		}
		total += info.Size()
	}
	if total > 16384 {
		return fmt.Errorf("combined fixture evidence exceeds 16384")
	}
	return nil
}
func extensionArtifactOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, ch := range value {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	return true
}
func extensionArtifactSelected(cfg extensionArtifactGate, argv []string) bool {
	if cfg.Stage == "dispatch-reader-deadline" {
		return reflect.DeepEqual(argv, []string{"--git-dir=" + cfg.Physical, "cat-file", "--batch-command"})
	}
	if cfg.Stage == "prepared" {
		return reflect.DeepEqual(argv, []string{"--git-dir=" + cfg.Physical, "hash-object", "-w", "--stdin"})
	}
	return cfg.Stage == "lost-ack" && len(argv) > 1 && argv[0] == "--git-dir="+cfg.Physical && argv[1] == "update-ref"
}
func extensionArtifactValidateStage(cfg extensionArtifactGate, record extensionArtifactStage) error {
	raw, err := json.Marshal(record.Argv)
	if err != nil || len(raw) > 4096 || record.Token != cfg.Token || record.Stage != cfg.Stage || record.PID <= 0 || record.NS <= 0 || !extensionArtifactSelected(cfg, record.Argv) {
		return fmt.Errorf("invalid selected gate")
	}
	if cfg.Stage == "lost-ack" {
		a := record.Argv
		if len(a) != 7 || a[2] != "-m" || a[3] != "hn: append record.revise" || a[4] != cfg.Ref || a[6] != cfg.Old || !extensionArtifactOID(a[5]) || len(a[5]) != len(cfg.Old) || a[5] == cfg.Old {
			return fmt.Errorf("invalid source-issued update-ref")
		}
	}
	return nil
}
func extensionArtifactGateMain(args []string) {
	if len(args) < 2 {
		os.Exit(90)
	}
	dir := args[0]
	var cfg extensionArtifactGate
	if extensionArtifactReadRecord(filepath.Join(dir, "config.json"), &cfg) != nil {
		os.Exit(91)
	}
	argv := args[1:]
	if !extensionArtifactSelected(cfg, argv) {
		if syscall.Exec(cfg.Git, append([]string{cfg.Git}, argv...), os.Environ()) != nil {
			os.Exit(92)
		}
		return
	}
	record := extensionArtifactStage{Stage: cfg.Stage, Token: cfg.Token, Argv: argv, PID: os.Getpid(), NS: time.Now().UnixNano()}
	if extensionArtifactValidateStage(cfg, record) != nil || os.Mkdir(filepath.Join(dir, "claim"), 0700) != nil {
		os.Exit(93)
	}
	name := "gate.json"
	if cfg.Stage == "lost-ack" {
		name = "request.json"
	}
	if extensionArtifactAtomic(filepath.Join(dir, name), record) != nil {
		os.Exit(94)
	}
	// CommandContext owns this process. It never starts real Git or a sleep child.
	time.Sleep(30 * time.Second)
	_ = extensionArtifactAtomic(filepath.Join(dir, "failure.json"), map[string]any{"safety": true, "pid": os.Getpid()})
	os.Exit(95)
}
func extensionArtifactFrame(value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
	frame := append(prefix[:], raw...)
	n, err := os.Stdout.Write(frame)
	if err != nil || n != len(frame) {
		return fmt.Errorf("incomplete raw frame")
	}
	return nil
}
func extensionArtifactRecoveryPeer(request map[string]any, dir string) {
	var cfg extensionArtifactGate
	if extensionArtifactReadRecord(filepath.Join(dir, "config.json"), &cfg) != nil {
		os.Exit(86)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	before := time.Now().UnixNano()
	if extensionArtifactFrame(request) != nil {
		os.Exit(87)
	}
	after := time.Now().UnixNano()
	if extensionArtifactAtomic(filepath.Join(dir, "peer.json"), map[string]any{"before_ns": before, "after_ns": after, "pid": os.Getpid()}) != nil {
		os.Exit(88)
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	poll := time.NewTicker(2 * time.Millisecond)
	defer poll.Stop()
	sent := false
	for {
		select {
		case <-signals:
			if extensionArtifactAtomic(filepath.Join(dir, "term.json"), map[string]any{"unix_ns": time.Now().UnixNano(), "pid": os.Getpid()}) != nil {
				os.Exit(89)
			}
			os.Exit(0)
		case <-timer.C:
			_ = extensionArtifactAtomic(filepath.Join(dir, "failure.json"), map[string]any{"safety": true, "pid": os.Getpid()})
			os.Exit(90)
		case <-poll.C:
			if cfg.Stage == "dispatch-reader-deadline" || sent {
				continue
			}
			name := "gate.json"
			if cfg.Stage == "lost-ack" {
				name = "witness.json"
			}
			var witness extensionArtifactStage
			err := extensionArtifactReadRecord(filepath.Join(dir, name), &witness)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil || extensionArtifactValidateStage(cfg, witness) != nil || (cfg.Stage == "lost-ack" && (witness.GitPID <= 0 || witness.GitExit == nil || *witness.GitExit != 0 || witness.GitWaitNS <= 0)) {
				os.Exit(91)
			}
			start := time.Now().UnixNano()
			if extensionArtifactFrame(map[string]any{"protocol": "hn.extension/1", "cancel": "interrupted"}) != nil {
				os.Exit(92)
			}
			if extensionArtifactAtomic(filepath.Join(dir, "cancel.json"), map[string]any{"before_ns": start, "after_ns": time.Now().UnixNano(), "count": 1}) != nil {
				os.Exit(93)
			}
			sent = true
		}
	}
}

// Only the parent owns the delegated direct child. Both Wait goroutines have a
// single consumer through a closed completion channel, and are joined on return.
func extensionArtifactDelegate(cfg extensionArtifactGate, record extensionArtifactStage, cwd string, frozen []string, hostDone <-chan struct{}, witness string, claimed *bool, afterStart func(*exec.Cmd)) error {
	if err := extensionArtifactValidateStage(cfg, record); err != nil {
		return err
	}
	if record.GitPID != 0 || record.GitExit != nil || record.GitWaitNS != 0 {
		return fmt.Errorf("request falsely claimed Git settlement")
	}
	if *claimed {
		return fmt.Errorf("duplicate delegated request")
	}
	*claimed = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	select {
	case <-hostDone:
		return fmt.Errorf("host exited before delegate start")
	default:
	}
	env := []string{}
	for _, entry := range frozen {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(key, "GIT_") {
			env = append(env, entry)
		}
	}
	env = append(env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1")
	cmd := exec.CommandContext(ctx, cfg.Git, record.Argv...)
	cmd.Dir = cwd
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(nil)
	var out bytes.Buffer
	cmd.Stdout = &projectBoundWriter{w: &out, left: 4096}
	cmd.Stderr = io.Discard
	// Ownership/cancellation is installed before Start; successful Start has one Wait.
	done := make(chan struct{})
	started := false
	var waitErr error
	defer func() {
		cancel()
		if started {
			<-done
		}
	}()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("delegate start failed")
	}
	started = true
	go func() { waitErr = cmd.Wait(); close(done) }()
	if afterStart != nil {
		afterStart(cmd)
	}
	select {
	case <-hostDone:
		cancel()
		<-done
		return fmt.Errorf("host exited during delegate")
	case <-done:
	}
	if waitErr != nil {
		return fmt.Errorf("delegate nonzero/output failure")
	}
	select {
	case <-hostDone:
		return fmt.Errorf("host exited before witness")
	default:
	}
	exit := cmd.ProcessState.ExitCode()
	record.GitPID = cmd.Process.Pid
	record.GitExit = &exit
	record.GitWaitNS = time.Now().UnixNano()
	record.NS = record.GitWaitNS
	if err := extensionArtifactAtomic(witness, record); err != nil {
		return fmt.Errorf("delegate witness failed: %w", err)
	}
	select {
	case <-hostDone:
		return fmt.Errorf("host exited at witness")
	default:
	}
	return nil
}

type extensionArtifactRecoveryResult struct {
	failure   error
	candidate string
}

func (r *extensionArtifactRecoveryResult) fail(err error) { r.failure = errors.Join(r.failure, err) }

func extensionArtifactRecoveryRun(t *testing.T, f *extensionArtifactFixture, stage string, params map[string]any, timeout int, before string) (result extensionArtifactRecoveryResult) {
	t.Helper()
	dir := t.TempDir()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	git, err = filepath.Abs(git)
	if err != nil {
		t.Fatal(err)
	}
	lexical := f.git("rev-parse", "--absolute-git-dir") // requireGitRepository's actual resolution.
	physical, err := filepath.EvalSymlinks(lexical)
	if err != nil {
		t.Fatal(err)
	}
	if physical != lexical {
		t.Fatal("fixture private Git directory unexpectedly aliased", lexical, physical)
	}
	// This exact fixture must not give the delegated Git an unowned hook child.
	hookConfig := exec.Command(git, "--git-dir="+physical, "config", "--get", "core.hooksPath")
	hookConfig.Dir = f.repo
	hookConfig.Env = operationalTestEnvironment(f.home)
	if raw, e := hookConfig.Output(); e == nil || len(raw) != 0 {
		t.Fatal("alternate hooks configuration")
	} else if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatal("hook configuration inspection", e)
	}
	if hook, e := os.Stat(filepath.Join(physical, "hooks", "reference-transaction")); e == nil && hook.Mode().Perm()&0111 != 0 {
		t.Fatal("executable reference-transaction hook")
	} else if e != nil && !os.IsNotExist(e) {
		t.Fatal(e)
	}
	cfg := extensionArtifactGate{Stage: stage, Token: fmt.Sprintf("%x", sha256.Sum256([]byte(dir))), Git: git, Lexical: lexical, Physical: physical, Ref: artifactRef(f.actor), Old: before}
	if err = extensionArtifactAtomic(filepath.Join(dir, "config.json"), cfg); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nexec " + quote(self) + " -test.run=^TestExtensionArtifactChild$ -- --wp18-gate " + quote(dir) + " \"$@\"\n"
	if err = os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	request := map[string]any{"protocol": "hn.extension/1", "request_id": "interrupted", "method": "artifact.mutate", "params": params, "timeout_ms": timeout}
	raw, err := json.Marshal([]any{request})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(f.output + ".ready")
	if err = os.WriteFile(f.output+".go", []byte("go"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(f.binary, "extension", "run", f.module, "--json")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironmentWith(f.home, map[string]string{"HN_WP18_CHILD": "1", "HN_WP18_RECOVERY_DIR": dir, "PATH": dir + string(os.PathListSeparator) + os.Getenv("PATH")})
	frozen := append([]string(nil), cmd.Env...)
	cwd := cmd.Dir
	var output bytes.Buffer
	cmd.Stdout = &projectBoundWriter{w: &output, left: 4096}
	cmd.Stderr = io.Discard
	done := make(chan struct{})
	var hostErr error
	launched := false
	joined := false
	// Register cleanup before Start. SIGTERM invokes the host's supported owner
	// cleanup; escalation is recorded as failure, never a qualifying stimulus.
	cleanup := func() {
		if !launched || joined {
			return
		}
		select {
		case <-done:
			joined = true
			return
		default:
		}
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(7 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			result.fail(fmt.Errorf("host cleanup escalated"))
		}
		joined = true
	}
	defer cleanup()
	started := time.Now()
	if err = cmd.Start(); err != nil {
		result.fail(fmt.Errorf("host start failed"))
		return
	}
	launched = true
	go func() { hostErr = cmd.Wait(); close(done) }()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	poll := time.NewTicker(2 * time.Millisecond)
	defer poll.Stop()
	claimed := false
	delegateClaimed := false
	stageReceipt := time.Duration(0)
	previousPoll := time.Duration(0)
	receiptLower := time.Duration(0)
	supervise := true
	for supervise {
		select {
		case <-done:
			joined = true
			supervise = false
		case <-timer.C:
			result.fail(fmt.Errorf("fixture safety action"))
			supervise = false
		case <-poll.C:
			now := time.Since(started)
			if stage == "lost-ack" && !claimed {
				var record extensionArtifactStage
				e := extensionArtifactReadRecord(filepath.Join(dir, "request.json"), &record)
				if e == nil {
					claimed = true
					if e = extensionArtifactValidateStage(cfg, record); e == nil {
						result.candidate = record.Argv[5]
						e = extensionArtifactDelegate(cfg, record, cwd, frozen, done, filepath.Join(dir, "witness.json"), &delegateClaimed, nil)
					}
					if e != nil {
						result.fail(e)
						supervise = false
					}
				} else if !os.IsNotExist(e) {
					result.fail(e)
					supervise = false
				}
			}
			if stageReceipt == 0 {
				name := "gate.json"
				if stage == "lost-ack" {
					name = "witness.json"
				}
				var record extensionArtifactStage
				e := extensionArtifactReadRecord(filepath.Join(dir, name), &record)
				if e == nil {
					if e = extensionArtifactValidateStage(cfg, record); e != nil {
						result.fail(e)
						supervise = false
					} else {
						stageReceipt = time.Since(started)
						receiptLower = previousPoll
					}
				}
				if e != nil && !os.IsNotExist(e) {
					result.fail(e)
					supervise = false
				}
			}
			if stage != "dispatch-reader-deadline" && stageReceipt == 0 && now > 10*time.Second {
				result.fail(fmt.Errorf("stage arrival exceeded 10000ms"))
				supervise = false
			}
			previousPoll = now
		}
	}
	cleanup()
	elapsed := time.Since(started)
	// This executes on every post-Start path, including delegate/witness/host races.
	final := exec.Command(git, "--git-dir="+physical, "rev-parse", cfg.Ref)
	final.Dir = cwd
	final.Env = operationalTestEnvironment(f.home)
	ref, refErr := final.Output()
	finalRef := strings.TrimSpace(string(ref))
	if refErr != nil || len(ref) > 256 {
		result.fail(fmt.Errorf("unwrapped final ref inspection failed"))
	}
	records := map[string]any{}
	for _, name := range []string{"gate.json", "request.json", "witness.json", "peer.json", "cancel.json", "term.json", "failure.json"} {
		var v map[string]any
		e := extensionArtifactReadRecord(filepath.Join(dir, name), &v)
		if e == nil {
			records[name] = v
		} else if !os.IsNotExist(e) {
			result.fail(e)
		}
	}
	if err = extensionArtifactEvidenceBound(dir); err != nil {
		result.fail(err)
	}
	if records["failure.json"] != nil {
		result.fail(fmt.Errorf("helper/peer safety action"))
	}
	var envelope map[string]any
	if json.Unmarshal(output.Bytes(), &envelope) != nil {
		result.fail(fmt.Errorf("host envelope missing"))
	}
	failure, _ := envelope["error"].(map[string]any)
	tuple, _ := failure["operation"].(map[string]any)
	if hostErr == nil || envelope["ok"] != false || envelope["data"] != nil || failure["origin"] != "host" || failure["code"] != "uncertain_operation" || failure["recovery"] != "inspect_operation" || !reflect.DeepEqual(tuple, map[string]any{"protocol": "hn-artifact/1", "actor": f.actor, "operation": "interrupted"}) {
		result.fail(fmt.Errorf("strict host uncertain_operation/inspect_operation/tuple absent"))
	}
	peer, _ := records["peer.json"].(map[string]any)
	term, _ := records["term.json"].(map[string]any)
	number := func(v any) int64 {
		if n, ok := v.(json.Number); ok {
			x, _ := n.Int64()
			return x
		}
		x, _ := v.(float64)
		return int64(x)
	}
	// Signal zero observes absence only. A present/reused identity, EPERM, or
	// unknown outcome fails; it is not exact kernel-signal or exit telemetry.
	peerPID := int(number(peer["pid"]))
	groupObservation := "missing-peer-identity"
	groupGoneNS := int64(0)
	if peerPID > 0 {
		pidErr := syscall.Kill(peerPID, 0)
		groupErr := syscall.Kill(-peerPID, 0)
		switch {
		case pidErr == syscall.ESRCH && groupErr == syscall.ESRCH:
			groupObservation = "pid-and-group-observed-absent"
			groupGoneNS = time.Now().UnixNano()
		case pidErr == syscall.EPERM || groupErr == syscall.EPERM:
			groupObservation = "permission-unknown"
		case pidErr == nil || groupErr == nil:
			groupObservation = "present-or-reused-identity"
		default:
			groupObservation = "unknown-probe-result"
		}
	}
	if groupGoneNS == 0 || groupGoneNS < int64(number(term["unix_ns"])) {
		result.fail(fmt.Errorf("registered-child absence observation: %s", groupObservation))
	}

	gateName := "gate.json"
	if stage == "lost-ack" {
		gateName = "witness.json"
	}
	gate, _ := records[gateName].(map[string]any)
	if stage == "lost-ack" && (number(gate["git_pid"]) <= 0 || gate["git_exit"] == nil || number(gate["git_exit"]) != 0 || number(gate["git_wait_ns"]) <= 0) {
		result.fail(fmt.Errorf("lost-ack lacks successful joined real Git"))
	}
	if stageReceipt == 0 && gate != nil {
		stageReceipt = elapsed
		receiptLower = previousPoll
	}
	if gate == nil || number(gate["unix_ns"]) < number(peer["before_ns"]) || number(gate["unix_ns"]) >= number(term["unix_ns"]) {
		result.fail(fmt.Errorf("gate did not precede registered-child TERM"))
	}
	if stageReceipt == 0 || peer == nil || term == nil || number(peer["before_ns"]) > number(peer["after_ns"]) || number(peer["after_ns"]) >= number(term["unix_ns"]) {
		result.fail(fmt.Errorf("missing/out-of-order gate/peer/TERM evidence"))
	}
	if stage == "dispatch-reader-deadline" {
		delta := time.Duration(number(term["unix_ns"]) - number(peer["before_ns"]))
		if records["cancel.json"] != nil || delta < 2*time.Second || delta > 4*time.Second || groupGoneNS-number(peer["before_ns"]) >= int64(4*time.Second) || stageReceipt >= elapsed {
			result.fail(fmt.Errorf("literal2000ms deadline timing/cancel evidence failed"))
		}
	} else {
		cancel, _ := records["cancel.json"].(map[string]any)
		witness, _ := records["gate.json"].(map[string]any)
		if stage == "lost-ack" {
			witness, _ = records["witness.json"].(map[string]any)
		}
		if elapsed >= 20*time.Second || stageReceipt > 10*time.Second || cancel == nil || number(cancel["count"]) != 1 || number(cancel["before_ns"]) < number(witness["unix_ns"]) || number(cancel["after_ns"]) < number(cancel["before_ns"]) || number(cancel["after_ns"]) >= number(term["unix_ns"]) || groupGoneNS-number(cancel["before_ns"]) >= int64(2*time.Second) {
			result.fail(fmt.Errorf("stage/cancel/TERM/settlement bound failed"))
		}
	}
	event := map[string]any{"control": "bounded-artifact-recovery", "test": t.Name(), "stage": stage, "host_sha256": wp25Digest(t, f.binary), "request_timeout_ms": timeout, "lexical_git_dir": lexical, "physical_git_dir": physical, "host_exit": cmd.ProcessState.ExitCode(), "host_error_origin": failure["origin"], "host_error_code": failure["code"], "host_recovery": failure["recovery"], "host_operation": tuple, "host_launch_utc": started.UTC().Format(time.RFC3339Nano), "host_wait_after_ns": elapsed.Nanoseconds(), "stage_receipt_lower_ns": receiptLower.Nanoseconds(), "stage_receipt_upper_ns": stageReceipt.Nanoseconds(), "before_ref": before, "final_ref_unwrapped": finalRef, "candidate": result.candidate, "observations": records, "fixture_error": fmt.Sprint(result.failure), "child_absence_probe": groupObservation, "child_absence_observed_ns": groupGoneNS, "clock_caveat": "source proves host first-byte origin; peer/gate/TERM are independent wall clocks; parent launch/Wait/poll intervals use monotonic clock, no exact host timestamp/kernel signal telemetry"}
	raw, err = json.Marshal(event)
	total := len(raw)
	files, boundErr := os.ReadDir(dir)
	if boundErr != nil {
		t.Fatal(boundErr)
	}
	for _, entry := range files {
		info, e := entry.Info()
		if e != nil {
			t.Fatal(e)
		}
		if info.Mode().IsRegular() {
			total += int(info.Size())
		}
	}
	if err != nil || total > 16384 {
		t.Fatal("combined recovery evidence exceeds 16384", err, total)
	}
	t.Log(string(raw))
	wp25Record(t, event)
	return
}
func extensionArtifactPublishedEvent(t *testing.T, f *extensionArtifactFixture, oid string) any {
	t.Helper()
	var event map[string]any
	if err := json.Unmarshal([]byte(f.git("show", oid+":artifact.json")), &event); err != nil {
		t.Fatal(err)
	}
	raw := []byte(f.git("show", oid+":artifact.json"))
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}
func extensionArtifactReaderDeadline(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read,create,revise")
	typ := revisionEvent(f.publish(artifactStoreType, "type", ""))
	id := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "opening")), "opening", ""))
	params := extensionArtifactMutationParams(f, "record.revise", id, []string{id}, revisionPayload(typ, "deadline recovery"), "interrupted")
	before := f.protectedSnapshot()
	beforeRef := f.git("rev-parse", artifactRef(f.actor))
	observed := extensionArtifactRecoveryRun(t, f, "dispatch-reader-deadline", params, 2000, beforeRef)
	if observed.failure != nil {
		t.Fatal(observed.failure)
	}
	if !reflect.DeepEqual(before, f.protectedSnapshot()) || f.git("rev-parse", artifactRef(f.actor)) != beforeRef {
		t.Fatal("reader deadline changed protected signing/intent/lock/object/ref state")
	}
	lookup := map[string]any{"type_id": "example.test/note", "actor": f.actor, "operation": "interrupted"}
	absent := f.request("artifact.operation", lookup, "")["data"].(map[string]any)
	keys := []string{"status", "protocol", "actor", "operation", "family", "type_id", "request", "resource_id", "event_id", "result", "source_cut", "reason"}
	if len(absent) != len(keys) {
		t.Fatal("operation projection key count", absent)
	}
	for _, key := range keys {
		if _, ok := absent[key]; !ok {
			t.Fatal("operation projection key", key)
		}
	}
	if absent["status"] != "verified_absent" || absent["protocol"] != "hn-artifact/1" || absent["actor"] != f.actor || absent["operation"] != "interrupted" || absent["type_id"] != "example.test/note" {
		t.Fatal(absent)
	}
	for _, key := range []string{"family", "request", "resource_id", "event_id", "result", "reason"} {
		if absent[key] != nil {
			t.Fatal("absent projection nonnull", key)
		}
	}
	cut, _ := absent["source_cut"].(string)
	if len(cut) != 71 || !strings.HasPrefix(cut, "sha256:") || !extensionArtifactOID(cut[7:]) {
		t.Fatal("invalid source cut")
	}
	again := f.request("artifact.operation", lookup, "")["data"].(map[string]any)
	if !reflect.DeepEqual(absent, again) || !reflect.DeepEqual(before, f.protectedSnapshot()) {
		t.Fatal("lookup altered protected state/source cut")
	}
	accepted := f.request("artifact.mutate", params, "")["data"].(map[string]any)
	found := f.request("artifact.operation", lookup, "")["data"].(map[string]any)
	afterRef := f.git("rev-parse", artifactRef(f.actor))
	if accepted["replayed"] != false || found["status"] != "found" || found["event_id"] != accepted["event_id"] || afterRef == beforeRef || found["event_id"] != extensionArtifactPublishedEvent(t, f, afterRef) {
		t.Fatal("ordinary formerly-absent tuple recovery", accepted, found)
	}
	replay := f.request("artifact.mutate", params, "")["data"].(map[string]any)
	if replay["replayed"] != true || replay["event_id"] != found["event_id"] || f.git("rev-parse", artifactRef(f.actor)) != afterRef {
		t.Fatal("same-input replay", replay)
	}
}

// Explicit focused controls share this existing child parent; they are never
// another qualification journey or an alternate recovery implementation.
func extensionArtifactFixtureControls(t *testing.T) {
	cfg := extensionArtifactGate{Stage: "lost-ack", Token: "fresh-control", Physical: "/private/repo/.git", Ref: "refs/hn/artifacts/control", Old: strings.Repeat("a", 40)}
	good := extensionArtifactStage{Stage: cfg.Stage, Token: cfg.Token, Argv: []string{"--git-dir=" + cfg.Physical, "update-ref", "-m", "hn: append record.revise", cfg.Ref, strings.Repeat("b", 40), cfg.Old}, PID: 1, NS: 1}
	if err := extensionArtifactValidateStage(cfg, good); err != nil {
		t.Fatal(err)
	}
	for _, control := range []string{"wrong-gate", "wrong-repo", "wrong-ref", "wrong-old", "wrong-message", "wrong-arity", "bad-candidate", "oversized", "wrong-token", "wrong-stage"} {
		t.Run(control, func(t *testing.T) {
			r := good
			r.Argv = append([]string(nil), good.Argv...)
			switch control {
			case "wrong-gate":
				r.Argv[1] = "hash-object"
			case "wrong-repo":
				r.Argv[0] = "--git-dir=/other/.git"
			case "wrong-ref":
				r.Argv[4] += "/other"
			case "wrong-old":
				r.Argv[6] = strings.Repeat("c", 40)
			case "wrong-message":
				r.Argv[3] = "hn: append record.open"
			case "wrong-arity":
				r.Argv = append(r.Argv, "extra")
			case "bad-candidate":
				r.Argv[5] = "not-an-oid"
			case "oversized":
				r.Argv[5] = strings.Repeat("d", 4097)
			case "wrong-token":
				r.Token = "stale"
			case "wrong-stage":
				r.Stage = "prepared"
			}
			if extensionArtifactValidateStage(cfg, r) == nil {
				t.Fatal("invalid selected request accepted")
			}
		})
	}
	t.Run("exact-reader-prepared-and-no-match", func(t *testing.T) {
		for _, stage := range []string{"dispatch-reader-deadline", "prepared"} {
			c := cfg
			c.Stage = stage
			a := []string{"--git-dir=" + c.Physical, "cat-file", "--batch-command"}
			if stage == "prepared" {
				a = []string{"--git-dir=" + c.Physical, "hash-object", "-w", "--stdin"}
			}
			if !extensionArtifactSelected(c, a) || extensionArtifactSelected(c, append(a, "extra")) || extensionArtifactSelected(c, []string{"--git-dir=/other/.git", a[1], a[2]}) {
				t.Fatal("nonexact gate selected")
			}
		}
	})
	for _, control := range []string{"malformed", "oversized-record", "duplicate-key", "incomplete", "trailing-record", "unknown-field"} {
		t.Run(control, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "request.json")
			raw, _ := json.Marshal(good)
			switch control {
			case "malformed":
				raw = []byte("not-json")
			case "oversized-record":
				raw = []byte(strings.Repeat(" ", 6145))
			case "duplicate-key":
				raw = []byte(`{"stage":"lost-ack","stage":"prepared"}`)
			case "incomplete":
				raw = raw[:len(raw)-1]
			case "trailing-record":
				raw = append(raw, []byte("{}")...)
			case "unknown-field":
				raw = []byte(`{"unexpected":true}`)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			var record extensionArtifactStage
			if extensionArtifactReadRecord(path, &record) == nil {
				t.Fatal("bad/incomplete record accepted")
			}
		})
	}
	t.Run("atomic-duplicate-incomplete-and-combined-bound", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "witness.json")
		if err := extensionArtifactAtomic(path, good); err != nil {
			t.Fatal(err)
		}
		if extensionArtifactAtomic(path, good) == nil {
			t.Fatal("duplicate witness accepted")
		}
		if err := extensionArtifactEvidenceBound(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "request.json.tmp"), []byte("incomplete"), 0600); err != nil {
			t.Fatal(err)
		}
		if extensionArtifactEvidenceBound(dir) == nil {
			t.Fatal("incomplete record accepted")
		}
		os.Remove(filepath.Join(dir, "request.json.tmp"))
		if err := os.WriteFile(filepath.Join(dir, "request.json"), make([]byte, 16385), 0600); err != nil {
			t.Fatal(err)
		}
		if extensionArtifactEvidenceBound(dir) == nil {
			t.Fatal("combined overflow accepted")
		}
	})
	for _, control := range []string{"write", "short-write", "close", "rename"} {
		t.Run("atomic-"+control+"-failure", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "witness.json")
			write := (*os.File).Write
			closeFile := (*os.File).Close
			rename := os.Rename
			switch control {
			case "write":
				write = func(*os.File, []byte) (int, error) { return 0, fmt.Errorf("injected write refusal") }
			case "short-write":
				write = func(file *os.File, raw []byte) (int, error) { return file.Write(raw[:len(raw)-1]) }
			case "close":
				closeFile = func(file *os.File) error {
					if err := file.Close(); err != nil {
						return err
					}
					return fmt.Errorf("injected close refusal")
				}
			case "rename":
				rename = func(string, string) error { return fmt.Errorf("injected rename refusal") }
			}
			if extensionArtifactAtomicUsing(path, good, write, closeFile, rename) == nil {
				t.Fatal("failed witness published")
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatal("failed atomic witness left final/temp", err)
			}
		})
	}
	t.Run("compiled-self-exact-no-match-duplicate-and-cleanup", func(t *testing.T) {
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		git, err = filepath.Abs(git)
		if err != nil {
			t.Fatal(err)
		}
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		c := cfg
		c.Stage = "prepared"
		c.Git = git
		if err = extensionArtifactAtomic(filepath.Join(dir, "config.json"), c); err != nil {
			t.Fatal(err)
		}
		command := func(argv ...string) *exec.Cmd {
			cmd := exec.Command(self, append([]string{"-test.run=^TestExtensionArtifactChild$", "--", "--wp18-gate", dir}, argv...)...)
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "HN_WP18_FIXTURE_CONTROLS=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			return cmd
		}
		noMatch := command("--version")
		if err = noMatch.Run(); err != nil {
			t.Fatal("nonmatch failed exec real Git", err)
		}
		if _, err = os.Stat(filepath.Join(dir, "gate.json")); !os.IsNotExist(err) {
			t.Fatal("nonmatch created gate")
		}
		first := command("--git-dir="+c.Physical, "hash-object", "-w", "--stdin")
		started := false
		waited := false
		defer func() {
			if started && !waited {
				_ = first.Process.Kill()
				_ = first.Wait()
			}
		}()
		if err = first.Start(); err != nil {
			t.Fatal(err)
		}
		started = true
		deadline := time.Now().Add(time.Second)
		for {
			if _, err = os.Stat(filepath.Join(dir, "gate.json")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("compiled gate never arrived")
			}
			time.Sleep(2 * time.Millisecond)
		}
		duplicate := command("--git-dir="+c.Physical, "hash-object", "-w", "--stdin")
		if err = duplicate.Run(); err == nil {
			t.Fatal("duplicate helper claim accepted")
		} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 93 {
			t.Fatal("duplicate failed for another cause", err)
		}
		_ = first.Process.Kill()
		err = first.Wait()
		waited = true
		if err == nil || first.ProcessState == nil {
			t.Fatal("blocked helper not joined")
		}
	})

	t.Run("delegate-output-bound", func(t *testing.T) {
		for _, size := range []int{4096, 4097} {
			var output bytes.Buffer
			writer := projectBoundWriter{w: &output, left: 4096}
			_, err := writer.Write(make([]byte, size))
			if (err != nil) != (size == 4097) || output.Len() > 4096 {
				t.Fatal("delegate stdout bound", size, err, output.Len())
			}
		}
	})

	for _, control := range []string{"one-real-git-success", "nonzero-real-git", "witness-failure-after-publication", "host-already-exited", "host-exit-race", "delegate-start-failure"} {
		t.Run(control, func(t *testing.T) {
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			git, err = filepath.Abs(git)
			if err != nil {
				t.Fatal(err)
			}
			repo := t.TempDir()
			home := t.TempDir()
			frozen := operationalTestEnvironment(home)
			run := func(input string, args ...string) string {
				cmd := exec.Command(git, args...)
				cmd.Dir = repo
				cmd.Env = frozen
				cmd.Stdin = strings.NewReader(input)
				raw, err := cmd.Output()
				if err != nil {
					t.Fatal("control git", args, err)
				}
				return strings.TrimSpace(string(raw))
			}
			run("", "init", "--quiet")
			run("", "config", "user.name", "Fixture")
			run("", "config", "user.email", "fixture@example.invalid")
			tree := run("", "mktree")
			old := run("", "commit-tree", tree, "-m", "old")
			candidate := run("", "commit-tree", tree, "-p", old, "-m", "candidate")
			ref := "refs/hn/artifacts/control"
			run("", "update-ref", ref, old)
			c := extensionArtifactGate{Stage: "lost-ack", Token: "fresh-control", Git: git, Physical: filepath.Join(repo, ".git"), Ref: ref, Old: old}
			r := extensionArtifactStage{Stage: c.Stage, Token: c.Token, Argv: []string{"--git-dir=" + c.Physical, "update-ref", "-m", "hn: append record.revise", ref, candidate, old}, PID: os.Getpid(), NS: time.Now().UnixNano()}
			dir := t.TempDir()
			witness := filepath.Join(dir, "witness.json")
			done := make(chan struct{})
			if control == "delegate-start-failure" {
				c.Git = filepath.Join(dir, "missing-git")
			}
			if control == "nonzero-real-git" {
				r.Argv[5] = strings.Repeat("d", 40)
			}
			if control == "witness-failure-after-publication" {
				witness = filepath.Join(dir, "missing", "witness.json")
			}
			if control == "host-already-exited" {
				close(done)
			}
			var afterStart func(*exec.Cmd)
			var racedChild *exec.Cmd
			if control == "host-exit-race" {
				afterStart = func(child *exec.Cmd) { racedChild = child; close(done) }
			}
			claimed := false
			err = extensionArtifactDelegate(c, r, repo, frozen, done, witness, &claimed, afterStart)
			final := run("", "rev-parse", ref) // Always inspect; never undo a raced ref.
			if control == "host-exit-race" && (racedChild == nil || racedChild.ProcessState == nil || syscall.Kill(racedChild.Process.Pid, 0) != syscall.ESRCH) {
				t.Fatal("raced real Git was not Wait-reaped")
			}
			if control == "one-real-git-success" {
				var w extensionArtifactStage
				if err != nil || extensionArtifactReadRecord(witness, &w) != nil || final != candidate {
					t.Fatal("real Git/witness positive", err)
				}
				if extensionArtifactDelegate(c, r, repo, frozen, done, witness, &claimed, afterStart) == nil {
					t.Fatal("duplicate request launched/acknowledged")
				}
			} else {
				if err == nil {
					t.Fatal("failure control accepted")
				}
				if _, e := os.Stat(witness); !os.IsNotExist(e) {
					t.Fatal("failure published success witness")
				}
				if control == "witness-failure-after-publication" && final != candidate {
					t.Fatal("actual publication on witness failure lost")
				}
				if control == "host-exit-race" && final != old && final != candidate {
					t.Fatal("unexpected raced ref")
				}
				if control != "witness-failure-after-publication" && control != "host-exit-race" && final != old {
					t.Fatal("unexpected control ref effect")
				}
			}
		})
	}
}

func TestExtensionArtifactPolicyAndValidationRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read,create,revise,resolve,type.publish")
	typ := revisionEvent(f.publish(artifactStoreType, "type", ""))
	p := revisionPayload(typ, "ordinary custom data")
	f.call("", "", "issue", "open", "--operation", "shared-key", "--json", "Native key scope")
	params := extensionArtifactMutationParams(f, "record.open", nil, []string{}, p, "shared-key")
	id := extensionArtifactEvent(f.request("artifact.mutate", params, ""))
	lookup := f.request("artifact.operation", map[string]any{"type_id": "example.test/note", "actor": f.actor, "operation": "shared-key"}, "")["data"].(map[string]any)
	if lookup["event_id"] != id || lookup["protocol"] != "hn-artifact/1" {
		t.Fatal(lookup)
	}
	// Closed schema and required dependencies are validated before private intent.
	bad := revisionPayload(typ, "bad fields")
	bad["fields"] = map[string]any{"unknown": true}
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, bad, "bad-fields"), "invalid_input")
	bad = revisionPayload(typ, "missing required target")
	bad["relationships"] = []any{map[string]any{"relation": "example.test/link", "target": "sha256:" + strings.Repeat("d", 64), "target_mode": "revision", "required": true}}
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, bad, "required-target"), "incomplete_dependency")
	var publication map[string]any
	json.Unmarshal([]byte(artifactStoreType), &publication)
	publication["payload"].(map[string]any)["schema"].(map[string]any)["language"] = "hn.schema/2"
	f.request("artifact.type.publish", map[string]any{"parents": []string{}, "payload": publication["payload"], "actor": f.actor, "operation": "unsupported-schema"}, "unsupported_schema")
	// A valid optional missing target is retained with an explicit diagnostic.
	bad["relationships"].([]any)[0].(map[string]any)["required"] = false
	optionalID := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, bad, "optional-target"), ""))
	view := extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(optionalID, nil, "view"), ""))[0].(map[string]any)
	if notes := view["diagnostics"].([]any); len(notes) != 1 || notes[0].(map[string]any)["code"] != "missing_optional" {
		t.Fatal(view)
	}
	// A public record may reference a required dependency in an ungranted type;
	// verification uses it internally, with no authority to export its body.
	hiddenType := revisionEvent(f.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", "hidden.test/dependency"), "hidden-type", ""))
	hidden := revisionPayload(hiddenType, "UNGRANTED-DEPENDENCY-BODY")
	hidden["type_id"] = "hidden.test/dependency"
	hiddenID := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, hidden), "hidden-dependency", ""))
	linked := revisionPayload(typ, "links remain inert")
	linked["relationships"] = []any{map[string]any{"relation": "example.test/link", "target": hiddenID, "target_mode": "logical", "required": true}}
	linkedID := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, linked, "linked"), ""))
	returned := f.request("artifact.read", extensionArtifactReadParams(linkedID, nil, "inspect"), "")
	raw, _ := json.Marshal(returned)
	if bytes.Contains(raw, []byte("UNGRANTED-DEPENDENCY-BODY")) {
		t.Fatal("dependency body exported")
	}
	// Select a real contribution policy through its public command.
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{f.actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("WP18 contribution policy")), Moderators: []string{f.actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	raw, _ = json.Marshal(policy)
	path := filepath.Join(f.repo, ".hn", "policy.json")
	os.MkdirAll(filepath.Dir(path), 0700)
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	f.git("add", ".hn/policy.json")
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Stored policy")
	branch := f.git("symbolic-ref", "HEAD")
	selectPolicy := exec.Command(f.binary, "contribution", "authority", "set", "--ref", branch, "--expect", "absent")
	selectPolicy.Dir = f.repo
	selectPolicy.Env = operationalTestEnvironment(f.home)
	if raw, e := selectPolicy.CombinedOutput(); e != nil {
		t.Fatalf("select policy %v %s", e, raw)
	}
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, p, "policy-refused"), "unsupported_capability")
	publication["payload"].(map[string]any)["schema"].(map[string]any)["language"] = "hn.schema/1"
	f.request("artifact.type.publish", map[string]any{"parents": []string{}, "payload": publication["payload"], "actor": f.actor, "operation": "policy-type-refused"}, "unsupported_capability")
}

func TestExtensionArtifactConflictRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read,create,revise,resolve")
	b := artifactCLI(t)
	typ := revisionEvent(f.publish(artifactStoreType, "type", ""))
	original := revisionPayload(typ, "original")
	opening := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, original, "open"), ""))
	b.git("fetch", f.repo, "+refs/hn/artifacts/*:refs/hn/remotes/public/artifacts/*")
	right := revisionEvent(revisionCall(b, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(typ, "right")), "right", ""))
	left := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", opening, []string{opening}, revisionPayload(typ, "left"), "left"), ""))
	f.git("fetch", b.repo, "+refs/hn/artifacts/*:refs/hn/remotes/public/artifacts/*")
	heads := []string{left, right}
	sort.Strings(heads)
	views := extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(opening, nil, "heads"), ""))
	view := views[0].(map[string]any)
	if view["conflict"] != true || view["state"] != nil || len(view["heads"].([]any)) != 2 {
		t.Fatal(view)
	}
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", opening, []string{left}, revisionPayload(typ, "overwrite"), "conflict-refuse"), "resource_conflict")
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.resolve", opening, []string{left}, revisionPayload(typ, "partial"), "partial-refuse"), "stale_revision")
	resolution := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.resolve", opening, heads, revisionPayload(typ, "explicit result"), "resolution"), ""))
	final := extensionArtifactItems(f.request("artifact.read", extensionArtifactReadParams(opening, nil, "view"), ""))[0].(map[string]any)
	if final["conflict"] != false || final["heads"].([]any)[0].(map[string]any)["id"] != resolution {
		t.Fatal(final)
	}
	// Invalid selected signed/ref history is never an empty successful view.
	f.git("update-ref", "refs/hn/artifacts/malformed", f.git("rev-parse", artifactRef(f.actor)))
	f.request("artifact.read", extensionArtifactReadParams(opening, nil, "view"), "invalid_history")
}

func TestExtensionArtifactNoReadGrantRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=create")
	missing := "sha256:" + strings.Repeat("d", 64)
	for _, method := range []string{"artifact.read", "artifact.type.read", "artifact.search", "artifact.operation"} {
		p := extensionArtifactReadParams(missing, nil, "inspect")
		switch method {
		case "artifact.type.read":
			p = map[string]any{"id": missing, "type_id": nil, "limit": 200, "cursor": nil}
		case "artifact.search":
			p = extensionArtifactSearchParams(nil, nil, "", nil)
		case "artifact.operation":
			p = map[string]any{"type_id": "example.test/note", "actor": f.actor, "operation": "missing"}
		}
		f.request(method, p, "denied_access")
	}
	opaque := extensionArtifactReadParams(nil, nil, "inspect")
	opaque["opaque"], opaque["opaque_id"] = true, missing
	f.request("artifact.read", opaque, "denied_access")
}

// Independent public metadata reconstruction: do not use the adapter's Cut or
// projection helpers to generate expectations for the actual caller.
func extensionArtifactExpectedCut(t *testing.T, f *extensionArtifactFixture, types, ids []string, acceptedRefs bool) string {
	t.Helper()
	types = append([]string{}, types...)
	sort.Strings(types)
	ids = append([]string{}, ids...)
	sort.Strings(ids)
	refs := []map[string]any{}
	if acceptedRefs {
		lines := f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/artifacts/", "refs/hn/remotes/*/artifacts/*/*")
		for _, line := range strings.Split(lines, "\n") {
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) != 2 {
				t.Fatal("public ref witness", line)
			}
			refs = append(refs, map[string]any{"ref": parts[0], "oid": parts[1]})
		}
		sort.Slice(refs, func(i, j int) bool { return refs[i]["ref"].(string) < refs[j]["ref"].(string) })
	}
	common := f.git("rev-parse", "--path-format=absolute", "--git-common-dir")
	physical, err := filepath.EvalSymlinks(common)
	if err != nil {
		t.Fatal(err)
	}
	digest := func(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
	repository := digest(append([]byte("hn-extension-repository-v1\x00"), []byte(physical)...))
	view, err := json.Marshal(map[string]any{"admitted_ids": ids, "retained_refs": []any{}, "visible_ids": ids})
	if err != nil {
		t.Fatal(err)
	}
	cut, err := json.Marshal(map[string]any{"authority_commit": "", "policy_digest": "", "refs": refs, "repository_id": repository, "selection": map[string]any{"families": []any{}, "types": types}, "view_digest": digest(view)})
	if err != nil {
		t.Fatal(err)
	}
	return digest(cut)
}

func TestExtensionArtifactSelectedTypeCutRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read,revise", "artifact:empty.test/note=read")
	typ := revisionEvent(f.publish(artifactStoreType, "selected-type", ""))
	opening := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "selected first")), "selected-open", ""))
	// Both same-stream unrelated data and a separate accepted actor stream remain
	// actual ref witnesses without becoming IDs in the selected type projection.
	hiddenType := revisionEvent(f.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", "unrelated.test/hidden"), "hidden-type", ""))
	hidden := revisionPayload(hiddenType, "UNGRANTED-CUT-BODY")
	hidden["type_id"] = "unrelated.test/hidden"
	revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, hidden), "hidden-record", "")
	b := artifactCLI(t)
	b.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", "separate.test/stream"), "unrelated-stream", "")
	f.git("fetch", b.repo, "+refs/hn/artifacts/*:refs/hn/remotes/public/artifacts/*")
	mutation := f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", opening, []string{opening}, revisionPayload(typ, "selected next"), "selected-revise"), "")["data"].(map[string]any)
	revision := mutation["event_id"].(string)
	want := extensionArtifactExpectedCut(t, f, []string{"example.test/note"}, []string{typ, opening, revision}, true)
	if mutation["source_cut"] != want {
		t.Fatal("mutation Cut scope", mutation["source_cut"], want)
	}
	for _, tc := range []struct {
		method string
		params map[string]any
	}{
		{"artifact.read", extensionArtifactReadParams(opening, nil, "view")},
		{"artifact.read", extensionArtifactReadParams(revision, nil, "heads")},
		{"artifact.read", extensionArtifactReadParams(opening, nil, "history")},
		{"artifact.read", extensionArtifactReadParams(typ, nil, "inspect")},
		{"artifact.type.read", map[string]any{"id": typ, "type_id": nil, "limit": 200, "cursor": nil}},
		{"artifact.search", extensionArtifactSearchParams("example.test/note", nil, "missing match", nil)},
		{"artifact.search", extensionArtifactSearchParams("example.test/note", "ticket", "", nil)},
		{"artifact.operation", map[string]any{"type_id": "example.test/note", "actor": f.actor, "operation": "selected-revise"}},
	} {
		data := f.request(tc.method, tc.params, "")["data"].(map[string]any)
		if data["source_cut"] != want {
			t.Fatalf("%s varied its complete type Cut: %v want %s", tc.method, data["source_cut"], want)
		}
		if nested, ok := data["result"].(map[string]any); ok && nested["source_cut"] != want {
			t.Fatal("operation nested Cut scope", nested)
		}
		raw, _ := json.Marshal(data)
		if bytes.Contains(raw, []byte("UNGRANTED-CUT-BODY")) {
			t.Fatal("Cut membership exported unrelated body")
		}
	}
	empty := f.request("artifact.search", extensionArtifactSearchParams("empty.test/note", nil, "", nil), "")["data"].(map[string]any)
	emptyWant := extensionArtifactExpectedCut(t, f, []string{"empty.test/note"}, nil, true)
	if len(empty["items"].([]any)) != 0 || empty["source_cut"] != emptyWant || emptyWant == want {
		t.Fatal("exact empty type disappeared from selection", empty)
	}
}

func TestExtensionArtifactEmptyNamespaceCutRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:unobserved.test/*=read")
	f.publish(artifactStoreType, "unrelated-type", "")
	data := f.request("artifact.search", extensionArtifactSearchParams(nil, nil, "", nil), "")["data"].(map[string]any)
	want := extensionArtifactExpectedCut(t, f, nil, nil, true)
	opaqueWant := extensionArtifactExpectedCut(t, f, nil, nil, false)
	if len(data["items"].([]any)) != 0 || data["next_cursor"] != nil || data["source_cut"] != want || want == opaqueWant {
		t.Fatal("empty observed namespace lost accepted witnesses or implied all", data)
	}
}

func TestExtensionArtifactPostProjectionSourceRecheckRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read")
	typ := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f.artifactCLIFixture, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "current view")), "open", ""))
	// The remote is a real accepted witness of the same verified actor chain.
	localRef := artifactRef(f.actor)
	remoteRef := strings.Replace(localRef, "refs/hn/artifacts/", "refs/hn/remotes/public/artifacts/", 1)
	tip := f.git("rev-parse", localRef)
	prior := f.git("rev-parse", localRef+"^")
	f.git("update-ref", remoteRef, tip)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := t.TempDir()
	counter := filepath.Join(wrapper, "count")
	marker := filepath.Join(wrapper, "changed")
	// The shared loader has completed both coherent full enumerations; change the
	// third (post-projection) observation. An ordinary snapshot-only reader misses it.
	script := "#!/bin/sh\nscan=0; local_scope=0; remote_scope=0\nfor arg do [ \"$arg\" = for-each-ref ] && scan=1; [ \"$arg\" = 'refs/hn/artifacts/' ] && local_scope=1; [ \"$arg\" = 'refs/hn/remotes/*/artifacts/*/*' ] && remote_scope=1; done\nif [ \"$scan:$local_scope:$remote_scope\" = 1:1:1 ]; then n=0; [ -f \"$HN_WP18_COUNT\" ] && n=$(cat \"$HN_WP18_COUNT\"); n=$((n+1)); echo \"$n\" > \"$HN_WP18_COUNT\"; if [ \"$n\" = 3 ]; then '" + git + "' -c core.logAllRefUpdates=false update-ref \"$HN_WP18_REF\" \"$HN_WP18_PRIOR\" || exit $?; : > \"$HN_WP18_CHANGED\"; fi; fi\nexec '" + git + "' \"$@\"\n"
	if err = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	f.overrides["PATH"] = wrapper + string(os.PathListSeparator) + os.Getenv("PATH")
	f.overrides["HN_WP18_COUNT"], f.overrides["HN_WP18_CHANGED"], f.overrides["HN_WP18_REF"], f.overrides["HN_WP18_PRIOR"] = counter, marker, remoteRef, prior
	// Exempt only the exact test-controlled ref. Every object, signer, intent and
	// other path remains under the normal after-handshake no-effect comparison.
	f.expectedExternalRef = filepath.Join(f.repo, ".git", filepath.FromSlash(remoteRef))
	f.request("artifact.read", extensionArtifactReadParams(opening, nil, "view"), "concurrent_update")
	if _, err = os.Stat(marker); err != nil {
		t.Fatal("post-projection control did not execute", err)
	}
	delete(f.overrides, "PATH")
	if f.git("rev-parse", remoteRef) != prior || f.git("rev-parse", localRef) != tip {
		t.Fatal("wrong witness changed")
	}
}

func TestExtensionArtifactReplayIntentFieldsRawCLI(t *testing.T) {
	f := extensionArtifactCLI(t, "artifact:example.test/note=read,create,revise,type.publish")
	typ := revisionEvent(f.publish(artifactStoreType, "schema-one", ""))
	p := revisionPayload(typ, "immutable content")
	opening := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, p, "pinned-open"), ""))
	evolved := strings.Replace(strings.Replace(artifactStoreType, `"parents":[]`, `"parents":["`+typ+`"]`, 1), `"schema_version":1`, `"schema_version":2`, 1)
	typ2 := revisionEvent(f.publish(evolved, "schema-two", ""))
	// Both schemas admit a fresh opening. Only the immutable actor/key binding
	// makes this otherwise valid new-schema opening a changed-request refusal.
	schemaChanged := revisionPayload(typ2, "immutable content")
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, schemaChanged, "pinned-open"), "operation_conflict")
	revised := revisionPayload(typ, "next immutable content")
	next := extensionArtifactEvent(f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", opening, []string{opening}, revised, "pinned-revise"), ""))
	// The replacement parent is the actual current head, making this a valid
	// successor with a fresh key; reusing the original key must still refuse.
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", opening, []string{next}, revised, "pinned-revise"), "operation_conflict")
	// Positive controls prove the two candidate intents are otherwise admissible.
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.open", nil, []string{}, schemaChanged, "fresh-schema-key"), "")
	f.request("artifact.mutate", extensionArtifactMutationParams(f, "record.revise", opening, []string{next}, revised, "fresh-parent-key"), "")
}
