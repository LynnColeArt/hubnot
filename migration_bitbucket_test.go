//go:build !noforge

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/migration"
)

const bbRepoUUID = "{11111111-1111-1111-1111-111111111111}"

func bbFixture(t *testing.T, status int, replace bool) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	repoReads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Errorf("unexpected mutation %s", r.Method)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/pullrequests"):
			if status != 200 {
				w.WriteHeader(status)
				return
			}
			fmt.Fprint(w, `{"values":[{"id":7,"title":"PR","author":{"uuid":"alice"},"created_on":"2026-01-01T00:00:00Z"}]}`)
		case strings.HasSuffix(r.URL.Path, "/comments"):
			fmt.Fprint(w, `{"values":[{"id":2,"content":{"raw":"root"},"inline":{"path":"a.go","to":5},"user":{"uuid":"alice"}},{"id":3,"parent":{"id":2},"content":{"raw":"reply"},"user":{"uuid":"bob"}},{"id":4,"deleted":true}]}`)
		case strings.HasSuffix(r.URL.Path, "/activity"):
			fmt.Fprint(w, `{"values":[{"approval":{"user":{"uuid":"carol"},"date":"2026-01-02T00:00:00Z"}},{"update":{"state":"MERGED","author":{"uuid":"alice"}}}]}`)
		case strings.Contains(r.URL.Path, "/src/"):
			fmt.Fprint(w, "pipelines:\r\n  default: []\r\n")
		default:
			repoReads++
			id := bbRepoUUID
			if replace && repoReads > 1 {
				id = "{22222222-2222-2222-2222-222222222222}"
			}
			fmt.Fprintf(w, `{"uuid":%q,"full_name":"ws/repo","created_on":"2026-01-01T00:00:00Z","project":{"uuid":"group-not-repository"}}`, id)
		}
	}))
	return srv, &calls
}
func bbConfig(endpoint string) migration.ProviderConfig {
	return migration.ProviderConfig{Locator: migration.Locator{Kind: "bitbucket", Endpoint: endpoint, Repository: "ws/repo"}, APIVersion: "2.0", CredentialMode: "anonymous", WorkflowRef: strings.Repeat("a", 40)}
}
func TestBitbucketRegisteredOfflineSnapshot(t *testing.T) {
	srv, calls := bbFixture(t, 200, false)
	defer srv.Close()
	cfg := bbConfig(srv.URL)
	s, e := OpenMigrationSource(cfg)
	if e != nil {
		t.Fatal(e)
	}
	d, e := OpenMigrationDestination(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if *calls != 0 {
		t.Fatal("constructor/metadata performed I/O")
	}
	cap, e := s.Capabilities(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range cap {
		if c.Kind == "issue" && (c.Read || c.Create || c.Reason == "") {
			t.Fatal(c)
		}
	}
	sel := migration.Selection{Source: cfg.Locator, Kinds: []string{"change_request", "review", "inline_comment", "reply", "workflow"}}
	snap, blobs, e := s.Snapshot(context.Background(), sel, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	var root, reply migration.Record
	pipeline := false
	deleted := false
	approvalAttribution := false
	inlinePosition := false
	for _, r := range snap.Records {
		counts[r.Key.Kind]++
		if r.Key.Kind == "review" && r.Author == "carol" && r.CreatedAt == "2026-01-02T00:00:00Z" {
			approvalAttribution = true
		}
		if r.Key.ProjectID != bbRepoUUID || r.Key.Provider != "bitbucket" || r.Key.Instance != srv.URL+"/2.0" {
			t.Fatal(r.Key)
		}
		for _, digest := range r.Blobs {
			f, e := blobs.Open(context.Background(), digest)
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := io.ReadAll(f)
			f.Close()
			if migration.Digest(raw) != digest {
				t.Fatal("digest mismatch")
			}
			if r.Key.ID == "pr:7:comment:2" && strings.Contains(string(raw), `"to":5`) && strings.Contains(string(raw), `"path":"a.go"`) {
				inlinePosition = true
			}
			if r.Key.Kind == "workflow" && string(raw) == "pipelines:\r\n  default: []\r\n" {
				pipeline = true
			}
		}
		if r.Key.ID == "pr:7:comment:2" {
			root = r
		}
		if r.Key.ID == "pr:7:comment:3" {
			reply = r
		}
	}
	for _, f := range snap.Findings {
		if strings.Contains(f.Message, "deleted") {
			deleted = true
		}
	}
	if counts["change_request"] != 1 || counts["review"] != 2 || counts["inline_comment"] != 2 || counts["reply"] != 1 || !pipeline || !deleted || !approvalAttribution || !inlinePosition {
		t.Fatalf("coverage %v pipeline %v deleted %v", counts, pipeline, deleted)
	}
	parent := false
	for _, l := range reply.Links {
		if l.Relation == "parent" && l.Target == root.Key {
			parent = true
		}
	}
	if !parent {
		t.Fatal("reply topology lost")
	}
	if _, e = migration.CanonicalSnapshot(snap, migration.DefaultLimits()); e != nil {
		t.Fatal(e)
	}
	before := *calls
	intents, findings, e := d.Plan(context.Background(), snap, migration.Selection{Destination: cfg.Locator}, migration.DefaultLimits())
	if e != nil || len(intents) != 0 || len(findings) == 0 {
		t.Fatal(intents, findings, e)
	}
	if *calls != before {
		t.Fatal("unsupported planning contacted provider")
	}
	if _, e = d.Apply(context.Background(), migration.Intent{Kind: "comment"}, nil); e == nil {
		t.Fatal("unsupported write accepted")
	}
	if _, e = d.Reconcile(context.Background(), migration.Intent{}, nil); e == nil {
		t.Fatal("fabricated receipt recovery")
	}
	if *calls != before {
		t.Fatal("unsupported writes contacted provider")
	}
}
func TestBitbucketUnsupportedAndMissingDifferFromEmpty(t *testing.T) {
	for _, status := range []int{200, 401, 403, 404, 410} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv, calls := bbFixture(t, status, false)
			defer srv.Close()
			cfg := bbConfig(srv.URL)
			s, e := OpenMigrationSource(cfg)
			if e != nil {
				t.Fatal(e)
			}
			if _, _, e = s.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"issue"}}, migration.DefaultLimits()); e == nil || *calls != 0 {
				t.Fatal("removed issue selection not refused offline", e, *calls)
			}
			snap, _, e := s.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"change_request"}}, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			missing := false
			for _, f := range snap.Findings {
				missing = missing || f.Status == "missing_dependency"
			}
			if missing != (status != 200) {
				t.Fatal(status, snap.Findings)
			}
		})
	}
}
func TestBitbucketReplacedRepositoryAndBudgets(t *testing.T) {
	srv, _ := bbFixture(t, 200, true)
	defer srv.Close()
	cfg := bbConfig(srv.URL)
	s, _ := OpenMigrationSource(cfg)
	if _, _, e := s.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"change_request"}}, migration.DefaultLimits()); e == nil {
		t.Fatal("mixed repository incarnation accepted")
	}
	for _, budget := range []string{"records", "blobs", "blob-bytes", "manifest"} {
		t.Run(budget, func(t *testing.T) {
			srv, _ := bbFixture(t, 200, false)
			defer srv.Close()
			cfg := bbConfig(srv.URL)
			s, _ := OpenMigrationSource(cfg)
			l := migration.DefaultLimits()
			switch budget {
			case "records":
				l.Records = 1
			case "blobs":
				l.Blobs = 1
			case "blob-bytes":
				l.BlobBytes = 3
			case "manifest":
				l.ManifestBytes = 3
			}
			if _, _, e := s.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator}, l); e == nil {
				t.Fatal("incomplete success on budget")
			}
		})
	}
}
func TestBitbucketProfileRefusalsAndTokenEphemeral(t *testing.T) {
	for _, field := range []string{"project", "organization", "version", "mode", "workflow", "planning"} {
		cfg := bbConfig("")
		switch field {
		case "project":
			cfg.Locator.Project = "board"
		case "organization":
			cfg.Locator.Organization = "org"
		case "version":
			cfg.APIVersion = "1.0"
		case "mode":
			cfg.CredentialMode = "app_password"
		case "workflow":
			cfg.WorkflowRef = "main"
		case "planning":
			cfg.PlanningSelectors = []string{"board"}
		}
		if _, e := OpenMigrationSource(cfg); e == nil {
			t.Fatal("accepted", field)
		}
	}
	cfg := bbConfig("")
	cfg.CredentialMode = "api_token_basic"
	cfg.Token = "a@example.com:private"
	raw, _ := json.Marshal(cfg)
	if strings.Contains(string(raw), "private") || strings.Contains(string(raw), "a@example.com") {
		t.Fatal("serialized token")
	}
	if _, e := OpenMigrationSource(cfg); e != nil {
		t.Fatal(e)
	}
}

func TestBitbucketEmptyPRConnectionAndMissingThreadParent(t *testing.T) {
	for _, mode := range []string{"empty", "orphan", "private-comments", "private-workflow"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/pullrequests"):
					if mode == "empty" {
						fmt.Fprint(w, `{"values":[]}`)
					} else {
						fmt.Fprint(w, `{"values":[{"id":1}]}`)
					}
				case strings.HasSuffix(r.URL.Path, "/comments"):
					if mode == "private-comments" {
						w.WriteHeader(403)
					} else {
						fmt.Fprint(w, `{"values":[{"id":4,"parent":{"id":99},"content":{"raw":"surviving reply"}}]}`)
					}
				case strings.Contains(r.URL.Path, "/src/"):
					if mode == "private-workflow" {
						w.WriteHeader(404)
					} else {
						fmt.Fprint(w, "exact yaml")
					}
				default:
					fmt.Fprintf(w, `{"uuid":%q,"full_name":"ws/repo","created_on":"2026-01-01T00:00:00Z"}`, bbRepoUUID)
				}
			}))
			defer srv.Close()
			cfg := bbConfig(srv.URL)
			s, e := OpenMigrationSource(cfg)
			if e != nil {
				t.Fatal(e)
			}
			snap, _, e := s.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"inline_comment", "workflow"}}, migration.DefaultLimits())
			if e != nil {
				t.Fatal(e)
			}
			missing := false
			for _, f := range snap.Findings {
				missing = missing || f.Status == "missing_dependency"
			}
			if missing != (mode != "empty") {
				t.Fatalf("%s findings: %v", mode, snap.Findings)
			}
			if mode == "orphan" {
				found := false
				for _, r := range snap.Records {
					if r.Key.Kind == "reply" {
						found = true
						for _, link := range r.Links {
							if link.Relation == "parent" {
								t.Fatal("fabricated parent")
							}
						}
					}
				}
				if !found {
					t.Fatal("available orphan bytes lost")
				}
			}
		})
	}
}

func TestBitbucketStableUUIDAcrossSelectorRename(t *testing.T) {
	name := "ws/old"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"uuid":%q,"full_name":%q,"created_on":"2026-01-01T00:00:00Z"}`, bbRepoUUID, name)
	}))
	defer srv.Close()
	var previous string
	for _, selector := range []string{"ws/old", "ws/new"} {
		name = selector
		cfg := bbConfig(srv.URL)
		cfg.Locator.Repository = selector
		s, e := OpenMigrationSource(cfg)
		if e != nil {
			t.Fatal(e)
		}
		snap, _, e := s.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"repository"}}, migration.DefaultLimits())
		if e != nil {
			t.Fatal(e)
		}
		if previous != "" && snap.SourceID != previous {
			t.Fatal("mutable selector became identity")
		}
		previous = snap.SourceID
	}
}

func TestBitbucketSelectedAttachmentsStayBoundedAndOnOrigin(t *testing.T) {
	for _, mode := range []string{"cross-origin", "oversize", "private", "retrieved"} {
		t.Run(mode, func(t *testing.T) {
			otherCalls := 0
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { otherCalls++; fmt.Fprint(w, "must not fetch") }))
			defer other.Close()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/downloads/asset") {
					if mode == "private" {
						w.WriteHeader(403)
					} else {
						fmt.Fprint(w, strings.Repeat("x", 2048))
					}
					return
				}
				fmt.Fprintf(w, `{"uuid":%q,"full_name":"ws/repo","created_on":"2026-01-01T00:00:00Z"}`, bbRepoUUID)
			}))
			defer srv.Close()
			cfg := bbConfig(srv.URL)
			asset := srv.URL + "/2.0/downloads/asset"
			if mode == "cross-origin" {
				asset = other.URL + "/2.0/downloads/asset"
			}
			cfg.Attachments = []migration.AttachmentSelection{{URL: asset, Name: "asset", Parent: migration.Key{Provider: "bitbucket", Instance: srv.URL + "/2.0", ProjectID: bbRepoUUID, Kind: "attachment", ID: "repository:" + bbRepoUUID}}}
			s, e := OpenMigrationSource(cfg)
			if e != nil {
				t.Fatal(e)
			}
			l := migration.DefaultLimits()
			if mode == "oversize" {
				l.BlobBytes = 1024
			}
			snap, _, e := s.Snapshot(context.Background(), migration.Selection{Source: cfg.Locator, Kinds: []string{"attachment"}}, l)
			if mode == "cross-origin" || mode == "oversize" {
				if e == nil {
					t.Fatal("hostile attachment accepted")
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				if mode == "retrieved" && len(snap.Blobs) != 2 {
					t.Fatal("selected attachment not retrieved")
				}
				if mode == "private" {
					found := false
					for _, f := range snap.Findings {
						found = found || f.Status == "missing_dependency"
					}
					if !found {
						t.Fatal("private asset claimed empty")
					}
				}
			}
			if otherCalls != 0 {
				t.Fatal("cross-origin forwarding")
			}
		})
	}
}

func TestBitbucketSharedGitAndOfflineNativePreservation(t *testing.T) {
	root := inReplicationTestRepository(t)
	srv, _ := bbFixture(t, 200, false)
	defer srv.Close()
	cfg := bbConfig(srv.URL)
	cfg.CredentialMode = "api_token_bearer"
	cfg.Token = "explicit-transient-bitbucket-credential"
	api, e := OpenMigrationSource(cfg)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	l := migration.DefaultLimits()
	capture := ComposeMigrationSources(api, LocalMigrationSource{GitDir: filepath.Join(root, ".git")})
	sel := migration.Selection{Source: cfg.Locator, Refs: []string{"refs/heads/main"}, Kinds: []string{"change_request", "inline_comment", "workflow"}}
	snap, blobs, e := capture.Snapshot(ctx, sel, l)
	if e != nil {
		t.Fatal(e)
	}
	defer blobs.(io.Closer).Close()
	if len(snap.NativeRefs) != 1 || len(snap.Records) < 5 {
		t.Fatal("shared Git/code and provider communication composition lost content")
	}
	var archive bytes.Buffer
	if e = migration.WriteArchive(ctx, &archive, snap, blobs, l); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(archive.Bytes(), []byte(cfg.Token)) {
		t.Fatal("credential leaked to archive")
	}
	dest := migrationBare(t)
	d := LocalMigrationDestination{Repository: dest}
	sel.Destination = d.Locator()
	decisions := []migration.Decision{{FindingID: "local/governance", Action: "preserve", Mapping: json.RawMessage(`{}`)}}
	for _, f := range snap.Findings {
		if f.Status != "preserved" && f.Status != "mapped" {
			decisions = append(decisions, migration.Decision{FindingID: f.ID, Action: "acknowledge", Mapping: json.RawMessage(`{}`)})
		}
	}
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "saved"), snap, blobs, sel, d, l, decisions)
	if e != nil {
		t.Fatal(e)
	}
	saved, _ := json.Marshal(session.Plan)
	if bytes.Contains(saved, []byte(cfg.Token)) {
		t.Fatal("credential leaked to plan")
	}
	srv.Close() // All following work uses verified local bytes with provider absent.
	if e = ApplyMigration(ctx, session, d, ""); e != nil {
		t.Fatal(e)
	}
	got, e := ReadLocalMigration(ctx, dest, session.Plan.SnapshotDigest, l)
	if e != nil {
		t.Fatal(e)
	}
	defer got.Close()
	a, _ := migration.CanonicalSnapshot(snap, l)
	b, _ := migration.CanonicalSnapshot(got.Snapshot, l)
	if !bytes.Equal(a, b) {
		t.Fatal("offline native preservation changed attributed records or Git identities")
	}
}
