//go:build !noforge

// Package azureapi implements explicitly scoped Azure DevOps REST operations.
// It has no credential discovery, OAuth registration, SDK or execution authority.
package azureapi

import (
	"bytes"
	"context"
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
const PageSize = 200

// Profiles are fixture-qualified, not inferred from a hostname. Server 2022
// supports REST 7.0; WIT Comments is separately versioned preview. References:
// https://learn.microsoft.com/azure/devops/integrate/concepts/rest-api-versioning
// https://learn.microsoft.com/rest/api/azure/devops/wit/comments/add?view=azure-devops-rest-7.0
// https://learn.microsoft.com/rest/api/azure/devops/wit/comments/get-comments?view=azure-devops-rest-7.1
func Versions(profile string) (string, string, error) {
	switch profile {
	case "services-7.1":
		return "7.1", "7.1-preview.4", nil
	case "server-7.0":
		return "7.0", "7.0-preview.3", nil
	default:
		return "", "", errors.New("Azure requires services-7.1 or server-7.0 API profile")
	}
}

type readBudget struct {
	mu        sync.Mutex
	remaining int64
}
type Client struct {
	base                                  *url.URL
	token, mode, version, commentsVersion string
	http                                  *http.Client
	budget                                *readBudget
}
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string { return fmt.Sprintf("Azure API returned HTTP %d", e.StatusCode) }
func ValidSegment(s string) bool {
	return s != "" && len(s) <= 256 && utf8.ValidString(s) && strings.TrimSpace(s) == s && s != "." && s != ".." && !strings.ContainsAny(s, "/\\?#%\x00\r\n\t") && !strings.ContainsAny(s, "'\"")
}
func ValidGUID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' || strings.ToLower(s) != s {
		return false
	}
	_, e := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	return e == nil
}
func PositiveID(s string) (int, error) {
	n, e := strconv.Atoi(s)
	if e != nil || n <= 0 || n > 2147483647 || strconv.Itoa(n) != s {
		return 0, errors.New("Azure ID must be a canonical positive int32")
	}
	return n, nil
}
func NewClient(endpoint, token, mode, profile string) (*Client, error) {
	v, cv, e := Versions(profile)
	if e != nil {
		return nil, e
	}
	if mode != "bearer" && mode != "pat" {
		return nil, errors.New("Azure requires explicit supplied bearer or pat credential mode")
	}
	u, e := url.Parse(endpoint)
	if e != nil || u == nil || u.Host == "" || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || strings.Contains(endpoint, "#") || u.RawPath != "" || strings.HasSuffix(u.Host, ":") {
		return nil, errors.New("invalid Azure endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return nil, errors.New("Azure requires HTTPS or explicit loopback HTTP")
	}
	if p := u.Port(); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid Azure port")
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if path.Clean(u.Path) != u.Path && u.Path != "" || strings.ContainsAny(u.Path, "\\\x00\r\n\t") {
		return nil, errors.New("invalid Azure endpoint path")
	}
	for _, s := range strings.Split(strings.TrimPrefix(u.Path, "/"), "/") {
		if s != "" && !ValidSegment(s) {
			return nil, errors.New("invalid Azure endpoint segment")
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
			return nil, errors.New("invalid Azure token")
		}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxResponseHeaderBytes = 64 << 10
	return &Client{base: u, token: token, mode: mode, version: v, commentsVersion: cv, http: &http.Client{Timeout: 30 * time.Second, Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) BaseURL() string { return c.base.String() }
func (c *Client) BoundedReadClient(n int64) *Client {
	v := *c
	if n < 0 {
		n = 0
	}
	if n > MaxTotalBytes {
		n = MaxTotalBytes
	}
	v.budget = &readBudget{remaining: n}
	return &v
}
func (c *Client) endpoint(parts []string, q url.Values, comments bool) (string, error) {
	u := *c.base
	for _, p := range parts {
		if !ValidSegment(p) {
			return "", errors.New("invalid Azure path selector")
		}
		u.Path += "/" + p
	}
	if q == nil {
		q = url.Values{}
	}
	v := c.version
	if comments {
		v = c.commentsVersion
	}
	q.Set("api-version", v)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func (c *Client) request(ctx context.Context, method, raw, media string, body []byte, limit int64) ([]byte, http.Header, error) {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.Scheme != c.base.Scheme || u.Host != c.base.Host || u.RawPath != "" || !strings.HasPrefix(u.Path, c.base.Path+"/") || path.Clean(u.Path) != u.Path || strings.ContainsAny(u.Path, "\\\x00\r\n") {
		return nil, nil, errors.New("Azure refuses foreign or ambiguous resource URL")
	}
	req, e := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if e != nil {
		return nil, nil, errors.New("invalid Azure request")
	}
	req.Header.Set("Accept", "application/json")
	if media != "" {
		req.Header.Set("Content-Type", media)
	}
	if c.token != "" {
		if c.mode == "pat" {
			req.SetBasicAuth("", c.token)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
	}
	res, e := c.http.Do(req)
	if e != nil {
		return nil, nil, errors.New("Azure transport failed; remote effect may be uncertain")
	}
	defer res.Body.Close()
	if limit < 0 {
		return nil, nil, errors.New("invalid Azure response limit")
	}
	if limit > MaxTotalBytes {
		limit = MaxTotalBytes
	}
	if c.budget != nil {
		c.budget.mu.Lock()
		defer c.budget.mu.Unlock()
		if limit > c.budget.remaining {
			limit = c.budget.remaining
		}
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if c.budget != nil {
		c.budget.remaining -= int64(len(data))
		if c.budget.remaining < 0 {
			c.budget.remaining = 0
		}
	}
	if e != nil {
		return nil, nil, errors.New("Azure response read failed")
	}
	if int64(len(data)) > limit {
		return nil, nil, errors.New("Azure response or aggregate read budget exceeded")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, nil, &HTTPError{res.StatusCode}
	}
	return data, res.Header, nil
}
func (c *Client) json(ctx context.Context, method string, parts []string, q url.Values, comments bool, input, out any, patch bool) (http.Header, error) {
	u, e := c.endpoint(parts, q, comments)
	if e != nil {
		return nil, e
	}
	var body []byte
	if input != nil {
		body, e = json.Marshal(input)
		if e != nil {
			return nil, e
		}
	}
	media := "application/json"
	if patch {
		media = "application/json-patch+json"
	}
	b, h, e := c.request(ctx, method, u, media, body, MaxResponseBytes)
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, out); e != nil {
		return nil, errors.New("invalid Azure API JSON")
	}
	return h, nil
}
func (c *Client) Download(ctx context.Context, raw string, limit int64) ([]byte, string, error) {
	b, h, e := c.request(ctx, "GET", raw, "", nil, limit)
	return b, h.Get("Content-Type"), e
}
