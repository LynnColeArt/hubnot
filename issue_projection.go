package main

import (
	"container/heap"
	"fmt"
	"sort"
)

type IssueHead struct {
	ID    string     `json:"id"`
	Actor string     `json:"actor"`
	State IssueState `json:"state"`
}
type IssueView struct {
	ID       string        `json:"id"`
	Creator  string        `json:"creator"`
	Heads    []IssueHead   `json:"heads"`
	Conflict bool          `json:"conflict"`
	State    *IssueState   `json:"state"`
	History  []StoredEvent `json:"-"`
	Comments []StoredEvent `json:"-"`
}
type IssueEdge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Kind      string `json:"kind"`
	Revision  string `json:"revision"`
	Actor     string `json:"actor"`
	Ambiguous bool   `json:"ambiguous"`
}
type IssueCycle struct {
	Kind   string   `json:"kind"`
	Issues []string `json:"issues"`
}
type IssueCatalog struct {
	Issues map[string]*IssueView
	IDs    []string
	Graph  []IssueEdge
	Cycles []IssueCycle
}

func issueRootID(stored StoredEvent) string {
	if stored.Event.Kind == "issue.open" {
		return stored.ID
	}
	if stored.Event.Kind == "issue.revise" || stored.Event.Kind == "issue.comment" {
		return stored.Event.Subject
	}
	return ""
}

// issueReferences is the single source for issue replication dependency closure.
func issueReferences(e Event) []string {
	refs := []string{}
	if e.Kind == "issue.revise" || e.Kind == "issue.comment" {
		refs = append(refs, e.Subject)
	}
	if e.Kind == "issue.revise" {
		refs = append(refs, e.Parents...)
	}
	if isIssueKind(e.Kind) && e.Issue != nil {
		for _, r := range e.Issue.Relations {
			refs = append(refs, r.Target)
		}
	}
	return sortedUniqueStrings(refs...)
}
func validateIssueReferences(stored StoredEvent, byID map[string]StoredEvent) error {
	e := stored.Event
	if !isIssueKind(e.Kind) {
		return nil
	}
	if e.Kind != "issue.open" {
		root, exists := byID[e.Subject]
		if !exists || root.Event.Kind != "issue.open" {
			return fmt.Errorf("issue fact %s requires opening %s", stored.ID, e.Subject)
		}
	}
	for _, parent := range e.Parents {
		value, exists := byID[parent]
		if !exists || (value.Event.Kind != "issue.open" && value.Event.Kind != "issue.revise") || issueRootID(value) != e.Subject {
			return fmt.Errorf("issue revision %s has unavailable or wrong-lineage parent %s", stored.ID, parent)
		}
	}
	if e.Issue != nil {
		for _, r := range e.Issue.Relations {
			target, exists := byID[r.Target]
			if !exists || target.Event.Kind != "issue.open" {
				return fmt.Errorf("issue %s has unavailable relation target %s", stored.ID, r.Target)
			}
			if r.Target == issueRootID(stored) {
				return fmt.Errorf("issue %s has a self relation", stored.ID)
			}
		}
	}
	return nil
}

// validateIssueRelationships also computes topological issue history. It is pure
// and does not recurse into collectEvents or generic relationship validation.
func validateIssueRelationships(events []StoredEvent) ([]StoredEvent, error) {
	byID := make(map[string]StoredEvent, len(events))
	for _, s := range events {
		byID[s.ID] = s
	}
	indegree := map[string]int{}
	children := map[string][]string{}
	for _, s := range events {
		if !isIssueKind(s.Event.Kind) {
			continue
		}
		if err := validateIssueEvent(s.Event); err != nil {
			return nil, fmt.Errorf("issue fact %s: %w", s.ID, err)
		}
		if err := validateIssueReferences(s, byID); err != nil {
			return nil, err
		}
		if s.Event.Kind == "issue.comment" {
			continue
		}
		if _, exists := indegree[s.ID]; exists {
			continue
		}
		indegree[s.ID] = len(s.Event.Parents)
		for _, p := range s.Event.Parents {
			children[p] = append(children[p], s.ID)
		}
	}
	ready := &issueIDHeap{}
	heap.Init(ready)
	for id, n := range indegree {
		if n == 0 {
			heap.Push(ready, id)
		}
	}
	ordered := make([]StoredEvent, 0, len(indegree))
	for ready.Len() > 0 {
		id := heap.Pop(ready).(string)
		ordered = append(ordered, byID[id])
		for _, child := range children[id] {
			indegree[child]--
			if indegree[child] == 0 {
				heap.Push(ready, child)
			}
		}
	}
	if len(ordered) != len(indegree) {
		return nil, fmt.Errorf("issue revision lineage contains a cycle")
	}
	return ordered, nil
}

type issueIDHeap []string

func (h issueIDHeap) Len() int           { return len(h) }
func (h issueIDHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h issueIDHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *issueIDHeap) Push(v any)        { *h = append(*h, v.(string)) }
func (h *issueIDHeap) Pop() any          { old := *h; n := len(old); v := old[n-1]; *h = old[:n-1]; return v }

// BuildIssueCatalog consumes already verified signed events. No cache or time
// ordering supplies authority; heads are maximal vertices of revision ancestry.
func BuildIssueCatalog(events []StoredEvent) (*IssueCatalog, error) {
	ordered, err := validateIssueRelationships(events)
	if err != nil {
		return nil, err
	}
	cat := &IssueCatalog{Issues: map[string]*IssueView{}, IDs: []string{}, Graph: []IssueEdge{}, Cycles: []IssueCycle{}}
	consumed := map[string]bool{}
	for _, stored := range ordered {
		for _, p := range stored.Event.Parents {
			consumed[p] = true
		}
	}
	for _, stored := range ordered {
		e := stored.Event
		root := issueRootID(stored)
		if e.Kind == "issue.open" {
			cat.Issues[root] = &IssueView{ID: root, Creator: e.Actor, Heads: []IssueHead{}, History: []StoredEvent{}, Comments: []StoredEvent{}}
			cat.IDs = append(cat.IDs, root)
		}
		view := cat.Issues[root]
		view.History = append(view.History, stored)
		if !consumed[stored.ID] {
			state := IssueState{Title: e.Title, Body: e.Body, Status: "open", Criteria: []IssueCriterion{}, Labels: []string{}, Assignees: []string{}, Relations: []IssueRelation{}, Metadata: map[string]string{}}
			if e.Issue != nil {
				state, err = CanonicalIssueState(*e.Issue)
				if err != nil {
					return nil, err
				}
			}
			view.Heads = append(view.Heads, IssueHead{stored.ID, e.Actor, state})
		}
	}
	seenComments := map[string]bool{}
	for _, stored := range events {
		if stored.Event.Kind == "issue.comment" && !seenComments[stored.ID] {
			cat.Issues[stored.Event.Subject].Comments = append(cat.Issues[stored.Event.Subject].Comments, stored)
			seenComments[stored.ID] = true
		}
	}
	sort.Strings(cat.IDs)
	for _, id := range cat.IDs {
		view := cat.Issues[id]
		sort.Slice(view.Heads, func(i, j int) bool { return view.Heads[i].ID < view.Heads[j].ID })
		view.Conflict = len(view.Heads) > 1
		if len(view.Heads) == 1 {
			state := view.Heads[0].State
			view.State = &state
		}
		sort.Slice(view.Comments, func(i, j int) bool {
			a, b := view.Comments[i], view.Comments[j]
			if a.Event.Timestamp != b.Event.Timestamp {
				return a.Event.Timestamp < b.Event.Timestamp
			}
			return a.ID < b.ID
		})
		for _, head := range view.Heads {
			for _, r := range head.State.Relations {
				cat.Graph = append(cat.Graph, IssueEdge{id, r.Target, r.Kind, head.ID, head.Actor, view.Conflict})
			}
		}
	}
	sortIssueEdges(cat.Graph)
	cat.Cycles = issueGraphCycles(cat.Graph)
	return cat, nil
}
func sortIssueEdges(edges []IssueEdge) {
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Revision < b.Revision
	})
}

// GraphFor includes incoming and outgoing links, retaining signed source and
// provenance even for related links which callers may navigate bidirectionally.
func (cat *IssueCatalog) GraphFor(id string) []IssueEdge {
	out := []IssueEdge{}
	for _, edge := range cat.Graph {
		if edge.Source == id || edge.Target == id {
			out = append(out, edge)
		}
	}
	return out
}

// ValidateIssueCandidate checks only new relationships for cycles. Existing
// distributed cycles do not prevent unrelated edits. Head/snapshot guards belong
// to the command layer; selected parents are the heads this candidate consumes.
func ValidateIssueCandidate(cat *IssueCatalog, id string, parents []string, state IssueState) error {
	canonical, err := CanonicalIssueState(state)
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, p := range parents {
		selected[p] = true
	}
	edges := make([]IssueEdge, 0, len(cat.Graph)+len(canonical.Relations))
	old := map[IssueRelation]bool{}
	for _, edge := range cat.Graph {
		if edge.Source == id {
			old[IssueRelation{edge.Kind, edge.Target}] = true
		}
		if edge.Source != id || !selected[edge.Revision] {
			edges = append(edges, edge)
		}
	}
	for _, r := range canonical.Relations {
		if r.Target == id {
			return fmt.Errorf("issue cannot relate to itself")
		}
		if _, ok := cat.Issues[r.Target]; !ok {
			return fmt.Errorf("issue relation target %s is unavailable", r.Target)
		}
		edges = append(edges, IssueEdge{Source: id, Target: r.Target, Kind: r.Kind})
	}
	components := map[string]map[string]int{}
	for index, cycle := range issueGraphCycles(edges) {
		if components[cycle.Kind] == nil {
			components[cycle.Kind] = map[string]int{}
		}
		for _, member := range cycle.Issues {
			components[cycle.Kind][member] = index + 1
		}
	}
	for _, r := range canonical.Relations {
		if r.Kind == "related" || old[r] {
			continue
		}
		component := components[r.Kind][id]
		if component != 0 && component == components[r.Kind][r.Target] {
			return fmt.Errorf("issue relation would introduce a %s cycle", r.Kind)
		}
	}
	return nil
}

// Iterative Kosaraju traversal keeps cycle diagnosis bounded by graph size;
// descendants of cycles are not misreported as cycle members.
func issueGraphCycles(edges []IssueEdge) []IssueCycle {
	result := []IssueCycle{}
	for _, kind := range []string{"blocks", "parent"} {
		next, prev := map[string][]string{}, map[string][]string{}
		nodes := map[string]bool{}
		for _, e := range edges {
			if e.Kind == kind {
				next[e.Source] = append(next[e.Source], e.Target)
				prev[e.Target] = append(prev[e.Target], e.Source)
				nodes[e.Source] = true
				nodes[e.Target] = true
			}
		}
		ids := make([]string, 0, len(nodes))
		for id := range nodes {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		visited := map[string]bool{}
		order := []string{}
		type frame struct {
			id     string
			finish bool
		}
		for _, id := range ids {
			if visited[id] {
				continue
			}
			stack := []frame{{id, false}}
			for len(stack) > 0 {
				f := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if f.finish {
					order = append(order, f.id)
					continue
				}
				if visited[f.id] {
					continue
				}
				visited[f.id] = true
				stack = append(stack, frame{f.id, true})
				for _, child := range next[f.id] {
					if !visited[child] {
						stack = append(stack, frame{child, false})
					}
				}
			}
		}
		assigned := map[string]bool{}
		for i := len(order) - 1; i >= 0; i-- {
			id := order[i]
			if assigned[id] {
				continue
			}
			members := []string{}
			stack := []string{id}
			for len(stack) > 0 {
				n := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if assigned[n] {
					continue
				}
				assigned[n] = true
				members = append(members, n)
				stack = append(stack, prev[n]...)
			}
			if len(members) > 1 {
				sort.Strings(members)
				result = append(result, IssueCycle{kind, members})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Issues[0] < result[j].Issues[0]
	})
	return result
}
