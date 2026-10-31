package main

import (
	"context"
	"sort"
	"strings"
)

// A closure contains only verified public actor prefixes. Its ref tips can be
// resolved in an isolated Git repository by the same reader before promotion;
// neither selection nor verification has access to an intent or signing key.
// Items retain previous/parents/type_revision/conversion_sources and link edges;
// their commit/parentCommit metadata and Sources give transport callers the exact
// ancestry graph without another signature/schema/lineage implementation.
// This producer uses read/catalog bounds. Exchange wire/decoded/object/record/
// ancestry limits belong to the live exchange consumer before its promotion.
type artifactClosure struct {
	IDs     []string
	Sources []artifactSource
	Items   []*storedArtifact
	Objects []string
}

type artifactClosureQueue struct {
	ids  []string
	seen map[string]bool
}

func (q *artifactClosureQueue) add(ctx context.Context, id string) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact closure cancelled")
	}
	if !validMemoryDigestID(id) {
		return artifactFailure("invalid_input", "closure requires full artifact IDs")
	}
	if q.seen[id] {
		return nil
	}
	if len(q.seen) >= artifactReadEntries {
		return artifactFailure("resource_limit", "closure exceeds selected artifact read limit")
	}
	q.seen[id] = true
	q.ids = append(q.ids, id)
	return nil
}

func selectArtifactClosure(ctx context.Context, h *artifactHistory, roots []string) (*artifactClosure, error) {
	if len(roots) == 0 {
		return nil, artifactFailure("invalid_input", "requires an explicit artifact selection")
	}
	catalog, err := buildArtifactCatalog(ctx, h)
	if err != nil {
		return nil, err
	}
	queue := artifactClosureQueue{ids: []string{}, seen: map[string]bool{}}
	enqueue := func(id string) error {
		if ctx.Err() != nil {
			return artifactFailure("cancelled", "artifact closure cancelled")
		}
		if !validMemoryDigestID(id) {
			return artifactFailure("invalid_input", "closure requires full artifact IDs")
		}
		if h.ByID[id] == nil {
			return artifactFailure("incomplete_dependency", "required closure artifact missing: "+id)
		}
		return queue.add(ctx, id)
	}
	for _, id := range roots {
		if !validMemoryDigestID(id) {
			return nil, artifactFailure("invalid_input", "closure requires full artifact IDs")
		}
		if h.ByID[id] == nil {
			return nil, artifactFailure("not_found", "artifact not found in selected accepted history")
		}
		if err = enqueue(id); err != nil {
			return nil, err
		}
	}
	diagnostics := map[string][]any{}
	for offset := 0; offset < len(queue.ids); offset++ {
		id := queue.ids[offset]
		s := h.ByID[id]
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact closure cancelled")
		}
		dependencies := artifactIDs(s.Envelope["parents"])
		for _, field := range []string{"previous", "record"} {
			if dependency := artifactText(s.Envelope, field); dependency != "" {
				dependencies = append(dependencies, dependency)
			}
		}
		p := artifactPayload(s)
		if s.verified.Kind() != "type.publish" {
			dependencies = append(dependencies, artifactText(p, "type_revision"))
			dependencies = append(dependencies, artifactIDs(p["conversion_sources"])...)
		}
		for _, dependency := range dependencies {
			if err = enqueue(dependency); err != nil {
				return nil, err
			}
		}
		// Diagnostics remain on the artifact containing the link. Optional links
		// retain their signed target IDs without expanding the selected set.
		noted := map[string]bool{}
		notes := []artifactDiagnostic{}
		note := func(code, target, message string) {
			key := code + "\x00" + target + "\x00" + message
			if !noted[key] {
				noted[key] = true
				copyID := target
				notes = append(notes, artifactDiagnostic{code, &copyID, message})
			}
		}
		for _, field := range []string{"members", "relationships"} {
			links, _ := p[field].([]any)
			for _, value := range links {
				if ctx.Err() != nil {
					return nil, artifactFailure("cancelled", "artifact closure cancelled")
				}
				link := value.(map[string]any)
				targetID, mode := artifactText(link, "target"), artifactText(link, "target_mode")
				target := h.ByID[targetID]
				if target == nil {
					if link["required"] == true {
						return nil, artifactFailure("incomplete_dependency", "required "+mode+" target missing: "+targetID)
					}
					note("missing_optional", targetID, "optional "+mode+" target missing")
					continue
				}
				if !strings.HasPrefix(target.verified.Kind(), "record.") || mode == "logical" && target.verified.Kind() != "record.open" {
					return nil, artifactFailure("invalid_history", "closure target has wrong record lineage")
				}
				if mode == "logical" && len(catalog.heads[targetID]) > 1 {
					note("target_conflict", targetID, "logical target has multiple current heads")
				}
				if link["required"] != true {
					continue
				}
				if err = enqueue(targetID); err != nil {
					return nil, err
				}
				if mode == "logical" {
					for _, head := range catalog.heads[targetID] {
						if err = enqueue(head); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		sort.Slice(notes, func(i, j int) bool {
			a, b := notes[i], notes[j]
			if a.Code != b.Code {
				return a.Code < b.Code
			}
			if *a.ID != *b.ID {
				return *a.ID < *b.ID
			}
			return a.Message < b.Message
		})
		diagnostics[id] = []any{}
		for _, note := range notes {
			diagnostics[id] = append(diagnostics[id], note)
		}
	}
	result := &artifactClosure{IDs: queue.ids, Sources: []artifactSource{}, Items: []*storedArtifact{}}
	sort.Strings(result.IDs)
	for _, id := range result.IDs {
		copyArtifact := *h.ByID[id]
		copyArtifact.Diagnostics = diagnostics[id]
		result.Items = append(result.Items, &copyArtifact)
	}
	for actor, chain := range h.Chains {
		var tip *storedArtifact
		for _, s := range chain {
			if ctx.Err() != nil {
				return nil, artifactFailure("cancelled", "artifact closure cancelled")
			}
			if queue.seen[s.ID] {
				tip = s
			}
		}
		if tip != nil {
			result.Sources = append(result.Sources, artifactSource{artifactRef(actor), tip.commit})
		}
	}
	sort.Slice(result.Sources, func(i, j int) bool { return result.Sources[i].Ref < result.Sources[j].Ref })
	// Resolve precisely the selected commit/tree/blob inventory through the real
	// hash/signature reader. Do not enumerate arbitrary ODB objects or copy the
	// source's unselected tails, working tree or private descriptors.
	reader := artifactObjectReader{ctx: ctx, dir: h.gitDir, seen: map[string]bool{}}
	for _, item := range result.Items {
		verified, _, e := reader.commit(item.commit)
		if e != nil {
			return nil, e
		}
		if verified.ID != item.ID {
			return nil, artifactFailure("verification_failure", "selected closure identity changed")
		}
	}
	result.Objects = make([]string, 0, len(reader.seen))
	for oid := range reader.seen {
		result.Objects = append(result.Objects, oid)
	}
	sort.Strings(result.Objects)
	return result, nil
}
