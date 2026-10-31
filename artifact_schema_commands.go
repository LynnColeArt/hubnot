package main

import (
	"context"
	"hubnot/internal/artifact"
	"io"
	"os"
	"strings"
	"time"
)

// Later compiled publish/show slices extend this map, preserving one dispatcher.
var artifactTypeCommands = map[string]func([]string) error{"validate": cmdArtifactTypeValidate}

func init() {
	artifactCommands["type"] = cmdArtifactType
	artifactCommands["schema"] = cmdArtifactSchema
	artifactCommands["record"] = cmdArtifactRecord
}
func cmdArtifactType(args []string) error {
	if len(args) > 0 {
		if handler, ok := artifactTypeCommands[args[0]]; ok {
			return handler(args[1:])
		}
	}
	return artifactOutput(nil, &artifact.Error{Code: "invalid_input", Message: "expected an artifact type subcommand"}, hasJSON(args))
}
func cmdArtifactSchema(args []string) error { return artifactValidationCommand("schema", args) }
func cmdArtifactRecord(args []string) error { return artifactValidationCommand("record", args) }
func cmdArtifactTypeValidate(args []string) error {
	return artifactValidationCommand("type", append([]string{"validate"}, args...))
}

func artifactValidationCommand(kind string, args []string) error {
	machine := hasJSON(args)
	invalid := func(message string) error {
		return artifactOutput(nil, &artifact.Error{Code: "invalid_input", Message: message}, machine)
	}
	if len(args) == 0 || args[0] != "validate" {
		return invalid("expected validate subcommand")
	}
	path := ""
	seenInput, seenJSON := false, false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--input" || strings.HasPrefix(arg, "--input="):
			if seenInput {
				return invalid("duplicate input flag")
			}
			seenInput = true
			if arg == "--input" {
				i++
				if i >= len(args) {
					return invalid("input flag requires a file")
				}
				path = args[i]
			} else {
				path = strings.TrimPrefix(arg, "--input=")
			}
			if path == "" {
				return invalid("input flag requires a file")
			}
		case arg == "--json" || arg == "--json=true" || arg == "--json=false":
			if seenJSON {
				return invalid("duplicate json flag")
			}
			seenJSON = true
			machine = arg != "--json=false"
		default:
			return invalid("unknown flag or positional argument")
		}
	}
	if !seenInput {
		return invalid("requires exactly one --input FILE|-")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	limit := artifact.MaxSchemaBytes
	if kind == "type" {
		limit = artifact.MaxTypeBytes
	}
	if kind == "record" {
		limit = artifact.MaxValidationBytes
	}
	raw, e := artifactValidationInput(ctx, path, limit)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	var data any
	switch kind {
	case "schema":
		_, e = artifact.ValidateSchema(ctx, raw)
		data = map[string]any{"language": "hn.schema/1", "schema_validation": "verified", "integrity": "not_applicable"}
	case "type":
		var typ artifact.TypeDeclaration
		typ, e = artifact.ValidateType(ctx, raw)
		data = map[string]any{"type_id": typ.TypeID, "schema_version": typ.SchemaVersion, "shape": typ.Shape, "schema_validation": "verified", "integrity": "not_checked", "type_lineage": "not_checked"}
	case "record":
		data, e = artifact.ValidateRecord(ctx, raw)
	}
	return artifactOutput(data, e, machine)
}
func artifactValidationInput(ctx context.Context, path string, limit int) ([]byte, error) {
	failure := func(code, message string) ([]byte, error) { return nil, &artifact.Error{Code: code, Message: message} }
	var f *os.File
	if path == "-" {
		f = os.Stdin
	} else {
		// An explicit local file is data, never a device/pipe endpoint or reference.
		info, e := os.Stat(path)
		if e != nil || !info.Mode().IsRegular() {
			return failure("invalid_input", "cannot open validation input file")
		}
		f, e = os.Open(path)
		if e != nil {
			return failure("invalid_input", "cannot open validation input file")
		}
		defer f.Close()
	}
	// Closing this read-only input unblocks a partial stdin read on expiration.
	// No background parser, writer, signer or publication can survive the command.
	stop := context.AfterFunc(ctx, func() { _ = f.Close() })
	defer stop()
	raw, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if ctx.Err() != nil {
		return failure("cancelled", "artifact validation cancelled")
	}
	if e != nil {
		return failure("invalid_input", "cannot read validation input")
	}
	if len(raw) > limit {
		return failure("resource_limit", "validation input exceeds byte limit")
	}
	return raw, nil
}
