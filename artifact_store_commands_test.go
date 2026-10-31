package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	c "hubnot/internal/contribution"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func artifactStoreBinary(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("HN_WP05_BINARY"); p != "" {
		return p
	}
	return schemaCommandBinary(t)
}

type artifactCLIFixture struct {
	t                         *testing.T
	binary, repo, home, actor string
	overrides                 map[string]string
}

func artifactCLI(t *testing.T) *artifactCLIFixture { return artifactCLIFormat(t, "sha1") }
func artifactCLIFormat(t *testing.T, format string) *artifactCLIFixture {
	t.Helper()
	root := t.TempDir()
	f := &artifactCLIFixture{t: t, binary: artifactStoreBinary(t), repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
	f.git("init", "--quiet", "--object-format="+format)
	f.call("", "", "init", "--name", "Artifact author")
	cmd := exec.Command(f.binary, "identity", "show", "--json")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironment(f.home)
	b, e := cmd.Output()
	if e != nil {
		t.Fatalf("identity %v %s", e, b)
	}
	// The native identity envelope retains its own machine shape.
	var v map[string]any
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	f.actor = v["data"].(map[string]any)["actor"].(string)
	return f
}
func (f *artifactCLIFixture) git(args ...string) string {
	f.t.Helper()
	if e := os.MkdirAll(f.repo, 0700); e != nil {
		f.t.Fatal(e)
	}
	c := exec.Command("git", args...)
	c.Dir = f.repo
	c.Env = operationalTestEnvironment(f.home)
	b, e := c.CombinedOutput()
	if e != nil {
		f.t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func (f *artifactCLIFixture) call(input, code string, args ...string) map[string]any {
	f.t.Helper()
	c := exec.Command(f.binary, args...)
	c.Dir = f.repo
	c.Env = operationalTestEnvironmentWith(f.home, f.overrides)
	c.Stdin = strings.NewReader(input)
	b, e := c.Output()
	if args[0] == "init" {
		if e != nil {
			f.t.Fatalf("init %v %s", e, b)
		}
		return nil
	}
	var v map[string]any
	if je := json.Unmarshal(b, &v); je != nil {
		f.t.Fatalf("JSON %v %s", je, b)
	}
	if code == "" {
		if e != nil || v["ok"] != true {
			f.t.Fatalf("positive %v %s", e, b)
		}
	} else {
		p, ok := v["error"].(map[string]any)
		if e == nil || v["ok"] != false || v["data"] != nil || !ok || p["code"] != code {
			f.t.Fatalf("want exact %s: %v %s", code, e, b)
		}
	}
	return v
}
func (f *artifactCLIFixture) publish(input, key, code string) map[string]any {
	return f.call(input, code, "artifact", "type", "publish", "--input", "-", "--actor", f.actor, "--operation", key, "--json")
}

const artifactStoreType = `{"parents":[],"payload":{"type_id":"example.test/note","schema_version":1,"shape":"document","schema":{"language":"hn.schema/1","root":{"type":"object","properties":{},"required":[],"additionalProperties":false}}}}`

func TestArtifactStorePublishCLI(t *testing.T) {
	f := artifactCLI(t)
	native := f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/memory")
	got := f.publish(artifactStoreType, "publish-1", "")["data"].(map[string]any)
	id := got["event_id"].(string)
	if got["resource_id"] != id || got["replayed"] != false || got["protocol"] != "hn-artifact/1" {
		t.Fatal(got)
	}
	refs := strings.Fields(f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/artifacts"))
	if len(refs) != 2 {
		t.Fatal(refs)
	}
	stream := sha256.Sum256([]byte("hn-artifact-stream-v1\x00" + f.actor))
	if refs[0] != "refs/hn/artifacts/"+f.actor+"/"+hex.EncodeToString(stream[:]) {
		t.Fatal(refs)
	}
	raw := []byte(f.git("show", refs[1]+":artifact.json"))
	sig := f.git("show", refs[1]+":signature")
	var env map[string]any
	if e := json.Unmarshal(raw, &env); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	if id != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("artifact identity differs from exact bytes")
	}
	public, _ := base64.RawStdEncoding.DecodeString(env["public_key"].(string))
	signature, _ := base64.RawStdEncoding.DecodeString(sig)
	if !ed25519.Verify(public, append([]byte("hn-artifact/1\x00"), raw...), signature) {
		t.Fatal("independent signature verification failed")
	}
	if env["sequence"] != float64(1) || env["previous"] != "" || env["actor"] != f.actor {
		t.Fatal(env)
	}
	if got := f.git("cat-file", "-p", refs[1]); !strings.HasSuffix(got, "hn artifact "+id) {
		t.Fatal(got)
	}
	// Recompute Cut from the public result/ref and the frozen literal grammar.
	digest := func(raw []byte) string { value := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(value[:]) }
	view, _ := json.Marshal(map[string]any{"admitted_ids": []string{id}, "visible_ids": []string{id}, "retained_refs": []any{}})
	cut, _ := json.Marshal(map[string]any{"repository_id": digest([]byte("hn-extension-repository-v1\x00" + filepath.Join(f.repo, ".git"))), "refs": []any{map[string]any{"ref": refs[0], "oid": refs[1]}}, "selection": map[string]any{"families": []string{}, "types": []string{"example.test/note"}}, "authority_commit": "", "policy_digest": "", "view_digest": digest(view)})
	if got["source_cut"] != digest(cut) {
		t.Fatal("source cut differs from independent frozen recomputation", got)
	}
	shown := f.call("", "", "artifact", "type", "show", id, "--json")["data"].(map[string]any)
	items := shown["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != id {
		t.Fatal(shown)
	}
	before := f.git("for-each-ref", "--format=%(refname) %(objectname)")
	again := f.publish(artifactStoreType, "publish-1", "")["data"].(map[string]any)
	if again["event_id"] != id || again["replayed"] != true {
		t.Fatal(again)
	}
	f.publish(strings.Replace(artifactStoreType, "example.test/note", "example.test/other", 1), "publish-1", "operation_conflict")
	if before != f.git("for-each-ref", "--format=%(refname) %(objectname)") {
		t.Fatal("replay/refusal altered public refs")
	}
	if native != f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/memory") {
		t.Fatal("custom write altered original native/memory history")
	}
	if bytes.Contains(raw, []byte("privateKey")) {
		t.Fatal("private identity leaked")
	}
}

func (f *artifactCLIFixture) gitInput(input string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironment(f.home)
	cmd.Stdin = strings.NewReader(input)
	b, e := cmd.CombinedOutput()
	if e != nil {
		f.t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func (f *artifactCLIFixture) snapshot() map[string]string {
	f.t.Helper()
	out := map[string]string{}
	for _, root := range []string{f.repo, f.home} {
		if _, e := os.Stat(root); os.IsNotExist(e) {
			continue
		}
		e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				info, e := d.Info()
				if e != nil {
					return e
				}
				out[p] = "directory:" + info.Mode().String()
				return nil
			}
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			sum := sha256.Sum256(b)
			info, e := d.Info()
			if e != nil {
				return e
			}
			out[p] = hex.EncodeToString(sum[:]) + ":" + info.Mode().String()
			return nil
		})
		if e != nil {
			f.t.Fatal(e)
		}
	}
	return out
}
func (f *artifactCLIFixture) unchanged(before map[string]string) {
	f.t.Helper()
	after := f.snapshot()
	if !reflect.DeepEqual(before, after) {
		for p, digest := range after {
			if before[p] != digest {
				f.t.Errorf("unexpected changed/created path %s", p)
			}
		}
		for p := range before {
			if _, ok := after[p]; !ok {
				f.t.Errorf("unexpected deleted path %s", p)
			}
		}
		f.t.Fatal("refusal/read altered fixture files, objects or private state")
	}
}
func artifactErrorShape(t *testing.T, v map[string]any, origin, code, recovery string, operation any) {
	t.Helper()
	p := v["error"].(map[string]any)
	if len(p) != 5 || p["origin"] != origin || p["code"] != code || p["recovery"] != recovery || !reflect.DeepEqual(p["operation"], operation) {
		t.Fatal(p)
	}
}
func TestArtifactStoreOpenAndRefusalsCLI(t *testing.T) {
	f := artifactCLI(t)
	typ := f.publish(artifactStoreType, "type", "")["data"].(map[string]any)["event_id"].(string)
	input := `{"record":null,"parents":[],"payload":{"shape":"document","type_id":"example.test/note","type_revision":"` + typ + `","title":"Data only","content":"Never execute this text","fields":{},"members":[],"relationships":[],"conversion_sources":[]}}`
	open := func(raw, key, code string) map[string]any {
		return f.call(raw, code, "artifact", "open", "--input", "-", "--actor", f.actor, "--operation", key, "--json")
	}
	result := open(input, "open", "")["data"].(map[string]any)
	id := result["event_id"].(string)
	if id == typ || result["type_id"] != "example.test/note" {
		t.Fatal(result)
	}
	before := f.snapshot()
	again := open(input, "open", "")["data"].(map[string]any)
	if again["event_id"] != id || again["replayed"] != true {
		t.Fatal(again)
	}
	f.unchanged(before)
	for _, control := range []struct{ name, raw, code string }{
		{"missing-schema", strings.Replace(input, typ, "sha256:"+strings.Repeat("0", 64), 1), "incomplete_dependency"},
		{"wrong-fields", strings.Replace(input, `"fields":{}`, `"fields":{"unexpected":true}`, 1), "invalid_input"},
		{"bad-title", strings.Replace(input, `"Data only"`, `" "`, 1), "invalid_input"},
		{"duplicate-key", strings.Replace(input, `"record":null`, `"record":null,"record":null`, 1), "invalid_input"},
		{"changed-replay", strings.Replace(input, "Data only", "Changed", 1), "operation_conflict"},
	} {
		t.Run(control.name, func(t *testing.T) {
			before := f.snapshot()
			key := "refused-" + control.name
			if control.name == "changed-replay" {
				key = "open"
			}
			v := open(control.raw, key, control.code)
			artifactErrorShape(t, v, "artifact", control.code, "do_not_retry", nil)
			f.unchanged(before)
		})
	}
	before = f.snapshot()
	v := f.call(artifactStoreType, "actor_mismatch", "artifact", "type", "publish", "--input", "-", "--actor", strings.Repeat("1", 64), "--operation", "wrong-signer", "--json")
	artifactErrorShape(t, v, "native", "actor_mismatch", "do_not_retry", nil)
	f.unchanged(before)
	f.publish(strings.Replace(artifactStoreType, "hn.schema/1", "hn.schema/2", 1), "unknown-schema", "unsupported_schema")
	// Mutation's actor binding is not an instruction to initialize a missing key.
	other := &artifactCLIFixture{t: t, binary: f.binary, repo: filepath.Join(t.TempDir(), "empty"), home: filepath.Join(t.TempDir(), "emptyhome"), actor: f.actor}
	other.git("init", "--quiet")
	before = other.snapshot()
	other.publish(artifactStoreType, "missing-signer", "repository_error")
	other.unchanged(before)
}
func TestArtifactStoreSelectedPolicyCLI(t *testing.T) {
	f := artifactCLI(t)
	typ := f.publish(artifactStoreType, "before-policy", "")["data"].(map[string]any)["event_id"].(string)
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{f.actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("artifact policy refusal")), Moderators: []string{f.actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	b, _ := json.Marshal(policy)
	p := filepath.Join(f.repo, ".hn", "policy.json")
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	f.git("add", ".hn/policy.json")
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Stored inert policy")
	// Presence in repository data is not policy selection.
	f.publish(strings.Replace(artifactStoreType, "example.test/note", "example.test/unselected", 1), "unselected-policy", "")
	branch := f.git("symbolic-ref", "HEAD")
	c1 := exec.Command(f.binary, "contribution", "authority", "set", "--ref", branch, "--expect", "absent")
	c1.Dir = f.repo
	c1.Env = operationalTestEnvironment(f.home)
	if b, e := c1.CombinedOutput(); e != nil {
		t.Fatalf("select policy %v %s", e, b)
	}
	// Existing outer native lock bookkeeping is permitted; exercise it once before
	// freezing all files, then assert the custom refusal has no additional effects.
	c1 = exec.Command(f.binary, "identity", "show", "--json")
	c1.Dir = f.repo
	c1.Env = operationalTestEnvironment(f.home)
	if b, e := c1.CombinedOutput(); e != nil {
		t.Fatalf("lock priming %v %s", e, b)
	}
	before := f.snapshot()
	v := f.publish(artifactStoreType, "refused-policy", "unsupported_capability")
	artifactErrorShape(t, v, "host", "unsupported_capability", "do_not_retry", nil)
	f.unchanged(before)
	opening := `{"record":null,"parents":[],"payload":{"shape":"document","type_id":"example.test/note","type_revision":"` + typ + `","title":"Selected refusal","content":"","fields":{},"members":[],"relationships":[],"conversion_sources":[]}}`
	v = f.call(opening, "unsupported_capability", "artifact", "open", "--input", "-", "--actor", f.actor, "--operation", "refused-open-policy", "--json")
	artifactErrorShape(t, v, "host", "unsupported_capability", "do_not_retry", nil)
	f.unchanged(before)
	if _, e := os.Stat(filepath.Join(f.repo, ".git", "hn", "artifacts", "intents", f.actor, strings.TrimPrefix(eventID([]byte("refused-policy")), "sha256:")+".json")); !os.IsNotExist(e) {
		t.Fatal("selected-policy mutation created private intent", e)
	}
}

func TestArtifactStoreTypeEvolutionCLI(t *testing.T) {
	f := artifactCLI(t)
	first := f.publish(artifactStoreType, "first", "")["data"].(map[string]any)["event_id"].(string)
	next := strings.Replace(artifactStoreType, `"parents":[]`, `"parents":["`+first+`"]`, 1)
	before := f.snapshot()
	f.publish(next, "same-version", "invalid_input")
	f.unchanged(before)
	next = strings.Replace(next, `"schema_version":1`, `"schema_version":2`, 1)
	second := f.publish(next, "second", "")["data"].(map[string]any)["event_id"].(string)
	f.call("", "", "artifact", "type", "show", first, "--json")
	f.call("", "", "artifact", "type", "show", second, "--json")
	ref := artifactRef(f.actor)
	latest := f.git("rev-parse", ref)
	prior := f.git("rev-parse", latest+"^")
	accepted := strings.Replace(ref, "refs/hn/", "refs/hn/remotes/known/", 1)
	f.git("update-ref", accepted, latest)
	f.git("update-ref", ref, prior)
	before = f.snapshot()
	f.publish(strings.Replace(artifactStoreType, "example.test/note", "example.test/third", 1), "stale-local-root", "actor_stream_conflict")
	f.unchanged(before)
	// Reads require no signer or repair and can still verify both accepted types.
	f.call("", "", "artifact", "type", "show", second, "--json")
}

func TestArtifactStoreSHA256GitCLI(t *testing.T) {
	f := artifactCLIFormat(t, "sha256")
	if got := f.git("rev-parse", "--show-object-format"); got != "sha256" {
		t.Fatal("wrong Git object format", got)
	}
	result := f.publish(artifactStoreType, "sha256-git", "")["data"].(map[string]any)
	commit := f.git("rev-parse", artifactRef(f.actor))
	if len(commit) != 64 {
		t.Fatal("SHA256 Git commit identity", commit)
	}
	raw := []byte(f.git("show", commit+":artifact.json"))
	sum := sha256.Sum256(raw)
	if result["event_id"] != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("artifact identity depends on transport hash", result)
	}
	f.call("", "", "artifact", "type", "show", result["event_id"].(string), "--json")
	// The frozen signed artifact retains exactly its public identity when its Git
	// transport is constructed under the other object format.
	ref := artifactFrozenRef(t, f)
	frozen := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	if len(frozen) != 64 {
		t.Fatal(frozen)
	}
	f.git("update-ref", ref, frozen)
	f.call("", "", "artifact", "type", "show", artifactVectorID, "--json")
}
