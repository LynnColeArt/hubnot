//go:build !nogithub && !noforge

package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureIssue(id int) Issue {
	return Issue{ID: int64(id), NodeID: fmt.Sprintf("I_%d", id), Number: id, Title: "Ticket", State: "open", Body: "Description", User: User{Login: "alice"}}
}

func fixtureComment(id int) Comment {
	return Comment{ID: int64(id), NodeID: fmt.Sprintf("IC_%d", id), Body: "Comment", User: User{Login: "bob"}}
}

func newTestClient(t *testing.T, handler http.HandlerFunc, token string) (*Client, *httptest.Server) {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	c, err := NewClient(s.URL, token)
	if err != nil {
		t.Fatal(err)
	}
	return c, s
}

func TestListIssuesFollowsSameEndpointPaginationAndSkipsPRs(t *testing.T) {
	var requests atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/repos/alice/project/issues" || r.URL.Query().Get("state") != "all" || r.URL.Query().Get("per_page") != "100" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Errorf("missing API request headers")
		}
		switch r.URL.Query().Get("page") {
		case "1":
			w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?state=all&per_page=100&page=2>; rel="next", <http://%s%s?state=all&per_page=100&page=2>; rel="last"`, r.Host, r.URL.Path, r.Host, r.URL.Path))
			pr := fixtureIssue(2)
			pr.PullRequest = json.RawMessage(`{"url":"https://example.invalid/pr/2"}`)
			_ = json.NewEncoder(w).Encode([]Issue{fixtureIssue(1), pr})
		case "2":
			_ = json.NewEncoder(w).Encode([]Issue{fixtureIssue(3)})
		default:
			t.Errorf("unexpected page")
		}
	}, "test-token")
	issues, err := c.ListIssues(context.Background(), "alice/project")
	if err != nil || requests.Load() != 2 || len(issues) != 2 || issues[0].Number != 1 || issues[1].Number != 3 {
		t.Fatalf("issues=%v requests=%d err=%v", issues, requests.Load(), err)
	}
}

func TestPaginationNeverEscapesOrReturnsPartialData(t *testing.T) {
	for _, next := range []string{
		"https://other.invalid/repos/a/b/issues?state=all&per_page=100&page=2",
		"/repos/a/other/issues?state=all&per_page=100&page=2",
		"/repos/a/b/issues?state=all&per_page=100&page=1",
		"/repos/a/b/issues?state=open&per_page=100&page=2",
		"/repos/a/b/issues?state=all&per_page=100&page=2&access_token=secret",
	} {
		t.Run(next, func(t *testing.T) {
			var requests atomic.Int32
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Link", "<"+next+">; rel=\"next\"")
				_ = json.NewEncoder(w).Encode([]Issue{fixtureIssue(1)})
			}, "")
			issues, err := c.ListIssues(context.Background(), "a/b")
			if err == nil || issues != nil || requests.Load() != 1 || strings.Contains(err.Error(), "secret") {
				t.Fatalf("issues=%v requests=%d err=%v", issues, requests.Load(), err)
			}
		})
	}
}

func TestReadAndWriteIssueCommentContract(t *testing.T) {
	var calls atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/comments"):
			_ = json.NewEncoder(w).Encode([]Comment{fixtureComment(9)})
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/comments"):
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["body"] != "new comment" {
				t.Errorf("comment body=%v err=%v", body, err)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(fixtureComment(10))
		case r.Method == "POST":
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, found := body["state"]; found {
				t.Error("create must not send unsupported state")
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(fixtureIssue(1))
		case r.Method == "PATCH":
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			for _, field := range []string{"labels", "assignees"} {
				if string(body[field]) != "[]" {
					t.Errorf("%s must clear explicitly: %s", field, body[field])
				}
			}
			if string(body["state"]) != `"closed"` {
				t.Errorf("state=%s", body["state"])
			}
			v := fixtureIssue(1)
			v.State = "closed"
			_ = json.NewEncoder(w).Encode(v)
		default:
			_ = json.NewEncoder(w).Encode(fixtureIssue(1))
		}
	}, "write-token")
	ctx := context.Background()
	if _, err := c.GetIssue(ctx, "a/b", 1); err != nil {
		t.Fatal(err)
	}
	if rows, err := c.ListComments(ctx, "a/b", 1); err != nil || len(rows) != 1 || rows[0].ID != 9 {
		t.Fatalf("comments=%v err=%v", rows, err)
	}
	if _, err := c.CreateIssue(ctx, "a/b", IssueInput{Title: "Ticket", Body: "Description", State: "closed"}); err != nil {
		t.Fatal(err)
	}
	if issue, err := c.UpdateIssue(ctx, "a/b", 1, IssueInput{Title: "Ticket", State: "closed"}); err != nil || issue.State != "closed" {
		t.Fatalf("issue=%v err=%v", issue, err)
	}
	if _, err := c.CreateComment(ctx, "a/b", 1, "new comment"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 5 {
		t.Fatal(calls.Load())
	}
}

func TestAnonymousWritesAndInvalidSelectorsNeverRequest(t *testing.T) {
	var calls atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(fixtureIssue(1))
	}, "")
	ctx := context.Background()
	for _, repo := range []string{"a", "a/b/c", "../b", "a/..", "a/b?token=secret", "a/%2e%2e", "a/b\\c"} {
		if _, err := c.ListIssues(ctx, repo); err == nil {
			t.Errorf("accepted %q", repo)
		}
	}
	if _, err := c.GetIssue(ctx, "a/b", 0); err == nil {
		t.Error("accepted number zero")
	}
	if _, err := c.CreateIssue(ctx, "a/b", IssueInput{Title: "ticket"}); err == nil {
		t.Error("anonymous create accepted")
	}
	if _, err := c.UpdateIssue(ctx, "a/b", 1, IssueInput{Title: "ticket"}); err == nil {
		t.Error("anonymous update accepted")
	}
	if _, err := c.CreateComment(ctx, "a/b", 1, "comment"); err == nil {
		t.Error("anonymous comment accepted")
	}
	if calls.Load() != 0 {
		t.Fatalf("unexpected requests %d", calls.Load())
	}
	if _, err := c.GetIssue(ctx, "a/b", 1); err != nil {
		t.Fatal(err)
	}
}

func TestBaseURLAndTokenValidation(t *testing.T) {
	for _, base := range []string{"ftp://example.com", "http://example.com", "http://localhost.evil", "http://localhost", "https://user:secret@example.com", "https://example.com?token=secret", "https://example.com#secret", "https://example.com/%2e%2e", "https://example.com/a/../b", "https://example.com:bad", "https://example.com:", "https://"} {
		if _, err := NewClient(base, "secret"); err == nil || strings.Contains(err.Error(), "secret") {
			t.Errorf("base %q: %v", base, err)
		}
	}
	for _, base := range []string{"", "https://api.github.com", "https://enterprise.example/api/v3/", "http://127.0.0.1:1234", "http://[::1]:1234"} {
		if _, err := NewClient(base, ""); err != nil {
			t.Errorf("base %q: %v", base, err)
		}
	}
	if _, err := NewClient("", "secret\nheader"); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("invalid token: %v", err)
	}
}

func TestCanonicalBaseURLForStableSourceKeys(t *testing.T) {
	for _, base := range []string{"", "https://api.github.com", "https://API.GITHUB.COM/", "https://api.github.com///"} {
		c, err := NewClient(base, "")
		if err != nil {
			t.Fatal(err)
		}
		if c.BaseURL() != DefaultBaseURL {
			t.Fatalf("base %q canonicalized to %q", base, c.BaseURL())
		}
	}
	c, err := NewClient("https://ENTERPRISE.example/api/v3/", "")
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL() != "https://enterprise.example/api/v3" {
		t.Fatalf("enterprise base %q", c.BaseURL())
	}
}

func TestRedirectsErrorsAndFailuresDoNotLeakOrRetry(t *testing.T) {
	for _, status := range []int{302, 401, 403, 429, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var requests atomic.Int32
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Location", "https://secret.invalid/?token=secret-token")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"secret-token"}`))
			}, "secret-token")
			_, err := c.CreateIssue(context.Background(), "a/b", IssueInput{Title: "Ticket"})
			var apiErr *HTTPError
			if err == nil || !errors.As(err, &apiErr) || apiErr.StatusCode != status || strings.Contains(err.Error(), "secret") || requests.Load() != 1 {
				t.Fatalf("requests=%d err=%v", requests.Load(), err)
			}
		})
	}
	var requests atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		hj := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}, "secret-token")
	_, err := c.CreateIssue(context.Background(), "a/b", IssueInput{Title: "Ticket"})
	if err == nil || strings.Contains(err.Error(), "secret") || requests.Load() != 1 {
		t.Fatalf("requests=%d err=%v", requests.Load(), err)
	}
}

func TestResponseValidationAndSizeLimit(t *testing.T) {
	for _, body := range []string{"", "null", "{}", `{"id":1,"node_id":"I_1","number":2,"state":"open"}`, `{"id":1,"node_id":"I_1","number":1,"state":"open","body":"\ud800"}`, `{"id":1,"node_id":"I_1","number":1,"state":"open"} {}`, strings.Repeat(" ", 4<<20+1)} {
		t.Run(fmt.Sprintf("length-%d", len(body)), func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }, "")
			if _, err := c.GetIssue(context.Background(), "a/b", 1); err == nil {
				t.Error("accepted malformed/mismatched response")
			}
		})
	}
	for _, body := range []string{"null", "{}", "[] []", `[{"id":0}]`} {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }, "")
		if rows, err := c.ListIssues(context.Background(), "a/b"); err == nil || rows != nil {
			t.Fatalf("body=%q rows=%v err=%v", body, rows, err)
		}
	}
}

func TestCommentsPaginationAndEnterprisePrefix(t *testing.T) {
	var requests atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/api/v3/repos/a/b/issues/1/comments" {
			t.Errorf("enterprise prefix lost: %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `</api/v3/repos/a/b/issues/1/comments?per_page=100&page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]Comment{fixtureComment(1)})
			return
		}
		_ = json.NewEncoder(w).Encode([]Comment{fixtureComment(2)})
	}))
	defer s.Close()
	c, err := NewClient(s.URL+"/api/v3/", "")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := c.ListComments(context.Background(), "a/b", 1)
	if err != nil || len(rows) != 2 || requests.Load() != 2 {
		t.Fatalf("rows=%v requests=%d err=%v", rows, requests.Load(), err)
	}
}

func TestAggregateByteLimitReturnsNoPartialList(t *testing.T) {
	var requests atomic.Int32
	largeBody := strings.Repeat("x", 40<<10)
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		rows := make([]Issue, 100)
		for i := range rows {
			rows[i] = fixtureIssue((page-1)*100 + i + 1)
			rows[i].Body = largeBody
		}
		w.Header().Set("Link", fmt.Sprintf(`</repos/a/b/issues?state=all&per_page=100&page=%d>; rel="next"`, page+1))
		_ = json.NewEncoder(w).Encode(rows)
	}, "")
	rows, err := c.ListIssues(context.Background(), "a/b")
	if err == nil || rows != nil || requests.Load() != 17 || !strings.Contains(err.Error(), "aggregate") {
		t.Fatalf("requests=%d rows=%d err=%v", requests.Load(), len(rows), err)
	}
}

func TestDuplicateIdentityAndAmbiguousPaginationRefuse(t *testing.T) {
	for _, header := range []string{
		`</repos/a/b/issues?state=all&per_page=100&page=2>; rel="next", </repos/a/b/issues?state=all&per_page=100&page=2>; rel="next"`,
		`invalid-next-link`,
	} {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Link", header)
			_ = json.NewEncoder(w).Encode([]Issue{fixtureIssue(1)})
		}, "")
		if rows, err := c.ListIssues(context.Background(), "a/b"); err == nil || rows != nil {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
	}
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Comment{fixtureComment(1), fixtureComment(1)})
	}, "")
	if rows, err := c.ListComments(context.Background(), "a/b", 1); err == nil || rows != nil {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestPaginationAcceptsRelationshipWhitespaceAndCase(t *testing.T) {
	for _, relation := range []string{`rel = "next"`, `rel="NEXT"`, `REL="next"`} {
		t.Run(relation, func(t *testing.T) {
			var requests atomic.Int32
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Query().Get("page") == "1" {
					w.Header().Set("Link", `</repos/a/b/issues?state=all&per_page=100&page=2>; `+relation)
					_ = json.NewEncoder(w).Encode([]Issue{fixtureIssue(1)})
					return
				}
				_ = json.NewEncoder(w).Encode([]Issue{fixtureIssue(2)})
			}, "")
			rows, err := c.ListIssues(context.Background(), "a/b")
			if err != nil || requests.Load() != 2 || len(rows) != 2 {
				t.Fatalf("requests=%d rows=%v err=%v", requests.Load(), rows, err)
			}
		})
	}
}

func TestPageLimitAndCanceledRequests(t *testing.T) {
	var requests atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		w.Header().Set("Link", fmt.Sprintf(`</repos/a/b/issues?state=all&per_page=100&page=%d>; rel="next"`, page+1))
		_ = json.NewEncoder(w).Encode([]Issue{fixtureIssue(page)})
	}, "")
	if rows, err := c.ListIssues(context.Background(), "a/b"); err == nil || rows != nil || requests.Load() != 100 {
		t.Fatalf("requests=%d rows=%v err=%v", requests.Load(), rows, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := c.GetIssue(ctx, "a/b", 1); err == nil || time.Since(start) > time.Second {
		t.Fatalf("cancel err=%v elapsed=%v", err, time.Since(start))
	}
}
