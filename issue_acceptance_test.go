package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Acceptance DTOs describe the public wire contract, not the command implementation.
type publicIssueEnvelope struct {
	Schema   string                          `json:"schema"`
	OK       bool                            `json:"ok"`
	Data     json.RawMessage                 `json:"data"`
	Error    *struct{ Code, Message string } `json:"error"`
	Snapshot string                          `json:"snapshot"`
	Next     string                          `json:"next_cursor"`
}
type publicIssueMutation struct {
	Issue    string `json:"issue_id"`
	Event    string `json:"event_id"`
	Replayed bool   `json:"replayed"`
	Conflict bool   `json:"conflict"`
	Heads    int    `json:"head_count"`
}
type publicIssueView struct {
	ID, Creator   string
	State         *IssueState
	Conflict      bool
	Heads         []string `json:"head_ids"`
	HeadCount     int      `json:"head_count"`
	RevisionCount int      `json:"revision_count"`
	CommentCount  int      `json:"comment_count"`
}
type publicIssuePage struct {
	Items      []json.RawMessage
	Total      int
	Issue      string `json:"issue_id"`
	Conflict   bool
	HeadCount  int `json:"head_count"`
	CycleCount int `json:"cycle_count"`
}
type publicIssueHead struct {
	ID, Actor string
	State     IssueState
}
type issueAcceptanceBuffer struct {
	bytes.Buffer
	limit int
}

func (b *issueAcceptanceBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("acceptance output exceeds %d bytes", b.limit)
	}
	return b.Buffer.Write(p)
}

func issuePublicCall(t *testing.T, binary, repo string, input []byte, wantCode string, args ...string) publicIssueEnvelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, append([]string{"issue"}, args...)...)
	cmd.Dir = repo
	cmd.Env = operationalTestEnvironment(t.TempDir())
	cmd.Stdin = bytes.NewReader(input)
	out := &issueAcceptanceBuffer{limit: 16 << 20}
	diagnostics := &issueAcceptanceBuffer{limit: 64 << 10}
	cmd.Stdout = out
	cmd.Stderr = diagnostics
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("issue %v timed out: %s", args, diagnostics.String())
	}
	var envelope publicIssueEnvelope
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if decodeErr := decoder.Decode(&envelope); decodeErr != nil {
		t.Fatalf("issue %v JSON: %v; exit=%v stdout=%q stderr=%q", args, decodeErr, err, out.String(), diagnostics.String())
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		t.Fatalf("issue %v emitted more than one JSON value", args)
	}
	if envelope.Schema != "hn.issue/1" {
		t.Fatalf("wrong schema %q", envelope.Schema)
	}
	if wantCode == "" {
		if err != nil || !envelope.OK || envelope.Error != nil {
			t.Fatalf("issue %v: %v %+v stderr=%q", args, err, envelope, diagnostics.String())
		}
	} else if err == nil || envelope.OK || envelope.Error == nil || envelope.Error.Code != wantCode {
		t.Fatalf("issue %v want %s: exit=%v envelope=%s stderr=%q", args, wantCode, err, out.String(), diagnostics.String())
	}
	return envelope
}
func issuePublicData[T any](t *testing.T, e publicIssueEnvelope) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(e.Data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func issuePublicState(t *testing.T, state IssueState) []byte {
	t.Helper()
	if state.Criteria == nil {
		state.Criteria = []IssueCriterion{}
	}
	if state.Labels == nil {
		state.Labels = []string{}
	}
	if state.Assignees == nil {
		state.Assignees = []string{}
	}
	if state.Relations == nil {
		state.Relations = []IssueRelation{}
	}
	if state.Metadata == nil {
		state.Metadata = map[string]string{}
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func issuePublicOpen(t *testing.T, binary, repo, title string) publicIssueMutation {
	t.Helper()
	m := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, nil, "", "open", "--json", title))
	if !validEventID(m.Issue) || m.Issue != m.Event {
		t.Fatalf("invalid opening identity: %+v", m)
	}
	return m
}
func issuePublicRefs(t *testing.T, repo string) string {
	t.Helper()
	return runOperationalGit(t, repo, "for-each-ref", "--sort=refname", "--format=%(refname) %(objectname)", "refs/hn")
}
func issuePublicNoWrite(t *testing.T, binary, repo string, input []byte, code string, args ...string) publicIssueEnvelope {
	t.Helper()
	before := issuePublicRefs(t, repo)
	e := issuePublicCall(t, binary, repo, input, code, args...)
	if after := issuePublicRefs(t, repo); after != before {
		t.Fatalf("issue %v unexpectedly changed refs", args)
	}
	return e
}
func issuePublicRepository(t *testing.T, binary, name string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), name)
	initOperationalRepository(t, repo, name)
	runOperationalGit(t, repo, "commit", "--allow-empty", "-q", "-m", "seed")
	runOperationalCommand(t, binary, repo, "init", "--name", name)
	return repo
}
func issuePublicPages(t *testing.T, binary, repo, command, id string, limit int) ([]json.RawMessage, publicIssueEnvelope) {
	t.Helper()
	args := []string{command}
	if id != "" {
		args = append(args, id)
	}
	args = append(args, "--json", "--limit", fmt.Sprint(limit))
	var items []json.RawMessage
	var first publicIssueEnvelope
	cursor := ""
	seen := map[string]bool{}
	for n := 0; n < 1000; n++ {
		query := append([]string{}, args...)
		if cursor != "" {
			query = append(query, "--cursor", cursor)
		}
		e := issuePublicCall(t, binary, repo, nil, "", query...)
		if n == 0 {
			first = e
		}
		if e.Snapshot != first.Snapshot {
			t.Fatal("snapshot changed within pages")
		}
		p := issuePublicData[publicIssuePage](t, e)
		if p.Items == nil || len(p.Items) > limit {
			t.Fatalf("invalid bounded page: %s", e.Data)
		}
		items = append(items, p.Items...)
		if e.Next == "" {
			if len(items) != p.Total {
				t.Fatalf("page total %d != items %d", p.Total, len(items))
			}
			return items, first
		}
		if seen[e.Next] {
			t.Fatal("repeated cursor")
		}
		seen[e.Next] = true
		cursor = e.Next
	}
	t.Fatal("unbounded pagination")
	return nil, first
}

func TestOperationalRichIssues(t *testing.T) {
	binary := buildOperationalBinary(t)
	t.Run("HumanLifecycleAndReplay", func(t *testing.T) {
		repo := issuePublicRepository(t, binary, "person")
		human := runOperationalCommand(t, binary, repo, "issue", "open", "Simple work")
		if !strings.Contains(human, "Simple work") {
			t.Fatalf("simple open: %s", human)
		}
		for _, args := range [][]string{{"issue", "list"}} {
			if text := runOperationalCommand(t, binary, repo, args...); !strings.Contains(text, "Simple work") {
				t.Fatal(text)
			}
		}
		opened := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, nil, "", "open", "--operation", "rich-open", "--json", "Rich work"))
		creationReplay := issuePublicData[publicIssueMutation](t, issuePublicNoWrite(t, binary, repo, nil, "", "open", "--operation", "rich-open", "--json", "Rich work"))
		if creationReplay.Event != opened.Event || !creationReplay.Replayed {
			t.Fatal("opening replay appended or changed identity")
		}

		actor := readOperationalIdentity(t, binary, repo).Actor
		commit := runOperationalGit(t, repo, "rev-parse", "refs/hn/actors/"+actor)
		payload := runOperationalGitBytes(t, repo, "show", commit+":event.json")
		signature := runOperationalGitBytes(t, repo, "show", commit+":signature")
		state := IssueState{Title: "Rich work revised", Body: "Description with context", Status: "open", Criteria: []IssueCriterion{{ID: "check", Text: "Observable result"}}, Labels: []string{"ready"}, Assignees: []string{"team/example"}, Relations: []IssueRelation{}, Metadata: map[string]string{"example/source": "consumer"}}
		data := issuePublicState(t, state)
		args := []string{"revise", opened.Issue, "--expect", opened.Event, "--input", "-", "--operation", "edit", "--json"}
		edited := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, data, "", args...))
		runOperationalCommand(t, binary, repo, "issue", "comment", opened.Issue, "A plain comment")
		commentArgs := []string{"comment", opened.Issue, "--operation", "discussion", "--json", "Structured retry"}
		comment := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, nil, "", commentArgs...))
		commentReplay := issuePublicData[publicIssueMutation](t, issuePublicNoWrite(t, binary, repo, nil, "", commentArgs...))
		if commentReplay.Event != comment.Event || !commentReplay.Replayed {
			t.Fatal("comment replay did not preserve identity")
		}
		issuePublicNoWrite(t, binary, repo, nil, "operation_conflict", "comment", opened.Issue, "--operation", "discussion", "--json", "Different discussion")

		replay := issuePublicData[publicIssueMutation](t, issuePublicNoWrite(t, binary, repo, data, "", args...))
		if replay.Event != edited.Event || !replay.Replayed {
			t.Fatalf("wrong replay: %+v", replay)
		}
		changed := state
		changed.Body = "Different"
		issuePublicNoWrite(t, binary, repo, issuePublicState(t, changed), "operation_conflict", args...)
		issuePublicNoWrite(t, binary, repo, data, "operation_conflict", "revise", opened.Issue, "--expect", edited.Event, "--input", "-", "--operation", "edit", "--json")
		other := issuePublicOpen(t, binary, repo, "Another issue")
		issuePublicNoWrite(t, binary, repo, data, "operation_conflict", "revise", other.Issue, "--expect", other.Event, "--input", "-", "--operation", "edit", "--json")

		issuePublicNoWrite(t, binary, repo, data, "stale_revision", "revise", opened.Issue, "--expect", opened.Event, "--input", "-", "--operation", "stale", "--json")
		closed := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, nil, "", "close", opened.Issue, "--expect", edited.Event, "--operation", "close", "--json"))
		reopened := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, nil, "", "reopen", opened.Issue, "--expect", closed.Event, "--operation", "reopen", "--json"))
		reopenReplay := issuePublicData[publicIssueMutation](t, issuePublicNoWrite(t, binary, repo, nil, "", "reopen", opened.Issue, "--expect", closed.Event, "--operation", "reopen", "--json"))
		if reopenReplay.Event != reopened.Event || !reopenReplay.Replayed {
			t.Fatal("reopen replay failed")
		}

		closeReplay := issuePublicData[publicIssueMutation](t, issuePublicNoWrite(t, binary, repo, nil, "", "close", opened.Issue, "--expect", edited.Event, "--operation", "close", "--json"))
		if closeReplay.Event != closed.Event {
			t.Fatal("close retry derived latest state")
		}
		issuePublicNoWrite(t, binary, repo, nil, "operation_conflict", "reopen", opened.Issue, "--expect", edited.Event, "--operation", "close", "--json")
		shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, repo, nil, "", "show", opened.Issue, "--json"))
		if shown.ID != opened.Issue || shown.State == nil || !reflect.DeepEqual(*shown.State, state) || shown.HeadCount != 1 || shown.Heads[0] != reopened.Event || shown.CommentCount != 2 {
			t.Fatalf("rich roundtrip: %+v", shown)
		}
		if text := runOperationalCommand(t, binary, repo, "issue", "show", opened.Issue); !strings.Contains(text, state.Title) {
			t.Fatal(text)
		}
		if !bytes.Equal(payload, runOperationalGitBytes(t, repo, "show", commit+":event.json")) || !bytes.Equal(signature, runOperationalGitBytes(t, repo, "show", commit+":signature")) {
			t.Fatal("opening signed bytes changed")
		}
		history, _ := issuePublicPages(t, binary, repo, "history", opened.Issue, 2)
		if len(history) != 6 {
			t.Fatalf("history count %d", len(history))
		}
	})
	t.Run("StrictJSONAndRefInvariants", func(t *testing.T) {
		repo := issuePublicRepository(t, binary, "strict")
		empty := issuePublicData[publicIssuePage](t, issuePublicCall(t, binary, repo, nil, "", "list", "--json"))
		if empty.Items == nil || len(empty.Items) != 0 {
			t.Fatal("empty list is not an empty array")
		}

		invalid := [][]byte{[]byte(`{"title":"x","unknown":1}`), []byte(`{"title":"x","title":"y"}`), []byte(`{"title":"x"}{}`), []byte(`{"title":null}`), []byte(`{"Title":"x"}`), []byte(`{"title":"\ud800"}`), append([]byte(`{"title":"`), append([]byte{0xff}, []byte(`"}`)...)...), []byte(`{"title":"` + strings.Repeat("x", 1025) + `"}`), []byte(strings.Repeat(" ", 256*1024+1))}
		for i, input := range invalid {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				code := "invalid_input"
				if i == len(invalid)-1 {
					code = "resource_limit"
				}
				issuePublicNoWrite(t, binary, repo, input, code, "open", "--input", "-", "--json=TRUE")
			})
		}
		for _, input := range []string{`{"title":"\ud83d\ude03"}`, `{"title":"\ufffd"}`} {
			issuePublicCall(t, binary, repo, []byte(input), "", "open", "--input", "-", "--json")
		}
		opened := issuePublicOpen(t, binary, repo, "guard")
		issuePublicNoWrite(t, binary, repo, nil, "invalid_input", "close", shortID(opened.Issue), "--expect", opened.Issue, "--json")
		issuePublicNoWrite(t, binary, repo, nil, "invalid_input", "list", "--json=1", "--limit", "0")
		issuePublicNoWrite(t, binary, repo, nil, "invalid_input", "list", "--json=true", "--json=garbage")
		inputFile := filepath.Join(t.TempDir(), "state.json")
		if err := os.WriteFile(inputFile, []byte(`{"title":"from file","metadata":{"example/escaped":"$(touch nope)\n\u001b[31m"}}`), 0600); err != nil {
			t.Fatal(err)
		}
		issuePublicCall(t, binary, repo, nil, "", "open", "--input", inputFile, "--json")
		if err := os.WriteFile(inputFile, []byte(`{"title":"bad file","extra":true}`), 0600); err != nil {
			t.Fatal(err)
		}
		issuePublicNoWrite(t, binary, repo, nil, "invalid_input", "open", "--input", inputFile, "--json")

		items, first := issuePublicPages(t, binary, repo, "list", "", 2)
		if len(items) != 4 {
			t.Fatalf("list count %d", len(items))
		}
		issuePublicCall(t, binary, repo, nil, "invalid_input", "list", "--json", "--limit", "3", "--cursor", first.Next)
		issuePublicCall(t, binary, repo, nil, "invalid_input", "heads", opened.Issue, "--json", "--limit", "2", "--cursor", first.Next)
		rawCursor, err := base64.RawURLEncoding.DecodeString(first.Next)
		if err != nil {
			t.Fatal(err)
		}
		var cursorFields map[string]any
		if err := json.Unmarshal(rawCursor, &cursorFields); err != nil {
			t.Fatal(err)
		}
		cursorFields["offset"] = 99999
		badCursor, err := json.Marshal(cursorFields)
		if err != nil {
			t.Fatal(err)
		}
		issuePublicCall(t, binary, repo, nil, "invalid_input", "list", "--json", "--limit", "2", "--cursor", base64.RawURLEncoding.EncodeToString(badCursor))

		issuePublicOpen(t, binary, repo, "later")
		issuePublicCall(t, binary, repo, nil, "stale_cursor", "list", "--json", "--limit", "2", "--cursor", first.Next)
		issuePublicCall(t, binary, repo, nil, "invalid_input", "list", "--json", "--cursor", "malformed")
		issuePublicCall(t, binary, repo, nil, "not_found", "show", eventID([]byte("absent acceptance issue")), "--json")
		plain := runOperationalCommand(t, binary, repo, "issue", "list", "--json=TRUE", "--json=false")
		if strings.HasPrefix(strings.TrimSpace(plain), "{") {
			t.Fatal("last false JSON flag did not select human output")
		}

	})
	t.Run("Consumer", func(t *testing.T) {
		repo := issuePublicRepository(t, binary, "consumer")
		example := filepath.Join(t.TempDir(), "issue-consumer")
		cmd := exec.Command("go", "build", "-o", example, "./examples/issue-consumer")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build consumer: %v %s", err, out)
		}
		output := runOperationalCommand(t, example, repo, "--hn", binary, "--repo", repo)
		var result struct {
			Issue    string `json:"issue_id"`
			Revision string `json:"revision_id"`
			Replay   string `json:"replay_id"`
			Stale    string `json:"stale_code"`
		}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatal(err)
		}
		if result.Revision != result.Replay || result.Stale != "stale_revision" {
			t.Fatal(output)
		}
		shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, repo, nil, "", "show", result.Issue, "--json"))
		if shown.HeadCount != 1 || shown.Heads[0] != result.Revision || shown.RevisionCount != 2 || shown.State == nil || len(shown.State.Criteria) != 1 || shown.State.Metadata["example/source"] != "issue-consumer" || len(shown.State.Assignees) != 1 || shown.State.Status != "open" {
			t.Fatalf("consumer claims differ from repository: %+v", shown)
		}
		if failure := runOperationalCommandFailure(t, example, repo, "--hn", filepath.Join(t.TempDir(), "absent"), "--repo", repo); failure == "" {
			t.Fatal("missing executable diagnosis")
		}
		if failure := runOperationalCommandFailure(t, example, repo, "--hn", binary, "--repo", filepath.Join(t.TempDir(), "absent")); failure == "" {
			t.Fatal("missing repository diagnosis")
		}

	})
}

func issuePublicReplicas(t *testing.T, binary string, names ...string) ([]string, string) {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed")
	initOperationalRepository(t, seed, "Seed")
	runOperationalGit(t, seed, "commit", "--allow-empty", "-q", "-m", "seed")
	remote := filepath.Join(t.TempDir(), "project.git")
	runOperationalGit(t, "", "clone", "--bare", "-q", seed, remote)
	repos := make([]string, 0, len(names))
	for _, name := range names {
		repo := filepath.Join(t.TempDir(), name)
		runOperationalGit(t, "", "clone", "-q", remote, repo)
		configureOperationalGit(t, repo, name)
		runOperationalCommand(t, binary, repo, "init", "--name", name)
		repos = append(repos, repo)
	}
	return repos, remote
}
func issuePublicRevise(t *testing.T, binary, repo, id, expect string, state IssueState) publicIssueMutation {
	t.Helper()
	return issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, issuePublicState(t, state), "", "revise", id, "--expect", expect, "--input", "-", "--json"))
}
func issuePublicHeadIDs(t *testing.T, items []json.RawMessage) []string {
	t.Helper()
	ids := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, raw := range items {
		var head publicIssueHead
		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatal(err)
		}
		if !validEventID(head.ID) || seen[head.ID] {
			t.Fatal("missing/duplicate head identity")
		}
		seen[head.ID] = true
		ids = append(ids, head.ID)
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("head order is not stable ID order")
	}
	return ids
}

func TestOperationalRichIssueReplication(t *testing.T) {
	binary := buildOperationalBinary(t)
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("ConflictArrivalReverse=%t", reverse), func(t *testing.T) {
			repos, _ := issuePublicReplicas(t, binary, "Alice", "Bob")
			alice, bob := repos[0], repos[1]
			root := issuePublicOpen(t, binary, alice, "Concurrent work")
			target := issuePublicOpen(t, binary, alice, "Related work")
			runOperationalCommand(t, binary, alice, "sync", "origin")
			runOperationalCommand(t, binary, bob, "sync", "origin")
			a := issuePublicRevise(t, binary, alice, root.Issue, root.Event, IssueState{Title: "Alice's work", Status: "open", Relations: []IssueRelation{{Kind: "related", Target: target.Issue}}})
			b := issuePublicRevise(t, binary, bob, root.Issue, root.Event, IssueState{Title: "Bob's work", Status: "open"})
			order := []string{alice, bob, alice}
			if reverse {
				order = []string{bob, alice, bob}
			}
			for _, repo := range order {
				runOperationalCommand(t, binary, repo, "sync", "origin")
			}
			for _, repo := range repos {
				shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, repo, nil, "", "show", root.Issue, "--json"))
				if !shown.Conflict || shown.State != nil || shown.HeadCount != 2 {
					t.Fatalf("conflict hidden: %+v", shown)
				}
				items, _ := issuePublicPages(t, binary, repo, "heads", root.Issue, 1)
				ids := issuePublicHeadIDs(t, items)
				want := []string{a.Event, b.Event}
				sort.Strings(want)
				if !reflect.DeepEqual(ids, want) {
					t.Fatalf("lost heads %v", ids)
				}
				actors := map[string]bool{}
				titles := map[string]bool{}
				for _, raw := range items {
					var h publicIssueHead
					json.Unmarshal(raw, &h)
					actors[h.Actor] = true
					titles[h.State.Title] = true
				}
				if len(actors) != 2 || !titles["Alice's work"] || !titles["Bob's work"] {
					t.Fatal("lost conflict attribution")
				}
			}
			graph, _ := issuePublicPages(t, binary, alice, "graph", target.Issue, 1)
			if len(graph) != 1 || !bytes.Contains(graph[0], []byte(`"ambiguous":true`)) || !bytes.Contains(graph[0], []byte(a.Event)) {
				t.Fatalf("incoming conflict graph: %s", graph)
			}
			state := issuePublicState(t, IssueState{Title: "Explicitly reconciled", Status: "open"})
			issuePublicNoWrite(t, binary, alice, state, "stale_revision", "resolve", root.Issue, "--expect", a.Event, "--expect", root.Event, "--input", "-", "--json")
			resolved := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, alice, state, "", "resolve", root.Issue, "--expect", a.Event, "--expect", b.Event, "--input", "-", "--operation", "resolve", "--json"))
			runOperationalCommand(t, binary, alice, "sync", "origin")
			runOperationalCommand(t, binary, bob, "sync", "origin")
			for _, repo := range repos {
				shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, repo, nil, "", "show", root.Issue, "--json"))
				if shown.Conflict || shown.HeadCount != 1 || shown.Heads[0] != resolved.Event || shown.RevisionCount != 4 {
					t.Fatalf("did not converge: %+v", shown)
				}
			}
		})
	}
	t.Run("DistributedGraphCycles", func(t *testing.T) {
		repos, _ := issuePublicReplicas(t, binary, "GraphA", "GraphB")
		a, b := repos[0], repos[1]
		x := issuePublicOpen(t, binary, a, "X")
		y := issuePublicOpen(t, binary, a, "Y")
		runOperationalCommand(t, binary, a, "sync")
		runOperationalCommand(t, binary, b, "sync")
		xRevision := issuePublicRevise(t, binary, a, x.Issue, x.Event, IssueState{Title: "X", Status: "open", Relations: []IssueRelation{{Kind: "blocks", Target: y.Issue}, {Kind: "parent", Target: y.Issue}}})
		yState := IssueState{Title: "Y", Status: "open", Relations: []IssueRelation{{Kind: "blocks", Target: x.Issue}, {Kind: "parent", Target: x.Issue}}}
		issuePublicNoWrite(t, binary, a, issuePublicState(t, yState), "invalid_input", "revise", y.Issue, "--expect", y.Event, "--input", "-", "--json")
		yRevision := issuePublicRevise(t, binary, b, y.Issue, y.Event, yState)
		runOperationalCommand(t, binary, a, "sync")
		runOperationalCommand(t, binary, b, "sync")
		runOperationalCommand(t, binary, a, "sync")
		items, first := issuePublicPages(t, binary, a, "graph", x.Issue, 1)
		page := issuePublicData[publicIssuePage](t, first)
		if page.CycleCount != 2 || len(items) != 8 {
			t.Fatalf("cycle graph: total=%d components=%d items=%s", len(items), page.CycleCount, items)
		}
		var joined []byte
		for _, item := range items {
			joined = append(joined, item...)
		}
		for _, id := range []string{xRevision.Event, yRevision.Event, x.Issue, y.Issue} {
			if !bytes.Contains(joined, []byte(id)) {
				t.Fatalf("graph omitted provenance %s", id)
			}
		}
	})
	t.Run("ExactSupplierAdmissionRecovery", func(t *testing.T) {
		repos, remote := issuePublicReplicas(t, binary, "Creator", "Parent", "Target", "Dependent")
		creator, parent, target, dependent := repos[0], repos[1], repos[2], repos[3]
		root := issuePublicOpen(t, binary, creator, "Root")
		runOperationalCommand(t, binary, creator, "sync")
		runOperationalCommand(t, binary, parent, "sync")
		revision := issuePublicRevise(t, binary, parent, root.Issue, root.Event, IssueState{Title: "Parent", Status: "open"})
		runOperationalCommand(t, binary, parent, "sync")
		linked := issuePublicOpen(t, binary, target, "Link target")
		runOperationalCommand(t, binary, target, "sync")
		runOperationalCommand(t, binary, dependent, "sync")
		last := issuePublicRevise(t, binary, dependent, root.Issue, revision.Event, IssueState{Title: "Dependent", Status: "open", Relations: []IssueRelation{{Kind: "blocks", Target: linked.Issue}}})
		runOperationalCommand(t, binary, dependent, "sync")
		actors := make([]string, len(repos))
		for i, repo := range repos {
			actors[i] = readOperationalIdentity(t, binary, repo).Actor
		}
		for _, step := range []struct {
			selected []string
			missing  string
		}{{[]string{actors[1], actors[2], actors[3]}, root.Issue}, {[]string{actors[0], actors[2], actors[3]}, revision.Event}, {[]string{actors[0], actors[1], actors[3]}, linked.Issue}} {
			receiver := filepath.Join(t.TempDir(), "Receiver")
			runOperationalGit(t, "", "clone", "-q", remote, receiver)
			configureOperationalGit(t, receiver, "Receiver")
			runOperationalCommand(t, binary, receiver, "init", "--name", "Receiver")
			selectOperationalReplication(t, binary, receiver, step.selected, nil)
			selectionBefore := runOperationalCommand(t, binary, receiver, "replication", "show", "origin")
			failure := runOperationalCommandFailure(t, binary, receiver, "sync", "origin")
			if !strings.Contains(failure, "dependency-missing") || !strings.Contains(failure, step.missing) {
				t.Fatalf("missing precise supplier: %s", failure)
			}
			refs := issuePublicRefs(t, receiver)
			if strings.Contains(refs, "refs/hn/remotes/origin/actors/"+actors[3]) {
				t.Fatal("dependent history promoted without supplier")
			}
			if after := runOperationalCommand(t, binary, receiver, "replication", "show", "origin"); after != selectionBefore {
				t.Fatal("failed sync broadened selection")
			}

			selectOperationalReplication(t, binary, receiver, actors[:4], nil)
			runOperationalCommand(t, binary, receiver, "sync", "origin")
			shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, receiver, nil, "", "show", root.Issue, "--json"))
			if shown.HeadCount != 1 || shown.Heads[0] != last.Event || shown.State.Title != "Dependent" {
				t.Fatalf("wrong admitted facts: %+v", shown)
			}
		}
	})
}

func TestOperationalRichIssueStagedHeads(t *testing.T) {
	binary := buildOperationalBinary(t)
	repo := issuePublicRepository(t, binary, "Resolver")
	root := issuePublicOpen(t, binary, repo, "Many heads")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if err := os.Chdir(original); err != nil {
				t.Fatal(err)
			}
		}()
		for i := 0; i < 201; i++ {
			actor := testIdentity(t, fmt.Sprintf("Concurrent %03d", i))
			event := newEvent(actor, "issue.revise", 1, "")
			event.Subject = root.Issue
			event.Parents = []string{root.Event}
			event.Issue = &IssueState{Title: fmt.Sprintf("Head %03d", i), Status: "open"}
			event.Intent = "revise"
			if _, err := appendEvent(event, actor); err != nil {
				t.Fatal(err)
			}
		}
	}()
	items, first := issuePublicPages(t, binary, repo, "heads", root.Issue, 50)
	ids := issuePublicHeadIDs(t, items)
	if len(ids) != 201 {
		t.Fatalf("only %d heads", len(ids))
	}
	shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, repo, nil, "", "show", root.Issue, "--json"))
	if len(shown.Heads) != 50 || shown.HeadCount != 201 || shown.State != nil {
		t.Fatalf("show truncation hidden: %+v", shown)
	}
	state := issuePublicState(t, IssueState{Title: "Combined", Status: "open"})
	args := []string{"resolve", root.Issue, "--partial", "--snapshot", first.Snapshot, "--input", "-", "--json"}
	for _, id := range ids[:200] {
		args = append(args, "--expect", id)
	}
	runOperationalCommand(t, binary, repo, "issue", "comment", root.Issue, "Changes snapshot without changing heads")
	issuePublicNoWrite(t, binary, repo, state, "stale_revision", args...)
	fresh := issuePublicCall(t, binary, repo, nil, "", "heads", root.Issue, "--json")
	args[4] = fresh.Snapshot
	ordinary := []string{"resolve", root.Issue, "--input", "-", "--json"}
	for _, id := range ids[:200] {
		ordinary = append(ordinary, "--expect", id)
	}
	issuePublicNoWrite(t, binary, repo, state, "stale_revision", ordinary...)
	partial := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, state, "", args...))
	if partial.Heads != 2 || !partial.Conflict {
		t.Fatalf("partial lost heads: %+v", partial)
	}
	remaining, _ := issuePublicPages(t, binary, repo, "heads", root.Issue, 1)
	remainingIDs := issuePublicHeadIDs(t, remaining)
	want := []string{ids[200], partial.Event}
	sort.Strings(want)
	if !reflect.DeepEqual(remainingIDs, want) {
		t.Fatalf("unselected head lost: %v", remainingIDs)
	}
	final := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repo, state, "", "resolve", root.Issue, "--expect", remainingIDs[0], "--expect", remainingIDs[1], "--input", "-", "--json"))
	if final.Heads != 1 || final.Conflict {
		t.Fatal("staged recovery did not converge")
	}
	history, _ := issuePublicPages(t, binary, repo, "history", root.Issue, 200)
	if len(history) != 205 {
		t.Fatalf("lost revision/comment history: %d", len(history))
	}
}

func TestOperationalRichIssueThousandQuery(t *testing.T) {
	binary := buildOperationalBinary(t)
	repo := issuePublicRepository(t, binary, "Inventory")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Error(err)
		}
	}()
	identity := testIdentity(t, "Inventory fixture")
	var previous *StoredEvent
	for i := 0; i < 1000; i++ {
		event := newEvent(identity, "issue.open", uint64(i+1), "")
		if previous != nil {
			event.Previous = previous.ID
		}
		event.Title = fmt.Sprintf("Inventory %04d", i)
		event.Body = "A realistic signed issue description with context and an observable outcome."
		previous, err = appendEvent(event, identity)
		if err != nil {
			t.Fatal(err)
		}
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := t.TempDir()
	callsPath := filepath.Join(wrapper, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ISSUE_ACCEPT_CALLS\"\nexec \"$ISSUE_ACCEPT_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ISSUE_ACCEPT_REAL_GIT", realGit)
	t.Setenv("ISSUE_ACCEPT_CALLS", callsPath)
	t.Setenv("PATH", wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
	readCalls := func() []byte {
		data, err := os.ReadFile(callsPath)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	clearCalls := func() {
		if err := os.WriteFile(callsPath, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	old, err := collectEvents()
	oldTime := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	oldCalls := readCalls()
	clearCalls()
	start = time.Now()
	rebuilt, _, err := collectIssueEvents()
	readerTime := time.Since(start)
	if err != nil || !reflect.DeepEqual(old, rebuilt) {
		t.Fatalf("integrated reader equality failed: %v", err)
	}
	readerCalls := readCalls()
	clearCalls()
	start = time.Now()
	items, _ := issuePublicPages(t, binary, repo, "list", "", 200)
	queryTime := time.Since(start)
	queryCalls := readCalls()
	expected := []string{}
	for _, stored := range old {
		if stored.Event.Kind == "issue.open" {
			expected = append(expected, stored.ID)
		}
	}
	sort.Strings(expected)
	actual := []string{}
	for _, raw := range items {
		var summary struct{ ID, Title, Status string }
		if err := json.Unmarshal(raw, &summary); err != nil {
			t.Fatal(err)
		}
		if summary.Status != "open" || !strings.HasPrefix(summary.Title, "Inventory ") {
			t.Fatalf("invalid query summary: %s", raw)
		}
		actual = append(actual, summary.ID)
	}
	if len(actual) != 1000 || !reflect.DeepEqual(actual, expected) {
		t.Fatal("public query omitted, duplicated or changed verified inventory")
	}
	if bytes.Count(readerCalls, []byte("cat-file --batch\n")) != 1 || bytes.Count(queryCalls, []byte("cat-file --batch\n")) != 5 || bytes.Count(queryCalls, []byte("rev-list ")) != 5 || bytes.Contains(queryCalls, []byte("show ")) {
		t.Fatalf("public query did not use constant batches: %s", queryCalls)
	}
	defaultPage := issuePublicData[publicIssuePage](t, issuePublicCall(t, binary, repo, nil, "", "list", "--json"))
	if len(defaultPage.Items) != 50 || defaultPage.Total != 1000 {
		t.Fatal("wrong default page bound")
	}
	issuePublicCall(t, binary, repo, nil, "invalid_input", "list", "--limit", "201", "--json")

	prefixes := map[byte]int{}
	prefix := ""
	for _, id := range actual {
		prefixes[id[7]]++
		if prefixes[id[7]] > 1 {
			prefix = id[7:8]
			break
		}
	}
	if prefix == "" {
		t.Fatal("fixture lacks ambiguous prefix")
	}
	issuePublicCall(t, binary, repo, nil, "ambiguous_id", "show", prefix, "--json")
	t.Logf("Integrated1000issue equality=true; old=%s/%d Git processes; batch=%s/%d processes; public5pages=%s/%d processes (5 batches,5 traversals)", oldTime, bytes.Count(oldCalls, []byte("\n")), readerTime, bytes.Count(readerCalls, []byte("\n")), queryTime, bytes.Count(queryCalls, []byte("\n")))
}

func TestOperationalRichIssueDocumentedInput(t *testing.T) {
	binary := buildOperationalBinary(t)
	repository := filepath.Join(t.TempDir(), "documented")
	initOperationalRepository(t, repository, "Example")
	runOperationalCommand(t, binary, repository, "init", "--name", "Example device")
	doc, err := os.ReadFile("docs/issues-v1.md")
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(doc), "<!-- example: rich-input -->")
	if !ok {
		t.Fatal("documented input missing")
	}
	_, after, ok = strings.Cut(after, "```json\n")
	if !ok {
		t.Fatal("documented JSON fence missing")
	}
	input, _, ok := strings.Cut(after, "\n```")
	if !ok {
		t.Fatal("documented JSON fence incomplete")
	}
	inputPath := filepath.Join(repository, "issue.json")
	if err := os.WriteFile(inputPath, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	opened := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repository, nil, "", "open", "--input", inputPath, "--operation", "onboarding-create", "--json"))
	issuePublicCall(t, binary, repository, nil, "", "show", opened.Issue, "--json")
	edited := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repository, nil, "", "revise", opened.Issue, "--expect", opened.Event, "--input", inputPath, "--operation", "onboarding-edit", "--json"))
	closed := issuePublicData[publicIssueMutation](t, issuePublicCall(t, binary, repository, nil, "", "close", opened.Issue, "--expect", edited.Event, "--operation", "onboarding-close", "--json"))
	issuePublicCall(t, binary, repository, nil, "", "reopen", opened.Issue, "--expect", closed.Event, "--operation", "onboarding-reopen", "--json")
	var desired IssueState
	if err := json.Unmarshal([]byte(input), &desired); err != nil {
		t.Fatal(err)
	}
	shown := issuePublicData[publicIssueView](t, issuePublicCall(t, binary, repository, nil, "", "show", opened.Issue, "--json"))
	if shown.State == nil || !reflect.DeepEqual(*shown.State, desired) {
		t.Fatal("documented input did not roundtrip")
	}
	for _, args := range [][]string{{"help", "--json"}, {"heads", opened.Issue, "--limit", "50", "--json"}, {"history", opened.Issue, "--kind", "revisions", "--limit", "50", "--json"}, {"history", opened.Issue, "--kind", "comments", "--limit", "50", "--json"}, {"graph", opened.Issue, "--limit", "50", "--json"}, {"list", "--limit", "50", "--json"}} {
		issuePublicCall(t, binary, repository, nil, "", args...)
	}
}
