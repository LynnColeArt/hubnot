//go:build !nogithub

package githubapi

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxWorkflowBytes = 1 << 20
const MaxWorkflows = 256

type Workflow struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	HTMLURL   string `json:"html_url"`
}
type WorkflowCommit struct{ SHA, TreeSHA string }
type WorkflowFile struct {
	Path, SHA, Commit string
	Content           []byte
}
type WorkflowDefinition struct {
	Workflow Workflow
	File     WorkflowFile
}
type WorkflowFileInput struct {
	Path, Branch, ExpectedSHA, Message string
	Content                            []byte
}
type workflowBudget struct{ bytes int }

func (c *Client) workflowRequest(ctx context.Context, method string, u *url.URL, input any, want int, b *workflowBudget) ([]byte, http.Header, error) {
	raw, h, err := c.request(ctx, method, u, input, want)
	if err != nil {
		return nil, nil, err
	}
	b.bytes += len(raw)
	if b.bytes > MaxTotalBytes {
		return nil, nil, errors.New("workflow read exceeds 64 MiB aggregate limit")
	}
	return raw, h, nil
}
func workflowDecode[T any](raw []byte) (T, error) {
	var zero T
	if !validJSONUnicode(raw) {
		return zero, errors.New("invalid workflow response JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return errors.New("workflow response nesting limit")
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				s, ok := k.(string)
				if !ok || len(seen) > 4096 {
					return errors.New("workflow response object limit")
				}
				var folded strings.Builder
				for _, r := range s {
					least := r
					for x := unicode.SimpleFold(r); x != r; x = unicode.SimpleFold(x) {
						if x < least {
							least = x
						}
					}
					folded.WriteRune(least)
				}
				key := folded.String()
				if seen[key] {
					return errors.New("duplicate workflow response key")
				}
				seen[key] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid workflow JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return zero, errors.New("invalid workflow response keys or bounds")
	}
	if _, err := d.Token(); err != io.EOF {
		return zero, errors.New("trailing workflow response JSON")
	}
	return decodeResponse[T](raw)
}
func workflowHex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func workflowSafeRef(s string) bool {
	if s == "" || len(s) > 256 || !utf8.ValidString(s) || strings.HasPrefix(s, "-") || strings.Contains(s, "..") || strings.Contains(s, "@{") || strings.HasSuffix(s, "/") || strings.HasSuffix(s, ".") {
		return false
	}
	for _, r := range s {
		if r <= 32 || r == 127 || strings.ContainsRune("~^:?*[\\", r) {
			return false
		}
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".lock") {
			return false
		}
	}
	return true
}

// ValidWorkflowPath deliberately excludes nested paths, traversal and alternate formats.
func ValidWorkflowPath(p string) bool {
	const prefix = ".github/workflows/"
	if !strings.HasPrefix(p, prefix) || len(p) > 220 {
		return false
	}
	n := strings.TrimPrefix(p, prefix)
	if !strings.HasSuffix(n, ".yml") && !strings.HasSuffix(n, ".yaml") {
		return false
	}
	for _, r := range n {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return n != "" && !strings.HasPrefix(n, ".") && !strings.Contains(n, "..")
}
func (c *Client) workflowURL(repo, suffix string) (*url.URL, error) {
	if !validRepo(repo) {
		return nil, errors.New("workflow repository must be OWNER/NAME")
	}
	u := *c.base
	u.Path += "/repos/" + repo + suffix
	return &u, nil
}
func (c *Client) ResolveWorkflowCommit(ctx context.Context, repo, ref string) (WorkflowCommit, error) {
	return c.resolveWorkflowCommit(ctx, repo, ref, &workflowBudget{})
}
func (c *Client) resolveWorkflowCommit(ctx context.Context, repo, ref string, b *workflowBudget) (WorkflowCommit, error) {
	if ref != "" && !workflowSafeRef(ref) {
		return WorkflowCommit{}, errors.New("invalid workflow source ref")
	}
	u, err := c.workflowURL(repo, "/commits")
	if err != nil {
		return WorkflowCommit{}, err
	}
	q := u.Query()
	q.Set("per_page", "1")
	if ref != "" {
		q.Set("sha", ref)
	}
	u.RawQuery = q.Encode()
	raw, _, err := c.workflowRequest(ctx, http.MethodGet, u, nil, 200, b)
	if err != nil {
		return WorkflowCommit{}, err
	}
	rows, err := workflowDecode[[]struct {
		SHA    string `json:"sha"`
		Commit struct {
			Tree struct {
				SHA string `json:"sha"`
			} `json:"tree"`
		} `json:"commit"`
	}](raw)
	if err != nil || len(rows) != 1 || !workflowHex(rows[0].SHA) || !workflowHex(rows[0].Commit.Tree.SHA) {
		return WorkflowCommit{}, errors.New("invalid workflow commit identity")
	}
	if workflowHex(ref) && rows[0].SHA != ref {
		return WorkflowCommit{}, errors.New("resolved workflow commit differs from explicitly pinned source SHA")
	}
	return WorkflowCommit{rows[0].SHA, rows[0].Commit.Tree.SHA}, nil
}
func (c *Client) ListWorkflows(ctx context.Context, repo string) ([]Workflow, error) {
	return c.listWorkflows(ctx, repo, &workflowBudget{})
}
func (c *Client) listWorkflows(ctx context.Context, repo string, b *workflowBudget) ([]Workflow, error) {
	u, err := c.workflowURL(repo, "/actions/workflows")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("per_page", "100")
	q.Set("page", "1")
	u.RawQuery = q.Encode()
	rows := []Workflow{}
	seen := map[int64]bool{}
	paths := map[string]bool{}
	for page := 1; page <= MaxPages; page++ {
		raw, h, err := c.workflowRequest(ctx, http.MethodGet, u, nil, 200, b)
		if err != nil {
			return nil, err
		}
		v, err := workflowDecode[struct {
			Total     int        `json:"total_count"`
			Workflows []Workflow `json:"workflows"`
		}](raw)
		if err != nil || v.Total < 0 || v.Total > MaxWorkflows || v.Workflows == nil || len(v.Workflows) > 100 {
			return nil, errors.New("invalid or oversized workflow listing")
		}
		for _, w := range v.Workflows {
			if w.ID <= 0 || w.NodeID == "" || len(w.NodeID) > 256 || !ValidWorkflowPath(w.Path) || w.Name == "" || len(w.Name) > 1024 || len(w.CreatedAt) > 64 || len(w.UpdatedAt) > 64 || len(w.HTMLURL) > 2048 || seen[w.ID] || paths[w.Path] {
				return nil, errors.New("invalid or duplicate workflow identity/path")
			}
			seen[w.ID] = true
			paths[w.Path] = true
			rows = append(rows, w)
		}
		if len(rows) > MaxWorkflows {
			return nil, errors.New("workflow listing exceeds 256 definitions")
		}
		next, err := nextPage(h.Values("Link"), u, page)
		if err != nil {
			return nil, err
		}
		if next == nil {
			if len(rows) != v.Total {
				return nil, errors.New("workflow listing changed or incomplete")
			}
			return rows, nil
		}
		u = next
	}
	return nil, errors.New("workflow listing page limit")
}
func (c *Client) GetWorkflowFile(ctx context.Context, repo, path string, pin WorkflowCommit) (WorkflowFile, error) {
	return c.getWorkflowFile(ctx, repo, path, pin, &workflowBudget{})
}
func (c *Client) getWorkflowFile(ctx context.Context, repo, path string, pin WorkflowCommit, b *workflowBudget) (WorkflowFile, error) {
	var zero WorkflowFile
	if !ValidWorkflowPath(path) || !workflowHex(pin.SHA) || !workflowHex(pin.TreeSHA) {
		return zero, errors.New("invalid workflow path or pinned commit")
	}
	tree := pin.TreeSHA
	var blob string
	for i, part := range strings.Split(path, "/") {
		u, err := c.workflowURL(repo, "/git/trees/"+tree)
		if err != nil {
			return zero, err
		}
		raw, _, err := c.workflowRequest(ctx, "GET", u, nil, 200, b)
		if err != nil {
			return zero, err
		}
		v, err := workflowDecode[struct {
			Truncated bool                                     `json:"truncated"`
			Tree      []struct{ Path, Mode, Type, SHA string } `json:"tree"`
		}](raw)
		if err != nil || v.Truncated || v.Tree == nil || len(v.Tree) > 1000 {
			return zero, errors.New("workflow tree is truncated or exceeds 1000 entries")
		}
		found := false
		seen := map[string]bool{}
		for _, e := range v.Tree {
			if seen[e.Path] {
				return zero, errors.New("duplicate workflow tree path")
			}
			seen[e.Path] = true
			if e.Path != part {
				continue
			}
			if !workflowHex(e.SHA) {
				return zero, errors.New("invalid workflow tree identity")
			}
			if i < 2 {
				if e.Mode != "040000" || e.Type != "tree" {
					return zero, errors.New("workflow path traverses non-directory or symlink")
				}
				tree = e.SHA
			} else {
				if (e.Mode != "100644" && e.Mode != "100755") || e.Type != "blob" {
					return zero, errors.New("workflow definition is not a regular Git blob")
				}
				blob = e.SHA
			}
			found = true
		}
		if !found {
			return zero, &HTTPError{StatusCode: 404}
		}
	}
	u, err := c.workflowURL(repo, "/contents/"+path)
	if err != nil {
		return zero, err
	}
	q := u.Query()
	q.Set("ref", pin.SHA)
	u.RawQuery = q.Encode()
	raw, _, err := c.workflowRequest(ctx, "GET", u, nil, 200, b)
	if err != nil {
		return zero, err
	}
	v, err := workflowDecode[struct {
		Type, Path, SHA, Encoding, Content string
		Size                               int
		SubmoduleGitURL                    string `json:"submodule_git_url"`
	}](raw)
	if err != nil || v.Type != "file" || v.Path != path || v.SHA != blob || v.Encoding != "base64" || v.SubmoduleGitURL != "" || v.Size < 1 || v.Size > MaxWorkflowBytes {
		return zero, errors.New("invalid workflow contents shape or size")
	}
	content, err := base64.StdEncoding.Strict().DecodeString(strings.ReplaceAll(v.Content, "\n", ""))
	if err != nil || len(content) != v.Size || !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 || workflowBlobSHA(content) != blob {
		return zero, errors.New("workflow content does not match exact UTF-8 Git blob")
	}
	return WorkflowFile{path, blob, pin.SHA, content}, nil
}

// ReadWorkflowDefinitions shares one response budget and one immutable source commit.
func (c *Client) ReadWorkflowDefinitions(ctx context.Context, repo, ref, key string) ([]WorkflowDefinition, error) {
	b := &workflowBudget{}
	pin, err := c.resolveWorkflowCommit(ctx, repo, ref, b)
	if err != nil {
		return nil, err
	}
	rows, err := c.listWorkflows(ctx, repo, b)
	if err != nil {
		return nil, err
	}
	out := []WorkflowDefinition{}
	bytes := 0
	found := key == ""
	for _, w := range rows {
		if key != "" && key != strconv.FormatInt(w.ID, 10) && key != w.Path {
			continue
		}
		found = true
		file, err := c.getWorkflowFile(ctx, repo, w.Path, pin, b)
		if err != nil {
			return nil, err
		}
		bytes += len(file.Content)
		if bytes > 8<<20 {
			return nil, errors.New("workflow source graph exceeds 8 MiB")
		}
		out = append(out, WorkflowDefinition{w, file})
	}
	if !found {
		return nil, errors.New("selected workflow not found")
	}
	return out, nil
}
func workflowBlobSHA(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	_, _ = h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}
func (c *Client) PutWorkflowFile(ctx context.Context, repo string, input WorkflowFileInput) (WorkflowFile, error) {
	var zero WorkflowFile
	if !ValidWorkflowPath(input.Path) || !workflowSafeRef(input.Branch) || input.ExpectedSHA != "" && !workflowHex(input.ExpectedSHA) || input.Message == "" || len(input.Message) > 1024 || !utf8.ValidString(input.Message) || strings.ContainsRune(input.Message, 0) || len(input.Content) == 0 || len(input.Content) > MaxWorkflowBytes || !utf8.Valid(input.Content) || bytes.IndexByte(input.Content, 0) >= 0 {
		return zero, errors.New("invalid workflow PUT path/branch/SHA/content")
	}
	u, err := c.workflowURL(repo, "/contents/"+input.Path)
	if err != nil {
		return zero, err
	}
	body := struct {
		Message string `json:"message"`
		Content string `json:"content"`
		Branch  string `json:"branch"`
		SHA     string `json:"sha,omitempty"`
	}{input.Message, base64.StdEncoding.EncodeToString(input.Content), input.Branch, input.ExpectedSHA}
	want := 200
	if input.ExpectedSHA == "" {
		want = 201
	}
	raw, _, err := c.workflowRequest(ctx, "PUT", u, body, want, &workflowBudget{})
	if err != nil {
		return zero, err
	}
	v, err := workflowDecode[struct {
		Content struct{ Path, SHA string } `json:"content"`
		Commit  struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}](raw)
	if err != nil || v.Content.Path != input.Path || v.Content.SHA != workflowBlobSHA(input.Content) || !workflowHex(v.Commit.SHA) {
		return zero, errors.New("workflow PUT result differs from intended blob; outcome may be unknown")
	}
	return WorkflowFile{input.Path, v.Content.SHA, v.Commit.SHA, input.Content}, nil
}
