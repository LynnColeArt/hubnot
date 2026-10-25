package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resourceState(t *testing.T, family string, state any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = CanonicalResourceState(family, raw)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func projectState() ProjectState {
	return ProjectState{Schema: "hn.project/1", Title: "Roadmap", Fields: []ProjectField{{ID: "score", Name: "Score", Type: "number"}, {ID: "phase", Name: "Phase", Type: "single_select", Options: []ProjectOption{{ID: "todo", Name: "Todo"}}}}, Views: []ProjectView{{ID: "board", Name: "Board", Layout: "board", GroupByField: "phase"}}}
}
func TestResourceStrictStates(t *testing.T) {
	good := resourceState(t, "project", projectState())
	if len(good) == 0 {
		t.Fatal("empty")
	}
	for _, raw := range []string{`{"schema":"hn.project/1","title":"x","title":"y"}`, `{"schema":"hn.project/1","title":"x","unknown":1}`, `{"schema":"hn.project/1","title":"\ud800"}`, `{"schema":"hn.project/1","title":null}`} {
		if _, err := CanonicalResourceState("project", []byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	p := projectState()
	p.Fields[0].ID = "../score"
	if _, err := CanonicalResourceState("project", resourceJSON(p)); err == nil {
		t.Fatal("unsafe ID")
	}
	for _, number := range []string{"NaN", "1e3", "01", "1.00", "-0", ".2", strings.Repeat("9", 65)} {
		s := ProjectItemState{Schema: "hn.project-item/1", ProjectID: eventID([]byte("p")), ProjectRevision: eventID([]byte("p")), Target: ProjectTarget{Kind: "draft", Title: "draft"}, Values: map[string]ProjectValue{"score": {Type: "number", Number: number}}}
		if _, err := CanonicalResourceState("project-item", resourceJSON(s)); err == nil {
			t.Fatalf("bad decimal %s", number)
		}
	}
	w := WorkflowState{Schema: "hn.workflow/1", Name: "ci", Title: "CI", Enabled: true}
	if _, err := CanonicalResourceState("workflow", resourceJSON(w)); err == nil {
		t.Fatal("enabled foreign only")
	}
	w.Enabled = false
	w.SourceDocuments = []WorkflowSourceDocument{{Format: "actions-yaml", Content: "name: inert", Digest: eventID([]byte("wrong"))}}
	if _, err := CanonicalResourceState("workflow", resourceJSON(w)); err == nil {
		t.Fatal("wrong digest")
	}
}
func resourceJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func resourceFixture(t *testing.T, label, family, root string, parents []string, state any) StoredEvent {
	t.Helper()
	intent := "open"
	kind := family + ".open"
	if root != "" {
		intent = "revise"
		kind = family + ".revise"
		if len(parents) > 1 {
			intent = "resolve"
		}
	}
	return StoredEvent{ID: eventID([]byte(label)), Event: Event{Kind: kind, Actor: eventID([]byte("actor")), Subject: root, Parents: parents, Intent: intent, Body: string(resourceState(t, family, state))}}
}
func TestResourceCatalogConflictAndSchemaPin(t *testing.T) {
	p := resourceFixture(t, "p", "project", "", nil, projectState())
	a := projectState()
	a.Title = "one"
	b := projectState()
	b.Title = "two"
	r1 := resourceFixture(t, "a", "project", p.ID, []string{p.ID}, a)
	r2 := resourceFixture(t, "b", "project", p.ID, []string{p.ID}, b)
	c, err := BuildResourceCatalog([]StoredEvent{r2, p, r1})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Resources[p.ID].Conflict || c.Resources[p.ID].State != nil {
		t.Fatal("hidden conflict")
	}
	r3 := resourceFixture(t, "c", "project", p.ID, sortedUniqueStrings(r1.ID, r2.ID), a)
	if _, err = BuildResourceCatalog([]StoredEvent{p, r1, r2, r3}); err != nil {
		t.Fatal(err)
	}
	item := ProjectItemState{Schema: "hn.project-item/1", ProjectID: p.ID, ProjectRevision: p.ID, Target: ProjectTarget{Kind: "draft", Title: "draft"}, Values: map[string]ProjectValue{"score": {Type: "number", Number: "1.5"}, "phase": {Type: "single_select", OptionID: "todo"}}}
	i := resourceFixture(t, "i", "project-item", "", nil, item)
	if _, err = BuildResourceCatalog([]StoredEvent{p, r1, r2, i}); err != nil {
		t.Fatal(err)
	}
	item.Values["phase"] = ProjectValue{Type: "single_select", OptionID: "missing"}
	i = resourceFixture(t, "bad", "project-item", "", nil, item)
	if _, err = BuildResourceCatalog([]StoredEvent{p, i}); err == nil {
		t.Fatal("wrong option")
	}
}
func TestResourceRepliesPinAnswerAndThread(t *testing.T) {
	d := resourceFixture(t, "thread", "discussion", "", nil, DiscussionState{Schema: "hn.discussion/1", Title: "Question", Category: "help", Status: "open"})
	r := resourceFixture(t, "reply", "discussion-reply", "", nil, DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: d.ID, Body: "original"})
	e := resourceFixture(t, "reply-edit", "discussion-reply", r.ID, []string{r.ID}, DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: d.ID, Body: "changed"})
	answer := resourceFixture(t, "answer", "discussion", d.ID, []string{d.ID}, DiscussionState{Schema: "hn.discussion/1", Title: "Question", Category: "help", Status: "open", AcceptedReplyRevision: r.ID})
	c, err := BuildResourceCatalog([]StoredEvent{answer, e, r, d})
	if err != nil {
		t.Fatal(err)
	}
	var state DiscussionState
	if err = DecodeResourceState("discussion", c.Resources[d.ID].State, &state); err != nil || state.AcceptedReplyRevision != r.ID {
		t.Fatal("answer moved", err)
	}
	other := resourceFixture(t, "other", "discussion", "", nil, DiscussionState{Schema: "hn.discussion/1", Title: "Other", Category: "help", Status: "open"})
	wrong := resourceFixture(t, "wrong", "discussion-reply", "", nil, DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: other.ID, ReplyTo: r.ID, Body: "bad"})
	if _, err = BuildResourceCatalog([]StoredEvent{d, r, other, wrong}); err == nil {
		t.Fatal("cross thread reply")
	}
}
func TestResourceNativeMutationReplayAndIssueOperationScope(t *testing.T) {
	issueCommandTestRepo(t)
	state := resourceState(t, "project", projectState())
	in := ResourceMutation{Family: "project", Intent: "open", State: state, Operation: "project-open"}
	first, err := MutateResource(in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := MutateResource(in)
	if err != nil || !again.Replayed || first.EventID != again.EventID {
		t.Fatal("replay", err)
	}
	in.Family = "discussion"
	in.State = resourceState(t, "discussion", DiscussionState{Schema: "hn.discussion/1", Title: "Topic", Category: "general", Status: "open"})
	if _, err = MutateResource(in); err == nil {
		t.Fatal("actor-global reuse")
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	identity, _ := loadIdentity()
	if _, err = observeIssueOperation(events, identity.Actor, "project-open"); err == nil {
		t.Fatal("resource leaked through issue operation")
	}
	changed := projectState()
	changed.Title = "edited"
	rev := ResourceMutation{Family: "project", Intent: "revise", ID: first.ResourceID, Expected: []string{first.EventID}, State: resourceState(t, "project", changed), Operation: "edit"}
	out, err := MutateResource(rev)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = MutateResource(ResourceMutation{Family: "project", Intent: "revise", ID: first.ResourceID, Expected: []string{first.EventID}, State: state}); err == nil {
		t.Fatal("stale")
	}
	again, err = MutateResource(rev)
	if err != nil || !again.Replayed || again.EventID != out.EventID {
		t.Fatal("immutable replay", err)
	}
}

func TestResourceAuthenticatedPayloadAndAuthority(t *testing.T) {
	identity := testIdentity(t, "Resource signer")
	event := newEvent(identity, "project.open", 1, "")
	event.Intent = "open"
	event.Body = string(resourceState(t, "project", projectState()))
	payload, sig, err := encodeAndSign(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = verifyEvent(payload, sig); err != nil {
		t.Fatal(err)
	}
	private, _ := identity.privateKey()
	for _, raw := range [][]byte{append(append([]byte{}, payload[:len(payload)-1]...), []byte(`,"kind":"project.open"}`)...), append(append([]byte{}, payload[:len(payload)-1]...), []byte(`,"extra":"invisible"}`)...), append(append([]byte{}, payload[:len(payload)-1]...), []byte(`,"Actor":"other"}`)...), append(append([]byte{}, payload[:len(payload)-1]...), []byte(`,"runner":"trusted"}`)...), append(append([]byte{}, payload[:len(payload)-1]...), []byte(strings.Repeat(" ", maxResourcePayload)+"}")...)} {
		if _, _, err = verifyEvent(raw, ed25519.Sign(private, raw)); err == nil {
			t.Fatal("authenticated hostile resource accepted")
		}
	}
	for _, mutate := range []func(*Event){func(e *Event) { e.Title = "smuggled" }, func(e *Event) { e.Policy = eventID([]byte("policy")) }, func(e *Event) { e.Issue = &IssueState{Title: "x", Status: "open"} }, func(e *Event) { e.Evidence = []string{eventID([]byte("fact"))} }, func(e *Event) { e.TargetActor = identity.Actor }} {
		bad := event
		mutate(&bad)
		if _, _, err = encodeAndSign(bad, identity); err == nil {
			t.Fatal("resource gained unrelated authority field")
		}
	}
	workflow := newEvent(identity, "workflow.open", 1, "")
	workflow.Intent = "open"
	source := strings.Repeat("x", 300<<10)
	workflow.Body = string(resourceState(t, "workflow", WorkflowState{Schema: "hn.workflow/1", Name: "inert", Title: "Inert", SourceDocuments: []WorkflowSourceDocument{{Format: "actions-yaml", Content: source, Digest: eventID([]byte(source))}}}))
	raw, signature, err := encodeAndSign(workflow, identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) <= maxResourcePayload {
		t.Fatal("test did not cross ordinary bound")
	}
	if _, _, err = verifyEvent(raw, signature); err != nil {
		t.Fatal("workflow bound", err)
	}
}
func TestResourceRelationshipRejectsWrongLineageCycleAndDepth(t *testing.T) {
	p := resourceFixture(t, "p", "project", "", nil, projectState())
	other := resourceFixture(t, "other-project", "project", "", nil, projectState())
	bad := resourceFixture(t, "wrong-root", "project", p.ID, []string{other.ID}, projectState())
	if _, err := BuildResourceCatalog([]StoredEvent{p, other, bad}); err == nil {
		t.Fatal("wrong lineage")
	}
	a := resourceFixture(t, "cycle-a", "project", p.ID, []string{eventID([]byte("cycle-b"))}, projectState())
	b := resourceFixture(t, "cycle-b", "project", p.ID, []string{a.ID}, projectState())
	if _, err := BuildResourceCatalog([]StoredEvent{p, a, b}); err == nil {
		t.Fatal("revision cycle")
	}
	d := resourceFixture(t, "d", "discussion", "", nil, DiscussionState{Schema: "hn.discussion/1", Title: "Thread", Category: "q", Status: "open"})
	events := []StoredEvent{d}
	parent := ""
	for i := 0; i < 9; i++ {
		r := resourceFixture(t, fmt.Sprint("reply", i), "discussion-reply", "", nil, DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: d.ID, ReplyTo: parent, Body: "reply"})
		events = append(events, r)
		parent = r.ID
		if _, err := BuildResourceCatalog(events); (i < 8 && err != nil) || (i == 8 && err == nil) {
			t.Fatalf("depth %d: %v", i+1, err)
		}
	}
}
func TestResourceCommandsDispatchPaginationAndSpecialization(t *testing.T) {
	root := issueCommandTestRepo(t)
	path := filepath.Join(root, "project.json")
	if err := os.WriteFile(path, resourceState(t, "project", projectState()), 0600); err != nil {
		t.Fatal(err)
	}
	var opened ResourceMutationResult
	out, err := captureTestOutput(t, func() error {
		return run([]string{"project", "open", "--input", path, "--operation", "cli-open", "--json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal([]byte(out), &envelope); err != nil || !envelope.OK {
		t.Fatal(out, err)
	}
	if err = json.Unmarshal(envelope.Data, &opened); err != nil {
		t.Fatal(err)
	}
	if _, err = captureTestOutput(t, func() error { return run([]string{"project", "show", opened.ResourceID, "--json"}) }); err != nil {
		t.Fatal(err)
	}
	if _, err = MutateResource(ResourceMutation{Family: "project", Intent: "open", State: resourceState(t, "project", projectState())}); err != nil {
		t.Fatal(err)
	}
	data, err := queryResource("project", "list", "", "", "", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	page := data.(resourcePage)
	if page.NextCursor == "" {
		t.Fatal("no cursor")
	}
	if _, err = queryResource("project", "list", "", "", "", 1, page.NextCursor); err != nil {
		t.Fatal(err)
	}
	_, err = MutateResource(ResourceMutation{Family: "project", Intent: "open", State: resourceState(t, "project", projectState())})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = queryResource("project", "list", "", "", "", 1, page.NextCursor); err == nil {
		t.Fatal("stale cursor")
	}
	called := false
	NativeResourceCommands["project"] = func(args []string) error { called = true; return nil }
	defer delete(NativeResourceCommands, "project")
	if err = run([]string{"project", "special"}); err != nil || !called {
		t.Fatal("specialization not selected", err)
	}
}

func TestResourceCollectionDateAndContainerBoundaries(t *testing.T) {
	bad := projectState()
	for len(bad.Fields) < 65 {
		bad.Fields = append(bad.Fields, ProjectField{ID: fmt.Sprint("field", len(bad.Fields)), Name: "Field", Type: "text"})
	}
	if _, err := ResourceStateJSON("project", bad); err == nil {
		t.Fatal("field collection bound")
	}
	bad = projectState()
	bad.Title = string([]byte{0xff})
	if _, err := ResourceStateJSON("project", bad); err == nil {
		t.Fatal("typed invalid UTF-8 repaired")
	}
	for _, date := range []string{"2026-02-29", "2024-2-29", "2024-02-30", "2024-02-29T00:00:00Z"} {
		if err := validateProjectValue(ProjectValue{Type: "date", Date: date}); err == nil {
			t.Fatal("bad date", date)
		}
	}
	if err := validateProjectValue(ProjectValue{Type: "date", Date: "2024-02-29"}); err != nil {
		t.Fatal(err)
	}
	p := resourceFixture(t, "project-a", "project", "", nil, projectState())
	q := resourceFixture(t, "project-b", "project", "", nil, projectState())
	s := ProjectItemState{Schema: "hn.project-item/1", ProjectID: p.ID, ProjectRevision: p.ID, Target: ProjectTarget{Kind: "draft", Title: "draft"}}
	item := resourceFixture(t, "item", "project-item", "", nil, s)
	s.ProjectID = q.ID
	s.ProjectRevision = q.ID
	edit := resourceFixture(t, "move", "project-item", item.ID, []string{item.ID}, s)
	if _, err := BuildResourceCatalog([]StoredEvent{p, q, item, edit}); err == nil {
		t.Fatal("item changed owning project")
	}
	d := resourceFixture(t, "thread", "discussion", "", nil, DiscussionState{Schema: "hn.discussion/1", Title: "Thread", Category: "q", Status: "open"})
	r := resourceFixture(t, "self", "discussion-reply", "", nil, DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: d.ID, ReplyTo: eventID([]byte("self")), Body: "cycle"})
	if _, err := BuildResourceCatalog([]StoredEvent{d, r}); err == nil {
		t.Fatal("self nesting cycle")
	}
}
func TestResourceCrossActorConflictAndOperationKeys(t *testing.T) {
	issueCommandTestRepo(t)
	firstActor, _ := loadIdentity()
	state := resourceState(t, "project", projectState())
	opened, err := MutateResource(ResourceMutation{Family: "project", Intent: "open", State: state, Operation: "same-key"})
	if err != nil {
		t.Fatal(err)
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	secondActor := testIdentity(t, "Collaborator")
	writeActiveTestIdentity(t, secondActor)
	if _, err = MutateResource(ResourceMutation{Family: "project", Intent: "open", State: state, Operation: "same-key"}); err != nil {
		t.Fatal("different actor key should be independent", err)
	}
	staleEvents := append([]StoredEvent{}, events...)
	a, err := nextEvent(firstActor, "project.revise")
	if err != nil {
		t.Fatal(err)
	}
	b, err := nextEvent(secondActor, "project.revise")
	if err != nil {
		t.Fatal(err)
	}
	in := ResourceMutation{Family: "project", Intent: "revise", ID: opened.ResourceID, Expected: []string{opened.EventID}, State: state}
	if _, err = applyResourceMutation(in, firstActor, a, staleEvents); err != nil {
		t.Fatal(err)
	}
	if _, err = applyResourceMutation(in, secondActor, b, staleEvents); err != nil {
		t.Fatal(err)
	}
	events, _, err = collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildResourceCatalog(events)
	if err != nil || !cat.Resources[opened.ResourceID].Conflict {
		t.Fatal("cross actor edits hidden", err)
	}
	view := cat.Resources[opened.ResourceID]
	parents := []string{}
	for _, h := range view.Heads {
		parents = append(parents, h.ID)
	}
	in.Intent = "resolve"
	in.Expected = parents
	if _, err = MutateResource(in); err != nil {
		t.Fatal("resolve", err)
	}
}

func TestResourceRejectsInapplicableEmptyUnionMembers(t *testing.T) {
	for _, raw := range []string{
		`{"schema":"hn.project-item/1","project_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","project_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","target":{"kind":"draft","title":"draft","id":""}}`,
		`{"schema":"hn.project-item/1","project_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","project_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","target":{"kind":"draft","title":"draft"},"values":{"score":{"type":"number","number":"1","text":""}}}`,
		`{"schema":"hn.project/1","title":"x","fields":[{"id":"text","name":"Text","type":"text","options":[]}]}`,
	} {
		family := "project-item"
		if strings.Contains(raw, `"schema":"hn.project/1"`) {
			family = "project"
		}
		if _, err := CanonicalResourceState(family, []byte(raw)); err == nil {
			t.Fatal("accepted inapplicable empty member", raw)
		}
	}
}
