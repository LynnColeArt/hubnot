//go:build !nogithub && !noforge

package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMigrationPullPaginationRetainsRawTopology(t *testing.T) {
	c, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "all" {
			t.Error("missing all-state selection")
		}
		page := r.URL.Query().Get("page")
		id := 1
		if page == "1" {
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/a/b/pulls?per_page=100&page=2&state=all>; rel="next"`, "http://"+r.Host))
		} else {
			id = 2
		}
		fmt.Fprintf(w, `[{"id":%d,"node_id":"P%d","number":%d,"head":{"sha":"exact-head"},"draft":true}]`, id, id, id)
	}, "secret")
	defer server.Close()
	rows, e := c.ListPullRequests(context.Background(), "a/b")
	if e != nil || len(rows) != 2 || !strings.Contains(string(rows[1].Raw), `"sha":"exact-head"`) {
		t.Fatal(rows, e)
	}
}
func TestMigrationAttachmentsNeverForwardToRedirectOrForeignOrigin(t *testing.T) {
	foreignCalls := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls++; t.Error("foreign origin requested") }))
	defer foreign.Close()
	c, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing same-origin credential")
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, foreign.URL, http.StatusFound)
			return
		}
		if r.URL.Path == "/private" {
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, "selected-blob")
	}, "secret")
	defer server.Close()
	for _, u := range []string{foreign.URL, server.URL + "/redirect", server.URL + "/private"} {
		if _, _, e := c.ReadAttachment(context.Background(), u, 100); e == nil {
			t.Fatal("unsafe attachment accepted", u)
		}
	}
	b, _, e := c.ReadAttachment(context.Background(), server.URL+"/blob", 100)
	if e != nil || string(b) != "selected-blob" {
		t.Fatal(string(b), e)
	}
	if _, _, e = c.ReadAttachment(context.Background(), server.URL+"/blob", 3); e == nil {
		t.Fatal("unbounded blob")
	}
	if foreignCalls != 0 {
		t.Fatal(foreignCalls)
	}
}
func TestMigrationSharedWireBudgetAndAmbiguousJSON(t *testing.T) {
	c, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "node_id": "R1", "full_name": "a/b", "created_at": "2026-01-01T00:00:00Z"})
	}, "")
	defer server.Close()
	bounded := c.BoundedReadClient(150)
	if _, e := bounded.GetRepository(context.Background(), "a/b"); e != nil {
		t.Fatal(e)
	}
	if _, e := bounded.GetRepository(context.Background(), "a/b"); e == nil {
		t.Fatal("aggregate budget reset across reads")
	}
	var v MigrationObject
	if json.Unmarshal([]byte(`{"id":1,"ID":2,"node_id":"n"}`), &v) == nil {
		t.Fatal("ambiguous API identity accepted")
	}
}

func TestMigrationWireBudgetAllowsExactEOFAndSharesAttachmentReads(t *testing.T) {
	raw := `{"id":1,"node_id":"R1","full_name":"a/b","created_at":"2026-01-01T00:00:00Z"}`
	c, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blob" {
			fmt.Fprint(w, "blob")
			return
		}
		fmt.Fprint(w, raw)
	}, "")
	defer server.Close()
	exact := c.BoundedReadClient(int64(len(raw)))
	if _, e := exact.GetRepository(context.Background(), "a/b"); e != nil {
		t.Fatal("exact response budget refused", e)
	}
	combined := c.BoundedReadClient(int64(len(raw) + 4))
	if _, e := combined.GetRepository(context.Background(), "a/b"); e != nil {
		t.Fatal(e)
	}
	if b, _, e := combined.ReadAttachment(context.Background(), server.URL+"/blob", 4); e != nil || string(b) != "blob" {
		t.Fatal("exact combined budget refused", e)
	}
	short := c.BoundedReadClient(int64(len(raw) + 3))
	if _, e := short.GetRepository(context.Background(), "a/b"); e != nil {
		t.Fatal(e)
	}
	if _, _, e := short.ReadAttachment(context.Background(), server.URL+"/blob", 4); e == nil {
		t.Fatal("attachment reset shared response budget")
	}
}

func TestMigrationReadBudgetIsSharedWithGraphQL(t *testing.T) {
	rest := `{"id":1,"node_id":"R1","full_name":"a/b","created_at":"2026-01-01T00:00:00Z"}`
	graphql := `{"data":{"repository":{"id":"R1","nameWithOwner":"a/b","hasDiscussionsEnabled":false}}}`
	c, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/graphql":
			fmt.Fprint(w, graphql)
		case "/blob":
			fmt.Fprint(w, "blob")
		default:
			fmt.Fprint(w, rest)
		}
	}, "secret")
	defer server.Close()
	for _, extra := range []int{3, 4} {
		bounded := c.BoundedReadClient(int64(len(rest) + len(graphql) + extra))
		if _, e := bounded.GetRepository(context.Background(), "a/b"); e != nil {
			t.Fatal(e)
		}
		if _, e := bounded.GetDiscussionRepository(context.Background(), "a/b"); e != nil {
			t.Fatal(e)
		}
		_, _, e := bounded.ReadAttachment(context.Background(), server.URL+"/blob", 4)
		if (e == nil) != (extra == 4) {
			t.Fatal("incorrect shared REST/GraphQL/attachment budget", extra, e)
		}
	}
}
func TestMigrationCanonicalOriginsRemoveDefaultPorts(t *testing.T) {
	for _, u := range []string{"https://EXAMPLE.invalid:443/", "http://127.0.0.1:80/"} {
		c, e := NewClient(u, "")
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(c.BaseURL(), ":443") || strings.Contains(c.BaseURL(), ":80") {
			t.Fatal("default port remained in instance identity", c.BaseURL())
		}
	}
}
