//go:build !noforge

// Package gitlabapi implements explicitly selected GitLab REST v4 transport.
// Credentials are supplied by the caller; construction performs no I/O.
package gitlabapi

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
	"sync"
	"time"
	"unicode/utf8"
)

const DefaultBaseURL = "https://gitlab.com/api/v4"
const MaxResponseBytes = 4 << 20
const MaxTotalBytes = 64 << 20
const MaxPages = 100

type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string { return fmt.Sprintf("GitLab API returned HTTP %d", e.StatusCode) }

type readBudget struct {
	sync.Mutex
	left int64
}
type Client struct {
	base   *url.URL
	token  string
	http   *http.Client
	budget *readBudget
}

func (c *Client) BaseURL() string { return c.base.String() }
func NewClient(root, token string) (*Client, error) {
	if root == "" {
		root = DefaultBaseURL
	}
	u, e := url.Parse(root)
	if e != nil || u.Host == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(root, "#") || u.RawPath != "" || strings.HasSuffix(u.Host, ":") {
		return nil, errors.New("invalid GitLab API root")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return nil, errors.New("GitLab requires HTTPS or explicit loopback HTTP")
	}
	if ip == nil {
		for _, s := range strings.Split(u.Hostname(), ".") {
			if s == "" || len(s) > 63 || strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
				return nil, errors.New("invalid GitLab hostname")
			}
			for _, ch := range s {
				if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
					return nil, errors.New("invalid GitLab hostname")
				}
			}
		}
	}
	if p := u.Port(); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid GitLab port")
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(u.Path, "/api/v4") || path.Clean(u.Path) != u.Path || strings.ContainsAny(u.Path, "\\\x00\r\n") {
		return nil, errors.New("explicit GitLab root must end in /api/v4")
	}
	u.Host = strings.ToLower(u.Host)
	if u.Scheme == "https" {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	} else {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	for _, ch := range token {
		if ch < 33 || ch > 126 {
			return nil, errors.New("invalid GitLab token")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxResponseHeaderBytes = 64 << 10
	return &Client{base: u, token: token, http: &http.Client{Timeout: 30 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func ValidateProject(s string) error {
	if s == "" || len(s) > 1024 {
		return errors.New("GitLab project requires immutable numeric ID or namespace/path")
	}
	if n, e := strconv.ParseInt(s, 10, 64); e == nil {
		if n > 0 && strconv.FormatInt(n, 10) == s {
			return nil
		}
		return errors.New("noncanonical project ID")
	}
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return errors.New("GitLab project path needs namespace/name")
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return errors.New("invalid GitLab project component")
		}
		for _, ch := range p {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-' || ch == '.') {
				return errors.New("invalid GitLab project component")
			}
		}
	}
	return nil
}
func PositiveKey(s string) (int, error) {
	n, e := strconv.Atoi(s)
	if e != nil || n <= 0 || strconv.Itoa(n) != s {
		return 0, errors.New("GitLab IID must be a canonical positive number")
	}
	return n, nil
}
func (c *Client) projectURL(project, suffix string) (*url.URL, error) {
	if e := ValidateProject(project); e != nil {
		return nil, e
	}
	return url.Parse(c.BaseURL() + "/projects/" + url.PathEscape(project) + suffix)
}
func (c *Client) BoundedReadClient(n int64) *Client {
	v := *c
	v.budget = &readBudget{left: n}
	return &v
}
func (c *Client) read(body io.Reader, max int64) ([]byte, error) {
	if c.budget != nil {
		c.budget.Lock()
		defer c.budget.Unlock()
		if c.budget.left < 0 {
			return nil, errors.New("GitLab aggregate byte budget exceeded")
		}
		if c.budget.left < max {
			max = c.budget.left
		}
	}
	b, e := io.ReadAll(io.LimitReader(body, max+1))
	if c.budget != nil {
		c.budget.left -= int64(len(b))
	}
	if e != nil || int64(len(b)) > max {
		return nil, errors.New("GitLab response byte budget exceeded or interrupted")
	}
	return b, nil
}
func (c *Client) request(ctx context.Context, method string, u *url.URL, input any, want int, max int64) ([]byte, http.Header, error) {
	if u.Scheme != c.base.Scheme || u.Host != c.base.Host || u.User != nil {
		return nil, nil, errors.New("GitLab same-origin credential boundary")
	}
	if method != "GET" && c.token == "" {
		return nil, nil, errors.New("GitLab writes require a supplied token")
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
		return nil, nil, errors.New("invalid GitLab request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Hubnot GitLab transport")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errors.New("GitLab request failed; write outcome may be unknown")
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		return nil, nil, &HTTPError{resp.StatusCode}
	}
	b, e := c.read(resp.Body, max)
	return b, resp.Header, e
}

// Provider JSON may contain null and unknown fields. Reject duplicate fields,
// excessive nesting, invalid UTF-8 and repaired surrogate pairs before decoding.
func decode(raw []byte, out any) error {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return errors.New("invalid GitLab JSON")
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		i++
		for i < len(raw) && raw[i] != '"' {
			if raw[i] == '\\' {
				i++
				if raw[i] == 'u' {
					n, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
					i += 4
					if n >= 0xdc00 && n <= 0xdfff {
						return errors.New("unpaired GitLab surrogate")
					}
					if n >= 0xd800 && n <= 0xdbff {
						if i+6 >= len(raw) || string(raw[i+1:i+3]) != "\\u" {
							return errors.New("unpaired GitLab surrogate")
						}
						low, _ := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
						if low < 0xdc00 || low > 0xdfff {
							return errors.New("unpaired GitLab surrogate")
						}
						i += 6
					}
				}
			}
			i++
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if e := scanValue(d, 0); e != nil {
		return e
	}
	return json.Unmarshal(raw, out)
}
func scanValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("GitLab JSON depth exceeded")
	}
	tok, e := d.Token()
	if e != nil {
		return e
	}
	switch tok {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return errors.New("duplicate GitLab JSON field")
			}
			seen[s] = true
			if e = scanValue(d, depth+1); e != nil {
				return e
			}
		}
		_, e = d.Token()
	case json.Delim('['):
		for d.More() {
			if e = scanValue(d, depth+1); e != nil {
				return e
			}
		}
		_, e = d.Token()
	}
	return e
}
func list[T any](ctx context.Context, c *Client, u *url.URL, valid func(T) bool, id func(T) string) ([]T, error) {
	out := []T{}
	seen := map[string]bool{}
	total := 0
	for p := 1; p <= MaxPages; p++ {
		q := u.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", "100")
		u.RawQuery = q.Encode()
		raw, h, e := c.request(ctx, "GET", u, nil, 200, MaxResponseBytes)
		if e != nil {
			return nil, e
		}
		total += len(raw)
		if total > MaxTotalBytes {
			return nil, errors.New("GitLab pagination byte budget exceeded")
		}
		var rows []T
		if e = decode(raw, &rows); e != nil || rows == nil || len(rows) > 100 {
			return nil, errors.New("invalid GitLab collection")
		}
		for _, v := range rows {
			k := id(v)
			if !valid(v) || k == "" || seen[k] {
				return nil, errors.New("invalid or duplicated GitLab identity")
			}
			seen[k] = true
			out = append(out, v)
		}
		next := h.Get("X-Next-Page")
		hasHeader := len(h.Values("X-Next-Page")) > 0
		for _, link := range strings.Split(h.Get("Link"), ",") {
			if !strings.Contains(link, `rel="next"`) {
				continue
			}
			parts := strings.Split(link, ";")
			rawURL := strings.Trim(strings.TrimSpace(parts[0]), "<>")
			v, e := url.Parse(rawURL)
			expected := *u
			eq := expected.Query()
			eq.Set("page", strconv.Itoa(p+1))
			expected.RawQuery = eq.Encode()
			if e != nil || v.Scheme != expected.Scheme || v.Host != expected.Host || v.EscapedPath() != expected.EscapedPath() || v.Query().Encode() != expected.Query().Encode() || v.User != nil || v.Fragment != "" {
				return nil, errors.New("unsafe GitLab pagination link")
			}
			if hasHeader && next != strconv.Itoa(p+1) {
				return nil, errors.New("conflicting GitLab pagination")
			}
			next = strconv.Itoa(p + 1)
		}
		if next == "" && (hasHeader || len(rows) < 100) {
			return out, nil
		}
		if next != "" && next != strconv.Itoa(p+1) {
			return nil, errors.New("nonsequential GitLab pagination")
		}
	}
	return nil, errors.New("GitLab page budget exceeded")
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}
type Issue struct {
	ID           int64           `json:"id"`
	IID          int             `json:"iid"`
	ProjectID    int64           `json:"project_id"`
	Title        string          `json:"title"`
	Body         string          `json:"description"`
	State        string          `json:"state"`
	Type         string          `json:"issue_type"`
	WebURL       string          `json:"web_url"`
	Author       User            `json:"author"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	Labels       []string        `json:"labels"`
	Assignees    []User          `json:"assignees"`
	Confidential bool            `json:"confidential"`
	Raw          json.RawMessage `json:"-"`
}

func (v *Issue) UnmarshalJSON(b []byte) error {
	type dto Issue
	var x dto
	if e := json.Unmarshal(b, &x); e != nil {
		return e
	}
	*v = Issue(x)
	v.Raw = append([]byte{}, b...)
	return nil
}

type Note struct {
	ID           int64           `json:"id"`
	Body         string          `json:"body"`
	Author       User            `json:"author"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	System       bool            `json:"system"`
	Internal     bool            `json:"internal"`
	Confidential bool            `json:"confidential"`
	Type         string          `json:"type"`
	NoteableID   int64           `json:"noteable_id"`
	Raw          json.RawMessage `json:"-"`
}

// Ordinary excludes both current and legacy private-note flags. Older v4
// instances may omit internal while still returning confidential:true.
func (v Note) Ordinary() bool { return !v.System && !v.Internal && !v.Confidential && v.Type == "" }

func (v *Note) UnmarshalJSON(b []byte) error {
	type dto Note
	var x dto
	if e := json.Unmarshal(b, &x); e != nil {
		return e
	}
	*v = Note(x)
	v.Raw = append([]byte{}, b...)
	return nil
}

type IssueInput struct {
	Title, Body, State, Type string
	Labels, Assignees        []string
}

func validIssue(v Issue) bool {
	return v.ID > 0 && v.IID > 0 && v.ProjectID > 0 && (v.State == "opened" || v.State == "closed")
}
func (c *Client) GetIssue(ctx context.Context, p string, iid int) (Issue, error) {
	if iid <= 0 {
		return Issue{}, errors.New("positive issue IID required")
	}
	u, e := c.projectURL(p, "/issues/"+strconv.Itoa(iid))
	if e != nil {
		return Issue{}, e
	}
	b, _, e := c.request(ctx, "GET", u, nil, 200, MaxResponseBytes)
	if e != nil {
		return Issue{}, e
	}
	var v Issue
	e = decode(b, &v)
	if e != nil || !validIssue(v) || v.IID != iid {
		return Issue{}, errors.New("invalid GitLab issue identity/state")
	}
	return v, nil
}
func (c *Client) ListIssues(ctx context.Context, p string) ([]Issue, error) {
	u, e := c.projectURL(p, "/issues")
	if e != nil {
		return nil, e
	}
	u.RawQuery = "scope=all&state=all"
	return list(ctx, c, u, validIssue, func(v Issue) string { return strconv.FormatInt(v.ID, 10) })
}
func (c *Client) ListNotes(ctx context.Context, p string, iid int) ([]Note, error) {
	if iid <= 0 {
		return nil, errors.New("positive issue IID required")
	}
	u, e := c.projectURL(p, "/issues/"+strconv.Itoa(iid)+"/notes")
	if e != nil {
		return nil, e
	}
	u.RawQuery = "sort=asc&order_by=created_at"
	return list(ctx, c, u, func(v Note) bool { return v.ID > 0 }, func(v Note) string { return strconv.FormatInt(v.ID, 10) })
}
func issuePayload(in IssueInput, create bool) (map[string]any, error) {
	if e := ValidateBody(in.Body); e != nil {
		return nil, e
	}
	if in.Title == "" || len(in.Title) > 255 || !utf8.ValidString(in.Title) || len(in.Body) > 1000000 || !utf8.ValidString(in.Body) || (in.Type != "" && in.Type != "issue") || (in.State != "opened" && in.State != "closed") || create && in.State != "opened" {
		return nil, errors.New("GitLab supports bounded ordinary issues; creates require opened state")
	}
	ids := []int64{}
	for _, s := range in.Assignees {
		n, e := strconv.ParseInt(s, 10, 64)
		if e != nil || n <= 0 || strconv.FormatInt(n, 10) != s {
			return nil, errors.New("GitLab assignees require explicit numeric user IDs")
		}
		ids = append(ids, n)
	}
	for _, s := range in.Labels {
		if strings.ContainsAny(s, ",\x00\r\n") || s == "" {
			return nil, errors.New("GitLab label cannot contain commas or control characters")
		}
	}
	v := map[string]any{"title": in.Title, "description": in.Body, "issue_type": "issue", "labels": strings.Join(in.Labels, ","), "assignee_ids": ids}
	if !create {
		if in.State == "closed" {
			v["state_event"] = "close"
		} else {
			v["state_event"] = "reopen"
		}
	}
	return v, nil
}
func (c *Client) writeIssue(ctx context.Context, p string, iid int, in IssueInput) (Issue, error) {
	v, e := issuePayload(in, iid == 0)
	if e != nil {
		return Issue{}, e
	}
	suffix := "/issues"
	method := "POST"
	want := 201
	if iid != 0 {
		if iid < 0 {
			return Issue{}, errors.New("invalid IID")
		}
		suffix += "/" + strconv.Itoa(iid)
		method = "PUT"
		want = 200
	}
	u, e := c.projectURL(p, suffix)
	if e != nil {
		return Issue{}, e
	}
	b, _, e := c.request(ctx, method, u, v, want, MaxResponseBytes)
	if e != nil {
		return Issue{}, e
	}
	var out Issue
	if e = decode(b, &out); e != nil || !validIssue(out) || iid != 0 && out.IID != iid {
		return Issue{}, errors.New("invalid written GitLab issue; recovery required")
	}
	return out, nil
}
func (c *Client) CreateIssue(ctx context.Context, p string, in IssueInput) (Issue, error) {
	return c.writeIssue(ctx, p, 0, in)
}
func (c *Client) UpdateIssue(ctx context.Context, p string, iid int, in IssueInput) (Issue, error) {
	if iid <= 0 {
		return Issue{}, errors.New("invalid IID")
	}
	return c.writeIssue(ctx, p, iid, in)
}
func (c *Client) CreateNote(ctx context.Context, p string, iid int, body string) (Note, error) {
	if e := ValidateBody(body); e != nil {
		return Note{}, e
	}
	if iid <= 0 || body == "" || len(body) > 1000000 || !utf8.ValidString(body) {
		return Note{}, errors.New("invalid GitLab note")
	}
	u, e := c.projectURL(p, "/issues/"+strconv.Itoa(iid)+"/notes")
	if e != nil {
		return Note{}, e
	}
	b, _, e := c.request(ctx, "POST", u, map[string]string{"body": body}, 201, MaxResponseBytes)
	if e != nil {
		return Note{}, e
	}
	var v Note
	if e = decode(b, &v); e != nil || v.ID <= 0 || !v.Ordinary() {
		return Note{}, errors.New("invalid written GitLab note; recovery required")
	}
	return v, nil
}

func (c *Client) GetNote(ctx context.Context, p string, iid int, noteID int64) (Note, error) {
	if iid <= 0 || noteID <= 0 {
		return Note{}, errors.New("positive note parent IID and immutable note ID required")
	}
	u, e := c.projectURL(p, "/issues/"+strconv.Itoa(iid)+"/notes/"+strconv.FormatInt(noteID, 10))
	if e != nil {
		return Note{}, e
	}
	b, _, e := c.request(ctx, "GET", u, nil, 200, MaxResponseBytes)
	if e != nil {
		return Note{}, e
	}
	var v Note
	if e = decode(b, &v); e != nil || v.ID != noteID {
		return Note{}, errors.New("invalid GitLab note identity")
	}
	return v, nil
}
func (c *Client) UpdateNote(ctx context.Context, p string, iid int, noteID int64, body string) (Note, error) {
	if e := ValidateBody(body); e != nil {
		return Note{}, e
	}
	if iid <= 0 || noteID <= 0 || body == "" || len(body) > 1000000 || !utf8.ValidString(body) {
		return Note{}, errors.New("invalid GitLab note update")
	}
	u, e := c.projectURL(p, "/issues/"+strconv.Itoa(iid)+"/notes/"+strconv.FormatInt(noteID, 10))
	if e != nil {
		return Note{}, e
	}
	b, _, e := c.request(ctx, "PUT", u, map[string]string{"body": body}, 200, MaxResponseBytes)
	if e != nil {
		return Note{}, e
	}
	var v Note
	if e = decode(b, &v); e != nil || v.ID != noteID || !v.Ordinary() {
		return Note{}, errors.New("invalid written GitLab note identity; recovery required")
	}
	return v, nil
}

// ValidateBody refuses potential quick actions rather than escaping source
// bytes. GitLab executes slash-command lines in descriptions and notes under
// the supplied token. A conservative refusal includes future server commands
// and fenced examples; the original text remains in the migration snapshot.
func ValidateBody(body string) error {
	for _, line := range strings.FieldsFunc(body, func(r rune) bool { return r == '\n' || r == '\r' || r == '\u0085' || r == '\u2028' || r == '\u2029' }) {
		if strings.HasPrefix(strings.TrimSpace(line), "/") {
			return errors.New("GitLab slash-command lines are refused; preserve the original source text without hosted execution")
		}
	}
	return nil
}
