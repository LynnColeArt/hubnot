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
	"errors"
	"fmt"
	"hubnot/internal/artifact"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	// Exercise the same bound through explicit witness verification, not merely
	// enumeration. All actual packed refs share one valid public chain.
	atBound := f.snapshot()
	history, e := loadArtifactHistoryFromSources(ctx, filepath.Join(f.repo, ".git"), sources)
	if e != nil || len(history.Sources) != 100000 || len(history.ByID) != 1 {
		t.Fatalf("explicit selected refs at bound: %v", e)
	}
	f.unchanged(atBound)
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

// Pin the product child configuration before initialization and keep the pin on
// the fixture, so creating a second actor cannot redirect the first actor.
func wp25ArtifactCLI(t *testing.T) *artifactCLIFixture {
	t.Helper()
	root := t.TempDir()
	f := &artifactCLIFixture{t: t, binary: artifactStoreBinary(t), repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
	// A normal user HOME exists before product invocation; create only the
	// fixture parent, without registry, enablement or identity state.
	if err := os.MkdirAll(f.home, 0700); err != nil {
		t.Fatal(err)
	}
	config := wp25FixtureConfig(t, f.home)
	f.overrides = map[string]string{"XDG_CONFIG_HOME": config}
	f.git("init", "--quiet")
	f.call("", "", "init", "--name", "Artifact author")
	f.actor = f.call("", "", "identity", "show", "--json")["data"].(map[string]any)["actor"].(string)
	return f
}

func TestArtifactStoreKeylessReadCLI(t *testing.T) {
	root := t.TempDir()
	f := &artifactCLIFixture{t: t, binary: artifactStoreBinary(t), repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
	config := wp25FixtureConfig(t, f.home)
	f.overrides = map[string]string{"XDG_CONFIG_HOME": config}
	f.git("init", "--quiet")
	ref := artifactFrozenRef(t, f)
	commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	f.git("update-ref", ref, commit)
	for _, path := range []string{filepath.Join(f.repo, ".hn", "extensions.json"), filepath.Join(config, "hubnot", "extensions", "registry.json"), filepath.Join(f.home, ".config", "hubnot", "extensions", "registry.json")} {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte("MALFORMED_PRIVATE_EXTENSION_SENTINEL"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	wp25MalformedRegistryReadControl(t, f.binary, f.repo, f.home, config)
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

// The explicit public reader must neither discover the caller's repository nor
// borrow its Cut. These fixtures contain public Git data only, with no identity.
func TestArtifactStoreExplicitPublicSources(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			f := &artifactCLIFixture{t: t, repo: filepath.Join(root, "public.git"), home: filepath.Join(root, "home")}
			f.git("init", "--quiet", "--bare", "--object-format="+format)
			ref := artifactFrozenRef(t, f)
			commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			f.git("update-ref", ref, commit)
			// An unrelated malformed accepted-looking ref is outside this explicit set.
			f.git("update-ref", "refs/hn/artifacts/unselected", commit)
			accepted := strings.Replace(ref, "refs/hn/", "refs/hn/remotes/origin/", 1)
			f.git("update-ref", accepted, commit)
			sources := []artifactSource{{Ref: accepted, OID: commit}, {Ref: ref, OID: commit}}
			original := append([]artifactSource(nil), sources...)
			before := f.snapshot()
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if cwd == f.repo {
				t.Fatal("fixture must remain outside selected public repository")
			}
			h, err := loadArtifactHistoryFromSources(context.Background(), f.repo, sources)
			if err != nil {
				t.Fatal(err)
			}
			if len(h.ByID) != 1 || h.ByID[artifactVectorID] == nil || len(h.Chains) != 1 || h.gitDir != f.repo || h.readBytes <= 0 || len(h.readObjects) != 4 || h.Cut != "" {
				t.Fatalf("explicit public result acquired ambient authority or lost verification: %+v", h)
			}
			if h.Sources[0].Ref != ref || !reflect.DeepEqual(sources, original) {
				t.Fatal("sources must be copied and sorted", h.Sources, sources)
			}
			if string(h.ByID[artifactVectorID].verified.Bytes()) != artifactVector {
				t.Fatal("signed identity changed")
			}
			if got, _ := os.Getwd(); got != cwd {
				t.Fatal("reader changed caller cwd")
			}
			f.unchanged(before)
			for _, tc := range []struct {
				name    string
				sources []artifactSource
				code    string
			}{
				{"duplicate", []artifactSource{{ref, commit}, {ref, commit}}, "invalid_history"},
				{"malformed-ref", []artifactSource{{"refs/hn/quarantine/artifacts/test", commit}}, "invalid_history"},
				{"invalid-oid", []artifactSource{{ref, "not-an-oid"}}, "invalid_history"},
				{"uppercase-oid", []artifactSource{{ref, strings.ToUpper(commit)}}, "invalid_history"},
				{"wrong-object-format", []artifactSource{{ref, strings.Repeat("1", 104-len(commit))}}, "invalid_history"},
				{"missing-ref", []artifactSource{{strings.Replace(accepted, "origin", "missing", 1), commit}}, "concurrent_update"},
				{"over-ref-bound", make([]artifactSource, 100001), "resource_limit"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					before := f.snapshot()
					_, err := loadArtifactHistoryFromSources(context.Background(), f.repo, tc.sources)
					var failure *artifact.Error
					if !errors.As(err, &failure) || failure.Code != tc.code {
						t.Fatalf("want %s, got %v", tc.code, err)
					}
					f.unchanged(before)
				})
			}
			// A valid alternate Git commit changes the selected witness even though its
			// signed semantic ID is identical. It cannot satisfy the supplied old OID.
			changed := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			if changed == commit { // commit-tree timestamps can coincide; change metadata explicitly.
				changed = f.git("-c", "user.name=Different", "-c", "user.email=fixture@example.invalid", "commit-tree", f.git("rev-parse", commit+"^{tree}"), "-m", "hn artifact "+artifactVectorID)
			}
			f.git("update-ref", ref, changed)
			before = f.snapshot()
			_, err = loadArtifactHistoryFromSources(context.Background(), f.repo, []artifactSource{{ref, commit}})
			var failure *artifact.Error
			if !errors.As(err, &failure) || failure.Code != "concurrent_update" {
				t.Fatal("changed selected witness", err)
			}
			f.unchanged(before)
			f.git("update-ref", ref, commit)
			// A matching ref cannot excuse an unavailable selected object.
			object := filepath.Join(f.repo, "objects", commit[:2], commit[2:])
			raw, err := os.ReadFile(object)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(object); err != nil {
				t.Fatal(err)
			}
			before = f.snapshot()
			_, err = loadArtifactHistoryFromSources(context.Background(), f.repo, []artifactSource{{ref, commit}})
			if !errors.As(err, &failure) || failure.Code != "verification_failure" {
				t.Fatal("missing selected object", err)
			}
			f.unchanged(before)
			if err = os.WriteFile(object, raw, 0444); err != nil {
				t.Fatal(err)
			}
			bad := artifactRawCommit(t, f, artifactVector, strings.Repeat("A", 86), "", "hn artifact "+artifactVectorID, false)
			f.git("update-ref", ref, bad)
			before = f.snapshot()
			_, err = loadArtifactHistoryFromSources(context.Background(), f.repo, []artifactSource{{ref, bad}})
			if !errors.As(err, &failure) || failure.Code != "verification_failure" {
				t.Fatal("tampered selected signature", err)
			}
			f.unchanged(before)
			_, err = loadArtifactHistoryFromSources(context.Background(), ".", nil)
			if !errors.As(err, &failure) || failure.Code != "repository_error" {
				t.Fatal("relative explicit directory", err)
			}
			_, err = loadArtifactHistoryFromSources(context.Background(), filepath.Join(root, "missing.git"), nil)
			if !errors.As(err, &failure) || failure.Code != "repository_error" {
				t.Fatal("missing explicit repository became empty success", err)
			}
			// Cancellation must be recognized even for an empty source set.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = loadArtifactHistoryFromSources(ctx, f.repo, nil)
			if !errors.As(err, &failure) || failure.Code != "cancelled" {
				t.Fatal("cancelled explicit read", err)
			}
		})
	}
}

func TestArtifactStoreSourceChangesDuringRead(t *testing.T) {
	// The child Git wrapper makes an actual ref update after the initial witness
	// observation, then delegates every read to real Git. There is no reader hook.
	for _, action := range []string{"change", "remove", "add"} {
		t.Run(action, func(t *testing.T) {
			f := artifactCLI(t)
			ref := artifactFrozenRef(t, f)
			commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			f.git("update-ref", ref, commit)
			alternate := f.git("-c", "user.name=Changed", "-c", "user.email=fixture@example.invalid", "commit-tree", f.git("rev-parse", commit+"^{tree}"), "-m", "hn artifact "+artifactVectorID)
			actualGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			wrapper := t.TempDir()
			marker := filepath.Join(wrapper, "observed")
			target, value := ref, alternate
			if action == "add" {
				target = strings.Replace(ref, "refs/hn/", "refs/hn/remotes/new/", 1)
				value = commit
			}
			update := "update-ref '" + target + "' '" + value + "'"
			if action == "remove" {
				update = "update-ref -d '" + target + "'"
			}
			script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = ls-tree ] && [ ! -e '" + marker + "' ]; then\n  : > '" + marker + "'\n  '" + actualGit + "' --git-dir='" + filepath.Join(f.repo, ".git") + "' " + update + " || exit 93\n fi\ndone\nexec '" + actualGit + "' \"$@\"\n"
			if err = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot()
			f.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH")}
			result := f.call("", "concurrent_update", "artifact", "type", "show", artifactVectorID, "--json")
			if action == "add" && result["error"].(map[string]any)["message"] != "artifact sources changed while reading" {
				t.Fatal("ordinary reader lost full selected-set addition check", result)
			}
			if _, err = os.Stat(marker); err != nil {
				t.Fatal("mutation control never ran", err)
			}
			// Restore only the deliberately changed witness, then compare every object,
			// private marker and ref to the pre-read snapshot.
			if action == "add" {
				f.git("update-ref", "-d", target)
			} else {
				f.git("update-ref", ref, commit)
			}
			f.unchanged(before)
		})
	}
}
