package extension

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These are literal mirrors of host-v1.md sections 1 and 4. Comparing complete
// sets catches additions as well as removals, independently of the matrix below.
func TestGrantClosedContractSets(t *testing.T) {
	nativeFamilies := map[string]bool{}
	for _, value := range nativeFamilyVocabulary() {
		nativeFamilies[value] = true
	}

	grantActions := map[string]bool{}
	for _, value := range grantActionVocabulary() {
		grantActions[value] = true
	}

	wantFamilies := map[string]bool{"issue": true, "project": true, "project-item": true, "discussion": true, "discussion-reply": true, "workflow": true}
	wantActions := map[string]bool{"read": true, "observe": true, "create": true, "revise": true, "resolve": true, "type.publish": true, "raw.inspect": true}
	if !reflect.DeepEqual(nativeFamilies, wantFamilies) {
		t.Fatalf("native data families: got %v want %v", nativeFamilies, wantFamilies)
	}
	if !reflect.DeepEqual(grantActions, wantActions) {
		t.Fatalf("grant actions: got %v want %v", grantActions, wantActions)
	}
}

func grantMatrixRegistration(t *testing.T, capabilities []string) (Registration, Binding) {
	t.Helper()
	r, registry := registrationFixture(t)
	r.Manifest.Capabilities = capabilities
	r.ManifestDigest = r.Manifest.Digest()
	r, err := Register(registry, r)
	if err != nil {
		t.Fatal(err)
	}
	return r, Binding{r.RegistrationID, "sha256:" + strings.Repeat("a", 64), strings.Repeat("1", 32)}
}

func expectGrantError(t *testing.T, err error, code string) {
	t.Helper()
	var public *Error
	if !errors.As(err, &public) || public.Origin != "host" || public.Code != code || public.Recovery != "do_not_retry" || public.Operation != nil {
		t.Fatalf("want host/%s do_not_retry operation:null, got %#v", code, err)
	}
}

func TestArtifactReadSelectorActionContract(t *testing.T) {
	r, binding := grantMatrixRegistration(t, []string{"artifact.read"})
	entry := EnablementEntry{r.RegistrationID, binding.RepositoryID, binding.Epoch, true, []Grant{{"artifact:example/type", []string{"read"}}, {"artifact:opaque", []string{"raw.inspect"}}}}
	for _, access := range []Access{{"artifact.read", "artifact:example/type", "read"}, {"artifact.read", "artifact:opaque", "raw.inspect"}} {
		if err := Authorize(r, entry, binding, access, false); err != nil {
			t.Fatalf("supported pairing %+v: %v", access, err)
		}
	}
	for _, selector := range []string{"artifact:example/type", "artifact:example/*"} {
		t.Run(selector, func(t *testing.T) {
			entry.Grants = []Grant{{selector, []string{"raw.inspect"}}}
			// A structurally legal custom grant is not a usable concrete read.
			expectGrantError(t, Authorize(r, entry, binding, Access{"artifact.read", "artifact:example/type", "read"}, false), "denied_access")
			expectGrantError(t, Authorize(r, entry, binding, Access{"artifact.read", "artifact:example/type", "raw.inspect"}, false), "invalid_input")
			expectGrantError(t, ValidateRegistrationGrants(r, entry), "denied_access")
		})
	}
	for _, grant := range []Grant{{"artifact:example/type", []string{"read"}}, {"artifact:example/*", []string{"read"}}, {"artifact:opaque", []string{"raw.inspect"}}} {
		entry.Grants = []Grant{grant}
		if err := ValidateRegistrationGrants(r, entry); err != nil {
			t.Fatalf("applicable grant %+v: %v", grant, err)
		}
	}
}

func TestGrantMethodSelectorActionMatrix(t *testing.T) {
	// Each row comes from the 14-row host-v1.md section 4 table. Broad grants
	// deliberately eliminate absent-grant refusals so cross-pairings must fail
	// at the method boundary, including all seven actions for every selector.
	rows := []struct {
		method                           string
		native, identity, custom, opaque []string
	}{
		{"capabilities.list", nil, nil, nil, nil},
		{"identity.public", nil, []string{"read"}, nil, nil},
		{"resource.read", []string{"read"}, nil, nil, nil},
		{"resource.mutate", []string{"create", "revise", "resolve"}, nil, nil, nil},
		{"resource.operation", []string{"read"}, nil, nil, nil},
		{"artifact.read", nil, nil, []string{"read"}, []string{"raw.inspect"}},
		{"artifact.mutate", nil, nil, []string{"create", "revise", "resolve"}, nil},
		{"artifact.operation", nil, nil, []string{"read"}, nil},
		{"artifact.search", nil, nil, []string{"read"}, nil},
		{"artifact.type.read", nil, nil, []string{"read"}, nil},
		{"artifact.type.publish", nil, nil, []string{"type.publish"}, nil},
		{"changes.capture", []string{"read", "observe"}, nil, []string{"read", "observe"}, nil},
		{"changes.page", []string{"read", "observe"}, nil, []string{"read", "observe"}, nil},
		{"changes.since", []string{"read", "observe"}, nil, []string{"read", "observe"}, nil},
	}
	capabilities := []string{}
	for _, row := range rows {
		capabilities = append(capabilities, row.method)
	}
	r, binding := grantMatrixRegistration(t, capabilities)
	families := []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"}
	selectors := []string{"", "native:identity", "artifact:example/type", "artifact:opaque"}
	grants := []Grant{{"native:identity", []string{"read"}}, {"artifact:example/type", []string{"read", "observe", "create", "revise", "resolve", "type.publish", "raw.inspect"}}, {"artifact:opaque", []string{"raw.inspect"}}}
	for _, family := range families {
		selectors = append(selectors, "native:"+family)
		grants = append(grants, Grant{"native:" + family, []string{"read", "observe", "create", "revise", "resolve"}})
	}
	entry := EnablementEntry{r.RegistrationID, binding.RepositoryID, binding.Epoch, true, grants}
	for _, row := range rows {
		t.Run(row.method, func(t *testing.T) {
			for _, selector := range selectors {
				for _, action := range []string{"", "read", "observe", "create", "revise", "resolve", "type.publish", "raw.inspect"} {
					allowed := []string{}
					switch {
					case selector == "native:identity":
						allowed = row.identity
					case strings.HasPrefix(selector, "native:"):
						allowed = row.native
					case selector == "artifact:opaque":
						allowed = row.opaque
					case selector == "artifact:example/type":
						allowed = row.custom
					}
					want := row.method == "capabilities.list" && selector == "" && action == ""
					for _, value := range allowed {
						if value == action {
							want = true
						}
					}
					access := Access{row.method, selector, action}
					err := Authorize(r, entry, binding, access, false)
					if want {
						if err != nil {
							t.Errorf("supported %+v: %v", access, err)
						}
					} else {
						expectGrantError(t, err, "invalid_input")
					}
				}
			}
		})
	}
	// Every method's supported scopes still require their exact grant. Observe
	// alone and read alone cannot authorize a complete changes request.
	for _, row := range rows {
		if row.method == "capabilities.list" {
			continue
		}
		t.Run(row.method+"_missing_grant", func(t *testing.T) {
			selector, action := "artifact:example/type", "read"
			if len(row.native) > 0 {
				selector, action = "native:issue", row.native[0]
			} else if len(row.identity) > 0 {
				selector, action = "native:identity", row.identity[0]
			} else {
				action = row.custom[0]
			}
			access := Access{row.method, selector, action}
			missing := entry
			missing.Grants = []Grant{}
			expectGrantError(t, Authorize(r, missing, binding, access, false), "denied_access")
			if strings.HasPrefix(row.method, "changes.") {
				for _, single := range []string{"read", "observe"} {
					missing.Grants = []Grant{{selector, []string{single}}}
					expectGrantError(t, Authorize(r, missing, binding, Access{row.method, selector, single}, false), "denied_access")
				}
			}
		})
	}
	for _, method := range []string{"memory.mutate", "proposal", "review", "decide", "merge", "policy.mutate", "identity.mutate", "run", "runner", "audit", "automerge", "sync.apply", "migration.apply"} {
		expectGrantError(t, Authorize(r, entry, binding, Access{method, "native:issue", "read"}, false), "unsupported_capability")
	}
	// Namespace grants match only their exact namespace, never opaque objects
	// or a similarly prefixed namespace/type name.
	entry.Grants = []Grant{{"artifact:example/*", []string{"read"}}}
	if err := Authorize(r, entry, binding, Access{"artifact.read", "artifact:example/other", "read"}, false); err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"artifact:examples/type", "artifact:other/type"} {
		expectGrantError(t, Authorize(r, entry, binding, Access{"artifact.read", selector, "read"}, false), "denied_access")
	}
}

func TestGrantClosedScopesAndRawAuthorization(t *testing.T) {
	r, registry := registrationFixture(t)
	r.Manifest.Capabilities = []string{"resource.read", "resource.mutate", "resource.operation", "identity.public", "artifact.read", "artifact.mutate", "artifact.type.publish", "changes.capture"}
	r.ManifestDigest = r.Manifest.Digest()
	r, err := Register(registry, r)
	if err != nil {
		t.Fatal(err)
	}
	binding := Binding{RegistrationID: r.RegistrationID, RepositoryID: "sha256:" + strings.Repeat("a", 64), Epoch: strings.Repeat("1", 32)}
	entry := EnablementEntry{RegistrationID: binding.RegistrationID, RepositoryID: binding.RepositoryID, Epoch: binding.Epoch, Enabled: true, Grants: []Grant{{Selector: "native:issue", Actions: []string{"read"}}}}
	for _, test := range []struct{ raw, code string }{
		{`{"method":"resource.read","selector":"native:issue","action":"read"}`, ""},
		{`{"method":"resource.operation","selector":"native:issue","action":"read"}`, ""},
		{`{"method":"resource.mutate","selector":"native:issue","action":"create"}`, "denied_access"},
		{`{"method":"resource.mutate","selector":"native:issue","action":"resolve"}`, "denied_access"},
		{`{"method":"artifact.type.publish","selector":"artifact:example/type","action":"type.publish"}`, "denied_access"},
		{`{"method":"artifact.read","selector":"artifact:opaque","action":"raw.inspect"}`, "denied_access"},
		{`{"method":"resource.read","selector":"native:project","action":"read"}`, "denied_access"},
		{`{"method":"changes.capture","selector":"native:issue","action":"read"}`, "denied_access"},
		{`{"method":"changes.capture","selector":"native:issue","action":"observe"}`, "denied_access"},
		{`{"method":"merge","selector":"native:issue","action":"read"}`, "unsupported_capability"},
		{`{"method":"resource.mutate","selector":"native:issue","action":"read"}`, "invalid_input"},
	} {
		var access Access
		if err := json.Unmarshal([]byte(test.raw), &access); err != nil {
			t.Fatal(err)
		}
		err := Authorize(r, entry, binding, access, false)
		if test.code == "" {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			expectCode(t, err, test.code)
		}
	}
	for _, field := range []string{"registration", "repository", "epoch"} {
		other := binding
		switch field {
		case "registration":
			other.RegistrationID = "sha256:" + strings.Repeat("b", 64)
		case "repository":
			other.RepositoryID = "sha256:" + strings.Repeat("b", 64)
		case "epoch":
			other.Epoch = strings.Repeat("2", 32)
		}
		expectCode(t, Authorize(r, entry, other, Access{"resource.read", "native:issue", "read"}, false), "denied_access")
	}
	entry.Grants = []Grant{{"native:issue", []string{"create"}}, {"artifact:example/*", []string{"read", "create", "revise", "resolve", "type.publish"}}, {"artifact:opaque", []string{"raw.inspect"}}}
	// Comment uses the same explicit create action as open, not revise or read.
	if action, err := NativeAction("issue", "comment"); err != nil || action != "create" {
		t.Fatalf("comment: %q %v", action, err)
	}
	if _, err := NativeAction("project", "comment"); err == nil {
		t.Fatal("non-issue comment admitted")
	}
	for _, access := range []Access{{"resource.mutate", "native:issue", "create"}, {"artifact.read", "artifact:example/type", "read"}, {"artifact.read", "artifact:opaque", "raw.inspect"}} {
		if err := Authorize(r, entry, binding, access, false); err != nil {
			t.Fatal(err)
		}
	}
	expectCode(t, Authorize(r, entry, binding, Access{"artifact.search", "artifact:example/type", "read"}, false), "denied_access")
	entry.Grants = append(entry.Grants, Grant{"native:project", []string{"read", "observe"}})
	if err := Authorize(r, entry, binding, Access{"changes.capture", "native:project", "observe"}, false); err != nil {
		t.Fatal(err)
	}
	expectCode(t, Authorize(r, entry, binding, Access{"artifact.mutate", "artifact:example/type", "create"}, true), "unsupported_capability")
	if err := Authorize(r, entry, binding, Access{"artifact.read", "artifact:example/type", "read"}, true); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"native:identity=create", "native:identity=raw.inspect", "native:issue=type.publish", "native:issue=raw.inspect", "native:runner=read", "artifact:opaque=read", "artifact:example/*/x=read", "artifact:hn/type=read", "artifact:example/t?pe=read", "artifact:example/type=read,read", "artifact:example/type=write", "artifact:example/type=read,observe,create,revise,resolve,type.publish,raw.inspect,eighth"} {
		_, err := ParseGrant(raw)
		expectCode(t, err, "invalid_input")
	}
	all, err := ParseGrant("artifact:example/type=read,observe,create,revise,resolve,type.publish,raw.inspect")
	if err != nil {
		t.Fatal(err)
	}
	grants := []Grant{}
	for i := 0; i < 256; i++ {
		g := all
		g.Selector = fmt.Sprintf("artifact:example/t%d", i)
		grants = append(grants, g)
	}
	if err := ValidateGrants(grants); err != nil {
		t.Fatal(err)
	}
	expectCode(t, ValidateGrants(append(grants, Grant{"artifact:example/over", []string{"read"}})), "resource_limit")
	expectCode(t, ValidateGrants([]Grant{{"native:issue", []string{"read"}}, {"native:issue", []string{"create"}}}), "invalid_input")
	if _, err := Remove(registry, r.Manifest.ModuleID); err != nil {
		t.Fatal(err)
	}
	fresh, err := Register(registry, r)
	if err != nil {
		t.Fatal(err)
	}
	expectCode(t, Authorize(fresh, entry, binding, Access{"resource.mutate", "native:issue", "create"}, false), "denied_access")
}

func TestEnablementStrictStorageBoundsAndRevocation(t *testing.T) {
	common := t.TempDir()
	store, err := OpenEnablements(common)
	if err != nil {
		t.Fatal(err)
	}
	r, registry := registrationFixture(t)
	r, err = Register(registry, r)
	if err != nil {
		t.Fatal(err)
	}
	grants := []Grant{{"native:issue", []string{"read"}}}
	entry, err := store.Set(r, true, grants)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(common, "hn", "extensions", "enabled.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := store.Set(r, false, grants)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Epoch == entry.Epoch || disabled.Enabled {
		t.Fatal("disable did not revoke")
	}
	current, found, err := store.Entry(r)
	if err != nil || !found {
		t.Fatalf("current entitlement: %v", err)
	}
	if err := Authorize(r, current, Binding{r.RegistrationID, entry.RepositoryID, entry.Epoch}, Access{"resource.read", "native:issue", "read"}, false); err == nil {
		t.Fatal("old epoch admitted")
	}
	// Entropy failure preserves exact enablement bytes.
	kept, _ := os.ReadFile(path)
	_, err = store.setWithRandom(r, true, grants, bytes.NewReader(nil))
	expectCode(t, err, "repository_error")
	after, _ := os.ReadFile(path)
	if !bytes.Equal(kept, after) {
		t.Fatal("entropy failure replaced state")
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"null_enabled", func(e map[string]any) { e["enabled"] = nil }},
		{"string_enabled", func(e map[string]any) { e["enabled"] = "false" }},
		{"null_grants", func(e map[string]any) { e["grants"] = nil }},
		{"null_action", func(e map[string]any) {
			e["grants"] = []any{map[string]any{"selector": "native:issue", "actions": []any{nil}}}
		}},
		{"null_selector", func(e map[string]any) { e["grants"] = []any{map[string]any{"selector": nil, "actions": []any{"read"}}} }},
		{"unknown", func(e map[string]any) { e["extra"] = true }},
		{"null_epoch", func(e map[string]any) { e["epoch"] = nil }},
		{"other_repository", func(e map[string]any) { e["repository_id"] = "sha256:" + strings.Repeat("f", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var value map[string]any
			json.Unmarshal(before, &value)
			test.mutate(value["entries"].([]any)[0].(map[string]any))
			raw, _ := json.Marshal(value)
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := store.Load()
			expectCode(t, err, "invalid_input")
			_, err = store.Set(r, false, grants)
			expectCode(t, err, "invalid_input")
			after, _ := os.ReadFile(path)
			if !bytes.Equal(after, raw) {
				t.Fatal("malformed input repaired")
			}
		})
	}
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	// Valid at-bound bytes load; the first extra byte refuses without repair.
	at := append(append([]byte{}, before...), bytes.Repeat([]byte(" "), RegistryLimit-len(before))...)
	if err := os.WriteFile(path, at, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(at, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load()
	expectCode(t, err, "resource_limit")
	var value map[string]any
	json.Unmarshal(before, &value)
	rawEntry := value["entries"].([]any)[0].(map[string]any)
	entries := []any{}
	for i := 0; i < 128; i++ {
		e := map[string]any{}
		for k, v := range rawEntry {
			e[k] = v
		}
		e["registration_id"] = fmt.Sprintf("sha256:%064x", i)
		entries = append(entries, e)
	}
	value["entries"] = entries
	raw, _ := json.Marshal(value)
	os.WriteFile(path, raw, 0600)
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	value["entries"] = append(entries, rawEntry)
	raw, _ = json.Marshal(value)
	os.WriteFile(path, raw, 0600)
	_, err = store.Load()
	expectCode(t, err, "resource_limit")
	// Symlinked private state is refused, never followed or overwritten.
	os.Remove(path)
	target := filepath.Join(t.TempDir(), "target")
	os.WriteFile(target, before, 0600)
	os.Symlink(target, path)
	_, err = store.Load()
	expectCode(t, err, "invalid_input")
	_, err = store.Set(r, false, grants)
	expectCode(t, err, "invalid_input")
	targetAfter, _ := os.ReadFile(target)
	if !reflect.DeepEqual(targetAfter, before) {
		t.Fatal("symlink target overwritten")
	}
}
