package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/migration"
)

// Run the real session + imported-record spool path with literal OS aliases and
// a child environment that has no TMPDIR. Do not fix this test by physicalizing
// its inputs, forwarding TMPDIR, or changing the existing public journey.
func TestMigrationMacSystemAliasesAndDefaultTemp(t *testing.T) {
	const child = "HN_TEST_MAC_DEFAULT_TEMP"
	if os.Getenv(child) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestMigrationMacSystemAliasesAndDefaultTemp$", "-test.v")
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "TMPDIR=") && !strings.HasPrefix(v, child+"=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, child+"=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("default-environment Mac migration: %v\n%s", err, out)
		}
		return
	}
	if _, exists := os.LookupEnv("TMPDIR"); exists {
		t.Fatal("regression child inherited a temp-directory workaround")
	}
	for _, parent := range []string{"/tmp", "/var/tmp"} {
		t.Run(parent, func(t *testing.T) {
			root, err := os.MkdirTemp(parent, "hn-mac-alias-test-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(root)
			ctx := context.Background()
			limits := migration.DefaultLimits()
			dest := filepath.Join(root, "destination")
			if _, err = projectGit(ctx, "", nil, 4096, "init", "--quiet", "--bare", dest); err != nil {
				t.Fatal(err)
			}
			foreign, blobs := migrationForeignSnapshot()
			d := LocalMigrationDestination{Repository: dest}
			session, err := PrepareMigration(ctx, filepath.Join(root, "session"), foreign, blobs, migration.Selection{Destination: d.Locator()}, d, limits, []migration.Decision{{FindingID: "local/governance", Action: "preserve", Mapping: json.RawMessage(`{}`)}})
			if err != nil {
				t.Fatal(err)
			}
			if err = ApplyMigration(ctx, session, d, ""); err != nil {
				t.Fatal(err)
			}
			// Reopen using the original alias, not the canonical session path.
			if _, err = OpenMigrationSession(ctx, filepath.Join(root, "session")); err != nil {
				t.Fatal(err)
			}
			gitdir, err := migrationDestinationGitDir(ctx, dest)
			if err != nil {
				t.Fatal(err)
			}
			source := ImportedMigrationSource{GitDir: gitdir, SnapshotDigests: []string{session.Plan.SnapshotDigest}}
			observed, spool, err := source.Snapshot(ctx, migration.Selection{}, limits)
			if err != nil {
				t.Fatal(err)
			}
			defer spool.(io.Closer).Close()
			archive := ArchiveMigrationDestination{Path: filepath.Join(root, "continued.hn")}
			export, err := PrepareMigration(ctx, filepath.Join(root, "export"), observed, spool, migration.Selection{Destination: archive.Locator()}, archive, limits, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = ApplyMigration(ctx, export, archive, ""); err != nil {
				t.Fatal(err)
			}
			restored, restoredBlobs, err := (ArchiveMigrationSource{Path: archive.Path}).Snapshot(ctx, migration.Selection{}, limits)
			if err != nil {
				t.Fatal(err)
			}
			defer restoredBlobs.(io.Closer).Close()
			if len(restored.Records) != len(foreign.Records) || len(restored.Blobs) != len(foreign.Blobs) {
				t.Fatal("alias/default-temp round trip lost records or blobs")
			}
			for _, b := range restored.Blobs {
				r, err := restoredBlobs.Open(ctx, b.Digest)
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(r)
				r.Close()
				if err != nil || string(data) != string(blobs[b.Digest]) {
					t.Fatal("alias/default-temp round trip changed blob bytes", err)
				}
			}
		})
	}
}
