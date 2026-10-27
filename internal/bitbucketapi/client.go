//go:build !noforge

// Package bitbucketapi implements bounded Bitbucket Cloud REST 2.0 reads.
// It does not implement Bitbucket Data Center or the retired Issue Tracker.
package bitbucketapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"hubnot/internal/migration"
)

const (
	MaxResponseBytes int64 = 4 << 20
	MaxTotalBytes    int64 = 64 << 20
	MaxPages               = 100
	MaxRecords             = 10000
)

type Client struct {
	base   *url.URL
	token  string
	mode   string
	http   *http.Client
	budget *readBudget
}
type readBudget struct {
	mu   sync.Mutex
	left int64
}
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Bitbucket Cloud HTTP %d; selected content unavailable", e.StatusCode)
}

// NewClient is offline. Production endpoints are Cloud only; explicit loopback
// roots permit deterministic local fixtures, never arbitrary Data Center roots.
// CredentialMode must be anonymous, oauth2, api_token_bearer, or api_token_basic.
// Basic's opaque transient Token is email:api-token (split at the first colon).
// No value is read from the environment or persisted. App passwords are retired.
// https://developer.atlassian.com/cloud/bitbucket/changelog/ (2026-08-18)
// https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/
func NewClient(endpoint, token, mode string) (*Client, error) {
	if endpoint == "" {
		endpoint = "https://api.bitbucket.org/2.0"
	}
	normalized, e := migration.NormalizeInstance(endpoint)
	if e != nil {
		return nil, errors.New("invalid Bitbucket Cloud endpoint")
	}
	u, e := url.Parse(normalized)
	if e != nil {
		return nil, errors.New("invalid Bitbucket Cloud endpoint")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	loop := host == "localhost" || ip != nil && ip.IsLoopback()
	if !(u.Scheme == "https" && u.Host == "api.bitbucket.org" || loop) || u.Path != "" && u.Path != "/2.0" || u.Opaque != "" {
		return nil, errors.New("requires Bitbucket Cloud REST 2.0 endpoint; Data Center is unsupported")
	}
	u.Path = "/2.0"
	if len(token) > 16384 || strings.ContainsAny(token, "\x00\r\n") || !utf8.ValidString(token) {
		return nil, errors.New("invalid supplied Bitbucket credential")
	}
	switch mode {
	case "anonymous":
		if token != "" {
			return nil, errors.New("anonymous mode cannot carry a credential")
		}
	case "oauth2", "api_token_bearer":
		if token == "" || strings.ContainsAny(token, " \t") {
			return nil, errors.New("explicit bearer token required")
		}
	case "api_token_basic":
		email, secret, ok := strings.Cut(token, ":")
		a, err := mail.ParseAddress(email)
		if !ok || err != nil || a.Address != email || len(email) > 320 || secret == "" || strings.ContainsAny(secret, " \t") {
			return nil, errors.New("api_token_basic requires supplied email:api-token")
		}
	default:
		return nil, errors.New("select anonymous, oauth2, api_token_bearer or api_token_basic; app passwords unsupported")
	}
	h := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Bitbucket redirects refused") }}
	return &Client{base: u, token: token, mode: mode, http: h, budget: &readBudget{left: MaxTotalBytes}}, nil
}
func (c *Client) BaseURL() string { return c.base.String() }

// WithBudget makes an operation-local aggregate wire budget without mutating c.
func (c *Client) WithBudget(max int64) *Client {
	v := *c
	if max > MaxTotalBytes {
		max = MaxTotalBytes
	}
	v.budget = &readBudget{left: max}
	return &v
}
func ValidateRepository(repo string) error {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return errors.New("Bitbucket repository requires workspace/slug")
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 256 || p == "." || p == ".." {
			return errors.New("invalid Bitbucket repository selector")
		}
		for _, r := range p {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.", r)) {
				return errors.New("invalid Bitbucket repository selector")
			}
		}
	}
	return nil
}
func (c *Client) repoURL(repo, suffix string) (*url.URL, error) {
	if e := ValidateRepository(repo); e != nil {
		return nil, e
	}
	u := *c.base
	u.Path += "/repositories/" + repo + suffix
	return &u, nil
}
func (c *Client) validURL(u *url.URL) bool {
	return u != nil && u.User == nil && u.Fragment == "" && u.Opaque == "" && u.Scheme == c.base.Scheme && strings.EqualFold(u.Host, c.base.Host) && strings.HasPrefix(u.Path, c.base.Path+"/") && !strings.Contains(u.Path, "..") && u.RawPath == ""
}
func (c *Client) get(ctx context.Context, u *url.URL, max int64) ([]byte, error) {
	if !c.validURL(u) || max <= 0 || max > 1<<54 {
		return nil, errors.New("Bitbucket request outside configured origin/root or invalid budget")
	}
	c.budget.mu.Lock()
	defer c.budget.mu.Unlock()
	if c.budget.left <= 0 {
		return nil, errors.New("Bitbucket aggregate wire budget exceeded")
	}
	if max > c.budget.left {
		max = c.budget.left
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return nil, errors.New("invalid Bitbucket request")
	}
	req.Header.Set("Accept", "application/json")
	if c.mode == "api_token_basic" {
		email, token, _ := strings.Cut(c.token, ":")
		req.SetBasicAuth(email, token)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, errors.New("Bitbucket request failed; no automatic retry")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{resp.StatusCode}
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, max+1))
	c.budget.left -= int64(len(raw))
	if e != nil || int64(len(raw)) > max {
		return nil, errors.New("Bitbucket response/aggregate byte budget exceeded or interrupted")
	}
	return raw, nil
}

// decode allows provider extensions and null values but rejects duplicate keys,
// trailing values, excessive nesting and invalid UTF-8 before typed decoding.
func decode(raw []byte, out any) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid Bitbucket JSON encoding")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 32 {
			return errors.New("Bitbucket JSON nesting budget exceeded")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					key, ok := k.(string)
					if !ok || seen[key] {
						return errors.New("duplicate Bitbucket JSON field")
					}
					seen[key] = true
					if e = value(depth + 1); e != nil {
						return e
					}
				}
			case '[':
				for d.More() {
					if e = value(depth + 1); e != nil {
						return e
					}
				}
			default:
				return errors.New("invalid Bitbucket JSON delimiter")
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if e := value(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing Bitbucket JSON")
	}
	return json.Unmarshal(raw, out)
}
func (c *Client) list(ctx context.Context, u *url.URL) ([]json.RawMessage, error) {
	rows := []json.RawMessage{}
	seen := map[string]bool{}
	for page := 0; u != nil; page++ {
		if page >= MaxPages || seen[u.String()] {
			return nil, errors.New("Bitbucket pagination cycle/page budget exceeded")
		}
		seen[u.String()] = true
		raw, e := c.get(ctx, u, MaxResponseBytes)
		if e != nil {
			return nil, e
		}
		var p struct {
			Values []json.RawMessage `json:"values"`
			Next   string            `json:"next"`
		}
		if e = decode(raw, &p); e != nil {
			return nil, e
		}
		if p.Values == nil {
			return nil, errors.New("Bitbucket response missing selected values")
		}
		if len(p.Values) > MaxRecords-len(rows) {
			return nil, errors.New("Bitbucket record budget exceeded")
		}
		rows = append(rows, p.Values...)
		if p.Next == "" {
			break
		}
		next, e := url.Parse(p.Next)
		if e != nil {
			return nil, errors.New("invalid Bitbucket next link")
		}
		next = u.ResolveReference(next)
		// Query cursors remain opaque. Bind to this exact connection path, not
		// merely the host: another same-origin resource is not a next page.
		if !c.validURL(next) || next.Path != u.Path {
			return nil, errors.New("Bitbucket next link outside connection origin/path")
		}
		u = next
	}
	return rows, nil
}
