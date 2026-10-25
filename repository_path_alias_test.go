package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrivatePathContainmentThroughTemporaryDirectoryAlias(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix temporary-directory aliases use TMPDIR")
	}
	parent := t.TempDir()
	physical := filepath.Join(parent, "physical")
	alias := filepath.Join(parent, "alias")
	if err := os.Mkdir(physical, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	// Reproduce macOS /var -> /private/var on any Unix test host. Git resolves
	// the physical directory while TempDir retains the TMPDIR spelling.
	t.Setenv("TMPDIR", alias)
	t.Run("identity", TestNamespaceCutoverPrivatePaths)
	t.Run("replication", TestReplicationSelectionRoundTripAndRejectsAmbiguity)
}
