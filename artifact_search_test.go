package main

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestArtifactSearchPatternBounds(t *testing.T) {
	for _, test := range []struct{ name, pattern, code string }{
		{"Unicode byte equality", strings.Repeat("😀", 1024), ""},
		{"Unicode byte one over", strings.Repeat("😀", 1024) + "a", "resource_limit"},
		{"instruction equality", strings.Repeat("a", 4094), ""},
		{"instruction one over", strings.Repeat("a", 4095), "resource_limit"},
		{"expansion", `(?:` + strings.Repeat("a", 2000) + `){3}`, "resource_limit"},
		{"invalid nested repeat", `(?:a{1000}){10}`, "invalid_input"},
		{"syntax", "[", "invalid_input"},
		{"lookaround", "(?=a)", "invalid_input"},
		{"backreference", `(a)\1`, "invalid_input"},
		{"invalid UTF8", string([]byte{255}), "invalid_input"},
		{"empty explicit regex", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := compileArtifactSearchPattern(test.pattern)
			if test.code != "" {
				revisionError(t, err, test.code)
			} else if err != nil || pattern.expression == nil || pattern.instructions < 2 || pattern.instructions > 4096 {
				t.Fatal(pattern, err)
			}
		})
	}
}

func TestArtifactSearchCheckedWholeSelectionWork(t *testing.T) {
	for _, test := range []struct {
		used                uint64
		instructions, bytes int
		want                uint64
		code                string
	}{
		{0, 1, 67108864, 67108864, ""},
		{0, 1, 67108865, 0, "resource_limit"},
		{67108863, 1, 1, 67108864, ""},
		{67108864, 1, 1, 0, "resource_limit"},
		{0, 4096, 16384, 67108864, ""},
		{0, 4096, 16385, 0, "resource_limit"},
		{0, math.MaxInt, math.MaxInt, 0, "resource_limit"},
		{math.MaxUint64, 2, 1, 0, "resource_limit"},
	} {
		got, err := artifactSearchAddWork(test.used, test.instructions, test.bytes)
		if test.code != "" {
			revisionError(t, err, test.code)
		} else if got != test.want || err != nil {
			t.Fatal(test, got, err)
		}
	}
}

func TestArtifactSearchFieldsAndIndependentAnchors(t *testing.T) {
	v := artifactRecordView{ID: "logical", TypeID: "inventory.test/part", Shape: "document", Heads: []artifactRecordHead{
		{ID: "revision", Actor: "actor", TypeRevision: "schema", Payload: map[string]any{"title": "雪", "content": "first\nsecond", "fields": map[string]any{"quantity": json.Number("7"), "nested": map[string]any{"sku": "<A>"}}}},
	}}
	fields, err := artifactSearchFields(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"logical", "inventory.test/part", "document", "revision", "actor", "schema", "雪", "first\nsecond", `{"nested":{"sku":"\u003cA\u003e"},"quantity":7}`} {
		found := false
		for _, field := range fields {
			found = found || field == want
		}
		if !found {
			t.Fatalf("missing public searchable text %q in %q", want, fields)
		}
	}
	p, err := compileArtifactSearchPattern(`(?s)雪.*first`)
	if err != nil || artifactSearchMatches(fields, "", &p) {
		t.Fatal("regex crossed independent public fields", err)
	}
	p, err = compileArtifactSearchPattern(`(?m)^second$`)
	if err != nil || !artifactSearchMatches(fields, "", &p) || !artifactSearchMatches(fields, `"quantity":7`, nil) {
		t.Fatal("multiline/canonical structured field omitted", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = readArtifactSearch(ctx, artifactSearchOptions{limit: 1, mode: "literal", text: "x"})
	revisionError(t, err, "cancelled")
}

func TestArtifactSearchPageResponseBound(t *testing.T) {
	// Measure the actual complete envelope, then hit equality and first over.
	page := artifactListPage{Items: []artifactRecordView{{State: map[string]any{"content": ""}}}, SourceCut: "cut"}
	raw, err := artifactSearchResponse(page)
	if err != nil {
		t.Fatal(err)
	}
	padding := 8388608 - len(raw)
	page.Items[0].State["content"] = strings.Repeat("a", padding)
	raw, err = artifactSearchResponse(page)
	if err != nil || len(raw) != 8388608 {
		t.Fatal(len(raw), err)
	}
	page.Items[0].State["content"] = strings.Repeat("a", padding+1)
	raw, err = artifactSearchResponse(page)
	revisionError(t, err, "resource_limit")
	if raw != nil {
		t.Fatal("oversized response was partially available")
	}
	if !reflect.DeepEqual(page.Items[0].State["content"], strings.Repeat("a", padding+1)) {
		t.Fatal("full state was truncated")
	}
}
