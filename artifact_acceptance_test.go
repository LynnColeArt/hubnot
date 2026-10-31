package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Lose the acknowledgement only after Git has committed the real CAS. The next
// command is a new process, with no fake application result or private-state copy.
func artifactJourneyLostAck(t *testing.T, f *artifactCLIFixture, input, key string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := t.TempDir()
	marker := filepath.Join(wrapper, "published")
	script := "#!/bin/sh\nfor arg do if [ \"$arg\" = update-ref ]; then '" + git + "' \"$@\" || exit $?; : > \"$HN_WP11_CAS\"; while :; do sleep 1; done; fi; done\nexec '" + git + "' \"$@\"\n"
	if err = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(f.binary, "artifact", "revise", "--input", "-", "--actor", f.actor, "--operation", key, "--json")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironmentWith(f.home, map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP11_CAS": marker})
	cmd.Stdin = strings.NewReader(input)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	reached := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err = os.Stat(marker); err == nil {
			reached = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err = cmd.Wait(); err == nil || !reached || output.Len() != 0 {
		t.Fatalf("lost ack cause: reached=%v err=%v output=%s", reached, err, output.String())
	}
}
func artifactJourneyView(t *testing.T, f *artifactCLIFixture, id string) map[string]any {
	t.Helper()
	return artifactPage(t, f.call("", "", "artifact", "show", id, "--json"))["items"].([]any)[0].(map[string]any)
}
func artifactJourneyRemote(f *artifactCLIFixture, command, id string) map[string]any {
	f.t.Helper()
	return f.call("", "", "artifact", command, "--remote", "public", "--id", id, "--json")["data"].(map[string]any)
}
func TestArtifactFullPublicJourneyCLI(t *testing.T) {
	a, b := artifactCLI(t), artifactCLI(t)
	native := a.call("", "", "issue", "open", "--operation", "native-open", "--json", "Native still ordinary")["data"].(map[string]any)
	nativeID := native["issue_id"].(string)
	nativeRefs := a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/")
	remote := filepath.Join(t.TempDir(), "public.git")
	a.git("init", "--bare", "--quiet", remote)
	for _, f := range []*artifactCLIFixture{a, b} {
		f.git("remote", "add", "public", remote)
	}
	marker := filepath.Join(t.TempDir(), "must-not-execute")
	exact := "# Exact public text\n雪 e\u0301\r\n$(touch " + marker + ") <script>throw 1</script>"
	docSchema := revisionEvent(a.publish(artifactStoreType, "doc-type", ""))
	doc := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(docSchema, exact)), "doc", ""))
	ids := []string{doc}
	schemas := map[string]string{"document": docSchema}
	for _, shape := range []string{"ticket", "list"} {
		typeInput := strings.ReplaceAll(strings.ReplaceAll(artifactStoreType, "example.test/note", "example.test/"+shape), `"shape":"document"`, `"shape":"`+shape+`"`)
		schema := revisionEvent(a.publish(typeInput, shape+"-type", ""))
		schemas[shape] = schema
		payload := revisionPayload(schema, exact)
		payload["shape"] = shape
		payload["type_id"] = "example.test/" + shape
		if shape == "list" {
			payload["members"] = []any{map[string]any{"member_id": "z-first", "target": doc, "target_mode": "logical", "required": true}, map[string]any{"member_id": "a-second", "target": ids[1], "target_mode": "revision", "required": true}}
		}
		ids = append(ids, revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, payload), shape, "")))
	}
	list := ids[2]
	artifactJourneyRemote(a, "push", list)
	artifactJourneyRemote(b, "fetch", list)
	for _, id := range ids {
		if !reflect.DeepEqual(artifactJourneyView(t, a, id), artifactJourneyView(t, b, id)) {
			t.Fatal("independent clone view", id)
		}
	}
	before := a.snapshot()
	for _, shape := range []string{"document", "list", "ticket"} {
		p := artifactPage(t, artifactList(a, "", "--shape", shape, "--limit", "200"))
		if len(p["items"].([]any)) != 1 || p["items"].([]any)[0].(map[string]any)["shape"] != shape {
			t.Fatal(p)
		}
		typePage := artifactPage(t, a.call("", "", "artifact", "type", "show", schemas[shape], "--json"))
		if typePage["items"].([]any)[0].(map[string]any)["id"] != schemas[shape] {
			t.Fatal(typePage)
		}
	}
	for _, mode := range []string{"show", "heads", "history"} {
		a.call("", "invalid_input", "artifact", mode, strings.TrimPrefix(doc, "sha256:")[:12], "--json")
	}
	bad := revisionPayload(docSchema, "invalid fields")
	bad["fields"] = map[string]any{"unknown": true}
	revisionCall(a, "open", revisionInput(t, nil, []string{}, bad), "invalid-schema", "invalid_input")
	bad = revisionPayload("sha256:"+strings.Repeat("d", 64), "missing schema")
	revisionCall(a, "open", revisionInput(t, nil, []string{}, bad), "missing-schema", "incomplete_dependency")
	a.unchanged(before)
	// Two actors revise the same exact opening. One loses its acknowledgement.
	right := revisionEvent(revisionCall(b, "revise", revisionInput(t, doc, []string{doc}, revisionPayload(docSchema, "right")), "right", ""))
	leftInput := revisionInput(t, doc, []string{doc}, revisionPayload(docSchema, "left "+exact))
	artifactJourneyLostAck(t, a, leftInput, "left-lost-ack")
	lookup := operationCall(a, "left-lost-ack", "example.test/note", "")["data"].(map[string]any)
	left := lookup["event_id"].(string)
	if lookup["status"] != "found" || lookup["result"].(map[string]any)["replayed"] != false {
		t.Fatal(lookup)
	}
	before = a.snapshot()
	originalIntent := operationIntent(t, a, "left-lost-ack")
	retry := revisionCall(a, "revise", leftInput, "left-lost-ack", "")["data"].(map[string]any)
	if retry["event_id"] != left || retry["replayed"] != true {
		t.Fatal(retry)
	}
	settledIntent := operationIntent(t, a, "left-lost-ack")
	for key, value := range originalIntent {
		if key != "stage" && key != "result" && !reflect.DeepEqual(value, settledIntent[key]) {
			t.Fatal("retry replaced immutable intent", key)
		}
	}
	if settledIntent["stage"] != "published" {
		t.Fatal(settledIntent)
	}
	// Settlement may update only this existing private descriptor; no object,
	// ref, other file or immutable candidate may change during an exact retry.
	delete(before, operationIntentPath(a, "left-lost-ack"))
	afterSettlement := a.snapshot()
	delete(afterSettlement, operationIntentPath(a, "left-lost-ack"))
	if !reflect.DeepEqual(before, afterSettlement) {
		t.Fatal("retry changed public or unrelated private state")
	}
	before = a.snapshot()
	revisionCall(a, "revise", strings.Replace(leftInput, "left ", "changed ", 1), "left-lost-ack", "operation_conflict")
	operationCall(a, "left-lost-ack", "example.test/ticket", "operation_conflict")
	a.unchanged(before)
	artifactJourneyRemote(b, "push", list)
	artifactJourneyRemote(a, "fetch", list)
	heads := []string{left, right}
	sort.Strings(heads)
	view := artifactJourneyView(t, a, doc)
	if view["conflict"] != true || view["state"] != nil || !reflect.DeepEqual(revisionHeads(t, view), heads) {
		t.Fatal(view)
	}
	filtered := artifactPage(t, artifactList(a, "", "--type", "example.test/note", "--limit", "1"))["items"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(filtered, view) {
		t.Fatal("list concealed conflict", filtered, view)
	}
	before = a.snapshot()
	revisionCall(a, "resolve", revisionInput(t, doc, []string{left}, revisionPayload(docSchema, "partial")), "partial", "stale_revision")
	revisionCall(a, "revise", revisionInput(t, doc, []string{left}, revisionPayload(docSchema, "overwrite")), "conflicted", "resource_conflict")
	a.unchanged(before)
	resolved := revisionEvent(revisionCall(a, "resolve", revisionInput(t, doc, heads, revisionPayload(docSchema, "resolved "+exact)), "resolve", ""))
	oldHistory := artifactPage(t, a.call("", "", "artifact", "history", doc, "--json"))["items"].([]any)
	evolved := strings.Replace(strings.Replace(artifactStoreType, `"parents":[]`, `"parents":["`+docSchema+`"]`, 1), `"schema_version":1`, `"schema_version":2`, 1)
	schema2 := revisionEvent(a.publish(evolved, "schema-two", ""))
	payload := revisionPayload(schema2, "converted "+exact)
	payload["conversion_sources"] = []any{resolved}
	converted := revisionEvent(revisionCall(a, "convert", revisionInput(t, doc, []string{resolved}, payload), "convert", ""))
	history := artifactPage(t, a.call("", "", "artifact", "history", doc, "--json"))["items"].([]any)
	for _, old := range oldHistory {
		found := false
		for _, now := range history {
			found = found || reflect.DeepEqual(old, now)
		}
		if !found {
			t.Fatal("signed history reinterpreted", old)
		}
	}
	if len(history) != len(oldHistory)+1 {
		t.Fatal(history)
	}
	view = artifactJourneyView(t, a, doc)
	if view["state"].(map[string]any)["content"] != "converted "+exact || !reflect.DeepEqual(revisionHeads(t, view), []string{converted}) {
		t.Fatal(view)
	}
	// Public file reconstruction needs no registered authoring consumer or identity.
	privateMarker := "WP11-PRIVATE-KEY-CREDENTIAL-GRANT-JOURNAL"
	for _, relative := range []string{".hn/private", ".git/hn/extensions/private", ".git/hn/artifacts/private"} {
		p := filepath.Join(a.repo, relative)
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(privateMarker), 0600); e != nil {
			t.Fatal(e)
		}
	}
	before = a.snapshot()
	path, out := exchangeExport(a, list)
	a.unchanged(before)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(privateMarker)) || out["output_digest"] != eventID(raw) {
		t.Fatal("public export bytes", out)
	}
	if exchangePrivateLeak(append(raw, []byte(privateMarker)...), []string{privateMarker}) != privateMarker {
		t.Fatal("privacy detector control")
	}
	dst := exchangeDestination(t, a, "sha1")
	dst.call("", "", "artifact", "import", "--input", path, "--json")
	for _, id := range ids {
		if !reflect.DeepEqual(artifactJourneyView(t, a, id), artifactJourneyView(t, dst, id)) {
			t.Fatal("file reconstructed view", id)
		}
	}
	if !reflect.DeepEqual(closureItems(a, list), closureItems(dst, list)) {
		t.Fatal("file closure bytes")
	}
	dst.actor = a.actor
	before = dst.snapshot()
	publicLookup := operationCall(dst, "left-lost-ack", "example.test/note", "")["data"].(map[string]any)
	if publicLookup["status"] != "found" || publicLookup["result"].(map[string]any)["replayed"] != false || publicLookup["event_id"] != left {
		t.Fatal(publicLookup)
	}
	artifactList(dst, "", "--limit", "200")
	dst.unchanged(before)
	for _, p := range []string{dst.home, filepath.Join(dst.repo, ".git", "hn"), filepath.Join(dst.repo, ".hn")} {
		if _, err = os.Stat(p); !os.IsNotExist(err) {
			t.Fatal("public-only lookup initialized private state", p, err)
		}
	}
	// Artifact work preserves the existing native facts. The following
	// intentional native identity rotation has its own native signed effects.
	a.call("", "", "issue", "show", nativeID, "--json")
	if got := a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/"); got != nativeRefs {
		t.Fatal("artifact path changed native history", got, nativeRefs)
	}
	// Exact active-actor pin applies to both known and new mutation keys.
	rotate := exec.Command(a.binary, "identity", "rotate", "--name", "Replacement")
	rotate.Dir = a.repo
	rotate.Env = operationalTestEnvironment(a.home)
	if raw, err := rotate.CombinedOutput(); err != nil {
		t.Fatalf("rotate %v %s", err, raw)
	}
	nativeRefs = a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/")
	before = a.snapshot()
	for _, attempt := range []struct{ key, input string }{{"left-lost-ack", leftInput}, {"new-wrong-actor", revisionInput(t, doc, []string{converted}, revisionPayload(schema2, "valid current intent"))}} {
		v := revisionCall(a, "revise", attempt.input, attempt.key, "actor_mismatch")
		artifactErrorShape(t, v, "native", "actor_mismatch", "do_not_retry", nil)
		a.unchanged(before)
	}
	// Native reads keep their native envelope and historical signed refs.
	a.call("", "", "issue", "show", nativeID, "--json")
	if got := a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/"); got != nativeRefs {
		t.Fatal("artifact path changed native history", got, nativeRefs)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("stored text executed", err)
	}
	t.Logf("three shapes, immutable history, conflict/resolve/convert and public-only recovery: %s %s %s", doc, ids[1], list)
}

func TestArtifactCombinedOpaqueAndNativeCLI(t *testing.T) {
	// The malformed signature is hash-consistent public Git data, so its precise
	// failure is signature verification, rather than an unrelated object hash.
	src := artifactCLI(t)
	bad := artifactRawCommit(t, src, artifactVector, strings.Repeat("A", 86), "", "hn artifact "+artifactVectorID, false)
	ref := artifactFrozenRef(t, src)
	src.git("update-ref", ref, bad)
	bundle := exchangeGitOnlyBundle(t, src, []artifactSource{{ref, bad}})
	path := exchangeWrite(t, bundle)
	dst := exchangeDestination(t, src, "sha1")
	before := dst.snapshot()
	dst.call("", "verification_failure", "artifact", "import", "--input", path, "--json")
	dst.unchanged(before)
	opaque := dst.call("", "", "artifact", "import", "--input", path, "--preserve-opaque", "--json")["data"].(map[string]any)
	if opaque["mode"] != "opaque" || len(opaque["accepted_refs"].([]any)) != 0 || len(opaque["imported_ids"].([]any)) != 0 {
		t.Fatal(opaque)
	}
	for i, object := range bundle.Objects {
		id := eventID(append([]byte(object.Type+"\x00"), object.body...))
		view := dst.call("", "", "artifact", "inspect", "--opaque", "--opaque-id", id, "--json")["data"].(map[string]any)
		if view["id"] != nil || view["envelope"] != nil || view["signature"] != nil || view["validation"] != "opaque" || view["raw"].(map[string]any)["data"] != object.Data {
			t.Fatal(view)
		}
		output := filepath.Join(t.TempDir(), fmt.Sprintf("opaque-%d.json", i))
		dst.call("", "", "artifact", "export", "--opaque", "--opaque-id", id, "--output", output, "--json")
		reconstructed := exchangeDestination(t, src, "sha1")
		reconstructed.call("", "", "artifact", "import", "--input", output, "--preserve-opaque", "--json")
		if !reflect.DeepEqual(view, reconstructed.call("", "", "artifact", "inspect", "--opaque", "--opaque-id", id, "--json")["data"]) {
			t.Fatal("opaque raw bytes changed")
		}
	}
	before = dst.snapshot()
	page := artifactPage(t, artifactList(dst, "", "--limit", "1"))
	if len(page["items"].([]any)) != 0 {
		t.Fatal("opaque acquired accepted visibility", page)
	}
	dst.unchanged(before)
	before = src.snapshot()
	artifactList(src, "verification_failure", "--limit", "1")
	src.unchanged(before)
	// Unknown/malformed custom data cannot pollute native event decoding.
	src.call("", "", "issue", "open", "--operation", "native-with-malformed-custom", "--json", "Native after opaque diagnosis")
	native := src.call("", "", "issue", "list", "--json")["data"].(map[string]any)
	if native["total"] != float64(1) {
		raw, _ := json.Marshal(native)
		t.Fatal(string(raw))
	}
}
