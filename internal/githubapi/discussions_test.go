//go:build !nogithub && !noforge

package githubapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discussionFixture(id string) map[string]any {
	return map[string]any{"__typename": "Discussion", "id": id, "number": 7, "title": "Question", "body": "body", "url": "https://github.com/o/r/discussions/7", "author": nil, "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-02T00:00:00Z", "repository": map[string]any{"id": "R1"}, "category": categoryFixture(), "closed": false, "locked": false, "answer": nil, "poll": nil, "reactions": map[string]any{"totalCount": 2}, "upvoteCount": 1, "userContentEdits": nil}
}
func categoryFixture() map[string]any {
	return map[string]any{"id": "C1", "name": "Help", "slug": "help", "description": "questions", "emoji": "?", "isAnswerable": true}
}
func commentFixture(id, parent string) map[string]any {
	var p any
	if parent != "" {
		p = map[string]any{"id": parent}
	}
	return map[string]any{"__typename": "DiscussionComment", "id": id, "discussion": map[string]any{"id": "D1"}, "replyTo": p, "body": "comment " + id, "author": map[string]any{"login": "alice"}, "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-02T00:00:00Z", "url": "https://github.com/o/r/discussions/7#" + id, "isAnswer": false, "isMinimized": false, "upvoteCount": 0, "reactions": map[string]any{"totalCount": 0}, "userContentEdits": map[string]any{"totalCount": 0}}
}
func connection(nodes []any, next bool, cursor any) map[string]any {
	return map[string]any{"nodes": nodes, "pageInfo": map[string]any{"hasNextPage": next, "endCursor": cursor}}
}
func TestDiscussionCompleteIndependentPagination(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var q struct {
			Query     string
			Variables map[string]any
		}
		json.NewDecoder(r.Body).Decode(&q)
		var data any
		switch {
		case strings.Contains(q.Query, "HNDiscussionNumber"):
			data = map[string]any{"repository": map[string]any{"discussion": discussionFixture("D1")}}
		case strings.Contains(q.Query, "HNDiscussionComments"):
			cur := q.Variables["after"]
			nodes := []any{commentFixture("A", "")}
			next := true
			var end any = "top1"
			if cur == "top1" {
				nodes = []any{commentFixture("B", "")}
				next = false
				end = nil
			}
			data = map[string]any{"node": map[string]any{"__typename": "Discussion", "id": "D1", "comments": connection(nodes, next, end)}}
		case strings.Contains(q.Query, "HNDiscussionReplies"):
			id := q.Variables["id"].(string)
			nodes := []any{}
			next := false
			var end any
			if id == "A" {
				nodes = []any{commentFixture("A1", "A")}
				if q.Variables["after"] == nil {
					next = true
					end = "reply1"
				} else {
					nodes = []any{commentFixture("A2", "A")}
				}
			}
			data = map[string]any{"node": map[string]any{"__typename": "DiscussionComment", "id": id, "replies": connection(nodes, next, end)}}
		default:
			t.Errorf("unexpected query %s", q.Query)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer s.Close()
	c, _ := NewClient(s.URL, "token")
	d, err := c.GetDiscussion(context.Background(), "o/r", 7)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "D1" || d.Author != "" || len(d.Comments) != 4 || d.Comments[2].ParentID != "A" || calls != 6 {
		t.Fatalf("thread %+v calls %d", d, calls)
	}
}
func TestDiscussionRejectsCursorCyclesAndWrongIdentity(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		t.Run(map[bool]string{false: "cycle", true: "identity"}[wrong], func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var q struct{ Query string }
				json.NewDecoder(r.Body).Decode(&q)
				var data any
				if strings.Contains(q.Query, "HNDiscussionNumber") {
					data = map[string]any{"repository": map[string]any{"discussion": discussionFixture("D1")}}
				} else {
					id := "D1"
					if wrong {
						id = "OTHER"
					}
					data = map[string]any{"node": map[string]any{"__typename": "Discussion", "id": id, "comments": connection([]any{}, true, "same")}}
				}
				json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer s.Close()
			c, _ := NewClient(s.URL, "token")
			if d, err := c.GetDiscussion(context.Background(), "o/r", 7); err == nil || d.ID != "" {
				t.Fatalf("partial accepted %+v %v", d, err)
			}
		})
	}
}

func TestDiscussionRepositoryCategoriesAndListing(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Query     string
			Variables map[string]any
		}
		json.NewDecoder(r.Body).Decode(&q)
		if q.Variables["owner"] != "o" || q.Variables["name"] != "r" {
			t.Error("invalid selector")
		}
		repo := map[string]any{"id": "R1"}
		switch {
		case strings.Contains(q.Query, "HNDiscussionRepository"):
			repo["nameWithOwner"] = "o/r"
			repo["hasDiscussionsEnabled"] = false
		case strings.Contains(q.Query, "HNDiscussionCategories"):
			cat := categoryFixture()
			next := q.Variables["after"] == nil
			var cursor any = "C-page"
			if !next {
				cat["id"] = "C2"
				cat["name"] = "General"
				cat["slug"] = "general"
				cursor = nil
			}
			repo["discussionCategories"] = connection([]any{cat}, next, cursor)
		case strings.Contains(q.Query, "HNDiscussions"):
			d := discussionFixture("D1")
			d["poll"] = map[string]any{"id": "P1"}
			d["locked"] = true
			d["activeLockReason"] = "SPAM"
			d["userContentEdits"] = map[string]any{"totalCount": 3}
			repo["discussions"] = connection([]any{d}, false, nil)
		default:
			t.Error(q.Query)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": repo}})
	}))
	defer s.Close()
	c, _ := NewClient(s.URL, "token")
	ctx := context.Background()
	repo, err := c.GetDiscussionRepository(ctx, "o/r")
	if err != nil || repo.ID != "R1" || repo.HasDiscussionsEnabled {
		t.Fatalf("%+v %v", repo, err)
	}
	cats, err := c.ListDiscussionCategories(ctx, "o/r")
	if err != nil || len(cats) != 2 || cats[1].ID != "C2" {
		t.Fatalf("%+v %v", cats, err)
	}
	rows, err := c.ListDiscussions(ctx, "o/r")
	if err != nil || len(rows) != 1 || rows[0].PollID != "P1" || !rows[0].Locked || *rows[0].EditCount != 3 || rows[0].Comments != nil {
		t.Fatalf("%+v %v", rows, err)
	}
}
func TestDiscussionRequiredFieldsAndInaccessibleNodes(t *testing.T) {
	for _, mode := range []string{"null", "wrong-type", "missing-body", "missing-category", "unknown-state", "bad-date", "missing-boolean", "invalid-answer", "missing-reactions"} {
		t.Run(mode, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				d := discussionFixture("D1")
				var n any = d
				switch mode {
				case "null":
					n = nil
				case "wrong-type":
					d["__typename"] = "Issue"
				case "missing-body":
					delete(d, "body")
				case "missing-category":
					d["category"] = nil
				case "unknown-state":
					d["stateReason"] = "UNKNOWN"
				case "bad-date":
					d["createdAt"] = "bad"
				case "missing-boolean":
					delete(d, "closed")
				case "invalid-answer":
					d["answer"] = map[string]any{"id": ""}
				case "missing-reactions":
					delete(d, "reactions")
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"node": n}})
			}))
			defer s.Close()
			c, _ := NewClient(s.URL, "token")
			if d, err := c.GetDiscussionByID(context.Background(), "D1"); err == nil || d.ID != "" {
				t.Fatalf("partial %+v %v", d, err)
			}
		})
	}
}

func TestDiscussionMutationsSingleValidatedOperation(t *testing.T) {
	type invoke func(*Client) (string, error)
	ctx := context.Background()
	input := DiscussionInput{Title: "Question", Body: "body", CategoryID: "C1"}
	cases := []struct {
		field, query string
		call         invoke
		comment      bool
		configure    func(map[string]any)
	}{
		{"createDiscussion", "HNCreateDiscussion", func(c *Client) (string, error) { v, e := c.CreateDiscussion(ctx, "R1", input); return v.ID, e }, false, nil},
		{"updateDiscussion", "HNUpdateDiscussion", func(c *Client) (string, error) { v, e := c.UpdateDiscussion(ctx, "D1", input); return v.ID, e }, false, nil},
		{"addDiscussionComment", "HNAddDiscussionComment", func(c *Client) (string, error) {
			v, e := c.AddDiscussionComment(ctx, "D1", "A", "comment X")
			return v.ID, e
		}, true, nil},
		{"updateDiscussionComment", "HNUpdateDiscussionComment", func(c *Client) (string, error) {
			v, e := c.UpdateDiscussionComment(ctx, "X", "comment X")
			return v.ID, e
		}, true, nil},
		{"markDiscussionCommentAsAnswer", "HNMarkDiscussionAnswer", func(c *Client) (string, error) {
			v, e := c.MarkDiscussionCommentAsAnswer(ctx, "D1", "X")
			return v.ID, e
		}, false, func(d map[string]any) { d["answer"] = map[string]any{"id": "X"} }},
		{"unmarkDiscussionCommentAsAnswer", "HNUnmarkDiscussionAnswer", func(c *Client) (string, error) {
			v, e := c.UnmarkDiscussionCommentAsAnswer(ctx, "D1", "X")
			return v.ID, e
		}, false, nil},
		{"closeDiscussion", "HNCloseDiscussion", func(c *Client) (string, error) { v, e := c.CloseDiscussion(ctx, "D1", "OUTDATED"); return v.ID, e }, false, func(d map[string]any) { d["closed"] = true; d["stateReason"] = "OUTDATED" }},
		{"reopenDiscussion", "HNReopenDiscussion", func(c *Client) (string, error) { v, e := c.ReopenDiscussion(ctx, "D1"); return v.ID, e }, false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			for _, bad := range []bool{false, true} {
				calls := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var q struct {
						Query     string
						Variables map[string]map[string]any
					}
					json.NewDecoder(r.Body).Decode(&q)
					if !strings.Contains(q.Query, tc.query) || !strings.HasPrefix(q.Query, "mutation ") || q.Variables["input"] == nil {
						t.Errorf("bad mutation %+v", q)
					}
					d := discussionFixture("D1")
					if tc.configure != nil {
						tc.configure(d)
					}
					payload := map[string]any{"discussion": d}
					if tc.comment {
						parent := ""
						if tc.field == "addDiscussionComment" {
							parent = "A"
						}
						payload = map[string]any{"comment": commentFixture("X", parent)}
					}
					if bad {
						if tc.comment {
							payload["comment"].(map[string]any)["id"] = "WRONG"
							payload["comment"].(map[string]any)["body"] = "wrong"
						} else {
							d["id"] = "WRONG"
							d["repository"] = map[string]any{"id": "WRONG"}
						}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{tc.field: payload}})
				}))
				c, _ := NewClient(s.URL, "token")
				id, err := tc.call(c)
				s.Close()
				if calls != 1 || (bad && (err == nil || id != "")) || (!bad && (err != nil || id == "")) {
					t.Fatalf("bad=%v id=%s err=%v calls=%d", bad, id, err, calls)
				}
			}
		})
	}
}
func TestDiscussionMutationErrorNeverRetries(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"data":{"createDiscussion":null},"errors":[{"message":"private-token"}]}`))
	}))
	defer s.Close()
	c, _ := NewClient(s.URL, "private-token")
	_, err := c.CreateDiscussion(context.Background(), "R1", DiscussionInput{Title: "Question", Body: "body", CategoryID: "C1"})
	if err == nil || strings.Contains(err.Error(), "private-token") || calls != 1 {
		t.Fatalf("%v calls=%d", err, calls)
	}
}
func TestDiscussionInvalidSelectorsNeverRequest(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; t.Error("invalid input reached provider") }))
	defer s.Close()
	c, _ := NewClient(s.URL, "token")
	ctx := context.Background()
	checks := []func() error{
		func() error { _, e := c.GetDiscussion(ctx, "../x", 1); return e }, func() error { _, e := c.GetDiscussion(ctx, "o/r", 0); return e }, func() error { _, e := c.GetDiscussionByID(ctx, "bad id"); return e },
		func() error { _, e := c.CreateDiscussion(ctx, "", DiscussionInput{}); return e }, func() error { _, e := c.CloseDiscussion(ctx, "D1", "WRONG"); return e }, func() error { _, e := c.AddDiscussionComment(ctx, "D1", "", " "); return e },
	}
	for _, check := range checks {
		if check() == nil {
			t.Error("invalid selector accepted")
		}
	}
	if calls != 0 {
		t.Fatal(calls)
	}
}

func TestDiscussionRefusesDecoderEquivalentTypedKeys(t *testing.T) {
	for _, fields := range []string{
		`"body":"body","BODY":"manufactured"`,
		`"id":"other","ID":"D1"`,
		`"answer":{"id":"other","ID":"A"}`,
		`"answer":{"id":"other"},"ANSWER":{"id":"A"}`,
		`"locked":false,"locKed":true`,
	} {
		t.Run(fields, func(t *testing.T) {
			fixture := discussionFixture("D1")
			first, _, _ := strings.Cut(fields, ":")
			delete(fixture, strings.Trim(first, `"`))
			node, _ := json.Marshal(fixture)
			raw := string(node)
			raw = raw[:len(raw)-1] + "," + fields + "}"
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var q struct{ Query string }
				json.NewDecoder(r.Body).Decode(&q)
				if strings.Contains(q.Query, "HNDiscussionID") {
					w.Write([]byte(`{"data":{"node":` + raw + `}}`))
					return
				}
				var data any
				if strings.Contains(q.Query, "HNDiscussionComments") {
					nodes := []any{}
					if strings.HasPrefix(fields, `"answer"`) {
						comment := commentFixture("A", "")
						comment["isAnswer"] = true
						nodes = append(nodes, comment)
					}
					data = map[string]any{"node": map[string]any{"__typename": "Discussion", "id": "D1", "comments": connection(nodes, false, nil)}}
				} else {
					data = map[string]any{"node": map[string]any{"__typename": "DiscussionComment", "id": "A", "replies": connection([]any{}, false, nil)}}
				}
				json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer s.Close()
			c, _ := NewClient(s.URL, "token")
			if d, err := c.GetDiscussionByID(context.Background(), "D1"); err == nil || d.ID != "" {
				t.Fatal("ambiguous typed discussion accepted")
			}
		})
	}
}

func TestDiscussionCommentRefusesDecoderEquivalentParentKeys(t *testing.T) {
	comment, _ := json.Marshal(commentFixture("A", "P"))
	raw := string(comment)
	raw = raw[:len(raw)-1] + `,"REPLYTO":{"id":"other"}}`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"updateDiscussionComment":{"comment":` + raw + `}}}`))
	}))
	defer s.Close()
	c, _ := NewClient(s.URL, "token")
	if v, err := c.UpdateDiscussionComment(context.Background(), "A", "comment A"); err == nil || v.ID != "" {
		t.Fatal("ambiguous parent accepted")
	}
}
