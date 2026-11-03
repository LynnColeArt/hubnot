package extension

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"testing"
	"time"
)

func testFrame(payload []byte) []byte {
	b := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(b, uint32(len(payload)))
	copy(b[4:], payload)
	return b
}
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code || e.Origin != "host" || e.Operation != nil {
		t.Fatalf("got %v, want host/%s with null operation", err, code)
	}
}

type byteReader struct{ io.Reader }

func (r byteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}
func TestFrameReadBounds(t *testing.T) {
	for _, n := range []int{1, FrameLimit} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			payload := bytes.Repeat([]byte(" "), n)
			got, err := ReadFrame(context.Background(), io.NopCloser(bytes.NewReader(testFrame(payload))))
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("at bound %d: %v", n, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		raw  []byte
		code string
	}{
		{"empty", nil, "invalid_input"}, {"partial prefix", []byte{0, 0}, "invalid_input"}, {"zero", []byte{0, 0, 0, 0}, "invalid_input"}, {"over", []byte{0, 128, 0, 1}, "resource_limit"}, {"uint32 max", []byte{255, 255, 255, 255}, "resource_limit"}, {"partial payload", []byte{0, 0, 0, 2, '{'}, "invalid_input"}, {"trailing", append(testFrame([]byte("{}")), 0), "invalid_input"}, {"coalesced", append(testFrame([]byte("{}")), testFrame([]byte("{}"))...), "invalid_input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadFrame(context.Background(), io.NopCloser(bytes.NewReader(tc.raw)))
			requireCode(t, err, tc.code)
		})
	}
	raw := testFrame([]byte("{}"))
	got, err := ReadFrame(context.Background(), io.NopCloser(byteReader{bytes.NewReader(raw)}))
	if err != nil || string(got) != "{}" {
		t.Fatal(err)
	}
}
func TestFrameCancelIncludesFinalEOF(t *testing.T) {
	for _, raw := range [][]byte{nil, {0, 0}, {0, 0, 0, 2, '{'}, testFrame([]byte("{}"))} {
		t.Run("blocked", func(t *testing.T) {
			r, w := io.Pipe()
			defer w.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			done := make(chan struct{})
			go func() { defer close(done); _, _ = w.Write(raw) }()
			start := time.Now()
			_, err := ReadFrame(ctx, r)
			requireCode(t, err, "cancelled")
			if time.Since(start) > time.Second {
				t.Fatal("reader did not unblock")
			}
			<-done
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ReadFrame(ctx, io.NopCloser(bytes.NewReader(testFrame([]byte("{}")))))
	requireCode(t, err, "cancelled")
}
