//go:build !noforge

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"hubnot/internal/gitlabapi"
	"hubnot/internal/migration"
)

func init() {
	if e := migrationProviders.Register(migration.ProviderRegistration{Kind: "gitlab", TokenEnv: "HN_GITLAB_TOKEN", RequiredCoverage: []migration.CoverageRequirement{{Kind: "issue", Read: true, Preserve: true, Create: true, Update: true}, {Kind: "comment", Read: true, Preserve: true, Create: true, Update: true}}, NewSource: func(c migration.ProviderConfig) (migration.Source, error) { return newGitLabMigration(c) }, NewDestination: func(c migration.ProviderConfig) (migration.Destination, error) { return newGitLabMigration(c) }}); e != nil {
		panic(e)
	}
}

type gitlabMigration struct {
	config migration.ProviderConfig
	client *gitlabapi.Client
}

func newGitLabMigration(c migration.ProviderConfig) (*gitlabMigration, error) {
	if c.Locator.Kind != "gitlab" || c.Locator.Organization != "" || c.Locator.Project != "" {
		return nil, errors.New("GitLab requires repository project selector; boards are separate planning selections")
	}
	if c.APIVersion != "" && c.APIVersion != "v4" {
		return nil, errors.New("GitLab supports REST v4 only")
	}
	if c.CredentialMode != "" && c.CredentialMode != "private-token" {
		return nil, errors.New("GitLab requires supplied private-token authentication")
	}
	if e := gitlabapi.ValidateProject(c.Locator.Repository); e != nil {
		return nil, e
	}
	for _, id := range c.PlanningSelectors {
		if _, e := gitlabapi.PositiveKey(id); e != nil {
			return nil, errors.New("GitLab planning selectors are project-local board IDs")
		}
	}
	client, e := gitlabapi.NewClient(c.Locator.Endpoint, c.Token)
	if e != nil {
		return nil, e
	}
	c.Locator.Endpoint = client.BaseURL()
	c.Attachments = append([]migration.AttachmentSelection{}, c.Attachments...)
	c.PlanningSelectors = append([]string{}, c.PlanningSelectors...)
	return &gitlabMigration{c, client}, nil
}
func (g *gitlabMigration) Locator() migration.Locator { return g.config.Locator }
func (g *gitlabMigration) Capabilities(context.Context) ([]migration.Capability, error) {
	out := []migration.Capability{{Kind: "issue", Read: true, Preserve: true, Create: true, Update: true, Limits: []string{"REST v4; opened ordinary issue creates only, opened/closed ordinary issue updates", "source state/type retained; explicit destination mappings; assignees require numeric IDs", "255 byte titles; 1,000,000 byte body including recovery marker; no confidential issue recreation", "slash-command lines (including whitespace/fenced examples) refuse destination recreation; exact source bytes remain preserved", "100 pages/collection; 4 MiB response; 64 MiB shared wire ceiling and positive selected snapshot budgets", "authors/times/permissions remain descriptive; no impersonation or hosted authority transfer"}}, {Kind: "comment", Read: true, Preserve: true, Create: true, Update: true, Limits: []string{"ordinary issue notes only; immutable parent identity checked against confirmed engine receipt", "system/internal/confidential/thread notes retained as inert foreign records"}}}
	for _, kind := range []string{"change_request", "review", "discussion", "reply", "inline_comment", "project", "project_item", "workflow", "attachment"} {
		out = append(out, migration.Capability{Kind: kind, Read: true, Preserve: true, Limits: []string{"preservation only; retrieved API JSON/selected bytes remain inert; no native destination recreation", "boards require separate ID selections; CI preserves pinned .gitlab-ci.yml only, includes/external CI config not fetched", "explicit same-origin uploads only; API version/tier/permission unavailability reported distinctly from empty results"}})
	}
	return out, nil
}
func gitlabJSON(v any) []byte     { b, _ := json.Marshal(v); return b }
func gitlabID(n int64) string     { return "id:" + strconv.FormatInt(n, 10) }
func gitlabRevision(v any) string { return migration.Digest(gitlabJSON(v)) }
func gitlabUnavailable(e error) bool {
	var h *gitlabapi.HTTPError
	return errors.As(e, &h) && (h.StatusCode == 401 || h.StatusCode == 403 || h.StatusCode == 404 || h.StatusCode == 410 || h.StatusCode == 405 || h.StatusCode == 501)
}

type gitlabBlobs map[string][]byte

func (b gitlabBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	v, ok := b[d]
	if !ok {
		return nil, errors.New("blob outside GitLab snapshot")
	}
	return io.NopCloser(bytes.NewReader(v)), nil
}

type gitlabObservation struct {
	s       migration.Snapshot
	blobs   gitlabBlobs
	l       migration.Limits
	total   int64
	project string
	err     error
}

func (b *gitlabObservation) key(kind, id string) migration.Key {
	return migration.Key{Provider: "gitlab", Instance: b.s.Source.Endpoint, ProjectID: b.project, Kind: kind, ID: id}
}
func (b *gitlabObservation) finding(path, status, message string) {
	b.s.Findings = append(b.s.Findings, migration.Finding{ID: "gitlab-source-" + strconv.Itoa(len(b.s.Findings)), Path: path, Status: status, Message: message})
}
func (b *gitlabObservation) blob(raw []byte, name, media string) string {
	if b.err != nil {
		return ""
	}
	d := migration.Digest(raw)
	if _, ok := b.blobs[d]; ok {
		return d
	}
	if len(b.blobs) >= b.l.Blobs || int64(len(raw)) > b.l.BlobBytes || int64(len(raw)) > b.l.AttachmentBytes-b.total {
		b.err = errors.New("GitLab snapshot blob budget exceeded")
		return ""
	}
	b.total += int64(len(raw))
	b.blobs[d] = append([]byte{}, raw...)
	b.s.Blobs = append(b.s.Blobs, migration.Blob{Digest: d, Size: int64(len(raw)), Name: name, MediaType: media})
	return d
}
func (b *gitlabObservation) add(r migration.Record) {
	if b.err != nil {
		return
	}
	if len(b.s.Records) >= b.l.Records {
		b.err = errors.New("GitLab snapshot record budget exceeded")
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
func (b *gitlabObservation) preserve(kind, id, selector, url, author, created, updated string, raw []byte, links []migration.Link) migration.Key {
	d := b.blob(raw, kind+"-"+id, "application/json")
	k := b.key(kind, id)
	b.add(migration.Record{Key: k, Selector: selector, Revision: migration.Digest(raw), SourceURL: url, Author: author, CreatedAt: created, UpdatedAt: updated, Schema: "hn.migration.preserved/1", Content: gitlabJSON(migration.PreservedContent{Schema: "hn.migration.preserved/1", Summary: "Retrieved GitLab " + kind + "; foreign users/reviews/permissions/workflow remain descriptive and inert", SourceBlob: d}), Links: links, Blobs: []string{d}})
	return k
}
func (b *gitlabObservation) unavailable(kind string, e error) error {
	if gitlabUnavailable(e) {
		b.finding(kind, "missing_dependency", "GitLab resource unavailable for this permission/version/tier; no unread content claimed preserved")
		return nil
	}
	return e
}
func gitlabWant(sel migration.Selection, kind string) bool {
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
func (g *gitlabMigration) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	fail := func(e error) (migration.Snapshot, migration.BlobSource, error) { return migration.Snapshot{}, nil, e }
	if e := l.Validate(); e != nil {
		return fail(e)
	}
	normalized, e := gitlabapi.NewClient(sel.Source.Endpoint, "")
	if e != nil {
		return fail(e)
	}
	sel.Source.Endpoint = normalized.BaseURL()
	if sel.Source != g.Locator() {
		return fail(errors.New("GitLab source locator mismatch"))
	}
	wire := l.ManifestBytes + l.AttachmentBytes
	if wire > gitlabapi.MaxTotalBytes {
		wire = gitlabapi.MaxTotalBytes
	}
	c := g.client.BoundedReadClient(wire)
	p, e := c.GetProject(ctx, g.Locator().Repository)
	if e != nil {
		return fail(e)
	}
	project := strconv.FormatInt(p.ID, 10)
	b := gitlabObservation{s: migration.Snapshot{Schema: "hn.project-snapshot/1", Source: g.Locator(), SourceID: "gitlab:" + migration.Digest(gitlabJSON([]string{"gitlab", g.Locator().Endpoint, gitlabID(p.ID)}))[7:], Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}, blobs: gitlabBlobs{}, l: l, project: gitlabID(p.ID)}
	want := func(kind string) bool { return gitlabWant(sel, kind) }
	b.preserve("attachment", "project-metadata:"+gitlabID(p.ID), p.Path, "", "", "", "", p.Raw, nil)
	// Keep project metadata (including feature visibility/settings) inspectable,
	// without confusing this repository container with a planning board.
	if want("issue") || want("comment") || want("discussion") || want("reply") || want("inline_comment") {
		rows, err := c.ListIssues(ctx, project)
		if err != nil {
			if e = b.unavailable("issue", err); e != nil {
				return fail(e)
			}
		} else {
			for _, v := range rows {
				if v.ProjectID != p.ID {
					return fail(errors.New("issue belongs to different GitLab project"))
				}
				iid := strconv.Itoa(v.IID)
				key := b.key("issue", gitlabID(v.ID))
				rawDigest := b.blob(v.Raw, "issue-"+iid, "application/json")
				assignees := []string{}
				for _, u := range v.Assignees {
					assignees = append(assignees, u.Username)
				}
				labels := append([]string{}, v.Labels...)
				content := gitlabJSON(migration.IssueContent{Schema: "hn.migration.issue/1", Title: v.Title, Body: v.Body, State: v.State, Type: v.Type, Labels: labels, Assignees: assignees})
				_, valid := migration.CanonicalContent("issue", content)
				representable := valid == nil && !v.Confidential
				if representable {
					b.add(migration.Record{Key: key, Selector: iid, Revision: migration.Digest(v.Raw), SourceURL: v.WebURL, Author: v.Author.Username, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Schema: "hn.migration.issue/1", Content: content, Blobs: []string{rawDigest}})
				} else {
					key = b.preserve("attachment", "issue-source:"+gitlabID(v.ID), iid, v.WebURL, v.Author.Username, v.CreatedAt, v.UpdatedAt, v.Raw, nil)
					b.finding(iid, "unsupported", "Confidential or oversized issue preserved as original source bytes; no ordinary issue recreation")
				}
				parent := migration.Link{Relation: key.Kind, Target: key, Revision: migration.Digest(v.Raw)}
				noteKeys := map[int64]migration.Key{}
				if want("comment") || want("reply") || want("inline_comment") {
					notes, err := c.ListNotes(ctx, project, v.IID)
					if err != nil {
						if e = b.unavailable("issue-notes:"+iid, err); e != nil {
							return fail(e)
						}
					} else {
						for _, n := range notes {
							if n.NoteableID != 0 && n.NoteableID != v.ID {
								return fail(errors.New("GitLab issue note parent identity mismatch"))
							}
							nc := gitlabJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: n.Body})
							_, err := migration.CanonicalContent("comment", nc)
							nk := b.key("comment", gitlabID(n.ID))
							if representable && n.Ordinary() && err == nil {
								d := b.blob(n.Raw, "note-"+gitlabID(n.ID), "application/json")
								b.add(migration.Record{Key: nk, Selector: strconv.FormatInt(n.ID, 10), Revision: migration.Digest(n.Raw), SourceURL: gitlabNoteURL(v.WebURL, n.ID), Author: n.Author.Username, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, Schema: "hn.migration.comment/1", Content: nc, Links: []migration.Link{parent}, Blobs: []string{d}})
							} else {
								kind := "reply"
								if n.Type == "DiffNote" {
									kind = "inline_comment"
								}
								nk = b.preserve(kind, gitlabID(n.ID), strconv.FormatInt(n.ID, 10), gitlabNoteURL(v.WebURL, n.ID), n.Author.Username, n.CreatedAt, n.UpdatedAt, n.Raw, []migration.Link{parent})
								b.finding("note:"+gitlabID(n.ID), "preserved", "System/internal/confidential/thread/oversized note retained as source evidence, never ordinary user-authored destination comment")
							}
							noteKeys[n.ID] = nk
						}
					}
				}
				if want("discussion") || want("reply") || want("inline_comment") {
					ds, err := c.ListDiscussions(ctx, project, "issues", v.IID)
					if err != nil {
						if e = b.unavailable("issue-discussions:"+iid, err); e != nil {
							return fail(e)
						}
					} else {
						for _, d := range ds {
							dk := b.preserve("discussion", "issue:"+gitlabID(v.ID)+":"+d.ID, d.ID, v.WebURL, "", "", "", d.Raw, []migration.Link{parent})
							previous := migration.Key{}
							for _, n := range d.Notes {
								if n.NoteableID != 0 && n.NoteableID != v.ID {
									return fail(errors.New("GitLab discussion parent mismatch"))
								}
								nk, ok := noteKeys[n.ID]
								if ok {
									for i := range b.s.Records {
										if b.s.Records[i].Key == nk {
											// Thread topology is authoritative even if an older
											// notes endpoint omitted the descriptive note type.
											if nk.Kind == "comment" && (!d.Individual || !n.Ordinary()) {
												nk.Kind = "reply"
												if n.Type == "DiffNote" {
													nk.Kind = "inline_comment"
												}
												r := &b.s.Records[i]
												digest := b.blob(n.Raw, "thread-note-"+gitlabID(n.ID), "application/json")
												r.Key, r.Schema = nk, "hn.migration.preserved/1"
												r.Content = gitlabJSON(migration.PreservedContent{Schema: r.Schema, Summary: "Attached thread note remains inert source evidence", SourceBlob: digest})
												present := false
												for _, old := range r.Blobs {
													present = present || old == digest
												}
												if !present {
													r.Blobs = append(r.Blobs, digest)
												}
												noteKeys[n.ID] = nk
												b.finding("note:"+gitlabID(n.ID), "preserved", "Thread topology prevents ordinary comment recreation")
											}
											b.s.Records[i].Links = append(b.s.Records[i].Links, migration.Link{Relation: "discussion", Target: dk})
											if previous.ID != "" {
												b.s.Records[i].Links = append(b.s.Records[i].Links, migration.Link{Relation: "previous_note", Target: previous})
											}
										}
									}
								} else {
									kind := "reply"
									if n.Type == "DiffNote" {
										kind = "inline_comment"
									}
									links := []migration.Link{parent, {Relation: "discussion", Target: dk}}
									if previous.ID != "" {
										links = append(links, migration.Link{Relation: "previous_note", Target: previous})
									}
									nk = b.preserve(kind, gitlabID(n.ID), strconv.FormatInt(n.ID, 10), gitlabNoteURL(v.WebURL, n.ID), n.Author.Username, n.CreatedAt, n.UpdatedAt, n.Raw, links)
									noteKeys[n.ID] = nk
								}
								previous = nk
							}
						}
					}
				}
			}
		}
	}
	if want("change_request") || want("review") || want("discussion") || want("reply") || want("inline_comment") {
		rows, err := c.ListMergeRequests(ctx, project)
		if err != nil {
			if e = b.unavailable("change_request", err); e != nil {
				return fail(e)
			}
		} else {
			for _, v := range rows {
				if v.ProjectID != p.ID {
					return fail(errors.New("MR belongs to different GitLab project"))
				}
				key := b.preserve("change_request", gitlabID(v.ID), strconv.Itoa(v.IID), v.WebURL, v.Author.Username, v.CreatedAt, v.UpdatedAt, v.Raw, nil)
				link := migration.Link{Relation: "change_request", Target: key, Revision: migration.Digest(v.Raw)}
				if want("review") {
					raw, err := c.ReadApprovals(ctx, project, v.IID)
					if err != nil {
						if e = b.unavailable("review:"+strconv.Itoa(v.IID), err); e != nil {
							return fail(e)
						}
					} else {
						var observed struct {
							ID  int64 `json:"id"`
							IID int   `json:"iid"`
						}
						if json.Unmarshal(raw, &observed) != nil || observed.ID != v.ID || observed.IID != v.IID {
							return fail(errors.New("GitLab review parent identity changed"))
						}
						b.preserve("review", "mr:"+gitlabID(v.ID)+":approvals", strconv.Itoa(v.IID), v.WebURL, "", "", "", raw, []migration.Link{link})
					}
				}
				ds, err := c.ListDiscussions(ctx, project, "merge_requests", v.IID)
				if err != nil {
					if e = b.unavailable("mr-discussions:"+strconv.Itoa(v.IID), err); e != nil {
						return fail(e)
					}
				} else {
					for _, d := range ds {
						dk := b.preserve("discussion", "mr:"+gitlabID(v.ID)+":"+d.ID, d.ID, v.WebURL, "", "", "", d.Raw, []migration.Link{link})
						var previous migration.Key
						for _, n := range d.Notes {
							if n.NoteableID != 0 && n.NoteableID != v.ID {
								return fail(errors.New("GitLab MR note parent mismatch"))
							}
							kind := "reply"
							if n.Type == "DiffNote" {
								kind = "inline_comment"
							}
							links := []migration.Link{link, {Relation: "discussion", Target: dk}}
							if previous.ID != "" {
								links = append(links, migration.Link{Relation: "previous_note", Target: previous})
							}
							previous = b.preserve(kind, gitlabID(n.ID), strconv.FormatInt(n.ID, 10), gitlabNoteURL(v.WebURL, n.ID), n.Author.Username, n.CreatedAt, n.UpdatedAt, n.Raw, links)
						}
					}
				}
			}
		}
	}
	if want("project") || want("project_item") {
		if len(g.config.PlanningSelectors) == 0 {
			b.finding("boards", "missing_dependency", "Select project-local board IDs separately; repository project is not a planning board")
		} else {
			rows, err := c.ListBoards(ctx, project)
			if err != nil {
				if e = b.unavailable("boards", err); e != nil {
					return fail(e)
				}
			} else {
				found := map[string]bool{}
				for _, v := range rows {
					id := strconv.FormatInt(v.ID, 10)
					selected := false
					for _, x := range g.config.PlanningSelectors {
						selected = selected || x == id
					}
					if !selected {
						continue
					}
					found[id] = true
					key := b.preserve("project", "board:"+id, id, "", "", "", "", v.Raw, nil)
					if want("project_item") {
						n, _ := gitlabapi.PositiveKey(id)
						lists, err := c.ListBoardLists(ctx, project, n)
						if err != nil {
							if e = b.unavailable("board-lists:"+id, err); e != nil {
								return fail(e)
							}
						} else {
							for _, x := range lists {
								b.preserve("project_item", "board:"+id+":list:"+strconv.FormatInt(x.ID, 10), strconv.FormatInt(x.ID, 10), "", "", "", "", x.Raw, []migration.Link{{Relation: "board", Target: key}})
							}
						}
					}
				}
				for _, id := range g.config.PlanningSelectors {
					if !found[id] {
						b.finding("board:"+id, "missing_dependency", "Selected board not returned; deleted/inaccessible content not preserved")
					}
				}
			}
		}
	}
	if want("workflow") {
		ref := g.config.WorkflowRef
		if ref == "" {
			ref = p.DefaultBranch
		}
		if p.CIConfigPath != "" && p.CIConfigPath != ".gitlab-ci.yml" {
			b.finding("workflow", "unsupported", "Custom/external ci_config_path retained in project metadata; not silently replaced by default CI source")
			b.preserve("workflow", "ci-config-selection", p.CIConfigPath, "", "", "", "", p.Raw, nil)
		} else if ref == "" {
			b.finding("workflow", "missing_dependency", "No CI revision or default branch available")
		} else {
			v, err := c.ReadCI(ctx, project, ref)
			if err != nil {
				if e = b.unavailable("workflow", err); e != nil {
					return fail(e)
				}
			} else {
				b.preserve("workflow", v.Commit+":"+v.Path, v.Path, "", "", "", "", v.Raw, nil)
				d := b.blob(v.Content, v.Path, "text/yaml")
				if b.err == nil {
					last := &b.s.Records[len(b.s.Records)-1]
					last.Blobs = append(last.Blobs, d)
				}
				b.finding("workflow-includes", "unsupported", "Only pinned .gitlab-ci.yml source retrieved; includes, remote templates and execution authority are not recreated")
			}
		}
	}
	if len(g.config.Attachments) > l.Blobs {
		return fail(errors.New("selected GitLab upload count exceeds blob budget"))
	}
	for _, a := range g.config.Attachments {
		index := -1
		for i, r := range b.s.Records {
			if r.Key == a.Parent {
				index = i
				break
			}
		}
		if index < 0 {
			b.finding("attachment", "missing_dependency", "Selected upload parent was not retrieved")
			continue
		}
		raw, media, err := c.ReadAttachment(ctx, a.URL, l.BlobBytes)
		if err != nil {
			if gitlabUnavailable(err) || strings.Contains(err.Error(), "same-origin") {
				b.finding("attachment", "missing_dependency", "Selected upload inaccessible or outside credential origin; bytes not preserved")
				continue
			}
			return fail(err)
		}
		d := b.blob(raw, a.Name, media)
		if b.err == nil {
			duplicate := false
			for _, x := range b.s.Records[index].Blobs {
				duplicate = duplicate || x == d
			}
			if !duplicate {
				b.s.Records[index].Blobs = append(b.s.Records[index].Blobs, d)
			}
		}
	}
	if b.err != nil {
		return fail(b.err)
	}
	current, e := c.GetProject(ctx, g.Locator().Repository)
	if e != nil || gitlabProjectRevision(current) != gitlabProjectRevision(p) {
		return fail(errors.New("GitLab project changed during snapshot"))
	}
	raw, e := migration.CanonicalSnapshot(b.s, l)
	if e != nil {
		return fail(e)
	}
	_ = json.Unmarshal(raw, &b.s)
	return b.s, b.blobs, nil
}
func gitlabNoteURL(parent string, id int64) string {
	if parent == "" {
		return ""
	}
	return parent + "#note_" + strconv.FormatInt(id, 10)
}
func gitlabProjectRevision(p gitlabapi.Project) string {
	return gitlabRevision([]string{gitlabID(p.ID), p.CreatedAt})
}

// gl1 binds the authenticated plan to a project incarnation without extending
// frozen schemas: gl1.<project-baseline SHA256 hex>.<source-key SHA256 hex>.
func (g *gitlabMigration) baseline(p gitlabapi.Project) string {
	return migration.Digest(gitlabJSON([]string{g.Locator().Kind, g.Locator().Endpoint, g.Locator().Repository, gitlabProjectRevision(p)}))[7:]
}
func (g *gitlabMigration) intentID(p gitlabapi.Project, k migration.Key) string {
	return "gl1." + g.baseline(p) + "." + migration.Digest([]byte(k.String()))[7:]
}
func (g *gitlabMigration) checkProject(ctx context.Context, in migration.Intent) (gitlabapi.Project, error) {
	parts := strings.Split(in.ID, ".")
	if len(parts) != 3 || parts[0] != "gl1" {
		return gitlabapi.Project{}, errors.New("invalid GitLab intent project binding")
	}
	for _, s := range parts[1:] {
		if len(s) != 64 || strings.ToLower(s) != s {
			return gitlabapi.Project{}, errors.New("invalid GitLab intent digest")
		}
		if _, e := hex.DecodeString(s); e != nil {
			return gitlabapi.Project{}, e
		}
	}
	p, e := g.client.GetProject(ctx, g.Locator().Repository)
	if e != nil {
		return p, e
	}
	if parts[1] != g.baseline(p) {
		return p, errors.New("GitLab destination project incarnation changed")
	}
	if !p.IssuesEnabled {
		return p, errors.New("GitLab destination issue tracker is disabled or inaccessible")
	}
	return p, nil
}
func (g *gitlabMigration) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if sel.Destination != g.Locator() {
		return nil, nil, errors.New("GitLab destination locator mismatch")
	}
	if _, e := migration.CanonicalSnapshot(s, l); e != nil {
		return nil, nil, e
	}
	planner := *g
	wire := l.ManifestBytes + l.AttachmentBytes
	if wire > gitlabapi.MaxTotalBytes {
		wire = gitlabapi.MaxTotalBytes
	}
	planner.client = g.client.BoundedReadClient(wire)
	g = &planner
	p, e := g.client.GetProject(ctx, g.Locator().Repository)
	if e != nil {
		return nil, nil, e
	}
	project := strconv.FormatInt(p.ID, 10)
	out := []migration.Intent{}
	findings := []migration.Finding{}
	parents := map[string]migration.Intent{}
	finding := func(r migration.Record, status, message string) {
		findings = append(findings, migration.Finding{ID: "gitlab-destination-" + migration.Digest([]byte(r.Key.String()))[7:], Status: status, Path: r.Key.String(), Message: message})
	}
	for _, r := range s.Records {
		if r.Key.Kind != "issue" && r.Key.Kind != "work_item" {
			continue
		}
		if !p.IssuesEnabled {
			finding(r, "unsupported", "Destination project issue tracker disabled or not observable; no writes planned")
			continue
		}
		var v migration.IssueContent
		_ = json.Unmarshal(r.Content, &v)
		state, typ := v.State, v.Type
		if x, ok := sel.Mappings["state:"+state]; ok {
			state = x
		}
		if x, ok := sel.Mappings["type:"+typ]; ok {
			typ = x
		}
		if typ == "" && v.Type == "" && r.Key.Kind == "issue" {
			typ = "issue"
		}
		_, hasState := sel.Mappings["state:"+v.State]
		_, hasType := sel.Mappings["type:"+v.Type]
		if r.Key.Kind == "work_item" && (!hasState || !hasType) {
			finding(r, "requires_mapping", "Work items require explicit source type and state mappings to ordinary GitLab issues")
			continue
		}
		labels := []string{}
		assignees := []string{}
		missing := false
		for _, group := range []struct {
			prefix string
			values []string
			target *[]string
		}{{"label:", v.Labels, &labels}, {"assignee:", v.Assignees, &assignees}} {
			for _, x := range group.values {
				mapped, ok := sel.Mappings[group.prefix+x]
				if !ok {
					missing = true
				} else if mapped != "" {
					*group.target = append(*group.target, mapped)
				}
			}
		}
		if missing {
			finding(r, "requires_mapping", "Explicit label/assignee mappings or deliberate empty omissions required")
			continue
		}
		in := migration.Intent{ID: g.intentID(p, r.Key), Kind: "issue", Action: "create", Sources: []migration.Key{r.Key}, DependsOn: []string{}}
		w := migration.IssueWrite{Schema: "hn.migration.issue-write/1", Title: v.Title, Body: v.Body, State: state, Type: typ, Labels: labels, Assignees: assignees}
		if target := sel.Mappings["target:"+r.Key.String()]; target != "" {
			iid, err := gitlabapi.PositiveKey(target)
			if err != nil {
				return nil, nil, err
			}
			current, err := g.client.GetIssue(ctx, project, iid)
			if err != nil {
				return nil, nil, err
			}
			if current.ProjectID != p.ID {
				return nil, nil, errors.New("GitLab target project changed")
			}
			if current.Type != "issue" || current.Confidential {
				finding(r, "unsupported", "Existing confidential or nonordinary issue cannot be overwritten as ordinary issue")
				continue
			}
			in.Action = "update"
			in.ExpectedID = gitlabID(current.ID)
			in.ExpectedRevision = migration.Digest(current.Raw)
			w.Target = &migration.RemoteRef{ID: target}
		}
		if typ != "issue" || (state != "opened" && state != "closed") || in.Action == "create" && state != "opened" {
			finding(r, "requires_mapping", "GitLab ordinary issue type and opened creates required; closed updates allowed; use explicit source state/type mappings")
			continue
		}
		if _, err := gitlabIssueProjection(w, ""); err != nil {
			finding(r, "unsupported", err.Error())
			continue
		}
		in.Payload = gitlabJSON(w)
		if _, err := migration.CanonicalWrite(in, false); err != nil {
			return nil, nil, err
		}
		out = append(out, in)
		parents[r.Key.String()] = in
		finding(r, "mapped", "Issue projection mapped with deterministic API recovery marker; original source attribution/raw bytes stay in saved snapshot")
	}
	for _, r := range s.Records {
		if r.Key.Kind == "issue" || r.Key.Kind == "work_item" {
			continue
		}
		if r.Key.Kind != "comment" {
			finding(r, "unsupported", "Retrieved foreign "+r.Key.Kind+" retained in saved snapshot; no GitLab recreation")
			continue
		}
		var parent migration.Intent
		for _, link := range r.Links {
			if link.Relation == "issue" || link.Relation == "work_item" {
				parent = parents[link.Target.String()]
			}
		}
		if parent.ID == "" {
			finding(r, "missing_dependency", "Comment has no mapped ordinary issue parent")
			continue
		}
		var v migration.CommentContent
		_ = json.Unmarshal(r.Content, &v)
		if err := gitlabapi.ValidateBody(v.Body); err != nil {
			finding(r, "unsupported", err.Error())
			continue
		}
		w := migration.CommentWrite{Schema: "hn.migration.comment-write/1", Parent: migration.RemoteRef{Receipt: &migration.ReceiptRef{IntentID: parent.ID, Field: "remote_key"}}, Body: v.Body}
		in := migration.Intent{ID: g.intentID(p, r.Key), Kind: "comment", Action: "create", Sources: []migration.Key{r.Key}, DependsOn: []string{parent.ID}, Payload: gitlabJSON(w)}
		if target := sel.Mappings["target:"+r.Key.String()]; target != "" {
			if parent.Action != "update" {
				finding(r, "missing_dependency", "Existing note update requires an explicitly mapped existing parent issue")
				continue
			}
			noteID, err := gitlabapi.PositiveKey(target)
			if err != nil {
				return nil, nil, err
			}
			var parentWrite migration.IssueWrite
			_ = json.Unmarshal(parent.Payload, &parentWrite)
			iid, err := gitlabapi.PositiveKey(parentWrite.Target.ID)
			if err != nil {
				return nil, nil, err
			}
			current, err := g.client.GetNote(ctx, project, iid, int64(noteID))
			if err != nil {
				return nil, nil, err
			}
			parentID, _ := strconv.ParseInt(strings.TrimPrefix(parent.ExpectedID, "id:"), 10, 64)
			if !current.Ordinary() || current.NoteableID != parentID {
				finding(r, "unsupported", "Existing note is system/internal/confidential/threaded or belongs to another issue")
				continue
			}
			w.Target = &migration.RemoteRef{ID: target}
			in.Action = "update"
			in.ExpectedID = gitlabID(current.ID)
			in.ExpectedRevision = migration.Digest(current.Raw)
			in.Payload = gitlabJSON(w)
		}
		if _, err := migration.CanonicalWrite(in, false); err != nil {
			return nil, nil, err
		}
		out = append(out, in)
		finding(r, "mapped", "Ordinary note mapped to confirmed issue IID and immutable parent receipt; source attribution retained, API body includes recovery marker")
	}
	return out, findings, nil
}
func gitlabIssueProjection(w migration.IssueWrite, marker string) (gitlabapi.IssueInput, error) {
	if e := gitlabapi.ValidateBody(w.Body); e != nil {
		return gitlabapi.IssueInput{}, e
	}
	if w.Type != "issue" || w.State != "opened" && w.State != "closed" || len(w.Title) > 255 || len(w.Body+marker) > 1000000 {
		return gitlabapi.IssueInput{}, errors.New("GitLab ordinary issue type/state/title/body capacity unsupported; source remains preserved without truncation")
	}
	for _, label := range w.Labels {
		if strings.ContainsAny(label, ",\x00\r\n") || label == "" {
			return gitlabapi.IssueInput{}, errors.New("Mapped GitLab labels cannot contain commas/control characters")
		}
	}
	for _, id := range w.Assignees {
		if _, e := gitlabapi.PositiveKey(id); e != nil {
			return gitlabapi.IssueInput{}, errors.New("Mapped GitLab assignees require positive numeric user IDs")
		}
	}
	return gitlabapi.IssueInput{Title: w.Title, Body: w.Body + marker, State: w.State, Type: w.Type, Labels: w.Labels, Assignees: w.Assignees}, nil
}
func (g *gitlabMigration) bound(in migration.Intent, ac migration.ApplyContext) (migration.Intent, error) {
	w := ac.BoundWrite()
	if w.IntentID != in.ID || !migration.ValidDigest(w.PlanDigest) || w.PayloadDigest != migration.Digest(w.Payload) {
		return in, errors.New("GitLab context does not bind intent")
	}
	in.Payload = w.Payload
	if _, e := migration.CanonicalWrite(in, true); e != nil {
		return in, e
	}
	if in.Kind != "issue" && in.Kind != "comment" || in.Action != "create" && in.Action != "update" {
		return in, errors.New("unsupported GitLab migration write")
	}
	return in, nil
}
func gitlabMarker(ac migration.ApplyContext) string {
	w := ac.BoundWrite()
	return "\n\n<!-- hn-migration:" + w.PlanDigest + ":" + w.IntentID + ":" + w.PayloadDigest + " -->"
}
func (g *gitlabMigration) receipt(in migration.Intent, ac migration.ApplyContext, id, key, revision string) migration.Receipt {
	r := migrationReceipt(in, ac, g.Locator(), id, revision)
	r.RemoteKey = key
	return r
}
func (g *gitlabMigration) parent(ctx context.Context, p gitlabapi.Project, in migration.Intent, w migration.CommentWrite, ac migration.ApplyContext) (int, error) {
	iid, e := gitlabapi.PositiveKey(w.Parent.ID)
	if e != nil {
		return 0, e
	}
	expected := ""
	for _, id := range in.DependsOn {
		r, ok := ac.Dependency(id)
		if !ok || r.Destination != g.Locator() {
			return 0, errors.New("GitLab comment dependency unconfirmed")
		}
		if r.Kind == "issue" && r.RemoteKey == w.Parent.ID {
			if expected != "" {
				return 0, errors.New("ambiguous GitLab parent receipt")
			}
			expected = r.RemoteID
		}
	}
	if expected == "" {
		return 0, errors.New("GitLab note needs immutable confirmed parent")
	}
	v, e := g.client.GetIssue(ctx, strconv.FormatInt(p.ID, 10), iid)
	if e != nil {
		return 0, e
	}
	if gitlabID(v.ID) != expected || v.ProjectID != p.ID || v.Type != "issue" || v.Confidential {
		return 0, errors.New("GitLab note parent changed")
	}
	return iid, nil
}
func gitlabIssueMatches(v gitlabapi.Issue, in gitlabapi.IssueInput, project int64) bool {
	if v.ProjectID != project || v.Confidential || v.Title != in.Title || v.Body != in.Body || v.State != in.State || v.Type != in.Type {
		return false
	}
	ids := []string{}
	for _, u := range v.Assignees {
		ids = append(ids, strconv.FormatInt(u.ID, 10))
	}
	same := func(a, b []string) bool {
		a = append([]string{}, a...)
		b = append([]string{}, b...)
		sort.Strings(a)
		sort.Strings(b)
		return strings.Join(a, "\x00") == strings.Join(b, "\x00")
	}
	return same(v.Labels, in.Labels) && same(ids, in.Assignees)
}
func (g *gitlabMigration) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	in, e := g.bound(in, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	p, e := g.checkProject(ctx, in)
	if e != nil {
		return migration.Receipt{}, e
	}
	project := strconv.FormatInt(p.ID, 10)
	if in.Kind == "issue" {
		var w migration.IssueWrite
		_ = json.Unmarshal(in.Payload, &w)
		input, e := gitlabIssueProjection(w, gitlabMarker(ac))
		if e != nil {
			return migration.Receipt{}, e
		}
		var v gitlabapi.Issue
		if in.Action == "update" {
			iid, e := gitlabapi.PositiveKey(w.Target.ID)
			if e != nil {
				return migration.Receipt{}, e
			}
			current, e := g.client.GetIssue(ctx, project, iid)
			if e != nil {
				return migration.Receipt{}, e
			}
			if in.ExpectedID == "" || in.ExpectedRevision == "" || gitlabID(current.ID) != in.ExpectedID || migration.Digest(current.Raw) != in.ExpectedRevision || current.ProjectID != p.ID || current.Confidential || current.Type != "issue" {
				return migration.Receipt{}, errors.New("GitLab issue baseline changed")
			}
			v, e = g.client.UpdateIssue(ctx, project, iid, input)
			if e != nil {
				return migration.Receipt{}, e
			}
		} else {
			v, e = g.client.CreateIssue(ctx, project, input)
			if e != nil {
				return migration.Receipt{}, e
			}
		}
		if !gitlabIssueMatches(v, input, p.ID) || in.ExpectedID != "" && gitlabID(v.ID) != in.ExpectedID {
			return migration.Receipt{}, errors.New("GitLab returned different issue projection; recovery required")
		}
		return g.receipt(in, ac, gitlabID(v.ID), strconv.Itoa(v.IID), migration.Digest(v.Raw)), nil
	}
	var w migration.CommentWrite
	_ = json.Unmarshal(in.Payload, &w)
	iid, e := g.parent(ctx, p, in, w, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	body := w.Body + gitlabMarker(ac)
	var v gitlabapi.Note
	if in.Action == "update" {
		noteID, err := gitlabapi.PositiveKey(w.Target.ID)
		if err != nil {
			return migration.Receipt{}, err
		}
		current, err := g.client.GetNote(ctx, project, iid, int64(noteID))
		if err != nil {
			return migration.Receipt{}, err
		}
		if in.ExpectedID == "" || in.ExpectedRevision == "" || gitlabID(current.ID) != in.ExpectedID || migration.Digest(current.Raw) != in.ExpectedRevision || !current.Ordinary() || current.NoteableID != gitlabParentID(ac, in, w.Parent.ID) {
			return migration.Receipt{}, errors.New("GitLab note baseline changed")
		}
		v, e = g.client.UpdateNote(ctx, project, iid, int64(noteID), body)
	} else {
		v, e = g.client.CreateNote(ctx, project, iid, body)
	}
	if e != nil {
		return migration.Receipt{}, e
	}
	if in.ExpectedID != "" && gitlabID(v.ID) != in.ExpectedID || v.Body != body || !v.Ordinary() || v.NoteableID != 0 && v.NoteableID != gitlabParentID(ac, in, w.Parent.ID) {
		return migration.Receipt{}, errors.New("GitLab returned different note projection; recovery required")
	}
	return g.receipt(in, ac, gitlabID(v.ID), strconv.FormatInt(v.ID, 10), migration.Digest(v.Raw)), nil
}
func gitlabParentID(ac migration.ApplyContext, in migration.Intent, key string) int64 {
	for _, id := range in.DependsOn {
		r, ok := ac.Dependency(id)
		if ok && r.Kind == "issue" && r.RemoteKey == key {
			n, _ := strconv.ParseInt(strings.TrimPrefix(r.RemoteID, "id:"), 10, 64)
			return n
		}
	}
	return 0
}
func (g *gitlabMigration) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	in, e := g.bound(in, ac)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	p, e := g.checkProject(ctx, in)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	project := strconv.FormatInt(p.ID, 10)
	matches := []migration.Receipt{}
	if in.Kind == "issue" {
		var w migration.IssueWrite
		_ = json.Unmarshal(in.Payload, &w)
		input, e := gitlabIssueProjection(w, gitlabMarker(ac))
		if e != nil {
			return migration.Reconciliation{}, e
		}
		var rows []gitlabapi.Issue
		if in.Action == "update" {
			iid, e := gitlabapi.PositiveKey(w.Target.ID)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			v, e := g.client.GetIssue(ctx, project, iid)
			if e != nil {
				return migration.Reconciliation{}, e
			}
			rows = []gitlabapi.Issue{v}
		} else {
			rows, e = g.client.ListIssues(ctx, project)
			if e != nil {
				return migration.Reconciliation{}, e
			}
		}
		for _, v := range rows {
			if gitlabIssueMatches(v, input, p.ID) && (in.ExpectedID == "" || gitlabID(v.ID) == in.ExpectedID) {
				matches = append(matches, g.receipt(in, ac, gitlabID(v.ID), strconv.Itoa(v.IID), migration.Digest(v.Raw)))
			}
		}
	} else {
		var w migration.CommentWrite
		_ = json.Unmarshal(in.Payload, &w)
		iid, e := g.parent(ctx, p, in, w, ac)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		var rows []gitlabapi.Note
		if in.Action == "update" {
			noteID, err := gitlabapi.PositiveKey(w.Target.ID)
			if err != nil {
				return migration.Reconciliation{}, err
			}
			v, err := g.client.GetNote(ctx, project, iid, int64(noteID))
			if err != nil {
				return migration.Reconciliation{}, err
			}
			rows = []gitlabapi.Note{v}
		} else {
			rows, e = g.client.ListNotes(ctx, project, iid)
			if e != nil {
				return migration.Reconciliation{}, e
			}
		}
		for _, v := range rows {
			if (in.ExpectedID == "" || gitlabID(v.ID) == in.ExpectedID) && v.Body == w.Body+gitlabMarker(ac) && v.Ordinary() && (v.NoteableID == 0 || v.NoteableID == gitlabParentID(ac, in, w.Parent.ID)) {
				matches = append(matches, g.receipt(in, ac, gitlabID(v.ID), strconv.FormatInt(v.ID, 10), migration.Digest(v.Raw)))
			}
		}
	}
	switch len(matches) {
	case 1:
		return migration.Reconciliation{Status: "exact", Receipt: &matches[0], Reason: "Unique exact GitLab projection and bound recovery marker observed"}, nil
	case 0:
		return migration.Reconciliation{Status: "no_proof", Reason: "No exact positive evidence; absence never permits another create"}, nil
	default:
		return migration.Reconciliation{Status: "ambiguous", Reason: fmt.Sprintf("%d exact marker matches; manual recovery required", len(matches))}, nil
	}
}
