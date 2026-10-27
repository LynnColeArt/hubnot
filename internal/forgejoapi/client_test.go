//go:build !noforge

package forgejoapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForgejoClosedCreateUsesNumericLabels(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/a/b/issues" || r.Method != "POST" || r.Header.Get("Authorization") != "token explicit" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		var v map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			t.Fatal(err)
		}
		if string(v["closed"]) != "true" || string(v["labels"]) != "[41]" || v["state"] != nil {
			t.Errorf("wrong create projection %s", v)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":9001,"number":7,"title":"closed","state":"closed","labels":[{"id":41,"name":"bug"}],"assignees":[]}`))
	}))
	defer s.Close()
	c, e := NewClient(s.URL+"/api/v1", "explicit")
	if e != nil {
		t.Fatal(e)
	}
	v, e := c.CreateIssue(context.Background(), "a/b", IssueInput{Title: "closed", State: "closed", Labels: []int64{41}})
	if e != nil || v.ID != 9001 || v.Number != 7 {
		t.Fatal(v, e)
	}
}

func TestForgejoProfileVersionAndExplicitRoot(t *testing.T) {
	for _, major := range []string{"11", "16"} {
		t.Run(major, func(t *testing.T) {
			version := major + ".0.0+gitea-1.22.0"
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/forge/api/v1/version" {
					t.Error(r.URL)
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
			}))
			defer s.Close()
			c, e := NewProfileClient(s.URL+"/forge/api/v1/", "", "forgejo-"+major)
			if e != nil || calls != 0 {
				t.Fatal(e, calls)
			}
			if e = c.CheckVersion(context.Background()); e != nil {
				t.Fatal(e)
			}
			version = "1.27.3"
			if e = c.CheckVersion(context.Background()); e == nil {
				t.Fatal("Gitea accepted as Forgejo")
			}
		})
	}
	for _, root := range []string{"https://example.com", "https://a:p@example.com/api/v1", "https://example.com/api/v1?token=x", "http://example.com/api/v1", "https://example.com/x/../api/v1", "https://example.com/api/v1#"} {
		if _, e := NewClient(root, ""); e == nil {
			t.Fatal("accepted", root)
		}
	}
}

func TestForgejoPagingServerCapsAndHostileNext(t *testing.T) {
	for _, mode := range []string{"capped", "duplicate", "cycle", "foreign", "truncated", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("limit") != "3" {
					t.Error("configured limit lost")
				}
				page := r.URL.Query().Get("page")
				if mode == "foreign" {
					w.Header().Set("Link", `<https://other.example/api/v1/repos/a/b/issues?limit=3&page=2&state=all>; rel="next"`)
				}
				if mode == "cycle" {
					w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?limit=3&page=1&state=all>; rel="next"`, r.Host, r.URL.Path))
				}
				if mode == "truncated" {
					w.Header().Set("X-Total-Count", "2")
				}
				if mode == "oversized" {
					_, _ = w.Write([]byte(`[{"id":1,"number":1,"state":"open"},{"id":2,"number":2,"state":"open"},{"id":3,"number":3,"state":"open"},{"id":4,"number":4,"state":"open"}]`))
					return
				}
				if page == "1" || mode == "duplicate" && page == "2" {
					_, _ = w.Write([]byte(`[{"id":9001,"number":7,"state":"open"}]`))
				} else if mode == "capped" && page == "2" {
					_, _ = w.Write([]byte(`[{"id":9002,"number":8,"state":"closed"}]`))
				} else {
					_, _ = w.Write([]byte(`[]`))
				}
			}))
			defer s.Close()
			c, _ := NewClient(s.URL+"/api/v1", "")
			c.PageSize = 3
			vs, e := c.ListIssues(context.Background(), "a/b")
			if mode == "capped" {
				if e != nil || len(vs) != 2 || calls != 3 {
					t.Fatal(vs, calls, e)
				}
			} else if e == nil {
				t.Fatal("hostile pagination accepted", mode)
			}
		})
	}
}

func TestForgejoOriginRedirectAndExactWireBudgets(t *testing.T) {
	foreignCalls := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignCalls++
		t.Error("foreign origin contacted with", r.Header.Get("Authorization"))
	}))
	defer foreign.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, foreign.URL+"/asset", 302)
			return
		}
		_, _ = w.Write([]byte("12345"))
	}))
	defer s.Close()
	c, _ := NewClient(s.URL+"/api/v1", "secret")
	if _, _, e := c.ReadAttachment(context.Background(), foreign.URL+"/asset", 5); e == nil {
		t.Fatal("foreign asset accepted")
	}
	if _, _, e := c.ReadAttachment(context.Background(), s.URL+"/redirect", 5); e == nil {
		t.Fatal("redirect followed")
	}
	if foreignCalls != 0 {
		t.Fatal("credentials crossed origin")
	}
	for _, n := range []int64{4, 5} {
		c := c.BoundedReadClient(n)
		v, _, e := c.ReadAttachment(context.Background(), s.URL+"/asset", 5)
		if n == 5 && (e != nil || string(v) != "12345") {
			t.Fatal(v, e)
		}
		if n == 4 && e == nil {
			t.Fatal("aggregate excess accepted")
		}
		if _, _, e = c.ReadAttachment(context.Background(), s.URL+"/asset", 5); e == nil {
			t.Fatal("shared budget reset")
		}
	}
	if _, _, e := c.ReadAttachment(context.Background(), s.URL+"/asset", 4); e == nil {
		t.Fatal("per-resource excess accepted")
	}
}

func TestForgejoResponseUnicodeAndDuplicateRefusal(t *testing.T) {
	for _, v := range []string{`{"id":1,"id":2}`, `{"name":"\ud800"}`, `{"name":"\udc00"}`, strings.Repeat("[", 34) + "1" + strings.Repeat("]", 34)} {
		if StrictJSON([]byte(v)) {
			t.Fatal("ambiguous input accepted", v)
		}
	}
	if !StrictJSON([]byte(`{"name":"\ud83d\ude00","optional":null}`)) {
		t.Fatal("valid surrogate pair refused")
	}
}
