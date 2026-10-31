package main

import (
	"hubnot/internal/extension"
	"sync"
)

var optionalBootstrapOnce sync.Once

// Every optional map reader crosses the same completion barrier. Construction
// installs only compiled handlers and never invokes them or accesses local state.
func ensureOptionalBootstrap() {
	optionalBootstrapOnce.Do(func() {
		artifactCommands = map[string]func([]string) error{
			"--help":    cmdArtifactSearchHelp,
			"convert":   func(args []string) error { return artifactSuccessorCommand("record.convert", args) },
			"envelope":  cmdArtifactEnvelope,
			"export":    artifactExchangeExportCommand,
			"fetch":     func(args []string) error { return artifactGitCommand("fetch", args) },
			"heads":     func(args []string) error { return artifactCatalogCommand("heads", args) },
			"help":      cmdArtifactSearchHelp,
			"history":   func(args []string) error { return artifactCatalogCommand("history", args) },
			"import":    artifactExchangeImportCommand,
			"inspect":   artifactExchangeInspectCommand,
			"list":      cmdArtifactList,
			"open":      cmdArtifactOpen,
			"operation": cmdArtifactOperation,
			"push":      func(args []string) error { return artifactGitCommand("push", args) },
			"record":    cmdArtifactRecord,
			"resolve":   func(args []string) error { return artifactSuccessorCommand("record.resolve", args) },
			"revise":    func(args []string) error { return artifactSuccessorCommand("record.revise", args) },
			"schema":    cmdArtifactSchema,
			"search":    cmdArtifactSearch,
			"show":      func(args []string) error { return artifactCatalogCommand("show", args) },
			"type":      cmdArtifactType,
		}
		artifactTypeCommands = map[string]func([]string) error{
			"publish":  cmdArtifactTypePublish,
			"show":     cmdArtifactTypeShow,
			"validate": cmdArtifactTypeValidate,
		}
		extensionSubcommands = map[string]func([]string) (any, error){
			"disable":  extensionDisableAndCleanup,
			"enable":   extensionEnableAndCleanup,
			"frame":    extensionFrame,
			"inspect":  extensionInspectEnablement,
			"list":     extensionInspectEnablement,
			"register": extensionRegister,
			"remove":   extensionRemoveAndCleanup,
			"run":      extensionRun,
		}
		extensionHostMethods = map[string]extension.Dispatch{
			"artifact.mutate":       extensionArtifactDispatch,
			"artifact.operation":    extensionArtifactDispatch,
			"artifact.read":         extensionArtifactDispatch,
			"artifact.search":       extensionArtifactDispatch,
			"artifact.type.publish": extensionArtifactDispatch,
			"artifact.type.read":    extensionArtifactDispatch,
			"changes.capture":       extensionChangesDispatch,
			"changes.page":          extensionChangesDispatch,
			"changes.since":         extensionChangesDispatch,
			"identity.public":       extensionNativeDispatch,
			"resource.mutate":       extensionNativeDispatch,
			"resource.operation":    extensionNativeDispatch,
			"resource.read":         extensionNativeDispatch,
		}
	})
}

// Existing native registrations, including nil overrides, retain precedence.
func lookupLocalCommand(name string) (func([]string) error, bool) {
	if command, present := localCommands[name]; present {
		return command, true
	}
	switch name {
	case "artifact":
		return cmdArtifact, true
	case "extension":
		return extensionHostCommand, true
	default:
		return nil, false
	}
}

// Help includes effective optional commands without growing the native map.
func localCommandNames() []string {
	names := make([]string, 0, len(localCommands)+2)
	for name := range localCommands {
		names = append(names, name)
	}
	for _, name := range [2]string{"artifact", "extension"} {
		if _, present := localCommands[name]; !present {
			names = append(names, name)
		}
	}
	return names
}
