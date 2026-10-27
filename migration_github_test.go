//go:build !nogithub && !noforge

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"hubnot/internal/githubapi"
	"hubnot/internal/migration"
)

type githubMigrationFixture struct {
	mu                                                                    sync.Mutex
	server                                                                *httptest.Server
	repoID                                                                int64
	issue                                                                 githubapi.Issue
	comments                                                              []githubapi.Comment
	issueWrites, commentWrites                                            int
	loseIssue, loseComment, hideIssue, hideComments, duplicate, dropLabel bool
	defaultType                                                           string
	pageCycle                                                             bool
}

func newGithubMigrationFixture(t *testing.T) *githubMigrationFixture {
	f := &githubMigrationFixture{repoID: 42, comments: []githubapi.Comment{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer supplied-secret" {
			t.Error("missing explicit token")
		}
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch {
		case r.URL.Path == "/repos/a/b":
			write(githubapi.Repository{ID: f.repoID, NodeID: fmt.Sprintf("R%d", f.repoID), FullName: "a/b", CreatedAt: "2026-01-01T00:00:00Z", HasIssues: true})
		case r.URL.Path == "/repos/a/b/issues" && r.Method == "POST":
			var in githubapi.IssueInput
			if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
				t.Error(e)
			}
			if in.State != "" {
				t.Error("create sent state unsupported by API")
			}
			f.issueWrites++
			f.issue = githubapi.Issue{ID: 7001, NodeID: "I7001", Number: 7, Title: in.Title, Body: in.Body, State: "open", CreatedAt: "2026-01-02T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z", User: githubapi.User{Login: "destination-user"}, Labels: []githubapi.Label{}, Assignees: []githubapi.User{}}
			for _, x := range in.Labels {
				if !f.dropLabel {
					f.issue.Labels = append(f.issue.Labels, githubapi.Label{Name: x})
				}
			}
			for _, x := range in.Assignees {
				f.issue.Assignees = append(f.issue.Assignees, githubapi.User{Login: x})
			}
			if f.defaultType != "" {
				f.issue.Type = &githubapi.IssueType{Name: f.defaultType}
			}
			if f.loseIssue {
				f.loseIssue = false
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(201)
			write(f.issue)
		case r.URL.Path == "/repos/a/b/issues" && r.Method == "GET":
			rows := []githubapi.Issue{}
			if f.issue.ID > 0 && !f.hideIssue {
				rows = append(rows, f.issue)
			}
			if f.duplicate && len(rows) > 0 {
				x := f.issue
				x.ID++
				x.NodeID = "other"
				x.Number++
				rows = append(rows, x)
			}
			if f.pageCycle {
				w.Header().Set("Link", fmt.Sprintf(`<http://%s/repos/a/b/issues?state=all&per_page=100&page=1>; rel="next"`, r.Host))
			}
			write(rows)
		case r.URL.Path == "/repos/a/b/issues/7" && r.Method == "GET":
			write(f.issue)
		case r.URL.Path == "/repos/a/b/issues/7" && r.Method == "PATCH":
			var in githubapi.IssueInput
			_ = json.NewDecoder(r.Body).Decode(&in)
			f.issueWrites++
			f.issue.Title = in.Title
			f.issue.Body = in.Body
			f.issue.State = in.State
			f.issue.UpdatedAt = "2026-02-01T00:00:00Z"
			write(f.issue)
		case r.URL.Path == "/repos/a/b/issues/7/comments" && r.Method == "POST":
			var in struct {
				Body string `json:"body"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			f.commentWrites++
			v := githubapi.Comment{ID: 9001, NodeID: "C9001", Body: in.Body, CreatedAt: "2026-01-03T00:00:00Z", UpdatedAt: "2026-01-03T00:00:00Z", User: githubapi.User{Login: "destination-user"}}
			f.comments = append(f.comments, v)
			if f.loseComment {
				f.loseComment = false
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(201)
			write(v)
		case r.URL.Path == "/repos/a/b/issues/7/comments" && r.Method == "GET":
			if f.hideComments {
				write([]githubapi.Comment{})
			} else {
				write(f.comments)
			}
		case r.URL.Path == "/repos/a/b/pulls":
			write([]any{map[string]any{"id": 88, "node_id": "PR88", "number": 8, "body": "PR source", "head": map[string]any{"sha": strings.Repeat("a", 40)}}})
		case r.URL.Path == "/repos/a/b/issues/8/comments":
			write([]githubapi.Comment{{ID: 100, NodeID: "PC100", Body: "PR conversation"}})
		case r.URL.Path == "/repos/a/b/pulls/8/reviews":
			write([]any{map[string]any{"id": 101, "node_id": "RV101", "body": "review source", "state": "APPROVED", "commit_id": strings.Repeat("a", 40)}})
		case r.URL.Path == "/repos/a/b/pulls/8/comments":
			write([]any{map[string]any{"id": 102, "node_id": "RC102", "body": "inline source", "pull_request_review_id": 101, "path": "x.go", "line": 4}, map[string]any{"id": 103, "node_id": "RC103", "body": "reply source", "pull_request_review_id": 101, "in_reply_to_id": 102}})
		case r.URL.Path == "/attachment":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("verified attachment bytes"))
		case r.URL.Path == "/private":
			w.WriteHeader(403)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *githubMigrationFixture) config() migration.ProviderConfig {
	return migration.ProviderConfig{Locator: migration.Locator{Kind: "github", Endpoint: f.server.URL, Repository: "a/b"}, Token: "supplied-secret"}
}
func githubMigrationSourceFixture() (migration.Snapshot, githubMigrationBlobs) {
	key := migration.Key{Provider: "gitlab", Instance: "https://gitlab.invalid", ProjectID: "3", Kind: "issue", ID: "17"}
	comment := key
	comment.Kind = "comment"
	comment.ID = "18"
	s := migration.Snapshot{Schema: "hn.project-snapshot/1", Source: migration.Locator{Kind: "gitlab", Endpoint: key.Instance, Repository: "a/b"}, SourceID: "3", Records: []migration.Record{{Key: key, Selector: "2", Revision: "source-r1", Author: "original author", Schema: "hn.migration.issue/1", Content: githubMigrationJSON(migration.IssueContent{Schema: "hn.migration.issue/1", Title: "Moved issue", Body: "Original issue body", State: "open", Labels: []string{}, Assignees: []string{}}), Links: []migration.Link{}, Blobs: []string{}}, {Key: comment, Revision: "source-c1", Author: "original commenter", Schema: "hn.migration.comment/1", Content: githubMigrationJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: "Original comment"}), Links: []migration.Link{{Relation: "issue", Target: key, Revision: "source-r1"}}, Blobs: []string{}}}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}
	return s, githubMigrationBlobs{}
}
func TestGitHubMigrationRegistryCoverageAndConstructorsAreInert(t *testing.T) {
	c := migration.ProviderConfig{Locator: migration.Locator{Kind: "github", Endpoint: "https://unused.invalid", Repository: "a/b"}, Token: "secret"}
	s, e := OpenMigrationSource(c)
	if e != nil {
		t.Fatal(e)
	}
	d, e := OpenMigrationDestination(c)
	if e != nil {
		t.Fatal(e)
	}
	sc, _ := s.Capabilities(context.Background())
	dc, _ := d.Capabilities(context.Background())
	p, ok := migrationProviders.Lookup("github")
	if !ok {
		t.Fatal("missing registration")
	}
	if e = migration.ValidateProviderCoverage(p.RequiredCoverage, sc, dc); e != nil {
		t.Fatal(e)
	}
	if d.Locator() != c.Locator {
		t.Fatal("locator mismatch")
	}
	c.Locator.Repository = "changed"
	if d.Locator().Repository != "a/b" {
		t.Fatal("mutable locator escaped")
	}
}
func TestGitHubMigrationEngineLostResponsesAndPersistedParent(t *testing.T) {
	for _, lost := range []string{"issue", "comment"} {
		t.Run(lost, func(t *testing.T) {
			f := newGithubMigrationFixture(t)
			if lost == "issue" {
				f.loseIssue = true
			} else {
				f.loseComment = true
			}
			ctx := context.Background()
			source, blobs := githubMigrationSourceFixture()
			d, e := OpenMigrationDestination(f.config())
			if e != nil {
				t.Fatal(e)
			}
			session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), source, blobs, migration.Selection{Destination: d.Locator()}, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(ctx, session, d, ""); e == nil {
				t.Fatal("lost response accepted")
			}
			if lost == "issue" {
				f.hideIssue = true
			} else {
				f.hideComments = true
			}
			restarted, e := OpenMigrationSession(ctx, session.Directory)
			if e != nil {
				t.Fatal(e)
			}
			fresh, _ := OpenMigrationDestination(f.config())
			if e = ApplyMigration(ctx, restarted, fresh, ""); e == nil || !strings.Contains(e.Error(), "manual recovery") {
				t.Fatal("negative enumeration allowed retry", e)
			}
			if f.issueWrites != 1 || f.commentWrites > 1 {
				t.Fatal("uncertain write repeated")
			}
			f.hideIssue = false
			f.hideComments = false
			if e = ApplyMigration(ctx, restarted, fresh, ""); e != nil {
				t.Fatal(e)
			}
			if f.issueWrites != 1 || f.commentWrites != 1 {
				t.Fatal("wrong remote write counts", f.issueWrites, f.commentWrites)
			}
			state, e := migrationUnseal(session.Directory, "progress.json", 1<<20)
			if e != nil {
				t.Fatal(e)
			}
			var progress migration.Progress
			if e = json.Unmarshal(state, &progress); e != nil {
				t.Fatal(e)
			}
			if len(progress.Entries) != 2 {
				t.Fatal(progress)
			}
			issueReceipt := progress.Entries[0].Receipts[0]
			if issueReceipt.RemoteID != "id:7001" || issueReceipt.RemoteKey != "7" {
				t.Fatal("immutable identity conflated with number", issueReceipt)
			}
			if !strings.Contains(string(progress.Entries[1].Write.Payload), `"parent":{"id":"7"}`) {
				t.Fatal("parent not engine-persisted", string(state))
			}
			if strings.Contains(string(state), "supplied-secret") {
				t.Fatal("credential persisted")
			}
			saved := string(progress.Entries[1].Write.Payload)
			if e = ApplyMigration(ctx, restarted, fresh, ""); e != nil {
				t.Fatal(e)
			}
			state, _ = migrationUnseal(session.Directory, "progress.json", 1<<20)
			_ = json.Unmarshal(state, &progress)
			if saved != string(progress.Entries[1].Write.Payload) {
				t.Fatal("confirmed resume changed payload")
			}
		})
	}
}
func TestGitHubMigrationReplacementAndStaleResourceRefuseBeforeEffects(t *testing.T) {
	for _, change := range []string{"repository", "issue"} {
		t.Run(change, func(t *testing.T) {
			f := newGithubMigrationFixture(t)
			s, blobs := githubMigrationSourceFixture()
			d, _ := OpenMigrationDestination(f.config())
			sel := migration.Selection{Destination: d.Locator(), Mappings: map[string]string{}}
			if change == "issue" {
				f.issue = githubapi.Issue{ID: 7001, NodeID: "I7001", Number: 7, Title: "existing", State: "open"}
				sel.Mappings["target:"+s.Records[0].Key.String()] = "7"
			}
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if change == "repository" {
				f.repoID++
			} else {
				f.issue.UpdatedAt = "2026-02-01T00:00:00Z"
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("stale identity accepted")
			}
			if f.issueWrites != 0 || f.commentWrites != 0 {
				t.Fatal("stale destination mutated")
			}
		})
	}
}
func TestGitHubMigrationSnapshotPRTopologyAttachmentsAndBudgets(t *testing.T) {
	f := newGithubMigrationFixture(t)
	f.issue = githubapi.Issue{ID: 7001, NodeID: "I7001", Number: 7, Title: "Issue source", Body: "source", State: "closed"}
	c := f.config()
	c.Attachments = []migration.AttachmentSelection{{URL: f.server.URL + "/attachment", Name: "asset.bin", Parent: migration.Key{Provider: "github", Instance: f.server.URL, ProjectID: "id:42", Kind: "issue", ID: "id:7001"}}, {URL: f.server.URL + "/private", Parent: migration.Key{Provider: "github", Instance: f.server.URL, ProjectID: "id:42", Kind: "issue", ID: "id:7001"}}}
	source, _ := OpenMigrationSource(c)
	sel := migration.Selection{Source: c.Locator, Kinds: []string{"issue", "comment", "change_request", "review", "inline_comment"}}
	s, blobs, e := source.Snapshot(context.Background(), sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	kinds := map[string]int{}
	verified := false
	for _, r := range s.Records {
		kinds[r.Key.Kind]++
		if r.Key.Kind == "inline_comment" && r.Key.ID == "id:103" {
			if len(r.Links) != 3 {
				t.Fatal("reply/review topology flattened", r.Links)
			}
		}
		for _, digest := range r.Blobs {
			rc, e := blobs.Open(context.Background(), digest)
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := io.ReadAll(rc)
			_ = rc.Close()
			if migration.Digest(raw) != digest {
				t.Fatal("blob digest mismatch")
			}
			if string(raw) == "verified attachment bytes" {
				verified = true
			}
		}
	}
	for _, kind := range []string{"issue", "change_request", "reply", "review", "inline_comment"} {
		if kinds[kind] == 0 {
			t.Fatal("missing snapshot kind", kind)
		}
	}
	if !verified {
		t.Fatal("selected attachment missing")
	}
	if len(s.Findings) != 1 || s.Findings[0].Status != "missing_dependency" {
		t.Fatal("private blob claimed preserved", s.Findings)
	}
	l := migration.DefaultLimits()
	l.Records = 1
	if _, _, e = source.Snapshot(context.Background(), sel, l); e == nil {
		t.Fatal("record budget ignored")
	}
	l = migration.DefaultLimits()
	l.BlobBytes = 2
	if _, _, e = source.Snapshot(context.Background(), sel, l); e == nil {
		t.Fatal("blob budget ignored")
	}
	f.pageCycle = true
	if _, _, e = source.Snapshot(context.Background(), sel, migration.DefaultLimits()); e == nil {
		t.Fatal("pagination cycle returned partial success")
	}
}
func TestGitHubMigrationLossesClosedStateMappingsAndDroppedFields(t *testing.T) {
	f := newGithubMigrationFixture(t)
	s, _ := githubMigrationSourceFixture()
	var c migration.IssueContent
	_ = json.Unmarshal(s.Records[0].Content, &c)
	c.State = "closed"
	c.Labels = []string{"upstream"}
	s.Records[0].Content = githubMigrationJSON(c)
	d, _ := OpenMigrationDestination(f.config())
	sel := migration.Selection{Destination: d.Locator()}
	ins, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
	if e != nil || len(ins) != 0 || len(fs) != 2 {
		t.Fatal(ins, fs, e)
	}
	if fs[0].Status != "requires_mapping" || fs[1].Status != "missing_dependency" {
		t.Fatal(fs)
	}
	sel.Mappings = map[string]string{"state:closed": "open", "label:upstream": "mapped"}
	s.Blobs = []migration.Blob{}
	ins, _, e = d.Plan(context.Background(), s, sel, migration.DefaultLimits())
	if e != nil || len(ins) != 2 {
		t.Fatal(ins, e)
	}
	f.dropLabel = true
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, githubMigrationBlobs{}, sel, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
		t.Fatal("201 with dropped label confirmed")
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
		t.Fatal("different remote projection reconciled")
	}
	if f.issueWrites != 1 || f.commentWrites != 0 {
		t.Fatal("uncertain projection repeated")
	}
}

func TestGitHubMigrationAmbiguousCreateAndReplacedParentNeverRetry(t *testing.T) {
	for _, mode := range []string{"ambiguous", "parent"} {
		t.Run(mode, func(t *testing.T) {
			f := newGithubMigrationFixture(t)
			f.loseIssue = mode == "ambiguous"
			f.loseComment = mode == "parent"
			s, blobs := githubMigrationSourceFixture()
			d, _ := OpenMigrationDestination(f.config())
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, migration.Selection{Destination: d.Locator()}, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("expected lost response")
			}
			if mode == "ambiguous" {
				f.duplicate = true
			} else {
				f.issue.ID++
			}
			fresh, _ := OpenMigrationDestination(f.config())
			if e = ApplyMigration(context.Background(), session, fresh, ""); e == nil {
				t.Fatal("unsafe recovery accepted")
			}
			if f.issueWrites != 1 || f.commentWrites > 1 {
				t.Fatal("uncertain create repeated")
			}
		})
	}
}
func TestGitHubMigrationSourceNamespaceSeparatesInstancesAndSupportsDefaultSelection(t *testing.T) {
	f1 := newGithubMigrationFixture(t)
	f2 := newGithubMigrationFixture(t)
	var ids []string
	var refs []string
	for _, f := range []*githubMigrationFixture{f1, f2} {
		c := f.config()
		s, _ := OpenMigrationSource(c)
		snap, _, e := s.Snapshot(context.Background(), migration.Selection{Source: c.Locator, Kinds: []string{"issue"}}, migration.DefaultLimits())
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, snap.SourceID)
		ref, e := migrationObservationRef(snap.SourceID, migration.NativeRef{SourceRef: "refs/heads/main", OID: strings.Repeat("a", 40), Kind: "code"})
		if e != nil {
			t.Fatal(e)
		}
		refs = append(refs, ref)
	}
	if ids[0] == ids[1] || refs[0] == refs[1] {
		t.Fatal("immutable numeric project IDs collided across instances", ids, refs)
	}
}
func TestGitHubMigrationPreservesOversizedIssueBytesWithFinding(t *testing.T) {
	f := newGithubMigrationFixture(t)
	f.issue = githubapi.Issue{ID: 7001, NodeID: "I7001", Number: 7, Title: "Large issue", Body: strings.Repeat("x", 65537), State: "open"}
	c := f.config()
	source, _ := OpenMigrationSource(c)
	s, b, e := source.Snapshot(context.Background(), migration.Selection{Source: c.Locator, Kinds: []string{"issue"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Records) != 1 || s.Records[0].Key.Kind != "attachment" || len(s.Findings) != 1 || s.Findings[0].Status != "unsupported" {
		t.Fatal("oversized issue silently truncated", s)
	}
	rc, e := b.Open(context.Background(), s.Records[0].Blobs[0])
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(rc)
	_ = rc.Close()
	var original githubapi.Issue
	if e = json.Unmarshal(raw, &original); e != nil || original.Body != f.issue.Body {
		t.Fatal("source bytes not retained", e)
	}
}
func TestGitHubMigrationWorkflowSourceIsPinnedAndInert(t *testing.T) {
	workflow := newWorkflowGHFixture(t, "name: inert\non: workflow_dispatch\n")
	defer workflow.server.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/a/b" {
			_ = json.NewEncoder(w).Encode(githubapi.Repository{ID: 42, NodeID: "R42", FullName: "a/b", CreatedAt: "2026-01-01T00:00:00Z", DefaultBranch: "main"})
			return
		}
		workflow.server.Config.Handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	c := migration.ProviderConfig{Locator: migration.Locator{Kind: "github", Endpoint: server.URL, Repository: "a/b"}, Token: "supplied-secret"}
	source, _ := OpenMigrationSource(c)
	s, blobs, e := source.Snapshot(context.Background(), migration.Selection{Source: c.Locator, Kinds: []string{"workflow"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Records) != 1 || s.Records[0].Key.Kind != "workflow" || len(s.Records[0].Blobs) != 2 || workflow.writes != 0 {
		t.Fatal("workflow source not preserved inertly", s)
	}
	found := false
	for _, d := range s.Records[0].Blobs {
		rc, e := blobs.Open(context.Background(), d)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(raw) == string(workflow.content) {
			found = true
		}
	}
	if !found {
		t.Fatal("workflow YAML bytes missing")
	}
}
func TestGitHubMigrationDiscussionAndPlanningSnapshots(t *testing.T) {
	for _, kind := range []string{"discussion", "project"} {
		t.Run(kind, func(t *testing.T) {
			discussion := newDiscussionFixture(t)
			discussion.d = githubapi.Discussion{ID: "D1", RepositoryID: "R1", Number: 7, Title: "Question", Body: "Foreign discussion", Comments: []githubapi.DiscussionComment{{ID: "A", Body: "Top comment"}, {ID: "B", ParentID: "A", Body: "Nested reply"}}}
			p := projectFixture()
			planning := &githubProjectFake{p: &p}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/repos/a/b" {
					_ = json.NewEncoder(w).Encode(githubapi.Repository{ID: 42, NodeID: "R1", FullName: "a/b", CreatedAt: "2026-01-01T00:00:00Z", HasDiscussions: true})
					return
				}
				if r.URL.Path != "/graphql" {
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				if kind == "discussion" {
					discussion.server.Config.Handler.ServeHTTP(w, r)
				} else {
					planning.ServeHTTP(w, r)
				}
			}))
			defer server.Close()
			c := migration.ProviderConfig{Locator: migration.Locator{Kind: "github", Endpoint: server.URL, Repository: "a/b"}, Token: "supplied-secret"}
			if kind == "project" {
				c.PlanningSelectors = []string{"user:a/1"}
			}
			source, _ := OpenMigrationSource(c)
			s, _, e := source.Snapshot(context.Background(), migration.Selection{Source: c.Locator, Kinds: []string{kind}}, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			count := map[string]int{}
			for _, r := range s.Records {
				count[r.Key.Kind]++
				if kind == "discussion" && r.Key.ID == "B" && len(r.Links) != 2 {
					t.Fatal("nested discussion topology lost", r)
				}
			}
			if kind == "discussion" && (count[kind] != 1 || count["reply"] != 2) {
				t.Fatal(count)
			}
			if kind == "project" && (count[kind] != 1 || count["project_item"] != 1) {
				t.Fatal(count)
			}
			if planning.mutations != 0 || discussion.writes != 0 {
				t.Fatal("snapshot mutated provider")
			}
		})
	}
}

func TestGitHubMigrationUsesSharedSelectedGitTransportAndSecretFreeArchive(t *testing.T) {
	root := inReplicationTestRepository(t)
	source := newGithubMigrationFixture(t)
	source.issue = githubapi.Issue{ID: 7001, NodeID: "I7001", Number: 7, Title: "Source issue", Body: "Source body", State: "open"}
	destination := newGithubMigrationFixture(t)
	sourceConfig := source.config()
	apiSource, e := OpenMigrationSource(sourceConfig)
	if e != nil {
		t.Fatal(e)
	}
	apiDest, e := OpenMigrationDestination(destination.config())
	if e != nil {
		t.Fatal(e)
	}
	capture := ComposeMigrationSources(apiSource, LocalMigrationSource{GitDir: filepath.Join(root, ".git")})
	remote := migrationBare(t)
	sel := migration.Selection{Source: sourceConfig.Locator, Destination: apiDest.Locator(), Refs: []string{"refs/heads/main"}, Kinds: []string{"issue", "comment"}}
	ctx := context.Background()
	s, blobs, e := capture.Snapshot(ctx, sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer blobs.(io.Closer).Close()
	if len(s.NativeRefs) != 1 || len(s.Records) != 1 {
		t.Fatal("whole-project composition lost code or communications", s)
	}
	var archive bytes.Buffer
	if e = migration.WriteArchive(ctx, &archive, s, blobs, migration.DefaultLimits()); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(archive.Bytes(), []byte(sourceConfig.Token)) {
		t.Fatal("credential present in portable archive")
	}
	target := ComposeMigrationDestinations(apiDest, GitMigrationDestination{Remote: remote, Destination: apiDest.Locator(), Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}})
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "s"), s, blobs, sel, target, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	planJSON, _ := json.Marshal(session.Plan)
	if bytes.Contains(planJSON, []byte(sourceConfig.Token)) {
		t.Fatal("credential present in saved plan")
	}
	if e = ApplyMigration(ctx, session, target, ""); e != nil {
		t.Fatal(e)
	}
	head := mustGitText(t, "rev-parse", "HEAD")
	got, e := projectGit(ctx, remote, nil, 256, "show-ref", "--verify", "--hash", "refs/heads/main")
	if e != nil || strings.TrimSpace(string(got)) != head {
		t.Fatal("selected code not transported", e)
	}
	if source.issueWrites != 0 || destination.issueWrites != 1 {
		t.Fatal("source mutated or destination issue missing")
	}
}

func TestGitHubMigrationPlanReportsRecoveryMarkerCapacityBeforeWrites(t *testing.T) {
	for _, kind := range []string{"issue", "comment"} {
		t.Run(kind, func(t *testing.T) {
			f := newGithubMigrationFixture(t)
			s, _ := githubMigrationSourceFixture()
			d, _ := OpenMigrationDestination(f.config())
			sel := migration.Selection{Destination: d.Locator()}
			ins, _, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			marker := "\n\n<!-- hn-migration:" + migration.Digest(nil) + ":" + ins[0].ID + ":" + migration.Digest(nil) + " -->"
			budget := 65536 - len(marker)
			if kind == "issue" {
				var c migration.IssueContent
				_ = json.Unmarshal(s.Records[0].Content, &c)
				c.Body = strings.Repeat("x", budget+1)
				s.Records[0].Content = githubMigrationJSON(c)
			} else {
				var c migration.CommentContent
				_ = json.Unmarshal(s.Records[1].Content, &c)
				c.Body = strings.Repeat("x", budget+1)
				s.Records[1].Content = githubMigrationJSON(c)
			}
			ins, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			unsupported := false
			for _, finding := range fs {
				if finding.Status == "unsupported" && strings.Contains(finding.Message, "marker") {
					unsupported = true
				}
			}
			if !unsupported || kind == "issue" && len(ins) != 0 || kind == "comment" && len(ins) != 1 {
				t.Fatal("known marker capacity was not reported in plan", len(ins), fs)
			}
			if f.issueWrites != 0 || f.commentWrites != 0 {
				t.Fatal("planning mutated destination")
			}
		})
	}
}
func TestGitHubMigrationExplicitWorkItemToIssueMappingRetainsCommentDependency(t *testing.T) {
	f := newGithubMigrationFixture(t)
	s, blobs := githubMigrationSourceFixture()
	s.Source.Kind = "azure"
	s.Source.Endpoint = "https://azure.invalid"
	s.Records[0].Key.Provider = "azure"
	s.Records[0].Key.Instance = s.Source.Endpoint
	s.Records[0].Key.Kind = "work_item"
	s.Records[1].Key.Provider = "azure"
	s.Records[1].Key.Instance = s.Source.Endpoint
	s.Records[1].Links[0].Relation = "work_item"
	s.Records[1].Links[0].Target = s.Records[0].Key
	var c migration.IssueContent
	_ = json.Unmarshal(s.Records[0].Content, &c)
	c.Type = "Task"
	c.State = "Active"
	s.Records[0].Content = githubMigrationJSON(c)
	d, _ := OpenMigrationDestination(f.config())
	sel := migration.Selection{Destination: d.Locator()}
	ins, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
	if e != nil || len(ins) != 0 || fs[0].Status != "requires_mapping" {
		t.Fatal("work-item mapping was implicit or not explained", ins, fs, e)
	}
	sel.Mappings = map[string]string{"state:Active": "open", "type:Task": ""}
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(session.Plan.Intents) != 2 || session.Plan.Intents[0].Kind != "issue" || session.Plan.Intents[0].Sources[0].Kind != "work_item" {
		t.Fatal("source/destination models conflated", session.Plan.Intents)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
		t.Fatal(e)
	}
	if f.issueWrites != 1 || f.commentWrites != 1 {
		t.Fatal("work item/comment mapping missing")
	}
}
func TestGitHubMigrationSnapshotRetainsIssueTypeName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/a/b":
			fmt.Fprint(w, `{"id":42,"node_id":"R42","full_name":"a/b","created_at":"2026-01-01T00:00:00Z"}`)
		case "/repos/a/b/issues":
			fmt.Fprint(w, `[{"id":7001,"node_id":"I7001","number":7,"title":"Typed source","body":"body","state":"open","type":{"id":1,"name":"Bug"}}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	config := migration.ProviderConfig{Locator: migration.Locator{Kind: "github", Endpoint: server.URL, Repository: "a/b"}}
	source, _ := OpenMigrationSource(config)
	s, _, e := source.Snapshot(context.Background(), migration.Selection{Source: config.Locator, Kinds: []string{"issue"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	var c migration.IssueContent
	if len(s.Records) != 1 {
		t.Fatal(s)
	}
	if e = json.Unmarshal(s.Records[0].Content, &c); e != nil || c.Type != "Bug" {
		t.Fatal("retrieved issue type silently emptied", c, e)
	}
}

func TestGitHubMigrationExactMarkedBodyCapacitySucceeds(t *testing.T) {
	f := newGithubMigrationFixture(t)
	s, blobs := githubMigrationSourceFixture()
	d, _ := OpenMigrationDestination(f.config())
	sel := migration.Selection{Destination: d.Locator()}
	ins, _, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	wireMarker := "\n\n<!-- hn-migration:" + migration.Digest(nil) + ":" + ins[0].ID + ":" + migration.Digest(nil) + " -->"
	var c migration.IssueContent
	_ = json.Unmarshal(s.Records[0].Content, &c)
	c.Body = strings.Repeat("x", 65536-len(wireMarker))
	s.Records[0].Content = githubMigrationJSON(c)
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
		t.Fatal(e)
	}
	if len(f.issue.Body) != 65536 || f.issueWrites != 1 || f.commentWrites != 1 {
		t.Fatal("exact marked body capacity refused or changed", len(f.issue.Body), f.issueWrites, f.commentWrites)
	}
}
func TestGitHubMigrationUnsupportedDestinationTypesNeverClaimUntypedSuccess(t *testing.T) {
	for _, mode := range []string{"existing", "default"} {
		t.Run(mode, func(t *testing.T) {
			f := newGithubMigrationFixture(t)
			s, blobs := githubMigrationSourceFixture()
			d, _ := OpenMigrationDestination(f.config())
			sel := migration.Selection{Destination: d.Locator()}
			if mode == "existing" {
				f.issue = githubapi.Issue{ID: 7001, NodeID: "I7001", Number: 7, State: "open", Title: "Typed destination", Type: &githubapi.IssueType{Name: "Bug"}}
				sel.Mappings = map[string]string{"target:" + s.Records[0].Key.String(): "7"}
				ins, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
				if e != nil || len(ins) != 0 || fs[0].Status != "unsupported" {
					t.Fatal("typed update was silently planned as untyped", ins, fs, e)
				}
				if f.issueWrites != 0 {
					t.Fatal("planning changed a type")
				}
				return
			}
			f.defaultType = "Bug"
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("default typed issue confirmed as untyped")
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("default typed issue reconciled as untyped")
			}
			if f.issueWrites != 1 || f.commentWrites != 0 {
				t.Fatal("uncertain type projection retried")
			}
		})
	}
}
