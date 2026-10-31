package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hubnot/internal/extension"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func extensionProtocolBinary(t *testing.T) string {
	t.Helper()
	if supplied := os.Getenv("HN_WP15_BINARY"); supplied != "" {
		return supplied
	}
	return extensionTestBinary(t)
}
func protocolRawFrame(payload []byte) []byte {
	raw := make([]byte, len(payload)+4)
	binary.BigEndian.PutUint32(raw, uint32(len(payload)))
	copy(raw[4:], payload)
	return raw
}
func protocolTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, path)
		result[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestExtensionFrameInspectCLI(t *testing.T) {
	binary := extensionProtocolBinary(t)
	repo := t.TempDir()
	home := t.TempDir()
	config := filepath.Join(home, "config")
	registry := filepath.Join(config, "hubnot", "extensions", "registry.json")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, out)
	}
	for path, value := range map[string]string{registry: "malformed registry must remain unread", filepath.Join(repo, ".hn", "extensions.json"): "malformed declaration must remain unread", filepath.Join(repo, ".git", "hn", "extensions", "enabled.json"): "malformed enablement must remain unread"} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	beforeRepo, beforeHome := protocolTree(t, repo), protocolTree(t, home)
	env := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	invoke := func(raw []byte, want string, args ...string) map[string]any {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append([]string{"extension", "frame", "inspect"}, args...)...)
		cmd.Dir = repo
		cmd.Env = env
		cmd.Stdin = bytes.NewReader(raw)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatalf("CLI exceeded watchdog: %s", stderr.String())
		}
		var wrapper map[string]any
		if e := json.Unmarshal(stdout.Bytes(), &wrapper); e != nil {
			t.Fatalf("JSON %v: %q %q", e, stdout.String(), stderr.String())
		}
		if len(wrapper) != 3 {
			t.Fatalf("wrapper fields %v", wrapper)
		}
		if want == "" {
			if err != nil || wrapper["ok"] != true || wrapper["error"] != nil {
				t.Fatalf("positive: %v %s %s", err, stdout.String(), stderr.String())
			}
			return wrapper["data"].(map[string]any)
		}
		failure, ok := wrapper["error"].(map[string]any)
		if err == nil || wrapper["ok"] != false || wrapper["data"] != nil || !ok || failure["origin"] != "host" || failure["code"] != want || failure["recovery"] != "do_not_retry" || failure["operation"] != nil {
			t.Fatalf("want %s: %v %s %s", want, err, stdout.String(), stderr.String())
		}
		if strings.Contains(stdout.String(), "DO-NOT-ECHO") || strings.Contains(stderr.String(), "DO-NOT-ECHO") {
			t.Fatal("input leaked")
		}
		return nil
	}
	ready := []byte(`{"protocol":"hn.extension/1","ready":true}`)
	data := invoke(protocolRawFrame(ready), "", "--input", "-", "--kind", "ready", "--json")
	want := map[string]any{"frame_bytes": float64(len(ready) + 4), "payload_bytes": float64(len(ready)), "message_kind": "ready", "request_id": nil, "method": nil, "validation": "frame_and_typed_message", "session_state": "not_checked", "authorization": "not_checked", "semantic_validation": "not_checked", "signature_integrity": "not_checked", "dispatch": "not_performed"}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("data %v", data)
	}
	path := filepath.Join(t.TempDir(), "ready.frame")
	if err := os.WriteFile(path, protocolRawFrame(ready), 0600); err != nil {
		t.Fatal(err)
	}
	invoke(nil, "", "--input", path, "--kind", "ready")
	for _, tc := range []struct {
		name string
		raw  []byte
		code string
	}{
		{"zero", []byte{0, 0, 0, 0}, "invalid_input"}, {"oversize", []byte{0, 128, 0, 1}, "resource_limit"}, {"partial prefix", []byte{0, 0, 0}, "invalid_input"}, {"partial payload", []byte{0, 0, 0, 2, '{'}, "invalid_input"}, {"coalesced", append(protocolRawFrame(ready), protocolRawFrame(ready)...), "invalid_input"}, {"raw JSON", ready, "resource_limit"},
		{"primitive", protocolRawFrame([]byte(`true`)), "invalid_input"}, {"array", protocolRawFrame([]byte(`[]`)), "invalid_input"}, {"null", protocolRawFrame([]byte(`null`)), "invalid_input"}, {"null bool", protocolRawFrame([]byte(`{"protocol":"hn.extension/1","ready":null}`)), "invalid_input"}, {"unknown", protocolRawFrame([]byte(`{"protocol":"hn.extension/1","ready":true,"DO-NOT-ECHO":"secret"}`)), "invalid_input"}, {"duplicate", protocolRawFrame([]byte(`{"protocol":"hn.extension/1","ready":true,"ready":true}`)), "invalid_input"}, {"Unicode", protocolRawFrame([]byte(`{"protocol":"hn.extension/1","ready":"\ud800"}`)), "invalid_input"}, {"wrong version", protocolRawFrame([]byte(`{"protocol":"hn.extension/2","ready":true}`)), "incompatible_version"},
	} {
		t.Run(tc.name, func(t *testing.T) { invoke(tc.raw, tc.code, "--input", "-", "--kind", "ready") })
	}
	for _, args := range [][]string{{}, {"--kind", "ready"}, {"--input", "-"}, {"--input", "-", "--kind", "unknown"}, {"--input", "-", "--kind", "ready", "--input", "-"}, {"--input", "-", "--kind", "ready", "--json", "--json"}, {"--input", "-", "--kind", "ready", "--expected-request-id", "r"}, {"--input", "-", "--kind", "ready", "--timeout-ms", "0"}, {"--input", "-", "--kind", "ready", "--timeout-ms", "30001"}, {"--input", "-", "--kind", "ready", "--method", ""}} {
		invoke(protocolRawFrame(ready), "invalid_input", args...)
	}
	invoke(nil, "invalid_input", "--input", repo, "--kind", "ready")
	invoke(nil, "repository_error", "--input", filepath.Join(repo, "missing"), "--kind", "ready")
	request := []byte(`{"protocol":"hn.extension/1","request_id":"r","method":"capabilities.list","params":{}}`)
	invoke(protocolRawFrame(request), "", "--input", "-", "--kind", "request")
	response := []byte(`{"protocol":"hn.extension/1","request_id":"r","ok":true,"data":{"initialized":false,"actor":null,"name":null,"public_key":null},"error":null}`)
	invoke(protocolRawFrame(response), "", "--input", "-", "--kind", "response", "--expected-request-id", "r", "--method", "identity.public")
	invoke(protocolRawFrame(response), "invalid_input", "--input", "-", "--kind", "response", "--expected-request-id", "other", "--method", "identity.public")
	// The payload itself is valid at the byte cap; padding is transport whitespace.
	at := append(append([]byte{}, ready...), bytes.Repeat([]byte(" "), 8388608-len(ready))...)
	invoke(protocolRawFrame(at), "", "--input", "-", "--kind", "ready")
	if got := protocolTree(t, repo); !reflect.DeepEqual(got, beforeRepo) {
		t.Fatalf("repository refs/objects/private state changed: %v", got)
	}
	if got := protocolTree(t, home); !reflect.DeepEqual(got, beforeHome) {
		t.Fatalf("private config changed: %v", got)
	}
}
func TestExtensionFrameInspectPartialDeadlineCLI(t *testing.T) {
	binary := extensionProtocolBinary(t)
	ready := protocolRawFrame([]byte(`{"protocol":"hn.extension/1","ready":true}`))
	for _, raw := range [][]byte{nil, {0}, {0, 0, 0, 2, '{'}, ready} {
		t.Run("blocked input", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "extension", "frame", "inspect", "--input", "-", "--kind", "ready", "--timeout-ms", "30")
			cmd.Dir = t.TempDir()
			cmd.Env = operationalTestEnvironment(t.TempDir())
			pipe, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			if len(raw) > 0 {
				if _, err := pipe.Write(raw); err != nil {
					t.Fatal(err)
				}
			}
			start := time.Now()
			err = cmd.Wait()
			_ = pipe.Close()
			if ctx.Err() != nil || time.Since(start) > 2*time.Second {
				t.Fatal("partial frame/EOF timeout did not terminate")
			}
			var result extensionCLIResult
			if e := json.Unmarshal(stdout.Bytes(), &result); e != nil {
				t.Fatalf("%v %s %s", e, stdout.String(), stderr.String())
			}
			if err == nil || result.OK || result.Error.Code != "cancelled" || result.Error.Origin != "host" {
				t.Fatalf("want cancelled: %v %s %s", err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestExtensionFrameInputRestoresStdin(t *testing.T) {
	// No parallel test: this deliberately substitutes this test process's stdin,
	// then verifies both blocking modes are restored after timeout and double close.
	original := os.Stdin
	defer func() { os.Stdin = original }()
	for _, nonblocking := range []bool{false, true} {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = reader
		connection, err := reader.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		fd := -1
		if err := connection.Control(func(raw uintptr) { fd = int(raw) }); err != nil {
			t.Fatal(err)
		}
		if err := syscall.SetNonblock(fd, nonblocking); err != nil {
			t.Fatal(err)
		}
		input, err := openFrameInput(context.Background(), "-")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		_, err = extension.ReadFrame(ctx, input)
		cancel()
		var public *extension.Error
		if !errors.As(err, &public) || public.Code != "cancelled" {
			t.Fatalf("timeout: %v", err)
		}
		if err := input.Close(); err != nil {
			t.Fatal(err)
		}
		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFL, 0)
		if errno != 0 || (flags&syscall.O_NONBLOCK != 0) != nonblocking {
			t.Fatalf("stdin mode changed: %v %d", errno, flags)
		}
		_ = reader.Close()
		_ = writer.Close()
	}
}
