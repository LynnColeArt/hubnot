package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// The module is a separately executed consumer. Its handwritten framing does
// not import extension transport or dispatch helpers.
func TestExtensionNativeChild(t *testing.T) {
	if os.Getenv("HN_WP17_CHILD") != "1" {
		return
	}
	read := func() map[string]any {
		var prefix [4]byte
		if _, e := io.ReadFull(os.Stdin, prefix[:]); e != nil {
			os.Exit(80)
		}
		n := binary.BigEndian.Uint32(prefix[:])
		if n == 0 || n > 8388608 {
			os.Exit(81)
		}
		raw := make([]byte, n)
		if _, e := io.ReadFull(os.Stdin, raw); e != nil {
			os.Exit(82)
		}
		var v map[string]any
		if json.Unmarshal(raw, &v) != nil {
			os.Exit(83)
		}
		return v
	}
	write := func(v any) {
		raw, _ := json.Marshal(v)
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
		os.Stdout.Write(prefix[:])
		os.Stdout.Write(raw)
	}
	read()
	write(map[string]any{"protocol": "hn.extension/1", "ready": true})
	raw, e := os.ReadFile(os.Getenv("HN_WP17_REQUESTS"))
	if e != nil {
		os.Exit(84)
	}
	var requests []map[string]any
	if json.Unmarshal(raw, &requests) != nil {
		os.Exit(85)
	}
	responses := []any{}
	for _, r := range requests {
		write(r)
		if gate := os.Getenv("HN_WP17_CANCEL_GATE"); gate != "" {
			until := time.Now().Add(8 * time.Second)
			for {
				if _, e := os.Stat(gate); e == nil {
					break
				}
				if time.Now().After(until) {
					os.Exit(87)
				}
				time.Sleep(5 * time.Millisecond)
			}
			os.WriteFile(os.Getenv("HN_WP17_CANCEL_SENT"), []byte("cancel queued"), 0600)
			write(map[string]any{"protocol": "hn.extension/1", "cancel": r["request_id"]})
		}
		responses = append(responses, read())
	}
	raw, _ = json.Marshal(responses)
	if os.WriteFile(os.Getenv("HN_WP17_RESPONSES"), raw, 0600) != nil {
		os.Exit(86)
	}
	os.Exit(0)
}

type nativeHostFixture struct {
	t                                 *testing.T
	binary, repo, home, input, output string
	env                               []string
}

func newNativeHostFixture(t *testing.T, grants ...string) *nativeHostFixture {
	t.Helper()
	bin := os.Getenv("HN_WP17_BINARY")
	if bin == "" {
		bin = extensionTestBinary(t)
	}
	f := &nativeHostFixture{t: t, binary: bin, repo: t.TempDir(), home: t.TempDir()}
	f.input = filepath.Join(f.home, "requests.json")
	f.output = filepath.Join(f.home, "responses.json")
	f.env = operationalTestEnvironmentWith(f.home, map[string]string{"XDG_CONFIG_HOME": filepath.Join(f.home, "config"), "HN_WP17_CHILD": "1", "HN_WP17_REQUESTS": f.input, "HN_WP17_RESPONSES": f.output})
	g := exec.Command("git", "init", "-q", f.repo)
	g.Env = f.env
	if raw, e := g.CombinedOutput(); e != nil {
		t.Fatalf("git init: %v %s", e, raw)
	}
	manifest := filepath.Join(f.home, "manifest.json")
	os.WriteFile(manifest, []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.native/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","resource.read","resource.mutate","resource.operation"]}`), 0600)
	child, _ := os.Executable()
	f.command("extension", "register", "--manifest", manifest, "--executable", child, "--arg", "-test.run=^TestExtensionNativeChild$", "--json")
	args := []string{"extension", "enable", "example.native/tool"}
	for _, g := range grants {
		args = append(args, "--grant", g)
	}
	f.command(append(args, "--json")...)
	return f
}
func (f *nativeHostFixture) command(args ...string) map[string]any {
	f.t.Helper()
	c := exec.Command(f.binary, args...)
	c.Env = f.env
	c.Dir = f.repo
	var out, errout bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errout
	e := c.Run()
	var v map[string]any
	if json.Unmarshal(out.Bytes(), &v) != nil || e != nil {
		f.t.Fatalf("%v: %v %s %s", args, e, &out, &errout)
	}
	return v
}
func (f *nativeHostFixture) requests(requests ...map[string]any) []map[string]any {
	f.t.Helper()
	raw, _ := json.Marshal(requests)
	if e := os.WriteFile(f.input, raw, 0600); e != nil {
		f.t.Fatal(e)
	}
	f.command("extension", "run", "example.native/tool", "--json")
	raw, e := os.ReadFile(f.output)
	if e != nil {
		f.t.Fatal(e)
	}
	var r []map[string]any
	if e = json.Unmarshal(raw, &r); e != nil {
		f.t.Fatal(e)
	}
	if evidence := os.Getenv("HN_WP17_EVIDENCE"); evidence != "" {
		file, e := os.CreateTemp(evidence, "raw-stdio-*.json")
		if e != nil {
			f.t.Fatal(e)
		}
		raw, _ := json.Marshal(map[string]any{"test": f.t.Name(), "binary": f.binary, "requests": requests, "responses": r})
		if _, e = file.Write(raw); e != nil {
			f.t.Fatal(e)
		}
		file.Close()
	}
	return r
}
func nativeRequest(method string, p map[string]any) map[string]any {
	return map[string]any{"protocol": "hn.extension/1", "request_id": fmt.Sprintf("%s", method), "method": method, "params": p}
}
func nativeOK(t *testing.T, r map[string]any) map[string]any {
	t.Helper()
	if r["ok"] != true {
		t.Fatalf("expected success: %v", r)
	}
	return r["data"].(map[string]any)
}
func TestExtensionNativeIdentityMissingCLI(t *testing.T) {
	f := newNativeHostFixture(t, "native:identity=read")
	before := hostNativeSnapshot(t, f.repo)
	got := nativeOK(t, f.requests(nativeRequest("identity.public", map[string]any{}))[0])
	if len(got) != 4 || got["initialized"] != false || got["actor"] != nil || got["name"] != nil || got["public_key"] != nil {
		t.Fatalf("missing identity: %v", got)
	}
	if !bytes.Equal([]byte(fmt.Sprint(before)), []byte(fmt.Sprint(hostNativeSnapshot(t, f.repo)))) {
		t.Fatal("public read changed native state")
	}
}

func (f *nativeHostFixture) native(args ...string) string {
	f.t.Helper()
	c := exec.Command(f.binary, args...)
	c.Env = f.env
	c.Dir = f.repo
	raw, e := c.CombinedOutput()
	if e != nil {
		f.t.Fatalf("native %v: %v %s", args, e, raw)
	}
	return string(raw)
}
func (f *nativeHostFixture) actor() string {
	f.t.Helper()
	f.native("init", "--name", "Native host author")
	var v map[string]any
	raw := f.native("identity", "show", "--json")
	if json.Unmarshal([]byte(raw), &v) != nil {
		f.t.Fatal(raw)
	}
	return v["data"].(map[string]any)["actor"].(string)
}
func nativeMutation(family, actor, key string, state any) map[string]any {
	return nativeRequest("resource.mutate", map[string]any{"family": family, "intent": "open", "id": nil, "expected": []any{}, "actor": actor, "operation": key, "state": state, "comment": ""})
}
func nativeLookup(family, actor, key string) map[string]any {
	return nativeRequest("resource.operation", map[string]any{"family": family, "actor": actor, "operation": key})
}
func nativeReadRequest(family string, id any) map[string]any {
	return nativeRequest("resource.read", map[string]any{"family": family, "id": id, "limit": 200, "cursor": nil})
}
func nativeIssueState(title string) map[string]any {
	return map[string]any{"title": title, "body": "native body", "status": "open", "criteria": []any{}, "labels": []any{}, "assignees": []any{}, "relations": []any{}, "metadata": map[string]any{}}
}
func nativeDenial(t *testing.T, r map[string]any, origin, code, recovery string) {
	t.Helper()
	e, ok := r["error"].(map[string]any)
	if r["ok"] != false || r["data"] != nil || !ok || e["origin"] != origin || e["code"] != code || e["recovery"] != recovery {
		t.Fatalf("want %s/%s/%s: %v", origin, code, recovery, r)
	}
}
func nativeSnapshotEqual(t *testing.T, f *nativeHostFixture, before map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("refused/read request changed native objects, refs or private state")
	}
}
func TestExtensionNativeSixFamiliesCLI(t *testing.T) {
	families := []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"}
	grants := []string{"native:identity=read"}
	for _, family := range families {
		grants = append(grants, "native:"+family+"=read,create,revise,resolve")
	}
	f := newNativeHostFixture(t, grants...)
	actor := f.actor()
	ids := map[string]string{}
	for _, family := range families {
		t.Run(family, func(t *testing.T) {
			local := *f
			local.t = t
			f := &local
			var state any
			switch family {
			case "issue":
				state = nativeIssueState("Native issue")
			case "project":
				state = map[string]any{"schema": "hn.project/1", "title": "Native project", "body": "", "archived": false}
			case "project-item":
				state = map[string]any{"schema": "hn.project-item/1", "project_id": ids["project"], "project_revision": ids["project"], "target": map[string]any{"kind": "draft", "title": "Draft item", "body": ""}, "archived": false, "position": "0"}
			case "discussion":
				state = map[string]any{"schema": "hn.discussion/1", "title": "Native discussion", "body": "", "category": "general", "status": "open"}
			case "discussion-reply":
				state = map[string]any{"schema": "hn.discussion-reply/1", "discussion_id": ids["discussion"], "body": "Native reply"}
			case "workflow":
				state = map[string]any{"schema": "hn.workflow/1", "name": "native-fixture", "title": "Native workflow", "description": "stored only", "enabled": true, "pipeline": map[string]any{"version": "hn.pipeline/0", "steps": []any{map[string]any{"name": "never", "command": "touch " + filepath.Join(f.repo, "MUST-NOT-EXECUTE")}}}}
			}
			request := nativeMutation(family, actor, "open-"+family, state)
			opened := nativeOK(t, f.requests(request)[0])
			id := opened["resource_id"].(string)
			ids[family] = id
			if opened["event_id"] != id || opened["admission"] != nil || opened["head_count"] != float64(1) || opened["replayed"] != false || opened["protocol"] != "hn/0" || opened["family"] != family || opened["type_id"] != nil {
				t.Fatalf("opening normalization %v", opened)
			}
			before := hostNativeSnapshot(t, f.repo)
			recovered := nativeOK(t, f.requests(nativeLookup(family, actor, "open-"+family))[0])
			if recovered["status"] != "found" || recovered["event_id"] != id || recovered["resource_id"] != id || recovered["reason"] != nil {
				t.Fatalf("lookup %v", recovered)
			}
			replay := nativeOK(t, f.requests(request)[0])
			if replay["event_id"] != id || replay["replayed"] != true {
				t.Fatalf("immutable replay %v", replay)
			}
			view := nativeOK(t, f.requests(nativeReadRequest(family, id))[0])
			items := view["items"].([]any)
			if len(items) != 1 {
				t.Fatalf("read %v", view)
			}
			v := items[0].(map[string]any)
			if len(v) != 8 || v["protocol"] != "hn/0" || v["id"] != id || v["family"] != family || v["creator"] != actor || len(v["comments"].([]any)) != 0 || len(v["heads"].([]any)) != 1 {
				t.Fatalf("complete view %v", v)
			}
			if !reflect.DeepEqual(v["state"], state) {
				t.Fatalf("native state changed: %v != %v", v["state"], state)
			}
			nativeSnapshotEqual(t, f, before)
			request["params"].(map[string]any)["state"].(map[string]any)["metadata"] = map[string]any{"example/changed": "content"}
			nativeDenial(t, f.requests(request)[0], "native", "operation_conflict", "do_not_retry")
			nativeSnapshotEqual(t, f, before)
			bad := nativeMutation(family, strings.Repeat("a", 64), "wrong-actor-"+family, state)
			nativeDenial(t, f.requests(bad)[0], "native", "actor_mismatch", "do_not_retry")
			nativeSnapshotEqual(t, f, before)
			absent := nativeOK(t, f.requests(nativeLookup(family, actor, "absent-"+family))[0])
			if absent["status"] != "verified_absent" || absent["event_id"] != nil || absent["request"] != nil || absent["result"] != nil || absent["reason"] != nil {
				t.Fatalf("absence %v", absent)
			}
			changed := nativeMutation(family, actor, "revise-"+family, state)
			p := changed["params"].(map[string]any)
			p["id"], p["intent"], p["expected"] = id, "revise", []any{eventID([]byte("stale"))}
			nativeDenial(t, f.requests(changed)[0], "native", "stale_revision", "do_not_retry")
			nativeSnapshotEqual(t, f, before)
			p["expected"] = []any{id}
			revised := nativeOK(t, f.requests(changed)[0])
			if revised["resource_id"] != id || revised["event_id"] == id {
				t.Fatalf("revision %v", revised)
			}
			// Exact retry must precede stale current-head validation.
			again := nativeOK(t, f.requests(changed)[0])
			if again["event_id"] != revised["event_id"] || again["replayed"] != true {
				t.Fatalf("revision replay %v", again)
			}
			lookup := nativeOK(t, f.requests(nativeLookup(family, actor, "revise-"+family))[0])
			if lookup["event_id"] != revised["event_id"] {
				t.Fatalf("revision recovery %v", lookup)
			}
		})
	}
	if _, e := os.Stat(filepath.Join(f.repo, "MUST-NOT-EXECUTE")); !os.IsNotExist(e) {
		t.Fatal("workflow definition executed")
	}
	// The key namespace spans every native family and the lookup stays read-only
	// for the original actor after the active signer changes.
	nativeDenial(t, f.requests(nativeLookup("project", actor, "open-issue"))[0], "native", "operation_conflict", "do_not_retry")
	comment := nativeMutation("issue", actor, "comment", nil)
	p := comment["params"].(map[string]any)
	p["intent"], p["id"], p["comment"] = "comment", ids["issue"], "Retained comment"
	c := nativeOK(t, f.requests(comment)[0])
	again := nativeOK(t, f.requests(comment)[0])
	if c["event_id"] != again["event_id"] {
		t.Fatal("comment replay changed")
	}
	found := nativeOK(t, f.requests(nativeLookup("issue", actor, "comment"))[0])
	if found["event_id"] != c["event_id"] {
		t.Fatal("comment acknowledgement not recovered")
	}
	secondComment := nativeMutation("issue", actor, "comment-two", nil)
	sp := secondComment["params"].(map[string]any)
	sp["intent"], sp["id"], sp["comment"] = "comment", ids["issue"], "Second comment"
	nativeOK(t, f.requests(secondComment)[0])
	comments := nativeOK(t, f.requests(nativeReadRequest("issue", ids["issue"]))[0])["items"].([]any)[0].(map[string]any)["comments"].([]any)
	if len(comments) != 2 || comments[0].(map[string]any)["id"].(string) >= comments[1].(map[string]any)["id"].(string) {
		t.Fatalf("normalized comments order %v", comments)
	}
	f.native("identity", "rotate", "--name", "Rotated host author")
	before := hostNativeSnapshot(t, f.repo)
	found = nativeOK(t, f.requests(nativeLookup("issue", actor, "comment"))[0])
	if found["event_id"] != c["event_id"] {
		t.Fatal("lookup required active original signer")
	}
	nativeSnapshotEqual(t, f, before)
}
func TestExtensionNativeGrantDenialsCLI(t *testing.T) {
	for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"} {
		t.Run(family, func(t *testing.T) {
			f := newNativeHostFixture(t, "native:"+family+"=read")
			actor := f.actor()
			before := hostNativeSnapshot(t, f.repo)
			// Missing create must be checked before state semantics/dependencies/signer.
			state := map[string]any{}
			switch family {
			case "issue":
				state = nativeIssueState("Denied")
			case "project":
				state = map[string]any{"schema": "hn.project/1", "title": "Denied", "body": "", "archived": false}
			case "project-item":
				state = map[string]any{"schema": "hn.project-item/1", "project_id": eventID([]byte("missing")), "project_revision": eventID([]byte("missing")), "target": map[string]any{"kind": "draft", "title": "Denied", "body": ""}, "archived": false, "position": "0"}
			case "discussion":
				state = map[string]any{"schema": "hn.discussion/1", "title": "Denied", "body": "", "category": "general", "status": "open"}
			case "discussion-reply":
				state = map[string]any{"schema": "hn.discussion-reply/1", "discussion_id": eventID([]byte("missing")), "body": "Denied"}
			case "workflow":
				state = map[string]any{"schema": "hn.workflow/1", "name": "denied", "title": "Denied", "description": "", "enabled": false}
			}
			nativeDenial(t, f.requests(nativeMutation(family, actor, "denied", state))[0], "host", "denied_access", "do_not_retry")
			nativeSnapshotEqual(t, f, before)
		})
	}
}

func (f *nativeHostFixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.repo
	cmd.Env = f.env
	raw, e := cmd.CombinedOutput()
	if e != nil {
		f.t.Fatalf("git %v: %v %s", args, e, raw)
	}
	return strings.TrimSpace(string(raw))
}
func (f *nativeHostFixture) policy(actor string, rules []c.Rule) {
	f.t.Helper()
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: eventID([]byte("native-host-policy")), Moderators: []string{actor}, Rules: rules, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policy.Contributions.Limits.WireBytes = 128 << 10
	policy.Contributions.Limits.DecodedBytes = 1 << 20
	policy.Contributions.Limits.Objects = 1024
	policy.Contributions.Limits.Events = 256
	raw, _ := json.Marshal(policy)
	if e := os.MkdirAll(filepath.Join(f.repo, ".hn"), 0700); e != nil {
		f.t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(f.repo, ".hn", "policy.json"), raw, 0600); e != nil {
		f.t.Fatal(e)
	}
	f.git("add", ".hn/policy.json")
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Selected contribution policy")
	ref := f.git("symbolic-ref", "HEAD")
	f.native("contribution", "authority", "set", "--ref", ref, "--expect", "absent")
}
func TestExtensionNativeGovernedSixFamiliesCLI(t *testing.T) {
	families := []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"}
	grants := []string{}
	for _, family := range families {
		grants = append(grants, "native:"+family+"=read,create,revise,resolve")
	}
	f := newNativeHostFixture(t, grants...)
	actor := f.actor()
	rules := []c.Rule{}
	for _, op := range []c.Operation{c.IssueCreate, c.IssueReply, c.IssueEdit, c.ProjectCreate, c.ProjectEdit, c.ProjectItemCreate, c.ProjectItemEdit, c.DiscussionCreate, c.DiscussionReply, c.DiscussionEdit, c.WorkflowCreate, c.WorkflowEdit} {
		rules = append(rules, c.Rule{Operation: op, Automatic: true})
	}
	f.policy(actor, rules)
	ids := map[string]string{}
	for _, family := range families {
		var state any
		switch family {
		case "issue":
			state = nativeIssueState("Governed issue")
		case "project":
			state = map[string]any{"schema": "hn.project/1", "title": "Governed project", "body": "", "archived": false}
		case "project-item":
			state = map[string]any{"schema": "hn.project-item/1", "project_id": ids["project"], "project_revision": ids["project"], "target": map[string]any{"kind": "draft", "title": "Governed draft", "body": ""}, "archived": false, "position": "0"}
		case "discussion":
			state = map[string]any{"schema": "hn.discussion/1", "title": "Governed discussion", "body": "", "category": "general", "status": "open"}
		case "discussion-reply":
			state = map[string]any{"schema": "hn.discussion-reply/1", "discussion_id": ids["discussion"], "body": "Governed reply"}
		case "workflow":
			state = map[string]any{"schema": "hn.workflow/1", "name": "governed-definition", "title": "Governed workflow", "description": "inert", "enabled": false}
		}
		t.Logf("governed family %s", family)
		opened := nativeOK(t, f.requests(nativeMutation(family, actor, "governed-"+family, state))[0])
		if opened["admission"] != "admitted" || opened["head_count"] != float64(1) {
			t.Fatalf("governed %s: %v", family, opened)
		}
		ids[family] = opened["event_id"].(string)
	}
	for _, family := range families {
		t.Run(family, func(t *testing.T) {
			local := *f
			local.t = t
			f := &local
			id := ids[family]
			before := hostNativeSnapshot(t, f.repo)
			visible := nativeOK(t, f.requests(nativeReadRequest(family, id))[0])
			nativeSnapshotEqual(t, f, before)
			var hidden map[string]any
			if json.Unmarshal([]byte(f.native("contribution", "decide", "hide", "--revision", id, "--expect-heads", "absent", "--json")), &hidden) != nil {
				t.Fatal("hide JSON")
			}
			before = hostNativeSnapshot(t, f.repo)
			page := nativeOK(t, f.requests(nativeReadRequest(family, nil))[0])
			if len(page["items"].([]any)) != 0 || page["source_cut"] == visible["source_cut"] {
				t.Fatalf("hidden selection/cut %v", page)
			}
			nativeDenial(t, f.requests(nativeReadRequest(family, id))[0], "native", "not_found", "do_not_retry")
			lookup := nativeOK(t, f.requests(nativeLookup(family, actor, "governed-"+family))[0])
			if lookup["status"] != "uncertain" || lookup["reason"] != "visibility_restricted" || lookup["event_id"] != nil || lookup["resource_id"] != nil || lookup["request"] != nil || lookup["result"] != nil {
				t.Fatalf("hidden ID leak %v", lookup)
			}
			nativeSnapshotEqual(t, f, before)
			f.native("contribution", "decide", "restore", "--revision", id, "--expect-heads", hidden["id"].(string), "--json")
			restored := nativeOK(t, f.requests(nativeReadRequest(family, id))[0])
			if len(restored["items"].([]any)) != 1 || restored["source_cut"] == page["source_cut"] {
				t.Fatalf("restore %v", restored)
			}
		})
	}
}
func TestExtensionNativePendingAndPolicyRefusalCLI(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		f := newNativeHostFixture(t, "native:issue=read,create")
		actor := f.actor()
		f.policy(actor, []c.Rule{{Operation: c.IssueCreate}})
		opened := nativeOK(t, f.requests(nativeMutation("issue", actor, "pending-issue", nativeIssueState("Pending")))[0])
		if opened["admission"] != "pending" || opened["head_count"] != float64(0) {
			t.Fatalf("pending normalization %v", opened)
		}
		page := nativeOK(t, f.requests(nativeReadRequest("issue", nil))[0])
		if len(page["items"].([]any)) != 0 {
			t.Fatal("pending resource visible")
		}
		result := nativeOK(t, f.requests(nativeLookup("issue", actor, "pending-issue"))[0])
		if result["status"] != "uncertain" || result["reason"] != "visibility_restricted" || result["event_id"] != nil {
			t.Fatalf("pending lookup %v", result)
		}
	})
	t.Run("refused", func(t *testing.T) {
		f := newNativeHostFixture(t, "native:issue=read,create")
		actor := f.actor()
		f.policy(actor, []c.Rule{})
		before := hostNativeSnapshot(t, f.repo)
		nativeDenial(t, f.requests(nativeMutation("issue", actor, "refused-policy", nativeIssueState("Forbidden")))[0], "native", "permission_denied", "do_not_retry")
		nativeSnapshotEqual(t, f, before)
	})
}
func TestExtensionNativeSelectedCutAndPagingCLI(t *testing.T) {
	f := newNativeHostFixture(t, "native:issue=read,create", "native:project=read,create")
	actor := f.actor()
	one := nativeOK(t, f.requests(nativeMutation("issue", actor, "one", nativeIssueState("One")))[0])
	f.requests(nativeMutation("issue", actor, "two", nativeIssueState("Two")))
	request := nativeReadRequest("issue", nil)
	request["params"].(map[string]any)["limit"] = 1
	page := nativeOK(t, f.requests(request)[0])
	if len(page["items"].([]any)) != 1 || page["next_cursor"] == nil {
		t.Fatalf("first page %v", page)
	}
	request["params"].(map[string]any)["cursor"] = page["next_cursor"]
	last := nativeOK(t, f.requests(request)[0])
	if last["next_cursor"] != nil || last["source_cut"] != page["source_cut"] || last["items"].([]any)[0].(map[string]any)["id"] == page["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatalf("second page %v", last)
	}
	nativeOK(t, f.requests(nativeMutation("project", actor, "cross-family", map[string]any{"schema": "hn.project/1", "title": "Other family", "body": "", "archived": false}))[0])
	nativeDenial(t, f.requests(request)[0], "native", "stale_cursor", "rescan_required")
	current := nativeOK(t, f.requests(nativeReadRequest("issue", nil))[0])
	if current["source_cut"] == one["source_cut"] || current["source_cut"] == page["source_cut"] || len(current["items"].([]any)) != 2 {
		t.Fatalf("shared native witness invalidation %v", current)
	}
	nativeDenial(t, f.requests(nativeLookup("issue", actor, "cross-family"))[0], "native", "operation_conflict", "do_not_retry")
	before := hostNativeSnapshot(t, f.repo)
	bad := nativeMutation("issue", actor, "cross-family", nativeIssueState("Other content"))
	nativeDenial(t, f.requests(bad)[0], "native", "operation_conflict", "do_not_retry")
	nativeSnapshotEqual(t, f, before)
}
func TestExtensionNativeInvalidHistoryCLI(t *testing.T) {
	f := newNativeHostFixture(t, "native:issue=read")
	actor := f.actor()
	blob := f.git("hash-object", "-w", "--stdin")
	f.git("update-ref", "refs/hn/actors/"+actor, blob)
	control := exec.Command(f.binary, "issue", "list", "--json")
	control.Dir = f.repo
	control.Env = f.env
	output, controlErr := control.CombinedOutput()
	t.Logf("ordinary native blob-head control: exit=%v output=%s", controlErr, output)
	before := hostNativeSnapshot(t, f.repo)
	nativeDenial(t, f.requests(nativeReadRequest("issue", nil))[0], "native", "invalid_history", "do_not_retry")
	nativeSnapshotEqual(t, f, before)
}

func TestExtensionNativeCancellationSettlementCLI(t *testing.T) {
	for _, stage := range []string{"before-publication", "after-publication", "retention-promotion"} {
		t.Run(stage, func(t *testing.T) {
			f := newNativeHostFixture(t, "native:issue=read,create")
			actor := f.actor()
			if stage == "retention-promotion" {
				f.policy(actor, []c.Rule{{Operation: c.IssueCreate, Automatic: true}})
			}
			realGit, e := exec.LookPath("git")
			if e != nil {
				t.Fatal(e)
			}
			directory := t.TempDir()
			gate := filepath.Join(directory, "gate")
			sent := filepath.Join(directory, "sent")
			release := filepath.Join(directory, "release")
			// This is an external test Git executable. It stalls one real selected stage;
			// it neither synthesizes an event nor substitutes a writer implementation.
			script := `#!/bin/sh
stage="$HN_WP17_GIT_STAGE"
match=no
for arg do
 case "$arg" in
 for-each-ref) [ "$stage" = before-publication ] && match=yes ;;
 "hn: append issue.open") [ "$stage" = after-publication ] && match=yes ;;
 --stdin) if [ "$stage" = retention-promotion ]; then
  selected=no
  hooks=no
  update=no
  for item do
   [ "$item" = "--git-dir=$HN_WP17_SELECTED_GITDIR" ] && selected=yes
   [ "$item" = core.hooksPath=/dev/null ] && hooks=yes
   [ "$item" = update-ref ] && update=yes
  done
  if [ "$selected" = yes ] && [ "$hooks" = yes ] && [ "$update" = yes ]; then match=yes; fi
 fi ;;
 esac
done
if [ "$match" = yes ] && [ ! -f "$HN_WP17_CANCEL_GATE" ]; then
 if [ "$stage" = after-publication ]; then "$HN_WP17_REAL_GIT" "$@" || exit $?; fi
 printf '%s\n' "$@" > "$HN_WP17_CANCEL_GATE"
 count=0
 while [ ! -f "$HN_WP17_RELEASE" ]; do
  count=$((count+1)); [ "$count" -ge 500 ] && exit 88
  sleep 0.02
 done
 [ "$stage" = after-publication ] && exit 0
fi
exec "$HN_WP17_REAL_GIT" "$@"
`
			if e = os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0700); e != nil {
				t.Fatal(e)
			}
			selectedGitDir := f.git("rev-parse", "--absolute-git-dir")
			baseEnv := append([]string{}, f.env...)
			f.env = append(f.env, "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"), "HN_WP17_GIT_STAGE="+stage, "HN_WP17_CANCEL_GATE="+gate, "HN_WP17_CANCEL_SENT="+sent, "HN_WP17_RELEASE="+release, "HN_WP17_REAL_GIT="+realGit, "HN_WP17_SELECTED_GITDIR="+selectedGitDir)
			request := nativeMutation("issue", actor, "cancel-"+stage, nativeIssueState("Cancellation fixture"))
			raw, _ := json.Marshal([]any{request})
			os.WriteFile(f.input, raw, 0600)
			before := hostNativeSnapshot(t, f.repo)
			command := exec.Command(f.binary, "extension", "run", "example.native/tool", "--json")
			command.Dir = f.repo
			command.Env = f.env
			var out, stderr bytes.Buffer
			command.Stdout = &out
			command.Stderr = &stderr
			if e = command.Start(); e != nil {
				t.Fatal(e)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			until := time.Now().Add(10 * time.Second)
			for {
				if _, e = os.Stat(sent); e == nil {
					break
				}
				if time.Now().After(until) {
					command.Process.Kill()
					<-done
					t.Fatalf("cancel stage %s not reached: %s %s", stage, &out, &stderr)
				}
				time.Sleep(10 * time.Millisecond)
			}
			// Allow the independently read cancel control to cancel the request parent
			// before unblocking synchronous Git, which the host must join.
			time.Sleep(100 * time.Millisecond)
			if e = os.WriteFile(release, []byte("settle"), 0600); e != nil {
				t.Fatal(e)
			}
			select {
			case e = <-done:
				if e == nil {
					t.Fatalf("cancel reported clean completion: %s", &out)
				}
			case <-time.After(10 * time.Second):
				command.Process.Kill()
				<-done
				t.Fatal("native writer did not settle")
			}
			var envelope map[string]any
			if json.Unmarshal(out.Bytes(), &envelope) != nil {
				t.Fatalf("cancel envelope: %s %s", &out, &stderr)
			}
			code := "uncertain_operation"
			origin := "host"
			if stage == "retention-promotion" {
				code = "admission_pending"
				origin = "native"
			}
			nativeDenial(t, envelope, origin, code, "inspect_operation")
			op := envelope["error"].(map[string]any)["operation"].(map[string]any)
			if op["protocol"] != "hn/0" || op["actor"] != actor || op["operation"] != "cancel-"+stage {
				t.Fatalf("original cancellation identity %v", op)
			}
			f.env = baseEnv
			settled := hostNativeSnapshot(t, f.repo)
			time.Sleep(100 * time.Millisecond)
			nativeSnapshotEqual(t, f, settled)
			recovered := nativeOK(t, f.requests(nativeLookup("issue", actor, "cancel-"+stage))[0])
			if stage == "before-publication" {
				if recovered["status"] != "verified_absent" {
					t.Fatalf("known prepublication absence %v", recovered)
				}
				if !reflect.DeepEqual(before, settled) {
					t.Fatal("cancelled validation published or reserved native data")
				}
			} else if stage == "after-publication" {
				if recovered["status"] != "found" {
					t.Fatalf("published operation was called absent: %v", recovered)
				}
			} else {
				if recovered["status"] != "found" || recovered["reason"] != nil {
					t.Fatalf("verified admitted publication was lost during final pin cancellation: %v", recovered)
				}
				var rawLookup map[string]any
				if json.Unmarshal([]byte(f.native("issue", "operation", "--actor", actor, "--operation", "cancel-"+stage, "--json")), &rawLookup) != nil {
					t.Fatal("raw operation output")
				}
				original := rawLookup["data"].(map[string]any)["event_id"].(string)
				if recovered["event_id"] != original {
					t.Fatal("lookup changed the final-promotion signed publication")
				}
				if !validEventID(original) {
					t.Fatal("raw publication witness missing")
				}
				replay := nativeOK(t, f.requests(request)[0])
				if replay["event_id"] != original || replay["replayed"] != true || replay["admission"] != "admitted" {
					t.Fatalf("retention exact retry: %v", replay)
				}
				found := nativeOK(t, f.requests(nativeLookup("issue", actor, "cancel-"+stage))[0])
				if found["status"] != "found" || found["event_id"] != original {
					t.Fatalf("retention recovery changed signed event %v", found)
				}
			}
		})
	}
}

func TestExtensionNativeConflictAndResolveGrantCLI(t *testing.T) {
	for _, family := range []string{"issue", "project"} {
		t.Run(family, func(t *testing.T) {
			f := newNativeHostFixture(t, "native:"+family+"=read,create,revise,resolve")
			actor := f.actor()
			var state map[string]any
			if family == "issue" {
				state = nativeIssueState("Offline base")
			} else {
				state = map[string]any{"schema": "hn.project/1", "title": "Offline base", "body": "", "archived": false}
			}
			base := nativeOK(t, f.requests(nativeMutation(family, actor, "offline-base", state))[0])["event_id"].(string)
			f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "Offline fixture base")
			clone := t.TempDir()
			f.git("clone", "-q", "--no-local", f.repo, clone)
			other := *f
			other.repo = clone
			other.git("fetch", "-q", "origin", "refs/hn/actors/"+actor+":refs/hn/actors/"+actor)
			secondActor := other.actor()
			state["title"] = "First concurrent state"
			firstRequest := nativeMutation(family, actor, "first-edit", state)
			p := firstRequest["params"].(map[string]any)
			p["intent"], p["id"], p["expected"] = "revise", base, []any{base}
			first := nativeOK(t, f.requests(firstRequest)[0])["event_id"].(string)
			state["title"] = "Other concurrent state"
			input := filepath.Join(f.home, "offline-state.json")
			raw, _ := json.Marshal(state)
			os.WriteFile(input, raw, 0600)
			var result map[string]any
			if json.Unmarshal([]byte(other.native(family, "revise", base, "--expect", base, "--input", input, "--operation", "offline-edit", "--json")), &result) != nil {
				t.Fatal("offline native JSON")
			}
			second := result["data"].(map[string]any)["event_id"].(string)
			f.git("fetch", "-q", clone, "refs/hn/actors/"+secondActor+":refs/hn/actors/"+secondActor)
			before := hostNativeSnapshot(t, f.repo)
			page := nativeOK(t, f.requests(nativeReadRequest(family, base))[0])
			view := page["items"].([]any)[0].(map[string]any)
			if view["conflict"] != true || view["state"] != nil || len(view["heads"].([]any)) != 2 {
				t.Fatalf("concurrent heads flattened: %v", view)
			}
			heads := []string{}
			for _, h := range view["heads"].([]any) {
				heads = append(heads, h.(map[string]any)["id"].(string))
			}
			want := []string{first, second}
			sort.Strings(want)
			if !reflect.DeepEqual(heads, want) {
				t.Fatalf("head order %v != %v", heads, want)
			}
			conflict := family + "_conflict"
			if family == "project" {
				conflict = "resource_conflict"
			}
			p["operation"] = "refused-conflict"
			nativeDenial(t, f.requests(firstRequest)[0], "native", conflict, "do_not_retry")
			nativeSnapshotEqual(t, f, before)
			f.command("extension", "enable", "example.native/tool", "--grant", "native:"+family+"=read,create,revise", "--json")
			resolve := nativeMutation(family, actor, "resolve", state)
			q := resolve["params"].(map[string]any)
			q["intent"], q["id"], q["expected"] = "resolve", base, []any{want[0], want[1]}
			before = hostNativeSnapshot(t, f.repo)
			nativeDenial(t, f.requests(resolve)[0], "host", "denied_access", "do_not_retry")
			nativeSnapshotEqual(t, f, before)
			f.command("extension", "enable", "example.native/tool", "--grant", "native:"+family+"=read,create,revise,resolve", "--json")
			resolved := nativeOK(t, f.requests(resolve)[0])
			if resolved["conflict"] != false || resolved["head_count"] != float64(1) || resolved["resource_id"] != base {
				t.Fatalf("resolution %v", resolved)
			}
			replay := nativeOK(t, f.requests(resolve)[0])
			if replay["event_id"] != resolved["event_id"] || replay["replayed"] != true {
				t.Fatal("resolve retry checked stale heads first")
			}
		})
	}
}

func (f *nativeHostFixture) refusedSession(request map[string]any, code string) {
	f.t.Helper()
	raw, _ := json.Marshal([]any{request})
	if e := os.WriteFile(f.input, raw, 0600); e != nil {
		f.t.Fatal(e)
	}
	cmd := exec.Command(f.binary, "extension", "run", "example.native/tool", "--json")
	cmd.Dir = f.repo
	cmd.Env = f.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	e := cmd.Run()
	var result map[string]any
	if e == nil || json.Unmarshal(stdout.Bytes(), &result) != nil {
		f.t.Fatalf("want terminal %s: %v %s %s", code, e, &stdout, &stderr)
	}
	nativeDenial(f.t, result, "host", code, "do_not_retry")
}
func TestExtensionNativeUnboundAndUnsupportedFramesCLI(t *testing.T) {
	f := newNativeHostFixture(t, "native:issue=read,create")
	before := hostNativeSnapshot(t, f.repo)
	// There is no request-selectable repository. A forged repository field is a
	// malformed closed request before the host even considers a missing signer.
	request := nativeMutation("issue", strings.Repeat("a", 64), "wrong-repository", nativeIssueState("Never published"))
	request["params"].(map[string]any)["repository_id"] = eventID([]byte("another repository"))
	f.refusedSession(request, "invalid_input")
	nativeSnapshotEqual(t, f, before)
	for _, method := range []string{"memory.mutate", "proposal", "run", "runner", "policy.mutate", "identity.mutate", "sync.apply", "migration.apply"} {
		f.refusedSession(nativeRequest(method, map[string]any{}), "unsupported_capability")
		nativeSnapshotEqual(t, f, before)
	}
	request = nativeReadRequest("issue", nil)
	request["params"].(map[string]any)["limit"] = 201
	f.refusedSession(request, "resource_limit")
	nativeSnapshotEqual(t, f, before)
	request["params"].(map[string]any)["limit"] = 0
	f.refusedSession(request, "invalid_input")
	nativeSnapshotEqual(t, f, before)
	valid := nativeReadRequest("issue", nil)
	valid["params"].(map[string]any)["cursor"] = "not-a-native-cursor"
	nativeDenial(t, f.requests(valid)[0], "native", "invalid_cursor", "rescan_required")
	nativeSnapshotEqual(t, f, before)
}
