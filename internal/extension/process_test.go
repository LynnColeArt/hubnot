package extension

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func assetRead() map[string]any {
	var prefix [4]byte
	if _, e := io.ReadFull(os.Stdin, prefix[:]); e != nil {
		os.Exit(60)
	}
	data := make([]byte, binary.BigEndian.Uint32(prefix[:]))
	if _, e := io.ReadFull(os.Stdin, data); e != nil {
		os.Exit(61)
	}
	var value map[string]any
	if json.Unmarshal(data, &value) != nil {
		os.Exit(62)
	}
	return value
}
func assetWrite(value any) {
	data, _ := json.Marshal(value)
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
	_ = writeAllAsset(prefix[:])
	_ = writeAllAsset(data)
}
func writeAllAsset(b []byte) error {
	for len(b) > 0 {
		n, e := os.Stdout.Write(b)
		if e != nil {
			return e
		}
		b = b[n:]
	}
	return nil
}
func TestSessionProcessAsset(t *testing.T) {
	mode := os.Getenv("HN_SESSION_ASSET")
	if mode == "" {
		return
	}
	_ = os.WriteFile(os.Getenv("HN_SESSION_PID"), []byte(strconv.Itoa(os.Getpid())), 0600)
	_ = assetRead()
	assetWrite(map[string]any{"protocol": HostProtocol, "ready": true})
	if mode == "idle" {
		for {
			time.Sleep(time.Hour)
		}
	}
	request := map[string]any{"protocol": HostProtocol, "request_id": "write", "method": "resource.mutate", "timeout_ms": 100, "params": map[string]any{"family": "issue", "intent": "open", "id": nil, "expected": []any{}, "actor": repeatActor(), "operation": "actual-key", "state": map[string]any{"title": "ticket", "body": "", "status": "open", "criteria": []any{}, "labels": []any{}, "assignees": []any{}, "relations": []any{}, "metadata": map[string]any{}}, "comment": ""}}
	if mode == "read-timeout" {
		request = map[string]any{"protocol": HostProtocol, "request_id": "read", "method": "resource.read", "timeout_ms": 100, "params": map[string]any{"family": "issue", "id": nil, "cursor": nil, "limit": 20}}
	}
	assetWrite(request)
	switch mode {
	case "cancel":
		time.Sleep(20 * time.Millisecond)
		assetWrite(map[string]any{"protocol": HostProtocol, "cancel": "write"})
	case "second":
		time.Sleep(20 * time.Millisecond)
		assetWrite(request)
	case "wrong-cancel":
		time.Sleep(20 * time.Millisecond)
		assetWrite(map[string]any{"protocol": HostProtocol, "cancel": "wrong"})
	case "eof":
		os.Exit(0)
	}
	if mode == "lost-response" {
		_ = os.Stdin.Close()
		for {
			time.Sleep(time.Hour)
		}
	}
	_ = assetRead()
	os.Exit(0)
}
func repeatActor() string { return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
func sessionFixture(t *testing.T, mode string) (SessionConfig, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HN_SESSION_ASSET", mode)
	pid := filepath.Join(t.TempDir(), "pid")
	t.Setenv("HN_SESSION_PID", pid)
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	manifest := Manifest{"hn.extension-manifest/1", "example.session/tool", "1", HostProtocol, []string{"resource.read", "resource.mutate", "capabilities.list"}}
	r, e := NewRegistration(manifest, executable, []string{"-test.run=^TestSessionProcessAsset$"})
	if e != nil {
		t.Fatal(e)
	}
	r, e = Register(filepath.Join(t.TempDir(), "hubnot", "extensions", "registry.json"), r)
	if e != nil {
		t.Fatal(e)
	}
	common := t.TempDir()
	store, e := OpenEnablements(common)
	if e != nil {
		t.Fatal(e)
	}
	_, e = store.Set(r, true, []Grant{{"native:issue", []string{"read", "create"}}})
	if e != nil {
		t.Fatal(e)
	}
	cfg := SessionConfig{Registration: r, Store: store, Directory: t.TempDir()}
	cfg.Recheck = func(ctx context.Context, binding Binding) (Registration, EnablementEntry, error) {
		entry, _, e := store.Entry(r)
		if e == nil {
			e = Authorize(r, entry, binding, Access{Method: "capabilities.list"}, false)
		}
		return r, entry, e
	}
	return cfg, pid
}
func assertSessionPIDGone(t *testing.T, path string) {
	t.Helper()
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	pid, e := strconv.Atoi(string(raw))
	if e != nil {
		t.Fatal(e)
	}
	if e = syscall.Kill(pid, 0); e != syscall.ESRCH {
		t.Fatalf("child %d remains %v", pid, e)
	}
}
func TestSessionSlotsAndRevocationWait(t *testing.T) {
	store, e := OpenEnablements(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	ids := []slotIdentity{}
	slots := []*sessionSlot{}
	for i := 0; i < 4; i++ {
		id := slotIdentity{SessionID: fmtSessionID(i), RegistrationID: "sha256:" + repeatActor(), Epoch: "11111111111111111111111111111111"}
		slot, e := store.acquireSlot(id)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
		slots = append(slots, slot)
		defer slot.close(true)
	}
	if _, e = store.acquireSlot(ids[0]); e == nil {
		t.Fatal("fifth session admitted")
	} else {
		expectGrantError(t, e, "resource_limit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, e = store.WaitRetired(ctx, ids[0].RegistrationID, "22222222222222222222222222222222"); e == nil {
		t.Fatal("cleanup incorrectly succeeded while owners remained")
	}
	for _, slot := range slots {
		slot.close(true)
	}
	if stopped, e := store.WaitRetired(context.Background(), ids[0].RegistrationID, "22222222222222222222222222222222"); e != nil || len(stopped) != 0 {
		t.Fatalf("released owners %v %v", stopped, e)
	}
}
func fmtSessionID(n int) string {
	const digits = "0123456789abcdef"
	return "0000000000000000000000000000000" + string(digits[n])
}

func TestSessionCleanupFailureCannotBecomeManagementSuccess(t *testing.T) {
	if !reflect.DeepEqual(ownerStates, map[string]bool{"owned": true, "verified": true, "failed": true}) {
		t.Fatal("closed private ownership states drifted")
	}
	store, e := OpenEnablements(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	id := slotIdentity{SessionID: "11111111111111111111111111111111", RegistrationID: "sha256:" + repeatActor(), Epoch: "22222222222222222222222222222222"}
	slot, e := store.acquireSlot(id)
	if e != nil {
		t.Fatal(e)
	}
	slot.close(false)
	if _, e = store.WaitRetired(context.Background(), id.RegistrationID, "33333333333333333333333333333333"); e == nil {
		t.Fatal("failed child cleanup became successful disable")
	}
	if _, e = store.acquireSlot(id); e == nil {
		t.Fatal("new session erased failed cleanup evidence")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	owner, e := acquireRegistrationOwner(id)
	if e != nil {
		t.Fatal(e)
	}
	owner.close(false)
	if _, e = RetireRegistration(context.Background(), id.RegistrationID); e == nil {
		t.Fatal("failed global cleanup became successful removal")
	}
}
