package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const ghCompatibilityHelp = `Usage:
  hn gh issue create --title/-t TEXT [--body/-b TEXT] [--label/-l NAMES] [--assignee/-a NAMES]
  hn gh issue list [--state/-s open|closed|all] [--label/-l NAMES] [--assignee/-a NAMES] [--limit/-L N] [--json FIELDS]
  hn gh issue view ID [--comments] [--json FIELDS]
  hn gh issue edit ID [--title/-t TEXT] [--body/-b TEXT] [--add-label NAMES] [--remove-label NAMES] [--add-assignee NAMES] [--remove-assignee NAMES]
  hn gh issue comment ID --body/-b TEXT
  hn gh issue close|reopen ID

This parallel command interface operates on local native Hubnot issues; it does
not invoke gh, contact GitHub, authenticate to a forge, or synchronize tickets.
Mutations require full sha256:<64-hex> issue IDs. Views accept unique native ID
prefixes. GitHub issue numbers and --repo are unsupported. Use explicit
hn github import/export commands for optional GitHub transport.

Repeated/comma-separated label and assignee flags form sets. Flags may occur
before or after ID. The list limit defaults to 30 and is at most 100000; it
applies after filtering. Conflicted issues remain visible with unknown state.
JSON fields: id,title,body,state,labels,assignees,author,url,conflict.
View additionally supports comments. JSON output is a selected-field object
(view) or array (list), not the native issue envelope. State is OPEN/CLOSED or
null for conflicts; author/assignees use login, labels use name. Native author
login is a full actor fingerprint. url is null without recorded source URL.
Comments include id, body, author.login and createdAt. Unsupported flags,
fields and command families fail explicitly. Native hn issue remains unchanged.
`

type ghIssueOptions struct {
	command, id, title, body, state                                                   string
	labels, assignees, addLabels, removeLabels, addAssignees, removeAssignees, fields []string
	titleSet, bodySet, comments, json, help                                           bool
	limit                                                                             int
}

func cmdGH(args []string) error {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Print(ghCompatibilityHelp)
		return nil
	}
	if args[0] != "issue" {
		return errors.New("unsupported gh command family; run 'hn gh --help'")
	}
	args = args[1:]
	if len(args) == 0 {
		fmt.Print(ghCompatibilityHelp)
		return nil
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Print(ghCompatibilityHelp)
		return nil
	}
	o, err := parseGHIssueOptions(args)
	if err != nil {
		return err
	}
	if o.help {
		fmt.Print(ghCompatibilityHelp)
		return nil
	}
	switch o.command {
	case "list":
		return ghIssueList(o)
	case "view":
		return ghIssueView(o)
	default:
		return ghIssueMutate(o)
	}
}

func parseGHIssueOptions(args []string) (ghIssueOptions, error) {
	o := ghIssueOptions{command: args[0], state: "open", limit: 30}
	allowed := map[string]bool{"help": true, "h": true}
	allow := func(names ...string) {
		for _, name := range names {
			allowed[name] = true
		}
	}
	switch o.command {
	case "create":
		allow("title", "t", "body", "b", "label", "l", "assignee", "a")
	case "list":
		allow("state", "s", "label", "l", "assignee", "a", "limit", "L", "json")
	case "view":
		allow("comments", "json")
	case "edit":
		allow("title", "t", "body", "b", "add-label", "remove-label", "add-assignee", "remove-assignee")
	case "comment":
		allow("body", "b")
	case "close", "reopen":
	default:
		return o, errors.New("unsupported gh issue command; run 'hn gh --help'")
	}
	var positional []string
	end := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !end && arg == "--" {
			end = true
			continue
		}
		if end || !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		name, value, assigned := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		if !allowed[name] {
			return o, fmt.Errorf("unsupported flag %q for gh issue %s", arg, o.command)
		}
		if name == "comments" || name == "help" || name == "h" {
			if !assigned {
				value = "true"
			}
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return o, errors.New("--comments requires a boolean")
			}
			if name == "comments" {
				o.comments = parsed
			} else {
				o.help = parsed
			}
			continue
		}
		if !assigned {
			i++
			if i >= len(args) {
				return o, fmt.Errorf("flag --%s requires a value", name)
			}
			value = args[i]
		}
		switch name {
		case "title", "t":
			o.title = value
			o.titleSet = true
		case "body", "b":
			o.body = value
			o.bodySet = true
		case "state", "s":
			o.state = value
		case "limit", "L":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 100000 {
				return o, errors.New("--limit must be 1..100000")
			}
			o.limit = parsed
		case "json":
			o.json = true
			o.fields = strings.Split(value, ",")
		default:
			values := strings.Split(value, ",")
			for _, v := range values {
				if !issueIdentifier(v, 256) {
					return o, fmt.Errorf("invalid value for --%s", name)
				}
			}
			switch name {
			case "label", "l":
				o.labels = append(o.labels, values...)
			case "assignee", "a":
				o.assignees = append(o.assignees, values...)
			case "add-label":
				o.addLabels = append(o.addLabels, values...)
			case "remove-label":
				o.removeLabels = append(o.removeLabels, values...)
			case "add-assignee":
				o.addAssignees = append(o.addAssignees, values...)
			case "remove-assignee":
				o.removeAssignees = append(o.removeAssignees, values...)
			}
		}
	}
	if o.help {
		return o, nil
	}
	if o.command == "create" || o.command == "list" {
		if len(positional) != 0 {
			return o, errors.New("unexpected positional argument")
		}
	} else {
		if len(positional) != 1 {
			return o, errors.New("exactly one native issue ID is required")
		}
		o.id = positional[0]
		if o.command != "view" && !validEventID(o.id) {
			return o, errors.New("mutations require a full sha256:<64-hex> native issue ID")
		}
	}
	if o.state != "open" && o.state != "closed" && o.state != "all" {
		return o, errors.New("--state must be open, closed or all")
	}
	if o.command == "create" && (!o.titleSet || strings.TrimSpace(o.title) == "") {
		return o, errors.New("--title is required and cannot be empty")
	}
	if o.command == "comment" && (!o.bodySet || strings.TrimSpace(o.body) == "") {
		return o, errors.New("--body is required and cannot be empty")
	}
	if o.command == "edit" && !o.titleSet && !o.bodySet && len(o.addLabels)+len(o.removeLabels)+len(o.addAssignees)+len(o.removeAssignees) == 0 {
		return o, errors.New("edit requires at least one supported change")
	}
	if o.json {
		supported := map[string]bool{"id": true, "title": true, "body": true, "state": true, "labels": true, "assignees": true, "author": true, "url": true, "conflict": true, "comments": o.command == "view"}
		seen := map[string]bool{}
		for _, f := range o.fields {
			if !supported[f] || seen[f] {
				return o, fmt.Errorf("unsupported or duplicate JSON field %q", f)
			}
			seen[f] = true
		}
	}
	return o, nil
}

func ghIssueList(o ghIssueOptions) error {
	result := []map[string]any{}
	cursor := ""
	for {
		raw, _, next, err := executeIssueQuery(issueOptions{command: "list", details: true, limit: 200, cursor: cursor})
		if err != nil {
			return err
		}
		page := raw.(issuePage)
		for _, item := range page.Items.([]issueDetail) {
			if item.State != nil && (o.state != "all" && o.state != item.State.Status || !ghContainsAll(item.State.Labels, o.labels) || !ghContainsAll(item.State.Assignees, o.assignees)) {
				continue
			}
			// Unknown conflict state is intentionally visible rather than classified as open/closed.
			result = append(result, ghIssueObject(item.ID, item.Creator, item.State, item.Conflict))
			if len(result) == o.limit {
				break
			}
		}
		if len(result) == o.limit || next == "" {
			break
		}
		cursor = next
	}
	if o.json {
		selected := make([]map[string]any, 0, len(result))
		for _, item := range result {
			selected = append(selected, ghSelectFields(item, o.fields))
		}
		return json.NewEncoder(os.Stdout).Encode(selected)
	}
	for _, item := range result {
		fmt.Printf("%s\t%s\t%s\n", item["id"], ghHumanState(item), oneLine(fmt.Sprint(item["title"])))
	}
	return nil
}

func ghReadIssue(id string) (issueShow, error) {
	raw, _, _, err := executeIssueQuery(issueOptions{command: "show", id: id, limit: 200})
	if err != nil {
		return issueShow{}, err
	}
	return raw.(issueShow), nil
}
func ghIssueView(o ghIssueOptions) error {
	view, err := ghReadIssue(o.id)
	if err != nil {
		return err
	}
	item := ghIssueObject(view.ID, view.Creator, view.State, view.Conflict)
	if o.comments || ghHas(o.fields, "comments") {
		comments, err := ghIssueComments(view.ID)
		if err != nil {
			return err
		}
		item["comments"] = comments
	}
	if o.json {
		return json.NewEncoder(os.Stdout).Encode(ghSelectFields(item, o.fields))
	}
	fmt.Printf("%s\n%s\nState: %s\nAuthor: %s\n", oneLine(fmt.Sprint(item["title"])), view.ID, ghHumanState(item), view.Creator)
	if view.State != nil {
		fmt.Println(safeText(view.State.Body))
	} else {
		fmt.Println("Concurrent issue heads: inspect and resolve with hn issue.")
	}
	if comments, ok := item["comments"].([]map[string]any); ok {
		for _, comment := range comments {
			fmt.Printf("\n%s\n%s\n", comment["id"], safeText(comment["body"].(string)))
		}
	}
	return nil
}
func ghIssueComments(id string) ([]map[string]any, error) {
	result := []map[string]any{}
	cursor := ""
	for {
		raw, _, next, err := executeIssueQuery(issueOptions{command: "history", id: id, kind: "comments", limit: 200, cursor: cursor})
		if err != nil {
			return nil, err
		}
		for _, c := range raw.(issuePage).Items.([]issueHistoryItem) {
			result = append(result, map[string]any{"id": c.ID, "body": c.Body, "author": map[string]string{"login": c.Actor}, "createdAt": c.Timestamp})
		}
		if next == "" {
			return result, nil
		}
		cursor = next
	}
}

func ghIssueMutate(o ghIssueOptions) error {
	identity, err := inspectIdentity()
	if err != nil {
		return issueFailure("repository_error", errors.New("active identity unavailable; run 'hn init' or inspect identity configuration"))
	}
	native := issueOptions{actor: identity.Actor, id: o.id}
	if o.command == "create" {
		native.command = "open"
		state, err := CanonicalIssueState(IssueState{Title: o.title, Body: o.body, Status: "open", Labels: ghSetChange(nil, nil, o.labels), Assignees: ghSetChange(nil, nil, o.assignees)})
		if err != nil {
			return err
		}
		native.state = &state
	} else {
		view, err := ghReadIssue(o.id)
		if err != nil {
			return err
		}
		if view.Conflict || view.State == nil || len(view.HeadIDs) != 1 {
			return issueFailure("issue_conflict", errors.New("issue has concurrent heads; use hn issue heads/resolve"))
		}
		switch o.command {
		case "comment":
			native.command = "comment"
			native.body = o.body
		case "close", "reopen":
			native.expected = view.HeadIDs
			native.command = o.command
		case "edit":
			native.expected = view.HeadIDs
			native.command = "revise"
			state := *view.State
			if o.titleSet {
				state.Title = o.title
			}
			if o.bodySet {
				state.Body = o.body
			}
			state.Labels = ghSetChange(state.Labels, o.removeLabels, o.addLabels)
			state.Assignees = ghSetChange(state.Assignees, o.removeAssignees, o.addAssignees)
			canonical, err := CanonicalIssueState(state)
			if err != nil {
				return err
			}
			native.state = &canonical
		}
	}
	raw, err := executeIssueMutation(native)
	if err != nil {
		return err
	}
	result := raw.(*issueMutationResult)
	if result.Conflict {
		return issueFailure("issue_conflict", errors.New("write recorded alongside concurrent issue heads; inspect hn issue heads"))
	}
	fmt.Println(result.IssueID)
	return nil
}

func ghIssueObject(id, actor string, state *IssueState, conflict bool) map[string]any {
	result := map[string]any{"id": id, "author": map[string]string{"login": actor}, "title": nil, "body": nil, "state": nil, "labels": nil, "assignees": nil, "url": nil, "conflict": conflict}
	if state == nil {
		return result
	}
	result["title"] = state.Title
	result["body"] = state.Body
	result["state"] = strings.ToUpper(state.Status)
	labels := []map[string]string{}
	for _, label := range state.Labels {
		labels = append(labels, map[string]string{"name": label})
	}
	result["labels"] = labels
	assignees := []map[string]string{}
	for _, actor := range state.Assignees {
		assignees = append(assignees, map[string]string{"login": actor})
	}
	result["assignees"] = assignees
	// Imported URLs are descriptive source provenance, never used for transport.
	var source struct {
		URL     string `json:"url"`
		HTMLURL string `json:"html_url"`
	}
	if json.Unmarshal([]byte(state.Metadata["forge/source"]), &source) == nil {
		if source.URL != "" {
			result["url"] = source.URL
		} else if source.HTMLURL != "" {
			result["url"] = source.HTMLURL
		}
	}
	return result
}
func ghSelectFields(item map[string]any, fields []string) map[string]any {
	result := map[string]any{}
	for _, f := range fields {
		result[f] = item[f]
	}
	return result
}
func ghHas(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func ghContainsAll(have, want []string) bool {
	for _, v := range want {
		if !ghHas(have, v) {
			return false
		}
	}
	return true
}
func ghSetChange(values, remove, add []string) []string {
	result := []string{}
	for _, v := range values {
		if !ghHas(remove, v) && !ghHas(result, v) {
			result = append(result, v)
		}
	}
	for _, v := range add {
		if !ghHas(result, v) {
			result = append(result, v)
		}
	}
	return result
}
func ghHumanState(item map[string]any) string {
	if item["conflict"] == true {
		return "CONFLICT"
	}
	return fmt.Sprint(item["state"])
}
