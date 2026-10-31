package artifact

import "context"

// Canonical validates the bounded hn-artifact JSON value domain before returning
// C(value). Unlike signed-envelope Verify, unsigned mutation input may include
// noncanonical spacing and object key order.
func Canonical(raw []byte) ([]byte, error) {
	_, canonical, err := canonicalValue(raw)
	return canonical, err
}

// ValidateOpening checks complete unsigned opening payloads before a writer
// creates an immutable intent or asks the native signer to sign. It uses the
// exact declaration already validated by ValidateType, never a latest-name lookup.
func (t TypeDeclaration) ValidateOpening(ctx context.Context, raw []byte) error {
	if t.schema == nil {
		return refuse("invalid_input", "opening requires a validated exact declaration")
	}
	value, err := validationValue(ctx, raw, MaxEnvelopeBytes)
	if err != nil {
		return err
	}
	payload, ok := value.(map[string]any)
	if !ok {
		return refuse("invalid_input", "opening payload must be an object")
	}
	if err = payloadEnvelope("record.open", payload, nil); err != nil {
		return err
	}
	if text(payload, "type_id") != t.TypeID || text(payload, "shape") != t.Shape {
		return refuse("invalid_input", "opening type identity or shape mismatch")
	}
	return t.schema.validate(ctx, payload["fields"], 0)
}

// ValidateRecordContext shares the existing immediate record rules with the
// bounded accepted-history loader. The caller has already checked the strict
// JSON value domain and each signed envelope, bounded the parent/model objects,
// and charged the complete history to its read budget. This avoids serializing
// that graph through the inert utility's16MiB wire cap. It grants no accepted-view
// authority; the shared core rechecks individual signatures and exact identities.
func ValidateRecordContext(ctx context.Context, value map[string]any) (RecordValidation, error) {
	return validateRecordValue(ctx, value)
}
