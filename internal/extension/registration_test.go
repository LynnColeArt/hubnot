package extension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func registrationFixture(t *testing.T) (Registration, string) {
	t.Helper()
	dir := t.TempDir()
	executable := filepath.Join(dir, "module")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 77\n"), 0700); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRegistration(m, executable, []string{})
	if err != nil {
		t.Fatal(err)
	}
	return r, filepath.Join(t.TempDir(), "hubnot", "extensions", "registry.json")
}
func TestRegistrationIdentityAndLocalStorage(t *testing.T) {
	r, path := registrationFixture(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(r.Executable, link); err != nil {
		t.Fatal(err)
	}
	same, err := NewRegistration(r.Manifest, link, []string{})
	if err != nil || !reflect.DeepEqual(r, same) {
		t.Fatalf("physical executable binding: %v", err)
	}
	if r, err = Register(path, r); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Register(path, r); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("duplicate registration changed bytes")
	}
	for _, target := range []string{path, filepath.Dir(path), filepath.Dir(filepath.Dir(path))} {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0700)
		if target == path {
			want = 0600
		}
		if info.Mode().Perm() != want {
			t.Fatalf("mode %s: %v", target, info.Mode())
		}
	}
	changed := r
	changed.Argv = []string{"--secret"}
	changed.RegistrationID = changed.identity()
	_, err = Register(path, changed)
	expectCode(t, err, "invalid_input")
	after, _ = os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("changed registration overwrote old binding")
	}
	removed, err := Remove(path, r.Manifest.ModuleID)
	if err != nil || removed.RegistrationID != r.RegistrationID {
		t.Fatalf("remove: %v", err)
	}
	if _, err = Register(path, changed); err != nil {
		t.Fatal(err)
	}
	stored, err := LoadRegistry(path)
	if err != nil || stored.Registrations[0].RegistrationID == r.RegistrationID {
		t.Fatalf("new binding: %v", err)
	}
}
func TestRegistrationAvailabilityDoesNotRun(t *testing.T) {
	r, _ := registrationFixture(t)
	status, available, err := r.Availability()
	if err != nil {
		t.Fatal(err)
	}
	if status != "registered" || available != "not_probed" {
		t.Fatalf("%s %s", status, available)
	}
	if err := os.WriteFile(r.Executable, []byte("#!/bin/sh\nexit 99\n"), 0700); err != nil {
		t.Fatal(err)
	}
	status, available, err = r.Availability()
	if err != nil {
		t.Fatal(err)
	}
	if status != "changed" || available != "unavailable" {
		t.Fatalf("%s %s", status, available)
	}
	if err := os.Remove(r.Executable); err != nil {
		t.Fatal(err)
	}
	status, available, err = r.Availability()
	if err != nil {
		t.Fatal(err)
	}
	if status != "missing" || available != "unavailable" {
		t.Fatalf("%s %s", status, available)
	}
}
func TestRegistryStrictBoundsAndNoRepair(t *testing.T) {
	r, path := registrationFixture(t)
	if installed, err := Register(path, r); err != nil {
		t.Fatal(err)
	} else {
		r = installed
	}
	good, _ := os.ReadFile(path)
	for name, test := range map[string]struct {
		input []byte
		code  string
	}{
		"registrations casing": {[]byte(`{"protocol":"hn.extension-registry/1","Registrations":[]}`), "invalid_input"},
		"field casing":         {bytes.Replace(good, []byte(`"executable":`), []byte(`"Executable":`), 1), "invalid_input"},
		"duplicate":            {[]byte(`{"protocol":"hn.extension-registry/1","protocol":"hn.extension-registry/1","registrations":[]}`), "invalid_input"},
		"unknown":              {[]byte(`{"protocol":"hn.extension-registry/1","registrations":[],"extra":true}`), "invalid_input"},
		"null":                 {[]byte(`{"protocol":"hn.extension-registry/1","registrations":null}`), "invalid_input"},
		"corrupt digest":       {bytes.Replace(good, []byte(r.RegistrationID), []byte("sha256:"+strings.Repeat("0", 64)), 1), "verification_failure"},
		"oversize":             {bytes.Repeat([]byte{' '}, RegistryLimit+1), "resource_limit"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, test.input, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadRegistry(path)
			expectCode(t, err, test.code)
			expectCode(t, registrationError(path, r), test.code)
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(test.input, after) {
				t.Fatal("bad registry altered")
			}
		})
	}
	at := append(good, bytes.Repeat([]byte{' '}, RegistryLimit-len(good))...)
	if err := os.WriteFile(path, at, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(path); err != nil {
		t.Fatal(err)
	}
	registry := Registry{Protocol: "hn.extension-registry/1", Registrations: []Registration{}}
	for i := 0; i < 129; i++ {
		next := r
		next.Manifest.ModuleID = fmt.Sprintf("example.inventory/m%d", i)
		next.ManifestDigest = next.Manifest.Digest()
		next.RegistrationID = next.identity()
		registry.Registrations = append(registry.Registrations, next)
		if i == 127 || i == 128 {
			data, _ := json.Marshal(registry)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadRegistry(path)
			if i == 127 && err != nil {
				t.Fatal(err)
			}
			if i == 128 {
				expectCode(t, err, "resource_limit")
			}
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(r.Executable, path); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRegistry(path)
	expectCode(t, err, "invalid_input")
}
func TestRegistrationInputBounds(t *testing.T) {
	r, _ := registrationFixture(t)
	for _, tc := range []struct {
		name string
		argv []string
		code string
	}{
		{"count at", make([]string, 64), ""}, {"count over", make([]string, 65), "resource_limit"},
		{"item at", []string{strings.Repeat("x", 4096)}, ""}, {"item over", []string{strings.Repeat("x", 4097)}, "resource_limit"},
		{"total at", repeatArg(16, 4096), ""}, {"total over", append(repeatArg(16, 4096), "x"), "resource_limit"},
		{"NUL", []string{"a\x00b"}, "invalid_input"}, {"UTF8", []string{string([]byte{255})}, "invalid_input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRegistration(r.Manifest, r.Executable, tc.argv)
			if tc.code != "" {
				expectCode(t, err, tc.code)
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
	f, err := os.OpenFile(r.Executable, os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(ExecutableLimit); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := NewRegistration(r.Manifest, r.Executable, []string{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(r.Executable, ExecutableLimit+1); err != nil {
		t.Fatal(err)
	}
	_, err = NewRegistration(r.Manifest, r.Executable, []string{})
	expectCode(t, err, "resource_limit")
	_, _, err = r.Availability()
	expectCode(t, err, "resource_limit")
	if err := os.Chmod(r.Executable, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = NewRegistration(r.Manifest, r.Executable, []string{})
	expectCode(t, err, "unavailable_module")
}
func repeatArg(count, size int) []string {
	result := []string{}
	for i := 0; i < count; i++ {
		result = append(result, strings.Repeat("x", size))
	}
	return result
}

func TestStoredIncompatibleManifestRemainsInert(t *testing.T) {
	r, path := registrationFixture(t)
	r.RegistrationInstance = "00000000000000000000000000000001"
	r.Manifest.HostProtocol = "hn.extension/2"
	r.ManifestDigest = r.Manifest.Digest()
	r.RegistrationID = r.identity()
	if _, err := Register(path, r); err == nil {
		t.Fatal("new incompatible registration accepted")
	} else {
		expectCode(t, err, "incompatible_version")
	}
	registry := Registry{Protocol: "hn.extension-registry/1", Registrations: []Registration{r}}
	data, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	status, availability, err := stored.Registrations[0].Availability()
	if err != nil {
		t.Fatal(err)
	}
	if status != "incompatible" || availability != "not_probed" {
		t.Fatalf("%s %s", status, availability)
	}
}

func TestRegistryUpdateBoundsRefuseBeforeReplacement(t *testing.T) {
	r, path := registrationFixture(t)
	r.RegistrationInstance = "00000000000000000000000000000001"
	r.RegistrationID = r.identity()
	registry := Registry{Protocol: "hn.extension-registry/1", Registrations: []Registration{}}
	for i := 0; i < 128; i++ {
		next := r
		next.Manifest.ModuleID = fmt.Sprintf("example.inventory/m%d", i)
		next.ManifestDigest = next.Manifest.Digest()
		next.RegistrationID = next.identity()
		registry.Registrations = append(registry.Registrations, next)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	before := canonical(registry)
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	expectCode(t, registrationError(path, r), "resource_limit")
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("entry overflow replaced registry")
	}
	registry.Registrations = registry.Registrations[:15]
	for i := range registry.Registrations {
		registry.Registrations[i].Argv = repeatArg(16, 4096)
		registry.Registrations[i].RegistrationID = registry.Registrations[i].identity()
	}
	before = canonical(registry)
	if len(before) > RegistryLimit {
		t.Fatal("invalid at-bound fixture")
	}
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	r.Argv = repeatArg(16, 4096)
	r.RegistrationID = r.identity()
	expectCode(t, registrationError(path, r), "resource_limit")
	after, _ = os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("byte overflow replaced registry")
	}
}
func TestRegistryRejectsDirectoryAndLockLinks(t *testing.T) {
	r, path := registrationFixture(t)
	root := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(root, "hubnot")); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRegistry(path)
	expectCode(t, err, "invalid_input")
	expectCode(t, registrationError(path, r), "invalid_input")
	if err := os.Remove(filepath.Join(root, "hubnot")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(r.Executable, filepath.Join(filepath.Dir(path), "registry.lock")); err != nil {
		t.Fatal(err)
	}
	expectCode(t, registrationError(path, r), "repository_error")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock link caused registry write: %v", err)
	}
}

func registrationError(path string, r Registration) error { _, err := Register(path, r); return err }

func TestRegistrationInstanceEntropyAndRemovedBinding(t *testing.T) {
	candidate, path := registrationFixture(t)
	noEntropy := strings.NewReader("")
	_, err := registerWithRandom(path, candidate, noEntropy)
	expectCode(t, err, "repository_error")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("entropy failure replaced registry: %v", err)
	}
	first, err := registerWithRandom(path, candidate, bytes.NewReader(bytes.Repeat([]byte{1}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := registerWithRandom(path, candidate, strings.NewReader(""))
	if err != nil || !reflect.DeepEqual(first, duplicate) {
		t.Fatalf("duplicate drew entropy or changed stored registration: %v", err)
	}
	other := candidate
	other.Manifest.ModuleID = "example.inventory/other"
	other.ManifestDigest = other.Manifest.Digest()
	_, err = registerWithRandom(path, other, strings.NewReader(""))
	expectCode(t, err, "repository_error")
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("entropy failure changed existing registry")
	}
	if _, err := Remove(path, candidate.Manifest.ModuleID); err != nil {
		t.Fatal(err)
	}
	// Supplying a removed instance does not choose the next lifecycle identity.
	second, err := registerWithRandom(path, first, bytes.NewReader(bytes.Repeat([]byte{2}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	if first.RegistrationID == second.RegistrationID || first.RegistrationInstance == second.RegistrationInstance {
		t.Fatal("remove/re-register revived old registration binding")
	}
	oldDerivation := func(r Registration) string {
		return digest(canonical(map[string]any{"argv": r.Argv, "executable": r.Executable, "executable_digest": r.ExecutableDigest, "manifest_digest": r.ManifestDigest}))
	}
	if oldDerivation(first) != oldDerivation(second) {
		t.Fatal("fault control did not reproduce old deterministic lifecycle collision")
	}
	t.Log("exact-old-derivation fault control collapses removed/new identities; actual instance-bound IDs differ (not a pre-fix red run)")
}
func TestRegistryRejectsMissingMalformedAndMismatchedInstance(t *testing.T) {
	candidate, path := registrationFixture(t)
	installed, err := Register(path, candidate)
	if err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		input []byte
		code  string
	}{
		"missing":    {bytes.Replace(good, []byte(`,"registration_instance":"`+installed.RegistrationInstance+`"`), nil, 1), "invalid_input"},
		"malformed":  {bytes.Replace(good, []byte(installed.RegistrationInstance), []byte("not-a-hex-instance"), 1), "verification_failure"},
		"mismatched": {bytes.Replace(good, []byte(installed.RegistrationInstance), []byte(strings.Repeat("0", 32)), 1), "verification_failure"},
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(good, tc.input) {
				t.Fatal("invalid instance control did not change bytes")
			}
			if err := os.WriteFile(path, tc.input, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadRegistry(path)
			expectCode(t, err, tc.code)
			_, err = Register(path, candidate)
			expectCode(t, err, tc.code)
			after, _ := os.ReadFile(path)
			if !bytes.Equal(after, tc.input) {
				t.Fatal("invalid instance was repaired")
			}
		})
	}
}

func TestRegistryRawArrayTypesPreserveMalformedBytes(t *testing.T) {
	candidate, path := registrationFixture(t)
	candidate.Argv = []string{""}
	installed, err := Register(path, candidate)
	if err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
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
			_, err = LoadRegistry(path)
			expectCode(t, err, "invalid_input")
			_, err = Register(path, candidate)
			expectCode(t, err, "invalid_input")
			_, err = Remove(path, installed.Manifest.ModuleID)
			expectCode(t, err, "invalid_input")
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, malformed) {
				t.Fatalf("malformed array repaired: %v", err)
			}
		})
	}
	if err := os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	duplicate, err := registerWithRandom(path, candidate, strings.NewReader(""))
	if err != nil || !reflect.DeepEqual(duplicate, installed) {
		t.Fatalf("valid empty argument changed or drew entropy: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, good) {
		t.Fatalf("valid empty argument changed bytes: %v", err)
	}
}
