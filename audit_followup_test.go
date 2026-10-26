package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditPublicFollowUpInterruptedCreateRecoveryAndEditedIssue(t *testing.T) {
	binary := buildOperationalBinary(t)
	root, id, _ := auditFixture(t)
	definition := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "observations", Command: "sh", Args: []string{"-c", `i=1; while test "$i" -le 8; do printf 'HN_AUDIT_FINDING {"key":"item-%s","title":"Finding %s"}\n' "$i" "$i"; i=$((i+1)); done; exit 1`}}}}
	raw, _ := json.Marshal(definition)
	if err := os.WriteFile(filepath.Join(root, ".hn/pipelines/check.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-qm", "rules")
	auditPublic(t, binary, root, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	auditPublic(t, binary, root, true, "rule", "authorize", "check", "--source", "pipeline", "--at", "HEAD", "--json")
	request := auditPublicID(t, auditPublic(t, binary, root, true, "request", "--at", "HEAD", "--rule", "check", "--json"))
	result := auditPublicID(t, auditPublic(t, binary, root, true, "execute", request, "--backend", "host", "--allow-unsafe-host-execution", "--json"))
	before := mustGitText(t, "rev-parse", actorRef(id.Actor))
	cmd := exec.Command(binary, "audit", "follow-up", result, "--operation", "interrupted", "--json")
	cmd.Dir = root
	cmd.Env = operationalTestEnvironment(t.TempDir())
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Observe the production actor-ref publication boundary, then kill the client
	// before its response. No fabricated event or hook replaces issue creation.
	deadline := time.Now().Add(20 * time.Second)
	changed := false
	for time.Now().Before(deadline) {
		probe := exec.Command("git", "rev-parse", actorRef(id.Actor))
		probe.Dir = root
		head, err := probe.Output()
		if err == nil && strings.TrimSpace(string(head)) != before {
			changed = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if !changed {
		t.Fatalf("never observed native creation: %s", output.String())
	}
	events, err := collectIssueEventsForAuditTest()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || len(events) >= 8 {
		t.Fatalf("interruption was not partial: %d", len(events))
	}
	recovered := auditPublic(t, binary, root, true, "follow-up", result, "--operation", "interrupted", "--json")
	var associations auditFollowUpResult
	if err := json.Unmarshal(recovered, &associations); err != nil || len(associations.Associations) != 8 {
		t.Fatal(string(recovered), err)
	}
	events, err = collectIssueEventsForAuditTest()
	if err != nil || len(events) != 8 {
		t.Fatal("issue flood", len(events), err)
	}
	// Preserve an operator's later edits while returning the opening association.
	issue := associations.Associations[0].IssueID
	changedState := IssueState{Title: "Operator changed title", Body: "Keep my edits", Status: "closed"}
	payload := issuePublicState(t, changedState)
	issuePublicCall(t, binary, root, payload, "", "revise", issue, "--expect", issue, "--input", "-", "--json")
	auditPublic(t, binary, root, true, "follow-up", result, "--operation", "interrupted", "--json")
	shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, root, nil, "", "show", issue, "--json"))
	if shown.State == nil || shown.State.Title != changedState.Title || shown.State.Body != changedState.Body || shown.State.Status != "closed" || shown.RevisionCount != 2 {
		t.Fatal(shown)
	}
	// A different operation that only returns existing associations is still bound
	// durably before returning, even though it needs no issue-create receipt.
	auditPublic(t, binary, root, true, "follow-up", result, "--operation", "existing-only", "--json")
	before = mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	auditPublic(t, binary, root, false, "follow-up", result, "--finding", "item-1", "--operation", "existing-only", "--json")
	auditPublic(t, binary, root, false, "follow-up", result, "--finding", "absent", "--operation", "new", "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("refusal mutated issues")
	}
}

func TestAuditFollowUpUnsafePrivateIntentRefuses(t *testing.T) {
	root, id, at := auditFixture(t)
	auditAuthorizeFixture(t, id, at)
	request, err := AuditRequest(at, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	result, err := executeAudit(context.Background(), request.ID, "", id, hostBackend{})
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, ".git", "hn", "audit", "followups")
	if err := os.Symlink(t.TempDir(), directory); err != nil {
		t.Fatal(err)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err := followUpAudit(auditOptions{id: result.ID}, id); err == nil {
		t.Fatal("accepted symlinked intent directory")
	}
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("unsafe path mutated signed state")
	}
}
