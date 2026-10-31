package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	c "hubnot/internal/contribution"
	"hubnot/internal/extension"
)

func nativeInvocation(t *testing.T, method string, params map[string]any) extension.Invocation {
	t.Helper()
	manifest, e := extension.ParseManifest([]byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.unit/native","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","resource.read","resource.mutate","resource.operation"]}`))
	if e != nil {
		t.Fatal(e)
	}
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	candidate, e := extension.NewRegistration(manifest, executable, []string{})
	if e != nil {
		t.Fatal(e)
	}
	registration, e := extension.Register(filepath.Join(t.TempDir(), "hubnot", "extensions", "registry.json"), candidate)
	if e != nil {
		t.Fatal(e)
	}
	repo, _, e := artifactRepositoryID()
	if e != nil {
		t.Fatal(e)
	}
	grants := []extension.Grant{{Selector: "native:identity", Actions: []string{"read"}}}
	for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"} {
		grants = append(grants, extension.Grant{Selector: "native:" + family, Actions: []string{"read", "create", "revise", "resolve"}})
	}
	epoch := strings.Repeat("a", 32)
	return extension.Invocation{Request: extension.Request{Method: method, Params: params}, Registration: registration, Binding: extension.Binding{RegistrationID: registration.RegistrationID, RepositoryID: repo, Epoch: epoch}, Enablement: extension.EnablementEntry{RegistrationID: registration.RegistrationID, RepositoryID: repo, Epoch: epoch, Enabled: true, Grants: grants}}
}
func TestExtensionNativeRepositoryAndPredispatchCancellation(t *testing.T) {
	repo := issueCommandTestRepo(t)
	identity, e := inspectIdentity()
	if e != nil {
		t.Fatal(e)
	}
	params := nativeMutation("issue", identity.Actor, "must-not-publish", nativeIssueState("Never signed"))["params"].(map[string]any)
	call := nativeInvocation(t, "resource.mutate", params)
	before := hostNativeSnapshot(t, repo)
	wrong := eventID([]byte("other physical repository"))
	call.Binding.RepositoryID = wrong
	call.Enablement.RepositoryID = wrong
	_, e = extensionHostDispatch(context.Background(), call)
	var public *extension.Error
	if !errors.As(e, &public) || public.Code != "denied_access" || public.Recovery != "do_not_retry" {
		t.Fatalf("physical binding %v", e)
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, repo)) {
		t.Fatal("wrong repository created signer/intent/object/ref effects")
	}
	call = nativeInvocation(t, "resource.mutate", params)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before = hostNativeSnapshot(t, repo)
	_, e = extensionHostDispatch(ctx, call)
	if !errors.As(e, &public) || public.Code != "cancelled" || public.Recovery != "do_not_retry" {
		t.Fatalf("predispatch cancel %v", e)
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, repo)) {
		t.Fatal("cancelled request created effects")
	}
	if _, e = executeIssueMutationContext(ctx, issueOptions{command: "open", title: "No effect"}); !errors.Is(e, context.Canceled) {
		t.Fatalf("issue Context %v", e)
	}
	if _, e = MutateResourceContext(ctx, ResourceMutation{Family: "project"}); !errors.Is(e, context.Canceled) {
		t.Fatalf("resource Context %v", e)
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, repo)) {
		t.Fatal("Context writer published after definitive cancellation")
	}
}
func TestExtensionNativeLegacyIdentityReadDoesNotMigrate(t *testing.T) {
	repo := inIdentityTestRepository(t)
	identity := testIdentity(t, "Legacy reader")
	writeLegacyIdentity(t, identity)
	before := hostNativeSnapshot(t, repo)
	value, e := extensionNativeIdentity()
	if e != nil {
		t.Fatal(e)
	}
	v := value.(map[string]any)
	if len(v) != 4 || v["actor"] != identity.Actor || v["public_key"] != identity.PublicKey || v["initialized"] != true {
		t.Fatalf("public identity %v", v)
	}
	if !reflect.DeepEqual(before, hostNativeSnapshot(t, repo)) {
		t.Fatal("identity public migrated legacy signer")
	}
}
func TestExtensionNativeContextPreservesSignedIntent(t *testing.T) {
	for _, family := range []string{"issue", "project"} {
		t.Run(family, func(t *testing.T) {
			issueCommandTestRepo(t)
			identity, e := inspectIdentity()
			if e != nil {
				t.Fatal(e)
			}
			kind := family + ".open"
			captured, e := nextEvent(identity, kind)
			if e != nil {
				t.Fatal(e)
			}
			wanted := captured
			wanted.Intent = "open"
			wanted.Operation = "immutable-intent"
			wanted.Parents = []string{}
			var actualID string
			if family == "issue" {
				state, _ := CanonicalIssueState(IssueState{Title: "Signed exact intent", Body: "preserved", Status: "open"})
				wanted.Issue = &state
				wanted.Title = state.Title
				wanted.Body = state.Body
				wanted.Request, e = IssueRequestDigest(wanted)
				if e != nil {
					t.Fatal(e)
				}
				r, e := applyIssueMutationContext(context.Background(), issueOptions{command: "open", actor: identity.Actor, operation: wanted.Operation, state: &state}, identity, captured, []StoredEvent{}, "")
				if e != nil {
					t.Fatal(e)
				}
				actualID = r.EventID
			} else {
				raw, e := CanonicalResourceState(family, []byte(`{"schema":"hn.project/1","title":"Signed exact intent","body":"preserved","archived":false}`))
				if e != nil {
					t.Fatal(e)
				}
				wanted.Body = string(raw)
				wanted.Request, e = ResourceRequestDigest(wanted)
				if e != nil {
					t.Fatal(e)
				}
				r, e := applyResourceMutationContext(context.Background(), ResourceMutation{Family: family, Intent: "open", Actor: identity.Actor, Operation: wanted.Operation, State: raw}, identity, captured, []StoredEvent{})
				if e != nil {
					t.Fatal(e)
				}
				actualID = r.EventID
			}
			payload, signature, e := encodeAndSign(wanted, identity)
			if e != nil {
				t.Fatal(e)
			}
			events, _, e := collectIssueEvents()
			if e != nil {
				t.Fatal(e)
			}
			if len(events) != 1 || actualID != eventID(payload) || !bytes.Equal(events[0].Payload, payload) || !bytes.Equal(events[0].Signature, signature) {
				t.Fatal("Context seam changed signed bytes or request digest")
			}
		})
	}
}
func independentNativeHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func TestExtensionNativeCutMatchesFrozenObject(t *testing.T) {
	issueCommandTestRepo(t)
	value, e := executeIssueMutation(issueOptions{command: "open", title: "Selected", operation: "selected"})
	if e != nil {
		t.Fatal(e)
	}
	id := value.(*issueMutationResult).EventID
	projection, e := extensionNativeProjectionFor(context.Background(), "issue")
	if e != nil {
		t.Fatal(e)
	}
	repo, _, e := artifactRepositoryID()
	if e != nil {
		t.Fatal(e)
	}
	raw, e := gitText("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes")
	if e != nil {
		t.Fatal(e)
	}
	refs := []map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		refs = append(refs, map[string]any{"ref": parts[0], "oid": parts[1]})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i]["ref"].(string) < refs[j]["ref"].(string) })
	view := independentNativeHash(map[string]any{"admitted_ids": []string{id}, "visible_ids": []string{id}, "retained_refs": []any{}})
	want := independentNativeHash(map[string]any{"repository_id": repo, "refs": refs, "selection": map[string]any{"families": []string{"issue"}, "types": []string{}}, "authority_commit": "", "policy_digest": "", "view_digest": view})
	if projection.cut != want {
		t.Fatalf("Cut used another shape/selection: %s != %s", projection.cut, want)
	}
	input := projection.cutInputs
	if input.RepositoryID != repo || input.AuthorityCommit != "" || input.PolicyDigest != "" || !reflect.DeepEqual(input.Selection.Families, []string{"issue"}) || input.Selection.Types == nil || len(input.Selection.Types) != 0 {
		t.Fatalf("retained Cut binding inputs changed: %+v", input)
	}
	if !reflect.DeepEqual(input.AdmittedIDs, []string{id}) || !reflect.DeepEqual(input.VisibleIDs, []string{id}) || input.RetainedRefs == nil || len(input.RetainedRefs) != 0 {
		t.Fatalf("retained Cut view inputs changed: %+v", input)
	}
	inputRefs := []map[string]any{}
	for _, ref := range input.Refs {
		inputRefs = append(inputRefs, map[string]any{"ref": ref.Ref, "oid": ref.OID})
	}
	if !reflect.DeepEqual(inputRefs, refs) {
		t.Fatalf("retained source witnesses differ: %v != %v", inputRefs, refs)
	}
}
func TestExtensionNativeErrorRecoveryClosedInventory(t *testing.T) {
	cases := []struct{ code, recovery string }{
		{"actor_mismatch", "do_not_retry"}, {"admission_pending", "inspect_operation"}, {"admission_refused", "do_not_retry"}, {"ambiguous_id", "do_not_retry"}, {"concurrent_update", "retry_identical"}, {"invalid_cursor", "rescan_required"}, {"invalid_history", "do_not_retry"}, {"invalid_input", "do_not_retry"}, {"issue_conflict", "do_not_retry"}, {"not_found", "do_not_retry"}, {"operation_conflict", "do_not_retry"}, {"permission_denied", "do_not_retry"}, {"repository_error", "retry_identical"}, {"resource_conflict", "do_not_retry"}, {"resource_limit", "do_not_retry"}, {"stale_cursor", "rescan_required"}, {"stale_revision", "do_not_retry"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			e := extensionNativeError(&issueCommandError{Code: c.code, Message: "native refusal"}, map[string]any{"actor": strings.Repeat("a", 64), "operation": "key"}).(*extension.Error)
			if e.Origin != "native" || e.Code != c.code || e.Recovery != c.recovery {
				t.Fatalf("recovery %v", e)
			}
			if (e.Operation != nil) != (c.recovery == "inspect_operation") {
				t.Fatalf("operation reference %v", e)
			}
		})
	}
	for _, message := range []string{"read /private/journal/key: permission denied", "PRIVATE\nRAW STDERR", strings.Repeat("x", 4097)} {
		e := extensionNativeError(issueFailure("repository_error", errors.New(message)), nil).(*extension.Error)
		if strings.Contains(e.Message, "PRIVATE") || strings.Contains(e.Message, "/private") || len(e.Message) > 4096 {
			t.Fatal("private diagnostic leaked")
		}
	}
	e := extensionNativeError(fmt.Errorf("plain private failure /secret/path"), nil).(*extension.Error)
	if e.Code != "repository_error" || strings.Contains(e.Message, "secret") {
		t.Fatal(e)
	}
}

func TestExtensionNativeGovernedCutInputsCLI(t *testing.T) {
	f := newNativeHostFixture(t, "native:issue=read,create")
	actor := f.actor()
	f.policy(actor, []c.Rule{{Operation: c.IssueCreate, Automatic: true}})
	opened := nativeOK(t, f.requests(nativeMutation("issue", actor, "retained-inputs", nativeIssueState("Typed inputs")))[0])
	id := opened["event_id"].(string)
	t.Chdir(f.repo)
	projection, e := extensionNativeProjectionFor(context.Background(), "issue")
	if e != nil {
		t.Fatal(e)
	}
	input := projection.cutInputs
	policy, e := os.ReadFile(filepath.Join(f.repo, ".hn", "policy.json"))
	if e != nil {
		t.Fatal(e)
	}
	policySum := sha256.Sum256(policy)
	if input.AuthorityCommit != f.git("rev-parse", "HEAD") || input.PolicyDigest != "sha256:"+hex.EncodeToString(policySum[:]) || !reflect.DeepEqual(input.AdmittedIDs, []string{id}) || !reflect.DeepEqual(input.VisibleIDs, []string{id}) || !reflect.DeepEqual(input.Selection.Families, []string{"issue"}) || len(input.Selection.Types) != 0 {
		t.Fatalf("verified governed inputs were not retained: %+v", input)
	}
	expectedRefs, expectedRetained := []extensionNativeRef{}, []extensionNativeRef{}
	project := sha256.Sum256([]byte("native-host-policy"))
	prefix := "refs/hn/contributions/" + hex.EncodeToString(project[:]) + "/"
	raw := f.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes", prefix)
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		ref := extensionNativeRef{Ref: parts[0], OID: parts[1]}
		if strings.HasPrefix(ref.Ref, "refs/hn/actors/") {
			expectedRefs = append(expectedRefs, ref)
		} else if strings.HasPrefix(ref.Ref, "refs/hn/remotes/") {
			expectedRefs = append(expectedRefs, ref)
			expectedRetained = append(expectedRetained, ref)
		} else {
			expectedRetained = append(expectedRetained, ref)
		}
	}
	sort.Slice(expectedRefs, func(i, j int) bool { return expectedRefs[i].Ref < expectedRefs[j].Ref })
	sort.Slice(expectedRetained, func(i, j int) bool { return expectedRetained[i].Ref < expectedRetained[j].Ref })
	if !reflect.DeepEqual(input.Refs, expectedRefs) || !reflect.DeepEqual(input.RetainedRefs, expectedRetained) || len(input.Refs) != 2 || len(input.RetainedRefs) != 2 {
		t.Fatalf("accepted/retained preimage inputs changed: %+v", input)
	}
	f.assertGovernedCut("issue", id, true, projection.cut)
	if projection.cut != opened["source_cut"] {
		t.Fatalf("typed input retention changed current wire Cut: %s != %v", projection.cut, opened["source_cut"])
	}
}
