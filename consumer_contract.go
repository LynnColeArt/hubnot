package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const consumerSchema = "hn.consumer/1"

// consumerError uses the native closed vocabulary documented in extensions.md.
// These CLI intents have no operation key, so recovery never promises a safe
// mutation replay and operation is always null.
type consumerError struct {
	Origin    string `json:"origin"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Recovery  string `json:"recovery"`
	Operation any    `json:"operation"`
}

func (e *consumerError) Error() string { return e.Code + ": " + e.Message }
func consumerFailure(code, message string) error {
	return &consumerError{Origin: "native", Code: code, Message: message, Recovery: "do_not_retry"}
}

func consumerOutput(data any, err error) error {
	var failure *consumerError
	if err != nil {
		failure = &consumerError{Origin: "native", Code: "repository_error", Message: "native repository operation failed", Recovery: "do_not_retry"}
		var public *consumerError
		var command *issueCommandError
		var read *IssueReadError
		switch {
		case errors.As(err, &public):
			*failure = *public
		case errors.As(err, &command):
			failure.Code = command.Code
			failure.Message = command.Message
		case errors.As(err, &read):
			failure.Code = read.Code
			failure.Message = "native accepted history could not be verified"
		}
		switch failure.Code {
		case "actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision":
		default:
			failure.Code = "repository_error"
			failure.Message = "native repository operation failed"
		}
		// Unkeyed commands cannot inspect an original operation or promise replay.
		failure.Origin = "native"
		failure.Recovery = "do_not_retry"
		failure.Operation = nil
		if !utf8.ValidString(failure.Message) || len(failure.Message) > 4096 || strings.ContainsAny(failure.Message, "/\\\r\n") {
			failure.Message = "native operation refused; inspect local native diagnostics"
		}
		data = nil
	}
	if e := json.NewEncoder(os.Stdout).Encode(struct {
		OK    bool           `json:"ok"`
		Data  any            `json:"data"`
		Error *consumerError `json:"error"`
	}{err == nil, data, failure}); e != nil {
		return e
	}
	if err != nil {
		return fmt.Errorf("consumer request refused")
	}
	return nil
}

// Static discovery is dispatched before contribution locks or optional module
// bootstrap. Its data describes only support compiled and implemented here.
func cmdConsumerDiscovery(args []string) error {
	flags := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	machine := false
	flags.BoolVar(&machine, "json", false, "versioned JSON output")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !machine {
		return consumerOutput(nil, consumerFailure("invalid_input", "discovery requires --json and no other arguments"))
	}
	return consumerOutput(map[string]any{
		"schema": consumerSchema, "version": version,
		"capabilities": map[string]any{
			"native.discovery":           map[string]any{"version": 1, "commands": []string{"version --json", "--version --json", "capabilities --json"}},
			"contribution.existing-json": map[string]any{"version": 1, "envelope": false, "mode": "existing-default-json", "documentation": "docs/consumer-cli-v1.md"},
		},
	}, nil)
}

// Discovery has no string-valued options. Respect '--' so a positional value
// that happens to be --json never selects the new version output mode.
func consumerDiscoveryJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}
