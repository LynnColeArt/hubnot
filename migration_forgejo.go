//go:build !noforge

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"

	"hubnot/internal/forgejoapi"
	"hubnot/internal/migration"
)

func init() {
	registerV1Migration("forgejo", "HN_FORGEJO_TOKEN", func(c migration.ProviderConfig) (*v1Migration, error) { return newForgejoMigration(c) })
}
func registerV1Migration(kind, env string, newAdapter func(migration.ProviderConfig) (*v1Migration, error)) {
	e := migrationProviders.Register(migration.ProviderRegistration{Kind: kind, TokenEnv: env, RequiredCoverage: []migration.CoverageRequirement{{Kind: "issue", Read: true, Preserve: true, Create: true, Update: true}, {Kind: "comment", Read: true, Preserve: true, Create: true, Update: true}}, NewSource: func(c migration.ProviderConfig) (migration.Source, error) { return newAdapter(c) }, NewDestination: func(c migration.ProviderConfig) (migration.Destination, error) { return newAdapter(c) }})
	if e != nil {
		panic(e)
	}
}
func newForgejoMigration(c migration.ProviderConfig) (*v1Migration, error) {
	if c.Locator.Kind != "forgejo" {
		return nil, errors.New("Forgejo profile requires forgejo provider identity")
	}
	version := c.APIVersion
	if version == "" {
		version = "16"
	}
	client, e := forgejoapi.NewProfileClient(c.Locator.Endpoint, c.Token, "forgejo-"+version)
	if e != nil {
		return nil, e
	}
	return newV1Migration(c, client)
}

// v1Migration implements only the explicitly compared common endpoints. Profile
// construction/version checks and workflow directories stay transport-owned.
// No destination IDs or credentials are recovered from ambient process state.
type v1Migration struct {
	config migration.ProviderConfig
	client *forgejoapi.Client
}

func newV1Migration(c migration.ProviderConfig, client *forgejoapi.Client) (*v1Migration, error) {
	if c.Locator.Organization != "" || c.Locator.Project != "" {
		return nil, errors.New("repository and planning selectors must remain separate")
	}
	if c.CredentialMode != "" && c.CredentialMode != "token" {
		return nil, errors.New("profile requires explicitly supplied token authentication")
	}
	if e := forgejoapi.ValidateRepository(c.Locator.Repository); e != nil {
		return nil, e
	}
	c.Locator.Endpoint = client.BaseURL()
	c.PlanningSelectors = append([]string{}, c.PlanningSelectors...)
	c.Attachments = append([]migration.AttachmentSelection{}, c.Attachments...)
	return &v1Migration{c, client}, nil
}
func (g *v1Migration) Locator() migration.Locator { return g.config.Locator }
func (g *v1Migration) Capabilities(context.Context) ([]migration.Capability, error) {
	limits := []string{g.client.Profile() + "; explicit /api/v1; supplied token; instance scopes may refuse reads or writes", "one write per intent; no native user, permission, approval or workflow authority", "labels map to existing numeric IDs; updates require unchanged labels; types require explicit mapping to empty", "255-byte titles; 65536-byte body including recovery marker; 100 pages, 50 items/page, 4 MiB response, 64 MiB snapshot wire ceiling", "source authors/times retained as attribution; code refs use shared Git composition"}
	out := []migration.Capability{{Kind: "issue", Read: true, Preserve: true, Create: true, Update: true, Limits: limits}, {Kind: "comment", Read: true, Preserve: true, Create: true, Update: true, Limits: []string{"confirmed immutable issue parent; existing comment identity and revision checked before update"}}}
	for _, kind := range []string{"change_request", "review", "inline_comment", "reply", "workflow", "attachment"} {
		out = append(out, migration.Capability{Kind: kind, Read: true, Preserve: true, Limits: []string{"retrieved source only; no destination recreation", "selected same-origin assets; pinned workflow bytes remain inert; unavailable endpoints reported"}})
	}
	for _, kind := range []string{"project", "project_item", "discussion"} {
		out = append(out, migration.Capability{Kind: kind, Reason: g.client.Profile() + " has no qualified board/project or standalone discussion export API; UI availability and issue project IDs do not establish an export contract"})
	}
	return out, nil
}
func v1JSON(v any) []byte     { b, _ := json.Marshal(v); return b }
func v1ID(id int64) string    { return "id:" + strconv.FormatInt(id, 10) }
func v1Revision(v any) string { return migration.Digest(v1JSON(v)) }
func v1IssueRevision(v forgejoapi.Issue) string {
	if len(v.Raw) > 0 {
		return migration.Digest(v.Raw)
	}
	return v1Revision(v)
}
func v1CommentRevision(v forgejoapi.Comment) string {
	if len(v.Raw) > 0 {
		return migration.Digest(v.Raw)
	}
	return v1Revision(v)
}
func v1RepoRevision(r forgejoapi.Repository) string {
	return v1Revision([]string{v1ID(r.ID), r.CreatedAt})
}
func v1Unavailable(e error) bool {
	var h *forgejoapi.HTTPError
	return errors.As(e, &h) && (h.StatusCode == 401 || h.StatusCode == 403 || h.StatusCode == 404 || h.StatusCode == 410)
}
func v1Finding(id, status, p, message string) migration.Finding {
	return migration.Finding{ID: id, Status: status, Path: p, Message: message}
}

type v1Blobs map[string][]byte

func (b v1Blobs) Open(ctx context.Context, id string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	v, ok := b[id]
	if !ok {
		return nil, errors.New("blob absent from snapshot")
	}
	return io.NopCloser(bytes.NewReader(v)), nil
}

type v1Snapshot struct {
	s       migration.Snapshot
	b       v1Blobs
	l       migration.Limits
	project string
	total   int64
	err     error
}

func (b *v1Snapshot) key(kind, id string) migration.Key {
	return migration.Key{Provider: b.s.Source.Kind, Instance: b.s.Source.Endpoint, ProjectID: b.project, Kind: kind, ID: id}
}
func (b *v1Snapshot) blob(raw []byte, name, media string) string {
	if b.err != nil {
		return ""
	}
	id := migration.Digest(raw)
	if _, ok := b.b[id]; ok {
		return id
	}
	if len(b.b) >= b.l.Blobs || int64(len(raw)) > b.l.BlobBytes || int64(len(raw)) > b.l.AttachmentBytes-b.total {
		b.err = errors.New("forge snapshot blob budget exceeded")
		return ""
	}
	b.total += int64(len(raw))
	b.b[id] = append([]byte{}, raw...)
	b.s.Blobs = append(b.s.Blobs, migration.Blob{Digest: id, Size: int64(len(raw)), Name: name, MediaType: media})
	return id
}
func (b *v1Snapshot) add(r migration.Record) {
	if b.err != nil {
		return
	}
	if len(b.s.Records) >= b.l.Records {
		b.err = errors.New("forge snapshot record budget exceeded")
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
func (b *v1Snapshot) finding(id, status, p, message string) {
	b.s.Findings = append(b.s.Findings, v1Finding(b.s.Source.Kind+"-source-"+id, status, p, message))
}
func (b *v1Snapshot) preserve(kind, id, selector string, raw []byte, links []migration.Link) migration.Key {
	key := b.key(kind, id)
	var meta struct {
		HTMLURL   string          `json:"html_url"`
		User      forgejoapi.User `json:"user"`
		Created   string          `json:"created_at"`
		Updated   string          `json:"updated_at"`
		Submitted string          `json:"submitted_at"`
	}
	_ = json.Unmarshal(raw, &meta)
	if meta.Created == "" {
		meta.Created = meta.Submitted
	}
	d := b.blob(raw, kind+"-"+id, "application/json")
	b.add(migration.Record{Key: key, Selector: selector, Revision: migration.Digest(raw), SourceURL: meta.HTMLURL, Author: meta.User.Login, CreatedAt: meta.Created, UpdatedAt: meta.Updated, Schema: "hn.migration.preserved/1", Content: v1JSON(migration.PreservedContent{Schema: "hn.migration.preserved/1", Summary: "Retrieved " + b.s.Source.Kind + " " + kind + "; foreign source remains inert", SourceBlob: d}), Links: links, Blobs: []string{d}})
	b.finding("preserve-"+migration.Digest([]byte(key.String()))[7:], "preserved", key.String(), "Exact retrieved source bytes retained without native authority")
	return key
}
func (g *v1Migration) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	if e := l.Validate(); e != nil {
		return migration.Snapshot{}, nil, e
	}
	source := sel.Source
	normalized, e := forgejoapi.NewProfileClient(source.Endpoint, "", g.client.Profile())
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	source.Endpoint = normalized.BaseURL()
	if source != g.Locator() {
		return migration.Snapshot{}, nil, errors.New("source selection differs from configured profile")
	}
	c := g.client.BoundedReadClient(l.ManifestBytes + l.AttachmentBytes)
	if e = c.CheckVersion(ctx); e != nil {
		return migration.Snapshot{}, nil, e
	}
	repo, e := c.GetRepository(ctx, g.Locator().Repository)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	b := v1Snapshot{s: migration.Snapshot{Schema: "hn.project-snapshot/1", Source: g.Locator(), SourceID: g.Locator().Kind + ":" + migration.Digest(v1JSON([]string{g.Locator().Endpoint, v1ID(repo.ID)}))[7:], Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}, b: v1Blobs{}, l: l, project: v1ID(repo.ID)}
	want := func(k string) bool {
		if len(sel.Kinds) == 0 {
			return true
		}
		for _, v := range sel.Kinds {
			if v == k {
				return true
			}
		}
		return false
	}
	unavailable := func(id string, e error) error {
		if v1Unavailable(e) {
			b.finding(id, "missing_dependency", id, "Resource private, deleted, disabled or inaccessible; unread content is not preserved")
			return nil
		}
		return e
	}
	if want("issue") || want("comment") || want("change_request") || want("review") || want("inline_comment") || want("reply") {
		rows, err := c.ListIssueRows(ctx, g.Locator().Repository)
		if err != nil {
			if e = unavailable("issues", err); e != nil {
				return migration.Snapshot{}, nil, e
			}
		} else {
			for _, v := range rows {
				var parent migration.Key
				if v.IsPull() {
					if !(want("change_request") || want("review") || want("inline_comment") || want("reply")) {
						continue
					}
					pr, err := c.ReadPull(ctx, g.Locator().Repository, v.Number)
					if err != nil {
						if e = unavailable("pull-"+strconv.Itoa(v.Number), err); e != nil {
							return migration.Snapshot{}, nil, e
						}
						continue
					}
					parent = b.preserve("change_request", v1ID(pr.ID), strconv.Itoa(v.Number), pr.Raw, nil)
					if want("review") || want("inline_comment") {
						reviews, err := c.ListReviews(ctx, g.Locator().Repository, v.Number)
						if err != nil {
							if e = unavailable("reviews-"+strconv.Itoa(v.Number), err); e != nil {
								return migration.Snapshot{}, nil, e
							}
						} else {
							for _, review := range reviews {
								rk := b.preserve("review", v1ID(review.ID), strconv.FormatInt(review.ID, 10), review.Raw, []migration.Link{{Relation: "change_request", Target: parent}})
								if want("inline_comment") {
									comments, err := c.ListReviewComments(ctx, g.Locator().Repository, v.Number, review.ID)
									if err != nil {
										if e = unavailable("review-comments-"+strconv.FormatInt(review.ID, 10), err); e != nil {
											return migration.Snapshot{}, nil, e
										}
									} else {
										for _, x := range comments {
											var meta struct {
												Review int64 `json:"pull_request_review_id"`
											}
											_ = json.Unmarshal(x.Raw, &meta)
											if meta.Review != review.ID {
												return migration.Snapshot{}, nil, errors.New("review comment parent identity differs")
											}
											b.preserve("inline_comment", v1ID(x.ID), strconv.FormatInt(x.ID, 10), x.Raw, []migration.Link{{Relation: "change_request", Target: parent}, {Relation: "review", Target: rk}})
										}
									}
								}
							}
						}
					}
				} else {
					if !want("issue") && !want("comment") {
						continue
					}
					labels := []string{}
					assignees := []string{}
					for _, x := range v.Labels {
						labels = append(labels, x.Name)
					}
					for _, x := range v.Assignees {
						assignees = append(assignees, x.Login)
					}
					content := v1JSON(migration.IssueContent{Schema: "hn.migration.issue/1", Title: v.Title, Body: v.Body, State: v.State, Labels: labels, Assignees: assignees})
					parent = b.key("issue", v1ID(v.ID))
					if _, err := migration.CanonicalContent("issue", content); err != nil {
						parent = b.preserve("attachment", "issue-source:"+v1ID(v.ID), strconv.Itoa(v.Number), v.Raw, nil)
						b.finding("issue-capacity-"+strconv.Itoa(v.Number), "unsupported", parent.String(), "Issue exceeds closed record schema; full source bytes retained")
					} else {
						raw := b.blob(v.Raw, "issue-"+v1ID(v.ID), "application/json")
						b.add(migration.Record{Key: parent, Selector: strconv.Itoa(v.Number), Revision: v1IssueRevision(v), SourceURL: v.HTMLURL, Author: v.User.Login, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Schema: "hn.migration.issue/1", Content: content, Blobs: []string{raw}})
					}
				}
				if want("comment") || v.IsPull() && want("reply") {
					comments, err := c.ListComments(ctx, g.Locator().Repository, v.Number)
					if err != nil {
						if e = unavailable("comments-"+strconv.Itoa(v.Number), err); e != nil {
							return migration.Snapshot{}, nil, e
						}
					} else {
						for _, x := range comments {
							content := v1JSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: x.Body})
							_, invalid := migration.CanonicalContent("comment", content)
							if parent.Kind != "issue" || invalid != nil {
								b.preserve("reply", v1ID(x.ID), strconv.FormatInt(x.ID, 10), x.Raw, []migration.Link{{Relation: "source_record", Target: parent}})
							} else {
								raw := b.blob(x.Raw, "comment-"+v1ID(x.ID), "application/json")
								b.add(migration.Record{Key: b.key("comment", v1ID(x.ID)), Selector: strconv.FormatInt(x.ID, 10), Revision: v1CommentRevision(x), SourceURL: x.HTMLURL, Author: x.User.Login, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, Schema: "hn.migration.comment/1", Content: content, Links: []migration.Link{{Relation: "issue", Target: parent}}, Blobs: []string{raw}})
							}
						}
					}
				}
				if b.err != nil {
					return migration.Snapshot{}, nil, b.err
				}
			}
		}
	}
	for _, kind := range []string{"project", "project_item", "discussion"} {
		if want(kind) || kind == "project" && len(g.config.PlanningSelectors) > 0 {
			b.finding(kind, "unsupported", kind, g.client.Profile()+" has no qualified project/board or standalone discussion export API; no native recreation claimed")
		}
	}
	if want("workflow") {
		ref := g.config.WorkflowRef
		if ref == "" {
			ref = repo.DefaultBranch
		}
		if ref == "" {
			b.finding("workflow", "missing_dependency", "workflow", "No workflow source revision selected")
		} else {
			files, err := c.ReadWorkflowDefinitions(ctx, g.Locator().Repository, ref)
			if err != nil {
				if e = unavailable("workflow", err); e != nil {
					return migration.Snapshot{}, nil, e
				}
			} else {
				if len(files) == 0 {
					b.finding("workflow-empty", "missing_dependency", "workflow", "No workflow definitions retrieved at the pinned revision; missing/inaccessible directories are not claimed preserved")
				}
				for _, f := range files {
					b.preserve("workflow", f.Commit+":"+f.Path, f.Path, v1JSON(struct {
						Commit string          `json:"commit"`
						Path   string          `json:"path"`
						Source json.RawMessage `json:"source"`
					}{f.Commit, f.Path, f.Raw}), nil)
					d := b.blob(f.Content, f.Path, "text/plain")
					if b.err == nil {
						b.s.Records[len(b.s.Records)-1].Blobs = append(b.s.Records[len(b.s.Records)-1].Blobs, d)
					}
				}
			}
		}
	}
	if len(g.config.Attachments) > l.Blobs {
		return migration.Snapshot{}, nil, errors.New("selected attachment count exceeds budget")
	}
	for i, a := range g.config.Attachments {
		parent := -1
		for j, r := range b.s.Records {
			if r.Key == a.Parent {
				parent = j
				break
			}
		}
		id := "attachment-" + strconv.Itoa(i)
		if parent < 0 {
			b.finding(id, "missing_dependency", id, "Selected asset parent not retrieved")
			continue
		}
		data, media, err := c.ReadAttachment(ctx, a.URL, l.BlobBytes)
		if err != nil {
			if v1Unavailable(err) || strings.Contains(err.Error(), "same-origin") {
				b.finding(id, "missing_dependency", id, "Selected asset inaccessible or outside credential origin; no bytes preserved")
				continue
			}
			return migration.Snapshot{}, nil, err
		}
		d := b.blob(data, a.Name, media)
		if b.err == nil {
			b.s.Records[parent].Blobs = append(b.s.Records[parent].Blobs, d)
			b.finding(id, "preserved", id, "Selected attachment bytes retrieved and digest-addressed")
		}
	}
	if b.err != nil {
		return migration.Snapshot{}, nil, b.err
	}
	current, e := c.GetRepository(ctx, g.Locator().Repository)
	if e != nil || v1RepoRevision(current) != v1RepoRevision(repo) {
		return migration.Snapshot{}, nil, errors.New("repository changed during observation")
	}
	raw, e := migration.CanonicalSnapshot(b.s, l)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	_ = json.Unmarshal(raw, &b.s)
	return b.s, b.b, nil
}

func (g *v1Migration) baseline(r forgejoapi.Repository) string {
	return migration.Digest(v1JSON([]string{g.Locator().Kind, g.Locator().Endpoint, g.Locator().Repository, g.client.Profile(), v1RepoRevision(r)}))[7:]
}
func (g *v1Migration) intent(r forgejoapi.Repository, key migration.Key) migration.Intent {
	return migration.Intent{ID: "fv1." + g.baseline(r) + "." + migration.Digest([]byte(key.String()))[7:], Sources: []migration.Key{key}, DependsOn: []string{}}
}
func (g *v1Migration) checkRepo(ctx context.Context, in migration.Intent) error {
	p := strings.Split(in.ID, ".")
	if len(p) != 3 || p[0] != "fv1" {
		return errors.New("invalid repository-bound intent")
	}
	for _, s := range p[1:] {
		if len(s) != 64 || s != strings.ToLower(s) {
			return errors.New("invalid intent digest")
		}
		if _, e := hex.DecodeString(s); e != nil {
			return e
		}
	}
	if e := g.client.CheckVersion(ctx); e != nil {
		return e
	}
	r, e := g.client.GetRepository(ctx, g.Locator().Repository)
	if e != nil {
		return e
	}
	if p[1] != g.baseline(r) {
		return errors.New("destination repository incarnation changed")
	}
	if !r.HasIssues {
		return errors.New("destination issue API disabled")
	}
	return nil
}
func v1Index(s string) (int, error) {
	n, e := strconv.Atoi(s)
	if e != nil || n <= 0 || strconv.Itoa(n) != s {
		return 0, errors.New("canonical positive resource index required")
	}
	return n, nil
}
func v1CommentID(s string) (int64, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n <= 0 || strconv.FormatInt(n, 10) != s {
		return 0, errors.New("canonical positive comment ID required")
	}
	return n, nil
}
func v1SameStrings(a, b []string) bool {
	a = append([]string{}, a...)
	b = append([]string{}, b...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
func v1Labels(v forgejoapi.Issue) []string {
	out := []string{}
	for _, l := range v.Labels {
		out = append(out, strconv.FormatInt(l.ID, 10))
	}
	return out
}
func v1BodyBudget(id string) int {
	return forgejoapi.BodyCapacity - len(v1MarkerPrefix) - 2*len(migration.Digest(nil)) - 2 - len(id) - len(v1MarkerSuffix)
}

const v1MarkerPrefix = "\n\n<!-- hn-migration:"
const v1MarkerSuffix = " -->"

func v1Marker(ac migration.ApplyContext) string {
	w := ac.BoundWrite()
	return v1MarkerPrefix + w.PlanDigest + ":" + w.IntentID + ":" + w.PayloadDigest + v1MarkerSuffix
}

func (g *v1Migration) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if sel.Destination != g.Locator() {
		return nil, nil, errors.New("destination locator mismatch")
	}
	if _, e := migration.CanonicalSnapshot(s, l); e != nil {
		return nil, nil, e
	}
	read := *g
	read.client = g.client.BoundedReadClient(l.ManifestBytes + l.AttachmentBytes)
	g = &read
	if e := g.client.CheckVersion(ctx); e != nil {
		return nil, nil, e
	}
	repo, e := g.client.GetRepository(ctx, g.Locator().Repository)
	if e != nil {
		return nil, nil, e
	}
	if !repo.HasIssues {
		return nil, nil, errors.New("destination issue API disabled; no writes planned")
	}
	ins := []migration.Intent{}
	fs := []migration.Finding{}
	parents := map[string]migration.Intent{}
	finding := func(r migration.Record, suffix, status, msg string) {
		fs = append(fs, v1Finding(g.Locator().Kind+"-destination-"+migration.Digest([]byte(r.Key.String()))[7:]+suffix, status, r.Key.String(), msg))
	}
	for _, r := range s.Records {
		if len(r.Blobs) > 0 {
			finding(r, "-blobs", "unsupported", "Original source/attachment bytes remain inspectable in the saved snapshot; no remote attachment upload or non-equivalent field recreation")
		}
	}
	for _, r := range s.Records {
		if r.Key.Kind != "issue" && r.Key.Kind != "work_item" {
			continue
		}
		var c migration.IssueContent
		_ = json.Unmarshal(r.Content, &c)
		if r.Key.Kind == "work_item" {
			_, state := sel.Mappings["state:"+c.State]
			_, typ := sel.Mappings["type:"+c.Type]
			if !state || !typ {
				finding(r, "", "requires_mapping", "Work-item conversion requires explicit source state and type mappings")
				continue
			}
		}
		state, typ := c.State, c.Type
		if v, ok := sel.Mappings["state:"+state]; ok {
			state = v
		}
		if v, ok := sel.Mappings["type:"+typ]; ok {
			typ = v
		}
		if (state != "open" && state != "closed") || typ != "" {
			finding(r, "", "requires_mapping", "Map source state to open/closed and source type to empty; profile does not administer issue types")
			continue
		}
		labels := []string{}
		assignees := []string{}
		missing := false
		for _, group := range []struct {
			prefix string
			values []string
			out    *[]string
		}{{"label:", c.Labels, &labels}, {"assignee:", c.Assignees, &assignees}} {
			for _, v := range group.values {
				mapped, ok := sel.Mappings[group.prefix+v]
				if !ok {
					missing = true
				} else if mapped != "" {
					*group.out = append(*group.out, mapped)
				}
			}
		}
		if missing {
			finding(r, "", "requires_mapping", "Explicit label/assignee mappings or deliberate empty omission mappings required")
			continue
		}
		labelIDs := map[int64]bool{}
		for _, v := range labels {
			id, err := v1CommentID(v)
			if err != nil || labelIDs[id] {
				return nil, nil, errors.New("label mappings require distinct positive numeric label IDs")
			}
			labelIDs[id] = true
		}
		if len(labels) > 0 {
			available, err := g.client.ListLabels(ctx, g.Locator().Repository)
			if err != nil {
				return nil, nil, err
			}
			for _, x := range available {
				delete(labelIDs, x.ID)
			}
			if len(labelIDs) > 0 {
				finding(r, "", "missing_dependency", "Mapped destination label is unavailable; label creation is not implicit")
				continue
			}
		}
		in := g.intent(repo, r.Key)
		in.Kind = "issue"
		in.Action = "create"
		w := migration.IssueWrite{Schema: "hn.migration.issue-write/1", Title: c.Title, Body: c.Body, State: state, Type: typ, Labels: labels, Assignees: assignees}
		if len(w.Title) > 255 || len(w.Body) > v1BodyBudget(in.ID) {
			finding(r, "", "unsupported", "Title/body plus deterministic recovery marker exceed qualified profile capacity; source retained without truncation")
			continue
		}
		if target := sel.Mappings["target:"+r.Key.String()]; target != "" {
			n, err := v1Index(target)
			if err != nil {
				return nil, nil, err
			}
			current, err := g.client.GetIssue(ctx, g.Locator().Repository, n)
			if err != nil {
				return nil, nil, err
			}
			if current.IsPull() {
				return nil, nil, errors.New("issue target is a pull request")
			}
			if g.client.Profile() == "gitea-1.27" && current.ContentVersion <= 0 {
				finding(r, "", "unsupported", "Gitea 1.27 issue update requires an observed optimistic content version")
				continue
			}
			if !v1SameStrings(v1Labels(current), labels) {
				finding(r, "", "unsupported", "Issue PATCH cannot change labels in one mutation; mapped labels must equal existing labels")
				continue
			}
			in.Action = "update"
			in.ExpectedID = v1ID(current.ID)
			in.ExpectedRevision = v1IssueRevision(current)
			w.Target = &migration.RemoteRef{ID: target}
		}
		in.Payload = v1JSON(w)
		if _, e = migration.CanonicalWrite(in, false); e != nil {
			return nil, nil, e
		}
		ins = append(ins, in)
		parents[r.Key.String()] = in
		finding(r, "", "mapped", "Issue projection mapped; API body includes deterministic recovery marker; original attribution retained locally")
	}
	for _, r := range s.Records {
		if r.Key.Kind == "issue" || r.Key.Kind == "work_item" {
			continue
		}
		if r.Key.Kind != "comment" {
			finding(r, "", "unsupported", "Retrieved foreign "+r.Key.Kind+" retained in saved snapshot; no destination-native recreation")
			continue
		}
		var parent migration.Intent
		for _, link := range r.Links {
			if link.Relation == "issue" || link.Relation == "work_item" {
				parent = parents[link.Target.String()]
			}
		}
		if parent.ID == "" {
			finding(r, "", "missing_dependency", "Comment needs a mapped issue parent")
			continue
		}
		var c migration.CommentContent
		_ = json.Unmarshal(r.Content, &c)
		in := g.intent(repo, r.Key)
		in.Kind = "comment"
		in.Action = "create"
		in.DependsOn = []string{parent.ID}
		w := migration.CommentWrite{Schema: "hn.migration.comment-write/1", Body: c.Body, Parent: migration.RemoteRef{Receipt: &migration.ReceiptRef{IntentID: parent.ID, Field: "remote_key"}}}
		if len(w.Body) > v1BodyBudget(in.ID) {
			finding(r, "", "unsupported", "Comment plus recovery marker exceeds qualified profile capacity; no truncation")
			continue
		}
		if target := sel.Mappings["target:"+r.Key.String()]; target != "" {
			id, err := v1CommentID(target)
			if err != nil {
				return nil, nil, err
			}
			var pw migration.IssueWrite
			_ = json.Unmarshal(parent.Payload, &pw)
			if pw.Target == nil {
				finding(r, "", "missing_dependency", "Comment update requires an explicitly mapped existing issue parent")
				continue
			}
			n, err := v1Index(pw.Target.ID)
			if err != nil {
				return nil, nil, err
			}
			comments, err := g.client.ListComments(ctx, g.Locator().Repository, n)
			if err != nil {
				return nil, nil, err
			}
			found := false
			for _, x := range comments {
				if x.ID == id {
					in.ExpectedID = v1ID(id)
					in.ExpectedRevision = v1CommentRevision(x)
					found = true
				}
			}
			if !found {
				finding(r, "", "missing_dependency", "Selected existing comment not found under mapped parent")
				continue
			}
			in.Action = "update"
			w.Target = &migration.RemoteRef{ID: target}
		}
		in.Payload = v1JSON(w)
		if _, e = migration.CanonicalWrite(in, false); e != nil {
			return nil, nil, e
		}
		ins = append(ins, in)
		finding(r, "", "mapped", "Comment bound to confirmed parent receipt; API recovery marker disclosed; source author/time remain attribution")
	}
	return ins, fs, nil
}
func (g *v1Migration) bound(in migration.Intent, ac migration.ApplyContext) (migration.Intent, error) {
	w := ac.BoundWrite()
	if w.IntentID != in.ID || !migration.ValidDigest(w.PlanDigest) || w.PayloadDigest != migration.Digest(w.Payload) {
		return in, errors.New("write context not bound to intent")
	}
	in.Payload = w.Payload
	if _, e := migration.CanonicalWrite(in, true); e != nil {
		return in, e
	}
	if (in.Kind != "issue" && in.Kind != "comment") || (in.Action != "create" && in.Action != "update") {
		return in, errors.New("unsupported forge write")
	}
	return in, nil
}
func v1Input(w migration.IssueWrite, ac migration.ApplyContext) (forgejoapi.IssueInput, error) {
	if w.Type != "" || (w.State != "open" && w.State != "closed") {
		return forgejoapi.IssueInput{}, errors.New("explicit issue type/state mapping required")
	}
	in := forgejoapi.IssueInput{Title: w.Title, Body: w.Body + v1Marker(ac), State: w.State, Labels: []int64{}, Assignees: w.Assignees}
	if len(in.Title) > 255 || len(in.Body) > forgejoapi.BodyCapacity {
		return in, errors.New("write exceeds qualified capacity")
	}
	for _, x := range w.Labels {
		n, e := v1CommentID(x)
		if e != nil {
			return in, e
		}
		in.Labels = append(in.Labels, n)
	}
	return in, nil
}
func v1Matches(v forgejoapi.Issue, w forgejoapi.IssueInput) bool {
	if v.IsPull() || v.Title != w.Title || v.Body != w.Body || v.State != w.State {
		return false
	}
	labels := []string{}
	for _, x := range w.Labels {
		labels = append(labels, strconv.FormatInt(x, 10))
	}
	assignees := []string{}
	for _, x := range v.Assignees {
		assignees = append(assignees, x.Login)
	}
	return v1SameStrings(v1Labels(v), labels) && v1SameStrings(assignees, w.Assignees)
}
func (g *v1Migration) parent(ctx context.Context, in migration.Intent, w migration.CommentWrite, ac migration.ApplyContext) (int, error) {
	n, e := v1Index(w.Parent.ID)
	if e != nil {
		return 0, e
	}
	expected := ""
	for _, id := range in.DependsOn {
		r, ok := ac.Dependency(id)
		if !ok || r.Destination != g.Locator() {
			return 0, errors.New("unconfirmed comment parent dependency")
		}
		if r.Kind == "issue" && r.RemoteKey == w.Parent.ID {
			if expected != "" {
				return 0, errors.New("ambiguous parent")
			}
			expected = r.RemoteID
		}
	}
	if expected == "" {
		return 0, errors.New("comment requires confirmed immutable parent")
	}
	v, e := g.client.GetIssue(ctx, g.Locator().Repository, n)
	if e != nil {
		return 0, e
	}
	if v.IsPull() || v1ID(v.ID) != expected {
		return 0, errors.New("comment parent identity changed")
	}
	return n, nil
}
func (g *v1Migration) receipt(in migration.Intent, ac migration.ApplyContext, id, key, revision string) migration.Receipt {
	r := migrationReceipt(in, ac, g.Locator(), id, revision)
	r.RemoteKey = key
	return r
}
func (g *v1Migration) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	read := *g
	read.client = g.client.BoundedReadClient(forgejoapi.MaxTotalBytes)
	g = &read
	in, e := g.bound(in, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	if e = g.checkRepo(ctx, in); e != nil {
		return migration.Receipt{}, e
	}
	if in.Kind == "issue" {
		var w migration.IssueWrite
		_ = json.Unmarshal(in.Payload, &w)
		input, e := v1Input(w, ac)
		if e != nil {
			return migration.Receipt{}, e
		}
		var v forgejoapi.Issue
		if in.Action == "update" {
			n, e := v1Index(w.Target.ID)
			if e != nil {
				return migration.Receipt{}, e
			}
			current, e := g.client.GetIssue(ctx, g.Locator().Repository, n)
			if e != nil {
				return migration.Receipt{}, e
			}
			if in.ExpectedID == "" || in.ExpectedRevision == "" || v1ID(current.ID) != in.ExpectedID || v1IssueRevision(current) != in.ExpectedRevision || !v1SameStrings(v1Labels(current), w.Labels) {
				return migration.Receipt{}, errors.New("issue identity revision or label baseline changed")
			}
			if g.client.Profile() == "gitea-1.27" {
				if current.ContentVersion <= 0 {
					return migration.Receipt{}, errors.New("Gitea 1.27 update requires observed content version")
				}
				input.ContentVersion = current.ContentVersion
			}
			v, e = g.client.UpdateIssue(ctx, g.Locator().Repository, n, input)
		} else {
			if len(input.Labels) > 0 {
				available, err := g.client.ListLabels(ctx, g.Locator().Repository)
				if err != nil {
					return migration.Receipt{}, err
				}
				ids := map[int64]bool{}
				for _, label := range available {
					ids[label.ID] = true
				}
				for _, id := range input.Labels {
					if !ids[id] {
						return migration.Receipt{}, errors.New("mapped label disappeared before issue create")
					}
				}
			}
			v, e = g.client.CreateIssue(ctx, g.Locator().Repository, input)
		}
		if e != nil {
			return migration.Receipt{}, e
		}
		if !v1Matches(v, input) || in.ExpectedID != "" && in.ExpectedID != v1ID(v.ID) {
			return migration.Receipt{}, errors.New("different issue projection returned; recovery required")
		}
		return g.receipt(in, ac, v1ID(v.ID), strconv.Itoa(v.Number), v1IssueRevision(v)), nil
	}
	var w migration.CommentWrite
	_ = json.Unmarshal(in.Payload, &w)
	n, e := g.parent(ctx, in, w, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	body := w.Body + v1Marker(ac)
	if len(body) > forgejoapi.BodyCapacity {
		return migration.Receipt{}, errors.New("comment marker exceeds capacity")
	}
	var v forgejoapi.Comment
	if in.Action == "update" {
		id, e := v1CommentID(w.Target.ID)
		if e != nil {
			return migration.Receipt{}, e
		}
		rows, e := g.client.ListComments(ctx, g.Locator().Repository, n)
		if e != nil {
			return migration.Receipt{}, e
		}
		found := false
		for _, x := range rows {
			if x.ID == id && in.ExpectedID == v1ID(x.ID) && in.ExpectedRevision != "" && in.ExpectedRevision == v1CommentRevision(x) {
				found = true
			}
		}
		if !found {
			return migration.Receipt{}, errors.New("comment identity or revision changed")
		}
		v, e = g.client.UpdateComment(ctx, g.Locator().Repository, id, body)
	} else {
		v, e = g.client.CreateComment(ctx, g.Locator().Repository, n, body)
	}
	if e != nil {
		return migration.Receipt{}, e
	}
	if v.Body != body || in.ExpectedID != "" && v1ID(v.ID) != in.ExpectedID {
		return migration.Receipt{}, errors.New("different comment returned; recovery required")
	}
	return g.receipt(in, ac, v1ID(v.ID), strconv.FormatInt(v.ID, 10), v1CommentRevision(v)), nil
}
func (g *v1Migration) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	read := *g
	read.client = g.client.BoundedReadClient(forgejoapi.MaxTotalBytes)
	g = &read
	in, e := g.bound(in, ac)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	if e = g.checkRepo(ctx, in); e != nil {
		return migration.Reconciliation{}, e
	}
	matches := []migration.Receipt{}
	if in.Kind == "issue" {
		var w migration.IssueWrite
		_ = json.Unmarshal(in.Payload, &w)
		input, e := v1Input(w, ac)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		var rows []forgejoapi.Issue
		if in.Action == "update" {
			n, e := v1Index(w.Target.ID)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			v, e := g.client.GetIssue(ctx, g.Locator().Repository, n)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			rows = []forgejoapi.Issue{v}
		} else {
			rows, e = g.client.ListIssues(ctx, g.Locator().Repository)
			if e != nil {
				return migration.Reconciliation{}, e
			}
		}
		for _, v := range rows {
			if v1Matches(v, input) && (in.ExpectedID == "" || in.ExpectedID == v1ID(v.ID)) {
				matches = append(matches, g.receipt(in, ac, v1ID(v.ID), strconv.Itoa(v.Number), v1IssueRevision(v)))
			}
		}
	} else {
		var w migration.CommentWrite
		_ = json.Unmarshal(in.Payload, &w)
		n, e := g.parent(ctx, in, w, ac)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		rows, e := g.client.ListComments(ctx, g.Locator().Repository, n)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		for _, v := range rows {
			if v.Body == w.Body+v1Marker(ac) && (in.ExpectedID == "" || in.ExpectedID == v1ID(v.ID)) {
				matches = append(matches, g.receipt(in, ac, v1ID(v.ID), strconv.FormatInt(v.ID, 10), v1CommentRevision(v)))
			}
		}
	}
	if len(matches) == 1 {
		return migration.Reconciliation{Status: "exact", Receipt: &matches[0], Reason: "Unique exact bound marker and full intended projection observed"}, nil
	}
	if len(matches) > 1 {
		return migration.Reconciliation{Status: "ambiguous", Reason: "Multiple exact matches; operator recovery required"}, nil
	}
	return migration.Reconciliation{Status: "no_proof", Reason: "No exact positive evidence; never repeat an uncertain create"}, nil
}
