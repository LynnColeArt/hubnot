package main

import (
	"context"
	"fmt"
	"hubnot/internal/migration"
)

var migrationProviders migration.Registry

// MigrationProviderKinds is offline and reflects compiled-in optional adapters.
func MigrationProviderKinds() []string { return migrationProviders.Kinds() }

// OpenMigrationSource and OpenMigrationDestination are the command integration
// seams. Credentials are supplied explicitly on each invocation, never loaded
// from a saved plan. Native/local/archive construction remains independent.
// ComposeMigrationSources and ComposeMigrationDestinations add the existing
// shared Git companion; optional providers do not copy code themselves.
func OpenMigrationSource(c migration.ProviderConfig) (migration.Source, error) {
	p, ok := migrationProviders.Lookup(c.Locator.Kind)
	if !ok {
		return nil, fmt.Errorf("migration provider %q is excluded or unavailable", c.Locator.Kind)
	}
	s, _, e := p.Open(context.Background(), c)
	return s, e
}
func OpenMigrationDestination(c migration.ProviderConfig) (migration.Destination, error) {
	p, ok := migrationProviders.Lookup(c.Locator.Kind)
	if !ok {
		return nil, fmt.Errorf("migration provider %q is excluded or unavailable", c.Locator.Kind)
	}
	_, d, e := p.Open(context.Background(), c)
	return d, e
}
