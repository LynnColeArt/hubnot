package extension

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"

	"hubnot/internal/safepath"
)

const RegistryLimit = 1048576
const RegistrationLimit = 128
const ExecutableLimit = 134217728

type Registration struct {
	RegistrationID       string   `json:"registration_id"`
	RegistrationInstance string   `json:"registration_instance"`
	Manifest             Manifest `json:"manifest"`
	ManifestDigest       string   `json:"manifest_digest"`
	Executable           string   `json:"executable"`
	ExecutableDigest     string   `json:"executable_digest"`
	Argv                 []string `json:"argv"`
}
type Registry struct {
	Protocol      string         `json:"protocol"`
	Registrations []Registration `json:"registrations"`
}

func (r Registration) identity() string {
	return digest(canonical(map[string]any{"argv": r.Argv, "executable": r.Executable, "executable_digest": r.ExecutableDigest, "manifest_digest": r.ManifestDigest, "registration_instance": r.RegistrationInstance}))
}
func validArgv(argv []string) error {
	if argv == nil {
		return failure("invalid_input", "argv must be an array")
	}
	if len(argv) > 64 {
		return failure("resource_limit", "argv count exceeds 64")
	}
	total := 0
	for _, arg := range argv {
		if !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
			return failure("invalid_input", "argv must contain UTF-8 without NUL")
		}
		if len(arg) > 4096 {
			return failure("resource_limit", "argv value exceeds 4096 bytes")
		}
		total += len(arg)
	}
	if total > 65536 {
		return failure("resource_limit", "argv aggregate exceeds 65536 bytes")
	}
	return nil
}

// NewRegistration prepares and hashes a user-selected descriptor. Register alone
// assigns its private installation instance and final registration ID.
func NewRegistration(m Manifest, executable string, argv []string) (Registration, error) {
	if err := m.validate(); err != nil {
		return Registration{}, err
	}
	if err := validArgv(argv); err != nil {
		return Registration{}, err
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return Registration{}, failure("invalid_input", "invalid executable path")
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return Registration{}, failure("unavailable_module", "selected executable is unavailable")
	}
	hash, err := hashExecutable(physical)
	if err != nil {
		return Registration{}, err
	}
	r := Registration{Manifest: m, ManifestDigest: m.Digest(), Executable: physical, ExecutableDigest: hash, Argv: append([]string{}, argv...)}
	return r, nil
}
func hashExecutable(path string) (string, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", failure("unavailable_module", "selected executable is unavailable")
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", failure("unavailable_module", "selected executable must be a regular executable file")
	}
	if info.Size() > ExecutableLimit {
		return "", failure("resource_limit", "executable exceeds hashing limit")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, ExecutableLimit+1))
	if err != nil {
		return "", failure("unavailable_module", "cannot hash executable")
	}
	if n > ExecutableLimit {
		return "", failure("resource_limit", "executable exceeds hashing limit")
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
func (r Registration) validateStored() error {
	if err := r.validateDescriptor(); err != nil {
		return err
	}
	if !validHex(r.RegistrationInstance, 32) || r.RegistrationID != r.identity() {
		return failure("verification_failure", "registration instance or identity does not match its content")
	}
	return nil
}
func (r Registration) validateDescriptor() error {
	if err := r.Manifest.validateStructure(); err != nil {
		return err
	}
	if err := validArgv(r.Argv); err != nil {
		return err
	}
	if !filepath.IsAbs(r.Executable) || filepath.Clean(r.Executable) != r.Executable || strings.ContainsRune(r.Executable, 0) {
		return failure("invalid_input", "registry executable path must be absolute and clean")
	}
	if !validDigest(r.ExecutableDigest) || r.ManifestDigest != r.Manifest.Digest() {
		return failure("verification_failure", "registration identity does not match its content")
	}
	return nil
}
func validDigest(id string) bool {
	return len(id) == 71 && strings.HasPrefix(id, "sha256:") && validHex(id[7:], 64)
}
func validHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, b := range value {
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f') {
			return false
		}
	}
	return true
}

// Availability hashes files only during explicit inspection. It never probes by
// starting the process, and a valid hash is not observed runtime availability.
func (r Registration) Availability() (status, availability string, err error) {
	if r.Manifest.HostProtocol != HostProtocol {
		return "incompatible", "not_probed", nil
	}
	physical, err := filepath.EvalSymlinks(r.Executable)
	if err != nil {
		return "missing", "unavailable", nil
	}
	if physical != r.Executable {
		return "changed", "unavailable", nil
	}
	hash, err := hashExecutable(r.Executable)
	if err != nil {
		var public *Error
		if errors.As(err, &public) && public.Code == "resource_limit" {
			return "", "", err
		}
		return "changed", "unavailable", nil
	}
	if hash != r.ExecutableDigest {
		return "changed", "unavailable", nil
	}
	return "registered", "not_probed", nil
}

// RegistryPath is resolved only by explicit extension commands.
func RegistryPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", failure("invalid_input", "user configuration directory is unavailable")
	}
	if !filepath.IsAbs(config) {
		return "", failure("invalid_input", "user configuration directory must be absolute")
	}
	path, err := safepath.Canonical(filepath.Join(config, "hubnot", "extensions", "registry.json"))
	if err != nil {
		return "", failure("invalid_input", "unsafe configuration path")
	}
	return path, nil
}
func privateDirectory(path string, create bool) error {
	base := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	if err := safepath.CheckDir(base, false); err != nil {
		if !os.IsNotExist(err) {
			return failure("invalid_input", "unsafe user configuration directory")
		}
		if !create {
			return nil
		}
		ancestor := filepath.Dir(base)
		for {
			err := safepath.CheckDir(ancestor, false)
			if err == nil {
				break
			}
			if !os.IsNotExist(err) || filepath.Dir(ancestor) == ancestor {
				return failure("invalid_input", "unsafe user configuration directory")
			}
			ancestor = filepath.Dir(ancestor)
		}
		if err := os.MkdirAll(base, 0700); err != nil {
			return failure("repository_error", "cannot create private configuration directory")
		}
		if err := safepath.CheckDir(base, false); err != nil {
			return failure("invalid_input", "unsafe user configuration directory")
		}
	}
	for _, dir := range []string{filepath.Join(base, "hubnot"), filepath.Dir(path)} {
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			if !create {
				return nil
			}
			if err = os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
				return failure("repository_error", "cannot create private extension directory")
			}
			info, err = os.Lstat(dir)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return failure("invalid_input", "unsafe private extension directory")
		}
		if info.Mode().Perm() != 0700 {
			return failure("invalid_input", "private extension directories must have mode 0700")
		}
	}
	return nil
}
func LoadRegistry(path string) (Registry, error) {
	empty := Registry{Protocol: "hn.extension-registry/1", Registrations: []Registration{}}
	if err := privateDirectory(path, false); err != nil {
		return empty, err
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		return empty, nil
	}
	if err != nil {
		return empty, failure("invalid_input", "cannot safely open extension registry")
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return empty, failure("invalid_input", "registry must be a private regular file with mode 0600")
	}
	data, err := io.ReadAll(io.LimitReader(f, RegistryLimit+1))
	if err != nil {
		return empty, failure("invalid_input", "cannot read extension registry")
	}
	value, err := strictJSON(data, RegistryLimit)
	if err != nil {
		return empty, err
	}
	encoded, _ := json.Marshal(value)
	var registry Registry
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&registry) != nil || registry.Protocol != "hn.extension-registry/1" || registry.Registrations == nil {
		return empty, failure("invalid_input", "invalid extension registry")
	}
	object, ok := value.(map[string]any)
	if !ok || !exactFields(object, "protocol", "registrations") {
		return empty, failure("invalid_input", "invalid extension registry fields")
	}
	if len(registry.Registrations) > RegistrationLimit {
		return empty, failure("resource_limit", "too many registrations")
	}
	seen := map[string]bool{}
	for i, r := range registry.Registrations {
		raw := object["registrations"].([]any)[i]
		fields, ok := raw.(map[string]any)
		if !ok || !exactFields(fields, "registration_id", "registration_instance", "manifest", "manifest_digest", "executable", "executable_digest", "argv") {
			return empty, failure("invalid_input", "invalid registration fields")
		}
		manifestBytes, _ := json.Marshal(fields["manifest"])
		if _, err := parseManifest(manifestBytes, false); err != nil {
			return empty, err
		}
		if err := r.validateStored(); err != nil {
			return empty, err
		}
		if seen[r.Manifest.ModuleID] {
			return empty, failure("invalid_input", "duplicate registered module")
		}
		seen[r.Manifest.ModuleID] = true
	}
	return registry, nil
}

// UpdateRegistry serializes explicit local edits and atomically replaces a
// bounded private file. Malformed input is never repaired or overwritten.
func UpdateRegistry(path string, update func(*Registry) error) error {
	if err := privateDirectory(path, true); err != nil {
		return err
	}
	fd, err := syscall.Open(filepath.Join(filepath.Dir(path), "registry.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return failure("repository_error", "cannot lock extension registry")
	}
	lock := os.NewFile(uintptr(fd), "registry.lock")
	defer lock.Close()
	info, err := lock.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return failure("invalid_input", "unsafe registry lock")
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return failure("repository_error", "extension registry is busy")
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	registry, err := LoadRegistry(path)
	if err != nil {
		return err
	}
	if err = update(&registry); err != nil {
		return err
	}
	if len(registry.Registrations) > RegistrationLimit {
		return failure("resource_limit", "too many registrations")
	}
	sort.Slice(registry.Registrations, func(i, j int) bool {
		return registry.Registrations[i].Manifest.ModuleID < registry.Registrations[j].Manifest.ModuleID
	})
	data := canonical(registry)
	if len(data) > RegistryLimit {
		return failure("resource_limit", "registry exceeds byte limit")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".registry-*")
	if err != nil {
		return failure("repository_error", "cannot stage registry")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return failure("repository_error", "cannot persist registry")
	}
	if err = os.Rename(name, path); err != nil {
		return failure("repository_error", "cannot replace registry")
	}
	return nil
}

// Register installs an explicitly prepared descriptor. Lifecycle fields supplied
// by the caller are ignored: only this registry operation chooses an instance.
// An exact active duplicate returns the stored registration without reading
// entropy. Removal followed by registration always creates a fresh binding.
func Register(path string, candidate Registration) (Registration, error) {
	return registerWithRandom(path, candidate, rand.Reader)
}
func registerWithRandom(path string, candidate Registration, entropy io.Reader) (Registration, error) {
	if err := candidate.Manifest.validate(); err != nil {
		return Registration{}, err
	}
	if err := candidate.validateDescriptor(); err != nil {
		return Registration{}, err
	}
	var installed Registration
	err := UpdateRegistry(path, func(registry *Registry) error {
		for _, old := range registry.Registrations {
			if old.Manifest.ModuleID != candidate.Manifest.ModuleID {
				continue
			}
			oldBinding := canonical(map[string]any{"argv": old.Argv, "executable": old.Executable, "executable_digest": old.ExecutableDigest, "manifest_digest": old.ManifestDigest})
			newBinding := canonical(map[string]any{"argv": candidate.Argv, "executable": candidate.Executable, "executable_digest": candidate.ExecutableDigest, "manifest_digest": candidate.ManifestDigest})
			if bytes.Equal(oldBinding, newBinding) {
				installed = old
				return nil
			}
			return failure("invalid_input", "module is already registered; explicitly remove it before registering changed content")
		}
		if len(registry.Registrations) >= RegistrationLimit {
			return failure("resource_limit", "too many registrations")
		}
		var instance [16]byte
		if _, err := io.ReadFull(entropy, instance[:]); err != nil {
			return failure("repository_error", "cannot obtain registration randomness")
		}
		candidate.RegistrationInstance = hex.EncodeToString(instance[:])
		candidate.RegistrationID = candidate.identity()
		installed = candidate
		registry.Registrations = append(registry.Registrations, installed)
		return nil
	})
	if err != nil {
		return Registration{}, err
	}
	return installed, nil
}

func Remove(path, module string) (Registration, error) {
	var removed Registration
	err := UpdateRegistry(path, func(registry *Registry) error {
		for i, r := range registry.Registrations {
			if r.Manifest.ModuleID == module {
				removed = r
				registry.Registrations = append(registry.Registrations[:i], registry.Registrations[i+1:]...)
				return nil
			}
		}
		return &Error{Origin: "host", Code: "not_found", Message: "module is not registered", Recovery: "do_not_retry"}
	})
	return removed, err
}
