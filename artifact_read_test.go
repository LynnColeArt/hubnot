package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hubnot/internal/artifact"
	"hubnot/internal/migration"
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
	t.Run("witness-pair-wave", artifactWitnessPairWaveContract)
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
	Mode            string            `json:"mode"`
	Ledger          string            `json:"ledger"`
	Objects         map[string][]byte `json:"objects"`
	Sizes           map[string]int    `json:"sizes,omitempty"`
	ObjectFiles     map[string]string `json:"object_files,omitempty"`
	Types           map[string]string `json:"types,omitempty"`
	ActualGit       string            `json:"actual_git,omitempty"`
	SessionRoot     string            `json:"session_root,omitempty"`
	Sessions        string            `json:"sessions,omitempty"`
	Target          int               `json:"target,omitempty"`
	ObservedSession int               `json:"-"`
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
				cfg.ObservedSession = index
			}
			os.Exit(artifactCountBatchPeer(cfg, os.Args[i+2:]))
		}
	}
}
func artifactCountBatchPeer(cfg artifactCountBatchPeerConfig, argv []string) int {
	log := func(event map[string]any) {
		event["pid"], event["ppid"] = os.Getpid(), os.Getppid()
		event["session"], event["mode"] = cfg.ObservedSession, cfg.Mode
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
	if cfg.Mode == "stderr-overflow" {
		// Continue the valid protocol: removing the cap must turn this case
		// into a genuine signed success rather than another independent error.
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("S"), 8193))
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
				artifactIOStall(log, "eof")
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
				artifactIOStall(log, "info")
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
			artifactIOStall(log, "body")
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
			for _, mode := range []string{"pass", "stderr-at", "stderr-overflow", "exit-error", "trailing", "trailing-eof", "info-missing", "info-type", "info-negative", "info-noncanonical", "info-nonnumeric", "info-overflow", "info-oid", "info-extra", "info-short", "info-long", "info-error", "exit-after-info", "contents-oid", "contents-type", "contents-size", "contents-negative", "contents-extra", "contents-long", "contents-short-header", "hash", "short-body", "missing-lf", "wrong-lf", "stderr"} {
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

// Only controlled test peers call this. The marker is durably closed before
// sleeping, and the caller must observe it while the supplied CLI is alive.
func artifactIOStall(log func(map[string]any), phase string) {
	log(map[string]any{"event": "stall-entry", "phase": phase, "at": time.Now().UTC().Format(time.RFC3339Nano)})
	time.Sleep(time.Minute)
}

type artifactIOPhase struct {
	Mode, Phase, Ledger, Sessions string
	Session                       int
}

func artifactIOObservedCall(t *testing.T, f *artifactCLIFixture, input, diagnostic string, phase artifactIOPhase, args ...string) map[string]any {
	t.Helper()
	before := f.snapshot()
	watchdog, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(watchdog, f.binary, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = f.repo, operationalTestEnvironmentWith(f.home, f.overrides), strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatal("phase-witness candidate did not start", err)
	}
	candidatePID := cmd.Process.Pid
	processDone := make(chan struct{})
	observed := make(chan map[string]any, 1)
	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		deadline := time.NewTimer(5 * time.Second)
		tick := time.NewTicker(5 * time.Millisecond)
		defer deadline.Stop()
		defer tick.Stop()
		identityRaw := ""
		finish := func(event map[string]any, raw, failure string) {
			alive := cmd.Process.Signal(syscall.Signal(0)) == nil
			select {
			case <-processDone:
				alive = false
			default:
			}
			observed <- map[string]any{"expected_mode": phase.Mode, "expected_phase": phase.Phase, "expected_session": phase.Session, "candidate_pid": candidatePID, "candidate_alive_at_observation": alive, "marker": event, "marker_raw": raw, "session_identity_raw": identityRaw, "observed_at": time.Now().UTC().Format(time.RFC3339Nano), "observed_elapsed_ns": time.Since(started).Nanoseconds(), "error": failure}
		}
		for {
			select {
			case <-processDone:
				finish(nil, "", "candidate exited before required pre-deadline phase witness")
				return
			case <-deadline.C:
				finish(nil, "", "required phase was not observed within5s while candidate alive")
				return
			case <-tick.C:
				raw, err := os.ReadFile(phase.Ledger)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					finish(nil, "", "cannot read controlled phase ledger: "+err.Error())
					return
				}
				for _, line := range bytes.Split(raw, []byte{'\n'}) {
					var event map[string]any
					if json.Unmarshal(line, &event) != nil || event["event"] != "stall-entry" {
						continue
					}
					failure := ""
					pid, validPID := event["pid"].(float64)
					if event["phase"] != phase.Phase || event["mode"] != phase.Mode || event["session"] != float64(phase.Session) || event["ppid"] != float64(candidatePID) || !validPID || pid <= 0 || event["at"] == nil {
						failure = "wrong controlled phase/mode/session/PID witness"
					}
					identities, identityErr := os.ReadFile(phase.Sessions)
					matched := false
					for _, identity := range bytes.Split(identities, []byte{'\n'}) {
						var value map[string]any
						if json.Unmarshal(identity, &value) == nil && value["session"] == float64(phase.Session) && value["pid"] == event["pid"] && value["ppid"] == float64(candidatePID) {
							identityRaw, matched = string(identity), true
						}
					}
					if identityErr != nil || !matched {
						failure = "phase marker did not match actual selected child/session identity"
					}
					select {
					case <-processDone:
						failure = "phase arrived after candidate exit"
					default:
					}
					if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
						failure = "candidate was not alive at phase observation: " + err.Error()
					}
					if time.Since(started) >= 5*time.Second {
						failure = "phase observation was not early"
					}
					finish(event, string(line), failure)
					return
				}
			}
		}
	}()
	// Wait and observer both join before any assertion can terminate this test.
	runErr := cmd.Wait()
	close(processDone)
	witness := <-observed
	<-observerDone
	after := f.snapshot()
	wp25Record(t, map[string]any{"control": "pre-deadline-owned-phase-public-call", "source_assisted_fault_peer": true, "candidate": f.binary, "candidate_sha256": wp25Digest(t, f.binary), "argv": args, "cwd": f.repo, "candidate_pid": candidatePID, "stdout_raw": stdout.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(runErr), "watchdog_error": fmt.Sprint(watchdog.Err()), "elapsed_nanoseconds": time.Since(started).Nanoseconds(), "phase_observation": witness, "observer_joined": true, "protected_before": before, "protected_after": after, "product_stage_seconds": 30, "early_observer_seconds": 5})
	if witness["error"] != "" || witness["candidate_alive_at_observation"] != true {
		t.Fatal("required pre-deadline causal phase witness failed", witness)
	}
	if watchdog.Err() != nil {
		t.Fatal("phase control outer watchdog expired", watchdog.Err())
	}
	response := wp25LifecycleEnvelope(t, stdout.Bytes(), runErr, "artifact", "cancelled", "do_not_retry", nil)
	if response["error"].(map[string]any)["message"] != diagnostic {
		t.Fatal("phase cancellation changed exact diagnostic", response)
	}
	return response
}

func artifactIOReadStallEvidence(t *testing.T, ledger, mode string, objects int) {
	t.Helper()
	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			t.Fatal("invalid stalled-reader event")
		}
		kind := event["event"].(string)
		if action, ok := event["action"].(string); ok {
			kind += ":" + action
		}
		counts[kind]++
		if event["event"] == "stall-entry" && (event["phase"] != strings.TrimPrefix(mode, "stall-") || event["mode"] != mode) {
			t.Fatal("wrong reader stall marker", event)
		}
	}
	if counts["start"] != 1 || counts["stall-entry"] != 1 {
		t.Fatal("reader did not enter exactly one controlled stall", counts)
	}
	want := map[string]int{"request:info": 1}
	switch mode {
	case "stall-info":
	case "stall-body":
		want["header:info"], want["request:contents"], want["header:contents"] = 1, 1, 1
	case "stall-eof":
		want["request:info"], want["header:info"], want["request:contents"], want["header:contents"], want["body"], want["stdin-eof"] = objects, objects, objects, objects, objects, 1
	default:
		t.Fatal("unknown expected reader stall", mode)
	}
	for _, kind := range []string{"request:info", "header:info", "request:contents", "header:contents", "body", "stdin-eof"} {
		if counts[kind] != want[kind] {
			t.Fatal("reader stalled at a different protocol phase", mode, kind, counts, want)
		}
	}
	wp25Record(t, map[string]any{"control": "exact-reader-stall-phase-counts", "mode": mode, "counts": counts, "expected": want, "raw": string(raw)})
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
			before := f.snapshot()
			artifactIOObservedCall(t, f, "", "artifact read cancelled", artifactIOPhase{Mode: mode, Phase: strings.TrimPrefix(mode, "stall-"), Ledger: ledger, Sessions: sessions, Session: 2}, "artifact", "type", "show", artifactVectorID, "--json")
			f.unchanged(before)
			elapsed := time.Since(started)
			if elapsed < 29*time.Second || elapsed > 45*time.Second {
				t.Fatal("full-reader cancellation did not retain normal30s stage", elapsed)
			}
			artifactIOReadEvidence(t, ledger, sessions, noFetch, 2, kinds, false)
			artifactIOReadStallEvidence(t, ledger, mode, 4)
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
			before := f.snapshot()
			artifactIOObservedCall(t, f, "", "artifact read cancelled", artifactIOPhase{Mode: mode, Phase: strings.TrimPrefix(mode, "stall-"), Ledger: ledger, Sessions: filepath.Join(filepath.Dir(ledger), "sessions.jsonl"), Session: 1}, "artifact", "type", "show", artifactVectorID, "--json")
			f.unchanged(before)
			elapsed := time.Since(started)
			if elapsed < 29*time.Second || elapsed > 45*time.Second {
				t.Fatal("batch cancellation did not use actual normal30s stage", elapsed)
			}
			events := artifactCountBatchLedger(t, ledger, noFetch)
			artifactIOReadStallEvidence(t, ledger, mode, 1)
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

// Native retained-contribution transport has its own error and stderr policy.
// These source-assisted peers cross the actual supplied CLI boundary; they are
// separate from the artifact --batch-command peers above.
func nativeClosureRawFixture(t *testing.T, binary, format string) (*wp25PublicCountFixture, string, string, string) {
	t.Helper()
	f := wp25PublicCountNew(t, binary, "native-closure-"+format)
	if format == "sha256" {
		if err := os.RemoveAll(filepath.Join(f.repo, ".git")); err != nil {
			t.Fatal(err)
		}
		f.git(nil, "init", "-q", "--initial-branch=main", "--object-format=sha256")
		f.git(nil, "-c", "user.name=Native closure fixture", "-c", "user.email=fixture@hn.invalid", "commit", "-qm", "fixture", "--allow-empty")
	}
	// Command-lock creation is legitimate setup, not an effect of the observed
	// raw read. No private ledger, authority or registration is manufactured.
	wp25PublicCountCall(t, f, "native-closure-command-lock-setup", false, false, time.Minute, "contribution", "pending")
	seed := sha256.Sum256([]byte("WP25 native closure independent signer"))
	key := ed25519.NewKeyFromSeed(seed[:])
	public := key.Public().(ed25519.PublicKey)
	actorHash := sha256.Sum256(public)
	signer := wp25PublicGraphSigner{hex.EncodeToString(actorHash[:]), public, key}
	project := wp25NativeDigest([]byte("WP25 native closure public raw scope"))
	authority := wp25PublicGraphAuthority{f.git(nil, "rev-parse", "HEAD"), wp25NativeDigest([]byte("inert raw authority binding"))}
	head := wp25PublicGraphManifestBuild(t, f, project, authority, signer, 0, 1, "native-closure-one-record")
	id := ""
	for _, value := range head.expected {
		id = value
	}
	if id == "" {
		t.Fatal("independent signed selection empty")
	}
	return f, project, id, head.tip
}

func nativeClosurePeerInstall(t *testing.T, f *wp25PublicCountFixture, head, mode string) (string, string) {
	t.Helper()
	actualGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	objects := map[string]map[string]any{}
	roots := strings.Fields(head)
	ids := []string{}
	if len(roots) > 0 {
		ids = strings.Fields(f.git(nil, append([]string{"rev-list", "--objects", "--no-object-names"}, roots...)...))
	}
	sort.Strings(ids)
	for _, oid := range ids {
		kind := f.git(nil, "cat-file", "-t", oid)
		cmd := exec.Command(actualGit, "cat-file", kind, oid)
		cmd.Dir, cmd.Env = f.repo, f.env
		raw, e := cmd.Output()
		if e != nil {
			t.Fatal(e)
		}
		objects[oid] = map[string]any{"kind": kind, "size": len(raw), "body": base64.StdEncoding.EncodeToString(raw)}
	}
	root := t.TempDir()
	ledger, state := filepath.Join(root, "native-peer.jsonl"), filepath.Join(root, "state.json")
	targetRole := "contribution"
	if len(roots) > 1 {
		targetRole = "retained"
	}
	configuration := map[string]any{"git": actualGit, "mode": mode, "ledger": ledger, "state": state, "objects": objects, "target_role": targetRole}
	if mode == "body-real-truncated" {
		if targetRole != "retained" || len(ids) == 0 {
			t.Fatal("real truncation must target retained BODY only")
		}
		oid := ids[0]
		objectDir := filepath.Join(f.repo, ".git", "objects", oid[:2])
		_, dirErr := os.Stat(objectDir)
		if dirErr != nil && !os.IsNotExist(dirErr) {
			t.Fatal(dirErr)
		}
		if err := os.MkdirAll(objectDir, 0700); err != nil {
			t.Fatal(err)
		}
		path, backup := filepath.Join(objectDir, oid[2:]), filepath.Join(root, "original-native-object")
		originalInfo, statErr := os.Lstat(path)
		if statErr != nil && !os.IsNotExist(statErr) {
			t.Fatal(statErr)
		}
		configuration["fault_path"], configuration["fault_backup"] = path, backup
		configuration["fault_original_present"] = statErr == nil
		if statErr == nil {
			if !originalInfo.Mode().IsRegular() {
				t.Fatal("unexpected fixture object shape", originalInfo.Mode())
			}
			configuration["fault_original_mode"] = uint32(originalInfo.Mode().Perm())
			configuration["fault_original_digest"] = wp25Digest(t, path)
			if err := os.Link(path, backup); err != nil {
				t.Fatal("fixture hardlink preserving original inode", err)
			}
		}
		encoded, _ := json.Marshal(configuration)
		var fault nativeClosurePeerConfig
		if json.Unmarshal(encoded, &fault) != nil {
			t.Fatal("fixture restoration configuration")
		}
		t.Cleanup(func() {
			err := nativeClosurePeerRestoreFault(fault)
			var same bool
			if statErr == nil {
				restored, readErr := os.Stat(path)
				same = readErr == nil && os.SameFile(originalInfo, restored)
				if !same {
					t.Errorf("fixture original inode not restored: %v", readErr)
				}
			} else {
				_, readErr := os.Lstat(path)
				same = os.IsNotExist(readErr)
				if !same {
					t.Errorf("fixture override remains: %v", readErr)
				}
			}
			if os.IsNotExist(dirErr) {
				_ = os.Remove(objectDir)
			}
			wp25Record(t, map[string]any{"control": "native-real-git-truncation-fixture-cleanup", "original_present": statErr == nil, "original_inode_or_absence_restored": same, "restore_error": fmt.Sprint(err), "source_assisted": true})
			if err != nil {
				t.Errorf("fixture restoration: %v", err)
			}
		})
	}
	raw, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.json")
	if err = os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(root, "git")
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	configuredRace, raceSet := os.LookupEnv("GORACE")
	prefix := "#!/bin/sh\nunset GORACE\n"
	if raceSet {
		prefix = "#!/bin/sh\nexport GORACE=" + quote(configuredRace) + "\n"
	}
	script := prefix + "exec " + quote(self) + " '-test.run=^TestNativeContributionClosurePeer$' -- " + quote(config) + " \"$@\"\n"
	if err = os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for i, value := range f.env {
		if strings.HasPrefix(value, "PATH=") {
			f.env[i] = "PATH=" + root + string(os.PathListSeparator) + strings.TrimPrefix(value, "PATH=")
		}
	}
	wp25Record(t, map[string]any{"control": "native-closure-peer-configuration", "source_assisted": true, "candidate_observation": false, "mode": mode, "configuration_raw": string(raw), "wrapper_raw": script, "wrapper_sha256": wp25Digest(t, wrapper), "actual_git": actualGit, "actual_git_sha256": wp25Digest(t, actualGit), "configured_gorace_set": raceSet, "configured_gorace": configuredRace, "forwarded_gorace_set": raceSet, "forwarded_gorace": configuredRace, "separate_native_policy": "stderr discarded without cap; not artifact transport", "expected_sorted_oids": ids})
	t.Cleanup(func() {
		raw, readErr := os.ReadFile(ledger)
		wp25Record(t, map[string]any{"control": "native-closure-peer-ledger-final", "mode": mode, "raw": string(raw), "read_error": fmt.Sprint(readErr), "source_assisted": mode != "real-git"})
	})
	t.Cleanup(func() { nativeClosureReleaseDescriptorHolders(t, ledger) })
	return ledger, state
}

// This source-assisted protocol helper is inert in ordinary source test runs.
// Configured invocations exit directly so testing output cannot enter Git stdout.
func TestNativeContributionClosurePeer(t *testing.T) {
	if len(os.Args) < 4 || os.Args[1] != "-test.run=^TestNativeContributionClosurePeer$" || os.Args[2] != "--" {
		t.Skip("source-assisted native closure peer; requires explicit fixture configuration")
	}
	raw, err := os.ReadFile(os.Args[3])
	if err != nil {
		os.Exit(72)
	}
	var cfg nativeClosurePeerConfig
	if json.Unmarshal(raw, &cfg) != nil || cfg.Git == "" || cfg.Ledger == "" || cfg.State == "" {
		os.Exit(73)
	}
	if len(os.Args) == 6 && (os.Args[4] == "--stderr-holder" || os.Args[4] == "--stdout-holder") {
		descriptor := strings.TrimSuffix(strings.TrimPrefix(os.Args[4], "--"), "-holder")
		os.Exit(nativeClosurePeerDescriptorHolder(cfg, os.Args[3], os.Args[5], descriptor))
	}
	os.Exit(nativeClosurePeerRun(cfg, os.Args[3], os.Args[4:]))
}

type nativeClosurePeerConfig struct {
	Git                  string `json:"git"`
	Mode                 string `json:"mode"`
	Ledger               string `json:"ledger"`
	State                string `json:"state"`
	TargetRole           string `json:"target_role"`
	FaultPath            string `json:"fault_path"`
	FaultBackup          string `json:"fault_backup"`
	FaultOriginalDigest  string `json:"fault_original_digest"`
	FaultOriginalPresent bool   `json:"fault_original_present"`
	FaultOriginalMode    uint32 `json:"fault_original_mode"`
	Objects              map[string]struct {
		Kind string `json:"kind"`
		Size int    `json:"size"`
		Body []byte `json:"body"`
	} `json:"objects"`
}

type nativeClosurePeerState struct {
	Invocation  int    `json:"invocation"`
	Role        string `json:"role"`
	MetadataPID *int   `json:"metadata_pid,omitempty"`
	Session     int    `json:"session,omitempty"`
}

func nativeClosurePeerLog(cfg nativeClosurePeerConfig, event string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["event"], fields["pid"], fields["ppid"] = event, os.Getpid(), os.Getppid()
	fields["mode"], fields["at_ns"] = cfg.Mode, time.Now().UnixNano()
	race, raceSet := os.LookupEnv("GORACE")
	fields["gorace_value"], fields["gorace_set"] = race, raceSet
	raw, err := json.Marshal(fields)
	if err != nil {
		os.Exit(74)
	}
	file, err := os.OpenFile(cfg.Ledger, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(74)
	}
	_, err = file.Write(append(raw, '\n'))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		os.Exit(75)
	}
}

func nativeClosurePeerLoad(cfg nativeClosurePeerConfig) nativeClosurePeerState {
	state := nativeClosurePeerState{Role: "contribution"}
	raw, err := os.ReadFile(cfg.State)
	if os.IsNotExist(err) {
		return state
	}
	if err != nil || json.Unmarshal(raw, &state) != nil {
		os.Exit(76)
	}
	return state
}

func nativeClosurePeerSave(cfg nativeClosurePeerConfig, state nativeClosurePeerState) {
	raw, err := json.Marshal(state)
	if err != nil || os.WriteFile(cfg.State, raw, 0600) != nil {
		os.Exit(77)
	}
}

func nativeClosurePeerWrite(file *os.File, raw []byte) {
	if _, err := file.Write(raw); err != nil {
		os.Exit(78)
	}
}

func nativeClosurePeerExec(cfg nativeClosurePeerConfig, argv []string) int {
	if err := syscall.Exec(cfg.Git, append([]string{cfg.Git}, argv...), os.Environ()); err != nil {
		return 79
	}
	return 0
}

func nativeClosurePeerRun(cfg nativeClosurePeerConfig, config string, argv []string) int {
	log := func(event string, fields map[string]any) { nativeClosurePeerLog(cfg, event, fields) }
	if len(argv) >= 4 && reflect.DeepEqual(argv[len(argv)-4:], []string{"rev-list", "--objects", "--no-object-names", "--stdin"}) {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return 80
		}
		state := nativeClosurePeerLoad(cfg)
		state.Invocation++
		state.Role = "contribution"
		// Match bytes.splitlines(): CRLF/CR/LF separate lines and a final
		// line ending does not create an additional line.
		lines := bytes.ReplaceAll(data, []byte("\r\n"), []byte{'\n'})
		lines = bytes.ReplaceAll(lines, []byte{'\r'}, []byte{'\n'})
		if len(bytes.Split(bytes.TrimSuffix(lines, []byte{'\n'}), []byte{'\n'})) > 1 {
			state.Role = "retained"
		}
		nativeClosurePeerSave(cfg, state)
		cmd := exec.Command(cfg.Git, argv...)
		cmd.Stdin = bytes.NewReader(data)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err = cmd.Run()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				return 81
			}
			code = exit.ExitCode()
			if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				code = -int(status.Signal())
			}
		}
		log("enumeration", map[string]any{"argv": argv, "input_raw": string(data), "exit_code": code, "stdout_raw": stdout.String(), "invocation": state.Invocation, "role": state.Role})
		nativeClosurePeerWrite(os.Stdout, stdout.Bytes())
		nativeClosurePeerWrite(os.Stderr, stderr.Bytes())
		return code
	}
	phase := ""
	if len(argv) > 0 {
		switch argv[len(argv)-1] {
		case "--batch-check":
			phase = "metadata"
		case "--batch":
			phase = "body"
		}
	}
	if phase == "" {
		log("git-pass-through", map[string]any{"argv": argv})
		return nativeClosurePeerExec(cfg, argv)
	}
	state := nativeClosurePeerLoad(cfg)
	previous, previousLive := state.MetadataPID, false
	if phase == "body" && previous != nil && *previous != 0 {
		err := syscall.Kill(*previous, 0)
		if err == nil {
			previousLive = true
		} else if !errors.Is(err, syscall.ESRCH) {
			return 82
		}
	}
	if phase == "metadata" {
		pid := os.Getpid()
		state.MetadataPID = &pid
	}
	state.Session++
	nativeClosurePeerSave(cfg, state)
	session := state.Session
	log("start", map[string]any{"phase": phase, "argv": argv, "role": state.Role, "invocation": state.Invocation, "session": session, "previous_metadata_pid": previous, "previous_metadata_live": previousLive})
	if cfg.Mode == "real-git" || state.Role != cfg.TargetRole {
		return nativeClosurePeerExec(cfg, argv)
	}
	if cfg.Mode == "body-real-truncated" {
		if state.Role != "retained" {
			return 90
		}
		if phase == "metadata" {
			return nativeClosurePeerExec(cfg, argv)
		}
		return nativeClosurePeerRealTruncatedBody(cfg, argv)
	}
	stall := func(stage string) {
		challenge, ack := nativeClosureChallengePaths(cfg.Ledger, os.Getpid(), session)
		log("stall-entry", map[string]any{"phase": phase, "stage": stage, "session": session, "role": state.Role, "invocation": state.Invocation, "challenge_path": challenge, "ack_path": ack})
		deadline, lastNonce := time.Now().Add(60*time.Second), ""
		for time.Now().Before(deadline) {
			if raw, err := os.ReadFile(challenge); err == nil {
				var request nativeClosurePhaseEvent
				if json.Unmarshal(raw, &request) == nil && request.Event == "live-challenge" && len(request.Nonce) == 64 && request.Nonce != lastNonce && request.PID == os.Getpid() && request.PPID == os.Getppid() && request.Session == session && request.Mode == cfg.Mode && request.Role == state.Role && request.Phase == phase && request.Stage == stage {
					response := nativeClosurePhaseEvent{Event: "live-ack", PID: os.Getpid(), PPID: os.Getppid(), Session: session, Invocation: state.Invocation, Mode: cfg.Mode, Role: state.Role, Phase: phase, Stage: stage, Nonce: request.Nonce, AtNS: time.Now().UnixNano()}
					encoded, err := json.Marshal(response)
					if err != nil || os.WriteFile(ack, encoded, 0600) != nil {
						os.Exit(97)
					}
					lastNonce = request.Nonce
					log("live-challenge-acknowledged", map[string]any{"phase": phase, "stage": stage, "session": session, "role": state.Role, "invocation": state.Invocation, "nonce": request.Nonce, "ack_raw": string(encoded)})
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if cfg.Mode == phase+"-stderr9000" {
		nativeClosurePeerWrite(os.Stderr, bytes.Repeat([]byte{'e'}, 9000))
	}
	ids := make([]string, 0, len(cfg.Objects))
	for oid := range cfg.Objects {
		ids = append(ids, oid)
	}
	sort.Strings(ids)
	input := bufio.NewReader(os.Stdin)
	for {
		line, err := input.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return 83
		}
		if len(line) == 0 && errors.Is(err, io.EOF) {
			break
		}
		oid := strings.TrimRight(line, "\n")
		obj, found := cfg.Objects[oid]
		log("request", map[string]any{"phase": phase, "role": state.Role, "invocation": state.Invocation, "oid": oid, "session": session})
		if cfg.Mode == phase+"-stall-header" {
			stall("header")
		}
		if !found {
			nativeClosurePeerWrite(os.Stdout, []byte(oid+" missing\n"))
			continue
		}
		kind, size, answer := obj.Kind, strconv.Itoa(obj.Size), oid
		switch cfg.Mode {
		case "metadata-negative":
			size = "-1"
		case "metadata-huge":
			size = "99999999999999999999999999"
		case "metadata-object-over", "metadata-object-over-exit":
			size = "33554433"
		case "metadata-late-object-over":
			if oid == ids[len(ids)-1] {
				size = "33554433"
			}
		case "metadata-aggregate-over":
			if oid == ids[0] {
				size = "33554432"
			}
		case "metadata-noncanonical":
			size = "0" + size
		case "metadata-wrong-oid":
			answer = strings.Repeat("0", len(oid))
		case "metadata-wrong-kind":
			kind = "nonsense"
		case "metadata-header-over":
			answer = strings.Repeat("0", 300)
		}
		if phase == "body" {
			switch cfg.Mode {
			case "body-size-change":
				size = strconv.Itoa(obj.Size + 1)
			case "body-type-change":
				kind = "tag"
			case "body-oid-change":
				answer = strings.Repeat("0", len(oid))
			}
		}
		header := answer + " " + kind + " " + size + "\n"
		if cfg.Mode == phase+"-missing" {
			header = oid + " missing\n"
		}
		nativeClosurePeerWrite(os.Stdout, []byte(header))
		log("header", map[string]any{"phase": phase, "header_raw": header, "oid": oid, "session": session})
		if cfg.Mode == phase+"-missing" {
			continue
		}
		if phase == "body" {
			if cfg.Mode == "body-stall-content" {
				stall("content")
			}
			body := obj.Body
			if cfg.Mode == "body-short-exit0" || cfg.Mode == "body-short-exit23" {
				if len(body) > 0 {
					nativeClosurePeerWrite(os.Stdout, body[:len(body)-1])
				}
				log("short-exit", map[string]any{"phase": phase})
				if strings.HasSuffix(cfg.Mode, "0") {
					return 0
				}
				return 23
			}
			if (cfg.Mode == "body-hash" || cfg.Mode == "body-hash-exit") && len(body) > 0 {
				body = bytes.Clone(body)
				body[0] ^= 1
			}
			framed := append(bytes.Clone(body), '\n')
			if cfg.Mode == "body-lf" {
				framed[len(framed)-1] = 'x'
			}
			nativeClosurePeerWrite(os.Stdout, framed)
		}
		log("response", map[string]any{"phase": phase, "role": state.Role, "invocation": state.Invocation, "oid": oid, "kind": obj.Kind, "size": obj.Size, "session": session})
		if phase == "metadata" && cfg.Mode == "metadata-object-over-exit" || phase == "body" && cfg.Mode == "body-hash-exit" {
			return 23
		}
	}
	log("input-eof", map[string]any{"phase": phase, "role": state.Role, "invocation": state.Invocation, "session": session})
	if cfg.Mode == phase+"-stderr-holder" {
		self, err := filepath.Abs(os.Args[0])
		if err != nil {
			return 84
		}
		child := exec.Command(self, "-test.run=^TestNativeContributionClosurePeer$", "--", config, "--stderr-holder", phase)
		child.Stderr = os.Stderr // stdin/stdout use /dev/null; only stderr stays inherited.
		if err = child.Start(); err != nil {
			return 85
		}
		log("stderr-holder", map[string]any{"phase": phase, "holder_pid": child.Process.Pid, "release": filepath.Join(filepath.Dir(config), "release-holder"), "session": session})
		// The worker exits immediately. The fixture releases this descendant;
		// OS init owns its reap, independently of the native worker's Wait.
		_ = child.Process.Release()
	}
	if cfg.Mode == phase+"-stall-eof" {
		stall("eof")
	}
	if cfg.Mode == phase+"-trailing" {
		nativeClosurePeerWrite(os.Stdout, []byte{'x'})
	}
	if cfg.Mode == phase+"-exit" {
		return 23
	}
	log("normal-exit", map[string]any{"phase": phase, "role": state.Role, "invocation": state.Invocation, "session": session})
	return 0
}

func nativeClosurePeerDescriptorHolder(cfg nativeClosurePeerConfig, config, phase, descriptor string) int {
	release := filepath.Join(filepath.Dir(config), "release-holder")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return 86
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Preserve the holder receipt's distinct schema: it is an explicit exit
	// receipt, not a native worker completion or an OS orphan-reap claim.
	raw, err := json.Marshal(map[string]any{"event": descriptor + "-holder-exit", "pid": os.Getpid(), "ppid": os.Getppid(), "phase": phase, "at_ns": time.Now().UnixNano()})
	if err != nil {
		return 87
	}
	file, err := os.OpenFile(cfg.Ledger, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return 88
	}
	_, err = file.Write(append(raw, '\n'))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return 89
	}
	return 0
}

func nativeClosureReadCall(t *testing.T, f *wp25PublicCountFixture, project, id, head, mode string, success bool) {
	t.Helper()
	ids := strings.Fields(f.git(nil, "rev-list", "--objects", "--no-object-names", head))
	sort.Strings(ids)
	if len(ids) == 0 {
		t.Fatal("independent native closure empty")
	}
	first := ids[0]
	before := f.snapshot()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	args := []string{"contribution", "inspect", "--raw", "--project", project, id}
	cmd := exec.CommandContext(ctx, f.binary, args...)
	cmd.Dir, cmd.Env = f.repo, f.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err := cmd.Run()
	commandElapsed := time.Since(start)
	after := f.snapshot()
	wp25Record(t, map[string]any{"control": "native-closure-public-read", "mode": mode, "candidate_observation": true, "binary": f.binary, "binary_sha256": wp25Digest(t, f.binary), "argv": args, "cwd": f.repo, "stdout_raw": stdout.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "watchdog_error": fmt.Sprint(ctx.Err()), "command_elapsed_nanoseconds": int64(commandElapsed), "protected_before": before, "protected_after": after})
	if !reflect.DeepEqual(before, after) {
		t.Fatal("native raw read changed protected state")
	}
	if success {
		var response struct {
			Raw       bool           `json:"raw"`
			ID        string         `json:"id"`
			Payload   map[string]any `json:"payload"`
			Signature []byte         `json:"signature"`
		}
		if err != nil || json.Unmarshal(stdout.Bytes(), &response) != nil || !response.Raw || response.ID != id || response.Payload["project"] != project || len(response.Signature) != 64 {
			t.Fatalf("native public positive: %v stdout=%s stderr=%s", err, &stdout, &stderr)
		}
	} else {
		phase := strings.Split(mode, "-")[0]
		want := "contribution raw " + phase + " finalization failed: "
		if strings.HasSuffix(mode, "-exit") {
			want += "exit status 23"
		} else {
			want += "trailing stdout"
		}
		switch mode {
		case "metadata-missing", "metadata-negative", "metadata-huge", "metadata-object-over", "metadata-late-object-over", "metadata-object-over-exit", "metadata-aggregate-over":
			want = "Git byte budget exceeded"
		case "metadata-header-over":
			want = "contribution raw metadata protocol failed for " + first + ": bufio: buffer full"
		case "metadata-noncanonical":
			want = "contribution raw metadata protocol failed for " + first + ": noncanonical object size"
		case "metadata-wrong-oid":
			want = "contribution raw metadata protocol failed for " + first + ": invalid object header"
		case "metadata-wrong-kind":
			want = "contribution raw metadata protocol failed for " + first + ": invalid object type"
		case "body-size-change", "body-type-change", "body-oid-change":
			want = "contribution raw body protocol failed for " + first + ": object header changed"
		case "body-lf":
			want = "contribution raw body protocol failed for " + first + ": object body separator"
		case "body-short-exit0":
			want = "contribution raw body protocol failed for " + first + ": unexpected EOF"
		case "body-hash", "body-hash-exit":
			want = "contribution raw object hash mismatch: " + first
		case "body-missing":
			want = "bounded Git " + first + " failed: missing contribution object"
		case "body-short-exit23":
			want = "bounded Git " + first + " failed: exit status 23"
		}
		if err == nil || stdout.Len() != 0 || stderr.String() != "hn: "+want+"\n" {
			t.Fatalf("native public expected %q: %v stdout=%s stderr=%s", want, err, &stdout, &stderr)
		}
	}
}

func TestNativeContributionClosureProtocolCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			for _, mode := range []string{"real-git", "pass", "metadata-exit", "metadata-trailing", "body-exit", "body-trailing", "body-short-exit0", "metadata-stderr9000", "body-stderr9000", "metadata-missing", "metadata-negative", "metadata-huge", "metadata-object-over", "metadata-aggregate-over", "metadata-noncanonical", "metadata-wrong-oid", "metadata-wrong-kind", "body-missing", "body-size-change", "body-type-change", "body-oid-change", "body-hash", "body-lf", "body-short-exit23", "metadata-header-over", "metadata-late-object-over", "metadata-object-over-exit", "body-hash-exit"} {
				t.Run(mode, func(t *testing.T) {
					f, project, id, head := nativeClosureRawFixture(t, binary, format)
					ledger, _ := nativeClosurePeerInstall(t, f, head, mode)
					nativeClosureReadCall(t, f, project, id, head, mode, mode == "real-git" || mode == "pass" || strings.Contains(mode, "stderr9000"))
					raw, err := os.ReadFile(ledger)
					if err != nil {
						t.Fatal(err)
					}
					wp25Record(t, map[string]any{"control": "native-closure-peer-ledger", "source_assisted": mode != "real-git", "mode": mode, "raw": string(raw)})
					starts := []map[string]any{}
					for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
						var row map[string]any
						if json.Unmarshal(line, &row) != nil {
							t.Fatal("peer ledger invalid")
						}
						if row["event"] == "start" {
							starts = append(starts, row)
						}
					}
					want := 2
					if strings.HasPrefix(mode, "metadata-") && !strings.Contains(mode, "stderr9000") {
						want = 1
					}
					if len(starts) != want || starts[0]["phase"] != "metadata" {
						t.Fatal("native worker count/order", string(raw))
					}
					if want == 2 && (starts[1]["phase"] != "body" || starts[1]["previous_metadata_live"] != false || starts[1]["previous_metadata_pid"] != starts[0]["pid"]) {
						t.Fatal("metadata worker not reaped before body", string(raw))
					}
					nativeClosureAssertLedger(t, f, head, mode, raw)

				})
			}
		})
	}
}

type nativeClosurePhaseEvent struct {
	Event, Phase, Stage, Mode, Role string
	PID, PPID, Session, Invocation  int
	AtNS                            int64  `json:"at_ns"`
	Nonce                           string `json:"nonce,omitempty"`
	ChallengePath                   string `json:"challenge_path,omitempty"`
	AckPath                         string `json:"ack_path,omitempty"`
}

func nativeClosureObservedRead(t *testing.T, f *wp25PublicCountFixture, project, id, head, mode, ledger string) {
	t.Helper()
	parts := strings.Split(mode, "-")
	phase, stage := parts[0], parts[2]
	session := 1
	if phase == "body" {
		session = 2
	}
	before := f.snapshot()
	watchdog, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	args := []string{"contribution", "inspect", "--raw", "--project", project, id}
	cmd := exec.CommandContext(watchdog, f.binary, args...)
	cmd.Dir, cmd.Env = f.repo, f.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	candidatePID := cmd.Process.Pid
	processDone, observerDone := make(chan struct{}), make(chan struct{})
	observed := make(chan map[string]any, 1)
	go func() {
		defer close(observerDone)
		limit, tick := time.NewTimer(5*time.Second), time.NewTicker(5*time.Millisecond)
		defer limit.Stop()
		defer tick.Stop()
		liveProof := map[string]any{"peer_live_acknowledged": false}
		finish := func(marker nativeClosurePhaseEvent, raw, failure string) {
			alive := cmd.Process.Signal(syscall.Signal(0)) == nil
			select {
			case <-processDone:
				alive = false
			default:
			}
			observed <- map[string]any{"expected_phase": phase, "expected_stage": stage, "expected_session": session, "expected_mode": mode, "candidate_pid": candidatePID, "candidate_alive": alive, "peer_liveness": liveProof, "marker": marker, "marker_raw": raw, "observed_at": time.Now().UTC().Format(time.RFC3339Nano), "observed_elapsed_nanoseconds": int64(time.Since(start)), "error": failure}
		}
		for {
			select {
			case <-processDone:
				finish(nativeClosurePhaseEvent{}, "", "candidate exited before early phase")
				return
			case <-limit.C:
				finish(nativeClosurePhaseEvent{}, "", "required native phase missing within5s")
				return
			case <-tick.C:
				raw, err := os.ReadFile(ledger)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					finish(nativeClosurePhaseEvent{}, "", err.Error())
					return
				}
				for _, line := range bytes.Split(raw, []byte{'\n'}) {
					var marker nativeClosurePhaseEvent
					if json.Unmarshal(line, &marker) != nil || marker.Event != "stall-entry" {
						continue
					}
					failure, matched := "", false
					for _, entry := range bytes.Split(raw, []byte{'\n'}) {
						var actual nativeClosurePhaseEvent
						if json.Unmarshal(entry, &actual) == nil && actual.Event == "start" && actual.PID == marker.PID && actual.PPID == marker.PPID && actual.Session == marker.Session && actual.Phase == marker.Phase && actual.Role == marker.Role {
							matched = true
						}
					}
					if !matched || marker.PID <= 0 || marker.PPID != candidatePID || marker.Phase != phase || marker.Stage != stage || marker.Session != session || marker.Mode != mode || marker.Role != "contribution" || marker.AtNS <= 0 || marker.AtNS > time.Now().UnixNano() || time.Since(start) >= 5*time.Second {
						failure = "wrong native phase/mode/PID/PPID/session witness"
					}
					if failure != "" {
						finish(marker, string(line), failure)
						return
					}
					challenge, ack := nativeClosureChallengePaths(ledger, marker.PID, marker.Session)
					if marker.ChallengePath != challenge || marker.AckPath != ack {
						finish(marker, string(line), "wrong native challenge scope")
						return
					}
					nonce := make([]byte, 32)
					if _, err := rand.Read(nonce); err != nil {
						finish(marker, string(line), err.Error())
						return
					}
					request := marker
					request.Event = "live-challenge"
					request.Nonce = hex.EncodeToString(nonce)
					request.AtNS = time.Now().UnixNano()
					encoded, err := json.Marshal(request)
					if err != nil {
						finish(marker, string(line), err.Error())
						return
					}
					if err = os.WriteFile(challenge, encoded, 0600); err != nil {
						finish(marker, string(line), err.Error())
						return
					}
					liveProof = map[string]any{"peer_live_acknowledged": false, "challenge_raw": string(encoded), "challenge_path": challenge, "ack_path": ack, "challenge_written_at_ns": request.AtNS}
					for {
						select {
						case <-processDone:
							finish(marker, string(line), "candidate exited before live peer ACK")
							return
						case <-limit.C:
							finish(marker, string(line), "native peer liveness challenge unacknowledged within5s")
							return
						case <-tick.C:
							rawAck, readErr := os.ReadFile(ack)
							if os.IsNotExist(readErr) {
								continue
							}
							if readErr != nil {
								finish(marker, string(line), readErr.Error())
								return
							}
							var response nativeClosurePhaseEvent
							if json.Unmarshal(rawAck, &response) != nil {
								continue
							}
							if response.Event != "live-ack" || response.Nonce != request.Nonce || response.PID != marker.PID || response.PPID != candidatePID || response.Session != marker.Session || response.Invocation != marker.Invocation || response.Mode != mode || response.Role != marker.Role || response.Phase != phase || response.Stage != stage || response.AtNS < request.AtNS || response.AtNS > time.Now().UnixNano() || time.Since(start) >= 5*time.Second {
								finish(marker, string(line), "wrong live peer nonce/PID/PPID/session/phase ACK")
								return
							}
							liveProof["peer_live_acknowledged"] = true
							liveProof["ack_raw"] = string(rawAck)
							liveProof["ack_observed_at_ns"] = time.Now().UnixNano()
							finish(marker, string(line), "")
							return
						}
					}
				}
			}
		}
	}()
	err := cmd.Wait()
	commandElapsed := time.Since(start)
	close(processDone)
	witness := <-observed
	<-observerDone
	after := f.snapshot()
	wp25Record(t, map[string]any{"control": "native-closure-pre-deadline-phase", "mode": mode, "candidate": f.binary, "candidate_sha256": wp25Digest(t, f.binary), "candidate_pid": candidatePID, "argv": args, "cwd": f.repo, "stdout_raw": stdout.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "watchdog_error": fmt.Sprint(watchdog.Err()), "command_elapsed_nanoseconds": int64(commandElapsed), "phase_observation": witness, "observer_joined": true, "protected_before": before, "protected_after": after, "product_milliseconds": 30000})
	proof, proofOK := witness["peer_liveness"].(map[string]any)
	if witness["error"] != "" || witness["candidate_alive"] != true || !proofOK || proof["peer_live_acknowledged"] != true {
		t.Fatal("native early phase witness failed", witness)
	}
	if watchdog.Err() != nil || err == nil || stdout.Len() != 0 || commandElapsed < 29*time.Second || commandElapsed > 45*time.Second || !reflect.DeepEqual(before, after) {
		t.Fatal("native cancellation/outcome/effects", err, watchdog.Err(), commandElapsed, &stdout, &stderr)
	}
	ids := strings.Fields(f.git(nil, "rev-list", "--objects", "--no-object-names", head))
	sort.Strings(ids)
	want := "bounded Git --batch-check=%(objectsize) failed: signal: killed"
	if phase == "body" {
		want = "bounded Git " + ids[0] + " failed: signal: killed"
	}
	if stage == "eof" {
		want = "contribution raw " + phase + " finalization failed: signal: killed"
	}
	if stderr.String() != "hn: "+want+"\n" {
		t.Fatal("native parent cancellation diagnostic", want, stderr.String())
	}
	raw, readErr := os.ReadFile(ledger)
	if readErr != nil {
		t.Fatal(readErr)
	}
	counts := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var row map[string]any
		if json.Unmarshal(line, &row) != nil {
			t.Fatal("native cancellation ledger invalid")
		}
		worker, _ := row["phase"].(string)
		event, _ := row["event"].(string)
		counts[worker+":"+event]++
		if event == "start" {
			pid, _ := row["pid"].(float64)
			process, _ := os.FindProcess(int(pid))
			if process == nil || process.Signal(syscall.Signal(0)) == nil {
				t.Fatal("native worker not reaped", row)
			}
		}
	}
	wantMeta := len(ids)
	if phase == "metadata" && stage == "header" {
		wantMeta = 1
	}
	if counts["metadata:request"] != wantMeta {
		t.Fatal("metadata stage request count", counts)
	}
	if phase == "metadata" {
		if counts["body:start"] != 0 || counts["metadata:start"] != 1 {
			t.Fatal("body started before metadata admission/finish", counts)
		}
		if stage == "header" && (counts["metadata:header"] != 0 || counts["metadata:response"] != 0 || counts["metadata:input-eof"] != 0) {
			t.Fatal("metadata header stall stage", counts)
		}
		if stage == "eof" && (counts["metadata:response"] != len(ids) || counts["metadata:input-eof"] != 1) {
			t.Fatal("metadata finalization stage", counts)
		}
	} else {
		if counts["metadata:response"] != len(ids) || counts["metadata:input-eof"] != 1 || counts["metadata:normal-exit"] != 1 || counts["body:start"] != 1 {
			t.Fatal("metadata not completed before body", counts)
		}
		if stage == "header" && (counts["body:request"] != 1 || counts["body:header"] != 0) {
			t.Fatal("body header stage", counts)
		}
		if stage == "content" && (counts["body:request"] != 1 || counts["body:header"] != 1 || counts["body:response"] != 0) {
			t.Fatal("body content stage", counts)
		}
		if stage == "eof" && (counts["body:request"] != len(ids) || counts["body:response"] != len(ids) || counts["body:input-eof"] != 1) {
			t.Fatal("body finalization stage", counts)
		}
	}
	wp25Record(t, map[string]any{"control": "native-closure-cancellation-stage-counts", "mode": mode, "counts": counts, "all_workers_reaped": true, "observer_joined": true})
}

func TestNativeContributionClosureCancellationCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, mode := range []string{"metadata-stall-header", "metadata-stall-eof", "body-stall-header", "body-stall-content", "body-stall-eof"} {
		t.Run(mode, func(t *testing.T) {
			f, project, id, head := nativeClosureRawFixture(t, binary, "sha1")
			ledger, _ := nativeClosurePeerInstall(t, f, head, mode)
			nativeClosureObservedRead(t, f, project, id, head, mode, ledger)
		})
	}
}

// Direct tool progress is separate from the supplied candidate's public reads:
// both requests receive exact real Git replies while stdin remains open.
func TestNativeContributionClassicGitProgress(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f, _, id, head := nativeClosureRawFixture(t, binary, format)
			oid := f.git(nil, "rev-parse", head+":"+id[7:]+".payload")
			actualGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			read := exec.Command(actualGit, "--git-dir="+filepath.Join(f.repo, ".git"), "cat-file", "blob", oid)
			read.Env = f.env
			body, err := read.Output()
			if err != nil {
				t.Fatal(err)
			}
			version := f.git(nil, "--version")
			for _, mode := range []string{"--batch-check", "--batch"} {
				t.Run(mode, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, actualGit, "--git-dir="+filepath.Join(f.repo, ".git"), "cat-file", mode)
					cmd.Env = f.env
					input, err := cmd.StdinPipe()
					if err != nil {
						t.Fatal(err)
					}
					output, err := cmd.StdoutPipe()
					if err != nil {
						input.Close()
						t.Fatal(err)
					}
					cmd.Stderr = io.Discard
					if err = cmd.Start(); err != nil {
						input.Close()
						output.Close()
						t.Fatal(err)
					}
					waited := false
					defer func() {
						cancel()
						input.Close()
						output.Close()
						if !waited {
							_ = cmd.Process.Kill()
							_ = cmd.Wait()
						}
					}()
					header := []byte(fmt.Sprintf("%s blob %d\n", oid, len(body)))
					want := bytes.Clone(header)
					if mode == "--batch" {
						want = append(want, body...)
						want = append(want, '\n')
					}
					replies := []string{}
					for request := 0; request < 2; request++ {
						if _, err = io.WriteString(input, oid+"\n"); err != nil {
							t.Fatal(err)
						}
						actual := make([]byte, len(want))
						if _, err = io.ReadFull(output, actual); err != nil || !bytes.Equal(actual, want) {
							t.Fatal("real Git did not reply with open stdin", err, string(actual))
						}
						replies = append(replies, base64.StdEncoding.EncodeToString(actual))
					}
					if err = input.Close(); err != nil {
						t.Fatal(err)
					}
					extra, readErr := io.ReadAll(output)
					err = cmd.Wait()
					waited = true
					wp25Record(t, map[string]any{"control": "native-classic-real-git-incremental-progress", "tool_control_only": true, "candidate_observation": false, "git": actualGit, "git_sha256": wp25Digest(t, actualGit), "git_version": version, "object_format": format, "argv": cmd.Args, "pid": cmd.Process.Pid, "ppid": os.Getpid(), "exact_oid_requests_before_stdin_close": []string{oid, oid}, "raw_replies_base64_before_stdin_close": replies, "body_sha256": wp25NativeDigest(body), "extra_stdout_base64": base64.StdEncoding.EncodeToString(extra), "read_error": fmt.Sprint(readErr), "wait_error": fmt.Sprint(err), "process_success": cmd.ProcessState.Success(), "scope": "two real classic tool replies before closing stdin; supplied candidate behavior has its separate public controls"})
					if err != nil || readErr != nil || len(extra) != 0 || ctx.Err() != nil {
						t.Fatal("real Git finalization", err, readErr, len(extra), ctx.Err())
					}
				})
			}
		})
	}
}

func nativeClosureRegisterFixture(t *testing.T, binary string) (*wp25PublicCountFixture, string, string, string, []byte) {
	t.Helper()
	f := wp25PublicCountNew(t, binary, "native-closure-register")
	wp25PublicCountCall(t, f, "native-register-setup-init", false, true, time.Minute, "init", "--name", "Native closure maintainer")
	identity := wp25PublicCountCall(t, f, "native-register-setup-identity", true, true, time.Minute, "identity", "show", "--json")
	var who struct {
		OK   bool `json:"ok"`
		Data struct {
			Actor string `json:"actor"`
		} `json:"data"`
	}
	if json.Unmarshal(identity, &who) != nil || !who.OK || len(who.Data.Actor) != 64 {
		t.Fatal("real registration maintainer", string(identity))
	}
	seed := sha256.Sum256([]byte("WP25 native retained independent moderator"))
	key := ed25519.NewKeyFromSeed(seed[:])
	public := key.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(public)
	signer := wp25PublicGraphSigner{hex.EncodeToString(sum[:]), public, key}
	project := wp25NativeDigest([]byte("WP25 native retained closure scope"))
	limits := map[string]any{"wireBytes": 16777216, "decodedBytes": 33554432, "objects": 4096, "events": 2048, "depth": 256, "milliseconds": 30000, "window": map[string]any{"bytes": 268435456, "objects": 32768, "events": 16384, "requests": 256}, "retained": map[string]any{"bytes": 1073741824, "objects": 131072, "events": 65536, "requests": 1024}}
	policy := map[string]any{"version": "hn.policy/0", "maintainers": []string{who.Data.Actor}, "proposals": map[string]any{"requiredAccepts": 1, "trustedReviewers": []string{}}, "pipelines": map[string]any{}, "contributions": map[string]any{"project": project, "moderators": []string{signer.actor}, "rules": []any{}, "baseline": []string{}, "limits": limits}}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(f.repo, ".hn"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(f.repo, ".hn", "policy.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	f.git(nil, "add", ".hn/policy.json")
	f.git(nil, "-c", "user.name=Native retained fixture", "-c", "user.email=fixture@hn.invalid", "commit", "-qm", "explicit retained policy")
	authority := wp25PublicGraphAuthority{f.git(nil, "rev-parse", "HEAD"), wp25NativeDigest(raw)}
	wp25PublicCountCall(t, f, "native-register-setup-authority", false, true, time.Minute, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	head := wp25PublicGraphManifestBuild(t, f, project, authority, signer, 0, 1, "native-retained-one-record")
	roots := []string{authority.Commit, head.tip}
	sort.Strings(roots)
	plan := struct {
		Version   string                   `json:"version"`
		Project   string                   `json:"project"`
		Authority wp25PublicGraphAuthority `json:"authority"`
		Native    []string                 `json:"native"`
		Heads     []string                 `json:"heads"`
		Roots     []string                 `json:"roots"`
	}{"hn.contribution-retained-plan/1", project, authority, nil, []string{head.tip}, roots}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	wp25Record(t, map[string]any{"control": "native-retained-independent-setup", "source_assisted": true, "candidate_observation": false, "policy_raw": string(raw), "plan_raw": string(planRaw), "plan_digest": wp25NativeDigest(planRaw), "roots": roots, "signed_head": head.tip, "all_limits_literal_defaults": true})
	return f, project, head.tip, strings.Join(roots, " "), planRaw
}

func nativeClosureRegisterCall(t *testing.T, f *wp25PublicCountFixture, expected, mode string, success bool, expectedOID, ledger string) []byte {
	t.Helper()
	ledgerBefore, readErr := os.ReadFile(ledger)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	before := f.snapshot()
	refs := f.git(nil, "show-ref")
	args := []string{"contribution", "retained", "register", "--expect", expected}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.binary, args...)
	cmd.Dir, cmd.Env = f.repo, f.env
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	after := f.snapshot()
	wp25Record(t, map[string]any{"control": "native-retained-public-call", "mode": mode, "binary": f.binary, "binary_sha256": wp25Digest(t, f.binary), "argv": args, "cwd": f.repo, "stdout_raw": out.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "watchdog_error": fmt.Sprint(ctx.Err()), "command_elapsed_nanoseconds": int64(elapsed), "protected_before": before, "protected_after": after, "public_refs_before_raw": refs, "public_refs_after_raw": f.git(nil, "show-ref"), "allowed_failure_effects": "only existing reservation/journal/descriptor/lock; no coverage or public/ODB/signer/authority change"})
	if ctx.Err() != nil || (err == nil) != success || refs != f.git(nil, "show-ref") {
		t.Fatal("retained caller outcome/ref effects", err, ctx.Err(), &out, &stderr)
	}
	if !success {
		want := "retained raw metadata finalization failed: exit status 23"
		if mode == "body-exit" {
			want = "retained raw body finalization failed: exit status 23"
		}
		if mode == "body-missing" {
			want = "bounded Git " + expectedOID + " failed: retained object unavailable"
		}
		if mode == "body-real-truncated" {
			want = "bounded Git " + expectedOID + " failed: exit status 128"
		}
		if out.Len() != 0 || stderr.String() != "hn: "+want+"\n" {
			t.Fatal("retained exact native failure", want, &out, &stderr)
		}
	}
	ledgerAfter := wp25Read(t, ledger)
	if len(ledgerAfter) < len(ledgerBefore) || !bytes.Equal(ledgerAfter[:len(ledgerBefore)], ledgerBefore) {
		t.Fatal("registration ledger lost prefix")
	}
	nativeClosureRegistrationStages(t, mode, ledgerAfter[len(ledgerBefore):])
	return bytes.Clone(out.Bytes())
}

func nativeClosureRetainedJournal(t *testing.T, f *wp25PublicCountFixture, project, expected string, failed bool) map[string]any {
	t.Helper()
	root := filepath.Join(f.repo, ".git", "hn", "contribution", "inbox-"+project[7:])
	raw, err := os.ReadFile(filepath.Join(root, "journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	var journal contributionJournal
	if json.Unmarshal(raw, &journal) != nil || len(journal.Attempts) != 1 || len(journal.Ledger.Reservations) != 1 {
		t.Fatal("retained exact one durable charge", string(raw))
	}
	a := journal.Attempts[0]
	r := journal.Ledger.Reservations[0]
	if a.Intent != "retained-"+expected[7:] || a.Artifact != expected || r.Intent != a.Intent || r.Artifact != expected || r.Window != "initial" || a.Reserved != r.Cost || a.Reserved.WireBytes != 16777216 || a.Reserved.DecodedBytes != 33554432 || a.Reserved.Objects != 4096 || a.Reserved.Events != 2048 || a.Reserved.Depth != 256 || a.Reserved.Milliseconds != 30000 {
		t.Fatal("retained reservation identity/cost", string(raw))
	}
	for _, usage := range []struct{ Bytes, Objects, Events, Requests int64 }{{journal.Ledger.WindowUsed.Bytes, journal.Ledger.WindowUsed.Objects, journal.Ledger.WindowUsed.Events, journal.Ledger.WindowUsed.Requests}, {journal.Ledger.RetainedUsed.Bytes, journal.Ledger.RetainedUsed.Objects, journal.Ledger.RetainedUsed.Events, journal.Ledger.RetainedUsed.Requests}} {
		if usage.Bytes != 50335744 || usage.Objects != 4096 || usage.Events != 2048 || usage.Requests != 1 {
			t.Fatal("retained failure refunded/duplicated charge", usage)
		}
	}
	descriptor, err := os.ReadFile(filepath.Join(root, "retained-"+expected[7:]+".json"))
	if err != nil || wp25NativeDigest(descriptor) != expected {
		t.Fatal("immutable retained descriptor", err)
	}
	_, coverageErr := os.Stat(filepath.Join(root, "coverage-"+expected[7:]+".json"))
	if failed {
		if a.Status != "reserved" || a.DecodedBytes != 0 || a.Objects != 0 || len(a.ManifestWrites) != 0 || !os.IsNotExist(coverageErr) {
			t.Fatal("failed raw read escaped coverage/measured success", string(raw), coverageErr)
		}
	} else if a.Status != "failed" || a.DecodedBytes <= 0 || a.Objects <= 0 || coverageErr != nil {
		t.Fatal("successful explicit retained registration state", string(raw), coverageErr)
	}
	wp25Record(t, map[string]any{"control": "native-retained-private-accounting", "failed_raw_stage": failed, "journal_raw": string(raw), "descriptor_raw": string(descriptor), "coverage_error": fmt.Sprint(coverageErr), "one_exact_charge": true, "expected_plan": expected})
	return map[string]any{"ledger": journal.Ledger, "descriptor": string(descriptor)}
}

func TestNativeRetainedClosurePublicCLI(t *testing.T) {
	t.Run("exact-current-authority-guards", nativeExactCurrentAuthorityGuards)
	binary := extensionTestBinary(t)
	for _, mode := range []string{"pass", "metadata-exit", "body-exit", "body-missing", "body-real-truncated", "metadata-stderr9000", "body-stderr9000"} {
		t.Run(mode, func(t *testing.T) {
			f, project, head, roots, plan := nativeClosureRegisterFixture(t, binary)
			ids := strings.Fields(f.git(nil, append([]string{"rev-list", "--objects", "--no-object-names"}, strings.Fields(roots)...)...))
			sort.Strings(ids)
			first := ids[0]
			ledger, state := nativeClosurePeerInstall(t, f, roots, mode)
			expected := wp25NativeDigest(plan)
			success := mode == "pass" || strings.Contains(mode, "stderr9000")
			before := f.snapshot()
			receipt := nativeClosureRegisterCall(t, f, expected, mode, success, first, ledger)
			after := f.snapshot()
			privatePrefix := "/repo/.git/hn/contribution/inbox-" + project[7:] + "/"
			for path, value := range before {
				if !strings.HasPrefix(path, privatePrefix) && after[path] != value {
					t.Fatal("retained call changed forbidden source/key/ODB/ref state", path)
				}
			}
			allowed := map[string]bool{"journal.json": true, "lock": true, "retained-" + expected[7:] + ".json": true}
			if success {
				allowed["coverage-"+expected[7:]+".json"] = true
			}
			for path := range after {
				if _, ok := before[path]; !ok {
					if !strings.HasPrefix(path, privatePrefix) || !allowed[strings.TrimPrefix(path, privatePrefix)] {
						t.Fatal("retained call created forbidden state", path)
					}
				}
			}
			account := nativeClosureRetainedJournal(t, f, project, expected, !success)
			if mode == "body-real-truncated" {
				raw := wp25Read(t, ledger)
				found := 0
				for _, line := range bytes.Split(raw, []byte{'\n'}) {
					var event map[string]any
					if json.Unmarshal(line, &event) == nil && event["event"] == "real-git-body-fault" {
						found++
						if event["actual_exit_code"] != float64(128) || event["fault_oid"] != first || event["restored"] != true || event["real_git_wait_reaped"] != true || !strings.HasPrefix(event["actual_stdout_raw"].(string), first+" ") {
							t.Fatal("actual truncated producer/status/identity/restoration missing", event)
						}
					}
				}
				if found != 1 {
					t.Fatal("real truncated retained BODY control did not execute", string(raw))
				}
			}
			if !success {
				raw, err := os.ReadFile(ledger)
				if err != nil {
					t.Fatal(err)
				}
				var cfg map[string]any
				cfgPath := filepath.Join(filepath.Dir(ledger), "config.json")
				if json.Unmarshal(wp25Read(t, cfgPath), &cfg) != nil {
					t.Fatal("peer config")
				}
				if mode == "body-missing" {
					requests := []string{}
					for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
						var event map[string]any
						if json.Unmarshal(line, &event) == nil && event["role"] == "retained" && event["phase"] == "body" && event["event"] == "request" {
							requests = append(requests, event["oid"].(string))
						}
					}
					ids := strings.Fields(f.git(nil, append([]string{"rev-list", "--objects", "--no-object-names"}, strings.Fields(roots)...)...))
					sort.Strings(ids)
					if len(requests) != 1 || requests[0] != ids[0] {
						t.Fatal("missing retained body must bind first admitted exact OID", requests, ids)
					}
				}
				cfg["mode"] = "pass"
				encoded, _ := json.Marshal(cfg)
				if os.WriteFile(cfgPath, encoded, 0600) != nil {
					t.Fatal("recovery config")
				}
				if os.Remove(state) != nil {
					t.Fatal("reset test-only per-command peer state")
				}
				receipt = nativeClosureRegisterCall(t, f, expected, "recovery", true, first, ledger)
				recovered := nativeClosureRetainedJournal(t, f, project, expected, false)
				if !reflect.DeepEqual(account, recovered) {
					t.Fatal("identical recovery changed reservation/descriptor", account, recovered)
				}
			}
			var got struct {
				Registered string   `json:"registered"`
				Native     int      `json:"native"`
				Heads      []string `json:"heads"`
			}
			if json.Unmarshal(receipt, &got) != nil || got.Registered != expected || got.Native != 0 || !reflect.DeepEqual(got.Heads, []string{head}) {
				t.Fatal("independent public retained receipt", string(receipt))
			}
			beforeReplay := f.snapshot()
			nativeClosureRegisterCall(t, f, expected, "duplicate", true, first, ledger)
			if !reflect.DeepEqual(beforeReplay, f.snapshot()) {
				t.Fatal("retained replay duplicated state/charge")
			}
		})
	}
}

func nativeClosureGitData(t *testing.T, f *wp25PublicCountFixture, input []byte, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env, cmd.Stdin = f.repo, f.env, bytes.NewReader(input)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("source-assisted Git %q: %v %s", args, err, raw)
	}
	return raw
}

func TestNativeContributionLegacyFaultsCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			for _, mode := range []string{"truncated-blob", "missing-blob", "payload-tree", "signature-tree", "payload-commit", "wrong-mode", "signature65", "signature-invalid", "payload65537"} {
				t.Run(mode, func(t *testing.T) {
					f, project, id, head := nativeClosureRawFixture(t, binary, format)
					payloadName, signatureName := id[7:]+".payload", id[7:]+".signature"
					payloadOID := f.git(nil, "rev-parse", head+":"+payloadName)
					want := ""
					if mode == "truncated-blob" || mode == "missing-blob" {
						path := filepath.Join(f.repo, ".git", "objects", payloadOID[:2], payloadOID[2:])
						body := nativeClosureGitData(t, f, nil, "cat-file", "blob", payloadOID)
						var compressed bytes.Buffer
						w := zlib.NewWriter(&compressed)
						fmt.Fprintf(w, "blob %d\x00", len(body))
						w.Write(body)
						w.Close()
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						_ = os.Remove(path)
						if mode == "truncated-blob" {
							if err := os.WriteFile(path, compressed.Bytes()[:compressed.Len()-2], 0444); err != nil {
								t.Fatal(err)
							}
							want = "bounded Git " + payloadOID + " failed: exit status 128"
						} else {
							want = "bounded Git --stdin failed: exit status 128"
						}
					} else {
						type entry struct{ mode, oid string }
						entries := map[string]entry{}
						for _, row := range strings.Split(f.git(nil, "ls-tree", "-z", head), "\x00") {
							if row == "" {
								continue
							}
							metadata, name, _ := strings.Cut(row, "\t")
							parts := strings.Fields(metadata)
							entries[name] = entry{parts[0], parts[2]}
						}
						writeBlob := func(raw []byte) string {
							return strings.TrimSpace(string(nativeClosureGitData(t, f, raw, "hash-object", "-w", "--stdin")))
						}
						switch mode {
						case "payload-tree":
							entries[payloadName] = entry{"100644", f.git(nil, "rev-parse", "HEAD^{tree}")}
						case "signature-tree":
							entries[signatureName] = entry{"100644", f.git(nil, "rev-parse", "HEAD^{tree}")}
						case "payload-commit":
							entries[payloadName] = entry{"100644", f.git(nil, "rev-parse", "HEAD")}
							want = "missing native dependency"
						case "wrong-mode":
							v := entries[payloadName]
							v.mode = "100755"
							entries[payloadName] = v
							want = "unsupported contribution manifest tree"
						case "signature65":
							raw := nativeClosureGitData(t, f, nil, "cat-file", "blob", entries[signatureName].oid)
							entries[signatureName] = entry{"100644", writeBlob(append(raw, 'x'))}
						case "signature-invalid":
							raw := nativeClosureGitData(t, f, nil, "cat-file", "blob", entries[signatureName].oid)
							raw[0] ^= 1
							entries[signatureName] = entry{"100644", writeBlob(raw)}
							want = "invalid contribution signature"
						case "payload65537":
							raw := nativeClosureGitData(t, f, nil, "cat-file", "blob", payloadOID)
							raw = append(raw, bytes.Repeat([]byte{' '}, 65537-len(raw))...)
							entries[payloadName] = entry{"100644", writeBlob(raw)}
						}
						names := []string{}
						for name := range entries {
							names = append(names, name)
						}
						sort.Strings(names)
						var tree bytes.Buffer
						for _, name := range names {
							v := entries[name]
							rawOID, err := hex.DecodeString(v.oid)
							if err != nil {
								t.Fatal(err)
							}
							fmt.Fprintf(&tree, "%s %s\x00", v.mode, name)
							tree.Write(rawOID)
						}
						treeOID := strings.TrimSpace(string(nativeClosureGitData(t, f, tree.Bytes(), "hash-object", "--literally", "-t", "tree", "-w", "--stdin")))
						head = f.git(nil, "-c", "user.name=Native fault fixture", "-c", "user.email=fixture@hn.invalid", "commit-tree", treeOID, "-m", "explicit single fault")
						refs := strings.Fields(f.git(nil, "for-each-ref", "--format=%(refname)", "refs/hn/contributions"))
						if len(refs) != 1 {
							t.Fatal(refs)
						}
						f.git(nil, "update-ref", refs[0], head)
						if mode == "signature65" {
							want = "bounded Git " + head + ":" + signatureName + " failed: Git output budget exceeded"
						}
						if mode == "payload65537" {
							want = "bounded Git " + head + ":" + payloadName + " failed: Git output budget exceeded"
						}
						if mode == "payload-tree" {
							want = "invalid envelope JSON: invalid character 'e' in literal true (expecting 'u')"
						}
						if mode == "signature-tree" {
							formatted := nativeClosureGitData(t, f, nil, "show", head+":"+signatureName)
							want = "invalid contribution signature"
							if len(formatted) > 64 {
								want = "bounded Git " + head + ":" + signatureName + " failed: Git output budget exceeded"
							}
						}
					}
					ledger, _ := nativeClosurePeerInstall(t, f, "", "real-git")
					before := f.snapshot()
					args := []string{"contribution", "inspect", "--raw", "--project", project, id}
					cmd := exec.Command(binary, args...)
					cmd.Dir, cmd.Env = f.repo, f.env
					var out, stderr bytes.Buffer
					cmd.Stdout, cmd.Stderr = &out, &stderr
					err := cmd.Run()
					after := f.snapshot()
					wp25Record(t, map[string]any{"control": "native-legacy-public-single-fault", "mode": mode, "object_format": format, "binary": binary, "binary_sha256": wp25Digest(t, binary), "argv": args, "cwd": f.repo, "stdout_raw": out.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "expected_complete_stderr": "hn: " + want + "\n", "protected_before": before, "protected_after": after})
					if err == nil || out.Len() != 0 || stderr.String() != "hn: "+want+"\n" || !reflect.DeepEqual(before, after) {
						t.Fatal("native sole-fault boundary differs", want, err, &out, &stderr)
					}
					if mode == "truncated-blob" || mode == "missing-blob" {
						log := string(wp25Read(t, ledger))
						if strings.Contains(log, "\"show\"") {
							t.Fatal("raw failure reached semantic show", log)
						}
						return
					}
					log := string(wp25Read(t, ledger))
					roleShow := strings.Contains(log, head+":"+payloadName) || strings.Contains(log, head+":"+signatureName)
					if (mode == "payload-tree" || mode == "signature-tree") && !roleShow {
						t.Fatal("non-blob original semantic role show not observed", log)
					}
					if mode != "payload-tree" && mode != "signature-tree" && roleShow {
						t.Fatal("ordinary blob/error unexpectedly reread role", log)
					}
				})
			}
		})
	}
}

// This ledger assertion binds actual request identities and completion, rather
// than inferring a full reader from a worker's ordinal position.
func nativeClosureAssertLedger(t *testing.T, f *wp25PublicCountFixture, head, mode string, raw []byte) {
	t.Helper()
	if mode == "real-git" { // exec replaces the logging peer; real responses are checked by public output.
		return
	}
	ids := strings.Fields(f.git(nil, "rev-list", "--objects", "--no-object-names", head))
	sort.Strings(ids)
	requests := map[string][]string{}
	counts := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var row map[string]any
		if json.Unmarshal(line, &row) != nil {
			t.Fatal("native request ledger invalid")
		}
		phase, _ := row["phase"].(string)
		event, _ := row["event"].(string)
		counts[phase+":"+event]++
		if event == "request" {
			requests[phase] = append(requests[phase], row["oid"].(string))
		}
		if event == "start" {
			race, raceSet := os.LookupEnv("GORACE")
			if row["gorace_set"] != raceSet || row["gorace_value"] != race {
				t.Fatal("test peer dropped configured race harness option", row, race, raceSet)
			}
			pid := int(row["pid"].(float64))
			process, _ := os.FindProcess(pid)
			if process == nil || process.Signal(syscall.Signal(0)) == nil {
				t.Fatal("native worker not reaped", row)
			}
		}
	}
	metadataN, bodyN := len(ids), len(ids)
	if strings.HasPrefix(mode, "metadata-") && !strings.Contains(mode, "stderr9000") {
		bodyN = 0
		metadataN = 1
		if mode == "metadata-exit" || mode == "metadata-trailing" || mode == "metadata-late-object-over" {
			metadataN = len(ids)
		}
		if mode == "metadata-aggregate-over" {
			metadataN = 2
		}
	}
	if strings.HasPrefix(mode, "body-") && mode != "body-exit" && mode != "body-trailing" && mode != "body-stderr9000" {
		bodyN = 1
	}
	if !reflect.DeepEqual(requests["metadata"], ids[:metadataN]) || (bodyN == 0 && len(requests["body"]) != 0) || (bodyN > 0 && !reflect.DeepEqual(requests["body"], ids[:bodyN])) {
		t.Fatal("native exact sorted requests or first-fault stopping", mode, requests, ids, metadataN, bodyN)
	}
	if bodyN > 0 && (counts["metadata:response"] != len(ids) || counts["metadata:input-eof"] != 1 || counts["metadata:normal-exit"] != 1) {
		t.Fatal("body began without complete metadata EOF/normal-exit", counts)
	}
	if mode == "pass" || strings.Contains(mode, "stderr9000") {
		for _, phase := range []string{"metadata", "body"} {
			if counts[phase+":header"] != len(ids) || counts[phase+":response"] != len(ids) || counts[phase+":input-eof"] != 1 || counts[phase+":normal-exit"] != 1 {
				t.Fatal("native successful full phase omitted a request/header/body/finish", phase, counts)
			}
		}
	}
	wp25Record(t, map[string]any{"control": "native-closure-exact-request-and-first-error-ledger", "mode": mode, "expected_sorted_oids": ids, "metadata_requests": metadataN, "body_requests": bodyN, "counts": counts, "all_workers_reaped": true})
}

// A controlled descendant retains stderr after the actual worker exits. The
// native caller must join its discard drainer without waiting for that unrelated
// descriptor owner. The fixture releases and observes the holder on all paths.
func TestNativeContributionStderrHolderCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, phase := range []string{"metadata", "body"} {
		t.Run(phase, func(t *testing.T) {
			f, project, id, head := nativeClosureRawFixture(t, binary, "sha1")
			ledger, _ := nativeClosurePeerInstall(t, f, head, phase+"-stderr-holder")
			release := filepath.Join(filepath.Dir(ledger), "release-holder")
			released := false
			releaseHolder := func() {
				if !released {
					_ = os.WriteFile(release, []byte("release\n"), 0600)
					released = true
				}
			}
			t.Cleanup(releaseHolder)
			nativeClosureReadCall(t, f, project, id, head, phase+"-stderr-holder", true)
			raw := wp25Read(t, ledger)
			holder := 0
			for _, line := range bytes.Split(raw, []byte{'\n'}) {
				var event map[string]any
				if json.Unmarshal(line, &event) == nil && event["event"] == "stderr-holder" {
					holder = int(event["holder_pid"].(float64))
				}
			}
			process, _ := os.FindProcess(holder)
			if holder <= 0 || process == nil || process.Signal(syscall.Signal(0)) != nil {
				t.Fatal("candidate did not return while the controlled stderr holder was alive", string(raw))
			}
			wp25Record(t, map[string]any{"control": "native-stderr-holder-public-return", "phase": phase, "holder_pid": holder, "holder_still_alive_on_success": true, "raw": string(raw)})
			releaseHolder()
			limit := time.Now().Add(5 * time.Second)
			for {
				raw = wp25Read(t, ledger)
				found := false
				for _, line := range bytes.Split(raw, []byte{'\n'}) {
					var event map[string]any
					if json.Unmarshal(line, &event) == nil && event["event"] == "stderr-holder-exit" && int(event["pid"].(float64)) == holder {
						found = true
					}
				}
				if found {
					break
				}
				if time.Now().After(limit) {
					t.Fatal("controlled holder did not observe fixture release", string(raw))
				}
				time.Sleep(5 * time.Millisecond)
			}
			wp25Record(t, map[string]any{"control": "native-stderr-holder-fixture-release", "phase": phase, "holder_pid": holder, "raw": string(raw), "os_orphan_reaping": "OS init owns holder reap; explicit exit receipt retained; native worker reaped by caller"})
		})
	}
}

// These are private supplied-limit and descriptor-ownership tests of the linked
// helper, not claims of successful public admission at a new numeric limit.
func TestNativeContributionPrivateHelperBounds(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, mode := range []string{"empty-roots", "count-first-over", "object-equality", "object-first-over", "aggregate-equality", "aggregate-first-over", "framed-equality", "framed-first-over", "canceled-role"} {
		t.Run(mode, func(t *testing.T) {
			f, _, _, head := nativeClosureRawFixture(t, binary, "sha1")
			ids := strings.Fields(f.git(nil, "rev-list", "--objects", "--no-object-names", head))
			sort.Strings(ids)
			var total, largest int64
			last := int64(0)
			prefix := int64(len(projectGitSchema))
			for i, oid := range ids {
				kind := f.git(nil, "cat-file", "-t", oid)
				size, err := strconv.ParseInt(f.git(nil, "cat-file", "-s", oid), 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				total += size
				largest = max(largest, size)
				if i == len(ids)-1 {
					last = size
					break
				}
				prefix += int64(len(fmt.Sprintf("%s %s %d\n", oid, kind, size))) + size + 1
			}
			limits := migration.Limits{GitObjects: len(ids), GitObjectBytes: largest, GitTotalBytes: total}
			framed := prefix + last + 256
			roots := []string{head}
			want := ""
			switch mode {
			case "empty-roots":
				roots = nil
			case "count-first-over":
				limits.GitObjects--
				want = "Git traversal budget exceeded"
			case "object-first-over":
				limits.GitObjectBytes--
				want = "Git byte budget exceeded"
			case "aggregate-first-over":
				limits.GitTotalBytes--
				want = "Git byte budget exceeded"
			case "framed-first-over":
				framed--
				want = "contribution ref byte limit"
			}
			ledger, _ := nativeClosurePeerInstall(t, f, head, "pass")
			for _, value := range f.env {
				if strings.HasPrefix(value, "PATH=") {
					t.Setenv("PATH", strings.TrimPrefix(value, "PATH="))
				}
			}
			before := f.snapshot()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			material, err := contributionRawClosure(ctx, filepath.Join(f.repo, ".git"), roots, limits, framed, "contribution")
			if want == "" && err != nil || want != "" && (err == nil || err.Error() != want) {
				t.Fatal("private supplied boundary", mode, err, want)
			}
			if !reflect.DeepEqual(before, f.snapshot()) {
				t.Fatal("private helper changed source state")
			}
			raw, readErr := os.ReadFile(ledger)
			if mode == "empty-roots" {
				if !os.IsNotExist(readErr) || material == nil || string(material.Bytes()) != projectGitSchema || len(material.objects) != 0 {
					t.Fatal("empty selection executed Git or returned nonempty material", readErr, material)
				}
			} else if readErr != nil {
				t.Fatal(readErr)
			}
			counts := map[string]int{}
			for _, line := range bytes.Split(raw, []byte{'\n'}) {
				var event map[string]any
				if json.Unmarshal(line, &event) == nil {
					counts[fmt.Sprint(event["phase"])+":"+fmt.Sprint(event["event"])]++
				}
			}
			if want == "Git byte budget exceeded" && counts["body:start"] != 0 {
				t.Fatal("metadata bound started body", counts)
			}
			if mode == "count-first-over" && counts["metadata:start"] != 0 {
				t.Fatal("count bound started metadata", counts)
			}
			if mode == "framed-first-over" && counts["body:request"] != len(ids)-1 {
				t.Fatal("framed refusal did not precede exact last request", counts, ids)
			}
			if mode == "canceled-role" {
				roleCtx, roleCancel := context.WithCancel(context.Background())
				roleCancel()
				_, roleErr := material.selected(roleCtx, filepath.Join(f.repo, ".git"), ids[0], "preserved-role", largest)
				if roleErr == nil || roleErr.Error() != "bounded Git preserved-role failed: context canceled" {
					t.Fatal("indexed role lost caller cancellation admission", roleErr)
				}
			}
			wp25Record(t, map[string]any{"control": "native-private-helper-boundary", "source_assisted_linked_helper": true, "mode": mode, "limits": limits, "framed_limit": framed, "expected_error": want, "actual_error": fmt.Sprint(err), "counts": counts, "peer_raw": string(raw), "public_limit_claim": false})
		})
	}
}

// Descriptor exhaustion is isolated to a compiled-self subprocess. It cannot
// change the runner's own descriptor limits or leave a reduced limit behind.
func TestNativeContributionWorkerSetupOwnership(t *testing.T) {
	if os.Getenv("HN_WP25_NATIVE_SETUP_CHILD") == "1" {
		var original syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
			t.Fatal(err)
		}
		missing := t.TempDir()
		t.Setenv("PATH", missing)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := contributionClosureStart(ctx, missing, "--batch-check"); !errors.Is(err, context.Canceled) {
			t.Fatal("canceled setup", err)
		}
		warmR, warmW, warmErr := os.Pipe()
		if warmErr != nil {
			t.Fatal(warmErr)
		}
		warmR.Close()
		warmW.Close()
		fdNames := func() []string {
			observer, err := os.Open("/dev/fd")
			if err != nil {
				t.Fatal(err)
			}
			names, readErr := observer.Readdirnames(-1)
			closeErr := observer.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			sort.Strings(names)
			return names
		}
		baseline := fdNames()
		stable := fdNames()
		if !reflect.DeepEqual(baseline, stable) {
			t.Fatal("descriptor observer baseline unstable", baseline, stable)
		}
		t.Logf("descriptor detector stable baseline=%v repeated=%v", baseline, stable)
		held, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if held != nil {
				if err := held.Close(); err != nil {
					t.Error("close held descriptor after failed detector", err)
				}
			}
		}()
		withHeld := fdNames()
		if reflect.DeepEqual(baseline, withHeld) {
			t.Fatal("descriptor detector missed held descriptor", baseline, withHeld)
		}
		t.Logf("descriptor detector held baseline=%v held=%v", baseline, withHeld)
		closeErr := held.Close()
		held = nil
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		recovered := fdNames()
		if !reflect.DeepEqual(baseline, recovered) {
			t.Fatal("descriptor detector did not recover", baseline, recovered)
		}
		t.Logf("descriptor detector recovered baseline=%v recovered=%v", baseline, recovered)
		for _, free := range []int{0, 2, 4, 6} {
			before := fdNames()
			limit := original
			limit.Cur = min(limit.Cur, 64)
			if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
				t.Fatal(err)
			}
			opened := []*os.File{}
			for {
				f, err := os.Open(os.DevNull)
				if err != nil {
					if !errors.Is(err, syscall.EMFILE) {
						t.Fatal(err)
					}
					break
				}
				opened = append(opened, f)
			}
			if len(opened) < free {
				t.Fatal("insufficient isolated descriptors for setup control")
			}
			for i := 0; i < free; i++ {
				opened[len(opened)-1].Close()
				opened = opened[:len(opened)-1]
			}
			process, err := contributionClosureStart(context.Background(), missing, "--batch-check")
			for _, f := range opened {
				f.Close()
			}
			restore := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original)
			if restore != nil {
				t.Fatal(restore)
			}
			if process != nil {
				process.abort()
				t.Fatal("unexpected child in partial-pipe/Start failure", free)
			}
			if err == nil {
				t.Fatal("setup fault not reached", free)
			}
			after := fdNames()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("partially acquired descriptors leaked", free, before, after)
			}
			t.Logf("setup free=%d expected-error=%v descriptor-before=%v descriptor-after=%v", free, err, before, after)
		}
		return
	}
	harness, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, harness, "-test.run=^TestNativeContributionWorkerSetupOwnership$", "-test.v", "-test.count=1", "-test.timeout=20s")
	cmd.Env = append(os.Environ(), "HN_WP25_NATIVE_SETUP_CHILD=1")
	raw, err := cmd.CombinedOutput()
	wp25Record(t, map[string]any{"control": "native-worker-isolated-descriptor-setup", "source_assisted_linked_helper": true, "harness": harness, "harness_sha256": wp25Digest(t, harness), "argv": cmd.Args, "stdout_stderr_raw": string(raw), "process_error": fmt.Sprint(err), "watchdog_error": fmt.Sprint(ctx.Err()), "rlimit_scope": "compiled-self subprocess only; caller unchanged"})
	if err != nil || ctx.Err() != nil || strings.Count(string(raw), "--- PASS: TestNativeContributionWorkerSetupOwnership") != 1 || strings.Contains(string(raw), "--- SKIP:") || strings.Contains(string(raw), "--- FAIL:") {
		t.Fatal("isolated worker setup child did not execute and pass", err, string(raw))
	}
}

func nativeClosureRegistrationStages(t *testing.T, mode string, raw []byte) {
	t.Helper()
	reads := []map[string]any{}
	counts := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			t.Fatal("registration stage ledger invalid", string(raw))
		}
		role, _ := event["role"].(string)
		phase, _ := event["phase"].(string)
		name, _ := event["event"].(string)
		counts[role+":"+phase+":"+name]++
		if name == "enumeration" && role == "contribution" {
			reads = append(reads, event)
		}
		if name == "start" {
			process, _ := os.FindProcess(int(event["pid"].(float64)))
			if process == nil || process.Signal(syscall.Signal(0)) == nil {
				t.Fatal("registration worker still live after call", event)
			}
			if phase == "body" && event["previous_metadata_live"] != false {
				t.Fatal("registration overlapping metadata/body", event)
			}
		}
	}
	if len(reads) != 2 || reads[0]["input_raw"] != reads[1]["input_raw"] || reads[0]["stdout_raw"] != reads[1]["stdout_raw"] || reads[0]["exit_code"] != float64(0) || reads[1]["exit_code"] != float64(0) {
		t.Fatal("both independent equal current-plan reads missing", reads)
	}
	for _, phase := range []string{"metadata", "body"} {
		if counts["contribution:"+phase+":start"] != 2 {
			t.Fatal("plan read skipped raw worker", counts)
		}
	}
	retained := 1
	if mode == "duplicate" {
		retained = 0
	}
	if counts["retained::enumeration"] != retained || counts["retained:metadata:start"] != retained {
		t.Fatal("post-reservation raw stage count", mode, counts)
	}
	body := retained
	if mode == "metadata-exit" {
		body = 0
	}
	if counts["retained:body:start"] != body {
		t.Fatal("retained body started before metadata finish or omitted", counts)
	}
	wp25Record(t, map[string]any{"control": "native-retained-two-plan-recaptures-and-raw-stage", "mode": mode, "plan_reads": reads, "counts": counts, "raw": string(raw), "all_native_workers_reaped": true})
}

// Native contribution raw closure keeps its own32MiB supplied bound. A real
// historical9MiB blob exercises acceptance above artifact metadata's8MiB guard;
// this is neither public32MiB equality nor an exchange-wire size claim.
func TestNativeContributionLargeBlobPublicCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f, project, id, head := nativeClosureRawFixture(t, binary, format)
			raw := bytes.Repeat([]byte{'n'}, 9<<20)
			blob := strings.TrimSpace(string(nativeClosureGitData(t, f, raw, "hash-object", "-w", "--stdin")))
			tree := f.git(strings.NewReader("100644 blob "+blob+"\tlarge-history.blob\n"), "mktree")
			parent := f.git(nil, "-c", "user.name=Native large fixture", "-c", "user.email=fixture@hn.invalid", "commit-tree", tree, "-m", "historical nine MiB")
			currentTree := f.git(nil, "rev-parse", head+"^{tree}")
			tip := f.git(nil, "-c", "user.name=Native large fixture", "-c", "user.email=fixture@hn.invalid", "commit-tree", currentTree, "-p", parent, "-m", "signed current manifest and bounded history")
			refs := strings.Fields(f.git(nil, "for-each-ref", "--format=%(refname)", "refs/hn/contributions"))
			if len(refs) != 1 {
				t.Fatal(refs)
			}
			f.git(nil, "update-ref", refs[0], tip)
			if got := f.git(nil, "cat-file", "-s", blob); got != "9437184" {
				t.Fatal("independent large native blob size", got)
			}
			wp25Record(t, map[string]any{"control": "native-large-real-blob-construction", "source_assisted_construction": true, "object_format": format, "blob_oid": blob, "blob_bytes": len(raw), "blob_sha256": wp25NativeDigest(raw), "historical_parent": parent, "selected_head": tip, "selected_record_id": id, "native_decoded_limit": 33554432, "artifact_metadata_limit_is_not_native_policy": 8388608, "exchange_wire_equality_claim": false})
			ledger, _ := nativeClosurePeerInstall(t, f, "", "real-git")
			nativeClosureReadCall(t, f, project, id, tip, "real-git", true)
			entries := wp25Read(t, ledger)
			counts := map[string]int{}
			for _, line := range bytes.Split(entries, []byte{'\n'}) {
				var event map[string]any
				if json.Unmarshal(line, &event) == nil && event["event"] == "start" {
					counts[event["phase"].(string)]++
					if event["phase"] == "body" && event["previous_metadata_live"] != false {
						t.Fatal("large native body began before metadata reap", event)
					}
				}
			}
			if counts["metadata"] != 1 || counts["body"] != 1 {
				t.Fatal("large native closure did not use exactly two classic workers", counts)
			}
			wp25Record(t, map[string]any{"control": "native-large-real-blob-public-positive", "object_format": format, "actual_worker_counts": counts, "peer_raw": string(entries), "selected_id": id, "selected_head": tip, "actual_raw_blob_bytes": 9437184, "scope": "actual native raw inspect succeeds over an independently constructed historical real Git blob; no artifact8MiB policy reuse"})
		})
	}
}

// This test-only fault is strictly after retained metadata admission. Backup
// hardlinks preserve an existing original inode; parent cleanup also restores it
// if the peer is killed. No production diagnostic probe or retry is introduced.
func nativeClosurePeerRestoreFault(cfg nativeClosurePeerConfig) error {
	if cfg.FaultPath == "" {
		return nil
	}
	if cfg.FaultOriginalPresent {
		if _, err := os.Stat(cfg.FaultBackup); err == nil {
			if err = os.Rename(cfg.FaultBackup, cfg.FaultPath); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		info, err := os.Lstat(cfg.FaultPath)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(cfg.FaultPath)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(raw)
		if !info.Mode().IsRegular() || uint32(info.Mode().Perm()) != cfg.FaultOriginalMode || hex.EncodeToString(digest[:]) != cfg.FaultOriginalDigest {
			return errors.New("fixture original bytes/mode not restored")
		}
		return nil
	}
	if err := os.Remove(cfg.FaultPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
func nativeClosurePeerRealTruncatedBody(cfg nativeClosurePeerConfig, argv []string) int {
	if cfg.TargetRole != "retained" || cfg.FaultPath == "" || len(argv) < 1 || argv[len(argv)-1] != "--batch" {
		return 91
	}
	ids := make([]string, 0, len(cfg.Objects))
	for oid := range cfg.Objects {
		ids = append(ids, oid)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return 91
	}
	oid := ids[0]
	object := cfg.Objects[oid]
	if !strings.HasSuffix(cfg.FaultPath, filepath.Join(oid[:2], oid[2:])) {
		return 91
	}
	defer func() { _ = nativeClosurePeerRestoreFault(cfg) }()
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	if _, err := fmt.Fprintf(writer, "%s %d\x00", object.Kind, object.Size); err != nil {
		return 92
	}
	if _, err := writer.Write(object.Body); err != nil {
		return 92
	}
	if writer.Close() != nil || encoded.Len() < 3 {
		return 92
	}
	// Unlink only the disposable fixture pathname; the original inode is held by
	// the parent's hardlink until normal or parent-cleanup restoration.
	if err := os.Remove(cfg.FaultPath); err != nil && !os.IsNotExist(err) {
		return 93
	}
	if os.WriteFile(cfg.FaultPath, encoded.Bytes()[:encoded.Len()-2], 0444) != nil {
		return 93
	}
	cmd := exec.Command(cfg.Git, argv...)
	cmd.Stdin = os.Stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.MultiWriter(os.Stdout, &stdout), io.MultiWriter(os.Stderr, &stderr)
	if cmd.Start() != nil {
		return 94
	}
	pid := cmd.Process.Pid
	err := cmd.Wait()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return 95
		}
		code = exit.ExitCode()
	}
	restoreErr := nativeClosurePeerRestoreFault(cfg)
	nativeClosurePeerLog(cfg, "real-git-body-fault", map[string]any{"fault_oid": oid, "phase": "body", "role": "retained", "actual_git": cfg.Git, "argv": argv, "real_git_pid": pid, "real_git_ppid": os.Getpid(), "real_git_wait_reaped": true, "actual_exit_code": code, "actual_wait_error": fmt.Sprint(err), "actual_stdout_raw": stdout.String(), "actual_stderr_raw": stderr.String(), "restored": restoreErr == nil, "restore_error": fmt.Sprint(restoreErr), "fault": "source-assisted truncated loose-object zlib checksum after real metadata; stdin/stdout/stderr and actual exit forwarded"})
	if restoreErr != nil {
		return 96
	}
	return code
}

func nativeClosureChallengePaths(ledger string, pid, session int) (string, string) {
	prefix := filepath.Join(filepath.Dir(ledger), fmt.Sprintf("live-peer-%d-%d", pid, session))
	return prefix + ".challenge", prefix + ".ack"
}

// This also joins controlled stdout holders used by the marker-then-exit
// negative. Exit receipts are fixture evidence; OS init owns orphan reaping.
func nativeClosureReleaseDescriptorHolders(t *testing.T, ledger string) {
	t.Helper()
	raw, err := os.ReadFile(ledger)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Error(err)
		return
	}
	holders := map[int]string{}
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		var event map[string]any
		if json.Unmarshal(line, &event) == nil && (event["event"] == "stdout-holder" || event["event"] == "stderr-holder") {
			holders[int(event["holder_pid"].(float64))] = event["event"].(string) + "-exit"
		}
	}
	if len(holders) == 0 {
		return
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(ledger), "release-holder"), []byte("release\n"), 0600); err != nil {
		t.Error(err)
		return
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		raw, err = os.ReadFile(ledger)
		if err != nil {
			t.Error(err)
			return
		}
		remaining := map[int]string{}
		for pid, event := range holders {
			remaining[pid] = event
		}
		for _, line := range bytes.Split(raw, []byte{'\n'}) {
			var event map[string]any
			if json.Unmarshal(line, &event) == nil {
				if pid, ok := event["pid"].(float64); ok && event["event"] == remaining[int(pid)] {
					delete(remaining, int(pid))
				}
			}
		}
		if len(remaining) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("controlled descriptor holder did not acknowledge cleanup release: %v", remaining)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	wp25Record(t, map[string]any{"control": "native-descriptor-holder-parent-cleanup-release", "holders": holders, "all_explicit_exit_receipts_observed": true, "raw": string(raw), "os_orphan_reaping": "OS init; not claimed as native Wait ownership"})
}

// These bounded source-assisted controls use real Git data and owned direct
// children. They establish this correction's contracts, not platform qualification.
type intelWitnessPeerConfig struct {
	Git, Root, Mode string
	Sources         []artifactSource
}

type intelWitnessPeerEvent struct {
	Race              string   `json:"peer_gorace"`
	ConfiguredRace    string   `json:"configured_gorace"`
	ConfiguredRaceSet bool     `json:"configured_gorace_set"`
	Event             string   `json:"event"`
	PID               int      `json:"pid"`
	Args              []string `json:"args"`
	Block             int      `json:"block"`
	At                int64    `json:"at"`
	Exit              int      `json:"exit"`
	Raw               string   `json:"raw"`
}

func intelWitnessWriteEvent(c intelWitnessPeerConfig, event intelWitnessPeerEvent) {
	event.Race = os.Getenv("GORACE")
	event.PID = os.Getpid()
	event.At = time.Now().UnixNano()
	f, err := os.OpenFile(filepath.Join(c.Root, "ledger.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}
	err = json.NewEncoder(f).Encode(event)
	ce := f.Close()
	if err != nil || ce != nil {
		panic(fmt.Sprint(err, ce))
	}
}

func intelWitnessPeer() {
	for i, arg := range os.Args {
		if arg != "--intel-witness-peer" {
			continue
		}
		var c intelWitnessPeerConfig
		raw, err := os.ReadFile(os.Args[i+1])
		if err != nil || json.Unmarshal(raw, &c) != nil {
			os.Exit(96)
		}
		configuredSet, configuredRace := os.Args[i+2] != "", os.Args[i+3]
		delegateEnv := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GORACE=") {
				delegateEnv = append(delegateEnv, v)
			}
		}
		if configuredSet {
			delegateEnv = append(delegateEnv, "GORACE="+configuredRace)
		}
		args := os.Args[i+4:]
		block := -1
		primary, fallback := false, false
		for _, a := range args {
			primary = primary || a == "show-ref"
			fallback = fallback || a == "for-each-ref"
		}
		if primary || fallback {
			for n, s := range c.Sources {
				if len(args) > 0 && s.Ref == args[len(args)-1] {
					block = n / 64
					break
				}
			}
		}
		intelWitnessWriteEvent(c, intelWitnessPeerEvent{Event: "start", Args: args, Block: block, ConfiguredRace: configuredRace, ConfiguredRaceSet: configuredSet})
		for _, a := range args {
			if strings.HasPrefix(a, "--batch") {
				if err := syscall.Exec(c.Git, append([]string{c.Git}, args...), delegateEnv); err != nil {
					os.Exit(95)
				}
			}
		}
		cmd := exec.Command(c.Git, args...)
		cmd.Stdin = os.Stdin
		cmd.Env = delegateEnv
		raw, err = cmd.Output()
		exit := 0
		if err != nil {
			exit = 1
			if v, ok := err.(*exec.ExitError); ok {
				exit = v.ExitCode()
			}
		}
		intelWitnessWriteEvent(c, intelWitnessPeerEvent{Event: "git-wait", Args: args, Block: block, Exit: exit, Raw: string(raw)})
		for _, a := range args {
			if c.Mode == "authority-slow-repository" && a == "--absolute-git-dir" || c.Mode == "authority-slow-policy" && a == "show" {
				time.Sleep(250 * time.Millisecond)
			}
		}
		if block >= 0 {
			if primary && block == 0 {
				switch c.Mode {
				case "reverse", "earlier-malformed", "global-changed", "later-failure", "expiry-prefetch", "semantic-cleanup":
					time.Sleep(150 * time.Millisecond)
				}
				if c.Mode == "earlier-malformed" || c.Mode == "semantic-cleanup" {
					raw = []byte("malformed\n")
					exit = 0
				}
				if c.Mode == "global-changed" {
					raw = []byte(strings.Repeat("f", len(c.Sources[0].OID)) + " " + c.Sources[64].Ref + "\nmalformed\n")
					exit = 0
				}
				if c.Mode == "extra-duplicate" {
					raw = append([]byte("\nanything refs/unselected\n"+c.Sources[64].OID+" "+c.Sources[64].Ref+"\n"), raw...)
				}
				if c.Mode == "primary-partial" {
					raw = []byte("malformed\n")
					exit = 1
				}
				if c.Mode == "fallback-held" || c.Mode == "fallback-cancel" || c.Mode == "fallback-fail" || c.Mode == "fallback-overflow" || c.Mode == "fallback-at-bound" || c.Mode == "fallback-sentinel-malformed" || c.Mode == "fallback-below-sentinel-malformed" {
					raw = nil
					exit = 1
				}
				if c.Mode == "primary-overflow" {
					raw = bytes.Repeat([]byte("x"), 64*512+1)
					exit = 0
				}
				if c.Mode == "primary-at-bound" {
					raw = append(raw, bytes.Repeat([]byte(" "), 64*512-len(raw))...)
					exit = 0
				}
			}
			if primary && block == 1 {
				if c.Mode == "earlier-malformed" {
					raw = []byte("later failure ignored\n")
					exit = 1
				}
				if c.Mode == "fallback-held" || c.Mode == "fallback-cancel" || c.Mode == "semantic-cleanup" {
					time.Sleep(400 * time.Millisecond)
				}
				if c.Mode == "later-failure" {
					raw = []byte("later-malformed\n")
					exit = 0
				}
			}
			if fallback && block == 0 {
				if c.Mode == "fallback-sentinel-malformed" || c.Mode == "fallback-below-sentinel-malformed" {
					rows := 65
					if c.Mode == "fallback-below-sentinel-malformed" {
						rows = 64
					}
					raw = []byte("malformed\n" + strings.Repeat("anything refs/unselected\n", rows-1))
					exit = 0
				}
				if c.Mode == "fallback-held" || c.Mode == "fallback-cancel" {
					time.Sleep(150 * time.Millisecond)
				}
				if c.Mode == "fallback-fail" {
					exit = 1
				}
				if c.Mode == "fallback-overflow" {
					raw = bytes.Repeat([]byte("x"), 65*512+1)
					exit = 0
				}
				if c.Mode == "fallback-at-bound" {
					raw = append(raw, bytes.Repeat([]byte(" "), 65*512-len(raw))...)
					exit = 0
				}
			}
		}
		intelWitnessWriteEvent(c, intelWitnessPeerEvent{Event: "end", Args: args, Block: block, Exit: exit, Raw: string(raw)})
		_, _ = os.Stdout.Write(raw)
		os.Exit(exit)
	}
}

func intelWitnessInstall(t *testing.T, c intelWitnessPeerConfig) string {
	t.Helper()
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(c.Root, "config.json")
	if err = os.WriteFile(cfg, raw, 0600); err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	// Only this new race-instrumented peer omits the synthetic runtime exit
	// sleep. Its actual Git delegates receive the original GORACE spelling/set.
	script := "#!/bin/sh\nintel_peer_gorace_set=${GORACE+x}\nintel_peer_gorace=${GORACE-}\nGORACE=\"${intel_peer_gorace} atexit_sleep_ms=0\" exec " + quote(self) + " '-test.run=^TestArtifactStoreExplicitPublicSources$/^witness-pair-wave$' -- --intel-witness-peer " + quote(cfg) + " \"$intel_peer_gorace_set\" \"$intel_peer_gorace\" \"$@\"\n"
	if err = os.WriteFile(filepath.Join(c.Root, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	wp25Record(t, map[string]any{"control": "intel-witness-peer-configuration", "source_assisted": true, "config_raw": string(raw), "script_raw": script, "harness": self, "harness_sha256": wp25Digest(t, self), "git_sha256": wp25Digest(t, c.Git)})
	return c.Root + string(os.PathListSeparator) + os.Getenv("PATH")
}

func intelWitnessLedger(t *testing.T, root string) []intelWitnessPeerEvent {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "ledger.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	rows := []intelWitnessPeerEvent{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var r intelWitnessPeerEvent
		if json.Unmarshal(line, &r) != nil {
			t.Fatal(string(line))
		}
		rows = append(rows, r)
	}
	return rows
}

func intelWitnessFDs(t *testing.T) []string {
	t.Helper()
	f, err := os.Open("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	names, err := f.Readdirnames(-1)
	ce := f.Close()
	if err != nil || ce != nil {
		t.Fatal(err, ce)
	}
	sort.Strings(names)
	return names
}

func intelWitnessResult(t *testing.T, err error, code string) {
	t.Helper()
	if code == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var failure *artifact.Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func artifactWitnessPairWaveContract(t *testing.T) {
	intelWitnessPeer()
	if os.Getenv("HN_INTEL_RESOURCE_CHILD") != "" {
		intelWitnessLowFD(t)
		return
	}
	actualGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			f := &artifactCLIFixture{t: t, repo: filepath.Join(root, "public.git"), home: filepath.Join(root, "home")}
			f.git("init", "--bare", "--quiet", "--object-format="+format)
			commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			sources := []artifactSource{}
			for i := 0; i < 257; i++ {
				ref := fmt.Sprintf("refs/hn/remotes/p%03d/artifacts/", i) + strings.TrimPrefix(artifactFrozenRef(t, f), "refs/hn/artifacts/")
				f.git("update-ref", ref, commit)
				sources = append(sources, artifactSource{ref, commit})
			}
			for _, n := range []int{0, 1, 63, 64, 65, 127, 128, 129, 191, 192, 193, 257} {
				t.Run(fmt.Sprintf("selected-%d", n), func(t *testing.T) {
					before := f.snapshot()
					intelWitnessResult(t, artifactCheckSourceWitnesses(context.Background(), f.repo, sources[:n]), "")
					f.unchanged(before)
				})
			}
			// A full public read proves both witness phases and authentic object checks.
			before := f.snapshot()
			h, err := loadArtifactHistoryFromSources(context.Background(), f.repo, sources[:129])
			if err != nil || len(h.ByID) != 1 {
				t.Fatal(err, h)
			}
			f.unchanged(before)
			for _, mode := range []string{"lowFD", "E2BIG", "missing-exec", "bad-interpreter"} {
				t.Run(mode, func(t *testing.T) {
					c := intelWitnessPeerConfig{actualGit, f.repo, mode, sources[:129]}
					raw, e := json.Marshal(c)
					if e != nil {
						t.Fatal(e)
					}
					cmd := exec.Command(os.Args[0], "-test.v", "-test.run=^TestArtifactStoreExplicitPublicSources$/^witness-pair-wave$")
					cmd.Env = append(os.Environ(), "HN_INTEL_RESOURCE_CHILD="+string(raw))
					out, e := cmd.CombinedOutput()
					wp25Record(t, map[string]any{"control": "intel-witness-resource-child", "source_assisted": true, "mode": mode, "argv": cmd.Args, "stdout_stderr_raw": string(out), "exit_error": fmt.Sprint(e), "fully_wait_reaped": cmd.ProcessState != nil})
					if e != nil || !bytes.Contains(out, []byte("PASS")) || bytes.Contains(out, []byte("FAIL")) || bytes.Contains(out, []byte("SKIP")) {
						t.Fatal("resource child", e, string(out))
					}
				})
			}
			t.Run("completed-success-admission", func(t *testing.T) { intelWitnessCompletedAdmission(t, f, actualGit, sources[:64]) })
			for _, tc := range []struct{ mode, code string }{
				{"reverse", ""}, {"fallback-sentinel-malformed", "resource_limit"}, {"fallback-below-sentinel-malformed", "verification_failure"}, {"semantic-cleanup", "verification_failure"}, {"earlier-malformed", "verification_failure"}, {"global-changed", "concurrent_update"}, {"later-failure", "verification_failure"}, {"extra-duplicate", ""}, {"primary-partial", ""}, {"fallback-held", ""}, {"fallback-fail", "verification_failure"}, {"primary-at-bound", ""}, {"primary-overflow", ""}, {"fallback-at-bound", ""}, {"fallback-overflow", "verification_failure"}, {"expiry-prefetch", "cancelled"}, {"fallback-cancel", "cancelled"},
			} {
				t.Run(tc.mode, func(t *testing.T) {
					peer := t.TempDir()
					path := intelWitnessInstall(t, intelWitnessPeerConfig{actualGit, peer, tc.mode, sources})
					t.Setenv("PATH", path)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					if tc.mode == "expiry-prefetch" || tc.mode == "fallback-cancel" {
						ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
						defer cancel()
					}
					// Warm runtime descriptor initialization before the complete-set baseline.
					_, _ = projectGit(context.Background(), f.repo, nil, 256, "rev-parse", "--git-dir")
					base := intelWitnessFDs(t)
					started := time.Now()
					err := artifactCheckSourceWitnesses(ctx, f.repo, sources)
					elapsed := time.Since(started)
					intelWitnessResult(t, err, tc.code)
					if (tc.mode == "earlier-malformed" || tc.mode == "semantic-cleanup") && err.Error() != "verification_failure: invalid selected artifact source witness" {
						t.Fatal("later failure replaced earlier parser diagnostic", err)
					}
					if got := intelWitnessFDs(t); !reflect.DeepEqual(base, got) {
						t.Fatal("witness FD leak", base, got)
					}
					rows := intelWitnessLedger(t, peer)
					if tc.mode == "fallback-sentinel-malformed" || tc.mode == "fallback-below-sentinel-malformed" {
						want := 65
						if tc.mode == "fallback-below-sentinel-malformed" {
							want = 64
							if err.Error() != "verification_failure: invalid selected artifact source witness" {
								t.Fatal("competing parser refusal not live", err)
							}
						}
						matched := 0
						for _, r := range rows {
							if r.Event == "end" && r.Block == 0 && len(r.Args) > 1 && r.Args[1] == "for-each-ref" {
								matched++
								if r.Exit != 0 || strings.Count(r.Raw, "\n") != want || !strings.HasPrefix(r.Raw, "malformed\n") {
									t.Fatal("successful competing fallback stream", r)
								}
							}
						}
						if matched != 1 {
							t.Fatal("missing successful competing fallback", matched)
						}
					}
					starts, ends := map[int]bool{}, map[int]bool{}
					active, maxActive := 0, 0
					firstEnd := -1
					primaryStarts := 0
					for _, r := range rows {
						if r.Event == "start" {
							starts[r.PID] = true
							active++
							if active > maxActive {
								maxActive = active
							}
							if r.Block >= 0 && len(r.Args) > 1 && r.Args[1] == "show-ref" {
								primaryStarts++
								want := []string{"--git-dir=" + f.repo, "show-ref", "--verify", "--"}
								lo := r.Block * 64
								hi := min(lo+64, len(sources))
								for _, s := range sources[lo:hi] {
									want = append(want, s.Ref)
								}
								if !reflect.DeepEqual(want, r.Args) {
									t.Fatal("changed primary argv", r.Args, want)
								}
							}
						}
						if r.Event == "start" && r.Block >= 0 && len(r.Args) > 1 && r.Args[1] == "for-each-ref" {
							want := []string{"--git-dir=" + f.repo, "for-each-ref", "--count=65", "--format=%(objectname) %(refname)", "--"}
							lo := r.Block * 64
							hi := min(lo+64, len(sources))
							for _, v := range sources[lo:hi] {
								want = append(want, v.Ref)
							}
							if !reflect.DeepEqual(want, r.Args) {
								t.Fatal("changed fallback argv", r.Args, want)
							}
						}
						if r.Event == "end" {
							ends[r.PID] = true
							active--
							if r.Block >= 0 && firstEnd < 0 {
								firstEnd = r.Block
							}
						}
					}
					for pid := range starts {
						if syscall.Kill(pid, 0) == nil {
							t.Fatal("unreaped owned witness child", pid)
						}
					}
					if os.Getenv("HN_INTEL_TEST_STAGE") != "old" {
						if maxActive > 2 {
							t.Fatal("more than two producers", maxActive)
						}
						if tc.mode == "reverse" && (maxActive != 2 || firstEnd != 1 || primaryStarts != 5) {
							t.Fatal("pair-wave overlap/reversed completion", maxActive, firstEnd, primaryStarts)
						}
						if tc.code != "" && tc.mode != "later-failure" && primaryStarts > 2 {
							t.Fatal("next pair after refusal", primaryStarts)
						}
						if tc.mode == "reverse" {
							for b := 2; b < 5; b++ {
								var startAt int64
								priorEnds := map[int]int64{}
								for _, r := range rows {
									if r.Event == "end" && r.Block >= 0 {
										priorEnds[r.Block] = r.At
									}
									if r.Event == "start" && r.Block == b {
										startAt = r.At
									}
								}
								for p := (b/2 - 1) * 2; p < b/2*2; p++ {
									if priorEnds[p] == 0 || priorEnds[p] > startAt {
										t.Fatal("pair replenished before prior release", b, p, rows)
									}
								}
							}
						}
					}
					wp25Record(t, map[string]any{"control": "intel-witness", "source_assisted": true, "mode": tc.mode, "format": format, "error": fmt.Sprint(err), "elapsed_ns": elapsed.Nanoseconds(), "ledger": rows, "max_live_direct_producers": maxActive, "first_finished_block": firstEnd, "FD_before": base, "FD_after": intelWitnessFDs(t), "all_direct_children_absent": true, "stage": os.Getenv("HN_INTEL_TEST_STAGE")})
				})
			}
			// Prefix descendants exercise the real fallback count65 sentinel, even for
			// a final partial group. Failed primary output never reaches the parser.
			t.Run("real-prefix-sentinel", func(t *testing.T) {
				missing := sources[0]
				f.git("update-ref", "-d", missing.Ref)
				for i := 0; i < 65; i++ {
					f.git("update-ref", fmt.Sprintf("%s/child%02d", missing.Ref, i), commit)
				}
				before := f.snapshot()
				intelWitnessResult(t, artifactCheckSourceWitnesses(context.Background(), f.repo, []artifactSource{missing}), "resource_limit")
				f.unchanged(before)
				for i := 0; i < 65; i++ {
					f.git("update-ref", "-d", fmt.Sprintf("%s/child%02d", missing.Ref, i))
				}
				intelWitnessResult(t, artifactCheckSourceWitnesses(context.Background(), f.repo, []artifactSource{missing}), "concurrent_update")
				f.git("update-ref", missing.Ref, commit)
			})
			t.Run("cancelled-entry-no-child", func(t *testing.T) {
				peer := t.TempDir()
				path := intelWitnessInstall(t, intelWitnessPeerConfig{actualGit, peer, "entry", sources})
				t.Setenv("PATH", path)
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				intelWitnessResult(t, artifactCheckSourceWitnesses(ctx, f.repo, nil), "cancelled")
				intelWitnessResult(t, artifactCheckSourceWitnesses(ctx, f.repo, sources), "cancelled")
				if len(intelWitnessLedger(t, peer)) != 0 {
					t.Fatal("cancelled entry launched child")
				}
			})
			t.Run("exact-no-DWIM", func(t *testing.T) {
				wanted := sources[0]
				f.git("update-ref", "-d", wanted.Ref)
				alias := "refs/heads/" + wanted.Ref
				f.git("update-ref", alias, wanted.OID)
				defer f.git("update-ref", wanted.Ref, wanted.OID)
				before := f.snapshot()
				intelWitnessResult(t, artifactCheckSourceWitnesses(context.Background(), f.repo, []artifactSource{wanted}), "concurrent_update")
				f.unchanged(before)
			})
		})
	}
}

func intelWitnessLowFD(t *testing.T) {
	// Only invoked by the separately owned compiled-self child, never in parent.
	var c intelWitnessPeerConfig
	if err := json.Unmarshal([]byte(os.Getenv("HN_INTEL_RESOURCE_CHILD")), &c); err != nil {
		t.Fatal(err)
	}
	_, _ = projectGit(context.Background(), c.Root, nil, 256, "rev-parse", "--git-dir")
	base := intelWitnessFDs(t)
	if c.Mode != "lowFD" {
		switch c.Mode {
		case "E2BIG":
			t.Setenv("HN_INTEL_HUGE_CHILD_ONLY", strings.Repeat("x", 2*1024*1024))
		case "missing-exec":
			t.Setenv("PATH", t.TempDir())
		case "bad-interpreter":
			root := t.TempDir()
			if e := os.WriteFile(filepath.Join(root, "git"), []byte("#!/nonexistent/intel-interpreter\n"), 0700); e != nil {
				t.Fatal(e)
			}
			t.Setenv("PATH", root)
		default:
			t.Fatal("unknown child resource mode")
		}
		err := artifactCheckSourceWitnesses(context.Background(), c.Root, c.Sources)
		intelWitnessResult(t, err, "verification_failure")
		args := []string{"show-ref", "--verify", "--"}
		for _, v := range c.Sources[:64] {
			args = append(args, v.Ref)
		}
		raw, rawErr := projectGit(context.Background(), c.Root, nil, 64*512, args...)
		want := error(exec.ErrNotFound)
		if c.Mode == "E2BIG" {
			want = syscall.E2BIG
		}
		if c.Mode == "bad-interpreter" {
			want = syscall.ENOENT
		}
		if raw != nil || !errors.Is(rawErr, want) {
			t.Fatal("real exec fault provenance", c.Mode, rawErr, want)
		}
		wp25Record(t, map[string]any{"control": "intel-witness-exec-errno", "mode": c.Mode, "actual_error": fmt.Sprint(rawErr), "expected_errno": fmt.Sprint(want), "stdout_nil": raw == nil, "argv": args})
		if got := intelWitnessFDs(t); !reflect.DeepEqual(base, got) {
			t.Fatal("resource setup leaked", base, got)
		}
		wp25Record(t, map[string]any{"control": "intel-witness-child-only-exec-fault", "mode": c.Mode, "error": fmt.Sprint(err), "complete_FD_recovery": true})
		return
	}
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old); err != nil {
		t.Fatal(err)
	}
	reduced := old
	reduced.Cur = 64
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &reduced); err != nil {
		t.Fatal(err)
	}
	held := []*os.File{}
	for {
		f, err := os.Open(os.DevNull)
		if err != nil {
			break
		}
		held = append(held, f)
	}
	err := artifactCheckSourceWitnesses(context.Background(), c.Root, c.Sources)
	args := []string{"show-ref", "--verify", "--"}
	for _, v := range c.Sources[:64] {
		args = append(args, v.Ref)
	}
	raw, rawErr := projectGit(context.Background(), c.Root, nil, 64*512, args...)
	errnoOK := raw == nil && errors.Is(rawErr, syscall.EMFILE)
	restoreErr := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old)
	for _, f := range held {
		if e := f.Close(); e != nil {
			t.Fatal(e)
		}
	}
	if restoreErr != nil {
		t.Fatal(restoreErr)
	}
	intelWitnessResult(t, err, "verification_failure")
	if !errnoOK {
		t.Fatal("real lowFD fault provenance", rawErr)
	}
	wp25Record(t, map[string]any{"control": "intel-witness-exec-errno", "mode": "lowFD", "actual_error": fmt.Sprint(rawErr), "expected_errno": syscall.EMFILE.Error(), "stdout_nil": raw == nil, "argv": args})
	if got := intelWitnessFDs(t); !reflect.DeepEqual(base, got) {
		t.Fatal("lowFD leaked", base, got)
	}
	wp25Record(t, map[string]any{"control": "intel-witness-child-only-lowFD", "error": fmt.Sprint(err), "held": len(held), "complete_FD_recovery": true})
}

func nativeExactCurrentAuthorityGuards(t *testing.T) {
	binary := extensionTestBinary(t)
	t.Run("public-two-plans", func(t *testing.T) { intelAuthorityPublicPlans(t, binary) })
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := artifactCLIFormat(t, format)
			policyRaw := fmt.Sprintf(`{"version":"hn.policy/0","maintainers":[%q],"proposals":{"requiredAccepts":1,"trustedReviewers":[]},"pipelines":{},"contributions":{"project":%q,"moderators":[%q],"rules":[],"baseline":[],"limits":{"wireBytes":16777216,"decodedBytes":33554432,"objects":4096,"events":2048,"depth":256,"milliseconds":30000,"window":{"bytes":268435456,"objects":32768,"events":16384,"requests":256},"retained":{"bytes":1073741824,"objects":131072,"events":65536,"requests":1024}}}}`, f.actor, wp25NativeDigest([]byte("intel authority guards")), f.actor)
			if err := os.MkdirAll(filepath.Join(f.repo, ".hn"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(f.repo, ".hn/policy.json"), []byte(policyRaw), 0600); err != nil {
				t.Fatal(err)
			}
			f.git("add", ".hn/policy.json")
			f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@hn.invalid", "commit", "-qm", "policy")
			f.git("branch", "-M", "main")
			contributionCLI(t, binary, f.repo, true, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
			t.Chdir(f.repo)
			a, err := loadContributionAuthority()
			if err != nil || a == nil {
				t.Fatal(err)
			}
			ref := a.current().Ref
			source := contributionAuthoritySource{a}
			p1, err := source.Historical(ref)
			if err != nil {
				t.Fatal(err)
			}
			p2, err := source.Historical(ref)
			if err != nil || !reflect.DeepEqual(p1, p2) || p1.Policy == p2.Policy || p1.Policy == a.Policy.Contributions {
				t.Fatal("authority must freshly parse each record", err)
			}
			cached := *a
			cached.Policy = a.Policy
			cached.Policy.Maintainers = []string{strings.Repeat("f", 64)}
			fresh, e := (contributionAuthoritySource{&cached}).Historical(ref)
			if e != nil || !reflect.DeepEqual(fresh.Maintainers, p1.Maintainers) {
				t.Fatal("returned cached maintainers", e, fresh)
			}
			legacy := f.git("show", ref.Commit+":.hn/policy.json")
			typed := f.git("show", ref.Commit+"^{commit}:.hn/policy.json")
			if legacy != typed || typed != policyRaw {
				t.Fatal("typed policy expression differs")
			}
			check := func(name string, authority *contributionAuthority, rerr string) {
				t.Helper()
				r := authority.current().Ref
				got, e := (contributionAuthoritySource{authority}).Historical(r)
				if rerr == "" {
					if e != nil {
						t.Fatal(name, e)
					}
				} else if e == nil || e.Error() != rerr {
					t.Fatal(name, "expected", rerr, "got", e, got)
				}
				wp25Record(t, map[string]any{"control": "intel-authority-guard", "source_assisted": true, "case": name, "format": format, "ref": r, "error": fmt.Sprint(e)})
			}
			authorityError := "historical authority unavailable"
			// Obtain the public error spelling from an invalid real object rather than
			// importing another package or weakening the exact error comparisons.
			bad := ref
			bad.Commit = strings.Repeat("1", len(ref.Commit))
			_, badErr := source.Historical(bad)
			if badErr == nil {
				t.Fatal("missing authority accepted")
			}
			authorityError = badErr.Error()
			for _, kind := range []string{"tree", "blob", "tag", "malformed-commit"} {
				t.Run(kind, func(t *testing.T) {
					copy := *a
					switch kind {
					case "tree":
						copy.Commit = f.git("rev-parse", ref.Commit+"^{tree}")
					case "blob":
						copy.Commit = f.git("rev-parse", ref.Commit+":.hn/policy.json")
					case "tag":
						copy.Commit = f.gitInput("object "+ref.Commit+"\ntype commit\ntag authority\ntagger Fixture <fixture@hn.invalid> 0 +0000\n\nfixture\n", "hash-object", "-w", "-t", "tag", "--stdin")
					case "malformed-commit":
						copy.Commit = f.gitInput("tree missing\n\nmalformed\n", "hash-object", "--literally", "-w", "-t", "commit", "--stdin")
					}
					want := authorityError
					if kind == "tag" {
						want = ""
					}
					check(kind, &copy, want)
				})
			}
			for _, which := range []string{"commit", "policy"} {
				for _, fault := range []string{"missing", "corrupt"} {
					t.Run(which+"-"+fault, func(t *testing.T) {
						oid := ref.Commit
						if which == "policy" {
							oid = f.git("rev-parse", ref.Commit+":.hn/policy.json")
						}
						path := filepath.Join(f.repo, ".git/objects", oid[:2], oid[2:])
						saved, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						err = os.Remove(path)
						if err == nil && fault == "corrupt" {
							err = os.WriteFile(path, []byte("corrupt zlib"), 0444)
						}
						if err != nil {
							t.Fatal(err)
						}
						defer func() {
							if fault == "corrupt" {
								if e := os.Remove(path); e != nil {
									t.Fatal(e)
								}
							}
							if e := os.WriteFile(path, saved, 0444); e != nil {
								t.Fatal(e)
							}
						}()
						check(which+"-"+fault, a, authorityError)
					})
				}
			}
			for _, tc := range []struct{ name, raw string }{
				{"trailing", policyRaw + " {}"}, {"shape", `{"version":"wrong"}`}, {"disabled", strings.Replace(policyRaw, `"contributions":`, `"unexpected":`, 1)},
				{"digest", strings.Replace(policyRaw, "intel-no-such-text", "changed", 1) + " "},
			} {
				t.Run(tc.name, func(t *testing.T) {
					blob := f.gitInput(tc.raw, "hash-object", "-w", "--stdin")
					sub := f.gitInput("100644 blob "+blob+"\tpolicy.json\n", "mktree")
					tree := f.gitInput("040000 tree "+sub+"\t.hn\n", "mktree")
					copy := *a
					copy.Commit = f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@hn.invalid", "commit-tree", tree, "-m", tc.name)
					check(tc.name, &copy, authorityError)
				})
			}
			for _, kind := range []string{"enabled-project", "disabled-policy", "sorted-maintainers", "absent-policy"} {
				t.Run(kind, func(t *testing.T) {
					var document map[string]any
					if e := json.Unmarshal([]byte(policyRaw), &document); e != nil {
						t.Fatal(e)
					}
					switch kind {
					case "enabled-project":
						document["contributions"].(map[string]any)["project"] = wp25NativeDigest([]byte("foreign scope"))
					case "disabled-policy":
						delete(document, "contributions")
					case "sorted-maintainers":
						document["maintainers"] = []string{strings.Repeat("f", 64), f.actor}
					}
					raw, e := json.Marshal(document)
					if e != nil {
						t.Fatal(e)
					}
					var tree string
					if kind == "absent-policy" {
						tree = f.gitInput("", "mktree")
					} else {
						blob := f.gitInput(string(raw), "hash-object", "-w", "--stdin")
						sub := f.gitInput("100644 blob "+blob+"\tpolicy.json\n", "mktree")
						tree = f.gitInput("040000 tree "+sub+"\t.hn\n", "mktree")
					}
					copy := *a
					copy.Commit = f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@hn.invalid", "commit-tree", tree, "-m", kind)
					copy.PolicyDigest = wp25NativeDigest(raw)
					if kind == "sorted-maintainers" {
						got, e := (contributionAuthoritySource{&copy}).Historical(copy.current().Ref)
						want := []string{f.actor, strings.Repeat("f", 64)}
						sort.Strings(want)
						if e != nil || !reflect.DeepEqual(got.Maintainers, want) {
							t.Fatal("fresh sorted maintainers", e, got)
						}
					} else {
						check(kind, &copy, authorityError)
					}
				})
			}
			wrong := ref
			wrong.Digest = wp25NativeDigest([]byte("wrong"))
			if _, e := source.Historical(wrong); e == nil || e.Error() != authorityError {
				t.Fatal("wrong digest accepted", e)
			}
			copy := *a
			copy.Policy = a.Policy
			copy.Policy.Contributions = nil
			check("current-disabled", &copy, authorityError)
			p := *a.Policy.Contributions
			copy = *a
			copy.Policy = a.Policy
			copy.Policy.Contributions = &p
			p.Project = wp25NativeDigest([]byte("different project"))
			check("current-project", &copy, authorityError)
			// A real durable pending transaction is installed only after constructor.
			dir := filepath.Join(f.repo, ".git")
			txn := replicationTransactionResult{ID: "txn-intel", Remote: "origin", pendingObjects: []string{ref.Commit}, acceptedObjects: []string{}}
			if err := createReplicationPendingAnchor(dir, txn); err != nil {
				t.Fatal(err)
			}
			if err := recordReplicationTransaction(dir, txn, "validated"); err != nil {
				t.Fatal(err)
			}
			check("fresh-pending", a, authorityError)
			if err := recordReplicationTransaction(dir, txn, "complete"); err != nil {
				t.Fatal(err)
			}
			if err := removeReplicationPendingAnchor(dir, txn.ID); err != nil {
				t.Fatal(err)
			}
			check("pending-recovery", a, "")
			t.Run("fresh-repository", func(t *testing.T) {
				t.Chdir(t.TempDir())
				_, e := source.Historical(ref)
				if e == nil || e.Error() != "not inside a Git repository" {
					t.Fatal("fresh repository prefix", e)
				}
			})
			for _, mode := range []string{"authority-slow-repository", "authority-slow-policy", "authority-work-ledger"} {
				t.Run(mode, func(t *testing.T) {
					actual, e := exec.LookPath("git")
					if e != nil {
						t.Fatal(e)
					}
					peer := t.TempDir()
					path := intelWitnessInstall(t, intelWitnessPeerConfig{actual, peer, mode, nil})
					t.Setenv("PATH", path)
					copy := *a
					p := *a.Policy.Contributions
					copy.Policy = a.Policy
					copy.Policy.Contributions = &p
					if mode != "authority-work-ledger" {
						p.Limits.Milliseconds = 50
						check(mode, &copy, authorityError)
					} else {
						for i := 0; i < 3; i++ {
							check("fresh-record", &copy, "")
						}
						rows := intelWitnessLedger(t, peer)
						counts := map[string]int{}
						for _, r := range rows {
							if r.Event == "start" {
								for _, v := range r.Args {
									if v == "rev-parse" || v == "merge-base" || v == "show" {
										counts[v]++
									}
								}
							}
							if r.Event == "start" && syscall.Kill(r.PID, 0) == nil {
								t.Fatal("authority child unreaped", r)
							}
						}
						if counts["rev-parse"] != 3 || counts["show"] != 3 {
							t.Fatal("per-record repository/policy observations", counts)
						}
						want := 3
						if os.Getenv("HN_INTEL_TEST_STAGE") != "old" {
							want = 0
						}
						if counts["merge-base"] != want {
							t.Fatal("per-record work accounting", counts, want)
						}
						oldRef := ref
						copy.Commit = f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@hn.invalid", "commit-tree", f.git("rev-parse", ref.Commit+"^{tree}"), "-p", ref.Commit, "-m", "descendant")
						if _, e := (contributionAuthoritySource{&copy}).Historical(oldRef); e != nil {
							t.Fatal("legacy noncurrent", e)
						}
						wp25Record(t, map[string]any{"control": "intel-authority-work-ledger", "source_assisted": true, "format": format, "counts": counts, "records": 3, "ledger": intelWitnessLedger(t, peer), "noncurrent_legacy_success": true, "stage": os.Getenv("HN_INTEL_TEST_STAGE")})
					}
				})
			}
			wp25Record(t, map[string]any{"control": "intel-authority-typed-compatibility", "source_assisted": true, "format": format, "policy_raw": policyRaw, "legacy_raw": legacy, "typed_raw": typed, "fresh_distinct_policy_values": true})
		})
	}
}

func intelAuthorityPublicPlans(t *testing.T, binary string) {
	f, project, _, _, plan := nativeClosureRegisterFixture(t, binary)
	actual, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	peer := t.TempDir()
	path := intelWitnessInstall(t, intelWitnessPeerConfig{actual, peer, "public-plans", nil})
	for i, v := range f.env {
		if strings.HasPrefix(v, "PATH=") {
			f.env[i] = "PATH=" + path
		}
	}
	before := f.git(nil, "show-ref")
	receipt := wp25PublicCountCall(t, f, "intel-authority-two-plans", false, true, time.Minute, "contribution", "retained", "register", "--expect", wp25NativeDigest(plan))
	if before != f.git(nil, "show-ref") {
		t.Fatal("retained registration changed public identities")
	}
	rows := intelWitnessLedger(t, peer)
	typed := 0
	legacy := 0
	for _, r := range rows {
		if r.Event == "start" {
			for _, a := range r.Args {
				if strings.HasSuffix(a, "^{commit}:.hn/policy.json") {
					typed++
				}
				if strings.HasSuffix(a, ":.hn/policy.json") && !strings.Contains(a, "^{commit}") {
					legacy++
				}
			}
			if syscall.Kill(r.PID, 0) == nil {
				t.Fatal("public plan Git not reaped", r)
			}
		}
	}
	if os.Getenv("HN_INTEL_TEST_STAGE") != "old" && typed != 2 {
		t.Fatal("both retained plans need independent typed record observations", typed, rows)
	}
	nativeClosureRetainedJournal(t, f, project, wp25NativeDigest(plan), false)
	wp25Record(t, map[string]any{"control": "intel-authority-public-two-plans", "source_assisted_fixture": true, "actual_supplied_candidate": binary, "candidate_sha256": wp25Digest(t, binary), "receipt_raw": string(receipt), "expected_plan_raw": string(plan), "typed_record_reads": typed, "legacy_policy_reads": legacy, "ledger": rows, "stage": os.Getenv("HN_INTEL_TEST_STAGE")})
}

// This SOURCE_ASSISTED seam isolates entry admission of the SAME successful,
// completed result. The live post-done check is source-traced separately; this
// cancel-before-call case does not claim to isolate its dynamic transition.
func intelWitnessCompletedAdmission(t *testing.T, f *artifactCLIFixture, actualGit string, sources []artifactSource) {
	t.Helper()
	peer := t.TempDir()
	path := intelWitnessInstall(t, intelWitnessPeerConfig{actualGit, peer, "completed-success-admission", sources})
	t.Setenv("PATH", path)
	_, err := projectGit(context.Background(), f.repo, nil, 256, "rev-parse", "--git-dir")
	if err != nil {
		t.Fatal(err)
	}
	baseline := intelWitnessFDs(t)
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	child, cancelJob := context.WithCancel(parent)
	args := []string{"show-ref", "--verify", "--"}
	var expected strings.Builder
	for _, source := range sources {
		args = append(args, source.Ref)
		fmt.Fprintf(&expected, "%s %s\n", source.OID, source.Ref)
	}
	job := &artifactWitnessJob{start: 0, end: len(sources), args: args, cancel: cancelJob, done: make(chan struct{})}
	defer func() { job.cancel(); <-job.done }()
	go func() { job.raw, job.err = projectGit(child, f.repo, nil, 64*512, job.args...); close(job.done) }()
	<-job.done
	completedAt := time.Now().UnixNano()
	if parent.Err() != nil || job.err != nil || len(job.raw) == 0 || string(job.raw) != expected.String() {
		t.Fatal("real primary must complete successfully BEFORE parent cancellation", parent.Err(), job.err, string(job.raw))
	}
	original := bytes.Clone(job.raw)
	rows := intelWitnessLedger(t, peer)
	for _, r := range rows {
		if r.Event == "start" && syscall.Kill(r.PID, 0) == nil {
			t.Fatal("completed real Git job not Wait-reaped", r)
		}
	}
	admitted := job.admit(parent)
	intelWitnessResult(t, admitted, "")
	cancelParent()
	cancelledAt := time.Now().UnixNano()
	refused := job.admit(parent)
	intelWitnessResult(t, refused, "cancelled")
	if job.err != nil || !bytes.Equal(job.raw, original) {
		t.Fatal("failed command cannot explain completed-result refusal", job.err)
	}
	job.cancel()
	<-job.done
	after := intelWitnessFDs(t)
	if !reflect.DeepEqual(baseline, after) {
		t.Fatal("admission seam descriptor leak", baseline, after)
	}
	wp25Record(t, map[string]any{"control": "intel-completed-success-admission", "source_assisted": true, "proof_scope": "entry admission of the same successfully completed primary; not a dynamic post-done transition or supplied-package claim", "original_parent_live_at_completion": true, "job_error_before_and_after_cancel": fmt.Sprint(job.err), "raw_stdout": string(original), "expected_stdout": expected.String(), "argv": args, "completed_at_ns": completedAt, "parent_cancelled_at_ns": cancelledAt, "uncancelled_admission_error": fmt.Sprint(admitted), "cancelled_admission_error": fmt.Sprint(refused), "real_command_succeeded_before_parent_cancel": true, "all_direct_PIDs_absent_before_parent_cancel": true, "ledger": rows, "FD_before": baseline, "FD_after": after})
}

// The wrapper delegates to actual Git. Its only injected observations are the
// named initial/final witness fault or the named persisted ref update, after
// capturing the real first complete enumeration. No production hook is used.
func nativeDiscoveryGit(t *testing.T, f *artifactCLIFixture, mode, ref, replacement string) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'" }
	ledger := filepath.Join(root, "ledger")
	body := filepath.Join(root, "body")
	first := filepath.Join(root, "first")
	update := quote(git) + " --git-dir=" + quote(filepath.Join(f.repo, ".git")) + " update-ref "
	if mode == "remove" {
		update += "-d " + quote(ref)
	} else {
		update += quote(ref) + " " + quote(replacement)
	}
	script := "#!/bin/sh\nset -eu\n" +
		"printf '%s\\n' \"$*\" >> " + quote(ledger) + "\n" +
		"case \" $* \" in\n" +
		" *' cat-file '*) : > " + quote(body) + ";;\n" +
		"esac\n" +
		"case \" $* \" in\n" +
		" *' show-ref '*|*' --count=65 '*)\n" +
		"  if { [ " + quote(mode) + " = first-fault ] && [ ! -e " + quote(body) + " ]; } || { [ " + quote(mode) + " = final-fault ] && [ -e " + quote(body) + " ]; }; then exit 93; fi;;\n" +
		"esac\n" +
		"case \" $* \" in\n" +
		" *' --count=100001 '*)\n" +
		"  if [ ! -e " + quote(first) + " ]; then\n" +
		"   : > " + quote(first) + "\n" +
		"   " + quote(git) + " \"$@\" > " + quote(first+".selection") + "\n" +
		"   case " + quote(mode) + " in change|remove|add) " + update + ";; esac\n" +
		"   cat " + quote(first+".selection") + "\n   exit 0\n  fi;;\n" +
		"esac\nexec " + quote(git) + " \"$@\"\n"
	if err = os.WriteFile(filepath.Join(root, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	return ledger
}

func nativeDiscoveryLedger(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wp25Record(t, map[string]any{"control": "native-discovery-real-Git-ledger", "raw": string(raw)})
	if strings.Contains(string(raw), " fetch ") {
		t.Fatal("history reader fetched objects")
	}
	return string(raw)
}

func TestArtifactDiscoveryComposition(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, route := range []string{"ordinary", "explicit"} {
			for _, mode := range []string{"healthy", "first-fault", "final-fault"} {
				t.Run(format+"/"+route+"/"+mode, func(t *testing.T) {
					root := t.TempDir()
					f := &artifactCLIFixture{t: t, repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
					f.git("init", "--quiet", "--object-format="+format)
					ref := artifactFrozenRef(t, f)
					commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
					// Cross a real 64-pattern worker boundary, sharing one valid
					// immutable history. This remains a bounded 65-ref fixture.
					for i := 0; i < 65; i++ {
						alias := fmt.Sprintf("refs/hn/remotes/p%03d/artifacts/", i) + strings.TrimPrefix(ref, "refs/hn/artifacts/")
						f.git("update-ref", alias, commit)
					}
					t.Chdir(f.repo)
					sources, err := artifactSources(context.Background(), filepath.Join(f.repo, ".git"))
					if err != nil {
						t.Fatal(err)
					}
					baseline, err := loadArtifactHistoryFromSources(context.Background(), filepath.Join(f.repo, ".git"), sources)
					if err != nil {
						t.Fatal(err)
					}
					cut, err := baseline.sourceCut()
					if err != nil {
						t.Fatal(err)
					}
					// Deliberately reverse caller input: admission must sort a copy.
					for i, j := 0, len(sources)-1; i < j; i, j = i+1, j-1 {
						sources[i], sources[j] = sources[j], sources[i]
					}
					original := append([]artifactSource(nil), sources...)
					ledger := nativeDiscoveryGit(t, f, mode, ref, commit)
					before := f.snapshot()
					fds := intelWitnessFDs(t)
					var h *artifactHistory
					if route == "ordinary" {
						h, err = loadArtifactHistory(context.Background())
					} else {
						h, err = loadArtifactHistoryFromSources(context.Background(), filepath.Join(f.repo, ".git"), sources)
					}
					trace := nativeDiscoveryLedger(t, ledger)
					t.Logf("route=%s mode=%s witness_commands=%d complete_enumerations=%d error=%v", route, mode, strings.Count(trace, " show-ref "), strings.Count(trace, " --count=100001 "), err)
					if !reflect.DeepEqual(fds, intelWitnessFDs(t)) || !reflect.DeepEqual(sources, original) {
						t.Fatal("reader leaked descriptors or changed supplied source snapshot")
					}
					f.unchanged(before)
					if mode == "final-fault" || mode == "first-fault" && route == "explicit" {
						intelWitnessResult(t, err, "verification_failure")
						if !strings.Contains(trace, " --count=65 ") {
							t.Fatal("failed witness never attempted its original fallback")
						}
						return
					}
					intelWitnessResult(t, err, "")
					wantWitness, wantEnumerations, wantCut := 2, 2, cut
					if route == "explicit" {
						wantWitness, wantEnumerations, wantCut = 4, 0, ""
					}
					if strings.Count(trace, " show-ref ") != wantWitness || strings.Count(trace, " --count=100001 ") != wantEnumerations {
						t.Fatalf("wrong source composition: want %d witnesses/%d complete enumerations; ledger:\n%s", wantWitness, wantEnumerations, trace)
					}
					if h.Cut != wantCut || len(h.ByID) != 1 || string(h.ByID[artifactVectorID].verified.Bytes()) != artifactVector || !reflect.DeepEqual(h.Sources, baseline.Sources) || len(h.readObjects) != 4 {
						t.Fatal("reader changed exact signed output, complete selection, verification or Cut", h)
					}
				})
			}
		}
	}
}

func TestArtifactDiscoveryPersistedChangesAndPrecedence(t *testing.T) {
	for _, mode := range []string{"add", "remove", "change", "integrity", "raw-count-bytes"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			f := &artifactCLIFixture{t: t, repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
			f.git("init", "--quiet")
			ref := artifactFrozenRef(t, f)
			commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			if mode == "integrity" {
				commit = artifactRawCommit(t, f, artifactVector, strings.Repeat("A", 86), "", "hn artifact "+artifactVectorID, false)
			}
			if mode == "raw-count-bytes" {
				// A real oversized raw commit must hit the unchanged count
				// preflight's metadata-before-body byte guard, before integrity.
				path := filepath.Join(root, "oversized-commit")
				body := "tree " + f.git("rev-parse", commit+"^{tree}") + "\nauthor Fixture <fixture@example.invalid> 1 +0000\ncommitter Fixture <fixture@example.invalid> 1 +0000\n\n" + strings.Repeat("x", artifactReadObjectBytes)
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				commit = f.git("hash-object", "-t", "commit", "-w", path)
			}
			f.git("update-ref", ref, commit)
			alternate := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			if alternate == commit {
				alternate = f.git("-c", "user.name=Changed", "-c", "user.email=fixture@example.invalid", "commit-tree", f.git("rev-parse", commit+"^{tree}"), "-m", "hn artifact "+artifactVectorID)
			}
			target, action := ref, mode
			if mode == "add" {
				target = strings.Replace(ref, "refs/hn/", "refs/hn/remotes/added/", 1)
			}
			if mode == "integrity" || mode == "raw-count-bytes" {
				action = "remove"
			}
			t.Chdir(f.repo)
			ledger := nativeDiscoveryGit(t, f, action, target, alternate)
			_, err := loadArtifactHistory(context.Background())
			trace := nativeDiscoveryLedger(t, ledger)
			code, message := "concurrent_update", "artifact source witness changed while reading"
			if mode == "remove" {
				message = "artifact source witness missing while reading"
			}
			if mode == "add" {
				message = "artifact sources changed while reading"
			}
			if mode == "integrity" {
				code, message = "verification_failure", "invalid artifact signature"
			}
			if mode == "raw-count-bytes" {
				code, message = "resource_limit", "artifact object exceeds read limit"
			}
			wp25Record(t, map[string]any{"control": "discovery-persisted-and-mixed-invalid", "mode": mode, "error": fmt.Sprint(err), "ledger": trace})
			intelWitnessResult(t, err, code)
			var failure *artifact.Error
			if !errors.As(err, &failure) || failure.Message != message {
				t.Fatalf("wrong accepted precedence/diagnostic: got %v want %s/%s", err, code, message)
			}
			// Explicit supplied-source precedence stays at the original initial
			// witness, including these same mixed-invalid captured objects.
			if mode == "integrity" || mode == "raw-count-bytes" {
				_, err = loadArtifactHistoryFromSources(context.Background(), filepath.Join(f.repo, ".git"), []artifactSource{{ref, commit}})
				intelWitnessResult(t, err, "concurrent_update")
			}
		})
	}
}
