package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"hubnot/internal/artifact"
	"os"
)

// Optional handlers are installed together on explicit dispatch, without I/O.
var artifactCommands map[string]func([]string) error

func cmdArtifact(args []string) error {
	ensureOptionalBootstrap()
	if len(args) > 0 {
		if handler, ok := artifactCommands[args[0]]; ok {
			return handler(args[1:])
		}
	}
	return artifactOutput(nil, &artifact.Error{Code: "invalid_input", Message: "expected an artifact subcommand"}, hasJSON(args))
}
func hasJSON(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "--json=true" {
			return true
		}
	}
	return false
}
func artifactOutput(data any, err error, machine bool) error {
	if !machine {
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(data)
	}
	var failure any
	if err != nil {
		code, message := "invalid_input", "artifact input refused"
		var ae *artifact.Error
		if errors.As(err, &ae) {
			code, message = ae.Code, ae.Message
		}
		failure = map[string]any{"origin": "artifact", "code": code, "message": message, "recovery": "do_not_retry", "operation": nil}
		data = nil
	}
	if e := json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": err == nil, "data": data, "error": failure}); e != nil {
		return e
	}
	if err != nil {
		return fmt.Errorf("artifact request refused")
	}
	return nil
}
