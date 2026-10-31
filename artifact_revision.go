package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"hubnot/internal/artifact"
	"strings"
)

// Successor parsing preserves the caller's full payload and ordered members;
// only parents must already be increasing. No latest-head content is inferred.
func artifactParseSuccessor(ctx context.Context, raw []byte, kind string) (map[string]any, error) {
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact mutation cancelled")
	}
	if len(raw) > artifact.MaxEnvelopeBytes {
		return nil, artifactFailure("resource_limit", "mutation input exceeds byte limit")
	}
	canonical, e := artifact.Canonical(raw)
	if e != nil {
		return nil, e
	}
	var input map[string]any
	d := json.NewDecoder(bytes.NewReader(canonical))
	d.UseNumber()
	if e = d.Decode(&input); e != nil || len(input) != 3 {
		return nil, artifactFailure("invalid_input", "successor requires exactly record, parents and payload")
	}
	record, ok := input["record"].(string)
	if !ok || !validMemoryDigestID(record) {
		return nil, artifactFailure("invalid_input", "successor requires full logical opening ID")
	}
	payload, ok := input["payload"].(map[string]any)
	if !ok {
		return nil, artifactFailure("invalid_input", "successor payload must be an object")
	}
	parents, ok := input["parents"].([]any)
	if !ok {
		return nil, artifactFailure("invalid_input", "successor parents must be an array")
	}
	ids := make([]string, len(parents))
	for i, p := range parents {
		var ok bool
		ids[i], ok = p.(string)
		if !ok {
			return nil, artifactFailure("invalid_input", "parent must be a full artifact ID")
		}
	}
	encoded, _ := json.Marshal(payload)
	if e = artifact.ValidateIntentPayload(ctx, kind, encoded, ids); e != nil {
		return nil, e
	}
	return input, nil
}

func writeArtifactSuccessor(ctx context.Context, kind, actor, operation string, raw []byte) (artifactMutationResult, error) {
	var zero artifactMutationResult
	if e := artifactValidateOperation(actor, operation); e != nil {
		return zero, e
	}
	input, e := artifactParseSuccessor(ctx, raw, kind)
	if e != nil {
		return zero, e
	}
	semantic := map[string]any{"protocol": "hn-artifact/1", "kind": kind, "actor": actor, "operation": operation, "record": input["record"], "parents": input["parents"], "payload": input["payload"]}
	requestBytes := artifactSemanticBytes(semantic)
	request := eventID(append([]byte("hn-artifact-request-v1\x00"), requestBytes...))
	preflight, e := inspectArtifactMutation(ctx, actor, operation, requestBytes, request)
	if e != nil {
		return zero, e
	}
	if preflight.Public == nil {
		if e = artifactCheckSuccessor(ctx, preflight.History, kind, input); e != nil {
			return zero, e
		}
		if preflight.Candidate == nil {
			if e = artifactCheckSuccessorSize(preflight.History, actor, requestBytes, request); e != nil {
				return zero, e
			}
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
	if e = artifactCheckSuccessor(ctx, attempt.History, kind, input); e != nil {
		return zero, e
	}
	if attempt.evidence.Candidate == nil {
		if e = artifactCheckSuccessorSize(attempt.History, actor, requestBytes, request); e != nil {
			return zero, e
		}
	}
	return appendArtifact(ctx, attempt)
}

func artifactCheckSuccessor(ctx context.Context, h *artifactHistory, kind string, input map[string]any) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact mutation cancelled")
	}
	payload := input["payload"].(map[string]any)
	openingID := artifactText(input, "record")
	opening := h.ByID[openingID]
	if opening == nil {
		return artifactFailure("incomplete_dependency", "logical record opening missing")
	}
	if opening.verified.Kind() != "record.open" {
		return artifactFailure("invalid_input", "logical record must name an opening")
	}
	typ := h.ByID[artifactText(payload, "type_revision")]
	if typ == nil {
		return artifactFailure("incomplete_dependency", "exact schema declaration missing")
	}
	if typ.verified.Kind() != "type.publish" {
		return artifactFailure("invalid_input", "exact schema is not a type declaration")
	}
	b, _ := json.Marshal(artifactPayload(typ))
	declaration, e := artifact.ValidateType(ctx, b)
	if e != nil {
		return e
	}
	b, _ = json.Marshal(payload)
	parents := artifactIDs(input["parents"])
	if e = declaration.ValidateSuccessor(ctx, kind, b, parents); e != nil {
		return e
	}
	sameType := func(s *storedArtifact) bool {
		p := artifactPayload(s)
		return artifactText(p, "type_id") == declaration.TypeID && artifactText(p, "shape") == declaration.Shape
	}
	if !sameType(opening) {
		return artifactFailure("invalid_input", "successor cannot change logical type identity or shape")
	}
	conversions := []string{}
	for _, id := range parents {
		parent := h.ByID[id]
		if parent == nil {
			return artifactFailure("incomplete_dependency", "exact successor parent missing")
		}
		logical := artifactText(parent.Envelope, "record")
		if parent.verified.Kind() == "record.open" {
			logical = parent.ID
		}
		if !strings.HasPrefix(parent.verified.Kind(), "record.") || logical != openingID || !sameType(parent) {
			return artifactFailure("invalid_input", "parent belongs to another logical record or type")
		}
		different := artifactText(artifactPayload(parent), "type_revision") != artifactText(payload, "type_revision")
		if kind == "record.revise" && different {
			return artifactFailure("invalid_input", "revise must retain exact parent type revision")
		}
		if kind == "record.convert" && !different {
			return artifactFailure("invalid_input", "convert must change exact type revision")
		}
		if different {
			conversions = append(conversions, id)
		}
	}
	sources := artifactIDs(payload["conversion_sources"])
	if !artifactSameIDs(sources, conversions) {
		return artifactFailure("invalid_input", "conversion sources must exactly match changed-schema parents")
	}
	heads := artifactRecordHeads(h, openingID)
	if kind != "record.resolve" && len(heads) > 1 {
		return artifactFailure("resource_conflict", "record has multiple current heads; explicit resolution required")
	}
	if !artifactSameIDs(parents, heads) {
		return artifactFailure("stale_revision", "parents must equal every current record head")
	}
	// Check target lineage after the successor grammar and exact schema. A
	// logical target may retain multiple heads; it does not choose a winner.
	return artifactCheckSuccessorTargets(ctx, h, payload)
}
func artifactCheckSuccessorTargets(ctx context.Context, h *artifactHistory, payload map[string]any) error {
	for _, field := range []string{"members", "relationships"} {
		links, _ := payload[field].([]any)
		for _, v := range links {
			if ctx.Err() != nil {
				return artifactFailure("cancelled", "artifact target validation cancelled")
			}
			link := v.(map[string]any)
			target := h.ByID[artifactText(link, "target")]
			if target == nil {
				if link["required"] == true {
					return artifactFailure("incomplete_dependency", "required "+artifactText(link, "target_mode")+" artifact target missing")
				}
				continue
			}
			if !strings.HasPrefix(target.verified.Kind(), "record.") || link["target_mode"] == "logical" && target.verified.Kind() != "record.open" {
				return artifactFailure("invalid_input", "artifact target has wrong lineage")
			}
		}
	}
	return nil
}
func artifactSameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, id := range a {
		if id != b[i] {
			return false
		}
	}
	return true
}

// Count the exact unsigned envelope width before creating a mutation attempt.
// Public-key/UTC-seconds syntax has fixed width; request and actor-chain fields
// come from this same immutable request/captured cut. This is a size check only,
// never a second envelope publisher, signer or candidate witness.
func artifactCheckSuccessorSize(h *artifactHistory, actor string, requestBytes []byte, request string) error {
	chain := h.Chains[actor]
	previous := ""
	if len(chain) > 0 {
		previous = chain[len(chain)-1].ID
	}
	metadata := map[string]any{"request": request, "public_key": strings.Repeat("A", base64.RawStdEncoding.EncodedLen(ed25519.PublicKeySize)), "stream": artifactStream(actor), "timestamp": "2000-01-01T00:00:00Z", "sequence": len(chain) + 1, "previous": previous}
	encoded, e := json.Marshal(metadata)
	if e != nil {
		return artifactFailure("invalid_input", "cannot size prospective artifact envelope")
	}
	// Merging two nonempty canonical JSON objects removes two braces and adds
	// one comma. The seven semantic fields and six metadata fields are disjoint.
	if len(requestBytes)+len(encoded)-1 > artifact.MaxEnvelopeBytes {
		return artifactFailure("resource_limit", "prospective artifact envelope exceeds byte limit")
	}
	return nil
}
