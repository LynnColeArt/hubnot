package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"hubnot/internal/extension"
)

// Extension command owners add dispatch functions here from their own init
// functions. This static seam, like localCommands, performs no registry I/O.
var extensionSubcommands = map[string]func([]string) (any, error){
	"register": extensionRegister,
	"inspect":  extensionInspect,
	"list":     extensionInspect,
	"remove":   extensionRemove,
}

func init() { localCommands["extension"] = cmdExtension }
func extensionInput(message string) error {
	return &extension.Error{Origin: "host", Code: "invalid_input", Message: message, Recovery: "do_not_retry"}
}
func cmdExtension(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println("Usage: hn extension register --manifest FILE --executable PATH [--arg VALUE] --json\n       hn extension inspect [MODULE_ID] --json\n       hn extension list --json\n       hn extension remove MODULE_ID --json")
		return nil
	}
	var data any
	var err error
	if command, ok := extensionSubcommands[args[0]]; ok {
		data, err = command(args[1:])
	} else {
		err = extensionInput("unknown extension command")
	}
	var public *extension.Error
	if err != nil {
		if !errors.As(err, &public) {
			public = &extension.Error{Origin: "host", Code: "repository_error", Message: "extension management failed", Recovery: "do_not_retry"}
		}
		data = nil
	}
	if encodeErr := json.NewEncoder(os.Stdout).Encode(struct {
		OK    bool             `json:"ok"`
		Data  any              `json:"data"`
		Error *extension.Error `json:"error"`
	}{err == nil, data, public}); encodeErr != nil {
		return encodeErr
	}
	return err
}
func extensionRegister(args []string) (any, error) {
	manifestFile, executable := "", ""
	argv := []string{}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		name := args[i]
		if name == "--json" {
			if seen[name] {
				return nil, extensionInput("duplicate --json")
			}
			seen[name] = true
			continue
		}
		if name != "--manifest" && name != "--executable" && name != "--arg" {
			return nil, extensionInput("unknown registration argument")
		}
		if i+1 == len(args) {
			return nil, extensionInput("missing registration argument value")
		}
		if name != "--arg" && seen[name] {
			return nil, extensionInput("duplicate registration argument")
		}
		seen[name] = true
		i++
		switch name {
		case "--manifest":
			manifestFile = args[i]
		case "--executable":
			executable = args[i]
		case "--arg":
			argv = append(argv, args[i])
		}
	}
	if manifestFile == "" || executable == "" {
		return nil, extensionInput("register requires --manifest and --executable")
	}
	var reader io.Reader
	var file *os.File
	if manifestFile == "-" {
		reader = os.Stdin
	} else {
		var err error
		file, err = os.Open(manifestFile)
		if err != nil {
			return nil, extensionInput("cannot read manifest file")
		}
		defer file.Close()
		reader = file
	}
	manifest, err := extension.ReadManifest(reader)
	if err != nil {
		return nil, err
	}
	registration, err := extension.NewRegistration(manifest, executable, argv)
	if err != nil {
		return nil, err
	}
	path, err := extension.RegistryPath()
	if err != nil {
		return nil, err
	}
	if registration, err = extension.Register(path, registration); err != nil {
		return nil, err
	}
	return struct {
		ModuleID         string `json:"module_id"`
		ModuleVersion    string `json:"module_version"`
		RegistrationID   string `json:"registration_id"`
		ManifestDigest   string `json:"manifest_digest"`
		ExecutableDigest string `json:"executable_digest"`
		Status           string `json:"status"`
		Enabled          bool   `json:"enabled"`
	}{manifest.ModuleID, manifest.ModuleVersion, registration.RegistrationID, registration.ManifestDigest, registration.ExecutableDigest, "registered", false}, nil
}
func extensionModuleArgument(args []string, required bool) (string, error) {
	module := ""
	jsonFlag := false
	for _, arg := range args {
		if arg == "--json" {
			if jsonFlag {
				return "", extensionInput("duplicate --json")
			}
			jsonFlag = true
			continue
		}
		if module != "" || !extension.ValidModuleID(arg) {
			return "", extensionInput("expected one namespaced module ID")
		}
		module = arg
	}
	if required && module == "" {
		return "", extensionInput("module ID is required")
	}
	return module, nil
}

type extensionModuleInspection struct {
	ModuleID         string  `json:"module_id"`
	ModuleVersion    string  `json:"module_version"`
	RegistrationID   string  `json:"registration_id"`
	ManifestDigest   string  `json:"manifest_digest"`
	ExecutableDigest string  `json:"executable_digest"`
	RepositoryID     *string `json:"repository_id"`
	Enabled          bool    `json:"enabled"`
	Compatible       bool    `json:"compatible"`
	Availability     string  `json:"availability"`
	Grants           []any   `json:"grants"`
	Status           string  `json:"status"`
}
type extensionUnsupported struct {
	Surface  string `json:"surface"`
	Status   string `json:"status"`
	Recovery string `json:"recovery"`
}

type extensionInspectionResult struct {
	Modules     []extensionModuleInspection `json:"modules"`
	Unsupported []extensionUnsupported      `json:"unsupported"`
}

func extensionInspect(args []string) (any, error) {
	module, err := extensionModuleArgument(args, false)
	if err != nil {
		return nil, err
	}
	path, err := extension.RegistryPath()
	if err != nil {
		return nil, err
	}
	registry, err := extension.LoadRegistry(path)
	if err != nil {
		return nil, err
	}
	modules := []extensionModuleInspection{}
	for _, r := range registry.Registrations {
		if module != "" && module != r.Manifest.ModuleID {
			continue
		}
		status, availability, err := r.Availability()
		if err != nil {
			return nil, err
		}
		modules = append(modules, extensionModuleInspection{ModuleID: r.Manifest.ModuleID, ModuleVersion: r.Manifest.ModuleVersion, RegistrationID: r.RegistrationID, ManifestDigest: r.ManifestDigest, ExecutableDigest: r.ExecutableDigest, Compatible: r.Manifest.HostProtocol == extension.HostProtocol, Availability: availability, Grants: []any{}, Status: status})
	}
	if module != "" && len(modules) == 0 {
		return nil, &extension.Error{Origin: "host", Code: "not_found", Message: "module is not registered", Recovery: "do_not_retry"}
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].ModuleID < modules[j].ModuleID })
	unsupported := []extensionUnsupported{}
	for _, surface := range []string{"memory.mutate", "proposal", "review", "decide", "merge", "policy.mutate", "identity.mutate", "run", "runner", "audit", "automerge", "sync.apply", "migration.apply"} {
		unsupported = append(unsupported, extensionUnsupported{surface, "unsupported", "do_not_retry"})
	}
	return extensionInspectionResult{modules, unsupported}, nil
}
func extensionRemove(args []string) (any, error) {
	module, err := extensionModuleArgument(args, true)
	if err != nil {
		return nil, err
	}
	path, err := extension.RegistryPath()
	if err != nil {
		return nil, err
	}
	registration, err := extension.Remove(path, module)
	if err != nil {
		return nil, err
	}
	return struct {
		ModuleID           string   `json:"module_id"`
		RegistrationID     string   `json:"registration_id"`
		Removed            bool     `json:"removed"`
		StoppedSessions    []string `json:"stopped_sessions"`
		PublicDataRetained bool     `json:"public_data_retained"`
	}{module, registration.RegistrationID, true, []string{}, true}, nil
}
