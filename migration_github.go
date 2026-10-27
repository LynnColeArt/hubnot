//go:build !nogithub && !noforge

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"hubnot/internal/githubapi"
	"hubnot/internal/migration"
)

func init() {
	if e := migrationProviders.Register(migration.ProviderRegistration{Kind: "github", TokenEnv: "HN_GITHUB_TOKEN", RequiredCoverage: []migration.CoverageRequirement{{Kind: "issue", Read: true, Preserve: true, Create: true}, {Kind: "comment", Read: true, Preserve: true, Create: true}}, NewSource: func(c migration.ProviderConfig) (migration.Source, error) { return newGitHubMigration(c) }, NewDestination: func(c migration.ProviderConfig) (migration.Destination, error) { return newGitHubMigration(c) }}); e != nil {
		panic(e)
	}
}

type githubMigration struct {
	config migration.ProviderConfig
	client *githubapi.Client
}

func newGitHubMigration(c migration.ProviderConfig) (*githubMigration, error) {
	if c.Locator.Kind != "github" || c.Locator.Organization != "" || c.Locator.Project != "" {
		return nil, errors.New("GitHub requires a repository selector, with no planning-project conflation")
	}
	if c.APIVersion != "" && c.APIVersion != "2022-11-28" {
		return nil, errors.New("GitHub adapter supports REST v2022-11-28")
	}
	if c.CredentialMode != "" && c.CredentialMode != "bearer" {
		return nil, errors.New("GitHub adapter requires supplied bearer token")
	}
	client, e := githubapi.NewClient(c.Locator.Endpoint, c.Token)
	if e != nil {
		return nil, e
	}
	if e = githubapi.ValidateRepository(c.Locator.Repository); e != nil {
		return nil, e
	}
	c.Locator.Endpoint = client.BaseURL()
	c.PlanningSelectors = append([]string{}, c.PlanningSelectors...)
	c.Attachments = append([]migration.AttachmentSelection{}, c.Attachments...)
	return &githubMigration{c, client}, nil
}
func (g *githubMigration) Locator() migration.Locator { return g.config.Locator }
func (g *githubMigration) Capabilities(context.Context) ([]migration.Capability, error) {
	out := []migration.Capability{{Kind: "issue", Read: true, Preserve: true, Create: true, Update: true, Limits: []string{"REST v2022-11-28; open-state creates/updates only; closed source state requires explicit mapping or preservation", "source authors/times remain attribution, not impersonation", "work_item inputs require explicit type/state mappings to GitHub issues; issue types are observed but never administered", "100 pages per connection; 4 MiB API response; 64 MiB aggregate snapshot wire budget; selected plan budgets are additional ceilings", "labels/assignees require explicit mapping; no hosted permission migration"}}, {Kind: "comment", Read: true, Preserve: true, Create: true, Limits: []string{"issue comments only; parent immutable identity checked from confirmed receipt"}}}
	for _, kind := range []string{"change_request", "review", "inline_comment", "discussion", "reply", "project", "project_item", "workflow", "attachment"} {
		out = append(out, migration.Capability{Kind: kind, Read: true, Preserve: true, Limits: []string{"preservation only; no destination-native recreation", "explicit planning/attachment selections; same-origin attachments only; workflow source inert"}})
	}
	return out, nil
}
func githubMigrationJSON(v any) []byte     { b, _ := json.Marshal(v); return b }
func githubMigrationID(id int64) string    { return "id:" + strconv.FormatInt(id, 10) }
func githubMigrationRevision(v any) string { return migration.Digest(githubMigrationJSON(v)) }
func githubMigrationRepoRevision(r githubapi.Repository) string {
	return githubMigrationRevision([]string{githubMigrationID(r.ID), r.NodeID, r.CreatedAt})
}
func githubMigrationFinding(id, status, path, message string) migration.Finding {
	return migration.Finding{ID: id, Status: status, Path: path, Message: message}
}
func githubMigrationUnavailable(e error) bool {
	var h *githubapi.HTTPError
	return errors.As(e, &h) && (h.StatusCode == http.StatusUnauthorized || h.StatusCode == http.StatusForbidden || h.StatusCode == http.StatusNotFound || h.StatusCode == http.StatusGone)
}

type githubMigrationBlobs map[string][]byte

func (b githubMigrationBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	v, ok := b[d]
	if !ok {
		return nil, errors.New("blob not in selected snapshot")
	}
	return io.NopCloser(bytes.NewReader(v)), nil
}

type githubSnapshotBuilder struct {
	projectID string
	s         migration.Snapshot
	blobs     githubMigrationBlobs
	l         migration.Limits
	total     int64
	err       error
}

func (b *githubSnapshotBuilder) key(kind, id string) migration.Key {
	return migration.Key{Provider: "github", Instance: b.s.Source.Endpoint, ProjectID: b.projectID, Kind: kind, ID: id}
}
func (b *githubSnapshotBuilder) blob(raw []byte, name, media string) string {
	if b.err != nil {
		return ""
	}
	d := migration.Digest(raw)
	if _, ok := b.blobs[d]; ok {
		return d
	}
	if len(b.blobs) >= b.l.Blobs || int64(len(raw)) > b.l.BlobBytes || int64(len(raw)) > b.l.AttachmentBytes-b.total {
		b.err = errors.New("GitHub snapshot blob budget exceeded")
		return ""
	}
	b.total += int64(len(raw))
	b.blobs[d] = append([]byte{}, raw...)
	b.s.Blobs = append(b.s.Blobs, migration.Blob{Digest: d, Size: int64(len(raw)), Name: name, MediaType: media})
	return d
}
func (b *githubSnapshotBuilder) add(r migration.Record) {
	if b.err != nil {
		return
	}
	if len(b.s.Records) >= b.l.Records {
		b.err = errors.New("GitHub snapshot record budget exceeded")
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
func (b *githubSnapshotBuilder) preserve(kind, id, selector, url, author, created, updated string, raw []byte, links []migration.Link) {
	d := b.blob(raw, kind+"-"+id, "application/json")
	b.add(migration.Record{Key: b.key(kind, id), Selector: selector, Revision: migration.Digest(raw), SourceURL: url, Author: author, CreatedAt: created, UpdatedAt: updated, Schema: "hn.migration.preserved/1", Content: githubMigrationJSON(migration.PreservedContent{Schema: "hn.migration.preserved/1", Summary: "Retrieved GitHub " + kind + "; foreign source remains inert", SourceBlob: d}), Links: links, Blobs: []string{d}})
}
func (g *githubMigration) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	if e := l.Validate(); e != nil {
		return migration.Snapshot{}, nil, e
	}
	sourceLocator := sel.Source
	normalized, e := githubapi.NewClient(sourceLocator.Endpoint, "")
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	sourceLocator.Endpoint = normalized.BaseURL()
	if sourceLocator != g.Locator() {
		return migration.Snapshot{}, nil, errors.New("GitHub source locator differs from selection")
	}
	// The wire budget includes every request in this operation. API hard limits
	// remain additional ceilings rather than silently replacing plan budgets.
	wire := l.ManifestBytes + l.AttachmentBytes
	if wire > githubapi.MaxTotalBytes {
		wire = githubapi.MaxTotalBytes
	}
	c := g.client.BoundedReadClient(wire)
	repo, e := c.GetRepository(ctx, g.Locator().Repository)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	b := githubSnapshotBuilder{s: migration.Snapshot{Schema: "hn.project-snapshot/1", Source: g.Locator(), SourceID: "github:" + migration.Digest(githubMigrationJSON([]string{g.Locator().Endpoint, githubMigrationID(repo.ID)}))[7:], Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}, blobs: githubMigrationBlobs{}, l: l, projectID: githubMigrationID(repo.ID)}
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
	finding := func(kind, status, message string) {
		b.s.Findings = append(b.s.Findings, githubMigrationFinding("github-source-"+kind, status, kind, message))
	}
	unavailable := func(kind string, e error) error {
		if githubMigrationUnavailable(e) {
			finding(kind, "missing_dependency", "GitHub resource inaccessible; no content claimed preserved")
			return nil
		}
		return e
	}
	if want("issue") || want("comment") {
		rows, err := c.ListIssues(ctx, repo.FullName)
		if err != nil {
			if e = unavailable("issue", err); e != nil {
				return migration.Snapshot{}, nil, e
			}
		} else {
			for _, v := range rows {
				labels := []string{}
				for _, x := range v.Labels {
					labels = append(labels, x.Name)
				}
				assignees := []string{}
				for _, x := range v.Assignees {
					assignees = append(assignees, x.Login)
				}
				content := githubMigrationJSON(migration.IssueContent{Schema: "hn.migration.issue/1", Title: v.Title, Body: v.Body, State: v.State, Type: githubMigrationIssueType(v), Labels: labels, Assignees: assignees})
				r := migration.Record{Key: b.key("issue", githubMigrationID(v.ID)), Selector: strconv.Itoa(v.Number), Revision: githubMigrationRevision(v), SourceURL: v.HTMLURL, Author: v.User.Login, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Schema: "hn.migration.issue/1", Content: content}
				representable := true
				if _, err := migration.CanonicalContent("issue", content); err != nil {
					representable = false
					b.preserve("attachment", "issue-source:"+r.Key.ID, r.Selector, r.SourceURL, r.Author, r.CreatedAt, r.UpdatedAt, githubMigrationJSON(v), nil)
					finding("issue-content-"+r.Selector, "unsupported", "Issue exceeds closed issue schema; retrieved source bytes retained as inert attachment record")
				} else {
					b.add(r)
				}
				if want("comment") {
					comments, err := c.ListComments(ctx, repo.FullName, v.Number)
					if err != nil {
						if e = unavailable("issue-comments-"+strconv.Itoa(v.Number), err); e != nil {
							return migration.Snapshot{}, nil, e
						}
					} else {
						for _, x := range comments {
							commentContent := githubMigrationJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: x.Body})
							if _, err := migration.CanonicalContent("comment", commentContent); !representable || err != nil {
								parent := r.Key
								if !representable {
									parent = b.key("attachment", "issue-source:"+r.Key.ID)
								}
								b.preserve("reply", "issue-comment:"+githubMigrationID(x.ID), strconv.FormatInt(x.ID, 10), x.HTMLURL, x.User.Login, x.CreatedAt, x.UpdatedAt, githubMigrationJSON(x), []migration.Link{{Relation: "source_record", Target: parent}})
								continue
							}
							b.add(migration.Record{Key: b.key("comment", githubMigrationID(x.ID)), Selector: strconv.FormatInt(x.ID, 10), Revision: githubMigrationRevision(x), SourceURL: x.HTMLURL, Author: x.User.Login, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, Schema: "hn.migration.comment/1", Content: githubMigrationJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: x.Body}), Links: []migration.Link{{Relation: "issue", Target: r.Key, Revision: r.Revision}}})
						}
					}
				}
				if b.err != nil {
					return migration.Snapshot{}, nil, b.err
				}
			}
		}
	}
	if want("change_request") || want("review") || want("inline_comment") {
		pulls, err := c.ListPullRequests(ctx, repo.FullName)
		if err != nil {
			if e = unavailable("change_request", err); e != nil {
				return migration.Snapshot{}, nil, e
			}
		} else {
			for _, p := range pulls {
				if p.Number <= 0 {
					return migration.Snapshot{}, nil, errors.New("invalid GitHub pull selector")
				}
				pk := b.key("change_request", githubMigrationID(p.ID))
				b.preserve("change_request", pk.ID, strconv.Itoa(p.Number), p.HTMLURL, p.User.Login, p.CreatedAt, p.UpdatedAt, p.Raw, nil)
				// PR general conversations use the issue-comment API but are preserved as
				// replies: they cannot become ordinary issue comments with a fabricated parent.
				conversations, err := c.ListComments(ctx, repo.FullName, p.Number)
				if err != nil {
					if e = unavailable("pull-conversation-"+strconv.Itoa(p.Number), err); e != nil {
						return migration.Snapshot{}, nil, e
					}
				} else {
					for _, x := range conversations {
						b.preserve("reply", "pull-comment:"+strconv.FormatInt(x.ID, 10), strconv.FormatInt(x.ID, 10), x.HTMLURL, x.User.Login, x.CreatedAt, x.UpdatedAt, githubMigrationJSON(x), []migration.Link{{Relation: "change_request", Target: pk}})
					}
				}
				if want("review") || want("inline_comment") {
					reviews, err := c.ListPullReviews(ctx, repo.FullName, p.Number)
					if err != nil {
						if e = unavailable("reviews-"+strconv.Itoa(p.Number), err); e != nil {
							return migration.Snapshot{}, nil, e
						}
					} else {
						for _, x := range reviews {
							b.preserve("review", githubMigrationID(x.ID), strconv.FormatInt(x.ID, 10), x.HTMLURL, x.User.Login, x.CreatedAt, x.UpdatedAt, x.Raw, []migration.Link{{Relation: "change_request", Target: pk}})
						}
					}
				}
				if want("inline_comment") {
					comments, err := c.ListPullReviewComments(ctx, repo.FullName, p.Number)
					if err != nil {
						if e = unavailable("inline-comments-"+strconv.Itoa(p.Number), err); e != nil {
							return migration.Snapshot{}, nil, e
						}
					} else {
						for _, x := range comments {
							links := []migration.Link{{Relation: "change_request", Target: pk}}
							if x.ReviewID > 0 {
								links = append(links, migration.Link{Relation: "review", Target: b.key("review", githubMigrationID(x.ReviewID))})
							}
							if x.ReplyTo > 0 {
								links = append(links, migration.Link{Relation: "reply_to", Target: b.key("inline_comment", githubMigrationID(x.ReplyTo))})
							}
							b.preserve("inline_comment", githubMigrationID(x.ID), strconv.FormatInt(x.ID, 10), x.HTMLURL, x.User.Login, x.CreatedAt, x.UpdatedAt, x.Raw, links)
						}
					}
				}
				if b.err != nil {
					return migration.Snapshot{}, nil, b.err
				}
			}
		}
	}
	if want("discussion") || want("reply") {
		if !repo.HasDiscussions {
			finding("discussion", "unsupported", "Discussions are disabled on this repository")
		} else if g.config.Token == "" {
			finding("discussion", "missing_dependency", "Discussion snapshots require an explicitly supplied GraphQL token")
		} else {
			rows, err := c.ListDiscussions(ctx, repo.FullName)
			if err != nil {
				if e = unavailable("discussion", err); e != nil {
					return migration.Snapshot{}, nil, e
				}
			} else {
				for _, row := range rows {
					v, err := c.GetDiscussion(ctx, repo.FullName, row.Number)
					if err != nil {
						if e = unavailable("discussion-"+row.ID, err); e != nil {
							return migration.Snapshot{}, nil, e
						}
						continue
					}
					if v.RepositoryID != repo.NodeID {
						return migration.Snapshot{}, nil, errors.New("GitHub discussion belongs to a different immutable repository")
					}
					key := b.key("discussion", v.ID)
					b.preserve("discussion", v.ID, strconv.Itoa(v.Number), v.URL, v.Author, v.CreatedAt, v.UpdatedAt, githubMigrationJSON(v), nil)
					for _, x := range v.Comments {
						links := []migration.Link{{Relation: "discussion", Target: key}}
						if x.ParentID != "" {
							links = append(links, migration.Link{Relation: "reply_to", Target: b.key("reply", x.ParentID)})
						}
						b.preserve("reply", x.ID, x.ID, x.URL, x.Author, x.CreatedAt, x.UpdatedAt, githubMigrationJSON(x), links)
					}
					if b.err != nil {
						return migration.Snapshot{}, nil, b.err
					}
				}
			}
		}
	}
	if want("project") || want("project_item") {
		if g.config.Token == "" && len(g.config.PlanningSelectors) > 0 {
			finding("project", "missing_dependency", "Planning snapshots require an explicitly supplied GraphQL token")
		} else if len(g.config.PlanningSelectors) == 0 {
			finding("project", "requires_mapping", "Planning projects are separately scoped; supply explicit user/org/node planning selectors")
		} else {
			for _, selector := range g.config.PlanningSelectors {
				v, err := c.GetProject(ctx, selector)
				if err != nil {
					if e = unavailable("project-"+selector, err); e != nil {
						return migration.Snapshot{}, nil, e
					}
					continue
				}
				key := b.key("project", v.ID)
				b.preserve("project", v.ID, selector, v.URL, v.Creator, v.CreatedAt, v.UpdatedAt, githubMigrationJSON(v), nil)
				for _, x := range v.Items {
					b.preserve("project_item", x.ID, x.ID, "", x.Creator, x.CreatedAt, x.UpdatedAt, githubMigrationJSON(x), []migration.Link{{Relation: "project", Target: key}})
				}
			}
		}
	}
	if want("workflow") {
		ref := g.config.WorkflowRef
		if ref == "" {
			ref = repo.DefaultBranch
		}
		if ref == "" {
			finding("workflow", "missing_dependency", "No workflow revision selected and repository has no default branch")
		} else {
			rows, err := c.ReadWorkflowDefinitions(ctx, repo.FullName, ref, "")
			if err != nil {
				if e = unavailable("workflow", err); e != nil {
					return migration.Snapshot{}, nil, e
				}
			} else {
				for _, x := range rows {
					b.preserve("workflow", githubMigrationID(x.Workflow.ID), x.File.Path, x.Workflow.HTMLURL, "", x.Workflow.CreatedAt, x.Workflow.UpdatedAt, githubMigrationJSON(x), nil)
					d := b.blob(x.File.Content, x.File.Path, "text/plain")
					if b.err == nil {
						last := &b.s.Records[len(b.s.Records)-1]
						last.Blobs = append(last.Blobs, d)
					}
				}
			}
		}
	}
	for i, a := range g.config.Attachments {
		if len(g.config.Attachments) > l.Blobs {
			return migration.Snapshot{}, nil, errors.New("selected attachment count budget exceeded")
		}
		parent := -1
		for j, r := range b.s.Records {
			if r.Key == a.Parent {
				parent = j
				break
			}
		}
		if parent < 0 {
			finding("attachment-"+strconv.Itoa(i), "missing_dependency", "Selected attachment parent was not retrieved")
			continue
		}
		data, media, err := c.ReadAttachment(ctx, a.URL, l.BlobBytes)
		if err != nil {
			if githubMigrationUnavailable(err) || strings.Contains(err.Error(), "same-origin") {
				finding("attachment-"+strconv.Itoa(i), "missing_dependency", "Selected attachment unavailable or outside credential origin; bytes not preserved")
				continue
			}
			return migration.Snapshot{}, nil, err
		}
		d := b.blob(data, a.Name, media)
		if b.err == nil {
			b.s.Records[parent].Blobs = append(b.s.Records[parent].Blobs, d)
		}
	}
	if b.err != nil {
		return migration.Snapshot{}, nil, b.err
	}
	// End-of-observation identity check refuses mixed incarnations at one selector.
	current, e := c.GetRepository(ctx, repo.FullName)
	if e != nil || githubMigrationRepoRevision(current) != githubMigrationRepoRevision(repo) {
		return migration.Snapshot{}, nil, errors.New("GitHub repository changed during snapshot")
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
				finding("link-"+strconv.Itoa(i)+"-"+strconv.Itoa(j), "missing_dependency", "Relationship target inaccessible; original reference remains in retrieved source blob")
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

// Intent IDs persist a repository-incarnation baseline without extending the
// frozen write schema: gh1.<64 lowercase SHA256 hex>.<64 source-key SHA256 hex>.
// The first digest binds locator (including origin), numeric+node repository IDs,
// and creation time. Apply/reconcile parse strictly and re-observe it before any
// mutation. Resource ExpectedID/ExpectedRevision retain their shared meaning.
func (g *githubMigration) baseline(r githubapi.Repository) string {
	return migration.Digest(githubMigrationJSON([]string{g.Locator().Kind, g.Locator().Endpoint, g.Locator().Repository, githubMigrationRepoRevision(r)}))[7:]
}
func (g *githubMigration) intentID(r githubapi.Repository, key migration.Key) string {
	return "gh1." + g.baseline(r) + "." + migration.Digest([]byte(key.String()))[7:]
}
func (g *githubMigration) checkRepo(ctx context.Context, in migration.Intent) error {
	p := strings.Split(in.ID, ".")
	if len(p) != 3 || p[0] != "gh1" {
		return errors.New("invalid GitHub intent repository binding")
	}
	for _, s := range p[1:] {
		if len(s) != 64 || strings.ToLower(s) != s {
			return errors.New("invalid GitHub intent digest")
		}
		if _, e := hex.DecodeString(s); e != nil {
			return errors.New("invalid GitHub intent digest")
		}
	}
	r, e := g.client.GetRepository(ctx, g.Locator().Repository)
	if e != nil {
		return e
	}
	if p[1] != g.baseline(r) {
		return errors.New("GitHub destination repository incarnation changed")
	}
	return nil
}
func (g *githubMigration) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if g.Locator() != sel.Destination {
		return nil, nil, errors.New("GitHub destination locator mismatch")
	}
	if _, e := migration.CanonicalSnapshot(s, l); e != nil {
		return nil, nil, e
	}
	repo, e := g.client.GetRepository(ctx, g.Locator().Repository)
	if e != nil {
		return nil, nil, e
	}
	ins := []migration.Intent{}
	fs := []migration.Finding{}
	parents := map[string]migration.Intent{}
	finding := func(r migration.Record, status, message string) {
		fs = append(fs, githubMigrationFinding("github-destination-"+migration.Digest([]byte(r.Key.String()))[7:], status, r.Key.String(), message))
	}
	for _, r := range s.Records {
		if len(r.Blobs) > 0 {
			fs = append(fs, githubMigrationFinding("github-destination-blobs-"+migration.Digest([]byte(r.Key.String()))[7:], "unsupported", r.Key.String(), "Retrieved blobs remain in saved snapshot; GitHub API attachment upload is not supported"))
		}
	}
	// Stable partition ensures issues precede comments regardless of archive order.
	for _, r := range s.Records {
		if r.Key.Kind != "issue" && r.Key.Kind != "work_item" {
			continue
		}
		var c migration.IssueContent
		_ = json.Unmarshal(r.Content, &c)
		if r.Key.Kind == "work_item" {
			_, stateMapped := sel.Mappings["state:"+c.State]
			_, typeMapped := sel.Mappings["type:"+c.Type]
			if !stateMapped || !typeMapped {
				finding(r, "requires_mapping", "Work-item to GitHub issue conversion requires explicit source type and state mappings")
				continue
			}
		}
		state := c.State
		if mapped, ok := sel.Mappings["state:"+state]; ok {
			state = mapped
		}
		typ := c.Type
		if mapped, ok := sel.Mappings["type:"+typ]; ok {
			typ = mapped
		}
		if state != "open" || typ != "" {
			finding(r, "requires_mapping", "GitHub issue creates/updates support open state and empty type only; closed creation would require a second write and type administration is unsupported")
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
			finding(r, "requires_mapping", "Explicit label/assignee mappings (or empty omission mappings) are required")
			continue
		}
		w := migration.IssueWrite{Schema: "hn.migration.issue-write/1", Title: c.Title, Body: c.Body, State: state, Type: typ, Labels: labels, Assignees: assignees}
		in := migration.Intent{ID: g.intentID(repo, r.Key), Kind: "issue", Action: "create", Sources: []migration.Key{r.Key}, DependsOn: []string{}}
		if len(w.Body) > githubMigrationBodyBudget(in.ID) {
			finding(r, "unsupported", "Issue body plus the required recovery marker exceeds GitHub's body capacity; source bytes remain preserved without truncation")
			continue
		}
		if target := sel.Mappings["target:"+r.Key.String()]; target != "" {
			number, err := githubKey(target)
			if err != nil {
				return nil, nil, err
			}
			current, err := g.client.GetIssue(ctx, repo.FullName, number)
			if err != nil {
				return nil, nil, err
			}
			if len(current.PullRequest) > 0 && string(current.PullRequest) != "null" {
				return nil, nil, errors.New("cannot update pull request through issue migration")
			}
			if githubMigrationIssueType(current) != "" {
				finding(r, "unsupported", "Existing destination issue has a type; type administration is unsupported and cannot be silently cleared")
				continue
			}
			in.Action = "update"
			in.ExpectedID = githubMigrationID(current.ID)
			in.ExpectedRevision = githubMigrationRevision(current)
			w.Target = &migration.RemoteRef{ID: target}
		}
		in.Payload = githubMigrationJSON(w)
		if _, e = migration.CanonicalWrite(in, false); e != nil {
			return nil, nil, e
		}
		ins = append(ins, in)
		parents[r.Key.String()] = in
		finding(r, "mapped", "Issue body/state/labels/assignees mapped; API body appends a deterministic HTML recovery marker; original author/times retained in source snapshot rather than impersonated")
	}
	for _, r := range s.Records {
		if r.Key.Kind == "issue" || r.Key.Kind == "work_item" {
			continue
		}
		if r.Key.Kind != "comment" {
			finding(r, "unsupported", "Retrieved foreign "+r.Key.Kind+" remains inspectable in the saved snapshot; no GitHub destination recreation")
			continue
		}
		var parent migration.Intent
		for _, link := range r.Links {
			if link.Relation == "issue" || link.Relation == "work_item" {
				parent = parents[link.Target.String()]
			}
		}
		if parent.ID == "" {
			finding(r, "missing_dependency", "Comment parent has no mapped issue intent")
			continue
		}
		var c migration.CommentContent
		_ = json.Unmarshal(r.Content, &c)
		w := migration.CommentWrite{Schema: "hn.migration.comment-write/1", Parent: migration.RemoteRef{Receipt: &migration.ReceiptRef{IntentID: parent.ID, Field: "remote_key"}}, Body: c.Body}
		in := migration.Intent{ID: g.intentID(repo, r.Key), Kind: "comment", Action: "create", Sources: []migration.Key{r.Key}, DependsOn: []string{parent.ID}, Payload: githubMigrationJSON(w)}
		if len(w.Body) > githubMigrationBodyBudget(in.ID) {
			finding(r, "unsupported", "Comment body plus the required recovery marker exceeds GitHub's body capacity; source bytes remain preserved without truncation")
			continue
		}
		ins = append(ins, in)
		finding(r, "mapped", "Comment mapped to confirmed issue receipt; API body appends a deterministic HTML recovery marker; original source attribution retained in saved snapshot")
	}
	return ins, fs, nil
}
func (g *githubMigration) bound(in migration.Intent, ac migration.ApplyContext) (migration.Intent, error) {
	w := ac.BoundWrite()
	if w.IntentID != in.ID || !migration.ValidDigest(w.PlanDigest) || w.PayloadDigest != migration.Digest(w.Payload) {
		return in, errors.New("GitHub write context does not bind intent")
	}
	in.Payload = w.Payload
	if _, e := migration.CanonicalWrite(in, true); e != nil {
		return in, e
	}
	if in.Kind != "issue" && in.Kind != "comment" || in.Action != "create" && in.Action != "update" || in.Kind == "comment" && in.Action != "create" {
		return in, errors.New("unsupported GitHub migration write")
	}
	return in, nil
}

const githubMigrationMarkerPrefix = "\n\n<!-- hn-migration:"
const githubMigrationMarkerSuffix = " -->"

func githubMigrationBodyBudget(intentID string) int {
	return 65536 - len(githubMigrationMarkerPrefix) - 2*len(migration.Digest(nil)) - 2 - len(intentID) - len(githubMigrationMarkerSuffix)
}
func githubMigrationIssueType(v githubapi.Issue) string {
	if v.Type == nil {
		return ""
	}
	return v.Type.Name
}
func githubMigrationMarker(ac migration.ApplyContext) string {
	w := ac.BoundWrite()
	return githubMigrationMarkerPrefix + w.PlanDigest + ":" + w.IntentID + ":" + w.PayloadDigest + githubMigrationMarkerSuffix
}
func githubMigrationIssueInput(w migration.IssueWrite, ac migration.ApplyContext) (githubapi.IssueInput, error) {
	if w.Type != "" || w.State != "open" {
		return githubapi.IssueInput{}, errors.New("GitHub migration requires explicitly mapped open issue state and empty type")
	}
	body := w.Body + githubMigrationMarker(ac)
	if len(body) > 65536 {
		return githubapi.IssueInput{}, errors.New("GitHub attribution marker exceeds body budget")
	}
	return githubapi.IssueInput{Title: w.Title, Body: body, State: w.State, Labels: w.Labels, Assignees: w.Assignees}, nil
}
func (g *githubMigration) commentParent(ctx context.Context, in migration.Intent, w migration.CommentWrite, ac migration.ApplyContext) (int, error) {
	number, e := githubKey(w.Parent.ID)
	if e != nil {
		return 0, e
	}
	var expected string
	for _, id := range in.DependsOn {
		r, ok := ac.Dependency(id)
		if !ok || r.Destination != g.Locator() {
			return 0, errors.New("unconfirmed GitHub comment dependency")
		}
		if r.Kind == "issue" && r.RemoteKey == w.Parent.ID {
			if expected != "" {
				return 0, errors.New("ambiguous comment parent")
			}
			expected = r.RemoteID
		}
	}
	if expected == "" {
		return 0, errors.New("comment requires a confirmed immutable issue parent")
	}
	current, e := g.client.GetIssue(ctx, g.Locator().Repository, number)
	if e != nil {
		return 0, e
	}
	if githubMigrationID(current.ID) != expected {
		return 0, errors.New("GitHub comment parent identity changed")
	}
	return number, nil
}
func (g *githubMigration) receipt(in migration.Intent, ac migration.ApplyContext, id, key, revision string) migration.Receipt {
	r := migrationReceipt(in, ac, g.Locator(), id, revision)
	r.RemoteKey = key
	return r
}
func (g *githubMigration) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
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
		input, e := githubMigrationIssueInput(w, ac)
		if e != nil {
			return migration.Receipt{}, e
		}
		var v githubapi.Issue
		if in.Action == "update" {
			n, e := githubKey(w.Target.ID)
			if e != nil {
				return migration.Receipt{}, e
			}
			current, e := g.client.GetIssue(ctx, g.Locator().Repository, n)
			if e != nil {
				return migration.Receipt{}, e
			}
			if in.ExpectedID == "" || in.ExpectedRevision == "" || githubMigrationID(current.ID) != in.ExpectedID || githubMigrationRevision(current) != in.ExpectedRevision {
				return migration.Receipt{}, errors.New("GitHub issue baseline changed")
			}
			v, e = g.client.UpdateIssue(ctx, g.Locator().Repository, n, input)
			if e != nil {
				return migration.Receipt{}, e
			}
		} else {
			v, e = g.client.CreateIssue(ctx, g.Locator().Repository, input)
			if e != nil {
				return migration.Receipt{}, e
			}
		}
		if !githubMigrationIssueMatches(v, input) {
			return migration.Receipt{}, errors.New("GitHub returned a different issue projection; recovery required")
		}
		if in.ExpectedID != "" && githubMigrationID(v.ID) != in.ExpectedID {
			return migration.Receipt{}, errors.New("GitHub returned changed issue identity")
		}
		return g.receipt(in, ac, githubMigrationID(v.ID), strconv.Itoa(v.Number), githubMigrationRevision(v)), nil
	}
	var w migration.CommentWrite
	_ = json.Unmarshal(in.Payload, &w)
	n, e := g.commentParent(ctx, in, w, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	body := w.Body + githubMigrationMarker(ac)
	if len(body) > 65536 {
		return migration.Receipt{}, errors.New("GitHub comment marker exceeds body budget")
	}
	v, e := g.client.CreateComment(ctx, g.Locator().Repository, n, body)
	if e != nil {
		return migration.Receipt{}, e
	}
	if v.Body != body {
		return migration.Receipt{}, errors.New("GitHub returned different comment; recovery required")
	}
	return g.receipt(in, ac, githubMigrationID(v.ID), strconv.FormatInt(v.ID, 10), githubMigrationRevision(v)), nil
}
func githubMigrationIssueMatches(v githubapi.Issue, w githubapi.IssueInput) bool {
	if githubMigrationIssueType(v) != "" || v.Title != w.Title || v.Body != w.Body || v.State != w.State || len(v.PullRequest) > 0 && string(v.PullRequest) != "null" {
		return false
	}
	labels := []string{}
	for _, x := range v.Labels {
		labels = append(labels, x.Name)
	}
	assignees := []string{}
	for _, x := range v.Assignees {
		assignees = append(assignees, x.Login)
	}
	same := func(a, b []string) bool {
		a = append([]string{}, a...)
		b = append([]string{}, b...)
		sort.Strings(a)
		sort.Strings(b)
		return strings.Join(a, "\x00") == strings.Join(b, "\x00")
	}
	return same(labels, w.Labels) && same(assignees, w.Assignees)
}
func (g *githubMigration) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
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
		input, e := githubMigrationIssueInput(w, ac)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		var rows []githubapi.Issue
		if in.Action == "update" {
			n, e := githubKey(w.Target.ID)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			v, e := g.client.GetIssue(ctx, g.Locator().Repository, n)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			rows = []githubapi.Issue{v}
		} else {
			rows, e = g.client.ListIssues(ctx, g.Locator().Repository)
			if e != nil {
				return migration.Reconciliation{}, e
			}
		}
		for _, v := range rows {
			if githubMigrationIssueMatches(v, input) && (in.ExpectedID == "" || githubMigrationID(v.ID) == in.ExpectedID) {
				matches = append(matches, g.receipt(in, ac, githubMigrationID(v.ID), strconv.Itoa(v.Number), githubMigrationRevision(v)))
			}
		}
	} else {
		var w migration.CommentWrite
		_ = json.Unmarshal(in.Payload, &w)
		n, e := g.commentParent(ctx, in, w, ac)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		rows, e := g.client.ListComments(ctx, g.Locator().Repository, n)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		for _, v := range rows {
			if v.Body == w.Body+githubMigrationMarker(ac) {
				matches = append(matches, g.receipt(in, ac, githubMigrationID(v.ID), strconv.FormatInt(v.ID, 10), githubMigrationRevision(v)))
			}
		}
	}
	if len(matches) == 1 {
		return migration.Reconciliation{Status: "exact", Receipt: &matches[0], Reason: "Unique exact remote projection and bound migration marker observed"}, nil
	}
	if len(matches) > 1 {
		return migration.Reconciliation{Status: "ambiguous", Reason: "Multiple exact migration marker matches; operator recovery required"}, nil
	}
	return migration.Reconciliation{Status: "no_proof", Reason: "No exact positive evidence; absence never authorizes another create"}, nil
}
