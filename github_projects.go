//go:build !nogithub

package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"

	"hubnot/internal/forge"
	"hubnot/internal/githubapi"
)

func init() {
	r := forgeProviders["github"]
	r.Capabilities = append(r.Capabilities, forge.Capability{Resource: forge.ResourceProject, Import: true, Export: true, MappedFields: []string{"explicit user/org board selectors", "title/readme/closed state", "text/number/date/single-select fields", "draft or explicitly mapped content items", "values/order/archive", "table/board view names/layouts", "source claims"}, Unsupported: []string{"iteration/multi-select and content properties require explicit per-finding omission", "view filters/grouping/sort/visible columns require explicit omission", "project automation, linked teams/repositories and permissions are not migrated", "schema deletion/type/option changes require manual migration", "remote API has no atomic compare-and-swap; single publisher required", "real export requires active native identity for signed private recovery; identity changes require selecting original signer or manual reconciliation"}})
	forgeProviders["github"] = r
	forgeResourceExporters[forgeResourceRoute{"github", forge.ResourceProject}] = githubExportProject
}
func githubProjectID(id string) string        { return strings.TrimPrefix(eventID([]byte(id)), "sha256:") }
func githubProjectOmitKey(path string) string { return "omit." + githubProjectID(path) }
func githubProjectGraph(p githubapi.ProjectV2, s forge.Selection) (forge.Graph, error) {
	g := forge.Graph{Nodes: []forge.Node{}, Findings: []forge.Finding{}}
	used := map[string]bool{}
	omit := func(path, message string) {
		key := githubProjectOmitKey(path)
		status := forge.RequiresMapping
		if s.Mappings[key] == "true" {
			status = forge.Mapped
			used[key] = true
			message = "explicitly omitted: " + message
		} else {
			message += "; acknowledge this loss with --map " + key + "=true"
		}
		g.Findings = append(g.Findings, forge.Finding{Status: status, Path: path, Message: message})
	}
	root := forge.Node{Kind: ResourceProject, Record: forge.Record{ID: p.ID, Key: strconv.Itoa(p.Number), Revision: p.UpdatedAt, URL: p.URL, Author: p.Creator, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}, Project: &forge.Project{Title: p.Title, Body: p.Readme, Archived: p.Closed}}
	if p.Workflows > 0 {
		omit("automation", "project automation trigger/action rules are not available for equivalent migration")
	}
	if p.Repositories > 0 || p.Teams > 0 || p.Template || p.Public || p.ShortDescription != "" {
		omit("settings", "linked repositories/teams, template/public visibility and short description do not become native permissions or configuration")
	}
	fields := map[string]forge.Field{}
	ignoredTitle := map[string]bool{}
	for _, f := range p.Fields {
		typ := map[string]string{"TEXT": "text", "NUMBER": "number", "DATE": "date", "SINGLE_SELECT": "single_select"}[f.DataType]
		if typ == "" {
			if f.DataType == "TITLE" {
				ignoredTitle[f.ID] = true
			}
			if f.DataType != "TITLE" {
				omit("field:"+f.ID, "unsupported field "+f.Name+" ("+f.DataType+")")
			}
			continue
		}
		nf := forge.Field{ID: githubProjectID(f.ID), Name: f.Name, Type: typ}
		style := false
		for _, o := range f.Options {
			if o.Color != "" && o.Color != "GRAY" || o.Description != "" {
				style = true
			}
		}
		if style {
			omit("field-style:"+f.ID, "single-select option colors/descriptions have no native equivalent")
		}
		for _, o := range f.Options {
			nf.Options = append(nf.Options, forge.Option{ID: githubProjectID(o.ID), Name: o.Name})
		}
		root.Project.Fields = append(root.Project.Fields, nf)
		fields[f.ID] = nf
	}
	for _, v := range p.Views {
		layout := map[string]string{"TABLE_LAYOUT": "table", "BOARD_LAYOUT": "board"}[v.Layout]
		if layout == "" {
			omit("view:"+v.ID, "unsupported view layout "+v.Layout)
			continue
		}
		root.Project.Views = append(root.Project.Views, forge.View{ID: githubProjectID(v.ID), Name: v.Name, Layout: layout})
		if v.Filter != "" || v.Groups > 0 || v.VerticalGroups > 0 || v.Sorts > 0 || v.VisibleFields > 0 {
			omit("view-config:"+v.ID, "view filter/group/sort/visible-field configuration has no complete native equivalent")
		}
	}
	g.Nodes = append(g.Nodes, root)
	for index, item := range p.Items {
		n := forge.Node{Kind: ResourceProjectItem, Record: forge.Record{ID: item.ID, Key: item.ID, Revision: item.UpdatedAt, Author: item.Creator, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}, Item: &forge.ProjectItem{Project: forge.Ref{Kind: ResourceProject, ID: p.ID, Revision: p.UpdatedAt}, Position: strconv.Itoa(index + 1), Archived: item.Archived, Values: map[string]forge.Value{}}}
		c := item.Content
		if c == nil {
			omit("content:"+item.ID, "inaccessible item content cannot be reconstructed")
			continue
		}
		switch c.Type {
		case "DraftIssue":
			n.Item.Target = forge.Target{Kind: "draft", Title: c.Title, Body: c.Body}
		case "Issue", "PullRequest":
			kind := "issue"
			if c.Type == "PullRequest" {
				kind = "proposal"
			}
			ref := forge.Ref{Kind: kind, ID: "node:" + c.ID}
			key := forgeReferenceMappingKey(ref)
			if s.Mappings[key] != "" {
				n.Item.Target = forge.Target{Kind: kind, Ref: ref}
			} else {
				linkKey := "external." + githubProjectID(c.ID)
				if s.Mappings[linkKey] == "true" {
					n.Item.Target = forge.Target{Kind: "external", URL: c.URL}
					used[linkKey] = true
					g.Findings = append(g.Findings, forge.Finding{Status: forge.Mapped, Path: "content:" + item.ID, Message: "explicitly kept external content URL; source issue/PR properties are not native authority"})
				} else {
					n.Item.Target = forge.Target{Kind: kind, Ref: ref}
					g.Findings = append(g.Findings, forge.Finding{Status: forge.MissingDependency, Path: "content:" + item.ID, Message: "map signed native content via --map " + key + "=NATIVE_ID or keep descriptive URL via --map " + linkKey + "=true"})
				}
			}
		default:
			omit("content:"+item.ID, "unsupported project content kind")
			continue
		}
		for _, v := range item.Values {
			f, exists := fields[v.FieldID]
			if !exists {
				if ignoredTitle[v.FieldID] {
					continue
				}
				if v.FieldID != "" || v.Type != "ProjectV2ItemFieldTextValue" {
					omit("value:"+item.ID+":"+v.Type+":"+v.FieldID, "underlying content or unsupported custom field value is not migrated")
				}
				continue
			}
			nv := forge.Value{Type: f.Type}
			switch f.Type {
			case "text":
				if v.Type != "ProjectV2ItemFieldTextValue" {
					return g, errors.New("GitHub field/value type mismatch")
				}
				nv.Text = v.Text
			case "number":
				if v.Type != "ProjectV2ItemFieldNumberValue" {
					return g, errors.New("GitHub field/value type mismatch")
				}
				number, err := githubProjectNumber(string(v.Number))
				if err != nil {
					return g, err
				}
				nv.Number = number
			case "date":
				if v.Type != "ProjectV2ItemFieldDateValue" {
					return g, errors.New("GitHub field/value type mismatch")
				}
				nv.Date = v.Date
			case "single_select":
				if v.Type != "ProjectV2ItemFieldSingleSelectValue" {
					return g, errors.New("GitHub field/value type mismatch")
				}
				nv.OptionID = githubProjectID(v.OptionID)
			}
			if _, duplicate := n.Item.Values[f.ID]; duplicate {
				return g, errors.New("duplicate GitHub field value")
			}
			n.Item.Values[f.ID] = nv
		}
		g.Nodes = append(g.Nodes, n)
	}
	for k := range used {
		g.UsedMappings = append(g.UsedMappings, k)
	}
	sort.Strings(g.UsedMappings)
	return g, nil
}
func githubProjectNumber(s string) (string, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", errors.New("GitHub numeric field is not a finite supported number")
	}
	v := strconv.FormatFloat(f, 'f', -1, 64)
	original, ok := new(big.Rat).SetString(s)
	canonical, canonicalOK := new(big.Rat).SetString(v)
	if !ok || !canonicalOK || original.Cmp(canonical) != 0 {
		return "", errors.New("GitHub number exceeds exact supported decimal precision")
	}
	if !validResourceDecimal(v) {
		return "", errors.New("GitHub numeric field exceeds native decimal bounds")
	}
	return v, nil
}
func (g *githubForge) ReadProjects(ctx context.Context, s forge.Selection) (forge.Graph, error) {
	if s.Key != "" {
		return forge.Graph{}, errors.New("project selection uses --project user:LOGIN/NUMBER, org:LOGIN/NUMBER or node:ID; --key is not used")
	}
	p, err := g.client.GetProject(ctx, s.Project)
	if err != nil {
		return forge.Graph{}, err
	}
	if err := githubProjectAlreadyNative(p); err != nil {
		return forge.Graph{}, err
	}
	return githubProjectGraph(p, s)
}

// A structured terminal proof associates a publication with a native snapshot.
// Ordinary lookalike prose is never parsed as an association.
const githubProjectProofPrefix = "\n\n<!-- hn:project-transfer:"

type githubProjectProof struct {
	Schema, NativeID, SnapshotDigest, BodyDigest string
	Bindings                                     map[string]string
	Anchors                                      map[string]string
}

func githubProjectBody(body string, proof githubProjectProof) string {
	proof.Schema = "hn.github-project-proof/1"
	proof.BodyDigest = eventID([]byte(body))
	raw, _ := json.Marshal(proof)
	return body + githubProjectProofPrefix + base64.RawURLEncoding.EncodeToString(raw) + " -->"
}
func githubParseProjectBody(body string) (string, githubProjectProof, bool) {
	var p githubProjectProof
	i := strings.LastIndex(body, githubProjectProofPrefix)
	if i < 0 || !strings.HasSuffix(body, " -->") {
		return body, p, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body[i+len(githubProjectProofPrefix) : len(body)-4])
	if err != nil || len(raw) > 1<<20 || forgeStrictJSON(raw, &p) != nil || p.Schema != "hn.github-project-proof/1" || !validEventID(p.NativeID) || !validEventID(p.SnapshotDigest) || p.BodyDigest != eventID([]byte(body[:i])) {
		return body, githubProjectProof{}, false
	}
	return body[:i], p, true
}

type githubProjectStep struct {
	Action, Bind, NativeID, NativeRevision string
	Input                                  map[string]any
}
type githubProjectCheckpoint struct {
	JournalActor, JournalPublicKey, JournalSignature string
	Start                                            *githubapi.ProjectV2
	InitialBindings                                  map[string]string
	SameSource                                       bool
	Schema, Scope, Endpoint, Selector                string
	Mappings                                         map[string]string
	Snapshot                                         forgeResourceSnapshot
	Bindings                                         map[string]string
	Steps                                            []githubProjectStep
	Next                                             int
	Pending                                          *forgeRemoteIntent
	RemoteDigest                                     string
	PendingInvariant                                 string
	Complete                                         bool
}

func githubProjectScope(endpoint, selector, id string) string {
	return eventID([]byte(strings.Join([]string{"github-project-export/1", endpoint, selector, id}, "\x00")))
}
func githubProjectCheckpointName(scope string) string {
	return "project-export-" + strings.TrimPrefix(scope, "sha256:") + ".json"
}
func githubProjectRoot(s forgeResourceSnapshot) (forge.Node, error) {
	for _, n := range s.Graph.Nodes {
		if n.Kind == ResourceProject && n.Record.ID == s.NativeID {
			return n, nil
		}
	}
	return forge.Node{}, errors.New("native project root missing")
}
func githubProjectCopyRemote(p githubapi.ProjectV2) (githubapi.ProjectV2, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return p, err
	}
	var out githubapi.ProjectV2
	if err = json.Unmarshal(raw, &out); err != nil {
		return p, err
	}
	return out, nil
}
func githubProjectRemoteDigest(p githubapi.ProjectV2) string {
	p, _ = githubProjectCopyRemote(p)
	p.UpdatedAt = ""
	p.CreatedAt = ""
	p.Creator = ""
	if len(p.Fields) == 0 {
		p.Fields = nil
	}
	if len(p.Items) == 0 {
		p.Items = nil
	}
	if len(p.Views) == 0 {
		p.Views = nil
	}
	for i := range p.Fields {
		if len(p.Fields[i].Options) == 0 {
			p.Fields[i].Options = nil
		}
	}
	for i := range p.Items {
		p.Items[i].UpdatedAt = ""
		p.Items[i].CreatedAt = ""
		p.Items[i].Creator = ""
		if len(p.Items[i].Values) == 0 {
			p.Items[i].Values = nil
		}
	}
	return forgeDigestJSON(p)
}

// The journal authenticates this pre-step invariant as well as the exact write.
// Positive recovery may change only the mutation's declared target fields; all
// unrelated remote state must still equal the original pre-step observation.
func githubProjectStepInvariant(p githubapi.ProjectV2, step githubProjectStep, input map[string]any) (string, error) {
	var err error
	p, err = githubProjectCopyRemote(p)
	if err != nil {
		return "", err
	}
	str := func(k string) string { v, _ := input[k].(string); return v }
	switch step.Action {
	case "create":
		if p.Readme != "" || p.Closed || p.ShortDescription != "" || len(p.Items) != 0 {
			return "", errors.New("new project has changes outside its exact create intent")
		}
		return eventID([]byte("hn.github-project-absent/1")), nil
	case "update":
		p.Title = ""
		p.Readme = ""
		p.Closed = false
	case "field-create":
		fields := []githubapi.ProjectField{}
		for _, f := range p.Fields {
			if f.Name != str("name") {
				fields = append(fields, f)
			}
		}
		p.Fields = fields
	case "field-update":
		for i := range p.Fields {
			if p.Fields[i].ID == str("fieldId") {
				p.Fields[i].Name = ""
			}
		}
	case "view-create":
		views := []githubapi.ProjectView{}
		for _, v := range p.Views {
			if v.Name != str("name") {
				views = append(views, v)
			}
		}
		p.Views = views
	case "view-update":
		for i := range p.Views {
			if p.Views[i].ID == str("viewId") {
				p.Views[i].Name = ""
				p.Views[i].Layout = ""
			}
		}
	case "item-draft", "item-content":
		items := []githubapi.ProjectItem{}
		for _, item := range p.Items {
			match := item.Content != nil && (step.Action == "item-draft" && item.Content.Type == "DraftIssue" && item.Content.Title == str("title") && item.Content.Body == str("body") || step.Action == "item-content" && item.Content.ID == str("contentId"))
			if !match {
				items = append(items, item)
				continue
			}
			for _, v := range item.Values {
				for _, field := range p.Fields {
					if v.FieldID == field.ID {
						switch field.DataType {
						case "TEXT", "NUMBER", "DATE", "SINGLE_SELECT":
							return "", errors.New("new item acquired custom values outside the exact add intent")
						}
					}
				}
			}
		}
		p.Items = items
	case "draft-update":
		for i := range p.Items {
			if p.Items[i].Content != nil && p.Items[i].Content.ID == str("draftIssueId") {
				p.Items[i].Content.Title = ""
				p.Items[i].Content.Body = ""
			}
		}
	case "value-set", "value-clear":
		for i := range p.Items {
			if p.Items[i].ID == str("itemId") {
				values := []githubapi.ProjectValue{}
				for _, v := range p.Items[i].Values {
					if v.FieldID != str("fieldId") {
						values = append(values, v)
					}
				}
				p.Items[i].Values = values
			}
		}
	case "item-archive", "item-unarchive":
		for i := range p.Items {
			if p.Items[i].ID == str("itemId") {
				p.Items[i].Archived = false
			}
		}
	case "item-position":
		others := []githubapi.ProjectItem{}
		var moving *githubapi.ProjectItem
		for _, item := range p.Items {
			if item.ID == str("itemId") {
				copy := item
				moving = &copy
			} else {
				others = append(others, item)
			}
		}
		if moving == nil {
			return "", errors.New("positioned item is absent from pre-step snapshot")
		}
		p.Items = append(others, *moving)
	default:
		return "", errors.New("unknown project mutation invariant")
	}
	return githubProjectRemoteDigest(p), nil
}

func githubProjectCompact(id string) forgeRemoteResult {
	return forgeRemoteObserved(id, "", "", struct {
		ID string `json:"id"`
	}{id})
}

// Bindings are typed records in action-specific identity slots. Native text is
// always opaque, even when it happens to spell a former placeholder token.
type githubProjectInputRef struct {
	Binding string `json:"hn_binding"`
}

func githubProjectInput(action string, raw map[string]any, bindings map[string]string) (map[string]any, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, errors.New("invalid project step")
	}
	var out map[string]any
	if json.Unmarshal(b, &out) != nil {
		return nil, errors.New("invalid project step")
	}
	slots, known := map[string]string{"create": "ownerId", "update": "projectId readme", "field-create": "projectId", "field-update": "fieldId", "item-content": "projectId contentId", "item-draft": "projectId", "draft-update": "draftIssueId", "value-set": "projectId itemId fieldId", "value-clear": "projectId itemId fieldId", "item-archive": "projectId itemId", "item-unarchive": "projectId itemId", "item-position": "projectId itemId afterId", "view-create": "projectId", "view-update": "viewId"}[action]
	if !known {
		return nil, errors.New("unknown typed project step")
	}
	resolve := func(v any) (any, error) {
		ref, ok := v.(map[string]any)
		if !ok {
			return v, nil
		}
		key, ok := ref["hn_binding"].(string)
		if !ok || len(ref) != 1 {
			return nil, errors.New("invalid typed project identity binding")
		}
		value, ok := bindings[key]
		if !ok || value == "" {
			return nil, errors.New("missing exact project step binding")
		}
		return value, nil
	}
	for _, slot := range strings.Fields(slots) {
		if v, ok := out[slot]; ok {
			out[slot], err = resolve(v)
			if err != nil {
				return nil, err
			}
		}
	}
	if action == "value-set" {
		if value, ok := out["value"].(map[string]any); ok {
			if v, exists := value["singleSelectOptionId"]; exists {
				value["singleSelectOptionId"], err = resolve(v)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return out, nil
}

func githubProjectNativeItems(s forgeResourceSnapshot) []forge.Node {
	out := []forge.Node{}
	for _, n := range s.Graph.Nodes {
		if n.Item != nil {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := new(big.Rat).SetString(out[i].Item.Position)
		b, _ := new(big.Rat).SetString(out[j].Item.Position)
		if a == nil {
			a = new(big.Rat)
		}
		if b == nil {
			b = new(big.Rat)
		}
		if c := a.Cmp(b); c != 0 {
			return c < 0
		}
		return out[i].Record.ID < out[j].Record.ID
	})
	return out
}
func githubProjectBinding(id string) githubProjectInputRef { return githubProjectInputRef{id} }
func githubProjectFieldType(typ string) string {
	return map[string]string{"text": "TEXT", "number": "NUMBER", "date": "DATE", "single_select": "SINGLE_SELECT"}[typ]
}
func githubProjectValueInput(v forge.Value, field string, bindings bool) (map[string]any, error) {
	switch v.Type {
	case "text":
		return map[string]any{"text": v.Text}, nil
	case "date":
		return map[string]any{"date": v.Date}, nil
	case "single_select":
		id := any(v.OptionID)
		if bindings {
			id = githubProjectBinding("option:" + field + ":" + v.OptionID)
		}
		return map[string]any{"singleSelectOptionId": id}, nil
	case "number":
		n, err := githubProjectNumber(v.Number)
		if err != nil || n != v.Number {
			return nil, errors.New("native number cannot be represented exactly by the GitHub Float wire format")
		}
		f, _ := strconv.ParseFloat(n, 64)
		return map[string]any{"number": f}, nil
	}
	return nil, errors.New("unsupported project value")
}
func githubProjectFindField(p githubapi.ProjectV2, id string) (githubapi.ProjectField, bool) {
	for _, f := range p.Fields {
		if f.ID == id {
			return f, true
		}
	}
	return githubapi.ProjectField{}, false
}
func githubProjectFindItem(p githubapi.ProjectV2, id string) (githubapi.ProjectItem, bool) {
	for _, i := range p.Items {
		if i.ID == id {
			return i, true
		}
	}
	return githubapi.ProjectItem{}, false
}
func githubProjectSameOptions(a []forge.Option, b []githubapi.ProjectOption) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func githubPlanProject(s forgeResourceSnapshot, o transferOptions, endpoint string, remote *githubapi.ProjectV2, bindings map[string]string, sameSource bool) ([]githubProjectStep, []forge.Finding, error) {
	root, err := githubProjectRoot(s)
	if err != nil {
		return nil, nil, err
	}
	steps := []githubProjectStep{}
	findings := []forge.Finding{{Status: forge.Mapped, Path: "project", Message: "supported fields/items and explicit content mappings; provider-created defaults are retained"}, {Status: forge.Preserved, Path: "authority", Message: "no provider permission, automation or source author becomes native authority"}, {Status: forge.Mapped, Path: "recovery-identity", Message: "actual export requires the active native identity to sign private recovery state; reuse the same identity for recovery; dry-run needs no private key"}}
	used := map[string]bool{}
	add := func(action, bind, id, revision string, input map[string]any) {
		steps = append(steps, githubProjectStep{action, bind, id, revision, input})
	}
	names := map[string]bool{}
	for _, f := range root.Project.Fields {
		if names[f.Name] {
			return nil, nil, errors.New("duplicate native field names cannot be reconciled unambiguously")
		}
		names[f.Name] = true
	}
	names = map[string]bool{}
	for _, v := range root.Project.Views {
		if names[v.Name] {
			return nil, nil, errors.New("duplicate native view names cannot be reconciled unambiguously")
		}
		names[v.Name] = true
	}
	projectID := githubProjectBinding("project")
	if remote == nil {
		owner := bindings["owner"]
		if owner == "" {
			return nil, nil, errors.New("missing explicit project owner")
		}
		add("create", "project", s.NativeID, s.NativeRevision, map[string]any{"ownerId": owner, "title": "hn-transfer-" + strings.TrimPrefix(githubProjectScope(endpoint, o.project, s.NativeID), "sha256:")})
	}
	for _, f := range root.Project.Fields {
		if f.Type == "single_select" && len(f.Options) == 0 {
			return nil, nil, errors.New("GitHub single-select fields require at least one option")
		}
		names := map[string]bool{}
		for _, opt := range f.Options {
			if names[opt.Name] {
				return nil, nil, errors.New("duplicate native option names cannot be mapped unambiguously")
			}
			names[opt.Name] = true
		}
		key := "field:" + f.ID
		rid := bindings[key]
		if rid != "" {
			if remote == nil {
				return nil, nil, errors.New("field binding without project")
			}
			rf, ok := githubProjectFindField(*remote, rid)
			if !ok || rf.DataType != githubProjectFieldType(f.Type) || !githubProjectSameOptions(f.Options, rf.Options) {
				return nil, nil, errors.New("project schema type/option changes require explicit manual schema migration before any writes")
			}
			for i, opt := range f.Options {
				bindings["option:"+f.ID+":"+opt.ID] = rf.Options[i].ID
			}
			if rf.Name != f.Name {
				add("field-update", "", s.NativeID, s.NativeRevision, map[string]any{"fieldId": rid, "name": f.Name})
			}
		} else {
			opts := []any{}
			for _, opt := range f.Options {
				opts = append(opts, map[string]any{"name": opt.Name, "color": "GRAY", "description": ""})
			}
			input := map[string]any{"projectId": projectID, "name": "hn-field-" + githubProjectID(s.NativeID+"\n"+f.ID), "dataType": githubProjectFieldType(f.Type)}
			if f.Type == "single_select" {
				input["singleSelectOptions"] = opts
			}
			add("field-create", key, s.NativeID, s.NativeRevision, input)
			add("field-update", "", s.NativeID, s.NativeRevision, map[string]any{"fieldId": githubProjectBinding(key), "name": f.Name})
		}
	}
	// Existing fields removed locally must never disappear silently on the destination.
	if remote != nil {
		for key := range bindings {
			if strings.HasPrefix(key, "field:") {
				found := false
				for _, f := range root.Project.Fields {
					if key == "field:"+f.ID {
						found = true
					}
				}
				if !found {
					return nil, nil, errors.New("project field deletion requires manual schema migration")
				}
			}
		}
	}
	items := githubProjectNativeItems(s)
	if remote != nil {
		for _, ri := range remote.Items {
			found := false
			for _, n := range items {
				if bindings["item:"+n.Record.ID] == ri.ID {
					found = true
				}
			}
			if !found {
				return nil, nil, errors.New("remote project has unmapped items; import or explicitly resolve membership before publishing")
			}
		}
	}

	for _, n := range items {
		item := n.Item
		key := "item:" + n.Record.ID
		rid := bindings[key]
		target := item.Target
		content := ""
		if target.Kind != "draft" {
			mappingKey := "content." + githubProjectID(target.Ref.ID+target.URL)
			content = o.mappings[mappingKey]
			if content == "" && sameSource && rid != "" && remote != nil {
				old, ok := githubProjectFindItem(*remote, rid)
				if ok && old.Content != nil {
					content = old.Content.ID
				}
			}
			if content == "" {
				findings = append(findings, forge.Finding{Status: forge.RequiresMapping, Path: "content:" + n.Record.ID, Message: "supply destination Issue/PullRequest global ID via --map " + mappingKey + "=NODE_ID"})
			} else {
				used[mappingKey] = o.mappings[mappingKey] != ""
				bindings["content:"+n.Record.ID] = content
			}
		}
		if rid != "" {
			if remote == nil {
				return nil, nil, errors.New("item binding without remote project")
			}
			old, ok := githubProjectFindItem(*remote, rid)
			if !ok {
				return nil, nil, errors.New("mapped remote project item was deleted; refusing resurrection")
			}
			if target.Kind == "draft" {
				if old.Content == nil || old.Content.Type != "DraftIssue" {
					return nil, nil, errors.New("project target type changed; manual target migration required")
				}
				body := target.Body
				if !sameSource || n.Metadata[forgeResourceSourceKey] == "" {
					body = githubProjectDraftBody(s, n)
				}
				if old.Content.Title != target.Title || old.Content.Body != body {
					add("draft-update", "", n.Record.ID, n.Record.Revision, map[string]any{"draftIssueId": old.Content.ID, "title": target.Title, "body": body})
				}
			} else if old.Content == nil || old.Content.ID != content {
				return nil, nil, errors.New("mapped remote item target differs from explicit content mapping")
			}
		} else if target.Kind == "draft" {
			add("item-draft", key, n.Record.ID, n.Record.Revision, map[string]any{"projectId": projectID, "title": target.Title, "body": githubProjectDraftBody(s, n)})
		} else {
			add("item-content", key, n.Record.ID, n.Record.Revision, map[string]any{"projectId": projectID, "contentId": content})
		}
		valueKeys := []string{}
		for field := range item.Values {
			valueKeys = append(valueKeys, field)
		}
		sort.Strings(valueKeys)
		for _, field := range valueKeys {
			v, err := githubProjectValueInput(item.Values[field], field, true)
			if err != nil {
				return nil, nil, err
			}
			valueInput := map[string]any{"projectId": projectID, "itemId": githubProjectBinding(key), "fieldId": githubProjectBinding("field:" + field), "value": v}
			already := false
			if remote != nil && rid != "" {
				resolved, e := githubProjectInput("value-set", valueInput, bindings)
				if e == nil {
					_, already, _ = githubProjectVerifyStep(githubProjectStep{Action: "value-set"}, resolved, *remote)
				}
			}
			if !already {
				add("value-set", "", n.Record.ID, n.Record.Revision, valueInput)
			}
		}
		if remote != nil && rid != "" {
			old, _ := githubProjectFindItem(*remote, rid)
			for _, v := range old.Values {
				nativeField := ""
				for k, b := range bindings {
					if strings.HasPrefix(k, "field:") && b == v.FieldID {
						nativeField = strings.TrimPrefix(k, "field:")
					}
				}
				if nativeField != "" {
					if _, ok := item.Values[nativeField]; !ok {
						add("value-clear", "", n.Record.ID, n.Record.Revision, map[string]any{"projectId": projectID, "itemId": rid, "fieldId": v.FieldID})
					}
				}
			}
		}
		action := "item-unarchive"
		if item.Archived {
			action = "item-archive"
		}
		archiveNeeded := true
		if remote != nil && rid != "" {
			old, _ := githubProjectFindItem(*remote, rid)
			archiveNeeded = old.Archived != item.Archived
		}
		if archiveNeeded {
			add(action, "", n.Record.ID, n.Record.Revision, map[string]any{"projectId": projectID, "itemId": githubProjectBinding(key)})
		}
	}
	orderMatches := remote != nil && len(remote.Items) == len(items)
	if orderMatches {
		for i, n := range items {
			if remote.Items[i].ID != bindings["item:"+n.Record.ID] {
				orderMatches = false
			}
		}
	}
	previous := any(nil)
	for _, n := range items {
		key := "item:" + n.Record.ID
		if !orderMatches {
			add("item-position", "", n.Record.ID, n.Record.Revision, map[string]any{"projectId": projectID, "itemId": githubProjectBinding(key), "afterId": previous})
		}
		previous = githubProjectBinding(key)
	}
	for _, v := range root.Project.Views {
		if v.GroupByField != "" {
			path := "view-group:" + v.ID
			key := githubProjectOmitKey(path)
			if o.mappings[key] != "true" {
				findings = append(findings, forge.Finding{Status: forge.RequiresMapping, Path: path, Message: "GitHub view mutation cannot set native grouping; explicitly omit with --map " + key + "=true"})
			} else {
				used[key] = true
				findings = append(findings, forge.Finding{Status: forge.Mapped, Path: path, Message: "explicitly omitted native grouping"})
			}
		}
		key := "view:" + v.ID
		layout := map[string]string{"table": "TABLE_LAYOUT", "board": "BOARD_LAYOUT"}[v.Layout]
		if bindings[key] == "" {
			add("view-create", key, s.NativeID, s.NativeRevision, map[string]any{"projectId": projectID, "name": "hn-view-" + githubProjectID(s.NativeID+v.ID), "layout": layout})
		}
		viewMatches := false
		if remote != nil {
			for _, rv := range remote.Views {
				if rv.ID == bindings[key] && rv.Name == v.Name && rv.Layout == layout {
					viewMatches = true
				}
			}
		}
		if !viewMatches {
			add("view-update", "", s.NativeID, s.NativeRevision, map[string]any{"viewId": githubProjectBinding(key), "name": v.Name, "layout": layout})
		}
	}
	for k := range o.mappings {
		if !used[k] {
			return nil, nil, errors.New("unrecognized or unused project export mapping key")
		}
	}
	body := any(root.Project.Body)
	if !sameSource {
		body = githubProjectBinding("readme")
	}
	expectedBody := root.Project.Body
	if !sameSource {
		expectedBody = githubProjectReadme(githubProjectCheckpoint{Snapshot: s, Bindings: bindings})
	}
	if remote == nil || remote.Title != root.Project.Title || remote.Readme != expectedBody || remote.Closed != root.Project.Archived {
		add("update", "", s.NativeID, s.NativeRevision, map[string]any{"projectId": projectID, "title": root.Project.Title, "readme": body, "closed": root.Project.Archived})
	}
	return steps, findings, nil
}
func githubProjectDraftBody(s forgeResourceSnapshot, n forge.Node) string {
	raw, _ := json.Marshal(struct{ Schema, NativeID, BodyDigest string }{"hn.github-project-draft/1", n.Record.ID, eventID([]byte(n.Item.Target.Body))})
	return n.Item.Target.Body + "\n\n<!-- hn:project-draft:" + base64.RawURLEncoding.EncodeToString(raw) + " -->"
}
func githubProjectReadme(cp githubProjectCheckpoint) string {
	root, _ := githubProjectRoot(cp.Snapshot)
	bindings := map[string]string{}
	for k, v := range cp.Bindings {
		if k != "owner" && k != "readme" {
			bindings[k] = v
		}
	}
	return githubProjectBody(root.Project.Body, githubProjectProof{NativeID: cp.Snapshot.NativeID, SnapshotDigest: cp.Snapshot.Digest, Bindings: bindings, Anchors: cp.Snapshot.Anchors})
}
func githubProjectResolveStep(cp githubProjectCheckpoint, step githubProjectStep) (map[string]any, error) {
	b := map[string]string{}
	for k, v := range cp.Bindings {
		b[k] = v
	}
	b["readme"] = githubProjectReadme(cp)
	return githubProjectInput(step.Action, step.Input, b)
}

// Preflight every future journal form, reserving the maximum escaped remote ID
// and all field-option bindings, before the first mutation can become durable.
func githubProjectCheckpointBudget(cp githubProjectCheckpoint) error {
	raw, _ := json.Marshal(cp)
	var worst githubProjectCheckpoint
	if forgeStrictJSON(raw, &worst) != nil {
		return errors.New("invalid project journal")
	}
	if len(worst.Steps) > 4096 {
		return errors.New("project transfer exceeds 4096 mutation step bound")
	}
	worst.JournalActor = strings.Repeat("a", 64)
	worst.JournalPublicKey = strings.Repeat("a", 43)
	worst.JournalSignature = strings.Repeat("a", 86)
	maxID := strings.Repeat("<", 1024)
	root, err := githubProjectRoot(cp.Snapshot)
	if err != nil {
		return err
	}
	for _, step := range cp.Steps {
		if step.Bind != "" && worst.Bindings[step.Bind] == "" {
			worst.Bindings[step.Bind] = maxID
		}
	}
	for _, f := range root.Project.Fields {
		for _, opt := range f.Options {
			key := "option:" + f.ID + ":" + opt.ID
			if worst.Bindings[key] == "" {
				worst.Bindings[key] = maxID
			}
		}
	}
	var largest json.RawMessage
	for _, step := range worst.Steps {
		input, err := githubProjectResolveStep(worst, step)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(input)
		if len(payload)+16384 > githubapi.MaxResponseBytes {
			return errors.New("future project mutation exceeds 4 MiB GraphQL request bound")
		}
		if len(payload) > len(largest) {
			largest = payload
		}
	}
	worst.PendingInvariant = eventID([]byte("worst invariant"))
	worst.Next = len(worst.Steps)
	worst.Pending = &forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: cp.Scope, Provider: "github", Endpoint: cp.Endpoint, Kind: "project", Action: strings.Repeat("a", 64), NativeID: cp.Snapshot.NativeID, NativeRevision: cp.Snapshot.NativeRevision, TargetID: maxID, Payload: largest, PayloadDigest: eventID(largest), Phase: "confirmed", ResultID: maxID, ResultPayload: json.RawMessage(`{"id":"` + maxID + `"}`), ResultDigest: eventID([]byte(maxID))}
	b, err := json.Marshal(worst)
	if err != nil || len(b) > forgeCheckpointLimit {
		return errors.New("complete project checkpoint including future IDs, payloads and results exceeds 8 MiB; split the project before publishing")
	}
	return nil
}

func githubProjectSourceBindings(s forgeResourceSnapshot, p githubapi.ProjectV2, endpoint string) (map[string]string, bool, error) {
	out := map[string]string{}
	root, err := githubProjectRoot(s)
	if err != nil {
		return nil, false, err
	}
	if root.Metadata[forgeResourceSourceKey] == "" {
		return out, false, nil
	}
	source, _, _, err := forgeImportedResourceBaseline(s.NativeID)
	if err != nil {
		return nil, false, err
	}
	if source.Provider != "github" || source.Endpoint != endpoint || source.Record.ID != p.ID {
		return out, false, nil
	}
	out["project"] = p.ID
	for _, f := range p.Fields {
		nativeID := githubProjectID(f.ID)
		for _, nf := range root.Project.Fields {
			if nf.ID == nativeID {
				out["field:"+nf.ID] = f.ID
				for _, opt := range f.Options {
					for _, no := range nf.Options {
						if no.ID == githubProjectID(opt.ID) {
							out["option:"+nf.ID+":"+no.ID] = opt.ID
						}
					}
				}
			}
		}
	}
	for _, v := range p.Views {
		for _, nv := range root.Project.Views {
			if nv.ID == githubProjectID(v.ID) {
				out["view:"+nv.ID] = v.ID
			}
		}
	}
	for _, n := range s.Graph.Nodes {
		if n.Item == nil {
			continue
		}
		if n.Metadata[forgeResourceSourceKey] == "" {
			continue
		}
		src, _, _, err := forgeImportedResourceBaseline(n.Record.ID)
		if err != nil {
			return nil, false, err
		}
		if src.Provider == "github" && src.Endpoint == endpoint {
			out["item:"+n.Record.ID] = src.Record.ID
		}
	}
	return out, true, nil
}
func githubProjectCheckSourceBaseline(s forgeResourceSnapshot, p githubapi.ProjectV2, endpoint string) error {
	graph, err := githubProjectGraph(p, forge.Selection{})
	if err != nil {
		return err
	}
	byID := map[string]forge.Node{}
	for _, n := range graph.Nodes {
		byID[n.Record.ID] = n
	}
	for _, n := range s.Graph.Nodes {
		if n.Metadata[forgeResourceSourceKey] == "" {
			continue
		}
		src, obs, _, err := forgeImportedResourceBaseline(n.Record.ID)
		if err != nil {
			return err
		}
		if src.Provider != "github" || src.Endpoint != endpoint {
			continue
		}
		remote, ok := byID[src.Record.ID]
		if !ok {
			return errors.New("source project dependency was deleted; refusing resurrection")
		}
		if forgeResourceRemoteDigest(remote) != obs.RemoteDigest {
			return errors.New("remote project or item changed since its signed import baseline; import and resolve before exporting")
		}
	}
	return nil
}
func githubProjectCheckAssociation(cp githubProjectCheckpoint, p githubapi.ProjectV2) error {
	root, err := githubProjectRoot(cp.Snapshot)
	if err != nil {
		return err
	}
	same := false
	if root.Metadata[forgeResourceSourceKey] != "" {
		src, _, _, err := forgeImportedResourceBaseline(cp.Snapshot.NativeID)
		if err != nil {
			return err
		}
		same = src.Provider == "github" && src.Endpoint == cp.Endpoint && src.Record.ID == p.ID
	}
	if !same {
		_, proof, ok := githubParseProjectBody(p.Readme)
		temp := "hn-transfer-" + strings.TrimPrefix(cp.Scope, "sha256:")
		if !(ok && proof.NativeID == cp.Snapshot.NativeID) && p.Title != temp {
			return errors.New("remote project lacks exact native publication association; refusing checkpoint-selected target")
		}
	}
	for k, id := range cp.Bindings {
		switch {
		case k == "project":
			if id != p.ID {
				return errors.New("project binding changed")
			}
		case strings.HasPrefix(k, "field:"):
			f, ok := githubProjectFindField(p, id)
			if !ok {
				return errors.New("mapped project field missing")
			}
			native := strings.TrimPrefix(k, "field:")
			var nf *forge.Field
			for i := range root.Project.Fields {
				if root.Project.Fields[i].ID == native {
					nf = &root.Project.Fields[i]
				}
			}
			if nf == nil || f.DataType != githubProjectFieldType(nf.Type) || !githubProjectSameOptions(nf.Options, f.Options) {
				return errors.New("remote field differs from exact native schema")
			}
			if !same && f.Name != nf.Name && f.Name != "hn-field-"+githubProjectID(cp.Snapshot.NativeID+"\n"+native) {
				return errors.New("remote field lacks exact native publication association")
			}
		case strings.HasPrefix(k, "item:"):
			i, ok := githubProjectFindItem(p, id)
			if !ok {
				return errors.New("mapped remote project item missing")
			}
			native := strings.TrimPrefix(k, "item:")
			var n forge.Node
			for _, candidate := range cp.Snapshot.Graph.Nodes {
				if candidate.Record.ID == native {
					n = candidate
				}
			}
			if n.Item == nil || i.Content == nil {
				return errors.New("project item target unavailable")
			}
			if !same && n.Item.Target.Kind == "draft" && i.Content.Body != githubProjectDraftBody(cp.Snapshot, n) {
				return errors.New("draft publication association or content changed")
			}
			if n.Item.Target.Kind != "draft" && i.Content.ID != cp.Bindings["content:"+native] {
				return errors.New("project content mapping changed")
			}
		}
	}
	return nil
}
func githubProjectHydrateOptions(cp *githubProjectCheckpoint, p githubapi.ProjectV2) error {
	root, _ := githubProjectRoot(cp.Snapshot)
	for _, f := range root.Project.Fields {
		rid := cp.Bindings["field:"+f.ID]
		if rid == "" {
			continue
		}
		rf, ok := githubProjectFindField(p, rid)
		if !ok || !githubProjectSameOptions(f.Options, rf.Options) {
			return errors.New("remote field options differ from native schema")
		}
		for i, o := range f.Options {
			cp.Bindings["option:"+f.ID+":"+o.ID] = rf.Options[i].ID
		}
	}
	return nil
}
func githubProjectVerifyStep(step githubProjectStep, input map[string]any, p githubapi.ProjectV2) (string, bool, error) {
	str := func(k string) string { v, _ := input[k].(string); return v }
	switch step.Action {
	case "create":
		if p.Title == str("title") && p.Owner.ID == str("ownerId") {
			return p.ID, true, nil
		}
	case "update":
		closed, _ := input["closed"].(bool)
		if p.ID == str("projectId") && p.Title == str("title") && p.Readme == str("readme") && p.Closed == closed {
			return p.ID, true, nil
		}
	case "field-create":
		matches := []string{}
		for _, f := range p.Fields {
			if f.Name == str("name") && f.DataType == str("dataType") {
				opts, _ := input["singleSelectOptions"].([]any)
				if len(opts) != len(f.Options) {
					continue
				}
				equal := true
				for i, opt := range opts {
					m, _ := opt.(map[string]any)
					if m["name"] != f.Options[i].Name {
						equal = false
					}
				}
				if equal {
					matches = append(matches, f.ID)
				}
			}
		}
		if len(matches) > 1 {
			return "", false, errors.New("ambiguous remote field creation")
		}
		if len(matches) == 1 {
			return matches[0], true, nil
		}
	case "field-update":
		f, ok := githubProjectFindField(p, str("fieldId"))
		if ok && f.Name == str("name") {
			return f.ID, true, nil
		}
	case "view-create", "view-update":
		matches := []string{}
		for _, v := range p.Views {
			if v.Name == str("name") && v.Layout == str("layout") && (step.Action == "view-create" || v.ID == str("viewId")) {
				matches = append(matches, v.ID)
			}
		}
		if len(matches) > 1 {
			return "", false, errors.New("ambiguous remote view creation")
		}
		if len(matches) == 1 {
			return matches[0], true, nil
		}
	case "item-draft", "item-content":
		matches := []string{}
		for _, i := range p.Items {
			if i.Content == nil {
				continue
			}
			if step.Action == "item-draft" && i.Content.Type == "DraftIssue" && i.Content.Title == str("title") && i.Content.Body == str("body") || step.Action == "item-content" && i.Content.ID == str("contentId") {
				matches = append(matches, i.ID)
			}
		}
		if len(matches) > 1 {
			return "", false, errors.New("ambiguous remote item creation")
		}
		if len(matches) == 1 {
			return matches[0], true, nil
		}
	case "draft-update":
		for _, i := range p.Items {
			if i.Content != nil && i.Content.ID == str("draftIssueId") && i.Content.Title == str("title") && i.Content.Body == str("body") {
				return i.Content.ID, true, nil
			}
		}
	case "item-archive", "item-unarchive":
		i, ok := githubProjectFindItem(p, str("itemId"))
		if ok && i.Archived == (step.Action == "item-archive") {
			return i.ID, true, nil
		}
	case "value-set", "value-clear":
		i, ok := githubProjectFindItem(p, str("itemId"))
		if !ok {
			return "", false, nil
		}
		found := false
		for _, v := range i.Values {
			if v.FieldID != str("fieldId") {
				continue
			}
			found = true
			if step.Action == "value-clear" {
				return "", false, nil
			}
			desired, _ := input["value"].(map[string]any)
			actual := map[string]any{}
			switch v.Type {
			case "ProjectV2ItemFieldTextValue":
				actual["text"] = v.Text
			case "ProjectV2ItemFieldDateValue":
				actual["date"] = v.Date
			case "ProjectV2ItemFieldNumberValue":
				f, err := strconv.ParseFloat(string(v.Number), 64)
				if err != nil {
					return "", false, err
				}
				actual["number"] = f
			case "ProjectV2ItemFieldSingleSelectValue":
				actual["singleSelectOptionId"] = v.OptionID
			}
			if forgeDigestJSON(actual) == forgeDigestJSON(desired) {
				return i.ID, true, nil
			}
		}
		if !found && step.Action == "value-clear" {
			return i.ID, true, nil
		}
	case "item-position":
		id := str("itemId")
		after := str("afterId")
		for index, item := range p.Items {
			if item.ID == id && (index == 0 && after == "" || index > 0 && p.Items[index-1].ID == after) {
				return id, true, nil
			}
		}
	}
	return "", false, nil
}

func githubExportProject(ctx context.Context, identity forge.Identity, o transferOptions, current forgeResourceSnapshot) error {
	g, ok := identity.(*githubForge)
	if !ok {
		return errors.New("GitHub project exporter requires selected GitHub adapter")
	}
	var signer *Identity
	var err error
	if !o.dryRun {
		signer, err = loadIdentity()
		if err != nil {
			return errors.New("project export requires active native identity to authenticate private recovery; run hn init or select an identity")
		}
		if err = validateIdentity(signer); err != nil {
			return err
		}
	}
	selector, err := githubapi.ParseProjectSelector(o.project, true)
	if err != nil {
		return err
	}
	scope := githubProjectScope(g.Endpoint(), o.project, current.NativeID)
	name := githubProjectCheckpointName(scope)
	cp := githubProjectCheckpoint{}
	exists, err := forgeLoadJSON(name, &cp)
	if err != nil {
		return err
	}
	var remote *githubapi.ProjectV2
	previousExport := false
	previousBindings := map[string]string{}
	fetch := func(id string) (*githubapi.ProjectV2, error) {
		p, e := g.client.GetProject(ctx, "node:"+id)
		if e != nil {
			return nil, e
		}
		return &p, nil
	}
	if exists {
		if err = githubProjectVerifyCheckpoint(cp, signer); err != nil {
			return err
		}
		if cp.Schema != "hn.github-project-export/1" || cp.Scope != scope || cp.Endpoint != g.Endpoint() || cp.Selector != o.project || cp.Snapshot.NativeID != current.NativeID || forgeDigestJSON(cp.Mappings) != forgeDigestJSON(o.mappings) || cp.Next < 0 || cp.Next > len(cp.Steps) || cp.Bindings == nil {
			return errors.New("project checkpoint scope/selection differs")
		}
		if err = forgeAuthenticateResourceSnapshot(cp.Snapshot); err != nil {
			return err
		}
		if err = githubProjectValidatePlan(ctx, g, cp, o); err != nil {
			return err
		}
		if cp.Bindings["project"] != "" {
			remote, err = fetch(cp.Bindings["project"])
			if err != nil {
				return err
			}
			if err = githubProjectCheckAssociation(cp, *remote); err != nil {
				return err
			}
		}
		if cp.Complete {
			if remote == nil {
				return errors.New("complete project checkpoint has no target")
			}
			if githubProjectRemoteDigest(*remote) != cp.RemoteDigest {
				return errors.New("remote project changed since export baseline; refusing overwrite")
			}
			if cp.Snapshot.Digest == current.Digest {
				fmt.Println("GitHub project already matches the selected native snapshot")
				return nil
			}
			previousExport = true
			previousBindings = githubProjectCopyBindings(cp.Bindings)
			exists = false
		}
	}
	if !exists {
		cp = githubProjectCheckpoint{Schema: "hn.github-project-export/1", Scope: scope, Endpoint: g.Endpoint(), Selector: o.project, Mappings: o.mappings, Snapshot: current, Bindings: map[string]string{}}
		if remote == nil {
			if selector.Number > 0 || selector.ID != "" {
				p, e := g.client.GetProject(ctx, o.project)
				if e != nil {
					return e
				}
				remote = &p
			} else {
				owner, e := g.client.GetProjectOwner(ctx, o.project)
				if e != nil {
					return e
				}
				cp.Bindings["owner"] = owner.ID
				list, e := g.client.ListProjects(ctx, o.project)
				if e != nil {
					return e
				}
				for _, p := range list {
					_, proof, valid := githubParseProjectBody(p.Readme)
					if valid && proof.NativeID == current.NativeID {
						if remote != nil {
							return errors.New("multiple remote boards claim this native project")
						}
						remote, err = fetch(p.ID)
						if err != nil {
							return err
						}
					}
				}
			}
		}
		same := false
		if remote != nil {
			bindings, isSource, e := githubProjectSourceBindings(current, *remote, g.Endpoint())
			if e != nil {
				return e
			}
			same = isSource
			if same {
				if e = githubProjectCheckSourceBaseline(current, *remote, g.Endpoint()); e != nil && !previousExport {
					candidate := cp
					candidate.Snapshot = current
					candidate.Bindings = bindings
					if githubProjectMatchesSnapshot(candidate, *remote) != nil {
						return e
					}
				}
				cp.Bindings = bindings
				if previousExport {
					for key, value := range previousBindings {
						if existing := cp.Bindings[key]; existing != "" && existing != value {
							return errors.New("completed source binding differs from immutable source identity")
						}
						cp.Bindings[key] = value
					}
				}
			} else {
				_, proof, valid := githubParseProjectBody(remote.Readme)
				if !valid || proof.NativeID != current.NativeID {
					return errors.New("destination board is not associated with selected native project")
				}
				events, _, e := collectIssueEvents()
				if e != nil {
					return e
				}
				oldEvents, e := forgeEventsAtAnchors(events, proof.Anchors)
				if e != nil {
					return e
				}
				old, e := forgeSnapshotResourceAt(current.NativeID, forge.ResourceProject, oldEvents)
				if e != nil || old.Digest != proof.SnapshotDigest {
					return errors.New("remote association references unavailable or mismatched native snapshot")
				}
				cp.Bindings = proof.Bindings
				if cp.Bindings["project"] != remote.ID {
					return errors.New("remote project proof binding mismatch")
				}
				oldCP := cp
				oldCP.Snapshot = old
				if e = githubProjectCheckAssociation(oldCP, *remote); e != nil {
					return e
				}
				if e = githubProjectMatchesSnapshot(oldCP, *remote); e != nil {
					return e
				}
			}
			cp.RemoteDigest = githubProjectRemoteDigest(*remote)
		}
		cp.Start = remote
		cp.InitialBindings = githubProjectCopyBindings(cp.Bindings)
		cp.SameSource = same
		steps, findings, e := githubPlanProject(current, o, g.Endpoint(), remote, cp.Bindings, same)
		if e != nil {
			return e
		}
		plan := forge.Plan{Resource: forge.ResourceProject, Direction: "export", Nodes: len(current.Graph.Nodes), Findings: findings}
		if e = json.NewEncoder(os.Stdout).Encode(plan); e != nil {
			return e
		}
		if plan.Blocked() {
			return errors.New("project export requires explicit content or unsupported-semantics mapping")
		}
		cp.Steps = steps
		// Preflight destination content identities before creating any board or field.
		for key, id := range cp.Bindings {
			if strings.HasPrefix(key, "content:") {
				c, e := g.client.GetProjectContent(ctx, id)
				if e != nil {
					return e
				}
				native := strings.TrimPrefix(key, "content:")
				for _, n := range current.Graph.Nodes {
					if n.Record.ID == native && n.Item != nil {
						if n.Item.Target.Kind == "issue" && c.Type != "Issue" || n.Item.Target.Kind == "proposal" && c.Type != "PullRequest" {
							return errors.New("explicit project content mapping has wrong provider type")
						}
					}
				}
			}
		}
		if err = githubProjectCheckpointBudget(cp); err != nil {
			return err
		}
		if o.dryRun {
			return nil
		}
		if err = githubProjectSaveCheckpoint(name, &cp, signer); err != nil {
			return err
		}
	} else {
		if err = githubProjectCheckpointBudget(cp); err != nil {
			return err
		}
		if o.dryRun {
			return json.NewEncoder(os.Stdout).Encode(forge.Plan{Resource: forge.ResourceProject, Direction: "export", Nodes: len(cp.Snapshot.Graph.Nodes), Findings: []forge.Finding{{Status: forge.Mapped, Path: "recovery", Message: "resume exact frozen native snapshot; uncertain writes require exact positive observation"}}})
		}
	}
	save := func() error { return githubProjectSaveCheckpoint(name, &cp, signer) }
	for cp.Next < len(cp.Steps) {
		step := cp.Steps[cp.Next]
		input, e := githubProjectResolveStep(cp, step)
		if e != nil {
			return e
		}
		payload, _ := json.Marshal(input)
		if cp.Pending == nil {
			if remote != nil && cp.RemoteDigest != "" && githubProjectRemoteDigest(*remote) != cp.RemoteDigest {
				return errors.New("remote project changed between transfer steps")
			}
			if remote == nil {
				cp.PendingInvariant = eventID([]byte("hn.github-project-absent/1"))
			} else {
				cp.PendingInvariant, e = githubProjectStepInvariant(*remote, step, input)
				if e != nil {
					return e
				}
			}
			cp.Pending = &forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: scope, Provider: "github", Endpoint: g.Endpoint(), Kind: "project", Action: step.Action, NativeID: step.NativeID, NativeRevision: step.NativeRevision, TargetID: cp.Bindings["project"], Payload: payload, PayloadDigest: eventID(payload), Phase: "prepared"}
		}
		intent := cp.Pending
		if !validEventID(cp.PendingInvariant) {
			return errors.New("project recovery lacks authenticated pre-step invariant; reconcile manually")
		}
		if intent.Scope != scope || intent.Provider != "github" || intent.Endpoint != g.Endpoint() || intent.Kind != "project" || intent.Action != step.Action || intent.NativeID != step.NativeID || intent.NativeRevision != step.NativeRevision || string(intent.Payload) != string(payload) || intent.TargetID != cp.Bindings["project"] {
			return errors.New("saved project intent differs from frozen native step")
		}
		observe := func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, bool, error) {
			var p *githubapi.ProjectV2
			var e error
			if step.Action == "create" && cp.Bindings["project"] == "" {
				list, err := g.client.ListProjects(ctx, o.project)
				if err != nil {
					return forgeRemoteResult{}, false, err
				}
				for _, candidate := range list {
					if candidate.Title == input["title"] && candidate.Owner.ID == input["ownerId"] {
						if p != nil {
							return forgeRemoteResult{}, false, errors.New("ambiguous project creation")
						}
						p, e = fetch(candidate.ID)
						if e != nil {
							return forgeRemoteResult{}, false, e
						}
					}
				}
				if p == nil {
					return forgeRemoteResult{}, false, nil
				}
			} else {
				p, e = fetch(cp.Bindings["project"])
				if e != nil {
					return forgeRemoteResult{}, false, e
				}
			}
			id, found, e := githubProjectVerifyStep(step, input, *p)
			if e != nil {
				return forgeRemoteResult{}, false, e
			}
			if found {
				invariant, err := githubProjectStepInvariant(*p, step, input)
				if err != nil || invariant != cp.PendingInvariant {
					return forgeRemoteResult{}, false, errors.New("remote project changed outside the exact pending mutation; manual reconciliation required")
				}
				remote = p
				return githubProjectCompact(id), true, nil
			}
			return forgeRemoteResult{}, false, nil
		}
		result, e := forgeExecuteRemoteIntent(ctx, intent, save, func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, error) {
			if remote != nil {
				fresh, e := fetch(remote.ID)
				if e != nil {
					return forgeRemoteResult{}, e
				}
				if githubProjectRemoteDigest(*fresh) != cp.RemoteDigest {
					return forgeRemoteResult{}, errors.New("remote project changed before mutation")
				}
			}
			raw, e := g.client.ProjectMutation(ctx, step.Action, input)
			if e != nil {
				return forgeRemoteResult{}, e
			}
			if step.Action == "create" {
				id, e := githubapi.ProjectMutationID(raw)
				if e != nil {
					return forgeRemoteResult{}, e
				}
				p, e := fetch(id)
				if e != nil {
					return forgeRemoteResult{}, e
				}
				rid, found, e := githubProjectVerifyStep(step, input, *p)
				if e != nil || !found {
					return forgeRemoteResult{}, errors.New("project mutation result differs from exact intent")
				}
				invariant, err := githubProjectStepInvariant(*p, step, input)
				if err != nil || invariant != cp.PendingInvariant {
					return forgeRemoteResult{}, errors.New("created project differs from exact pending construction; reconcile manually")
				}
				remote = p
				return githubProjectCompact(rid), nil
			}
			r, found, e := observe(ctx, i)
			if e != nil || !found {
				return r, errors.New("project mutation result differs from exact intent")
			}
			return r, nil
		}, observe, func(i forgeRemoteIntent, r forgeRemoteResult) error {
			if remote == nil {
				var e error
				target := cp.Bindings["project"]
				if target == "" && step.Action == "create" {
					target = r.ID
				}
				remote, e = fetch(target)
				if e != nil {
					return e
				}
			}
			id, found, e := githubProjectVerifyStep(step, input, *remote)
			if e != nil || !found || r.ID != id {
				return errors.New("confirmed project result differs from exact observed intent")
			}
			invariant, e := githubProjectStepInvariant(*remote, step, input)
			if e != nil || invariant != cp.PendingInvariant {
				return errors.New("confirmed project observation includes an unrelated remote change; reconcile manually")
			}
			return nil
		})
		if e != nil {
			return e
		}
		if step.Bind != "" {
			cp.Bindings[step.Bind] = result.ID
		}
		if remote == nil {
			return errors.New("missing confirmed project observation")
		}
		if e = githubProjectHydrateOptions(&cp, *remote); e != nil {
			return e
		}
		cp.RemoteDigest = githubProjectRemoteDigest(*remote)
		cp.Next++
		cp.Pending = nil
		cp.PendingInvariant = ""
		if e = save(); e != nil {
			return e
		}
	}
	if remote == nil {
		return errors.New("missing final project observation")
	}
	if err = githubProjectMatchesSnapshot(cp, *remote); err != nil {
		return err
	}
	cp.Complete = true
	if err = save(); err != nil {
		return err
	}
	fmt.Println("Exported native project to GitHub Projects V2")
	return nil
}

func githubProjectMatchesSnapshot(cp githubProjectCheckpoint, p githubapi.ProjectV2) error {
	root, err := githubProjectRoot(cp.Snapshot)
	if err != nil {
		return err
	}
	body := root.Project.Body
	if _, proof, ok := githubParseProjectBody(p.Readme); ok && proof.NativeID == cp.Snapshot.NativeID {
		body = githubProjectReadme(cp)
	}
	if p.Title != root.Project.Title || p.Readme != body || p.Closed != root.Project.Archived {
		return errors.New("remote project differs from native publication baseline")
	}
	for _, f := range root.Project.Fields {
		rf, ok := githubProjectFindField(p, cp.Bindings["field:"+f.ID])
		if !ok || rf.Name != f.Name || rf.DataType != githubProjectFieldType(f.Type) || !githubProjectSameOptions(f.Options, rf.Options) {
			return errors.New("remote project schema differs from native publication baseline")
		}
	}
	items := githubProjectNativeItems(cp.Snapshot)
	if len(items) != len(p.Items) {
		return errors.New("remote project item membership changed")
	}
	for index, n := range items {
		rid := cp.Bindings["item:"+n.Record.ID]
		if p.Items[index].ID != rid {
			return errors.New("remote project item ordering changed")
		}
		item := p.Items[index]
		if item.Archived != n.Item.Archived || item.Content == nil {
			return errors.New("remote project item state changed")
		}
		target := n.Item.Target
		if target.Kind == "draft" {
			expected := target.Body
			if n.Metadata[forgeResourceSourceKey] == "" {
				expected = githubProjectDraftBody(cp.Snapshot, n)
			}
			if item.Content.Type != "DraftIssue" || item.Content.Title != target.Title || item.Content.Body != expected {
				return errors.New("remote draft content changed")
			}
		} else if item.Content.ID != cp.Bindings["content:"+n.Record.ID] {
			return errors.New("remote content target changed")
		}
		for field, v := range n.Item.Values {
			value, err := githubProjectValueInput(v, field, true)
			if err != nil {
				return err
			}
			input, err := githubProjectInput("value-set", map[string]any{"itemId": rid, "fieldId": cp.Bindings["field:"+field], "value": value}, cp.Bindings)
			if err != nil {
				return err
			}
			if _, found, err := githubProjectVerifyStep(githubProjectStep{Action: "value-set"}, input, p); err != nil || !found {
				return errors.New("remote field value changed")
			}
		}
		for _, v := range item.Values {
			for key, id := range cp.Bindings {
				if strings.HasPrefix(key, "field:") && v.FieldID == id {
					if _, ok := n.Item.Values[strings.TrimPrefix(key, "field:")]; !ok {
						return errors.New("remote field value added")
					}
				}
			}
		}
	}
	for _, v := range root.Project.Views {
		found := false
		for _, rv := range p.Views {
			if rv.ID == cp.Bindings["view:"+v.ID] && rv.Name == v.Name && rv.Layout == map[string]string{"table": "TABLE_LAYOUT", "board": "BOARD_LAYOUT"}[v.Layout] {
				found = true
			}
		}
		if !found {
			return errors.New("remote view differs from native publication baseline")
		}
	}
	return nil
}

func githubProjectCopyBindings(b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range b {
		out[k] = v
	}
	return out
}
func githubProjectValidatePlan(ctx context.Context, g *githubForge, cp githubProjectCheckpoint, o transferOptions) error {
	if cp.InitialBindings == nil {
		return errors.New("project journal lacks exact initial plan")
	}
	if cp.Start == nil {
		if cp.SameSource {
			return errors.New("source plan lacks project")
		}
		owner, err := g.client.GetProjectOwner(ctx, o.project)
		if err != nil {
			return err
		}
		if len(cp.InitialBindings) != 1 || cp.InitialBindings["owner"] != owner.ID {
			return errors.New("project creation owner differs from exact selector")
		}
	} else if cp.SameSource {
		expected, same, err := githubProjectSourceBindings(cp.Snapshot, *cp.Start, cp.Endpoint)
		if err != nil || !same {
			return errors.New("initial project source bindings differ from signed immutable identity")
		}
		// The verified journal may carry bindings created by earlier completed exports.
		// Every original source binding is still rederived from immutable native facts;
		// additional bindings are authenticated by the selected identity's whole-journal signature.
		for key, value := range expected {
			if cp.InitialBindings[key] != value {
				return errors.New("initial project source binding differs from immutable source identity")
			}
		}
	} else {
		_, proof, ok := githubParseProjectBody(cp.Start.Readme)
		if !ok || proof.NativeID != cp.Snapshot.NativeID || forgeDigestJSON(proof.Bindings) != forgeDigestJSON(cp.InitialBindings) {
			return errors.New("initial project bindings lack exact publication proof")
		}
		events, _, err := collectIssueEvents()
		if err != nil {
			return err
		}
		oldEvents, err := forgeEventsAtAnchors(events, proof.Anchors)
		if err != nil {
			return err
		}
		old, err := forgeSnapshotResourceAt(cp.Snapshot.NativeID, forge.ResourceProject, oldEvents)
		if err != nil || old.Digest != proof.SnapshotDigest {
			return errors.New("initial project proof is not bound to actual signed native facts")
		}
	}
	bindings := githubProjectCopyBindings(cp.InitialBindings)
	steps, findings, err := githubPlanProject(cp.Snapshot, o, cp.Endpoint, cp.Start, bindings, cp.SameSource)
	if err != nil {
		return err
	}
	if (forge.Plan{Findings: findings}).Blocked() || forgeDigestJSON(steps) != forgeDigestJSON(cp.Steps) {
		return errors.New("saved project steps differ from exact native snapshot plan")
	}
	for k, v := range bindings {
		if cp.Bindings[k] != v {
			return errors.New("saved project binding differs from exact initial plan")
		}
	}
	return nil
}
func githubProjectAlreadyNative(p githubapi.ProjectV2) error {
	_, proof, ok := githubParseProjectBody(p.Readme)
	if !ok {
		return nil
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		return err
	}
	found := false
	for _, e := range events {
		if e.ID == proof.NativeID {
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	oldEvents, err := forgeEventsAtAnchors(events, proof.Anchors)
	if err != nil {
		return errors.New("project publication claims an existing native identity but its signed snapshot is inaccessible")
	}
	s, err := forgeSnapshotResourceAt(proof.NativeID, forge.ResourceProject, oldEvents)
	if err != nil || s.Digest != proof.SnapshotDigest {
		return errors.New("project publication claims existing native identity with mismatched snapshot; refusing duplicate import")
	}
	if err = forgeAuthenticateResourceSnapshot(s); err != nil {
		return err
	}
	return errors.New("project publication is already native in this repository; refusing duplicate import")
}

const githubProjectJournalDomain = "hn.github-project-private-checkpoint/1\x00"

func githubProjectJournalPayload(cp githubProjectCheckpoint) ([]byte, error) {
	cp.JournalSignature = ""
	raw, err := json.Marshal(cp)
	if err != nil || len(raw) > forgeCheckpointLimit {
		return nil, errors.New("signed project checkpoint exceeds JSON bounds")
	}
	return append([]byte(githubProjectJournalDomain), raw...), nil
}
func githubProjectSaveCheckpoint(name string, cp *githubProjectCheckpoint, identity *Identity) error {
	if identity == nil {
		return errors.New("project recovery persistence requires selected native identity")
	}
	cp.JournalActor = identity.Actor
	cp.JournalPublicKey = identity.PublicKey
	payload, err := githubProjectJournalPayload(*cp)
	if err != nil {
		return err
	}
	private, err := identity.privateKey()
	if err != nil {
		return err
	}
	cp.JournalSignature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return forgeStoreJSON(name, cp)
}
func githubProjectVerifyCheckpoint(cp githubProjectCheckpoint, identity *Identity) error {
	public, err := base64.RawStdEncoding.DecodeString(cp.JournalPublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || actorForPublicKey(public) != cp.JournalActor {
		return errors.New("project recovery identity envelope is invalid")
	}
	if identity != nil && (identity.Actor != cp.JournalActor || identity.PublicKey != cp.JournalPublicKey) {
		return errors.New("project recovery belongs to a different native identity; select its original signer or reconcile manually before proceeding")
	}
	signature, err := base64.RawStdEncoding.DecodeString(cp.JournalSignature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("project recovery signature is invalid; progress and bindings cannot authorize writes")
	}
	payload, err := githubProjectJournalPayload(cp)
	if err != nil {
		return err
	}
	if !ed25519.Verify(public, payload, signature) {
		return errors.New("project recovery signature does not authenticate exact scope, snapshot, progress, bindings and pending intent; no writes permitted")
	}
	return nil
}
