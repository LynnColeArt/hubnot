package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"regexp/syntax"
	"unicode/utf8"
)

const (
	maxMemoryGrepPatternBytes = 4096
	maxMemoryGrepInstructions = 4096
	// Conservative deterministic work estimate: compiled instructions multiplied
	// by candidate UTF-8 bytes plus one per independently matched field.
	maxMemoryGrepWork = 64 << 20
)

type memoryGrepPattern struct {
	expression   *regexp.Regexp
	instructions int
}

func compileMemoryGrepPattern(pattern string, ignoreCase bool) (memoryGrepPattern, error) {
	if len(pattern) > maxMemoryGrepPatternBytes || !utf8.ValidString(pattern) {
		return memoryGrepPattern{}, fmt.Errorf("grep pattern must be valid UTF-8 and at most %d bytes", maxMemoryGrepPatternBytes)
	}
	expression := pattern
	if ignoreCase {
		expression = "(?i)" + expression
	}
	parsed, err := syntax.Parse(expression, syntax.Perl)
	if err != nil {
		return memoryGrepPattern{}, fmt.Errorf("invalid grep pattern (Go regexp syntax): %w", err)
	}
	program, err := syntax.Compile(parsed.Simplify())
	if err != nil {
		return memoryGrepPattern{}, fmt.Errorf("invalid grep pattern: %w", err)
	}
	if len(program.Inst) > maxMemoryGrepInstructions {
		return memoryGrepPattern{}, fmt.Errorf("grep pattern exceeds %d compiled instructions", maxMemoryGrepInstructions)
	}
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return memoryGrepPattern{}, fmt.Errorf("invalid grep pattern: %w", err)
	}
	return memoryGrepPattern{compiled, len(program.Inst)}, nil
}

func cmdMemoryGrep(args []string) error {
	flags := quietFlags("memory grep")
	request := RecallRequestV0{Version: intValuePointer(memoryCommandVersion)}
	at := bindMemoryRecallFlags(flags, &request)
	flags.Lookup("max-content-bytes").Usage = "maximum encoded author data bytes"
	jsonOutput := flags.Bool("json", false, "emit JSON")
	var ignoreCase bool
	flags.BoolVar(&ignoreCase, "i", false, "Unicode case-insensitive matching")
	flags.BoolVar(&ignoreCase, "ignore-case", false, "Unicode case-insensitive matching")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return usageError("usage: hn memory grep [filters] [bounds] [-i|--ignore-case] [--json] PATTERN")
	}
	pattern := flags.Arg(0)
	// Validate syntax/complexity before anything may rebuild derived index state.
	compiled, err := compileMemoryGrepPattern(pattern, ignoreCase)
	if err != nil {
		return err
	}
	if *at == "" {
		*at = "HEAD"
	}
	request.AtCommit, err = resolveCommit(*at)
	if err != nil {
		return err
	}
	request, err = normalizeRecallRequestV0(request)
	if err != nil {
		return err
	}
	rows, source, policy, err := loadMemoryRecallSelection(request)
	if err != nil {
		return err
	}
	rows, err = matchMemoryGrepRows(rows, compiled)
	if err != nil {
		return err
	}
	digest, err := memoryGrepQueryDigest(request, pattern, ignoreCase, source, policy)
	if err != nil {
		return err
	}
	envelope, err := buildMemorySearchEnvelope(request, digest, rows, "author data", func(data MemoryRecord) ([]byte, error) {
		return json.Marshal(data)
	})
	if err != nil {
		return err
	}
	return printMemorySearchEnvelope(envelope, *jsonOutput, func(row MemoryIndexRecordV0) {
		for _, field := range memoryGrepFields(row.Data) {
			if compiled.expression.MatchString(field.value) {
				fmt.Printf("  Match %s: %q\n", field.name, field.value)
			}
		}
	})
}

func matchMemoryGrepRows(rows []MemoryIndexRecordV0, pattern memoryGrepPattern) ([]MemoryIndexRecordV0, error) {
	// Preflight the entire selection, so a work-budget failure cannot emit a
	// partial page or depend on where the first matching field happens to occur.
	remaining := maxMemoryGrepWork / pattern.instructions
	for _, row := range rows {
		fields := memoryGrepFields(row.Data)
		for _, field := range fields {
			if len(field.value) >= remaining {
				return nil, fmt.Errorf("grep search exceeds work limit; narrow recall filters or simplify the pattern")
			}
			remaining -= len(field.value) + 1
		}
	}
	matches := make([]MemoryIndexRecordV0, 0)
	for _, row := range rows {
		matched := false
		for _, field := range memoryGrepFields(row.Data) {
			if pattern.expression.MatchString(field.value) {
				matched = true
				break
			}
		}
		if matched {
			matches = append(matches, row)
		}
	}
	return matches, nil
}

func memoryGrepQueryDigest(request RecallRequestV0, pattern string, ignoreCase bool, sourceFingerprint, policyDigest string) (string, error) {
	request.Cursor = ""
	material := struct {
		Domain            string          `json:"domain"`
		Request           RecallRequestV0 `json:"request"`
		Pattern           string          `json:"pattern"`
		IgnoreCase        bool            `json:"ignoreCase"`
		SourceFingerprint string          `json:"sourceFingerprint"`
		PolicyDigest      string          `json:"policyDigest"`
	}{"hn-memory-grep-query-v0", request, pattern, ignoreCase, sourceFingerprint, policyDigest}
	encoded, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	return memoryID(encoded), nil
}

// Field names are generated from the schema; values remain inert author data.
// Keep strings separate so anchors cannot cross unrelated list items or fields.
type memoryGrepField struct {
	name  string
	value string
}

func memoryGrepFields(data MemoryRecord) []memoryGrepField {
	fields := []memoryGrepField{{"data.content", data.Content}}
	appendItems := func(name string, items []string) {
		for index, value := range items {
			fields = append(fields, memoryGrepField{fmt.Sprintf("data.%s[%d]", name, index), value})
		}
	}
	appendItems("topics", data.Topics)
	if data.Handoff != nil {
		appendItems("handoff.completed", data.Handoff.Completed)
		appendItems("handoff.assumptions", data.Handoff.Assumptions)
		appendItems("handoff.blockers", data.Handoff.Blockers)
		appendItems("handoff.nextActions", data.Handoff.NextActions)
	}
	return fields
}
