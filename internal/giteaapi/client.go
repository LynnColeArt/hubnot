//go:build !noforge

// Package giteaapi selects independently tested Gitea 1.24 and 1.27 profiles.
// Common /api/v1 endpoints were compared with each versioned Swagger, rather
// than inferred from Forgejo. Other Gitea minor families are refused. 1.27 adds
// optimistic issue content versions; the migration adapter sends that guard.
package giteaapi

import "hubnot/internal/forgejoapi"

type Client struct{ *forgejoapi.Client }

func NewClient(root, token string) (*Client, error) {
	return NewVersionClient(root, token, "1.27")
}
func NewVersionClient(root, token, version string) (*Client, error) {
	if version == "" {
		version = "1.27"
	}
	c, e := forgejoapi.NewProfileClient(root, token, "gitea-"+version)
	if e != nil {
		return nil, e
	}
	return &Client{c}, nil
}
