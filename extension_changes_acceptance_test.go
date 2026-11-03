package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	c "hubnot/internal/contribution"
	"hubnot/internal/extension"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func changesFixture(t *testing.T, grants ...string) *nativeHostFixture {
	t.Helper()
	f := newNativeHostFixture(t, "native:issue=read")
	config := wp25FixtureConfig(t, f.home)
	for i, entry := range f.env {
		if strings.HasPrefix(entry, "XDG_CONFIG_HOME=") {
			f.env[i] = "XDG_CONFIG_HOME=" + config
		}
	}
	f.command("extension", "remove", "example.native/tool", "--json")
	manifest := filepath.Join(f.home, "changes-manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.native/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["resource.read","resource.mutate","artifact.read","artifact.mutate","artifact.type.publish","changes.capture","changes.page","changes.since"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	child, _ := os.Executable()
	f.command("extension", "register", "--manifest", manifest, "--executable", child, "--arg", "-test.run=^TestExtensionChangesChild$", "--json")
	args := []string{"extension", "enable", "example.native/tool"}
	for _, grant := range grants {
		args = append(args, "--grant", grant)
	}
	f.command(append(args, "--json")...)
	f.env = append(f.env, "HN_WP19_CHILD=1", "HN_WP19_INPUT="+f.input, "HN_WP19_OUTPUT="+f.output)
	hostBytes, err := os.ReadFile(f.binary)
	if err != nil {
		t.Fatal(err)
	}
	childBytes, err := os.ReadFile(child)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual supplied host SHA256 %x; actual raw consumer SHA256 %x", sha256.Sum256(hostBytes), sha256.Sum256(childBytes))
	return f
}
func TestExtensionChangesRawEmptyCapture(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	r := changesRun(t, f, []map[string]any{nativeRequest("changes.capture", map[string]any{"selection": map[string]any{"families": []string{"issue"}, "types": []string{}}, "limit": 20})}, nil)[0]
	data := nativeOK(t, r)
	if data["complete"] != true || data["checkpoint"] == nil || data["next_cursor"] != nil || len(data["items"].([]any)) != 0 {
		t.Fatalf("empty capture must complete: %v", data)
	}
}

// This separately executed module uses handwritten raw frames and generic JSON.
// $N.field references a previous public response; no host token internals leak
// into the consumer. Files are private test coordination, never product bypasses.
func TestExtensionChangesChild(t *testing.T) {
	if os.Getenv("HN_WP19_CHILD") != "1" {
		if control := os.Getenv("HN_WP25_CHANGES_CONTROL"); control != "" {
			changesDiagnosticControl(t, control)
		}
		return
	}
	hello := hostRead(os.Stdin)
	hostWrite(map[string]any{"protocol": "hn.extension/1", "ready": true})
	raw, e := os.ReadFile(os.Getenv("HN_WP19_INPUT"))
	if e != nil {
		os.Exit(90)
	}
	var script struct {
		Requests         []map[string]any `json:"requests"`
		Pauses           []int            `json:"pauses"`
		ConsumerState    string           `json:"consumer_state"`
		Observation      string           `json:"observation"`
		Withhold         bool             `json:"diagnostic_withhold"`
		ObservationFault string           `json:"diagnostic_observation_fault"`
	}
	if json.Unmarshal(raw, &script) != nil {
		os.Exit(91)
	}
	peer := changesDiagnosticPeer{PID: os.Getpid(), Request: -1, Exit: -1}
	observe := func(phase string) {
		peer.Phase, peer.At = phase, time.Now().UTC()
		raw, err := json.Marshal(peer)
		fault := ""
		if phase == "request-send-before" {
			fault = script.ObservationFault
		}
		class := "marshal-failure"
		if err == nil && script.Observation != "" {
			class = changesDiagnosticAtomic(script.Observation, raw, 2048, fault)
		}
		if script.Observation != "" && class != "" {
			peer.Phase, peer.Exit, peer.ObservationFailure = "observation-failure", 99, class
			raw, _ = json.Marshal(peer)
			// A failed diagnostic cannot continue the fixture. Attempt one fixed
			// terminal projection; an unwritable destination stays explicitly unknown.
			_ = changesDiagnosticAtomic(script.Observation, raw, 2048, "")
			os.Exit(99)
		}
	}
	exit := func(code int) {
		peer.Exit = code
		observe("explicit-exit")
		os.Exit(code)
	}
	observe("hello-ready")
	responses := []map[string]any{}
	var resolve func(any) any
	resolve = func(v any) any {
		switch x := v.(type) {
		case string:
			if strings.HasPrefix(x, "$") {
				parts := strings.SplitN(x[1:], ".", 2)
				n, e := strconv.Atoi(parts[0])
				if e != nil || len(parts) != 2 || n >= len(responses) {
					exit(92)
				}
				return responses[n]["data"].(map[string]any)[parts[1]]
			}
		case []any:
			for i := range x {
				x[i] = resolve(x[i])
			}
		case map[string]any:
			for k, item := range x {
				x[k] = resolve(item)
			}
		}
		return v
	}
	output := os.Getenv("HN_WP19_OUTPUT")
	durable := map[string]any{}
	if script.ConsumerState != "" {
		if b, e := os.ReadFile(script.ConsumerState); e == nil {
			if json.Unmarshal(b, &durable) != nil {
				exit(97)
			}
		}
	}
	staging := map[string]any{}
	stagingSnapshot := ""
	for n, request := range script.Requests {
		peer.Request = n
		request["request_id"] = fmt.Sprintf("change-%d", n)
		observe("request-send-before")
		hostWrite(resolve(request))
		observe("request-send-after")
		observe("response-read-before")
		response := hostRead(os.Stdin)
		observe("response-read-after")
		responses = append(responses, response)
		if script.ConsumerState != "" && strings.HasPrefix(request["method"].(string), "changes.") {
			if response["ok"] == true {
				data := response["data"].(map[string]any)
				snapshot := data["snapshot_id"].(string)
				if stagingSnapshot != snapshot {
					staging = map[string]any{}
					// A duplicate final page for an already committed identical snapshot
					// must retain its earlier pages instead of replacing them with a suffix.
					if prior, ok := durable[data["selection_digest"].(string)].(map[string]any); ok && prior["snapshot_id"] == snapshot {
						for id, entry := range prior["entries"].(map[string]any) {
							staging[id] = entry
						}
					}
					stagingSnapshot = snapshot
				}
				for _, value := range data["items"].([]any) {
					entry := value.(map[string]any)
					staging[entry["resource_id"].(string)] = entry
				}
				if data["complete"] == true && data["next_cursor"] == nil && data["checkpoint"] != nil {
					durable[data["selection_digest"].(string)] = map[string]any{"snapshot_id": snapshot, "checkpoint": data["checkpoint"], "entries": staging}
					observe("state-commit-before")
					committed, _ := json.Marshal(durable)
					if os.WriteFile(script.ConsumerState+".tmp", committed, 0600) != nil || os.Rename(script.ConsumerState+".tmp", script.ConsumerState) != nil {
						exit(98)
					}
					observe("state-commit-after")
					staging = map[string]any{}
					stagingSnapshot = ""
				}
			} else {
				staging = map[string]any{}
				stagingSnapshot = ""
			}
		}
		observe("response-publish-before")
		raw, _ = json.Marshal(response)
		if !script.Withhold && os.WriteFile(fmt.Sprintf("%s.%d", output, n), raw, 0600) != nil {
			exit(93)
		}
		if !script.Withhold {
			observe("response-publish-after")
		}
		if slicesContainsInt(script.Pauses, n) {
			observe("pause")
			deadline := time.Now().Add(11 * time.Minute)
			heartbeat := time.Now()
			count := 0
			for {
				if _, e = os.Stat(fmt.Sprintf("%s.continue-%d", output, n)); e == nil {
					observe("release")
					break
				}
				if time.Now().After(deadline) {
					exit(94)
				}
				if time.Since(heartbeat) > 20*time.Second {
					peer.Keepalive = count + 1
					observe("keepalive-send-before")
					hostWrite(map[string]any{"protocol": "hn.extension/1", "request_id": fmt.Sprintf("keepalive-%d-%d", n, count), "method": "capabilities.list", "params": map[string]any{}})
					observe("keepalive-send-after")
					observe("keepalive-read-before")
					alive := hostRead(os.Stdin)
					peer.KeepaliveOK = alive["ok"] == true
					observe("keepalive-result")
					if alive["ok"] != true {
						exit(96)
					}
					count++
					heartbeat = time.Now()
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
	observe("transcript-publish-before")
	raw, _ = json.Marshal(map[string]any{"hello": hello, "requests": script.Requests, "responses": responses})
	if os.WriteFile(output, raw, 0600) != nil {
		exit(95)
	}
	observe("transcript-publish-after")
	peer.Exit = 0
	observe("explicit-exit")
	os.Exit(0)
}
func slicesContainsInt(values []int, n int) bool {
	for _, v := range values {
		if v == n {
			return true
		}
	}
	return false
}
func changesRun(t *testing.T, f *nativeHostFixture, requests []map[string]any, after map[int]func()) []map[string]any {
	t.Helper()
	d := changesDiagnosticCall{t: t, f: f, index: -1, started: time.Now(), failedBefore: t.Failed()}
	// Installed before Start: Goexit from a synchronous callback still joins and
	// preserves the failure before testing removes the fixture's TempDirs.
	defer d.finalize()
	root := os.Getenv("HN_WP25_EVIDENCE")
	if root == "" {
		root = f.home
	}
	var err error
	d.directory, err = os.MkdirTemp(root, "changes-diagnostic-")
	if err != nil {
		d.fail("observation-directory")
	}
	pauses := []int{}
	for n := range after {
		pauses = append(pauses, n)
	}
	raw, _ := json.Marshal(map[string]any{"requests": requests, "pauses": pauses, "consumer_state": filepath.Join(f.home, "observer-state.json"), "observation": filepath.Join(d.directory, "peer.json"), "diagnostic_withhold": changesDiagnosticEnv(f.env, "HN_WP25_CHANGES_WITHHOLD") == "1", "diagnostic_observation_fault": changesDiagnosticEnv(f.env, "HN_WP25_CHANGES_OBSERVATION_FAULT")})
	if err := os.WriteFile(f.input, raw, 0600); err != nil {
		d.fail("script-write")
	}
	for n := range requests {
		os.Remove(fmt.Sprintf("%s.%d", f.output, n))
		os.Remove(fmt.Sprintf("%s.continue-%d", f.output, n))
	}
	d.command = exec.Command(f.binary, "extension", "run", "example.native/tool", "--json")
	d.command.Dir, d.command.Env = f.repo, f.env
	d.command.Stdout, d.command.Stderr = &d.stdout, &d.stderr
	d.started = time.Now()
	if err := d.command.Start(); err != nil {
		d.fail("start-failure")
	}
	d.done = make(chan struct{})
	// The sole Wait owner publishes all settlement fields with this channel.
	go func() {
		d.waitErr = d.command.Wait()
		d.settled = time.Now()
		d.waits = 1
		close(d.done)
	}()
	for n := range requests {
		d.index = n
		if callback := after[n]; callback != nil {
			deadline := time.Now().Add(15 * time.Second)
			for {
				if d.observedSettlement() {
					d.fail("early-settlement")
				}
				if _, err := os.Stat(fmt.Sprintf("%s.%d", f.output, n)); err == nil {
					break
				}
				if time.Now().After(deadline) {
					d.fail("timeout")
				}
				time.Sleep(5 * time.Millisecond)
			}
			callback()
			if d.observedSettlement() {
				d.fail("early-settlement")
			}
			if err := os.WriteFile(fmt.Sprintf("%s.continue-%d", f.output, n), []byte("continue"), 0600); err != nil {
				d.fail("continuation-write")
			}
		}
	}
	<-d.done
	d.observed = time.Now()
	if d.waitErr != nil {
		d.fail("terminal-failure")
	}
	raw, e := os.ReadFile(f.output)
	if e != nil {
		d.fail("transcript-read")
	}
	var result struct {
		Responses []map[string]any `json:"responses"`
	}
	if json.Unmarshal(raw, &result) != nil {
		d.fail("consumer-json")
	}
	d.complete = true

	if evidence := os.Getenv("HN_WP19_EVIDENCE"); evidence != "" {
		out, e := os.CreateTemp(evidence, "raw-changes-*.json")
		if e != nil {
			t.Fatal(e)
		}
		hostBytes, e := os.ReadFile(f.binary)
		if e != nil {
			t.Fatal(e)
		}
		consumer, e := os.Executable()
		if e != nil {
			t.Fatal(e)
		}
		consumerBytes, e := os.ReadFile(consumer)
		if e != nil {
			t.Fatal(e)
		}
		evidenceRaw, _ := json.Marshal(map[string]any{"test": t.Name(), "host_binary": f.binary, "host_sha256": fmt.Sprintf("%x", sha256.Sum256(hostBytes)), "consumer_sha256": fmt.Sprintf("%x", sha256.Sum256(consumerBytes)), "transcript": json.RawMessage(raw)})
		out.Write(evidenceRaw)
		out.Close()
	}
	return result.Responses
}

// Diagnostic observations contain enums and counts only. These fixed slots are
// independent of private consumer-state and the unchanged successful transcript.
type changesDiagnosticPeer struct {
	ObservationFailure string    `json:"observation_failure"`
	Phase              string    `json:"phase"`
	PID                int       `json:"pid"`
	Request            int       `json:"request"`
	Keepalive          int       `json:"keepalive"`
	KeepaliveOK        bool      `json:"keepalive_ok"`
	Exit               int       `json:"exit_intent"`
	At                 time.Time `json:"at"`
}

type changesDiagnosticCall struct {
	t                                             *testing.T
	f                                             *nativeHostFixture
	command                                       *exec.Cmd
	done                                          chan struct{}
	stdout, stderr                                bytes.Buffer
	started, settled, observed, triggered         time.Time
	waitErr                                       error
	waits, index                                  int
	directory, trigger                            string
	term, kill, complete, finalized, failedBefore bool
}

func changesDiagnosticEnv(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], key+"=") {
			return strings.TrimPrefix(env[i], key+"=")
		}
	}
	return ""
}
func (d *changesDiagnosticCall) observedSettlement() bool {
	select {
	case <-d.done:
		d.observed = time.Now()
		return true
	default:
		return false
	}
}
func (d *changesDiagnosticCall) fail(trigger string) {
	d.trigger, d.triggered = trigger, time.Now()
	d.finalize()
	d.t.Fatal("raw consumer diagnostic:", trigger)
}
func (d *changesDiagnosticCall) finalize() {
	if d.finalized {
		return
	}
	d.finalized = true
	if d.complete && d.t.Failed() == d.failedBefore {
		// The normal path has already joined the sole Wait owner.
		if err := os.RemoveAll(d.directory); err != nil {
			d.t.Error("changes diagnostic cleanup-failure")
		}
		d.t.Logf("changes diagnostic success: waits=%d joined=true", d.waits)
		return
	}
	if d.trigger == "" {
		d.trigger, d.triggered = "callback-or-incomplete", time.Now()
	}
	if d.observed.IsZero() {
		d.observed = time.Now()
	}
	if d.done != nil {
		select {
		case <-d.done:
		default:
			d.term = true
			_ = d.command.Process.Signal(syscall.SIGTERM)
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-d.done:
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				d.kill = true // NONQUALIFYING, never presented as ordinary cleanup.
				_ = d.command.Process.Kill()
				<-d.done
			}
		}
	}
	// No ProcessState or buffer access above this point: done is joined, or
	// Start failed and no Wait/buffer writer exists.
	var peer *changesDiagnosticPeer
	peerStatus := "unavailable"
	if d.directory != "" {
		peer, peerStatus = changesDiagnosticReadPeer(filepath.Join(d.directory, "peer.json"))
	}
	if peer != nil && peer.ObservationFailure != "" {
		peerStatus = "observation-write-failure"
	}
	markers := map[string]changesDiagnosticMarker{}
	for _, slot := range []struct {
		name, path string
		limit      int64
	}{
		{"previous_response", fmt.Sprintf("%s.%d", d.f.output, d.index-1), extension.FrameLimit},
		{"current_response", fmt.Sprintf("%s.%d", d.f.output, d.index), extension.FrameLimit},
		{"previous_continuation", fmt.Sprintf("%s.continue-%d", d.f.output, d.index-1), 8},
		{"current_continuation", fmt.Sprintf("%s.continue-%d", d.f.output, d.index), 8},
		{"final_transcript", d.f.output, 32 << 20},
	} {
		markers[slot.name] = changesDiagnosticInspect(slot.path, slot.limit)
	}
	exit, signal := -1, 0
	if d.done != nil && d.command.ProcessState != nil {
		exit = d.command.ProcessState.ExitCode()
		if status, ok := d.command.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			signal = int(status.Signal())
		}
	}
	record := map[string]any{
		"schema": "hn.changes-diagnostic/1", "test_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(d.t.Name()))),
		"call": filepath.Base(d.directory), "trigger": d.trigger, "request_index": d.index,
		"started_at": d.started.UTC(), "settled_at": d.settled.UTC(), "observed_at": d.observed.UTC(), "triggered_at": d.triggered.UTC(),
		"settled_elapsed_ns": changesDiagnosticElapsed(d.started, d.settled), "observed_elapsed_ns": changesDiagnosticElapsed(d.started, d.observed), "triggered_elapsed_ns": changesDiagnosticElapsed(d.started, d.triggered),
		"command_pid": changesDiagnosticPID(d.command), "waits": d.waits, "joined": d.done != nil, "exit": exit, "signal": signal, "wait_failed": d.waitErr != nil,
		"cleanup_term": d.term, "cleanup_hard_kill_NONQUALIFYING": d.kill,
		"stdout": changesDiagnosticBytes(d.stdout.Bytes()), "stderr": changesDiagnosticBytes(d.stderr.Bytes()),
		"public_terminal": changesDiagnosticPublic(d.stdout.Bytes()), "peer_status": peerStatus, "peer": peer,
		"markers": markers, "peer_terminal": "unknown",
	}
	if peer != nil && peer.Exit >= 0 {
		record["peer_terminal"] = "intent-only"
	}
	// The peer has stopped before collection; only failure.json survives.
	if d.directory != "" {
		for _, name := range []string{"peer.json", "peer.next"} {
			if err := os.Remove(filepath.Join(d.directory, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				record["diagnostic_cleanup_failure"] = true
				d.t.Error("changes diagnostic cleanup-failure")
			}
		}
	}
	raw, err := json.Marshal(record)
	if err != nil || len(raw) > 8192 {
		raw = []byte(`{"schema":"hn.changes-diagnostic/1","diagnostic_failure":"terminal-overflow"}`)
		d.t.Error("changes diagnostic terminal-overflow")
	}
	class := "observation-directory"
	if d.directory != "" {
		class = changesDiagnosticAtomic(filepath.Join(d.directory, "failure.json"), raw, 8192, "")
	}
	if class != "" && d.directory != "" {
		if err := os.Remove(filepath.Join(d.directory, "failure.next")); err != nil && !errors.Is(err, os.ErrNotExist) {
			d.t.Error("changes diagnostic temporary-cleanup-failure")
		}
	}
	if class == "" && os.Getenv("HN_WP25_EVIDENCE") != "" {
		d.t.Logf("changes diagnostic retained: call=%s trigger=%s waits=%d joined=%t", filepath.Base(d.directory), d.trigger, d.waits, d.done != nil)
	} else {
		// This bounded record contains no raw payload, message, stderr or state.
		d.t.Logf("changes diagnostic: %s", raw)
		if class != "" {
			d.t.Error("changes diagnostic publication:", class)
		}
	}
	if d.kill {
		d.t.Error("changes diagnostic cleanup hard kill NONQUALIFYING")
	}
}
func changesDiagnosticPID(command *exec.Cmd) int {
	if command == nil || command.Process == nil {
		return 0
	}
	return command.Process.Pid
}
func changesDiagnosticElapsed(start, end time.Time) int64 {
	if end.IsZero() {
		return -1
	}
	return end.Sub(start).Nanoseconds()
}
func changesDiagnosticBytes(raw []byte) map[string]any {
	return map[string]any{"bytes": len(raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
}

type changesDiagnosticMarker struct {
	Status string `json:"status"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256,omitempty"`
}

func changesDiagnosticInspect(path string, limit int64) changesDiagnosticMarker {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return changesDiagnosticMarker{Status: "absent"}
	}
	if err != nil || !info.Mode().IsRegular() {
		return changesDiagnosticMarker{Status: "unreadable"}
	}
	r := changesDiagnosticMarker{Status: "oversized", Bytes: info.Size()}
	if info.Size() > limit {
		return r
	}
	file, err := os.Open(path)
	if err != nil {
		r.Status = "unreadable"
		return r
	}
	h := sha256.New()
	var buffer [4096]byte
	n, readErr := io.CopyBuffer(h, io.LimitReader(file, limit+1), buffer[:])
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || n != info.Size() {
		r.Status = "unreadable"
		return r
	}
	if n > limit {
		return r
	}
	r.Status, r.SHA256 = "present", fmt.Sprintf("%x", h.Sum(nil))
	return r
}

// One exclusive fixed next slot, checked Write/Close/Rename, no append/history.
// fault is used only by the explicit finite controls; normal callers pass "".
func changesDiagnosticAtomic(path string, raw []byte, limit int, fault string) (class string) {
	if len(raw) > limit {
		return "record-overflow"
	}
	base := filepath.Base(path)
	if base != "peer.json" && base != "failure.json" {
		return "invalid-slot"
	}
	directory := filepath.Dir(path)
	total := len(raw)
	for _, name := range []string{"peer.json", "peer.next", "failure.json", "failure.next"} {
		info, err := os.Lstat(filepath.Join(directory, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return "slot-unreadable"
		}
		cap := int64(8192)
		if strings.HasPrefix(name, "peer.") {
			cap = 2048
		}
		if info.Size() > cap {
			return "slot-overflow"
		}
		total += int(info.Size())
	}
	if total > 16384 {
		return "total-overflow"
	}
	next := strings.TrimSuffix(path, ".json") + ".next"
	file, err := os.OpenFile(next, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "open-failure"
	}
	defer func() {
		if err := os.Remove(next); err != nil && !errors.Is(err, os.ErrNotExist) {
			class = "temporary-cleanup-failure"
		}
	}()
	if fault == "write" {
		_ = file.Close()
	}
	data := raw
	if fault == "short-write" && len(data) > 0 {
		data = data[:len(data)-1]
	}
	n, writeErr := file.Write(data)
	if fault == "close" {
		_ = file.Close()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return "write-failure"
	}
	if n != len(raw) {
		return "short-write"
	}
	if closeErr != nil {
		return "close-failure"
	}
	if fault == "rename" {
		if err = os.Mkdir(path, 0700); err != nil {
			return "control-failure"
		}
		defer os.Remove(path)
	}
	if err = os.Rename(next, path); err != nil {
		return "rename-failure"
	}
	return ""
}

// Detect duplicate keys and trailing data before decoding into closed records.
func changesDiagnosticJSON(raw []byte, dst any) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var scan func(int) bool
	scan = func(depth int) bool {
		if depth > 64 {
			return false
		}
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, e := decoder.Token()
				name, ok := key.(string)
				if e != nil || !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !scan(depth + 1) {
					return false
				}
			}
		case '[':
			for decoder.More() {
				if !scan(depth + 1) {
					return false
				}
			}
		default:
			return false
		}
		end, err := decoder.Token()
		return err == nil && ((delim == '{' && end == json.Delim('}')) || (delim == '[' && end == json.Delim(']')))
	}
	if !scan(0) {
		return false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return false
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst) == nil
}
func changesDiagnosticReadPeer(path string) (*changesDiagnosticPeer, string) {
	marker := changesDiagnosticInspect(path, 2048)
	if marker.Status != "present" {
		return nil, marker.Status
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "unreadable"
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, 2049))
	closeErr := file.Close()
	var peer changesDiagnosticPeer
	if readErr != nil || closeErr != nil || len(raw) > 2048 {
		return nil, "unreadable"
	}
	// Struct decoding alone accepts absent/null primitives as zero values.
	// Admit the complete existing wire schema before projecting typed fields.
	var fields map[string]json.RawMessage
	if !changesDiagnosticJSON(raw, &fields) || len(fields) != 8 {
		return nil, "malformed"
	}
	for _, key := range []string{"observation_failure", "phase", "pid", "request", "keepalive", "keepalive_ok", "exit_intent", "at"} {
		value, present := fields[key]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, "malformed"
		}
	}
	if !changesDiagnosticJSON(raw, &peer) {
		return nil, "malformed"
	}
	phases := map[string]bool{}
	for _, phase := range []string{"hello-ready", "request-send-before", "request-send-after", "response-read-before", "response-read-after", "state-commit-before", "state-commit-after", "response-publish-before", "response-publish-after", "pause", "release", "keepalive-send-before", "keepalive-send-after", "keepalive-read-before", "keepalive-result", "transcript-publish-before", "transcript-publish-after", "explicit-exit", "observation-failure"} {
		phases[phase] = true
	}
	if !phases[peer.Phase] || peer.PID <= 0 || peer.Request < -1 || peer.Keepalive < 0 || peer.At.IsZero() || !slicesContainsInt([]int{-1, 0, 92, 93, 94, 95, 96, 97, 98, 99}, peer.Exit) || (peer.Exit != -1 && peer.Phase != "explicit-exit" && peer.Phase != "observation-failure") {
		return nil, "malformed"
	}
	if peer.ObservationFailure != "" && (!slicesContainsString([]string{"marshal-failure", "record-overflow", "total-overflow", "slot-unreadable", "slot-overflow", "invalid-slot", "open-failure", "write-failure", "short-write", "close-failure", "rename-failure", "temporary-cleanup-failure"}, peer.ObservationFailure) || peer.Phase != "observation-failure" || peer.Exit != 99) {
		return nil, "malformed"
	}
	return &peer, "valid"
}
func changesDiagnosticPublic(raw []byte) map[string]any {
	r := map[string]any{"parse": "malformed"}
	if len(raw) == 0 {
		r["parse"] = "empty"
		return r
	}
	if len(raw) > 8192 {
		r["parse"] = "oversized"
		return r
	}
	var envelope map[string]json.RawMessage
	if !changesDiagnosticJSON(raw, &envelope) || len(envelope) != 3 || envelope["ok"] == nil || envelope["data"] == nil || envelope["error"] == nil {
		return r
	}
	var ok bool
	if string(envelope["ok"]) != "true" && string(envelope["ok"]) != "false" {
		return r
	}
	if json.Unmarshal(envelope["ok"], &ok) != nil {
		return r
	}
	if ok {
		if string(envelope["error"]) != "null" {
			return r
		}
		r["parse"], r["ok"] = "valid", true
		return r
	}
	if string(envelope["data"]) != "null" {
		return r
	}
	// Validate the actual error with the existing closed public protocol schema.
	// Only the three enum fields survive; message/operation/data never do.
	wrapped := []byte(`{"protocol":"hn.extension/1","request_id":"diagnostic","ok":false,"data":null,"error":` + string(envelope["error"]) + `}`)
	if _, err := extension.InspectMessage(wrapped, "response", "diagnostic", ""); err != nil {
		return r
	}
	var public struct {
		Origin   string `json:"origin"`
		Code     string `json:"code"`
		Recovery string `json:"recovery"`
	}
	if json.Unmarshal(envelope["error"], &public) != nil {
		return r
	}
	r["parse"], r["ok"], r["origin"], r["code"], r["recovery"] = "valid", false, public.Origin, public.Code, public.Recovery
	return r
}

// Explicit opt-in only, within the existing registered parent. Deliberate Fatal
// cases run in joined test subprocesses so the outer control can inspect evidence.
func changesDiagnosticControl(t *testing.T, control string) {
	t.Helper()
	if control == "redaction-peer" {
		fmt.Fprint(os.Stdout, `{"ok":false,"data":null,"error":{"origin":"host","code":"unavailable_module","message":"PRIVATE_SENTINEL","recovery":"do_not_retry","operation":null}}`)
		fmt.Fprint(os.Stderr, "PRIVATE_SENTINEL")
		os.Exit(1)
	}
	if control == "nonsettling-peer" {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM)
		defer signal.Stop(ch)
		if err := os.WriteFile(os.Getenv("HN_WP25_CHANGES_READY"), []byte("ready"), 0600); err != nil {
			t.Fatal("control ready")
		}
		time.Sleep(15 * time.Second)
		return
	}
	if control == "suite" {
		changesDiagnosticControls(t)
		return
	}
	if control == "storage" {
		changesDiagnosticStorageControls(t)
		return
	}
	if control == "start-failure" || control == "nonsettling" || control == "redaction" {
		f := &nativeHostFixture{t: t, binary: filepath.Join(t.TempDir(), "absent"), repo: t.TempDir(), home: t.TempDir()}
		f.input, f.output = filepath.Join(f.home, "input"), filepath.Join(f.home, "output")
		if control == "start-failure" {
			changesRun(t, f, nil, nil)
			return
		}
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		d := changesDiagnosticCall{t: t, f: f, index: 0, started: time.Now()}
		defer d.finalize()
		root := os.Getenv("HN_WP25_EVIDENCE")
		d.directory, err = os.MkdirTemp(root, "changes-diagnostic-")
		if err != nil {
			d.fail("observation-directory")
		}
		d.command = exec.Command(self, "-test.run=^TestExtensionChangesChild$", "-test.timeout=20s")
		peerControl := "nonsettling-peer"
		if control == "redaction" {
			peerControl = "redaction-peer"
			if err := os.WriteFile(filepath.Join(f.home, "observer-state.json"), []byte("PRIVATE_SENTINEL"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		d.command.Env = append(os.Environ(), "HN_WP25_CHANGES_CONTROL="+peerControl, "HN_WP25_CHANGES_READY="+f.output)
		d.command.Stdout, d.command.Stderr = &d.stdout, &d.stderr
		if err = d.command.Start(); err != nil {
			d.fail("start-failure")
		}
		d.done = make(chan struct{})
		go func() { d.waitErr = d.command.Wait(); d.settled = time.Now(); d.waits = 1; close(d.done) }()
		if control == "redaction" {
			<-d.done
			d.fail("terminal-failure")
			return
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err = os.Stat(f.output); err == nil {
				break
			}
			if time.Now().After(deadline) {
				d.fail("control-ready-timeout")
			}
			time.Sleep(5 * time.Millisecond)
		}
		d.fail("direct-nonsettling")
		return
	}
	if !slicesContainsString([]string{"resolver92", "callback-settlement", "timeout", "callback-fatal", "publication-failure", "observation-failure", "positive"}, control) {
		t.Fatal("unknown changes diagnostic control")
	}
	if os.Getenv("HN_WP17_BINARY") == "" {
		t.Fatal("diagnostic control requires supplied host")
	}
	f := changesFixture(t, "native:issue=read,observe")
	selection := changesSelection([]string{"issue"}, []string{})
	if control == "positive" {
		f.actor()
		changesOpenIssue(f, "diagnostic first")
		changesOpenIssue(f, "diagnostic second")
		var pid int
		responses := changesRun(t, f, []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 20)}, map[int]func(){0: func() {
			time.Sleep(21 * time.Second)
			peer := changesDiagnosticControlPeer(t, f)
			if peer.Keepalive < 1 || !peer.KeepaliveOK {
				t.Fatal("original20s keepalive not observed")
			}
			pid = peer.PID
		}})
		if len(responses) != 2 || len(changesIDs(t, responses[0])) != 1 || len(changesIDs(t, responses[1])) != 1 {
			t.Fatal("positive actual page results")
		}
		if len(changesConsumerSelection(t, f, nativeOK(t, responses[1]))["entries"].(map[string]any)) != 2 {
			t.Fatal("positive durable consumer state")
		}
		changesDiagnosticAbsent(t, pid)
		return
	}
	if control == "timeout" {
		f.env = append(f.env, "HN_WP25_CHANGES_WITHHOLD=1")
	}
	if control == "observation-failure" {
		f.env = append(f.env, "HN_WP25_CHANGES_OBSERVATION_FAULT=write")
	}
	if control == "callback-fatal" || control == "publication-failure" {
		changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, map[int]func(){0: func() {
			if control == "publication-failure" {
				raw, e := os.ReadFile(f.input)
				var script struct {
					Observation string `json:"observation"`
				}
				if e != nil || json.Unmarshal(raw, &script) != nil {
					t.Fatal("control observation path")
				}
				if e = os.Mkdir(filepath.Join(filepath.Dir(script.Observation), "failure.json"), 0700); e != nil {
					t.Fatal("control publication refusal")
				}
			}
			t.Fatal("deliberate callback Fatal")
		}})
		return
	}
	if control == "timeout" || control == "observation-failure" {
		changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, map[int]func(){0: func() { t.Fatal("withheld response unexpectedly published") }})
		return
	}
	// Same real supplied-host/resolver92 stimulus as the preserved OLD RED.
	changesRun(t, f, []map[string]any{
		changesCapture(selection, 20), changesPage("$9.capture", "$0.next_cursor", 20),
	}, map[int]func(){0: func() {
		raw, e := os.ReadFile(f.output + ".0")
		var response map[string]any
		if e != nil || json.Unmarshal(raw, &response) != nil {
			t.Fatal("initial actual capture absent")
		}
		nativeOK(t, response)
		t.Log("initial actual capture observed; release continue-0 toward existing resolver92")
		if control == "callback-settlement" {
			if err := os.WriteFile(f.output+".continue-0", []byte("continue"), 0600); err != nil {
				t.Fatal("control release")
			}
			time.Sleep(time.Second)
		}
	}, 1: func() { t.Fatal("resolver92 unexpectedly published response1") }})
	t.Fatal("resolver92 unexpectedly succeeded")
}
func changesDiagnosticControlPeer(t *testing.T, f *nativeHostFixture) *changesDiagnosticPeer {
	t.Helper()
	raw, err := os.ReadFile(f.input)
	var script struct {
		Observation string `json:"observation"`
	}
	if err != nil || json.Unmarshal(raw, &script) != nil {
		t.Fatal("control observation path")
	}
	peer, status := changesDiagnosticReadPeer(script.Observation)
	if status != "valid" {
		t.Fatal("control peer observation:", status)
	}
	return peer
}
func changesDiagnosticAbsent(t *testing.T, pid int) {
	t.Helper()
	// Only this call's validated, self-reported peer identity is queried. No
	// arbitrary process is signalled or discovered; signal0 is absence auditing.
	if pid <= 0 || syscall.Kill(pid, 0) != syscall.ESRCH {
		t.Fatal("owned diagnostic peer absence unverified")
	}
}
func changesDiagnosticControls(t *testing.T) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, trigger string
		waits         int
		kill          bool
	}{
		{"resolver92", "early-settlement", 1, false},
		{"callback-settlement", "early-settlement", 1, false},
		{"redaction", "terminal-failure", 1, false},
		{"timeout", "timeout", 1, false},
		{"callback-fatal", "callback-or-incomplete", 1, false},
		{"publication-failure", "callback-or-incomplete", 1, false},
		{"observation-failure", "early-settlement", 1, false},
		{"start-failure", "start-failure", 0, false},
		{"nonsettling", "direct-nonsettling", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if evidence := os.Getenv("HN_WP25_EVIDENCE"); evidence != "" {
				root, err = os.MkdirTemp(evidence, "changes-control-")
				if err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(self, "-test.run=^TestExtensionChangesChild$", "-test.v", "-test.timeout=60s")
			command.Env = append(os.Environ(), "HN_WP25_CHANGES_CONTROL="+tc.name, "HN_WP25_EVIDENCE="+root)
			var out, stderr bytes.Buffer
			command.Stdout, command.Stderr = &out, &stderr
			started := time.Now()
			if err = command.Start(); err != nil {
				t.Fatal("control Start")
			}
			pid := command.Process.Pid
			runErr := command.Wait() // This nested testing process also has one owner.
			elapsed := time.Since(started)
			if runErr == nil || command.ProcessState.ExitCode() != 1 {
				t.Fatal("deliberate failing control did not fail as expected")
			}
			entries, e := os.ReadDir(root)
			if e != nil || len(entries) != 1 {
				t.Fatal("control per-call evidence cardinality")
			}
			directory := filepath.Join(root, entries[0].Name())
			slots, e := os.ReadDir(directory)
			if e != nil || len(slots) != 1 || slots[0].Name() != "failure.json" {
				t.Fatal("control fixed final slot")
			}
			raw, e := os.ReadFile(filepath.Join(directory, "failure.json"))
			if tc.name == "publication-failure" {
				if e == nil || !strings.Contains(out.String(), "changes diagnostic publication: slot-unreadable") {
					t.Fatal("configured-path failure not explicit")
				}
				for _, line := range strings.Split(out.String(), "\n") {
					if at := strings.Index(line, "changes diagnostic: "); at >= 0 {
						raw = []byte(line[at+len("changes diagnostic: "):])
						e = nil
						break
					}
				}
			}
			if e != nil || len(raw) > 8192 {
				t.Fatal("control failure record")
			}
			var record map[string]any
			if !changesDiagnosticJSON(raw, &record) || record["trigger"] != tc.trigger || record["waits"] != float64(tc.waits) || record["joined"] != (tc.waits == 1) || record["cleanup_hard_kill_NONQUALIFYING"] != tc.kill {
				t.Fatal("control trigger/Wait/cleanup contract")
			}
			if strings.Contains(string(raw), "raw consumer:") || strings.Contains(string(raw), "PRIVATE_SENTINEL") {
				t.Fatal("raw failure output leaked")
			}
			if peer, ok := record["peer"].(map[string]any); ok {
				changesDiagnosticAbsent(t, int(peer["pid"].(float64)))
			} else if tc.name != "start-failure" && tc.name != "nonsettling" && tc.name != "observation-failure" && tc.name != "redaction" {
				t.Fatal("control owned peer not observed")
			}
			if tc.name == "resolver92" || tc.name == "callback-settlement" {
				public := record["public_terminal"].(map[string]any)
				peer := record["peer"].(map[string]any)
				if public["parse"] != "valid" || public["origin"] != "host" || public["code"] != "unavailable_module" || public["recovery"] != "do_not_retry" || peer["exit_intent"] != float64(92) || record["exit"] != float64(1) || record["settled_elapsed_ns"].(float64) <= 0 || record["observed_elapsed_ns"].(float64) < record["settled_elapsed_ns"].(float64) {
					t.Fatal("resolver92 precise joined public failure")
				}
				if elapsed >= 15*time.Second {
					t.Fatal("early settlement waited old15s deadline")
				}
			}
			if tc.name == "callback-settlement" && (record["request_index"] != float64(0) || record["observed_elapsed_ns"].(float64)-record["settled_elapsed_ns"].(float64) < 500000000) {
				t.Fatal("synchronous callback settlement/observation clocks")
			}
			if tc.name == "redaction" {
				public := record["public_terminal"].(map[string]any)
				if public["parse"] != "valid" || public["code"] != "unavailable_module" || record["stderr"].(map[string]any)["bytes"] != float64(len("PRIVATE_SENTINEL")) {
					t.Fatal("joined actual output privacy projection")
				}
				changesDiagnosticAbsent(t, int(record["command_pid"].(float64)))
			}
			if tc.name == "timeout" && (elapsed < 15*time.Second || record["cleanup_term"] != true || record["triggered_elapsed_ns"].(float64) >= record["settled_elapsed_ns"].(float64)) {
				t.Fatal("timeout/cleanup clocks conflated")
			}
			if tc.name == "observation-failure" && record["peer_status"] != "observation-write-failure" {
				t.Fatal("observation failure not explicit")
			}
			if tc.name == "nonsettling" {
				changesDiagnosticAbsent(t, int(record["command_pid"].(float64)))
			}
			if tc.name == "publication-failure" {
				t.Logf("control publication-failure fallback record: %s", raw)
			}
			t.Logf("control=%s nested_pid=%d nested_wait=1 exit=1 elapsed_ns=%d record_bytes=%d record_sha256=%x stdout_sha256=%x stderr_sha256=%x", tc.name, pid, elapsed.Nanoseconds(), len(raw), sha256.Sum256(raw), sha256.Sum256(out.Bytes()), sha256.Sum256(stderr.Bytes()))
		})
	}
	t.Run("positive", func(t *testing.T) { changesDiagnosticControl(t, "positive") })
	t.Run("storage", changesDiagnosticStorageControls)
}
func changesDiagnosticStorageControls(t *testing.T) {
	t.Helper()
	for _, tc := range []struct{ fault, want string }{{"write", "write-failure"}, {"short-write", "short-write"}, {"close", "close-failure"}} {
		t.Run(tc.fault, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "peer.json")
			if got := changesDiagnosticAtomic(path, []byte(`{}`), 2048, tc.fault); got != tc.want {
				t.Fatal("atomic checked fault:", got)
			}
			if entries, e := os.ReadDir(filepath.Dir(path)); e != nil || len(entries) != 0 {
				t.Fatal("atomic fault left partial slot")
			}
		})
	}
	t.Run("rename", func(t *testing.T) {
		root := t.TempDir()
		// Exercise an actual checked rename failure after a successful close.
		// The destination is introduced by the fault control after slot admission.
		if got := changesDiagnosticAtomic(filepath.Join(root, "failure.json"), []byte(`{}`), 8192, "rename"); got != "rename-failure" {
			t.Fatal("rename fault:", got)
		}
	})
	t.Run("bounds", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			limit int
		}{{"peer.json", 2048}, {"failure.json", 8192}} {
			path := filepath.Join(t.TempDir(), tc.name)
			if got := changesDiagnosticAtomic(path, bytes.Repeat([]byte("x"), tc.limit), tc.limit, ""); got != "" {
				t.Fatal("at bound", got)
			}
			if got := changesDiagnosticAtomic(path, bytes.Repeat([]byte("x"), tc.limit+1), tc.limit, ""); got != "record-overflow" {
				t.Fatal("over bound", got)
			}
			info, e := os.Stat(path)
			if e != nil || info.Mode().Perm() != 0600 || info.Size() != int64(tc.limit) {
				t.Fatal("bound/mode changed")
			}
		}
		root := t.TempDir()
		for _, slot := range []struct {
			name string
			size int
		}{{"peer.json", 2048}, {"peer.next", 2048}, {"failure.next", 8192}} {
			if err := os.WriteFile(filepath.Join(root, slot.name), bytes.Repeat([]byte("x"), slot.size), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if got := changesDiagnosticAtomic(filepath.Join(root, "failure.json"), bytes.Repeat([]byte("x"), 8192), 8192, ""); got != "total-overflow" {
			t.Fatal("total overflow", got)
		}
	})
	t.Run("strict-phase", func(t *testing.T) {
		peer := changesDiagnosticPeer{Phase: "pause", PID: os.Getpid(), Request: 0, Exit: -1, At: time.Now().UTC()}
		raw, _ := json.Marshal(peer)
		for _, wire := range [][]byte{append(raw, []byte(` {}`)...), bytes.Replace(raw, []byte(`"phase":"pause"`), []byte(`"phase":"pause","phase":"pause"`), 1), []byte(`{"phase":"PRIVATE_SENTINEL"}`), bytes.Repeat([]byte("x"), 2049)} {
			path := filepath.Join(t.TempDir(), "peer.json")
			if e := os.WriteFile(path, wire, 0600); e != nil {
				t.Fatal(e)
			}
			if peer, status := changesDiagnosticReadPeer(path); peer != nil || (status != "malformed" && status != "oversized") {
				t.Fatal("invalid phase accepted")
			}
		}
	})
	t.Run("required-fields", func(t *testing.T) {
		keys := []string{"observation_failure", "phase", "pid", "request", "keepalive", "keepalive_ok", "exit_intent", "at"}
		wrongTypes := map[string]json.RawMessage{
			"observation_failure": json.RawMessage(`0`), "phase": json.RawMessage(`0`),
			"pid": json.RawMessage(`"1"`), "request": json.RawMessage(`"0"`),
			"keepalive": json.RawMessage(`"0"`), "exit_intent": json.RawMessage(`"0"`),
			"keepalive_ok": json.RawMessage(`"false"`), "at": json.RawMessage(`0`),
		}
		for _, phase := range []string{"pause", "explicit-exit", "observation-failure"} {
			t.Run(phase, func(t *testing.T) {
				complete := changesDiagnosticPeer{Phase: phase, PID: os.Getpid(), Request: 0, Exit: -1, At: time.Now().UTC()}
				if phase == "explicit-exit" {
					complete.Exit = 0
				}
				if phase == "observation-failure" {
					complete.Exit, complete.ObservationFailure = 99, "write-failure"
				}
				raw, err := json.Marshal(complete)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "peer.json")
				check := func(t *testing.T, wire []byte, valid bool) {
					t.Helper()
					if err := os.WriteFile(path, wire, 0600); err != nil {
						t.Fatal(err)
					}
					peer, status := changesDiagnosticReadPeer(path)
					if valid {
						if status != "valid" || peer == nil || !reflect.DeepEqual(*peer, complete) {
							t.Fatal("complete peer changed")
						}
					} else if peer != nil || status != "malformed" {
						t.Fatal("incomplete or incorrectly typed peer accepted")
					}
				}
				t.Run("complete", func(t *testing.T) { check(t, raw, true) })
				for _, key := range keys {
					for _, mutation := range []string{"delete", "null", "wrong-type"} {
						t.Run(key+"/"+mutation, func(t *testing.T) {
							var fields map[string]json.RawMessage
							if err := json.Unmarshal(raw, &fields); err != nil {
								t.Fatal(err)
							}
							switch mutation {
							case "delete":
								delete(fields, key)
							case "null":
								fields[key] = json.RawMessage(`null`)
							case "wrong-type":
								fields[key] = wrongTypes[key]
							}
							wire, err := json.Marshal(fields)
							if err != nil {
								t.Fatal(err)
							}
							check(t, wire, false)
						})
					}
				}
				for _, key := range []string{"pid", "request", "keepalive", "exit_intent"} {
					t.Run(key+"/fractional-type", func(t *testing.T) {
						var fields map[string]json.RawMessage
						if err := json.Unmarshal(raw, &fields); err != nil {
							t.Fatal(err)
						}
						fields[key] = json.RawMessage(`0.5`)
						wire, err := json.Marshal(fields)
						if err != nil {
							t.Fatal(err)
						}
						check(t, wire, false)
					})
				}
				for _, value := range []string{`"invalid"`, `"0001-01-01T00:00:00Z"`} {
					t.Run("invalid-time/"+value, func(t *testing.T) {
						var fields map[string]json.RawMessage
						if err := json.Unmarshal(raw, &fields); err != nil {
							t.Fatal(err)
						}
						fields["at"] = json.RawMessage(value)
						wire, err := json.Marshal(fields)
						if err != nil {
							t.Fatal(err)
						}
						check(t, wire, false)
					})
				}
			})
		}
	})
	t.Run("public-redaction", func(t *testing.T) {
		wire := []byte(`{"ok":false,"data":null,"error":{"origin":"host","code":"unavailable_module","message":"PRIVATE_SENTINEL","recovery":"do_not_retry","operation":null}}`)
		safe := changesDiagnosticPublic(wire)
		raw, _ := json.Marshal(safe)
		if safe["parse"] != "valid" || safe["code"] != "unavailable_module" || bytes.Contains(raw, []byte("PRIVATE_SENTINEL")) {
			t.Fatal("public projection/redaction")
		}
		for _, bad := range [][]byte{nil, []byte("PRIVATE_SENTINEL"), append(wire, []byte(` {}`)...), bytes.Replace(wire, []byte(`"ok":false`), []byte(`"ok":true,"ok":false`), 1), bytes.Replace(wire, []byte("unavailable_module"), []byte("PRIVATE_SENTINEL"), 1), bytes.Repeat([]byte("x"), 8193)} {
			safe := changesDiagnosticPublic(bad)
			raw, _ := json.Marshal(safe)
			if safe["parse"] == "valid" || bytes.Contains(raw, []byte("PRIVATE_SENTINEL")) {
				t.Fatal("malformed public projection")
			}
		}
		// Markers retain a digest and count, never raw private bytes.
		path := filepath.Join(t.TempDir(), "marker")
		if err := os.WriteFile(path, []byte("PRIVATE_SENTINEL"), 0600); err != nil {
			t.Fatal(err)
		}
		marker := changesDiagnosticInspect(path, 64)
		raw, _ = json.Marshal(marker)
		if marker.Status != "present" || bytes.Contains(raw, []byte("PRIVATE_SENTINEL")) || changesDiagnosticInspect(path, 1).Status != "oversized" {
			t.Fatal("marker bound/redaction")
		}
	})
}

func changesSelection(families, types []string) map[string]any {
	return map[string]any{"families": families, "types": types}
}
func changesCapture(selection map[string]any, limit int) map[string]any {
	return nativeRequest("changes.capture", map[string]any{"selection": selection, "limit": limit})
}
func changesPage(capture, cursor any, limit int) map[string]any {
	return nativeRequest("changes.page", map[string]any{"capture": capture, "cursor": cursor, "limit": limit})
}
func changesSince(checkpoint any, selection map[string]any, limit int) map[string]any {
	return nativeRequest("changes.since", map[string]any{"checkpoint": checkpoint, "selection": selection, "limit": limit})
}
func changesIDs(t *testing.T, response map[string]any) []string {
	t.Helper()
	data := nativeOK(t, response)
	out := []string{}
	for _, entry := range data["items"].([]any) {
		out = append(out, entry.(map[string]any)["resource_id"].(string))
	}
	return out
}
func changesRefuse(t *testing.T, response map[string]any, code string) {
	t.Helper()
	nativeDenial(t, response, "host", code, map[string]string{"checkpoint_invalidated": "rescan_required", "verification_failure": "do_not_retry", "denied_access": "do_not_retry", "unsupported_capability": "do_not_retry", "resource_limit": "do_not_retry", "invalid_input": "do_not_retry"}[code])
}
func changesOpenIssue(f *nativeHostFixture, title string) string {
	f.t.Helper()
	return betaRecoveryID(f.t, f.native("issue", "open", "--json", title))
}
func TestExtensionChangesPagingAndDurableNativeAppend(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	ids := []string{changesOpenIssue(f, "first"), changesOpenIssue(f, "second"), changesOpenIssue(f, "third")}
	sort.Strings(ids)
	selection := changesSelection([]string{"issue"}, []string{})
	requests := []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 1), changesPage("$0.capture", "$0.next_cursor", 1), changesPage("$0.capture", "$0.next_cursor", 200), changesPage("$0.capture", "$0.next_cursor", 200)}
	responses := changesRun(t, f, requests, nil)
	first, second, duplicate, final := nativeOK(t, responses[0]), nativeOK(t, responses[1]), nativeOK(t, responses[2]), nativeOK(t, responses[3])
	if !reflect.DeepEqual(second, duplicate) {
		t.Fatal("duplicate page changed", second, duplicate)
	}
	if !reflect.DeepEqual(final, nativeOK(t, responses[4])) {
		t.Fatal("duplicate final page changed")
	}
	if len(changesConsumerSelection(t, f, final)["entries"].(map[string]any)) != 3 {
		t.Fatal("duplicate final page lost previously committed page entries")
	}

	for _, page := range []map[string]any{first, second, final} {
		if page["snapshot_id"] != first["snapshot_id"] || page["source_cut"] != first["source_cut"] || page["selection_digest"] != first["selection_digest"] {
			t.Fatal("mixed capture")
		}
	}
	if first["checkpoint"] != nil || first["complete"] != false || second["checkpoint"] != nil || final["checkpoint"] == nil || final["complete"] != true || final["next_cursor"] != nil {
		t.Fatal("completion contract", responses)
	}
	got := append(changesIDs(t, responses[0]), changesIDs(t, responses[3])...)
	if !reflect.DeepEqual(got, ids) {
		t.Fatal("variable limit skipped", got, ids)
	}
	// A distinct host session cannot resume a cursor, but a completed checkpoint
	// remains usable after an actual local append verified by the native reader.
	newID := changesOpenIssue(f, "append after restart")
	next := changesRun(t, f, []map[string]any{changesPage(first["capture"], first["next_cursor"], 20), changesSince(final["checkpoint"], selection, 200)}, nil)
	changesRefuse(t, next[0], "checkpoint_invalidated")
	appended := changesIDs(t, next[1])
	if len(appended) != 4 || !slicesContainsString(appended, newID) {
		t.Fatal("since must replace entire current set", next)
	}
	if nativeOK(t, next[1])["snapshot_id"] == first["snapshot_id"] {
		t.Fatal("append did not change whole snapshot")
	}
}
func slicesContainsString(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}
func TestExtensionChangesStalePageAndSafeRescan(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	changesOpenIssue(f, "original one")
	changesOpenIssue(f, "original two")
	selection := changesSelection([]string{"issue"}, []string{})
	committed := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	checkpoint := nativeOK(t, committed)["checkpoint"]
	prior := changesIDs(t, committed)
	priorState, err := os.ReadFile(filepath.Join(f.home, "observer-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	preserved := func() {
		current, e := os.ReadFile(filepath.Join(f.home, "observer-state.json"))
		if e != nil || !bytes.Equal(priorState, current) {
			t.Fatal("interrupted raw observer changed its durable visible set/checkpoint", e)
		}
	}
	responses := changesRun(t, f, []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 200), changesSince(checkpoint, selection, 200)}, map[int]func(){0: func() { preserved(); changesOpenIssue(f, "interleaved append") }, 1: preserved})
	changesRefuse(t, responses[1], "checkpoint_invalidated")
	if nativeOK(t, responses[0])["checkpoint"] != nil {
		t.Fatal("partial capture advanced checkpoint")
	}
	if len(prior) != 2 || checkpoint == nil {
		t.Fatal("previous committed set lost")
	}
	if len(changesIDs(t, responses[2])) != 3 {
		t.Fatal("rescan did not reconcile complete replacement")
	}
	state := changesConsumerSelection(t, f, nativeOK(t, responses[2]))
	if len(state["entries"].(map[string]any)) != 3 || state["checkpoint"] != nativeOK(t, responses[2])["checkpoint"] {
		t.Fatal("observer did not atomically persist completed replacement", state)
	}
}
func TestExtensionChangesCustomAndMixedAppend(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe", "native:project=read,observe", "artifact:example.test/*=read,observe")
	actor := f.actor()
	a := &artifactCLIFixture{t: t, binary: f.binary, repo: f.repo, home: f.home, actor: actor}
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "first custom")), "record-one", ""))
	issue := changesOpenIssue(f, "native alongside custom")
	a.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", "example.unselected/note"), "unselected-type", "")
	selection := changesSelection([]string{"issue", "project"}, []string{"example.test/note"})
	first := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	if got := changesIDs(t, first); len(got) != 2 || !slicesContainsString(got, opening) || !slicesContainsString(got, issue) {
		t.Fatal("mixed capture", first)
	}
	changesAssertMixedCut(t, f, nativeOK(t, first), selection, []string{schema, opening, issue})
	revision := revisionEvent(revisionCall(a, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "edited custom")), "record-revise", ""))
	second := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, first)["checkpoint"], selection, 200)}, nil)[0]
	if len(changesIDs(t, second)) != 2 {
		t.Fatal("custom append sparse/absent", second)
	}
	found := false
	for _, raw := range nativeOK(t, second)["items"].([]any) {
		entry := raw.(map[string]any)
		if entry["resource_id"] == opening {
			found = true
			if !reflect.DeepEqual(entry["revision_ids"], []any{revision}) {
				t.Fatal("custom revision identity", entry)
			}
		}
	}
	if !found {
		t.Fatal("missing custom opening")
	}
	// An exact selection cannot borrow a checkpoint from the mixed view.
	wrong := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, second)["checkpoint"], changesSelection([]string{"issue"}, []string{}), 200)}, nil)[0]
	changesRefuse(t, wrong, "checkpoint_invalidated")
}

func TestExtensionChangesAuthorizationAndVerifierLoss(t *testing.T) {
	selection := changesSelection([]string{"issue"}, []string{})
	for _, grant := range []string{"native:issue=read", "native:issue=observe"} {
		t.Run(grant, func(t *testing.T) {
			f := changesFixture(t, grant)
			before := hostNativeSnapshot(t, f.repo)
			r := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
			changesRefuse(t, r, "denied_access")
			if _, err := os.Stat(filepath.Join(f.repo, ".git", "hn", "extensions", "changes")); !os.IsNotExist(err) {
				t.Fatal("ungranted read initialized verifier")
			}
			if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
				t.Fatal("ungranted changes modified protected state")
			}
		})
	}
	f := changesFixture(t, "native:issue=read,observe")
	first := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
	checkpoint := nativeOK(t, first)["checkpoint"].(string)
	// A forged token cannot turn an incomplete or unrelated checkpoint into a view.
	forged := checkpoint[:128] + map[bool]string{true: "0", false: "1"}[checkpoint[128] != '0']
	responses := changesRun(t, f, []map[string]any{changesSince(forged, selection, 20), changesSince(strings.Repeat("x", 4096), selection, 20)}, nil)
	for _, r := range responses {
		changesRefuse(t, r, "checkpoint_invalidated")
	}
	// Re-enablement is an independent new epoch even with identical grants and
	// the same intact verifier. Then test missing verifier using a fresh checkpoint.
	f.command("extension", "enable", "example.native/tool", "--grant", "native:issue=read,observe", "--json")
	changesRefuse(t, changesRun(t, f, []map[string]any{changesSince(checkpoint, selection, 20)}, nil)[0], "checkpoint_invalidated")
	fresh := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
	checkpoint = nativeOK(t, fresh)["checkpoint"].(string)
	key := filepath.Join(f.repo, ".git", "hn", "extensions", "changes", "key")
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	before := hostNativeSnapshot(t, f.repo)
	changesRefuse(t, changesRun(t, f, []map[string]any{changesSince(checkpoint, selection, 20)}, nil)[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("since repaired missing verifier instead of invalidating")
	}

}
func TestExtensionChangesUnsupportedCustomPolicyBeforeState(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe", "artifact:example.test/*=read,observe")
	actor := f.actor()
	f.policy(actor, []c.Rule{{Operation: c.IssueCreate, Automatic: true}})
	before := hostNativeSnapshot(t, f.repo)
	r := changesRun(t, f, []map[string]any{changesCapture(changesSelection([]string{"issue"}, []string{"example.test/note"}), 20)}, nil)[0]
	changesRefuse(t, r, "unsupported_capability")
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("unsupported selected custom policy initialized state or modified history")
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git", "hn", "extensions", "changes")); !os.IsNotExist(err) {
		t.Fatal("unsupported custom view initialized verifier")
	}
}
func TestExtensionChangesInvalidHistoryAndQuarantine(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	actor := f.actor()
	visible := changesOpenIssue(f, "accepted body")
	head := f.git("rev-parse", "refs/hn/actors/"+actor)
	// A syntactically plausible raw quarantine ref is not an accepted feed.
	f.git("update-ref", "refs/hn/quarantine/not-an-admission/actors/"+actor, head)
	selection := changesSelection([]string{"issue"}, []string{})
	first := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
	if !reflect.DeepEqual(changesIDs(t, first), []string{visible}) {
		t.Fatal("quarantine duplicated/promoted", first)
	}
	// A normal Git commit under an accepted actor ref is deliberately invalid
	// signed history. Capture must refuse, never return an apparently empty set.
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "not a signed event")
	f.git("update-ref", "refs/hn/actors/"+actor, f.git("rev-parse", "HEAD"))
	before := hostNativeSnapshot(t, f.repo)
	response := changesRun(t, f, []map[string]any{changesCapture(selection, 20), changesSince(nativeOK(t, first)["checkpoint"], selection, 20)}, nil)
	for _, r := range response {
		changesRefuse(t, r, "verification_failure")
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("failed verification modified history/private state")
	}
}
func TestExtensionChangesGovernedTwoActorVisibility(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	actor := f.actor()
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "Two actor fixture base")
	other := *f
	other.repo = t.TempDir()
	f.git("clone", "-q", "--no-local", f.repo, other.repo)
	secondActor := other.actor()
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor, secondActor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: eventID([]byte("native-host-policy")), Moderators: []string{actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}, {Operation: c.IssueReply, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policyRaw, _ := json.Marshal(policy)
	policyPath := filepath.Join(f.repo, ".hn", "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, policyRaw, 0600); err != nil {
		t.Fatal(err)
	}
	f.git("add", ".hn/policy.json")
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Select two actor contribution policy")
	f.native("contribution", "authority", "set", "--ref", f.git("symbolic-ref", "HEAD"), "--expect", "absent")
	other.git("fetch", "-q", f.repo, f.git("symbolic-ref", "HEAD"))
	other.git("reset", "--hard", "FETCH_HEAD")
	other.native("contribution", "authority", "set", "--ref", f.git("symbolic-ref", "HEAD"), "--expect", "absent")
	ownID := changesOpenIssue(f, "Visible first actor")
	hiddenTitle := "PRIVATE-HIDDEN-WP19 second actor body"
	otherID := changesOpenIssue(&other, hiddenTitle)
	f.git("fetch", "-q", other.repo, "refs/hn/actors/"+secondActor+":refs/hn/actors/"+secondActor)
	var project string
	sum := sha256.Sum256([]byte("native-host-policy"))
	project = "sha256:" + hex.EncodeToString(sum[:])
	manifestRef := "refs/hn/contributions/" + project[7:] + "/" + secondActor
	f.git("fetch", "-q", other.repo, manifestRef+":"+manifestRef)
	var retained struct {
		Digest string `json:"digest"`
	}
	if json.Unmarshal([]byte(f.native("contribution", "retained", "show", "--json")), &retained) != nil || retained.Digest == "" {
		t.Fatal("retained inspection")
	}
	f.native("contribution", "retained", "register", "--expect", retained.Digest, "--json")
	selection := changesSelection([]string{"issue"}, []string{})
	initial := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	if got := changesIDs(t, initial); len(got) != 2 || !slicesContainsString(got, ownID) || !slicesContainsString(got, otherID) {
		t.Fatal("two governed actor views", initial)
	}
	var hidden map[string]any
	if json.Unmarshal([]byte(f.native("contribution", "decide", "hide", "--revision", otherID, "--expect-heads", "absent", "--json")), &hidden) != nil {
		t.Fatal("hide JSON")
	}
	if !strings.Contains(f.git("show", "refs/hn/actors/"+secondActor+":event.json"), hiddenTitle) {
		t.Fatal("raw hidden-event detector failed")
	}
	afterHide := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, initial)["checkpoint"], selection, 200), changesCapture(selection, 200)}, nil)
	changesRefuse(t, afterHide[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(changesIDs(t, afterHide[1]), []string{ownID}) {
		t.Fatal("hidden material entered normal feed", afterHide)
	}
	wire, _ := json.Marshal(afterHide)
	if bytes.Contains(wire, []byte(hiddenTitle)) || bytes.Contains(wire, []byte(otherID)) {
		t.Fatal("hidden raw identity/body leak")
	}
	f.native("contribution", "decide", "restore", "--revision", otherID, "--expect-heads", hidden["id"].(string), "--json")
	restored := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	if len(changesIDs(t, restored)) != 2 {
		t.Fatal("restore without edit omitted resource")
	}
	f.native("contribution", "decide", "block", "--actor", secondActor, "--expect-heads", "absent", "--json")
	blocked := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, restored)["checkpoint"], selection, 200), changesCapture(selection, 200)}, nil)
	changesRefuse(t, blocked[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(changesIDs(t, blocked[1]), []string{ownID}) {
		t.Fatal("blocked resource visible", blocked)
	}
	// An authority-only public policy edit invalidates the old checkpoint even
	// though neither resource's signed revision changed.
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "Authority-only checkpoint transition")
	authority := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, blocked[1])["checkpoint"], selection, 200)}, nil)[0]
	changesRefuse(t, authority, "checkpoint_invalidated")
}

// Source-assisted fixture producer: signs real hn/0 events and writes their
// public Git commit trees with fast-import. All observations below are actual
// external raw host requests against those complete verified actor chains.
// This producer grants no product clock/limit bypass and does not touch private
// capture/checkpoint files.
func changesBulkProducer(t *testing.T, f *nativeHostFixture, bodies []string, comments ...string) ([]string, []map[string]any) {
	t.Helper()
	seed := bytes.Repeat([]byte{39}, 32)
	key := ed25519.NewKeyFromSeed(seed)
	public := key.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(public)
	identity := &Identity{Actor: hex.EncodeToString(sum[:]), Name: "WP19 source-assisted producer", PublicKey: base64.RawStdEncoding.EncodeToString(public), PrivateKey: base64.RawStdEncoding.EncodeToString(key)}
	ref := "refs/hn/actors/" + identity.Actor
	var stream bytes.Buffer
	fmt.Fprintf(&stream, "reset %s\nfrom %040d\n\n", ref, 0)
	previous := ""
	ids := []string{}
	entries := []map[string]any{}
	write := func(event Event) string {
		raw, signature, err := encodeAndSign(event, identity)
		if err != nil {
			t.Fatal(err)
		}
		id := eventID(raw)
		message := "hn event " + id
		fmt.Fprintf(&stream, "commit %s\nmark :%d\ncommitter Fixture <fixture@hn.invalid> 1793433600 +0000\ndata %d\n%s\n", ref, event.Sequence, len(message), message)
		if event.Sequence > 1 {
			fmt.Fprintf(&stream, "from :%d\n", event.Sequence-1)
		}
		fmt.Fprintf(&stream, "deleteall\nM 100644 inline event.json\ndata %d\n", len(raw))
		stream.Write(raw)
		stream.WriteByte('\n')
		sig := base64.RawStdEncoding.EncodeToString(signature)
		fmt.Fprintf(&stream, "M 100644 inline signature\ndata %d\n%s\n\n", len(sig), sig)
		previous = id
		return id
	}
	for n, body := range bodies {
		event := newEvent(identity, "issue.open", uint64(n+1), previous)
		event.Timestamp = "2026-10-31T08:00:00Z"
		event.Title = fmt.Sprintf("Bulk%05d", n)
		event.Body = body
		id := write(event)
		ids = append(ids, id)
		state := map[string]any{"title": event.Title, "body": body, "status": "open", "criteria": []any{}, "labels": []any{}, "assignees": []any{}, "relations": []any{}, "metadata": map[string]any{}}
		value := map[string]any{"protocol": "hn/0", "family": "issue", "id": id, "creator": identity.Actor, "heads": []any{map[string]any{"id": id, "actor": identity.Actor, "state": state}}, "conflict": false, "state": state, "comments": []any{}}
		entries = append(entries, map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": id, "revision_ids": []string{id}, "value": value})
	}
	sequence := len(bodies)
	for _, comment := range comments {
		if comment == "" {
			continue
		}
		sequence++
		event := newEvent(identity, "issue.comment", uint64(sequence), previous)
		event.Timestamp = "2026-10-31T08:00:00Z"
		event.Subject = ids[0]
		event.Body = comment
		id := write(event)
		value := entries[0]["value"].(map[string]any)
		value["comments"] = append(value["comments"].([]any), map[string]any{"id": id, "actor": identity.Actor, "body": comment})
		revisions := append(entries[0]["revision_ids"].([]string), id)
		sort.Strings(revisions)
		entries[0]["revision_ids"] = revisions
	}
	stream.WriteString("done\n")
	cmd := exec.Command("git", "fast-import", "--quiet", "--done", "--force")
	cmd.Dir = f.repo
	cmd.Env = f.env
	cmd.Stdin = &stream
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source-assisted fast-import: %v %s", err, raw)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i]["resource_id"].(string) < entries[j]["resource_id"].(string) })
	return ids, entries
}
func TestExtensionChangesRawEntryCapacity(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	bodies := make([]string, 10000)
	ids, expected := changesBulkProducer(t, f, bodies, "")
	expectedRaw, _ := json.Marshal(expected)
	t.Logf("source-assisted verified entry fixture:10000 entries C bytes=%d", len(expectedRaw))
	selection := changesSelection([]string{"issue"}, []string{})
	requests := []map[string]any{changesCapture(selection, 200)}
	for n := 1; n < 50; n++ {
		requests = append(requests, changesPage("$0.capture", fmt.Sprintf("$%d.next_cursor", n-1), 200))
	}
	responses := changesRun(t, f, requests, nil)
	seen := map[string]bool{}
	all := []any{}
	first := nativeOK(t, responses[0])
	for n, response := range responses {
		data := nativeOK(t, response)
		for _, entry := range data["items"].([]any) {
			id := entry.(map[string]any)["resource_id"].(string)
			if seen[id] {
				t.Fatal("duplicate/skipped capacity page", id)
			}
			seen[id] = true
			all = append(all, entry)
		}
		if data["snapshot_id"] != first["snapshot_id"] || data["source_cut"] != first["source_cut"] {
			t.Fatal("capacity page mixed cut")
		}
		if n < 49 && (data["complete"] != false || data["checkpoint"] != nil) {
			t.Fatal("early capacity checkpoint")
		}
	}
	if len(seen) != len(ids) || nativeOK(t, responses[49])["complete"] != true || nativeOK(t, responses[49])["checkpoint"] == nil {
		t.Fatal("10000-entry replacement not complete", len(seen))
	}
	actualRaw, _ := json.Marshal(all)
	if !bytes.Equal(actualRaw, expectedRaw) {
		t.Fatal("complete array differs from independently assembled verified fixture")
	}
	wantSnapshot := independentNativeHash(map[string]any{"source_cut": first["source_cut"], "selection_digest": first["selection_digest"], "entries": all})
	if first["snapshot_id"] != wantSnapshot {
		t.Fatal("snapshot ID omitted whole entry set")
	}
	bodies = append(bodies, "")
	changesBulkProducer(t, f, bodies, "")
	before := hostNativeSnapshot(t, f.repo)
	refused := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	changesRefuse(t, refused, "resource_limit")
	message := refused["error"].(map[string]any)["message"].(string)
	if !strings.Contains(message, "selections") || strings.Contains(message, "smaller page") {
		t.Fatal("capture capacity gives false page guidance", message)
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("10001-entry capture published private state")
	}
}
func TestExtensionChangesRawCanonicalCapacity(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	//128*two complete states leave a small adjustable tail below16MiB.
	bodies := make([]string, 128)
	for i := range bodies {
		bodies[i] = strings.Repeat("x", 64000)
	}
	_, entries := changesBulkProducer(t, f, bodies, "")
	base, _ := json.Marshal(entries)
	// A visible comment adds a single-copy text field, allowing exact +1 bytes
	// while each individual state/comment remains within its own native bounds.
	_, entries = changesBulkProducer(t, f, bodies, "a")
	withComment, _ := json.Marshal(entries)
	remaining := 16777216 - len(withComment)
	// Increase body bytes in pairs first, leaving one comment for odd precision.
	for n := 0; n < len(bodies) && remaining > 65534; n++ {
		add := min(1536, remaining/2)
		bodies[n] += strings.Repeat("x", add)
		remaining -= 2 * add
	}
	if remaining < 0 || remaining > 65534 {
		t.Fatalf("fixture adjustment cannot reach exact capacity:base=%d tail=%d", len(base), remaining)
	}
	comment := strings.Repeat("c", remaining+1)
	_, entries = changesBulkProducer(t, f, bodies, comment)
	exact, _ := json.Marshal(entries)
	if len(exact) != 16777216 {
		t.Fatal("source-assisted at-bound C length", len(exact))
	}
	t.Logf("source-assisted verified full C array at=%d over=%d", len(exact), len(exact)+1)
	selection := changesSelection([]string{"issue"}, []string{})
	r := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	if nativeOK(t, r)["complete"] != false || len(changesIDs(t, r)) != 1 {
		t.Fatal("16MiB whole capture refused/truncated")
	}
	changesBulkProducer(t, f, bodies, comment+"c")
	before := hostNativeSnapshot(t, f.repo)
	refused := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	changesRefuse(t, refused, "resource_limit")
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("over-capacity initialized new state")
	}
}

func TestExtensionChangesRealTenMinuteExpiry(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	changesOpenIssue(f, "expiry one")
	changesOpenIssue(f, "expiry two")
	selection := changesSelection([]string{"issue"}, []string{})
	var expires time.Time
	var began time.Time
	responses := changesRun(t, f, []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 1), changesPage("$0.capture", "$0.next_cursor", 1)}, map[int]func(){
		0: func() {
			raw, err := os.ReadFile(f.output + ".0")
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			if json.Unmarshal(raw, &response) != nil {
				t.Fatal("expiry response")
			}
			data := nativeOK(t, response)
			expires, err = time.Parse("2006-01-02T15:04:05Z", data["expires_at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			began = time.Now()
			if until := time.Until(expires); until < 598*time.Second || until > 600*time.Second {
				t.Fatalf("not a real ten-minute fresh capture: %v", until)
			}
			t.Logf("genuine live clock capture accepted at%s; advertised expiry%s", began.UTC().Format(time.RFC3339Nano), expires.Format(time.RFC3339))
			deadline := expires.Add(-time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(min(30*time.Second, time.Until(deadline)))
				t.Logf("retained-session wall clock elapsed%v", time.Since(began))
			}
		},
		1: func() {
			raw, err := os.ReadFile(f.output + ".1")
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			if json.Unmarshal(raw, &response) != nil {
				t.Fatal("before-expiry response")
			}
			if nativeOK(t, response)["complete"] != true {
				t.Fatal("page failed while still valid")
			}
			time.Sleep(max(0, time.Until(expires.Add(time.Second))))
		},
	})
	changesRefuse(t, responses[2], "checkpoint_invalidated")
	t.Logf("genuine live expiry invalidated after%v; no HMAC corruption or product test clock", time.Since(began))
}

func TestExtensionChangesRawWireBoundary(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	comments := make([]string, 127)
	for i := range comments {
		comments[i] = strings.Repeat("c", 65000)
	}
	comments = append(comments, "c")
	_, entries := changesBulkProducer(t, f, []string{""}, comments...)
	selection := changesSelection([]string{"issue"}, []string{})
	// All dynamic host IDs/tokens have fixed grammar lengths; calculate the exact
	// full ordinary wire JSON frame independently, including wrapper/checkpoint.
	id := "sha256:" + strings.Repeat("a", 64)
	page := map[string]any{"capture": strings.Repeat("b", 64), "snapshot_id": id, "selection": selection, "selection_digest": id, "source_cut": id, "mode": "replace", "items": entries, "next_cursor": nil, "complete": true, "checkpoint": strings.Repeat("d", 129), "expires_at": "2026-10-31T08:10:00Z"}
	wrapper := map[string]any{"protocol": "hn.extension/1", "request_id": "change-0", "ok": true, "data": page, "error": nil}
	raw, _ := json.Marshal(wrapper)
	remaining := 8388608 - len(raw)
	for n := 0; n < 127 && remaining > 65534; n++ {
		add := min(536, remaining-65534)
		comments[n] += strings.Repeat("c", add)
		remaining -= add
	}
	if remaining < 0 || remaining > 65534 {
		t.Fatal("wire adjustment outside native text bound", remaining)
	}
	comments[len(comments)-1] = strings.Repeat("c", remaining+1)
	_, entries = changesBulkProducer(t, f, []string{""}, comments...)
	page["items"] = entries
	raw, _ = json.Marshal(wrapper)
	if len(raw) != 8388608 {
		t.Fatal("independent wire length", len(raw))
	}
	t.Logf("source-assisted complete single state full wire at=%d over=%d", len(raw), len(raw)+1)
	response := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	if nativeOK(t, response)["complete"] != true {
		t.Fatal("at-frame complete state refused")
	}
	actual, _ := json.Marshal(response)
	if len(actual) != 8388608 {
		t.Fatal("actual complete response length", len(actual))
	}
	comments[len(comments)-1] += "c"
	changesBulkProducer(t, f, []string{""}, comments...)
	before := hostNativeSnapshot(t, f.repo)
	refusal := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	changesRefuse(t, refusal, "resource_limit")
	if !strings.Contains(refusal["error"].(map[string]any)["message"].(string), "export") {
		t.Fatal("oversized single state falsely promises smaller page")
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("oversized wire created capture/checkpoint state")
	}
}
func TestExtensionChangesRawTypedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		request    map[string]any
	}{
		{"zero-page", "invalid_input", changesCapture(changesSelection([]string{"issue"}, []string{}), 0)},
		{"over-page", "resource_limit", changesCapture(changesSelection([]string{"issue"}, []string{}), 201)},
		{"no-implicit-all", "invalid_input", changesCapture(changesSelection([]string{}, []string{}), 20)},
		{"capture-token-over", "resource_limit", changesPage(strings.Repeat("x", 4097), "cursor", 20)},
		{"cursor-over", "resource_limit", changesPage("capture", strings.Repeat("x", 4097), 20)},
		{"checkpoint-over", "resource_limit", changesSince(strings.Repeat("x", 4097), changesSelection([]string{"issue"}, []string{}), 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := changesFixture(t, "native:issue=read,observe")
			script, _ := json.Marshal(map[string]any{"requests": []any{tc.request}, "pauses": []int{}})
			if err := os.WriteFile(f.input, script, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(f.binary, "extension", "run", "example.native/tool", "--json")
			cmd.Dir = f.repo
			cmd.Env = f.env
			before := hostNativeSnapshot(t, f.repo)
			stdout, err := cmd.Output()
			if err == nil {
				t.Fatal("malformed raw host request did not terminate session")
			}
			var parent map[string]any
			if json.Unmarshal(stdout, &parent) != nil {
				t.Fatalf("parent management output %s", stdout)
			}
			if parent["error"].(map[string]any)["code"] != tc.code {
				t.Fatal("wrong typed boundary cause", parent)
			}
			// The rejected typed request is observed at the parent management
			// boundary; startup/session failure may close the child before a
			// terminal wire response is delivered. No business adapter ran.
			if evidence := os.Getenv("HN_WP19_EVIDENCE"); evidence != "" {
				file, e := os.CreateTemp(evidence, "typed-boundary-*.json")
				if e != nil {
					t.Fatal(e)
				}
				out, _ := json.Marshal(map[string]any{"request": tc.request, "parent": parent})
				file.Write(out)
				file.Close()
			}
			if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
				t.Fatal("typed refusal wrote private/native state")
			}
		})
	}
}

func TestExtensionChangesNativeDefaultRemainsInert(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	id := changesOpenIssue(f, "ordinary native setup")
	config := wp25FixtureConfig(t, f.home)
	registry := filepath.Join(config, "hubnot", "extensions", "registry.json")
	if raw, err := os.ReadFile(registry); err != nil || len(raw) == 0 {
		t.Fatal("actual registration did not create selected OS registry", registry, err)
	}
	if err := os.WriteFile(registry, []byte("{ deliberately malformed registry"), 0600); err != nil {
		t.Fatal(err)
	}
	wp25MalformedRegistryReadControl(t, f.binary, f.repo, f.home, config)
	before := hostNativeSnapshot(t, f.repo)
	child, _ := os.Executable()
	trace := extensionBaselineTraceRun(t, f.env, f.repo, f.binary, "issue", "show", id, "--json")
	if cause := extensionBaselineViolation(trace, registry, child, before, hostNativeSnapshot(t, f.repo)); cause != "" {
		t.Fatal("ordinary native command gained extension work", cause)
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git", "hn", "extensions", "changes")); !os.IsNotExist(err) {
		t.Fatal("ordinary native read initialized verifier")
	}
	// Prove the actual detector recognizes a deliberately opened registry path.
	if *extensionBaselineTrace != "" {
		actual, err := os.ReadFile(registry)
		if err != nil || len(actual) == 0 {
			t.Fatal(err)
		}
		command := exec.Command(*extensionBaselineTrace, "-f", "-qq", "-s", "4096", "-e", "trace=openat,execve,connect", "-o", filepath.Join(f.home, "negative.trace"), "cat", registry)
		command.Env = f.env
		command.Dir = f.repo
		if _, err = command.Output(); err != nil {
			t.Fatal("detector negative control", err)
		}
		raw, err := os.ReadFile(filepath.Join(f.home, "negative.trace"))
		if err != nil {
			t.Fatal(err)
		}
		if cause := extensionBaselineViolation(string(raw), registry, child, before, before); cause != "registry-read" {
			t.Fatal("actual registry-read negative control missed", cause)
		}
	}
}

func TestExtensionChangesVerifiedHistoryRewindInvalidates(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	actor := f.actor()
	firstID := changesOpenIssue(f, "retained prefix")
	changesOpenIssue(f, "later accepted event")
	selection := changesSelection([]string{"issue"}, []string{})
	original := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	ref := "refs/hn/actors/" + actor
	f.git("update-ref", ref, f.git("rev-parse", ref+"^"))
	// The remaining prefix is itself valid. This is loss of the old checkpoint's
	// authenticated accepted evidence, rather than corrupt-signature detection.
	responses := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, original)["checkpoint"], selection, 200), changesCapture(selection, 200)}, nil)
	changesRefuse(t, responses[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(changesIDs(t, responses[1]), []string{firstID}) {
		t.Fatal("fresh verified prefix capture", responses)
	}
}

func TestExtensionChangesAllVisibleCommentRevisions(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	comments := make([]string, 1023)
	for i := range comments {
		comments[i] = "visible comment"
	}
	selection := changesSelection([]string{"issue"}, []string{})
	_, entries := changesBulkProducer(t, f, []string{""}, comments...)
	at := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	entry := nativeOK(t, at)["items"].([]any)[0].(map[string]any)
	if len(entry["revision_ids"].([]any)) != 1024 {
		t.Fatal("at1024 revisions", entry)
	}
	comments = append(comments, "one more visible comment")
	_, entries = changesBulkProducer(t, f, []string{""}, comments...)
	expected, _ := json.Marshal(entries)
	t.Logf("source-assisted1025 visible revisions full C entry bytes=%d, below published capture/frame bounds", len(expected))
	over := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	entry = nativeOK(t, over)["items"].([]any)[0].(map[string]any)
	if len(entry["revision_ids"].([]any)) != 1025 || len(entry["value"].(map[string]any)["comments"].([]any)) != 1024 {
		t.Fatal("visible comments/revision identities truncated")
	}
}

func changesConsumerSelection(t *testing.T, f *nativeHostFixture, page map[string]any) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.home, "observer-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if json.Unmarshal(raw, &state) != nil {
		t.Fatal("observer state")
	}
	selected, ok := state[page["selection_digest"].(string)].(map[string]any)
	if !ok {
		t.Fatal("observer omitted completed selection")
	}
	return selected
}
func changesAssertMixedCut(t *testing.T, f *nativeHostFixture, page, selection map[string]any, ids []string) {
	t.Helper()
	refs := []map[string]any{}
	for _, line := range strings.Split(f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/artifacts/"), "\n") {
		row := strings.Fields(line)
		if len(row) != 2 {
			t.Fatal("witness fixture", line)
		}
		refs = append(refs, map[string]any{"ref": row[0], "oid": row[1]})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i]["ref"].(string) < refs[j]["ref"].(string) })
	sort.Strings(ids)
	view := independentNativeHash(map[string]any{"admitted_ids": ids, "visible_ids": ids, "retained_refs": []any{}})
	common, err := filepath.EvalSymlinks(filepath.Join(f.repo, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("hn-extension-repository-v1\x00" + common))
	expected := independentNativeHash(map[string]any{"repository_id": "sha256:" + hex.EncodeToString(sum[:]), "refs": refs, "selection": selection, "authority_commit": "", "policy_digest": "", "view_digest": view})
	if page["source_cut"] != expected {
		t.Fatal("mixed selected Cut differs from independent frozen preimage", page["source_cut"], expected)
	}
}

func TestExtensionChangesSourceRaceDuringPage(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	actor := f.actor()
	changesOpenIssue(f, "race one")
	changesOpenIssue(f, "race two")
	ref := "refs/hn/actors/" + actor
	beforeHead := f.git("rev-parse", ref)
	changesOpenIssue(f, "prepared future accepted source")
	afterHead := f.git("rev-parse", ref)
	f.git("update-ref", ref, beforeHead)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	armed := filepath.Join(directory, "armed")
	marker := filepath.Join(directory, "actual-source-race")
	// Source-assisted Git scheduling control. It changes a real accepted ref at
	// the shared reader's cat-file boundary, after its initial witness discovery;
	// the product receives no test flag or alternate clock/verifier.
	script := "#!/bin/sh\nif [ \"$1\" = cat-file ] && [ \"$2\" = --batch ] && [ -f '" + armed + "' ]; then\n rm '" + armed + "'\n '" + realGit + "' update-ref '" + ref + "' '" + afterHead + "' '" + beforeHead + "' || exit 82\n echo actual-witness-change > '" + marker + "'\nfi\nexec '" + realGit + "' \"$@\"\n"
	if err = os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for i, value := range f.env {
		if strings.HasPrefix(value, "PATH=") {
			f.env[i] = "PATH=" + directory + ":" + strings.TrimPrefix(value, "PATH=")
		}
	}
	arm := func() {
		if err := os.WriteFile(armed, []byte("arm"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	selection := changesSelection([]string{"issue"}, []string{})
	responses := changesRun(t, f, []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 1), changesPage("$0.capture", "$0.next_cursor", 1), changesCapture(selection, 1)}, map[int]func(){
		0: arm,
		1: func() {
			if _, err := os.Stat(marker); err != nil {
				t.Fatal("actual shared-read race did not fire", err)
			}
			f.git("update-ref", ref, beforeHead)
		},
		2: arm,
	})
	changesRefuse(t, responses[1], "checkpoint_invalidated")
	// Restoring the exact prior Cut cannot revive a capture once a reader saw it
	// race. This request uses the same still-live host SessionID.
	changesRefuse(t, responses[2], "checkpoint_invalidated")
	nativeDenial(t, responses[3], "native", "concurrent_update", "retry_identical")
}
