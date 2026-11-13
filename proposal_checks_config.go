package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const proposalChecksVersion = "hn.proposal-checks/1"
const proposalChecksPath = ".hn/proposal-checks.json"

type proposalChecksConfig struct {
	Version   string              `json:"version"`
	Pipelines map[string][]string `json:"pipelines"`
}

// Reject duplicate object keys before decoding into Go maps.
func checkUniqueJSONValue(d *json.Decoder, depth int) error {
	if depth > 16 {
		return fmt.Errorf("configuration is nested too deeply")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate or invalid configuration key")
			}
			seen[name] = true
			if err := checkUniqueJSONValue(d, depth+1); err != nil {
				return err
			}
		}
	case json.Delim('['):
		for d.More() {
			if err := checkUniqueJSONValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	_, err = d.Token()
	return err
}

func parseProposalChecksConfig(raw []byte, hasPipeline func(string) bool) (proposalChecksConfig, error) {
	var config proposalChecksConfig
	if len(raw) > maxPipelineSize {
		return config, fmt.Errorf("trigger configuration exceeds size limit")
	}
	tokens := json.NewDecoder(bytes.NewReader(raw))
	if err := checkUniqueJSONValue(tokens, 0); err != nil {
		return config, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, err
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return config, err
	}
	if config.Version != proposalChecksVersion || config.Pipelines == nil {
		return config, fmt.Errorf("expected version hn.proposal-checks/1 and a pipelines object")
	}
	for name, events := range config.Pipelines {
		if !validPipelineName(name) || !hasPipeline(name) {
			return config, fmt.Errorf("trigger references an invalid or missing pipeline")
		}
		if events == nil {
			return config, fmt.Errorf("trigger events must be a list")
		}
		seen := map[string]bool{}
		for _, event := range events {
			if event != "proposal.open" && event != "proposal.revise" {
				return config, fmt.Errorf("unsupported proposal event")
			}
			if seen[event] {
				return config, fmt.Errorf("duplicate proposal event")
			}
			seen[event] = true
		}
		canonical := make([]string, 0, len(events))
		for _, event := range []string{"proposal.open", "proposal.revise"} {
			if seen[event] {
				canonical = append(canonical, event)
			}
		}
		config.Pipelines[name] = canonical
	}
	return config, nil
}

func cmdProposalChecks(args []string) error {
	if len(args) >= 2 && args[len(args)-1] == "--json" {
		switch args[0] {
		case "init":
			if len(args) == 3 {
				return consumerOutput(map[string]bool{"registered": true}, automaticSchedule("init", args[1]))
			}
		case "observe":
			if len(args) == 2 {
				return consumerOutput(map[string]bool{"observed": true}, automaticSchedule("observe", ""))
			}
		case "status":
			if len(args) == 3 && validEventID(args[1]) {
				return automaticCheckStatus(args[1])
			}
		}
	}

	if len(args) != 3 || args[2] != "--json" {
		return consumerOutput(nil, consumerFailure("invalid_input", "expected proposal-checks read or validate with JSON output"))
	}
	switch args[0] {
	case "read":
		revision := args[1]
		if !validGitOID(revision) {
			return consumerOutput(nil, consumerFailure("invalid_input", "expected exact target commit"))
		}
		_, exists, err := exactTreeEntry(revision, proposalChecksPath)
		if err != nil {
			return consumerOutput(nil, err)
		}
		if !exists {
			return consumerOutput(map[string]any{"source": revision, "digest": nil, "configuration": proposalChecksConfig{proposalChecksVersion, map[string][]string{}}}, nil)
		}
		raw, err := gitOutput("show", revision+":"+proposalChecksPath)
		if err != nil {
			return consumerOutput(nil, err)
		}
		config, err := parseProposalChecksConfig(raw, func(name string) bool { _, _, _, err := loadPipeline(revision, name); return err == nil })
		if err != nil {
			return consumerOutput(nil, consumerFailure("invalid_input", err.Error()))
		}
		return consumerOutput(map[string]any{"source": revision, "digest": eventID(raw), "configuration": config}, nil)
	case "validate":
		if args[1] != "-" {
			return consumerOutput(nil, consumerFailure("invalid_input", "expected draft input on stdin"))
		}
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxPipelineSize+1))
		if err != nil || len(raw) > maxPipelineSize {
			return consumerOutput(nil, consumerFailure("invalid_input", "draft exceeds size limit or could not be read"))
		}
		var draft struct {
			Configuration json.RawMessage               `json:"configuration"`
			Pipelines     map[string]PipelineDefinition `json:"pipelines"`
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := checkUniqueJSONValue(json.NewDecoder(bytes.NewReader(raw)), 0); err != nil {
			return consumerOutput(nil, consumerFailure("invalid_input", err.Error()))
		}
		if err := decoder.Decode(&draft); err != nil || ensureJSONEnd(decoder) != nil || draft.Pipelines == nil {
			return consumerOutput(nil, consumerFailure("invalid_input", "expected configuration and complete draft pipelines"))
		}
		config, err := parseProposalChecksConfig(draft.Configuration, func(name string) bool {
			pipeline, ok := draft.Pipelines[name]
			return ok && validatePipeline(pipeline) == nil
		})
		if err != nil {
			return consumerOutput(nil, consumerFailure("invalid_input", err.Error()))
		}
		canonical, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return consumerOutput(nil, err)
		}
		return consumerOutput(map[string]any{"configuration": config, "canonical": string(canonical) + "\n"}, nil)
	default:
		return consumerOutput(nil, consumerFailure("invalid_input", "unknown proposal-checks action"))
	}
}

// The result binds this intent through its signed request subject.
type automaticRunIntent struct {
	Target        string `json:"target"`
	Version       string `json:"version"`
	Configuration string `json:"configuration"`
	Source        string `json:"source"`
	Check         string `json:"check"`
	Attempt       string `json:"attempt"`
}

func (i automaticRunIntent) valid() bool {
	return i.Version == proposalChecksVersion && validPublicationBaseRef(i.Target) && validGitOID(i.Source) &&
		validEventID(i.Configuration) && validEventID(i.Check) && validEventID(i.Attempt)
}
