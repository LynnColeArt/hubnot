package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const (
	maxAuditPayload  = 2 << 20
	maxAuditPack     = 64 << 20
	maxAuditObjects  = 100000
	maxAuditObject   = 16 << 20
	maxAuditExpanded = 128 << 20
)

type AuditSource struct {
	Kind     string `json:"kind"`
	Commit   string `json:"commit,omitempty"`
	Path     string `json:"path,omitempty"`
	Revision string `json:"revision,omitempty"`
}

type AuditFinding struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	Path  string `json:"path,omitempty"`
	Line  int    `json:"line,omitempty"`
}

// AuditFact is the closed observational protocol. No audit event is a run or a
// proposal review/decision. Definition contains exact inert runnable JSON bytes.
type AuditFact struct {
	Scope            string         `json:"scope"`
	Rule             string         `json:"rule"`
	PolicyCommit     string         `json:"policyCommit"`
	PolicyDigest     string         `json:"policyDigest"`
	Parents          []string       `json:"parents,omitempty"`
	Source           *AuditSource   `json:"source,omitempty"`
	Definition       string         `json:"definition,omitempty"`
	DefinitionDigest string         `json:"definitionDigest,omitempty"`
	Reason           string         `json:"reason,omitempty"`
	Authorization    string         `json:"authorization,omitempty"`
	CodeCommit       string         `json:"codeCommit,omitempty"`
	Intent           string         `json:"intent,omitempty"`
	Request          string         `json:"request,omitempty"`
	Outcome          string         `json:"outcome,omitempty"`
	Findings         []AuditFinding `json:"findings,omitempty"`
	Log              string         `json:"log,omitempty"`
	Backend          string         `json:"backend,omitempty"`
	Platform         string         `json:"platform,omitempty"`
	Runner           string         `json:"runner,omitempty"`
	ExitCode         int            `json:"exitCode,omitempty"`
	DurationMS       int64          `json:"durationMs,omitempty"`
}

func isAuditKind(kind string) bool {
	switch kind {
	case "audit.rule-authorize", "audit.rule-revoke", "audit.request", "audit.result":
		return true
	}
	return false
}

func auditPipeline(raw []byte) (PipelineDefinition, error) {
	var p PipelineDefinition
	if err := strictResourceJSON(raw, &p, maxPipelineSize); err != nil {
		return p, err
	}
	return p, validatePipeline(p)
}

func validateAuditEvent(e Event) error {
	if e.Audit == nil {
		return fmt.Errorf("audit fact is required")
	}
	expected := Event{Protocol: e.Protocol, Kind: e.Kind, Actor: e.Actor, ActorName: e.ActorName, PublicKey: e.PublicKey, Sequence: e.Sequence, Timestamp: e.Timestamp, Previous: e.Previous, Audit: e.Audit}
	if e.Kind == "audit.request" {
		expected.Operation = e.Operation
		if len(e.Operation) > 256 {
			return fmt.Errorf("audit operation too long")
		}
	}
	if !reflect.DeepEqual(e, expected) {
		return fmt.Errorf("audit event contains unrelated envelope fields")
	}
	if e.Previous != "" && !validEventID(e.Previous) {
		return fmt.Errorf("invalid audit actor predecessor")
	}
	f := e.Audit
	if !issueIdentifier(f.Scope, 128) || !validPipelineName(f.Rule) || !validGitOID(f.PolicyCommit) || !validEventID(f.PolicyDigest) {
		return fmt.Errorf("audit requires scope, rule and exact authority policy")
	}
	allowed := AuditFact{Scope: f.Scope, Rule: f.Rule, PolicyCommit: f.PolicyCommit, PolicyDigest: f.PolicyDigest}
	switch e.Kind {
	case "audit.rule-authorize":
		allowed.Parents = f.Parents
		allowed.Source = f.Source
		allowed.Definition = f.Definition
		allowed.DefinitionDigest = f.DefinitionDigest
		if f.Source == nil || eventID([]byte(f.Definition)) != f.DefinitionDigest {
			return fmt.Errorf("audit authorization requires exact source and definition digest")
		}
		if _, err := auditPipeline([]byte(f.Definition)); err != nil {
			return fmt.Errorf("audit definition: %w", err)
		}
		s := f.Source
		switch s.Kind {
		case "pipeline", "git":
			if !validGitOID(s.Commit) || !safeRelativePath(s.Path) || s.Revision != "" {
				return fmt.Errorf("audit source requires exact already-present commit/path")
			}
			if s.Kind == "pipeline" && (!strings.HasPrefix(s.Path, ".hn/pipelines/") || !strings.HasSuffix(s.Path, ".json")) {
				return fmt.Errorf("pipeline source must name a committed .hn/pipelines/*.json")
			}
		case "workflow":
			if !validEventID(s.Revision) || s.Commit != "" || s.Path != "" {
				return fmt.Errorf("workflow source requires exact native revision")
			}
		default:
			return fmt.Errorf("unsupported audit source kind")
		}
	case "audit.rule-revoke":
		allowed.Parents = f.Parents
		allowed.Reason = f.Reason
		if len(f.Parents) == 0 || !issueText(f.Reason, 4096, true) {
			return fmt.Errorf("revocation requires predecessor heads and reason")
		}
	case "audit.request":
		allowed.Authorization = f.Authorization
		allowed.CodeCommit = f.CodeCommit
		allowed.DefinitionDigest = f.DefinitionDigest
		allowed.Intent = f.Intent
		if !validEventID(f.Authorization) || !validGitOID(f.CodeCommit) || !validEventID(f.DefinitionDigest) || !issueText(f.Intent, 4096, true) {
			return fmt.Errorf("audit request requires authorization, exact code/definition and intent")
		}
	case "audit.result":
		allowed.Authorization = f.Authorization
		allowed.CodeCommit = f.CodeCommit
		allowed.DefinitionDigest = f.DefinitionDigest
		allowed.Request = f.Request
		allowed.Outcome = f.Outcome
		allowed.Findings = f.Findings
		allowed.Log = f.Log
		allowed.Backend = f.Backend
		allowed.Platform = f.Platform
		allowed.Runner = f.Runner
		allowed.ExitCode = f.ExitCode
		allowed.DurationMS = f.DurationMS
		if !validEventID(f.Request) || !validEventID(f.Authorization) || !validGitOID(f.CodeCommit) || !validEventID(f.DefinitionDigest) || !validEventID(f.Log) || (f.Outcome != "passed" && f.Outcome != "failed") || (f.Backend != "sandbox" && f.Backend != "host") || !issueText(f.Platform, 128, true) || !issueText(f.Runner, 256, true) || f.DurationMS < 0 || f.ExitCode < 0 || f.ExitCode > 255 {
			return fmt.Errorf("invalid audit result binding or provenance")
		}
		if (f.Outcome == "passed" && f.ExitCode != 0) || (f.Outcome == "failed" && f.ExitCode == 0) {
			return fmt.Errorf("audit outcome and exit code disagree")
		}
		if len(f.Findings) > 128 {
			return fmt.Errorf("too many audit findings")
		}
		seen := map[string]bool{}
		for _, finding := range f.Findings {
			if !issueIdentifier(finding.Key, 128) || seen[finding.Key] || !issueText(finding.Title, 512, true) || !issueText(finding.Body, 16<<10, false) || (finding.Path != "" && (!safeRelativePath(finding.Path) || len(finding.Path) > 1024)) || finding.Line < 0 || (finding.Line > 0 && finding.Path == "") {
				return fmt.Errorf("invalid or duplicate audit finding")
			}
			seen[finding.Key] = true
		}
	default:
		return fmt.Errorf("unsupported audit kind")
	}
	if len(f.Parents) > 64 || !validEvidenceIDs(f.Parents) {
		return fmt.Errorf("invalid audit predecessor heads")
	}
	if !reflect.DeepEqual(*f, allowed) {
		return fmt.Errorf("audit payload contains fields unrelated to its kind")
	}
	raw, _ := json.Marshal(e)
	if len(raw) > maxAuditPayload {
		return fmt.Errorf("audit payload too large")
	}
	return nil
}

func auditReferences(e Event) []string {
	if !isAuditKind(e.Kind) || e.Audit == nil {
		return nil
	}
	f := e.Audit
	refs := append([]string(nil), f.Parents...)
	if f.Authorization != "" {
		refs = append(refs, f.Authorization)
	}
	if f.Request != "" {
		refs = append(refs, f.Request)
	}
	if f.Source != nil && f.Source.Kind == "workflow" {
		refs = append(refs, f.Source.Revision)
	}
	return sortedUniqueStrings(refs...)
}

func auditObjectRoots(e Event) []string {
	if !isAuditKind(e.Kind) || e.Audit == nil {
		return nil
	}
	f := e.Audit
	roots := []string{f.PolicyCommit}
	if f.CodeCommit != "" {
		roots = append(roots, f.CodeCommit)
	}
	if f.Source != nil && f.Source.Commit != "" {
		roots = append(roots, f.Source.Commit)
	}
	return sortedUniqueStrings(roots...)
}

// Packs deliberately forbid deltas. Bound both the compressed input and each
// expanded object before handing hostile bytes to Git's checksum/object parser.
func validateAuditPack(pack []byte) error {
	_, err := auditPackObjectIDs(pack, 40)
	return err
}

func auditPackObjectIDs(pack []byte, oidLength int) ([]string, error) {
	return auditPackObjectIDsAndSizes(pack, oidLength, nil)
}

func auditPackObjectIDsAndSizes(pack []byte, oidLength int, sizes map[string]int64) ([]string, error) {
	if len(pack) < 32 || len(pack) > maxAuditPack || string(pack[:4]) != "PACK" || binary.BigEndian.Uint32(pack[4:8]) != 2 {
		return nil, fmt.Errorf("invalid bounded audit dependency pack")
	}
	count := binary.BigEndian.Uint32(pack[8:12])
	if count == 0 || count > maxAuditObjects {
		return nil, fmt.Errorf("audit dependency object count exceeded")
	}
	ids := []string{}
	r := bytes.NewReader(pack[12:])
	total := int64(0)
	for i := uint32(0); i < count; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		kind := (b >> 4) & 7
		size := uint64(b & 15)
		shift := uint(4)
		for b&128 != 0 {
			if shift > 60 {
				return nil, fmt.Errorf("audit object size overflow")
			}
			b, err = r.ReadByte()
			if err != nil {
				return nil, err
			}
			size |= uint64(b&127) << shift
			shift += 7
		}
		if kind < 1 || kind > 4 || size > maxAuditObject {
			return nil, fmt.Errorf("audit pack contains delta or oversized object")
		}
		total += int64(size)
		if total > maxAuditExpanded {
			return nil, fmt.Errorf("expanded audit pack too large")
		}
		z, err := zlib.NewReader(r)
		if err != nil {
			return nil, err
		}
		var h hash.Hash = sha1.New()
		if oidLength == 64 {
			h = sha256.New()
		}
		fmt.Fprintf(h, "%s %d%c", []string{"", "commit", "tree", "blob", "tag"}[kind], size, byte(0))
		n, err := io.Copy(h, io.LimitReader(z, int64(size)+1))
		closeErr := z.Close()
		if err != nil || closeErr != nil || n != int64(size) {
			return nil, fmt.Errorf("audit dependency object size mismatch")
		}
		oid := hex.EncodeToString(h.Sum(nil))
		ids = append(ids, oid)
		if sizes != nil {
			sizes[oid] = int64(size)
		}
	}
	if r.Len() != 20 && r.Len() != 32 {
		return nil, fmt.Errorf("audit pack has unexpected trailing data")
	}
	trailer := r.Len()
	var checksum hash.Hash = sha1.New()
	if trailer == 32 {
		checksum = sha256.New()
	}
	_, _ = checksum.Write(pack[:len(pack)-trailer])
	if !bytes.Equal(checksum.Sum(nil), pack[len(pack)-trailer:]) {
		return nil, fmt.Errorf("audit pack checksum mismatch")
	}
	if len(sortedUniqueStrings(ids...)) != len(ids) {
		return nil, fmt.Errorf("duplicate audit pack object")
	}
	return sortedUniqueStrings(ids...), nil
}

func auditAttachments(e Event, log []byte) (map[string][]byte, error) {
	objects, err := auditDependencyObjectIDsAt("", e)
	if err != nil {
		return nil, err
	}
	pack, err := gitInput([]byte(strings.Join(objects, "\n")+"\n"), nil, "pack-objects", "--stdout", "--window=0", "--depth=0")
	if err != nil {
		return nil, err
	}
	if err := validateAuditPack(pack); err != nil {
		return nil, err
	}
	result := map[string][]byte{"audit.pack": pack}
	if e.Kind == "audit.result" {
		result["log.txt"] = log
	}
	return result, nil
}

func validateAuditAttachments(e Event, attachments map[string][]byte) error {
	if err := validateAuditPack(attachments["audit.pack"]); err != nil {
		return err
	}
	want := 1
	if e.Kind == "audit.result" {
		want++
		log, ok := attachments["log.txt"]
		if !ok || len(log) > maxRunLogSize || eventID(log) != e.Audit.Log {
			return fmt.Errorf("audit result requires bounded matching log.txt")
		}
	}
	if len(attachments) != want {
		return fmt.Errorf("unsupported audit attachments")
	}
	return nil
}

// importAuditPack runs only in an already selected object database. Fetched
// facts use the private quarantine database; no accepted ref is created here.
func importAuditPack(gitdir string, pack []byte, e Event) error {
	if err := validateAuditPack(pack); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "hn-audit-objects-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	format := "sha1"
	if len(e.Audit.PolicyCommit) == 64 {
		format = "sha256"
	}
	if _, err := gitOutput("init", "--bare", "--quiet", "--object-format="+format, temporary); err != nil {
		return err
	}
	// Validate Git's object syntax without demanding complete parent history or
	// unrelated members of policy/source path-proof trees. Required snapshot
	// members and actual edge types are checked below before installation.
	if _, err := gitInputAt(temporary, pack, "index-pack", "--stdin", "--fsck-objects", "--max-input-size=67108864"); err != nil {
		return err
	}
	if err := validateAuditPackBinding(temporary, e, pack); err != nil {
		return err
	}
	_, err = gitInputAt(gitdir, pack, "index-pack", "--stdin", "--max-input-size=67108864")
	return err
}

func auditCommitAvailable(gitdir, accepted, commit string) error {
	typ, err := gitTextAt(gitdir, "cat-file", "-t", commit)
	if err != nil && accepted != "" {
		typ, err = gitTextAt(accepted, "cat-file", "-t", commit)
	}
	if err != nil || typ != "commit" {
		return fmt.Errorf("audit exact commit %s unavailable", commit)
	}
	return nil
}

// validateAuditRelationshipAt is the one historical permission/relationship
// contract used by local storage, keyless reads and quarantined admission.
func validateAuditRelationshipAt(gitdir, accepted string, s StoredEvent, byID map[string]StoredEvent) error {
	e := s.Event
	f := e.Audit
	if err := validateAuditEvent(e); err != nil {
		return err
	}
	for _, root := range auditObjectRoots(e) {
		if err := auditCommitAvailable(gitdir, accepted, root); err != nil {
			return err
		}
	}
	p, digest, err := loadReplicationPolicyAt(gitdir, accepted, f.PolicyCommit)
	if err != nil {
		return err
	}
	if p.Audits == nil || p.Audits.Scope != f.Scope || digest != f.PolicyDigest {
		return fmt.Errorf("audit recorded policy does not match scope/digest")
	}
	roles := p.Audits.RuleOwners
	switch e.Kind {
	case "audit.request":
		roles = p.Audits.Requesters
	case "audit.result":
		roles = p.Audits.Executors
	}
	if !actorListed(e.Actor, roles) {
		return fmt.Errorf("audit signer lacks recorded policy role")
	}
	for _, ref := range auditReferences(e) {
		if _, ok := byID[ref]; !ok {
			return fmt.Errorf("audit dependency %s unavailable", ref)
		}
	}
	for _, parentID := range f.Parents {
		parent := byID[parentID]
		if (parent.Event.Kind != "audit.rule-authorize" && parent.Event.Kind != "audit.rule-revoke") || parent.Event.Audit.Scope != f.Scope || parent.Event.Audit.Rule != f.Rule {
			return fmt.Errorf("audit rule predecessor mismatch")
		}
	}
	if e.Kind == "audit.rule-authorize" {
		source := f.Source
		var raw []byte
		if source.Kind == "workflow" {
			revision := byID[source.Revision]
			if resourceFamily(revision.Event.Kind) != ResourceWorkflow {
				return fmt.Errorf("audit source is not a native workflow")
			}
			var state WorkflowState
			if err := strictResourceJSON([]byte(revision.Event.Body), &state, maxAuditPayload); err != nil {
				return err
			}
			raw, _, err = runnableWorkflowDefinition(state)
		} else {
			raw, err = gitOutputAt(gitdir, "show", source.Commit+":"+source.Path)
			if err != nil && accepted != "" {
				raw, err = gitOutputAt(accepted, "show", source.Commit+":"+source.Path)
			}
		}
		if err != nil || !bytes.Equal(raw, []byte(f.Definition)) {
			return fmt.Errorf("audit source does not match exact authorized definition")
		}
	}
	if e.Kind == "audit.request" || e.Kind == "audit.result" {
		auth := byID[f.Authorization]
		if auth.Event.Kind != "audit.rule-authorize" || auth.Event.Audit.Scope != f.Scope || auth.Event.Audit.Rule != f.Rule || auth.Event.Audit.DefinitionDigest != f.DefinitionDigest || auth.Event.Audit.PolicyDigest != f.PolicyDigest || !actorListed(auth.Event.Actor, p.Audits.RuleOwners) {
			return fmt.Errorf("audit rule authorization mismatch")
		}
		if e.Kind == "audit.result" {
			request := byID[f.Request]
			if request.Event.Kind != "audit.request" {
				return fmt.Errorf("audit result requires an audit request")
			}
			q := request.Event.Audit
			if q.Scope != f.Scope || q.Rule != f.Rule || q.Authorization != f.Authorization || q.CodeCommit != f.CodeCommit || q.DefinitionDigest != f.DefinitionDigest || q.PolicyDigest != f.PolicyDigest || !actorListed(request.Event.Actor, p.Audits.Requesters) {
				return fmt.Errorf("audit result does not match exact request")
			}
		}
	}
	return nil
}

type AuditAuthorizeOptions struct {
	Rule      string
	Source    AuditSource
	Expected  []string
	PolicyRef string
}

func auditExpectedHeads(a *AuditAuthority, rule string, expected []string, events []StoredEvent) error {
	actual := []string{}
	for _, s := range currentAuditRuleHeads(a, rule, events) {
		actual = append(actual, s.ID)
	}
	if !validEvidenceIDs(expected) || len(expected) > 64 || !reflect.DeepEqual(sortedUniqueStrings(actual...), sortedUniqueStrings(expected...)) {
		return fmt.Errorf("audit rule heads changed; provide all exact current --expect heads")
	}
	return nil
}

func auditFactFor(a *AuditAuthority, rule string) *AuditFact {
	return &AuditFact{Scope: a.Policy.Audits.Scope, Rule: rule, PolicyCommit: a.Commit, PolicyDigest: a.PolicyDigest}
}

func appendAudit(e Event, id *Identity, log []byte) (*StoredEvent, error) {
	attachments, err := auditAttachments(e, log)
	if err != nil {
		return nil, err
	}
	return appendEventWithAttachments(e, id, attachments)
}

func AuditAuthorize(options AuditAuthorizeOptions, id *Identity) (*StoredEvent, error) {
	a, err := currentAuditAuthority(options.PolicyRef, id.Actor, "owner")
	if err != nil {
		return nil, err
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	if err := auditExpectedHeads(a, options.Rule, options.Expected, events); err != nil {
		return nil, err
	}
	source := options.Source
	var raw []byte
	if source.Kind == "workflow" {
		selected, err := selectAuditWorkflow(source.Revision, events)
		if err != nil {
			return nil, err
		}
		raw, _, err = runnableWorkflowDefinition(selected.State)
		if err != nil {
			return nil, err
		}
		source.Revision = selected.Revision
	} else {
		if !validGitOID(source.Commit) || !safeRelativePath(source.Path) {
			return nil, fmt.Errorf("audit source requires resolved commit and safe path")
		}
		raw, err = auditSourceBlobAt("", source.Commit, source.Path)
		if err != nil {
			return nil, err
		}
	}
	e, err := nextEvent(id, "audit.rule-authorize")
	if err != nil {
		return nil, err
	}
	e.Audit = auditFactFor(a, options.Rule)
	e.Audit.Parents = options.Expected
	e.Audit.Source = &source
	e.Audit.Definition = string(raw)
	e.Audit.DefinitionDigest = eventID(raw)
	return appendAudit(e, id, nil)
}

func AuditRevoke(rule string, expected []string, reason, policyRef string, id *Identity) (*StoredEvent, error) {
	a, err := currentAuditAuthority(policyRef, id.Actor, "owner")
	if err != nil {
		return nil, err
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	if err := auditExpectedHeads(a, rule, expected, events); err != nil {
		return nil, err
	}
	e, err := nextEvent(id, "audit.rule-revoke")
	if err != nil {
		return nil, err
	}
	e.Audit = auditFactFor(a, rule)
	e.Audit.Parents = expected
	e.Audit.Reason = reason
	return appendAudit(e, id, nil)
}

func AuditRequest(commit, rule, policyRef string, id *Identity) (*StoredEvent, error) {
	return AuditRequestWithIntent(commit, rule, policyRef, "explicit audit", "", id)
}

func AuditRequestWithIntent(commit, rule, policyRef, intent, operation string, id *Identity) (*StoredEvent, error) {
	a, err := currentAuditAuthority(policyRef, id.Actor, "requester")
	if err != nil {
		return nil, err
	}
	if !validGitOID(commit) {
		return nil, fmt.Errorf("audit target must be a resolved Git commit")
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	auth, err := usableAuditRule(a, rule, events)
	if err != nil {
		return nil, err
	}
	e, err := nextEvent(id, "audit.request")
	if err != nil {
		return nil, err
	}
	e.Operation = operation
	e.Audit = auditFactFor(a, rule)
	e.Audit.Authorization = auth.ID
	e.Audit.CodeCommit = commit
	e.Audit.DefinitionDigest = auth.Event.Audit.DefinitionDigest
	e.Audit.Intent = intent
	return appendAudit(e, id, nil)
}

type AuditExecution struct {
	Request       StoredEvent
	Authorization StoredEvent
	Authority     AuditAuthority
	Pipeline      PipelineDefinition
}

// ValidateAuditExecution performs current authority checks immediately before
// execution. It performs no execution, fetch, branch mutation or private adoption.
func ValidateAuditExecution(requestID, actor, policyRef string) (*AuditExecution, error) {
	if !validEventID(requestID) {
		return nil, fmt.Errorf("audit requires full request ID")
	}
	a, err := currentAuditAuthority(policyRef, actor, "executor")
	if err != nil {
		return nil, err
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	request, err := resolveEvent(events, requestID)
	if err != nil {
		return nil, err
	}
	if request.Event.Kind != "audit.request" {
		return nil, fmt.Errorf("not an audit request")
	}
	f := request.Event.Audit
	if f.Scope != a.Policy.Audits.Scope || f.PolicyDigest != a.PolicyDigest || !actorListed(request.Event.Actor, a.Policy.Audits.Requesters) {
		return nil, fmt.Errorf("audit request has stale current policy or requester")
	}
	if _, err := gitOutput("merge-base", "--is-ancestor", f.PolicyCommit, a.Commit); err != nil {
		return nil, fmt.Errorf("request authority is outside current anchored history")
	}
	auth, err := usableAuditRule(a, f.Rule, events)
	if err != nil {
		return nil, err
	}
	if auth.ID != f.Authorization {
		return nil, fmt.Errorf("audit request names a stale rule authorization")
	}
	pipeline, err := auditPipeline([]byte(auth.Event.Audit.Definition))
	if err != nil {
		return nil, err
	}
	return &AuditExecution{Request: *request, Authorization: *auth, Authority: *a, Pipeline: pipeline}, nil
}

func validateCurrentAuditWrite(e Event, events []StoredEvent) error {
	role := "owner"
	if e.Kind == "audit.request" {
		role = "requester"
	}
	if e.Kind == "audit.result" {
		role = "executor"
	}
	a, err := currentAuditAuthority("", e.Actor, role)
	if err != nil {
		return err
	}
	f := e.Audit
	if f.Scope != a.Policy.Audits.Scope || f.PolicyCommit != a.Commit || f.PolicyDigest != a.PolicyDigest {
		return fmt.Errorf("audit write does not match current anchored authority")
	}
	if e.Kind == "audit.rule-authorize" && f.Source != nil && f.Source.Kind == "workflow" {
		if _, err := selectAuditWorkflow(f.Source.Revision, events); err != nil {
			return err
		}
	}
	if e.Kind == "audit.rule-authorize" || e.Kind == "audit.rule-revoke" {
		return auditExpectedHeads(a, f.Rule, f.Parents, events)
	}
	auth, err := usableAuditRule(a, f.Rule, events)
	if err != nil {
		return err
	}
	if auth.ID != f.Authorization {
		return fmt.Errorf("audit write has stale rule authorization")
	}
	if e.Kind == "audit.result" {
		_, err = ValidateAuditExecution(f.Request, e.Actor, "")
		return err
	}
	return nil
}

func selectAuditWorkflow(revision string, events []StoredEvent) (*workflowSelection, error) {
	if !validEventID(revision) {
		return nil, fmt.Errorf("audit workflow requires exact revision")
	}
	catalog, err := BuildResourceCatalog(events)
	if err != nil {
		return nil, err
	}
	for id, view := range catalog.Resources {
		if view.Family == ResourceWorkflow && !view.Conflict && len(view.Heads) == 1 && view.Heads[0].ID == revision {
			return selectNativeWorkflow(id, revision)
		}
	}
	return nil, fmt.Errorf("audit workflow revision is unavailable, stale or conflicted")
}

// auditDependencyObjectIDsAt retains the target snapshot, policy path proofs,
// and source path proofs. Commit parents are deliberately not traversed. Their
// absence cannot authorize work; current ancestry is checked in the local anchor.
func auditDependencyObjectIDsAt(gitdir string, e Event) ([]string, error) {
	f := e.Audit
	objects := []string{}
	addPath := func(commit, path string) error {
		tree, err := gitTextAt(gitdir, "rev-parse", "--verify", commit+"^{tree}")
		if err != nil {
			return err
		}
		objects = append(objects, commit, tree)
		parts := strings.Split(path, "/")
		for index := range parts {
			selected := strings.Join(parts[:index+1], "/")
			entry, err := gitTextAt(gitdir, "ls-tree", commit, "--", selected)
			if err != nil {
				return err
			}
			fields := strings.Fields(entry)
			if len(fields) < 3 {
				return fmt.Errorf("audit dependency path missing: %s", selected)
			}
			if index == len(parts)-1 && (fields[0] != "100644" && fields[0] != "100755") {
				return fmt.Errorf("audit dependency must be a regular blob")
			}
			if index < len(parts)-1 && fields[0] != "040000" {
				return fmt.Errorf("audit dependency parent must be a tree")
			}
			objects = append(objects, fields[2])
		}
		return nil
	}
	if err := addPath(f.PolicyCommit, ".hn/policy.json"); err != nil {
		return nil, err
	}
	if f.Source != nil && f.Source.Commit != "" {
		if err := addPath(f.Source.Commit, f.Source.Path); err != nil {
			return nil, err
		}
	}
	if f.CodeCommit != "" {
		listed, err := gitTextAt(gitdir, "rev-list", "--objects", "--no-walk", f.CodeCommit)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(listed, "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				objects = append(objects, fields[0])
			}
		}
	}
	objects = sortedUniqueStrings(objects...)
	if len(objects) > maxAuditObjects {
		return nil, fmt.Errorf("audit dependency object count exceeded")
	}
	return objects, nil
}

func validateAuditPackBinding(gitdir string, e Event, pack []byte) error {
	actual, err := auditPackObjectIDs(pack, len(e.Audit.PolicyCommit))
	if err != nil {
		return err
	}
	expected, err := auditDependencyObjectIDsAt(gitdir, e)
	if err != nil {
		return err
	}
	if err := validateAuditObjectShapesAt(gitdir, actual); err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("audit dependency pack differs from exact signed snapshot/path closure")
	}
	return nil
}

// RecordAuditResult is the only audit result writer. The caller supplies an
// explicitly completed execution; this function never runs repository code.
// It rechecks current authority after execution and before signing evidence.
func RecordAuditResult(requestID, policyRef string, id *Identity, result executionResult, findings []AuditFinding, backend, platform, runner string) (*StoredEvent, error) {
	selected, err := ValidateAuditExecution(requestID, id.Actor, policyRef)
	if err != nil {
		return nil, err
	}
	e, err := nextEvent(id, "audit.result")
	if err != nil {
		return nil, err
	}
	q := selected.Request.Event.Audit
	e.Audit = auditFactFor(&selected.Authority, q.Rule)
	f := e.Audit
	f.Authorization = q.Authorization
	f.CodeCommit = q.CodeCommit
	f.DefinitionDigest = q.DefinitionDigest
	f.Request = requestID
	f.Outcome = result.Outcome
	f.ExitCode = result.ExitCode
	f.DurationMS = result.DurationMS
	f.Log = eventID(result.Log)
	f.Findings = findings
	f.Backend = backend
	f.Platform = platform
	f.Runner = runner
	return appendAudit(e, id, result.Log)
}

func validateAuditObjectShapesAt(gitdir string, objects []string) error {
	types := make(map[string]string, len(objects))
	for _, object := range objects {
		typ, err := gitTextAt(gitdir, "cat-file", "-t", object)
		if err != nil {
			return err
		}
		types[object] = typ
	}
	for _, object := range objects {
		typ := types[object]
		switch typ {
		case "blob":
		case "commit":
			raw, err := gitTextAt(gitdir, "cat-file", "commit", object)
			if err != nil {
				return err
			}
			lines := strings.Split(raw, "\n")
			if len(lines) < 4 || !strings.HasPrefix(lines[0], "tree ") || types[strings.TrimPrefix(lines[0], "tree ")] != "tree" {
				return fmt.Errorf("malformed audit commit")
			}
			for _, line := range lines[1:] {
				if parent, ok := strings.CutPrefix(line, "parent "); ok {
					// Parent history may be absent. When present in this exact pack,
					// it must have the type required by a commit-parent edge.
					if !validGitOID(parent) || len(parent) != len(object) || (types[parent] != "" && types[parent] != "commit") {
						return fmt.Errorf("malformed audit parent")
					}
				}
				if line == "" {
					break
				}
			}
		case "tree":
			raw, err := gitOutputAt(gitdir, "cat-file", "tree", object)
			if err != nil {
				return err
			}
			names := map[string]bool{}
			for len(raw) > 0 {
				space := bytes.IndexByte(raw, ' ')
				nul := bytes.IndexByte(raw, 0)
				oidBytes := len(object) / 2
				if space <= 0 || nul <= space || len(raw)-nul-1 < oidBytes {
					return fmt.Errorf("malformed audit tree")
				}
				mode, name := string(raw[:space]), string(raw[space+1:nul])
				oid := hex.EncodeToString(raw[nul+1 : nul+1+oidBytes])
				raw = raw[nul+1+oidBytes:]
				if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.EqualFold(name, ".git") || names[name] || oid == strings.Repeat("0", len(object)) {
					return fmt.Errorf("malformed audit tree")
				}
				want := "blob"
				switch mode {
				case "40000":
					want = "tree"
				case "160000":
					want = "commit"
				case "100644", "100755", "120000":
				default:
					return fmt.Errorf("unsupported audit tree mode")
				}
				// Compare the raw entry mode against cat-file's independent type
				// from the isolated pack; ls-tree normalizes modes and types.
				// Missing non-proof edges and gitlinks are intentional; exact
				// dependency closure separately requires the full target snapshot.
				if actual := types[oid]; actual != "" && actual != want {
					return fmt.Errorf("audit tree edge %q requires %s, found %s", name, want, actual)
				}
				names[name] = true
			}
		default:
			return fmt.Errorf("unexpected audit dependency object type")
		}
	}
	return nil
}

// measureAuditSnapshotDependenciesAt counts expanded snapshot objects against
// the existing per-selection budgets before any pack is unpacked. Repeated
// objects within a history count once, and only the prior accepted history is
// discounted; ambient object availability grants no budget exemption.
func measureAuditSnapshotDependenciesAt(gitdir, acceptedGitdir, root, acceptedRoot string, budgets ReplicationBudgets, selection string, m ReplicationMeasurements) (ReplicationMeasurements, error) {
	previous := map[string]int64{}
	if acceptedRoot != "" {
		if err := auditHistoryPackSizesAt(acceptedGitdir, acceptedRoot, previous, nil); err != nil {
			return m, err
		}
	}
	sizes := map[string]int64{}
	err := auditHistoryPackSizesAt(gitdir, root, sizes, func(id string, size int64) error {
		if _, exists := previous[id]; exists {
			return nil
		}
		m.Objects++
		m.TotalBytes += size
		if size > m.LargestObjectBytes {
			m.LargestObjectBytes = size
		}
		if err := enforceReplicationBudgets(selection, budgets, m); err != nil {
			return replicationBudgetExceeded{err}
		}
		return nil
	})
	return m, err
}

func auditHistoryPackSizesAt(gitdir, root string, sizes map[string]int64, add func(string, int64) error) error {
	commits, err := gitTextAt(gitdir, "rev-list", "--reverse", root)
	if err != nil {
		return err
	}
	for _, commit := range strings.Fields(commits) {
		size, err := gitTextAt(gitdir, "cat-file", "-s", commit+":audit.pack")
		if err != nil {
			continue
		}
		n, err := strconv.ParseInt(size, 10, 64)
		if err != nil || n < 0 || n > maxAuditPack {
			return fmt.Errorf("audit dependency pack exceeds bounds")
		}
		pack, err := gitOutputAt(gitdir, "show", commit+":audit.pack")
		if err != nil {
			return err
		}
		current := map[string]int64{}
		if _, err := auditPackObjectIDsAndSizes(pack, len(commit), current); err != nil {
			return err
		}
		for id, size := range current {
			if _, seen := sizes[id]; seen {
				continue
			}
			sizes[id] = size
			if add != nil {
				if err := add(id, size); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func auditSourceBlobAt(gitdir, commit, path string) ([]byte, error) {
	size, err := gitTextAt(gitdir, "cat-file", "-s", commit+":"+path)
	if err != nil {
		return nil, err
	}
	n, err := strconv.ParseInt(size, 10, 64)
	if err != nil || n < 0 || n > maxPipelineSize {
		return nil, fmt.Errorf("audit source exceeds pipeline bound")
	}
	return gitOutputAt(gitdir, "show", commit+":"+path)
}

// strictAuditJSON also rejects declared fields belonging to a different audit
// kind, even when null/empty decoding would erase their presence.
func strictAuditJSON(raw []byte, e *Event) error {
	if err := strictResourceJSON(raw, e, maxAuditPayload); err != nil {
		return err
	}
	check := func(raw []byte, fields string) error {
		var values map[string]json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return err
		}
		allowed := strings.Fields(fields)
		for key := range values {
			if !stringListed(key, allowed) {
				return fmt.Errorf("field %q is not declared for this audit shape", key)
			}
		}
		return nil
	}
	envelope := "protocol kind actor actorName publicKey sequence timestamp previous audit"
	if e.Kind == "audit.request" {
		envelope += " operation"
	}
	if err := check(raw, envelope); err != nil {
		return err
	}
	var outer map[string]json.RawMessage
	_ = json.Unmarshal(raw, &outer)
	fields := "scope rule policyCommit policyDigest"
	switch e.Kind {
	case "audit.rule-authorize":
		fields += " parents source definition definitionDigest"
	case "audit.rule-revoke":
		fields += " parents reason"
	case "audit.request":
		fields += " authorization codeCommit definitionDigest intent"
	case "audit.result":
		fields += " authorization codeCommit definitionDigest request outcome findings log backend platform runner exitCode durationMs"
	}
	if err := check(outer["audit"], fields); err != nil {
		return err
	}
	if e.Kind == "audit.rule-authorize" && e.Audit != nil && e.Audit.Source != nil {
		var fact map[string]json.RawMessage
		_ = json.Unmarshal(outer["audit"], &fact)
		sourceFields := "kind commit path"
		if e.Audit.Source.Kind == "workflow" {
			sourceFields = "kind revision"
		}
		if err := check(fact["source"], sourceFields); err != nil {
			return err
		}
	}
	return nil
}
