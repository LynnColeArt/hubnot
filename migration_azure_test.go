//go:build !noforge

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"hubnot/internal/azureapi"
	"hubnot/internal/migration"
)

const azureProjectID = "11111111-1111-4111-8111-111111111111"
const azureRepoID = "22222222-2222-4222-8222-222222222222"
const azureProcessID = "33333333-3333-4333-8333-333333333333"

type azureFixture struct {
	definitionOverride                                                               json.RawMessage
	apiVersion, commentsVersion                                                      string
	pat                                                                              bool
	mu                                                                               sync.Mutex
	server                                                                           *httptest.Server
	projectID, repoID, processID                                                     string
	item                                                                             azureapi.WorkItem
	comments                                                                         []azureapi.Comment
	writes, commentWrites                                                            int
	loseItem, loseComment, hideItem, hideComments, duplicate, conflict, threadCursor bool
}

func azureFixtureItem(id int, title, body string) azureapi.WorkItem {
	fields := map[string]json.RawMessage{}
	for k, v := range map[string]any{"System.TeamProject": "P", "System.Title": title, "System.Description": body, "System.WorkItemType": "Task", "System.State": "Active", "System.CreatedBy": map[string]string{"displayName": "source author"}, "System.CreatedDate": "2026-01-01T00:00:00Z", "System.ChangedDate": "2026-01-02T00:00:00Z", "Custom.Observed": "custom source"} {
		fields[k] = azureJSON(v)
	}
	return azureapi.WorkItem{ID: id, Rev: 1, Fields: fields, Relations: []json.RawMessage{azureJSON(map[string]any{"rel": "AttachedFile", "url": "https://example.invalid/file", "attributes": map[string]string{"name": "source.txt"}})}}
}
func newAzureFixture(t *testing.T) *azureFixture {
	f := &azureFixture{projectID: azureProjectID, repoID: azureRepoID, processID: azureProcessID, comments: []azureapi.Comment{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		auth := "Bearer supplied-azure-secret"
		if f.pat {
			auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(":supplied-azure-secret"))
		}
		if r.Header.Get("Authorization") != auth {
			t.Error("explicit token missing")
		}
		version := "7.1"
		if f.apiVersion != "" {
			version = f.apiVersion
		}
		if strings.Contains(r.URL.Path, "/comments") {
			version = "7.1-preview.4"
			if f.commentsVersion != "" {
				version = f.commentsVersion
			}
		}
		if r.URL.Query().Get("api-version") != version && r.URL.Path != "/attachment" && r.URL.Path != "/denied" {
			t.Error("API profile mismatch", r.URL)
		}
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		project := azureapi.Project{ID: f.projectID, Name: "P", State: "wellFormed", Capabilities: map[string]map[string]string{"processTemplate": {"templateTypeId": f.processID}}}
		switch {
		case strings.HasPrefix(r.URL.Path, "/org/_apis/projects/"):
			original := map[string]json.RawMessage{}
			_ = json.Unmarshal(azureJSON(project), &original)
			original["description"] = azureJSON("full planning description")
			original["visibility"] = azureJSON("private")
			write(original)
		case strings.HasPrefix(r.URL.Path, "/org/"+f.projectID+"/_apis/git/repositories/") && strings.Count(r.URL.Path, "/") == 6:
			rid := f.repoID
			if strings.HasSuffix(r.URL.Path, "/R2") {
				rid = "44444444-4444-4444-8444-444444444444"
			}
			write(azureapi.Repository{ID: rid, Name: "R", Project: project})
		case strings.Contains(r.URL.Path, "/wit/workitemtypes/"):
			if !strings.HasSuffix(r.URL.Path, "/Task") {
				w.WriteHeader(404)
				return
			}
			write(map[string]any{"name": "Task", "referenceName": "Microsoft.VSTS.WorkItemTypes.Task", "states": []map[string]string{{"name": "Active"}, {"name": "Done"}}})
		case strings.HasSuffix(r.URL.Path, "/wit/wiql"):
			var q map[string]string
			_ = json.NewDecoder(r.Body).Decode(&q)
			if !strings.Contains(q["query"], "[System.TeamProject] = @project") {
				t.Error("unscoped query")
			}
			rows := []map[string]int{}
			if f.item.ID > 0 && !f.hideItem {
				rows = append(rows, map[string]int{"id": f.item.ID})
				if f.duplicate {
					rows = append(rows, map[string]int{"id": f.item.ID + 1})
				}
			}
			write(map[string]any{"queryType": "flat", "asOf": "2026-01-01T00:00:00Z", "workItems": rows})
		case strings.HasSuffix(r.URL.Path, "/wit/workitems") && r.Method == "GET":
			ids := strings.Split(r.URL.Query().Get("ids"), ",")
			rows := []azureapi.WorkItem{}
			for _, id := range ids {
				n, _ := strconv.Atoi(id)
				v := f.item
				v.ID = n
				rows = append(rows, v)
			}
			write(map[string]any{"count": len(rows), "value": rows})
		case strings.Contains(r.URL.Path, "/wit/workitems/") && (r.Method == "POST" || r.Method == "PATCH"):
			if r.Header.Get("Content-Type") != "application/json-patch+json" {
				t.Error("not JSON patch")
			}
			var ops []azureapi.Patch
			_ = json.NewDecoder(r.Body).Decode(&ops)
			if r.Method == "PATCH" {
				if len(ops) < 2 || ops[0].Op != "test" || ops[0].Path != "/rev" || int(ops[0].Value.(float64)) != f.item.Rev {
					t.Error("missing revision test")
				}
				if f.conflict {
					w.WriteHeader(409)
					return
				}
			}
			f.writes++
			if f.item.ID == 0 {
				f.item = azureFixtureItem(71, "", "")
			} else {
				f.item.Rev++
			}
			for _, op := range ops {
				if op.Op == "add" {
					f.item.Fields[strings.TrimPrefix(op.Path, "/fields/")] = azureJSON(op.Value)
				}
			}
			if f.loseItem {
				f.loseItem = false
				w.WriteHeader(500)
				return
			}
			write(f.item)
		case strings.Contains(r.URL.Path, "/wit/workitems/") && r.Method == "GET":
			if f.hideItem {
				w.WriteHeader(404)
				return
			}
			write(f.item)
		case strings.HasSuffix(r.URL.Path, "/comments") && r.Method == "POST":
			var in map[string]string
			_ = json.NewDecoder(r.Body).Decode(&in)
			parts := strings.Split(r.URL.Path, "/")
			parent, _ := strconv.Atoi(parts[len(parts)-2])
			if parent != f.item.ID {
				t.Error("incorrect parent", parent, f.item.ID)
			}
			f.commentWrites++
			v := azureapi.Comment{ID: 91, WorkItemID: parent, Version: 1, Text: in["text"], CreatedBy: azureapi.Identity{DisplayName: "destination user"}}
			f.comments = append(f.comments, v)
			if f.loseComment {
				f.loseComment = false
				w.WriteHeader(500)
				return
			}
			write(v)
		case strings.HasSuffix(r.URL.Path, "/comments") && r.Method == "GET":
			rows := f.comments
			if f.hideComments {
				rows = []azureapi.Comment{}
			}
			write(map[string]any{"count": len(rows), "totalCount": len(rows), "comments": rows})
		case strings.HasSuffix(r.URL.Path, "/pullrequests"):
			write(map[string]any{"count": 1, "value": []any{map[string]any{"pullRequestId": 17, "title": "PR source", "repository": map[string]any{"id": f.repoID, "project": map[string]string{"id": f.projectID}}, "sourceRefName": "refs/heads/source", "targetRefName": "refs/heads/main", "lastMergeSourceCommit": map[string]string{"commitId": strings.Repeat("a", 40)}}}})
		case strings.Contains(r.URL.Path, "/pullRequests/") && !strings.HasSuffix(r.URL.Path, "/threads"):
			write(map[string]any{"pullRequestId": 17, "description": strings.Repeat("full PR description ", 40), "repository": map[string]any{"id": f.repoID, "project": map[string]string{"id": f.projectID}}, "sourceRefName": "refs/heads/source", "targetRefName": "refs/heads/main", "lastMergeSourceCommit": map[string]string{"commitId": strings.Repeat("a", 40)}})
		case strings.HasSuffix(r.URL.Path, "/threads"):
			if f.threadCursor {
				w.Header().Set("x-ms-continuationtoken", "unexpected")
			}
			write(map[string]any{"count": 1, "value": []any{map[string]any{"id": 5, "status": "active", "threadContext": map[string]any{"filePath": "/x.go", "rightFileStart": map[string]int{"line": 3, "offset": 1}}, "pullRequestThreadContext": map[string]any{"iterationContext": map[string]int{"firstComparingIteration": 1, "secondComparingIteration": 2}}, "comments": []any{map[string]any{"id": 1, "content": "inline source", "author": map[string]string{"displayName": "reviewer"}}, map[string]any{"id": 2, "parentCommentId": 1, "content": "reply source"}}}}})
		case r.URL.Path == "/denied":
			w.WriteHeader(403)
		case r.URL.Path == "/attachment":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("attachment bytes"))
		case strings.HasSuffix(r.URL.Path, "/items"):
			if r.URL.Query().Get("versionDescriptor.versionType") != "commit" {
				t.Error("unbound YAML")
			}
			if r.Header.Get("Accept") != "application/octet-stream" {
				write(map[string]string{"path": "/azure-pipelines.yml", "objectId": strings.Repeat("b", 40)})
			} else {
				_, _ = w.Write([]byte("steps:\n- script: never-execute\n"))
			}
		case strings.Contains(r.URL.Path, "/build/definitions/"):
			if f.definitionOverride != nil {
				_, _ = w.Write(f.definitionOverride)
				return
			}
			write(map[string]any{"id": 4, "revision": 2, "process": map[string]any{"type": 1, "phases": []any{map[string]string{"name": "inert classic"}}}, "variables": map[string]any{"secret": map[string]any{"isSecret": true, "value": "must-not-persist"}, "ordinary": map[string]any{"value": "retained"}}})
		default:
			t.Error("unexpected Azure request", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *azureFixture) config() migration.ProviderConfig {
	return migration.ProviderConfig{Locator: migration.Locator{Kind: "azure", Endpoint: f.server.URL, Organization: "org", Project: "P", Repository: "R"}, Token: "supplied-azure-secret", CredentialMode: "bearer", APIVersion: "services-7.1"}
}
func azureInputSnapshot() (migration.Snapshot, migration.BlobSource) {
	loc := migration.Locator{Kind: "fixture", Endpoint: "https://source.invalid", Project: "project"}
	key := migration.Key{Provider: "fixture", Instance: loc.Endpoint, ProjectID: "immutable-project", Kind: "work_item", ID: "8"}
	s := migration.Snapshot{Schema: "hn.project-snapshot/1", Source: loc, SourceID: "immutable-source", Records: []migration.Record{{Key: key, Revision: "12", Schema: "hn.migration.issue/1", Content: azureJSON(migration.IssueContent{Schema: "hn.migration.issue/1", Title: "source title", Body: "source body", Type: "Bug", State: "New", Labels: []string{}, Assignees: []string{}}), Links: []migration.Link{}, Blobs: []string{}}, {Key: migration.Key{Provider: key.Provider, Instance: key.Instance, ProjectID: key.ProjectID, Kind: "comment", ID: "9"}, Revision: "1", Schema: "hn.migration.comment/1", Content: azureJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: "source comment"}), Links: []migration.Link{{Relation: "work_item", Target: key}}, Blobs: []string{}}}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}
	return s, azureBlobs{}
}
func azureSelection(d migration.Destination) migration.Selection {
	return migration.Selection{Destination: d.Locator(), Mappings: map[string]string{"process": azureProcessID, "type:Bug": "Task", "state:New": "Active"}}
}
func TestAzureMigrationOfflineRegistrationAndSelectors(t *testing.T) {
	config := migration.ProviderConfig{Locator: migration.Locator{Kind: "azure", Endpoint: "https://offline.invalid", Organization: "org", Project: "Boards"}, APIVersion: "server-7.0", CredentialMode: "pat"}
	s, e := OpenMigrationSource(config)
	if e != nil {
		t.Fatal(e)
	}
	d, e := OpenMigrationDestination(config)
	if e != nil {
		t.Fatal(e)
	}
	sc, _ := s.Capabilities(context.Background())
	dc, _ := d.Capabilities(context.Background())
	p, ok := migrationProviders.Lookup("azure")
	if !ok {
		t.Fatal("missing registration")
	}
	if e = migration.ValidateProviderCoverage(p.RequiredCoverage, sc, dc); e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"org", "project", "profile", "mode"} {
		bad := config
		switch field {
		case "org":
			bad.Locator.Organization = ""
		case "project":
			bad.Locator.Project = ""
		case "profile":
			bad.APIVersion = "7.1"
		case "mode":
			bad.CredentialMode = ""
		}
		if _, e = OpenMigrationSource(bad); e == nil {
			t.Fatal("missing explicit field accepted", field)
		}
	}
}
func TestAzureMigrationExplicitProcessTypeStateAndCapacity(t *testing.T) {
	f := newAzureFixture(t)
	d, e := OpenMigrationDestination(f.config())
	if e != nil {
		t.Fatal(e)
	}
	s, _ := azureInputSnapshot()
	for _, name := range []string{"process", "invalid_process", "type", "invalid_type", "state", "invalid_state", "capacity"} {
		t.Run(name, func(t *testing.T) {
			sel := azureSelection(d)
			switch name {
			case "process":
				delete(sel.Mappings, "process")
			case "invalid_process":
				sel.Mappings["process"] = "44444444-4444-4444-8444-444444444444"
			case "type":
				delete(sel.Mappings, "type:Bug")
			case "invalid_type":
				sel.Mappings["type:Bug"] = "Absent"
			case "state":
				delete(sel.Mappings, "state:New")
			case "invalid_state":
				sel.Mappings["state:New"] = "Closed"
			case "capacity":
				var c migration.IssueContent
				_ = json.Unmarshal(s.Records[0].Content, &c)
				c.Title = strings.Repeat("x", 256)
				s.Records[0].Content = azureJSON(c)
			}
			ins, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			if len(ins) != 0 || len(fs) < 2 {
				t.Fatal("unsupported plan produced intents", ins, fs)
			}
			if f.writes != 0 {
				t.Fatal("planning mutated")
			}
		})
	}
}
func TestAzureMigrationEngineLostResponsesAndExactParent(t *testing.T) {
	for _, lost := range []string{"item", "comment"} {
		t.Run(lost, func(t *testing.T) {
			f := newAzureFixture(t)
			f.loseItem = lost == "item"
			f.loseComment = lost == "comment"
			ctx := context.Background()
			s, blobs := azureInputSnapshot()
			d, e := OpenMigrationDestination(f.config())
			if e != nil {
				t.Fatal(e)
			}
			session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), s, blobs, azureSelection(d), d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(ctx, session, d, ""); e == nil {
				t.Fatal("lost response accepted")
			}
			f.hideItem = lost == "item"
			f.hideComments = lost == "comment"
			fresh, _ := OpenMigrationDestination(f.config())
			restarted, e := OpenMigrationSession(ctx, session.Directory)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(ctx, restarted, fresh, ""); e == nil {
				t.Fatal("negative observation authorized retry")
			}
			if f.writes != 1 || f.commentWrites > 1 {
				t.Fatal("duplicate effects")
			}
			f.hideItem = false
			f.hideComments = false
			if e = ApplyMigration(ctx, restarted, fresh, ""); e != nil {
				t.Fatal(e)
			}
			if f.writes != 1 || f.commentWrites != 1 {
				t.Fatal("wrong write counts", f.writes, f.commentWrites)
			}
			raw, e := migrationUnseal(session.Directory, "progress.json", 1<<20)
			if e != nil {
				t.Fatal(e)
			}
			var progress migration.Progress
			_ = json.Unmarshal(raw, &progress)
			if len(progress.Entries) != 2 || progress.Entries[0].Receipts[0].RemoteID != azureRemoteID(azureProjectID, 71) || progress.Entries[0].Receipts[0].Revision != "1" || !strings.Contains(string(progress.Entries[1].Write.Payload), `"parent":{"id":"71"}`) {
				t.Fatal("parent/revision/identity not engine persisted", string(raw))
			}
			if strings.Contains(string(raw), "supplied-azure-secret") {
				t.Fatal("token persisted")
			}
			saved := string(progress.Entries[1].Write.Payload)
			if e = ApplyMigration(ctx, restarted, fresh, ""); e != nil {
				t.Fatal(e)
			}
			raw, _ = migrationUnseal(session.Directory, "progress.json", 1<<20)
			_ = json.Unmarshal(raw, &progress)
			if saved != string(progress.Entries[1].Write.Payload) {
				t.Fatal("resolved parent rewritten")
			}
		})
	}
}
func TestAzureMigrationSnapshotPreservesCustomFieldsAttachmentsAndPRTopology(t *testing.T) {
	f := newAzureFixture(t)
	f.item = azureFixtureItem(12, "observed", "description")
	f.comments = []azureapi.Comment{{ID: 6, WorkItemID: 12, Version: 4, Text: "observed comment", CreatedBy: azureapi.Identity{DisplayName: "foreign actor"}}}
	config := f.config()
	config.WorkflowRef = strings.Repeat("a", 40)
	config.PlanningSelectors = []string{"yaml:/azure-pipelines.yml", "definition:4@2"}
	config.Attachments = []migration.AttachmentSelection{{URL: f.server.URL + "/attachment", Name: "proof.txt", Parent: migration.Key{Provider: "azure", Instance: f.server.URL + "/org", ProjectID: azureProjectID, Kind: "work_item", ID: "12"}}}
	src, e := OpenMigrationSource(config)
	if e != nil {
		t.Fatal(e)
	}
	snapshot, blobs, e := src.Snapshot(context.Background(), migration.Selection{Source: config.Locator}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	combined := ""
	inlineLinks := 0
	for _, r := range snapshot.Records {
		counts[r.Key.Kind]++
		if r.Key.ProjectID != azureProjectID {
			t.Fatal("project identity conflated", r.Key)
		}
		if r.Key.Kind == "work_item" {
			if r.Revision != "1" || len(r.Blobs) != 2 {
				t.Fatal("work item/raw attachment missing", r)
			}
		}
		if r.Key.Kind == "inline_comment" {
			inlineLinks += len(r.Links)
		}
	}
	for _, b := range snapshot.Blobs {
		reader, e := blobs.Open(context.Background(), b.Digest)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(reader)
		reader.Close()
		combined += string(raw)
	}
	if counts["work_item"] != 1 || counts["comment"] != 1 || counts["change_request"] != 1 || counts["review"] != 1 || counts["inline_comment"] != 2 || counts["workflow"] != 2 || inlineLinks != 5 {
		t.Fatal("missing topology", counts, inlineLinks)
	}
	for _, value := range []string{"custom source", "AttachedFile", "attachment bytes", "rightFileStart", "firstComparingIteration", "sourceRefName", "never-execute", "inert classic", "foreign actor"} {
		if !strings.Contains(combined, value) {
			t.Fatal("retrieved content lost", value)
		}
	}
	if strings.Contains(combined, "must-not-persist") {
		t.Fatal("secret variable value preserved")
	}
	secondConfig := config
	secondConfig.Locator.Repository = "R2"
	secondSource, e := OpenMigrationSource(secondConfig)
	if e != nil {
		t.Fatal(e)
	}
	other, _, e := secondSource.Snapshot(context.Background(), migration.Selection{Source: secondConfig.Locator, Kinds: []string{"work_item"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if other.SourceID == snapshot.SourceID {
		t.Fatal("two repos in project collide")
	}
}
func TestAzureMigrationBoardOnlySnapshot(t *testing.T) {
	f := newAzureFixture(t)
	f.item = azureFixtureItem(12, "Boards task", "body")
	config := f.config()
	config.Locator.Repository = ""
	src, e := OpenMigrationSource(config)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := src.Snapshot(context.Background(), migration.Selection{Source: config.Locator, Kinds: []string{"work_item", "comment"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Records) != 1 || s.Records[0].Key.ProjectID != azureProjectID {
		t.Fatal(s)
	}
}
func TestAzureMigrationImmutableReplacementAndRevisionGuard(t *testing.T) {
	for _, change := range []string{"project", "repository", "process", "revision", "conflict"} {
		t.Run(change, func(t *testing.T) {
			f := newAzureFixture(t)
			f.item = azureFixtureItem(71, "existing", "body")
			s, blobs := azureInputSnapshot()
			s.Records = s.Records[:1]
			d, _ := OpenMigrationDestination(f.config())
			sel := azureSelection(d)
			sel.Mappings["target:"+s.Records[0].Key.String()] = "71"
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			switch change {
			case "project":
				f.projectID = "44444444-4444-4444-8444-444444444444"
			case "repository":
				f.repoID = "44444444-4444-4444-8444-444444444444"
			case "process":
				f.processID = "44444444-4444-4444-8444-444444444444"
			case "revision":
				f.item.Rev++
			case "conflict":
				f.conflict = true
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("stale/revision conflict accepted")
			}
			if f.writes != 0 {
				t.Fatal("write after stale baseline")
			}
		})
	}
}
func TestAzureMigrationAmbiguousReconciliationNeverDuplicates(t *testing.T) {
	f := newAzureFixture(t)
	f.loseItem = true
	s, blobs := azureInputSnapshot()
	d, _ := OpenMigrationDestination(f.config())
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, azureSelection(d), d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
		t.Fatal("lost response accepted")
	}
	f.duplicate = true
	for i := 0; i < 2; i++ {
		if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
			t.Fatal("ambiguous recovery accepted")
		}
	}
	if f.writes != 1 || f.commentWrites != 0 {
		t.Fatal("duplicate effects")
	}
}
func TestAzureMigrationThreadCursorRefusesIncompleteSnapshot(t *testing.T) {
	f := newAzureFixture(t)
	f.threadCursor = true
	config := f.config()
	src, _ := OpenMigrationSource(config)
	_, _, e := src.Snapshot(context.Background(), migration.Selection{Source: config.Locator, Kinds: []string{"change_request"}}, migration.DefaultLimits())
	if e == nil {
		t.Fatal("unexpected thread cursor accepted")
	}
}

func azureAcknowledge(findings []migration.Finding) []migration.Decision {
	ds := []migration.Decision{}
	for _, f := range findings {
		if f.Status != "mapped" && f.Status != "preserved" {
			ds = append(ds, migration.Decision{FindingID: f.ID, Action: "preserve", Mapping: json.RawMessage(`{}`)})
		}
	}
	return ds
}
func TestAzureMigrationSharedGitCompanionAndSecretFreeArchive(t *testing.T) {
	root := inReplicationTestRepository(t)
	f := newAzureFixture(t)
	f.item = azureFixtureItem(12, "source task", "source body")
	f.comments = []azureapi.Comment{{ID: 6, WorkItemID: 12, Version: 3, Text: "source comment"}}
	apiSource, e := OpenMigrationSource(f.config())
	if e != nil {
		t.Fatal(e)
	}
	targetFixture := newAzureFixture(t)
	apiTarget, e := OpenMigrationDestination(targetFixture.config())
	if e != nil {
		t.Fatal(e)
	}
	capture := ComposeMigrationSources(apiSource, LocalMigrationSource{GitDir: filepath.Join(root, ".git")})
	sel := migration.Selection{Source: f.config().Locator, Destination: apiTarget.Locator(), Kinds: []string{"work_item", "comment"}, Refs: []string{"refs/heads/main"}, Mappings: map[string]string{"process": azureProcessID, "type:Task": "Task", "state:Active": "Active"}}
	ctx := context.Background()
	snapshot, blobs, e := capture.Snapshot(ctx, sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer blobs.(io.Closer).Close()
	if len(snapshot.NativeRefs) != 1 || len(snapshot.Records) != 2 {
		t.Fatal("code/collaboration selection lost", snapshot)
	}
	var archive bytes.Buffer
	if e = migration.WriteArchive(ctx, &archive, snapshot, blobs, migration.DefaultLimits()); e != nil {
		t.Fatal(e)
	}
	verified, e := migration.ReadArchive(ctx, bytes.NewReader(archive.Bytes()), migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer verified.Close()
	for _, b := range snapshot.Blobs {
		r, e := verified.Open(ctx, b.Digest)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(r)
		r.Close()
		if bytes.Contains(raw, []byte(f.config().Token)) {
			t.Fatal("credential exported")
		}
	}
	remote := migrationBare(t)
	target := ComposeMigrationDestinations(apiTarget, GitMigrationDestination{Remote: remote, Destination: apiTarget.Locator(), Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}})
	_, fs, e := target.Plan(ctx, snapshot, sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	decisions := azureAcknowledge(append(snapshot.Findings, fs...))
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), snapshot, blobs, sel, target, migration.DefaultLimits(), decisions)
	if e != nil {
		t.Fatal(e)
	}
	planRaw, _ := json.Marshal(session.Plan)
	if bytes.Contains(planRaw, []byte(f.config().Token)) {
		t.Fatal("credential saved")
	}
	if e = ApplyMigration(ctx, session, target, ""); e != nil {
		t.Fatal(e)
	}
	head := mustGitText(t, "rev-parse", "HEAD")
	got, e := projectGit(ctx, remote, nil, 256, "show-ref", "--verify", "--hash", "refs/heads/main")
	if e != nil || strings.TrimSpace(string(got)) != head {
		t.Fatal("selected Git code not transported", e)
	}
	if f.writes != 0 || targetFixture.writes != 1 || targetFixture.commentWrites != 1 {
		t.Fatal("wrong provider effects")
	}
}
func TestAzureMigrationMarkerCapacityExactBoundary(t *testing.T) {
	for _, kind := range []string{"work_item", "comment"} {
		t.Run(kind, func(t *testing.T) {
			f := newAzureFixture(t)
			s, blobs := azureInputSnapshot()
			d, _ := OpenMigrationDestination(f.config())
			sel := azureSelection(d)
			ins, _, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			index := 0
			if kind == "comment" {
				index = 1
			}
			budget := azureBodyBudget(ins[index].ID)
			if kind == "work_item" {
				var c migration.IssueContent
				_ = json.Unmarshal(s.Records[0].Content, &c)
				c.Body = strings.Repeat("x", budget)
				s.Records[0].Content = azureJSON(c)
			} else {
				s.Records[1].Content = azureJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: strings.Repeat("x", budget)})
			}
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
				t.Fatal("exact capacity refused", e)
			}
			recordIndex := 0
			for i, r := range s.Records {
				if r.Key.Kind == kind {
					recordIndex = i
				}
			}
			if kind == "work_item" {
				if len(f.item.Field("System.Description")) != 65536 {
					t.Fatal("wrong description budget")
				}
				var c migration.IssueContent
				_ = json.Unmarshal(s.Records[recordIndex].Content, &c)
				c.Body += "x"
				s.Records[recordIndex].Content = azureJSON(c)
			} else {
				if len(f.comments[0].Text) != 65536 {
					t.Fatal("wrong comment budget")
				}
				s.Records[recordIndex].Content = azureJSON(migration.CommentContent{Schema: "hn.migration.comment/1", Body: strings.Repeat("x", budget+1)})
			}
			ins, fs, e := d.Plan(context.Background(), s, sel, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			expected := 0
			if kind == "comment" {
				expected = 1
			}
			if len(ins) != expected {
				t.Fatal("excess not preflighted", len(ins), fs)
			}
		})
	}
}
func TestAzureMigrationSnapshotBudgetsExactAndExcess(t *testing.T) {
	f := newAzureFixture(t)
	f.item = azureFixtureItem(12, "task", "body")
	config := f.config()
	config.Locator.Repository = ""
	src, _ := OpenMigrationSource(config)
	sel := migration.Selection{Source: config.Locator, Kinds: []string{"work_item", "comment"}}
	initial, _, e := src.Snapshot(context.Background(), sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	limits := migration.DefaultLimits()
	limits.Records = 1
	limits.Blobs = 1
	limits.BlobBytes = initial.Blobs[0].Size
	limits.AttachmentBytes = initial.Blobs[0].Size
	if _, _, e = src.Snapshot(context.Background(), sel, limits); e != nil {
		t.Fatal("exact snapshot budget refused", e)
	}
	limits.BlobBytes--
	if _, _, e = src.Snapshot(context.Background(), sel, limits); e == nil {
		t.Fatal("excess blob accepted")
	}
	config.Locator.Repository = ""
	src, _ = OpenMigrationSource(config)
	sel.Refs = []string{"refs/heads/main"}
	if _, _, e = src.Snapshot(context.Background(), sel, migration.DefaultLimits()); e == nil {
		t.Fatal("repository-free code selection accepted")
	}
}
func TestAzureMigrationResumeCannotChangeAPIProfileOrParentProject(t *testing.T) {
	for _, change := range []string{"profile", "parent"} {
		t.Run(change, func(t *testing.T) {
			f := newAzureFixture(t)
			f.loseComment = true
			s, blobs := azureInputSnapshot()
			d, _ := OpenMigrationDestination(f.config())
			session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, azureSelection(d), d, migration.DefaultLimits(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if e = ApplyMigration(context.Background(), session, d, ""); e == nil {
				t.Fatal("lost comment response accepted")
			}
			cfg := f.config()
			if change == "profile" {
				cfg.APIVersion = "server-7.0"
				f.apiVersion = "7.0"
				f.commentsVersion = "7.0-preview.3"
			} else {
				f.item.Fields["System.TeamProject"] = azureJSON("other project")
			}
			fresh, _ := OpenMigrationDestination(cfg)
			if e = ApplyMigration(context.Background(), session, fresh, ""); e == nil {
				t.Fatal("changed wire/parent baseline accepted")
			}
			if f.writes != 1 || f.commentWrites != 1 {
				t.Fatal("duplicate effect")
			}
		})
	}
}
func TestAzureMigrationServerPATReadsAndWrites(t *testing.T) {
	f := newAzureFixture(t)
	f.apiVersion = "7.0"
	f.commentsVersion = "7.0-preview.3"
	f.pat = true
	cfg := f.config()
	cfg.APIVersion = "server-7.0"
	cfg.CredentialMode = "pat"
	cfg.Locator.Repository = ""
	d, e := OpenMigrationDestination(cfg)
	if e != nil {
		t.Fatal(e)
	}
	s, blobs := azureInputSnapshot()
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, azureSelection(d), d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
		t.Fatal(e)
	}
	src, _ := OpenMigrationSource(cfg)
	observed, _, e := src.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"work_item", "comment"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if len(observed.Records) != 2 || f.writes != 1 || f.commentWrites != 1 {
		t.Fatal("Server mapped baseline missing")
	}
}

func TestAzureMigrationUpdateSucceedsWithExactRevisionGuard(t *testing.T) {
	f := newAzureFixture(t)
	f.item = azureFixtureItem(71, "existing", "existing body")
	s, blobs := azureInputSnapshot()
	s.Records = s.Records[:1]
	d, _ := OpenMigrationDestination(f.config())
	sel := azureSelection(d)
	sel.Mappings["target:"+s.Records[0].Key.String()] = "71"
	session, e := PrepareMigration(context.Background(), filepath.Join(t.TempDir(), "s"), s, blobs, sel, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(context.Background(), session, d, ""); e != nil {
		t.Fatal(e)
	}
	if f.writes != 1 || f.item.Rev != 2 || f.item.Field("System.Title") != "source title" {
		t.Fatal("guarded update missing")
	}
}
func TestAzureMigrationAttachmentAuthorizationAndUnknownKindAreHonest(t *testing.T) {
	f := newAzureFixture(t)
	f.item = azureFixtureItem(12, "task", "body")
	config := f.config()
	config.Attachments = []migration.AttachmentSelection{{URL: f.server.URL + "/denied", Name: "denied", Parent: migration.Key{Provider: "azure", Instance: f.server.URL + "/org", ProjectID: azureProjectID, Kind: "work_item", ID: "12"}}}
	src, _ := OpenMigrationSource(config)
	s, _, e := src.Snapshot(context.Background(), migration.Selection{Source: config.Locator, Kinds: []string{"work_item", "discussion"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	missing, unsupported := false, false
	for _, f := range s.Findings {
		missing = missing || f.Status == "missing_dependency" && strings.Contains(f.ID, "attachment")
		unsupported = unsupported || f.Status == "unsupported" && strings.Contains(f.ID, "discussion")
	}
	if !missing || !unsupported || len(s.Blobs) != 1 {
		t.Fatal("unread attachment or kind called preserved", s.Findings, s.Blobs)
	}
}

func TestAzureMigrationSelectedProjectPreservesCompleteRetrievedMetadata(t *testing.T) {
	f := newAzureFixture(t)
	config := f.config()
	config.Locator.Repository = ""
	src, e := OpenMigrationSource(config)
	if e != nil {
		t.Fatal(e)
	}
	s, blobs, e := src.Snapshot(context.Background(), migration.Selection{Source: config.Locator, Kinds: []string{"project"}}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Records) != 1 || len(s.Records[0].Blobs) != 1 {
		t.Fatal("selected project missing")
	}
	r, e := blobs.Open(context.Background(), s.Records[0].Blobs[0])
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	raw, _ := io.ReadAll(r)
	if !strings.Contains(string(raw), "full planning description") || !strings.Contains(string(raw), `"visibility":"private"`) {
		t.Fatal("retrieved project fields lost", string(raw))
	}
}

func TestAzureMigrationClassicServiceManagedCredentialsExcluded(t *testing.T) {
	f := newAzureFixture(t)
	cfg := f.config()
	cfg.Locator.Repository = ""
	cfg.PlanningSelectors = []string{"definition:4@2"}
	src, e := OpenMigrationSource(cfg)
	if e != nil {
		t.Fatal(e)
	}
	definition := func(secret string) json.RawMessage {
		return azureJSON(map[string]any{"id": 4, "revision": 2, "process": map[string]int{"type": 1}, "properties": map[string]any{"large": int64(9007199254740993)}, "variables": map[string]any{"secret": map[string]any{"isSecret": true, "value": secret + "-top"}, "ordinary": map[string]any{"isSecret": false, "value": "ordinary-retained"}}, "variableGroups": []any{map[string]any{"id": 9, "name": "group-retained", "variables": map[string]any{"secret": map[string]any{"isSecret": true, "value": secret + "-group"}, "ordinary": map[string]any{"value": "group-ordinary-retained"}}}}, "processParameters": map[string]any{"dataSourceBindings": []any{map[string]any{"endpointId": "endpoint-reference-retained", "headers": []any{map[string]any{"name": "Authorization", "value": secret + "-header"}}}}}})
	}
	var firstDigest string
	for _, secret := range []string{"private-value-first", "private-value-second-longer"} {
		f.definitionOverride = definition(secret)
		s, blobs, e := src.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"workflow"}}, migration.DefaultLimits())
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Records) != 1 || len(s.Blobs) != 1 {
			t.Fatal("selected classic missing", s)
		}
		r, e := blobs.Open(context.Background(), s.Blobs[0].Digest)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(r)
		r.Close()
		for _, text := range []string{secret, "-top", "-group", "-header"} {
			if strings.Contains(string(raw), text) {
				t.Fatal("credential leaked", text, string(raw))
			}
		}
		for _, text := range []string{"ordinary-retained", "group-ordinary-retained", "endpoint-reference-retained", "Authorization", "group-retained", "9007199254740993"} {
			if !strings.Contains(string(raw), text) {
				t.Fatal("nonsecret source metadata lost", text)
			}
		}
		excluded := false
		for _, finding := range s.Findings {
			if finding.Status == "unsupported" && strings.Contains(finding.Message, "credential values excluded") {
				excluded = true
			}
		}
		if !excluded {
			t.Fatal("missing explicit credential exclusion finding", s.Findings)
		}
		canonical, e := migration.CanonicalSnapshot(s, migration.DefaultLimits())
		if e != nil {
			t.Fatal(e)
		}
		digest := migration.Digest(canonical)
		if firstDigest == "" {
			firstDigest = digest
		} else if firstDigest != digest {
			t.Fatal("original credential value fingerprint persisted")
		}
		if s.Records[0].Revision != migration.Digest(raw) || s.Records[0].Blobs[0] != migration.Digest(raw) {
			t.Fatal("evidence does not bind sanitized bytes")
		}
		var archive bytes.Buffer
		if e = migration.WriteArchive(context.Background(), &archive, s, blobs, migration.DefaultLimits()); e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(archive.Bytes(), []byte(secret)) {
			t.Fatal("archive leaked credentials")
		}
	}
}
func TestAzureMigrationClassicMalformedCredentialDeclarationsRefuse(t *testing.T) {
	for _, fields := range []string{
		`"variables":{"x":{"isSecret":"true","value":"private"}}`,
		`"variables":{"x":{"isSecret":null,"value":"private"}}`,
		`"variables":{"x":{"isSecret":1,"value":"private"}}`,
		`"variables":{"x":{"isSecret":true,"isSecret":false,"value":"private"}}`,
		`"variables":{"x":{"IsSecret":true,"value":"private"}}`,
		`"Variables":{"x":{"isSecret":true,"value":"private"}}`,
		`"variables":{"x":{"isSecret":true,"Value":"private"}}`,
		`"variables":{"x":{"isSecret":false,"value":"public","value":"private"}}`,
		`"variableGroups":[{"id":9,"variables":{"x":{"isSecret":null,"value":"private"}}}]`,
		`"processParameters":{"dataSourceBindings":[{"headers":[{"name":"Authorization","Value":"private"}]}]}`,
	} {
		t.Run(strconv.Itoa(len(fields))+fields[:12], func(t *testing.T) {
			f := newAzureFixture(t)
			f.definitionOverride = json.RawMessage(`{"id":4,"revision":2,"process":{"type":1},` + fields + `}`)
			cfg := f.config()
			cfg.Locator.Repository = ""
			cfg.PlanningSelectors = []string{"definition:4@2"}
			src, _ := OpenMigrationSource(cfg)
			if _, _, e := src.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"workflow"}}, migration.DefaultLimits()); e == nil {
				t.Fatal("ambiguous credential declaration copied", fields)
			}
			if f.writes != 0 || f.commentWrites != 0 {
				t.Fatal("malformed source caused effects")
			}
		})
	}
}
