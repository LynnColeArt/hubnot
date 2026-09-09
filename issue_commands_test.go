package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func issueCommandTestRepo(t *testing.T) string {
	root := inReplicationTestRepository(t)
	writeActiveTestIdentity(t, testIdentity(t, "CLI"))
	return root
}
func issueCommandInput(t *testing.T, root string, state IssueState) string {
	t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "input.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func issueCommandJSON(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	capture, captureErr := os.CreateTemp(t.TempDir(), "stdout")
	if captureErr != nil {
		t.Fatal(captureErr)
	}
	defer capture.Close()
	previous := os.Stdout
	defer func() { os.Stdout = previous }()
	os.Stdout = capture
	err := cmdIssue(args)
	os.Stdout = previous
	if _, seekErr := capture.Seek(0, io.SeekStart); seekErr != nil {
		t.Fatal(seekErr)
	}
	encoded, readErr := io.ReadAll(capture)
	if readErr != nil {
		t.Fatal(readErr)
	}
	out := string(encoded)
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewBufferString(out))
	if parseErr := decoder.Decode(&envelope); parseErr != nil {
		t.Fatalf("JSON response %q: %v (command error %v)", out, parseErr, err)
	}
	if envelope["schema"] != "hn.issue/1" {
		t.Fatalf("schema: %#v", envelope)
	}
	return envelope, err
}
func issueResultData(t *testing.T, result map[string]any, err error) map[string]any {
	t.Helper()
	if err != nil {
		t.Fatalf("command failed: %v (%#v)", err, result)
	}
	data, ok := result["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data: %#v", result)
	}
	return data
}
func TestIssueCommandsReplayLifecycle(t *testing.T) {
	root := issueCommandTestRepo(t)
	state, _ := CanonicalIssueState(IssueState{Title: "Rich", Status: "open", Criteria: []IssueCriterion{{ID: "c", Text: "works"}}, Metadata: map[string]string{"consumer/context": "x"}})
	input := issueCommandInput(t, root, state)
	result, err := issueCommandJSON(t, "open", "--input", input, "--operation", "create", "--json")
	created := issueResultData(t, result, err)
	id := created["issue_id"].(string)
	rev := created["event_id"].(string)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	result, err = issueCommandJSON(t, "open", "--input", input, "--operation", "create", "--json")
	replay := issueResultData(t, result, err)
	if replay["event_id"] != rev || replay["replayed"] != true {
		t.Fatal("create replay lost identity")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("replay appended")
	}
	result, err = issueCommandJSON(t, "close", id, "--expect", rev, "--operation", "close", "--json")
	closed := issueResultData(t, result, err)["event_id"].(string)
	result, err = issueCommandJSON(t, "reopen", id, "--expect", closed, "--json")
	opened := issueResultData(t, result, err)["event_id"].(string)
	result, err = issueCommandJSON(t, "close", id, "--expect", rev, "--operation", "close", "--json")
	if issueResultData(t, result, err)["event_id"] != closed {
		t.Fatal("old close did not replay")
	}
	result, err = issueCommandJSON(t, "show", id, "--json")
	data := issueResultData(t, result, err)
	if data["state"].(map[string]any)["status"] != "open" || data["head_ids"].([]any)[0] != opened {
		t.Fatal("replay changed current lifecycle")
	}
	before = mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	result, err = issueCommandJSON(t, "revise", id, "--expect", rev, "--input", input, "--json")
	if err == nil || result["error"].(map[string]any)["code"] != "stale_revision" {
		t.Fatalf("stale response: %#v %v", result, err)
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("stale appended")
	}
}
func TestIssueCommandsStrictInput(t *testing.T) {
	root := issueCommandTestRepo(t)
	path := filepath.Join(root, "bad.json")
	for _, raw := range []string{`{"title":"x","title":"y"}`, `{"title":"x","metadata":{"a/b":"x","a/b":"y"}}`, `{"title":"x","unknown":1}`, `{"title":"x","Title":"y"}`, `{"title":"x","criteria":[{"id":"x","text":"x","extra":1}]}`, `{"title":"x"} {}`, `null`, `{"title":"x","labels":null}`, `{"title":3}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := issueCommandJSON(t, "open", "--input", path, "--json")
		if err == nil || result["error"].(map[string]any)["code"] != "invalid_input" {
			t.Fatalf("accepted %s: %#v %v", raw, result, err)
		}
	}
	if refs := mustGitText(t, "for-each-ref", "--format=%(refname)", "refs/hn"); refs != "" {
		t.Fatal("invalid input changed refs")
	}
}

func TestIssueCommandsPaginationAndSnapshot(t *testing.T) {
	root := issueCommandTestRepo(t)
	for i := 0; i < 5; i++ {
		result, err := issueCommandJSON(t, "open", "--json", fmt.Sprintf("issue %d", i))
		issueResultData(t, result, err)
	}
	cursor := ""
	ids := []string{}
	for {
		args := []string{"list", "--json", "--limit", "2"}
		if cursor != "" {
			args = append(args, "--cursor", cursor)
		}
		result, err := issueCommandJSON(t, args...)
		data := issueResultData(t, result, err)
		if data["total"] != float64(5) {
			t.Fatal("wrong total")
		}
		for _, item := range data["items"].([]any) {
			ids = append(ids, item.(map[string]any)["id"].(string))
		}
		next, _ := result["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if len(ids) != 5 || !sort.StringsAreSorted(ids) {
		t.Fatalf("bad pagination: %#v", ids)
	}
	result, err := issueCommandJSON(t, "list", "--json", "--limit", "2")
	issueResultData(t, result, err)
	cursor = result["next_cursor"].(string)
	result, err = issueCommandJSON(t, "open", "--json", "later")
	issueResultData(t, result, err)
	result, err = issueCommandJSON(t, "list", "--json", "--limit", "2", "--cursor", cursor)
	assertIssueCode(t, result, err, "stale_cursor")
	result, err = issueCommandJSON(t, "show", ids[0], "--json", "--limit", "2", "--cursor", cursor)
	assertIssueCode(t, result, err, "invalid_input")
	for _, token := range []string{"bad%%%", base64.RawURLEncoding.EncodeToString([]byte(`{"version":1,"offset":9999999999999999999999999}`))} {
		result, err = issueCommandJSON(t, "list", "--json", "--cursor", token)
		assertIssueCode(t, result, err, "invalid_input")
	}
	bad := filepath.Join(root, "large.json")
	if err := os.WriteFile(bad, bytes.Repeat([]byte(" "), maxIssuePayload+1), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = issueCommandJSON(t, "open", "--json", "--input", bad)
	assertIssueCode(t, result, err, "resource_limit")
}
func assertIssueCode(t *testing.T, result map[string]any, err error, code string) {
	t.Helper()
	if err == nil || result["ok"] != false || result["error"].(map[string]any)["code"] != code {
		t.Fatalf("want %s: %#v %v", code, result, err)
	}
}
func appendIssueCommandFixture(t *testing.T, actor *Identity, event Event) *StoredEvent {
	t.Helper()
	captured, err := nextEvent(actor, event.Kind)
	if err != nil {
		t.Fatal(err)
	}
	event.Protocol = captured.Protocol
	event.Actor = captured.Actor
	event.ActorName = captured.ActorName
	event.PublicKey = captured.PublicKey
	event.Sequence = captured.Sequence
	event.Previous = captured.Previous
	event.Timestamp = captured.Timestamp
	stored, err := appendEvent(event, actor)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}
func TestIssueCommandsConflictAndPartialResolution(t *testing.T) {
	root := issueCommandTestRepo(t)
	result, err := issueCommandJSON(t, "open", "--json", "root")
	id := issueResultData(t, result, err)["issue_id"].(string)
	actor := testIdentity(t, "Parallel editor")
	parents := []string{}
	// These signed siblings model offline edits; the CLI consumes their real Git history.
	for i := 0; i < 205; i++ {
		event := Event{Kind: "issue.revise", Subject: id, Parents: []string{id}, Issue: &IssueState{Title: fmt.Sprintf("option %d", i), Status: "open"}, Intent: "revise"}
		stored := appendIssueCommandFixture(t, actor, event)
		parents = append(parents, stored.ID)
	}
	input := issueCommandInput(t, root, IssueState{Title: "Combined", Status: "open", Criteria: []IssueCriterion{}, Labels: []string{}, Assignees: []string{}, Relations: []IssueRelation{}, Metadata: map[string]string{}})
	result, err = issueCommandJSON(t, "show", id, "--json", "--limit", "2")
	data := issueResultData(t, result, err)
	if data["state"] != nil || data["head_count"] != float64(205) || len(data["head_ids"].([]any)) != 2 {
		t.Fatalf("conflict truncated invisibly: %#v", data)
	}
	snapshot := result["snapshot"].(string)
	result, err = issueCommandJSON(t, "revise", id, "--expect", parents[0], "--input", input, "--json")
	assertIssueCode(t, result, err, "issue_conflict")
	result, err = issueCommandJSON(t, "resolve", id, "--expect", parents[0], "--expect", parents[1], "--input", input, "--json")
	assertIssueCode(t, result, err, "stale_revision")
	args := []string{"resolve", id, "--partial", "--snapshot", snapshot, "--input", input, "--operation", "stage1", "--json"}
	for _, parent := range parents[:200] {
		args = append(args, "--expect", parent)
	}
	result, err = issueCommandJSON(t, args...)
	data = issueResultData(t, result, err)
	if data["head_count"] != float64(6) || data["conflict"] != true {
		t.Fatalf("partial dropped heads: %#v", data)
	}
	stageID := data["event_id"].(string)
	result, err = issueCommandJSON(t, args...)
	if issueResultData(t, result, err)["event_id"] != stageID {
		t.Fatal("partial replay checked stale snapshot")
	}
	result, err = issueCommandJSON(t, "resolve", id, "--partial", "--snapshot", snapshot, "--expect", parents[200], "--expect", parents[201], "--input", input, "--json")
	assertIssueCode(t, result, err, "stale_revision")
	args = []string{"resolve", id, "--expect", stageID, "--input", input, "--json"}
	for _, parent := range parents[200:] {
		args = append(args, "--expect", parent)
	}
	result, err = issueCommandJSON(t, args...)
	data = issueResultData(t, result, err)
	if data["conflict"] != false || data["head_count"] != float64(1) {
		t.Fatal("staged recovery did not converge")
	}
	result, err = issueCommandJSON(t, "history", id, "--kind", "revisions", "--limit", "200", "--json")
	data = issueResultData(t, result, err)
	if data["total"] != float64(208) || len(data["items"].([]any)) != 200 || result["next_cursor"] == nil {
		t.Fatal("history lost ancestry")
	}
}
func TestIssueCommandsCapturedActorRaceAndOperationConflict(t *testing.T) {
	root := issueCommandTestRepo(t)
	identity, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	o, err := parseIssueOptions([]string{"open", "--operation", "race", "--json", "requested"})
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
	competing := appendIssueCommandFixture(t, identity, Event{Kind: "issue.open", Title: "competing"})
	_, err = applyIssueMutation(o, identity, captured, events, snapshot)
	var typed *issueCommandError
	if !errors.As(err, &typed) || typed.Code != "concurrent_update" {
		t.Fatalf("race not classified: %v", err)
	}
	current, err := nextEvent(identity, "issue.open")
	if err != nil || current.Previous != competing.ID {
		t.Fatal("losing request published")
	}
	result, err := issueCommandJSON(t, "open", "--operation", "race", "--json", "requested")
	id := issueResultData(t, result, err)["issue_id"].(string)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	result, err = issueCommandJSON(t, "open", "--operation", "race", "--json", "changed")
	assertIssueCode(t, result, err, "operation_conflict")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("key conflict appended")
	}
	result, err = issueCommandJSON(t, "comment", id, "--operation", "comment", "--json", "hello")
	comment := issueResultData(t, result, err)["event_id"]
	result, err = issueCommandJSON(t, "comment", id, "--operation", "comment", "--json", "hello")
	if issueResultData(t, result, err)["event_id"] != comment {
		t.Fatal("comment retry duplicated")
	}
	result, err = issueCommandJSON(t, "history", id, "--kind", "comments", "--json")
	if issueResultData(t, result, err)["total"] != float64(1) {
		t.Fatal("comment history duplicated")
	}
	_ = root
}
func TestIssueCommandsPublicJSONExitAndStdin(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := issueCommandTestRepo(t)
	call := func(input string, args ...string) (map[string]any, error) {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir = root
		cmd.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		var response map[string]any
		decoder := json.NewDecoder(&stdout)
		if decodeErr := decoder.Decode(&response); decodeErr != nil {
			t.Fatalf("public JSON: %v stderr %s", decodeErr, stderr.String())
		}
		if _, decodeErr := decoder.Token(); decodeErr != io.EOF {
			t.Fatal("extra stdout document or prose")
		}
		return response, err
	}
	result, err := call(`{"title":"From stdin"}`, "issue", "open", "--input", "-", "--operation", "stdin", "--json")
	id := issueResultData(t, result, err)["issue_id"].(string)
	result, err = call("", "issue", "close", id, "--expect", eventID([]byte("old")), "--json")
	assertIssueCode(t, result, err, "stale_revision")
	result, err = call(`{"title":"x","metadata":{"ns/key":"one","ns/key":"two"}}`, "issue", "open", "--input", "-", "--json")
	assertIssueCode(t, result, err, "invalid_input")
	result, err = call("", "issue", "show", eventID([]byte("missing")), "--json")
	assertIssueCode(t, result, err, "not_found")
	result, err = call("", "issue", "close", shortID(id), "--expect", id, "--json")
	assertIssueCode(t, result, err, "invalid_input")
}

func TestIssueCommandsGraphPagesAndDiagnostics(t *testing.T) {
	root := issueCommandTestRepo(t)
	result, err := issueCommandJSON(t, "open", "--json", "A")
	a := issueResultData(t, result, err)["issue_id"].(string)
	result, err = issueCommandJSON(t, "open", "--json", "B")
	b := issueResultData(t, result, err)["issue_id"].(string)
	state, _ := CanonicalIssueState(IssueState{Title: "A", Status: "open", Relations: []IssueRelation{{Kind: "blocks", Target: b}}})
	input := issueCommandInput(t, root, state)
	result, err = issueCommandJSON(t, "revise", a, "--expect", a, "--input", input, "--json")
	aRevision := issueResultData(t, result, err)["event_id"].(string)
	state.Title = "B"
	state.Relations = []IssueRelation{{Kind: "blocks", Target: a}}
	input = issueCommandInput(t, root, state)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	result, err = issueCommandJSON(t, "revise", b, "--expect", b, "--input", input, "--json")
	assertIssueCode(t, result, err, "invalid_input")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("cycle append")
	}
	actor := testIdentity(t, "Offline editor")
	appendIssueCommandFixture(t, actor, Event{Kind: "issue.revise", Subject: b, Parents: []string{b}, Issue: &state, Intent: "revise"})
	appendIssueCommandFixture(t, actor, Event{Kind: "issue.revise", Subject: a, Parents: []string{a}, Issue: &IssueState{Title: "Alternative", Status: "open"}, Intent: "revise"})
	cursor := ""
	edges, members := 0, 0
	component := ""
	for {
		args := []string{"graph", a, "--limit", "1", "--json"}
		if cursor != "" {
			args = append(args, "--cursor", cursor)
		}
		result, err = issueCommandJSON(t, args...)
		data := issueResultData(t, result, err)
		if data["total"] != float64(4) || data["cycle_count"] != float64(1) {
			t.Fatalf("cycle totals: %#v", data)
		}
		row := data["items"].([]any)[0].(map[string]any)
		if row["type"] == "edge" {
			edges++
			edge := row["edge"].(map[string]any)
			if edge["source"] == a && (edge["ambiguous"] != true || edge["revision"] != aRevision) {
				t.Fatal("conflict provenance lost")
			}
		} else {
			members++
			if component == "" {
				component = row["component"].(string)
			}
			if row["component"] != component {
				t.Fatal("component unstable")
			}
		}
		cursor, _ = result["next_cursor"].(string)
		if cursor == "" {
			break
		}
	}
	if edges != 2 || members != 2 {
		t.Fatalf("missing graph records: %d %d", edges, members)
	}
}
func TestIssueCommandsOperationScopeAndDuplicateHistory(t *testing.T) {
	issueCommandTestRepo(t)
	identity, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	other := testIdentity(t, "Other")
	state, _ := CanonicalIssueState(IssueState{Title: "Other", Status: "open"})
	event := Event{Kind: "issue.open", Title: state.Title, Issue: &state, Intent: "open", Operation: "shared"}
	event.Request, _ = IssueRequestDigest(event)
	appendIssueCommandFixture(t, other, event)
	result, err := issueCommandJSON(t, "open", "--operation", "shared", "--json", "Local")
	localID := issueResultData(t, result, err)["issue_id"].(string)
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	var duplicate Event
	for _, stored := range events {
		if stored.ID == localID {
			duplicate = stored.Event
		}
	}
	appendIssueCommandFixture(t, identity, duplicate)
	result, err = issueCommandJSON(t, "open", "--operation", "shared", "--json", "Local")
	assertIssueCode(t, result, err, "operation_conflict")
}
func TestIssueCommandsStableReadErrors(t *testing.T) {
	issueCommandTestRepo(t)
	actor := testIdentity(t, "Reader")
	prefixes := map[byte]int{}
	for i := 0; i < 17; i++ {
		stored := appendIssueCommandFixture(t, actor, Event{Kind: "issue.open", Title: fmt.Sprint(i)})
		prefixes[strings.TrimPrefix(stored.ID, "sha256:")[0]]++
	}
	prefix := ""
	for digit, count := range prefixes {
		if count > 1 {
			prefix = string(digit)
			break
		}
	}
	if prefix == "" {
		t.Fatal("fixture lacks colliding prefix")
	}
	result, err := issueCommandJSON(t, "show", prefix, "--json")
	assertIssueCode(t, result, err, "ambiguous_id")
	mustGit(t, "update-ref", "refs/hn/actors/not-an-actor", "HEAD")
	result, err = issueCommandJSON(t, "list", "--json")
	assertIssueCode(t, result, err, "invalid_history")
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	result, err = issueCommandJSON(t, "list", "--json")
	assertIssueCode(t, result, err, "repository_error")
	result, err = issueCommandJSON(t, "show", "not-an-id", "--json")
	assertIssueCode(t, result, err, "invalid_input")
}

func TestIssueCommandsCrossActorRacePreservesBothEdits(t *testing.T) {
	root := issueCommandTestRepo(t)
	result, err := issueCommandJSON(t, "open", "--json", "root")
	id := issueResultData(t, result, err)["issue_id"].(string)
	state, _ := CanonicalIssueState(IssueState{Title: "Local", Status: "open"})
	input := issueCommandInput(t, root, state)
	options, err := parseIssueOptions([]string{"revise", id, "--expect", id, "--input", input, "--operation", "edit", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	captured, err := nextEvent(identity, "issue.revise")
	if err != nil {
		t.Fatal(err)
	}
	events, snapshot, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	other := testIdentity(t, "Concurrent")
	remote := appendIssueCommandFixture(t, other, Event{Kind: "issue.revise", Subject: id, Parents: []string{id}, Issue: &IssueState{Title: "Other", Status: "open"}, Intent: "revise"})
	local, err := applyIssueMutation(options, identity, captured, events, snapshot)
	if err != nil {
		t.Fatalf("cross-actor race was incorrectly treated as global CAS: %v", err)
	}
	_, err = applyIssueMutation(options, identity, captured, events, snapshot)
	var typed *issueCommandError
	if !errors.As(err, &typed) || typed.Code != "concurrent_update" {
		t.Fatalf("same captured actor state appended twice: %v", err)
	}
	result, err = issueCommandJSON(t, "heads", id, "--json")
	data := issueResultData(t, result, err)
	seen := map[string]bool{}
	for _, raw := range data["items"].([]any) {
		seen[raw.(map[string]any)["id"].(string)] = true
	}
	if len(seen) != 2 || !seen[remote.ID] || !seen[local.EventID] {
		t.Fatal("cross-actor history lost an edit")
	}
	result, err = issueCommandJSON(t, "revise", id, "--expect", id, "--input", input, "--operation", "edit", "--json")
	if issueResultData(t, result, err)["event_id"] != local.EventID {
		t.Fatal("race loser retry did not find original operation")
	}
}
