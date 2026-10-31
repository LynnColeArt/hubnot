package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestArtifactGitIndependentClonesCLI(t *testing.T) {
	src, dst := artifactCLI(t), artifactCLI(t)
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
	for _, path := range []string{filepath.Join(dst.repo, ".git", "hn", "artifacts", "intents", src.actor), filepath.Join(dst.home, ".config", "hubnot", "identity")} {
		if _, err := os.Stat(path); err == nil {
			t.Fatal("source private state copied", path)
		}
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
