package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func artifactList(f *artifactCLIFixture, code string, args ...string) map[string]any {
	f.t.Helper()
	return f.call("", code, append([]string{"artifact", "list", "--json"}, args...)...)
}
func artifactPage(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	if len(result) != 3 || result["ok"] != true || result["error"] != nil {
		t.Fatal("success envelope", result)
	}
	page := result["data"].(map[string]any)
	if len(page) != 3 || page["source_cut"] == "" {
		t.Fatal("Page contract", page)
	}
	for _, k := range []string{"items", "source_cut", "next_cursor"} {
		if _, ok := page[k]; !ok {
			t.Fatal("missing Page field", k)
		}
	}
	return page
}
func artifactPageIDs(t *testing.T, page map[string]any) []string {
	t.Helper()
	ids := []string{}
	for _, raw := range page["items"].([]any) {
		ids = append(ids, raw.(map[string]any)["id"].(string))
	}
	return ids
}
func TestArtifactListLiveCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	ids := []string{}
	for index, text := range []string{"first\n雪 e\u0301", "second", "third"} {
		ids = append(ids, revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, text)), "open-"+strconv.Itoa(index), "")))
	}
	sort.Strings(ids)
	before := f.snapshot()
	first := artifactPage(t, artifactList(f, "", "--limit", "1"))
	if !reflect.DeepEqual(artifactPageIDs(t, first), ids[:1]) || first["next_cursor"] == nil {
		t.Fatal(first, ids)
	}
	cursor := first["next_cursor"].(string)
	second := artifactPage(t, artifactList(f, "", "--limit", "1", "--cursor", cursor))
	if !reflect.DeepEqual(artifactPageIDs(t, second), ids[1:2]) || second["source_cut"] != first["source_cut"] || second["next_cursor"] == nil {
		t.Fatal(second)
	}
	last := artifactPage(t, artifactList(f, "", "--limit", "1", "--cursor", second["next_cursor"].(string)))
	if !reflect.DeepEqual(artifactPageIDs(t, last), ids[2:]) || last["next_cursor"] != nil {
		t.Fatal(last)
	}
	all := artifactPage(t, artifactList(f, "", "--limit", "200", "--type", "example.test/note", "--shape", "document"))
	if !reflect.DeepEqual(artifactPageIDs(t, all), ids) || all["next_cursor"] != nil {
		t.Fatal(all)
	}
	empty := artifactPage(t, artifactList(f, "", "--limit", "1", "--shape", "ticket"))
	if len(empty["items"].([]any)) != 0 || empty["next_cursor"] != nil {
		t.Fatal(empty)
	}
	for _, args := range [][]string{
		{}, {"--limit", "0"}, {"--limit", "-1"}, {"--limit", "one"}, {"--limit", "1", "--limit", "1"},
		{"--limit", "1", "--shape", "unknown"}, {"--limit", "1", "--type", "hn/issue"},
		{"--limit", "1", "--type", "note"}, {"--limit", "1", "--shape", ""},
		{"--limit", "1", "--cursor", ""},
		{"--limit", "1", "--json"}, {"--limit", "1", "--unknown", "x"},
	} {
		v := artifactList(f, "invalid_input", args...)
		artifactErrorShape(t, v, "artifact", "invalid_input", "do_not_retry", nil)
		f.unchanged(before)
	}
	changedSize := artifactPage(t, artifactList(f, "", "--limit", "200", "--cursor", cursor))
	if !reflect.DeepEqual(artifactPageIDs(t, changedSize), ids[1:]) || changedSize["next_cursor"] != nil || changedSize["source_cut"] != first["source_cut"] {
		t.Fatal("changed page size skipped results", changedSize)
	}
	for _, args := range [][]string{{"--limit", "1", "--cursor", "malformed"}, {"--limit", "1", "--type", "example.test/note", "--cursor", cursor}, {"--limit", "1", "--shape", "document", "--cursor", cursor}} {
		invalidCursor := artifactList(f, "checkpoint_invalidated", args...)
		artifactErrorShape(t, invalidCursor, "host", "checkpoint_invalidated", "rescan_required", nil)
		f.unchanged(before)
	}
	rawCursor, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(rawCursor, &fields); err != nil {
		t.Fatal(err)
	}
	fields["offset"] = 999999
	rawCursor, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{strings.Repeat("x", 4096), base64.RawURLEncoding.EncodeToString(rawCursor)} {
		refused := artifactList(f, "checkpoint_invalidated", "--limit", "1", "--cursor", token)
		artifactErrorShape(t, refused, "host", "checkpoint_invalidated", "rescan_required", nil)
		f.unchanged(before)
	}
	artifactList(f, "resource_limit", "--limit", "201")
	artifactList(f, "resource_limit", "--limit", "1", "--cursor", strings.Repeat("x", 4097))
	f.unchanged(before)
	revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "changes selected cut")), "later", "")
	before = f.snapshot()
	stale := artifactList(f, "checkpoint_invalidated", "--limit", "1", "--cursor", cursor)
	artifactErrorShape(t, stale, "host", "checkpoint_invalidated", "rescan_required", nil)
	f.unchanged(before)
}
func TestArtifactLocalHelpAndHumanListCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	id := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "data only")), "open", ""))
	before := f.snapshot()
	for _, args := range [][]string{{"artifact", "help"}, {"artifact", "--help"}, {"artifact", "list", "--limit", "1"}} {
		cmd := exec.Command(f.binary, args...)
		cmd.Dir = f.repo
		cmd.Env = operationalTestEnvironment(f.home)
		raw, err := cmd.Output()
		if err != nil {
			t.Fatalf("human %v: %v %s", args, err, raw)
		}
		if json.Valid(raw) {
			t.Fatal("human list/help must be descriptive", string(raw))
		}
		if args[1] == "list" {
			if !strings.Contains(string(raw), id) || !strings.Contains(string(raw), "example.test/note") {
				t.Fatal(string(raw))
			}
		} else {
			for _, word := range []string{"list", "show", "heads", "history", "operation", "export", "import", "fetch", "push", "inspect", "inert"} {
				if !strings.Contains(string(raw), word) {
					t.Fatal("help omits", word, string(raw))
				}
			}
		}
	}
	f.unchanged(before)
	// An empty public-only reader must neither initialize identity nor private state.
	empty := exchangeDestination(t, f, "sha1")
	before = empty.snapshot()
	page := artifactPage(t, artifactList(empty, "", "--limit", "1"))
	if len(page["items"].([]any)) != 0 {
		t.Fatal(page)
	}
	empty.unchanged(before)
	if _, err := os.Stat(empty.home); !os.IsNotExist(err) {
		t.Fatal("read initialized home", err)
	}
}

func artifactListRaw(t *testing.T, f *artifactCLIFixture, limit string) []byte {
	t.Helper()
	cmd := exec.Command(f.binary, "artifact", "list", "--limit", limit, "--json")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironment(f.home)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("list output: %v %s", err, raw)
	}
	return raw
}
func TestArtifactListExactPageBoundCLI(t *testing.T) {
	a, b := artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	const contentBytes = 820000
	payload := func(n int) map[string]any { return revisionPayload(schema, strings.Repeat("x", n)) }
	doc := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, payload(contentBytes)), "doc", ""))
	for i := 0; i < 4; i++ {
		revisionCall(a, "open", revisionInput(t, nil, []string{}, payload(contentBytes)), "other-"+strconv.Itoa(i), "")
	}
	revisionFetch(b, a, "a")
	revisionCall(a, "revise", revisionInput(t, doc, []string{doc}, payload(contentBytes)), "left", "")
	right := revisionEvent(revisionCall(b, "revise", revisionInput(t, doc, []string{doc}, payload(contentBytes)), "right", ""))
	revisionFetch(a, b, "b")
	raw := artifactListRaw(t, a, "5")
	// One conflicted head contributes its content once (state=null), so a byte
	// added to this independent actor's head adds exactly one response byte.
	// Measure the real envelope, including its terminating newline, rather than
	// deriving an expected size from a production serializer or resource constant.
	gap := 8388608 - len(raw)
	if gap < 1 || contentBytes+gap >= 1040000 {
		t.Fatal("fixture cannot reach exact published bound", gap)
	}
	right = revisionEvent(revisionCall(b, "revise", revisionInput(t, doc, []string{right}, payload(contentBytes+gap)), "at-bound", ""))
	revisionFetch(a, b, "b")
	before := a.snapshot()
	raw = artifactListRaw(t, a, "5")
	if len(raw) != 8388608 {
		t.Fatal("not exact encoded page boundary", len(raw))
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	page := artifactPage(t, result)
	if len(page["items"].([]any)) != 5 || page["next_cursor"] != nil {
		t.Fatal("at-bound silently truncated", page["next_cursor"])
	}
	a.unchanged(before)
	revisionCall(b, "revise", revisionInput(t, doc, []string{right}, payload(contentBytes+gap+1)), "one-over", "")
	revisionFetch(a, b, "b")
	before = a.snapshot()
	refused := artifactList(a, "resource_limit", "--limit", "5")
	artifactErrorShape(t, refused, "artifact", "resource_limit", "do_not_retry", nil)
	if !strings.Contains(refused["error"].(map[string]any)["message"].(string), "reduce --limit") {
		t.Fatal(refused)
	}
	a.unchanged(before)
	t.Logf("encoded list response: at=8388608 accepted; one additional UTF-8 byte in a sole conflict head refused resource_limit before output; calibrated head content=%d", contentBytes+gap)
	first := artifactPage(t, artifactList(a, "", "--limit", "1"))
	remaining := artifactPage(t, artifactList(a, "", "--limit", "200", "--cursor", first["next_cursor"].(string)))
	if len(remaining["items"].([]any)) != 4 || remaining["next_cursor"] != nil || remaining["source_cut"] != first["source_cut"] {
		t.Fatal("narrowed page lost continuation", remaining["next_cursor"])
	}
	a.unchanged(before)
}

func TestArtifactListFrozenShapeInventory(t *testing.T) {
	artifactListShapes := map[string]bool{}
	for _, value := range artifactListShapeVocabulary() {
		artifactListShapes[value] = true
	}

	// artifact-v1 section 2 and host-v1 section 4 define this closed family.
	expected := map[string]bool{"document": true, "list": true, "ticket": true}
	if !reflect.DeepEqual(artifactListShapes, expected) {
		t.Fatal("frozen shape inventory drift", artifactListShapes)
	}
}
