package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func workflowTestState() WorkflowState {
	return WorkflowState{Schema: "hn.workflow/1", Name: "check", Title: "Native check", Enabled: true, Pipeline: &PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "inert until explicitly executed", Command: "sh", Args: []string{"-c", "touch workflow-should-never-run"}}}}, Metadata: map[string]string{"native/keep": "preserved"}}
}
func workflowOpenTest(t *testing.T, state WorkflowState) *ResourceMutationResult {
	t.Helper()
	raw, err := ResourceStateJSON(ResourceWorkflow, state)
	if err != nil {
		t.Fatal(err)
	}
	result, err := MutateResource(ResourceMutation{Family: ResourceWorkflow, Intent: "open", State: raw})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func workflowCall(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	capture, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Close()
	old := os.Stdout
	os.Stdout = capture
	defer func() { os.Stdout = old }()
	callErr := run(append([]string{"workflow"}, args...))
	os.Stdout = old
	if _, err = capture.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err = json.NewDecoder(capture).Decode(&envelope); err != nil {
		t.Fatalf("JSON response: %v (command %v)", err, callErr)
	}
	return envelope, callErr
}
func workflowData(t *testing.T, envelope map[string]any, err error) map[string]any {
	t.Helper()
	if err != nil || envelope["ok"] != true {
		t.Fatalf("command failed: %v %#v", err, envelope)
	}
	return envelope["data"].(map[string]any)
}
func TestWorkflowMaterializationCanonicalGuardedAndInert(t *testing.T) {
	root := issueCommandTestRepo(t)
	s := workflowTestState()
	opened := workflowOpenTest(t, s)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	envelope, err := workflowCall(t, "show", opened.ResourceID, "--json")
	shown := workflowData(t, envelope, err)
	if shown["status"] != "native-runnable" || shown["runnable"] != true {
		t.Fatal(shown)
	}
	envelope, err = workflowCall(t, "materialize", opened.ResourceID, "--expect", opened.EventID, "--json")
	materialized := workflowData(t, envelope, err)
	raw, _ := json.Marshal(s.Pipeline)
	path := filepath.Join(root, ".hn", "pipelines", "check.json")
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, raw) || materialized["definition_digest"] != eventID(raw) || materialized["native_revision"] != opened.EventID {
		t.Fatal(materialized, string(got), err)
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("materialize mutated signed refs")
	}
	if _, err = os.Stat(filepath.Join(root, "workflow-should-never-run")); !os.IsNotExist(err) {
		t.Fatal("pipeline executed")
	}
	envelope, err = workflowCall(t, "materialize", opened.ResourceID, "--json")
	workflowData(t, envelope, err)
	if err = os.WriteFile(path, []byte("operator edited"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = workflowCall(t, "materialize", opened.ResourceID, "--json"); err == nil {
		t.Fatal("overwrote differing file")
	}
	got, _ = os.ReadFile(path)
	if string(got) != "operator edited" {
		t.Fatal("refusal changed file")
	}
	envelope, err = workflowCall(t, "materialize", opened.ResourceID, "--overwrite", "--json")
	workflowData(t, envelope, err)
	got, _ = os.ReadFile(path)
	if !bytes.Equal(got, raw) {
		t.Fatal("explicit overwrite failed")
	}
	if _, err = workflowCall(t, "materialize", opened.ResourceID, "--expect", eventID([]byte("stale")), "--json"); err == nil {
		t.Fatal("stale materialization")
	}
}
func TestWorkflowMaterializationRejectsSymlinkPaths(t *testing.T) {
	for _, component := range []string{".hn", "pipelines", "check.json"} {
		t.Run(component, func(t *testing.T) {
			root := issueCommandTestRepo(t)
			opened := workflowOpenTest(t, workflowTestState())
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "sentinel")
			if err := os.WriteFile(sentinel, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			var link, target string
			switch component {
			case ".hn":
				link = filepath.Join(root, ".hn")
				target = outside
			case "pipelines":
				if err := os.Mkdir(filepath.Join(root, ".hn"), 0755); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(root, ".hn", "pipelines")
				target = outside
			default:
				if err := os.MkdirAll(filepath.Join(root, ".hn", "pipelines"), 0755); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(root, ".hn", "pipelines", "check.json")
				target = sentinel
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skip(err)
			}
			if _, err := workflowCall(t, "materialize", opened.ResourceID, "--overwrite", "--json"); err == nil {
				t.Fatal("followed symlink")
			}
			got, _ := os.ReadFile(sentinel)
			if string(got) != "unchanged" {
				t.Fatal("outside file changed")
			}
			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
				t.Fatal("outside write", entries, err)
			}
		})
	}
}
func TestWorkflowDisabledForeignConflictAndDefinitionValidation(t *testing.T) {
	root := issueCommandTestRepo(t)
	disabled := workflowTestState()
	disabled.Enabled = false
	source := "name: foreign\njobs: no native equivalence"
	foreign := WorkflowState{Schema: "hn.workflow/1", Name: "foreign", Title: "Foreign source", SourceDocuments: []WorkflowSourceDocument{{Format: "actions-yaml", Content: source, Digest: eventID([]byte(source))}}}
	for _, s := range []WorkflowState{disabled, foreign} {
		opened := workflowOpenTest(t, s)
		shown, err := workflowCall(t, "show", opened.ResourceID, "--json")
		data := workflowData(t, shown, err)
		if data["runnable"] != false {
			t.Fatal("disabled/inert reported runnable")
		}
		if _, err = workflowCall(t, "materialize", opened.ResourceID, "--json"); err == nil {
			t.Fatal("materialized disabled/inert workflow")
		}
		if _, err = workflowCall(t, "request", opened.ResourceID, "--proposal", eventID([]byte("proposal")), "--json"); err == nil {
			t.Fatal("requested disabled/inert workflow")
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".hn")); !os.IsNotExist(err) {
		t.Fatal("invalid materialization made directories")
	}
	actor, _ := loadIdentity()
	base := workflowOpenTest(t, workflowTestState())
	state, _ := ResourceStateJSON(ResourceWorkflow, workflowTestState())
	for _, name := range []string{"first", "second"} {
		s := workflowTestState()
		s.Title = name
		state, _ = ResourceStateJSON(ResourceWorkflow, s)
		appendIssueCommandFixture(t, actor, Event{Kind: "workflow.revise", Subject: base.ResourceID, Parents: []string{base.EventID}, Intent: "revise", Body: string(state)})
	}
	shown, err := workflowCall(t, "show", base.ResourceID, "--json")
	data := workflowData(t, shown, err)
	if data["status"] != "conflicted" || data["state"] != nil {
		t.Fatal("conflict hidden", data)
	}
	if _, err = workflowCall(t, "materialize", base.ResourceID, "--json"); err == nil {
		t.Fatal("materialized arbitrary conflict winner")
	}
	bad := workflowTestState()
	bad.Name = "../escape"
	if _, err := ResourceStateJSON(ResourceWorkflow, bad); err == nil {
		t.Fatal("unsafe name")
	}
	bad = workflowTestState()
	bad.Pipeline.Version = "other/0"
	if _, err := ResourceStateJSON(ResourceWorkflow, bad); err == nil {
		t.Fatal("invalid pipeline version")
	}
}
func workflowProposalTest(t *testing.T, base, head string) *StoredEvent {
	t.Helper()
	actor, _ := loadIdentity()
	proposal := appendIssueCommandFixture(t, actor, Event{Kind: "proposal.open", Title: "Check workflow", Base: base, Head: head})
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	return proposal
}
func TestWorkflowRequestBindsCanonicalProposalBytesAndPreservesCI(t *testing.T) {
	root := issueCommandTestRepo(t)
	actor, _ := loadIdentity()
	writeTestPolicy(t, root, PolicyDocument{Version: policyVersion, Maintainers: []string{actor.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-q", "-m", "policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	opened := workflowOpenTest(t, workflowTestState())
	envelope, err := workflowCall(t, "materialize", opened.ResourceID, "--json")
	workflowData(t, envelope, err)
	mustGit(t, "add", ".hn/pipelines/check.json")
	mustGit(t, "commit", "-q", "-m", "native definition")
	head := mustGitText(t, "rev-parse", "HEAD")
	proposal := workflowProposalTest(t, base, head)
	before := mustGitText(t, "rev-parse", "HEAD")
	envelope, err = workflowCall(t, "request", opened.ResourceID, "--proposal", proposal.ID, "--expect", opened.EventID, "--actor", actor.Actor, "--json")
	data := workflowData(t, envelope, err)
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	requests := runRequests(events)
	if len(requests) != 1 {
		t.Fatalf("requests %d", len(requests))
	}
	request := requests[0]
	raw, _ := json.Marshal(workflowTestState().Pipeline)
	if request.Event.Subject != proposal.ID || request.Event.Commit != head || request.Event.Definition != eventID(raw) || request.Event.Pipeline != "check" || len(request.Event.Evidence) != 0 || data["request_id"] != request.ID || data["native_revision"] != opened.EventID {
		t.Fatal("request binding", request.Event, data)
	}
	if before != mustGitText(t, "rev-parse", "HEAD") {
		t.Fatal("request committed")
	}
	if _, err = os.Stat(filepath.Join(root, "workflow-should-never-run")); !os.IsNotExist(err) {
		t.Fatal("request executed pipeline")
	}
	// Equal parsed semantics with differing bytes still refuse exact binding.
	if err = os.WriteFile(filepath.Join(root, ".hn", "pipelines", "check.json"), append(raw, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn/pipelines/check.json")
	mustGit(t, "commit", "-q", "-m", "different bytes")
	changed := workflowProposalTest(t, base, mustGitText(t, "rev-parse", "HEAD"))
	refs := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err = workflowCall(t, "request", opened.ResourceID, "--proposal", changed.ID, "--json"); err == nil {
		t.Fatal("accepted mismatched immutable pipeline bytes")
	}
	if refs != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("refusal wrote event")
	}
	if _, err = workflowCall(t, "request", opened.ResourceID, "--proposal", proposal.ID, "--actor", strings.Repeat("a", 64), "--json"); err == nil {
		t.Fatal("request actor mismatch")
	}
	if refs != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("actor refusal wrote event")
	}
}
func TestWorkflowPublicCLIWithoutProvider(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "hn")
	build := exec.Command("go", "build", "-tags", "nogithub", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native-only CLI: %v %s", err, output)
	}
	root := issueCommandTestRepo(t)
	input := filepath.Join(root, "workflow.json")
	raw, _ := ResourceStateJSON(ResourceWorkflow, workflowTestState())
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	invoke := func(args ...string) map[string]any {
		command := exec.Command(binary, append([]string{"workflow"}, args...)...)
		command.Dir = root
		command.Env = append(os.Environ(), "HN_GITHUB_TOKEN=unused-no-network")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("CLI %v: %v %s", args, err, output)
		}
		var envelope map[string]any
		if err = json.Unmarshal(output, &envelope); err != nil {
			t.Fatal(string(output), err)
		}
		return envelope
	}
	opened := invoke("open", "--input", input, "--operation", "public-native", "--json")["data"].(map[string]any)
	id := opened["resource_id"].(string)
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(paths.active); err != nil {
		t.Fatal(err)
	}
	shown := invoke("show", id, "--json")["data"].(map[string]any)
	if shown["runnable"] != true {
		t.Fatal(shown)
	}
	invoke("materialize", id, "--expect", opened["event_id"].(string), "--json")
	if _, err := os.Stat(filepath.Join(root, "workflow-should-never-run")); !os.IsNotExist(err) {
		t.Fatal("public materialization executed command")
	}
}

func TestWorkflowMaterializationRejectsInternalLinksAndPreservesHardlinkTargets(t *testing.T) {
	root := issueCommandTestRepo(t)
	opened := workflowOpenTest(t, workflowTestState())
	real := filepath.Join(root, "real-hn")
	if err := os.MkdirAll(filepath.Join(real, "pipelines"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real-hn", filepath.Join(root, ".hn")); err != nil {
		t.Skip(err)
	}
	if _, err := workflowCall(t, "materialize", opened.ResourceID, "--json"); err == nil {
		t.Fatal("accepted internal directory symlink")
	}
	if entries, err := os.ReadDir(filepath.Join(real, "pipelines")); err != nil || len(entries) != 0 {
		t.Fatal("internal symlink target changed", entries, err)
	}
	if err := os.Remove(filepath.Join(root, ".hn")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".hn", "pipelines"), 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "hardlink-target")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, ".hn", "pipelines", "check.json")
	if err := os.Link(outside, destination); err != nil {
		t.Skip(err)
	}
	envelope, err := workflowCall(t, "materialize", opened.ResourceID, "--overwrite", "--json")
	workflowData(t, envelope, err)
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "outside sentinel" {
		t.Fatal("overwrite mutated hardlink target", string(got), err)
	}
	entries, err := os.ReadDir(filepath.Dir(destination))
	if err != nil || len(entries) != 1 || entries[0].Name() != "check.json" {
		t.Fatal("temporary artifacts remain", entries, err)
	}
}
func TestWorkflowGenericAuthoringRevisionAndResolutionRemainAvailable(t *testing.T) {
	root := issueCommandTestRepo(t)
	s := workflowTestState()
	source := "name: preserved source"
	s.SourceDocuments = []WorkflowSourceDocument{{Format: "actions-yaml", Content: source, Digest: eventID([]byte(source))}}
	raw, _ := ResourceStateJSON(ResourceWorkflow, s)
	input := filepath.Join(root, "state.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	envelope, err := workflowCall(t, "open", "--input", input, "--operation", "generic-open", "--json")
	opened := workflowData(t, envelope, err)
	id := opened["resource_id"].(string)
	first := opened["event_id"].(string)
	s.Title = "Revised title"
	raw, _ = ResourceStateJSON(ResourceWorkflow, s)
	if err = os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	envelope, err = workflowCall(t, "revise", id, "--expect", first, "--input", input, "--operation", "generic-edit", "--json")
	edited := workflowData(t, envelope, err)
	envelope, err = workflowCall(t, "revise", id, "--expect", first, "--input", input, "--operation", "generic-edit", "--json")
	replay := workflowData(t, envelope, err)
	if replay["replayed"] != true || replay["event_id"] != edited["event_id"] {
		t.Fatal("generic immutable replay lost")
	}
	if _, err = workflowCall(t, "materialize", id, "--expect", first, "--json"); err == nil {
		t.Fatal("stale revision materialized")
	}
	shown, err := workflowCall(t, "show", id, "--json")
	data := workflowData(t, shown, err)
	state := data["state"].(map[string]any)
	if state["metadata"].(map[string]any)["native/keep"] != "preserved" || state["source_documents"].([]any)[0].(map[string]any)["content"] != source {
		t.Fatal("authoring lost metadata/source", state)
	}
	actor, _ := loadIdentity()
	branch := appendIssueCommandFixture(t, actor, Event{Kind: "workflow.revise", Subject: id, Parents: []string{first}, Intent: "revise", Body: string(raw)})
	heads := sortedUniqueStrings(edited["event_id"].(string), branch.ID)
	envelope, err = workflowCall(t, "resolve", id, "--expect", heads[0], "--expect", heads[1], "--input", input, "--json")
	workflowData(t, envelope, err)
	shown, err = workflowCall(t, "show", id, "--json")
	if workflowData(t, shown, err)["conflict"] != false {
		t.Fatal("generic resolution unavailable")
	}
}
func TestWorkflowRequestRejectsMissingPolicyAndWrongCodeRef(t *testing.T) {
	root := issueCommandTestRepo(t)
	opened := workflowOpenTest(t, workflowTestState())
	base := mustGitText(t, "rev-parse", "HEAD")
	envelope, err := workflowCall(t, "materialize", opened.ResourceID, "--json")
	workflowData(t, envelope, err)
	mustGit(t, "add", ".hn/pipelines/check.json")
	mustGit(t, "commit", "-q", "-m", "pipeline without policy")
	head := mustGitText(t, "rev-parse", "HEAD")
	proposal := workflowProposalTest(t, base, head)
	refs := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err = workflowCall(t, "request", opened.ResourceID, "--proposal", proposal.ID, "--json"); err == nil {
		t.Fatal("bypassed missing base policy")
	}
	if refs != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("policy refusal wrote event")
	}
	actor, _ := loadIdentity()
	writeTestPolicy(t, root, PolicyDocument{Version: policyVersion, Maintainers: []string{actor.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-q", "-m", "policy")
	base = mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "proposal change")
	head = mustGitText(t, "rev-parse", "HEAD")
	proposal = workflowProposalTest(t, base, head)
	mustGit(t, "update-ref", proposalRef(proposal.ID), base)
	refs = mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err = workflowCall(t, "request", opened.ResourceID, "--proposal", proposal.ID, "--json"); err == nil {
		t.Fatal("bypassed exact proposal code ref")
	}
	if refs != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("code-ref refusal wrote event")
	}
}
