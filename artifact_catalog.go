package main

import (
	"context"
	"sort"
	"strings"
)

type artifactDiagnostic struct {
	Code    string  `json:"code"`
	ID      *string `json:"id"`
	Message string  `json:"message"`
}
type artifactRecordHead struct {
	ID           string         `json:"id"`
	Actor        string         `json:"actor"`
	TypeRevision string         `json:"type_revision"`
	Payload      map[string]any `json:"payload"`
}
type artifactRecordView struct {
	Protocol    string               `json:"protocol"`
	ID          string               `json:"id"`
	TypeID      string               `json:"type_id"`
	Shape       string               `json:"shape"`
	Heads       []artifactRecordHead `json:"heads"`
	Conflict    bool                 `json:"conflict"`
	State       map[string]any       `json:"state"`
	Diagnostics []artifactDiagnostic `json:"diagnostics"`
}

// A catalog belongs to one complete verified selected history and source cut.
// It indexes immutable records, never arrival order or timestamps.
type artifactCatalog struct {
	history *artifactHistory
	records map[string][]string
	heads   map[string][]string
}

func artifactRecordHeads(h *artifactHistory, opening string) []string {
	heads := map[string]bool{}
	for _, s := range h.ByID {
		if s.verified.Kind() == "record.open" && s.ID == opening || artifactText(s.Envelope, "record") == opening {
			heads[s.ID] = true
		}
	}
	for _, s := range h.ByID {
		if artifactText(s.Envelope, "record") == opening {
			for _, id := range artifactIDs(s.Envelope["parents"]) {
				delete(heads, id)
			}
		}
	}
	out := make([]string, 0, len(heads))
	for id := range heads {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
func buildArtifactCatalog(ctx context.Context, h *artifactHistory) (*artifactCatalog, error) {
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "catalog construction cancelled")
	}
	c := &artifactCatalog{history: h, records: map[string][]string{}, heads: map[string][]string{}}
	if len(h.ByID) > artifactReadEntries {
		return nil, artifactFailure("resource_limit", "catalog exceeds selected artifact read limit")
	}
	for _, s := range h.ByID {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "catalog construction cancelled")
		}
		if !strings.HasPrefix(s.verified.Kind(), "record.") {
			continue
		}
		logical := artifactText(s.Envelope, "record")
		if s.verified.Kind() == "record.open" {
			logical = s.ID
		}
		opening := h.ByID[logical]
		if opening == nil {
			return nil, artifactFailure("incomplete_dependency", "catalog logical opening missing")
		}
		if opening.verified.Kind() != "record.open" {
			return nil, artifactFailure("invalid_history", "catalog logical identity is not an opening")
		}
		payload := artifactPayload(s)
		initial := artifactPayload(opening)
		if artifactText(payload, "type_id") != artifactText(initial, "type_id") || artifactText(payload, "shape") != artifactText(initial, "shape") {
			return nil, artifactFailure("invalid_history", "catalog record changed logical type")
		}
		for _, id := range artifactIDs(s.Envelope["parents"]) {
			parent := h.ByID[id]
			if parent == nil {
				return nil, artifactFailure("incomplete_dependency", "catalog exact parent missing")
			}
			pl := artifactText(parent.Envelope, "record")
			if parent.verified.Kind() == "record.open" {
				pl = parent.ID
			}
			if !strings.HasPrefix(parent.verified.Kind(), "record.") || pl != logical {
				return nil, artifactFailure("invalid_history", "catalog parent belongs to another logical record")
			}
		}
		c.records[logical] = append(c.records[logical], s.ID)
	}
	// Index each edge once rather than scanning all histories for each record.
	active := map[string]bool{}
	for logical, revisions := range c.records {
		sort.Strings(revisions)
		c.heads[logical] = []string{}
		for _, id := range revisions {
			active[id] = true
		}
	}
	for _, s := range h.ByID {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "catalog construction cancelled")
		}
		for _, id := range artifactIDs(s.Envelope["parents"]) {
			delete(active, id)
		}
	}
	for logical, revisions := range c.records {
		for _, id := range revisions {
			if active[id] {
				c.heads[logical] = append(c.heads[logical], id)
			}
		}
		if len(c.heads[logical]) == 0 {
			return nil, artifactFailure("invalid_history", "record ancestry has no head")
		}
	}
	return c, nil
}
func (c *artifactCatalog) logical(id string) (string, error) {
	s := c.history.ByID[id]
	if s == nil || !strings.HasPrefix(s.verified.Kind(), "record.") {
		return "", artifactFailure("not_found", "record not found in selected accepted history")
	}
	if s.verified.Kind() == "record.open" {
		return id, nil
	}
	return artifactText(s.Envelope, "record"), nil
}
func (c *artifactCatalog) view(ctx context.Context, id string) (artifactRecordView, error) {
	var zero artifactRecordView
	logical, e := c.logical(id)
	if e != nil {
		return zero, e
	}
	opening := artifactPayload(c.history.ByID[logical])
	v := artifactRecordView{Protocol: "hn-artifact/1", ID: logical, TypeID: artifactText(opening, "type_id"), Shape: artifactText(opening, "shape"), Heads: []artifactRecordHead{}, Diagnostics: []artifactDiagnostic{}}
	for _, head := range c.heads[logical] {
		s := c.history.ByID[head]
		p := artifactPayload(s)
		v.Heads = append(v.Heads, artifactRecordHead{s.ID, s.verified.Actor(), artifactText(p, "type_revision"), p})
	}
	v.Conflict = len(v.Heads) > 1
	if len(v.Heads) == 1 {
		v.State = v.Heads[0].Payload
	}
	v.Diagnostics, e = c.diagnostics(ctx, c.heads[logical])
	if e != nil {
		return zero, e
	}
	return v, nil
}

// Required links traverse exact revisions or every current logical head. The
// visited set is shared across all roots/edges; cycles are inert valid data.
// Optional links are preserved and diagnosed but do not acquire closure edges.
func (c *artifactCatalog) diagnostics(ctx context.Context, roots []string) ([]artifactDiagnostic, error) {
	diagnostics := []artifactDiagnostic{}
	seen := map[string]bool{}
	noted := map[string]bool{}
	queue := []string{}
	enqueue := func(id string) error {
		if !seen[id] {
			if len(seen) >= artifactReadEntries {
				return artifactFailure("resource_limit", "linked catalog exceeds selected artifact read limit")
			}
			seen[id] = true
			queue = append(queue, id)
		}
		return nil
	}
	note := func(code, id, message string) {
		key := code + "\x00" + id + "\x00" + message
		if !noted[key] {
			noted[key] = true
			copyID := id
			diagnostics = append(diagnostics, artifactDiagnostic{code, &copyID, message})
		}
	}
	for _, id := range roots {
		if e := enqueue(id); e != nil {
			return nil, e
		}
	}
	for offset := 0; offset < len(queue); offset++ {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "linked catalog reconstruction cancelled")
		}
		s := c.history.ByID[queue[offset]]
		if s == nil {
			return nil, artifactFailure("incomplete_dependency", "exact traversal revision missing")
		}
		p := artifactPayload(s)
		for _, field := range []string{"members", "relationships"} {
			links, _ := p[field].([]any)
			for _, value := range links {
				if ctx.Err() != nil {
					return nil, artifactFailure("cancelled", "linked catalog reconstruction cancelled")
				}
				link := value.(map[string]any)
				id := artifactText(link, "target")
				mode := artifactText(link, "target_mode")
				target := c.history.ByID[id]
				if target == nil {
					if link["required"] == true {
						note("incomplete_dependency", id, "required "+mode+" target missing")
					} else {
						note("missing_optional", id, "optional "+mode+" target missing")
					}
					continue
				}
				if !strings.HasPrefix(target.verified.Kind(), "record.") || mode == "logical" && target.verified.Kind() != "record.open" {
					return nil, artifactFailure("invalid_history", "linked target has wrong record lineage")
				}
				if mode == "logical" && len(c.heads[id]) > 1 {
					note("target_conflict", id, "logical target has multiple current heads")
				}
				if link["required"] != true {
					continue
				}
				if mode == "logical" {
					if e := enqueue(id); e != nil {
						return nil, e
					}
					for _, head := range c.heads[id] {
						if e := enqueue(head); e != nil {
							return nil, e
						}
					}
				} else {
					if e := enqueue(id); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if *a.ID != *b.ID {
			return *a.ID < *b.ID
		}
		return a.Message < b.Message
	})
	return diagnostics, nil
}
