package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func closureItems(f *artifactCLIFixture, id string) []any {
	f.t.Helper()
	before := f.snapshot()
	defer f.unchanged(before)
	data := f.call("", "", "artifact", "inspect", "--id", id, "--json")["data"].(map[string]any)
	if len(data) != 3 || data["next_cursor"] != nil || data["source_cut"] == "" {
		f.t.Fatal("inspect must return the frozen complete page shape", data)
	}
	return data["items"].([]any)
}

func TestArtifactClosureCausalRefusalsCLI(t *testing.T) {
	for _, tc := range []struct{ name, code, message string }{
		{"schema", "incomplete_dependency", "exact artifact schema missing"},
		{"parent", "incomplete_dependency", "type parent missing"},
		{"unsupported-schema", "unsupported_schema", "schema"},
		{"signature", "verification_failure", "signature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := artifactCLI(t)
			raw, sig := artifactSignedCLI(t, func(m map[string]any) {
				if tc.name == "schema" {
					m["kind"] = "record.open"
					m["payload"] = revisionPayload("sha256:"+strings.Repeat("d", 64), "missing schema")
				} else if tc.name == "parent" {
					m["parents"] = []string{"sha256:" + strings.Repeat("d", 64)}
				} else if tc.name == "unsupported-schema" {
					m["payload"].(map[string]any)["schema"].(map[string]any)["language"] = "hn.schema/9"
				}
			})
			if tc.name == "signature" {
				if sig[0] == 'A' {
					sig = "B" + sig[1:]
				} else {
					sig = "A" + sig[1:]
				}
			}
			id := eventID([]byte(raw))
			var envelope map[string]any
			if e := json.Unmarshal([]byte(raw), &envelope); e != nil {
				t.Fatal(e)
			}
			actor := envelope["actor"].(string)
			commit := artifactRawCommit(t, f, raw, sig, "", "hn artifact "+id, false)
			f.git("update-ref", artifactRef(actor), commit)
			before := f.snapshot()
			v := f.call("", tc.code, "artifact", "inspect", "--id", id, "--json")
			artifactErrorShape(t, v, "artifact", tc.code, "do_not_retry", nil)
			if !strings.Contains(v["error"].(map[string]any)["message"].(string), tc.message) {
				t.Fatal("wrong causal refusal", v)
			}
			f.unchanged(before)
		})
	}
}

func TestArtifactClosureMissingGitObjectCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "intact control")), "open", ""))
	closureItems(f, opening)
	oid := f.git("rev-parse", artifactRef(f.actor)+":signature")
	path := filepath.Join(f.repo, ".git", "objects", oid[:2], oid[2:])
	if e := os.Remove(path); e != nil {
		t.Fatal(e)
	}
	before := f.snapshot()
	v := f.call("", "verification_failure", "artifact", "inspect", "--id", opening, "--json")
	if !strings.Contains(v["error"].(map[string]any)["message"].(string), "object") {
		t.Fatal("wrong missing-object cause", v)
	}
	artifactErrorShape(t, v, "artifact", "verification_failure", "do_not_retry", nil)
	f.unchanged(before)
}

func TestArtifactClosurePublicOnlyReconstructionCLI(t *testing.T) {
	f, dst := artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "portable public data")), "open", ""))
	marker := "WP08-PRIVATE-CREDENTIAL-KEY-GRANT-JOURNAL-MARKER"
	for _, relative := range []string{".hn/private-descriptor", ".git/hn/artifacts/private-journal", ".git/hn/extensions/private-grant"} {
		path := filepath.Join(f.repo, relative)
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte(marker), 0600); e != nil {
			t.Fatal(e)
		}
	}
	before := f.snapshot()
	items := closureItems(f, opening)
	raw, e := json.Marshal(items)
	if e != nil || strings.Contains(string(raw), marker) || strings.Contains(string(raw), f.home) || strings.Contains(string(raw), f.repo) {
		t.Fatal("private descriptor leaked", e)
	}
	// This independent Git-only reconstruction receives ONLY the public page.
	// Artifact IDs are preserved even though Git metadata is independently authored.
	commits := map[string]string{}
	pending := append([]any{}, items...)
	for len(pending) > 0 {
		next := []any{}
		for _, item := range pending {
			v := item.(map[string]any)
			env := v["envelope"].(map[string]any)
			previous := env["previous"].(string)
			if previous != "" && commits[previous] == "" {
				next = append(next, item)
				continue
			}
			body, e := json.Marshal(env)
			if e != nil {
				t.Fatal(e)
			}
			if eventID(body) != v["id"] {
				t.Fatal("independent public ID differs")
			}
			commit := artifactRawCommit(t, dst, string(body), v["signature"].(string), commits[previous], "hn artifact "+v["id"].(string), false)
			commits[v["id"].(string)] = commit
			actor := env["actor"].(string)
			dst.git("update-ref", "refs/hn/remotes/public/artifacts/"+actor+"/"+strings.TrimPrefix(artifactStream(actor), "sha256:"), commit)
		}
		if len(next) == len(pending) {
			t.Fatal("public selection lacks complete actor prefixes")
		}
		pending = next
	}
	if !reflect.DeepEqual(items, closureItems(dst, opening)) {
		t.Fatal("public-only clone changed signed identities/content")
	}
	if got := revisionReadView(dst, opening)["state"].(map[string]any)["content"]; got != "portable public data" {
		t.Fatal(got)
	}
	f.unchanged(before)
}

func TestArtifactClosureSelectorRefusalsCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	before := f.snapshot()
	for _, args := range [][]string{
		{}, {schema}, {"--id"}, {"--id", "sha256:abc"},
		{"--id", schema, "--id", schema}, {"--id", schema, "--json"},
		{"--id", schema, "--unknown"}, {"--opaque-id", schema},
	} {
		call := append([]string{"artifact", "inspect"}, args...)
		call = append(call, "--json")
		v := f.call("", "invalid_input", call...)
		artifactErrorShape(t, v, "artifact", "invalid_input", "do_not_retry", nil)
		f.unchanged(before)
	}
	v := f.call("", "not_found", "artifact", "inspect", "--id", "sha256:"+strings.Repeat("d", 64), "--json")
	artifactErrorShape(t, v, "artifact", "not_found", "do_not_retry", nil)
	f.unchanged(before)
}

func closureItemIDs(t *testing.T, items []any) []string {
	t.Helper()
	ids := []string{}
	for _, item := range items {
		v := item.(map[string]any)
		if len(v) != 5 || v["validation"] != "verified" || v["envelope"] == nil || v["signature"] == "" {
			t.Fatal("inspect returned a non-public or incomplete SignedArtifact", v)
		}
		for _, key := range []string{"id", "envelope", "signature", "validation", "diagnostics"} {
			if _, exists := v[key]; !exists {
				t.Fatal("missing frozen SignedArtifact field", key)
			}
		}
		env := v["envelope"].(map[string]any)
		raw := closurePublicJSON(t, env)
		digest := sha256.Sum256(raw)
		if v["id"] != "sha256:"+hex.EncodeToString(digest[:]) {
			t.Fatal("independent artifact digest mismatch", v["id"])
		}
		key, e := base64.RawStdEncoding.DecodeString(env["public_key"].(string))
		if e != nil || len(key) != ed25519.PublicKeySize {
			t.Fatal("invalid public key", e)
		}
		actor := sha256.Sum256(key)
		if env["actor"] != hex.EncodeToString(actor[:]) {
			t.Fatal("independent actor digest mismatch")
		}
		sig, e := base64.RawStdEncoding.DecodeString(v["signature"].(string))
		if e != nil || !ed25519.Verify(ed25519.PublicKey(key), append([]byte("hn-artifact/1\x00"), raw...), sig) {
			t.Fatal("independent public signature verification failed", e)
		}
		ids = append(ids, v["id"].(string))
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("closure IDs are not deterministic", ids)
	}
	return ids
}

// The fixtures use finite public JSON values. Canonical byte construction here
// is independent of the production artifact package and verifies every returned
// signed object before the independent public reconstruction consumes it.
func closurePublicJSON(t *testing.T, value any) []byte {
	t.Helper()
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(value); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

func TestArtifactClosureSelectedPrefixCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "public 雪 e\u0301")), "open", ""))
	// A later record in this actor stream must not turn an exact selection into
	// an all-repository export. Predecessors, however, are indispensable.
	later := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "unselected tail")), "later", ""))
	before := f.snapshot()
	want := []string{schema, opening}
	sort.Strings(want)
	got := closureItemIDs(t, closureItems(f, opening))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exact selected closure: want %v, got %v (tail %s)", want, got, later)
	}
	if got := closureItemIDs(t, closureItems(f, schema)); !reflect.DeepEqual(got, []string{schema}) {
		t.Fatal("type inspection borrowed later records", got)
	}
	f.unchanged(before)
}

func TestArtifactClosureLogicalConflictAndCycleCLI(t *testing.T) {
	a, b, unrelated := artifactCLI(t), artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "target")), "target", ""))
	revisionFetch(b, a, "a")
	ar := revisionEvent(revisionCall(a, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "A head")), "ar", ""))
	br := revisionEvent(revisionCall(b, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "B head")), "br", ""))
	revisionFetch(a, b, "b")
	foreign := revisionEvent(unrelated.publish(artifactStoreType, "foreign", ""))
	revisionFetch(a, unrelated, "unrelated")
	p := revisionPayload(schema, "required logical target plus optional absence")
	p["relationships"] = []any{
		map[string]any{"relation": "example.test/points", "target": opening, "target_mode": "logical", "required": true},
		map[string]any{"relation": "example.test/optional", "target": "sha256:" + strings.Repeat("d", 64), "target_mode": "revision", "required": false},
	}
	linked := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, p), "linked", ""))
	before := a.snapshot()
	items := closureItems(a, linked)
	want := []string{schema, opening, ar, br, linked}
	sort.Strings(want)
	if got := closureItemIDs(t, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("all conflict heads required, unrelated actor %s excluded: %v", foreign, got)
	}
	for _, item := range items {
		v := item.(map[string]any)
		if v["id"] != linked {
			continue
		}
		codes := []string{}
		for _, d := range v["diagnostics"].([]any) {
			codes = append(codes, d.(map[string]any)["code"].(string))
		}
		if !reflect.DeepEqual(codes, []string{"missing_optional", "target_conflict"}) {
			t.Fatal("explicit nonblocking diagnostics", v)
		}
	}
	a.unchanged(before)
	// A list and a document form a valid required logical cycle. The ordered
	// member array remains byte-identical; traversal order cannot sort members.
	listType := strings.Replace(artifactStoreType, "example.test/note", "example.test/list", 1)
	listType = strings.Replace(listType, `"shape":"document"`, `"shape":"list"`, 1)
	ls := revisionEvent(a.publish(listType, "list-type", ""))
	p = revisionPayload(ls, "ordered membership")
	p["type_id"], p["shape"] = "example.test/list", "list"
	p["members"] = []any{
		map[string]any{"member_id": "z-first", "target": linked, "target_mode": "logical", "required": true},
		map[string]any{"member_id": "a-second", "target": ar, "target_mode": "revision", "required": true},
	}
	expectedMembers := p["members"]
	list := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, p), "list", ""))
	p = revisionPayload(schema, "back-link")
	p["relationships"] = []any{map[string]any{"relation": "example.test/back", "target": list, "target_mode": "logical", "required": true}}
	linked2 := revisionEvent(revisionCall(a, "revise", revisionInput(t, linked, []string{linked}, p), "linked2", ""))
	want = append(want, ls, list, linked2)
	sort.Strings(want)
	cycleItems := closureItems(a, list)
	if got := closureItemIDs(t, cycleItems); !reflect.DeepEqual(got, want) {
		t.Fatal("cycle closure incomplete", got, want)
	}
	for _, item := range cycleItems {
		v := item.(map[string]any)
		if v["id"] == list && !reflect.DeepEqual(v["envelope"].(map[string]any)["payload"].(map[string]any)["members"], expectedMembers) {
			t.Fatal("closure changed signed collection order", v)
		}
	}
}

func TestArtifactClosureConversionDependenciesCLI(t *testing.T) {
	a, b, c := artifactCLI(t), artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "schema-v1", ""))
	opening := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "old exact schema")), "open", ""))
	revisionFetch(b, a, "a")
	v2 := strings.Replace(artifactStoreType, `"parents":[]`, `"parents":["`+schema+`"]`, 1)
	v2 = strings.Replace(v2, `"schema_version":1`, `"schema_version":2`, 1)
	schema2 := revisionEvent(b.publish(v2, "schema-v2", ""))
	revisionFetch(c, a, "a")
	revisionFetch(c, b, "b")
	p := revisionPayload(schema2, "explicit conversion")
	p["conversion_sources"] = []any{opening}
	converted := revisionEvent(revisionCall(c, "convert", revisionInput(t, opening, []string{opening}, p), "convert", ""))
	want := []string{schema, schema2, opening, converted}
	sort.Strings(want)
	if got := closureItemIDs(t, closureItems(c, converted)); !reflect.DeepEqual(got, want) {
		t.Fatal("conversion lost schema ancestry or exact source", got, want)
	}
	want = []string{schema, opening}
	sort.Strings(want)
	if got := closureItemIDs(t, closureItems(c, opening)); !reflect.DeepEqual(got, want) {
		t.Fatal("exact opening selection silently followed current successors", got)
	}
}
