package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func extensionTestBinary(t *testing.T) string {
	t.Helper()
	if supplied := os.Getenv("HN_WP13_BINARY"); supplied != "" {
		return supplied
	}
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "hn")
	git := exec.Command("git", "rev-parse", "--absolute-git-dir")
	raw, err := git.Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Env = append(os.Environ(), "GIT_DIR="+strings.TrimSpace(string(raw)), "GIT_WORK_TREE="+source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	t.Logf("actual CLI binary SHA256 %x", sum)
	return binary
}

type extensionCLIResult struct {
	OK    bool                       `json:"ok"`
	Data  map[string]json.RawMessage `json:"data"`
	Error struct {
		Origin   string `json:"origin"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		Recovery string `json:"recovery"`
	} `json:"error"`
}

func extensionTestConfigDir(t *testing.T, home string) string {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func TestExtensionRegistrationCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	home := t.TempDir()
	dir := t.TempDir()
	config := extensionTestConfigDir(t, home)
	environment := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	invoke := func(wantCode string, args ...string) extensionCLIResult {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = dir
		cmd.Env = environment
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		var result extensionCLIResult
		if json.Unmarshal(stdout.Bytes(), &result) != nil {
			t.Fatalf("%v: expected JSON; err=%v stdout=%s stderr=%s", args, err, &stdout, &stderr)
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || len(envelope) != 3 || envelope["ok"] == nil || envelope["data"] == nil || envelope["error"] == nil {
			t.Fatalf("management envelope: %s", &stdout)
		}
		if wantCode == "" {
			if err != nil || !result.OK {
				t.Fatalf("%v: %v stdout=%s stderr=%s", args, err, &stdout, &stderr)
			}
		} else if err == nil || result.OK || result.Error.Code != wantCode || result.Error.Recovery != "do_not_retry" || result.Error.Origin != "host" || result.Data != nil {
			t.Fatalf("%v want %s; err=%v stdout=%s stderr=%s", args, wantCode, err, &stdout, &stderr)
		}
		if wantCode != "" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(envelope["error"], &fields); err != nil || len(fields) != 5 || string(fields["operation"]) != "null" {
				t.Fatalf("error shape: %s", envelope["error"])
			}
		} else if string(envelope["error"]) != "null" {
			t.Fatal("success has non-null error")
		}
		if strings.Contains(stdout.String(), dir) || strings.Contains(stdout.String(), "PRIVATE_ARGV_MARKER") {
			t.Fatalf("private registration details leaked: %s", &stdout)
		}
		return result
	}
	native := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = dir
		cmd.Env = environment
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("native %v: %v %s", args, err, out)
		}
		if strings.Contains(string(out), "PRIVATE_STATE_MARKER") {
			t.Fatal("native command leaked extension private state")
		}
		return string(out)
	}
	git := exec.Command("git", "init", "-q", dir)
	git.Env = environment
	if out, err := git.CombinedOutput(); err != nil {
		t.Fatalf("init Git: %v %s", err, out)
	}
	native("init", "--name", "Extension Test")
	manifest := filepath.Join(dir, "manifest.json")
	valid := `{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["resource.read"]}`
	if err := os.WriteFile(manifest, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(dir, "module")
	marker := filepath.Join(dir, "started")
	if err := os.WriteFile(module, []byte("#!/bin/sh\nprintf forbidden > '"+marker+"'\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	initial := extensionPrivateSnapshot(t, dir)
	register := []string{"extension", "register", "--manifest", manifest, "--executable", module, "--arg", "PRIVATE_ARGV_MARKER", "--json"}
	first := invoke("", register...)
	if string(first.Data["status"]) != `"registered"` || string(first.Data["enabled"]) != "false" || len(first.Data) != 7 {
		t.Fatalf("registration success shape: %v", first.Data)
	}
	var manifestValue map[string]any
	if err := json.Unmarshal([]byte(valid), &manifestValue); err != nil {
		t.Fatal(err)
	}
	canonicalManifest, err := json.Marshal(manifestValue)
	if err != nil {
		t.Fatal(err)
	}
	executableBytes, err := os.ReadFile(module)
	if err != nil {
		t.Fatal(err)
	}
	for key, expected := range map[string]string{"manifest_digest": extensionCLIDigest(canonicalManifest), "executable_digest": extensionCLIDigest(executableBytes)} {
		var actual string
		if json.Unmarshal(first.Data[key], &actual) != nil || actual != expected {
			t.Fatalf("independent %s digest: %s want %s", key, first.Data[key], expected)
		}
	}
	duplicate := invoke("", register...)
	if !reflect.DeepEqual(first.Data, duplicate.Data) {
		t.Fatal("duplicate registration changed identity")
	}
	inspection := invoke("", "extension", "inspect", "example.inventory/tool", "--json")
	var modules []map[string]json.RawMessage
	if err := json.Unmarshal(inspection.Data["modules"], &modules); err != nil || len(modules) != 1 {
		t.Fatalf("inspection: %v %v", modules, err)
	}
	if len(modules[0]) != 11 || string(modules[0]["availability"]) != `"not_probed"` || string(modules[0]["status"]) != `"registered"` || string(modules[0]["enabled"]) != "false" || string(modules[0]["grants"]) != "[]" {
		t.Fatalf("inspection contract %v", modules)
	}
	var unsupported []map[string]string
	json.Unmarshal(inspection.Data["unsupported"], &unsupported)
	if len(unsupported) != 13 {
		t.Fatalf("unsupported inventory: %v", unsupported)
	}
	registry := filepath.Join(config, "hubnot", "extensions", "registry.json")
	before, err := os.ReadFile(registry)
	if err != nil {
		t.Fatal(err)
	}
	decodeRegistry := func() map[string]any {
		t.Helper()
		var value map[string]any
		if err := json.Unmarshal(before, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	stored := decodeRegistry()["registrations"].([]any)[0].(map[string]any)
	instance := stored["registration_instance"].(string)
	instanceBytes, err := hex.DecodeString(instance)
	if err != nil || len(instanceBytes) != 16 || strings.ToLower(instance) != instance {
		t.Fatalf("private instance grammar: %q", instance)
	}
	identityFields := map[string]any{"argv": stored["argv"], "executable": stored["executable"], "executable_digest": stored["executable_digest"], "manifest_digest": stored["manifest_digest"], "registration_instance": instance}
	identityBytes, err := json.Marshal(identityFields)
	if err != nil {
		t.Fatal(err)
	}
	if stored["registration_id"] != extensionCLIDigest(identityBytes) {
		t.Fatal("independent instance-bound registration ID mismatch")
	}
	for _, bad := range []struct{ name, code string }{{"missing", "invalid_input"}, {"malformed", "verification_failure"}, {"mismatched", "verification_failure"}} {
		modified := decodeRegistry()
		reg := modified["registrations"].([]any)[0].(map[string]any)
		switch bad.name {
		case "missing":
			delete(reg, "registration_instance")
		case "malformed":
			reg["registration_instance"] = "BAD"
		case "mismatched":
			replacement := strings.Repeat("0", 32)
			if replacement == instance {
				replacement = strings.Repeat("f", 32)
			}
			reg["registration_instance"] = replacement
		}
		raw, err := json.Marshal(modified)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(registry, raw, 0600); err != nil {
			t.Fatal(err)
		}
		invoke(bad.code, "extension", "inspect", "--json")
		invoke(bad.code, register...)
		retained, err := os.ReadFile(registry)
		if err != nil || !bytes.Equal(retained, raw) {
			t.Fatalf("invalid instance repaired: %v", err)
		}
	}
	incompatible := decodeRegistry()
	future := incompatible["registrations"].([]any)[0].(map[string]any)
	futureManifest := future["manifest"].(map[string]any)
	futureManifest["host_protocol"] = "hn.extension/2"
	futureManifestBytes, _ := json.Marshal(futureManifest)
	future["manifest_digest"] = extensionCLIDigest(futureManifestBytes)
	futureIdentity := map[string]any{"argv": future["argv"], "executable": future["executable"], "executable_digest": future["executable_digest"], "manifest_digest": future["manifest_digest"], "registration_instance": future["registration_instance"]}
	futureIdentityBytes, _ := json.Marshal(futureIdentity)
	future["registration_id"] = extensionCLIDigest(futureIdentityBytes)
	futureBytes, _ := json.Marshal(incompatible)
	if err := os.WriteFile(registry, futureBytes, 0600); err != nil {
		t.Fatal(err)
	}
	futureInspection := invoke("", "extension", "inspect", "--json")
	var futureModules []map[string]json.RawMessage
	json.Unmarshal(futureInspection.Data["modules"], &futureModules)
	if len(futureModules) != 1 || string(futureModules[0]["compatible"]) != "false" || string(futureModules[0]["status"]) != `"incompatible"` || string(futureModules[0]["availability"]) != `"not_probed"` {
		t.Fatalf("incompatible inspection: %v", futureModules)
	}
	if err := os.WriteFile(registry, before, 0600); err != nil {
		t.Fatal(err)
	}
	changed := append([]string{}, register...)
	changed[len(changed)-2] = "changed"
	invoke("invalid_input", changed...)
	after, _ := os.ReadFile(registry)
	if !bytes.Equal(before, after) {
		t.Fatal("refused changed registration overwrote state")
	}
	if err := os.WriteFile(manifest, []byte(strings.TrimSuffix(valid, "}")+`,"executable":"cloned"}`), 0600); err != nil {
		t.Fatal(err)
	}
	invoke("invalid_input", register...)
	if err := os.WriteFile(manifest, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(strings.Replace(valid, `"host_protocol":"hn.extension/1"`, `"host_protocol":"hn.extension/2"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	invoke("incompatible_version", register...)
	after, _ = os.ReadFile(registry)
	if !bytes.Equal(before, after) {
		t.Fatal("incompatible registration changed private registry")
	}
	if err := os.WriteFile(manifest, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	invoke("not_found", "extension", "inspect", "example.inventory/missing", "--json")
	// A retained enablement in an independent repository names the removed ID.
	secondRepo := t.TempDir()
	secondGit := exec.Command("git", "init", "-q", secondRepo)
	secondGit.Env = environment
	if out, err := secondGit.CombinedOutput(); err != nil {
		t.Fatalf("second repository: %v %s", err, out)
	}
	var oldID string
	if json.Unmarshal(first.Data["registration_id"], &oldID) != nil {
		t.Fatal("registration ID")
	}
	staleEntry := map[string]any{"protocol": "hn.extension-enablement/1", "entries": []any{map[string]any{"registration_id": oldID, "repository_id": extensionCLIDigest([]byte("hn-extension-repository-v1\x00" + filepath.Join(secondRepo, ".git"))), "epoch": "00000000000000000000000000000001", "enabled": true, "grants": []any{map[string]any{"selector": "native:issue", "actions": []string{"read"}}}}}}
	stalePath := filepath.Join(secondRepo, ".git", "hn", "extensions", "enabled.json")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0700); err != nil {
		t.Fatal(err)
	}
	staleBytes, err := json.Marshal(staleEntry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stalePath, staleBytes, 0600); err != nil {
		t.Fatal(err)
	}
	invoke("", "extension", "remove", "example.inventory/tool", "--json")
	reinstalled := invoke("", register...)
	if bytes.Equal(first.Data["registration_id"], reinstalled.Data["registration_id"]) {
		t.Fatal("identical reinstall reused removed lifecycle ID")
	}
	originalDir := dir
	dir = secondRepo
	secondInspection := invoke("", "extension", "inspect", "example.inventory/tool", "--json")
	dir = originalDir
	var secondModules []map[string]json.RawMessage
	json.Unmarshal(secondInspection.Data["modules"], &secondModules)
	if len(secondModules) != 1 || string(secondModules[0]["enabled"]) != "false" || bytes.Equal(secondModules[0]["registration_id"], first.Data["registration_id"]) {
		t.Fatalf("retained old grant binding matched new registration: %v", secondModules)
	}
	retained, err := os.ReadFile(stalePath)
	if err != nil || !bytes.Equal(retained, staleBytes) {
		t.Fatalf("registration searched/rewrote independent repository state: %v", err)
	}
	t.Log("cross-repository retained ENTRY names old ID; fresh registration reports disabled with a distinct ID; downstream enable/run enforcement remains WP14/WP16")
	if err := os.WriteFile(module, []byte("#!/bin/sh\nexit 79\n"), 0700); err != nil {
		t.Fatal(err)
	}
	inspection = invoke("", "extension", "inspect", "--json")
	json.Unmarshal(inspection.Data["modules"], &modules)
	if string(modules[0]["status"]) != `"changed"` || string(modules[0]["availability"]) != `"unavailable"` {
		t.Fatalf("changed executable inspection %v", modules)
	}
	if err := os.Remove(module); err != nil {
		t.Fatal(err)
	}
	inspection = invoke("", "extension", "inspect", "--json")
	json.Unmarshal(inspection.Data["modules"], &modules)
	if string(modules[0]["status"]) != `"missing"` {
		t.Fatalf("missing executable inspection %v", modules)
	}
	removed := invoke("", "extension", "remove", "example.inventory/tool", "--json")
	if len(removed.Data) != 5 || string(removed.Data["public_data_retained"]) != "true" || string(removed.Data["stopped_sessions"]) != "[]" {
		t.Fatalf("remove shape: %v", removed)
	}
	if !reflect.DeepEqual(initial, extensionPrivateSnapshot(t, dir)) {
		t.Fatal("extension management modified repository data, keys, objects or refs")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("management started module: %v", err)
	}
	// An actual invocation is the negative process detector control.
	if err := os.WriteFile(module, []byte("#!/bin/sh\nprintf forbidden > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	control := exec.Command(module)
	if out, err := control.CombinedOutput(); err != nil {
		t.Fatalf("process detector control: %v %s", err, out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("process detector did not trigger: %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	// Malformed real extension paths cannot change ordinary native journeys.
	for _, path := range []string{registry, filepath.Join(dir, ".git", "hn", "extensions", "enabled.json"), filepath.Join(dir, ".hn", "extensions.json")} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("malformed PRIVATE_STATE_MARKER"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	protected := extensionPrivateSnapshot(t, dir)
	native("version")
	native("help")
	native("identity", "show", "--json")
	native("issue", "list", "--json")
	if !reflect.DeepEqual(protected, extensionPrivateSnapshot(t, dir)) {
		t.Fatal("native reads changed extension/private/public state")
	}
	invoke("invalid_input", "extension", "inspect", "--json")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("native command started module: %v", err)
	}
}
func extensionPrivateSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, ".hn")} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			files[strings.TrimPrefix(path, root)] = hex.EncodeToString(sum[:])
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// Trace tests use the actual OS file/process boundary. No source interception
// substitutes for observing both absolute and relative path reads.
func TestExtensionNativeReadDetector(t *testing.T) {
	strace, err := exec.LookPath("strace")
	if err != nil {
		t.Skip("OS syscall trace unavailable; native effect checks remain in CLI test")
	}
	binary := extensionTestBinary(t)
	home := t.TempDir()
	dir := t.TempDir()
	config := extensionTestConfigDir(t, home)
	registry := filepath.Join(config, "hubnot", "extensions", "registry.json")
	if err := os.MkdirAll(filepath.Dir(registry), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry, []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	environment := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	for _, args := range [][]string{{"git", "init", "-q", dir}, {binary, "init", "--name", "Trace Test"}} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = environment
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("trace fixture setup: %v %s", err, out)
		}
	}
	for _, path := range []string{filepath.Join(dir, ".git", "hn", "extensions", "enabled.json"), filepath.Join(dir, ".hn", "extensions.json")} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("malformed"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	trace := func(args ...string) string {
		t.Helper()
		log := filepath.Join(t.TempDir(), "trace")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, strace, append([]string{"-f", "-qq", "-e", "trace=%file,process", "-o", log}, args...)...)
		command.Dir = dir
		command.Env = operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
		out, runErr := command.CombinedOutput()
		if len(args) > 2 && args[0] == binary && args[1] == "extension" && args[2] == "inspect" {
			if runErr == nil || !bytes.Contains(out, []byte(`"code":"invalid_input"`)) {
				t.Fatalf("explicit malformed-registry control: %v %s", runErr, out)
			}
		} else if runErr != nil {
			t.Fatalf("trace command %v: %v %s", args, runErr, out)
		}
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	detector := func(log string) bool {
		return strings.Contains(log, "registry.json") || strings.Contains(log, "enabled.json") || strings.Contains(log, "extensions.json")
	}
	for _, args := range [][]string{{binary, "version"}, {binary, "help"}, {binary, "extension", "help"}, {binary, "identity", "show", "--json"}, {binary, "issue", "list", "--json"}} {
		if log := trace(args...); detector(log) {
			t.Fatalf("ordinary startup read registry: %s", log)
		}
	}
	for _, target := range []string{registry, filepath.Join(dir, ".git", "hn", "extensions", "enabled.json"), filepath.Join(dir, ".hn", "extensions.json")} {
		relative, err := filepath.Rel(dir, target)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{target, relative} {
			if log := trace("cat", path); !detector(log) {
				t.Fatalf("read detector missed %q: %s", path, log)
			}
		}
	}
	if log := trace(binary, "extension", "inspect", "--json"); !detector(log) {
		t.Fatalf("explicit inspection did not exercise real registry path: %s", log)
	}
}

func extensionCLIDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestExtensionRegistryRawArrayTypesCLI(t *testing.T) {
	binary := extensionTestBinary(t)
	home := t.TempDir()
	dir := t.TempDir()
	config := extensionTestConfigDir(t, home)
	env := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	git := exec.Command("git", "init", "-q", dir)
	git.Env = env
	if out, err := git.CombinedOutput(); err != nil {
		t.Fatalf("fixture git: %v %s", err, out)
	}
	manifest := filepath.Join(dir, "manifest.json")
	module := filepath.Join(dir, "module")
	marker := filepath.Join(dir, "module-started")
	if err := os.WriteFile(manifest, []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["resource.read"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(module, []byte("#!/bin/sh\nprintf forbidden > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	invoke := func(t *testing.T, wantCode string, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = dir
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		var envelope struct {
			OK    bool            `json:"ok"`
			Data  json.RawMessage `json:"data"`
			Error *struct {
				Origin    string          `json:"origin"`
				Code      string          `json:"code"`
				Recovery  string          `json:"recovery"`
				Operation json.RawMessage `json:"operation"`
			} `json:"error"`
		}
		if decodeErr := json.Unmarshal(stdout.Bytes(), &envelope); decodeErr != nil {
			t.Fatalf("%v: invalid JSON: %v %s %s", args, decodeErr, &stdout, &stderr)
		}
		if wantCode == "" {
			if err != nil || !envelope.OK || envelope.Error != nil {
				t.Fatalf("%v: valid string array failed: %v %s %s", args, err, &stdout, &stderr)
			}
		} else if err == nil || envelope.OK || string(envelope.Data) != "null" || envelope.Error == nil || envelope.Error.Origin != "host" || envelope.Error.Code != wantCode || envelope.Error.Recovery != "do_not_retry" || string(envelope.Error.Operation) != "null" {
			t.Fatalf("%v: want host/%s with unchanged malformed bytes; err=%v stdout=%s stderr=%s", args, wantCode, err, &stdout, &stderr)
		}
	}
	register := []string{"extension", "register", "--manifest", manifest, "--executable", module, "--arg", "", "--json"}
	before := extensionPrivateSnapshot(t, dir)
	invoke(t, "", register...)
	path := filepath.Join(config, "hubnot", "extensions", "registry.json")
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	invoke(t, "", "extension", "inspect", "example.inventory/tool", "--json")
	invoke(t, "", register...)
	positive, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(positive, good) {
		t.Fatalf("valid empty argument duplicate changed bytes: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"null argv element":    func(value map[string]any) { value["registrations"].([]any)[0].(map[string]any)["argv"] = []any{nil} },
		"boolean argv element": func(value map[string]any) { value["registrations"].([]any)[0].(map[string]any)["argv"] = []any{true} },
		"number argv element":  func(value map[string]any) { value["registrations"].([]any)[0].(map[string]any)["argv"] = []any{2} },
		"object argv element": func(value map[string]any) {
			value["registrations"].([]any)[0].(map[string]any)["argv"] = []any{map[string]any{}}
		},
		"array argv element": func(value map[string]any) {
			value["registrations"].([]any)[0].(map[string]any)["argv"] = []any{[]any{}}
		},
		"null argv array": func(value map[string]any) { value["registrations"].([]any)[0].(map[string]any)["argv"] = nil },
		"null capabilities array": func(value map[string]any) {
			value["registrations"].([]any)[0].(map[string]any)["manifest"].(map[string]any)["capabilities"] = nil
		},
		"null registrations array": func(value map[string]any) { value["registrations"] = nil },
	} {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(good, &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			malformed, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, malformed, 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"extension", "inspect", "example.inventory/tool", "--json"}, register, {"extension", "remove", "example.inventory/tool", "--json"}} {
				invoke(t, "invalid_input", args...)
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(after, malformed) {
					t.Fatalf("%v repaired malformed array bytes: %v", args, err)
				}
			}
		})
	}
	if !reflect.DeepEqual(before, extensionPrivateSnapshot(t, dir)) {
		t.Fatal("array refusal modified repository keys, public objects, refs or private state")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("array refusal launched module: %v", err)
	}
}
