package extension

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"hubnot/internal/safepath"
)

const GrantLimit = 256

// Grant is explicit operator authority, separate from manifest declarations.
type Grant struct {
	Selector string   `json:"selector"`
	Actions  []string `json:"actions"`
}
type EnablementEntry struct {
	RegistrationID string  `json:"registration_id"`
	RepositoryID   string  `json:"repository_id"`
	Epoch          string  `json:"epoch"`
	Enabled        bool    `json:"enabled"`
	Grants         []Grant `json:"grants"`
}
type Enablements struct {
	Protocol string            `json:"protocol"`
	Entries  []EnablementEntry `json:"entries"`
}

// Binding contains host-selected local identities, never module request fields.
// Epoch pins a session to the exact enablement observed at its creation.
type Binding struct{ RegistrationID, RepositoryID, Epoch string }

// Access is a normalized method scope. Method-specific request parsers must
// derive selector/action from their closed params, never accept them as overrides.
type Access struct {
	Method   string `json:"method"`
	Selector string `json:"selector"`
	Action   string `json:"action"`
}

var nativeFamilies = map[string]bool{"issue": true, "project": true, "project-item": true, "discussion": true, "discussion-reply": true, "workflow": true}
var grantActions = map[string]bool{"read": true, "observe": true, "create": true, "revise": true, "resolve": true, "type.publish": true, "raw.inspect": true}

func validSelector(selector string, wildcard bool) bool {
	if family, ok := strings.CutPrefix(selector, "native:"); ok {
		return nativeFamilies[family] || family == "identity"
	}
	typeID, ok := strings.CutPrefix(selector, "artifact:")
	if !ok {
		return false
	}
	if typeID == "opaque" {
		return true
	}
	if wildcard && strings.HasSuffix(typeID, "/*") {
		typeID = strings.TrimSuffix(typeID, "/*") + "/type"
	}
	return ValidModuleID(typeID)
}

func ValidateGrants(grants []Grant) error {
	if grants == nil {
		return failure("invalid_input", "grants must be an array")
	}
	if len(grants) > GrantLimit {
		return failure("resource_limit", "too many grants")
	}
	seen := map[string]bool{}
	for _, grant := range grants {
		if !validSelector(grant.Selector, true) || seen[grant.Selector] || len(grant.Actions) < 1 || len(grant.Actions) > 7 {
			return failure("invalid_input", "invalid or duplicate grant selector/actions")
		}
		seen[grant.Selector] = true
		actions := map[string]bool{}
		for _, action := range grant.Actions {
			if !grantActions[action] || actions[action] {
				return failure("invalid_input", "unknown or duplicate grant action")
			}
			actions[action] = true
			if grant.Selector == "native:identity" && action != "read" || grant.Selector == "artifact:opaque" && action != "raw.inspect" || strings.HasPrefix(grant.Selector, "native:") && (action == "type.publish" || action == "raw.inspect") {
				return failure("invalid_input", "action is not supported on this selector")
			}
		}
	}
	return nil
}

func ParseGrant(value string) (Grant, error) {
	selector, actions, ok := strings.Cut(value, "=")
	grant := Grant{selector, strings.Split(actions, ",")}
	if !ok {
		return Grant{}, failure("invalid_input", "grant requires SELECTOR=ACTION[,ACTION]")
	}
	if err := ValidateGrants([]Grant{grant}); err != nil {
		return Grant{}, err
	}
	return grant, nil
}

// NativeAction pins the existing native intents; comment is issue create.
func NativeAction(family, intent string) (string, error) {
	if !nativeFamilies[family] {
		return "", failure("invalid_input", "unknown native family")
	}
	switch intent {
	case "open":
		return "create", nil
	case "revise", "resolve":
		return intent, nil
	case "comment":
		if family == "issue" {
			return "create", nil
		}
	}
	return "", failure("invalid_input", "unsupported native intent")
}

func methodAllows(access Access) bool {
	native := strings.HasPrefix(access.Selector, "native:") && access.Selector != "native:identity"
	custom := strings.HasPrefix(access.Selector, "artifact:")
	switch access.Method {
	case "capabilities.list":
		return access.Selector == "" && access.Action == ""
	case "identity.public":
		return access.Selector == "native:identity" && access.Action == "read"
	case "resource.read", "resource.operation":
		return native && access.Action == "read"
	case "resource.mutate":
		if !native {
			return false
		}
		family := strings.TrimPrefix(access.Selector, "native:")
		for _, intent := range []string{"open", "revise", "resolve", "comment"} {
			action, err := NativeAction(family, intent)
			if err == nil && action == access.Action {
				return true
			}
		}
	case "artifact.read":
		if access.Selector == "artifact:opaque" {
			return access.Action == "raw.inspect"
		}
		return custom && access.Action == "read"
	case "artifact.operation", "artifact.search", "artifact.type.read":
		return custom && access.Selector != "artifact:opaque" && access.Action == "read"
	case "artifact.mutate":
		return custom && access.Selector != "artifact:opaque" && (access.Action == "create" || access.Action == "revise" || access.Action == "resolve")
	case "artifact.type.publish":
		return custom && access.Selector != "artifact:opaque" && access.Action == "type.publish"
	case "changes.capture", "changes.page", "changes.since":
		return (native || custom && access.Selector != "artifact:opaque") && (access.Action == "observe" || access.Action == "read")
	}
	return false
}

func grantsAction(grants []Grant, selector, action string) bool {
	for _, grant := range grants {
		matches := grant.Selector == selector
		if strings.HasPrefix(selector, "artifact:") && strings.HasSuffix(grant.Selector, "/*") {
			matches = strings.HasPrefix(selector, strings.TrimSuffix(grant.Selector, "*"))
		}
		if matches {
			for _, allowed := range grant.Actions {
				if allowed == action {
					return true
				}
			}
		}
	}
	return false
}

// Authorize is effect-free. Callers select the registration/repository/epoch and
// current policy; no request can substitute those identities. Every selected
// resource in a multi-scope method needs its own check before dispatch.
func Authorize(registration Registration, entry EnablementEntry, binding Binding, access Access, selectedContribution bool) error {
	if err := registration.validateStored(); err != nil {
		return err
	}
	if err := ValidateGrants(entry.Grants); err != nil {
		return err
	}
	if !entry.Enabled || binding.RegistrationID != registration.RegistrationID || entry.RegistrationID != binding.RegistrationID || entry.RepositoryID != binding.RepositoryID || entry.Epoch != binding.Epoch || !validDigest(binding.RepositoryID) || !validHex(binding.Epoch, 32) {
		return failure("denied_access", "registration, repository or enablement epoch is not current")
	}
	if registration.Manifest.HostProtocol != HostProtocol {
		return failure("incompatible_version", "unsupported host protocol")
	}
	if !methods[access.Method] {
		return failure("unsupported_capability", "method is not supported")
	}
	if access.Method != "capabilities.list" && !validSelector(access.Selector, false) || !methodAllows(access) {
		return failure("invalid_input", "method selector/action does not match")
	}
	if access.Method == "capabilities.list" {
		return nil
	}
	declared := false
	for _, method := range registration.Manifest.Capabilities {
		if method == access.Method {
			declared = true
		}
	}
	if !declared {
		return failure("denied_access", "method is not declared")
	}
	if !grantsAction(entry.Grants, access.Selector, access.Action) {
		return failure("denied_access", "action is not granted")
	}
	if strings.HasPrefix(access.Method, "changes.") && (!grantsAction(entry.Grants, access.Selector, "read") || !grantsAction(entry.Grants, access.Selector, "observe")) {
		return failure("denied_access", "observation requires both read and observe")
	}
	if selectedContribution && (access.Method == "artifact.mutate" || access.Method == "artifact.type.publish") {
		return failure("unsupported_capability", "custom writes under selected contribution policy are unsupported")
	}
	return nil
}

// ValidateRegistrationGrants rejects scope that has no declared method. It uses
// the same value authorization boundary as inventory and subsequent host calls.
func ValidateRegistrationGrants(registration Registration, entry EnablementEntry) error {
	if err := ValidateGrants(entry.Grants); err != nil {
		return err
	}
	binding := Binding{entry.RegistrationID, entry.RepositoryID, entry.Epoch}
	for _, grant := range entry.Grants {
		for _, action := range grant.Actions {
			selector := grant.Selector
			if strings.HasSuffix(selector, "/*") {
				selector = strings.TrimSuffix(selector, "*") + "type"
			}
			applicable := false
			for _, method := range registration.Manifest.Capabilities {
				access := Access{method, selector, action}
				if !methodAllows(access) {
					continue
				}
				// Observe grants can be installed separately, but only read+observe
				// together can authorize an actual observation request.
				if action == "observe" {
					applicable = true
					break
				}
				if err := Authorize(registration, entry, binding, access, false); err == nil {
					applicable = true
					break
				}
			}
			if !applicable {
				return failure("denied_access", "grant has no matching declared method")
			}
		}
	}
	return nil
}

// EnablementStore is constructed only after the CLI's existing safe Git/worktree
// resolver verifies this physical common directory and its reciprocal pointers.
type EnablementStore struct{ common, repositoryID, path string }

func OpenEnablements(common string) (*EnablementStore, error) {
	if !filepath.IsAbs(common) || filepath.Clean(common) != common || safepath.CheckDir(common, false) != nil {
		return nil, failure("invalid_input", "unsafe physical Git common directory")
	}
	return &EnablementStore{common: common, repositoryID: digest([]byte("hn-extension-repository-v1\x00" + common)), path: filepath.Join(common, "hn", "extensions", "enabled.json")}, nil
}
func (s *EnablementStore) RepositoryID() string { return s.repositoryID }

func (s *EnablementStore) directory(create bool) error {
	if err := safepath.CheckDir(s.common, false); err != nil {
		return failure("invalid_input", "unsafe physical Git common directory")
	}
	for _, path := range []string{filepath.Join(s.common, "hn"), filepath.Dir(s.path)} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			if !create {
				return nil
			}
			if err = os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
				return failure("repository_error", "cannot create private enablement directory")
			}
			info, err = os.Lstat(path)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
			return failure("invalid_input", "enablement directories must be physical and mode 0700")
		}
	}
	return nil
}

func (s *EnablementStore) Load() (Enablements, error) {
	empty := Enablements{"hn.extension-enablement/1", []EnablementEntry{}}
	if err := s.directory(false); err != nil {
		return empty, err
	}
	fd, err := syscall.Open(s.path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		return empty, nil
	}
	if err != nil {
		return empty, failure("invalid_input", "cannot safely open enablement")
	}
	f := os.NewFile(uintptr(fd), s.path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return empty, failure("invalid_input", "enablement must be a private regular file with mode 0600")
	}
	data, err := io.ReadAll(io.LimitReader(f, RegistryLimit+1))
	if err != nil {
		return empty, failure("invalid_input", "cannot read enablement")
	}
	value, err := strictJSON(data, RegistryLimit)
	if err != nil {
		return empty, err
	}
	object, ok := value.(map[string]any)
	if !ok || !exactFields(object, "protocol", "entries") {
		return empty, failure("invalid_input", "invalid enablement fields")
	}
	protocol, ok := object["protocol"].(string)
	if !ok || protocol != empty.Protocol {
		return empty, failure("invalid_input", "invalid enablement protocol")
	}
	entries, ok := object["entries"].([]any)
	if !ok {
		return empty, failure("invalid_input", "entries must be an array")
	}
	if len(entries) > RegistrationLimit {
		return empty, failure("resource_limit", "too many enablement entries")
	}
	seen := map[string]bool{}
	for _, raw := range entries {
		fields, ok := raw.(map[string]any)
		if !ok || !exactFields(fields, "registration_id", "repository_id", "epoch", "enabled", "grants") {
			return empty, failure("invalid_input", "invalid enablement entry fields")
		}
		for _, name := range []string{"registration_id", "repository_id", "epoch"} {
			if _, ok := fields[name].(string); !ok {
				return empty, failure("invalid_input", "enablement identities must be strings")
			}
		}
		if _, ok := fields["enabled"].(bool); !ok {
			return empty, failure("invalid_input", "enabled must be boolean")
		}
		grants, ok := fields["grants"].([]any)
		if !ok {
			return empty, failure("invalid_input", "grants must be an array")
		}
		if len(grants) > GrantLimit {
			return empty, failure("resource_limit", "too many grants")
		}
		for _, rawGrant := range grants {
			grant, ok := rawGrant.(map[string]any)
			if !ok || !exactFields(grant, "selector", "actions") {
				return empty, failure("invalid_input", "invalid grant fields")
			}
			if _, ok := grant["selector"].(string); !ok {
				return empty, failure("invalid_input", "selector must be a string")
			}
			actions, ok := grant["actions"].([]any)
			if !ok {
				return empty, failure("invalid_input", "actions must be an array")
			}
			for _, action := range actions {
				if _, ok := action.(string); !ok {
					return empty, failure("invalid_input", "action must be a string")
				}
			}
		}
		encoded, _ := json.Marshal(fields)
		var entry EnablementEntry
		_ = json.Unmarshal(encoded, &entry)
		if !validDigest(entry.RegistrationID) || entry.RepositoryID != s.repositoryID || !validHex(entry.Epoch, 32) || seen[entry.RegistrationID] {
			return empty, failure("invalid_input", "invalid, duplicate or foreign enablement identity")
		}
		if err := ValidateGrants(entry.Grants); err != nil {
			return empty, err
		}
		seen[entry.RegistrationID] = true
		empty.Entries = append(empty.Entries, entry)
	}
	return empty, nil
}

// Entry never lets a stale registration ID select the current module's grants.
func (s *EnablementStore) Entry(registration Registration) (EnablementEntry, bool, error) {
	state, err := s.Load()
	if err != nil {
		return EnablementEntry{}, false, err
	}
	for _, entry := range state.Entries {
		if entry.RegistrationID == registration.RegistrationID {
			return entry, true, nil
		}
	}
	return EnablementEntry{}, false, nil
}

func (s *EnablementStore) Set(registration Registration, enabled bool, grants []Grant) (EnablementEntry, error) {
	return s.setWithRandom(registration, enabled, grants, rand.Reader)
}
func (s *EnablementStore) setWithRandom(registration Registration, enabled bool, grants []Grant, entropy io.Reader) (EnablementEntry, error) {
	if err := registration.validateStored(); err != nil {
		return EnablementEntry{}, err
	}
	if err := ValidateGrants(grants); err != nil {
		return EnablementEntry{}, err
	}
	var random [16]byte
	if _, err := io.ReadFull(entropy, random[:]); err != nil {
		return EnablementEntry{}, failure("repository_error", "cannot obtain enablement randomness")
	}
	entry := EnablementEntry{registration.RegistrationID, s.repositoryID, hex.EncodeToString(random[:]), enabled, grants}
	if enabled {
		if err := ValidateRegistrationGrants(registration, entry); err != nil {
			return EnablementEntry{}, err
		}
		status, _, err := registration.Availability()
		if err != nil {
			return EnablementEntry{}, err
		}
		if status == "incompatible" {
			return EnablementEntry{}, failure("incompatible_version", "unsupported host protocol")
		}
		if status != "registered" {
			return EnablementEntry{}, failure("unavailable_module", "registered executable is missing or changed")
		}
	}
	if _, err := s.Load(); err != nil {
		return EnablementEntry{}, err
	}
	if err := s.directory(true); err != nil {
		return EnablementEntry{}, err
	}
	fd, err := syscall.Open(filepath.Join(filepath.Dir(s.path), "enabled.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return EnablementEntry{}, failure("repository_error", "cannot lock enablement")
	}
	lock := os.NewFile(uintptr(fd), "enabled.lock")
	defer lock.Close()
	info, err := lock.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return EnablementEntry{}, failure("invalid_input", "unsafe enablement lock")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return EnablementEntry{}, failure("repository_error", "enablement is busy")
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	state, err := s.Load()
	if err != nil {
		return EnablementEntry{}, err
	}
	replaced := false
	for i, old := range state.Entries {
		if old.RegistrationID == entry.RegistrationID {
			state.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		if len(state.Entries) >= RegistrationLimit {
			return EnablementEntry{}, failure("resource_limit", "too many enablement entries")
		}
		state.Entries = append(state.Entries, entry)
	}
	sort.Slice(state.Entries, func(i, j int) bool { return state.Entries[i].RegistrationID < state.Entries[j].RegistrationID })
	data := canonical(state)
	if len(data) > RegistryLimit {
		return EnablementEntry{}, failure("resource_limit", "enablement exceeds byte limit")
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".enabled-*")
	if err != nil {
		return EnablementEntry{}, failure("repository_error", "cannot stage enablement")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return EnablementEntry{}, failure("repository_error", "cannot persist enablement")
	}
	if err := os.Rename(name, s.path); err != nil {
		return EnablementEntry{}, failure("repository_error", "cannot replace enablement")
	}
	return entry, nil
}
