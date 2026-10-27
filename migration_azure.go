//go:build !noforge

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"hubnot/internal/azureapi"
	"hubnot/internal/migration"
)

func init() {
	if e := migrationProviders.Register(migration.ProviderRegistration{Kind: "azure", RequiredCoverage: []migration.CoverageRequirement{{Kind: "work_item", Read: true, Preserve: true, Create: true, Update: true}, {Kind: "comment", Read: true, Preserve: true, Create: true}}, NewSource: func(c migration.ProviderConfig) (migration.Source, error) { return newAzureMigration(c) }, NewDestination: func(c migration.ProviderConfig) (migration.Destination, error) { return newAzureMigration(c) }}); e != nil {
		panic(e)
	}
}

type azureMigration struct {
	config migration.ProviderConfig
	client *azureapi.Client
}

func newAzureMigration(c migration.ProviderConfig) (*azureMigration, error) {
	if c.Locator.Kind != "azure" || !azureapi.ValidSegment(c.Locator.Organization) || !azureapi.ValidSegment(c.Locator.Project) || c.Locator.Repository != "" && !azureapi.ValidSegment(c.Locator.Repository) {
		return nil, errors.New("Azure requires explicit organization/collection and project; repository is a separate optional selector")
	}
	client, e := azureapi.NewClient(c.Locator.Endpoint, c.Token, c.CredentialMode, c.APIVersion)
	if e != nil {
		return nil, e
	}
	c.Locator.Endpoint = client.BaseURL()
	c.PlanningSelectors = append([]string{}, c.PlanningSelectors...)
	c.Attachments = append([]migration.AttachmentSelection{}, c.Attachments...)
	return &azureMigration{c, client}, nil
}
func (a *azureMigration) Locator() migration.Locator { return a.config.Locator }
func (a *azureMigration) Capabilities(context.Context) ([]migration.Capability, error) {
	out := []migration.Capability{{Kind: "work_item", Read: true, Preserve: true, Create: true, Update: true, Limits: []string{"Explicit process GUID, type and state mappings validated against destination metadata; title/description/type/state only; no universal open/closed mapping, custom-field recreation or bypassRules", "Services REST 7.1/comments 7.1-preview.4; Server 2022 REST 7.0/comments 7.0-preview.3; supplied Entra bearer or PAT only", "200 Work Items per detail batch; 100 pages per collection; 4 MiB API response; 64 MiB total operation read budget; plan budgets are additional ceilings", "Original fields/relations and attribution preserved as inert source blobs; deterministic plain-text recovery marker augments destination descriptions"}}, {Kind: "comment", Read: true, Preserve: true, Create: true, Limits: []string{"WIT comments, continuation-token complete reads; confirmed engine receipt resolves parent; original authors/times remain attribution; deleted comments preservation only"}}}
	for _, k := range []string{"project", "change_request", "review", "inline_comment", "reply", "workflow", "attachment"} {
		out = append(out, migration.Capability{Kind: k, Read: true, Preserve: true, Limits: []string{"Preservation only; no destination-native recreation or execution/permission adoption", "PRs require explicit repository; selected YAML requires exact existing commit and path; classic definitions require ID@revision; attachment bytes require explicit same-origin selection", "Unexpected thread continuation is refused because the qualified endpoint documents no cursor parameter"}})
	}
	return out, nil
}
func azureJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func azureID(n int) string   { return strconv.Itoa(n) }
func azureFinding(id, status, path, message string) migration.Finding {
	return migration.Finding{ID: id, Status: status, Path: path, Message: message}
}
func azureUnavailable(e error) bool {
	var h *azureapi.HTTPError
	return errors.As(e, &h) && (h.StatusCode == 401 || h.StatusCode == 403 || h.StatusCode == 404 || h.StatusCode == 410)
}

type azureBlobs map[string][]byte

func (b azureBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	v, ok := b[d]
	if !ok {
		return nil, errors.New("Azure blob not in selected snapshot")
	}
	return io.NopCloser(bytes.NewReader(v)), nil
}

type azureBuilder struct {
	s       migration.Snapshot
	blobs   azureBlobs
	limits  migration.Limits
	project string
	total   int64
	err     error
}

func (b *azureBuilder) key(kind, id string) migration.Key {
	return migration.Key{Provider: "azure", Instance: b.s.Source.Endpoint + "/" + b.s.Source.Organization, ProjectID: b.project, Kind: kind, ID: id}
}
func (b *azureBuilder) blob(raw []byte, name, media string) string {
	if b.err != nil {
		return ""
	}
	d := migration.Digest(raw)
	if _, ok := b.blobs[d]; ok {
		return d
	}
	if len(b.blobs) >= b.limits.Blobs || int64(len(raw)) > b.limits.BlobBytes || int64(len(raw)) > b.limits.AttachmentBytes-b.total {
		b.err = errors.New("Azure snapshot blob budget exceeded")
		return ""
	}
	b.total += int64(len(raw))
	b.blobs[d] = append([]byte{}, raw...)
	b.s.Blobs = append(b.s.Blobs, migration.Blob{Digest: d, Size: int64(len(raw)), Name: name, MediaType: media})
	return d
}
func (b *azureBuilder) add(r migration.Record) {
	if b.err != nil {
		return
	}
	if len(b.s.Records) >= b.limits.Records {
		b.err = errors.New("Azure snapshot record budget exceeded")
		return
	}
	if r.Links == nil {
		r.Links = []migration.Link{}
	}
	if r.Blobs == nil {
		r.Blobs = []string{}
	}
	b.s.Records = append(b.s.Records, r)
}
func (b *azureBuilder) preserve(kind, id, summary string, raw []byte, links []migration.Link) {
	d := b.blob(raw, kind+"-"+id, "application/json")
	b.add(migration.Record{Key: b.key(kind, id), Revision: migration.Digest(raw), Schema: "hn.migration.preserved/1", Content: azureJSON(migration.PreservedContent{Schema: "hn.migration.preserved/1", Summary: summary, SourceBlob: d}), Blobs: []string{d}, Links: links})
}
func (a *azureMigration) observe(ctx context.Context, c *azureapi.Client) (azureapi.Project, azureapi.Repository, error) {
	l := a.Locator()
	p, e := c.GetProject(ctx, l.Organization, l.Project)
	if e != nil {
		return p, azureapi.Repository{}, e
	}
	r := azureapi.Repository{}
	if l.Repository != "" {
		r, e = c.GetRepository(ctx, l.Organization, p.ID, l.Repository)
		if e == nil && r.Project.ID != p.ID {
			e = errors.New("Azure repository belongs to a different project")
		}
	}
	return p, r, e
}
func (a *azureMigration) baseline(p azureapi.Project, r azureapi.Repository) string {
	return migration.Digest(azureJSON([]string{a.Locator().Kind, a.Locator().Endpoint, a.Locator().Organization, a.Locator().Project, a.Locator().Repository, a.config.APIVersion, p.ID, p.ProcessID(), r.ID, r.Project.ID}))[7:]
}
func (a *azureMigration) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	if e := l.Validate(); e != nil {
		return migration.Snapshot{}, nil, e
	}
	if len(sel.Refs) > 0 && a.Locator().Repository == "" {
		return migration.Snapshot{}, nil, errors.New("Azure code refs require explicit repository selection")
	}
	normalized, e := migration.NormalizeInstance(sel.Source.Endpoint)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	sel.Source.Endpoint = normalized
	if sel.Source != a.Locator() {
		return migration.Snapshot{}, nil, errors.New("Azure source locator mismatch")
	}
	c := a.client.BoundedReadClient(l.ManifestBytes + l.AttachmentBytes)
	p, repo, e := a.observe(ctx, c)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	b := azureBuilder{s: migration.Snapshot{Schema: "hn.project-snapshot/1", Source: a.Locator(), SourceID: "azure:" + migration.Digest(azureJSON([]string{a.Locator().Endpoint, a.Locator().Organization, p.ID, repo.ID}))[7:], Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}, blobs: azureBlobs{}, limits: l, project: p.ID}
	want := func(kind string) bool {
		if len(sel.Kinds) == 0 {
			return true
		}
		for _, k := range sel.Kinds {
			if k == kind {
				return true
			}
		}
		return false
	}
	finding := func(id, status, message string) {
		b.s.Findings = append(b.s.Findings, azureFinding("azure-source-"+id, status, id, message))
	}
	knownKinds := map[string]bool{"work_item": true, "comment": true, "project": true, "change_request": true, "review": true, "inline_comment": true, "reply": true, "workflow": true, "attachment": true}
	for _, kind := range sel.Kinds {
		if !knownKinds[kind] {
			finding("unavailable-"+kind, "unsupported", "Azure profile does not implement selected resource kind "+kind)
		}
	}
	if want("project") {
		b.preserve("project", p.ID, "Retrieved Azure planning project/process metadata; inert observation", azureJSON(p), nil)
		finding("project", "preserved", "Project metadata retrieved; no planning/permission authority adopted")
	}
	if want("work_item") || want("comment") {
		rows, err := c.ListWorkItems(ctx, a.Locator().Organization, p.ID, l.Records)
		if azureUnavailable(err) {
			finding("work_item", "missing_dependency", "Work Items inaccessible; unread content was not preserved")
		} else if err != nil {
			return migration.Snapshot{}, nil, err
		} else {
			for _, v := range rows {
				if e = v.Validate(p); e != nil {
					return migration.Snapshot{}, nil, e
				}
				id := azureID(v.ID)
				raw := v.Raw
				d := b.blob(raw, "work-item-"+id, "application/json")
				content := migration.IssueContent{Schema: "hn.migration.issue/1", Title: v.Field("System.Title"), Body: v.Field("System.Description"), State: v.Field("System.State"), Type: v.Field("System.WorkItemType"), Labels: []string{}, Assignees: []string{}}
				if tags := v.Field("System.Tags"); tags != "" {
					seen := map[string]bool{}
					for _, tag := range strings.Split(tags, ";") {
						tag = strings.TrimSpace(tag)
						if tag != "" && !seen[tag] {
							content.Labels = append(content.Labels, tag)
							seen[tag] = true
						}
					}
				}
				if assigned := v.Actor("System.AssignedTo"); assigned != "" {
					content.Assignees = append(content.Assignees, assigned)
				}
				if _, err := migration.CanonicalContent("work_item", azureJSON(content)); err != nil {
					b.preserve("project", "work-item-source:"+id, "Work Item exceeds canonical field limits; exact source retained", raw, nil)
					finding("work_item-"+id, "unsupported", "Work Item exceeds canonical representation; source retained without truncation")
					if want("comment") {
						finding("comments-"+id, "missing_dependency", "Comments not read because their Work Item parent exceeds the canonical representation; unread comments are not preserved")
					}
					continue
				}
				b.add(migration.Record{Key: b.key("work_item", id), Selector: id, Revision: strconv.Itoa(v.Rev), Author: v.Actor("System.CreatedBy"), CreatedAt: v.Field("System.CreatedDate"), UpdatedAt: v.Field("System.ChangedDate"), Schema: content.Schema, Content: azureJSON(content), Blobs: []string{d}})
				// All original relation attributes and custom fields remain in the raw blob.
				// Explicit graph links are only admitted when their targets were retrieved.
				for _, rel := range v.Relations {
					var r struct {
						Rel string `json:"rel"`
						URL string `json:"url"`
					}
					if json.Unmarshal(rel, &r) == nil && r.Rel != "" {
						finding("relation-"+id+"-"+migration.Digest(rel)[7:], "preserved", "Retrieved relation/attachment descriptor retained in original Work Item blob; URL is not downloaded bytes")
					}
				}
				if want("comment") {
					comments, err := c.ListComments(ctx, a.Locator().Organization, p.ID, v.ID, l.Records-len(b.s.Records))
					if azureUnavailable(err) {
						finding("comments-"+id, "missing_dependency", "Work Item comments inaccessible; unread comments were not preserved")
						continue
					}
					if err != nil {
						return migration.Snapshot{}, nil, err
					}
					for _, comment := range comments {
						cid := id + ":" + azureID(comment.Key())
						cd := b.blob(comment.Raw, "comment-"+cid, "application/json")
						links := []migration.Link{{Relation: "work_item", Target: b.key("work_item", id)}}
						cc := migration.CommentContent{Schema: "hn.migration.comment/1", Body: comment.Text}
						if comment.IsDeleted || len(comment.Text) > 65536 || comment.Text == "" {
							b.preserve("reply", cid, "Deleted/oversized WIT comment retained as inert source", comment.Raw, links)
							continue
						}
						b.add(migration.Record{Key: b.key("comment", cid), Selector: azureID(comment.Key()), Revision: strconv.Itoa(comment.Version), Author: comment.CreatedBy.DisplayName, CreatedAt: comment.CreatedDate, UpdatedAt: comment.ModifiedDate, Schema: cc.Schema, Content: azureJSON(cc), Links: links, Blobs: []string{cd}})
					}
				}
			}
			finding("work_item", "preserved", "Complete selected WIQL/detail observation retrieved with explicit work-item IDs, revisions, original fields and relations")
			if want("comment") {
				finding("comment", "preserved", "Available comments retrieved with complete continuation-token enumeration; individual unavailable parents reported separately")
			}
		}
	}
	if want("change_request") || want("review") || want("inline_comment") || want("reply") {
		if repo.ID == "" {
			finding("pullrequests", "missing_dependency", "Explicit repository selection required for PR conversations")
		} else {
			prs, err := c.ListPullRequests(ctx, a.Locator().Organization, p.ID, repo.ID, l.Records-len(b.s.Records))
			if azureUnavailable(err) {
				finding("pullrequests", "missing_dependency", "PR collection inaccessible; unread PRs not preserved")
			} else if err != nil {
				return migration.Snapshot{}, nil, err
			} else {
				for _, raw := range prs {
					var pr struct {
						ID int `json:"pullRequestId"`
					}
					_ = json.Unmarshal(raw, &pr)
					pid := repo.ID + ":" + azureID(pr.ID)
					pk := b.key("change_request", pid)
					b.preserve("change_request", pid, "Retrieved Azure PR with repository and ref/commit context; inert", raw, nil)
					threads, err := c.ListThreads(ctx, a.Locator().Organization, p.ID, repo.ID, pr.ID, l.Records-len(b.s.Records))
					if azureUnavailable(err) {
						finding("threads-"+pid, "missing_dependency", "PR threads inaccessible; unread threads not preserved")
						continue
					}
					if err != nil {
						return migration.Snapshot{}, nil, err
					}
					for _, thread := range threads {
						var tr struct {
							ID            int               `json:"id"`
							ThreadContext json.RawMessage   `json:"threadContext"`
							Comments      []json.RawMessage `json:"comments"`
						}
						_ = json.Unmarshal(thread, &tr)
						tid := pid + ":" + azureID(tr.ID)
						tk := b.key("review", tid)
						b.preserve("review", tid, "Retrieved Azure thread including status, properties and inline/diff/iteration context; inert", thread, []migration.Link{{Relation: "change_request", Target: pk}})
						seen := map[int]bool{}
						for _, comment := range tr.Comments {
							var cm struct {
								ID       int `json:"id"`
								ParentID int `json:"parentCommentId"`
							}
							if json.Unmarshal(comment, &cm) != nil || cm.ID <= 0 || seen[cm.ID] {
								return migration.Snapshot{}, nil, errors.New("invalid or duplicate Azure PR thread comment")
							}
							seen[cm.ID] = true
							kind := "reply"
							if len(tr.ThreadContext) > 0 && string(tr.ThreadContext) != "null" {
								kind = "inline_comment"
							}
							links := []migration.Link{{Relation: "change_request", Target: pk}, {Relation: "thread", Target: tk}}
							if cm.ParentID > 0 {
								links = append(links, migration.Link{Relation: "reply_to", Target: b.key(kind, tid+":"+azureID(cm.ParentID))})
							}
							b.preserve(kind, tid+":"+azureID(cm.ID), "Retrieved PR thread comment; original attribution/inline parent context remains inert", comment, links)
						}
					}
				}
				finding("pullrequests", "preserved", "PRs and available threads/comments retrieved with explicit repository/PR/thread/reply topology")
			}
		}
	}
	if want("workflow") {
		if e = a.snapshotWorkflows(ctx, c, &b, p, repo, finding); e != nil {
			return migration.Snapshot{}, nil, e
		}
	}
	for i, attachment := range a.config.Attachments {
		parent := -1
		for j, r := range b.s.Records {
			if r.Key == attachment.Parent {
				parent = j
				break
			}
		}
		id := "attachment-" + strconv.Itoa(i)
		if parent < 0 {
			finding(id, "missing_dependency", "Selected attachment parent was not retrieved")
			continue
		}
		raw, media, err := c.Download(ctx, attachment.URL, l.BlobBytes)
		if azureUnavailable(err) {
			finding(id, "missing_dependency", "Selected attachment inaccessible; no downloaded bytes preserved")
			continue
		}
		if err != nil {
			return migration.Snapshot{}, nil, err
		}
		d := b.blob(raw, attachment.Name, media)
		if b.err == nil {
			seen := false
			for _, existing := range b.s.Records[parent].Blobs {
				if existing == d {
					seen = true
				}
			}
			if !seen {
				b.s.Records[parent].Blobs = append(b.s.Records[parent].Blobs, d)
			}
			finding(id, "preserved", "Selected attachment bytes retrieved and content-addressed; no native attachment recreation")
		}
	}
	if b.err != nil {
		return migration.Snapshot{}, nil, b.err
	}
	currentP, currentR, e := a.observe(ctx, c)
	if e != nil || a.baseline(p, repo) != a.baseline(currentP, currentR) {
		return migration.Snapshot{}, nil, errors.New("Azure project/repository identity changed during observation")
	}
	keys := map[string]bool{}
	for _, r := range b.s.Records {
		keys[r.Key.String()] = true
	}
	for i, r := range b.s.Records {
		links := []migration.Link{}
		for j, link := range r.Links {
			if keys[link.Target.String()] {
				links = append(links, link)
			} else {
				finding("link-"+strconv.Itoa(i)+"-"+strconv.Itoa(j), "missing_dependency", "Relationship target unavailable; original reference remains in source blob")
			}
		}
		b.s.Records[i].Links = links
	}
	raw, e := migration.CanonicalSnapshot(b.s, l)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	_ = json.Unmarshal(raw, &b.s)
	return b.s, b.blobs, nil
}

func (a *azureMigration) snapshotWorkflows(ctx context.Context, c *azureapi.Client, b *azureBuilder, p azureapi.Project, repo azureapi.Repository, finding func(string, string, string)) error {
	if len(a.config.PlanningSelectors) == 0 {
		finding("workflow", "missing_dependency", "No explicit yaml:/path or definition:ID@revision selected; unread definitions are not preserved")
		return nil
	}
	for i, selector := range a.config.PlanningSelectors {
		id := "workflow-" + strconv.Itoa(i)
		if strings.HasPrefix(selector, "yaml:") {
			if repo.ID == "" || !migration.ValidOID(a.config.WorkflowRef) || len(a.config.WorkflowRef) != 40 {
				finding(id, "requires_mapping", "YAML requires explicit repository, exact existing SHA1 commit and yaml:/path selection")
				continue
			}
			raw, media, e := c.ReadYAML(ctx, a.Locator().Organization, p.ID, repo.ID, a.config.WorkflowRef, strings.TrimPrefix(selector, "yaml:"), b.limits.BlobBytes)
			if azureUnavailable(e) {
				finding(id, "missing_dependency", "Selected YAML source inaccessible; not preserved")
				continue
			}
			if e != nil {
				return e
			}
			d := b.blob(raw, selector, media)
			key := repo.ID + ":" + a.config.WorkflowRef + ":" + strings.TrimPrefix(selector, "yaml:")
			b.add(migration.Record{Key: b.key("workflow", key), Selector: selector, Revision: a.config.WorkflowRef, Schema: "hn.migration.preserved/1", Content: azureJSON(migration.PreservedContent{Schema: "hn.migration.preserved/1", Summary: "Exact selected YAML source; inert data, never executed or adopted as policy", SourceBlob: d}), Blobs: []string{d}})
			finding(id, "preserved", "Selected exact YAML bytes retrieved; workflow execution and hosted authority not adopted")
		} else if strings.HasPrefix(selector, "definition:") {
			parts := strings.Split(strings.TrimPrefix(selector, "definition:"), "@")
			if len(parts) != 2 {
				return errors.New("Azure classic definition selector requires definition:ID@revision")
			}
			n, e := azureapi.PositiveID(parts[0])
			if e != nil {
				return e
			}
			rev, e := azureapi.PositiveID(parts[1])
			if e != nil {
				return e
			}
			raw, e := c.ReadDefinition(ctx, a.Locator().Organization, p.ID, n, rev)
			if azureUnavailable(e) {
				finding(id, "missing_dependency", "Selected classic definition inaccessible; not preserved")
				continue
			}
			if e != nil {
				return e
			}
			// The API normally masks secrets. Even an overprivileged/malformed response
			// cannot place a secret variable value into the snapshot archive.
			var object map[string]json.RawMessage
			if e = json.Unmarshal(raw, &object); e != nil {
				return e
			}
			if vars, ok := object["variables"]; ok {
				var values map[string]map[string]json.RawMessage
				if e = json.Unmarshal(vars, &values); e != nil {
					return e
				}
				for name, value := range values {
					var secret bool
					_ = json.Unmarshal(value["isSecret"], &secret)
					if secret {
						delete(value, "value")
						values[name] = value
					}
				}
				object["variables"] = azureJSON(values)
			}
			raw = azureJSON(object)
			b.preserve("workflow", "definition:"+parts[0]+"@"+parts[1], "Retrieved classic build definition at exact revision; secret variable values excluded; execution/queues/permissions remain inert", raw, nil)
			finding(id, "preserved", "Selected classic definition retrieved at exact revision; secret values excluded and execution/permission adoption unsupported")
		} else {
			return errors.New("Azure workflow selector must be yaml:/path or definition:ID@revision")
		}
	}
	return nil
}

// Intent IDs carry authenticated baseline values within the frozen schema:
// az1.<project/repository/process baseline>.<type vocabulary digest>.<source key>.
// The engine authenticates the plan/intents; every effect re-observes these
// immutable identities and the current type vocabulary before a single write.
func (a *azureMigration) intentID(p azureapi.Project, r azureapi.Repository, t azureapi.WorkItemType, key migration.Key) string {
	return "az1." + a.baseline(p, r) + "." + migration.Digest(azureJSON(t))[7:] + "." + migration.Digest([]byte(key.String()))[7:]
}
func azureTypeState(t azureapi.WorkItemType, state string) bool {
	for _, v := range t.States {
		if v.Name == state {
			return true
		}
	}
	return false
}

const azureMarkerPrefix = "\n\n[hn-migration:"

func azureMarker(ac migration.ApplyContext) string {
	w := ac.BoundWrite()
	return azureMarkerPrefix + w.PlanDigest + ":" + w.IntentID + ":" + w.PayloadDigest + "]"
}
func azureBodyBudget(id string) int {
	return 65536 - len(azureMarkerPrefix) - 2*len(migration.Digest(nil)) - len(id) - 3
}
func (a *azureMigration) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if sel.Destination != a.Locator() {
		return nil, nil, errors.New("Azure destination locator mismatch")
	}
	if _, e := migration.CanonicalSnapshot(s, l); e != nil {
		return nil, nil, e
	}
	c := a.client.BoundedReadClient(azureapi.MaxTotalBytes)
	p, repo, e := a.observe(ctx, c)
	if e != nil {
		return nil, nil, e
	}
	ins := []migration.Intent{}
	fs := []migration.Finding{}
	parents := map[string]migration.Intent{}
	finding := func(r migration.Record, status, message string) {
		fs = append(fs, azureFinding("azure-destination-"+migration.Digest([]byte(r.Key.String() + ":" + message))[7:], status, r.Key.String(), message))
	}
	for _, r := range s.Records {
		if len(r.Blobs) > 0 {
			finding(r, "unsupported", "Original fields, relations and retrieved blobs remain inspectable in saved snapshot; no custom-field/attachment recreation")
		}
	}
	for _, r := range s.Records {
		if r.Key.Kind != "work_item" && r.Key.Kind != "issue" {
			continue
		}
		var v migration.IssueContent
		_ = json.Unmarshal(r.Content, &v)
		typ, typeOK := sel.Mappings["type:"+v.Type]
		state, stateOK := sel.Mappings["state:"+v.State]
		if !azureapi.ValidGUID(p.ProcessID()) || sel.Mappings["process"] != p.ProcessID() || !typeOK || !stateOK || typ == "" || state == "" {
			finding(r, "requires_mapping", "Azure requires explicit destination process GUID and source type/state mappings; no universal open/closed assumption")
			continue
		}
		t, e := c.GetType(ctx, a.Locator().Organization, p.ID, typ)
		if azureUnavailable(e) {
			finding(r, "requires_mapping", "Mapped destination Work Item type is unavailable in selected process")
			continue
		}
		if e != nil {
			return nil, nil, e
		}
		if !azureTypeState(t, state) {
			finding(r, "requires_mapping", "Mapped state is not part of the observed destination Work Item type/process")
			continue
		}
		omitted := true
		for _, group := range []struct {
			prefix string
			values []string
		}{{"label:", v.Labels}, {"assignee:", v.Assignees}} {
			for _, value := range group.values {
				mapped, ok := sel.Mappings[group.prefix+value]
				if !ok || mapped != "" {
					omitted = false
				}
			}
		}
		if !omitted {
			finding(r, "requires_mapping", "Azure label/assignee destination recreation is unsupported; each source value requires an explicit empty omission mapping")
			continue
		}
		w := migration.IssueWrite{Schema: "hn.migration.issue-write/1", Title: v.Title, Body: v.Body, Type: typ, State: state, Labels: []string{}, Assignees: []string{}}
		in := migration.Intent{ID: a.intentID(p, repo, t, r.Key), Kind: "work_item", Action: "create", Sources: []migration.Key{r.Key}, DependsOn: []string{}}
		if len(w.Title) > 255 || len(w.Body) > azureBodyBudget(in.ID) {
			finding(r, "unsupported", "Title/description plus exact recovery marker exceeds Azure adapter capacity; source retained without truncation")
			continue
		}
		if target := sel.Mappings["target:"+r.Key.String()]; target != "" {
			n, e := azureapi.PositiveID(target)
			if e != nil {
				return nil, nil, e
			}
			current, e := c.GetWorkItem(ctx, a.Locator().Organization, p.ID, n)
			if e != nil {
				return nil, nil, e
			}
			if e = current.Validate(p); e != nil {
				return nil, nil, e
			}
			if current.Field("System.WorkItemType") != typ {
				finding(r, "requires_mapping", "Existing Work Item type differs; type conversion is not supported")
				continue
			}
			in.Action = "update"
			in.ExpectedID = azureRemoteID(p.ID, current.ID)
			in.ExpectedRevision = strconv.Itoa(current.Rev)
			w.Target = &migration.RemoteRef{ID: target}
		}
		in.Payload = azureJSON(w)
		if _, e = migration.CanonicalWrite(in, false); e != nil {
			return nil, nil, e
		}
		ins = append(ins, in)
		parents[r.Key.String()] = in
		finding(r, "mapped", "Explicit process/type/state mapped to Azure title/description; deterministic plain-text recovery marker appended; original source attribution remains in snapshot")
	}
	for _, r := range s.Records {
		if r.Key.Kind == "work_item" || r.Key.Kind == "issue" {
			continue
		}
		if r.Key.Kind != "comment" {
			finding(r, "unsupported", "Retrieved foreign "+r.Key.Kind+" remains inert and inspectable in snapshot; no Azure destination recreation")
			continue
		}
		var parent migration.Intent
		for _, link := range r.Links {
			if link.Relation == "work_item" || link.Relation == "issue" {
				parent = parents[link.Target.String()]
			}
		}
		if parent.ID == "" {
			finding(r, "missing_dependency", "Comment requires a mapped Work Item predecessor")
			continue
		}
		var v migration.CommentContent
		_ = json.Unmarshal(r.Content, &v)
		in := migration.Intent{ID: a.intentID(p, repo, azureapi.WorkItemType{}, r.Key), Kind: "comment", Action: "create", Sources: []migration.Key{r.Key}, DependsOn: []string{parent.ID}}
		if len(v.Body) > azureBodyBudget(in.ID) {
			finding(r, "unsupported", "Comment plus exact recovery marker exceeds Azure adapter capacity; source retained without truncation")
			continue
		}
		in.Payload = azureJSON(migration.CommentWrite{Schema: "hn.migration.comment-write/1", Parent: migration.RemoteRef{Receipt: &migration.ReceiptRef{IntentID: parent.ID, Field: "remote_key"}}, Body: v.Body})
		ins = append(ins, in)
		finding(r, "mapped", "Comment parent resolved and persisted by migration engine from confirmed Work Item receipt; recovery marker appended")
	}
	return ins, fs, nil
}
func azureRemoteID(project string, id int) string {
	return "work_item:" + project + ":" + strconv.Itoa(id)
}
func (a *azureMigration) bound(in migration.Intent, ac migration.ApplyContext) (migration.Intent, error) {
	w := ac.BoundWrite()
	if w.IntentID != in.ID || !migration.ValidDigest(w.PlanDigest) || w.PayloadDigest != migration.Digest(w.Payload) {
		return in, errors.New("Azure write context does not bind intent")
	}
	in.Payload = w.Payload
	if _, e := migration.CanonicalWrite(in, true); e != nil {
		return in, e
	}
	if in.Kind != "work_item" && in.Kind != "comment" || in.Action != "create" && in.Action != "update" || in.Kind == "comment" && in.Action != "create" {
		return in, errors.New("unsupported Azure write")
	}
	return in, nil
}
func (a *azureMigration) check(ctx context.Context, c *azureapi.Client, in migration.Intent) (azureapi.Project, error) {
	parts := strings.Split(in.ID, ".")
	if len(parts) != 4 || parts[0] != "az1" {
		return azureapi.Project{}, errors.New("invalid Azure intent binding")
	}
	for _, part := range parts[1:] {
		if len(part) != 64 || strings.ToLower(part) != part {
			return azureapi.Project{}, errors.New("invalid Azure intent digest")
		}
		if _, e := hex.DecodeString(part); e != nil {
			return azureapi.Project{}, e
		}
	}
	p, r, e := a.observe(ctx, c)
	if e != nil {
		return p, e
	}
	if parts[1] != a.baseline(p, r) {
		return p, errors.New("Azure project/repository/process identity changed")
	}
	t := azureapi.WorkItemType{}
	if in.Kind == "work_item" {
		var w migration.IssueWrite
		_ = json.Unmarshal(in.Payload, &w)
		t, e = c.GetType(ctx, a.Locator().Organization, p.ID, w.Type)
		if e != nil {
			return p, e
		}
		if !azureTypeState(t, w.State) {
			return p, errors.New("Azure mapped state unavailable")
		}
	}
	if parts[2] != migration.Digest(azureJSON(t))[7:] {
		return p, errors.New("Azure Work Item type/process vocabulary changed")
	}
	return p, nil
}
func (a *azureMigration) parent(ctx context.Context, c *azureapi.Client, p azureapi.Project, in migration.Intent, w migration.CommentWrite, ac migration.ApplyContext) (int, error) {
	n, e := azureapi.PositiveID(w.Parent.ID)
	if e != nil {
		return 0, e
	}
	expected := ""
	for _, id := range in.DependsOn {
		r, ok := ac.Dependency(id)
		if !ok || r.Destination != a.Locator() {
			return 0, errors.New("Azure comment dependency unconfirmed")
		}
		if r.Kind == "work_item" && r.RemoteKey == w.Parent.ID {
			if expected != "" {
				return 0, errors.New("ambiguous Azure comment parent")
			}
			expected = r.RemoteID
		}
	}
	if expected == "" || expected != azureRemoteID(p.ID, n) {
		return 0, errors.New("Azure comment parent is not confirmed immutable identity")
	}
	current, e := c.GetWorkItem(ctx, a.Locator().Organization, p.ID, n)
	if e != nil {
		return 0, e
	}
	if e = current.Validate(p); e != nil {
		return 0, e
	}
	return n, nil
}
func azureInput(w migration.IssueWrite, ac migration.ApplyContext) (azureapi.WorkItemInput, error) {
	if len(w.Labels) != 0 || len(w.Assignees) != 0 || w.Type == "" || w.State == "" {
		return azureapi.WorkItemInput{}, errors.New("Azure requires mapped type/state and explicit label/assignee omission")
	}
	body := w.Body + azureMarker(ac)
	if len(body) > 65536 || len(w.Title) > 255 {
		return azureapi.WorkItemInput{}, errors.New("Azure write exceeds capacity")
	}
	return azureapi.WorkItemInput{Title: w.Title, Body: body, State: w.State, Type: w.Type, Labels: []string{}, Assignees: []string{}}, nil
}
func azureMatches(v azureapi.WorkItem, w azureapi.WorkItemInput, p azureapi.Project) bool {
	return v.Validate(p) == nil && v.Field("System.Title") == w.Title && v.Field("System.Description") == w.Body && v.Field("System.State") == w.State && v.Field("System.WorkItemType") == w.Type
}
func (a *azureMigration) receipt(in migration.Intent, ac migration.ApplyContext, id, key, rev string) migration.Receipt {
	r := migrationReceipt(in, ac, a.Locator(), id, rev)
	r.RemoteKey = key
	return r
}
func (a *azureMigration) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	in, e := a.bound(in, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	c := a.client.BoundedReadClient(azureapi.MaxTotalBytes)
	p, e := a.check(ctx, c, in)
	if e != nil {
		return migration.Receipt{}, e
	}
	if in.Kind == "work_item" {
		var w migration.IssueWrite
		_ = json.Unmarshal(in.Payload, &w)
		input, e := azureInput(w, ac)
		if e != nil {
			return migration.Receipt{}, e
		}
		n, rev := 0, 0
		if in.Action == "update" {
			n, e = azureapi.PositiveID(w.Target.ID)
			if e != nil {
				return migration.Receipt{}, e
			}
			current, e := c.GetWorkItem(ctx, a.Locator().Organization, p.ID, n)
			if e != nil {
				return migration.Receipt{}, e
			}
			if current.Validate(p) != nil || in.ExpectedID != azureRemoteID(p.ID, current.ID) || in.ExpectedRevision != strconv.Itoa(current.Rev) {
				return migration.Receipt{}, errors.New("Azure Work Item revision/identity changed")
			}
			rev = current.Rev
		}
		v, e := c.WriteWorkItem(ctx, a.Locator().Organization, p.ID, n, rev, input)
		if e != nil {
			return migration.Receipt{}, e
		}
		if !azureMatches(v, input, p) || in.ExpectedID != "" && in.ExpectedID != azureRemoteID(p.ID, v.ID) {
			return migration.Receipt{}, errors.New("Azure returned different Work Item projection; recovery required")
		}
		return a.receipt(in, ac, azureRemoteID(p.ID, v.ID), azureID(v.ID), strconv.Itoa(v.Rev)), nil
	}
	var w migration.CommentWrite
	_ = json.Unmarshal(in.Payload, &w)
	n, e := a.parent(ctx, c, p, in, w, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	body := w.Body + azureMarker(ac)
	if len(body) > 65536 {
		return migration.Receipt{}, errors.New("Azure comment exceeds marker capacity")
	}
	v, e := c.CreateComment(ctx, a.Locator().Organization, p.ID, n, body)
	if e != nil {
		return migration.Receipt{}, e
	}
	if v.Text != body || v.IsDeleted {
		return migration.Receipt{}, errors.New("Azure returned different comment; recovery required")
	}
	return a.receipt(in, ac, "comment:"+p.ID+":"+azureID(n)+":"+azureID(v.Key()), azureID(v.Key()), strconv.Itoa(v.Version)), nil
}
func (a *azureMigration) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	in, e := a.bound(in, ac)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	c := a.client.BoundedReadClient(azureapi.MaxTotalBytes)
	p, e := a.check(ctx, c, in)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	matches := []migration.Receipt{}
	if in.Kind == "work_item" {
		var w migration.IssueWrite
		_ = json.Unmarshal(in.Payload, &w)
		input, e := azureInput(w, ac)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		var rows []azureapi.WorkItem
		if in.Action == "update" {
			n, e := azureapi.PositiveID(w.Target.ID)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			v, e := c.GetWorkItem(ctx, a.Locator().Organization, p.ID, n)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			rows = []azureapi.WorkItem{v}
		} else {
			rows, e = c.ListWorkItems(ctx, a.Locator().Organization, p.ID, 20000)
			if e != nil {
				return migration.Reconciliation{}, e
			}
		}
		for _, v := range rows {
			if azureMatches(v, input, p) && (in.ExpectedID == "" || in.ExpectedID == azureRemoteID(p.ID, v.ID)) {
				matches = append(matches, a.receipt(in, ac, azureRemoteID(p.ID, v.ID), azureID(v.ID), strconv.Itoa(v.Rev)))
			}
		}
	} else {
		var w migration.CommentWrite
		_ = json.Unmarshal(in.Payload, &w)
		n, e := a.parent(ctx, c, p, in, w, ac)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		rows, e := c.ListComments(ctx, a.Locator().Organization, p.ID, n, 20000)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		for _, v := range rows {
			if !v.IsDeleted && v.Text == w.Body+azureMarker(ac) {
				matches = append(matches, a.receipt(in, ac, "comment:"+p.ID+":"+azureID(n)+":"+azureID(v.Key()), azureID(v.Key()), strconv.Itoa(v.Version)))
			}
		}
	}
	if len(matches) == 1 {
		return migration.Reconciliation{Status: "exact", Receipt: &matches[0], Reason: "Unique exact bound recovery marker and full intended projection observed"}, nil
	}
	if len(matches) > 1 {
		return migration.Reconciliation{Status: "ambiguous", Reason: "Multiple exact matches; manual recovery required"}, nil
	}
	return migration.Reconciliation{Status: "no_proof", Reason: "No exact positive proof; absence never authorizes another create"}, nil
}
