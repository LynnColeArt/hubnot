//go:build !nogithub && !noforge

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/forge"
	"hubnot/internal/githubapi"
)

type githubImportFixture struct {
	issue    githubapi.Issue
	comments []githubapi.Comment
	second   []githubapi.Issue
	requests int
}

func newGitHubImportFixture(t *testing.T) (*githubImportFixture, *httptest.Server) {
	t.Helper()
	f := &githubImportFixture{issue: githubapi.Issue{ID: 11, NodeID: "I_11", Number: 7, Title: "External", Body: "source body", State: "open", HTMLURL: "https://github.com/acme/project/issues/7", User: githubapi.User{Login: "andrew"}, CreatedAt: "2026-10-24T01:00:00Z", UpdatedAt: "2026-10-24T01:00:00Z", Labels: []githubapi.Label{{Name: "bug"}}, Assignees: []githubapi.User{{Login: "alice"}}}, comments: []githubapi.Comment{{ID: 22, NodeID: "IC_22", Body: "source comment", HTMLURL: "https://github.com/acme/project/issues/7#issuecomment-22", User: githubapi.User{Login: "reviewer"}, CreatedAt: "2026-10-24T02:00:00Z", UpdatedAt: "2026-10-24T02:00:00Z"}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests++
		if r.Method != "GET" {
			t.Errorf("import attempted %s", r.Method)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/project/issues":
			if r.URL.Query().Get("page") == "2" {
				json.NewEncoder(w).Encode(f.second)
				return
			}
			if f.second != nil {
				w.Header().Set("Link", fmt.Sprintf(`<%s&page=2>; rel="next"`, "?per_page=100&state=all"))
			}
			json.NewEncoder(w).Encode([]githubapi.Issue{f.issue})
		case "/repos/acme/project/issues/7":
			json.NewEncoder(w).Encode(f.issue)
		case "/repos/acme/project/issues/7/comments":
			json.NewEncoder(w).Encode(f.comments)
		case "/repos/acme/project/issues/8/comments":
			json.NewEncoder(w).Encode([]githubapi.Comment{})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	return f, server
}
func githubTestCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	old := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = old }()
	callErr := run(append([]string{"github"}, args...))
	os.Stdout = old
	if _, err = file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), callErr
}
func githubTestImport(t *testing.T, server *httptest.Server, extra ...string) string {
	t.Helper()
	args := append([]string{"import", "--repo", "acme/project", "--api-url", server.URL}, extra...)
	out, err := githubTestCommand(t, args...)
	if err != nil {
		t.Fatalf("import: %v (%s)", err, out)
	}
	return out
}
func githubTestItems(t *testing.T) []issueDetail {
	t.Helper()
	rows, err := forgeNativeIssues()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
func TestGitHubImportRepeatUpdateAndProvenance(t *testing.T) {
	issueCommandTestRepo(t)
	t.Setenv("HN_GITHUB_TOKEN", "")
	f, server := newGitHubImportFixture(t)
	githubTestImport(t, server)
	items := githubTestItems(t)
	if len(items) != 1 {
		t.Fatal(items)
	}
	id := items[0].ID
	state := items[0].State
	if state.Title != "External" || state.Body != "source body" || len(state.Labels) != 1 || state.Metadata["forge/source"] == "" || state.Metadata["forge/baseline"] == "" {
		t.Fatal(state)
	}
	initialSource := items[0].OpeningMetadata["forge/source"]
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	githubTestImport(t, server)
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); after != before {
		t.Fatal("repeat import mutated refs")
	}
	comments, err := forgeNativeComments(id)
	if err != nil || len(comments) != 1 {
		t.Fatalf("comments %v %v", comments, err)
	}
	if !strings.Contains(comments[0].Body, "reviewer") || !strings.Contains(comments[0].Body, "source comment") {
		t.Fatal(comments)
	}
	f.issue.Title = "Changed upstream"
	f.issue.UpdatedAt = "2026-10-24T03:00:00Z"
	f.comments[0].Body = "edited comment"
	f.comments[0].UpdatedAt = "2026-10-24T03:00:00Z"
	githubTestImport(t, server)
	items = githubTestItems(t)
	if items[0].State.Title != "Changed upstream" || items[0].OpeningMetadata["forge/source"] != initialSource || !strings.Contains(items[0].State.Metadata["forge/observation"], "03:00:00") {
		t.Fatal(items)
	}
	comments, err = forgeNativeComments(id)
	if err != nil || len(comments) != 2 {
		t.Fatalf("edited comments %d %v", len(comments), err)
	}
	// No optional checkpoint state is needed to recognize signed source provenance.
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	if err := os.RemoveAll(filepath.Join(gitDir, "hn", "forge")); err != nil {
		t.Fatal(err)
	}
	githubTestImport(t, server)
	comments, err = forgeNativeComments(id)
	if err != nil || len(comments) != 2 {
		t.Fatalf("clone-like retry duplicated: %d %v", len(comments), err)
	}
}
func TestGitHubImportCompetingEditsAndNativeOnlyFields(t *testing.T) {
	root := issueCommandTestRepo(t)
	f, server := newGitHubImportFixture(t)
	githubTestImport(t, server)
	item := githubTestItems(t)[0]
	view, err := ghReadIssue(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	state := *view.State
	state.Title = "local edit"
	state.Criteria = []IssueCriterion{{ID: "keep", Text: "native"}}
	state.Metadata["native/keep"] = "yes"
	input := issueCommandInput(t, root, state)
	result, err := issueCommandJSON(t, "revise", item.ID, "--expect", view.HeadIDs[0], "--input", input, "--json")
	issueResultData(t, result, err)
	githubTestImport(t, server)
	item = githubTestItems(t)[0]
	if item.State.Title != "local edit" || item.State.Metadata["native/keep"] != "yes" || len(item.State.Criteria) != 1 {
		t.Fatal(item.State)
	}
	f.issue.Title = "remote edit"
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err := githubTestCommand(t, "import", "--repo", "acme/project", "--api-url", server.URL); err == nil {
		t.Fatal("competing edits accepted")
	}
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); after != before {
		t.Fatal("competing import mutated refs")
	}
}
func TestGitHubImportPreflightDryRunAndPagination(t *testing.T) {
	issueCommandTestRepo(t)
	f, server := newGitHubImportFixture(t)
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	githubTestImport(t, server, "--dry-run")
	if _, err := os.Stat(filepath.Join(gitDir, "hn", "forge")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created adapter state: %v", err)
	}
	if rows := githubTestItems(t); len(rows) != 0 {
		t.Fatal(rows)
	}
	f.comments[0].Body = strings.Repeat("x", 65536)
	if _, err := githubTestCommand(t, "import", "--repo", "acme/project", "--api-url", server.URL); err == nil {
		t.Fatal("overlarge attributed comment accepted")
	}
	if rows := githubTestItems(t); len(rows) != 0 {
		t.Fatal("invalid comment partially imported issue")
	}
	f.comments = []githubapi.Comment{}
	second := f.issue
	second.ID = 12
	second.NodeID = "I_12"
	second.Number = 8
	second.HTMLURL = "https://github.com/acme/project/issues/8"
	f.second = []githubapi.Issue{second}
	githubTestImport(t, server)
	if rows := githubTestItems(t); len(rows) != 2 {
		t.Fatalf("paginated import: %d", len(rows))
	}
}
func TestGitHubImportDuplicateSourceAndStrictArguments(t *testing.T) {
	issueCommandTestRepo(t)
	_, server := newGitHubImportFixture(t)
	githubTestImport(t, server)
	state := *githubTestItems(t)[0].State
	if _, err := executeIssueMutation(issueOptions{command: "open", state: &state}); err != nil {
		t.Fatal(err)
	}
	if _, err := githubTestCommand(t, "import", "--repo", "acme/project", "--api-url", server.URL); err == nil {
		t.Fatal("duplicate source accepted")
	}
	for _, args := range [][]string{{"import", "--repo", "acme/project", "--issue", "0"}, {"import", "--repo", "acme/project", "--unknown"}, {"import", "--repo", "acme/project", "extra"}, {"export", "bad", "--repo", "acme/project"}} {
		if _, err := githubTestCommand(t, args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

type exampleForge struct{ requests int }

func (f *exampleForge) Kind() string     { return "example" }
func (f *exampleForge) Endpoint() string { return "https://forge.example/api" }
func (f *exampleForge) GetIssue(context.Context, string, string) (forge.Issue, error) {
	f.requests++
	return forge.Issue{ID: "opaque:content/alpha", Key: "TEAM-42", Title: "Neutral issue", Body: "portable", State: "open", Author: "a contributor", URL: "https://forge.example/ticket/TEAM-42"}, nil
}
func (f *exampleForge) ListIssues(ctx context.Context, project string) ([]forge.Issue, error) {
	v, err := f.GetIssue(ctx, project, "TEAM-42")
	return []forge.Issue{v}, err
}
func (f *exampleForge) ListComments(context.Context, string, string) ([]forge.Comment, error) {
	f.requests++
	return []forge.Comment{{ID: "message:alpha/beta", Body: "portable comment", Author: "another contributor"}}, nil
}
func (f *exampleForge) CreateIssue(context.Context, string, forge.IssueInput) (forge.Issue, error) {
	return forge.Issue{}, errors.New("unexpected write")
}
func (f *exampleForge) UpdateIssue(context.Context, string, string, forge.IssueInput) (forge.Issue, error) {
	return forge.Issue{}, errors.New("unexpected write")
}
func (f *exampleForge) CreateComment(context.Context, string, string, string) (forge.Comment, error) {
	return forge.Comment{}, errors.New("unexpected write")
}
func TestForgeNeutralOpaqueKeysAndCapabilityRefusal(t *testing.T) {
	issueCommandTestRepo(t)
	f := &exampleForge{}
	factoryCalls := 0
	forgeProviders["example"] = forgeRegistration{New: func(string, string) (forge.Provider, error) { factoryCalls++; return f, nil }, TokenEnv: "HN_EXAMPLE_TEST_TOKEN", Capabilities: []forge.Capability{{Resource: forge.ResourceIssue, Import: true, Export: true, MappedFields: []string{"title", "body"}, Unsupported: []string{}}}}
	t.Cleanup(func() { delete(forgeProviders, "example") })
	if err := run([]string{"transfer", "import", "--provider", "example", "--project", "arbitrary/project", "--key", "TEAM-42"}); err != nil {
		t.Fatal(err)
	}
	rows := githubTestItems(t)
	if len(rows) != 1 || rows[0].State.Title != "Neutral issue" {
		t.Fatal(rows)
	}
	source, err := forgeParseSource(rows[0].OpeningMetadata["forge/source"])
	if err != nil || source.ID != "opaque:content/alpha" || source.Key != "TEAM-42" {
		t.Fatalf("source %#v %v", source, err)
	}
	before := f.requests
	beforeFactory := factoryCalls
	for _, resource := range []string{"project", "workflow", "discussion"} {
		if err := run([]string{"transfer", "import", "--provider", "example", "--project", "arbitrary/project", "--resource", resource}); err == nil {
			t.Fatalf("accepted %s", resource)
		}
	}
	if f.requests != before || factoryCalls != beforeFactory {
		t.Fatal("unsupported resource touched provider")
	}
	// Capability inspection needs no native repository or provider factory/credential.
	t.Chdir(t.TempDir())
	if err := run([]string{"transfer", "capabilities", "--provider", "example"}); err != nil {
		t.Fatal(err)
	}
	if factoryCalls != beforeFactory {
		t.Fatal("capability report constructed provider")
	}
}

func TestForgeImportRecoversOriginalIntentWithoutRefreshingHead(t *testing.T) {
	root := issueCommandTestRepo(t)
	f, server := newGitHubImportFixture(t)
	githubTestImport(t, server)
	item := githubTestItems(t)[0]
	current, err := ghReadIssue(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := inspectIdentity()
	if err != nil {
		t.Fatal(err)
	}
	f.issue.Title = "new upstream"
	f.issue.UpdatedAt = "2026-10-24T04:00:00Z"
	provider := &githubForge{client: mustGitHubClient(t, server.URL)}
	prepared, err := forgePrepareImport(provider, "acme/project", githubForgeIssue(f.issue), nil)
	if err != nil {
		t.Fatal(err)
	}
	state := prepared.state
	state.Metadata["forge/source"] = item.OpeningMetadata["forge/source"]
	scope := forgeImportScope(provider, transferOptions{project: "acme/project"})
	pending := forgeNativeIntent{Schema: "hn.forge-native-intent/1", Scope: scope, Actor: identity.Actor, Command: "revise", IssueID: item.ID, Expected: current.HeadIDs, State: &state, Operation: forgeOperation("revise", forgeSourceKey(prepared.source), strings.Join(current.HeadIDs, ","), forgeDigestJSON(state))}
	if err := forgeStoreJSON("native-intent.json", pending); err != nil {
		t.Fatal(err)
	}
	if _, err := forgeAttemptNative(pending); err != nil {
		t.Fatal(err)
	} // native write succeeded; response/checkpoint confirmation was lost.
	current, err = ghReadIssue(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	local := *current.State
	local.Body = "local edit after original write"
	input := issueCommandInput(t, root, local)
	result, err := issueCommandJSON(t, "revise", item.ID, "--expect", current.HeadIDs[0], "--input", input, "--json")
	issueResultData(t, result, err)
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	githubTestImport(t, server)
	after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if before != after {
		t.Fatal("recovery minted a replacement intent or overwrote local head")
	}
	if current, err = ghReadIssue(item.ID); err != nil || current.State.Body != "local edit after original write" {
		t.Fatalf("state %#v %v", current, err)
	}
}
func mustGitHubClient(t *testing.T, endpoint string) *githubapi.Client {
	t.Helper()
	client, err := githubapi.NewClient(endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestForgePortableCommentProofAndPrivateCheckpointSafety(t *testing.T) {
	issueCommandTestRepo(t)
	_, server := newGitHubImportFixture(t)
	githubTestImport(t, server)
	item := githubTestItems(t)[0]
	comments, err := forgeNativeComments(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	proof, raw, present, err := forgeParseComment(comments[0].Body)
	if err != nil || !present || raw != "source comment" {
		t.Fatalf("proof %#v %q %v", proof, raw, err)
	}
	forged := strings.Replace(comments[0].Body, "source comment", "altered comment", 1)
	if _, _, _, err := forgeParseComment(forged); err == nil {
		t.Fatal("accepted altered attributed comment")
	}
	writeActiveTestIdentity(t, testIdentity(t, "different importer"))
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	githubTestImport(t, server)
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); after != before {
		t.Fatal("changing importing actor duplicated source content")
	}
	if err := forgeStoreJSON("example.json", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	path, err := forgeCheckpointPath("example.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("checkpoint mode %v %v", info, err)
	}
	if err := os.WriteFile(path, []byte(`{"a":"first","a":"second"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var value map[string]string
	if _, err := forgeLoadJSON("example.json", &value); err == nil {
		t.Fatal("duplicate checkpoint keys accepted")
	}
	if _, err := forgeCheckpointPath("../escape"); err == nil {
		t.Fatal("checkpoint path traversal")
	}
}

func TestForgePublicRoutingAndTransportExcludedBuild(t *testing.T) {
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	regular := buildOperationalBinary(t)
	core := filepath.Join(t.TempDir(), "hn-core")
	build := exec.Command("go", "build", "-tags", "nogithub", "-o", core, ".")
	build.Dir = source
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("nogithub build: %v %s", err, raw)
	}
	root := issueCommandTestRepo(t)
	id := strings.TrimSpace(runOperationalCommand(t, regular, root, "gh", "issue", "create", "--title", "local through public route"))
	if !validEventID(id) {
		t.Fatal(id)
	}
	if out := runOperationalCommand(t, core, root, "gh", "issue", "view", id, "--json", "title"); !strings.Contains(out, "local through public route") {
		t.Fatal(out)
	}
	_, server := newGitHubImportFixture(t)
	runOperationalCommand(t, regular, root, "transfer", "import", "--provider", "github", "--project", "acme/project", "--endpoint", server.URL)
	if out := runOperationalCommand(t, core, root, "gh", "issue", "list", "--json", "title"); !strings.Contains(out, "External") {
		t.Fatal(out)
	}
	if out := runOperationalCommandFailure(t, core, root, "github", "import", "--repo", "acme/project", "--api-url", server.URL); !strings.Contains(out, "excluded") {
		t.Fatal(out)
	}
	if out := runOperationalCommandFailure(t, core, root, "transfer", "import", "--provider", "github", "--project", "acme/project", "--endpoint", server.URL); !strings.Contains(out, "unavailable") {
		t.Fatal(out)
	}
}

func TestForgeImportRejectsManufacturedBaseline(t *testing.T) {
	root := issueCommandTestRepo(t)
	f, server := newGitHubImportFixture(t)
	githubTestImport(t, server)
	item := githubTestItems(t)[0]
	view, err := ghReadIssue(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	local := *view.State
	local.Title = "local work"
	digest, err := forgeManagedDigest(local)
	if err != nil {
		t.Fatal(err)
	}
	local.Metadata["forge/baseline"] = digest
	input := issueCommandInput(t, root, local)
	result, err := issueCommandJSON(t, "revise", item.ID, "--expect", view.HeadIDs[0], "--input", input, "--json")
	issueResultData(t, result, err)
	f.issue.Title = "upstream competing change"
	before := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
	if _, err := githubTestCommand(t, "import", "--repo", "acme/project", "--api-url", server.URL); err == nil {
		t.Fatal("manufactured baseline accepted")
	}
	if after := mustGitText(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); after != before {
		t.Fatal("tampered baseline import mutated refs")
	}
	view, err = ghReadIssue(item.ID)
	if err != nil || view.State.Title != "local work" {
		t.Fatalf("local work lost: %#v %v", view.State, err)
	}
}
func TestForgeImportPreservesLiteralExportMarkerProse(t *testing.T) {
	issueCommandTestRepo(t)
	f, server := newGitHubImportFixture(t)
	f.issue.Body = "Document this literal prefix:\n\n<!-- hn:forge-issue:not-an-export"
	githubTestImport(t, server)
	if got := githubTestItems(t)[0].State.Body; got != f.issue.Body {
		t.Fatalf("source prose changed: %q", got)
	}
}

func TestForgeImportSelectionCannotReplayAnotherPendingIssue(t *testing.T) {
	issueCommandTestRepo(t)
	f, server := newGitHubImportFixture(t)
	provider := &githubForge{client: mustGitHubClient(t, server.URL)}
	prepared, err := forgePrepareImport(provider, "acme/project", githubForgeIssue(f.issue), nil)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := inspectIdentity()
	if err != nil {
		t.Fatal(err)
	}
	pending := forgeNativeIntent{Schema: "hn.forge-native-intent/1", Scope: forgeImportScope(provider, transferOptions{project: "acme/project", key: "7"}), Actor: identity.Actor, Command: "open", State: &prepared.state, Operation: forgeOperation("open", forgeSourceKey(prepared.source), prepared.baseline)}
	if err := forgeStoreJSON("native-intent.json", pending); err != nil {
		t.Fatal(err)
	}
	requests := f.requests
	if _, err := githubTestCommand(t, "import", "--repo", "acme/project", "--issue", "8", "--api-url", server.URL); err == nil {
		t.Fatal("different selection replayed pending mutation")
	}
	if f.requests != requests || len(githubTestItems(t)) != 0 {
		t.Fatal("mismatched selection touched provider or native state")
	}
	githubTestImport(t, server, "--issue", "7")
	if rows := githubTestItems(t); len(rows) != 1 {
		t.Fatal(rows)
	}
}
func TestForgeStrictMappingsAndLocalExtensionDispatch(t *testing.T) {
	for _, maps := range [][]string{{"--map", ""}, {"--map", "=value"}, {"--map", "key="}, {"--map", "key=value", "--map", "key=other"}, {"--map", "key=value\nmore"}} {
		args := append([]string{"import", "--provider", "example", "--project", "opaque"}, maps...)
		if _, err := parseTransferOptions(args); err == nil {
			t.Fatalf("accepted mappings %v", maps)
		}
	}
	o, err := parseTransferOptions([]string{"import", "--provider", "example", "--project", "opaque", "--key", "TEAM-42", "--map", "category=Build tooling", "--map", "branch=feature/x"})
	if err != nil || o.key != "TEAM-42" || o.mappings["category"] != "Build tooling" || len(o.mappings) != 2 {
		t.Fatalf("options %#v %v", o, err)
	}
	t.Chdir(t.TempDir())
	called := false
	localCommands["example-local"] = func(args []string) error {
		called = true
		if len(args) != 2 || args[0] != "one" || args[1] != "two" {
			t.Fatalf("extension args %v", args)
		}
		return nil
	}
	t.Cleanup(func() { delete(localCommands, "example-local") })
	if err := run([]string{"example-local", "one", "two"}); err != nil || !called {
		t.Fatalf("extension %v %v", called, err)
	}
}
func TestForgeActualExportProofRefusesUnprovenReverseMapping(t *testing.T) {
	issueCommandTestRepo(t)
	f, server := newGitHubImportFixture(t)
	actor := strings.Repeat("a", 64)
	id := "sha256:" + strings.Repeat("b", 64)
	proof := forgeIssueProvenance{Schema: "hn.forge-issue/1", ID: id, Actor: actor, Head: id, Digest: eventID([]byte("native body"))}
	encoded, err := forgeFormatIssueExport(proof, "native body")
	if err != nil {
		t.Fatal(err)
	}
	if _, plain, present := forgeParseIssueExport(encoded); !present || plain != "native body" {
		t.Fatal("valid issue proof not recognized")
	}
	f.issue.Body = encoded
	if _, err := githubTestCommand(t, "import", "--repo", "acme/project", "--api-url", server.URL); err == nil {
		t.Fatal("unproven reverse native mapping accepted")
	}
	if rows := githubTestItems(t); len(rows) != 0 {
		t.Fatal("reverse import created duplicate")
	}
}
