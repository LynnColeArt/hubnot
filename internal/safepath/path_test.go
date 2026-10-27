package safepath

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func physicalFixture(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSystemAliasRequiresExactBoundaryTargetAndOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix system-alias semantics")
	}
	base := physicalFixture(t)
	target, alias := filepath.Join(base, "physical"), filepath.Join(base, "system-temp")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("physical", alias); err != nil {
		t.Fatal(err)
	}
	trusted := func(os.FileInfo) bool { return true }
	for _, suffix := range []string{"", "/session", "/nested/session"} {
		got, err := canonicalAlias(alias+suffix, alias, target, trusted)
		if err != nil || got != target+suffix {
			t.Fatalf("OS alias did not preserve suffix: %s %v", got, err)
		}
	}
	lookalike := alias + "-untrusted/session"
	if got, err := canonicalAlias(lookalike, alias, target, trusted); err != nil || got != lookalike {
		t.Fatal("prefix lookalike treated as a trusted alias", got, err)
	}
	if _, err := canonicalAlias(alias, alias, target, func(os.FileInfo) bool { return false }); err == nil {
		t.Fatal("untrusted owner accepted")
	}
	if _, err := canonicalAlias(alias, alias, base, trusted); err == nil {
		t.Fatal("unexpected link target accepted")
	}
	// A same-user/configured link is not a production OS alias, even when it
	// resolves to the identical physical directory and permissions are private.
	if err := CheckDir(alias, true); err == nil {
		t.Fatal("arbitrary alias accepted by production directory policy")
	}
	child := filepath.Join(target, "linked-child")
	if err := os.Symlink(base, child); err != nil {
		t.Fatal(err)
	}
	translated, err := canonicalAlias(filepath.Join(alias, "linked-child"), alias, target, trusted)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckDir(translated, false); err == nil {
		t.Fatal("trusted alias exempted an untrusted descendant")
	}
}

func TestPrivateDirectoriesAndTemporarySpools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix private-directory modes")
	}
	base := physicalFixture(t)
	private := filepath.Join(base, "private")
	if err := os.Mkdir(private, 0700); err != nil {
		t.Fatal(err)
	}
	if err := CheckDir(private, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(private, 0755); err != nil {
		t.Fatal(err)
	}
	if err := CheckDir(private, true); err == nil {
		t.Fatal("public session directory accepted")
	}
	if err := CheckDir(private, false); err != nil {
		t.Fatal("ordinary nonprivate parent refused", err)
	}
	t.Setenv("TMPDIR", base)
	dir, err := MkdirTemp("spool-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if filepath.Dir(dir) != base || !strings.HasPrefix(filepath.Base(dir), "spool-") {
		t.Fatal("temporary spool escaped selected physical parent", dir)
	}
	if err := CheckDir(dir, true); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(physicalFixture(t), "configured-temp")
	if err := os.Symlink(base, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", alias)
	before, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if p, err := MkdirTemp("must-not-exist-"); err == nil {
		os.RemoveAll(p)
		t.Fatal("arbitrary configured temporary link accepted")
	}
	after, err := os.ReadDir(base)
	if err != nil || len(before) != len(after) {
		t.Fatal("refused temporary alias still created a spool", err)
	}
}
