//go:build !noforge

package azureapi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Project struct {
	Raw          json.RawMessage              `json:"-"`
	ID           string                       `json:"id"`
	Name         string                       `json:"name"`
	State        string                       `json:"state"`
	Capabilities map[string]map[string]string `json:"capabilities"`
}

func (p Project) ProcessID() string { return p.Capabilities["processTemplate"]["templateTypeId"] }

type Repository struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Project Project `json:"project"`
}
type WorkItem struct {
	ID        int                        `json:"id"`
	Rev       int                        `json:"rev"`
	Fields    map[string]json.RawMessage `json:"fields"`
	Relations []json.RawMessage          `json:"relations,omitempty"`
	URL       string                     `json:"url"`
	Raw       json.RawMessage            `json:"-"`
}

func (w WorkItem) Field(name string) string {
	var s string
	_ = json.Unmarshal(w.Fields[name], &s)
	return s
}
func (w WorkItem) Actor(name string) string {
	var s string
	if json.Unmarshal(w.Fields[name], &s) == nil {
		return s
	}
	var a Identity
	_ = json.Unmarshal(w.Fields[name], &a)
	return a.DisplayName
}
func (w WorkItem) Validate(project Project) error {
	if w.ID <= 0 || w.Rev <= 0 || w.Field("System.TeamProject") != project.Name || w.Field("System.WorkItemType") == "" || w.Field("System.State") == "" || w.Field("System.Title") == "" {
		return errors.New("invalid or cross-project Azure work item")
	}
	return nil
}

type Identity struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}
type Comment struct {
	ID           int             `json:"id"`
	CommentID    int             `json:"commentId,omitempty"`
	WorkItemID   int             `json:"workItemId"`
	Version      int             `json:"version"`
	Text         string          `json:"text"`
	CreatedBy    Identity        `json:"createdBy"`
	CreatedDate  string          `json:"createdDate"`
	ModifiedDate string          `json:"modifiedDate"`
	IsDeleted    bool            `json:"isDeleted"`
	Raw          json.RawMessage `json:"-"`
}

func (c Comment) Key() int {
	if c.ID != 0 {
		return c.ID
	}
	return c.CommentID
}
func (c Comment) Validate(parent int) error {
	if c.Key() <= 0 || c.Version <= 0 || c.WorkItemID != parent || c.ID != 0 && c.CommentID != 0 && c.ID != c.CommentID {
		return errors.New("invalid Azure comment identity")
	}
	return nil
}

type WorkItemType struct {
	Name          string `json:"name"`
	ReferenceName string `json:"referenceName"`
	IsDisabled    bool   `json:"isDisabled"`
	States        []struct {
		Name string `json:"name"`
	} `json:"states"`
}

func (c *Client) GetProject(ctx context.Context, org, project string) (Project, error) {
	var raw json.RawMessage
	_, e := c.json(ctx, "GET", []string{org, "_apis", "projects", project}, url.Values{"includeCapabilities": {"true"}}, false, nil, &raw, false)
	var p Project
	if e == nil {
		e = json.Unmarshal(raw, &p)
		p.Raw = raw
	}
	if e == nil && (!ValidGUID(p.ID) || p.Name == "" || p.State != "" && p.State != "wellFormed") {
		e = errors.New("invalid/unavailable Azure project identity")
	}
	return p, e
}
func (c *Client) GetRepository(ctx context.Context, org, project, repo string) (Repository, error) {
	var r Repository
	_, e := c.json(ctx, "GET", []string{org, project, "_apis", "git", "repositories", repo}, nil, false, nil, &r, false)
	if e == nil && (!ValidGUID(r.ID) || !ValidGUID(r.Project.ID)) {
		e = errors.New("invalid Azure repository identity")
	}
	return r, e
}
func (c *Client) GetType(ctx context.Context, org, project, typ string) (WorkItemType, error) {
	var t WorkItemType
	_, e := c.json(ctx, "GET", []string{org, project, "_apis", "wit", "workitemtypes", typ}, nil, false, nil, &t, false)
	if e == nil && (t.Name != typ || t.ReferenceName == "" || t.IsDisabled || len(t.States) == 0) {
		e = errors.New("Azure destination work item type unavailable")
	}
	return t, e
}
func (c *Client) GetWorkItem(ctx context.Context, org, project string, id int) (WorkItem, error) {
	if _, e := PositiveID(strconv.Itoa(id)); e != nil {
		return WorkItem{}, e
	}
	var raw json.RawMessage
	_, e := c.json(ctx, "GET", []string{org, project, "_apis", "wit", "workitems", strconv.Itoa(id)}, url.Values{"$expand": {"All"}}, false, nil, &raw, false)
	var w WorkItem
	if e == nil {
		e = json.Unmarshal(raw, &w)
		w.Raw = raw
		if w.ID != id || w.Rev <= 0 {
			e = errors.New("Azure returned different work item")
		}
	}
	return w, e
}

// ListWorkItems uses a stable ASOF and increasing immutable IDs to exhaust a
// bounded WIQL selection. A full page is never mistaken for query completion.
// Detail batches are <=200 and must return exactly the selected IDs (no Omit).
// https://learn.microsoft.com/azure/devops/boards/queries/wiql-syntax
// https://learn.microsoft.com/rest/api/azure/devops/wit/work-items/list?view=azure-devops-rest-7.1
func (c *Client) ListWorkItems(ctx context.Context, org, project string, limit int) ([]WorkItem, error) {
	if limit < 1 {
		return nil, errors.New("invalid Azure work item budget")
	}
	out := []WorkItem{}
	last := 0
	asOf := ""
	for page := 0; page < MaxPages; page++ {
		query := fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project AND [System.Id] > %d ORDER BY [System.Id] ASC", last)
		if asOf != "" {
			query += " ASOF '" + asOf + "'"
		}
		var result struct {
			QueryType string `json:"queryType"`
			AsOf      string `json:"asOf"`
			WorkItems []struct {
				ID int `json:"id"`
			} `json:"workItems"`
			WorkItemRelations json.RawMessage `json:"workItemRelations"`
			IsTruncated       bool            `json:"isTruncated"`
		}
		h, e := c.json(ctx, "POST", []string{org, project, "_apis", "wit", "wiql"}, url.Values{"$top": {strconv.Itoa(PageSize)}, "timePrecision": {"true"}}, false, map[string]string{"query": query}, &result, false)
		if e != nil {
			return nil, e
		}
		if result.WorkItems == nil || result.QueryType != "flat" || result.IsTruncated || len(result.WorkItemRelations) > 0 && string(result.WorkItemRelations) != "null" || h.Get("x-ms-continuationtoken") != "" || len(result.WorkItems) > PageSize {
			return nil, errors.New("incomplete Azure WIQL selection")
		}
		if _, e = time.Parse(time.RFC3339Nano, result.AsOf); e != nil {
			return nil, errors.New("Azure WIQL omitted valid observation time")
		}
		if asOf == "" {
			asOf = result.AsOf
		} else if result.AsOf != asOf {
			return nil, errors.New("Azure WIQL observation time changed")
		}
		if len(result.WorkItems) > limit-len(out) {
			return nil, errors.New("Azure work item selection budget exceeded")
		}
		ids := []string{}
		for _, r := range result.WorkItems {
			if r.ID <= last {
				return nil, errors.New("Azure WIQL cycle or unordered identity")
			}
			last = r.ID
			ids = append(ids, strconv.Itoa(r.ID))
		}
		if len(ids) > 0 {
			var rows struct {
				Count int               `json:"count"`
				Value []json.RawMessage `json:"value"`
			}
			_, e = c.json(ctx, "GET", []string{org, project, "_apis", "wit", "workitems"}, url.Values{"ids": {strings.Join(ids, ",")}, "asOf": {asOf}, "$expand": {"All"}, "errorPolicy": {"Fail"}}, false, nil, &rows, false)
			if e != nil {
				return nil, e
			}
			if rows.Value == nil || rows.Count != len(ids) || len(rows.Value) != len(ids) {
				return nil, errors.New("Azure work item detail batch incomplete")
			}
			wanted := map[int]bool{}
			for _, s := range ids {
				n, _ := strconv.Atoi(s)
				wanted[n] = true
			}
			for _, raw := range rows.Value {
				var w WorkItem
				if e = json.Unmarshal(raw, &w); e != nil || !wanted[w.ID] || w.Rev <= 0 {
					return nil, errors.New("Azure batch returned missing/duplicate/unselected identity")
				}
				delete(wanted, w.ID)
				w.Raw = raw
				out = append(out, w)
			}
		}
		if len(ids) < PageSize {
			return out, nil
		}
	}
	return nil, errors.New("Azure WIQL page budget exceeded")
}
func (c *Client) ListComments(ctx context.Context, org, project string, parent, limit int) ([]Comment, error) {
	if _, e := PositiveID(strconv.Itoa(parent)); e != nil {
		return nil, e
	}
	if limit < 0 {
		return nil, errors.New("invalid Azure comment budget")
	}
	out := []Comment{}
	token := ""
	seenToken := map[string]bool{}
	seenID := map[int]bool{}
	total := -1
	for page := 0; page < MaxPages; page++ {
		q := url.Values{"$top": {strconv.Itoa(PageSize)}, "includeDeleted": {"true"}, "order": {"asc"}}
		if token != "" {
			q.Set("continuationToken", token)
		}
		var result struct {
			Count             int               `json:"count"`
			TotalCount        int               `json:"totalCount"`
			Comments          []json.RawMessage `json:"comments"`
			ContinuationToken string            `json:"continuationToken"`
			NextPage          string            `json:"nextPage"`
		}
		h, e := c.json(ctx, "GET", []string{org, project, "_apis", "wit", "workItems", strconv.Itoa(parent), "comments"}, q, true, nil, &result, false)
		if e != nil {
			return nil, e
		}
		if result.Comments == nil || result.Count != len(result.Comments) || len(result.Comments) > PageSize || len(result.Comments) > limit-len(out) {
			return nil, errors.New("Azure comments truncated or budget exceeded")
		}
		if total < 0 {
			total = result.TotalCount
		} else if total != result.TotalCount {
			return nil, errors.New("Azure comments changed during observation")
		}
		for _, raw := range result.Comments {
			var v Comment
			if e = json.Unmarshal(raw, &v); e != nil {
				return nil, e
			}
			v.Raw = raw
			if e = v.Validate(parent); e != nil {
				return nil, e
			}
			if seenID[v.Key()] {
				return nil, errors.New("duplicate Azure comment")
			}
			seenID[v.Key()] = true
			out = append(out, v)
		}
		if result.NextPage != "" {
			u, e := url.Parse(result.NextPage)
			if e != nil || u.Scheme != c.base.Scheme || u.Host != c.base.Host || u.User != nil || u.Fragment != "" || u.Path != c.base.Path+"/"+org+"/"+project+"/_apis/wit/workItems/"+strconv.Itoa(parent)+"/comments" || u.Query().Get("api-version") != c.commentsVersion || u.Query().Get("continuationToken") != result.ContinuationToken {
				return nil, errors.New("invalid Azure comment next-page identity/cursor")
			}
		}
		next := result.ContinuationToken
		header := h.Get("x-ms-continuationtoken")
		if next != "" && header != "" && next != header {
			return nil, errors.New("conflicting Azure comment cursors")
		}
		if next == "" {
			next = header
		}
		if next == "" {
			if result.NextPage != "" {
				return nil, errors.New("Azure comments next page omitted continuation token")
			}
			if total != len(out) {
				return nil, errors.New("Azure comment enumeration incomplete")
			}
			return out, nil
		}
		if len(result.Comments) == 0 || len(next) > 4096 || seenToken[next] {
			return nil, errors.New("Azure comment cursor cycle")
		}
		seenToken[next] = true
		token = next
	}
	return nil, errors.New("Azure comments page budget exceeded")
}

type Patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}
type WorkItemInput struct {
	Title, Body, Type, State string
	Labels, Assignees        []string
}

func (c *Client) WriteWorkItem(ctx context.Context, org, project string, id, rev int, in WorkItemInput) (WorkItem, error) {
	if !ValidSegment(in.Type) || in.Title == "" || len(in.Title) > 255 || len(in.Body) > 65536 || in.State == "" || len(in.Labels) != 0 || len(in.Assignees) != 0 {
		return WorkItem{}, errors.New("Azure supports explicit title/description/type/state only")
	}
	ops := []Patch{}
	method := "POST"
	target := "$" + in.Type
	if id != 0 {
		if id < 1 || rev < 1 {
			return WorkItem{}, errors.New("Azure update requires ID and revision")
		}
		method = "PATCH"
		target = strconv.Itoa(id)
		ops = append(ops, Patch{"test", "/rev", rev}, Patch{"test", "/fields/System.WorkItemType", in.Type})
	}
	ops = append(ops, Patch{"add", "/fields/System.Title", in.Title}, Patch{"add", "/fields/System.Description", in.Body}, Patch{"add", "/fields/System.State", in.State})
	var raw json.RawMessage
	_, e := c.json(ctx, method, []string{org, project, "_apis", "wit", "workitems", target}, url.Values{"$expand": {"All"}}, false, ops, &raw, true)
	var w WorkItem
	if e == nil {
		e = json.Unmarshal(raw, &w)
		w.Raw = raw
		if w.ID < 1 || w.Rev < 1 || id != 0 && w.ID != id {
			e = errors.New("Azure write returned different identity")
		}
	}
	return w, e
}
func (c *Client) CreateComment(ctx context.Context, org, project string, parent int, body string) (Comment, error) {
	if parent < 1 || len(body) > 65536 {
		return Comment{}, errors.New("invalid Azure comment input")
	}
	var raw json.RawMessage
	_, e := c.json(ctx, "POST", []string{org, project, "_apis", "wit", "workItems", strconv.Itoa(parent), "comments"}, nil, true, map[string]string{"text": body}, &raw, false)
	var v Comment
	if e == nil {
		e = json.Unmarshal(raw, &v)
		v.Raw = raw
		if e == nil {
			e = v.Validate(parent)
		}
	}
	return v, e
}

// PR lists use documented skip/top; the threads endpoint returns an entire
// collection, without a documented continuation parameter. Unexpected cursors
// on that endpoint refuse incomplete success rather than inventing pagination.
// https://learn.microsoft.com/rest/api/azure/devops/git/pull-requests/get-pull-requests?view=azure-devops-rest-7.1
// https://learn.microsoft.com/rest/api/azure/devops/git/pull-request-threads/list?view=azure-devops-rest-7.1
func (c *Client) ListPullRequests(ctx context.Context, org, project, repo string, limit int) ([]json.RawMessage, error) {
	out := []json.RawMessage{}
	seen := map[int]bool{}
	for page := 0; page < MaxPages; page++ {
		var result struct {
			Count int               `json:"count"`
			Value []json.RawMessage `json:"value"`
		}
		h, e := c.json(ctx, "GET", []string{org, project, "_apis", "git", "repositories", repo, "pullrequests"}, url.Values{"searchCriteria.status": {"all"}, "$top": {strconv.Itoa(PageSize)}, "$skip": {strconv.Itoa(page * PageSize)}}, false, nil, &result, false)
		if e != nil {
			return nil, e
		}
		if result.Value == nil || result.Count != len(result.Value) || len(result.Value) > PageSize || len(result.Value) > limit-len(out) || h.Get("x-ms-continuationtoken") != "" {
			return nil, errors.New("Azure PR list incomplete or budget exceeded")
		}
		for _, raw := range result.Value {
			var v struct {
				ID         int        `json:"pullRequestId"`
				Repository Repository `json:"repository"`
			}
			if json.Unmarshal(raw, &v) != nil || v.ID <= 0 || seen[v.ID] || v.Repository.ID != repo || v.Repository.Project.ID != project {
				return nil, errors.New("Azure PR identity mismatch or duplicate")
			}
			seen[v.ID] = true
			// List responses truncate descriptions to 400 characters. Retrieve
			// each selected PR detail before claiming source preservation.
			detail, e := c.getPullRequest(ctx, org, project, repo, v.ID)
			if e != nil {
				return nil, e
			}
			out = append(out, detail)
		}
		if len(result.Value) < PageSize {
			return out, nil
		}
	}
	return nil, errors.New("Azure PR page budget exceeded")
}
func (c *Client) ListThreads(ctx context.Context, org, project, repo string, pr int, limit int) ([]json.RawMessage, error) {
	var result struct {
		Count int               `json:"count"`
		Value []json.RawMessage `json:"value"`
	}
	h, e := c.json(ctx, "GET", []string{org, project, "_apis", "git", "repositories", repo, "pullRequests", strconv.Itoa(pr), "threads"}, nil, false, nil, &result, false)
	if e != nil {
		return nil, e
	}
	if result.Value == nil || result.Count != len(result.Value) || len(result.Value) > limit || h.Get("x-ms-continuationtoken") != "" {
		return nil, errors.New("Azure thread collection incomplete, unsupported cursor or budget exceeded")
	}
	seen := map[int]bool{}
	for _, raw := range result.Value {
		var v struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(raw, &v) != nil || v.ID <= 0 || seen[v.ID] {
			return nil, errors.New("invalid or duplicate Azure thread")
		}
		seen[v.ID] = true
	}
	return result.Value, nil
}

// YAML is selected by exact existing commit and path, never executed. Classic
// definitions are selected by exact numeric revision; callers redact secret
// variable values before preservation and never adopt queues/permissions.
func (c *Client) ReadYAML(ctx context.Context, org, project, repo, commit, filename string, limit int64) ([]byte, string, error) {
	if len(commit) != 40 || strings.ToLower(commit) != commit {
		return nil, "", errors.New("Azure YAML requires exact SHA1 commit")
	}
	if _, e := hex.DecodeString(commit); e != nil {
		return nil, "", e
	}
	if !strings.HasPrefix(filename, "/") || path.Clean(filename) != filename || strings.ContainsAny(filename, "\\\x00\r\n") {
		return nil, "", errors.New("invalid Azure YAML path")
	}
	u, e := c.endpoint([]string{org, project, "_apis", "git", "repositories", repo, "items"}, url.Values{"path": {filename}, "download": {"true"}, "versionDescriptor.versionType": {"commit"}, "versionDescriptor.version": {commit}}, false)
	if e != nil {
		return nil, "", e
	}
	return c.Download(ctx, u, limit)
}
func (c *Client) ReadDefinition(ctx context.Context, org, project string, id, revision int) (json.RawMessage, error) {
	if id <= 0 || revision <= 0 {
		return nil, errors.New("Azure classic definition requires ID and revision")
	}
	var raw json.RawMessage
	_, e := c.json(ctx, "GET", []string{org, project, "_apis", "build", "definitions", strconv.Itoa(id)}, url.Values{"revision": {strconv.Itoa(revision)}}, false, nil, &raw, false)
	if e != nil {
		return nil, e
	}
	var v struct {
		ID       int `json:"id"`
		Revision int `json:"revision"`
		Process  struct {
			Type int `json:"type"`
		} `json:"process"`
	}
	if json.Unmarshal(raw, &v) != nil || v.ID != id || v.Revision != revision || v.Process.Type != 1 {
		return nil, errors.New("Azure classic definition revision/type differs")
	}
	return raw, nil
}

func (c *Client) getPullRequest(ctx context.Context, org, project, repo string, id int) (json.RawMessage, error) {
	var raw json.RawMessage
	_, e := c.json(ctx, "GET", []string{org, project, "_apis", "git", "repositories", repo, "pullRequests", strconv.Itoa(id)}, nil, false, nil, &raw, false)
	if e != nil {
		return nil, e
	}
	var v struct {
		ID         int        `json:"pullRequestId"`
		Repository Repository `json:"repository"`
	}
	if json.Unmarshal(raw, &v) != nil || v.ID != id || v.Repository.ID != repo || v.Repository.Project.ID != project {
		return nil, errors.New("Azure PR detail identity changed")
	}
	return raw, nil
}

// SanitizeDefinition follows the qualified BuildDefinition schema, not a
// heuristic scan of user-authored task/YAML strings. Secret-marked variables
// occur at variables and variableGroups[].variables; service authorization
// values occur at processParameters.dataSourceBindings[].headers[].value.
// https://learn.microsoft.com/rest/api/azure/devops/build/definitions/get?view=azure-devops-rest-7.1
// Returned bytes (and only these bytes) may enter preservation digests.
func SanitizeDefinition(raw json.RawMessage) (json.RawMessage, int, error) {
	if e := definitionUniqueJSON(raw); e != nil {
		return nil, 0, e
	}
	object, e := definitionObject(raw, "variables", "variableGroups", "processParameters")
	if e != nil {
		return nil, 0, e
	}
	excluded := 0
	redactVariables := func(container map[string]json.RawMessage) error {
		variables, present := container["variables"]
		if !present || string(bytes.TrimSpace(variables)) == "null" {
			return nil
		}
		var values map[string]json.RawMessage
		if json.Unmarshal(variables, &values) != nil || values == nil {
			return errors.New("invalid Azure classic variable collection")
		}
		names := map[string]bool{}
		for name, value := range values {
			lower := strings.ToLower(name)
			if names[lower] {
				return errors.New("ambiguous Azure classic variable names")
			}
			names[lower] = true
			v, e := definitionObject(value, "isSecret", "value")
			if e != nil {
				return e
			}
			secret := false
			if flag, ok := v["isSecret"]; ok {
				switch string(bytes.TrimSpace(flag)) {
				case "true":
					secret = true
				case "false":
				default:
					return errors.New("Azure classic isSecret must be an unambiguous boolean")
				}
			}
			if secret {
				if _, ok := v["value"]; ok {
					delete(v, "value")
					excluded++
				}
			}
			values[name], e = json.Marshal(v)
			if e != nil {
				return e
			}
		}
		container["variables"], e = json.Marshal(values)
		return e
	}
	if e = redactVariables(object); e != nil {
		return nil, 0, e
	}
	if groups, present := object["variableGroups"]; present && string(bytes.TrimSpace(groups)) != "null" {
		var rows []json.RawMessage
		if json.Unmarshal(groups, &rows) != nil || rows == nil {
			return nil, 0, errors.New("invalid Azure classic variable groups")
		}
		for i, row := range rows {
			group, e := definitionObject(row, "variables")
			if e != nil {
				return nil, 0, e
			}
			if e = redactVariables(group); e != nil {
				return nil, 0, e
			}
			rows[i], e = json.Marshal(group)
			if e != nil {
				return nil, 0, e
			}
		}
		object["variableGroups"], e = json.Marshal(rows)
		if e != nil {
			return nil, 0, e
		}
	}
	if parameters, present := object["processParameters"]; present && string(bytes.TrimSpace(parameters)) != "null" {
		params, e := definitionObject(parameters, "dataSourceBindings")
		if e != nil {
			return nil, 0, e
		}
		if bindings, present := params["dataSourceBindings"]; present && string(bytes.TrimSpace(bindings)) != "null" {
			var rows []json.RawMessage
			if json.Unmarshal(bindings, &rows) != nil || rows == nil {
				return nil, 0, errors.New("invalid Azure classic data source bindings")
			}
			for i, row := range rows {
				binding, e := definitionObject(row, "headers")
				if e != nil {
					return nil, 0, e
				}
				if headers, present := binding["headers"]; present && string(bytes.TrimSpace(headers)) != "null" {
					var entries []json.RawMessage
					if json.Unmarshal(headers, &entries) != nil || entries == nil {
						return nil, 0, errors.New("invalid Azure classic authorization headers")
					}
					for j, entry := range entries {
						header, e := definitionObject(entry, "name", "value")
						if e != nil {
							return nil, 0, e
						}
						var name string
						if json.Unmarshal(header["name"], &name) != nil || name == "" {
							return nil, 0, errors.New("invalid Azure classic authorization header name")
						}
						if _, present := header["value"]; present {
							delete(header, "value")
							excluded++
						}
						entries[j], e = json.Marshal(header)
						if e != nil {
							return nil, 0, e
						}
					}
					binding["headers"], e = json.Marshal(entries)
					if e != nil {
						return nil, 0, e
					}
				}
				rows[i], e = json.Marshal(binding)
				if e != nil {
					return nil, 0, e
				}
			}
			params["dataSourceBindings"], e = json.Marshal(rows)
			if e != nil {
				return nil, 0, e
			}
		}
		object["processParameters"], e = json.Marshal(params)
		if e != nil {
			return nil, 0, e
		}
	}
	sanitized, e := json.Marshal(object)
	return sanitized, excluded, e
}
func definitionObject(raw json.RawMessage, reserved ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, errors.New("invalid Azure classic definition object")
	}
	for name := range object {
		for _, key := range reserved {
			if strings.EqualFold(name, key) && name != key {
				return nil, errors.New("ambiguous Azure classic credential field casing")
			}
		}
	}
	return object, nil
}

// Duplicate keys anywhere in a definition are refused before map decoding can
// erase a contradictory secret flag/value. Null is allowed for ordinary API
// metadata; isSecret is validated separately as exactly true or false.
func definitionUniqueJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid Azure classic JSON encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 64 {
			return errors.New("Azure classic JSON nesting budget exceeded")
		}
		token, e := decoder.Token()
		if e != nil {
			return errors.New("invalid Azure classic JSON")
		}
		switch token {
		case json.Delim('{'):
			names := map[string]bool{}
			for decoder.More() {
				key, e := decoder.Token()
				if e != nil {
					return errors.New("invalid Azure classic JSON key")
				}
				name, ok := key.(string)
				if !ok || names[name] {
					return errors.New("duplicate Azure classic JSON key")
				}
				names[name] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
			end, e := decoder.Token()
			if e != nil || end != json.Delim('}') {
				return errors.New("invalid Azure classic JSON object")
			}
		case json.Delim('['):
			for decoder.More() {
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
			end, e := decoder.Token()
			if e != nil || end != json.Delim(']') {
				return errors.New("invalid Azure classic JSON array")
			}
		case json.Delim('}'), json.Delim(']'):
			return errors.New("invalid Azure classic JSON delimiter")
		}
		return nil
	}
	if e := walk(0); e != nil {
		return e
	}
	if _, e := decoder.Token(); e != io.EOF {
		return errors.New("trailing Azure classic JSON")
	}
	return nil
}
