package forge

import (
	"encoding/json"
	"testing"
)

func TestResourcePlanClassifications(t *testing.T) {
	p := Plan{Resource: ResourceProject, Direction: "import", Findings: []Finding{{Status: Mapped, Path: "title"}, {Status: Preserved, Path: "source"}}}
	if p.Blocked() {
		t.Fatal("supported plan blocked")
	}
	for _, status := range []FindingStatus{RequiresMapping, Unsupported, MissingDependency, IdentityConflict} {
		p.Findings = append(p.Findings, Finding{Status: status, Path: "ref"})
		if !p.Blocked() {
			t.Fatal(status)
		}
		p.Findings = p.Findings[:2]
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) == 0 {
		t.Fatal(err)
	}
}
