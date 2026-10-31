package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// The tested consumer is the supplied optional executable, never this test binary.
func fakeTrackerBinary(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("HN_WP20_CONSUMER"); p != "" {
		return p
	}
	p := filepath.Join(t.TempDir(), "fake-tracker")
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	git := exec.Command("git", "rev-parse", "--absolute-git-dir")
	directory, err := git.Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", p, "./examples/extensions/fake-tracker")
	cmd.Env = append(os.Environ(), "GIT_DIR="+strings.TrimSpace(string(directory)), "GIT_WORK_TREE="+source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone consumer build: %v: %s", err, out)
	}
	return p
}

func TestExtensionFakeTrackerStandaloneRecovery(t *testing.T) {
	consumer := fakeTrackerBinary(t)
	f := fakeTrackerFixture(t, consumer, "native:identity=read", "native:issue=read,observe,create,revise")
	actor := f.actor()
	f.trackerPlan(map[string]any{"action": "import", "pause": "native-sent:comment"})
	run := f.trackerStart()
	marker := f.waitTrackerMarker("native-sent:comment")
	if int(marker["pid"].(float64)) == os.Getpid() {
		t.Fatal("consumer is test process")
	}
	// The actual native comment must exist while its acknowledgement is unread.
	f.waitTrackerComments(1)
	beforeKill := hostNativeSnapshot(t, f.repo)
	f.killTracker(marker, run)
	uncertain := f.journal()
	if uncertain["native"].(map[string]any)["comment"].(map[string]any)["result"] != nil {
		t.Fatal("lost reply was already journaled as success")
	}
	original := uncertain["native"].(map[string]any)["comment"].(map[string]any)["params"]
	f.trackerRun(map[string]any{"action": "import"})
	j := f.journal()
	issue := j["native"].(map[string]any)["issue"].(map[string]any)
	comment := j["native"].(map[string]any)["comment"].(map[string]any)
	if !reflect.DeepEqual(original, comment["params"]) || j["actor"] != actor || j["public_key"] == "" {
		t.Fatal("restart changed pinned actor/key/exact intent")
	}
	if comment["lookup"].(map[string]any)["status"] != "found" || comment["result"].(map[string]any)["replayed"] != true {
		t.Fatal("restart did not recover and replay original operation", comment)
	}
	if !reflect.DeepEqual(beforeKill, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("recovery duplicated native facts/private native intents")
	}
	f.waitTrackerComments(1)
	id := issue["result"].(map[string]any)["resource_id"].(string)
	forgedPath := filepath.Join(f.home, "unassociated-native.json")
	forgedState := nativeIssueState("descriptive source marker cannot associate this issue")
	forgedState["metadata"] = map[string]any{"example.tracker/source": "tracker:item:42"}
	fakeWrite(t, forgedPath, forgedState)
	f.command("issue", "open", "--input", forgedPath, "--operation", "descriptive-marker-control", "--json")
	f.trackerRun(map[string]any{"action": "observe", "fresh": true, "limit": 1})
	checkpoint := f.journal()["checkpoint"]
	if f.serviceState()["writes"] != float64(0) {
		t.Fatal("import echoed to external service")
	}
	// Full native replacement deliberately removes the descriptive marker. The
	// connector-owned exact association still identifies this native resource;
	// the same marker on the unrelated control above grants no association.
	state := nativeIssueState("native edit exported once")
	input := filepath.Join(f.home, "revision.json")
	fakeWrite(t, input, state)
	head := issue["result"].(map[string]any)["event_id"].(string)
	f.command("issue", "revise", id, "--expect", head, "--input", input, "--operation", "native-edit", "--json")
	f.trackerPlan(map[string]any{"action": "observe", "lose_external_ack": true, "pause": "external-uncertain"})
	run = f.trackerStart()
	marker = f.waitTrackerMarker("external-uncertain")
	external := f.serviceState()
	if external["writes"] != float64(1) || f.journal()["checkpoint"] != checkpoint {
		t.Fatal("external response loss advanced checkpoint or did not really write")
	}
	externalItem := external["item"].(map[string]any)
	if externalItem["title"] != state["title"] || externalItem["body"] != state["body"] || externalItem["version"] != float64(2) {
		t.Fatal("actual external effect did not preserve the observed native edit")
	}
	if len(external["receipts"].(map[string]any)) != 1 {
		t.Fatal("external effect has no unique private receipt")
	}
	for _, value := range external["receipts"].(map[string]any) {
		request := value.(map[string]any)["request"].(map[string]any)
		if request["resource"] != id || request["source"] != "tracker:item:42" || request["expected_version"] != float64(1) {
			t.Fatal("external receipt changed its owned association or expected version")
		}
	}
	f.killTracker(marker, run)
	f.trackerRun(map[string]any{"action": "observe"})
	after := f.journal()
	if after["checkpoint"] == checkpoint || f.serviceState()["writes"] != float64(1) {
		t.Fatal("restart failed private reconciliation before checkpoint")
	}
	f.trackerRun(map[string]any{"action": "observe"})
	if f.serviceState()["writes"] != float64(1) {
		t.Fatal("duplicate observation re-exported")
	}
	// Raw changed immutable replay must reach the actual host, with no new effects.
	changed := comment["params"].(map[string]any)
	changed["comment"] = "changed replay"
	before := hostNativeSnapshot(t, f.repo)
	f.trackerRun(map[string]any{"action": "raw", "request": nativeRequest("resource.mutate", changed)})
	nativeDenial(t, f.lastResponse(), "native", "operation_conflict", "do_not_retry")
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
	f.assertTrackerPrivacy(id)
}

type fakeTrackerTestFixture struct {
	*nativeHostFixture
	consumer, config, plan, journalPath, report, marker, serviceFile, api string
	parent                                                                map[string]any
	apiItemReads                                                          atomic.Int64
}

func fakeWrite(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func fakeRead(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func fakeTrackerFixture(t *testing.T, consumer string, grants ...string) *fakeTrackerTestFixture {
	return fakeTrackerObservedFixture(t, consumer, false, grants...)
}
func fakeTrackerObservedFixture(t *testing.T, consumer string, observeReads bool, grants ...string) *fakeTrackerTestFixture {
	t.Helper()
	base := newNativeHostFixture(t, "native:issue=read")
	base.command("extension", "remove", "example.native/tool", "--json")
	f := &fakeTrackerTestFixture{nativeHostFixture: base, consumer: consumer}
	f.config = filepath.Join(f.home, "tracker-config.json")
	f.plan = filepath.Join(f.home, "tracker-plan.json")
	f.journalPath = filepath.Join(f.home, "tracker-journal.json")
	f.report = filepath.Join(f.home, "tracker-report.json")
	f.marker = filepath.Join(f.home, "tracker-marker.json")
	f.serviceFile = filepath.Join(f.home, "tracker-service.json")
	fakeWrite(t, f.serviceFile, map[string]any{"item": map[string]any{"id": "42", "version": 1, "title": "fake external issue", "body": "exact external text\n", "author": "external claimed author", "time": "1901-01-01T00:00:00Z", "comments": []any{map[string]any{"id": "c1", "body": "external comment claim", "author": "external comment author", "time": "2099-01-01T00:00:00Z"}}}, "receipts": map[string]any{}, "writes": 0})
	credential := filepath.Join(f.home, "tracker-credential.json")
	fakeWrite(t, credential, map[string]any{"token": "private-tracker-token"})
	ready := filepath.Join(f.home, "tracker-ready")
	cmd := exec.Command(consumer, "serve", "--state", f.serviceFile, "--listen", "127.0.0.1:0", "--ready", ready, "--credential", credential)
	cmd.Env = f.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	deadline := time.Now().Add(10 * time.Second)
	for {
		raw, err := os.ReadFile(ready)
		if err == nil && strings.HasPrefix(string(raw), "http://127.0.0.1:") {
			f.api = string(raw)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake API startup: %s", &stderr)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if observeReads {
		// This test-only transparent observer forwards every request to the
		// actual supplied service binary; it cannot supply a response itself.
		target, err := url.Parse(f.api)
		if err != nil {
			t.Fatal(err)
		}
		forward := httputil.NewSingleHostReverseProxy(target)
		observer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/item" {
				f.apiItemReads.Add(1)
			}
			forward.ServeHTTP(w, r)
		}))
		t.Cleanup(observer.Close)
		f.api = observer.URL
	}
	fakeWrite(t, f.config, map[string]any{"api": f.api, "token": "private-tracker-token", "connection": "local-test-connection", "source": "tracker:item:42", "journal": f.journalPath, "report": f.report, "marker": f.marker})
	manifest := filepath.Join(f.home, "tracker-manifest.json")
	fakeWrite(t, manifest, map[string]any{"protocol": "hn.extension-manifest/1", "module_id": "example.tracker/tool", "module_version": "1", "host_protocol": "hn.extension/1", "capabilities": []string{"identity.public", "resource.read", "resource.mutate", "resource.operation", "changes.capture", "changes.page", "changes.since", "capabilities.list"}})
	f.command("extension", "register", "--manifest", manifest, "--executable", consumer, "--arg", "--config", "--arg", f.config, "--arg", "--plan", "--arg", f.plan, "--json")
	args := []string{"extension", "enable", "example.tracker/tool"}
	for _, g := range grants {
		args = append(args, "--grant", g)
	}
	f.command(append(args, "--json")...)
	for _, p := range []string{consumer, f.binary} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("supplied executable %s SHA256 %x", p, sha256.Sum256(raw))
	}
	t.Cleanup(func() { f.trackerEvidence("final") })
	return f
}
func (f *fakeTrackerTestFixture) trackerPlan(plan map[string]any) {
	f.t.Helper()
	f.trackerEvidence("before-next-plan")
	_ = os.Remove(f.marker)
	_ = os.Remove(f.marker + ".continue")
	fakeWrite(f.t, f.plan, plan)
}
func (f *fakeTrackerTestFixture) trackerStart() *exec.Cmd {
	f.t.Helper()
	f.parent = nil
	cmd := exec.Command(f.binary, "extension", "run", "example.tracker/tool", "--json")
	cmd.Dir = f.repo
	cmd.Env = f.env
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Start(); err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		if cmd.ProcessState == nil {
			// Let the real host perform its owned child/group cleanup, including
			// when a deliberate broken-behavior control fails this test early.
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
		}
	})
	return cmd
}
func (f *fakeTrackerTestFixture) trackerRun(plan map[string]any) {
	f.t.Helper()
	f.trackerPlan(plan)
	cmd := f.trackerStart()
	if err := cmd.Wait(); err != nil {
		f.trackerEvidence("failed-run")
		f.t.Fatalf("tracker %v: %v %s %s report=%v", plan, err, cmd.Stdout, cmd.Stderr, f.readReport())
	}
	if err := json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &f.parent); err != nil {
		f.t.Fatal("invalid host management summary")
	}
	f.trackerEvidence("completed-run")
}
func (f *fakeTrackerTestFixture) readReport() map[string]any {
	f.t.Helper()
	return fakeRead(f.t, f.report)
}
func (f *fakeTrackerTestFixture) lastResponse() map[string]any {
	f.t.Helper()
	calls := f.readReport()["calls"].([]any)
	return calls[len(calls)-1].(map[string]any)["response"].(map[string]any)
}
func (f *fakeTrackerTestFixture) journal() map[string]any      { return fakeRead(f.t, f.journalPath) }
func (f *fakeTrackerTestFixture) serviceState() map[string]any { return fakeRead(f.t, f.serviceFile) }
func (f *fakeTrackerTestFixture) waitTrackerMarker(phase string) map[string]any {
	f.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if raw, err := os.ReadFile(f.marker); err == nil {
			var v map[string]any
			if json.Unmarshal(raw, &v) == nil && v["phase"] == phase {
				return v
			}
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("no consumer marker %s", phase)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func (f *fakeTrackerTestFixture) killTracker(marker map[string]any, cmd *exec.Cmd) {
	f.t.Helper()
	f.trackerEvidence("before-kill")
	p, err := os.FindProcess(int(marker["pid"].(float64)))
	if err != nil {
		f.t.Fatal(err)
	}
	if err = p.Kill(); err != nil {
		f.t.Fatal(err)
	}
	if err = cmd.Wait(); err == nil {
		f.t.Fatal("killed consumer silently succeeded")
	}
	if err = json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &f.parent); err != nil {
		f.t.Fatal("killed host omitted structured result")
	}
	failure, ok := f.parent["error"].(map[string]any)
	if f.parent["ok"] != false || f.parent["data"] != nil || !ok || failure["origin"] != "host" {
		f.t.Fatal("killed consumer parent result", f.parent)
	}
	switch failure["code"] {
	case "unavailable_module":
		if failure["recovery"] != "do_not_retry" || failure["operation"] != nil {
			f.t.Fatal("completed host result must retain its exact exit refusal", f.parent)
		}
	case "uncertain_operation":
		// Publication is known to this test, but the host may still have been
		// settling its dispatched request when the pipe closed. Preserve its exact
		// original operation binding rather than relabeling it absent/cancelled.
		if !strings.HasPrefix(marker["phase"].(string), "native-sent:") || failure["recovery"] != "inspect_operation" {
			f.t.Fatal("unexpected pending parent operation", f.parent)
		}
		operation, ok := failure["operation"].(map[string]any)
		if !ok || operation["protocol"] != "hn/0" || operation["actor"] != f.journal()["actor"] {
			f.t.Fatal("lost original native operation binding", f.parent)
		}
		matched := false
		for _, value := range f.journal()["native"].(map[string]any) {
			intent := value.(map[string]any)
			if intent["params"].(map[string]any)["operation"] == operation["operation"] && intent["result"] == nil {
				matched = true
			}
		}
		if !matched {
			f.t.Fatal("parent invented pending operation", f.parent)
		}
	default:
		f.t.Fatal("wrong consumer-kill refusal cause", f.parent)
	}
	f.t.Logf("actual killed-consumer parent cause: %s/%s", failure["code"], failure["recovery"])
	f.trackerEvidence("after-kill")
}
func (f *fakeTrackerTestFixture) waitTrackerComments(count int) {
	f.t.Helper()
	j := f.journal()
	f.waitNativeOperation(j["native"].(map[string]any)["comment"].(map[string]any)["params"].(map[string]any)["operation"].(string))
	for {
		j := f.journal()
		id := j["native"].(map[string]any)["issue"].(map[string]any)["result"].(map[string]any)["resource_id"].(string)
		v := f.command("issue", "history", id, "--kind", "comments", "--json")
		raw, _ := json.Marshal(v)
		if v["data"].(map[string]any)["total"] == float64(count) && bytes.Count(raw, []byte("external comment claim")) == count {
			return
		}
		f.t.Fatalf("native comment count/content differs after publication: %s", raw)
	}
}
func (f *fakeTrackerTestFixture) assertTrackerPrivacy(id string) {
	f.t.Helper()
	public := f.native("issue", "history", id, "--json") + f.native("extension", "inspect", "--json")
	for _, secret := range []string{"private-tracker-token", f.journalPath, "local-test-connection"} {
		if strings.Contains(public, secret) {
			f.t.Fatal("private connector material leaked", secret)
		}
	}
	// Public Git object traversal includes all transported signed issue facts.
	raw := f.git("log", "--all", "-p", "--format=")
	if strings.Contains(raw, "private-tracker-token") || strings.Contains(raw, "local-test-connection") {
		f.t.Fatal("public objects contain connector private data")
	}
	if !strings.Contains(public, "external claimed author") || !strings.Contains(public, "1901-01-01T00:00:00Z") {
		f.t.Fatal("external descriptive claims missing")
	}
	// Every imported claim is signed by the actual native actor, rather than the
	// claimed author or either deliberately misleading claimed timestamp.
	history := f.command("issue", "history", id, "--json")["data"].(map[string]any)["items"].([]any)
	actor := f.journal()["actor"]
	for _, value := range history {
		if value.(map[string]any)["actor"] != actor {
			f.t.Fatal("external author impersonated native signer")
		}
	}
	f.assertTrackerClone(id)
}

func (f *fakeTrackerTestFixture) assertTrackerClone(id string) {
	f.t.Helper()
	bare := filepath.Join(f.t.TempDir(), "public.git")
	f.git("init", "--bare", "-q", bare)
	f.git("remote", "add", "public", bare)
	f.native("sync", "public")
	clone := &nativeHostFixture{t: f.t, binary: f.binary, repo: f.t.TempDir(), home: f.t.TempDir()}
	clone.env = operationalTestEnvironmentWith(clone.home, map[string]string{"XDG_CONFIG_HOME": filepath.Join(clone.home, "config")})
	command := exec.Command("git", "init", "-q", clone.repo)
	command.Env = clone.env
	if out, err := command.CombinedOutput(); err != nil {
		f.t.Fatalf("independent repo: %v %s", err, out)
	}
	if clone.actor() == f.journal()["actor"] {
		f.t.Fatal("independent clone reused source signer")
	}
	clone.git("remote", "add", "public", bare)
	clone.native("replication", "select", "public", "--actor", f.journal()["actor"].(string))
	clone.native("sync", "public")
	original := f.command("issue", "history", id, "--json")["data"]
	copied := clone.command("issue", "history", id, "--json")["data"]
	if !reflect.DeepEqual(original, copied) {
		f.t.Fatal("plugin-free public transfer changed exact issue/comment history")
	}
	if strings.Contains(clone.native("extension", "inspect", "--json"), "example.tracker/tool") {
		f.t.Fatal("clone inherited registration")
	}
	for _, name := range []string{"tracker-config.json", "tracker-journal.json", "fake-tracker"} {
		if _, err := os.Stat(filepath.Join(clone.home, name)); !os.IsNotExist(err) {
			f.t.Fatal("clone inherited private connector asset", name)
		}
	}
	f.command("extension", "remove", "example.tracker/tool", "--json")
	if !reflect.DeepEqual(original, f.command("issue", "history", id, "--json")["data"]) {
		f.t.Fatal("removing module changed signed history")
	}
}

func (f *fakeTrackerTestFixture) openNative(title string) string {
	f.t.Helper()
	path := filepath.Join(f.home, "native-open.json")
	fakeWrite(f.t, path, nativeIssueState(title))
	result := f.command("issue", "open", "--input", path, "--operation", title, "--json")
	return result["data"].(map[string]any)["issue_id"].(string)
}
func (f *fakeTrackerTestFixture) trackerAttempt(plan map[string]any) (map[string]any, error) {
	f.t.Helper()
	f.trackerPlan(plan)
	cmd := f.trackerStart()
	err := cmd.Wait()
	var result map[string]any
	if e := json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &result); e != nil {
		f.t.Fatalf("parent summary: %s %s", cmd.Stdout, cmd.Stderr)
	}
	f.parent = result
	f.trackerEvidence("attempt-result")
	return result, err
}
func TestExtensionFakeTrackerReplacementDisconnectAndInvalidation(t *testing.T) {
	consumer := fakeTrackerBinary(t)
	f := fakeTrackerFixture(t, consumer, "native:identity=read", "native:issue=read,observe,create,revise")
	actor := f.actor()
	f.trackerRun(map[string]any{"action": "import"})
	f.trackerRun(map[string]any{"action": "observe", "fresh": true, "limit": 200})
	original := f.journal()
	base := f.git("rev-parse", "refs/hn/actors/"+actor)
	f.openNative("unrelated-one")
	f.openNative("unrelated-two")
	f.trackerPlan(map[string]any{"action": "observe", "fresh": true, "limit": 1, "pause": "page"})
	run := f.trackerStart()
	marker := f.waitTrackerMarker("page")
	partial := f.lastResponse()["data"].(map[string]any)
	if partial["complete"] != false || partial["checkpoint"] != nil || !reflect.DeepEqual(original, f.journal()) {
		t.Fatal("partial page advanced or replaced committed state")
	}
	f.killTracker(marker, run)
	f.trackerRun(map[string]any{"action": "raw", "request": nativeRequest("changes.page", map[string]any{"capture": partial["capture"], "cursor": partial["next_cursor"], "limit": 1})})
	nativeDenial(t, f.lastResponse(), "host", "checkpoint_invalidated", "rescan_required")
	if !reflect.DeepEqual(original, f.journal()) {
		t.Fatal("disconnected capture changed durable state")
	}
	f.trackerRun(map[string]any{"action": "observe", "fresh": true, "limit": 1})
	complete := f.journal()
	if len(complete["entries"].(map[string]any)) != 3 || complete["checkpoint"] == original["checkpoint"] {
		t.Fatal("completed pages did not replace exact selection")
	}
	// Accepted source changes between pages invalidate the live capture. Its suffix
	// cannot be silently spliced into a fresh set or committed as a partial view.
	f.trackerPlan(map[string]any{"action": "observe", "fresh": true, "limit": 1, "pause": "page"})
	run = f.trackerStart()
	f.waitTrackerMarker("page")
	f.openNative("source-change-during-capture")
	if err := os.WriteFile(f.marker+".continue", []byte("continue"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run.Wait(); err == nil {
		t.Fatal("changed-cut capture silently succeeded")
	}
	nativeDenial(t, f.lastResponse(), "host", "checkpoint_invalidated", "rescan_required")
	if !reflect.DeepEqual(complete, f.journal()) || f.serviceState()["writes"] != float64(0) {
		t.Fatal("invalidated staging changed checkpoint or external state")
	}
	// Rewind to an independently valid accepted prefix: the old authenticated
	// checkpoint loses its evidence. A fresh capture must remove the absent suffix.
	f.git("update-ref", "refs/hn/actors/"+actor, base)
	before := hostNativeSnapshot(t, f.repo)
	if _, err := f.trackerAttempt(map[string]any{"action": "observe"}); err == nil {
		t.Fatal("lost checkpoint evidence silently succeeded")
	}
	nativeDenial(t, f.lastResponse(), "host", "checkpoint_invalidated", "rescan_required")
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
	if !reflect.DeepEqual(complete, f.journal()) {
		t.Fatal("invalidated checkpoint replaced old committed set")
	}
	f.trackerRun(map[string]any{"action": "observe", "fresh": true, "limit": 1})
	replaced := f.journal()
	atBound := hostNativeSnapshot(t, f.repo)
	if _, err := f.trackerAttempt(map[string]any{"action": "observe", "fresh": true, "limit": 201}); err == nil {
		t.Fatal("one-over local page plan accepted")
	}
	if f.readReport()["status"] != "invalid_input" {
		t.Fatal("wrong local page plan cause")
	}
	nativeSnapshotEqual(t, f.nativeHostFixture, atBound)
	if !reflect.DeepEqual(replaced, f.journal()) {
		t.Fatal("over-limit local page plan changed committed state")
	}
	if len(replaced["entries"].(map[string]any)) != 1 || replaced["checkpoint"] == complete["checkpoint"] {
		t.Fatal("fresh replacement retained absent resources")
	}
}

func (f *fakeTrackerTestFixture) externalEdit(expected int, title string) {
	f.t.Helper()
	request := map[string]any{"key": "external-user-edit-" + strconv.Itoa(expected), "expected_version": expected, "source": "tracker:item:42", "resource": "external-42", "revisions": []string{}, "title": title, "body": "external concurrent body", "lose_ack": false}
	raw, _ := json.Marshal(request)
	req, err := http.NewRequest("POST", f.api+"/writes", bytes.NewReader(raw))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer private-tracker-token")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		f.t.Fatal("external edit", response.StatusCode)
	}
}
func TestExtensionFakeTrackerConcurrentEditsRefuseWithoutResolution(t *testing.T) {
	consumer := fakeTrackerBinary(t)
	f := fakeTrackerFixture(t, consumer, "native:identity=read", "native:issue=read,observe,create,revise")
	f.actor()
	f.trackerRun(map[string]any{"action": "import"})
	f.trackerRun(map[string]any{"action": "observe", "fresh": true})
	f.externalEdit(1, "explicit external update")
	f.trackerRun(map[string]any{"action": "update"})
	f.trackerRun(map[string]any{"action": "observe"})
	if f.serviceState()["writes"] != float64(1) {
		t.Fatal("explicit imported update echoed to external API")
	}
	j := f.journal()
	issue := j["native"].(map[string]any)["issue"].(map[string]any)["result"].(map[string]any)
	id := issue["resource_id"].(string)
	state := nativeIssueState("independent native edit")
	state["metadata"] = map[string]any{"example.tracker/source": "tracker:item:42"}
	path := filepath.Join(f.home, "conflict-native.json")
	fakeWrite(t, path, state)
	f.command("issue", "revise", id, "--expect", j["native_head"].(string), "--input", path, "--operation", "native-concurrent", "--json")
	f.externalEdit(2, "independent external edit")
	externalBeforeRefusal := f.serviceState()
	before := hostNativeSnapshot(t, f.repo)
	checkpoint := j["checkpoint"]
	if _, err := f.trackerAttempt(map[string]any{"action": "update"}); err == nil {
		t.Fatal("stale concurrent import silently overwrote native edit")
	}
	nativeDenial(t, f.lastResponse(), "native", "stale_revision", "do_not_retry")
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
	if _, err := f.trackerAttempt(map[string]any{"action": "observe"}); err == nil {
		t.Fatal("external version conflict silently resolved")
	}
	if f.readReport()["status"] != "external_conflict" || !reflect.DeepEqual(externalBeforeRefusal, f.serviceState()) || f.journal()["checkpoint"] != checkpoint || !reflect.DeepEqual(j["entries"], f.journal()["entries"]) {
		t.Fatal("external conflict changed state or checkpoint")
	}
	f.assertOnlyNewCaptureDescriptor(before)
	// Authenticate that exact delivered checkpoint through its actual public
	// caller. A corrupted token refuses with no new capture/native effects.
	completed := nativeOK(t, f.lastResponse())
	deliveredCheckpoint := completed["checkpoint"].(string)
	unchangedJournal, err := os.ReadFile(f.journalPath)
	if err != nil {
		t.Fatal(err)
	}
	externalBefore := f.serviceState()
	authBefore := hostNativeSnapshot(t, f.repo)
	f.trackerRun(map[string]any{"action": "raw", "request": nativeRequest("changes.since", map[string]any{"checkpoint": deliveredCheckpoint, "selection": completed["selection"], "limit": 200})})
	authenticated := nativeOK(t, f.lastResponse())
	for _, field := range []string{"snapshot_id", "selection", "selection_digest", "source_cut", "items"} {
		if !reflect.DeepEqual(completed[field], authenticated[field]) {
			t.Fatal("checkpoint did not authenticate exact accepted descriptor", field)
		}
	}
	f.assertOnlyNewCaptureDescriptor(authBefore)
	corrupt := deliveredCheckpoint[:len(deliveredCheckpoint)-1] + "0"
	if corrupt == deliveredCheckpoint {
		corrupt = deliveredCheckpoint[:len(deliveredCheckpoint)-1] + "1"
	}
	authBefore = hostNativeSnapshot(t, f.repo)
	f.trackerRun(map[string]any{"action": "raw", "request": nativeRequest("changes.since", map[string]any{"checkpoint": corrupt, "selection": completed["selection"], "limit": 200})})
	nativeDenial(t, f.lastResponse(), "host", "checkpoint_invalidated", "rescan_required")
	nativeSnapshotEqual(t, f.nativeHostFixture, authBefore)
	journalAfter, err := os.ReadFile(f.journalPath)
	if err != nil || !bytes.Equal(unchangedJournal, journalAfter) || !reflect.DeepEqual(externalBefore, f.serviceState()) {
		t.Fatal("checkpoint authentication controls changed connector/external state", err)
	}
	if f.command("issue", "show", id, "--json")["data"].(map[string]any)["state"].(map[string]any)["title"] != "independent native edit" {
		t.Fatal("silent native conflict resolution")
	}
}
func TestExtensionFakeTrackerRawGrantRepositoryAndPrivacyControls(t *testing.T) {
	consumer := fakeTrackerBinary(t)
	f := fakeTrackerFixture(t, consumer, "native:issue=read")
	request := nativeMutation("issue", strings.Repeat("a", 64), "ungranted", nativeIssueState("must not exist"))
	before := hostNativeSnapshot(t, f.repo)
	f.trackerRun(map[string]any{"action": "raw", "request": request})
	nativeDenial(t, f.lastResponse(), "host", "denied_access", "do_not_retry")
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
	if _, err := os.Stat(f.journalPath); !os.IsNotExist(err) {
		t.Fatal("raw denied call created connector intent")
	}
	f.trackerRun(map[string]any{"action": "raw", "request": nativeReadRequest("issue", nil)})
	if len(nativeOK(t, f.lastResponse())["items"].([]any)) != 0 {
		t.Fatal("denied write published")
	}
	forged := nativeReadRequest("issue", nil)
	forged["params"].(map[string]any)["repository_id"] = "sha256:" + strings.Repeat("b", 64)
	parent, err := f.trackerAttempt(map[string]any{"action": "raw", "request": forged})
	if err == nil {
		t.Fatal("forged repository selector accepted")
	}
	e := parent["error"].(map[string]any)
	if e["origin"] != "host" || e["code"] != "invalid_input" || e["recovery"] != "do_not_retry" {
		t.Fatal("wrong repository field refusal cause", parent)
	}
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
	f.trackerRun(map[string]any{"action": "raw", "request": nativeRequest("capabilities.list", map[string]any{})})
	public, _ := json.Marshal(f.lastResponse())
	for _, secret := range []string{"private-tracker-token", f.journalPath, "local-test-connection"} {
		if bytes.Contains(public, []byte(secret)) {
			t.Fatal("capability privacy leak", secret)
		}
	}
	originalRepo := f.repo
	f.repo = t.TempDir()
	defer func() { f.repo = originalRepo }()
	cmd := exec.Command("git", "init", "-q", f.repo)
	cmd.Env = f.env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("other repo %v %s", err, out)
	}
	reportBefore, _ := os.ReadFile(f.report)
	otherBefore := hostNativeSnapshot(t, f.repo)
	parent, err = f.trackerAttempt(map[string]any{"action": "raw", "request": request})
	if err == nil {
		t.Fatal("repository inherited grants")
	}
	e = parent["error"].(map[string]any)
	if e["code"] != "denied_access" || e["recovery"] != "do_not_retry" {
		t.Fatal("wrong unenabled repository cause", parent)
	}
	after, _ := os.ReadFile(f.report)
	if !bytes.Equal(reportBefore, after) {
		t.Fatal("wrong-repo run spawned consumer")
	}
	nativeSnapshotEqual(t, f.nativeHostFixture, otherBefore)
	if f.serviceState()["writes"] != float64(0) {
		t.Fatal("negative control made external effects")
	}
}

func TestExtensionFakeTrackerNativeDefaultInert(t *testing.T) {
	consumer := fakeTrackerBinary(t)
	f := fakeTrackerFixture(t, consumer, "native:issue=read")
	f.actor()
	id := f.openNative("ordinary-native-default")
	registry := filepath.Join(f.home, "config", "hubnot", "extensions", "registry.json")
	if err := os.WriteFile(registry, []byte("{ deliberately malformed registry"), 0600); err != nil {
		t.Fatal(err)
	}
	fakeWrite(t, f.config, map[string]any{"token": "PRIVATE-DO-NOT-OPEN-CONFIG"})
	before := hostNativeSnapshot(t, f.repo)
	f.trackerEvidence("before-native-default")
	trace := extensionBaselineTraceRun(t, f.env, f.repo, f.binary, "issue", "show", id, "--json")
	f.trackerTraceEvidence("native-default", []byte(trace))
	f.trackerEvidence("after-native-default")
	if cause := extensionBaselineViolation(trace, registry, consumer, before, hostNativeSnapshot(t, f.repo)); cause != "" {
		t.Fatal("native default gained extension work", cause)
	}
	if _, err := os.Stat(f.journalPath); !os.IsNotExist(err) {
		t.Fatal("ordinary native read initialized connector")
	}
	if f.serviceState()["writes"] != float64(0) {
		t.Fatal("ordinary native read wrote to API")
	}
	// Actual detector negative control, using a real read of this malformed path.
	if *extensionBaselineTrace != "" {
		path := filepath.Join(f.home, "negative.trace")
		cmd := exec.Command(*extensionBaselineTrace, "-f", "-qq", "-s", "4096", "-e", "trace=openat,execve,connect", "-o", path, "cat", registry)
		cmd.Env = f.env
		cmd.Dir = f.repo
		if _, err := cmd.Output(); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f.trackerTraceEvidence("actual-registry-read-control", raw)
		if cause := extensionBaselineViolation(string(raw), registry, consumer, before, before); cause != "registry-read" {
			t.Fatal("detector missed deliberate registry read", cause)
		}
	}
}

func TestExtensionFakeTrackerRecoveryKeepsOriginalContentAndParents(t *testing.T) {
	consumer := fakeTrackerBinary(t)
	f := fakeTrackerObservedFixture(t, consumer, true, "native:identity=read", "native:issue=read,observe,create,revise")
	f.actor()
	f.trackerPlan(map[string]any{"action": "import", "pause": "native-sent:issue"})
	run := f.trackerStart()
	marker := f.waitTrackerMarker("native-sent:issue")
	initial := f.journal()
	original := initial["native"].(map[string]any)["issue"].(map[string]any)["params"]
	f.waitNativeOperation(original.(map[string]any)["operation"].(string))
	initialIssues := f.command("issue", "list", "--json")["data"].(map[string]any)["items"].([]any)
	if len(initialIssues) != 1 {
		t.Fatal("initial issue was not really published")
	}
	originalIssueID := initialIssues[0].(map[string]any)["id"]
	if f.journal()["native"].(map[string]any)["issue"].(map[string]any)["result"] != nil {
		t.Fatal("issue acknowledgement was not lost")
	}
	f.killTracker(marker, run)
	f.externalEdit(1, "external source changed while disconnected")
	f.trackerRun(map[string]any{"action": "import"})
	recovered := f.journal()
	issue := recovered["native"].(map[string]any)["issue"].(map[string]any)
	if !reflect.DeepEqual(original, issue["params"]) || issue["lookup"].(map[string]any)["status"] != "found" || issue["result"].(map[string]any)["resource_id"] != originalIssueID {
		t.Fatal("restart regenerated original import from latest external content")
	}
	if recovered["external_version"] != float64(1) {
		t.Fatal("restart silently replaced original external version with newer source version")
	}
	recoveredIssues := f.command("issue", "list", "--json")["data"].(map[string]any)["items"].([]any)
	if len(recoveredIssues) != 1 || recoveredIssues[0].(map[string]any)["id"] != originalIssueID {
		t.Fatal("lost issue acknowledgement created a duplicate")
	}
	f.waitTrackerComments(1)
	before := hostNativeSnapshot(t, f.repo)
	f.trackerRun(map[string]any{"action": "import"})
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
	// A later explicit import update has its own key and exact original parent.
	f.trackerPlan(map[string]any{"action": "update", "pause": "native-sent:update"})
	run = f.trackerStart()
	marker = f.waitTrackerMarker("native-sent:update")
	id := recovered["native"].(map[string]any)["issue"].(map[string]any)["result"].(map[string]any)["resource_id"].(string)
	pendingUpdate := f.journal()["native"].(map[string]any)["update-2"].(map[string]any)["params"].(map[string]any)
	f.waitNativeOperation(pendingUpdate["operation"].(string))
	current := f.command("issue", "show", id, "--json")["data"].(map[string]any)
	if current["state"].(map[string]any)["title"] != "external source changed while disconnected" {
		t.Fatal("published update has wrong content")
	}
	f.killTracker(marker, run)
	pending := f.journal()["native"].(map[string]any)["update-2"].(map[string]any)["params"]
	f.externalEdit(2, "third external version while original update acknowledgement is lost")
	originalUpdate := current["head_ids"].([]any)[0].(string)
	path := filepath.Join(f.home, "later-native.json")
	fakeWrite(t, path, nativeIssueState("newer native revision after unread update reply"))
	f.command("issue", "revise", id, "--expect", originalUpdate, "--input", path, "--operation", "after-unread-update", "--json")
	before = hostNativeSnapshot(t, f.repo)
	reads := f.apiItemReads.Load()
	parent, recoveryErr := f.trackerAttempt(map[string]any{"action": "update"})
	if f.apiItemReads.Load() != reads {
		t.Fatalf("pending update recovery read fresh external source before original operation settlement: parent=%v error=%v", parent, recoveryErr)
	}
	if recoveryErr != nil {
		t.Fatal("original pending update recovery failed", parent, recoveryErr)
	}
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
	if f.journal()["external_version"] != float64(2) || f.serviceState()["item"].(map[string]any)["version"] != float64(3) {
		t.Fatal("pending update recovery silently adopted newer external version")
	}
	if _, minted := f.journal()["native"].(map[string]any)["update-3"]; minted {
		t.Fatal("unresolved original update minted a new operation")
	}
	update := f.journal()["native"].(map[string]any)["update-2"].(map[string]any)
	if !reflect.DeepEqual(pending, update["params"]) || update["lookup"].(map[string]any)["event_id"] != originalUpdate || update["result"].(map[string]any)["event_id"] != originalUpdate || update["result"].(map[string]any)["replayed"] != true {
		t.Fatal("restart changed original content/parents or lost original operation identity")
	}
	newer := f.command("issue", "show", id, "--json")["data"].(map[string]any)
	if newer["state"].(map[string]any)["title"] != "newer native revision after unread update reply" {
		t.Fatal("recovery silently overwrote a newer native head")
	}
	changedParents := update["params"].(map[string]any)
	changedParents["expected"] = newer["head_ids"]
	before = hostNativeSnapshot(t, f.repo)
	f.trackerRun(map[string]any{"action": "raw", "request": nativeRequest("resource.mutate", changedParents)})
	nativeDenial(t, f.lastResponse(), "native", "operation_conflict", "do_not_retry")
	nativeSnapshotEqual(t, f.nativeHostFixture, before)
}

// The host completed an accepted replacement before the connector encountered
// its external conflict. That may persist one host checkpoint descriptor; it
// must not alter any previous private byte or create a public/native object.
func (f *fakeTrackerTestFixture) assertOnlyNewCaptureDescriptor(before map[string]string) {
	f.t.Helper()
	after := hostNativeSnapshot(f.t, f.repo)
	for path, hash := range before {
		if after[path] != hash {
			f.t.Fatal("capture changed previous native/private state", path)
		}
	}
	added := 0
	for path := range after {
		if _, ok := before[path]; ok {
			continue
		}
		name := filepath.Base(path)
		if !strings.HasPrefix(path, "/.git/hn/extensions/changes/") || len(name) != 69 || !strings.HasSuffix(name, ".json") {
			f.t.Fatal("capture created unexpected native/public effect", path)
		}
		if _, err := hex.DecodeString(strings.TrimSuffix(name, ".json")); err != nil {
			f.t.Fatal("invalid checkpoint descriptor identity")
		}
		page := nativeOK(f.t, f.lastResponse())
		if page["complete"] != true || page["checkpoint"] == nil {
			f.t.Fatal("no completed page authorizes this descriptor")
		}
		// This private-storage assertion belongs only to the integration
		// fixture. The standalone connector treats the token as opaque.
		token := page["checkpoint"].(string)
		if len(token) != 129 || name != token[:64]+".json" {
			f.t.Fatal("exception is not the exact delivered checkpoint descriptor")
		}
		descriptor := fakeRead(f.t, filepath.Join(f.repo, strings.TrimPrefix(path, "/")))
		for _, field := range []string{"selection", "selection_digest", "source_cut", "snapshot_id"} {
			if !reflect.DeepEqual(descriptor[field], page[field]) {
				f.t.Fatal("private descriptor differs from exact completed page", field)
			}
		}
		hello := fakeRead(f.t, f.report)["hello"].(map[string]any)["hello"].(map[string]any)
		if descriptor["repository"] != hello["repository_id"] || descriptor["registration"] != hello["registration_id"] {
			f.t.Fatal("private descriptor differs from authenticated session binding")
		}
		added++
	}
	if added != 1 {
		f.t.Fatal("expected one completed host capture descriptor", added)
	}
}

// This scheduling probe reads one immutable public Git event at a time. Only
// after the actual ref publication does the accepted CLI reader verify it, so
// this test never turns a genuine concurrent_update refusal into a retry loop.
func (f *fakeTrackerTestFixture) waitNativeOperation(operation string) {
	f.t.Helper()
	actor := f.journal()["actor"].(string)
	deadline := time.Now().Add(10 * time.Second)
	for {
		oid := f.git("for-each-ref", "--format=%(objectname)", "refs/hn/actors/"+actor)
		if oid != "" {
			raw := f.git("show", oid+":event.json")
			var event map[string]any
			if json.Unmarshal([]byte(raw), &event) != nil {
				f.t.Fatal("published native event is invalid JSON")
			}
			if event["operation"] == operation {
				return
			}
		}
		if time.Now().After(deadline) {
			f.t.Fatal("native operation never published", operation)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Detailed traces, credentials and connector recovery descriptors are deliberately
// external/private; none are committed or included in public product output.
func (f *fakeTrackerTestFixture) trackerEvidence(phase string) {
	root := os.Getenv("HN_WP20_EVIDENCE")
	if root == "" {
		return
	}
	f.t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		f.t.Fatal(err)
	}
	trace := map[string]any{"test": f.t.Name(), "phase": phase, "parent": f.parent, "native_snapshot": hostNativeSnapshot(f.t, f.repo), "refs": f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/")}
	for label, path := range map[string]string{"config": f.config, "plan": f.plan, "journal": f.journalPath, "report": f.report, "service": f.serviceFile} {
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			f.t.Fatal(err)
		}
		trace[label] = json.RawMessage(raw)
	}
	for label, path := range map[string]string{"host": f.binary, "consumer": f.consumer} {
		raw, err := os.ReadFile(path)
		if err != nil {
			f.t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		trace[label+"_sha256"] = hex.EncodeToString(sum[:])
	}
	file, err := os.CreateTemp(root, strings.ReplaceAll(f.t.Name(), "/", "_")+"-"+phase+"-*.json")
	if err != nil {
		f.t.Fatal(err)
	}
	if err = json.NewEncoder(file).Encode(trace); err != nil {
		file.Close()
		f.t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fakeTrackerTestFixture) trackerTraceEvidence(label string, raw []byte) {
	root := os.Getenv("HN_WP20_EVIDENCE")
	if root == "" {
		return
	}
	f.t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		f.t.Fatal(err)
	}
	file, err := os.CreateTemp(root, f.t.Name()+"-"+label+"-*.trace")
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err = file.Write(raw); err != nil {
		file.Close()
		f.t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		f.t.Fatal(err)
	}
}

// A scripted hostile host drives the supplied standalone consumer. This is a
// post-implementation fault control; the real public-host roundtrip is above.
func TestExtensionFakeTrackerRejectsHostResponseFaults(t *testing.T) {
	consumer := fakeTrackerBinary(t)
	key := bytes.Repeat([]byte{7}, 32)
	actorSum := sha256.Sum256(key)
	actor := hex.EncodeToString(actorSum[:])
	id := func(c string) string { return "sha256:" + strings.Repeat(c, 64) }
	hello := map[string]any{"protocol": "hn.extension/1", "hello": map[string]any{"session_id": strings.Repeat("1", 32), "module_id": "example.tracker/tool", "module_version": "1", "repository_id": id("d"), "registration_id": id("e")}, "limits": map[string]any{"frame_bytes": 8388608, "read_timeout_ms": 30000, "mutation_timeout_ms": 240000, "page_max": 200, "one_outstanding": true}}
	frame := func(v any) []byte {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]byte, len(raw)+4)
		binary.BigEndian.PutUint32(out, uint32(len(raw)))
		copy(out[4:], raw)
		return out
	}
	for _, fault := range []string{"valid complete responses", "terminal null ID", "wrong correlation", "wrong nested type", "inconsistent ok data error"} {
		t.Run(fault, func(t *testing.T) {
			var apiReads atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// This script has exactly one possible API request, on the valid
				// path; its counter is examined only after consumer exit.
				apiReads.Add(1)
				if r.Method != "GET" || r.URL.Path != "/item" || r.Header.Get("Authorization") != "Bearer private-fault-token" {
					http.Error(w, "invalid", 400)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "42", "version": 1, "title": "external issue", "body": "original content", "author": "claimed author", "time": "1901-01-01T00:00:00Z", "comments": []any{map[string]any{"id": "c1", "body": "external comment", "author": "claimed commenter", "time": "2099-01-01T00:00:00Z"}}})
			}))
			defer server.Close()
			dir := t.TempDir()
			config, plan, journal, report := filepath.Join(dir, "config"), filepath.Join(dir, "plan"), filepath.Join(dir, "journal"), filepath.Join(dir, "report")
			fakeWrite(t, config, map[string]any{"api": server.URL, "token": "private-fault-token", "connection": "fault-control", "source": "tracker:item:42", "journal": journal, "report": report})
			fakeWrite(t, plan, map[string]any{"action": "import"})
			identity := map[string]any{"protocol": "hn.extension/1", "request_id": "tracker-1", "ok": true, "data": map[string]any{"initialized": true, "actor": actor, "name": "Actual native signer", "public_key": base64.RawStdEncoding.EncodeToString(key)}, "error": nil}
			if fault == "terminal null ID" {
				identity["request_id"] = nil
				identity["ok"] = false
				identity["data"] = nil
				identity["error"] = map[string]any{"origin": "host", "code": "invalid_input", "message": "terminal frame refusal", "recovery": "do_not_retry", "operation": nil}
			}
			if fault == "wrong correlation" {
				identity["request_id"] = "tracker-other"
			}
			if fault == "wrong nested type" {
				identity["data"].(map[string]any)["public_key"] = 7
			}
			if fault == "inconsistent ok data error" {
				identity["ok"] = false
			}
			mutation := func(requestID, eventID string) map[string]any {
				return map[string]any{"protocol": "hn.extension/1", "request_id": requestID, "ok": true, "data": map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": id("a"), "event_id": eventID, "replayed": false, "conflict": false, "head_count": 1, "admission": nil, "source_cut": id("d")}, "error": nil}
			}
			input := append(frame(hello), frame(identity)...)
			input = append(input, frame(mutation("tracker-2", id("b")))...)
			input = append(input, frame(mutation("tracker-3", id("c")))...)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, consumer, "--config", config, "--plan", plan)
			cmd.Stdin = bytes.NewReader(input)
			var output, diagnostic bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &diagnostic
			err := cmd.Run()
			if ctx.Err() != nil {
				t.Fatal("supplied consumer did not finish bounded fault control", ctx.Err())
			}
			if fault == "valid complete responses" && err != nil {
				t.Fatal("valid full-response control failed", err, diagnostic.String())
			}
			if fault != "valid complete responses" && err == nil {
				t.Fatal("supplied binary accepted malformed/terminal response")
			}
			var frames []map[string]any
			wire := bytes.NewReader(output.Bytes())
			for wire.Len() > 0 {
				var prefix [4]byte
				if _, err := io.ReadFull(wire, prefix[:]); err != nil {
					t.Fatal(err)
				}
				n := binary.BigEndian.Uint32(prefix[:])
				if n == 0 || n > 8388608 {
					t.Fatal("invalid consumer output prefix", n)
				}
				raw := make([]byte, n)
				if _, err := io.ReadFull(wire, raw); err != nil {
					t.Fatal(err)
				}
				var v map[string]any
				if err := json.Unmarshal(raw, &v); err != nil {
					t.Fatal(err)
				}
				frames = append(frames, v)
			}
			wantFrames := 2
			if fault == "valid complete responses" {
				wantFrames = 4
			}
			if len(frames) != wantFrames || frames[0]["ready"] != true || frames[1]["method"] != "identity.public" {
				t.Fatal("consumer continued beyond response boundary or skipped the valid request", frames)
			}
			if fault == "valid complete responses" {
				if apiReads.Load() != 1 || frames[2]["method"] != "resource.mutate" || frames[3]["method"] != "resource.mutate" {
					t.Fatal("positive control did not exercise actual following native requests", apiReads.Load(), frames)
				}
				j := fakeRead(t, journal)
				if j["native"].(map[string]any)["comment"].(map[string]any)["result"].(map[string]any)["event_id"] != id("c") {
					t.Fatal("positive control failed to handle complete final response")
				}
			} else {
				if apiReads.Load() != 0 {
					t.Fatal("invalid response allowed an external read/effect", apiReads.Load())
				}
				if _, err := os.Stat(journal); !os.IsNotExist(err) {
					t.Fatal("invalid response persisted identity/native intent", err)
				}
				if diagnostic.String() != "fake tracker stopped: invalid_input\n" {
					t.Fatal("wrong sanitized fault cause", diagnostic.String())
				}
			}
			if base := os.Getenv("HN_WP20_EVIDENCE"); base != "" {
				destination := filepath.Join(base, t.Name())
				if err := os.MkdirAll(destination, 0700); err != nil {
					t.Fatal(err)
				}
				for name, raw := range map[string][]byte{"scripted-host-frames": input, "actual-consumer-stdout": output.Bytes(), "actual-consumer-stderr": diagnostic.Bytes()} {
					if err := os.WriteFile(filepath.Join(destination, name), raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
				binary, err := os.ReadFile(consumer)
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(binary)
				fakeWrite(t, filepath.Join(destination, "identity.json"), map[string]any{"consumer_sha256": hex.EncodeToString(sum[:]), "control": fault, "api_reads": apiReads.Load(), "output_frames": len(frames), "exit_success": cmd.ProcessState.Success()})
			}
		})
	}
}
