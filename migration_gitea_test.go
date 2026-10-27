//go:build !noforge

package main

import (
	"context"
	"hubnot/internal/migration"
	"testing"
)

func TestGiteaMigrationSharedGitAndOfflineImport(t *testing.T) {
	runV1GitAndOfflineImport(t, "gitea", "1.27.3")
}
func TestGiteaMigrationProjectionLossRefusesRecovery(t *testing.T) {
	runV1ProjectionLoss(t, "gitea", "1.27.3")
}

func TestGiteaMigrationEngineRecovery(t *testing.T) {
	for _, version := range []string{"1.24.7", "1.27.3"} {
		t.Run(version, func(t *testing.T) { runV1EngineRecovery(t, "gitea", version) })
	}
}
func TestGiteaMigrationSnapshotTopologyAssetsWorkflows(t *testing.T) {
	for _, version := range []string{"1.24.7", "1.27.3"} {
		t.Run(version, func(t *testing.T) { runV1Snapshot(t, "gitea", version) })
	}
}
func TestGiteaMigrationIdentityAndResourceGuards(t *testing.T) { runV1Guards(t, "gitea", "1.27.3") }
func TestGiteaMigrationUpdatesAndMappings(t *testing.T) {
	for _, version := range []string{"1.24.7", "1.27.3"} {
		t.Run(version, func(t *testing.T) { runV1UpdatesAndMappings(t, "gitea", version) })
	}
}
func TestGiteaMigrationBudgetsAndAvailability(t *testing.T) { runV1Limits(t, "gitea", "1.27.3") }

func TestV1MigrationRegistrationsOfflineAndDistinct(t *testing.T) {
	for _, kind := range []string{"forgejo", "gitea"} {
		c := migration.ProviderConfig{Locator: migration.Locator{Kind: kind, Endpoint: "https://offline.example/api/v1", Repository: "a/b"}}
		s, e := OpenMigrationSource(c)
		if e != nil {
			t.Fatal(e)
		}
		d, e := OpenMigrationDestination(c)
		if e != nil {
			t.Fatal(e)
		}
		sc, _ := s.Capabilities(context.Background())
		dc, _ := d.Capabilities(context.Background())
		p, ok := migrationProviders.Lookup(kind)
		if !ok || migration.ValidateProviderCoverage(p.RequiredCoverage, sc, dc) != nil {
			t.Fatal("vacuous registry profile", kind)
		}
		if d.Locator() != c.Locator {
			t.Fatal("identity changed", d.Locator())
		}
		for _, cap := range sc {
			if cap.Kind == "project" && (cap.Read || cap.Preserve || cap.Reason == "") {
				t.Fatal("board API invented")
			}
		}
		c.APIVersion = "99"
		if _, e = OpenMigrationSource(c); e == nil {
			t.Fatal("untested configured version accepted")
		}
		c.APIVersion = ""
		c.CredentialMode = "ambient"
		if _, e = OpenMigrationSource(c); e == nil {
			t.Fatal("ambient credentials accepted")
		}
	}
}
