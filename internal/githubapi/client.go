//go:build !nogithub && !noforge

// Package githubapi is the optional, non-authoritative GitHub issue transport.
// It performs no requests until explicitly called and has no gh dependency.
package githubapi

import (
	"bytes"
	"context"
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
	"time"
	"unicode/utf8"
)

const (
	DefaultBaseURL   = "https://api.github.com"
	RequestTimeout   = 30 * time.Second
	MaxResponseBytes = 4 << 20
	MaxTotalBytes    = 64 << 20
	MaxPages         = 100
)

type User struct {
	Login string `json:"login"`
}

type Label struct {
	Name string `json:"name"`
}

// IssueType is descriptive source metadata; migrations do not administer types.
type IssueType struct {
	Name string `json:"name"`
}

type Issue struct {
	Type        *IssueType      `json:"type,omitempty"`
	ID          int64           `json:"id"`
	NodeID      string          `json:"node_id"`
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	State       string          `json:"state"`
	HTMLURL     string          `json:"html_url"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	User        User            `json:"user"`
	Labels      []Label         `json:"labels"`
	Assignees   []User          `json:"assignees"`
	PullRequest json.RawMessage `json:"pull_request,omitempty"`
}

type Comment struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	User      User   `json:"user"`
}

// IssueInput is a complete projection of supported mutable issue fields.
// Empty labels/assignees explicitly clear those fields on updates.
type IssueInput struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	State     string   `json:"state,omitempty"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
}

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

// BaseURL returns the validated base with a lowercase host and no trailing
// slash. Coordinators can use this value in stable external-source keys.
func (c *Client) BaseURL() string { return c.base.String() }

// HTTPError deliberately excludes remote response bodies and headers, which
// can contain arbitrary private content or reflected credentials.
type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("GitHub API returned HTTP %d", e.StatusCode)
}

// NewClient permits plain HTTP only for explicit loopback IP addresses. It
// rejects URL credentials and ambiguous paths before any request is made.
func NewClient(baseURL, token string) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u == nil || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.Host == "" || strings.HasSuffix(u.Host, ":") || u.RawPath != "" || strings.Contains(baseURL, "#") {
		return nil, errors.New("invalid GitHub API base URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, errors.New("GitHub API URL requires HTTPS or explicit loopback HTTP")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host == "" || (ip == nil && !validHostname(host)) || (u.Scheme == "http" && (ip == nil || !ip.IsLoopback())) {
		return nil, errors.New("GitHub API URL requires a valid host; HTTP requires a loopback IP")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid GitHub API port")
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path != "" && (path.Clean(u.Path) != u.Path || strings.ContainsAny(u.Path, "\\\x00\r\n")) {
		return nil, errors.New("invalid GitHub API base path")
	}
	u.Host = strings.ToLower(u.Host)
	if u.Scheme == "https" && strings.HasSuffix(u.Host, ":443") {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}
	if u.Scheme == "http" && strings.HasSuffix(u.Host, ":80") {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	for _, c := range token {
		if c < 33 || c > 126 {
			return nil, errors.New("invalid GitHub API token")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxResponseHeaderBytes = 64 << 10
	return &Client{base: u, token: token, http: &http.Client{
		Timeout:       RequestTimeout,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func validHostname(host string) bool {
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func validRepo(repo string) bool {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 256 || part == "." || part == ".." {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
				return false
			}
		}
	}
	return true
}

func (c *Client) endpoint(repo string, number int, comments bool) (*url.URL, error) {
	if !validRepo(repo) || number < 0 || (comments && number == 0) {
		return nil, errors.New("GitHub repository must be OWNER/NAME and issue numbers must be positive")
	}
	u := *c.base
	u.Path += "/repos/" + repo + "/issues"
	if number > 0 {
		u.Path += "/" + strconv.Itoa(number)
	}
	if comments {
		u.Path += "/comments"
	}
	return &u, nil
}

func (c *Client) request(ctx context.Context, method string, u *url.URL, input any, want int) ([]byte, http.Header, error) {
	if method != http.MethodGet && c.token == "" {
		return nil, nil, errors.New("GitHub writes require HN_GITHUB_TOKEN")
	}
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return nil, nil, errors.New("invalid GitHub request JSON")
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, nil, errors.New("could not construct GitHub request")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Hubnot optional GitHub issue transport")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("GitHub request canceled: %w", ctx.Err())
		}
		return nil, nil, errors.New("GitHub request failed; a write outcome may be unknown")
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		return nil, nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, nil, errors.New("could not read GitHub response; a write outcome may be unknown")
	}
	if len(raw) > MaxResponseBytes {
		return nil, nil, errors.New("GitHub response exceeds 4 MiB limit")
	}
	return raw, resp.Header, nil
}

func decodeResponse[T any](raw []byte) (T, error) {
	var zero T
	if !strictGraphQLJSON(raw) {
		return zero, errors.New("invalid or ambiguous GitHub response JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var result *T
	if err := decoder.Decode(&result); err != nil || result == nil {
		return zero, errors.New("invalid GitHub response shape")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return zero, errors.New("GitHub response must contain one JSON value")
	}
	return *result, nil
}

// Reject Unicode repairs that encoding/json would otherwise perform silently.
func validJSONUnicode(raw []byte) bool {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return false
	}
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		unit, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit < 0xd800 || unit > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return false
		}
		low, _ := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

func validIssue(v Issue) bool {
	return v.ID > 0 && v.NodeID != "" && v.Number > 0 && (v.State == "open" || v.State == "closed") && (v.Type == nil || v.Type.Name != "" && len(v.Type.Name) <= 256 && utf8.ValidString(v.Type.Name))
}

func validComment(v Comment) bool { return v.ID > 0 && v.NodeID != "" }

func (c *Client) GetIssue(ctx context.Context, repo string, number int) (Issue, error) {
	if number <= 0 {
		return Issue{}, errors.New("GitHub issue number must be positive")
	}
	u, err := c.endpoint(repo, number, false)
	if err != nil {
		return Issue{}, err
	}
	raw, _, err := c.request(ctx, http.MethodGet, u, nil, http.StatusOK)
	if err != nil {
		return Issue{}, err
	}
	v, err := decodeResponse[Issue](raw)
	if err != nil {
		return Issue{}, err
	}
	if !validIssue(v) || v.Number != number {
		return Issue{}, errors.New("GitHub issue response identity/state is invalid")
	}
	return v, nil
}

func (c *Client) ListIssues(ctx context.Context, repo string) ([]Issue, error) {
	u, err := c.endpoint(repo, 0, false)
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("state", "all")
	u.RawQuery = query.Encode()
	rows, err := listPages[Issue](ctx, c, u, func(v Issue) bool { return validIssue(v) }, func(v Issue) int64 { return v.ID })
	if err != nil {
		return nil, err
	}
	result := make([]Issue, 0, len(rows))
	for _, row := range rows {
		if len(row.PullRequest) == 0 || string(row.PullRequest) == "null" {
			result = append(result, row)
		}
	}
	return result, nil
}

func (c *Client) ListComments(ctx context.Context, repo string, number int) ([]Comment, error) {
	u, err := c.endpoint(repo, number, true)
	if err != nil {
		return nil, err
	}
	return listPages[Comment](ctx, c, u, validComment, func(v Comment) int64 { return v.ID })
}

func listPages[T any](ctx context.Context, c *Client, first *url.URL, valid func(T) bool, id func(T) int64) ([]T, error) {
	u := *first
	query := u.Query()
	query.Set("per_page", "100")
	query.Set("page", "1")
	u.RawQuery = query.Encode()
	rows := make([]T, 0)
	seen := make(map[int64]bool)
	total := 0
	for pageNumber := 1; pageNumber <= MaxPages; pageNumber++ {
		raw, headers, err := c.request(ctx, http.MethodGet, &u, nil, http.StatusOK)
		if err != nil {
			return nil, err
		}
		total += len(raw)
		if total > MaxTotalBytes {
			return nil, errors.New("GitHub listing exceeds 64 MiB aggregate limit")
		}
		pageRows, err := decodeResponse[[]T](raw)
		if err != nil || pageRows == nil {
			return nil, errors.New("invalid GitHub listing JSON")
		}
		if len(pageRows) > 100 {
			return nil, errors.New("GitHub listing page exceeds 100 items")
		}
		for _, row := range pageRows {
			key := id(row)
			if !valid(row) || seen[key] {
				return nil, errors.New("invalid or duplicate GitHub listing identity")
			}
			seen[key] = true
		}
		rows = append(rows, pageRows...)
		next, err := nextPage(headers.Values("Link"), &u, pageNumber)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return rows, nil
		}
		if pageNumber == MaxPages {
			return nil, errors.New("GitHub listing exceeds 100 page limit")
		}
		u = *next
	}
	return nil, errors.New("GitHub listing page limit reached")
}

func nextPage(headers []string, current *url.URL, pageNumber int) (*url.URL, error) {
	var next *url.URL
	for _, header := range headers {
		for _, part := range strings.Split(header, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			close := strings.IndexByte(part, '>')
			if !strings.HasPrefix(part, "<") || close < 1 {
				return nil, errors.New("invalid GitHub pagination header")
			}
			isNext := false
			for _, parameter := range strings.Split(part[close+1:], ";") {
				name, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
				if ok && strings.EqualFold(strings.TrimSpace(name), "rel") {
					for _, rel := range strings.Fields(strings.Trim(strings.TrimSpace(value), `"`)) {
						isNext = isNext || strings.EqualFold(rel, "next")
					}
				}
			}
			if !isNext {
				continue
			}
			u, err := url.Parse(part[1:close])
			if err != nil || next != nil || u.User != nil || u.Fragment != "" || u.RawFragment != "" || u.Opaque != "" || strings.Contains(part[1:close], "#") {
				return nil, errors.New("invalid GitHub next page")
			}
			u = current.ResolveReference(u)
			if u.Scheme != current.Scheme || !strings.EqualFold(u.Host, current.Host) || u.EscapedPath() != current.EscapedPath() {
				return nil, errors.New("GitHub pagination cannot leave the requested endpoint")
			}
			query, err := url.ParseQuery(u.RawQuery)
			expected := current.Query()
			expected.Set("page", strconv.Itoa(pageNumber+1))
			if err != nil || query.Encode() != expected.Encode() {
				return nil, errors.New("GitHub pagination changed query or page sequence")
			}
			next = u
		}
	}
	return next, nil
}

func (c *Client) writeIssue(ctx context.Context, method, repo string, number int, input IssueInput) (Issue, error) {
	u, err := c.endpoint(repo, number, false)
	if err != nil {
		return Issue{}, err
	}
	if strings.TrimSpace(input.Title) == "" || (input.State != "" && input.State != "open" && input.State != "closed") {
		return Issue{}, errors.New("invalid GitHub issue input")
	}
	if input.Labels == nil {
		input.Labels = []string{}
	}
	if input.Assignees == nil {
		input.Assignees = []string{}
	}
	want := http.StatusOK
	if method == http.MethodPost {
		input.State = "" // GitHub issue creation does not accept state.
		want = http.StatusCreated
	}
	raw, _, err := c.request(ctx, method, u, input, want)
	if err != nil {
		return Issue{}, err
	}
	v, err := decodeResponse[Issue](raw)
	if err != nil {
		return Issue{}, err
	}
	if !validIssue(v) || (number > 0 && v.Number != number) {
		return Issue{}, errors.New("GitHub issue response identity/state is invalid; write outcome may be unknown")
	}
	return v, nil
}

func (c *Client) CreateIssue(ctx context.Context, repo string, input IssueInput) (Issue, error) {
	return c.writeIssue(ctx, http.MethodPost, repo, 0, input)
}

func (c *Client) UpdateIssue(ctx context.Context, repo string, number int, input IssueInput) (Issue, error) {
	if number <= 0 {
		return Issue{}, errors.New("GitHub issue number must be positive")
	}
	return c.writeIssue(ctx, http.MethodPatch, repo, number, input)
}

func (c *Client) CreateComment(ctx context.Context, repo string, number int, body string) (Comment, error) {
	u, err := c.endpoint(repo, number, true)
	if err != nil {
		return Comment{}, err
	}
	raw, _, err := c.request(ctx, http.MethodPost, u, struct {
		Body string `json:"body"`
	}{body}, http.StatusCreated)
	if err != nil {
		return Comment{}, err
	}
	v, err := decodeResponse[Comment](raw)
	if err != nil {
		return Comment{}, err
	}
	if !validComment(v) {
		return Comment{}, errors.New("invalid GitHub comment response identity; write outcome may be unknown")
	}
	return v, nil
}
