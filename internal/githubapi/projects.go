//go:build !nogithub && !noforge

package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Project selectors deliberately distinguish planning-board ownership from repositories.
type ProjectSelector struct {
	OwnerKind, Login, ID string
	Number               int
}

func ParseProjectSelector(s string, ownerOnly bool) (ProjectSelector, error) {
	var p ProjectSelector
	kind, rest, ok := strings.Cut(s, ":")
	if !ok {
		return p, projectError()
	}
	if kind == "node" && validGraphQLID(rest) {
		p.ID = rest
		return p, nil
	}
	if kind != "user" && kind != "org" {
		return p, projectError()
	}
	p.OwnerKind = kind
	login, number, has := strings.Cut(rest, "/")
	if login == "" || len(login) > 100 {
		return p, projectError()
	}
	for _, r := range login {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return p, projectError()
		}
	}
	if strings.HasPrefix(login, "-") || strings.HasSuffix(login, "-") {
		return p, projectError()
	}
	p.Login = login
	if !has && ownerOnly {
		return p, nil
	}
	n, err := strconv.Atoi(number)
	if !has || err != nil || n <= 0 || n > 2147483647 || strconv.Itoa(n) != number {
		return p, projectError()
	}
	p.Number = n
	return p, nil
}

type ProjectOwner struct{ ID, Kind, Login string }
type ProjectOption struct{ ID, Name, Color, Description string }
type ProjectField struct {
	ID, Name, DataType string
	Options            []ProjectOption
}
type ProjectValue struct {
	Type, FieldID, Text, Date, OptionID string
	Number                              json.Number
}
type ProjectContent struct{ Type, ID, Title, Body, URL string }
type ProjectItem struct {
	ID, ProjectID, CreatedAt, UpdatedAt, Creator string
	Archived                                     bool
	Content                                      *ProjectContent
	Values                                       []ProjectValue
}
type ProjectView struct {
	ID, Name, Layout, Filter                     string
	Groups, VerticalGroups, Sorts, VisibleFields int
}
type ProjectV2 struct {
	ID, Title, Readme, ShortDescription, URL, CreatedAt, UpdatedAt, Creator string
	Number                                                                  int
	Closed, Public, Template                                                bool
	Owner                                                                   ProjectOwner
	Fields                                                                  []ProjectField
	Items                                                                   []ProjectItem
	Views                                                                   []ProjectView
	Workflows, Repositories, Teams                                          int
}

type projectOwnerNode struct {
	Type      string `json:"__typename"`
	ID, Login string
}
type projectNode struct {
	Type                                        string `json:"__typename"`
	ID                                          string
	Number                                      *int
	Title, Readme                               *string
	ShortDescription, URL, CreatedAt, UpdatedAt string
	Creator                                     *graphQLActor
	Closed, Public, Template                    *bool
	Owner                                       *projectOwnerNode
	Workflows, Repositories, Teams              *graphQLCount
}
type projectFieldNode struct {
	Type               string `json:"__typename"`
	ID, Name, DataType string
	Options            []ProjectOption
}
type projectContentNode struct {
	Type                 string `json:"__typename"`
	ID, Title, Body, URL string
}
type projectItemNode struct {
	ID, CreatedAt, UpdatedAt string
	Project                  *graphQLRef
	Creator                  *graphQLActor
	IsArchived               *bool
	Content                  *projectContentNode
}
type projectValueNode struct {
	Type                 string `json:"__typename"`
	ID                   string
	Field                *projectFieldNode
	Text, Date, OptionID *string
	Number               *json.Number
}
type projectViewNode struct {
	ID, Name, Layout, Filter                           string
	GroupByFields, VerticalGroupByFields, SortByFields *graphQLCount
	Configuration                                      *struct{ VisibleFields *graphQLCount }
}

const projectFields = `__typename id number title readme shortDescription url createdAt updatedAt creator{login} closed public template owner{__typename ... on User{id login} ... on Organization{id login}} workflows(first:1){totalCount} repositories(first:1){totalCount} teams(first:1){totalCount}`
const projectFieldFields = `__typename ... on ProjectV2Field{id name dataType} ... on ProjectV2SingleSelectField{id name dataType options{id name color description}} ... on ProjectV2IterationField{id name dataType} ... on ProjectV2MultiSelectField{id name dataType}`
const projectItemFields = `id createdAt updatedAt creator{login} isArchived project{id} content{__typename ... on DraftIssue{id title body} ... on Issue{id title body url} ... on PullRequest{id title body url}}`
const projectValueFields = `__typename ... on ProjectV2ItemFieldTextValue{id text field{` + projectFieldFields + `}} ... on ProjectV2ItemFieldNumberValue{id number field{` + projectFieldFields + `}} ... on ProjectV2ItemFieldDateValue{id date field{` + projectFieldFields + `}} ... on ProjectV2ItemFieldSingleSelectValue{id optionId field{` + projectFieldFields + `}}`
const projectViewFields = `id name layout filter groupByFields(first:1){totalCount} verticalGroupByFields(first:1){totalCount} sortByFields(first:1){totalCount} configuration{visibleFields(first:1){totalCount}}`
const projectUserQuery = `query HNUserProject($login:String!,$number:Int!){user(login:$login){projectV2(number:$number){` + projectFields + `}}}`
const projectOrgQuery = `query HNOrgProject($login:String!,$number:Int!){organization(login:$login){projectV2(number:$number){` + projectFields + `}}}`
const projectIDQuery = `query HNProjectID($id:ID!){node(id:$id){... on ProjectV2{` + projectFields + `}}}`
const projectOwnerUserQuery = `query HNProjectUserOwner($login:String!){user(login:$login){__typename id login}}`
const projectOwnerOrgQuery = `query HNProjectOrgOwner($login:String!){organization(login:$login){__typename id login}}`
const projectListUserQuery = `query HNUserProjects($login:String!,$after:String){user(login:$login){projectsV2(first:100,after:$after){nodes{` + projectFields + `} ` + graphQLPageFields + `}}}`
const projectListOrgQuery = `query HNOrgProjects($login:String!,$after:String){organization(login:$login){projectsV2(first:100,after:$after){nodes{` + projectFields + `} ` + graphQLPageFields + `}}}`
const projectFieldQuery = `query HNProjectFields($id:ID!,$after:String){node(id:$id){... on ProjectV2{id fields(first:100,after:$after){nodes{` + projectFieldFields + `} ` + graphQLPageFields + `}}}}`
const projectItemsQuery = `query HNProjectItems($id:ID!,$after:String){node(id:$id){... on ProjectV2{id items(first:100,after:$after,archivedStates:[ARCHIVED,NOT_ARCHIVED]){nodes{` + projectItemFields + `} ` + graphQLPageFields + `}}}}`
const projectValuesQuery = `query HNProjectValues($id:ID!,$after:String){node(id:$id){... on ProjectV2Item{id project{id} fieldValues(first:100,after:$after){nodes{` + projectValueFields + `} ` + graphQLPageFields + `}}}}`
const projectViewsQuery = `query HNProjectViews($id:ID!,$after:String){node(id:$id){... on ProjectV2{id views(first:100,after:$after){nodes{` + projectViewFields + `} ` + graphQLPageFields + `}}}}`
const createProjectMutation = `mutation HNCreateProject($input:CreateProjectV2Input!){createProjectV2(input:$input){projectV2{` + projectFields + `}}}`
const updateProjectMutation = `mutation HNUpdateProject($input:UpdateProjectV2Input!){updateProjectV2(input:$input){projectV2{` + projectFields + `}}}`
const createProjectFieldMutation = `mutation HNCreateProjectField($input:CreateProjectV2FieldInput!){createProjectV2Field(input:$input){projectV2Field{` + projectFieldFields + `}}}`
const updateProjectFieldMutation = `mutation HNUpdateProjectField($input:UpdateProjectV2FieldInput!){updateProjectV2Field(input:$input){projectV2Field{` + projectFieldFields + `}}}`
const addProjectContentMutation = `mutation HNAddProjectContent($input:AddProjectV2ItemByIdInput!){addProjectV2ItemById(input:$input){item{` + projectItemFields + `}}}`
const addProjectDraftMutation = `mutation HNAddProjectDraft($input:AddProjectV2DraftIssueInput!){addProjectV2DraftIssue(input:$input){projectItem{` + projectItemFields + `}}}`
const updateProjectDraftMutation = `mutation HNUpdateProjectDraft($input:UpdateProjectV2DraftIssueInput!){updateProjectV2DraftIssue(input:$input){draftIssue{id title body}}}`
const setProjectValueMutation = `mutation HNSetProjectValue($input:UpdateProjectV2ItemFieldValueInput!){updateProjectV2ItemFieldValue(input:$input){projectV2Item{` + projectItemFields + `}}}`
const clearProjectValueMutation = `mutation HNClearProjectValue($input:ClearProjectV2ItemFieldValueInput!){clearProjectV2ItemFieldValue(input:$input){projectV2Item{` + projectItemFields + `}}}`
const archiveProjectItemMutation = `mutation HNArchiveProjectItem($input:ArchiveProjectV2ItemInput!){archiveProjectV2Item(input:$input){item{` + projectItemFields + `}}}`
const unarchiveProjectItemMutation = `mutation HNUnarchiveProjectItem($input:UnarchiveProjectV2ItemInput!){unarchiveProjectV2Item(input:$input){item{` + projectItemFields + `}}}`
const positionProjectItemMutation = `mutation HNPositionProjectItem($input:UpdateProjectV2ItemPositionInput!){updateProjectV2ItemPosition(input:$input){clientMutationId}}`
const createProjectViewMutation = `mutation HNCreateProjectView($input:CreateProjectV2ViewInput!){createProjectV2View(input:$input){projectV2View{` + projectViewFields + `}}}`
const updateProjectViewMutation = `mutation HNUpdateProjectView($input:UpdateProjectV2ViewInput!){updateProjectV2View(input:$input){projectV2View{` + projectViewFields + `}}}`

func projectError() error {
	return errors.New("invalid or inaccessible GitHub Projects V2 snapshot or selector (use user:LOGIN/NUMBER, org:LOGIN/NUMBER or node:ID)")
}
func validProject(n *projectNode) bool {
	return n != nil && n.Type == "ProjectV2" && validGraphQLID(n.ID) && n.Number != nil && *n.Number > 0 && n.Title != nil && strings.TrimSpace(*n.Title) != "" && n.Closed != nil && n.Public != nil && n.Template != nil && n.Owner != nil && validGraphQLID(n.Owner.ID) && (n.Owner.Type == "User" || n.Owner.Type == "Organization") && n.Owner.Login != "" && validSourceTime(n.CreatedAt) && validSourceTime(n.UpdatedAt) && validActor(n.Creator) && validCount(n.Workflows) && validCount(n.Repositories) && validCount(n.Teams)
}
func (n projectNode) value() ProjectV2 {
	body := ""
	if n.Readme != nil {
		body = *n.Readme
	}
	return ProjectV2{ID: n.ID, Title: *n.Title, Readme: body, ShortDescription: n.ShortDescription, URL: n.URL, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, Creator: actorLogin(n.Creator), Number: *n.Number, Closed: *n.Closed, Public: *n.Public, Template: *n.Template, Owner: ProjectOwner{n.Owner.ID, n.Owner.Type, n.Owner.Login}, Workflows: *n.Workflows.TotalCount, Repositories: *n.Repositories.TotalCount, Teams: *n.Teams.TotalCount}
}
func validProjectField(n *projectFieldNode) bool {
	if n == nil || !validGraphQLID(n.ID) || n.Name == "" || n.DataType == "" {
		return false
	}
	switch n.Type {
	case "ProjectV2Field", "ProjectV2SingleSelectField", "ProjectV2IterationField", "ProjectV2MultiSelectField":
	default:
		return false
	}
	seen := map[string]bool{}
	for _, o := range n.Options {
		if !validGraphQLID(o.ID) || o.Name == "" || seen[o.ID] {
			return false
		}
		seen[o.ID] = true
	}
	return true
}
func (n projectFieldNode) value() ProjectField {
	return ProjectField{n.ID, n.Name, n.DataType, n.Options}
}
func validProjectItem(n *projectItemNode) bool {
	return n != nil && validGraphQLID(n.ID) && n.Project != nil && validGraphQLID(n.Project.ID) && n.IsArchived != nil && validSourceTime(n.CreatedAt) && validSourceTime(n.UpdatedAt) && validActor(n.Creator) && (n.Content == nil || validGraphQLID(n.Content.ID))
}
func (n projectItemNode) value() ProjectItem {
	v := ProjectItem{ID: n.ID, ProjectID: n.Project.ID, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, Creator: actorLogin(n.Creator), Archived: *n.IsArchived}
	if n.Content != nil {
		v.Content = &ProjectContent{n.Content.Type, n.Content.ID, n.Content.Title, n.Content.Body, n.Content.URL}
	}
	return v
}
func validProjectView(n *projectViewNode) bool {
	return n != nil && validGraphQLID(n.ID) && n.Name != "" && n.Layout != "" && validCount(n.GroupByFields) && validCount(n.VerticalGroupByFields) && validCount(n.SortByFields) && n.Configuration != nil && validCount(n.Configuration.VisibleFields)
}
func (n projectViewNode) value() ProjectView {
	return ProjectView{n.ID, n.Name, n.Layout, n.Filter, *n.GroupByFields.TotalCount, *n.VerticalGroupByFields.TotalCount, *n.SortByFields.TotalCount, *n.Configuration.VisibleFields.TotalCount}
}
func (c *Client) GetProjectOwner(ctx context.Context, selector string) (ProjectOwner, error) {
	p, err := ParseProjectSelector(selector, true)
	if err != nil || p.Number != 0 || p.ID != "" {
		return ProjectOwner{}, projectError()
	}
	doc := projectOwnerUserQuery
	key := "user"
	typ := "User"
	if p.OwnerKind == "org" {
		doc = projectOwnerOrgQuery
		key = "organization"
		typ = "Organization"
	}
	var out map[string]*projectOwnerNode
	if err = c.graphQL(ctx, doc, map[string]any{"login": p.Login}, &out, &graphQLBudget{}); err != nil {
		return ProjectOwner{}, err
	}
	n := out[key]
	if n == nil || n.Type != typ || !validGraphQLID(n.ID) || !strings.EqualFold(n.Login, p.Login) {
		return ProjectOwner{}, projectError()
	}
	return ProjectOwner{n.ID, n.Type, n.Login}, nil
}
func (c *Client) projectHeader(ctx context.Context, selector string, b *graphQLBudget) (*projectNode, error) {
	p, err := ParseProjectSelector(selector, false)
	if err != nil {
		return nil, err
	}
	var out struct {
		Node               *projectNode
		User, Organization *struct{ ProjectV2 *projectNode }
	}
	doc := projectIDQuery
	vars := map[string]any{"id": p.ID}
	if p.ID == "" {
		vars = map[string]any{"login": p.Login, "number": p.Number}
		doc = projectUserQuery
		if p.OwnerKind == "org" {
			doc = projectOrgQuery
		}
	}
	if err = c.graphQL(ctx, doc, vars, &out, b); err != nil {
		return nil, err
	}
	n := out.Node
	if p.ID == "" {
		if p.OwnerKind == "user" && out.User != nil {
			n = out.User.ProjectV2
		}
		if p.OwnerKind == "org" && out.Organization != nil {
			n = out.Organization.ProjectV2
		}
	}
	if !validProject(n) || p.ID != "" && n.ID != p.ID || p.ID == "" && (*n.Number != p.Number || !strings.EqualFold(n.Owner.Login, p.Login) || (n.Owner.Type == "User") != (p.OwnerKind == "user")) {
		return nil, projectError()
	}
	return n, nil
}

// ListProjects returns metadata only, for exact positive create reconciliation.
func (c *Client) ListProjects(ctx context.Context, selector string) ([]ProjectV2, error) {
	p, err := ParseProjectSelector(selector, true)
	if err != nil || p.Number != 0 || p.ID != "" {
		return nil, projectError()
	}
	b := &graphQLBudget{}
	key := "user"
	doc := projectListUserQuery
	if p.OwnerKind == "org" {
		key = "organization"
		doc = projectListOrgQuery
	}
	rows, err := graphQLPages(func(after *string) (*graphQLConnection[projectNode], error) {
		var out map[string]*struct {
			ProjectsV2 *graphQLConnection[projectNode]
		}
		if err := c.graphQL(ctx, doc, map[string]any{"login": p.Login, "after": after}, &out, b); err != nil {
			return nil, err
		}
		if out[key] == nil {
			return nil, projectError()
		}
		return out[key].ProjectsV2, nil
	}, validProject, func(n *projectNode) string { return n.ID })
	if err != nil {
		return nil, err
	}
	result := []ProjectV2{}
	for _, n := range rows {
		if !strings.EqualFold(n.Owner.Login, p.Login) || (n.Owner.Type == "User") != (p.OwnerKind == "user") {
			return nil, projectError()
		}
		result = append(result, n.value())
	}
	return result, nil
}
func (c *Client) GetProject(ctx context.Context, selector string) (ProjectV2, error) {
	b := &graphQLBudget{}
	n, err := c.projectHeader(ctx, selector, b)
	if err != nil {
		return ProjectV2{}, err
	}
	p := n.value()
	fields, err := graphQLPages(func(after *string) (*graphQLConnection[projectFieldNode], error) {
		var out struct {
			Node *struct {
				ID     string
				Fields *graphQLConnection[projectFieldNode]
			}
		}
		if err := c.graphQL(ctx, projectFieldQuery, map[string]any{"id": p.ID, "after": after}, &out, b); err != nil {
			return nil, err
		}
		if out.Node == nil || out.Node.ID != p.ID {
			return nil, projectError()
		}
		return out.Node.Fields, nil
	}, validProjectField, func(n *projectFieldNode) string { return n.ID })
	if err != nil {
		return p, err
	}
	if len(fields) > 80 {
		return p, errors.New("GitHub project exceeds field bounds")
	}
	for _, f := range fields {
		p.Fields = append(p.Fields, f.value())
	}
	items, err := graphQLPages(func(after *string) (*graphQLConnection[projectItemNode], error) {
		var out struct {
			Node *struct {
				ID    string
				Items *graphQLConnection[projectItemNode]
			}
		}
		if err := c.graphQL(ctx, projectItemsQuery, map[string]any{"id": p.ID, "after": after}, &out, b); err != nil {
			return nil, err
		}
		if out.Node == nil || out.Node.ID != p.ID {
			return nil, projectError()
		}
		return out.Node.Items, nil
	}, func(n *projectItemNode) bool { return validProjectItem(n) && n.Project.ID == p.ID }, func(n *projectItemNode) string { return n.ID })
	if err != nil {
		return p, err
	}
	if len(items) > 255 {
		return p, errors.New("GitHub project exceeds 255 item transfer bound")
	}
	for _, n := range items {
		item := n.value()
		values, err := graphQLPages(func(after *string) (*graphQLConnection[projectValueNode], error) {
			var out struct {
				Node *struct {
					ID          string
					Project     *graphQLRef
					FieldValues *graphQLConnection[projectValueNode]
				}
			}
			if err := c.graphQL(ctx, projectValuesQuery, map[string]any{"id": item.ID, "after": after}, &out, b); err != nil {
				return nil, err
			}
			if out.Node == nil || out.Node.ID != item.ID || out.Node.Project == nil || out.Node.Project.ID != p.ID {
				return nil, projectError()
			}
			return out.Node.FieldValues, nil
		}, func(v *projectValueNode) bool { return v.Type != "" }, func(v *projectValueNode) string {
			if v.ID != "" {
				return v.ID
			}
			return "unsupported:" + v.Type
		})
		if err != nil {
			return p, err
		}
		if len(values) > 80 {
			return p, errors.New("GitHub item exceeds field value bounds")
		}
		for _, v := range values {
			field := ""
			switch v.Type {
			case "ProjectV2ItemFieldTextValue", "ProjectV2ItemFieldNumberValue", "ProjectV2ItemFieldDateValue", "ProjectV2ItemFieldSingleSelectValue":
				if !validGraphQLID(v.ID) || v.Field == nil {
					return p, projectError()
				}
			}
			if v.Field != nil {
				if !validProjectField(v.Field) {
					return p, projectError()
				}
				field = v.Field.ID
			}
			value := ProjectValue{Type: v.Type, FieldID: field}
			switch v.Type {
			case "ProjectV2ItemFieldTextValue":
				if v.Text == nil {
					return p, errors.New("GitHub text field value is null or absent; complete typed snapshot refused")
				}
				value.Text = *v.Text
			case "ProjectV2ItemFieldNumberValue":
				if v.Number == nil {
					return p, errors.New("GitHub number field value is null or absent; complete typed snapshot refused")
				}
				value.Number = *v.Number
			case "ProjectV2ItemFieldDateValue":
				if v.Date == nil {
					return p, errors.New("GitHub date field value is null or absent; complete typed snapshot refused")
				}
				value.Date = *v.Date
			case "ProjectV2ItemFieldSingleSelectValue":
				if v.OptionID == nil || !validGraphQLID(*v.OptionID) {
					return p, errors.New("GitHub option field value is null, absent or invalid; complete typed snapshot refused")
				}
				value.OptionID = *v.OptionID
			}
			item.Values = append(item.Values, value)
		}
		var check struct{ Node *projectItemNode }
		if err := c.graphQL(ctx, projectItemHeaderQuery, map[string]any{"id": item.ID}, &check, b); err != nil {
			return p, err
		}
		if !validProjectItem(check.Node) || check.Node.ID != item.ID || check.Node.Project.ID != p.ID || check.Node.UpdatedAt != n.UpdatedAt {
			return p, errors.New("GitHub project item changed during traversal")
		}
		a, _ := json.Marshal(n)
		z, _ := json.Marshal(check.Node)
		if string(a) != string(z) {
			return p, errors.New("GitHub project item content changed during traversal")
		}
		p.Items = append(p.Items, item)
	}
	views, err := graphQLPages(func(after *string) (*graphQLConnection[projectViewNode], error) {
		var out struct {
			Node *struct {
				ID    string
				Views *graphQLConnection[projectViewNode]
			}
		}
		if err := c.graphQL(ctx, projectViewsQuery, map[string]any{"id": p.ID, "after": after}, &out, b); err != nil {
			return nil, err
		}
		if out.Node == nil || out.Node.ID != p.ID {
			return nil, projectError()
		}
		return out.Node.Views, nil
	}, validProjectView, func(n *projectViewNode) string { return n.ID })
	if err != nil {
		return p, err
	}
	if len(views) > 16 {
		return p, errors.New("GitHub project exceeds 16 view bound")
	}
	for _, v := range views {
		p.Views = append(p.Views, v.value())
	}
	end, err := c.projectHeader(ctx, "node:"+p.ID, b)
	if err != nil {
		return p, err
	}
	a, _ := json.Marshal(n)
	z, _ := json.Marshal(end)
	if string(a) != string(z) {
		return p, errors.New("GitHub project changed during traversal; retry read")
	}
	return p, nil
}

// ProjectMutation performs exactly one typed mutation and returns its validated
// response object. No mutation is retried here. The caller owns durable intent.
func (c *Client) ProjectMutation(ctx context.Context, action string, input map[string]any) (json.RawMessage, error) {
	doc := ""
	field := ""
	member := ""
	switch action {
	case "create":
		doc = createProjectMutation
		field = "createProjectV2"
		member = "projectV2"
	case "update":
		doc = updateProjectMutation
		field = "updateProjectV2"
		member = "projectV2"
	case "field-create":
		doc = createProjectFieldMutation
		field = "createProjectV2Field"
		member = "projectV2Field"
	case "field-update":
		doc = updateProjectFieldMutation
		field = "updateProjectV2Field"
		member = "projectV2Field"
	case "item-content":
		doc = addProjectContentMutation
		field = "addProjectV2ItemById"
		member = "item"
	case "item-draft":
		doc = addProjectDraftMutation
		field = "addProjectV2DraftIssue"
		member = "projectItem"
	case "draft-update":
		doc = updateProjectDraftMutation
		field = "updateProjectV2DraftIssue"
		member = "draftIssue"
	case "value-set":
		doc = setProjectValueMutation
		field = "updateProjectV2ItemFieldValue"
		member = "projectV2Item"
	case "value-clear":
		doc = clearProjectValueMutation
		field = "clearProjectV2ItemFieldValue"
		member = "projectV2Item"
	case "item-archive":
		doc = archiveProjectItemMutation
		field = "archiveProjectV2Item"
		member = "item"
	case "item-unarchive":
		doc = unarchiveProjectItemMutation
		field = "unarchiveProjectV2Item"
		member = "item"
	case "item-position":
		doc = positionProjectItemMutation
		field = "updateProjectV2ItemPosition"
	case "view-create":
		doc = createProjectViewMutation
		field = "createProjectV2View"
		member = "projectV2View"
	case "view-update":
		doc = updateProjectViewMutation
		field = "updateProjectV2View"
		member = "projectV2View"
	default:
		return nil, errors.New("unsupported GitHub project mutation")
	}
	// Strictly whitelist fields so this typed seam cannot smuggle permission/public changes.
	allowed := map[string]string{"create": "ownerId title", "update": "projectId title readme closed", "field-create": "projectId name dataType singleSelectOptions", "field-update": "fieldId name", "item-content": "projectId contentId", "item-draft": "projectId title body", "draft-update": "draftIssueId title body", "value-set": "projectId itemId fieldId value", "value-clear": "projectId itemId fieldId", "item-archive": "projectId itemId", "item-unarchive": "projectId itemId", "item-position": "projectId itemId afterId", "view-create": "projectId name layout", "view-update": "viewId name layout"}[action]
	if len(input) == 0 {
		return nil, projectError()
	}
	for k, v := range input {
		if !strings.Contains(" "+allowed+" ", " "+k+" ") {
			return nil, projectError()
		}
		if strings.HasSuffix(k, "Id") && v != nil {
			val, ok := v.(string)
			if !ok || !validGraphQLID(val) {
				return nil, projectError()
			}
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > MaxResponseBytes || !utf8.Valid(encoded) {
		return nil, projectError()
	}
	var out map[string]json.RawMessage
	if err = c.graphQL(ctx, doc, map[string]any{"input": input}, &out, &graphQLBudget{}); err != nil {
		return nil, err
	}
	var payload map[string]json.RawMessage
	if json.Unmarshal(out[field], &payload) != nil {
		return nil, projectError()
	}
	if member == "" {
		if _, ok := payload["clientMutationId"]; !ok {
			return nil, projectError()
		}
		return json.RawMessage(`{"id":"position"}`), nil
	}
	raw := payload[member]
	if len(raw) == 0 || string(raw) == "null" {
		return nil, projectError()
	}
	var id struct{ ID string }
	if json.Unmarshal(raw, &id) != nil || !validGraphQLID(id.ID) {
		return nil, projectError()
	}
	return raw, nil
}
func ProjectMutationID(raw json.RawMessage) (string, error) {
	var n struct{ ID string }
	if json.Unmarshal(raw, &n) != nil || !validGraphQLID(n.ID) {
		return "", fmt.Errorf("invalid project mutation identity")
	}
	return n.ID, nil
}

const projectContentQuery = `query HNProjectContent($id:ID!){node(id:$id){__typename ... on Issue{id title body url} ... on PullRequest{id title body url}}}`

func (c *Client) GetProjectContent(ctx context.Context, id string) (ProjectContent, error) {
	if !validGraphQLID(id) {
		return ProjectContent{}, projectError()
	}
	var out struct{ Node *projectContentNode }
	if err := c.graphQL(ctx, projectContentQuery, map[string]any{"id": id}, &out, &graphQLBudget{}); err != nil {
		return ProjectContent{}, err
	}
	n := out.Node
	if n == nil || n.ID != id || (n.Type != "Issue" && n.Type != "PullRequest") || n.URL == "" {
		return ProjectContent{}, projectError()
	}
	return ProjectContent{n.Type, n.ID, n.Title, n.Body, n.URL}, nil
}

const projectItemHeaderQuery = `query HNProjectItemHeader($id:ID!){node(id:$id){... on ProjectV2Item{` + projectItemFields + `}}}`
