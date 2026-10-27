//go:build !noforge

package main

import (
	"context"
	"errors"
	"strconv"

	"hubnot/internal/forge"
	"hubnot/internal/gitlabapi"
)

type gitlabForge struct{ client *gitlabapi.Client }

func init() {
	forgeProviders["gitlab"] = forgeRegistration{New: func(endpoint, token string) (forge.Provider, error) {
		c, e := gitlabapi.NewClient(endpoint, token)
		if e != nil {
			return nil, e
		}
		return &gitlabForge{c}, nil
	}, TokenEnv: "HN_GITLAB_TOKEN", Capabilities: []forge.Capability{{Resource: forge.ResourceIssue, Import: true, Export: true, MappedFields: []string{"title", "description", "opened/closed state", "labels", "explicit numeric assignee IDs", "ordinary issue notes", "source attribution"}, Unsupported: []string{"closed creates need a separate state change and are refused", "confidential/nonordinary issues", "system/internal/confidential/thread notes are migration preservation records", "slash-command lines refuse destination writes; source text remains unchanged", "merge requests/reviews/boards/CI/uploads use migration snapshot rather than issue transfer"}}}}
}
func (g *gitlabForge) Kind() string     { return "gitlab" }
func (g *gitlabForge) Endpoint() string { return g.client.BaseURL() }
func gitlabForgeIssue(v gitlabapi.Issue) forge.Issue {
	state := v.State
	if state == "opened" {
		state = "open"
	}
	assignees := []string{}
	for _, u := range v.Assignees {
		assignees = append(assignees, u.Username)
	}
	return forge.Issue{ID: gitlabID(v.ID), Key: strconv.Itoa(v.IID), Title: v.Title, Body: v.Body, State: state, Author: v.Author.Username, URL: v.WebURL, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Labels: append([]string{}, v.Labels...), Assignees: assignees}
}
func gitlabForgeComment(v gitlabapi.Note, parentURL string) forge.Comment {
	return forge.Comment{ID: gitlabID(v.ID), Body: v.Body, Author: v.Author.Username, URL: gitlabNoteURL(parentURL, v.ID), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func gitlabForgeVisible(v gitlabapi.Issue) error {
	if v.Confidential || v.Type != "issue" {
		return errors.New("legacy GitLab transfer supports ordinary public issues; use migration for attributed preservation")
	}
	return nil
}
func (g *gitlabForge) GetIssue(ctx context.Context, p, key string) (forge.Issue, error) {
	n, e := gitlabapi.PositiveKey(key)
	if e != nil {
		return forge.Issue{}, e
	}
	v, e := g.client.GetIssue(ctx, p, n)
	if e != nil {
		return forge.Issue{}, e
	}
	if e = gitlabForgeVisible(v); e != nil {
		return forge.Issue{}, e
	}
	return gitlabForgeIssue(v), nil
}
func (g *gitlabForge) ListIssues(ctx context.Context, p string) ([]forge.Issue, error) {
	rows, e := g.client.ListIssues(ctx, p)
	if e != nil {
		return nil, e
	}
	out := []forge.Issue{}
	for _, v := range rows {
		if e = gitlabForgeVisible(v); e != nil {
			return nil, e
		}
		out = append(out, gitlabForgeIssue(v))
	}
	return out, nil
}
func (g *gitlabForge) ListComments(ctx context.Context, p, key string) ([]forge.Comment, error) {
	n, e := gitlabapi.PositiveKey(key)
	if e != nil {
		return nil, e
	}
	parent, e := g.client.GetIssue(ctx, p, n)
	if e != nil {
		return nil, e
	}
	if e = gitlabForgeVisible(parent); e != nil {
		return nil, e
	}
	rows, e := g.client.ListNotes(ctx, p, n)
	if e != nil {
		return nil, e
	}
	out := []forge.Comment{}
	for _, v := range rows {
		if !v.Ordinary() {
			return nil, errors.New("legacy GitLab transfer cannot flatten system/internal/thread notes; use migration snapshot")
		}
		if v.NoteableID != 0 && v.NoteableID != parent.ID {
			return nil, errors.New("GitLab note parent identity changed")
		}
		out = append(out, gitlabForgeComment(v, parent.WebURL))
	}
	return out, nil
}
func gitlabForgeInput(v forge.IssueInput) gitlabapi.IssueInput {
	state := v.State
	if state == "open" || state == "" {
		state = "opened"
	}
	return gitlabapi.IssueInput{Title: v.Title, Body: v.Body, State: state, Type: "issue", Labels: v.Labels, Assignees: v.Assignees}
}
func (g *gitlabForge) CreateIssue(ctx context.Context, p string, in forge.IssueInput) (forge.Issue, error) {
	v, e := g.client.CreateIssue(ctx, p, gitlabForgeInput(in))
	return gitlabForgeIssue(v), e
}
func (g *gitlabForge) UpdateIssue(ctx context.Context, p, key string, in forge.IssueInput) (forge.Issue, error) {
	n, e := gitlabapi.PositiveKey(key)
	if e != nil {
		return forge.Issue{}, e
	}
	before, e := g.client.GetIssue(ctx, p, n)
	if e != nil {
		return forge.Issue{}, e
	}
	if e = gitlabForgeVisible(before); e != nil {
		return forge.Issue{}, e
	}
	v, e := g.client.UpdateIssue(ctx, p, n, gitlabForgeInput(in))
	return gitlabForgeIssue(v), e
}
func (g *gitlabForge) CreateComment(ctx context.Context, p, key, body string) (forge.Comment, error) {
	n, e := gitlabapi.PositiveKey(key)
	if e != nil {
		return forge.Comment{}, e
	}
	before, e := g.client.GetIssue(ctx, p, n)
	if e != nil {
		return forge.Comment{}, e
	}
	if e = gitlabForgeVisible(before); e != nil {
		return forge.Comment{}, e
	}
	v, e := g.client.CreateNote(ctx, p, n, body)
	return gitlabForgeComment(v, before.WebURL), e
}
