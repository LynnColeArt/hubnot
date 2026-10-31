package extension

import (
	"bytes"
	"errors"
	"strings"
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
