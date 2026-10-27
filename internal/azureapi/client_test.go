//go:build !noforge

package azureapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Version-qualified fixtures follow Microsoft Learn WIT 7.1/7.0, Comments
// 7.1-preview.4/7.0-preview.3, WIQL and Git threads REST contracts. These are
// deliberately local API boundaries, with no SDK, CLI or account discovery.
func TestExplicitVersionsCredentialsAndOfflineConstruction(t *testing.T) {
	for _, profile := range []struct{ version, api, comments, mode string }{{"services-7.1", "7.1", "7.1-preview.4", "bearer"}, {"server-7.0", "7.0", "7.0-preview.3", "pat"}} {
		t.Run(profile.version, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				auth := "Bearer secret"
				if profile.mode == "pat" {
					auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
				}
				if r.Header.Get("Authorization") != auth {
					t.Error("supplied credentials missing")
				}
				if strings.Contains(r.URL.Path, "comments") {
					if r.URL.Query().Get("api-version") != profile.comments {
						t.Error(r.URL)
					}
					json.NewEncoder(w).Encode(map[string]any{"count": 0, "totalCount": 0, "comments": []any{}})
				} else {
					if r.URL.Query().Get("api-version") != profile.api {
						t.Error(r.URL)
					}
					json.NewEncoder(w).Encode(map[string]any{"id": "11111111-1111-4111-8111-111111111111", "name": "P", "capabilities": map[string]any{"processTemplate": map[string]string{"templateTypeId": "process"}}})
				}
			}))
			defer s.Close()
			c, e := NewClient(s.URL, "secret", profile.mode, profile.version)
			if e != nil {
				t.Fatal(e)
			}
			if calls != 0 {
				t.Fatal("constructor performed I/O")
			}
			if _, e = c.GetProject(context.Background(), "org", "P"); e != nil {
				t.Fatal(e)
			}
			if _, e = c.ListComments(context.Background(), "org", "P", 1, 10); e != nil {
				t.Fatal(e)
			}
		})
	}
	for _, v := range []string{"", "7.1", "services-8.0", "server-6.0"} {
		if _, e := NewClient("https://dev.azure.com", "", "bearer", v); e == nil {
			t.Fatal("unqualified version accepted", v)
		}
	}
	for _, u := range []string{"http://example.com", "https://user:pass@host", "https://host/?x=1", "https://host/a/../b", "https://host/%2f", "https://host/#x"} {
		if _, e := NewClient(u, "", "bearer", "services-7.1"); e == nil {
			t.Fatal("unsafe endpoint", u)
		}
	}
}
func TestRedirectAndForeignBlobNeverForwardCredentials(t *testing.T) {
	foreign := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreign++ }))
	defer other.Close()
	own := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) }))
	defer own.Close()
	c, _ := NewClient(own.URL, "secret", "bearer", "services-7.1")
	if _, _, e := c.Download(context.Background(), other.URL+"/file", 100); e == nil {
		t.Fatal("foreign blob allowed")
	}
	if _, _, e := c.Download(context.Background(), own.URL+"/file", 100); e == nil {
		t.Fatal("redirect followed")
	}
	if foreign != 0 {
		t.Fatal("foreign origin received request")
	}
}
func TestSharedWireExactBudgetAndExcess(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("1234")) }))
	defer s.Close()
	c, _ := NewClient(s.URL, "", "bearer", "services-7.1")
	c = c.BoundedReadClient(8)
	for i := 0; i < 2; i++ {
		b, _, e := c.Download(context.Background(), s.URL+"/file", 4)
		if e != nil || string(b) != "1234" {
			t.Fatal(e, string(b))
		}
	}
	if _, _, e := c.Download(context.Background(), s.URL+"/file", 4); e == nil {
		t.Fatal("aggregate excess allowed")
	}
	c = c.BoundedReadClient(100)
	if _, _, e := c.Download(context.Background(), s.URL+"/file", 3); e == nil {
		t.Fatal("per blob excess allowed")
	}
}
