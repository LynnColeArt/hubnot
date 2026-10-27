package safepath

import (
	"os"
	"syscall"
)

func systemAliases(path string) (string, error) {
	// These exact top-level entries are protected by the OS, not configured by
	// the repository or TMPDIR. Do not add user home/checkout links here.
	for _, pair := range [][2]string{{"/var", "/private/var"}, {"/tmp", "/private/tmp"}} {
		var err error
		path, err = canonicalAlias(path, pair[0], pair[1], func(info os.FileInfo) bool {
			st, ok := info.Sys().(*syscall.Stat_t)
			return ok && st.Uid == 0
		})
		if err != nil {
			return "", err
		}
	}
	return path, nil
}
