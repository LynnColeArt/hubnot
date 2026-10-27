//go:build !noforge

package main

import (
	"errors"
	"hubnot/internal/giteaapi"
	"hubnot/internal/migration"
)

func init() { registerV1Migration("gitea", "HN_GITEA_TOKEN", newGiteaMigration) }
func newGiteaMigration(c migration.ProviderConfig) (*v1Migration, error) {
	if c.Locator.Kind != "gitea" {
		return nil, errors.New("Gitea profile requires gitea provider identity")
	}
	client, e := giteaapi.NewVersionClient(c.Locator.Endpoint, c.Token, c.APIVersion)
	if e != nil {
		return nil, e
	}
	return newV1Migration(c, client.Client)
}
