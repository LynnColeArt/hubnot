//go:build !noforge

package bitbucketapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudIdentityAndRawSource(t *testing.T) {
	raw := []byte("pipelines:\r\n  default: []\r\n# exact bytes\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/src/"):
			w.Write(raw)
		case strings.HasSuffix(r.URL.Path, "/comments"):
			fmt.Fprint(w, `{"values":[{"id":3,"parent":{"id":2},"inline":{"path":"a.go","to":4},"content":{"raw":"reply"}}]}`)
		case strings.HasSuffix(r.URL.Path, "/activity"):
			fmt.Fprint(w, `{"values":[{"approval":{"date":"2026-01-02T00:00:00Z","user":{"uuid":"actor"}}}]}`)
		default:
			fmt.Fprint(w, `{"uuid":"{11111111-1111-1111-1111-111111111111}","full_name":"ws/repo","created_on":"2026-01-01T00:00:00Z"}`)
		}
	}))
	defer srv.Close()
	c, _ := NewClient(srv.URL, "", "anonymous")
	repo, e := c.Repository(context.Background(), "ws/repo")
	if e != nil || repo.UUID == "" {
		t.Fatal(repo, e)
	}
	cs, e := c.Comments(context.Background(), "ws/repo", 1)
	if e != nil || len(cs) != 1 || cs[0].Parent.ID != 2 || cs[0].Inline.Path != "a.go" {
		t.Fatal(cs, e)
	}
	a, e := c.Activity(context.Background(), "ws/repo", 1)
	if e != nil || len(a) != 1 || !strings.Contains(string(a[0]), "approval") {
		t.Fatal(a, e)
	}
	got, e := c.Pipeline(context.Background(), "ws/repo", strings.Repeat("a", 40), 1000)
	if e != nil || string(got) != string(raw) {
		t.Fatal(string(got), e)
	}
	if _, e = c.Pipeline(context.Background(), "ws/repo", "main", 1000); e == nil {
		t.Fatal("unpinned workflow accepted")
	}
	if _, e = c.Pipeline(context.Background(), "ws/repo", strings.Repeat("a", 40), 3); e == nil {
		t.Fatal("oversize accepted")
	}
	if _, e = c.WithBudget(5).Repository(context.Background(), "ws/repo"); e == nil {
		t.Fatal("aggregate budget ignored")
	}
}
