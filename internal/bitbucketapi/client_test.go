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

func TestOpaquePaginationAndCredentials(t *testing.T) {
	var paths []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer supplied" {
			t.Error("missing explicit bearer")
		}
		if r.URL.RawQuery == "cursor=opaque%2Bvalue" {
			fmt.Fprint(w, `{"values":[{"id":2}]}`)
			return
		}
		fmt.Fprintf(w, `{"values":[{"id":1}],"next":%q}`, srv.URL+"/2.0/repositories/ws/repo/pullrequests?cursor=opaque%2Bvalue")
	}))
	defer srv.Close()
	c, e := NewClient(srv.URL, "supplied", "api_token_bearer")
	if e != nil {
		t.Fatal(e)
	}
	rows, e := c.PullRequests(context.Background(), "ws/repo")
	if e != nil || len(rows) != 2 {
		t.Fatalf("rows %v: %v", rows, e)
	}
	if len(paths) != 2 || !strings.Contains(paths[0], "state=MERGED") || paths[1] != "/2.0/repositories/ws/repo/pullrequests?cursor=opaque%2Bvalue" {
		t.Fatal(paths)
	}
}

func TestHostilePaginationAndRedirects(t *testing.T) {
	for _, mode := range []string{"cross-origin", "other-connection", "loop", "redirect", "missing-values", "duplicate-field", "duplicate-record", "oversize", "page-budget"} {
		t.Run(mode, func(t *testing.T) {
			leaked := 0
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked++; fmt.Fprint(w, `{"values":[]}`) }))
			defer other.Close()
			var srv *httptest.Server
			requests := 0
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				switch mode {
				case "cross-origin":
					fmt.Fprintf(w, `{"values":[],"next":%q}`, other.URL+"/2.0/repositories/ws/repo/pullrequests")
				case "other-connection":
					fmt.Fprintf(w, `{"values":[],"next":%q}`, srv.URL+"/2.0/repositories/ws/other/pullrequests")
				case "loop":
					fmt.Fprintf(w, `{"values":[],"next":%q}`, srv.URL+r.URL.RequestURI())
				case "redirect":
					http.Redirect(w, r, other.URL, http.StatusFound)
				case "missing-values":
					fmt.Fprint(w, `{"size":0}`)
				case "duplicate-field":
					fmt.Fprint(w, `{"values":[],"values":[]}`)
				case "duplicate-record":
					fmt.Fprint(w, `{"values":[{"id":1},{"id":1}]}`)
				case "oversize":
					fmt.Fprint(w, strings.Repeat(" ", int(MaxResponseBytes)+1))
				case "page-budget":
					fmt.Fprintf(w, `{"values":[],"next":%q}`, srv.URL+"/2.0/repositories/ws/repo/pullrequests?cursor="+fmt.Sprint(requests))
				}
			}))
			defer srv.Close()
			c, e := NewClient(srv.URL, "private", "oauth2")
			if e != nil {
				t.Fatal(e)
			}
			if _, e = c.PullRequests(context.Background(), "ws/repo"); e == nil {
				t.Fatal("accepted hostile connection")
			}
			if leaked != 0 {
				t.Fatal("forwarded request/credentials")
			}
			if mode == "page-budget" && requests != MaxPages {
				t.Fatalf("page budget requests = %d, want %d", requests, MaxPages)
			}
		})
	}
}

func TestCloudProfilesAndBasic(t *testing.T) {
	for _, endpoint := range []string{"https://bitbucket.example/rest/api/1.0", "https://api.bitbucket.org/1.0", "http://api.bitbucket.org/2.0", "https://api.bitbucket.org/2.0?token=bad"} {
		if _, e := NewClient(endpoint, "", "anonymous"); e == nil {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	for _, mode := range []string{"", "app_password", "bearer"} {
		if _, e := NewClient("", "secret", mode); e == nil {
			t.Fatalf("accepted ambiguous/retired mode %q", mode)
		}
	}
	for _, secret := range []string{"token", "bad:token", "a@b:", "a@b:tok\nen"} {
		if _, e := NewClient("", secret, "api_token_basic"); e == nil {
			t.Fatal("invalid basic credential")
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "a@example.com" || p != "token" {
			t.Error("wrong basic auth")
		}
		fmt.Fprint(w, `{"uuid":"{11111111-1111-1111-1111-111111111111}","full_name":"ws/repo","created_on":"2026-01-01T00:00:00Z"}`)
	}))
	defer srv.Close()
	c, e := NewClient(srv.URL, "a@example.com:token", "api_token_basic")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Repository(context.Background(), "ws/repo"); e != nil {
		t.Fatal(e)
	}
}
