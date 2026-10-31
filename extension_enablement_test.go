package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtensionEnablementCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	home, repo := t.TempDir(), t.TempDir()
	env := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, "config")})
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = repo, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	invoke := func(code string, args ...string) extensionCLIResult {
		t.Helper()
		cmd := exec.Command(binary, append([]string{"extension"}, args...)...)
		cmd.Dir, cmd.Env = repo, env
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		err := cmd.Run()
		var result extensionCLIResult
		if json.Unmarshal(out.Bytes(), &result) != nil {
			t.Fatalf("%v: invalid JSON %v %s %s", args, err, &out, &stderr)
		}
		if code == "" {
			if err != nil || !result.OK {
				t.Fatalf("%v: %v %s %s", args, err, &out, &stderr)
			}
		} else if err == nil || result.OK || result.Error.Code != code || result.Error.Origin != "host" || result.Error.Recovery != "do_not_retry" || result.Data != nil {
			t.Fatalf("%v: want %s: %v %s %s", args, code, err, &out, &stderr)
		}
		if strings.Contains(out.String(), repo) || strings.Contains(out.String(), "PRIVATE_EXTENSION") {
			t.Fatal("private path or argument leaked")
		}
		return result
	}
	module, manifest := filepath.Join(repo, "module"), filepath.Join(repo, "manifest.json")
	marker := filepath.Join(repo, "started")
	moduleBytes := []byte("#!/bin/sh\nprintf forbidden > '" + marker + "'\n")
	if err := os.WriteFile(module, moduleBytes, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["resource.read","resource.mutate","resource.operation","identity.public","artifact.read","artifact.mutate","artifact.type.publish","changes.capture"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	register := func() extensionCLIResult {
		return invoke("", "register", "--manifest", manifest, "--executable", module, "--arg", "PRIVATE_EXTENSION", "--json")
	}
	registered := register()
	protected := func() map[string]string {
		all := extensionPrivateSnapshot(t, repo)
		for p := range all {
			if strings.HasPrefix(p, "/.git/hn/extensions/") {
				delete(all, p)
			}
		}
		return all
	}
	before := protected()
	invoke("invalid_input", "enable", "example.inventory/tool", "--grant", "native:issue=write", "--json")
	invoke("invalid_input", "enable", "example.inventory/tool", "--grant", "native:identity=create", "--json")
	invoke("invalid_input", "enable", "example.inventory/tool", "--grant", "native:issue=type.publish", "--json")
	invoke("invalid_input", "enable", "example.inventory/tool", "--repository", repo, "--grant", "native:issue=read", "--json")
	statePath := filepath.Join(repo, ".git", "hn", "extensions", "enabled.json")
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatal("refused grant wrote enablement")
	}
	enabled := invoke("", "enable", "example.inventory/tool", "--grant", "native:issue=read", "--grant", "artifact:example.inventory/*=read,observe", "--json")
	if len(enabled.Data) != 8 || string(enabled.Data["enabled"]) != "true" || string(enabled.Data["cleanup"]) != `"not_needed"` || string(enabled.Data["stopped_sessions"]) != "[]" {
		t.Fatalf("enable shape: %v", enabled.Data)
	}
	wantRepoID := extensionCLIDigest([]byte("hn-extension-repository-v1\x00" + filepath.Join(repo, ".git")))
	var repoID, epoch string
	json.Unmarshal(enabled.Data["repository_id"], &repoID)
	json.Unmarshal(enabled.Data["epoch"], &epoch)
	if repoID != wantRepoID || len(epoch) != 32 {
		t.Fatalf("physical repository binding: %q %q", repoID, epoch)
	}
	for _, p := range []string{statePath, filepath.Dir(statePath), filepath.Dir(filepath.Dir(statePath))} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0700)
		if p == statePath {
			want = 0600
		}
		if info.Mode().Perm() != want {
			t.Fatalf("unsafe mode %s %v", p, info.Mode())
		}
	}
	inspect := func() map[string]json.RawMessage {
		t.Helper()
		result := invoke("", "list", "--json")
		var modules []map[string]json.RawMessage
		if json.Unmarshal(result.Data["modules"], &modules) != nil || len(modules) != 1 {
			t.Fatal("inventory")
		}
		return modules[0]
	}
	view := inspect()
	if string(view["enabled"]) != "true" || string(view["availability"]) != `"not_probed"` || string(view["repository_id"]) != string(enabled.Data["repository_id"]) || string(view["grants"]) != string(enabled.Data["grants"]) {
		t.Fatalf("inventory: %v", view)
	}
	// The OS resolves a working-directory alias before trusted Git validation.
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	physical := repo
	repo = alias
	if string(inspect()["repository_id"]) != string(enabled.Data["repository_id"]) {
		t.Fatal("alias acquired distinct authority")
	}
	repo = physical
	// Linked worktrees share exactly the same common directory and epoch.
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "anchor")
	linked := filepath.Join(t.TempDir(), "linked")
	git("worktree", "add", "-q", "-b", "linked", linked)
	before = protected()
	repo = linked
	linkedView := inspect()
	if string(linkedView["repository_id"]) != string(enabled.Data["repository_id"]) || string(linkedView["enabled"]) != "true" {
		t.Fatal("linked worktree lost common binding")
	}
	// A forged reciprocal pointer refuses instead of borrowing common authority.
	gitdir, err := os.ReadFile(filepath.Join(linked, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	private := strings.TrimSpace(strings.TrimPrefix(string(gitdir), "gitdir:"))
	back := filepath.Join(private, "gitdir")
	backBytes, err := os.ReadFile(back)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(back, []byte(filepath.Join(physical, ".git")), 0600); err != nil {
		t.Fatal(err)
	}
	invoke("repository_error", "enable", "example.inventory/tool", "--grant", "native:issue=read", "--json")
	if err := os.WriteFile(back, backBytes, 0600); err != nil {
		t.Fatal(err)
	}
	repo = physical
	disabled := invoke("", "disable", "example.inventory/tool", "--json")
	if string(disabled.Data["enabled"]) != "false" || string(disabled.Data["epoch"]) == string(enabled.Data["epoch"]) {
		t.Fatal("disable did not revoke epoch")
	}
	if !reflect.DeepEqual(before, protected()) {
		t.Fatal("management changed signed objects, refs, identity or journals")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("management launched module")
	}
	// Re-enable rotates even an unchanged grant set, and removal revokes locally.
	reenabled := invoke("", "enable", "example.inventory/tool", "--grant", "native:issue=read", "--json")
	if string(reenabled.Data["epoch"]) == string(disabled.Data["epoch"]) {
		t.Fatal("enable reused epoch")
	}
	invoke("", "remove", "example.inventory/tool", "--json")
	reinstalled := register()
	if bytes.Equal(registered.Data["registration_id"], reinstalled.Data["registration_id"]) || string(inspect()["enabled"]) != "false" || string(inspect()["grants"]) != "[]" {
		t.Fatal("reinstall revived old grant")
	}
	// Stale executable enablement cannot be renewed or reported enabled.
	invoke("", "enable", "example.inventory/tool", "--grant", "native:issue=read", "--json")
	if err := os.WriteFile(module, []byte("#!/bin/sh\nexit 79\n"), 0700); err != nil {
		t.Fatal(err)
	}
	stale, _ := os.ReadFile(statePath)
	invoke("unavailable_module", "enable", "example.inventory/tool", "--grant", "native:issue=read", "--json")
	retained, _ := os.ReadFile(statePath)
	if !bytes.Equal(stale, retained) || string(inspect()["enabled"]) != "false" {
		t.Fatal("stale executable entitlement remained active")
	}
	// Malformed raw booleans and arrays never coerce to legitimate zero values.
	var state map[string]any
	if json.Unmarshal(stale, &state) != nil {
		t.Fatal("state")
	}
	for _, mutation := range []func(map[string]any){
		func(e map[string]any) { e["enabled"] = nil },
		func(e map[string]any) { e["grants"] = nil },
		func(e map[string]any) {
			e["grants"] = []any{map[string]any{"selector": "native:issue", "actions": []any{nil}}}
		},
	} {
		var candidate map[string]any
		json.Unmarshal(stale, &candidate)
		mutation(candidate["entries"].([]any)[0].(map[string]any))
		raw, _ := json.Marshal(candidate)
		if err := os.WriteFile(statePath, raw, 0600); err != nil {
			t.Fatal(err)
		}
		invoke("invalid_input", "inspect", "--json")
		invoke("invalid_input", "disable", "example.inventory/tool", "--json")
		kept, _ := os.ReadFile(statePath)
		if !bytes.Equal(raw, kept) {
			t.Fatal("malformed state repaired")
		}
	}
	if err := os.WriteFile(statePath, stale, 0600); err != nil {
		t.Fatal(err)
	}
	// A real native initialization proves the protected-state detector is live.
	control := exec.Command(binary, "init", "--name", "Detector")
	control.Dir, control.Env = repo, env
	if out, err := control.CombinedOutput(); err != nil {
		t.Fatalf("detector control: %v %s", err, out)
	}
	if reflect.DeepEqual(before, protected()) {
		t.Fatal("state detector missed real signer/native initialization")
	}
}
