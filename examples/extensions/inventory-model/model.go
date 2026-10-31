package main

import "fmt"

func (c Command) request(actor string) (string, map[string]any, error) {
	shape := map[string]string{"item": "document", "stock": "list", "maintenance": "ticket"}[c.Model]
	if shape == "" {
		return "", nil, fmt.Errorf("unknown inventory model %q", c.Model)
	}
	typeID := "example.inventory/" + c.Model
	parents := c.Parents
	if parents == nil {
		parents = []string{}
	}
	switch c.Action {
	case "type":
		if c.SchemaVersion != 1 && c.SchemaVersion != 2 {
			return "", nil, fmt.Errorf("inventory schema version must be 1 or 2")
		}
		properties, required := inventorySchema(c.Model)
		if c.SchemaVersion == 2 {
			properties["unit"] = map[string]any{"type": "string", "maxBytes": 64}
			required = append(required, "unit")
		}
		payload := map[string]any{"type_id": typeID, "schema_version": c.SchemaVersion, "shape": shape, "schema": map[string]any{"language": "hn.schema/1", "root": map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}}}
		return "artifact.type.publish", map[string]any{"parents": parents, "payload": payload, "actor": actor, "operation": c.Operation}, nil
	case "open", "revise", "resolve", "convert":
		fields := c.Fields
		if fields == nil {
			fields = map[string]any{}
		}
		members := c.Members
		if members == nil {
			members = []map[string]any{}
		}
		links := c.Relationships
		if links == nil {
			links = []map[string]any{}
		}
		sources := c.ConversionSources
		if sources == nil {
			sources = []string{}
		}
		payload := map[string]any{"type_id": typeID, "type_revision": c.TypeRevision, "shape": shape, "title": c.Title, "content": c.Content, "fields": fields, "members": members, "relationships": links, "conversion_sources": sources}
		return "artifact.mutate", map[string]any{"kind": "record." + c.Action, "record": c.Record, "parents": parents, "payload": payload, "actor": actor, "operation": c.Operation}, nil

	}
	return "", nil, fmt.Errorf("unknown inventory action %q", c.Action)
}
func inventorySchema(model string) (map[string]any, []string) {
	text := func() map[string]any { return map[string]any{"type": "string", "maxBytes": 64} }
	switch model {
	case "item":
		return map[string]any{"sku": text(), "quantity": map[string]any{"type": "integer", "minimum": 0}, "location": text()}, []string{"sku", "quantity", "location"}
	case "stock":
		return map[string]any{"warehouse": text(), "counted": map[string]any{"type": "boolean"}}, []string{"warehouse", "counted"}
	case "maintenance":
		return map[string]any{"priority": map[string]any{"type": "string", "maxBytes": 7, "enum": []string{"routine", "urgent"}}, "completed": map[string]any{"type": "boolean"}}, []string{"priority", "completed"}
	}
	panic("validated model missing")
}
