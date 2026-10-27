//go:build !nogithub && !noforge

package githubapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProjectSelectors(t *testing.T) {
	for _, s := range []string{"user:alice/1", "org:team/42", "node:PVT_abc"} {
		if _, err := ParseProjectSelector(s, false); err != nil {
			t.Fatal(s, err)
		}
	}
	for _, s := range []string{"alice/repo", "user:alice/01", "org:team/0", "user:alice/1/2", "node:", "user:alice", "user:../1"} {
		if _, err := ParseProjectSelector(s, false); err == nil {
			t.Fatal("accepted", s)
		}
	}
	if _, err := ParseProjectSelector("org:team", true); err != nil {
		t.Fatal(err)
	}
}
func TestProjectGraphQLErrorsNeverPublishPartial(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": "secret"}}, "data": map[string]any{"user": map[string]any{"id": "U"}}})
	}))
	defer s.Close()
	c, err := NewClient(s.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.GetProjectOwner(context.Background(), "user:alice"); err == nil {
		t.Fatal("accepted partial")
	}
}
