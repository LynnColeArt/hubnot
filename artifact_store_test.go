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
	// This forged descriptor names a candidate from a different immutable key.
	// Genuine pending/lost-ack retries are exercised by ExactRetryRecoveryCLI.
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
	v = f.publish(artifactStoreType, "uncertain", "verification_failure")
	artifactErrorShape(t, v, "artifact", "verification_failure", "do_not_retry", nil)
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

func artifactTestDescriptor(t *testing.T, f *artifactCLIFixture, operation string, published bool) (string, map[string]any) {
	t.Helper()
	f.publish(artifactStoreType, "descriptor-template", "")
	dir := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor)
	raw, e := os.ReadFile(filepath.Join(dir, strings.TrimPrefix(eventID([]byte("descriptor-template")), "sha256:")+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var intent map[string]any
	if e = json.Unmarshal(raw, &intent); e != nil {
		t.Fatal(e)
	}
	if !published {
		semantic := intent["semantic_request"].(map[string]any)
		semantic["operation"] = operation
		request, _ := json.Marshal(semantic)
		intent["operation"] = operation
		intent["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), request...))
		intent["stage"] = "prepared"
		intent["expected_oid"] = f.git("rev-parse", artifactRef(f.actor))
		intent["candidate"] = nil
		intent["result"] = nil
	}
	return filepath.Join(dir, strings.TrimPrefix(eventID([]byte(operation)), "sha256:")+".json"), intent
}
func TestArtifactStoreStrictIntentCLI(t *testing.T) {
	t.Setenv("HN_WP05_BINARY", artifactStoreBinary(t))
	// Independent literal closed contract, including explicitly nullable fields.
	required := []string{"protocol", "repository_id", "actor", "operation", "request", "semantic_request", "stage", "expected_ref", "expected_oid", "candidate", "result"}
	if len(required) != 11 {
		t.Fatal("invalid contract mirror")
	}
	for _, published := range []bool{false, true} {
		mode := "pending"
		if published {
			mode = "public-replay"
		}
		for _, field := range required {
			t.Run(mode+"/missing-"+field, func(t *testing.T) {
				f := artifactCLI(t)
				operation := "strict-pending"
				if published {
					operation = "descriptor-template"
				}
				path, intent := artifactTestDescriptor(t, f, operation, published)
				delete(intent, field)
				raw, _ := json.Marshal(intent)
				if e := os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
				before := f.snapshot()
				v := f.publish(artifactStoreType, operation, "verification_failure")
				artifactErrorShape(t, v, "artifact", "verification_failure", "do_not_retry", nil)
				f.unchanged(before)
			})
		}
	}
	for _, control := range []struct {
		name string
		edit func(map[string]any)
		raw  func([]byte) []byte
	}{
		{name: "all-nullable-missing", edit: func(m map[string]any) { delete(m, "expected_oid"); delete(m, "candidate"); delete(m, "result") }},
		{name: "null-protocol", edit: func(m map[string]any) { m["protocol"] = nil }},
		{name: "null-semantic", edit: func(m map[string]any) { m["semantic_request"] = nil }},
		{name: "wrong-expected-oid-type", edit: func(m map[string]any) { m["expected_oid"] = false }},
		{name: "wrong-candidate-type", edit: func(m map[string]any) { m["candidate"] = []any{} }},
		{name: "wrong-result-type", edit: func(m map[string]any) { m["result"] = []any{} }},
		{name: "unknown-key", edit: func(m map[string]any) { m["extra"] = true }},
		{name: "duplicate-key", raw: func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"stage":`), []byte(`"stage":"prepared","stage":`), 1)
		}},
		{name: "duplicate-semantic-key", raw: func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"kind":`), []byte(`"kind":"type.publish","kind":`), 1)
		}},
		{name: "null-root", raw: func([]byte) []byte { return []byte("null") }},
		{name: "trailing-json", raw: func(b []byte) []byte { return append(b, []byte(" {}")...) }},
	} {
		t.Run(control.name, func(t *testing.T) {
			f := artifactCLI(t)
			path, intent := artifactTestDescriptor(t, f, "strict-pending", false)
			if control.edit != nil {
				control.edit(intent)
			}
			raw, _ := json.Marshal(intent)
			if control.raw != nil {
				raw = control.raw(raw)
			}
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			before := f.snapshot()
			f.publish(artifactStoreType, "strict-pending", "verification_failure")
			f.unchanged(before)
		})
	}
}

// Faults are real failed Git stages in the current app, not alternate writers or
// injected callbacks. All child work is complete before the exact retry begins.
func artifactTestFault(t *testing.T, f *artifactCLIFixture, operation, stage string) map[string]any {
	t.Helper()
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrapper := t.TempDir()
	marker := filepath.Join(wrapper, "published")
	script := "#!/bin/sh\nfor arg do\n case \"$arg\" in\n hash-object) [ \"$HN_WP05_STAGE\" = prepared ] && exit 86 ;;\n update-ref)\n  [ \"$HN_WP05_STAGE\" = publishing ] && exit 86\n  if [ \"$HN_WP05_STAGE\" = lost_ack ]; then '" + git + "' \"$@\" || exit $?; : > \"$HN_WP05_PUBLISHED\"; exit 0; fi ;;\n for-each-ref) [ -f \"$HN_WP05_PUBLISHED\" ] && exit 86 ;;\n esac\ndone\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	f.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP05_STAGE": stage, "HN_WP05_PUBLISHED": marker}
	code := "repository_error"
	if stage == "publishing" {
		code = "concurrent_update"
	}
	if stage == "lost_ack" {
		code = "uncertain_operation"
	}
	v := f.publish(artifactStoreType, operation, code)
	recovery := "do_not_retry"
	var op any
	if code == "concurrent_update" {
		recovery = "retry_identical"
	}
	if code == "uncertain_operation" {
		recovery = "inspect_operation"
		op = map[string]any{"protocol": "hn-artifact/1", "actor": f.actor, "operation": operation}
	}
	artifactErrorShape(t, v, "artifact", code, recovery, op)
	f.overrides = nil
	path := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor, strings.TrimPrefix(eventID([]byte(operation)), "sha256:")+".json")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var intent map[string]any
	if e = json.Unmarshal(raw, &intent); e != nil {
		t.Fatal(e)
	}
	return intent
}
func TestArtifactStoreExactRetryRecoveryCLI(t *testing.T) {
	t.Setenv("HN_WP05_BINARY", artifactStoreBinary(t))
	for _, stage := range []string{"prepared", "publishing", "lost_ack"} {
		t.Run(stage, func(t *testing.T) {
			f := artifactCLI(t)
			f.publish(artifactStoreType, "base", "")
			operation := "recover-" + stage
			original := artifactTestFault(t, f, operation, stage)
			path := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor, strings.TrimPrefix(eventID([]byte(operation)), "sha256:")+".json")
			if original["stage"] != "prepared" && original["stage"] != "publishing" {
				t.Fatal("fault did not stop the intended stage", original)
			}
			beforeRefs := f.git("for-each-ref", "--format=%(refname) %(objectname)")
			var candidate map[string]any
			if stage != "prepared" {
				candidate = original["candidate"].(map[string]any)
				git, e := exec.LookPath("git")
				if e != nil {
					t.Fatal(e)
				}
				wrapper := t.TempDir()
				marker := filepath.Join(wrapper, "forbidden-rebuild")
				script := "#!/bin/sh\nfor arg do case \"$arg\" in hash-object|mktree|commit-tree) echo rebuild > \"$HN_WP05_REBUILD\"; exit 87 ;; esac; done\nexec '" + git + "' \"$@\"\n"
				if e = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); e != nil {
					t.Fatal(e)
				}
				f.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP05_REBUILD": marker}
				defer func() {
					if raw, e := os.ReadFile(marker); !os.IsNotExist(e) {
						t.Errorf("exact retry rebuilt signed candidate: %v %s", e, raw)
					}
				}()
			}
			result := f.publish(artifactStoreType, operation, "")["data"].(map[string]any)
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			var after map[string]any
			if e = json.Unmarshal(raw, &after); e != nil {
				t.Fatal(e)
			}
			if after["stage"] != "published" || after["result"].(map[string]any)["replayed"] != false {
				t.Fatal("retry did not persist original result", after)
			}
			for _, key := range []string{"protocol", "repository_id", "actor", "operation", "request", "semantic_request", "expected_ref", "expected_oid"} {
				if !reflect.DeepEqual(original[key], after[key]) {
					t.Fatal("retry overwrote original witness", key)
				}
			}
			if candidate != nil {
				if !reflect.DeepEqual(candidate, after["candidate"]) || candidate["id"] != result["event_id"] || candidate["commit"] != f.git("rev-parse", artifactRef(f.actor)) {
					t.Fatal("retry did not preserve exact signed candidate", result, after)
				}
			}
			if stage == "lost_ack" {
				if result["replayed"] != true || beforeRefs != f.git("for-each-ref", "--format=%(refname) %(objectname)") {
					t.Fatal("lost acknowledgement caused another publication", result)
				}
			} else if result["replayed"] != false {
				t.Fatal("unpublished original attempt incorrectly reported replay", result)
			}
			f.overrides = nil
			before := f.snapshot()
			again := f.publish(artifactStoreType, operation, "")["data"].(map[string]any)
			if again["event_id"] != result["event_id"] || again["replayed"] != true {
				t.Fatal(again)
			}
			f.unchanged(before)
		})
	}
}

func TestArtifactStoreNoncreatingEvidence(t *testing.T) {
	f := artifactCLI(t)
	f.publish(artifactStoreType, "base", "")
	original, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(f.repo); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := os.Chdir(original); e != nil {
			t.Error(e)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		t.Fatal(e)
	}
	repo, _, e := artifactRepositoryID()
	if e != nil {
		t.Fatal(e)
	}
	statuses := map[string]bool{}
	read := func(key, status, reason string) {
		t.Helper()
		before := f.snapshot()
		evidence, e := readArtifactOperationEvidence(ctx, h, repo, f.actor, key)
		if e != nil || evidence.Status != status || evidence.Reason != reason {
			t.Fatal("wrong cut-scoped evidence", e, evidence.Status, evidence.Reason)
		}
		statuses[evidence.Status] = true
		f.unchanged(before)
	}
	read("never-used", "verified_absent", "")
	read("base", "found", "")
	dir := filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor)
	heldPath := filepath.Join(dir, strings.TrimPrefix(eventID([]byte("held")), "sha256:")+".lock")
	held, e := os.OpenFile(heldPath, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = artifactTestLock(held); e != nil {
		t.Fatal(e)
	}
	read("held", "uncertain", "publication_unconfirmed")
	artifactTestUnlock(held)
	held.Close()
	path := filepath.Join(dir, strings.TrimPrefix(eventID([]byte("bad")), "sha256:")+".json")
	for _, control := range []struct {
		mode       os.FileMode
		body, code string
	}{{0600, "{broken", "verification_failure"}, {0400, "{}", "repository_error"}} {
		if e = os.WriteFile(path, []byte(control.body), 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.Chmod(path, control.mode); e != nil {
			t.Fatal(e)
		}
		before := f.snapshot()
		_, e = readArtifactOperationEvidence(ctx, h, repo, f.actor, "bad")
		var failure *artifact.Error
		if !errors.As(e, &failure) || failure.Code != control.code {
			t.Fatal("read failure became absence", e)
		}
		f.unchanged(before)
		if e = os.Chmod(path, 0600); e != nil {
			t.Fatal(e)
		}
	}
	artifactTestFault(t, f, "readonly-pending", "prepared")
	h, e = loadArtifactHistory(ctx)
	if e != nil {
		t.Fatal(e)
	}
	read("readonly-pending", "uncertain", "private_intent_pending")
	// Independent closed inventory from the frozen public operation contract.
	if len(statuses) != 3 || !statuses["found"] || !statuses["verified_absent"] || !statuses["uncertain"] {
		t.Fatal("operation status inventory drift", statuses)
	}
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	wrapper := t.TempDir()
	racePath := filepath.Join(dir, strings.TrimPrefix(eventID([]byte("racing-absence")), "sha256:")+".lock")
	script := "#!/bin/sh\nfor arg do if [ \"$arg\" = for-each-ref ]; then : > \"$HN_WP05_NEW_LOCK\"; chmod 600 \"$HN_WP05_NEW_LOCK\"; fi; done\nexec '" + git + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HN_WP05_NEW_LOCK", racePath)
	evidence, e := readArtifactOperationEvidence(ctx, h, repo, f.actor, "racing-absence")
	if e != nil || evidence.Status != "uncertain" || evidence.Reason != "publication_unconfirmed" {
		t.Fatal("new writer during source recheck became absence", e, evidence)
	}
	if _, e = os.Stat(racePath); e != nil {
		t.Fatal("race control did not create lock", e)
	}
}

func TestArtifactStoreStrictNestedIntentCLI(t *testing.T) {
	t.Setenv("HN_WP05_BINARY", artifactStoreBinary(t))
	for _, field := range []string{"id", "commit"} {
		t.Run("candidate/missing-"+field, func(t *testing.T) {
			f := artifactCLI(t)
			path, intent := artifactTestDescriptor(t, f, "descriptor-template", true)
			delete(intent["candidate"].(map[string]any), field)
			raw, _ := json.Marshal(intent)
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			before := f.snapshot()
			f.publish(artifactStoreType, "descriptor-template", "verification_failure")
			f.unchanged(before)
		})
	}
	// Literal frozen MutationResult shape; null is permitted only for family and
	// admission, never inferred from an absent field or an incompatible raw type.
	resultFields := []string{"protocol", "family", "type_id", "resource_id", "event_id", "replayed", "conflict", "head_count", "admission", "source_cut"}
	if len(resultFields) != 10 {
		t.Fatal("invalid result contract mirror")
	}
	for _, field := range resultFields {
		t.Run("result/missing-"+field, func(t *testing.T) {
			f := artifactCLI(t)
			path, intent := artifactTestDescriptor(t, f, "descriptor-template", true)
			delete(intent["result"].(map[string]any), field)
			raw, _ := json.Marshal(intent)
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			before := f.snapshot()
			f.publish(artifactStoreType, "descriptor-template", "verification_failure")
			f.unchanged(before)
		})
	}
	for _, control := range []struct {
		name string
		edit func(map[string]any)
		raw  func([]byte) []byte
	}{
		{name: "candidate-unknown", edit: func(m map[string]any) { m["candidate"].(map[string]any)["extra"] = true }},
		{name: "candidate-null-id", edit: func(m map[string]any) { m["candidate"].(map[string]any)["id"] = nil }},
		{name: "result-null-event", edit: func(m map[string]any) { m["result"].(map[string]any)["event_id"] = nil }},
		{name: "result-string-count", edit: func(m map[string]any) { m["result"].(map[string]any)["head_count"] = "1" }},
		{name: "result-wrong-family", edit: func(m map[string]any) { m["result"].(map[string]any)["family"] = "issue" }},
		{name: "result-wrong-identity", edit: func(m map[string]any) { m["result"].(map[string]any)["event_id"] = "sha256:" + strings.Repeat("1", 64) }},
		{name: "result-duplicate", raw: func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"head_count":`), []byte(`"head_count":1,"head_count":`), 1)
		}},
		{name: "candidate-duplicate", raw: func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"commit":`), []byte(`"commit":"0000000000000000000000000000000000000000","commit":`), 1)
		}},
	} {
		t.Run(control.name, func(t *testing.T) {
			f := artifactCLI(t)
			path, intent := artifactTestDescriptor(t, f, "descriptor-template", true)
			if control.edit != nil {
				control.edit(intent)
			}
			raw, _ := json.Marshal(intent)
			if control.raw != nil {
				raw = control.raw(raw)
			}
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			before := f.snapshot()
			f.publish(artifactStoreType, "descriptor-template", "verification_failure")
			f.unchanged(before)
		})
	}
}

func artifactTestPendingOpening(t *testing.T, f *artifactCLIFixture, operation string) (string, map[string]any, map[string]any) {
	t.Helper()
	path, intent := artifactTestDescriptor(t, f, operation, false)
	payload := map[string]any{"shape": "document", "type_id": "example.test/note", "type_revision": "sha256:" + strings.Repeat("0", 64), "title": "Unresolved exact schema", "content": "data", "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
	semantic := intent["semantic_request"].(map[string]any)
	semantic["kind"] = "record.open"
	semantic["payload"] = payload
	return path, intent, map[string]any{"record": nil, "parents": []any{}, "payload": payload}
}
func artifactTestWriteDescriptor(t *testing.T, path string, intent map[string]any) {
	t.Helper()
	semantic, _ := json.Marshal(intent["semantic_request"])
	intent["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), semantic...))
	raw, _ := json.Marshal(intent)
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestArtifactStoreIntentPayloadShapeCLI(t *testing.T) {
	t.Setenv("HN_WP05_BINARY", artifactStoreBinary(t))
	for _, control := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"members-null", func(s map[string]any) { s["payload"].(map[string]any)["members"] = nil }},
		{"missing-content", func(s map[string]any) { delete(s["payload"].(map[string]any), "content") }},
		{"empty-title", func(s map[string]any) { s["payload"].(map[string]any)["title"] = " " }},
		{"invalid-link", func(s map[string]any) {
			s["payload"].(map[string]any)["relationships"] = []any{map[string]any{"relation": "example.test/link", "target": "not-an-id", "target_mode": "revision", "required": true}}
		}},
		{"convert-no-parent", func(s map[string]any) {
			s["kind"] = "record.convert"
			s["record"] = "sha256:" + strings.Repeat("1", 64)
		}},
	} {
		t.Run(control.name, func(t *testing.T) {
			f := artifactCLI(t)
			path, intent, input := artifactTestPendingOpening(t, f, "bad-payload")
			control.edit(intent["semantic_request"].(map[string]any))
			artifactTestWriteDescriptor(t, path, intent)
			raw, _ := json.Marshal(input)
			before := f.snapshot()
			f.call(string(raw), "verification_failure", "artifact", "open", "--input", "-", "--actor", f.actor, "--operation", "bad-payload", "--json")
			f.unchanged(before)
		})
	}
}
func TestArtifactStorePendingContextUnavailable(t *testing.T) {
	f := artifactCLI(t)
	path, intent, input := artifactTestPendingOpening(t, f, "missing-exact-schema")
	artifactTestWriteDescriptor(t, path, intent)
	original, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(f.repo); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := os.Chdir(original); e != nil {
			t.Error(e)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		t.Fatal(e)
	}
	repo, _, e := artifactRepositoryID()
	if e != nil {
		t.Fatal(e)
	}
	before := f.snapshot()
	evidence, e := readArtifactOperationEvidence(ctx, h, repo, f.actor, "missing-exact-schema")
	if e != nil || evidence.Status != "uncertain" || evidence.Reason != "private_intent_pending" {
		t.Fatal("valid unresolved hint claimed schema authority or absence", e, evidence)
	}
	f.unchanged(before)
	raw, _ := json.Marshal(input)
	f.call(string(raw), "incomplete_dependency", "artifact", "open", "--input", "-", "--actor", f.actor, "--operation", "missing-exact-schema", "--json")
	f.unchanged(before)
}
