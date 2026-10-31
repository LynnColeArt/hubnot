package artifact

import "context"

// ValidateSuccessor validates a complete unsigned successor against the already
// selected immutable declaration. It does not resolve history or current heads.
// ValidateIntentPayload guards kind and sorted bounded parent counts before the
// shared payload grammar can index a conversion parent.
func (t TypeDeclaration) ValidateSuccessor(ctx context.Context, kind string, raw []byte, parents []string) error {
	switch kind {
	case "record.revise", "record.resolve", "record.convert":
	default:
		return refuse("invalid_input", "expected a record successor kind")
	}
	if t.schema == nil {
		return refuse("invalid_input", "successor requires a validated exact declaration")
	}
	if err := ValidateIntentPayload(ctx, kind, raw, parents); err != nil {
		return err
	}
	value, err := validationValue(ctx, raw, MaxEnvelopeBytes)
	if err != nil {
		return err
	}
	payload := value.(map[string]any)
	if text(payload, "type_id") != t.TypeID || text(payload, "shape") != t.Shape {
		return refuse("invalid_input", "successor type identity or shape mismatch")
	}
	return t.schema.validate(ctx, payload["fields"], 0)
}
