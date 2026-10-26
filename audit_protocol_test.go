package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditAuthorizeFixture(t *testing.T, id *Identity, at string) *StoredEvent {
	t.Helper()
	if _, err := AuditAuthoritySet("refs/heads/main", "absent"); err != nil {
		t.Fatal(err)
	}
	auth, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "check", Source: AuditSource{Kind: "pipeline", Commit: at, Path: ".hn/pipelines/check.json"}}, id)
	if err != nil {
		t.Fatal(err)
	}
	return auth
}

func TestAuditNativeReplicationOldCodeKeylessAndNoAnchor(t *testing.T) {
	publisher, id, at := auditFixture(t)
	// An unreferenced target is deliberately outside the published main history.
	mustGit(t, "switch", "-qc", "old-target")
	if err := os.WriteFile("old-code", []byte("inspect me"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "old-code")
	mustGit(t, "commit", "-qm", "old code")
	target := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "switch", "-q", "main")
	mustGit(t, "branch", "-D", "old-target")
	auth := auditAuthorizeFixture(t, id, at)
	req, err := AuditRequest(target, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RecordAuditResult(req.ID, "", id, executionResult{Outcome: "failed", ExitCode: 1, Log: []byte("rule failed\n")}, []AuditFinding{{Key: "failure", Title: "Failure"}}, "host", "linux/amd64", "test")
	if err != nil {
		t.Fatal(err)
	}
	remote := filepath.Join(t.TempDir(), "project.git")
	receiver := filepath.Join(t.TempDir(), "reader")
	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", actorRef(id.Actor)+":"+actorRef(id.Actor))
	mustGit(t, "clone", "-q", "--no-local", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput("cat-file", "-e", target); err == nil {
		t.Fatal("target preexists replication")
	}
	selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Actors: []string{id.Actor}, Budgets: defaultReplicationBudgets()}
	received, err := runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	if !replicationOutcomeHasStatus(received.Outcomes, replicationActor, id.Actor, replicationPromoted) {
		t.Fatalf("not promoted: %#v", received.Outcomes)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("history: %d", len(events))
	}
	for _, original := range []*StoredEvent{auth, req, result} {
		found, err := resolveEvent(events, original.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(found.Payload, original.Payload) || !bytes.Equal(found.Signature, original.Signature) {
			t.Fatal("signed bytes changed")
		}
	}
	if content := mustGitText(t, "show", target+":old-code"); content != "inspect me" {
		t.Fatal(content)
	}
	if _, err := ValidateAuditExecution(req.ID, id.Actor, ""); err == nil {
		t.Fatal("import silently adopted authority")
	}
	if _, err := AuditAuthoritySet("refs/heads/main", "absent"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAuditExecution(req.ID, id.Actor, ""); err != nil {
		t.Fatal(err)
	}
	t.Logf("snapshot request pack %d bytes; 3 signed audit facts replicated and old target restored", len(req.Attachments["audit.pack"]))
}

func TestAuditStrictSignedShapeAndBoundedFindings(t *testing.T) {
	_, id, at := auditFixture(t)
	auth := auditAuthorizeFixture(t, id, at)
	key, _ := id.privateKey()
	raw := bytes.Replace(auth.Payload, []byte(`"scope":"project"`), []byte(`"scope":"project","surprise":true`), 1)
	if _, _, err := verifyEvent(raw, ed25519.Sign(key, raw)); err == nil {
		t.Fatal("unknown nested signed field")
	}
	raw = bytes.Replace(auth.Payload, []byte(`"scope":"project"`), []byte(`"scope":"project","scope":"project"`), 1)
	if _, _, err := verifyEvent(raw, ed25519.Sign(key, raw)); err == nil {
		t.Fatal("duplicate signed field")
	}
	req, err := AuditRequest(at, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	before := mustGitText(t, "rev-parse", actorRef(id.Actor))
	for _, findings := range [][]AuditFinding{{{Key: "x", Title: "x"}, {Key: "x", Title: "again"}}, {{Key: "x", Title: "x", Path: "../escape"}}, {{Key: "x", Title: "x", Body: strings.Repeat("x", (16<<10)+1)}}} {
		if _, err := RecordAuditResult(req.ID, "", id, executionResult{Outcome: "failed", ExitCode: 1, Log: []byte("fail")}, findings, "host", "linux", "test"); err == nil {
			t.Fatal("invalid findings")
		}
	}
	if _, err := RecordAuditResult(req.ID, "", id, executionResult{Outcome: "failed", ExitCode: 1, Log: make([]byte, maxRunLogSize+1)}, nil, "host", "linux", "test"); err == nil {
		t.Fatal("oversized log")
	}
	if got := mustGitText(t, "rev-parse", actorRef(id.Actor)); got != before {
		t.Fatal("ref changed on refusal")
	}
}

func TestAuditRevocationConflictAndReauthorization(t *testing.T) {
	_, id, at := auditFixture(t)
	auth := auditAuthorizeFixture(t, id, at)
	req, err := AuditRequest(at, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := AuditRevoke("check", []string{auth.ID}, "withdraw rule", "", id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAuditExecution(req.ID, id.Actor, ""); err == nil {
		t.Fatal("revoked rule executed")
	}
	if _, err := AuditRequest(at, "check", "", id); err == nil {
		t.Fatal("revoked rule requested")
	}
	next, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "check", Source: *auth.Event.Audit.Source, Expected: []string{revoked.ID}}, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAuditExecution(req.ID, id.Actor, ""); err == nil {
		t.Fatal("old request revived by new authorization")
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	// A separately observed owner authorization with no predecessor is concurrent.
	concurrent := *next
	concurrent.ID = eventID([]byte("concurrent"))
	fact := *next.Event.Audit
	fact.Parents = nil
	concurrent.Event.Audit = &fact
	a, err := AuditAuthorityShow()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := usableAuditRule(a, "check", append(events, concurrent)); err == nil {
		t.Fatal("conflict chosen by timestamp")
	}
}

func TestAuditForeignAuthorityHistoryCannotPoisonCurrentHeads(t *testing.T) {
	root, id, at := auditFixture(t)
	auth := auditAuthorizeFixture(t, id, at)
	req, err := AuditRequest(at, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	attacker := testIdentity(t, "Foreign owner")
	mustGit(t, "switch", "-qc", "foreign")
	policy, _, _, err := loadPolicy(at)
	if err != nil {
		t.Fatal(err)
	}
	policy.Audits.RuleOwners = []string{attacker.Actor}
	policy.Audits.Requesters = []string{attacker.Actor}
	policy.Audits.Executors = []string{attacker.Actor}
	writeTestPolicy(t, root, policy)
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "foreign self-appointed owner")
	foreign := mustGitText(t, "rev-parse", "HEAD")
	// Candidate self-authorization through the local writer is refused.
	if _, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "check", Source: *auth.Event.Audit.Source, PolicyRef: foreign}, attacker); err == nil {
		t.Fatal("candidate selected own authority")
	}
	// Native signed foreign observations are historical, never current authority.
	_, _, digest, err := loadPolicy(foreign)
	if err != nil {
		t.Fatal(err)
	}
	forged := newEvent(attacker, "audit.rule-authorize", 1, "")
	fact := *auth.Event.Audit
	fact.PolicyCommit = foreign
	fact.PolicyDigest = digest
	fact.Parents = nil
	forged.Audit = &fact
	attachments, err := auditAttachments(forged, nil)
	if err != nil {
		t.Fatal(err)
	}
	stored := appendRawSignedEventCommit(t, attacker, forged, "", attachments)
	revoke := newEvent(attacker, "audit.rule-revoke", 2, stored.ID)
	revoke.Audit = &AuditFact{Scope: fact.Scope, Rule: fact.Rule, PolicyCommit: foreign, PolicyDigest: digest, Parents: []string{auth.ID}, Reason: "poison"}
	attachments, err = auditAttachments(revoke, nil)
	if err != nil {
		t.Fatal(err)
	}
	appendRawSignedEventCommit(t, attacker, revoke, stored.Commit, attachments)
	// Also fork the policy history with identical public policy bytes and a real
	// current owner, to prove membership/digest alone cannot introduce new heads.
	mustGit(t, "switch", "-q", "main")
	samePolicyFork := mustGitText(t, "commit-tree", mustGitText(t, "rev-parse", at+"^{tree}"), "-m", "unrelated identical policy")
	forkOwner := newEvent(id, "audit.rule-authorize", 3, req.ID)
	same := *auth.Event.Audit
	same.PolicyCommit = samePolicyFork
	same.Parents = nil
	forkOwner.Audit = &same
	extras, err := auditAttachments(forkOwner, nil)
	if err != nil {
		t.Fatal(err)
	}
	appendRawSignedEventCommit(t, id, forkOwner, req.Commit, extras)
	events, err := collectEvents()
	if err != nil {
		t.Fatal("historical observations lost:", err)
	}
	if len(events) != 5 {
		t.Fatalf("events: %d", len(events))
	}
	if _, err := ValidateAuditExecution(req.ID, id.Actor, ""); err != nil {
		t.Fatal("foreign/forked authority poisoned active rule:", err)
	}
}

func TestAuditPackRejectsUnexpectedObjectsCorruptionAndMissingMembers(t *testing.T) {
	_, id, at := auditFixture(t)
	auth := auditAuthorizeFixture(t, id, at)
	pack := auth.Attachments["audit.pack"]
	for _, bad := range [][]byte{nil, pack[:len(pack)-1], append(append([]byte{}, pack...), 0), make([]byte, maxAuditPack+1)} {
		if err := importAuditPack("", bad, auth.Event); err == nil {
			t.Fatal("bad pack accepted")
		}
	}
	extra, err := gitInput([]byte("unrelated object"), nil, "hash-object", "-w", "--stdin")
	if err != nil {
		t.Fatal(err)
	}
	objects, err := auditDependencyObjectIDsAt("", auth.Event)
	if err != nil {
		t.Fatal(err)
	}
	objects = append(objects, strings.TrimSpace(string(extra)))
	junk, err := gitInput([]byte(strings.Join(objects, "\n")+"\n"), nil, "pack-objects", "--stdout", "--window=0", "--depth=0")
	if err != nil {
		t.Fatal(err)
	}
	if err := importAuditPack("", junk, auth.Event); err == nil {
		t.Fatal("unreachable injected object accepted")
	}
	if err := validateAuditAttachments(auth.Event, map[string][]byte{}); err == nil {
		t.Fatal("missing pack")
	}
	if err := validateAuditAttachments(auth.Event, map[string][]byte{"audit.pack": pack, "junk": {}}); err == nil {
		t.Fatal("undeclared member")
	}
}

func TestAuditFactsNeverSatisfyProposalEvidence(t *testing.T) {
	root, id, at := auditFixture(t)
	p, _, _, err := loadPolicy(at)
	if err != nil {
		t.Fatal(err)
	}
	p.Pipelines["check"] = PipelinePolicy{RequiredResults: 1, TrustedRunners: []string{id.Actor}}
	writeTestPolicy(t, root, p)
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "require real CI")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-qm", "proposal code")
	head := mustGitText(t, "rev-parse", "HEAD")
	proposal := appendPolicyTestProposal(t, id, base, head, "audit cannot grant CI")
	auditAuthorizeFixture(t, id, head)
	req, err := AuditRequest(head, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range []string{"passed", "failed"} {
		code := 0
		if outcome == "failed" {
			code = 1
		}
		result, err := RecordAuditResult(req.ID, "", id, executionResult{Outcome: outcome, ExitCode: code, Log: []byte(outcome)}, nil, "host", "linux", "test")
		if err != nil {
			t.Fatal(err)
		}
		events, err := collectEvents()
		if err != nil {
			t.Fatal(err)
		}
		evaluation, err := evaluateProposal(proposal, events)
		if err != nil {
			t.Fatal(err)
		}
		if evaluation.Ready || evaluation.Accepted || len(evaluation.Evidence) > 0 {
			t.Fatal("audit altered proposal readiness")
		}
		byID := map[string]StoredEvent{}
		for _, s := range events {
			byID[s.ID] = s
		}
		decision := StoredEvent{Event: Event{Kind: "proposal.decision", Actor: id.Actor, Policy: evaluation.PolicyDigest, Verdict: "accept", Evidence: []string{result.ID}}}
		if err := validateDecisionEvent(decision, *proposal, byID); err == nil {
			t.Fatal("audit accepted as CI evidence")
		}
	}
}

func TestAuditLocalAndFetchedForgedRelationshipParity(t *testing.T) {
	_, id, at := auditFixture(t)
	auth := auditAuthorizeFixture(t, id, at)
	e, err := nextEvent(id, "audit.request")
	if err != nil {
		t.Fatal(err)
	}
	e.Audit = &AuditFact{Scope: "project", Rule: "wrong-rule", PolicyCommit: at, PolicyDigest: auth.Event.Audit.PolicyDigest, Authorization: auth.ID, CodeCommit: at, DefinitionDigest: auth.Event.Audit.DefinitionDigest, Intent: "inspect"}
	before := mustGitText(t, "rev-parse", actorRef(id.Actor))
	if _, err := appendAudit(e, id, nil); err == nil {
		t.Fatal("local forged relationship")
	}
	byID := map[string]StoredEvent{auth.ID: *auth}
	if _, err := replicationEventDependency("", "", StoredEvent{Event: e}, byID, nil, nil, nil); err == nil {
		t.Fatal("fetched forged relationship")
	}
	e.Audit.Authorization = eventID([]byte("missing"))
	dependency, err := replicationEventDependency("", "", StoredEvent{Event: e}, byID, nil, nil, nil)
	if err != nil || dependency == nil {
		t.Fatal("missing authorization not classified", err)
	}
	if err := validateExactEventReferenceClosure([]StoredEvent{{Event: e}}); err == nil {
		t.Fatal("missing shallow/reference closure")
	}
	if mustGitText(t, "rev-parse", actorRef(id.Actor)) != before {
		t.Fatal("ref changed on refusal")
	}
}

func TestAuditOldEventEncodingPreserved(t *testing.T) {
	id := testIdentity(t, "old actor")
	e := newEvent(id, "issue.open", 1, "")
	e.Timestamp = "2026-10-26T00:00:00Z"
	e.Title = "old"
	raw, _, err := encodeAndSign(e, id)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["audit"]; ok {
		t.Fatal("old encoding gained audit member")
	}
}

func TestAuditFetchedHostileFactsStayQuarantined(t *testing.T) {
	for _, kind := range []string{"wrong-rule", "missing-rule", "missing-pack", "corrupt-pack", "extra-tree-member", "expanded-budget"} {
		t.Run(kind, func(t *testing.T) {
			publisher, id, at := auditFixture(t)
			auth := auditAuthorizeFixture(t, id, at)
			e, err := nextEvent(id, "audit.request")
			if err != nil {
				t.Fatal(err)
			}
			e.Audit = &AuditFact{Scope: "project", Rule: "check", PolicyCommit: at, PolicyDigest: auth.Event.Audit.PolicyDigest, Authorization: auth.ID, CodeCommit: at, DefinitionDigest: auth.Event.Audit.DefinitionDigest, Intent: "inspect"}
			switch kind {
			case "wrong-rule":
				e.Audit.Rule = "forged"
			case "missing-rule":
				e.Audit.Authorization = eventID([]byte("absent"))
			}
			attachments, err := auditAttachments(e, nil)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "missing-pack":
				delete(attachments, "audit.pack")
			case "corrupt-pack":
				attachments["audit.pack"][len(attachments["audit.pack"])-1] ^= 1
			case "extra-tree-member":
				attachments["unexpected"] = []byte("unexpected")
			}
			appendRawSignedEventCommit(t, id, e, auth.Commit, attachments)
			remote := filepath.Join(t.TempDir(), "project.git")
			receiver := filepath.Join(t.TempDir(), "reader")
			mustGit(t, "clone", "-q", "--bare", publisher, remote)
			mustGit(t, "remote", "add", "origin", remote)
			mustGit(t, "push", "-q", "origin", actorRef(id.Actor)+":"+actorRef(id.Actor))
			mustGit(t, "clone", "-q", "--no-local", remote, receiver)
			if err := os.Chdir(receiver); err != nil {
				t.Fatal(err)
			}
			budgets := defaultReplicationBudgets()
			if kind == "expanded-budget" {
				// Ordinary actor trees fit this quota; the expanded declared snapshot is
				// additional material and must be counted before unpack/promotion.
				listed := mustGitText(t, "--git-dir="+filepath.Join(remote), "rev-list", "--objects", actorRef(id.Actor))
				budgets.MaxObjects = int64(len(strings.Split(listed, "\n")))
			}
			selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Actors: []string{id.Actor}, Budgets: budgets}
			result, err := runReplicationTransaction(selection)
			if err != nil {
				t.Fatal(err)
			}
			if replicationOutcomeHasStatus(result.Outcomes, replicationActor, id.Actor, replicationPromoted) {
				t.Fatal("hostile fact promoted", kind)
			}
			assertRefAbsent(t, acceptedActorRef("origin", id.Actor))
			events, err := collectEvents()
			if err != nil || len(events) != 0 {
				t.Fatalf("quarantine leaked accepted visibility: %d %v", len(events), err)
			}
			if _, err := AuditAuthorityShow(); err == nil {
				t.Fatal("imported private anchor")
			}
		})
	}
}

func TestAuditWorkflowAuthorizationIsExplicitAndRevisionBound(t *testing.T) {
	_, id, at := auditFixture(t)
	if _, err := AuditAuthoritySet("refs/heads/main", "absent"); err != nil {
		t.Fatal(err)
	}
	raw := resourceState(t, "workflow", workflowTestState())
	workflow, err := MutateResource(ResourceMutation{Family: ResourceWorkflow, Intent: "open", State: raw})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuditRequest(at, "workflow-rule", "", id); err == nil {
		t.Fatal("publication implicitly authorized rule")
	}
	auth, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "workflow-rule", Source: AuditSource{Kind: "workflow", Revision: workflow.EventID}}, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuditRequest(at, "workflow-rule", "", id); err != nil {
		t.Fatal(err)
	}
	state := workflowTestState()
	state.Description = "unrelated publication edit"
	if _, err := MutateResource(ResourceMutation{Family: ResourceWorkflow, Intent: "revise", ID: workflow.ResourceID, Expected: []string{workflow.EventID}, State: resourceState(t, "workflow", state)}); err != nil {
		t.Fatal(err)
	}
	// The exact authorized definition stays valid until an explicit rule successor.
	request, err := AuditRequest(at, "workflow-rule", "", id)
	if err != nil {
		t.Fatal(err)
	}
	if request.Event.Audit.Authorization != auth.ID {
		t.Fatal("workflow edit silently replaced rule")
	}
	if _, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "stale", Source: AuditSource{Kind: "workflow", Revision: workflow.EventID}}, id); err == nil {
		t.Fatal("new authorization selected stale native revision")
	}
	for _, state := range []WorkflowState{{Schema: "hn.workflow/1", Name: "foreign", Title: "Foreign", SourceDocuments: []WorkflowSourceDocument{{Format: "actions-yaml", Content: "name: inert", Digest: eventID([]byte("name: inert"))}}}, {Schema: "hn.workflow/1", Name: "disabled", Title: "Disabled", Pipeline: workflowTestState().Pipeline}} {
		opened, err := MutateResource(ResourceMutation{Family: ResourceWorkflow, Intent: "open", State: resourceState(t, "workflow", state)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := AuditAuthorize(AuditAuthorizeOptions{Rule: state.Name, Source: AuditSource{Kind: "workflow", Revision: opened.EventID}}, id); err == nil {
			t.Fatal("non-runnable workflow authorized")
		}
	}
}

func TestAuditSnapshotPackExcludesUnrelatedHistoryAndSourceContent(t *testing.T) {
	_, id, at := auditFixture(t)
	if err := os.WriteFile("historic-unrelated", bytes.Repeat([]byte("z"), 1<<20), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "historic-unrelated")
	mustGit(t, "commit", "-qm", "unrelated history")
	historical := mustGitText(t, "rev-parse", "HEAD")
	oldBlob := mustGitText(t, "rev-parse", historical+":historic-unrelated")
	mustGit(t, "rm", "-q", "historic-unrelated")
	if err := os.WriteFile("unrelated-current", bytes.Repeat([]byte("a"), 1<<20), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "unrelated-current")
	mustGit(t, "commit", "-qm", "current tree")
	current := mustGitText(t, "rev-parse", "HEAD")
	currentBlob := mustGitText(t, "rev-parse", current+":unrelated-current")
	auth := auditAuthorizeFixture(t, id, at)
	objects, err := auditPackObjectIDs(auth.Attachments["audit.pack"], len(at))
	if err != nil {
		t.Fatal(err)
	}
	for _, oid := range []string{historical, oldBlob, currentBlob} {
		if stringListed(oid, objects) {
			t.Fatal("unrelated history/content in source policy pack", oid)
		}
	}
	if len(auth.Attachments["audit.pack"]) > 4096 {
		t.Fatal("source/policy path proof unexpectedly large")
	}
	req, err := AuditRequest(current, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	objects, err = auditPackObjectIDs(req.Attachments["audit.pack"], len(at))
	if err != nil {
		t.Fatal(err)
	}
	if !stringListed(currentBlob, objects) || stringListed(oldBlob, objects) || stringListed(historical, objects) {
		t.Fatal("target snapshot did not preserve exact snapshot-only semantics")
	}
}

func TestAuditSourceSymlinkAndUnauthorizedRolesRefuse(t *testing.T) {
	_, id, at := auditFixture(t)
	auth := auditAuthorizeFixture(t, id, at)
	stranger := testIdentity(t, "not an owner")
	if _, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "other", Source: *auth.Event.Audit.Source}, stranger); err == nil {
		t.Fatal("unlisted owner")
	}
	if _, err := AuditRequest(at, "check", "", stranger); err == nil {
		t.Fatal("unlisted requester")
	}
	req, err := AuditRequest(at, "check", "", id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAuditExecution(req.ID, stranger.Actor, ""); err == nil {
		t.Fatal("unlisted executor")
	}
	if err := os.Symlink(".hn/pipelines/check.json", "rule-link"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "rule-link")
	mustGit(t, "commit", "-qm", "symlink rule")
	commit := mustGitText(t, "rev-parse", "HEAD")
	if _, err := AuditAuthorize(AuditAuthorizeOptions{Rule: "link", Source: AuditSource{Kind: "git", Commit: commit, Path: "rule-link"}}, id); err == nil {
		t.Fatal("symlink source")
	}
	// The branch remains authoritative even on a candidate checkout.
	mustGit(t, "switch", "-qc", "candidate")
	if _, err := ValidateAuditExecution(req.ID, id.Actor, ""); err != nil {
		t.Fatal("checkout alone altered authority", err)
	}
}

func TestAuditSignedSchemasRejectPresentUnrelatedEmptyFields(t *testing.T) {
	_, id, at := auditFixture(t)
	auth := auditAuthorizeFixture(t, id, at)
	key, _ := id.privateKey()
	for _, raw := range [][]byte{
		bytes.Replace(auth.Payload, []byte(`"audit":`), []byte(`"title":"","audit":`), 1),
		bytes.Replace(auth.Payload, []byte(`"scope":"project"`), []byte(`"reason":null,"scope":"project"`), 1),
		bytes.Replace(auth.Payload, []byte(`"kind":"pipeline"`), []byte(`"revision":"","kind":"pipeline"`), 1),
	} {
		if _, _, err := verifyEvent(raw, ed25519.Sign(key, raw)); err == nil {
			t.Fatal("unrelated explicitly present empty field accepted", string(raw))
		}
	}
}
