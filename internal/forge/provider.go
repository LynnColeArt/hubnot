// Package forge defines provider-neutral issue transfer values. It has no
// network implementation, forge credentials, or native Hubnot authority.
package forge

import "context"

type Issue struct {
	ID, Key, Title, Body, State, Author, URL, CreatedAt, UpdatedAt string
	Labels, Assignees                                              []string
	IsChangeRequest                                                bool
}
type Comment struct{ ID, Body, Author, URL, CreatedAt, UpdatedAt string }
type IssueInput struct {
	Title, Body, State string
	Labels, Assignees  []string
}

// Provider translates an explicit external project and opaque provider key.
// ID is immutable identity; Key is only the provider's lookup/display selector.
type Provider interface {
	Kind() string
	Endpoint() string
	GetIssue(context.Context, string, string) (Issue, error)
	ListIssues(context.Context, string) ([]Issue, error)
	ListComments(context.Context, string, string) ([]Comment, error)
	CreateIssue(context.Context, string, IssueInput) (Issue, error)
	UpdateIssue(context.Context, string, string, IssueInput) (Issue, error)
	CreateComment(context.Context, string, string, string) (Comment, error)
}

// Resource names the thing being transferred, not an equivalence between
// provider-specific workflow engines, project structures or discussion models.
type Resource string

const (
	ResourceIssue      Resource = "issue"
	ResourceProject    Resource = "project"
	ResourceWorkflow   Resource = "workflow"
	ResourceDiscussion Resource = "discussion"
)

func Resources() []Resource {
	return []Resource{ResourceIssue, ResourceProject, ResourceWorkflow, ResourceDiscussion}
}
func KnownResource(resource Resource) bool {
	for _, known := range Resources() {
		if resource == known {
			return true
		}
	}
	return false
}

type Capability struct {
	Resource     Resource `json:"resource"`
	Import       bool     `json:"import"`
	Export       bool     `json:"export"`
	Reason       string   `json:"reason,omitempty"`
	MappedFields []string `json:"mapped_fields"`
	Unsupported  []string `json:"unsupported"`
}
type MappingReport struct {
	Provider     string       `json:"provider"`
	Capabilities []Capability `json:"capabilities"`
}
