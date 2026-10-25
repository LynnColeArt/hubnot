package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func ghTestCall(t *testing.T, args ...string) (string, error) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	old := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = old }()
	callErr := cmdGH(args)
	os.Stdout = old
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), callErr
}
func ghTestOK(t *testing.T, args ...string) string {
	t.Helper()
	out, err := ghTestCall(t, args...)
	if err != nil {
		t.Fatalf("%v: %v (%s)", args, err, out)
	}
	return out
}
func TestGHCompatibilityLifecycle(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "unusable-no-network")
	created := strings.TrimSpace(ghTestOK(t, "issue", "create", "--title", "Compatible", "-b", "native body", "-l", "bug,ui", "-a", "alice"))
	if !validEventID(created) {
		t.Fatalf("create result %q", created)
	}
	raw := ghTestOK(t, "issue", "view", created, "--json", "id,title,body,state,labels,assignees,author")
	var item map[string]any
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatal(err)
	}
	if item["id"] != created || item["title"] != "Compatible" || item["state"] != "OPEN" || len(item) != 7 {
		t.Fatalf("view: %s", raw)
	}
	ghTestOK(t, "issue", "edit", "--title=Changed", created, "--remove-label", "ui", "--add-label", "fixed", "--add-assignee", "bob")
	ghTestOK(t, "issue", "comment", created, "-b", "literal $(touch nope) --json")
	raw = ghTestOK(t, "issue", "view", created, "--comments", "--json", "comments")
	if !strings.Contains(raw, "literal $(touch nope) --json") {
		t.Fatal(raw)
	}
	ghTestOK(t, "issue", "close", created)
	raw = ghTestOK(t, "issue", "list", "-s", "closed", "-l", "fixed", "-a", "bob", "--json", "id,state")
	if !strings.Contains(raw, created) || !strings.Contains(raw, "CLOSED") {
		t.Fatal(raw)
	}
	ghTestOK(t, "issue", "reopen", created)
	result, err := issueCommandJSON(t, "show", created, "--json")
	native := issueResultData(t, result, err)
	state := native["state"].(map[string]any)
	if state["title"] != "Changed" || state["status"] != "open" || native["comment_count"] != float64(1) {
		t.Fatalf("native state %#v", native)
	}
}
func TestGHCompatibilityPreservesNativeFields(t *testing.T) {
	root := issueCommandTestRepo(t)
	state, _ := CanonicalIssueState(IssueState{Title: "rich", Status: "open", Criteria: []IssueCriterion{{ID: "a", Text: "keep"}}, Metadata: map[string]string{"native/keep": "yes"}})
	input := issueCommandInput(t, root, state)
	result, err := issueCommandJSON(t, "open", "--input", input, "--json")
	id := issueResultData(t, result, err)["issue_id"].(string)
	ghTestOK(t, "issue", "edit", id, "--body", "updated")
	result, err = issueCommandJSON(t, "show", id, "--json")
	data := issueResultData(t, result, err)["state"].(map[string]any)
	if data["metadata"].(map[string]any)["native/keep"] != "yes" || len(data["criteria"].([]any)) != 1 {
		t.Fatal(data)
	}
}
func TestGHCompatibilityRejectsUnsupportedAndMalformedInput(t *testing.T) {
	issueCommandTestRepo(t)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	for _, args := range [][]string{
		{"pr", "list"}, {"issue", "list", "--repo", "x/y"}, {"issue", "create", "-t", "x", "--web"},
		{"issue", "list", "--json", "id,number"}, {"issue", "list", "--json", ""}, {"issue", "list", "-L", "0"},
		{"issue", "list", "--state", "bad"}, {"issue", "create", "--title"}, {"issue", "create", "-t", ""},
		{"issue", "comment", "12", "-b", "x"}, {"issue", "list", "--json", "id", "extra"},
		{"issue", "create", "-t", "x", "--json", "bad"},
	} {
		out, err := ghTestCall(t, args...)
		if err == nil {
			t.Fatalf("accepted %v (%s)", args, out)
		}
	}
	after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if before != after {
		t.Fatal("invalid inputs mutated refs")
	}
}
func TestGHCompatibilityOfflineHelp(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{"help"}, {"issue", "--help"}, {"issue", "list", "--help"}} {
		if out := ghTestOK(t, args...); !strings.Contains(out, "hn gh issue") {
			t.Fatal(out)
		}
	}
}
func TestGHCompatibilityListReadsBeyondFirstNativePage(t *testing.T) {
	issueCommandTestRepo(t)
	// Native pages contain 200 items. A filtered limit must apply after filtering.
	identity, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		state := IssueState{Title: "unmatched", Status: "open"}
		canonical, err := CanonicalIssueState(state)
		if err != nil {
			t.Fatal(err)
		}
		event, err := nextEvent(identity, "issue.open")
		if err != nil {
			t.Fatal(err)
		}
		event.Title = canonical.Title
		event.Issue = &canonical
		event.Intent = "open"
		if _, err := appendEvent(event, identity); err != nil {
			t.Fatal(err)
		}
	}
	_, _, cursor, err := executeIssueQuery(issueOptions{command: "list", details: true, limit: 200})
	if err != nil || cursor == "" {
		t.Fatalf("expected second page: %v", err)
	}
	raw, _, _, err := executeIssueQuery(issueOptions{command: "list", details: true, limit: 200, cursor: cursor})
	if err != nil {
		t.Fatal(err)
	}
	rows := raw.(issuePage).Items.([]issueDetail)
	last := rows[len(rows)-1]
	state := *last.State
	state.Title = "matched"
	state.Labels = []string{"needle"}
	if _, err := executeIssueMutation(issueOptions{command: "revise", id: last.ID, expected: []string{last.ID}, state: &state}); err != nil {
		t.Fatal(err)
	}
	out := ghTestOK(t, "issue", "list", "--label", "needle", "--limit", "1", "--json", "title")
	if !strings.Contains(out, "matched") {
		t.Fatal(out)
	}
}

func TestGHCompatibilityLiteralFlagValuesAndReadWithoutIdentity(t *testing.T) {
	root := issueCommandTestRepo(t)
	id := strings.TrimSpace(ghTestOK(t, "issue", "create", "--title", "--help", "--body", "--comments"))
	if !validEventID(id) {
		t.Fatal(id)
	}
	if err := os.RemoveAll(root + "/.git/hn"); err != nil {
		t.Fatal(err)
	}
	out := ghTestOK(t, "issue", "view", id, "--json", "title,body")
	if !strings.Contains(out, `"title":"--help"`) || !strings.Contains(out, `"body":"--comments"`) {
		t.Fatal(out)
	}
	ghTestOK(t, "issue", "list", "--json", "id")
}
func TestGHCompatibilityConflictRefusesMutation(t *testing.T) {
	issueCommandTestRepo(t)
	id := strings.TrimSpace(ghTestOK(t, "issue", "create", "--title", "base"))
	identity, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"left", "right"} {
		state, err := CanonicalIssueState(IssueState{Title: title, Status: "open"})
		if err != nil {
			t.Fatal(err)
		}
		event, err := nextEvent(identity, "issue.revise")
		if err != nil {
			t.Fatal(err)
		}
		event.Subject = id
		event.Parents = []string{id}
		event.Issue = &state
		event.Intent = "revise"
		if _, err = appendEvent(event, identity); err != nil {
			t.Fatal(err)
		}
	}
	out := ghTestOK(t, "issue", "view", id, "--json", "id,state,conflict")
	if !strings.Contains(out, `"state":null`) || !strings.Contains(out, `"conflict":true`) {
		t.Fatal(out)
	}
	out = ghTestOK(t, "issue", "list", "--json", "id,state,conflict")
	if !strings.Contains(out, id) || !strings.Contains(out, `"conflict":true`) {
		t.Fatal(out)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	for _, args := range [][]string{{"issue", "edit", id, "--title", "winner"}, {"issue", "close", id}, {"issue", "reopen", id}, {"issue", "comment", id, "--body", "new"}} {
		if _, err := ghTestCall(t, args...); err == nil {
			t.Fatalf("mutated conflict: %v", args)
		}
	}
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); before != after {
		t.Fatal("conflict mutation changed refs")
	}
}
