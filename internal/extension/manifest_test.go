package extension

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

const validManifest = `{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["resource.read"]}`

func expectCode(t *testing.T, err error, code string) {
	t.Helper()
	var public *Error
	if !errors.As(err, &public) || public.Code != code {
		t.Fatalf("want %s; got %v", code, err)
	}
}
func TestManifestStrictPublicContract(t *testing.T) {
	manifest, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	reordered := ` {"capabilities":["resource.read"],"host_protocol":"hn.extension/1","module_version":"1","module_id":"example.inventory/tool","protocol":"hn.extension-manifest/1"} `
	second, err := ParseManifest([]byte(reordered))
	if err != nil || manifest.Digest() != second.Digest() {
		t.Fatalf("canonical digest changed: %v", err)
	}
	for name, input := range map[string]string{
		"duplicate":              strings.Replace(validManifest, `"module_version":"1"`, `"module_version":"1","module_version":"2"`, 1),
		"unknown":                strings.TrimSuffix(validManifest, "}") + `,"executable":"/untrusted"}`,
		"missing":                strings.Replace(validManifest, `"module_version":"1",`, "", 1),
		"null capabilities":      strings.Replace(validManifest, `["resource.read"]`, `null`, 1),
		"duplicate capability":   strings.Replace(validManifest, `["resource.read"]`, `["resource.read","resource.read"]`, 1),
		"unsupported capability": strings.Replace(validManifest, `resource.read`, `runner`, 1),
		"reserved":               strings.Replace(validManifest, `example.inventory/tool`, `hn/tool`, 1),
		"invalid ID":             strings.Replace(validManifest, `example.inventory/tool`, `../tool`, 1),
		"unicode replacement":    strings.Replace(validManifest, `"1"`, `"\ud800"`, 1),
		"invalid UTF8":           validManifest + string([]byte{0xff}),
		"trailing":               validManifest + ` {}`,
		"field casing":           strings.Replace(validManifest, `"protocol":`, `"Protocol":`, 1),
		"nonobject":              `[]`,
		"long version":           strings.Replace(validManifest, `"1"`, `"`+strings.Repeat("v", 65)+`"`, 1),
		"whitespace version":     strings.Replace(validManifest, `"1"`, `"v 1"`, 1),
	} {
		t.Run(name, func(t *testing.T) { _, err := ParseManifest([]byte(input)); expectCode(t, err, "invalid_input") })
	}
	for _, field := range []string{"hn.extension-manifest/1", "hn.extension/1"} {
		_, err := ParseManifest([]byte(strings.Replace(validManifest, field, field+"-unknown", 1)))
		expectCode(t, err, "incompatible_version")
	}
	_, err = ParseManifest(bytes.Repeat([]byte{' '}, ManifestLimit+1))
	expectCode(t, err, "resource_limit")
	at := append([]byte(validManifest), bytes.Repeat([]byte{' '}, ManifestLimit-len(validManifest))...)
	if _, err = ReadManifest(bytes.NewReader(at)); err != nil {
		t.Fatal(err)
	}
	_, err = ReadManifest(bytes.NewReader(append(at, ' ')))
	expectCode(t, err, "resource_limit")
	for _, version := range []string{strings.Repeat("v", 64), `\ud83d\ude00`} {
		input := strings.Replace(validManifest, `"1"`, `"`+version+`"`, 1)
		_, err = ParseManifest([]byte(input))
		if strings.HasPrefix(version, "v") {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			expectCode(t, err, "invalid_input")
		}
	}
}

// A fresh process isolates startup from other tests that validate manifests.
// This guards the default-path cost without resetting shared production state.
func TestModulePatternLazyConcurrentFirstUse(t *testing.T) {
	const child = "HN_WP13_MODULE_PATTERN_CHILD"
	if os.Getenv(child) != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestModulePatternLazyConcurrentFirstUse$", "-test.v")
		command.Env = append(os.Environ(), child+"=1")
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fresh-process validator check: %v\n%s", err, out)
		} else {
			t.Logf("fresh-process validator check:\n%s", out)
		}
		return
	}
	if modulePattern != nil {
		t.Fatal("module validator constructed before optional validation")
	}
	_, err := ParseManifest([]byte(`{}`))
	expectCode(t, err, "invalid_input")
	if modulePattern != nil {
		t.Fatal("structural refusal unnecessarily constructed module validator")
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 64; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			id, want := "example.inventory/tool", true
			if i%2 != 0 {
				id, want = "hn/tool", false
			}
			if got := ValidModuleID(id); got != want {
				t.Errorf("concurrent ValidModuleID(%q) = %v, want %v", id, got, want)
			}
		}(i)
	}
	close(start)
	workers.Wait()
	if modulePattern == nil {
		t.Fatal("optional validation did not construct module validator")
	}
}

func TestModuleIDFrozenAcceptance(t *testing.T) {
	for _, example := range []struct {
		id    string
		valid bool
	}{
		{"a/b", true}, {"example.inventory/tool", true}, {"a.-09/b.-09", true},
		{strings.Repeat("a", 64) + "/" + strings.Repeat("b", 64), true},
		{"hnx/tool", true}, {"hubnotx/tool", true},
		{"", false}, {"a", false}, {"a/b/c", false}, {"/b", false}, {"a/", false},
		{"1a/b", false}, {"a/1b", false}, {"A/b", false}, {"a/B", false},
		{"a_b/c", false}, {"a/b_c", false}, {"é/b", false}, {"a/é", false},
		{" a/b", false}, {"a/b ", false}, {"a/b\n", false}, {"a/b\x00", false},
		{"hn/tool", false}, {"hubnot/tool", false},
		{strings.Repeat("a", 65) + "/b", false}, {"a/" + strings.Repeat("b", 65), false},
	} {
		if got := ValidModuleID(example.id); got != example.valid {
			t.Errorf("ValidModuleID(%q) = %v, want %v", example.id, got, example.valid)
		}
	}
	manifest, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := manifest.Digest(), "sha256:36c9e4de174f41cdba88f5ccb0f1641014f6a346d7876d98816d0d81aee1859b"; got != want {
		t.Fatalf("frozen manifest digest = %s, want %s", got, want)
	}
	_, err = ParseManifest([]byte(strings.Replace(validManifest, "example.inventory/tool", "hubnot/tool", 1)))
	var public *Error
	if !errors.As(err, &public) || public.Origin != "host" || public.Code != "invalid_input" || public.Message != "invalid or reserved module ID" || public.Recovery != "do_not_retry" || public.Operation != nil {
		t.Fatalf("frozen module refusal changed: %#v", err)
	}
}
