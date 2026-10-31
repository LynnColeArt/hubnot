package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"hubnot/internal/artifact"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const artifactReadEntries = 100000
const artifactReadObjectBytes = 8388608
const artifactReadTotalBytes = 268435456

type artifactSource struct {
	Ref string `json:"ref"`
	OID string `json:"oid"`
}
type storedArtifact struct {
	ID           string         `json:"id"`
	Envelope     map[string]any `json:"envelope"`
	Signature    string         `json:"signature"`
	Validation   string         `json:"validation"`
	Diagnostics  []any          `json:"diagnostics"`
	commit       string
	verified     artifact.Verified
	parentCommit string
}
type artifactHistory struct {
	Sources     []artifactSource
	ByID        map[string]*storedArtifact
	Chains      map[string][]*storedArtifact
	Cut         string
	gitDir      string
	readBytes   int64
	readObjects map[string]bool
}

func artifactFailure(code, message string) error {
	return &artifact.Error{Code: code, Message: message}
}
func artifactText(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func artifactPayload(s *storedArtifact) map[string]any {
	m, _ := s.Envelope["payload"].(map[string]any)
	return m
}
func artifactIDs(v any) []string {
	a, _ := v.([]any)
	out := make([]string, 0, len(a))
	for _, v := range a {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}
func artifactStream(actor string) string { return eventID([]byte("hn-artifact-stream-v1\x00" + actor)) }
func artifactRef(actor string) string {
	return "refs/hn/artifacts/" + actor + "/" + strings.TrimPrefix(artifactStream(actor), "sha256:")
}
func parseArtifactRef(ref string) (actor string, ok bool) {
	p := strings.Split(ref, "/")
	if len(p) == 5 && p[0] == "refs" && p[1] == "hn" && p[2] == "artifacts" {
		actor = p[3]
		return actor, validActorFingerprint(actor) && ref == artifactRef(actor)
	}
	if len(p) == 7 && p[0] == "refs" && p[1] == "hn" && p[2] == "remotes" && validMemoryRemote(p[3]) && p[4] == "artifacts" {
		actor = p[5]
		return actor, validActorFingerprint(actor) && p[6] == strings.TrimPrefix(artifactStream(actor), "sha256:")
	}
	return "", false
}
func artifactSources(ctx context.Context, dir string) ([]artifactSource, error) {
	b, e := projectGit(ctx, dir, nil, 32*1024*1024, "for-each-ref", "--count=100001", "--format=%(refname) %(objectname)", "refs/hn/artifacts/", "refs/hn/remotes/*/artifacts/*/*")
	if e != nil {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact read cancelled")
		}
		return nil, artifactFailure("verification_failure", "cannot enumerate accepted artifact refs")
	}
	out := []artifactSource{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		p := strings.Fields(line)
		if len(p) != 2 {
			return nil, artifactFailure("invalid_history", "invalid accepted ref")
		}
		_, ok := parseArtifactRef(p[0])
		if !ok {
			if strings.HasPrefix(p[0], "refs/hn/artifacts/") || strings.Contains(strings.TrimPrefix(p[0], "refs/hn/remotes/"), "/artifacts/") {
				return nil, artifactFailure("invalid_history", "malformed accepted artifact ref")
			}
			continue
		}
		if !validGitOID(p[1]) {
			return nil, artifactFailure("invalid_history", "invalid accepted Git identity")
		}
		out = append(out, artifactSource{p[0], p[1]})
		if len(out) > artifactReadEntries {
			return nil, artifactFailure("resource_limit", "selected artifact ref limit exceeded")
		}
	}
	// The count sentinel detects truncation before any history is projected.
	if bytes.Count(b, []byte("\n")) >= 100001 {
		return nil, artifactFailure("resource_limit", "accepted ref enumeration exceeds read limit")
	}
	return out, nil
}

type artifactObjectReader struct {
	ctx   context.Context
	dir   string
	total int64
	seen  map[string]bool
}

func (r *artifactObjectReader) object(oid, kind string, limit int64) ([]byte, error) {
	if !validGitOID(oid) {
		return nil, artifactFailure("invalid_history", "invalid Git object identity")
	}
	meta, e := projectGit(r.ctx, r.dir, []byte(oid+"\n"), 256, "cat-file", "--batch-check=%(objecttype) %(objectsize)")
	if e != nil {
		return nil, r.readError()
	}
	p := strings.Fields(string(meta))
	if len(p) != 2 || p[0] != kind {
		return nil, artifactFailure("verification_failure", "accepted artifact object is missing or has wrong type")
	}
	n, e := strconv.ParseInt(p[1], 10, 64)
	if e != nil || n < 0 {
		return nil, artifactFailure("verification_failure", "invalid Git object size")
	}
	if n > artifactReadObjectBytes || n > limit {
		return nil, artifactFailure("resource_limit", "artifact object exceeds read limit")
	}
	if !r.seen[oid] {
		if n > artifactReadTotalBytes-r.total {
			return nil, artifactFailure("resource_limit", "artifact aggregate exceeds read limit")
		}
		r.total += n
		r.seen[oid] = true
	}
	b, e := projectGit(r.ctx, r.dir, nil, n+1, "cat-file", kind, oid)
	if e != nil || int64(len(b)) != n {
		return nil, r.readError()
	}
	var digest hash.Hash = sha1.New()
	if len(oid) == 64 {
		digest = sha256.New()
	}
	fmt.Fprintf(digest, "%s %d\x00", kind, n)
	_, _ = digest.Write(b)
	if hex.EncodeToString(digest.Sum(nil)) != oid {
		return nil, artifactFailure("verification_failure", "accepted Git object hash mismatch")
	}
	return b, nil
}
func (r *artifactObjectReader) readError() error {
	if r.ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact read cancelled")
	}
	return artifactFailure("verification_failure", "accepted artifact object unavailable; no fetch attempted")
}
func (r *artifactObjectReader) commit(oid string) (*storedArtifact, string, error) {
	b, e := r.object(oid, "commit", artifactReadObjectBytes)
	if e != nil {
		return nil, "", e
	}
	headers, message, ok := strings.Cut(string(b), "\n\n")
	if !ok {
		return nil, "", artifactFailure("invalid_history", "invalid artifact commit")
	}
	tree, parent := "", ""
	for _, line := range strings.Split(headers, "\n") {
		if strings.HasPrefix(line, "tree ") {
			if tree != "" {
				return nil, "", artifactFailure("invalid_history", "multiple artifact trees")
			}
			tree = strings.TrimPrefix(line, "tree ")
		}
		if strings.HasPrefix(line, "parent ") {
			if parent != "" {
				return nil, "", artifactFailure("actor_stream_conflict", "artifact stream has multiple Git parents")
			}
			parent = strings.TrimPrefix(line, "parent ")
		}
	}
	if _, e = r.object(tree, "tree", artifactReadObjectBytes); e != nil {
		return nil, "", e
	}
	entries, e := projectGit(r.ctx, r.dir, nil, 1024, "ls-tree", "-z", tree)
	if e != nil {
		return nil, "", r.readError()
	}
	names := map[string]string{}
	for _, entry := range bytes.Split(entries, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		header, name, ok := strings.Cut(string(entry), "\t")
		f := strings.Fields(header)
		if !ok || len(f) != 3 || f[0] != "100644" || f[1] != "blob" || (name != "artifact.json" && name != "signature") || names[name] != "" {
			return nil, "", artifactFailure("invalid_history", "artifact tree requires exactly two regular blobs")
		}
		names[name] = f[2]
	}
	if len(names) != 2 {
		return nil, "", artifactFailure("invalid_history", "artifact tree requires artifact.json and signature")
	}
	raw, e := r.object(names["artifact.json"], "blob", artifact.MaxEnvelopeBytes)
	if e != nil {
		return nil, "", e
	}
	sig, e := r.object(names["signature"], "blob", 86)
	if e != nil {
		return nil, "", e
	}
	v, e := artifact.Verify(raw, string(sig), nil)
	if e != nil {
		return nil, "", e
	}
	if message != "hn artifact "+v.ID()+"\n" {
		return nil, "", artifactFailure("invalid_history", "artifact commit message mismatch")
	}
	var env map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if e = d.Decode(&env); e != nil {
		return nil, "", artifactFailure("invalid_history", "invalid artifact envelope")
	}
	return &storedArtifact{ID: v.ID(), Envelope: env, Signature: string(sig), Validation: "verified", Diagnostics: []any{}, commit: oid, verified: v, parentCommit: parent}, parent, nil
}
func loadArtifactHistory(ctx context.Context) (*artifactHistory, error) {
	dir, e := requireGitRepository()
	if e != nil {
		return nil, artifactFailure("repository_error", "artifact command requires a Git repository")
	}
	sources, e := artifactSources(ctx, dir)
	if e != nil {
		return nil, e
	}
	h, e := loadArtifactHistoryFromSources(ctx, dir, sources)
	if e != nil {
		return nil, e
	}
	// Discovery owns the complete accepted selection, including additions/removals.
	after, e := artifactSources(ctx, dir)
	if e != nil {
		return nil, e
	}
	a, _ := json.Marshal(sources)
	b, _ := json.Marshal(after)
	if !bytes.Equal(a, b) {
		return nil, artifactFailure("concurrent_update", "artifact sources changed while reading")
	}
	h.Cut, e = h.sourceCut()
	return h, e
}

// loadArtifactHistoryFromSources verifies only an explicit public source set in
// an application-selected absolute Git directory. It does not discover sources,
// select policy or borrow repository identity from cwd. Cut stays empty: this is
// internal verification data, never a public accepted-view query result. A caller
// must keep the supplied objects available while using its budgeted reader.
func loadArtifactHistoryFromSources(ctx context.Context, dir string, sources []artifactSource) (*artifactHistory, error) {
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact read cancelled")
	}
	if !filepath.IsAbs(dir) {
		return nil, artifactFailure("repository_error", "explicit artifact Git directory must be absolute")
	}
	if len(sources) > artifactReadEntries {
		return nil, artifactFailure("resource_limit", "selected artifact ref limit exceeded")
	}
	format, e := projectGit(ctx, dir, nil, 32, "rev-parse", "--show-object-format")
	if e != nil {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact read cancelled")
		}
		return nil, artifactFailure("repository_error", "cannot identify explicit artifact Git repository")
	}
	oidLength := 40
	switch strings.TrimSpace(string(format)) {
	case "sha1":
	case "sha256":
		oidLength = 64
	default:
		return nil, artifactFailure("repository_error", "unsupported explicit artifact Git object format")
	}
	sources = append([]artifactSource{}, sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Ref < sources[j].Ref })
	for i, source := range sources {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact read cancelled")
		}
		if _, ok := parseArtifactRef(source.Ref); !ok || !validGitOID(source.OID) || len(source.OID) != oidLength || source.OID != strings.ToLower(source.OID) {
			return nil, artifactFailure("invalid_history", "invalid explicit artifact source")
		}
		if i > 0 && sources[i-1].Ref == source.Ref {
			return nil, artifactFailure("invalid_history", "duplicate explicit artifact source")
		}
	}
	if e := artifactCheckSourceWitnesses(ctx, dir, sources); e != nil {
		return nil, e
	}
	h := &artifactHistory{Sources: sources, ByID: map[string]*storedArtifact{}, Chains: map[string][]*storedArtifact{}}
	reader := artifactObjectReader{ctx: ctx, dir: dir, seen: map[string]bool{}}
	cache := map[string]*storedArtifact{}
	parents := map[string]string{}
	for _, source := range sources {
		if ctx.Err() != nil {
			return nil, artifactFailure("cancelled", "artifact read cancelled")
		}
		actor, _ := parseArtifactRef(source.Ref)
		if known := cache[source.OID]; known != nil {
			if known.verified.Actor() != actor {
				return nil, artifactFailure("invalid_history", "artifact ref owner mismatch")
			}
			continue
		}
		chain := []*storedArtifact{}
		visiting := map[string]bool{}
		for oid := source.OID; oid != ""; oid = parents[oid] {
			if ctx.Err() != nil {
				return nil, artifactFailure("cancelled", "artifact read cancelled")
			}
			if visiting[oid] {
				return nil, artifactFailure("invalid_history", "cyclic artifact stream")
			}
			visiting[oid] = true
			s := cache[oid]
			if s == nil {
				if len(cache) >= artifactReadEntries {
					return nil, artifactFailure("resource_limit", "selected artifact record limit exceeded")
				}
				var parent string
				s, parent, e = reader.commit(oid)
				if e != nil {
					return nil, e
				}
				cache[oid] = s
				parents[oid] = parent
			}
			if s.verified.Actor() != actor {
				return nil, artifactFailure("invalid_history", "artifact ref owner mismatch")
			}
			chain = append(chain, s)
		}
		for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
			chain[i], chain[j] = chain[j], chain[i]
		}
		var previous *artifact.Verified
		for i, s := range chain {
			if ctx.Err() != nil {
				return nil, artifactFailure("cancelled", "artifact read cancelled")
			}
			if i == 0 && !s.verified.Genesis() {
				return nil, artifactFailure("invalid_history", "artifact stream missing genesis")
			}
			checked, err := artifact.Verify(s.verified.Bytes(), s.Signature, previous)
			if err != nil {
				return nil, err
			}
			previous = &checked
			if old := h.ByID[s.ID]; old != nil && old.Signature != s.Signature {
				return nil, artifactFailure("verification_failure", "contradictory signed artifact")
			}
			h.ByID[s.ID] = s
		}
		if old := h.Chains[actor]; old != nil {
			n := len(old)
			if len(chain) < n {
				n = len(chain)
			}
			for i := 0; i < n; i++ {
				if old[i].ID != chain[i].ID || old[i].commit != chain[i].commit {
					return nil, artifactFailure("actor_stream_conflict", "divergent accepted actor streams")
				}
			}
			if len(chain) < len(old) {
				continue
			}
		}
		h.Chains[actor] = chain
	}
	if e = h.validate(ctx); e != nil {
		return nil, e
	}
	if e = artifactCheckSourceWitnesses(ctx, dir, sources); e != nil {
		return nil, e
	}
	h.gitDir, h.readBytes, h.readObjects = dir, reader.total, reader.seen
	return h, nil
}

// Ref metadata is checked independently of object availability so a retained
// witness with a missing object reaches the normal no-fetch object refusal.
func artifactCheckSourceWitnesses(ctx context.Context, dir string, sources []artifactSource) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact read cancelled")
	}
	if len(sources) == 0 {
		return nil
	}
	var input strings.Builder
	wanted := make(map[string]string, len(sources))
	for _, source := range sources {
		input.WriteString(source.Ref)
		input.WriteByte('\n')
		wanted[source.Ref] = source.OID
	}
	raw, e := projectGit(ctx, dir, []byte(input.String()), 32*1024*1024, "for-each-ref", "--count=100001", "--format=%(refname) %(objectname)", "--stdin")
	if e != nil {
		if ctx.Err() != nil {
			return artifactFailure("cancelled", "artifact read cancelled")
		}
		return artifactFailure("verification_failure", "cannot verify selected artifact source witnesses")
	}
	if bytes.Count(raw, []byte("\n")) >= 100001 {
		return artifactFailure("resource_limit", "selected source witness enumeration exceeds read limit")
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		p := strings.Fields(line)
		if len(p) != 2 {
			return artifactFailure("verification_failure", "invalid selected artifact source witness")
		}
		if oid, selected := wanted[p[0]]; selected {
			if p[1] != oid {
				return artifactFailure("concurrent_update", "artifact source witness changed while reading")
			}
			delete(wanted, p[0])
		}
	}
	if len(wanted) != 0 {
		return artifactFailure("concurrent_update", "artifact source witness missing while reading")
	}
	return nil
}
func (h *artifactHistory) validate(ctx context.Context) error {
	ops := map[string]bool{}
	for _, s := range h.ByID {
		key := s.verified.Actor() + "\x00" + artifactText(s.Envelope, "operation")
		if ops[key] {
			return artifactFailure("invalid_history", "multiple artifacts use one actor operation")
		}
		ops[key] = true
		p := artifactPayload(s)
		if s.verified.Kind() == "type.publish" {
			raw, _ := json.Marshal(p)
			typ, e := artifact.ValidateType(ctx, raw)
			if e != nil {
				return e
			}
			for _, id := range artifactIDs(s.Envelope["parents"]) {
				parent := h.ByID[id]
				if parent == nil {
					return artifactFailure("incomplete_dependency", "type parent missing")
				}
				if parent.verified.Kind() != "type.publish" {
					return artifactFailure("invalid_history", "type parent is not a declaration")
				}
				raw, _ = json.Marshal(artifactPayload(parent))
				old, e := artifact.ValidateType(ctx, raw)
				if e != nil {
					return e
				}
				if old.TypeID != typ.TypeID || old.Shape != typ.Shape || old.SchemaVersion >= typ.SchemaVersion {
					return artifactFailure("invalid_history", "invalid type evolution")
				}
			}
			continue
		}
		typ := h.ByID[artifactText(p, "type_revision")]
		if typ == nil {
			return artifactFailure("incomplete_dependency", "exact artifact schema missing")
		}
		ps := []any{}
		for _, id := range artifactIDs(s.Envelope["parents"]) {
			parent := h.ByID[id]
			if parent == nil {
				return artifactFailure("incomplete_dependency", "artifact parent missing")
			}
			ps = append(ps, parent.signedValue())
		}
		var opening any
		if id := artifactText(s.Envelope, "record"); id != "" {
			o := h.ByID[id]
			if o == nil {
				return artifactFailure("incomplete_dependency", "artifact opening missing")
			}
			opening = o.signedValue()
		}
		if _, e := artifact.ValidateRecordContext(ctx, map[string]any{"artifact": s.signedValue(), "type": typ.signedValue(), "parents": ps, "opening": opening}); e != nil {
			return e
		}
		for _, field := range []string{"members", "relationships"} {
			links, _ := p[field].([]any)
			for _, value := range links {
				link := value.(map[string]any)
				target := h.ByID[artifactText(link, "target")]
				if target == nil {
					if link["required"] == true {
						return artifactFailure("incomplete_dependency", "required artifact target missing")
					}
					continue
				}
				if !strings.HasPrefix(target.verified.Kind(), "record.") || (link["target_mode"] == "logical" && target.verified.Kind() != "record.open") {
					return artifactFailure("invalid_history", "artifact target has wrong lineage")
				}
			}
		}
	}
	return nil
}
func artifactRepositoryID() (string, string, error) {
	dir, e := contributionGitDirectory()
	if e != nil {
		return "", "", artifactFailure("repository_error", "cannot identify physical common Git directory")
	}
	return eventID([]byte("hn-extension-repository-v1\x00" + dir)), dir, nil
}
func (h *artifactHistory) sourceCut() (string, error) {
	repo, _, e := artifactRepositoryID()
	if e != nil {
		return "", e
	}
	ids := make([]string, 0, len(h.ByID))
	types := []string{}
	seen := map[string]bool{}
	for id, s := range h.ByID {
		ids = append(ids, id)
		typ := artifactText(artifactPayload(s), "type_id")
		if !seen[typ] {
			types = append(types, typ)
			seen[typ] = true
		}
	}
	sort.Strings(ids)
	sort.Strings(types)
	view, _ := json.Marshal(map[string]any{"admitted_ids": ids, "visible_ids": ids, "retained_refs": []artifactSource{}})
	refs := make([]any, 0, len(h.Sources))
	for _, source := range h.Sources {
		refs = append(refs, map[string]any{"ref": source.Ref, "oid": source.OID})
	}
	cut, _ := json.Marshal(map[string]any{"repository_id": repo, "refs": refs, "selection": map[string]any{"families": []string{}, "types": types}, "authority_commit": "", "policy_digest": "", "view_digest": eventID(view)})
	return eventID(cut), nil
}

// The verified loader owns these immutable maps. Borrowing their bounded values
// avoids a second aggregate JSON serialization of shared parent envelopes.
func (s *storedArtifact) signedValue() map[string]any {
	return map[string]any{"id": s.ID, "envelope": s.Envelope, "signature": s.Signature, "validation": s.Validation, "diagnostics": s.Diagnostics}
}
