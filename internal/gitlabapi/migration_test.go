//go:build !noforge

package gitlabapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPinnedCISourceAndSelectedUpload(t *testing.T) {
	sha := strings.Repeat("a", 40)
	yaml := "include: other.yml\njob:\n  script: echo inert\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/repository/commits/"):
			fmt.Fprintf(w, `{"id":%q}`, sha)
		case strings.Contains(r.URL.Path, "/repository/files/"):
			if r.URL.Query().Get("ref") != sha {
				t.Error("unpinned read")
			}
			fmt.Fprintf(w, `{"file_path":".gitlab-ci.yml","commit_id":%q,"encoding":"base64","content":%q}`, sha, base64.StdEncoding.EncodeToString([]byte(yaml)))
		case strings.HasSuffix(r.URL.Path, "/uploads/17"):
			fmt.Fprint(w, "exact upload bytes")
		default:
			t.Error(r.URL)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c, _ := NewClient(srv.URL+"/api/v4", "explicit")
	v, e := c.ReadCI(context.Background(), "42", "main")
	if e != nil || v.Commit != sha || string(v.Content) != yaml {
		t.Fatal(v, e)
	}
	b, _, e := c.ReadAttachment(context.Background(), srv.URL+"/api/v4/projects/42/uploads/17", 18)
	if e != nil || string(b) != "exact upload bytes" {
		t.Fatal(string(b), e)
	}
	if _, _, e = c.ReadAttachment(context.Background(), "https://elsewhere.invalid/upload", 20); e == nil {
		t.Fatal("cross-origin upload accepted")
	}
}

func TestAmbiguousAndHostileProviderJSON(t *testing.T) {
	for _, raw := range []string{`{"id":1,"id":2}`, `{"x":"\ud800"}`, `{"x":"\udc00"}`, `{"x":[` + strings.Repeat("[", 40) + `0` + strings.Repeat("]", 40) + `]}`} {
		var v any
		if decode([]byte(raw), &v) == nil {
			t.Fatal("accepted ambiguous JSON", raw)
		}
	}
	var v any
	if e := decode([]byte(`{"nullable":null,"author":{"username":"\ud83d\ude00"}}`), &v); e != nil {
		t.Fatal(e)
	}
}
