package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDistributedPipelineRun(t *testing.T) {
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	t.Cleanup(func() {
		if err := os.Chdir(originalDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	seed := filepath.Join(root, "seed")
	remote := filepath.Join(root, "project.git")
	aliceDirectory := filepath.Join(root, "alice")
	bobDirectory := filepath.Join(root, "bob")
	if err := os.MkdirAll(filepath.Join(seed, ".hn", "pipelines"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(seed, ".hn", "actions"), 0o755); err != nil {
		t.Fatal(err)
	}
	hostMarker := filepath.Join(root, "host-only-marker")
	if err := os.WriteFile(hostMarker, []byte("sandbox must not see this"), 0o600); err != nil {
		t.Fatal(err)
	}
	pipeline := fmt.Sprintf(`{
  "version": "hn.pipeline/0",
  "steps": [
    {
      "name": "Custom repository action",
      "command": "./.hn/actions/pass",
      "args": ["expected-argument", %q],
      "timeoutSeconds": 30
    }
  ]
}`, hostMarker)
	action := "#!/bin/sh\nset -eu\nif [ -e \"$2\" ]; then visibility=visible; else visibility=hidden; fi\nprintf 'custom action: %s at %s; host-marker-%s\\n' \"$1\" \"$HN_COMMIT\" \"$visibility\"\n"
	writeTestPolicy(t, seed, PolicyDocument{
		Version:     policyVersion,
		Maintainers: []string{strings.Repeat("a", 64)},
		Proposals:   ProposalPolicy{RequiredAccepts: 1},
		Pipelines:   map[string]PipelinePolicy{},
	})
	if err := os.WriteFile(filepath.Join(seed, ".hn", "pipelines", "check.json"), []byte(pipeline), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, ".hn", "actions", "pass"), []byte(action), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "-C", seed, "init", "-q", "-b", "main")
	mustGit(t, "-C", seed, "config", "user.name", "Seed")
	mustGit(t, "-C", seed, "config", "user.email", "seed@hn.invalid")
	mustGit(t, "-C", seed, "add", ".hn")
	mustGit(t, "-C", seed, "commit", "-q", "-m", "pipeline")
	mustGit(t, "clone", "-q", "--bare", seed, remote)
	mustGit(t, "clone", "-q", remote, aliceDirectory)
	mustGit(t, "clone", "-q", remote, bobDirectory)

	if err := os.Chdir(aliceDirectory); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "config", "user.name", "Alice")
	mustGit(t, "config", "user.email", "alice@hn.invalid")
	base := mustGitText(t, "rev-parse", "main")
	mustGit(t, "switch", "-q", "-c", "feature")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "feature")
	head := mustGitText(t, "rev-parse", "HEAD")
	alice, _, err := createIdentity("Alice")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := nextEvent(alice, "proposal.open")
	if err != nil {
		t.Fatal(err)
	}
	proposal.Title = "Run custom action"
	proposal.Base = base
	proposal.Head = head
	storedProposal, err := appendEvent(proposal, alice)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(storedProposal.ID, head); err != nil {
		t.Fatal(err)
	}
	beforePublication := mustGitText(t, "ls-remote", remote, "refs/hn/*")
	requestOutput, err := captureTestOutput(t, func() error {
		return cmdRunRequest([]string{storedProposal.ID, "check"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestOutput, "Recorded locally; run 'hn sync REMOTE' to publish this request") {
		t.Fatal("run request omitted its unpublished state", requestOutput)
	}
	if afterPublication := mustGitText(t, "ls-remote", remote, "refs/hn/*"); afterPublication != beforePublication {
		t.Fatal("request creation unexpectedly published facts")
	}
	aliceEvents, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	requests := runRequests(aliceEvents)
	if len(requests) != 1 {
		t.Fatalf("run requests = %d, want 1", len(requests))
	}
	request := requests[0]
	if err := cmdSync(nil); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(bobDirectory); err != nil {
		t.Fatal(err)
	}
	bob, _, err := createIdentity("Bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := cmdSync(nil); err != nil {
		t.Fatal(err)
	}
	if err := cmdRunExecute([]string{request.ID, "--backend", "host"}); err == nil || !strings.Contains(err.Error(), "host execution requires") {
		t.Fatalf("run without host-execution opt-in returned %v", err)
	}
	ran, err := runnerOnce(runnerOptions{
		Remote:        "origin",
		Pipeline:      "check",
		AcceptedActor: bob.Actor,
		Backend:       hostBackend{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ran {
		t.Fatal("runner accepted a request from an actor outside its policy")
	}
	ran, err = runnerOnce(runnerOptions{
		Remote:        "origin",
		Pipeline:      "check",
		AcceptedActor: alice.Actor,
		Backend:       hostBackend{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("runner did not discover the accepted pending request")
	}
	bobEvents, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	results := currentRunResults(bobEvents, request.ID)
	if len(results) != 1 {
		t.Fatalf("run results = %d, want 1", len(results))
	}
	result := results[0]
	if result.Event.Outcome != "passed" || result.Event.ExitCode != 0 {
		t.Fatalf("result = %s exit %d, want passed exit 0", result.Event.Outcome, result.Event.ExitCode)
	}
	log := string(result.Attachments["log.txt"])
	if !strings.Contains(log, "custom action: expected-argument at "+head) {
		t.Fatalf("verified log does not contain custom action output:\n%s", log)
	}
	if !strings.Contains(log, "host-marker-visible") {
		t.Fatalf("host backend did not observe the host marker:\n%s", log)
	}
	if result.Event.Log != eventID(result.Attachments["log.txt"]) {
		t.Fatal("result log digest does not match attachment")
	}
	if sandboxUsableForTest(t) {
		if err := cmdRunExecute([]string{request.ID, "--backend", "sandbox", "--rerun"}); err != nil {
			t.Fatal(err)
		}
		bobEvents, err = collectEvents()
		if err != nil {
			t.Fatal(err)
		}
		results = currentRunResults(bobEvents, request.ID)
		if len(results) != 1 {
			t.Fatalf("sandbox results = %d, want 1", len(results))
		}
		result = results[0]
		sandboxLog := string(result.Attachments["log.txt"])
		if result.Event.Outcome != "passed" || result.Event.Backend != "sandbox" || !strings.Contains(sandboxLog, "Backend sandbox") || !strings.Contains(sandboxLog, "host-marker-hidden") {
			t.Fatalf("sandbox result was not a verified pass: %#v", result.Event)
		}
	}
	if err := cmdSync(nil); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(aliceDirectory); err != nil {
		t.Fatal(err)
	}
	if err := cmdSync(nil); err != nil {
		t.Fatal(err)
	}
	finalEvents, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	finalResults := currentRunResults(finalEvents, request.ID)
	if len(finalResults) != 1 || finalResults[0].ID != result.ID {
		t.Fatalf("Alice did not receive Bob's result %s", result.ID)
	}
}

func sandboxUsableForTest(t *testing.T) bool {
	t.Helper()
	backend := newBubblewrapBackend()
	if err := backend.Available(); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	root := t.TempDir()
	environment := runnerEnvironment("/home/hn", "/tmp", "test", sandboxPath())
	return backend.RunStep(ctx, root, PipelineStep{Name: "Probe", Command: "true"}, environment, io.Discard) == nil
}
