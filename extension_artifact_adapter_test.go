package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

// Independent advisory literals are metadata vectors, not signed-history or
// runtime evidence. Expectations are frozen literal preimages and SHA256s.
func TestExtensionArtifactCutLiteralVectors(t *testing.T) {
	repository := "sha256:" + strings.Repeat("0", 64)
	refs := []artifactSource{{Ref: "refs/hn/artifacts/" + strings.Repeat("a", 64) + "/836585156fb39042580cf3ae63f1b754e6838710f85d586b1f34a540728e71a7", OID: strings.Repeat("1", 40)}, {Ref: "refs/hn/remotes/public/artifacts/" + strings.Repeat("a", 64) + "/836585156fb39042580cf3ae63f1b754e6838710f85d586b1f34a540728e71a7", OID: strings.Repeat("2", 40)}}
	ids := []string{"sha256:" + strings.Repeat("5", 64), "sha256:" + strings.Repeat("3", 64), "sha256:" + strings.Repeat("4", 64), "sha256:" + strings.Repeat("3", 64)}
	for _, tc := range []struct {
		name, viewPreimage, viewDigest, cutPreimage, cutDigest string
		types, ids                                             []string
		refs                                                   []artifactSource
	}{
		{name: "normal", viewPreimage: `{"admitted_ids":["sha256:3333333333333333333333333333333333333333333333333333333333333333","sha256:4444444444444444444444444444444444444444444444444444444444444444","sha256:5555555555555555555555555555555555555555555555555555555555555555"],"retained_refs":[],"visible_ids":["sha256:3333333333333333333333333333333333333333333333333333333333333333","sha256:4444444444444444444444444444444444444444444444444444444444444444","sha256:5555555555555555555555555555555555555555555555555555555555555555"]}`, viewDigest: "sha256:639debba1a9e75c6ba34529cb3fce1651c27e3a375a39f9e1566fe6facd9df2e", cutPreimage: `{"authority_commit":"","policy_digest":"","refs":[{"oid":"1111111111111111111111111111111111111111","ref":"refs/hn/artifacts/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/836585156fb39042580cf3ae63f1b754e6838710f85d586b1f34a540728e71a7"},{"oid":"2222222222222222222222222222222222222222","ref":"refs/hn/remotes/public/artifacts/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/836585156fb39042580cf3ae63f1b754e6838710f85d586b1f34a540728e71a7"}],"repository_id":"sha256:0000000000000000000000000000000000000000000000000000000000000000","selection":{"families":[],"types":["example/note"]},"view_digest":"sha256:639debba1a9e75c6ba34529cb3fce1651c27e3a375a39f9e1566fe6facd9df2e"}`, cutDigest: "sha256:4e658ed082cea5c9be2439fc4528e19051b89abe62d9d7fcf2af725b4d6aaf1a", types: []string{"example/note", "example/note"}, ids: ids, refs: []artifactSource{refs[1], refs[0], refs[0]}},
		{name: "empty namespace", viewPreimage: `{"admitted_ids":[],"retained_refs":[],"visible_ids":[]}`, viewDigest: "sha256:1f3340ea256c3808d709ad97cb10dc1f379f9ddd6a7277c4b6bc43221a200f9d", cutPreimage: `{"authority_commit":"","policy_digest":"","refs":[{"oid":"1111111111111111111111111111111111111111","ref":"refs/hn/artifacts/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/836585156fb39042580cf3ae63f1b754e6838710f85d586b1f34a540728e71a7"},{"oid":"2222222222222222222222222222222222222222","ref":"refs/hn/remotes/public/artifacts/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/836585156fb39042580cf3ae63f1b754e6838710f85d586b1f34a540728e71a7"}],"repository_id":"sha256:0000000000000000000000000000000000000000000000000000000000000000","selection":{"families":[],"types":[]},"view_digest":"sha256:1f3340ea256c3808d709ad97cb10dc1f379f9ddd6a7277c4b6bc43221a200f9d"}`, cutDigest: "sha256:22e9db95091b80ddadd2a6ad555e2b52477ef2674d1be61c2204e8f515c601cb", types: nil, ids: nil, refs: []artifactSource{refs[1], refs[0], refs[0]}},
		{name: "opaque", viewPreimage: `{"admitted_ids":[],"retained_refs":[],"visible_ids":[]}`, viewDigest: "sha256:1f3340ea256c3808d709ad97cb10dc1f379f9ddd6a7277c4b6bc43221a200f9d", cutPreimage: `{"authority_commit":"","policy_digest":"","refs":[],"repository_id":"sha256:0000000000000000000000000000000000000000000000000000000000000000","selection":{"families":[],"types":[]},"view_digest":"sha256:1f3340ea256c3808d709ad97cb10dc1f379f9ddd6a7277c4b6bc43221a200f9d"}`, cutDigest: "sha256:df2e63a287b224bc0adfb69fd27badcb35bfced67afefaeb8c623177f6233618", types: nil, ids: nil, refs: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if actual := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(tc.viewPreimage))); actual != tc.viewDigest {
				t.Fatal("literal view vector", actual)
			}
			if actual := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(tc.cutPreimage))); actual != tc.cutDigest {
				t.Fatal("literal Cut vector", actual)
			}
			cut, err := extensionArtifactCut(repository, tc.refs, tc.types, tc.ids)
			if err != nil || cut != tc.cutDigest {
				t.Fatal("canonical Cut mapping", cut, err)
			}
		})
	}
	conflicting := append([]artifactSource{}, refs...)
	conflicting = append(conflicting, artifactSource{Ref: refs[0].Ref, OID: strings.Repeat("9", 40)})
	if _, err := extensionArtifactCut(repository, conflicting, nil, nil); err == nil {
		t.Fatal("contradictory accepted witness silently selected")
	}
}

func TestExtensionArtifactOpaqueQueryIdentityIsSeparateFromCut(t *testing.T) {
	call := extension.Invocation{Binding: extension.Binding{RepositoryID: "sha256:" + strings.Repeat("0", 64)}, Request: extension.Request{Method: "artifact.read", Params: map[string]any{"id": nil, "opaque_id": "sha256:" + strings.Repeat("1", 64), "type_id": nil, "limit": float64(1), "cursor": nil, "mode": "inspect", "opaque": true}}}
	first := extensionArtifactQuery(call)
	cut, err := extensionArtifactCut(call.Binding.RepositoryID, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	call.Request.Params["opaque_id"] = "sha256:" + strings.Repeat("2", 64)
	second := extensionArtifactQuery(call)
	otherCut, err := extensionArtifactCut(call.Binding.RepositoryID, nil, nil, nil)
	if err != nil || cut != otherCut || first == second {
		t.Fatal("opaque query identity confused with shared empty projection Cut", cut, otherCut, first, second, err)
	}
}
