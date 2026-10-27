//go:build !nogithub && !noforge

package githubapi

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func workflowBlob(content []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(content))), content...)))
}
func TestWorkflowPinnedRegularFileAndExactBytes(t *testing.T) {
	content := []byte("name: CI\r\non: [push]\n# café\n")
	commit := strings.Repeat("a", 40)
	tree := strings.Repeat("b", 40)
	blob := workflowBlob(content)
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/a/b/commits":
			if r.URL.Query().Get("sha") != "branch/with/slash" {
				t.Fatal(r.URL)
			}
			json.NewEncoder(w).Encode([]any{map[string]any{"sha": commit, "commit": map[string]any{"tree": map[string]any{"sha": tree}}}})
		case "/repos/a/b/git/trees/" + tree:
			json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": ".github", "mode": "040000", "type": "tree", "sha": strings.Repeat("c", 40)}}})
		case "/repos/a/b/git/trees/" + strings.Repeat("c", 40):
			json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": "workflows", "mode": "040000", "type": "tree", "sha": strings.Repeat("d", 40)}}})
		case "/repos/a/b/git/trees/" + strings.Repeat("d", 40):
			json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": "ci.yml", "mode": "100644", "type": "blob", "sha": blob}}})
		case "/repos/a/b/contents/.github/workflows/ci.yml":
			if r.URL.Query().Get("ref") != commit {
				t.Fatal("not pinned", r.URL)
			}
			json.NewEncoder(w).Encode(map[string]any{"type": "file", "path": ".github/workflows/ci.yml", "sha": blob, "size": len(content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(content), "download_url": "https://evil.invalid/token"})
		default:
			t.Fatal(r.URL)
		}
	}, "")
	pin, err := c.ResolveWorkflowCommit(context.Background(), "a/b", "branch/with/slash")
	if err != nil {
		t.Fatal(err)
	}
	file, err := c.GetWorkflowFile(context.Background(), "a/b", ".github/workflows/ci.yml", pin)
	if err != nil || string(file.Content) != string(content) || file.SHA != blob || file.Commit != commit {
		t.Fatal(file, err)
	}
}
func TestWorkflowSymlinkAndUnsafePathRefused(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": ".github", "mode": "120000", "type": "blob", "sha": strings.Repeat("b", 40)}}})
	}, "token")
	for _, path := range []string{"../evil.yml", ".github/workflows/../x.yml", ".github/workflows/x.json", ".github/workflows/a/b.yml"} {
		if _, err := c.GetWorkflowFile(context.Background(), "a/b", path, WorkflowCommit{SHA: strings.Repeat("a", 40), TreeSHA: strings.Repeat("b", 40)}); err == nil {
			t.Fatal("accepted", path)
		}
	}
	if calls != 0 {
		t.Fatal(calls)
	}
	if _, err := c.GetWorkflowFile(context.Background(), "a/b", ".github/workflows/ci.yml", WorkflowCommit{SHA: strings.Repeat("a", 40), TreeSHA: strings.Repeat("b", 40)}); err == nil {
		t.Fatal("symlink accepted")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestWorkflowPUTCASAndNoRetry(t *testing.T) {
	content := []byte("on: workflow_dispatch\n")
	blob := workflowBlob(content)
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PUT" || r.URL.Path != "/repos/a/b/contents/.github/workflows/ci.yml" {
			t.Fatal(r.Method, r.URL)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["branch"] != "feature/workflow" || body["sha"] != strings.Repeat("a", 40) || body["content"] != base64.StdEncoding.EncodeToString(content) {
			t.Fatal(body)
		}
		json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"path": ".github/workflows/ci.yml", "sha": blob}, "commit": map[string]any{"sha": strings.Repeat("c", 40)}})
	}, "token")
	got, err := c.PutWorkflowFile(context.Background(), "a/b", WorkflowFileInput{Path: ".github/workflows/ci.yml", Branch: "feature/workflow", ExpectedSHA: strings.Repeat("a", 40), Content: content, Message: "Export explicit native workflow"})
	if err != nil || got.SHA != blob || calls != 1 {
		t.Fatal(got, err, calls)
	}
	c2, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(409) }, "token")
	_, err = c2.PutWorkflowFile(context.Background(), "a/b", WorkflowFileInput{Path: ".github/workflows/ci.yml", Branch: "main", ExpectedSHA: strings.Repeat("a", 40), Content: content, Message: "x"})
	if err == nil || calls != 2 {
		t.Fatal(err, calls)
	}
}

func TestWorkflowDiscoveryPaginationAndHostileJSON(t *testing.T) {
	for _, payload := range []string{
		`{"total_count":0,"workflows":[],"Workflows":[{"id":2}]}`,
		`{"total_count":0,"workflows":[],"workflows":[]}`,
		`{"total_count":1,"workflows":[]}`,
		`{"total_count":257,"workflows":[]}`,
		`{"total_count":0,"workflows":[]} {}`,
	} {
		t.Run(payload, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, payload) }, "")
			if rows, err := c.ListWorkflows(context.Background(), "a/b"); err == nil || rows != nil {
				t.Fatal(rows, err)
			}
		})
	}
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := r.URL.Query().Get("page")
		id := 1
		if page == "1" {
			w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?per_page=100&page=2>; rel="next"`, r.Host, r.URL.Path))
		} else {
			id = 2
		}
		json.NewEncoder(w).Encode(map[string]any{"total_count": 2, "workflows": []any{map[string]any{"id": id, "node_id": fmt.Sprint("W", id), "name": "CI", "path": fmt.Sprintf(".github/workflows/ci%d.yml", id)}}})
	}, "")
	rows, err := c.ListWorkflows(context.Background(), "a/b")
	if err != nil || len(rows) != 2 || calls != 2 {
		t.Fatal(rows, err, calls)
	}
}
func TestWorkflowPUTCreationAndResultValidation(t *testing.T) {
	content := []byte("on: workflow_dispatch\n")
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		if _, ok := in["sha"]; ok {
			t.Fatal("create sent sha")
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"path": ".github/workflows/new.yml", "sha": workflowBlob(content)}, "commit": map[string]any{"sha": strings.Repeat("d", 40)}})
	}, "token")
	if _, err := c.PutWorkflowFile(context.Background(), "a/b", WorkflowFileInput{Path: ".github/workflows/new.yml", Branch: "main", Message: "explicit create", Content: content}); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	for _, bad := range []string{"main\nHeader: token", "../main", "feature/.hidden", "feature.lock", "feature//x"} {
		if _, err := c.PutWorkflowFile(context.Background(), "a/b", WorkflowFileInput{Path: ".github/workflows/new.yml", Branch: bad, Message: "x", Content: content}); err == nil {
			t.Fatal("bad branch", bad)
		}
	}
	if calls != 1 {
		t.Fatal("invalid input performed writes", calls)
	}
}
