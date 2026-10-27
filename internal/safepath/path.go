// Package safepath defines the filesystem boundary for migration state and
// temporary spools. OS-owned aliases are distinct from user-controlled links.
package safepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Canonical makes a path absolute and translates only verified platform-owned
// aliases. It never evaluates arbitrary symlinks in the selected path. Call
// CheckDir before accessing an existing directory; a new leaf need not exist.
func Canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return systemAliases(abs)
}

// canonicalAlias is deliberately private: callers cannot declare trusted links.
// Only the platform implementation supplies the fixed aliases and owner check.
func canonicalAlias(path, alias, target string, owner func(os.FileInfo) bool) (string, error) {
	if path != alias && !strings.HasPrefix(path, alias+string(filepath.Separator)) {
		return path, nil
	}
	info, err := os.Lstat(alias)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return path, nil
	}
	if info.Mode()&os.ModeSymlink == 0 || !owner(info) {
		return "", fmt.Errorf("unsafe system path alias %s", alias)
	}
	link, err := os.Readlink(alias)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(alias), link)
	}
	if filepath.Clean(link) != target {
		return "", fmt.Errorf("unexpected system path alias %s", alias)
	}
	return target + strings.TrimPrefix(path, alias), nil
}

// CheckDir rejects links at every remaining component. Private directories must
// also be inaccessible to group/other users. Canonical's OS alias exception
// never exempts descendants, including links pointing back inside the same root.
func CheckDir(path string, private bool) error {
	abs, err := Canonical(path)
	if err != nil {
		return err
	}
	for p := abs; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink/unsafe directory %s", p)
		}
		if private && p == abs && info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("directory must be private: %s", p)
		}
		if filepath.Dir(p) == p {
			return nil
		}
	}
}

// MkdirTemp selects and verifies a physical temporary parent before creation.
// It also works when a closed child environment has no TMPDIR at all. Custom
// TMPDIR links are not trusted merely because the environment selected them.
func MkdirTemp(pattern string) (string, error) {
	parent, err := Canonical(os.TempDir())
	if err != nil {
		return "", err
	}
	if err = CheckDir(parent, false); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		return "", err
	}
	if err = CheckDir(dir, true); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
