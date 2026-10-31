package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// These schemas are owned by this example. They are not Spec Kitty schemas and
// neither their names nor their contents confer namespace or policy authority.
const namespace = "example.charter/"

func stringField() map[string]any { return map[string]any{"type": "string", "maxBytes": 64} }
func enumField(values ...string) map[string]any {
	longest := 0
	for _, v := range values {
		if len(v) > longest {
			longest = len(v)
		}
	}
	return map[string]any{"type": "string", "maxBytes": longest, "enum": values}
}
func declaration(name, shape string, version int) map[string]any {
	properties := map[string]any{}
	required := []string{}
	add := func(name string, node map[string]any) { properties[name] = node; required = append(required, name) }
	switch name {
	case "directive":
		add("code", stringField())
		add("severity", enumField("must", "should"))
		if version == 2 {
			add("rationale", stringField())
		}
	case "tactic":
		add("code", stringField())
		add("goal", stringField())
	case "pack":
		add("label", stringField())
	case "charter":
		add("project", stringField())
		add("status", enumField("selected"))
	}
	return map[string]any{"type_id": namespace + name, "shape": shape, "schema_version": version, "schema": map[string]any{"language": "hn.schema/1", "root": map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}}}
}
func record(name, shape, typ, title, text string, fields map[string]any) map[string]any {
	return map[string]any{"shape": shape, "type_id": namespace + name, "type_revision": typ, "title": title, "content": text, "fields": fields, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
}
func relationship(relation, id string) map[string]any {
	return map[string]any{"relation": namespace + relation, "target": id, "target_mode": "revision", "required": true}
}
func member(label, id string) map[string]any {
	return map[string]any{"member_id": label, "target": id, "target_mode": "revision", "required": true}
}

type selectionState struct {
	Protocol string                    `json:"protocol"`
	Actor    string                    `json:"actor"`
	IDs      map[string]string         `json:"ids"`
	Payloads map[string]map[string]any `json:"payloads"`
}

func loadState(path string) (selectionState, error) {
	var s selectionState
	f, e := os.Open(path)
	if e != nil {
		return s, e
	}
	defer f.Close()
	v, e := readInput(f)
	if e != nil {
		return s, e
	}
	raw, e := json.Marshal(v)
	if e != nil {
		return s, e
	}
	if e = strict(raw, &s); e != nil {
		return s, e
	}
	if s.Protocol != "example.charter-selection/1" || len(s.IDs) != 8 || len(s.Payloads) != 4 {
		return s, errors.New("invalid example selection state")
	}
	for _, id := range s.IDs {
		if !digestPattern.MatchString(id) {
			return s, errors.New("invalid selected identity")
		}
	}
	return s, nil
}
func actor(c *client) (string, error) {
	data, e := c.require("identity.public", map[string]any{})
	if e != nil {
		return "", e
	}
	a, ok := data["actor"].(string)
	if data["initialized"] != true || !ok || !sessionActor.MatchString(a) {
		return "", errors.New("initialize a repository identity explicitly before publishing")
	}
	return a, nil
}
func event(c *client, method string, params map[string]any) (string, error) {
	data, e := c.require(method, params)
	if e != nil {
		return "", e
	}
	id, ok := data["event_id"].(string)
	if !ok || !digestPattern.MatchString(id) {
		return "", errors.New("invalid mutation identity")
	}
	return id, nil
}
func publish(c *client, path, text string) (map[string]any, error) {
	a, e := actor(c)
	if e != nil {
		return nil, e
	}
	s := selectionState{Protocol: "example.charter-selection/1", Actor: a, IDs: map[string]string{}, Payloads: map[string]map[string]any{}}
	for _, model := range []struct{ name, shape string }{{"directive", "document"}, {"tactic", "document"}, {"pack", "list"}, {"charter", "ticket"}} {
		id, e := event(c, "artifact.type.publish", map[string]any{"parents": []string{}, "payload": declaration(model.name, model.shape, 1), "actor": a, "operation": "example-charter-type-" + model.name + "-1"})
		if e != nil {
			return nil, e
		}
		s.IDs[model.name+"_type"] = id
	}
	if text == "" {
		text = "# Preserve review evidence\nKeep exact input revisions with each decision.\n雪 e\u0301\r\n$(echo stored-text-is-not-a-command)"
	}
	s.Payloads["directive"] = record("directive", "document", s.IDs["directive_type"], "Evidence integrity directive", text, map[string]any{"code": "EVIDENCE-01", "severity": "must"})
	s.Payloads["tactic"] = record("tactic", "document", s.IDs["tactic_type"], "Inspect evidence before approving", "Compare the proposed inputs to the stored signed revisions.", map[string]any{"code": "COMPARE-01", "goal": "Make each decision independently checkable"})
	for _, name := range []string{"directive", "tactic", "pack", "charter"} {
		switch name {
		case "tactic":
			s.Payloads[name]["relationships"] = []any{relationship("implements", s.IDs["directive"])}
		case "pack":
			p := record(name, "list", s.IDs["pack_type"], "Evidence review pack", "Order is explicit: directive first, tactic second.", map[string]any{"label": "Evidence review starter"})
			p["members"] = []any{member("z-directive", s.IDs["directive"]), member("a-tactic", s.IDs["tactic"])}
			p["relationships"] = []any{relationship("includes", s.IDs["directive"]), relationship("includes", s.IDs["tactic"])}
			s.Payloads[name] = p
		case "charter":
			p := record(name, "ticket", s.IDs["charter_type"], "Example project selection", "A demonstration selection of three exact signed revisions; no activation.", map[string]any{"project": "Evidence demonstration", "status": "selected"})
			p["relationships"] = []any{relationship("selects-directive", s.IDs["directive"]), relationship("selects-tactic", s.IDs["tactic"]), relationship("selects-pack", s.IDs["pack"])}
			s.Payloads[name] = p
		}
		id, e := event(c, "artifact.mutate", map[string]any{"kind": "record.open", "record": nil, "parents": []string{}, "payload": s.Payloads[name], "actor": a, "operation": "example-charter-open-" + name + "-1"})
		if e != nil {
			return nil, e
		}
		s.IDs[name] = id
	}
	if e = save(path, s); e != nil {
		return nil, e
	}
	return map[string]any{"state": s, "generic_validation": "host-verified signed schema data", "semantic_interpretation": "demonstration exact selection only; no authority"}, nil
}
func evolve(c *client, path, parent string) (map[string]any, error) {
	s, e := loadState(path)
	if e != nil {
		return nil, e
	}
	if !digestPattern.MatchString(parent) {
		return nil, errors.New("evolution requires an explicitly selected exact parent")
	}
	a, e := actor(c)
	if e != nil {
		return nil, e
	}
	if a != s.Actor {
		return nil, errors.New("state actor differs; choose a new explicit operation instead")
	}
	id, e := event(c, "artifact.type.publish", map[string]any{"parents": []string{s.IDs["directive_type"]}, "payload": declaration("directive", "document", 2), "actor": a, "operation": "example-charter-directive-type-2"})
	if e != nil {
		return nil, e
	}
	p := record("directive", "document", id, "Evidence integrity directive version two", "Converted current evidence guidance; the charter still selects the original.", map[string]any{"code": "EVIDENCE-01", "severity": "should", "rationale": "Keep historical selections reproducible"})
	p["conversion_sources"] = []string{parent}
	converted, e := event(c, "artifact.mutate", map[string]any{"kind": "record.convert", "record": s.IDs["directive"], "parents": []string{parent}, "payload": p, "actor": a, "operation": "example-charter-directive-convert-2"})
	if e != nil {
		return nil, e
	}
	// Deliberately do not rewrite the stored selection to a latest view.
	return map[string]any{"new_type": id, "converted": converted, "payload": p, "stored_selection": s.IDs, "semantic_interpretation": "demonstration conversion only; selection unchanged"}, nil
}
func exactPayload(c *client, id, typ string) (map[string]any, error) {
	data, e := c.require("artifact.read", map[string]any{"id": id, "opaque_id": nil, "type_id": typ, "limit": 200, "cursor": nil, "mode": "inspect", "opaque": false})
	if e != nil {
		return nil, e
	}
	items, ok := data["items"].([]any)
	if !ok || data["next_cursor"] != nil {
		return nil, errors.New("incomplete exact inspect result")
	}
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("invalid signed artifact")
		}
		if row["id"] != id {
			continue
		}
		if row["validation"] != "verified" {
			return nil, errors.New("selected artifact is not generically verified")
		}
		envelope, ok := row["envelope"].(map[string]any)
		if !ok {
			return nil, errors.New("missing selected envelope")
		}
		p, ok := envelope["payload"].(map[string]any)
		if !ok {
			return nil, errors.New("missing selected payload")
		}
		return p, nil
	}
	return nil, fmt.Errorf("exact selected revision absent: %s", id)
}
func inspectSelection(c *client, path string) (map[string]any, error) {
	s, e := loadState(path)
	if e != nil {
		return nil, e
	}
	// Interpret the public ticket, not a cached/latest payload. The local state
	// supplies only the exact ticket identity; links and order come from signed data.
	ticket, e := exactPayload(c, s.IDs["charter"], namespace+"charter")
	if e != nil {
		return nil, e
	}
	links, ok := ticket["relationships"].([]any)
	if !ok || len(links) != 3 {
		return map[string]any{"generic_validation": "host-verified signed schema data", "semantic_interpretation": "rejected: example ticket requires three selections"}, nil
	}
	selected := map[string]string{}
	for _, v := range links {
		link, ok := v.(map[string]any)
		if !ok || link["target_mode"] != "revision" || link["required"] != true {
			return nil, errors.New("example selection requires exact required links")
		}
		relation, _ := link["relation"].(string)
		target, _ := link["target"].(string)
		if !digestPattern.MatchString(target) {
			return nil, errors.New("invalid selected target")
		}
		if _, exists := selected[relation]; exists {
			return nil, errors.New("duplicate example selection relation")
		}
		selected[relation] = target
	}
	for _, relation := range []string{"selects-directive", "selects-tactic", "selects-pack"} {
		if selected[namespace+relation] == "" {
			return map[string]any{"generic_validation": "host-verified signed schema data", "semantic_interpretation": "rejected: example ticket requires named directive, tactic and pack selections"}, nil
		}
	}
	doc, e := exactPayload(c, selected[namespace+"selects-directive"], namespace+"directive")
	if e != nil {
		return nil, e
	}
	tactic, e := exactPayload(c, selected[namespace+"selects-tactic"], namespace+"tactic")
	if e != nil {
		return nil, e
	}
	pack, e := exactPayload(c, selected[namespace+"selects-pack"], namespace+"pack")
	if e != nil {
		return nil, e
	}
	members, ok := pack["members"].([]any)
	if !ok || len(members) != 2 {
		return map[string]any{"generic_validation": "host-verified signed schema data", "semantic_interpretation": "rejected: example pack requires two ordered members"}, nil
	}
	for i, want := range []struct{ label, target string }{{"z-directive", selected[namespace+"selects-directive"]}, {"a-tactic", selected[namespace+"selects-tactic"]}} {
		m, ok := members[i].(map[string]any)
		if !ok || m["member_id"] != want.label || m["target"] != want.target || m["target_mode"] != "revision" || m["required"] != true {
			return map[string]any{"generic_validation": "host-verified signed schema data", "semantic_interpretation": "rejected: example pack order or exact target differs from ticket"}, nil
		}
	}
	return map[string]any{"generic_validation": "host-verified signed schema data", "semantic_interpretation": "demonstration exact selection only; no authority", "selected_directive": selected[namespace+"selects-directive"], "selected_text": doc["content"], "selected_directive_payload": doc, "selected_tactic_payload": tactic, "selected_pack_payload": pack}, nil
}
