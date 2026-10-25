package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hubnot/internal/forge"
)

const forgeTransferTimeout = 5 * time.Minute
const forgeCheckpointLimit = 8 << 20
const forgeCommandHelp = `Usage:
  hn transfer import --provider PROVIDER --project PROJECT [--key KEY] [--endpoint URL] [--dry-run]
  hn transfer export RESOURCE_ID --provider PROVIDER --project PROJECT [--endpoint URL] [--dry-run]
  hn transfer capabilities --provider PROVIDER [--resource issue|project|workflow|discussion]

Explicit optional resource transfer; GitHub is the first provider adapter. The
local hn issue and hn gh issue interfaces require no provider or credentials.
Only explicitly selected adapters read their credentials (GitHub: HN_GITHUB_TOKEN,
optional for public reads, required for writes). Dry-run creates no native facts
or checkpoints. The five-minute command deadline bounds a whole transfer;
interrupted imports may leave valid partial progress, recovered on retry.
Native actors sign source observations; external author/time are source claims.
Recovery files in the Git directory under hn/forge are private (0700/0600).
Only one local transfer may run at a time. After a crash, verify no transfer is
running before manually removing a stale hn/forge/transfer.lock in the Git directory.
A nogithub build excludes the GitHub adapter and remains fully usable offline.
Use --resource to select an import/export resource (default issue). Explicit
provider mappings use repeated --map KEY=VALUE (no empty/duplicate keys; 64 max).
Issue transfer currently needs no mappings and refuses unsupported mapping keys.
Typed project/workflow/discussion readers and exporters are optional adapter
capabilities. Unsupported resources refuse before provider access; missing
mappings or dependencies refuse before any mutation. Resource imports preflight
complete bounded graphs and resume exact saved snapshots after interruption.
The capabilities report is read-only and lists exact supported mappings/losses;
workflow execution semantics always require an explicit mapping.
`

type transferOptions struct {
	command, provider, project, id, endpoint, key string
	resource                                      forge.Resource
	mappings                                      map[string]string
	dryRun                                        bool
}
type forgeRegistration struct {
	New func(string, string) (forge.Provider, error)
	// NewResource permits adapters that do not implement issue CRUD.
	NewResource  func(string, string) (forge.Identity, error)
	TokenEnv     string
	Capabilities []forge.Capability
}

var forgeProviders = map[string]forgeRegistration{}
var forgeExport func(context.Context, forge.Provider, transferOptions) error

func cmdTransfer(args []string) error {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Print(forgeCommandHelp)
		return nil
	}
	o, err := parseTransferOptions(args)
	if err != nil {
		return err
	}
	if o.command == "help" {
		fmt.Print(forgeCommandHelp)
		return nil
	}
	return runForgeTransfer(o)
}
func runForgeTransfer(o transferOptions) error {
	registration, ok := forgeProviders[o.provider]
	if !ok {
		return errors.New("requested forge provider is unavailable in this build; use hn transfer help")
	}
	if o.command == "capabilities" {
		if o.resource != "" && !forge.KnownResource(o.resource) {
			return errors.New("unknown transfer resource")
		}
		return json.NewEncoder(os.Stdout).Encode(forgeCapabilityReport(o.provider, o.resource))
	}
	if o.resource == "" {
		o.resource = forge.ResourceIssue
	}
	if !forge.KnownResource(o.resource) {
		return errors.New("unknown transfer resource")
	}
	allowed := false
	for _, capability := range registration.Capabilities {
		if capability.Resource == o.resource {
			allowed = o.command == "import" && capability.Import || o.command == "export" && capability.Export
		}
	}
	if !allowed {
		return errors.New("requested resource has no implemented provider mapping; inspect hn transfer capabilities before migrating")
	}
	if o.resource == forge.ResourceIssue && len(o.mappings) > 0 {
		return errors.New("issue transfer has no configurable field mappings; unsupported --map values refuse before provider access")
	}
	if !issueIdentifier(o.project, 512) || o.key != "" && !issueIdentifier(o.key, 256) {
		return errors.New("invalid opaque project or issue selector")
	}
	if o.resource == forge.ResourceIssue && o.command == "export" && forgeExport == nil {
		return errors.New("issue export is not installed yet")
	}
	if o.resource != forge.ResourceIssue && !forgeResourceInstalled(o) {
		return errors.New("requested resource transfer handler is unavailable")
	}
	var provider forge.Identity
	var err error
	token := ""
	if registration.TokenEnv != "" {
		token = os.Getenv(registration.TokenEnv)
	}
	if o.resource == forge.ResourceIssue && registration.New != nil {
		provider, err = registration.New(o.endpoint, token)
	} else if registration.NewResource != nil {
		provider, err = registration.NewResource(o.endpoint, token)
	} else if registration.New != nil {
		provider, err = registration.New(o.endpoint, token)
	} else {
		return errors.New("provider has no installed factory")
	}
	if err != nil {
		return err
	}
	if provider == nil || provider.Kind() != o.provider || !forgeValidLocator(provider.Endpoint()) {
		return errors.New("provider factory returned an inconsistent identity")
	}
	o.endpoint = provider.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), forgeTransferTimeout)
	defer cancel()
	if !o.dryRun {
		release, err := forgeAcquireLock()
		if err != nil {
			return err
		}
		defer release()
	}
	if o.resource != forge.ResourceIssue {
		return forgeTransferResource(ctx, provider, o)
	}
	issueProvider, ok := provider.(forge.Provider)
	if !ok {
		return errors.New("provider does not implement issue transfer")
	}
	if o.command == "export" {
		if forgeExport == nil {
			return errors.New("issue export is not installed yet")
		}
		return forgeExport(ctx, issueProvider, o)
	}
	return forgeImport(ctx, issueProvider, o)
}
func parseTransferOptions(args []string) (transferOptions, error) {
	o := transferOptions{command: args[0], resource: forge.ResourceIssue}
	rest := args[1:]
	if o.command != "import" && o.command != "export" && o.command != "capabilities" {
		return o, errors.New("unsupported transfer command; run 'hn transfer help'")
	}
	if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
		o.command = "help"
		return o, nil
	}
	if o.command == "export" {
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
			return o, errors.New("export requires a native resource ID before flags")
		}
		o.id = rest[0]
		rest = rest[1:]
		if !validEventID(o.id) {
			return o, errors.New("export requires a full sha256:<64-hex> native resource ID")
		}
	}
	flags := flag.NewFlagSet("transfer "+o.command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&o.provider, "provider", "", "explicit provider")
	mappings := transferMappings{}
	flags.Var(&mappings, "map", "explicit provider mapping KEY=VALUE (repeatable)")
	resource := string(o.resource)
	flags.StringVar(&resource, "resource", "", "issue, project, workflow or discussion")
	flags.StringVar(&o.project, "project", "", "external project selector")
	flags.StringVar(&o.endpoint, "endpoint", "", "explicit provider endpoint")
	flags.BoolVar(&o.dryRun, "dry-run", false, "read-only preview")
	if o.command == "import" {
		flags.StringVar(&o.key, "key", "", "one opaque external issue key")
	}
	if err := flags.Parse(rest); err != nil {
		return o, errors.New("invalid transfer flags; run 'hn transfer help'")
	}
	o.resource = forge.Resource(resource)
	o.mappings = map[string]string(mappings)
	if o.command == "capabilities" {
		if flags.NArg() != 0 || o.provider == "" {
			return o, errors.New("capabilities requires --provider")
		}
		return o, nil
	}
	if o.resource == "" {
		o.resource = forge.ResourceIssue
	}
	if flags.NArg() != 0 || o.provider == "" || o.project == "" {
		return o, errors.New("--provider and --project are required, without extra arguments")
	}
	providedKey := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "key" {
			providedKey = true
		}
	})
	if providedKey && o.key == "" {
		return o, errors.New("--key cannot be empty")
	}
	return o, nil
}

func forgeNativeIssues() ([]issueDetail, error) {
	result := []issueDetail{}
	cursor := ""
	for {
		raw, _, next, err := executeIssueQuery(issueOptions{command: "list", details: true, limit: 200, cursor: cursor})
		if err != nil {
			return nil, err
		}
		result = append(result, raw.(issuePage).Items.([]issueDetail)...)
		if next == "" {
			return result, nil
		}
		cursor = next
	}
}
func forgeNativeComments(id string) ([]issueHistoryItem, error) {
	result := []issueHistoryItem{}
	cursor := ""
	for {
		raw, _, next, err := executeIssueQuery(issueOptions{command: "history", id: id, kind: "comments", limit: 200, cursor: cursor})
		if err != nil {
			return nil, err
		}
		result = append(result, raw.(issuePage).Items.([]issueHistoryItem)...)
		if next == "" {
			return result, nil
		}
		cursor = next
	}
}
func forgeNativeRevisions(id string) ([]issueHistoryItem, error) {
	result := []issueHistoryItem{}
	cursor := ""
	for {
		raw, _, next, err := executeIssueQuery(issueOptions{command: "history", id: id, kind: "revisions", limit: 200, cursor: cursor})
		if err != nil {
			return nil, err
		}
		result = append(result, raw.(issuePage).Items.([]issueHistoryItem)...)
		if next == "" {
			return result, nil
		}
		cursor = next
	}
}

// Checkpoint names are opaque filenames, not paths provided by source tickets.
func forgeCheckpointPath(name string) (string, error) {
	if name == "" || name == "." || name == ".." || len(name) > 200 {
		return "", errors.New("invalid forge checkpoint name")
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.') {
			return "", errors.New("invalid forge checkpoint name")
		}
	}
	dir, err := requireGitRepository()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hn", "forge", name), nil
}
func forgeStateDir(create bool) (string, error) {
	path, err := forgeCheckpointPath("state.json")
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	for _, p := range []string{filepath.Dir(dir), dir} {
		if create {
			err = ensurePrivateDirectory(p)
		} else {
			err = checkPrivateDirectory(p)
		}
		if errors.Is(err, os.ErrNotExist) && !create {
			return dir, nil
		}
		if err != nil {
			return "", errors.New("unsafe forge checkpoint directory; inspect private Git-directory state")
		}
	}
	return dir, nil
}
func forgeLoadJSON(name string, target any) (bool, error) {
	if _, err := forgeStateDir(false); err != nil {
		return false, err
	}
	path, err := forgeCheckpointPath(name)
	if err != nil {
		return false, err
	}
	raw, err := readPrivateFileBounded(path, forgeCheckpointLimit)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("forge checkpoint is unreadable or unsafe")
	}
	if err := forgeStrictJSON(raw, target); err != nil {
		return false, errors.New("invalid forge checkpoint JSON")
	}
	return true, nil
}
func forgeStoreJSON(name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > forgeCheckpointLimit {
		return errors.New("forge checkpoint exceeds supported JSON bounds")
	}
	if _, err := forgeStateDir(true); err != nil {
		return err
	}
	path, err := forgeCheckpointPath(name)
	if err != nil {
		return err
	}
	if err := writePrivateFileAtomic(path, raw); err != nil {
		return errors.New("could not atomically save forge checkpoint")
	}
	return nil
}
func forgeRemoveCheckpoint(name string) error {
	if _, err := forgeStateDir(false); err != nil {
		return err
	}
	path, err := forgeCheckpointPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return errors.New("could not remove confirmed forge checkpoint")
	}
	return syncPrivateDirectory(filepath.Dir(path))
}
func forgeAcquireLock() (func(), error) {
	dir, err := forgeStateDir(true)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "transfer.lock")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, errors.New("forge transfer lock unavailable; if stale, verify no local transfer is running before removing hn/forge/transfer.lock in the Git directory")
	}
	_, writeErr := fmt.Fprintf(file, "pid %d\n", os.Getpid())
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		os.Remove(path)
		return nil, errors.New("could not persist forge transfer lock")
	}
	return func() { _ = os.Remove(path); _ = syncPrivateDirectory(dir) }, nil
}
func forgeStrictJSON(raw []byte, target any) error {
	if err := validateIssueJSONUnicode(raw); err != nil {
		return errors.New("invalid JSON or Unicode")
	}

	// Detect duplicate object keys, including escaped spellings, before decoding.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return errors.New("JSON nesting limit")
		}
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					key, err := dec.Token()
					if err != nil {
						return err
					}
					s, ok := key.(string)
					if !ok || seen[s] {
						return errors.New("duplicate JSON key")
					}
					seen[s] = true
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for dec.More() {
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("invalid JSON delimiter")
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return decodePrivateJSON(raw, target, "forge data")
}

func forgeCapabilityReport(provider string, selected forge.Resource) forge.MappingReport {
	registration := forgeProviders[provider]
	result := forge.MappingReport{Provider: provider, Capabilities: []forge.Capability{}}
	for _, resource := range forge.Resources() {
		if selected != "" && selected != resource {
			continue
		}
		item := forge.Capability{Resource: resource, Reason: "explicit provider mapping not implemented", MappedFields: []string{}, Unsupported: []string{"native representation and provider mapping required"}}
		for _, available := range registration.Capabilities {
			if available.Resource == resource {
				item = available
			}
		}
		if resource == forge.ResourceIssue && forgeExport == nil {
			item.Export = false
		}
		if resource != forge.ResourceIssue {
			if !forgeResourceInstalled(transferOptions{command: "export", provider: provider, resource: resource}) {
				item.Export = false
			}
		}
		result.Capabilities = append(result.Capabilities, item)
	}
	return result
}

type transferMappings map[string]string

func (m *transferMappings) String() string { return "explicit provider mappings" }
func (m *transferMappings) Set(raw string) error {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || !issueIdentifier(key, 128) || !issueIdentifier(value, 4096) {
		return errors.New("mapping requires nonempty bounded KEY=VALUE")
	}
	if *m == nil {
		*m = transferMappings{}
	}
	if _, exists := (*m)[key]; exists {
		return errors.New("duplicate explicit mapping key")
	}
	if len(*m) >= 64 {
		return errors.New("at most 64 explicit mappings are supported")
	}
	(*m)[key] = value
	return nil
}
