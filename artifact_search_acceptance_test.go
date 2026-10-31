package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func artifactSearchCall(f *artifactCLIFixture, code string, args ...string) map[string]any {
	f.t.Helper()
	return f.call("", code, append([]string{"artifact", "search", "--json"}, args...)...)
}

func artifactSearchIDs(t *testing.T, response map[string]any) []string {
	t.Helper()
	page := artifactPage(t, response)
	ids := []string{}
	for _, value := range page["items"].([]any) {
		view := value.(map[string]any)
		if len(view) != 8 || view["protocol"] != "hn-artifact/1" || view["diagnostics"] == nil || view["heads"] == nil {
			t.Fatal("incomplete RecordView", view)
		}
		ids = append(ids, view["id"].(string))
	}
	return ids
}

func artifactSearchWant(t *testing.T, response map[string]any, wanted ...string) {
	t.Helper()
	sort.Strings(wanted)
	got := artifactSearchIDs(t, response)
	if len(got) != len(wanted) || len(got) != 0 && !reflect.DeepEqual(got, wanted) {
		t.Fatal("search IDs", got, wanted)
	}
}

func TestArtifactSearchHelpAndPublicOnlyReaderCLI(t *testing.T) {
	f := artifactCLI(t)
	// Explicit artifact help exposes the complete live search usage in both forms.
	// Running against a public-only empty reader must not initialize an identity.
	empty := exchangeDestination(t, f, "sha1")
	before := empty.snapshot()
	const usage = "hn artifact search [--type TYPE_ID] --literal TEXT|--regex PATTERN --limit 1..200 [--cursor TOKEN] [--json]"
	for _, name := range []string{"help", "--help"} {
		cmd := exec.Command(empty.binary, "artifact", name)
		cmd.Dir, cmd.Env = empty.repo, operationalTestEnvironment(empty.home)
		raw, err := cmd.Output()
		if err != nil || strings.Count(string(raw), usage) != 1 || !strings.Contains(string(raw), "hn artifact list") || !strings.Contains(string(raw), "inert") {
			t.Fatalf("combined artifact %s help: %v %s", name, err, raw)
		}
	}
	for _, mode := range []string{"--literal", "--regex"} {
		artifactSearchWant(t, artifactSearchCall(empty, "", mode, "", "--limit", "1"))
	}
	empty.unchanged(before)
	if _, err := os.Stat(empty.home); !os.IsNotExist(err) {
		t.Fatal("public-only search/help initialized private home", err)
	}
}

func TestArtifactSearchPublicCLI(t *testing.T) {
	f := artifactCLI(t)
	binaryBytes, err := os.ReadFile(f.binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("supplied CLI SHA256 %x", sha256.Sum256(binaryBytes))
	schema := revisionEvent(f.publish(artifactStoreType, "search-type", ""))
	id := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "雪\nsearchable public text needle[42]\ne\u0301")), "search-open", ""))
	before := f.snapshot()
	page := artifactPage(t, f.call("", "", "artifact", "search", "--literal", "雪", "--limit", "1", "--json"))
	items := page["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != id {
		t.Fatal(page)
	}
	f.unchanged(before)
	for _, args := range [][]string{
		{"--literal", "needle[42]"}, {"--regex", `needle\[42\]`},
		{"--regex", `(?m)^雪$`}, {"--regex", `(?s)雪.*public`},
		{"--literal", "e\u0301"}, {"--literal", "Revision data"},
		{"--literal", schema}, {"--literal", id}, {"--literal", f.actor},
		{"--literal", "example.test/note"}, {"--literal", "document"},
	} {
		artifactSearchWant(t, artifactSearchCall(f, "", append(args, "--limit", "200")...), id)
	}
	for _, args := range [][]string{{"--literal", "é"}, {"--regex", `needle[42]`}, {"--regex", `(?s)Revision data.*雪`}, {"--literal", "missing-token"}} {
		artifactSearchWant(t, artifactSearchCall(f, "", append(args, "--limit", "200")...))
	}
	artifactSearchWant(t, artifactSearchCall(f, "", "--regex", strings.Repeat("😀", 1024), "--limit", "1"))
	f.unchanged(before)
	// Generic structured data and public type metadata have the same behavior
	// for unrelated inventory and charter-like application names.
	for _, name := range []string{"inventory.test/part", "planning.test/directive"} {
		typeInput := strings.ReplaceAll(artifactStoreType, "example.test/note", name)
		typeInput = strings.Replace(typeInput, `"properties":{}`, `"properties":{"label":{"type":"string","maxBytes":64}}`, 1)
		typeID := revisionEvent(f.publish(typeInput, name+"-type", ""))
		p := revisionPayload(typeID, "Same inert generic text")
		p["type_id"], p["fields"] = name, map[string]any{"label": "part-雪"}
		wanted := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, p), name+"-open", ""))
		before = f.snapshot()
		for _, text := range []string{name, typeID, `"label":"part-雪"`} {
			artifactSearchWant(t, artifactSearchCall(f, "", "--type", name, "--literal", text, "--limit", "200"), wanted)
		}
		artifactSearchWant(t, artifactSearchCall(f, "", "--type", "example.test/note", "--literal", "part-雪", "--limit", "1"))
		f.unchanged(before)
	}
	for _, shape := range []string{"list", "ticket"} {
		typeName := "example.test/" + shape
		typeInput := strings.ReplaceAll(artifactStoreType, "example.test/note", typeName)
		typeInput = strings.Replace(typeInput, `"shape":"document"`, `"shape":"`+shape+`"`, 1)
		typeRevision := revisionEvent(f.publish(typeInput, shape+"-type", ""))
		p := revisionPayload(typeRevision, "shape-independent-token")
		p["type_id"], p["shape"] = typeName, shape
		missing := "sha256:" + strings.Repeat("d", 64)
		p["relationships"] = []any{map[string]any{"relation": "example.test/points", "target": missing, "target_mode": "revision", "required": false}}
		id := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, p), shape+"-open", ""))
		before = f.snapshot()
		page := artifactPage(t, artifactSearchCall(f, "", "--type", typeName, "--literal", "shape-independent-token", "--limit", "1"))
		view := page["items"].([]any)[0].(map[string]any)
		diagnostics := view["diagnostics"].([]any)
		if view["id"] != id || view["shape"] != shape || len(diagnostics) != 1 || diagnostics[0].(map[string]any)["code"] != "missing_optional" || diagnostics[0].(map[string]any)["id"] != missing || !reflect.DeepEqual(view, revisionReadView(f, id)) {
			t.Fatal("search omitted full state or validation diagnostics", view)
		}
		f.unchanged(before)
	}
}

func TestArtifactSearchPagingAndRefusalsCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	wanted := []string{}
	for i := 0; i < 14; i++ {
		text := "unrelated public data"
		if i%3 == 0 {
			text = "page-token"
		}
		id := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, text)), fmt.Sprint("open-", i), ""))
		if i%3 == 0 {
			wanted = append(wanted, id)
		}
	}
	sort.Strings(wanted)
	before := f.snapshot()
	first := artifactPage(t, artifactSearchCall(f, "", "--literal", "page-token", "--limit", "1"))
	cursor := first["next_cursor"].(string)
	got := []string{first["items"].([]any)[0].(map[string]any)["id"].(string)}
	cut := first["source_cut"]
	for token := cursor; token != ""; {
		next := artifactPage(t, artifactSearchCall(f, "", "--literal", "page-token", "--limit", "2", "--cursor", token))
		if next["source_cut"] != cut {
			t.Fatal("page source changed", next)
		}
		for _, item := range next["items"].([]any) {
			got = append(got, item.(map[string]any)["id"].(string))
		}
		token, _ = next["next_cursor"].(string)
	}
	if !reflect.DeepEqual(got, wanted) {
		t.Fatal("paging omitted or repeated matches", got, wanted)
	}
	for _, args := range [][]string{
		{"--literal", "changed", "--cursor", cursor},
		{"--regex", "page-token", "--cursor", cursor},
		{"--literal", "page-token", "--type", "example.test/note", "--cursor", cursor},
		{"--literal", "page-token", "--cursor", "malformed"},
		{"--literal", "page-token", "--cursor", base64.RawURLEncoding.EncodeToString([]byte(`{"version":1,"offset":1,"unknown":true}`))},
	} {
		v := artifactSearchCall(f, "checkpoint_invalidated", append(args, "--limit", "1")...)
		e := v["error"].(map[string]any)
		if e["origin"] != "host" || e["recovery"] != "rescan_required" || e["operation"] != nil {
			t.Fatal(v)
		}
	}
	for _, args := range [][]string{
		{"--limit", "1"}, {"--literal", "x", "--regex", "x", "--limit", "1"},
		{"--literal", "x", "--literal", "x", "--limit", "1"}, {"--literal", "x", "--limit", "0"},
		{"--literal", "x"}, {"--literal", "x", "--shape", "document", "--limit", "1"},
		{"--literal", "x", "--field", "label", "--limit", "1"}, {"--literal", "x", "--type", "hn/issue", "--limit", "1"},
		{"--regex", "[", "--limit", "1"}, {"--regex", `(?=x)`, "--limit", "1"},
	} {
		artifactSearchCall(f, "invalid_input", args...)
	}
	for _, args := range [][]string{
		{"--literal", "x", "--limit", "201"},
		{"--regex", strings.Repeat("😀", 1024) + "a", "--limit", "1"},
		{"--regex", strings.Repeat("a", 4095), "--limit", "1"},
		{"--literal", "x", "--limit", "1", "--cursor", strings.Repeat("x", 4097)},
	} {
		artifactSearchCall(f, "resource_limit", args...)
	}
	f.unchanged(before)
	// A new accepted signed revision changes the exact cut even when the text
	// still matches; continuation must explicitly require a rescan.
	revisionCall(f, "revise", revisionInput(t, wanted[0], []string{wanted[0]}, revisionPayload(schema, "page-token revised")), "changed-cut", "")
	before = f.snapshot()
	artifactSearchCall(f, "checkpoint_invalidated", "--literal", "page-token", "--limit", "1", "--cursor", cursor)
	f.unchanged(before)
}

func TestArtifactSearchEntireSelectionBudgetCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	pattern := strings.Repeat("a", 4094) // Exactly 4096 compiled instructions.
	// Independent frozen public text accounting: two full IDs, type, shape,
	// actor, exact schema, title, content and canonical fields. 16384 bytes at
	// 4096 instructions is exactly 67108864; a new revision adds just one byte.
	overhead := 2*71 + len("example.test/note") + len("document") + len(f.actor) + len(schema) + len("Revision data") + len("{}")
	content := pattern + strings.Repeat("b", 16384-overhead-len(pattern))
	id := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, content)), "early-match", ""))
	artifactSearchWant(t, artifactSearchCall(f, "", "--regex", pattern, "--limit", "1"), id)
	revisionCall(f, "revise", revisionInput(t, id, []string{id}, revisionPayload(schema, content+"b")), "work-one-over", "")
	before := f.snapshot()
	artifactSearchCall(f, "resource_limit", "--regex", pattern, "--limit", "1")
	f.unchanged(before)
	// A non-matching record alone exhausts work. It must not be hidden by the
	// first matching page, and the complete error envelope contains no results.
	revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, strings.Repeat("b", 17000))), "late-expensive", "")
	before = f.snapshot()
	artifactSearchCall(f, "resource_limit", "--regex", pattern, "--limit", "1")
	artifactSearchWant(t, artifactSearchCall(f, "", "--literal", pattern, "--limit", "1"), id)
	f.unchanged(before)
	other := strings.ReplaceAll(artifactStoreType, "example.test/note", "other.test/note")
	otherSchema := revisionEvent(f.publish(other, "other-type", ""))
	p := revisionPayload(otherSchema, pattern)
	p["type_id"] = "other.test/note"
	otherID := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, p), "filtered-match", ""))
	before = f.snapshot()
	artifactSearchWant(t, artifactSearchCall(f, "", "--type", "other.test/note", "--regex", pattern, "--limit", "1"), otherID)
	artifactSearchCall(f, "resource_limit", "--regex", pattern, "--limit", "1")
	f.unchanged(before)
	// A changed cut invalidates continuation explicitly, even when the changed
	// selection would now exceed the regex budget. The caller must rescan.
	revisionCall(f, "open", revisionInput(t, nil, []string{}, p), "filtered-second", "")
	first := artifactPage(t, artifactSearchCall(f, "", "--type", "other.test/note", "--regex", pattern, "--limit", "1"))
	cursor := first["next_cursor"].(string)
	p["content"] = strings.Repeat("b", 17000)
	revisionCall(f, "open", revisionInput(t, nil, []string{}, p), "filtered-changed-cut", "")
	before = f.snapshot()
	artifactSearchCall(f, "checkpoint_invalidated", "--type", "other.test/note", "--regex", pattern, "--limit", "1", "--cursor", cursor)
	artifactSearchCall(f, "resource_limit", "--type", "other.test/note", "--regex", pattern, "--limit", "1")
	f.unchanged(before)
}

func TestArtifactSearchOversizedPageCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	content := "complete-large-state " + strings.Repeat("x", 900000)
	for i := 0; i < 5; i++ {
		revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, content)), fmt.Sprint("large-", i), "")
	}
	before := f.snapshot()
	artifactSearchCall(f, "resource_limit", "--literal", "complete-large-state", "--limit", "200")
	page := artifactPage(t, artifactSearchCall(f, "", "--literal", "complete-large-state", "--limit", "1"))
	item := page["items"].([]any)[0].(map[string]any)
	if item["state"].(map[string]any)["content"] != content || page["next_cursor"] == nil {
		t.Fatal("page silently truncated state or matches")
	}
	f.unchanged(before)
}

func TestArtifactSearchConflictAndUnselectedCLI(t *testing.T) {
	a, b := artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	id := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "superseded-only-token")), "opening", ""))
	b.git("fetch", a.repo, "+refs/hn/artifacts/*:refs/hn/remotes/public/artifacts/*")
	left := revisionEvent(revisionCall(a, "revise", revisionInput(t, id, []string{id}, revisionPayload(schema, "left-match")), "left", ""))
	right := revisionEvent(revisionCall(b, "revise", revisionInput(t, id, []string{id}, revisionPayload(schema, "right-match")), "right", ""))
	a.git("fetch", b.repo, "+refs/hn/artifacts/*:refs/hn/remotes/peer/artifacts/*")
	before := a.snapshot()
	for _, text := range []string{"left-match", "right-match"} {
		page := artifactPage(t, artifactSearchCall(a, "", "--literal", text, "--limit", "1"))
		view := page["items"].([]any)[0].(map[string]any)
		wantHeads := []string{left, right}
		sort.Strings(wantHeads)
		if view["id"] != id || view["conflict"] != true || view["state"] != nil || !reflect.DeepEqual(revisionHeads(t, view), wantHeads) {
			t.Fatal("search concealed a conflict", view)
		}
	}
	artifactSearchWant(t, artifactSearchCall(a, "", "--literal", "superseded-only-token", "--limit", "200"))
	a.unchanged(before)
	// Signed public artifacts fetched into an unrelated namespace remain outside
	// accepted search even though their objects are physically present.
	c := artifactCLI(t)
	cs := revisionEvent(c.publish(artifactStoreType, "hidden-type", ""))
	revisionCall(c, "open", revisionInput(t, nil, []string{}, revisionPayload(cs, "unselected-secret-token")), "hidden", "")
	a.git("fetch", c.repo, "+refs/hn/artifacts/*:refs/raw/artifacts/*")
	before = a.snapshot()
	artifactSearchWant(t, artifactSearchCall(a, "", "--literal", "unselected-secret-token", "--limit", "200"))
	a.unchanged(before)
}

func TestArtifactSearchInertConsumerControlCLI(t *testing.T) {
	f := artifactCLI(t)
	config := filepath.Join(f.home, "config")
	f.overrides = map[string]string{"XDG_CONFIG_HOME": config}
	registry := filepath.Join(config, "hubnot", "extensions", "registry.json")
	marker := filepath.Join(t.TempDir(), "consumer-started")
	consumer := filepath.Join(t.TempDir(), "crashing-consumer")
	if err := os.WriteFile(consumer, []byte("#!/bin/sh\nprintf started > '"+marker+"'\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	content := "$(" + consumer + ")\n<script>throw 1</script>\n\x1b[31m inert-consumer-token"
	id := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, content)), "inert", ""))
	// Exercise absent, disabled, crashing and malformed private consumer state. The
	// trap names the actual host registry path; no registrar is needed for reads.
	for _, state := range []string{"absent", "disabled", "crashing", "malformed"} {
		if state != "absent" {
			if err := os.MkdirAll(filepath.Dir(registry), 0700); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(map[string]any{"disabled": state == "disabled", "executable": consumer, "privateCredential": "PRIVATE-WP12"})
			if err != nil {
				t.Fatal(err)
			}
			if state == "malformed" {
				raw = raw[:len(raw)-1]
			}
			if err := os.WriteFile(registry, raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		before := f.snapshot()
		result := artifactSearchCall(f, "", "--literal", "inert-consumer-token", "--limit", "1")
		artifactSearchWant(t, result, id)
		raw, _ := json.Marshal(result)
		if bytes.Contains(raw, []byte("PRIVATE-WP12")) {
			t.Fatal("private state leaked", string(raw))
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("search started consumer", err)
		}
		f.unchanged(before)
		trace := extensionBaselineTraceRun(t, operationalTestEnvironmentWith(f.home, f.overrides), f.repo, f.binary, "artifact", "search", "--literal", "inert-consumer-token", "--limit", "1", "--json")
		if strings.Contains(trace, registry) || strings.Contains(trace, consumer) || strings.Contains(trace, "connect(") {
			t.Fatal("registry read, consumer start or provider connection", trace)
		}
		f.unchanged(before)
	}
	// The real process deliberately violates the prohibition. Prove the marker
	// detector observes that exact effect, even though this consumer crashes.
	if err := exec.Command(consumer).Run(); err == nil {
		t.Fatal("control did not crash")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "started" {
		t.Fatal("process marker detector missed deliberate start", err)
	}
	t.Log("process marker control detected deliberate consumer start")
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	// Subsequent native issue and memory reads still use the original pathways.
	f.call("", "", "issue", "open", "--operation", "native-after-search", "--json", "Native after search")
	f.call("", "", "issue", "list", "--json")
	f.git("-c", "user.name=Search test", "-c", "user.email=search@example.test", "commit", "--allow-empty", "-qm", "memory anchor")
	for _, args := range [][]string{
		{"memory", "record", "--kind", "observation", "--at", "HEAD", "--applies", "exact", "--content", "native-memory-after-search", "--json"},
		{"memory", "recall", "--at", "HEAD", "--include-untrusted", "--query", "native-memory-after-search", "--json"},
		{"memory", "grep", "--at", "HEAD", "--include-untrusted", "--json", "native-memory-after-search"},
	} {
		cmd := exec.Command(f.binary, args...)
		cmd.Dir, cmd.Env = f.repo, operationalTestEnvironmentWith(f.home, f.overrides)
		out, err := cmd.CombinedOutput()
		if err != nil || !json.Valid(out) {
			t.Fatalf("native memory %v: %v %s", args, err, out)
		}
		if args[1] != "record" {
			var result MemoryRecallEnvelopeV0
			if err := json.Unmarshal(out, &result); err != nil || result.Returned != 1 || result.Memories[0].Data.Content != "native-memory-after-search" {
				t.Fatalf("native memory did not retain its data: %v %s", err, out)
			}
		}
	}
	before := f.snapshot()
	for _, args := range [][]string{
		{"--help"}, {"issue", "list", "--json"},
		{"memory", "recall", "--at", "HEAD", "--include-untrusted", "--query", "native-memory-after-search", "--json"},
		{"memory", "grep", "--at", "HEAD", "--include-untrusted", "--json", "native-memory-after-search"},
	} {
		trace := extensionBaselineTraceRun(t, operationalTestEnvironmentWith(f.home, f.overrides), f.repo, f.binary, args...)
		if strings.Contains(trace, registry) || strings.Contains(trace, consumer) || strings.Contains(trace, "connect(") {
			t.Fatal("native command read registry, started consumer or connected provider", args, trace)
		}
		f.unchanged(before)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("native journey started consumer", err)
	}
}
