package main

import (
	"archive/tar"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type betaHistoricalCorpus struct {
	Version       string            `json:"version"`
	Source        string            `json:"source_commit"`
	ArchiveDigest string            `json:"release_archive_sha256"`
	Files         map[string]string `json:"files_sha256"`
	Refs          map[string]string `json:"refs"`
	IDs           map[string]string `json:"ids"`
	FutureActors  map[string]string `json:"future_actors"`
}

func betaReadJSON(t *testing.T, path string, target any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}

// Map keys may be added to documented JSON. Existing field meanings, types,
// historical identities and collection membership must remain compatible.
// Snapshots/cursors are query-instance tokens, and warnings are human prose.
func betaSemanticSubset(t *testing.T, path string, want, got any) {
	t.Helper()
	switch expected := want.(type) {
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("%s: want object, got %#v", path, got)
		}
		for key, value := range expected {
			if path == "" {
				if key == "queryDigest" || key == "warning" {
					continue
				}
				if _, token := value.(string); key == "snapshot" && token {
					continue
				}
			}
			found, exists := actual[key]
			if !exists {
				t.Fatalf("%s/%s: missing historical field", path, key)
			}
			betaSemanticSubset(t, path+"/"+key, value, found)
		}
	case []any:
		actual, ok := got.([]any)
		if !ok || len(expected) != len(actual) {
			t.Fatalf("%s: collection changed: want %#v got %#v", path, want, got)
		}
		for i, value := range expected {
			betaSemanticSubset(t, fmt.Sprintf("%s/%d", path, i), value, actual[i])
		}
	default:
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("%s: want %#v got %#v", path, want, got)
		}
	}
}

func betaHistoricalObjects(t *testing.T, repo string, env []string, refs map[string]string) []byte {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"rev-list", "--objects"}
	for _, oid := range refs {
		args = append(args, oid)
	}
	names := portablePublic(t, git, repo, env, true, args...)
	objects := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(names), "\n") {
		objects[strings.Fields(line)[0]] = true
	}
	ids := make([]string, 0, len(objects))
	for id := range objects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cmd := exec.Command(git, "cat-file", "--batch")
	cmd.Dir, cmd.Env = repo, env
	cmd.Stdin = strings.NewReader(strings.Join(ids, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func betaAssertHistoricalRefs(t *testing.T, repo string, env []string, refs map[string]string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for ref, oid := range refs {
		if got := strings.TrimSpace(portablePublic(t, git, repo, env, true, "rev-parse", ref)); got != oid {
			t.Fatalf("historical ref %s changed: %s != %s", ref, got, oid)
		}
	}
}

func betaFutureArchive(t *testing.T, original, output, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(bytes.NewReader(raw))
	var result bytes.Buffer
	writer := tar.NewWriter(&result)
	changed := false
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "manifest.json" {
			needle := []byte(`"schema":"` + from + `"`)
			if bytes.Count(data, needle) != 1 {
				t.Fatal("version fixture must change exactly one existing schema")
			}
			data = bytes.Replace(data, needle, []byte(`"schema":"`+to+`"`), 1)
			header.Size = int64(len(data))
			changed = true
		}
		if err = writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if !changed {
		t.Fatal("missing historical archive manifest")
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, result.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestBetaReleasedWriterCompatibility(t *testing.T) {
	releases := []struct{ version, source, archive string }{
		{"0.0.1-dev", "f82b57420f0c825a9ea6f3b9f93ede680c8cf06f", "7606efc4018c6afb93538f7325603854860039fc642e05f8a06529cca246d070"},
		{"0.0.2-dev", "e80880abc792e2c386182e06027fc251b134c0d2", "cde0c34fb8b43a461ca73d850b255e94a17be0c0ac54a14f2e42536042bd7a21"},
		{"0.0.3-dev", "01a152cfbb822a7be9ccbc51dac34e3caa0cf925", "dfb54281846272d2fe5a84ad8c2fcd099aeadf5ad901859a857ca60f4ea8ba77"},
	}
	for _, variant := range []struct{ name, tags string }{{"standard", ""}, {"native-only", "noforge"}} {
		t.Run(variant.name, func(t *testing.T) {
			binary := buildMigrationCLIBinary(t, variant.tags) // once per reader variant
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			gitOnly := t.TempDir()
			if err = os.Symlink(git, filepath.Join(gitOnly, "git")); err != nil {
				t.Fatal(err)
			}
			env := []string{"PATH=" + gitOnly, "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="}
			for _, release := range releases {
				t.Run(release.version, func(t *testing.T) {
					dir, err := filepath.Abs(filepath.Join("testdata", "compatibility", release.version))
					if err != nil {
						t.Fatal(err)
					}
					var corpus betaHistoricalCorpus
					betaReadJSON(t, filepath.Join(dir, "provenance.json"), &corpus)
					if corpus.Version != release.version || corpus.Source != release.source || corpus.ArchiveDigest != release.archive {
						t.Fatal("fixture release provenance changed")
					}
					for name, digest := range corpus.Files {
						raw, err := os.ReadFile(filepath.Join(dir, name))
						if err != nil {
							t.Fatal(err)
						}
						if fmt.Sprintf("%x", sha256.Sum256(raw)) != digest {
							t.Fatalf("frozen fixture %s digest mismatch", name)
						}
					}
					var expected map[string]any
					betaReadJSON(t, filepath.Join(dir, "observations.json"), &expected)
					parent := t.TempDir()
					repo := filepath.Join(parent, "reader")
					portablePublic(t, git, parent, env, true, "clone", "-q", filepath.Join(dir, "public.bundle"), repo)
					portablePublic(t, git, repo, env, true, "fetch", "--no-write-fetch-head", filepath.Join(dir, "public.bundle"), "+refs/hn/*:refs/hn/*")
					call := func(ok bool, args ...string) string { return portablePublic(t, binary, repo, env, ok, args...) }
					observe := func(family string, args ...string) {
						t.Helper()
						var got any
						if err := json.Unmarshal([]byte(call(true, args...)), &got); err != nil {
							t.Fatal(err)
						}
						betaSemanticSubset(t, "", expected[family], got)
					}
					beforeObjects := betaHistoricalObjects(t, repo, env, corpus.Refs)
					observeHistorical := func() {
						t.Helper()
						for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"} {
							observe(family, family, "show", corpus.IDs[family], "--json")
						}
						observe("memory", "memory", "show", corpus.IDs["memory"], "--json")
						observe("recall", "memory", "recall", "--at", "HEAD", "--include-untrusted", "--lifecycle", "all", "--json")
					}
					observeHistorical()
					// Recall may create its documented disposable lexical index. Public
					// reconstruction must not create a signer or adopt audit authority.
					call(false, "identity", "show", "--json")
					for _, name := range []string{"identity.json", "identities", "active", "rotation.json", "audit/authority.json"} {
						if _, err := os.Stat(filepath.Join(repo, ".git", "hn", name)); !os.IsNotExist(err) {
							t.Fatalf("keyless reads created %s: %v", name, err)
						}
					}
					if _, err := os.Stat(filepath.Join(repo, ".hn")); !os.IsNotExist(err) {
						t.Fatalf("reconstruction installed public policy/configuration: %v", err)
					}
					betaAssertHistoricalRefs(t, repo, env, corpus.Refs)
					call(true, "init", "--name", "Current reader continuation")
					var identity struct{ Data struct{ Actor string } }
					if err := json.Unmarshal([]byte(call(true, "identity", "show", "--json")), &identity); err != nil {
						t.Fatal(err)
					}
					if identity.Data.Actor == "" || corpus.Refs["refs/hn/actors/"+identity.Data.Actor] != "" {
						t.Fatal("continuation reused historical actor")
					}
					call(true, "issue", "open", "--operation", "new-reader-issue", "--json", "Current reader work")
					// Read historic observations after an unrelated actor append, before adding
					// a new memory (which intentionally changes recall collection membership).
					observeHistorical()
					call(true, "memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "descendants", "--content", "New actor memory", "--json")
					call(true, "issue", "comment", corpus.IDs["issue"], "New actor continuation comment")
					var continued struct {
						Data struct {
							ID           string
							CommentCount int `json:"comment_count"`
							State        map[string]any
						}
					}
					if err := json.Unmarshal([]byte(call(true, "issue", "show", corpus.IDs["issue"], "--json")), &continued); err != nil {
						t.Fatal(err)
					}
					if continued.Data.ID != corpus.IDs["issue"] || continued.Data.CommentCount != 2 || continued.Data.State["status"] != "closed" {
						t.Fatal("new actor did not continue historical issue")
					}
					observe("memory", "memory", "show", corpus.IDs["memory"], "--json")
					betaAssertHistoricalRefs(t, repo, env, corpus.Refs)
					if !bytes.Equal(beforeObjects, betaHistoricalObjects(t, repo, env, corpus.Refs)) {
						t.Fatal("continuation rewrote historical Git objects, payloads or signatures")
					}
					for _, kind := range []string{"protocol", "resource"} {
						t.Run("future-"+kind, func(t *testing.T) {
							actor := corpus.FutureActors[kind]
							ref := "refs/hn/actors/" + actor
							portablePublic(t, git, repo, env, true, "fetch", "--no-write-fetch-head", filepath.Join(dir, "future-"+kind+".bundle"), ref+":"+ref)
							t.Cleanup(func() { portablePublic(t, git, repo, env, true, "update-ref", "-d", ref) })
							// Independently verify the synthetic fixture's signature before testing
							// the public version refusal; a corrupt signature cannot earn this pass.
							payload := []byte(portablePublic(t, git, repo, env, true, "show", ref+":event.json"))
							signature := strings.TrimSpace(portablePublic(t, git, repo, env, true, "show", ref+":signature"))
							var event struct {
								PublicKey string `json:"publicKey"`
								Actor     string
							}
							if err := json.Unmarshal(payload, &event); err != nil {
								t.Fatal(err)
							}
							key, err := base64.RawStdEncoding.DecodeString(event.PublicKey)
							if err != nil {
								t.Fatal(err)
							}
							sig, err := base64.RawStdEncoding.DecodeString(signature)
							if err != nil {
								t.Fatal(err)
							}
							if len(key) != ed25519.PublicKeySize || fmt.Sprintf("%x", sha256.Sum256(key)) != actor || event.Actor != actor || !ed25519.Verify(key, payload, sig) {
								t.Fatal("future fixture is not a valid signed actor envelope")
							}
							for _, args := range [][]string{{"issue", "list", "--json"}, {"issue", "open", "--json", "Refused future-format append"}} {
								before := policyPublicSnapshot(t, repo)
								output := call(false, args...)
								var failure struct {
									OK    bool
									Error struct{ Code, Message string }
								}
								if err := json.NewDecoder(strings.NewReader(output)).Decode(&failure); err != nil {
									t.Fatal(err)
								}
								message := "unsupported protocol"
								if kind == "resource" {
									message = "invalid project schema"
								}
								if failure.OK || failure.Error.Code == "" || !strings.Contains(failure.Error.Message, message) || strings.Contains(failure.Error.Message, "signature") {
									t.Fatalf("wrong refusal boundary: %s", output)
								}
								if !reflect.DeepEqual(before, policyPublicSnapshot(t, repo)) {
									t.Fatal("unsupported signed format changed accepted history or private state")
								}
							}
						})
					}
					if release.version == "0.0.1-dev" {
						return
					} // project archives began in 0.0.2-dev
					archive := filepath.Join(dir, "project.hn")
					observe("archive", "migrate", "inspect", "--archive", archive)
					// Import the actual historical archive, preserving every selected ref/OID.
					destination := migrationCLIRepository(t)
					session := filepath.Join(parent, "historical-import")
					portablePublic(t, binary, repo, env, true, "migrate", "plan", "--source-kind", "archive", "--source-repository", archive, "--destination-kind", "local", "--destination-repository", destination, "--session", session)
					portablePublic(t, binary, repo, env, true, "migrate", "apply", "--session", session, "--confirm-apply")
					importedRefs := portablePublic(t, git, destination, env, true, "for-each-ref", "--format=%(objectname)")
					for _, oid := range corpus.Refs {
						if !strings.Contains(importedRefs, oid) {
							t.Fatalf("historical archive lost exact ref OID %s", oid)
						}
					}
					if !bytes.Equal(beforeObjects, betaHistoricalObjects(t, destination, env, corpus.Refs)) {
						t.Fatal("archive import changed selected historical object closure")
					}
					for _, schema := range []struct{ from, to, message string }{{"hn.project-archive/1", "hn.project-archive/999", "unsupported archive schema"}, {"hn.project-snapshot/1", "hn.project-snapshot/999", "invalid snapshot source/schema"}} {
						t.Run(schema.to, func(t *testing.T) {
							future := filepath.Join(t.TempDir(), "future.hn")
							betaFutureArchive(t, archive, future, schema.from, schema.to)
							before := policyPublicSnapshot(t, repo)
							destBefore := policyPublicSnapshot(t, destination)
							out := call(false, "migrate", "inspect", "--archive", future)
							if !strings.Contains(out, schema.message) {
								t.Fatalf("did not reach version boundary: %s", out)
							}
							refusedSession := filepath.Join(t.TempDir(), "refused-import")
							out = call(false, "migrate", "plan", "--source-kind", "archive", "--source-repository", future, "--destination-kind", "local", "--destination-repository", destination, "--session", refusedSession)
							if !strings.Contains(out, schema.message) {
								t.Fatalf("plan failed for wrong reason: %s", out)
							}
							if _, err := os.Stat(refusedSession); !os.IsNotExist(err) {
								t.Fatal("unsupported archive saved a session")
							}
							if !reflect.DeepEqual(before, policyPublicSnapshot(t, repo)) || !reflect.DeepEqual(destBefore, policyPublicSnapshot(t, destination)) {
								t.Fatal("unsupported archive changed accepted history or private state")
							}
						})
					}
				})
			}
		})
	}
}
