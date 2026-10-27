//go:build !noforge

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"

	"hubnot/internal/bitbucketapi"
	"hubnot/internal/migration"
)

func init() {
	if e := migrationProviders.Register(migration.ProviderRegistration{Kind: "bitbucket", TokenEnv: "HN_BITBUCKET_TOKEN", RequiredCoverage: []migration.CoverageRequirement{{Kind: "repository", Read: true, Preserve: true}, {Kind: "change_request", Read: true, Preserve: true}, {Kind: "review", Read: true, Preserve: true}, {Kind: "inline_comment", Read: true, Preserve: true}, {Kind: "workflow", Read: true, Preserve: true}}, NewSource: func(c migration.ProviderConfig) (migration.Source, error) { return newBitbucketMigration(c) }, NewDestination: func(c migration.ProviderConfig) (migration.Destination, error) { return newBitbucketMigration(c) }}); e != nil {
		panic(e)
	}
}

type bitbucketMigration struct {
	config migration.ProviderConfig
	client *bitbucketapi.Client
}

func newBitbucketMigration(c migration.ProviderConfig) (*bitbucketMigration, error) {
	if c.Locator.Kind != "bitbucket" || c.Locator.Project != "" || c.Locator.Organization != "" || len(c.PlanningSelectors) > 0 {
		return nil, errors.New("Bitbucket Cloud requires workspace/slug repository; workspace projects are not planning boards")
	}
	if c.APIVersion != "2.0" {
		return nil, errors.New("explicit Bitbucket Cloud API profile 2.0 required")
	}
	if c.WorkflowRef != "" && !migration.ValidOID(c.WorkflowRef) {
		return nil, errors.New("Bitbucket pipeline source requires explicit immutable commit hash")
	}
	if e := bitbucketapi.ValidateRepository(c.Locator.Repository); e != nil {
		return nil, e
	}
	client, e := bitbucketapi.NewClient(c.Locator.Endpoint, c.Token, c.CredentialMode)
	if e != nil {
		return nil, e
	}
	c.Locator.Endpoint = client.BaseURL()
	c.Attachments = append([]migration.AttachmentSelection{}, c.Attachments...)
	return &bitbucketMigration{config: c, client: client}, nil
}
func (b *bitbucketMigration) Locator() migration.Locator { return b.config.Locator }
func (b *bitbucketMigration) Capabilities(context.Context) ([]migration.Capability, error) {
	out := []migration.Capability{}
	for _, kind := range []string{"repository", "change_request", "review", "inline_comment", "reply", "workflow", "attachment"} {
		out = append(out, migration.Capability{Kind: kind, Read: true, Preserve: true, Limits: []string{"Cloud REST 2.0; preservation-only source; no destination-native writes in frozen v1", "repository metadata is an inert attachment record; shared Git companion transports selected code", "PR comments include global/inline/reply topology; review records are content-identified activity observations, not native approvals", "workflow requires explicit full commit hash; bitbucket-pipelines.yml preserved as exact inert bytes", "100 pages/connection; 10000 rows/connection; 4 MiB JSON response; 64 MiB aggregate wire ceiling; selected snapshot budgets also apply", "explicit same-origin API attachments only; foreign authors/times are attribution, never signing authority"}})
	}
	for _, kind := range []string{"issue", "comment", "project", "project_item", "discussion"} {
		reason := "No standalone discussion/planning-board API in this profile; workspace projects group repositories and are not planning boards"
		if kind == "issue" || kind == "comment" {
			reason = "Native Issue Tracker and issue-comment APIs removed 2026-08-20; no Jira or Data Center substitution"
		}
		out = append(out, migration.Capability{Kind: kind, Reason: reason})
	}
	return out, nil
}
func bitbucketJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }
func bitbucketFinding(id, status, path, message string) migration.Finding {
	return migration.Finding{ID: "bitbucket-" + id, Status: status, Path: path, Message: message}
}
func bitbucketUnavailable(e error) bool {
	var h *bitbucketapi.HTTPError
	return errors.As(e, &h) && (h.StatusCode == http.StatusUnauthorized || h.StatusCode == http.StatusForbidden || h.StatusCode == http.StatusNotFound || h.StatusCode == http.StatusGone)
}

// Activity variants expose provenance under different nested keys. Original
// bytes remain authoritative, including variants with no author/date fields.
func bitbucketActivityAttribution(raw []byte) (author, created, updated string) {
	var variants map[string]json.RawMessage
	if json.Unmarshal(raw, &variants) != nil {
		return
	}
	type attribution struct {
		User      bitbucketapi.Account `json:"user"`
		Author    bitbucketapi.Account `json:"author"`
		Date      string               `json:"date"`
		CreatedOn string               `json:"created_on"`
		UpdatedOn string               `json:"updated_on"`
	}
	for _, kind := range []string{"approval", "update", "comment", "changes_requested"} {
		if data, ok := variants[kind]; ok {
			var v attribution
			if json.Unmarshal(data, &v) != nil {
				return
			}
			author, created, updated = v.User.UUID, v.CreatedOn, v.UpdatedOn
			if author == "" {
				author = v.Author.UUID
			}
			if created == "" {
				created = v.Date
			}
			return
		}
	}
	return
}
func (b *bitbucketMigration) normalize(l migration.Locator) (migration.Locator, error) {
	// Normalize without looking up credentials or contacting the provider.
	c, e := bitbucketapi.NewClient(l.Endpoint, "", "anonymous")
	if e != nil {
		return l, e
	}
	l.Endpoint = c.BaseURL()
	return l, nil
}

type bitbucketBlobs map[string][]byte

func (b bitbucketBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, ok := b[d]
	if !ok {
		return nil, errors.New("blob outside selected Bitbucket snapshot")
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}

type bitbucketSnapshot struct {
	s       migration.Snapshot
	blobs   bitbucketBlobs
	l       migration.Limits
	total   int64
	project string
	err     error
}

func (b *bitbucketSnapshot) key(kind, id string) migration.Key {
	return migration.Key{Provider: "bitbucket", Instance: b.s.Source.Endpoint, ProjectID: b.project, Kind: kind, ID: id}
}
func (b *bitbucketSnapshot) finding(id, status, path, message string) {
	b.s.Findings = append(b.s.Findings, bitbucketFinding(id, status, path, message))
}
func (b *bitbucketSnapshot) blob(raw []byte, name, media string) string {
	if b.err != nil {
		return ""
	}
	d := migration.Digest(raw)
	if _, ok := b.blobs[d]; ok {
		return d
	}
	if len(b.blobs) >= b.l.Blobs || int64(len(raw)) > b.l.BlobBytes || int64(len(raw)) > b.l.AttachmentBytes-b.total {
		b.err = errors.New("Bitbucket snapshot blob budget exceeded")
		return ""
	}
	b.total += int64(len(raw))
	b.blobs[d] = append([]byte{}, raw...)
	b.s.Blobs = append(b.s.Blobs, migration.Blob{Digest: d, Size: int64(len(raw)), Name: name, MediaType: media})
	return d
}
func (b *bitbucketSnapshot) preserve(kind, id, selector, sourceURL, author, created, updated string, raw []byte, links []migration.Link) {
	if b.err != nil {
		return
	}
	if len(b.s.Records) >= b.l.Records {
		b.err = errors.New("Bitbucket snapshot record budget exceeded")
		return
	}
	d := b.blob(raw, kind+"-"+migration.Digest([]byte(id))[7:], "application/json")
	if b.err != nil {
		return
	}
	if links == nil {
		links = []migration.Link{}
	}
	b.s.Records = append(b.s.Records, migration.Record{Key: b.key(kind, id), Selector: selector, Revision: migration.Digest(raw), SourceURL: sourceURL, Author: author, CreatedAt: created, UpdatedAt: updated, Schema: "hn.migration.preserved/1", Content: bitbucketJSON(migration.PreservedContent{Schema: "hn.migration.preserved/1", Summary: "Retrieved Bitbucket Cloud " + kind + "; inert foreign observation", SourceBlob: d}), Links: links, Blobs: []string{d}})
}
func (b *bitbucketMigration) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	if e := l.Validate(); e != nil {
		return migration.Snapshot{}, nil, e
	}
	source, e := b.normalize(sel.Source)
	if e != nil || source != b.Locator() {
		return migration.Snapshot{}, nil, errors.New("Bitbucket source differs from selected locator")
	}
	allowed := map[string]bool{}
	caps, _ := b.Capabilities(ctx)
	for _, c := range caps {
		allowed[c.Kind] = c.Read
	}
	for _, k := range sel.Kinds {
		if !allowed[k] {
			return migration.Snapshot{}, nil, fmt.Errorf("Bitbucket selected %s unavailable; inspect capabilities before reads; use native/archive preservation of available resources", k)
		}
	}
	want := func(k string) bool {
		if len(sel.Kinds) == 0 {
			return true
		}
		for _, s := range sel.Kinds {
			if s == k {
				return true
			}
		}
		return false
	}
	if len(b.config.Attachments) > l.Blobs {
		return migration.Snapshot{}, nil, errors.New("selected Bitbucket attachment count budget exceeded")
	}
	c := b.client.WithBudget(l.ManifestBytes + l.AttachmentBytes)
	repo, e := c.Repository(ctx, b.Locator().Repository)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	builder := bitbucketSnapshot{s: migration.Snapshot{Schema: "hn.project-snapshot/1", Source: b.Locator(), SourceID: "bitbucket:" + migration.Digest(bitbucketJSON([]string{b.Locator().Endpoint, repo.UUID}))[7:], Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}, blobs: bitbucketBlobs{}, l: l, project: repo.UUID}
	web := "https://bitbucket.org/" + repo.FullName
	builder.preserve("attachment", "repository:"+repo.UUID, repo.FullName, web, "", repo.CreatedOn, "", repo.Raw, nil)
	builder.finding("repository", "preserved", "repository", "Retrieved repository UUID metadata preserved as inert attachment; selected code refs use the shared Git companion")
	for _, cap := range caps {
		if !cap.Read && len(sel.Kinds) == 0 {
			builder.finding("source-"+cap.Kind, "unsupported", cap.Kind, cap.Reason)
		}
	}
	missing := func(id, path string, err error) error {
		if !bitbucketUnavailable(err) {
			return err
		}
		builder.finding(id, "missing_dependency", path, "Selected Bitbucket content is inaccessible/private/deleted; no unread bytes claimed preserved")
		return nil
	}
	if want("change_request") || want("review") || want("inline_comment") || want("reply") {
		prs, err := c.PullRequests(ctx, repo.FullName)
		if err != nil {
			if e = missing("prs", "change_request", err); e != nil {
				return migration.Snapshot{}, nil, e
			}
		} else {
			for _, pr := range prs {
				n := strconv.FormatInt(pr.ID, 10)
				id := "pr:" + n
				url := web + "/pull-requests/" + n
				builder.preserve("change_request", id, n, url, pr.Author.UUID, pr.CreatedOn, pr.UpdatedOn, pr.Raw, nil)
				parent := builder.key("change_request", id)
				if want("inline_comment") || want("reply") {
					comments, err := c.Comments(ctx, repo.FullName, pr.ID)
					if err != nil {
						if e = missing("comments-"+n, "inline_comment", err); e != nil {
							return migration.Snapshot{}, nil, e
						}
					} else {
						kinds := map[int64]string{}
						for _, comment := range comments {
							kind := "reply"
							if comment.Inline != nil && comment.Parent == nil {
								kind = "inline_comment"
							}
							kinds[comment.ID] = kind
						}
						for _, comment := range comments {
							cid := id + ":comment:" + strconv.FormatInt(comment.ID, 10)
							links := []migration.Link{{Relation: "change_request", Target: parent}}
							if comment.Parent != nil {
								kind, ok := kinds[comment.Parent.ID]
								if ok {
									links = append(links, migration.Link{Relation: "parent", Target: builder.key(kind, id+":comment:"+strconv.FormatInt(comment.Parent.ID, 10))})
								} else {
									builder.finding("parent-"+cid, "missing_dependency", cid, "PR comment parent was not retrieved; original parent ID retained in exact source bytes")
								}
							}
							builder.preserve(kinds[comment.ID], cid, strconv.FormatInt(comment.ID, 10), url, comment.User.UUID, comment.CreatedOn, comment.UpdatedOn, comment.Raw, links)
							if comment.Deleted {
								builder.finding("deleted-"+cid, "missing_dependency", cid, "PR comment deleted; retrieved tombstone retained, deleted text unavailable")
							}
						}
					}
				}
				if want("review") {
					activity, err := c.Activity(ctx, repo.FullName, pr.ID)
					if err != nil {
						if e = missing("activity-"+n, "review", err); e != nil {
							return migration.Snapshot{}, nil, e
						}
					} else {
						counts := map[string]int{}
						for _, raw := range activity {
							digest := migration.Digest(raw)
							counts[digest]++
							aid := id + ":activity:" + digest + ":" + strconv.Itoa(counts[digest])
							author, created, updated := bitbucketActivityAttribution(raw)
							builder.preserve("review", aid, n, url, author, created, updated, raw, []migration.Link{{Relation: "change_request", Target: parent}})
						}
						builder.finding("activity-identity-"+n, "preserved", id, "Activity lacks universal event IDs; exact attributed source bytes retained as content-identified observations, not native review authority")
					}
				}
				if builder.err != nil {
					return migration.Snapshot{}, nil, builder.err
				}
			}
		}
	}
	if want("workflow") {
		if b.config.WorkflowRef == "" {
			builder.finding("pipeline-ref", "missing_dependency", "workflow", "Select an immutable commit hash to retrieve bitbucket-pipelines.yml; no mutable default revision inferred")
		} else {
			raw, err := c.Pipeline(ctx, repo.FullName, b.config.WorkflowRef, l.BlobBytes)
			if err != nil {
				if e = missing("pipeline", "workflow", err); e != nil {
					return migration.Snapshot{}, nil, e
				}
			} else {
				// The source blob itself is the exact YAML byte stream, not a JSON wrapper.
				d := builder.blob(raw, "bitbucket-pipelines.yml", "text/plain")
				if builder.err == nil {
					builder.s.Records = append(builder.s.Records, migration.Record{Key: builder.key("workflow", b.config.WorkflowRef+":bitbucket-pipelines.yml"), Selector: "bitbucket-pipelines.yml", Revision: b.config.WorkflowRef, SourceURL: web + "/src/" + b.config.WorkflowRef + "/bitbucket-pipelines.yml", Schema: "hn.migration.preserved/1", Content: bitbucketJSON(migration.PreservedContent{Schema: "hn.migration.preserved/1", Summary: "Exact pinned pipeline source; inert until separately converted and authorized", SourceBlob: d}), Links: []migration.Link{}, Blobs: []string{d}})
				}
			}
		}
	}
	for i, a := range b.config.Attachments {
		if !want("attachment") {
			return migration.Snapshot{}, nil, errors.New("attachment config requires attachment selection")
		}
		parent := -1
		for j, r := range builder.s.Records {
			if r.Key == a.Parent {
				parent = j
				break
			}
		}
		if parent < 0 {
			builder.finding("attachment-parent-"+strconv.Itoa(i), "missing_dependency", "attachment", "Selected attachment parent was not retrieved")
			continue
		}
		raw, err := c.Attachment(ctx, a.URL, l.BlobBytes)
		if err != nil {
			if bitbucketUnavailable(err) {
				builder.finding("attachment-"+strconv.Itoa(i), "missing_dependency", "attachment", "Selected attachment unavailable; no unread bytes preserved")
				continue
			}
			return migration.Snapshot{}, nil, err
		}
		digest := builder.blob(raw, a.Name, "application/octet-stream")
		if builder.err == nil {
			r := &builder.s.Records[parent]
			duplicate := false
			for _, d := range r.Blobs {
				duplicate = duplicate || d == digest
			}
			if !duplicate {
				r.Blobs = append(r.Blobs, digest)
			}
		}
	}
	if builder.err != nil {
		return migration.Snapshot{}, nil, builder.err
	}
	current, e := c.Repository(ctx, repo.FullName)
	if e != nil || current.UUID != repo.UUID || current.CreatedOn != repo.CreatedOn {
		return migration.Snapshot{}, nil, errors.New("Bitbucket repository incarnation changed during snapshot")
	}
	raw, e := migration.CanonicalSnapshot(builder.s, l)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	if e = json.Unmarshal(raw, &builder.s); e != nil {
		return migration.Snapshot{}, nil, e
	}
	return builder.s, builder.blobs, nil
}

// The frozen v1 writes only represent issue/work-item parents. They cannot
// represent PR targets, branches, thread positions, or repository creation.
// Refuse rather than inventing a second payload schema or an unbound effect.
func (b *bitbucketMigration) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if e := ctx.Err(); e != nil {
		return nil, nil, e
	}
	dest, e := b.normalize(sel.Destination)
	if e != nil || dest != b.Locator() {
		return nil, nil, errors.New("Bitbucket destination differs from selection")
	}
	if _, e = migration.CanonicalSnapshot(s, l); e != nil {
		return nil, nil, e
	}
	findings := []migration.Finding{bitbucketFinding("destination", "unsupported", "destination", "Cloud destination has no representable frozen v1 writes: Issue Tracker removed; PR/thread/workflow/repository recreation unavailable. Preserve communications in a native repository or portable archive; shared Git companion handles selected code only")}
	kinds := map[string]bool{}
	for _, r := range s.Records {
		kinds[r.Key.Kind] = true
	}
	names := []string{}
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		findings = append(findings, bitbucketFinding("destination-"+k, "unsupported", k, "Retrieved "+k+" content remains in source snapshot; Bitbucket destination does not recreate it. Explicit native/archive preservation or loss decision required"))
	}
	return []migration.Intent{}, findings, nil
}
func (b *bitbucketMigration) Apply(context.Context, migration.Intent, migration.ApplyContext) (migration.Receipt, error) {
	return migration.Receipt{}, errors.New("Bitbucket destination writes unsupported by frozen v1; no side effect or receipt")
}
func (b *bitbucketMigration) Reconcile(context.Context, migration.Intent, migration.ApplyContext) (migration.Reconciliation, error) {
	return migration.Reconciliation{}, errors.New("Bitbucket has no supported write to reconcile; operator must inspect original destination")
}
