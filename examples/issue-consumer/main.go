// issue-consumer demonstrates the public hn.issue/1 contract. It intentionally
// creates and revises demonstration work in the explicitly supplied repository.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type envelope struct {
	Schema string                          `json:"schema"`
	OK     bool                            `json:"ok"`
	Data   json.RawMessage                 `json:"data"`
	Error  *struct{ Code, Message string } `json:"error"`
}
type mutation struct {
	Issue    string `json:"issue_id"`
	Event    string `json:"event_id"`
	Replayed bool   `json:"replayed"`
}
type state struct {
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Status    string            `json:"status"`
	Criteria  []criterion       `json:"criteria"`
	Labels    []string          `json:"labels"`
	Assignees []string          `json:"assignees"`
	Metadata  map[string]string `json:"metadata"`
}
type criterion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type commandError struct {
	code, message string
	cause         error
}

func (e *commandError) Error() string { return e.code + ": " + e.message }
func (e *commandError) Unwrap() error { return e.cause }

type capture struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *capture) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

type client struct{ binary, repository string }

func (c client) call(input []byte, args ...string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, c.binary, append([]string{"issue"}, args...)...)
	command.Dir = c.repository
	command.Stdin = bytes.NewReader(input)
	command.WaitDelay = 100 * time.Millisecond
	out, diagnostic := &capture{limit: 8 << 20}, &capture{limit: 32 << 10}
	command.Stdout = out
	command.Stderr = diagnostic
	exitErr := command.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("hn command timed out: %w", ctx.Err())
	}
	if out.overflow || diagnostic.overflow {
		return nil, fmt.Errorf("hn response exceeded example's bounded capture")
	}
	if out.Len() == 0 && exitErr != nil {
		return nil, fmt.Errorf("run hn: %w (stderr %q)", exitErr, diagnostic.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	var response envelope
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode hn response: %w", err)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || response.Schema != "hn.issue/1" {
		return nil, fmt.Errorf("unexpected hn response schema or trailing output")
	}
	if !response.OK {
		if response.Error == nil || exitErr == nil {
			return nil, fmt.Errorf("inconsistent hn failure response")
		}
		return nil, &commandError{response.Error.Code, response.Error.Message, exitErr}
	}
	if exitErr != nil || response.Error != nil || len(response.Data) == 0 {
		return nil, fmt.Errorf("inconsistent hn success response: %v", exitErr)
	}
	return response.Data, nil
}
func (c client) mutate(input []byte, args ...string) (mutation, error) {
	data, err := c.call(input, args...)
	if err != nil {
		return mutation{}, err
	}
	var result mutation
	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if !fullID(result.Issue) || !fullID(result.Event) {
		return result, fmt.Errorf("mutation omitted full identities")
	}
	return result, nil
}
func fullID(id string) bool {
	if len(id) != 71 || id[:7] != "sha256:" {
		return false
	}
	_, err := hex.DecodeString(id[7:])
	return err == nil
}

func demonstrate(c client) error {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	operation := "example-" + hex.EncodeToString(nonce)
	desired := state{Title: "Consumer demonstration", Body: "Read, revise, retry and reject stale work.", Status: "open", Criteria: []criterion{{"roundtrip", "The public CLI retains the requested work."}}, Labels: []string{"example"}, Assignees: []string{"example/team"}, Metadata: map[string]string{"example/source": "issue-consumer"}}
	initial, _ := json.Marshal(desired)
	opened, err := c.mutate(initial, "open", "--input", "-", "--operation", operation+"-open", "--json")
	if err != nil {
		return err
	}
	shown, err := c.call(nil, "show", opened.Issue, "--json")
	if err != nil {
		return err
	}
	var view struct {
		ID    string
		Heads []string `json:"head_ids"`
		State *state
	}
	if err = json.Unmarshal(shown, &view); err != nil {
		return err
	}
	if view.ID != opened.Issue || len(view.Heads) != 1 || view.Heads[0] != opened.Event || view.State == nil {
		return fmt.Errorf("opening is not the observed current issue")
	}
	listed, err := c.call(nil, "list", "--limit", "50", "--json")
	if err != nil {
		return err
	}
	var page struct{ Items []json.RawMessage }
	if err = json.Unmarshal(listed, &page); err != nil || page.Items == nil {
		return fmt.Errorf("list did not return an item array")
	}
	desired.Body = "Updated through a guarded full-state replacement."
	updated, _ := json.Marshal(desired)
	args := []string{"revise", opened.Issue, "--expect", opened.Event, "--input", "-", "--operation", operation + "-revise", "--json"}
	revised, err := c.mutate(updated, args...)
	if err != nil {
		return err
	}
	replayed, err := c.mutate(updated, args...)
	if err != nil {
		return err
	}
	if replayed.Event != revised.Event || !replayed.Replayed {
		return fmt.Errorf("retry did not return the original revision")
	}
	_, err = c.mutate(updated, "revise", opened.Issue, "--expect", opened.Event, "--input", "-", "--operation", operation+"-stale", "--json")
	var stale *commandError
	if !errors.As(err, &stale) || stale.code != "stale_revision" {
		return fmt.Errorf("expected stale_revision, received %v", err)
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Issue    string `json:"issue_id"`
		Original string `json:"original_revision_id"`
		Revision string `json:"revision_id"`
		Replay   string `json:"replay_id"`
		Stale    string `json:"stale_code"`
	}{opened.Issue, opened.Event, revised.Event, replayed.Event, stale.code})
}
func main() {
	binary := flag.String("hn", "", "path to hn executable")
	repository := flag.String("repo", "", "explicit initialized demonstration repository")
	flag.Parse()
	if *binary == "" || *repository == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: issue-consumer --hn /absolute/hn --repo /explicit/repository (creates demonstration work)")
		os.Exit(2)
	}
	executable, err := exec.LookPath(*binary)
	if err == nil {
		executable, err = filepath.Abs(executable)
	}
	var directory string
	if err == nil {
		directory, err = filepath.Abs(*repository)
	}
	if err == nil {
		err = demonstrate(client{executable, directory})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
