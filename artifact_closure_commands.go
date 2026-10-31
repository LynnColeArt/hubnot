package main

import (
	"context"
	"reflect"
	"strings"
	"time"
)

func init() { artifactCommands["inspect"] = artifactClosureCommand }

func artifactClosureCommand(args []string) error {
	machine := hasJSON(args)
	id := ""
	seen := map[string]bool{}
	invalid := func(message string) error {
		return artifactOutput(nil, artifactFailure("invalid_input", message), machine)
	}
	for i := 0; i < len(args); i++ {
		name, value, eq := strings.Cut(args[i], "=")
		if seen[name] {
			return invalid("duplicate inspect flag")
		}
		seen[name] = true
		switch name {
		case "--json":
			if eq && value != "true" && value != "false" {
				return invalid("invalid json flag")
			}
			machine = !eq || value == "true"
		case "--id":
			if !eq {
				i++
				if i >= len(args) {
					return invalid("inspect requires --id ID")
				}
				value = args[i]
			}
			id = value
		default:
			return invalid("inspect requires --id ID and optional --json")
		}
	}
	if !validMemoryDigestID(id) {
		return invalid("inspect requires a full artifact ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	closure, e := selectArtifactClosure(ctx, h, []string{id})
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	// The export prefix refs are derived values. The observation witness remains
	// the actual accepted source refs captured by the repository reader.
	after, e := artifactSources(ctx, h.gitDir)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	if !reflect.DeepEqual(h.Sources, after) {
		return artifactOutput(nil, artifactFailure("concurrent_update", "artifact sources changed while computing closure"), machine)
	}
	if ctx.Err() != nil {
		return artifactOutput(nil, artifactFailure("cancelled", "artifact closure cancelled"), machine)
	}
	return artifactOutput(map[string]any{"items": closure.Items, "source_cut": h.Cut, "next_cursor": nil}, nil, machine)
}
