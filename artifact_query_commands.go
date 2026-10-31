package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() {
	artifactCommands["list"] = cmdArtifactList
	artifactCommands["help"] = cmdArtifactHelp
	artifactCommands["--help"] = cmdArtifactHelp
}

// This help is local to the explicit artifact command. Registration performs no
// I/O and leaves the ordinary native startup/help path untouched.
const artifactLocalHelp = `hn artifact type publish --input FILE|- --actor ACTOR --operation KEY [--json]
hn artifact type show FULL_ID [--json]
hn artifact open|revise|resolve|convert --input FILE|- --actor ACTOR --operation KEY [--json]
hn artifact show|heads|history FULL_ID [--json]
hn artifact list [--type TYPE_ID] [--shape document|list|ticket] --limit 1..200 [--cursor TOKEN] [--json]
hn artifact operation --actor ACTOR --operation KEY --type TYPE_ID [--json]
hn artifact export --id FULL_ID [--id FULL_ID] --output FILE [--json]
hn artifact import --input FILE|- [--preserve-opaque] [--json]
hn artifact fetch|push --remote NAME --id FULL_ID [--id FULL_ID] [--json]
hn artifact inspect --id FULL_ID [--json]
hn artifact inspect --opaque --opaque-id FULL_ID [--json]
hn artifact export --opaque --opaque-id FULL_ID --output FILE [--json]
hn artifact envelope validate|inspect --input FILE|- --signature RAW_BASE64 [--json]
hn artifact schema|record validate --input FILE|- [--json]
hn artifact type validate --input FILE|- [--json]

Custom content is inert data. Inspection never executes or activates stored text.
Use complete sha256: IDs. Mutations carry explicit exact parents and immutable
actor/operation keys; inspect operation after a lost acknowledgement, then retry
only the identical intent. A resolution names every current head. Type evolution
keeps old schemas; conversion explicitly names its source revision.
List cursors bind filters and the accepted source cut. Page size may change;
checkpoint_invalidated requires restarting from the first page. No specialized
consumer or extension registry is required. Opaque bytes are explicitly preserved
public data and never acquire accepted visibility or signature validity.
`

func cmdArtifactHelp(args []string) error {
	if len(args) != 0 {
		return artifactOutput(nil, artifactFailure("invalid_input", "artifact help accepts no arguments"), hasJSON(args))
	}
	_, err := fmt.Fprint(os.Stdout, artifactLocalHelp)
	return err
}

var artifactListShapes = map[string]bool{"document": true, "list": true, "ticket": true}

type artifactListOptions struct {
	typeID, shape, cursor string
	limit                 int
	machine               bool
}

func cmdArtifactList(args []string) error {
	o := artifactListOptions{machine: hasJSON(args)}
	seen := map[string]bool{}
	invalid := func(message string) error {
		return artifactOutput(nil, artifactFailure("invalid_input", message), o.machine)
	}
	for i := 0; i < len(args); i++ {
		name, value, equal := strings.Cut(args[i], "=")
		if seen[name] {
			return invalid("duplicate list flag")
		}
		seen[name] = true
		if name == "--json" {
			if equal && value != "true" && value != "false" {
				return invalid("invalid json flag")
			}
			o.machine = !equal || value == "true"
			continue
		}
		if name != "--type" && name != "--shape" && name != "--limit" && name != "--cursor" {
			return invalid("unknown list flag or positional argument")
		}
		if !equal {
			i++
			if i >= len(args) {
				return invalid("list flag requires a value")
			}
			value = args[i]
		}
		if value == "" {
			return invalid("list flag requires a value")
		}
		switch name {
		case "--type":
			if !artifactOperationType.MatchString(value) || strings.HasPrefix(value, "hn/") || strings.HasPrefix(value, "hubnot/") {
				return invalid("requires a valid custom type ID")
			}
			o.typeID = value
		case "--shape":
			if !artifactListShapes[value] {
				return invalid("requires document, list or ticket shape")
			}
			o.shape = value
		case "--limit":
			n, err := strconv.ParseInt(value, 10, 32)
			if err != nil || n < 1 {
				return invalid("requires --limit 1..200")
			}
			if n > 200 {
				return artifactOutput(nil, artifactFailure("resource_limit", "page limit exceeds 200"), o.machine)
			}
			o.limit = int(n)
		case "--cursor":
			if len(value) > 4096 {
				return artifactOutput(nil, artifactFailure("resource_limit", "cursor exceeds 4096 bytes"), o.machine)
			}
			o.cursor = value
		}
	}
	if o.limit == 0 {
		return invalid("requires --limit 1..200")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, err := readArtifactList(ctx, o)
	if err != nil {
		return artifactListOutput(nil, err, o.machine)
	}
	// The list endpoint returns Page(T): bound the complete encoded success
	// envelope before emitting either machine or human output. Ordinary history
	// and inspect retain their independent selected-history contracts.
	encoded, encodeErr := json.Marshal(map[string]any{"ok": true, "data": page, "error": nil})
	if encodeErr != nil {
		return encodeErr
	}
	if ctx.Err() != nil {
		return artifactOutput(nil, artifactFailure("cancelled", "artifact list cancelled"), o.machine)
	}
	if len(encoded)+1 > 8388608 {
		return artifactOutput(nil, artifactFailure("resource_limit", "artifact list response exceeds 8 MiB; reduce --limit or use explicit export"), o.machine)
	}
	if o.machine {
		return artifactOutput(page, nil, true)
	}
	if _, err = fmt.Fprintf(os.Stdout, "Artifacts at %s\n", page.SourceCut); err != nil {
		return err
	}
	for _, view := range page.Items {
		if _, err = fmt.Fprintf(os.Stdout, "%s  %s  %s  heads=%d conflict=%t\n", view.ID, view.TypeID, view.Shape, len(view.Heads), view.Conflict); err != nil {
			return err
		}
		// JSON escaping keeps control bytes in untrusted titles/content inert even in
		// the descriptive human view; the complete state remains available.
		raw, e := json.MarshalIndent(view, "", "  ")
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintln(os.Stdout, string(raw)); e != nil {
			return e
		}
	}
	if page.NextCursor != nil {
		_, err = fmt.Fprintf(os.Stdout, "Continue with --cursor %s and the same filters.\n", *page.NextCursor)
	}
	return err
}

type artifactListPage struct {
	Items      []artifactRecordView `json:"items"`
	SourceCut  string               `json:"source_cut"`
	NextCursor *string              `json:"next_cursor"`
}

// This is a private continuation encoding, never a grant or authority token.
// Every request independently verifies the complete accepted selection.
type artifactListCursor struct {
	Version int    `json:"version"`
	TypeID  string `json:"type_id"`
	Shape   string `json:"shape"`
	Cut     string `json:"cut"`
	Offset  int    `json:"offset"`
}

func artifactListCheckpoint() error {
	return &artifactStorageError{Origin: "host", Code: "checkpoint_invalidated", Message: "artifact list cursor no longer matches the selection; restart without --cursor"}
}
func artifactListOutput(data any, err error, machine bool) error {
	if failure, ok := err.(*artifactStorageError); ok && failure.Origin == "host" && failure.Code == "checkpoint_invalidated" && machine {
		if e := json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "data": nil, "error": map[string]any{"origin": "host", "code": "checkpoint_invalidated", "message": failure.Message, "recovery": "rescan_required", "operation": nil}}); e != nil {
			return e
		}
		return fmt.Errorf("artifact request refused")
	}
	return artifactStorageOutput(data, err, machine, "", "")
}
func readArtifactList(ctx context.Context, o artifactListOptions) (artifactListPage, error) {
	page := artifactListPage{Items: []artifactRecordView{}}
	h, err := loadArtifactHistory(ctx)
	if err != nil {
		return page, err
	}
	c, err := buildArtifactCatalog(ctx, h)
	if err != nil {
		return page, err
	}
	ids := []string{}
	for id := range c.records {
		payload := artifactPayload(h.ByID[id])
		if (o.typeID == "" || artifactText(payload, "type_id") == o.typeID) && (o.shape == "" || artifactText(payload, "shape") == o.shape) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	start := 0
	if o.cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(o.cursor)
		if e != nil || base64.RawURLEncoding.EncodeToString(raw) != o.cursor {
			return page, artifactListCheckpoint()
		}
		var token artifactListCursor
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if e = d.Decode(&token); e != nil {
			return page, artifactListCheckpoint()
		}
		// Re-encoding also rejects duplicate keys and alternative token encodings.
		canonical, e := json.Marshal(token)
		if e != nil || !bytes.Equal(raw, canonical) || d.Decode(new(any)) != io.EOF || token.Version != 1 || token.TypeID != o.typeID || token.Shape != o.shape || token.Cut != h.Cut || token.Offset <= 0 || token.Offset >= len(ids) {
			return page, artifactListCheckpoint()
		}
		start = token.Offset
	}
	end := start + o.limit
	if end > len(ids) {
		end = len(ids)
	}
	for _, id := range ids[start:end] {
		if ctx.Err() != nil {
			return page, artifactFailure("cancelled", "artifact list cancelled")
		}
		view, e := c.view(ctx, id)
		if e != nil {
			return page, e
		}
		page.Items = append(page.Items, view)
	}
	after, err := artifactSources(ctx, h.gitDir)
	if err != nil {
		return page, err
	}
	if !reflect.DeepEqual(h.Sources, after) {
		return page, artifactFailure("concurrent_update", "artifact sources changed while listing")
	}
	if ctx.Err() != nil {
		return page, artifactFailure("cancelled", "artifact list cancelled")
	}
	page.SourceCut = h.Cut
	if end < len(ids) {
		raw, e := json.Marshal(artifactListCursor{Version: 1, TypeID: o.typeID, Shape: o.shape, Cut: h.Cut, Offset: end})
		if e != nil {
			return page, e
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &token
	}
	return page, nil
}
