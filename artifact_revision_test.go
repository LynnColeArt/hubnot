package main

import (
	"context"
	"encoding/json"
	"errors"
	"hubnot/internal/artifact"
	"reflect"
	"strings"
	"testing"
)

func revisionError(t *testing.T, e error, code string) {
	t.Helper()
	var a *artifact.Error
	if !errors.As(e, &a) || a.Code != code {
		t.Fatalf("want %s, got %v", code, e)
	}
}
func TestArtifactSuccessorStrictInput(t *testing.T) {
	id := "sha256:" + strings.Repeat("a", 64)
	schema := "sha256:" + strings.Repeat("b", 64)
	valid := revisionInput(t, id, []string{id}, revisionPayload(schema, "snow 雪\ne\u0301"))
	got, e := artifactParseSuccessor(context.Background(), []byte(valid), "record.revise")
	if e != nil {
		t.Fatal(e)
	}
	if got["payload"].(map[string]any)["content"] != "snow 雪\ne\u0301" {
		t.Fatal(got)
	}
	for _, raw := range []string{
		strings.Replace(valid, `"record":`, `"record":null,"record":`, 1),
		strings.Replace(valid, `"parents":["`+id+`"]`, `"parents":["`+id+`","`+id+`"]`, 1),
		strings.Replace(valid, `"record":"`+id+`"`, `"record":null`, 1),
		strings.Replace(valid, `"parents":["`+id+`"]`, `"parents":[]`, 1),
		strings.TrimSuffix(valid, "}") + `,"unknown":true}`,
	} {
		_, e := artifactParseSuccessor(context.Background(), []byte(raw), "record.revise")
		revisionError(t, e, "invalid_input")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = artifactParseSuccessor(ctx, []byte(valid), "record.revise")
	revisionError(t, e, "cancelled")
	// Parent order is caller intent and cannot be silently sorted.
	raw := revisionInput(t, id, []string{schema, id}, revisionPayload(schema, ""))
	_, e = artifactParseSuccessor(context.Background(), []byte(raw), "record.resolve")
	revisionError(t, e, "invalid_input")
	b, e := json.Marshal(got)
	if e != nil || !reflect.DeepEqual(string(b), valid) {
		t.Fatal("input changed", e)
	}
}
