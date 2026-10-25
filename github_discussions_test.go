//go:build !nogithub

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hubnot/internal/forge"
	"hubnot/internal/githubapi"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubDiscussionGraphPreservesIndependentAnswerAndClaims(t *testing.T) {
	count := 0
	d := githubapi.Discussion{ID: "D1", Number: 7, Title: "Question", Body: "body", Author: "alice", URL: "https://github.com/o/r/discussions/7", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z", Category: githubapi.DiscussionCategory{ID: "C1", Slug: "help", Name: "Help", IsAnswerable: true}, AnswerID: "A", EditCount: &count, Comments: []githubapi.DiscussionComment{{ID: "A", DiscussionID: "D1", Body: "answer", IsAnswer: true, Author: "bob", URL: "https://github.com/o/r/discussions/7#A", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z", EditCount: &count}, {ID: "B", DiscussionID: "D1", ParentID: "A", Body: "nested", URL: "https://github.com/o/r/discussions/7#B", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z", EditCount: &count}}}
	g, err := githubDiscussionGraph([]githubapi.Discussion{d}, forge.Selection{Mappings: map[string]string{"history": "snapshot"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 3 || g.Nodes[0].Discussion.AcceptedReply.ID != "A" || g.Nodes[2].Reply.Parent.ID != "A" || g.Nodes[1].Record.Author != "bob" {
		t.Fatalf("lost typed data %+v", g)
	}
	if _, err := githubDiscussionGraph([]githubapi.Discussion{d}, forge.Selection{}); err == nil {
		t.Fatal("inaccessible history silently lost")
	}
	d.Comments[1].DiscussionID = "OTHER"
	if _, err := githubDiscussionGraph([]githubapi.Discussion{d}, forge.Selection{Mappings: map[string]string{"history": "snapshot"}}); err == nil {
		t.Fatal("cross-thread reply accepted")
	}
}

func TestGitHubDiscussionPublicRoutingIsInstalled(t *testing.T) {
	if !forgeResourceInstalled(transferOptions{provider: "github", resource: forge.ResourceDiscussion, command: "export"}) {
		t.Fatal("exporter absent")
	}
	var _ forge.DiscussionReader = (*githubForge)(nil)
	_ = context.Background()
}

type githubDiscussionFixture struct {
	d                                  githubapi.Discussion
	writes, createRoots, createReplies int
	lose                               string
	server                             *httptest.Server
}

func discussionAPICategory() map[string]any {
	return map[string]any{"id": "C1", "name": "Help", "slug": "help", "description": "help", "emoji": "?", "isAnswerable": true}
}
func discussionAPIConnection(nodes []any) map[string]any {
	return map[string]any{"nodes": nodes, "pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil}}
}
func discussionAPINode(d githubapi.Discussion) map[string]any {
	var answer any
	if d.AnswerID != "" {
		answer = map[string]any{"id": d.AnswerID}
	}
	return map[string]any{"__typename": "Discussion", "id": d.ID, "number": 7, "repository": map[string]any{"id": "R1"}, "title": d.Title, "body": d.Body, "url": "https://github.com/o/r/discussions/7", "author": map[string]any{"login": "alice"}, "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-02T00:00:00Z", "category": discussionAPICategory(), "closed": d.Closed, "locked": d.Locked, "answer": answer, "poll": nil, "reactions": map[string]any{"totalCount": d.ReactionCount}, "upvoteCount": 0, "userContentEdits": map[string]any{"totalCount": 0}}
}
func discussionAPIReply(r githubapi.DiscussionComment, answer string) map[string]any {
	var parent any
	if r.ParentID != "" {
		parent = map[string]any{"id": r.ParentID}
	}
	return map[string]any{"__typename": "DiscussionComment", "id": r.ID, "discussion": map[string]any{"id": "D1"}, "replyTo": parent, "body": r.Body, "url": "https://github.com/o/r/discussions/7#" + r.ID, "author": map[string]any{"login": "bob"}, "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-02T00:00:00Z", "isAnswer": r.ID == answer, "isMinimized": false, "reactions": map[string]any{"totalCount": 0}, "upvoteCount": 0, "userContentEdits": map[string]any{"totalCount": 0}}
}
func newDiscussionFixture(t *testing.T) *githubDiscussionFixture {
	t.Helper()
	f := &githubDiscussionFixture{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Query     string
			Variables map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		var data any
		input, _ := q.Variables["input"].(map[string]any)
		str := func(k string) string { v, _ := input[k].(string); return v }
		mutation := strings.HasPrefix(q.Query, "mutation ")
		switch {
		case strings.Contains(q.Query, "HNDiscussionRepository"):
			data = map[string]any{"repository": map[string]any{"id": "R1", "nameWithOwner": "o/r", "hasDiscussionsEnabled": true}}
		case strings.Contains(q.Query, "HNDiscussionCategories"):
			data = map[string]any{"repository": map[string]any{"id": "R1", "discussionCategories": discussionAPIConnection([]any{discussionAPICategory()})}}
		case strings.Contains(q.Query, "HNDiscussions("):
			rows := []any{}
			if f.d.ID != "" {
				rows = append(rows, discussionAPINode(f.d))
			}
			data = map[string]any{"repository": map[string]any{"id": "R1", "discussions": discussionAPIConnection(rows)}}
		case strings.Contains(q.Query, "HNDiscussionNumber"):
			data = map[string]any{"repository": map[string]any{"discussion": discussionAPINode(f.d)}}
		case strings.Contains(q.Query, "HNDiscussionID"):
			data = map[string]any{"node": discussionAPINode(f.d)}
		case strings.Contains(q.Query, "HNDiscussionComments"):
			rows := []any{}
			for _, v := range f.d.Comments {
				if v.ParentID == "" {
					rows = append(rows, discussionAPIReply(v, f.d.AnswerID))
				}
			}
			data = map[string]any{"node": map[string]any{"__typename": "Discussion", "id": "D1", "comments": discussionAPIConnection(rows)}}
		case strings.Contains(q.Query, "HNDiscussionReplies"):
			id, _ := q.Variables["id"].(string)
			rows := []any{}
			for _, v := range f.d.Comments {
				if v.ParentID == id {
					rows = append(rows, discussionAPIReply(v, f.d.AnswerID))
				}
			}
			data = map[string]any{"node": map[string]any{"__typename": "DiscussionComment", "id": id, "replies": discussionAPIConnection(rows)}}
		case strings.Contains(q.Query, "HNCreateDiscussion"):
			f.createRoots++
			f.d.ID = "D1"
			f.d.Title = str("title")
			f.d.Body = str("body")
			data = map[string]any{"createDiscussion": map[string]any{"discussion": discussionAPINode(f.d)}}
		case strings.Contains(q.Query, "HNUpdateDiscussion("):
			f.d.Title = str("title")
			f.d.Body = str("body")
			data = map[string]any{"updateDiscussion": map[string]any{"discussion": discussionAPINode(f.d)}}
		case strings.Contains(q.Query, "HNAddDiscussionComment"):
			f.createReplies++
			v := githubapi.DiscussionComment{ID: fmt.Sprintf("A%d", f.createReplies), DiscussionID: "D1", Body: str("body"), ParentID: str("replyToId")}
			f.d.Comments = append(f.d.Comments, v)
			data = map[string]any{"addDiscussionComment": map[string]any{"comment": discussionAPIReply(v, f.d.AnswerID)}}
		case strings.Contains(q.Query, "HNUpdateDiscussionComment"):
			for i := range f.d.Comments {
				if f.d.Comments[i].ID == str("commentId") {
					f.d.Comments[i].Body = str("body")
					data = map[string]any{"updateDiscussionComment": map[string]any{"comment": discussionAPIReply(f.d.Comments[i], f.d.AnswerID)}}
				}
			}
		case strings.Contains(q.Query, "HNUnmarkDiscussionAnswer"):
			f.d.AnswerID = ""
			data = map[string]any{"unmarkDiscussionCommentAsAnswer": map[string]any{"discussion": discussionAPINode(f.d)}}
		case strings.Contains(q.Query, "HNMarkDiscussionAnswer"):
			f.d.AnswerID = str("id")
			data = map[string]any{"markDiscussionCommentAsAnswer": map[string]any{"discussion": discussionAPINode(f.d)}}
		case strings.Contains(q.Query, "HNCloseDiscussion"):
			f.d.Closed = true
			n := discussionAPINode(f.d)
			n["stateReason"] = "RESOLVED"
			data = map[string]any{"closeDiscussion": map[string]any{"discussion": n}}
		case strings.Contains(q.Query, "HNReopenDiscussion"):
			f.d.Closed = false
			data = map[string]any{"reopenDiscussion": map[string]any{"discussion": discussionAPINode(f.d)}}
		default:
			t.Errorf("unexpected GraphQL document: %s", q.Query)
			http.Error(w, "bad query", 500)
			return
		}
		if mutation {
			f.writes++
			if f.lose != "" && strings.Contains(q.Query, f.lose) {
				f.lose = ""
				http.Error(w, "response lost", 500)
				return
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(f.server.Close)
	return f
}
func discussionTransferArgs(f *githubDiscussionFixture, command, id string) []string {
	a := []string{"transfer", command}
	if id != "" {
		a = append(a, id)
	}
	return append(a, "--provider", "github", "--project", "o/r", "--resource", "discussion", "--endpoint", f.server.URL, "--map", "history=snapshot")
}
func TestGitHubDiscussionNativeExportRecoveryAndConflicts(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	root := discussionWrite(t, "discussion", "open", "--title", "Native question", "--body", "Details", "--category", "help")
	reply := discussionWrite(t, "discussion", "reply", "add", root.ResourceID, "--body", "Answer")
	discussionWrite(t, "discussion", "reply", "add", root.ResourceID, "--reply-to", reply.ResourceID, "--body", "Nested")
	answered := discussionWrite(t, "discussion", "answer", root.ResourceID, "--expect", root.EventID, "--reply", reply.EventID)
	discussionWrite(t, "discussion", "close", root.ResourceID, "--expect", answered.EventID)
	args := append(discussionTransferArgs(f, "export", root.ResourceID), "--map", "category=C1")
	f.lose = "HNAddDiscussionComment"
	if err := run(args); err == nil {
		t.Fatal("lost reply response was not uncertain")
	}
	if f.createRoots != 1 || f.createReplies != 1 {
		t.Fatal("wrong initial mutation sequence")
	}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	if f.createRoots != 1 || f.createReplies != 2 || f.d.AnswerID == "" || !f.d.Closed {
		t.Fatalf("incomplete recovery %+v", f)
	}
	before := f.writes
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	if f.writes != before {
		t.Fatal("repeated export wrote duplicates")
	}
	if err := run(append(discussionTransferArgs(f, "import", ""), "--key", "7")); err == nil {
		t.Fatal("own native publication imported as duplicate")
	}
	t.Run("unrelated-native-destination", func(t *testing.T) {
		issueCommandTestRepo(t)
		if err := run(append(discussionTransferArgs(f, "import", ""), "--key", "7")); err != nil {
			t.Fatal(err)
		}
		if len(bridgeFamily(bridgeCatalog(t), ResourceDiscussion)) != 1 {
			t.Fatal("unrelated destination could not import source text")
		}
	})
	f.d.Comments[0].Body = "remote edit"
	if err := run(args); err == nil {
		t.Fatal("lost association silently adopted")
	}
	if f.writes != before {
		t.Fatal("external edit overwritten")
	}
}
func TestGitHubDiscussionImportAndSameSourceReturn(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	f.d = githubapi.Discussion{ID: "D1", Title: "Foreign question", Body: "Source body", AnswerID: "A1", Comments: []githubapi.DiscussionComment{{ID: "A1", DiscussionID: "D1", Body: "Source answer"}, {ID: "B1", DiscussionID: "D1", ParentID: "A1", Body: "Source nested"}}}
	args := append(discussionTransferArgs(f, "import", ""), "--key", "7")
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	cat := bridgeCatalog(t)
	threads := bridgeFamily(cat, ResourceDiscussion)
	if len(threads) != 1 || len(bridgeFamily(cat, ResourceDiscussionReply)) != 2 {
		t.Fatal("typed import missing")
	}
	before := len(cat.IDs)
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	if len(bridgeCatalog(t).IDs) != before {
		t.Fatal("import duplicated")
	}
	export := append(discussionTransferArgs(f, "export", threads[0].ID), "--map", "category=C1")
	if err := run(export); err != nil {
		t.Fatal(err)
	}
	if f.writes != 0 || f.d.Body != "Source body" || f.d.Comments[0].Body != "Source answer" {
		t.Fatal("same-source echo")
	}
	var d DiscussionState
	DecodeResourceState(ResourceDiscussion, threads[0].State, &d)
	edited := discussionWrite(t, "discussion", "revise", threads[0].ID, "--expect", threads[0].Heads[0].ID, "--body", "Native changed")
	_ = edited
	if err := run(export); err != nil {
		t.Fatal(err)
	}
	if f.d.Body != "Native changed" || f.createRoots != 0 || f.createReplies != 0 {
		t.Fatal("same-source update duplicated")
	}
}
func TestGitHubDiscussionPreflightHistoryCategoryAndHistoricalAnswer(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	root := discussionWrite(t, "discussion", "open", "--title", "Q", "--body", "D", "--category", "help")
	reply := discussionWrite(t, "discussion", "reply", "add", root.ResourceID, "--body", "Old answer")
	discussionWrite(t, "discussion", "answer", root.ResourceID, "--expect", root.EventID, "--reply", reply.EventID)
	args := discussionTransferArgs(f, "export", root.ResourceID)
	if err := run(args); err == nil {
		t.Fatal("category omission allowed")
	}
	discussionWrite(t, "discussion", "reply", "edit", reply.ResourceID, "--expect", reply.EventID, "--body", "Changed answer")
	args = append(args, "--map", "category=C1")
	if err := run(args); err == nil {
		t.Fatal("historical native answer silently moved")
	}
	if f.writes != 0 {
		t.Fatal("preflight wrote")
	}
	dir, _ := forgeStateDir(false)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "discussion-export-") {
			t.Fatal("failed preflight created journal", filepath.Join(dir, e.Name()))
		}
	}
}

func TestGitHubDiscussionUnknownCreateDoesNotRetryAndTamperedPlanRefuses(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	root := discussionWrite(t, "discussion", "open", "--title", "Q", "--body", "body", "--category", "help")
	args := append(discussionTransferArgs(f, "export", root.ResourceID), "--map", "category=C1")
	f.lose = "HNCreateDiscussion"
	if err := run(args); err == nil {
		t.Fatal("lost create accepted")
	}
	if f.createRoots != 1 {
		t.Fatal("create not attempted")
	}
	saved := f.d
	f.d = githubapi.Discussion{}
	if err := run(args); err == nil {
		t.Fatal("negative scan permitted creation retry")
	}
	if f.createRoots != 1 {
		t.Fatal("unknown creation retried")
	}
	o := transferOptions{project: "o/r", id: root.ResourceID}
	scope := githubDiscussionScope(f.server.URL, o)
	name := "discussion-export-" + strings.TrimPrefix(scope, "sha256:") + ".json"
	var cp githubDiscussionCheckpoint
	found, err := forgeLoadJSON(name, &cp)
	if err != nil || !found {
		t.Fatal("missing checkpoint", err, name)
	}
	cp.Actions[0].Desired.RepositoryID = "OTHER_REPOSITORY"
	if err := forgeStoreJSON(name, &cp); err != nil {
		t.Fatal(err)
	}
	f.d = saved
	if err := run(args); err == nil {
		t.Fatal("tampered destination adopted")
	}
	if f.createRoots != 1 {
		t.Fatal("tampered checkpoint wrote")
	}
}
func TestGitHubDiscussionLossAcknowledgementAndMarkerProse(t *testing.T) {
	n := 0
	d := githubapi.Discussion{ID: "D", Number: 1, Title: "Q", Body: "ordinary <!-- hn.discussion/1:hello --> prose", Category: githubapi.DiscussionCategory{Slug: "help"}, EditCount: &n, ReactionCount: 2}
	s := forge.Selection{Mappings: map[string]string{"history": "snapshot"}}
	if _, err := githubDiscussionGraph([]githubapi.Discussion{d}, s); err == nil {
		t.Fatal("votes silently dropped")
	}
	s.Mappings["losses"] = "acknowledge"
	g, err := githubDiscussionGraph([]githubapi.Discussion{d}, s)
	if err != nil || g.Nodes[0].Discussion.Body != d.Body {
		t.Fatal("ordinary prose altered", err)
	}
	if _, ok := githubDiscussionParseProof(d.Body); ok {
		t.Fatal("ordinary marker-like prose trusted")
	}
}
func TestGitHubDiscussionWholeCheckpointBudget(t *testing.T) {
	n := forge.Node{Kind: ResourceDiscussion, Record: forge.Record{ID: eventID([]byte("id")), Revision: eventID([]byte("rev"))}, Discussion: &forge.Discussion{Title: "Q", Body: strings.Repeat("<", 1<<20), Category: "help", Status: "open"}}
	snapshot := forgeResourceSnapshot{NativeID: n.Record.ID, Graph: forge.Graph{Nodes: []forge.Node{n}}}
	cp := githubDiscussionCheckpoint{Endpoint: "https://api.github.com/", CategoryID: "C", Snapshot: &snapshot, Previous: &snapshot, Actions: []githubDiscussionAction{{NativeID: n.Record.ID, Action: "create_root", Desired: githubDiscussionDesired(n, "https://api.github.com/", "C", false)}}}
	if err := githubDiscussionBudget(cp, map[string]forge.Node{n.Record.ID: n}); err == nil {
		t.Fatal("whole future journal overflow accepted")
	}
}
func TestGitHubDiscussionSourceReplyDeletionPreserved(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	f.d = githubapi.Discussion{ID: "D1", Title: "Q", Body: "Body", Comments: []githubapi.DiscussionComment{{ID: "A1", DiscussionID: "D1", Body: "Reply"}}}
	if err := run(append(discussionTransferArgs(f, "import", ""), "--key", "7")); err != nil {
		t.Fatal(err)
	}
	threads := bridgeFamily(bridgeCatalog(t), ResourceDiscussion)
	f.d.Comments = nil
	args := append(discussionTransferArgs(f, "export", threads[0].ID), "--map", "category=C1")
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	if f.writes != 0 || len(f.d.Comments) != 0 {
		t.Fatal("deleted source reply resurrected")
	}
}

func TestGitHubDiscussionRegressionRejectsManufacturedParentBinding(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	root := discussionWrite(t, "discussion", "open", "--title", "Q", "--body", "Body", "--category", "help")
	parent := discussionWrite(t, "discussion", "reply", "add", root.ResourceID, "--body", "Native parent")
	discussionWrite(t, "discussion", "reply", "add", root.ResourceID, "--reply-to", parent.ResourceID, "--body", "Native child")
	args := append(discussionTransferArgs(f, "export", root.ResourceID), "--map", "category=C1")
	f.lose = "HNAddDiscussionComment"
	if err := run(args); err == nil {
		t.Fatal("expected uncertain initial parent write")
	}
	scope := githubDiscussionScope(f.server.URL, transferOptions{project: "o/r", id: root.ResourceID})
	name := "discussion-export-" + strings.TrimPrefix(scope, "sha256:") + ".json"
	var cp githubDiscussionCheckpoint
	found, err := forgeLoadJSON(name, &cp)
	if err != nil || !found {
		t.Fatal(found, err)
	}
	if cp.Next != 1 || len(cp.Actions) != 3 {
		t.Fatalf("unexpected initial plan next=%d actions=%d", cp.Next, len(cp.Actions))
	}
	f.d.Comments = append(f.d.Comments, githubapi.DiscussionComment{ID: "HUMAN", DiscussionID: "D1", Body: "Unrelated human comment"})
	cp.Next = 2
	cp.Pending = nil
	cp.Bindings[parent.ResourceID] = githubDiscussionBinding{RemoteID: "HUMAN"}
	delete(cp.Baselines, parent.ResourceID)
	if err := forgeStoreJSON(name, &cp); err != nil {
		t.Fatal(err)
	}
	before := f.writes
	err = run(args)
	if f.writes != before {
		t.Fatalf("unproven checkpoint parent redirected native child: writes=%d err=%v parent=%s", f.writes-before, err, f.d.Comments[len(f.d.Comments)-1].ParentID)
	}
	if err == nil {
		t.Fatal("unproven parent checkpoint accepted")
	}
}

func TestGitHubDiscussionRegressionSourceRepoRenameRetainsIdentity(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	f.d = githubapi.Discussion{ID: "D1", Title: "Foreign question", Body: "Source body"}
	if err := run(append(discussionTransferArgs(f, "import", ""), "--key", "7")); err != nil {
		t.Fatal(err)
	}
	root := bridgeFamily(bridgeCatalog(t), ResourceDiscussion)[0]
	oldHandler := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		oldHandler.ServeHTTP(rec, r)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		w.Write([]byte(strings.ReplaceAll(rec.Body.String(), `"nameWithOwner":"o/r"`, `"nameWithOwner":"o/renamed"`)))
	})
	args := append(discussionTransferArgs(f, "export", root.ID), "--map", "category=C1")
	for i := range args {
		if args[i] == "o/r" {
			args[i] = "o/renamed"
		}
	}
	before := f.writes
	err := run(args)
	if f.writes != before {
		t.Fatalf("repository rename duplicated stable source D1/R1: roots=%d writes=%d err=%v", f.createRoots, f.writes-before, err)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestGitHubDiscussionRealExportIdentityChangeAndKeyFreePreview(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	root := discussionWrite(t, "discussion", "open", "--title", "Q", "--body", "Body", "--category", "help")
	args := append(discussionTransferArgs(f, "export", root.ResourceID), "--map", "category=C1")
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	original, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	replacement := testIdentity(t, "Other exporter")
	if _, err := storeIdentityRecord(replacement, identityLifecycleAvailable); err != nil {
		t.Fatal(err)
	}
	if err := replaceActiveIdentity(replacement.Actor, original.Actor); err != nil {
		t.Fatal(err)
	}
	before := f.writes
	if err := run(args); err == nil || !strings.Contains(err.Error(), "different native identity") {
		t.Fatal("identity change silently adopted journal", err)
	}
	if f.writes != before {
		t.Fatal("identity change wrote")
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	keydir := filepath.Dir(paths.identities)
	hidden := keydir + "-unavailable"
	if err := os.Rename(keydir, hidden); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(hidden, keydir)
	if err := run(append(args, "--dry-run")); err != nil {
		t.Fatal("preview required native private keys", err)
	}
	if f.writes != before {
		t.Fatal("preview wrote")
	}
	if err := run(args); err == nil {
		t.Fatal("real export without native signing identity succeeded")
	}
}
func TestGitHubDiscussionCanonicalCheckpointRejectsUnsignedAndAliases(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	root := discussionWrite(t, "discussion", "open", "--title", "Q", "--body", "Body", "--category", "help")
	args := append(discussionTransferArgs(f, "export", root.ResourceID), "--map", "category=C1")
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	scope := githubDiscussionScope(f.server.URL, transferOptions{project: "o/r", id: root.ResourceID})
	name := "discussion-export-" + strings.TrimPrefix(scope, "sha256:") + ".json"
	path, _ := forgeCheckpointPath(name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before := f.writes
	for _, changed := range [][]byte{append([]byte(" "), raw...), []byte(strings.Replace(string(raw), `"Scope":`, `"SCOPE":`, 1)), []byte(strings.Replace(string(raw), `"Signature":"`, `"Signature":"invalid`, 1))} {
		if err := os.WriteFile(path, changed, 0600); err != nil {
			t.Fatal(err)
		}
		if err := run(args); err == nil {
			t.Fatal("noncanonical/invalid signature accepted")
		}
		if f.writes != before {
			t.Fatal("tampered envelope wrote")
		}
	}
}

func TestGitHubDiscussionDistinctStableRepositoryCreatesIntentionalCopy(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "fixture-token")
	f := newDiscussionFixture(t)
	f.d = githubapi.Discussion{ID: "D1", Title: "Original", Body: "Source body"}
	if err := run(append(discussionTransferArgs(f, "import", ""), "--key", "7")); err != nil {
		t.Fatal(err)
	}
	root := bridgeFamily(bridgeCatalog(t), ResourceDiscussion)[0]
	original := discussionAPINode(f.d)
	oldHandler := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(request))
		var q struct {
			Query     string
			Variables map[string]any
		}
		json.Unmarshal(request, &q)
		recorder := httptest.NewRecorder()
		oldHandler.ServeHTTP(recorder, r)
		var response map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Error(err)
			return
		}
		data := response["data"].(map[string]any)
		destinationNode := func() map[string]any {
			n := discussionAPINode(f.d)
			n["id"] = "D2"
			n["repository"] = map[string]any{"id": "R2"}
			return n
		}
		switch {
		case strings.Contains(q.Query, "HNDiscussionRepository"):
			data["repository"] = map[string]any{"id": "R2", "nameWithOwner": "other/repo", "hasDiscussionsEnabled": true}
		case strings.Contains(q.Query, "HNDiscussionCategories"):
			data["repository"].(map[string]any)["id"] = "R2"
		case strings.Contains(q.Query, "HNDiscussions("):
			nodes := []any{}
			if f.createRoots > 0 {
				nodes = append(nodes, destinationNode())
			}
			data["repository"] = map[string]any{"id": "R2", "discussions": discussionAPIConnection(nodes)}
		case strings.Contains(q.Query, "HNDiscussionID"):
			if q.Variables["id"] == "D1" {
				data["node"] = original
			} else {
				data["node"] = destinationNode()
			}
		case strings.Contains(q.Query, "HNDiscussionComments"):
			data["node"].(map[string]any)["id"] = q.Variables["id"]
		case strings.Contains(q.Query, "HNCreateDiscussion"):
			data["createDiscussion"] = map[string]any{"discussion": destinationNode()}
		}
		for k, v := range recorder.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(recorder.Code)
		json.NewEncoder(w).Encode(response)
	})
	args := append(discussionTransferArgs(f, "export", root.ID), "--map", "category=C1")
	for i := range args {
		if args[i] == "o/r" {
			args[i] = "other/repo"
		}
	}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	if f.createRoots != 1 || f.writes != 1 {
		t.Fatal("distinct repository did not create exactly one copy")
	}
	before := f.writes
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	if f.writes != before {
		t.Fatal("intentional copy duplicated")
	}
}
