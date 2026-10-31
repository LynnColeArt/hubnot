package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func operationCall(f *artifactCLIFixture, key, typ, code string) map[string]any {
	return f.call("", code, "artifact", "operation", "--actor", f.actor, "--operation", key, "--type", typ, "--json")
}

func TestArtifactOperationReadOnlyCLI(t *testing.T) {
	f := artifactCLI(t)
	before := f.snapshot()
	data := operationCall(f, "unused", "example.test/note", "")["data"].(map[string]any)
	if data["status"] != "verified_absent" || data["result"] != nil || data["request"] != nil || data["reason"] != nil {
		t.Fatal(data)
	}
	f.unchanged(before)
}

func TestArtifactOperationPublishedLookupAndRetryCLI(t *testing.T) {
	f := artifactCLI(t)
	original := f.publish(artifactStoreType, "published", "")["data"].(map[string]any)
	if original["replayed"] != false {
		t.Fatal("first publication must report a new mutation", original)
	}
	// Frozen lookup_conditions reports the original result with replayed=false.
	// A status read and an explicit mutation retry have distinct meanings.
	for _, publicOnly := range []bool{false, true} {
		if publicOnly {
			if err := os.Remove(operationIntentPath(f, "published")); err != nil {
				t.Fatal(err)
			}
		}
		before := f.snapshot()
		lookup := operationCall(f, "published", "example.test/note", "")["data"].(map[string]any)
		result := lookup["result"].(map[string]any)
		if lookup["status"] != "found" || result["replayed"] != false || result["event_id"] != original["event_id"] || result["resource_id"] != original["resource_id"] {
			t.Fatal("lookup changed the original result", publicOnly, lookup, original)
		}
		f.unchanged(before)
		retry := f.publish(artifactStoreType, "published", "")["data"].(map[string]any)
		if retry["replayed"] != true || retry["event_id"] != original["event_id"] || retry["resource_id"] != original["resource_id"] {
			t.Fatal("explicit retry did not replay the original mutation", publicOnly, retry, original)
		}
		f.unchanged(before)
	}
}

func TestArtifactOperationFoundAndSourceCutCLI(t *testing.T) {
	f := artifactCLI(t)
	schema := revisionEvent(f.publish(artifactStoreType, "type", ""))
	operationStatus(t, f, "type", "found", "")
	input := revisionInput(t, nil, []string{}, revisionPayload(schema, "PRIVATE-OPERATION-MARKER is inert public content"))
	opening := revisionEvent(revisionCall(f, "open", input, "open", ""))
	original := operationStatus(t, f, "open", "found", "")
	next := revisionInput(t, opening, []string{opening}, revisionPayload(schema, "later content"))
	revisionCall(f, "revise", next, "revise", "")
	current := operationStatus(t, f, "open", "found", "")
	if original["event_id"] != opening || current["event_id"] != opening || original["request"] != current["request"] || original["source_cut"] == current["source_cut"] {
		t.Fatal(original, current)
	}
	shown := f.call("", "", "artifact", "show", opening, "--json")["data"].(map[string]any)
	if shown["source_cut"] != current["source_cut"] {
		t.Fatal("lookup did not use complete current cut", shown, current)
	}
	before := f.snapshot()
	operationCall(f, "open", "example.test/other", "operation_conflict")
	f.unchanged(before)
	// Public proof survives a missing hint, and readers neither need nor create keys.
	if err := os.Remove(operationIntentPath(f, "open")); err != nil {
		t.Fatal(err)
	}
	rotate := exec.Command(f.binary, "identity", "rotate", "--name", "Replacement actor")
	rotate.Dir = f.repo
	rotate.Env = operationalTestEnvironment(f.home)
	if raw, err := rotate.CombinedOutput(); err != nil {
		t.Fatalf("rotate active actor: %v %s", err, raw)
	}
	selected := f.call("", "", "identity", "show", "--json")["data"].(map[string]any)["actor"]
	if selected == f.actor {
		t.Fatal("rotation control retained original actor")
	}
	operationStatus(t, f, "open", "found", "")
	before = f.snapshot()
	revisionCall(f, "open", input, "open", "actor_mismatch")
	f.unchanged(before)
	revisionCall(f, "open", input, "new-wrong-actor", "actor_mismatch")
	f.unchanged(before)
	for _, name := range []string{"active", "identity.json"} {
		if err := os.Remove(filepath.Join(f.repo, ".git", "hn", name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	operationStatus(t, f, "open", "found", "")
	operationStatus(t, f, "unused-without-signer", "verified_absent", "")
}

func TestArtifactOperationWriterEvidenceCLI(t *testing.T) {
	f := artifactCLI(t)
	f.publish(artifactStoreType, "base", "")
	operationStatus(t, f, "unused", "verified_absent", "")
	path := strings.TrimSuffix(operationIntentPath(f, "held"), ".json") + ".lock"
	held, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := artifactTestLock(held); err != nil {
		t.Fatal(err)
	}
	operationStatus(t, f, "held", "uncertain", "publication_unconfirmed")
	artifactTestUnlock(held)
	operationStatus(t, f, "held", "verified_absent", "")
	artifactTestFault(t, f, "pending", "prepared")
	operationStatus(t, f, "pending", "uncertain", "private_intent_pending")
	before := f.snapshot()
	operationCall(f, "pending", "example.test/other", "operation_conflict")
	f.unchanged(before)
	f.publish(artifactStoreType, "pruned", "")
	value := operationIntent(t, f, "pruned")
	f.git("update-ref", artifactRef(f.actor), value["expected_oid"].(string))
	operationStatus(t, f, "pruned", "uncertain", "unavailable_history")
}

func TestArtifactOperationNewWriterCLI(t *testing.T) {
	f := artifactCLI(t)
	f.publish(artifactStoreType, "base", "")
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := t.TempDir()
	counter := filepath.Join(wrapper, "counter")
	path := strings.TrimSuffix(operationIntentPath(f, "new-writer"), ".json") + ".lock"
	// loadArtifactHistory verifies two source observations; the evidence reader's third
	// enumeration creates a new writer witness after its initial lock observations.
	script := "#!/bin/sh\nfor arg do if [ \"$arg\" = for-each-ref ]; then\n if [ -f \"$HN_WP07_COUNT.2\" ]; then : > \"$HN_WP07_LOCK\"; chmod 600 \"$HN_WP07_LOCK\"; elif [ -f \"$HN_WP07_COUNT\" ]; then : > \"$HN_WP07_COUNT.2\"; else : > \"$HN_WP07_COUNT\"; fi\nfi; done\nexec '" + git + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	f.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP07_COUNT": counter, "HN_WP07_LOCK": path}
	data := operationCall(f, "new-writer", "example.test/note", "")["data"].(map[string]any)
	if data["status"] != "uncertain" || data["reason"] != "publication_unconfirmed" {
		t.Fatal(data)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("new-writer control did not run", err)
	}
	f.overrides = nil
	operationStatus(t, f, "new-writer", "verified_absent", "")
}

func TestArtifactOperationRefusalsAndBoundsCLI(t *testing.T) {
	f := artifactCLI(t)
	operationStatus(t, f, strings.Repeat("k", 128), "verified_absent", "")
	for _, typ := range []string{strings.Repeat("a", 64) + "/n", "a/" + strings.Repeat("n", 64)} {
		before := f.snapshot()
		operationCall(f, "bound", typ, "")
		f.unchanged(before)
	}
	for _, control := range []struct{ key, typ string }{
		{strings.Repeat("k", 129), "a/n"}, {"bad key", "a/n"}, {"\u007f", "a/n"}, {"snow雪", "a/n"},
		{"k", strings.Repeat("a", 65) + "/n"}, {"k", "a/" + strings.Repeat("n", 65)},
		{"k", "hn/n"}, {"k", "hubnot/n"}, {"k", "A/n"}, {"k", "a/n/x"},
	} {
		before := f.snapshot()
		operationCall(f, control.key, control.typ, "invalid_input")
		f.unchanged(before)
	}
	before := f.snapshot()
	for _, args := range [][]string{
		{"--actor", f.actor, "--operation", "k", "--type", "a/n", "--type", "a/n", "--json"},
		{"--actor", f.actor, "--operation", "k", "--json"},
		{"--actor", f.actor, "--operation", "k", "--type", "a/n", "--json", "--json"},
		{"--actor", f.actor, "--operation", "k", "--type", "a/n", "--unknown", "x", "--json"},
		{"--actor", strings.Repeat("0", 63), "--operation", "k", "--type", "a/n", "--json"},
	} {
		f.call("", "invalid_input", append([]string{"artifact", "operation"}, args...)...)
		f.unchanged(before)
	}
	// Repository failures and malformed private evidence are never absence.
	missing := &artifactCLIFixture{t: t, binary: f.binary, repo: t.TempDir(), home: f.home, actor: f.actor}
	missing.call("", "repository_error", "artifact", "operation", "--actor", f.actor, "--operation", "k", "--type", "a/n", "--json")
	f.publish(artifactStoreType, "bad", "")
	for _, raw := range []string{"{broken", `{"protocol":"hn-artifact-intent/1","protocol":"hn-artifact-intent/1"}`} {
		if err := os.WriteFile(operationIntentPath(f, "bad"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		before = f.snapshot()
		operationCall(f, "bad", "example.test/note", "verification_failure")
		f.unchanged(before)
	}
}

func TestArtifactOperationInterruptedRestartCLI(t *testing.T) {
	for _, kind := range []string{"type", "open", "revise"} {
		for _, stage := range []string{"prepared", "publishing", "lost_ack"} {
			t.Run(kind+"/"+stage, func(t *testing.T) {
				f := artifactCLI(t)
				schema := revisionEvent(f.publish(artifactStoreType, "type-base", ""))
				input := artifactStoreType
				command := []string{"artifact", "type", "publish"}
				if kind != "type" {
					input = revisionInput(t, nil, []string{}, revisionPayload(schema, "original immutable text 雪 e\u0301"))
					command = []string{"artifact", kind}
					if kind == "revise" {
						opening := revisionEvent(revisionCall(f, "open", input, "opening-base", ""))
						input = revisionInput(t, opening, []string{opening}, revisionPayload(schema, "original immutable text 雪 e\u0301"))
					}
				}
				key := "interrupted-" + kind + "-" + stage
				args := append(command, "--input", "-", "--actor", f.actor, "--operation", key, "--json")
				beforeRef := f.git("rev-parse", artifactRef(f.actor))
				git, err := exec.LookPath("git")
				if err != nil {
					t.Fatal(err)
				}
				wrapper := t.TempDir()
				marker := filepath.Join(wrapper, "stage-reached")
				script := "#!/bin/sh\nfor arg do case \"$arg\" in\n hash-object) if [ \"$HN_WP07_STAGE\" = prepared ]; then : > \"$HN_WP07_REACHED\"; while :; do sleep 1; done; fi ;;\n update-ref) if [ \"$HN_WP07_STAGE\" = lost_ack ]; then '" + git + "' \"$@\" || exit $?; fi\n : > \"$HN_WP07_REACHED\"; while :; do sleep 1; done ;;\n esac; done\nexec '" + git + "' \"$@\"\n"
				if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				env := map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP07_STAGE": stage, "HN_WP07_REACHED": marker}
				cmd := exec.Command(f.binary, args...)
				cmd.Dir = f.repo
				cmd.Env = operationalTestEnvironmentWith(f.home, env)
				cmd.Stdin = strings.NewReader(input)
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				var output bytes.Buffer
				cmd.Stdout = &output
				cmd.Stderr = &output
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				deadline := time.Now().Add(10 * time.Second)
				reached := false
				for time.Now().Before(deadline) {
					if _, err := os.Stat(marker); err == nil {
						reached = true
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
					t.Fatal(err)
				}
				if err := cmd.Wait(); err == nil {
					t.Fatal("interruption did not terminate the actual app")
				}
				if !reached || output.Len() != 0 {
					t.Fatal("wrong interruption boundary", reached, output.String())
				}
				original := operationIntent(t, f, key)
				expectedStage := "publishing"
				if stage == "prepared" {
					expectedStage = "prepared"
				}
				if original["stage"] != expectedStage || original["result"] != nil {
					t.Fatal(original)
				}
				if stage == "lost_ack" {
					lookup := operationStatus(t, f, key, "found", "")
					if lookup["result"].(map[string]any)["replayed"] != false {
						t.Fatal("post-CAS lost acknowledgment lookup must report the original result", lookup)
					}
					if f.git("rev-parse", artifactRef(f.actor)) == beforeRef {
						t.Fatal("lost acknowledgement control did not publish")
					}
				} else {
					operationStatus(t, f, key, "uncertain", "private_intent_pending")
					if f.git("rev-parse", artifactRef(f.actor)) != beforeRef {
						t.Fatal("prepublication interruption published")
					}
				}
				before := f.snapshot()
				changed := strings.Replace(input, "example.test/note", "example.test/other", 1)
				f.call(changed, "operation_conflict", args...)
				f.unchanged(before)
				if kind != "type" {
					f.call(strings.Replace(input, "original immutable text", "changed text", 1), "operation_conflict", args...)
					f.unchanged(before)
				}
				if kind == "revise" {
					for _, field := range []string{"record", "parents", "relationships"} {
						var control map[string]any
						if err := json.Unmarshal([]byte(input), &control); err != nil {
							t.Fatal(err)
						}
						switch field {
						case "record":
							control["record"] = "sha256:" + strings.Repeat("0", 64)
						case "parents":
							control["parents"] = []string{"sha256:" + strings.Repeat("0", 64)}
						case "relationships":
							control["payload"].(map[string]any)["relationships"] = []any{map[string]any{"relation": "example.test/link", "target": "sha256:" + strings.Repeat("0", 64), "target_mode": "revision", "required": false}}
						}
						raw, err := json.Marshal(control)
						if err != nil {
							t.Fatal(err)
						}
						f.call(string(raw), "operation_conflict", args...)
						f.unchanged(before)
					}
				}
				// An already signed original candidate must be resumed byte-for-byte. Make
				// every object-construction command fail if an exact retry tries to rebuild.
				marker2 := filepath.Join(wrapper, "forbidden-rebuild")
				if stage != "prepared" {
					script = "#!/bin/sh\nfor arg do case \"$arg\" in hash-object|mktree|commit-tree) : > \"$HN_WP07_REBUILD\"; exit 87 ;; esac; done\nexec '" + git + "' \"$@\"\n"
					if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
						t.Fatal(err)
					}
					f.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP07_REBUILD": marker2}
				}
				objects := f.git("cat-file", "--batch-all-objects", "--batch-check=%(objectname)")
				recovered := f.call(input, "", args...)["data"].(map[string]any)
				f.overrides = nil
				completed := operationIntent(t, f, key)
				for _, field := range []string{"protocol", "repository_id", "actor", "operation", "request", "semantic_request", "expected_ref", "expected_oid"} {
					if !reflect.DeepEqual(original[field], completed[field]) {
						t.Fatal("retry replaced immutable request/witness", field)
					}
				}
				if completed["stage"] != "published" {
					t.Fatal(completed)
				}
				if stage != "prepared" {
					if !reflect.DeepEqual(original["candidate"], completed["candidate"]) || objects != f.git("cat-file", "--batch-all-objects", "--batch-check=%(objectname)") {
						t.Fatal("retry rebuilt original signed candidate")
					}
					if _, err := os.Stat(marker2); !os.IsNotExist(err) {
						t.Fatal("rebuild control triggered", err)
					}
				}
				if recovered["replayed"] != (stage == "lost_ack") {
					t.Fatal(recovered)
				}
				status := operationStatus(t, f, key, "found", "")
				if recovered["event_id"] != status["event_id"] {
					t.Fatal(recovered, status)
				}
				before = f.snapshot()
				again := f.call(input, "", args...)["data"].(map[string]any)
				if again["event_id"] != recovered["event_id"] || again["replayed"] != true {
					t.Fatal(again)
				}
				f.unchanged(before)
				// A public chain contains one occurrence of this exact actor/key.
				count := 0
				for _, commit := range strings.Fields(f.git("rev-list", artifactRef(f.actor))) {
					var envelope map[string]any
					if err := json.Unmarshal([]byte(f.git("show", commit+":artifact.json")), &envelope); err != nil {
						t.Fatal(err)
					}
					if envelope["operation"] == key {
						count++
					}
				}
				if count != 1 {
					t.Fatal("operation was publicly appended more than once", count)
				}
			})
		}
	}
}

func TestArtifactOperationInvalidPublicHistoryCLI(t *testing.T) {
	f := artifactCLI(t)
	ref := artifactFrozenRef(t, f)
	for _, control := range []struct{ signature, message, code string }{
		{strings.Repeat("A", 86), "hn artifact " + artifactVectorID, "verification_failure"},
		{artifactVectorSignature, "wrong public message", "invalid_history"},
	} {
		commit := artifactRawCommit(t, f, artifactVector, control.signature, "", control.message, false)
		f.git("update-ref", ref, commit)
		before := f.snapshot()
		operationCall(f, "unused", "example.test/note", control.code)
		f.unchanged(before)
	}
}

func TestArtifactOperationJournalFailureCLI(t *testing.T) {
	f := artifactCLI(t)
	f.publish(artifactStoreType, "base", "")
	// A journal path cannot be replaced by a directory. The real writer must
	// refuse before signing/publication; status must report the unsafe evidence.
	path := operationIntentPath(f, "journal-failure")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot()
	f.publish(artifactStoreType, "journal-failure", "repository_error")
	f.unchanged(before)
	operationCall(f, "journal-failure", "example.test/note", "repository_error")
	f.unchanged(before)
}

func TestArtifactOperationCurrentConflictAndPublicReconstructionCLI(t *testing.T) {
	a, b, reader := artifactCLI(t), artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	input := revisionInput(t, nil, []string{}, revisionPayload(schema, "original"))
	opening := revisionEvent(revisionCall(a, "open", input, "open", ""))
	original := operationStatus(t, a, "open", "found", "")
	revisionFetch(b, a, "a")
	revisionCall(a, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "A")), "revise", "")
	revisionCall(b, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "B")), "revise", "")
	revisionFetch(a, b, "b")
	current := operationStatus(t, a, "open", "found", "")
	result := current["result"].(map[string]any)
	if current["event_id"] != opening || current["request"] != original["request"] || current["source_cut"] == original["source_cut"] || result["head_count"] != float64(2) || result["conflict"] != true {
		t.Fatal(current, original)
	}
	before := a.snapshot()
	replay := revisionCall(a, "open", input, "open", "")["data"].(map[string]any)
	if replay["event_id"] != opening || replay["head_count"] != float64(2) || replay["conflict"] != true || replay["replayed"] != true {
		t.Fatal(replay)
	}
	a.unchanged(before)
	// Reconstruct solely from public Git refs in an independent initialized reader.
	revisionFetch(reader, a, "a")
	revisionFetch(reader, b, "b")
	readerActor := reader.actor
	reader.actor = a.actor
	recovered := operationStatus(t, reader, "open", "found", "")
	if recovered["event_id"] != opening || recovered["request"] != original["request"] || recovered["result"].(map[string]any)["head_count"] != float64(2) || recovered["result"].(map[string]any)["replayed"] != false {
		t.Fatal(recovered)
	}
	reader.actor = readerActor
}
