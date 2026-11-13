package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
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

type catalogFixtureSource struct {
	config migration.ProviderConfig
}

func (s catalogFixtureSource) Capabilities(context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "project", Read: true, Preserve: true}}, nil
}

func (s catalogFixtureSource) Snapshot(context.Context, migration.Selection, migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	return migration.Snapshot{
		Schema: "hn.project-snapshot/1", Source: s.config.Locator, SourceID: "catalog-fixture",
		Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{},
	}, nil, nil
}

func TestMigrationCatalogCustomSourceCanPlanAndApply(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repository")
	if output, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	const kind = "test_catalog_custom"
	err := migrationProviders.Register(migration.ProviderRegistration{
		Kind: kind, Destinations: []string{"local"},
		SourceFields: []migration.ProviderField{{Name: "format", Label: "Format", Required: true, Choices: []string{"native"}}},
		NewSource: func(c migration.ProviderConfig) (migration.Source, error) {
			if c.Options["format"] != "native" {
				return nil, fmt.Errorf("custom source did not receive the configuration")
			}
			return catalogFixtureSource{config: c}, nil
		},
		NewDestination: func(migration.ProviderConfig) (migration.Destination, error) {
			return LocalMigrationDestination{Repository: repo}, nil
		},
		RequiredCoverage: []migration.CoverageRequirement{{Kind: "project", Read: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session := filepath.Join(root, "session")
	args := []string{"plan", "--session", session, "--source-kind", kind, "--source-repository", "fixture",
		"--source-option", "format=native", "--kind", "project", "--destination-kind", "local", "--destination-repository", repo,
		"--decision", "local/governance=preserve"}
	if err = runMigrationCLI(ctx, args, strings.NewReader(""), io.Discard); err != nil {
		t.Fatal(err)
	}
	if err = runMigrationCLI(ctx, []string{"apply", "--session", session, "--confirm-apply"}, strings.NewReader(""), io.Discard); err != nil {
		t.Fatal(err)
	}
	imports, err := ListLocalMigrations(filepath.Join(repo, ".git"), migration.DefaultLimits())
	if err != nil || len(imports) != 1 {
		t.Fatalf("local imports: %v, %v", imports, err)
	}
}
