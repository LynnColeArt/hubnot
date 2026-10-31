package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"hubnot/internal/extension"
)

func init() {
	// Static registration only. No repository, registry or enablement access is
	// performed until an explicit extension management command is dispatched.
	extensionSubcommands["enable"] = extensionEnable
	extensionSubcommands["disable"] = extensionDisable
	extensionSubcommands["inspect"] = extensionInspectEnablement
	extensionSubcommands["list"] = extensionInspectEnablement
	extensionSubcommands["remove"] = extensionRemoveEnablement
}

type extensionEnablementResult struct {
	ModuleID        string            `json:"module_id"`
	RegistrationID  string            `json:"registration_id"`
	RepositoryID    string            `json:"repository_id"`
	Epoch           string            `json:"epoch"`
	Enabled         bool              `json:"enabled"`
	Grants          []extension.Grant `json:"grants"`
	StoppedSessions []string          `json:"stopped_sessions"`
	Cleanup         string            `json:"cleanup"`
}

func extensionRepository(required bool) (*extension.EnablementStore, error) {
	// Reuse the established physical Git/common-directory and reciprocal-pointer
	// verifier. Module argv, declarations and request fields choose no repository.
	common, err := contributionGitDirectory()
	if err == nil {
		return extension.OpenEnablements(common)
	}
	if !required {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, gitErr := projectGit(ctx, "", nil, 4096, "rev-parse", "--absolute-git-dir"); gitErr != nil {
			// Broken repository markers are errors, not an outside-repository
			// inventory with its repository identity silently erased.
			cwd, cwdErr := os.Getwd()
			if cwdErr == nil {
				for p := cwd; ; p = filepath.Dir(p) {
					if _, statErr := os.Lstat(filepath.Join(p, ".git")); statErr == nil {
						return nil, extensionRepositoryError()
					}
					if filepath.Dir(p) == p {
						break
					}
				}
			}
			return nil, nil
		}
	}
	return nil, extensionRepositoryError()
}
func extensionRepositoryError() error {
	return &extension.Error{Origin: "host", Code: "repository_error", Message: "cannot verify the physical repository", Recovery: "do_not_retry"}
}

func extensionRegisteredModule(module string) (extension.Registration, error) {
	path, err := extension.RegistryPath()
	if err != nil {
		return extension.Registration{}, err
	}
	registry, err := extension.LoadRegistry(path)
	if err != nil {
		return extension.Registration{}, err
	}
	for _, registration := range registry.Registrations {
		if registration.Manifest.ModuleID == module {
			return registration, nil
		}
	}
	return extension.Registration{}, &extension.Error{Origin: "host", Code: "not_found", Message: "module is not registered", Recovery: "do_not_retry"}
}

func extensionEnable(args []string) (any, error) {
	module := ""
	grants := []extension.Grant{}
	jsonFlag := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			if jsonFlag {
				return nil, extensionInput("duplicate --json")
			}
			jsonFlag = true
		case "--grant":
			if i+1 == len(args) {
				return nil, extensionInput("missing grant value")
			}
			i++
			grant, err := extension.ParseGrant(args[i])
			if err != nil {
				return nil, err
			}
			grants = append(grants, grant)
		default:
			if module != "" || !extension.ValidModuleID(args[i]) {
				return nil, extensionInput("expected module ID and explicit --grant values")
			}
			module = args[i]
		}
	}
	if module == "" || len(grants) == 0 {
		return nil, extensionInput("enable requires module ID and at least one explicit --grant")
	}
	if err := extension.ValidateGrants(grants); err != nil {
		return nil, err
	}
	registration, err := extensionRegisteredModule(module)
	if err != nil {
		return nil, err
	}
	store, err := extensionRepository(true)
	if err != nil {
		return nil, err
	}
	entry, err := store.Set(registration, true, grants)
	if err != nil {
		return nil, err
	}
	result := extensionEnablementResult{module, entry.RegistrationID, entry.RepositoryID, entry.Epoch, entry.Enabled, entry.Grants, []string{}, "not_needed"}
	return result, nil
}

func extensionDisable(args []string) (any, error) {
	module, err := extensionModuleArgument(args, true)
	if err != nil {
		return nil, err
	}
	registration, err := extensionRegisteredModule(module)
	if err != nil {
		return nil, err
	}
	store, err := extensionRepository(true)
	if err != nil {
		return nil, err
	}
	old, found, err := store.Entry(registration)
	if err != nil {
		return nil, err
	}
	grants := []extension.Grant{}
	if found {
		grants = old.Grants
	}
	entry, err := store.Set(registration, false, grants)
	if err != nil {
		return nil, err
	}
	// This slice has no run command or session owner. WP16 wraps these current
	// handlers with finite cleanup before reporting management completion.
	result := extensionEnablementResult{module, entry.RegistrationID, entry.RepositoryID, entry.Epoch, false, entry.Grants, []string{}, "not_needed"}
	return result, nil
}

func extensionInspectEnablement(args []string) (any, error) {
	data, err := extensionInspect(args)
	if err != nil {
		return nil, err
	}
	result := data.(extensionInspectionResult)
	store, err := extensionRepository(false)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return result, nil
	}
	state, err := store.Load()
	if err != nil {
		return nil, err
	}
	for i := range result.Modules {
		module := &result.Modules[i]
		repositoryID := store.RepositoryID()
		module.RepositoryID = &repositoryID
		for _, entry := range state.Entries {
			if entry.RegistrationID != module.RegistrationID {
				continue
			}
			module.Grants = []any{}
			for _, grant := range entry.Grants {
				module.Grants = append(module.Grants, grant)
			}
			module.Enabled = entry.Enabled && module.Compatible && module.Status == "registered"
			if module.Enabled {
				registration, err := extensionRegisteredModule(module.ModuleID)
				if err != nil {
					return nil, err
				}
				if err := extension.ValidateRegistrationGrants(registration, entry); err != nil {
					return nil, err
				}
			}
		}
	}
	return result, nil
}

func extensionRemoveEnablement(args []string) (any, error) {
	module, err := extensionModuleArgument(args, true)
	if err != nil {
		return nil, err
	}
	registration, err := extensionRegisteredModule(module)
	if err != nil {
		return nil, err
	}
	store, err := extensionRepository(false)
	if err != nil {
		return nil, err
	}
	if store != nil {
		entry, found, err := store.Entry(registration)
		if err != nil {
			return nil, err
		}
		if found {
			if _, err := store.Set(registration, false, entry.Grants); err != nil {
				return nil, err
			}
		}
	}
	// The WP13 remover retains the closed response and atomic registry edit.
	// Removed registration instances never match retained entries elsewhere.
	return extensionRemove(args)
}
