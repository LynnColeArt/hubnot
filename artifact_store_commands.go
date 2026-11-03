package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hubnot/internal/artifact"
	"os"
	"strings"
	"time"
)

func cmdArtifactTypePublish(args []string) error {
	return artifactMutationCommand("type.publish", args)
}
func cmdArtifactOpen(args []string) error { return artifactMutationCommand("record.open", args) }
func artifactMutationCommand(kind string, args []string) error {
	machine := hasJSON(args)
	values := map[string]string{}
	seen := map[string]bool{}
	invalid := func(message string) error {
		return artifactOutput(nil, artifactFailure("invalid_input", message), machine)
	}
	for i := 0; i < len(args); i++ {
		name, value, eq := strings.Cut(args[i], "=")
		if name == "--json" {
			if seen[name] || eq && (value != "true" && value != "false") {
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
	data, e := writeArtifact(ctx, kind, values["--actor"], values["--operation"], raw)
	return artifactStorageOutput(data, e, machine, values["--actor"], values["--operation"])
}
func cmdArtifactTypeShow(args []string) error {
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
			return artifactOutput(nil, artifactFailure("invalid_input", "requires one exact type declaration ID"), machine)
		}
	}
	if !validMemoryDigestID(id) {
		return artifactOutput(nil, artifactFailure("invalid_input", "requires full type declaration ID"), machine)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h, e := loadArtifactHistory(ctx)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	s := h.ByID[id]
	if s == nil || s.verified.Kind() != "type.publish" {
		return artifactOutput(nil, artifactFailure("not_found", "type declaration not found in selected accepted history"), machine)
	}
	return artifactOutput(map[string]any{"items": []*storedArtifact{s}, "source_cut": h.Cut, "next_cursor": nil}, nil, machine)
}

type artifactStorageError struct{ Origin, Code, Message string }

func (e *artifactStorageError) Error() string { return e.Code + ": " + e.Message }
func artifactStorageOutput(data any, err error, machine bool, actor, operation string) error {
	if err == nil || !machine {
		return artifactOutput(data, err, machine)
	}
	origin, code, message, recovery := "artifact", "invalid_input", "artifact request refused", "do_not_retry"
	var storage *artifactStorageError
	var ordinary *artifact.Error
	if errors.As(err, &storage) {
		origin, code, message = storage.Origin, storage.Code, storage.Message
	} else if errors.As(err, &ordinary) {
		code, message = ordinary.Code, ordinary.Message
	}
	var op any
	if code == "concurrent_update" {
		recovery = "retry_identical"
	}
	if code == "uncertain_operation" {
		recovery = "inspect_operation"
		op = map[string]any{"protocol": "hn-artifact/1", "actor": actor, "operation": operation}
	}
	if e := json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "data": nil, "error": map[string]any{"origin": origin, "code": code, "message": message, "recovery": recovery, "operation": op}}); e != nil {
		return e
	}
	return fmt.Errorf("artifact request refused")
}
