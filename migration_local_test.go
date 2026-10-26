package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"hubnot/internal/migration"
)

func TestMigrationLocalForeignRecordsOfflineAndArchiveRoundTrip(t *testing.T) {
	ctx := context.Background()
	snap, blobs := migrationForeignSnapshot()
	l := migration.DefaultLimits()
	dest := migrationBare(t)
	d := LocalMigrationDestination{dest}
	session, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "local-plan"), snap, blobs, migration.Selection{Destination: migration.Locator{Kind: "local", Repository: dest}}, d, l, []migration.Decision{{FindingID: "local/governance", Action: "preserve", Mapping: json.RawMessage(`{}`)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, session, d, ""); e != nil {
		t.Fatal(e)
	}
	a, e := ReadLocalMigration(ctx, dest, session.Plan.SnapshotDigest, l)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if len(a.Snapshot.Records) != 3 {
		t.Fatal("foreign attribution lost")
	}
	found := false
	for _, r := range a.Snapshot.Records {
		if r.Key.Kind == "comment" {
			found = r.Author == "source commenter" && len(r.Links) == 1 && r.Links[0].Target.ID == "42" && len(r.Blobs) == 1
		}
	}
	if !found {
		t.Fatal("source conversation topology/attribution lost")
	}
	r, e := a.Open(ctx, snap.Blobs[0].Digest)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(r)
	r.Close()
	if string(raw) != string(blobs[snap.Blobs[0].Digest]) {
		t.Fatal("retrieved bytes changed")
	}
	output := filepath.Join(t.TempDir(), "project.tar")
	ad := ArchiveMigrationDestination{output}
	as, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "archive-plan"), a.Snapshot, a, migration.Selection{Destination: migration.Locator{Kind: "archive", Repository: output}}, ad, l, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, as, ad, ""); e != nil {
		t.Fatal(e)
	}
	source := ArchiveMigrationSource{output}
	got, bs, e := source.Snapshot(ctx, migration.Selection{}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer bs.(*migration.Archive).Close()
	b1, _ := migration.CanonicalSnapshot(got, l)
	b2, _ := migration.CanonicalSnapshot(snap, l)
	if string(b1) != string(b2) {
		t.Fatal("archive changed snapshot")
	}
	if _, _, e = ad.Plan(ctx, snap, migration.Selection{Destination: migration.Locator{Kind: "archive", Repository: output}}, l); e == nil {
		t.Fatal("archive overwrite planned")
	}
}
func TestMigrationLocalStaleRefPreservesVisibleState(t *testing.T) {
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
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "saved"), p.Snapshot, p, migration.Selection{Destination: migration.Locator{Kind: "local", Repository: dest}}, d, l, []migration.Decision{{FindingID: "local/governance", Action: "preserve", Mapping: json.RawMessage(`{}`)}})
	if e != nil {
		t.Fatal(e)
	}
	// A changed ref after the preview invalidates the absent baseline before any
	// import archive or accepted observation is installed.
	ref, _ := migrationObservationRef(p.Snapshot.SourceID, p.Snapshot.NativeRefs[0])
	other, e := projectGit(ctx, dest, []byte("unrelated"), 256, "hash-object", "-w", "--stdin")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = projectGit(ctx, dest, nil, 256, "update-ref", ref, stringTrim(other)); e != nil {
		t.Fatal(e)
	}
	before := migrationObjectFiles(t, dest)
	if e = ApplyMigration(ctx, s, d, ""); e == nil {
		t.Fatal("stale observation applied")
	}
	after := migrationObjectFiles(t, dest)
	if len(before) != len(after) {
		t.Fatal("stale refusal copied objects")
	}
	if _, e = os.Lstat(filepath.Join(dest, "hn", "imports")); !os.IsNotExist(e) {
		t.Fatal("stale refusal published content")
	}
	got, _ := projectGit(ctx, dest, nil, 256, "show-ref", "--verify", "--hash", ref)
	if string(got) != string(other) {
		t.Fatal("stale ref overwritten")
	}
}
func stringTrim(b []byte) string {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return string(b[:len(b)-1])
	}
	return string(b)
}
func TestMigrationLocalContinuationUsesOwnSignerAndKeepsDestinationPolicy(t *testing.T) {
	source := inReplicationTestRepository(t)
	id := testIdentity(t, "source author")
	event := newEvent(id, "issue.open", 1, "")
	event.Title = "imported issue"
	stored, e := appendEvent(event, id)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	l := migration.DefaultLimits()
	p, e := ExportProjectSnapshot(ctx, filepath.Join(source, ".git"), []string{actorRef(id.Actor)}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	dest := t.TempDir()
	if e = os.Chdir(dest); e != nil {
		t.Fatal(e)
	}
	mustGit(t, "init", "-q", "-b", "local")
	mustGit(t, "config", "user.name", "Local")
	mustGit(t, "config", "user.email", "local@invalid")
	if e = os.Mkdir(".hn", 0700); e != nil {
		t.Fatal(e)
	}
	policy := []byte("destination policy must survive byte-for-byte")
	if e = os.WriteFile(".hn/policy.json", policy, 0600); e != nil {
		t.Fatal(e)
	}
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-qm", "destination policy")
	d := LocalMigrationDestination{dest}
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "saved"), p.Snapshot, p, migration.Selection{Destination: migration.Locator{Kind: "local", Repository: dest}}, d, l, []migration.Decision{{FindingID: "local/governance", Action: "preserve", Mapping: json.RawMessage(`{}`)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, s, d, ""); e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(".hn/policy.json")
	if e != nil || string(got) != string(policy) {
		t.Fatal("destination policy changed")
	}
	events, e := collectEvents()
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, v := range events {
		if v.ID == stored.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("imported native issue not inspectable")
	}
	own := testIdentity(t, "local author")
	reply := newEvent(own, "issue.comment", 1, "")
	reply.Subject = stored.ID
	reply.Body = "offline continuation"
	if _, e = appendEvent(reply, own); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Lstat(filepath.Join(dest, ".git", "hn", "audit", "authority.json")); !os.IsNotExist(e) {
		t.Fatal("import activated audit authority")
	}
}
