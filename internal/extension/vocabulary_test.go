package extension

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func checkFixedVocabulary(t *testing.T, values, expected []string, contains func(string) bool) {
	t.Helper()
	if len(values) != len(expected) {
		t.Fatalf("vocabulary length %d != %d", len(values), len(expected))
	}
	want := map[string]bool{}
	for _, v := range expected {
		want[v] = true
	}
	seen := map[string]bool{}
	for _, v := range values {
		if !want[v] || seen[v] || !contains(v) {
			t.Fatalf("unexpected, duplicate or rejected value %q", v)
		}
		seen[v] = true
	}
	for _, v := range append([]string{"", "unknown", " ", "\x00"}, expected...) {
		for _, bad := range []string{v + " ", " " + v, v + "\x00", strings.ToUpper(v), "prefix." + v, v + ".suffix"} {
			if !want[bad] && contains(bad) {
				t.Fatalf("near miss admitted %q", bad)
			}
		}
	}
	if contains("") || contains("unknown") {
		t.Fatal("unknown value admitted")
	}
}
func TestFixedVocabularies(t *testing.T) {
	methods := methodVocabulary()
	families := nativeFamilyVocabulary()
	actions := grantActionVocabulary()
	states := ownerStateVocabulary()
	checkFixedVocabulary(t, methods[:], []string{"capabilities.list", "identity.public", "resource.read", "resource.mutate", "resource.operation", "artifact.read", "artifact.mutate", "artifact.operation", "artifact.search", "artifact.type.read", "artifact.type.publish", "changes.capture", "changes.page", "changes.since"}, validMethod)
	checkFixedVocabulary(t, families[:], []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"}, validNativeFamily)
	checkFixedVocabulary(t, actions[:], []string{"read", "observe", "create", "revise", "resolve", "type.publish", "raw.inspect"}, validGrantAction)
	checkFixedVocabulary(t, states[:], []string{"owned", "verified", "failed"}, validOwnerState)
	methods[0] = "changed"
	families[0] = "changed"
	actions[0] = "changed"
	states[0] = "changed"
	if methodVocabulary()[0] != "capabilities.list" || nativeFamilyVocabulary()[0] != "issue" || grantActionVocabulary()[0] != "read" || ownerStateVocabulary()[0] != "owned" {
		t.Fatal("returned arrays alias shared state")
	}
	want := []string{"artifact.mutate", "artifact.operation", "artifact.read", "artifact.search", "artifact.type.publish", "artifact.type.read", "capabilities.list", "changes.capture", "changes.page", "changes.since", "identity.public", "resource.mutate", "resource.operation", "resource.read"}
	names := methodNames()
	if !reflect.DeepEqual(names, want) || !sort.StringsAreSorted(names) {
		t.Fatal(names)
	}
	names[0] = "changed"
	if !reflect.DeepEqual(methodNames(), want) || validMethod("changed") {
		t.Fatal("methodNames returned shared state")
	}
}
func TestVocabularyActualCallers(t *testing.T) {
	for _, name := range []string{"capabilities.list", "Capabilities.list", "capabilities.list ", "capabilities.list\x00", "unknown"} {
		request, _ := json.Marshal(map[string]any{"protocol": HostProtocol, "request_id": "r", "method": name, "params": map[string]any{}})
		_, err := InspectMessage(request, "request", "", "")
		if name == "capabilities.list" {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			requireCode(t, err, "unsupported_capability")
			response, _ := json.Marshal(map[string]any{"protocol": HostProtocol, "request_id": "r", "ok": true, "data": nil, "error": nil})
			_, err = InspectMessage(response, "response", "r", name)
			requireCode(t, err, "unsupported_capability")
		}
	}

	for _, name := range []string{"resource.read", "resource.read ", "Resource.read", "resource.read\x00", "unknown"} {
		m := Manifest{Protocol: "hn.extension-manifest/1", ModuleID: "example.test/tool", ModuleVersion: "1", HostProtocol: HostProtocol, Capabilities: []string{name}}
		raw, _ := json.Marshal(m)
		_, err := ParseManifest(raw)
		if name == "resource.read" {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			requireCode(t, err, "invalid_input")
		}
	}
	for _, grant := range []Grant{{Selector: "native:issue", Actions: []string{"read"}}, {Selector: "native:Issue", Actions: []string{"read"}}, {Selector: "native:issue", Actions: []string{"read "}}} {
		err := ValidateGrants([]Grant{grant})
		if grant.Selector == "native:issue" && grant.Actions[0] == "read" {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			requireCode(t, err, "invalid_input")
		}
	}
	for _, state := range []string{"owned", "verified", "failed", "Owned", "failed ", "failed\x00", ""} {
		file, err := os.CreateTemp(t.TempDir(), "owner")
		if err != nil {
			t.Fatal(err)
		}
		value := slotIdentity{SessionID: strings.Repeat("1", 32), RegistrationID: "sha256:" + strings.Repeat("2", 64), Epoch: strings.Repeat("3", 32), Cleanup: state}
		if err = writeOwner(file, value); err != nil {
			t.Fatal(err)
		}
		_, err = readOwner(file)
		file.Close()
		if state == "owned" || state == "verified" || state == "failed" {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			requireCode(t, err, "repository_error")
		}
	}
}
func TestVocabularyConcurrentValues(t *testing.T) {
	var wait sync.WaitGroup
	for range 64 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 8 {
				if !validMethod("resource.read") || !validNativeFamily("issue") || !validGrantAction("read") || !validOwnerState("owned") || validMethod("resource.read ") {
					t.Error("concurrent vocabulary drift")
				}
				names := methodNames()
				names[0] = "private copy"
			}
		}()
	}
	wait.Wait()
}

func TestVocabularyStartupUnconstructed(t *testing.T) {
	if os.Getenv("HN_VOCABULARY_PROFILE") != "1" {
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "-test.run=^TestVocabularyStartupUnconstructed$", "-test.count=1")
		cmd.Env = append(os.Environ(), "HN_VOCABULARY_PROFILE=1", "GODEBUG=memprofilerate=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("startup construction profile: %v\n%s", err, out)
		}
		return
	}
	if runtime.MemProfileRate != 1 {
		t.Fatal("startup profiling unavailable")
	}
	n, _ := runtime.MemProfile(nil, true)
	records := make([]runtime.MemProfileRecord, n+128)
	n, ok := runtime.MemProfile(records, true)
	if !ok {
		t.Fatal("startup profile exceeded buffer")
	}
	for _, record := range records[:n] {
		frames := runtime.CallersFrames(record.Stack())
		for {
			frame, more := frames.Next()
			if (strings.HasSuffix(frame.File, "/internal/extension/manifest.go") || strings.HasSuffix(frame.File, "/internal/extension/grants.go") || strings.HasSuffix(frame.File, "/internal/extension/process.go")) && (strings.HasSuffix(frame.Function, ".init") || strings.Contains(frame.Function, ".map.init.")) {
				t.Errorf("optional vocabulary constructed at startup: %s (%d allocated objects)", frame.Function, record.AllocObjects)
				break
			}
			if !more {
				break
			}
		}
	}
}
