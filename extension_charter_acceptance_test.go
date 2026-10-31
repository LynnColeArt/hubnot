package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// This fixture launches the shipped example, never a test-child publisher.
type charterFixture struct {
	*artifactCLIFixture
	consumer, input, output, state string
	repositoryID, registrationID   string
	grants                         []string
}

func charterConsumer(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("HN_WP21_CONSUMER"); p != "" {
		raw, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		copy := filepath.Join(t.TempDir(), "charter-model")
		if e = os.WriteFile(copy, raw, 0700); e != nil {
			t.Fatal(e)
		}
		return copy
	}
	p := filepath.Join(t.TempDir(), "charter-model")
	c := exec.Command("go", "build", "-buildvcs=false", "-o", p, "./examples/extensions/charter-model")
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("standalone charter publisher must build: %v\n%s", e, b)
	}
	return p
}

func charterCLI(t *testing.T, consumer string) *charterFixture {
	t.Helper()
	if p := os.Getenv("HN_WP21_BINARY"); p != "" {
		t.Setenv("HN_WP05_BINARY", p)
	}
	f := &charterFixture{artifactCLIFixture: wp25ArtifactCLI(t), consumer: consumer,
		grants: []string{"native:identity=read", "artifact:example.charter/*=read,create,revise,type.publish"}}
	f.input, f.output, f.state = filepath.Join(f.home, "input.json"), filepath.Join(f.home, "output.json"), filepath.Join(f.home, "state.json")
	f.register()
	return f
}
func (f *charterFixture) register() {
	f.t.Helper()
	command := exec.Command(f.consumer, "--manifest")
	command.Env = operationalTestEnvironmentWith(f.home, f.overrides)
	b, e := command.Output()
	if e != nil {
		f.t.Fatal(e)
	}
	p := filepath.Join(f.home, "manifest.json")
	if e = os.MkdirAll(f.home, 0700); e != nil {
		f.t.Fatal(e)
	}
	if e = os.WriteFile(p, b, 0600); e != nil {
		f.t.Fatal(e)
	}
	f.call("", "", "extension", "register", "--manifest", p, "--executable", f.consumer, "--arg", "--input", "--arg", f.input, "--arg", "--output", "--arg", f.output, "--arg", "--state", "--arg", f.state, "--json")
	// A real registration must trip the same selected-path existence detector
	// later used to exclude private consumer state from the independent clone.
	registry := filepath.Join(f.overrides["XDG_CONFIG_HOME"], "hubnot", "extensions", "registry.json")
	if raw, err := os.ReadFile(registry); err != nil || len(raw) == 0 {
		f.t.Fatal("real registration missed actual OS registry", registry, err)
	}
	if _, err := os.Stat(registry); err != nil {
		f.t.Fatal("registry absence detector missed actual registration", err)
	}
	f.call("", "", "extension", "inspect", "example.charter/model", "--json")
	f.enable()
}
func (f *charterFixture) enable() {
	args := []string{"extension", "enable", "example.charter/model"}
	for _, g := range f.grants {
		args = append(args, "--grant", g)
	}
	data := f.call("", "", append(args, "--json")...)["data"].(map[string]any)
	f.repositoryID, f.registrationID = data["repository_id"].(string), data["registration_id"].(string)
}
func charterJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func (f *charterFixture) run(v map[string]any) map[string]any {
	f.t.Helper()
	v["expected_repository_id"], v["expected_registration_id"] = f.repositoryID, f.registrationID
	if e := os.WriteFile(f.input, charterJSON(f.t, v), 0600); e != nil {
		f.t.Fatal(e)
	}
	f.call("", "", "extension", "run", "example.charter/model", "--json")
	b, e := os.ReadFile(f.output)
	if e != nil {
		f.t.Fatal(e)
	}
	var out map[string]any
	if e = json.Unmarshal(b, &out); e != nil {
		f.t.Fatal(e)
	}
	if evidence := os.Getenv("HN_WP21_EVIDENCE"); evidence != "" {
		if e = os.MkdirAll(evidence, 0700); e != nil {
			f.t.Fatal(e)
		}
		file, e := os.CreateTemp(evidence, "public-transcript-*.json")
		if e != nil {
			f.t.Fatal(e)
		}
		_, e = file.Write(charterJSON(f.t, map[string]any{"test": f.t.Name(), "command": v, "result": out}))
		closeErr := file.Close()
		if e != nil || closeErr != nil {
			f.t.Fatal(e, closeErr)
		}
	}
	return out
}
func (f *charterFixture) request(method string, p map[string]any, code string) map[string]any {
	f.t.Helper()
	before := f.snapshot()
	out := f.run(map[string]any{"action": "call", "method": method, "params": p})
	response := out["response"].(map[string]any)
	if code == "" {
		if response["ok"] != true {
			f.t.Fatalf("%s: %s", method, charterJSON(f.t, out))
		}
	} else {
		err, _ := response["error"].(map[string]any)
		origin := "artifact"
		if code == "denied_access" {
			origin = "host"
		}
		if response["ok"] != false || response["data"] != nil || err["origin"] != origin || err["code"] != code || err["recovery"] != "do_not_retry" {
			f.t.Fatalf("want exact %s: %s", code, charterJSON(f.t, out))
		}
		after := f.snapshot()
		for _, p := range []string{f.input, f.output} {
			delete(before, p)
			delete(after, p)
		}
		for p := range before {
			if strings.Contains(p, "/extensions/session-") || strings.Contains(p, "/extensions/owners-") {
				delete(before, p)
			}
		}
		for p := range after {
			if strings.Contains(p, "/extensions/session-") || strings.Contains(p, "/extensions/owners-") {
				delete(after, p)
			}
		}
		if !reflect.DeepEqual(before, after) {
			f.t.Fatal("refusal changed protected public/private state")
		}
	}
	return response
}
func charterParams(f *charterFixture, kind string, id any, parents []string, p map[string]any, key string) map[string]any {
	return map[string]any{"kind": kind, "record": id, "parents": parents, "payload": p, "actor": f.actor, "operation": key}
}
func charterCopy(t *testing.T, p map[string]any) map[string]any {
	t.Helper()
	var v map[string]any
	_ = json.Unmarshal(charterJSON(t, p), &v)
	return v
}
func charterLeaks(raw []byte) bool {
	if bytes.Contains(raw, []byte("PRIVATE-WP21")) {
		return true
	}
	var bundle struct {
		Objects []struct {
			Data string `json:"data"`
		} `json:"objects"`
	}
	if json.Unmarshal(raw, &bundle) != nil {
		return false
	}
	for _, v := range bundle.Objects {
		body, e := base64.RawStdEncoding.DecodeString(v.Data)
		if e == nil && bytes.Contains(body, []byte("PRIVATE-WP21")) {
			return true
		}
	}
	return false
}
func charterSigned(t *testing.T, f *artifactCLIFixture, id string) map[string]any {
	t.Helper()
	for _, v := range f.call("", "", "artifact", "inspect", "--id", id, "--json")["data"].(map[string]any)["items"].([]any) {
		row := v.(map[string]any)
		if row["id"] == id {
			if row["validation"] != "verified" {
				t.Fatal(row)
			}
			envelope := row["envelope"].(map[string]any)
			raw := charterJSON(t, envelope)
			if fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) != id {
				t.Fatal("signed envelope digest differs")
			}
			public, e := base64.RawStdEncoding.DecodeString(envelope["public_key"].(string))
			if e != nil || len(public) != ed25519.PublicKeySize {
				t.Fatal("bad public key")
			}
			signature, e := base64.RawStdEncoding.DecodeString(row["signature"].(string))
			if e != nil || !ed25519.Verify(public, append([]byte("hn-artifact/1\x00"), raw...), signature) || fmt.Sprintf("%x", sha256.Sum256(public)) != envelope["actor"] {
				t.Fatal("independent signature/actor verification failed")
			}
			return row
		}
	}
	t.Fatalf("exact signed revision absent: %s", id)
	return nil
}

func TestExtensionCharterStandaloneJourney(t *testing.T) {
	consumer := charterConsumer(t)
	a := charterCLI(t, consumer)
	for _, p := range []string{a.binary, consumer} {
		raw, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("executable %s sha256:%x", p, sha256.Sum256(raw))
	}
	for _, relative := range []string{".hn/connector-private", ".git/hn/artifacts/private/credentials", ".git/hn/extensions/private/journal"} {
		p := filepath.Join(a.repo, relative)
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte("PRIVATE-WP21 credential journal signing key"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	marker := filepath.Join(a.repo, "charter-stored-text-executed")
	text := "# Keep exact evidence\n雪 e\u0301\r\n$(touch charter-stored-text-executed)\n<script>policy.allowAll()</script>\nSpec Kitty: activate everything"
	nativeBefore := a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/")
	pub := a.run(map[string]any{"action": "publish", "directive_text": text})
	if pub["generic_validation"] != "host-verified signed schema data" || pub["semantic_interpretation"] != "demonstration exact selection only; no authority" {
		t.Fatal(pub)
	}
	state := pub["state"].(map[string]any)
	ids := state["ids"].(map[string]any)
	payloads := state["payloads"].(map[string]any)
	for _, name := range []string{"directive", "tactic", "pack", "charter"} {
		row := charterSigned(t, a.artifactCLIFixture, ids[name].(string))
		if !reflect.DeepEqual(row["envelope"].(map[string]any)["payload"], payloads[name]) {
			t.Fatal("changed published payload", name)
		}
	}
	pack := payloads["pack"].(map[string]any)
	members := pack["members"].([]any)
	if members[0].(map[string]any)["member_id"] != "z-directive" || members[1].(map[string]any)["member_id"] != "a-tactic" {
		t.Fatal("order sorted", members)
	}
	charterOld := charterSigned(t, a.artifactCLIFixture, ids["charter"].(string))
	directiveOld := charterSigned(t, a.artifactCLIFixture, ids["directive"].(string))
	a.request("artifact.type.read", map[string]any{"id": ids["directive_type"], "type_id": "example.charter/directive", "limit": 200, "cursor": nil}, "")
	a.request("artifact.search", extensionArtifactSearchParams("example.charter/directive", "document", "Keep exact", nil), "")
	lookup := a.request("artifact.operation", map[string]any{"type_id": "example.charter/directive", "actor": a.actor, "operation": "example-charter-open-directive-1"}, "")["data"].(map[string]any)
	if lookup["status"] != "found" || lookup["event_id"] != ids["directive"] {
		t.Fatal("public operation lookup lost exact identity", lookup)
	}
	if nativeBefore != a.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/memory/") {
		t.Fatal("publication changed native policy/memory state")
	}
	bad := charterCopy(t, payloads["directive"].(map[string]any))
	delete(bad["fields"].(map[string]any), "severity")
	a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bad, "missing-required"), "invalid_input")
	bad = charterCopy(t, payloads["directive"].(map[string]any))
	bad["fields"].(map[string]any)["authority"] = true
	a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bad, "unknown-field"), "invalid_input")
	// Generic validity does not validate the example's own three-link selection.
	badTicket := charterCopy(t, payloads["charter"].(map[string]any))
	badTicket["relationships"] = []any{}
	badTicketID := extensionArtifactEvent(a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, badTicket, "generic-valid-domain-incomplete"), ""))
	charterSigned(t, a.artifactCLIFixture, badTicketID)
	stateRaw, e := os.ReadFile(a.state)
	if e != nil {
		t.Fatal(e)
	}
	invalidState := charterCopy(t, state)
	invalidState["ids"].(map[string]any)["charter"] = badTicketID
	if e = os.WriteFile(a.state, charterJSON(t, invalidState), 0600); e != nil {
		t.Fatal(e)
	}
	semantic := a.run(map[string]any{"action": "inspect-selection"})
	if semantic["generic_validation"] != "host-verified signed schema data" || semantic["semantic_interpretation"] != "rejected: example ticket requires three selections" {
		t.Fatal("generic validity confused with demonstration semantics", semantic)
	}
	if e = os.WriteFile(a.state, stateRaw, 0600); e != nil {
		t.Fatal(e)
	}
	// Every example-owned free-text field bound is exercised at/over the live schema.
	for _, test := range []struct{ name, field string }{{"directive", "code"}, {"tactic", "code"}, {"tactic", "goal"}, {"pack", "label"}, {"charter", "project"}} {
		bound := charterCopy(t, payloads[test.name].(map[string]any))
		bound["fields"].(map[string]any)[test.field] = strings.Repeat("x", 64)
		a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bound, test.name+"-"+test.field+"-at-bound"), "")
		bound["fields"].(map[string]any)[test.field] = strings.Repeat("x", 65)
		a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bound, test.name+"-"+test.field+"-over-bound"), "invalid_input")
	}
	for _, test := range []struct{ name, field, value string }{{"directive", "severity", "should"}, {"charter", "status", "selected"}} {
		bound := charterCopy(t, payloads[test.name].(map[string]any))
		bound["fields"].(map[string]any)[test.field] = test.value
		a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bound, test.field+"-at-bound"), "")
		bound["fields"].(map[string]any)[test.field] = test.value + "x"
		a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bound, test.field+"-over-bound"), "invalid_input")
	}
	// Independent B gets only public refs through an actual bare transport.
	b := charterCLI(t, consumer)
	if b.actor == a.actor {
		t.Fatal("actors not independent")
	}
	remote := filepath.Join(t.TempDir(), "public.git")
	a.git("init", "--bare", "--quiet", remote)
	for _, f := range []*charterFixture{a, b} {
		f.git("remote", "add", "public", remote)
	}
	charterID := ids["charter"].(string)
	docID := ids["directive"].(string)
	a.call("", "", "artifact", "push", "--remote", "public", "--id", charterID, "--json")
	b.call("", "", "artifact", "fetch", "--remote", "public", "--id", charterID, "--json")
	leftPayload := charterCopy(t, payloads["directive"].(map[string]any))
	leftPayload["content"] = "Left explicit edit " + text
	rightPayload := charterCopy(t, leftPayload)
	rightPayload["content"] = "Right independent edit"
	left := extensionArtifactEvent(a.request("artifact.mutate", charterParams(a, "record.revise", docID, []string{docID}, leftPayload, "left"), ""))
	right := extensionArtifactEvent(b.request("artifact.mutate", charterParams(b, "record.revise", docID, []string{docID}, rightPayload, "right"), ""))
	b.call("", "", "artifact", "push", "--remote", "public", "--id", charterID, "--id", right, "--json")
	a.call("", "", "artifact", "fetch", "--remote", "public", "--id", charterID, "--id", right, "--json")
	heads := []string{left, right}
	sort.Strings(heads)
	view := artifactJourneyView(t, a.artifactCLIFixture, docID)
	if view["conflict"] != true || view["state"] != nil || !reflect.DeepEqual(revisionHeads(t, view), heads) {
		t.Fatal(view)
	}
	a.request("artifact.mutate", charterParams(a, "record.revise", docID, []string{left}, leftPayload, "overwrite-conflict"), "resource_conflict")
	a.request("artifact.mutate", charterParams(a, "record.resolve", docID, heads, leftPayload, "denied-resolution"), "denied_access")
	a.grants = []string{"native:identity=read", "artifact:example.charter/*=read,create,revise,resolve,type.publish"}
	a.enable()
	a.request("artifact.mutate", charterParams(a, "record.resolve", docID, []string{left}, leftPayload, "partial-resolution"), "stale_revision")
	resolved := extensionArtifactEvent(a.request("artifact.mutate", charterParams(a, "record.resolve", docID, heads, leftPayload, "explicit-resolution"), ""))
	a.request("artifact.mutate", charterParams(a, "record.revise", docID, []string{left}, leftPayload, "stale-after-resolution"), "stale_revision")
	evolved := a.run(map[string]any{"action": "evolve", "parent": resolved})
	convertedPayload := evolved["payload"].(map[string]any)
	bound2 := charterCopy(t, convertedPayload)
	bound2["conversion_sources"] = []any{}
	bound2["fields"].(map[string]any)["rationale"] = strings.Repeat("r", 64)
	a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bound2, "rationale-at-bound"), "")
	bound2["fields"].(map[string]any)["rationale"] = strings.Repeat("r", 65)
	a.request("artifact.mutate", charterParams(a, "record.open", nil, []string{}, bound2, "rationale-over-bound"), "invalid_input")
	converted := evolved["converted"].(string)
	view = artifactJourneyView(t, a.artifactCLIFixture, docID)
	if !reflect.DeepEqual(revisionHeads(t, view), []string{converted}) || view["state"].(map[string]any)["type_revision"] == directiveOld["envelope"].(map[string]any)["payload"].(map[string]any)["type_revision"] {
		t.Fatal("conversion did not change current schema", view)
	}
	if !reflect.DeepEqual(charterOld, charterSigned(t, a.artifactCLIFixture, charterID)) || !reflect.DeepEqual(directiveOld, charterSigned(t, a.artifactCLIFixture, docID)) {
		t.Fatal("old selected signed payload changed")
	}
	interpret := a.run(map[string]any{"action": "inspect-selection"})
	if interpret["selected_directive"] != docID || interpret["selected_text"] != text {
		t.Fatal(interpret)
	}
	// Transfer all current public streams with exact selection still selecting old revisions.
	a.call("", "", "artifact", "push", "--remote", "public", "--id", charterID, "--id", converted, "--json")
	dst := wp25ArtifactCLI(t)
	dst.git("remote", "add", "public", remote)
	dst.call("", "", "artifact", "fetch", "--remote", "public", "--id", charterID, "--id", converted, "--json")
	if !reflect.DeepEqual(a.call("", "", "artifact", "history", docID, "--json")["data"].(map[string]any)["items"], dst.call("", "", "artifact", "history", docID, "--json")["data"].(map[string]any)["items"]) {
		t.Fatal("independent clone lost signed history")
	}
	if got := artifactJourneyView(t, dst, docID); !reflect.DeepEqual(revisionHeads(t, got), []string{converted}) {
		t.Fatal("current clone view did not follow heads", got)
	}
	a.call("", "", "extension", "disable", "example.charter/model", "--json")
	a.call("", "", "extension", "remove", "example.charter/model", "--json")
	if e := os.Remove(consumer); e != nil {
		t.Fatal(e)
	}
	for _, f := range []*artifactCLIFixture{a.artifactCLIFixture, dst} {
		if !reflect.DeepEqual(charterOld, charterSigned(t, f, charterID)) || !reflect.DeepEqual(directiveOld, charterSigned(t, f, docID)) {
			t.Fatal("clone lost old exact selection")
		}
		for _, id := range []string{ids["tactic"].(string), ids["pack"].(string)} {
			if !reflect.DeepEqual(charterSigned(t, a.artifactCLIFixture, id), charterSigned(t, f, id)) {
				t.Fatal("clone payload mismatch")
			}
		}
		for _, args := range [][]string{{"artifact", "search", "--type", "example.charter/directive", "--literal", "historical", "--limit", "200", "--json"}, {"artifact", "search", "--type", "example.charter/directive", "--regex", "Converted.*guidance", "--limit", "200", "--json"}, {"artifact", "list", "--type", "example.charter/pack", "--shape", "list", "--limit", "200", "--json"}, {"artifact", "history", docID, "--json"}, {"artifact", "type", "show", ids["directive_type"].(string), "--json"}} {
			out := f.call("", "", args...)
			if len(out["data"].(map[string]any)["items"].([]any)) == 0 {
				t.Fatal("generic selection unexpectedly empty", args)
			}
		}
		bundle, _ := exchangeExport(f, charterID, converted)
		raw, _ := os.ReadFile(bundle)
		if charterLeaks(raw) {
			t.Fatal("private state exported")
		}
		f.call("", "", "issue", "open", "--operation", "after-consumer-removal", "--json", "Ordinary issue remains usable")
		f.git("-c", "user.name=Example", "-c", "user.email=example@example.invalid", "commit", "--allow-empty", "-qm", "Native memory anchor")
		c := exec.Command(f.binary, "memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "exact", "--content", "Ordinary memory remains usable", "--json")
		c.Dir = f.repo
		c.Env = operationalTestEnvironmentWith(f.home, f.overrides)
		if raw, e := c.CombinedOutput(); e != nil {
			t.Fatalf("native memory: %v %s", e, raw)
		}
	}
	if _, e := os.Stat(filepath.Join(dst.overrides["XDG_CONFIG_HOME"], "hubnot", "extensions", "registry.json")); !os.IsNotExist(e) {
		t.Fatal("destination has a consumer registry", e)
	}
	for _, f := range []*artifactCLIFixture{a.artifactCLIFixture, b.artifactCLIFixture, dst} {
		if _, e := os.Stat(filepath.Join(f.repo, "charter-stored-text-executed")); !os.IsNotExist(e) {
			t.Fatal("stored directive executed")
		}
	}
	// Deliberately activate each detector to prove it observes its asserted effect.
	if e := os.WriteFile(marker, []byte("detector control"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(marker); e != nil {
		t.Fatal("execution detector failed")
	}
	_ = os.Remove(marker)
	if !charterLeaks(charterJSON(t, map[string]any{"objects": []any{map[string]any{"data": base64.RawStdEncoding.EncodeToString([]byte("PRIVATE-WP21 credential"))}}})) {
		t.Fatal("decoded public object leak detector failed")
	}
	if evidence := os.Getenv("HN_WP21_EVIDENCE"); evidence != "" {
		if e := os.MkdirAll(evidence, 0700); e != nil {
			t.Fatal(e)
		}
		bundle, _ := exchangeExport(dst, charterID, converted)
		raw, _ := os.ReadFile(bundle)
		if e := os.WriteFile(filepath.Join(evidence, "charter-public-bundle.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(evidence, "charter-state.json"), charterJSON(t, state), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range []string{a.binary, consumer} {
		if raw, e := os.ReadFile(p); e == nil {
			t.Logf("executable %s sha256:%x", p, sha256.Sum256(raw))
		}
	}
}

func TestExtensionCharterMissingRequiredType(t *testing.T) {
	consumer := charterConsumer(t)
	a, b := charterCLI(t, consumer), charterCLI(t, consumer)
	pub := a.run(map[string]any{"action": "publish"})
	state := pub["state"].(map[string]any)
	ids := state["ids"].(map[string]any)
	revisionFetch(b.artifactCLIFixture, a.artifactCLIFixture, "source")
	p := charterCopy(t, state["payloads"].(map[string]any)["directive"].(map[string]any))
	p["content"] = "dependent stream requiring an external schema"
	id := extensionArtifactEvent(b.request("artifact.mutate", charterParams(b, "record.open", nil, []string{}, p, "dependent"), ""))
	path, _ := exchangeExport(b.artifactCLIFixture, id)
	raw, _ := os.ReadFile(path)
	var bundle map[string]any
	_ = json.Unmarshal(raw, &bundle)
	omit := map[string]bool{}
	for _, oid := range strings.Fields(a.git("rev-list", "--objects", "--no-object-names", a.git("rev-parse", artifactRef(a.actor)))) {
		omit[oid] = true
	}
	objects := []any{}
	for _, v := range bundle["objects"].([]any) {
		if !omit[v.(map[string]any)["oid"].(string)] {
			objects = append(objects, v)
		}
	}
	bundle["objects"] = objects
	refs := []any{}
	for _, v := range bundle["refs"].([]any) {
		if v.(map[string]any)["ref"] != artifactRef(a.actor) {
			refs = append(refs, v)
		}
	}
	bundle["refs"] = refs
	dst := wp25ArtifactCLI(t)
	before := dst.snapshot()
	result := dst.call("", "incomplete_dependency", "artifact", "import", "--input", exchangeWrite(t, bundle), "--json")
	if !strings.Contains(result["error"].(map[string]any)["message"].(string), "schema") {
		t.Fatal(result)
	}
	dst.unchanged(before)
	t.Logf("omitted exact required declaration %s while retaining valid independent dependent stream", ids["directive_type"])
}

func Example_charterPublicModel() {
	fmt.Println("example.charter: inert documents, ordered packs and exact selection tickets") // Output: example.charter: inert documents, ordered packs and exact selection tickets
}
