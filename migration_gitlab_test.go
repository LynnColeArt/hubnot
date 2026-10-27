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

	"hubnot/internal/gitlabapi"
	"hubnot/internal/migration"
)

// Shared with WP11: this fixture serves documented REST v4 responses and can
// remain in the test process while a separately built hn CLI contacts Server.
type gitlabMigrationFixture struct {
	Mu                                                              sync.Mutex
	Server                                                          *httptest.Server
	ProjectID                                                       int64
	Issue                                                           gitlabapi.Issue
	Notes                                                           []gitlabapi.Note
	IssueWrites, NoteWrites                                         int
	LoseIssue, LoseNote, HideIssue, HideNotes, Duplicate, DropLabel bool
	PrivateNotes, MissingReviews, MissingBoards                     bool
	LegacyPrivateNote                                               bool
}

func newGitLabMigrationFixture(t *testing.T) *gitlabMigrationFixture {
	t.Helper()
	f := &gitlabMigrationFixture{ProjectID: 42, Notes: []gitlabapi.Note{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.Mu.Lock()
		defer f.Mu.Unlock()
		if r.Header.Get("PRIVATE-TOKEN") != "supplied-secret" {
			t.Error("missing explicit GitLab token")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("unexpected authorization mode")
		}
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch {
		case (r.URL.Path == "/gitlab/api/v4/projects/group/nested/repo" || r.URL.Path == "/gitlab/api/v4/projects/42"):
			write(map[string]any{"id": f.ProjectID, "path_with_namespace": "group/nested/repo", "created_at": "2026-01-01T00:00:00Z", "default_branch": "main", "issues_enabled": true, "permissions": map[string]any{"project_access": map[string]any{"access_level": 40}}, "ci_config_path": ""})
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues" && r.Method == "POST":
			var in struct {
				Title      string  `json:"title"`
				Body       string  `json:"description"`
				Labels     string  `json:"labels"`
				Assignees  []int64 `json:"assignee_ids"`
				Type       string  `json:"issue_type"`
				StateEvent string  `json:"state_event"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in.StateEvent != "" {
				t.Error("create attempted state change")
			}
			f.IssueWrites++
			f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: f.ProjectID, Title: in.Title, Body: in.Body, Type: in.Type, State: "opened", Labels: []string{}, Assignees: []gitlabapi.User{}, Author: gitlabapi.User{ID: 10, Username: "destination"}, CreatedAt: "2026-01-02T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}
			if in.Labels != "" && !f.DropLabel {
				f.Issue.Labels = strings.Split(in.Labels, ",")
			}
			for _, id := range in.Assignees {
				f.Issue.Assignees = append(f.Issue.Assignees, gitlabapi.User{ID: id, Username: "mapped"})
			}
			if f.LoseIssue {
				f.LoseIssue = false
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(201)
			write(f.Issue)
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues" && r.Method == "GET":
			rows := []gitlabapi.Issue{}
			if f.Issue.ID > 0 && !f.HideIssue {
				rows = append(rows, f.Issue)
			}
			if f.Duplicate && len(rows) > 0 {
				v := f.Issue
				v.ID++
				v.IID++
				rows = append(rows, v)
			}
			write(rows)
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues/7" && r.Method == "GET":
			write(f.Issue)
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues/7" && r.Method == "PUT":
			var in struct {
				Title     string  `json:"title"`
				Body      string  `json:"description"`
				State     string  `json:"state_event"`
				Labels    string  `json:"labels"`
				Assignees []int64 `json:"assignee_ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			f.IssueWrites++
			f.Issue.Title = in.Title
			f.Issue.Body = in.Body
			if in.State == "close" {
				f.Issue.State = "closed"
			} else {
				f.Issue.State = "opened"
			}
			f.Issue.Labels = []string{}
			if in.Labels != "" {
				f.Issue.Labels = strings.Split(in.Labels, ",")
			}
			f.Issue.Assignees = []gitlabapi.User{}
			for _, id := range in.Assignees {
				f.Issue.Assignees = append(f.Issue.Assignees, gitlabapi.User{ID: id})
			}
			f.Issue.UpdatedAt = "2026-02-01T00:00:00Z"
			write(f.Issue)
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues/7/notes" && r.Method == "POST":
			var in struct {
				Body string `json:"body"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			f.NoteWrites++
			v := gitlabapi.Note{ID: 9001, Body: in.Body, Author: gitlabapi.User{ID: 10, Username: "destination"}, NoteableID: f.Issue.ID, CreatedAt: "2026-01-03T00:00:00Z", UpdatedAt: "2026-01-03T00:00:00Z"}
			f.Notes = append(f.Notes, v)
			if f.LoseNote {
				f.LoseNote = false
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(201)
			write(v)
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues/7/notes/9001":
			if len(f.Notes) == 0 {
				w.WriteHeader(404)
				return
			}
			if r.Method == "PUT" {
				var in struct {
					Body string `json:"body"`
				}
				_ = json.NewDecoder(r.Body).Decode(&in)
				f.NoteWrites++
				f.Notes[0].Body = in.Body
				f.Notes[0].UpdatedAt = "2026-02-01T00:00:00Z"
				if f.LoseNote {
					f.LoseNote = false
					w.WriteHeader(500)
					return
				}
			}
			if f.LegacyPrivateNote {
				write(map[string]any{"id": 9001, "body": f.Notes[0].Body, "noteable_id": 7001, "confidential": true})
				return
			}
			write(f.Notes[0])
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues/7/notes":
			if f.PrivateNotes {
				w.WriteHeader(403)
			} else if f.HideNotes {
				write([]gitlabapi.Note{})
			} else if f.LegacyPrivateNote {
				body := "legacy private note"
				if len(f.Notes) > 0 {
					body = f.Notes[0].Body
				}
				write([]any{map[string]any{"id": 9001, "body": body, "noteable_id": 7001, "confidential": true, "author": map[string]any{"id": 3, "username": "source-author"}}})
			} else {
				write(f.Notes)
			}
		case r.URL.Path == "/gitlab/api/v4/projects/42/issues/7/discussions":
			write([]any{map[string]any{"id": "thread-hash", "individual_note": false, "notes": []any{map[string]any{"id": 9002, "body": "system changed status", "system": true, "type": "DiscussionNote", "author": map[string]any{"id": 3, "username": "old-user"}, "noteable_id": 7001}, map[string]any{"id": 9003, "body": "thread reply", "system": false, "type": "DiscussionNote", "author": map[string]any{"id": 4, "username": "deleted-user"}, "noteable_id": 7001}}}})
		case r.URL.Path == "/gitlab/api/v4/projects/42/merge_requests":
			write([]any{map[string]any{"id": 8001, "iid": 11, "project_id": 42, "description": "MR source", "state": "merged", "sha": strings.Repeat("a", 40), "author": map[string]any{"id": 3, "username": "reviewer"}}})
		case r.URL.Path == "/gitlab/api/v4/projects/42/merge_requests/11/approvals":
			if f.MissingReviews {
				w.WriteHeader(403)
			} else {
				write(map[string]any{"id": 8001, "iid": 11, "approved_by": []any{map[string]any{"user": map[string]any{"id": 3, "username": "reviewer"}}}})
			}
		case r.URL.Path == "/gitlab/api/v4/projects/42/merge_requests/11/discussions":
			write([]any{map[string]any{"id": "mr-thread", "individual_note": false, "notes": []any{map[string]any{"id": 9100, "body": "inline MR note", "type": "DiffNote", "noteable_id": 8001, "position": map[string]any{"new_path": "x.go", "new_line": 4, "head_sha": strings.Repeat("a", 40)}}, map[string]any{"id": 9101, "body": "MR reply", "type": "DiscussionNote", "noteable_id": 8001}}}})
		case r.URL.Path == "/gitlab/api/v4/projects/42/boards":
			if f.MissingBoards {
				w.WriteHeader(404)
			} else {
				write([]any{map[string]any{"id": 3, "name": "team board", "lists": []any{map[string]any{"id": 4, "label": map[string]any{"id": 8, "name": "todo"}, "position": 0}}}})
			}
		case r.URL.Path == "/gitlab/api/v4/projects/42/boards/3/lists":
			write([]any{map[string]any{"id": 4, "label": map[string]any{"id": 8, "name": "todo"}, "position": 0}})
		case strings.HasPrefix(r.URL.Path, "/gitlab/api/v4/projects/42/repository/commits/"):
			write(map[string]any{"id": strings.Repeat("a", 40)})
		case r.URL.Path == "/gitlab/api/v4/projects/42/repository/files/.gitlab-ci.yml":
			if r.URL.Query().Get("ref") != strings.Repeat("a", 40) {
				t.Error("CI read not pinned")
			}
			write(map[string]any{"file_path": ".gitlab-ci.yml", "encoding": "base64", "commit_id": strings.Repeat("a", 40), "content": base64.StdEncoding.EncodeToString([]byte("include: external.yml\njob:\n  script: echo inert\n"))})
		case r.URL.Path == "/gitlab/api/v4/projects/42/uploads/17":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = io.WriteString(w, "exact selected upload")
		case r.URL.Path == "/gitlab/api/v4/projects/42/uploads/private":
			w.WriteHeader(403)
		default:
			t.Errorf("unexpected GitLab request %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.Server.Close)
	return f
}
func (f *gitlabMigrationFixture) Config() migration.ProviderConfig {
	return migration.ProviderConfig{Locator: migration.Locator{Kind: "gitlab", Endpoint: f.Server.URL + "/gitlab/api/v4", Repository: "group/nested/repo"}, Token: "supplied-secret", APIVersion: "v4", CredentialMode: "private-token"}
}
func gitlabMigrationSourceFixture() (migration.Snapshot, gitlabBlobs) {
	key := migration.Key{Provider: "native", Instance: "https://source.invalid", ProjectID: "immutable", Kind: "issue", ID: "source-issue"}
	note := key
	note.Kind = "comment"
	note.ID = "source-note"
	s := migration.Snapshot{Schema: "hn.project-snapshot/1", Source: migration.Locator{Kind: "native", Endpoint: key.Instance, Repository: "source"}, SourceID: "source", Records: []migration.Record{{Key: key, Selector: "1", Revision: "source-v1", Author: "original author", Schema: "hn.migration.issue/1", Content: gitlabJSON(migration.IssueContent{Schema: "hn.migration.issue/1", Title: "Moved issue", Body: "Exact original body", State: "opened", Type: "issue", Labels: []string{}, Assignees: []string{}}), Links: []migration.Link{}, Blobs: []string{}}, {Key: note, Revision: "note-v1", Author: "original commenter", Schema: "hn.migration.comment/1", Content: gitlabJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: "Original note"}), Links: []migration.Link{{Relation: "issue", Target: key, Revision: "source-v1"}}, Blobs: []string{}}}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}
	return s, gitlabBlobs{}
}
func TestGitLabRegistryAndInertConstruction(t *testing.T) {
	c := migration.ProviderConfig{Locator: migration.Locator{Kind: "gitlab", Endpoint: "https://unused.invalid/gitlab/api/v4", Repository: "group/nested/repo"}, Token: "supplied"}
	s, e := OpenMigrationSource(c)
	if e != nil {
		t.Fatal(e)
	}
	d, e := OpenMigrationDestination(c)
	if e != nil {
		t.Fatal(e)
	}
	p, ok := migrationProviders.Lookup("gitlab")
	if !ok {
		t.Fatal("registration missing")
	}
	sc, _ := s.Capabilities(context.Background())
	dc, _ := d.Capabilities(context.Background())
	if e = migration.ValidateProviderCoverage(p.RequiredCoverage, sc, dc); e != nil {
		t.Fatal(e)
	}
	if d.Locator() != c.Locator {
		t.Fatal(d.Locator())
	}
	for _, field := range []string{"version", "mode", "board", "project"} {
		bad := c
		switch field {
		case "version":
			bad.APIVersion = "v3"
		case "mode":
			bad.CredentialMode = "ambient"
		case "board":
			bad.PlanningSelectors = []string{"board-name"}
		case "project":
			bad.Locator.Project = "board"
		}
		if _, e = OpenMigrationSource(bad); e == nil {
			t.Fatal("accepted", field)
		}
	}
}
func TestGitLabEngineLostResponsesFreshReceiptsAndNoDuplicateEffects(t *testing.T) {
	for _, lost := range []string{"issue", "note"} {
		t.Run(lost, func(t *testing.T) {
			f := newGitLabMigrationFixture(t)
			f.LoseIssue = lost == "issue"
			f.LoseNote = lost == "note"
			s, blobs := gitlabMigrationSourceFixture()
			d, _ := OpenMigrationDestination(f.Config())
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "session"), s, blobs, migration.Selection{Destination: d.Locator()}, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("lost response accepted")
			}
			f.HideIssue = lost == "issue"
			f.HideNotes = lost == "note"
			fresh, _ := OpenMigrationDestination(f.Config())
			restarted, e := OpenMigrationSession(context.Background(), session.Directory)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(context.Background(), restarted, fresh, ""); e == nil || !strings.Contains(e.Error(), "manual recovery") {
				t.Fatal("negative observation allowed duplicate", e)
			}
			if f.IssueWrites != 1 || f.NoteWrites > 1 {
				t.Fatal("write repeated")
			}
			f.HideIssue = false
			f.HideNotes = false
			if e = ApplyMigration(context.Background(), restarted, fresh, ""); e != nil {
				t.Fatal(e)
			}
			if f.IssueWrites != 1 || f.NoteWrites != 1 {
				t.Fatal(f.IssueWrites, f.NoteWrites)
			}
			raw, e := migrationUnseal(session.Directory, "progress.json", 1<<20)
			if e != nil {
				t.Fatal(e)
			}
			var p migration.Progress
			if e = json.Unmarshal(raw, &p); e != nil {
				t.Fatal(e)
			}
			if p.Entries[0].Receipts[0].RemoteID != "id:7001" || p.Entries[0].Receipts[0].RemoteKey != "7" {
				t.Fatal("global ID conflated with IID", p)
			}
			if !strings.Contains(string(p.Entries[1].Write.Payload), `"parent":{"id":"7"}`) || strings.Contains(string(raw), "supplied-secret") {
				t.Fatal(string(raw))
			}
			before := string(p.Entries[1].Write.Payload)
			if e = ApplyMigration(context.Background(), restarted, fresh, ""); e != nil {
				t.Fatal(e)
			}
			raw, _ = migrationUnseal(session.Directory, "progress.json", 1<<20)
			_ = json.Unmarshal(raw, &p)
			if string(p.Entries[1].Write.Payload) != before || f.IssueWrites != 1 || f.NoteWrites != 1 {
				t.Fatal("confirmed resume altered payload/effects")
			}
		})
	}
}
func TestGitLabSnapshotFidelityTopologyUploadsAndInertCI(t *testing.T) {
	f := newGitLabMigrationFixture(t)
	f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "Source issue", Body: "original", State: "closed", Type: "incident", Author: gitlabapi.User{ID: 3, Username: "old-user"}, Labels: []string{}, Assignees: []gitlabapi.User{}}
	f.Notes = []gitlabapi.Note{{ID: 9001, Body: "ordinary note", NoteableID: 7001, Author: gitlabapi.User{ID: 3, Username: "old-user"}}, {ID: 9002, Body: "system changed status", NoteableID: 7001, System: true, Type: "DiscussionNote"}, {ID: 9003, Body: "thread reply", NoteableID: 7001, Type: "DiscussionNote"}}
	c := f.Config()
	c.PlanningSelectors = []string{"3"}
	parent := migration.Key{Provider: "gitlab", Instance: c.Locator.Endpoint, ProjectID: "id:42", Kind: "issue", ID: "id:7001"}
	c.Attachments = []migration.AttachmentSelection{{URL: f.Server.URL + "/gitlab/api/v4/projects/42/uploads/17", Name: "asset", Parent: parent}, {URL: f.Server.URL + "/gitlab/api/v4/projects/42/uploads/private", Parent: parent}}
	src, _ := OpenMigrationSource(c)
	s, blobs, e := src.Snapshot(context.Background(), migration.Selection{Source: c.Locator}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	kinds := map[string]int{}
	verified := false
	yaml := false
	for _, r := range s.Records {
		kinds[r.Key.Kind]++
		if r.Key.ProjectID != "id:42" {
			t.Fatal(r.Key)
		}
		if r.Key.Kind == "issue" {
			var v migration.IssueContent
			_ = json.Unmarshal(r.Content, &v)
			if v.Type != "incident" || v.State != "closed" || r.Selector != "7" || r.Key.ID != "id:7001" {
				t.Fatal(v, r)
			}
		}
		if r.Key.Kind == "comment" && r.Key.ID != "id:9001" {
			t.Fatal("system/thread became ordinary comment", r)
		}
		if r.Key.ID == "id:9003" && len(r.Links) != 3 {
			t.Fatal("thread ordering lost", r.Links)
		}
		if r.Key.ID == "id:9101" && len(r.Links) != 3 {
			t.Fatal("MR reply topology lost", r.Links)
		}
		for _, d := range r.Blobs {
			reader, e := blobs.Open(context.Background(), d)
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := io.ReadAll(reader)
			_ = reader.Close()
			if migration.Digest(raw) != d {
				t.Fatal("digest mismatch")
			}
			verified = verified || string(raw) == "exact selected upload"
			yaml = yaml || strings.Contains(string(raw), "script: echo inert")
		}
	}
	for _, k := range []string{"issue", "comment", "discussion", "reply", "change_request", "review", "inline_comment", "project", "project_item", "workflow"} {
		if kinds[k] == 0 {
			t.Fatal("missing kind", k, kinds)
		}
	}
	if !verified || !yaml {
		t.Fatal("selected source bytes missing")
	}
	if !strings.HasPrefix(s.SourceID, "gitlab:") || strings.Contains(s.SourceID, "nested") {
		t.Fatal("mutable SourceID", s.SourceID)
	}
	missing := false
	for _, x := range s.Findings {
		missing = missing || x.Status == "missing_dependency"
	}
	if !missing {
		t.Fatal("private upload claimed preserved")
	}
	if f.IssueWrites != 0 || f.NoteWrites != 0 {
		t.Fatal("snapshot mutated remote")
	}
	l := migration.DefaultLimits()
	l.Records = 1
	if _, _, e = src.Snapshot(context.Background(), migration.Selection{Source: c.Locator}, l); e == nil {
		t.Fatal("record limit ignored")
	}
	l = migration.DefaultLimits()
	l.BlobBytes = 4
	if _, _, e = src.Snapshot(context.Background(), migration.Selection{Source: c.Locator}, l); e == nil {
		t.Fatal("blob limit ignored")
	}
}
func TestGitLabStaleProjectIssueAndParentRefuseBeforeEffects(t *testing.T) {
	for _, changed := range []string{"project", "issue-revision", "issue-id", "parent"} {
		t.Run(changed, func(t *testing.T) {
			f := newGitLabMigrationFixture(t)
			s, blobs := gitlabMigrationSourceFixture()
			sel := migration.Selection{Mappings: map[string]string{}}
			if changed == "issue-revision" || changed == "issue-id" {
				f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "existing", Body: "existing", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}}
				sel.Mappings["target:"+s.Records[0].Key.String()] = "7"
			}
			if changed == "parent" {
				f.LoseNote = true
			}
			d, _ := OpenMigrationDestination(f.Config())
			sel.Destination = d.Locator()
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			expected := 0
			switch changed {
			case "project":
				f.ProjectID++
			case "issue-revision":
				f.Issue.UpdatedAt = "2026-02-01T00:00:00Z"
			case "issue-id":
				f.Issue.ID++
			case "parent":
				if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
					t.Fatal("loss accepted")
				}
				expected = 1
				f.Issue.ID++
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("stale destination accepted")
			}
			if f.IssueWrites != expected || f.NoteWrites > expected {
				t.Fatal("stale destination mutated", f.IssueWrites, f.NoteWrites)
			}
		})
	}
}
func TestGitLabPlanningRefusalsAreBeforeEffects(t *testing.T) {
	for _, problem := range []string{"closed", "type", "title", "label", "assignee", "work-item"} {
		t.Run(problem, func(t *testing.T) {
			f := newGitLabMigrationFixture(t)
			s, _ := gitlabMigrationSourceFixture()
			var v migration.IssueContent
			_ = json.Unmarshal(s.Records[0].Content, &v)
			m := map[string]string{}
			switch problem {
			case "closed":
				v.State = "closed"
			case "type":
				v.Type = "incident"
			case "title":
				v.Title = strings.Repeat("x", 256)
			case "label":
				v.Labels = []string{"x"}
				m["label:x"] = "bad,comma"
			case "assignee":
				v.Assignees = []string{"old"}
				m["assignee:old"] = "username"
			case "work-item":
				s.Records = s.Records[:1]
				s.Records[0].Key.Kind = "work_item"
			}
			s.Records[0].Content = gitlabJSON(v)
			d, _ := OpenMigrationDestination(f.Config())
			ins, fs, e := d.Plan(context.Background(), s, migration.Selection{Destination: d.Locator(), Mappings: m}, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			if len(ins) != 0 || len(fs) == 0 || f.IssueWrites != 0 || f.NoteWrites != 0 {
				t.Fatal(ins, fs)
			}
		})
	}
}
func TestGitLabInaccessibleTierAndDeletedSelectionsAreFindings(t *testing.T) {
	f := newGitLabMigrationFixture(t)
	f.MissingReviews = true
	f.MissingBoards = true
	c := f.Config()
	c.PlanningSelectors = []string{"3"}
	src, _ := OpenMigrationSource(c)
	s, _, e := src.Snapshot(context.Background(), migration.Selection{Source: c.Locator, Kinds: []string{"change_request", "review", "project"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range s.Records {
		if r.Key.Kind == "review" || r.Key.Kind == "project" {
			t.Fatal("unread content preserved", r)
		}
	}
	if len(s.Findings) != 2 {
		t.Fatal(s.Findings)
	}
}
func TestGitLabAmbiguousAndPrivateRecoveryNeverRepeatsCreate(t *testing.T) {
	for _, condition := range []string{"duplicate", "private"} {
		t.Run(condition, func(t *testing.T) {
			f := newGitLabMigrationFixture(t)
			f.LoseIssue = true
			s, blobs := gitlabMigrationSourceFixture()
			d, _ := OpenMigrationDestination(f.Config())
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, migration.Selection{Destination: d.Locator()}, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			_ = ApplyMigration(context.Background(), session, d, "")
			if condition == "duplicate" {
				f.Duplicate = true
			} else {
				f.Issue.Confidential = true
			}
			fresh, _ := OpenMigrationDestination(f.Config())
			if e = ApplyMigration(context.Background(), session, fresh, ""); e == nil {
				t.Fatal("unproven create accepted")
			}
			if f.IssueWrites != 1 || f.NoteWrites != 0 {
				t.Fatal("duplicate effects")
			}
		})
	}
}
func TestGitLabExplicitUpdateAndMapping(t *testing.T) {
	f := newGitLabMigrationFixture(t)
	f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "existing", Body: "existing", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}}
	s, blobs := gitlabMigrationSourceFixture()
	var v migration.IssueContent
	_ = json.Unmarshal(s.Records[0].Content, &v)
	v.State = "resolved"
	v.Type = "bug"
	v.Labels = []string{"old-label"}
	v.Assignees = []string{"old-user"}
	s.Records[0].Content = gitlabJSON(v)
	d, _ := OpenMigrationDestination(f.Config())
	sel := migration.Selection{Destination: d.Locator(), Mappings: map[string]string{"state:resolved": "closed", "type:bug": "issue", "label:old-label": "mapped", "assignee:old-user": "10", "target:" + s.Records[0].Key.String(): "7"}}
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
		t.Fatal(e)
	}
	if f.IssueWrites != 1 || f.NoteWrites != 1 || f.Issue.State != "closed" || f.Issue.ID != 7001 || f.Issue.Assignees[0].ID != 10 {
		t.Fatal(f.Issue, f.IssueWrites, f.NoteWrites)
	}
}

func TestGitLabNoteUpdateBindsRevisionAndRecoversOnlyExactProjection(t *testing.T) {
	for _, mode := range []string{"success", "stale", "lost"} {
		t.Run(mode, func(t *testing.T) {
			f := newGitLabMigrationFixture(t)
			f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "existing", Body: "existing", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}}
			f.Notes = []gitlabapi.Note{{ID: 9001, Body: "existing note", NoteableID: 7001}}
			s, blobs := gitlabMigrationSourceFixture()
			d, _ := OpenMigrationDestination(f.Config())
			sel := migration.Selection{Destination: d.Locator(), Mappings: map[string]string{"target:" + s.Records[0].Key.String(): "7", "target:" + s.Records[1].Key.String(): "9001"}}
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "stale" {
				f.Notes[0].UpdatedAt = "2026-03-01T00:00:00Z"
			}
			if mode == "lost" {
				f.LoseNote = true
			}
			e = ApplyMigration(context.Background(), session, d, "")
			if mode == "stale" {
				if e == nil || f.NoteWrites != 0 {
					t.Fatal("stale note overwritten", e)
				}
				return
			}
			if mode == "lost" {
				if e == nil {
					t.Fatal("lost update accepted")
				}
				fresh, _ := OpenMigrationDestination(f.Config())
				restarted, err := OpenMigrationSession(context.Background(), session.Directory)
				if err != nil {
					t.Fatal(err)
				}
				e = ApplyMigration(context.Background(), restarted, fresh, "")
			}
			if e != nil || f.IssueWrites != 1 || f.NoteWrites != 1 || f.Notes[0].ID != 9001 || !strings.HasPrefix(f.Notes[0].Body, "Original note") {
				t.Fatal(e, f.IssueWrites, f.NoteWrites, f.Notes)
			}
		})
	}
}
func TestGitLabSourceIdentityScopesInstances(t *testing.T) {
	ids := []string{}
	for i := 0; i < 2; i++ {
		f := newGitLabMigrationFixture(t)
		src, _ := OpenMigrationSource(f.Config())
		s, _, e := src.Snapshot(context.Background(), migration.Selection{Source: f.Config().Locator, Kinds: []string{"issue"}}, migration.DefaultLimits())
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, s.SourceID)
	}
	if ids[0] == ids[1] {
		t.Fatal("project IDs collided across self-hosted instances")
	}
}

func TestGitLabUsesSharedGitCompanionAndKeepsPortableSourceArchive(t *testing.T) {
	root := inReplicationTestRepository(t)
	srcFixture := newGitLabMigrationFixture(t)
	srcFixture.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "Selected issue", Body: "Original body", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}}
	dstFixture := newGitLabMigrationFixture(t)
	apiSource, _ := OpenMigrationSource(srcFixture.Config())
	apiDest, _ := OpenMigrationDestination(dstFixture.Config())
	remote := migrationBare(t)
	ctx := context.Background()
	l := migration.DefaultLimits()
	sel := migration.Selection{Source: srcFixture.Config().Locator, Destination: apiDest.Locator(), Refs: []string{"refs/heads/main"}, Kinds: []string{"issue"}}
	source := ComposeMigrationSources(apiSource, LocalMigrationSource{GitDir: filepath.Join(root, ".git")})
	snap, blobs, e := source.Snapshot(ctx, sel, l)
	if e != nil {
		t.Fatal(e)
	}
	defer blobs.(io.Closer).Close()
	if len(snap.NativeRefs) != 1 {
		t.Fatal("selected Git ref missing")
	}
	var archive bytes.Buffer
	if e = migration.WriteArchive(ctx, &archive, snap, blobs, l); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(archive.Bytes(), []byte(srcFixture.Config().Token)) {
		t.Fatal("credential in portable archive")
	}
	d := ComposeMigrationDestinations(apiDest, GitMigrationDestination{Remote: remote, Destination: apiDest.Locator(), Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}})
	_, fs, e := d.Plan(ctx, snap, sel, l)
	if e != nil {
		t.Fatal(e)
	}
	decisions := []migration.Decision{}
	for _, f := range fs {
		if f.Status != "mapped" && f.Status != "preserved" {
			decisions = append(decisions, migration.Decision{FindingID: f.ID, Action: "preserve", Mapping: json.RawMessage(`{}`)})
		}
	}
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), snap, blobs, sel, d, l, decisions)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, session, d, ""); e != nil {
		t.Fatal(e)
	}
	head := mustGitText(t, "rev-parse", "HEAD")
	got, e := projectGit(ctx, remote, nil, 256, "show-ref", "--verify", "--hash", "refs/heads/main")
	if e != nil || strings.TrimSpace(string(got)) != head || srcFixture.IssueWrites != 0 || dstFixture.IssueWrites != 1 {
		t.Fatal("code/communications transport failed", e, string(got))
	}
	verified, e := migration.ReadArchive(ctx, bytes.NewReader(archive.Bytes()), l)
	if e != nil {
		t.Fatal(e)
	}
	defer verified.Close()
	raw1, _ := migration.CanonicalSnapshot(snap, l)
	raw2, _ := migration.CanonicalSnapshot(verified.Snapshot, l)
	if !bytes.Equal(raw1, raw2) {
		t.Fatal("archive changed retrieved source observations")
	}
}

func TestGitLabSnapshotExactRecordBlobAndAggregateBudgets(t *testing.T) {
	f := newGitLabMigrationFixture(t)
	f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "bounded", Body: "source", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}}
	c := f.Config()
	src, _ := OpenMigrationSource(c)
	sel := migration.Selection{Source: c.Locator, Kinds: []string{"issue"}}
	l := migration.DefaultLimits()
	snap, _, e := src.Snapshot(context.Background(), sel, l)
	if e != nil {
		t.Fatal(e)
	}
	l.Records = len(snap.Records)
	l.Blobs = len(snap.Blobs)
	l.AttachmentBytes = 0
	l.BlobBytes = 1
	for _, b := range snap.Blobs {
		l.AttachmentBytes += b.Size
		if b.Size > l.BlobBytes {
			l.BlobBytes = b.Size
		}
	}
	if _, _, e = src.Snapshot(context.Background(), sel, l); e != nil {
		t.Fatal("exact source budgets rejected", e)
	}
	l.AttachmentBytes--
	if _, _, e = src.Snapshot(context.Background(), sel, l); e == nil {
		t.Fatal("one extra source byte accepted")
	}
}

func TestGitLabConfidentialAndOversizedSourceBytesRemainInspectable(t *testing.T) {
	for _, mode := range []string{"confidential", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			f := newGitLabMigrationFixture(t)
			body := "private source"
			if mode == "oversized" {
				body = strings.Repeat("x", 65537)
			}
			f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "source", Body: body, State: "opened", Type: "issue", Confidential: mode == "confidential", Labels: []string{}, Assignees: []gitlabapi.User{}}
			src, _ := OpenMigrationSource(f.Config())
			snap, blobs, e := src.Snapshot(context.Background(), migration.Selection{Source: f.Config().Locator, Kinds: []string{"issue"}}, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, r := range snap.Records {
				if r.Key.Kind == "issue" {
					t.Fatal("unrepresentable/private source became ordinary issue")
				}
				for _, d := range r.Blobs {
					rc, err := blobs.Open(context.Background(), d)
					if err != nil {
						t.Fatal(err)
					}
					raw, _ := io.ReadAll(rc)
					_ = rc.Close()
					var v gitlabapi.Issue
					if json.Unmarshal(raw, &v) == nil && v.ID == 7001 && v.Body == body {
						found = true
					}
				}
			}
			if !found || len(snap.Findings) == 0 {
				t.Fatal("original bytes or unavailable mapping finding missing")
			}
		})
	}
}

func TestGitLabOldV4ConfidentialNotePreservationAndTargetRefusal(t *testing.T) {
	f := newGitLabMigrationFixture(t)
	f.LegacyPrivateNote = true
	f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "source", Body: "source", Type: "issue", State: "opened", Labels: []string{}, Assignees: []gitlabapi.User{}}
	f.Notes = []gitlabapi.Note{{ID: 9001, Body: "private body", NoteableID: 7001}}
	c := f.Config()
	src, _ := OpenMigrationSource(c)
	snap, blobs, e := src.Snapshot(context.Background(), migration.Selection{Source: c.Locator, Kinds: []string{"issue", "comment"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range snap.Records {
		if r.Key.Kind == "comment" {
			t.Fatal("deprecated confidential flag became public ordinary comment")
		}
		if r.Key.Kind == "reply" {
			for _, d := range r.Blobs {
				rc, err := blobs.Open(context.Background(), d)
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := io.ReadAll(rc)
				_ = rc.Close()
				found = found || strings.Contains(string(raw), `"confidential":true`) && strings.Contains(string(raw), `private body`)
			}
		}
	}
	if !found || len(snap.Findings) == 0 {
		t.Fatal("private original bytes/finding missing")
	}
	legacy, _ := forgeProviders["gitlab"].New(c.Locator.Endpoint, c.Token)
	if _, e = legacy.ListComments(context.Background(), "42", "7"); e == nil {
		t.Fatal("legacy flattened confidential note")
	}
	s, _ := gitlabMigrationSourceFixture()
	d, _ := OpenMigrationDestination(c)
	ins, fs, e := d.Plan(context.Background(), s, migration.Selection{Destination: d.Locator(), Mappings: map[string]string{"target:" + s.Records[0].Key.String(): "7", "target:" + s.Records[1].Key.String(): "9001"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if len(ins) != 1 || ins[0].Kind != "issue" {
		t.Fatal("private note target planned", ins)
	}
	unsupported := false
	for _, finding := range fs {
		unsupported = unsupported || finding.Status == "unsupported"
	}
	if !unsupported || f.NoteWrites != 0 || f.IssueWrites != 0 {
		t.Fatal(fs)
	}
}
func TestGitLabOldV4PrivateNoteCannotProveLostCreate(t *testing.T) {
	f := newGitLabMigrationFixture(t)
	f.LoseNote = true
	s, blobs := gitlabMigrationSourceFixture()
	d, _ := OpenMigrationDestination(f.Config())
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, migration.Selection{Destination: d.Locator()}, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
		t.Fatal("lost response accepted")
	}
	f.LegacyPrivateNote = true
	fresh, _ := OpenMigrationDestination(f.Config())
	restarted, e := OpenMigrationSession(context.Background(), session.Directory)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), restarted, fresh, ""); e == nil || !strings.Contains(e.Error(), "manual recovery") {
		t.Fatal("private response confirmed an ordinary note", e)
	}
	if f.NoteWrites != 1 || f.IssueWrites != 1 {
		t.Fatal("private uncertainty repeated effects")
	}
}

func TestGitLabPlanRefusesQuickActionsAndRetainsExactSource(t *testing.T) {
	for _, kind := range []string{"issue", "comment"} {
		for _, update := range []bool{false, true} {
			t.Run(kind+fmt.Sprint(update), func(t *testing.T) {
				f := newGitLabMigrationFixture(t)
				f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "source", Body: "safe source", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}}
				f.Notes = []gitlabapi.Note{{ID: 9001, Body: "safe note", NoteableID: 7001}}
				body := "Exact source\n \t/confidential\n/move other/project"
				if kind == "issue" {
					f.Issue.Body = body
				} else {
					f.Notes[0].Body = body
				}
				src, _ := OpenMigrationSource(f.Config())
				snap, blobs, e := src.Snapshot(context.Background(), migration.Selection{Source: f.Config().Locator, Kinds: []string{"issue", "comment"}}, migration.DefaultLimits())
				if e != nil {
					t.Fatal(e)
				}
				preserved := false
				for _, r := range snap.Records {
					for _, d := range r.Blobs {
						rc, err := blobs.Open(context.Background(), d)
						if err != nil {
							t.Fatal(err)
						}
						raw, _ := io.ReadAll(rc)
						_ = rc.Close()
						var v map[string]json.RawMessage
						if json.Unmarshal(raw, &v) == nil {
							var actual string
							field := "description"
							if kind == "comment" {
								field = "body"
							}
							_ = json.Unmarshal(v[field], &actual)
							preserved = preserved || actual == body
						}
					}
				}
				if !preserved {
					t.Fatal("source slash-command bytes changed")
				}
				d, _ := OpenMigrationDestination(f.Config())
				mappings := map[string]string{}
				if update {
					for _, r := range snap.Records {
						if r.Key.Kind == "issue" {
							mappings["target:"+r.Key.String()] = "7"
						}
						if r.Key.Kind == "comment" {
							mappings["target:"+r.Key.String()] = "9001"
						}
					}
				}
				ins, fs, e := d.Plan(context.Background(), snap, migration.Selection{Destination: d.Locator(), Mappings: mappings}, migration.DefaultLimits())
				if e != nil {
					t.Fatal(e)
				}
				for _, in := range ins {
					if in.Kind == kind {
						t.Fatal("dangerous text planned", in)
					}
				}
				found := false
				for _, finding := range fs {
					found = found || finding.Status == "unsupported" && strings.Contains(finding.Message, "slash-command")
				}
				if !found || f.IssueWrites != 0 || f.NoteWrites != 0 {
					t.Fatal(fs)
				}
			})
		}
	}
}
