package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"hubnot/internal/forge"
)

const forgeResourceGraphLimit = 8 << 20
const forgeResourceNodeLimit = 256
const forgeResourceSourceKey = "forge/resource-source"
const forgeResourceObservationKey = "forge/resource-observation"
const forgeResourceBaselineKey = "forge/resource-baseline"

type forgeResourceRoute struct {
	Provider string
	Resource forge.Resource
}

// Resource adapters install only the exports they implement. Each callback
// receives an authenticated exact native snapshot and owns its typed step plan;
// every remote mutation uses forgeExecuteRemoteIntent, under the caller's lock.
type forgeResourceExporter func(context.Context, forge.Identity, transferOptions, forgeResourceSnapshot) error

var forgeResourceExporters = map[forgeResourceRoute]forgeResourceExporter{}
var forgeMutateResource = MutateResource

func forgeValidLocator(s string) bool { return issueIdentifier(s, 2048) }
func forgeResourceInstalled(o transferOptions) bool {
	if o.command == "import" {
		return o.resource == forge.ResourceProject || o.resource == forge.ResourceDiscussion || o.resource == forge.ResourceWorkflow
	}
	return forgeResourceExporters[forgeResourceRoute{o.provider, o.resource}] != nil
}
func forgeTransferResource(ctx context.Context, p forge.Identity, o transferOptions) error {
	if o.command == "export" {
		snapshot, err := forgeSnapshotResource(o.id, o.resource)
		if err != nil {
			return err
		}
		handler := forgeResourceExporters[forgeResourceRoute{o.provider, o.resource}]
		if handler == nil {
			return errors.New("resource exporter is unavailable")
		}
		if err := forgeAuthenticateResourceSnapshot(snapshot); err != nil {
			return err
		}
		return handler(ctx, p, o, snapshot)
	}
	return forgeImportResources(ctx, p, o)
}
func forgeReadResourceGraph(ctx context.Context, p forge.Identity, o transferOptions) (forge.Graph, error) {
	selection := forge.Selection{Project: o.project, Key: o.key, Mappings: o.mappings}
	switch o.resource {
	case forge.ResourceProject:
		if reader, ok := p.(forge.ProjectReader); ok {
			return reader.ReadProjects(ctx, selection)
		}
	case forge.ResourceDiscussion:
		if reader, ok := p.(forge.DiscussionReader); ok {
			return reader.ReadDiscussions(ctx, selection)
		}
	case forge.ResourceWorkflow:
		if reader, ok := p.(forge.WorkflowReader); ok {
			return reader.ReadWorkflows(ctx, selection)
		}
	}
	return forge.Graph{}, errors.New("provider does not implement the selected typed resource reader")
}

type forgeResourceSource struct {
	Schema   string       `json:"schema"`
	Provider string       `json:"provider"`
	Endpoint string       `json:"endpoint"`
	Kind     string       `json:"kind"`
	Project  string       `json:"project"`
	Record   forge.Record `json:"record"`
}
type forgeResourceObservation struct {
	Source       forgeResourceSource `json:"source"`
	RemoteDigest string              `json:"remote_digest"`
}

func forgeResourceIdentity(s forgeResourceSource) string {
	return s.Provider + "\n" + s.Endpoint + "\n" + s.Kind + "\n" + s.Record.ID
}
func forgeNodeKey(kind, id string) string { return kind + "\n" + id }
func forgeResourceOrigin(p forge.Identity, o transferOptions, n forge.Node) forgeResourceSource {
	return forgeResourceSource{"hn.forge-resource-source/1", p.Kind(), p.Endpoint(), n.Kind, o.project, n.Record}
}
func forgeValidateResourceSource(s forgeResourceSource) error {
	if s.Schema != "hn.forge-resource-source/1" || !issueIdentifier(s.Provider, 64) || !forgeValidLocator(s.Endpoint) || !validResourceFamily(s.Kind) || !issueIdentifier(s.Project, 512) || !issueIdentifier(s.Record.ID, 1024) || !issueIdentifier(s.Record.Key, 1024) || s.Record.Revision != "" && !issueIdentifier(s.Record.Revision, 1024) || s.Record.URL != "" && !forgeValidLocator(s.Record.URL) || s.Record.Author != "" && !issueIdentifier(s.Record.Author, 256) || !forgeValidTime(s.Record.CreatedAt) || !forgeValidTime(s.Record.UpdatedAt) {
		return errors.New("invalid typed resource source")
	}
	_, err := forgeMetadata(s)
	return err
}
func forgeParseResourceSource(raw string) (forgeResourceSource, error) {
	var s forgeResourceSource
	if len(raw) > 4096 || forgeStrictJSON([]byte(raw), &s) != nil {
		return s, errors.New("invalid typed source metadata")
	}
	return s, forgeValidateResourceSource(s)
}
func forgeValidateGraph(p forge.Identity, g forge.Graph, selected forge.Resource) error {
	raw, err := json.Marshal(g)
	if err != nil || len(raw) > forgeResourceGraphLimit || len(g.Nodes) > forgeResourceNodeLimit || len(g.Findings) > 1024 || len(g.UsedMappings) > 64 {
		return errors.New("resource graph exceeds bounded transfer limits")
	}
	if !forgeValidLocator(p.Endpoint()) || !issueIdentifier(p.Kind(), 64) {
		return errors.New("invalid provider identity")
	}
	roots := 0
	seen := map[string]bool{}
	for _, n := range g.Nodes {
		key := forgeNodeKey(n.Kind, n.Record.ID)
		if seen[key] {
			return errors.New("duplicate resource identity in graph")
		}
		seen[key] = true
		count := 0
		for _, present := range []bool{n.Project != nil, n.Item != nil, n.Discussion != nil, n.Reply != nil, n.Workflow != nil} {
			if present {
				count++
			}
		}
		if count != 1 || n.Kind == "project" != (n.Project != nil) || n.Kind == "project-item" != (n.Item != nil) || n.Kind == "discussion" != (n.Discussion != nil) || n.Kind == "discussion-reply" != (n.Reply != nil) || n.Kind == "workflow" != (n.Workflow != nil) {
			return errors.New("resource node requires exactly its declared typed value")
		}
		allowed := forge.Resource(n.Kind) == selected || selected == forge.ResourceProject && n.Kind == ResourceProjectItem || selected == forge.ResourceDiscussion && n.Kind == ResourceDiscussionReply
		if !allowed {
			return errors.New("source graph contains an unselected resource family")
		}
		if err := forgeValidateNodeRefs(n); err != nil {
			return err
		}
		if forge.Resource(n.Kind) == selected {
			roots++
		}
		if err := forgeValidateResourceSource(forgeResourceOrigin(p, transferOptions{project: "preview"}, n)); err != nil {
			return err
		}
		if err := resourceMetadata(n.Metadata); err != nil {
			return err
		}
		for key := range n.Metadata {
			if strings.HasPrefix(key, "forge/") {
				return errors.New("source graph cannot supply native forge provenance metadata")
			}
		}
	}
	if len(g.Nodes) > 0 && roots == 0 {
		return errors.New("graph does not contain the selected resource family")
	}
	for _, f := range g.Findings {
		if !issueIdentifier(f.Path, 1024) || !issueText(f.Message, 4096, false) {
			return errors.New("invalid resource mapping report")
		}
		switch f.Status {
		case forge.Mapped, forge.Preserved, forge.RequiresMapping, forge.Unsupported, forge.MissingDependency, forge.IdentityConflict:
		default:
			return errors.New("unknown resource mapping classification")
		}
	}
	return nil
}
func forgeResourcePlanReport(g forge.Graph, o transferOptions) forge.Plan {
	return forge.Plan{Resource: o.resource, Direction: o.command, Nodes: len(g.Nodes), Findings: append([]forge.Finding{{Status: forge.Mapped, Path: "typed-native-state", Message: "independent signed resource roots and exact reference mappings"}, {Status: forge.Preserved, Path: "native-metadata", Message: "native metadata is preserved; source attribution grants no authority"}}, g.Findings...)}
}

func forgeResourceStateParts(family string, raw json.RawMessage) (any, map[string]string, error) {
	canonical, err := CanonicalResourceState(family, raw)
	if err != nil {
		return nil, nil, err
	}
	var value any
	var metadata map[string]string
	switch family {
	case ResourceProject:
		s := &ProjectState{}
		_ = json.Unmarshal(canonical, s)
		value, metadata = s, s.Metadata
	case ResourceProjectItem:
		s := &ProjectItemState{}
		_ = json.Unmarshal(canonical, s)
		value, metadata = s, s.Metadata
	case ResourceDiscussion:
		s := &DiscussionState{}
		_ = json.Unmarshal(canonical, s)
		value, metadata = s, s.Metadata
	case ResourceDiscussionReply:
		s := &DiscussionReplyState{}
		_ = json.Unmarshal(canonical, s)
		value, metadata = s, s.Metadata
	case ResourceWorkflow:
		s := &WorkflowState{}
		_ = json.Unmarshal(canonical, s)
		value, metadata = s, s.Metadata
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	return value, metadata, nil
}
func forgeSetResourceMetadata(value any, m map[string]string) {
	switch s := value.(type) {
	case *ProjectState:
		s.Metadata = m
	case *ProjectItemState:
		s.Metadata = m
	case *DiscussionState:
		s.Metadata = m
	case *DiscussionReplyState:
		s.Metadata = m
	case *WorkflowState:
		s.Metadata = m
	}
}

// Native metadata remains local descriptive data; managed content is typed state.
func forgeResourceDigest(family string, raw json.RawMessage) (string, error) {
	v, _, err := forgeResourceStateParts(family, raw)
	if err != nil {
		return "", err
	}
	forgeSetResourceMetadata(v, nil)
	b, err := json.Marshal(v)
	return eventID(b), err
}
func forgeResourceRemoteDigest(n forge.Node) string {
	n.Record = forge.Record{}
	n.Metadata = nil
	raw, _ := json.Marshal(n)
	return eventID(raw)
}
func forgeResourceOperation(m ResourceMutation) string {
	return forgeOperation("resource-import/1", m.Family, m.Intent, m.ID, strings.Join(m.Expected, "\n"), eventID(m.State))
}
func forgeResourceWitness(view *ResourceView, opening string) (*StoredEvent, error) {
	if view.Conflict || len(view.Heads) != 1 {
		return nil, errors.New("mapped resource has concurrent heads")
	}
	byID := map[string]StoredEvent{}
	for _, e := range view.History {
		byID[e.ID] = e
	}
	frontier := []string{view.Heads[0].ID}
	seen := map[string]bool{}
	witnesses := map[string]StoredEvent{}
	for len(frontier) > 0 {
		id := frontier[0]
		frontier = frontier[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		e, ok := byID[id]
		if !ok {
			return nil, errors.New("resource witness ancestry missing")
		}
		m := ResourceMutation{Family: view.Family, Intent: e.Event.Intent, ID: e.Event.Subject, Expected: e.Event.Parents, State: json.RawMessage(e.Event.Body)}
		if e.Event.Operation == forgeResourceOperation(m) {
			_, metadata, err := forgeResourceStateParts(view.Family, m.State)
			if err != nil {
				return nil, err
			}
			var obs forgeResourceObservation
			digest, _ := forgeResourceDigest(view.Family, m.State)
			if metadata[forgeResourceSourceKey] != opening || forgeStrictJSON([]byte(metadata[forgeResourceObservationKey]), &obs) != nil || forgeValidateResourceSource(obs.Source) != nil || !validEventID(obs.RemoteDigest) || metadata[forgeResourceBaselineKey] != digest {
				return nil, errors.New("invalid signed import baseline witness")
			}
			source, _ := forgeParseResourceSource(opening)
			if forgeResourceIdentity(source) != forgeResourceIdentity(obs.Source) {
				return nil, errors.New("baseline source identity mismatch")
			}
			witnesses[id] = e
			continue
		}
		frontier = append(frontier, e.Event.Parents...)
	}
	if len(witnesses) != 1 {
		return nil, errors.New("resource baseline has no unique signed import witness")
	}
	var witness StoredEvent
	for _, e := range witnesses {
		witness = e
	}
	_, current, err := forgeResourceStateParts(view.Family, view.State)
	if err != nil {
		return nil, err
	}
	_, prior, _ := forgeResourceStateParts(view.Family, json.RawMessage(witness.Event.Body))
	for _, key := range []string{forgeResourceSourceKey, forgeResourceObservationKey, forgeResourceBaselineKey} {
		if current[key] != prior[key] {
			return nil, errors.New("native edit changed signed resource baseline metadata")
		}
	}
	return &witness, nil
}

type forgeResourceMapping struct {
	View    *ResourceView
	Source  forgeResourceSource
	Opening string
	Witness StoredEvent
}

func forgeResourceMappings(cat *ResourceCatalog) (map[string]forgeResourceMapping, error) {
	result := map[string]forgeResourceMapping{}
	for _, id := range cat.IDs {
		v := cat.Resources[id]
		var root *StoredEvent
		for i := range v.History {
			if v.History[i].ID == v.ID {
				root = &v.History[i]
				break
			}
		}
		if root == nil {
			return nil, errors.New("resource opening unavailable")
		}
		_, metadata, err := forgeResourceStateParts(v.Family, json.RawMessage(root.Event.Body))
		if err != nil {
			return nil, err
		}
		raw := metadata[forgeResourceSourceKey]
		if raw == "" {
			continue
		}
		source, err := forgeParseResourceSource(raw)
		if err != nil || source.Kind != v.Family {
			return nil, errors.New("invalid immutable resource source mapping")
		}
		key := forgeResourceIdentity(source)
		if _, exists := result[key]; exists {
			return nil, errors.New("duplicate immutable resource source mapping")
		}
		witness, err := forgeResourceWitness(v, raw)
		if err != nil {
			return nil, err
		}
		result[key] = forgeResourceMapping{v, source, raw, *witness}
	}
	return result, nil
}

type forgeResourceBinding struct {
	ID             string `json:"id"`
	Revision       string `json:"revision"`
	SourceRevision string `json:"source_revision,omitempty"`
}

func forgeReferenceMappingKey(ref forge.Ref) string {
	return "ref." + ref.Kind + "." + strings.TrimPrefix(eventID([]byte(ref.ID)), "sha256:")
}
func forgeLookupResourceRef(ref forge.Ref, bindings map[string]forgeResourceBinding) (forgeResourceBinding, error) {
	if ref.ID == "" {
		return forgeResourceBinding{}, nil
	}
	binding, ok := bindings[forgeNodeKey(ref.Kind, ref.ID)]
	if !ok {
		return binding, fmt.Errorf("missing dependency %s (supply --map %s=NATIVE_ID)", ref.Kind, forgeReferenceMappingKey(ref))
	}
	if ref.Revision != "" && ref.Revision != binding.SourceRevision {
		return binding, errors.New("source reference pins an unavailable revision")
	}
	return binding, nil
}

// Typed conversion is deliberately explicit: only declared reference fields can
// bind native identities. No JSON-pointer patching or source-selected paths.
func forgeResourceNodeState(n forge.Node, bindings map[string]forgeResourceBinding, noAnswer bool) (json.RawMessage, error) {
	var state any
	switch n.Kind {
	case ResourceProject:
		p := n.Project
		s := ProjectState{Schema: "hn.project/1", Title: p.Title, Body: p.Body, Archived: p.Archived, Metadata: n.Metadata}
		for _, f := range p.Fields {
			nf := ProjectField{ID: f.ID, Name: f.Name, Type: f.Type}
			for _, o := range f.Options {
				nf.Options = append(nf.Options, ProjectOption{ID: o.ID, Name: o.Name})
			}
			s.Fields = append(s.Fields, nf)
		}
		for _, v := range p.Views {
			s.Views = append(s.Views, ProjectView{ID: v.ID, Name: v.Name, Layout: v.Layout, GroupByField: v.GroupByField})
		}
		state = s
	case ResourceProjectItem:
		p := n.Item
		project, err := forgeLookupResourceRef(p.Project, bindings)
		if err != nil {
			return nil, err
		}
		target := ProjectTarget{Kind: p.Target.Kind, Title: p.Target.Title, Body: p.Target.Body, URL: p.Target.URL}
		if p.Target.Kind == "issue" || p.Target.Kind == "proposal" || p.Target.Kind == "discussion" {
			if p.Target.Ref.Kind != p.Target.Kind {
				return nil, errors.New("target reference type mismatch")
			}
			b, err := forgeLookupResourceRef(p.Target.Ref, bindings)
			if err != nil {
				return nil, err
			}
			target.ID = b.ID
		} else if p.Target.Ref.ID != "" || p.Target.Ref.Kind != "" || p.Target.Ref.Revision != "" {
			return nil, errors.New("descriptive target has unexpected native reference")
		}
		values := map[string]ProjectValue{}
		for id, v := range p.Values {
			values[id] = ProjectValue{Type: v.Type, Text: v.Text, Number: v.Number, Date: v.Date, OptionID: v.OptionID}
		}
		state = ProjectItemState{Schema: "hn.project-item/1", ProjectID: project.ID, ProjectRevision: project.Revision, Target: target, Values: values, Archived: p.Archived, Position: p.Position, Metadata: n.Metadata}
	case ResourceDiscussion:
		p := n.Discussion
		answer := ""
		if !noAnswer && p.AcceptedReply.ID != "" {
			if p.AcceptedReply.Kind != ResourceDiscussionReply {
				return nil, errors.New("answer reference requires a reply")
			}
			b, err := forgeLookupResourceRef(p.AcceptedReply, bindings)
			if err != nil {
				return nil, err
			}
			answer = b.Revision
		}
		state = DiscussionState{Schema: "hn.discussion/1", Title: p.Title, Body: p.Body, Category: p.Category, Status: p.Status, AcceptedReplyRevision: answer, Metadata: n.Metadata}
	case ResourceDiscussionReply:
		p := n.Reply
		thread, err := forgeLookupResourceRef(p.Discussion, bindings)
		if err != nil {
			return nil, err
		}
		parent, err := forgeLookupResourceRef(p.Parent, bindings)
		if err != nil {
			return nil, err
		}
		state = DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: thread.ID, ReplyTo: parent.ID, Body: p.Body, Metadata: n.Metadata}
	case ResourceWorkflow:
		p := n.Workflow
		var pipeline *PipelineDefinition
		if len(p.Pipeline) > 0 {
			pipeline = &PipelineDefinition{}
			if err := strictResourceJSON(p.Pipeline, pipeline, maxPipelineSize); err != nil {
				return nil, err
			}
		}
		s := WorkflowState{Schema: "hn.workflow/1", Name: p.Name, Title: p.Title, Description: p.Description, Enabled: p.Enabled, Pipeline: pipeline, Metadata: n.Metadata}
		for _, d := range p.Sources {
			s.SourceDocuments = append(s.SourceDocuments, WorkflowSourceDocument{Format: d.Format, Content: d.Content, Digest: d.Digest})
		}
		state = s
	default:
		return nil, errors.New("unsupported typed resource node")
	}
	return ResourceStateJSON(n.Kind, state)
}
func forgeResourceAttach(raw json.RawMessage, family, opening string, observation forgeResourceObservation, base map[string]string) (json.RawMessage, error) {
	value, metadata, err := forgeResourceStateParts(family, raw)
	if err != nil {
		return nil, err
	}
	if base != nil {
		metadata = map[string]string{}
		for k, v := range base {
			metadata[k] = v
		}
	}
	metadata[forgeResourceSourceKey] = opening
	obs, err := forgeMetadata(observation)
	if err != nil {
		return nil, err
	}
	metadata[forgeResourceObservationKey] = obs
	digest, err := forgeResourceDigest(family, raw)
	if err != nil {
		return nil, err
	}
	metadata[forgeResourceBaselineKey] = digest
	forgeSetResourceMetadata(value, metadata)
	return ResourceStateJSON(family, value)
}

type forgeResourceStep struct {
	Node     int                             `json:"node"`
	NoAnswer bool                            `json:"no_answer"`
	ID       string                          `json:"id"`
	Expected []string                        `json:"expected"`
	Output   string                          `json:"output"`
	Opening  string                          `json:"opening"`
	Bindings map[string]forgeResourceBinding `json:"bindings"`
}
type forgeResourceJournal struct {
	Anchors                                                map[string]string
	Schema, Scope, Actor, Provider, Endpoint, Project, Key string
	Resource                                               forge.Resource
	Graph                                                  forge.Graph
	Steps                                                  []forgeResourceStep
	Next                                                   int
	Replacements                                           map[string]string
	Pending                                                *ResourceMutation
}

func forgeResourceScope(p forge.Identity, o transferOptions) string {
	raw, _ := json.Marshal(o.mappings)
	return forgeOperation("resource-import-scope/1", p.Kind(), p.Endpoint(), string(o.resource), o.project, o.key, string(raw))
}
func forgeResourceJournalName(scope string) string {
	return "resource-import-" + strings.TrimPrefix(eventID([]byte(scope)), "sha256:") + ".json"
}
func forgeCloneBindings(in map[string]forgeResourceBinding) map[string]forgeResourceBinding {
	out := map[string]forgeResourceBinding{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func forgePreviewID(scope string, index int) string {
	return eventID([]byte(fmt.Sprintf("hn resource preview only %s %d", scope, index)))
}

func forgeBuildResourcePlan(p forge.Identity, o transferOptions, g forge.Graph, events []StoredEvent) (forgeResourceJournal, error) {
	journal := forgeResourceJournal{Schema: "hn.forge-resource-import/1", Scope: forgeResourceScope(p, o), Provider: p.Kind(), Endpoint: p.Endpoint(), Project: o.project, Key: o.key, Resource: o.resource, Graph: g, Replacements: map[string]string{}, Anchors: forgeResourceAnchors(events)}
	if err := forgeValidateGraph(p, g, o.resource); err != nil {
		return journal, err
	}
	report := forgeResourcePlanReport(g, o)
	if report.Blocked() {
		return journal, errors.New("resource migration plan requires mapping or has unsupported/missing/conflicting data")
	}
	cat, err := BuildResourceCatalog(events)
	if err != nil {
		return journal, err
	}
	mappings, err := forgeResourceMappings(cat)
	if err != nil {
		return journal, err
	}
	usedMappings := map[string]bool{}
	for _, key := range g.UsedMappings {
		if !issueIdentifier(key, 128) || usedMappings[key] {
			return journal, errors.New("invalid consumed provider mapping declaration")
		}
		if _, ok := o.mappings[key]; !ok {
			return journal, errors.New("provider consumed an absent mapping")
		}
		usedMappings[key] = true
	}
	bindings := map[string]forgeResourceBinding{}
	mapped := map[string]forgeResourceMapping{}
	for i, n := range g.Nodes {
		source := forgeResourceOrigin(p, o, n)
		key := forgeNodeKey(n.Kind, n.Record.ID)
		if m, ok := mappings[forgeResourceIdentity(source)]; ok {
			mapped[key] = m
			bindings[key] = forgeResourceBinding{m.View.ID, m.Witness.ID, n.Record.Revision}
		} else {
			preview := forgePreviewID(journal.Scope, i)
			bindings[key] = forgeResourceBinding{preview, preview, n.Record.Revision}
		}
	}
	// External target mappings are explicit and checked against accepted native facts.
	byID := map[string]StoredEvent{}
	for _, e := range events {
		byID[e.ID] = e
	}
	for _, n := range g.Nodes {
		refs := []forge.Ref{}
		if n.Item != nil {
			refs = append(refs, n.Item.Target.Ref)
		}
		for _, ref := range refs {
			if ref.ID == "" {
				continue
			}
			key := forgeNodeKey(ref.Kind, ref.ID)
			if _, ok := bindings[key]; ok {
				continue
			}
			nativeID := o.mappings[forgeReferenceMappingKey(ref)]
			if nativeID != "" {
				usedMappings[forgeReferenceMappingKey(ref)] = true
			}
			if nativeID == "" && ref.Kind == "issue" {
				for _, e := range events {
					if e.Event.Kind != "issue.open" || e.Event.Issue == nil {
						continue
					}
					source, err := forgeParseSource(e.Event.Issue.Metadata["forge/source"])
					if err == nil && source.Provider == p.Kind() && source.Endpoint == p.Endpoint() && source.ID == ref.ID {
						if nativeID != "" && nativeID != e.ID {
							return journal, errors.New("duplicate issue source mapping")
						}
						nativeID = e.ID
					}
				}
			}
			e, ok := byID[nativeID]
			if !ok || ref.Kind == "issue" && e.Event.Kind != "issue.open" || ref.Kind == "discussion" && e.Event.Kind != "discussion.open" || ref.Kind == "proposal" && !isProposalKind(e.Event.Kind) {
				return journal, fmt.Errorf("missing or wrong-type dependency; supply --map %s=NATIVE_ID", forgeReferenceMappingKey(ref))
			}
			if ref.Kind != "issue" && ref.Kind != "discussion" && ref.Kind != "proposal" {
				return journal, errors.New("unsupported project target mapping")
			}
			bindings[key] = forgeResourceBinding{nativeID, nativeID, ref.Revision}
		}
	}
	for key := range o.mappings {
		if !usedMappings[key] {
			return journal, errors.New("unsupported or unused explicit mapping key")
		}
	}
	simulated := append([]StoredEvent{}, events...)
	current := map[string]json.RawMessage{}
	heads := map[string]string{}
	opening := map[string]string{}
	for key, m := range mapped {
		current[key] = m.View.State
		heads[key] = m.View.Heads[0].ID
		opening[key] = m.Opening
	}
	add := func(index int, noAnswer bool) error {
		n := g.Nodes[index]
		key := forgeNodeKey(n.Kind, n.Record.ID)
		source := forgeResourceOrigin(p, o, n)
		desired, err := forgeResourceNodeState(n, bindings, noAnswer)
		if err != nil {
			return err
		}
		remoteNode := n
		if noAnswer {
			copy := *n.Discussion
			copy.AcceptedReply = forge.Ref{}
			remoteNode.Discussion = &copy
		}
		remoteDigest := forgeResourceRemoteDigest(remoteNode)
		if prior := current[key]; prior != nil {
			_, metadata, _ := forgeResourceStateParts(n.Kind, prior)
			var obs forgeResourceObservation
			if forgeStrictJSON([]byte(metadata[forgeResourceObservationKey]), &obs) != nil {
				return errors.New("missing signed resource observation")
			}
			if obs.RemoteDigest == remoteDigest {
				return nil
			}
			nativeDigest, _ := forgeResourceDigest(n.Kind, prior)
			desiredDigest, _ := forgeResourceDigest(n.Kind, desired)
			if nativeDigest != metadata[forgeResourceBaselineKey] && nativeDigest != desiredDigest {
				return errors.New("competing native and provider resource edits")
			}
		}
		sourceJSON := opening[key]
		if sourceJSON == "" {
			sourceJSON, err = forgeMetadata(source)
			if err != nil {
				return err
			}
			opening[key] = sourceJSON
		}
		base := map[string]string(nil)
		if prior := current[key]; prior != nil {
			_, base, _ = forgeResourceStateParts(n.Kind, prior)
		}
		state, err := forgeResourceAttach(desired, n.Kind, sourceJSON, forgeResourceObservation{source, remoteDigest}, base)
		if err != nil {
			return err
		}
		id, intent := "", "open"
		expected := []string{}
		if heads[key] != "" {
			id = bindings[key].ID
			intent = "revise"
			expected = []string{heads[key]}
		}
		output := forgePreviewID(journal.Scope, forgeResourceNodeLimit+len(journal.Steps))
		if id == "" {
			output = bindings[key].ID
		}
		step := forgeResourceStep{Node: index, NoAnswer: noAnswer, ID: id, Expected: expected, Output: output, Opening: sourceJSON, Bindings: forgeCloneBindings(bindings)}
		journal.Steps = append(journal.Steps, step)
		event := Event{Kind: n.Kind + "." + intent, Intent: intent, Subject: id, Parents: expected, Body: string(state)}
		eventRaw, _ := json.Marshal(event)
		if len(eventRaw) > resourcePayloadLimit(n.Kind)-4096 {
			return errors.New("resource state leaves insufficient authenticated event envelope budget")
		}
		simulated = append(simulated, StoredEvent{ID: output, Event: event})
		current[key] = state
		heads[key] = output
		b := bindings[key]
		b.Revision = output
		bindings[key] = b
		return nil
	}
	// Establish containers, then independent replies/items, then exact answer pins.
	for i, n := range g.Nodes {
		if n.Project != nil || n.Workflow != nil {
			if err := add(i, false); err != nil {
				return journal, err
			}
		} else if n.Discussion != nil && current[forgeNodeKey(n.Kind, n.Record.ID)] == nil {
			if err := add(i, true); err != nil {
				return journal, err
			}
		}
	}
	pending := map[int]bool{}
	for i, n := range g.Nodes {
		if n.Reply != nil {
			pending[i] = true
		}
	}
	for len(pending) > 0 {
		progress := false
		for i, n := range g.Nodes {
			if !pending[i] {
				continue
			}
			if n.Reply.Discussion.Kind != "discussion" || n.Reply.Parent.ID != "" && n.Reply.Parent.Kind != "discussion-reply" {
				return journal, errors.New("invalid reply reference type")
			}
			parentReady := n.Reply.Parent.ID == ""
			if !parentReady {
				for j, pn := range g.Nodes {
					if pn.Kind == n.Reply.Parent.Kind && pn.Record.ID == n.Reply.Parent.ID {
						parentReady = !pending[j]
					}
				}
			}
			if !parentReady {
				continue
			}
			if err := add(i, false); err != nil {
				return journal, err
			}
			delete(pending, i)
			progress = true
		}
		if !progress {
			return journal, errors.New("missing or cyclic reply parents")
		}
	}
	for i, n := range g.Nodes {
		if n.Item != nil {
			if n.Item.Project.Kind != "project" {
				return journal, errors.New("item must reference a project")
			}
			if err := add(i, false); err != nil {
				return journal, err
			}
		}
	}
	for i, n := range g.Nodes {
		if n.Discussion != nil {
			if err := add(i, false); err != nil {
				return journal, err
			}
		}
	}
	if _, err := BuildResourceCatalog(simulated); err != nil {
		return journal, fmt.Errorf("resource graph preflight: %w", err)
	}
	raw, _ := json.Marshal(journal)
	if len(raw) > forgeCheckpointLimit {
		return journal, errors.New("complete resource plan exceeds private checkpoint bound")
	}
	return journal, nil
}

func forgeResourceResolve(id string, replacements map[string]string) string {
	if actual, ok := replacements[id]; ok {
		return actual
	}
	return id
}
func forgeResourceStepMutation(j forgeResourceJournal, index int, events []StoredEvent) (ResourceMutation, error) {
	if index < 0 || index >= len(j.Steps) {
		return ResourceMutation{}, errors.New("invalid resource step")
	}
	step := j.Steps[index]
	if step.Node < 0 || step.Node >= len(j.Graph.Nodes) {
		return ResourceMutation{}, errors.New("invalid resource node")
	}
	n := j.Graph.Nodes[step.Node]
	bindings := forgeCloneBindings(step.Bindings)
	for key, b := range bindings {
		b.ID = forgeResourceResolve(b.ID, j.Replacements)
		b.Revision = forgeResourceResolve(b.Revision, j.Replacements)
		bindings[key] = b
	}
	state, err := forgeResourceNodeState(n, bindings, step.NoAnswer)
	if err != nil {
		return ResourceMutation{}, err
	}
	id := forgeResourceResolve(step.ID, j.Replacements)
	expected := []string{}
	for _, p := range step.Expected {
		expected = append(expected, forgeResourceResolve(p, j.Replacements))
	}
	var base map[string]string
	if id != "" {
		found := false
		for _, e := range events {
			if len(expected) == 1 && e.ID == expected[0] {
				if resourceRootID(e) != id || resourceFamily(e.Event.Kind) != n.Kind {
					return ResourceMutation{}, errors.New("pending resource expectation has wrong identity")
				}
				_, base, err = forgeResourceStateParts(n.Kind, json.RawMessage(e.Event.Body))
				if err != nil {
					return ResourceMutation{}, err
				}
				found = true
				break
			}
		}
		if !found {
			return ResourceMutation{}, errors.New("pending exact native expectation unavailable")
		}
		if base[forgeResourceSourceKey] != step.Opening {
			return ResourceMutation{}, errors.New("pending native source association changed")
		}
	}
	source := forgeResourceOrigin(resourceJournalIdentity{j.Provider, j.Endpoint}, transferOptions{project: j.Project}, n)
	if err := forgeValidateResourceSource(source); err != nil {
		return ResourceMutation{}, err
	}
	initial, err := forgeParseResourceSource(step.Opening)
	if err != nil || forgeResourceIdentity(source) != forgeResourceIdentity(initial) {
		return ResourceMutation{}, errors.New("pending immutable source mismatch")
	}
	remoteNode := n
	if step.NoAnswer {
		copy := *n.Discussion
		copy.AcceptedReply = forge.Ref{}
		remoteNode.Discussion = &copy
	}
	state, err = forgeResourceAttach(state, n.Kind, step.Opening, forgeResourceObservation{source, forgeResourceRemoteDigest(remoteNode)}, base)
	if err != nil {
		return ResourceMutation{}, err
	}
	intent := "open"
	if id != "" {
		intent = "revise"
	}
	m := ResourceMutation{Family: n.Kind, Intent: intent, ID: id, Expected: expected, State: state, Actor: j.Actor}
	m.Operation = forgeResourceOperation(m)
	return m, nil
}

type resourceJournalIdentity struct{ provider, endpoint string }

func (p resourceJournalIdentity) Kind() string     { return p.provider }
func (p resourceJournalIdentity) Endpoint() string { return p.endpoint }
func forgeRunResourceJournal(ctx context.Context, p forge.Identity, o transferOptions, j *forgeResourceJournal, name string) error {
	if j.Schema != "hn.forge-resource-import/1" || j.Scope != forgeResourceScope(p, o) || j.Provider != p.Kind() || j.Endpoint != p.Endpoint() || j.Project != o.project || j.Key != o.key || j.Resource != o.resource || j.Next < 0 || j.Next > len(j.Steps) || len(j.Steps) > 2*forgeResourceNodeLimit || len(j.Replacements) > 2*forgeResourceNodeLimit {
		return errors.New("resource recovery checkpoint does not match selection")
	}
	if err := forgeValidateGraph(p, j.Graph, o.resource); err != nil {
		return err
	}
	if forgeResourcePlanReport(j.Graph, o).Blocked() {
		return errors.New("pending graph has unresolved mapping report")
	}
	accepted, _, err := collectIssueEvents()
	if err != nil {
		return err
	}
	original, err := forgeEventsAtAnchors(accepted, j.Anchors)
	if err != nil {
		return err
	}
	rebuilt, err := forgeBuildResourcePlan(p, o, j.Graph, original)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(rebuilt.Steps)
	b, _ := json.Marshal(j.Steps)
	if string(a) != string(b) {
		return errors.New("pending resource plan differs from authenticated original snapshot")
	}
	confirmed := map[string]string{}
	for i := 0; i < j.Next; i++ {
		copy := *j
		copy.Replacements = confirmed
		m, err := forgeResourceStepMutation(copy, i, accepted)
		if err != nil {
			return err
		}
		observed, err := ObserveResourceOperation(accepted, j.Actor, m.Operation, m.Family)
		if err != nil || string(observed.State) != string(m.State) {
			return errors.New("resource recovery progress has no matching signed operation")
		}
		confirmed[j.Steps[i].Output] = observed.EventID
	}
	a, _ = json.Marshal(confirmed)
	b, _ = json.Marshal(j.Replacements)
	if string(a) != string(b) {
		return errors.New("resource recovery bindings differ from signed progress")
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	if j.Actor != identity.Actor {
		return errors.New("resource recovery signing actor changed")
	}
	for j.Next < len(j.Steps) {
		if err := ctx.Err(); err != nil {
			return err
		}
		events, _, err := collectIssueEvents()
		if err != nil {
			return err
		}
		m, err := forgeResourceStepMutation(*j, j.Next, events)
		if err != nil {
			return err
		}
		if j.Pending != nil {
			a, _ := json.Marshal(j.Pending)
			b, _ := json.Marshal(m)
			if string(a) != string(b) {
				return errors.New("resource pending intent differs from frozen typed graph")
			}
		} else {
			j.Pending = &m
			if err := forgeStoreJSON(name, j); err != nil {
				return err
			}
		}
		result, err := forgeMutateResource(m)
		if err != nil {
			return err
		}
		if !validEventID(result.EventID) || !validEventID(result.ResourceID) || result.Conflict {
			return errors.New("native resource write did not confirm one exact revision")
		}
		// Authenticate returned/replayed result before binding a dependent reference.
		accepted, _, err := collectIssueEvents()
		if err != nil {
			return err
		}
		observed, err := ObserveResourceOperation(accepted, j.Actor, m.Operation, m.Family)
		if err != nil || observed.EventID != result.EventID || observed.ResourceID != result.ResourceID || string(observed.State) != string(m.State) {
			return errors.New("native result did not match exact signed mutation")
		}
		step := j.Steps[j.Next]
		if m.Intent == "open" {
			if result.EventID != result.ResourceID {
				return errors.New("native opening identity mismatch")
			}
		} else if result.ResourceID != m.ID {
			return errors.New("native revision identity mismatch")
		}
		j.Replacements[step.Output] = result.EventID
		j.Next++
		j.Pending = nil
		if err := forgeStoreJSON(name, j); err != nil {
			return err
		}
	}
	return forgeRemoveCheckpoint(name)
}
func forgeImportResources(ctx context.Context, p forge.Identity, o transferOptions) error {
	scope := forgeResourceScope(p, o)
	name := forgeResourceJournalName(scope)
	var saved forgeResourceJournal
	found, err := forgeLoadJSON(name, &saved)
	if err != nil {
		return err
	}
	if found {
		if o.dryRun {
			return errors.New("pending native resource import exists; resume exact transfer before previewing newer source")
		}
		return forgeRunResourceJournal(ctx, p, o, &saved, name)
	}
	graph, err := forgeReadResourceGraph(ctx, p, o)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		return err
	}
	journal, err := forgeBuildResourcePlan(p, o, graph, events)
	if err != nil {
		return forgeResourcePlanFailure(graph, o, err)
	}
	report := forgeResourcePlanReport(graph, o)
	if o.dryRun {
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	if len(journal.Steps) == 0 {
		return nil
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	journal.Actor = identity.Actor
	if err := forgeStoreJSON(name, &journal); err != nil {
		return err
	}
	return forgeRunResourceJournal(ctx, p, o, &journal, name)
}

// A snapshot retains exact signed native versions, including schema/answer pins.
// Graph uses native IDs as its opaque identities; adapters map these explicitly.
type forgeResourceVersion struct {
	ID, Revision, Family, Actor, Timestamp string
	State                                  json.RawMessage
}
type forgeResourceSnapshot struct {
	Anchors                  map[string]string
	NativeID, NativeRevision string
	Resource                 forge.Resource
	Graph                    forge.Graph
	Versions                 []forgeResourceVersion
	Digest                   string
}

func forgeNodeFromNative(v *ResourceView) (forge.Node, error) {
	if v.Conflict || len(v.Heads) != 1 {
		return forge.Node{}, errors.New("export resource has concurrent heads")
	}
	value, metadata, err := forgeResourceStateParts(v.Family, v.State)
	if err != nil {
		return forge.Node{}, err
	}
	var opening, head StoredEvent
	for _, e := range v.History {
		if e.ID == v.ID {
			opening = e
		}
		if e.ID == v.Heads[0].ID {
			head = e
		}
	}
	n := forge.Node{Kind: v.Family, Record: forge.Record{ID: v.ID, Key: v.ID, Revision: head.ID, Author: head.Event.Actor, CreatedAt: opening.Event.Timestamp, UpdatedAt: head.Event.Timestamp}, Metadata: metadata}
	switch s := value.(type) {
	case *ProjectState:
		p := &forge.Project{Title: s.Title, Body: s.Body, Archived: s.Archived}
		for _, f := range s.Fields {
			nf := forge.Field{ID: f.ID, Name: f.Name, Type: f.Type}
			for _, o := range f.Options {
				nf.Options = append(nf.Options, forge.Option{ID: o.ID, Name: o.Name})
			}
			p.Fields = append(p.Fields, nf)
		}
		for _, v := range s.Views {
			p.Views = append(p.Views, forge.View{ID: v.ID, Name: v.Name, Layout: v.Layout, GroupByField: v.GroupByField})
		}
		n.Project = p
	case *ProjectItemState:
		values := map[string]forge.Value{}
		for id, v := range s.Values {
			values[id] = forge.Value{Type: v.Type, Text: v.Text, Number: v.Number, Date: v.Date, OptionID: v.OptionID}
		}
		target := forge.Target{Kind: s.Target.Kind, Title: s.Target.Title, Body: s.Target.Body, URL: s.Target.URL}
		if s.Target.ID != "" {
			target.Ref = forge.Ref{Kind: s.Target.Kind, ID: s.Target.ID}
		}
		n.Item = &forge.ProjectItem{Project: forge.Ref{Kind: "project", ID: s.ProjectID, Revision: s.ProjectRevision}, Target: target, Values: values, Archived: s.Archived, Position: s.Position}
	case *DiscussionState:
		answer := forge.Ref{}
		if s.AcceptedReplyRevision != "" {
			answer = forge.Ref{Kind: "discussion-reply", Revision: s.AcceptedReplyRevision}
		}
		n.Discussion = &forge.Discussion{Title: s.Title, Body: s.Body, Category: s.Category, Status: s.Status, AcceptedReply: answer}
	case *DiscussionReplyState:
		parent := forge.Ref{}
		if s.ReplyTo != "" {
			parent = forge.Ref{Kind: "discussion-reply", ID: s.ReplyTo}
		}
		n.Reply = &forge.Reply{Discussion: forge.Ref{Kind: "discussion", ID: s.DiscussionID}, Parent: parent, Body: s.Body}
	case *WorkflowState:
		p := &forge.Workflow{Name: s.Name, Title: s.Title, Description: s.Description, Enabled: s.Enabled}
		if s.Pipeline != nil {
			p.Pipeline, _ = json.Marshal(s.Pipeline)
		}
		for _, d := range s.SourceDocuments {
			p.Sources = append(p.Sources, forge.SourceDocument{Format: d.Format, Content: d.Content, Digest: d.Digest})
		}
		n.Workflow = p
	}
	return n, nil
}
func forgeSnapshotResource(id string, resource forge.Resource) (forgeResourceSnapshot, error) {
	events, _, err := collectIssueEvents()
	if err != nil {
		return forgeResourceSnapshot{}, err
	}
	return forgeSnapshotResourceAt(id, resource, events)
}
func forgeSnapshotResourceAt(id string, resource forge.Resource, events []StoredEvent) (forgeResourceSnapshot, error) {
	snapshot := forgeResourceSnapshot{Anchors: forgeResourceAnchors(events), NativeID: id, Resource: resource, Graph: forge.Graph{Nodes: []forge.Node{}, Findings: []forge.Finding{}}, Versions: []forgeResourceVersion{}}
	if !validEventID(id) {
		return snapshot, errors.New("resource export requires a full native ID")
	}
	cat, err := BuildResourceCatalog(events)
	if err != nil {
		return snapshot, err
	}
	root := cat.Resources[id]
	if root == nil || root.Family != string(resource) {
		return snapshot, errors.New("selected native resource does not match family")
	}
	if root.Conflict {
		return snapshot, errors.New("selected native resource has concurrent heads")
	}
	snapshot.NativeRevision = root.Heads[0].ID
	selected := map[string]bool{id: true}
	for _, candidate := range cat.IDs {
		v := cat.Resources[candidate]
		if resource == forge.ResourceProject && v.Family == ResourceProjectItem || resource == forge.ResourceDiscussion && v.Family == ResourceDiscussionReply {
			for _, h := range v.Heads {
				value, _, err := forgeResourceStateParts(v.Family, h.State)
				if err != nil {
					return snapshot, err
				}
				switch s := value.(type) {
				case *ProjectItemState:
					if s.ProjectID == id {
						selected[candidate] = true
					}
				case *DiscussionReplyState:
					if s.DiscussionID == id {
						selected[candidate] = true
					}
				}
			}
		}
	}
	byID := map[string]StoredEvent{}
	for _, e := range events {
		byID[e.ID] = e
	}
	pins := map[string]bool{}
	ids := []string{}
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := cat.Resources[id]
		n, err := forgeNodeFromNative(v)
		if err != nil {
			return snapshot, err
		}
		if n.Discussion != nil && n.Discussion.AcceptedReply.Revision != "" {
			answer, ok := byID[n.Discussion.AcceptedReply.Revision]
			if !ok {
				return snapshot, errors.New("native answer version inaccessible")
			}
			n.Discussion.AcceptedReply.ID = resourceRootID(answer)
		}
		snapshot.Graph.Nodes = append(snapshot.Graph.Nodes, n)
		pins[v.Heads[0].ID] = true
		pins[v.ID] = true
		for _, h := range v.Heads {
			for _, ref := range resourceReferences(byID[h.ID].Event) {
				if e, ok := byID[ref]; ok && isResourceKind(e.Event.Kind) {
					pins[ref] = true
				}
			}
		}
	}
	versions := []string{}
	for id := range pins {
		versions = append(versions, id)
	}
	sort.Strings(versions)
	for _, revision := range versions {
		e := byID[revision]
		canonical, err := CanonicalResourceState(resourceFamily(e.Event.Kind), []byte(e.Event.Body))
		if err != nil {
			return snapshot, err
		}
		snapshot.Versions = append(snapshot.Versions, forgeResourceVersion{resourceRootID(e), e.ID, resourceFamily(e.Event.Kind), e.Event.Actor, e.Event.Timestamp, canonical})
	}
	raw, _ := json.Marshal(snapshot)
	if len(raw) > forgeResourceGraphLimit || len(snapshot.Graph.Nodes) > forgeResourceNodeLimit {
		return snapshot, errors.New("native resource snapshot exceeds transfer bounds")
	}
	snapshot.Digest = eventID(raw)
	return snapshot, nil
}

// Checkpoints may keep old snapshots; authenticate their exact revisions, not
// the current heads, before any adapter resumes an external mutation.
func forgeAuthenticateResourceSnapshot(snapshot forgeResourceSnapshot) error {
	events, _, err := collectIssueEvents()
	if err != nil {
		return err
	}
	original, err := forgeEventsAtAnchors(events, snapshot.Anchors)
	if err != nil {
		return err
	}
	expected, err := forgeSnapshotResourceAt(snapshot.NativeID, snapshot.Resource, original)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(expected)
	b, _ := json.Marshal(snapshot)
	if string(a) != string(b) {
		return errors.New("resource export snapshot differs from exact authenticated native graph")
	}
	return nil
}
func forgeImportedResourceBaseline(id string) (forgeResourceSource, forgeResourceObservation, forgeResourceVersion, error) {
	var source forgeResourceSource
	var observation forgeResourceObservation
	var version forgeResourceVersion
	events, _, err := collectIssueEvents()
	if err != nil {
		return source, observation, version, err
	}
	cat, err := BuildResourceCatalog(events)
	if err != nil {
		return source, observation, version, err
	}
	mappings, err := forgeResourceMappings(cat)
	if err != nil {
		return source, observation, version, err
	}
	for _, m := range mappings {
		if m.View.ID == id {
			_, metadata, _ := forgeResourceStateParts(m.View.Family, json.RawMessage(m.Witness.Event.Body))
			_ = forgeStrictJSON([]byte(metadata[forgeResourceObservationKey]), &observation)
			source = m.Source
			version = forgeResourceVersion{m.View.ID, m.Witness.ID, m.View.Family, m.Witness.Event.Actor, m.Witness.Event.Timestamp, json.RawMessage(m.Witness.Event.Body)}
			return source, observation, version, nil
		}
	}
	return source, observation, version, errors.New("native resource has no verified imported baseline")
}

func forgeValidateNodeRefs(n forge.Node) error {
	check := func(ref forge.Ref, kind string, required bool) error {
		if ref.ID == "" {
			if required || ref.Kind != "" || ref.Revision != "" {
				return errors.New("missing or incomplete typed resource reference")
			}
			return nil
		}
		if ref.Kind != kind || !issueIdentifier(ref.ID, 1024) || ref.Revision != "" && !issueIdentifier(ref.Revision, 1024) {
			return errors.New("invalid typed resource reference")
		}
		return nil
	}
	if n.Item != nil {
		if err := check(n.Item.Project, ResourceProject, true); err != nil {
			return err
		}
		if n.Item.Target.Ref.ID != "" {
			if err := check(n.Item.Target.Ref, n.Item.Target.Kind, true); err != nil {
				return err
			}
		}
	}
	if n.Reply != nil {
		if err := check(n.Reply.Discussion, ResourceDiscussion, true); err != nil {
			return err
		}
		if err := check(n.Reply.Parent, ResourceDiscussionReply, false); err != nil {
			return err
		}
	}
	if n.Discussion != nil {
		if err := check(n.Discussion.AcceptedReply, ResourceDiscussionReply, false); err != nil {
			return err
		}
	}
	return nil
}
func forgeResourceAnchors(events []StoredEvent) map[string]string {
	anchors := map[string]string{}
	sequence := map[string]uint64{}
	for _, e := range events {
		if e.Event.Sequence > sequence[e.Event.Actor] {
			sequence[e.Event.Actor] = e.Event.Sequence
			anchors[e.Event.Actor] = e.ID
		}
	}
	return anchors
}
func forgeEventsAtAnchors(events []StoredEvent, anchors map[string]string) ([]StoredEvent, error) {
	if len(anchors) > 100000 {
		return nil, errors.New("native snapshot anchor bound exceeded")
	}
	limits := map[string]uint64{}
	for _, e := range events {
		if anchors[e.Event.Actor] == e.ID {
			limits[e.Event.Actor] = e.Event.Sequence
		}
	}
	if len(limits) != len(anchors) {
		return nil, errors.New("native snapshot signed actor anchor unavailable")
	}
	selected := []StoredEvent{}
	for _, e := range events {
		if limit, ok := limits[e.Event.Actor]; ok && e.Event.Sequence <= limit {
			selected = append(selected, e)
		}
	}
	return selected, nil
}

type forgeResourceBlocked struct{ Plan forge.Plan }

func (e *forgeResourceBlocked) Error() string {
	raw, _ := json.Marshal(e.Plan)
	return "resource migration blocked: " + string(raw)
}
func forgeResourcePlanFailure(g forge.Graph, o transferOptions, err error) error {
	plan := forgeResourcePlanReport(g, o)
	status := forge.Unsupported
	message := err.Error()
	if strings.Contains(message, "missing") || strings.Contains(message, "unavailable") || strings.Contains(message, "dependency") || strings.Contains(message, "cyclic") {
		status = forge.MissingDependency
	}
	if strings.Contains(message, "conflict") || strings.Contains(message, "competing") || strings.Contains(message, "duplicate") || strings.Contains(message, "baseline") || strings.Contains(message, "witness") {
		status = forge.IdentityConflict
	}
	plan.Findings = append(plan.Findings, forge.Finding{Status: status, Path: string(o.resource), Message: message})
	return &forgeResourceBlocked{plan}
}
