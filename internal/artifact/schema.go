package artifact

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"unicode/utf8"
)

const MaxTypeBytes = 131072
const MaxSchemaBytes = 65536
const MaxValidationBytes = 16777216

// TypeDeclaration describes an inert, exact declaration. A validated name does
// not establish namespace ownership, signature integrity or publication.
type TypeDeclaration struct {
	TypeID        string
	SchemaVersion int64
	Shape         string
	schema        *Schema
}

// Schema is the closed hn.schema/1 grammar; it contains no executable hooks.
// Its fields are private so only a completely checked schema can be evaluated.
type Schema struct{ node map[string]any }

func cancelled(ctx context.Context) error {
	if ctx.Err() != nil {
		return refuse("cancelled", "artifact validation cancelled")
	}
	return nil
}

// validationValue shares the envelope's strict token/value domain, while the
// aggregate record-context wrapper has its own larger, finite consumption cap.
func validationValue(ctx context.Context, raw []byte, limit int) (any, error) {
	if e := cancelled(ctx); e != nil {
		return nil, e
	}
	if len(raw) > limit {
		return nil, refuse("resource_limit", "validation input exceeds byte limit")
	}
	if !utf8.Valid(raw) {
		return nil, refuse("invalid_input", "malformed UTF-8")
	}
	if e := unicodeEscapes(raw); e != nil {
		return nil, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, e := value(d, 0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, refuse("invalid_input", "trailing JSON data")
	}
	if e = cancelled(ctx); e != nil {
		return nil, e
	}
	return v, nil
}
func sized(v any, limit int) error {
	b, e := json.Marshal(v)
	if e != nil {
		return refuse("invalid_input", "invalid JSON value")
	}
	if len(b) > limit {
		return refuse("resource_limit", "canonical value exceeds byte limit")
	}
	return nil
}

// ValidateType checks a complete TypePayload, without selecting a signer or a
// latest version. Signed identity is checked separately by ValidateRecord.
func ValidateType(ctx context.Context, raw []byte) (TypeDeclaration, error) {
	v, e := validationValue(ctx, raw, MaxTypeBytes)
	if e != nil {
		return TypeDeclaration{}, e
	}
	return typeDeclaration(ctx, v)
}
func typeDeclaration(ctx context.Context, v any) (TypeDeclaration, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return TypeDeclaration{}, refuse("invalid_input", "type payload must be an object")
	}
	if e := payloadEnvelope("type.publish", m, nil); e != nil {
		return TypeDeclaration{}, e
	}
	schema, e := schemaDeclaration(ctx, m["schema"])
	if e != nil {
		return TypeDeclaration{}, e
	}
	n, _ := integer(m["schema_version"])
	return TypeDeclaration{TypeID: text(m, "type_id"), SchemaVersion: n, Shape: text(m, "shape"), schema: schema}, nil
}

// ValidateSchema accepts only the complete hn.schema/1 language wrapper.
func ValidateSchema(ctx context.Context, raw []byte) (*Schema, error) {
	v, e := validationValue(ctx, raw, MaxSchemaBytes)
	if e != nil {
		return nil, e
	}
	return schemaDeclaration(ctx, v)
}
func schemaDeclaration(ctx context.Context, v any) (*Schema, error) {
	if e := cancelled(ctx); e != nil {
		return nil, e
	}
	m, ok := v.(map[string]any)
	if !ok || !closed(m, "language", "root") {
		return nil, refuse("invalid_input", "schema requires language and root")
	}
	language, ok := m["language"].(string)
	if !ok {
		return nil, refuse("invalid_input", "schema language must be a string")
	}
	if language != "hn.schema/1" {
		return nil, refuse("unsupported_schema", "unsupported schema language")
	}
	if e := sized(m, MaxSchemaBytes); e != nil {
		return nil, e
	}
	root, ok := m["root"].(map[string]any)
	if !ok || text(root, "type") != "object" {
		return nil, refuse("invalid_input", "schema root must be object")
	}
	if e := checkNode(ctx, root, 0); e != nil {
		return nil, e
	}
	return &Schema{root}, nil
}
func allowedKeys(m map[string]any, required []string, optional ...string) bool {
	keys := map[string]bool{}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return false
		}
		keys[k] = true
	}
	for _, k := range optional {
		keys[k] = true
	}
	for k := range m {
		if !keys[k] {
			return false
		}
	}
	return true
}
func checkNode(ctx context.Context, v any, depth int) error {
	if e := cancelled(ctx); e != nil {
		return e
	}
	m, ok := v.(map[string]any)
	if !ok {
		return refuse("invalid_input", "schema node must be object")
	}
	switch text(m, "type") {
	case "object":
		if !allowedKeys(m, []string{"type", "properties", "required", "additionalProperties"}) || m["additionalProperties"] != false {
			return refuse("invalid_input", "invalid object schema keywords")
		}
		depth++
		if depth > 8 {
			return refuse("resource_limit", "schema exceeds 8 container levels")
		}
		props, ok := m["properties"].(map[string]any)
		if !ok {
			return refuse("invalid_input", "properties must be object")
		}
		if len(props) > 64 {
			return refuse("resource_limit", "schema exceeds 64 properties")
		}
		req, ok := m["required"].([]any)
		if !ok {
			return refuse("invalid_input", "required must be array")
		}
		if len(req) > 1024 {
			return refuse("resource_limit", "required exceeds 1024 entries")
		}
		seen := map[string]bool{}
		for _, r := range req {
			s, ok := r.(string)
			_, present := props[s]
			if !ok || !present || seen[s] {
				return refuse("invalid_input", "required must name distinct declared fields")
			}
			seen[s] = true
		}
		for k, n := range props {
			if !fieldPattern.MatchString(k) {
				return refuse("invalid_input", "invalid schema field name")
			}
			if e := checkNode(ctx, n, depth); e != nil {
				return e
			}
		}
	case "array":
		if !allowedKeys(m, []string{"type", "items", "maxItems"}) {
			return refuse("invalid_input", "invalid array schema keywords")
		}
		depth++
		if depth > 8 {
			return refuse("resource_limit", "schema exceeds 8 container levels")
		}
		n, ok := integer(m["maxItems"])
		if !ok || n < 0 || n > 1024 {
			return refuse("invalid_input", "maxItems must be integer 0..1024")
		}
		return checkNode(ctx, m["items"], depth)
	case "string":
		if !allowedKeys(m, []string{"type", "maxBytes"}, "enum") {
			return refuse("invalid_input", "invalid string schema keywords")
		}
		n, ok := integer(m["maxBytes"])
		if !ok || n < 0 || n > MaxEnvelopeBytes {
			return refuse("invalid_input", "maxBytes must be integer 0..1048576")
		}
	case "integer":
		if !allowedKeys(m, []string{"type"}, "minimum", "maximum", "enum") {
			return refuse("invalid_input", "invalid integer schema keywords")
		}
		for _, k := range []string{"minimum", "maximum"} {
			if v, exists := m[k]; exists {
				if _, ok := integer(v); !ok {
					return refuse("invalid_input", "integer bound must be safe integer")
				}
			}
		}
		lo, lok := integer(m["minimum"])
		hi, hok := integer(m["maximum"])
		if lok && hok && lo > hi {
			return refuse("invalid_input", "minimum exceeds maximum")
		}
	case "boolean":
		if !allowedKeys(m, []string{"type"}, "enum") {
			return refuse("invalid_input", "invalid boolean schema keywords")
		}
	case "null":
		if !closed(m, "type") {
			return refuse("invalid_input", "invalid null schema keywords")
		}
	default:
		return refuse("invalid_input", "unsupported schema node type")
	}
	if ev, exists := m["enum"]; exists {
		enum, ok := ev.([]any)
		if !ok || len(enum) == 0 {
			return refuse("invalid_input", "enum must be nonempty array")
		}
		if len(enum) > 1024 {
			return refuse("resource_limit", "enum exceeds 1024 entries")
		}
		seen := map[string]bool{}
		for _, v := range enum {
			if e := scalar(m, v); e != nil {
				return e
			}
			b, _ := json.Marshal(v)
			if seen[string(b)] {
				return refuse("invalid_input", "enum values must be distinct")
			}
			seen[string(b)] = true
		}
	}
	return nil
}
func scalar(m map[string]any, v any) error {
	switch text(m, "type") {
	case "string":
		s, ok := v.(string)
		n, _ := integer(m["maxBytes"])
		if !ok || int64(len(s)) > n {
			return refuse("invalid_input", "string does not satisfy schema")
		}
	case "integer":
		n, ok := integer(v)
		if !ok {
			return refuse("invalid_input", "value must be integer")
		}
		if lo, exists := integer(m["minimum"]); exists && n < lo {
			return refuse("invalid_input", "integer below minimum")
		}
		if hi, exists := integer(m["maximum"]); exists && n > hi {
			return refuse("invalid_input", "integer above maximum")
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return refuse("invalid_input", "value must be boolean")
		}
	case "null":
		if v != nil {
			return refuse("invalid_input", "value must be null")
		}
	}
	return nil
}
func (s *Schema) validate(ctx context.Context, v any, depth int) error {
	return validateNode(ctx, s.node, v, depth)
}
func validateNode(ctx context.Context, m map[string]any, v any, depth int) error {
	if e := cancelled(ctx); e != nil {
		return e
	}
	switch text(m, "type") {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return refuse("invalid_input", "value must be object")
		}
		depth++
		if depth > 8 || len(obj) > 64 {
			return refuse("resource_limit", "value exceeds object bounds")
		}
		props := m["properties"].(map[string]any)
		for _, r := range m["required"].([]any) {
			if _, ok := obj[r.(string)]; !ok {
				return refuse("invalid_input", "required field missing")
			}
		}
		for k, v := range obj {
			node, ok := props[k].(map[string]any)
			if !ok {
				return refuse("invalid_input", "undeclared field")
			}
			if e := validateNode(ctx, node, v, depth); e != nil {
				return e
			}
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return refuse("invalid_input", "value must be array")
		}
		depth++
		if depth > 8 || len(arr) > 1024 {
			return refuse("resource_limit", "value exceeds array bounds")
		}
		n, _ := integer(m["maxItems"])
		if int64(len(arr)) > n {
			return refuse("invalid_input", "array exceeds maxItems")
		}
		for _, v := range arr {
			if e := validateNode(ctx, m["items"].(map[string]any), v, depth); e != nil {
				return e
			}
		}
	default:
		if e := scalar(m, v); e != nil {
			return e
		}
		if ev, exists := m["enum"]; exists {
			b, _ := json.Marshal(v)
			for _, item := range ev.([]any) {
				x, _ := json.Marshal(item)
				if bytes.Equal(b, x) {
					return nil
				}
			}
			return refuse("invalid_input", "value is not in enum")
		}
	}
	return nil
}
