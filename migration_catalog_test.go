package main

import (
	"bytes"
	"testing"

	"hubnot/internal/migration"
)

func TestMigrationCatalogReflectsInstalledRegistry(t *testing.T) {
	var registry migration.Registry
	if err := registry.Register(migration.ProviderRegistration{
		Kind: "custom", Destinations: []string{"local"},
		SourceFields:     []migration.ProviderField{{Name: "format", Label: "Format", Required: true, Choices: []string{"native", "portable"}}},
		NewSource:        func(migration.ProviderConfig) (migration.Source, error) { panic("catalog contacted source") },
		NewDestination:   func(migration.ProviderConfig) (migration.Destination, error) { panic("catalog contacted destination") },
		RequiredCoverage: []migration.CoverageRequirement{{Kind: "issue", Read: true}},
	}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeMigrationRegistryCatalog(&output, &registry); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"format"`)) || !bytes.Contains(output.Bytes(), []byte(`"portable"`)) ||
		bytes.Contains(output.Bytes(), []byte(`"github"`)) || migrationRegistryPathAvailable(&registry, "custom", "archive") ||
		!migrationRegistryPathAvailable(&registry, "custom", "local") {
		t.Fatal(output.String())
	}
}
