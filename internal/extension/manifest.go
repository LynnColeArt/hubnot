// Package extension handles explicit local extension management. Reading its
// declarations never launches a process or gives a declaration authority.
package extension

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const ManifestLimit = 131072
const HostProtocol = "hn.extension/1"

// Error is the bounded public error used by explicit extension commands.
type Error struct {
	Origin    string `json:"origin"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Recovery  string `json:"recovery"`
	Operation any    `json:"operation"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func failure(code, message string) error {
	return &Error{Origin: "host", Code: code, Message: message, Recovery: "do_not_retry"}
}

// Manifest is descriptive interest in methods, never permission to invoke them.
type Manifest struct {
	Protocol      string   `json:"protocol"`
	ModuleID      string   `json:"module_id"`
	ModuleVersion string   `json:"module_version"`
	HostProtocol  string   `json:"host_protocol"`
	Capabilities  []string `json:"capabilities"`
}

var modulePattern = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,63}/[a-z][a-z0-9.-]{0,63}$`)

func ValidModuleID(id string) bool {
	return modulePattern.MatchString(id) && !strings.HasPrefix(id, "hn/") && !strings.HasPrefix(id, "hubnot/")
}

var methods = map[string]bool{"capabilities.list": true, "identity.public": true, "resource.read": true, "resource.mutate": true, "resource.operation": true, "artifact.read": true, "artifact.mutate": true, "artifact.operation": true, "artifact.search": true, "artifact.type.read": true, "artifact.type.publish": true, "changes.capture": true, "changes.page": true, "changes.since": true}

func ParseManifest(data []byte) (Manifest, error) {
	return parseManifest(data, true)
}

func parseManifest(data []byte, compatible bool) (Manifest, error) {
	var m Manifest
	value, err := strictJSON(data, ManifestLimit)
	if err != nil {
		return m, err
	}
	object, ok := value.(map[string]any)
	if !ok || !exactFields(object, "protocol", "module_id", "module_version", "host_protocol", "capabilities") {
		return m, failure("invalid_input", "manifest must contain exactly the five manifest fields")
	}
	encoded, _ := json.Marshal(object)
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&m); err != nil {
		return m, failure("invalid_input", "invalid manifest fields")
	}
	if err = m.validateStructure(); err != nil {
		return m, err
	}
	if compatible && m.HostProtocol != HostProtocol {
		return m, failure("incompatible_version", "unsupported host protocol")
	}
	return m, nil
}
func (m Manifest) validate() error {
	if err := m.validateStructure(); err != nil {
		return err
	}
	if m.HostProtocol != HostProtocol {
		return failure("incompatible_version", "unsupported host protocol")
	}
	return nil
}
func (m Manifest) validateStructure() error {
	if m.Protocol != "hn.extension-manifest/1" {
		return failure("incompatible_version", "unsupported manifest protocol")
	}
	if !ValidModuleID(m.ModuleID) {
		return failure("invalid_input", "invalid or reserved module ID")
	}
	if len(m.ModuleVersion) < 1 || len(m.ModuleVersion) > 64 {
		return failure("invalid_input", "module version must have 1..64 printable ASCII bytes")
	}
	for _, b := range []byte(m.ModuleVersion) {
		if b < 0x21 || b > 0x7e {
			return failure("invalid_input", "module version must have 1..64 printable ASCII bytes")
		}
	}
	if len(m.HostProtocol) < 1 || len(m.HostProtocol) > 64 {
		return failure("invalid_input", "invalid host protocol declaration")
	}
	for _, b := range []byte(m.HostProtocol) {
		if b < 0x21 || b > 0x7e {
			return failure("invalid_input", "invalid host protocol declaration")
		}
	}
	if m.Capabilities == nil {
		return failure("invalid_input", "capabilities must be an array")
	}
	seen := map[string]bool{}
	for _, method := range m.Capabilities {
		if !methods[method] || seen[method] {
			return failure("invalid_input", "unknown or duplicate capability")
		}
		seen[method] = true
	}
	return nil
}
func ReadManifest(r io.Reader) (Manifest, error) {
	data, err := io.ReadAll(io.LimitReader(r, ManifestLimit+1))
	if err != nil {
		return Manifest{}, failure("invalid_input", "cannot read manifest")
	}
	return ParseManifest(data)
}

// Canonical manifest and registration identities use sorted UTF-8 object keys.
func canonical(value any) []byte {
	data, _ := json.Marshal(value)
	var object any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	_ = decoder.Decode(&object)
	data, _ = json.Marshal(object)
	return data
}
func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func (m Manifest) Digest() string { return digest(canonical(m)) }

// strictJSON rejects duplicate keys, malformed Unicode and values outside the
// public integer domain before decoding any closed contract object.
func strictJSON(data []byte, limit int) (any, error) {
	if len(data) > limit {
		return nil, failure("resource_limit", "JSON input exceeds its byte limit")
	}
	if !utf8.Valid(data) || !validEscapes(data) {
		return nil, failure("invalid_input", "JSON must contain valid Unicode scalars")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := jsonValue(decoder, 0)
	if err != nil {
		return nil, failure("invalid_input", "malformed or duplicate-key JSON")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, failure("invalid_input", "JSON has trailing data")
	}
	return value, nil
}
func jsonValue(d *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("JSON depth")
	}
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch v := token.(type) {
	case json.Delim:
		if v == '{' {
			object := map[string]any{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("key")
				}
				if _, ok = object[name]; ok {
					return nil, fmt.Errorf("duplicate")
				}
				value, err := jsonValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				object[name] = value
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return nil, fmt.Errorf("object")
			}
			return object, nil
		}
		if v == '[' {
			array := []any{}
			for d.More() {
				value, err := jsonValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return nil, fmt.Errorf("array")
			}
			return array, nil
		}
		return nil, fmt.Errorf("delimiter")
	case json.Number:
		s := string(v)
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || strings.ContainsAny(s, ".eE+") || s == "-0" || n < -9007199254740991 || n > 9007199254740991 {
			return nil, fmt.Errorf("number")
		}
		return v, nil
	default:
		return token, nil
	}
}
func validEscapes(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}
		i++
		for ; i < len(data) && data[i] != '"'; i++ {
			if data[i] != '\\' {
				continue
			}
			i++
			if i >= len(data) {
				return false
			}
			if data[i] != 'u' {
				continue
			}
			if i+4 >= len(data) {
				return false
			}
			n, e := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
			if e != nil {
				return false
			}
			i += 4
			if n >= 0xdc00 && n <= 0xdfff {
				return false
			}
			if n >= 0xd800 && n <= 0xdbff {
				if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
					return false
				}
				low, e := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
				if e != nil || low < 0xdc00 || low > 0xdfff {
					return false
				}
				i += 6
			}
		}
	}
	return true
}

func exactFields(object map[string]any, names ...string) bool {
	if len(object) != len(names) {
		return false
	}
	for _, name := range names {
		if _, present := object[name]; !present {
			return false
		}
	}
	return true
}
