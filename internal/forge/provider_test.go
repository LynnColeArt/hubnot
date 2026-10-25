package forge

import (
	"encoding/json"
	"testing"
)

func TestExplicitResourceCapabilityReport(t *testing.T) {
	resources := Resources()
	if len(resources) != 4 {
		t.Fatal(resources)
	}
	for _, r := range resources {
		if !KnownResource(r) {
			t.Fatal(r)
		}
	}
	if KnownResource("everything") {
		t.Fatal("unknown resource accepted")
	}
	report := MappingReport{Provider: "example", Capabilities: []Capability{{Resource: ResourceIssue, Import: true, Export: true, MappedFields: []string{"title", "body"}, Unsupported: []string{"workflow execution"}}, {Resource: ResourceWorkflow, Reason: "explicit mapping required", MappedFields: []string{}, Unsupported: []string{"commands", "secrets", "trust"}}}}
	raw, err := json.Marshal(report)
	if err != nil || !json.Valid(raw) {
		t.Fatalf("report: %s %v", raw, err)
	}
}
