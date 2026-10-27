package migration

import "testing"

func TestProviderCoverageRejectsVacuousAndDishonestProfiles(t *testing.T) {
	required := []CoverageRequirement{{Kind: "issue", Read: true, Preserve: true, Create: true}, {Kind: "comment", Read: true, Preserve: true, Create: true}}
	source := []Capability{{Kind: "issue", Read: true, Preserve: true}, {Kind: "comment", Read: true, Preserve: true}}
	dest := []Capability{{Kind: "issue", Create: true}, {Kind: "comment", Create: true}}
	if e := ValidateProviderCoverage(required, source, dest); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name     string
		required []CoverageRequirement
		s, d     []Capability
	}{{"unsupported", required, []Capability{{Kind: "issue", Reason: "unavailable"}}, nil}, {"missing comment write", required, source, dest[:1]}, {"fake preservation", required, []Capability{{Kind: "issue", Preserve: true}}, dest}, {"duplicate", required, append(source, source[0]), dest}, {"empty baseline", nil, source, dest}, {"vacuous baseline", []CoverageRequirement{{Kind: "issue"}}, source, dest}} {
		t.Run(tc.name, func(t *testing.T) {
			if e := ValidateProviderCoverage(tc.required, tc.s, tc.d); e == nil {
				t.Fatal("invalid coverage accepted")
			}
		})
	}
}
