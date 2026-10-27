//go:build !nogithub && !noforge

package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// ValidateRepository applies the existing transfer transport selector contract
// without a request, so provider constructors cannot invent a second syntax.
func ValidateRepository(repo string) error {
	if !validRepo(repo) {
		return errors.New("GitHub repository requires OWNER/NAME")
	}
	return nil
}

// Repository identity is independent of its mutable OWNER/NAME selector.
// CreatedAt participates in a stable repository incarnation revision.
type Repository struct {
	ID             int64  `json:"id"`
	NodeID         string `json:"node_id"`
	FullName       string `json:"full_name"`
	CreatedAt      string `json:"created_at"`
	DefaultBranch  string `json:"default_branch"`
	HasIssues      bool   `json:"has_issues"`
	HasDiscussions bool   `json:"has_discussions"`
}

func (c *Client) GetRepository(ctx context.Context, repo string) (Repository, error) {
	u, e := c.workflowURL(repo, "")
	if e != nil {
		return Repository{}, e
	}
	raw, _, e := c.request(ctx, http.MethodGet, u, nil, http.StatusOK)
	if e != nil {
		return Repository{}, e
	}
	v, e := workflowDecode[Repository](raw)
	if e != nil || v.ID <= 0 || v.NodeID == "" || !strings.EqualFold(v.FullName, repo) || !validSourceTime(v.CreatedAt) {
		return Repository{}, errors.New("invalid or replaced GitHub repository identity")
	}
	return v, nil
}

// MigrationObject retains the original API JSON, including review positions,
// commit IDs, and thread relationships without granting native authority.
type MigrationObject struct {
	Raw       json.RawMessage
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Number    int    `json:"number"`
	HTMLURL   string `json:"html_url"`
	Body      string `json:"body"`
	User      User   `json:"user"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ReviewID  int64  `json:"pull_request_review_id"`
	ReplyTo   int64  `json:"in_reply_to_id"`
}

func (v *MigrationObject) UnmarshalJSON(b []byte) error {
	type dto MigrationObject
	var d dto
	if !strictGraphQLJSON(b) {
		return errors.New("ambiguous GitHub migration object")
	}
	if e := json.Unmarshal(b, &d); e != nil {
		return e
	}
	*v = MigrationObject(d)
	v.Raw = append([]byte{}, b...)
	return nil
}
func (c *Client) migrationObjects(ctx context.Context, repo, suffix string) ([]MigrationObject, error) {
	u, e := c.workflowURL(repo, suffix)
	if e != nil {
		return nil, e
	}
	if suffix == "/pulls" {
		q := u.Query()
		q.Set("state", "all")
		u.RawQuery = q.Encode()
	}
	return listPages(ctx, c, u, func(v MigrationObject) bool { return v.ID > 0 && v.NodeID != "" }, func(v MigrationObject) int64 { return v.ID })
}
func (c *Client) ListPullRequests(ctx context.Context, repo string) ([]MigrationObject, error) {
	return c.migrationObjects(ctx, repo, "/pulls")
}
func (c *Client) ListPullReviews(ctx context.Context, repo string, number int) ([]MigrationObject, error) {
	if number <= 0 {
		return nil, errors.New("invalid pull number")
	}
	return c.migrationObjects(ctx, repo, "/pulls/"+strconv.Itoa(number)+"/reviews")
}
func (c *Client) ListPullReviewComments(ctx context.Context, repo string, number int) ([]MigrationObject, error) {
	if number <= 0 {
		return nil, errors.New("invalid pull number")
	}
	return c.migrationObjects(ctx, repo, "/pulls/"+strconv.Itoa(number)+"/comments")
}

// ReadAttachment accepts only an explicitly selected same-origin URL. GitHub's
// external upload hosts need a separately scoped unauthenticated transport; this
// adapter reports them unavailable rather than forwarding API credentials.
func (c *Client) ReadAttachment(ctx context.Context, rawURL string, max int64) ([]byte, string, error) {
	u, e := url.Parse(rawURL)
	if e != nil || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.Scheme != c.base.Scheme || !strings.EqualFold(u.Host, c.base.Host) || max <= 0 || max > 1<<54 {
		return nil, "", errors.New("attachment requires explicit same-origin URL and positive budget")
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return nil, "", errors.New("invalid attachment URL")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, "", errors.New("attachment request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", &HTTPError{resp.StatusCode}
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if e != nil || int64(len(data)) > max {
		return nil, "", errors.New("attachment byte budget exceeded or interrupted")
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// BoundedReadClient shares one aggregate wire-byte budget across all REST,
// GraphQL, workflow and attachment reads in a snapshot. The original client is
// unchanged and may be reused concurrently. Requests never automatically retry.
func (c *Client) BoundedReadClient(max int64) *Client {
	v := *c
	h := *c.http
	h.Transport = &migrationReadTransport{base: c.http.Transport, left: max}
	v.http = &h
	return &v
}

type migrationReadTransport struct {
	base http.RoundTripper
	mu   sync.Mutex
	left int64
}

func (t *migrationReadTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.mu.Lock()
	remaining := t.left
	t.mu.Unlock()
	if remaining <= 0 {
		return nil, errors.New("GitHub snapshot wire-byte budget exhausted")
	}
	resp, e := t.base.RoundTrip(r)
	if e == nil {
		resp.Body = &migrationReadBody{ReadCloser: resp.Body, budget: t}
	}
	return resp, e
}

type migrationReadBody struct {
	io.ReadCloser
	budget *migrationReadTransport
}

func (b *migrationReadBody) Read(p []byte) (int, error) {
	b.budget.mu.Lock()
	defer b.budget.mu.Unlock()
	if b.budget.left < 0 {
		return 0, fmt.Errorf("GitHub snapshot wire-byte budget exhausted")
	}
	if int64(len(p)) > b.budget.left+1 {
		p = p[:b.budget.left+1]
	}
	n, e := b.ReadCloser.Read(p)
	b.budget.left -= int64(n)
	if b.budget.left < 0 {
		return 0, errors.New("GitHub snapshot wire-byte budget exceeded")
	}
	return n, e
}
