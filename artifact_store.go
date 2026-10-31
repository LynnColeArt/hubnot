package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hubnot/internal/artifact"
	"os"
	"path/filepath"
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
func artifactIntentLock(actor, operation string) (string, func(), error) {
	_, common, e := artifactRepositoryID()
	if e != nil {
		return "", nil, e
	}
	path := common
	for _, name := range []string{"hn", "artifacts", "intents", actor} {
		path = filepath.Join(path, name)
		if e = ensurePrivateDirectory(path); e != nil {
			return "", nil, artifactFailure("repository_error", "unsafe artifact intent directory")
		}
	}
	key := strings.TrimPrefix(eventID([]byte(operation)), "sha256:")
	file := filepath.Join(path, key+".lock")
	fd, e := syscall.Open(file, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
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
	return filepath.Join(path, key+".json"), func() { _ = syscall.Flock(fd, syscall.LOCK_UN); _ = f.Close() }, nil
}
func artifactReadIntent(path string) (*artifactIntent, error) {
	if info, e := os.Lstat(path); e == nil && info.Mode().IsRegular() && info.Size() > artifactIntentBytes {
		return nil, artifactFailure("resource_limit", "artifact intent exceeds byte limit")
	}
	raw, e := readPrivateFileBounded(path, artifactIntentBytes)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, artifactFailure("repository_error", "cannot read immutable artifact intent")
	}
	var intent artifactIntent
	if e = decodePrivateJSON(raw, &intent, "artifact intent"); e != nil {
		return nil, artifactFailure("verification_failure", "invalid artifact intent")
	}
	switch intent.Stage {
	case "prepared":
		if intent.Candidate != nil || intent.Result != nil {
			return nil, artifactFailure("verification_failure", "invalid prepared artifact intent")
		}
	case "publishing":
		if intent.Candidate == nil || intent.Result != nil {
			return nil, artifactFailure("verification_failure", "invalid publishing artifact intent")
		}
	case "published":
		if intent.Candidate == nil || intent.Result == nil {
			return nil, artifactFailure("verification_failure", "invalid published artifact intent")
		}
	default:
		return nil, artifactFailure("verification_failure", "unknown artifact intent stage")
	}
	if intent.Candidate != nil && (!validMemoryDigestID(intent.Candidate.ID) || !validGitOID(intent.Candidate.Commit)) {
		return nil, artifactFailure("verification_failure", "invalid artifact publication candidate")
	}
	return &intent, nil
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

// writeArtifact is the only custom writer. Its current production callers are
// type publish and record open; transport and record revision use later slices.
func writeArtifact(ctx context.Context, kind, actor, operation string, raw []byte) (artifactMutationResult, error) {
	var zero artifactMutationResult
	if e := artifactValidateOperation(actor, operation); e != nil {
		return zero, e
	}
	input, e := artifactParseMutation(raw, kind)
	if e != nil {
		return zero, e
	}
	authority, e := loadContributionAuthority()
	if e != nil {
		return zero, artifactFailure("repository_error", "cannot verify selected contribution policy")
	}
	if authority != nil && authority.Policy.Contributions != nil {
		return zero, &artifactStorageError{Origin: "host", Code: "unsupported_capability", Message: "custom mutation under selected contribution policy is unsupported"}
	}
	semantic, requestBytes, request, e := artifactSemantic(kind, actor, operation, input)
	if e != nil {
		return zero, e
	}
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		return zero, e
	}
	replay, e := artifactReplay(h, actor, operation, request)
	if e != nil {
		return zero, e
	}
	if replay == nil {
		if e = artifactCheckMutation(ctx, h, kind, input); e != nil {
			return zero, e
		}
	}
	identity, e := inspectIdentity()
	if e != nil || validateIdentity(identity) != nil {
		return zero, artifactFailure("repository_error", "active native signing identity unavailable")
	}
	if identity.Actor != actor {
		return zero, &artifactStorageError{Origin: "native", Code: "actor_mismatch", Message: "active signing actor differs from the expected actor; repair the binding"}
	}
	if replay != nil {
		return artifactResult(h, replay, true), nil
	}
	return appendArtifact(ctx, identity, h, semantic, requestBytes, request)
}

// appendArtifact is the shared publication boundary already used by type/open.
// Callers prepare and validate complete semantic input against the supplied
// verified source cut before entering this one intent/signer/CAS implementation.
func appendArtifact(ctx context.Context, identity *Identity, h *artifactHistory, semantic map[string]any, requestBytes []byte, request string) (artifactMutationResult, error) {
	var zero artifactMutationResult
	actor, operation, kind := identity.Actor, artifactText(semantic, "operation"), artifactText(semantic, "kind")
	var e error
	ref := artifactRef(actor)
	expected := ""
	for _, s := range h.Sources {
		if s.Ref == ref {
			expected = s.OID
		}
	}
	chain := h.Chains[actor]
	if len(chain) > 0 && (expected == "" || chain[len(chain)-1].commit != expected) {
		return zero, artifactFailure("actor_stream_conflict", "local signing root is missing or stale against accepted history")
	}
	repo, _, e := artifactRepositoryID()
	if e != nil {
		return zero, e
	}
	path, unlock, e := artifactIntentLock(actor, operation)
	if e != nil {
		return zero, e
	}
	defer unlock()
	intent, e := artifactReadIntent(path)
	if e != nil {
		return zero, e
	}
	if intent != nil {
		if intent.Protocol != "hn-artifact-intent/1" || intent.RepositoryID != repo || intent.Actor != actor || intent.Operation != operation || intent.Request != request || !bytes.Equal(intent.SemanticRequest, requestBytes) {
			return zero, artifactFailure("operation_conflict", "private operation key binds a different immutable request")
		}
		if intent.ExpectedRef != ref || intent.ExpectedOID != nil && (!validGitOID(*intent.ExpectedOID) || *intent.ExpectedOID != strings.ToLower(*intent.ExpectedOID)) {
			return zero, artifactFailure("verification_failure", "invalid immutable artifact publication witness")
		}
		if intent.Stage != "prepared" || intent.Candidate != nil || intent.Result != nil {
			return zero, artifactFailure("uncertain_operation", "prior publication requires explicit operation recovery")
		}
	}
	var expectedOID *string
	if expected != "" {
		copy := expected
		expectedOID = &copy
	}
	intent = &artifactIntent{Protocol: "hn-artifact-intent/1", RepositoryID: repo, Actor: actor, Operation: operation, Request: request, SemanticRequest: requestBytes, Stage: "prepared", ExpectedRef: ref, ExpectedOID: expectedOID}
	if e = artifactSaveIntent(path, intent); e != nil {
		return zero, e
	}
	if ctx.Err() != nil {
		return zero, artifactFailure("cancelled", "artifact mutation cancelled before signing")
	}
	envelope := map[string]any{}
	for k, v := range semantic {
		envelope[k] = v
	}
	envelope["request"] = request
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
	v, e := artifact.Verify(payload, signature, previous)
	if e != nil {
		return zero, e
	}
	// Sources are checked again before any public object, and update-ref still
	// carries the captured expected-old value. No CAS retry reloads or resigns.
	dir, e := requireGitRepository()
	if e != nil {
		return zero, artifactFailure("repository_error", "Git repository unavailable")
	}
	now, e := artifactSources(ctx, dir)
	if e != nil {
		return zero, e
	}
	a, _ := json.Marshal(h.Sources)
	b, _ := json.Marshal(now)
	if !bytes.Equal(a, b) {
		return zero, artifactFailure("concurrent_update", "artifact sources changed before publication")
	}
	candidate, e := artifactWriteCandidate(ctx, dir, payload, signature, v.ID(), expected)
	if e != nil {
		return zero, e
	}
	intent.Stage = "publishing"
	intent.Candidate = &artifactCandidate{v.ID(), candidate}
	if e = artifactSaveIntent(path, intent); e != nil {
		return zero, e
	}
	if ctx.Err() != nil {
		return zero, artifactFailure("cancelled", "artifact mutation cancelled before publication")
	}
	if _, e = projectGit(ctx, dir, nil, 4096, "update-ref", "-m", "hn: append "+kind, ref, candidate, expected); e != nil {
		if ctx.Err() != nil {
			return zero, artifactFailure("uncertain_operation", "artifact publication acknowledgement unavailable")
		}
		return zero, artifactFailure("concurrent_update", "artifact actor head changed; no retry performed")
	}
	verified, e := loadArtifactHistory(ctx)
	if e != nil {
		return zero, artifactFailure("uncertain_operation", "published artifact could not be verified")
	}
	stored := verified.ByID[v.ID()]
	if stored == nil {
		return zero, artifactFailure("uncertain_operation", "artifact publication not observed")
	}
	result := artifactResult(verified, stored, false)
	intent.Stage = "published"
	intent.Result = &result
	if e = artifactSaveIntent(path, intent); e != nil {
		return zero, artifactFailure("uncertain_operation", "published artifact result could not be persisted")
	}
	return result, nil
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

// artifactReplay runs before mutable record-head checks. History loading already
// refuses duplicate actor/key matches; a request mismatch is never a new append.
func artifactReplay(h *artifactHistory, actor, operation, request string) (*storedArtifact, error) {
	for _, s := range h.Chains[actor] {
		if artifactText(s.Envelope, "operation") == operation {
			if artifactText(s.Envelope, "request") != request {
				return nil, artifactFailure("operation_conflict", "operation key already binds a different immutable request")
			}
			return s, nil
		}
	}
	return nil, nil
}
