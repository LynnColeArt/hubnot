package forge

import (
	"context"
	"encoding/json"
)

// Identity is the only common requirement for an optional adapter. Endpoint is
// an inert bounded locator; a network adapter validates its own URL policy.
// Merely reading source metadata must never open this locator.
type Identity interface {
	Kind() string
	Endpoint() string
}
type Selection struct {
	Project, Key string
	Mappings     map[string]string
}
type ProjectReader interface {
	ReadProjects(context.Context, Selection) (Graph, error)
}
type DiscussionReader interface {
	ReadDiscussions(context.Context, Selection) (Graph, error)
}
type WorkflowReader interface {
	ReadWorkflows(context.Context, Selection) (Graph, error)
}

// Record separates immutable provider identity from mutable selectors. Revision
// denotes the exact provider observation, when that provider exposes revisions.
type Record struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Revision  string `json:"revision,omitempty"`
	URL       string `json:"url,omitempty"`
	Author    string `json:"author,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}
type Ref struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision string `json:"revision,omitempty"`
}
type Graph struct {
	// UsedMappings names explicit provider mapping keys actually validated by the reader.
	UsedMappings []string  `json:"used_mappings,omitempty"`
	Nodes        []Node    `json:"nodes"`
	Findings     []Finding `json:"findings"`
}

// Node is a closed typed union. Child items and replies are independent native
// aggregates; they are never flattened into issue bodies or container events.
type Node struct {
	Kind       string            `json:"kind"`
	Record     Record            `json:"record"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Project    *Project          `json:"project,omitempty"`
	Item       *ProjectItem      `json:"item,omitempty"`
	Discussion *Discussion       `json:"discussion,omitempty"`
	Reply      *Reply            `json:"reply,omitempty"`
	Workflow   *Workflow         `json:"workflow,omitempty"`
}
type Project struct {
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	Archived bool    `json:"archived"`
	Fields   []Field `json:"fields"`
	Views    []View  `json:"views"`
}
type Field struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []Option `json:"options,omitempty"`
}
type Option struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type View struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Layout       string `json:"layout"`
	GroupByField string `json:"group_by_field,omitempty"`
}
type ProjectItem struct {
	Project  Ref              `json:"project"`
	Target   Target           `json:"target"`
	Values   map[string]Value `json:"values"`
	Archived bool             `json:"archived"`
	Position string           `json:"position"`
}
type Target struct {
	Kind  string `json:"kind"`
	Ref   Ref    `json:"ref"`
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	URL   string `json:"url,omitempty"`
}
type Value struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Number   string `json:"number,omitempty"`
	Date     string `json:"date,omitempty"`
	OptionID string `json:"option_id,omitempty"`
}
type Discussion struct {
	Title         string `json:"title"`
	Body          string `json:"body"`
	Category      string `json:"category"`
	Status        string `json:"status"`
	AcceptedReply Ref    `json:"accepted_reply"`
}
type Reply struct {
	Discussion Ref    `json:"discussion"`
	Parent     Ref    `json:"parent"`
	Body       string `json:"body"`
}
type Workflow struct {
	Name        string           `json:"name"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Enabled     bool             `json:"enabled"`
	Pipeline    json.RawMessage  `json:"pipeline,omitempty"`
	Sources     []SourceDocument `json:"sources"`
}
type SourceDocument struct {
	Format  string `json:"format"`
	Content string `json:"content"`
	Digest  string `json:"digest"`
}

type FindingStatus string

const (
	Mapped            FindingStatus = "mapped"
	Preserved         FindingStatus = "preserved"
	RequiresMapping   FindingStatus = "requires_mapping"
	Unsupported       FindingStatus = "unsupported"
	MissingDependency FindingStatus = "missing_dependency"
	IdentityConflict  FindingStatus = "identity_conflict"
)

type Finding struct {
	Status  FindingStatus `json:"status"`
	Path    string        `json:"path"`
	Message string        `json:"message"`
}
type Plan struct {
	Resource  Resource  `json:"resource"`
	Direction string    `json:"direction"`
	Nodes     int       `json:"nodes"`
	Findings  []Finding `json:"findings"`
}

func (p Plan) Blocked() bool {
	for _, f := range p.Findings {
		if f.Status != Mapped && f.Status != Preserved {
			return true
		}
	}
	return false
}
