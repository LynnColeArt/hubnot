package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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

type artifactBlobPeerConfig struct {
	Mode, Ledger, ActualGit, SessionRoot, Sessions string
	Target                                         int
}

// This supplemental peer observes the actual supplied candidate's filename
// protocol. Valid requests are stored by the real Git executable; only the
// explicitly selected acknowledgement/finalization fault is synthetic.
func TestArtifactBlobWriterPeer(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--" || i+1 >= len(os.Args) {
			continue
		}
		raw, err := os.ReadFile(os.Args[i+1])
		var cfg artifactBlobPeerConfig
		if err != nil || json.Unmarshal(raw, &cfg) != nil {
			os.Exit(71)
		}
		index := 1
		for ; index <= 8; index++ {
			err = os.Mkdir(filepath.Join(cfg.SessionRoot, strconv.Itoa(index)), 0700)
			if err == nil {
				break
			}
			if !os.IsExist(err) {
				os.Exit(72)
			}
		}
		if index > 8 {
			os.Exit(73)
		}
		log := func(path string, event map[string]any) {
			event["session"], event["pid"], event["ppid"] = index, os.Getpid(), os.Getppid()
			event["mode"] = cfg.Mode
			b, _ := json.Marshal(event)
			file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				os.Exit(74)
			}
			_, err = file.Write(append(b, '\n'))
			closeErr := file.Close()
			if err != nil || closeErr != nil {
				os.Exit(75)
			}
		}
		argv := os.Args[i+2:]
		log(cfg.Sessions, map[string]any{"event": "worker", "argv": argv, "target": index == cfg.Target, "actual_git": cfg.ActualGit})
		if index != cfg.Target {
			if syscall.Exec(cfg.ActualGit, append([]string{cfg.ActualGit}, argv...), os.Environ()) != nil {
				os.Exit(76)
			}
		}
		dir, command, paths, filters, blob := "", false, false, false, false
		for j, value := range argv {
			if strings.HasPrefix(value, "--git-dir=") {
				dir = strings.TrimPrefix(value, "--git-dir=")
			}
			command = command || value == "hash-object"
			paths = paths || value == "--stdin-paths"
			filters = filters || value == "--no-filters"
			blob = blob || value == "-t" && j+1 < len(argv) && argv[j+1] == "blob"
		}
		if dir == "" || !command || !paths || !filters || !blob || os.Getenv("GIT_NO_LAZY_FETCH") != "1" || os.Getenv("GIT_NO_REPLACE_OBJECTS") != "1" {
			os.Exit(77)
		}
		if cfg.Mode == "stderr" {
			_, _ = os.Stderr.Write(bytes.Repeat([]byte{'S'}, 8193))
			os.Exit(78)
		}
		if cfg.Mode == "stderr-at" || cfg.Mode == "stderr-overflow" {
			n := 8192
			if cfg.Mode == "stderr-overflow" {
				n++
			}
			log(cfg.Ledger, map[string]any{"event": "stderr-then-real-git", "bytes": n, "real_git": cfg.ActualGit})
			if _, err := os.Stderr.Write(bytes.Repeat([]byte{'S'}, n)); err != nil {
				os.Exit(78)
			}
			// Same PID, argv and environment now execute the genuine interactive
			// worker. No unrelated peer exit can make the overflow test pass.
			if syscall.Exec(cfg.ActualGit, append([]string{cfg.ActualGit}, argv...), os.Environ()) != nil {
				os.Exit(76)
			}
		}
		reader := bufio.NewReader(os.Stdin)
		faultSent := false
		afterFault := 0
		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF && line == "" {
				log(cfg.Ledger, map[string]any{"event": "stdin-eof"})
				if cfg.Mode == "stall-eof" {
					artifactIOStall(func(event map[string]any) { log(cfg.Ledger, event) }, "eof")
				}
				if cfg.Mode == "trailing" {
					fmt.Fprint(os.Stdout, "unexpected final output\n")
				}
				if cfg.Mode == "exit-error" {
					os.Exit(79)
				}
				os.Exit(0)
			}
			if err != nil || len(line) > 16384 {
				os.Exit(80)
			}
			path, err := strconv.Unquote(strings.TrimSuffix(line, "\n"))
			if err != nil || !filepath.IsAbs(path) {
				os.Exit(81)
			}
			if faultSent {
				// Do not spawn a real storage child after an intentionally broken
				// response. Record further feeding; the test must reject it except
				// the one request made before detecting a queued extra ACK line.
				log(cfg.Ledger, map[string]any{"event": "filename-after-fault-response", "raw": line, "path": path})
				afterFault++
				if afterFault > 8 {
					os.Exit(87)
				}
				continue
			}
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 33554432 {
				os.Exit(82)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				os.Exit(83)
			}
			cmd := exec.Command(cfg.ActualGit, "--git-dir="+dir, "hash-object", "-w", "--no-filters", "-t", "blob", "--", path)
			cmd.Env = os.Environ()
			ack, err := cmd.Output()
			if err != nil {
				os.Exit(84)
			}
			oid := strings.TrimSuffix(string(ack), "\n")
			framed := append([]byte(fmt.Sprintf("blob %d\x00", len(body))), body...)
			digest := sha1.Sum(framed)
			independent := hex.EncodeToString(digest[:])
			if len(oid) == 64 {
				digest := sha256.Sum256(framed)
				independent = hex.EncodeToString(digest[:])
			}
			if oid != independent {
				os.Exit(85)
			}
			log(cfg.Ledger, map[string]any{"event": "filename", "raw": line, "path": path, "mode": info.Mode().String(), "bytes": len(body), "body_sha256": wp25RecordCountHash(body), "independent_oid": independent, "real_git_ack": string(ack), "real_git_child_pid": cmd.Process.Pid, "real_git_wait_joined": true})
			if cfg.Mode == "stall-ack" {
				artifactIOStall(func(event map[string]any) { log(cfg.Ledger, event) }, "ack")
			}
			switch cfg.Mode {
			case "wrong-ack":
				ack = []byte(strings.Repeat("0", len(oid)) + "\n")
				faultSent = true
			case "short-ack":
				written, err := os.Stdout.Write([]byte(oid))
				log(cfg.Ledger, map[string]any{"event": "partial-ack", "raw": oid, "written_bytes": written, "write_error": fmt.Sprint(err)})
				os.Exit(0)
			case "extra-ack":
				ack = append(ack, []byte("unexpected acknowledgement\n")...)
				faultSent = true
			}
			log(cfg.Ledger, map[string]any{"event": "ack", "raw": string(ack), "expected_oid": oid})
			if _, err := os.Stdout.Write(ack); err != nil {
				os.Exit(86)
			}
		}
	}
}

func artifactBlobPeerInstall(t *testing.T, f *artifactCLIFixture, mode string, target int) (string, string, map[string]string) {
	t.Helper()
	actual, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cfg := artifactBlobPeerConfig{mode, filepath.Join(dir, "peer.jsonl"), actual, filepath.Join(dir, "state"), filepath.Join(dir, "sessions.jsonl"), target}
	if err := os.Mkdir(cfg.SessionRoot, 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config.json")
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = --stdin-paths ]; then exec " + quote(self) + " '-test.run=^TestArtifactBlobWriterPeer$' -- " + quote(config) + " \"$@\"; fi\ndone\nexec " + quote(actual) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	overrides := map[string]string{}
	for k, v := range f.overrides {
		overrides[k] = v
	}
	overrides["PATH"] = dir + string(os.PathListSeparator) + os.Getenv("PATH")
	version, err := exec.Command(actual, "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	wp25Record(t, map[string]any{"control": "blob-writer-peer-config", "source_assisted": true, "mode": mode, "target_worker": target, "config_raw": string(raw), "script_raw": script, "real_git": actual, "real_git_sha256": wp25Digest(t, actual), "real_git_version_raw": string(version), "compiled_peer": self})
	return cfg.Ledger, cfg.Sessions, overrides
}

func artifactBlobRequestCounts(t *testing.T, mode string, raw []byte) {
	t.Helper()
	counts := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			t.Fatal("invalid blob request-count ledger")
		}
		kind := event["event"].(string)
		counts[kind]++
		if kind == "partial-ack" && (event["written_bytes"] != float64(len(event["raw"].(string))) || event["write_error"] != "<nil>") {
			t.Fatal("short-ACK control did not actually write its partial frame", event)
		}
		if kind == "stall-entry" && (event["mode"] != mode || event["phase"] != strings.TrimPrefix(mode, "stall-")) {
			t.Fatal("wrong blob stall phase", event)
		}
		if kind == "stderr-then-real-git" {
			expected := float64(8192)
			if mode == "stderr-overflow" {
				expected++
			}
			if event["bytes"] != expected {
				t.Fatal("stderr cap control used wrong literal bytes", event)
			}
		}
	}
	if mode == "stderr-at" || mode == "stderr-overflow" {
		if counts["stderr-then-real-git"] != 1 || counts["filename"] != 0 || counts["ack"] != 0 || counts["stall-entry"] != 0 {
			t.Fatal("stderr-only control did not exec the actual Git worker", mode, counts)
		}
		wp25Record(t, map[string]any{"control": "private-stderr-bound-real-git-worker", "mode": mode, "counts": counts, "raw": string(raw), "scope": "private8192 safety ceiling; valid real Git continues at samePID, no unrelated exit fault"})
		return
	}
	wantFiles, wantACK, wantPartial, wantEOF, wantStalls := 2, 2, 0, -1, 0
	switch mode {
	case "pass", "trailing", "exit-error":
		wantEOF = 1
	case "wrong-ack":
		wantFiles, wantACK = 1, 1
	case "short-ack":
		wantFiles, wantACK, wantPartial = 1, 0, 1
	case "extra-ack":
		// The queued extra line is detected by the next ACK read. At most one
		// intervening filename can be sent before that first detected fault.
		wantFiles, wantACK = 1, 1
	case "stderr":
		wantFiles, wantACK = 0, 0
	case "stall-ack":
		wantFiles, wantACK, wantEOF, wantStalls = 1, 0, 0, 1
	case "stall-eof":
		wantEOF, wantStalls = 1, 1
	default:
		t.Fatal("unknown blob fault mode", mode)
	}
	if counts["filename"] != wantFiles || counts["ack"] != wantACK || counts["partial-ack"] != wantPartial || counts["stall-entry"] != wantStalls || wantEOF >= 0 && counts["stdin-eof"] != wantEOF {
		t.Fatal("blob writer reached a different phase or fed after ACK failure", mode, counts, wantFiles, wantACK, wantPartial, wantEOF, wantStalls)
	}
	if mode == "extra-ack" {
		if counts["filename-after-fault-response"] > 1 {
			t.Fatal("writer continued after detecting extra ACK", counts)
		}
	} else if counts["filename-after-fault-response"] != 0 {
		t.Fatal("writer sent another filename after its first detected ACK fault", mode, counts)
	}
	wp25Record(t, map[string]any{"control": "exact-blob-request-stop-and-stall-counts", "mode": mode, "counts": counts, "expected_filename_requests": wantFiles, "expected_ack_intents": wantACK, "expected_partial_ack_writes": wantPartial, "expected_stall_entries": wantStalls, "extra_ack_intervening_request_ceiling": 1, "raw": string(raw)})
}

func TestArtifactBlobWriterProtocolCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			src := artifactCLIFormat(t, format)
			ref := artifactFrozenRef(t, src)
			root := artifactRawCommit(t, src, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			src.git("update-ref", ref, root)
			path, _ := exchangeExport(src, artifactVectorID)
			for _, target := range []int{1, 2} {
				for _, mode := range []string{"pass", "stderr-at", "stderr-overflow", "wrong-ack", "short-ack", "extra-ack", "trailing", "exit-error", "stderr", "stall-ack", "stall-eof"} {
					t.Run(fmt.Sprintf("worker%d-%s", target, mode), func(t *testing.T) {
						src.t = t
						dst := exchangeDestination(t, src, format)
						// Exercise quoted byte paths without an alias or a shell path.
						scratchRoot := filepath.Join(t.TempDir(), "space quote\" backslash\\ newline\n雪")
						if err := os.Mkdir(scratchRoot, 0700); err != nil {
							t.Fatal(err)
						}
						dst.overrides = map[string]string{"TMPDIR": scratchRoot, "XDG_CONFIG_HOME": wp25FixtureConfig(t, dst.home)}
						ledger, sessions, overrides := artifactBlobPeerInstall(t, dst, mode, target)
						dst.overrides = overrides
						before := dst.snapshot()
						code := "verification_failure"
						message := "cannot copy verified exchange object"
						if mode == "pass" || mode == "stderr-at" {
							code = ""
						} else if strings.HasPrefix(mode, "stall-") {
							code, message = "cancelled", "artifact exchange cancelled"
						}
						started := time.Now()
						var response map[string]any
						if strings.HasPrefix(mode, "stall-") {
							response = artifactIOObservedCall(t, dst, "", message, artifactIOPhase{Mode: mode, Phase: strings.TrimPrefix(mode, "stall-"), Ledger: ledger, Sessions: sessions, Session: target}, "artifact", "import", "--input", path, "--json")
						} else {
							response = wp25ArtifactCall(dst, "", code, "artifact", "import", "--input", path, "--json")
						}
						elapsed := time.Since(started)
						if strings.HasPrefix(mode, "stall-") && (elapsed < 29*time.Second || elapsed > 45*time.Second) {
							t.Fatal("writer cancellation changed actual30s stage", elapsed)
						}
						if code == "" {
							if !reflect.DeepEqual(response["data"].(map[string]any)["imported_ids"], []any{artifactVectorID}) {
								t.Fatal("blob worker lost selected signed ID", response)
							}
							if !reflect.DeepEqual(closureItems(src, artifactVectorID), closureItems(dst, artifactVectorID)) {
								t.Fatal("blob ACK journey changed exact signed closure")
							}
						} else {
							artifactErrorShape(t, response, "artifact", code, "do_not_retry", nil)
							if response["data"] != nil || response["error"].(map[string]any)["message"] != message {
								t.Fatal("blob fault acquired success data or changed diagnostic", response)
							}
							after := dst.snapshot()
							objectRoot := filepath.Join(dst.repo, ".git", "objects") + string(os.PathSeparator)
							for name, value := range before {
								if after[name] != value {
									t.Fatal("failed copy changed/deleted a preexisting byte or mode", name)
								}
							}
							for name := range after {
								if _, exists := before[name]; !exists && !strings.HasPrefix(name, objectRoot) {
									t.Fatal("failed copy published refs/private/authority state", name)
								}
							}
							wp25Record(t, map[string]any{"control": "blob-writer-refusal-state", "protected_before": before, "protected_after": after, "allowed_residue": "hash-verified unreferenced public ODB objects only; never removed existing/deduplicated objects"})
						}
						raw, err := os.ReadFile(sessions)
						workers := target
						if mode == "pass" || mode == "stderr-at" {
							workers = 2
						}
						if err != nil || bytes.Count(raw, []byte{'\n'}) != workers {
							t.Fatal("missing or extra blob worker phase", err, string(raw))
						}
						peer, err := os.ReadFile(ledger)
						if err != nil && mode != "stderr" {
							t.Fatal("blob peer never observed request/finalization", err)
						}
						artifactBlobRequestCounts(t, mode, peer)
						for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
							var event map[string]any
							if json.Unmarshal(line, &event) != nil {
								t.Fatal("invalid worker ledger")
							}
							pid := strconv.Itoa(int(event["pid"].(float64)))
							out, err := exec.Command("ps", "-p", pid, "-o", "pid=,ppid=,comm=").CombinedOutput()
							if err == nil || len(bytes.TrimSpace(out)) != 0 {
								t.Fatal("blob worker survived public return", pid, string(out))
							}
						}
						for _, line := range bytes.Split(bytes.TrimSpace(peer), []byte{'\n'}) {
							if len(line) == 0 {
								continue
							}
							var event map[string]any
							if json.Unmarshal(line, &event) != nil {
								t.Fatal("invalid blob request ledger")
							}
							if event["event"] == "filename" {
								if _, err := os.Lstat(event["path"].(string)); !os.IsNotExist(err) {
									t.Fatal("owned scratch survived public return", event, err)
								}
								pid := strconv.Itoa(int(event["real_git_child_pid"].(float64)))
								out, err := exec.Command("ps", "-p", pid, "-o", "pid=,ppid=,comm=").CombinedOutput()
								if err == nil || len(bytes.TrimSpace(out)) != 0 {
									t.Fatal("peer's real Git storage child survived Wait", pid, string(out))
								}
							}
						}
						entries, err := os.ReadDir(scratchRoot)
						if err != nil || len(entries) != 0 {
							t.Fatal("scratch/quarantine survived copy completion/refusal", err, entries)
						}
						wp25Record(t, map[string]any{"control": "blob-writer-peer-ledgers", "source_assisted": true, "format": format, "mode": mode, "target_worker": target, "sessions_raw": string(raw), "peer_raw": string(peer), "elapsed_nanoseconds": elapsed.Nanoseconds(), "unchanged_product_stage_seconds": 30})
					})
				}
			}
		})
	}
}

func TestArtifactBlobWriterUnreachablePublicCLI(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			src := artifactCLIFormat(t, format)
			ref := artifactFrozenRef(t, src)
			root := artifactRawCommit(t, src, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
			src.git("update-ref", ref, root)
			path, _ := exchangeExport(src, artifactVectorID)
			wire, err := os.ReadFile(path)
			var table map[string]any
			if err != nil || json.Unmarshal(wire, &table) != nil {
				t.Fatal(err)
			}
			//9MiB is above selected-object8MiB but its base64 still fits the
			// independent16MiB wire cap.32MiB scratch is an enclosing private
			// accounting ceiling, never a single public-body equality promise.
			body := bytes.Repeat([]byte{'U'}, 9<<20)
			framed := append([]byte(fmt.Sprintf("blob %d\x00", len(body))), body...)
			hash := sha1.Sum(framed)
			oid := hex.EncodeToString(hash[:])
			if format == "sha256" {
				hash := sha256.Sum256(framed)
				oid = hex.EncodeToString(hash[:])
			}
			table["objects"] = append(table["objects"].([]any), map[string]any{"oid": oid, "type": "blob", "data": base64.RawStdEncoding.EncodeToString(body)})
			wire, err = json.Marshal(table)
			if err != nil || len(wire) >= 16777216 {
				t.Fatal("public unreachable blob must fit unchanged wire", err, len(wire))
			}
			input := exchangeWrite(t, table)
			retained := ""
			if evidence := os.Getenv("HN_WP25_EVIDENCE"); evidence != "" {
				retained = filepath.Join(evidence, "source-assistance", "blob-unreachable", format+"-public.json")
				if err := os.MkdirAll(filepath.Dir(retained), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(retained, wire, 0600); err != nil {
					t.Fatal(err)
				}
			}
			dst := exchangeDestination(t, src, format)
			result := wp25ArtifactCall(dst, "", "", "artifact", "import", "--input", input, "--json")
			data := result["data"].(map[string]any)
			if !reflect.DeepEqual(data["imported_ids"], []any{artifactVectorID}) || len(data["opaque_ids"].([]any)) != 0 || !reflect.DeepEqual(closureItems(src, artifactVectorID), closureItems(dst, artifactVectorID)) {
				t.Fatal("unreachable public blob changed selected signed history", result)
			}
			cmd := exec.Command("git", "-C", dst.repo, "cat-file", "-e", oid)
			cmd.Env = operationalTestEnvironmentWith(dst.home, map[string]string{"GIT_NO_LAZY_FETCH": "1"})
			output, absent := cmd.CombinedOutput()
			if absent == nil {
				t.Fatal("unreachable public blob entered accepted destination ODB")
			}
			wp25Record(t, map[string]any{"control": "blob-writer-unreachable-public-over-selected-cap", "source_assisted_construction": true, "candidate_observation": true, "format": format, "unreachable_oid": oid, "body_bytes": len(body), "body_sha256": wp25RecordCountHash(body), "wire_bytes": len(wire), "wire_sha256": wp25RecordCountHash(wire), "retained_public_input": retained, "selected_id": artifactVectorID, "destination_absence_command": cmd.Args, "destination_absence_output_raw": string(output), "destination_absence_error": fmt.Sprint(absent), "scope": "actual public9MiB unreachable body fits16MiB wire; not public32MiB single-body admission"})
			// A malformed LAST table entry must be found before ANY blob/tree/
			// commit copy. The valid-file detector control below must dispatch.
			actual, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			wrapper := t.TempDir()
			marker := filepath.Join(wrapper, "copy-dispatched")
			quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
			script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = hash-object ]; then printf '%s\\n' \"$@\" > " + quote(marker) + "; exit 86; fi\ndone\nexec " + quote(actual) + " \"$@\"\n"
			if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			bad := bytes.Clone(body)
			bad[0] ^= 1
			entries := table["objects"].([]any)
			entries[len(entries)-1].(map[string]any)["data"] = base64.RawStdEncoding.EncodeToString(bad)
			broken := exchangeWrite(t, table)
			if retained != "" {
				if err := os.WriteFile(strings.TrimSuffix(retained, ".json")+"-invalid-last.json", wp25Read(t, broken), 0600); err != nil {
					t.Fatal(err)
				}
			}
			negative := exchangeDestination(t, src, format)
			negative.overrides = map[string]string{"PATH": wrapper + string(os.PathListSeparator) + os.Getenv("PATH")}
			before := negative.snapshot()
			response := wp25ArtifactCall(negative, "", "verification_failure", "artifact", "import", "--input", broken, "--json")
			artifactErrorShape(t, response, "artifact", "verification_failure", "do_not_retry", nil)
			if response["data"] != nil || response["error"].(map[string]any)["message"] != "exchange Git object hash mismatch" {
				t.Fatal("later invalid table entry changed its exact refusal", response)
			}
			negative.unchanged(before)
			if raw, err := os.ReadFile(marker); !os.IsNotExist(err) {
				t.Fatal("later invalid table entry dispatched copy", err, string(raw))
			}
			positive := exchangeDestination(t, src, format)
			positive.overrides = negative.overrides
			before = positive.snapshot()
			response = wp25ArtifactCall(positive, "", "verification_failure", "artifact", "import", "--input", input, "--json")
			artifactErrorShape(t, response, "artifact", "verification_failure", "do_not_retry", nil)
			if response["data"] != nil || response["error"].(map[string]any)["message"] != "cannot copy verified exchange object" {
				t.Fatal("copy-dispatch detector did not refuse at its actual stage", response)
			}
			positive.unchanged(before)
			observed, err := os.ReadFile(marker)
			if err != nil || !bytes.Contains(observed, []byte("hash-object")) {
				t.Fatal("copy detector was vacuous", err, string(observed))
			}
			wp25Record(t, map[string]any{"control": "whole-public-table-validation-before-any-copy", "source_assisted_detector": true, "format": format, "bad_last_oid": oid, "bad_body_sha256": wp25RecordCountHash(bad), "script_raw": script, "positive_copy_marker_raw": string(observed), "negative_state_unchanged": true, "positive_detector_state_unchanged": true, "all_copy_kinds_observed": "any hash-object argv, not only blob worker"})
		})
	}
}
