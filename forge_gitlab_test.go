//go:build !noforge

package main

import (
	"context"
	"strings"
	"testing"

	"hubnot/internal/forge"
	"hubnot/internal/gitlabapi"
)

func TestGitLabLegacyTransferUsesRegisteredIssueBoundary(t *testing.T) {
	f := newGitLabMigrationFixture(t)
	registration, ok := forgeProviders["gitlab"]
	if !ok || registration.TokenEnv != "HN_GITLAB_TOKEN" {
		t.Fatal("legacy registration absent")
	}
	p, e := registration.New(f.Config().Locator.Endpoint, "supplied-secret")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	v, e := p.CreateIssue(ctx, "42", forge.IssueInput{Title: "legacy issue", Body: "source body", State: "open", Labels: []string{}, Assignees: []string{}})
	if e != nil {
		t.Fatal(e)
	}
	if v.ID != "id:7001" || v.Key != "7" || v.State != "open" {
		t.Fatal(v)
	}
	read, e := p.GetIssue(ctx, "42", "7")
	if e != nil || read.ID != v.ID {
		t.Fatal(read, e)
	}
	rows, e := p.ListIssues(ctx, "42")
	if e != nil || len(rows) != 1 {
		t.Fatal(rows, e)
	}
	n, e := p.CreateComment(ctx, "42", "7", "ordinary note")
	if e != nil || n.ID != "id:9001" {
		t.Fatal(n, e)
	}
	notes, e := p.ListComments(ctx, "42", "7")
	if e != nil || len(notes) != 1 || notes[0].Body != "ordinary note" {
		t.Fatal(notes, e)
	}
	v, e = p.UpdateIssue(ctx, "42", "7", forge.IssueInput{Title: "updated", Body: "updated body", State: "closed", Labels: []string{}, Assignees: []string{}})
	if e != nil || v.State != "closed" {
		t.Fatal(v, e)
	}
	if f.IssueWrites != 2 || f.NoteWrites != 1 {
		t.Fatal(f.IssueWrites, f.NoteWrites)
	}
	before := f.IssueWrites
	if _, e = p.CreateIssue(ctx, "42", forge.IssueInput{Title: "closed", State: "closed"}); e == nil || f.IssueWrites != before {
		t.Fatal("legacy closed create produced effects")
	}
	f.Notes = append(f.Notes, gitlabapi.Note{ID: 9002, Body: "system", System: true})
	if _, e = p.ListComments(ctx, "42", "7"); e == nil || !strings.Contains(e.Error(), "system/internal/thread") {
		t.Fatal("system note flattened", e)
	}
	f.Issue.Confidential = true
	if _, e = p.GetIssue(ctx, "42", "7"); e == nil {
		t.Fatal("confidential issue silently flattened")
	}
}

func TestGitLabLegacyTransferPublicCLIAndNativeContinuation(t *testing.T) {
	binary := buildOperationalBinary(t)
	repos, _ := issuePublicReplicas(t, binary, "GitLab importer")
	f := newGitLabMigrationFixture(t)
	f.Mu.Lock()
	f.Issue = gitlabapi.Issue{ID: 7001, IID: 7, ProjectID: 42, Title: "Public imported issue", Body: "Source description", State: "opened", Type: "issue", Labels: []string{}, Assignees: []gitlabapi.User{}, Author: gitlabapi.User{ID: 3, Username: "foreign author"}}
	f.Notes = []gitlabapi.Note{{ID: 9001, Body: "Source note", NoteableID: 7001, Author: gitlabapi.User{ID: 4, Username: "foreign commenter"}}}
	f.Mu.Unlock()
	cli := interoperabilityCLI{binary: binary, repo: repos[0], env: operationalTestEnvironmentWith(t.TempDir(), map[string]string{"HN_GITLAB_TOKEN": "supplied-secret"})}
	args := []string{"transfer", "import", "--provider", "gitlab", "--project", "42", "--key", "7", "--endpoint", f.Config().Locator.Endpoint}
	preview := append(append([]string{}, args...), "--dry-run")
	cli.call(t, false, preview...)
	cli.call(t, false, args...)
	id := interopID(t, cli.call(t, false, "gh", "issue", "list", "--json", "id,title"))
	view := interopData[publicIssueView](t, cli, "issue", "show", id)
	if view.State == nil || view.State.Title != "Public imported issue" || view.State.Body != "Source description" || view.CommentCount != 1 {
		t.Fatal(view)
	}
	cli.call(t, false, args...)
	view = interopData[publicIssueView](t, cli, "issue", "show", id)
	if view.CommentCount != 1 {
		t.Fatal("repeat import duplicated note")
	}
	cli.call(t, false, "gh", "issue", "comment", id, "--body", "Local continuation")
	view = interopData[publicIssueView](t, cli, "issue", "show", id)
	if view.CommentCount != 2 || f.IssueWrites != 0 || f.NoteWrites != 0 {
		t.Fatal("native continuation mutated GitLab", view)
	}
}
