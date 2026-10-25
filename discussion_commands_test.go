package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func discussionCLI(t *testing.T, args ...string) (json.RawMessage, error) {
	t.Helper()
	out, err := captureTestOutput(t, func() error { return run(append(args, "--json")) })
	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if parseErr := json.Unmarshal([]byte(out), &envelope); parseErr != nil {
		t.Fatalf("JSON %q: %v (command: %v)", out, parseErr, err)
	}
	if envelope.OK != (err == nil) {
		t.Fatalf("inconsistent result %s: %v", out, err)
	}
	return envelope.Data, err
}
func discussionWrite(t *testing.T, args ...string) ResourceMutationResult {
	t.Helper()
	data, err := discussionCLI(t, args...)
	if err != nil {
		t.Fatal(err)
	}
	var result ResourceMutationResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestDiscussionNativeLifecyclePinnedAnswerAndReads(t *testing.T) {
	issueCommandTestRepo(t)
	thread := discussionWrite(t, "discussion", "open", "--title", "Question", "--body", "Details", "--category", "help", "--operation", "thread")
	reply := discussionWrite(t, "discussion", "reply", "add", thread.ResourceID, "--body", "Original answer", "--operation", "reply")
	nested := discussionWrite(t, "discussion", "reply", "add", thread.ResourceID, "--reply-to", reply.ResourceID, "--body", "Nested")
	accepted := discussionWrite(t, "discussion", "answer", thread.ResourceID, "--expect", thread.EventID, "--reply", reply.EventID, "--operation", "answer")
	edited := discussionWrite(t, "discussion", "reply", "edit", reply.ResourceID, "--expect", reply.EventID, "--body", "New answer", "--operation", "edit")
	again := discussionWrite(t, "discussion", "reply", "edit", reply.ResourceID, "--expect", reply.EventID, "--body", "New answer", "--operation", "edit")
	if again.EventID != edited.EventID || !again.Replayed {
		t.Fatal("edit replay moved")
	}
	data, err := discussionCLI(t, "discussion", "show", thread.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	var shown struct {
		ID             string       `json:"id"`
		ReplyCount     int          `json:"reply_count"`
		AcceptedAnswer ResourceHead `json:"accepted_answer"`
	}
	if err := json.Unmarshal(data, &shown); err != nil {
		t.Fatal(err)
	}
	var pinned DiscussionReplyState
	if err := json.Unmarshal(shown.AcceptedAnswer.State, &pinned); err != nil {
		t.Fatal(err)
	}
	if shown.ID != thread.ResourceID || shown.ReplyCount != 2 || shown.AcceptedAnswer.ID != reply.EventID || shown.AcceptedAnswer.Actor == "" || pinned.Body != "Original answer" {
		t.Fatalf("answer lost exact content: %s", data)
	}
	data, err = discussionCLI(t, "discussion", "replies", thread.ResourceID, "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Items      []ResourceView `json:"items"`
		NextCursor string         `json:"next_cursor"`
	}
	json.Unmarshal(data, &page)
	if len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("no bounded reply page: %s", data)
	}
	if _, err := discussionCLI(t, "discussion", "replies", thread.ResourceID, "--limit", "1", "--cursor", page.NextCursor); err != nil {
		t.Fatal(err)
	}
	closed := discussionWrite(t, "discussion", "close", thread.ResourceID, "--expect", accepted.EventID, "--operation", "close")
	reopened := discussionWrite(t, "discussion", "reopen", thread.ResourceID, "--expect", closed.EventID)
	replay := discussionWrite(t, "discussion", "close", thread.ResourceID, "--expect", accepted.EventID, "--operation", "close")
	if !replay.Replayed || replay.EventID != closed.EventID {
		t.Fatal("lifecycle replay changed")
	}
	changed := discussionWrite(t, "discussion", "category", thread.ResourceID, "--expect", reopened.EventID, "--category", "ideas")
	cleared := discussionWrite(t, "discussion", "answer", thread.ResourceID, "--expect", changed.EventID, "--clear")
	if cleared.EventID == changed.EventID {
		t.Fatal("clear did not revise")
	}
	if nested.ResourceID == reply.ResourceID {
		t.Fatal("nested reply is not independent")
	}
	identity, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := discussionCLI(t, "discussion-reply", "operation", "--actor", identity.Actor, "--operation", "edit"); err != nil {
		t.Fatal(err)
	}
	// Reads retain all signed attribution without access to the active private key.
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(paths.identities); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths.active); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Remove(paths.legacy); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := loadIdentity(); err == nil {
		t.Fatal("test retained an identity")
	}
	for _, args := range [][]string{{"discussion", "list"}, {"discussion", "show", thread.ResourceID}, {"discussion", "replies", thread.ResourceID}, {"discussion-reply", "show", reply.ResourceID}, {"discussion-reply", "history", reply.ResourceID}} {
		if _, err := discussionCLI(t, args...); err != nil {
			t.Fatal(err)
		}
	}
}
func TestDiscussionGuardsMetadataAndBounds(t *testing.T) {
	root := issueCommandTestRepo(t)
	identity, _ := loadIdentity()
	state := DiscussionState{Schema: "hn.discussion/1", Title: "Topic", Body: "Old", Category: "help", Status: "open", Metadata: map[string]string{"forge/source": "external claim", "native/context": "keep"}}
	path := filepath.Join(root, "discussion.json")
	raw, _ := ResourceStateJSON(ResourceDiscussion, state)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	thread := discussionWrite(t, "discussion", "open", "--input", path)
	edit := discussionWrite(t, "discussion", "revise", thread.ResourceID, "--expect", thread.EventID, "--body", "New", "--actor", identity.Actor, "--operation", "revise")
	data, err := discussionCLI(t, "discussion", "show", thread.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	var view ResourceView
	json.Unmarshal(data, &view)
	var current DiscussionState
	json.Unmarshal(view.State, &current)
	if current.Metadata["forge/source"] != "external claim" || current.Metadata["native/context"] != "keep" || current.Title != "Topic" {
		t.Fatalf("lost state: %s", data)
	}
	for _, args := range [][]string{
		{"discussion", "revise", thread.ResourceID, "--expect", thread.EventID, "--body", "Stale"},
		{"discussion", "revise", thread.ResourceID, "--expect", thread.EventID, "--body", "Other", "--operation", "revise"},
		{"discussion", "close", thread.ResourceID, "--expect", edit.EventID, "--actor", testIdentity(t, "Other").Actor},
		{"discussion", "close", thread.ResourceID},
		{"discussion", "open", "--title", "Bad", "--category", "help", "--lock"},
		{"discussion", "open", "--title", "Bad", "--category", "help", "--input", path},
	} {
		if _, err := captureTestOutput(t, func() error { return run(args) }); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	other := discussionWrite(t, "discussion", "open", "--title", "Other", "--category", "help")
	parent := ""
	for depth := 1; depth <= 8; depth++ {
		args := []string{"discussion", "reply", "add", thread.ResourceID, "--body", fmt.Sprint(depth)}
		if parent != "" {
			args = append(args, "--reply-to", parent)
		}
		r := discussionWrite(t, args...)
		parent = r.ResourceID
	}
	for _, args := range [][]string{{"discussion", "reply", "add", thread.ResourceID, "--body", "Too deep", "--reply-to", parent}, {"discussion", "reply", "add", other.ResourceID, "--body", "Wrong thread", "--reply-to", parent}, {"discussion", "answer", other.ResourceID, "--expect", other.EventID, "--reply", parent}} {
		if _, err := discussionCLI(t, args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	// Reply edits preserve provenance and cannot move their immutable parent.
	rstate := DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: thread.ResourceID, Body: "Source", Metadata: map[string]string{"forge/source": "original"}}
	raw, _ = ResourceStateJSON(ResourceDiscussionReply, rstate)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reply := discussionWrite(t, "discussion-reply", "open", "--input", path)
	discussionWrite(t, "discussion", "reply", "edit", reply.ResourceID, "--expect", reply.EventID, "--body", "Correction")
	data, err = discussionCLI(t, "discussion-reply", "show", reply.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(data, &view)
	json.Unmarshal(view.State, &rstate)
	if rstate.Metadata["forge/source"] != "original" || rstate.DiscussionID != thread.ResourceID {
		t.Fatal("reply metadata lost")
	}
	if _, err := discussionCLI(t, "discussion", "replies", thread.ResourceID, "--limit", "201"); err == nil {
		t.Fatal("unbounded page")
	}
	if _, err := discussionCLI(t, "discussion", "revise", thread.ResourceID, "--expect", edit.EventID, "--body", strings.Repeat("x", 65537)); err == nil {
		t.Fatal("unbounded body")
	}
}

func TestDiscussionConcurrentHeadsAndExplicitResolution(t *testing.T) {
	root := issueCommandTestRepo(t)
	firstActor, _ := loadIdentity()
	thread := discussionWrite(t, "discussion", "open", "--title", "Question", "--category", "help")
	reply := discussionWrite(t, "discussion", "reply", "add", thread.ResourceID, "--body", "Original")
	secondActor := testIdentity(t, "Collaborator")
	// Capture both valid actor histories before either collaboration edit arrives.
	writeActiveTestIdentity(t, secondActor)
	events, _, err := collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range []*Identity{firstActor, secondActor} {
		captured, err := nextEvent(actor, "discussion.revise")
		if err != nil {
			t.Fatal(err)
		}
		state := DiscussionState{Schema: "hn.discussion/1", Title: actor.Name, Category: "help", Status: "open"}
		raw, _ := ResourceStateJSON(ResourceDiscussion, state)
		if _, err := applyResourceMutation(ResourceMutation{Family: ResourceDiscussion, Intent: "revise", ID: thread.ResourceID, Expected: []string{thread.EventID}, State: raw}, actor, captured, events); err != nil {
			t.Fatal(err)
		}
	}
	data, err := discussionCLI(t, "discussion", "show", thread.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	var view ResourceView
	json.Unmarshal(data, &view)
	if !view.Conflict || len(view.Heads) != 2 || string(view.State) != "null" {
		t.Fatalf("hidden conflict: %s", data)
	}
	if _, err := discussionCLI(t, "discussion", "close", thread.ResourceID, "--expect", view.Heads[0].ID); err == nil {
		t.Fatal("closed conflicted thread")
	}
	path := filepath.Join(root, "resolution.json")
	state := DiscussionState{Schema: "hn.discussion/1", Title: "Resolved", Category: "help", Status: "open"}
	raw, _ := ResourceStateJSON(ResourceDiscussion, state)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := discussionCLI(t, "discussion", "resolve", thread.ResourceID, "--expect", view.Heads[0].ID, "--expect", thread.EventID, "--input", path); err == nil {
		t.Fatal("resolved incomplete current heads")
	}
	discussionWrite(t, "discussion", "resolve", thread.ResourceID, "--expect", view.Heads[0].ID, "--expect", view.Heads[1].ID, "--input", path, "--operation", "resolve-thread")
	// Replies use the same explicit whole-state conflict resolution surface.
	events, _, err = collectIssueEvents()
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range []*Identity{firstActor, secondActor} {
		captured, err := nextEvent(actor, "discussion-reply.revise")
		if err != nil {
			t.Fatal(err)
		}
		state := DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: thread.ResourceID, Body: actor.Name}
		raw, _ := ResourceStateJSON(ResourceDiscussionReply, state)
		if _, err := applyResourceMutation(ResourceMutation{Family: ResourceDiscussionReply, Intent: "revise", ID: reply.ResourceID, Expected: []string{reply.EventID}, State: raw}, actor, captured, events); err != nil {
			t.Fatal(err)
		}
	}
	data, err = discussionCLI(t, "discussion-reply", "show", reply.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(data, &view)
	if !view.Conflict || len(view.Heads) != 2 {
		t.Fatalf("hidden reply conflict: %s", data)
	}
	if _, err := discussionCLI(t, "discussion", "reply", "edit", reply.ResourceID, "--expect", view.Heads[0].ID, "--body", "Unsafe winner"); err == nil {
		t.Fatal("edited conflicted reply")
	}
	rstate := DiscussionReplyState{Schema: "hn.discussion-reply/1", DiscussionID: thread.ResourceID, Body: "Resolved answer"}
	raw, _ = ResourceStateJSON(ResourceDiscussionReply, rstate)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	resolved := discussionWrite(t, "discussion", "reply", "resolve", reply.ResourceID, "--expect", view.Heads[0].ID, "--expect", view.Heads[1].ID, "--input", path, "--operation", "resolve-reply")
	// Actor-pinned retries refuse an identity switch; operation lookup stays read-only.
	writeActiveTestIdentity(t, firstActor)
	if _, err := discussionCLI(t, "discussion", "reply", "edit", reply.ResourceID, "--expect", resolved.EventID, "--body", "Changed", "--actor", secondActor.Actor); err == nil {
		t.Fatal("actor switch accepted")
	}
	if _, err := discussionCLI(t, "discussion-reply", "operation", "--actor", secondActor.Actor, "--operation", "resolve-reply"); err != nil {
		t.Fatal(err)
	}
}
