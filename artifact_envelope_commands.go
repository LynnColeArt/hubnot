package main

import (
	"encoding/json"
	"flag"
	"hubnot/internal/artifact"
	"io"
	"os"
)

// Envelope inspection is an explicit file-only integrity check. It never loads
// identity, registry, accepted history or a specialized executable.
func cmdArtifactEnvelope(args []string) error {
	machine := hasJSON(args)
	invalid := func(message string) error {
		return artifactOutput(nil, &artifact.Error{Code: "invalid_input", Message: message}, machine)
	}
	if len(args) == 0 || (args[0] != "validate" && args[0] != "inspect") {
		return invalid("expected envelope validate or inspect")
	}
	flags := flag.NewFlagSet("artifact envelope", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("input", "", "canonical envelope file or - for stdin")
	signature := flags.String("signature", "", "canonical raw Base64 signature")
	previousInput := flags.String("previous-input", "", "optional preceding canonical envelope file")
	previousSignature := flags.String("previous-signature", "", "preceding raw Base64 signature")
	flags.BoolVar(&machine, "json", machine, "JSON output")
	if e := flags.Parse(args[1:]); e != nil || flags.NArg() != 0 || *input == "" || *signature == "" {
		return invalid("requires --input FILE|- and --signature BASE64")
	}
	if (*previousInput == "") != (*previousSignature == "") || (*previousInput == "-" && *input == "-") {
		return invalid("predecessor requires paired input and signature with at most one stdin selection")
	}
	var previous *artifact.Verified
	if *previousInput != "" {
		b, e := artifactEnvelopeInput(*previousInput)
		if e != nil {
			return artifactOutput(nil, e, machine)
		}
		v, e := artifact.Verify(b, *previousSignature, nil)
		if e != nil {
			return artifactOutput(nil, e, machine)
		}
		previous = &v
	}
	b, e := artifactEnvelopeInput(*input)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	v, e := artifact.Verify(b, *signature, previous)
	if e != nil {
		return artifactOutput(nil, e, machine)
	}
	continuity := "not_checked"
	if v.Genesis() {
		continuity = "genesis"
	}
	if previous != nil {
		continuity = "predecessor_verified"
	}
	// The envelope's own genesis rule was checked; further type/schema and
	// repository lineage verification is expressly outside this file-only slice.
	data := map[string]any{"id": v.ID(), "actor": v.Actor(), "kind": v.Kind(), "integrity": "verified", "continuity": continuity, "semantic_validation": "not_checked"}
	if args[0] == "inspect" {
		data["envelope"] = json.RawMessage(v.Bytes())
	}
	return artifactOutput(data, nil, machine)
}
func artifactEnvelopeInput(path string) ([]byte, error) {
	if path == "-" {
		return artifact.Read(os.Stdin)
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, &artifact.Error{Code: "invalid_input", Message: "cannot open envelope input"}
	}
	defer f.Close()
	return artifact.Read(f)
}
