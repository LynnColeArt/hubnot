//go:build !noforge

package bitbucketapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"hubnot/internal/migration"
)

type Repository struct {
	UUID      string          `json:"uuid"`
	FullName  string          `json:"full_name"`
	CreatedOn string          `json:"created_on"`
	Raw       json.RawMessage `json:"-"`
}
type Account struct {
	UUID        string `json:"uuid"`
	DisplayName string `json:"display_name"`
}
type Object struct {
	ID        int64   `json:"id"`
	Author    Account `json:"author"`
	User      Account `json:"user"`
	CreatedOn string  `json:"created_on"`
	UpdatedOn string  `json:"updated_on"`
	Deleted   bool    `json:"deleted"`
	Parent    *struct {
		ID int64 `json:"id"`
	} `json:"parent"`
	Inline *struct {
		Path string `json:"path"`
	} `json:"inline"`
	Raw json.RawMessage `json:"-"`
}

func validUUID(s string) bool {
	if len(s) != 38 || s[0] != '{' || s[37] != '}' {
		return false
	}
	v := s[1:37]
	if v[8] != '-' || v[13] != '-' || v[18] != '-' || v[23] != '-' {
		return false
	}
	_, e := hex.DecodeString(strings.ReplaceAll(v, "-", ""))
	return e == nil
}
func (c *Client) Repository(ctx context.Context, repo string) (Repository, error) {
	u, e := c.repoURL(repo, "")
	if e != nil {
		return Repository{}, e
	}
	raw, e := c.get(ctx, u, MaxResponseBytes)
	if e != nil {
		return Repository{}, e
	}
	var v Repository
	if e = decode(raw, &v); e != nil {
		return Repository{}, e
	}
	_, te := time.Parse(time.RFC3339Nano, v.CreatedOn)
	if !validUUID(v.UUID) || v.FullName != repo || te != nil {
		return Repository{}, errors.New("invalid or replaced Bitbucket repository identity")
	}
	v.UUID = strings.ToLower(v.UUID)
	v.Raw = append([]byte{}, raw...)
	return v, nil
}
func (c *Client) objects(ctx context.Context, repo, suffix string, allStates bool) ([]Object, error) {
	u, e := c.repoURL(repo, suffix)
	if e != nil {
		return nil, e
	}
	if allStates {
		q := u.Query()
		for _, s := range []string{"OPEN", "MERGED", "DECLINED", "SUPERSEDED"} {
			q.Add("state", s)
		}
		u.RawQuery = q.Encode()
	}
	rows, e := c.list(ctx, u)
	if e != nil {
		return nil, e
	}
	out := []Object{}
	seen := map[int64]bool{}
	for _, raw := range rows {
		var v Object
		if e = decode(raw, &v); e != nil {
			return nil, e
		}
		if v.ID <= 0 || seen[v.ID] || v.Parent != nil && v.Parent.ID <= 0 {
			return nil, errors.New("invalid/duplicate Bitbucket object identity")
		}
		seen[v.ID] = true
		v.Raw = append([]byte{}, raw...)
		out = append(out, v)
	}
	return out, nil
}
func (c *Client) PullRequests(ctx context.Context, repo string) ([]Object, error) {
	return c.objects(ctx, repo, "/pullrequests", true)
}
func (c *Client) Comments(ctx context.Context, repo string, pr int64) ([]Object, error) {
	if pr <= 0 {
		return nil, errors.New("invalid PR ID")
	}
	return c.objects(ctx, repo, "/pullrequests/"+strconv.FormatInt(pr, 10)+"/comments", false)
}

// Activity has no universal immutable event ID. Callers preserve exact observed
// bytes with content identities; they must not invent historical event IDs.
func (c *Client) Activity(ctx context.Context, repo string, pr int64) ([]json.RawMessage, error) {
	if pr <= 0 {
		return nil, errors.New("invalid PR ID")
	}
	u, e := c.repoURL(repo, "/pullrequests/"+strconv.FormatInt(pr, 10)+"/activity")
	if e != nil {
		return nil, e
	}
	return c.list(ctx, u)
}
func (c *Client) Pipeline(ctx context.Context, repo, commit string, max int64) ([]byte, error) {
	if !migration.ValidOID(commit) {
		return nil, errors.New("pipeline source requires explicit immutable commit hash")
	}
	u, e := c.repoURL(repo, "/src/"+commit+"/bitbucket-pipelines.yml")
	if e != nil {
		return nil, e
	}
	return c.get(ctx, u, max)
}

// Attachment reads are explicitly selected and stay within the API origin/root.
func (c *Client) Attachment(ctx context.Context, rawURL string, max int64) ([]byte, error) {
	u, e := c.base.Parse(rawURL)
	if e != nil {
		return nil, errors.New("invalid selected attachment URL")
	}
	return c.get(ctx, u, max)
}
