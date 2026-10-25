//go:build !nogithub

package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// graphQLBudget is shared by every connection traversed in one public method.
// Callers adding typed operations must pass the same budget to all nested reads.
type graphQLBudget struct{ bytes int }

func (c *Client) graphQL(ctx context.Context, document string, variables any, out any, budget *graphQLBudget) error {
	if c.token == "" {
		return errors.New("GitHub GraphQL requires HN_GITHUB_TOKEN")
	}
	if budget == nil || budget.bytes >= MaxTotalBytes {
		return errors.New("GitHub GraphQL exceeds 64 MiB aggregate limit")
	}
	if strings.TrimSpace(document) == "" || !utf8.ValidString(document) {
		return errors.New("invalid GitHub GraphQL document")
	}
	u := *c.base
	switch u.Path {
	case "":
		u.Path = "/graphql"
	case "/api/v3":
		u.Path = "/api/graphql"
	default:
		return errors.New("GitHub GraphQL requires the root API URL or enterprise /api/v3 base")
	}
	input := struct {
		Query     string `json:"query"`
		Variables any    `json:"variables"`
	}{document, variables}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > MaxResponseBytes {
		return errors.New("GitHub GraphQL request exceeds JSON limits")
	}
	raw, _, err := c.request(ctx, http.MethodPost, &u, input, http.StatusOK)
	if err != nil {
		return err
	}
	budget.bytes += len(raw)
	if budget.bytes > MaxTotalBytes {
		return errors.New("GitHub GraphQL exceeds 64 MiB aggregate limit")
	}
	if !strictGraphQLJSON(raw) {
		return errors.New("invalid or ambiguous GitHub GraphQL JSON")
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Data) == 0 || envelope.Data[0] != '{' {
		return errors.New("GitHub GraphQL response has no complete data object")
	}
	if len(envelope.Errors) > 0 && string(envelope.Errors) != "null" {
		var failures []json.RawMessage
		if json.Unmarshal(envelope.Errors, &failures) != nil || len(failures) > 0 {
			return errors.New("GitHub GraphQL returned errors; partial data refused and write outcome may be unknown")
		}
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return errors.New("invalid GitHub GraphQL response shape")
	}
	return nil
}

// encoding/json otherwise accepts duplicate object keys and silently overwrites
// earlier values. Bound nesting as well as bytes before decoding hostile input.
func strictGraphQLJSON(raw []byte) bool {
	if !validJSONUnicode(raw) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 64 {
			return false
		}
		token, err := d.Token()
		if err != nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return false
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return false
				}
				seen[s] = true
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	}
	if !value(0) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}

type graphQLPageInfo struct {
	HasNextPage *bool   `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}
type graphQLConnection[T any] struct {
	Nodes    []*T             `json:"nodes"`
	PageInfo *graphQLPageInfo `json:"pageInfo"`
}

// Each connection has its own cursor history; the operation shares byte budget.
func graphQLPages[T any](fetch func(*string) (*graphQLConnection[T], error), valid func(*T) bool, id func(*T) string) ([]T, error) {
	result := []T{}
	seenIDs := map[string]bool{}
	seenCursors := map[string]bool{}
	var cursor *string
	for page := 0; page < MaxPages; page++ {
		conn, err := fetch(cursor)
		if err != nil {
			return nil, err
		}
		if conn == nil || conn.Nodes == nil || len(conn.Nodes) > 100 || conn.PageInfo == nil || conn.PageInfo.HasNextPage == nil {
			return nil, errors.New("incomplete GitHub GraphQL connection")
		}
		for _, n := range conn.Nodes {
			if n == nil || !valid(n) || seenIDs[id(n)] {
				return nil, errors.New("invalid or duplicate GitHub GraphQL node")
			}
			seenIDs[id(n)] = true
			result = append(result, *n)
		}
		if !*conn.PageInfo.HasNextPage {
			return result, nil
		}
		next := conn.PageInfo.EndCursor
		if next == nil || !validGraphQLID(*next) || seenCursors[*next] {
			return nil, errors.New("invalid or repeated GitHub GraphQL cursor")
		}
		seenCursors[*next] = true
		cursor = next
	}
	return nil, errors.New("GitHub GraphQL connection exceeds 100 page limit")
}

func validGraphQLID(id string) bool {
	if id == "" || len(id) > 1024 || !utf8.ValidString(id) {
		return false
	}
	for _, r := range id {
		if r <= 32 || r == 127 {
			return false
		}
	}
	return true
}
