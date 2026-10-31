package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func hostRead(r io.Reader) map[string]any {
	var prefix [4]byte
	if _, e := io.ReadFull(r, prefix[:]); e != nil {
		os.Exit(70)
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n > 8388608 {
		os.Exit(71)
	}
	data := make([]byte, n)
	if _, e := io.ReadFull(r, data); e != nil {
		os.Exit(72)
	}
	var object map[string]any
	if json.Unmarshal(data, &object) != nil {
		os.Exit(73)
	}
	return object
}
func hostWrite(value any) {
	data, _ := json.Marshal(value)
	_, _ = os.Stdout.Write(protocolRawFrame(data))
}

// This compiled test asset runs as a separate executable with exact registered
// argv. Its application result file is owned by this reference consumer.
func TestExtensionChildAsset(t *testing.T) {
	if os.Getenv("HN_WP16_CHILD") != "1" {
		return
	}
	at := 0
	for i, arg := range os.Args {
		if arg == "--" {
			at = i + 1
			break
		}
	}
	if at == 0 || len(os.Args) != at+3 {
		os.Exit(74)
	}
	mode, marker, result := os.Args[at], os.Args[at+1], os.Args[at+2]
	if mode == "ignore-term" {
		signalIgnoreTerm()
	}
	_ = os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600)
	if mode == "descendant" {
		signalIgnoreTerm()
		for {
			time.Sleep(time.Hour)
		}
	}
	hello := hostRead(os.Stdin)
	if mode == "startup-hang" {
		_, _ = os.Stdout.Write([]byte{0, 0, 0})
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode == "incompatible" {
		hostWrite(map[string]any{"protocol": "hn.extension/9", "ready": true})
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode == "stderr-over" {
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("PRIVATE-DO-NOT-ECHO"), 4000))
		for {
			time.Sleep(time.Hour)
		}
	}
	hostWrite(map[string]any{"protocol": "hn.extension/1", "ready": true})
	if mode == "idle" || mode == "ignore-term" {
		if mode == "ignore-term" {
			signalIgnoreTerm()
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode == "crash" {
		os.Exit(19)
	}
	if mode == "crash-descendant" {
		executable, _ := os.Executable()
		child := exec.Command(executable, "-test.run=^TestExtensionChildAsset$", "--", "descendant", marker+".descendant", result)
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if child.Start() != nil {
			os.Exit(75)
		}
		until := time.Now().Add(time.Second)
		for time.Now().Before(until) {
			if _, e := os.Stat(marker + ".descendant"); e == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		os.Exit(19)
	}

	if mode == "oversize" {
		_, _ = os.Stdout.Write([]byte{0, 128, 0, 1})
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode == "zero" {
		_, _ = os.Stdout.Write([]byte{0, 0, 0, 0})
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode == "stderr-at" {
		_, _ = os.Stderr.Write(bytes.Repeat([]byte{'x'}, 65536))
	}
	requests := []map[string]any{{"protocol": "hn.extension/1", "request_id": "inventory", "method": "capabilities.list", "params": map[string]any{}}}
	if mode == "unsupported" {
		requests = append(requests, map[string]any{"protocol": "hn.extension/1", "request_id": "privileged", "method": "memory.mutate", "params": map[string]any{}})
	}
	if mode == "ungranted" {
		requests = append(requests, map[string]any{"protocol": "hn.extension/1", "request_id": "read", "method": "resource.read", "params": map[string]any{"family": "project", "id": nil, "limit": 20, "cursor": nil}})
	}
	if mode == "duplicate" {
		requests = append(requests, requests[0])
	}
	if mode == "ungranted-write" {
		requests = append(requests, map[string]any{"protocol": "hn.extension/1", "request_id": "write", "method": "resource.mutate", "params": map[string]any{"family": "issue", "intent": "open", "id": nil, "expected": []any{}, "actor": strings.Repeat("a", 64), "operation": "raw-ungranted", "state": map[string]any{"title": "ticket", "body": "", "status": "open", "criteria": []any{}, "labels": []any{}, "assignees": []any{}, "relations": []any{}, "metadata": map[string]any{}}, "comment": ""}})
	}
	responses := []any{}
	for _, request := range requests {
		if mode == "unsupported" && request["method"] == "memory.mutate" {
			// Pair this exact attempted request marker with the host's terminal
			// unsupported response, proving the real child reached raw framing.
			data, _ := json.Marshal(request)
			if os.WriteFile(marker+".request", data, 0600) != nil {
				os.Exit(76)
			}
		}
		if mode == "frame-at" {
			data, _ := json.Marshal(request)
			data = append(data, bytes.Repeat([]byte{' '}, 8388608-len(data))...)
			_, _ = os.Stdout.Write(protocolRawFrame(data))
		} else {
			hostWrite(request)
		}
		responses = append(responses, hostRead(os.Stdin))
	}
	encoded, _ := json.Marshal(map[string]any{"hello": hello, "responses": responses, "argv": os.Args[at:]})
	_ = os.WriteFile(result, encoded, 0600)
	os.Exit(0)
}
func signalIgnoreTerm() { signal.Ignore(syscall.SIGTERM) }

type hostFixture struct {
	t                                                            *testing.T
	binary, home, repo, config, manifest, marker, result, module string
	env                                                          []string
}

func newHostFixture(t *testing.T, mode string) *hostFixture {
	t.Helper()
	bin := os.Getenv("HN_WP16_BINARY")
	if bin == "" {
		bin = extensionTestBinary(t)
	}
	f := &hostFixture{t: t, binary: bin, home: t.TempDir(), repo: t.TempDir(), module: "example.host/tool"}
	f.config = filepath.Join(f.home, "config")
	if runtime.GOOS == "darwin" {
		// Match os.UserConfigDir under the child's isolated HOME. Darwin
		// ignores XDG_CONFIG_HOME, including for the malformed-registry control.
		f.config = filepath.Join(f.home, "Library", "Application Support")
	}
	f.manifest = filepath.Join(f.home, "manifest.json")
	f.marker = filepath.Join(f.home, "started")
	f.result = filepath.Join(f.home, "result.json")
	f.env = operationalTestEnvironmentWith(f.home, map[string]string{"XDG_CONFIG_HOME": f.config, "HN_WP16_CHILD": "1"})
	git := exec.Command("git", "init", "-q", f.repo)
	git.Env = f.env
	if out, e := git.CombinedOutput(); e != nil {
		t.Fatalf("init %v %s", e, out)
	}
	data := `{"protocol":"hn.extension-manifest/1","module_id":"example.host/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["capabilities.list","resource.read","resource.mutate"]}`
	if e := os.WriteFile(f.manifest, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	child, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	args := []string{"register", "--manifest", f.manifest, "--executable", child, "--arg", "-test.run=^TestExtensionChildAsset$", "--arg", "--", "--arg", mode, "--arg", f.marker, "--arg", f.result, "--json"}
	f.invoke("", args...)
	f.invoke("", "enable", f.module, "--grant", "native:issue=read", "--json")
	return f
}
func (f *hostFixture) command(args ...string) *exec.Cmd {
	cmd := exec.Command(f.binary, append([]string{"extension"}, args...)...)
	cmd.Env = f.env
	cmd.Dir = f.repo
	return cmd
}
func (f *hostFixture) invoke(code string, args ...string) map[string]any {
	f.t.Helper()
	cmd := f.command(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var envelope map[string]any
	if e := json.Unmarshal(stdout.Bytes(), &envelope); e != nil {
		f.t.Fatalf("%v: JSON %v %v %s %s", args, e, err, &stdout, &stderr)
	}
	if code == "" {
		if err != nil || envelope["ok"] != true {
			f.t.Fatalf("%v: want success: %v %s %s", args, err, &stdout, &stderr)
		}
	} else {
		e, ok := envelope["error"].(map[string]any)
		if err == nil || envelope["ok"] != false || envelope["data"] != nil || !ok || e["code"] != code {
			f.t.Fatalf("%v: want %s: %v %s %s", args, code, err, &stdout, &stderr)
		}
	}
	if strings.Contains(stdout.String()+stderr.String(), "PRIVATE-DO-NOT-ECHO") {
		f.t.Fatal("private stderr leaked")
	}
	return envelope
}
func waitHostMarker(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, e := os.ReadFile(path)
		if e == nil {
			pid, e := strconv.Atoi(string(b))
			if e == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child did not start")
	return 0
}
func assertHostExited(t *testing.T, pid int) {
	t.Helper()
	if e := syscall.Kill(pid, 0); e != syscall.ESRCH {
		t.Fatalf("owned child %d still exists: %v", pid, e)
	}
}
func TestExtensionRunCLI(t *testing.T) {
	f := newHostFixture(t, "normal")
	before := hostNativeSnapshot(t, f.repo)
	got := f.invoke("", "run", f.module, "--json")["data"].(map[string]any)
	if len(got) != 5 || got["outcome"] != "completed" || got["completed_requests"] != float64(1) || got["exit_code"] != float64(0) || len(got["pending_operations"].([]any)) != 0 {
		t.Fatalf("run summary %v", got)
	}
	session := got["session"].(map[string]any)
	if len(session) != 6 || session["protocol"] != "hn.extension/1" || session["module_id"] != f.module || len(session["session_id"].(string)) != 32 {
		t.Fatalf("session %v", session)
	}
	data, e := os.ReadFile(f.result)
	if e != nil {
		t.Fatal(e)
	}
	var consumer map[string]any
	if json.Unmarshal(data, &consumer) != nil {
		t.Fatal("consumer output")
	}
	if !reflect.DeepEqual(consumer["argv"], []any{"normal", f.marker, f.result}) {
		t.Fatalf("registered argv changed %v", consumer["argv"])
	}
	responses := consumer["responses"].([]any)
	inventory := responses[0].(map[string]any)["data"].(map[string]any)
	if !reflect.DeepEqual(inventory["session"], session) {
		t.Fatalf("bound session %v %v", inventory["session"], session)
	}
	expected := []string{"artifact.mutate", "artifact.operation", "artifact.read", "artifact.search", "artifact.type.publish", "artifact.type.read", "capabilities.list", "changes.capture", "changes.page", "changes.since", "identity.public", "resource.mutate", "resource.operation", "resource.read"}
	actual := []string{}
	for _, entry := range inventory["capabilities"].([]any) {
		actual = append(actual, entry.(map[string]any)["method"].(string))
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("closed methods %v", actual)
	}
	unsupported := []string{}
	for _, entry := range inventory["unsupported"].([]any) {
		unsupported = append(unsupported, entry.(map[string]any)["surface"].(string))
	}
	wantUnsupported := []string{"memory.mutate", "proposal", "review", "decide", "merge", "policy.mutate", "identity.mutate", "run", "runner", "audit", "automerge", "sync.apply", "migration.apply"}
	if !reflect.DeepEqual(unsupported, wantUnsupported) {
		t.Fatalf("closed unsupported %v", unsupported)
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("read-only host changed native objects/refs/private signing state")
	}
	assertHostExited(t, waitHostMarker(t, f.marker))
}
func TestExtensionRunRefusalsCLI(t *testing.T) {
	for _, tc := range []struct{ mode, code string }{{"incompatible", "incompatible_version"}, {"crash", "unavailable_module"}, {"oversize", "resource_limit"}, {"zero", "invalid_input"}, {"stderr-over", "resource_limit"}, {"duplicate", "invalid_input"}, {"stderr-at", ""}, {"frame-at", ""}, {"unsupported", "unsupported_capability"}, {"ungranted", ""}, {"ungranted-write", ""}} {
		t.Run(tc.mode, func(t *testing.T) {
			f := newHostFixture(t, tc.mode)
			before := hostNativeSnapshot(t, f.repo)
			outcome := f.invoke(tc.code, "run", f.module, "--json")
			assertHostExited(t, waitHostMarker(t, f.marker))
			if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
				t.Fatal("refusal changed native state")
			}
			if tc.mode == "unsupported" {
				data, err := os.ReadFile(f.marker + ".request")
				if err != nil {
					t.Fatalf("child did not reach privileged request: %v", err)
				}
				var attempted map[string]any
				want := map[string]any{"protocol": "hn.extension/1", "request_id": "privileged", "method": "memory.mutate", "params": map[string]any{}}
				if json.Unmarshal(data, &attempted) != nil || !reflect.DeepEqual(attempted, want) {
					t.Fatalf("actual raw request %s", data)
				}
				public := outcome["error"].(map[string]any)
				if len(public) != 5 || public["origin"] != "host" || public["recovery"] != "do_not_retry" || public["operation"] != nil {
					t.Fatalf("terminal unsupported error %v", public)
				}
			}
			if tc.mode == "ungranted" || tc.mode == "ungranted-write" {
				data, _ := os.ReadFile(f.result)
				want := "unsupported_capability"
				if tc.mode == "ungranted" || tc.mode == "ungranted-write" {
					want = "denied_access"
				}
				if !bytes.Contains(data, []byte(`"code":"`+want+`"`)) {
					t.Fatalf("actual request refusal %s", data)
				}
			}
		})
	}
}
func TestExtensionRunDisableAndSlotsCLI(t *testing.T) {
	f := newHostFixture(t, "ignore-term")
	linked := filepath.Join(t.TempDir(), "linked")
	for _, args := range [][]string{{"-C", f.repo, "-c", "user.name=Host fixture", "-c", "user.email=host@example.invalid", "commit", "--allow-empty", "-qm", "Physical repository slot fixture"}, {"-C", f.repo, "worktree", "add", "--detach", linked, "HEAD"}} {
		cmd := exec.Command("git", args...)
		cmd.Env = f.env
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("linked fixture %v %s", e, out)
		}
	}

	type running struct {
		cmd         *exec.Cmd
		done        chan error
		out, stderr bytes.Buffer
		pid         int
	}
	hosts := []*running{}
	defer func() {
		for _, host := range hosts {
			if host.cmd.Process != nil {
				_ = host.cmd.Process.Signal(syscall.SIGTERM)
			}
		}
	}()
	for i := 0; i < 4; i++ {
		os.Remove(f.marker)
		host := &running{cmd: f.command("run", f.module, "--json"), done: make(chan error, 1)}
		host.cmd.Stdout = &host.out
		host.cmd.Stderr = &host.stderr
		if e := host.cmd.Start(); e != nil {
			t.Fatal(e)
		}
		go func() { host.done <- host.cmd.Wait() }()
		host.pid = waitHostMarker(t, f.marker)
		hosts = append(hosts, host)
	}
	f.invoke("resource_limit", "run", f.module, "--json")
	originalRepo := f.repo
	f.repo = linked
	f.invoke("resource_limit", "run", f.module, "--json")
	f.repo = originalRepo
	started := time.Now()
	data := f.invoke("", "disable", f.module, "--json")["data"].(map[string]any)
	if time.Since(started) > 5*time.Second || data["cleanup"] != "verified" || len(data["stopped_sessions"].([]any)) != 4 {
		t.Fatalf("cleanup %v duration %s", data, time.Since(started))
	}
	for _, host := range hosts {
		select {
		case e := <-host.done:
			if e == nil {
				t.Fatal("revoked host reported success")
			}
		case <-time.After(time.Second):
			t.Fatal("host not joined")
		}
		assertHostExited(t, host.pid)
	}
	f.invoke("denied_access", "run", f.module, "--json")
	f.invoke("", "remove", f.module, "--json")
	if _, e := os.Stat(f.result); !os.IsNotExist(e) {
		t.Fatal("idle module ran business requests")
	}
}
func TestExtensionRunNoImplicitLaunchCLI(t *testing.T) {
	f := newHostFixture(t, "normal")
	f.invoke("", "inspect", f.module, "--json")
	if _, e := os.Stat(f.marker); !os.IsNotExist(e) {
		t.Fatal("management launched executable")
	}
	f.invoke("", "disable", f.module, "--json")
	f.invoke("denied_access", "run", f.module, "--json")
	if _, e := os.Stat(f.marker); !os.IsNotExist(e) {
		t.Fatal("disabled module launched")
	}
	f.invoke("", "remove", f.module, "--json")
	f.invoke("not_found", "run", f.module, "--json")
}
func TestExtensionRunStartupDeadlineCLI(t *testing.T) {
	f := newHostFixture(t, "startup-hang")
	start := time.Now()
	f.invoke("cancelled", "run", f.module, "--json")
	if d := time.Since(start); d < 9*time.Second || d > 15*time.Second {
		t.Fatalf("startup budget %s", d)
	}
	assertHostExited(t, waitHostMarker(t, f.marker))
}

func hostNativeSnapshot(t *testing.T, repo string) map[string]string {
	values := extensionPrivateSnapshot(t, repo)
	for path := range values {
		if strings.Contains(path, "/extensions/session-") {
			delete(values, path)
		}
	}
	return values
}

func TestExtensionRunRemoveAcrossRepositoriesCLI(t *testing.T) {
	f := newHostFixture(t, "idle")
	repo2 := t.TempDir()
	git := exec.Command("git", "init", "-q", repo2)
	git.Env = f.env
	if out, e := git.CombinedOutput(); e != nil {
		t.Fatalf("second repo %v %s", e, out)
	}
	original := f.repo
	f.repo = repo2
	f.invoke("", "enable", f.module, "--grant", "native:issue=read", "--json")
	host := f.command("run", f.module, "--json")
	var out, stderr bytes.Buffer
	host.Stdout = &out
	host.Stderr = &stderr
	if e := host.Start(); e != nil {
		t.Fatal(e)
	}
	defer host.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- host.Wait() }()
	pid := waitHostMarker(t, f.marker)
	// The removal caller has no repository selection; it uses only recorded
	// registration owners and never scans repository trees or signals disk PIDs.
	f.repo = t.TempDir()
	data := f.invoke("", "remove", f.module, "--json")["data"].(map[string]any)
	if len(data["stopped_sessions"].([]any)) != 1 {
		t.Fatalf("registration cleanup %v", data)
	}
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("removed host reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("removed host remained")
	}
	assertHostExited(t, pid)
	f.repo = original
	child, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	f.invoke("", "register", "--manifest", f.manifest, "--executable", child, "--arg", "-test.run=^TestExtensionChildAsset$", "--arg", "--", "--arg", "idle", "--arg", f.marker, "--arg", f.result, "--json")
	f.invoke("denied_access", "run", f.module, "--json")
	f.repo = repo2
	f.invoke("denied_access", "run", f.module, "--json")
}

func TestExtensionRunIdleDoesNotHoldNativeLockCLI(t *testing.T) {
	f := newHostFixture(t, "idle")
	native := func(wantBusy bool, args ...string) {
		t.Helper()
		cmd := exec.Command(f.binary, args...)
		cmd.Dir = f.repo
		cmd.Env = f.env
		out, e := cmd.CombinedOutput()
		if wantBusy {
			if e == nil || !bytes.Contains(out, []byte("contribution operation busy")) {
				t.Fatalf("lock detector missed exact busy cause: %v %s", e, out)
			}
		} else if e != nil {
			t.Fatalf("native progress: %v %s", e, out)
		}
	}
	native(false, "init", "--name", "Host lock fixture")
	directory := filepath.Join(f.repo, ".git", "hn", "contribution")
	if e := os.MkdirAll(directory, 0700); e != nil {
		t.Fatal(e)
	}
	// The wrapper's lock classification depends only on this marker's presence;
	// identity show does not consume a contribution policy or write native facts.
	if e := os.WriteFile(filepath.Join(directory, "authority.json"), []byte("lock-classification-marker"), 0600); e != nil {
		t.Fatal(e)
	}
	host := f.command("run", f.module, "--json")
	var output bytes.Buffer
	host.Stdout = &output
	host.Stderr = &output
	if e := host.Start(); e != nil {
		t.Fatal(e)
	}
	defer host.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- host.Wait() }()
	pid := waitHostMarker(t, f.marker)
	native(false, "identity", "show", "--json")
	lock, e := os.OpenFile(filepath.Join(directory, "operation.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		t.Fatal(e)
	}
	native(true, "identity", "show", "--json")
	_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	// A second registered/enabled process completes while the first remains idle.
	secondManifest := filepath.Join(f.home, "second.json")
	data, _ := os.ReadFile(f.manifest)
	data = bytes.ReplaceAll(data, []byte(f.module), []byte("example.host/second"))
	if e = os.WriteFile(secondManifest, data, 0600); e != nil {
		t.Fatal(e)
	}
	child, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	f.invoke("", "register", "--manifest", secondManifest, "--executable", child, "--arg", "-test.run=^TestExtensionChildAsset$", "--arg", "--", "--arg", "normal", "--arg", filepath.Join(f.home, "second.pid"), "--arg", filepath.Join(f.home, "second.result"), "--json")
	f.invoke("", "enable", "example.host/second", "--grant", "native:issue=read", "--json")
	f.invoke("", "run", "example.host/second", "--json")
	f.invoke("", "disable", f.module, "--json")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle host did not retire")
	}
	assertHostExited(t, pid)
	// Malformed optional state does not enter the ordinary native path.
	if e = os.WriteFile(filepath.Join(f.config, "hubnot", "extensions", "registry.json"), []byte("malformed optional registry"), 0600); e != nil {
		t.Fatal(e)
	}
	// The explicit path must consume this exact poisoned registry, while the
	// ordinary native command remains independent of it on each platform.
	f.invoke("invalid_input", "list", "--json")
	native(false, "identity", "show", "--json")
}

func TestExtensionRunIdleDeadlineCLI(t *testing.T) {
	f := newHostFixture(t, "idle")
	start := time.Now()
	f.invoke("cancelled", "run", f.module, "--json")
	if d := time.Since(start); d < 29*time.Second || d > 35*time.Second {
		t.Fatalf("idle read budget %s", d)
	}
	assertHostExited(t, waitHostMarker(t, f.marker))
}

func TestExtensionRunCrashCleansPipeOwningDescendantCLI(t *testing.T) {
	f := newHostFixture(t, "crash-descendant")
	start := time.Now()
	f.invoke("unavailable_module", "run", f.module, "--json")
	if time.Since(start) > 5*time.Second {
		t.Fatal("exited leader's pipe-owning descendant escaped cleanup watchdog")
	}
	assertHostExited(t, waitHostMarker(t, f.marker))
	// A process adopted by the OS may briefly remain as a reaped-pending zombie;
	// it must have exited and cannot retain stdio or execute application code.
	pid := waitHostMarker(t, f.marker+".descendant")
	if e := syscall.Kill(pid, 0); e != syscall.ESRCH {
		state, readErr := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		if syscall.Kill(pid, 0) != syscall.ESRCH && (readErr != nil || !strings.HasPrefix(strings.TrimSpace(string(state)), "Z")) {
			t.Fatalf("descendant %d still running: %v %s", pid, e, state)
		}
	}
}
