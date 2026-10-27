package main

import (
	"hubnot/internal/migration"
	"testing"
)

func TestMigrationProviderUnknownIsOfflineFailure(t *testing.T) {
	c := migration.ProviderConfig{Locator: migration.Locator{Kind: "uncompiled-provider", Endpoint: "https://unused.invalid", Repository: "owner/repo"}, Token: "must-not-leak"}
	if _, e := OpenMigrationSource(c); e == nil {
		t.Fatal("unknown source accepted")
	}
	if _, e := OpenMigrationDestination(c); e == nil {
		t.Fatal("unknown destination accepted")
	}
}
