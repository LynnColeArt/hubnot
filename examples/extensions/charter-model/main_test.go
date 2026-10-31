package main

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func TestStrictPublicFrames(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, `{"a":"\ud800"}`, `{"a":1.0}`, `{"a":9007199254740992}`, `{} {}`, `[]`, "{\"a\":\"\xff\"}"} {
		if _, e := decodeObject([]byte(raw)); e == nil {
			t.Errorf("accepted hostile public JSON %q", raw)
		}
	}
	if _, e := decodeObject([]byte(`{"a":"雪 e\u0301","b":9007199254740991}`)); e != nil {
		t.Fatal(e)
	}
	for _, n := range []uint32{0, frameBytes + 1} {
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], n)
		if _, e := readFrame(bytes.NewReader(prefix[:])); e == nil {
			t.Fatalf("accepted invalid length %d", n)
		}
	}
	// Equality and first-over, including payload, never silently truncate.
	for _, n := range []int{frameBytes, frameBytes + 1} {
		raw := []byte(`{"s":"` + strings.Repeat("x", n-8) + `"}`)
		var buf bytes.Buffer
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
		buf.Write(prefix[:])
		buf.Write(raw)
		_, e := readFrame(&buf)
		if (e == nil) != (n == frameBytes) {
			t.Fatalf("frame boundary %d: %v", n, e)
		}
	}
}

func TestHelloBindingAndClosedLimits(t *testing.T) {
	valid := `{"protocol":"hn.extension/1","hello":{"session_id":"00000000000000000000000000000001","repository_id":"sha256:` + strings.Repeat("a", 64) + `","module_id":"example.charter/model","module_version":"1","registration_id":"sha256:` + strings.Repeat("b", 64) + `"},"limits":{"frame_bytes":8388608,"read_timeout_ms":30000,"mutation_timeout_ms":240000,"page_max":200,"one_outstanding":true}}`
	if e := acceptHello([]byte(valid)); e != nil {
		t.Fatal(e)
	}
	if e := checkPinnedBinding([]byte(valid), map[string]any{"expected_repository_id": "sha256:" + strings.Repeat("a", 64), "expected_registration_id": "sha256:" + strings.Repeat("b", 64)}); e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"expected_repository_id", "expected_registration_id"} {
		if e := checkPinnedBinding([]byte(valid), map[string]any{field: "sha256:" + strings.Repeat("c", 64)}); e == nil || e.Error() != "operator-pinned host binding mismatch" {
			t.Fatal("wrong binding accepted", field, e)
		}
	}
	for _, raw := range []string{strings.Replace(valid, "example.charter/model", "other.example/model", 1), strings.Replace(valid, "8388608", "8388609", 1), strings.Replace(valid, `"one_outstanding":true`, `"one_outstanding":false`, 1), strings.Replace(valid, `"protocol":`, `"unknown":1,"protocol":`, 1), strings.Replace(valid, strings.Repeat("a", 64), "bad", 1)} {
		if acceptHello([]byte(raw)) == nil {
			t.Errorf("accepted incorrect binding/limit: %s", raw)
		}
	}
}

func TestLocalInputBound(t *testing.T) {
	for _, n := range []int{inputBytes, inputBytes + 1} {
		raw := `{"directive_text":"` + strings.Repeat("x", n-21) + `"}`
		if len(raw) != n {
			t.Fatalf("bad fixture %d != %d", len(raw), n)
		}
		_, e := readInput(strings.NewReader(raw))
		if (e == nil) != (n == inputBytes) {
			t.Fatalf("input boundary %d: %v", n, e)
		}
	}
}

func TestExampleMethodInventoryAndNestingBound(t *testing.T) {
	if !reflect.DeepEqual(methods, []string{"identity.public", "artifact.type.publish", "artifact.type.read", "artifact.mutate", "artifact.read", "artifact.search", "artifact.operation"}) {
		t.Fatal("example advertised unused or missing methods", methods)
	}
	for _, n := range []int{64, 65} {
		raw := strings.Repeat(`{"x":`, n) + `0` + strings.Repeat(`}`, n)
		_, e := decodeObject([]byte(raw))
		if (e == nil) != (n == 64) {
			t.Fatalf("nesting %d: %v", n, e)
		}
	}
}
