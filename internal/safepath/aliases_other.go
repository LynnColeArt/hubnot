//go:build !darwin

package safepath

func systemAliases(path string) (string, error) { return path, nil }
