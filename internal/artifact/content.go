package artifact

import (
	"context"
	"encoding/json"
	"strings"
)

// RecordValidation describes only a supplied candidate's exact schema and
// immediate record rules. It never attests accepted-view or ancestor closure.
type RecordValidation struct {
	ID                string `json:"id"`
	TypeRevision      string `json:"type_revision"`
	Integrity         string `json:"integrity"`
	SchemaValidation  string `json:"schema_validation"`
	RecordRules       string `json:"record_rules"`
	ActorHistory      string `json:"actor_history"`
	TypeLineage       string `json:"type_lineage"`
	CurrentHeads      string `json:"current_heads"`
	DependencyClosure string `json:"dependency_closure"`
}

type suppliedArtifact struct {
	verified          Verified
	envelope, payload map[string]any
}

func checkSupplied(ctx context.Context, v any) (suppliedArtifact, error) {
	if e := cancelled(ctx); e != nil {
		return suppliedArtifact{}, e
	}
	m, ok := v.(map[string]any)
	if !ok || !closed(m, "id", "envelope", "signature", "validation", "diagnostics") {
		return suppliedArtifact{}, refuse("invalid_input", "expected a normal signed artifact")
	}
	diagnostics, ok := m["diagnostics"].([]any)
	if !ok || len(diagnostics) != 0 || text(m, "validation") != "verified" {
		return suppliedArtifact{}, refuse("invalid_input", "supplied validation labels must be verified with empty diagnostics")
	}
	env, ok := m["envelope"].(map[string]any)
	if !ok {
		return suppliedArtifact{}, refuse("invalid_input", "supplied envelope must be object")
	}
	raw, e := json.Marshal(env)
	if e != nil {
		return suppliedArtifact{}, refuse("invalid_input", "invalid supplied envelope")
	}
	checked, e := Verify(raw, text(m, "signature"), nil)
	if e != nil {
		return suppliedArtifact{}, e
	}
	id, ok := m["id"].(string)
	if !ok || !digestPattern.MatchString(id) {
		return suppliedArtifact{}, refuse("invalid_input", "supplied artifact ID must be full digest")
	}
	if id != checked.ID() {
		return suppliedArtifact{}, refuse("verification_failure", "supplied artifact ID mismatch")
	}
	return suppliedArtifact{checked, env, env["payload"].(map[string]any)}, nil
}

// ValidateRecord rechecks every signature/ID and validates the candidate against
// the exact supplied declaration and immediate parents. It does no I/O and
// resolves no history, current heads, relationship targets or namespace names.
func ValidateRecord(ctx context.Context, raw []byte) (RecordValidation, error) {
	v, e := validationValue(ctx, raw, MaxValidationBytes)
	if e != nil {
		return RecordValidation{}, e
	}
	m, ok := v.(map[string]any)
	if !ok || !closed(m, "artifact", "type", "parents", "opening") {
		return RecordValidation{}, refuse("invalid_input", "record validation requires artifact, type, parents and opening")
	}
	candidate, e := checkSupplied(ctx, m["artifact"])
	if e != nil {
		return RecordValidation{}, e
	}
	if !strings.HasPrefix(candidate.verified.Kind(), "record.") {
		return RecordValidation{}, refuse("invalid_input", "candidate must be a record")
	}
	declaration, e := checkSupplied(ctx, m["type"])
	if e != nil {
		return RecordValidation{}, e
	}
	if declaration.verified.Kind() != "type.publish" || declaration.verified.ID() != text(candidate.payload, "type_revision") {
		return RecordValidation{}, refuse("invalid_input", "type must be the exact selected declaration")
	}
	typ, e := typeDeclaration(ctx, declaration.payload)
	if e != nil {
		return RecordValidation{}, e
	}
	if text(candidate.payload, "type_id") != typ.TypeID || text(candidate.payload, "shape") != typ.Shape {
		return RecordValidation{}, refuse("invalid_input", "candidate type identity or shape mismatch")
	}
	if e = typ.schema.validate(ctx, candidate.payload["fields"], 0); e != nil {
		return RecordValidation{}, e
	}
	provided, ok := m["parents"].([]any)
	if !ok {
		return RecordValidation{}, refuse("invalid_input", "parents must be array")
	}
	expected, _ := ids(candidate.envelope["parents"])
	parents := make([]suppliedArtifact, 0, len(provided))
	for _, value := range provided {
		p, e := checkSupplied(ctx, value)
		if e != nil {
			return RecordValidation{}, e
		}
		parents = append(parents, p)
	}
	if len(parents) < len(expected) {
		return RecordValidation{}, refuse("incomplete_dependency", "required immediate parents not supplied")
	}
	if len(parents) > len(expected) {
		return RecordValidation{}, refuse("invalid_input", "extra supplied parents")
	}
	for i, p := range parents {
		if p.verified.ID() != expected[i] {
			return RecordValidation{}, refuse("invalid_input", "supplied parents must match exact increasing parent IDs")
		}
	}
	if candidate.verified.Kind() == "record.open" {
		if m["opening"] != nil {
			return RecordValidation{}, refuse("invalid_input", "opening candidate requires null opening context")
		}
	} else {
		if m["opening"] == nil {
			return RecordValidation{}, refuse("incomplete_dependency", "required record opening not supplied")
		}
		opening, e := checkSupplied(ctx, m["opening"])
		if e != nil {
			return RecordValidation{}, e
		}
		logical := text(candidate.envelope, "record")
		if opening.verified.Kind() != "record.open" || opening.verified.ID() != logical {
			return RecordValidation{}, refuse("invalid_input", "opening does not match logical record")
		}
		if !sameRecordType(candidate, opening) {
			return RecordValidation{}, refuse("invalid_input", "opening type identity or shape mismatch")
		}
		conversions := []string{}
		for _, p := range parents {
			if !strings.HasPrefix(p.verified.Kind(), "record.") || !sameRecordType(candidate, p) {
				return RecordValidation{}, refuse("invalid_input", "parent type identity or shape mismatch")
			}
			if (p.verified.Kind() == "record.open" && p.verified.ID() != logical) || (p.verified.Kind() != "record.open" && text(p.envelope, "record") != logical) {
				return RecordValidation{}, refuse("invalid_input", "parent belongs to another logical record")
			}
			different := text(p.payload, "type_revision") != text(candidate.payload, "type_revision")
			switch candidate.verified.Kind() {
			case "record.revise":
				if different {
					return RecordValidation{}, refuse("invalid_input", "revise must retain exact parent type revision")
				}
			case "record.convert":
				if !different {
					return RecordValidation{}, refuse("invalid_input", "convert must change exact type revision")
				}
			}
			if different {
				conversions = append(conversions, p.verified.ID())
			}
		}
		actual, _ := ids(candidate.payload["conversion_sources"])
		if len(actual) != len(conversions) {
			return RecordValidation{}, refuse("invalid_input", "conversion sources do not match changed-schema parents")
		}
		for i, id := range conversions {
			if actual[i] != id {
				return RecordValidation{}, refuse("invalid_input", "conversion sources do not match changed-schema parents")
			}
		}
	}
	if e = cancelled(ctx); e != nil {
		return RecordValidation{}, e
	}
	return RecordValidation{candidate.verified.ID(), declaration.verified.ID(), "verified", "verified", "verified_from_supplied_context", "not_checked", "not_checked", "not_checked", "not_checked"}, nil
}
func sameRecordType(a, b suppliedArtifact) bool {
	return text(a.payload, "type_id") == text(b.payload, "type_id") && text(a.payload, "shape") == text(b.payload, "shape")
}
