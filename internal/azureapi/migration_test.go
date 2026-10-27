//go:build !noforge

package azureapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// WIQL fixtures use real documented flat/asOf result and 200-ID detail list
// contracts. Full-page, exact-budget and failure cases exercise actual requests.
func TestWIQLExhaustsFullPageAndRefusesIncompleteDetail(t *testing.T) {
	for _, mode := range []string{"complete", "exact", "over", "missing", "cycle", "truncated", "time"} {
		t.Run(mode, func(t *testing.T) {
			queryCalls := 0
			detailCalls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("api-version") != "7.1" {
					t.Error("unversioned request")
				}
				if strings.HasSuffix(r.URL.Path, "/wiql") {
					queryCalls++
					var q map[string]string
					_ = json.NewDecoder(r.Body).Decode(&q)
					if r.URL.Query().Get("$top") != "200" {
						t.Error("missing explicit page limit")
					}
					first := strings.Contains(q["query"], "[System.Id] > 0 ")
					if !first && !strings.Contains(q["query"], "ASOF '2026-01-01T00:00:00Z'") {
						t.Error("query not pinned")
					}
					rows := []map[string]int{}
					if first || mode == "cycle" {
						for i := 1; i <= 200; i++ {
							rows = append(rows, map[string]int{"id": i})
						}
					} else if mode != "exact" {
						rows = append(rows, map[string]int{"id": 201})
					}
					asOf := "2026-01-01T00:00:00Z"
					if !first && mode == "time" {
						asOf = "2026-01-02T00:00:00Z"
					}
					json.NewEncoder(w).Encode(map[string]any{"queryType": "flat", "asOf": asOf, "workItems": rows, "isTruncated": mode == "truncated"})
				} else {
					detailCalls++
					ids := strings.Split(r.URL.Query().Get("ids"), ",")
					if len(ids) > 200 || r.URL.Query().Get("errorPolicy") != "Fail" || r.URL.Query().Get("asOf") == "" {
						t.Error("unbounded/unpinned detail batch")
					}
					rows := []map[string]any{}
					for _, id := range ids {
						n, _ := strconv.Atoi(id)
						rows = append(rows, map[string]any{"id": n, "rev": 4, "fields": map[string]any{"System.Title": "source", "Custom.Field": "original"}})
					}
					if mode == "missing" {
						rows = rows[:len(rows)-1]
					}
					json.NewEncoder(w).Encode(map[string]any{"count": len(rows), "value": rows})
				}
			}))
			defer s.Close()
			c, _ := NewClient(s.URL, "", "bearer", "services-7.1")
			limit := 201
			if mode == "exact" || mode == "over" {
				limit = 200
			}
			rows, e := c.ListWorkItems(context.Background(), "org", "project", limit)
			if mode == "complete" || mode == "exact" {
				if e != nil || len(rows) != limit || queryCalls != 2 {
					t.Fatal("incomplete enumeration", len(rows), queryCalls, e)
				}
				if detailCalls < 1 || !strings.Contains(string(rows[0].Raw), "Custom.Field") {
					t.Fatal("detail content missing")
				}
			} else if e == nil {
				t.Fatal("incomplete or excessive result accepted", mode)
			}
		})
	}
}
func TestCommentsContinuationAndHonestCompleteness(t *testing.T) {
	for _, mode := range []string{"complete", "exact", "over", "cycle", "missing", "foreign_parent", "duplicate", "conflicting"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				token := r.URL.Query().Get("continuationToken")
				if calls > 1 && token != "opaque/+=cursor" {
					t.Error("cursor altered", token)
				}
				id := 1
				parent := 9
				if calls > 1 {
					id = 2
				}
				if mode == "foreign_parent" {
					parent = 10
				}
				if mode == "duplicate" {
					id = 1
				}
				next := ""
				if calls == 1 || mode == "cycle" {
					next = "opaque/+=cursor"
				}
				if mode == "conflicting" {
					w.Header().Set("x-ms-continuationtoken", "other")
				}
				total := 2
				if mode == "missing" {
					total = 3
				}
				json.NewEncoder(w).Encode(map[string]any{"count": 1, "totalCount": total, "continuationToken": next, "comments": []any{map[string]any{"commentId": id, "version": 3, "workItemId": parent, "text": fmt.Sprintf("comment %d", id)}}})
			}))
			defer s.Close()
			c, _ := NewClient(s.URL, "", "bearer", "services-7.1")
			limit := 2
			if mode == "over" {
				limit = 1
			}
			rows, e := c.ListComments(context.Background(), "org", "project", 9, limit)
			if mode == "complete" || mode == "exact" {
				if e != nil || len(rows) != 2 || calls != 2 {
					t.Fatal(rows, e, calls)
				}
			} else if e == nil {
				t.Fatal("incomplete/misbound enumeration accepted", mode)
			}
		})
	}
}
func TestWorkItemWriteUsesRevisionTestAndOneMutation(t *testing.T) {
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writes++
		if r.Method != "PATCH" || r.Header.Get("Content-Type") != "application/json-patch+json" {
			t.Error("wrong operation")
		}
		if r.URL.Query().Get("bypassRules") != "" {
			t.Error("bypassed rules")
		}
		var ops []Patch
		_ = json.NewDecoder(r.Body).Decode(&ops)
		if len(ops) != 5 || ops[0].Op != "test" || ops[0].Path != "/rev" || ops[0].Value != float64(7) || ops[1].Op != "test" || ops[1].Path != "/fields/System.WorkItemType" {
			t.Error("missing baseline guard", ops)
		}
		w.WriteHeader(409)
	}))
	defer s.Close()
	c, _ := NewClient(s.URL, "", "bearer", "services-7.1")
	_, e := c.WriteWorkItem(context.Background(), "org", "project", 19, 7, WorkItemInput{Title: "mapped", Body: "description", Type: "Task", State: "Active"})
	if e == nil || writes != 1 {
		t.Fatal("conflict retried/ignored", writes, e)
	}
}
