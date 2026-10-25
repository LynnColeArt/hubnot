package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResourceExactDependencies(t *testing.T) {
	project := resourceFixture(t, "project", "project", "", nil, projectState())
	schema := resourceFixture(t, "schema", "project", project.ID, []string{project.ID}, projectState())
	item := resourceFixture(t, "item", "project-item", "", nil, ProjectItemState{Schema: "hn.project-item/1", ProjectID: project.ID, ProjectRevision: schema.ID, Target: ProjectTarget{Kind: "draft", Title: "plan"}})
	for _, missing := range []StoredEvent{project, schema} {
		events := []StoredEvent{item}
		byID := map[string]StoredEvent{item.ID: item}
		for _, s := range []StoredEvent{project, schema} {
			if s.ID != missing.ID {
				events = append(events, s)
				byID[s.ID] = s
			}
		}
		if err := validateExactEventReferenceClosure(events); err == nil {
			t.Fatal("missing closure")
		}
		dep, err := replicationEventDependency("", "", item, byID, nil, nil, nil)
		if err != nil || dep == nil || dep.Missing != missing.ID {
			t.Fatalf("dependency: %#v %v", dep, err)
		}
	}
}
func TestResourceReplicationQuarantineRecoveryAndSignedBytes(t *testing.T) {
	selection, creator, legacy, privateRoot := setupSingleActorReplication(t)
	receiver, _ := os.Getwd()
	if err := os.Chdir(filepath.Join(privateRoot, "publisher")); err != nil {
		t.Fatal(err)
	}
	writeActiveTestIdentity(t, creator)
	project, err := MutateResource(ResourceMutation{Family: "project", Intent: "open", State: resourceState(t, "project", projectState()), Operation: "project"})
	if err != nil {
		t.Fatal(err)
	}
	author := testIdentity(t, "Item author")
	writeActiveTestIdentity(t, author)
	item, err := MutateResource(ResourceMutation{Family: "project-item", Intent: "open", State: resourceState(t, "project-item", ProjectItemState{Schema: "hn.project-item/1", ProjectID: project.ResourceID, ProjectRevision: project.EventID, Target: ProjectTarget{Kind: "issue", ID: legacy.ID}, Values: map[string]ProjectValue{"score": {Type: "number", Number: "2.5"}}}), Operation: "item"})
	if err != nil {
		t.Fatal(err)
	}
	thread, err := MutateResource(ResourceMutation{Family: "discussion", Intent: "open", State: resourceState(t, "discussion", DiscussionState{Schema: "hn.discussion/1", Title: "Question", Category: "q", Status: "open"})})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := MutateResource(ResourceMutation{Family: "discussion-reply", Intent: "open", State: resourceState(t, "discussion-reply", DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: thread.ResourceID, Body: "answer"})})
	if err != nil {
		t.Fatal(err)
	}
	_, err = MutateResource(ResourceMutation{Family: "discussion", Intent: "revise", ID: thread.ResourceID, Expected: []string{thread.EventID}, State: resourceState(t, "discussion", DiscussionState{Schema: "hn.discussion/1", Title: "Question", Category: "q", Status: "open", AcceptedReplyRevision: reply.EventID})})
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := MutateResource(ResourceMutation{Family: "workflow", Intent: "open", State: resourceState(t, "workflow", WorkflowState{Schema: "hn.workflow/1", Name: "test", Title: "Test", Enabled: true, Pipeline: &PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "test", Command: "true"}}}})})
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "push", "-q", "origin", actorRef(creator.Actor)+":"+actorRef(creator.Actor), actorRef(author.Actor)+":"+actorRef(author.Actor))
	if err = os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	selection.Actors = []string{author.Actor}
	result, err := runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	if !replicationOutcomeHasStatus(result.Outcomes, replicationActor, author.Actor, replicationDependencyMissing) {
		t.Fatal("missing schema not quarantined", result.Outcomes)
	}
	assertRefAbsent(t, acceptedActorRef("origin", author.Actor))
	selection.Actors = append(selection.Actors, creator.Actor)
	result, err = runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range selection.Actors {
		if !replicationOutcomeHasStatus(result.Outcomes, replicationActor, actor, replicationPromoted) {
			t.Fatal("not promoted", result.Outcomes)
		}
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildResourceCatalog(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{project.ResourceID, item.ResourceID, thread.ResourceID, reply.ResourceID, workflow.ResourceID} {
		if cat.Resources[id] == nil || cat.Resources[id].Conflict {
			t.Fatal("fresh clone reconstruction", id)
		}
	}
	stored, err := loadStoredEvent(legacy.Commit)
	if err != nil || !bytes.Equal(stored.Payload, legacy.Payload) || !bytes.Equal(stored.Signature, legacy.Signature) {
		t.Fatal("old signed bytes changed", err)
	}
}
