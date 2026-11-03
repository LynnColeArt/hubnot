package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hubnot/internal/artifact"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type artifactMutationResult struct {
	Protocol   string  `json:"protocol"`
	Family     *string `json:"family"`
	TypeID     string  `json:"type_id"`
	ResourceID string  `json:"resource_id"`
	EventID    string  `json:"event_id"`
	Replayed   bool    `json:"replayed"`
	Conflict   bool    `json:"conflict"`
	HeadCount  int     `json:"head_count"`
	Admission  *string `json:"admission"`
	SourceCut  string  `json:"source_cut"`
}
type artifactCandidate struct {
	ID     string `json:"id"`
	Commit string `json:"commit"`
}

// artifactIntent is the frozen private descriptor. Public artifacts never contain
// this descriptor; operation recovery can inspect it without a second writer.
type artifactIntent struct {
	Protocol        string                  `json:"protocol"`
	RepositoryID    string                  `json:"repository_id"`
	Actor           string                  `json:"actor"`
	Operation       string                  `json:"operation"`
	Request         string                  `json:"request"`
	SemanticRequest json.RawMessage         `json:"semantic_request"`
	Stage           string                  `json:"stage"`
	ExpectedRef     string                  `json:"expected_ref"`
	ExpectedOID     *string                 `json:"expected_oid"`
	Candidate       *artifactCandidate      `json:"candidate"`
	Result          *artifactMutationResult `json:"result"`
	rawHash         string
}

const artifactIntentBytes = 2097152

func artifactSemantic(kind, actor, operation string, input map[string]any) (map[string]any, []byte, string, error) {
	record := ""
	if kind != "type.publish" && input["record"] != nil {
		return nil, nil, "", artifactFailure("invalid_input", "opening requires record=null")
	}
	semantic := map[string]any{"protocol": "hn-artifact/1", "kind": kind, "actor": actor, "operation": operation, "record": record, "parents": input["parents"], "payload": input["payload"]}
	raw, e := json.Marshal(semantic)
	if e != nil {
		return nil, nil, "", artifactFailure("invalid_input", "invalid mutation input")
	}
	return semantic, raw, eventID(append([]byte("hn-artifact-request-v1\x00"), raw...)), nil
}
func artifactParseMutation(raw []byte, kind string) (map[string]any, error) {
	canonical, e := artifact.Canonical(raw)
	if e != nil {
		return nil, e
	}
	var input map[string]any
	d := json.NewDecoder(bytes.NewReader(canonical))
	d.UseNumber()
	if e = d.Decode(&input); e != nil || input == nil {
		return nil, artifactFailure("invalid_input", "mutation input must be an object")
	}
	expected := 2
	if kind != "type.publish" {
		expected = 3
	}
	if len(input) != expected || input["payload"] == nil || input["parents"] == nil {
		return nil, artifactFailure("invalid_input", "mutation input has unknown or missing fields")
	}
	if _, ok := input["payload"].(map[string]any); !ok {
		return nil, artifactFailure("invalid_input", "mutation payload must be an object")
	}
	parents, ok := input["parents"].([]any)
	if !ok {
		return nil, artifactFailure("invalid_input", "mutation parents must be an array")
	}
	if kind == "type.publish" {
		if len(parents) > 1 {
			return nil, artifactFailure("invalid_input", "type publication has at most one parent")
		}
	} else {
		v, ok := input["record"]
		if !ok || v != nil || len(parents) != 0 {
			return nil, artifactFailure("invalid_input", "opening requires null record and no parents")
		}
	}
	for _, v := range parents {
		id, ok := v.(string)
		if !ok || !validMemoryDigestID(id) {
			return nil, artifactFailure("invalid_input", "parent must be a full artifact ID")
		}
	}
	return input, nil
}
func artifactValidateOperation(actor, operation string) error {
	if !validActorFingerprint(actor) || len(operation) < 1 || len(operation) > 128 {
		return artifactFailure("invalid_input", "requires full actor and bounded operation key")
	}
	for _, b := range []byte(operation) {
		if b < 0x21 || b > 0x7e {
			return artifactFailure("invalid_input", "operation must be printable non-whitespace ASCII")
		}
	}
	return nil
}
func artifactCheckMutation(ctx context.Context, h *artifactHistory, kind string, input map[string]any) error {
	payload := input["payload"].(map[string]any)
	raw, _ := json.Marshal(payload)
	if kind == "type.publish" {
		typ, e := artifact.ValidateType(ctx, raw)
		if e != nil {
			return e
		}
		for _, id := range artifactIDs(input["parents"]) {
			s := h.ByID[id]
			if s == nil {
				return artifactFailure("incomplete_dependency", "type parent missing")
			}
			if s.verified.Kind() != "type.publish" {
				return artifactFailure("invalid_input", "type parent must be a declaration")
			}
			b, _ := json.Marshal(artifactPayload(s))
			old, e := artifact.ValidateType(ctx, b)
			if e != nil {
				return e
			}
			if old.TypeID != typ.TypeID || old.Shape != typ.Shape || old.SchemaVersion >= typ.SchemaVersion {
				return artifactFailure("invalid_input", "type evolution must preserve identity and increase version")
			}
		}
		return nil
	}
	typ := h.ByID[artifactText(payload, "type_revision")]
	if typ == nil {
		return artifactFailure("incomplete_dependency", "exact schema declaration missing")
	}
	if typ.verified.Kind() != "type.publish" {
		return artifactFailure("invalid_input", "exact schema is not a type declaration")
	}
	b, _ := json.Marshal(artifactPayload(typ))
	decl, e := artifact.ValidateType(ctx, b)
	if e != nil {
		return e
	}
	if e = decl.ValidateOpening(ctx, raw); e != nil {
		return e
	}
	for _, field := range []string{"members", "relationships"} {
		links, _ := payload[field].([]any)
		for _, v := range links {
			link := v.(map[string]any)
			target := h.ByID[artifactText(link, "target")]
			if target == nil {
				if link["required"] == true {
					return artifactFailure("incomplete_dependency", "required artifact target missing")
				}
				continue
			}
			if !strings.HasPrefix(target.verified.Kind(), "record.") || (link["target_mode"] == "logical" && target.verified.Kind() != "record.open") {
				return artifactFailure("invalid_input", "artifact target has wrong lineage")
			}
		}
	}
	return nil
}
func artifactIntentPath(actor, operation string, create bool) (string, error) {
	_, common, e := artifactRepositoryID()
	if e != nil {
		return "", e
	}
	path := common
	missing := false
	for _, name := range []string{"hn", "artifacts", "intents", actor} {
		path = filepath.Join(path, name)
		if create {
			if e = ensurePrivateDirectory(path); e != nil {
				return "", artifactFailure("repository_error", "unsafe artifact intent directory")
			}
			continue
		}
		if missing {
			continue
		}
		info, e := os.Lstat(path)
		if errors.Is(e, os.ErrNotExist) {
			missing = true
			continue
		}
		if e != nil || validatePrivateDirectory(path, info) != nil {
			return "", artifactFailure("repository_error", "unsafe artifact intent directory")
		}
	}
	return filepath.Join(path, strings.TrimPrefix(eventID([]byte(operation)), "sha256:")+".json"), nil
}
func artifactIntentLock(actor, operation string) (string, func(), error) {
	path, e := artifactIntentPath(actor, operation, true)
	if e != nil {
		return "", nil, e
	}
	fd, e := syscall.Open(strings.TrimSuffix(path, ".json")+".lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return "", nil, artifactFailure("concurrent_update", "cannot acquire artifact operation lock")
	}
	f := os.NewFile(uintptr(fd), "artifact operation lock")
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		f.Close()
		return "", nil, artifactFailure("repository_error", "unsafe artifact operation lock")
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return "", nil, artifactFailure("concurrent_update", "artifact operation is active")
	}
	return path, func() { _ = syscall.Flock(fd, syscall.LOCK_UN); _ = f.Close() }, nil
}

// The status/evidence reader never creates a directory or lock file. A held
// existing lock is evidence of uncertainty, not permission to steal a writer.
func artifactProbeLock(path string) (present, settled bool, err error) {
	fd, e := syscall.Open(strings.TrimSuffix(path, ".json")+".lock", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(e, os.ErrNotExist) {
		return false, true, nil
	}
	if e != nil {
		return false, false, artifactFailure("repository_error", "cannot inspect artifact operation settlement")
	}
	f := os.NewFile(uintptr(fd), "artifact operation settlement")
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return true, false, artifactFailure("repository_error", "unsafe artifact operation lock")
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		if errors.Is(e, syscall.EWOULDBLOCK) || errors.Is(e, syscall.EAGAIN) {
			return true, false, nil
		}
		return true, false, artifactFailure("repository_error", "cannot verify artifact operation settlement")
	}
	_ = syscall.Flock(fd, syscall.LOCK_UN)
	return true, true, nil
}

// artifactRawObject distinguishes absent required properties from explicit null,
// rejects duplicates before struct decoding, and never normalizes stored bytes.
func artifactRawObject(raw []byte, keys ...string) (map[string]json.RawMessage, error) {
	fail := func() (map[string]json.RawMessage, error) {
		return nil, artifactFailure("verification_failure", "invalid immutable artifact intent shape")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return fail()
	}
	out := map[string]json.RawMessage{}
	allowed := map[string]bool{}
	for _, k := range keys {
		allowed[k] = true
	}
	for d.More() {
		token, e = d.Token()
		key, ok := token.(string)
		if e != nil || !ok || !allowed[key] || out[key] != nil {
			return fail()
		}
		var value json.RawMessage
		if e = d.Decode(&value); e != nil {
			return fail()
		}
		out[key] = value
	}
	if token, e = d.Token(); e != nil || token != json.Delim('}') || len(out) != len(keys) {
		return fail()
	}
	if _, e = d.Token(); e != io.EOF {
		return fail()
	}
	return out, nil
}
func artifactRawString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", artifactFailure("verification_failure", "artifact intent requires a string")
	}
	canonical, e := artifact.Canonical(raw)
	if e != nil {
		return "", artifactFailure("verification_failure", "invalid artifact intent string")
	}
	var value string
	if e = json.Unmarshal(canonical, &value); e != nil {
		return "", artifactFailure("verification_failure", "invalid artifact intent string")
	}
	return value, nil
}
func artifactRawNull(raw json.RawMessage) bool { return bytes.Equal(raw, []byte("null")) }
func artifactReadIntent(ctx context.Context, path string) (*artifactIntent, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, artifactFailure("repository_error", "cannot read immutable artifact intent")
	}
	f := os.NewFile(uintptr(fd), "artifact intent")
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, artifactFailure("repository_error", "unsafe immutable artifact intent")
	}
	if info.Size() > artifactIntentBytes {
		return nil, artifactFailure("resource_limit", "artifact intent exceeds byte limit")
	}
	raw, e := io.ReadAll(io.LimitReader(f, artifactIntentBytes+1))
	if e != nil {
		return nil, artifactFailure("repository_error", "cannot read immutable artifact intent")
	}
	if len(raw) > artifactIntentBytes {
		return nil, artifactFailure("resource_limit", "artifact intent exceeds byte limit")
	}
	return decodeArtifactIntent(ctx, raw)
}
func decodeArtifactIntent(ctx context.Context, raw []byte) (*artifactIntent, error) {
	m, e := artifactRawObject(raw, "protocol", "repository_id", "actor", "operation", "request", "semantic_request", "stage", "expected_ref", "expected_oid", "candidate", "result")
	if e != nil {
		return nil, e
	}
	intent := &artifactIntent{rawHash: eventID(raw)}
	for key, target := range map[string]*string{"protocol": &intent.Protocol, "repository_id": &intent.RepositoryID, "actor": &intent.Actor, "operation": &intent.Operation, "request": &intent.Request, "stage": &intent.Stage, "expected_ref": &intent.ExpectedRef} {
		*target, e = artifactRawString(m[key])
		if e != nil {
			return nil, e
		}
	}
	fail := func() (*artifactIntent, error) {
		return nil, artifactFailure("verification_failure", "invalid immutable artifact intent binding")
	}
	if intent.Protocol != "hn-artifact-intent/1" || !validMemoryDigestID(intent.RepositoryID) || !validMemoryDigestID(intent.Request) || artifactValidateOperation(intent.Actor, intent.Operation) != nil || intent.ExpectedRef != artifactRef(intent.Actor) {
		return fail()
	}
	if !artifactRawNull(m["expected_oid"]) {
		oid, e := artifactRawString(m["expected_oid"])
		if e != nil || !validGitOID(oid) || oid != strings.ToLower(oid) {
			return fail()
		}
		intent.ExpectedOID = &oid
	}
	canonical, e := artifact.Canonical(m["semantic_request"])
	if e != nil || !bytes.Equal(canonical, m["semantic_request"]) {
		return fail()
	}
	semanticFields, e := artifactRawObject(canonical, "actor", "kind", "operation", "parents", "payload", "protocol", "record")
	if e != nil {
		return nil, e
	}
	var semantic map[string]any
	d := json.NewDecoder(bytes.NewReader(canonical))
	d.UseNumber()
	if e = d.Decode(&semantic); e != nil {
		return fail()
	}
	for _, k := range []string{"actor", "kind", "operation", "protocol", "record"} {
		if _, e = artifactRawString(semanticFields[k]); e != nil {
			return fail()
		}
	}
	kind := artifactText(semantic, "kind")
	switch kind {
	case "type.publish", "record.open", "record.revise", "record.resolve", "record.convert":
	default:
		return fail()
	}
	record := artifactText(semantic, "record")
	if artifactText(semantic, "protocol") != "hn-artifact/1" || artifactText(semantic, "actor") != intent.Actor || artifactText(semantic, "operation") != intent.Operation || (kind == "type.publish" || kind == "record.open") && record != "" || (kind != "type.publish" && kind != "record.open") && !validMemoryDigestID(record) {
		return fail()
	}
	parents, ok := semantic["parents"].([]any)
	if !ok || len(parents) > 1024 {
		return fail()
	}
	last := ""
	for _, v := range parents {
		id, ok := v.(string)
		if !ok || !validMemoryDigestID(id) || id <= last {
			return fail()
		}
		last = id
	}
	if _, ok := semantic["payload"].(map[string]any); !ok {
		return fail()
	}
	if eventID(append([]byte("hn-artifact-request-v1\x00"), canonical...)) != intent.Request {
		return fail()
	}
	if e = artifact.ValidateIntentPayload(ctx, kind, semanticFields["payload"], artifactIDs(semantic["parents"])); e != nil {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact intent validation cancelled")
		}
		return fail()
	}
	intent.SemanticRequest = bytes.Clone(canonical)
	if !artifactRawNull(m["candidate"]) {
		fields, e := artifactRawObject(m["candidate"], "id", "commit")
		if e != nil {
			return nil, e
		}
		id, e := artifactRawString(fields["id"])
		if e != nil {
			return nil, e
		}
		commit, e := artifactRawString(fields["commit"])
		if e != nil || !validMemoryDigestID(id) || !validGitOID(commit) || commit != strings.ToLower(commit) {
			return fail()
		}
		intent.Candidate = &artifactCandidate{id, commit}
	}
	if !artifactRawNull(m["result"]) {
		result, e := decodeArtifactResult(m["result"])
		if e != nil {
			return nil, e
		}
		intent.Result = result
	}
	switch intent.Stage {
	case "prepared":
		if intent.Candidate != nil || intent.Result != nil {
			return fail()
		}
	case "publishing":
		if intent.Candidate == nil || intent.Result != nil {
			return fail()
		}
	case "published":
		if intent.Candidate == nil || intent.Result == nil {
			return fail()
		}
	default:
		return fail()
	}
	if intent.Result != nil {
		result := intent.Result
		logical := intent.Candidate.ID
		if kind != "type.publish" && kind != "record.open" {
			logical = record
		}
		payload := semantic["payload"].(map[string]any)
		if result.TypeID != artifactText(payload, "type_id") || result.EventID != intent.Candidate.ID || result.ResourceID != logical || kind == "type.publish" && (result.HeadCount != 1 || result.Conflict) {
			return fail()
		}
	}
	return intent, nil
}
func decodeArtifactResult(raw []byte) (*artifactMutationResult, error) {
	m, e := artifactRawObject(raw, "protocol", "family", "type_id", "resource_id", "event_id", "replayed", "conflict", "head_count", "admission", "source_cut")
	if e != nil {
		return nil, e
	}
	out := &artifactMutationResult{}
	for key, target := range map[string]*string{"protocol": &out.Protocol, "type_id": &out.TypeID, "resource_id": &out.ResourceID, "event_id": &out.EventID, "source_cut": &out.SourceCut} {
		*target, e = artifactRawString(m[key])
		if e != nil {
			return nil, e
		}
	}
	fail := func() (*artifactMutationResult, error) {
		return nil, artifactFailure("verification_failure", "invalid artifact intent result")
	}
	if !artifactRawNull(m["family"]) || !artifactRawNull(m["admission"]) || out.Protocol != "hn-artifact/1" || out.TypeID == "" || !validMemoryDigestID(out.ResourceID) || !validMemoryDigestID(out.EventID) || !validMemoryDigestID(out.SourceCut) {
		return fail()
	}
	if !bytes.Equal(m["replayed"], []byte("false")) {
		return fail()
	}
	if bytes.Equal(m["conflict"], []byte("true")) {
		out.Conflict = true
	} else if !bytes.Equal(m["conflict"], []byte("false")) {
		return fail()
	}
	count, e := strconv.ParseInt(string(m["head_count"]), 10, 32)
	if e != nil || count < 1 || count > artifactReadEntries || strconv.FormatInt(count, 10) != string(m["head_count"]) || out.Conflict != (count > 1) {
		return fail()
	}
	out.HeadCount = int(count)
	return out, nil
}
func artifactSaveIntent(path string, intent *artifactIntent) error {
	raw, e := json.Marshal(intent)
	if e != nil || len(raw) > artifactIntentBytes {
		return artifactFailure("resource_limit", "artifact intent exceeds byte limit")
	}
	if e = writePrivateFileAtomic(path, raw); e != nil {
		return artifactFailure("repository_error", "cannot persist durable artifact intent")
	}
	return nil
}
func artifactResult(h *artifactHistory, s *storedArtifact, replayed bool) artifactMutationResult {
	logical, heads := s.ID, 1
	if s.verified.Kind() != "type.publish" {
		if s.verified.Kind() != "record.open" {
			logical = artifactText(s.Envelope, "record")
		}
		heads = artifactRecordHeadCount(h, logical)
	}
	return artifactMutationResult{Protocol: "hn-artifact/1", TypeID: artifactText(artifactPayload(s), "type_id"), ResourceID: logical, EventID: s.ID, Replayed: replayed, HeadCount: heads, Conflict: heads > 1, SourceCut: h.Cut}
}

// Exact replay preserves signed result identities while current heads are an
// observation of the complete verified selected view at the returned source cut.
func artifactRecordHeadCount(h *artifactHistory, opening string) int {
	heads := map[string]bool{}
	for _, s := range h.ByID {
		if s.verified.Kind() == "record.open" && s.ID == opening || artifactText(s.Envelope, "record") == opening {
			heads[s.ID] = true
		}
	}
	for _, s := range h.ByID {
		if artifactText(s.Envelope, "record") == opening {
			for _, parent := range artifactIDs(s.Envelope["parents"]) {
				delete(heads, parent)
			}
		}
	}
	return len(heads)
}

func artifactMutationPolicy() error {
	authority, e := loadContributionAuthority()
	if e != nil {
		return artifactFailure("repository_error", "cannot verify selected contribution policy")
	}
	if authority != nil && authority.Policy.Contributions != nil {
		return &artifactStorageError{Origin: "host", Code: "unsupported_capability", Message: "custom mutation under selected contribution policy is unsupported"}
	}
	return nil
}
func artifactMutationIdentity(actor string) (*Identity, error) {
	identity, e := inspectIdentity()
	if e != nil || validateIdentity(identity) != nil {
		return nil, artifactFailure("repository_error", "active native signing identity unavailable")
	}
	if identity.Actor != actor {
		return nil, &artifactStorageError{Origin: "native", Code: "actor_mismatch", Message: "active signing actor differs from the expected actor; repair the binding"}
	}
	return identity, nil
}
func checkArtifactRequest(evidence artifactOperationEvidence, requestBytes []byte, request string) error {
	if evidence.Intent != nil && (evidence.Intent.Request != request || !bytes.Equal(evidence.Intent.SemanticRequest, requestBytes)) {
		return artifactFailure("operation_conflict", "private operation key binds a different immutable request")
	}
	if evidence.Public != nil && (artifactText(evidence.Public.Envelope, "request") != request || !bytes.Equal(artifactSemanticBytes(evidence.Public.Envelope), requestBytes)) {
		return artifactFailure("operation_conflict", "operation key already binds a different immutable request")
	}
	return nil
}

// inspectArtifactMutation is a noncreating preflight shared by current type/open
// and later successor preparation. It never skips malformed private evidence.
func inspectArtifactMutation(ctx context.Context, actor, operation string, requestBytes []byte, request string) (artifactOperationEvidence, error) {
	var zero artifactOperationEvidence
	if e := artifactValidateOperation(actor, operation); e != nil {
		return zero, e
	}
	if e := artifactMutationPolicy(); e != nil {
		return zero, e
	}
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		return zero, e
	}
	repo, _, e := artifactRepositoryID()
	if e != nil {
		return zero, e
	}
	evidence, e := readArtifactOperationEvidence(ctx, h, repo, actor, operation)
	if e != nil {
		return zero, e
	}
	if e = checkArtifactRequest(evidence, requestBytes, request); e != nil {
		return zero, e
	}
	if evidence.Public == nil {
		if evidence.Reason == "unavailable_history" {
			return zero, artifactFailure("uncertain_operation", "original publication witness is unavailable")
		}
		if e = artifactSigningRoot(h, actor); e != nil {
			return zero, e
		}
		if evidence.Intent != nil && artifactExpectedOID(evidence.Intent) != artifactLocalHead(h, actor) {
			return zero, artifactFailure("uncertain_operation", "original expected-old publication witness is no longer current")
		}
	}
	return evidence, nil
}

// writeArtifact's active type/open callers validate prospective input before
// creating a lock; the locked attempt repeats coherent evidence before replay.
func writeArtifact(ctx context.Context, kind, actor, operation string, raw []byte) (artifactMutationResult, error) {
	var zero artifactMutationResult
	if e := artifactValidateOperation(actor, operation); e != nil {
		return zero, e
	}
	input, e := artifactParseMutation(raw, kind)
	if e != nil {
		return zero, e
	}
	semantic, requestBytes, request, e := artifactSemantic(kind, actor, operation, input)
	if e != nil {
		return zero, e
	}
	preflight, e := inspectArtifactMutation(ctx, actor, operation, requestBytes, request)
	if e != nil {
		return zero, e
	}
	if preflight.Public == nil {
		if e = artifactCheckMutation(ctx, preflight.History, kind, input); e != nil {
			return zero, e
		}
	}
	identity, e := artifactMutationIdentity(actor)
	if e != nil {
		return zero, e
	}
	attempt, e := beginArtifactMutation(ctx, identity, semantic, requestBytes, request)
	if e != nil {
		return zero, e
	}
	defer attempt.Close()
	if attempt.Replay != nil {
		return *attempt.Replay, nil
	}
	if e = artifactCheckMutation(ctx, attempt.History, kind, input); e != nil {
		return zero, e
	}
	return appendArtifact(ctx, attempt)
}

// artifactMutationAttempt owns exactly one per-key lock from coherent evidence
// reconciliation through the sole append/finish path. Close is idempotent.
type artifactMutationAttempt struct {
	History      *artifactHistory
	Replay       *artifactMutationResult
	identity     *Identity
	semantic     map[string]any
	requestBytes []byte
	request      string
	evidence     artifactOperationEvidence
	unlock       func()
	used         bool
}

func (a *artifactMutationAttempt) Close() {
	if a != nil && a.unlock != nil {
		a.unlock()
		a.unlock = nil
	}
}
func beginArtifactMutation(ctx context.Context, identity *Identity, semantic map[string]any, requestBytes []byte, request string) (*artifactMutationAttempt, error) {
	actor, operation := artifactText(semantic, "actor"), artifactText(semantic, "operation")
	if e := artifactMutationPolicy(); e != nil {
		return nil, e
	}
	current, e := artifactMutationIdentity(actor)
	if e != nil {
		return nil, e
	}
	if identity == nil || identity.Actor != current.Actor || identity.PublicKey != current.PublicKey || artifactValidateOperation(actor, operation) != nil || !bytes.Equal(requestBytes, artifactSemanticBytes(semantic)) || eventID(append([]byte("hn-artifact-request-v1\x00"), requestBytes...)) != request {
		return nil, artifactFailure("verification_failure", "mutation attempt binding changed")
	}
	if len(semantic) != 7 || validateIdentity(identity) != nil {
		return nil, artifactFailure("verification_failure", "invalid immutable mutation arguments")
	}
	canonical, e := artifact.Canonical(requestBytes)
	if e != nil || !bytes.Equal(canonical, requestBytes) {
		return nil, artifactFailure("verification_failure", "invalid immutable mutation request")
	}
	if _, e = artifactRawObject(canonical, "actor", "kind", "operation", "parents", "payload", "protocol", "record"); e != nil {
		return nil, e
	}
	var pinned map[string]any
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	if e = decoder.Decode(&pinned); e != nil {
		return nil, artifactFailure("verification_failure", "invalid immutable mutation request")
	}
	captured := *identity
	_, unlock, e := artifactIntentLock(actor, operation)
	if e != nil {
		return nil, e
	}
	attempt := &artifactMutationAttempt{identity: &captured, semantic: pinned, requestBytes: bytes.Clone(requestBytes), request: request, unlock: unlock}
	fail := func(e error) (*artifactMutationAttempt, error) { attempt.Close(); return nil, e }
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		return fail(e)
	}
	attempt.History = h
	repo, _, e := artifactRepositoryID()
	if e != nil {
		return fail(e)
	}
	evidence, e := readArtifactOperationEvidence(ctx, h, repo, actor, operation)
	if e != nil {
		return fail(e)
	}
	if e = checkArtifactRequest(evidence, requestBytes, request); e != nil {
		return fail(e)
	}
	if !evidence.coherent {
		return fail(artifactFailure("concurrent_update", "artifact evidence changed during mutation preparation"))
	}
	evidence.settled = true
	classifyArtifactEvidence(&evidence)
	attempt.evidence = evidence
	if evidence.Status == "found" {
		if e = finishArtifactPublication(attempt, evidence.Public); e != nil {
			return fail(e)
		}
		result := artifactResult(h, evidence.Public, true)
		attempt.Replay = &result
		return attempt, nil
	}
	if evidence.Reason == "unavailable_history" {
		return fail(artifactFailure("uncertain_operation", "original publication witness is unavailable"))
	}
	if evidence.Intent != nil && evidence.Intent.Stage == "published" {
		return fail(artifactFailure("uncertain_operation", "recorded publication is unavailable from selected history"))
	}
	head := artifactLocalHead(h, actor)
	if e = artifactSigningRoot(h, actor); e != nil {
		return fail(e)
	}
	if evidence.Intent != nil && artifactExpectedOID(evidence.Intent) != head {
		return fail(artifactFailure("uncertain_operation", "original expected-old publication witness is no longer current"))
	}
	return attempt, nil
}
func finishArtifactPublication(attempt *artifactMutationAttempt, stored *storedArtifact) error {
	intent := attempt.evidence.Intent
	if intent == nil {
		return nil
	}
	if intent.Stage == "published" {
		return nil
	}
	intent.Stage = "published"
	if intent.Candidate == nil {
		intent.Candidate = &artifactCandidate{stored.ID, stored.commit}
	}
	result := artifactResult(attempt.History, stored, false)
	intent.Result = &result
	if e := artifactSaveIntent(attempt.evidence.Path, intent); e != nil {
		return artifactFailure("uncertain_operation", "published artifact result could not be persisted")
	}
	return nil
}

// appendArtifact consumes the already owned attempt; it never reacquires a lock,
// selects a second signer, changes an existing witness, or re-signs a candidate.
func appendArtifact(ctx context.Context, attempt *artifactMutationAttempt) (artifactMutationResult, error) {
	var zero artifactMutationResult
	if attempt == nil || attempt.unlock == nil || attempt.used || attempt.Replay != nil {
		return zero, artifactFailure("repository_error", "artifact mutation attempt is not ready")
	}
	if !bytes.Equal(artifactSemanticBytes(attempt.semantic), attempt.requestBytes) {
		return zero, artifactFailure("verification_failure", "immutable mutation attempt changed")
	}
	attempt.used = true
	identity, h := attempt.identity, attempt.History
	actor, operation, kind := identity.Actor, artifactText(attempt.semantic, "operation"), artifactText(attempt.semantic, "kind")
	ref, expected := artifactRef(actor), artifactLocalHead(h, actor)
	intent := attempt.evidence.Intent
	if intent == nil {
		repo, _, e := artifactRepositoryID()
		if e != nil {
			return zero, e
		}
		var expectedOID *string
		if expected != "" {
			value := expected
			expectedOID = &value
		}
		intent = &artifactIntent{Protocol: "hn-artifact-intent/1", RepositoryID: repo, Actor: actor, Operation: operation, Request: attempt.request, SemanticRequest: attempt.requestBytes, Stage: "prepared", ExpectedRef: ref, ExpectedOID: expectedOID}
		if e = artifactSaveIntent(attempt.evidence.Path, intent); e != nil {
			return zero, e
		}
		attempt.evidence.Intent = intent
	}
	if ctx.Err() != nil {
		return zero, artifactFailure("cancelled", "artifact mutation cancelled before signing")
	}
	candidate := attempt.evidence.Candidate
	if candidate == nil {
		chain := h.Chains[actor]
		envelope := map[string]any{}
		for k, v := range attempt.semantic {
			envelope[k] = v
		}
		envelope["request"] = attempt.request
		envelope["public_key"] = identity.PublicKey
		envelope["stream"] = artifactStream(actor)
		envelope["timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
		envelope["sequence"] = len(chain) + 1
		envelope["previous"] = ""
		var previous *artifact.Verified
		if len(chain) > 0 {
			last := chain[len(chain)-1]
			envelope["previous"] = last.ID
			previous = &last.verified
		}
		payload, e := json.Marshal(envelope)
		if e != nil {
			return zero, artifactFailure("invalid_input", "cannot encode artifact envelope")
		}
		if len(payload) > artifact.MaxEnvelopeBytes {
			return zero, artifactFailure("resource_limit", "artifact envelope exceeds byte limit")
		}
		key, e := identity.privateKey()
		if e != nil {
			return zero, artifactFailure("repository_error", "native signer unavailable")
		}
		signature := base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), payload...)))
		verified, e := artifact.Verify(payload, signature, previous)
		if e != nil {
			return zero, e
		}
		if e = artifactAttemptSources(ctx, attempt); e != nil {
			return zero, e
		}
		commit, e := artifactWriteCandidate(ctx, h.gitDir, payload, signature, verified.ID(), expected)
		if e != nil {
			return zero, e
		}
		candidate = &storedArtifact{ID: verified.ID(), Envelope: envelope, Signature: signature, Validation: "verified", Diagnostics: []any{}, commit: commit, verified: verified, parentCommit: expected}
		intent.Stage = "publishing"
		intent.Candidate = &artifactCandidate{candidate.ID, candidate.commit}
		if e = artifactSaveIntent(attempt.evidence.Path, intent); e != nil {
			return zero, e
		}
	}
	if e := artifactAttemptSources(ctx, attempt); e != nil {
		return zero, e
	}
	if ctx.Err() != nil {
		return zero, artifactFailure("cancelled", "artifact mutation cancelled before publication")
	}
	if _, e := projectGit(ctx, h.gitDir, nil, 4096, "update-ref", "-m", "hn: append "+kind, ref, candidate.commit, artifactExpectedOID(intent)); e != nil {
		if ctx.Err() != nil {
			return zero, artifactFailure("uncertain_operation", "artifact publication acknowledgement unavailable")
		}
		return zero, artifactFailure("concurrent_update", "artifact actor head changed; no retry performed")
	}
	after, e := loadArtifactHistory(ctx)
	if e != nil {
		return zero, artifactFailure("uncertain_operation", "published artifact could not be verified")
	}
	stored := after.ByID[candidate.ID]
	if stored == nil || stored.commit != candidate.commit {
		return zero, artifactFailure("uncertain_operation", "artifact publication not observed")
	}
	attempt.History = after
	if e = finishArtifactPublication(attempt, stored); e != nil {
		return zero, e
	}
	return artifactResult(after, stored, false), nil
}
func artifactAttemptSources(ctx context.Context, attempt *artifactMutationAttempt) error {
	refs, e := artifactSources(ctx, attempt.History.gitDir)
	if e != nil {
		return e
	}
	old, _ := json.Marshal(attempt.History.Sources)
	current, _ := json.Marshal(refs)
	if !bytes.Equal(old, current) {
		return artifactFailure("concurrent_update", "artifact sources changed before publication")
	}
	return nil
}
func artifactWriteCandidate(ctx context.Context, dir string, payload []byte, signature, id, expected string) (string, error) {
	run := func(input []byte, args ...string) (string, error) {
		b, e := projectGit(ctx, dir, input, 4096, args...)
		if e != nil {
			if ctx.Err() != nil {
				return "", artifactFailure("cancelled", "artifact publication preparation cancelled")
			}
			return "", artifactFailure("repository_error", "cannot prepare artifact Git objects")
		}
		return strings.TrimSpace(string(b)), nil
	}
	blob, e := run(payload, "hash-object", "-w", "--stdin")
	if e != nil {
		return "", e
	}
	sig, e := run([]byte(signature), "hash-object", "-w", "--stdin")
	if e != nil {
		return "", e
	}
	tree, e := run([]byte("100644 blob "+blob+"\tartifact.json\n100644 blob "+sig+"\tsignature\n"), "mktree")
	if e != nil {
		return "", e
	}
	args := []string{"-c", "user.name=Hubnot artifact transport", "-c", "user.email=artifact@hn.invalid", "commit-tree", tree, "-m", "hn artifact " + id}
	if expected != "" {
		args = append(args, "-p", expected)
	}
	return run(nil, args...)
}

// artifactOperationEvidence contains cut-scoped facts, never an authorization to
// sign. WP07 can format these same facts without creating a mutation attempt.
type artifactOperationEvidence struct {
	History     *artifactHistory
	Intent      *artifactIntent
	Public      *storedArtifact
	Candidate   *storedArtifact
	Path        string
	Status      string
	Reason      string
	settled     bool
	coherent    bool
	unavailable bool
}

func artifactSemanticBytes(envelope map[string]any) []byte {
	m := map[string]any{}
	for _, key := range []string{"actor", "kind", "operation", "parents", "payload", "protocol", "record"} {
		m[key] = envelope[key]
	}
	raw, _ := json.Marshal(m)
	return raw
}
func artifactExpectedOID(intent *artifactIntent) string {
	if intent.ExpectedOID == nil {
		return ""
	}
	return *intent.ExpectedOID
}
func artifactSigningRoot(h *artifactHistory, actor string) error {
	head := artifactLocalHead(h, actor)
	chain := h.Chains[actor]
	if len(chain) > 0 && (head == "" || chain[len(chain)-1].commit != head) {
		return artifactFailure("actor_stream_conflict", "local signing root is missing or stale against accepted history")
	}
	return nil
}
func artifactLocalHead(h *artifactHistory, actor string) string {
	for _, source := range h.Sources {
		if source.Ref == artifactRef(actor) {
			return source.OID
		}
	}
	return ""
}
func readArtifactOperationEvidence(ctx context.Context, h *artifactHistory, repository, actor, operation string) (artifactOperationEvidence, error) {
	evidence := artifactOperationEvidence{History: h, Status: "uncertain", Reason: "publication_unconfirmed"}
	if e := artifactValidateOperation(actor, operation); e != nil {
		return evidence, e
	}
	path, e := artifactIntentPath(actor, operation, false)
	if e != nil {
		return evidence, e
	}
	evidence.Path = path
	present, settled, e := artifactProbeLock(path)
	if e != nil {
		return evidence, e
	}
	intent, e := artifactReadIntent(ctx, path)
	if e != nil {
		return evidence, e
	}
	evidence.Intent = intent
	for _, stored := range h.Chains[actor] {
		if artifactText(stored.Envelope, "operation") == operation {
			evidence.Public = stored
			break
		}
	}
	if intent != nil {
		format, e := projectGit(ctx, h.gitDir, nil, 80, "rev-parse", "--show-object-format")
		if e != nil {
			return evidence, artifactFailure("repository_error", "cannot verify artifact object format")
		}
		oidLength := 40
		if strings.TrimSpace(string(format)) == "sha256" {
			oidLength = 64
		} else if strings.TrimSpace(string(format)) != "sha1" {
			return evidence, artifactFailure("verification_failure", "unsupported Git object format")
		}
		if intent.ExpectedOID != nil && len(*intent.ExpectedOID) != oidLength || intent.Candidate != nil && len(intent.Candidate.Commit) != oidLength {
			return evidence, artifactFailure("verification_failure", "artifact witness has wrong Git object format")
		}
		if intent.RepositoryID != repository || intent.Actor != actor || intent.Operation != operation {
			return evidence, artifactFailure("verification_failure", "artifact intent belongs to another repository or actor/key")
		}
		if evidence.Public != nil && (intent.Request != artifactText(evidence.Public.Envelope, "request") || !bytes.Equal(intent.SemanticRequest, artifactSemanticBytes(evidence.Public.Envelope))) {
			return evidence, artifactFailure("verification_failure", "private intent contradicts verified public operation")
		}
		if intent.Candidate != nil {
			if evidence.Public != nil {
				if intent.Candidate.ID != evidence.Public.ID || intent.Candidate.Commit != evidence.Public.commit {
					return evidence, artifactFailure("verification_failure", "private candidate contradicts verified publication")
				}
				evidence.Candidate = evidence.Public
			} else {
				objects := make(map[string]bool, len(h.readObjects))
				for oid, seen := range h.readObjects {
					objects[oid] = seen
				}
				reader := artifactObjectReader{ctx: ctx, dir: h.gitDir, total: h.readBytes, seen: objects, scoped: true}
				defer reader.close()
				candidate, _, err := reader.commit(intent.Candidate.Commit)
				if err != nil {
					return evidence, err
				}
				if err := reader.finish(); err != nil {
					return evidence, err
				}
				if candidate.ID != intent.Candidate.ID || candidate.verified.Actor() != actor || !bytes.Equal(artifactSemanticBytes(candidate.Envelope), intent.SemanticRequest) {
					return evidence, artifactFailure("verification_failure", "private candidate does not match immutable request")
				}
				evidence.Candidate = candidate
			}
		}
		witness := evidence.Candidate
		if witness == nil {
			witness = evidence.Public
		}
		if witness != nil {
			if witness.parentCommit != artifactExpectedOID(intent) {
				return evidence, artifactFailure("verification_failure", "artifact candidate differs from expected-old witness")
			}
			var previous *artifact.Verified
			if intent.ExpectedOID != nil {
				for _, s := range h.Chains[actor] {
					if s.commit == *intent.ExpectedOID {
						previous = &s.verified
						break
					}
				}
				if previous == nil {
					evidence.unavailable = true
				}
			}
			if !evidence.unavailable && previous == nil && !witness.verified.Genesis() {
				return evidence, artifactFailure("invalid_history", "private candidate is missing its stream genesis")
			}
			if !evidence.unavailable {
				if _, err := artifact.Verify(witness.verified.Bytes(), witness.Signature, previous); err != nil {
					return evidence, err
				}
			}
		}
	}
	// The second descriptor/settlement observation and source-ref comparison make
	// genuinely unused keys verifiably absent at this Cut, without creating files.
	after, e := artifactReadIntent(ctx, path)
	if e != nil {
		return evidence, e
	}
	presentAfter, settledAfter, e := artifactProbeLock(path)
	if e != nil {
		return evidence, e
	}
	refs, e := artifactSources(ctx, h.gitDir)
	if e != nil {
		return evidence, e
	}
	last, e := artifactReadIntent(ctx, path)
	if e != nil {
		return evidence, e
	}
	presentLast, settledLast, e := artifactProbeLock(path)
	if e != nil {
		return evidence, e
	}
	beforeRefs, _ := json.Marshal(h.Sources)
	afterRefs, _ := json.Marshal(refs)
	sameIntent := intent == nil && after == nil && last == nil || intent != nil && after != nil && last != nil && intent.rawHash == after.rawHash && intent.rawHash == last.rawHash
	evidence.coherent = sameIntent && present == presentAfter && present == presentLast && bytes.Equal(beforeRefs, afterRefs)
	evidence.settled = settled && settledAfter && settledLast
	classifyArtifactEvidence(&evidence)
	return evidence, nil
}
func classifyArtifactEvidence(evidence *artifactOperationEvidence) {
	evidence.Status, evidence.Reason = "uncertain", "publication_unconfirmed"
	if !evidence.coherent {
		return
	}
	if evidence.Public != nil {
		evidence.Status, evidence.Reason = "found", ""
		return
	}
	if evidence.unavailable {
		evidence.Reason = "unavailable_history"
		return
	}
	if !evidence.settled {
		return
	}
	if evidence.Intent == nil {
		evidence.Status, evidence.Reason = "verified_absent", ""
		return
	}
	evidence.Reason = "private_intent_pending"
	if evidence.Intent.Stage == "published" {
		evidence.Reason = "unavailable_history"
	}
}
