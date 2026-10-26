package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hubnot/internal/migration"
)

const projectGitSchema = "hn.git-objects/1\n"

// ProjectSnapshot is a private, immutable-by-digest local export. Close removes
// its spool; it never captures the filesystem .git or worktree.
type ProjectSnapshot struct {
	Snapshot migration.Snapshot
	root     string
	digest   string
}

func (p *ProjectSnapshot) Close() error { return os.RemoveAll(p.root) }
func (p *ProjectSnapshot) Open(ctx context.Context, digest string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if digest != p.digest || digest == "" {
		return nil, fmt.Errorf("undeclared project blob")
	}
	return os.Open(filepath.Join(p.root, "git"))
}

// ExportProjectSnapshot reads only explicit code/actor/proposal/memory refs and
// their required code-object closure. Missing signed fact suppliers are refused,
// never obtained by walking unrelated refs or ambient accepted histories.
func ExportProjectSnapshot(ctx context.Context, gitDir string, refs []string, l migration.Limits) (result *ProjectSnapshot, err error) {
	if e := projectSafeEnvironment(); e != nil {
		return nil, e
	}
	if e := l.Validate(); e != nil {
		return nil, e
	}
	if e := projectReadOnlySource(ctx, gitDir); e != nil {
		return nil, e
	}
	if len(refs) == 0 || len(refs) > l.Refs {
		return nil, fmt.Errorf("explicit positive bounded ref selection required")
	}
	s := migration.Snapshot{Schema: "hn.project-snapshot/1", Source: migration.Locator{Kind: "local"}, Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}
	roots := []string{}
	seen := map[string]bool{}
	for _, ref := range refs {
		kind := migration.NativeKind(ref)
		if !migration.ValidRef(ref) || kind == "" || seen[ref] {
			return nil, fmt.Errorf("unsupported/duplicate explicit ref")
		}
		seen[ref] = true
		oid, e := projectGit(ctx, gitDir, nil, 256, "show-ref", "--verify", "--hash", ref)
		if e != nil {
			return nil, e
		}
		id := strings.TrimSpace(string(oid))
		if !migration.ValidOID(id) {
			return nil, fmt.Errorf("invalid selected OID")
		}
		s.NativeRefs = append(s.NativeRefs, migration.NativeRef{SourceRef: ref, OID: id, Kind: kind})
		roots = append(roots, id)
	}
	// Native readers can unpack audit attachments. Inspect the selected ordinary
	// closure in a disposable object DB so even source objects remain unchanged.
	selected, e := projectObjectIDs(ctx, gitDir, roots, l)
	if e != nil {
		return nil, e
	}
	inspection, e := os.MkdirTemp("", "hn-project-inspection-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(inspection)
	format := "sha1"
	if len(roots[0]) == 64 {
		format = "sha256"
	}
	if _, e = projectGit(ctx, "", nil, 4096, "init", "--bare", "--quiet", "--object-format="+format, inspection); e != nil {
		return nil, e
	}
	if e = projectCopyObjectIDs(ctx, gitDir, inspection, selected, l); e != nil {
		return nil, e
	}
	if _, e = projectGit(ctx, inspection, nil, 1<<20, "fsck", "--strict", "--full", "--no-reflogs", "--no-dangling"); e != nil {
		return nil, e
	}
	if _, e = projectAuditObjectUnion(ctx, inspection, s.NativeRefs, selected, l); e != nil {
		return nil, e
	}
	dependencies, e := projectDependencyRoots(inspection, s.NativeRefs)
	if e != nil {
		return nil, e
	}
	roots = append(roots, dependencies...)
	ids, e := projectObjectIDs(ctx, gitDir, roots, l)
	if e != nil {
		return nil, e
	}
	root, e := os.MkdirTemp("", "hn-project-snapshot-")
	if e != nil {
		return nil, e
	}
	p := &ProjectSnapshot{root: root}
	defer func() {
		if err != nil {
			_ = p.Close()
		}
	}()
	f, e := os.OpenFile(filepath.Join(root, "git"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	h := sha256.New()
	w := io.MultiWriter(f, h)
	if _, e = io.WriteString(w, projectGitSchema); e != nil {
		return nil, e
	}
	for _, id := range ids {
		metadata, e := projectGit(ctx, gitDir, []byte(id+"\n"), 256, "cat-file", "--batch-check=%(objecttype) %(objectsize)")
		if e != nil {
			return nil, e
		}
		fields := strings.Fields(string(metadata))
		if len(fields) != 2 {
			return nil, fmt.Errorf("missing Git object")
		}
		n, e := strconv.ParseInt(fields[1], 10, 64)
		if e != nil || n < 0 || n > l.GitObjectBytes {
			return nil, fmt.Errorf("Git object budget exceeded")
		}
		raw, e := projectGit(ctx, gitDir, nil, n, "cat-file", fields[0], id)
		if e != nil || int64(len(raw)) != n {
			return nil, fmt.Errorf("cannot read exact Git object")
		}
		if _, e = fmt.Fprintf(w, "%s %s %d\n", id, fields[0], n); e != nil {
			return nil, e
		}
		if _, e = w.Write(raw); e != nil {
			return nil, e
		}
		if _, e = io.WriteString(w, "\n"); e != nil {
			return nil, e
		}
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	stat, e := os.Stat(filepath.Join(root, "git"))
	if e != nil {
		return nil, e
	}
	p.digest = "sha256:" + hex.EncodeToString(h.Sum(nil))
	s.GitBundle = p.digest
	s.Blobs = append(s.Blobs, migration.Blob{Digest: p.digest, Size: stat.Size(), MediaType: "application/vnd.hn.git-objects", Name: "selected Git object closure"})
	sort.Slice(s.NativeRefs, func(i, j int) bool { return s.NativeRefs[i].SourceRef < s.NativeRefs[j].SourceRef })
	identity, _ := json.Marshal(s.NativeRefs)
	s.SourceID = migration.Digest(identity)
	p.Snapshot = s
	q, e := StageProjectSnapshot(ctx, s, p, l)
	if e != nil {
		return nil, e
	}
	_ = q.Close()
	return p, nil
}

// ProjectQuarantine contains validated objects only. No refs, identities, policy
// files or signing streams are installed in a destination. The caller owns Close.
type ProjectQuarantine struct {
	GitDir    string
	Snapshot  migration.Snapshot
	roots     []string
	objectIDs []string
}

func (q *ProjectQuarantine) Close() error        { return os.RemoveAll(q.GitDir) }
func (q *ProjectQuarantine) ObjectIDs() []string { return append([]string{}, q.objectIDs...) }
func (q *ProjectQuarantine) Roots() []string     { return append([]string{}, q.roots...) }

// StageProjectSnapshot validates digests/budgets and exact object/fact closure in
// an isolated bare repository. Native promotion belongs to the apply engine.
func StageProjectSnapshot(ctx context.Context, s migration.Snapshot, source migration.BlobSource, l migration.Limits) (result *ProjectQuarantine, err error) {
	if e := projectSafeEnvironment(); e != nil {
		return nil, e
	}
	raw, e := migration.CanonicalSnapshot(s, l)
	if e != nil {
		return nil, e
	}
	_ = json.Unmarshal(raw, &s)
	dir, e := os.MkdirTemp("", "hn-project-quarantine-")
	if e != nil {
		return nil, e
	}
	q := &ProjectQuarantine{GitDir: dir, Snapshot: s}
	defer func() {
		if err != nil {
			_ = q.Close()
		}
	}()
	format := "sha1"
	for _, r := range s.NativeRefs {
		if len(r.OID) == 64 {
			format = "sha256"
		}
		q.roots = append(q.roots, r.OID)
	}
	if _, e = projectGit(ctx, "", nil, 4096, "init", "--bare", "--quiet", "--object-format="+format, dir); e != nil {
		return nil, e
	}
	for _, b := range s.Blobs {
		if source == nil {
			return nil, fmt.Errorf("missing blob source")
		}
		r, e := source.Open(ctx, b.Digest)
		if e != nil {
			return nil, e
		}
		h := sha256.New()
		limited := io.LimitReader(r, b.Size+1)
		count := &projectCountReader{r: io.TeeReader(limited, h)}
		if b.Digest == s.GitBundle {
			q.objectIDs, e = projectReadObjects(ctx, dir, count, l)
		} else {
			_, e = io.Copy(io.Discard, count)
		}
		ce := r.Close()
		if e != nil {
			return nil, e
		}
		if ce != nil {
			return nil, ce
		}
		if count.n != b.Size || "sha256:"+hex.EncodeToString(h.Sum(nil)) != b.Digest {
			return nil, fmt.Errorf("project blob size/digest mismatch")
		}
	}
	if len(s.NativeRefs) == 0 {
		return q, nil
	}
	// Check ordinary full history before importing deliberately partial audit
	// snapshots. Full fsck afterwards would require omitted audit ancestry.
	if _, e = projectGit(ctx, dir, nil, 1<<20, "fsck", "--strict", "--full", "--no-reflogs", "--no-dangling"); e != nil {
		return nil, e
	}
	admitted, e := projectAuditObjectUnion(ctx, dir, s.NativeRefs, q.objectIDs, l)
	if e != nil {
		return nil, e
	}
	dependencies, e := projectDependencyRoots(dir, s.NativeRefs)
	if e != nil {
		return nil, e
	}
	q.roots = append(q.roots, dependencies...)
	ids, e := projectObjectIDs(ctx, dir, q.roots, l)
	if e != nil {
		return nil, e
	}
	if len(ids) != len(q.objectIDs) {
		return nil, fmt.Errorf("Git material has unrelated or missing objects")
	}
	for i := range ids {
		if ids[i] != q.objectIDs[i] {
			return nil, fmt.Errorf("Git material does not match selected closure")
		}
	}
	if e = validateProjectNative(dir, s.NativeRefs); e != nil {
		return nil, e
	}
	q.objectIDs = admitted
	return q, nil
}

type projectCountReader struct {
	r io.Reader
	n int64
}

func (r *projectCountReader) Read(p []byte) (int, error) {
	n, e := r.r.Read(p)
	r.n += int64(n)
	return n, e
}
func projectReadObjects(ctx context.Context, dir string, r io.Reader, l migration.Limits) ([]string, error) {
	br := bufio.NewReader(r)
	line, e := projectHeader(br)
	if e != nil || line != projectGitSchema {
		return nil, fmt.Errorf("invalid Git material schema")
	}
	ids := []string{}
	var total int64
	for {
		line, e = projectHeader(br)
		if e == io.EOF && line == "" {
			break
		}
		if e != nil || len(line) > 256 {
			return nil, fmt.Errorf("invalid Git object header")
		}
		f := strings.Fields(line)
		if len(f) != 3 || !migration.ValidOID(f[0]) || len(ids) > 0 && ids[len(ids)-1] >= f[0] {
			return nil, fmt.Errorf("invalid/duplicate/unsorted Git object")
		}
		switch f[1] {
		case "commit", "tree", "blob", "tag":
		default:
			return nil, fmt.Errorf("invalid Git object type")
		}
		n, e := strconv.ParseInt(f[2], 10, 64)
		if e != nil || n < 0 || n > l.GitObjectBytes || n > l.GitTotalBytes-total || len(ids) >= l.GitObjects {
			return nil, fmt.Errorf("Git expansion budget exceeded")
		}
		total += n
		data := make([]byte, n)
		if _, e = io.ReadFull(br, data); e != nil {
			return nil, e
		}
		separator, e := br.ReadByte()
		if e != nil || separator != '\n' {
			return nil, fmt.Errorf("invalid object separator")
		}
		var h hash.Hash = sha1.New()
		if len(f[0]) == 64 {
			h = sha256.New()
		}
		fmt.Fprintf(h, "%s %d\x00", f[1], n)
		_, _ = h.Write(data)
		if hex.EncodeToString(h.Sum(nil)) != f[0] {
			return nil, fmt.Errorf("Git object ID mismatch")
		}
		out, e := projectGit(ctx, dir, data, 256, "hash-object", "-w", "-t", f[1], "--stdin")
		if e != nil || strings.TrimSpace(string(out)) != f[0] {
			return nil, fmt.Errorf("Git object refused")
		}
		ids = append(ids, f[0])
	}
	return ids, nil
}
func projectObjectIDs(ctx context.Context, dir string, roots []string, l migration.Limits) ([]string, error) {
	if len(roots) == 0 {
		return []string{}, nil
	}
	out, e := projectGit(ctx, dir, []byte(strings.Join(roots, "\n")+"\n"), int64(l.GitObjects)*65+1, "rev-list", "--objects", "--no-object-names", "--stdin")
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, id := range strings.Fields(string(out)) {
		if !migration.ValidOID(id) || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		if len(ids) > l.GitObjects {
			return nil, fmt.Errorf("Git traversal budget exceeded")
		}
	}
	sort.Strings(ids)
	var total int64
	for _, id := range ids {
		out, e := projectGit(ctx, dir, []byte(id+"\n"), 256, "cat-file", "--batch-check=%(objectsize)")
		if e != nil {
			return nil, e
		}
		n, e := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if e != nil || n < 0 || n > l.GitObjectBytes || n > l.GitTotalBytes-total {
			return nil, fmt.Errorf("Git byte budget exceeded")
		}
		total += n
	}
	return ids, nil
}

// All Git output is bounded, replacement objects and ambient object stores are
// disabled, and the command context cancels a producer when a bound is exceeded.
func projectGit(ctx context.Context, dir string, input []byte, limit int64, args ...string) ([]byte, error) {
	if dir != "" {
		args = append([]string{"--git-dir=" + dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	env := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		if !strings.HasPrefix(k, "GIT_") {
			env = append(env, v)
		}
	}
	cmd.Env = append(env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1")
	cmd.Stdin = bytes.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = &projectBoundWriter{w: &out, left: limit}
	cmd.Stderr = io.Discard
	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("bounded Git %s failed: %w", args[len(args)-1], e)
	}
	return out.Bytes(), nil
}

type projectBoundWriter struct {
	w    io.Writer
	left int64
}

func (w *projectBoundWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.left {
		return 0, fmt.Errorf("Git output budget exceeded")
	}
	n, e := w.w.Write(b)
	w.left -= int64(n)
	return n, e
}
func validateProjectNative(dir string, refs []migration.NativeRef) error {
	events := []StoredEvent{}
	memories := []StoredMemory{}
	code := map[string]string{}
	eventIDs, memoryIDs := map[string]bool{}, map[string]bool{}
	for _, r := range refs {
		if _, err := gitOutputAt(dir, "rev-parse", "--verify", r.OID+"^{commit}"); err != nil {
			return fmt.Errorf("selected native/code ref must resolve to a commit")
		}
		switch r.Kind {
		case "actor":
			rows, e := validateQuarantinedActor(dir, projectActorOwner(r.SourceRef), r.OID)
			if e != nil {
				return e
			}
			for _, row := range rows {
				if !eventIDs[row.ID] {
					events = append(events, row)
					eventIDs[row.ID] = true
				}
			}
		case "proposal":
			id := "sha256:" + strings.TrimPrefix(r.SourceRef, "refs/hn/proposals/")
			if _, observed, ok := parseAcceptedProposalRef(r.SourceRef); ok {
				id = observed
			}
			if old, ok := code[id]; ok && old != r.OID {
				return fmt.Errorf("conflicting selected proposal observations")
			}
			code[id] = r.OID
		case "memory":
			source, e := projectMemorySource(r.SourceRef, r.OID)
			if e != nil {
				return e
			}
			rows, e := loadMemoryStreamAt(dir, source)
			if e != nil {
				return e
			}
			for _, row := range rows {
				if !memoryIDs[row.ID] {
					memories = append(memories, row)
					memoryIDs[row.ID] = true
				}
			}
		}
	}
	if e := validateActorChains(events); e != nil {
		return e
	}
	if e := validateExactEventReferenceClosure(events); e != nil {
		return e
	}
	if _, e := validateResourceRelationships(events); e != nil {
		return e
	}
	if _, e := validateIssueRelationships(events); e != nil {
		return e
	}
	if _, e := ProjectIdentityContinuity(events); e != nil {
		return e
	}
	byID := map[string]StoredEvent{}
	for _, v := range events {
		byID[v.ID] = v
	}
	for id, head := range code {
		v, ok := byID[id]
		if !ok || !isProposalKind(v.Event.Kind) || v.Event.Head != head {
			return fmt.Errorf("proposal ref lacks exact signed event/head")
		}
	}
	for _, v := range events {
		dep, e := replicationEventDependency(dir, dir, v, byID, code, map[string]string{}, map[string]string{})
		if e != nil {
			return e
		}
		if dep != nil {
			return fmt.Errorf("native dependency missing: %s", dep.Missing)
		}
		if v.Event.Kind == "run.request" {
			digest, e := replicationPipelineDigestAt(dir, dir, v.Event.Commit, v.Event.Pipeline)
			if e != nil || digest != v.Event.Definition {
				return fmt.Errorf("native pipeline dependency mismatch")
			}
		}
	}
	p := ProjectMemories(memories, MemoryProjectionContext{Events: events, Resolver: replicationMemoryResolver{quarantineGitDir: dir, acceptedGitDir: dir}})
	if len(p.Diagnostics) > 0 || len(p.MissingDependencies) > 0 {
		return fmt.Errorf("native memory dependency/relationship invalid")
	}
	for _, r := range p.Rows {
		if r.Applicability == memoryApplicabilityAnchorInvalid || r.Evidence == memoryEvidenceInvalid {
			return fmt.Errorf("native memory anchor/evidence invalid")
		}
	}
	return nil
}

func projectHeader(r *bufio.Reader) (string, error) {
	line, e := r.ReadSlice('\n')
	if e == io.EOF && len(line) == 0 {
		return "", io.EOF
	}
	if e != nil || len(line) > 256 {
		return "", fmt.Errorf("invalid or oversized Git material header")
	}
	return string(line), nil
}
func projectSafeEnvironment() error {
	for _, k := range []string{"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_REPLACE_REF_BASE", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_SHALLOW_FILE", "GIT_GRAFT_FILE"} {
		if os.Getenv(k) != "" {
			return fmt.Errorf("native snapshots refuse ambient Git object/config overrides: %s", k)
		}
	}
	return nil
}
func projectDependencyRoots(dir string, refs []migration.NativeRef) ([]string, error) {
	roots := []string{}
	for _, r := range refs {
		switch r.Kind {
		case "actor":
			rows, e := validateQuarantinedActor(dir, projectActorOwner(r.SourceRef), r.OID)
			if e != nil {
				return nil, e
			}
			for _, v := range rows {
				for _, id := range []string{v.Event.Base, v.Event.Head, v.Event.Commit} {
					if id != "" {
						if !migration.ValidOID(id) {
							return nil, fmt.Errorf("invalid native code dependency")
						}
						roots = append(roots, id)
					}
				}
			}
		case "memory":
			source, e := projectMemorySource(r.SourceRef, r.OID)
			if e != nil {
				return nil, e
			}
			rows, e := loadMemoryStreamAt(dir, source)
			if e != nil {
				return nil, e
			}
			for _, v := range rows {
				if v.Envelope.Record != nil {
					roots = append(roots, v.Envelope.Record.Anchor.Commit)
				}
			}
		}
	}
	return sortedUniqueStrings(roots...), nil
}

func projectActorOwner(ref string) string {
	if _, actor, ok := parseAcceptedActorRef(ref); ok {
		return actor
	}
	if strings.HasPrefix(ref, "refs/hn/actors/") {
		return strings.TrimPrefix(ref, "refs/hn/actors/")
	}
	return ""
}
func projectMemorySource(ref, head string) (memoryStreamSource, error) {
	if actor, stream, ok := parseMemoryRef(ref); ok {
		return memoryStreamSource{Ref: ref, Actor: actor, Stream: stream, Head: head}, nil
	}
	if remote, actor, stream, ok := parseAcceptedMemoryRef(ref); ok {
		return memoryStreamSource{Ref: ref, Remote: remote, Actor: actor, Stream: stream, Head: head, Accepted: true}, nil
	}
	return memoryStreamSource{}, fmt.Errorf("invalid selected memory observation")
}

// projectAuditObjectUnion preflights the union before any importing native read.
// Ambient destination availability never discounts a saved migration budget.
func projectAuditObjectUnion(ctx context.Context, dir string, refs []migration.NativeRef, ordinary []string, l migration.Limits) ([]string, error) {
	sizes := map[string]int64{}
	var total int64
	add := func(id string, n int64) error {
		if old, ok := sizes[id]; ok {
			if old != n {
				return fmt.Errorf("inconsistent object size")
			}
			return nil
		}
		if n < 0 || n > l.GitObjectBytes || n > l.GitTotalBytes-total || len(sizes) >= l.GitObjects {
			return fmt.Errorf("aggregate Git/audit expansion budget exceeded")
		}
		sizes[id] = n
		total += n
		return nil
	}
	for _, id := range ordinary {
		out, e := projectGit(ctx, dir, []byte(id+"\n"), 256, "cat-file", "--batch-check=%(objectsize)")
		if e != nil {
			return nil, e
		}
		n, e := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if e != nil {
			return nil, e
		}
		if e = add(id, n); e != nil {
			return nil, e
		}
	}
	for _, r := range refs {
		if r.Kind != "actor" {
			continue
		}
		measured := map[string]int64{}
		if e := auditHistoryPackSizesAt(dir, r.OID, measured, add); e != nil {
			return nil, e
		}
	}
	ids := make([]string, 0, len(sizes))
	for id := range sizes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// Explicit IDs avoid rev traversal across intentionally absent audit parents.
func projectCopyObjectIDs(ctx context.Context, source, dest string, ids []string, l migration.Limits) error {
	for _, id := range ids {
		meta, e := projectGit(ctx, source, []byte(id+"\n"), 256, "cat-file", "--batch-check=%(objecttype) %(objectsize)")
		if e != nil {
			return e
		}
		f := strings.Fields(string(meta))
		if len(f) != 2 {
			return fmt.Errorf("missing admitted object")
		}
		n, e := strconv.ParseInt(f[1], 10, 64)
		if e != nil || n < 0 || n > l.GitObjectBytes {
			return fmt.Errorf("invalid admitted object size")
		}
		raw, e := projectGit(ctx, source, nil, n, "cat-file", f[0], id)
		if e != nil {
			return e
		}
		out, e := projectGit(ctx, dest, raw, 256, "hash-object", "-w", "-t", f[0], "--stdin")
		if e != nil || strings.TrimSpace(string(out)) != id {
			return fmt.Errorf("admitted object copy mismatch")
		}
	}
	return nil
}

// Partial/promisor repositories can lazily fetch during nominally read-only
// object queries. This source format is deliberately unsupported for export.
func projectReadOnlySource(ctx context.Context, dir string) error {
	raw, e := projectGit(ctx, dir, nil, 1<<20, "config", "--includes", "--list", "--null")
	if e != nil {
		return e
	}
	for _, entry := range strings.Split(string(raw), "\x00") {
		key, _, _ := strings.Cut(entry, "\n")
		key = strings.ToLower(key)
		if key == "extensions.partialclone" || (strings.HasPrefix(key, "remote.") && strings.HasSuffix(key, ".promisor")) {
			return fmt.Errorf("read-only export refuses partial/promisor sources; materialize an ordinary complete repository explicitly")
		}
	}
	return nil
}
