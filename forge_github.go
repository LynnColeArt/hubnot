//go:build !nogithub

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"hubnot/internal/forge"
	"hubnot/internal/githubapi"
)

type githubForge struct{ client *githubapi.Client }

func init() {
	forgeProviders["github"] = forgeRegistration{New: func(endpoint, token string) (forge.Provider, error) {
		client, err := githubapi.NewClient(endpoint, token)
		if err != nil {
			return nil, err
		}
		return &githubForge{client}, nil
	}, TokenEnv: "HN_GITHUB_TOKEN", Capabilities: []forge.Capability{{Resource: forge.ResourceIssue, Import: true, Export: true, MappedFields: []string{"title", "body", "open/closed state", "labels", "assignees", "comments", "source attribution"}, Unsupported: []string{"pull requests", "reactions", "milestones", "attachments retained as external URLs only", "native criteria/relations require explicit export representation"}}}}
}
func (g *githubForge) Kind() string     { return "github" }
func (g *githubForge) Endpoint() string { return g.client.BaseURL() }
func githubKey(key string) (int, error) {
	n, err := strconv.Atoi(key)
	if err != nil || n <= 0 || strconv.Itoa(n) != key {
		return 0, errors.New("GitHub issue keys must be canonical positive numbers")
	}
	return n, nil
}
func githubOpaqueID(node string, id int64) string {
	if node != "" {
		return "node:" + node
	}
	return "id:" + strconv.FormatInt(id, 10)
}
func githubForgeIssue(v githubapi.Issue) forge.Issue {
	labels := []string{}
	for _, l := range v.Labels {
		labels = append(labels, l.Name)
	}
	assignees := []string{}
	for _, a := range v.Assignees {
		assignees = append(assignees, a.Login)
	}
	return forge.Issue{ID: githubOpaqueID(v.NodeID, v.ID), Key: strconv.Itoa(v.Number), Title: v.Title, Body: v.Body, State: v.State, Author: v.User.Login, URL: v.HTMLURL, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Labels: labels, Assignees: assignees, IsChangeRequest: len(v.PullRequest) > 0 && string(v.PullRequest) != "null"}
}
func githubForgeComment(v githubapi.Comment) forge.Comment {
	return forge.Comment{ID: githubOpaqueID(v.NodeID, v.ID), Body: v.Body, Author: v.User.Login, URL: v.HTMLURL, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func githubForgeInput(v forge.IssueInput) githubapi.IssueInput {
	return githubapi.IssueInput{Title: v.Title, Body: v.Body, State: v.State, Labels: v.Labels, Assignees: v.Assignees}
}
func (g *githubForge) GetIssue(ctx context.Context, project, key string) (forge.Issue, error) {
	n, err := githubKey(key)
	if err != nil {
		return forge.Issue{}, err
	}
	v, err := g.client.GetIssue(ctx, project, n)
	return githubForgeIssue(v), err
}
func (g *githubForge) ListIssues(ctx context.Context, project string) ([]forge.Issue, error) {
	values, err := g.client.ListIssues(ctx, project)
	if err != nil {
		return nil, err
	}
	result := make([]forge.Issue, 0, len(values))
	for _, v := range values {
		result = append(result, githubForgeIssue(v))
	}
	return result, nil
}
func (g *githubForge) ListComments(ctx context.Context, project, key string) ([]forge.Comment, error) {
	n, err := githubKey(key)
	if err != nil {
		return nil, err
	}
	values, err := g.client.ListComments(ctx, project, n)
	if err != nil {
		return nil, err
	}
	result := make([]forge.Comment, 0, len(values))
	for _, v := range values {
		result = append(result, githubForgeComment(v))
	}
	return result, nil
}
func (g *githubForge) CreateIssue(ctx context.Context, project string, input forge.IssueInput) (forge.Issue, error) {
	v, err := g.client.CreateIssue(ctx, project, githubForgeInput(input))
	return githubForgeIssue(v), err
}
func (g *githubForge) UpdateIssue(ctx context.Context, project, key string, input forge.IssueInput) (forge.Issue, error) {
	n, err := githubKey(key)
	if err != nil {
		return forge.Issue{}, err
	}
	v, err := g.client.UpdateIssue(ctx, project, n, githubForgeInput(input))
	return githubForgeIssue(v), err
}
func (g *githubForge) CreateComment(ctx context.Context, project, key, body string) (forge.Comment, error) {
	n, err := githubKey(key)
	if err != nil {
		return forge.Comment{}, err
	}
	v, err := g.client.CreateComment(ctx, project, n, body)
	return githubForgeComment(v), err
}

func cmdGitHub(args []string) error {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Print("Usage: hn github import --repo OWNER/REPO [--issue NUMBER] [--api-url URL] [--dry-run]\n       hn github export ISSUE --repo OWNER/REPO [--api-url URL] [--dry-run]\nConvenience aliases for hn transfer --provider github.\n")
		return nil
	}
	o := transferOptions{command: args[0], provider: "github"}
	rest := args[1:]
	if o.command != "import" && o.command != "export" {
		return errors.New("unsupported GitHub command; run 'hn github help'")
	}
	if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
		return cmdGitHub(nil)
	}
	if o.command == "export" {
		if len(rest) == 0 || !validEventID(rest[0]) {
			return errors.New("export requires a full native issue ID before flags")
		}
		o.id = rest[0]
		rest = rest[1:]
	}
	flags := flag.NewFlagSet("github", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&o.project, "repo", "", "explicit GitHub repository")
	flags.StringVar(&o.endpoint, "api-url", "", "explicit API URL")
	flags.BoolVar(&o.dryRun, "dry-run", false, "preview")
	number := 0
	if o.command == "import" {
		flags.IntVar(&number, "issue", 0, "issue number")
	}
	if err := flags.Parse(rest); err != nil {
		return errors.New("invalid GitHub flags; run 'hn github help'")
	}
	if flags.NArg() != 0 || strings.TrimSpace(o.project) == "" {
		return errors.New("--repo OWNER/REPO is required, without extra arguments")
	}
	provided := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "issue" {
			provided = true
		}
	})
	if provided {
		if number <= 0 {
			return errors.New("--issue must be positive")
		}
		o.key = strconv.Itoa(number)
	}
	return runForgeTransfer(o)
}
