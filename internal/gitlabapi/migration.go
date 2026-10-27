//go:build !noforge

package gitlabapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Project struct {
	ID            int64           `json:"id"`
	Path          string          `json:"path_with_namespace"`
	CreatedAt     string          `json:"created_at"`
	DefaultBranch string          `json:"default_branch"`
	IssuesEnabled bool            `json:"issues_enabled"`
	CIConfigPath  string          `json:"ci_config_path"`
	Raw           json.RawMessage `json:"-"`
}

func (c *Client) GetProject(ctx context.Context, p string) (Project, error) {
	u, e := c.projectURL(p, "")
	if e != nil {
		return Project{}, e
	}
	raw, _, e := c.request(ctx, "GET", u, nil, 200, MaxResponseBytes)
	if e != nil {
		return Project{}, e
	}
	var v Project
	if e = decode(raw, &v); e != nil || v.ID <= 0 || ValidateProject(v.Path) != nil {
		return Project{}, errors.New("invalid GitLab project")
	}
	if _, e = time.Parse(time.RFC3339Nano, v.CreatedAt); e != nil {
		return Project{}, errors.New("invalid GitLab project creation revision")
	}
	if n, e := strconv.ParseInt(p, 10, 64); e == nil && n != v.ID {
		return Project{}, errors.New("GitLab returned different project ID")
	}
	if strings.Contains(p, "/") && p != v.Path {
		return Project{}, errors.New("GitLab project path redirected or replaced")
	}
	v.Raw = append([]byte{}, raw...)
	return v, nil
}

// Object retains untouched API bytes and the metadata needed to link foreign
// records. Approval state/users, diff positions and board labels remain inert.
type Object struct {
	ID        int64           `json:"id"`
	IID       int             `json:"iid"`
	ProjectID int64           `json:"project_id"`
	WebURL    string          `json:"web_url"`
	Author    User            `json:"author"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	Raw       json.RawMessage `json:"-"`
}

func (v *Object) UnmarshalJSON(b []byte) error {
	type dto Object
	var x dto
	if e := json.Unmarshal(b, &x); e != nil {
		return e
	}
	*v = Object(x)
	v.Raw = append([]byte{}, b...)
	return nil
}

type Discussion struct {
	ID         string          `json:"id"`
	Individual bool            `json:"individual_note"`
	Notes      []Note          `json:"notes"`
	Raw        json.RawMessage `json:"-"`
}

func (v *Discussion) UnmarshalJSON(b []byte) error {
	type dto Discussion
	var x dto
	if e := json.Unmarshal(b, &x); e != nil {
		return e
	}
	*v = Discussion(x)
	v.Raw = append([]byte{}, b...)
	return nil
}
func (c *Client) ListMergeRequests(ctx context.Context, p string) ([]Object, error) {
	u, e := c.projectURL(p, "/merge_requests")
	if e != nil {
		return nil, e
	}
	u.RawQuery = "scope=all&state=all"
	return list(ctx, c, u, func(v Object) bool { return v.ID > 0 && v.IID > 0 && v.ProjectID > 0 }, func(v Object) string { return strconv.FormatInt(v.ID, 10) })
}
func (c *Client) ListDiscussions(ctx context.Context, p, resource string, iid int) ([]Discussion, error) {
	if resource != "issues" && resource != "merge_requests" || iid <= 0 {
		return nil, errors.New("invalid discussion parent")
	}
	u, e := c.projectURL(p, "/"+resource+"/"+strconv.Itoa(iid)+"/discussions")
	if e != nil {
		return nil, e
	}
	return list(ctx, c, u, func(v Discussion) bool {
		if v.ID == "" || len(v.Notes) == 0 {
			return false
		}
		seen := map[int64]bool{}
		for _, n := range v.Notes {
			if n.ID <= 0 || seen[n.ID] {
				return false
			}
			seen[n.ID] = true
		}
		return true
	}, func(v Discussion) string { return v.ID })
}
func (c *Client) ReadApprovals(ctx context.Context, p string, iid int) ([]byte, error) {
	if iid <= 0 {
		return nil, errors.New("invalid MR IID")
	}
	u, e := c.projectURL(p, "/merge_requests/"+strconv.Itoa(iid)+"/approvals")
	if e != nil {
		return nil, e
	}
	b, _, e := c.request(ctx, "GET", u, nil, 200, MaxResponseBytes)
	if e != nil {
		return nil, e
	}
	var v map[string]json.RawMessage
	if e = decode(b, &v); e != nil || v == nil {
		return nil, errors.New("invalid approvals observation")
	}
	return b, nil
}
func (c *Client) ListBoards(ctx context.Context, p string) ([]Object, error) {
	u, e := c.projectURL(p, "/boards")
	if e != nil {
		return nil, e
	}
	return list(ctx, c, u, func(v Object) bool { return v.ID > 0 }, func(v Object) string { return strconv.FormatInt(v.ID, 10) })
}
func (c *Client) ListBoardLists(ctx context.Context, p string, id int) ([]Object, error) {
	if id <= 0 {
		return nil, errors.New("invalid board ID")
	}
	u, e := c.projectURL(p, "/boards/"+strconv.Itoa(id)+"/lists")
	if e != nil {
		return nil, e
	}
	return list(ctx, c, u, func(v Object) bool { return v.ID > 0 }, func(v Object) string { return strconv.FormatInt(v.ID, 10) })
}

type CISource struct {
	Commit  string
	Path    string
	Content []byte
	Raw     json.RawMessage
}

func (c *Client) ReadCI(ctx context.Context, p, ref string) (CISource, error) {
	if ref == "" || len(ref) > 1024 || strings.ContainsAny(ref, "\x00\r\n") {
		return CISource{}, errors.New("explicit CI revision or default branch required")
	}
	u, e := c.projectURL(p, "/repository/commits/"+url.PathEscape(ref))
	if e != nil {
		return CISource{}, e
	}
	b, _, e := c.request(ctx, "GET", u, nil, 200, MaxResponseBytes)
	if e != nil {
		return CISource{}, e
	}
	var commit struct {
		ID string `json:"id"`
	}
	if e = decode(b, &commit); e != nil || !gitOID(commit.ID) {
		return CISource{}, errors.New("invalid CI commit revision")
	}
	u, e = c.projectURL(p, "/repository/files/"+url.PathEscape(".gitlab-ci.yml"))
	if e != nil {
		return CISource{}, e
	}
	q := u.Query()
	q.Set("ref", commit.ID)
	u.RawQuery = q.Encode()
	b, _, e = c.request(ctx, "GET", u, nil, 200, MaxResponseBytes)
	if e != nil {
		return CISource{}, e
	}
	var file struct {
		Path     string `json:"file_path"`
		Commit   string `json:"commit_id"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
		SHA256   string `json:"content_sha256"`
	}
	if e = decode(b, &file); e != nil || file.Path != ".gitlab-ci.yml" || file.Commit != commit.ID || file.Encoding != "base64" {
		return CISource{}, errors.New("invalid pinned CI file observation")
	}
	content, e := base64.StdEncoding.DecodeString(file.Content)
	if e != nil {
		return CISource{}, errors.New("invalid CI content encoding")
	}
	digest := sha256.Sum256(content)
	if file.SHA256 != "" && file.SHA256 != hex.EncodeToString(digest[:]) {
		return CISource{}, errors.New("CI source digest mismatch")
	}
	return CISource{commit.ID, file.Path, content, append([]byte{}, b...)}, nil
}
func gitOID(s string) bool {
	if len(s) != 40 && len(s) != 64 || s != strings.ToLower(s) {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil
}
func (c *Client) ReadAttachment(ctx context.Context, rawURL string, max int64) ([]byte, string, error) {
	u, e := url.Parse(rawURL)
	if e != nil || u.Scheme != c.base.Scheme || u.Host != c.base.Host || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(rawURL, "#") || max <= 0 {
		return nil, "", errors.New("GitLab attachment requires bounded same-origin URL")
	}
	if max > 16<<20 {
		max = 16 << 20
	}
	b, h, e := c.request(ctx, "GET", u, nil, 200, max)
	return b, h.Get("Content-Type"), e
}
