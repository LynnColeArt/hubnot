package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

// This projection deliberately selects only public operation facts. The private
// intent, candidate commit and lock evidence never cross the read boundary.
type artifactOperationResult struct {
	Status     string                  `json:"status"`
	Protocol   string                  `json:"protocol"`
	Actor      string                  `json:"actor"`
	Operation  string                  `json:"operation"`
	Family     *string                 `json:"family"`
	TypeID     string                  `json:"type_id"`
	Request    *string                 `json:"request"`
	ResourceID *string                 `json:"resource_id"`
	EventID    *string                 `json:"event_id"`
	Result     *artifactMutationResult `json:"result"`
	SourceCut  string                  `json:"source_cut"`
	Reason     *string                 `json:"reason"`
}

var artifactOperationType = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,63}/[a-z][a-z0-9.-]{0,63}$`)

// inspectArtifactOperation is a read, not a mutation preflight: no policy write
// check, signer, creating lock, intent repair or implicit retry is permitted.
func inspectArtifactOperation(ctx context.Context, typeID, actor, operation string) (artifactOperationResult, error) {
	var result artifactOperationResult
	if !artifactOperationType.MatchString(typeID) || strings.HasPrefix(typeID, "hn/") || strings.HasPrefix(typeID, "hubnot/") {
		return result, artifactFailure("invalid_input", "requires a valid custom type ID")
	}
	if err := artifactValidateOperation(actor, operation); err != nil {
		return result, err
	}
	history, err := loadArtifactHistory(ctx)
	if err != nil {
		return result, err
	}
	repository, _, err := artifactRepositoryID()
	if err != nil {
		return result, err
	}
	evidence, err := readArtifactOperationEvidence(ctx, history, repository, actor, operation)
	if err != nil {
		return result, err
	}
	// The key spans all custom types and kinds. A private request must agree with
	// the selector too, even while its publication remains unconfirmed.
	if evidence.Intent != nil {
		var semantic map[string]any
		if err := json.Unmarshal(evidence.Intent.SemanticRequest, &semantic); err != nil {
			return result, artifactFailure("verification_failure", "invalid immutable artifact request")
		}
		payload, _ := semantic["payload"].(map[string]any)
		if artifactText(payload, "type_id") != typeID {
			return result, artifactFailure("operation_conflict", "operation key binds a different custom type")
		}
	}
	if evidence.Public != nil && artifactText(artifactPayload(evidence.Public), "type_id") != typeID {
		return result, artifactFailure("operation_conflict", "operation key binds a different custom type")
	}
	result = artifactOperationResult{Status: evidence.Status, Protocol: "hn-artifact/1", Actor: actor, Operation: operation, TypeID: typeID, SourceCut: history.Cut}
	if evidence.Reason != "" {
		reason := evidence.Reason
		result.Reason = &reason
	}
	if evidence.Status == "found" {
		observed := artifactResult(history, evidence.Public, false)
		request := artifactText(evidence.Public.Envelope, "request")
		result.Request, result.ResourceID, result.EventID = &request, &observed.ResourceID, &observed.EventID
		result.Result = &observed
	}
	return result, nil
}
