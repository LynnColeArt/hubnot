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
	f.grant(false)
	return f
}
func (f *inventoryFixture) grant(resolve bool) {
	actions := "read,create,revise,type.publish"
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
		inventoryRemote(b.artifactCLIFixture, "push", right)
		inventoryRemote(a.artifactCLIFixture, "fetch", right)
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
	dst.git("remote", "add", "public", remote)
	for _, model := range []string{"item", "maintenance", "stock"} {
		id := converted[model]
		inventoryRemote(a.artifactCLIFixture, "push", id)
		inventoryRemote(dst, "fetch", id)
		if !reflect.DeepEqual(closureItems(a.artifactCLIFixture, id), closureItems(dst, id)) {
			t.Fatal("public signed closure changed", model)
		}
		if !reflect.DeepEqual(inventorySelected(t, dst, opening[model]), originals[model]) {
			t.Fatal("old meaning changed in clone")
		}
		dst.call("", "", "artifact", "type", "show", types[model], "--json")
		dst.call("", "", "artifact", "history", opening[model], "--json")
	}
	inventoryHostShapeSearch(t, remote, converted)
	inventoryGenericReads(t, dst, converted)
	if a.git("rev-parse", "HEAD") != policyCommit {
		t.Fatal("stored data changed selected native policy commit")
	}
	inventoryRemoveAndNative(t, a, converted, marker)
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
		for _, p := range []string{filepath.Join(f.repo, ".git", "hn", "extensions", "enabled.json"), filepath.Join(f.home, ".config", "hubnot", "extensions", "registry.json"), filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", a.actor)} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Fatal("clone inherited consumer/private state", p, err)
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
	f.call("", "", "memory", "record", "--kind", "observation", "--at", "HEAD", "--applies", "exact", "--content", "Ordinary memory after removal", "--json")
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
	for _, id := range ids {
		inventoryRemote(src, "push", id)
		inventoryRemote(dst, "fetch", id)
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
	t.Logf("approved public charter exchange %s exact openings %v", expectedDigest, ids)
}

// The publisher deliberately has no generic read/search client. This independent
// raw TEST-only caller exercises the frozen host's shape filter directly.
func inventoryHostShapeSearch(t *testing.T, remote string, ids map[string]string) {
	t.Helper()
	if host := os.Getenv("HN_WP22_BINARY"); host != "" {
		t.Setenv("HN_WP18_BINARY", host)
	}
	f := extensionArtifactCLI(t, "artifact:example.inventory/*=read")
	f.git("remote", "add", "public", remote)
	for _, id := range ids {
		inventoryRemote(f.artifactCLIFixture, "fetch", id)
	}
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
			cmd.Env = operationalTestEnvironment(f.home)
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
