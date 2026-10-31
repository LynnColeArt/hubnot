package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func signedValue(t *testing.T, raw, sig, id string) map[string]any {
	t.Helper()
	var v any
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	e := d.Decode(&v)
	if e != nil {
		t.Fatal(e)
	}
	return map[string]any{"id": id, "envelope": v, "signature": sig, "validation": "verified", "diagnostics": []any{}}
}
func fixtureContexts(t *testing.T) map[string]map[string]any {
	t.Helper()
	byID := map[string]map[string]any{}
	named := map[string]map[string]any{}
	for _, f := range frozenVectors {
		if f.name == "unknown-protocol" || f.name == "actor-key-mismatch" {
			continue
		}
		s := signedValue(t, f.raw, f.signature, f.id)
		byID[f.id] = s
		named[f.name] = s
	}
	out := map[string]map[string]any{}
	for name, s := range named {
		env := s["envelope"].(map[string]any)
		if text(env, "kind") == "type.publish" {
			continue
		}
		p := env["payload"].(map[string]any)
		parents := []any{}
		for _, id := range env["parents"].([]any) {
			parents = append(parents, byID[id.(string)])
		}
		var opening any
		if text(env, "record") != "" {
			opening = byID[text(env, "record")]
		}
		out[name] = map[string]any{"artifact": s, "type": byID[text(p, "type_revision")], "parents": parents, "opening": opening}
	}
	return out
}
func contextBytes(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRecordFrozenContext(t *testing.T) {
	contexts := fixtureContexts(t)
	for _, name := range []string{"asset-open", "asset-revise-a", "asset-revise-b", "asset-revise-c", "asset-resolve-ab", "asset-convert-v2", "note-open", "list-open", "note-link-back", "asset-resolve-mixed", "optional-missing-target", "missing-required-target"} {
		t.Run(name, func(t *testing.T) {
			b := contextBytes(t, contexts[name])
			before := string(b)
			got, e := ValidateRecord(context.Background(), b)
			if e != nil {
				t.Fatal(e)
			}
			if got.ID != contexts[name]["artifact"].(map[string]any)["id"] || got.ActorHistory != "not_checked" || got.DependencyClosure != "not_checked" || got.RecordRules != "verified_from_supplied_context" {
				t.Fatalf("overclaimed/incorrect result: %#v", got)
			}
			if string(b) != before {
				t.Fatal("validation rewrote exact input")
			}
		})
	}
}
func TestRecordContextRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		mutate     func(map[string]any)
	}{
		{"missing parents", "incomplete_dependency", func(m map[string]any) { m["parents"] = []any{} }},
		{"missing opening", "incomplete_dependency", func(m map[string]any) { m["opening"] = nil }},
		{"null parents", "invalid_input", func(m map[string]any) { m["parents"] = nil }},
		{"forged label", "invalid_input", func(m map[string]any) { m["artifact"].(map[string]any)["validation"] = "unsupported" }},
		{"forged ID", "verification_failure", func(m map[string]any) { m["artifact"].(map[string]any)["id"] = "sha256:" + strings.Repeat("0", 64) }},
		{"forged signature", "verification_failure", func(m map[string]any) {
			s := m["artifact"].(map[string]any)
			sig := s["signature"].(string)
			s["signature"] = "A" + sig[1:]
		}},
		{"wrong exact type", "invalid_input", func(m map[string]any) { m["type"] = fixtureContexts(t)["note-open"]["type"] }},
		{"extra parent", "invalid_input", func(m map[string]any) { m["parents"] = append(m["parents"].([]any), m["opening"]) }},
		{"null string", "invalid_input", func(m map[string]any) { m["artifact"].(map[string]any)["signature"] = nil }},
		{"null array element", "invalid_input", func(m map[string]any) { m["artifact"].(map[string]any)["diagnostics"] = []any{nil} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureContexts(t)["asset-revise-a"]
			tc.mutate(m)
			_, e := ValidateRecord(context.Background(), contextBytes(t, m))
			requireCode(t, e, tc.code)
		})
	}
}
func newSigned(t *testing.T, mutate func(map[string]any)) map[string]any {
	t.Helper()
	b, s := signedFixture(t, mutate)
	return signedValue(t, string(b), s, digest(b))
}
func mutateCandidate(t *testing.T, m map[string]any, fn func(map[string]any)) {
	t.Helper()
	old := m["artifact"].(map[string]any)["envelope"].(map[string]any)
	m["artifact"] = newSigned(t, func(v map[string]any) {
		for _, k := range []string{"kind", "record", "parents", "payload"} {
			v[k] = old[k]
		}
		fn(v)
	})
}
func TestRecordImmediateEvolution(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		fn            func(map[string]any)
	}{
		{"revise cannot switch schema", "asset-convert-v2", func(v map[string]any) {
			v["kind"] = "record.revise"
			v["payload"].(map[string]any)["conversion_sources"] = []any{}
		}},
		{"convert must switch schema", "asset-revise-a", func(v map[string]any) {
			v["kind"] = "record.convert"
			v["payload"].(map[string]any)["conversion_sources"] = v["parents"]
		}},
		{"mixed resolve names conversions", "asset-resolve-mixed", func(v map[string]any) { v["payload"].(map[string]any)["conversion_sources"] = []any{} }},
		{"fields not patch", "asset-revise-a", func(v map[string]any) { v["payload"].(map[string]any)["fields"] = map[string]any{} }},
		{"wrong type identity", "asset-open", func(v map[string]any) { v["payload"].(map[string]any)["type_id"] = "example/other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureContexts(t)[tc.fixture]
			mutateCandidate(t, m, tc.fn)
			_, e := ValidateRecord(context.Background(), contextBytes(t, m))
			requireCode(t, e, "invalid_input")
		})
	}
	// Both an old-schema opening and a new-schema conversion validate against the
	// exact declaration supplied; publication never selects a latest schema.
	for _, name := range []string{"asset-open", "asset-convert-v2"} {
		_, e := ValidateRecord(context.Background(), contextBytes(t, fixtureContexts(t)[name]))
		if e != nil {
			t.Fatal(e)
		}
	}
}
func TestRecordContentAndCollectionBounds(t *testing.T) {
	for _, n := range []int{1024, 1025} {
		m := fixtureContexts(t)["list-open"]
		mutateCandidate(t, m, func(v map[string]any) {
			p := v["payload"].(map[string]any)
			a := []any{}
			for i := 0; i < n; i++ {
				a = append(a, map[string]any{"member_id": fmt.Sprintf("m%d", i), "target": frozenVectors[3].id, "target_mode": "logical", "required": true})
			}
			p["members"] = a
		})
		_, e := ValidateRecord(context.Background(), contextBytes(t, m))
		if n == 1024 && e != nil {
			t.Fatal(e)
		}
		if n == 1025 {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, n := range []int{256, 257} {
		m := fixtureContexts(t)["note-open"]
		mutateCandidate(t, m, func(v map[string]any) {
			p := v["payload"].(map[string]any)
			a := []any{}
			for i := 0; i < n; i++ {
				a = append(a, map[string]any{"relation": fmt.Sprintf("example/r%d", i), "target": frozenVectors[3].id, "target_mode": "revision", "required": false})
			}
			p["relationships"] = a
		})
		_, e := ValidateRecord(context.Background(), contextBytes(t, m))
		if n == 256 && e != nil {
			t.Fatal(e)
		}
		if n == 257 {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, change := range []string{"duplicate", "mode", "null member", "duplicate relationship"} {
		m := fixtureContexts(t)["list-open"]
		mutateCandidate(t, m, func(v map[string]any) {
			p := v["payload"].(map[string]any)
			members := p["members"].([]any)
			switch change {
			case "duplicate":
				members[1].(map[string]any)["member_id"] = members[0].(map[string]any)["member_id"]
			case "mode":
				members[0].(map[string]any)["target_mode"] = "both"
			case "null member":
				members[0] = nil
			case "duplicate relationship":
				r := map[string]any{"relation": "example/link", "target": frozenVectors[3].id, "target_mode": "logical", "required": false}
				p["relationships"] = []any{r, r}
			}
		})
		_, e := ValidateRecord(context.Background(), contextBytes(t, m))
		requireCode(t, e, "invalid_input")
	}
	m := fixtureContexts(t)["list-open"]
	raw := contextBytes(t, m)
	members := m["artifact"].(map[string]any)["envelope"].(map[string]any)["payload"].(map[string]any)["members"].([]any)
	sort.Slice(members, func(i, j int) bool {
		return text(members[i].(map[string]any), "member_id") > text(members[j].(map[string]any), "member_id")
	})
	if reflect.DeepEqual(raw, contextBytes(t, m)) {
		t.Fatal("order was normalized")
	}
	_, e := ValidateRecord(context.Background(), contextBytes(t, m))
	requireCode(t, e, "verification_failure")
}

func TestExactTypeIdentityAcrossSigners(t *testing.T) {
	m := fixtureContexts(t)["note-open"]
	oldType := m["type"].(map[string]any)
	payload := oldType["envelope"].(map[string]any)["payload"]
	other := newSigned(t, func(v map[string]any) { v["payload"] = payload })
	if other["id"] == oldType["id"] {
		t.Fatal("different signer must produce explicit distinct exact type identity")
	}
	m["type"] = other
	mutateCandidate(t, m, func(v map[string]any) { v["payload"].(map[string]any)["type_revision"] = other["id"] })
	got, e := ValidateRecord(context.Background(), contextBytes(t, m))
	if e != nil || got.TypeRevision != other["id"] {
		t.Fatal(got, e)
	}
	// Equal descriptive names/version cannot substitute the other signed identity.
	m["type"] = oldType
	_, e = ValidateRecord(context.Background(), contextBytes(t, m))
	requireCode(t, e, "invalid_input")
}

func TestSignedCollectionReorder(t *testing.T) {
	m := fixtureContexts(t)["list-open"]
	original := m["artifact"].(map[string]any)["id"]
	mutateCandidate(t, m, func(v map[string]any) {
		members := v["payload"].(map[string]any)["members"].([]any)
		members[0], members[1] = members[1], members[0]
	})
	got, e := ValidateRecord(context.Background(), contextBytes(t, m))
	if e != nil || got.ID == original {
		t.Fatalf("signed order change must be visible: %#v %v", got, e)
	}
}

func TestSuppliedParentCountBound(t *testing.T) {
	for _, n := range []int{1024, 1025} {
		m := fixtureContexts(t)["note-open"]
		opening := m["artifact"].(map[string]any)
		pairs := make([]map[string]any, 0, n)
		for i := 0; i < n; i++ {
			parent := newSigned(t, func(v map[string]any) {
				v["kind"] = "record.revise"
				v["record"] = opening["id"]
				v["parents"] = []any{opening["id"]}
				p := map[string]any{}
				for k, value := range opening["envelope"].(map[string]any)["payload"].(map[string]any) {
					p[k] = value
				}
				p["content"] = fmt.Sprintf("parent%d", i)
				v["payload"] = p
			})
			pairs = append(pairs, parent)
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i]["id"].(string) < pairs[j]["id"].(string) })
		parents := []any{}
		ids := []any{}
		for _, p := range pairs {
			parents = append(parents, p)
			ids = append(ids, p["id"])
		}
		mutateCandidate(t, m, func(v map[string]any) { v["kind"] = "record.resolve"; v["record"] = opening["id"]; v["parents"] = ids })
		m["opening"] = opening
		m["parents"] = parents
		_, e := ValidateRecord(context.Background(), contextBytes(t, m))
		if n == 1024 && e != nil {
			t.Fatal(e)
		}
		if n == 1025 {
			requireCode(t, e, "resource_limit")
		}
	}
}
