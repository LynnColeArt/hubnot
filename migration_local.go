package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hubnot/internal/migration"
)

// LocalMigrationSource reads explicit committed refs only; no credential access.
type LocalMigrationSource struct{ GitDir string }

func (s LocalMigrationSource) Capabilities(context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "project", Read: true, Preserve: true, Limits: []string{}, Reason: "explicit selected native Git history; select imported foreign records separately with ImportedMigrationSource"}}, nil
}
func (s LocalMigrationSource) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	p, e := ExportProjectSnapshot(ctx, s.GitDir, sel.Refs, l)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	return p.Snapshot, p, nil
}

// ArchiveMigrationSource owns no files; the returned blob source is Close-able.
type ArchiveMigrationSource struct{ Path string }

func (s ArchiveMigrationSource) Capabilities(ctx context.Context) ([]migration.Capability, error) {
	return LocalMigrationSource{}.Capabilities(ctx)
}
func (s ArchiveMigrationSource) Snapshot(ctx context.Context, _ migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	f, e := os.Open(s.Path)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	defer f.Close()
	a, e := migration.ReadArchive(ctx, f, l)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	q, e := StageProjectSnapshot(ctx, a.Snapshot, a, l)
	if e != nil {
		a.Close()
		return migration.Snapshot{}, nil, e
	}
	q.Close()
	return a.Snapshot, a, nil
}

type LocalMigrationDestination struct{ Repository string }

func (d LocalMigrationDestination) Capabilities(context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "project", Read: true, Preserve: true, Create: true, Limits: []string{"foreign records remain attributed inert content", "source governance is preserved as historical evidence"}, Reason: "native observations and offline archive"}}, nil
}
func (d LocalMigrationDestination) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if sel.Destination.Kind != "local" || sel.Destination.Repository != d.Repository || !filepath.IsAbs(d.Repository) {
		return nil, nil, fmt.Errorf("destination locator mismatch")
	}
	dir, e := migrationDestinationGitDir(ctx, d.Repository)
	if e != nil {
		return nil, nil, e
	}
	baseline, e := migrationBaseline(ctx, dir)
	if e != nil {
		return nil, nil, e
	}
	raw, e := migration.CanonicalSnapshot(s, l)
	if e != nil {
		return nil, nil, e
	}
	format, e := projectGit(ctx, dir, nil, 256, "rev-parse", "--show-object-format")
	if e != nil {
		return nil, nil, e
	}
	refs := []migration.RefUpdate{}
	for _, r := range s.NativeRefs {
		if (len(r.OID) == 64) != (string(format) == "sha256\n") {
			return nil, nil, fmt.Errorf("source/destination Git object formats differ")
		}
		ref, e := migrationObservationRef(s.SourceID, r)
		if e != nil {
			return nil, nil, e
		}
		old, e := migrationReadRef(ctx, dir, ref)
		if e != nil {
			return nil, nil, e
		}
		if old != "" && old != r.OID {
			return nil, nil, fmt.Errorf("destination observation conflict %s", ref)
		}
		refs = append(refs, migration.RefUpdate{Ref: ref, OID: r.OID, ExpectedOld: old})
	}
	payload, _ := json.Marshal(migration.ProjectWrite{Schema: "hn.migration.project-write/1", SnapshotDigest: migration.Digest(raw), Refs: refs, Governance: "preserve", Baseline: baseline})
	fs := []migration.Finding{{ID: "local/governance", Status: "requires_mapping", Path: "governance", Message: "Preserve source governance as historical evidence; destination governance and local authority stay under explicit local control."}, {ID: "local/preserved", Status: "preserved", Path: "project", Message: "All retrieved records and verified blobs remain attributed and inspectable offline; native observations retain exact signed bytes."}}
	return []migration.Intent{{ID: "preserve-project", Action: "preserve", Kind: "project", Sources: []migration.Key{}, Payload: payload, DependsOn: []string{}}}, fs, nil
}
func migrationLocalWrite(ctx context.Context, d LocalMigrationDestination, in migration.Intent, ac migration.ApplyContext) (string, migration.ProjectWrite, error) {
	b := ac.BoundWrite()
	bound := in
	bound.Payload = b.Payload
	if _, e := migration.CanonicalWrite(bound, true); e != nil {
		return "", migration.ProjectWrite{}, e
	}
	var w migration.ProjectWrite
	_ = json.Unmarshal(b.Payload, &w)
	dir, e := migrationDestinationGitDir(ctx, d.Repository)
	if e != nil {
		return "", w, e
	}
	base, e := migrationBaseline(ctx, dir)
	if e != nil || base != w.Baseline {
		return "", w, fmt.Errorf("destination identity/HEAD changed")
	}
	return dir, w, nil
}
func migrationReceipt(in migration.Intent, ac migration.ApplyContext, destination migration.Locator, id, revision string) migration.Receipt {
	b := ac.BoundWrite()
	return migration.Receipt{PlanDigest: b.PlanDigest, IntentID: in.ID, Kind: in.Kind, Action: in.Action, Destination: destination, RemoteID: id, RemoteKey: id, Revision: revision, PayloadDigest: b.PayloadDigest}
}
func (d LocalMigrationDestination) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	dir, w, e := migrationLocalWrite(ctx, d, in, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	s, e := migrationContextSnapshot(ac, w.SnapshotDigest)
	if e != nil {
		return migration.Receipt{}, e
	}
	q, e := StageProjectSnapshot(ctx, s, migrationContextBlobs{ac}, migrationContextLimits(ac))
	if e != nil {
		return migration.Receipt{}, e
	}
	defer q.Close()
	if e = migrationCheckRefGuards(ctx, dir, w.Refs); e != nil {
		return migration.Receipt{}, e
	}
	path, e := migrationLocalArchivePath(dir, w.SnapshotDigest, true)
	if e != nil {
		return migration.Receipt{}, e
	}
	if e = migrationPublishArchive(ctx, path, s, migrationContextBlobs{ac}, migrationContextLimits(ac)); e != nil {
		return migration.Receipt{}, e
	}
	if e = migrationPromote(ctx, dir, q, w, migrationContextLimits(ac)); e != nil {
		return migration.Receipt{}, e
	}
	return migrationReceipt(in, ac, migration.Locator{Kind: "local", Repository: d.Repository}, w.SnapshotDigest, w.SnapshotDigest), nil
}
func (d LocalMigrationDestination) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	dir, w, e := migrationLocalWrite(ctx, d, in, ac)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	for _, r := range w.Refs {
		actual, e := migrationReadRef(ctx, dir, r.Ref)
		if e != nil {
			return migration.Reconciliation{}, e
		}
		if actual != r.OID {
			return migration.Reconciliation{Status: "no_proof", Reason: "destination refs do not prove the exact completed promotion"}, nil
		}
	}
	a, e := ReadLocalMigration(ctx, dir, w.SnapshotDigest, migrationContextLimits(ac))
	if e != nil {
		return migration.Reconciliation{Status: "no_proof", Reason: "saved local archive is absent or does not verify"}, nil
	}
	a.Close()
	r := migrationReceipt(in, ac, migration.Locator{Kind: "local", Repository: d.Repository}, w.SnapshotDigest, w.SnapshotDigest)
	return migration.Reconciliation{Status: "exact", Receipt: &r, Reason: "exact refs and verified immutable project archive"}, nil
}

// Snapshot/limits are engine-owned; local preservation is a root responsibility,
// while optional provider adapters only need the public ApplyContext contract.
func migrationContextSnapshot(ac migration.ApplyContext, digest string) (migration.Snapshot, error) {
	c, ok := ac.(migrationApplyContext)
	if !ok {
		return migration.Snapshot{}, fmt.Errorf("local destination requires engine context")
	}
	b, e := migration.CanonicalSnapshot(c.snapshot, migrationContextLimits(ac))
	if e != nil || migration.Digest(b) != digest {
		return migration.Snapshot{}, fmt.Errorf("local snapshot binding mismatch")
	}
	return c.snapshot, nil
}
func migrationContextLimits(ac migration.ApplyContext) migration.Limits {
	c, ok := ac.(migrationApplyContext)
	if !ok {
		return migration.Limits{}
	}
	return c.limits
}

type migrationContextBlobs struct{ c migration.ApplyContext }

func (b migrationContextBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	r, _, e := b.c.OpenBlob(ctx, d)
	return r, e
}
func migrationLocalArchivePath(dir, digest string, create bool) (string, error) {
	if !migration.ValidDigest(digest) {
		return "", fmt.Errorf("invalid snapshot digest")
	}
	for _, part := range []string{"hn", "imports"} {
		dir = filepath.Join(dir, part)
		st, e := os.Lstat(dir)
		if os.IsNotExist(e) && create {
			e = os.Mkdir(dir, 0700)
			if e != nil {
				return "", e
			}
			st, e = os.Lstat(dir)
		}
		if e != nil {
			return "", e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0077 != 0 {
			return "", fmt.Errorf("unsafe local import store")
		}
	}
	return filepath.Join(dir, digest[7:]+".tar"), nil
}
func migrationPublishArchive(ctx context.Context, path string, s migration.Snapshot, source migration.BlobSource, l migration.Limits) error {
	if e := migrationNoLinks(filepath.Dir(path)); e != nil {
		return e
	}
	if _, e := os.Lstat(path); e == nil {
		a, e := migrationReadArchivePath(ctx, path, l)
		if e != nil {
			return e
		}
		defer a.Close()
		raw, _ := migration.CanonicalSnapshot(a.Snapshot, l)
		expected, _ := migration.CanonicalSnapshot(s, l)
		if string(raw) != string(expected) {
			return fmt.Errorf("destination archive conflict")
		}
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".hn-migration-")
	if e != nil {
		return e
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	if e = migration.WriteArchive(ctx, f, s, source, l); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Link(f.Name(), path)
}
func migrationReadArchivePath(ctx context.Context, path string, l migration.Limits) (*migration.Archive, error) {
	st, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("unsafe archive file")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return migration.ReadArchive(ctx, f, l)
}
func ReadLocalMigration(ctx context.Context, gitDir, digest string, l migration.Limits) (*migration.Archive, error) {
	p, e := migrationLocalArchivePath(gitDir, digest, false)
	if e != nil {
		return nil, e
	}
	a, e := migrationReadArchivePath(ctx, p, l)
	if e != nil {
		return nil, e
	}
	raw, e := migration.CanonicalSnapshot(a.Snapshot, l)
	if e != nil || migration.Digest(raw) != digest {
		a.Close()
		return nil, fmt.Errorf("local snapshot digest mismatch")
	}
	q, e := StageProjectSnapshot(ctx, a.Snapshot, a, l)
	if e != nil {
		a.Close()
		return nil, e
	}
	q.Close()
	return a, nil
}

// ArchiveMigrationDestination publishes a verified archive without replacement.
type ArchiveMigrationDestination struct{ Path string }

func (d ArchiveMigrationDestination) Capabilities(ctx context.Context) ([]migration.Capability, error) {
	return LocalMigrationDestination{}.Capabilities(ctx)
}
func (d ArchiveMigrationDestination) Plan(_ context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if sel.Destination.Kind != "archive" || sel.Destination.Repository != d.Path || !filepath.IsAbs(d.Path) {
		return nil, nil, fmt.Errorf("explicit absolute archive destination required")
	}
	if _, e := os.Lstat(d.Path); !os.IsNotExist(e) {
		return nil, nil, fmt.Errorf("archive destination must be absent")
	}
	raw, e := migration.CanonicalSnapshot(s, l)
	if e != nil {
		return nil, nil, e
	}
	p, _ := json.Marshal(migration.ProjectWrite{Schema: "hn.migration.project-write/1", SnapshotDigest: migration.Digest(raw), Refs: []migration.RefUpdate{}, Governance: "preserve", Baseline: migration.Digest([]byte(d.Path))})
	return []migration.Intent{{ID: "preserve-archive", Action: "preserve", Kind: "project", Sources: []migration.Key{}, Payload: p, DependsOn: []string{}}}, []migration.Finding{{ID: "archive/preserved", Status: "preserved", Path: "project", Message: "Retrieved content and exact native history preserved; no governance activated."}}, nil
}
func (d ArchiveMigrationDestination) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	var w migration.ProjectWrite
	if e := json.Unmarshal(ac.BoundWrite().Payload, &w); e != nil {
		return migration.Receipt{}, e
	}
	if w.Baseline != migration.Digest([]byte(d.Path)) {
		return migration.Receipt{}, fmt.Errorf("archive destination changed")
	}
	s, e := migrationContextSnapshot(ac, w.SnapshotDigest)
	if e != nil {
		return migration.Receipt{}, e
	}
	if e = migrationPublishArchive(ctx, d.Path, s, migrationContextBlobs{ac}, migrationContextLimits(ac)); e != nil {
		return migration.Receipt{}, e
	}
	return migrationReceipt(in, ac, migration.Locator{Kind: "archive", Repository: d.Path}, w.SnapshotDigest, w.SnapshotDigest), nil
}
func (d ArchiveMigrationDestination) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	var w migration.ProjectWrite
	_ = json.Unmarshal(ac.BoundWrite().Payload, &w)
	if w.Baseline != migration.Digest([]byte(d.Path)) {
		return migration.Reconciliation{}, fmt.Errorf("archive destination changed")
	}
	a, e := migrationReadArchivePath(ctx, d.Path, migrationContextLimits(ac))
	if e != nil {
		return migration.Reconciliation{Status: "no_proof", Reason: "archive unavailable"}, nil
	}
	defer a.Close()
	raw, e := migration.CanonicalSnapshot(a.Snapshot, migrationContextLimits(ac))
	if e != nil || migration.Digest(raw) != w.SnapshotDigest {
		return migration.Reconciliation{Status: "ambiguous", Reason: "archive differs"}, nil
	}
	r := migrationReceipt(in, ac, migration.Locator{Kind: "archive", Repository: d.Path}, w.SnapshotDigest, w.SnapshotDigest)
	return migration.Reconciliation{Status: "exact", Receipt: &r, Reason: "verified exact archive"}, nil
}

func (d LocalMigrationDestination) Locator() migration.Locator {
	return migration.Locator{Kind: "local", Repository: d.Repository}
}
func (d ArchiveMigrationDestination) Locator() migration.Locator {
	return migration.Locator{Kind: "archive", Repository: d.Path}
}

// ImportedMigrationSource explicitly selects previously preserved foreign
// observations. It never substitutes archived code for the current Git refs.
type ImportedMigrationSource struct {
	GitDir          string
	SnapshotDigests []string
}

func (s ImportedMigrationSource) Capabilities(context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "foreign_records", Read: true, Preserve: true, Limits: []string{"only explicitly selected imported snapshot digests", "current code/native refs are selected independently", "unselected imports are not claimed as read"}, Reason: "offline attributed records and verified attachments"}}, nil
}

type migrationImportedBlobs struct {
	root     string
	snapshot migration.Snapshot
}

func (b *migrationImportedBlobs) Close() error { return os.RemoveAll(b.root) }
func (b *migrationImportedBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	return (migrationDiskBlobs{b.root, b.snapshot}).Open(ctx, d)
}
func (s ImportedMigrationSource) Snapshot(ctx context.Context, _ migration.Selection, l migration.Limits) (result migration.Snapshot, source migration.BlobSource, err error) {
	if e := l.Validate(); e != nil {
		return result, nil, e
	}
	if !filepath.IsAbs(s.GitDir) || len(s.SnapshotDigests) == 0 || len(s.SnapshotDigests) > l.Refs {
		return result, nil, fmt.Errorf("explicit bounded imported snapshot selection required")
	}
	selected := append([]string{}, s.SnapshotDigests...)
	sort.Strings(selected)
	for i, d := range selected {
		if !migration.ValidDigest(d) || (i > 0 && selected[i-1] == d) {
			return result, nil, fmt.Errorf("invalid/duplicate imported snapshot selection")
		}
	}
	root, e := os.MkdirTemp("", "hn-imported-records-")
	if e != nil {
		return result, nil, e
	}
	b := &migrationImportedBlobs{root: root}
	defer func() {
		if err != nil {
			b.Close()
		}
	}()
	if e = os.Mkdir(filepath.Join(root, "blobs"), 0700); e != nil {
		return result, nil, e
	}
	identity, _ := json.Marshal(selected)
	result = migration.Snapshot{Schema: "hn.project-snapshot/1", Source: migration.Locator{Kind: "local", Repository: s.GitDir}, SourceID: migration.Digest(identity), Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{{ID: "imports/selection", Status: "mapped", Path: "imported_snapshots", Message: "Only explicitly selected imported snapshots were read; no coverage is claimed for unselected imports. Missing selected imports refuse the observation."}}}
	records := map[string][]byte{}
	blobs := map[string]migration.Blob{}
	var total int64
	for _, digest := range selected {
		a, e := ReadLocalMigration(ctx, s.GitDir, digest, l)
		if e != nil {
			return result, nil, fmt.Errorf("selected imported snapshot unavailable or invalid: %w", e)
		}
		e = func() error {
			defer a.Close()
			for _, r := range a.Snapshot.Records {
				raw, _ := json.Marshal(r)
				if old, ok := records[r.Key.String()]; ok {
					if !bytes.Equal(old, raw) {
						return fmt.Errorf("conflicting imported record revisions require an explicit snapshot choice")
					}
					continue
				}
				if len(result.Records) >= l.Records {
					return fmt.Errorf("aggregate imported record budget exceeded")
				}
				records[r.Key.String()] = raw
				result.Records = append(result.Records, r)
			}
			for _, blob := range a.Snapshot.Blobs {
				if blob.Digest == a.Snapshot.GitBundle {
					continue
				}
				if old, ok := blobs[blob.Digest]; ok {
					if old != blob {
						return fmt.Errorf("conflicting imported blob metadata")
					}
					continue
				}
				if len(blobs) >= l.Blobs || blob.Size > l.BlobBytes || blob.Size > l.AttachmentBytes-total {
					return fmt.Errorf("aggregate imported blob budget exceeded")
				}
				r, e := a.Open(ctx, blob.Digest)
				if e != nil {
					return e
				}
				f, e := os.OpenFile(filepath.Join(root, "blobs", blob.Digest[7:]), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if e != nil {
					r.Close()
					return e
				}
				n, e := io.Copy(f, io.LimitReader(r, blob.Size+1))
				ce := f.Close()
				r.Close()
				if e != nil {
					return e
				}
				if ce != nil {
					return ce
				}
				if n != blob.Size {
					return fmt.Errorf("imported blob length changed")
				}
				total += n
				blobs[blob.Digest] = blob
				result.Blobs = append(result.Blobs, blob)
			}
			for _, f := range a.Snapshot.Findings {
				if len(result.Findings) >= l.Records*8+l.Refs*8+32 {
					return fmt.Errorf("aggregate imported finding budget exceeded")
				}
				f.ID = "import/" + migration.Digest([]byte(digest + "\x00" + f.ID))[7:]
				result.Findings = append(result.Findings, f)
			}
			return nil
		}()
		if e != nil {
			return result, nil, e
		}
	}
	raw, e := migration.CanonicalSnapshot(result, l)
	if e != nil {
		return result, nil, e
	}
	_ = json.Unmarshal(raw, &result)
	b.snapshot = result
	return result, b, nil
}

// ListLocalMigrations discovers bounded immutable archive IDs without opening
// record contents or selecting any archive for a subsequent migration.
func ListLocalMigrations(gitDir string, l migration.Limits) ([]string, error) {
	if e := l.Validate(); e != nil {
		return nil, e
	}
	placeholder := migration.Digest(nil)
	path, e := migrationLocalArchivePath(gitDir, placeholder, false)
	if os.IsNotExist(e) {
		return []string{}, nil
	}
	if e != nil {
		return nil, e
	}
	f, e := os.Open(filepath.Dir(path))
	if e != nil {
		return nil, e
	}
	defer f.Close()
	entries, e := f.ReadDir(l.Refs + 1)
	if e != nil && e != io.EOF {
		return nil, e
	}
	if len(entries) > l.Refs {
		return nil, fmt.Errorf("import listing exceeds ref budget")
	}
	out := []string{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".tar") {
			return nil, fmt.Errorf("unexpected import store entry")
		}
		digest := "sha256:" + strings.TrimSuffix(entry.Name(), ".tar")
		if !migration.ValidDigest(digest) {
			return nil, fmt.Errorf("unexpected import store filename")
		}
		out = append(out, digest)
	}
	sort.Strings(out)
	return out, nil
}
