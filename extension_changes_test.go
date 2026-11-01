package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"hubnot/internal/artifact"
	"hubnot/internal/extension"
)

func changesUnitError(t *testing.T, err error, code string) {
	t.Helper()
	var e *extension.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("want %s: %v", code, err)
	}
}
func TestExtensionChangesCanonicalLiteralsAndFullArray(t *testing.T) {
	value := struct {
		Z string `json:"z"`
		A any    `json:"a"`
	}{"<&>\u2028\u2029雪", map[string]any{"z": json.Number("9007199254740991"), "a": []any{true, nil, "\n"}}}
	got, err := extensionChangesCanonical(value)
	want := `{"a":{"a":[true,null,"\n"],"z":9007199254740991},"z":"\u003c\u0026\u003e\u2028\u2029雪"}`
	if err != nil || string(got) != want {
		t.Fatalf("literal C mismatch: %s %v", got, err)
	}
	shared, err := artifact.Canonical(got)
	if err != nil || !bytes.Equal(shared, got) {
		t.Fatal("shared C disagreement", err)
	}
	// This is trusted, verified-value serialization, not the signed-envelope parser:
	// the full capture may exceed its unrelated1024-array and1MiB input bounds.
	entries := make([]extensionChangeEntry, 10000)
	for i := range entries {
		entries[i] = extensionChangeEntry{Protocol: "hn/0", ResourceID: fmt.Sprintf("sha256:%064x", i), RevisionIDs: []string{}, Value: strings.Repeat("x", 128)}
	}
	encoded, err := extensionChangesBound(entries)
	if err != nil || len(encoded) <= 1048576 {
		t.Fatal("full array inherited envelope parser cap", len(encoded), err)
	}
	entries = append(entries, entries[0])
	_, err = extensionChangesBound(entries)
	changesUnitError(t, err, "resource_limit")
}
func TestExtensionChangesCanonicalByteBoundary(t *testing.T) {
	entries := []extensionChangeEntry{{Protocol: "hn/0", RevisionIDs: []string{}, Value: ""}}
	raw, err := extensionChangesCanonical(entries)
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Value = strings.Repeat("x", 16777216-len(raw))
	at, err := extensionChangesBound(entries)
	if err != nil || len(at) != 16777216 {
		t.Fatal("at16MiB", len(at), err)
	}
	entries[0].Value = entries[0].Value.(string) + "x"
	_, err = extensionChangesBound(entries)
	changesUnitError(t, err, "resource_limit")
	// HTML escaping is part of C capacity, not the unescaped Go string length.
	entries[0].Value = strings.Repeat("<", (16777216-len(raw))/6+1)
	_, err = extensionChangesBound(entries)
	changesUnitError(t, err, "resource_limit")
}
func TestExtensionChangesFrameFullWrapperBoundary(t *testing.T) {
	family := "issue"
	selection := extensionChangeSelection{Families: []string{"issue"}, Types: []string{}}
	capture := &extensionChangeCapture{token: strings.Repeat("a", 64), created: time.Unix(1793433600, 0), descriptor: extensionChangeCheckpoint{Selection: selection, SnapshotID: "sha256:" + strings.Repeat("a", 64), SelectionDigest: "sha256:" + strings.Repeat("b", 64), SourceCut: "sha256:" + strings.Repeat("c", 64)}, entries: []extensionChangeEntry{{Protocol: "hn/0", Family: &family, ResourceID: "sha256:" + strings.Repeat("d", 64), RevisionIDs: []string{}, Value: ""}}}
	call := extension.Invocation{Request: extension.Request{ID: "frame-test", Params: map[string]any{"limit": float64(1)}}}
	checkpoint := strings.Repeat("c", 129)
	page, err := extensionChangesRender(call, capture, 0, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(map[string]any{"protocol": "hn.extension/1", "request_id": call.Request.ID, "ok": true, "data": page, "error": nil})
	capture.entries[0].Value = strings.Repeat("x", 8388608-len(wire))
	page, err = extensionChangesRender(call, capture, 0, checkpoint)
	if err != nil {
		t.Fatal("at8MiB", err)
	}
	wire, _ = json.Marshal(map[string]any{"protocol": "hn.extension/1", "request_id": call.Request.ID, "ok": true, "data": page, "error": nil})
	if len(wire) != 8388608 {
		t.Fatal("wrong full frame size", len(wire))
	}
	capture.entries[0].Value = capture.entries[0].Value.(string) + "x"
	_, err = extensionChangesRender(call, capture, 0, checkpoint)
	changesUnitError(t, err, "resource_limit")
	if !strings.Contains(err.Error(), "export") {
		t.Fatal("one oversized state suggested smaller pages")
	}
}
func TestExtensionChangesPrivateProofAccountingAndEviction(t *testing.T) {
	// Constructed maximum source accounting, not a claim that200000 unique actor
	// fixtures were verified. The actual caller tests exercise real verified refs.
	descriptor := extensionChangeCheckpoint{Selection: extensionChangeSelection{Families: []string{}, Types: []string{strings.Repeat("x", 8<<20)}}, Facts: extensionChangeFacts{Refs: make([]extensionNativeRef, 200000), RetainedRefs: []extensionNativeRef{}}, RetainedDigest: "sha256:" + strings.Repeat("d", 64)}
	for i := range descriptor.Facts.Refs {
		descriptor.Facts.Refs[i] = extensionNativeRef{Ref: "refs/hn/remotes/" + strings.Repeat("r", 128) + "/artifacts/" + strings.Repeat("a", 64) + "/" + strings.Repeat("b", 64), OID: strings.Repeat("c", 64)}
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 128<<20 || len(raw)+16<<20 > 256<<20 {
		t.Fatal("maximum legal proof/capture cannot fit eviction budgets", len(raw))
	}
	descriptor.Facts.Refs = nil
	descriptor.Selection.Types = []string{}
	for _, fixture := range []struct {
		name   string
		offset int
		oldest int
	}{
		{name: "creation_order_mtimes", offset: 0, oldest: 0},
		// Creation-first descriptor 0 must survive: descriptor 32 is oldest.
		{name: "permuted_mtimes", offset: 32, oldest: 32},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			directory := t.TempDir()
			state, err := extensionChangesOpenState(directory, true)
			if err != nil {
				t.Fatal(err)
			}
			defer state.close()
			checkpointDir := filepath.Join(directory, "hn", "extensions", "changes")
			tokens := make([]string, 65)
			// Whole-day separation avoids filesystem timestamp-resolution ties.
			base := time.Unix(946684800, 0)
			for i := 0; i < 64; i++ {
				id := fmt.Sprintf("%064x", i)
				tokens[i] = state.token(id, descriptor)
				if err = state.write(id, descriptor); err != nil {
					t.Fatal(err)
				}
				mtime := base.Add(time.Duration((i+fixture.offset)%64) * 24 * time.Hour)
				path := filepath.Join(checkpointDir, id+".json")
				if err = os.Chtimes(path, mtime, mtime); err != nil {
					t.Fatal("set descriptor mtime", id, err)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal("read descriptor mtime", id, err)
				}
				if !info.ModTime().Equal(mtime) {
					t.Fatalf("descriptor %s mtime: want %s, got %s", id, mtime, info.ModTime())
				}
				t.Logf("descriptor %s desired_mtime=%s readback_mtime=%s", id, mtime.UTC().Format(time.RFC3339), info.ModTime().UTC().Format(time.RFC3339))
			}
			// All first 64 mtimes have been set and read back before pruning.
			current := fmt.Sprintf("%064x", 64)
			tokens[64] = state.token(current, descriptor)
			if err = state.write(current, descriptor); err != nil {
				t.Fatal(err)
			}
			_, err = state.read(tokens[fixture.oldest])
			changesUnitError(t, err, "checkpoint_invalidated")
			t.Logf("minimum-mtime token invalidated: %s", tokens[fixture.oldest])
			wantNames := map[string]bool{"key": true, "lock": true}
			for i, token := range tokens {
				if i == fixture.oldest {
					continue
				}
				if _, err = state.read(token); err != nil {
					t.Fatal("intended descriptor lost", i, token, err)
				}
				wantNames[fmt.Sprintf("%064x.json", i)] = true
				t.Logf("valid retained token: %s", token)
			}
			entries, err := os.ReadDir(checkpointDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 66 {
				t.Fatal("want exactly 64 descriptors plus key and lock", len(entries))
			}
			names := make([]string, 0, len(entries))
			descriptors := 0
			for _, entry := range entries {
				if !wantNames[entry.Name()] || !entry.Type().IsRegular() {
					t.Fatal("unexpected checkpoint entry", entry.Name(), entry.Type())
				}
				delete(wantNames, entry.Name())
				names = append(names, entry.Name())
				if strings.HasSuffix(entry.Name(), ".json") {
					descriptors++
				}
			}
			if descriptors != 64 || len(wantNames) != 0 {
				t.Fatal("checkpoint filename inventory mismatch", descriptors, wantNames)
			}
			t.Logf("exact checkpoint entries: %v", names)
		})
	}
	extensionChanges.Lock()
	prior := extensionChanges.captures
	extensionChanges.captures = map[string]*extensionChangeCapture{}
	extensionChanges.Unlock()
	defer func() { extensionChanges.Lock(); extensionChanges.captures = prior; extensionChanges.Unlock() }()
	for i := 0; i < 17; i++ {
		extensionChangesRetain(&extensionChangeCapture{token: fmt.Sprintf("%d", i), created: time.Now().Add(time.Duration(i) * time.Millisecond), bytes: 16 << 20})
	}
	extensionChanges.Lock()
	_, oldExists := extensionChanges.captures["0"]
	count := len(extensionChanges.captures)
	extensionChanges.Unlock()
	if oldExists || count != 16 {
		t.Fatal("capture eviction failed", oldExists, count)
	}
}
func TestExtensionChangesPrivateVerifierReplacementAndSymlink(t *testing.T) {
	common := t.TempDir()
	state, err := extensionChangesOpenState(common, true)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := extensionChangeCheckpoint{Selection: extensionChangeSelection{Families: []string{"issue"}, Types: []string{}}, Facts: extensionChangeFacts{Refs: []extensionNativeRef{}, RetainedRefs: []extensionNativeRef{}}}
	id := strings.Repeat("a", 64)
	token := state.token(id, descriptor)
	if err = state.write(id, descriptor); err != nil {
		t.Fatal(err)
	}
	state.close()
	key := filepath.Join(common, "hn", "extensions", "changes", "key")
	if err = os.WriteFile(key, bytes.Repeat([]byte{1}, 32), 0600); err != nil {
		t.Fatal(err)
	}
	state, err = extensionChangesOpenState(common, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.read(token)
	changesUnitError(t, err, "checkpoint_invalidated")
	state.close()
	os.Remove(key)
	if err = os.Symlink(filepath.Join(t.TempDir(), "unrelated"), key); err != nil {
		t.Fatal(err)
	}
	_, err = extensionChangesOpenState(common, true)
	if err == nil {
		t.Fatal("followed verifier symlink")
	}
}
func TestExtensionChangesErrorMappingPreservesDiagnostics(t *testing.T) {
	call := extension.Invocation{}
	cause := errors.New("PRIVATE /journal/key stderr")
	err := issueReadFailure("invalid_history", cause)
	public := extensionChangesError(err, call)
	changesUnitError(t, public, "verification_failure")
	if !errors.Is(err, cause) || strings.Contains(public.Error(), "PRIVATE") || strings.Contains(public.Error(), "/journal") {
		t.Fatal("private diagnostic boundary", public)
	}
	for _, code := range []string{"incomplete_dependency", "unsupported_schema", "actor_stream_conflict"} {
		mapped := extensionChangesError(&artifact.Error{Code: code, Message: "bounded artifact refusal"}, call).(*extension.Error)
		if mapped.Origin != "artifact" || mapped.Code != code {
			t.Fatal("artifact meaning erased", mapped)
		}
	}
	changesUnitError(t, extensionChangesError(context.Canceled, call), "cancelled")
}
func TestExtensionChangesRegisteredMethodSet(t *testing.T) {
	ensureOptionalBootstrap()
	got := []string{}
	for _, method := range []string{"changes.capture", "changes.page", "changes.since"} {
		if extensionHostMethods[method] != nil {
			got = append(got, method)
		}
	}
	if !reflect.DeepEqual(got, []string{"changes.capture", "changes.page", "changes.since"}) || len(extensionHostMethods) != 13 {
		t.Fatal("actual method registration drift", got, len(extensionHostMethods))
	}
	// capabilities.list is host-owned; thirteen adapters plus that live method are
	// the frozen fourteen-method inventory, without new unsupported surfaces.
}

// Source-assisted emission guards supplement the real wall-clock raw-host case.
// An expired object cannot issue a page/checkpoint even after projection work;
// cancellation keeps its own cause and no persistence is attempted.
func TestExtensionChangesExpiryAtEmission(t *testing.T) {
	expired := &extensionChangeCapture{token: "expired-emission", created: time.Now().Add(-10 * time.Minute)}
	_, err := extensionChangesPage(context.Background(), extension.Invocation{}, expired, 0, nil)
	changesUnitError(t, err, "checkpoint_invalidated")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = extensionChangesPage(ctx, extension.Invocation{}, expired, 0, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("expiry erased cancellation", err)
	}
}

func TestExtensionChangesSourceRaceClassification(t *testing.T) {
	for _, err := range []error{issueReadFailure("concurrent_update", errors.New("source changed")), issueFailure("concurrent_update", errors.New("source changed")), &artifact.Error{Code: "concurrent_update", Message: "source changed"}} {
		if !extensionChangesSourceRace(err) {
			t.Fatal("lost source-race cause", err)
		}
	}
	for _, err := range []error{context.Canceled, issueFailure("invalid_history", errors.New("bad signature")), issueReadFailure("invalid_history", issueFailure("concurrent_update", errors.New("underlying race is not accepted verification"))), &artifact.Error{Code: "unsupported_schema", Message: "unsupported"}, extensionChangesInvalid(), errors.New("plain repository failure")} {
		if extensionChangesSourceRace(err) {
			t.Fatal("arbitrary refusal became page race", err)
		}
	}
}
