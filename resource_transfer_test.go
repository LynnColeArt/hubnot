package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/forge"
)

type bridgeProvider struct {
	graph forge.Graph
	reads int
}

func (p *bridgeProvider) Kind() string     { return "bundlefake" }
func (p *bridgeProvider) Endpoint() string { return "bundle:opaque-test-store" }
func (p *bridgeProvider) ReadProjects(context.Context, forge.Selection) (forge.Graph, error) {
	p.reads++
	return p.graph, nil
}
func (p *bridgeProvider) ReadDiscussions(context.Context, forge.Selection) (forge.Graph, error) {
	p.reads++
	return p.graph, nil
}
func (p *bridgeProvider) ReadWorkflows(context.Context, forge.Selection) (forge.Graph, error) {
	p.reads++
	return p.graph, nil
}
func bridgeRecord(id string) forge.Record {
	return forge.Record{ID: id, Key: "opaque:" + id, Revision: "rev-A", Author: "external writer", CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-01-02T03:04:05Z"}
}
func bridgeProjectGraph() forge.Graph {
	return forge.Graph{Nodes: []forge.Node{
		{Kind: "project", Record: bridgeRecord("board-alpha"), Project: &forge.Project{Title: "Roadmap", Fields: []forge.Field{{ID: "status", Name: "Status", Type: "single_select", Options: []forge.Option{{ID: "todo", Name: "Todo"}}}}, Views: []forge.View{{ID: "board", Name: "Board", Layout: "board", GroupByField: "status"}}}},
		{Kind: "project-item", Record: bridgeRecord("card-Z"), Item: &forge.ProjectItem{Project: forge.Ref{Kind: "project", ID: "board-alpha"}, Target: forge.Target{Kind: "draft", Title: "Implement"}, Values: map[string]forge.Value{"status": {Type: "single_select", OptionID: "todo"}}, Position: "1"}},
	}}
}
func bridgeDiscussionGraph() forge.Graph {
	return forge.Graph{Nodes: []forge.Node{
		{Kind: "discussion", Record: bridgeRecord("topic-A"), Discussion: &forge.Discussion{Title: "Question", Body: "Original question", Category: "help", Status: "open", AcceptedReply: forge.Ref{Kind: "discussion-reply", ID: "answer-Z", Revision: "rev-A"}}},
		{Kind: "discussion-reply", Record: bridgeRecord("answer-Z"), Reply: &forge.Reply{Discussion: forge.Ref{Kind: "discussion", ID: "topic-A"}, Body: "Answer"}},
		{Kind: "discussion-reply", Record: bridgeRecord("nested-P"), Reply: &forge.Reply{Discussion: forge.Ref{Kind: "discussion", ID: "topic-A"}, Parent: forge.Ref{Kind: "discussion-reply", ID: "answer-Z"}, Body: "Nested"}},
	}}
}
func bridgeRegister(t *testing.T, p *bridgeProvider) {
	t.Helper()
	old, exists := forgeProviders[p.Kind()]
	t.Cleanup(func() {
		if exists {
			forgeProviders[p.Kind()] = old
		} else {
			delete(forgeProviders, p.Kind())
		}
	})
	forgeProviders[p.Kind()] = forgeRegistration{NewResource: func(endpoint, token string) (forge.Identity, error) {
		if token != "" {
			t.Fatal("cross-provider credential")
		}
		return p, nil
	}, Capabilities: []forge.Capability{{Resource: forge.ResourceProject, Import: true, Export: true}, {Resource: forge.ResourceDiscussion, Import: true, Export: true}, {Resource: forge.ResourceWorkflow, Import: true, Export: true}}}
}
func bridgeOptions(resource forge.Resource) transferOptions {
	return transferOptions{command: "import", provider: "bundlefake", project: "opaque-space", key: "opaque-key", resource: resource}
}
func bridgeCatalog(t *testing.T) *ResourceCatalog {
	t.Helper()
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildResourceCatalog(events)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}
func bridgeFamily(cat *ResourceCatalog, family string) []*ResourceView {
	out := []*ResourceView{}
	for _, id := range cat.IDs {
		if cat.Resources[id].Family == family {
			out = append(out, cat.Resources[id])
		}
	}
	return out
}
func TestResourceBridgeProjectGraphAndRepeat(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	bridgeRegister(t, p)
	if err := runForgeTransfer(bridgeOptions(forge.ResourceProject)); err != nil {
		t.Fatal(err)
	}
	cat := bridgeCatalog(t)
	projects, items := bridgeFamily(cat, "project"), bridgeFamily(cat, "project-item")
	if len(projects) != 1 || len(items) != 1 {
		t.Fatal("separate roots missing", len(cat.IDs))
	}
	var item ProjectItemState
	if err := DecodeResourceState("project-item", items[0].State, &item); err != nil {
		t.Fatal(err)
	}
	if item.ProjectID != projects[0].ID || item.ProjectRevision != projects[0].Heads[0].ID || item.Values["status"].OptionID != "todo" {
		t.Fatalf("not remapped: %+v", item)
	}
	before := len(items[0].History) + len(projects[0].History)
	if err := runForgeTransfer(bridgeOptions(forge.ResourceProject)); err != nil {
		t.Fatal(err)
	}
	cat = bridgeCatalog(t)
	after := 0
	for _, v := range cat.Resources {
		after += len(v.History)
	}
	if before != after {
		t.Fatalf("repeat wrote %d->%d", before, after)
	}
}
func TestResourceBridgeDiscussionReferencesAndSnapshot(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeDiscussionGraph()}
	bridgeRegister(t, p)
	if err := runForgeTransfer(bridgeOptions(forge.ResourceDiscussion)); err != nil {
		t.Fatal(err)
	}
	cat := bridgeCatalog(t)
	threads, replies := bridgeFamily(cat, "discussion"), bridgeFamily(cat, "discussion-reply")
	if len(threads) != 1 || len(replies) != 2 {
		t.Fatal("missing resources")
	}
	var d DiscussionState
	_ = DecodeResourceState("discussion", threads[0].State, &d)
	if !validEventID(d.AcceptedReplyRevision) {
		t.Fatal("answer not pinned")
	}
	found := false
	for _, r := range replies {
		var s DiscussionReplyState
		_ = DecodeResourceState("discussion-reply", r.State, &s)
		if r.Heads[0].ID == d.AcceptedReplyRevision {
			found = true
		}
		if s.DiscussionID != threads[0].ID {
			t.Fatal("thread ref")
		}
	}
	if !found {
		t.Fatal("answer wrong")
	}
	snap, err := forgeSnapshotResource(threads[0].ID, forge.ResourceDiscussion)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Graph.Nodes) != 3 || snap.NativeID != threads[0].ID || !validEventID(snap.NativeRevision) {
		t.Fatalf("snapshot %+v", snap)
	}
}
func TestResourceBridgePreflightAndDryRunNoWrites(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	bridgeRegister(t, p)
	o := bridgeOptions(forge.ResourceProject)
	o.dryRun = true
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if len(bridgeCatalog(t).IDs) != 0 {
		t.Fatal("dry run wrote facts")
	}
	dir, _ := forgeCheckpointPath("resource-test.json")
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("checkpoint created")
	}
	p.graph.Nodes[1].Item.Values["status"] = forge.Value{Type: "single_select", OptionID: "absent"}
	o.dryRun = false
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("invalid graph accepted")
	}
	if len(bridgeCatalog(t).IDs) != 0 {
		t.Fatal("partial preflight write")
	}
	p.graph = bridgeProjectGraph()
	p.graph.Findings = []forge.Finding{{Status: forge.RequiresMapping, Path: "field/unknown", Message: "map required"}}
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("mapping omitted")
	}
	if len(bridgeCatalog(t).IDs) != 0 {
		t.Fatal("unsupported wrote")
	}
}
func TestResourceBridgeThreeWayAndTamperedBaseline(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	bridgeRegister(t, p)
	o := bridgeOptions(forge.ResourceProject)
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	v := bridgeFamily(bridgeCatalog(t), "project")[0]
	var s ProjectState
	_ = DecodeResourceState("project", v.State, &s)
	s.Title = "local edit"
	raw, _ := ResourceStateJSON("project", s)
	if _, err := MutateResource(ResourceMutation{Family: "project", Intent: "revise", ID: v.ID, Expected: []string{v.Heads[0].ID}, State: raw}); err != nil {
		t.Fatal(err)
	}
	if err := runForgeTransfer(o); err != nil {
		t.Fatal("unchanged remote should preserve local", err)
	}
	v = bridgeFamily(bridgeCatalog(t), "project")[0]
	_ = DecodeResourceState("project", v.State, &s)
	if s.Title != "local edit" {
		t.Fatal("lost local edit")
	}
	p.graph.Nodes[0].Project.Title = "remote edit"
	p.graph.Nodes[0].Record.Revision = "rev-B"
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("competing changes accepted")
	}
	// A native edit cannot manufacture a new import baseline/observation witness.
	s.Metadata["forge/resource-baseline"], _ = forgeResourceDigest("project", v.State)
	raw, _ = ResourceStateJSON("project", s)
	if _, err := MutateResource(ResourceMutation{Family: "project", Intent: "revise", ID: v.ID, Expected: []string{v.Heads[0].ID}, State: raw}); err != nil {
		t.Fatal(err)
	}
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("tampered baseline accepted")
	}
}
func TestResourceBridgeWorkflowAndMissingTarget(t *testing.T) {
	issueCommandTestRepo(t)
	source := "name: inert\non: push\n"
	p := &bridgeProvider{graph: forge.Graph{Nodes: []forge.Node{{Kind: "workflow", Record: bridgeRecord("ci-path"), Workflow: &forge.Workflow{Name: "ci", Title: "CI", Sources: []forge.SourceDocument{{Format: "actions-yaml", Content: source, Digest: eventID([]byte(source))}}}}}}}
	bridgeRegister(t, p)
	if err := runForgeTransfer(bridgeOptions(forge.ResourceWorkflow)); err != nil {
		t.Fatal(err)
	}
	w := bridgeFamily(bridgeCatalog(t), "workflow")[0]
	var state WorkflowState
	_ = DecodeResourceState("workflow", w.State, &state)
	if state.Enabled || state.Pipeline != nil || state.SourceDocuments[0].Content != source {
		t.Fatal("foreign source executed/lost")
	}
	p.graph = bridgeProjectGraph()
	p.graph.Nodes[1].Item.Target = forge.Target{Kind: "issue", Ref: forge.Ref{Kind: "issue", ID: "missing-remote"}}
	if err := runForgeTransfer(bridgeOptions(forge.ResourceProject)); err == nil {
		t.Fatal("missing dependency accepted")
	}
	if len(bridgeFamily(bridgeCatalog(t), "project")) != 0 {
		t.Fatal("partial project write")
	}
}
func TestResourceBridgeRecoveryDoesNotRefreshSource(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeDiscussionGraph()}
	bridgeRegister(t, p)
	o := bridgeOptions(forge.ResourceDiscussion)
	old := forgeMutateResource
	calls := 0
	forgeMutateResource = func(in ResourceMutation) (*ResourceMutationResult, error) {
		out, err := old(in)
		calls++
		if calls == 2 && err == nil {
			return nil, errors.New("lost native response")
		}
		return out, err
	}
	t.Cleanup(func() { forgeMutateResource = old })
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("expected interruption")
	}
	reads := p.reads
	forgeMutateResource = old
	p.graph.Nodes[0].Discussion.Title = "newer remote"
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if p.reads != reads {
		t.Fatal("refreshed source before exact recovery completed")
	}
	cat := bridgeCatalog(t)
	d := bridgeFamily(cat, "discussion")[0]
	var s DiscussionState
	_ = DecodeResourceState("discussion", d.State, &s)
	if s.Title != "Question" || s.AcceptedReplyRevision == "" || len(bridgeFamily(cat, "discussion-reply")) != 2 {
		t.Fatal("did not resume old snapshot")
	}
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	d = bridgeFamily(bridgeCatalog(t), "discussion")[0]
	_ = DecodeResourceState("discussion", d.State, &s)
	if s.Title != "newer remote" {
		t.Fatal("new transfer did not observe source")
	}
}
func TestResourceBridgeRejectsUnknownUnion(t *testing.T) {
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	p.graph.Nodes[0].Discussion = &forge.Discussion{Title: "hidden"}
	if _, err := json.Marshal(p.graph); err != nil {
		t.Fatal(err)
	}
	if err := forgeValidateGraph(p, p.graph, forge.ResourceProject); err == nil {
		t.Fatal("ambiguous union accepted")
	}
	p.graph.Nodes[0].Discussion = nil
	p.graph.Nodes[0].Record.ID = strings.Repeat("x", 1025)
	if err := forgeValidateGraph(p, p.graph, forge.ResourceProject); err == nil {
		t.Fatal("unbounded identity")
	}
}

func TestResourceBridgePinsObservedSchemaAndAnswerAcrossLocalEdits(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	bridgeRegister(t, p)
	o := bridgeOptions(forge.ResourceProject)
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	project := bridgeFamily(bridgeCatalog(t), "project")[0]
	sourceRevision := project.Heads[0].ID
	var state ProjectState
	_ = DecodeResourceState("project", project.State, &state)
	state.Fields = nil
	state.Views = nil
	state.Title = "Local schema"
	raw, _ := ResourceStateJSON("project", state)
	local, err := MutateResource(ResourceMutation{Family: "project", Intent: "revise", ID: project.ID, Expected: []string{sourceRevision}, State: raw})
	if err != nil {
		t.Fatal(err)
	}
	item := p.graph.Nodes[1]
	item.Record = bridgeRecord("new-card")
	p.graph.Nodes = append(p.graph.Nodes, item)
	p.graph.Nodes[0].Record.Revision = "timestamp-only-version" // identical schema observation
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	cat := bridgeCatalog(t)
	project = bridgeFamily(cat, "project")[0]
	if project.Heads[0].ID != local.EventID {
		t.Fatal("unchanged source clobbered native schema")
	}
	for _, v := range bridgeFamily(cat, "project-item") {
		var s ProjectItemState
		_ = DecodeResourceState("project-item", v.State, &s)
		if s.ProjectRevision != sourceRevision {
			t.Fatal("item reinterpreted against current schema")
		}
	}
	p.graph = bridgeDiscussionGraph()
	o = bridgeOptions(forge.ResourceDiscussion)
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	cat = bridgeCatalog(t)
	thread := bridgeFamily(cat, "discussion")[0]
	var d DiscussionState
	_ = DecodeResourceState("discussion", thread.State, &d)
	answerRevision := d.AcceptedReplyRevision
	for _, v := range bridgeFamily(cat, "discussion-reply") {
		if v.Heads[0].ID == answerRevision {
			var reply DiscussionReplyState
			_ = DecodeResourceState("discussion-reply", v.State, &reply)
			reply.Body = "Native answer correction"
			raw, _ := ResourceStateJSON("discussion-reply", reply)
			if _, err := MutateResource(ResourceMutation{Family: "discussion-reply", Intent: "revise", ID: v.ID, Expected: []string{answerRevision}, State: raw}); err != nil {
				t.Fatal(err)
			}
		}
	}
	p.graph.Nodes[0].Discussion.Title = "Remote question edit"
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	thread = bridgeFamily(bridgeCatalog(t), "discussion")[0]
	_ = DecodeResourceState("discussion", thread.State, &d)
	if d.AcceptedReplyRevision != answerRevision {
		t.Fatal("answer moved to locally edited head")
	}
}
func TestResourceBridgeDryRunWithoutPrivateIdentity(t *testing.T) {
	issueCommandTestRepo(t)
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(paths.identities); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths.active); err != nil {
		t.Fatal(err)
	}
	p := &bridgeProvider{graph: bridgeDiscussionGraph()}
	bridgeRegister(t, p)
	o := bridgeOptions(forge.ResourceDiscussion)
	o.dryRun = true
	if err := runForgeTransfer(o); err != nil {
		t.Fatal("preview loaded signing identity", err)
	}
	if len(bridgeCatalog(t).IDs) > 0 {
		t.Fatal("preview signed")
	}
}
func TestResourceBridgeAuthenticatedSnapshotsAndRegistryExport(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeDiscussionGraph()}
	bridgeRegister(t, p)
	if err := runForgeTransfer(bridgeOptions(forge.ResourceDiscussion)); err != nil {
		t.Fatal(err)
	}
	id := bridgeFamily(bridgeCatalog(t), "discussion")[0].ID
	snapshot, err := forgeSnapshotResource(id, forge.ResourceDiscussion)
	if err != nil {
		t.Fatal(err)
	}
	// A later local edit is allowed while recovering an exact older export.
	v := bridgeFamily(bridgeCatalog(t), "discussion")[0]
	var state DiscussionState
	_ = DecodeResourceState("discussion", v.State, &state)
	state.Title = "Later local"
	raw, _ := ResourceStateJSON("discussion", state)
	if _, err := MutateResource(ResourceMutation{Family: "discussion", Intent: "revise", ID: id, Expected: []string{v.Heads[0].ID}, State: raw}); err != nil {
		t.Fatal(err)
	}
	if err := forgeAuthenticateResourceSnapshot(snapshot); err != nil {
		t.Fatal("old snapshot lost authenticity", err)
	}
	altered := snapshot
	altered.Graph.Nodes = append([]forge.Node{}, snapshot.Graph.Nodes[:1]...)
	altered.Digest = ""
	encoded, _ := json.Marshal(altered)
	altered.Digest = eventID(encoded)
	if err := forgeAuthenticateResourceSnapshot(altered); err == nil {
		t.Fatal("unsigned graph trim accepted")
	}
	route := forgeResourceRoute{p.Kind(), forge.ResourceDiscussion}
	calls := 0
	forgeResourceExporters[route] = func(_ context.Context, _ forge.Identity, o transferOptions, s forgeResourceSnapshot) error {
		calls++
		return forgeAuthenticateResourceSnapshot(s)
	}
	t.Cleanup(func() { delete(forgeResourceExporters, route) })
	o := bridgeOptions(forge.ResourceDiscussion)
	o.command = "export"
	o.id = id
	o.dryRun = true
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("resource exporter not routed")
	}
}
func TestResourceBridgeRemoteJournalNonHTTPIdentity(t *testing.T) {
	payload := json.RawMessage(`{"body":"exact"}`)
	intent := forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: eventID([]byte("scope")), Provider: "bundlefake", Endpoint: "bundle:opaque-store", Kind: "project", Action: "create-item", NativeID: eventID([]byte("native")), NativeRevision: eventID([]byte("revision")), Payload: payload, PayloadDigest: eventID(payload), Phase: "prepared"}
	writes := 0
	saves := 0
	_, err := forgeExecuteRemoteIntent(context.Background(), &intent, func() error { saves++; return nil }, func(_ context.Context, i forgeRemoteIntent) (forgeRemoteResult, error) {
		writes++
		if i.Phase != "in_flight" {
			t.Fatal("write not journaled")
		}
		return forgeRemoteObserved("opaque-item", "key-A", "rev-B", json.RawMessage(payload)), nil
	}, func(context.Context, forgeRemoteIntent) (forgeRemoteResult, bool, error) {
		t.Fatal("unexpected reconcile")
		return forgeRemoteResult{}, false, nil
	}, func(_ forgeRemoteIntent, r forgeRemoteResult) error {
		if string(r.Payload) != string(payload) {
			return errors.New("payload changed")
		}
		return nil
	})
	if err != nil || writes != 1 || saves != 3 || intent.Phase != "confirmed" {
		t.Fatalf("journal %d %d %+v %v", writes, saves, intent, err)
	}
}

func TestResourceBridgeCrossResourceMappingAndReports(t *testing.T) {
	issueCommandTestRepo(t)
	native := &IssueState{Title: "Native work", Status: "open"}
	created, err := executeIssueMutation(issueOptions{command: "open", state: native})
	if err != nil {
		t.Fatal(err)
	}
	issueID := created.(*issueMutationResult).IssueID
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	bridgeRegister(t, p)
	ref := forge.Ref{Kind: "issue", ID: "issue-with-opaque-key"}
	p.graph.Nodes[1].Item.Target = forge.Target{Kind: "issue", Ref: ref}
	o := bridgeOptions(forge.ResourceProject)
	err = runForgeTransfer(o)
	var blocked *forgeResourceBlocked
	if !errors.As(err, &blocked) {
		t.Fatal("missing classified plan", err)
	}
	found := false
	for _, f := range blocked.Plan.Findings {
		found = found || f.Status == forge.MissingDependency
	}
	if !found {
		t.Fatal("missing dependency not classified")
	}
	if len(bridgeCatalog(t).IDs) > 0 {
		t.Fatal("partial dependency import")
	}
	o.mappings = map[string]string{forgeReferenceMappingKey(ref): issueID}
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	item := bridgeFamily(bridgeCatalog(t), "project-item")[0]
	var state ProjectItemState
	_ = DecodeResourceState("project-item", item.State, &state)
	if state.Target.ID != issueID || state.Target.Kind != "issue" {
		t.Fatal("cross-resource reference not remapped")
	}
	p.graph = bridgeProjectGraph()
	o.mappings = map[string]string{"typo": "unused"}
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("unknown mapping silently ignored")
	}
}
func TestResourceBridgeRejectsTamperedRecoveryBindings(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	bridgeRegister(t, p)
	o := bridgeOptions(forge.ResourceProject)
	old := forgeMutateResource
	calls := 0
	forgeMutateResource = func(m ResourceMutation) (*ResourceMutationResult, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("interrupted before child")
		}
		return old(m)
	}
	t.Cleanup(func() { forgeMutateResource = old })
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("no interruption")
	}
	forgeMutateResource = old
	name := forgeResourceJournalName(forgeResourceScope(p, o))
	var journal forgeResourceJournal
	if found, err := forgeLoadJSON(name, &journal); err != nil || !found {
		t.Fatal(err)
	}
	for key := range journal.Replacements {
		journal.Replacements[key] = eventID([]byte("invented"))
	}
	if err := forgeStoreJSON(name, journal); err != nil {
		t.Fatal(err)
	}
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("unsigned recovery binding accepted")
	}
	if len(bridgeFamily(bridgeCatalog(t), "project-item")) > 0 {
		t.Fatal("tampering published child")
	}
}

func TestResourceBridgeAllFamiliesSurviveNativeGitTransport(t *testing.T) {
	selection, actor, legacy, privateRoot := setupSingleActorReplication(t)
	receiver, _ := os.Getwd()
	if err := os.Chdir(filepath.Join(privateRoot, "publisher")); err != nil {
		t.Fatal(err)
	}
	writeActiveTestIdentity(t, actor)
	p := &bridgeProvider{graph: bridgeProjectGraph()}
	bridgeRegister(t, p)
	if err := runForgeTransfer(bridgeOptions(forge.ResourceProject)); err != nil {
		t.Fatal(err)
	}
	p.graph = bridgeDiscussionGraph()
	if err := runForgeTransfer(bridgeOptions(forge.ResourceDiscussion)); err != nil {
		t.Fatal(err)
	}
	text := "name: preserved source\n"
	p.graph = forge.Graph{Nodes: []forge.Node{{Kind: "workflow", Record: bridgeRecord("ci"), Workflow: &forge.Workflow{Name: "ci", Title: "CI", Sources: []forge.SourceDocument{{Format: "actions-yaml", Content: text, Digest: eventID([]byte(text))}}}}}}
	if err := runForgeTransfer(bridgeOptions(forge.ResourceWorkflow)); err != nil {
		t.Fatal(err)
	}
	before, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "push", "-q", "origin", actorRef(actor.Actor)+":"+actorRef(actor.Actor))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if _, err := runReplicationTransaction(selection); err != nil {
		t.Fatal(err)
	}
	after, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]StoredEvent{}
	for _, e := range after {
		byID[e.ID] = e
	}
	for _, e := range before {
		got, ok := byID[e.ID]
		if !ok || !bytes.Equal(got.Payload, e.Payload) || !bytes.Equal(got.Signature, e.Signature) {
			t.Fatal("signed transfer bytes changed", e.ID)
		}
	}
	if _, ok := byID[legacy.ID]; !ok {
		t.Fatal("issue lost")
	}
	cat := bridgeCatalog(t)
	for _, family := range []string{"project", "project-item", "discussion", "discussion-reply", "workflow"} {
		if len(bridgeFamily(cat, family)) == 0 {
			t.Fatal("missing native family", family)
		}
	}
	// No private key or adapter checkpoint exists in the receiving repository.
	for _, resource := range []forge.Resource{forge.ResourceProject, forge.ResourceDiscussion, forge.ResourceWorkflow} {
		id := bridgeFamily(cat, string(resource))[0].ID
		snapshot, err := forgeSnapshotResource(id, resource)
		if err != nil {
			t.Fatal(err)
		}
		if err := forgeAuthenticateResourceSnapshot(snapshot); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResourceBridgePreflightsActualSignerEnvelope(t *testing.T) {
	issueCommandTestRepo(t)
	writeActiveTestIdentity(t, testIdentity(t, strings.Repeat("n", 60000)))
	graph := bridgeProjectGraph()
	graph.Nodes[0].Project.Fields = nil
	graph.Nodes[0].Project.Views = nil
	graph.Nodes[1].Item.Values = map[string]forge.Value{}
	for i := 0; i < 49; i++ {
		id := fmt.Sprintf("f%d", i)
		graph.Nodes[0].Project.Fields = append(graph.Nodes[0].Project.Fields, forge.Field{ID: id, Name: id, Type: "text"})
		graph.Nodes[1].Item.Values[id] = forge.Value{Type: "text", Text: strings.Repeat("x", 4096)}
	}
	p := &bridgeProvider{graph: graph}
	bridgeRegister(t, p)
	if err := runForgeTransfer(bridgeOptions(forge.ResourceProject)); err == nil {
		t.Fatal("authenticated payload ceiling ignored")
	}
	if len(bridgeCatalog(t).IDs) != 0 {
		t.Fatal("signed a partial graph before validating later actual signer envelope")
	}
}

func TestResourceBridgePreflightsPendingJournalBudget(t *testing.T) {
	issueCommandTestRepo(t)
	small := forge.Node{Kind: "workflow", Record: bridgeRecord("small"), Workflow: &forge.Workflow{Name: "small", Title: "Small"}}
	p := &bridgeProvider{graph: forge.Graph{Nodes: []forge.Node{small}}}
	bridgeRegister(t, p)
	a, b := strings.Repeat("x", 900000), strings.Repeat("y", 900000)
	for _, id := range []string{"large-a", "large-b", "large-c", "large-d"} {
		p.graph.Nodes = append(p.graph.Nodes, forge.Node{Kind: "workflow", Record: bridgeRecord(id), Workflow: &forge.Workflow{Name: id, Title: id, Sources: []forge.SourceDocument{{Format: "source-a", Content: a, Digest: eventID([]byte(a))}, {Format: "source-b", Content: b, Digest: eventID([]byte(b))}}}})
	}
	if err := runForgeTransfer(bridgeOptions(forge.ResourceWorkflow)); err == nil {
		t.Fatal("whole journal budget ignored")
	}
	if len(bridgeCatalog(t).IDs) > 0 {
		t.Fatal("signed facts before predictable pending checkpoint overflow")
	}
	preview := bridgeOptions(forge.ResourceWorkflow)
	preview.dryRun = true
	if err := runForgeTransfer(preview); err == nil {
		t.Fatal("dry-run missed pending checkpoint budget")
	}
}
func TestResourceBridgeBindsWholeFutureGraphAfterInterruption(t *testing.T) {
	issueCommandTestRepo(t)
	p := &bridgeProvider{graph: bridgeDiscussionGraph()}
	bridgeRegister(t, p)
	o := bridgeOptions(forge.ResourceDiscussion)
	old := forgeMutateResource
	calls := 0
	forgeMutateResource = func(m ResourceMutation) (*ResourceMutationResult, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("interrupted before answer")
		}
		return old(m)
	}
	t.Cleanup(func() { forgeMutateResource = old })
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("expected interruption")
	}
	forgeMutateResource = old
	name := forgeResourceJournalName(forgeResourceScope(p, o))
	var journal forgeResourceJournal
	if ok, err := forgeLoadJSON(name, &journal); err != nil || !ok {
		t.Fatal(err)
	}
	journal.Graph.Nodes[2].Reply.Body = "manufactured future source body"
	if err := forgeStoreJSON(name, journal); err != nil {
		t.Fatal(err)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("changed future snapshot accepted")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("changed future graph signed")
	}
}
