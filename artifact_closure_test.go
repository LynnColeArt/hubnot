package main

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestArtifactClosureTraversalBudget(t *testing.T) {
	q := artifactClosureQueue{ids: []string{}, seen: map[string]bool{}}
	ctx := context.Background()
	for i := 0; i < 100000; i++ {
		if e := q.add(ctx, fmt.Sprintf("sha256:%064x", i)); e != nil {
			t.Fatal("read bound must accept equality", i, e)
		}
	}
	if len(q.ids) != 100000 || len(q.seen) != 100000 {
		t.Fatal("selection silently truncated")
	}
	before := append([]string{}, q.ids...)
	revisionError(t, q.add(ctx, fmt.Sprintf("sha256:%064x", 100000)), "resource_limit")
	if !reflect.DeepEqual(q.ids, before) {
		t.Fatal("over-limit input changed selected queue")
	}
	if e := q.add(ctx, before[0]); e != nil {
		t.Fatal("visited cycles should terminate without consuming budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	revisionError(t, q.add(cancelled, before[0]), "cancelled")
	if !reflect.DeepEqual(q.ids, before) {
		t.Fatal("cancelled traversal changed selection")
	}
}

func TestArtifactClosureExactGitInventory(t *testing.T) {
	f, dst := artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "selected")), "open", ""))
	tip := f.git("rev-parse", artifactRef(f.actor))
	unselected := revisionEvent(revisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "tail")), "tail", ""))
	// rev-list's public reachability is an independent exact Git object oracle.
	lines := strings.Split(f.git("rev-list", "--objects", "--no-object-names", tip), "\n")
	sort.Strings(lines)
	t.Chdir(f.repo)
	h, e := loadArtifactHistory(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	closure, e := selectArtifactClosure(context.Background(), h, []string{opening})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(closure.Objects, lines) {
		t.Fatal("closure object inventory differs from exact public Git reachability", closure.Objects, lines)
	}
	if !reflect.DeepEqual(closure.Sources, []artifactSource{{artifactRef(f.actor), tip}}) {
		t.Fatal("prefix tip is not the exact selected commit", closure.Sources)
	}
	want := []string{schema, opening}
	sort.Strings(want)
	if !reflect.DeepEqual(closure.IDs, want) || len(closure.Items) != 2 {
		t.Fatal("unselected tail entered closure", unselected, closure.IDs)
	}
	page := f.call("", "", "artifact", "inspect", "--id", opening, "--json")["data"].(map[string]any)
	if page["source_cut"] != h.Cut {
		t.Fatal("inspect invented a prefix Cut instead of retaining the actual selected-source witness", page)
	}
	if !reflect.DeepEqual(closureItemIDs(t, page["items"].([]any)), closure.IDs) {
		t.Fatal("live inspector bypassed the shared closure producer")
	}
	// Ref witnesses are the actual newer accepted refs; deriving a smaller
	// transport prefix cannot change them or pretend the prefix is local state.
	if h.Sources[0].OID == tip {
		t.Fatal("control lacks a later source witness")
	}
	if h.Sources[0].OID != f.git("rev-parse", artifactRef(f.actor)) {
		t.Fatal("selection rewound its source")
	}
	// Copy the exact public prefix OID into an independently initialized clone.
	// Fetching the OID preserves every Git body; no transport commit is rewritten.
	quarantineSources := []artifactSource{}
	for _, source := range closure.Sources {
		actor, ok := parseArtifactRef(source.Ref)
		if !ok {
			t.Fatal(source)
		}
		remoteRef := "refs/hn/remotes/selected/artifacts/" + actor + "/" + strings.TrimPrefix(artifactStream(actor), "sha256:")
		dst.git("fetch", "--no-write-fetch-head", f.repo, "+"+source.OID+":"+remoteRef)
		quarantineSources = append(quarantineSources, artifactSource{remoteRef, source.OID})
		if dst.git("rev-parse", remoteRef) != source.OID {
			t.Fatal("transport changed exact Git tip")
		}
	}
	if got := closureItemIDs(t, closureItems(dst, opening)); !reflect.DeepEqual(got, want) {
		t.Fatal("exact public Git roundtrip changed identities", got)
	}
	// Scope the independent object oracle to the selected artifact tip, excluding
	// dst's own native identity stream created by its independent initialization.
	objects := strings.Split(dst.git("rev-list", "--objects", "--no-object-names", tip), "\n")
	sort.Strings(objects)
	if !reflect.DeepEqual(objects, lines) {
		t.Fatal("Git transport changed public object bodies")
	}
	// The explicit source core must verify the isolated public repository even
	// while ambient cwd remains the source. Its empty Cut is internal evidence,
	// never an accepted-view identity borrowed from the source/destination.
	quarantineDir := filepath.Join(dst.repo, ".git")
	quarantine, e := loadArtifactHistoryFromSources(context.Background(), quarantineDir, quarantineSources)
	if e != nil || quarantine.Cut != "" || quarantine.gitDir != quarantineDir {
		t.Fatal("explicit public verification borrowed accepted authority", quarantine, e)
	}
	verifiedClosure, e := selectArtifactClosure(context.Background(), quarantine, []string{opening})
	if e != nil || !reflect.DeepEqual(verifiedClosure.IDs, want) || !reflect.DeepEqual(verifiedClosure.Objects, lines) {
		t.Fatal("quarantine verifier disagrees with live producer", verifiedClosure, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = selectArtifactClosure(ctx, h, []string{opening})
	revisionError(t, e, "cancelled")
}

func TestArtifactClosureBeyondExchangeAncestryCLI(t *testing.T) {
	f := artifactCLI(t)
	previous, parent, last := "", "", ""
	for sequence := 1; sequence <= 257; sequence++ {
		raw, sig := artifactSignedCLI(t, func(m map[string]any) {
			m["operation"] = fmt.Sprintf("read-depth-%d", sequence)
			m["sequence"], m["previous"] = sequence, previous
		})
		last = eventID([]byte(raw))
		parent = artifactRawCommit(t, f, raw, sig, parent, "hn artifact "+last, false)
		previous = last
	}
	// The independent fixture signer is public test material, not f's identity.
	raw := f.git("show", parent+":artifact.json")
	actorStart := strings.Index(raw, `"actor":"`) + len(`"actor":"`)
	actor := raw[actorStart : actorStart+64]
	f.git("update-ref", artifactRef(actor), parent)
	before := f.snapshot()
	if ids := closureItemIDs(t, closureItems(f, last)); len(ids) != 257 {
		t.Fatal("ordinary inspect acquired the exchange ancestry cap", len(ids))
	}
	f.unchanged(before)
}
