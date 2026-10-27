package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"hubnot/internal/migration"
)

// MigrationSession owns a saved immutable observation and bounded blob spool.
// Exported values are inspectable copies; apply always reloads authenticated files.
type MigrationSession struct {
	Directory string
	Plan      migration.Plan
	Snapshot  migration.Snapshot
}

func PrepareMigration(ctx context.Context, dir string, s migration.Snapshot, source migration.BlobSource, sel migration.Selection, d migration.Destination, l migration.Limits, decisions []migration.Decision) (session *MigrationSession, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	p, e := migration.NewPlan(ctx, s, sel, d, l, decisions)
	if e != nil {
		return nil, e
	}
	raw, e := migration.CanonicalSnapshot(s, l)
	if e != nil {
		return nil, e
	}
	_ = json.Unmarshal(raw, &s)
	if e = migrationInitKey(dir); e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	if e = os.Mkdir(filepath.Join(dir, "blobs"), 0700); e != nil {
		return nil, e
	}
	// Reuse canonical archive verification as an immutable spool, then copy only
	// declared digests. Source adapters and source repositories need not survive.
	archive, e := os.CreateTemp(dir, "snapshot-")
	if e != nil {
		return nil, e
	}
	defer func() { archive.Close(); os.Remove(archive.Name()) }()
	if e = migration.WriteArchive(ctx, archive, s, source, l); e != nil {
		return nil, e
	}
	if _, e = archive.Seek(0, 0); e != nil {
		return nil, e
	}
	verified, e := migration.ReadArchive(ctx, archive, l)
	if e != nil {
		return nil, e
	}
	defer verified.Close()
	for _, b := range s.Blobs {
		r, e := verified.Open(ctx, b.Digest)
		if e != nil {
			return nil, e
		}
		f, e := os.OpenFile(filepath.Join(dir, "blobs", b.Digest[7:]), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			r.Close()
			return nil, e
		}
		_, e = io.Copy(f, r)
		ce := f.Close()
		r.Close()
		if e != nil {
			return nil, e
		}
		if ce != nil {
			return nil, ce
		}
	}
	// Stage before saving a usable plan, including canonical native audit admission.
	q, e := StageProjectSnapshot(ctx, s, migrationDiskBlobs{dir, s}, l)
	if e != nil {
		return nil, e
	}
	for _, in := range p.Intents {
		if in.Kind == "git_refs" {
			var w migration.GitWrite
			_ = json.Unmarshal(in.Payload, &w)
			if e = migrationGitAncestry(ctx, q, w); e != nil {
				q.Close()
				return nil, e
			}
		}
	}
	q.Close()
	if e = migrationSeal(dir, "snapshot.json", raw); e != nil {
		return nil, e
	}
	pb, _ := migration.CanonicalPlan(p)
	if e = migrationSeal(dir, "plan.json", pb); e != nil {
		return nil, e
	}
	initial := migration.Progress{Schema: "hn.migration-progress/1", PlanDigest: migration.Digest(pb), Actor: "", Entries: []migration.ProgressEntry{}}
	checkpoint, e := migration.CanonicalProgress(initial, p)
	if e != nil {
		return nil, e
	}
	if e = migrationSeal(dir, "progress.json", checkpoint); e != nil {
		return nil, e
	}
	return &MigrationSession{dir, p, s}, nil
}
func OpenMigrationSession(ctx context.Context, dir string) (*MigrationSession, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, e := migrationUnseal(dir, "plan.json", 64<<20)
	if e != nil {
		return nil, e
	}
	var p migration.Plan
	if e = migration.StrictJSON(raw, &p); e != nil {
		return nil, e
	}
	b, e := migration.CanonicalPlan(p)
	if e != nil || !bytes.Equal(b, raw) {
		return nil, fmt.Errorf("noncanonical saved plan")
	}
	raw, e = migrationUnseal(dir, "snapshot.json", p.Limits.ManifestBytes)
	if e != nil {
		return nil, e
	}
	var s migration.Snapshot
	if e = migration.StrictJSON(raw, &s); e != nil {
		return nil, e
	}
	b, e = migration.CanonicalSnapshot(s, p.Limits)
	if e != nil || !bytes.Equal(raw, b) || migration.Digest(raw) != p.SnapshotDigest {
		return nil, fmt.Errorf("saved observation mismatch")
	}
	checkpoint, e := migrationUnseal(dir, "progress.json", p.Limits.ManifestBytes*4)
	if e != nil {
		return nil, fmt.Errorf("saved progress missing or unreadable; recovery required: %w", e)
	}
	var progress migration.Progress
	if e = migration.StrictJSON(checkpoint, &progress); e != nil {
		return nil, e
	}
	if _, e = migration.CanonicalProgress(progress, p); e != nil {
		return nil, e
	}
	return &MigrationSession{dir, p, s}, nil
}

type migrationDiskBlobs struct {
	dir string
	s   migration.Snapshot
}

func (b migrationDiskBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	for _, v := range b.s.Blobs {
		if v.Digest == d {
			path := filepath.Join(b.dir, "blobs")
			if e := migrationPrivateDir(path); e != nil {
				return nil, e
			}
			file := filepath.Join(path, d[7:])
			st, e := os.Lstat(file)
			if e != nil {
				return nil, e
			}
			if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Size() != v.Size {
				return nil, fmt.Errorf("unsafe immutable blob")
			}
			f, e := os.Open(file)
			if e != nil {
				return nil, e
			}
			h := sha256.New()
			n, e := io.Copy(h, io.LimitReader(f, v.Size+1))
			if e != nil || n != v.Size || "sha256:"+hex.EncodeToString(h.Sum(nil)) != d {
				f.Close()
				return nil, fmt.Errorf("immutable blob changed")
			}
			if _, e = f.Seek(0, 0); e != nil {
				f.Close()
				return nil, e
			}
			return f, nil
		}
	}
	return nil, fmt.Errorf("undeclared blob")
}

type migrationApplyContext struct {
	write    migration.BoundWrite
	deps     map[string]migration.Receipt
	blobs    migration.BlobSource
	snapshot migration.Snapshot
	limits   migration.Limits
}

func (c migrationApplyContext) BoundWrite() migration.BoundWrite {
	b := c.write
	b.Payload = append(json.RawMessage{}, b.Payload...)
	return b
}
func (c migrationApplyContext) Dependency(id string) (migration.Receipt, bool) {
	r, ok := c.deps[id]
	return r, ok
}
func (c migrationApplyContext) OpenBlob(ctx context.Context, d string) (io.ReadCloser, migration.Blob, error) {
	for _, b := range c.snapshot.Blobs {
		if b.Digest == d {
			r, e := c.blobs.Open(ctx, d)
			return r, b, e
		}
	}
	return nil, migration.Blob{}, fmt.Errorf("undeclared blob")
}

// ApplyMigration persists prepared and in-flight states before the one effect.
// Every error after in-flight is uncertain. Negative reconciliation never retries.
func ApplyMigration(ctx context.Context, session *MigrationSession, d migration.Destination, actor string) error {
	unlock, e := migrationLock(session.Directory)
	if e != nil {
		return e
	}
	defer unlock()
	s, e := OpenMigrationSession(ctx, session.Directory)
	if e != nil {
		return e
	}
	p := s.Plan
	if d.Locator() != p.Destination {
		return fmt.Errorf("configured destination differs from saved plan destination")
	}
	raw, _ := migration.CanonicalPlan(p)
	digest := migration.Digest(raw)
	// Reverify all saved bytes and native relationships before any destination write.
	q, e := StageProjectSnapshot(ctx, s.Snapshot, migrationDiskBlobs{s.Directory, s.Snapshot}, p.Limits)
	if e != nil {
		return e
	}
	defer q.Close()
	var progress migration.Progress
	existing, e := migrationUnseal(s.Directory, "progress.json", p.Limits.ManifestBytes*4)
	if e != nil {
		return fmt.Errorf("saved progress missing or unreadable; recovery required: %w", e)
	}
	if e = migration.StrictJSON(existing, &progress); e != nil {
		return e
	}
	if _, e = migration.CanonicalProgress(progress, p); e != nil {
		return e
	}
	bindActor := progress.Actor == "" && len(progress.Entries) == 0
	if !bindActor && progress.Actor != actor {
		return fmt.Errorf("migration actor changed")
	}
	if bindActor {
		progress.Actor = actor
	}

	if e = migrationPreflightGit(ctx, p, progress, q, d, migrationApplyContext{blobs: migrationDiskBlobs{s.Directory, s.Snapshot}, snapshot: s.Snapshot, limits: p.Limits}); e != nil {
		return e
	}
	save := func() error {
		b, e := migration.CanonicalProgress(progress, p)
		if e != nil {
			return e
		}
		if int64(len(b)) > p.Limits.ManifestBytes*4 {
			return fmt.Errorf("progress exceeds bound")
		}
		return migrationSeal(s.Directory, "progress.json", b)
	}
	if bindActor {
		if e = save(); e != nil {
			return e
		}
	}
	confirmed := map[string]migration.Receipt{}
	for i, in := range p.Intents {
		if d.Locator() != p.Destination {
			return fmt.Errorf("configured destination changed during migration")
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		if i < len(progress.Entries) && progress.Entries[i].State == "confirmed" {
			confirmed[in.ID] = progress.Entries[i].Receipts[0]
			continue
		}
		if i == len(progress.Entries) {
			w, e := migration.BindWrite(digest, in, confirmed)
			if e != nil {
				return e
			}
			progress.Entries = append(progress.Entries, migration.ProgressEntry{IntentID: in.ID, State: "prepared", Write: w, Receipts: []migration.Receipt{}})
			if e = save(); e != nil {
				return e
			}
		}
		entry := &progress.Entries[i]
		deps := map[string]migration.Receipt{}
		for _, id := range in.DependsOn {
			r, ok := confirmed[id]
			if !ok {
				return fmt.Errorf("dependency not confirmed")
			}
			deps[id] = r
		}
		ac := migrationApplyContext{entry.Write, deps, migrationDiskBlobs{s.Directory, s.Snapshot}, s.Snapshot, p.Limits}
		var receipt migration.Receipt
		if entry.State == "inflight" {
			recon, e := d.Reconcile(ctx, migrationCloneIntent(in), ac)
			if e != nil {
				return e
			}
			if _, e = migration.CanonicalReconciliation(recon, p, in, entry.Write); e != nil {
				return e
			}
			if recon.Status != "exact" || recon.Receipt == nil {
				return fmt.Errorf("manual recovery required for uncertain %s: %s", in.ID, recon.Reason)
			}
			receipt = *recon.Receipt
		} else {
			entry.State = "inflight"
			if e = save(); e != nil {
				return e
			}
			receipt, e = d.Apply(ctx, migrationCloneIntent(in), ac)
			if e != nil {
				return fmt.Errorf("uncertain %s; resume reconciles only: %w", in.ID, e)
			}
		}
		if e = migration.ValidateReceipt(receipt, p, in, entry.Write); e != nil {
			return e
		}
		entry.Receipts = []migration.Receipt{receipt}
		entry.State = "confirmed"
		if e = save(); e != nil {
			return e
		}
		confirmed[in.ID] = receipt
	}
	return nil
}

// InspectMigration returns canonical source records, relationships and immutable
// blob bytes without importing any provider client or requiring a signing key.
func InspectMigration(ctx context.Context, session *MigrationSession) (migration.Snapshot, migration.BlobSource, error) {
	s, e := OpenMigrationSession(ctx, session.Directory)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	return s.Snapshot, migrationDiskBlobs{s.Directory, s.Snapshot}, nil
}

// ComposeMigrationDestinations combines communication writes with the shared
// code transport. Routing depends only on the closed git_refs intent kind and
// therefore survives reopening a saved session without a fresh provider preview.
func ComposeMigrationDestinations(communications, code migration.Destination) migration.Destination {
	return migrationCompositeDestination{communications, code}
}

type migrationCompositeDestination struct{ communications, code migration.Destination }

func (d migrationCompositeDestination) Capabilities(ctx context.Context) ([]migration.Capability, error) {
	a, e := d.communications.Capabilities(ctx)
	if e != nil {
		return nil, e
	}
	b, e := d.code.Capabilities(ctx)
	return append(a, b...), e
}
func (d migrationCompositeDestination) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	a, af, e := d.communications.Plan(ctx, s, sel, l)
	if e != nil {
		return nil, nil, e
	}
	for _, in := range a {
		if in.Kind == "git_refs" {
			return nil, nil, fmt.Errorf("communication destination cannot own Git transport")
		}
	}
	b, bf, e := d.code.Plan(ctx, s, sel, l)
	if e != nil {
		return nil, nil, e
	}
	for _, in := range b {
		if in.Kind != "git_refs" {
			return nil, nil, fmt.Errorf("code destination must own only Git transport")
		}
	}
	return append(a, b...), append(af, bf...), nil
}
func (d migrationCompositeDestination) owner(in migration.Intent) migration.Destination {
	if in.Kind == "git_refs" {
		return d.code
	}
	return d.communications
}
func (d migrationCompositeDestination) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	return d.owner(in).Apply(ctx, in, ac)
}
func (d migrationCompositeDestination) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	return d.owner(in).Reconcile(ctx, in, ac)
}

// ComposeMigrationSources joins provider communications with explicit already
// present local Git refs. It never clones/fetches through a provider. At most one
// child supplies native material; callers close the returned combined source.
func ComposeMigrationSources(primary migration.Source, additional ...migration.Source) migration.Source {
	return migrationCompositeSource{append([]migration.Source{primary}, additional...)}
}

type migrationCompositeSource struct{ children []migration.Source }

func (s migrationCompositeSource) Capabilities(ctx context.Context) ([]migration.Capability, error) {
	out := []migration.Capability{}
	for _, c := range s.children {
		v, e := c.Capabilities(ctx)
		if e != nil {
			return nil, e
		}
		out = append(out, v...)
	}
	return out, nil
}

type migrationCompositeBlobs struct {
	sources []migration.BlobSource
	owner   map[string]migration.BlobSource
}

func (b *migrationCompositeBlobs) Open(ctx context.Context, d string) (io.ReadCloser, error) {
	source, ok := b.owner[d]
	if !ok {
		return nil, fmt.Errorf("undeclared composite blob")
	}
	return source.Open(ctx, d)
}
func (b *migrationCompositeBlobs) Close() error {
	var result error
	for _, s := range b.sources {
		if c, ok := s.(io.Closer); ok {
			if e := c.Close(); e != nil {
				result = errors.Join(result, e)
			}
		}
	}
	return result
}
func (s migrationCompositeSource) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (result migration.Snapshot, source migration.BlobSource, err error) {
	b := &migrationCompositeBlobs{owner: map[string]migration.BlobSource{}}
	defer func() {
		if err != nil {
			b.Close()
		}
	}()
	for i, c := range s.children {
		v, bs, e := c.Snapshot(ctx, sel, l)
		if e != nil {
			return result, nil, e
		}
		if bs != nil {
			b.sources = append(b.sources, bs)
		} else if len(v.Blobs) > 0 {
			return result, nil, fmt.Errorf("source declared blobs without bytes")
		}
		if _, e = migration.CanonicalSnapshot(v, l); e != nil {
			return result, nil, e
		}
		if i == 0 {
			result = v
			result.Blobs = []migration.Blob{}
		} else {
			if v.GitBundle != "" {
				if result.GitBundle != "" {
					return result, nil, fmt.Errorf("multiple native Git sources require an explicit unified selection")
				}
				result.GitBundle = v.GitBundle
			}
			result.NativeRefs = append(result.NativeRefs, v.NativeRefs...)
			result.Records = append(result.Records, v.Records...)
			result.Findings = append(result.Findings, v.Findings...)
		}
		for _, blob := range v.Blobs {
			if _, ok := b.owner[blob.Digest]; ok {
				for _, old := range result.Blobs {
					if old.Digest == blob.Digest && old.Size != blob.Size {
						return result, nil, fmt.Errorf("conflicting blob declaration")
					}
				}
				continue
			}
			b.owner[blob.Digest] = bs
			result.Blobs = append(result.Blobs, blob)
		}
	}
	raw, e := migration.CanonicalSnapshot(result, l)
	if e != nil {
		return result, nil, e
	}
	_ = json.Unmarshal(raw, &result)
	return result, b, nil
}

func migrationPreflightGit(ctx context.Context, p migration.Plan, progress migration.Progress, q *ProjectQuarantine, d migration.Destination, ac migrationApplyContext) error {
	for i, in := range p.Intents {
		if in.Kind != "git_refs" {
			continue
		}
		if i < len(progress.Entries) && progress.Entries[i].State != "prepared" {
			continue
		}
		var w migration.GitWrite
		_ = json.Unmarshal(in.Payload, &w)
		if e := migrationGitAncestry(ctx, q, w); e != nil {
			return e
		}
		if preflight, ok := d.(interface {
			Preflight(context.Context, migration.Intent, migration.ApplyContext) error
		}); ok {
			raw, _ := migration.CanonicalPlan(p)
			write, e := migration.BindWrite(migration.Digest(raw), in, nil)
			ac.write = write
			if e != nil {
				return e
			}
			if e = preflight.Preflight(ctx, migrationCloneIntent(in), ac); e != nil {
				return e
			}
		} else {
			return fmt.Errorf("Git intents require the shared transport preflight boundary")
		}
	}
	return nil
}

func migrationCloneIntent(in migration.Intent) migration.Intent {
	in.Payload = append(json.RawMessage{}, in.Payload...)
	in.Sources = append([]migration.Key{}, in.Sources...)
	in.DependsOn = append([]string{}, in.DependsOn...)
	return in
}

func (d migrationCompositeDestination) Preflight(ctx context.Context, in migration.Intent, ac migration.ApplyContext) error {
	if p, ok := d.owner(in).(interface {
		Preflight(context.Context, migration.Intent, migration.ApplyContext) error
	}); ok {
		return p.Preflight(ctx, in, ac)
	}
	if in.Kind == "git_refs" {
		return fmt.Errorf("Git destination lacks shared transport preflight")
	}
	return nil
}

func (d migrationCompositeDestination) Locator() migration.Locator {
	a := d.communications.Locator()
	if a != d.code.Locator() {
		return migration.Locator{}
	}
	return a
}
