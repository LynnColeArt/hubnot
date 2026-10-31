package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() {
	NativeResourceCommands[ResourceDiscussion] = cmdDiscussion
	NativeResourceCommands[ResourceDiscussionReply] = cmdDiscussionReply
}

// Full-state commands remain available for importers and explicit resolutions.
func cmdDiscussion(args []string) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println("hn discussion open --title TITLE --category CATEGORY [--body BODY]\nhn discussion revise ID --expect REV [--title TITLE] [--body BODY] [--category CATEGORY]\nhn discussion close|reopen ID --expect REV\nhn discussion category ID --expect REV --category CATEGORY\nhn discussion reply add THREAD --body BODY [--reply-to REPLY_ROOT]\nhn discussion reply edit REPLY --expect REV --body BODY\nhn discussion answer THREAD --expect REV (--reply EXACT_REPLY_REV | --clear)\nhn discussion replies THREAD [--limit 1..200] [--cursor TOKEN]\nMutations accept --actor ACTOR, --operation KEY and --json. Categories/status are descriptive; they grant no moderation rights. Show includes pinned answer content and reply count; replies are paged separately. Full-state open/revise/resolve --input FILE|- and list/show/heads/history/operation remain available.")
		return nil
	}
	if args[0] == "reply" {
		return cmdDiscussionReply(args[1:])
	}
	if args[0] == "show" || args[0] == "replies" {
		return discussionRead(args)
	}
	if args[0] == "open" || args[0] == "revise" {
		if discussionHasInput(args[1:]) {
			return CmdResource(ResourceDiscussion, args)
		}
		return discussionEdit(ResourceDiscussion, args)
	}
	switch args[0] {
	case "close", "reopen", "category", "answer":
		return discussionEdit(ResourceDiscussion, args)
	default:
		return CmdResource(ResourceDiscussion, args)
	}
}
func cmdDiscussionReply(args []string) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println("hn discussion reply add THREAD --body BODY [--reply-to REPLY_ROOT]\nhn discussion reply edit REPLY --expect REV --body BODY\nFull-state discussion-reply open/revise/resolve --input FILE|- and list/show/heads/history/operation are also available. Mutations accept --actor, --operation and --json.")
		return nil
	}
	switch args[0] {
	case "add", "edit":
		return discussionEdit(ResourceDiscussionReply, args)
	default:
		return CmdResource(ResourceDiscussionReply, args)
	}
}
func discussionHasInput(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--input" || strings.HasPrefix(a, "--input=") || a == "-input" || strings.HasPrefix(a, "-input=") {
			return true
		}
		// String flag values can themselves resemble flags (for example a body of
		// "--input"). Match the parser instead of interpreting those as selectors.
		name := strings.TrimLeft(a, "-")
		switch name {
		case "title", "body", "category", "actor", "operation", "expect":
			i++
		}
	}
	return false
}
func discussionResult(data any, err error, machine bool) error {
	if machine {
		envelope := map[string]any{"ok": err == nil, "data": data}
		if err != nil {
			code := "resource_error"
			var typed *issueCommandError
			if errors.As(err, &typed) {
				code = typed.Code
			}
			envelope["error"] = map[string]string{"code": code, "message": err.Error()}
		}
		if encodeErr := json.NewEncoder(os.Stdout).Encode(envelope); encodeErr != nil {
			return encodeErr
		}
	} else if err == nil {
		raw, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(raw))
	}
	return err
}

// The expected immutable revision supplies the base, never the current moving
// head. This preserves metadata and makes an old operation retry byte-identical.
func discussionBase(family, id string, expected []string) (json.RawMessage, error) {
	if !validEventID(id) || len(expected) != 1 || !validEventID(expected[0]) {
		return nil, issueFailure("invalid_input", errors.New("edit requires full resource ID and one full --expect revision"))
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
	for _, stored := range view.History {
		if stored.ID == expected[0] {
			return json.RawMessage(stored.Event.Body), nil
		}
	}
	return nil, issueFailure("stale_revision", errors.New("expected revision is not in this resource history"))
}
func discussionEdit(family string, args []string) error {
	command := args[0]
	flags := flag.NewFlagSet(family+" "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var title, body, category, actor, key, replyTo, reply string
	var machine, clear bool
	expected := issueExpected{}
	flags.StringVar(&title, "title", "", "")
	flags.StringVar(&body, "body", "", "")
	flags.StringVar(&category, "category", "", "")
	flags.StringVar(&actor, "actor", "", "")
	flags.StringVar(&key, "operation", "", "")
	flags.StringVar(&replyTo, "reply-to", "", "")
	flags.StringVar(&reply, "reply", "", "")
	flags.BoolVar(&machine, "json", false, "")
	flags.BoolVar(&clear, "clear", false, "")
	flags.Var(&expected, "expect", "")
	rest := args[1:]
	id := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		id = rest[0]
		rest = rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("unexpected positional arguments")
	}
	supplied := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { supplied[f.Name] = true })
	allowed := map[string]bool{"actor": true, "operation": true, "json": true}
	switch command {
	case "open":
		allowed["title"] = true
		allowed["body"] = true
		allowed["category"] = true
	case "revise":
		allowed["title"] = true
		allowed["body"] = true
		allowed["category"] = true
		allowed["expect"] = true
	case "category":
		allowed["category"] = true
		allowed["expect"] = true
	case "close", "reopen":
		allowed["expect"] = true
	case "answer":
		allowed["expect"] = true
		allowed["reply"] = true
		allowed["clear"] = true
	case "add":
		allowed["body"] = true
		allowed["reply-to"] = true
	case "edit":
		allowed["body"] = true
		allowed["expect"] = true
	default:
		return usageError("unsupported discussion mutation")
	}
	for name := range supplied {
		if !allowed[name] {
			return usageError("flag --" + name + " does not apply to " + command)
		}
	}
	var result any
	perform := func() error {
		var state any
		intent := "revise"
		target := id
		if family == ResourceDiscussion {
			s := DiscussionState{}
			if command == "open" {
				if id != "" || !supplied["title"] || !supplied["category"] {
					return usageError("open requires --title and --category, without ID")
				}
				s = DiscussionState{Schema: "hn.discussion/1", Title: title, Body: body, Category: category, Status: "open"}
				intent = "open"
			} else {
				raw, err := discussionBase(family, id, expected)
				if err != nil {
					return err
				}
				if err = json.Unmarshal(raw, &s); err != nil {
					return err
				}
			}
			switch command {
			case "revise":
				if !supplied["title"] && !supplied["body"] && !supplied["category"] {
					return usageError("revise requires a title, body or category change")
				}
				if supplied["title"] {
					s.Title = title
				}
				if supplied["body"] {
					s.Body = body
				}
				if supplied["category"] {
					s.Category = category
				}
			case "close":
				s.Status = "closed"
			case "reopen":
				s.Status = "open"
			case "category":
				if !supplied["category"] {
					return usageError("category requires --category")
				}
				s.Category = category
			case "answer":
				if (supplied["reply"] && supplied["clear"]) || (!supplied["reply"] && !clear) || supplied["clear"] && !clear {
					return usageError("answer requires exactly one of --reply EXACT_REV or --clear")
				}
				if supplied["reply"] && !validEventID(reply) {
					return usageError("answer requires a full exact reply revision")
				}
				s.AcceptedReplyRevision = reply
			}
			state = s
		} else {
			if !supplied["body"] {
				return usageError("reply requires --body")
			}
			s := DiscussionReplyState{}
			if command == "add" {
				if !validEventID(id) {
					return usageError("reply add requires full thread ID")
				}
				s = DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: id, ReplyTo: replyTo, Body: body}
				target = ""
				intent = "open"
			} else {
				raw, err := discussionBase(family, id, expected)
				if err != nil {
					return err
				}
				if err = json.Unmarshal(raw, &s); err != nil {
					return err
				}
				s.Body = body
			}
			state = s
		}
		raw, err := ResourceStateJSON(family, state)
		if err != nil {
			return issueFailure("invalid_input", err)
		}
		result, err = MutateResource(ResourceMutation{Family: family, Intent: intent, ID: target, Expected: expected, State: raw, Actor: actor, Operation: key})
		return err
	}
	err := perform()
	return discussionResult(result, err, machine)
}

type discussionView struct {
	*ResourceView
	ReplyCount     int           `json:"reply_count"`
	AcceptedAnswer *ResourceHead `json:"accepted_answer,omitempty"`
}

func discussionRead(args []string) error {
	command := args[0]
	flags := flag.NewFlagSet("discussion "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var machine bool
	var cursor string
	limit := 50
	flags.BoolVar(&machine, "json", false, "")
	flags.IntVar(&limit, "limit", 50, "")
	flags.StringVar(&cursor, "cursor", "", "")
	rest := args[1:]
	id := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		id = rest[0]
		rest = rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("unexpected positional arguments")
	}
	var result any
	perform := func() error {
		if limit < 1 || limit > 200 {
			return issueFailure("invalid_input", errors.New("page limit must be 1..200"))
		}
		if command == "show" {
			used := false
			flags.Visit(func(f *flag.Flag) {
				if f.Name == "limit" || f.Name == "cursor" {
					used = true
				}
			})
			if used {
				return usageError("show does not accept pagination; use replies")
			}
		}
		events, snapshot, err := collectIssueEvents()
		if err != nil {
			return err
		}
		cat, err := buildGovernedResourceCatalog(events)
		if err != nil {
			return err
		}
		var thread *ResourceView
		prefix := strings.TrimPrefix(id, "sha256:")
		if len(prefix) < 8 {
			return issueFailure("not_found", errors.New("discussion not found"))
		}
		for _, root := range cat.IDs {
			view := cat.Resources[root]
			if view.Family == ResourceDiscussion && strings.HasPrefix(strings.TrimPrefix(root, "sha256:"), prefix) {
				if thread != nil {
					return issueFailure("ambiguous_id", errors.New("discussion prefix is ambiguous"))
				}
				thread = view
			}
		}
		if thread == nil {
			return issueFailure("not_found", errors.New("discussion not found"))
		}
		id = thread.ID
		replies := []*ResourceView{}
		for _, root := range cat.IDs {
			view := cat.Resources[root]
			if view.Family == ResourceDiscussionReply {
				var s DiscussionReplyState
				_ = json.Unmarshal(view.Heads[0].State, &s)
				if s.DiscussionID == id {
					replies = append(replies, view)
				}
			}
		}
		if command == "show" {
			view := discussionView{ResourceView: thread, ReplyCount: len(replies)}
			if !thread.Conflict {
				var s DiscussionState
				_ = json.Unmarshal(thread.State, &s)
				if s.AcceptedReplyRevision != "" {
					for _, e := range events {
						if e.ID == s.AcceptedReplyRevision {
							view.AcceptedAnswer = &ResourceHead{ID: e.ID, Actor: e.Event.Actor, State: json.RawMessage(e.Event.Body)}
							break
						}
					}
				}
			}
			result = view
			return nil
		}
		offset := 0
		if cursor != "" {
			raw, err := base64.RawURLEncoding.DecodeString(cursor)
			if err != nil || len(raw) > 4096 {
				return issueFailure("invalid_cursor", errors.New("invalid reply cursor"))
			}
			var c resourceCursor
			if err := strictResourceJSON(raw, &c, 4096); err != nil || c.Family != ResourceDiscussion || c.Command != command || c.ID != id || c.Limit != limit || c.Offset < 0 {
				return issueFailure("invalid_cursor", errors.New("cursor does not match reply query"))
			}
			if c.Snapshot != snapshot {
				return issueFailure("stale_cursor", errors.New("accepted resource snapshot changed"))
			}
			offset = c.Offset
		}
		if offset > len(replies) {
			return issueFailure("invalid_cursor", errors.New("cursor offset exceeds replies"))
		}
		end := offset + limit
		if end > len(replies) {
			end = len(replies)
		}
		page := resourcePage{Items: replies[offset:end], Snapshot: snapshot}
		if end < len(replies) {
			raw, _ := json.Marshal(resourceCursor{ResourceDiscussion, command, id, snapshot, end, limit})
			page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
		}
		result = page
		return nil
	}
	err := perform()
	return discussionResult(result, err, machine)
}
