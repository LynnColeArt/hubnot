//go:build !noforge

package main

import (
	"context"
	"errors"
	"strconv"

	"hubnot/internal/forge"
	"hubnot/internal/forgejoapi"
)

func init() {
	forgeProviders["forgejo"] = forgeRegistration{TokenEnv: "HN_FORGEJO_TOKEN", New: func(endpoint, token string) (forge.Provider, error) {
		c, e := forgejoapi.NewClient(endpoint, token)
		if e != nil {
			return nil, e
		}
		return &v1Forge{kind: "forgejo", client: c}, nil
	}, Capabilities: v1ForgeCapabilities("forgejo-16")}
}
func v1ForgeCapabilities(profile string) []forge.Capability {
	return []forge.Capability{{Resource: forge.ResourceIssue, Import: true, Export: true, MappedFields: []string{"title", "body", "open/closed state", "existing labels", "assignees", "comments", "source attribution"}, Unsupported: []string{profile + "; explicit /api/v1 and supplied token required", "255-byte title and 65536-byte body ceiling", "updates cannot change labels in one mutation", "pull requests excluded from legacy issue transfer", "attachments are external references only in legacy transfer; selected bytes require migration snapshot", "milestones/reactions/types/permissions and native authority are not recreated"}}}
}

type v1Forge struct {
	kind   string
	client *forgejoapi.Client
}

func (g *v1Forge) Kind() string     { return g.kind }
func (g *v1Forge) Endpoint() string { return g.client.BaseURL() }
func v1ForgeIssue(v forgejoapi.Issue) forge.Issue {
	labels := []string{}
	assignees := []string{}
	for _, x := range v.Labels {
		labels = append(labels, x.Name)
	}
	for _, x := range v.Assignees {
		assignees = append(assignees, x.Login)
	}
	return forge.Issue{ID: v1ID(v.ID), Key: strconv.Itoa(v.Number), Title: v.Title, Body: v.Body, State: v.State, Author: v.User.Login, URL: v.HTMLURL, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Labels: labels, Assignees: assignees, IsChangeRequest: v.IsPull()}
}
func v1ForgeComment(v forgejoapi.Comment) forge.Comment {
	return forge.Comment{ID: v1ID(v.ID), Body: v.Body, Author: v.User.Login, URL: v.HTMLURL, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func (g *v1Forge) GetIssue(ctx context.Context, repo, key string) (forge.Issue, error) {
	n, e := v1Index(key)
	if e != nil {
		return forge.Issue{}, e
	}
	if e = g.client.CheckVersion(ctx); e != nil {
		return forge.Issue{}, e
	}
	v, e := g.client.GetIssue(ctx, repo, n)
	return v1ForgeIssue(v), e
}
func (g *v1Forge) ListIssues(ctx context.Context, repo string) ([]forge.Issue, error) {
	if e := g.client.CheckVersion(ctx); e != nil {
		return nil, e
	}
	vs, e := g.client.ListIssues(ctx, repo)
	if e != nil {
		return nil, e
	}
	out := []forge.Issue{}
	for _, v := range vs {
		out = append(out, v1ForgeIssue(v))
	}
	return out, nil
}
func (g *v1Forge) ListComments(ctx context.Context, repo, key string) ([]forge.Comment, error) {
	n, e := v1Index(key)
	if e != nil {
		return nil, e
	}
	if e = g.client.CheckVersion(ctx); e != nil {
		return nil, e
	}
	vs, e := g.client.ListComments(ctx, repo, n)
	if e != nil {
		return nil, e
	}
	out := []forge.Comment{}
	for _, v := range vs {
		out = append(out, v1ForgeComment(v))
	}
	return out, nil
}
func (g *v1Forge) input(ctx context.Context, repo string, in forge.IssueInput) (forgejoapi.IssueInput, error) {
	out := forgejoapi.IssueInput{Title: in.Title, Body: in.Body, State: in.State, Labels: []int64{}, Assignees: in.Assignees}
	if out.State == "" {
		out.State = "open"
	}
	if len(in.Labels) > 0 {
		labels, e := g.client.ListLabels(ctx, repo)
		if e != nil {
			return out, e
		}
		for _, name := range in.Labels {
			id := int64(0)
			for _, label := range labels {
				if label.Name == name {
					if id != 0 {
						return out, errors.New("ambiguous label name")
					}
					id = label.ID
				}
			}
			if id <= 0 {
				return out, errors.New("label does not exist in selected repository")
			}
			out.Labels = append(out.Labels, id)
		}
	}
	return out, nil
}
func (g *v1Forge) CreateIssue(ctx context.Context, repo string, in forge.IssueInput) (forge.Issue, error) {
	if e := g.client.CheckVersion(ctx); e != nil {
		return forge.Issue{}, e
	}
	input, e := g.input(ctx, repo, in)
	if e != nil {
		return forge.Issue{}, e
	}
	v, e := g.client.CreateIssue(ctx, repo, input)
	if e == nil && !v1Matches(v, input) {
		e = errors.New("provider returned different issue projection; recovery required")
	}
	return v1ForgeIssue(v), e
}
func (g *v1Forge) UpdateIssue(ctx context.Context, repo, key string, in forge.IssueInput) (forge.Issue, error) {
	n, e := v1Index(key)
	if e != nil {
		return forge.Issue{}, e
	}
	if e = g.client.CheckVersion(ctx); e != nil {
		return forge.Issue{}, e
	}
	input, e := g.input(ctx, repo, in)
	if e != nil {
		return forge.Issue{}, e
	}
	current, e := g.client.GetIssue(ctx, repo, n)
	if e != nil {
		return forge.Issue{}, e
	}
	labels := []string{}
	for _, x := range input.Labels {
		labels = append(labels, strconv.FormatInt(x, 10))
	}
	if current.IsPull() || !v1SameStrings(v1Labels(current), labels) {
		return forge.Issue{}, errors.New("single issue PATCH requires ordinary issue and unchanged labels")
	}
	if g.client.Profile() == "gitea-1.27" {
		if current.ContentVersion <= 0 {
			return forge.Issue{}, errors.New("Gitea update requires observed content version")
		}
		input.ContentVersion = current.ContentVersion
	}
	v, e := g.client.UpdateIssue(ctx, repo, n, input)
	if e == nil && (!v1Matches(v, input) || v.ID != current.ID) {
		e = errors.New("provider returned different issue projection or identity; recovery required")
	}
	return v1ForgeIssue(v), e
}
func (g *v1Forge) CreateComment(ctx context.Context, repo, key, body string) (forge.Comment, error) {
	n, e := v1Index(key)
	if e != nil {
		return forge.Comment{}, e
	}
	if e = g.client.CheckVersion(ctx); e != nil {
		return forge.Comment{}, e
	}
	v, e := g.client.CreateComment(ctx, repo, n, body)
	if e == nil && v.Body != body {
		e = errors.New("provider returned different comment; recovery required")
	}
	return v1ForgeComment(v), e
}
