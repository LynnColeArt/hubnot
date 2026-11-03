package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hubnot/internal/extension"
)

// The optional bootstrap installs synchronous typed adapters before dispatch.
// The current host honestly refuses methods without a compiled adapter.
var extensionHostMethods map[string]extension.Dispatch

func extensionHostDispatch(ctx context.Context, call extension.Invocation) (any, error) {
	ensureOptionalBootstrap()
	request := call.Request
	if handler, ok := extensionHostMethods[request.Method]; ok {
		return handler(ctx, call)
	}
	if err := extension.AuthorizeRequest(call); err != nil {
		return nil, err
	}
	return nil, &extension.Error{Origin: "host", Code: "unsupported_capability", Message: "method adapter is unavailable in this host", Recovery: "do_not_retry"}
}

func extensionHostCommand(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: hn extension register --manifest FILE --executable PATH [--arg VALUE] --json\n       hn extension inspect [MODULE_ID] --json\n       hn extension list --json\n       hn extension enable MODULE_ID --grant SELECTOR=ACTION[,ACTION] --json\n       hn extension disable MODULE_ID --json\n       hn extension remove MODULE_ID --json\n       hn extension run MODULE_ID --json\n       hn extension frame inspect --input FILE|- --kind hello|ready|request|response|cancel [--expected-request-id ID] [--method METHOD] [--timeout-ms MS] --json")
		return nil
	}
	return cmdExtension(args)
}
func extensionRun(args []string) (any, error) {
	module, err := extensionModuleArgument(args, true)
	if err != nil {
		return nil, err
	}
	// Start the absolute launch budget before registry/repository verification.
	started := time.Now()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, startupCancel := context.WithDeadline(ctx, started.Add(extension.StartupBudget))
	defer startupCancel()
	registration, err := extensionRegisteredModule(module)
	if err != nil {
		return nil, err
	}
	store, err := extensionRepository(true)
	if err != nil {
		return nil, err
	}
	directory, err := os.Getwd()
	if err != nil {
		return nil, extensionRepositoryError()
	}
	recheck := func(checkContext context.Context, binding extension.Binding) (extension.Registration, extension.EnablementEntry, error) {
		if checkContext.Err() != nil {
			return extension.Registration{}, extension.EnablementEntry{}, frameInputError("cancelled", "host validation cancelled")
		}
		current, e := extensionRegisteredModule(module)
		if e != nil {
			return current, extension.EnablementEntry{}, e
		}
		physical, e := extensionRepository(true)
		if e != nil {
			return current, extension.EnablementEntry{}, e
		}
		entry, found, e := physical.Entry(current)
		if e != nil {
			return current, entry, e
		}
		if !found || current.RegistrationID != binding.RegistrationID || physical.RepositoryID() != binding.RepositoryID {
			return current, entry, frameInputError("denied_access", "registration or repository binding changed")
		}
		if e = extension.Authorize(current, entry, binding, extension.Access{Method: "capabilities.list"}, false); e != nil {
			return current, entry, e
		}
		status, _, e := current.Availability()
		if e != nil {
			return current, entry, e
		}
		if status == "incompatible" {
			return current, entry, frameInputError("incompatible_version", "unsupported host protocol")
		}
		if status != "registered" {
			return current, entry, frameInputError("unavailable_module", "registered executable is missing or changed")
		}
		if checkContext.Err() != nil {
			return current, entry, frameInputError("cancelled", "host validation cancelled")
		}
		return current, entry, nil
	}
	if startup.Err() != nil {
		return nil, frameInputError("cancelled", "module startup cancelled")
	}
	// Run receives the original parent. Its own absolute startup budget includes
	// the elapsed CLI validation, while the session can outlive that startup timer.
	return extension.Run(ctx, extension.SessionConfig{Registration: registration, Store: store, Directory: directory, Started: started, Recheck: recheck, Dispatch: func(requestContext context.Context, call extension.Invocation) (any, error) {
		return extensionHostDispatch(requestContext, call)
	}})
}
func extensionCleanupResult(data any, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	result := data.(extensionEnablementResult)
	store, err := extensionRepository(true)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), extension.ExitBudget)
	defer cancel()
	stopped, err := store.WaitRetired(ctx, result.RegistrationID, result.Epoch)
	if err != nil {
		return nil, err
	}
	result.StoppedSessions = stopped
	if len(stopped) > 0 {
		result.Cleanup = "verified"
	}
	return result, nil
}
func extensionEnableAndCleanup(args []string) (any, error) {
	return extensionCleanupResult(extensionEnable(args))
}
func extensionDisableAndCleanup(args []string) (any, error) {
	return extensionCleanupResult(extensionDisable(args))
}
func extensionRemoveAndCleanup(args []string) (any, error) {
	module, err := extensionModuleArgument(args, true)
	if err != nil {
		return nil, err
	}
	registration, err := extensionRegisteredModule(module)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), extension.ExitBudget)
	defer cancel()
	stopped, err := extension.RetireRegistration(ctx, registration.RegistrationID)
	if err != nil {
		return nil, err
	}
	// Revoke the current repository's epoch as well. Other repository entries
	// retain the retired registration ID and cannot authorize a fresh instance.
	store, err := extensionRepository(false)
	if err != nil {
		return nil, err
	}
	if store != nil {
		entry, found, e := store.Entry(registration)
		if e != nil {
			return nil, e
		}
		if found {
			if _, e = store.Set(registration, false, entry.Grants); e != nil {
				return nil, e
			}
		}
	}
	path, err := extension.RegistryPath()
	if err != nil {
		return nil, err
	}
	removed, err := extension.Remove(path, module)
	if err != nil {
		return nil, err
	}
	if removed.RegistrationID != registration.RegistrationID {
		return nil, frameInputError("verification_failure", "registration changed during removal")
	}
	return struct {
		ModuleID           string   `json:"module_id"`
		RegistrationID     string   `json:"registration_id"`
		Removed            bool     `json:"removed"`
		StoppedSessions    []string `json:"stopped_sessions"`
		PublicDataRetained bool     `json:"public_data_retained"`
	}{module, removed.RegistrationID, true, stopped, true}, nil
}
