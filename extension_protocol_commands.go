package main

import (
	"context"
	"io"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"hubnot/internal/extension"
)

func init() { extensionSubcommands["frame"] = extensionFrame }

func extensionFrame(args []string) (any, error) {
	if len(args) == 0 || args[0] != "inspect" {
		return nil, extensionInput("expected frame inspect")
	}
	values := map[string]string{}
	for i := 1; i < len(args); i++ {
		name := args[i]
		switch name {
		case "--input", "--kind", "--expected-request-id", "--method", "--timeout-ms", "--json":
		default:
			return nil, extensionInput("unknown frame inspection argument")
		}
		if _, exists := values[name]; exists {
			return nil, extensionInput("duplicate frame inspection argument")
		}
		if name == "--json" {
			values[name] = "true"
			continue
		}
		if i+1 == len(args) {
			return nil, extensionInput("missing frame inspection argument value")
		}
		i++
		values[name] = args[i]
	}
	input, hasInput := values["--input"]
	kind, hasKind := values["--kind"]
	if !hasInput || input == "" || !hasKind {
		return nil, extensionInput("frame inspect requires input and kind")
	}
	switch kind {
	case "hello", "ready", "request", "response", "cancel":
	default:
		return nil, extensionInput("unknown frame inspection kind")
	}
	for _, flag := range []string{"--expected-request-id", "--method"} {
		if value, present := values[flag]; present && value == "" {
			return nil, extensionInput("empty frame correlation argument")
		}
	}
	timeout := 30000
	if raw, present := values["--timeout-ms"]; present {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 30000 {
			return nil, extensionInput("frame timeout must be 1..30000 milliseconds")
		}
		timeout = n
	}
	switch kind {
	case "hello", "ready", "request":
		if _, ok := values["--expected-request-id"]; ok {
			return nil, extensionInput("correlation flags are not permitted for this kind")
		}
		if _, ok := values["--method"]; ok {
			return nil, extensionInput("correlation flags are not permitted for this kind")
		}
	case "cancel":
		if _, ok := values["--expected-request-id"]; !ok {
			return nil, extensionInput("cancel requires expected request ID")
		}
		if _, ok := values["--method"]; ok {
			return nil, extensionInput("cancel forbids method")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()
	reader, err := openFrameInput(ctx, input)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	payload, err := extension.ReadFrame(ctx, reader)
	if err != nil {
		return nil, err
	}
	result, err := extension.InspectMessage(payload, kind, values["--expected-request-id"], values["--method"])
	if ctx.Err() != nil {
		return nil, &extension.Error{Origin: "host", Code: "cancelled", Message: "frame inspection cancelled", Recovery: "do_not_retry"}
	}
	return result, err
}
func frameInputError(code, message string) error {
	return &extension.Error{Origin: "host", Code: code, Message: message, Recovery: "do_not_retry"}
}
func openFrameInput(ctx context.Context, path string) (io.ReadCloser, error) {
	if path == "-" {
		// Inherited stdin may be a blocking descriptor. Closing that original
		// os.File does not necessarily unblock a kernel read. Give the runtime poller
		// an owned nonblocking duplicate so cancellation can close and join its read.
		connection, err := os.Stdin.SyscallConn()
		if err != nil {
			return nil, frameInputError("repository_error", "cannot open frame input")
		}
		fd, stdinFD := -1, -1
		var duplicateError error
		if err = connection.Control(func(raw uintptr) { stdinFD = int(raw); fd, duplicateError = syscall.Dup(stdinFD) }); err != nil || duplicateError != nil {
			return nil, frameInputError("repository_error", "cannot open frame input")
		}

		syscall.CloseOnExec(fd)
		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFL, 0)
		if errno != 0 {
			_ = syscall.Close(fd)
			return nil, frameInputError("repository_error", "cannot open frame input")
		}
		if err = syscall.SetNonblock(fd, true); err != nil {
			_ = syscall.Close(fd)
			return nil, frameInputError("repository_error", "cannot open frame input")
		}
		return &frameStdin{File: os.NewFile(uintptr(fd), "frame input"), inheritedFD: stdinFD, nonblocking: flags&syscall.O_NONBLOCK != 0}, nil
	}
	type opened struct {
		file *os.File
		err  error
	}
	result := make(chan opened)
	// Include path resolution/open/stat in the absolute utility deadline. A late
	// open is closed by its owner; it can never resume inspection after timeout.
	go func() {
		var file *os.File
		info, err := os.Stat(path)
		if err == nil && !info.Mode().IsRegular() {
			err = frameInputError("invalid_input", "frame input must be a regular file")
		}
		if err == nil {
			file, err = os.Open(path)
		}
		if err == nil {
			info, err = file.Stat()
			if err == nil && !info.Mode().IsRegular() {
				err = frameInputError("invalid_input", "frame input must be a regular file")
			}
		}
		select {
		case result <- opened{file, err}:
		case <-ctx.Done():
			if file != nil {
				_ = file.Close()
			}
		}
	}()
	select {
	case <-ctx.Done():
		return nil, frameInputError("cancelled", "frame inspection cancelled")
	case opened := <-result:
		if opened.err != nil {
			if opened.file != nil {
				_ = opened.file.Close()
			}
			if public, ok := opened.err.(*extension.Error); ok {
				return nil, public
			}
			return nil, frameInputError("repository_error", "cannot open frame input")
		}
		return opened.file, nil
	}
}

// dup shares file-status flags with the inherited description. Restore its exact
// blocking mode after closing/joining the owned pollable descriptor, including
// cancellation and double Close from the caller. No later command inherits a
// changed stdin mode from this explicit, exclusive input consumer.
type frameStdin struct {
	*os.File
	inheritedFD int
	nonblocking bool
	once        sync.Once
	closeError  error
}

func (input *frameStdin) Close() error {
	input.once.Do(func() {
		input.closeError = input.File.Close()
		if err := syscall.SetNonblock(input.inheritedFD, input.nonblocking); input.closeError == nil {
			input.closeError = err
		}
	})
	return input.closeError
}
