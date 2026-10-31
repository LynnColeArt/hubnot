package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestArtifactCatalogCancellationAndBound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := &artifactHistory{ByID: map[string]*storedArtifact{"present": {}}}
	_, e := buildArtifactCatalog(ctx, h)
	revisionError(t, e, "cancelled")
	h.ByID = make(map[string]*storedArtifact, artifactReadEntries+1)
	// Count gate precedes any dereference; no fabricated verified values needed.
	for i := 0; i <= artifactReadEntries; i++ {
		h.ByID[fmt.Sprint(i)] = nil
	}
	_, e = buildArtifactCatalog(context.Background(), h)
	revisionError(t, e, "resource_limit")
}

func revisionReadView(f *artifactCLIFixture, id string) map[string]any {
	return f.call("", "", "artifact", "show", id, "--json")["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
}
func TestArtifactCatalogCollectionAndCycleCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	a := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "A雪 e\u0301")), "a", ""))
	p := revisionPayload(schema, "B")
	p["relationships"] = []any{map[string]any{"relation": "example.test/points", "target": a, "target_mode": "logical", "required": true}}
	b := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, p), "b", ""))
	p = revisionPayload(schema, "A points back")
	p["relationships"] = []any{map[string]any{"relation": "example.test/points", "target": b, "target_mode": "logical", "required": true}}
	a2 := revisionEvent(revisionCall(f, "revise", revisionInput(t, a, []string{a}, p), "a2", ""))
	listType := strings.Replace(artifactStoreType, "example.test/note", "example.test/list", 1)
	listType = strings.Replace(listType, `"shape":"document"`, `"shape":"list"`, 1)
	ls := revisionEvent(f.publish(listType, "list-type", ""))
	p = revisionPayload(ls, "Ordered 雪\ne\u0301")
	p["type_id"] = "example.test/list"
	p["shape"] = "list"
	members := []any{
		map[string]any{"member_id": "z-first", "target": a, "target_mode": "logical", "required": true},
		map[string]any{"member_id": "a-second", "target": a, "target_mode": "revision", "required": true},
		map[string]any{"member_id": "middle-third", "target": a2, "target_mode": "revision", "required": true},
	}
	p["members"] = members
	list := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, p), "list", ""))
	before := f.snapshot()
	view := revisionReadView(f, list)
	wanted, _ := json.Marshal(p)
	got, _ := json.Marshal(view["state"])
	if string(wanted) != string(got) || !reflect.DeepEqual(view["diagnostics"], []any{}) {
		t.Fatal(view)
	}
	f.unchanged(before)
	// Distinct exact/logical missing dependencies refuse before any signer or journal effects.
	for _, mode := range []string{"revision", "logical"} {
		p = revisionPayload(schema, "missing")
		p["relationships"] = []any{map[string]any{"relation": "example.test/points", "target": "sha256:" + strings.Repeat("d", 64), "target_mode": mode, "required": true}}
		v := revisionCall(f, "revise", revisionInput(t, a, []string{a2}, p), "missing-"+mode, "incomplete_dependency")
		if !strings.Contains(v["error"].(map[string]any)["message"].(string), mode) {
			t.Fatal(v)
		}
		f.unchanged(before)
	}
	p = revisionPayload(schema, "optional")
	p["relationships"] = []any{map[string]any{"relation": "example.test/points", "target": "sha256:" + strings.Repeat("d", 64), "target_mode": "revision", "required": false}}
	optional := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, p), "optional", ""))
	view = revisionReadView(f, optional)
	diagnostics := view["diagnostics"].([]any)
	if len(diagnostics) != 1 || diagnostics[0].(map[string]any)["code"] != "missing_optional" {
		t.Fatal(view)
	}
}

func TestArtifactCatalogVerifiedHistoryChecks(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "original")), "open", ""))
	other := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "other")), "other", ""))
	revised := revisionEvent(revisionCall(f, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "current")), "revise", ""))
	t.Chdir(f.repo)
	h, e := loadArtifactHistory(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	catalog, e := buildArtifactCatalog(context.Background(), h)
	if e != nil {
		t.Fatal(e)
	}
	original, e := catalog.view(context.Background(), opening)
	if e != nil {
		t.Fatal(e)
	}
	// Reinsert the independently verified immutable artifacts in reverse ID order.
	ids := []string{}
	for id := range h.ByID {
		ids = append(ids, id)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	permuted := *h
	permuted.ByID = map[string]*storedArtifact{}
	for _, id := range ids {
		permuted.ByID[id] = h.ByID[id]
	}
	again, e := buildArtifactCatalog(context.Background(), &permuted)
	if e != nil {
		t.Fatal(e)
	}
	observed, e := again.view(context.Background(), opening)
	if e != nil || !reflect.DeepEqual(original, observed) {
		t.Fatal(observed, e)
	}
	delete(permuted.ByID, opening)
	_, e = buildArtifactCatalog(context.Background(), &permuted)
	revisionError(t, e, "incomplete_dependency")
	permuted.ByID[opening] = h.ByID[opening]
	s := *h.ByID[revised]
	env := map[string]any{}
	for k, v := range s.Envelope {
		env[k] = v
	}
	env["record"] = other
	s.Envelope = env
	permuted.ByID[revised] = &s
	_, e = buildArtifactCatalog(context.Background(), &permuted)
	revisionError(t, e, "invalid_history")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = catalog.view(ctx, opening)
	revisionError(t, e, "cancelled")
}
