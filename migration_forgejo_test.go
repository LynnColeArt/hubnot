//go:build !noforge

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"hubnot/internal/forgejoapi"
	"hubnot/internal/migration"
)

type v1MigrationFixture struct {
	mu                                                                           sync.Mutex
	server                                                                       *httptest.Server
	kind, version, repoName                                                      string
	repoID                                                                       int64
	issue                                                                        forgejoapi.Issue
	comments                                                                     []forgejoapi.Comment
	issueWrites, commentWrites, otherWrites, requests                            int
	loseIssue, loseComment, hide, duplicate, private, replacedParent, dropLabels bool
	topology, workflows                                                          bool
	inaccessibleIssues                                                           bool
}

func newV1Fixture(t *testing.T, kind, version string) *v1MigrationFixture {
	f := &v1MigrationFixture{kind: kind, version: version, repoID: 42, repoName: "a/b", comments: []forgejoapi.Comment{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests++
		if r.Header.Get("Authorization") != "token explicit-secret" {
			t.Error("explicit credential missing")
		}
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		page := func(v any, n int) { w.Header().Set("X-Total-Count", fmt.Sprint(n)); write(v) }
		root := "/api/v1/repos/" + f.repoName
		if r.Method != "GET" && r.Method != "POST" && r.Method != "PATCH" {
			f.otherWrites++
		}
		switch {
		case r.URL.Path == "/api/v1/version":
			write(map[string]string{"version": f.version})
		case r.URL.Path == root:
			if f.private {
				w.WriteHeader(404)
				return
			}
			write(forgejoapi.Repository{ID: f.repoID, FullName: f.repoName, CreatedAt: "2026-01-01T00:00:00Z", DefaultBranch: "main", HasIssues: true})
		case r.URL.Path == root+"/labels":
			page([]forgejoapi.Label{{ID: 41, Name: "bug"}}, 1)
		case r.URL.Path == root+"/issues" && r.Method == "POST":
			var in struct {
				Title     string   `json:"title"`
				Body      string   `json:"body"`
				Closed    bool     `json:"closed"`
				Labels    []int64  `json:"labels"`
				Assignees []string `json:"assignees"`
			}
			if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
				t.Error(e)
			}
			f.issueWrites++
			state := "open"
			if in.Closed {
				state = "closed"
			}
			f.issue = forgejoapi.Issue{ID: 7001, Number: 7, Title: in.Title, Body: in.Body, State: state, ContentVersion: 1, Labels: []forgejoapi.Label{}, Assignees: []forgejoapi.User{}, CreatedAt: "2026-01-02T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}
			for _, id := range in.Labels {
				if !f.dropLabels {
					f.issue.Labels = append(f.issue.Labels, forgejoapi.Label{ID: id, Name: "bug"})
				}
			}
			for _, x := range in.Assignees {
				f.issue.Assignees = append(f.issue.Assignees, forgejoapi.User{Login: x})
			}
			if f.loseIssue {
				f.loseIssue = false
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(201)
			write(f.issue)
		case r.URL.Path == root+"/issues" && r.Method == "GET":
			if f.inaccessibleIssues {
				w.WriteHeader(403)
				return
			}
			rows := []forgejoapi.Issue{}
			if f.issue.ID > 0 && !f.hide {
				rows = append(rows, f.issue)
			}
			if f.duplicate && len(rows) > 0 {
				v := f.issue
				v.ID++
				v.Number++
				rows = append(rows, v)
			}
			if f.topology {
				rows = append(rows, forgejoapi.Issue{ID: 8001, Number: 8, State: "open", Title: "pull", PullRequest: json.RawMessage(`{"merged":false}`)})
			}
			page(rows, len(rows))
		case r.URL.Path == root+"/issues/7" && r.Method == "GET":
			if f.replacedParent {
				v := f.issue
				v.ID++
				write(v)
			} else {
				write(f.issue)
			}
		case r.URL.Path == root+"/issues/7" && r.Method == "PATCH":
			var in struct {
				Title, Body, State string
				Assignees          []string
				ContentVersion     int64 `json:"content_version"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if strings.HasPrefix(f.version, "1.27.") && in.ContentVersion != f.issue.ContentVersion {
				t.Error("optimistic content baseline not sent")
			}
			f.issueWrites++
			f.issue.Title = in.Title
			f.issue.Body = in.Body
			f.issue.State = in.State
			f.issue.Assignees = []forgejoapi.User{}
			for _, x := range in.Assignees {
				f.issue.Assignees = append(f.issue.Assignees, forgejoapi.User{Login: x})
			}
			f.issue.UpdatedAt = "2026-02-01T00:00:00Z"
			f.issue.ContentVersion++
			write(f.issue)
		case r.URL.Path == root+"/issues/7/comments" && r.Method == "GET":
			if f.hide {
				page([]forgejoapi.Comment{}, 0)
			} else {
				page(f.comments, len(f.comments))
			}
		case r.URL.Path == root+"/issues/7/comments" && r.Method == "POST":
			var in struct{ Body string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			f.commentWrites++
			v := forgejoapi.Comment{ID: 9001 + int64(len(f.comments)), Body: in.Body, CreatedAt: "2026-01-03T00:00:00Z", UpdatedAt: "2026-01-03T00:00:00Z"}
			f.comments = append(f.comments, v)
			if f.loseComment {
				f.loseComment = false
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(201)
			write(v)
		case r.URL.Path == root+"/issues/comments/9001" && r.Method == "PATCH":
			var in struct{ Body string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			f.commentWrites++
			f.comments[0].Body = in.Body
			f.comments[0].UpdatedAt = "2026-02-01T00:00:00Z"
			write(f.comments[0])
		case r.URL.Path == root+"/pulls/8" && f.topology:
			write(map[string]any{"id": 8101, "number": 8, "head": map[string]string{"sha": strings.Repeat("b", 40)}, "base": map[string]string{"sha": strings.Repeat("a", 40)}, "body": "PR source", "user": map[string]string{"login": "reviewer"}})
		case r.URL.Path == root+"/pulls/8/reviews" && f.topology:
			page([]any{map[string]any{"id": 8201, "state": "APPROVED", "dismissed": true, "body": "foreign review", "commit_id": strings.Repeat("b", 40)}}, 1)
		case r.URL.Path == root+"/pulls/8/reviews/8201/comments" && f.topology:
			page([]any{map[string]any{"id": 8301, "pull_request_review_id": 8201, "path": "main.go", "position": 4, "original_position": 3, "diff_hunk": "@@ source @@", "resolver": map[string]string{"login": "foreign-admin"}}}, 1)
		case r.URL.Path == root+"/issues/8/comments" && f.topology:
			page([]forgejoapi.Comment{{ID: 8401, Body: "PR conversation"}}, 1)
		case strings.HasPrefix(r.URL.Path, root+"/git/commits/") && f.workflows:
			write(map[string]string{"sha": strings.Repeat("a", 40)})
		case strings.HasPrefix(r.URL.Path, root+"/contents/") && f.workflows:
			if r.URL.Query().Get("ref") != strings.Repeat("a", 40) {
				t.Error("workflow read was not pinned")
			}
			p := strings.TrimPrefix(r.URL.Path, root+"/contents/")
			if p == ".forgejo/workflows" || p == ".gitea/workflows" {
				if p == ".forgejo/workflows" {
					if f.kind == "gitea" {
						t.Error("Gitea used Forgejo directory")
					}
					w.WriteHeader(404)
					return
				}
				write([]map[string]any{{"type": "file", "path": ".gitea/workflows/test.yml"}})
			} else {
				data := []byte("jobs:\n  hostile:\n    run: touch SHOULD_NOT_EXIST\n")
				write(map[string]any{"type": "file", "path": p, "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data), "size": len(data)})
			}
		case r.URL.Path == "/asset":
			_, _ = w.Write([]byte("exact asset bytes"))
		case r.URL.Path == "/private-asset":
			w.WriteHeader(403)
		default:
			if r.Method != "GET" {
				f.otherWrites++
			}
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *v1MigrationFixture) config() migration.ProviderConfig {
	version := "16"
	if f.kind == "gitea" {
		version = "1.27"
		if strings.HasPrefix(f.version, "1.24.") {
			version = "1.24"
		}
	} else if strings.HasPrefix(f.version, "11.") {
		version = "11"
	}
	return migration.ProviderConfig{Locator: migration.Locator{Kind: f.kind, Endpoint: f.server.URL + "/api/v1", Repository: f.repoName}, Token: "explicit-secret", APIVersion: version}
}
func (f *v1MigrationFixture) seed() {
	f.issue = forgejoapi.Issue{ID: 7001, Number: 7, Title: "source issue", Body: "source body", State: "closed", Labels: []forgejoapi.Label{}, Assignees: []forgejoapi.User{}, ContentVersion: 3, CreatedAt: "2026-01-02T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}
	f.comments = []forgejoapi.Comment{{ID: 9001, Body: "source comment", CreatedAt: "2026-01-03T00:00:00Z", UpdatedAt: "2026-01-03T00:00:00Z"}}
}
func v1Source(t *testing.T, kind, version string) (migration.Snapshot, migration.BlobSource) {
	t.Helper()
	f := newV1Fixture(t, kind, version)
	f.seed()
	s, e := OpenMigrationSource(f.config())
	if e != nil {
		t.Fatal(e)
	}
	snapshot, blobs, e := s.Snapshot(context.Background(), migration.Selection{Source: f.config().Locator, Kinds: []string{"issue", "comment"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	return snapshot, blobs
}
func v1Decisions(s migration.Snapshot, fs []migration.Finding) []migration.Decision {
	out := []migration.Decision{}
	for _, f := range append(append([]migration.Finding{}, s.Findings...), fs...) {
		if f.Status != "mapped" && f.Status != "preserved" {
			out = append(out, migration.Decision{FindingID: f.ID, Action: "preserve", Mapping: json.RawMessage(`{}`)})
		}
	}
	return out
}
func v1Prepare(t *testing.T, s migration.Snapshot, b migration.BlobSource, d migration.Destination, sel migration.Selection) *MigrationSession {
	t.Helper()
	_, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "session"), s, b, sel, d, migration.DefaultLimits(), v1Decisions(s, fs))
	if e != nil {
		t.Fatal(e)
	}
	return session
}

func runV1EngineRecovery(t *testing.T, kind, version string) {
	for _, lost := range []string{"issue", "comment"} {
		t.Run(lost, func(t *testing.T) {
			ctx := context.Background()
			source, blobs := v1Source(t, kind, version)
			f := newV1Fixture(t, kind, version)
			f.loseIssue = lost == "issue"
			f.loseComment = lost == "comment"
			d, e := OpenMigrationDestination(f.config())
			if e != nil {
				t.Fatal(e)
			}
			session := v1Prepare(t, source, blobs, d, migration.Selection{Destination: d.Locator()})
			if e = ApplyMigration(ctx, session, d, ""); e == nil {
				t.Fatal("uncertain write accepted")
			}
			f.hide = true
			restart, e := OpenMigrationSession(ctx, session.Directory)
			if e != nil {
				t.Fatal(e)
			}
			fresh, e := OpenMigrationDestination(f.config())
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(ctx, restart, fresh, ""); e == nil || !strings.Contains(e.Error(), "manual recovery") {
				t.Fatal("negative observation repeated create", e)
			}
			if f.issueWrites != 1 || f.commentWrites > 1 {
				t.Fatal("duplicate effects")
			}
			f.hide = false
			if lost == "issue" {
				f.inaccessibleIssues = true
				if e = ApplyMigration(ctx, restart, fresh, ""); e == nil {
					t.Fatal("inaccessible observation allowed retry")
				}
				if f.issueWrites != 1 {
					t.Fatal("inaccessible create repeated")
				}
				f.inaccessibleIssues = false
			}
			if lost == "issue" {
				f.duplicate = true
				if e = ApplyMigration(ctx, restart, fresh, ""); e == nil {
					t.Fatal("ambiguous observation confirmed")
				}
				if f.issueWrites != 1 {
					t.Fatal("duplicate issue")
				}
				f.duplicate = false
			}
			if e = ApplyMigration(ctx, restart, fresh, ""); e != nil {
				t.Fatal(e)
			}
			if f.issueWrites != 1 || f.commentWrites != 1 || f.otherWrites != 0 {
				t.Fatal("unexpected effects", f.issueWrites, f.commentWrites, f.otherWrites)
			}
			raw, e := migrationUnseal(session.Directory, "progress.json", 1<<20)
			if e != nil {
				t.Fatal(e)
			}
			var p migration.Progress
			if e = json.Unmarshal(raw, &p); e != nil {
				t.Fatal(e)
			}
			if len(p.Entries) != 2 || p.Entries[0].Receipts[0].RemoteID != "id:7001" || p.Entries[0].Receipts[0].RemoteKey != "7" {
				t.Fatal("global identity conflated with index", p)
			}
			if !bytes.Contains(p.Entries[1].Write.Payload, []byte(`"parent":{"id":"7"}`)) {
				t.Fatal("parent was not persisted by engine", string(raw))
			}
			if p.Entries[1].Write.PayloadDigest != migration.Digest(p.Entries[1].Write.Payload) {
				t.Fatal("actual payload digest mismatch")
			}
			if bytes.Contains(raw, []byte("explicit-secret")) {
				t.Fatal("token persisted")
			}
			if e = ApplyMigration(ctx, restart, fresh, ""); e != nil || f.issueWrites != 1 || f.commentWrites != 1 {
				t.Fatal("confirmed writes repeated", e)
			}
		})
	}
}
func TestForgejoMigrationEngineRecovery(t *testing.T) {
	for _, version := range []string{"11.0.11+gitea-1.22.0", "16.0.0-dev-785+gitea-1.22.0"} {
		t.Run(version, func(t *testing.T) { runV1EngineRecovery(t, "forgejo", version) })
	}
}

func runV1Snapshot(t *testing.T, kind, version string) {
	f := newV1Fixture(t, kind, version)
	f.seed()
	f.topology = true
	f.workflows = true
	c := f.config()
	parent := migration.Key{Provider: kind, Instance: c.Locator.Endpoint, ProjectID: "id:42", Kind: "issue", ID: "id:7001"}
	c.Attachments = []migration.AttachmentSelection{{URL: f.server.URL + "/asset", Name: "asset.dat", Parent: parent}, {URL: f.server.URL + "/private-asset", Name: "private.dat", Parent: parent}, {URL: "https://foreign.example/asset", Name: "foreign.dat", Parent: parent}}
	c.PlanningSelectors = []string{"board"}
	s, e := OpenMigrationSource(c)
	if e != nil {
		t.Fatal(e)
	}
	snapshot, blobs, e := s.Snapshot(context.Background(), migration.Selection{Source: c.Locator}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	rawSeen, assetSeen, workflowSeen := false, false, false
	for _, r := range snapshot.Records {
		counts[r.Key.Kind]++
		if r.Key.Provider != kind || r.Key.ProjectID != "id:42" {
			t.Fatal("bad source namespace", r.Key)
		}
		if r.Key.Kind == "change_request" && r.Key.ID != "id:8101" {
			t.Fatal("PR identity conflated with issue index")
		}
		if r.Key.Kind == "review" && len(r.Links) != 1 {
			t.Fatal("review link missing")
		}
		if r.Key.Kind == "inline_comment" && len(r.Links) != 2 {
			t.Fatal("inline topology missing")
		}
		if r.Key.Kind == "reply" && (len(r.Links) != 1 || r.Links[0].Target.Kind != "change_request") {
			t.Fatal("PR conversation flattened into issue comment")
		}
	}
	for _, b := range snapshot.Blobs {
		r, e := blobs.Open(context.Background(), b.Digest)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(r)
		r.Close()
		if migration.Digest(raw) != b.Digest || int64(len(raw)) != b.Size {
			t.Fatal("blob digest mismatch")
		}
		rawSeen = rawSeen || bytes.Contains(raw, []byte(`"diff_hunk":"@@ source @@"`))
		assetSeen = assetSeen || string(raw) == "exact asset bytes"
		workflowSeen = workflowSeen || strings.Contains(string(raw), "touch SHOULD_NOT_EXIST")
	}
	for _, k := range []string{"issue", "comment", "change_request", "review", "inline_comment", "reply", "workflow"} {
		if counts[k] != 1 {
			t.Fatal("missing actual resource", k, counts)
		}
	}
	if !rawSeen || !assetSeen || !workflowSeen {
		t.Fatal("retrieved bytes missing", rawSeen, assetSeen, workflowSeen)
	}
	missing, board := 0, false
	for _, finding := range snapshot.Findings {
		if finding.Status == "missing_dependency" {
			missing++
		}
		if finding.Path == "project" && finding.Status == "unsupported" {
			board = true
		}
	}
	if missing != 2 || !board || f.otherWrites != 0 || f.issueWrites != 0 || f.commentWrites != 0 {
		t.Fatal("false availability or effect", snapshot.Findings)
	}
	// A mutable owner/name is a selector, not source identity.
	oldID := snapshot.SourceID
	f.repoName = "moved/renamed"
	renamed := f.config()
	renamed.WorkflowRef = "main"
	ss, e := OpenMigrationSource(renamed)
	if e != nil {
		t.Fatal(e)
	}
	other, _, e := ss.Snapshot(context.Background(), migration.Selection{Source: renamed.Locator, Kinds: []string{"issue"}}, migration.DefaultLimits())
	if e != nil || other.SourceID != oldID {
		t.Fatal("source identity depends on mutable repository name", other.SourceID, oldID, e)
	}
	// The exported archive and source records remain readable without network.
	var archive bytes.Buffer
	if e = migration.WriteArchive(context.Background(), &archive, snapshot, blobs, migration.DefaultLimits()); e != nil {
		t.Fatal(e)
	}
	read, e := migration.ReadArchive(context.Background(), bytes.NewReader(archive.Bytes()), migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer read.Close()
	if _, e = read.Open(context.Background(), snapshot.Blobs[0].Digest); e != nil {
		t.Fatal(e)
	}
}
func TestForgejoMigrationSnapshotTopologyAssetsWorkflows(t *testing.T) {
	runV1Snapshot(t, "forgejo", "16.0.0-dev+gitea-1.22.0")
}

func runV1Guards(t *testing.T, kind, version string) {
	for _, change := range []string{"repository", "issue-revision", "comment-parent", "comment-revision", "version"} {
		t.Run(change, func(t *testing.T) {
			s, b := v1Source(t, kind, version)
			f := newV1Fixture(t, kind, version)
			d, e := OpenMigrationDestination(f.config())
			if e != nil {
				t.Fatal(e)
			}
			sel := migration.Selection{Destination: d.Locator(), Mappings: map[string]string{}}
			if change == "issue-revision" || change == "comment-revision" {
				f.seed()
				for _, r := range s.Records {
					sel.Mappings["target:"+r.Key.String()] = r.Selector
				}
			}
			if change == "comment-parent" {
				f.loseComment = true
			}
			session := v1Prepare(t, s, b, d, sel)
			if change == "comment-parent" {
				if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
					t.Fatal("lost comment accepted")
				}
				f.replacedParent = true
			} else if change == "repository" {
				f.repoID++
			} else if change == "version" {
				f.version = "99.0.0"
			} else if change == "issue-revision" {
				f.issue.UpdatedAt = "2026-03-01T00:00:00Z"
			} else {
				f.comments[0].UpdatedAt = "2026-03-01T00:00:00Z"
			}
			before := f.issueWrites + f.commentWrites
			fresh, _ := OpenMigrationDestination(f.config())
			if change == "version" {
				fresh = d
			}
			if e = ApplyMigration(context.Background(), session, fresh, ""); e == nil {
				t.Fatal("changed baseline accepted")
			}
			if change == "comment-revision" {
				if f.issueWrites != 1 || f.commentWrites != 0 {
					t.Fatal("stale comment mutated")
				}
			} else if f.issueWrites+f.commentWrites != before {
				t.Fatal("effect after changed baseline")
			}
		})
	}
}
func TestForgejoMigrationIdentityAndResourceGuards(t *testing.T) { runV1Guards(t, "forgejo", "16.0.0") }

func runV1UpdatesAndMappings(t *testing.T, kind, version string) {
	s, b := v1Source(t, kind, version)
	f := newV1Fixture(t, kind, version)
	f.seed()
	d, e := OpenMigrationDestination(f.config())
	if e != nil {
		t.Fatal(e)
	}
	sel := migration.Selection{Destination: d.Locator(), Mappings: map[string]string{}}
	for i := range s.Records {
		r := &s.Records[i]
		sel.Mappings["target:"+r.Key.String()] = r.Selector
		if r.Key.Kind == "issue" {
			var c migration.IssueContent
			_ = json.Unmarshal(r.Content, &c)
			c.Title = "updated title"
			c.State = "open"
			r.Content = v1JSON(c)
		} else {
			r.Content = v1JSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: "updated comment"})
		}
	}
	session := v1Prepare(t, s, b, d, sel)
	if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
		t.Fatal(e)
	}
	if f.issueWrites != 1 || f.commentWrites != 1 || f.issue.Title != "updated title" || f.issue.State != "open" || !strings.HasPrefix(f.comments[0].Body, "updated comment") {
		t.Fatal("update contract failed", f.issue, f.comments)
	}
	// Typed work items require explicit mappings; issue label values map to IDs,
	// and changing labels in PATCH cannot be concealed as a second request.
	s, b = v1Source(t, kind, version)
	for i := range s.Records {
		r := &s.Records[i]
		if r.Key.Kind == "issue" {
			r.Key.Kind = "work_item"
			var c migration.IssueContent
			_ = json.Unmarshal(r.Content, &c)
			c.Type = "Bug"
			c.State = "Resolved"
			c.Labels = []string{"source-bug"}
			c.Assignees = []string{"source-user"}
			r.Content = v1JSON(c)
		} else {
			r.Links[0].Relation = "work_item"
			r.Links[0].Target.Kind = "work_item"
		}
	}
	f = newV1Fixture(t, kind, version)
	d, _ = OpenMigrationDestination(f.config())
	sel = migration.Selection{Destination: d.Locator(), Mappings: map[string]string{}}
	ins, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
	if e != nil || len(ins) != 0 || len(fs) == 0 {
		t.Fatal("implicit typed mapping", ins, fs, e)
	}
	sel.Mappings = map[string]string{"type:Bug": "", "state:Resolved": "closed", "label:source-bug": "41", "assignee:source-user": "mapped-user"}
	session = v1Prepare(t, s, b, d, sel)
	if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
		t.Fatal(e)
	}
	if f.issue.State != "closed" || len(f.issue.Labels) != 1 || f.issue.Labels[0].ID != 41 || f.issue.Assignees[0].Login != "mapped-user" {
		t.Fatal("explicit mapping lost", f.issue)
	}
	for _, r := range s.Records {
		if r.Key.Kind == "work_item" {
			sel.Mappings["target:"+r.Key.String()] = "7"
		}
	}
	sel.Mappings["label:source-bug"] = ""
	before := f.issueWrites
	ins, fs, e = d.Plan(context.Background(), s, sel, migration.DefaultLimits())
	if e != nil || len(ins) != 0 || f.issueWrites != before {
		t.Fatal("label-changing update accepted", ins, fs, e)
	}
}
func TestForgejoMigrationUpdatesAndMappings(t *testing.T) {
	runV1UpdatesAndMappings(t, "forgejo", "16.0.0")
}

func runV1Limits(t *testing.T, kind, version string) {
	f := newV1Fixture(t, kind, version)
	f.seed()
	c := f.config()
	s, _ := OpenMigrationSource(c)
	sel := migration.Selection{Source: c.Locator, Kinds: []string{"issue", "comment"}}
	l := migration.DefaultLimits()
	l.Records = 1
	if _, _, e := s.Snapshot(context.Background(), sel, l); e == nil {
		t.Fatal("record excess accepted")
	}
	l = migration.DefaultLimits()
	l.BlobBytes = 4
	if _, _, e := s.Snapshot(context.Background(), sel, l); e == nil {
		t.Fatal("source blob excess accepted")
	}
	f.private = true
	if _, _, e := s.Snapshot(context.Background(), sel, migration.DefaultLimits()); e == nil {
		t.Fatal("private repository called preserved")
	}
	f.private = false
	f.inaccessibleIssues = true
	missing, _, err := s.Snapshot(context.Background(), sel, migration.DefaultLimits())
	if err != nil || len(missing.Records) != 0 || len(missing.Findings) != 1 || missing.Findings[0].Status != "missing_dependency" {
		t.Fatal("private/deleted issue list called preserved", missing.Findings, err)
	}
	f.inaccessibleIssues = false
	f.version = "99.0.0"
	if _, _, e := s.Snapshot(context.Background(), sel, migration.DefaultLimits()); e == nil {
		t.Fatal("untested version accepted")
	}
	source, b := v1Source(t, kind, version)
	f = newV1Fixture(t, kind, version)
	d, _ := OpenMigrationDestination(f.config())
	sel = migration.Selection{Destination: d.Locator()}
	ins, _, e := d.Plan(context.Background(), source, sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	id := ins[0].ID
	for _, extra := range []int{0, 1} {
		changed := source
		changed.Records = append([]migration.Record{}, source.Records...)
		for i := range changed.Records {
			if changed.Records[i].Key.Kind == "issue" {
				var c migration.IssueContent
				_ = json.Unmarshal(changed.Records[i].Content, &c)
				c.Body = strings.Repeat("x", v1BodyBudget(id)+extra)
				changed.Records[i].Content = v1JSON(c)
			}
		}
		ins, fs, e := d.Plan(context.Background(), changed, sel, migration.DefaultLimits())
		if e != nil {
			t.Fatal(e)
		}
		if extra == 0 {
			if len(ins) != 2 {
				t.Fatal("exact marker capacity refused", fs)
			}
			session := v1Prepare(t, changed, b, d, sel)
			if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
				t.Fatal(e)
			}
			if len(f.issue.Body) != forgejoapi.BodyCapacity {
				t.Fatal("capacity projection changed")
			}
		} else if len(ins) != 0 {
			t.Fatal("known capacity excess allowed", fs)
		}
	}
}
func TestForgejoMigrationBudgetsAndAvailability(t *testing.T) { runV1Limits(t, "forgejo", "16.0.0") }

func TestForgejoMigrationOversizedSourcePreservesBytes(t *testing.T) {
	f := newV1Fixture(t, "forgejo", "16.0.0")
	f.seed()
	f.issue.Body = strings.Repeat("x", 65537)
	s, _ := OpenMigrationSource(f.config())
	snapshot, blobs, e := s.Snapshot(context.Background(), migration.Selection{Source: f.config().Locator, Kinds: []string{"issue", "comment"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if len(snapshot.Records) != 2 || snapshot.Records[0].Key.Kind == "issue" {
		t.Fatal("oversized source silently truncated", snapshot.Records)
	}
	found := false
	for _, b := range snapshot.Blobs {
		r, _ := blobs.Open(context.Background(), b.Digest)
		raw, _ := io.ReadAll(r)
		r.Close()
		found = found || bytes.Contains(raw, []byte(strings.Repeat("x", 65537)))
	}
	if !found {
		t.Fatal("source bytes absent")
	}
}

func runV1GitAndOfflineImport(t *testing.T, kind, version string) {
	root := inReplicationTestRepository(t)
	source := newV1Fixture(t, kind, version)
	source.seed()
	destination := newV1Fixture(t, kind, version)
	apiSource, e := OpenMigrationSource(source.config())
	if e != nil {
		t.Fatal(e)
	}
	apiDestination, e := OpenMigrationDestination(destination.config())
	if e != nil {
		t.Fatal(e)
	}
	selected := migration.Selection{Source: source.config().Locator, Destination: apiDestination.Locator(), Refs: []string{"refs/heads/main"}, Kinds: []string{"issue", "comment"}}
	composed := ComposeMigrationSources(apiSource, LocalMigrationSource{GitDir: filepath.Join(root, ".git")})
	snapshot, blobs, e := composed.Snapshot(context.Background(), selected, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer blobs.(io.Closer).Close()
	remote := migrationBare(t)
	target := ComposeMigrationDestinations(apiDestination, GitMigrationDestination{Remote: remote, Destination: apiDestination.Locator(), Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}})
	session := v1Prepare(t, snapshot, blobs, target, selected)
	if e = ApplyMigration(context.Background(), session, target, ""); e != nil {
		t.Fatal(e)
	}
	head := mustGitText(t, "rev-parse", "HEAD")
	observed, e := projectGit(context.Background(), remote, nil, 256, "show-ref", "--verify", "--hash", "refs/heads/main")
	if e != nil || strings.TrimSpace(string(observed)) != head {
		t.Fatal("shared Git composition lost selected history", e)
	}
	if source.issueWrites != 0 || destination.issueWrites != 1 || destination.commentWrites != 1 {
		t.Fatal("source changed or destination missing")
	}
	localRepo := migrationBare(t)
	local := LocalMigrationDestination{Repository: localRepo}
	localSelection := migration.Selection{Destination: local.Locator()}
	localSession := v1Prepare(t, snapshot, blobs, local, localSelection)
	if e = ApplyMigration(context.Background(), localSession, local, ""); e != nil {
		t.Fatal(e)
	}
	// Close sources: native inspection now uses only the verified saved bytes.
	source.server.Close()
	destination.server.Close()
	archive, e := ReadLocalMigration(context.Background(), localRepo, localSession.Plan.SnapshotDigest, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer archive.Close()
	if len(archive.Snapshot.Records) != 2 {
		t.Fatal("foreign records missing offline")
	}
	for _, blob := range archive.Snapshot.Blobs {
		r, e := archive.Open(context.Background(), blob.Digest)
		if e != nil {
			t.Fatal(e)
		}
		data, _ := io.ReadAll(r)
		r.Close()
		if migration.Digest(data) != blob.Digest {
			t.Fatal("offline digest mismatch")
		}
	}
	refs, e := projectGit(context.Background(), localRepo, nil, 4096, "for-each-ref", "--format=%(refname)", "refs/hn/actors")
	if e != nil || len(bytes.TrimSpace(refs)) != 0 {
		t.Fatal("foreign users acquired native authority", string(refs), e)
	}
}
func TestForgejoMigrationSharedGitAndOfflineImport(t *testing.T) {
	runV1GitAndOfflineImport(t, "forgejo", "16.0.0")
}

func runV1ProjectionLoss(t *testing.T, kind, version string) {
	source, blobs := v1Source(t, kind, version)
	for i := range source.Records {
		if source.Records[i].Key.Kind == "issue" {
			var c migration.IssueContent
			_ = json.Unmarshal(source.Records[i].Content, &c)
			c.Labels = []string{"source-bug"}
			source.Records[i].Content = v1JSON(c)
		}
	}
	f := newV1Fixture(t, kind, version)
	f.dropLabels = true
	d, _ := OpenMigrationDestination(f.config())
	sel := migration.Selection{Destination: d.Locator(), Mappings: map[string]string{"label:source-bug": "41"}}
	session := v1Prepare(t, source, blobs, d, sel)
	if e := ApplyMigration(context.Background(), session, d, ""); e == nil {
		t.Fatal("successful HTTP response concealed lost labels")
	}
	if e := ApplyMigration(context.Background(), session, d, ""); e == nil {
		t.Fatal("inexact projection confirmed")
	}
	if f.issueWrites != 1 || f.commentWrites != 0 {
		t.Fatal("inexact create repeated")
	}
}
func TestForgejoMigrationProjectionLossRefusesRecovery(t *testing.T) {
	runV1ProjectionLoss(t, "forgejo", "16.0.0")
}
