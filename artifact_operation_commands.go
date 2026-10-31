package main

import (
	"context"
	"strings"
	"time"
)

func init() { artifactCommands["operation"] = cmdArtifactOperation }

func cmdArtifactOperation(args []string) error {
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
		if name != "--actor" && name != "--operation" && name != "--type" || seen[name] {
			return invalid("unknown or duplicate operation flag")
		}
		seen[name] = true
		if !eq {
			i++
			if i >= len(args) {
				return invalid("operation flag requires a value")
			}
			value = args[i]
		}
		if value == "" {
			return invalid("operation flag requires a value")
		}
		values[name] = value
	}
	if len(values) != 3 {
		return invalid("requires --actor ACTOR --operation KEY --type TYPE_ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := inspectArtifactOperation(ctx, values["--type"], values["--actor"], values["--operation"])
	return artifactOutput(result, err, machine)
}
