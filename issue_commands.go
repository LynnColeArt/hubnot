package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const issueJSONSchema = "hn.issue/1"

type issueCommandError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *issueCommandError) Error() string { return e.Code + ": " + e.Message }
func issueFailure(code string, err error) error {
	if err == nil {
		return nil
	}
	var command *issueCommandError
	if errors.As(err, &command) {
		return err
	}
	var read *IssueReadError
	if errors.As(err, &read) {
		code = read.Code
	}
	return &issueCommandError{code, oneLine(err.Error())}
}

type issueEnvelope struct {
	Schema     string             `json:"schema"`
	OK         bool               `json:"ok"`
	Data       any                `json:"data,omitempty"`
	Error      *issueCommandError `json:"error,omitempty"`
	Snapshot   string             `json:"snapshot,omitempty"`
	NextCursor string             `json:"next_cursor,omitempty"`
}
type issueOptions struct {
	command, id, input, body, operation, snapshot, cursor, kind, actor string
	expected                                                           []string
	limit                                                              int
	json, partial, details                                             bool
	state                                                              *IssueState
	title                                                              string
}
type issueExpected []string

func (v *issueExpected) String() string     { return strings.Join(*v, ",") }
func (v *issueExpected) Set(s string) error { *v = append(*v, s); return nil }

const issueHelp = `Usage:
  hn issue open [--body TEXT] [--operation KEY] [--json] TITLE
  hn issue open --input FILE|- [--operation KEY] [--json]
  hn issue revise ISSUE --expect REV --input FILE|- [--operation KEY] [--json]
  hn issue close|reopen ISSUE --expect REV [--operation KEY] [--json]
  hn issue resolve ISSUE --expect REV --expect REV ... --input FILE|- [--operation KEY] [--json]
  hn issue resolve ISSUE --partial --snapshot TOKEN --expect REV ... --input FILE|- [--operation KEY] [--json]
  hn issue comment ISSUE [--body TEXT] [--operation KEY] [--json] [TEXT]
  hn issue operation --actor ACTOR --operation KEY [--json]
  hn issue list [--details] [--limit N] [--cursor TOKEN] [--json]
  hn issue show|heads|graph ISSUE [--limit N] [--cursor TOKEN] [--json]
  hn issue history ISSUE [--kind all|revisions|comments] [--limit N] [--cursor TOKEN] [--json]

All issue mutations accept optional --actor ACTOR to pin the signing identity.
Operation lookup requires an explicit full actor fingerprint and operation key.
List --details returns complete current state and immutable opening metadata.
Conflict state is null; detail mode is bound to its continuation cursors.
Mutation ISSUE and REV identifiers must be full sha256:<64-hex> IDs.
Read commands accept unique prefixes. Flags follow ISSUE, and precede free text.
Input is a full issue state: title, body, status, criteria, labels, assignees,
relations and metadata. Omitted optional fields reset to empty; status defaults open.
JSON input is strict and at most 256 KiB. Page size defaults 50, maximum 200.
Issue show pages head IDs; heads pages full attributed states. History orders
revisions topologically, then comments by timestamp/ID when --kind all.
Graph pages signed edges and individual cycle members with component IDs.
Ordinary resolve requires all current heads; partial requires 2..200 current heads
and the exact observed snapshot, preserving unconsumed heads. Operation retries
return the original event. Snapshot checks and actor CAS are not global issue locks.
`

// Scan the same registered flag grammar even when a value later fails parsing.
// The last valid json flag selects output; malformed booleans leave that choice
// intact. Values, positional text and text after -- cannot select machine mode.
func issueMachineRequested(args []string, flags *flag.FlagSet) bool {
	machine := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}
		name, value, assigned := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		registered := flags.Lookup(name)
		if registered == nil {
			continue
		}
		boolean, isBoolean := registered.Value.(interface{ IsBoolFlag() bool })
		if isBoolean && boolean.IsBoolFlag() {
			if name == "json" {
				if !assigned {
					value = "true"
				}
				if parsed, err := strconv.ParseBool(value); err == nil {
					machine = parsed
				}
			}
		} else if !assigned {
			i++ // A value may itself look like --json or --.
		}
	}
	return machine
}

func cmdIssue(args []string) error {
	options, err := parseIssueOptions(args)
	if err == nil && options.command == "help" {
		if options.json {
			return json.NewEncoder(os.Stdout).Encode(issueEnvelope{Schema: issueJSONSchema, OK: true, Data: map[string]string{"help": issueHelp}})
		}
		fmt.Print(issueHelp)
		return nil
	}
	var data any
	var snapshot, cursor string
	if err == nil {
		if issueIsMutation(options.command) {
			data, err = executeIssueMutation(options)
		} else {
			data, snapshot, cursor, err = executeIssueQuery(options)
		}
	}
	if err != nil {
		err = issueFailure("repository_error", err)
		if options.json {
			var typed *issueCommandError
			errors.As(err, &typed)
			if outputErr := json.NewEncoder(os.Stdout).Encode(issueEnvelope{Schema: issueJSONSchema, OK: false, Error: typed}); outputErr != nil {
				return outputErr
			}
		}
		return err
	}
	if options.json {
		return json.NewEncoder(os.Stdout).Encode(issueEnvelope{Schema: issueJSONSchema, OK: true, Data: data, Snapshot: snapshot, NextCursor: cursor})
	}
	renderIssueHuman(options, data, cursor)
	return nil
}

// Preserve direct entry points used by existing integrations and tests.
func cmdIssueOpen(args []string) error    { return cmdIssue(append([]string{"open"}, args...)) }
func cmdIssueComment(args []string) error { return cmdIssue(append([]string{"comment"}, args...)) }
func cmdIssueList() error                 { return cmdIssue([]string{"list"}) }
func cmdIssueShow(id string) error        { return cmdIssue([]string{"show", id}) }
func issueIsMutation(command string) bool {
	switch command {
	case "open", "revise", "resolve", "close", "reopen", "comment":
		return true
	}
	return false
}

func parseIssueOptions(args []string) (issueOptions, error) {
	o := issueOptions{limit: 50, kind: "all"}
	invalid := func(message string) (issueOptions, error) {
		return o, issueFailure("invalid_input", errors.New(message))
	}
	if len(args) == 0 {
		return invalid(issueHelp)
	}
	o.command = args[0]
	args = args[1:]
	var commandError string
	switch o.command {
	case "help", "--help", "-h":
		o.command = "help"
	case "open", "list", "operation":
	case "revise", "resolve", "close", "reopen", "comment", "show", "heads", "history", "graph":
		if len(args) == 0 || strings.HasPrefix(args[0], "-") {
			commandError = "issue ID required before flags"
		} else {
			o.id = args[0]
			args = args[1:]
		}
	default:
		commandError = "unknown issue command; run 'hn issue help'"
	}
	flags := flag.NewFlagSet("issue "+o.command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&o.json, "json", false, "JSON output")
	var expected issueExpected
	if issueIsMutation(o.command) || o.command == "operation" {
		flags.StringVar(&o.actor, "actor", "", "expected public signing actor")
	}
	if issueIsMutation(o.command) {
		flags.StringVar(&o.operation, "operation", "", "actor-scoped operation key")
		switch o.command {
		case "open":
			flags.StringVar(&o.body, "body", "", "description")
			flags.StringVar(&o.input, "input", "", "state JSON")
		case "comment":
			flags.StringVar(&o.body, "body", "", "comment")
		default:
			flags.Var(&expected, "expect", "current revision (repeat for resolve)")
			if o.command == "revise" || o.command == "resolve" {
				flags.StringVar(&o.input, "input", "", "state JSON")
			}
			if o.command == "resolve" {
				flags.BoolVar(&o.partial, "partial", false, "staged resolution")
				flags.StringVar(&o.snapshot, "snapshot", "", "observed snapshot")
			}
		}
	} else if o.command == "operation" {
		flags.StringVar(&o.operation, "operation", "", "actor-scoped operation key")
	} else if o.command != "help" {
		if o.command == "list" {
			flags.BoolVar(&o.details, "details", false, "complete issue states")
		}
		flags.IntVar(&o.limit, "limit", 50, "page size")
		flags.StringVar(&o.cursor, "cursor", "", "continuation")
		if o.command == "history" {
			flags.StringVar(&o.kind, "kind", "all", "all|revisions|comments")
		}
	}
	machine := issueMachineRequested(args, flags)
	parseErr := flags.Parse(args)
	o.json = machine
	if commandError != "" {
		return invalid(commandError)
	}
	if parseErr != nil {
		return invalid(parseErr.Error())
	}
	if o.command == "help" {
		if flags.NArg() != 0 {
			return invalid("unexpected help arguments")
		}
		return o, nil
	}
	literal := false
	for _, arg := range args {
		if arg == "--" {
			literal = true
			break
		}
	}
	if !literal {
		for _, arg := range flags.Args() {
			if strings.HasPrefix(arg, "--") {
				return invalid("flags must precede free text; use -- before literal text beginning with --")
			}
		}
	}
	provided := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	o.expected = append([]string{}, expected...)
	if provided["actor"] && !validActorFingerprint(o.actor) {
		return invalid("actor must be a full lowercase public fingerprint")
	}
	if o.command == "operation" && (!provided["actor"] || !provided["operation"]) {
		return invalid("operation lookup requires --actor and --operation")
	}
	if o.id != "" && issueIsMutation(o.command) {
		if err := requireFullEventID(o.id); err != nil {
			return o, issueFailure("invalid_input", err)
		}
	}
	if o.id != "" && !issueIsMutation(o.command) {
		if _, err := issueQueryPrefix(o.id); err != nil {
			return o, err
		}
	}
	if provided["operation"] {
		if len(o.operation) == 0 || len(o.operation) > 128 {
			return invalid("operation key must contain 1..128 printable non-whitespace ASCII bytes")
		}
		for _, c := range o.operation {
			if c < 33 || c > 126 {
				return invalid("operation key must contain printable non-whitespace ASCII bytes")
			}
		}
	}
	if o.limit < 1 || o.limit > 200 {
		return invalid("page limit must be 1..200")
	}
	if o.kind != "all" && o.kind != "revisions" && o.kind != "comments" {
		return invalid("history kind must be all, revisions or comments")
	}
	if len(o.cursor) > 4096 {
		return invalid("cursor is too long")
	}
	switch o.command {
	case "open":
		if provided["input"] {
			if o.input == "" || provided["body"] || flags.NArg() != 0 {
				return invalid("--input requires a file or -, without title/body arguments")
			}
		} else {
			o.title = strings.TrimSpace(strings.Join(flags.Args(), " "))
			if o.title == "" {
				return invalid("issue title cannot be empty")
			}
		}
	case "comment":
		if provided["body"] && flags.NArg() > 0 {
			return invalid("provide comment with --body or positional text, not both")
		}
		if !provided["body"] {
			o.body = strings.Join(flags.Args(), " ")
		}
		if strings.TrimSpace(o.body) == "" {
			return invalid("comment body cannot be empty")
		}
	default:
		if flags.NArg() != 0 {
			return invalid("unexpected positional argument")
		}
	}
	if o.command == "revise" || o.command == "resolve" {
		if o.input == "" {
			return invalid("--input FILE|- is required")
		}
	}
	if o.command == "revise" || o.command == "close" || o.command == "reopen" {
		if len(o.expected) != 1 {
			return invalid("exactly one --expect revision is required")
		}
	}
	if o.command == "resolve" {
		if len(o.expected) < 2 || len(o.expected) > maxIssueParents {
			return invalid("resolve requires 2..200 expected revisions")
		}
		if o.partial != (o.snapshot != "") {
			return invalid("--partial and --snapshot must be supplied together")
		}
		if o.partial && !validEventID(o.snapshot) {
			return invalid("snapshot must be the full token returned by a query")
		}
	}
	sort.Strings(o.expected)
	for i, id := range o.expected {
		if !validEventID(id) || (i > 0 && o.expected[i-1] == id) {
			return invalid("expected revisions must be distinct full event IDs")
		}
	}
	if o.input != "" {
		state, err := readIssueState(o.input)
		if err != nil {
			return o, err
		}
		o.state = &state
	}
	return o, nil
}

func readIssueState(path string) (IssueState, error) {
	var input io.Reader = os.Stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return IssueState{}, issueFailure("invalid_input", err)
		}
		defer file.Close()
		input = file
	}
	raw, err := io.ReadAll(io.LimitReader(input, maxIssuePayload+1))
	if err != nil {
		return IssueState{}, issueFailure("invalid_input", err)
	}
	if len(raw) > maxIssuePayload {
		return IssueState{}, issueFailure("resource_limit", errors.New("issue input exceeds 256 KiB"))
	}
	if err := validateIssueJSONUnicode(raw); err != nil {
		return IssueState{}, issueFailure("invalid_input", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := parseIssueJSONValue(decoder, 0)
	if err != nil {
		return IssueState{}, issueFailure("invalid_input", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return IssueState{}, issueFailure("invalid_input", errors.New("input must contain exactly one JSON value"))
	}
	object, ok := value.(map[string]any)
	if !ok {
		return IssueState{}, issueFailure("invalid_input", errors.New("issue input must be an object"))
	}
	if err := issueInputKeys(object, []string{"title", "body", "status", "criteria", "labels", "assignees", "relations", "metadata"}); err != nil {
		return IssueState{}, issueFailure("invalid_input", err)
	}
	for name, keys := range map[string][]string{"criteria": {"id", "text"}, "relations": {"kind", "target"}} {
		if rows, ok := object[name].([]any); ok {
			for _, row := range rows {
				fields, ok := row.(map[string]any)
				if !ok {
					return IssueState{}, issueFailure("invalid_input", fmt.Errorf("%s must contain objects", name))
				}
				if err := issueInputKeys(fields, keys); err != nil {
					return IssueState{}, issueFailure("invalid_input", err)
				}
			}
		}
	}
	state := IssueState{Status: "open"}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return IssueState{}, issueFailure("invalid_input", err)
	}
	canonical, err := CanonicalIssueState(state)
	return canonical, issueFailure("invalid_input", err)
}

// encoding/json repairs malformed Unicode. Reject it before decoding new input
// so the immutable signed state preserves the submitted strings losslessly.
func validateIssueJSONUnicode(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("issue JSON must contain valid UTF-8")
	}
	if !json.Valid(raw) {
		return errors.New("issue input must be valid JSON")
	}
	inString := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue // Includes escaped quotes and literal backslashes.
		}
		unit, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4 // json.Valid guarantees a complete hexadecimal escape.
		if unit >= 0xdc00 && unit <= 0xdfff {
			return errors.New("unpaired low surrogate in issue JSON")
		}
		if unit < 0xd800 || unit > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return errors.New("unpaired high surrogate in issue JSON")
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return errors.New("unpaired high surrogate in issue JSON")
		}
		i += 6
	}
	return nil
}

func issueInputKeys(object map[string]any, allowed []string) error {
	for key := range object {
		found := false
		for _, name := range allowed {
			if key == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	return nil
}
func parseIssueJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 16 {
		return nil, errors.New("JSON nesting exceeds 16")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, errors.New("null is not a valid issue field value")
	}
	delim, container := token.(json.Delim)
	if !container {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("object key must be a string")
			}
			if _, exists := object[key]; exists {
				return nil, fmt.Errorf("duplicate JSON field %q", key)
			}
			value, err := parseIssueJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := parseIssueJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return array, nil
	default:
		return nil, errors.New("unexpected JSON delimiter")
	}
}

type issueMutationResult struct {
	IssueID   string `json:"issue_id"`
	EventID   string `json:"event_id"`
	Replayed  bool   `json:"replayed"`
	Conflict  bool   `json:"conflict"`
	HeadCount int    `json:"head_count"`
}

func issueEventKind(command string) string {
	if command == "open" {
		return "issue.open"
	}
	if command == "comment" {
		return "issue.comment"
	}
	return "issue.revise"
}
func executeIssueMutation(o issueOptions) (any, error) {
	identity, err := loadIdentity()
	if err != nil {
		return nil, issueFailure("repository_error", err)
	}
	if err := checkIssueActor(o, identity); err != nil {
		return nil, err
	}
	captured, err := nextEvent(identity, issueEventKind(o.command))
	if err != nil {
		return nil, issueFailure("repository_error", err)
	}
	events, snapshot, err := collectIssueEvents()
	if err != nil {
		return nil, issueFailure("repository_error", err)
	}
	return applyIssueMutation(o, identity, captured, events, snapshot)
}

// applyIssueMutation keeps the actor sequence captured before the verified read.
// Keeping this seam explicit also allows deterministic race-boundary tests.
func checkIssueActor(o issueOptions, identity *Identity) error {
	if o.actor != "" && o.actor != identity.Actor {
		return issueFailure("actor_mismatch", errors.New("active signing actor differs from the expected actor; repair the binding"))
	}
	return nil
}

func applyIssueMutation(o issueOptions, identity *Identity, captured Event, events []StoredEvent, snapshot string) (*issueMutationResult, error) {
	if err := checkIssueActor(o, identity); err != nil {
		return nil, err
	}
	cat, err := BuildIssueCatalog(events)
	if err != nil {
		return nil, issueFailure("invalid_history", err)
	}
	event := captured
	event.Intent = o.command
	event.Subject = o.id
	event.Parents = append([]string{}, o.expected...)
	event.Operation = o.operation
	switch o.command {
	case "open":
		state := IssueState{Title: o.title, Body: o.body, Status: "open"}
		if o.state != nil {
			state = *o.state
		}
		canonical, err := CanonicalIssueState(state)
		if err != nil {
			return nil, issueFailure("invalid_input", err)
		}
		event.Issue = &canonical
		event.Title = canonical.Title
		event.Body = canonical.Body
	case "comment":
		event.Body = o.body
	case "revise", "resolve":
		event.Issue = o.state
	}
	var prior *StoredEvent
	if o.operation != "" {
		for i := range events {
			stored := &events[i]
			if stored.Event.Actor == identity.Actor && stored.Event.Operation == o.operation {
				if prior != nil && prior.ID != stored.ID {
					return nil, issueFailure("operation_conflict", errors.New("actor operation key has multiple signed records; inspect history"))
				}
				prior = stored
			}
		}
	}
	if prior != nil {
		if prior.Event.Intent != o.command || prior.Event.Kind != event.Kind || prior.Event.Subject != event.Subject || !equalIssueIDs(prior.Event.Parents, event.Parents) {
			return nil, issueFailure("operation_conflict", errors.New("operation key was already used for different intent, issue or expectations"))
		}
		// Convenience retry uses the originally signed state, before current heads.
		if o.command == "close" || o.command == "reopen" {
			event.Issue = prior.Event.Issue
		}
		wanted, err := IssueRequestDigest(event)
		if err != nil {
			return nil, issueFailure("invalid_input", err)
		}
		recorded, err := IssueRequestDigest(prior.Event)
		if err != nil || recorded != prior.Event.Request {
			return nil, issueFailure("invalid_history", errors.New("recorded operation digest does not match signed semantics"))
		}
		if wanted != recorded {
			return nil, issueFailure("operation_conflict", errors.New("operation key was already used for different content"))
		}
		id := issueRootID(*prior)
		view := cat.Issues[id]
		return &issueMutationResult{id, prior.ID, true, view.Conflict, len(view.Heads)}, nil
	}
	view := cat.Issues[o.id]
	if o.command != "open" && view == nil {
		return nil, issueFailure("not_found", errors.New("issue not found"))
	}
	if o.command == "revise" || o.command == "close" || o.command == "reopen" {
		if view.Conflict {
			return nil, issueFailure("issue_conflict", errors.New("issue has concurrent heads; inspect heads and resolve explicitly"))
		}
		if len(o.expected) != 1 || len(view.Heads) != 1 || o.expected[0] != view.Heads[0].ID {
			return nil, issueFailure("stale_revision", errors.New("expected revision is no longer the current head"))
		}
		if o.command == "close" || o.command == "reopen" {
			state := *view.State
			if o.command == "close" {
				state.Status = "closed"
			} else {
				state.Status = "open"
			}
			event.Issue = &state
		}
	}
	if o.command == "resolve" {
		current := map[string]bool{}
		for _, h := range view.Heads {
			current[h.ID] = true
		}
		if o.partial && snapshot != o.snapshot {
			return nil, issueFailure("stale_revision", errors.New("partial resolution snapshot changed; inspect heads again"))
		}
		if !o.partial && len(o.expected) != len(current) {
			return nil, issueFailure("stale_revision", errors.New("resolution must name every current head"))
		}
		for _, id := range o.expected {
			if !current[id] {
				return nil, issueFailure("stale_revision", errors.New("resolution contains a non-current head"))
			}
		}
	}
	if event.Issue != nil {
		canonical, err := CanonicalIssueState(*event.Issue)
		if err != nil {
			return nil, issueFailure("invalid_input", err)
		}
		event.Issue = &canonical
		if err := ValidateIssueCandidate(cat, o.id, o.expected, canonical); err != nil {
			return nil, issueFailure("invalid_input", err)
		}
	}
	if event.Operation != "" {
		event.Request, err = IssueRequestDigest(event)
		if err != nil {
			return nil, issueFailure("invalid_input", err)
		}
	}
	if err := validateEventContent(event); err != nil {
		return nil, issueFailure("invalid_input", err)
	}
	stored, err := appendEvent(event, identity)
	if err != nil {
		next, nextErr := nextEvent(identity, event.Kind)
		if nextErr == nil && (next.Previous != captured.Previous || next.Sequence != captured.Sequence) {
			return nil, issueFailure("concurrent_update", errors.New("actor history changed; retry the same operation and expectations"))
		}
		return nil, issueFailure("repository_error", err)
	}
	after, err := BuildIssueCatalog(append(append([]StoredEvent{}, events...), *stored))
	if err != nil {
		return nil, issueFailure("invalid_history", err)
	}
	id := issueRootID(*stored)
	view = after.Issues[id]
	return &issueMutationResult{id, stored.ID, false, view.Conflict, len(view.Heads)}, nil
}
func equalIssueIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type issueCursor struct {
	Version  int    `json:"version"`
	Snapshot string `json:"snapshot"`
	Query    string `json:"query"`
	Offset   int    `json:"offset"`
}
type issuePage struct {
	Items      any    `json:"items"`
	Total      int    `json:"total"`
	IssueID    string `json:"issue_id,omitempty"`
	Conflict   bool   `json:"conflict"`
	HeadCount  int    `json:"head_count,omitempty"`
	CycleCount int    `json:"cycle_count"`
}
type issueSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Creator   string `json:"creator"`
	Conflict  bool   `json:"conflict"`
	HeadCount int    `json:"head_count"`
}
type issueShow struct {
	ID            string      `json:"id"`
	Creator       string      `json:"creator"`
	State         *IssueState `json:"state"`
	Conflict      bool        `json:"conflict"`
	HeadIDs       []string    `json:"head_ids"`
	HeadCount     int         `json:"head_count"`
	RevisionCount int         `json:"revision_count"`
	CommentCount  int         `json:"comment_count"`
}
type issueHistoryItem struct {
	ID        string      `json:"id"`
	Actor     string      `json:"actor"`
	Kind      string      `json:"kind"`
	Timestamp string      `json:"timestamp"`
	Parents   []string    `json:"parents"`
	State     *IssueState `json:"state,omitempty"`
	Body      string      `json:"body,omitempty"`
	Intent    string      `json:"intent,omitempty"`
	Operation string      `json:"operation,omitempty"`
}
type issueGraphItem struct {
	Type      string     `json:"type"`
	Edge      *IssueEdge `json:"edge,omitempty"`
	Kind      string     `json:"kind,omitempty"`
	Component string     `json:"component,omitempty"`
	IssueID   string     `json:"issue_id,omitempty"`
}

type issueOperationResult struct {
	IssueID   string      `json:"issue_id"`
	EventID   string      `json:"event_id"`
	Actor     string      `json:"actor"`
	Operation string      `json:"operation"`
	Intent    string      `json:"intent"`
	Kind      string      `json:"kind"`
	Timestamp string      `json:"timestamp"`
	Parents   []string    `json:"parents"`
	State     *IssueState `json:"state,omitempty"`
	Body      *string     `json:"body,omitempty"`
	Request   string      `json:"request"`
}

type issueDetail struct {
	OpeningMetadata map[string]string `json:"opening_metadata"`
	ID              string            `json:"id"`
	Creator         string            `json:"creator"`
	State           *IssueState       `json:"state"`
	Conflict        bool              `json:"conflict"`
	HeadCount       int               `json:"head_count"`
}

func observeIssueOperation(events []StoredEvent, actor, operation string) (*issueOperationResult, error) {
	var match *StoredEvent
	for i := range events {
		e := &events[i]
		if e.Event.Actor != actor || e.Event.Operation != operation {
			continue
		}
		if match != nil && match.ID != e.ID {
			return nil, issueFailure("operation_conflict", errors.New("actor operation key has multiple signed records; inspect history"))
		}
		match = e
	}
	if match == nil {
		return nil, issueFailure("not_found", errors.New("actor operation was not found in verified history"))
	}
	e := match.Event
	result := &issueOperationResult{IssueID: issueRootID(*match), EventID: match.ID, Actor: e.Actor,
		Operation: e.Operation, Intent: e.Intent, Kind: e.Kind, Timestamp: e.Timestamp,
		Parents: append([]string{}, e.Parents...), State: e.Issue, Request: e.Request}
	if e.Kind == "issue.comment" {
		result.Body = &e.Body
	}
	return result, nil
}

func issueQueryFingerprint(o issueOptions) string {
	query := []string{o.command, o.id, o.kind, strconv.Itoa(o.limit)}
	if o.details {
		query = append(query, "details")
	}
	encoded, _ := json.Marshal(query)
	return eventID(encoded)
}
func issuePageBounds(o issueOptions, snapshot string, total int) (int, int, string, error) {
	start := 0
	query := issueQueryFingerprint(o)
	if o.cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(o.cursor)
		if err != nil {
			return 0, 0, "", issueFailure("invalid_input", errors.New("invalid cursor encoding"))
		}
		var cursor issueCursor
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.Query != query || cursor.Offset < 0 {
			return 0, 0, "", issueFailure("invalid_input", errors.New("cursor does not match this query or range"))
		}
		if err := ensureJSONEnd(decoder); err != nil {
			return 0, 0, "", issueFailure("invalid_input", err)
		}
		if cursor.Snapshot != snapshot {
			return 0, 0, "", issueFailure("stale_cursor", errors.New("accepted snapshot changed; restart query"))
		}
		if cursor.Offset >= total {
			return 0, 0, "", issueFailure("invalid_input", errors.New("cursor offset is outside this result"))
		}
		start = cursor.Offset
	}
	end := start + o.limit
	if end > total {
		end = total
	}
	next := ""
	if end < total {
		raw, _ := json.Marshal(issueCursor{1, snapshot, query, end})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return start, end, next, nil
}
func issueQueryPrefix(query string) (string, error) {
	prefix := strings.TrimPrefix(query, "sha256:")
	if prefix == "" || len(prefix) > 64 {
		return "", issueFailure("invalid_input", errors.New("invalid issue ID prefix"))
	}
	for _, r := range prefix {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return "", issueFailure("invalid_input", errors.New("invalid issue ID prefix"))
		}
	}
	return prefix, nil
}
func resolveIssueQuery(cat *IssueCatalog, query string) (string, error) {
	prefix, err := issueQueryPrefix(query)
	if err != nil {
		return "", err
	}
	found := ""
	for _, id := range cat.IDs {
		if strings.HasPrefix(strings.TrimPrefix(id, "sha256:"), prefix) {
			if found != "" {
				return "", issueFailure("ambiguous_id", errors.New("issue ID prefix is ambiguous"))
			}
			found = id
		}
	}
	if found == "" {
		return "", issueFailure("not_found", errors.New("issue not found"))
	}
	return found, nil
}
func executeIssueQuery(o issueOptions) (any, string, string, error) {
	events, snapshot, err := collectIssueEvents()
	if err != nil {
		return nil, "", "", issueFailure("repository_error", err)
	}
	if o.command == "operation" {
		result, err := observeIssueOperation(events, o.actor, o.operation)
		return result, snapshot, "", err
	}
	cat, err := BuildIssueCatalog(events)
	if err != nil {
		return nil, "", "", issueFailure("invalid_history", err)
	}
	var view *IssueView
	if o.command != "list" {
		id, err := resolveIssueQuery(cat, o.id)
		if err != nil {
			return nil, "", "", err
		}
		o.id = id
		view = cat.Issues[id]
	}
	var items any
	var total int
	cycleCount := 0
	switch o.command {
	case "list":
		if o.details {
			rows := []issueDetail{}
			for _, id := range cat.IDs {
				v := cat.Issues[id]
				opening := map[string]string{}
				for _, historical := range v.History {
					if historical.ID == v.ID && historical.Event.Issue != nil {
						for key, value := range historical.Event.Issue.Metadata {
							opening[key] = value
						}
						break
					}
				}
				rows = append(rows, issueDetail{OpeningMetadata: opening, ID: v.ID, Creator: v.Creator,
					State: v.State, Conflict: v.Conflict, HeadCount: len(v.Heads)})
			}
			items, total = rows, len(rows)
			break
		}
		rows := []issueSummary{}
		for _, id := range cat.IDs {
			v := cat.Issues[id]
			row := issueSummary{ID: id, Creator: v.Creator, Conflict: v.Conflict, HeadCount: len(v.Heads)}
			if v.State != nil {
				row.Title = v.State.Title
				row.Status = v.State.Status
			}
			rows = append(rows, row)
		}
		items = rows
		total = len(rows)
	case "show", "heads":
		items = view.Heads
		total = len(view.Heads)
	case "history":
		rows := []issueHistoryItem{}
		if o.kind != "comments" {
			for _, e := range view.History {
				state := e.Event.Issue
				if state != nil {
					canonical, err := CanonicalIssueState(*state)
					if err != nil {
						return nil, "", "", issueFailure("invalid_history", err)
					}
					state = &canonical
				}
				if state == nil {
					legacy := IssueState{Title: e.Event.Title, Body: e.Event.Body, Status: "open", Criteria: []IssueCriterion{}, Labels: []string{}, Assignees: []string{}, Relations: []IssueRelation{}, Metadata: map[string]string{}}
					state = &legacy
				}
				rows = append(rows, issueHistoryItem{ID: e.ID, Actor: e.Event.Actor, Kind: e.Event.Kind, Timestamp: e.Event.Timestamp, Parents: append([]string{}, e.Event.Parents...), State: state, Intent: e.Event.Intent, Operation: e.Event.Operation})
			}
		}
		if o.kind != "revisions" {
			for _, e := range view.Comments {
				rows = append(rows, issueHistoryItem{ID: e.ID, Actor: e.Event.Actor, Kind: e.Event.Kind, Timestamp: e.Event.Timestamp, Parents: []string{}, Body: e.Event.Body, Intent: e.Event.Intent, Operation: e.Event.Operation})
			}
		}
		items = rows
		total = len(rows)
	case "graph":
		rows := []issueGraphItem{}
		for _, edge := range cat.GraphFor(o.id) {
			copy := edge
			rows = append(rows, issueGraphItem{Type: "edge", Edge: &copy})
		}
		for _, cycle := range cat.Cycles {
			member := false
			for _, id := range cycle.Issues {
				if id == o.id {
					member = true
					break
				}
			}
			if !member {
				continue
			}
			cycleCount++
			encoded, _ := json.Marshal(cycle)
			component := eventID(encoded)
			for _, id := range cycle.Issues {
				rows = append(rows, issueGraphItem{Type: "cycle_member", Kind: cycle.Kind, Component: component, IssueID: id})
			}
		}
		items = rows
		total = len(rows)
	}
	start, end, next, err := issuePageBounds(o, snapshot, total)
	if err != nil {
		return nil, "", "", err
	}
	switch rows := items.(type) {
	case []issueDetail:
		items = rows[start:end]
	case []issueSummary:
		items = rows[start:end]
	case []IssueHead:
		items = rows[start:end]
	case []issueHistoryItem:
		items = rows[start:end]
	case []issueGraphItem:
		items = rows[start:end]
	}
	if o.command == "show" {
		ids := []string{}
		for _, head := range view.Heads[start:end] {
			ids = append(ids, head.ID)
		}
		return issueShow{view.ID, view.Creator, view.State, view.Conflict, ids, len(view.Heads), len(view.History), len(view.Comments)}, snapshot, next, nil
	}
	page := issuePage{Items: items, Total: total, IssueID: o.id, CycleCount: cycleCount}
	if view != nil {
		page.Conflict = view.Conflict
		page.HeadCount = len(view.Heads)
	}
	return page, snapshot, next, nil
}

func renderIssueHuman(o issueOptions, data any, cursor string) {
	switch value := data.(type) {
	case *issueOperationResult:
		fmt.Printf("Operation %s by %s\nIssue: %s\nEvent: %s (%s, %s)\n", oneLine(value.Operation), value.Actor, value.IssueID, value.EventID, value.Intent, value.Timestamp)
		if value.State != nil {
			renderIssueState(*value.State)
		}
		if value.Body != nil {
			fmt.Println(safeText(*value.Body))
		}
	case *issueMutationResult:
		verb := "Updated"
		if o.command == "open" {
			verb = "Opened"
		}
		if o.command == "comment" {
			verb = "Commented on"
		}
		fmt.Printf("%s issue %s with event %s\n", verb, shortID(value.IssueID), shortID(value.EventID))
		if o.command == "open" {
			title := o.title
			if o.state != nil {
				title = o.state.Title
			}
			fmt.Println(oneLine(title))
		}
		if value.Replayed {
			fmt.Println("Replayed the original operation.")
		}
		if value.Conflict {
			fmt.Printf("Conflict remains: %d heads. Inspect with hn issue heads %s\n", value.HeadCount, value.IssueID)
		}
	case issueShow:
		fmt.Printf("Issue %s\n", shortID(value.ID))
		fmt.Printf("Opened by %s\n", oneLine(value.Creator))
		if value.State != nil {
			renderIssueState(*value.State)
		} else {
			fmt.Printf("Conflict: %d heads; inspect hn issue heads %s\n", value.HeadCount, value.ID)
		}
		for _, id := range value.HeadIDs {
			fmt.Printf("Head: %s\n", id)
		}
		fmt.Printf("%d revisions, %d comments. Inspect with hn issue history %s\n", value.RevisionCount, value.CommentCount, value.ID)
	case issuePage:
		switch rows := value.Items.(type) {
		case []issueDetail:
			for _, row := range rows {
				fmt.Printf("Issue %s (opened by %s)\n", row.ID, row.Creator)
				if row.State != nil {
					renderIssueState(*row.State)
				} else {
					fmt.Printf("Conflict: %d heads\n", row.HeadCount)
				}
			}
		case []issueSummary:
			if len(rows) == 0 {
				fmt.Println("No issues.")
			}
			for _, row := range rows {
				if row.Conflict {
					fmt.Printf("%s  [conflict: %d heads]\n", shortID(row.ID), row.HeadCount)
				} else {
					fmt.Printf("%s  %s  [%s]\n", shortID(row.ID), oneLine(row.Title), row.Status)
				}
			}
		case []IssueHead:
			for _, head := range rows {
				fmt.Printf("%s (%s)\n", head.ID, oneLine(head.Actor))
				renderIssueState(head.State)
			}
		case []issueHistoryItem:
			for _, row := range rows {
				fmt.Printf("%s %s (%s)\n", row.ID, row.Kind, oneLine(row.Actor))
				if row.State != nil {
					fmt.Printf("%s [%s]\n", oneLine(row.State.Title), row.State.Status)
				}
				if row.Body != "" {
					fmt.Println(safeText(row.Body))
				}
			}
		case []issueGraphItem:
			for _, row := range rows {
				if row.Edge != nil {
					edge := row.Edge
					fmt.Printf("%s %s %s (revision %s, ambiguous=%t)\n", edge.Source, edge.Kind, edge.Target, edge.Revision, edge.Ambiguous)
				} else {
					fmt.Printf("%s cycle %s member %s\n", row.Kind, row.Component, row.IssueID)
				}
			}
		}
		fmt.Printf("Total records: %d\n", value.Total)
	}
	if cursor != "" {
		fmt.Printf("Continue with --cursor %s using the same query and limit.\n", cursor)
	}
}

func renderIssueState(state IssueState) {
	fmt.Printf("%s [%s]\n", oneLine(state.Title), state.Status)
	if state.Body != "" {
		fmt.Printf("\n%s\n", safeText(state.Body))
	}
	for _, criterion := range state.Criteria {
		fmt.Printf("Criterion %s: %s\n", oneLine(criterion.ID), oneLine(criterion.Text))
	}
	if len(state.Labels) > 0 {
		fmt.Printf("Labels: %s\n", oneLine(strings.Join(state.Labels, ", ")))
	}
	if len(state.Assignees) > 0 {
		fmt.Printf("Assignees: %s\n", oneLine(strings.Join(state.Assignees, ", ")))
	}
	for _, relation := range state.Relations {
		fmt.Printf("%s: %s\n", relation.Kind, relation.Target)
	}
	keys := make([]string, 0, len(state.Metadata))
	for key := range state.Metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("%s: %s\n", oneLine(key), oneLine(state.Metadata[key]))
	}
}
