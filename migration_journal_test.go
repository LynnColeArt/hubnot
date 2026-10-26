package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hubnot/internal/migration"
)

func TestMigrationJournalTamperLockAndUnsafePaths(t *testing.T) {
	ctx := context.Background()
	snap, bs := migrationForeignSnapshot()
	d := &migrationReceiptDestination{locator: migration.Locator{Kind: "fixture", Repository: "d"}}
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), snap, bs, migration.Selection{Destination: d.locator}, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	unlock, e := migrationLock(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplyMigration(ctx, s, d, ""); e == nil {
		t.Fatal("concurrent apply escaped lock")
	}
	unlock()
	if d.creates != 0 {
		t.Fatal("locked mutation")
	}
	raw, e := os.ReadFile(filepath.Join(s.Directory, "plan.json"))
	if e != nil {
		t.Fatal(e)
	}
	for i := range raw {
		if raw[i] == 'T' {
			raw[i] = 'U'
			break
		}
	}
	if e = os.WriteFile(filepath.Join(s.Directory, "plan.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = OpenMigrationSession(ctx, s.Directory); e == nil {
		t.Fatal("tampered session accepted")
	}
	outside := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if e = os.Symlink(outside, alias); e != nil {
		t.Fatal(e)
	}
	if _, e = PrepareMigration(ctx, filepath.Join(alias, "saved"), snap, bs, migration.Selection{Destination: d.locator}, d, migration.DefaultLimits(), nil); e == nil {
		t.Fatal("symlink parent accepted")
	}
	if _, e = os.Lstat(filepath.Join(outside, "saved")); !os.IsNotExist(e) {
		t.Fatal("unsafe parent mutated")
	}
}

func (d *migrationReceiptDestination) Locator() migration.Locator { return d.locator }
func TestMigrationMissingCheckpointNeverReplaysCreates(t *testing.T) {
	ctx := context.Background()
	snap, blobs := migrationForeignSnapshot()
	d := &migrationReceiptDestination{locator: migration.Locator{Kind: "fixture", Repository: "planned"}}
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "session"), snap, blobs, migration.Selection{Destination: d.locator}, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Lstat(filepath.Join(s.Directory, "progress.json")); e != nil {
		t.Fatal("initial checkpoint absent", e)
	}
	if e = ApplyMigration(ctx, s, d, "operator"); e == nil {
		t.Fatal("expected uncertain comment")
	}
	if e = os.Remove(filepath.Join(s.Directory, "progress.json")); e != nil {
		t.Fatal(e)
	}
	if _, e = OpenMigrationSession(ctx, s.Directory); e == nil {
		t.Fatal("missing recovery evidence treated as new session")
	}
	if e = ApplyMigration(ctx, s, d, "operator"); e == nil {
		t.Fatal("missing progress replayed migration")
	}
	if d.creates != 1 || d.comments != 1 {
		t.Fatal("uncertain writes repeated after missing checkpoint")
	}
}
func TestMigrationDestinationChangeRefusedBeforeAnyEffect(t *testing.T) {
	ctx := context.Background()
	snap, blobs := migrationForeignSnapshot()
	planned := migration.Locator{Kind: "fixture", Repository: "planned"}
	d := &migrationReceiptDestination{locator: planned}
	s, e := PrepareMigration(ctx, filepath.Join(t.TempDir(), "saved"), snap, blobs, migration.Selection{Destination: planned}, d, migration.DefaultLimits(), nil)
	if e != nil {
		t.Fatal(e)
	}
	d.locator.Repository = "changed"
	if e = ApplyMigration(ctx, s, d, ""); e == nil {
		t.Fatal("changed destination applied")
	}
	if d.creates+d.comments != 0 {
		t.Fatal("destination mismatch detected after effect")
	}
	if _, e = PrepareMigration(ctx, filepath.Join(t.TempDir(), "new"), snap, blobs, migration.Selection{Destination: planned}, d, migration.DefaultLimits(), nil); e == nil {
		t.Fatal("changed destination planned")
	}
	code := GitMigrationDestination{Remote: migrationBare(t), Destination: planned}
	composed := ComposeMigrationDestinations(d, code)
	if composed.Locator() == planned {
		t.Fatal("inconsistent composed destination identity hidden")
	}
	if e = ApplyMigration(ctx, s, composed, ""); e == nil {
		t.Fatal("inconsistent composed destination applied")
	}
	if d.creates+d.comments != 0 {
		t.Fatal("composed identity mismatch allowed effect")
	}
}
