package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hubnot/internal/artifact"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestArtifactStoreHistoricalNativeBytesCLI(t *testing.T) {
	binary := artifactStoreBinary(t)
	for _, release := range []struct{ version, source, archive string }{
		{"0.0.1-dev", "f82b57420f0c825a9ea6f3b9f93ede680c8cf06f", "7606efc4018c6afb93538f7325603854860039fc642e05f8a06529cca246d070"},
		{"0.0.2-dev", "e80880abc792e2c386182e06027fc251b134c0d2", "cde0c34fb8b43a461ca73d850b255e94a17be0c0ac54a14f2e42536042bd7a21"},
		{"0.0.3-dev", "01a152cfbb822a7be9ccbc51dac34e3caa0cf925", "dfb54281846272d2fe5a84ad8c2fcd099aeadf5ad901859a857ca60f4ea8ba77"},
	} {
		version := release.version
		t.Run(version, func(t *testing.T) {
			dir, e := filepath.Abs(filepath.Join("testdata", "compatibility", version))
			if e != nil {
				t.Fatal(e)
			}
			var corpus betaHistoricalCorpus
			betaReadJSON(t, filepath.Join(dir, "provenance.json"), &corpus)
			if corpus.Version != release.version || corpus.Source != release.source || corpus.ArchiveDigest != release.archive {
				t.Fatal("frozen release provenance changed")
			}
			for name, want := range corpus.Files {
				raw, e := os.ReadFile(filepath.Join(dir, name))
				if e != nil {
					t.Fatal(e)
				}
				sum := sha256.Sum256(raw)
				if hex.EncodeToString(sum[:]) != want {
					t.Fatal("frozen release fixture bytes changed", name)
				}
			}
			root := t.TempDir()
			f := &artifactCLIFixture{t: t, binary: binary, repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
			git, e := exec.LookPath("git")
			if e != nil {
				t.Fatal(e)
			}
			env := operationalTestEnvironment(f.home)
			portablePublic(t, git, root, env, true, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "clone", "-q", filepath.Join(dir, "public.bundle"), f.repo)
			f.git("config", "maintenance.auto", "false")
			f.git("config", "gc.auto", "0")
			f.git("fetch", "--no-write-fetch-head", filepath.Join(dir, "public.bundle"), "+refs/hn/*:refs/hn/*")
			before := betaHistoricalObjects(t, f.repo, env, corpus.Refs)
			var expected map[string]any
			betaReadJSON(t, filepath.Join(dir, "observations.json"), &expected)
			observe := func() {
				for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow", "memory"} {
					b := portablePublic(t, binary, f.repo, env, true, family, "show", corpus.IDs[family], "--json")
					var got any
					if e := json.Unmarshal([]byte(b), &got); e != nil {
						t.Fatal(e)
					}
					betaSemanticSubset(t, "", expected[family], got)
				}
			}
			observe()
			f.call("", "", "init", "--name", "New custom author")
			var identity struct{ Data struct{ Actor string } }
			raw := portablePublic(t, binary, f.repo, env, true, "identity", "show", "--json")
			if e = json.Unmarshal([]byte(raw), &identity); e != nil {
				t.Fatal(e)
			}
			f.actor = identity.Data.Actor
			id := f.publish(artifactStoreType, "historical-custom", "")["data"].(map[string]any)["event_id"].(string)
			f.call("", "", "artifact", "type", "show", id, "--json")
			observe()
			betaAssertHistoricalRefs(t, f.repo, env, corpus.Refs)
			after := betaHistoricalObjects(t, f.repo, env, corpus.Refs)
			if !bytes.Equal(before, after) {
				t.Fatal("custom publication changed frozen hn/0 or hn-memory/0 bytes")
			}
		})
	}
}

func TestArtifactStoreObjectBudgets(t *testing.T) {
	// Exact production reader budget boundary, using real Git objects. Set only
	// prior aggregate accounting; the object size and bytes come from Git.
	withMemoryRepository(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		dir, e := requireGitRepository()
		if e != nil {
			t.Fatal(e)
		}
		for _, size := range []int{artifactReadObjectBytes, artifactReadObjectBytes + 1} {
			body := bytes.Repeat([]byte("x"), size)
			out, e := gitInput(body, nil, "hash-object", "-w", "--stdin")
			if e != nil {
				t.Fatal(e)
			}
			oid := strings.TrimSpace(string(out))
			reader := artifactObjectReader{ctx: ctx, dir: dir, seen: map[string]bool{}}
			got, e := reader.object(oid, "blob", artifactReadObjectBytes)
			if size == artifactReadObjectBytes {
				if e != nil || len(got) != size {
					t.Fatal("object at bound", e, len(got))
				}
				reader = artifactObjectReader{ctx: ctx, dir: dir, total: artifactReadTotalBytes - int64(size), seen: map[string]bool{}}
				if _, e = reader.object(oid, "blob", artifactReadObjectBytes); e != nil {
					t.Fatal("aggregate at bound", e)
				}
				reader = artifactObjectReader{ctx: ctx, dir: dir, total: artifactReadTotalBytes - int64(size) + 1, seen: map[string]bool{}}
				_, e = reader.object(oid, "blob", artifactReadObjectBytes)
				var refusal *artifact.Error
				if !errors.As(e, &refusal) || refusal.Code != "resource_limit" {
					t.Fatal("aggregate one over", e)
				}
			} else {
				var refusal *artifact.Error
				if !errors.As(e, &refusal) || refusal.Code != "resource_limit" {
					t.Fatal("object one over", e)
				}
			}
		}
	})
}

func TestArtifactStoreIntentLockAndUncertainCLI(t *testing.T) {
	f := artifactCLI(t)
	f.publish(artifactStoreType, "first", "")
	// Hold the real per-key lock in a separate process; the public command must
	// refuse before signing, intent persistence or any public Git object effects.
	lockdir := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor)
	key := strings.TrimPrefix(eventID([]byte("busy")), "sha256:")
	lockpath := filepath.Join(lockdir, key+".lock")
	file, e := os.OpenFile(lockpath, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	if e = artifactTestLock(file); e != nil {
		t.Fatal(e)
	}
	defer artifactTestUnlock(file)
	before := f.snapshot()
	v := f.publish(artifactStoreType, "busy", "concurrent_update")
	artifactErrorShape(t, v, "artifact", "concurrent_update", "retry_identical", nil)
	f.unchanged(before)
	// A durable publishing descriptor with an unconfirmed candidate must not be
	// overwritten, resigned or acknowledged as an absence on a later exact retry.
	existing := filepath.Join(lockdir, strings.TrimPrefix(eventID([]byte("first")), "sha256:")+".json")
	raw, e := os.ReadFile(existing)
	if e != nil {
		t.Fatal(e)
	}
	var intent map[string]any
	if e = json.Unmarshal(raw, &intent); e != nil {
		t.Fatal(e)
	}
	intent["operation"] = "uncertain"
	semantic := intent["semantic_request"].(map[string]any)
	semantic["operation"] = "uncertain"
	request, _ := json.Marshal(semantic)
	intent["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), request...))
	intent["stage"] = "publishing"
	intent["result"] = nil
	raw, _ = json.Marshal(intent)
	path := filepath.Join(lockdir, strings.TrimPrefix(eventID([]byte("uncertain")), "sha256:")+".json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	// Prime a safe lock file so refusal's only permitted lock bookkeeping is fixed.
	if e = os.WriteFile(strings.TrimSuffix(path, ".json")+".lock", nil, 0600); e != nil {
		t.Fatal(e)
	}
	before = f.snapshot()
	v = f.publish(artifactStoreType, "uncertain", "uncertain_operation")
	artifactErrorShape(t, v, "artifact", "uncertain_operation", "inspect_operation", map[string]any{"protocol": "hn-artifact/1", "actor": f.actor, "operation": "uncertain"})
	f.unchanged(before)
	intent["stage"] = "invented-stage"
	raw, _ = json.Marshal(intent)
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	before = f.snapshot()
	v = f.publish(artifactStoreType, "uncertain", "verification_failure")
	artifactErrorShape(t, v, "artifact", "verification_failure", "do_not_retry", nil)
	f.unchanged(before)
}

func artifactTestLock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func artifactTestUnlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

func TestArtifactStoreCASRaceCLI(t *testing.T) {
	f := artifactCLI(t)
	first := f.publish(artifactStoreType, "first", "")["data"].(map[string]any)["event_id"].(string)
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrapper := t.TempDir()
	gate := filepath.Join(wrapper, "gate")
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = update-ref ] && [ -n \"$HN_WP05_GATE\" ]; then\n : > \"$HN_WP05_GATE.ready\"\n n=0; while [ ! -f \"$HN_WP05_GATE.release\" ]; do n=$((n+1)); [ $n -lt 1000 ] || exit 91; sleep 0.01; done\n fi\ndone\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(f.binary, "artifact", "type", "publish", "--input", "-", "--actor", f.actor, "--operation", "loser", "--json")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironmentWith(f.home, map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP05_GATE": gate})
	cmd.Stdin = strings.NewReader(artifactStoreType)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = cmd.Process.Kill() }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, e = os.Stat(gate + ".ready"); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("writer did not reach expected-old CAS", stdout.String(), stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	winner := f.publish(artifactStoreType, "winner", "")["data"].(map[string]any)["event_id"].(string)
	if winner == first {
		t.Fatal("racing control did not append")
	}
	head := f.git("rev-parse", artifactRef(f.actor))
	if e = os.WriteFile(gate+".release", nil, 0600); e != nil {
		t.Fatal(e)
	}
	if e = cmd.Wait(); e == nil {
		t.Fatal("stale CAS unexpectedly succeeded")
	}
	var v map[string]any
	if e = json.Unmarshal(stdout.Bytes(), &v); e != nil {
		t.Fatal(e, stdout.String(), stderr.String())
	}
	artifactErrorShape(t, v, "artifact", "concurrent_update", "retry_identical", nil)
	if f.git("rev-parse", artifactRef(f.actor)) != head {
		t.Fatal("losing CAS replaced winning actor head")
	}
	// Replay still names the winner's exact identity after another operation.
	got := f.publish(artifactStoreType, "winner", "")["data"].(map[string]any)
	if got["event_id"] != winner || got["replayed"] != true {
		t.Fatal(got)
	}
}

func TestArtifactStoreUnsignedPreflight(t *testing.T) {
	ctx := context.Background()
	var zero artifact.TypeDeclaration
	if e := zero.ValidateOpening(ctx, []byte(`{}`)); e == nil {
		t.Fatal("unvalidated declaration allowed unsigned opening")
	}
	for _, raw := range []string{`{"parents":[],"parents":[],"payload":{}}`, `{"parents":[],"payload":{"a":"\ud800"}}`, `{"parents":[],"payload":{"a":9007199254740992}}`} {
		if _, e := artifactParseMutation([]byte(raw), "type.publish"); e == nil {
			t.Fatal("strict parser lost malformed input", raw)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := artifact.ValidateType(ctx, []byte(`{}`)); e == nil {
		t.Fatal("cancelled validation succeeded")
	}
}

func TestArtifactStoreIntentStageContractCLI(t *testing.T) {
	f := artifactCLI(t)
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrapper := t.TempDir()
	key := strings.TrimPrefix(eventID([]byte("stages")), "sha256:")
	path := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor, key+".json")
	script := "#!/bin/sh\nfor arg do\n case \"$arg\" in\n hash-object) cp \"$HN_WP05_INTENT\" \"$HN_WP05_STAGES/prepared.json\" ;;\n update-ref) cp \"$HN_WP05_INTENT\" \"$HN_WP05_STAGES/publishing.json\" ;;\n esac\ndone\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	f.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP05_INTENT": path, "HN_WP05_STAGES": wrapper}
	result := f.publish(artifactStoreType, "stages", "")["data"].(map[string]any)
	// Independent literal contract inventory: exactly these three durable stages,
	// and exactly these eleven descriptor keys. Expectations do not use runtime constants.
	stages := []string{"prepared", "publishing", "published"}
	keys := []string{"protocol", "repository_id", "actor", "operation", "request", "semantic_request", "stage", "expected_ref", "expected_oid", "candidate", "result"}
	// The contract descriptor has eleven keys; this count protects the literal
	// mirror itself from accidental shape drift.
	if len(keys) != 11 || len(stages) != 3 {
		t.Fatal("invalid independent contract mirror")
	}
	for _, stage := range stages {
		p := filepath.Join(wrapper, stage+".json")
		if stage == "published" {
			p = path
		}
		raw, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		var intent map[string]any
		if e = json.Unmarshal(raw, &intent); e != nil {
			t.Fatal(e)
		}
		if len(intent) != len(keys) {
			t.Fatal("descriptor field inventory", intent)
		}
		for _, key := range keys {
			if _, ok := intent[key]; !ok {
				t.Fatal("missing descriptor field", key)
			}
		}
		if intent["protocol"] != "hn-artifact-intent/1" || intent["stage"] != stage || intent["actor"] != f.actor || intent["operation"] != "stages" || intent["expected_oid"] != nil {
			t.Fatal(intent)
		}
		if stage == "prepared" && (intent["candidate"] != nil || intent["result"] != nil) {
			t.Fatal("prepared has publication evidence", intent)
		}
		if stage == "publishing" && (intent["candidate"] == nil || intent["result"] != nil) {
			t.Fatal("publishing evidence shape", intent)
		}
		if stage == "published" {
			candidate := intent["candidate"].(map[string]any)
			if len(candidate) != 2 || candidate["id"] != result["event_id"] || candidate["commit"] != f.git("rev-parse", artifactRef(f.actor)) || !reflect.DeepEqual(intent["result"], result) {
				t.Fatal("published identity witness", intent)
			}
		}
	}
}

func TestArtifactStorePrivateIntentByteBudgetCLI(t *testing.T) {
	f := artifactCLI(t)
	f.publish(artifactStoreType, "original", "")
	dir := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor)
	original := filepath.Join(dir, strings.TrimPrefix(eventID([]byte("original")), "sha256:")+".json")
	raw, e := os.ReadFile(original)
	if e != nil {
		t.Fatal(e)
	}
	var template map[string]any
	if e = json.Unmarshal(raw, &template); e != nil {
		t.Fatal(e)
	}
	for _, size := range []int{2097152, 2097153} {
		operation := "intent-at-bound"
		if size > 2097152 {
			operation = "intent-over-bound"
		}
		intent := map[string]any{}
		for k, v := range template {
			intent[k] = v
		}
		semantic := map[string]any{}
		for k, v := range template["semantic_request"].(map[string]any) {
			semantic[k] = v
		}
		semantic["operation"] = operation
		request, _ := json.Marshal(semantic)
		intent["semantic_request"] = semantic
		intent["operation"] = operation
		intent["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), request...))
		intent["stage"] = "prepared"
		intent["candidate"] = nil
		intent["result"] = nil
		intent["expected_oid"] = f.git("rev-parse", artifactRef(f.actor))
		encoded, _ := json.Marshal(intent)
		if len(encoded) >= size {
			t.Fatal("invalid intent budget fixture")
		}
		encoded = append(encoded, bytes.Repeat([]byte(" "), size-len(encoded))...)
		path := filepath.Join(dir, strings.TrimPrefix(eventID([]byte(operation)), "sha256:")+".json")
		if e = os.WriteFile(path, encoded, 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(strings.TrimSuffix(path, ".json")+".lock", nil, 0600); e != nil {
			t.Fatal(e)
		}
		if size == 2097152 {
			f.publish(artifactStoreType, operation, "")
		} else {
			before := f.snapshot()
			f.publish(artifactStoreType, operation, "resource_limit")
			f.unchanged(before)
		}
	}
}
