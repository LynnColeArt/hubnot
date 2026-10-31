package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"hubnot/internal/safepath"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	artifactExchangeWire    = 16777216
	artifactExchangeDecoded = 33554432
	artifactExchangeObjects = 4096
	artifactExchangeRecords = 2048
	artifactExchangeDepth   = 256
	artifactExchangeStage   = 30 * time.Second
)
const artifactExchangeProtocol = "hn-artifact-exchange/1"

var artifactExchangeModes = map[string]bool{"accepted": true, "opaque": true}
var artifactExchangeObjectTypes = map[string]bool{"blob": true, "tree": true, "commit": true}

type artifactExchangeObject struct {
	OID  string `json:"oid"`
	Type string `json:"type"`
	Data string `json:"data"`
	body []byte
}
type artifactExchange struct {
	Protocol string                   `json:"protocol"`
	Mode     string                   `json:"mode"`
	Refs     []artifactSource         `json:"refs"`
	Objects  []artifactExchangeObject `json:"objects"`
}
type artifactExchangeBudget struct{ objects, decoded int }

func (b *artifactExchangeBudget) charge(ctx context.Context, n int) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact exchange cancelled")
	}
	if b.objects >= artifactExchangeObjects {
		return artifactFailure("resource_limit", "exchange object count exceeds 4096")
	}
	if n < 0 || n > artifactExchangeDecoded-b.decoded {
		return artifactFailure("resource_limit", "exchange decoded aggregate exceeds 33554432 bytes")
	}
	b.objects++
	b.decoded += n
	return nil
}

// A raw Git commit body is not a verified signed artifact. Charge this guard
// only from the shared reader/closure results, never from opaque object types.
func artifactExchangeCheckRecords(ctx context.Context, count int) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact exchange cancelled")
	}
	if count > artifactExchangeRecords {
		return artifactFailure("resource_limit", "exchange signed record count exceeds 2048")
	}
	return nil
}
func artifactExchangeHash(kind string, body []byte, oidLength int) string {
	var h hash.Hash = sha1.New()
	if oidLength == 64 {
		h = sha256.New()
	}
	fmt.Fprintf(h, "%s %d\x00", kind, len(body))
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
func artifactOpaqueID(o artifactExchangeObject) string {
	digest := sha256.New()
	_, _ = io.WriteString(digest, o.Type+"\x00")
	_, _ = digest.Write(o.body)
	return "sha256:" + hex.EncodeToString(digest.Sum(nil))
}
func artifactExchangeString(raw json.RawMessage) (string, error) {
	var s string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", artifactFailure("invalid_input", "exchange field requires a string")
	}
	// Every exchange string belongs to an ASCII grammar. This also rejects JSON's
	// lossy replacement of invalid surrogate escapes instead of accepting aliases.
	for _, c := range s {
		if c > 127 {
			return "", artifactFailure("invalid_input", "exchange string must be ASCII")
		}
	}
	return s, nil
}
func artifactExchangeShape(raw []byte, keys ...string) (map[string]json.RawMessage, error) {
	v, e := artifactRawObject(raw, keys...)
	if e != nil {
		return nil, artifactFailure("invalid_input", "invalid exchange object fields or duplicate keys")
	}
	return v, nil
}
func artifactExchangeArray(ctx context.Context, raw []byte, limit int, visit func(json.RawMessage) error) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('[') {
		return artifactFailure("invalid_input", "exchange requires non-null arrays")
	}
	count := 0
	for d.More() {
		if ctx.Err() != nil {
			return artifactFailure("cancelled", "artifact exchange cancelled")
		}
		if count >= limit {
			return artifactFailure("resource_limit", "exchange array count exceeds limit")
		}
		var item json.RawMessage
		if d.Decode(&item) != nil {
			return artifactFailure("invalid_input", "invalid exchange array")
		}
		if e = visit(item); e != nil {
			return e
		}
		count++
	}
	if tok, e = d.Token(); e != nil || tok != json.Delim(']') {
		return artifactFailure("invalid_input", "invalid exchange array")
	}
	if _, e = d.Token(); e != io.EOF {
		return artifactFailure("invalid_input", "trailing exchange array bytes")
	}
	return nil
}
func decodeArtifactExchange(ctx context.Context, raw []byte, oidLength int) (*artifactExchange, error) {
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact exchange cancelled")
	}
	if len(raw) > artifactExchangeWire {
		return nil, artifactFailure("resource_limit", "exchange wire exceeds 16777216 bytes")
	}
	if !utf8.Valid(raw) {
		return nil, artifactFailure("invalid_input", "exchange is not UTF-8")
	}
	v, e := artifactExchangeShape(raw, "protocol", "mode", "refs", "objects")
	if e != nil {
		return nil, e
	}
	protocol, e := artifactExchangeString(v["protocol"])
	if e != nil {
		return nil, e
	}
	mode, e := artifactExchangeString(v["mode"])
	if e != nil {
		return nil, e
	}
	if protocol != artifactExchangeProtocol || !artifactExchangeModes[mode] {
		return nil, artifactFailure("invalid_input", "unsupported exchange wrapper")
	}
	b := &artifactExchange{Protocol: protocol, Mode: mode, Refs: []artifactSource{}, Objects: []artifactExchangeObject{}}
	names := map[string]bool{}
	e = artifactExchangeArray(ctx, v["refs"], artifactReadEntries, func(raw json.RawMessage) error {
		r, e := artifactExchangeShape(raw, "ref", "oid")
		if e != nil {
			return e
		}
		ref, e := artifactExchangeString(r["ref"])
		if e != nil {
			return e
		}
		oid, e := artifactExchangeString(r["oid"])
		if e != nil {
			return e
		}
		if _, ok := parseArtifactRef(ref); !ok || !validGitOID(oid) || len(oid) != oidLength || oid != strings.ToLower(oid) || names[ref] {
			return artifactFailure("invalid_input", "invalid, duplicate or object-format mismatched exchange ref")
		}
		names[ref] = true
		b.Refs = append(b.Refs, artifactSource{ref, oid})
		return nil
	})
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	budget := artifactExchangeBudget{}
	e = artifactExchangeArray(ctx, v["objects"], artifactExchangeObjects+1, func(raw json.RawMessage) error {
		v, e := artifactExchangeShape(raw, "oid", "type", "data")
		if e != nil {
			return e
		}
		o := artifactExchangeObject{}
		if o.OID, e = artifactExchangeString(v["oid"]); e != nil {
			return e
		}
		if o.Type, e = artifactExchangeString(v["type"]); e != nil {
			return e
		}
		if o.Data, e = artifactExchangeString(v["data"]); e != nil {
			return e
		}
		if !validGitOID(o.OID) || len(o.OID) != oidLength || o.OID != strings.ToLower(o.OID) || seen[o.OID] || !artifactExchangeObjectTypes[o.Type] {
			return artifactFailure("invalid_input", "invalid or duplicate exchange Git object")
		}
		// Charge the decoded size before allocating its buffer. Strict unpadded
		// Base64 plus re-encoding disallows padding, ignored newlines and spare bits.
		n := base64.RawStdEncoding.DecodedLen(len(o.Data))
		if e = budget.charge(ctx, n); e != nil {
			return e
		}
		o.body, e = base64.RawStdEncoding.Strict().DecodeString(o.Data)
		if e != nil || base64.RawStdEncoding.EncodeToString(o.body) != o.Data {
			return artifactFailure("invalid_input", "exchange data requires canonical unpadded Base64")
		}
		if artifactExchangeHash(o.Type, o.body, oidLength) != o.OID {
			return artifactFailure("verification_failure", "exchange Git object hash mismatch")
		}
		seen[o.OID] = true
		b.Objects = append(b.Objects, o)
		return nil
	})
	if e != nil {
		return nil, e
	}
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact exchange validation cancelled")
	}
	sort.Slice(b.Refs, func(i, j int) bool { return b.Refs[i].Ref < b.Refs[j].Ref })
	sort.Slice(b.Objects, func(i, j int) bool { return b.Objects[i].OID < b.Objects[j].OID })
	return b, nil
}
func artifactExchangeFormat(ctx context.Context, dir string) (int, error) {
	b, e := projectGit(ctx, dir, nil, 32, "rev-parse", "--show-object-format")
	if e != nil {
		return 0, artifactExchangeGitError(ctx, "repository_error", "cannot identify exchange repository")
	}
	switch strings.TrimSpace(string(b)) {
	case "sha1":
		return 40, nil
	case "sha256":
		return 64, nil
	}
	return 0, artifactFailure("repository_error", "unsupported exchange object format")
}
func artifactExchangeGitError(ctx context.Context, code, message string) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact exchange cancelled")
	}
	return artifactFailure(code, message)
}
func readArtifactExchange(ctx context.Context, path string) ([]byte, error) {
	var f *os.File
	if path == "-" {
		f = os.Stdin
	} else {
		absolute, e := safepath.Canonical(path)
		if e != nil || safepath.CheckDir(filepath.Dir(absolute), false) != nil {
			return nil, artifactFailure("invalid_input", "unsafe exchange input path")
		}
		fd, e := syscall.Open(absolute, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if e != nil {
			return nil, artifactFailure("invalid_input", "cannot open exchange input regular file")
		}
		f = os.NewFile(uintptr(fd), "exchange input")
		defer f.Close()
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() {
			return nil, artifactFailure("invalid_input", "exchange input must be a regular file")
		}
		if info.Size() > artifactExchangeWire {
			return nil, artifactFailure("resource_limit", "exchange wire exceeds 16777216 bytes")
		}
	}
	stop := context.AfterFunc(ctx, func() { _ = f.Close() })
	defer stop()
	raw, e := io.ReadAll(io.LimitReader(f, artifactExchangeWire+1))
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact exchange input cancelled")
	}
	if e != nil {
		return nil, artifactFailure("invalid_input", "cannot read exchange input")
	}
	if len(raw) > artifactExchangeWire {
		return nil, artifactFailure("resource_limit", "exchange wire exceeds 16777216 bytes")
	}
	return raw, nil
}
func artifactExchangeBytes(b *artifactExchange) ([]byte, error) {
	raw, e := json.Marshal(b)
	if e != nil {
		return nil, artifactFailure("invalid_input", "cannot encode exchange")
	}
	if len(raw) > artifactExchangeWire {
		return nil, artifactFailure("resource_limit", "exchange wire exceeds 16777216 bytes")
	}
	return raw, nil
}
func artifactExchangeWrite(ctx context.Context, path string, raw []byte) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact exchange copy cancelled")
	}
	absolute, e := safepath.Canonical(path)
	if e != nil || path == "-" || safepath.CheckDir(filepath.Dir(absolute), false) != nil {
		return artifactFailure("invalid_input", "unsafe exchange output path")
	}
	if info, e := os.Lstat(absolute); e == nil {
		if !info.Mode().IsRegular() {
			return artifactFailure("invalid_input", "exchange output must be a regular file")
		}
	} else if !os.IsNotExist(e) {
		return artifactFailure("invalid_input", "cannot inspect exchange output")
	}
	f, e := os.CreateTemp(filepath.Dir(absolute), ".hn-artifact-exchange-")
	if e != nil {
		return artifactFailure("repository_error", "cannot create exchange output")
	}
	temp := f.Name()
	defer os.Remove(temp)
	defer f.Close()
	for offset := 0; offset < len(raw); {
		if ctx.Err() != nil {
			return artifactFailure("cancelled", "artifact exchange copy cancelled")
		}
		end := offset + 65536
		if end > len(raw) {
			end = len(raw)
		}
		n, e := f.Write(raw[offset:end])
		if e != nil || n != end-offset {
			return artifactFailure("repository_error", "cannot write exchange output")
		}
		offset = end
	}
	if e = f.Sync(); e != nil {
		return artifactFailure("repository_error", "cannot persist exchange output")
	}
	if e = f.Close(); e != nil {
		return artifactFailure("repository_error", "cannot close exchange output")
	}
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact exchange copy cancelled")
	}
	if e = os.Rename(temp, absolute); e != nil {
		return artifactFailure("repository_error", "cannot publish exchange output")
	}
	// Replacement has happened. A directory-sync error is an uncertain result,
	// never a pre-effect refusal or a claim that the old index survived.
	if e = syncPrivateDirectory(filepath.Dir(absolute)); e != nil {
		return artifactFailure("uncertain_operation", "exchange output persistence requires inspection")
	}
	return nil
}
func artifactExchangeObjectsFromClosure(ctx context.Context, h *artifactHistory, c *artifactClosure) ([]artifactExchangeObject, error) {
	if e := artifactExchangeCheckRecords(ctx, len(c.Items)); e != nil {
		return nil, e
	}
	reader := artifactObjectReader{ctx: ctx, dir: h.gitDir, seen: map[string]bool{}}
	out := []artifactExchangeObject{}
	budget := artifactExchangeBudget{}
	for _, oid := range c.Objects {
		meta, e := projectGit(ctx, h.gitDir, []byte(oid+"\n"), 256, "cat-file", "--batch-check=%(objecttype)")
		if e != nil {
			return nil, artifactExchangeGitError(ctx, "verification_failure", "cannot inspect selected exchange object")
		}
		kind := strings.TrimSpace(string(meta))
		if !artifactExchangeObjectTypes[kind] {
			return nil, artifactFailure("verification_failure", "unsupported exchange object type")
		}
		body, e := reader.object(oid, kind, artifactReadObjectBytes)
		if e != nil {
			return nil, e
		}
		if e = budget.charge(ctx, len(body)); e != nil {
			return nil, e
		}
		out = append(out, artifactExchangeObject{oid, kind, base64.RawStdEncoding.EncodeToString(body), body})
	}
	return out, nil
}

// The stream-depth budget is enforced against the input table before the ordinary
// shared history reader can walk its much larger read/catalog budget.
func artifactExchangeAncestry(ctx context.Context, b *artifactExchange) error {
	objects := map[string]artifactExchangeObject{}
	for _, o := range b.Objects {
		objects[o.OID] = o
	}
	for _, ref := range b.Refs {
		seen := map[string]bool{}
		oid := ref.OID
		for oid != "" {
			if ctx.Err() != nil {
				return artifactFailure("cancelled", "artifact exchange cancelled")
			}
			if seen[oid] {
				return artifactFailure("invalid_history", "cyclic exchange ancestry")
			}
			if len(seen) >= artifactExchangeDepth {
				return artifactFailure("resource_limit", "exchange ancestry depth exceeds 256")
			}
			seen[oid] = true
			o, ok := objects[oid]
			if !ok || o.Type != "commit" {
				return artifactFailure("verification_failure", "exchange ancestry object missing or wrong type")
			}
			headers, _, ok := strings.Cut(string(o.body), "\n\n")
			if !ok {
				return artifactFailure("invalid_history", "invalid exchange commit")
			}
			oid = ""
			for _, line := range strings.Split(headers, "\n") {
				if strings.HasPrefix(line, "parent ") {
					if oid != "" {
						return artifactFailure("actor_stream_conflict", "exchange stream has multiple Git parents")
					}
					oid = strings.TrimPrefix(line, "parent ")
				}
			}
		}
	}
	return nil
}
func artifactExchangeCopy(ctx context.Context, dir string, objects []artifactExchangeObject) error {
	budget := artifactExchangeBudget{}
	for _, o := range objects {
		if e := budget.charge(ctx, len(o.body)); e != nil {
			return e
		}
		if !artifactExchangeObjectTypes[o.Type] || !validGitOID(o.OID) || o.OID != artifactExchangeHash(o.Type, o.body, len(o.OID)) {
			return artifactFailure("verification_failure", "invalid raw exchange object before copy")
		}
		// Commit metadata is unsigned transport data. Git's ordinary object
		// writer imposes extra email syntax absent from the frozen contract.
		// Store exact hash-checked bodies; the shared quarantine reader still
		// validates complete signed/schema/lineage rules before promotion.
		written, e := projectGit(ctx, dir, o.body, 128, "hash-object", "--literally", "-w", "--stdin", "-t", o.Type)
		if e != nil || strings.TrimSpace(string(written)) != o.OID {
			return artifactExchangeGitError(ctx, "verification_failure", "cannot copy verified exchange object")
		}
	}
	return nil
}
func artifactExchangeSetRefs(ctx context.Context, dir string, refs []artifactSource, old map[string]string) error {
	var input strings.Builder
	input.WriteString("start\n")
	for _, ref := range refs {
		expected := old[ref.Ref]
		if expected == "" {
			expected = strings.Repeat("0", len(ref.OID))
		}
		fmt.Fprintf(&input, "update %s %s %s\n", ref.Ref, ref.OID, expected)
	}
	input.WriteString("prepare\ncommit\n")
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact exchange cancelled before ref transaction")
	}
	if _, e := projectGit(ctx, dir, []byte(input.String()), 65536, "update-ref", "--stdin"); e != nil {
		if ctx.Err() != nil {
			return artifactFailure("uncertain_operation", "exchange ref transaction outcome requires inspection")
		}
		// The joined process may have committed before losing its acknowledgement.
		// Settle every selected target, never infer absence from an exit status.
		observed, settlementError := artifactSources(ctx, dir)
		if settlementError != nil {
			return artifactFailure("uncertain_operation", "exchange ref transaction witnesses unavailable")
		}
		actual := map[string]string{}
		for _, source := range observed {
			actual[source.Ref] = source.OID
		}
		allProposed := len(refs) > 0
		changedProposed := false
		for _, source := range refs {
			if actual[source.Ref] != source.OID {
				allProposed = false
			}
			if source.OID != old[source.Ref] && actual[source.Ref] == source.OID {
				changedProposed = true
			}
		}
		if allProposed || changedProposed {
			return artifactFailure("uncertain_operation", "exchange refs may have committed before acknowledgement was lost")
		}
		return artifactFailure("concurrent_update", "exchange accepted refs changed before atomic promotion")
	}
	return nil
}
func validateArtifactExchange(ctx context.Context, b *artifactExchange, format int) (*artifactHistory, []string, func(), error) {
	if len(b.Refs) == 0 || len(b.Objects) == 0 {
		return nil, nil, nil, artifactFailure("invalid_input", "accepted exchange requires explicit refs and objects")
	}
	if e := artifactExchangeAncestry(ctx, b); e != nil {
		return nil, nil, nil, e
	}
	dir, e := safepath.MkdirTemp("hn-artifact-quarantine-")
	if e != nil {
		return nil, nil, nil, artifactFailure("repository_error", "cannot create artifact quarantine")
	}
	close := func() { _ = os.RemoveAll(dir) }
	failed := func(e error) (*artifactHistory, []string, func(), error) { close(); return nil, nil, nil, e }
	name := "sha1"
	if format == 64 {
		name = "sha256"
	}
	if _, e = projectGit(ctx, "", nil, 4096, "init", "--bare", "--quiet", "--object-format="+name, dir); e != nil {
		return failed(artifactExchangeGitError(ctx, "repository_error", "cannot initialize artifact quarantine"))
	}
	if e = artifactExchangeCopy(ctx, dir, b.Objects); e != nil {
		return failed(e)
	}
	if e = artifactExchangeSetRefs(ctx, dir, b.Refs, map[string]string{}); e != nil {
		if ctx.Err() != nil {
			return failed(artifactFailure("cancelled", "artifact quarantine validation cancelled"))
		}
		// These witnesses exist only in the disposable quarantine. Even an
		// acknowledgement lost here cannot have published destination refs.
		return failed(artifactFailure("verification_failure", "cannot establish quarantine source witnesses"))
	}
	h, e := loadArtifactHistoryFromSources(ctx, dir, b.Refs)
	if e != nil {
		return failed(e)
	}
	if e = artifactExchangeCheckRecords(ctx, len(h.ByID)); e != nil {
		return failed(e)
	}
	roots := []string{}
	for id := range h.ByID {
		roots = append(roots, id)
	}
	c, e := selectArtifactClosure(ctx, h, roots)
	if e != nil {
		return failed(e)
	}
	table := map[string]bool{}
	for _, object := range b.Objects {
		table[object.OID] = true
	}
	for _, oid := range c.Objects {
		if !table[oid] {
			return failed(artifactFailure("verification_failure", "exchange object table lacks complete selected closure"))
		}
	}
	// Unreachable table bodies were bounded and hash-checked, but they are not
	// roots, accepted data or an implicit opaque-preservation request.
	return h, c.Objects, close, nil
}
func exportArtifactExchange(ctx context.Context, roots []string) (*artifactExchange, int, error) {
	validation, cancel := context.WithTimeout(ctx, artifactExchangeStage)
	defer cancel()
	h, e := loadArtifactHistory(validation)
	if e != nil {
		return nil, 0, e
	}
	c, e := selectArtifactClosure(validation, h, roots)
	if e != nil {
		return nil, 0, e
	}
	b := &artifactExchange{Protocol: artifactExchangeProtocol, Mode: "accepted", Refs: c.Sources}
	b.Objects, e = artifactExchangeObjectsFromClosure(validation, h, c)
	if e != nil {
		return nil, 0, e
	}
	if e = artifactExchangeAncestry(validation, b); e != nil {
		return nil, 0, e
	}
	after, e := artifactSources(validation, h.gitDir)
	if e != nil {
		return nil, 0, e
	}
	if !reflect.DeepEqual(h.Sources, after) {
		return nil, 0, artifactFailure("concurrent_update", "artifact source witnesses changed during export")
	}
	if validation.Err() != nil {
		return nil, 0, artifactFailure("cancelled", "artifact exchange validation cancelled")
	}
	return b, len(c.IDs), nil
}
func importArtifactExchange(ctx context.Context, b *artifactExchange) (map[string]any, error) {
	if e := artifactMutationPolicy(); e != nil {
		return nil, e
	}
	validation, cancel := context.WithTimeout(ctx, artifactExchangeStage)
	defer cancel()
	dir, e := contributionGitDirectory()
	if e != nil {
		return nil, artifactFailure("repository_error", "cannot identify exchange destination")
	}
	format, e := artifactExchangeFormat(validation, dir)
	if e != nil {
		return nil, e
	}
	incoming, selectedOIDs, close, e := validateArtifactExchange(validation, b, format)
	if e != nil {
		return nil, e
	}
	defer close()
	current, e := loadArtifactHistory(validation)
	if e != nil {
		return nil, e
	}
	for actor, chain := range incoming.Chains {
		prior := current.Chains[actor]
		n := len(chain)
		if len(prior) < n {
			n = len(prior)
		}
		for i := 0; i < n; i++ {
			if prior[i].ID != chain[i].ID || prior[i].commit != chain[i].commit {
				return nil, artifactFailure("actor_stream_conflict", "import diverges from accepted actor stream")
			}
		}
	}
	promoted := []artifactSource{}
	old := map[string]string{}
	for _, source := range current.Sources {
		old[source.Ref] = source.OID
	}
	actors := []string{}
	for actor := range incoming.Chains {
		actors = append(actors, actor)
	}
	sort.Strings(actors)
	for _, actor := range actors {
		chain := incoming.Chains[actor]
		tip := chain[len(chain)-1].commit
		ref := "refs/hn/remotes/imported/artifacts/" + actor + "/" + strings.TrimPrefix(artifactStream(actor), "sha256:")
		if previous := old[ref]; previous != "" {
			found := false
			for _, s := range chain {
				if s.commit == previous {
					found = true
				}
			}
			if !found {
				return nil, artifactFailure("concurrent_update", "import would rewind accepted file ref")
			}
		}
		promoted = append(promoted, artifactSource{ref, tip})
	}
	sort.Slice(promoted, func(i, j int) bool { return promoted[i].Ref < promoted[j].Ref })
	// A new copy stage gets its own30s ceiling, while retaining caller cancellation.
	copyCtx, copyCancel := context.WithTimeout(ctx, artifactExchangeStage)
	defer copyCancel()
	after, e := artifactSources(copyCtx, dir)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(current.Sources, after) {
		return nil, artifactFailure("concurrent_update", "destination sources changed before exchange copy")
	}
	selected := map[string]bool{}
	for _, oid := range selectedOIDs {
		selected[oid] = true
	}
	selectedObjects := []artifactExchangeObject{}
	for _, object := range b.Objects {
		if selected[object.OID] {
			selectedObjects = append(selectedObjects, object)
		}
	}
	if e = artifactExchangeCopy(copyCtx, dir, selectedObjects); e != nil {
		return nil, e
	}
	if e = artifactExchangeSetRefs(copyCtx, dir, promoted, old); e != nil {
		return nil, e
	}
	// Only the ordinary destination loader may produce an accepted public Cut.
	accepted, e := loadArtifactHistory(copyCtx)
	if e != nil {
		return nil, artifactFailure("uncertain_operation", "promoted exchange requires destination inspection")
	}
	ids := []string{}
	for id := range incoming.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	diagnostics := []artifactDiagnostic{}
	catalog, e := buildArtifactCatalog(copyCtx, accepted)
	if e != nil {
		return nil, artifactFailure("uncertain_operation", "promoted exchange requires destination inspection")
	}
	diagnostics, e = catalog.diagnostics(copyCtx, ids)
	if e != nil {
		return nil, artifactFailure("uncertain_operation", "promoted exchange requires destination inspection")
	}
	return map[string]any{"mode": "accepted", "accepted_refs": promoted, "imported_ids": ids, "opaque_ids": []string{}, "diagnostics": diagnostics, "source_cut": accepted.Cut}, nil
}

// Explicit opaque data is bounded in aggregate, serialized under a private lock,
// and resolved ONLY from this index. The normal accepted ODB is never a fallback.
func artifactOpaquePath(create bool) (string, error) {
	dir, e := contributionGitDirectory()
	if e != nil {
		return "", artifactFailure("repository_error", "cannot identify opaque index repository")
	}
	missing := false
	for _, part := range []string{"hn", "artifact-opaque"} {
		dir = filepath.Join(dir, part)
		if create {
			if ensurePrivateDirectory(dir) != nil {
				return "", artifactFailure("repository_error", "unsafe opaque index directory")
			}
		} else if !missing {
			info, e := os.Lstat(dir)
			if os.IsNotExist(e) {
				missing = true
			} else if e != nil || validatePrivateDirectory(dir, info) != nil {
				return "", artifactFailure("repository_error", "unsafe opaque index directory")
			}
		}
	}
	return filepath.Join(dir, "public.json"), nil
}
func loadArtifactOpaqueIndex(ctx context.Context) (*artifactExchange, error) {
	path, e := artifactOpaquePath(false)
	if e != nil {
		return nil, e
	}
	if info, statErr := os.Lstat(path); os.IsNotExist(statErr) {
		return &artifactExchange{Protocol: artifactExchangeProtocol, Mode: "opaque", Refs: []artifactSource{}, Objects: []artifactExchangeObject{}}, nil
	} else if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, artifactFailure("invalid_input", "unsafe opaque public index file")
	}
	raw, e := readArtifactExchange(ctx, path)
	if e != nil {
		return nil, e
	}
	dir, e := contributionGitDirectory()
	if e != nil {
		return nil, artifactFailure("repository_error", "cannot identify opaque repository")
	}
	format, e := artifactExchangeFormat(ctx, dir)
	if e != nil {
		return nil, e
	}
	b, e := decodeArtifactExchange(ctx, raw, format)
	if e != nil {
		return nil, e
	}
	if b.Mode != "opaque" || len(b.Refs) != 0 {
		return nil, artifactFailure("verification_failure", "invalid public opaque index")
	}
	return b, nil
}
func preserveArtifactExchange(ctx context.Context, b *artifactExchange) (map[string]any, error) {
	copyCtx, cancel := context.WithTimeout(ctx, artifactExchangeStage)
	defer cancel()
	h, e := loadArtifactHistory(copyCtx)
	if e != nil {
		return nil, e
	}
	path, e := artifactOpaquePath(true)
	if e != nil {
		return nil, e
	}
	fd, e := syscall.Open(path+".lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return nil, artifactFailure("concurrent_update", "opaque index is busy")
	}
	f := os.NewFile(uintptr(fd), "opaque index lock")
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, artifactFailure("repository_error", "unsafe opaque index lock")
	}
	if syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return nil, artifactFailure("concurrent_update", "opaque index is busy")
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	prior, e := loadArtifactOpaqueIndex(copyCtx)
	if e != nil {
		return nil, e
	}
	objects := map[string]artifactExchangeObject{}
	for _, o := range prior.Objects {
		objects[o.OID] = o
	}
	ids := []string{}
	for _, o := range b.Objects {
		if old, ok := objects[o.OID]; ok && (old.Type != o.Type || !bytes.Equal(old.body, o.body)) {
			return nil, artifactFailure("verification_failure", "contradictory opaque Git object identity")
		}
		objects[o.OID] = o
		ids = append(ids, artifactOpaqueID(o))
	}
	sort.Strings(ids)
	merged := &artifactExchange{Protocol: artifactExchangeProtocol, Mode: "opaque", Refs: []artifactSource{}, Objects: []artifactExchangeObject{}}
	budget := artifactExchangeBudget{}
	for _, o := range objects {
		if e = budget.charge(copyCtx, len(o.body)); e != nil {
			return nil, e
		}
		merged.Objects = append(merged.Objects, o)
	}
	sort.Slice(merged.Objects, func(i, j int) bool { return merged.Objects[i].OID < merged.Objects[j].OID })
	raw, e := artifactExchangeBytes(merged)
	if e != nil {
		return nil, e
	}
	priorRaw, e := artifactExchangeBytes(prior)
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(priorRaw, raw) {
		if e = artifactExchangeWrite(copyCtx, path, raw); e != nil {
			return nil, e
		}
	}
	return map[string]any{"mode": "opaque", "accepted_refs": []artifactSource{}, "imported_ids": []string{}, "opaque_ids": ids, "diagnostics": []artifactDiagnostic{{Code: "opaque_unverified", Message: "public raw objects preserved without accepted validation"}}, "source_cut": h.Cut}, nil
}
func readArtifactOpaque(ctx context.Context, id string) (map[string]any, error) {
	b, e := loadArtifactOpaqueIndex(ctx)
	if e != nil {
		return nil, e
	}
	for _, o := range b.Objects {
		if artifactOpaqueID(o) == id {
			return map[string]any{"id": nil, "opaque_id": id, "envelope": nil, "signature": nil, "validation": "opaque", "raw": o, "diagnostics": []artifactDiagnostic{{Code: "opaque_unverified", Message: "public raw object has no accepted semantic validity"}}}, nil
		}
	}
	return nil, artifactFailure("not_found", "opaque public object not found")
}
func exportArtifactOpaque(ctx context.Context, ids []string) (*artifactExchange, error) {
	prior, e := loadArtifactOpaqueIndex(ctx)
	if e != nil {
		return nil, e
	}
	byID := map[string]artifactExchangeObject{}
	for _, o := range prior.Objects {
		byID[artifactOpaqueID(o)] = o
	}
	b := &artifactExchange{Protocol: artifactExchangeProtocol, Mode: "opaque", Refs: []artifactSource{}, Objects: []artifactExchangeObject{}}
	for _, id := range ids {
		o, ok := byID[id]
		if !ok {
			return nil, artifactFailure("not_found", "opaque public object not found")
		}
		b.Objects = append(b.Objects, o)
	}
	sort.Slice(b.Objects, func(i, j int) bool { return b.Objects[i].OID < b.Objects[j].OID })
	return b, nil
}
