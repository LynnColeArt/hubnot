package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"hubnot/internal/artifact"
	"hubnot/internal/extension"
)

func TestExtensionArtifactCursorSelectionAndCanonicalEncoding(t *testing.T) {
	call := extension.Invocation{Request: extension.Request{Method: "artifact.read", Params: map[string]any{"type_id": "example.test/note", "id": nil, "opaque_id": nil, "mode": "view", "opaque": false, "cursor": nil, "limit": float64(1)}}}
	page := extensionArtifactPageAt(call, "cut", []any{"a", "b", "c"}, 0)
	if len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal(page)
	}
	call.Request.Params["cursor"] = *page.NextCursor
	call.Request.Params["limit"] = float64(2)
	offset, err := extensionArtifactOffset(call, "cut", 3)
	if err != nil || offset != 1 {
		t.Fatal(offset, err)
	}
	page = extensionArtifactPageAt(call, "cut", []any{"a", "b", "c"}, offset)
	if len(page.Items) != 2 || page.NextCursor != nil {
		t.Fatal(page)
	}
	for _, change := range []func(){
		func() { call.Request.Params["type_id"] = "other.test/note" },
		func() { call.Request.Params["mode"] = "history" },
		func() { call.Binding.Epoch = "changed" },
		func() {
			call.Enablement.Grants = []extension.Grant{{Selector: "artifact:example.test/note", Actions: []string{"read"}}}
		},
	} {
		call.Request.Params["type_id"], call.Request.Params["mode"] = "example.test/note", "view"
		call.Binding.Epoch = ""
		call.Enablement.Grants = nil
		change()
		if _, err = extensionArtifactOffset(call, "cut", 3); err == nil {
			t.Fatal("changed selection accepted old cursor")
		}
	}
	call.Binding.Epoch = ""
	call.Enablement.Grants = nil
	call.Request.Params["type_id"], call.Request.Params["mode"] = "example.test/note", "view"
	if _, err = extensionArtifactOffset(call, "different-cut", 3); err == nil {
		t.Fatal("changed cut accepted")
	}
	valid := extensionArtifactCursor{1, extensionArtifactQuery(call), "cut", 1}
	raw, _ := json.Marshal(valid)
	for _, invalid := range []string{"", "!", base64.RawURLEncoding.EncodeToString(append(raw, ' ')), base64.RawURLEncoding.EncodeToString([]byte(`{"version":1,"version":1,"query":"x","cut":"cut","offset":1}`))} {
		call.Request.Params["cursor"] = invalid
		if _, err = extensionArtifactOffset(call, "cut", 3); err == nil {
			t.Fatal("noncanonical cursor accepted", invalid)
		}
	}
}
func TestExtensionArtifactPublicErrorAdaptation(t *testing.T) {
	call := extension.Invocation{Request: extension.Request{Params: map[string]any{"actor": "original", "operation": "original-key"}}}
	for _, tc := range []struct {
		err                    error
		origin, code, recovery string
		operation              bool
	}{
		{artifactFailure("invalid_input", "safe"), "artifact", "invalid_input", "do_not_retry", false},
		{&artifactStorageError{Origin: "native", Code: "actor_mismatch", Message: "safe"}, "native", "actor_mismatch", "do_not_retry", false},
		{artifactListCheckpoint(), "host", "checkpoint_invalidated", "rescan_required", false},
		{&artifact.Error{Code: "uncertain_operation", Message: "safe"}, "artifact", "uncertain_operation", "inspect_operation", true},
		{errors.New("PRIVATE-WP18 raw stderr secret path"), "artifact", "repository_error", "do_not_retry", false},
	} {
		err := extensionArtifactError(tc.err, call).(*extension.Error)
		if err.Origin != tc.origin || err.Code != tc.code || err.Recovery != tc.recovery || (err.Operation != nil) != tc.operation || err.Message == "PRIVATE-WP18 raw stderr secret path" {
			t.Fatal(err)
		}
	}
}
