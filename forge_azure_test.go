//go:build !noforge

package main

import (
	"strings"
	"testing"
)

func TestAzureLegacyCapabilitiesExplainMappedMigrationBoundary(t *testing.T) {
	r, ok := forgeProviders["azure"]
	if !ok {
		t.Fatal("Azure missing from legacy discovery")
	}
	if r.TokenEnv != "" {
		t.Fatal("legacy Azure must not discover ambient credentials")
	}
	if len(r.Capabilities) != 4 {
		t.Fatal("missing legacy resource coverage")
	}
	for _, c := range r.Capabilities {
		if c.Import || c.Export || !strings.Contains(c.Reason, "process/type/state") {
			t.Fatal("legacy surface invents Azure equivalence", c)
		}
	}
	for _, direction := range []string{"import", "export"} {
		if e := runForgeTransfer(transferOptions{command: direction, provider: "azure", project: "org/P", endpoint: "https://must-not-contact.invalid"}); e == nil || !strings.Contains(e.Error(), "no implemented provider mapping") {
			t.Fatal("missing pre-network refusal", e)
		}
	}
}
