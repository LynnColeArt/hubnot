package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestArtifactGitIndependentClonesCLI(t *testing.T) {
	src, dst := wp25ArtifactCLI(t), wp25ArtifactCLI(t)
	// The destination is independently initialized and must retain its own
	// legitimate signer. Resolve the selected Git directory rather than HOME.
	sourceGitDir := src.git("rev-parse", "--absolute-git-dir")
	destinationGitDir := dst.git("rev-parse", "--absolute-git-dir")
	sourceKey := filepath.Join(sourceGitDir, "hn", "identities", src.actor+".json")
	sourceKeyBytes, sourceKeyError := os.ReadFile(sourceKey)
	var sourceIdentityRecord struct {
		PrivateKey string `json:"privateKey"`
	}
	if sourceKeyError != nil || json.Unmarshal(sourceKeyBytes, &sourceIdentityRecord) != nil || sourceIdentityRecord.PrivateKey == "" {
		t.Fatal("source private key fixture unavailable", sourceKeyError)
	}
	if src.actor == dst.actor {
		t.Fatal("independent destination reused source actor")
	}
	destinationSourceKey := filepath.Join(destinationGitDir, "hn", "identities", src.actor+".json")
	sourceKeyAbsent := func() bool {
		_, err := os.Lstat(destinationSourceKey)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal("inspect source-key absence", err)
		}
		return os.IsNotExist(err)
	}
	if !sourceKeyAbsent() {
		t.Fatal("source key already present in independent destination")
	}
	destinationKeyring := func() map[string]string {
		out := map[string]string{}
		for path, digest := range dst.snapshot() {
			root := filepath.Join(destinationGitDir, "hn")
			if strings.HasPrefix(path, filepath.Join(root, "identities")+string(os.PathSeparator)) || path == filepath.Join(root, "active") || path == filepath.Join(root, "rotation.json") || path == filepath.Join(root, "identity.json") {
				out[path] = digest
			}
		}
		return out
	}
	destinationIdentity := destinationKeyring()
	if destinationIdentity[filepath.Join(destinationGitDir, "hn", "identities", dst.actor+".json")] == "" || destinationIdentity[filepath.Join(destinationGitDir, "hn", "active")] == "" {
		t.Fatal("destination legitimate identity fixture unavailable")
	}
	remote := filepath.Join(t.TempDir(), "public.git")
	src.git("init", "--bare", "--quiet", remote)
	src.git("remote", "add", "public", remote)
	dst.git("remote", "add", "public", remote)
	schema := revisionEvent(src.publish(artifactStoreType, "schema", ""))
	opening := revisionEvent(revisionCall(src, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "transport 雪 e\u0301")), "opening", ""))
	tail := revisionEvent(revisionCall(src, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "unselected")), "tail", ""))
	marker := "WP09-PRIVATE-SIGNING-GRANT-INTENT-MARKER"
	for _, relative := range []string{".git/hn/extensions/private-grant", ".git/hn/artifacts/private-journal", ".hn/private-credentials"} {
		path := filepath.Join(src.repo, relative)
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte(marker), 0600); e != nil {
			t.Fatal(e)
		}
	}
	before := src.snapshot()
	result := src.call("", "", "artifact", "push", "--remote", "public", "--id", opening, "--json")["data"].(map[string]any)
	if result["mode"] != "accepted" || result["records"] != float64(2) || result["objects"] != float64(8) || len(result) != 7 {
		t.Fatal("push contract", result)
	}
	descriptor := map[string]any{"protocol": "hn-artifact-transfer/1", "remote": "public", "refs": result["refs"], "ids": []string{opening}, "mode": "accepted"}
	raw := closurePublicJSON(t, descriptor)
	if result["output_digest"] != eventID(raw) {
		t.Fatal("transfer digest", result)
	}
	src.unchanged(before)
	tip := result["refs"].([]any)[0].(map[string]any)["oid"].(string)
	expectedObjects := strings.Fields(src.git("rev-list", "--objects", "--no-object-names", tip))
	sort.Strings(expectedObjects)
	local := dst.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/artifacts/")
	received := dst.call("", "", "artifact", "fetch", "--remote", "public", "--id", opening, "--json")["data"].(map[string]any)
	want := []string{schema, opening}
	sort.Strings(want)
	b, _ := json.Marshal(received["imported_ids"])
	w, _ := json.Marshal(want)
	if string(b) != string(w) || len(received) != 6 || received["source_cut"] == "" {
		t.Fatal("fetch contract", received)
	}
	if got := closureItemIDs(t, closureItems(dst, opening)); !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
	if got := revisionReadView(dst, opening)["state"].(map[string]any)["content"]; got != "transport 雪 e\u0301" {
		t.Fatal(got)
	}
	if dst.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/artifacts/") != local {
		t.Fatal("local signing refs changed")
	}
	dst.call("", "not_found", "artifact", "show", tail, "--json")
	for _, path := range []string{filepath.Join(destinationGitDir, "hn", "artifacts", "intents", src.actor)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("source private state copied", path)
		}
	}
	if !sourceKeyAbsent() {
		t.Fatal("source actor private key copied into destination keyring")
	}
	if !reflect.DeepEqual(destinationIdentity, destinationKeyring()) {
		t.Fatal("fetch changed destination legitimate actor or private-key bytes")
	}
	// A harmless private test marker at exactly the source actor's destination
	// key path must trip the detector. Never copy source signing material.
	if err := os.WriteFile(destinationSourceKey, []byte("PRIVATE SOURCE-KEY DETECTOR CONTROL"), 0600); err != nil {
		t.Fatal(err)
	}
	if sourceKeyAbsent() {
		t.Fatal("source-key absence detector missed deliberate selected-path file")
	}
	if err := os.Remove(destinationSourceKey); err != nil {
		t.Fatal(err)
	}
	if !sourceKeyAbsent() || !reflect.DeepEqual(destinationIdentity, destinationKeyring()) {
		t.Fatal("source-key control failed to restore independent destination identity")
	}
	actualObjects := strings.Fields(dst.git("rev-list", "--objects", "--no-object-names", tip))
	sort.Strings(actualObjects)
	if !reflect.DeepEqual(actualObjects, expectedObjects) {
		t.Fatal("exact raw public object inventory changed")
	}
	for path := range dst.snapshot() {
		info, e := os.Stat(path)
		if e != nil || info.IsDir() {
			continue
		}
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(b), marker) {
			t.Fatal("private marker copied", path)
		}
	}
	refs := dst.git("for-each-ref", "--format=%(refname)", "refs/hn/remotes/public/artifacts/")
	if !strings.Contains(refs, src.actor) {
		t.Fatal("accepted namespace", refs)
	}
}

func TestArtifactGitCausalRefusalsCLI(t *testing.T) {
	for _, tc := range []struct{ name, code, message string }{
		{"signature", "verification_failure", "signature"},
		{"schema", "incomplete_dependency", "schema missing"},
		{"unsupported-schema", "unsupported_schema", "schema"},
		{"unknown-protocol", "invalid_input", "protocol"},
		{"missing-object", "verification_failure", "fetch failed"},
		{"corrupt-tree", "invalid_history", "tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, dst := artifactCLI(t), artifactCLI(t)
			remote := filepath.Join(t.TempDir(), "public.git")
			src.git("init", "--bare", "--quiet", remote)
			dst.git("remote", "add", "public", remote)
			raw, sig := artifactSignedCLI(t, func(m map[string]any) {
				switch tc.name {
				case "schema":
					m["kind"] = "record.open"
					m["payload"] = revisionPayload("sha256:"+strings.Repeat("d", 64), "missing")
				case "unsupported-schema":
					m["payload"].(map[string]any)["schema"].(map[string]any)["language"] = "hn.schema/9"
				case "unknown-protocol":
					m["protocol"] = "hn-artifact/9"
				}
			})
			if tc.name == "signature" {
				sig = "A" + sig[1:]
				if sig == artifactVectorSignature {
					t.Fatal("bad fixture")
				}
			}
			var env map[string]any
			_ = json.Unmarshal([]byte(raw), &env)
			id := eventID([]byte(raw))
			actor := env["actor"].(string)
			commit := artifactRawCommit(t, src, raw, sig, "", "hn artifact "+id, tc.name == "corrupt-tree")
			src.git("update-ref", artifactRef(actor), commit)
			src.git("push", "--quiet", remote, artifactRef(actor))
			if tc.name == "missing-object" {
				oid := src.git("rev-parse", commit+":signature")
				if e := os.Remove(filepath.Join(remote, "objects", oid[:2], oid[2:])); e != nil {
					t.Fatal(e)
				}
			}
			before := dst.snapshot()
			v := dst.call("", tc.code, "artifact", "fetch", "--remote", "public", "--id", id, "--json")
			if !strings.Contains(v["error"].(map[string]any)["message"].(string), tc.message) {
				t.Fatal("wrong causal refusal", v)
			}
			artifactErrorShape(t, v, "artifact", tc.code, "do_not_retry", nil)
			dst.unchanged(before)
		})
	}
}

func TestArtifactGitSelectorRefusalsCLI(t *testing.T) {
	f := artifactCLI(t)
	id := revisionEvent(f.publish(artifactStoreType, "schema", ""))
	before := f.snapshot()
	for _, args := range [][]string{{}, {"--remote", "public"}, {"--remote", "public", "--id", "sha256:abc"}, {"--remote", "public", "--id", id, "--id", id}, {"--remote", "_bad", "--id", id}, {"--remote", strings.Repeat("x", 65), "--id", id}, {"--remote", "bad.lock", "--id", id}, {"--remote", "public", "--id", id, "--all"}, {"--remote", "public", "--id", id, "--preserve-opaque"}} {
		for _, action := range []string{"fetch", "push"} {
			call := append([]string{"artifact", action}, args...)
			call = append(call, "--json")
			f.call("", "invalid_input", call...)
			f.unchanged(before)
		}
	}
}

func TestArtifactGitLogicalHeadsAndOrderCLI(t *testing.T) {
	a, b, dst := artifactCLI(t), artifactCLI(t), artifactCLI(t)
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "target")), "open", ""))
	revisionFetch(b, a, "a")
	ar := revisionEvent(revisionCall(a, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "A")), "ar", ""))
	br := revisionEvent(revisionCall(b, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "B")), "br", ""))
	revisionFetch(a, b, "b")
	listType := strings.Replace(strings.Replace(artifactStoreType, "example.test/note", "example.test/list", 1), `"shape":"document"`, `"shape":"list"`, 1)
	ls := revisionEvent(a.publish(listType, "list-type", ""))
	p := revisionPayload(ls, "ordered")
	p["type_id"], p["shape"] = "example.test/list", "list"
	p["members"] = []any{map[string]any{"member_id": "z-first", "target": opening, "target_mode": "logical", "required": true}, map[string]any{"member_id": "a-second", "target": ar, "target_mode": "revision", "required": true}}
	p["relationships"] = []any{map[string]any{"relation": "example.test/optional", "target": "sha256:" + strings.Repeat("d", 64), "target_mode": "revision", "required": false}}
	list := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, p), "list", ""))
	remote := filepath.Join(t.TempDir(), "public.git")
	a.git("init", "--bare", "--quiet", remote)
	a.git("remote", "add", "public", remote)
	dst.git("remote", "add", "public", remote)
	a.call("", "", "artifact", "push", "--remote", "public", "--id", list, "--json")
	dst.call("", "", "artifact", "fetch", "--remote", "public", "--id", list, "--json")
	expected := closureItems(a, list)
	actual := closureItems(dst, list)
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal("transport changed exact public signed objects or diagnostics")
	}
	want := []string{schema, opening, ar, br, ls, list}
	sort.Strings(want)
	if !reflect.DeepEqual(closureItemIDs(t, actual), want) {
		t.Fatal("logical selection lost a head")
	}
	if heads := revisionHeads(t, revisionReadView(dst, opening)); len(heads) != 2 {
		t.Fatal(heads)
	}
}

func TestArtifactGitAncestryLimitCLI(t *testing.T) {
	f := artifactCLI(t)
	remote := filepath.Join(t.TempDir(), "public.git")
	f.git("init", "--bare", "--quiet", remote)
	f.git("remote", "add", "public", remote)
	previous, parent, last, actor := "", "", "", ""
	for sequence := 1; sequence <= 257; sequence++ {
		raw, sig := artifactSignedCLI(t, func(m map[string]any) {
			m["operation"] = fmt.Sprintf("transport-depth-%d", sequence)
			m["sequence"], m["previous"] = sequence, previous
			actor = m["actor"].(string)
		})
		last = eventID([]byte(raw))
		parent = artifactRawCommit(t, f, raw, sig, parent, "hn artifact "+last, false)
		previous = last
		if sequence == 256 {
			f.git("update-ref", artifactRef(actor), parent)
			f.call("", "", "artifact", "push", "--remote", "public", "--id", last, "--json")
		}
	}
	f.git("update-ref", artifactRef(actor), parent)
	before := f.snapshot()
	v := f.call("", "resource_limit", "artifact", "push", "--remote", "public", "--id", last, "--json")
	if !strings.Contains(v["error"].(map[string]any)["message"].(string), "ancestry") {
		t.Fatal(v)
	}
	f.unchanged(before)
	// Inspection deliberately retains its larger100000-record read contract.
	if got := closureItemIDs(t, closureItems(f, last)); len(got) != 257 {
		t.Fatal(got)
	}
}

func TestArtifactGitCancelledAndExpectedOld(t *testing.T) {
	src, dst := artifactCLI(t), artifactCLI(t)
	id := revisionEvent(src.publish(artifactStoreType, "schema", ""))
	remote := filepath.Join(t.TempDir(), "public.git")
	src.git("init", "--bare", "--quiet", remote)
	src.git("remote", "add", "public", remote)
	dst.git("remote", "add", "public", remote)
	src.call("", "", "artifact", "push", "--remote", "public", "--id", id, "--json")
	t.Chdir(dst.repo)
	before := dst.snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := replicateArtifacts(ctx, "fetch", "public", []string{id})
	revisionError(t, e, "cancelled")
	dst.unchanged(before)
	// A concurrent process changes exactly the expected-old target after capture.
	ref := "refs/hn/remotes/public/artifacts/" + src.actor + "/" + strings.TrimPrefix(artifactStream(src.actor), "sha256:")
	injected := dst.gitInput("competing public CAS value", "hash-object", "-w", "--stdin")
	replicationBeforePromoteHook = func() error { dst.git("update-ref", ref, injected); return nil }
	t.Cleanup(func() { replicationBeforePromoteHook = nil })
	_, e = replicateArtifacts(context.Background(), "fetch", "public", []string{id})
	revisionError(t, e, "concurrent_update")
	if dst.git("rev-parse", ref) != injected {
		t.Fatal("CAS overwrote the competing value")
	}
	if dst.git("for-each-ref", "--format=%(refname)", "refs/hn/artifacts/") != "" {
		t.Fatal("accepted transport wrote local signing roots")
	}
}

func TestArtifactGitFrozenTransferBounds(t *testing.T) {
	// Independent literal sizes are the frozen artifact-v1 section6 boundaries.
	if e := artifactTransferLimit(4096, 2048, 256, 33554432, 16777216); e != nil {
		t.Fatal("equality must be accepted", e)
	}
	for _, tc := range []struct {
		name                    string
		objects, records, depth int
		decoded, wire           int64
	}{
		{"object", 4097, 2048, 256, 33554432, 16777216},
		{"signed record", 4096, 2049, 256, 33554432, 16777216},
		{"ancestry", 4096, 2048, 257, 33554432, 16777216},
		{"decoded aggregate", 4096, 2048, 256, 33554433, 16777216},
		{"wire", 4096, 2048, 256, 33554432, 16777217},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := artifactTransferLimit(tc.objects, tc.records, tc.depth, tc.decoded, tc.wire)
			revisionError(t, e, "resource_limit")
			if !strings.Contains(e.Error(), tc.name) {
				t.Fatal(e)
			}
		})
	}
}

func TestArtifactGitPolicyBeforeEffectsCLI(t *testing.T) {
	f := artifactCLI(t)
	id := revisionEvent(f.publish(artifactStoreType, "schema", ""))
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{f.actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: c.Digest([]byte("transport policy refusal")), Moderators: []string{f.actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	b, _ := json.Marshal(policy)
	path := filepath.Join(f.repo, ".hn", "policy.json")
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	f.git("add", ".hn/policy.json")
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Inert policy")
	branch := f.git("symbolic-ref", "HEAD")
	cmd := exec.Command(f.binary, "contribution", "authority", "set", "--ref", branch, "--expect", "absent")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironment(f.home)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("select policy %v %s", e, b)
	}
	// Prime existing native outer-lock bookkeeping; transport itself gains none.
	cmd = exec.Command(f.binary, "identity", "show", "--json")
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironment(f.home)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("prime %v %s", e, b)
	}
	before := f.snapshot()
	for _, action := range []string{"fetch", "push"} {
		// The remote deliberately does not exist: policy must refuse before lookup.
		v := f.call("", "unsupported_capability", "artifact", action, "--remote", "absent", "--id", id, "--json")
		artifactErrorShape(t, v, "host", "unsupported_capability", "do_not_retry", nil)
		f.unchanged(before)
	}
}

func TestArtifactGitSHA256CLI(t *testing.T) {
	src, dst := artifactCLIFormat(t, "sha256"), artifactCLIFormat(t, "sha256")
	id := revisionEvent(src.publish(artifactStoreType, "schema", ""))
	remote := filepath.Join(t.TempDir(), "public.git")
	src.git("init", "--bare", "--quiet", "--object-format=sha256", remote)
	src.git("remote", "add", "public", remote)
	dst.git("remote", "add", "public", remote)
	sent := src.call("", "", "artifact", "push", "--remote", "public", "--id", id, "--json")["data"].(map[string]any)
	if len(sent["refs"].([]any)[0].(map[string]any)["oid"].(string)) != 64 {
		t.Fatal(sent)
	}
	dst.call("", "", "artifact", "fetch", "--remote", "public", "--id", id, "--json")
	if !reflect.DeepEqual(closureItems(src, id), closureItems(dst, id)) {
		t.Fatal("SHA256 Git transfer changed signed identity")
	}
}

func TestArtifactGitRefTransactionRecoveryShape(t *testing.T) {
	for _, tc := range []struct{ code, recovery string }{{"concurrent_update", "retry_identical"}, {"uncertain_operation", "inspect_operation"}} {
		raw, e := captureTestOutput(t, func() error { return artifactGitOutput(nil, artifactFailure(tc.code, "public ref transaction"), true) })
		if e == nil {
			t.Fatal("transaction failure became success")
		}
		var v map[string]any
		if e = json.Unmarshal([]byte(raw), &v); e != nil {
			t.Fatal(e)
		}
		artifactErrorShape(t, v, "artifact", tc.code, tc.recovery, nil)
	}
}

func TestArtifactGitWireBudgetBeforeCopyCLI(t *testing.T) {
	f := artifactCLI(t)
	remote := filepath.Join(t.TempDir(), "public.git")
	f.git("init", "--bare", "--quiet", remote)
	f.git("remote", "add", "public", remote)
	previous, parent, actor, schema := "", "", "", ""
	for sequence := 1; sequence <= 14; sequence++ {
		raw, sig := artifactSignedCLI(t, func(m map[string]any) {
			m["operation"] = fmt.Sprintf("wire-%d", sequence)
			m["sequence"], m["previous"] = sequence, previous
			actor = m["actor"].(string)
			if sequence > 1 {
				m["kind"] = "record.open"
				p := revisionPayload(schema, strings.Repeat("x", 900000))
				p["type_id"] = "example.inventory/note"
				p["fields"] = map[string]any{"code": "x"}
				m["payload"] = p
			}
		})
		previous = eventID([]byte(raw))
		if sequence == 1 {
			schema = previous
		}
		parent = artifactRawCommit(t, f, raw, sig, parent, "hn artifact "+previous, false)
	}
	// This independent encoder uses raw Git bodies and the frozen exchange object
	// shape, rather than calling production transport accounting to pick a bound.
	wireSize := func(tip string) int {
		objects := []map[string]string{}
		oids := strings.Fields(f.git("rev-list", "--objects", "--no-object-names", tip))
		sort.Strings(oids)
		for _, oid := range oids {
			kind := f.git("cat-file", "-t", oid)
			cmd := exec.Command("git", "cat-file", kind, oid)
			cmd.Dir = f.repo
			cmd.Env = operationalTestEnvironment(f.home)
			raw, e := cmd.Output()
			if e != nil {
				t.Fatal(e)
			}
			objects = append(objects, map[string]string{"oid": oid, "type": kind, "data": base64.RawStdEncoding.EncodeToString(raw)})
		}
		raw, e := json.Marshal(map[string]any{"protocol": "hn-artifact-exchange/1", "mode": "accepted", "refs": []map[string]string{{"ref": artifactRef(actor), "oid": tip}}, "objects": objects})
		if e != nil {
			t.Fatal(e)
		}
		return len(raw)
	}
	makeLast := func(n int) (string, string) {
		raw, sig := artifactSignedCLI(t, func(m map[string]any) {
			m["operation"] = "wire-last"
			m["sequence"], m["previous"] = 15, previous
			m["kind"] = "record.open"
			p := revisionPayload(schema, strings.Repeat("x", n))
			p["type_id"] = "example.inventory/note"
			p["fields"] = map[string]any{"code": "x"}
			m["payload"] = p
		})
		id := eventID([]byte(raw))
		return id, artifactRawCommit(t, f, raw, sig, parent, "hn artifact "+id, false)
	}
	n := 850000
	var id, tip string
	// Unpadded Base64 changes by one or two bytes per added source byte. Converge
	// on the largest representable accepted wire size, then use the next byte.
	for attempt := 0; attempt < 8; attempt++ {
		id, tip = makeLast(n)
		size := wireSize(tip)
		if size <= 16777216 && size >= 16777214 {
			break
		}
		n += (16777216 - size) * 3 / 4
		if attempt == 7 {
			t.Fatal("wire-bound fixture did not converge", size)
		}
	}
	acceptedSize := wireSize(tip)
	if acceptedSize > 16777216 || acceptedSize < 16777214 {
		t.Fatal("invalid equality-neighbor fixture", acceptedSize)
	}
	f.git("update-ref", artifactRef(actor), tip)
	f.call("", "", "artifact", "push", "--remote", "public", "--id", id, "--json")
	for wireSize(tip) <= 16777216 {
		n++
		id, tip = makeLast(n)
	}
	f.git("update-ref", artifactRef(actor), tip)
	before := f.snapshot()
	v := f.call("", "resource_limit", "artifact", "push", "--remote", "public", "--id", id, "--json")
	if !strings.Contains(v["error"].(map[string]any)["message"].(string), "wire") {
		t.Fatal(v)
	}
	f.unchanged(before)
	t.Logf("independent equivalent-wire accepted=%d first-over=%d", acceptedSize, wireSize(tip))
}

func TestArtifactGitRefGrammar(t *testing.T) {
	actor := strings.Repeat("a", 64)
	local := artifactRef(actor)
	for _, tc := range []struct {
		ref   string
		valid bool
	}{
		{local, true},
		{strings.Replace(local, "refs/hn/", "refs/hn/remotes/origin/", 1), true},
		{strings.Replace(local, "refs/hn/", "refs/hn/remotes/_bad/", 1), false},
		{strings.Replace(local, "refs/hn/", "refs/hn/remotes/bad.lock/", 1), false},
		{strings.Replace(local, "refs/hn/", "refs/hn/remotes/"+strings.Repeat("r", 65)+"/", 1), false},
		{"refs/hn/actors/" + actor, false},
		{local + "/extra", false},
	} {
		if got := validArtifactTransportRef(tc.ref); got != tc.valid {
			t.Fatal(tc.ref, got, tc.valid)
		}
	}
}

func TestArtifactGitDispatchedPushFailureIsUncertainCLI(t *testing.T) {
	f := artifactCLI(t)
	id := revisionEvent(f.publish(artifactStoreType, "schema", ""))
	remote := filepath.Join(t.TempDir(), "public.git")
	f.git("init", "--bare", "--quiet", remote)
	f.git("remote", "add", "public", remote)
	if e := os.WriteFile(filepath.Join(remote, "hooks", "pre-receive"), []byte("#!/bin/sh\nexit 1\n"), 0700); e != nil {
		t.Fatal(e)
	}
	before := f.snapshot()
	v := f.call("", "uncertain_operation", "artifact", "push", "--remote", "public", "--id", id, "--json")
	artifactErrorShape(t, v, "artifact", "uncertain_operation", "inspect_operation", nil)
	f.unchanged(before)
	if got := f.git("--git-dir="+remote, "for-each-ref", "--format=%(refname)", "refs/hn/artifacts/"); got != "" {
		t.Fatal("rejecting remote control published", got)
	}
}

func TestArtifactGitIndependentNativeSelectionBudgetCLI(t *testing.T) {
	src, dst := artifactCLI(t), artifactCLI(t)
	id := revisionEvent(src.publish(artifactStoreType, "schema", ""))
	remote := filepath.Join(t.TempDir(), "public.git")
	src.git("init", "--bare", "--quiet", remote)
	src.git("remote", "add", "public", remote)
	dst.git("remote", "add", "public", remote)
	native := func(f *artifactCLIFixture, wantFailure bool, args ...string) string {
		t.Helper()
		cmd := exec.Command(f.binary, args...)
		cmd.Dir = f.repo
		cmd.Env = operationalTestEnvironment(f.home)
		b, e := cmd.CombinedOutput()
		if (e != nil) != wantFailure {
			t.Fatalf("native command %v: %v %s", args, e, b)
		}
		return string(b)
	}
	native(src, false, "issue", "open", "Native budget control")
	src.git("push", "--quiet", remote, actorRef(src.actor))
	native(dst, false, "replication", "select", "public", "--actor", src.actor, "--max-objects", "1")
	selection := filepath.Join(dst.repo, ".git", "hn", "replication", "selections", "cHVibGlj.json")
	before, e := os.ReadFile(selection)
	if e != nil {
		t.Fatal(e)
	}
	src.call("", "", "artifact", "push", "--remote", "public", "--id", id, "--json")
	dst.call("", "", "artifact", "fetch", "--remote", "public", "--id", id, "--json")
	after, e := os.ReadFile(selection)
	if e != nil || string(before) != string(after) {
		t.Fatal("custom transport changed native selection", e)
	}
	result := native(dst, true, "sync", "public")
	if !strings.Contains(result, "max-objects budget") {
		t.Fatal("native lower budget was not enforced", result)
	}
	if got := dst.git("for-each-ref", "--format=%(refname)", acceptedActorRef("public", src.actor)); got != "" {
		t.Fatal("native over-budget root promoted", got)
	}
	if !reflect.DeepEqual(closureItemIDs(t, closureItems(dst, id)), []string{id}) {
		t.Fatal("native refusal altered custom accepted data")
	}
}

func TestArtifactGitFrozenRawBodiesCLI(t *testing.T) {
	// These exact accepted vectors include Git transport metadata without email
	// angle brackets. Their body bytes, not a commit-tree reconstruction, are the
	// portability contract. Neither repository has a signer or hn init state.
	text, e := os.ReadFile("kitty-specs/optional-extensions-01M661C0/contracts/conformance-fixtures.md")
	if e != nil {
		t.Fatal(e)
	}
	type literal struct {
		id, ref, commit string
		objects         map[string]struct {
			kind string
			raw  []byte
		}
	}
	vectors := []literal{}
	for _, name := range []string{"type-note-v1", "optional-missing-target"} {
		_, section, ok := strings.Cut(string(text), "## Signed vector: "+name+"\n")
		if !ok {
			t.Fatal("missing frozen vector", name)
		}
		section = strings.SplitN(section, "\n## ", 2)[0]
		blocks := regexp.MustCompile("(?s)~~~json\\n(.*?)\\n~~~").FindAllStringSubmatch(section, -1)
		if len(blocks) != 2 {
			t.Fatal("unexpected frozen vector shape", name)
		}
		var meta map[string]any
		if e = json.Unmarshal([]byte(blocks[1][1]), &meta); e != nil {
			t.Fatal(e)
		}
		git := meta["git_sha1"].(map[string]any)
		raw, e := hex.DecodeString(meta["canonical_utf8_hex"].(string))
		if e != nil {
			t.Fatal(e)
		}
		payloadOID, e := hex.DecodeString(git["payload_blob"].(string))
		if e != nil {
			t.Fatal(e)
		}
		signatureOID, e := hex.DecodeString(git["signature_blob"].(string))
		if e != nil {
			t.Fatal(e)
		}
		tree := append(append(append([]byte("100644 artifact.json\x00"), payloadOID...), []byte("100644 signature\x00")...), signatureOID...)
		v := literal{id: meta["id"].(string), ref: meta["local_ref"].(string), commit: git["commit"].(string), objects: map[string]struct {
			kind string
			raw  []byte
		}{
			git["payload_blob"].(string): {"blob", raw}, git["signature_blob"].(string): {"blob", []byte(meta["signature"].(string))}, git["tree"].(string): {"tree", tree}, git["commit"].(string): {"commit", []byte(git["commit_body"].(string))},
		}}
		if eventID(raw) != v.id || bytes.Contains(v.objects[v.commit].raw, []byte("<")) {
			t.Fatal("frozen identity/metadata control changed", name)
		}
		for oid, obj := range v.objects {
			h := sha1.New()
			fmt.Fprintf(h, "%s %d\x00", obj.kind, len(obj.raw))
			h.Write(obj.raw)
			if hex.EncodeToString(h.Sum(nil)) != oid {
				t.Fatal("independent frozen Git hash mismatch", oid)
			}
		}
		vectors = append(vectors, v)
	}
	keyless := func(name string) *artifactCLIFixture {
		root := t.TempDir()
		f := &artifactCLIFixture{t: t, binary: artifactStoreBinary(t), repo: filepath.Join(root, name), home: filepath.Join(root, "home")}
		f.git("init", "--quiet")
		return f
	}
	src, dst, broken, projected := keyless("source"), keyless("destination"), keyless("missing"), keyless("projected")
	rawGit := func(f *artifactCLIFixture, input []byte, args ...string) []byte {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = f.repo
		cmd.Env = operationalTestEnvironment(f.home)
		cmd.Stdin = bytes.NewReader(input)
		b, e := cmd.Output()
		if e != nil {
			t.Fatalf("raw fixture Git %v: %v", args, e)
		}
		return b
	}
	for _, v := range vectors {
		for oid, obj := range v.objects {
			got := strings.TrimSpace(string(rawGit(src, obj.raw, "hash-object", "-w", "--literally", "-t", obj.kind, "--stdin")))
			if got != oid {
				t.Fatal("literal fixture object changed", got, oid)
			}
		}
		src.git("update-ref", v.ref, v.commit)
	}
	remote := filepath.Join(t.TempDir(), "public.git")
	src.git("init", "--bare", "--quiet", remote)
	for _, f := range []*artifactCLIFixture{src, dst, broken, projected} {
		f.git("remote", "add", "public", remote)
	}
	selected := vectors[1].id
	sourceInspect := src.call("", "", "artifact", "inspect", "--id", selected, "--json")["data"].(map[string]any)
	want := []string{vectors[0].id, vectors[1].id}
	sort.Strings(want)
	if !reflect.DeepEqual(closureItemIDs(t, sourceInspect["items"].([]any)), want) {
		t.Fatal("literal source did not verify")
	}
	before := src.snapshot()
	src.call("", "", "artifact", "push", "--remote", "public", "--id", selected, "--json")
	src.unchanged(before)
	fetched := dst.call("", "", "artifact", "fetch", "--remote", "public", "--id", selected, "--json")["data"].(map[string]any)
	inspected := dst.call("", "", "artifact", "inspect", "--id", selected, "--json")["data"].(map[string]any)
	if inspected["source_cut"] == "" || fetched["source_cut"] != inspected["source_cut"] || !reflect.DeepEqual(sourceInspect["items"], inspected["items"]) {
		t.Fatal("literal public reconstruction/Cut changed", fetched, inspected)
	}
	optional := false
	for _, item := range inspected["items"].([]any) {
		for _, d := range item.(map[string]any)["diagnostics"].([]any) {
			if d.(map[string]any)["code"] == "missing_optional" {
				optional = true
			}
		}
	}
	if !optional {
		t.Fatal("signed optional target diagnostic lost")
	}
	for _, v := range vectors {
		for oid, obj := range v.objects {
			for _, f := range []*artifactCLIFixture{src, dst} {
				if !bytes.Equal(rawGit(f, nil, "cat-file", obj.kind, oid), obj.raw) {
					t.Fatal("raw public Git body changed", oid)
				}
			}
			if !bytes.Equal(rawGit(src, nil, "--git-dir="+remote, "cat-file", obj.kind, oid), obj.raw) {
				t.Fatal("remote rewrote raw public Git body", oid)
			}
		}
	}
	if got := dst.git("for-each-ref", "--format=%(refname)", "refs/hn/artifacts/"); got != "" {
		t.Fatal("keyless fetch wrote local signing roots", got)
	}
	for _, f := range []*artifactCLIFixture{src, dst} {
		for _, p := range []string{f.home, filepath.Join(f.repo, ".git", "hn", "artifacts", "intents")} {
			if _, e := os.Stat(p); !os.IsNotExist(e) {
				t.Fatal("public transport created identity or intent", p, e)
			}
		}
	}
	// Existing invalid destination facts must still fail before copying any public
	// source bytes, even though the artifact-specific raw writer can store them.
	bad := projected.gitInput("invalid existing destination root", "hash-object", "-w", "--stdin")
	projected.git("update-ref", vectors[0].ref, bad)
	projectionBefore := projected.snapshot()
	projected.call("", "verification_failure", "artifact", "fetch", "--remote", "public", "--id", selected, "--json")
	projected.unchanged(projectionBefore)
	// Removing only the required schema supplier leaves the exact signed record
	// present, but must not copy or promote an incomplete selection.
	src.git("--git-dir="+remote, "update-ref", "-d", vectors[0].ref)
	missingBefore := broken.snapshot()
	v := broken.call("", "incomplete_dependency", "artifact", "fetch", "--remote", "public", "--id", selected, "--json")
	if !strings.Contains(v["error"].(map[string]any)["message"].(string), "schema missing") {
		t.Fatal("wrong missing-schema cause", v)
	}
	broken.unchanged(missingBefore)
}

// This fixture contains only inert public type declarations from the frozen
// corpus. It never initializes a signer or handles private recovery material.
type artifactNamespaceFixture struct {
	src, dst                                     *artifactCLIFixture
	selected, oldTip, newTip, accepted, sentinel string
}

func newArtifactNamespaceFixture(t *testing.T) artifactNamespaceFixture {
	t.Helper()
	keyless := func(name string) *artifactCLIFixture {
		root := t.TempDir()
		f := &artifactCLIFixture{t: t, binary: artifactStoreBinary(t), repo: filepath.Join(root, name), home: filepath.Join(root, "home")}
		f.git("init", "--quiet")
		return f
	}
	src, dst := keyless("public-source"), keyless("public-destination")
	text, e := os.ReadFile("kitty-specs/optional-extensions-01M661C0/contracts/conformance-fixtures.md")
	if e != nil {
		t.Fatal(e)
	}
	f := artifactNamespaceFixture{src: src, dst: dst, sentinel: "refs/namespace-test/public-sentinel"}
	sourceRef := ""
	for i, name := range []string{"type-note-v1", "type-list-v1"} {
		_, section, ok := strings.Cut(string(text), "## Signed vector: "+name+"\n")
		if !ok {
			t.Fatal(name)
		}
		section = strings.SplitN(section, "\n## ", 2)[0]
		blocks := regexp.MustCompile("(?s)~~~json\\n(.*?)\\n~~~").FindAllStringSubmatch(section, -1)
		if len(blocks) != 2 {
			t.Fatal(name)
		}
		var meta map[string]any
		if e = json.Unmarshal([]byte(blocks[1][1]), &meta); e != nil {
			t.Fatal(e)
		}
		g := meta["git_sha1"].(map[string]any)
		raw, e := hex.DecodeString(meta["canonical_utf8_hex"].(string))
		if e != nil {
			t.Fatal(e)
		}
		payload, e := hex.DecodeString(g["payload_blob"].(string))
		if e != nil {
			t.Fatal(e)
		}
		signature, e := hex.DecodeString(g["signature_blob"].(string))
		if e != nil {
			t.Fatal(e)
		}
		tree := append(append(append([]byte("100644 artifact.json\x00"), payload...), []byte("100644 signature\x00")...), signature...)
		objects := []struct {
			kind, oid string
			raw       []byte
		}{{"blob", g["payload_blob"].(string), raw}, {"blob", g["signature_blob"].(string), []byte(meta["signature"].(string))}, {"tree", g["tree"].(string), tree}, {"commit", g["commit"].(string), []byte(g["commit_body"].(string))}}
		for _, o := range objects {
			if got := src.gitInput(string(o.raw), "hash-object", "-w", "--literally", "-t", o.kind, "--stdin"); got != o.oid {
				t.Fatal("public literal OID changed", got, o.oid)
			}
		}
		sourceRef = meta["local_ref"].(string)
		if i == 0 {
			f.oldTip = g["commit"].(string)
		} else {
			f.newTip = g["commit"].(string)
			f.selected = meta["id"].(string)
		}
	}
	src.git("update-ref", sourceRef, f.newTip)
	remote := filepath.Join(t.TempDir(), "public.git")
	src.git("init", "--bare", "--quiet", remote)
	src.git("--git-dir="+remote, "fetch", "--no-tags", "--no-write-fetch-head", src.repo, sourceRef+":"+sourceRef)
	dst.git("remote", "add", "public", remote)
	dst.git("fetch", "--no-tags", "--no-write-fetch-head", src.repo, f.oldTip+":"+f.sentinel)
	f.accepted = strings.Replace(sourceRef, "refs/hn/", "refs/hn/remotes/public/", 1)
	dst.git("update-ref", f.accepted, f.oldTip)
	return f
}

func namespacePublicBody(t *testing.T, f *artifactCLIFixture, ref string) []byte {
	t.Helper()
	cmd := exec.Command("git", "cat-file", "commit", ref)
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironment(f.home)
	b, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func assertArtifactNamespaceResult(t *testing.T, f artifactNamespaceFixture, before []byte) {
	t.Helper()
	sentinelOID := f.dst.git("rev-parse", f.sentinel)
	after := namespacePublicBody(t, f.dst, f.sentinel)
	accepted := f.dst.git("for-each-ref", "--format=%(refname) %(objectname) %(symref)", "refs/hn/remotes/public/artifacts/")
	t.Logf("public sentinel before=%s after=%s body_before=%x body_after=%x accepted_after=%q", f.oldTip, sentinelOID, before, after, accepted)
	if sentinelOID != f.oldTip || !bytes.Equal(before, after) {
		t.Fatal("artifact fetch advanced unrelated public sentinel", f.sentinel)
	}
	if accepted != f.accepted+" "+f.newTip {
		t.Fatal("fetch did not publish only the exact named direct accepted ref", accepted)
	}
	if got := f.dst.git("for-each-ref", "--format=%(refname)", "refs/hn/artifacts/"); got != "" {
		t.Fatal("public fixture acquired local artifact roots", got)
	}
	if _, e := os.Stat(f.dst.home); !os.IsNotExist(e) {
		t.Fatal("public fixture created private identity state", e)
	}
}

func TestArtifactGitFetchNamedAcceptedNamespaceCLI(t *testing.T) {
	for _, symbolic := range []bool{false, true} {
		name := "direct-positive"
		if symbolic {
			name = "preexisting-public-alias"
		}
		t.Run(name, func(t *testing.T) {
			f := newArtifactNamespaceFixture(t)
			if symbolic {
				f.dst.git("symbolic-ref", f.accepted, f.sentinel)
			}
			before := namespacePublicBody(t, f.dst, f.sentinel)
			t.Log("accepted_before", f.dst.git("for-each-ref", "--format=%(refname) %(objectname) %(symref)", "refs/hn/remotes/public/artifacts/"))
			f.dst.call("", "", "artifact", "fetch", "--remote", "public", "--id", f.selected, "--json")
			assertArtifactNamespaceResult(t, f, before)
		})
	}
}

func TestArtifactGitFetchNamedNamespaceRace(t *testing.T) {
	f := newArtifactNamespaceFixture(t)
	before := namespacePublicBody(t, f.dst, f.sentinel)
	t.Chdir(f.dst.repo)
	replicationBeforePromoteHook = func() error { f.dst.git("symbolic-ref", f.accepted, f.sentinel); return nil }
	t.Cleanup(func() { replicationBeforePromoteHook = nil })
	_, e := replicateArtifacts(context.Background(), "fetch", "public", []string{f.selected})
	if e != nil {
		t.Fatal(e)
	}
	assertArtifactNamespaceResult(t, f, before)
}

func TestArtifactGitNamedTransactionStaleOldIsAtomic(t *testing.T) {
	f := newArtifactNamespaceFixture(t)
	// Make only the verified public successor objects available; publication is
	// still exclusively the transaction under test, with one deliberately stale
	// expected value. The unrelated sentinel is public data throughout.
	f.dst.git("fetch", "--no-tags", "--no-write-fetch-head", f.src.repo, f.newTip)
	other := strings.Replace(f.accepted, "/public/", "/other/", 1)
	f.dst.git("symbolic-ref", f.accepted, f.sentinel)
	f.dst.git("update-ref", other, f.oldTip)
	before := f.dst.snapshot()
	t.Chdir(f.dst.repo)
	e := promoteReplicationRefs([]replicationPromotion{{Ref: f.accepted, NewOID: f.newTip, OldOID: f.oldTip}, {Ref: other, NewOID: f.newTip, OldOID: strings.Repeat("0", len(f.oldTip))}})
	if e == nil || !strings.Contains(e.Error(), "atomically promote accepted refs") {
		t.Fatal("wrong expected-old refusal", e)
	}
	f.dst.unchanged(before)
	if f.dst.git("rev-parse", f.sentinel) != f.oldTip || f.dst.git("symbolic-ref", f.accepted) != f.sentinel || f.dst.git("rev-parse", other) != f.oldTip {
		t.Fatal("stale transaction partially published")
	}
}

func TestArtifactGitFetchAcknowledgementCLI(t *testing.T) {
	for _, tc := range []struct{ name, code, recovery string }{{"committed-error", "uncertain_operation", "inspect_operation"}, {"unavailable-witness", "uncertain_operation", "inspect_operation"}, {"stale-old", "concurrent_update", "retry_identical"}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newArtifactNamespaceFixture(t)
			sourceBefore := f.src.snapshot()
			sentinelBefore := namespacePublicBody(t, f.dst, f.sentinel)
			competing := f.dst.gitInput("public competing selected-target value", "hash-object", "-w", "--stdin")
			realGit, e := exec.LookPath("git")
			if e != nil {
				t.Fatal(e)
			}
			wrap := t.TempDir()
			committed := filepath.Join(wrap, "committed")
			refused := filepath.Join(wrap, "stale-refused")
			denied := filepath.Join(wrap, "witness-denied")
			// Only the real final local stdin transaction is faulted. Quarantine Git,
			// raw copying and initial source/destination witnesses delegate normally.
			script := `#!/bin/sh
update=0; stream=0; explicit=0; witness=0
for arg do
 case "$arg" in update-ref) update=1 ;; --stdin) stream=1 ;; --git-dir=*) explicit=1 ;; for-each-ref) witness=1 ;; esac
done
if [ "$HN_WP09_ACK_MODE" = unavailable-witness ] && [ -f "$HN_WP09_ACK_COMMITTED" ] && [ "$witness" = 1 ]; then
 : > "$HN_WP09_ACK_DENIED"
 exit 87
fi
if [ "$PWD" = "$HN_WP09_ACK_REPO" ] && [ "$update$stream$explicit" = 110 ]; then
 if [ "$HN_WP09_ACK_MODE" = stale-old ]; then
  "$HN_WP09_ACK_GIT" update-ref --no-deref "$HN_WP09_ACK_REF" "$HN_WP09_ACK_COMPETING" "$HN_WP09_ACK_OLD" || exit 90
  "$HN_WP09_ACK_GIT" "$@"
  result=$?
  [ "$result" != 0 ] || exit 91
  : > "$HN_WP09_ACK_REFUSED"
  exit "$result"
 fi
 "$HN_WP09_ACK_GIT" "$@" || exit $?
 : > "$HN_WP09_ACK_COMMITTED"
 exit 86
fi
exec "$HN_WP09_ACK_GIT" "$@"
`
			if e = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(f.dst.binary, "artifact", "fetch", "--remote", "public", "--id", f.selected, "--json")
			cmd.Dir = f.dst.repo
			cmd.Env = operationalTestEnvironmentWith(f.dst.home, map[string]string{"PATH": wrap + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP09_ACK_MODE": tc.name, "HN_WP09_ACK_GIT": realGit, "HN_WP09_ACK_REPO": f.dst.repo, "HN_WP09_ACK_REF": f.accepted, "HN_WP09_ACK_OLD": f.oldTip, "HN_WP09_ACK_COMPETING": competing, "HN_WP09_ACK_COMMITTED": committed, "HN_WP09_ACK_REFUSED": refused, "HN_WP09_ACK_DENIED": denied})
			raw, callErr := cmd.Output()
			var response map[string]any
			if e = json.Unmarshal(raw, &response); e != nil {
				t.Fatal("response", e, string(raw))
			}
			target := f.dst.git("rev-parse", f.accepted)
			afterRefs := f.dst.git("for-each-ref", "--format=%(refname) %(objectname) %(symref)", "refs/hn/remotes/public/artifacts/")
			sentinelAfter := namespacePublicBody(t, f.dst, f.sentinel)
			if f.dst.git("rev-parse", f.sentinel) != f.oldTip || !bytes.Equal(sentinelBefore, sentinelAfter) {
				t.Fatal("fetch fault changed unrelated public sentinel")
			}
			if tc.name == "stale-old" {
				if _, e = os.Stat(refused); e != nil {
					t.Fatal("control did not exercise real stale-old Git refusal", e)
				}
				if _, e = os.Stat(committed); !os.IsNotExist(e) || target != competing {
					t.Fatal("stale transaction published selected successor", target, e)
				}
			} else {
				if _, e = os.Stat(committed); e != nil || target != f.newTip {
					t.Fatal("control did not commit selected successor", target, e)
				}
				if !bytes.Equal(namespacePublicBody(t, f.dst, f.accepted), namespacePublicBody(t, f.src, f.newTip)) {
					t.Fatal("committed raw successor changed")
				}
			}
			t.Logf("mode=%s old_target=%s selected_new=%s observed_target=%s accepted_refs=%q public_sentinel=%s sentinel_body_before=%x sentinel_body_after=%x response=%s", tc.name, f.oldTip, f.newTip, target, afterRefs, f.oldTip, sentinelBefore, sentinelAfter, raw)
			f.src.unchanged(sourceBefore)
			for _, p := range []string{f.dst.home, filepath.Join(f.dst.repo, ".hn"), filepath.Join(f.dst.repo, ".git", "hn")} {
				if _, e = os.Stat(p); !os.IsNotExist(e) {
					t.Fatal("public fault created private state", p, e)
				}
			}
			if callErr == nil || response["ok"] != false || response["data"] != nil {
				t.Fatal("failed acknowledgement became success", callErr, response)
			}
			artifactErrorShape(t, response, "artifact", tc.code, tc.recovery, nil)
			if tc.name == "unavailable-witness" {
				if _, e = os.Stat(denied); e != nil {
					t.Fatal("control did not deny the actual post-error witness", e)
				}
			}
		})
	}
}
