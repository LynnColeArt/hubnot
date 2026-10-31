package main

import (
	"context"
	"hubnot/internal/artifact"
	"strings"
	"time"
)

func artifactSuccessorCommand(kind string, args []string) error {
	machine := hasJSON(args)
	values := map[string]string{}
	seen := map[string]bool{}
	invalid := func(message string) error {
		return artifactOutput(nil, artifactFailure("invalid_input", message), machine)
	}
	for i := 0; i < len(args); i++ {
		name, value, eq := strings.Cut(args[i], "=")
		if name == "--json" {
			if seen[name] || eq && value != "true" && value != "false" {
				return invalid("invalid or duplicate json flag")
			}
			seen[name] = true
			machine = !eq || value == "true"
			continue
		}
		if name != "--input" && name != "--actor" && name != "--operation" || seen[name] {
			return invalid("unknown or duplicate mutation flag")
		}
		seen[name] = true
		if !eq {
			i++
			if i >= len(args) {
				return invalid("mutation flag requires a value")
			}
			value = args[i]
		}
		if value == "" {
			return invalid("mutation flag requires a value")
		}
		values[name] = value
	}
	if len(values) != 3 {
		return invalid("requires --input FILE|- --actor ACTOR --operation KEY")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, e := artifactValidationInput(ctx, values["--input"], artifact.MaxEnvelopeBytes)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	result, e := writeArtifactSuccessor(ctx, kind, values["--actor"], values["--operation"], raw)
	return artifactStorageOutput(result, e, machine, values["--actor"], values["--operation"])
}
func artifactCatalogCommand(mode string, args []string) error {
	machine := hasJSON(args)
	id := ""
	seenJSON := false
	for _, arg := range args {
		if arg == "--json" || arg == "--json=true" || arg == "--json=false" {
			if seenJSON {
				return artifactOutput(nil, artifactFailure("invalid_input", "duplicate json flag"), machine)
			}
			seenJSON = true
			machine = arg != "--json=false"
		} else if id == "" && !strings.HasPrefix(arg, "-") {
			id = arg
		} else {
			return artifactOutput(nil, artifactFailure("invalid_input", "requires one exact record ID"), machine)
		}
	}
	if !validMemoryDigestID(id) {
		return artifactOutput(nil, artifactFailure("invalid_input", "requires full record ID"), machine)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	c, e := buildArtifactCatalog(ctx, h)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	logical, e := c.logical(id)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	if mode == "history" {
		items := make([]*storedArtifact, 0, len(c.records[logical]))
		for _, revision := range c.records[logical] {
			if ctx.Err() != nil {
				return artifactOutput(nil, artifactFailure("cancelled", "artifact history cancelled"), machine)
			}
			items = append(items, h.ByID[revision])
		}
		return artifactOutput(map[string]any{"items": items, "source_cut": h.Cut, "next_cursor": nil}, nil, machine)
	}
	view, e := c.view(ctx, logical)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	return artifactOutput(map[string]any{"items": []artifactRecordView{view}, "source_cut": h.Cut, "next_cursor": nil}, nil, machine)
}
