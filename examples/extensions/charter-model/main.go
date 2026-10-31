// charter-model is an optional, standalone example consumer of hn.extension/1.
// It deliberately imports only the Go standard library.
package main

import (
	"bytes"
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
	"unicode/utf8"
)

const protocol = "hn.extension/1"
const moduleID = "example.charter/model"
const frameBytes = 8388608
const inputBytes = 1048576

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var sessionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var sessionActor = regexp.MustCompile(`^[0-9a-f]{64}$`)
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
	if len(m) != 5 || m["protocol"] != protocol || m["request_id"] != id {
		return nil, errors.New("response envelope/correlation mismatch")
	}
	for _, k := range []string{"protocol", "request_id", "ok", "data", "error"} {
		if _, yes := m[k]; !yes {
			return nil, errors.New("incomplete response envelope")
		}
	}
	ok, exists := m["ok"].(bool)
	if !exists {
		return nil, errors.New("missing response outcome")
	}
	if ok {
		if m["error"] != nil || m["data"] == nil {
			return nil, errors.New("malformed success")
		}
	} else {
		err, valid := m["error"].(map[string]any)
		if !valid || len(err) != 5 || m["data"] != nil {
			return nil, errors.New("malformed refusal")
		}
		for _, k := range []string{"origin", "code", "message", "recovery", "operation"} {
			if _, yes := err[k]; !yes {
				return nil, errors.New("incomplete refusal")
			}
		}
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
