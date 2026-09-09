package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIssueExactReferenceClosure(t *testing.T) {
	root, target := issueRoot("root"), issueRoot("target")
	a := issueRevision("a", root.ID, []string{root.ID}, IssueState{Title: "A", Status: "open"})
	b := issueRevision("b", root.ID, []string{root.ID}, IssueState{Title: "B", Status: "open"})
	r := issueRevision("resolve", root.ID, []string{a.ID, b.ID}, IssueState{Title: "Resolve", Status: "open", Relations: []IssueRelation{{"blocks", target.ID}}})
	all := []StoredEvent{root, target, a, b, r}
	if err := validateEventRelationships(all); err != nil {
		t.Fatal(err)
	}
	for _, missing := range []StoredEvent{root, target, a, b} {
		events := []StoredEvent{}
		byID := map[string]StoredEvent{}
		for _, e := range all {
			if e.ID != missing.ID {
				events = append(events, e)
				byID[e.ID] = e
			}
		}
		if err := validateExactEventReferenceClosure(events); err == nil {
			t.Fatalf("closure missed %s", missing.ID)
		}
		dep, err := replicationEventDependency("", "", r, byID, nil, nil, nil)
		if err != nil || dep == nil || dep.Missing != missing.ID {
			t.Fatalf("dependency %s: %#v %v", missing.ID, dep, err)
		}
	}
	bad := r
	bad.Event.Parents = []string{target.ID}
	if err := validateEventRelationships([]StoredEvent{root, target, bad}); err == nil {
		t.Fatal("generic loader accepted cross-root parent")
	}
}

func TestIssueReplicationQuarantineAndRecovery(t *testing.T) {
	selection, creator, root, privateRoot := setupSingleActorReplication(t)
	receiver, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	publisher := filepath.Join(privateRoot, "publisher")
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	actor := testIdentity(t, "Editor")
	r := newEvent(actor, "issue.revise", 1, "")
	r.Subject = root.ID
	r.Parents = []string{root.ID}
	r.Issue = &IssueState{Title: "Edited", Status: "open"}
	r.Intent = "revise"
	revision, err := appendEvent(r, actor)
	if err != nil {
		t.Fatal(err)
	}
	dependent := testIdentity(t, "Dependent")
	d := newEvent(dependent, "issue.revise", 1, "")
	d.Subject = root.ID
	d.Parents = []string{revision.ID}
	d.Issue = &IssueState{Title: "Dependent", Status: "open"}
	d.Intent = "revise"
	dependentRevision, err := appendEvent(d, dependent)
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "push", "-q", "origin", actorRef(actor.Actor)+":"+actorRef(actor.Actor), actorRef(dependent.Actor)+":"+actorRef(dependent.Actor))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	selection.Actors = []string{actor.Actor, dependent.Actor}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	result, err := runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range selection.Actors {
		if !replicationOutcomeHasStatus(result.Outcomes, replicationActor, id, replicationDependencyMissing) {
			t.Fatalf("actor %s was not dependency-missing", id)
		}
		assertRefAbsent(t, acceptedActorRef("origin", id))
	}
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); after != before {
		t.Fatal("quarantine moved refs")
	}
	selection.Actors = append(selection.Actors, creator.Actor)
	result, err = runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range selection.Actors {
		if !replicationOutcomeHasStatus(result.Outcomes, replicationActor, id, replicationPromoted) {
			t.Fatalf("actor %s was not promoted on recovery", id)
		}
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildIssueCatalog(events)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Issues[root.ID].Heads[0].ID != dependentRevision.ID {
		t.Fatal("recovery changed revision")
	}
	got, err := loadStoredEvent(root.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Payload, root.Payload) || !reflect.DeepEqual(got.Signature, root.Signature) {
		t.Fatal("legacy signed bytes changed")
	}
}

func TestIssueRichOpeningRelationDependency(t *testing.T) {
	root := issueRoot("rich")
	missing := eventID([]byte("missing"))
	root.Event.Issue = &IssueState{Title: "rich", Status: "open", Relations: []IssueRelation{{"related", missing}}}
	root.Event.Intent = "open"
	if err := validateExactEventReferenceClosure([]StoredEvent{root}); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("opening target closure: %v", err)
	}
	dep, err := replicationEventDependency("", "", root, map[string]StoredEvent{root.ID: root}, nil, nil, nil)
	if err != nil || dep == nil || dep.Missing != missing {
		t.Fatalf("opening dependency: %#v %v", dep, err)
	}
}
