//go:build !noforge

package main

import (
	"context"
	"hubnot/internal/forge"
	"strings"
	"testing"
)

func runV1LegacyTransfer(t *testing.T, kind, version string) {
	issueCommandTestRepo(t)
	env := "HN_FORGEJO_TOKEN"
	if kind == "gitea" {
		env = "HN_GITEA_TOKEN"
	}
	t.Setenv(env, "explicit-secret")
	source := newV1Fixture(t, kind, version)
	source.seed()
	source.topology = true
	args := []string{"import", "--provider", kind, "--project", "a/b", "--endpoint", source.server.URL + "/api/v1"}
	if out, e := captureTestOutput(t, func() error { return run(append([]string{"transfer"}, args...)) }); e != nil {
		t.Fatal(out, e)
	}
	issues, e := forgeNativeIssues()
	if e != nil || len(issues) != 1 {
		t.Fatal("PR imported as ordinary issue", issues, e)
	}
	id := issues[0].ID
	if issues[0].State.Title != "source issue" || issues[0].State.Status != "closed" {
		t.Fatal("source projection lost", issues[0])
	}
	comments, e := forgeNativeComments(id)
	if e != nil || len(comments) != 1 {
		t.Fatal("comment import missing", comments, e)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, e = captureTestOutput(t, func() error { return cmdTransfer(args) }); e != nil {
		t.Fatal(e)
	}
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); after != before {
		t.Fatal("repeat import changed signed facts")
	}
	t.Setenv(env, "")
	if _, e = executeIssueMutation(issueOptions{command: "comment", id: id, body: "continued native work offline"}); e != nil {
		t.Fatal("imported issue needs provider to continue", e)
	}
	t.Setenv(env, "explicit-secret")
	destination := newV1Fixture(t, kind, version)
	export := []string{"export", id, "--provider", kind, "--project", "a/b", "--endpoint", destination.server.URL + "/api/v1"}
	if out, e := captureTestOutput(t, func() error { return cmdTransfer(export) }); e != nil {
		t.Fatal(out, e)
	}
	writes := destination.issueWrites + destination.commentWrites
	if writes == 0 || destination.issue.State != "closed" || destination.commentWrites != 2 {
		t.Fatal("legacy export did not reach real API", destination.issueWrites, destination.commentWrites)
	}
	if _, e = captureTestOutput(t, func() error { return cmdTransfer(export) }); e != nil || writes != destination.issueWrites+destination.commentWrites {
		t.Fatal("legacy retry duplicated", e)
	}
}
func TestForgejoLegacyTransferPublicCommands(t *testing.T) {
	runV1LegacyTransfer(t, "forgejo", "16.0.0")
}

func runV1LegacyCapabilitiesAndLabelRefusal(t *testing.T, kind, version string) {
	f := newV1Fixture(t, kind, version)
	f.seed()
	registration, ok := forgeProviders[kind]
	if !ok {
		t.Fatal("legacy registration absent")
	}
	p, e := registration.New(f.server.URL+"/api/v1", "explicit-secret")
	if e != nil || p.Kind() != kind {
		t.Fatal(e)
	}
	before := f.requests
	if out, e := captureTestOutput(t, func() error { return cmdTransfer([]string{"capabilities", "--provider", kind}) }); e != nil || !strings.Contains(out, `"import":true`) || f.requests != before {
		t.Fatal("capabilities not inert", out, e)
	}
	_, e = p.UpdateIssue(context.Background(), "a/b", "7", forge.IssueInput{Title: "updated", State: "closed", Labels: []string{"bug"}})
	if e == nil || f.issueWrites != 0 {
		t.Fatal("label-changing PATCH performed", e)
	}
	if _, e = p.GetIssue(context.Background(), "a/b", "007"); e == nil {
		t.Fatal("noncanonical selector accepted")
	}
}
func TestForgejoLegacyCapabilitiesAndLabelRefusal(t *testing.T) {
	runV1LegacyCapabilitiesAndLabelRefusal(t, "forgejo", "16.0.0")
}
