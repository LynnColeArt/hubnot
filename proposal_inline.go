package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type proposalInlineAnchor struct {
	Path      string `json:"path"`
	Side      string `json:"side"`
	Line      int    `json:"line"`
	ReplyTo   string `json:"replyTo,omitempty"`
	State     string `json:"state,omitempty"`
	Operation string `json:"operation"`
}

var inlineHunk = regexp.MustCompile(`^@@ -([0-9]+)(,([0-9]+))? \+([0-9]+)(,([0-9]+))? @@`)

func validateInlineLine(proposal Event, anchor proposalInlineAnchor) error {
	patch, err := projectGit(context.Background(), "", nil, 128*1024,
		"--no-replace-objects", "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-relative", "--find-renames", proposal.Base, proposal.Head, "--",
		":(top,literal)"+anchor.Path, ":(top,literal,exclude)"+anchor.Path+"/")
	if err != nil {
		return err
	}
	// A pathspec can select more than one file. Match the exact path using a
	// separate NUL-delimited name query before accepting hunk coordinates.
	names, err := projectGit(context.Background(), "", nil, 8192,
		"--no-replace-objects", "diff", "--no-ext-diff", "--no-relative", "--name-only", "-z", "--find-renames", proposal.Base, proposal.Head, "--", ":(top,literal)"+anchor.Path, ":(top,literal,exclude)"+anchor.Path+"/")
	if err != nil || string(names) != anchor.Path+"\x00" {
		return fmt.Errorf("file is unavailable in this revision's diff")
	}
	for _, line := range strings.Split(string(patch), "\n") {
		h := inlineHunk.FindStringSubmatch(line)
		if h == nil {
			continue
		}
		start, count := h[4], h[6]
		if anchor.Side == "LEFT" {
			start, count = h[1], h[3]
		}
		first, _ := strconv.Atoi(start)
		size := 1
		if count != "" {
			size, _ = strconv.Atoi(count)
		}
		if anchor.Line >= first && anchor.Line-first < size {
			return nil
		}
	}
	return fmt.Errorf("line is outside the selected revision's patch")
}

func validateInlineContent(event Event) error {
	a := event.Inline
	if a == nil || !validEventID(event.Subject) || !utf8.ValidString(a.Path) ||
		len(a.Path) == 0 || len(a.Path) > 4096 || strings.ContainsRune(a.Path, 0) ||
		(a.Side != "LEFT" && a.Side != "RIGHT") || a.Line < 1 ||
		(a.ReplyTo != "" && !validEventID(a.ReplyTo)) || len(event.Body) > 16384 ||
		!validEventID(a.Operation) {
		return fmt.Errorf("invalid inline review comment")
	}
	if a.State == "" && strings.TrimSpace(event.Body) == "" ||
		a.State != "" && (a.State != "resolved" && a.State != "open" || a.ReplyTo == "" || event.Body != "") {
		return fmt.Errorf("invalid inline thread state or comment body")
	}
	return nil
}

func cmdProposalInline(action string, args []string) error {
	if err := runProposalInline(action, args); err != nil {
		return consumerOutput(nil, err)
	}
	return nil
}

func runProposalInline(action string, args []string) error {
	flags := quietFlags("proposal " + action)
	path := flags.String("path", "", "")
	side := flags.String("side", "", "")
	line := flags.Int("line", 0, "")
	state := flags.String("state", "", "")
	reply := flags.String("reply-to", "", "")
	body := flags.String("body", "", "")
	actor := flags.String("actor", "", "")
	operation := flags.String("operation", "", "")
	flags.Bool("json", false, "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || !validEventID(flags.Arg(0)) {
		return fmt.Errorf("provide an exact proposal revision ID")
	}
	revision := flags.Arg(0)
	identity, signerErr := loadIdentity()
	var event Event
	if action == "inline-post" {
		if signerErr != nil {
			return signerErr
		}
		if *actor != identity.Actor || strings.TrimSpace(*operation) == "" || len(*operation) > 128 {
			return fmt.Errorf("active signer or operation ID is invalid")
		}
		var err error
		event, err = nextEvent(identity, "proposal.inline")
		if err != nil {
			return err
		}
	}
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "proposal inline review"}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("proposal inline review"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	events, err = consumerSelectedEvents(events)
	if err != nil {
		return err
	}
	var proposal *StoredEvent
	var comments []StoredEvent
	for i := range events {
		event := &events[i]
		if event.ID == revision && isProposalKind(event.Event.Kind) {
			proposal = event
		}
		if event.Event.Kind == "proposal.inline" && event.Event.Subject == revision {
			comments = append(comments, *event)
		}
	}
	if proposal == nil {
		return fmt.Errorf("proposal revision is unavailable")
	}
	rows := make([]any, 0, len(comments))
	states := map[string]StoredEvent{}
	roots := map[string]StoredEvent{}
	for _, comment := range comments {
		if comment.Event.Inline.ReplyTo == "" {
			roots[comment.ID] = comment
		}
	}
	for _, comment := range comments {
		inline := comment.Event.Inline
		if inline.State == "" || !matchingInlineThread(comment.Event, roots[inline.ReplyTo].Event) {
			continue
		}
		prior := states[inline.ReplyTo]
		if comment.Event.Sequence > prior.Event.Sequence ||
			(comment.Event.Sequence == prior.Event.Sequence && comment.ID > prior.ID) {
			states[inline.ReplyTo] = comment
		}
	}
	threadStates := map[string]string{}
	for id, root := range roots {
		threadStates[id] = "open"
		if change, ok := states[root.ID]; ok {
			threadStates[id] = change.Event.Inline.State
		}
	}
	for _, comment := range comments {
		if comment.Event.Inline.State == "" {
			rows = append(rows, consumerStoredEvent(comment))
		}
	}
	if action == "inline-list" {
		signer := ""
		if signerErr == nil {
			signer = identity.Actor
		}
		return consumerOutput(map[string]any{"comments": rows, "actor": signer, "threadStates": threadStates}, nil)
	}
	anchor := proposalInlineAnchor{Path: *path, Side: *side, Line: *line, ReplyTo: *reply, State: *state}
	if *reply != "" {
		found := false
		for _, comment := range comments {
			if comment.ID == *reply && comment.Event.Inline.ReplyTo == "" {
				if *state != "" && comment.Event.Actor != identity.Actor {
					return fmt.Errorf("only the thread author can change its status")
				}
				parent := comment.Event.Inline
				if parent.Path != anchor.Path || parent.Side != anchor.Side || parent.Line != anchor.Line {
					return fmt.Errorf("reply anchor differs from thread")
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("thread is unavailable on this revision")
		}
	}
	request := eventID([]byte(*operation))
	anchor.Operation = request
	for _, comment := range comments {
		if comment.Event.Actor == identity.Actor && comment.Event.Inline.Operation == request {
			if *comment.Event.Inline != anchor || comment.Event.Body != *body {
				return fmt.Errorf("operation ID already used for different content")
			}
			return consumerOutput(consumerMutationReceipt(&comment), nil)
		}
	}
	if *state != "" {
		if threadStates[*reply] == *state {
			return fmt.Errorf("thread already has this status")
		}
	} else if err := validateInlineLine(proposal.Event, anchor); err != nil {
		return err
	}
	event.Subject, event.Body, event.Inline = revision, *body, &anchor
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	return consumerOutput(consumerMutationReceipt(stored), nil)
}

func matchingInlineThread(reply, root Event) bool {
	return root.Kind == "proposal.inline" && root.Subject == reply.Subject &&
		root.Inline != nil && reply.Inline != nil && root.Inline.ReplyTo == "" &&
		(reply.Inline.State == "" || root.Actor == reply.Actor) &&
		root.Inline.Path == reply.Inline.Path && root.Inline.Side == reply.Inline.Side && root.Inline.Line == reply.Inline.Line
}
