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
