// Package migration defines bounded, provider-neutral project observations.
// Foreign content is descriptive evidence and never native authority.
package migration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Locator struct {
	Kind         string `json:"kind"`
	Endpoint     string `json:"endpoint"`
	Organization string `json:"organization"`
	Project      string `json:"project"`
	Repository   string `json:"repository"`
}
type Key struct {
	Provider  string `json:"provider"`
	Instance  string `json:"instance"`
	ProjectID string `json:"project_id"`
	Kind      string `json:"kind"`
	ID        string `json:"id"`
}
type Link struct {
	Relation string `json:"relation"`
	Target   Key    `json:"target"`
	Revision string `json:"revision"`
}
type Blob struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"media_type"`
	Name      string `json:"name"`
}
type Record struct {
	Key       Key             `json:"key"`
	Selector  string          `json:"selector"`
	Revision  string          `json:"revision"`
	SourceURL string          `json:"source_url"`
	Author    string          `json:"author"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	Schema    string          `json:"schema"`
	Content   json.RawMessage `json:"content"`
	Links     []Link          `json:"links"`
	Blobs     []string        `json:"blobs"`
}
type NativeRef struct {
	SourceRef string `json:"source_ref"`
	OID       string `json:"oid"`
	Kind      string `json:"kind"`
}
type Snapshot struct {
	Schema     string      `json:"schema"`
	Source     Locator     `json:"source"`
	SourceID   string      `json:"source_id"`
	Records    []Record    `json:"records"`
	Blobs      []Blob      `json:"blobs"`
	NativeRefs []NativeRef `json:"native_refs"`
	GitBundle  string      `json:"git_bundle"`
	Findings   []Finding   `json:"findings"`
}
type Limits struct {
	Records         int   `json:"records"`
	Blobs           int   `json:"blobs"`
	Refs            int   `json:"refs"`
	GitObjects      int   `json:"git_objects"`
	BlobBytes       int64 `json:"blob_bytes"`
	ManifestBytes   int64 `json:"manifest_bytes"`
	AttachmentBytes int64 `json:"attachment_bytes"`
	GitObjectBytes  int64 `json:"git_object_bytes"`
	GitTotalBytes   int64 `json:"git_total_bytes"`
}
type Finding struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Path    string `json:"path"`
	Message string `json:"message"`
}
type Capability struct {
	Kind     string   `json:"kind"`
	Read     bool     `json:"read"`
	Preserve bool     `json:"preserve"`
	Create   bool     `json:"create"`
	Update   bool     `json:"update"`
	Limits   []string `json:"limits"`
	Reason   string   `json:"reason"`
}
type Selection struct {
	Source      Locator           `json:"source"`
	Destination Locator           `json:"destination"`
	Refs        []string          `json:"refs"`
	Kinds       []string          `json:"kinds"`
	Mappings    map[string]string `json:"mappings"`
}
type BlobSource interface {
	Open(context.Context, string) (io.ReadCloser, error)
}
type Source interface {
	Capabilities(context.Context) ([]Capability, error)
	Snapshot(context.Context, Selection, Limits) (Snapshot, BlobSource, error)
}

// IssueContent and CommentContent are closed v1 schemas. Attribution belongs to Record.
type IssueContent struct {
	Schema    string   `json:"schema"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	State     string   `json:"state"`
	Type      string   `json:"type"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
}
type CommentContent struct {
	Schema string `json:"schema"`
	Body   string `json:"body"`
}

// PreservedContent retains original retrieved bytes without claiming native equivalence.
// SourceBlob must name a declared and record-referenced blob. Summary is descriptive.
type PreservedContent struct {
	Schema     string `json:"schema"`
	Summary    string `json:"summary"`
	SourceBlob string `json:"source_blob"`
}

func DefaultLimits() Limits {
	return Limits{10000, 1024, 1024, 100000, 16 << 20, 16 << 20, 256 << 20, 16 << 20, 512 << 20}
}
func (l Limits) Validate() error {
	for _, n := range []int{l.Records, l.Blobs, l.Refs, l.GitObjects} {
		if n > 1<<30 {
			return fmt.Errorf("snapshot count budget exceeds safe arithmetic bounds")
		}
	}
	// Leave headroom for bounded stream framing and size+1 probes.
	for _, n := range []int64{int64(l.Records), int64(l.Blobs), int64(l.Refs), int64(l.GitObjects), l.BlobBytes, l.ManifestBytes, l.AttachmentBytes, l.GitObjectBytes, l.GitTotalBytes} {
		if n > 1<<54 {
			return fmt.Errorf("snapshot budget exceeds safe arithmetic bounds")
		}
	}
	if l.Records <= 0 || l.Blobs <= 0 || l.Refs <= 0 || l.GitObjects <= 0 || l.BlobBytes <= 0 || l.ManifestBytes <= 0 || l.AttachmentBytes <= 0 || l.GitObjectBytes <= 0 || l.GitTotalBytes <= 0 {
		return fmt.Errorf("all snapshot budgets must be positive")
	}
	return nil
}
func Digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func ValidDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil && strings.ToLower(s) == s
}
func (k Key) String() string { b, _ := json.Marshal(k); return string(b) }
func text(s string, n int, required bool) bool {
	return utf8.ValidString(s) && len(s) <= n && (!required || s != "") && !strings.ContainsRune(s, 0)
}
func validKey(k Key) bool {
	instance, err := NormalizeInstance(k.Instance)
	return err == nil && instance == k.Instance && text(k.Provider, 128, true) && strings.ToLower(k.Provider) == k.Provider && text(k.Instance, 4096, true) && text(k.ProjectID, 1024, true) && text(k.Kind, 128, true) && text(k.ID, 4096, true)
}

// NormalizeInstance canonicalizes an explicit provider root without interpreting
// opaque IDs as URLs. Credentials, queries and fragments cannot be identity.
func NormalizeInstance(raw string) (string, error) {
	if !validURL(raw) || raw == "" || !text(raw, 4096, true) {
		return "", fmt.Errorf("invalid provider instance")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if u.Scheme == "https" && strings.HasSuffix(u.Host, ":443") {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}
	if u.Scheme == "http" && strings.HasSuffix(u.Host, ":80") {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}
func validURL(s string) bool {
	if s == "" {
		return true
	}
	u, e := url.Parse(s)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil && u.Fragment == "" && u.RawQuery == ""
}
func ValidRef(r string) bool {
	for _, c := range r {
		if c <= 32 || c == 127 {
			return false
		}
	}
	if len(r) > 1024 || !strings.HasPrefix(r, "refs/") || strings.ContainsAny(r, " ~^:?*[\\\x00\r\n\t") || strings.Contains(r, "..") || strings.Contains(r, "@{") || strings.Contains(r, "//") || strings.HasSuffix(r, "/") {
		return false
	}
	for _, s := range strings.Split(r, "/") {
		if s == "" || strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") || strings.HasSuffix(s, ".lock") {
			return false
		}
	}
	return utf8.ValidString(r)
}
func ValidOID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil && strings.ToLower(s) == s
}
func NativeKind(ref string) string {
	if strings.HasPrefix(ref, "refs/hn/remotes/") {
		parts := strings.Split(strings.TrimPrefix(ref, "refs/hn/remotes/"), "/")
		if len(parts) == 3 {
			switch parts[1] {
			case "actors":
				return "actor"
			case "proposals":
				return "proposal"
			}
		}
		if len(parts) == 4 && parts[1] == "memory" {
			return "memory"
		}
		return ""
	}
	switch {
	case strings.HasPrefix(ref, "refs/heads/"), strings.HasPrefix(ref, "refs/tags/"):
		return "code"
	case strings.HasPrefix(ref, "refs/hn/actors/"):
		return "actor"
	case strings.HasPrefix(ref, "refs/hn/proposals/"):
		return "proposal"
	case strings.HasPrefix(ref, "refs/hn/memory/"):
		return "memory"
	}
	return ""
}

// StrictJSON rejects unknown, duplicate, null and case-aliased fields and lossy Unicode.
func StrictJSON(raw []byte, out any) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8")
	}
	// encoding/json replaces lone UTF-16 surrogates; reject those before decoding.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		i++
		for i < len(raw) && raw[i] != '"' {
			if raw[i] == '\\' {
				i++
				if i >= len(raw) {
					break
				}
				if raw[i] == 'u' && i+4 < len(raw) {
					var n uint16
					if _, e := fmt.Sscanf(string(raw[i+1:i+5]), "%04x", &n); e != nil {
						return e
					}
					if n >= 0xd800 && n <= 0xdfff {
						if n > 0xdbff || i+10 >= len(raw) || string(raw[i+5:i+7]) != "\\u" {
							return fmt.Errorf("unpaired surrogate")
						}
						var low uint16
						if _, e := fmt.Sscanf(string(raw[i+7:i+11]), "%04x", &low); e != nil || low < 0xdc00 || low > 0xdfff {
							return fmt.Errorf("unpaired surrogate")
						}
						i += 6
					}
					i += 4
				}
			}
			i++
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, e := jsonValue(d, 0)
	if e != nil {
		return e
	}
	if _, e = d.Token(); e != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	if e = exactKeys(v, reflect.TypeOf(out)); e != nil {
		return e
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func jsonValue(d *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("JSON nesting exceeds 32")
	}
	t, e := d.Token()
	if e != nil {
		return nil, e
	}
	if t == nil {
		return nil, fmt.Errorf("null is not supported")
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return nil, e
			}
			s, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("invalid key")
			}
			if _, ok = m[s]; ok {
				return nil, fmt.Errorf("duplicate field %s", s)
			}
			m[s], e = jsonValue(d, depth+1)
			if e != nil {
				return nil, e
			}
		}
		_, e = d.Token()
		return m, e
	case json.Delim('['):
		a := []any{}
		for d.More() {
			v, e := jsonValue(d, depth+1)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		_, e = d.Token()
		return a, e
	}
	return t, nil
}
func exactKeys(v any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == reflect.TypeOf(json.RawMessage{}) {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object")
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			fields[strings.Split(f.Tag.Get("json"), ",")[0]] = f.Type
		}
		for k, v := range m {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("unknown field %q", k)
			}
			if e := exactKeys(v, ft); e != nil {
				return e
			}
		}
		if len(m) != len(fields) {
			return fmt.Errorf("missing required field")
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return fmt.Errorf("expected array")
		}
		for _, v := range a {
			if e := exactKeys(v, t.Elem()); e != nil {
				return e
			}
		}
	}
	return nil
}

func CanonicalContent(kind string, raw []byte) (json.RawMessage, error) {
	var out any
	switch kind {
	case "issue", "work_item":
		out = &IssueContent{}
	case "comment":
		out = &CommentContent{}
	case "discussion", "reply", "project", "project_item", "workflow", "change_request", "review", "inline_comment", "attachment":
		out = &PreservedContent{}
	default:
		return nil, fmt.Errorf("unsupported record kind %q", kind)
	}
	if len(raw) > 1<<20 {
		return nil, fmt.Errorf("content exceeds 1 MiB; preserve raw bytes as a blob with a finding")
	}
	if e := StrictJSON(raw, out); e != nil {
		return nil, e
	}
	switch c := out.(type) {
	case *IssueContent:
		if c.Schema != "hn.migration.issue/1" || !text(c.Title, 1024, true) || !text(c.Body, 65536, false) || !text(c.State, 256, true) || !text(c.Type, 256, false) || len(c.Labels) > 256 || len(c.Assignees) > 256 {
			return nil, fmt.Errorf("invalid issue content")
		}
		for _, values := range [][]string{c.Labels, c.Assignees} {
			seen := map[string]bool{}
			for _, v := range values {
				if !text(v, 1024, true) || seen[v] {
					return nil, fmt.Errorf("invalid or duplicate issue label/assignee")
				}
				seen[v] = true
			}
		}
	case *CommentContent:
		if c.Schema != "hn.migration.comment/1" || !text(c.Body, 65536, true) {
			return nil, fmt.Errorf("invalid comment content")
		}
	case *PreservedContent:
		if c.Schema != "hn.migration.preserved/1" || !text(c.Summary, 65536, false) || !ValidDigest(c.SourceBlob) {
			return nil, fmt.Errorf("invalid preserved source content")
		}
	}
	b, e := json.Marshal(out)
	return b, e
}

// CanonicalSnapshot validates the complete observation manifest, never provider state.
func CanonicalSnapshot(s Snapshot, l Limits) ([]byte, error) {
	if e := l.Validate(); e != nil {
		return nil, e
	}
	if s.Schema != "hn.project-snapshot/1" || !text(s.SourceID, 4096, true) || !text(s.Source.Kind, 128, true) || !validURL(s.Source.Endpoint) || !text(s.Source.Endpoint, 4096, false) || !text(s.Source.Organization, 1024, false) || !text(s.Source.Project, 1024, false) || !text(s.Source.Repository, 4096, false) {
		return nil, fmt.Errorf("invalid snapshot source/schema")
	}
	if len(s.Records) > l.Records || len(s.Blobs) > l.Blobs+1 || len(s.NativeRefs) > l.Refs {
		return nil, fmt.Errorf("snapshot count budget exceeded")
	}
	s.Records = append([]Record{}, s.Records...)
	s.Blobs = append([]Blob{}, s.Blobs...)
	s.NativeRefs = append([]NativeRef{}, s.NativeRefs...)
	s.Findings = append([]Finding{}, s.Findings...)
	blobs := map[string]Blob{}
	var total int64
	attachments := 0
	for _, b := range s.Blobs {
		if !ValidDigest(b.Digest) || b.Size < 0 || !text(b.Name, 4096, false) || !text(b.MediaType, 256, false) {
			return nil, fmt.Errorf("invalid blob")
		}
		if _, ok := blobs[b.Digest]; ok {
			return nil, fmt.Errorf("duplicate blob")
		}
		blobs[b.Digest] = b
		if b.Digest == s.GitBundle {
			if b.Size > GitMaterialLimit(l) {
				return nil, fmt.Errorf("Git material budget exceeded")
			}
		} else {
			attachments++
			if b.Size > l.BlobBytes || b.Size > l.AttachmentBytes-total {
				return nil, fmt.Errorf("attachment budget exceeded")
			}
			total += b.Size
		}
	}
	if attachments > l.Blobs {
		return nil, fmt.Errorf("blob count budget exceeded")
	}
	if (len(s.NativeRefs) > 0) != (s.GitBundle != "") {
		return nil, fmt.Errorf("native refs require Git material")
	}
	if s.GitBundle != "" {
		if _, ok := blobs[s.GitBundle]; !ok {
			return nil, fmt.Errorf("missing Git material")
		}
	}
	refs := map[string]bool{}
	for _, r := range s.NativeRefs {
		if !ValidRef(r.SourceRef) || !ValidOID(r.OID) || NativeKind(r.SourceRef) != r.Kind || r.Kind == "" || refs[r.SourceRef] {
			return nil, fmt.Errorf("invalid/duplicate native ref")
		}
		refs[r.SourceRef] = true
	}
	keys := map[string]bool{}
	for _, r := range s.Records {
		if !validKey(r.Key) || keys[r.Key.String()] {
			return nil, fmt.Errorf("invalid/duplicate record key")
		}
		keys[r.Key.String()] = true
	}
	for i, r := range s.Records {
		if !text(r.Selector, 4096, false) || !text(r.Revision, 4096, true) || !text(r.Author, 4096, false) || !text(r.SourceURL, 4096, false) || !validURL(r.SourceURL) {
			return nil, fmt.Errorf("invalid record attribution")
		}
		for _, ts := range []string{r.CreatedAt, r.UpdatedAt} {
			if ts != "" {
				if _, e := time.Parse(time.RFC3339Nano, ts); e != nil {
					return nil, fmt.Errorf("invalid source time")
				}
			}
		}
		c, e := CanonicalContent(r.Key.Kind, r.Content)
		if e != nil {
			return nil, e
		}
		var schema struct {
			Schema string `json:"schema"`
		}
		_ = json.Unmarshal(c, &schema)
		if schema.Schema != r.Schema {
			return nil, fmt.Errorf("record content/schema mismatch")
		}
		r.Content = c
		r.Links = append([]Link{}, r.Links...)
		r.Blobs = append([]string{}, r.Blobs...)
		parent := 0
		links := map[string]bool{}
		for _, link := range r.Links {
			if !text(link.Relation, 128, true) || !validKey(link.Target) || !text(link.Revision, 4096, false) {
				return nil, fmt.Errorf("invalid link")
			}
			b, _ := json.Marshal(link)
			if links[string(b)] {
				return nil, fmt.Errorf("duplicate link")
			}
			links[string(b)] = true
			if !keys[link.Target.String()] {
				return nil, fmt.Errorf("missing linked record dependency")
			}
			if link.Relation == "issue" || link.Relation == "work_item" {
				parent++
				if link.Target.Kind != link.Relation {
					return nil, fmt.Errorf("wrong parent kind")
				}
			}
		}
		if r.Key.Kind == "comment" && parent != 1 {
			return nil, fmt.Errorf("comment requires exactly one issue/work_item parent")
		}
		seen := map[string]bool{}
		for _, d := range r.Blobs {
			if _, ok := blobs[d]; !ok || d == s.GitBundle || seen[d] {
				return nil, fmt.Errorf("missing/duplicate attachment")
			}
			seen[d] = true
		}
		if schema.Schema == "hn.migration.preserved/1" {
			var p PreservedContent
			_ = json.Unmarshal(c, &p)
			if !seen[p.SourceBlob] {
				return nil, fmt.Errorf("preserved source bytes missing")
			}
		}
		sort.Slice(r.Links, func(i, j int) bool {
			a, _ := json.Marshal(r.Links[i])
			b, _ := json.Marshal(r.Links[j])
			return string(a) < string(b)
		})
		sort.Strings(r.Blobs)
		s.Records[i] = r
	}
	findingIDs := map[string]bool{}
	for _, f := range s.Findings {
		if !text(f.ID, 1024, true) || findingIDs[f.ID] || !text(f.Path, 4096, false) || !text(f.Message, 65536, true) {
			return nil, fmt.Errorf("invalid finding")
		}
		findingIDs[f.ID] = true
		switch f.Status {
		case "mapped", "preserved", "requires_mapping", "unsupported", "missing_dependency", "identity_conflict":
		default:
			return nil, fmt.Errorf("invalid finding status")
		}
	}
	sort.Slice(s.Records, func(i, j int) bool { return s.Records[i].Key.String() < s.Records[j].Key.String() })
	sort.Slice(s.Blobs, func(i, j int) bool { return s.Blobs[i].Digest < s.Blobs[j].Digest })
	sort.Slice(s.NativeRefs, func(i, j int) bool { return s.NativeRefs[i].SourceRef < s.NativeRefs[j].SourceRef })
	sort.Slice(s.Findings, func(i, j int) bool { return s.Findings[i].ID < s.Findings[j].ID })
	b, e := json.Marshal(s)
	if int64(len(b)) > l.ManifestBytes {
		return nil, fmt.Errorf("manifest budget exceeded")
	}
	return b, e
}

// GitMaterialLimit includes bounded object headers in addition to raw Git bytes.
func GitMaterialLimit(l Limits) int64 {
	const max = int64(^uint64(0) >> 1)
	if l.GitTotalBytes > max-128 || int64(l.GitObjects) > (max-l.GitTotalBytes-128)/256 {
		return max
	}
	return l.GitTotalBytes + int64(l.GitObjects)*256 + 128
}
