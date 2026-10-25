//go:build !nogithub

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"hubnot/internal/forge"
	"hubnot/internal/githubapi"
)

func init() {
	r := forgeProviders["github"]
	r.Capabilities = append(r.Capabilities, forge.Capability{Resource: forge.ResourceDiscussion, Import: true, Export: true, MappedFields: []string{"title/body", "independent replies and reply-to", "category via explicit category ID", "open/closed state", "exact accepted reply content", "source attribution"}, Unsupported: []string{"complete edit/deletion history: --map history=snapshot required", "polls, locks, votes/reactions and minimized/deleted content require --map losses=acknowledge", "nested replies beyond one parent cannot be exported", "permissions and moderation authority"}})
	forgeProviders["github"] = r
	forgeResourceExporters[forgeResourceRoute{"github", forge.ResourceDiscussion}] = githubExportDiscussion
}

func githubDiscussionMappings(s forge.Selection, exporting bool) ([]string, error) {
	used := []string{}
	if s.Mappings["history"] != "snapshot" {
		return nil, errors.New("discussion migration requires --map history=snapshot: GitHub does not provide complete edit/deletion history")
	}
	used = append(used, "history")
	for k, v := range s.Mappings {
		switch k {
		case "history":
		case "losses":
			if v != "acknowledge" {
				return nil, errors.New("losses mapping must be acknowledge")
			}
			used = append(used, k)
		case "category":
			if !exporting || !issueIdentifier(v, 1024) {
				return nil, errors.New("category mapping is an explicit destination category ID for export")
			}
			used = append(used, k)
		default:
			return nil, errors.New("unsupported discussion mapping key")
		}
	}
	return used, nil
}
func githubDiscussionLosses(d githubapi.Discussion) []string {
	losses := []string{}
	if d.Locked || d.ActiveLockReason != "" {
		losses = append(losses, "lock/moderation state")
	}
	if d.PollID != "" {
		losses = append(losses, "poll")
	}
	if d.ReactionCount > 0 || d.UpvoteCount > 0 {
		losses = append(losses, "votes/reactions")
	}
	for _, r := range d.Comments {
		if r.IsMinimized || r.DeletedAt != "" {
			losses = append(losses, "minimized/deleted reply")
		}
		if r.ReactionCount > 0 || r.UpvoteCount > 0 {
			losses = append(losses, "reply votes/reactions")
		}
	}
	return losses
}
func githubDiscussionRecord(id, key, revision, url, author, created, updated string) forge.Record {
	return forge.Record{ID: id, Key: key, Revision: revision, URL: url, Author: author, CreatedAt: created, UpdatedAt: updated}
}
func githubDiscussionGraph(ds []githubapi.Discussion, s forge.Selection) (forge.Graph, error) {
	used, err := githubDiscussionMappings(s, false)
	if err != nil {
		return forge.Graph{}, err
	}
	graph := forge.Graph{UsedMappings: used, Nodes: []forge.Node{}, Findings: []forge.Finding{{Status: forge.Preserved, Path: "history", Message: "present-day text snapshots only; complete edit/deletion history is inaccessible and explicitly acknowledged"}}}
	for _, d := range ds {
		if proof, ok := githubDiscussionParseProof(d.Body); ok {
			native, err := githubDiscussionNativePublication(proof, d.Body)
			if err != nil {
				return graph, err
			}
			if native {
				return graph, errors.New("discussion is already native at a verified signed revision; refusing duplicate import")
			}
		}
		losses := githubDiscussionLosses(d)
		if len(losses) > 0 && s.Mappings["losses"] != "acknowledge" {
			return graph, errors.New("unsupported discussion semantics require --map losses=acknowledge: " + strings.Join(losses, ", "))
		}
		claim, _ := json.Marshal(struct {
			Category                                                              githubapi.DiscussionCategory
			Locked                                                                bool
			ActiveLockReason, PollID, StateReason, AnswerChosenAt, AnswerChosenBy string
			ReactionCount, UpvoteCount                                            int
			EditCount                                                             *int
		}{d.Category, d.Locked, d.ActiveLockReason, d.PollID, d.StateReason, d.AnswerChosenAt, d.AnswerChosenBy, d.ReactionCount, d.UpvoteCount, d.EditCount})
		status := "open"
		if d.Closed {
			status = "closed"
		}
		root := forge.Node{Kind: ResourceDiscussion, Record: githubDiscussionRecord(d.ID, strconv.Itoa(d.Number), d.UpdatedAt, d.URL, d.Author, d.CreatedAt, d.UpdatedAt), Metadata: map[string]string{"github/discussion-claims": string(claim)}, Discussion: &forge.Discussion{Title: d.Title, Body: d.Body, Category: d.Category.Slug, Status: status}}
		seen := map[string]githubapi.DiscussionComment{}
		for _, r := range d.Comments {
			if r.ID == "" || r.DiscussionID != d.ID {
				return graph, errors.New("reply identity is outside selected discussion")
			}
			if _, ok := seen[r.ID]; ok {
				return graph, errors.New("duplicate reply identity")
			}
			seen[r.ID] = r
		}
		if d.AnswerID != "" {
			r, ok := seen[d.AnswerID]
			if !ok || !r.IsAnswer {
				return graph, errors.New("accepted answer is inaccessible or inconsistent")
			}
			root.Discussion.AcceptedReply = forge.Ref{Kind: ResourceDiscussionReply, ID: r.ID, Revision: r.UpdatedAt}
		}
		graph.Nodes = append(graph.Nodes, root)
		for _, r := range d.Comments {
			if proof, ok := githubDiscussionParseProof(r.Body); ok {
				native, err := githubDiscussionNativePublication(proof, r.Body)
				if err != nil {
					return graph, err
				}
				if native {
					return graph, errors.New("reply is already native at a verified signed revision; refusing duplicate import")
				}
			}
			if r.IsAnswer != (r.ID == d.AnswerID) {
				return graph, errors.New("inconsistent accepted answer snapshot")
			}
			parent := forge.Ref{}
			if r.ParentID != "" {
				p, ok := seen[r.ParentID]
				if !ok || p.ParentID != "" {
					return graph, errors.New("reply parent is inaccessible or unsupported")
				}
				parent = forge.Ref{Kind: ResourceDiscussionReply, ID: r.ParentID}
			}
			claim, _ := json.Marshal(struct {
				LastEditedAt, DeletedAt, MinimizedReason string
				IsMinimized                              bool
				ReactionCount, UpvoteCount               int
				EditCount                                *int
			}{r.LastEditedAt, r.DeletedAt, r.MinimizedReason, r.IsMinimized, r.ReactionCount, r.UpvoteCount, r.EditCount})
			graph.Nodes = append(graph.Nodes, forge.Node{Kind: ResourceDiscussionReply, Record: githubDiscussionRecord(r.ID, r.ID, r.UpdatedAt, r.URL, r.Author, r.CreatedAt, r.UpdatedAt), Metadata: map[string]string{"github/reply-claims": string(claim)}, Reply: &forge.Reply{Discussion: forge.Ref{Kind: ResourceDiscussion, ID: d.ID}, Parent: parent, Body: r.Body}})
		}
		if len(losses) > 0 {
			graph.Findings = append(graph.Findings, forge.Finding{Status: forge.Preserved, Path: "discussion/" + d.ID + "/unsupported", Message: "explicitly acknowledged descriptive claims; not native authority: " + strings.Join(losses, ", ")})
		}
	}
	return graph, nil
}
func (g *githubForge) ReadDiscussions(ctx context.Context, s forge.Selection) (forge.Graph, error) {
	if _, err := githubDiscussionMappings(s, false); err != nil {
		return forge.Graph{}, err
	}
	ds := []githubapi.Discussion{}
	if s.Key != "" {
		n, err := githubKey(s.Key)
		if err != nil {
			return forge.Graph{}, err
		}
		d, err := g.client.GetDiscussion(ctx, s.Project, n)
		if err != nil {
			return forge.Graph{}, err
		}
		ds = append(ds, d)
	} else {
		list, err := g.client.ListDiscussions(ctx, s.Project)
		if err != nil {
			return forge.Graph{}, err
		}
		if len(list) > forgeResourceNodeLimit {
			return forge.Graph{}, errors.New("discussion listing exceeds native graph limit")
		}
		for _, d := range list {
			full, err := g.client.GetDiscussion(ctx, s.Project, d.Number)
			if err != nil {
				return forge.Graph{}, err
			}
			if full.ID != d.ID || full.RepositoryID != d.RepositoryID {
				return forge.Graph{}, errors.New("discussion identity changed while hydrating listing")
			}
			ds = append(ds, full)
			bounded, _ := json.Marshal(ds)
			if len(bounded) > forgeResourceGraphLimit {
				return forge.Graph{}, errors.New("complete discussion source exceeds 8MiB transfer graph limit")
			}
			nodes := 0
			for _, v := range ds {
				nodes += 1 + len(v.Comments)
			}
			if nodes > forgeResourceNodeLimit {
				return forge.Graph{}, errors.New("discussion graph exceeds native node limit")
			}
		}
	}
	return githubDiscussionGraph(ds, s)
}

// Proofs are descriptive associations, not authority. Every adoption additionally
// checks exact native snapshot bytes or a signed imported source witness.
type githubDiscussionProof struct{ Schema, Endpoint, NativeID, Revision, ContentDigest string }

const githubDiscussionMarker = "\n\n<!-- hn.discussion/1:"

func githubDiscussionRender(n forge.Node, endpoint string) string {
	body := ""
	if n.Discussion != nil {
		body = n.Discussion.Body
	} else {
		body = n.Reply.Body
	}
	content := body + "\n\nSource: Hubnot " + n.Record.ID + " at " + n.Record.Revision + "; native signer " + n.Record.Author + "; source time " + n.Record.UpdatedAt + ". Attribution does not confer provider identity or authority."
	if len(n.Metadata) > 0 {
		metadata, _ := json.Marshal(n.Metadata)
		content += "\n\nNative metadata and imported source claims (descriptive JSON): " + string(metadata)
	}
	p := githubDiscussionProof{"hn.discussion/1", endpoint, n.Record.ID, n.Record.Revision, eventID([]byte(content))}
	b, _ := json.Marshal(p)
	return content + githubDiscussionMarker + base64.RawURLEncoding.EncodeToString(b) + " -->"
}
func githubDiscussionParseProof(body string) (githubDiscussionProof, bool) {
	var p githubDiscussionProof
	pos := strings.LastIndex(body, githubDiscussionMarker)
	if pos < 0 || !strings.HasSuffix(body, " -->") {
		return p, false
	}
	encoded := body[pos+len(githubDiscussionMarker) : len(body)-4]
	if len(encoded) > 8192 {
		return p, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || forgeStrictJSON(raw, &p) != nil || p.Schema != "hn.discussion/1" || !forgeValidLocator(p.Endpoint) || !validEventID(p.NativeID) || !validEventID(p.Revision) || p.ContentDigest != eventID([]byte(body[:pos])) {
		return githubDiscussionProof{}, false
	}
	return p, true
}

type githubDiscussionProjection struct{ ID, RepositoryID, ParentID, Title, Body, CategoryID, AnswerID, Status string }

func githubDiscussionRootProjection(d githubapi.Discussion) githubDiscussionProjection {
	status := "open"
	if d.Closed {
		status = "closed"
	}
	return githubDiscussionProjection{ID: d.ID, RepositoryID: d.RepositoryID, Title: d.Title, Body: d.Body, CategoryID: d.Category.ID, AnswerID: d.AnswerID, Status: status}
}
func githubDiscussionReplyProjection(r githubapi.DiscussionComment) githubDiscussionProjection {
	return githubDiscussionProjection{ID: r.ID, RepositoryID: r.DiscussionID, ParentID: r.ParentID, Body: r.Body}
}

type githubDiscussionBinding struct {
	RemoteID   string
	SameSource bool
}
type githubDiscussionAction struct {
	NativeID, Action string
	Desired          githubDiscussionProjection
}
type githubDiscussionCheckpoint struct {
	Schema, Scope, Endpoint, Project, CategoryID string
	RepositoryID, SourceRepositoryID             string
	Signer, SignerPublicKey, Signature           string
	Snapshot                                     *forgeResourceSnapshot
	Previous                                     *forgeResourceSnapshot
	Bindings                                     map[string]githubDiscussionBinding
	Baselines                                    map[string]githubDiscussionProjection
	Actions                                      []githubDiscussionAction
	Next                                         int
	Pending                                      *forgeRemoteIntent
}

func githubDiscussionScope(endpoint string, o transferOptions) string {
	return eventID([]byte(strings.Join([]string{"github-discussion-export/1", endpoint, o.project, o.id}, "\x00")))
}
func githubDiscussionRoot(s forgeResourceSnapshot) (forge.Node, map[string]forge.Node, error) {
	nodes := map[string]forge.Node{}
	root := forge.Node{}
	for _, n := range s.Graph.Nodes {
		nodes[n.Record.ID] = n
		if n.Record.ID == s.NativeID && n.Discussion != nil {
			root = n
		}
	}
	if root.Discussion == nil {
		return root, nil, errors.New("native discussion snapshot lacks root")
	}
	for _, n := range nodes {
		if n.Reply != nil {
			if n.Reply.Discussion.ID != root.Record.ID {
				return root, nil, errors.New("native reply has wrong discussion")
			}
			if n.Reply.Parent.ID != "" {
				p, ok := nodes[n.Reply.Parent.ID]
				if !ok || p.Reply == nil || p.Reply.Parent.ID != "" {
					return root, nil, errors.New("GitHub supports replies to top-level comments only; map native nested replies manually")
				}
			}
		}
	}
	a := root.Discussion.AcceptedReply
	if a.ID != "" {
		r, ok := nodes[a.ID]
		if !ok || r.Reply == nil {
			return root, nil, errors.New("native accepted reply inaccessible")
		}
		if r.Record.Revision != a.Revision {
			return root, nil, errors.New("accepted reply pins historical content; GitHub answers follow mutable comments; explicitly reconcile answer before export")
		}
	}
	return root, nodes, nil
}
func githubDiscussionDesired(n forge.Node, endpoint, category string, same bool) githubDiscussionProjection {
	p := githubDiscussionProjection{}
	if n.Discussion != nil {
		p.Title = n.Discussion.Title
		p.CategoryID = category
		p.Status = n.Discussion.Status
		p.Body = n.Discussion.Body
	} else {
		p.Body = n.Reply.Body
		p.ParentID = n.Reply.Parent.ID
	}
	if !same {
		p.Body = githubDiscussionRender(n, endpoint)
	}
	return p
}
func githubDiscussionEqual(a, b githubDiscussionProjection) bool {
	a.ID = ""
	b.ID = ""
	a.RepositoryID = ""
	b.RepositoryID = ""
	raw, _ := json.Marshal(a)
	other, _ := json.Marshal(b)
	return string(raw) == string(other)
}
func githubDiscussionNativeEqual(a, b forge.Node) bool {
	a.Metadata = nil
	b.Metadata = nil
	a.Record = forge.Record{}
	b.Record = forge.Record{}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func githubDiscussionSource(n forge.Node, endpoint, sourceRepositoryID, destinationRepositoryID string) (forgeResourceSource, forge.Node, bool, error) {
	var empty forgeResourceSource
	if n.Metadata[forgeResourceSourceKey] == "" {
		return empty, forge.Node{}, false, nil
	}
	source, _, version, err := forgeImportedResourceBaseline(n.Record.ID)
	if err != nil {
		return empty, forge.Node{}, false, err
	}
	if source.Provider != "github" || source.Endpoint != endpoint || source.Kind != n.Kind || sourceRepositoryID == "" || sourceRepositoryID != destinationRepositoryID {
		return source, forge.Node{}, false, nil
	}
	v, _, err := forgeResourceStateParts(n.Kind, version.State)
	if err != nil {
		return source, forge.Node{}, false, err
	}
	old := n
	switch s := v.(type) {
	case *DiscussionState:
		old.Discussion = &forge.Discussion{Title: s.Title, Body: s.Body, Category: s.Category, Status: s.Status}
		if s.AcceptedReplyRevision != "" {
			events, _, e := collectIssueEvents()
			if e != nil {
				return source, old, false, e
			}
			for _, event := range events {
				if event.ID == s.AcceptedReplyRevision {
					old.Discussion.AcceptedReply = forge.Ref{Kind: ResourceDiscussionReply, ID: resourceRootID(event), Revision: event.ID}
					break
				}
			}
			if old.Discussion.AcceptedReply.ID == "" {
				return source, old, false, errors.New("signed source answer revision unavailable")
			}
		}
	case *DiscussionReplyState:
		old.Reply = &forge.Reply{Discussion: forge.Ref{Kind: ResourceDiscussion, ID: s.DiscussionID}, Body: s.Body}
		if s.ReplyTo != "" {
			old.Reply.Parent = forge.Ref{Kind: ResourceDiscussionReply, ID: s.ReplyTo}
		}
	}
	return source, old, true, nil
}
func githubDiscussionOrdered(root forge.Node, nodes map[string]forge.Node) []forge.Node {
	ids := []string{}
	for id, n := range nodes {
		if n.Reply != nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := nodes[ids[i]], nodes[ids[j]]
		if (a.Reply.Parent.ID == "") != (b.Reply.Parent.ID == "") {
			return a.Reply.Parent.ID == ""
		}
		return ids[i] < ids[j]
	})
	result := []forge.Node{root}
	for _, id := range ids {
		result = append(result, nodes[id])
	}
	return result
}
func githubDiscussionRequireLosses(d githubapi.Discussion, o transferOptions) error {
	if len(githubDiscussionLosses(d)) > 0 && o.mappings["losses"] != "acknowledge" {
		return errors.New("destination has poll, lock, reactions or inaccessible/minimized content; inspect it and explicitly --map losses=acknowledge before migration")
	}
	return nil
}
func githubDiscussionCategory(ctx context.Context, g *githubForge, o transferOptions, root forge.Node) (githubapi.DiscussionRepository, githubapi.DiscussionCategory, error) {
	repo, err := g.client.GetDiscussionRepository(ctx, o.project)
	if err != nil {
		return repo, githubapi.DiscussionCategory{}, err
	}
	if !repo.HasDiscussionsEnabled {
		return repo, githubapi.DiscussionCategory{}, errors.New("destination discussions are disabled")
	}
	if o.mappings["category"] == "" {
		return repo, githubapi.DiscussionCategory{}, errors.New("discussion export requires --map category=EXISTING_CATEGORY_ID")
	}
	categories, err := g.client.ListDiscussionCategories(ctx, o.project)
	if err != nil {
		return repo, githubapi.DiscussionCategory{}, err
	}
	for _, c := range categories {
		if c.ID == o.mappings["category"] {
			if root.Discussion.AcceptedReply.ID != "" && !c.IsAnswerable {
				return repo, c, errors.New("destination category does not accept answers")
			}
			return repo, c, nil
		}
	}
	return repo, githubapi.DiscussionCategory{}, errors.New("mapped destination category is inaccessible")
}
func githubDiscussionDiscover(ctx context.Context, g *githubForge, o transferOptions, cp *githubDiscussionCheckpoint, root forge.Node, repoID string) (*githubapi.Discussion, error) {
	binding := cp.Bindings[root.Record.ID]
	source, _, same, err := githubDiscussionSource(root, g.Endpoint(), cp.SourceRepositoryID, cp.RepositoryID)
	if err != nil {
		return nil, err
	}
	if same {
		if binding.RemoteID != "" && (binding.RemoteID != source.Record.ID || !binding.SameSource) {
			return nil, errors.New("checkpoint source association differs from signed native source")
		}
		binding = githubDiscussionBinding{source.Record.ID, true}
	}
	if binding.RemoteID != "" {
		d, err := g.client.GetDiscussionByID(ctx, binding.RemoteID)
		if err != nil {
			return nil, err
		}
		if d.RepositoryID != repoID {
			return nil, errors.New("mapped discussion belongs to another repository")
		}
		if !same {
			p, ok := githubDiscussionParseProof(d.Body)
			if !ok || p.Endpoint != g.Endpoint() || p.NativeID != root.Record.ID {
				return nil, errors.New("external discussion association missing; preserve edits and reconcile manually")
			}
		}
		cp.Bindings[root.Record.ID] = binding
		return &d, nil
	}
	list, err := g.client.ListDiscussions(ctx, o.project)
	if err != nil {
		return nil, err
	}
	var found *githubapi.Discussion
	for _, d := range list {
		p, ok := githubDiscussionParseProof(d.Body)
		if !ok || p.Endpoint != g.Endpoint() || p.NativeID != root.Record.ID {
			continue
		}
		if found != nil {
			return nil, errors.New("multiple external discussions claim native identity")
		}
		if d.Body != githubDiscussionRender(root, g.Endpoint()) || d.Title != root.Discussion.Title {
			return nil, errors.New("external marker is not exact current native content; reconcile manually")
		}
		full, err := g.client.GetDiscussion(ctx, o.project, d.Number)
		if err != nil {
			return nil, err
		}
		if full.ID != d.ID || full.RepositoryID != repoID {
			return nil, errors.New("external discovery identity changed")
		}
		found = &full
	}
	if found != nil {
		cp.Bindings[root.Record.ID] = githubDiscussionBinding{RemoteID: found.ID}
	}
	return found, nil
}
func githubDiscussionBuildPlan(ctx context.Context, g *githubForge, o transferOptions, cp *githubDiscussionCheckpoint, repo githubapi.DiscussionRepository, root forge.Node, nodes map[string]forge.Node) error {
	current, err := githubDiscussionDiscover(ctx, g, o, cp, root, repo.ID)
	if err != nil {
		return err
	}
	remote := map[string]githubDiscussionProjection{}

	if current != nil {
		if err := githubDiscussionRequireLosses(*current, o); err != nil {
			return err
		}
		remote[current.ID] = githubDiscussionRootProjection(*current)
		for _, r := range current.Comments {
			remote[r.ID] = githubDiscussionReplyProjection(r)
		}
	}
	previous := map[string]forge.Node{}
	if cp.Previous != nil {
		if err := forgeAuthenticateResourceSnapshot(*cp.Previous); err != nil {
			return err
		}
		_, previous, err = githubDiscussionRoot(*cp.Previous)
		if err != nil {
			return err
		}
	}
	// Recover every signed source association before resolving root answer and
	// parent identities. Newly exported marker associations are collected too.
	for id, n := range nodes {
		source, _, same, e := githubDiscussionSource(n, g.Endpoint(), cp.SourceRepositoryID, cp.RepositoryID)
		if e != nil {
			return e
		}
		if same {
			b := cp.Bindings[id]
			if b.RemoteID != "" && (b.RemoteID != source.Record.ID || !b.SameSource) {
				return errors.New("saved source binding differs from signed witness")
			}
			cp.Bindings[id] = githubDiscussionBinding{source.Record.ID, true}
			continue
		}
		if current != nil && n.Reply != nil {
			for _, r := range current.Comments {
				proof, ok := githubDiscussionParseProof(r.Body)
				if !ok || proof.NativeID != id || proof.Endpoint != g.Endpoint() {
					continue
				}
				b := cp.Bindings[id]
				if b.RemoteID != "" && b.RemoteID != r.ID {
					return errors.New("ambiguous exported reply association")
				}
				cp.Bindings[id] = githubDiscussionBinding{RemoteID: r.ID}
			}
		}
	}
	preserveRoot := false
	cp.Actions = nil
	cp.Next = 0
	ordered := githubDiscussionOrdered(root, nodes)
	for _, n := range ordered {
		binding := cp.Bindings[n.Record.ID]
		source, old, same, err := githubDiscussionSource(n, g.Endpoint(), cp.SourceRepositoryID, cp.RepositoryID)
		if err != nil {
			return err
		}
		if same {
			if binding.RemoteID != "" && (binding.RemoteID != source.Record.ID || !binding.SameSource) {
				return errors.New("reply checkpoint differs from signed source association")
			}
			binding = githubDiscussionBinding{source.Record.ID, true}
			cp.Bindings[n.Record.ID] = binding
		}
		desired := githubDiscussionDesired(n, g.Endpoint(), cp.CategoryID, binding.SameSource)
		if n.Reply != nil && n.Reply.Parent.ID != "" {
			desired.ParentID = cp.Bindings[n.Reply.Parent.ID].RemoteID
		}
		if n.Reply != nil && binding.RemoteID == "" && current != nil {
			for _, r := range current.Comments {
				p, ok := githubDiscussionParseProof(r.Body)
				if !ok || p.NativeID != n.Record.ID || p.Endpoint != g.Endpoint() {
					continue
				}
				if binding.RemoteID != "" {
					return errors.New("multiple replies claim native identity")
				}
				if r.Body != desired.Body || r.ParentID != desired.ParentID {
					return errors.New("reply marker does not match exact current native content")
				}
				binding.RemoteID = r.ID
			}
			if binding.RemoteID != "" {
				cp.Bindings[n.Record.ID] = binding
			}
		}
		got, exists := remote[binding.RemoteID]
		if binding.RemoteID != "" && !exists {
			if same && n.Reply != nil && githubDiscussionNativeEqual(n, old) {
				continue
			}
			return errors.New("mapped remote reply deleted or inaccessible; refusing resurrection")
		}
		if binding.RemoteID == "" {
			action := "create_reply"
			if n.Discussion != nil {
				action = "create_root"
				desired.RepositoryID = repo.ID
				desired.Status = "open"
				desired.AnswerID = ""
			}
			cp.Actions = append(cp.Actions, githubDiscussionAction{n.Record.ID, action, desired})
			continue
		}
		if n.Reply != nil {
			if got.RepositoryID != cp.Bindings[root.Record.ID].RemoteID || got.ParentID != desired.ParentID {
				return errors.New("remote reply parent/thread association differs")
			}
			if !same {
				p, ok := githubDiscussionParseProof(got.Body)
				if !ok || p.NativeID != n.Record.ID || p.Endpoint != g.Endpoint() {
					return errors.New("reply provenance marker lost; external edit preserved, reconcile manually")
				}
			}
		}
		baseline, hasBaseline := cp.Baselines[n.Record.ID]
		oldNative, hasPrevious := previous[n.Record.ID]
		if hasBaseline && hasPrevious {
			expected := githubDiscussionDesired(oldNative, g.Endpoint(), cp.CategoryID, binding.SameSource)
			if oldNative.Reply != nil && oldNative.Reply.Parent.ID != "" {
				expected.ParentID = cp.Bindings[oldNative.Reply.Parent.ID].RemoteID
			}
			if oldNative.Discussion != nil {
				expected.AnswerID = cp.Bindings[oldNative.Discussion.AcceptedReply.ID].RemoteID
			}
			if !githubDiscussionEqual(baseline, expected) {
				return errors.New("checkpoint baseline differs from authenticated prior native snapshot")
			}
		} else if same {
			baseline = githubDiscussionDesired(old, g.Endpoint(), cp.CategoryID, true)
			if n.Reply != nil {
				baseline.ParentID = got.ParentID
			} else {
				var claims struct {
					Category                                                              githubapi.DiscussionCategory
					Locked                                                                bool
					ActiveLockReason, PollID, StateReason, AnswerChosenAt, AnswerChosenBy string
					ReactionCount, UpvoteCount                                            int
					EditCount                                                             *int
				}
				_, _, v, e := forgeImportedResourceBaseline(n.Record.ID)
				if e != nil {
					return e
				}
				_, m, e := forgeResourceStateParts(n.Kind, v.State)
				if e != nil {
					return e
				}
				if forgeStrictJSON([]byte(m["github/discussion-claims"]), &claims) != nil {
					return errors.New("imported category witness unavailable")
				}
				baseline.CategoryID = claims.Category.ID
				if old.Discussion.AcceptedReply.ID != "" {
					baseline.AnswerID = cp.Bindings[old.Discussion.AcceptedReply.ID].RemoteID
					if baseline.AnswerID == "" {
						return errors.New("source accepted answer witness inaccessible")
					}
				}
			}
			hasBaseline = true
		} else {
			baseline = desired
			if n.Discussion != nil {
				baseline.AnswerID = cp.Bindings[root.Discussion.AcceptedReply.ID].RemoteID
			}
			hasBaseline = true
		}
		compareDesired := desired
		if n.Discussion != nil {
			compareDesired.AnswerID = cp.Bindings[root.Discussion.AcceptedReply.ID].RemoteID
		}
		if !githubDiscussionEqual(got, baseline) {
			unchanged := hasPrevious && githubDiscussionNativeEqual(n, oldNative) || same && githubDiscussionNativeEqual(n, old)
			if unchanged {
				if n.Discussion != nil {
					preserveRoot = true
				}
				continue
			}
			return errors.New("competing native/external discussion edits; no overwrite performed")
		}
		cp.Baselines[n.Record.ID] = got
		if n.Discussion != nil {
			compareDesired.Status = got.Status
			compareDesired.AnswerID = got.AnswerID
		}
		if !githubDiscussionEqual(got, compareDesired) {
			action := "update_reply"
			if n.Discussion != nil {
				action = "update_root"
			}
			cp.Actions = append(cp.Actions, githubDiscussionAction{n.Record.ID, action, compareDesired})
		}
	}
	// Answer and lifecycle writes follow every content mutation. The historical
	// accepted content check above guarantees this maps the exact native revision.
	if preserveRoot {
		return nil
	}
	desiredAnswer := root.Discussion.AcceptedReply.ID
	oldAnswer := ""
	oldStatus := "open"
	if current != nil {
		oldAnswer = current.AnswerID
		if current.Closed {
			oldStatus = "closed"
		}
	}
	if oldAnswer != "" && oldAnswer != cp.Bindings[desiredAnswer].RemoteID {
		cp.Actions = append(cp.Actions, githubDiscussionAction{root.Record.ID, "unmark_answer", githubDiscussionProjection{AnswerID: oldAnswer}})
	}
	if desiredAnswer != "" && (cp.Bindings[desiredAnswer].RemoteID == "" || oldAnswer != cp.Bindings[desiredAnswer].RemoteID) {
		cp.Actions = append(cp.Actions, githubDiscussionAction{root.Record.ID, "mark_answer", githubDiscussionProjection{AnswerID: desiredAnswer}})
	}
	if root.Discussion.Status != oldStatus {
		cp.Actions = append(cp.Actions, githubDiscussionAction{root.Record.ID, "set_status", githubDiscussionProjection{Status: root.Discussion.Status}})
	}
	return nil
}

func githubDiscussionIntent(cp *githubDiscussionCheckpoint, a githubDiscussionAction, nodes map[string]forge.Node) forgeRemoteIntent {
	n := nodes[a.NativeID]
	want := a.Desired
	binding := cp.Bindings[a.NativeID]
	root := cp.Bindings[cp.Snapshot.NativeID].RemoteID
	if n.Reply != nil {
		want.RepositoryID = root
		want.ParentID = cp.Bindings[n.Reply.Parent.ID].RemoteID
	}
	if a.Action == "mark_answer" {
		want.AnswerID = cp.Bindings[a.Desired.AnswerID].RemoteID
	}
	want.ID = binding.RemoteID
	if a.Action == "create_root" {
		want.ID = ""
	} else if a.Action == "create_reply" {
		want.ID = ""
	}
	if a.Action == "unmark_answer" || a.Action == "mark_answer" || a.Action == "set_status" {
		want.ID = root
	}
	raw, _ := json.Marshal(want)
	return forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: cp.Scope, Provider: "github", Endpoint: cp.Endpoint, Kind: ResourceDiscussion, Action: a.Action, NativeID: a.NativeID, NativeRevision: n.Record.Revision, TargetID: binding.RemoteID, Payload: raw, PayloadDigest: eventID(raw), Phase: "prepared"}
}
func githubDiscussionResultProjection(r forgeRemoteResult) (githubDiscussionProjection, error) {
	var p githubDiscussionProjection
	err := forgeStrictJSON(r.Payload, &p)
	if err != nil || p.ID != r.ID {
		return p, errors.New("invalid discussion mutation projection")
	}
	return p, nil
}
func githubDiscussionValidateResult(i forgeRemoteIntent, r forgeRemoteResult) error {
	var want githubDiscussionProjection
	if forgeStrictJSON(i.Payload, &want) != nil {
		return errors.New("invalid discussion intent payload")
	}
	got, err := githubDiscussionResultProjection(r)
	if err != nil {
		return err
	}
	if want.ID != "" && got.ID != want.ID {
		return errors.New("discussion mutation identity mismatch")
	}
	switch i.Action {
	case "create_root", "update_root":
		if got.Title != want.Title || got.Body != want.Body || got.CategoryID != want.CategoryID || (want.RepositoryID != "" && got.RepositoryID != want.RepositoryID) {
			return errors.New("discussion content mutation differs")
		}
	case "create_reply", "update_reply":
		if got.Body != want.Body || got.ParentID != want.ParentID || got.RepositoryID != want.RepositoryID {
			return errors.New("reply content mutation differs")
		}
	case "mark_answer", "unmark_answer":
		if i.Action == "unmark_answer" {
			want.AnswerID = ""
		}
		if got.AnswerID != want.AnswerID {
			return errors.New("answer mutation differs")
		}
	case "set_status":
		if got.Status != want.Status {
			return errors.New("discussion status mutation differs")
		}
	default:
		return errors.New("unsupported discussion mutation")
	}
	return nil
}
func githubDiscussionWrite(ctx context.Context, g *githubForge, i forgeRemoteIntent) (forgeRemoteResult, error) {
	var p githubDiscussionProjection
	if forgeStrictJSON(i.Payload, &p) != nil {
		return forgeRemoteResult{}, errors.New("invalid discussion action payload")
	}
	var d githubapi.Discussion
	var r githubapi.DiscussionComment
	var err error
	switch i.Action {
	case "create_root":
		d, err = g.client.CreateDiscussion(ctx, p.RepositoryID, githubapi.DiscussionInput{Title: p.Title, Body: p.Body, CategoryID: p.CategoryID})
	case "update_root":
		d, err = g.client.UpdateDiscussion(ctx, p.ID, githubapi.DiscussionInput{Title: p.Title, Body: p.Body, CategoryID: p.CategoryID})
	case "create_reply":
		r, err = g.client.AddDiscussionComment(ctx, p.RepositoryID, p.ParentID, p.Body)
	case "update_reply":
		r, err = g.client.UpdateDiscussionComment(ctx, p.ID, p.Body)
	case "mark_answer":
		d, err = g.client.MarkDiscussionCommentAsAnswer(ctx, p.ID, p.AnswerID)
	case "unmark_answer":
		d, err = g.client.UnmarkDiscussionCommentAsAnswer(ctx, p.ID, p.AnswerID)
	case "set_status":
		if p.Status == "closed" {
			d, err = g.client.CloseDiscussion(ctx, p.ID, "RESOLVED")
		} else {
			d, err = g.client.ReopenDiscussion(ctx, p.ID)
		}
	default:
		return forgeRemoteResult{}, errors.New("unsupported discussion action")
	}
	if err != nil {
		return forgeRemoteResult{}, err
	}
	if r.ID != "" {
		return forgeRemoteObserved(r.ID, "", "", githubDiscussionReplyProjection(r)), nil
	}
	return forgeRemoteObserved(d.ID, "", "", githubDiscussionRootProjection(d)), nil
}
func githubDiscussionReconcile(ctx context.Context, g *githubForge, o transferOptions, cp *githubDiscussionCheckpoint, i forgeRemoteIntent) (forgeRemoteResult, bool, error) {
	var want githubDiscussionProjection
	if forgeStrictJSON(i.Payload, &want) != nil {
		return forgeRemoteResult{}, false, errors.New("invalid pending discussion payload")
	}
	if i.Action == "create_root" {
		list, err := g.client.ListDiscussions(ctx, o.project)
		if err != nil {
			return forgeRemoteResult{}, false, err
		}
		var result forgeRemoteResult
		found := false
		for _, d := range list {
			candidate := forgeRemoteObserved(d.ID, "", "", githubDiscussionRootProjection(d))
			if githubDiscussionValidateResult(i, candidate) != nil {
				continue
			}
			if found {
				return result, false, errors.New("ambiguous exact discussion creation matches")
			}
			found = true
			result = candidate
		}
		return result, found, nil
	}
	root := cp.Bindings[cp.Snapshot.NativeID].RemoteID
	if root == "" {
		return forgeRemoteResult{}, false, errors.New("pending reply/root action lacks root association")
	}
	d, err := g.client.GetDiscussionByID(ctx, root)
	if err != nil {
		return forgeRemoteResult{}, false, err
	}
	repo, err := g.client.GetDiscussionRepository(ctx, o.project)
	if err != nil {
		return forgeRemoteResult{}, false, err
	}
	if d.RepositoryID != repo.ID {
		return forgeRemoteResult{}, false, errors.New("pending root belongs to another repository")
	}
	b := cp.Bindings[cp.Snapshot.NativeID]
	if !b.SameSource {
		proof, ok := githubDiscussionParseProof(d.Body)
		if !ok || proof.NativeID != cp.Snapshot.NativeID || proof.Endpoint != cp.Endpoint {
			return forgeRemoteResult{}, false, errors.New("pending root association missing")
		}
	}

	if i.Action == "create_reply" || i.Action == "update_reply" {
		found := false
		var result forgeRemoteResult
		for _, r := range d.Comments {
			candidate := forgeRemoteObserved(r.ID, "", "", githubDiscussionReplyProjection(r))
			if githubDiscussionValidateResult(i, candidate) != nil {
				continue
			}
			if found {
				return result, false, errors.New("ambiguous exact reply creation matches")
			}
			result = candidate
			found = true
		}
		return result, found, nil
	}
	result := forgeRemoteObserved(d.ID, "", "", githubDiscussionRootProjection(d))
	return result, githubDiscussionValidateResult(i, result) == nil, nil
}
func githubDiscussionBudget(cp githubDiscussionCheckpoint, nodes map[string]forge.Node) error {
	// Reserve the largest legal escaped opaque identity in EVERY future binding
	// and result, including the full frozen graph, action list and prior witness.
	worst := strings.Repeat("<", 1024)
	cp.Signer = strings.Repeat("a", 64)
	cp.SignerPublicKey = strings.Repeat("a", 43)
	cp.Signature = strings.Repeat("a", 86)
	cp.Bindings = map[string]githubDiscussionBinding{}
	cp.Baselines = map[string]githubDiscussionProjection{}
	for id, n := range nodes {
		cp.Bindings[id] = githubDiscussionBinding{worst, true}
		p := githubDiscussionDesired(n, cp.Endpoint, cp.CategoryID, false)
		p.ID = worst
		p.RepositoryID = worst
		p.ParentID = worst
		p.AnswerID = worst
		cp.Baselines[id] = p
	}
	max := 0
	for _, a := range cp.Actions {
		intent := githubDiscussionIntent(&cp, a, nodes)
		intent.TargetID = worst
		intent.Phase = "confirmed"
		intent.ResultID = worst
		intent.ResultRevision = worst
		p := githubDiscussionDesired(nodes[a.NativeID], cp.Endpoint, cp.CategoryID, false)
		p.ID = worst
		p.RepositoryID = worst
		p.ParentID = worst
		p.AnswerID = worst
		raw, _ := json.Marshal(p)
		intent.ResultPayload = raw
		intent.ResultDigest = eventID(raw)
		cp.Pending = &intent
		b, err := json.Marshal(cp)
		if err != nil {
			return err
		}
		if len(b) > max {
			max = len(b)
		}
	}
	if len(cp.Actions) == 0 {
		b, _ := json.Marshal(cp)
		max = len(b)
	}
	if max > forgeCheckpointLimit {
		return errors.New("whole discussion export checkpoint including all future bindings and pending results exceeds 8MiB; no provider writes performed")
	}
	return nil
}
func githubDiscussionCheckActions(cp *githubDiscussionCheckpoint, nodes map[string]forge.Node, repositoryID string) error {
	root := nodes[cp.Snapshot.NativeID]
	if cp.Next < 0 || cp.Next > len(cp.Actions) {
		return errors.New("invalid discussion recovery progress")
	}
	for _, a := range cp.Actions {
		n, ok := nodes[a.NativeID]
		if !ok {
			return errors.New("pending action outside authenticated native graph")
		}
		expected := githubDiscussionDesired(n, cp.Endpoint, cp.CategoryID, cp.Bindings[a.NativeID].SameSource)
		switch a.Action {
		case "create_root", "update_root":
			if a.Action == "create_root" && a.Desired.RepositoryID != repositoryID {
				return errors.New("saved destination differs from selected repository")
			}
			if n.Discussion == nil || a.Desired.Title != expected.Title || a.Desired.Body != expected.Body || a.Desired.CategoryID != expected.CategoryID {
				return errors.New("root plan differs from authenticated snapshot")
			}
		case "create_reply", "update_reply":
			if n.Reply == nil || a.Desired.Body != expected.Body {
				return errors.New("reply plan differs from authenticated snapshot")
			}
		case "mark_answer":
			if a.NativeID != root.Record.ID || a.Desired.AnswerID != root.Discussion.AcceptedReply.ID {
				return errors.New("answer plan differs from exact native pin")
			}
		case "unmark_answer":
			expected, err := githubDiscussionPriorAnswer(cp, nodes)
			if err != nil {
				return err
			}
			if a.NativeID != root.Record.ID || a.Desired.AnswerID == "" || a.Desired.AnswerID != expected {
				return errors.New("invalid unmark plan")
			}
		case "set_status":
			if a.NativeID != root.Record.ID || a.Desired.Status != root.Discussion.Status {
				return errors.New("status plan differs from native snapshot")
			}
		default:
			return errors.New("unsupported saved discussion action")
		}
	}
	if cp.Pending != nil {
		if cp.Next >= len(cp.Actions) {
			return errors.New("pending action beyond plan")
		}
		expected := githubDiscussionIntent(cp, cp.Actions[cp.Next], nodes)
		actual := *cp.Pending
		actual.Phase = "prepared"
		actual.ResultID = ""
		actual.ResultKey = ""
		actual.ResultRevision = ""
		actual.ResultPayload = nil
		actual.ResultDigest = ""
		x, _ := json.Marshal(expected)
		y, _ := json.Marshal(actual)
		if string(x) != string(y) {
			return errors.New("pending intent differs from authenticated discussion action")
		}
	}
	return nil
}
func githubExportDiscussion(ctx context.Context, p forge.Identity, o transferOptions, snapshot forgeResourceSnapshot) error {
	g, ok := p.(*githubForge)
	if !ok {
		return errors.New("GitHub discussion adapter requires its installed client")
	}
	if _, err := githubDiscussionMappings(forge.Selection{Mappings: o.mappings}, true); err != nil {
		return err
	}
	var identity *Identity
	if !o.dryRun {
		if os.Getenv("HN_GITHUB_TOKEN") == "" {
			return errors.New("GitHub discussion writes require HN_GITHUB_TOKEN")
		}
		var err error
		identity, err = loadIdentity()
		if err != nil {
			return errors.New("real discussion export requires an active native signing identity; select the original exporter identity to recover")
		}
		if err = validateIdentity(identity); err != nil {
			return err
		}
	}

	scope := githubDiscussionScope(g.Endpoint(), o)
	name := "discussion-export-" + strings.TrimPrefix(scope, "sha256:") + ".json"
	cp := githubDiscussionCheckpoint{}
	found, err := forgeLoadJSON(name, &cp)
	if err != nil {
		return err
	}
	if found {
		if err := githubDiscussionVerifyCheckpoint(name, cp, identity); err != nil {
			return err
		}
	}
	if found && (cp.Schema != "hn.github-discussion-export/1" || cp.Scope != scope || cp.Endpoint != g.Endpoint() || cp.Project != o.project || cp.CategoryID != o.mappings["category"]) {
		return errors.New("discussion checkpoint scope/category mismatch")
	}
	if !found {
		cp = githubDiscussionCheckpoint{Schema: "hn.github-discussion-export/1", Scope: scope, Endpoint: g.Endpoint(), Project: o.project, CategoryID: o.mappings["category"], Bindings: map[string]githubDiscussionBinding{}, Baselines: map[string]githubDiscussionProjection{}}
	}
	resuming := cp.Snapshot != nil
	if resuming && o.dryRun {
		return errors.New("exact discussion export is pending; resume before previewing newer state")
	}
	if !resuming {
		cp.Snapshot = &snapshot
	}
	if cp.Snapshot.NativeID != o.id || forgeAuthenticateResourceSnapshot(*cp.Snapshot) != nil {
		return errors.New("saved discussion snapshot differs from authenticated native history")
	}
	root, nodes, err := githubDiscussionRoot(*cp.Snapshot)
	if err != nil {
		return err
	}
	repo, category, err := githubDiscussionCategory(ctx, g, o, root)
	if err != nil {
		return err
	}
	cp.CategoryID = category.ID
	sourceRepositoryID, err := githubDiscussionSourceRepository(ctx, g, root)
	if err != nil {
		return err
	}
	if found && (cp.RepositoryID != repo.ID || cp.SourceRepositoryID != sourceRepositoryID) {
		return errors.New("stable source/destination repository identity changed; explicitly reconcile the signed export checkpoint")
	}
	cp.RepositoryID = repo.ID
	cp.SourceRepositoryID = sourceRepositoryID
	if !resuming {
		if err := githubDiscussionBuildPlan(ctx, g, o, &cp, repo, root, nodes); err != nil {
			return err
		}
	}
	if err := githubDiscussionCheckActions(&cp, nodes, repo.ID); err != nil {
		return err
	}
	if cp.Previous != nil {
		if err := forgeAuthenticateResourceSnapshot(*cp.Previous); err != nil {
			return err
		}
	}
	for id, b := range cp.Bindings {
		n, ok := nodes[id]
		if !ok {
			return errors.New("saved binding outside selected graph")
		}
		source, _, same, e := githubDiscussionSource(n, cp.Endpoint, cp.SourceRepositoryID, cp.RepositoryID)
		if e != nil {
			return e
		}
		if same != b.SameSource || same && b.RemoteID != source.Record.ID {
			return errors.New("saved binding differs from signed source")
		}
		if err := githubDiscussionCheckBaseline(&cp, id, nodes); err != nil {
			return err
		}
	}

	if err := githubDiscussionBudget(cp, nodes); err != nil {
		return err
	}
	report := forge.Plan{Resource: forge.ResourceDiscussion, Direction: "export", Nodes: len(nodes), Findings: []forge.Finding{{Status: forge.Mapped, Path: "category", Message: category.ID}, {Status: forge.Preserved, Path: "history", Message: "current text snapshots and source claims; source identity grants no provider authority"}, {Status: forge.Mapped, Path: "publication", Message: fmt.Sprintf("%d separate journaled provider mutations; no complete history, roles or moderation equivalence", len(cp.Actions))}}}
	if o.dryRun {
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	if len(cp.Actions) == 0 {
		return nil
	}
	save := func() error {
		if err := githubDiscussionSignCheckpoint(&cp, identity); err != nil {
			return err
		}
		return forgeStoreJSON(name, &cp)
	}
	if !resuming {
		if err := save(); err != nil {
			return err
		}
	}
	for cp.Next < len(cp.Actions) {
		a := cp.Actions[cp.Next]
		if cp.Pending == nil {
			i := githubDiscussionIntent(&cp, a, nodes)
			cp.Pending = &i
		}
		// Re-observe even a confirmed response left by a crash before progress
		// persistence; mutable checkpoint result bytes never authorize adoption.
		if cp.Pending.Phase == "confirmed" {
			cp.Pending.Phase = "in_flight"
		}
		if cp.Pending.Phase == "prepared" {
			if err := githubDiscussionBeforeWrite(ctx, g, o, &cp, *cp.Pending, nodes); err != nil {
				return err
			}
		}
		result, err := forgeExecuteRemoteIntent(ctx, cp.Pending, save, func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, error) {
			return githubDiscussionWrite(ctx, g, i)
		}, func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, bool, error) {
			return githubDiscussionReconcile(ctx, g, o, &cp, i)
		}, githubDiscussionValidateResult)
		if err != nil {
			return err
		}
		projection, err := githubDiscussionResultProjection(result)
		if err != nil {
			return err
		}
		b := cp.Bindings[a.NativeID]
		b.RemoteID = result.ID
		cp.Bindings[a.NativeID] = b
		cp.Baselines[a.NativeID] = projection
		cp.Next++
		cp.Pending = nil
		if err := save(); err != nil {
			return err
		}
	}
	cp.Previous = cp.Snapshot
	cp.Snapshot = nil
	cp.Actions = nil
	cp.Next = 0
	return save()
}

func githubDiscussionSourceCategory(n forge.Node) (string, error) {
	_, _, v, err := forgeImportedResourceBaseline(n.Record.ID)
	if err != nil {
		return "", err
	}
	_, m, err := forgeResourceStateParts(n.Kind, v.State)
	if err != nil {
		return "", err
	}
	var claims struct {
		Category                                                              githubapi.DiscussionCategory
		Locked                                                                bool
		ActiveLockReason, PollID, StateReason, AnswerChosenAt, AnswerChosenBy string
		ReactionCount, UpvoteCount                                            int
		EditCount                                                             *int
	}
	if forgeStrictJSON([]byte(m["github/discussion-claims"]), &claims) != nil || claims.Category.ID == "" {
		return "", errors.New("signed imported category claim missing")
	}
	return claims.Category.ID, nil
}

// Reconstruct the overwrite baseline from authenticated native history and
// completed deterministic steps. Editable checkpoint baselines cannot authorize
// overwriting an unrelated external edit.
func githubDiscussionCheckBaseline(cp *githubDiscussionCheckpoint, id string, nodes map[string]forge.Node) error {
	baseline, exists := cp.Baselines[id]
	if !exists {
		return nil
	}
	n := nodes[id]
	b := cp.Bindings[id]
	expected := githubDiscussionProjection{}
	known := false
	if cp.Previous != nil {
		for _, old := range cp.Previous.Graph.Nodes {
			if old.Record.ID == id {
				expected = githubDiscussionDesired(old, cp.Endpoint, cp.CategoryID, b.SameSource)
				if old.Reply != nil {
					expected.ParentID = cp.Bindings[old.Reply.Parent.ID].RemoteID
				} else {
					expected.AnswerID = cp.Bindings[old.Discussion.AcceptedReply.ID].RemoteID
				}
				known = true
			}
		}
	}
	if !known {
		source, old, same, err := githubDiscussionSource(n, cp.Endpoint, cp.SourceRepositoryID, cp.RepositoryID)
		if err != nil {
			return err
		}
		if b.SameSource != same || same && b.RemoteID != source.Record.ID {
			return errors.New("saved binding differs from signed source witness")
		}
		if same {
			expected = githubDiscussionDesired(old, cp.Endpoint, cp.CategoryID, true)
			if old.Reply != nil {
				expected.ParentID = cp.Bindings[old.Reply.Parent.ID].RemoteID
			} else {
				expected.CategoryID, err = githubDiscussionSourceCategory(n)
				if err != nil {
					return err
				}
				expected.AnswerID = cp.Bindings[old.Discussion.AcceptedReply.ID].RemoteID
			}
			known = true
		}
	}
	for k := 0; k < cp.Next; k++ {
		a := cp.Actions[k]
		if a.NativeID != id {
			continue
		}
		switch a.Action {
		case "create_root":
			expected = a.Desired
			expected.Status = "open"
			expected.AnswerID = ""
			known = true
		case "update_root":
			expected.Title = a.Desired.Title
			expected.Body = a.Desired.Body
			expected.CategoryID = a.Desired.CategoryID
			known = true
		case "create_reply", "update_reply":
			expected = a.Desired
			expected.ParentID = cp.Bindings[n.Reply.Parent.ID].RemoteID
			known = true
		case "mark_answer":
			expected.AnswerID = cp.Bindings[a.Desired.AnswerID].RemoteID
		case "unmark_answer":
			expected.AnswerID = ""
		case "set_status":
			expected.Status = a.Desired.Status
		}
	}
	if !known { // Exact discovery of an already-published current native graph.
		expected = githubDiscussionDesired(n, cp.Endpoint, cp.CategoryID, false)
		if n.Reply != nil {
			expected.ParentID = cp.Bindings[n.Reply.Parent.ID].RemoteID
		} else {
			expected.AnswerID = cp.Bindings[n.Discussion.AcceptedReply.ID].RemoteID
		}
	}
	if !githubDiscussionEqual(baseline, expected) {
		return errors.New("saved discussion baseline is not proven by native history/completed steps")
	}
	return nil
}
func githubDiscussionBeforeWrite(ctx context.Context, g *githubForge, o transferOptions, cp *githubDiscussionCheckpoint, i forgeRemoteIntent, nodes map[string]forge.Node) error {
	if cp.Previous != nil {
		if err := forgeAuthenticateResourceSnapshot(*cp.Previous); err != nil {
			return err
		}
	}
	for id, b := range cp.Bindings {
		n, ok := nodes[id]
		if !ok {
			return errors.New("saved binding outside selected native graph")
		}
		source, _, same, err := githubDiscussionSource(n, cp.Endpoint, cp.SourceRepositoryID, cp.RepositoryID)
		if err != nil {
			return err
		}
		if b.SameSource != same || same && b.RemoteID != source.Record.ID {
			return errors.New("saved binding differs from signed source witness")
		}
		if err := githubDiscussionCheckBaseline(cp, id, nodes); err != nil {
			return err
		}
	}
	if i.Action == "create_root" {
		return nil
	}
	rootID := cp.Bindings[cp.Snapshot.NativeID].RemoteID
	d, err := g.client.GetDiscussionByID(ctx, rootID)
	if err != nil {
		return err
	}
	if d.RepositoryID != cp.RepositoryID {
		return errors.New("external discussion moved to another repository; reconcile before mutation")
	}
	if err := githubDiscussionRequireLosses(d, o); err != nil {
		return err
	}
	rootBinding := cp.Bindings[cp.Snapshot.NativeID]
	if !rootBinding.SameSource {
		proof, ok := githubDiscussionParseProof(d.Body)
		if !ok || proof.NativeID != cp.Snapshot.NativeID || proof.Endpoint != cp.Endpoint {
			return errors.New("remote root association changed before mutation")
		}
	}
	if i.Action == "create_reply" {
		if base, ok := cp.Baselines[cp.Snapshot.NativeID]; ok && !githubDiscussionEqual(base, githubDiscussionRootProjection(d)) {
			return errors.New("external discussion changed before reply publication")
		}
		return nil
	}
	got := githubDiscussionRootProjection(d)
	if i.Action == "update_reply" {
		found := false
		for _, r := range d.Comments {
			if r.ID == cp.Bindings[i.NativeID].RemoteID {
				got = githubDiscussionReplyProjection(r)
				found = true
			}
		}
		if !found {
			return errors.New("remote reply disappeared before update")
		}
	}
	if baseline, ok := cp.Baselines[i.NativeID]; !ok || !githubDiscussionEqual(baseline, got) {
		return errors.New("external content changed after preflight; no overwrite performed")
	}
	return nil
}

func githubDiscussionPriorAnswer(cp *githubDiscussionCheckpoint, nodes map[string]forge.Node) (string, error) {
	if cp.Previous != nil {
		for _, n := range cp.Previous.Graph.Nodes {
			if n.Record.ID == cp.Snapshot.NativeID {
				return cp.Bindings[n.Discussion.AcceptedReply.ID].RemoteID, nil
			}
		}
	}
	_, old, same, err := githubDiscussionSource(nodes[cp.Snapshot.NativeID], cp.Endpoint, cp.SourceRepositoryID, cp.RepositoryID)
	if err != nil {
		return "", err
	}
	if same {
		return cp.Bindings[old.Discussion.AcceptedReply.ID].RemoteID, nil
	}
	return "", nil
}

func githubDiscussionNativePublication(proof githubDiscussionProof, body string) (bool, error) {
	events, _, err := collectIssueEvents()
	if err != nil {
		return false, err
	}
	var version, opening StoredEvent
	found := false
	for _, e := range events {
		if e.ID == proof.Revision {
			version = e
			found = true
		}
		if e.ID == proof.NativeID {
			opening = e
		}
	}
	if !found {
		return false, nil
	}
	if resourceRootID(version) != proof.NativeID || (resourceFamily(version.Event.Kind) != ResourceDiscussion && resourceFamily(version.Event.Kind) != ResourceDiscussionReply) {
		return false, nil
	}
	raw, err := CanonicalResourceState(resourceFamily(version.Event.Kind), []byte(version.Event.Body))
	if err != nil {
		return false, err
	}
	node, err := forgeNodeFromNative(&ResourceView{ID: proof.NativeID, Family: resourceFamily(version.Event.Kind), Heads: []ResourceHead{{ID: version.ID, Actor: version.Event.Actor, State: raw}}, State: raw, History: []StoredEvent{opening, version}})
	if err != nil {
		return false, err
	}
	return githubDiscussionRender(node, proof.Endpoint) == body, nil
}

const githubDiscussionCheckpointDomain = "hn.github-discussion-checkpoint-signature/1\x00"

func githubDiscussionCheckpointBytes(cp githubDiscussionCheckpoint) ([]byte, error) {
	cp.Signature = ""
	raw, err := json.Marshal(cp)
	if err != nil || len(raw) > forgeCheckpointLimit {
		return nil, errors.New("discussion checkpoint signature payload exceeds supported bounds")
	}
	return append([]byte(githubDiscussionCheckpointDomain), raw...), nil
}
func githubDiscussionSignCheckpoint(cp *githubDiscussionCheckpoint, identity *Identity) error {
	if identity == nil {
		return errors.New("real discussion export requires the selected native signing identity")
	}
	cp.Signer = identity.Actor
	cp.SignerPublicKey = identity.PublicKey
	payload, err := githubDiscussionCheckpointBytes(*cp)
	if err != nil {
		return err
	}
	key, err := identity.privateKey()
	if err != nil {
		return err
	}
	cp.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, payload))
	return nil
}
func githubDiscussionVerifyCheckpoint(name string, cp githubDiscussionCheckpoint, identity *Identity) error {
	// Only producer-canonical JSON is admitted, so aliases, unknown fields,
	// duplicate keys and trailing data cannot alter the interpreted envelope.
	path, err := forgeCheckpointPath(name)
	if err != nil {
		return err
	}
	raw, err := readPrivateFileBounded(path, forgeCheckpointLimit)
	if err != nil {
		return errors.New("discussion checkpoint cannot be authenticated")
	}
	canonical, err := json.Marshal(cp)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("discussion checkpoint is not canonical signed JSON; explicit recovery required")
	}
	public, err := base64.RawStdEncoding.DecodeString(cp.SignerPublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || base64.RawStdEncoding.EncodeToString(public) != cp.SignerPublicKey || actorForPublicKey(public) != cp.Signer {
		return errors.New("discussion checkpoint signer is invalid; explicit recovery required")
	}
	if identity != nil && (cp.Signer != identity.Actor || cp.SignerPublicKey != identity.PublicKey) {
		return errors.New("discussion checkpoint belongs to a different native identity; select the original exporter identity for explicit recovery")
	}
	signature, err := base64.RawStdEncoding.DecodeString(cp.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || base64.RawStdEncoding.EncodeToString(signature) != cp.Signature {
		return errors.New("discussion checkpoint signature is missing or invalid; explicit recovery required")
	}
	payload, err := githubDiscussionCheckpointBytes(cp)
	if err != nil {
		return err
	}
	if !ed25519.Verify(public, payload, signature) {
		return errors.New("discussion checkpoint signature does not match its complete recovery state; explicit recovery required")
	}
	return nil
}
func githubDiscussionSourceRepository(ctx context.Context, g *githubForge, root forge.Node) (string, error) {
	if root.Metadata[forgeResourceSourceKey] == "" {
		return "", nil
	}
	source, _, _, err := forgeImportedResourceBaseline(root.Record.ID)
	if err != nil {
		return "", err
	}
	if source.Provider != "github" || source.Endpoint != g.Endpoint() || source.Kind != ResourceDiscussion {
		return "", nil
	}
	// owner/name is a lookup hint. Resolve the signed immutable discussion ID
	// before deciding whether this is a return export or a deliberate new copy.
	original, err := g.client.GetDiscussionByID(ctx, source.Record.ID)
	if err != nil {
		return "", errors.New("cannot establish the stable source discussion/repository identity; explicitly reconcile before copying")
	}
	if original.ID != source.Record.ID || original.RepositoryID == "" {
		return "", errors.New("source discussion identity changed; explicit recovery required")
	}
	return original.RepositoryID, nil
}
