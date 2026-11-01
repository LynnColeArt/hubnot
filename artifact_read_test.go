package main

import (
	"bufio"
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
	"strconv"
	"strings"
	"syscall"
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

// These controls exercise the actual public reader. Git-only graph construction
// and a bounded Git peer isolate traversal from the subsequent signed verifier.
func TestArtifactStoreCountPreflightGraphCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) { artifactCountGraphCLI(t, format) })
	}
}
func artifactCountGraphCLI(t *testing.T, format string) {
	f := artifactCLIFormat(t, format)
	config := wp25FixtureConfig(t, f.home)
	f.overrides = map[string]string{"XDG_CONFIG_HOME": config}
	ref := artifactFrozenRef(t, f)
	root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	f.git("update-ref", ref, root)
	check := func(t *testing.T, code, message string) {
		t.Helper()
		f.t = t
		before := f.snapshot()
		result := wp25ArtifactCall(f, "", code, "artifact", "type", "show", artifactVectorID, "--json")
		if code != "" {
			problem := result["error"].(map[string]any)
			if len(problem) != 5 || problem["origin"] != "artifact" || problem["recovery"] != "do_not_retry" || problem["operation"] != nil || problem["message"] != message {
				t.Fatal("graph refusal identity", result)
			}
		} else {
			items := result["data"].(map[string]any)["items"].([]any)
			if len(items) != 1 || items[0].(map[string]any)["id"] != artifactVectorID {
				t.Fatal("graph traversal lost signed public identity", result)
			}
		}
		wp25Unchanged(f, before)
	}
	check(t, "", "")
	// A real optional commit-graph is neither count authority nor required
	// storage. The command-local raw-parent mode works with it present and with
	// an independently corrupted header, without changing repository config.
	f.git("commit-graph", "write", "--reachable")
	graphPath := filepath.Join(f.repo, ".git", "objects", "info", "commit-graph")
	graph, err := os.ReadFile(graphPath)
	if err != nil || len(graph) < 8 || string(graph[:4]) != "CGPH" {
		t.Fatal("real Git commit-graph control unavailable", err)
	}
	check(t, "", "")
	f.git("-c", "core.commitGraph=true", "commit-graph", "verify")
	corrupt := append([]byte(nil), graph...)
	copy(corrupt[:4], "BAD!")
	// Git creates this owned fixture cache read-only. Make only the test's
	// temporary copy writable before introducing the deliberate corruption.
	if err := os.Chmod(graphPath, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(graphPath, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	wp25Record(t, map[string]any{"control": "count-preflight-real-commit-graph", "path": graphPath, "valid_sha256": wp25RecordCountHash(graph), "corrupt_sha256": wp25RecordCountHash(corrupt), "corruption": "replace CGPH header with BAD!; remaining bytes exact", "traversal": "raw cat-file commit bytes; commit-graph is not count authority"})
	verify := exec.Command("git", "-C", f.repo, "-c", "core.commitGraph=true", "commit-graph", "verify")
	verify.Env = operationalTestEnvironmentWith(f.home, f.overrides)
	verifyRaw, verifyErr := verify.CombinedOutput()
	wp25Record(t, map[string]any{"control": "count-preflight-corrupt-graph-detector", "argv": verify.Args, "stdout_stderr_raw": string(verifyRaw), "process_error": fmt.Sprint(verifyErr)})
	if verifyErr == nil {
		t.Fatal("commit-graph corruption control was not real", string(verifyRaw))
	}
	check(t, "", "")
	if err := os.Remove(graphPath); err != nil {
		t.Fatal(err)
	}
	// Duplicate immutable tips in distinct accepted refs count once.
	alias := strings.Replace(ref, "refs/hn/", "refs/hn/remotes/overlap/", 1)
	f.git("update-ref", alias, root)
	check(t, "", "")
	f.git("update-ref", "-d", alias)
	child := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, root, "hn artifact "+artifactVectorID, false)
	for _, view := range []string{"shallow", filepath.Join("info", "grafts")} {
		t.Run(view, func(t *testing.T) {
			f.git("update-ref", ref, child)
			path := filepath.Join(f.repo, ".git", view)
			// Git sees a root; the original raw-parent reader must still see the
			// forbidden parent of this signed genesis and refuse it.
			if err := os.WriteFile(path, []byte(child+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			defer os.Remove(path)
			check(t, "invalid_history", "actor stream predecessor mismatch")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
		f.t = t
	}
	f.git("update-ref", ref, root)
	other := f.git("-c", "user.name=Other", "-c", "user.email=fixture@example.invalid", "commit-tree", f.git("rev-parse", root+"^{tree}"), "-m", "hn artifact "+artifactVectorID)
	merge := f.git("-c", "user.name=Merge", "-c", "user.email=fixture@example.invalid", "commit-tree", f.git("rev-parse", root+"^{tree}"), "-p", root, "-p", other, "-m", "hn artifact "+artifactVectorID)
	f.git("update-ref", ref, merge)
	check(t, "actor_stream_conflict", "artifact stream has multiple Git parents")
	f.git("update-ref", ref, root)
	check(t, "", "")
}

// The peer is supplemental, source-assisted protocol evidence. Its config
// contains only independently obtained public commit bytes and selected fault
// responses. All ordinary Git calls and all signed/tree verification remain
// real Git. Real-Git/public byte controls below run separately, without a peer.
type artifactCountBatchPeerConfig struct {
	Mode        string            `json:"mode"`
	Ledger      string            `json:"ledger"`
	Objects     map[string][]byte `json:"objects"`
	Sizes       map[string]int    `json:"sizes,omitempty"`
	ObjectFiles map[string]string `json:"object_files,omitempty"`
	Types       map[string]string `json:"types,omitempty"`
	ActualGit   string            `json:"actual_git,omitempty"`
	SessionRoot string            `json:"session_root,omitempty"`
	Sessions    string            `json:"sessions,omitempty"`
	Target      int               `json:"target,omitempty"`
}

func TestArtifactStoreCountPreflightBatchPeer(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			raw, err := os.ReadFile(os.Args[i+1])
			if err != nil {
				os.Exit(72)
			}
			var cfg artifactCountBatchPeerConfig
			if json.Unmarshal(raw, &cfg) != nil || cfg.Ledger == "" {
				os.Exit(73)
			}
			if cfg.SessionRoot != "" {
				index := 1
				for ; index <= 32; index++ {
					err = os.Mkdir(filepath.Join(cfg.SessionRoot, strconv.Itoa(index)), 0700)
					if err == nil {
						break
					}
					if !os.IsExist(err) {
						os.Exit(85)
					}
				}
				if index > 32 {
					os.Exit(86)
				}
				role := "real-git-forward"
				if index == cfg.Target {
					role = "target-peer"
				}
				event, _ := json.Marshal(map[string]any{"session": index, "role": role, "pid": os.Getpid(), "ppid": os.Getppid(), "argv": os.Args[i+2:], "actual_git": cfg.ActualGit})
				file, e := os.OpenFile(cfg.Sessions, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				if e != nil {
					os.Exit(87)
				}
				_, e = file.Write(append(event, '\n'))
				closeErr := file.Close()
				if e != nil || closeErr != nil {
					os.Exit(88)
				}
				if index != cfg.Target {
					if syscall.Exec(cfg.ActualGit, append([]string{cfg.ActualGit}, os.Args[i+2:]...), os.Environ()) != nil {
						os.Exit(89)
					}
				}
			}
			os.Exit(artifactCountBatchPeer(cfg, os.Args[i+2:]))
		}
	}
}
func artifactCountBatchPeer(cfg artifactCountBatchPeerConfig, argv []string) int {
	log := func(event map[string]any) {
		raw, _ := json.Marshal(event)
		file, err := os.OpenFile(cfg.Ledger, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(74)
		}
		_, err = file.Write(append(raw, '\n'))
		file.Close()
		if err != nil {
			os.Exit(75)
		}
	}
	log(map[string]any{"event": "start", "pid": os.Getpid(), "argv": argv, "source_assisted": true})
	command, unbuffered, explicit := false, false, false
	for _, arg := range argv {
		if arg == "cat-file" {
			command = true
		}
		if arg == "--batch-command" {
			unbuffered = true
		}
		if arg == "--buffer" {
			return 76
		}
		if strings.HasPrefix(arg, "--git-dir=") {
			explicit = true
		}
	}
	if !command || !unbuffered || !explicit || os.Getenv("GIT_NO_LAZY_FETCH") != "1" || os.Getenv("GIT_NO_REPLACE_OBJECTS") != "1" {
		log(map[string]any{"event": "unsafe-argv-or-environment"})
		return 77
	}
	if cfg.Mode == "failure" {
		return 71
	}
	if cfg.Mode == "empty" {
		return 0
	}
	if cfg.Mode == "stderr" {
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("S"), 8193))
		return 71
	}
	if cfg.Mode == "stderr-at" {
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("S"), 8192))
	}
	reader := bufio.NewReaderSize(os.Stdin, 512)
	for {
		line, err := reader.ReadSlice('\n')
		if err == io.EOF && len(line) == 0 {
			log(map[string]any{"event": "stdin-eof"})
			if cfg.Mode == "trailing-eof" {
				fmt.Fprint(os.Stdout, "unexpected final output\n")
			}
			if cfg.Mode == "stall-eof" {
				time.Sleep(60 * time.Second)
			}
			if cfg.Mode == "exit-error" {
				return 71
			}
			return 0
		}
		if err != nil || len(line) > 80 {
			log(map[string]any{"event": "invalid-input", "raw": string(line), "error": fmt.Sprint(err)})
			return 78
		}
		fields := strings.Fields(string(line))
		if len(fields) != 2 || (fields[0] != "info" && fields[0] != "contents") {
			return 79
		}
		action, oid := fields[0], fields[1]
		log(map[string]any{"event": "request", "action": action, "oid": oid, "raw": string(line)})
		body, exists := cfg.Objects[oid]
		if cfg.Mode == "hash-with-invalid-parent" {
			// Correlated headers announce this injected body's exact length. The
			// selected real OID deliberately remains unchanged, so hash verification
			// must win over the independently malformed raw parent.
			separator := bytes.Index(body, []byte("\n\n"))
			if separator < 0 {
				return 84
			}
			changed := append([]byte{}, body[:separator+1]...)
			changed = append(changed, []byte("parent not-an-oid\n")...)
			body = append(changed, body[separator+1:]...)
			log(map[string]any{"event": "mixed-hash-parent-fault", "oid": oid, "body_raw": string(body), "body_sha256": wp25RecordCountHash(body)})
		}
		n := len(body)
		if size, ok := cfg.Sizes[oid]; ok {
			n = size
		}
		kind := cfg.Types[oid]
		if kind == "" {
			kind = "commit"
		}
		header := fmt.Sprintf("%s %s %d\n", oid, kind, n)
		if action == "info" {
			switch cfg.Mode {
			case "stall-info":
				time.Sleep(60 * time.Second)
			case "info-missing":
				header = oid + " missing\n"
			case "info-type":
				header = fmt.Sprintf("%s blob %d\n", oid, n)
			case "info-negative":
				header = oid + " commit -1\n"
			case "info-noncanonical":
				header = fmt.Sprintf("%s %s 0%d\n", oid, kind, n)
			case "info-nonnumeric":
				header = oid + " commit not-a-size\n"
			case "info-overflow":
				header = oid + " commit 9223372036854775808\n"
			case "info-oid":
				header = strings.Repeat("0", len(oid)) + " commit 1\n"
			case "info-extra":
				header = strings.TrimSuffix(header, "\n") + " extra\n"
			case "info-short":
				header = strings.TrimSuffix(header, "\n")
				fmt.Fprint(os.Stdout, header)
				return 0
			case "info-long":
				fmt.Fprint(os.Stdout, strings.Repeat("H", 257)+"\n")
				return 0
			case "info-error":
				header = "fatal: deliberate peer error\n"
			}
			if !exists {
				header = oid + " missing\n"
			}
			log(map[string]any{"event": "header", "action": action, "raw": header})
			fmt.Fprint(os.Stdout, header)
			if cfg.Mode == "exit-after-info" {
				return 71
			}
			continue
		}
		switch cfg.Mode {
		case "contents-oid":
			header = strings.Repeat("0", len(oid)) + " commit " + strconv.Itoa(n) + "\n"
		case "contents-type":
			header = fmt.Sprintf("%s blob %d\n", oid, n)
		case "contents-size":
			header = fmt.Sprintf("%s commit %d\n", oid, n+1)
		case "contents-negative":
			header = oid + " commit -1\n"
		case "contents-extra":
			header = strings.TrimSuffix(header, "\n") + " extra\n"
		case "contents-long":
			header = strings.Repeat("H", 257) + "\n"
		case "contents-short-header":
			fmt.Fprint(os.Stdout, strings.TrimSuffix(header, "\n"))
			return 0
		}
		log(map[string]any{"event": "header", "action": action, "raw": header})
		fmt.Fprint(os.Stdout, header)
		if cfg.Mode == "stall-body" {
			time.Sleep(60 * time.Second)
		}
		if path := cfg.ObjectFiles[oid]; path != "" {
			// Fixture bytes are read only after the real host separately authorizes
			// contents. The peer never preloads aggregate bodies during info.
			file, err := os.Open(path)
			if err != nil {
				return 81
			}
			body, err = io.ReadAll(io.LimitReader(file, 9437185))
			closeErr := file.Close()
			if err != nil || closeErr != nil || len(body) > 9437184 {
				return 82
			}
			log(map[string]any{"event": "fixture-body-open", "oid": oid, "bytes": len(body), "path": path})
		} else if n > len(body) {
			// Withheld over-limit fixtures deliberately cannot supply a body. A
			// faulty host issuing contents gets header then EOF immediately, which
			// makes the cap-removal control fail causally rather than waiting30s.
			log(map[string]any{"event": "withheld-body-requested", "oid": oid, "declared": n, "available": len(body)})
			return 83
		}
		if cfg.Mode == "short-body" {
			if len(body) > 0 {
				os.Stdout.Write(body[:len(body)-1])
			}
			return 0
		}
		if cfg.Mode == "hash" {
			body = append([]byte{}, body...)
			if len(body) > 0 {
				body[len(body)-1] ^= 1
			}
		}
		if _, err := os.Stdout.Write(body); err != nil {
			return 80
		}
		log(map[string]any{"event": "body", "oid": oid, "bytes": len(body), "sha256": wp25RecordCountHash(body)})
		if cfg.Mode == "missing-lf" {
			return 0
		}
		if cfg.Mode == "wrong-lf" {
			os.Stdout.Write([]byte("X"))
			return 0
		}
		os.Stdout.Write([]byte("\n"))
		if cfg.Mode == "trailing" {
			fmt.Fprint(os.Stdout, "unexpected trailing output\n")
		}
	}
}
func artifactCountBatchRaw(t *testing.T, f *artifactCLIFixture, oid string) []byte {
	t.Helper()
	cmd := exec.Command("git", "-C", f.repo, "cat-file", "commit", oid)
	cmd.Env = operationalTestEnvironmentWith(f.home, f.overrides)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func artifactCountBatchPeerInstall(t *testing.T, f *artifactCLIFixture, mode string, objects map[string][]byte, sizes map[string]int, bodyFiles ...map[string]string) (string, string, map[string]string) {
	t.Helper()
	actual, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.jsonl")
	noFetch := filepath.Join(dir, "forbidden-fetch")
	cfg := artifactCountBatchPeerConfig{Mode: mode, Ledger: ledger, Objects: objects, Sizes: sizes, ActualGit: actual, Target: 1, SessionRoot: filepath.Join(dir, "sessions"), Sessions: filepath.Join(dir, "sessions.jsonl")}
	if err := os.Mkdir(cfg.SessionRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if len(bodyFiles) > 0 {
		cfg.ObjectFiles = bodyFiles[0]
	}
	raw, _ := json.Marshal(cfg)
	config := filepath.Join(dir, "peer.json")
	if err := os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = fetch ]; then printf forbidden > " + quote(noFetch) + "; fi\n if [ \"$arg\" = --batch-command ]; then exec " + quote(self) + " '-test.run=^TestArtifactStoreCountPreflightBatchPeer$' -- " + quote(config) + " \"$@\"; fi\ndone\nexec " + quote(actual) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	overrides := map[string]string{}
	for k, v := range f.overrides {
		overrides[k] = v
	}
	overrides["PATH"] = dir + string(os.PathListSeparator) + os.Getenv("PATH")
	wp25Record(t, map[string]any{"control": "count-preflight-batch-peer-config", "source_assisted": true, "mode": mode, "objects": sizes, "public_body_count": len(objects), "config_sha256": wp25RecordCountHash(raw), "script_raw": script, "real_git": actual, "compiled_peer": self})
	return ledger, noFetch, overrides
}
func artifactCountBatchLedger(t *testing.T, path, noFetch string, forwarded ...int) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("batch peer never started", err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	events := []map[string]any{}
	starts := 0
	pending := map[string]bool{}
	for _, line := range lines {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			t.Fatal("peer ledger invalid JSON", string(raw))
		}
		events = append(events, event)
		if event["event"] == "start" {
			starts++
			pid := strconv.Itoa(int(event["pid"].(float64)))
			ps := exec.Command("ps", "-p", pid, "-o", "pid=,ppid=,comm=")
			out, err := ps.CombinedOutput()
			wp25Record(t, map[string]any{"control": "count-preflight-batch-reaping", "pid": pid, "ps_raw": string(out), "ps_error": fmt.Sprint(err)})
			if err == nil || len(bytes.TrimSpace(out)) != 0 {
				t.Fatal("batch peer survived public return", pid, string(out))
			}
		}
		if event["event"] == "request" {
			oid := event["oid"].(string)
			if event["action"] == "info" {
				if pending[oid] {
					t.Fatal("same unique object metadata requested twice", oid)
				}
				pending[oid] = true
			} else if !pending[oid] {
				t.Fatal("contents request preceded object info", oid)
			}
		}
	}
	if starts != 1 {
		t.Fatal("nonempty traversal did not have exactly one scoped batch child", starts)
	}
	expectedForwarded := 0
	if len(forwarded) != 0 {
		expectedForwarded = forwarded[0]
	}
	sessionRaw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "sessions.jsonl"))
	if err != nil {
		t.Fatal("missing batch invocation ledger", err)
	}
	sessions := bytes.Split(bytes.TrimSpace(sessionRaw), []byte{'\n'})
	if len(sessions) != 1+expectedForwarded {
		t.Fatal("omitted/duplicated count or full-reader invocation", len(sessions), string(sessionRaw))
	}
	for index, line := range sessions {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil || event["session"] != float64(index+1) {
			t.Fatal("invalid ordered batch invocation", string(line))
		}
		expectedRole := "target-peer"
		if index != 0 {
			expectedRole = "real-git-forward"
		}
		if event["role"] != expectedRole {
			t.Fatal("count/full-reader routing changed", event)
		}
		pid := strconv.Itoa(int(event["pid"].(float64)))
		out, err := exec.Command("ps", "-p", pid, "-o", "pid=,ppid=,comm=").CombinedOutput()
		if err == nil || len(bytes.TrimSpace(out)) != 0 {
			t.Fatal("forwarded or count child survived public return", pid, string(out))
		}
	}
	wp25Record(t, map[string]any{"control": "count-preflight-all-session-ledger", "source_assisted": true, "raw": string(sessionRaw), "expected_count_sessions": 1, "expected_real_git_full_reader_sessions": expectedForwarded, "attribution": "explicit traversal order plus independent full-reader protocol/role controls; additional or omitted traversal fails"})
	if content, err := os.ReadFile(noFetch); !os.IsNotExist(err) {
		t.Fatal("batch caller fetched", err, string(content))
	}
	wp25Record(t, map[string]any{"control": "count-preflight-batch-peer-ledger", "source_assisted": true, "raw": string(raw), "events": events, "ledger_sha256": wp25RecordCountHash(raw)})
	return events
}

// Select a numbered process only in this fixture's Git shim. All other batch
// children exec the original Git at the same PID; no candidate flag or runtime
// capability marker is involved. Separate ledgers retain every forwarded child.
func artifactFullBatchPeerInstall(t *testing.T, f *artifactCLIFixture, mode string, objects map[string][]byte, kinds map[string]string, target ...int) (string, string, string, map[string]string) {
	t.Helper()
	ledger, noFetch, overrides := artifactCountBatchPeerInstall(t, f, mode, objects, nil)
	dir := strings.Split(overrides["PATH"], string(os.PathListSeparator))[0]
	path := filepath.Join(dir, "peer.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg artifactCountBatchPeerConfig
	if json.Unmarshal(raw, &cfg) != nil {
		t.Fatal("invalid peer config")
	}
	cfg.ActualGit, err = exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Types, cfg.Target = kinds, 2
	if len(target) != 0 {
		cfg.Target = target[0]
	}
	cfg.SessionRoot, cfg.Sessions = filepath.Join(dir, "sessions"), filepath.Join(dir, "sessions.jsonl")
	raw, err = json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	wp25Record(t, map[string]any{"control": "full-reader-peer-config", "source_assisted": true, "target_session": cfg.Target, "attribution": "explicit named owner and raw invocation order; public controls identify count separately from full reader; private one-shot has no count traversal", "types": kinds, "config_raw": string(raw), "config_sha256": wp25RecordCountHash(raw)})
	return ledger, noFetch, cfg.Sessions, overrides
}

func TestArtifactStoreFullReaderProtocolCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := artifactCLIFormat(t, format)
			f.overrides = map[string]string{"XDG_CONFIG_HOME": wp25FixtureConfig(t, f.home)}
			base := f.overrides
			ref := artifactFrozenRef(t, f)
			root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			f.git("update-ref", ref, root)
			tree := f.git("rev-parse", root+"^{tree}")
			kinds := map[string]string{root: "commit", tree: "tree"}
			for _, line := range strings.Split(f.git("ls-tree", tree), "\n") {
				fields := strings.Fields(line)
				if len(fields) != 4 || fields[1] != "blob" {
					t.Fatal("independent selected tree", line)
				}
				kinds[fields[2]] = "blob"
			}
			objects := map[string][]byte{}
			for oid, kind := range kinds {
				cmd := exec.Command("git", "-C", f.repo, "cat-file", kind, oid)
				cmd.Env = operationalTestEnvironmentWith(f.home, f.overrides)
				var err error
				objects[oid], err = cmd.Output()
				if err != nil {
					t.Fatal(err)
				}
			}
			artifactCountBatchCheck(t, f, "", "")
			for _, mode := range []string{"pass", "stderr-at", "exit-error", "trailing", "trailing-eof", "info-missing", "info-type", "info-negative", "info-noncanonical", "info-nonnumeric", "info-overflow", "info-oid", "info-extra", "info-short", "info-long", "info-error", "exit-after-info", "contents-oid", "contents-type", "contents-size", "contents-negative", "contents-extra", "contents-long", "contents-short-header", "hash", "short-body", "missing-lf", "wrong-lf", "stderr"} {
				t.Run(mode, func(t *testing.T) {
					f.t = t
					ledger, noFetch, sessions, overrides := artifactFullBatchPeerInstall(t, f, mode, objects, kinds)
					f.overrides = overrides
					defer func() { f.overrides = base }()
					code, message := "verification_failure", "accepted artifact object unavailable; no fetch attempted"
					if mode == "pass" || mode == "stderr-at" {
						code, message = "", ""
					} else if mode == "info-negative" || mode == "info-noncanonical" || mode == "info-nonnumeric" || mode == "info-overflow" {
						message = "invalid Git object size"
					} else if mode == "info-missing" || mode == "info-type" {
						message = "accepted artifact object is missing or has wrong type"
					} else if mode == "hash" {
						message = "accepted Git object hash mismatch"
					}
					artifactCountBatchCheck(t, f, code, message)
					raw, err := os.ReadFile(sessions)
					if err != nil || bytes.Count(raw, []byte{'\n'}) != 2 {
						t.Fatal("missing/extra count or full-reader session", err, string(raw))
					}
					peer, err := os.ReadFile(ledger)
					if err != nil {
						t.Fatal("targeted full-reader peer never executed", err)
					}
					if mode == "pass" || mode == "stderr-at" {
						info, contents := map[string]int{}, map[string]int{}
						pending, eof := "", 0
						for _, line := range bytes.Split(bytes.TrimSpace(peer), []byte{'\n'}) {
							var event map[string]any
							if json.Unmarshal(line, &event) != nil {
								t.Fatal("invalid full-reader protocol ledger")
							}
							if event["event"] == "stdin-eof" {
								eof++
							}
							if event["event"] != "request" {
								continue
							}
							oid := event["oid"].(string)
							if _, known := kinds[oid]; !known {
								t.Fatal("full-reader selected an unexpected object", event)
							}
							if event["action"] == "info" {
								if pending != "" {
									t.Fatal("full-reader pipelined metadata ahead of contents", event)
								}
								pending = oid
								info[oid]++
							} else {
								if pending != oid {
									t.Fatal("full-reader contents preceded exact metadata", event)
								}
								pending = ""
								contents[oid]++
							}
						}
						if pending != "" || eof != 1 || len(kinds) != 4 {
							t.Fatal("full-reader phase incomplete", pending, eof, kinds)
						}
						for oid, kind := range kinds {
							if info[oid] != 1 || contents[oid] != 1 {
								t.Fatal("second session was not the exact commit/tree/both-blob full reader", oid, kind, info, contents)
							}
						}
					}
					for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
						var event map[string]any
						if json.Unmarshal(line, &event) != nil {
							t.Fatal("invalid session ledger")
						}
						pid := strconv.Itoa(int(event["pid"].(float64)))
						out, err := exec.Command("ps", "-p", pid, "-o", "pid=,ppid=,comm=").CombinedOutput()
						if err == nil || len(bytes.TrimSpace(out)) != 0 {
							t.Fatal("full-reader/count child survived public return", pid, string(out))
						}
					}
					if data, err := os.ReadFile(noFetch); !os.IsNotExist(err) {
						t.Fatal("full-reader fault fetched", err, string(data))
					}
					wp25Record(t, map[string]any{"control": "full-reader-peer-ledgers", "source_assisted": true, "mode": mode, "session_raw": string(raw), "peer_raw": string(peer)})
				})
				f.t = t
			}
		})
	}
}

func artifactIOReadObjects(t *testing.T, f *artifactCLIFixture, commits ...string) (map[string][]byte, map[string]string) {
	t.Helper()
	kinds := map[string]string{}
	for _, commit := range commits {
		kinds[commit] = "commit"
		tree := f.git("rev-parse", commit+"^{tree}")
		kinds[tree] = "tree"
		for _, line := range strings.Split(f.git("ls-tree", tree), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 4 || fields[1] != "blob" {
				t.Fatal("independent selected tree", line)
			}
			kinds[fields[2]] = "blob"
		}
	}
	objects := map[string][]byte{}
	for oid, kind := range kinds {
		cmd := exec.Command("git", "-C", f.repo, "cat-file", kind, oid)
		cmd.Env = operationalTestEnvironmentWith(f.home, f.overrides)
		var err error
		objects[oid], err = cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
	}
	return objects, kinds
}

func artifactIOReadEvidence(t *testing.T, ledger, sessions, noFetch string, invocations int, expected map[string]string, complete bool) {
	t.Helper()
	raw, err := os.ReadFile(sessions)
	if err != nil || bytes.Count(raw, []byte{'\n'}) != invocations {
		t.Fatal("owner omitted/duplicated a raw reader", err, string(raw), invocations)
	}
	for index, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil || event["session"] != float64(index+1) {
			t.Fatal("invalid reader invocation order", string(line))
		}
		pid := strconv.Itoa(int(event["pid"].(float64)))
		out, err := exec.Command("ps", "-p", pid, "-o", "pid=,ppid=,comm=").CombinedOutput()
		if err == nil || len(bytes.TrimSpace(out)) != 0 {
			t.Fatal("reader child survived owner return", pid, string(out))
		}
	}
	peer, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal("selected owner never reached protocol peer", err)
	}
	infos, contents := map[string]int{}, map[string]int{}
	pending, eof, starts := "", 0, 0
	for _, line := range bytes.Split(bytes.TrimSpace(peer), []byte{'\n'}) {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			t.Fatal("invalid owner protocol ledger")
		}
		switch event["event"] {
		case "start":
			starts++
		case "stdin-eof":
			eof++
		case "request":
			oid := event["oid"].(string)
			if _, ok := expected[oid]; !ok {
				t.Fatal("owner read outside its expected exact objects", event)
			}
			if event["action"] == "info" {
				if pending != "" {
					t.Fatal("owner pipelined raw contents", event)
				}
				pending = oid
				infos[oid]++
			} else {
				if pending != oid {
					t.Fatal("owner contents preceded exact metadata", event)
				}
				pending = ""
				contents[oid]++
			}
		}
	}
	if starts != 1 {
		t.Fatal("owner did not have one selected peer", starts)
	}
	if complete {
		if pending != "" || eof != 1 {
			t.Fatal("owner escaped before complete EOF", pending, eof)
		}
		for oid, kind := range expected {
			if infos[oid] != 1 || contents[oid] != 1 {
				t.Fatal("owner did not consume exact commit/tree/blob role", oid, kind, infos, contents)
			}
		}
	}
	if body, err := os.ReadFile(noFetch); !os.IsNotExist(err) {
		t.Fatal("raw owner attempted hidden fetch", err, string(body))
	}
	wp25Record(t, map[string]any{"control": "full-reader-owner-ledgers", "source_assisted": true, "sessions_raw": string(raw), "peer_raw": string(peer), "expected_object_types": expected, "complete_frames": complete})
}

func TestArtifactFullReaderOwnerFinalizationCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, owner := range []string{"closure", "export", "inventory", "copy-reader", "candidate"} {
			for _, mode := range []string{"pass", "exit-error", "trailing-eof", "info-missing"} {
				t.Run(format+"/"+owner+"/"+mode, func(t *testing.T) {
					f := artifactCLIFormat(t, format)
					f.overrides = map[string]string{"XDG_CONFIG_HOME": wp25FixtureConfig(t, f.home)}
					base := f.overrides
					ref := artifactFrozenRef(t, f)
					root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
					f.git("update-ref", ref, root)
					objects, kinds := artifactIOReadObjects(t, f, root)
					expected := kinds
					target, all := 3, 3
					args := []string{"artifact", "inspect", "--id", artifactVectorID, "--json"}
					output, remote, remoteBefore := "", "", ""
					message := "accepted artifact object unavailable; no fetch attempted"
					switch owner {
					case "export":
						target, all = 4, 4
						output = filepath.Join(t.TempDir(), "export.json")
						args = []string{"artifact", "export", "--id", artifactVectorID, "--output", output, "--json"}
					case "inventory", "copy-reader":
						target, all = 4, 5
						if owner == "copy-reader" {
							target = 5
						}
						message = "selected artifact object unavailable"
						remote = filepath.Join(t.TempDir(), "public.git")
						f.git("init", "--bare", "--quiet", "--object-format="+format, remote)
						f.git("remote", "add", "public", remote)
						remoteBefore = f.git("--git-dir="+remote, "for-each-ref", "--format=%(refname) %(objectname)")
						args = []string{"artifact", "push", "--remote", "public", "--id", artifactVectorID, "--json"}
					case "candidate":
						// The real writer creates a signed candidate and immutable intent,
						// then a real failed CAS stage leaves it unpublished for inspection.
						f.git("update-ref", "-d", ref)
						wp25ArtifactCall(f, artifactStoreType, "", "artifact", "type", "publish", "--input", "-", "--actor", f.actor, "--operation", "io-base", "--json")
						actual, err := exec.LookPath("git")
						if err != nil {
							t.Fatal(err)
						}
						wrapper := t.TempDir()
						marker := filepath.Join(wrapper, "cas-refused")
						quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
						script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = update-ref ]; then printf dispatched > " + quote(marker) + "; exit 86; fi\ndone\nexec " + quote(actual) + " \"$@\"\n"
						if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
							t.Fatal(err)
						}
						f.overrides = map[string]string{"XDG_CONFIG_HOME": base["XDG_CONFIG_HOME"], "PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH")}
						refusal := wp25ArtifactCall(f, artifactStoreType, "concurrent_update", "artifact", "type", "publish", "--input", "-", "--actor", f.actor, "--operation", "io-candidate", "--json")
						artifactErrorShape(t, refusal, "artifact", "concurrent_update", "retry_identical", nil)
						observed, err := os.ReadFile(marker)
						if err != nil || string(observed) != "dispatched" {
							t.Fatal("candidate fixture failed to reach the real CAS refusal", err, string(observed))
						}
						f.overrides = base
						intent := operationIntent(t, f, "io-candidate")
						candidate := intent["candidate"].(map[string]any)
						commit := candidate["commit"].(string)
						objects, kinds = artifactIOReadObjects(t, f, commit)
						expected = kinds
						args = []string{"artifact", "operation", "--actor", f.actor, "--operation", "io-candidate", "--type", "example.test/note", "--json"}
						wp25Record(t, map[string]any{"control": "full-reader-candidate-public-setup", "source_assisted_construction": true, "actor": f.actor, "operation": "io-candidate", "candidate_commit": commit, "candidate_id": candidate["id"], "intent_path": operationIntentPath(f, "io-candidate"), "intent_sha256": wp25Digest(t, operationIntentPath(f, "io-candidate")), "cas_script_raw": script, "cas_marker_raw": string(observed)})
					}
					if mode == "info-missing" {
						switch owner {
						case "export":
							message = "unsupported exchange object type"
						case "closure", "candidate":
							message = "accepted artifact object is missing or has wrong type"
						}
					}
					ledger, noFetch, sessions, overrides := artifactFullBatchPeerInstall(t, f, mode, objects, kinds, target)
					f.overrides = overrides
					before := f.snapshot()
					code := "verification_failure"
					if mode == "pass" {
						code = ""
					}
					response := wp25ArtifactCall(f, "", code, args...)
					f.unchanged(before)
					if code != "" {
						artifactErrorShape(t, response, "artifact", code, "do_not_retry", nil)
						if response["data"] != nil || response["error"].(map[string]any)["message"] != message {
							t.Fatal("owner changed sole-fault diagnostic or exposed success", response)
						}
						all = target
						if output != "" {
							if _, err := os.Lstat(output); !os.IsNotExist(err) {
								t.Fatal("failed export published output", err)
							}
						}
						if remote != "" && remoteBefore != f.git("--git-dir="+remote, "for-each-ref", "--format=%(refname) %(objectname)") {
							t.Fatal("reader refusal promoted remote refs")
						}
					} else {
						data := response["data"].(map[string]any)
						switch owner {
						case "closure":
							items := data["items"].([]any)
							if len(items) != 1 || items[0].(map[string]any)["id"] != artifactVectorID {
								t.Fatal("closure owner lost exact signed ID", data)
							}
						case "export":
							raw, err := os.ReadFile(output)
							var table map[string]any
							if err != nil || json.Unmarshal(raw, &table) != nil || len(table["objects"].([]any)) != 4 || data["records"] != float64(1) {
								t.Fatal("export owner lost exact inventory", err, data)
							}
						case "inventory", "copy-reader":
							if data["records"] != float64(1) || f.git("--git-dir="+remote, "rev-parse", ref) != root {
								t.Fatal("transport owner lost exact source/record", data)
							}
						case "candidate":
							if data["status"] != "uncertain" || data["reason"] != "private_intent_pending" || data["actor"] != f.actor || data["operation"] != "io-candidate" || data["event_id"] != nil || data["result"] != nil {
								t.Fatal("candidate owner altered operation identity/status", data)
							}
						}
					}
					artifactIOReadEvidence(t, ledger, sessions, noFetch, all, expected, mode != "info-missing")
					if mode == "info-missing" {
						raw, err := os.ReadFile(ledger)
						if err != nil || bytes.Contains(raw, []byte(`"action":"contents"`)) {
							t.Fatal("missing metadata authorized body contents", err, string(raw))
						}
					}
				})
			}
		}
	}
}

func TestArtifactFullReaderCancellationCLI(t *testing.T) {
	for _, mode := range []string{"stall-info", "stall-body", "stall-eof"} {
		t.Run(mode, func(t *testing.T) {
			f := wp25ArtifactCLI(t)
			ref := artifactFrozenRef(t, f)
			root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			f.git("update-ref", ref, root)
			objects, kinds := artifactIOReadObjects(t, f, root)
			base := f.overrides
			ledger, noFetch, sessions, overrides := artifactFullBatchPeerInstall(t, f, mode, objects, kinds)
			f.overrides = overrides
			started := time.Now()
			artifactCountBatchCheck(t, f, "cancelled", "artifact read cancelled")
			elapsed := time.Since(started)
			if elapsed < 29*time.Second || elapsed > 45*time.Second {
				t.Fatal("full-reader cancellation did not retain normal30s stage", elapsed)
			}
			artifactIOReadEvidence(t, ledger, sessions, noFetch, 2, kinds, false)
			wp25Record(t, map[string]any{"control": "full-reader-normal-stage-cancellation", "source_assisted_fault_peer": true, "candidate_observation": true, "mode": mode, "elapsed_nanoseconds": elapsed.Nanoseconds(), "product_budget_seconds": 30})
			f.overrides = base
			artifactCountBatchCheck(t, f, "", "")
		})
	}
}

func TestArtifactFullReaderOneShotLifecycle(t *testing.T) {
	for _, method := range []string{"object", "commit", "managed-repeat"} {
		for _, mode := range []string{"pass", "exit-error", "trailing-eof"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				f := wp25ArtifactCLI(t)
				root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
				objects, kinds := artifactIOReadObjects(t, f, root)
				oid := root
				expected := kinds
				if method != "commit" {
					for candidate, kind := range kinds {
						if kind == "blob" {
							oid = candidate
							break
						}
					}
					expected = map[string]string{oid: "blob"}
				}
				ledger, noFetch, sessions, overrides := artifactFullBatchPeerInstall(t, f, mode, objects, kinds, 1)
				t.Setenv("PATH", overrides["PATH"])
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				reader := artifactObjectReader{ctx: ctx, dir: f.git("rev-parse", "--absolute-git-dir"), seen: map[string]bool{}}
				defer reader.close()
				before := f.snapshot()
				var err error
				if method == "commit" {
					var value *storedArtifact
					value, _, err = reader.commit(oid)
					if err == nil && value.ID != artifactVectorID {
						t.Fatal("one-shot commit changed signed identity", value)
					}
				} else {
					reader.scoped = method == "managed-repeat"
					var body []byte
					body, err = reader.object(oid, "blob", artifactReadObjectBytes)
					if err == nil && !bytes.Equal(body, objects[oid]) {
						t.Fatal("one-shot object changed raw bytes")
					}
					if err == nil && method == "managed-repeat" {
						body, err = reader.object(oid, "blob", artifactReadObjectBytes)
						if err == nil && (!bytes.Equal(body, objects[oid]) || reader.total != int64(len(body)) || len(reader.seen) != 1) {
							t.Fatal("managed raw reread charged a duplicate or retained wrong bytes", reader.total, reader.seen)
						}
						if err == nil {
							err = reader.finish()
						}
					}
				}
				if mode == "pass" && err != nil || mode != "pass" && err == nil {
					t.Fatal("one-shot/managed owner lost finalization failure", mode, err)
				}
				if err != nil {
					problem, ok := err.(*artifact.Error)
					if !ok || problem.Code != "verification_failure" || problem.Message != "accepted artifact object unavailable; no fetch attempted" {
						t.Fatal("one-shot changed exact sole-fault diagnostic", err)
					}
				}
				reader.close()
				f.unchanged(before)
				if method != "managed-repeat" {
					artifactIOReadEvidence(t, ledger, sessions, noFetch, 1, expected, true)
				} else {
					raw, readErr := os.ReadFile(ledger)
					if readErr != nil {
						t.Fatal(readErr)
					}
					info, contents, eof := 0, 0, 0
					for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
						var event map[string]any
						if json.Unmarshal(line, &event) != nil {
							t.Fatal("invalid repeated-object ledger")
						}
						if event["event"] == "request" && event["oid"] == oid {
							if event["action"] == "info" {
								info++
							} else {
								contents++
							}
						}
						if event["event"] == "stdin-eof" {
							eof++
						}
					}
					if info != 2 || contents != 2 || eof != 1 {
						t.Fatal("full reader cached bytes instead of authentic raw reread", info, contents, eof)
					}
					artifactIOReadEvidence(t, ledger, sessions, noFetch, 1, expected, false)
				}
				wp25Record(t, map[string]any{"control": "full-reader-private-one-shot-and-reread", "source_assisted": true, "supplied_package_claim": false, "method": method, "mode": mode, "error": fmt.Sprint(err), "total_unique_bytes": reader.total})
			})
		}
	}
}

func TestArtifactIOPipeOwnership(t *testing.T) {
	if os.Getenv("HN_WP25_PIPE_SETUP_CHILD") == "1" {
		// Resource exhaustion is confined to this disposable compiled-self
		// process. The parent/runner limit and every production setting stay put.
		input, output, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		input.Close()
		output.Close()
		var original syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
			t.Fatal(err)
		}
		limited := original
		limited.Cur = 64
		if original.Cur < limited.Cur {
			limited.Cur = original.Cur
		}
		if limited.Cur < 16 {
			t.Fatal("isolated pipe control has insufficient descriptor allowance", original)
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limited); err != nil {
			t.Fatal(err)
		}
		files := []*os.File{}
		defer func() {
			for _, file := range files {
				file.Close()
			}
			if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
				t.Error(err)
			}
		}()
		for len(files) <= 64 {
			file, err := os.Open(os.DevNull)
			if err != nil {
				if !errors.Is(err, syscall.EMFILE) {
					t.Fatal(err)
				}
				break
			}
			files = append(files, file)
		}
		if len(files) < 2 || len(files) > 64 {
			t.Fatal("bounded descriptor exhaustion was not established", len(files))
		}
		for range 2 {
			files[len(files)-1].Close()
			files = files[:len(files)-1]
		}
		// Exactly two free slots admit stdin's pipe, then fail stdout's pipe.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		session, err := newArtifactPreflightSession(ctx, os.Getenv("HN_WP25_PIPE_SETUP_GIT_DIR"))
		if session != nil {
			session.close()
			t.Fatal("partial pipe setup unexpectedly started a child")
		}
		problem, ok := err.(*artifact.Error)
		if !ok || problem.Code != "verification_failure" || problem.Message != "accepted artifact object unavailable; no fetch attempted" {
			t.Fatal("partial pipe setup changed refusal", err)
		}
		for range 2 {
			file, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatal("partial setup leaked an acquired descriptor", err)
			}
			files = append(files, file)
		}
		if file, err := os.Open(os.DevNull); !errors.Is(err, syscall.EMFILE) {
			if file != nil {
				file.Close()
			}
			t.Fatal("partial setup control did not retain its exact exhausted boundary", err)
		}
		t.Logf("source-assisted partial stdout-pipe failure: child_pid=%d child_limit=%d original_limit=%d exactly_two_slots_restored=true", os.Getpid(), limited.Cur, original.Cur)
		return
	}
	f := wp25ArtifactCLI(t)
	dir := f.git("rev-parse", "--absolute-git-dir")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := artifactObjectReader{ctx: ctx, dir: dir, seen: map[string]bool{}}
	_, err := reader.object(strings.Repeat("a", 40), "blob", artifactReadObjectBytes)
	problem, ok := err.(*artifact.Error)
	if !ok || problem.Code != "cancelled" || reader.session != nil {
		t.Fatal("already-cancelled reader started work or changed refusal", err)
	}
	writer := artifactBlobWriter{ctx: ctx, dir: dir}
	if err := writer.write(strings.Repeat("a", 40), []byte("not written")); !errors.Is(err, context.Canceled) || writer.session != nil || writer.scratch != nil {
		t.Fatal("already-cancelled blob writer acquired resources", err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	marker := filepath.Join(root, "forbidden-git-child")
	script := "#!/bin/sh\nprintf forbidden > '" + strings.ReplaceAll(marker, "'", "'\"'\"'") + "'\nexit 87\n"
	if err := os.WriteFile(filepath.Join(root, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var before, after syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &before); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(self, "-test.run=^TestArtifactIOPipeOwnership$", "-test.v", "-test.count=1", "-test.timeout=1m")
	child.Env = operationalTestEnvironmentWith(f.home, map[string]string{"PATH": root + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP25_PIPE_SETUP_CHILD": "1", "HN_WP25_PIPE_SETUP_GIT_DIR": dir})
	raw, childErr := child.CombinedOutput()
	if childErr != nil || bytes.Count(raw, []byte("--- PASS: TestArtifactIOPipeOwnership")) != 1 || bytes.Contains(raw, []byte("FAIL")) || bytes.Contains(raw, []byte("SKIP")) {
		t.Fatal("isolated partial-pipe control did not execute/pass", childErr, string(raw))
	}
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &after); err != nil || after != before {
		t.Fatal("child resource fault changed parent allowance", err, before, after)
	}
	if body, err := os.ReadFile(marker); !os.IsNotExist(err) {
		t.Fatal("partial pipe setup dispatched Git", err, string(body))
	}
	wp25Record(t, map[string]any{"control": "private-partial-pipe-and-pre-cancel-ownership", "source_assisted": true, "supplied_package_claim": false, "argv": child.Args, "stdout_stderr_raw": string(raw), "process_error": fmt.Sprint(childErr), "parent_rlimit_before": before, "parent_rlimit_after": after, "git_never_started": true})
}

func TestArtifactFullReaderFirstErrorCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, fault := range []string{"message", "signature", "signature-role-size"} {
			t.Run(format+"/"+fault, func(t *testing.T) {
				f := artifactCLIFormat(t, format)
				f.overrides = map[string]string{"XDG_CONFIG_HOME": wp25FixtureConfig(t, f.home)}
				ref := artifactFrozenRef(t, f)
				message, signature := "hn artifact "+artifactVectorID, artifactVectorSignature
				code, diagnostic := "invalid_history", "artifact commit message mismatch"
				switch fault {
				case "message":
					message = "wrong artifact message"
				case "signature":
					signature = "A" + signature[1:]
					if signature == artifactVectorSignature {
						signature = "B" + signature[1:]
					}
					code, diagnostic = "verification_failure", "invalid artifact signature"
				case "signature-role-size":
					signature += "A"
					code, diagnostic = "resource_limit", "artifact object exceeds read limit"
				}
				root := artifactRawCommit(t, f, artifactVector, signature, "", message, false)
				f.git("update-ref", ref, root)
				// The mandatory real-Git sole-fault response precedes any peer.
				artifactCountBatchCheck(t, f, code, diagnostic)
				objects, kinds := artifactIOReadObjects(t, f, root)
				base := f.overrides
				ledger, noFetch, sessions, overrides := artifactFullBatchPeerInstall(t, f, "exit-error", objects, kinds)
				f.overrides = overrides
				artifactCountBatchCheck(t, f, code, diagnostic)
				artifactIOReadEvidence(t, ledger, sessions, noFetch, 2, kinds, false)
				if fault == "signature-role-size" {
					raw, err := os.ReadFile(ledger)
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
						var event map[string]any
						if json.Unmarshal(line, &event) != nil {
							t.Fatal("invalid role-size ledger")
						}
						if event["event"] == "request" && event["action"] == "contents" && len(objects[event["oid"].(string)]) == 87 {
							t.Fatal("signature role first-over authorized body contents", event)
						}
					}
				}
				f.overrides = base
				wp25Record(t, map[string]any{"control": "first-detected-read-error-before-cleanup", "source_assisted_fault_peer": true, "format": format, "fault": fault, "original_code": code, "original_message": diagnostic, "cleanup_parent_cancellation_claim": false, "scope": "ordered semantic/role failure survives mandatory cleanup; no universal independent simultaneous-fault ordering claim"})
			})
		}
	}
}
func artifactCountBatchCheck(t *testing.T, f *artifactCLIFixture, code, message string) {
	t.Helper()
	f.t = t
	before := f.snapshot()
	result := wp25ArtifactCall(f, "", code, "artifact", "type", "show", artifactVectorID, "--json")
	if code != "" {
		problem := result["error"].(map[string]any)
		if result["data"] != nil || len(problem) != 5 || problem["origin"] != "artifact" || problem["recovery"] != "do_not_retry" || problem["operation"] != nil || problem["message"] != message {
			t.Fatal("batch refusal identity", result)
		}
	} else {
		items := result["data"].(map[string]any)["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["id"] != artifactVectorID {
			t.Fatal("batch success lost signed identity", result)
		}
	}
	wp25Unchanged(f, before)
}
func TestArtifactStoreCountPreflightBatchProtocolCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := artifactCLIFormat(t, format)
			f.overrides = map[string]string{"XDG_CONFIG_HOME": wp25FixtureConfig(t, f.home)}
			ref := artifactFrozenRef(t, f)
			root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			f.git("update-ref", ref, root)
			body := artifactCountBatchRaw(t, f, root)
			base := f.overrides
			artifactCountBatchCheck(t, f, "", "") // Mandatory real-Git positive before peers.
			modes := []string{"pass", "failure", "empty", "info-missing", "info-type", "info-negative", "info-nonnumeric", "info-overflow", "info-oid", "info-extra", "info-short", "info-long", "info-error", "exit-after-info", "contents-oid", "contents-type", "contents-size", "contents-negative", "contents-extra", "contents-long", "contents-short-header", "short-body", "missing-lf", "wrong-lf", "hash", "hash-with-invalid-parent", "trailing", "exit-error", "stderr"}
			for _, mode := range modes {
				t.Run(mode, func(t *testing.T) {
					f.t = t
					ledger, noFetch, overrides := artifactCountBatchPeerInstall(t, f, mode, map[string][]byte{root: body}, nil)
					f.overrides = overrides
					defer func() { f.overrides = base }()
					code, message := "verification_failure", "accepted artifact object unavailable; no fetch attempted"
					switch mode {
					case "pass":
						code, message = "", ""
					case "info-missing", "info-type":
						message = "accepted artifact object is missing or has wrong type"
					case "info-negative", "info-nonnumeric", "info-overflow":
						message = "invalid Git object size"
					case "hash", "hash-with-invalid-parent":
						message = "accepted Git object hash mismatch"
					}
					artifactCountBatchCheck(t, f, code, message)
					forwarded := 0
					if mode == "pass" {
						forwarded = 1
					}
					events := artifactCountBatchLedger(t, ledger, noFetch, forwarded)
					if strings.HasPrefix(mode, "info-") || mode == "empty" || mode == "failure" || mode == "stderr" {
						for _, event := range events {
							if event["event"] == "request" && event["action"] == "contents" {
								t.Fatal("invalid metadata authorized a contents request", mode, events)
							}
						}
					}
					if mode == "pass" {
						infos, contents, eof := 0, 0, 0
						for _, event := range events {
							if event["event"] == "request" {
								if event["action"] == "info" {
									infos++
								} else {
									contents++
								}
							}
							if event["event"] == "stdin-eof" {
								eof++
							}
						}
						if infos != 1 || contents != 1 || eof != 1 {
							t.Fatal("one-object scoped protocol did not complete exact session", events)
						}
					}
					f.overrides = base
				})
				f.t = t
			}
			// Overlapping captured tips retain the same raw ancestor once. The real
			// Git positive is followed by a supplemental ledger over identical bytes.
			child := artifactRawCommit(t, f, artifactSecondVector, artifactSecondSignature, root, "hn artifact "+wp25RecordCountHash([]byte(artifactSecondVector)), false)
			alias := strings.Replace(ref, "refs/hn/", "refs/hn/remotes/overlap/", 1)
			f.git("update-ref", ref, child)
			f.git("update-ref", alias, root)
			artifactCountBatchCheck(t, f, "", "")
			overlapObjects := map[string][]byte{root: body, child: artifactCountBatchRaw(t, f, child)}
			overlapLedger, overlapFetch, overlapEnv := artifactCountBatchPeerInstall(t, f, "pass", overlapObjects, nil)
			f.overrides = overlapEnv
			artifactCountBatchCheck(t, f, "", "")
			events := artifactCountBatchLedger(t, overlapLedger, overlapFetch, 1)
			counts := map[string]int{}
			for _, event := range events {
				if event["event"] == "request" {
					key := event["action"].(string) + " " + event["oid"].(string)
					counts[key]++
				}
			}
			for oid := range overlapObjects {
				if counts["info "+oid] != 1 || counts["contents "+oid] != 1 {
					t.Fatal("overlapping ancestors were counted or charged more than once", counts)
				}
			}
			f.overrides = base
			f.git("update-ref", ref, root)
			f.git("update-ref", "-d", alias)
			// An empty accepted selection must not start a batch child at all.
			f.git("update-ref", "-d", ref)
			ledger, noFetch, overrides := artifactCountBatchPeerInstall(t, f, "failure", nil, nil)
			f.overrides = overrides
			before := f.snapshot()
			result := wp25ArtifactCall(f, "", "", "artifact", "list", "--limit", "1", "--json")
			if len(result["data"].(map[string]any)["items"].([]any)) != 0 {
				t.Fatal("empty selection gained items", result)
			}
			wp25Unchanged(f, before)
			if _, err := os.Stat(ledger); !os.IsNotExist(err) {
				t.Fatal("empty selection started batch process", err)
			}
			if _, err := os.Stat(noFetch); !os.IsNotExist(err) {
				t.Fatal("empty selection fetched", err)
			}
			f.overrides = base
			f.git("update-ref", ref, root)
			artifactCountBatchCheck(t, f, "", "")
		})
	}
}
func TestArtifactStoreCountPreflightCancellationCLI(t *testing.T) {
	for _, mode := range []string{"stall-info", "stall-body", "stall-eof"} {
		t.Run(mode, func(t *testing.T) {
			f := wp25ArtifactCLI(t)
			ref := artifactFrozenRef(t, f)
			commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			f.git("update-ref", ref, commit)
			body := artifactCountBatchRaw(t, f, commit)
			base := f.overrides
			ledger, noFetch, overrides := artifactCountBatchPeerInstall(t, f, mode, map[string][]byte{commit: body}, nil)
			f.overrides = overrides
			defer func() { f.overrides = base }()
			started := time.Now()
			artifactCountBatchCheck(t, f, "cancelled", "artifact read cancelled")
			elapsed := time.Since(started)
			if elapsed < 29*time.Second || elapsed > 45*time.Second {
				t.Fatal("batch cancellation did not use actual normal30s stage", elapsed)
			}
			events := artifactCountBatchLedger(t, ledger, noFetch)
			wp25Record(t, map[string]any{"control": "count-preflight-batch-cancellation", "mode": mode, "elapsed_ns": elapsed.Nanoseconds(), "events": events})
			f.overrides = base
			artifactCountBatchCheck(t, f, "", "")
		})
	}
}
func TestArtifactStoreCountPreflightBeforeBodyCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := artifactCLIFormat(t, format)
			f.overrides = map[string]string{"XDG_CONFIG_HOME": wp25FixtureConfig(t, f.home)}
			ref := artifactFrozenRef(t, f)
			root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			tree := f.git("rev-parse", root+"^{tree}")
			base := f.overrides
			for _, n := range []int{8388608, 8388609, 9437184} {
				for _, position := range []string{"tip", "ancestor"} {
					t.Run(fmt.Sprintf("%d-%s", n, position), func(t *testing.T) {
						f.t = t
						prefix := "tree " + tree + "\nauthor Fixture <fixture@hn.invalid> 1793433600 +0000\ncommitter Fixture <fixture@hn.invalid> 1793433600 +0000\n\n"
						body := []byte(prefix + strings.Repeat("x", n-len(prefix)))
						big := f.gitInput(string(body), "hash-object", "--literally", "-t", "commit", "-w", "--stdin")
						tip := big
						objects := map[string][]byte{big: body}
						sizes := map[string]int{big: n}
						if position == "ancestor" {
							tip = artifactRawCommit(t, f, artifactVector, artifactVectorSignature, big, "hn artifact "+artifactVectorID, false)
							objects[tip] = artifactCountBatchRaw(t, f, tip)
							sizes[tip] = len(objects[tip])
						}
						f.git("update-ref", ref, tip)
						code, message := "resource_limit", "artifact object exceeds read limit"
						if n == 8388608 {
							code, message = "invalid_history", "artifact commit message mismatch"
						}
						// Actual real Git + actual supplied reader, without any peer substitution.
						artifactCountBatchCheck(t, f, code, message)
						wp25Record(t, map[string]any{"control": "count-preflight-real-Git-bounded-commit", "position": position, "commit_bytes": n, "commit_oid": big, "selected_tip": tip, "commit_body_sha256": wp25RecordCountHash(body), "real_git": true, "expected_code": code, "expected_message": message})
						// Supplemental request ledger replays the exact real metadata and bounded
						// raw bodies. Oversized body is intentionally unavailable to this peer.
						if n > 8388608 {
							objects[big] = nil
						}
						ledger, noFetch, overrides := artifactCountBatchPeerInstall(t, f, "pass", objects, sizes)
						f.overrides = overrides
						defer func() { f.overrides = base }()
						artifactCountBatchCheck(t, f, code, message)
						forwarded := 0
						if n == 8388608 {
							forwarded = 1
						}
						events := artifactCountBatchLedger(t, ledger, noFetch, forwarded)
						info, contents := 0, 0
						for _, event := range events {
							if event["event"] == "request" && event["oid"] == big {
								if event["action"] == "info" {
									info++
								} else {
									contents++
								}
							}
						}
						if info != 1 || (n > 8388608 && contents != 0) || (n == 8388608 && contents != 1) {
							t.Fatal("metadata gate did not precede bounded body authorization", n, position, events)
						}
						f.overrides = base
						f.git("update-ref", ref, root)
						artifactCountBatchCheck(t, f, "", "")
					})
					f.t = t
				}
			}
		})
	}
}

func TestArtifactStoreCountPreflightRawParentCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := artifactCLIFormat(t, format)
			f.overrides = map[string]string{"XDG_CONFIG_HOME": wp25FixtureConfig(t, f.home)}
			ref := artifactFrozenRef(t, f)
			root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			tree := f.git("rev-parse", root+"^{tree}")
			for _, mode := range []string{"invalid-parent", "missing-parent", "missing-separator", "message-parent", "continuation-parent"} {
				t.Run(mode, func(t *testing.T) {
					f.t = t
					body := "tree " + tree + "\nauthor Fixture <fixture@hn.invalid> 1793433600 +0000\ncommitter Fixture <fixture@hn.invalid> 1793433600 +0000\n\nhn artifact " + artifactVectorID + "\n"
					code, message := "invalid_history", "invalid Git object identity"
					switch mode {
					case "invalid-parent":
						body = "tree " + tree + "\nparent not-an-oid\n\nhn artifact " + artifactVectorID + "\n"
					case "missing-parent":
						body = "tree " + tree + "\nparent " + strings.Repeat("f", len(root)) + "\n\nhn artifact " + artifactVectorID + "\n"
						code, message = "verification_failure", "accepted artifact object is missing or has wrong type"
					case "missing-separator":
						body = "tree " + tree + "\n"
						message = "invalid artifact commit"
					case "message-parent":
						body = "tree " + tree + "\nauthor Fixture <fixture@hn.invalid> 1793433600 +0000\ncommitter Fixture <fixture@hn.invalid> 1793433600 +0000\n\nparent " + strings.Repeat("f", len(root)) + "\nhn artifact " + artifactVectorID + "\n"
						message = "artifact commit message mismatch"
					case "continuation-parent":
						body = "tree " + tree + "\nauthor Fixture <fixture@hn.invalid> 1793433600 +0000\ncommitter Fixture <fixture@hn.invalid> 1793433600 +0000\nx-wp25 inert\n parent " + strings.Repeat("f", len(root)) + "\n\nhn artifact " + artifactVectorID + "\n"
						code, message = "", ""
					}
					oid := f.gitInput(body, "hash-object", "--literally", "-t", "commit", "-w", "--stdin")
					// SOURCE-ASSISTED malformed-ref setup: real Git's update-ref refuses
					// malformed commit syntax before the public reader can inspect it.
					// Select the existing real Git object by a physical accepted ref only.
					gitDir := f.git("rev-parse", "--absolute-git-dir")
					selectedRefPath := filepath.Join(gitDir, filepath.FromSlash(ref))
					if err := os.MkdirAll(filepath.Dir(selectedRefPath), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(selectedRefPath, []byte(oid+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
					wp25Record(t, map[string]any{"control": "count-preflight-raw-parent-real-Git", "mode": mode, "oid": oid, "body_raw": body, "body_sha256": wp25RecordCountHash([]byte(body)), "format": format, "selected_ref_path": selectedRefPath, "ref_setup": "source-assisted physical accepted ref; actual Git object and supplied public caller"})
					artifactCountBatchCheck(t, f, code, message)
					if err := os.WriteFile(selectedRefPath, []byte(root+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
					artifactCountBatchCheck(t, f, "", "")
				})
				f.t = t
			}
		})
	}
}

// These sources deliberately share the same signed genesis while differing in
// non-semantic Git header bytes. They are NOT an accepted whole actor history:
// after equality passes the private count gate, the retained full verifier must
// still refuse divergent raw actor roots. That is a narrow admission witness.
func TestArtifactStoreCountPreflightUniqueCommitBytesCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := artifactCLIFormat(t, format)
			f.overrides = map[string]string{"XDG_CONFIG_HOME": wp25FixtureConfig(t, f.home)}
			baseOverrides := f.overrides
			ref := artifactFrozenRef(t, f)
			root := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			tree := f.git("rev-parse", root+"^{tree}")
			sources := []string{}
			objects := map[string][]byte{}
			sizes := map[string]int{}
			bodyFiles := map[string]string{}
			bodyDigests := map[string]string{}
			fixtureDir := t.TempDir()
			small := 512
			total := 0
			makeBody := func(index, n int) (string, string) {
				prefix := fmt.Sprintf("tree %s\nauthor WP25 <fixture@hn.invalid> 1793433600 +0000\ncommitter WP25 <fixture@hn.invalid> 1793433600 +0000\nx-wp25 %02d ", tree, index)
				suffix := "\n\nhn artifact " + artifactVectorID + "\n"
				padding := n - len(prefix) - len(suffix)
				if padding < 0 {
					t.Fatal("aggregate commit fixture underflow", n)
				}
				raw := []byte(prefix + strings.Repeat("x", padding) + suffix)
				if len(raw) != n {
					t.Fatal("aggregate raw commit size", len(raw), n)
				}
				path := filepath.Join(fixtureDir, fmt.Sprintf("commit-%02d-%d.raw", index, n))
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				oid := f.git("hash-object", "--literally", "-t", "commit", "-w", path)
				bodyDigests[oid] = wp25RecordCountHash(raw)
				return oid, path
			}
			//31 full8MiB bodies, one8MiB-minus512 body and one512B body.
			// Every individual object remains at or below the independent8MiB cap.
			for i := 0; i < 33; i++ {
				n := 8388608
				if i == 31 {
					n -= small
				}
				if i == 32 {
					n = small
				}
				oid, path := makeBody(i, n)
				if _, duplicate := objects[oid]; duplicate {
					t.Fatal("aggregate fixture object unexpectedly duplicated")
				}
				source := strings.Replace(ref, "refs/hn/", fmt.Sprintf("refs/hn/remotes/aggr%02d/", i), 1)
				f.git("update-ref", source, oid)
				sources = append(sources, source)
				objects[oid] = nil
				sizes[oid] = n
				bodyFiles[oid] = path
				total += n
			}
			if total != 268435456 || len(objects) != 33 {
				t.Fatal("aggregate independent source dimensions", total, len(objects))
			}
			duplicate := strings.Replace(ref, "refs/hn/", "refs/hn/remotes/aggrzz/", 1)
			first := f.git("rev-parse", sources[0])
			f.git("update-ref", duplicate, first)
			wp25Record(t, map[string]any{"control": "count-preflight-unique-commit-aggregate-fixture", "format": format, "source_assisted_construction": true, "sources": sources, "duplicate_alias": duplicate, "duplicate_oid": first, "objects": sizes, "raw_commit_body_sha256": bodyDigests, "unique_commit_bytes": total, "individual_commit_cap": 8388608, "aggregate_cap": 268435456, "whole_history_is_valid": false, "later_expected": "actor_stream_conflict/divergent accepted actor streams", "private_gate_only_admission": true})
			// Mandatory real-Git/public equality, with duplicate source selected. The
			// later exact conflict proves this check grants no accepted-view trust.
			artifactCountBatchCheck(t, f, "actor_stream_conflict", "divergent accepted actor streams")
			ledger, noFetch, overrides := artifactCountBatchPeerInstall(t, f, "pass", objects, sizes, bodyFiles)
			f.overrides = overrides
			defer func() { f.overrides = baseOverrides }()
			artifactCountBatchCheck(t, f, "actor_stream_conflict", "divergent accepted actor streams")
			events := artifactCountBatchLedger(t, ledger, noFetch, 1)
			info, contents, eof := map[string]int{}, map[string]int{}, 0
			for _, event := range events {
				if event["event"] == "request" {
					oid := event["oid"].(string)
					if event["action"] == "info" {
						info[oid]++
					} else {
						contents[oid]++
					}
				}
				if event["event"] == "stdin-eof" {
					eof++
				}
			}
			for oid := range objects {
				if info[oid] != 1 || contents[oid] != 1 {
					t.Fatal("aggregate equality or duplicate charging failed", oid, info[oid], contents[oid])
				}
			}
			if eof != 1 {
				t.Fatal("aggregate equality session did not finish and reap", events)
			}
			f.overrides = baseOverrides
			// Grow only the final small raw body by one byte. This is still a valid
			// bounded commit object; the unique subset is now exactly256MiB+1.
			oldLast := f.git("rev-parse", sources[32])
			overOID, overFile := makeBody(32, small+1)
			f.git("update-ref", sources[32], overOID)
			delete(objects, oldLast)
			delete(sizes, oldLast)
			delete(bodyFiles, oldLast)
			objects[overOID] = nil
			sizes[overOID] = small + 1
			wp25Record(t, map[string]any{"control": "count-preflight-unique-commit-aggregate-first-over", "format": format, "selected_last_ref": sources[32], "selected_last_oid": overOID, "raw_last_sha256": bodyDigests[overOID], "raw_last_bytes": small + 1, "raw_last_file": overFile, "unique_commit_bytes": 268435457, "whole_history_is_valid": false, "expected_code": "resource_limit", "expected_message": "artifact aggregate exceeds read limit"})
			// Actual real Git observes the exact aggregate refusal independently.
			artifactCountBatchCheck(t, f, "resource_limit", "artifact aggregate exceeds read limit")
			// The supplemental peer has all authorized predecessor bodies, but withholds
			// the first-over target. It must see info and must never see contents for it.
			ledger, noFetch, overrides = artifactCountBatchPeerInstall(t, f, "pass", objects, sizes, bodyFiles)
			f.overrides = overrides
			artifactCountBatchCheck(t, f, "resource_limit", "artifact aggregate exceeds read limit")
			events = artifactCountBatchLedger(t, ledger, noFetch)
			info, contents = map[string]int{}, map[string]int{}
			for _, event := range events {
				if event["event"] == "request" {
					oid := event["oid"].(string)
					if event["action"] == "info" {
						info[oid]++
					} else {
						contents[oid]++
					}
				}
			}
			if info[overOID] != 1 || contents[overOID] != 0 {
				t.Fatal("aggregate guard authorized first-over contents", info[overOID], contents[overOID], events)
			}
			for oid := range bodyFiles {
				if info[oid] != 1 || contents[oid] != 1 {
					t.Fatal("aggregate refusal did not follow exact bounded prefix", oid, info[oid], contents[oid])
				}
			}
			f.overrides = baseOverrides
		})
	}
}
