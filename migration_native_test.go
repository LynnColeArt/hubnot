package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"hubnot/internal/migration"
)

func migrationObjectFiles(t *testing.T, dir string) []string {
	t.Helper()
	rows := []string{}
	e := filepath.WalkDir(filepath.Join(dir, "objects"), func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(dir, p)
			rows = append(rows, rel+" "+migration.Digest(b))
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	sort.Strings(rows)
	return rows
}
func migrationBare(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo.git")
	if _, e := projectGit(context.Background(), "", nil, 4096, "init", "--bare", "--quiet", dir); e != nil {
		t.Fatal(e)
	}
	return dir
}
func TestMigrationAuditOnlyClosurePrivateExportAndLocalAdmission(t *testing.T) {
	root, id, target := auditFixture(t)
	ctx := context.Background()
	l := migration.DefaultLimits()
	// The policy proof deliberately omits an unrelated tree sibling and commit
	// parents. The requested target is older, so it cannot fill those holes.
	if e := os.WriteFile("unrelated", []byte("not a proof dependency"), 0600); e != nil {
		t.Fatal(e)
	}
	mustGit(t, "add", "unrelated")
	mustGit(t, "commit", "-qm", "policy proof with omitted parent and sibling")
	policy := mustGitText(t, "rev-parse", "HEAD")
	sibling := mustGitText(t, "rev-parse", "HEAD:unrelated")
	auth := auditAuthorizeFixture(t, id, policy)
	req, e := AuditRequest(target, "check", "", id)
	if e != nil {
		t.Fatal(e)
	}
	source := migrationBare(t)
	head := mustGitText(t, "rev-parse", actorRef(id.Actor))
	ordinary, e := projectObjectIDs(ctx, filepath.Join(root, ".git"), []string{head}, l)
	if e != nil {
		t.Fatal(e)
	}
	if e = projectCopyObjectIDs(ctx, filepath.Join(root, ".git"), source, ordinary, l); e != nil {
		t.Fatal(e)
	}
	if _, e = projectGit(ctx, source, nil, 256, "update-ref", actorRef(id.Actor), head); e != nil {
		t.Fatal(e)
	}
	before := migrationObjectFiles(t, source)
	refsBefore, _ := projectGit(ctx, source, nil, 4096, "show-ref")
	p, e := ExportProjectSnapshot(ctx, source, []string{actorRef(id.Actor)}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if !reflect.DeepEqual(before, migrationObjectFiles(t, source)) {
		t.Fatal("export mutated source object database")
	}
	refsAfter, _ := projectGit(ctx, source, nil, 4096, "show-ref")
	if string(refsBefore) != string(refsAfter) {
		t.Fatal("source refs changed")
	}
	q, e := StageProjectSnapshot(ctx, p.Snapshot, p, l)
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	if !stringListed(policy, q.ObjectIDs()) || !stringListed(target, q.ObjectIDs()) || stringListed(sibling, q.ObjectIDs()) {
		t.Fatal("audit proof closure changed")
	}
	if !reflect.DeepEqual(q.Roots(), []string{head}) {
		t.Fatal("audit roots leaked into full-history roots", q.Roots())
	}
	limited := l
	limited.GitObjects = len(ordinary)
	if _, e = StageProjectSnapshot(ctx, p.Snapshot, p, limited); e == nil {
		t.Fatal("expanded audit objects escaped aggregate Git limit")
	}
	dest := migrationBare(t)
	loc := migration.Locator{Kind: "local", Repository: dest}
	d := LocalMigrationDestination{dest}
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), p.Snapshot, p, migration.Selection{Destination: loc}, d, l, []migration.Decision{{FindingID: "local/governance", Action: "preserve", Mapping: json.RawMessage(`{}`)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, session, d, ""); e != nil {
		t.Fatal(e)
	}
	ref, _ := migrationObservationRef(p.Snapshot.SourceID, p.Snapshot.NativeRefs[0])
	got, _ := projectGit(ctx, dest, nil, 256, "show-ref", "--verify", "--hash", ref)
	if strings.TrimSpace(string(got)) != head {
		t.Fatal("native observation not promoted")
	}
	for _, event := range []*StoredEvent{auth, req} {
		loaded, e := loadStoredEventAt(dest, event.Commit)
		if e != nil || loaded.ID != event.ID {
			t.Fatalf("keyless audit read %s: %v", event.ID, e)
		}
		if !reflect.DeepEqual(loaded.Attachments, event.Attachments) {
			t.Fatal("signed attachment bytes changed")
		}
	}
	for _, private := range []string{"hn/audit/authority.json", "hn/identity.json", "hn/identities"} {
		if _, e = os.Lstat(filepath.Join(dest, private)); !os.IsNotExist(e) {
			t.Fatal("private authority/identity imported", private)
		}
	}
	archive, e := ReadLocalMigration(ctx, dest, session.Plan.SnapshotDigest, l)
	if e != nil {
		t.Fatal(e)
	}
	archive.Close()
	// A confirmed second application has no effects and needs no source.
	p.Close()
	if e = ApplyMigration(ctx, session, d, ""); e != nil {
		t.Fatal(e)
	}
}
func TestMigrationOrdinaryIncompleteHistoryStillRefused(t *testing.T) {
	root := inReplicationTestRepository(t)
	mustGit(t, "commit", "--allow-empty", "-qm", "child")
	ctx := context.Background()
	dir := filepath.Join(root, ".git")
	head := mustGitText(t, "rev-parse", "HEAD")
	tree := mustGitText(t, "rev-parse", "HEAD^{tree}")
	source := migrationBare(t)
	l := migration.DefaultLimits()
	if e := projectCopyObjectIDs(ctx, dir, source, []string{head, tree}, l); e != nil {
		t.Fatal(e)
	}
	if _, e := projectGit(ctx, source, nil, 256, "update-ref", "refs/heads/main", head); e != nil {
		t.Fatal(e)
	}
	if _, e := ExportProjectSnapshot(ctx, source, []string{"refs/heads/main"}, l); e == nil {
		t.Fatal("incomplete ordinary ancestry accepted")
	}
}
func TestMigrationNativeLostAckReconcilesExactRefs(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	l := migration.DefaultLimits()
	p, e := ExportProjectSnapshot(ctx, filepath.Join(root, ".git"), []string{"refs/heads/main"}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	dest := migrationBare(t)
	d := LocalMigrationDestination{dest}
	sel := migration.Selection{Destination: migration.Locator{Kind: "local", Repository: dest}}
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "saved"), p.Snapshot, p, sel, d, l, []migration.Decision{{FindingID: "local/governance", Action: "preserve", Mapping: json.RawMessage(`{}`)}})
	if e != nil {
		t.Fatal(e)
	}
	lost := migrationLostAckDestination{Destination: d}
	if e = ApplyMigration(ctx, s, lost, ""); e == nil {
		t.Fatal("expected lost acknowledgement")
	}
	if e = ApplyMigration(ctx, s, d, ""); e != nil {
		t.Fatal(e)
	}
}

type migrationLostAckDestination struct{ migration.Destination }

func (d migrationLostAckDestination) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	_, e := d.Destination.Apply(ctx, in, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	return migration.Receipt{}, io.ErrUnexpectedEOF
}

type migrationFixtureSource struct {
	s migration.Snapshot
	b migration.BlobSource
}

func (s migrationFixtureSource) Capabilities(context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "issue", Read: true, Preserve: true}, {Kind: "comment", Read: true, Preserve: true}}, nil
}
func (s migrationFixtureSource) Snapshot(context.Context, migration.Selection, migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	return s.s, s.b, nil
}
func TestMigrationComposedCodeAndCommunicationsGuardedGitResume(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	l := migration.DefaultLimits()
	sourceGit := filepath.Join(root, ".git")
	snap, blobs := migrationForeignSnapshot()
	capture := ComposeMigrationSources(migrationFixtureSource{snap, blobs}, LocalMigrationSource{sourceGit})
	remote := migrationBare(t)
	loc := migration.Locator{Kind: "fixture", Repository: "destination-project"}
	selection := migration.Selection{Source: snap.Source, Destination: loc, Refs: []string{"refs/heads/main"}}
	combined, bs, e := capture.Snapshot(ctx, selection, l)
	if e != nil {
		t.Fatal(e)
	}
	defer bs.(io.Closer).Close()
	if len(combined.Records) != 3 || len(combined.NativeRefs) != 1 || combined.SourceID != snap.SourceID {
		t.Fatal("source composition lost scope/attribution")
	}
	communications := &migrationReceiptDestination{locator: loc, commentOK: true}
	gitDest := GitMigrationDestination{Remote: remote, Destination: loc, Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}}
	destination := ComposeMigrationDestinations(communications, migrationLostAckDestination{gitDest})
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "plan"), combined, bs, selection, destination, l, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, session, destination, ""); e == nil {
		t.Fatal("expected lost Git acknowledgement")
	}
	head := mustGitText(t, "rev-parse", "HEAD")
	observed, e := projectGit(ctx, remote, nil, 256, "show-ref", "--verify", "--hash", "refs/heads/main")
	if e != nil || strings.TrimSpace(string(observed)) != head {
		t.Fatal("code did not migrate", e)
	}
	// Reopening uses only saved bytes and structural destination composition.
	session, e = OpenMigrationSession(ctx, session.Directory)
	if e != nil {
		t.Fatal(e)
	}
	destination = ComposeMigrationDestinations(communications, gitDest)
	if e = ApplyMigration(ctx, session, destination, ""); e != nil {
		t.Fatal(e)
	}
	if communications.creates != 1 || communications.comments != 1 {
		t.Fatal("communications repeated during Git recovery")
	}
	// A subsequent selected source head advances code by guarded fast-forward.
	mustGit(t, "commit", "--allow-empty", "-qm", "next selected revision")
	combined2, bs2, e := capture.Snapshot(ctx, selection, l)
	if e != nil {
		t.Fatal(e)
	}
	defer bs2.(io.Closer).Close()
	codeOnly, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "advance"), combined2, bs2, selection, gitDest, l, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, codeOnly, gitDest, ""); e != nil {
		t.Fatal(e)
	}
	advanced := mustGitText(t, "rev-parse", "HEAD")
	if advanced == head {
		t.Fatal("fixture did not advance")
	}
	observed, _ = projectGit(ctx, remote, nil, 256, "show-ref", "--verify", "--hash", "refs/heads/main")
	if strings.TrimSpace(string(observed)) != advanced {
		t.Fatal("fast-forward not applied")
	}
	// The old snapshot cannot overwrite the advanced remote history.
	if _, e = PrepareMigration(ctx, filepath.Join(t.TempDir(), "rewind"), combined, bs, selection, gitDest, l, nil); e == nil {
		t.Fatal("non-fast-forward code rewrite planned")
	}
	for _, unsafe := range []string{"ext::sh -c bad", "ssh://host/repo", "https://user:secret@host/repo", "file:///tmp/repo", "-upload-pack=bad", "relative/repository"} {
		if migration.ValidGitRemote(unsafe) {
			t.Fatal("unsafe Git remote", unsafe)
		}
	}
}
func TestMigrationComposedGitStaleBaselinePreventsProviderWrites(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	l := migration.DefaultLimits()
	snap, blobs := migrationForeignSnapshot()
	sel := migration.Selection{Destination: migration.Locator{Kind: "fixture", Repository: "d"}, Refs: []string{"refs/heads/main"}}
	s, bs, e := ComposeMigrationSources(migrationFixtureSource{snap, blobs}, LocalMigrationSource{filepath.Join(root, ".git")}).Snapshot(ctx, sel, l)
	if e != nil {
		t.Fatal(e)
	}
	defer bs.(io.Closer).Close()
	remote := migrationBare(t)
	communication := &migrationReceiptDestination{locator: sel.Destination, commentOK: true}
	gd := GitMigrationDestination{Remote: remote, Destination: sel.Destination, Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}}
	dest := ComposeMigrationDestinations(communication, gd)
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "plan"), s, bs, sel, dest, l, nil)
	if e != nil {
		t.Fatal(e)
	}
	head := mustGitText(t, "rev-parse", "HEAD")
	if e = projectCopyObjectIDs(ctx, filepath.Join(root, ".git"), remote, []string{head, mustGitText(t, "rev-parse", "HEAD^{tree}")}, l); e != nil {
		t.Fatal(e)
	}
	if _, e = projectGit(ctx, remote, nil, 256, "update-ref", "refs/heads/main", head); e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, session, dest, ""); e == nil {
		t.Fatal("stale remote applied")
	}
	if communication.creates+communication.comments != 0 {
		t.Fatal("provider mutated before shared Git baseline preflight")
	}
}
func TestMigrationExportRefusesPromisorWithoutLazyFetch(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	dir := filepath.Join(root, ".git")
	before := migrationObjectFiles(t, dir)
	mustGit(t, "config", "remote.fake.promisor", "true")
	mustGit(t, "config", "remote.fake.url", "ext::must-not-execute")
	if _, e := ExportProjectSnapshot(ctx, dir, []string{"refs/heads/main"}, migration.DefaultLimits()); e == nil || !strings.Contains(e.Error(), "promisor") {
		t.Fatal("partial source not safely refused", e)
	}
	if !reflect.DeepEqual(before, migrationObjectFiles(t, dir)) {
		t.Fatal("refused source changed object database")
	}
}

func TestMigrationAuthenticatedHTTPSGitRedactionAndFreshResumeCredentials(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	l := migration.DefaultLimits()
	source, e := ExportProjectSnapshot(ctx, filepath.Join(root, ".git"), []string{"refs/heads/main"}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer source.Close()
	parent := t.TempDir()
	remoteDir := filepath.Join(parent, "repo.git")
	if _, e = projectGit(ctx, "", nil, 4096, "init", "--bare", "--quiet", remoteDir); e != nil {
		t.Fatal(e)
	}
	if _, e = projectGit(ctx, remoteDir, nil, 256, "config", "http.receivepack", "true"); e != nil {
		t.Fatal(e)
	}
	token := base64.StdEncoding.EncodeToString([]byte("operator:transient-secret-sentinel-4711"))
	authorization := "Basic " + token
	var requests, redirected atomic.Int64
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1); w.WriteHeader(500) }))
	defer other.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != authorization {
			w.Header().Set("WWW-Authenticate", `Basic realm="migration fixture"`)
			w.WriteHeader(401)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/redirect.git/") {
			http.Redirect(w, r, other.URL+"/stolen", http.StatusTemporaryRedirect)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/echo-error.git/") {
			w.WriteHeader(500)
			fmt.Fprint(w, authorization)
			return
		}
		migrationServeGitBackend(w, r, parent)
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "fixture-ca.pem")
	if e = os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); e != nil {
		t.Fatal(e)
	}
	credentials := &GitHTTPSCredentials{Origin: server.URL, Authorization: authorization, CAFile: ca}
	loc := migration.Locator{Kind: "fixture", Repository: "private-code"}
	d := GitMigrationDestination{Remote: server.URL + "/repo.git", Destination: loc, Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}, Credentials: credentials}
	sel := migration.Selection{Destination: loc}
	// Neither accidental destination serialization nor canonical saved state may
	// persist the transient credential input.
	accidental, _ := json.Marshal(d)
	if bytes.Contains(accidental, []byte(token)) || bytes.Contains(accidental, []byte(authorization)) {
		t.Fatal("credentials serializable")
	}
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "private-plan"), source.Snapshot, source, sel, d, l, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, s, migrationLostAckDestination{d}, ""); e == nil {
		t.Fatal("expected lost HTTPS acknowledgement")
	}
	for _, file := range []string{"plan.json", "snapshot.json", "progress.json"} {
		raw, e := os.ReadFile(filepath.Join(s.Directory, file))
		if e != nil {
			t.Fatal(e)
		}
		for _, secret := range []string{authorization, token, "transient-secret-sentinel-4711"} {
			if bytes.Contains(raw, []byte(secret)) {
				t.Fatal("secret persisted", file)
			}
		}
	}
	d.Credentials = nil
	if e = ApplyMigration(ctx, s, d, ""); e == nil {
		t.Fatal("private resume silently recovered credentials")
	}
	if strings.Contains(e.Error(), token) || strings.Contains(e.Error(), authorization) {
		t.Fatal("Git error disclosed authentication")
	}
	d.Credentials = credentials
	if e = ApplyMigration(ctx, s, d, ""); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/redirect.git", "/echo-error.git"} {
		bad := d
		bad.Remote = server.URL + path
		_, _, e = bad.Plan(ctx, source.Snapshot, sel, l)
		if e == nil {
			t.Fatal("transport accepted redirect/server error")
		}
		if strings.Contains(e.Error(), token) || strings.Contains(e.Error(), authorization) {
			t.Fatal("server error leaked token")
		}
	}
	if redirected.Load() != 0 {
		t.Fatal("credential-bearing cross-origin redirect followed")
	}
	if requests.Load() < 5 {
		t.Fatal("authenticated fixture was not exercised")
	}
	for _, bad := range []GitHTTPSCredentials{{Origin: other.URL, Authorization: authorization}, {Origin: server.URL, Authorization: authorization + "\r\nInjected: true"}, {Origin: server.URL, Authorization: "Basic not-base64"}, {Origin: server.URL, Authorization: "Bearer token?invalid"}, {Origin: server.URL, Authorization: "Bearer "}} {
		if _, e = migrationGitAuth(d.Remote, &bad); e == nil {
			t.Fatal("invalid transient Git auth accepted")
		}
	}
	// Bearer syntax is independently accepted without a provider-specific branch.
	if _, e = migrationGitAuth(d.Remote, &GitHTTPSCredentials{Origin: server.URL, Authorization: "Bearer opaque-token_1"}); e != nil {
		t.Fatal(e)
	}
}

func (d migrationLostAckDestination) Preflight(ctx context.Context, in migration.Intent, ac migration.ApplyContext) error {
	if p, ok := d.Destination.(interface {
		Preflight(context.Context, migration.Intent, migration.ApplyContext) error
	}); ok {
		return p.Preflight(ctx, in, ac)
	}
	return nil
}

func migrationServeGitBackend(w http.ResponseWriter, r *http.Request, parent string) {
	cmd := exec.Command("git", "http-backend")
	cmd.Env = append(os.Environ(), "GIT_PROJECT_ROOT="+parent, "GIT_HTTP_EXPORT_ALL=1", "PATH_INFO="+r.URL.Path, "QUERY_STRING="+r.URL.RawQuery, "REQUEST_METHOD="+r.Method, "CONTENT_TYPE="+r.Header.Get("Content-Type"), "CONTENT_LENGTH="+strconv.FormatInt(r.ContentLength, 10), "REMOTE_USER=operator")
	cmd.Stdin = r.Body
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if e := cmd.Run(); e != nil {
		w.WriteHeader(500)
		return
	}
	reader := bufio.NewReader(&out)
	headers, e := textproto.NewReader(reader).ReadMIMEHeader()
	if e != nil {
		w.WriteHeader(500)
		return
	}
	status := 200
	for k, values := range headers {
		if strings.EqualFold(k, "Status") {
			fmt.Sscanf(values[0], "%d", &status)
			continue
		}
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(status)
	io.Copy(w, reader)
}
func TestMigrationHTTPSIgnoresAmbientCredentialStores(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	if _, e := projectGit(ctx, "", nil, 4096, "init", "--bare", "--quiet", filepath.Join(parent, "repo.git")); e != nil {
		t.Fatal(e)
	}
	explicit := "Basic " + base64.StdEncoding.EncodeToString([]byte("explicit:chosen-token"))
	var ambientHeaders atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" && auth != explicit {
			ambientHeaders.Add(1)
		}
		if auth != explicit {
			w.Header().Set("WWW-Authenticate", `Basic realm="explicit only"`)
			w.WriteHeader(401)
			return
		}
		migrationServeGitBackend(w, r, parent)
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if e := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); e != nil {
		t.Fatal(e)
	}
	outer := t.TempDir()
	marker := []byte("machine 127.0.0.1 login ambient-user password ambient-secret-marker\n")
	for _, name := range []string{".netrc", "_netrc"} {
		if e := os.WriteFile(filepath.Join(outer, name), marker, 0600); e != nil {
			t.Fatal(e)
		}
	}
	beforeHome := os.Getenv("HOME")
	beforeUserProfile := os.Getenv("USERPROFILE")
	child := exec.Command(os.Args[0], "-test.run=^TestMigrationHTTPSCredentialIsolationChild$", "-test.v")
	child.Env = []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		switch k {
		case "HOME", "USERPROFILE", "NETRC", "CURL_HOME":
			continue
		}
		child.Env = append(child.Env, v)
	}
	child.Env = append(child.Env, "HOME="+outer, "USERPROFILE="+outer, "NETRC="+filepath.Join(outer, ".netrc"), "CURL_HOME="+outer, "HN_GIT_AUTH_CHILD=1", "HN_GIT_AUTH_REMOTE="+server.URL+"/repo.git", "HN_GIT_AUTH_CA="+ca)
	out, e := child.CombinedOutput()
	if e != nil {
		t.Fatalf("isolated Git credential child failed: %s", out)
	}
	for _, secret := range []string{"ambient-secret-marker", "chosen-token", explicit} {
		if bytes.Contains(out, []byte(secret)) {
			t.Fatal("credential output leak")
		}
	}
	if ambientHeaders.Load() != 0 {
		t.Fatal("ambient netrc credential sent")
	}
	if os.Getenv("HOME") != beforeHome || os.Getenv("USERPROFILE") != beforeUserProfile {
		t.Fatal("parent environment changed")
	}
	for _, name := range []string{".netrc", "_netrc"} {
		raw, e := os.ReadFile(filepath.Join(outer, name))
		if e != nil || !bytes.Equal(raw, marker) {
			t.Fatal("outer credential file changed")
		}
	}
}
func TestMigrationHTTPSCredentialIsolationChild(t *testing.T) {
	if os.Getenv("HN_GIT_AUTH_CHILD") != "1" {
		return
	}
	remote := os.Getenv("HN_GIT_AUTH_REMOTE")
	origin := strings.TrimSuffix(remote, "/repo.git")
	options := &GitHTTPSCredentials{Origin: origin, CAFile: os.Getenv("HN_GIT_AUTH_CA")}
	refs := []migration.RefUpdate{{Ref: "refs/heads/main"}}
	if _, e := migrationRemoteRefs(context.Background(), remote, refs, options); e == nil {
		t.Fatal("anonymous private remote access unexpectedly succeeded")
	}
	options.Authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte("explicit:chosen-token"))
	if _, e := migrationRemoteRefs(context.Background(), remote, refs, options); e != nil {
		t.Fatal(e)
	}
}
