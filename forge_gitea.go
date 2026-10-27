//go:build !noforge

package main

import (
	"hubnot/internal/forge"
	"hubnot/internal/giteaapi"
)

func init() {
	forgeProviders["gitea"] = forgeRegistration{TokenEnv: "HN_GITEA_TOKEN", New: func(endpoint, token string) (forge.Provider, error) {
		c, e := giteaapi.NewClient(endpoint, token)
		if e != nil {
			return nil, e
		}
		return &v1Forge{kind: "gitea", client: c.Client}, nil
	}, Capabilities: v1ForgeCapabilities("gitea-1.27")}
}
