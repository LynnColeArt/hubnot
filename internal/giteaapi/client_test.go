//go:build !noforge

package giteaapi

import (
	"context"
	"encoding/json"
	"hubnot/internal/forgejoapi"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGiteaSeparateVersionFamiliesAndGuardedUpdate(t *testing.T) {
	for _, version := range []string{"1.24", "1.27"} {
		t.Run(version, func(t *testing.T) {
			serverVersion := version + ".7"
			writes := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "token explicit" {
					t.Error("credential scope changed")
				}
				switch r.URL.Path {
				case "/api/v1/version":
					_ = json.NewEncoder(w).Encode(map[string]string{"version": serverVersion})
				case "/api/v1/repos/a/b/issues/7":
					writes++
					var in map[string]json.RawMessage
					_ = json.NewDecoder(r.Body).Decode(&in)
					if r.Method != "PATCH" || in["labels"] != nil || string(in["state"]) != `"closed"` {
						t.Error("wrong update", in)
					}
					if version == "1.27" && string(in["content_version"]) != "9" {
						t.Error("missing content guard")
					}
					_, _ = w.Write([]byte(`{"id":9001,"number":7,"title":"closed","state":"closed"}`))
				default:
					t.Error(r.URL)
					w.WriteHeader(404)
				}
			}))
			defer s.Close()
			c, e := NewVersionClient(s.URL+"/api/v1", "explicit", version)
			if e != nil || writes != 0 {
				t.Fatal(e)
			}
			if e = c.CheckVersion(context.Background()); e != nil {
				t.Fatal(e)
			}
			_, e = c.UpdateIssue(context.Background(), "a/b", 7, forgejoapi.IssueInput{Title: "closed", State: "closed", ContentVersion: 9})
			if e != nil || writes != 1 {
				t.Fatal(e, writes)
			}
			serverVersion = "16.0.0+gitea-1.22.0"
			if e = c.CheckVersion(context.Background()); e == nil {
				t.Fatal("Forgejo accepted as Gitea")
			}
		})
	}
	if _, e := NewVersionClient("https://example.com/api/v1", "", "1.26"); e == nil {
		t.Fatal("untested minor family accepted")
	}
	if _, e := NewClient("", ""); e == nil {
		t.Fatal("Gitea instance guessed")
	}
}
func TestGiteaWorkflowDirectoryIsDistinct(t *testing.T) {
	sha := strings.Repeat("a", 40)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ".forgejo") {
			t.Error("Forgejo path used by Gitea")
		}
		if strings.Contains(r.URL.Path, "/git/commits/") {
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": sha})
			return
		}
		if r.URL.Query().Get("ref") != sha {
			t.Error("mutable ref in file read")
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer s.Close()
	c, _ := NewClient(s.URL+"/api/v1", "")
	files, e := c.ReadWorkflowDefinitions(context.Background(), "a/b", "feature/workflow")
	if e != nil || len(files) != 0 {
		t.Fatal(files, e)
	}
}
