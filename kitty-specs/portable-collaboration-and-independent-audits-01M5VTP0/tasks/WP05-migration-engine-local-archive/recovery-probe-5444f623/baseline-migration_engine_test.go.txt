package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/migration"
)

type migrationMemoryBlobs map[string][]byte

func (b migrationMemoryBlobs) Open(_ context.Context, d string) (io.ReadCloser, error) {
	v, ok := b[d]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(string(v))), nil
}
func migrationForeignSnapshot() (migration.Snapshot, migrationMemoryBlobs) {
	data := []byte("retrieved original workflow: inert")
	digest := migration.Digest(data)
	key := migration.Key{Provider: "fixture", Instance: "https://fixture.invalid", ProjectID: "p1", Kind: "workflow", ID: "w1"}
	s := migration.Snapshot{Schema: "hn.project-snapshot/1", Source: migration.Locator{Kind: "fixture", Endpoint: "https://fixture.invalid", Repository: "p1"}, SourceID: "immutable-project-p1", Records: []migration.Record{{Key: key, Revision: "r1", Schema: "hn.migration.preserved/1", Content: json.RawMessage(`{"schema":"hn.migration.preserved/1","summary":"Inert source","source_blob":"` + digest + `"}`), Links: []migration.Link{}, Blobs: []string{digest}}}, Blobs: []migration.Blob{{Digest: digest, Size: int64(len(data)), Name: "workflow", MediaType: "text/plain"}}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}
	issueKey := migration.Key{Provider: "fixture", Instance: "https://fixture.invalid", ProjectID: "p1", Kind: "issue", ID: "42"}
	commentKey := issueKey
	commentKey.Kind = "comment"
	commentKey.ID = "43"
	s.Records = append(s.Records, migration.Record{Key: issueKey, Revision: "i1", Author: "source author", CreatedAt: "2026-10-26T00:00:00Z", Schema: "hn.migration.issue/1", Content: json.RawMessage(`{"schema":"hn.migration.issue/1","title":"Foreign issue","body":"Original body","state":"open","type":"","labels":[],"assignees":[]}`), Links: []migration.Link{}, Blobs: []string{}}, migration.Record{Key: commentKey, Revision: "c1", Author: "source commenter", Schema: "hn.migration.comment/1", Content: json.RawMessage(`{"schema":"hn.migration.comment/1","body":"Source-only conversation"}`), Links: []migration.Link{{Relation: "issue", Target: issueKey, Revision: "i1"}}, Blobs: []string{digest}})
	return s, migrationMemoryBlobs{digest: data}
}

type migrationReceiptDestination struct {
	locator        migration.Locator
	creates        int
	comments       int
	exact          bool
	commentOK      bool
	commentPayload string
	blobDigest     string
	blobReads      int
	commentReceipt migration.Receipt
}

func (d *migrationReceiptDestination) Capabilities(context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "issue", Create: true}, {Kind: "comment", Create: true}}, nil
}
func (d *migrationReceiptDestination) Plan(context.Context, migration.Snapshot, migration.Selection, migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	return []migration.Intent{{ID: "issue", Kind: "issue", Action: "create", Sources: []migration.Key{}, Payload: json.RawMessage(`{"schema":"hn.migration.issue-write/1","title":"Title","body":"","state":"open","type":"","labels":[],"assignees":[]}`), DependsOn: []string{}}, {ID: "comment", Kind: "comment", Action: "create", Sources: []migration.Key{}, Payload: json.RawMessage(`{"schema":"hn.migration.comment-write/1","parent":{"receipt":{"intent_id":"issue","field":"remote_id"}},"body":"Comment"}`), DependsOn: []string{"issue"}}}, []migration.Finding{}, nil
}
func (d *migrationReceiptDestination) Apply(_ context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	// The engine supplies an immutable plan copy even to an adapter that reuses
	// or edits its incoming template buffer while translating a DTO.
	if len(in.Payload) > 0 {
		in.Payload[0] = 'x'
	}
	if d.blobDigest != "" {
		if e := d.checkBlob(ac); e != nil {
			return migration.Receipt{}, e
		}
	}
	if in.ID == "issue" {
		d.creates++
		return migrationReceipt(in, ac, d.locator, "4711", "r1"), nil
	}
	d.comments++
	r, ok := ac.Dependency("issue")
	if !ok || r.RemoteID != "4711" {
		return migration.Receipt{}, errors.New("missing confirmed issue")
	}
	d.commentPayload = string(ac.BoundWrite().Payload)
	if !strings.Contains(d.commentPayload, `"parent":{"id":"4711"}`) {
		return migration.Receipt{}, errors.New("unresolved parent")
	}
	b := ac.BoundWrite()
	b.Payload[0] = 'x'
	if string(ac.BoundWrite().Payload) != d.commentPayload {
		return migration.Receipt{}, errors.New("mutable payload escaped")
	}
	d.commentReceipt = migrationReceipt(in, ac, d.locator, "9001", "r2")
	if d.commentOK {
		return d.commentReceipt, nil
	}
	return migration.Receipt{}, io.ErrUnexpectedEOF
}
func (d *migrationReceiptDestination) Reconcile(_ context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	if d.blobDigest != "" {
		if e := d.checkBlob(ac); e != nil {
			return migration.Reconciliation{}, e
		}
	}
	if string(ac.BoundWrite().Payload) != d.commentPayload {
		return migration.Reconciliation{}, errors.New("resume changed actual payload")
	}
	r, ok := ac.Dependency("issue")
	if !ok || r.RemoteID != "4711" {
		return migration.Reconciliation{}, errors.New("resume changed parent")
	}
	if !d.exact {
		return migration.Reconciliation{Status: "no_proof", Reason: "negative lookup"}, nil
	}
	return migration.Reconciliation{Status: "exact", Receipt: &d.commentReceipt}, nil
}
func TestMigrationPersistedCommentBindingAndNegativeLookupNeverRetries(t *testing.T) {
	ctx := context.Background()
	snap, blobs := migrationForeignSnapshot()
	d := &migrationReceiptDestination{locator: migration.Locator{Kind: "fixture", Repository: "destination"}}
	d.blobDigest = snap.Blobs[0].Digest
	t.Setenv("HN_MIGRATION_SECRET_SENTINEL", "private-ambient-value")
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), snap, blobs, migration.Selection{Destination: d.locator}, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(s.Plan)
	if strings.Contains(string(raw), "private-ambient-value") {
		t.Fatal("ambient secret leaked")
	}
	if e = ApplyMigration(ctx, s, d, "operator"); e == nil {
		t.Fatal("expected uncertain comment")
	}
	if d.creates != 1 || d.comments != 1 {
		t.Fatal("wrong number writes")
	}
	// Original blobs are no longer available; saved verified spool owns resume.
	for k := range blobs {
		delete(blobs, k)
	}
	s, e = OpenMigrationSession(ctx, s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, s, d, "operator"); e == nil || !strings.Contains(e.Error(), "manual recovery") {
		t.Fatal("negative lookup authorized retry", e)
	}
	if d.creates != 1 || d.comments != 1 {
		t.Fatal("repeated uncertain create")
	}
	d.exact = true
	if e = ApplyMigration(ctx, s, d, "operator"); e != nil {
		t.Fatal(e)
	}
	if d.creates != 1 || d.comments != 1 {
		t.Fatal("repeated confirmed create")
	}
	if d.blobReads < 4 {
		t.Fatal("blob access not supplied to apply and reconciliation")
	}
	if e = ApplyMigration(ctx, s, d, "other-actor"); e == nil {
		t.Fatal("actor changed")
	}
}
func TestMigrationVerifiedBlobRefusalBeforeWrites(t *testing.T) {
	ctx := context.Background()
	snap, blobs := migrationForeignSnapshot()
	d := &migrationReceiptDestination{locator: migration.Locator{Kind: "fixture", Repository: "d"}}
	blobs[snap.Blobs[0].Digest] = []byte("corrupt")
	if _, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), snap, blobs, migration.Selection{Destination: d.locator}, d, migration.DefaultLimits(), nil); e == nil {
		t.Fatal("corrupt source saved")
	}
	snap, blobs = migrationForeignSnapshot()
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), snap, blobs, migration.Selection{Destination: d.locator}, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(s.Directory, "blobs", snap.Blobs[0].Digest[7:]), []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, s, d, ""); e == nil {
		t.Fatal("corrupt saved blob applied")
	}
	if d.creates+d.comments != 0 {
		t.Fatal("mutation before verification")
	}
}

func (d *migrationReceiptDestination) checkBlob(ac migration.ApplyContext) error {
	r, b, e := ac.OpenBlob(context.Background(), d.blobDigest)
	if e != nil {
		return e
	}
	raw, e := io.ReadAll(r)
	r.Close()
	if e != nil || migration.Digest(raw) != b.Digest || int64(len(raw)) != b.Size {
		return errors.New("unverified context blob")
	}
	if _, _, e = ac.OpenBlob(context.Background(), "../../private"); e == nil {
		return errors.New("arbitrary blob access")
	}
	d.blobReads++
	return nil
}
