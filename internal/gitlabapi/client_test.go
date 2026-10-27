//go:build !noforge

package gitlabapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpointAndTokenIsolation(t *testing.T) {
	for _, root := range []string{"https://gitlab.com", "http://example.com/api/v4", "https://x/api/v3", "https://user:secret@x/api/v4", "https://x/api/v4?x=1", "https://x/a/../api/v4", "https://x/api/v4#"} {
		if _, err := NewClient(root, "token"); err == nil {
			t.Fatalf("accepted %q", root)
		}
	}
	for _, project := range []string{"../x", "a//b", "a/%2fb", "a/b?x", "01"} {
		if ValidateProject(project) == nil {
			t.Fatalf("accepted project %q", project)
		}
	}
	var escaped string
	sinkCalls := 0
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sinkCalls++ }))
	defer sink.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		if r.Header.Get("PRIVATE-TOKEN") != "explicit" {
			t.Error("explicit token absent")
		}
		w.Header().Set("Location", sink.URL)
		w.WriteHeader(302)
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL+"/gitlab/api/v4", "explicit")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.GetProject(context.Background(), "group/nested/repo"); err == nil {
		t.Fatal("redirect accepted")
	}
	if escaped != "/gitlab/api/v4/projects/group%2Fnested%2Frepo" || sinkCalls != 0 {
		t.Fatal(escaped, sinkCalls)
	}
}

func TestIssueNotesPaginationAndWireBudget(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("X-Next-Page", "2")
			fmt.Fprint(w, `[{"id":900,"iid":7,"project_id":42,"title":"x","description":"original","state":"opened","issue_type":"issue","labels":[],"assignees":[]}]`)
		} else {
			fmt.Fprint(w, `[{"id":901,"iid":8,"project_id":42,"title":"y","description":"y","state":"closed","issue_type":"issue","labels":[],"assignees":[]}]`)
		}
	}))
	defer srv.Close()
	c, _ := NewClient(srv.URL+"/api/v4", "")
	rows, err := c.ListIssues(context.Background(), "42")
	if err != nil || len(rows) != 2 || rows[0].ID != 900 || rows[0].IID != 7 || calls != 2 {
		t.Fatal(rows, err, calls)
	}
	raw := `{"id":42,"path_with_namespace":"a/b","created_at":"2026-01-01T00:00:00Z"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw) }))
	defer server.Close()
	base, _ := NewClient(server.URL+"/api/v4", "")
	if _, err = base.BoundedReadClient(int64(len(raw))).GetProject(context.Background(), "42"); err != nil {
		t.Fatal("exact budget rejected", err)
	}
	if _, err = base.BoundedReadClient(int64(len(raw)-1)).GetProject(context.Background(), "42"); err == nil {
		t.Fatal("extra byte accepted")
	}
}

func TestOneMutationAndAPISpecificProjection(t *testing.T) {
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writes++
		if r.Method != "PUT" || !strings.HasSuffix(r.URL.Path, "/issues/7") {
			t.Error(r.Method, r.URL)
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["state_event"] != "close" || in["description"] != "body" || in["issue_type"] != "issue" {
			t.Error(in)
		}
		if _, ok := in["state"]; ok {
			t.Error("sent non-API state")
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c, _ := NewClient(srv.URL+"/api/v4", "token")
	_, err := c.UpdateIssue(context.Background(), "42", 7, IssueInput{Title: "title", Body: "body", State: "closed", Type: "issue", Labels: []string{}, Assignees: []string{}})
	if err == nil || writes != 1 {
		t.Fatal(err, writes)
	}
	if _, err = c.CreateIssue(context.Background(), "42", IssueInput{Title: "x", State: "closed", Type: "issue"}); err == nil || writes != 1 {
		t.Fatal("closed create mutated", err, writes)
	}
}

func TestPaginationRefusesCyclesForeignOriginsDuplicateIDsAndPageExhaustion(t *testing.T) {
	for _, mode := range []string{"cycle", "foreign", "duplicate", "pages"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch mode {
				case "cycle":
					w.Header().Set("X-Next-Page", "1")
				case "foreign":
					w.Header().Set("Link", `<https://foreign.invalid/api/v4/projects/42/issues?page=2&per_page=100&scope=all&state=all>; rel="next"`)
				case "duplicate":
					if calls == 1 {
						w.Header().Set("X-Next-Page", "2")
					}
				case "pages":
					w.Header().Set("X-Next-Page", fmt.Sprint(calls+1))
				}
				id := calls
				if mode == "duplicate" {
					id = 1
				}
				fmt.Fprintf(w, `[{"id":%d,"iid":%d,"project_id":42,"state":"opened"}]`, id, id)
			}))
			defer srv.Close()
			c, _ := NewClient(srv.URL+"/api/v4", "")
			if _, e := c.ListIssues(context.Background(), "42"); e == nil {
				t.Fatal("unsafe pagination accepted")
			}
			if mode == "pages" && calls != MaxPages || mode != "pages" && calls > 2 {
				t.Fatal("pagination was not bounded", calls)
			}
		})
	}
}
