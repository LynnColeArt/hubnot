package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const artifactVector = `{"actor":"81ebe0bf3a2be9358144a0cabbea22d5b79c008514c61207556a6b5d044d07c0","kind":"type.publish","operation":"type-note-v1","parents":[],"payload":{"schema":{"language":"hn.schema/1","root":{"additionalProperties":false,"properties":{"code":{"maxBytes":64,"type":"string"}},"required":["code"],"type":"object"}},"schema_version":1,"shape":"document","type_id":"example.inventory/note"},"previous":"","protocol":"hn-artifact/1","public_key":"yimwBSxhzDCWe9uCqanwoHsavi3uPQpYiOwM8wMqcf8","record":"","request":"sha256:887e78418d32957fbbbb77ff080a3d035a500bec823f56968e959cc492dab773","sequence":1,"stream":"sha256:2c33e48cae82e7e2b1b6010528340c2473c2d2f55800bcc7223f584a5cf21dbb","timestamp":"2026-10-30T00:00:00Z"}`
const artifactVectorSignature = "ufmWhDOKJg58F7xjCVWERo8JSeVktgBB9HY6aD26X6NUfq82KL4Jd15rd1hu6jiIIauXIkUIVyUZ2OfcC+NTBw"
const artifactVectorID = "sha256:c37a9d97a7347301bf621cadc26c7b2b709556be792f19bc058e493569757ee3"

// wp25FixtureConfig selects the same OS config directory as the child CLI.
// Every caller also pins that path in its own child environment; t.Setenv alone
// cannot isolate two fixtures created in one test.
func wp25FixtureConfig(t *testing.T, home string) string {
	t.Helper()
	return extensionTestConfigDir(t, home)
}

// The explicit management command must reject the malformed sentinel. Merely
// preserving an arbitrary file would not prove it is the selected registry.
func wp25MalformedRegistryReadControl(t *testing.T, binary, repo, home, config string) {
	t.Helper()
	cmd := exec.Command(binary, "extension", "inspect", "--json")
	cmd.Dir = repo
	cmd.Env = operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	out, err := cmd.Output()
	var result extensionCLIResult
	var envelope struct {
		Error map[string]json.RawMessage `json:"error"`
	}
	if json.Unmarshal(out, &result) != nil || json.Unmarshal(out, &envelope) != nil || err == nil || result.OK || result.Data != nil || result.Error.Origin != "host" || result.Error.Code != "invalid_input" || result.Error.Recovery != "do_not_retry" || len(envelope.Error) != 5 || string(envelope.Error["operation"]) != "null" {
		t.Fatalf("actual malformed-registry read control: %v %s", err, out)
	}
}

func TestArtifactEnvelopeCLI(t *testing.T) {
	binary := buildOperationalBinary(t)
	cmd := exec.Command(binary, "artifact", "envelope", "validate", "--input", "-", "--signature", artifactVectorSignature, "--json")
	cmd.Dir = t.TempDir()
	cmd.Stdin = strings.NewReader(artifactVector)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), artifactVectorID) {
		t.Fatalf("valid frozen envelope must verify through hn: %v\n%s", err, out)
	}
}

func TestArtifactEnvelopeCLIRefusalsAndBounds(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repository")
	home := filepath.Join(root, "home")
	config := wp25FixtureConfig(t, home)
	env := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	if e := os.MkdirAll(repo, 0700); e != nil {
		t.Fatal(e)
	}
	init := exec.Command("git", "init", "--quiet", repo)
	if out, e := init.CombinedOutput(); e != nil {
		t.Fatalf("init: %v %s", e, out)
	}
	sentinels := map[string]string{filepath.Join(repo, ".hn", "extensions.json"): "INVALID DECLARATION PRIVATE_SENTINEL", filepath.Join(config, "hubnot", "extensions", "registry.json"): "INVALID REGISTRY PRIVATE_SENTINEL", filepath.Join(home, ".config", "hubnot", "extensions", "registry.json"): "ARBITRARY OLD-PATH PRIVATE_SENTINEL", filepath.Join(repo, ".git", "hn", "extensions", "enabled.json"): "INVALID ENABLEMENT PRIVATE_SENTINEL", filepath.Join(repo, ".git", "hn", "artifacts", "intents", "private.json"): "PRIVATE_SENTINEL"}
	for p, s := range sentinels {
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	snapshot := func() map[string]string {
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
				h := sha256.Sum256(b)
				out[p] = hex.EncodeToString(h[:])
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	previousPath := filepath.Join(root, "previous.json")
	if e := os.WriteFile(previousPath, []byte(artifactVector), 0600); e != nil {
		t.Fatal(e)
	}
	wrongPreviousPath := filepath.Join(root, "wrong-previous.json")
	if e := os.WriteFile(wrongPreviousPath, []byte(artifactSecondVector), 0600); e != nil {
		t.Fatal(e)
	}
	wp25MalformedRegistryReadControl(t, binary, repo, home, config)
	before := snapshot()
	call := func(input, sig, action, code string, extra ...string) map[string]any {
		t.Helper()
		args := []string{"artifact", "envelope", action, "--input", "-", "--signature", sig, "--json"}
		args = append(args, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Dir = repo
		cmd.Env = env
		cmd.Stdin = strings.NewReader(input)
		out, e := cmd.Output()
		var result map[string]any
		if je := json.Unmarshal(out, &result); je != nil {
			t.Fatalf("public JSON: %v %s", je, out)
		}
		if code == "" {
			if e != nil || result["ok"] != true {
				t.Fatalf("positive CLI: %v %s", e, out)
			}
		} else {
			if e == nil || result["ok"] != false || result["data"] != nil {
				t.Fatalf("negative CLI: %v %s", e, out)
			}
			problem, ok := result["error"].(map[string]any)
			if !ok || problem["code"] != code || !artifactFrozenErrorCodes[code] || problem["origin"] != "artifact" || problem["recovery"] != "do_not_retry" || problem["operation"] != nil {
				t.Fatalf("wrong refusal: %s", out)
			}
		}
		if strings.Contains(string(out), "PRIVATE_SENTINEL") {
			t.Fatal("private data leaked")
		}
		return result
	}
	inspected := call(artifactVector, artifactVectorSignature, "inspect", "")["data"].(map[string]any)
	if inspected["id"] != artifactVectorID || inspected["integrity"] != "verified" || inspected["continuity"] != "genesis" || inspected["semantic_validation"] != "not_checked" || inspected["envelope"] == nil {
		t.Fatalf("inspection metadata: %#v", inspected)
	}
	second := call(artifactSecondVector, artifactSecondSignature, "validate", "", "--previous-input", previousPath, "--previous-signature", artifactVectorSignature)["data"].(map[string]any)
	if second["continuity"] != "predecessor_verified" {
		t.Fatal(second)
	}
	unchecked := call(artifactSecondVector, artifactSecondSignature, "validate", "")["data"].(map[string]any)
	if unchecked["continuity"] != "not_checked" {
		t.Fatal(unchecked)
	}
	call(artifactSecondVector, artifactSecondSignature, "validate", "invalid_history", "--previous-input", wrongPreviousPath, "--previous-signature", artifactSecondSignature)
	call(artifactSecondVector, artifactSecondSignature, "validate", "invalid_input", "--previous-input", previousPath)

	call(artifactVector, "A"+artifactVectorSignature[1:], "validate", "verification_failure")
	call(strings.Replace(artifactVector, "type-note-v1", "type-note-v2", 1), artifactVectorSignature, "validate", "verification_failure")
	call(" "+artifactVector, artifactVectorSignature, "validate", "invalid_input")
	call(strings.Replace(artifactVector, `"sequence":1`, `"sequence":1,"sequence":1`, 1), artifactVectorSignature, "validate", "invalid_input")
	call(strings.Replace(artifactVector, `"sequence":1`, `"sequence":1.0`, 1), artifactVectorSignature, "validate", "invalid_input")
	call(strings.Replace(artifactVector, `"sequence":1`, `"sequence":1,"unknown":true`, 1), artifactVectorSignature, "validate", "invalid_input")
	call(artifactVector, artifactVectorSignature+"=", "validate", "invalid_input")
	raw, sig := artifactSignedCLI(t, func(m map[string]any) { m["protocol"] = "hn-artifact/2" })
	protocolResult := call(raw, sig, "validate", "invalid_input")
	if protocolResult["error"].(map[string]any)["message"] != "unsupported artifact protocol" {
		t.Fatal(protocolResult)
	}
	raw, sig = artifactSignedCLI(t, func(m map[string]any) { m["kind"] = "record.unknown" })
	kindResult := call(raw, sig, "validate", "invalid_input")
	if kindResult["error"].(map[string]any)["message"] != "unsupported artifact kind" {
		t.Fatal(kindResult)
	}
	raw, sig = artifactSignedCLI(t, func(m map[string]any) { m["actor"] = strings.Repeat("0", 64) })
	call(raw, sig, "validate", "verification_failure")
	raw, sig = artifactSignedCLI(t, func(m map[string]any) { m["previous"] = "sha256:" + strings.Repeat("0", 64) })
	call(raw, sig, "validate", "invalid_input")
	raw, sig = artifactSignedCLI(t, func(m map[string]any) { m["sequence"] = 0 })
	call(raw, sig, "validate", "invalid_input")
	// Independently produce a signed record envelope of exactly 1 MiB.
	raw, sig = artifactSignedCLI(t, func(m map[string]any) {
		m["kind"] = "record.open"
		m["payload"] = map[string]any{"shape": "document", "type_id": "example/note", "type_revision": artifactVectorID, "title": "Exact text é é <>&", "content": "", "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
	})
	padding := 1048576 - len(raw)
	raw, sig = artifactSignedCLI(t, func(m map[string]any) {
		m["kind"] = "record.open"
		m["payload"] = map[string]any{"shape": "document", "type_id": "example/note", "type_revision": artifactVectorID, "title": "Exact text é é <>&", "content": strings.Repeat("x", padding), "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
	})
	if len(raw) != 1048576 {
		t.Fatalf("boundary fixture has %d bytes", len(raw))
	}
	call(raw, sig, "validate", "")
	call(raw+"x", sig, "validate", "resource_limit")
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("inert validation changed repository/private files: before %v after %v", before, after)
	}
}

func artifactSignedCLI(t *testing.T, mutate func(map[string]any)) (string, string) {
	t.Helper()
	m := map[string]any{}
	if e := json.Unmarshal([]byte(artifactVector), &m); e != nil {
		t.Fatal(e)
	}
	seed := sha256.Sum256([]byte("WP03 public CLI test-only actor"))
	key := ed25519.NewKeyFromSeed(seed[:])
	public := key.Public().(ed25519.PublicKey)
	actor := sha256.Sum256(public)
	m["actor"] = hex.EncodeToString(actor[:])
	m["public_key"] = base64.RawStdEncoding.EncodeToString(public)
	stream := sha256.Sum256([]byte("hn-artifact-stream-v1\x00" + m["actor"].(string)))
	m["stream"] = "sha256:" + hex.EncodeToString(stream[:])
	mutate(m)
	semantic := map[string]any{}
	for _, k := range []string{"actor", "kind", "operation", "parents", "payload", "protocol", "record"} {
		semantic[k] = m[k]
	}
	b, _ := json.Marshal(semantic)
	request := sha256.Sum256(append([]byte("hn-artifact-request-v1\x00"), b...))
	m["request"] = "sha256:" + hex.EncodeToString(request[:])
	raw, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return string(raw), base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), raw...)))
}

const artifactSecondVector = `{"actor":"81ebe0bf3a2be9358144a0cabbea22d5b79c008514c61207556a6b5d044d07c0","kind":"type.publish","operation":"type-list-v1","parents":[],"payload":{"schema":{"language":"hn.schema/1","root":{"additionalProperties":false,"properties":{},"required":[],"type":"object"}},"schema_version":1,"shape":"list","type_id":"example.inventory/list"},"previous":"sha256:c37a9d97a7347301bf621cadc26c7b2b709556be792f19bc058e493569757ee3","protocol":"hn-artifact/1","public_key":"yimwBSxhzDCWe9uCqanwoHsavi3uPQpYiOwM8wMqcf8","record":"","request":"sha256:66bdc590b7856b090240b6d013a3ce0fcad104264bd2297ea6bc67612eee1a27","sequence":2,"stream":"sha256:2c33e48cae82e7e2b1b6010528340c2473c2d2f55800bcc7223f584a5cf21dbb","timestamp":"2026-10-30T00:00:00Z"}`
const artifactSecondSignature = "cSU+0RTHgBua8SQYKZkTGi8aASODTVk4qncdm0zIrKpC8IyiSihmQaTqiEwBO8P0WHC+BkaWaLmOEk/bzVjIBg"

func TestArtifactEnvelopeCLIPublishedBounds(t *testing.T) {
	binary := buildOperationalBinary(t)
	type boundary struct {
		name   string
		at     int
		code   string
		change func(map[string]any, int)
	}
	record := func(m map[string]any) {
		m["kind"] = "record.open"
		m["payload"] = map[string]any{"shape": "document", "type_id": "example/note", "type_revision": artifactVectorID, "title": "title", "content": "", "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
	}
	cases := []boundary{
		{"operation", 128, "invalid_input", func(m map[string]any, n int) { m["operation"] = strings.Repeat("x", n) }},
		{"title", 1024, "invalid_input", func(m map[string]any, n int) {
			record(m)
			m["payload"].(map[string]any)["title"] = strings.Repeat("x", n)
		}},
		{"namespace", 64, "invalid_input", func(m map[string]any, n int) {
			m["payload"].(map[string]any)["type_id"] = strings.Repeat("x", n) + "/name"
		}},
		{"type-name", 64, "invalid_input", func(m map[string]any, n int) {
			m["payload"].(map[string]any)["type_id"] = "example/" + strings.Repeat("x", n)
		}},
		{"field-name", 64, "invalid_input", func(m map[string]any, n int) {
			record(m)
			m["payload"].(map[string]any)["fields"] = map[string]any{strings.Repeat("x", n): true}
		}},
		{"properties", 64, "resource_limit", func(m map[string]any, n int) {
			record(m)
			fields := map[string]any{}
			for i := 0; i < n; i++ {
				fields[fmt.Sprintf("p%d", i)] = i
			}
			m["payload"].(map[string]any)["fields"] = fields
		}},
		{"depth", 8, "resource_limit", func(m map[string]any, n int) {
			record(m)
			fields := map[string]any{}
			current := fields
			for i := 1; i < n; i++ {
				next := map[string]any{}
				current["nest"] = next
				current = next
			}
			m["payload"].(map[string]any)["fields"] = fields
		}},
		{"array", 1024, "resource_limit", func(m map[string]any, n int) {
			record(m)
			m["payload"].(map[string]any)["fields"] = map[string]any{"array": make([]any, n)}
		}},
		{"parents", 1024, "resource_limit", func(m map[string]any, n int) {
			record(m)
			m["kind"] = "record.resolve"
			m["record"] = artifactVectorID
			parents := []any{}
			for i := 0; i < n; i++ {
				parents = append(parents, fmt.Sprintf("sha256:%064x", i))
			}
			m["parents"] = parents
		}},
		{"members", 1024, "resource_limit", func(m map[string]any, n int) {
			record(m)
			p := m["payload"].(map[string]any)
			p["shape"] = "list"
			members := []any{}
			for i := 0; i < n; i++ {
				members = append(members, map[string]any{"member_id": fmt.Sprintf("member%d", i), "target": artifactVectorID, "target_mode": "revision", "required": false})
			}
			p["members"] = members
		}},
		{"member-id", 128, "invalid_input", func(m map[string]any, n int) {
			record(m)
			p := m["payload"].(map[string]any)
			p["shape"] = "list"
			p["members"] = []any{map[string]any{"member_id": strings.Repeat("x", n), "target": artifactVectorID, "target_mode": "logical", "required": true}}
		}},
		{"relationships", 256, "resource_limit", func(m map[string]any, n int) {
			record(m)
			links := []any{}
			for i := 0; i < n; i++ {
				links = append(links, map[string]any{"relation": fmt.Sprintf("example/r%d", i), "target": artifactVectorID, "target_mode": "revision", "required": false})
			}
			m["payload"].(map[string]any)["relationships"] = links
		}},
		{"schema-bytes", 65536, "resource_limit", func(m map[string]any, n int) {
			node := map[string]any{"type": "string", "maxBytes": 65536, "enum": []any{""}}
			schema := map[string]any{"language": "hn.schema/1", "root": map[string]any{"type": "object", "properties": map[string]any{"text": node}, "required": []any{}, "additionalProperties": false}}
			base, _ := json.Marshal(schema)
			node["enum"] = []any{strings.Repeat("x", n-len(base))}
			m["payload"].(map[string]any)["schema"] = schema
		}},
	}
	directory := t.TempDir()
	check := func(raw, sig, code string) {
		t.Helper()
		cmd := exec.Command(binary, "artifact", "envelope", "validate", "--input", "-", "--signature", sig, "--json")
		cmd.Dir = directory
		cmd.Env = operationalTestEnvironment(directory)
		cmd.Stdin = strings.NewReader(raw)
		out, e := cmd.Output()
		var result struct {
			OK    bool `json:"ok"`
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if je := json.Unmarshal(out, &result); je != nil {
			t.Fatalf("invalid JSON %v %s", je, out)
		}
		if code == "" {
			if e != nil || !result.OK {
				t.Fatalf("at bound: %v %s", e, out)
			}
		} else if e == nil || result.OK || result.Error == nil || result.Error.Code != code {
			t.Fatalf("over bound wanted %s: %v %s", code, e, out)
		}
	}
	for _, b := range cases {
		t.Run(b.name, func(t *testing.T) {
			for _, n := range []int{b.at, b.at + 1} {
				raw, sig := artifactSignedCLI(t, func(m map[string]any) { b.change(m, n) })
				code := ""
				if n > b.at {
					code = b.code
				}
				check(raw, sig, code)
			}
		})
	}
	for _, b := range []struct {
		name   string
		max    int64
		change func(map[string]any, int64)
	}{
		{"schema-version", 2147483647, func(m map[string]any, n int64) { m["payload"].(map[string]any)["schema_version"] = n }},
		{"sequence", 9007199254740991, func(m map[string]any, n int64) { m["sequence"] = n; m["previous"] = artifactVectorID }},
	} {
		t.Run(b.name, func(t *testing.T) {
			for _, n := range []int64{b.max, b.max + 1} {
				raw, sig := artifactSignedCLI(t, func(m map[string]any) { b.change(m, n) })
				code := ""
				if n > b.max {
					code = "invalid_input"
				}
				check(raw, sig, code)
			}
		})
	}
	// With the 64 KiB schema ceiling, a valid closed type cannot reach 128 KiB.
	// At equality this deliberately invalid identifier reaches the identifier
	// refusal; the extra byte instead triggers the earlier aggregate defense.
	for _, n := range []int{131072, 131073} {
		raw, sig := artifactSignedCLI(t, func(m map[string]any) {
			p := m["payload"].(map[string]any)
			p["type_id"] = ""
			base, _ := json.Marshal(p)
			p["type_id"] = strings.Repeat("x", n-len(base))
		})
		code := "invalid_input"
		if n > 131072 {
			code = "resource_limit"
		}
		check(raw, sig, code)
	}
}

// This mirror is checked against the frozen contract, and every exercised
// artifact-origin CLI refusal must belong to it. Host negotiation codes cannot
// silently enter this input-verification surface.
var artifactFrozenErrorCodes = map[string]bool{
	"invalid_input": true, "resource_limit": true, "verification_failure": true,
	"invalid_history": true, "incomplete_dependency": true, "unsupported_schema": true,
	"actor_stream_conflict": true, "operation_conflict": true, "stale_revision": true,
	"resource_conflict": true, "concurrent_update": true, "not_found": true,
	"repository_error": true, "cancelled": true, "uncertain_operation": true,
}

func TestArtifactEnvelopeErrorContractMirror(t *testing.T) {
	contract, e := os.ReadFile("kitty-specs/optional-extensions-01M661C0/contracts/host-v1.md")
	if e != nil {
		t.Fatal(e)
	}
	_, tail, ok := strings.Cut(string(contract), "Artifact-origin codes are exactly ")
	if !ok {
		t.Fatal("artifact error set absent from contract")
	}
	block, _, ok := strings.Cut(tail, ".\nDiagnostic")
	if !ok {
		t.Fatal("artifact error set terminator absent")
	}
	want := []string{}
	for _, code := range strings.Split(block, ",") {
		want = append(want, strings.TrimSpace(code))
	}
	sort.Strings(want)
	got := []string{}
	for code := range artifactFrozenErrorCodes {
		got = append(got, code)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifact error mirror differs from contract: %v vs %v", got, want)
	}
}

func TestArtifactEnvelopeCLIPredecessorStdin(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	config := wp25FixtureConfig(t, home)
	env := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	if e := os.MkdirAll(repo, 0700); e != nil {
		t.Fatal(e)
	}
	init := exec.Command("git", "init", "--quiet", repo)
	if out, e := init.CombinedOutput(); e != nil {
		t.Fatalf("git init: %v %s", e, out)
	}
	first := filepath.Join(root, "first.json")
	second := filepath.Join(root, "second.json")
	bad := filepath.Join(root, "bad.json")
	for p, data := range map[string]string{first: artifactVector, second: artifactSecondVector, bad: "{"} {
		if e := os.WriteFile(p, []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range []string{filepath.Join(repo, ".hn", "extensions.json"), filepath.Join(repo, ".git", "hn", "extensions", "enabled.json"), filepath.Join(config, "hubnot", "extensions", "registry.json"), filepath.Join(home, ".config", "hubnot", "extensions", "registry.json"), filepath.Join(repo, ".git", "hn", "artifacts", "intents", "marker.json")} {
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte("INVALID PRIVATE_STDIN_SENTINEL"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	snapshot := func() map[string]string {
		inventory := map[string]string{}
		if e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			h := sha256.Sum256(b)
			inventory[p] = hex.EncodeToString(h[:])
			return nil
		}); e != nil {
			t.Fatal(e)
		}
		return inventory
	}
	wp25MalformedRegistryReadControl(t, binary, repo, home, config)
	before := snapshot()
	call := func(action, input, previous, stdin, code, message string, blocked bool, predecessorSignatures ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		previousSignature := artifactVectorSignature
		if len(predecessorSignatures) > 0 {
			previousSignature = predecessorSignatures[0]
		}
		cmd := exec.CommandContext(ctx, binary, "artifact", "envelope", action, "--input", input, "--signature", artifactSecondSignature, "--previous-input", previous, "--previous-signature", previousSignature, "--json")
		cmd.Dir = repo
		cmd.Env = env
		if blocked {
			read, write, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer read.Close()
			defer write.Close()
			cmd.Stdin = read
		} else {
			cmd.Stdin = strings.NewReader(stdin)
		}
		out, e := cmd.Output()
		if ctx.Err() != nil {
			t.Fatal("double stdin must refuse before reading the open pipe")
		}
		var result map[string]any
		if je := json.Unmarshal(out, &result); je != nil {
			t.Fatalf("JSON: %v %s", je, out)
		}
		if code != "" {
			problem, ok := result["error"].(map[string]any)
			if e == nil || result["ok"] != false || result["data"] != nil || !ok || problem["origin"] != "artifact" || problem["code"] != code || !artifactFrozenErrorCodes[code] || problem["recovery"] != "do_not_retry" || problem["operation"] != nil || !strings.Contains(problem["message"].(string), message) {
				t.Fatalf("wrong refusal: %v %s", e, out)
			}
		} else {
			data, ok := result["data"].(map[string]any)
			if e != nil || result["ok"] != true || result["error"] != nil || !ok || data["continuity"] != "predecessor_verified" || data["semantic_validation"] != "not_checked" || data["integrity"] != "verified" {
				t.Fatalf("stdin pair: %v %s", e, out)
			}
			h := sha256.Sum256([]byte(artifactSecondVector))
			if data["id"] != "sha256:"+hex.EncodeToString(h[:]) {
				t.Fatal(data)
			}
			fields := 6
			if action == "inspect" {
				fields++
				b, _ := json.Marshal(data["envelope"])
				if string(b) != artifactSecondVector {
					t.Fatalf("inspect changed envelope: %s", b)
				}
			}
			if len(data) != fields {
				t.Fatalf("unexpected success fields: %v", data)
			}
		}
		if strings.Contains(string(out), "PRIVATE_STDIN_SENTINEL") {
			t.Fatal("private marker leaked")
		}
	}
	for _, action := range []string{"validate", "inspect"} {
		call(action, second, "-", artifactVector, "", "", false)
		call(action, "-", first, artifactSecondVector, "", "", false)
	}
	call("validate", "-", "-", "", "invalid_input", "at most one stdin", true)
	call("validate", second, "-", strings.Repeat("x", 1048577), "resource_limit", "envelope exceeds", false)
	// A bad predecessor must fail before attempting to open the missing current file.
	call("validate", filepath.Join(root, "missing-current.json"), "-", "{", "invalid_input", "malformed object", false)
	call("validate", second, "-", artifactSecondVector, "invalid_history", "predecessor mismatch", false, artifactSecondSignature)
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("stdin checks changed public/private inventory: %v %v", before, after)
	}
}
