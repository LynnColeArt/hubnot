package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This fixture registers the actual standalone inventory executable. It never
// substitutes the Go test child or imports a domain model into production core.
type inventoryFixture struct {
	*artifactCLIFixture
	consumer, input, output, manifest string
	config                            string
}

func inventoryBinary(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("HN_WP22_CONSUMER"); path != "" {
		return path
	}
	path := filepath.Join(t.TempDir(), "inventory-model")
	cmd := exec.Command("go", "build", "-o", path, "./examples/extensions/inventory-model")
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build standalone inventory consumer: %v %s", err, raw)
	}
	return path
}
func inventoryCLI(t *testing.T, consumer string) *inventoryFixture {
	t.Helper()
	host := os.Getenv("HN_WP22_BINARY")
	if host == "" {
		host = extensionTestBinary(t)
	}
	t.Setenv("HN_WP05_BINARY", host)
	f := &inventoryFixture{artifactCLIFixture: artifactCLI(t)}
	f.config = inventoryConfig(f.artifactCLIFixture)
	f.input = filepath.Join(f.home, "inventory-command.json")
	f.output = filepath.Join(f.home, "inventory-result.json")
	f.manifest = filepath.Join(f.home, "inventory-manifest.json")
	if err := os.MkdirAll(f.home, 0700); err != nil {
		t.Fatal(err)
	}
	f.consumer = filepath.Join(f.home, "inventory-publisher")
	binaryBytes, err := os.ReadFile(consumer)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.consumer, binaryBytes, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/publisher","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","artifact.type.publish","artifact.mutate"]}`
	if err := os.WriteFile(f.manifest, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	f.call("", "", "extension", "register", "--manifest", f.manifest, "--executable", f.consumer, "--arg", "--input", "--arg", f.input, "--arg", "--output", "--arg", f.output, "--json")
	// A real registration must reach this actual OS config path. This is the
	// positive control for the later independent clone absence detector.
	registry := filepath.Join(f.config, "hubnot", "extensions", "registry.json")
	if raw, err := os.ReadFile(registry); err != nil || len(raw) == 0 {
		t.Fatal("registration did not create the actual OS registry", registry, err)
	}
	if inventoryAbsent(registry) == nil {
		t.Fatal("private-state absence detector missed actual registration")
	}
	f.grant(false)
	return f
}

// Resolve after isolating HOME/XDG, using the same approved OS-aware fixture as
// registration acceptance. Darwin deliberately ignores XDG_CONFIG_HOME.
func inventoryConfig(f *artifactCLIFixture) string {
	// A real user HOME already exists; create only the fixture's private HOME,
	// never its extension registry, so explicit export has a safe parent.
	if err := os.MkdirAll(f.home, 0700); err != nil {
		f.t.Fatal(err)
	}
	config := extensionTestConfigDir(f.t, f.home)
	f.overrides = map[string]string{"XDG_CONFIG_HOME": config}
	return config
}
func inventoryAbsent(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("unexpected private state: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (f *inventoryFixture) grant(resolve bool) {
	actions := "create,revise,type.publish"
	if resolve {
		actions += ",resolve"
	}
	f.call("", "", "extension", "enable", "example.inventory/publisher", "--grant", "native:identity=read", "--grant", "artifact:example.inventory/*="+actions, "--json")
}
func (f *inventoryFixture) protected() map[string]string {
	v := f.snapshot()
	for p := range v {
		if p == f.input || p == f.output || strings.Contains(p, "/extensions/session-") || strings.Contains(p, "/extensions/owners-") {
			delete(v, p)
		}
	}
	return v
}
func (f *inventoryFixture) command(c map[string]any, code string) map[string]any {
	f.t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		f.t.Fatal(err)
	}
	if err = os.WriteFile(f.input, raw, 0600); err != nil {
		f.t.Fatal(err)
	}
	before := f.protected()
	result := f.call("", "", "extension", "run", "example.inventory/publisher", "--json")
	public, _ := json.Marshal(result)
	raw, err = os.ReadFile(f.output)
	if err != nil {
		f.t.Fatal("missing consumer result", err)
	}
	var response map[string]any
	if err = json.Unmarshal(raw, &response); err != nil {
		f.t.Fatal(err)
	}
	if code == "" {
		if response["ok"] != true {
			f.t.Fatalf("inventory %v: %s", c, raw)
		}
	} else {
		failure, _ := response["error"].(map[string]any)
		if response["ok"] != false || response["data"] != nil || failure["code"] != code {
			f.t.Fatalf("want exact %s: %s", code, raw)
		}
		origin := "artifact"
		if code == "denied_access" {
			origin = "host"
		}
		if len(failure) != 5 || failure["origin"] != origin || failure["recovery"] != "do_not_retry" || failure["operation"] != nil {
			f.t.Fatal("wrong refusal origin/recovery/operation", failure)
		}
		if after := f.protected(); !reflect.DeepEqual(before, after) {
			f.t.Fatal("refusal altered refs, objects, signer, intents or private state", c)
		}
	}

	if err := inventoryLeak(append(public, raw...)); err != nil {
		f.t.Fatal(err)
	}

	if evidence := os.Getenv("HN_WP22_EVIDENCE"); evidence != "" {
		if err := os.MkdirAll(evidence, 0700); err != nil {
			f.t.Fatal(err)
		}
		journal, err := os.OpenFile(filepath.Join(evidence, "actor-"+f.actor+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			f.t.Fatal(err)
		}
		err = json.NewEncoder(journal).Encode(map[string]any{"command": c, "response": response, "management": result})
		closeErr := journal.Close()
		if err != nil {
			f.t.Fatal(err)
		}
		if closeErr != nil {
			f.t.Fatal(closeErr)
		}
	}
	return response
}
func inventoryLeak(raw []byte) error {
	candidates := [][]byte{raw}
	var bundle map[string]any
	if json.Unmarshal(raw, &bundle) == nil {
		objects, _ := bundle["objects"].([]any)
		for _, value := range objects {
			object, ok := value.(map[string]any)
			if !ok {
				continue
			}
			encoded, _ := object["data"].(string)
			if decoded, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
				candidates = append(candidates, decoded)
			}
		}
	}
	for _, candidate := range candidates {
		for _, marker := range []string{"WP22-PRIVATE-CREDENTIAL", "semantic_request", "expected_ref", "registration_instance", "executable_digest"} {
			if bytes.Contains(candidate, []byte(marker)) {
				return fmt.Errorf("private marker %s leaked", marker)
			}
		}
	}
	return nil
}
func inventoryInactive(marker string) error {
	if _, err := os.Stat(marker); err == nil {
		return fmt.Errorf("stored text activated: %s", marker)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
func inventoryType(f *inventoryFixture, model string, version int, parents []string) string {
	return extensionArtifactEvent(f.command(map[string]any{"action": "type", "model": model, "schema_version": version, "parents": parents, "operation": fmt.Sprintf("type-%s-%d", model, version)}, ""))
}
func inventoryFields(model string) map[string]any {
	switch model {
	case "item":
		return map[string]any{"sku": "PUMP-0042", "quantity": 7, "location": "North shelf 雪"}
	case "stock":
		return map[string]any{"warehouse": "Depot 9", "counted": true}
	case "maintenance":
		return map[string]any{"priority": "urgent", "completed": false}
	}
	panic(model)
}
func inventoryCommand(model, action, typ, text, key string, record any, parents []string) map[string]any {
	return map[string]any{"action": action, "model": model, "type_revision": typ, "title": "Inventory " + model, "content": text, "fields": inventoryFields(model), "members": []any{}, "relationships": []any{}, "conversion_sources": []string{}, "operation": key, "record": record, "parents": parents}
}
func inventorySelected(t *testing.T, f *artifactCLIFixture, id string) map[string]any {
	t.Helper()
	items := closureItems(f, id)
	for _, item := range items {
		if item.(map[string]any)["id"] == id {
			return item.(map[string]any)
		}
	}
	t.Fatal("missing exact signed revision", id)
	return nil
}
func inventoryPayload(t *testing.T, f *artifactCLIFixture, id string) map[string]any {
	return inventorySelected(t, f, id)["envelope"].(map[string]any)["payload"].(map[string]any)
}
func inventoryRemote(f *artifactCLIFixture, command, id string) {
	f.call("", "", "artifact", command, "--remote", "public", "--id", id, "--json")
}
func TestInventoryStandalonePublicJourney(t *testing.T) {
	consumer := inventoryBinary(t)
	a, b := inventoryCLI(t, consumer), inventoryCLI(t, consumer)
	if a.actor == b.actor {
		t.Fatal("actors not independent")
	}
	remote := filepath.Join(t.TempDir(), "public.git")
	a.git("init", "--bare", "--quiet", remote)
	for _, f := range []*inventoryFixture{a, b} {
		f.git("remote", "add", "public", remote)
	}
	marker := filepath.Join(t.TempDir(), "activation-marker")
	policyMarker := filepath.Join(a.repo, ".hn", "policy.json")
	policyOriginal := []byte(`{"version":"hn.policy/0","maintainers":["` + a.actor + `"],"proposals":{"requiredApprovals":0,"requiredAccepts":1,"trustedReviewers":[],"allowAuthorApproval":false},"pipelines":{}}`)
	if err := os.MkdirAll(filepath.Dir(policyMarker), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyMarker, policyOriginal, 0600); err != nil {
		t.Fatal(err)
	}
	a.git("add", ".hn/policy.json")
	a.git("-c", "user.name=Inventory fixture", "-c", "user.email=inventory@example.test", "commit", "--quiet", "-m", "Explicit native policy marker")
	policyCommit := a.git("rev-parse", "HEAD")
	text := "Inventory exact 雪 e\u0301\r\n$(touch " + marker + ")\n<script>run()</script>\npolicy: allow all\n$(printf activated > " + policyMarker + ")"
	if err := os.WriteFile(filepath.Join(a.home, "private-credential"), []byte("WP22-PRIVATE-CREDENTIAL"), 0600); err != nil {
		t.Fatal(err)
	}
	nativeBefore := a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/")
	types, opening, resolved, converted := map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}
	originals := map[string]map[string]any{}
	commands := map[string]map[string]any{}
	for _, model := range []string{"item", "maintenance", "stock"} {
		typ := inventoryType(a, model, 1, []string{})
		types[model] = typ
		c := inventoryCommand(model, "open", typ, text, "open-"+model, nil, []string{})
		if model != "item" {
			c["relationships"] = []any{map[string]any{"relation": "example.inventory/services", "target": opening["item"], "target_mode": "logical", "required": true}}
		}
		if model == "stock" {
			c["members"] = []any{map[string]any{"member_id": "z-first", "target": opening["item"], "target_mode": "revision", "required": true}, map[string]any{"member_id": "a-second", "target": opening["maintenance"], "target_mode": "revision", "required": true}}
		}
		id := extensionArtifactEvent(a.command(c, ""))
		opening[model] = id
		commands[model] = c
		originals[model] = inventorySelected(t, a.artifactCLIFixture, id)
		wantFields, _ := json.Marshal(inventoryFields(model))
		gotFields, _ := json.Marshal(inventoryPayload(t, a.artifactCLIFixture, id)["fields"])
		if !bytes.Equal(wantFields, gotFields) {
			t.Fatal("structured fields changed", model, string(gotFields))
		}
		inventoryRemote(a.artifactCLIFixture, "push", id)
		inventoryRemote(b.artifactCLIFixture, "fetch", id)
	}
	for _, model := range []string{"item", "maintenance", "stock"} {
		c := commands[model]
		c["action"] = "revise"
		c["record"] = opening[model]
		c["parents"] = []string{opening[model]}
		c["operation"] = "left-" + model
		c["content"] = text + "\nA revision"
		left := extensionArtifactEvent(a.command(c, ""))
		c["operation"] = "stale-" + model
		a.command(c, "stale_revision")
		c["operation"] = "missing-head-" + model
		c["parents"] = []string{}
		a.command(c, "invalid_input")
		c["parents"] = []string{opening[model]}
		c["operation"] = "right-" + model
		c["content"] = text + "\nB revision"
		right := extensionArtifactEvent(b.command(c, ""))
		// Keep the exact shared opening cut explicitly selected. Pushing only
		// an earlier record dependency would correctly refuse to rewind the
		// remote actor stream that already includes all three openings.
		b.call("", "", "artifact", "push", "--remote", "public", "--id", opening["stock"], "--id", right, "--json")
		a.call("", "", "artifact", "fetch", "--remote", "public", "--id", opening["stock"], "--id", right, "--json")
		view := revisionReadView(a.artifactCLIFixture, opening[model])
		heads := []string{left, right}
		sort.Strings(heads)
		if view["conflict"] != true || view["state"] != nil || !reflect.DeepEqual(revisionHeads(t, view), heads) {
			t.Fatal("two real actor heads", view)
		}
		c["operation"] = "conflict-" + model
		c["parents"] = []string{left}
		a.command(c, "resource_conflict")
		c["action"] = "resolve"
		c["operation"] = "resolve-denied-" + model
		c["parents"] = heads
		a.command(c, "denied_access")
		a.grant(true)
		c["operation"] = "resolve-partial-" + model
		c["parents"] = []string{left}
		a.command(c, "stale_revision")
		c["operation"] = "resolve-" + model
		c["parents"] = heads
		c["content"] = text
		resolved[model] = extensionArtifactEvent(a.command(c, ""))
		a.grant(false)
		c["action"] = "revise"
		c["parents"] = []string{resolved[model]}
		c["operation"] = "invalid-required-" + model
		c["fields"] = map[string]any{}
		a.command(c, "invalid_input")
		fields := inventoryFields(model)
		fields["unknown"] = true
		c["fields"] = fields
		c["operation"] = "invalid-unknown-" + model
		a.command(c, "invalid_input")
		c["fields"] = inventoryFields(model)
		originalLinks := c["relationships"]
		c["relationships"] = []any{map[string]any{"relation": "example.inventory/services", "target": "sha256:" + strings.Repeat("e", 64), "target_mode": "revision", "required": true}}
		c["operation"] = "missing-target-" + model
		missing := a.command(c, "incomplete_dependency")
		if !strings.Contains(missing["error"].(map[string]any)["message"].(string), "target") {
			t.Fatal("missing required target cause", missing)
		}
		c["relationships"] = originalLinks
		typ2 := inventoryType(a, model, 2, []string{types[model]})
		unchangedView := revisionReadView(a.artifactCLIFixture, opening[model])
		unchangedPayload := unchangedView["state"].(map[string]any)
		if unchangedPayload["type_revision"] != types[model] || unchangedPayload["content"] != text || !reflect.DeepEqual(revisionHeads(t, unchangedView), []string{resolved[model]}) {
			t.Fatal("new declaration reinterpreted existing record", model, unchangedView)
		}
		c["type_revision"] = typ2
		c["action"] = "convert"
		c["conversion_sources"] = []string{resolved[model]}
		c["operation"] = "convert-" + model
		fields = inventoryFields(model)
		fields["unit"] = "each"
		c["fields"] = fields
		converted[model] = extensionArtifactEvent(a.command(c, ""))
		t.Logf("%s type1=%s opening=%s left=%s right=%s resolved=%s type2=%s converted=%s", model, types[model], opening[model], left, right, resolved[model], typ2, converted[model])
		old := inventorySelected(t, a.artifactCLIFixture, opening[model])
		if !reflect.DeepEqual(old, originals[model]) {
			t.Fatal("conversion changed original signed meaning", model)
		}
		payload := inventoryPayload(t, a.artifactCLIFixture, converted[model])
		convertedFields, _ := json.Marshal(fields)
		storedFields, _ := json.Marshal(payload["fields"])
		if !bytes.Equal(convertedFields, storedFields) || payload["type_revision"] != typ2 || !reflect.DeepEqual(payload["conversion_sources"], []any{resolved[model]}) {
			t.Fatal("conversion lost explicit new state/source", model, payload)
		}
		oldPayload := originals[model]["envelope"].(map[string]any)["payload"].(map[string]any)
		for _, key := range []string{"title", "content", "members", "relationships"} {
			if !reflect.DeepEqual(payload[key], oldPayload[key]) {
				t.Fatal("conversion lost exact text/order/link", model, key)
			}
		}
		view = revisionReadView(a.artifactCLIFixture, opening[model])
		if view["conflict"] != false || !reflect.DeepEqual(revisionHeads(t, view), []string{converted[model]}) {
			t.Fatal(view)
		}
	}

	if nativeBefore != a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/") {
		t.Fatal("inventory changed native histories")
	}
	// Each exact converted history travels through a real bare remote to a new,
	// independently initialized clone that has never registered a consumer.
	dst := artifactCLI(t)
	dstConfig := inventoryConfig(dst)
	dst.git("remote", "add", "public", remote)
	// Select the complete known final public model in one transaction. A
	// single earlier converted record can select an old B dependency prefix
	// after the shared remote has all three concurrent edits, which must refuse
	// to rewind. These are the original explicit IDs, never latest-head guesses.
	selection := []string{"--id", converted["item"], "--id", converted["maintenance"], "--id", converted["stock"], "--json"}
	a.call("", "", append([]string{"artifact", "push", "--remote", "public"}, selection...)...)
	dst.call("", "", append([]string{"artifact", "fetch", "--remote", "public"}, selection...)...)
	for _, model := range []string{"item", "maintenance", "stock"} {
		id := converted[model]
		if !reflect.DeepEqual(closureItems(a.artifactCLIFixture, id), closureItems(dst, id)) {
			t.Fatal("public signed closure changed", model)
		}
		if !reflect.DeepEqual(inventorySelected(t, dst, opening[model]), originals[model]) {
			t.Fatal("old meaning changed in clone")
		}
		dst.call("", "", "artifact", "type", "show", types[model], "--json")
		dst.call("", "", "artifact", "history", opening[model], "--json")
	}
	// The independent raw host fixture registers a test process; it must not
	// share the plugin-free clone's config merely through an inherited XDG.
	beforeRawHost := dst.snapshot()
	t.Run("independent-host-shape-search", func(t *testing.T) {
		inventoryHostShapeSearch(t, remote, converted)
	})
	dst.unchanged(beforeRawHost)
	if err := inventoryAbsent(filepath.Join(dstConfig, "hubnot", "extensions", "registry.json")); err != nil {
		t.Fatal("raw host fixture contaminated plugin-free clone", err)
	}
	inventoryGenericReads(t, dst, converted)
	inventoryCombinedCorpora(t, a.artifactCLIFixture, dst, []byte(inventoryApprovedCharterBundle), "sha256:24f344ba260b8a75b28e8c7a7cdcc5a083ab3a1f027e53b9872beda8af95d954")
	if a.git("rev-parse", "HEAD") != policyCommit {
		t.Fatal("stored data changed selected native policy commit")
	}
	inventoryRemoveAndNative(t, a, converted, marker)
	// The same approved original public model remains readable after the
	// unrelated publisher registration and executable are actually removed.
	inventoryApprovedCharterReads(t, a.artifactCLIFixture)
	inventoryApprovedCharterReads(t, dst)
	if raw, err := os.ReadFile(policyMarker); err != nil || !bytes.Equal(raw, policyOriginal) {
		t.Fatal("stored policy text changed authority marker", err)
	}
	if err := inventoryInactive(marker); err != nil {
		t.Fatal(err)
	}
	// Controls exercise the same detectors against actual effects/bytes.
	if raw, err := exec.Command("sh", "-c", `touch "$1"`, "activation-control", marker).CombinedOutput(); err != nil {
		t.Fatalf("activation control: %v %s", err, raw)
	}
	if inventoryInactive(marker) == nil {
		t.Fatal("activation detector failed")
	}
	os.Remove(marker)
	leakControl := filepath.Join(t.TempDir(), "deliberately-leaked-output")
	privateBytes, err := os.ReadFile(filepath.Join(a.home, "private-credential"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(leakControl, privateBytes, 0600); err != nil {
		t.Fatal(err)
	}
	leaked, err := os.ReadFile(leakControl)
	if err != nil || inventoryLeak(leaked) == nil {
		t.Fatal("leak detector failed", err)
	}
	encodedControl, err := json.Marshal(map[string]any{"objects": []any{map[string]any{"data": base64.RawStdEncoding.EncodeToString(privateBytes)}}})
	if err != nil || inventoryLeak(encodedControl) == nil {
		t.Fatal("encoded public-object leak detector failed", err)
	}
	if err := os.WriteFile(policyMarker, []byte("deliberate activation"), 0600); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(policyMarker); err != nil || bytes.Equal(raw, policyOriginal) {
		t.Fatal("policy activation detector failed", err)
	}
	if err := os.WriteFile(policyMarker, policyOriginal, 0600); err != nil {
		t.Fatal(err)
	}
	for _, f := range []*artifactCLIFixture{dst} {
		for path := range f.snapshot() {
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(raw, []byte("WP22-PRIVATE-CREDENTIAL")) {
					t.Fatal("private credential transported", path)
				}
			}
		}
		for _, p := range []string{filepath.Join(f.repo, ".git", "hn", "extensions", "enabled.json"), filepath.Join(dstConfig, "hubnot", "extensions", "registry.json"), filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", a.actor)} {
			if err := inventoryAbsent(p); err != nil {
				t.Fatal("clone inherited consumer/private state", err)
			}
		}
	}
	for _, p := range []string{a.binary, consumer} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("executable %s sha256:%x", p, sha256.Sum256(raw))
	}
	t.Logf("actors A=%s B=%s opening=%v converted=%v exact-text-sha256:%x", a.actor, b.actor, opening, converted, sha256.Sum256([]byte(text)))
}
func inventoryGenericReads(t *testing.T, f *artifactCLIFixture, ids map[string]string) {
	t.Helper()
	for _, model := range []string{"item", "stock", "maintenance"} {
		shape := map[string]string{"item": "document", "stock": "list", "maintenance": "ticket"}[model]
		for _, filter := range [][]string{{"--type", "example.inventory/" + model}, {"--shape", shape}} {
			args := []string{"artifact", "search", "--literal", "Inventory exact", "--limit", "200", "--json"}
			if filter[0] == "--shape" {
				args = []string{"artifact", "list", "--limit", "200", "--json"}
			}
			args = append(args, filter...)
			items := artifactPage(t, f.call("", "", args...))["items"].([]any)

			found := false
			for _, item := range items {
				row := item.(map[string]any)
				if filter[0] == "--shape" && row["shape"] != shape {
					t.Fatal("generic shape filter", row)
				}
				found = found || row["type_id"] == "example.inventory/"+model
			}
			if !found || filter[0] == "--type" && len(items) != 1 {
				t.Fatal("generic filter", items)
			}
		}
		f.call("", "", "artifact", "search", "--type", "example.inventory/"+model, "--regex", "Inventory (exact|absent)", "--limit", "200", "--json")
		f.call("", "", "artifact", "inspect", "--id", ids[model], "--json")
		path := filepath.Join(f.home, model+"-public.json")
		f.call("", "", "artifact", "export", "--id", ids[model], "--output", path, "--json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = inventoryLeak(raw); err != nil {
			t.Fatal(err)
		}
	}
	f.call("", "invalid_input", "artifact", "search", "--regex", "[", "--limit", "200", "--json")
}
func inventoryRemoveAndNative(t *testing.T, f *inventoryFixture, ids map[string]string, marker string) {
	t.Helper()
	f.call("", "", "extension", "remove", "example.inventory/publisher", "--json")
	if err := os.Remove(f.consumer); err != nil {
		t.Fatal(err)
	}
	inventoryGenericReads(t, f.artifactCLIFixture, ids)
	f.call("", "", "issue", "open", "--operation", "inventory-native-issue", "--json", "Ordinary ticket after removal")
	f.git("-c", "user.name=Inventory fixture", "-c", "user.email=inventory@example.test", "commit", "--allow-empty", "--quiet", "-m", "native anchor")
	// Native memory retains its own hn-memory/0 JSON envelope. Do not route
	// it through the artifact/issue ok/data fixture or change native semantics.
	nativeMemory := func(args ...string) map[string]any {
		cmd := exec.Command(f.binary, args...)
		cmd.Dir = f.repo
		cmd.Env = operationalTestEnvironmentWith(f.home, f.overrides)
		raw, err := cmd.Output()
		var result map[string]any
		if err != nil || json.Unmarshal(raw, &result) != nil || result["version"] != float64(0) {
			t.Fatalf("native memory after actual removal: %v %s", err, raw)
		}
		return result
	}
	created := nativeMemory("memory", "record", "--kind", "observation", "--at", "HEAD", "--applies", "exact", "--content", "Ordinary memory after removal", "--json")
	memoryID, ok := created["memoryId"].(string)
	if !ok || !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(memoryID) || created["actor"] != f.actor {
		t.Fatal("native memory lost its original public identity", created)
	}
	beforeRead := f.snapshot()
	shown := nativeMemory("memory", "show", memoryID, "--json")
	envelope, envelopeOK := shown["envelope"].(map[string]any)
	projection, projectionOK := shown["projection"].(map[string]any)
	data, dataOK := projection["data"].(map[string]any)
	if shown["memoryId"] != memoryID || !envelopeOK || envelope["protocol"] != "hn-memory/0" || envelope["actor"] != f.actor || !projectionOK || !dataOK || data["kind"] != "observation" || data["content"] != "Ordinary memory after removal" {
		t.Fatal("native exact memory readback changed after removal", shown)
	}
	f.unchanged(beforeRead)
	t.Log("native memory identity/readback after actual removal", memoryID)
	if err := inventoryInactive(marker); err != nil {
		t.Fatal(err)
	}
}

func TestInventoryMissingRequiredSchemaPublicExchange(t *testing.T) {
	consumer := inventoryBinary(t)
	a, b := inventoryCLI(t, consumer), inventoryCLI(t, consumer)
	typ := inventoryType(a, "item", 1, []string{})
	tip := a.git("rev-parse", artifactRef(a.actor))
	revisionFetch(b.artifactCLIFixture, a.artifactCLIFixture, "schema-public")
	id := extensionArtifactEvent(b.command(inventoryCommand("item", "open", typ, "external inventory schema", "foreign-open", nil, []string{}), ""))
	path, _ := exchangeExport(b.artifactCLIFixture, id)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var bundle map[string]any
	if err = json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	omit := map[string]bool{}
	for _, oid := range strings.Fields(a.git("rev-list", "--objects", "--no-object-names", tip)) {
		omit[oid] = true
	}
	objects, refs := []any{}, []any{}
	for _, o := range bundle["objects"].([]any) {
		if !omit[o.(map[string]any)["oid"].(string)] {
			objects = append(objects, o)
		}
	}
	for _, r := range bundle["refs"].([]any) {
		if r.(map[string]any)["ref"] != artifactRef(a.actor) {
			refs = append(refs, r)
		}
	}
	bundle["objects"] = objects
	bundle["refs"] = refs
	dst := artifactCLI(t)
	before := dst.snapshot()
	result := dst.call("", "incomplete_dependency", "artifact", "import", "--input", exchangeWrite(t, bundle), "--json")
	if !strings.Contains(result["error"].(map[string]any)["message"].(string), "schema") {
		t.Fatal(result)
	}
	dst.unchanged(before)
	sum := sha256.Sum256(raw)
	t.Log("complete public bundle", hex.EncodeToString(sum[:]), "omitted exact schema", typ)
}

// Assert the example's declared field constraints at and immediately beyond each
// boundary through the real standalone process and generic host validator.
func TestInventoryStructuredFieldBounds(t *testing.T) {
	f := inventoryCLI(t, inventoryBinary(t))
	for _, model := range []string{"item", "stock", "maintenance"} {
		typ := inventoryType(f, model, 1, []string{})
		for _, version := range []int{1, 2} {
			if version == 2 {
				typ = inventoryType(f, model, 2, []string{typ})
			}
			fields := inventoryFields(model)
			for key, value := range fields {
				if _, ok := value.(string); ok {
					fields[key] = strings.Repeat("x", 64)
				}
			}
			if model == "item" {
				fields["quantity"] = 0
			}
			if model == "maintenance" {
				fields["priority"] = "routine"
			}
			if version == 2 {
				fields["unit"] = strings.Repeat("u", 64)
			}
			c := inventoryCommand(model, "open", typ, "field boundary", fmt.Sprintf("bound-%s-%d", model, version), nil, []string{})
			c["fields"] = fields
			f.command(c, "")
			for key, value := range fields {
				text, ok := value.(string)
				if !ok {
					continue
				}
				fields[key] = text + "x"
				c["operation"] = fmt.Sprintf("over-%s-%d-%s", model, version, key)
				f.command(c, "invalid_input")
				fields[key] = value
			}
			if model == "item" {
				fields["quantity"] = -1
				c["operation"] = fmt.Sprintf("under-item-%d", version)
				f.command(c, "invalid_input")
			}
		}
	}
}

func TestInventoryCoreHasNoModelRecognition(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "examples" || d.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, name := range []string{"example.inventory/item", "example.inventory/stock", "example.inventory/maintenance", "inventory-model"} {
			if bytes.Contains(raw, []byte(name)) {
				t.Errorf("core production source recognizes example model %q: %s", name, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// This consumes only a frozen, independently approved PUBLIC exchange. It does
// not load another consumer's private state, source model or application journal.
func inventoryCombinedCorpora(t *testing.T, src, dst *artifactCLIFixture, public []byte, expectedDigest string) {
	t.Helper()
	sum := sha256.Sum256(public)
	if "sha256:"+hex.EncodeToString(sum[:]) != expectedDigest {
		t.Fatal("approved charter fixture digest changed")
	}
	if err := inventoryLeak(public); err != nil {
		t.Fatal(err)
	}
	var bundle map[string]any
	if err := json.Unmarshal(public, &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle["protocol"] != "hn-artifact-exchange/1" || bundle["mode"] != "accepted" {
		t.Fatal("requires accepted public exchange")
	}
	records := map[string]map[string]any{}
	types := map[string]bool{}
	shapes := map[string]bool{}
	title := ""
	for _, value := range bundle["objects"].([]any) {
		object := value.(map[string]any)
		if object["type"] != "blob" {
			continue
		}
		raw, err := base64.RawStdEncoding.DecodeString(object["data"].(string))
		if err != nil {
			t.Fatal(err)
		}
		var envelope map[string]any
		if json.Unmarshal(raw, &envelope) != nil || envelope["protocol"] != "hn-artifact/1" || envelope["kind"] != "record.open" {
			continue
		}
		payload := envelope["payload"].(map[string]any)
		typeID := payload["type_id"].(string)
		if strings.HasPrefix(typeID, "example.inventory/") {
			t.Fatal("charter fixture replaced by inventory")
		}
		id := sha256.Sum256(raw)
		records["sha256:"+hex.EncodeToString(id[:])] = envelope
		types[typeID] = true
		shapes[payload["shape"].(string)] = true
		if payload["shape"] == "ticket" {
			title = payload["title"].(string)
		}
	}
	if len(shapes) != 3 || len(records) < 3 || title == "" {
		t.Fatal("approved charter corpus must contain its document/list/ticket records")
	}
	path := filepath.Join(src.home, "approved-charter-public.json")
	if err := os.WriteFile(path, public, 0600); err != nil {
		t.Fatal(err)
	}
	src.call("", "", "artifact", "import", "--input", path, "--json")
	ids := make([]string, 0, len(records))
	for id := range records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	// This unchanged original selected root contains every exact public
	// dependency. Transfer it once; separately pushing older record IDs in
	// digest order could correctly refuse a rewind of its own actor stream.
	selected := "sha256:a95d403301ef60ac8c6667ac65a1f026a740a119192cd96545c8923e3eb844ac"
	inventoryRemote(src, "push", selected)
	inventoryRemote(dst, "fetch", selected)
	for _, id := range ids {
		got := inventorySelected(t, dst, id)["envelope"].(map[string]any)
		wantRaw, _ := json.Marshal(records[id])
		gotRaw, _ := json.Marshal(got)
		if !bytes.Equal(wantRaw, gotRaw) {
			t.Fatal("approved public envelope changed", id)
		}
	}
	for _, query := range [][]string{{"--literal", "e"}, {"--regex", "Inventory exact|" + regexp.QuoteMeta(title)}} {
		args := append([]string{"artifact", "search"}, query...)
		args = append(args, "--limit", "200", "--json")
		page := artifactPage(t, dst.call("", "", args...))
		inventoryFound, charterFound := false, false
		for _, item := range page["items"].([]any) {
			row := item.(map[string]any)
			typeID := row["type_id"].(string)
			inventoryFound = inventoryFound || strings.HasPrefix(typeID, "example.inventory/")
			charterFound = charterFound || types[typeID]
		}
		if !inventoryFound || !charterFound || page["next_cursor"] != nil {
			t.Fatal("generic search did not span complete two-model corpus", page)
		}
	}
	for typeID := range types {
		page := artifactPage(t, dst.call("", "", "artifact", "search", "--type", typeID, "--regex", ".", "--limit", "200", "--json"))
		if len(page["items"].([]any)) == 0 {
			t.Fatal("approved type disappeared", typeID)
		}
		for _, item := range page["items"].([]any) {
			if item.(map[string]any)["type_id"] != typeID {
				t.Fatal("type filter crossed models")
			}
		}
	}
	for _, shape := range []string{"document", "list", "ticket"} {
		page := artifactPage(t, dst.call("", "", "artifact", "list", "--shape", shape, "--limit", "200", "--json"))
		inventoryFound, charterFound := false, false
		for _, item := range page["items"].([]any) {
			row := item.(map[string]any)
			if row["shape"] != shape {
				t.Fatal("shape filter mismatch", row)
			}
			typeID := row["type_id"].(string)
			inventoryFound = inventoryFound || strings.HasPrefix(typeID, "example.inventory/")
			charterFound = charterFound || types[typeID]
		}
		if !inventoryFound || !charterFound {
			t.Fatal("shape filter did not span both corpora", shape)
		}
	}
	inventoryApprovedCharterReads(t, src)
	inventoryApprovedCharterReads(t, dst)
	t.Logf("approved public charter exchange %s exact openings %v", expectedDigest, ids)
}

// The publisher deliberately has no generic read/search client. This independent
// raw TEST-only caller exercises the frozen host's shape filter directly.
func inventoryHostShapeSearch(t *testing.T, remote string, ids map[string]string) {
	t.Helper()
	// This caller owns a separate local registration. The inherited raw helper
	// explicitly isolates HOME but otherwise inherits XDG_CONFIG_HOME on Linux.
	// Bound that environment before its first registration, then subtest cleanup
	// restores the parent environment. Darwin uses its isolated HOME instead.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "raw-host-config"))
	if host := os.Getenv("HN_WP22_BINARY"); host != "" {
		t.Setenv("HN_WP18_BINARY", host)
	}
	f := extensionArtifactCLI(t, "artifact:example.inventory/*=read")
	f.git("remote", "add", "public", remote)
	// Select all three exact final histories together. Separate selections can
	// advance a shared dependency and then correctly refuse its older prefix.
	selected := []string{ids["item"], ids["maintenance"], ids["stock"]}
	sort.Strings(selected)
	selection := []string{"artifact", "fetch", "--remote", "public"}
	for _, id := range selected {
		selection = append(selection, "--id", id)
	}
	f.call("", "", append(selection, "--json")...)
	for model, shape := range map[string]string{"item": "document", "stock": "list", "maintenance": "ticket"} {
		for _, query := range []map[string]any{extensionArtifactSearchParams("example.inventory/"+model, shape, "Inventory exact", nil), extensionArtifactSearchParams(nil, shape, nil, "Inventory (exact|absent)")} {
			response := f.request("artifact.search", query, "")
			items := extensionArtifactItems(response)
			if len(items) != 1 || items[0].(map[string]any)["type_id"] != "example.inventory/"+model || items[0].(map[string]any)["shape"] != shape {
				t.Fatal("host shape search", response)
			}
		}
	}
}

// Fault injection targets the actually supplied standalone consumer process. The
// scripted peer is TEST-only: this proves client stop-before-next-call behavior,
// not that a real host emits malformed responses or that framing grants authority.
func TestInventorySuppliedConsumerResponseFaultsStopBeforeEffects(t *testing.T) {
	f := inventoryCLI(t, inventoryBinary(t))
	command := map[string]any{"action": "type", "model": "item", "schema_version": 1, "parents": []string{}, "operation": "must-not-publish"}
	input, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.input, input, 0600); err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(f.repo, ".hn", "policy.json")
	if err = os.MkdirAll(filepath.Dir(policy), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(policy, []byte(`{"version":"hn.policy/0","maintainers":["`+f.actor+`"],"proposals":{"requiredApprovals":0,"requiredAccepts":1,"trustedReviewers":[],"allowAuthorApproval":false},"pipelines":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	hello := map[string]any{"protocol": "hn.extension/1", "hello": map[string]any{"session_id": strings.Repeat("1", 32), "repository_id": "sha256:" + strings.Repeat("a", 64), "module_id": "example.inventory/publisher", "module_version": "1", "registration_id": "sha256:" + strings.Repeat("b", 64)}, "limits": map[string]any{"frame_bytes": 8388608, "read_timeout_ms": 30000, "mutation_timeout_ms": 240000, "page_max": 200, "one_outstanding": true}}
	tuple := map[string]any{"protocol": "hn/0", "actor": f.actor, "operation": "original-native-intent"}
	failure := map[string]any{"origin": "native", "code": "stale_revision", "message": "expected revision is no longer the current head", "recovery": "do_not_retry", "operation": tuple}
	cases := []struct {
		name     string
		response map[string]any
		terminal bool
	}{
		{"missing-ok", map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "data": nil, "error": nil}, false},
		{"null-ok", map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "ok": nil, "data": nil, "error": nil}, false},
		{"malformed-identity", map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "ok": true, "data": map[string]any{"initialized": true, "actor": f.actor, "name": "Claim", "public_key": "not-a-public-key"}, "error": nil}, false},
		{"unknown-error-value", map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "ok": false, "data": nil, "error": map[string]any{"origin": "host", "code": "invented", "message": "not authoritative", "recovery": "retry_changed", "operation": nil}}, false},
		{"uninitialized", map[string]any{"protocol": "hn.extension/1", "request_id": "identity", "ok": true, "data": map[string]any{"initialized": false, "actor": nil, "name": nil, "public_key": nil}, "error": nil}, false},
		{"terminal-error", map[string]any{"protocol": "hn.extension/1", "request_id": nil, "ok": false, "data": nil, "error": failure}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.artifactCLIFixture.t = t
			if err := os.Remove(f.output); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			before := f.protected()
			incoming := append(inventoryFaultFrame(t, hello), inventoryFaultFrame(t, tc.response)...)
			cmd := exec.Command(f.consumer, "--input", f.input, "--output", f.output)
			cmd.Dir = f.repo
			cmd.Env = operationalTestEnvironmentWith(f.home, f.overrides)
			cmd.Stdin = bytes.NewReader(incoming)
			var wire, diagnostic bytes.Buffer
			cmd.Stdout = &wire
			cmd.Stderr = &diagnostic
			runErr := cmd.Run()
			if tc.terminal && runErr != nil || !tc.terminal && runErr == nil {
				t.Fatalf("client fault disposition: %v %s", runErr, diagnostic.String())
			}
			if err := inventoryOnlyIdentity(wire.Bytes()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, f.protected()) {
				t.Fatal("response fault changed authority/native signer/intents/refs/private state")
			}
			result, readErr := os.ReadFile(f.output)
			if tc.terminal {
				var got map[string]any
				if readErr != nil || json.Unmarshal(result, &got) != nil || got["request_id"] != nil || got["ok"] != false || !reflect.DeepEqual(got["error"], failure) {
					t.Fatalf("terminal error lost exact original code/recovery/tuple: %v %s", readErr, result)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatal("malformed response became application output", readErr, string(result))
			}
			// The same detector must reject an actual additional framed publication.
			unsafe := append(append([]byte{}, wire.Bytes()...), inventoryFaultFrame(t, map[string]any{"protocol": "hn.extension/1", "request_id": "forbidden", "method": "artifact.type.publish", "params": map[string]any{}})...)
			if inventoryOnlyIdentity(unsafe) == nil {
				t.Fatal("next-publication detector did not trigger")
			}
		})
	}
}
func inventoryFaultFrame(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	prefix := make([]byte, 4)
	binary.BigEndian.PutUint32(prefix, uint32(len(raw)))
	return append(prefix, raw...)
}
func inventoryOnlyIdentity(wire []byte) error {
	reader := bytes.NewReader(wire)
	frames := []map[string]any{}
	for reader.Len() > 0 {
		var prefix [4]byte
		if _, err := io.ReadFull(reader, prefix[:]); err != nil {
			return err
		}
		length := binary.BigEndian.Uint32(prefix[:])
		if length == 0 || length > 8388608 {
			return fmt.Errorf("invalid client frame")
		}
		raw := make([]byte, int(length))
		if _, err := io.ReadFull(reader, raw); err != nil {
			return err
		}
		var frame map[string]any
		if err := json.Unmarshal(raw, &frame); err != nil {
			return err
		}
		frames = append(frames, frame)
	}
	if len(frames) != 2 || len(frames[0]) != 2 || frames[0]["protocol"] != "hn.extension/1" || frames[0]["ready"] != true || len(frames[1]) != 4 || frames[1]["protocol"] != "hn.extension/1" || frames[1]["method"] != "identity.public" || frames[1]["request_id"] != "identity" {
		return fmt.Errorf("client sent a request beyond ready/identity: %v", frames)
	}
	params, ok := frames[1]["params"].(map[string]any)
	if !ok || len(params) != 0 {
		return fmt.Errorf("identity request params changed")
	}
	return nil
}

// Preserve the original public fixture identities and exact query expectations.
// Broad "exact" search also matches our inventory text; its charter subset must
// still equal the approved corpus, while the mixed-model checks above require
// both models. Type-filtered queries and exact closure may not leak other types.
func inventoryApprovedCharterReads(t *testing.T, f *artifactCLIFixture) {
	t.Helper()
	for _, asset := range []struct{ raw, digest string }{
		{inventoryApprovedCharterBundle, "24f344ba260b8a75b28e8c7a7cdcc5a083ab3a1f027e53b9872beda8af95d954"},
		{inventoryApprovedCharterCorpus, "3f74b9ca3bc4ec8019c8aabdd4a194da40d1f08741884a2e9bb720c7cbfe757c"},
		{inventoryApprovedCharterSelection, "3bc1822c01d28c1736dd55bcd9d9f44ab9e16719453dac8cd233a05590652bae"},
	} {
		sum := sha256.Sum256([]byte(asset.raw))
		if hex.EncodeToString(sum[:]) != asset.digest {
			t.Fatal("approved public asset bytes changed")
		}
	}
	var corpus struct {
		SelectedIDs map[string]string `json:"selected_ids"`
		Queries     []struct {
			Args []string `json:"args"`
			IDs  []string `json:"expected_ids"`
			Next any      `json:"next_cursor"`
		} `json:"expected_generic_results"`
	}
	var selection struct {
		Actor    string                    `json:"actor"`
		IDs      map[string]string         `json:"ids"`
		Payloads map[string]map[string]any `json:"payloads"`
	}
	if err := json.Unmarshal([]byte(inventoryApprovedCharterCorpus), &corpus); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(inventoryApprovedCharterSelection), &selection); err != nil {
		t.Fatal(err)
	}
	if len(corpus.SelectedIDs) != 8 || !reflect.DeepEqual(corpus.SelectedIDs, selection.IDs) || len(selection.Payloads) != 4 || selection.Actor != "5f98ba5d26caa2175fbbf5d3c77f9f5d6c560b5df5a875e6534a27d99b5189b0" {
		t.Fatal("approved public identities changed")
	}
	before := f.snapshot()
	for _, q := range corpus.Queries {
		page := artifactPage(t, f.call("", "", q.Args...))
		if page["next_cursor"] != q.Next {
			t.Fatal("approved public query unexpectedly paged", q.Args)
		}
		got := []string{}
		for _, value := range page["items"].([]any) {
			row := value.(map[string]any)
			if strings.HasPrefix(fmt.Sprint(row["type_id"]), "example.inventory/") {
				if !reflect.DeepEqual(q.Args, []string{"artifact", "search", "--literal", "exact", "--limit", "200", "--json"}) {
					t.Fatal("inventory crossed approved charter query", q.Args, row)
				}
				continue
			}
			got = append(got, row["id"].(string))
		}
		sort.Strings(got)
		want := append([]string{}, q.IDs...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("approved original query identities changed", q.Args, got, want)
		}
	}
	for name, payload := range selection.Payloads {
		item := inventorySelected(t, f, selection.IDs[name])
		envelope := item["envelope"].(map[string]any)
		if item["validation"] != "verified" || len(item["diagnostics"].([]any)) != 0 || envelope["actor"] != selection.Actor || !reflect.DeepEqual(envelope["payload"], payload) {
			t.Fatal("original signed charter identity/text/order/links changed", name, item)
		}
	}
	f.unchanged(before)
}

// Original public bytes from independently approved WP21 cycle1 event
// 01M696Z14NWEJ2R179RQEE17FT. Producer4eb5e574 is deliberately retained;
// current23983 readers validated these historical bytes, not re-signed them.
// These inert test constants create no production model or module dependency.
const inventoryApprovedCharterBundle = `{"protocol":"hn-artifact-exchange/1","mode":"accepted","refs":[{"ref":"refs/hn/artifacts/5f98ba5d26caa2175fbbf5d3c77f9f5d6c560b5df5a875e6534a27d99b5189b0/d31b14e1165c31db1c666cb9c87b2f37bbb41d0b469ceedaf0e1d904e0b85ff2","oid":"e995093117b5821241ff9bd12b3532625110beb9"}],"objects":[{"oid":"00d1b0f18025b55e0850bcc51e77ef56d16b91af","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24AVOLUDAGajW+kVQhcgdvTd2fQX2kxMDA2NDQgc2lnbmF0dXJlALkWcxPqNs1WuQ5Ov+NLeQINsrkQ"},{"oid":"14f5942d8915548b368b0afcc0b0416bce7a6951","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoicmVjb3JkLm9wZW4iLCJvcGVyYXRpb24iOiJleGFtcGxlLWNoYXJ0ZXItb3Blbi1jaGFydGVyLTEiLCJwYXJlbnRzIjpbXSwicGF5bG9hZCI6eyJjb250ZW50IjoiQSBkZW1vbnN0cmF0aW9uIHNlbGVjdGlvbiBvZiB0aHJlZSBleGFjdCBzaWduZWQgcmV2aXNpb25zOyBubyBhY3RpdmF0aW9uLiIsImNvbnZlcnNpb25fc291cmNlcyI6W10sImZpZWxkcyI6eyJwcm9qZWN0IjoiRXZpZGVuY2UgZGVtb25zdHJhdGlvbiIsInN0YXR1cyI6InNlbGVjdGVkIn0sIm1lbWJlcnMiOltdLCJyZWxhdGlvbnNoaXBzIjpbeyJyZWxhdGlvbiI6ImV4YW1wbGUuY2hhcnRlci9zZWxlY3RzLWRpcmVjdGl2ZSIsInJlcXVpcmVkIjp0cnVlLCJ0YXJnZXQiOiJzaGEyNTY6ZmJjNmQxNmUxNzY1ZjFiNjNmZTQ1NGE1NmEzODJjYzMxMmE4ZjhmZDFjOGZjNTUzY2I5YjNjMzA0Yjk1Mjk4YSIsInRhcmdldF9tb2RlIjoicmV2aXNpb24ifSx7InJlbGF0aW9uIjoiZXhhbXBsZS5jaGFydGVyL3NlbGVjdHMtdGFjdGljIiwicmVxdWlyZWQiOnRydWUsInRhcmdldCI6InNoYTI1NjoyZTE5ZTkyYjExNjFjYTdhNmJlMmZkNWIxNWU2ZjYxYzlhNDU2ZWZhNjgyNTQxNzMyYzdkZWRkNTJjYWY2NmFlIiwidGFyZ2V0X21vZGUiOiJyZXZpc2lvbiJ9LHsicmVsYXRpb24iOiJleGFtcGxlLmNoYXJ0ZXIvc2VsZWN0cy1wYWNrIiwicmVxdWlyZWQiOnRydWUsInRhcmdldCI6InNoYTI1Njo0ZGJhOTUzMjAwMjU4YjllMDEzMjk5MDg2MTFkM2ExNmNiMGMzYmRhZTU2NDE4M2Y2YWFjYmEzZjc4YzI3NzI1IiwidGFyZ2V0X21vZGUiOiJyZXZpc2lvbiJ9XSwic2hhcGUiOiJ0aWNrZXQiLCJ0aXRsZSI6IkV4YW1wbGUgcHJvamVjdCBzZWxlY3Rpb24iLCJ0eXBlX2lkIjoiZXhhbXBsZS5jaGFydGVyL2NoYXJ0ZXIiLCJ0eXBlX3JldmlzaW9uIjoic2hhMjU2OmM4MjJmYTQyMTc4ZDg5OWEwMzBiNjNjYzU1MzYyODYwZTQ4MmI5Y2Y2NzYzYWY1M2U4YTk5MzliZTJlMTY2MjkifSwicHJldmlvdXMiOiJzaGEyNTY6NGRiYTk1MzIwMDI1OGI5ZTAxMzI5OTA4NjExZDNhMTZjYjBjM2JkYWU1NjQxODNmNmFhY2JhM2Y3OGMyNzcyNSIsInByb3RvY29sIjoiaG4tYXJ0aWZhY3QvMSIsInB1YmxpY19rZXkiOiIrdTlzZTlHYzQzT09GVWdKUGFUcHpIWWNDK1hjS0RlaFhXU3FtSjBqdzNVIiwicmVjb3JkIjoiIiwicmVxdWVzdCI6InNoYTI1NjpjYTczZjA3OWYyMzQ1MzE5OTJjMGI5NDRkNjU2NTgzYTk2ZDU0ZTcyYWViNDFkNDUwZDg1MjNkOWZhZGVhOGQwIiwic2VxdWVuY2UiOjgsInN0cmVhbSI6InNoYTI1NjpkMzFiMTRlMTE2NWMzMWRiMWM2NjZjYjljODdiMmYzN2JiYjQxZDBiNDY5Y2VlZGFmMGUxZDkwNGUwYjg1ZmYyIiwidGltZXN0YW1wIjoiMjAyNi0xMC0zMVQxMjowMToyN1oifQ"},{"oid":"1e64baed55396403d5d7d5bfdac67c9794201964","type":"commit","data":"dHJlZSA2NjliZjhhY2NlY2M4Y2VlOWE5NTk0M2NhYmYxNzNmODY2ZmEwZDNjCnBhcmVudCBlOTVkZTNhODk2ZjQ3OTRjNTJkMDc1NWRjM2Y0N2ExOWVhM2E2ZDhhCmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6ZDA3ZWRlYTE4ZTRhZjAxNjMxNGU1ZDA0YjNmZjJmZmU1ZTdkZjk3YTUxZDFiOWFiODRmNzlkMjA4N2I1OTQ2NAo"},{"oid":"22f506342ca1e5ca0fd651a312ac76e6f45c9d25","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoidHlwZS5wdWJsaXNoIiwib3BlcmF0aW9uIjoiZXhhbXBsZS1jaGFydGVyLXR5cGUtZGlyZWN0aXZlLTEiLCJwYXJlbnRzIjpbXSwicGF5bG9hZCI6eyJzY2hlbWEiOnsibGFuZ3VhZ2UiOiJobi5zY2hlbWEvMSIsInJvb3QiOnsiYWRkaXRpb25hbFByb3BlcnRpZXMiOmZhbHNlLCJwcm9wZXJ0aWVzIjp7ImNvZGUiOnsibWF4Qnl0ZXMiOjY0LCJ0eXBlIjoic3RyaW5nIn0sInNldmVyaXR5Ijp7ImVudW0iOlsibXVzdCIsInNob3VsZCJdLCJtYXhCeXRlcyI6NiwidHlwZSI6InN0cmluZyJ9fSwicmVxdWlyZWQiOlsiY29kZSIsInNldmVyaXR5Il0sInR5cGUiOiJvYmplY3QifX0sInNjaGVtYV92ZXJzaW9uIjoxLCJzaGFwZSI6ImRvY3VtZW50IiwidHlwZV9pZCI6ImV4YW1wbGUuY2hhcnRlci9kaXJlY3RpdmUifSwicHJldmlvdXMiOiIiLCJwcm90b2NvbCI6ImhuLWFydGlmYWN0LzEiLCJwdWJsaWNfa2V5IjoiK3U5c2U5R2M0M09PRlVnSlBhVHB6SFljQytYY0tEZWhYV1NxbUowanczVSIsInJlY29yZCI6IiIsInJlcXVlc3QiOiJzaGEyNTY6ZDMwZDA5Zjg1ZWIzZmIwODhiYzEyOThiOGY0OGI1NTUyNjhjMDQwNGFjYjAxYTE4ZTc0YmFjNGM3YjA1YjQxNyIsInNlcXVlbmNlIjoxLCJzdHJlYW0iOiJzaGEyNTY6ZDMxYjE0ZTExNjVjMzFkYjFjNjY2Y2I5Yzg3YjJmMzdiYmI0MWQwYjQ2OWNlZWRhZjBlMWQ5MDRlMGI4NWZmMiIsInRpbWVzdGFtcCI6IjIwMjYtMTAtMzFUMTI6MDE6MjRaIn0"},{"oid":"23871e65a47a60cf84668a546b7555ad39268246","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24AqzDM16ZDMBsb5DEM7FWXYRFbw5ExMDA2NDQgc2lnbmF0dXJlAER6SofwCNCIlUP0we+xsCup7bOl"},{"oid":"2a20282c4d44e234a8f3e2321c5fd67ea68e6167","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoidHlwZS5wdWJsaXNoIiwib3BlcmF0aW9uIjoiZXhhbXBsZS1jaGFydGVyLXR5cGUtdGFjdGljLTEiLCJwYXJlbnRzIjpbXSwicGF5bG9hZCI6eyJzY2hlbWEiOnsibGFuZ3VhZ2UiOiJobi5zY2hlbWEvMSIsInJvb3QiOnsiYWRkaXRpb25hbFByb3BlcnRpZXMiOmZhbHNlLCJwcm9wZXJ0aWVzIjp7ImNvZGUiOnsibWF4Qnl0ZXMiOjY0LCJ0eXBlIjoic3RyaW5nIn0sImdvYWwiOnsibWF4Qnl0ZXMiOjY0LCJ0eXBlIjoic3RyaW5nIn19LCJyZXF1aXJlZCI6WyJjb2RlIiwiZ29hbCJdLCJ0eXBlIjoib2JqZWN0In19LCJzY2hlbWFfdmVyc2lvbiI6MSwic2hhcGUiOiJkb2N1bWVudCIsInR5cGVfaWQiOiJleGFtcGxlLmNoYXJ0ZXIvdGFjdGljIn0sInByZXZpb3VzIjoic2hhMjU2OjBlZTQ2MjZkMTZkZmQ3ZTUxMDM1YzBlOWM4YThiODgxNjgwY2EzMTM5MGMxMTQwYmMxNGU4ZGM1NjIzZTA0ZDgiLCJwcm90b2NvbCI6ImhuLWFydGlmYWN0LzEiLCJwdWJsaWNfa2V5IjoiK3U5c2U5R2M0M09PRlVnSlBhVHB6SFljQytYY0tEZWhYV1NxbUowanczVSIsInJlY29yZCI6IiIsInJlcXVlc3QiOiJzaGEyNTY6OGZlODg0OWFhZmE3ZmE3Mjg3OWQ2ZDI4ZTU0ZDc4YmQ0MGI4YTM2OGY1ODE4ZWEyOTk2YzE1NDg2YWYwYTkxZiIsInNlcXVlbmNlIjoyLCJzdHJlYW0iOiJzaGEyNTY6ZDMxYjE0ZTExNjVjMzFkYjFjNjY2Y2I5Yzg3YjJmMzdiYmI0MWQwYjQ2OWNlZWRhZjBlMWQ5MDRlMGI4NWZmMiIsInRpbWVzdGFtcCI6IjIwMjYtMTAtMzFUMTI6MDE6MjVaIn0"},{"oid":"2cd321bec8f6c69e65af22fde5a1421e1e1fcb20","type":"commit","data":"dHJlZSAyMzg3MWU2NWE0N2E2MGNmODQ2NjhhNTQ2Yjc1NTVhZDM5MjY4MjQ2CnBhcmVudCAzM2M3YjU0OTVmOTEwYTdhNDBkMTg2MmFkZmMwMTVhODIzYWFkOTk3CmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6ZmJjNmQxNmUxNzY1ZjFiNjNmZTQ1NGE1NmEzODJjYzMxMmE4ZjhmZDFjOGZjNTUzY2I5YjNjMzA0Yjk1Mjk4YQo"},{"oid":"33c7b5495f910a7a40d1862adfc015a823aad997","type":"commit","data":"dHJlZSAwMGQxYjBmMTgwMjViNTVlMDg1MGJjYzUxZTc3ZWY1NmQxNmI5MWFmCnBhcmVudCAxZTY0YmFlZDU1Mzk2NDAzZDVkN2Q1YmZkYWM2N2M5Nzk0MjAxOTY0CmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6YzgyMmZhNDIxNzhkODk5YTAzMGI2M2NjNTUzNjI4NjBlNDgyYjljZjY3NjNhZjUzZThhOTkzOWJlMmUxNjYyOQo"},{"oid":"447a4a87f008d0889543f4c1efb1b02ba9edb3a5","type":"blob","data":"K2M0eCs2N0lmeHdiYmVwSVV0dkwvOHpzOC9zNUI5a0IrN3lJVlJEV0lvdGZHUjBtNklVeGg4MGlMc2VhaEZPc3FDQ01WV0lBb1llMnpzeGE1cGVWQUE"},{"oid":"4c104be64066749baac26fd8b210a0ab774b9a39","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoidHlwZS5wdWJsaXNoIiwib3BlcmF0aW9uIjoiZXhhbXBsZS1jaGFydGVyLXR5cGUtcGFjay0xIiwicGFyZW50cyI6W10sInBheWxvYWQiOnsic2NoZW1hIjp7Imxhbmd1YWdlIjoiaG4uc2NoZW1hLzEiLCJyb290Ijp7ImFkZGl0aW9uYWxQcm9wZXJ0aWVzIjpmYWxzZSwicHJvcGVydGllcyI6eyJsYWJlbCI6eyJtYXhCeXRlcyI6NjQsInR5cGUiOiJzdHJpbmcifX0sInJlcXVpcmVkIjpbImxhYmVsIl0sInR5cGUiOiJvYmplY3QifX0sInNjaGVtYV92ZXJzaW9uIjoxLCJzaGFwZSI6Imxpc3QiLCJ0eXBlX2lkIjoiZXhhbXBsZS5jaGFydGVyL3BhY2sifSwicHJldmlvdXMiOiJzaGEyNTY6N2ZjNjM0NWY1MmMxMDQ0NzNmZjI1Yzk4NDkxMzM5N2ViNzE5NmQ3YzMyYTMyYTEyMzQ4YzE1MzY1NTg1NzAwZiIsInByb3RvY29sIjoiaG4tYXJ0aWZhY3QvMSIsInB1YmxpY19rZXkiOiIrdTlzZTlHYzQzT09GVWdKUGFUcHpIWWNDK1hjS0RlaFhXU3FtSjBqdzNVIiwicmVjb3JkIjoiIiwicmVxdWVzdCI6InNoYTI1NjpiZDg0OTViNWFmM2UxNWI4NmMyODY5MDlmMmEyNmM2OTQ4OTgwZDI4MDE4OTBkNjZlY2IyMDc4YWU5ZTQ4ZDM3Iiwic2VxdWVuY2UiOjMsInN0cmVhbSI6InNoYTI1NjpkMzFiMTRlMTE2NWMzMWRiMWM2NjZjYjljODdiMmYzN2JiYjQxZDBiNDY5Y2VlZGFmMGUxZDkwNGUwYjg1ZmYyIiwidGltZXN0YW1wIjoiMjAyNi0xMC0zMVQxMjowMToyNVoifQ"},{"oid":"54e2d40c019a8d6fa455085c81dbd37767d05f69","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoidHlwZS5wdWJsaXNoIiwib3BlcmF0aW9uIjoiZXhhbXBsZS1jaGFydGVyLXR5cGUtY2hhcnRlci0xIiwicGFyZW50cyI6W10sInBheWxvYWQiOnsic2NoZW1hIjp7Imxhbmd1YWdlIjoiaG4uc2NoZW1hLzEiLCJyb290Ijp7ImFkZGl0aW9uYWxQcm9wZXJ0aWVzIjpmYWxzZSwicHJvcGVydGllcyI6eyJwcm9qZWN0Ijp7Im1heEJ5dGVzIjo2NCwidHlwZSI6InN0cmluZyJ9LCJzdGF0dXMiOnsiZW51bSI6WyJzZWxlY3RlZCJdLCJtYXhCeXRlcyI6OCwidHlwZSI6InN0cmluZyJ9fSwicmVxdWlyZWQiOlsicHJvamVjdCIsInN0YXR1cyJdLCJ0eXBlIjoib2JqZWN0In19LCJzY2hlbWFfdmVyc2lvbiI6MSwic2hhcGUiOiJ0aWNrZXQiLCJ0eXBlX2lkIjoiZXhhbXBsZS5jaGFydGVyL2NoYXJ0ZXIifSwicHJldmlvdXMiOiJzaGEyNTY6ZDA3ZWRlYTE4ZTRhZjAxNjMxNGU1ZDA0YjNmZjJmZmU1ZTdkZjk3YTUxZDFiOWFiODRmNzlkMjA4N2I1OTQ2NCIsInByb3RvY29sIjoiaG4tYXJ0aWZhY3QvMSIsInB1YmxpY19rZXkiOiIrdTlzZTlHYzQzT09GVWdKUGFUcHpIWWNDK1hjS0RlaFhXU3FtSjBqdzNVIiwicmVjb3JkIjoiIiwicmVxdWVzdCI6InNoYTI1Njo3ZWRlMjYzNGM3YTJjOTRjYjFmOTA3Mjg2MDk2YjVmYTM3MGMwZWQwNjQ2ODMxNDZmYWQyMTViNTMyMDI2NDhkIiwic2VxdWVuY2UiOjQsInN0cmVhbSI6InNoYTI1NjpkMzFiMTRlMTE2NWMzMWRiMWM2NjZjYjljODdiMmYzN2JiYjQxZDBiNDY5Y2VlZGFmMGUxZDkwNGUwYjg1ZmYyIiwidGltZXN0YW1wIjoiMjAyNi0xMC0zMVQxMjowMToyNVoifQ"},{"oid":"5b91ecae964135fcc1c34101bdc534ae8f6111c2","type":"blob","data":"TXRnY0JjSThlRjRoazloSlhCcVVHMzI3dTViYjRCR2hwYk9Ya2dzdVkxWm1KYUJ3RWpBeGlwd3A4SGN6Z2Q3d2ZrTW5KeEQzQlpXY0FEZndrdWpZQmc"},{"oid":"669bf8accecc8cee9a95943cabf173f866fa0d3c","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24ATBBL5kBmdJuqwm/YshCgq3dLmjkxMDA2NDQgc2lnbmF0dXJlAFuR7K6WQTX8wcNBAb3FNK6PYRHC"},{"oid":"7d6927a3f1e939bf49e094323ea23d0d59681a6f","type":"commit","data":"dHJlZSA5OTcxZmI3MDA4YTFjYzk5ODhjNzA1NzAyYzViZmFmOTUzMzhmZjExCnBhcmVudCBiYzRiMzRjNWIzMWFjY2YzYzE2Yjg3YTgwNmFiMWM3YWNiY2NjYzAzCmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg2IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg2IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6NGRiYTk1MzIwMDI1OGI5ZTAxMzI5OTA4NjExZDNhMTZjYjBjM2JkYWU1NjQxODNmNmFhY2JhM2Y3OGMyNzcyNQo"},{"oid":"907a47c8ef545abcc496da3c5fc4e0bf1d3979ea","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24AKiAoLE1E4jSo8+IyHF/WfqaOYWcxMDA2NDQgc2lnbmF0dXJlAMHqkc80ePXLoMs75D8BBT/seTSW"},{"oid":"9971fb7008a1cc9988c705702c5bfaf95338ff11","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24AuS7jGD43RG6Mg9bwxCKR5MiwglAxMDA2NDQgc2lnbmF0dXJlANp7Se+oJ49fVTVEBsTqolDSFZdO"},{"oid":"ab30ccd7a643301b1be4310cec559761115bc391","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoicmVjb3JkLm9wZW4iLCJvcGVyYXRpb24iOiJleGFtcGxlLWNoYXJ0ZXItb3Blbi1kaXJlY3RpdmUtMSIsInBhcmVudHMiOltdLCJwYXlsb2FkIjp7ImNvbnRlbnQiOiIjIFByZXNlcnZlIHJldmlldyBldmlkZW5jZVxuS2VlcCBleGFjdCBpbnB1dCByZXZpc2lvbnMgd2l0aCBlYWNoIGRlY2lzaW9uLlxu6ZuqIGXMgVxyXG4kKGVjaG8gc3RvcmVkLXRleHQtaXMtbm90LWEtY29tbWFuZCkiLCJjb252ZXJzaW9uX3NvdXJjZXMiOltdLCJmaWVsZHMiOnsiY29kZSI6IkVWSURFTkNFLTAxIiwic2V2ZXJpdHkiOiJtdXN0In0sIm1lbWJlcnMiOltdLCJyZWxhdGlvbnNoaXBzIjpbXSwic2hhcGUiOiJkb2N1bWVudCIsInRpdGxlIjoiRXZpZGVuY2UgaW50ZWdyaXR5IGRpcmVjdGl2ZSIsInR5cGVfaWQiOiJleGFtcGxlLmNoYXJ0ZXIvZGlyZWN0aXZlIiwidHlwZV9yZXZpc2lvbiI6InNoYTI1NjowZWU0NjI2ZDE2ZGZkN2U1MTAzNWMwZTljOGE4Yjg4MTY4MGNhMzEzOTBjMTE0MGJjMTRlOGRjNTYyM2UwNGQ4In0sInByZXZpb3VzIjoic2hhMjU2OmM4MjJmYTQyMTc4ZDg5OWEwMzBiNjNjYzU1MzYyODYwZTQ4MmI5Y2Y2NzYzYWY1M2U4YTk5MzliZTJlMTY2MjkiLCJwcm90b2NvbCI6ImhuLWFydGlmYWN0LzEiLCJwdWJsaWNfa2V5IjoiK3U5c2U5R2M0M09PRlVnSlBhVHB6SFljQytYY0tEZWhYV1NxbUowanczVSIsInJlY29yZCI6IiIsInJlcXVlc3QiOiJzaGEyNTY6NDJkMTcwOTE4NjBiYjE1MGZlZjVjZmE4N2VhODJmOGIzYTg2Y2E4MGQwY2Q5NWY5ZWE2OGZhMWMzYWFmYTk3NyIsInNlcXVlbmNlIjo1LCJzdHJlYW0iOiJzaGEyNTY6ZDMxYjE0ZTExNjVjMzFkYjFjNjY2Y2I5Yzg3YjJmMzdiYmI0MWQwYjQ2OWNlZWRhZjBlMWQ5MDRlMGI4NWZmMiIsInRpbWVzdGFtcCI6IjIwMjYtMTAtMzFUMTI6MDE6MjVaIn0"},{"oid":"ab5ee5e88469c665d86b2047492a75c900e700d0","type":"blob","data":"L2hzdzhlVVdEYkxmYWNLU3g4ZFQxR1c1eGs3TXFOcEJrUDkwS1RMYXNyWStScFpOOGdRVDBXeXlydG5PakhKa0tKOTI1L2ZzYVJYemh6TFlMVlpJQWc"},{"oid":"b9167313ea36cd56b90e4ebfe34b79020db2b910","type":"blob","data":"S3RyVDBZN2xOQWgzV3ZlTlNTblIzenlrSVo5SmlhSXRROTh6ZUxSbUExMlNlZWhEaGlVN0lvUXNiTmppekFSUU1CeFJwYnpuUEwxWjJocGw5dzF6RGc"},{"oid":"b92ee3183e37446e8c83d6f0c42291e4c8b08250","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoicmVjb3JkLm9wZW4iLCJvcGVyYXRpb24iOiJleGFtcGxlLWNoYXJ0ZXItb3Blbi1wYWNrLTEiLCJwYXJlbnRzIjpbXSwicGF5bG9hZCI6eyJjb250ZW50IjoiT3JkZXIgaXMgZXhwbGljaXQ6IGRpcmVjdGl2ZSBmaXJzdCwgdGFjdGljIHNlY29uZC4iLCJjb252ZXJzaW9uX3NvdXJjZXMiOltdLCJmaWVsZHMiOnsibGFiZWwiOiJFdmlkZW5jZSByZXZpZXcgc3RhcnRlciJ9LCJtZW1iZXJzIjpbeyJtZW1iZXJfaWQiOiJ6LWRpcmVjdGl2ZSIsInJlcXVpcmVkIjp0cnVlLCJ0YXJnZXQiOiJzaGEyNTY6ZmJjNmQxNmUxNzY1ZjFiNjNmZTQ1NGE1NmEzODJjYzMxMmE4ZjhmZDFjOGZjNTUzY2I5YjNjMzA0Yjk1Mjk4YSIsInRhcmdldF9tb2RlIjoicmV2aXNpb24ifSx7Im1lbWJlcl9pZCI6ImEtdGFjdGljIiwicmVxdWlyZWQiOnRydWUsInRhcmdldCI6InNoYTI1NjoyZTE5ZTkyYjExNjFjYTdhNmJlMmZkNWIxNWU2ZjYxYzlhNDU2ZWZhNjgyNTQxNzMyYzdkZWRkNTJjYWY2NmFlIiwidGFyZ2V0X21vZGUiOiJyZXZpc2lvbiJ9XSwicmVsYXRpb25zaGlwcyI6W3sicmVsYXRpb24iOiJleGFtcGxlLmNoYXJ0ZXIvaW5jbHVkZXMiLCJyZXF1aXJlZCI6dHJ1ZSwidGFyZ2V0Ijoic2hhMjU2OmZiYzZkMTZlMTc2NWYxYjYzZmU0NTRhNTZhMzgyY2MzMTJhOGY4ZmQxYzhmYzU1M2NiOWIzYzMwNGI5NTI5OGEiLCJ0YXJnZXRfbW9kZSI6InJldmlzaW9uIn0seyJyZWxhdGlvbiI6ImV4YW1wbGUuY2hhcnRlci9pbmNsdWRlcyIsInJlcXVpcmVkIjp0cnVlLCJ0YXJnZXQiOiJzaGEyNTY6MmUxOWU5MmIxMTYxY2E3YTZiZTJmZDViMTVlNmY2MWM5YTQ1NmVmYTY4MjU0MTczMmM3ZGVkZDUyY2FmNjZhZSIsInRhcmdldF9tb2RlIjoicmV2aXNpb24ifV0sInNoYXBlIjoibGlzdCIsInRpdGxlIjoiRXZpZGVuY2UgcmV2aWV3IHBhY2siLCJ0eXBlX2lkIjoiZXhhbXBsZS5jaGFydGVyL3BhY2siLCJ0eXBlX3JldmlzaW9uIjoic2hhMjU2OmQwN2VkZWExOGU0YWYwMTYzMTRlNWQwNGIzZmYyZmZlNWU3ZGY5N2E1MWQxYjlhYjg0Zjc5ZDIwODdiNTk0NjQifSwicHJldmlvdXMiOiJzaGEyNTY6MmUxOWU5MmIxMTYxY2E3YTZiZTJmZDViMTVlNmY2MWM5YTQ1NmVmYTY4MjU0MTczMmM3ZGVkZDUyY2FmNjZhZSIsInByb3RvY29sIjoiaG4tYXJ0aWZhY3QvMSIsInB1YmxpY19rZXkiOiIrdTlzZTlHYzQzT09GVWdKUGFUcHpIWWNDK1hjS0RlaFhXU3FtSjBqdzNVIiwicmVjb3JkIjoiIiwicmVxdWVzdCI6InNoYTI1NjpmYTA4OTI5MTA0MWU1ZDFlZDhiMWFlOTE4ZDU5MGY2NzQ2YzUyMTZkMjU2NjFjMGJlZTdiNmZkNmYzNjRiMGY2Iiwic2VxdWVuY2UiOjcsInN0cmVhbSI6InNoYTI1NjpkMzFiMTRlMTE2NWMzMWRiMWM2NjZjYjljODdiMmYzN2JiYjQxZDBiNDY5Y2VlZGFmMGUxZDkwNGUwYjg1ZmYyIiwidGltZXN0YW1wIjoiMjAyNi0xMC0zMVQxMjowMToyNloifQ"},{"oid":"bc4b34c5b31accf3c16b87a806ab1c7acbcccc03","type":"commit","data":"dHJlZSBlZmQzZmRmMzAxN2RlMDI3Zjk5NWQ1OTdmN2MxZmY0ODYwYzIwN2M1CnBhcmVudCAyY2QzMjFiZWM4ZjZjNjllNjVhZjIyZmRlNWExNDIxZTFlMWZjYjIwCmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg2IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg2IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6MmUxOWU5MmIxMTYxY2E3YTZiZTJmZDViMTVlNmY2MWM5YTQ1NmVmYTY4MjU0MTczMmM3ZGVkZDUyY2FmNjZhZQo"},{"oid":"bf87c69022518f2c12a1f5fa3c7743d48b36a4ad","type":"blob","data":"eyJhY3RvciI6IjVmOThiYTVkMjZjYWEyMTc1ZmJiZjVkM2M3N2Y5ZjVkNmM1NjBiNWRmNWE4NzVlNjUzNGEyN2Q5OWI1MTg5YjAiLCJraW5kIjoicmVjb3JkLm9wZW4iLCJvcGVyYXRpb24iOiJleGFtcGxlLWNoYXJ0ZXItb3Blbi10YWN0aWMtMSIsInBhcmVudHMiOltdLCJwYXlsb2FkIjp7ImNvbnRlbnQiOiJDb21wYXJlIHRoZSBwcm9wb3NlZCBpbnB1dHMgdG8gdGhlIHN0b3JlZCBzaWduZWQgcmV2aXNpb25zLiIsImNvbnZlcnNpb25fc291cmNlcyI6W10sImZpZWxkcyI6eyJjb2RlIjoiQ09NUEFSRS0wMSIsImdvYWwiOiJNYWtlIGVhY2ggZGVjaXNpb24gaW5kZXBlbmRlbnRseSBjaGVja2FibGUifSwibWVtYmVycyI6W10sInJlbGF0aW9uc2hpcHMiOlt7InJlbGF0aW9uIjoiZXhhbXBsZS5jaGFydGVyL2ltcGxlbWVudHMiLCJyZXF1aXJlZCI6dHJ1ZSwidGFyZ2V0Ijoic2hhMjU2OmZiYzZkMTZlMTc2NWYxYjYzZmU0NTRhNTZhMzgyY2MzMTJhOGY4ZmQxYzhmYzU1M2NiOWIzYzMwNGI5NTI5OGEiLCJ0YXJnZXRfbW9kZSI6InJldmlzaW9uIn1dLCJzaGFwZSI6ImRvY3VtZW50IiwidGl0bGUiOiJJbnNwZWN0IGV2aWRlbmNlIGJlZm9yZSBhcHByb3ZpbmciLCJ0eXBlX2lkIjoiZXhhbXBsZS5jaGFydGVyL3RhY3RpYyIsInR5cGVfcmV2aXNpb24iOiJzaGEyNTY6N2ZjNjM0NWY1MmMxMDQ0NzNmZjI1Yzk4NDkxMzM5N2ViNzE5NmQ3YzMyYTMyYTEyMzQ4YzE1MzY1NTg1NzAwZiJ9LCJwcmV2aW91cyI6InNoYTI1NjpmYmM2ZDE2ZTE3NjVmMWI2M2ZlNDU0YTU2YTM4MmNjMzEyYThmOGZkMWM4ZmM1NTNjYjliM2MzMDRiOTUyOThhIiwicHJvdG9jb2wiOiJobi1hcnRpZmFjdC8xIiwicHVibGljX2tleSI6Iit1OXNlOUdjNDNPT0ZVZ0pQYVRwekhZY0MrWGNLRGVoWFdTcW1KMGp3M1UiLCJyZWNvcmQiOiIiLCJyZXF1ZXN0Ijoic2hhMjU2OmMwMTAxZTY1MmJkYjQ0OWI2OTU5NTMzMDAzNTRjNWFiZGQyNzgyNzYyNDg0YzI3YzQ5ZGQzODhlMWFhNGFmYWYiLCJzZXF1ZW5jZSI6Niwic3RyZWFtIjoic2hhMjU2OmQzMWIxNGUxMTY1YzMxZGIxYzY2NmNiOWM4N2IyZjM3YmJiNDFkMGI0NjljZWVkYWYwZTFkOTA0ZTBiODVmZjIiLCJ0aW1lc3RhbXAiOiIyMDI2LTEwLTMxVDEyOjAxOjI2WiJ9"},{"oid":"c1ea91cf3478f5cba0cb3be43f01053fec793496","type":"blob","data":"QnA5R3BGcHBxZDJ3NG5nT2pLSytmUndSTkNKNzhWU1VQemNlZXN5azBBOHRXSXI4LzVGYlp6bXFCdzNublVKaklwVURKbm1XRDNQSy9scnR2cldKQ1E"},{"oid":"ccad9c97a08fe8cf40ad00c42821c6f43aa2d9cb","type":"commit","data":"dHJlZSBmZGVhY2Y5YWFiNzRhMTA5MmYwZmNjMTU1ZWNlMjA3YzYxYjlkNjRmCmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg0IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg0IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6MGVlNDYyNmQxNmRmZDdlNTEwMzVjMGU5YzhhOGI4ODE2ODBjYTMxMzkwYzExNDBiYzE0ZThkYzU2MjNlMDRkOAo"},{"oid":"da7b49efa8278f5f55354406c4eaa250d215974e","type":"blob","data":"aEVrNWpORzh6amhqV1lURkc2TTFSd1NDRlRvcEFBYW9PUnE4WTFEY1kyRy9nTzBKQVkzRlBjY0NuS2tWaW5HMmFBeGVKd2M3bFJwb1pyb05vRjh6Qmc"},{"oid":"df0a42caa5b041c2c67d016e388a02369ce2547d","type":"blob","data":"eHdBdExFcG95bXpNQTBsWTBNaDY0NUNRR0hlWlloQkVKTzM2d3kxNTZORW5YVTU1a1NlN2lnZkYwc0VvRlI1K2xEajJYTW1EbUpvZ2p6WE93UTVJQkE"},{"oid":"e95de3a896f4794c52d0755dc3f47a19ea3a6d8a","type":"commit","data":"dHJlZSA5MDdhNDdjOGVmNTQ1YWJjYzQ5NmRhM2M1ZmM0ZTBiZjFkMzk3OWVhCnBhcmVudCBjY2FkOWM5N2EwOGZlOGNmNDBhZDAwYzQyODIxYzZmNDNhYTJkOWNiCmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg1IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6N2ZjNjM0NWY1MmMxMDQ0NzNmZjI1Yzk4NDkxMzM5N2ViNzE5NmQ3YzMyYTMyYTEyMzQ4YzE1MzY1NTg1NzAwZgo"},{"oid":"e995093117b5821241ff9bd12b3532625110beb9","type":"commit","data":"dHJlZSBmYzg0Y2MyYmNkZTE0NmJjNTVkMTliNmI1YzIyMDY4ZDUwY2YzZTQ3CnBhcmVudCA3ZDY5MjdhM2YxZTkzOWJmNDllMDk0MzIzZWEyM2QwZDU5NjgxYTZmCmF1dGhvciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg3IC0wNTAwCmNvbW1pdHRlciBIdWJub3QgYXJ0aWZhY3QgdHJhbnNwb3J0IDxhcnRpZmFjdEBobi5pbnZhbGlkPiAxNzkzNDQ4MDg3IC0wNTAwCgpobiBhcnRpZmFjdCBzaGEyNTY6YTk1ZDQwMzMwMWVmNjBhYzhjNjY2N2FjNjVhMWYwMjZhNzQwYTExOTE5MmNkOTY1NDVjODkyM2UzZWI4NDRhYwo"},{"oid":"efd3fdf3017de027f995d597f7c1ff4860c207c5","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24Av4fGkCJRjywSofX6PHdD1Is2pK0xMDA2NDQgc2lnbmF0dXJlAPnePYKNf6POl6G9WiFiGvXK0Ahm"},{"oid":"f9de3d828d7fa3ce97a1bd5a21621af5cad00866","type":"blob","data":"S0UxN2NqclcrSnhZVUMreXhwTWxRRksrTmlCNkhTUHkwdjU5SVArdXljYVB1UmRZNUwvM0dUOXRIVTVQWjRhM09NWkkxV1RGRXZBZktIelZMRmd4Qnc"},{"oid":"fc84cc2bcde146bc55d19b6b5c22068d50cf3e47","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24AFPWULYkVVIs2iwr8wLBBa856aVExMDA2NDQgc2lnbmF0dXJlAN8KQsqlsEHCxn0BbjiKAjac4lR9"},{"oid":"fdeacf9aab74a1092f0fcc155ece207c61b9d64f","type":"tree","data":"MTAwNjQ0IGFydGlmYWN0Lmpzb24AIvUGNCyh5coP1lGjEqx25vRcnSUxMDA2NDQgc2lnbmF0dXJlAKte5eiEacZl2GsgR0kqdckA5wDQ"}]}`

const inventoryApprovedCharterCorpus = `{
  "bundle_sha256": "sha256:24f344ba260b8a75b28e8c7a7cdcc5a083ab3a1f027e53b9872beda8af95d954",
  "bytes": 19128,
  "claim": "Exact signed public closure from real standalone publication; inert example data, no production Spec Kitty compatibility or authority. No signed body was rewritten.",
  "expected_generic_results": [
    {
      "args": [
        "artifact",
        "search",
        "--literal",
        "EVIDENCE-01",
        "--limit",
        "200",
        "--json"
      ],
      "expected_ids": [
        "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a"
      ],
      "next_cursor": null
    },
    {
      "args": [
        "artifact",
        "search",
        "--type",
        "example.charter/directive",
        "--literal",
        "EVIDENCE-01",
        "--limit",
        "200",
        "--json"
      ],
      "expected_ids": [
        "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a"
      ],
      "next_cursor": null
    },
    {
      "args": [
        "artifact",
        "search",
        "--regex",
        "Inspect evidence|Evidence review",
        "--limit",
        "200",
        "--json"
      ],
      "expected_ids": [
        "sha256:2e19e92b1161ca7a6be2fd5b15e6f61c9a456efa682541732c7dedd52caf66ae",
        "sha256:4dba953200258b9e01329908611d3a16cb0c3bdae564183f6aacba3f78c27725"
      ],
      "next_cursor": null
    },
    {
      "args": [
        "artifact",
        "search",
        "--literal",
        "exact",
        "--limit",
        "200",
        "--json"
      ],
      "expected_ids": [
        "sha256:a95d403301ef60ac8c6667ac65a1f026a740a119192cd96545c8923e3eb844ac",
        "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a"
      ],
      "next_cursor": null
    },
    {
      "args": [
        "artifact",
        "list",
        "--type",
        "example.charter/pack",
        "--shape",
        "list",
        "--limit",
        "200",
        "--json"
      ],
      "expected_ids": [
        "sha256:4dba953200258b9e01329908611d3a16cb0c3bdae564183f6aacba3f78c27725"
      ],
      "next_cursor": null
    },
    {
      "args": [
        "artifact",
        "inspect",
        "--id",
        "sha256:a95d403301ef60ac8c6667ac65a1f026a740a119192cd96545c8923e3eb844ac",
        "--json"
      ],
      "expected_ids": [
        "sha256:0ee4626d16dfd7e51035c0e9c8a8b881680ca31390c1140bc14e8dc5623e04d8",
        "sha256:2e19e92b1161ca7a6be2fd5b15e6f61c9a456efa682541732c7dedd52caf66ae",
        "sha256:4dba953200258b9e01329908611d3a16cb0c3bdae564183f6aacba3f78c27725",
        "sha256:7fc6345f52c104473ff25c984913397eb7196d7c32a32a12348c15365585700f",
        "sha256:a95d403301ef60ac8c6667ac65a1f026a740a119192cd96545c8923e3eb844ac",
        "sha256:c822fa42178d899a030b63cc55362860e482b9cf6763af53e8a9939be2e16629",
        "sha256:d07edea18e4af016314e5d04b3ff2ffe5e7df97a51d1b9ab84f79d2087b59464",
        "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a"
      ],
      "next_cursor": null
    }
  ],
  "host_sha256": "252ccc88010795ef1f6d9c8cc0a7251b2c7bd73567132e5dc3cdc722ef393a75",
  "objects": 32,
  "producing_source_commit": "4eb5e57470c74cb37a72fc5a37c3c20c0351b600",
  "producing_source_inputs_sha256": "ed8e63ded69433bf8ab21a490361a9e3abee033596b0a32f6e80a8a804dba293",
  "producing_source_tree": "0c7ec57e884115ea5ab5d88e7764d696b03774da",
  "protocol": "example.charter-public-corpus/1",
  "refs": 1,
  "roots": [
    "sha256:a95d403301ef60ac8c6667ac65a1f026a740a119192cd96545c8923e3eb844ac"
  ],
  "selected_ids": {
    "charter": "sha256:a95d403301ef60ac8c6667ac65a1f026a740a119192cd96545c8923e3eb844ac",
    "charter_type": "sha256:c822fa42178d899a030b63cc55362860e482b9cf6763af53e8a9939be2e16629",
    "directive": "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a",
    "directive_type": "sha256:0ee4626d16dfd7e51035c0e9c8a8b881680ca31390c1140bc14e8dc5623e04d8",
    "pack": "sha256:4dba953200258b9e01329908611d3a16cb0c3bdae564183f6aacba3f78c27725",
    "pack_type": "sha256:d07edea18e4af016314e5d04b3ff2ffe5e7df97a51d1b9ab84f79d2087b59464",
    "tactic": "sha256:2e19e92b1161ca7a6be2fd5b15e6f61c9a456efa682541732c7dedd52caf66ae",
    "tactic_type": "sha256:7fc6345f52c104473ff25c984913397eb7196d7c32a32a12348c15365585700f"
  },
  "standalone_consumer_sha256": "6153358cb5d099a9a200d339d70f96dd5b158085628dc182ef27ddc42f1c69bb",
  "type_ids": [
    "example.charter/directive",
    "example.charter/tactic",
    "example.charter/pack",
    "example.charter/charter"
  ]
}
`

const inventoryApprovedCharterSelection = `{
  "protocol": "example.charter-selection/1",
  "actor": "5f98ba5d26caa2175fbbf5d3c77f9f5d6c560b5df5a875e6534a27d99b5189b0",
  "ids": {
    "charter": "sha256:a95d403301ef60ac8c6667ac65a1f026a740a119192cd96545c8923e3eb844ac",
    "charter_type": "sha256:c822fa42178d899a030b63cc55362860e482b9cf6763af53e8a9939be2e16629",
    "directive": "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a",
    "directive_type": "sha256:0ee4626d16dfd7e51035c0e9c8a8b881680ca31390c1140bc14e8dc5623e04d8",
    "pack": "sha256:4dba953200258b9e01329908611d3a16cb0c3bdae564183f6aacba3f78c27725",
    "pack_type": "sha256:d07edea18e4af016314e5d04b3ff2ffe5e7df97a51d1b9ab84f79d2087b59464",
    "tactic": "sha256:2e19e92b1161ca7a6be2fd5b15e6f61c9a456efa682541732c7dedd52caf66ae",
    "tactic_type": "sha256:7fc6345f52c104473ff25c984913397eb7196d7c32a32a12348c15365585700f"
  },
  "payloads": {
    "charter": {
      "content": "A demonstration selection of three exact signed revisions; no activation.",
      "conversion_sources": [],
      "fields": {
        "project": "Evidence demonstration",
        "status": "selected"
      },
      "members": [],
      "relationships": [
        {
          "relation": "example.charter/selects-directive",
          "required": true,
          "target": "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a",
          "target_mode": "revision"
        },
        {
          "relation": "example.charter/selects-tactic",
          "required": true,
          "target": "sha256:2e19e92b1161ca7a6be2fd5b15e6f61c9a456efa682541732c7dedd52caf66ae",
          "target_mode": "revision"
        },
        {
          "relation": "example.charter/selects-pack",
          "required": true,
          "target": "sha256:4dba953200258b9e01329908611d3a16cb0c3bdae564183f6aacba3f78c27725",
          "target_mode": "revision"
        }
      ],
      "shape": "ticket",
      "title": "Example project selection",
      "type_id": "example.charter/charter",
      "type_revision": "sha256:c822fa42178d899a030b63cc55362860e482b9cf6763af53e8a9939be2e16629"
    },
    "directive": {
      "content": "# Preserve review evidence\nKeep exact input revisions with each decision.\n雪 é\r\n$(echo stored-text-is-not-a-command)",
      "conversion_sources": [],
      "fields": {
        "code": "EVIDENCE-01",
        "severity": "must"
      },
      "members": [],
      "relationships": [],
      "shape": "document",
      "title": "Evidence integrity directive",
      "type_id": "example.charter/directive",
      "type_revision": "sha256:0ee4626d16dfd7e51035c0e9c8a8b881680ca31390c1140bc14e8dc5623e04d8"
    },
    "pack": {
      "content": "Order is explicit: directive first, tactic second.",
      "conversion_sources": [],
      "fields": {
        "label": "Evidence review starter"
      },
      "members": [
        {
          "member_id": "z-directive",
          "required": true,
          "target": "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a",
          "target_mode": "revision"
        },
        {
          "member_id": "a-tactic",
          "required": true,
          "target": "sha256:2e19e92b1161ca7a6be2fd5b15e6f61c9a456efa682541732c7dedd52caf66ae",
          "target_mode": "revision"
        }
      ],
      "relationships": [
        {
          "relation": "example.charter/includes",
          "required": true,
          "target": "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a",
          "target_mode": "revision"
        },
        {
          "relation": "example.charter/includes",
          "required": true,
          "target": "sha256:2e19e92b1161ca7a6be2fd5b15e6f61c9a456efa682541732c7dedd52caf66ae",
          "target_mode": "revision"
        }
      ],
      "shape": "list",
      "title": "Evidence review pack",
      "type_id": "example.charter/pack",
      "type_revision": "sha256:d07edea18e4af016314e5d04b3ff2ffe5e7df97a51d1b9ab84f79d2087b59464"
    },
    "tactic": {
      "content": "Compare the proposed inputs to the stored signed revisions.",
      "conversion_sources": [],
      "fields": {
        "code": "COMPARE-01",
        "goal": "Make each decision independently checkable"
      },
      "members": [],
      "relationships": [
        {
          "relation": "example.charter/implements",
          "required": true,
          "target": "sha256:fbc6d16e1765f1b63fe454a56a382cc312a8f8fd1c8fc553cb9b3c304b95298a",
          "target_mode": "revision"
        }
      ],
      "shape": "document",
      "title": "Inspect evidence before approving",
      "type_id": "example.charter/tactic",
      "type_revision": "sha256:7fc6345f52c104473ff25c984913397eb7196d7c32a32a12348c15365585700f"
    }
  }
}`
