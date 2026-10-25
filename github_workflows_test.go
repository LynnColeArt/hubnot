//go:build !nogithub

package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/forge"
)

func workflowGHBlob(b []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(b))), b...)))
}

type workflowGHFixture struct {
	content []byte
	writes  int
	lose    bool
	server  *httptest.Server
}

func newWorkflowGHFixture(t *testing.T, content string) *workflowGHFixture {
	t.Helper()
	f := &workflowGHFixture{content: []byte(content)}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blob := workflowGHBlob(f.content)
		commit := strings.Repeat("a", 40)
		switch {
		case r.URL.Path == "/repos/a/b/commits":
			json.NewEncoder(w).Encode([]any{map[string]any{"sha": commit, "commit": map[string]any{"tree": map[string]any{"sha": strings.Repeat("b", 40)}}}})
		case r.URL.Path == "/repos/a/b/actions/workflows":
			json.NewEncoder(w).Encode(map[string]any{"total_count": 1, "workflows": []any{map[string]any{"id": 1, "node_id": "W_1", "name": "CI", "path": ".github/workflows/ci.yml", "state": "active", "created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-02T03:04:05Z", "html_url": "https://example.invalid/ci"}}})
		case strings.Contains(r.URL.Path, "/git/trees/"):
			name, mode, kind, sha := ".github", "040000", "tree", strings.Repeat("c", 40)
			if strings.HasSuffix(r.URL.Path, strings.Repeat("c", 40)) {
				name, sha = "workflows", strings.Repeat("d", 40)
			}
			if strings.HasSuffix(r.URL.Path, strings.Repeat("d", 40)) {
				name, mode, kind, sha = "ci.yml", "100644", "blob", blob
			}
			json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": name, "mode": mode, "type": kind, "sha": sha}}})
		case r.URL.Path == "/repos/a/b/contents/.github/workflows/ci.yml" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"type": "file", "path": ".github/workflows/ci.yml", "sha": blob, "size": len(f.content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(f.content)})
		case r.URL.Path == "/repos/a/b/contents/.github/workflows/ci.yml" && r.Method == "PUT":
			var in map[string]string
			json.NewDecoder(r.Body).Decode(&in)
			if in["sha"] != blob || in["branch"] != "main" {
				w.WriteHeader(409)
				return
			}
			next, err := base64.StdEncoding.DecodeString(in["content"])
			if err != nil {
				t.Error(err)
			}
			f.content = next
			f.writes++
			if f.lose {
				f.lose = false
				w.WriteHeader(500)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"path": ".github/workflows/ci.yml", "sha": workflowGHBlob(next)}, "commit": map[string]any{"sha": commit}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}
func workflowTransfer(t *testing.T, f *workflowGHFixture, args ...string) (string, error) {
	t.Helper()
	return captureTestOutput(t, func() error {
		return cmdTransfer(append(args, "--provider", "github", "--resource", "workflow", "--project", "a/b", "--endpoint", f.server.URL))
	})
}
func TestGitHubWorkflowImportIsNativeExactAndInert(t *testing.T) {
	root := issueCommandTestRepo(t)
	f := newWorkflowGHFixture(t, "name: CI\r\non: [push]\n# café ${{ secrets.NOPE }}\n")
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err := workflowTransfer(t, f, "import", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("dry-run wrote facts")
	}
	if _, err := workflowTransfer(t, f, "import"); err != nil {
		t.Fatal(err)
	}
	views := bridgeFamily(bridgeCatalog(t), ResourceWorkflow)
	if len(views) != 1 {
		t.Fatal(views)
	}
	var state WorkflowState
	json.Unmarshal(views[0].State, &state)
	if state.Enabled || state.Pipeline != nil || len(state.SourceDocuments) != 1 || state.SourceDocuments[0].Content != string(f.content) || state.SourceDocuments[0].Digest != eventID(f.content) {
		t.Fatal(state)
	}
	if _, err := workflowCall(t, "materialize", views[0].ID, "--json"); err == nil {
		t.Fatal("inert YAML materialized as runnable")
	}
	if _, err := os.Stat(filepath.Join(root, ".hn", "pipelines", "ci.json")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := workflowTransfer(t, f, "import"); err != nil {
		t.Fatal(err)
	}
	if len(bridgeFamily(bridgeCatalog(t), ResourceWorkflow)[0].History) != 1 {
		t.Fatal("duplicate import")
	}
}
func TestGitHubWorkflowExplicitMappingAndGeneratedExactRoundtrip(t *testing.T) {
	issueCommandTestRepo(t)
	state := workflowTestState()
	state.Pipeline.Steps[0].Command = "printf"
	state.Pipeline.Steps[0].Args = []string{"quote ' $HOME $(false)\n${{ secrets.NOPE }}"}
	config := githubWorkflowExportConfig{Format: "actions", Runner: "ubuntu-24.04", Trigger: "workflow_dispatch", Checkout: "none", Runtime: "python3-posix"}
	content, err := githubRenderNativeWorkflow(state, config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "${{") || strings.Contains(string(content), "$HOME") || !strings.Contains(string(content), "permissions:") {
		t.Fatal(string(content))
	}
	f := newWorkflowGHFixture(t, string(content))
	if _, err = workflowTransfer(t, f, "import", "--key", "1", "--map", "native=generated", "--map", "enable=true"); err != nil {
		t.Fatal(err)
	}
	views := bridgeFamily(bridgeCatalog(t), ResourceWorkflow)
	var got WorkflowState
	json.Unmarshal(views[0].State, &got)
	if !got.Enabled || got.Pipeline == nil || got.Pipeline.Steps[0].Args[0] != state.Pipeline.Steps[0].Args[0] {
		t.Fatal(got)
	}
	f.content = append(f.content, []byte("# changed\n")...)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err = workflowTransfer(t, f, "import", "--map", "native=generated"); err == nil {
		t.Fatal("accepted tampered renderer")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("bad mapping wrote facts")
	}
}
func TestGitHubWorkflowExportPUTRecoveryAndRemoteConflict(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "test-token")
	f := newWorkflowGHFixture(t, "old workflow\n")
	old := workflowGHBlob(f.content)
	native := WorkflowState{Schema: "hn.workflow/1", Name: "ci", Title: "Saved source", SourceDocuments: []WorkflowSourceDocument{{Format: "github-actions-yaml", Content: "on: workflow_dispatch\n", Digest: eventID([]byte("on: workflow_dispatch\n"))}}}
	opened := workflowOpenTest(t, native)
	args := []string{"export", opened.ResourceID, "--map", "format=source", "--map", "branch=main", "--map", "path=.github/workflows/ci.yml", "--map", "expected-sha=" + old}
	out, err := workflowTransfer(t, f, append(args, "--dry-run")...)
	if err != nil || f.writes != 0 || !strings.Contains(out, "may activate") {
		t.Fatal(out, err, f.writes)
	}
	f.lose = true
	if _, err = workflowTransfer(t, f, args...); err == nil || f.writes != 1 {
		t.Fatal(err, f.writes)
	}
	if _, err = workflowTransfer(t, f, args...); err != nil || f.writes != 1 {
		t.Fatal(err, f.writes)
	}
	if _, err = workflowTransfer(t, f, args...); err != nil || f.writes != 1 {
		t.Fatal(err, f.writes)
	}
	f.content = []byte("remote edited\n")
	if _, err = workflowTransfer(t, f, args...); err == nil || f.writes != 1 {
		t.Fatal("remote conflict overwritten", err, f.writes)
	}
}
func TestGitHubWorkflowPreflightMappingsAndCheckpointBounds(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "test-token")
	f := newWorkflowGHFixture(t, "old\n")
	opened := workflowOpenTest(t, workflowTestState())
	if _, err := workflowTransfer(t, f, "export", opened.ResourceID, "--map", "format=actions", "--map", "branch=main", "--map", "path=.github/workflows/ci.yml", "--map", "expected-sha="+workflowGHBlob(f.content)); err == nil || f.writes != 0 {
		t.Fatal(err)
	}
	s, err := forgeSnapshotResource(opened.ResourceID, forge.ResourceWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	s.Graph.Findings = append(s.Graph.Findings, forge.Finding{Status: forge.Mapped, Path: "x", Message: strings.Repeat("x", forgeCheckpointLimit)})
	c := githubWorkflowCheckpoint{Snapshot: s}
	if githubWorkflowCheckpointBudget(c) == nil {
		t.Fatal("accepted future oversized checkpoint")
	}
}

func TestGitHubWorkflowExplicitPipelineFileAndEnableGuard(t *testing.T) {
	root := issueCommandTestRepo(t)
	f := newWorkflowGHFixture(t, "on: [push]\njobs: arbitrary\n")
	pipeline := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "test", Command: "printf", Args: []string{"mapped"}}}}
	raw, _ := json.Marshal(pipeline)
	file := filepath.Join(root, "explicit.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err := workflowTransfer(t, f, "import", "--map", "enable=true"); err == nil {
		t.Fatal("enabled arbitrary YAML")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("failed mapping wrote")
	}
	if _, err := workflowTransfer(t, f, "import", "--key", "1", "--map", "pipeline-file="+file, "--map", "enable=true"); err != nil {
		t.Fatal(err)
	}
	views := bridgeFamily(bridgeCatalog(t), ResourceWorkflow)
	var s WorkflowState
	json.Unmarshal(views[0].State, &s)
	if !s.Enabled || s.Pipeline == nil || s.Pipeline.Steps[0].Args[0] != "mapped" || s.SourceDocuments[0].Content != string(f.content) {
		t.Fatal(s)
	}
}
func TestGitHubWorkflowUnknownNegativeResultNeverRetriesPUT(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "token")
	f := newWorkflowGHFixture(t, "old\n")
	old := workflowGHBlob(f.content)
	next := "on: workflow_dispatch\n"
	opened := workflowOpenTest(t, WorkflowState{Schema: "hn.workflow/1", Name: "ci", Title: "Exact", SourceDocuments: []WorkflowSourceDocument{{Format: "github-actions-yaml", Content: next, Digest: eventID([]byte(next))}}})
	args := []string{"export", opened.ResourceID, "--map", "format=source", "--map", "branch=main", "--map", "path=.github/workflows/ci.yml", "--map", "expected-sha=" + old}
	f.lose = true
	if _, err := workflowTransfer(t, f, args...); err == nil || f.writes != 1 {
		t.Fatal(err, f.writes)
	}
	f.content = []byte("old\n")
	if _, err := workflowTransfer(t, f, args...); err == nil || f.writes != 1 {
		t.Fatal("negative lookup retried PUT", err, f.writes)
	}
}
func TestGitHubWorkflowExactIntentCannotChangeCASOrDestination(t *testing.T) {
	issueCommandTestRepo(t)
	opened := workflowOpenTest(t, workflowTestState())
	snapshot, err := forgeSnapshotResource(opened.ResourceID, forge.ResourceWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	c := githubWorkflowExportConfig{Format: "actions", Branch: "main", Path: ".github/workflows/ci.yml", ExpectedSHA: strings.Repeat("a", 40), Runner: "ubuntu-24.04", Trigger: "workflow_dispatch", Checkout: "none", Runtime: "python3-posix"}
	content, err := githubWorkflowContent(snapshot, c)
	if err != nil {
		t.Fatal(err)
	}
	destination := githubWorkflowDestination{"github", "https://api.github.com", "a/b", opened.ResourceID, c}
	checkpoint := githubWorkflowCheckpoint{Schema: "hn.github-workflow-export/1", Scope: githubWorkflowScope(destination), Destination: destination, Snapshot: snapshot}
	intent := githubWorkflowIntent(checkpoint, content, strings.Repeat("b", 40))
	checkpoint.Pending = &intent
	if githubValidateWorkflowIntent(checkpoint, content) == nil {
		t.Fatal("checkpoint changed explicit expected SHA")
	}
	intent = githubWorkflowIntent(checkpoint, content, c.ExpectedSHA)
	checkpoint.Pending = &intent
	if err := githubValidateWorkflowIntent(checkpoint, content); err != nil {
		t.Fatal(err)
	}
	checkpoint.Pending.NativeRevision = eventID([]byte("other"))
	if githubValidateWorkflowIntent(checkpoint, content) == nil {
		t.Fatal("checkpoint changed source revision")
	}
}

func TestGitHubWorkflowNativePublicationDoesNotDuplicateSignedRoot(t *testing.T) {
	issueCommandTestRepo(t)
	opened := workflowOpenTest(t, workflowTestState())
	snapshot, err := forgeSnapshotResource(opened.ResourceID, forge.ResourceWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	content, err := githubWorkflowContent(snapshot, githubWorkflowExportConfig{Format: "actions", Runner: "ubuntu-24.04", Trigger: "workflow_dispatch", Checkout: "none", Runtime: "python3-posix"})
	if err != nil {
		t.Fatal(err)
	}
	f := newWorkflowGHFixture(t, string(content))
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	// No export journal exists: immutable native facts alone establish the proof.
	for _, flags := range [][]string{{"import"}, {"import", "--dry-run"}, {"import", "--map", "native=generated"}} {
		if _, err := workflowTransfer(t, f, flags...); err == nil || !strings.Contains(err.Error(), "existing signed native workflow") {
			t.Fatal(err)
		}
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") || len(bridgeFamily(bridgeCatalog(t), ResourceWorkflow)) != 1 {
		t.Fatal("native publication duplicated facts")
	}
	t.Run("unrelated destination", func(t *testing.T) {
		issueCommandTestRepo(t)
		if _, err := workflowTransfer(t, f, "import", "--map", "native=generated"); err != nil {
			t.Fatal(err)
		}
		if len(bridgeFamily(bridgeCatalog(t), ResourceWorkflow)) != 1 {
			t.Fatal("unrelated import refused")
		}
	})
}

func TestGitHubWorkflowNativePublicationProofSurvivesGitReplication(t *testing.T) {
	selection, actor, _, privateRoot := setupSingleActorReplication(t)
	receiver, _ := os.Getwd()
	if err := os.Chdir(filepath.Join(privateRoot, "publisher")); err != nil {
		t.Fatal(err)
	}
	writeActiveTestIdentity(t, actor)
	opened := workflowOpenTest(t, workflowTestState())
	snapshot, err := forgeSnapshotResource(opened.ResourceID, forge.ResourceWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	content, err := githubWorkflowContent(snapshot, githubWorkflowExportConfig{Format: "actions", Runner: "ubuntu-24.04", Trigger: "workflow_dispatch", Checkout: "none", Runtime: "python3-posix"})
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "push", "-q", "origin", actorRef(actor.Actor)+":"+actorRef(actor.Actor))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	selection.Actors = []string{actor.Actor}
	result, err := runReplicationTransaction(selection)
	if err != nil || !replicationOutcomeHasStatus(result.Outcomes, replicationActor, actor.Actor, replicationPromoted) {
		t.Fatal(result, err)
	}
	f := newWorkflowGHFixture(t, string(content))
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err := workflowTransfer(t, f, "import", "--dry-run"); err == nil || !strings.Contains(err.Error(), "existing signed native workflow") {
		t.Fatal(err)
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") || len(bridgeFamily(bridgeCatalog(t), ResourceWorkflow)) != 1 {
		t.Fatal("replica publication proof duplicated native root")
	}
}
func TestGitHubWorkflowPreflightReservesFutureConfirmedResult(t *testing.T) {
	c := githubWorkflowCheckpoint{Schema: "hn.github-workflow-export/1", Destination: githubWorkflowDestination{Project: "a/b", Config: githubWorkflowExportConfig{Path: ".github/workflows/ci.yml"}}, Pending: &forgeRemoteIntent{Phase: "prepared", Payload: json.RawMessage(`""`)}}
	raw, _ := json.Marshal(c)
	padding := forgeCheckpointLimit - len(raw) - 100
	c.Pending.Payload = json.RawMessage(`"` + strings.Repeat("x", padding) + `"`)
	raw, _ = json.Marshal(c)
	if len(raw) > forgeCheckpointLimit {
		t.Fatal("fixture already oversized")
	}
	if githubWorkflowCheckpointBudget(c) == nil {
		t.Fatal("future confirmed result was not reserved before dispatch")
	}
}

func TestGitHubWorkflowNativeMappingRejectsAmbiguousKeysBeforeFacts(t *testing.T) {
	root := issueCommandTestRepo(t)
	f := newWorkflowGHFixture(t, "on: [push]\n")
	file := filepath.Join(root, "ambiguous.json")
	raw := []byte(`{"version":"hn.pipeline/0","steps":[{"name":"check","command":"true","Command":"false"}]}`)
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err := workflowTransfer(t, f, "import", "--key", "1", "--map", "pipeline-file="+file); err == nil {
		t.Fatal("ambiguous native mapping accepted")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("ambiguous mapping wrote facts")
	}
}

func TestGitHubWorkflowRendererExecutesLiteralArgvWithoutInjection(t *testing.T) {
	for _, tool := range []string{"bash", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("explicit optional export runtime unavailable:", tool)
		}
	}
	dir := t.TempDir()
	args := []string{"dollar $HOME", "quote ' and \"", "line one\nline two", "$(touch injected); touch injected", "${{ secrets.NOPE }}"}
	commandArgs := append([]string{"-c", "import json,sys; print(json.dumps(sys.argv[1:]))"}, args...)
	state := WorkflowState{Schema: "hn.workflow/1", Name: "literal", Title: "Literal fixture", Pipeline: &PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "literal argv", Command: "python3", Args: commandArgs, WorkingDirectory: ".", TimeoutSeconds: 5}}}}
	content, err := githubRenderNativeWorkflow(state, githubWorkflowExportConfig{Format: "actions", Runner: "ubuntu-24.04", Trigger: "workflow_dispatch", Checkout: "none", Runtime: "python3-posix"})
	if err != nil {
		t.Fatal(err)
	}
	script := ""
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "          python3 -c ") {
			script = strings.TrimPrefix(line, "          ")
		}
	}
	if script == "" {
		t.Fatal("own generated fixture has no literal argv script")
	}
	cmd := exec.Command("bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	var got []string
	if json.Unmarshal(output, &got) != nil || len(got) != len(args) {
		t.Fatal(string(output))
	}
	for i := range args {
		if got[i] != args[i] {
			t.Fatalf("argv[%d] changed: %q != %q", i, got[i], args[i])
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "injected")); !os.IsNotExist(err) {
		t.Fatal("argument executed as shell code", err)
	}
}

func TestGitHubWorkflowCheckpointCannotSubstituteAnotherSignedWorkflow(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "token")
	first := workflowOpenTest(t, workflowTestState())
	otherState := workflowTestState()
	otherState.Name = "other"
	otherState.Pipeline.Steps[0].Command = "printf"
	otherState.Pipeline.Steps[0].Args = []string{"wrong workflow"}
	other := workflowOpenTest(t, otherState)
	otherSnapshot, err := forgeSnapshotResource(other.ResourceID, forge.ResourceWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	f := newWorkflowGHFixture(t, "old\n")
	config := githubWorkflowExportConfig{Format: "actions", Branch: "main", Path: ".github/workflows/ci.yml", ExpectedSHA: workflowGHBlob(f.content), Runner: "ubuntu-24.04", Trigger: "workflow_dispatch", Checkout: "none", Runtime: "python3-posix"}
	content, err := githubWorkflowContent(otherSnapshot, config)
	if err != nil {
		t.Fatal(err)
	}
	destination := githubWorkflowDestination{"github", f.server.URL, "a/b", first.ResourceID, config}
	checkpoint := githubWorkflowCheckpoint{Schema: "hn.github-workflow-export/1", Scope: githubWorkflowScope(destination), Destination: destination, Snapshot: otherSnapshot}
	intent := githubWorkflowIntent(checkpoint, content, config.ExpectedSHA)
	checkpoint.Pending = &intent
	name := "workflow-export-" + strings.TrimPrefix(checkpoint.Scope, "sha256:") + ".json"
	if err := forgeStoreJSON(name, checkpoint); err != nil {
		t.Fatal(err)
	}
	args := []string{"export", first.ResourceID, "--map", "format=actions", "--map", "branch=main", "--map", "path=.github/workflows/ci.yml", "--map", "expected-sha=" + config.ExpectedSHA, "--map", "runner=ubuntu-24.04", "--map", "trigger=workflow_dispatch", "--map", "checkout=none", "--map", "runtime=python3-posix"}
	if _, err := workflowTransfer(t, f, args...); err == nil || f.writes != 0 || string(f.content) != "old\n" {
		t.Fatal("substituted another signed workflow", err, f.writes)
	}
}
