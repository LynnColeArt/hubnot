package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func changesFixture(t *testing.T, grants ...string) *nativeHostFixture {
	t.Helper()
	f := newNativeHostFixture(t, "native:issue=read")
	f.command("extension", "remove", "example.native/tool", "--json")
	manifest := filepath.Join(f.home, "changes-manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.native/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["resource.read","resource.mutate","artifact.read","artifact.mutate","artifact.type.publish","changes.capture","changes.page","changes.since"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	child, _ := os.Executable()
	f.command("extension", "register", "--manifest", manifest, "--executable", child, "--arg", "-test.run=^TestExtensionChangesChild$", "--json")
	args := []string{"extension", "enable", "example.native/tool"}
	for _, grant := range grants {
		args = append(args, "--grant", grant)
	}
	f.command(append(args, "--json")...)
	f.env = append(f.env, "HN_WP19_CHILD=1", "HN_WP19_INPUT="+f.input, "HN_WP19_OUTPUT="+f.output)
	return f
}
func TestExtensionChangesRawEmptyCapture(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	r := changesRun(t, f, []map[string]any{nativeRequest("changes.capture", map[string]any{"selection": map[string]any{"families": []string{"issue"}, "types": []string{}}, "limit": 20})}, nil)[0]
	data := nativeOK(t, r)
	if data["complete"] != true || data["checkpoint"] == nil || data["next_cursor"] != nil || len(data["items"].([]any)) != 0 {
		t.Fatalf("empty capture must complete: %v", data)
	}
}

// This separately executed module uses handwritten raw frames and generic JSON.
// $N.field references a previous public response; no host token internals leak
// into the consumer. Files are private test coordination, never product bypasses.
func TestExtensionChangesChild(t *testing.T) {
	if os.Getenv("HN_WP19_CHILD") != "1" {
		return
	}
	hello := hostRead(os.Stdin)
	hostWrite(map[string]any{"protocol": "hn.extension/1", "ready": true})
	raw, e := os.ReadFile(os.Getenv("HN_WP19_INPUT"))
	if e != nil {
		os.Exit(90)
	}
	var script struct {
		Requests      []map[string]any `json:"requests"`
		Pauses        []int            `json:"pauses"`
		ConsumerState string           `json:"consumer_state"`
	}
	if json.Unmarshal(raw, &script) != nil {
		os.Exit(91)
	}
	responses := []map[string]any{}
	var resolve func(any) any
	resolve = func(v any) any {
		switch x := v.(type) {
		case string:
			if strings.HasPrefix(x, "$") {
				parts := strings.SplitN(x[1:], ".", 2)
				n, e := strconv.Atoi(parts[0])
				if e != nil || len(parts) != 2 || n >= len(responses) {
					os.Exit(92)
				}
				return responses[n]["data"].(map[string]any)[parts[1]]
			}
		case []any:
			for i := range x {
				x[i] = resolve(x[i])
			}
		case map[string]any:
			for k, item := range x {
				x[k] = resolve(item)
			}
		}
		return v
	}
	output := os.Getenv("HN_WP19_OUTPUT")
	durable := map[string]any{}
	if script.ConsumerState != "" {
		if b, e := os.ReadFile(script.ConsumerState); e == nil {
			if json.Unmarshal(b, &durable) != nil {
				os.Exit(97)
			}
		}
	}
	staging := map[string]any{}
	stagingSnapshot := ""
	for n, request := range script.Requests {
		request["request_id"] = fmt.Sprintf("change-%d", n)
		hostWrite(resolve(request))
		response := hostRead(os.Stdin)
		responses = append(responses, response)
		if script.ConsumerState != "" && strings.HasPrefix(request["method"].(string), "changes.") {
			if response["ok"] == true {
				data := response["data"].(map[string]any)
				snapshot := data["snapshot_id"].(string)
				if stagingSnapshot != snapshot {
					staging = map[string]any{}
					stagingSnapshot = snapshot
				}
				for _, value := range data["items"].([]any) {
					entry := value.(map[string]any)
					staging[entry["resource_id"].(string)] = entry
				}
				if data["complete"] == true && data["next_cursor"] == nil && data["checkpoint"] != nil {
					durable[data["selection_digest"].(string)] = map[string]any{"snapshot_id": snapshot, "checkpoint": data["checkpoint"], "entries": staging}
					committed, _ := json.Marshal(durable)
					if os.WriteFile(script.ConsumerState+".tmp", committed, 0600) != nil || os.Rename(script.ConsumerState+".tmp", script.ConsumerState) != nil {
						os.Exit(98)
					}
					staging = map[string]any{}
					stagingSnapshot = ""
				}
			} else {
				staging = map[string]any{}
				stagingSnapshot = ""
			}
		}
		raw, _ = json.Marshal(response)
		if os.WriteFile(fmt.Sprintf("%s.%d", output, n), raw, 0600) != nil {
			os.Exit(93)
		}
		if slicesContainsInt(script.Pauses, n) {
			deadline := time.Now().Add(11 * time.Minute)
			heartbeat := time.Now()
			count := 0
			for {
				if _, e = os.Stat(fmt.Sprintf("%s.continue-%d", output, n)); e == nil {
					break
				}
				if time.Now().After(deadline) {
					os.Exit(94)
				}
				if time.Since(heartbeat) > 20*time.Second {
					hostWrite(map[string]any{"protocol": "hn.extension/1", "request_id": fmt.Sprintf("keepalive-%d-%d", n, count), "method": "capabilities.list", "params": map[string]any{}})
					alive := hostRead(os.Stdin)
					if alive["ok"] != true {
						os.Exit(96)
					}
					count++
					heartbeat = time.Now()
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
	raw, _ = json.Marshal(map[string]any{"hello": hello, "requests": script.Requests, "responses": responses})
	if os.WriteFile(output, raw, 0600) != nil {
		os.Exit(95)
	}
	os.Exit(0)
}
func slicesContainsInt(values []int, n int) bool {
	for _, v := range values {
		if v == n {
			return true
		}
	}
	return false
}
func changesRun(t *testing.T, f *nativeHostFixture, requests []map[string]any, after map[int]func()) []map[string]any {
	t.Helper()
	pauses := []int{}
	for n := range after {
		pauses = append(pauses, n)
	}
	raw, _ := json.Marshal(map[string]any{"requests": requests, "pauses": pauses, "consumer_state": filepath.Join(f.home, "observer-state.json")})
	if err := os.WriteFile(f.input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for n := range requests {
		os.Remove(fmt.Sprintf("%s.%d", f.output, n))
		os.Remove(fmt.Sprintf("%s.continue-%d", f.output, n))
	}
	command := exec.Command(f.binary, "extension", "run", "example.native/tool", "--json")
	command.Dir = f.repo
	command.Env = f.env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if command.ProcessState == nil {
			command.Process.Kill()
			command.Wait()
		}
	}()
	for n := range requests {
		if callback := after[n]; callback != nil {
			deadline := time.Now().Add(15 * time.Second)
			for {
				if _, err := os.Stat(fmt.Sprintf("%s.%d", f.output, n)); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("consumer did not publish response", n)
				}
				time.Sleep(5 * time.Millisecond)
			}
			callback()
			if err := os.WriteFile(fmt.Sprintf("%s.continue-%d", f.output, n), []byte("continue"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	err := command.Wait()
	raw, e := os.ReadFile(f.output)
	if e != nil || err != nil {
		t.Fatalf("raw consumer: %v %v %s %s", err, e, &stdout, &stderr)
	}
	var result struct {
		Responses []map[string]any `json:"responses"`
	}
	if json.Unmarshal(raw, &result) != nil {
		t.Fatal("consumer JSON")
	}
	if evidence := os.Getenv("HN_WP19_EVIDENCE"); evidence != "" {
		out, e := os.CreateTemp(evidence, "raw-changes-*.json")
		if e != nil {
			t.Fatal(e)
		}
		out.Write(raw)
		out.Close()
	}
	return result.Responses
}
func changesSelection(families, types []string) map[string]any {
	return map[string]any{"families": families, "types": types}
}
func changesCapture(selection map[string]any, limit int) map[string]any {
	return nativeRequest("changes.capture", map[string]any{"selection": selection, "limit": limit})
}
func changesPage(capture, cursor any, limit int) map[string]any {
	return nativeRequest("changes.page", map[string]any{"capture": capture, "cursor": cursor, "limit": limit})
}
func changesSince(checkpoint any, selection map[string]any, limit int) map[string]any {
	return nativeRequest("changes.since", map[string]any{"checkpoint": checkpoint, "selection": selection, "limit": limit})
}
func changesIDs(t *testing.T, response map[string]any) []string {
	t.Helper()
	data := nativeOK(t, response)
	out := []string{}
	for _, entry := range data["items"].([]any) {
		out = append(out, entry.(map[string]any)["resource_id"].(string))
	}
	return out
}
func changesRefuse(t *testing.T, response map[string]any, code string) {
	t.Helper()
	nativeDenial(t, response, "host", code, map[string]string{"checkpoint_invalidated": "rescan_required", "verification_failure": "do_not_retry", "denied_access": "do_not_retry", "unsupported_capability": "do_not_retry", "resource_limit": "do_not_retry", "invalid_input": "do_not_retry"}[code])
}
func changesOpenIssue(f *nativeHostFixture, title string) string {
	f.t.Helper()
	return betaRecoveryID(f.t, f.native("issue", "open", "--json", title))
}
func TestExtensionChangesPagingAndDurableNativeAppend(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	ids := []string{changesOpenIssue(f, "first"), changesOpenIssue(f, "second"), changesOpenIssue(f, "third")}
	sort.Strings(ids)
	selection := changesSelection([]string{"issue"}, []string{})
	requests := []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 1), changesPage("$0.capture", "$0.next_cursor", 1), changesPage("$0.capture", "$0.next_cursor", 200)}
	responses := changesRun(t, f, requests, nil)
	first, second, duplicate, final := nativeOK(t, responses[0]), nativeOK(t, responses[1]), nativeOK(t, responses[2]), nativeOK(t, responses[3])
	if !reflect.DeepEqual(second, duplicate) {
		t.Fatal("duplicate page changed", second, duplicate)
	}
	for _, page := range []map[string]any{first, second, final} {
		if page["snapshot_id"] != first["snapshot_id"] || page["source_cut"] != first["source_cut"] || page["selection_digest"] != first["selection_digest"] {
			t.Fatal("mixed capture")
		}
	}
	if first["checkpoint"] != nil || first["complete"] != false || second["checkpoint"] != nil || final["checkpoint"] == nil || final["complete"] != true || final["next_cursor"] != nil {
		t.Fatal("completion contract", responses)
	}
	got := append(changesIDs(t, responses[0]), changesIDs(t, responses[3])...)
	if !reflect.DeepEqual(got, ids) {
		t.Fatal("variable limit skipped", got, ids)
	}
	// A distinct host session cannot resume a cursor, but a completed checkpoint
	// remains usable after an actual local append verified by the native reader.
	newID := changesOpenIssue(f, "append after restart")
	next := changesRun(t, f, []map[string]any{changesPage(first["capture"], first["next_cursor"], 20), changesSince(final["checkpoint"], selection, 200)}, nil)
	changesRefuse(t, next[0], "checkpoint_invalidated")
	appended := changesIDs(t, next[1])
	if len(appended) != 4 || !slicesContainsString(appended, newID) {
		t.Fatal("since must replace entire current set", next)
	}
	if nativeOK(t, next[1])["snapshot_id"] == first["snapshot_id"] {
		t.Fatal("append did not change whole snapshot")
	}
}
func slicesContainsString(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}
func TestExtensionChangesStalePageAndSafeRescan(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	changesOpenIssue(f, "original one")
	changesOpenIssue(f, "original two")
	selection := changesSelection([]string{"issue"}, []string{})
	committed := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	checkpoint := nativeOK(t, committed)["checkpoint"]
	prior := changesIDs(t, committed)
	priorState, err := os.ReadFile(filepath.Join(f.home, "observer-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	preserved := func() {
		current, e := os.ReadFile(filepath.Join(f.home, "observer-state.json"))
		if e != nil || !bytes.Equal(priorState, current) {
			t.Fatal("interrupted raw observer changed its durable visible set/checkpoint", e)
		}
	}
	responses := changesRun(t, f, []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 200), changesSince(checkpoint, selection, 200)}, map[int]func(){0: func() { preserved(); changesOpenIssue(f, "interleaved append") }, 1: preserved})
	changesRefuse(t, responses[1], "checkpoint_invalidated")
	if nativeOK(t, responses[0])["checkpoint"] != nil {
		t.Fatal("partial capture advanced checkpoint")
	}
	if len(prior) != 2 || checkpoint == nil {
		t.Fatal("previous committed set lost")
	}
	if len(changesIDs(t, responses[2])) != 3 {
		t.Fatal("rescan did not reconcile complete replacement")
	}
	state := changesConsumerSelection(t, f, nativeOK(t, responses[2]))
	if len(state["entries"].(map[string]any)) != 3 || state["checkpoint"] != nativeOK(t, responses[2])["checkpoint"] {
		t.Fatal("observer did not atomically persist completed replacement", state)
	}
}
func TestExtensionChangesCustomAndMixedAppend(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe", "native:project=read,observe", "artifact:example.test/*=read,observe")
	actor := f.actor()
	a := &artifactCLIFixture{t: t, binary: f.binary, repo: f.repo, home: f.home, actor: actor}
	schema := revisionEvent(a.publish(artifactStoreType, "type", ""))
	opening := revisionEvent(revisionCall(a, "open", revisionInput(t, nil, []string{}, revisionPayload(schema, "first custom")), "record-one", ""))
	issue := changesOpenIssue(f, "native alongside custom")
	a.publish(strings.ReplaceAll(artifactStoreType, "example.test/note", "example.unselected/note"), "unselected-type", "")
	selection := changesSelection([]string{"issue", "project"}, []string{"example.test/note"})
	first := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	if got := changesIDs(t, first); len(got) != 2 || !slicesContainsString(got, opening) || !slicesContainsString(got, issue) {
		t.Fatal("mixed capture", first)
	}
	changesAssertMixedCut(t, f, nativeOK(t, first), selection, []string{schema, opening, issue})
	revision := revisionEvent(revisionCall(a, "revise", revisionInput(t, opening, []string{opening}, revisionPayload(schema, "edited custom")), "record-revise", ""))
	second := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, first)["checkpoint"], selection, 200)}, nil)[0]
	if len(changesIDs(t, second)) != 2 {
		t.Fatal("custom append sparse/absent", second)
	}
	found := false
	for _, raw := range nativeOK(t, second)["items"].([]any) {
		entry := raw.(map[string]any)
		if entry["resource_id"] == opening {
			found = true
			if !reflect.DeepEqual(entry["revision_ids"], []any{revision}) {
				t.Fatal("custom revision identity", entry)
			}
		}
	}
	if !found {
		t.Fatal("missing custom opening")
	}
	// An exact selection cannot borrow a checkpoint from the mixed view.
	wrong := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, second)["checkpoint"], changesSelection([]string{"issue"}, []string{}), 200)}, nil)[0]
	changesRefuse(t, wrong, "checkpoint_invalidated")
}

func TestExtensionChangesAuthorizationAndVerifierLoss(t *testing.T) {
	selection := changesSelection([]string{"issue"}, []string{})
	for _, grant := range []string{"native:issue=read", "native:issue=observe"} {
		t.Run(grant, func(t *testing.T) {
			f := changesFixture(t, grant)
			before := hostNativeSnapshot(t, f.repo)
			r := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
			changesRefuse(t, r, "denied_access")
			if _, err := os.Stat(filepath.Join(f.repo, ".git", "hn", "extensions", "changes")); !os.IsNotExist(err) {
				t.Fatal("ungranted read initialized verifier")
			}
			if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
				t.Fatal("ungranted changes modified protected state")
			}
		})
	}
	f := changesFixture(t, "native:issue=read,observe")
	first := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
	checkpoint := nativeOK(t, first)["checkpoint"].(string)
	// A forged token cannot turn an incomplete or unrelated checkpoint into a view.
	forged := checkpoint[:128] + map[bool]string{true: "0", false: "1"}[checkpoint[128] != '0']
	responses := changesRun(t, f, []map[string]any{changesSince(forged, selection, 20), changesSince(strings.Repeat("x", 4096), selection, 20)}, nil)
	for _, r := range responses {
		changesRefuse(t, r, "checkpoint_invalidated")
	}
	// Re-enablement is an independent new epoch even with identical grants and
	// the same intact verifier. Then test missing verifier using a fresh checkpoint.
	f.command("extension", "enable", "example.native/tool", "--grant", "native:issue=read,observe", "--json")
	changesRefuse(t, changesRun(t, f, []map[string]any{changesSince(checkpoint, selection, 20)}, nil)[0], "checkpoint_invalidated")
	fresh := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
	checkpoint = nativeOK(t, fresh)["checkpoint"].(string)
	key := filepath.Join(f.repo, ".git", "hn", "extensions", "changes", "key")
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	before := hostNativeSnapshot(t, f.repo)
	changesRefuse(t, changesRun(t, f, []map[string]any{changesSince(checkpoint, selection, 20)}, nil)[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("since repaired missing verifier instead of invalidating")
	}

}
func TestExtensionChangesUnsupportedCustomPolicyBeforeState(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe", "artifact:example.test/*=read,observe")
	actor := f.actor()
	f.policy(actor, []c.Rule{{Operation: c.IssueCreate, Automatic: true}})
	before := hostNativeSnapshot(t, f.repo)
	r := changesRun(t, f, []map[string]any{changesCapture(changesSelection([]string{"issue"}, []string{"example.test/note"}), 20)}, nil)[0]
	changesRefuse(t, r, "unsupported_capability")
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("unsupported selected custom policy initialized state or modified history")
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git", "hn", "extensions", "changes")); !os.IsNotExist(err) {
		t.Fatal("unsupported custom view initialized verifier")
	}
}
func TestExtensionChangesInvalidHistoryAndQuarantine(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	actor := f.actor()
	visible := changesOpenIssue(f, "accepted body")
	head := f.git("rev-parse", "refs/hn/actors/"+actor)
	// A syntactically plausible raw quarantine ref is not an accepted feed.
	f.git("update-ref", "refs/hn/quarantine/not-an-admission/actors/"+actor, head)
	selection := changesSelection([]string{"issue"}, []string{})
	first := changesRun(t, f, []map[string]any{changesCapture(selection, 20)}, nil)[0]
	if !reflect.DeepEqual(changesIDs(t, first), []string{visible}) {
		t.Fatal("quarantine duplicated/promoted", first)
	}
	// A normal Git commit under an accepted actor ref is deliberately invalid
	// signed history. Capture must refuse, never return an apparently empty set.
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "not a signed event")
	f.git("update-ref", "refs/hn/actors/"+actor, f.git("rev-parse", "HEAD"))
	before := hostNativeSnapshot(t, f.repo)
	response := changesRun(t, f, []map[string]any{changesCapture(selection, 20), changesSince(nativeOK(t, first)["checkpoint"], selection, 20)}, nil)
	for _, r := range response {
		changesRefuse(t, r, "verification_failure")
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("failed verification modified history/private state")
	}
}
func TestExtensionChangesGovernedTwoActorVisibility(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	actor := f.actor()
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "Two actor fixture base")
	other := *f
	other.repo = t.TempDir()
	f.git("clone", "-q", "--no-local", f.repo, other.repo)
	secondActor := other.actor()
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor, secondActor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Contributions: &c.Policy{Project: eventID([]byte("native-host-policy")), Moderators: []string{actor}, Rules: []c.Rule{{Operation: c.IssueCreate, Automatic: true}, {Operation: c.IssueReply, Automatic: true}}, Baseline: []string{}, Limits: c.DefaultLimits()}}
	policyRaw, _ := json.Marshal(policy)
	policyPath := filepath.Join(f.repo, ".hn", "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, policyRaw, 0600); err != nil {
		t.Fatal(err)
	}
	f.git("add", ".hn/policy.json")
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Select two actor contribution policy")
	f.native("contribution", "authority", "set", "--ref", f.git("symbolic-ref", "HEAD"), "--expect", "absent")
	other.git("fetch", "-q", f.repo, f.git("symbolic-ref", "HEAD"))
	other.git("reset", "--hard", "FETCH_HEAD")
	other.native("contribution", "authority", "set", "--ref", f.git("symbolic-ref", "HEAD"), "--expect", "absent")
	ownID := changesOpenIssue(f, "Visible first actor")
	hiddenTitle := "PRIVATE-HIDDEN-WP19 second actor body"
	otherID := changesOpenIssue(&other, hiddenTitle)
	f.git("fetch", "-q", other.repo, "refs/hn/actors/"+secondActor+":refs/hn/actors/"+secondActor)
	var project string
	sum := sha256.Sum256([]byte("native-host-policy"))
	project = "sha256:" + hex.EncodeToString(sum[:])
	manifestRef := "refs/hn/contributions/" + project[7:] + "/" + secondActor
	f.git("fetch", "-q", other.repo, manifestRef+":"+manifestRef)
	var retained struct {
		Digest string `json:"digest"`
	}
	if json.Unmarshal([]byte(f.native("contribution", "retained", "show", "--json")), &retained) != nil || retained.Digest == "" {
		t.Fatal("retained inspection")
	}
	f.native("contribution", "retained", "register", "--expect", retained.Digest, "--json")
	selection := changesSelection([]string{"issue"}, []string{})
	initial := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	if got := changesIDs(t, initial); len(got) != 2 || !slicesContainsString(got, ownID) || !slicesContainsString(got, otherID) {
		t.Fatal("two governed actor views", initial)
	}
	var hidden map[string]any
	if json.Unmarshal([]byte(f.native("contribution", "decide", "hide", "--revision", otherID, "--expect-heads", "absent", "--json")), &hidden) != nil {
		t.Fatal("hide JSON")
	}
	if !strings.Contains(f.git("show", "refs/hn/actors/"+secondActor+":event.json"), hiddenTitle) {
		t.Fatal("raw hidden-event detector failed")
	}
	afterHide := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, initial)["checkpoint"], selection, 200), changesCapture(selection, 200)}, nil)
	changesRefuse(t, afterHide[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(changesIDs(t, afterHide[1]), []string{ownID}) {
		t.Fatal("hidden material entered normal feed", afterHide)
	}
	wire, _ := json.Marshal(afterHide)
	if bytes.Contains(wire, []byte(hiddenTitle)) || bytes.Contains(wire, []byte(otherID)) {
		t.Fatal("hidden raw identity/body leak")
	}
	f.native("contribution", "decide", "restore", "--revision", otherID, "--expect-heads", hidden["id"].(string), "--json")
	restored := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	if len(changesIDs(t, restored)) != 2 {
		t.Fatal("restore without edit omitted resource")
	}
	f.native("contribution", "decide", "block", "--actor", secondActor, "--expect-heads", "absent", "--json")
	blocked := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, restored)["checkpoint"], selection, 200), changesCapture(selection, 200)}, nil)
	changesRefuse(t, blocked[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(changesIDs(t, blocked[1]), []string{ownID}) {
		t.Fatal("blocked resource visible", blocked)
	}
	// An authority-only public policy edit invalidates the old checkpoint even
	// though neither resource's signed revision changed.
	f.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "Authority-only checkpoint transition")
	authority := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, blocked[1])["checkpoint"], selection, 200)}, nil)[0]
	changesRefuse(t, authority, "checkpoint_invalidated")
}

// Source-assisted fixture producer: signs real hn/0 events and writes their
// public Git commit trees with fast-import. All observations below are actual
// external raw host requests against those complete verified actor chains.
// This producer grants no product clock/limit bypass and does not touch private
// capture/checkpoint files.
func changesBulkProducer(t *testing.T, f *nativeHostFixture, bodies []string, comments ...string) ([]string, []map[string]any) {
	t.Helper()
	seed := bytes.Repeat([]byte{39}, 32)
	key := ed25519.NewKeyFromSeed(seed)
	public := key.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(public)
	identity := &Identity{Actor: hex.EncodeToString(sum[:]), Name: "WP19 source-assisted producer", PublicKey: base64.RawStdEncoding.EncodeToString(public), PrivateKey: base64.RawStdEncoding.EncodeToString(key)}
	ref := "refs/hn/actors/" + identity.Actor
	var stream bytes.Buffer
	fmt.Fprintf(&stream, "reset %s\nfrom %040d\n\n", ref, 0)
	previous := ""
	ids := []string{}
	entries := []map[string]any{}
	write := func(event Event) string {
		raw, signature, err := encodeAndSign(event, identity)
		if err != nil {
			t.Fatal(err)
		}
		id := eventID(raw)
		message := "hn event " + id
		fmt.Fprintf(&stream, "commit %s\nmark :%d\ncommitter Fixture <fixture@hn.invalid> 1793433600 +0000\ndata %d\n%s\n", ref, event.Sequence, len(message), message)
		if event.Sequence > 1 {
			fmt.Fprintf(&stream, "from :%d\n", event.Sequence-1)
		}
		fmt.Fprintf(&stream, "deleteall\nM 100644 inline event.json\ndata %d\n", len(raw))
		stream.Write(raw)
		stream.WriteByte('\n')
		sig := base64.RawStdEncoding.EncodeToString(signature)
		fmt.Fprintf(&stream, "M 100644 inline signature\ndata %d\n%s\n\n", len(sig), sig)
		previous = id
		return id
	}
	for n, body := range bodies {
		event := newEvent(identity, "issue.open", uint64(n+1), previous)
		event.Timestamp = "2026-10-31T08:00:00Z"
		event.Title = fmt.Sprintf("Bulk%05d", n)
		event.Body = body
		id := write(event)
		ids = append(ids, id)
		state := map[string]any{"title": event.Title, "body": body, "status": "open", "criteria": []any{}, "labels": []any{}, "assignees": []any{}, "relations": []any{}, "metadata": map[string]any{}}
		value := map[string]any{"protocol": "hn/0", "family": "issue", "id": id, "creator": identity.Actor, "heads": []any{map[string]any{"id": id, "actor": identity.Actor, "state": state}}, "conflict": false, "state": state, "comments": []any{}}
		entries = append(entries, map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": id, "revision_ids": []string{id}, "value": value})
	}
	sequence := len(bodies)
	for _, comment := range comments {
		if comment == "" {
			continue
		}
		sequence++
		event := newEvent(identity, "issue.comment", uint64(sequence), previous)
		event.Timestamp = "2026-10-31T08:00:00Z"
		event.Subject = ids[0]
		event.Body = comment
		id := write(event)
		value := entries[0]["value"].(map[string]any)
		value["comments"] = append(value["comments"].([]any), map[string]any{"id": id, "actor": identity.Actor, "body": comment})
		revisions := append(entries[0]["revision_ids"].([]string), id)
		sort.Strings(revisions)
		entries[0]["revision_ids"] = revisions
	}
	stream.WriteString("done\n")
	cmd := exec.Command("git", "fast-import", "--quiet", "--done", "--force")
	cmd.Dir = f.repo
	cmd.Env = f.env
	cmd.Stdin = &stream
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source-assisted fast-import: %v %s", err, raw)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i]["resource_id"].(string) < entries[j]["resource_id"].(string) })
	return ids, entries
}
func TestExtensionChangesRawEntryCapacity(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	bodies := make([]string, 10000)
	ids, expected := changesBulkProducer(t, f, bodies, "")
	expectedRaw, _ := json.Marshal(expected)
	t.Logf("source-assisted verified entry fixture:10000 entries C bytes=%d", len(expectedRaw))
	selection := changesSelection([]string{"issue"}, []string{})
	requests := []map[string]any{changesCapture(selection, 200)}
	for n := 1; n < 50; n++ {
		requests = append(requests, changesPage("$0.capture", fmt.Sprintf("$%d.next_cursor", n-1), 200))
	}
	responses := changesRun(t, f, requests, nil)
	seen := map[string]bool{}
	all := []any{}
	first := nativeOK(t, responses[0])
	for n, response := range responses {
		data := nativeOK(t, response)
		for _, entry := range data["items"].([]any) {
			id := entry.(map[string]any)["resource_id"].(string)
			if seen[id] {
				t.Fatal("duplicate/skipped capacity page", id)
			}
			seen[id] = true
			all = append(all, entry)
		}
		if data["snapshot_id"] != first["snapshot_id"] || data["source_cut"] != first["source_cut"] {
			t.Fatal("capacity page mixed cut")
		}
		if n < 49 && (data["complete"] != false || data["checkpoint"] != nil) {
			t.Fatal("early capacity checkpoint")
		}
	}
	if len(seen) != len(ids) || nativeOK(t, responses[49])["complete"] != true || nativeOK(t, responses[49])["checkpoint"] == nil {
		t.Fatal("10000-entry replacement not complete", len(seen))
	}
	actualRaw, _ := json.Marshal(all)
	if !bytes.Equal(actualRaw, expectedRaw) {
		t.Fatal("complete array differs from independently assembled verified fixture")
	}
	wantSnapshot := independentNativeHash(map[string]any{"source_cut": first["source_cut"], "selection_digest": first["selection_digest"], "entries": all})
	if first["snapshot_id"] != wantSnapshot {
		t.Fatal("snapshot ID omitted whole entry set")
	}
	bodies = append(bodies, "")
	changesBulkProducer(t, f, bodies, "")
	before := hostNativeSnapshot(t, f.repo)
	refused := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	changesRefuse(t, refused, "resource_limit")
	message := refused["error"].(map[string]any)["message"].(string)
	if !strings.Contains(message, "selections") || strings.Contains(message, "smaller page") {
		t.Fatal("capture capacity gives false page guidance", message)
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("10001-entry capture published private state")
	}
}
func TestExtensionChangesRawCanonicalCapacity(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	//128*two complete states leave a small adjustable tail below16MiB.
	bodies := make([]string, 128)
	for i := range bodies {
		bodies[i] = strings.Repeat("x", 64000)
	}
	_, entries := changesBulkProducer(t, f, bodies, "")
	base, _ := json.Marshal(entries)
	// A visible comment adds a single-copy text field, allowing exact +1 bytes
	// while each individual state/comment remains within its own native bounds.
	_, entries = changesBulkProducer(t, f, bodies, "a")
	withComment, _ := json.Marshal(entries)
	remaining := 16777216 - len(withComment)
	// Increase body bytes in pairs first, leaving one comment for odd precision.
	for n := 0; n < len(bodies) && remaining > 65534; n++ {
		add := min(1536, remaining/2)
		bodies[n] += strings.Repeat("x", add)
		remaining -= 2 * add
	}
	if remaining < 0 || remaining > 65534 {
		t.Fatalf("fixture adjustment cannot reach exact capacity:base=%d tail=%d", len(base), remaining)
	}
	comment := strings.Repeat("c", remaining+1)
	_, entries = changesBulkProducer(t, f, bodies, comment)
	exact, _ := json.Marshal(entries)
	if len(exact) != 16777216 {
		t.Fatal("source-assisted at-bound C length", len(exact))
	}
	t.Logf("source-assisted verified full C array at=%d over=%d", len(exact), len(exact)+1)
	selection := changesSelection([]string{"issue"}, []string{})
	r := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	if nativeOK(t, r)["complete"] != false || len(changesIDs(t, r)) != 1 {
		t.Fatal("16MiB whole capture refused/truncated")
	}
	changesBulkProducer(t, f, bodies, comment+"c")
	before := hostNativeSnapshot(t, f.repo)
	refused := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	changesRefuse(t, refused, "resource_limit")
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("over-capacity initialized new state")
	}
}

func TestExtensionChangesRealTenMinuteExpiry(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	changesOpenIssue(f, "expiry one")
	changesOpenIssue(f, "expiry two")
	selection := changesSelection([]string{"issue"}, []string{})
	var expires time.Time
	var began time.Time
	responses := changesRun(t, f, []map[string]any{changesCapture(selection, 1), changesPage("$0.capture", "$0.next_cursor", 1), changesPage("$0.capture", "$0.next_cursor", 1)}, map[int]func(){
		0: func() {
			raw, err := os.ReadFile(f.output + ".0")
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			if json.Unmarshal(raw, &response) != nil {
				t.Fatal("expiry response")
			}
			data := nativeOK(t, response)
			expires, err = time.Parse("2006-01-02T15:04:05Z", data["expires_at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			began = time.Now()
			if until := time.Until(expires); until < 598*time.Second || until > 600*time.Second {
				t.Fatalf("not a real ten-minute fresh capture: %v", until)
			}
			t.Logf("genuine live clock capture accepted at%s; advertised expiry%s", began.UTC().Format(time.RFC3339Nano), expires.Format(time.RFC3339))
			deadline := expires.Add(-time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(min(30*time.Second, time.Until(deadline)))
				t.Logf("retained-session wall clock elapsed%v", time.Since(began))
			}
		},
		1: func() {
			raw, err := os.ReadFile(f.output + ".1")
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			if json.Unmarshal(raw, &response) != nil {
				t.Fatal("before-expiry response")
			}
			if nativeOK(t, response)["complete"] != true {
				t.Fatal("page failed while still valid")
			}
			time.Sleep(max(0, time.Until(expires.Add(time.Second))))
		},
	})
	changesRefuse(t, responses[2], "checkpoint_invalidated")
	t.Logf("genuine live expiry invalidated after%v; no HMAC corruption or product test clock", time.Since(began))
}

func TestExtensionChangesRawWireBoundary(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	comments := make([]string, 127)
	for i := range comments {
		comments[i] = strings.Repeat("c", 65000)
	}
	comments = append(comments, "c")
	_, entries := changesBulkProducer(t, f, []string{""}, comments...)
	selection := changesSelection([]string{"issue"}, []string{})
	// All dynamic host IDs/tokens have fixed grammar lengths; calculate the exact
	// full ordinary wire JSON frame independently, including wrapper/checkpoint.
	id := "sha256:" + strings.Repeat("a", 64)
	page := map[string]any{"capture": strings.Repeat("b", 64), "snapshot_id": id, "selection": selection, "selection_digest": id, "source_cut": id, "mode": "replace", "items": entries, "next_cursor": nil, "complete": true, "checkpoint": strings.Repeat("d", 129), "expires_at": "2026-10-31T08:10:00Z"}
	wrapper := map[string]any{"protocol": "hn.extension/1", "request_id": "change-0", "ok": true, "data": page, "error": nil}
	raw, _ := json.Marshal(wrapper)
	remaining := 8388608 - len(raw)
	for n := 0; n < 127 && remaining > 65534; n++ {
		add := min(536, remaining-65534)
		comments[n] += strings.Repeat("c", add)
		remaining -= add
	}
	if remaining < 0 || remaining > 65534 {
		t.Fatal("wire adjustment outside native text bound", remaining)
	}
	comments[len(comments)-1] = strings.Repeat("c", remaining+1)
	_, entries = changesBulkProducer(t, f, []string{""}, comments...)
	page["items"] = entries
	raw, _ = json.Marshal(wrapper)
	if len(raw) != 8388608 {
		t.Fatal("independent wire length", len(raw))
	}
	t.Logf("source-assisted complete single state full wire at=%d over=%d", len(raw), len(raw)+1)
	response := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	if nativeOK(t, response)["complete"] != true {
		t.Fatal("at-frame complete state refused")
	}
	actual, _ := json.Marshal(response)
	if len(actual) != 8388608 {
		t.Fatal("actual complete response length", len(actual))
	}
	comments[len(comments)-1] += "c"
	changesBulkProducer(t, f, []string{""}, comments...)
	before := hostNativeSnapshot(t, f.repo)
	refusal := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	changesRefuse(t, refusal, "resource_limit")
	if !strings.Contains(refusal["error"].(map[string]any)["message"].(string), "export") {
		t.Fatal("oversized single state falsely promises smaller page")
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
		t.Fatal("oversized wire created capture/checkpoint state")
	}
}
func TestExtensionChangesRawTypedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		request    map[string]any
	}{
		{"zero-page", "invalid_input", changesCapture(changesSelection([]string{"issue"}, []string{}), 0)},
		{"over-page", "resource_limit", changesCapture(changesSelection([]string{"issue"}, []string{}), 201)},
		{"no-implicit-all", "invalid_input", changesCapture(changesSelection([]string{}, []string{}), 20)},
		{"capture-token-over", "resource_limit", changesPage(strings.Repeat("x", 4097), "cursor", 20)},
		{"cursor-over", "resource_limit", changesPage("capture", strings.Repeat("x", 4097), 20)},
		{"checkpoint-over", "resource_limit", changesSince(strings.Repeat("x", 4097), changesSelection([]string{"issue"}, []string{}), 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := changesFixture(t, "native:issue=read,observe")
			script, _ := json.Marshal(map[string]any{"requests": []any{tc.request}, "pauses": []int{}})
			if err := os.WriteFile(f.input, script, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(f.binary, "extension", "run", "example.native/tool", "--json")
			cmd.Dir = f.repo
			cmd.Env = f.env
			before := hostNativeSnapshot(t, f.repo)
			stdout, err := cmd.Output()
			if err == nil {
				t.Fatal("malformed raw host request did not terminate session")
			}
			var parent map[string]any
			if json.Unmarshal(stdout, &parent) != nil {
				t.Fatalf("parent management output %s", stdout)
			}
			if parent["error"].(map[string]any)["code"] != tc.code {
				t.Fatal("wrong typed boundary cause", parent)
			}
			// The rejected typed request is observed at the parent management
			// boundary; startup/session failure may close the child before a
			// terminal wire response is delivered. No business adapter ran.
			if evidence := os.Getenv("HN_WP19_EVIDENCE"); evidence != "" {
				file, e := os.CreateTemp(evidence, "typed-boundary-*.json")
				if e != nil {
					t.Fatal(e)
				}
				out, _ := json.Marshal(map[string]any{"request": tc.request, "parent": parent})
				file.Write(out)
				file.Close()
			}
			if !reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
				t.Fatal("typed refusal wrote private/native state")
			}
		})
	}
}

func TestExtensionChangesNativeDefaultRemainsInert(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	f.actor()
	id := changesOpenIssue(f, "ordinary native setup")
	registry := filepath.Join(f.home, "config", "hubnot", "extensions", "registry.json")
	if _, err := os.Stat(registry); err != nil {
		t.Fatal("actual registry trap unavailable", err)
	}
	if err := os.WriteFile(registry, []byte("{ deliberately malformed registry"), 0600); err != nil {
		t.Fatal(err)
	}
	before := hostNativeSnapshot(t, f.repo)
	child, _ := os.Executable()
	trace := extensionBaselineTraceRun(t, f.env, f.repo, f.binary, "issue", "show", id, "--json")
	if cause := extensionBaselineViolation(trace, registry, child, before, hostNativeSnapshot(t, f.repo)); cause != "" {
		t.Fatal("ordinary native command gained extension work", cause)
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git", "hn", "extensions", "changes")); !os.IsNotExist(err) {
		t.Fatal("ordinary native read initialized verifier")
	}
	// Prove the actual detector recognizes a deliberately opened registry path.
	if *extensionBaselineTrace != "" {
		actual, err := os.ReadFile(registry)
		if err != nil || len(actual) == 0 {
			t.Fatal(err)
		}
		command := exec.Command(*extensionBaselineTrace, "-f", "-qq", "-s", "4096", "-e", "trace=openat,execve,connect", "-o", filepath.Join(f.home, "negative.trace"), "cat", registry)
		command.Env = f.env
		command.Dir = f.repo
		if _, err = command.Output(); err != nil {
			t.Fatal("detector negative control", err)
		}
		raw, err := os.ReadFile(filepath.Join(f.home, "negative.trace"))
		if err != nil {
			t.Fatal(err)
		}
		if cause := extensionBaselineViolation(string(raw), registry, child, before, before); cause != "registry-read" {
			t.Fatal("actual registry-read negative control missed", cause)
		}
	}
}

func TestExtensionChangesVerifiedHistoryRewindInvalidates(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	actor := f.actor()
	firstID := changesOpenIssue(f, "retained prefix")
	changesOpenIssue(f, "later accepted event")
	selection := changesSelection([]string{"issue"}, []string{})
	original := changesRun(t, f, []map[string]any{changesCapture(selection, 200)}, nil)[0]
	ref := "refs/hn/actors/" + actor
	f.git("update-ref", ref, f.git("rev-parse", ref+"^"))
	// The remaining prefix is itself valid. This is loss of the old checkpoint's
	// authenticated accepted evidence, rather than corrupt-signature detection.
	responses := changesRun(t, f, []map[string]any{changesSince(nativeOK(t, original)["checkpoint"], selection, 200), changesCapture(selection, 200)}, nil)
	changesRefuse(t, responses[0], "checkpoint_invalidated")
	if !reflect.DeepEqual(changesIDs(t, responses[1]), []string{firstID}) {
		t.Fatal("fresh verified prefix capture", responses)
	}
}

func TestExtensionChangesAllVisibleCommentRevisions(t *testing.T) {
	f := changesFixture(t, "native:issue=read,observe")
	comments := make([]string, 1023)
	for i := range comments {
		comments[i] = "visible comment"
	}
	selection := changesSelection([]string{"issue"}, []string{})
	_, entries := changesBulkProducer(t, f, []string{""}, comments...)
	at := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	entry := nativeOK(t, at)["items"].([]any)[0].(map[string]any)
	if len(entry["revision_ids"].([]any)) != 1024 {
		t.Fatal("at1024 revisions", entry)
	}
	comments = append(comments, "one more visible comment")
	_, entries = changesBulkProducer(t, f, []string{""}, comments...)
	expected, _ := json.Marshal(entries)
	t.Logf("source-assisted1025 visible revisions full C entry bytes=%d, below published capture/frame bounds", len(expected))
	over := changesRun(t, f, []map[string]any{changesCapture(selection, 1)}, nil)[0]
	entry = nativeOK(t, over)["items"].([]any)[0].(map[string]any)
	if len(entry["revision_ids"].([]any)) != 1025 || len(entry["value"].(map[string]any)["comments"].([]any)) != 1024 {
		t.Fatal("visible comments/revision identities truncated")
	}
}

func changesConsumerSelection(t *testing.T, f *nativeHostFixture, page map[string]any) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.home, "observer-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if json.Unmarshal(raw, &state) != nil {
		t.Fatal("observer state")
	}
	selected, ok := state[page["selection_digest"].(string)].(map[string]any)
	if !ok {
		t.Fatal("observer omitted completed selection")
	}
	return selected
}
func changesAssertMixedCut(t *testing.T, f *nativeHostFixture, page, selection map[string]any, ids []string) {
	t.Helper()
	refs := []map[string]any{}
	for _, line := range strings.Split(f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors/", "refs/hn/artifacts/"), "\n") {
		row := strings.Fields(line)
		if len(row) != 2 {
			t.Fatal("witness fixture", line)
		}
		refs = append(refs, map[string]any{"ref": row[0], "oid": row[1]})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i]["ref"].(string) < refs[j]["ref"].(string) })
	sort.Strings(ids)
	view := independentNativeHash(map[string]any{"admitted_ids": ids, "visible_ids": ids, "retained_refs": []any{}})
	common, err := filepath.EvalSymlinks(filepath.Join(f.repo, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("hn-extension-repository-v1\x00" + common))
	expected := independentNativeHash(map[string]any{"repository_id": "sha256:" + hex.EncodeToString(sum[:]), "refs": refs, "selection": selection, "authority_commit": "", "policy_digest": "", "view_digest": view})
	if page["source_cut"] != expected {
		t.Fatal("mixed selected Cut differs from independent frozen preimage", page["source_cut"], expected)
	}
}
