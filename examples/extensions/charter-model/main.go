// charter-model is an optional, standalone example consumer of hn.extension/1.
// It deliberately imports only the Go standard library.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const protocol = "hn.extension/1"
const moduleID = "example.charter/model"
const frameBytes = 8388608
const inputBytes = 1048576

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var sessionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var sessionActor = regexp.MustCompile(`^[0-9a-f]{64}$`)
var customName = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,63}/[a-z][a-z0-9.-]{0,63}$`)
var fieldName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,63}$`)
var memberName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var methods = []string{"identity.public", "artifact.type.publish", "artifact.type.read", "artifact.mutate", "artifact.read", "artifact.search", "artifact.operation"}

// decodeObject retains integer values and rejects lossy JSON input. Host data is
// untrusted even on a pipe. Duplicate keys and lone UTF-16 surrogates must not be
// silently normalized by encoding/json.
func decodeObject(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("invalid UTF-8")
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		i++
		for i < len(raw) && raw[i] != '"' {
			if raw[i] != '\\' {
				i++
				continue
			}
			i++
			if i >= len(raw) {
				return nil, io.ErrUnexpectedEOF
			}
			if raw[i] == 'u' {
				if i+4 >= len(raw) {
					return nil, io.ErrUnexpectedEOF
				}
				n, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
				if e != nil {
					return nil, e
				}
				i += 4
				if n >= 0xd800 && n <= 0xdbff {
					if i+6 >= len(raw) || string(raw[i+1:i+3]) != "\\u" {
						return nil, errors.New("unpaired surrogate")
					}
					low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
					if e != nil || low < 0xdc00 || low > 0xdfff {
						return nil, errors.New("unpaired surrogate")
					}
					i += 6
				} else if n >= 0xdc00 && n <= 0xdfff {
					return nil, errors.New("unpaired surrogate")
				}
			}
			i++
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) (any, error)
	value = func(depth int) (any, error) {
		if depth > 64 {
			return nil, errors.New("JSON nesting exceeds 64")
		}
		tok, e := d.Token()
		if e != nil {
			return nil, e
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				m := map[string]any{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						return nil, e
					}
					k, ok := key.(string)
					if !ok {
						return nil, errors.New("object key is not a string")
					}
					if _, ok = m[k]; ok {
						return nil, errors.New("duplicate object key")
					}
					m[k], e = value(depth + 1)
					if e != nil {
						return nil, e
					}
				}
				if end, e := d.Token(); e != nil || end != json.Delim('}') {
					return nil, errors.New("unterminated object")
				}
				return m, nil
			case '[':
				a := []any{}
				for d.More() {
					x, e := value(depth + 1)
					if e != nil {
						return nil, e
					}
					a = append(a, x)
				}
				if end, e := d.Token(); e != nil || end != json.Delim(']') {
					return nil, errors.New("unterminated array")
				}
				return a, nil
			}
			return nil, errors.New("unexpected JSON delimiter")
		case json.Number:
			n, e := strconv.ParseInt(string(v), 10, 64)
			if e != nil || n < -9007199254740991 || n > 9007199254740991 || strconv.FormatInt(n, 10) != string(v) {
				return nil, errors.New("not a canonical safe integer")
			}
			return v, nil
		default:
			return tok, nil
		}
	}
	v, e := value(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("expected JSON object")
	}
	return m, nil
}

func readFrame(r io.Reader) ([]byte, error) {
	var prefix [4]byte
	if _, e := io.ReadFull(r, prefix[:]); e != nil {
		return nil, e
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 || n > frameBytes {
		return nil, errors.New("invalid frame length")
	}
	raw := make([]byte, n)
	if _, e := io.ReadFull(r, raw); e != nil {
		return nil, e
	}
	if _, e := decodeObject(raw); e != nil {
		return nil, e
	}
	return raw, nil
}
func writeFrame(w io.Writer, v any) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if len(raw) == 0 || len(raw) > frameBytes {
		return errors.New("outgoing frame exceeds bound")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
	b := append(prefix[:], raw...)
	for len(b) > 0 {
		n, e := w.Write(b)
		if e != nil {
			return e
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func strict(raw []byte, v any) error {
	if _, e := decodeObject(raw); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(v)
}
func acceptHello(raw []byte) error {
	var h struct {
		Protocol string `json:"protocol"`
		Hello    struct {
			Session      string `json:"session_id"`
			Repository   string `json:"repository_id"`
			Module       string `json:"module_id"`
			Version      string `json:"module_version"`
			Registration string `json:"registration_id"`
		} `json:"hello"`
		Limits struct {
			Frame       int  `json:"frame_bytes"`
			Read        int  `json:"read_timeout_ms"`
			Mutation    int  `json:"mutation_timeout_ms"`
			Page        int  `json:"page_max"`
			Outstanding bool `json:"one_outstanding"`
		} `json:"limits"`
	}
	if e := strict(raw, &h); e != nil {
		return e
	}
	if h.Protocol != protocol || h.Hello.Module != moduleID || h.Hello.Version != "1" || !sessionPattern.MatchString(h.Hello.Session) || !digestPattern.MatchString(h.Hello.Repository) || !digestPattern.MatchString(h.Hello.Registration) || h.Limits.Frame != frameBytes || h.Limits.Read != 30000 || h.Limits.Mutation != 240000 || h.Limits.Page != 200 || !h.Limits.Outstanding {
		return errors.New("host binding or limits mismatch")
	}
	return nil
}
func readInput(r io.Reader) (map[string]any, error) {
	raw, e := io.ReadAll(io.LimitReader(r, inputBytes+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > inputBytes {
		return nil, errors.New("input exceeds 1048576 bytes")
	}
	return decodeObject(raw)
}

func checkPinnedBinding(raw []byte, command map[string]any) error {
	h, e := decodeObject(raw)
	if e != nil {
		return e
	}
	binding, ok := h["hello"].(map[string]any)
	if !ok {
		return errors.New("missing host binding")
	}
	for _, pair := range [][2]string{{"expected_repository_id", "repository_id"}, {"expected_registration_id", "registration_id"}} {
		if expected, ok := command[pair[0]]; ok && expected != binding[pair[1]] {
			return errors.New("operator-pinned host binding mismatch")
		}
	}
	return nil
}

type client struct {
	in       io.Reader
	out      io.Writer
	sequence int
}

// These checks cover only the example's current public methods. They validate
// wire values, not signatures, schema semantics, accepted history or authority.
func responseObject(v any, required string) map[string]any {
	m, ok := v.(map[string]any)
	keys := strings.Fields(required)
	if !ok || len(m) != len(keys) {
		return nil
	}
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			return nil
		}
	}
	return m
}
func responseString(v any, min, max int) bool {
	s, ok := v.(string)
	return ok && len(s) >= min && len(s) <= max
}
func responseBool(v any) bool { _, ok := v.(bool); return ok }
func responseChoice(v any, choices string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, choice := range strings.Fields(choices) {
		if s == choice {
			return true
		}
	}
	return false
}
func responseID(v any) bool    { s, ok := v.(string); return ok && digestPattern.MatchString(s) }
func responseActor(v any) bool { s, ok := v.(string); return ok && sessionActor.MatchString(s) }
func responseType(v any) bool {
	s, ok := v.(string)
	return ok && customName.MatchString(s) && !strings.HasPrefix(s, "hn/") && !strings.HasPrefix(s, "hubnot/")
}
func responseNumber(v any, min, max int64) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	i, e := n.Int64()
	return e == nil && i >= min && i <= max
}
func responseKey(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) < 1 || len(s) > 128 {
		return false
	}
	for _, b := range []byte(s) {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}
func responseBase64(v any, size int) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	raw, e := base64.RawStdEncoding.Strict().DecodeString(s)
	return e == nil && (size < 0 || len(raw) == size) && base64.RawStdEncoding.EncodeToString(raw) == s
}
func responseArray(v any, max int, check func(any) bool) bool {
	a, ok := v.([]any)
	if !ok || len(a) > max {
		return false
	}
	for _, item := range a {
		if !check(item) {
			return false
		}
	}
	return true
}
func responseIDs(v any) bool {
	last := ""
	return responseArray(v, 1024, func(x any) bool {
		if !responseID(x) || x.(string) <= last {
			return false
		}
		last = x.(string)
		return true
	})
}
func responseBound(v any, max int) bool {
	raw, e := json.Marshal(v)
	return e == nil && len(raw) <= max
}
func responseNullable(v any, check func(any) bool) bool { return v == nil || check(v) }
func responseDiagnostics(v any) bool {
	return responseArray(v, frameBytes, func(x any) bool {
		m := responseObject(x, "code id message")
		return m != nil && responseChoice(m["code"], "incomplete_dependency unsupported_schema target_conflict missing_optional invalid_history opaque_unverified") && responseNullable(m["id"], responseID) && responseString(m["message"], 0, 4096)
	})
}
func responseFields(v any, depth int) bool {
	switch x := v.(type) {
	case map[string]any:
		if depth > 8 || len(x) > 64 {
			return false
		}
		for k, value := range x {
			if !fieldName.MatchString(k) || !responseFields(value, depth+1) {
				return false
			}
		}
		return true
	case []any:
		if depth > 8 {
			return false
		}
		return responseArray(x, 1024, func(item any) bool { return responseFields(item, depth+1) })
	default:
		return true // Scalars already satisfy the strict bounded JSON parser.
	}
}
func responseSchemaNode(v any, depth int) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	required, optional := "type", ""
	switch m["type"] {
	case "object":
		required += " properties required additionalProperties"
		if depth > 8 || m["additionalProperties"] != false {
			return false
		}
		p, ok := m["properties"].(map[string]any)
		if !ok || len(p) > 64 {
			return false
		}
		for key, node := range p {
			if !fieldName.MatchString(key) || !responseSchemaNode(node, depth+1) {
				return false
			}
		}
		seen := map[string]bool{}
		if !responseArray(m["required"], 1024, func(x any) bool {
			s, ok := x.(string)
			if !ok || !fieldName.MatchString(s) || seen[s] {
				return false
			}
			seen[s] = true
			return true
		}) {
			return false
		}
	case "array":
		required += " items maxItems"
		if depth > 8 || !responseNumber(m["maxItems"], 0, 1024) || !responseSchemaNode(m["items"], depth+1) {
			return false
		}
	case "string":
		required += " maxBytes"
		optional = "enum"
		if !responseNumber(m["maxBytes"], 0, 1048576) {
			return false
		}
	case "integer":
		optional = "minimum maximum enum"
		for _, k := range []string{"minimum", "maximum"} {
			if n, ok := m[k]; ok && !responseNumber(n, -9007199254740991, 9007199254740991) {
				return false
			}
		}
	case "boolean":
		optional = "enum"
	case "null":
	default:
		return false
	}
	allowed := map[string]bool{}
	for _, k := range strings.Fields(required) {
		allowed[k] = true
		if _, ok := m[k]; !ok {
			return false
		}
	}
	for _, k := range strings.Fields(optional) {
		allowed[k] = true
	}
	for k := range m {
		if !allowed[k] {
			return false
		}
	}
	if values, ok := m["enum"]; ok {
		a, ok := values.([]any)
		if !ok || len(a) < 1 || len(a) > 1024 {
			return false
		}
		seen := map[string]bool{}
		for _, value := range a {
			valid := false
			switch m["type"] {
			case "string":
				valid = responseString(value, 0, 1048576)
			case "integer":
				valid = responseNumber(value, -9007199254740991, 9007199254740991)
			case "boolean":
				valid = responseBool(value)
			}
			raw, _ := json.Marshal(value)
			key := string(raw)
			if !valid || seen[key] {
				return false
			}
			seen[key] = true
		}
	}
	return true
}
func responseTypePayload(v any) bool {
	m := responseObject(v, "type_id schema_version shape schema")
	if m == nil || !responseType(m["type_id"]) || !responseNumber(m["schema_version"], 1, 2147483647) || !responseChoice(m["shape"], "document list ticket") || !responseBound(v, 131072) {
		return false
	}
	s := responseObject(m["schema"], "language root")
	if s == nil || !responseString(s["language"], 1, 64) || !responseBound(s, 65536) {
		return false
	}
	root, ok := s["root"].(map[string]any)
	return ok && root["type"] == "object" && responseSchemaNode(root, 1)
}
func responseRecordPayload(v any) bool {
	m := responseObject(v, "shape type_id type_revision title content fields members relationships conversion_sources")
	if m == nil || !responseChoice(m["shape"], "document list ticket") || !responseType(m["type_id"]) || !responseID(m["type_revision"]) || !responseString(m["title"], 1, 1024) || strings.TrimSpace(m["title"].(string)) == "" || !responseString(m["content"], 0, frameBytes) || !responseIDs(m["conversion_sources"]) {
		return false
	}
	fields, ok := m["fields"].(map[string]any)
	if !ok || !responseFields(fields, 1) {
		return false
	}
	seen := map[string]bool{}
	if !responseArray(m["members"], 1024, func(v any) bool {
		x := responseObject(v, "member_id target target_mode required")
		if x == nil {
			return false
		}
		id, ok := x["member_id"].(string)
		if !ok || !memberName.MatchString(id) || seen[id] || !responseID(x["target"]) || !responseChoice(x["target_mode"], "revision logical") || !responseBool(x["required"]) {
			return false
		}
		seen[id] = true
		return true
	}) {
		return false
	}
	if m["shape"] != "list" && len(m["members"].([]any)) != 0 {
		return false
	}
	seen = map[string]bool{}
	return responseArray(m["relationships"], 256, func(v any) bool {
		x := responseObject(v, "relation target target_mode required")
		if x == nil || !responseType(x["relation"]) || !responseID(x["target"]) || !responseChoice(x["target_mode"], "revision logical") || !responseBool(x["required"]) {
			return false
		}
		raw, _ := json.Marshal(x)
		if seen[string(raw)] {
			return false
		}
		seen[string(raw)] = true
		return true
	})
}
func responseSigned(v any, opaque bool) bool {
	if m, ok := v.(map[string]any); ok && m["validation"] == "opaque" {
		x := responseObject(v, "id opaque_id envelope signature validation raw diagnostics")
		if !opaque || x == nil || x["id"] != nil || x["envelope"] != nil || x["signature"] != nil || !responseID(x["opaque_id"]) || !responseDiagnostics(x["diagnostics"]) {
			return false
		}
		r := responseObject(x["raw"], "oid type data")
		return r != nil && responseString(r["oid"], 1, 128) && responseString(r["type"], 1, 128) && responseBase64(r["data"], -1)
	}
	m := responseObject(v, "id envelope signature validation diagnostics")
	if m == nil || !responseID(m["id"]) || !responseBase64(m["signature"], 64) || !responseChoice(m["validation"], "verified unsupported incomplete") || !responseDiagnostics(m["diagnostics"]) {
		return false
	}
	e := responseObject(m["envelope"], "protocol kind actor public_key stream sequence previous timestamp operation request record parents payload")
	if e == nil || e["protocol"] != "hn-artifact/1" || !responseChoice(e["kind"], "type.publish record.open record.revise record.resolve record.convert") || !responseActor(e["actor"]) || !responseBase64(e["public_key"], 32) || !responseID(e["stream"]) || !responseNumber(e["sequence"], 1, 9007199254740991) || !(e["previous"] == "" || responseID(e["previous"])) || !(e["record"] == "" || responseID(e["record"])) || !responseKey(e["operation"]) || !responseID(e["request"]) || !responseIDs(e["parents"]) || !responseBound(e, 1048576) {
		return false
	}
	timestamp, ok := e["timestamp"].(string)
	if !ok || len(timestamp) != 20 {
		return false
	}
	at, err := time.Parse("2006-01-02T15:04:05Z", timestamp)
	if err != nil || at.Year() < 1 || at.Format("2006-01-02T15:04:05Z") != timestamp {
		return false
	}
	if e["kind"] == "type.publish" {
		return responseTypePayload(e["payload"])
	}
	return responseRecordPayload(e["payload"])
}
func responseView(v any) bool {
	m := responseObject(v, "protocol id type_id shape heads conflict state diagnostics")
	if m == nil || m["protocol"] != "hn-artifact/1" || !responseID(m["id"]) || !responseType(m["type_id"]) || !responseChoice(m["shape"], "document list ticket") || !responseBool(m["conflict"]) || !responseNullable(m["state"], responseRecordPayload) || !responseDiagnostics(m["diagnostics"]) {
		return false
	}
	if m["conflict"] == true && m["state"] != nil {
		return false
	}
	return responseArray(m["heads"], frameBytes, func(v any) bool {
		h := responseObject(v, "id actor type_revision payload")
		return h != nil && responseID(h["id"]) && responseActor(h["actor"]) && responseID(h["type_revision"]) && responseRecordPayload(h["payload"])
	})
}
func responseMutation(v any) bool {
	m := responseObject(v, "protocol family type_id resource_id event_id replayed conflict head_count admission source_cut")
	return m != nil && m["protocol"] == "hn-artifact/1" && m["family"] == nil && m["admission"] == nil && responseType(m["type_id"]) && responseID(m["resource_id"]) && responseID(m["event_id"]) && responseBool(m["replayed"]) && responseBool(m["conflict"]) && responseNumber(m["head_count"], 0, 9007199254740991) && responseID(m["source_cut"])
}
func responseOperation(v any) bool {
	m := responseObject(v, "status protocol actor operation family type_id request resource_id event_id result source_cut reason")
	if m == nil || m["protocol"] != "hn-artifact/1" || m["family"] != nil || !responseType(m["type_id"]) || !responseActor(m["actor"]) || !responseKey(m["operation"]) || !responseID(m["source_cut"]) {
		return false
	}
	for _, key := range []string{"request", "resource_id", "event_id"} {
		if !responseNullable(m[key], responseID) {
			return false
		}
	}
	switch m["status"] {
	case "found":
		return responseID(m["request"]) && responseID(m["resource_id"]) && responseID(m["event_id"]) && responseMutation(m["result"]) && m["reason"] == nil
	case "verified_absent":
		return m["request"] == nil && m["resource_id"] == nil && m["event_id"] == nil && m["result"] == nil && m["reason"] == nil
	case "uncertain":
		return m["result"] == nil && responseChoice(m["reason"], "incomplete_selection unavailable_history private_intent_pending visibility_restricted publication_unconfirmed")
	}
	return false
}

var responseErrorCodes = map[string]string{
	"host":     "denied_access incompatible_version unavailable_module unsupported_capability resource_limit cancelled uncertain_operation checkpoint_invalidated verification_failure invalid_input not_found repository_error",
	"native":   "actor_mismatch admission_pending admission_refused ambiguous_id concurrent_update invalid_cursor invalid_history invalid_input issue_conflict not_found operation_conflict permission_denied repository_error resource_conflict resource_limit stale_cursor stale_revision",
	"artifact": "invalid_input resource_limit verification_failure invalid_history incomplete_dependency unsupported_schema actor_stream_conflict operation_conflict stale_revision resource_conflict concurrent_update not_found repository_error cancelled uncertain_operation",
}

func responseFailure(v any) bool {
	m := responseObject(v, "origin code message recovery operation")
	if m == nil {
		return false
	}
	origin, ok := m["origin"].(string)
	if !ok || !responseChoice(m["code"], responseErrorCodes[origin]) || !responseString(m["message"], 0, 4096) || !responseChoice(m["recovery"], "do_not_retry retry_identical inspect_operation rescan_required") {
		return false
	}
	return responseNullable(m["operation"], func(v any) bool {
		o := responseObject(v, "protocol actor operation")
		return o != nil && responseChoice(o["protocol"], "hn/0 hn-artifact/1") && responseActor(o["actor"]) && responseKey(o["operation"])
	})
}
func validateResponse(v map[string]any, method string, params map[string]any, id string) error {
	m := responseObject(v, "protocol request_id ok data error")
	if m == nil || m["protocol"] != protocol || m["request_id"] != id || !responseBool(m["ok"]) {
		return errors.New("response envelope/correlation mismatch")
	}
	valid := false
	if m["ok"] == false {
		valid = m["data"] == nil && responseFailure(m["error"])
	} else if m["error"] == nil {
		switch method {
		case "identity.public":
			d := responseObject(m["data"], "initialized actor name public_key")
			valid = d != nil && responseBool(d["initialized"]) && responseNullable(d["actor"], responseActor) && responseNullable(d["name"], func(v any) bool { return responseString(v, 0, frameBytes) }) && responseNullable(d["public_key"], func(v any) bool { return responseBase64(v, 32) })
		case "artifact.mutate", "artifact.type.publish":
			valid = responseMutation(m["data"])
		case "artifact.operation":
			valid = responseOperation(m["data"])
		case "artifact.read", "artifact.type.read", "artifact.search":
			p := responseObject(m["data"], "items source_cut next_cursor")
			if p != nil && responseID(p["source_cut"]) && responseNullable(p["next_cursor"], func(v any) bool { return responseString(v, 1, 4096) }) {
				valid = responseArray(p["items"], 200, func(v any) bool {
					if method == "artifact.search" || method == "artifact.read" && responseChoice(params["mode"], "view heads") {
						return responseView(v)
					}
					return responseSigned(v, method == "artifact.read" && params["opaque"] == true)
				})
			}
		}
	}
	if !valid {
		return errors.New("invalid typed method response")
	}
	return nil
}

func (c *client) call(method string, params map[string]any) (map[string]any, error) {
	allowed := false
	for _, m := range methods {
		allowed = allowed || m == method
	}
	if !allowed {
		return nil, errors.New("method is outside example capabilities")
	}
	c.sequence++
	id := fmt.Sprintf("charter-%d", c.sequence)
	if e := writeFrame(c.out, map[string]any{"protocol": protocol, "request_id": id, "method": method, "params": params}); e != nil {
		return nil, e
	}
	raw, e := readFrame(c.in)
	if e != nil {
		return nil, e
	}
	m, e := decodeObject(raw)
	if e != nil {
		return nil, e
	}
	if e = validateResponse(m, method, params, id); e != nil {
		return nil, e
	}
	return m, nil
}
func (c *client) require(method string, params map[string]any) (map[string]any, error) {
	response, e := c.call(method, params)
	if e != nil {
		return nil, e
	}
	if response["ok"] != true {
		failure := response["error"].(map[string]any)
		return nil, fmt.Errorf("%s/%s: inspect the explicit application result", failure["origin"], failure["code"])
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		return nil, errors.New("expected object result")
	}
	return data, nil
}
func save(path string, v any) error {
	raw, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	file, e := os.CreateTemp(filepath.Dir(path), ".charter-*")
	if e != nil {
		return e
	}
	name := file.Name()
	defer os.Remove(name)
	if _, e = file.Write(raw); e == nil {
		e = file.Sync()
	}
	closeErr := file.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}

func main() {
	manifest := flag.Bool("manifest", false, "print inert manifest and exit")
	input := flag.String("input", "", "bounded application command JSON file")
	output := flag.String("output", "", "private application result JSON file")
	state := flag.String("state", "", "example selection state file")
	flag.Parse()
	if *manifest {
		raw, _ := json.Marshal(map[string]any{"protocol": "hn.extension-manifest/1", "module_id": moduleID, "module_version": "1", "host_protocol": protocol, "capabilities": methods})
		fmt.Println(string(raw))
		return
	}
	if flag.NArg() != 0 || *input == "" || *output == "" || *state == "" {
		fmt.Fprintln(os.Stderr, "supply --input, --output and --state, or --manifest")
		os.Exit(2)
	}
	if e := execute(*input, *output, *state); e != nil {
		fmt.Fprintln(os.Stderr, "charter-model:", e)
		os.Exit(1)
	}
}
func execute(input, output, state string) error {
	f, e := os.Open(input)
	if e != nil {
		return e
	}
	command, e := readInput(f)
	_ = f.Close()
	if e != nil {
		return e
	}
	allowed := map[string]bool{"action": true, "expected_repository_id": true, "expected_registration_id": true}
	switch command["action"] {
	case "publish":
		allowed["directive_text"] = true
	case "evolve":
		allowed["parent"] = true
	case "call":
		allowed["method"] = true
		allowed["params"] = true
	case "inspect-selection":
	default:
		return errors.New("application action is unavailable")
	}
	for k, v := range command {
		if !allowed[k] {
			return errors.New("unknown application command field")
		}
		if k != "params" {
			if _, ok := v.(string); !ok {
				return errors.New("application command field must be a string")
			}
		}
	}
	hello, e := readFrame(os.Stdin)
	if e != nil {
		return e
	}
	if e = acceptHello(hello); e != nil {
		return e
	}
	if e = checkPinnedBinding(hello, command); e != nil {
		return e
	}
	if e = writeFrame(os.Stdout, map[string]any{"protocol": protocol, "ready": true}); e != nil {
		return e
	}
	c := &client{in: os.Stdin, out: os.Stdout}
	result := map[string]any{}
	switch command["action"] {
	case "call":
		method, _ := command["method"].(string)
		params, ok := command["params"].(map[string]any)
		if !ok {
			return errors.New("call requires params object")
		}
		response, e := c.call(method, params)
		if e != nil {
			return e
		}
		result["response"] = response
	case "publish":
		text, _ := command["directive_text"].(string)
		result, e = publish(c, state, text)
	case "evolve":
		parent, _ := command["parent"].(string)
		result, e = evolve(c, state, parent)
	case "inspect-selection":
		result, e = inspectSelection(c, state)
	default:
		return errors.New("application action is unavailable")
	}
	if e != nil {
		return e
	}
	return save(output, result)
}
