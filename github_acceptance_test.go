package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type interoperabilityCLI struct {
	binary, repo string
	env          []string
}

func (f interoperabilityCLI) call(t *testing.T, wantFailure bool, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.binary, args...)
	cmd.Dir = f.repo
	cmd.Env = f.env
	out := &issueAcceptanceBuffer{limit: 16 << 20}
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if ctx.Err() != nil || (err != nil) != wantFailure {
		t.Fatalf("hn %v: %v %s", args, err, out.String())
	}
	return out.String()
}
func interopData[T any](t *testing.T, f interoperabilityCLI, args ...string) T {
	t.Helper()
	raw := f.call(t, false, append(args, "--json")...)
	var envelope struct {
		OK   bool
		Data json.RawMessage
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || !envelope.OK {
		t.Fatalf("%v: invalid envelope %s (%v)", args, raw, err)
	}
	var v T
	if err := json.Unmarshal(envelope.Data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func interopFile(t *testing.T, repo, name string, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(repo, name)
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func interopID(t *testing.T, raw string) string {
	t.Helper()
	id := regexp.MustCompile(`sha256:[0-9a-f]{64}`).FindString(raw)
	if id == "" {
		t.Fatal("missing public native ID", raw)
	}
	return id
}
func interopFixture(t *testing.T, binary, repo string) interoperabilityCLI {
	t.Helper()
	bin := t.TempDir()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	// Only the child sees this PATH: gh is absent, while Go/build/test tooling
	// and every other test retain the process environment.
	return interoperabilityCLI{binary, repo, operationalTestEnvironmentWith(t.TempDir(), map[string]string{"PATH": bin})}
}
func TestInteroperabilityPublicAcceptance(t *testing.T) {
	normal := buildOperationalBinary(t)
	native := filepath.Join(t.TempDir(), "hn-core")
	build := exec.Command("go", "build", "-tags", "nogithub", "-o", native, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	for _, variant := range []struct{ name, binary string }{{"default", normal}, {"nogithub", native}} {
		t.Run(variant.name, func(t *testing.T) {
			repos, _ := issuePublicReplicas(t, variant.binary, "Publisher", "Reader")
			f := interopFixture(t, variant.binary, repos[0])
			reader := interopFixture(t, native, repos[1])
			help := f.call(t, false, "help")
			for _, name := range []string{"project", "discussion", "workflow"} {
				if !strings.Contains(help, "hn "+name+" help") {
					t.Fatalf("native command %s undiscoverable", name)
				}
			}
			id := interopID(t, f.call(t, false, "gh", "issue", "create", "-t", "Offline issue", "-b", "Initial", "-l", "bug"))
			f.call(t, false, "gh", "issue", "edit", id, "--body", "Edited", "--add-assignee", "andrew")
			f.call(t, false, "gh", "issue", "comment", id, "-b", "Local comment")
			f.call(t, false, "gh", "issue", "close", id)
			f.call(t, false, "gh", "issue", "reopen", id)
			issue := interopData[publicIssueView](t, f, "issue", "show", id)
			if issue.State == nil || issue.State.Body != "Edited" || issue.State.Status != "open" || issue.CommentCount != 1 {
				t.Fatalf("gh/native state differs %+v", issue)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(f.call(t, false, "gh", "issue", "view", id, "--json", "id,state,comments")), &fields); err != nil || len(fields) != 3 {
				t.Fatal(fields, err)
			}
			f.call(t, false, "gh", "issue", "list", "--state", "all", "--json", "id,title")
			f.call(t, true, "gh", "pr", "list")
			f.call(t, true, "gh", "issue", "view", id, "--repo", "o/r")
			p := projectState()
			project := interopData[ResourceMutationResult](t, f, "project", "open", "--input", interopFile(t, f.repo, "project.json", p))
			values := map[string]ProjectValue{"score": {Type: "number", Number: "1.25"}, "phase": {Type: "single_select", OptionID: "todo"}}
			item := interopData[ResourceMutationResult](t, f, "project", "item", "add", "--project", project.ResourceID, "--target", "issue:"+id, "--input", interopFile(t, f.repo, "values.json", values), "--position", "1.25")
			p.Fields[0].Type = "text"
			interopData[ResourceMutationResult](t, f, "project", "revise", project.ResourceID, "--expect", project.EventID, "--input", interopFile(t, f.repo, "project-new.json", p))
			itemView := interopData[ResourceView](t, f, "project-item", "show", item.ResourceID)
			var is ProjectItemState
			json.Unmarshal(itemView.State, &is)
			if is.ProjectRevision != project.EventID || is.Values["score"].Number != "1.25" || is.Target.ID != id {
				t.Fatal("historical typed item changed", is)
			}
			thread := interopData[ResourceMutationResult](t, f, "discussion", "open", "--title", "Question", "--category", "help", "--body", "Details")
			reply := interopData[ResourceMutationResult](t, f, "discussion", "reply", "add", thread.ResourceID, "--body", "Original answer")
			interopData[ResourceMutationResult](t, f, "discussion", "reply", "add", thread.ResourceID, "--reply-to", reply.ResourceID, "--body", "Nested")
			interopData[ResourceMutationResult](t, f, "discussion", "answer", thread.ResourceID, "--expect", thread.EventID, "--reply", reply.EventID)
			interopData[ResourceMutationResult](t, f, "discussion", "reply", "edit", reply.ResourceID, "--expect", reply.EventID, "--body", "New answer")
			shown := interopData[struct {
				ReplyCount     int          `json:"reply_count"`
				AcceptedAnswer ResourceHead `json:"accepted_answer"`
			}](t, f, "discussion", "show", thread.ResourceID)
			var rs DiscussionReplyState
			json.Unmarshal(shown.AcceptedAnswer.State, &rs)
			if shown.ReplyCount != 2 || shown.AcceptedAnswer.ID != reply.EventID || rs.Body != "Original answer" {
				t.Fatal("answer revision drift", shown)
			}
			wstate := workflowTestState()
			workflow := interopData[ResourceMutationResult](t, f, "workflow", "open", "--input", interopFile(t, f.repo, "workflow.json", wstate))
			interopData[map[string]any](t, f, "workflow", "materialize", workflow.ResourceID, "--expect", workflow.EventID)
			pipeline, _ := json.Marshal(wstate.Pipeline)
			materialized, err := os.ReadFile(filepath.Join(f.repo, ".hn", "pipelines", "check.json"))
			if err != nil || string(materialized) != string(pipeline) {
				t.Fatal("materialized bytes", err)
			}
			foreign := WorkflowState{Schema: "hn.workflow/1", Name: "foreign", Title: "Foreign definition", SourceDocuments: []WorkflowSourceDocument{{Format: "actions-yaml", Content: "name: inert\n", Digest: eventID([]byte("name: inert\n"))}}}
			foreignRoot := interopData[ResourceMutationResult](t, f, "workflow", "open", "--input", interopFile(t, f.repo, "foreign.json", foreign))
			beforeForeign := issuePublicRefs(t, f.repo)
			f.call(t, true, "workflow", "materialize", foreignRoot.ResourceID, "--json")
			if beforeForeign != issuePublicRefs(t, f.repo) {
				t.Fatal("foreign materialize refusal signed facts")
			}
			// Native signed Git replication carries all families without any adapter journal.
			f.call(t, false, "sync", "origin")
			reader.call(t, false, "sync", "origin")
			readerGit := runOperationalGit(t, reader.repo, "rev-parse", "--absolute-git-dir")
			for _, name := range []string{"active", "identity.json", "identities"} {
				if err := os.RemoveAll(filepath.Join(readerGit, "hn", name)); err != nil {
					t.Fatal(err)
				}
			}
			reader.call(t, true, "identity", "show", "--json")
			for _, pair := range [][2]string{{"issue", id}, {"project", project.ResourceID}, {"project-item", item.ResourceID}, {"discussion", thread.ResourceID}, {"discussion-reply", reply.ResourceID}, {"workflow", workflow.ResourceID}} {
				raw := reader.call(t, false, pair[0], "show", pair[1], "--json")
				if !strings.Contains(raw, pair[1]) {
					t.Fatal("lost family", pair)
				}
			}
			// Exact immutable actor commits must be transported byte-for-byte, including
			// resource signatures, rather than regenerated by the receiving build.
			publisherRefs := runOperationalGit(t, f.repo, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors")
			receiverRefs := runOperationalGit(t, reader.repo, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/remotes")
			_ = receiverRefs
			if strings.TrimSpace(publisherRefs) == "" {
				t.Fatal("missing signed actor transport evidence")
			}
			for _, line := range strings.Split(strings.TrimSpace(publisherRefs), "\n") {
				parts := strings.Fields(line)
				if len(parts) == 2 {
					runOperationalGit(t, reader.repo, "cat-file", "-e", parts[1]+"^{commit}")
				}
			}
			gitdir := runOperationalGit(t, reader.repo, "rev-parse", "--absolute-git-dir")
			if _, err := os.Stat(filepath.Join(gitdir, "hn", "forge")); !os.IsNotExist(err) {
				t.Fatal("Git transported private adapter journal", err)
			}
			reader.call(t, false, "workflow", "materialize", workflow.ResourceID, "--expect", workflow.EventID, "--json")
			copied, err := os.ReadFile(filepath.Join(reader.repo, ".hn", "pipelines", "check.json"))
			if err != nil || string(copied) != string(pipeline) {
				t.Fatal("key-free clone materialization", err)
			}
			// A governed request binds exact committed bytes and does not run the pipeline.
			identity := interopData[struct{ Actor string }](t, f, "identity", "show")
			policy := PolicyDocument{Version: policyVersion, Maintainers: []string{identity.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}}
			interopFile(t, f.repo, ".hn/policy.json", policy)
			runOperationalGit(t, f.repo, "add", ".hn/policy.json")
			runOperationalGit(t, f.repo, "commit", "-q", "-m", "policy")
			base := runOperationalGit(t, f.repo, "rev-parse", "HEAD")
			runOperationalGit(t, f.repo, "add", ".hn/pipelines/check.json")
			runOperationalGit(t, f.repo, "commit", "-q", "-m", "native pipeline")
			head := runOperationalGit(t, f.repo, "rev-parse", "HEAD")
			f.call(t, false, "proposal", "open", "--base", base, "--head", head, "Workflow acceptance")
			proposalRef := strings.TrimSpace(runOperationalGit(t, f.repo, "for-each-ref", "--format=%(refname)", "refs/hn/proposals"))
			proposal := "sha256:" + strings.TrimPrefix(proposalRef, "refs/hn/proposals/")
			if !validEventID(proposal) {
				t.Fatal("invalid published proposal ref", proposalRef)
			}
			requested := interopData[map[string]any](t, f, "workflow", "request", workflow.ResourceID, "--expect", workflow.EventID, "--proposal", proposal)
			if requested["native_revision"] != workflow.EventID || requested["request_id"] == "" {
				t.Fatal("request lost exact native binding", requested)
			}
			if _, err := os.Stat(filepath.Join(f.repo, "workflow-should-never-run")); !os.IsNotExist(err) {
				t.Fatal("materialize/request executed workflow")
			}
			if variant.name == "nogithub" {
				failure := f.call(t, true, "github", "import", "--repo", "o/r")
				if !strings.Contains(failure, "excluded") {
					t.Fatal(failure)
				}
			}
		})
	}
	t.Run("imported-issue-without-transport", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if strings.HasSuffix(r.URL.Path, "/comments") {
				fmt.Fprint(w, `[]`)
				return
			}
			fmt.Fprint(w, `{"id":123,"node_id":"I123","number":7,"title":"Imported","body":"Source text","state":"open","html_url":"https://github.com/o/r/issues/7","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","user":{"login":"andrew"},"labels":[],"assignees":[]}`)
		}))
		repo := issuePublicRepository(t, normal, "imported")
		f := interopFixture(t, normal, repo)
		f.call(t, false, "transfer", "import", "--provider", "github", "--project", "o/r", "--key", "7", "--endpoint", server.URL)
		rows := interopData[publicIssuePage](t, f, "issue", "list", "--details")
		if len(rows.Items) != 1 {
			t.Fatal("import absent")
		}
		var v publicIssueView
		json.Unmarshal(rows.Items[0], &v)
		before := calls.Load()
		server.Close()
		core := interopFixture(t, native, repo)
		core.call(t, false, "gh", "issue", "edit", v.ID, "--body", "Offline edit")
		shown := interopData[publicIssueView](t, core, "issue", "show", v.ID)
		if shown.State.Body != "Offline edit" || shown.State.Metadata["forge/source"] == "" || calls.Load() != before {
			t.Fatal("import depended on provider")
		}
	})
}
