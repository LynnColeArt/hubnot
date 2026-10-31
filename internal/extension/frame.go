package extension

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
)

// FrameLimit bounds a payload before allocation, including hostile length words.
const FrameLimit = 8388608

// ReadFrame consumes exactly one frame and EOF. Cancellation closes the explicit
// input, including when waiting for the final EOF. It does not create a session.
// The caller owns this input exclusively and must supply a Close that unblocks
// its reads (as regular files, pipes and the CLI stdin do).
func ReadFrame(ctx context.Context, input io.ReadCloser) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, failure("cancelled", "frame inspection cancelled")
	}
	type result struct {
		payload []byte
		err     error
	}
	done := make(chan result, 1)
	go func() { payload, err := readFrame(input); done <- result{payload, err} }()
	select {
	case <-ctx.Done():
		_ = input.Close()
		<-done
		return nil, failure("cancelled", "frame inspection cancelled")
	case got := <-done:
		if ctx.Err() != nil {
			return nil, failure("cancelled", "frame inspection cancelled")
		}
		return got.payload, got.err
	}
}
func frameReadError(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return failure("invalid_input", "incomplete frame")
	}
	return failure("repository_error", "cannot read frame input")
}
func readFrame(input io.Reader) ([]byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(input, prefix[:]); err != nil {
		return nil, frameReadError(err)
	}
	size := binary.BigEndian.Uint32(prefix[:])
	if size == 0 {
		return nil, failure("invalid_input", "frame payload is empty")
	}
	if size > FrameLimit {
		return nil, failure("resource_limit", "frame payload exceeds byte limit")
	}
	payload := make([]byte, int(size))
	if _, err := io.ReadFull(input, payload); err != nil {
		return nil, frameReadError(err)
	}
	var tail [1]byte
	n, err := input.Read(tail[:])
	// A legal Reader can return (0,nil); ReadFull must continue until EOF or data.
	if n == 0 && err == nil {
		n, err = io.ReadFull(input, tail[:])
	}
	if n > 0 {
		return nil, failure("invalid_input", "frame input has trailing bytes")
	}
	if err != io.EOF {
		return nil, frameReadError(err)
	}
	return payload, nil
}
