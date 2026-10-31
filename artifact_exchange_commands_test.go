package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The same detector consumes public files and command diagnostics. Its poisoned
// real-file control below proves a prohibited marker is actually observable.
func exchangePrivateLeak(raw []byte, markers []string) string {
	for _, marker := range markers {
		if bytes.Contains(raw, []byte(marker)) {
			return marker
		}
	}
	return ""
}

func exchangeDestination(t *testing.T, source *artifactCLIFixture, format string) *artifactCLIFixture {
	t.Helper()
	root := t.TempDir()
	dst := &artifactCLIFixture{t: t, binary: source.binary, repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
	dst.git("init", "--quiet", "--object-format="+format)
	return dst
}
func exchangeWrite(t *testing.T, value any) string {
	t.Helper()
	raw, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "exchange.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func exchangeExport(f *artifactCLIFixture, ids ...string) (string, map[string]any) {
	f.t.Helper()
	path := filepath.Join(f.t.TempDir(), "public.json")
	args := []string{"artifact", "export", "--output", path, "--json"}
	for _, id := range ids {
		args = append(args, "--id", id)
	}
	return path, f.call("", "", args...)["data"].(map[string]any)
}
func TestArtifactExchangeIndependentCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			src := artifactCLIFormat(t, format)
			dst := exchangeDestination(t, src, format)
			typ := revisionEvent(src.publish(artifactStoreType, "type", ""))
			opening := revisionEvent(revisionCall(src, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "public exact text\n世界")), "open", ""))
			tip := src.git("rev-parse", artifactRef(src.actor))
			tail := revisionEvent(revisionCall(src, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "unselected tail")), "tail", ""))
			marker := "WP10-PRIVATE-CREDENTIAL-KEY-GRANT-JOURNAL-SENTINEL"
			for _, relative := range []string{".hn/private-key-marker", ".git/hn/extensions/grants/private", ".git/hn/artifacts/intents/private"} {
				p := filepath.Join(src.repo, relative)
				if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(p, []byte(marker), 0600); e != nil {
					t.Fatal(e)
				}
			}
			before := src.snapshot()
			if evidence := os.Getenv("HN_WP10_EVIDENCE_DIR"); evidence != "" {
				b, _ := json.MarshalIndent(map[string]any{"source": before, "destination": dst.snapshot(), "type": typ, "opening": opening, "tip": tip, "actor": src.actor}, "", "  ")
				if e := os.WriteFile(filepath.Join(evidence, "initial-fixture.json"), b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			path, out := exchangeExport(src, opening)
			src.unchanged(before)
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			markers := []string{marker, src.home, src.repo}
			metadata, _ := json.Marshal(out)
			for _, public := range [][]byte{raw, metadata} {
				if found := exchangePrivateLeak(public, markers); found != "" {
					t.Fatal("private export/diagnostic leak", found)
				}
			}
			// Deliberately leak into a real file and feed it through the SAME
			// detector; this is evidence that absence is not a broken scanner.
			poisoned := filepath.Join(t.TempDir(), "deliberate-private-leak.json")
			leaked, _ := json.Marshal(map[string]any{"diagnostics": []string{marker}})
			if e = os.WriteFile(poisoned, leaked, 0600); e != nil {
				t.Fatal(e)
			}
			leaked, e = os.ReadFile(poisoned)
			if e != nil || exchangePrivateLeak(leaked, markers) != marker {
				t.Fatal("private leak control was not detected", e)
			}
			if out["output_digest"] != eventID(raw) || out["records"] != float64(2) {
				t.Fatal(out)
			}
			var wire map[string]any
			if e = json.Unmarshal(raw, &wire); e != nil {
				t.Fatal(e)
			}
			if wire["protocol"] != "hn-artifact-exchange/1" || wire["mode"] != "accepted" {
				t.Fatal(wire)
			}
			refs := wire["refs"].([]any)
			if len(refs) != 1 || refs[0].(map[string]any)["oid"] != tip || refs[0].(map[string]any)["ref"] != artifactRef(src.actor) {
				t.Fatal("not minimal canonical prefix", refs)
			}
			again, _ := exchangeExport(src, opening)
			second, _ := os.ReadFile(again)
			if !bytes.Equal(raw, second) {
				t.Fatal("nondeterministic export")
			}
			result := dst.call("", "", "artifact", "import", "--input", path, "--json")["data"].(map[string]any)
			diagnostic, _ := json.Marshal(result)
			if found := exchangePrivateLeak(diagnostic, markers); found != "" {
				t.Fatal("import diagnostics leaked private state", found)
			}
			ids := []any{typ, opening}
			if strings.Compare(typ, opening) > 0 {
				ids = []any{opening, typ}
			}
			if !reflect.DeepEqual(result["imported_ids"], ids) || result["source_cut"] == "" {
				t.Fatal(result)
			}
			if got := closureItems(dst, opening); !reflect.DeepEqual(got, closureItems(src, opening)) {
				t.Fatal("public reconstruction differs", got)
			}
			dst.call("", "not_found", "artifact", "show", tail, "--json")
			if dst.git("for-each-ref", "--format=%(refname)", "refs/hn/artifacts/") != "" {
				t.Fatal("import installed local signing root")
			}
			for _, p := range []string{dst.home, filepath.Join(dst.repo, ".hn"), filepath.Join(dst.repo, ".git", "hn")} {
				if _, e := os.Stat(p); !os.IsNotExist(e) {
					t.Fatal("import created private state", p, e)
				}
			}
			before = dst.snapshot()
			dst.call("", "", "artifact", "import", "--input", path, "--json")
			dst.call(string(raw), "", "artifact", "import", "--input", "-", "--json")
			dst.unchanged(before)
		})
	}
}
func TestArtifactExchangeRefusalsCLI(t *testing.T) {
	src := artifactCLI(t)
	typ := revisionEvent(src.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(src, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "accepted")), "open", ""))
	path, _ := exchangeExport(src, opening)
	raw, _ := os.ReadFile(path)
	var wire map[string]any
	_ = json.Unmarshal(raw, &wire)
	tests := []struct {
		name, code string
		change     func(map[string]any)
	}{
		{"missing-public-blob", "verification_failure", func(v map[string]any) {
			objects := v["objects"].([]any)
			for i, o := range objects {
				if o.(map[string]any)["type"] == "blob" {
					v["objects"] = append(objects[:i:i], objects[i+1:]...)
					return
				}
			}
		}},
		{"corrupt-hash", "verification_failure", func(v map[string]any) { v["objects"].([]any)[0].(map[string]any)["data"] = "Y29ycnVwdA" }},
		{"padded-base64", "invalid_input", func(v map[string]any) { v["objects"].([]any)[0].(map[string]any)["data"] = "YQ==" }},
		{"duplicate-object", "invalid_input", func(v map[string]any) { v["objects"] = append(v["objects"].([]any), v["objects"].([]any)[0]) }},
		{"duplicate-ref", "invalid_input", func(v map[string]any) { v["refs"] = append(v["refs"].([]any), v["refs"].([]any)[0]) }},
		{"private-ref", "invalid_input", func(v map[string]any) { v["refs"].([]any)[0].(map[string]any)["ref"] = "refs/hn/private/key" }},
		{"null-objects", "invalid_input", func(v map[string]any) { v["objects"] = nil }},
		{"unknown-mode", "invalid_input", func(v map[string]any) { v["mode"] = "automatic" }},
		{"tag-object", "invalid_input", func(v map[string]any) { v["objects"].([]any)[0].(map[string]any)["type"] = "tag" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := exchangeDestination(t, src, "sha1")
			var v map[string]any
			_ = json.Unmarshal(raw, &v)
			tc.change(v)
			bad := exchangeWrite(t, v)
			before := dst.snapshot()
			dst.call("", tc.code, "artifact", "import", "--input", bad, "--json")
			dst.unchanged(before)
		})
	}
	for name, body := range map[string]string{"duplicate-key": strings.Replace(string(raw), `"protocol":`, `"protocol":"hn-artifact-exchange/1","protocol":`, 1), "trailing": string(raw) + " {}", "null-refs": `{"protocol":"hn-artifact-exchange/1","mode":"accepted","refs":null,"objects":[]}`} {
		t.Run(name, func(t *testing.T) {
			dst := exchangeDestination(t, src, "sha1")
			bad := filepath.Join(t.TempDir(), "bad")
			_ = os.WriteFile(bad, []byte(body), 0600)
			before := dst.snapshot()
			dst.call("", "invalid_input", "artifact", "import", "--input", bad, "--json")
			dst.unchanged(before)
		})
	}
	t.Run("symlink", func(t *testing.T) {
		dst := exchangeDestination(t, src, "sha1")
		link := filepath.Join(t.TempDir(), "link")
		_ = os.Symlink(path, link)
		before := dst.snapshot()
		dst.call("", "invalid_input", "artifact", "import", "--input", link, "--json")
		dst.unchanged(before)
	})
	t.Run("object-format", func(t *testing.T) {
		dst := exchangeDestination(t, src, "sha256")
		before := dst.snapshot()
		dst.call("", "invalid_input", "artifact", "import", "--input", path, "--json")
		dst.unchanged(before)
	})
}
func TestArtifactExchangeOpaqueCLI(t *testing.T) {
	src := artifactCLI(t)
	dst := exchangeDestination(t, src, "sha1")
	typ := revisionEvent(src.publish(artifactStoreType, "type", ""))
	path, _ := exchangeExport(src, typ)
	raw, _ := os.ReadFile(path)
	var bundle map[string]any
	_ = json.Unmarshal(raw, &bundle)
	// Explicit preservation stores public bodies without admitting even valid signed data.
	before := dst.snapshot()
	dst.call("", "invalid_input", "artifact", "import", "--input", path, "--preserve-opaque", "--preserve-opaque", "--json")
	dst.unchanged(before)
	acceptedPreserved := exchangeDestination(t, src, "sha1")
	preserved := acceptedPreserved.call("", "", "artifact", "import", "--input", path, "--preserve-opaque", "--json")["data"].(map[string]any)
	if preserved["mode"] != "opaque" || len(preserved["accepted_refs"].([]any)) != 0 || acceptedPreserved.git("for-each-ref", "--format=%(refname)", "refs/hn") != "" {
		t.Fatal("preservation flag admitted accepted data", preserved)
	}
	bundle["mode"] = "opaque"
	opaquePath := exchangeWrite(t, bundle)
	before = dst.snapshot()
	dst.call("", "invalid_input", "artifact", "import", "--input", opaquePath, "--json")
	dst.unchanged(before)
	result := dst.call("", "", "artifact", "import", "--input", opaquePath, "--preserve-opaque", "--json")["data"].(map[string]any)
	if len(result["accepted_refs"].([]any)) != 0 || len(result["imported_ids"].([]any)) != 0 {
		t.Fatal("opaque promoted", result)
	}
	ids := result["opaque_ids"].([]any)
	if len(ids) != len(bundle["objects"].([]any)) {
		t.Fatal(result)
	}
	if dst.git("for-each-ref", "--format=%(refname)", "refs/hn") != "" {
		t.Fatal("opaque created accepted ref")
	}
	dst.call("", "not_found", "artifact", "show", typ, "--json")
	id := ids[0].(string)
	view := dst.call("", "", "artifact", "inspect", "--opaque", "--opaque-id", id, "--json")["data"].(map[string]any)
	if view["opaque_id"] != id || view["validation"] != "opaque" {
		t.Fatal(view)
	}
	out := filepath.Join(t.TempDir(), "opaque.json")
	exported := dst.call("", "", "artifact", "export", "--opaque", "--opaque-id", id, "--output", out, "--json")["data"].(map[string]any)
	if exported["records"] != float64(0) || len(exported["refs"].([]any)) != 0 || !reflect.DeepEqual(exported["opaque_ids"], []any{id}) {
		t.Fatal(exported)
	}
	third := exchangeDestination(t, src, "sha1")
	third.call("", "", "artifact", "import", "--input", out, "--preserve-opaque", "--json")
	if !reflect.DeepEqual(view, third.call("", "", "artifact", "inspect", "--opaque", "--opaque-id", id, "--json")["data"]) {
		t.Fatal("opaque body did not survive independently")
	}
	// Opaque IDs address only the bounded index, never arbitrary reachable ODB objects.
	absent := "sha256:" + strings.Repeat("1", 64)
	before = dst.snapshot()
	dst.call("", "not_found", "artifact", "inspect", "--opaque", "--opaque-id", absent, "--json")
	dst.unchanged(before)
}
func TestArtifactExchangeSelectorRefusalsCLI(t *testing.T) {
	f := artifactCLI(t)
	id := revisionEvent(f.publish(artifactStoreType, "type", ""))
	out := filepath.Join(t.TempDir(), "out")
	for _, args := range [][]string{
		{"export", "--id", id, "--id", id, "--output", out},
		{"export", "--id", id, "--output", out, "--output", out},
		{"export", "--id", id, "--output", out, "--opaque"},
		{"export", "--opaque", "--opaque-id", id, "--id", id, "--output", out},
		{"export", "--output", out}, {"import", "--input", "-", "--id", id},
		{"import", "--input", "-", "--input", "-"}, {"inspect", "--opaque", "--id", id},
	} {
		before := f.snapshot()
		f.call("", "invalid_input", append([]string{"artifact"}, append(args, "--json")...)...)
		f.unchanged(before)
	}
}

func TestArtifactExchangeWireBoundCLI(t *testing.T) {
	src := artifactCLI(t)
	dst := exchangeDestination(t, src, "sha1")
	o := exchangeObject("blob", []byte("public equality"))
	raw, _ := json.Marshal(exchangeBundle(o))
	raw = append(raw, bytes.Repeat([]byte(" "), 16777216-len(raw))...)
	path := filepath.Join(t.TempDir(), "at-limit")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	got := dst.call("", "", "artifact", "import", "--input", path, "--preserve-opaque", "--json")["data"].(map[string]any)
	if !reflect.DeepEqual(got["opaque_ids"], []any{eventID([]byte("blob\x00public equality"))}) {
		t.Fatal(got)
	}
	over := filepath.Join(t.TempDir(), "over-limit")
	_ = os.WriteFile(over, append(raw, ' '), 0600)
	before := dst.snapshot()
	result := dst.call("", "resource_limit", "artifact", "import", "--input", over, "--preserve-opaque", "--json")
	if !strings.Contains(result["error"].(map[string]any)["message"].(string), "wire") {
		t.Fatal(result)
	}
	dst.unchanged(before)
}
func TestArtifactExchangeMissingSchemaCLI(t *testing.T) {
	a, b := artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	schemaTip := a.git("rev-parse", artifactRef(a.actor))
	revisionFetch(b, a, "schema")
	opening := revisionEvent(revisionCall(b, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "requires external exact schema")), "opening", ""))
	path, _ := exchangeExport(b, opening)
	raw, _ := os.ReadFile(path)
	var v map[string]any
	_ = json.Unmarshal(raw, &v)
	omit := map[string]bool{}
	for _, id := range strings.Fields(a.git("rev-list", "--objects", "--no-object-names", schemaTip)) {
		omit[id] = true
	}
	objects := []any{}
	for _, o := range v["objects"].([]any) {
		if !omit[o.(map[string]any)["oid"].(string)] {
			objects = append(objects, o)
		}
	}
	v["objects"] = objects
	refs := []any{}
	for _, r := range v["refs"].([]any) {
		if r.(map[string]any)["ref"] != artifactRef(a.actor) {
			refs = append(refs, r)
		}
	}
	v["refs"] = refs
	dst := exchangeDestination(t, a, "sha1")
	before := dst.snapshot()
	result := dst.call("", "incomplete_dependency", "artifact", "import", "--input", exchangeWrite(t, v), "--json")
	if !strings.Contains(result["error"].(map[string]any)["message"].(string), "schema") {
		t.Fatal(result)
	}
	dst.unchanged(before)
}
func TestArtifactExchangeShapesAndHeadsCLI(t *testing.T) {
	a, b := artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "original 世界")), "opening", ""))
	revisionFetch(b, a, "a")
	left := revisionEvent(revisionCall(a, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "left")), "left", ""))
	right := revisionEvent(revisionCall(b, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "right")), "right", ""))
	revisionFetch(a, b, "b")
	ticketInput := strings.Replace(strings.Replace(artifactStoreType, "example.test/note", "example.test/ticket", 1), `"shape":"document"`, `"shape":"ticket"`, 1)
	ticketSchema := revisionEvent(a.publish(ticketInput, "ticket-type", ""))
	p := revisionPayload(ticketSchema, "ticket exact text")
	p["shape"] = "ticket"
	p["type_id"] = "example.test/ticket"
	ticket := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, p), "ticket", ""))
	listInput := strings.Replace(strings.Replace(artifactStoreType, "example.test/note", "example.test/list", 1), `"shape":"document"`, `"shape":"list"`, 1)
	listSchema := revisionEvent(a.publish(listInput, "list-type", ""))
	p = revisionPayload(listSchema, "ordered 雪\ne\u0301")
	p["shape"] = "list"
	p["type_id"] = "example.test/list"
	p["members"] = []any{map[string]any{"member_id": "z-first", "target": opening, "target_mode": "logical", "required": true}, map[string]any{"member_id": "a-second", "target": ticket, "target_mode": "revision", "required": true}}
	p["relationships"] = []any{map[string]any{"relation": "example.test/link", "target": left, "target_mode": "revision", "required": true}, map[string]any{"relation": "example.test/optional", "target": "sha256:" + strings.Repeat("d", 64), "target_mode": "revision", "required": false}}
	list := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, p), "list", ""))
	path, _ := exchangeExport(a, list)
	dst := exchangeDestination(t, a, "sha1")
	dst.call("", "", "artifact", "import", "--input", path, "--json")
	for _, id := range []string{opening, ticket, list} {
		if !reflect.DeepEqual(revisionReadView(a, id), revisionReadView(dst, id)) {
			t.Fatal("shape/order/link/head view changed", id)
		}
	}
	heads := revisionHeads(t, revisionReadView(dst, opening))
	if len(heads) != 2 || !((heads[0] == left && heads[1] == right) || (heads[0] == right && heads[1] == left)) {
		t.Fatal(heads)
	}
	if !reflect.DeepEqual(closureItems(a, list), closureItems(dst, list)) {
		t.Fatal("exact public history changed")
	}
}

func TestArtifactExchangeCASWrongOldCLI(t *testing.T) {
	src := artifactCLI(t)
	other := artifactCLI(t)
	if other.actor > src.actor {
		src, other = other, src
	}
	typ := revisionEvent(src.publish(artifactStoreType, "type", ""))
	first, _ := exchangeExport(src, typ)
	dst := exchangeDestination(t, src, "sha1")
	dst.call("", "", "artifact", "import", "--input", first, "--json")
	ref := "refs/hn/remotes/imported/artifacts/" + src.actor + "/" + strings.TrimPrefix(artifactStream(src.actor), "sha256:")
	old := dst.git("rev-parse", ref)
	opening := revisionEvent(revisionCall(src, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "next")), "next", ""))
	revisionFetch(other, src, "source")
	otherOpening := revisionEvent(revisionCall(other, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "other actor")), "other", ""))
	revisionFetch(src, other, "other")
	path, exported := exchangeExport(src, opening, otherOpening)
	if len(exported["refs"].([]any)) != 2 {
		t.Fatal("CAS control must exercise two promoted refs", exported)
	}
	// Populate only public ODB bodies before the fault. This makes the post-refusal
	// complete filesystem comparison sensitive to transaction effects themselves.
	dst.git("fetch", "--quiet", "--no-tags", src.repo, artifactRef(src.actor)+":refs/hn/quarantine/wp10-control")
	dst.git("fetch", "--quiet", "--no-tags", other.repo, artifactRef(other.actor)+":refs/hn/quarantine/wp10-other-control")
	actualGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrap := t.TempDir()
	marker := filepath.Join(t.TempDir(), "cas-dispatched")
	script := "#!/bin/sh\ntarget=0; update=0\nfor arg do\n [ \"$arg\" = \"--git-dir=$HN_WP10_DEST\" ] && target=1\n [ \"$arg\" = update-ref ] && update=1\ndone\nif [ \"$target$update\" = 11 ]; then\n '" + actualGit + "' --git-dir=\"$HN_WP10_DEST\" update-ref -d \"$HN_WP10_REF\" \"$HN_WP10_OLD\" || exit 89\n : > \"$HN_WP10_MARKER\"\nfi\nexec '" + actualGit + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	dst.overrides = map[string]string{"PATH": wrap + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP10_DEST": filepath.Join(dst.repo, ".git"), "HN_WP10_REF": ref, "HN_WP10_OLD": old, "HN_WP10_MARKER": marker}
	before := dst.snapshot()
	result := dst.call("", "concurrent_update", "artifact", "import", "--input", path, "--json")
	artifactErrorShape(t, result, "artifact", "concurrent_update", "retry_identical", nil)
	if _, e = os.Stat(marker); e != nil {
		t.Fatal("CAS control never reached destination transaction", e)
	}
	if dst.git("for-each-ref", "--format=%(refname)", ref) != "" {
		t.Fatal("wrong-old transaction published")
	}
	otherRef := "refs/hn/remotes/imported/artifacts/" + other.actor + "/" + strings.TrimPrefix(artifactStream(other.actor), "sha256:")
	if dst.git("for-each-ref", "--format=%(refname)", otherRef) != "" {
		t.Fatal("failed CAS partially promoted another selected ref")
	}
	dst.git("update-ref", ref, old)
	// Contract§5/T042 forbids partial accepted publication. Git may prepare
	// new EMPTY ref-parent directories before refusing CAS. Root adjudicated
	// this narrower filesystem allowance; every file/object/private byte still
	// must match and no other new path is permitted.
	allowedParents := map[string]bool{}
	gitDir := filepath.Join(dst.repo, ".git")
	for _, selected := range []string{ref, otherRef} {
		for parent := filepath.Dir(filepath.Join(gitDir, filepath.FromSlash(selected))); parent != gitDir; parent = filepath.Dir(parent) {
			allowedParents[parent] = true
		}
	}
	after := dst.snapshot()
	for path := range after {
		if _, existed := before[path]; existed {
			continue
		}
		info, e := os.Lstat(path)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !allowedParents[path] {
			t.Fatal("CAS refusal created an unexpected path or data", path, e)
		}
		delete(after, path)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("CAS refusal changed files, objects, private state or pre-existing directories")
	}
}
func TestArtifactExchangeOpaqueAggregateAndLockCLI(t *testing.T) {
	src := artifactCLI(t)
	dst := exchangeDestination(t, src, "sha1")
	objects := []artifactExchangeObject{}
	for i := 0; i < 4096; i++ {
		objects = append(objects, exchangeObject("blob", []byte(fmt.Sprintf("public opaque %d", i))))
	}
	path := exchangeWrite(t, exchangeBundle(objects...))
	out := dst.call("", "", "artifact", "import", "--input", path, "--preserve-opaque", "--json")["data"].(map[string]any)
	if len(out["opaque_ids"].([]any)) != 4096 {
		t.Fatal("opaque equality truncated")
	}
	before := dst.snapshot()
	dst.call("", "", "artifact", "import", "--input", path, "--preserve-opaque", "--json")
	dst.unchanged(before)
	over := exchangeWrite(t, exchangeBundle(exchangeObject("blob", []byte("new distinct opaque public object"))))
	dst.call("", "resource_limit", "artifact", "import", "--input", over, "--preserve-opaque", "--json")
	dst.unchanged(before)
	index := filepath.Join(dst.repo, ".git", "hn", "artifact-opaque", "public.json")
	lock, e := os.OpenFile(index+".lock", os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	if e = artifactTestLock(lock); e != nil {
		t.Fatal(e)
	}
	dst.call("", "concurrent_update", "artifact", "import", "--input", over, "--preserve-opaque", "--json")
	dst.unchanged(before)
	artifactTestUnlock(lock)
	// Symlink replacement never redirects the private index reader/writer.
	body, e := os.ReadFile(index)
	if e != nil {
		t.Fatal(e)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	_ = os.WriteFile(outside, body, 0600)
	_ = os.Remove(index)
	_ = os.Symlink(outside, index)
	before = dst.snapshot()
	dst.call("", "invalid_input", "artifact", "inspect", "--opaque", "--opaque-id", out["opaque_ids"].([]any)[0].(string), "--json")
	dst.unchanged(before)
}

func TestArtifactExchangeDepthLiveCLI(t *testing.T) {
	src := artifactCLI(t)
	dst := exchangeDestination(t, src, "sha1")
	previous, parent, last := "", "", ""
	var atPath, atTip, actor string
	for sequence := 1; sequence <= 257; sequence++ {
		raw, sig := artifactSignedCLI(t, func(m map[string]any) {
			actor = artifactText(m, "actor")
			m["operation"] = fmt.Sprintf("exchange-depth-%d", sequence)
			m["sequence"], m["previous"] = sequence, previous
		})
		last = eventID([]byte(raw))
		parent = artifactRawCommit(t, src, raw, sig, parent, "hn artifact "+last, false)
		previous = last
		if sequence == 256 {
			ref := artifactRef(actor)
			src.git("update-ref", ref, parent)
			atTip = parent
			atPath, _ = exchangeExport(src, last)
			result := dst.call("", "", "artifact", "import", "--input", atPath, "--json")["data"].(map[string]any)
			if len(result["imported_ids"].([]any)) != 256 {
				t.Fatal("depth256 silently truncated", result)
			}
		}
	}
	ref := artifactRef(actor)
	src.git("update-ref", ref, parent)
	before := src.snapshot()
	output := filepath.Join(t.TempDir(), "refused")
	v := src.call("", "resource_limit", "artifact", "export", "--id", last, "--output", output, "--json")
	if !strings.Contains(v["error"].(map[string]any)["message"].(string), "depth") {
		t.Fatal(v)
	}
	src.unchanged(before)
	if _, e := os.Stat(output); !os.IsNotExist(e) {
		t.Fatal("refused export wrote output")
	}
	atRaw, _ := os.ReadFile(atPath)
	var bundle map[string]any
	_ = json.Unmarshal(atRaw, &bundle)
	bundle["refs"].([]any)[0].(map[string]any)["oid"] = parent
	// Add only the independently enumerated257th commit/tree/blobs. This bypasses
	// the refusing exporter to exercise the actual import consumer of the bad file.
	for _, oid := range strings.Fields(src.git("rev-list", "--objects", "--no-object-names", atTip+".."+parent)) {
		kind := src.git("cat-file", "-t", oid)
		c := exec.Command("git", "cat-file", kind, oid)
		c.Dir = src.repo
		c.Env = operationalTestEnvironment(src.home)
		body, e := c.Output()
		if e != nil {
			t.Fatal(e)
		}
		bundle["objects"] = append(bundle["objects"].([]any), map[string]any{"oid": oid, "type": kind, "data": base64.RawStdEncoding.EncodeToString(body)})
	}
	over := exchangeWrite(t, bundle)
	before = dst.snapshot()
	v = dst.call("", "resource_limit", "artifact", "import", "--input", over, "--json")
	if !strings.Contains(v["error"].(map[string]any)["message"].(string), "depth") {
		t.Fatal(v)
	}
	dst.unchanged(before)
}
func TestArtifactExchangeSelectedPolicyCLI(t *testing.T) {
	src := artifactCLI(t)
	typ := revisionEvent(src.publish(artifactStoreType, "type", ""))
	path, _ := exchangeExport(src, typ)
	dst := artifactCLI(t)
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{dst.actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("exchange refusal")), Moderators: []string{dst.actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	raw, _ := json.Marshal(policy)
	p := filepath.Join(dst.repo, ".hn", "policy.json")
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	dst.git("add", ".hn/policy.json")
	dst.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Stored inert policy")
	cmd := exec.Command(dst.binary, "contribution", "authority", "set", "--ref", dst.git("symbolic-ref", "HEAD"), "--expect", "absent")
	cmd.Dir = dst.repo
	cmd.Env = operationalTestEnvironment(dst.home)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	cmd = exec.Command(dst.binary, "identity", "show", "--json")
	cmd.Dir = dst.repo
	cmd.Env = operationalTestEnvironment(dst.home)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	before := dst.snapshot()
	v := dst.call("", "unsupported_capability", "artifact", "import", "--input", path, "--json")
	artifactErrorShape(t, v, "host", "unsupported_capability", "do_not_retry", nil)
	dst.unchanged(before)
	// Preservation is explicit public storage and admits no custom governed data.
	result := dst.call("", "", "artifact", "import", "--input", path, "--preserve-opaque", "--json")["data"].(map[string]any)
	if result["mode"] != "opaque" || len(result["accepted_refs"].([]any)) != 0 {
		t.Fatal(result)
	}
	if dst.git("for-each-ref", "--format=%(refname)", "refs/hn/remotes/imported/artifacts/") != "" {
		t.Fatal("preservation bypassed custom policy")
	}
}

func exchangeGitOnlyBundle(t *testing.T, f *artifactCLIFixture, refs []artifactSource) *artifactExchange {
	t.Helper()
	seen := map[string]bool{}
	objects := []artifactExchangeObject{}
	for _, ref := range refs {
		for _, oid := range strings.Fields(f.git("rev-list", "--objects", "--no-object-names", ref.OID)) {
			if seen[oid] {
				continue
			}
			seen[oid] = true
			kind := f.git("cat-file", "-t", oid)
			cmd := exec.Command("git", "cat-file", kind, oid)
			cmd.Dir = f.repo
			cmd.Env = operationalTestEnvironment(f.home)
			body, e := cmd.Output()
			if e != nil {
				t.Fatal(e)
			}
			objects = append(objects, artifactExchangeObject{OID: oid, Type: kind, Data: base64.RawStdEncoding.EncodeToString(body), body: body})
		}
	}
	return &artifactExchange{Protocol: "hn-artifact-exchange/1", Mode: "accepted", Refs: refs, Objects: objects}
}
func TestArtifactExchangeMalformedSignaturePreservationCLI(t *testing.T) {
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
	result := dst.call("", "", "artifact", "import", "--input", path, "--preserve-opaque", "--json")["data"].(map[string]any)
	if result["mode"] != "opaque" || len(result["accepted_refs"].([]any)) != 0 {
		t.Fatal("malformed signature was admitted", result)
	}
	for _, raw := range bundle.Objects {
		id := eventID(append([]byte(raw.Type+"\x00"), raw.body...))
		v := dst.call("", "", "artifact", "inspect", "--opaque", "--opaque-id", id, "--json")["data"].(map[string]any)
		if v["id"] != nil || v["envelope"] != nil || v["signature"] != nil || v["raw"].(map[string]any)["data"] != raw.Data {
			t.Fatal("opaque inspection invented signature validity", v)
		}
	}
	src.call("", "", "identity", "show", "--json")
}

func TestArtifactExchangeLostAcknowledgementCLI(t *testing.T) {
	src := artifactCLI(t)
	typ := revisionEvent(src.publish(artifactStoreType, "type", ""))
	path, _ := exchangeExport(src, typ)
	dst := exchangeDestination(t, src, "sha1")
	ref := "refs/hn/remotes/imported/artifacts/" + src.actor + "/" + strings.TrimPrefix(artifactStream(src.actor), "sha256:")
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrap := t.TempDir()
	marker := filepath.Join(t.TempDir(), "committed-before-error")
	script := "#!/bin/sh\ntarget=0;update=0\nfor arg do\n [ \"$arg\" = \"--git-dir=$HN_WP10_DEST\" ] && target=1\n [ \"$arg\" = update-ref ] && update=1\ndone\nif [ \"$target$update\" = 11 ];then\n '" + git + "' \"$@\" || exit $?\n : > \"$HN_WP10_MARKER\"\n exit 86\nfi\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	overrides := map[string]string{"PATH": wrap + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP10_DEST": filepath.Join(dst.repo, ".git"), "HN_WP10_MARKER": marker}
	before := dst.snapshot()
	cmd := exec.Command(dst.binary, "artifact", "import", "--input", path, "--json")
	cmd.Dir = dst.repo
	cmd.Env = operationalTestEnvironmentWith(dst.home, overrides)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, callErr := cmd.Output()
	var result map[string]any
	if e = json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e, string(raw))
	}
	if _, e = os.Stat(marker); e != nil {
		t.Fatal("control did not complete actual transaction", e)
	}
	tip := dst.git("rev-parse", ref)
	if tip != src.git("rev-parse", artifactRef(src.actor)) {
		t.Fatal("lost-ack fixture lacks committed public ref")
	}
	if evidence := os.Getenv("HN_WP10_EVIDENCE_DIR"); evidence != "" {
		b, _ := json.MarshalIndent(map[string]any{"before": before, "after": dst.snapshot(), "response": result, "stderr": stderr.String(), "published_ref": ref, "published_oid": tip}, "", "  ")
		if e = os.WriteFile(filepath.Join(evidence, "lost-ack-fixture.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if callErr == nil || result["ok"] != false || result["error"].(map[string]any)["code"] != "uncertain_operation" {
		t.Fatalf("committed Git exit error must be uncertain_operation: %v %s", callErr, raw)
	}
	artifactErrorShape(t, result, "artifact", "uncertain_operation", "inspect_operation", nil)
	if _, e = os.Stat(dst.home); !os.IsNotExist(e) {
		t.Fatal("transport installed signing identity", e)
	}
}

func TestArtifactExchangeFrozenTransportBodiesCLI(t *testing.T) {
	fixturePath := "kitty-specs/optional-extensions-01M661C0/contracts/conformance-fixtures.md"
	document, e := os.ReadFile(fixturePath)
	if os.IsNotExist(e) {
		document, e = exec.Command("git", "show", "feature/optional-extensions:"+fixturePath).Output()
	}
	if e != nil {
		t.Fatal(e)
	}
	vectors := map[string]map[string]any{}
	for _, block := range regexp.MustCompile("(?s)~~~json\\n(.*?)\\n~~~").FindAllSubmatch(document, -1) {
		var meta map[string]any
		if json.Unmarshal(block[1], &meta) != nil || meta["canonical_utf8_hex"] == nil {
			continue
		}
		raw, e := hex.DecodeString(meta["canonical_utf8_hex"].(string))
		if e != nil {
			t.Fatal(e)
		}
		var env map[string]any
		if e = json.Unmarshal(raw, &env); e != nil {
			t.Fatal(e)
		}
		name, _ := env["operation"].(string)
		if name == "type-note-v1" || name == "optional-missing-target" {
			vectors[name] = meta
		}
	}
	if len(vectors) != 2 {
		t.Fatal("required frozen vectors missing")
	}
	src := artifactCLI(t)
	expected := map[string]string{"type-note-v1": "sha256:c37a9d97a7347301bf621cadc26c7b2b709556be792f19bc058e493569757ee3", "optional-missing-target": "sha256:3b8ebecc72ddc736f0a0903a1e9ccbdde5de8ab23a36a3ebdf3f169e7765a510"}
	for _, name := range []string{"type-note-v1", "optional-missing-target"} {
		meta := vectors[name]
		if meta["id"] != expected[name] {
			t.Fatal("frozen identity drift", name)
		}
		raw, e := hex.DecodeString(meta["canonical_utf8_hex"].(string))
		if e != nil {
			t.Fatal(e)
		}
		if eventID(raw) != expected[name] {
			t.Fatal("independent signed digest mismatch")
		}
		git := meta["git_sha1"].(map[string]any)
		payload := src.gitInput(string(raw), "hash-object", "-w", "--stdin")
		signature := src.gitInput(meta["signature"].(string), "hash-object", "-w", "--stdin")
		if payload != git["payload_blob"] || signature != git["signature_blob"] {
			t.Fatal("literal blob identity mismatch")
		}
		tree := src.gitInput("100644 blob "+payload+"\tartifact.json\n100644 blob "+signature+"\tsignature\n", "mktree")
		if tree != git["tree"] {
			t.Fatal("literal tree mismatch")
		}
		body := git["commit_body"].(string)
		if strings.Contains(body, "<") {
			t.Fatal("control lost the exact frozen unsigned metadata")
		}
		commit := src.gitInput(body, "hash-object", "--literally", "-w", "-t", "commit", "--stdin")
		if commit != git["commit"] {
			t.Fatal("literal commit bytes rewritten")
		}
		src.git("update-ref", meta["local_ref"].(string), commit)
	}
	root := expected["optional-missing-target"]
	before := src.snapshot()
	items := closureItems(src, root)
	if len(items) != 2 {
		t.Fatal("frozen known closure not accepted", items)
	}
	path, exported := exchangeExport(src, root)
	src.unchanged(before)
	if exported["records"] != float64(2) {
		t.Fatal(exported)
	}
	dst := exchangeDestination(t, src, "sha1")
	initial := dst.snapshot()
	cmd := exec.Command(dst.binary, "artifact", "import", "--input", path, "--json")
	cmd.Dir = dst.repo
	cmd.Env = operationalTestEnvironment(dst.home)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, callErr := cmd.Output()
	var result map[string]any
	if e = json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e, string(raw))
	}
	if evidence := os.Getenv("HN_WP10_EVIDENCE_DIR"); evidence != "" {
		wire, _ := os.ReadFile(path)
		if e = os.WriteFile(filepath.Join(evidence, "frozen-public.json"), wire, 0600); e != nil {
			t.Fatal(e)
		}
		observation, _ := json.MarshalIndent(map[string]any{"fixture_digest": eventID(document), "before": initial, "after": dst.snapshot(), "response": result, "stderr": stderr.String(), "accepted_source_items": items, "export_result": exported}, "", "  ")
		if e = os.WriteFile(filepath.Join(evidence, "frozen-observation.json"), observation, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if callErr != nil || result["ok"] != true {
		t.Fatalf("reader-accepted literal raw bodies must import unchanged: %v %s", callErr, raw)
	}
	if !reflect.DeepEqual(items, closureItems(dst, root)) {
		t.Fatal("frozen public content/diagnostics changed")
	}
	for _, meta := range vectors {
		git := meta["git_sha1"].(map[string]any)
		cmd := exec.Command("git", "cat-file", "commit", git["commit"].(string))
		cmd.Dir = dst.repo
		cmd.Env = operationalTestEnvironment(dst.home)
		body, e := cmd.Output()
		if e != nil || string(body) != git["commit_body"].(string) {
			t.Fatal("import rewrote literal unsigned Git transport bytes", e)
		}
	}
	// Correct raw transport copying must still refuse a hash mismatch before effects.
	wire, _ := os.ReadFile(path)
	var damaged map[string]any
	_ = json.Unmarshal(wire, &damaged)
	damaged["objects"].([]any)[0].(map[string]any)["data"] = ""
	badDst := exchangeDestination(t, src, "sha1")
	snapshot := badDst.snapshot()
	badDst.call("", "verification_failure", "artifact", "import", "--input", exchangeWrite(t, damaged), "--json")
	badDst.unchanged(snapshot)
}

func TestArtifactExchangeRefAliasesBeyondObjectCountCLI(t *testing.T) {
	src := artifactCLI(t)
	commit := artifactRawCommit(t, src, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	ref := artifactFrozenRef(t, src)
	src.git("update-ref", ref, commit)
	b := exchangeGitOnlyBundle(t, src, []artifactSource{{ref, commit}})
	if len(b.Objects) != 4 {
		t.Fatal("ref alias control must have exactly four public objects")
	}
	actor, _ := parseArtifactRef(ref)
	b.Refs = []artifactSource{}
	for i := 0; i < 4097; i++ {
		b.Refs = append(b.Refs, artifactSource{"refs/hn/remotes/alias-" + fmt.Sprintf("%04d", i) + "/artifacts/" + actor + "/" + strings.TrimPrefix(artifactStream(actor), "sha256:"), commit})
	}
	path := exchangeWrite(t, b)
	info, e := os.Stat(path)
	if e != nil || info.Size() >= 16777216 {
		t.Fatal("alias fixture exceeds wire bound", e)
	}
	dst := exchangeDestination(t, src, "sha1")
	before := dst.snapshot()
	sourceBefore := src.snapshot()
	cmd := exec.Command(dst.binary, "artifact", "import", "--input", path, "--json")
	cmd.Dir = dst.repo
	cmd.Env = operationalTestEnvironment(dst.home)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, callErr := cmd.Output()
	var response map[string]any
	if e = json.Unmarshal(raw, &response); e != nil {
		t.Fatal(e, string(raw))
	}
	src.unchanged(sourceBefore)
	if evidence := os.Getenv("HN_WP10_EVIDENCE_DIR"); evidence != "" {
		wire, _ := os.ReadFile(path)
		_ = os.WriteFile(filepath.Join(evidence, "aliases-public.json"), wire, 0600)
		v, _ := json.MarshalIndent(map[string]any{"before": before, "after": dst.snapshot(), "response": response, "stderr": stderr.String(), "objects": len(b.Objects), "refs": len(b.Refs), "wire_bytes": info.Size()}, "", "  ")
		if e = os.WriteFile(filepath.Join(evidence, "aliases-observation.json"), v, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if callErr != nil || response["ok"] != true {
		dst.unchanged(before)
		t.Fatalf("4097distinct ref witnesses with4objects must not hit the object bound: %v %s", callErr, raw)
	}
	result := response["data"].(map[string]any)
	expectedRef := "refs/hn/remotes/imported/artifacts/" + actor + "/" + strings.TrimPrefix(artifactStream(actor), "sha256:")
	if !reflect.DeepEqual(result["imported_ids"], []any{artifactVectorID}) || !reflect.DeepEqual(result["accepted_refs"], []any{map[string]any{"ref": expectedRef, "oid": commit}}) {
		t.Fatal("distinct source witnesses did not deduplicate exact actor prefix", result)
	}
	inspection := dst.call("", "", "artifact", "inspect", "--id", artifactVectorID, "--json")["data"].(map[string]any)
	if inspection["source_cut"] != result["source_cut"] || result["source_cut"] == "" {
		t.Fatal("import did not return actual destination Cut", result, inspection)
	}
	if !reflect.DeepEqual(closureItems(src, artifactVectorID), closureItems(dst, artifactVectorID)) {
		t.Fatal("aliased public history changed")
	}
}

func TestArtifactExchangeUnreachableObjectsCLI(t *testing.T) {
	src := artifactCLI(t)
	typ := revisionEvent(src.publish(artifactStoreType, "type", ""))
	path, _ := exchangeExport(src, typ)
	wire, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var bundle map[string]any
	if e = json.Unmarshal(wire, &bundle); e != nil {
		t.Fatal(e)
	}
	body := []byte("unselected public blob must not enter destination ODB or opaque index")
	oid := src.gitInput(string(body), "hash-object", "--stdin")
	extra := map[string]any{"oid": oid, "type": "blob", "data": base64.RawStdEncoding.EncodeToString(body)}
	bundle["objects"] = append(bundle["objects"].([]any), extra)
	input := exchangeWrite(t, bundle)
	dst := exchangeDestination(t, src, "sha1")
	before := dst.snapshot()
	cmd := exec.Command(dst.binary, "artifact", "import", "--input", input, "--json")
	cmd.Dir = dst.repo
	cmd.Env = operationalTestEnvironment(dst.home)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, callErr := cmd.Output()
	var response map[string]any
	if e = json.Unmarshal(raw, &response); e != nil {
		t.Fatal(e, string(raw))
	}
	if evidence := os.Getenv("HN_WP10_EVIDENCE_DIR"); evidence != "" {
		v, _ := json.MarshalIndent(map[string]any{"before": before, "after": dst.snapshot(), "response": response, "stderr": stderr.String(), "extra_oid": oid}, "", "  ")
		if e = os.WriteFile(filepath.Join(evidence, "unreachable-observation.json"), v, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if callErr != nil || response["ok"] != true {
		dst.unchanged(before)
		t.Fatalf("hash-valid unreachable public blob is not an implicit selected root or a refusal: %v %s", callErr, raw)
	}
	data := response["data"].(map[string]any)
	if !reflect.DeepEqual(data["imported_ids"], []any{typ}) || len(data["opaque_ids"].([]any)) != 0 {
		t.Fatal("unreachable object affected selected results", data)
	}
	absent := exec.Command("git", "cat-file", "-e", oid)
	absent.Dir = dst.repo
	absent.Env = operationalTestEnvironmentWith(dst.home, map[string]string{"GIT_NO_LAZY_FETCH": "1"})
	if e = absent.Run(); e == nil {
		t.Fatal("unreachable table body was copied to destination ODB")
	}
	if _, e = os.Stat(filepath.Join(dst.repo, ".git", "hn", "artifact-opaque", "public.json")); !os.IsNotExist(e) {
		t.Fatal("normal accepted import implicitly preserved opaque data", e)
	}
	if !reflect.DeepEqual(closureItems(src, typ), closureItems(dst, typ)) {
		t.Fatal("selected public history changed")
	}
	// ALL table hashes are mandatory, even when the object is not selected.
	extra["data"] = base64.RawStdEncoding.EncodeToString([]byte("corrupt extra body"))
	broken := exchangeWrite(t, bundle)
	negative := exchangeDestination(t, src, "sha1")
	snapshot := negative.snapshot()
	v := negative.call("", "verification_failure", "artifact", "import", "--input", broken, "--json")
	if !strings.Contains(v["error"].(map[string]any)["message"].(string), "hash") {
		t.Fatal(v)
	}
	negative.unchanged(snapshot)
}

func TestArtifactExchangeOpaqueRawCommitsAreNotSignedRecordsCLI(t *testing.T) {
	src := artifactCLI(t)
	dst := exchangeDestination(t, src, "sha1")
	objects := []artifactExchangeObject{}
	for i := 0; i < 4096; i++ {
		body := []byte(fmt.Sprintf("unverified public raw commit %d\n", i))
		hash := sha1.Sum(append([]byte(fmt.Sprintf("commit %d\x00", len(body))), body...))
		objects = append(objects, artifactExchangeObject{OID: hex.EncodeToString(hash[:]), Type: "commit", Data: base64.RawStdEncoding.EncodeToString(body), body: body})
	}
	input := exchangeWrite(t, exchangeBundle(objects...))
	before := dst.snapshot()
	cmd := exec.Command(dst.binary, "artifact", "import", "--input", input, "--preserve-opaque", "--json")
	cmd.Dir = dst.repo
	cmd.Env = operationalTestEnvironment(dst.home)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, callErr := cmd.Output()
	var response map[string]any
	if e := json.Unmarshal(raw, &response); e != nil {
		t.Fatal(e, string(raw))
	}
	if evidence := os.Getenv("HN_WP10_EVIDENCE_DIR"); evidence != "" {
		v, _ := json.MarshalIndent(map[string]any{"before": before, "after": dst.snapshot(), "response": response, "stderr": stderr.String(), "raw_objects": 4096, "verified_signed_records": 0}, "", "  ")
		if e := os.WriteFile(filepath.Join(evidence, "raw-commit-observation.json"), v, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if callErr != nil || response["ok"] != true {
		dst.unchanged(before)
		t.Fatalf("4096raw opaque commit objects are not2048verified signed records: %v %s", callErr, raw)
	}
	data := response["data"].(map[string]any)
	if data["mode"] != "opaque" || len(data["opaque_ids"].([]any)) != 4096 || len(data["imported_ids"].([]any)) != 0 || len(data["accepted_refs"].([]any)) != 0 {
		t.Fatal("raw opaque objects became accepted signed records", data)
	}
	id := artifactOpaqueID(objects[4095])
	view := dst.call("", "", "artifact", "inspect", "--opaque", "--opaque-id", id, "--json")["data"].(map[string]any)
	if view["id"] != nil || view["validation"] != "opaque" || view["raw"].(map[string]any)["data"] != objects[4095].Data {
		t.Fatal(view)
	}
	output := filepath.Join(t.TempDir(), "opaque-public.json")
	exported := dst.call("", "", "artifact", "export", "--opaque", "--opaque-id", id, "--output", output, "--json")["data"].(map[string]any)
	if exported["records"] != float64(0) || exported["objects"] != float64(1) {
		t.Fatal("opaque export claimed signed records", exported)
	}
	// Aggregate object/hash guards remain mandatory regardless of record validity.
	before = dst.snapshot()
	over := exchangeWrite(t, exchangeBundle(exchangeObject("blob", []byte("4097th raw object"))))
	dst.call("", "resource_limit", "artifact", "import", "--input", over, "--preserve-opaque", "--json")
	dst.unchanged(before)
	bad := exchangeBundle(objects[0])
	bad.Objects[0].Data = base64.RawStdEncoding.EncodeToString([]byte("changed body"))
	negative := exchangeDestination(t, src, "sha1")
	snapshot := negative.snapshot()
	negative.call("", "verification_failure", "artifact", "import", "--input", exchangeWrite(t, bad), "--preserve-opaque", "--json")
	negative.unchanged(snapshot)
}
