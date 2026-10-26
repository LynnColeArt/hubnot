package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

func auditPrivateState(t *testing.T, repository string) string {
	t.Helper()
	directory := filepath.Join(repository, ".git", "hn", "audit")
	if _, err := os.Lstat(directory); os.IsNotExist(err) {
		return "absent"
	}
	var snapshot bytes.Buffer
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&snapshot, "%s %s\n", relative, info.Mode())
		if !entry.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(&snapshot, "%x\n", raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot.String()
}

func TestAuditPublicFollowUpNormalizedCollisionPreservesState(t *testing.T) {
	binary := buildOperationalBinary(t)
	for _, collision := range []struct{ name, first, second string }{{"case", "X", "x"}, {"whitespace", " x ", "x"}} {
		t.Run(collision.name, func(t *testing.T) {
			root, id, at := auditFixture(t)
			auditAuthorizeFixture(t, id, at)
			request, err := AuditRequest(at, "check", "", id)
			if err != nil {
				t.Fatal(err)
			}
			// These raw-unique keys are valid signed historical facts. The earlier
			// ordinary finding proves preflight covers the complete immutable selection.
			findings := []AuditFinding{{Key: "a-safe", Title: "Unambiguous finding"}, {Key: collision.first, Title: "First spelling", Body: "First body"}, {Key: collision.second, Title: "Second spelling", Body: "Different body"}}
			result, err := RecordAuditResult(request.ID, "", id, executionResult{Outcome: "failed", ExitCode: 1, Log: []byte("accepted signed observation")}, findings, "host", "linux/amd64", "regression-fixture")
			if err != nil {
				t.Fatal("valid raw-unique signed result refused", err)
			}
			remote := filepath.Join(t.TempDir(), "collision.git")
			mustGit(t, "init", "--bare", "-q", remote)
			mustGit(t, "remote", "add", "origin", remote)
			runOperationalCommand(t, binary, root, "sync")
			receiver := filepath.Join(t.TempDir(), "reader")
			mustGit(t, "clone", "-q", remote, receiver)
			runOperationalCommand(t, binary, receiver, "replication", "select", "origin", "--actor", id.Actor)
			runOperationalCommand(t, binary, receiver, "sync")
			runOperationalCommand(t, binary, receiver, "init", "--name", "Follow-up reader")
			for _, repository := range []struct{ name, path string }{{"local", root}, {"imported", receiver}} {
				t.Run(repository.name, func(t *testing.T) {
					shown := auditPublic(t, binary, repository.path, true, "show", result.ID, "--json")
					var original auditRecord
					if err := json.Unmarshal(shown, &original); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(original.Event.Audit.Findings, findings) {
						t.Fatal("signed raw finding keys changed during admission")
					}
					refsBefore := runOperationalGit(t, repository.path, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes")
					issuesBefore := issuePublicData[publicIssuePage](t, issuePublicCall(t, binary, repository.path, nil, "", "list", "--json")).Total
					privateBefore := auditPrivateState(t, repository.path)
					attempts := [][]string{
						{"--operation", "collision-all"},
						{"--finding", "x", "--operation", "collision-selected"},
						{"--finding", collision.first, "--operation", "collision-raw-selected"},
						{"--operation", "collision-all"},
					}
					for _, options := range attempts {
						args := append([]string{"audit", "follow-up", result.ID, "--json"}, options...)
						command := exec.Command(binary, args...)
						command.Dir = repository.path
						command.Env = operationalTestEnvironment(t.TempDir())
						out, commandErr := command.Output()
						var response struct {
							Schema string
							OK     bool
							Error  *struct{ Code, Message string }
						}
						if err := json.Unmarshal(out, &response); err != nil {
							t.Fatal(string(out), err)
						}
						if commandErr == nil || response.OK || response.Error == nil || response.Error.Code != "audit_refused" || !strings.Contains(response.Error.Message, "ambiguous") || !strings.Contains(response.Error.Message, "normalized") {
							t.Errorf("expected actionable normalized ambiguity refusal for %v: exit=%v response=%s", options, commandErr, out)
						}
						if refsAfter := runOperationalGit(t, repository.path, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes"); refsAfter != refsBefore {
							t.Errorf("ambiguous selection changed accepted actor refs for %v", options)
						}
						if issuesAfter := issuePublicData[publicIssuePage](t, issuePublicCall(t, binary, repository.path, nil, "", "list", "--json")).Total; issuesAfter != issuesBefore {
							t.Errorf("ambiguous selection changed issue count: before=%d after=%d", issuesBefore, issuesAfter)
						}
						if privateAfter := auditPrivateState(t, repository.path); privateAfter != privateBefore {
							t.Errorf("ambiguous selection changed private intent state for %v", options)
						}
						if after := auditPublic(t, binary, repository.path, true, "show", result.ID, "--json"); !bytes.Equal(shown, after) {
							t.Error("refusal changed the immutable signed result")
						}
					}
				})
			}
		})
	}
}
