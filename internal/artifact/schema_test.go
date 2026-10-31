package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func schemaJSON(node string) []byte { return []byte(`{"language":"hn.schema/1","root":` + node + `}`) }
func objectSchema(properties, required string) string {
	return `{"type":"object","properties":` + properties + `,"required":` + required + `,"additionalProperties":false}`
}
func TestSchemaFiniteKeywords(t *testing.T) {
	cases := []struct{ name, node, good, bad string }{
		{"string", `{"type":"string","maxBytes":4,"enum":["é","yes"]}`, `"é"`, `"no"`},
		{"UTF8 byte bound", `{"type":"string","maxBytes":2}`, `"é"`, `"€"`},
		{"integer bounds", `{"type":"integer","minimum":-2,"maximum":2}`, `-2`, `3`},
		{"integer no coercion", `{"type":"integer"}`, `0`, `"0"`},
		{"boolean no coercion", `{"type":"boolean"}`, `false`, `null`},
		{"empty array", `{"type":"array","items":{"type":"null"},"maxItems":0}`, `[]`, `[null]`},
		{"array items", `{"type":"array","items":{"type":"integer"},"maxItems":2}`, `[1]`, `["1"]`},
		{"closed object", `{"type":"object","properties":{},"required":[],"additionalProperties":false}`, `{}`, `{"unexpected":null}`},
		{"integer", `{"type":"integer","minimum":-2,"maximum":2,"enum":[-2,2]}`, `2`, `0`},
		{"boolean", `{"type":"boolean","enum":[true]}`, `true`, `false`},
		{"null", `{"type":"null"}`, `null`, `false`},
		{"array", `{"type":"array","items":{"type":"integer"},"maxItems":2}`, `[1,2]`, `[1,2,3]`},
		{"object", objectSchema(`{"nested":{"type":"boolean"}}`, `["nested"]`), `{"nested":true}`, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, e := ValidateSchema(context.Background(), schemaJSON(objectSchema(`{"v":`+tc.node+`}`, `["v"]`)))
			if e != nil {
				t.Fatal(e)
			}
			for i, raw := range []string{tc.good, tc.bad} {
				v, e := parse([]byte(`{"v":` + raw + `}`))
				if e != nil {
					t.Fatal(e)
				}
				e = s.validate(context.Background(), v, 0)
				if i == 0 && e != nil {
					t.Fatal(e)
				}
				if i == 1 {
					requireCode(t, e, "invalid_input")
				}
			}
		})
	}
}
func TestSchemaHostileDeclarations(t *testing.T) {
	nodes := []string{
		`{"type":"object","properties":{},"required":[],"additionalProperties":true}`,
		objectSchema(`{}`, `["missing"]`), objectSchema(`{"x":{"type":"integer"}}`, `["x","x"]`),
		`{"type":"array","items":{"type":"null"},"maxItems":1025}`,
		`{"type":"string","maxBytes":1,"enum":["é"]}`, `{"type":"integer","minimum":2,"maximum":1}`,
		`{"type":"boolean","enum":[true,true]}`, `{"type":"integer","enum":[]}`,
		`{"type":"string","maxBytes":3,"pattern":".*"}`, `{"type":"null","enum":[null]}`,
		`{"$ref":"https://example.invalid"}`, `{"type":"number"}`, `{"type":"string","maxBytes":null}`,
	}
	for _, node := range nodes {
		_, e := ValidateSchema(context.Background(), schemaJSON(objectSchema(`{"x":`+node+`}`, `[]`)))
		requireCode(t, e, "invalid_input")
	}
	for _, raw := range []string{`null`, `{"language":"hn.schema/2","root":{}}`, `{"language":"hn.schema/1","root":null}`, `{"language":"hn.schema/1","root":{"type":"null"}}`, `{"language":"hn.schema/1","language":"hn.schema/1","root":{}}`} {
		_, e := ValidateSchema(context.Background(), []byte(raw))
		if strings.Contains(raw, "hn.schema/2") {
			requireCode(t, e, "unsupported_schema")
		} else {
			requireCode(t, e, "invalid_input")
		}
	}
}
func TestSchemaBounds(t *testing.T) {
	for _, n := range []int{8, 9} {
		node := `{"type":"null"}`
		for i := 1; i < n; i++ {
			node = `{"type":"array","items":` + node + `,"maxItems":1}`
		}
		_, e := ValidateSchema(context.Background(), schemaJSON(objectSchema(`{"x":`+node+`}`, `[]`)))
		if n == 8 && e != nil {
			t.Fatal(e)
		}
		if n == 9 {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, n := range []int{64, 65} {
		p := []string{}
		for i := 0; i < n; i++ {
			p = append(p, fmt.Sprintf(`"p%d":{"type":"null"}`, i))
		}
		_, e := ValidateSchema(context.Background(), schemaJSON(objectSchema(`{`+strings.Join(p, ",")+`}`, `[]`)))
		if n == 64 && e != nil {
			t.Fatal(e)
		}
		if n == 65 {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, n := range []int{1024, 1025} {
		vals := []string{}
		for i := 0; i < n; i++ {
			vals = append(vals, fmt.Sprint(i))
		}
		_, e := ValidateSchema(context.Background(), schemaJSON(objectSchema(`{"x":{"type":"integer","enum":[`+strings.Join(vals, ",")+`]}}`, `[]`)))
		if n == 1024 && e != nil {
			t.Fatal(e)
		}
		if n == 1025 {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, n := range []int{65536, 65537} {
		schema := schemaJSON(objectSchema(`{"x":{"type":"string","maxBytes":1048576,"enum":[""]}}`, `[]`))
		schema = []byte(strings.Replace(string(schema), `"enum":[""]`, `"enum":["`+strings.Repeat("x", n-len(schema))+`"]`, 1))
		_, e := ValidateSchema(context.Background(), schema)
		if n == 65536 && e != nil {
			t.Fatal(e)
		}
		if n == 65537 {
			requireCode(t, e, "resource_limit")
		}
	}
}
func TestTypeDeclaration(t *testing.T) {
	for _, shape := range []string{"document", "list", "ticket"} {
		for _, name := range []string{"example.inventory/note", "hn/note", "hubnot/note", "Example/note", "example/" + strings.Repeat("a", 65)} {
			raw, _ := json.Marshal(map[string]any{"type_id": name, "shape": shape, "schema_version": 1, "schema": json.RawMessage(schemaJSON(objectSchema(`{}`, `[]`)))})
			v, e := ValidateType(context.Background(), raw)
			if name == "example.inventory/note" {
				if e != nil || v.TypeID != name || v.Shape != shape {
					t.Fatal(v, e)
				}
			} else {
				requireCode(t, e, "invalid_input")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := ValidateSchema(ctx, schemaJSON(objectSchema(`{}`, `[]`)))
	requireCode(t, e, "cancelled")
}
