package artifact

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"
)

var digestPattern = newLazyPattern(`^sha256:[0-9a-f]{64}$`)
var actorPattern = newLazyPattern(`^[0-9a-f]{64}$`)
var typePattern = newLazyPattern(`^[a-z][a-z0-9.-]{0,63}/[a-z][a-z0-9.-]{0,63}$`)
var fieldPattern = newLazyPattern(`^[A-Za-z_][A-Za-z0-9_.-]{0,63}$`)
var memberPattern = newLazyPattern(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// Optional validators compile only when used, so native commands do not pay
// their construction cost. OnceValue also serializes concurrent first use.
type lazyPattern struct {
	compiled func() *regexp.Regexp
}

func newLazyPattern(pattern string) lazyPattern {
	return lazyPattern{sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(pattern)
	})}
}

func (p lazyPattern) MatchString(s string) bool { return p.compiled().MatchString(s) }

// Verified attests exact signed bytes and envelope invariants only. Type/schema,
// record lineage and accepted repository visibility require additional context.
// Private fields prevent callers from fabricating a verified predecessor.
type Verified struct {
	raw      []byte
	id       string
	actor    string
	stream   string
	sequence int64
	kind     string
}

func (v Verified) ID() string    { return v.id }
func (v Verified) Actor() string { return v.actor }
func (v Verified) Kind() string  { return v.kind }
func (v Verified) Genesis() bool { return v.sequence == 1 }
func (v Verified) Bytes() []byte { return bytes.Clone(v.raw) }

// Verify authenticates a single immutable envelope. A supplied predecessor must
// extend this actor's stream; nil validates only sequence/previous syntax.
func Verify(raw []byte, signature string, previous *Verified) (Verified, error) {
	v, canonical, e := canonicalValue(raw)
	if e != nil {
		return Verified{}, e
	}
	if !bytes.Equal(raw, canonical) {
		return Verified{}, refuse("invalid_input", "signed envelope is not canonical JSON")
	}
	m, ok := v.(map[string]any)
	if !ok || !closed(m, "protocol", "kind", "actor", "public_key", "stream", "sequence", "previous", "timestamp", "operation", "request", "record", "parents", "payload") {
		return Verified{}, refuse("invalid_input", "envelope requires exactly the protocol fields")
	}
	if text(m, "protocol") != "hn-artifact/1" {
		return Verified{}, refuse("invalid_input", "unsupported artifact protocol")
	}
	kind := text(m, "kind")
	switch kind {
	case "type.publish", "record.open", "record.revise", "record.resolve", "record.convert":
	default:
		return Verified{}, refuse("invalid_input", "unsupported artifact kind")
	}
	actor := text(m, "actor")
	if !actorPattern.MatchString(actor) {
		return Verified{}, refuse("invalid_input", "actor must be a full lower-case fingerprint")
	}
	public, e := rawBase64(text(m, "public_key"), ed25519.PublicKeySize)
	if e != nil {
		return Verified{}, e
	}
	sum := sha256.Sum256(public)
	if actor != hex.EncodeToString(sum[:]) {
		return Verified{}, refuse("verification_failure", "actor does not match public key")
	}
	stream := digest(append([]byte("hn-artifact-stream-v1\x00"), []byte(actor)...))
	if text(m, "stream") != stream {
		return Verified{}, refuse("verification_failure", "actor stream identity mismatch")
	}
	seq, ok := integer(m["sequence"])
	if !ok || seq < 1 || seq > maxInteger {
		return Verified{}, refuse("invalid_input", "sequence must be a positive safe integer")
	}
	prev, ok := m["previous"].(string)
	if !ok || (seq == 1 && prev != "") || (seq > 1 && !digestPattern.MatchString(prev)) {
		return Verified{}, refuse("invalid_input", "sequence and previous disagree")
	}
	ts := text(m, "timestamp")
	date, e := time.Parse("2006-01-02T15:04:05Z", ts)
	if e != nil || date.Year() < 1 || date.Format("2006-01-02T15:04:05Z") != ts {
		return Verified{}, refuse("invalid_input", "timestamp must be Gregorian UTC seconds")
	}
	op := text(m, "operation")
	if len(op) < 1 || len(op) > 128 {
		return Verified{}, refuse("invalid_input", "operation must contain 1..128 printable ASCII bytes")
	}
	for _, c := range []byte(op) {
		if c < 0x21 || c > 0x7e {
			return Verified{}, refuse("invalid_input", "operation must contain printable non-whitespace ASCII")
		}
	}
	record, ok := m["record"].(string)
	if !ok || ((kind == "type.publish" || kind == "record.open") && record != "") || ((kind != "type.publish" && kind != "record.open") && !digestPattern.MatchString(record)) {
		return Verified{}, refuse("invalid_input", "invalid logical record identity")
	}
	parents, e := ids(m["parents"])
	if e != nil {
		return Verified{}, e
	}
	switch kind {
	case "type.publish":
		if len(parents) > 1 {
			return Verified{}, refuse("invalid_input", "type publication has at most one parent")
		}
	case "record.open":
		if len(parents) != 0 {
			return Verified{}, refuse("invalid_input", "opening cannot have parents")
		}
	case "record.revise", "record.convert":
		if len(parents) != 1 {
			return Verified{}, refuse("invalid_input", "revision requires one parent")
		}
	case "record.resolve":
		if len(parents) == 0 {
			return Verified{}, refuse("invalid_input", "resolution requires parents")
		}
	}
	payload, ok := m["payload"].(map[string]any)
	if !ok {
		return Verified{}, refuse("invalid_input", "payload must be an object")
	}
	if e = payloadEnvelope(kind, payload, parents); e != nil {
		return Verified{}, e
	}
	semantic := map[string]any{}
	for _, k := range []string{"actor", "kind", "operation", "parents", "payload", "protocol", "record"} {
		semantic[k] = m[k]
	}
	request, _ := json.Marshal(semantic)
	if text(m, "request") != digest(append([]byte("hn-artifact-request-v1\x00"), request...)) {
		return Verified{}, refuse("verification_failure", "immutable request digest mismatch")
	}
	sig, e := rawBase64(signature, ed25519.SignatureSize)
	if e != nil {
		return Verified{}, e
	}
	if !ed25519.Verify(public, append([]byte("hn-artifact/1\x00"), raw...), sig) {
		return Verified{}, refuse("verification_failure", "invalid artifact signature")
	}
	result := Verified{bytes.Clone(raw), digest(raw), actor, stream, seq, kind}
	if previous != nil && (previous.id == "" || previous.actor != actor || previous.stream != stream || previous.sequence+1 != seq || previous.id != prev) {
		return Verified{}, refuse("invalid_history", "actor stream predecessor mismatch")
	}
	return result, nil
}
func digest(raw []byte) string { s := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(s[:]) }
func rawBase64(s string, size int) ([]byte, error) {
	if len(s) != base64.RawStdEncoding.EncodedLen(size) {
		return nil, refuse("invalid_input", "invalid canonical Base64 length")
	}
	b, e := base64.RawStdEncoding.Strict().DecodeString(s)
	if e != nil || len(b) != size || base64.RawStdEncoding.EncodeToString(b) != s {
		return nil, refuse("invalid_input", "invalid canonical Base64 encoding")
	}
	return b, nil
}
func closed(m map[string]any, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}
func text(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func integer(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, e := n.Int64()
	return i, e == nil
}
func ids(v any) ([]string, error) {
	a, ok := v.([]any)
	if !ok {
		return nil, refuse("invalid_input", "IDs must be an array")
	}
	out := make([]string, 0, len(a))
	for _, v := range a {
		s, ok := v.(string)
		if !ok || !digestPattern.MatchString(s) || (len(out) > 0 && s <= out[len(out)-1]) {
			return nil, refuse("invalid_input", "IDs must be increasing distinct full digests")
		}
		out = append(out, s)
	}
	return out, nil
}
func validType(s string) bool {
	return typePattern.MatchString(s) && !strings.HasPrefix(s, "hn/") && !strings.HasPrefix(s, "hubnot/")
}
func payloadEnvelope(kind string, p map[string]any, parents []string) error {
	if kind == "type.publish" {
		if !closed(p, "type_id", "schema_version", "shape", "schema") {
			return refuse("invalid_input", "type payload requires exactly the protocol fields")
		}
		b, _ := json.Marshal(p)
		if len(b) > 131072 {
			return refuse("resource_limit", "type payload exceeds 131072 bytes")
		}
		n, ok := integer(p["schema_version"])
		if !ok || n < 1 || n > 2147483647 {
			return refuse("invalid_input", "invalid schema version")
		}
		schema, ok := p["schema"].(map[string]any)
		if !ok {
			return refuse("invalid_input", "schema must be an object")
		}
		b, _ = json.Marshal(schema)
		if len(b) > 65536 {
			return refuse("resource_limit", "schema exceeds 65536 bytes")
		}
	} else {
		if !closed(p, "shape", "type_id", "type_revision", "title", "content", "fields", "members", "relationships", "conversion_sources") {
			return refuse("invalid_input", "record payload requires exactly the protocol fields")
		}
		if !digestPattern.MatchString(text(p, "type_revision")) {
			return refuse("invalid_input", "invalid exact type revision")
		}
		title := text(p, "title")
		if len(title) < 1 || len(title) > 1024 || strings.TrimSpace(title) == "" {
			return refuse("invalid_input", "title must contain 1..1024 UTF-8 bytes and visible text")
		}
		if _, ok := p["content"].(string); !ok {
			return refuse("invalid_input", "content must be text")
		}
		fields, ok := p["fields"].(map[string]any)
		if !ok {
			return refuse("invalid_input", "fields must be an object")
		}
		if e := fieldBounds(fields, 1); e != nil {
			return e
		}
		members, ok := p["members"].([]any)
		if !ok {
			return refuse("invalid_input", "members must be an array")
		}
		if text(p, "shape") != "list" && len(members) != 0 {
			return refuse("invalid_input", "only lists have members")
		}
		seen := map[string]bool{}
		for _, v := range members {
			m, ok := v.(map[string]any)
			if !ok || !closed(m, "member_id", "target", "target_mode", "required") || !memberPattern.MatchString(text(m, "member_id")) || seen[text(m, "member_id")] {
				return refuse("invalid_input", "invalid or duplicate collection member")
			}
			seen[text(m, "member_id")] = true
			if e := link(m); e != nil {
				return e
			}
		}
		rels, ok := p["relationships"].([]any)
		if !ok {
			return refuse("invalid_input", "relationships must be an array")
		}
		if len(rels) > 256 {
			return refuse("resource_limit", "relationships exceeds 256 entries")
		}
		seen = map[string]bool{}
		for _, v := range rels {
			r, ok := v.(map[string]any)
			if !ok || !closed(r, "relation", "target", "target_mode", "required") || !validType(text(r, "relation")) {
				return refuse("invalid_input", "invalid relationship")
			}
			if e := link(r); e != nil {
				return e
			}
			b, _ := json.Marshal(r)
			if seen[string(b)] {
				return refuse("invalid_input", "duplicate relationship")
			}
			seen[string(b)] = true
		}
		sources, e := ids(p["conversion_sources"])
		if e != nil {
			return e
		}
		switch kind {
		case "record.open", "record.revise":
			if len(sources) != 0 {
				return refuse("invalid_input", "unexpected conversion sources")
			}
		case "record.convert":
			if len(sources) != 1 || sources[0] != parents[0] {
				return refuse("invalid_input", "conversion source must equal parent")
			}
		case "record.resolve":
			for _, s := range sources {
				found := false
				for _, p := range parents {
					found = found || p == s
				}
				if !found {
					return refuse("invalid_input", "conversion source must be a parent")
				}
			}
		}
	}
	if !validType(text(p, "type_id")) {
		return refuse("invalid_input", "invalid or reserved type identifier")
	}
	switch text(p, "shape") {
	case "document", "list", "ticket":
	default:
		return refuse("invalid_input", "invalid artifact shape")
	}
	return nil
}
func link(m map[string]any) error {
	if !digestPattern.MatchString(text(m, "target")) || (text(m, "target_mode") != "logical" && text(m, "target_mode") != "revision") {
		return refuse("invalid_input", "invalid typed target")
	}
	if _, ok := m["required"].(bool); !ok {
		return refuse("invalid_input", "required must be boolean")
	}
	return nil
}
func fieldBounds(v any, depth int) error {
	switch v := v.(type) {
	case map[string]any:
		if depth > 8 {
			return refuse("resource_limit", "fields exceed 8 container levels")
		}
		for k, x := range v {
			if !fieldPattern.MatchString(k) {
				return refuse("invalid_input", "invalid field name")
			}
			if e := fieldBounds(x, depth+1); e != nil {
				return e
			}
		}
	case []any:
		if depth > 8 {
			return refuse("resource_limit", "fields exceed 8 container levels")
		}
		for _, x := range v {
			if e := fieldBounds(x, depth+1); e != nil {
				return e
			}
		}
	}
	return nil
}
