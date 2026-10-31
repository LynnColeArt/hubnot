package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// The fixture writer is deliberately Git-only. No production writer or verifier
// constructs the malformed tree/chain controls consumed by the actual hn reader.
func artifactRawCommit(t *testing.T, f *artifactCLIFixture, raw, sig, parent, message string, extra bool) string {
	t.Helper()
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "object")
		if e := os.WriteFile(p, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		return f.git("hash-object", "-w", p)
	}
	payload := write(raw)
	signature := write(sig)
	treeText := fmt.Sprintf("100644 blob %s\tartifact.json\n100644 blob %s\tsignature\n", payload, signature)
	if extra {
		treeText += fmt.Sprintf("100644 blob %s\tprivate.txt\n", payload)
	}
	path := filepath.Join(t.TempDir(), "tree")
	if e := os.WriteFile(path, []byte(treeText), 0600); e != nil {
		t.Fatal(e)
	}
	// mktree input is independent from the application.
	tree := f.gitInput(treeText, "mktree")
	args := []string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-m", message}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	return f.git(args...)
}
func artifactFrozenRef(t *testing.T, f *artifactCLIFixture) string {
	t.Helper()
	var env map[string]any
	if e := json.Unmarshal([]byte(artifactVector), &env); e != nil {
		t.Fatal(e)
	}
	actor := env["actor"].(string)
	stream := sha256.Sum256([]byte("hn-artifact-stream-v1\x00" + actor))
	return "refs/hn/artifacts/" + actor + "/" + hex.EncodeToString(stream[:])
}
func TestArtifactStoreAcceptedReadCLI(t *testing.T) {
	f := artifactCLI(t)
	ref := artifactFrozenRef(t, f)
	commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	accepted := strings.Replace(ref, "refs/hn/", "refs/hn/remotes/origin/", 1)
	f.git("update-ref", accepted, commit)
	shown := f.call("", "", "artifact", "type", "show", artifactVectorID, "--json")["data"].(map[string]any)
	if len(shown["items"].([]any)) != 1 {
		t.Fatal(shown)
	}
	f.git("update-ref", "-d", accepted)
	f.git("update-ref", "refs/hn/quarantine/artifacts/test", commit)
	f.git("update-ref", "refs/other/unselected", commit)
	before := f.snapshot()
	f.call("", "not_found", "artifact", "type", "show", artifactVectorID, "--json")
	f.unchanged(before)
	f.git("update-ref", ref, commit)
	f.call("", "", "artifact", "type", "show", artifactVectorID, "--json")
	for _, control := range []struct {
		name, raw, sig, parent, message, code string
		extra                                 bool
	}{
		{"signature", artifactVector, strings.Repeat("A", 86), "", "hn artifact " + artifactVectorID, "verification_failure", false},
		{"extra-entry", artifactVector, artifactVectorSignature, "", "hn artifact " + artifactVectorID, "invalid_history", true},
		{"wrong-message", artifactVector, artifactVectorSignature, "", "wrong", "invalid_history", false},
		{"genesis-with-parent", artifactVector, artifactVectorSignature, commit, "hn artifact " + artifactVectorID, "invalid_history", false},
	} {
		t.Run(control.name, func(t *testing.T) {
			bad := artifactRawCommit(t, f, control.raw, control.sig, control.parent, control.message, control.extra)
			f.git("update-ref", ref, bad)
			before := f.snapshot()
			f.call("", control.code, "artifact", "type", "show", artifactVectorID, "--json")
			f.unchanged(before)
			f.git("update-ref", ref, commit)
		})
	}
	// A type publication with a correctly signed, absent evolution parent refuses
	// completeness; an invalid history must never become an empty successful page.
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = 7
	}
	key := ed25519.NewKeyFromSeed(seed)
	public := key.Public().(ed25519.PublicKey)
	actorSum := sha256.Sum256(public)
	actor := hex.EncodeToString(actorSum[:])
	var env map[string]any
	json.Unmarshal([]byte(artifactVector), &env)
	env["actor"] = actor
	env["public_key"] = base64.RawStdEncoding.EncodeToString(public)
	stream := sha256.Sum256([]byte("hn-artifact-stream-v1\x00" + actor))
	env["stream"] = "sha256:" + hex.EncodeToString(stream[:])
	env["parents"] = []string{"sha256:" + strings.Repeat("1", 64)}
	semantic := map[string]any{}
	for _, k := range []string{"actor", "kind", "operation", "parents", "payload", "protocol", "record"} {
		semantic[k] = env[k]
	}
	request, _ := json.Marshal(semantic)
	sum := sha256.Sum256(append([]byte("hn-artifact-request-v1\x00"), request...))
	env["request"] = "sha256:" + hex.EncodeToString(sum[:])
	raw, _ := json.Marshal(env)
	digest := sha256.Sum256(raw)
	id := "sha256:" + hex.EncodeToString(digest[:])
	sig := base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), raw...)))
	bad := artifactRawCommit(t, f, string(raw), sig, "", "hn artifact "+id, false)
	f.git("update-ref", "refs/hn/artifacts/"+actor+"/"+hex.EncodeToString(stream[:]), bad)
	before = f.snapshot()
	f.call("", "incomplete_dependency", "artifact", "type", "show", artifactVectorID, "--json")
	f.unchanged(before)
}

func TestArtifactStoreMissingObjectNoFetchCLI(t *testing.T) {
	f := artifactCLI(t)
	ref := artifactFrozenRef(t, f)
	commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	f.git("update-ref", ref, commit)
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrapper := t.TempDir()
	marker := filepath.Join(wrapper, "forbidden")
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = fetch ]; then echo fetch >> \"$HN_WP05_MARKER\"; fi\n if [ \"$arg\" = cat-file ] && [ \"$GIT_NO_LAZY_FETCH\" != 1 ]; then echo lazy-fetch-enabled >> \"$HN_WP05_MARKER\"; fi\ndone\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	f.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP05_MARKER": marker}
	f.call("", "", "artifact", "type", "show", artifactVectorID, "--json")
	oid := f.git("rev-parse", commit+":signature")
	f.git("config", "remote.promised.url", filepath.Join(t.TempDir(), "unavailable"))
	f.git("config", "remote.promised.promisor", "true")
	f.git("config", "remote.promised.partialclonefilter", "blob:none")
	if e = os.Remove(filepath.Join(f.repo, ".git", "objects", oid[:2], oid[2:])); e != nil {
		t.Fatal(e)
	}
	before := f.snapshot()
	f.call("", "verification_failure", "artifact", "type", "show", artifactVectorID, "--json")
	f.unchanged(before)
	if b, e := os.ReadFile(marker); !os.IsNotExist(e) {
		t.Fatalf("forbidden fetch path %v %s", e, b)
	}
}

func TestArtifactStoreSelectedRefBudget(t *testing.T) {
	f := artifactCLI(t)
	ref := artifactFrozenRef(t, f)
	commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	suffix := strings.TrimPrefix(ref, "refs/hn/")
	// Real packed refs make the exact 100000/100001 boundary practical without
	// creating hundreds of thousands of filesystem entries. Git consumes them.
	var packed bytes.Buffer
	packed.WriteString("# pack-refs with: sorted\n")
	for i := 0; i < artifactReadEntries; i++ {
		fmt.Fprintf(&packed, "%s refs/hn/remotes/r%06d/%s\n", commit, i, suffix)
	}
	path := filepath.Join(f.repo, ".git", "packed-refs")
	if e := os.WriteFile(path, packed.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sources, e := artifactSources(ctx, filepath.Join(f.repo, ".git"))
	if e != nil || len(sources) != artifactReadEntries {
		t.Fatalf("selected refs at bound: %d %v", len(sources), e)
	}
	fmt.Fprintf(&packed, "%s refs/hn/remotes/r%06d/%s\n", commit, artifactReadEntries, suffix)
	if e = os.WriteFile(path, packed.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	before := f.snapshot()
	f.call("", "resource_limit", "artifact", "type", "show", artifactVectorID, "--json")
	f.unchanged(before)
}

func TestArtifactStoreLargeVerifiedParentContextCLI(t *testing.T) {
	f := artifactCLI(t)
	// A legal selected stream has thirty imported sibling revisions. Each payload
	// is individually below1MiB; the complete history is below256MiB. The resolve
	// context exceeds the INERT file validator's16MiB cap, which is not a read cap.
	seed := bytes.Repeat([]byte{19}, 32)
	key := ed25519.NewKeyFromSeed(seed)
	public := key.Public().(ed25519.PublicKey)
	actorDigest := sha256.Sum256(public)
	actor := hex.EncodeToString(actorDigest[:])
	streamDigest := sha256.Sum256([]byte("hn-artifact-stream-v1\x00" + actor))
	stream := "sha256:" + hex.EncodeToString(streamDigest[:])
	sequence := 0
	previous, commit := "", ""
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(sum[:]) }
	appendPublic := func(kind, record, operation string, parents []string, payload map[string]any) string {
		sequence++
		semantic := map[string]any{"actor": actor, "kind": kind, "operation": operation, "parents": parents, "payload": payload, "protocol": "hn-artifact/1", "record": record}
		request, _ := json.Marshal(semantic)
		env := map[string]any{}
		for k, v := range semantic {
			env[k] = v
		}
		env["public_key"] = base64.RawStdEncoding.EncodeToString(public)
		env["stream"] = stream
		env["sequence"] = sequence
		env["previous"] = previous
		env["timestamp"] = "2026-10-30T00:00:00Z"
		env["request"] = digest(append([]byte("hn-artifact-request-v1\x00"), request...))
		raw, e := json.Marshal(env)
		if e != nil {
			t.Fatal(e)
		}
		if len(raw) > 1048576 {
			t.Fatal("fixture violates envelope cap")
		}
		id := digest(raw)
		sig := base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), raw...)))
		commit = artifactRawCommit(t, f, string(raw), sig, commit, "hn artifact "+id, false)
		previous = id
		return id
	}
	var mutation map[string]any
	if e := json.Unmarshal([]byte(artifactStoreType), &mutation); e != nil {
		t.Fatal(e)
	}
	typ := appendPublic("type.publish", "", "large-type", []string{}, mutation["payload"].(map[string]any))
	payload := map[string]any{"shape": "document", "type_id": "example.test/note", "type_revision": typ, "title": "Imported branches", "content": strings.Repeat("v", 600000), "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []string{}}
	opening := appendPublic("record.open", "", "large-open", []string{}, payload)
	heads := []string{}
	for i := 0; i < 30; i++ {
		heads = append(heads, appendPublic("record.revise", opening, fmt.Sprintf("large-branch-%d", i), []string{opening}, payload))
	}
	sort.Strings(heads)
	payload["content"] = "Explicit resolution of every imported head"
	appendPublic("record.resolve", opening, "large-resolve", heads, payload)
	f.git("update-ref", "refs/hn/remotes/imported/artifacts/"+actor+"/"+strings.TrimPrefix(stream, "sha256:"), commit)
	before := f.snapshot()
	shown := f.call("", "", "artifact", "type", "show", typ, "--json")["data"].(map[string]any)
	if len(shown["items"].([]any)) != 1 {
		t.Fatal(shown)
	}
	f.unchanged(before)
}

func TestArtifactStoreConflictingActorGitHistoriesCLI(t *testing.T) {
	f := artifactCLI(t)
	ref := artifactFrozenRef(t, f)
	first := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	tree := f.git("rev-parse", first+"^{tree}")
	second := f.git("-c", "user.name=Different transport author", "-c", "user.email=other@example.invalid", "commit-tree", tree, "-m", "hn artifact "+artifactVectorID)
	if first == second {
		t.Fatal("negative control failed to create divergent Git histories")
	}
	f.git("update-ref", ref, first)
	f.git("update-ref", strings.Replace(ref, "refs/hn/", "refs/hn/remotes/divergent/", 1), second)
	before := f.snapshot()
	f.call("", "actor_stream_conflict", "artifact", "type", "show", artifactVectorID, "--json")
	f.unchanged(before)
}

func TestArtifactStoreCorruptGitObjectCLI(t *testing.T) {
	f := artifactCLI(t)
	ref := artifactFrozenRef(t, f)
	commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	f.git("update-ref", ref, commit)
	path := filepath.Join(f.repo, ".git", "objects", commit[:2], commit[2:])
	packed, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	reader, e := zlib.NewReader(bytes.NewReader(packed))
	if e != nil {
		t.Fatal(e)
	}
	body, e := io.ReadAll(reader)
	if e != nil {
		t.Fatal(e)
	}
	if e = reader.Close(); e != nil {
		t.Fatal(e)
	}
	changed := bytes.Replace(body, []byte("Fixture"), []byte("Fixturz"), 1)
	if bytes.Equal(body, changed) {
		t.Fatal("corrupt-ID control did not change transport metadata")
	}
	var out bytes.Buffer
	writer := zlib.NewWriter(&out)
	if _, e = writer.Write(changed); e != nil {
		t.Fatal(e)
	}
	if e = writer.Close(); e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(path, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	before := f.snapshot()
	v := f.call("", "verification_failure", "artifact", "type", "show", artifactVectorID, "--json")
	if v["error"].(map[string]any)["message"] != "accepted Git object hash mismatch" {
		t.Fatal("wrong corrupt-ID refusal cause", v)
	}
	f.unchanged(before)
}

func TestArtifactStoreKeylessReadCLI(t *testing.T) {
	root := t.TempDir()
	f := &artifactCLIFixture{t: t, binary: artifactStoreBinary(t), repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
	f.git("init", "--quiet")
	ref := artifactFrozenRef(t, f)
	commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	f.git("update-ref", ref, commit)
	for _, path := range []string{filepath.Join(f.repo, ".hn", "extensions.json"), filepath.Join(f.home, ".config", "hubnot", "extensions", "registry.json")} {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte("MALFORMED_PRIVATE_EXTENSION_SENTINEL"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	before := f.snapshot()
	f.call("", "", "artifact", "type", "show", artifactVectorID, "--json")
	f.unchanged(before)
	if _, e := os.Stat(filepath.Join(f.repo, ".git", "hn")); !os.IsNotExist(e) {
		t.Fatal("keyless read initialized private state", e)
	}
}

func TestArtifactStoreOpeningReplayObservesImportedHeadsCLI(t *testing.T) {
	f := artifactCLI(t)
	typ := f.publish(artifactStoreType, "type", "")["data"].(map[string]any)["event_id"].(string)
	input := `{"record":null,"parents":[],"payload":{"shape":"document","type_id":"example.test/note","type_revision":"` + typ + `","title":"Original opening","content":"public data","fields":{},"members":[],"relationships":[],"conversion_sources":[]}}`
	args := []string{"artifact", "open", "--input", "-", "--actor", f.actor, "--operation", "original-open", "--json"}
	original := f.call(input, "", args...)["data"].(map[string]any)
	opening := original["event_id"].(string)
	var mutation map[string]any
	if e := json.Unmarshal([]byte(input), &mutation); e != nil {
		t.Fatal(e)
	}
	for _, seedByte := range []byte{17, 18} {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seedByte}, 32))
		public := key.Public().(ed25519.PublicKey)
		sum := sha256.Sum256(public)
		actor := hex.EncodeToString(sum[:])
		sum = sha256.Sum256([]byte("hn-artifact-stream-v1\x00" + actor))
		stream := "sha256:" + hex.EncodeToString(sum[:])
		semantic := map[string]any{"protocol": "hn-artifact/1", "actor": actor, "operation": "imported-branch", "kind": "record.revise", "record": opening, "parents": []string{opening}, "payload": mutation["payload"]}
		request, _ := json.Marshal(semantic)
		sum = sha256.Sum256(append([]byte("hn-artifact-request-v1\x00"), request...))
		env := map[string]any{}
		for k, v := range semantic {
			env[k] = v
		}
		env["request"] = "sha256:" + hex.EncodeToString(sum[:])
		env["public_key"] = base64.RawStdEncoding.EncodeToString(public)
		env["stream"] = stream
		env["sequence"] = 1
		env["previous"] = ""
		env["timestamp"] = "2026-10-30T00:00:00Z"
		raw, _ := json.Marshal(env)
		sum = sha256.Sum256(raw)
		id := "sha256:" + hex.EncodeToString(sum[:])
		sig := base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), raw...)))
		commit := artifactRawCommit(t, f, string(raw), sig, "", "hn artifact "+id, false)
		f.git("update-ref", "refs/hn/remotes/imported/artifacts/"+actor+"/"+strings.TrimPrefix(stream, "sha256:"), commit)
	}
	before := f.snapshot()
	replayed := f.call(input, "", args...)["data"].(map[string]any)
	if replayed["replayed"] != true || replayed["resource_id"] != opening || replayed["event_id"] != opening || replayed["head_count"] != float64(2) || replayed["conflict"] != true || replayed["source_cut"] == original["source_cut"] {
		t.Fatal("replay changed immutable identity or hid imported conflict", replayed)
	}
	f.unchanged(before)
}
