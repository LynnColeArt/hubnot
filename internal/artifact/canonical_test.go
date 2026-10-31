package artifact

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func requireCode(t *testing.T, e error, code string) {
	t.Helper()
	if e == nil {
		t.Fatalf("expected %s refusal", code)
	}
	a, ok := e.(*Error)
	if !ok || a.Code != code {
		t.Fatalf("expected %s, got %v", code, e)
	}
}
func TestCanonicalDomain(t *testing.T) {
	raw := []byte(` {"é":"é","z":"<>&\u2028\u2029/\b\t\n\f\r\u0001","a":-9007199254740991,"😀":"\ud83d\ude00"} `)
	want := `{"a":-9007199254740991,"z":"\u003c\u003e\u0026\u2028\u2029/\b\t\n\f\r\u0001","é":"é","😀":"😀"}`
	got, e := canonical(raw)
	if e != nil || string(got) != want {
		t.Fatalf("canonical bytes %q, %v", got, e)
	}
	for _, s := range []string{`{"a":0,"\u0061":1}`, `"\ud800"`, `"\udc00"`, `"\ud800\u0061"`, `-0`, `1.0`, `1e0`, `01`, `+1`, `9007199254740992`, `-9007199254740992`, `{} {}`, `{"a":}`, "\"\xff\""} {
		t.Run(fmt.Sprintf("%q", s), func(t *testing.T) { _, e := canonical([]byte(s)); requireCode(t, e, "invalid_input") })
	}
}
func TestCanonicalConsumptionBounds(t *testing.T) {
	for _, n := range []int{MaxEnvelopeBytes, MaxEnvelopeBytes + 1} {
		b, e := Read(strings.NewReader(strings.Repeat("x", n)))
		if n == MaxEnvelopeBytes {
			if e != nil || len(b) != n {
				t.Fatal(len(b), e)
			}
		} else {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, n := range []int{64, 65} {
		parts := []string{}
		for i := 0; i < n; i++ {
			parts = append(parts, fmt.Sprintf(`"p%d":0`, i))
		}
		_, e := canonical([]byte("{" + strings.Join(parts, ",") + "}"))
		if n == 64 && e != nil {
			t.Fatal(e)
		}
		if n == 65 {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, n := range []int{1024, 1025} {
		_, e := canonical([]byte("[" + strings.Repeat("0,", n-1) + "0]"))
		if n == 1024 && e != nil {
			t.Fatal(e)
		}
		if n == 1025 {
			requireCode(t, e, "resource_limit")
		}
	}
	for _, n := range []int{32, 33} {
		_, e := canonical([]byte(strings.Repeat("[", n) + "0" + strings.Repeat("]", n)))
		if n == 32 && e != nil {
			t.Fatal(e)
		}
		if n == 33 {
			requireCode(t, e, "resource_limit")
		}
	}
	raw := append([]byte{'"'}, bytes.Repeat([]byte{'x'}, MaxEnvelopeBytes-2)...)
	raw = append(raw, '"')
	if _, e := canonical(raw); e != nil {
		t.Fatal(e)
	}
	_, e := canonical(append(raw, ' '))
	requireCode(t, e, "resource_limit")
}

func canonical(raw []byte) ([]byte, error) { _, b, e := canonicalValue(raw); return b, e }
