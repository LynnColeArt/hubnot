//go:build !nogithub

package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGraphQLTransportRefusesPartialAndAmbiguousJSON(t *testing.T) {
	for _, raw := range []string{
		`{"data":{"ok":true},"errors":[{"message":"secret-token"}]}`,
		`{"data":{"ok":true},"data":{"ok":false}}`,
		`{"data":{"ok":true,"ok":false}}`,
		`{"data":{"ok":true}} {}`,
		`{"data":null}`, `{"data":{"ok":"\ud800"}}`,
		`{"data":{"ok":true},"errors":{}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(raw)) }))
			defer s.Close()
			c, _ := NewClient(s.URL, "secret-token")
			var out struct {
				OK bool `json:"ok"`
			}
			err := c.graphQL(context.Background(), "query { viewer { login } }", nil, &out, &graphQLBudget{})
			if err == nil || strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("unsafe acceptance/error: %v", err)
			}
		})
	}
}

func TestGraphQLEndpointAuthenticationAndBudget(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/graphql" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Query     string
			Variables map[string]any
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Variables["value"] != "user \" text" {
			t.Error("variables lost")
		}
		w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer s.Close()
	c, _ := NewClient(s.URL+"/api/v3", "token")
	var out struct{ OK bool }
	if err := c.graphQL(context.Background(), "query Fixed($value:String!){viewer{login}}", map[string]any{"value": "user \" text"}, &out, &graphQLBudget{}); err != nil || !out.OK {
		t.Fatal(err)
	}
	anonymous, _ := NewClient(s.URL, " ")
	_ = anonymous
	noToken, _ := NewClient(s.URL, "")
	if noToken.graphQL(context.Background(), "query{}", nil, &out, &graphQLBudget{}) == nil {
		t.Fatal("anonymous GraphQL")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.graphQL(ctx, "query{}", nil, &out, &graphQLBudget{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if c.graphQL(context.Background(), "query{}", nil, &out, &graphQLBudget{bytes: MaxTotalBytes}) == nil {
		t.Fatal("aggregate budget accepted")
	}
	if calls != 1 {
		t.Fatalf("unexpected calls %d", calls)
	}
}

func TestGraphQLSecurityAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		redirect bool
	}{
		{name: "HTTP redaction", status: 403, body: "secret-token"},
		{name: "redirect", status: 302, body: "secret-token", redirect: true},
		{name: "oversize", status: 200, body: `{"data":{"text":"` + strings.Repeat("x", MaxResponseBytes) + `"}}`},
		{name: "nesting", status: 200, body: `{"data":{"x":` + strings.Repeat("[", 70) + `0` + strings.Repeat("]", 70) + `}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if tc.redirect {
					w.Header().Set("Location", "https://example.invalid/steal")
				}
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer s.Close()
			c, _ := NewClient(s.URL, "secret-token")
			var out map[string]any
			err := c.graphQL(context.Background(), "mutation{anything}", nil, &out, &graphQLBudget{})
			if err == nil || strings.Contains(err.Error(), "secret-token") || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}
func TestGraphQLAggregateLimitNoPartialAcceptance(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data":{"ok":true}}`)) }))
	defer s.Close()
	c, _ := NewClient(s.URL, "token")
	var out map[string]any
	err := c.graphQL(context.Background(), "query{viewer{login}}", nil, &out, &graphQLBudget{bytes: MaxTotalBytes - 2})
	if err == nil || out != nil {
		t.Fatalf("partial result %v %v", out, err)
	}
}
func TestGraphQLConnectionsFailClosed(t *testing.T) {
	for _, mode := range []string{"missing-cursor", "missing-pageinfo", "missing-nodes", "null-node", "duplicate", "oversize", "page-limit"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			yes, no := true, false
			rows, err := graphQLPages(func(after *string) (*graphQLConnection[graphQLRef], error) {
				calls++
				cursor := strings.Repeat("x", calls)
				conn := &graphQLConnection[graphQLRef]{Nodes: []*graphQLRef{}, PageInfo: &graphQLPageInfo{HasNextPage: &no}}
				switch mode {
				case "missing-cursor":
					conn.PageInfo.HasNextPage = &yes
				case "missing-pageinfo":
					conn.PageInfo = nil
				case "missing-nodes":
					conn.Nodes = nil
				case "null-node":
					conn.Nodes = []*graphQLRef{nil}
				case "duplicate":
					conn.Nodes = []*graphQLRef{{ID: "X"}, {ID: "X"}}
				case "oversize":
					for i := 0; i < 101; i++ {
						conn.Nodes = append(conn.Nodes, &graphQLRef{ID: strings.Repeat("x", i+1)})
					}
				case "page-limit":
					conn.PageInfo.HasNextPage = &yes
					conn.PageInfo.EndCursor = &cursor
				}
				return conn, nil
			}, func(n *graphQLRef) bool { return validGraphQLID(n.ID) }, func(n *graphQLRef) string { return n.ID })
			if err == nil || rows != nil {
				t.Fatalf("partial slice accepted %+v %v", rows, err)
			}
			if mode == "page-limit" && calls != 100 {
				t.Fatalf("calls %d", calls)
			}
		})
	}
}
