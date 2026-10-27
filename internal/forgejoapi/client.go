//go:build !noforge

// Package forgejoapi implements bounded, explicit /api/v1 transport. The common
// endpoints used by Gitea are verified against its own pinned Swagger schemas;
// profiles retain independent version checks and workflow directory semantics.
package forgejoapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const MaxResponseBytes int64 = 4 << 20
const MaxTotalBytes int64 = 64 << 20
const MaxPages = 100
const BodyCapacity = 65536 // Conservative tested profile ceiling, no truncation.

type User struct {
	Login string `json:"login"`
}
type Label struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type Repository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	CreatedAt     string `json:"created_at"`
	DefaultBranch string `json:"default_branch"`
	HasIssues     bool   `json:"has_issues"`
}
type Issue struct {
	ContentVersion int64           `json:"content_version,omitempty"`
	ID             int64           `json:"id"`
	Number         int             `json:"number"`
	Title          string          `json:"title"`
	Body           string          `json:"body"`
	State          string          `json:"state"`
	HTMLURL        string          `json:"html_url"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
	User           User            `json:"user"`
	Labels         []Label         `json:"labels"`
	Assignees      []User          `json:"assignees"`
	PullRequest    json.RawMessage `json:"pull_request,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

func (v *Issue) UnmarshalJSON(raw []byte) error {
	type plain Issue
	var p plain
	if e := json.Unmarshal(raw, &p); e != nil {
		return e
	}
	*v = Issue(p)
	v.Raw = append([]byte{}, raw...)
	return nil
}
func (v Issue) IsPull() bool { return len(v.PullRequest) > 0 && string(v.PullRequest) != "null" }

type Comment struct {
	ID        int64           `json:"id"`
	Body      string          `json:"body"`
	User      User            `json:"user"`
	HTMLURL   string          `json:"html_url"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	Raw       json.RawMessage `json:"-"`
}

func (v *Comment) UnmarshalJSON(raw []byte) error {
	type plain Comment
	var p plain
	if e := json.Unmarshal(raw, &p); e != nil {
		return e
	}
	*v = Comment(p)
	v.Raw = append([]byte{}, raw...)
	return nil
}

type IssueInput struct {
	Title, Body, State string
	Labels             []int64
	Assignees          []string
	ContentVersion     int64
}
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string { return fmt.Sprintf("forge API returned HTTP %d", e.StatusCode) }

type wireBudget struct {
	mu        sync.Mutex
	remaining int64
}
type Client struct {
	base           *url.URL
	token, profile string
	http           *http.Client
	// PageSize is configurable before use; every listing validates 1..50.
	PageSize int
	budget   *wireBudget
}

func NewClient(root, token string) (*Client, error) {
	return NewProfileClient(root, token, "forgejo-16")
}

// NewProfileClient shares only endpoints checked in Forgejo v11.0.11, Codeberg
// Forgejo 16 Swagger and Gitea v1.24.7/v1.27.0. Primary schemas:
// https://codeberg.org/forgejo/forgejo/raw/tag/v11.0.11/templates/swagger/v1_json.tmpl
// https://codeberg.org/swagger.v1.json
// https://github.com/go-gitea/gitea/blob/v1.27.0/templates/swagger/v1_json.tmpl
// It never discovers instance credentials.
func NewProfileClient(root, token, profile string) (*Client, error) {
	if profile != "forgejo-11" && profile != "forgejo-16" && profile != "gitea-1.24" && profile != "gitea-1.27" {
		return nil, errors.New("untested forge API profile")
	}
	if root == "" && strings.HasPrefix(profile, "forgejo-") {
		root = "https://codeberg.org/api/v1"
	}
	u, e := url.Parse(root)
	if e != nil || u == nil || u.Host == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || strings.Contains(root, "#") {
		return nil, errors.New("explicit forge /api/v1 URL required")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return nil, errors.New("HTTPS or explicit loopback HTTP required")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(u.Path, "/api/v1") || path.Clean(u.Path) != u.Path || strings.ContainsAny(u.Path, "\\\x00\r\n") {
		return nil, errors.New("forge root must end in /api/v1")
	}
	if p := u.Port(); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid port")
		}
	}
	if strings.HasSuffix(u.Host, ":") {
		return nil, errors.New("invalid host")
	}
	if ip == nil {
		for _, s := range strings.Split(u.Hostname(), ".") {
			if s == "" || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
				return nil, errors.New("invalid host")
			}
			for _, r := range s {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
					return nil, errors.New("invalid host")
				}
			}
		}
	}
	u.Host = strings.ToLower(u.Host)
	if u.Scheme == "https" {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	} else {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	for _, r := range token {
		if r < 33 || r > 126 {
			return nil, errors.New("invalid supplied token")
		}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxResponseHeaderBytes = 64 << 10
	return &Client{base: u, token: token, profile: profile, PageSize: 50, http: &http.Client{Timeout: 30 * time.Second, Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) BaseURL() string { return c.base.String() }
func (c *Client) Profile() string { return c.profile }
func (c *Client) BoundedReadClient(n int64) *Client {
	v := *c
	if n > MaxTotalBytes {
		n = MaxTotalBytes
	}
	v.budget = &wireBudget{remaining: n}
	return &v
}
func ValidateRepository(repo string) error {
	p := strings.Split(repo, "/")
	if len(p) != 2 {
		return errors.New("repository requires OWNER/NAME")
	}
	for _, s := range p {
		if s == "" || s == "." || s == ".." || len(s) > 256 {
			return errors.New("invalid repository")
		}
		for _, r := range s {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
				return errors.New("invalid repository")
			}
		}
	}
	return nil
}
func (c *Client) repoPath(repo, suffix string) (string, error) {
	if e := ValidateRepository(repo); e != nil {
		return "", e
	}
	return "/repos/" + repo + suffix, nil
}
func (c *Client) request(ctx context.Context, method, p string, q url.Values, input any, cap int64) ([]byte, http.Header, error) {
	u := *c.base
	u.Path += p
	// A ref is one encoded path segment, including refs containing '/'.
	if strings.Contains(p, "%") {
		decoded, e := url.PathUnescape(u.Path)
		if e != nil {
			return nil, nil, errors.New("invalid escaped endpoint")
		}
		u.RawPath, u.Path = u.Path, decoded
	}
	u.RawQuery = q.Encode()
	return c.requestURL(ctx, method, &u, input, cap)
}
func (c *Client) requestURL(ctx context.Context, method string, u *url.URL, input any, cap int64) ([]byte, http.Header, error) {
	if u.Scheme != c.base.Scheme || u.Host != c.base.Host || u.User != nil || u.Fragment != "" {
		return nil, nil, errors.New("same-origin URL required")
	}
	if method != "GET" && c.token == "" {
		return nil, nil, errors.New("writes require explicitly supplied token")
	}
	var body io.Reader
	if input != nil {
		b, e := json.Marshal(input)
		if e != nil {
			return nil, nil, e
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, u.String(), body)
	if e != nil {
		return nil, nil, errors.New("invalid request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Hubnot explicit "+c.profile+" transport")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errors.New("forge request failed; write outcome may be unknown")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && !(method == "POST" && resp.StatusCode == 201) {
		return nil, nil, &HTTPError{resp.StatusCode}
	}
	if cap < 0 {
		return nil, nil, errors.New("invalid read budget")
	}
	if c.budget != nil {
		c.budget.mu.Lock()
		defer c.budget.mu.Unlock()
		if cap > c.budget.remaining {
			cap = c.budget.remaining
		}
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, cap+1))
	if e != nil {
		return nil, nil, errors.New("response read failed; write outcome may be unknown")
	}
	if int64(len(b)) > cap {
		return nil, nil, errors.New("forge read budget exceeded")
	}
	if c.budget != nil {
		c.budget.remaining -= int64(len(b))
	}
	return b, resp.Header, nil
}

// StrictJSON rejects duplicate keys and Unicode repair before a DTO can lose
// hostile data. Unknown source fields remain in Raw for preservation.
func StrictJSON(b []byte) bool {
	if !utf8.Valid(b) || !json.Valid(b) {
		return false
	}
	quoted := false
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || b[i] != '\\' {
			continue
		}
		i++
		if b[i] != 'u' {
			continue
		}
		n, _ := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
				return false
			}
			lo, _ := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
			if lo < 0xdc00 || lo > 0xdfff {
				return false
			}
			i += 6
		}
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var walk func(int) bool
	walk = func(depth int) bool {
		if depth > 32 {
			return false
		}
		t, e := d.Token()
		if e != nil {
			return false
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				s, ok := k.(string)
				if e != nil || !ok || seen[s] {
					return false
				}
				seen[s] = true
				if !walk(depth + 1) {
					return false
				}
			}
			end, e := d.Token()
			return e == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !walk(depth + 1) {
					return false
				}
			}
			end, e := d.Token()
			return e == nil && end == json.Delim(']')
		}
		return false
	}
	if !walk(0) {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
func Decode[T any](b []byte) (T, error) {
	var v T
	if !StrictJSON(b) || bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return v, errors.New("invalid ambiguous response JSON")
	}
	e := json.Unmarshal(b, &v)
	return v, e
}
func (c *Client) CheckVersion(ctx context.Context) error {
	b, _, e := c.request(ctx, "GET", "/version", nil, nil, MaxResponseBytes)
	if e != nil {
		return e
	}
	v, e := Decode[struct {
		Version string `json:"version"`
	}](b)
	if e != nil {
		return e
	}
	prefix := strings.TrimPrefix(c.profile, "forgejo-") + "."
	if strings.HasPrefix(c.profile, "gitea-") {
		prefix = strings.TrimPrefix(c.profile, "gitea-") + "."
	}
	if !strings.HasPrefix(v.Version, prefix) {
		return errors.New("server version outside independently tested " + c.profile + " profile")
	}
	core := strings.FieldsFunc(v.Version, func(r rune) bool { return r == '+' || r == '-' })
	if len(core) == 0 {
		return errors.New("invalid server version")
	}
	parts := strings.Split(core[0], ".")
	if len(parts) != 3 {
		return errors.New("invalid server version")
	}
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil || strconv.FormatUint(n, 10) != p {
			return errors.New("invalid server version")
		}
	}
	return nil
}
func (c *Client) GetRepository(ctx context.Context, repo string) (Repository, error) {
	p, e := c.repoPath(repo, "")
	if e != nil {
		return Repository{}, e
	}
	b, _, e := c.request(ctx, "GET", p, nil, nil, MaxResponseBytes)
	if e != nil {
		return Repository{}, e
	}
	v, e := Decode[Repository](b)
	_, dateErr := time.Parse(time.RFC3339, v.CreatedAt)
	if e == nil && (v.ID <= 0 || ValidateRepository(v.FullName) != nil || dateErr != nil) {
		e = errors.New("invalid immutable repository identity")
	}
	return v, e
}
func nextPage(h http.Header, current *url.URL, page int) (bool, error) {
	next := false
	for _, header := range h.Values("Link") {
		for _, part := range strings.Split(header, ",") {
			part = strings.TrimSpace(part)
			end := strings.IndexByte(part, '>')
			if !strings.HasPrefix(part, "<") || end < 1 {
				return false, errors.New("invalid pagination header")
			}
			isNext := false
			for _, p := range strings.Split(part[end+1:], ";") {
				k, v, ok := strings.Cut(strings.TrimSpace(p), "=")
				if ok && k == "rel" {
					for _, r := range strings.Fields(strings.Trim(v, "\"")) {
						isNext = isNext || r == "next"
					}
				}
			}
			if !isNext {
				continue
			}
			u, e := url.Parse(part[1:end])
			if e != nil || next || u.User != nil || u.Fragment != "" || strings.Contains(part[1:end], "#") {
				return false, errors.New("invalid next page")
			}
			u = current.ResolveReference(u)
			q := current.Query()
			q.Set("page", strconv.Itoa(page+1))
			if u.Scheme != current.Scheme || u.Host != current.Host || u.EscapedPath() != current.EscapedPath() || u.Query().Encode() != q.Encode() {
				return false, errors.New("pagination changed origin endpoint or query")
			}
			next = true
		}
	}
	return next, nil
}
func list[T any](ctx context.Context, c *Client, p string, q url.Values, id func(T) int64) ([]T, error) {
	if c.PageSize < 1 || c.PageSize > 50 {
		return nil, errors.New("page size must be 1..50")
	}
	if q == nil {
		q = url.Values{}
	}
	q.Set("limit", strconv.Itoa(c.PageSize))
	seen := map[int64]bool{}
	out := []T{}
	total := int64(0)
	expected := -1
	for page := 1; page <= MaxPages; page++ {
		q.Set("page", strconv.Itoa(page))
		b, h, e := c.request(ctx, "GET", p, q, nil, MaxResponseBytes)
		if e != nil {
			return nil, e
		}
		total += int64(len(b))
		if total > MaxTotalBytes {
			return nil, errors.New("listing aggregate budget exceeded")
		}
		v, e := Decode[[]T](b)
		if e != nil || v == nil || len(v) > c.PageSize {
			return nil, errors.New("invalid listing page")
		}
		for _, x := range v {
			n := id(x)
			if n <= 0 || seen[n] {
				return nil, errors.New("invalid duplicate listing identity")
			}
			seen[n] = true
		}
		out = append(out, v...)
		if raw := h.Get("X-Total-Count"); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 0 || (expected >= 0 && n != expected) {
				return nil, errors.New("inconsistent listing total")
			}
			expected = n
		}
		u := *c.base
		u.Path += p
		u.RawQuery = q.Encode()
		next, e := nextPage(h, &u, page)
		if e != nil {
			return nil, e
		}
		if expected >= 0 && len(out) > expected {
			return nil, errors.New("listing exceeds total")
		}
		// Do not infer completion from a server's configurable maximum page size.
		// Without totals/links, an empty sentinel page establishes completion.
		if !next && (expected >= 0 && len(out) == expected || len(v) == 0) {
			if expected >= 0 && len(out) != expected {
				return nil, errors.New("truncated listing")
			}
			return out, nil
		}
	}
	return nil, errors.New("listing page budget exceeded")
}
func (c *Client) ListIssueRows(ctx context.Context, repo string) ([]Issue, error) {
	p, e := c.repoPath(repo, "/issues")
	if e != nil {
		return nil, e
	}
	rows, e := list(ctx, c, p, url.Values{"state": {"all"}}, func(v Issue) int64 {
		if v.Number <= 0 || (v.State != "open" && v.State != "closed") {
			return 0
		}
		return v.ID
	})
	if e != nil {
		return nil, e
	}
	selectors := map[int]bool{}
	for _, v := range rows {
		if selectors[v.Number] {
			return nil, errors.New("duplicate repository-local issue index")
		}
		selectors[v.Number] = true
	}
	return rows, nil
}
func (c *Client) ListIssues(ctx context.Context, repo string) ([]Issue, error) {
	vs, e := c.ListIssueRows(ctx, repo)
	if e != nil {
		return nil, e
	}
	out := []Issue{}
	for _, v := range vs {
		if !v.IsPull() {
			out = append(out, v)
		}
	}
	return out, nil
}
func index(n int) error {
	if n <= 0 {
		return errors.New("canonical positive index required")
	}
	return nil
}
func (c *Client) GetIssue(ctx context.Context, repo string, n int) (Issue, error) {
	if e := index(n); e != nil {
		return Issue{}, e
	}
	p, e := c.repoPath(repo, "/issues/"+strconv.Itoa(n))
	if e != nil {
		return Issue{}, e
	}
	b, _, e := c.request(ctx, "GET", p, nil, nil, MaxResponseBytes)
	if e != nil {
		return Issue{}, e
	}
	v, e := Decode[Issue](b)
	if e == nil && (v.ID <= 0 || v.Number != n || (v.State != "open" && v.State != "closed")) {
		e = errors.New("invalid issue identity")
	}
	return v, e
}
func (c *Client) ListComments(ctx context.Context, repo string, n int) ([]Comment, error) {
	if e := index(n); e != nil {
		return nil, e
	}
	p, e := c.repoPath(repo, "/issues/"+strconv.Itoa(n)+"/comments")
	if e != nil {
		return nil, e
	}
	return list(ctx, c, p, nil, func(v Comment) int64 { return v.ID })
}
func (c *Client) ListLabels(ctx context.Context, repo string) ([]Label, error) {
	p, e := c.repoPath(repo, "/labels")
	if e != nil {
		return nil, e
	}
	return list(ctx, c, p, nil, func(v Label) int64 {
		if v.Name == "" {
			return 0
		}
		return v.ID
	})
}
func (c *Client) writeIssue(ctx context.Context, repo string, n int, in IssueInput) (Issue, error) {
	if !utf8.ValidString(in.Title) || !utf8.ValidString(in.Body) || strings.TrimSpace(in.Title) == "" || len(in.Title) > 255 || len(in.Body) > BodyCapacity || (in.State != "open" && in.State != "closed") {
		return Issue{}, errors.New("unsupported issue projection")
	}
	for _, id := range in.Labels {
		if id <= 0 {
			return Issue{}, errors.New("labels require positive numeric IDs")
		}
	}
	if in.Labels == nil {
		in.Labels = []int64{}
	}
	if in.Assignees == nil {
		in.Assignees = []string{}
	}
	p, e := c.repoPath(repo, "/issues")
	if e != nil {
		return Issue{}, e
	}
	method := "POST"
	var input any = struct {
		Title     string   `json:"title"`
		Body      string   `json:"body"`
		Closed    bool     `json:"closed"`
		Labels    []int64  `json:"labels"`
		Assignees []string `json:"assignees"`
	}{in.Title, in.Body, in.State == "closed", in.Labels, in.Assignees}
	if n > 0 {
		method = "PATCH"
		p += "/" + strconv.Itoa(n)
		input = struct {
			Title          string   `json:"title"`
			Body           string   `json:"body"`
			State          string   `json:"state"`
			Assignees      []string `json:"assignees"`
			ContentVersion int64    `json:"content_version,omitempty"`
		}{in.Title, in.Body, in.State, in.Assignees, in.ContentVersion}
	}
	b, _, e := c.request(ctx, method, p, nil, input, MaxResponseBytes)
	if e != nil {
		return Issue{}, e
	}
	v, e := Decode[Issue](b)
	if e == nil && (v.ID <= 0 || v.Number <= 0 || n > 0 && v.Number != n || v.IsPull()) {
		e = errors.New("invalid issue write identity")
	}
	return v, e
}
func (c *Client) CreateIssue(ctx context.Context, repo string, in IssueInput) (Issue, error) {
	return c.writeIssue(ctx, repo, 0, in)
}

// UpdateIssue does not send labels: this endpoint cannot mutate them. Callers
// must check the complete expected label projection before this single write.
func (c *Client) UpdateIssue(ctx context.Context, repo string, n int, in IssueInput) (Issue, error) {
	if e := index(n); e != nil {
		return Issue{}, e
	}
	return c.writeIssue(ctx, repo, n, in)
}
func (c *Client) writeComment(ctx context.Context, repo string, n int, id int64, body string) (Comment, error) {
	if !utf8.ValidString(body) || len(body) > BodyCapacity || body == "" {
		return Comment{}, errors.New("comment exceeds profile capacity")
	}
	suffix := "/issues/" + strconv.Itoa(n) + "/comments"
	method := "POST"
	if id > 0 {
		suffix = "/issues/comments/" + strconv.FormatInt(id, 10)
		method = "PATCH"
	} else if e := index(n); e != nil {
		return Comment{}, e
	}
	p, e := c.repoPath(repo, suffix)
	if e != nil {
		return Comment{}, e
	}
	b, _, e := c.request(ctx, method, p, nil, struct {
		Body string `json:"body"`
	}{body}, MaxResponseBytes)
	if e != nil {
		return Comment{}, e
	}
	v, e := Decode[Comment](b)
	if e == nil && (v.ID <= 0 || id > 0 && v.ID != id) {
		e = errors.New("invalid comment identity")
	}
	return v, e
}
func (c *Client) CreateComment(ctx context.Context, repo string, n int, body string) (Comment, error) {
	return c.writeComment(ctx, repo, n, 0, body)
}
func (c *Client) UpdateComment(ctx context.Context, repo string, id int64, body string) (Comment, error) {
	if id <= 0 {
		return Comment{}, errors.New("positive comment ID required")
	}
	return c.writeComment(ctx, repo, 0, id, body)
}

// RawObject retains all version-specific fields (review states, diff anchors,
// resolver, dismissal, original author) without claiming native review authority.
type RawObject struct {
	ID  int64
	Raw json.RawMessage
}

func (v *RawObject) UnmarshalJSON(b []byte) error {
	var x struct {
		ID int64 `json:"id"`
	}
	if e := json.Unmarshal(b, &x); e != nil {
		return e
	}
	v.ID = x.ID
	v.Raw = append([]byte{}, b...)
	return nil
}
func (c *Client) ReadPull(ctx context.Context, repo string, n int) (RawObject, error) {
	if e := index(n); e != nil {
		return RawObject{}, e
	}
	p, e := c.repoPath(repo, "/pulls/"+strconv.Itoa(n))
	if e != nil {
		return RawObject{}, e
	}
	b, _, e := c.request(ctx, "GET", p, nil, nil, MaxResponseBytes)
	if e != nil {
		return RawObject{}, e
	}
	v, e := Decode[RawObject](b)
	if e == nil && v.ID <= 0 {
		e = errors.New("invalid pull identity")
	}
	return v, e
}
func (c *Client) ListReviews(ctx context.Context, repo string, n int) ([]RawObject, error) {
	if e := index(n); e != nil {
		return nil, e
	}
	p, e := c.repoPath(repo, "/pulls/"+strconv.Itoa(n)+"/reviews")
	if e != nil {
		return nil, e
	}
	return list(ctx, c, p, nil, func(v RawObject) int64 { return v.ID })
}
func (c *Client) ListReviewComments(ctx context.Context, repo string, n int, id int64) ([]RawObject, error) {
	if e := index(n); e != nil {
		return nil, e
	}
	if id <= 0 {
		return nil, errors.New("positive review ID required")
	}
	p, e := c.repoPath(repo, "/pulls/"+strconv.Itoa(n)+"/reviews/"+strconv.FormatInt(id, 10)+"/comments")
	if e != nil {
		return nil, e
	}
	return list(ctx, c, p, nil, func(v RawObject) int64 { return v.ID })
}
func (c *Client) ReadAttachment(ctx context.Context, raw string, limit int64) ([]byte, string, error) {
	u, e := url.Parse(raw)
	if e != nil || u == nil || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || strings.Contains(raw, "#") || path.Clean(u.Path) != u.Path {
		return nil, "", errors.New("invalid same-origin attachment")
	}
	if limit <= 0 {
		return nil, "", errors.New("positive attachment budget required")
	}
	b, h, e := c.requestURL(ctx, "GET", u, nil, limit)
	return b, h.Get("Content-Type"), e
}

type WorkflowFile struct {
	Path, Commit string
	Content      []byte
	Raw          json.RawMessage
}

func ValidOID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil && s == strings.ToLower(s)
}
func (c *Client) ReadWorkflowDefinitions(ctx context.Context, repo, ref string) ([]WorkflowFile, error) {
	if ref == "" || ref == "." || ref == ".." || len(ref) > 256 || strings.ContainsAny(ref, "\\\x00\r\n") {
		return nil, errors.New("explicit workflow revision required")
	}
	p, e := c.repoPath(repo, "/git/commits/"+url.PathEscape(ref))
	if e != nil {
		return nil, e
	}
	b, _, e := c.request(ctx, "GET", p, url.Values{"stat": {"false"}, "verification": {"false"}, "files": {"false"}}, nil, MaxResponseBytes)
	if e != nil {
		return nil, e
	}
	commit, e := Decode[struct {
		SHA string `json:"sha"`
	}](b)
	if e != nil || !ValidOID(commit.SHA) {
		return nil, errors.New("invalid pinned workflow commit")
	}
	dirs := []string{".forgejo/workflows", ".gitea/workflows"}
	if strings.HasPrefix(c.profile, "gitea-") {
		dirs = []string{".gitea/workflows"}
	}
	type entry struct {
		Type, Path, Encoding, Content string
		Size                          int64
	}
	out := []WorkflowFile{}
	for _, dir := range dirs {
		p, _ := c.repoPath(repo, "/contents/"+dir)
		b, _, e := c.request(ctx, "GET", p, url.Values{"ref": {commit.SHA}}, nil, MaxResponseBytes)
		if e != nil {
			var h *HTTPError
			if errors.As(e, &h) && h.StatusCode == 404 {
				continue
			}
			return nil, e
		}
		rows, e := Decode[[]entry](b)
		if e != nil || len(rows) > 1000 {
			return nil, errors.New("invalid workflow directory")
		}
		seen := map[string]bool{}
		for _, r := range rows {
			if r.Type != "file" || path.Dir(r.Path) != dir || seen[r.Path] || strings.ContainsAny(r.Path, "%?#\\\x00") {
				return nil, errors.New("unsafe workflow entry")
			}
			seen[r.Path] = true
			if !strings.HasSuffix(r.Path, ".yml") && !strings.HasSuffix(r.Path, ".yaml") {
				continue
			}
			p, _ := c.repoPath(repo, "/contents/"+r.Path)
			raw, _, e := c.request(ctx, "GET", p, url.Values{"ref": {commit.SHA}}, nil, MaxResponseBytes)
			if e != nil {
				return nil, e
			}
			v, e := Decode[entry](raw)
			if e != nil || v.Type != "file" || v.Path != r.Path || v.Encoding != "base64" || v.Size < 0 || v.Size > MaxResponseBytes {
				return nil, errors.New("invalid workflow file")
			}
			data, e := base64.StdEncoding.DecodeString(strings.ReplaceAll(v.Content, "\n", ""))
			if e != nil || int64(len(data)) != v.Size {
				return nil, errors.New("workflow content size mismatch")
			}
			out = append(out, WorkflowFile{r.Path, commit.SHA, data, append([]byte{}, raw...)})
		}
	}
	return out, nil
}
