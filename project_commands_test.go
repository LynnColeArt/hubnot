package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projectCLI(t *testing.T, args ...string) (json.RawMessage, error) {
	t.Helper()
	out, err := captureTestOutput(t, func() error { return run(append(args, "--json")) })
	var e struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal([]byte(out), &e) != nil {
		t.Fatalf("invalid JSON %q (%v)", out, err)
	}
	if e.OK != (err == nil) {
		t.Fatalf("wrong status %s (%v)", out, err)
	}
	return e.Data, err
}
func projectOK(t *testing.T, args ...string) json.RawMessage {
	t.Helper()
	data, err := projectCLI(t, args...)
	if err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return data
}
func projectMutation(t *testing.T, args ...string) ResourceMutationResult {
	t.Helper()
	var r ResourceMutationResult
	if err := json.Unmarshal(projectOK(t, args...), &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func projectInput(t *testing.T, root, name string, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, name)
	if err = os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func projectView(t *testing.T, family, id string) ResourceView {
	t.Helper()
	var v ResourceView
	if err := json.Unmarshal(projectOK(t, family, "show", id), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestProjectCommandLifecycleAndReplay(t *testing.T) {
	root := issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "unusable-no-network")
	p := projectState()
	p.Metadata = map[string]string{"native/keep": "yes", "forge/source": "descriptive"}
	opened := projectMutation(t, "project", "open", "--input", projectInput(t, root, "project.json", p), "--operation", "p-open")
	edited := projectMutation(t, "project", "revise", opened.ResourceID, "--expect", opened.EventID, "--title", "Changed", "--operation", "p-edit")
	v := projectView(t, "project", opened.ResourceID)
	var state ProjectState
	_ = json.Unmarshal(v.State, &state)
	if state.Title != "Changed" || state.Metadata["native/keep"] != "yes" || len(state.Fields) != 2 || len(state.Views) != 1 {
		t.Fatalf("lost fields/provenance: %s", v.State)
	}
	projectMutation(t, "project", "revise", opened.ResourceID, "--expect", edited.EventID, "--body", "later")
	replay := projectMutation(t, "project", "revise", opened.ResourceID, "--expect", opened.EventID, "--title", "Changed", "--operation", "p-edit")
	if !replay.Replayed || replay.EventID != edited.EventID {
		t.Fatal("immutable replay lost")
	}
	if _, err := projectCLI(t, "project", "revise", opened.ResourceID, "--expect", opened.EventID, "--title", "stale"); err == nil {
		t.Fatal("stale edit accepted")
	}
	wrong := testIdentity(t, "Wrong")
	if _, err := projectCLI(t, "project", "open", "--title", "wrong", "--actor", wrong.Actor); err == nil {
		t.Fatal("actor mismatch accepted")
	}
}
func TestProjectItemsPinSchemaAndSortNumbers(t *testing.T) {
	root := issueCommandTestRepo(t)
	p := projectState()
	opened := projectMutation(t, "project", "open", "--input", projectInput(t, root, "p.json", p))
	values := map[string]ProjectValue{"score": {Type: "number", Number: "1.25"}, "phase": {Type: "single_select", OptionID: "todo"}}
	vp := projectInput(t, root, "values.json", values)
	var want []string
	for _, pos := range []string{"10", "2", "-1.25", "0.01", "0.001"} {
		r := projectMutation(t, "project", "item", "add", "--project", opened.ResourceID, "--draft-title", "draft", "--input", vp, "--position", pos, "--operation", "item"+pos)
		want = append(want, r.ResourceID)
	}
	raw := projectOK(t, "project-item", "list", "--project", opened.ResourceID, "--limit", "2")
	var page struct {
		Items      []ResourceView `json:"items"`
		NextCursor string         `json:"next_cursor"`
	}
	_ = json.Unmarshal(raw, &page)
	if len(page.Items) != 2 || page.Items[0].ID != want[2] || page.Items[1].ID != want[4] || page.NextCursor == "" {
		t.Fatalf("not numeric order %s", raw)
	}
	raw = projectOK(t, "project", "item", "list", "--project", opened.ResourceID, "--limit", "2", "--cursor", page.NextCursor)
	_ = json.Unmarshal(raw, &page)
	if page.Items[0].ID != want[3] || page.Items[1].ID != want[1] {
		t.Fatalf("bad continuation %s", raw)
	}
	p.Fields[0].Type = "text"
	newer := projectMutation(t, "project", "revise", opened.ResourceID, "--expect", opened.EventID, "--input", projectInput(t, root, "new.json", p))
	replay := projectMutation(t, "project-item", "add", "--project", opened.ResourceID, "--draft-title", "draft", "--input", vp, "--position", "10", "--operation", "item10")
	if !replay.Replayed || replay.ResourceID != want[0] {
		t.Fatal("implicit schema repinned on replay")
	}
	v := projectView(t, "project-item", want[0])
	var state ProjectItemState
	_ = json.Unmarshal(v.State, &state)
	if state.ProjectRevision != opened.EventID {
		t.Fatal("historical schema changed")
	}
	if _, err := projectCLI(t, "project-item", "add", "--project", opened.ResourceID, "--draft-title", "bad", "--input", vp); err == nil {
		t.Fatal("number coerced into current text field")
	}
	state.ProjectRevision = newer.EventID
	state.Values["score"] = ProjectValue{Type: "text", Text: "migrated"}
	state.Metadata = map[string]string{"native/keep": "yes"}
	revised := projectMutation(t, "project-item", "revise", v.ID, "--expect", v.Heads[0].ID, "--input", projectInput(t, root, "item.json", state))
	archived := projectMutation(t, "project", "item", "archive", v.ID, "--expect", revised.EventID, "--operation", "archive")
	av := projectView(t, "project-item", v.ID)
	_ = json.Unmarshal(av.State, &state)
	if !state.Archived || state.Metadata["native/keep"] != "yes" || state.ProjectRevision != newer.EventID {
		t.Fatalf("archive lost state %s", av.State)
	}
	if replay := projectMutation(t, "project-item", "archive", v.ID, "--expect", revised.EventID, "--operation", "archive"); !replay.Replayed || replay.EventID != archived.EventID {
		t.Fatal("archive replay failed")
	}
	if _, err := projectCLI(t, "project-item", "list", "--project", opened.ResourceID, "--limit", "2", "--cursor", page.NextCursor); err == nil {
		t.Fatal("stale page accepted")
	}
}
func TestProjectItemTypedTargetsAndHostileValues(t *testing.T) {
	root := issueCommandTestRepo(t)
	p := projectState()
	p.Fields = append(p.Fields, ProjectField{ID: "date", Name: "Date", Type: "date"}, ProjectField{ID: "notes", Name: "Notes", Type: "text"})
	opened := projectMutation(t, "project", "open", "--input", projectInput(t, root, "p.json", p))
	issueState, _ := CanonicalIssueState(IssueState{Title: "native", Status: "open"})
	ir, err := issueCommandJSON(t, "open", "--input", issueCommandInput(t, root, issueState), "--json")
	issueID := issueResultData(t, ir, err)["issue_id"].(string)
	discussion, err := MutateResource(ResourceMutation{Family: ResourceDiscussion, Intent: "open", State: resourceState(t, ResourceDiscussion, DiscussionState{Schema: "hn.discussion/1", Title: "thread", Category: "general", Status: "open"})})
	if err != nil {
		t.Fatal(err)
	}
	values := projectInput(t, root, "all-values.json", map[string]ProjectValue{"date": {Type: "date", Date: "2024-02-29"}, "notes": {Type: "text", Text: "native notes"}, "score": {Type: "number", Number: "-2.5"}, "phase": {Type: "single_select", OptionID: "todo"}})
	for _, target := range []string{"issue:" + issueID, "discussion:" + discussion.ResourceID, "external:https://example.test/task"} {
		r := projectMutation(t, "project-item", "add", "--project", opened.ResourceID, "--target", target, "--input", values)
		if projectView(t, "project-item", r.ResourceID).Conflict {
			t.Fatal("new item conflicted")
		}
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	for _, raw := range []string{`{"score":{"type":"number","number":"NaN"}}`, `{"date":{"type":"date","date":"2025-02-29"}}`, `{"phase":{"type":"single_select","option_id":"missing"}}`, `{"score":{"type":"number","number":"1","number":"2"}}`, `{"score":{"type":"number","number":"1","text":""}}`, `{"unknown":{"type":"text","text":"x"}}`} {
		path := filepath.Join(root, "hostile.json")
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := projectCLI(t, "project-item", "add", "--project", opened.ResourceID, "--draft-title", "bad", "--input", path); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, args := range [][]string{{"project-item", "add", "--project", opened.ResourceID, "--target", "issue:" + discussion.ResourceID}, {"project-item", "add", "--project", opened.ResourceID, "--target", "external:https://user:pass@example.test"}, {"project-item", "add", "--project", opened.ResourceID, "--draft-title", "x", "--target", "issue:" + issueID}, {"project", "revise", opened.ResourceID, "--title", "missing guard"}, {"project", "open", "--title", "x", "--unsupported"}} {
		if _, err := captureTestOutput(t, func() error { return run(args) }); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("hostile input wrote facts")
	}
}
func TestProjectCommandConflictsAndExplicitResolution(t *testing.T) {
	root := issueCommandTestRepo(t)
	opened := projectMutation(t, "project", "open", "--title", "Conflict")
	first, _ := loadIdentity()
	events, _, _ := collectIssueEvents()
	second := testIdentity(t, "Other")
	stale := append([]StoredEvent{}, events...)
	a, _ := nextEvent(first, "project.revise")
	b, _ := nextEvent(second, "project.revise")
	input := ResourceMutation{Family: ResourceProject, Intent: "revise", ID: opened.ResourceID, Expected: []string{opened.EventID}, State: resourceState(t, ResourceProject, projectState())}
	ar, err := applyResourceMutation(input, first, a, stale)
	if err != nil {
		t.Fatal(err)
	}
	br, err := applyResourceMutation(input, second, b, stale)
	if err != nil {
		t.Fatal(err)
	}
	v := projectView(t, "project", opened.ResourceID)
	if !v.Conflict || string(v.State) != "null" {
		t.Fatalf("hidden conflict %+v", v)
	}
	if _, err := projectCLI(t, "project-item", "add", "--project", opened.ResourceID, "--draft-title", "no winner"); err == nil {
		t.Fatal("selected conflicted schema")
	}
	projectMutation(t, "project-item", "add", "--project", opened.ResourceID, "--project-revision", opened.EventID, "--draft-title", "explicit historical schema")
	projectMutation(t, "project", "resolve", opened.ResourceID, "--expect", ar.EventID, "--expect", br.EventID, "--input", projectInput(t, root, "resolved.json", projectState()))
	if projectView(t, "project", opened.ResourceID).Conflict {
		t.Fatal("resolution not visible")
	}
}
func TestProjectHelpIsOffline(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{"project", "help"}, {"project-item", "help"}, {"project", "item", "help"}} {
		out, err := captureTestOutput(t, func() error { return run(args) })
		if err != nil || !strings.Contains(out, "--project") {
			t.Fatalf("help %q %v", out, err)
		}
	}
}

func TestProjectItemConflictsRequireAllHeadsAndKeepProvenance(t *testing.T) {
	root := issueCommandTestRepo(t)
	p := projectMutation(t, "project", "open", "--title", "Board")
	item := projectMutation(t, "project-item", "add", "--project", p.ResourceID, "--draft-title", "draft")
	first, _ := loadIdentity()
	second := testIdentity(t, "Other item editor")
	events, _, _ := collectIssueEvents()
	a, _ := nextEvent(first, "project-item.revise")
	b, _ := nextEvent(second, "project-item.revise")
	var state ProjectItemState
	_ = json.Unmarshal(projectView(t, "project-item", item.ResourceID).State, &state)
	state.Position = "2"
	state.Metadata = map[string]string{"forge/source": "retained"}
	input := ResourceMutation{Family: ResourceProjectItem, Intent: "revise", ID: item.ResourceID, Expected: []string{item.EventID}, State: resourceState(t, ResourceProjectItem, state)}
	ar, err := applyResourceMutation(input, first, a, events)
	if err != nil {
		t.Fatal(err)
	}
	state.Position = "10"
	input.State = resourceState(t, ResourceProjectItem, state)
	br, err := applyResourceMutation(input, second, b, events)
	if err != nil {
		t.Fatal(err)
	}
	v := projectView(t, "project-item", item.ResourceID)
	if !v.Conflict || len(v.Heads) != 2 || string(v.State) != "null" {
		t.Fatalf("hidden item conflict %+v", v)
	}
	if _, err := projectCLI(t, "project-item", "archive", item.ResourceID, "--expect", ar.EventID); err == nil {
		t.Fatal("archive picked a conflict head")
	}
	path := projectInput(t, root, "resolve-item.json", state)
	if _, err := projectCLI(t, "project-item", "resolve", item.ResourceID, "--expect", ar.EventID, "--input", path); err == nil {
		t.Fatal("partial resolution accepted")
	}
	resolved := projectMutation(t, "project-item", "resolve", item.ResourceID, "--expect", ar.EventID, "--expect", br.EventID, "--input", path, "--operation", "resolve-item")
	replay := projectMutation(t, "project-item", "resolve", item.ResourceID, "--expect", br.EventID, "--expect", ar.EventID, "--input", path, "--operation", "resolve-item")
	if !replay.Replayed || replay.EventID != resolved.EventID {
		t.Fatal("resolution replay failed")
	}
	v = projectView(t, "project-item", item.ResourceID)
	_ = json.Unmarshal(v.State, &state)
	if v.Conflict || state.Metadata["forge/source"] != "retained" || state.Position != "10" {
		t.Fatalf("bad resolution %s", v.State)
	}
}

func TestProjectItemProposalTargetAndReadWithoutIdentity(t *testing.T) {
	issueCommandTestRepo(t)
	identity, _ := loadIdentity()
	event, err := nextEvent(identity, "proposal.open")
	if err != nil {
		t.Fatal(err)
	}
	event.Title = "plan"
	event.Base = mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "proposal feature")
	event.Head = mustGitText(t, "rev-parse", "HEAD")
	proposal, err := appendEvent(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	p := projectMutation(t, "project", "open", "--title", "Plan")
	item := projectMutation(t, "project-item", "add", "--project", p.ResourceID, "--target", "proposal:"+proposal.ID, "--operation", "proposal-item")
	if _, err := projectCLI(t, "project-item", "add", "--project", p.ResourceID, "--draft-title", "changed intent", "--operation", "proposal-item"); err == nil {
		t.Fatal("operation key accepted different content")
	}
	if _, err := projectCLI(t, "project", "open", "--title", "other kind", "--operation", "proposal-item"); err == nil {
		t.Fatal("actor-global operation key reused across families")
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(paths.active); err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(paths.identities); err != nil {
		t.Fatal(err)
	}
	if _, err = loadIdentity(); err == nil {
		t.Fatal("test retained signing identity")
	}
	projectOK(t, "project", "show", p.ResourceID)
	projectOK(t, "project-item", "show", item.ResourceID)
	projectOK(t, "project-item", "list", "--project", p.ResourceID)
}

func TestProjectLiteralFlagsAndNumericPositionPrecision(t *testing.T) {
	issueCommandTestRepo(t)
	p := projectMutation(t, "project", "open", "--title", "--input", "--body", "literal --json and $(no-op)")
	v := projectView(t, "project", p.ResourceID)
	var state ProjectState
	_ = json.Unmarshal(v.State, &state)
	if state.Title != "--input" || state.Body != "literal --json and $(no-op)" {
		t.Fatal("literal flag value reinterpreted")
	}
	a := projectMutation(t, "project-item", "add", "--project", p.ResourceID, "--draft-title", "a", "--position", "0.0000000000000000000000000000000000000002")
	b := projectMutation(t, "project-item", "add", "--project", p.ResourceID, "--draft-title", "b", "--position", "0.0000000000000000000000000000000000000001")
	raw := projectOK(t, "project-item", "list", "--project", p.ResourceID)
	var page struct {
		Items []ResourceView `json:"items"`
	}
	_ = json.Unmarshal(raw, &page)
	if len(page.Items) != 2 || page.Items[0].ID != b.ResourceID || page.Items[1].ID != a.ResourceID {
		t.Fatalf("decimal precision lost %s", raw)
	}
	for _, pos := range []string{"", "NaN", "1e5", "01", "1.0", "-0", "+1"} {
		if _, err := projectCLI(t, "project-item", "add", "--project", p.ResourceID, "--draft-title", "invalid", "--position", pos); err == nil {
			t.Fatalf("accepted position %q", pos)
		}
	}
}
