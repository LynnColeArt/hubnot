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

func auditPublic(t *testing.T, binary, root string, ok bool, args ...string) json.RawMessage {
	t.Helper()
	cmd := exec.Command(binary, append([]string{"audit"}, args...)...)
	cmd.Dir = root
	cmd.Env = operationalTestEnvironment(t.TempDir())
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	out, err := cmd.Output()
	var envelope struct {
		Schema string
		OK     bool
		Data   json.RawMessage
		Error  any
	}
	if json.Unmarshal(out, &envelope) != nil || envelope.Schema != "hn.audit/1" || envelope.OK != ok || (err == nil) != ok {
		t.Fatalf("audit %v: %v %s", args, err, out)
	}
	return envelope.Data
}
func auditPublicID(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v struct{ ID string }
	if err := json.Unmarshal(raw, &v); err != nil || !validEventID(v.ID) {
		t.Fatalf("bad ID: %s %v", raw, err)
	}
	return v.ID
}

func TestAuditPublicExplicitExecutionFollowUpAndReplication(t *testing.T) {
	binary := buildOperationalBinary(t)
	t.Run("ordinary", func(t *testing.T) { auditPublicExecutionScenario(t, binary) })
	native := filepath.Join(t.TempDir(), "hn-native")
	build := exec.Command("go", "build", "-tags", "nogithub", "-o", native, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	t.Run("nogithub", func(t *testing.T) { auditPublicExecutionScenario(t, native) })
}
func auditPublicExecutionScenario(t *testing.T, binary string) {
	root, id, at := auditFixture(t)
	if err := os.WriteFile(filepath.Join(root, "code.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "code.txt")
	mustGit(t, "commit", "-qm", "old audited code")
	at = mustGitText(t, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "code.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	// The authorized source is separate from the audited snapshot and emits inert findings.
	definition := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "audit", Command: "sh", Args: []string{"-c", `test "$HN_COMMIT" != ""; test "$(cat code.txt)" = old || exit 41; printf '%s\n' 'HN_AUDIT_FINDING {"key":"Missing-Test","title":"Add coverage","body":"observed","path":"code.go","line":1}'; exit 1`}}}}
	raw, _ := json.Marshal(definition)
	if err := os.WriteFile(filepath.Join(root, ".hn/pipelines/check.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn", "code.txt")
	mustGit(t, "commit", "-qm", "rules")
	source := mustGitText(t, "rev-parse", "HEAD")
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	auditPublic(t, binary, root, false, "request", "--at", at, "--rule", "check", "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("refusal mutated facts")
	}
	auditPublic(t, binary, root, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	auth := auditPublicID(t, auditPublic(t, binary, root, true, "rule", "authorize", "check", "--source", "pipeline", "--at", source, "--json"))
	request := auditPublicID(t, auditPublic(t, binary, root, true, "request", "--at", at, "--rule", "check", "--operation", "request-once", "--json"))
	if got := auditPublicID(t, auditPublic(t, binary, root, true, "request", "--at", at, "--rule", "check", "--operation", "request-once", "--json")); got != request {
		t.Fatal("request retry duplicated")
	}
	auditPublic(t, binary, root, false, "request", "--at", source, "--rule", "check", "--operation", "request-once", "--json")
	auditPublic(t, binary, root, false, "execute", request, "--backend", "host", "--json")
	mustGit(t, "commit", "--allow-empty", "-qm", "safe code advance")
	result := auditPublicID(t, auditPublic(t, binary, root, true, "execute", request, "--backend", "host", "--allow-unsafe-host-execution", "--json"))
	shown := string(auditPublic(t, binary, root, true, "show", result, "--json"))
	if !strings.Contains(shown, `"outcome":"failed"`) || !strings.Contains(shown, at) || !strings.Contains(shown, `"key":"missing-test"`) {
		t.Fatal(shown)
	}
	if output := runOperationalCommandFailure(t, binary, root, "run", "show", result); !strings.Contains(output, "not a run request") {
		t.Fatal(output)
	}
	// No implicit issue creation; losing follow-up output is recovered from signed issue operations.
	if events, err := collectIssueEventsForAuditTest(); err != nil || len(events) != 0 {
		t.Fatal("execute opened issues", err)
	}
	first := auditPublic(t, binary, root, true, "follow-up", result, "--operation", "triage-once", "--json")
	again := auditPublic(t, binary, root, true, "follow-up", result, "--operation", "triage-once", "--json")
	var one, two struct {
		Associations []struct {
			IssueID string `json:"issue_id"`
		}
	}
	_ = json.Unmarshal(first, &one)
	_ = json.Unmarshal(again, &two)
	if len(one.Associations) != 1 || len(two.Associations) != 1 || one.Associations[0].IssueID != two.Associations[0].IssueID {
		t.Fatal(string(first), string(again))
	}
	// A new result with the same finding returns the original association.
	next := auditPublicID(t, auditPublic(t, binary, root, true, "execute", request, "--backend", "host", "--allow-unsafe-host-execution", "--json"))
	auditPublic(t, binary, root, false, "follow-up", next, "--operation", "triage-once", "--json")
	dedup := string(auditPublic(t, binary, root, true, "follow-up", next, "--json"))
	if !strings.Contains(dedup, one.Associations[0].IssueID) {
		t.Fatal(dedup)
	}
	auditPublic(t, binary, root, true, "rule", "revoke", "check", "--expect", auth, "--reason", "retired", "--json")
	auditPublic(t, binary, root, false, "execute", request, "--backend", "host", "--allow-unsafe-host-execution", "--json")
	// Actual production Git transport; private authority and identities do not replicate.
	remote := filepath.Join(t.TempDir(), "remote.git")
	mustGit(t, "init", "--bare", "-q", remote)
	mustGit(t, "remote", "add", "origin", remote)
	runOperationalCommand(t, binary, root, "sync")
	clone := filepath.Join(t.TempDir(), "reader")
	mustGit(t, "clone", "-q", remote, clone)
	runOperationalCommand(t, binary, clone, "replication", "select", "origin", "--actor", id.Actor)
	runOperationalCommand(t, binary, clone, "sync")
	if !strings.Contains(string(auditPublic(t, binary, clone, true, "show", result, "--json")), result) {
		t.Fatal("missing replicated result")
	}
	auditPublic(t, binary, clone, true, "logs", result, "--json")
	auditPublic(t, binary, clone, false, "authority", "show", "--json")
	auditPublic(t, binary, clone, false, "execute", request, "--backend", "host", "--allow-unsafe-host-execution", "--json")
}
func collectIssueEventsForAuditTest() ([]StoredEvent, error) {
	events, _, err := collectIssueEvents()
	filtered := []StoredEvent{}
	for _, s := range events {
		if isIssueKind(s.Event.Kind) {
			filtered = append(filtered, s)
		}
	}
	return filtered, err
}

func TestAuditPublicCandidateStalePolicyAndRoleRefusals(t *testing.T) {
	binary := buildOperationalBinary(t)
	root, id, at := auditFixture(t)
	auditPublic(t, binary, root, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	auditPublic(t, binary, root, true, "rule", "authorize", "check", "--source", "pipeline", "--at", at, "--json")
	request := auditPublicID(t, auditPublic(t, binary, root, true, "request", "--at", at, "--rule", "check", "--json"))
	// Refuse missing authorization, unowned rules and non-native rule sources.
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	auditPublic(t, binary, root, false, "request", "--at", at, "--rule", "missing", "--json")
	auditPublic(t, binary, root, false, "rule", "authorize", "other", "--source", "foreign-yaml", "--at", at, "--json")
	auditPublic(t, binary, root, false, "rule", "authorize", "other", "--source", "workflow", "--workflow", eventID([]byte("foreign-only")), "--json")
	auditPublic(t, binary, root, false, "request", "--at", at, "--rule", "check", "--actor", strings.Repeat("b", 64), "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("refusal changed signed state")
	}
	p, _, _, err := loadPolicy(at)
	if err != nil {
		t.Fatal(err)
	}
	p.Audits.RuleOwners = []string{strings.Repeat("b", 64)}
	p.Audits.Executors = []string{strings.Repeat("b", 64)}
	writeTestPolicy(t, root, p)
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "withdraw owner and executor")
	auditPublic(t, binary, root, false, "rule", "authorize", "other", "--source", "pipeline", "--at", at, "--path", ".hn/pipelines/check.json", "--json")
	auditPublic(t, binary, root, false, "execute", request, "--backend", "host", "--allow-unsafe-host-execution", "--json")
	// Current main withdraws all audit opt-in. A checked-out candidate with the
	// old roles, and even an exact old policy assertion, cannot restore it.
	p.Audits = nil
	writeTestPolicy(t, root, p)
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "disable audit")
	mustGit(t, "switch", "-qc", "candidate", at)
	before = mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	auditPublic(t, binary, root, false, "rule", "authorize", "candidate", "--source", "git", "--at", at, "--path", ".hn/pipelines/check.json", "--policy-ref", at, "--json")
	auditPublic(t, binary, root, false, "request", "--at", at, "--rule", "check", "--policy-ref", at, "--json")
	auditPublic(t, binary, root, false, "execute", request, "--policy-ref", at, "--backend", "host", "--allow-unsafe-host-execution", "--json")
	auditPublic(t, binary, root, true, "show", request, "--json")
	auditPublic(t, binary, root, true, "rule", "show", "check", "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("candidate changed signed state")
	}
	if id.Actor == "" {
		t.Fatal("missing actor")
	}
}

func TestAuditPublicRequestConcurrentOperationAndLostAcknowledgement(t *testing.T) {
	binary := buildOperationalBinary(t)
	root, id, at := auditFixture(t)
	auditPublic(t, binary, root, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	auditPublic(t, binary, root, true, "rule", "authorize", "check", "--source", "pipeline", "--at", at, "--json")
	release, err := lockAuditRequestOperation(id.Actor, "busy")
	if err != nil {
		t.Fatal(err)
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	auditPublic(t, binary, root, false, "request", "--at", at, "--rule", "check", "--operation", "busy", "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("guard conflict published")
	}
	release()
	var commands [2]*exec.Cmd
	var outputs [2]bytes.Buffer
	for i := range commands {
		commands[i] = exec.Command(binary, "audit", "request", "--at", at, "--rule", "check", "--operation", "concurrent", "--json")
		commands[i].Dir = root
		commands[i].Env = operationalTestEnvironment(t.TempDir())
		commands[i].Stdout = &outputs[i]
		if err := commands[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	successes := 0
	for i, cmd := range commands {
		err := cmd.Wait()
		var response struct {
			OK    bool
			Error *struct{ Message string }
		}
		if json.Unmarshal(outputs[i].Bytes(), &response) != nil {
			t.Fatal(outputs[i].String(), err)
		}
		if err == nil && response.OK {
			successes++
		} else if response.Error == nil || !strings.Contains(response.Error.Message, "retry") {
			t.Fatal(outputs[i].String(), err)
		}
	}
	if successes == 0 {
		t.Fatal("both concurrent invocations failed")
	}
	request := auditPublicID(t, auditPublic(t, binary, root, true, "request", "--at", at, "--rule", "check", "--operation", "concurrent", "--json"))
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, s := range events {
		if s.Event.Operation == "concurrent" {
			count++
			if s.ID != request {
				t.Fatal("wrong request recovery")
			}
		}
	}
	if count != 1 {
		t.Fatal("concurrent operation duplicated", count)
	}
	// Discard the acknowledgement after actual append, then recover from history.
	cmd := exec.Command(binary, "audit", "request", "--at", at, "--rule", "check", "--operation", "lost-ack", "--json")
	cmd.Dir = root
	cmd.Env = operationalTestEnvironment(t.TempDir())
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	before = mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	auditPublic(t, binary, root, true, "request", "--at", at, "--rule", "check", "--operation", "lost-ack", "--json")
	auditPublic(t, binary, root, false, "request", "--at", at, "--rule", "check", "--intent", "changed", "--operation", "lost-ack", "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("retry or conflict appended")
	}
	if got := mustGitText(t, "for-each-ref", "--format=%(refname)", "refs/hn/local"); got != "" {
		t.Fatal("committed private guard ref", got)
	}
}

func TestAuditPublicOperatorReferenceHookAndInertSources(t *testing.T) {
	binary := buildOperationalBinary(t)
	root, id, _ := auditFixture(t)
	pipelineMarker := filepath.Join(root, "pipeline-executed")
	definition := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "inert until execute", Command: "touch", Args: []string{pipelineMarker}}}}
	raw, _ := json.Marshal(definition)
	if err := os.WriteFile(filepath.Join(root, ".hn/pipelines/check.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".hn", "source-hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	sourceHook := []byte("#!/bin/sh\ntouch source-hook-executed\n")
	if err := os.WriteFile(filepath.Join(root, ".hn/source-hooks/post-checkout"), sourceHook, 0755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-qm", "inert rules and source hooks")
	auditPublic(t, binary, root, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	auditPublic(t, binary, root, true, "rule", "authorize", "check", "--source", "pipeline", "--at", "HEAD", "--json")
	hook := `#!/bin/sh
if test "$1" != prepared; then exit 0; fi
while read old new ref; do
 case "$ref" in refs/hn/actors/*)
  printf '%s\n' "$ref" >> .git/operator-hook-marker
  if test -e .git/refuse-audit; then exit 1; fi
 esac
done
exit 0
`
	if err := os.WriteFile(filepath.Join(root, ".git/hooks/reference-transaction"), []byte(hook), 0755); err != nil {
		t.Fatal(err)
	}
	request := auditPublicID(t, auditPublic(t, binary, root, true, "request", "--at", "HEAD", "--rule", "check", "--operation", "hook-request", "--json"))
	marker := filepath.Join(root, ".git/operator-hook-marker")
	got, err := os.ReadFile(marker)
	if err != nil || strings.Count(string(got), "refs/hn/actors/") != 1 {
		t.Fatal("real fact hook absent or private guard ran it", string(got), err)
	}
	auditPublic(t, binary, root, true, "show", request, "--json")
	auditPublic(t, binary, root, true, "list", "--json")
	auditPublic(t, binary, root, true, "request", "--at", "HEAD", "--rule", "check", "--operation", "hook-request", "--json")
	after, _ := os.ReadFile(marker)
	if !bytes.Equal(got, after) {
		t.Fatal("read or replay ran an operator hook")
	}
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	reject := filepath.Join(root, ".git/refuse-audit")
	if err := os.WriteFile(reject, []byte("reject"), 0600); err != nil {
		t.Fatal(err)
	}
	auditPublic(t, binary, root, false, "request", "--at", "HEAD", "--rule", "check", "--operation", "rejected-request", "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("hook refusal changed accepted refs")
	}
	if err := os.Remove(reject); err != nil {
		t.Fatal(err)
	}
	auditPublic(t, binary, root, true, "request", "--at", "HEAD", "--rule", "check", "--operation", "rejected-request", "--json")
	remote := filepath.Join(t.TempDir(), "inert.git")
	mustGit(t, "init", "--bare", "-q", remote)
	mustGit(t, "remote", "add", "origin", remote)
	runOperationalCommand(t, binary, root, "sync")
	receiver := filepath.Join(t.TempDir(), "reader")
	mustGit(t, "clone", "-q", remote, receiver)
	runOperationalCommand(t, binary, receiver, "replication", "select", "origin", "--actor", id.Actor)
	runOperationalCommand(t, binary, receiver, "sync")
	auditPublic(t, binary, receiver, true, "show", request, "--json")
	for _, path := range []string{pipelineMarker, filepath.Join(root, "source-hook-executed"), filepath.Join(receiver, "source-hook-executed")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("inert source executed", path, err)
		}
	}
}

func TestAuditPublicInvalidUnicodeRefusesBeforeMutation(t *testing.T) {
	binary := buildOperationalBinary(t)
	root, _, at := auditFixture(t)
	auditPublic(t, binary, root, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	auditPublic(t, binary, root, true, "rule", "authorize", "check", "--source", "pipeline", "--at", at, "--json")
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	invalid := string([]byte{'b', 0xff})
	auditPublic(t, binary, root, false, "request", "--at", at, "--rule", "check", "--operation", invalid, "--json")
	auditPublic(t, binary, root, false, "rule", "authorize", "other", "--source", "git", "--at", at, "--path", invalid, "--json")
	if before != mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn") {
		t.Fatal("invalid Unicode mutated signed facts")
	}
	if _, err := os.Stat(filepath.Join(root, ".git/hn/audit/requests")); !os.IsNotExist(err) {
		t.Fatal("invalid Unicode wrote private intent", err)
	}
}
