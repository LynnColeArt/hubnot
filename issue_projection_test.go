package main

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func issueFixture(label string, e Event) StoredEvent {
	return StoredEvent{ID: eventID([]byte(label)), Event: e}
}
func issueRoot(label string) StoredEvent {
	return issueFixture(label, Event{Kind: "issue.open", Actor: label, Title: label, Timestamp: "2026-09-09T00:00:00Z"})
}
func issueRevision(label, root string, parents []string, state IssueState) StoredEvent {
	sort.Strings(parents)
	intent := "revise"
	if len(parents) > 1 {
		intent = "resolve"
	}
	return issueFixture(label, Event{Kind: "issue.revise", Actor: label, Subject: root, Parents: parents, Issue: &state, Intent: intent})
}
func TestIssueCatalogPreservesAndResolvesConcurrentHeads(t *testing.T) {
	root := issueRoot("root")
	a := issueRevision("a", root.ID, []string{root.ID}, IssueState{Title: "A", Status: "open"})
	b := issueRevision("b", root.ID, []string{root.ID}, IssueState{Title: "B", Status: "closed"})
	events := []StoredEvent{root, a, b}
	cat, err := BuildIssueCatalog(events)
	if err != nil {
		t.Fatal(err)
	}
	view := cat.Issues[root.ID]
	if !view.Conflict || view.State != nil || len(view.Heads) != 2 {
		t.Fatalf("lost conflict: %#v", view)
	}
	events = []StoredEvent{b, root, a}
	again, err := BuildIssueCatalog(events)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cat, again) {
		t.Fatal("input order changes catalog")
	}
	resolution := issueRevision("resolve", root.ID, []string{a.ID, b.ID}, IssueState{Title: "Resolved", Status: "open"})
	events = append(events, resolution)
	cat, err = BuildIssueCatalog(events)
	if err != nil {
		t.Fatal(err)
	}
	view = cat.Issues[root.ID]
	if view.Conflict || len(view.Heads) != 1 || view.State.Title != "Resolved" || len(view.History) != 4 {
		t.Fatalf("bad resolution: %#v", view)
	}
}
func TestIssueCatalogRejectsBrokenLineage(t *testing.T) {
	root := issueRoot("root")
	other := issueRoot("other")
	wrong := issueRevision("wrong", root.ID, []string{other.ID}, IssueState{Title: "Bad", Status: "open"})
	if _, err := BuildIssueCatalog([]StoredEvent{root, other, wrong}); err == nil {
		t.Fatal("cross-root accepted")
	}
	wrong.Event.Parents = []string{eventID([]byte("missing"))}
	if _, err := BuildIssueCatalog([]StoredEvent{root, wrong}); err == nil {
		t.Fatal("missing parent accepted")
	}
	a := issueRevision("a", root.ID, []string{root.ID}, IssueState{Title: "A", Status: "open"})
	b := issueRevision("b", root.ID, []string{a.ID}, IssueState{Title: "B", Status: "open"})
	a.Event.Parents = []string{b.ID}
	if _, err := BuildIssueCatalog([]StoredEvent{root, a, b}); err == nil {
		t.Fatal("cycle accepted")
	}
}
func TestIssueCatalogGraphCyclesAndCandidate(t *testing.T) {
	a, b := issueRoot("a"), issueRoot("b")
	ar := issueRevision("ar", a.ID, []string{a.ID}, IssueState{Title: "A", Status: "open", Relations: []IssueRelation{{"blocks", b.ID}}})
	br := issueRevision("br", b.ID, []string{b.ID}, IssueState{Title: "B", Status: "open", Relations: []IssueRelation{{"blocks", a.ID}}})
	cat, err := BuildIssueCatalog([]StoredEvent{a, b, ar})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateIssueCandidate(cat, b.ID, []string{b.ID}, *br.Event.Issue); err == nil {
		t.Fatal("local cycle accepted")
	}
	cat, err = BuildIssueCatalog([]StoredEvent{a, b, ar, br})
	if err != nil {
		t.Fatalf("distributed cycle discarded: %v", err)
	}
	if len(cat.Cycles) != 1 || len(cat.Graph) != 2 {
		t.Fatalf("cycle diagnostic missing: %#v", cat)
	}
	unchanged := *br.Event.Issue
	unchanged.Title = "renamed"
	if err := ValidateIssueCandidate(cat, b.ID, []string{br.ID}, unchanged); err != nil {
		t.Fatalf("existing cycle blocks unrelated edit: %v", err)
	}
	sibling := issueRevision("sibling", a.ID, []string{a.ID}, IssueState{Title: "other", Status: "open"})
	cat, err = BuildIssueCatalog([]StoredEvent{a, b, ar, br, sibling})
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range cat.Graph {
		if edge.Source == a.ID && (!edge.Ambiguous || edge.Revision != ar.ID) {
			t.Fatal("missing conflict provenance")
		}
	}
}
func TestIssueCatalogPartialResolutionOverPageLimit(t *testing.T) {
	root := issueRoot("root")
	events := []StoredEvent{root}
	parents := []string{}
	for i := 0; i < 205; i++ {
		r := issueRevision(fmt.Sprint(i), root.ID, []string{root.ID}, IssueState{Title: fmt.Sprint(i), Status: "open"})
		events = append(events, r)
		if i < 200 {
			parents = append(parents, r.ID)
		}
	}
	partial := issueRevision("partial", root.ID, parents, IssueState{Title: "partial", Status: "open"})
	events = append(events, partial)
	cat, err := BuildIssueCatalog(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Issues[root.ID].Heads) != 6 {
		t.Fatalf("unconsumed heads lost: %d", len(cat.Issues[root.ID].Heads))
	}
}
