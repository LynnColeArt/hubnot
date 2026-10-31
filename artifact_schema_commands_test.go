package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hubnot/internal/artifact"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func schemaCommandBinary(t *testing.T) string {
	t.Helper()
	if supplied := os.Getenv("HN_WP04_BINARY"); supplied != "" {
		return supplied
	}
	source, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	binary := filepath.Join(t.TempDir(), "hn")
	git := exec.Command("git", "rev-parse", "--absolute-git-dir")
	git.Dir = source
	out, e := git.Output()
	if e != nil {
		t.Fatal(e)
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = source
	build.Env = append(os.Environ(), "GIT_DIR="+strings.TrimSpace(string(out)), "GIT_WORK_TREE="+source)
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("lane binary: %v %s", e, out)
	}
	return binary
}
func cliFixtureContexts(t *testing.T) map[string]map[string]any {
	t.Helper()
	b, e := os.ReadFile("kitty-specs/optional-extensions-01M661C0/contracts/conformance-fixtures.md")
	if e != nil {
		t.Fatal(e)
	}
	blocks := regexp.MustCompile("(?s)~~~json\\n(.*?)\\n~~~").FindAllSubmatch(b, -1)
	byID := map[string]map[string]any{}
	named := map[string]map[string]any{}
	for _, block := range blocks {
		var blockValue any
		if e := json.Unmarshal(block[1], &blockValue); e != nil {
			t.Fatal(e)
		}
		meta, ok := blockValue.(map[string]any)
		if !ok {
			continue
		}
		rawHex, ok := meta["canonical_utf8_hex"].(string)
		if !ok {
			continue
		}
		raw, e := hex.DecodeString(rawHex)
		if e != nil {
			t.Fatal(e)
		}
		var env map[string]any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if e := d.Decode(&env); e != nil {
			t.Fatal(e)
		}
		if env["protocol"] != "hn-artifact/1" {
			continue
		}
		s := map[string]any{"id": meta["id"], "signature": meta["signature"], "envelope": env, "validation": "verified", "diagnostics": []any{}}
		if _, exists := byID[meta["id"].(string)]; !exists {
			byID[meta["id"].(string)] = s
		}
		if _, exists := named[env["operation"].(string)]; !exists {
			named[env["operation"].(string)] = s
		}
	}
	out := map[string]map[string]any{}
	for name, s := range named {
		env := s["envelope"].(map[string]any)
		if env["kind"] == "type.publish" {
			out[name] = map[string]any{"type": s}
			continue
		}
		p := env["payload"].(map[string]any)
		parents := []any{}
		for _, id := range env["parents"].([]any) {
			parents = append(parents, byID[id.(string)])
		}
		var opening any
		if env["record"] != "" {
			opening = byID[env["record"].(string)]
		}
		out[name] = map[string]any{"artifact": s, "type": byID[p["type_revision"].(string)], "parents": parents, "opening": opening}
	}
	return out
}
func TestArtifactSchemaCommands(t *testing.T) {
	binary := schemaCommandBinary(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	if out, e := exec.Command("git", "init", "--quiet", repo).CombinedOutput(); e != nil {
		t.Fatalf("fixture Git: %v %s", e, out)
	}
	for _, p := range []string{filepath.Join(repo, ".hn", "extensions.json"), filepath.Join(repo, ".git", "hn", "extensions", "enabled.json"), filepath.Join(repo, ".git", "hn", "artifacts", "intents", "sentinel.json"), filepath.Join(home, ".config", "hubnot", "extensions", "registry.json")} {
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte("MALFORMED PRIVATE_WP04_SENTINEL"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	snapshot := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() {
				b, e := os.ReadFile(p)
				if e != nil {
					return e
				}
				sum := sha256.Sum256(b)
				out[p] = hex.EncodeToString(sum[:])
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	before := snapshot()
	call := func(group, input, code string, extra ...string) map[string]any {
		t.Helper()
		args := []string{"artifact", group, "validate", "--input", "-", "--json"}
		args = append(args, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Dir = repo
		cmd.Env = operationalTestEnvironment(home)
		cmd.Stdin = strings.NewReader(input)
		out, e := cmd.Output()
		var result map[string]any
		if je := json.Unmarshal(out, &result); je != nil {
			t.Fatalf("JSON output: %v %s", je, out)
		}
		if code == "" {
			if e != nil || result["ok"] != true || result["error"] != nil {
				t.Fatalf("positive: %v %s", e, out)
			}
		} else {
			if e == nil || result["ok"] != false || result["data"] != nil {
				t.Fatalf("refusal: %v %s", e, out)
			}
			p, ok := result["error"].(map[string]any)
			if !ok || p["code"] != code || p["origin"] != "artifact" || p["recovery"] != "do_not_retry" || p["operation"] != nil {
				t.Fatalf("wrong exact cause: %s", out)
			}
		}
		if bytes.Contains(out, []byte("PRIVATE_WP04_SENTINEL")) {
			t.Fatal("private marker leaked")
		}
		return result
	}
	fixtures := cliFixtureContexts(t)
	encode := func(v any) string {
		t.Helper()
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	for _, name := range []string{"type-note-v1", "type-list-v1", "type-asset-v1", "type-asset-v2"} {
		typ := fixtures[name]["type"].(map[string]any)["envelope"].(map[string]any)["payload"]
		r := call("type", encode(typ), "")["data"].(map[string]any)
		if len(r) != 6 || r["integrity"] != "not_checked" || r["schema_validation"] != "verified" || r["type_lineage"] != "not_checked" {
			t.Fatal(r)
		}
	}
	typ := fixtures["type-note-v1"]["type"].(map[string]any)["envelope"].(map[string]any)["payload"].(map[string]any)
	schema := encode(typ["schema"])
	r := call("schema", schema, "")["data"].(map[string]any)
	if !reflect.DeepEqual(r, map[string]any{"language": "hn.schema/1", "schema_validation": "verified", "integrity": "not_applicable"}) {
		t.Fatal(r)
	}
	for _, name := range []string{"asset-open", "asset-revise-a", "asset-convert-v2", "asset-resolve-mixed", "note-open", "list-open", "note-link-back", "optional-missing-target"} {
		m, ok := fixtures[name]
		if !ok {
			t.Fatalf("missing public fixture %s", name)
		}
		r := call("record", encode(m), "")["data"].(map[string]any)
		if len(r) != 9 || r["id"] != m["artifact"].(map[string]any)["id"] || r["integrity"] != "verified" || r["schema_validation"] != "verified" || r["record_rules"] != "verified_from_supplied_context" || r["actor_history"] != "not_checked" || r["type_lineage"] != "not_checked" || r["current_heads"] != "not_checked" || r["dependency_closure"] != "not_checked" {
			t.Fatal(r)
		}
	}

	pretty, e := json.MarshalIndent(fixtures["note-open"], "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	call("record", string(pretty), "")
	// Each embedded signed envelope retains the existing canonical 1MiB cap,
	// independently of the larger composite context cap.
	for _, n := range []int{1048576, 1048577} {
		makeRecord := func(padding int) (string, string) {
			return artifactSignedCLI(t, func(m map[string]any) {
				old := fixtures["note-open"]["artifact"].(map[string]any)["envelope"].(map[string]any)
				m["kind"] = "record.open"
				p := map[string]any{}
				for k, v := range old["payload"].(map[string]any) {
					p[k] = v
				}
				p["content"] = strings.Repeat("x", padding)
				m["payload"] = p
			})
		}
		empty, _ := makeRecord(0)
		raw, sig := makeRecord(n - len(empty))
		sum := sha256.Sum256([]byte(raw))
		candidate := map[string]any{"id": "sha256:" + hex.EncodeToString(sum[:]), "envelope": json.RawMessage(raw), "signature": sig, "validation": "verified", "diagnostics": []any{}}
		input := map[string]any{"artifact": candidate, "type": fixtures["note-open"]["type"], "parents": []any{}, "opening": nil}
		code := ""
		if n > 1048576 {
			code = "resource_limit"
		}
		call("record", encode(input), code)
	}
	// A charter-like consumer model uses exactly the same generic mechanism.
	charterSchema := json.RawMessage(`{"language":"hn.schema/1","root":{"type":"object","properties":{"directive":{"type":"string","maxBytes":128},"selected":{"type":"boolean"}},"required":["directive","selected"],"additionalProperties":false}}`)
	var charterSchemaValue map[string]any
	if e := json.Unmarshal(charterSchema, &charterSchemaValue); e != nil {
		t.Fatal(e)
	}
	charterType := map[string]any{"type_id": "example.charter/directive", "schema_version": 1, "shape": "document", "schema": charterSchemaValue}
	call("type", encode(charterType), "")
	signed := func(raw, sig string) map[string]any {
		sum := sha256.Sum256([]byte(raw))
		return map[string]any{"id": "sha256:" + hex.EncodeToString(sum[:]), "envelope": json.RawMessage(raw), "signature": sig, "validation": "verified", "diagnostics": []any{}}
	}
	typeBytes, typeSig := artifactSignedCLI(t, func(m map[string]any) { m["payload"] = charterType })
	typeSigned := signed(typeBytes, typeSig)
	recordBytes, recordSig := artifactSignedCLI(t, func(m map[string]any) {
		m["kind"] = "record.open"
		m["payload"] = map[string]any{"shape": "document", "type_id": "example.charter/directive", "type_revision": typeSigned["id"], "title": "Selected directive", "content": "Line one\nSélection é 😀\n", "fields": map[string]any{"directive": "Keep exact facts", "selected": true}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
	})
	charter := map[string]any{"artifact": signed(recordBytes, recordSig), "type": typeSigned, "parents": []any{}, "opening": nil}
	call("record", encode(charter), "")
	badRecord, badSig := artifactSignedCLI(t, func(m map[string]any) {
		var env map[string]any
		if e := json.Unmarshal([]byte(recordBytes), &env); e != nil {
			t.Fatal(e)
		}
		m["kind"] = "record.open"
		p := env["payload"].(map[string]any)
		p["fields"].(map[string]any)["selected"] = "true"
		m["payload"] = p
	})
	charter["artifact"] = signed(badRecord, badSig)
	call("record", encode(charter), "invalid_input")
	call("schema", strings.Replace(schema, "hn.schema/1", "hn.schema/2", 1), "unsupported_schema")
	for _, raw := range []string{"null", `{"language":"hn.schema/1","root":null}`, strings.Replace(schema, `"language":`, `"unknown":false,"language":`, 1), strings.Replace(schema, `"language":`, `"language":"hn.schema/1","language":`, 1), schema + " {}", strings.Replace(schema, `"required":["code"]`, `"required":[null]`, 1), strings.Replace(schema, `"maxBytes":64`, `"maxBytes":null`, 1), strings.Replace(schema, `"type":"string"`, `"type":null`, 1)} {
		call("schema", raw, "invalid_input")
	}
	for _, extra := range [][]string{{"--input", "-"}, {"--json"}, {"--unknown"}, {"position"}, {"--input="}, {"--json=false", "--json=true"}} {
		call("schema", schema, "invalid_input", extra...)
	}
	m := fixtures["asset-revise-a"]
	m["opening"] = nil
	call("record", encode(m), "incomplete_dependency")
	m = cliFixtureContexts(t)["asset-open"]
	m["artifact"].(map[string]any)["id"] = "sha256:" + strings.Repeat("0", 64)
	call("record", encode(m), "verification_failure")
	for _, group := range []string{"type", "schema", "record"} {
		limit := 65536
		if group == "type" {
			limit = 131072
		}
		if group == "record" {
			limit = 16777216
		}
		call(group, strings.Repeat(" ", limit+1), "resource_limit")
	}
	// The raw type cap is reachable with permitted surrounding whitespace; the
	// largest canonical type is constrained compositionally by the schema cap.
	typeRaw := encode(typ)
	call("type", typeRaw+strings.Repeat(" ", 131072-len(typeRaw)), "")

	recordRaw := encode(fixtures["note-open"])
	call("record", recordRaw+strings.Repeat(" ", 16777216-len(recordRaw)), "")
	for _, raw := range []string{`{"language":"hn.schema/1","root":{"type":"object","properties":{},"required":[null],"additionalProperties":false}}`, `{"language":"hn.schema/1","root":{"type":"object","properties":{},"required":[],"additionalProperties":false},"bad":"\ud800"}`, string([]byte{'"', 255, '"'})} {
		call("schema", raw, "invalid_input")
	}
	// An explicit file path is consumed as data, without reference expansion.
	inputFile := filepath.Join(t.TempDir(), "schema.json")
	if e := os.WriteFile(inputFile, []byte(schema), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(binary, "artifact", "schema", "validate", "--input", inputFile, "--json")
	cmd.Dir = repo
	cmd.Env = operationalTestEnvironment(home)
	if out, e := cmd.Output(); e != nil || !bytes.Contains(out, []byte(`"schema_validation":"verified"`)) {
		t.Fatalf("file input: %v %s", e, out)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("inert validation changed repository/private files: before=%v after=%v", before, after)
	}
}

func TestArtifactValidationInputCancellation(t *testing.T) {
	read, write, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer read.Close()
	defer write.Close()
	old := os.Stdin
	os.Stdin = read
	defer func() { os.Stdin = old }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, e = artifactValidationInput(ctx, "-", 65536)
	problem, ok := e.(*artifact.Error)
	if !ok || problem.Code != "cancelled" || time.Since(start) > time.Second {
		t.Fatalf("partial stdin cancellation: %v", e)
	}
}
