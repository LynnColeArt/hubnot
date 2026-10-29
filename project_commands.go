package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"
)

func init() {
	NativeResourceCommands[ResourceProject] = cmdProject
	NativeResourceCommands[ResourceProjectItem] = cmdProjectItem
}

const projectHelp = `hn project open --input FILE|- | --title TEXT [--body TEXT] [--actor ACTOR] [--operation KEY] [--json]
hn project revise FULL_ID --expect FULL_REV --input FILE|- | --title TEXT | --body TEXT [--actor ACTOR] [--operation KEY] [--json]
hn project resolve FULL_ID --expect HEAD_A --expect HEAD_B ... --input FILE|- [--actor ACTOR] [--operation KEY] [--json]
hn project list|show|heads|history [ID] [--limit 1..200] [--cursor TOKEN] [--json]
hn project operation --actor ACTOR --operation KEY [--json]
hn project item <command> ... (same as hn project-item)
Fields, options, views and metadata use exact full-state --input JSON; convenience edits preserve them.
` + projectItemHelp

const projectItemHelp = `hn project-item add --project FULL_ID [--project-revision FULL_REV] (--target issue:FULL_ID|proposal:FULL_ID|discussion:FULL_ID|external:URL | --draft-title TEXT [--draft-body TEXT]) [--input VALUES_FILE|-] [--position DECIMAL] [--actor ACTOR] [--operation KEY] [--json]
hn project-item archive FULL_ID --expect FULL_REV [--actor ACTOR] [--operation KEY] [--json]
hn project-item open --input FULL_STATE_FILE|- [--actor ACTOR] [--operation KEY] [--json]
hn project-item revise|resolve FULL_ID --expect FULL_REV ... --input FULL_STATE_FILE|- [--actor ACTOR] [--operation KEY] [--json]
hn project-item list [--project FULL_ID] [--limit 1..200] [--cursor TOKEN] [--json]
hn project-item show|heads|history ID [--limit 1..200] [--cursor TOKEN] [--json]
hn project-item operation --actor ACTOR --operation KEY [--json]
Add --input contains a map of field IDs to typed values, e.g. {"score":{"type":"number","number":"2.5"}}.
Add pins the sole project head; --project-revision explicitly selects a historical or competing schema revision.
An operation retry retains its original implicit schema pin. Changing explicit input with that key fails.
Item revise/resolve uses full state, including explicit project_revision to migrate schema. Archive preserves all other state.
List includes archived items and all conflict heads; ordinary items sort by exact numeric position then ID, conflicts follow by ID.
Pagination binds project selection, limit and accepted-ref snapshot. Show/heads/history accept unambiguous prefixes (8+ hex digits).
All writes require full IDs and exact --expect guards. Resolve requires every current head; no timestamp chooses a winner.
`

// Follow flag token boundaries so text such as --input or --json remains a
// literal value when supplied to --title, --draft-title or another string flag.
func projectFlagValue(args []string, name string) (string, bool) {
	value, found := "", false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			continue
		}
		option, v, assigned := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !assigned {
			if option == "json" || option == "help" {
				v = "true"
			} else if i+1 < len(args) {
				i++
				v = args[i]
			}
		}
		if option == name {
			value, found = v, true
		}
	}
	return value, found
}
func projectHasFlag(args []string, name string) bool {
	_, found := projectFlagValue(args, name)
	return found
}
func projectMachineRequested(args []string) bool {
	value, found := projectFlagValue(args, "json")
	if !found {
		return false
	}
	machine, err := strconv.ParseBool(value)
	return err != nil || machine
}
func projectHelpRequested(args []string) bool {
	return len(args) == 0 || args[0] == "help" || args[0] == "--help" || len(args) == 2 && args[1] == "--help"
}
func cmdProject(args []string) error {
	if len(args) > 0 && args[0] == "item" {
		return cmdProjectItem(args[1:])
	}
	if projectHelpRequested(args) {
		fmt.Print(projectHelp)
		return nil
	}
	if (args[0] == "open" || args[0] == "revise") && !projectHasFlag(args, "input") {
		return projectCommandOutput(args, func() (any, error) { return projectConvenience(args) })
	}
	return CmdResource(ResourceProject, args)
}
func cmdProjectItem(args []string) error {
	if projectHelpRequested(args) {
		fmt.Print(projectItemHelp)
		return nil
	}
	switch args[0] {
	case "add":
		return projectCommandOutput(args, func() (any, error) { return projectItemAdd(args[1:]) })
	case "archive":
		return projectCommandOutput(args, func() (any, error) { return projectItemArchive(args[1:]) })
	case "list":
		return projectCommandOutput(args, func() (any, error) { return projectItemList(args[1:]) })
	default:
		return CmdResource(ResourceProjectItem, args)
	}
}
func projectCommandOutput(args []string, execute func() (any, error)) error {
	data, err := execute()
	if projectMachineRequested(args) {
		envelope := map[string]any{"ok": err == nil, "data": data}
		if err != nil {
			code := "resource_error"
			var typed *issueCommandError
			if errors.As(err, &typed) {
				code = typed.Code
			}
			envelope["error"] = map[string]string{"code": code, "message": err.Error()}
		}
		if writeErr := json.NewEncoder(os.Stdout).Encode(envelope); writeErr != nil {
			return writeErr
		}
	} else if err == nil {
		raw, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(raw))
	}
	return err
}
func projectFlags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}
func projectParse(f *flag.FlagSet, args []string) error {
	if err := f.Parse(args); err != nil {
		return usageError(err.Error())
	}
	if f.NArg() != 0 {
		return usageError("unexpected positional arguments")
	}
	return nil
}
func projectMutationFlags(f *flag.FlagSet, actor, key *string) {
	f.StringVar(actor, "actor", "", "")
	f.StringVar(key, "operation", "", "")
	f.Bool("json", false, "")
}
func projectConvenience(args []string) (any, error) {
	command := args[0]
	rest := args[1:]
	id := ""
	if command == "revise" && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		id = rest[0]
		rest = rest[1:]
	}
	f := projectFlags("project " + command)
	var title, body, actor, key string
	expected := issueExpected{}
	f.StringVar(&title, "title", "", "")
	f.StringVar(&body, "body", "", "")
	f.Var(&expected, "expect", "")
	projectMutationFlags(f, &actor, &key)
	if err := projectParse(f, rest); err != nil {
		return nil, err
	}
	titleSet, bodySet := false, false
	f.Visit(func(v *flag.Flag) { titleSet = titleSet || v.Name == "title"; bodySet = bodySet || v.Name == "body" })
	state := ProjectState{Schema: "hn.project/1"}
	if command == "open" {
		if !titleSet || len(expected) != 0 {
			return nil, usageError("project open requires --title and no expectations")
		}
	} else {
		if !titleSet && !bodySet {
			return nil, usageError("project revise requires --title and/or --body, or full-state --input")
		}
		raw, err := projectExpectedState(ResourceProject, id, expected)
		if err != nil {
			return nil, err
		}
		if err = DecodeResourceState(ResourceProject, raw, &state); err != nil {
			return nil, err
		}
	}
	if titleSet {
		state.Title = title
	}
	if bodySet {
		state.Body = body
	}
	raw, err := ResourceStateJSON(ResourceProject, state)
	if err != nil {
		return nil, issueFailure("invalid_input", err)
	}
	return MutateResource(ResourceMutation{Family: ResourceProject, Intent: command, ID: id, Expected: expected, State: raw, Actor: actor, Operation: key})
}

// Convenience changes derive only from the explicitly expected historical state,
// so retrying an operation after later revisions does not refresh its input.
func projectExpectedState(family, id string, expected []string) (json.RawMessage, error) {
	if !validEventID(id) || len(expected) != 1 || !validEventID(expected[0]) {
		return nil, usageError("mutation requires full resource ID and exactly one full --expect revision")
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		return nil, err
	}
	cat, err := buildGovernedResourceCatalog(events)
	if err != nil {
		return nil, err
	}
	view := cat.Resources[id]
	if view == nil || view.Family != family {
		return nil, issueFailure("not_found", errors.New("resource not found in requested family"))
	}
	for _, v := range view.History {
		if v.ID == expected[0] {
			return json.RawMessage(v.Event.Body), nil
		}
	}
	return nil, issueFailure("invalid_input", errors.New("expected revision belongs to another resource or is unavailable"))
}
func projectReadInput(path string) ([]byte, error) {
	var reader io.Reader = os.Stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader = file
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxResourcePayload+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxResourcePayload {
		return nil, errors.New("project input exceeds 256 KiB")
	}
	return raw, nil
}
func projectItemAdd(args []string) (any, error) {
	f := projectFlags("project-item add")
	var project, revision, target, title, body, path, position, actor, key string
	f.StringVar(&project, "project", "", "")
	f.StringVar(&revision, "project-revision", "", "")
	f.StringVar(&target, "target", "", "")
	f.StringVar(&title, "draft-title", "", "")
	f.StringVar(&body, "draft-body", "", "")
	f.StringVar(&path, "input", "", "")
	f.StringVar(&position, "position", "0", "")
	projectMutationFlags(f, &actor, &key)
	if err := projectParse(f, args); err != nil {
		return nil, err
	}
	set := map[string]bool{}
	f.Visit(func(v *flag.Flag) { set[v.Name] = true })
	if !validEventID(project) || set["project-revision"] && !validEventID(revision) {
		return nil, usageError("add requires full --project and optional full --project-revision")
	}
	if !validResourceDecimal(position) {
		return nil, issueFailure("invalid_input", errors.New("position must be a canonical decimal string"))
	}
	state := ProjectItemState{Schema: "hn.project-item/1", ProjectID: project, ProjectRevision: revision, Position: position}
	if set["draft-title"] {
		if set["target"] {
			return nil, usageError("choose exactly one --target or --draft-title")
		}
		state.Target = ProjectTarget{Kind: "draft", Title: title, Body: body}
	} else {
		if !set["target"] || set["draft-body"] {
			return nil, usageError("choose --target or --draft-title [--draft-body]")
		}
		kind, value, ok := strings.Cut(target, ":")
		if !ok {
			return nil, usageError("target requires kind:FULL_ID or external:URL")
		}
		state.Target = ProjectTarget{Kind: kind, ID: value}
		if kind == "external" {
			state.Target.ID = ""
			state.Target.URL = value
		}
	}
	if set["input"] {
		if path == "" {
			return nil, usageError("--input requires a path or -")
		}
		raw, err := projectReadInput(path)
		if err != nil {
			return nil, err
		}
		if err = strictResourceJSON(raw, &state.Values, maxResourcePayload); err != nil {
			return nil, issueFailure("invalid_input", err)
		}
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		return nil, err
	}
	// A prior immutable request is the only authority for an omitted schema on
	// retry. Rebuild all caller-supplied fields and let the engine compare digests.
	identity, err := loadIdentity()
	if err != nil {
		return nil, err
	}
	if actor != "" && actor != identity.Actor {
		return nil, issueFailure("actor_mismatch", errors.New("active signing actor differs from expected actor"))
	}
	actor = identity.Actor
	if key != "" {
		prior, err := findResourceOperation(events, actor, key)
		if err != nil {
			return nil, err
		}
		if prior != nil {
			if prior.Event.Kind != "project-item.open" {
				return nil, issueFailure("operation_conflict", errors.New("operation belongs to another kind"))
			}
			var original ProjectItemState
			if err = DecodeResourceState(ResourceProjectItem, []byte(prior.Event.Body), &original); err != nil {
				return nil, err
			}
			if state.ProjectRevision == "" {
				state.ProjectRevision = original.ProjectRevision
			}
		}
	}
	if state.ProjectRevision == "" {
		cat, err := buildGovernedResourceCatalog(events)
		if err != nil {
			return nil, err
		}
		p := cat.Resources[project]
		if p == nil || p.Family != ResourceProject {
			return nil, issueFailure("not_found", errors.New("project not found"))
		}
		if p.Conflict {
			return nil, issueFailure("resource_conflict", errors.New("project has concurrent heads; select an explicit --project-revision or resolve"))
		}
		state.ProjectRevision = p.Heads[0].ID
	}
	raw, err := ResourceStateJSON(ResourceProjectItem, state)
	if err != nil {
		return nil, issueFailure("invalid_input", err)
	}
	return MutateResource(ResourceMutation{Family: ResourceProjectItem, Intent: "open", State: raw, Actor: actor, Operation: key})
}
func projectItemArchive(args []string) (any, error) {
	id := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id = args[0]
		args = args[1:]
	}
	f := projectFlags("project-item archive")
	expected := issueExpected{}
	var actor, key string
	f.Var(&expected, "expect", "")
	projectMutationFlags(f, &actor, &key)
	if err := projectParse(f, args); err != nil {
		return nil, err
	}
	raw, err := projectExpectedState(ResourceProjectItem, id, expected)
	if err != nil {
		return nil, err
	}
	var state ProjectItemState
	if err = DecodeResourceState(ResourceProjectItem, raw, &state); err != nil {
		return nil, err
	}
	state.Archived = true
	raw, err = ResourceStateJSON(ResourceProjectItem, state)
	if err != nil {
		return nil, err
	}
	return MutateResource(ResourceMutation{Family: ResourceProjectItem, Intent: "revise", ID: id, Expected: expected, State: raw, Actor: actor, Operation: key})
}
func projectItemList(args []string) (any, error) {
	f := projectFlags("project-item list")
	var project, cursor string
	limit := 50
	f.StringVar(&project, "project", "", "")
	f.StringVar(&cursor, "cursor", "", "")
	f.IntVar(&limit, "limit", 50, "")
	f.Bool("json", false, "")
	if err := projectParse(f, args); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 || project != "" && !validEventID(project) {
		return nil, usageError("list requires --limit 1..200 and optional full --project ID")
	}
	events, snapshot, err := collectIssueEvents()
	if err != nil {
		return nil, err
	}
	cat, err := buildGovernedResourceCatalog(events)
	if err != nil {
		return nil, err
	}
	if project != "" {
		p := cat.Resources[project]
		if p == nil || p.Family != ResourceProject {
			return nil, issueFailure("not_found", errors.New("project not found"))
		}
	}
	items := []*ResourceView{}
	for _, id := range cat.IDs {
		v := cat.Resources[id]
		if v.Family != ResourceProjectItem {
			continue
		}
		var state ProjectItemState
		_ = json.Unmarshal(v.Heads[0].State, &state)
		if project == "" || state.ProjectID == project {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Conflict != b.Conflict {
			return !a.Conflict
		}
		if a.Conflict {
			return a.ID < b.ID
		}
		var as, bs ProjectItemState
		_ = json.Unmarshal(a.State, &as)
		_ = json.Unmarshal(b.State, &bs)
		an, _ := new(big.Rat).SetString(as.Position)
		bn, _ := new(big.Rat).SetString(bs.Position)
		if c := an.Cmp(bn); c != 0 {
			return c < 0
		}
		return a.ID < b.ID
	})
	offset := 0
	if cursor != "" {
		if len(cursor) > 5500 {
			return nil, issueFailure("invalid_cursor", errors.New("invalid item cursor"))
		}
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, issueFailure("invalid_cursor", err)
		}
		var c resourceCursor
		if err = strictResourceJSON(raw, &c, 4096); err != nil || c.Family != ResourceProjectItem || c.Command != "position-list" || c.ID != project || c.Limit != limit || c.Offset < 0 {
			return nil, issueFailure("invalid_cursor", errors.New("cursor does not match item query"))
		}
		if c.Snapshot != snapshot {
			return nil, issueFailure("stale_cursor", errors.New("accepted resource snapshot changed"))
		}
		offset = c.Offset
	}
	if offset > len(items) {
		return nil, issueFailure("invalid_cursor", errors.New("cursor offset exceeds result"))
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := resourcePage{Items: items[offset:end], Snapshot: snapshot}
	if end < len(items) {
		raw, _ := json.Marshal(resourceCursor{ResourceProjectItem, "position-list", project, snapshot, end, limit})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}
