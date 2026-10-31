package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func exchangeObject(kind string, body []byte) artifactExchangeObject {
	return artifactExchangeObject{OID: artifactExchangeHash(kind, body, 40), Type: kind, Data: base64.RawStdEncoding.EncodeToString(body), body: body}
}
func exchangeBundle(objects ...artifactExchangeObject) *artifactExchange {
	return &artifactExchange{Protocol: "hn-artifact-exchange/1", Mode: "opaque", Refs: []artifactSource{}, Objects: objects}
}
func TestArtifactExchangePublishedBudget(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		initial artifactExchangeBudget
		n       int
		message string
	}{
		{"objects", artifactExchangeBudget{objects: 4095}, 0, "object count"},
		{"decoded", artifactExchangeBudget{decoded: 33554431}, 1, "decoded aggregate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.initial
			if e := b.charge(ctx, tc.n); e != nil {
				t.Fatal("equality refused", e)
			}
			before := b
			e := b.charge(ctx, 1)
			revisionError(t, e, "resource_limit")
			if !strings.Contains(e.Error(), tc.message) || b != before {
				t.Fatal("wrong bound or charged refused input", e, b)
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	b := artifactExchangeBudget{}
	revisionError(t, b.charge(cancelled, 0), "cancelled")
	if b != (artifactExchangeBudget{}) {
		t.Fatal("cancelled input charged")
	}
	if e := artifactExchangeCheckRecords(ctx, 2048); e != nil {
		t.Fatal("verified signed-record equality refused", e)
	}
	e := artifactExchangeCheckRecords(ctx, 2049)
	revisionError(t, e, "resource_limit")
	if !strings.Contains(e.Error(), "signed record count") {
		t.Fatal(e)
	}
	revisionError(t, artifactExchangeCheckRecords(cancelled, 2048), "cancelled")
	if artifactExchangeStage != 30*time.Second {
		t.Fatal("stage ceiling drift")
	}
}
func TestArtifactExchangeStrictObjectTable(t *testing.T) {
	objects := []artifactExchangeObject{}
	for i := 0; i < 4096; i++ {
		objects = append(objects, exchangeObject("blob", []byte(fmt.Sprintf("public-%d", i))))
	}
	raw, _ := json.Marshal(exchangeBundle(objects...))
	b, e := decodeArtifactExchange(context.Background(), raw, 40)
	if e != nil || len(b.Objects) != 4096 {
		t.Fatal("4096 objects equality refused", e)
	}
	objects = append(objects, exchangeObject("blob", []byte("one-over")))
	raw, _ = json.Marshal(exchangeBundle(objects...))
	_, e = decodeArtifactExchange(context.Background(), raw, 40)
	revisionError(t, e, "resource_limit")
	// Signed-record and decoded caps are independently charged in the actual
	// validation/copy budget. Base64's16MiB wire ceiling makes32MiB decoded input
	// unreachable through a valid file; these tests do not claim such a file exists.
	for _, tc := range []struct{ name, raw, code string }{
		{"null-object", `{"protocol":"hn-artifact-exchange/1","mode":"opaque","refs":[],"objects":[null]}`, "invalid_input"},
		{"duplicate-nested-key", `{"protocol":"hn-artifact-exchange/1","mode":"opaque","refs":[],"objects":[{"oid":"x","oid":"y","type":"blob","data":""}]}`, "invalid_input"},
		{"surrogate", `{"protocol":"hn-artifact-exchange/1","mode":"\ud800","refs":[],"objects":[]}`, "invalid_input"},
		{"unknown-field", `{"protocol":"hn-artifact-exchange/1","mode":"opaque","refs":[],"objects":[],"grant":true}`, "invalid_input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, e := decodeArtifactExchange(context.Background(), []byte(tc.raw), 40)
			revisionError(t, e, tc.code)
		})
	}
}
func TestArtifactExchangeAncestryBound(t *testing.T) {
	objects := []artifactExchangeObject{}
	parent := ""
	for i := 0; i < 257; i++ {
		body := "tree " + strings.Repeat("1", 40) + "\n"
		if parent != "" {
			body += "parent " + parent + "\n"
		}
		body += "\npublic\n"
		o := exchangeObject("commit", []byte(body))
		objects = append(objects, o)
		parent = o.OID
		b := exchangeBundle(objects...)
		b.Refs = []artifactSource{{artifactRef(strings.Repeat("a", 64)), parent}}
		e := artifactExchangeAncestry(context.Background(), b)
		if i < 256 && e != nil {
			t.Fatal("at/below depth refused", i+1, e)
		}
		if i == 256 {
			revisionError(t, e, "resource_limit")
			if !strings.Contains(e.Error(), "depth") {
				t.Fatal(e)
			}
		}
	}
}
func TestArtifactExchangeConsumptionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := readArtifactExchange(ctx, filepath.Join(t.TempDir(), "absent"))
	if e == nil {
		t.Fatal("missing input unexpectedly read")
	}
	_, e = decodeArtifactExchange(ctx, []byte(`{}`), 40)
	revisionError(t, e, "cancelled")
	dir := t.TempDir()
	path := filepath.Join(dir, "out")
	revisionError(t, artifactExchangeWrite(ctx, path, []byte("public")), "cancelled")
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("cancelled copy created output", e)
	}
	// An expired actual copy stage cannot create Git objects or run a subprocess.
	revisionError(t, artifactExchangeCopy(ctx, dir, []artifactExchangeObject{exchangeObject("blob", []byte("public"))}), "cancelled")
}
func TestArtifactExchangeContractMirror(t *testing.T) {
	artifactExchangeModes := map[string]bool{}
	for _, value := range artifactExchangeModeVocabulary() {
		artifactExchangeModes[value] = true
	}

	artifactExchangeObjectTypes := map[string]bool{}
	for _, value := range artifactExchangeObjectTypeVocabulary() {
		artifactExchangeObjectTypes[value] = true
	}

	if !reflect.DeepEqual(artifactExchangeModes, map[string]bool{"accepted": true, "opaque": true}) || !reflect.DeepEqual(artifactExchangeObjectTypes, map[string]bool{"blob": true, "tree": true, "commit": true}) || artifactExchangeProtocol != "hn-artifact-exchange/1" {
		t.Fatal("exchange closed inventories drifted")
	}
	// Independent frozen field inventories catch both missing and additional keys.
	b := exchangeBundle(exchangeObject("blob", []byte("public")))
	raw, e := artifactExchangeBytes(b)
	if e != nil {
		t.Fatal(e)
	}
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	keys := func(v map[string]any) map[string]bool {
		out := map[string]bool{}
		for k := range v {
			out[k] = true
		}
		return out
	}
	if !reflect.DeepEqual(keys(value), map[string]bool{"protocol": true, "mode": true, "refs": true, "objects": true}) {
		t.Fatal(value)
	}
	if !reflect.DeepEqual(keys(value["objects"].([]any)[0].(map[string]any)), map[string]bool{"oid": true, "type": true, "data": true}) {
		t.Fatal(value)
	}
	for _, mode := range []string{"accepted", "opaque"} {
		b.Mode = mode
		raw, _ = artifactExchangeBytes(b)
		if _, e = decodeArtifactExchange(context.Background(), raw, 40); e != nil {
			t.Fatal("frozen mode missing", mode, e)
		}
	}
	for _, mode := range []string{"", "unsupported", "accepted ", "automatic"} {
		b.Mode = mode
		raw, _ = json.Marshal(b)
		_, e = decodeArtifactExchange(context.Background(), raw, 40)
		revisionError(t, e, "invalid_input")
	}
	for _, kind := range []string{"blob", "tree", "commit"} {
		raw, _ = json.Marshal(exchangeBundle(exchangeObject(kind, []byte("public"))))
		if _, e = decodeArtifactExchange(context.Background(), raw, 40); e != nil {
			t.Fatal("frozen object type missing", kind, e)
		}
	}
	for _, kind := range []string{"tag", "", "Blob", "private"} {
		raw, _ = json.Marshal(exchangeBundle(exchangeObject(kind, []byte("public"))))
		_, e = decodeArtifactExchange(context.Background(), raw, 40)
		revisionError(t, e, "invalid_input")
	}
	// Opaque identity uses the domain-separated type/body preimage, not Git's hash.
	o := exchangeObject("blob", []byte("public"))
	if artifactOpaqueID(o) != eventID([]byte("blob\x00public")) || artifactOpaqueID(o) == "sha256:"+o.OID {
		t.Fatal("opaque identity mismatch")
	}
	if bytes.Contains(raw, []byte("body")) {
		t.Fatal("decoded implementation storage leaked")
	}
}

func TestArtifactExchangeLiveCopyCancellation(t *testing.T) {
	f := artifactCLI(t)
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrap := t.TempDir()
	marker := filepath.Join(t.TempDir(), "copy-dispatched")
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = hash-object ]; then : > \"$HN_WP10_CANCEL_MARKER\"; exec sleep 30; fi\ndone\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", wrap+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HN_WP10_CANCEL_MARKER", marker)
	before := f.snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- artifactExchangeCopy(ctx, filepath.Join(f.repo, ".git"), []artifactExchangeObject{exchangeObject("blob", []byte("must not be copied"))})
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, e = os.Stat(marker); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("copy cancellation control never dispatched")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case e := <-done:
		revisionError(t, e, "cancelled")
	case <-time.After(3 * time.Second):
		t.Fatal("copy subprocess outlived cancellation")
	}
	f.unchanged(before)
}
func TestArtifactExchangeRefCancellationAfterPublication(t *testing.T) {
	f := artifactCLI(t)
	_ = f.publish(artifactStoreType, "type", "")
	oid := f.git("rev-parse", artifactRef(f.actor))
	ref := "refs/hn/remotes/cancel/artifacts/" + f.actor + "/" + strings.TrimPrefix(artifactStream(f.actor), "sha256:")
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrap := t.TempDir()
	marker := filepath.Join(t.TempDir(), "published")
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = update-ref ]; then '" + git + "' \"$@\" || exit $?; : > \"$HN_WP10_CANCEL_MARKER\"; exec sleep 30; fi\ndone\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", wrap+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HN_WP10_CANCEL_MARKER", marker)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- artifactExchangeSetRefs(ctx, filepath.Join(f.repo, ".git"), []artifactSource{{ref, oid}}, map[string]string{})
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, e = os.Stat(marker); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("publication control never reached committed transaction")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case e := <-done:
		revisionError(t, e, "uncertain_operation")
	case <-time.After(3 * time.Second):
		t.Fatal("ref transaction process did not settle")
	}
	// Query with the unwrapped executable: the retained public ref proves that a
	// post-dispatch cancellation must never be mislabeled a definitive refusal.
	cmd := exec.Command(git, "-C", f.repo, "rev-parse", ref)
	raw, e := cmd.Output()
	if e != nil || strings.TrimSpace(string(raw)) != oid {
		t.Fatal("publication witness missing", e, string(raw))
	}
}

func TestArtifactExchangeUnavailableTransactionWitnesses(t *testing.T) {
	f := artifactCLI(t)
	_ = f.publish(artifactStoreType, "type", "")
	oid := f.git("rev-parse", artifactRef(f.actor))
	ref := "refs/hn/remotes/unavailable/artifacts/" + f.actor + "/" + strings.TrimPrefix(artifactStream(f.actor), "sha256:")
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrap := t.TempDir()
	marker := filepath.Join(t.TempDir(), "transaction-error")
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = update-ref ];then : > \"$HN_WP10_CANCEL_MARKER\";exit 86;fi\n if [ \"$arg\" = for-each-ref ] && [ -f \"$HN_WP10_CANCEL_MARKER\" ];then exit 87;fi\ndone\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", wrap+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HN_WP10_CANCEL_MARKER", marker)
	before := f.snapshot()
	e = artifactExchangeSetRefs(context.Background(), filepath.Join(f.repo, ".git"), []artifactSource{{ref, oid}}, map[string]string{})
	revisionError(t, e, "uncertain_operation")
	if !strings.Contains(e.Error(), "witnesses") {
		t.Fatal(e)
	}
	f.unchanged(before)
}
