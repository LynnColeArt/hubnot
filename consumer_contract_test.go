package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConsumerStaticDiscoveryCLI(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "default"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			binary := runnerAcceptanceBinary(t, native)
			root := t.TempDir()
			work := filepath.Join(root, "work")
			home := filepath.Join(root, "home")
			shim := filepath.Join(root, "bin")
			for _, p := range []string{work, home, shim} {
				if err := os.MkdirAll(p, 0700); err != nil {
					t.Fatal(err)
				}
			}
			marker := filepath.Join(root, "git-called")
			if err := os.WriteFile(filepath.Join(shim, "git"), []byte("#!/bin/sh\nprintf called > '"+marker+"'\nexit 91\n"), 0700); err != nil {
				t.Fatal(err)
			}
			// Malformed authority/identity sentinels make accidental loading observable.
			sentinel := filepath.Join(home, ".config", "hubnot", "identity.json")
			if err := os.MkdirAll(filepath.Dir(sentinel), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sentinel, []byte("PRIVATE_MALFORMED_IDENTITY"), 0600); err != nil {
				t.Fatal(err)
			}
			before := consumerFiles(t, root)
			for _, args := range [][]string{{"version", "--json"}, {"--version", "--json"}, {"capabilities", "--json"}} {
				cmd := exec.Command(binary, args...)
				cmd.Dir = work
				cmd.Env = operationalTestEnvironmentWith(home, map[string]string{"PATH": shim, "XDG_CONFIG_HOME": filepath.Join(home, ".config")})
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				out, err := cmd.Output()
				var result struct {
					OK    bool                       `json:"ok"`
					Data  map[string]json.RawMessage `json:"data"`
					Error any                        `json:"error"`
				}
				if err != nil || json.Unmarshal(out, &result) != nil || !result.OK || result.Error != nil || string(result.Data["schema"]) != `"hn.consumer/1"` {
					t.Fatalf("%v: err=%v stdout=%s stderr=%s", args, err, out, stderr.String())
				}
				if len(result.Data["capabilities"]) == 0 {
					t.Fatalf("capabilities missing: %s", out)
				}
			}
			for _, args := range [][]string{{"version", "--json", "--unknown"}, {"capabilities", "--json", "--unknown"}, {"capabilities", "--json", "extra"}} {
				cmd := exec.Command(binary, args...)
				cmd.Dir = work
				cmd.Env = operationalTestEnvironmentWith(home, map[string]string{"PATH": shim})
				out, err := cmd.Output()
				var result struct {
					OK    bool                       `json:"ok"`
					Data  any                        `json:"data"`
					Error map[string]json.RawMessage `json:"error"`
				}
				if err == nil || json.Unmarshal(out, &result) != nil || result.OK || result.Data != nil || len(result.Error) != 5 || string(result.Error["code"]) != `"invalid_input"` || string(result.Error["origin"]) != `"native"` || string(result.Error["recovery"]) != `"do_not_retry"` || string(result.Error["operation"]) != "null" {
					t.Fatalf("invalid discovery %v: %v %s", args, err, out)
				}
			}
			for _, args := range [][]string{{"version"}, {"--version"}, {"version", "--", "--json"}} {
				cmd := exec.Command(binary, args...)
				cmd.Dir = work
				cmd.Env = operationalTestEnvironmentWith(home, map[string]string{"PATH": shim})
				out, err := cmd.CombinedOutput()
				if err != nil || string(out) != version+"\n" {
					t.Fatalf("legacy %v: %v %q", args, err, out)
				}
			}
			if after := consumerFiles(t, root); !reflect.DeepEqual(before, after) {
				t.Fatalf("static discovery changed private/work state: before=%v after=%v", before, after)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("static discovery invoked git: %v", err)
			}
		})
	}
}

func consumerFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			result[p] = "dir"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		result[p] = info.Mode().String() + ":" + string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
