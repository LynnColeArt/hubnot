// Package artifact verifies inert hn-artifact/1 data. It grants no authority.
package artifact

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

const MaxEnvelopeBytes = 1048576
const maxInteger = int64(9007199254740991)

// Error is a bounded public refusal, never an echo of untrusted content.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string          { return e.Code + ": " + e.Message }
func refuse(code, message string) error { return &Error{code, message} }

// Read consumes at most the envelope bound plus one byte, including on stdin.
func Read(r io.Reader) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, MaxEnvelopeBytes+1))
	if e != nil {
		return nil, refuse("invalid_input", "cannot read envelope")
	}
	if len(b) > MaxEnvelopeBytes {
		return nil, refuse("resource_limit", "envelope exceeds 1048576 bytes")
	}
	return b, nil
}

// canonicalValue rejects duplicate keys and malformed scalars before
// encoding/json could replace them, then encodes exact C(value).
func canonicalValue(raw []byte) (any, []byte, error) {
	v, e := parse(raw)
	if e != nil {
		return nil, nil, e
	}
	b, e := json.Marshal(v)
	return v, b, e
}
func parse(raw []byte) (any, error) {
	if len(raw) > MaxEnvelopeBytes {
		return nil, refuse("resource_limit", "envelope exceeds 1048576 bytes")
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
	return v, nil
}
func value(d *json.Decoder, depth int) (any, error) {
	t, e := d.Token()
	if e != nil {
		return nil, refuse("invalid_input", "malformed JSON")
	}
	switch t := t.(type) {
	case json.Delim:
		// Keyword wrappers add structural nesting beyond modeled schema depth.
		if depth >= 32 {
			return nil, refuse("resource_limit", "JSON nesting exceeds parser bound")
		}
		switch t {
		case '{':
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, refuse("invalid_input", "malformed object")
				}
				s, ok := k.(string)
				if !ok {
					return nil, refuse("invalid_input", "malformed object key")
				}
				if _, ok = m[s]; ok {
					return nil, refuse("invalid_input", "duplicate JSON key")
				}
				if len(m) >= 64 {
					return nil, refuse("resource_limit", "object exceeds 64 properties")
				}
				v, e := value(d, depth+1)
				if e != nil {
					return nil, e
				}
				m[s] = v
			}
			if _, e = d.Token(); e != nil {
				return nil, refuse("invalid_input", "malformed object")
			}
			return m, nil
		case '[':
			a := []any{}
			for d.More() {
				if len(a) >= 1024 {
					return nil, refuse("resource_limit", "array exceeds 1024 entries")
				}
				v, e := value(d, depth+1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			if _, e = d.Token(); e != nil {
				return nil, refuse("invalid_input", "malformed array")
			}
			return a, nil
		}
		return nil, refuse("invalid_input", "unexpected delimiter")
	case json.Number:
		s := string(t)
		n, e := strconv.ParseInt(s, 10, 64)
		if e != nil || n < -maxInteger || n > maxInteger || strconv.FormatInt(n, 10) != s {
			return nil, refuse("invalid_input", "number must be canonical safe integer")
		}
		return t, nil
	case string, bool, nil:
		return t, nil
	}
	return nil, refuse("invalid_input", "unexpected JSON value")
}
func unicodeEscapes(b []byte) error {
	in := false
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			in = !in
			continue
		}
		if !in || b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) {
			break
		}
		if b[i] != 'u' {
			continue
		}
		if i+4 >= len(b) {
			return refuse("invalid_input", "malformed Unicode escape")
		}
		n, e := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
		if e != nil {
			return refuse("invalid_input", "malformed Unicode escape")
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return refuse("invalid_input", "unpaired Unicode surrogate")
		}
		if n < 0xd800 || n > 0xdbff {
			continue
		}
		if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
			return refuse("invalid_input", "unpaired Unicode surrogate")
		}
		lo, e := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
		if e != nil || utf16.DecodeRune(rune(n), rune(lo)) == utf8.RuneError {
			return refuse("invalid_input", "unpaired Unicode surrogate")
		}
		i += 6
	}
	return nil
}
