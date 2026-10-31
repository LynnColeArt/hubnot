package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hubnot/internal/artifact"
	"os"
	"sort"
	"strings"
)

func init() {
	artifactCommands["export"] = artifactExchangeExportCommand
	artifactCommands["import"] = artifactExchangeImportCommand
	artifactCommands["inspect"] = artifactExchangeInspectCommand
}

type artifactExchangeFlags struct {
	input, output             string
	ids, opaqueIDs            []string
	opaque, preserve, machine bool
}

func parseArtifactExchangeFlags(command string, args []string) (artifactExchangeFlags, error) {
	f := artifactExchangeFlags{ids: []string{}, opaqueIDs: []string{}, machine: hasJSON(args)}
	seen := map[string]bool{}
	selections := map[string]bool{}
	invalid := func() (artifactExchangeFlags, error) {
		return f, artifactFailure("invalid_input", "invalid or duplicate artifact exchange selector; export requires explicit IDs and --output FILE; import requires --input FILE|-")
	}
	for i := 0; i < len(args); i++ {
		name, value, equal := strings.Cut(args[i], "=")
		if seen[name] && name != "--id" && name != "--opaque-id" {
			return invalid()
		}
		seen[name] = true
		switch name {
		case "--json", "--opaque", "--preserve-opaque":
			if equal && value != "true" && value != "false" {
				return invalid()
			}
			enabled := !equal || value == "true"
			switch name {
			case "--json":
				f.machine = enabled
			case "--opaque":
				f.opaque = enabled
			case "--preserve-opaque":
				f.preserve = enabled
			}
		case "--input", "--output", "--id", "--opaque-id":
			if !equal {
				i++
				if i >= len(args) {
					return invalid()
				}
				value = args[i]
			}
			if value == "" {
				return invalid()
			}
			switch name {
			case "--input":
				f.input = value
			case "--output":
				f.output = value
			default:
				if !validMemoryDigestID(value) || selections[name+value] {
					return invalid()
				}
				selections[name+value] = true
				if name == "--id" {
					f.ids = append(f.ids, value)
				} else {
					f.opaqueIDs = append(f.opaqueIDs, value)
				}
			}
		default:
			return invalid()
		}
	}
	switch command {
	case "export":
		if f.input != "" || seen["--preserve-opaque"] || f.output == "" || f.output == "-" || f.opaque && (len(f.ids) != 0 || len(f.opaqueIDs) == 0) || !f.opaque && (len(f.ids) == 0 || len(f.opaqueIDs) != 0) {
			return invalid()
		}
	case "import":
		if f.input == "" || f.output != "" || len(f.ids) != 0 || len(f.opaqueIDs) != 0 || seen["--opaque"] {
			return invalid()
		}
	case "inspect":
		if !f.opaque || len(f.opaqueIDs) != 1 || len(f.ids) != 0 || f.input != "" || f.output != "" || seen["--preserve-opaque"] {
			return invalid()
		}
	}
	return f, nil
}
func artifactExchangeOutput(data any, err error, machine bool) error {
	if err == nil || !machine {
		return artifactOutput(data, err, machine)
	}
	origin, code, message := "artifact", "invalid_input", "artifact exchange refused"
	var ae *artifact.Error
	var se *artifactStorageError
	if errors.As(err, &ae) {
		code, message = ae.Code, ae.Message
	} else if errors.As(err, &se) {
		origin, code, message = se.Origin, se.Code, se.Message
	}
	recovery := "do_not_retry"
	if code == "concurrent_update" {
		recovery = "retry_identical"
	}
	if code == "uncertain_operation" {
		recovery = "inspect_operation"
	}
	if e := json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "data": nil, "error": map[string]any{"origin": origin, "code": code, "message": message, "recovery": recovery, "operation": nil}}); e != nil {
		return e
	}
	return fmt.Errorf("artifact exchange refused")
}
func artifactExchangeExportCommand(args []string) error {
	f, e := parseArtifactExchangeFlags("export", args)
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	validation, cancel := context.WithTimeout(context.Background(), artifactExchangeStage)
	defer cancel()
	var b *artifactExchange
	records := 0
	if f.opaque {
		b, e = exportArtifactOpaque(validation, f.opaqueIDs)
	} else {
		b, records, e = exportArtifactExchange(validation, f.ids)
	}
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	raw, e := artifactExchangeBytes(b)
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	copyCtx, copyCancel := context.WithTimeout(context.Background(), artifactExchangeStage)
	defer copyCancel()
	if e = artifactExchangeWrite(copyCtx, f.output, raw); e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	ids := []string{}
	if f.opaque {
		for _, o := range b.Objects {
			ids = append(ids, artifactOpaqueID(o))
		}
		sort.Strings(ids)
	}
	return artifactExchangeOutput(map[string]any{"mode": b.Mode, "output_digest": eventID(raw), "records": records, "objects": len(b.Objects), "refs": b.Refs, "opaque_ids": ids, "bytes": len(raw)}, nil, f.machine)
}
func artifactExchangeImportCommand(args []string) error {
	f, e := parseArtifactExchangeFlags("import", args)
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	validation, cancel := context.WithTimeout(context.Background(), artifactExchangeStage)
	defer cancel()
	dir, e := contributionGitDirectory()
	if e != nil {
		return artifactExchangeOutput(nil, artifactFailure("repository_error", "artifact import requires a safe Git repository"), f.machine)
	}
	format, e := artifactExchangeFormat(validation, dir)
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	raw, e := readArtifactExchange(validation, f.input)
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	b, e := decodeArtifactExchange(validation, raw, format)
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	if len(b.Objects) == 0 {
		return artifactExchangeOutput(nil, artifactFailure("invalid_input", "exchange requires public objects"), f.machine)
	}
	if validation.Err() != nil {
		return artifactExchangeOutput(nil, artifactFailure("cancelled", "artifact exchange validation cancelled"), f.machine)
	}
	var data map[string]any
	if f.preserve {
		data, e = preserveArtifactExchange(context.Background(), b)
	} else if b.Mode == "opaque" {
		e = artifactFailure("invalid_input", "opaque exchange requires explicit --preserve-opaque; it cannot be accepted")
	} else {
		data, e = importArtifactExchange(context.Background(), b)
	}
	return artifactExchangeOutput(data, e, f.machine)
}
func artifactExchangeInspectCommand(args []string) error {
	opaque := false
	for _, arg := range args {
		name, _, _ := strings.Cut(arg, "=")
		if name == "--opaque" || name == "--opaque-id" {
			opaque = true
		}
	}
	if !opaque {
		return artifactClosureCommand(args)
	}
	f, e := parseArtifactExchangeFlags("inspect", args)
	if e != nil {
		return artifactExchangeOutput(nil, e, f.machine)
	}
	ctx, cancel := context.WithTimeout(context.Background(), artifactExchangeStage)
	defer cancel()
	data, e := readArtifactOpaque(ctx, f.opaqueIDs[0])
	return artifactExchangeOutput(data, e, f.machine)
}
