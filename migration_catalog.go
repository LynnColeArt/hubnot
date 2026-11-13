package main

import (
	"io"
	"slices"

	"hubnot/internal/migration"
)

type migrationCatalogField = migration.ProviderField

type migrationCatalogAdapter struct {
	Kind   string                  `json:"kind"`
	Fields []migrationCatalogField `json:"fields"`
}

type migrationCatalogPath struct {
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	Operations  []string `json:"operations"`
}

func writeMigrationCatalog(out io.Writer) error {
	return writeMigrationRegistryCatalog(out, &migrationProviders)
}

func writeMigrationRegistryCatalog(out io.Writer, registry *migration.Registry) error {
	localFields := []migrationCatalogField{{Name: "repository", Label: "Repository or path", Required: true}}
	sources := []migrationCatalogAdapter{
		{"local", localFields}, {"archive", localFields}, {"saved", localFields}, {"imported", localFields},
	}
	destinations := []migrationCatalogAdapter{{"local", []migrationCatalogField{}}, {"archive", localFields}}
	providers := registry.Kinds()
	for _, kind := range providers {
		fields := []migrationCatalogField{
			{Name: "repository", Label: "Repository", Required: false},
			{Name: "endpoint", Label: "API endpoint", Required: true},
			{Name: "version", Label: "API version", Required: true},
			{Name: "credentialMode", Label: "Credential mode", Required: true},
			{Name: "organization", Label: "Organization", Required: false},
			{Name: "project", Label: "Project", Required: false},
		}
		provider, _ := registry.Lookup(kind)
		sourceFields, destinationFields := fields, fields
		if provider.SourceFields != nil {
			sourceFields = provider.SourceFields
		}
		if provider.DestinationFields != nil {
			destinationFields = provider.DestinationFields
		}
		sources = append(sources, migrationCatalogAdapter{kind, sourceFields})
		destinations = append(destinations, migrationCatalogAdapter{kind, destinationFields})
	}
	paths := []migrationCatalogPath{}
	for _, source := range sources {
		for _, destination := range destinations {
			if !migrationRegistryPathAvailable(registry, source.Kind, destination.Kind) {
				continue
			}
			paths = append(paths, migrationCatalogPath{source.Kind, destination.Kind, []string{"plan", "inspect", "apply", "resume"}})
		}
	}
	return migrationCLIJSON(out, struct {
		Schema       string                    `json:"schema"`
		Providers    []string                  `json:"providers"`
		Sources      []migrationCatalogAdapter `json:"sources"`
		Destinations []migrationCatalogAdapter `json:"destinations"`
		Paths        []migrationCatalogPath    `json:"paths"`
	}{"hn.migration-catalog/2", providers, sources, destinations, paths})
}

func migrationPathAvailable(source, destination string) bool {
	return migrationRegistryPathAvailable(&migrationProviders, source, destination)
}

func migrationRegistryPathAvailable(registry *migration.Registry, source, destination string) bool {
	if provider, ok := registry.Lookup(source); ok && provider.Destinations != nil {
		return slices.Contains(provider.Destinations, destination)
	}
	return true
}
