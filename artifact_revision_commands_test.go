package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func revisionInput(t *testing.T, record any, parents []string, payload map[string]any) string {
	t.Helper()
	b, e := json.Marshal(map[string]any{"record": record, "parents": parents, "payload": payload})
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func revisionPayload(schema, text string) map[string]any {
	return map[string]any{"type_id": "example.test/note", "type_revision": schema, "shape": "document", "title": "Revision data", "content": text, "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
}
func revisionCall(f *artifactCLIFixture, kind, input, key, code string) map[string]any {
	return f.call(input, code, "artifact", kind, "--input", "-", "--actor", f.actor, "--operation", key, "--json")
}
func revisionEvent(v map[string]any) string { return v["data"].(map[string]any)["event_id"].(string) }
func TestArtifactRevisionLiveCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "original\n雪 e\u0301")), "open", ""))
	input := revisionInput(t, opening, []string{opening}, revisionPayload(schema, "revision\n雪 e\u0301"))
	revised := revisionEvent(revisionCall(f, "revise", input, "revise", ""))
	before := f.snapshot()
	replay := revisionCall(f, "revise", input, "revise", "")["data"].(map[string]any)
	if replay["event_id"] != revised || replay["replayed"] != true {
		t.Fatal(replay)
	}
	f.unchanged(before)
	revisionCall(f, "revise", strings.Replace(input, "revision", "different", 1), "revise", "operation_conflict")
	f.unchanged(before)
	revisionCall(f, "revise", input, "stale", "stale_revision")
	f.unchanged(before)
	shown := f.call("", "", "artifact", "show", opening, "--json")["data"].(map[string]any)
	view := shown["items"].([]any)[0].(map[string]any)
	if view["id"] != opening || view["conflict"] != false || view["state"].(map[string]any)["content"] != "revision\n雪 e\u0301" {
		t.Fatal(view)
	}
	expectedKeys := []string{"protocol", "id", "type_id", "shape", "heads", "conflict", "state", "diagnostics"}
	if len(view) != len(expectedKeys) {
		t.Fatal("RecordView fields", view)
	}
	for _, key := range expectedKeys {
		if _, ok := view[key]; !ok {
			t.Fatal("missing RecordView field", key)
		}
	}
	headView := f.call("", "", "artifact", "heads", opening, "--json")["data"].(map[string]any)["items"].([]any)[0]
	if !reflect.DeepEqual(view, headView) {
		t.Fatal("heads differs from same selected view")
	}
	heads := view["heads"].([]any)
	if len(heads) != 1 || heads[0].(map[string]any)["id"] != revised {
		t.Fatal(heads)
	}
	history := f.call("", "", "artifact", "history", opening, "--json")["data"].(map[string]any)["items"].([]any)
	if len(history) != 2 {
		t.Fatal(history)
	}
	f.unchanged(before)
	for _, mode := range []string{"show", "heads", "history"} {
		f.call("", "not_found", "artifact", mode, "sha256:"+strings.Repeat("d", 64), "--json")
		f.unchanged(before)
		f.call("", "invalid_input", "artifact", mode, opening, "--json", "--json")
		f.unchanged(before)
	}
	// Publishing a newer schema never changes the existing exact schema or bytes.
	newType := strings.Replace(artifactStoreType, `"parents":[]`, `"parents":["`+schema+`"]`, 1)
	newType = strings.Replace(newType, `"schema_version":1`, `"schema_version":2`, 1)
	schema2 := revisionEvent(f.publish(newType, "type2", ""))
	p := revisionPayload(schema2, "converted")
	p["conversion_sources"] = []any{revised}
	converted := revisionEvent(revisionCall(f, "convert", revisionInput(t, opening, []string{revised}, p), "convert", ""))
	snapshot := f.snapshot()
	history2 := f.call("", "", "artifact", "history", opening, "--json")["data"].(map[string]any)["items"].([]any)
	for _, old := range history {
		found := false
		for _, n := range history2 {
			if reflect.DeepEqual(old, n) {
				found = true
			}
		}
		if !found {
			t.Fatal("historical signed bytes changed")
		}
	}
	if len(history2) != 3 || converted == revised {
		t.Fatal(history2)
	}
	f.unchanged(snapshot)
	// Replay the earlier successor after a conversion changed the sole head.
	earlier := revisionCall(f, "revise", input, "revise", "")["data"].(map[string]any)
	if earlier["event_id"] != revised || earlier["replayed"] != true {
		t.Fatal(earlier)
	}
	f.unchanged(snapshot)

}

// Git transport here copies only public actor refs/objects. Every actor is
// independently initialized; no identity, intent or other private file is copied.
func revisionFetch(dst, src *artifactCLIFixture, remote string) {
	dst.git("fetch", "--no-write-fetch-head", src.repo, "+refs/hn/artifacts/*:refs/hn/remotes/"+remote+"/artifacts/*")
}
func revisionHeads(t *testing.T, view map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, v := range view["heads"].([]any) {
		out = append(out, v.(map[string]any)["id"].(string))
	}
	return out
}
func TestArtifactRevisionIndependentActorsCLI(t *testing.T) {
	a, b, c, d := artifactCLI(t), artifactCLI(t), artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	newer := strings.Replace(artifactStoreType, `"parents":[]`, `"parents":["`+schema+`"]`, 1)
	newer = strings.Replace(newer, `"schema_version":1`, `"schema_version":2`, 1)
	schema2 := revisionEvent(a.publish(newer, "type2", ""))
	opening := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "opening 雪 e\u0301")), "open", ""))
	foreign := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "another record")), "foreign", ""))
	revisionFetch(b, a, "a")
	revisionFetch(c, a, "a")
	ar := revisionEvent(revisionCall(a, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "actor A")), "a-revise", ""))
	br := revisionEvent(revisionCall(b, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "actor B")), "b-revise", ""))
	cp := revisionPayload(schema2, "actor C converted")
	cp["conversion_sources"] = []any{opening}
	cr := revisionEvent(revisionCall(c, "convert", revisionInput(t, opening, []string{opening}, cp), "c-convert", ""))
	revisionFetch(a, b, "b")
	ab := []string{ar, br}
	sort.Strings(ab)
	view := revisionReadView(a, opening)
	if view["state"] != nil || view["conflict"] != true || !reflect.DeepEqual(revisionHeads(t, view), ab) {
		t.Fatal(view)
	}
	// Arrival order and remote names cannot select a different signed record state.
	revisionFetch(d, b, "z")
	revisionFetch(d, a, "a")
	if !reflect.DeepEqual(view, revisionReadView(d, opening)) {
		t.Fatal("arrival order changed record view")
	}
	// A conflicted logical link is valid data and exposes a diagnostic; an
	// exact link to the same historical revision does not substitute current heads.
	for _, mode := range []string{"logical", "revision"} {
		p := revisionPayload(schema, "linked conflict")
		target := opening
		if mode == "revision" {
			target = ar
		}
		p["relationships"] = []any{map[string]any{"relation": "example.test/points", "target": target, "target_mode": mode, "required": true}}
		linked := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, p), "linked-"+mode, ""))
		diagnostics := revisionReadView(a, linked)["diagnostics"].([]any)
		if mode == "logical" {
			if len(diagnostics) != 1 || diagnostics[0].(map[string]any)["code"] != "target_conflict" {
				t.Fatal(diagnostics)
			}
		} else if len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
	}
	before := a.snapshot()
	for _, kind := range []string{"revise", "convert"} {
		p := revisionPayload(schema, "not a resolution")
		if kind == "convert" {
			p = revisionPayload(schema2, "not a resolution")
			p["conversion_sources"] = []any{ar}
		}
		revisionCall(a, kind, revisionInput(t, opening, []string{ar}, p), "conflict-"+kind, "resource_conflict")
		a.unchanged(before)
	}
	for name, parents := range map[string][]string{"missing": {ar}, "extra": {opening, ar, br}} {
		sort.Strings(parents)
		revisionCall(a, "resolve", revisionInput(t, opening, parents, revisionPayload(schema, "partial")), name, "stale_revision")
		a.unchanged(before)
	}
	revisionCall(a, "resolve", revisionInput(t, opening, []string{foreign}, revisionPayload(schema, "foreign")), "foreign-parent", "invalid_input")
	a.unchanged(before)
	revisionCall(a, "resolve", revisionInput(t, opening, []string{ab[1], ab[0]}, revisionPayload(schema, "unsorted")), "unsorted", "invalid_input")
	a.unchanged(before)
	// B can resolve AB before learning about C. Its signed resolution stays valid
	// when C later arrives; this local precondition is never an import-time rule.
	revisionFetch(b, a, "a")
	abInput := revisionInput(t, opening, ab, revisionPayload(schema, "resolved AB"))
	abr := revisionEvent(revisionCall(b, "resolve", abInput, "resolve-ab", ""))
	// Persist a complete immutable prepared descriptor for the unchanged AB
	// request. A newly selected third head must refuse without replacing it.
	revisionPreparedIntent(t, a, "open", "stale-third", "record.resolve", abInput)
	revisionFetch(a, c, "c")
	before = a.snapshot()
	revisionCall(a, "resolve", abInput, "stale-third", "stale_revision")
	a.unchanged(before)
	revisionFetch(a, b, "b")
	mixed := []string{abr, cr}
	sort.Strings(mixed)
	if got := revisionHeads(t, revisionReadView(a, opening)); !reflect.DeepEqual(got, mixed) {
		t.Fatal(got, mixed)
	}
	// An exact retry of B's original operation succeeds after current heads changed.
	revisionFetch(b, c, "c")
	replay := revisionCall(b, "resolve", abInput, "resolve-ab", "")["data"].(map[string]any)
	if replay["event_id"] != abr || replay["replayed"] != true || replay["head_count"] != float64(2) {
		t.Fatal(replay)
	}
	// Choose destination schema explicitly and name exactly changed-schema parents.
	payload := revisionPayload(schema2, "all actors resolved 雪 e\u0301")
	payload["conversion_sources"] = []any{abr}
	before = a.snapshot()
	bad := revisionPayload(schema2, "all actors resolved 雪 e\u0301")
	revisionCall(a, "resolve", revisionInput(t, opening, mixed, bad), "missing-conversion", "invalid_input")
	a.unchanged(before)
	final := revisionEvent(revisionCall(a, "resolve", revisionInput(t, opening, mixed, payload), "resolve-all", ""))
	view = revisionReadView(a, opening)
	if view["conflict"] != false || !reflect.DeepEqual(revisionHeads(t, view), []string{final}) || view["state"].(map[string]any)["type_revision"] != schema2 {
		t.Fatal(view)
	}
	history := a.call("", "", "artifact", "history", opening, "--json")["data"].(map[string]any)["items"].([]any)
	if len(history) != 6 {
		t.Fatal("lost predecessor", history)
	}
	// Public reconstruction needs no author identity or operation intent.
	revisionFetch(d, a, "a")
	revisionFetch(d, b, "z")
	revisionFetch(d, c, "c")
	if !reflect.DeepEqual(view, revisionReadView(d, opening)) {
		t.Fatal("independent reconstruction differs")
	}
	// Exact selected schema is never substituted by schema2 during ordinary revise.
	before = a.snapshot()
	wrong := revisionPayload(schema, "wrong old schema")
	revisionCall(a, "revise", revisionInput(t, opening, []string{final}, wrong), "wrong-schema", "invalid_input")
	a.unchanged(before)
}

func TestArtifactRevisionCausalRefusalsCLI(t *testing.T) {
	f := artifactCLI(t)
	typ := `{"parents":[],"payload":{"type_id":"example.test/note","schema_version":1,"shape":"document","schema":{"language":"hn.schema/1","root":{"type":"object","properties":{"code":{"type":"string","maxBytes":4}},"required":["code"],"additionalProperties":false}}}}`
	schema := revisionEvent(f.publish(typ, "typed", ""))
	p := revisionPayload(schema, "schema control")
	p["fields"] = map[string]any{"code": "snow"}
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, p), "open", ""))
	valid := revisionInput(t, opening, []string{opening}, p)
	before := f.snapshot()
	for _, tc := range []struct{ name, code, input string }{
		{"duplicate", "invalid_input", strings.Replace(valid, `"record":`, `"record":null,"record":`, 1)},
		{"unicode", "invalid_input", strings.Replace(valid, "schema control", `\ud800`, 1)},
		{"schema-value", "invalid_input", strings.Replace(valid, `"code":"snow"`, `"code":true`, 1)},
		{"schema-bound", "invalid_input", strings.Replace(valid, `"code":"snow"`, `"code":"snows"`, 1)},
		{"wrong-logical", "invalid_input", strings.Replace(valid, `"record":"`+opening+`"`, `"record":"`+schema+`"`, 1)},
		{"missing-schema", "incomplete_dependency", strings.Replace(valid, `"type_revision":"`+schema+`"`, `"type_revision":"sha256:`+strings.Repeat("d", 64)+`"`, 1)},
	} {
		v := revisionCall(f, "revise", tc.input, tc.name, tc.code)
		artifactErrorShape(t, v, "artifact", tc.code, "do_not_retry", nil)
		f.unchanged(before)
	}
	// Malformed successor input never creates a descriptor, even for convert's
	// empty-parent panic control or resolve's empty set.
	for _, kind := range []string{"revise", "resolve", "convert"} {
		revisionCall(f, kind, revisionInput(t, opening, []string{}, p), "empty-"+kind, "invalid_input")
		f.unchanged(before)
		f.call(valid, "invalid_input", "artifact", kind, "--input", "-", "--actor", f.actor, "--operation", "flags", "--operation", "flags", "--json")
		f.unchanged(before)
	}
	revisionCall(f, "convert", valid, "same-schema", "invalid_input")
	f.unchanged(before)
	positive := revisionEvent(revisionCall(f, "revise", valid, "at-schema-bound", ""))
	if positive == opening {
		t.Fatal("no successor")
	}
}

func revisionPreparedIntent(t *testing.T, f *artifactCLIFixture, template, key, kind, input string) {
	t.Helper()
	dir := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor)
	path := func(key string) string {
		return filepath.Join(dir, strings.TrimPrefix(eventID([]byte(key)), "sha256:")+".json")
	}
	raw, e := os.ReadFile(path(template))
	if e != nil {
		t.Fatal(e)
	}
	var intent map[string]any
	if e = json.Unmarshal(raw, &intent); e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	if e = json.Unmarshal([]byte(input), &m); e != nil {
		t.Fatal(e)
	}
	semantic := map[string]any{"protocol": "hn-artifact/1", "kind": kind, "actor": f.actor, "operation": key, "record": m["record"], "parents": m["parents"], "payload": m["payload"]}
	request, _ := json.Marshal(semantic)
	intent["operation"] = key
	intent["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), request...))
	intent["semantic_request"] = semantic
	intent["stage"] = "prepared"
	intent["expected_oid"] = f.git("rev-parse", artifactRef(f.actor))
	intent["candidate"] = nil
	intent["result"] = nil
	raw, e = json.Marshal(intent)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path(key), raw, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestArtifactRevisionEnvelopeBytesCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "")), "open", ""))
	// Independently size the future envelope from public signed syntax. All IDs,
	// keys and UTC seconds are fixed width; sequence is the next actual sequence.
	raw := f.git("show", artifactRef(f.actor)+":artifact.json")
	var envelope map[string]any
	if e := json.Unmarshal([]byte(raw), &envelope); e != nil {
		t.Fatal(e)
	}
	p := revisionPayload(schema, "")
	envelope["kind"] = "record.revise"
	envelope["record"] = opening
	envelope["parents"] = []string{opening}
	envelope["payload"] = p
	envelope["operation"] = "byte-bound-0"
	envelope["previous"] = opening
	envelope["sequence"] = envelope["sequence"].(float64) + 1
	b, e := json.Marshal(envelope)
	if e != nil {
		t.Fatal(e)
	}
	p["content"] = strings.Repeat("x", 1048576-len(b)+1)
	over := revisionInput(t, opening, []string{opening}, p)
	if len(over) > 1048576 {
		t.Fatal("control must fit input bound")
	}
	before := f.snapshot()
	revisionCall(f, "revise", over, "byte-bound-1", "resource_limit")
	f.unchanged(before)
	p["content"] = p["content"].(string)[1:]
	originalInput := revisionInput(t, opening, []string{opening}, p)
	atBound := revisionEvent(revisionCall(f, "revise", originalInput, "byte-bound-0", ""))
	raw = f.git("show", artifactRef(f.actor)+":artifact.json")
	if len(raw) != 1048576 {
		t.Fatal("at-bound signed envelope", len(raw))
	}
	// New candidates would now need a two-digit sequence and would exceed the
	// envelope bound with this old payload. Exact replay uses its original bytes.
	head := atBound
	for sequence := 4; sequence <= 10; sequence++ {
		head = revisionEvent(revisionCall(f, "revise", revisionInput(t, opening, []string{head}, revisionPayload(schema, "advance")), fmt.Sprintf("advance-%d", sequence), ""))
	}
	before = f.snapshot()
	replay := revisionCall(f, "revise", originalInput, "byte-bound-0", "")["data"].(map[string]any)
	if replay["event_id"] != atBound || replay["replayed"] != true {
		t.Fatal(replay)
	}
	f.unchanged(before)
}

func TestArtifactRevisionResumeOriginalCandidateCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "opening")), "open", ""))
	input := revisionInput(t, opening, []string{opening}, revisionPayload(schema, "original candidate 雪 e\u0301"))
	original := revisionEvent(revisionCall(f, "revise", input, "resume", ""))
	path := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor, strings.TrimPrefix(eventID([]byte("resume")), "sha256:")+".json")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var intent map[string]any
	if e = json.Unmarshal(raw, &intent); e != nil {
		t.Fatal(e)
	}
	candidate := intent["candidate"].(map[string]any)
	commit := candidate["commit"].(string)
	public := f.git("show", commit+":artifact.json")
	signature := f.git("show", commit+":signature")
	// Recreate a lost publication acknowledgement with its exact durable candidate
	// already present, but no public ref yet. Only fixture state is fault-injected.
	f.git("update-ref", artifactRef(f.actor), intent["expected_oid"].(string), commit)
	intent["stage"] = "publishing"
	intent["result"] = nil
	raw, e = json.Marshal(intent)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	objects := f.git("cat-file", "--batch-all-objects", "--batch-check=%(objectname)")
	got := revisionCall(f, "revise", input, "resume", "")["data"].(map[string]any)
	if got["event_id"] != original || f.git("rev-parse", artifactRef(f.actor)) != commit {
		t.Fatal("original candidate witness replaced", got)
	}
	if f.git("show", commit+":artifact.json") != public || f.git("show", commit+":signature") != signature || f.git("cat-file", "--batch-all-objects", "--batch-check=%(objectname)") != objects {
		t.Fatal("candidate resume signed or wrote new objects")
	}
	raw, e = os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var completed map[string]any
	if e = json.Unmarshal(raw, &completed); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(completed["candidate"], candidate) || completed["expected_oid"] != intent["expected_oid"] || !reflect.DeepEqual(completed["semantic_request"], intent["semantic_request"]) || completed["stage"] != "published" {
		t.Fatal("immutable witness changed", completed)
	}
	before := f.snapshot()
	again := revisionCall(f, "revise", input, "resume", "")["data"].(map[string]any)
	if again["event_id"] != original || again["replayed"] != true {
		t.Fatal(again)
	}
	f.unchanged(before)
}
