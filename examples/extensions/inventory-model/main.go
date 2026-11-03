// inventory-model is an explicitly registered example consumer of hn.extension/1.
// All inventory interpretation lives in this executable, outside Hubnot.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const protocol = "hn.extension/1"
const frameBytes = 8388608 // Frozen host transport limit, not a new example limit.
const moduleID = "example.inventory/publisher"

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var actorPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var sessionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var terminalResponse = errors.New("terminal host response; session stopped")

// Command is the example's local input format, never a Hubnot protocol type.
// Full state and exact parents are supplied by the operator; no latest-head retry
// or automatic conversion/resolution is performed.
type Command struct {
	Action            string           `json:"action"`
	Model             string           `json:"model"`
	Operation         string           `json:"operation"`
	SchemaVersion     int              `json:"schema_version"`
	Record            *string          `json:"record"`
	Parents           []string         `json:"parents"`
	TypeRevision      string           `json:"type_revision"`
	Title             string           `json:"title"`
	Content           string           `json:"content"`
	Fields            map[string]any   `json:"fields"`
	Members           []map[string]any `json:"members"`
	Relationships     []map[string]any `json:"relationships"`
	ConversionSources []string         `json:"conversion_sources"`
}
type responseFrame struct {
	Protocol  string          `json:"protocol"`
	RequestID *string         `json:"request_id"`
	OK        bool            `json:"ok"`
	Data      json.RawMessage `json:"data"`
	Error     json.RawMessage `json:"error"`
}

func main() {
	input := flag.String("input", "", "explicit command JSON file")
	output := flag.String("output", "", "private result JSON file")
	flag.Parse()
	if *input == "" || *output == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "inventory-model requires --input FILE --output FILE")
		os.Exit(1)
	}
	file, err := os.Open(*input)
	if err != nil {
		fail(err)
	}
	raw, err := io.ReadAll(io.LimitReader(file, frameBytes+1))
	closeErr := file.Close()
	if err != nil {
		fail(err)
	}
	if closeErr != nil {
		fail(closeErr)
	}
	if len(raw) > frameBytes {
		fail(errors.New("command exceeds host frame limit"))
	}
	var command Command
	if err = decodeObject(raw, &command); err != nil {
		fail(err)
	}
	var result bytes.Buffer
	if err = run(os.Stdin, os.Stdout, &result, command); err != nil {
		fail(err)
	}
	// The explicitly chosen result is private application output, not host stdout
	// or a public export. Refuse nonregular existing result paths.
	if info, err := os.Lstat(*output); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		fail(errors.New("result must be a regular file"))
	} else if err != nil && !os.IsNotExist(err) {
		fail(err)
	}
	f, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fail(err)
	}
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(result.Bytes())
	}
	closeErr = f.Close()
	if err != nil {
		fail(err)
	}
	if closeErr != nil {
		fail(closeErr)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, "inventory-model:", err); os.Exit(1) }
func run(in io.Reader, out, result io.Writer, c Command) error {
	var hello map[string]any
	if err := readFrame(in, &hello); err != nil {
		return err
	}
	if !validHello(hello) {
		return errors.New("incompatible host hello")
	}
	if err := writeFrame(out, map[string]any{"protocol": protocol, "ready": true}); err != nil {
		return err
	}
	identity, err := exchange(in, out, "identity", "identity.public", map[string]any{})
	if errors.Is(err, terminalResponse) {
		return json.NewEncoder(result).Encode(identity)
	}
	if err != nil {
		return err
	}
	if !identity.OK {
		return json.NewEncoder(result).Encode(identity)
	}
	var actor map[string]any
	if err = decodeObject(identity.Data, &actor); err != nil {
		return err
	}
	if actor["initialized"] != true {
		return errors.New("repository identity must be explicitly initialized")
	}
	method, params, err := c.request(actor["actor"].(string))
	if err != nil {
		return err
	}
	response, err := exchange(in, out, "inventory", method, params)
	if errors.Is(err, terminalResponse) {
		return json.NewEncoder(result).Encode(response)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(result).Encode(response)
}
func exchange(in io.Reader, out io.Writer, id, method string, params map[string]any) (responseFrame, error) {
	var response map[string]any
	if err := writeFrame(out, map[string]any{"protocol": protocol, "request_id": id, "method": method, "params": params}); err != nil {
		return responseFrame{}, err
	}
	if err := readFrame(in, &response); err != nil {
		return responseFrame{}, err
	}
	return checkedResponse(response, id, method, params)
}

// These checks cover only the response types this publisher actually consumes.
// They validate transport values, never signatures, schema/application meaning,
// authority or durable receipt. Invalid or terminal responses stop the session.
func checkedResponse(v map[string]any, id, method string, params map[string]any) (responseFrame, error) {
	var zero responseFrame
	if !exact(v, "protocol", "request_id", "ok", "data", "error") || v["protocol"] != protocol {
		return zero, errors.New("invalid response envelope")
	}
	ok, typed := v["ok"].(bool)
	if !typed {
		return zero, errors.New("response ok must be a boolean")
	}
	requestID, isID := v["request_id"].(string)
	if ok {
		if !isID || v["error"] != nil || !validSuccess(method, params, v["data"]) {
			return zero, errors.New("invalid typed success response")
		}
	} else if v["data"] != nil || !validError(v["error"]) {
		return zero, errors.New("invalid typed error response")
	}
	data, _ := json.Marshal(v["data"])
	failure, _ := json.Marshal(v["error"])
	response := responseFrame{Protocol: protocol, OK: ok, Data: data, Error: failure}
	if v["request_id"] == nil {
		return response, terminalResponse
	}
	if !isID || requestID != id {
		return zero, errors.New("uncorrelated host response")
	}
	response.RequestID = &requestID
	return response, nil
}
func exact(v map[string]any, keys ...string) bool {
	if v == nil || len(v) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := v[key]; !ok {
			return false
		}
	}
	return true
}
func boundedInt(v any, low, high int64) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	return err == nil && i >= low && i <= high
}
func printable(v any, maximum int) bool {
	s, ok := v.(string)
	if !ok || len(s) < 1 || len(s) > maximum {
		return false
	}
	for _, b := range []byte(s) {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}
func oneOf(v any, values ...string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, value := range values {
		if s == value {
			return true
		}
	}
	return false
}
func digest(v any) bool { s, ok := v.(string); return ok && digestPattern.MatchString(s) }
func canonicalBase64(v any, bytes int) ([]byte, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	raw, err := base64.RawStdEncoding.DecodeString(s)
	return raw, err == nil && len(raw) == bytes && base64.RawStdEncoding.EncodeToString(raw) == s
}
func validHello(v map[string]any) bool {
	if !exact(v, "protocol", "hello", "limits") || v["protocol"] != protocol {
		return false
	}
	h, ok := v["hello"].(map[string]any)
	if !ok || !exact(h, "session_id", "repository_id", "module_id", "module_version", "registration_id") {
		return false
	}
	session, ok := h["session_id"].(string)
	if !ok || !sessionPattern.MatchString(session) || !digest(h["repository_id"]) || !digest(h["registration_id"]) || h["module_id"] != moduleID || h["module_version"] != "1" {
		return false
	}
	l, ok := v["limits"].(map[string]any)
	return ok && exact(l, "frame_bytes", "read_timeout_ms", "mutation_timeout_ms", "page_max", "one_outstanding") && boundedInt(l["frame_bytes"], frameBytes, frameBytes) && boundedInt(l["read_timeout_ms"], 30000, 30000) && boundedInt(l["mutation_timeout_ms"], 240000, 240000) && boundedInt(l["page_max"], 200, 200) && l["one_outstanding"] == true
}
func validSuccess(method string, params map[string]any, value any) bool {
	v, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if method == "identity.public" {
		if !exact(v, "initialized", "actor", "name", "public_key") {
			return false
		}
		initialized, ok := v["initialized"].(bool)
		if !ok {
			return false
		}
		if !initialized {
			return v["actor"] == nil && v["name"] == nil && v["public_key"] == nil
		}
		actor, ok := v["actor"].(string)
		if !ok || !actorPattern.MatchString(actor) {
			return false
		}
		if _, ok = v["name"].(string); !ok {
			return false
		}
		public, ok := canonicalBase64(v["public_key"], 32)
		if !ok {
			return false
		}
		sum := sha256.Sum256(public)
		return actor == hex.EncodeToString(sum[:])
	}
	if method != "artifact.type.publish" && method != "artifact.mutate" {
		return false
	}
	if !exact(v, "protocol", "family", "type_id", "resource_id", "event_id", "replayed", "conflict", "head_count", "admission", "source_cut") || v["protocol"] != "hn-artifact/1" || v["family"] != nil || v["admission"] != nil || !digest(v["resource_id"]) || !digest(v["event_id"]) || !digest(v["source_cut"]) || !boundedInt(v["head_count"], 1, 9007199254740991) {
		return false
	}
	payload, ok := params["payload"].(map[string]any)
	if !ok || v["type_id"] != payload["type_id"] {
		return false
	}
	if _, ok = v["replayed"].(bool); !ok {
		return false
	}
	conflict, ok := v["conflict"].(bool)
	if !ok {
		return false
	}
	count, _ := strconv.ParseInt(v["head_count"].(json.Number).String(), 10, 64)
	if conflict != (count > 1) {
		return false
	}
	if method == "artifact.type.publish" {
		return !conflict && count == 1 && v["resource_id"] == v["event_id"]
	}
	if params["kind"] == "record.open" {
		return v["resource_id"] == v["event_id"]
	}
	record, ok := params["record"].(*string)
	return ok && record != nil && v["resource_id"] == *record
}
func validError(value any) bool {
	v, ok := value.(map[string]any)
	if !ok || !exact(v, "origin", "code", "message", "recovery", "operation") {
		return false
	}
	message, ok := v["message"].(string)
	if !ok || len(message) > 4096 || !oneOf(v["recovery"], "do_not_retry", "retry_identical", "inspect_operation", "rescan_required") {
		return false
	}

	origin, ok := v["origin"].(string)
	if !ok || !oneOf(v["code"], responseErrorCodes[origin]...) {
		return false
	}
	if v["operation"] == nil {
		return true
	}
	operation, ok := v["operation"].(map[string]any)
	if !ok || !exact(operation, "protocol", "actor", "operation") || !oneOf(operation["protocol"], "hn/0", "hn-artifact/1") || !printable(operation["operation"], 128) {
		return false
	}
	actor, ok := operation["actor"].(string)
	return ok && actorPattern.MatchString(actor)
}

func writeFrame(out io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) == 0 || len(raw) > frameBytes {
		return errors.New("frame exceeds host limit")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
	for _, part := range [][]byte{prefix[:], raw} {
		for len(part) > 0 {
			n, err := out.Write(part)
			if err != nil {
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}
func readFrame(in io.Reader, value any) error {
	var prefix [4]byte
	if _, err := io.ReadFull(in, prefix[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 || n > frameBytes {
		return errors.New("invalid host frame length")
	}
	raw := make([]byte, int(n))
	if _, err := io.ReadFull(in, raw); err != nil {
		return err
	}
	return decodeObject(raw, value)
}
func decodeObject(raw []byte, value any) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid UTF-8")
	}
	if err := validateEscapes(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("expected JSON object")
	}
	if err = validateMembers(d, '}'); err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(value)
}
func validateMembers(d *json.Decoder, end json.Delim) error {
	seen := map[string]bool{}
	for d.More() {
		if end == '}' {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return errors.New("duplicate or invalid JSON key")
			}
			seen[key] = true
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch v := token.(type) {
		case json.Delim:
			if v == '{' {
				err = validateMembers(d, '}')
			} else if v == '[' {
				err = validateMembers(d, ']')
			} else {
				return errors.New("invalid JSON delimiter")
			}
			if err != nil {
				return err
			}
		case json.Number:
			s := v.String()
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil || strings.ContainsAny(s, ".eE+") || s == "-0" || n < -9007199254740991 || n > 9007199254740991 {
				return errors.New("unsupported JSON number")
			}
		}
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token != end {
		return errors.New("invalid JSON close")
	}
	return nil
}

// encoding/json otherwise replaces lone UTF-16 surrogates with U+FFFD. Preserve
// the contract's exact text by rejecting malformed escape sequences first.
func validateEscapes(raw []byte) error {
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return errors.New("incomplete escape")
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return errors.New("incomplete Unicode escape")
		}
		v, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return err
		}
		i += 4
		if v >= 0xdc00 && v <= 0xdfff {
			return errors.New("unpaired Unicode surrogate")
		}
		if v < 0xd800 || v > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return errors.New("unpaired Unicode surrogate")
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return errors.New("unpaired Unicode surrogate")
		}
		i += 6
	}
	return nil
}

// Literal closed inventories from host-v1 section 6, not an imported SDK.
var responseErrorCodes = map[string][]string{
	"host":     {"denied_access", "incompatible_version", "unavailable_module", "unsupported_capability", "resource_limit", "cancelled", "uncertain_operation", "checkpoint_invalidated", "verification_failure", "invalid_input", "not_found", "repository_error"},
	"native":   {"actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision"},
	"artifact": {"invalid_input", "resource_limit", "verification_failure", "invalid_history", "incomplete_dependency", "unsupported_schema", "actor_stream_conflict", "operation_conflict", "stale_revision", "resource_conflict", "concurrent_update", "not_found", "repository_error", "cancelled", "uncertain_operation"},
}
