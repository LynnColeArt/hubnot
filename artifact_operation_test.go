package main

import (
	"context"
	"encoding/json"
	"hubnot/internal/artifact"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestArtifactOperationValidatorLazyStartup(t *testing.T) {
	// A fresh process observes package startup before another test can inspect an
	// operation. Native command startup must not construct this optional regexp.
	if os.Getenv("HN_WP07_VALIDATOR_STARTUP") != "1" {
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "-test.run=^TestArtifactOperationValidatorLazyStartup$", "-test.count=1")
		cmd.Env = append(os.Environ(), "HN_WP07_VALIDATOR_STARTUP=1")
		if raw, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("validator startup and concurrent first use: %v\n%s", err, raw)
		}
		return
	}
	if artifactOperationType != nil {
		t.Fatal("artifact operation regexp constructed before any operation inspection")
	}
	// Valid selectors reach the actor/key check; invalid selectors must retain
	// the exact earlier refusal. Neither case can reach repository/private state.
	tests := []struct {
		typeID string
		valid  bool
	}{
		{"a/n", true}, {"example.test/note", true}, {"a0.-/n9.-", true},
		{strings.Repeat("a", 64) + "/" + strings.Repeat("n", 64), true},
		{"", false}, {"a", false}, {"/n", false}, {"a/", false},
		{"A/n", false}, {"a/N", false}, {"0a/n", false}, {"a/0n", false},
		{"-a/n", false}, {"a/_n", false}, {"a/n/x", false},
		{"a/n\n", false}, {" a/n", false}, {"a/n ", false}, {"a/雪", false},
		{strings.Repeat("a", 65) + "/n", false}, {"a/" + strings.Repeat("n", 65), false},
		{"hn/n", false}, {"hubnot/n", false},
	}
	type observation struct {
		typeID, message string
		err             error
	}
	const callers = 32
	start := make(chan struct{})
	results := make(chan observation, callers*len(tests))
	for range callers {
		go func() {
			<-start
			for _, test := range tests {
				message := "requires a valid custom type ID"
				if test.valid {
					message = "requires full actor and bounded operation key"
				}
				_, err := inspectArtifactOperation(context.Background(), test.typeID, "", "")
				results <- observation{test.typeID, message, err}
			}
		}()
	}
	close(start)
	for range callers * len(tests) {
		result := <-results
		failure, ok := result.err.(*artifact.Error)
		if !ok || failure.Code != "invalid_input" || failure.Message != result.message {
			t.Fatalf("selector %q: want invalid_input %q, got %v", result.typeID, result.message, result.err)
		}
	}
	if artifactOperationType == nil {
		t.Fatal("operation inspection did not construct its validator")
	}
}

func operationIntentPath(f *artifactCLIFixture, key string) string {
	return filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor, strings.TrimPrefix(eventID([]byte(key)), "sha256:")+".json")
}
func operationIntent(t *testing.T, f *artifactCLIFixture, key string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(operationIntentPath(f, key))
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func operationWriteIntent(t *testing.T, f *artifactCLIFixture, key string, value map[string]any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(operationIntentPath(f, key), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func operationKeys(t *testing.T, value map[string]any, expected []string) {
	t.Helper()
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sort.Strings(expected)
	if !reflect.DeepEqual(keys, expected) {
		t.Fatal("closed contract drift", keys, expected)
	}
}
func operationStatus(t *testing.T, f *artifactCLIFixture, key, status, reason string) map[string]any {
	t.Helper()
	before := f.snapshot()
	data := operationCall(f, key, "example.test/note", "")["data"].(map[string]any)
	operationKeys(t, data, []string{"status", "protocol", "actor", "operation", "family", "type_id", "request", "resource_id", "event_id", "result", "source_cut", "reason"})
	if data["status"] != status || data["protocol"] != "hn-artifact/1" || data["actor"] != f.actor || data["operation"] != key || data["type_id"] != "example.test/note" || data["family"] != nil || !validMemoryDigestID(data["source_cut"].(string)) {
		t.Fatal(data)
	}
	if reason == "" {
		if data["reason"] != nil {
			t.Fatal(data)
		}
	} else if data["reason"] != reason {
		t.Fatal(data)
	}
	if status != "found" {
		for _, field := range []string{"request", "resource_id", "event_id", "result"} {
			if data[field] != nil {
				t.Fatal("invented result", data)
			}
		}
	} else {
		result := data["result"].(map[string]any)
		operationKeys(t, result, []string{"protocol", "family", "type_id", "resource_id", "event_id", "replayed", "conflict", "head_count", "admission", "source_cut"})
		if result["source_cut"] != data["source_cut"] || result["resource_id"] != data["resource_id"] || result["event_id"] != data["event_id"] || result["replayed"] != false || result["admission"] != nil || !validMemoryDigestID(data["request"].(string)) {
			t.Fatal(data)
		}
	}
	raw, _ := json.Marshal(data)
	for _, marker := range []string{"semantic_request", "expected_oid", "expected_ref", "repository_id", "candidate", "privateKey", "PRIVATE-OPERATION-MARKER"} {
		if strings.Contains(string(raw), marker) {
			t.Fatal("private operation evidence leaked", marker)
		}
	}
	f.unchanged(before)
	return data
}

func TestArtifactOperationStrictDescriptorCLI(t *testing.T) {
	// The frozen eleven-field descriptor is independently mirrored, with no extras.
	fields := []string{"protocol", "repository_id", "actor", "operation", "request", "semantic_request", "stage", "expected_ref", "expected_oid", "candidate", "result"}
	for _, field := range append(append([]string{}, fields...), "extra") {
		t.Run(field, func(t *testing.T) {
			f := artifactCLI(t)
			f.publish(artifactStoreType, "strict", "")
			value := operationIntent(t, f, "strict")
			operationKeys(t, value, append([]string{}, fields...))
			if field == "extra" {
				value[field] = "PRIVATE-OPERATION-MARKER"
			} else {
				delete(value, field)
			}
			operationWriteIntent(t, f, "strict", value)
			before := f.snapshot()
			v := operationCall(f, "strict", "example.test/note", "verification_failure")
			artifactErrorShape(t, v, "artifact", "verification_failure", "do_not_retry", nil)
			f.unchanged(before)
		})
	}
}

func TestArtifactOperationContradictionsCLI(t *testing.T) {
	for _, field := range []string{"repository_id", "actor", "operation", "request", "semantic_request", "candidate", "result"} {
		t.Run(field, func(t *testing.T) {
			f := artifactCLI(t)
			f.publish(artifactStoreType, "binding", "")
			value := operationIntent(t, f, "binding")
			switch field {
			case "repository_id":
				value[field] = "sha256:" + strings.Repeat("0", 64)
			case "actor":
				value[field] = strings.Repeat("0", 64)
			case "operation":
				value[field] = "another"
			case "request":
				value[field] = "sha256:" + strings.Repeat("0", 64)
			case "semantic_request":
				value[field].(map[string]any)["payload"].(map[string]any)["type_id"] = "example.test/other"
			case "candidate":
				value[field].(map[string]any)["id"] = "sha256:" + strings.Repeat("0", 64)
			case "result":
				value[field].(map[string]any)["event_id"] = "sha256:" + strings.Repeat("0", 64)
			}
			operationWriteIntent(t, f, "binding", value)
			before := f.snapshot()
			operationCall(f, "binding", "example.test/note", "verification_failure")
			f.unchanged(before)
		})
	}
}

func TestArtifactOperationClosedClassifications(t *testing.T) {
	// Independent literal mirror of all states/reasons the complete local reader
	// can produce. Selection/visibility reasons belong to the later host adapter.
	statuses := map[string]bool{}
	reasons := map[string]bool{}
	for _, test := range []struct {
		e              artifactOperationEvidence
		status, reason string
	}{
		{artifactOperationEvidence{coherent: true, settled: true}, "verified_absent", ""},
		{artifactOperationEvidence{coherent: true, settled: true, Public: &storedArtifact{}}, "found", ""},
		{artifactOperationEvidence{coherent: true, settled: false}, "uncertain", "publication_unconfirmed"},
		{artifactOperationEvidence{coherent: false, settled: true}, "uncertain", "publication_unconfirmed"},
		{artifactOperationEvidence{coherent: true, settled: true, Intent: &artifactIntent{Stage: "prepared"}}, "uncertain", "private_intent_pending"},
		{artifactOperationEvidence{coherent: true, settled: true, Intent: &artifactIntent{Stage: "publishing"}}, "uncertain", "private_intent_pending"},
		{artifactOperationEvidence{coherent: true, settled: true, Intent: &artifactIntent{Stage: "published"}}, "uncertain", "unavailable_history"},
		{artifactOperationEvidence{coherent: true, settled: true, unavailable: true}, "uncertain", "unavailable_history"},
	} {
		classifyArtifactEvidence(&test.e)
		if test.e.Status != test.status || test.e.Reason != test.reason {
			t.Fatal(test.e.Status, test.e.Reason, test.status, test.reason)
		}
		statuses[test.e.Status] = true
		if test.e.Reason != "" {
			reasons[test.e.Reason] = true
		}
	}
	if !reflect.DeepEqual(statuses, map[string]bool{"found": true, "verified_absent": true, "uncertain": true}) || !reflect.DeepEqual(reasons, map[string]bool{"publication_unconfirmed": true, "private_intent_pending": true, "unavailable_history": true}) {
		t.Fatal("operation classification inventory drift", statuses, reasons)
	}
}
