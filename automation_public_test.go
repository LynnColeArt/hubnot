package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func automationCall(t *testing.T, binary, root, input string, args ...string) (map[string]any, error) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var result map[string]any
	decoder := json.NewDecoder(&stdout)
	if decodeErr := decoder.Decode(&result); decodeErr != nil {
		t.Fatalf("JSON response: %v; stderr=%s", decodeErr, stderr.String())
	}
	if _, decodeErr := decoder.Token(); decodeErr != io.EOF {
		t.Fatal("more than one stdout JSON document")
	}
	return result, err
}

func TestAutomationPublicIdentityAndPin(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := issueCommandTestRepo(t)
	identity, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	result, err := automationCall(t, binary, root, "", "identity", "show", "--json")
	data := issueResultData(t, result, err)
	if result["schema"] != "hn.identity/1" || len(data) != 3 || data["actor"] != identity.Actor || data["name"] != identity.Name || data["public_key"] != identity.PublicKey {
		t.Fatalf("public identity contract: %v", result)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), identity.PrivateKey) {
		t.Fatal("private key exposed")
	}
	result, err = automationCall(t, binary, root, "", "identity", "show", "--json=1", "--unknown")
	assertIssueCode(t, result, err, "invalid_input")
	cmd := exec.Command(binary, "identity", "show", "--json=false")
	cmd.Dir = root
	human, err := cmd.Output()
	if err != nil || !strings.Contains(string(human), "Actor:  "+identity.Actor) {
		t.Fatalf("human identity changed: %s %v", human, err)
	}
	result, err = automationCall(t, binary, root, "", "issue", "open", "--actor", identity.Actor, "--operation", "create", "--json", "Original")
	id := issueResultData(t, result, err)["issue_id"].(string)
	other := testIdentity(t, "Changed identity")
	writeActiveTestIdentity(t, other)
	for _, tc := range []struct {
		name, input string
		args        []string
	}{
		{"open", "", []string{"open", "--actor", identity.Actor, "--operation", "create", "--json", "Original"}},
		{"comment", "", []string{"comment", id, "--actor", identity.Actor, "--json", "hello"}},
		{"close", "", []string{"close", id, "--expect", id, "--actor", identity.Actor, "--json"}},
		{"reopen", "", []string{"reopen", id, "--expect", id, "--actor", identity.Actor, "--json"}},
		{"revise", `{"title":"edit"}`, []string{"revise", id, "--expect", id, "--input", "-", "--actor", identity.Actor, "--json"}},
		{"resolve", `{"title":"resolve"}`, []string{"resolve", id, "--expect", id, "--expect", eventID([]byte("other")), "--input", "-", "--actor", identity.Actor, "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
			result, err := automationCall(t, binary, root, tc.input, append([]string{"issue"}, tc.args...)...)
			assertIssueCode(t, result, err, "actor_mismatch")
			if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
				t.Fatal("mismatched pin wrote refs")
			}
		})
	}
	result, err = automationCall(t, binary, root, "", "issue", "open", "--json", "Unpinned")
	issueResultData(t, result, err)
	for _, actor := range []string{"", "bad", strings.ToUpper(identity.Actor)} {
		result, err = automationCall(t, binary, t.TempDir(), "", "issue", "open", "--actor", actor, "--json", "bad")
		assertIssueCode(t, result, err, "invalid_input")
	}
}

func TestAutomationCapturedSigner(t *testing.T) {
	issueCommandTestRepo(t)
	identity, _ := loadIdentity()
	options, err := parseIssueOptions([]string{"open", "--actor", identity.Actor, "--operation", "captured", "title"})
	if err != nil {
		t.Fatal(err)
	}
	captured, err := nextEvent(identity, "issue.open")
	if err != nil {
		t.Fatal(err)
	}
	events, snapshot, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	other := testIdentity(t, "new active")
	writeActiveTestIdentity(t, other)
	// The captured object, not a second active-identity read, must sign the append.
	mutation, err := applyIssueMutation(options, identity, captured, events, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, stored := range after {
		if stored.ID == mutation.EventID {
			found = true
			if stored.Event.Actor != identity.Actor {
				t.Fatal("substituted active signer")
			}
		}
	}
	if !found {
		t.Fatal("append absent")
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	otherCaptured, _ := nextEvent(other, "issue.open")
	if _, err = applyIssueMutation(options, other, otherCaptured, after, snapshot); err == nil {
		t.Fatal("apply seam bypassed actor pin")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("wrong captured signer appended")
	}
}

func TestAutomationPublicOperation(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := issueCommandTestRepo(t)
	identity, _ := loadIdentity()
	original := `{"title":"Original","metadata":{"example/source":"complete"}}`
	result, err := automationCall(t, binary, root, original, "issue", "open", "--input", "-", "--operation", "create", "--json")
	id := issueResultData(t, result, err)["issue_id"].(string)
	result, err = automationCall(t, binary, root, "", "issue", "operation", "--actor", identity.Actor, "--operation", "create", "--json")
	recorded := issueResultData(t, result, err)
	if recorded["issue_id"] != id || recorded["event_id"] != id || recorded["actor"] != identity.Actor || recorded["operation"] != "create" || recorded["intent"] != "open" || recorded["kind"] != "issue.open" || recorded["timestamp"] == "" || recorded["request"] == "" || len(recorded["parents"].([]any)) != 0 {
		t.Fatalf("operation data: %v", recorded)
	}
	result, err = automationCall(t, binary, root, `{"title":"Later"}`, "issue", "revise", id, "--expect", id, "--input", "-", "--operation", "revise", "--json")
	issueResultData(t, result, err)
	result, err = automationCall(t, binary, root, "", "issue", "comment", id, "--body", "exact body\n", "--operation", "comment", "--json")
	issueResultData(t, result, err)
	result, err = automationCall(t, binary, root, "", "issue", "operation", "--actor", identity.Actor, "--operation", "create", "--json")
	if !reflect.DeepEqual(recorded, issueResultData(t, result, err)) {
		t.Fatal("original operation changed after later writes")
	}
	result, err = automationCall(t, binary, root, "", "issue", "operation", "--actor", identity.Actor, "--operation", "comment", "--json")
	comment := issueResultData(t, result, err)
	if body, present := comment["body"]; !present || body != "exact body\n" || comment["timestamp"] == "" {
		t.Fatalf("empty comment/time lost: %v", comment)
	}
	other := testIdentity(t, "other")
	result, err = automationCall(t, binary, root, "", "issue", "operation", "--actor", other.Actor, "--operation", "create", "--json")
	assertIssueCode(t, result, err, "not_found")
	result, err = automationCall(t, binary, root, `{"title":"Changed"}`, "issue", "open", "--input", "-", "--operation", "create", "--json")
	assertIssueCode(t, result, err, "operation_conflict")
	for _, args := range [][]string{
		{"--operation", "create"}, {"--actor", identity.Actor}, {"--actor", "", "--operation", "create"}, {"--actor", identity.Actor, "--operation", "bad key"}, {"--actor", identity.Actor, "--operation", "create", "--limit", "1"}, {"--actor", identity.Actor, "--operation", "create", "extra"},
	} {
		result, err = automationCall(t, binary, t.TempDir(), "", append([]string{"issue", "operation", "--json"}, args...)...)
		assertIssueCode(t, result, err, "invalid_input")
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range events {
		if stored.ID == id {
			appendIssueCommandFixture(t, identity, stored.Event)
			break
		}
	}
	result, err = automationCall(t, binary, root, "", "issue", "operation", "--actor", identity.Actor, "--operation", "create", "--json")
	assertIssueCode(t, result, err, "operation_conflict")
	mustGit(t, "update-ref", "refs/hn/actors/not-an-actor", "HEAD")
	result, err = automationCall(t, binary, root, "", "issue", "operation", "--actor", identity.Actor, "--operation", "absent", "--json")
	assertIssueCode(t, result, err, "invalid_history")
}

func TestAutomationPublicDetails(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := issueCommandTestRepo(t)
	want := map[string]any{}
	for _, title := range []string{"one", "two", "three"} {
		input, _ := json.Marshal(map[string]any{"title": title, "body": strings.Repeat("x", 60000), "metadata": map[string]string{"example/complete": strings.Repeat("m", 4096)}})
		result, err := automationCall(t, binary, root, string(input), "issue", "open", "--input", "-", "--json")
		id := issueResultData(t, result, err)["issue_id"].(string)
		result, err = automationCall(t, binary, root, "", "issue", "show", id, "--json")
		want[id] = issueResultData(t, result, err)["state"]
	}
	opening := map[string]any{}
	for id, state := range want {
		opening[id] = state.(map[string]any)["metadata"]
	}
	var conflictID string
	for id := range want {
		conflictID = id
		break
	}
	appendIssueCommandFixture(t, testIdentity(t, "left"), Event{Kind: "issue.revise", Subject: conflictID, Parents: []string{conflictID}, Issue: &IssueState{Title: "Left", Status: "open"}, Intent: "revise"})
	appendIssueCommandFixture(t, testIdentity(t, "right"), Event{Kind: "issue.revise", Subject: conflictID, Parents: []string{conflictID}, Issue: &IssueState{Title: "Right", Status: "open"}, Intent: "revise"})
	want[conflictID] = nil
	for id := range want {
		if id == conflictID {
			continue
		}
		result, err := automationCall(t, binary, root, `{"title":"rewritten","metadata":{"example/complete":"new"}}`, "issue", "revise", id, "--expect", id, "--input", "-", "--json")
		issueResultData(t, result, err)
		result, err = automationCall(t, binary, root, "", "issue", "show", id, "--json")
		want[id] = issueResultData(t, result, err)["state"]
		break
	}
	cursor := ""
	seen := map[string]bool{}
	for {
		args := []string{"issue", "list", "--details", "--limit", "1", "--json"}
		if cursor != "" {
			args = append(args, "--cursor", cursor)
		}
		result, err := automationCall(t, binary, root, "", args...)
		data := issueResultData(t, result, err)
		if data["total"] != float64(3) {
			t.Fatal("wrong catalog total")
		}
		for _, item := range data["items"].([]any) {
			row := item.(map[string]any)
			id := row["id"].(string)
			if seen[id] || !reflect.DeepEqual(want[id], row["state"]) {
				t.Fatal("detail state omitted/changed/duplicated")
			}
			if !reflect.DeepEqual(opening[id], row["opening_metadata"]) {
				t.Fatal("original signed opening metadata changed or disappeared")
			}
			seen[id] = true
			if id == conflictID && (row["conflict"] != true || row["head_count"] != float64(2)) {
				t.Fatal("conflict projection lost")
			}
		}
		next, _ := result["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
		wrong, wrongErr := automationCall(t, binary, root, "", "issue", "list", "--limit", "1", "--cursor", cursor, "--json")
		assertIssueCode(t, wrong, wrongErr, "invalid_input")
	}
	if len(seen) != 3 {
		t.Fatal("incomplete details")
	}
	result, err := automationCall(t, binary, root, "", "issue", "list", "--details=false", "--limit", "1", "--json")
	data := issueResultData(t, result, err)
	if _, present := data["items"].([]any)[0].(map[string]any)["state"]; present {
		t.Fatal("default summaries changed")
	}
	result, err = automationCall(t, binary, root, "", "issue", "list", "--details", "--limit", "1", "--cursor", result["next_cursor"].(string), "--json")
	assertIssueCode(t, result, err, "invalid_input")
}

func TestAutomationPublicLookupResourceFailure(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := issueCommandTestRepo(t)
	identity, _ := loadIdentity()
	payload := mustGitTextFromInput(t, bytes.Repeat([]byte("x"), 8*1024*1024+1), "hash-object", "-w", "--stdin")
	signature := mustGitTextFromInput(t, []byte("invalid"), "hash-object", "-w", "--stdin")
	commit := writeMemoryCommitFromEntries(t, []memoryTreeFixture{{Mode: "100644", Kind: "blob", Name: "event.json", OID: payload}, {Mode: "100644", Kind: "blob", Name: "signature", OID: signature}}, nil)
	mustGit(t, "update-ref", actorRef(identity.Actor), commit)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	result, err := automationCall(t, binary, root, "", "issue", "operation", "--actor", identity.Actor, "--operation", "absent", "--json")
	assertIssueCode(t, result, err, "resource_limit")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("lookup mutated refs")
	}
}

func TestAutomationPublicLegacyOpeningMetadata(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := issueCommandTestRepo(t)
	appendIssueCommandFixture(t, testIdentity(t, "legacy"), Event{Kind: "issue.open", Title: "Legacy"})
	result, err := automationCall(t, binary, root, "", "issue", "list", "--details", "--json")
	row := issueResultData(t, result, err)["items"].([]any)[0].(map[string]any)
	metadata, ok := row["opening_metadata"].(map[string]any)
	if !ok || len(metadata) != 0 {
		t.Fatalf("legacy opening metadata must be an empty object: %v", row)
	}
}

// Inventory private state without ever putting key contents in test diagnostics.
func automationPrivateSnapshot(t *testing.T, root string) map[string]struct {
	Mode os.FileMode
	Data string
} {
	t.Helper()
	result := map[string]struct {
		Mode os.FileMode
		Data string
	}{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if os.IsNotExist(err) && path == root {
			return nil
		}
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var data []byte
		if info.Mode().IsRegular() {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		result[relative] = struct {
			Mode os.FileMode
			Data string
		}{info.Mode(), string(data)}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAutomationPublicIdentityDiscoveryReadOnly(t *testing.T) {
	binary := buildOperationalBinary(t)
	for _, name := range []string{"normal", "legacy", "invalid_legacy", "invalid_active", "absent"} {
		t.Run(name, func(t *testing.T) {
			root := inIdentityTestRepository(t)
			identity := testIdentity(t, "Public discovery")
			paths, err := identityKeyringPaths()
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "normal":
				writeActiveTestIdentity(t, identity)
			case "legacy", "invalid_legacy", "invalid_active":
				legacy := writeLegacyIdentity(t, identity)
				if name == "invalid_legacy" {
					if err := os.WriteFile(legacy, []byte("invalid private identity"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if name == "invalid_active" {
					if err := os.WriteFile(paths.active, []byte("invalid actor"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := automationPrivateSnapshot(t, paths.root)
			refs := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)")
			result, callErr := automationCall(t, binary, root, "", "identity", "show", "--json")
			if !reflect.DeepEqual(before, automationPrivateSnapshot(t, paths.root)) {
				t.Fatal("machine discovery changed private paths, bytes or modes")
			}
			if refs != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)") {
				t.Fatal("machine discovery changed refs")
			}
			if name == "normal" || name == "legacy" {
				data := issueResultData(t, result, callErr)
				if len(data) != 3 || data["actor"] != identity.Actor || data["name"] != identity.Name || data["public_key"] != identity.PublicKey {
					t.Fatal("wrong public identity")
				}
			} else {
				assertIssueCode(t, result, callErr, "repository_error")
			}
			encoded, _ := json.Marshal(result)
			if strings.Contains(string(encoded), identity.PrivateKey) || strings.Contains(string(encoded), paths.root) {
				t.Fatal("private discovery details exposed")
			}
			if name == "legacy" {
				cmd := exec.Command(binary, "identity", "show")
				cmd.Dir = root
				output, err := cmd.Output()
				if err != nil || !strings.Contains(string(output), "Actor:  "+identity.Actor) {
					t.Fatal("human legacy discovery changed")
				}
				if actor, err := loadActiveActor(); err != nil || actor != identity.Actor {
					t.Fatal("human legacy migration changed")
				}
			}
		})
	}
}
