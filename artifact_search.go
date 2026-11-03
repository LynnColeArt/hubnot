package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func cmdArtifactSearchHelp(args []string) error {
	if err := cmdArtifactHelp(args); err != nil {
		return err
	}
	_, err := fmt.Fprintln(os.Stdout, `hn artifact search [--type TYPE_ID] --literal TEXT|--regex PATTERN --limit 1..200 [--cursor TOKEN] [--json]
Search current heads' title/content, canonical fields and public identity metadata.
Regex uses Go syntax, at most 4096 UTF-8 pattern bytes/expanded-compiled instructions
and 67108864 instruction-bytes over the entire type-filtered selection. A changed
query or source cut invalidates continuation; restart without --cursor. Reduce
--limit for a page exceeding 8 MiB. Matching content remains inert data.`)
	return err
}

type artifactSearchOptions struct {
	typeID, mode, text, cursor string
	limit                      int
	machine                    bool
}

func cmdArtifactSearch(args []string) error {
	o := artifactSearchOptions{machine: hasJSON(args)}
	seen := map[string]bool{}
	invalid := func(message string) error {
		return artifactOutput(nil, artifactFailure("invalid_input", message), o.machine)
	}
	for i := 0; i < len(args); i++ {
		name, value, equal := strings.Cut(args[i], "=")
		if seen[name] {
			return invalid("duplicate search flag")
		}
		seen[name] = true
		if name == "--json" {
			if equal && value != "true" && value != "false" {
				return invalid("invalid json flag")
			}
			o.machine = !equal || value == "true"
			continue
		}
		if name != "--type" && name != "--literal" && name != "--regex" && name != "--limit" && name != "--cursor" {
			return invalid("unknown search flag or positional argument")
		}
		if !equal {
			i++
			if i >= len(args) {
				return invalid("search flag requires a value")
			}
			value = args[i]
		}
		switch name {
		case "--type":
			if !artifactOperationType.MatchString(value) || strings.HasPrefix(value, "hn/") || strings.HasPrefix(value, "hubnot/") {
				return invalid("requires a valid custom type ID")
			}
			o.typeID = value
		case "--literal", "--regex":
			if o.mode != "" {
				return invalid("requires exactly one of --literal or --regex")
			}
			o.mode, o.text = strings.TrimPrefix(name, "--"), value
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
			if value == "" {
				return artifactListOutput(nil, artifactListCheckpoint(), o.machine)
			}
			if len(value) > 4096 {
				return artifactOutput(nil, artifactFailure("resource_limit", "cursor exceeds 4096 bytes"), o.machine)
			}
			o.cursor = value
		}
	}
	if o.mode == "" || o.limit == 0 {
		return invalid("requires exactly one of --literal or --regex and --limit 1..200")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, err := readArtifactSearch(ctx, o)
	if err != nil {
		return artifactListOutput(nil, err, o.machine)
	}
	if _, err = artifactSearchResponse(page); err != nil {
		return artifactOutput(nil, err, o.machine)
	}
	if ctx.Err() != nil {
		return artifactOutput(nil, artifactFailure("cancelled", "artifact search cancelled"), o.machine)
	}
	// The existing artifact renderer JSON-escapes every untrusted string in both
	// modes. Matching text remains complete inert data, including terminal escapes.
	return artifactOutput(page, nil, o.machine)
}

// Reuse the existing bounded Go-regexp compiler, including its pre-expansion
// check. Only its error adapter differs; memory's command and wire stay intact.
func compileArtifactSearchPattern(text string) (memoryGrepPattern, error) {
	if !utf8.ValidString(text) {
		return memoryGrepPattern{}, artifactFailure("invalid_input", "regex requires valid UTF-8")
	}
	if len(text) > 4096 {
		return memoryGrepPattern{}, artifactFailure("resource_limit", "regex exceeds 4096 UTF-8 pattern bytes")
	}
	p, err := compileMemoryGrepPattern(text, false)
	if err != nil {
		code := "invalid_input"
		// The shared compiler distinguishes bounded expansion/compilation from
		// syntax errors in these messages; do not parse or expand a second time.
		if strings.HasPrefix(err.Error(), "grep pattern exceeds ") {
			code = "resource_limit"
		}
		return memoryGrepPattern{}, artifactFailure(code, "artifact regex: "+err.Error())
	}
	return p, nil
}

// Unlike memory's independently matched-field allowance, the artifact contract
// charges exactly instructions times selected searchable UTF-8 bytes. Divide
// before multiplying so neither the multiplication nor accumulation can wrap.
func artifactSearchAddWork(used uint64, instructions, size int) (uint64, error) {
	const bound uint64 = 67108864
	if used > bound || instructions <= 0 || size < 0 || uint64(size) > (bound-used)/uint64(instructions) {
		return 0, artifactFailure("resource_limit", "artifact regex exceeds 67108864 selection instruction-bytes; narrow --type or simplify the regex")
	}
	return used + uint64(instructions)*uint64(size), nil
}

// Search only public identity metadata and every current head's exact text and
// canonical structured fields. Superseded revisions are available via history,
// and a conflict never chooses a head. Independent fields keep regex anchors local.
func artifactSearchFields(v artifactRecordView) ([]string, error) {
	fields := []string{v.ID, v.TypeID, v.Shape}
	for _, head := range v.Heads {
		raw, err := json.Marshal(head.Payload["fields"])
		if err != nil {
			return nil, artifactFailure("invalid_history", "cannot encode verified structured fields")
		}
		fields = append(fields, head.ID, head.Actor, head.TypeRevision, artifactText(head.Payload, "title"), artifactText(head.Payload, "content"), string(raw))
	}
	return fields, nil
}

func artifactSearchMatches(fields []string, text string, regex *memoryGrepPattern) bool {
	for _, field := range fields {
		if regex == nil && strings.Contains(field, text) || regex != nil && regex.expression.MatchString(field) {
			return true
		}
	}
	return false
}

// Extend the existing list continuation with a bounded query fingerprint, so a
// 4096-byte regex still produces a cursor well below the 4096-byte token cap.
// Page size may change; filters, mode, exact pattern, source cut and offset may not.
type artifactSearchCursor struct {
	artifactListCursor
	Query string `json:"query"`
}

func readArtifactSearch(ctx context.Context, o artifactSearchOptions) (artifactListPage, error) {
	page := artifactListPage{Items: []artifactRecordView{}}
	if ctx.Err() != nil {
		return page, artifactFailure("cancelled", "artifact search cancelled")
	}
	if !utf8.ValidString(o.text) || o.mode != "literal" && o.mode != "regex" || o.limit < 1 || o.limit > 200 {
		return page, artifactFailure("invalid_input", "invalid artifact search request")
	}
	var regex *memoryGrepPattern
	if o.mode == "regex" {
		p, err := compileArtifactSearchPattern(o.text)
		if err != nil {
			return page, err
		}
		regex = &p
	}
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
		if o.typeID == "" || artifactText(artifactPayload(h.ByID[id]), "type_id") == o.typeID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	queryBytes, err := json.Marshal([]string{"hn-artifact-search/1", o.typeID, o.mode, o.text})
	if err != nil {
		return page, err
	}
	query := eventID(queryBytes)
	start := 0
	if o.cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(o.cursor)
		var token artifactSearchCursor
		if e != nil || len(o.cursor) > 4096 || json.Unmarshal(raw, &token) != nil {
			return page, artifactListCheckpoint()
		}
		canonical, e := json.Marshal(token)
		if e != nil || !bytes.Equal(raw, canonical) || base64.RawURLEncoding.EncodeToString(raw) != o.cursor || token.Version != 1 || token.TypeID != o.typeID || token.Shape != "" || token.Query != query || token.Cut != h.Cut || token.Offset <= 0 || token.Offset >= len(ids) {
			return page, artifactListCheckpoint()
		}
		start = token.Offset
	}
	// Preflight the ENTIRE filtered selection before matching or paging. A late
	// expensive non-match cannot turn an otherwise valid first page into success.
	work := uint64(0)
	if regex != nil {
		for _, id := range ids {
			if ctx.Err() != nil {
				return page, artifactFailure("cancelled", "artifact search cancelled")
			}
			v := artifactRecordView{ID: id, TypeID: artifactText(artifactPayload(h.ByID[id]), "type_id"), Shape: artifactText(artifactPayload(h.ByID[id]), "shape")}
			for _, head := range c.heads[id] {
				s := h.ByID[head]
				v.Heads = append(v.Heads, artifactRecordHead{ID: head, Actor: s.verified.Actor(), TypeRevision: artifactText(artifactPayload(s), "type_revision"), Payload: artifactPayload(s)})
			}
			fields, e := artifactSearchFields(v)
			if e != nil {
				return page, e
			}
			for _, field := range fields {
				work, err = artifactSearchAddWork(work, regex.instructions, len(field))
				if err != nil {
					return page, err
				}
			}
		}
	}
	end := start
	for end < len(ids) {
		if ctx.Err() != nil {
			return artifactListPage{}, artifactFailure("cancelled", "artifact search cancelled")
		}
		view, e := c.view(ctx, ids[end])
		if e != nil {
			return artifactListPage{}, e
		}
		fields, e := artifactSearchFields(view)
		if e != nil {
			return artifactListPage{}, e
		}
		matched := artifactSearchMatches(fields, o.text, regex)
		// Scan past the last page match to avoid a cursor claiming more matches
		// when the rest of the verified selection is entirely non-matching.
		if matched && len(page.Items) == o.limit {
			break
		}
		end++
		if matched {
			page.Items = append(page.Items, view)
		}
	}
	after, err := artifactSources(ctx, h.gitDir)
	if err != nil {
		return artifactListPage{}, err
	}
	if !slices.Equal(h.Sources, after) {
		if o.cursor != "" {
			return artifactListPage{}, artifactListCheckpoint()
		}
		return artifactListPage{}, artifactFailure("concurrent_update", "artifact sources changed while searching")
	}
	if ctx.Err() != nil {
		return artifactListPage{}, artifactFailure("cancelled", "artifact search cancelled")
	}
	page.SourceCut = h.Cut
	if end < len(ids) {
		raw, e := json.Marshal(artifactSearchCursor{artifactListCursor{Version: 1, TypeID: o.typeID, Cut: h.Cut, Offset: end}, query})
		if e != nil {
			return artifactListPage{}, e
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &token
	}
	return page, nil
}

func artifactSearchResponse(page artifactListPage) ([]byte, error) {
	raw, err := json.Marshal(map[string]any{"ok": true, "data": page, "error": nil})
	if err != nil {
		return nil, err
	}
	if len(raw)+1 > 8388608 {
		return nil, artifactFailure("resource_limit", "artifact search response exceeds 8 MiB; reduce --limit or use explicit export for an oversized full state")
	}
	return append(raw, '\n'), nil
}
