package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"hubnot/internal/forge"
)

type forgeExportFake struct {
	issues                                              map[string]forge.Issue
	comments                                            map[string][]forge.Comment
	creates, updates, commentCreates                    int
	lostCreate, absentCreate, lostComment, ignoreLabels bool
	afterCreate                                         func()
}

func newForgeExportFake(t *testing.T) *forgeExportFake {
	t.Helper()
	f := &forgeExportFake{issues: map[string]forge.Issue{}, comments: map[string][]forge.Comment{}}
	old, existed := forgeProviders["other"]
	forgeProviders["other"] = forgeRegistration{New: func(string, string) (forge.Provider, error) { return f, nil }, TokenEnv: "HN_OTHER_TOKEN", Capabilities: []forge.Capability{{Resource: forge.ResourceIssue, Import: true, Export: true}}}
	t.Cleanup(func() {
		if existed {
			forgeProviders["other"] = old
		} else {
			delete(forgeProviders, "other")
		}
	})
	t.Setenv("HN_OTHER_TOKEN", "fixture-secret")
	return f
}
func (*forgeExportFake) Kind() string     { return "other" }
func (*forgeExportFake) Endpoint() string { return "https://other.example/api" }
func (f *forgeExportFake) GetIssue(_ context.Context, project, key string) (forge.Issue, error) {
	v, ok := f.issues[key]
	if !ok {
		return v, errors.New("missing external issue")
	}
	return v, nil
}
func (f *forgeExportFake) ListIssues(context.Context, string) ([]forge.Issue, error) {
	keys := []string{}
	for k := range f.issues {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := []forge.Issue{}
	for _, k := range keys {
		rows = append(rows, f.issues[k])
	}
	return rows, nil
}
func (f *forgeExportFake) ListComments(_ context.Context, project, key string) ([]forge.Comment, error) {
	return append([]forge.Comment{}, f.comments[key]...), nil
}
func (f *forgeExportFake) CreateIssue(_ context.Context, project string, input forge.IssueInput) (forge.Issue, error) {
	f.creates++
	if f.absentCreate {
		return forge.Issue{}, errors.New("lost write fixture-secret")
	}
	key := fmt.Sprint(len(f.issues) + 1)
	v := forge.Issue{ID: "issue:" + key, Key: key, Title: input.Title, Body: input.Body, State: "open", Labels: append([]string{}, input.Labels...), Assignees: append([]string{}, input.Assignees...), Author: "publisher", URL: "https://other.example/issues/" + key, CreatedAt: "2026-10-25T01:00:00Z", UpdatedAt: "2026-10-25T01:00:00Z"}
	f.issues[key] = v
	if f.afterCreate != nil {
		f.afterCreate()
	}
	if f.lostCreate {
		f.lostCreate = false
		return forge.Issue{}, errors.New("lost response fixture-secret")
	}
	return v, nil
}
func (f *forgeExportFake) UpdateIssue(_ context.Context, project, key string, input forge.IssueInput) (forge.Issue, error) {
	f.updates++
	v, ok := f.issues[key]
	if !ok {
		return v, errors.New("missing")
	}
	v.Title = input.Title
	v.Body = input.Body
	v.State = input.State
	v.Assignees = append([]string{}, input.Assignees...)
	if !f.ignoreLabels {
		v.Labels = append([]string{}, input.Labels...)
	}
	f.issues[key] = v
	return v, nil
}
func (f *forgeExportFake) CreateComment(_ context.Context, project, key, body string) (forge.Comment, error) {
	f.commentCreates++
	v := forge.Comment{ID: fmt.Sprintf("comment:%d", f.commentCreates), Body: body, Author: "publisher", URL: "https://other.example/comment", CreatedAt: "2026-10-25T01:00:00Z", UpdatedAt: "2026-10-25T01:00:00Z"}
	f.comments[key] = append(f.comments[key], v)
	if f.lostComment {
		f.lostComment = false
		return forge.Comment{}, errors.New("lost comment fixture-secret")
	}
	return v, nil
}
func forgeExportNativeOpen(t *testing.T, state IssueState) string {
	t.Helper()
	if state.Status == "" {
		state.Status = "open"
	}
	canonical, err := CanonicalIssueState(state)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := executeIssueMutation(issueOptions{command: "open", state: &canonical})
	if err != nil {
		t.Fatal(err)
	}
	return raw.(*issueMutationResult).IssueID
}
func forgeExportNativeEdit(t *testing.T, id string, edit func(*IssueState)) {
	t.Helper()
	view, err := ghReadIssue(id)
	if err != nil {
		t.Fatal(err)
	}
	state := *view.State
	edit(&state)
	_, err = executeIssueMutation(issueOptions{command: "revise", id: id, expected: view.HeadIDs, state: &state})
	if err != nil {
		t.Fatal(err)
	}
}
func forgeExportNativeComment(t *testing.T, id, body string) string {
	t.Helper()
	raw, err := executeIssueMutation(issueOptions{command: "comment", id: id, body: body})
	if err != nil {
		t.Fatal(err)
	}
	return raw.(*issueMutationResult).EventID
}
func forgeExportTestRun(t *testing.T, id string, extra ...string) (string, error) {
	t.Helper()
	args := append([]string{"export", id, "--provider", "other", "--project", "team/project"}, extra...)
	return captureTestOutput(t, func() error { return cmdTransfer(args) })
}

func TestForgeExportLifecycleRepeatAndNativeExtras(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	id := forgeExportNativeOpen(t, IssueState{Title: "Native", Body: "description", Status: "closed", Labels: []string{"bug"}, Assignees: []string{"andrew"}, Criteria: []IssueCriterion{{ID: "done", Text: "must remain"}}, Metadata: map[string]string{"team/priority": "high"}})
	forgeExportNativeComment(t, id, "native comment")
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if out, err := forgeExportTestRun(t, id); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if f.creates != 1 || f.updates != 1 || f.commentCreates != 1 || f.issues["1"].State != "closed" {
		t.Fatalf("writes %d/%d/%d issue=%v", f.creates, f.updates, f.commentCreates, f.issues["1"])
	}
	if !strings.Contains(f.issues["1"].Body, "must remain") || !strings.Contains(f.issues["1"].Body, "team/priority") {
		t.Fatal("native-only information silently dropped")
	}
	if _, _, present := forgeParseIssueExport(f.issues["1"].Body); !present {
		t.Fatal("missing valid native export proof")
	}
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	if f.creates != 1 || f.updates != 1 || f.commentCreates != 1 {
		t.Fatal("repeat duplicated remote writes")
	}
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); after != before {
		t.Fatal("export changed native signed data")
	}
}

func TestForgeExportDryRunAndPreflightFailureDoNotWrite(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	t.Setenv("HN_OTHER_TOKEN", "")
	id := forgeExportNativeOpen(t, IssueState{Title: "preview", Body: "body"})
	if _, err := forgeExportTestRun(t, id, "--dry-run"); err != nil {
		t.Fatal(err)
	}
	dir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	if _, err := os.Stat(filepath.Join(dir, "hn", "forge")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created recovery data: %v", err)
	}
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("missing write credential accepted")
	}
	t.Setenv("HN_OTHER_TOKEN", "fixture-secret")
	large := forgeExportNativeOpen(t, IssueState{Title: "large", Body: strings.Repeat("x", 65536)})
	if _, err := forgeExportTestRun(t, large); err == nil {
		t.Fatal("oversized attributed payload accepted")
	}
	if f.creates+f.updates+f.commentCreates != 0 {
		t.Fatal("preflight or dry-run performed writes")
	}
}

func TestForgeExportLostCreateReconcilesOriginalSnapshot(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	f.lostCreate = true
	id := forgeExportNativeOpen(t, IssueState{Title: "original", Body: "body"})
	if _, err := forgeExportTestRun(t, id); err == nil || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("lost response: %v", err)
	}
	forgeExportNativeEdit(t, id, func(s *IssueState) { s.Title = "later native edit" })
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	if f.creates != 1 || f.updates != 0 || f.issues["1"].Title != "original" {
		t.Fatal("recovery replaced original exact snapshot")
	}
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	if f.updates != 1 || f.issues["1"].Title != "later native edit" {
		t.Fatal("next explicit export did not publish later edit")
	}
}

func TestForgeExportUncertainNegativeScanNeverRepeatsPOST(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	f.absentCreate = true
	id := forgeExportNativeOpen(t, IssueState{Title: "unknown"})
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("expected uncertain create")
	}
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("empty scan treated as safe retry")
	}
	if f.creates != 1 {
		t.Fatal("blind POST retry")
	}
}

func TestForgeExportLostCommentAndDeletedPublishedComment(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	f.lostComment = true
	id := forgeExportNativeOpen(t, IssueState{Title: "comments"})
	forgeExportNativeComment(t, id, "one")
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("expected lost comment response")
	}
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	if f.commentCreates != 1 {
		t.Fatal("duplicated uncertain comment")
	}
	f.comments["1"] = nil
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	if f.commentCreates != 1 {
		t.Fatal("recreated externally deleted comment")
	}
}

func TestForgeExportRemoteOnlyEditsDoNotBecomeNewBaseline(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	id := forgeExportNativeOpen(t, IssueState{Title: "initial"})
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	v := f.issues["1"]
	v.Title = "remote edit"
	f.issues["1"] = v
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	if f.updates != 0 || f.issues["1"].Title != "remote edit" {
		t.Fatal("remote edit overwritten")
	}
	forgeExportNativeEdit(t, id, func(s *IssueState) { s.Title = "local edit" })
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("competing edits silently accepted")
	}
	if f.updates != 0 {
		t.Fatal("competing edits wrote")
	}
}

func TestForgeExportSourceRoundTripNoBodiesOrCommentEcho(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	f.issues["7"] = forge.Issue{ID: "original:7", Key: "7", Title: "source", Body: "ordinary body", State: "open", Author: "andrew", URL: "https://other.example/issues/7", CreatedAt: "2026-10-25T01:00:00Z", UpdatedAt: "2026-10-25T01:00:00Z"}
	f.comments["7"] = []forge.Comment{{ID: "source-comment:1", Body: "original comment", Author: "alice", URL: "https://other.example/comments/1"}}
	if _, err := captureTestOutput(t, func() error {
		return forgeImport(context.Background(), f, transferOptions{command: "import", provider: f.Kind(), project: "team/project", endpoint: f.Endpoint(), key: "7"})
	}); err != nil {
		t.Fatal(err)
	}
	items, err := forgeNativeIssues()
	if err != nil || len(items) != 1 {
		t.Fatalf("items %v %v", items, err)
	}
	id := items[0].ID
	forgeExportNativeEdit(t, id, func(s *IssueState) { s.Title = "local source edit" })
	forgeExportNativeComment(t, id, "a new native comment")
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	if f.creates != 0 || f.updates != 1 || f.commentCreates != 1 || f.issues["7"].Body != "ordinary body" {
		t.Fatalf("source roundtrip writes=%d/%d/%d body=%q", f.creates, f.updates, f.commentCreates, f.issues["7"].Body)
	}
	before, err := forgeNativeComments(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := captureTestOutput(t, func() error {
		return forgeImport(context.Background(), f, transferOptions{command: "import", provider: f.Kind(), project: "team/project", endpoint: f.Endpoint(), key: "7"})
	}); err != nil {
		t.Fatal(err)
	}
	after, err := forgeNativeComments(id)
	if err != nil || len(after) != len(before) {
		t.Fatalf("echoed comments before=%d after=%d err=%v", len(before), len(after), err)
	}
}

func TestForgeExportIgnoresOrdinaryMarkerProseAndRejectsDuplicateClaims(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	id := forgeExportNativeOpen(t, IssueState{Title: "claims", Body: "literal <!-- hn:forge-issue: prose"})
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	v := f.issues["1"]
	v.ID = "duplicate:2"
	v.Key = "2"
	f.issues["2"] = v
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("duplicate native issue claims selected arbitrarily")
	}
	if f.creates != 1 || f.updates != 0 {
		t.Fatal("ambiguous mapping wrote")
	}
}

func TestForgeExportPendingPayloadCannotReplaceSignedSnapshot(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	f.lostComment = true
	id := forgeExportNativeOpen(t, IssueState{Title: "checkpoint"})
	forgeExportNativeComment(t, id, "actual signed content")
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("expected uncertain comment")
	}
	o := transferOptions{id: id, provider: f.Kind(), endpoint: f.Endpoint(), project: "team/project"}
	name := forgeExportCheckpointName(forgeExportScope(f, o))
	var cp forgeIssueExportCheckpoint
	if found, err := forgeLoadJSON(name, &cp); err != nil || !found || cp.Pending == nil {
		t.Fatalf("checkpoint %v %v", found, err)
	}
	var payload struct{ NativeID, Body string }
	if err := forgeStrictJSON(cp.Pending.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload.Body = "changed recovery-file content"
	cp.Pending.Payload, _ = json.Marshal(payload)
	cp.Pending.PayloadDigest = eventID(cp.Pending.Payload)
	if err := forgeStoreJSON(name, cp); err != nil {
		t.Fatal(err)
	}
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("checkpoint content substituted for signed native comment")
	}
	if f.commentCreates != 1 {
		t.Fatal("corrupt checkpoint wrote")
	}
}

func TestForgeRemoteIntentPhasesAndPositiveOnlyRecovery(t *testing.T) {
	raw := json.RawMessage(`{"value":"exact"}`)
	i := forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: "sha256:" + strings.Repeat("a", 64), Provider: "other", Endpoint: "https://other.example/api", Kind: "issue", Action: "create", NativeID: "sha256:" + strings.Repeat("b", 64), NativeRevision: "sha256:" + strings.Repeat("c", 64), Payload: raw, PayloadDigest: eventID(raw), Phase: "prepared"}
	phases := []string{}
	writes := 0
	save := func() error { phases = append(phases, i.Phase); return nil }
	write := func(context.Context, forgeRemoteIntent) (forgeRemoteResult, error) {
		writes++
		return forgeRemoteResult{}, errors.New("lost private credential")
	}
	missing := func(context.Context, forgeRemoteIntent) (forgeRemoteResult, bool, error) {
		return forgeRemoteResult{}, false, nil
	}
	validate := func(_ forgeRemoteIntent, r forgeRemoteResult) error {
		if string(r.Payload) != string(raw) {
			return errors.New("payload mismatch")
		}
		return nil
	}
	if _, err := forgeExecuteRemoteIntent(context.Background(), &i, save, write, missing, validate); err == nil || strings.Contains(err.Error(), "credential") {
		t.Fatalf("unexpected uncertain result %v", err)
	}
	if strings.Join(phases, ",") != "prepared,in_flight" || writes != 1 {
		t.Fatalf("phases=%v writes=%d", phases, writes)
	}
	if _, err := forgeExecuteRemoteIntent(context.Background(), &i, save, write, missing, validate); err == nil || writes != 1 {
		t.Fatal("negative reconciliation repeated write")
	}
	positive := func(context.Context, forgeRemoteIntent) (forgeRemoteResult, bool, error) {
		return forgeRemoteResult{ID: "remote:1", Key: "one", Payload: raw, PayloadDigest: eventID(raw)}, true, nil
	}
	if _, err := forgeExecuteRemoteIntent(context.Background(), &i, save, write, positive, validate); err != nil || writes != 1 || i.Phase != "confirmed" {
		t.Fatalf("positive result writes=%d phase=%s err=%v", writes, i.Phase, err)
	}
	if _, err := forgeExecuteRemoteIntent(context.Background(), &i, save, write, missing, validate); err != nil || writes != 1 {
		t.Fatalf("confirmed result wrote again: %v", err)
	}
}

func TestForgeExportReverseCommentClaimsDoNotEraseUnverifiedContent(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	f.issues["7"] = forge.Issue{ID: "source:7", Key: "7", Title: "source", Body: "body", State: "open", URL: "https://other.example/issues/7"}
	options := transferOptions{provider: f.Kind(), project: "team/project", endpoint: f.Endpoint(), key: "7"}
	if _, err := captureTestOutput(t, func() error { return forgeImport(context.Background(), f, options) }); err != nil {
		t.Fatal(err)
	}
	items, err := forgeNativeIssues()
	if err != nil {
		t.Fatal(err)
	}
	id := items[0].ID
	commentID := forgeExportNativeComment(t, id, "real body")
	comments, err := forgeNativeComments(id)
	if err != nil {
		t.Fatal(err)
	}
	actual := comments[0]
	if actual.ID != commentID {
		t.Fatal("unexpected fixture comment")
	}
	forged := actual
	forged.ID = "sha256:" + strings.Repeat("f", 64)
	body, err := forgeFormatExportComment(f.Kind(), f.Endpoint(), "source:7", id, forged)
	if err != nil {
		t.Fatal(err)
	}
	f.comments["7"] = []forge.Comment{{ID: "forged:1", Body: body}, {ID: "ordinary:2", Body: "literal" + forgeExportCommentPrefix + "not-valid -->"}}
	if _, err := captureTestOutput(t, func() error { return forgeImport(context.Background(), f, options) }); err != nil {
		t.Fatal(err)
	}
	rows, err := forgeNativeComments(id)
	if err != nil || len(rows) != 3 {
		t.Fatalf("unverified source content erased, count=%d err=%v", len(rows), err)
	}
}

func TestForgeExportRefusesIgnoredFields(t *testing.T) {
	issueCommandTestRepo(t)
	f := newForgeExportFake(t)
	id := forgeExportNativeOpen(t, IssueState{Title: "fields"})
	if _, err := forgeExportTestRun(t, id); err != nil {
		t.Fatal(err)
	}
	f.ignoreLabels = true
	forgeExportNativeEdit(t, id, func(s *IssueState) { s.Labels = []string{"required"} })
	if _, err := forgeExportTestRun(t, id); err == nil {
		t.Fatal("ignored requested fields checkpointed as success")
	}
	if f.updates != 1 {
		t.Fatal("unexpected writes")
	}
}
