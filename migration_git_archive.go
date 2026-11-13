package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"hubnot/internal/migration"
)

const migrationArchiveRefPrefix = "refs/hn/migrations/"

// The plan can stage Git objects for transport. The published PR archive retains
// source metadata, while the same ref transaction pins the promoted commits.
func localMigrationMetadata(s migration.Snapshot) (migration.Snapshot, bool) {
	for _, record := range s.Records {
		if record.Key.Kind != "change_request" {
			continue
		}
		metadata := s
		metadata.GitBundle = ""
		metadata.NativeRefs = []migration.NativeRef{}
		metadata.Blobs = make([]migration.Blob, 0, len(s.Blobs))
		for _, blob := range s.Blobs {
			if blob.Digest != s.GitBundle {
				metadata.Blobs = append(metadata.Blobs, blob)
			}
		}
		return metadata, true
	}
	return s, false
}

func migrationGitArchiveUpdate(ctx context.Context, dir, digest, path string, limits migration.Limits) (migration.RefUpdate, error) {
	st, err := os.Stat(path)
	if err != nil {
		return migration.RefUpdate{}, err
	}
	if st.Size() > migrationGitArchiveLimit(limits) {
		return migration.RefUpdate{}, fmt.Errorf("migration archive exceeds Git transport budget")
	}
	out, err := projectGit(ctx, dir, nil, 256, "hash-object", "-w", "--no-filters", "--", path)
	if err != nil {
		return migration.RefUpdate{}, err
	}
	oid := strings.TrimSpace(string(out))
	if !migration.ValidOID(oid) {
		return migration.RefUpdate{}, fmt.Errorf("invalid migration archive object")
	}
	ref := migrationArchiveRefPrefix + digest[7:]
	old, err := migrationReadRef(ctx, dir, ref)
	if err != nil {
		return migration.RefUpdate{}, err
	}
	if old != "" && old != oid {
		return migration.RefUpdate{}, fmt.Errorf("migration archive ref already points to different bytes")
	}
	return migration.RefUpdate{Ref: ref, OID: oid, ExpectedOld: old}, nil
}

func migrationGitArchiveLimit(l migration.Limits) int64 {
	return l.GitTotalBytes + l.AttachmentBytes + l.ManifestBytes + int64(l.Blobs+4)*2048
}

func readMigrationGitArchive(ctx context.Context, dir, digest string, l migration.Limits) (*migration.Archive, error) {
	ref := migrationArchiveRefPrefix + digest[7:]
	oid, err := migrationReadRef(ctx, dir, ref)
	if err != nil {
		return nil, err
	}
	if oid == "" {
		return nil, os.ErrNotExist
	}
	raw, err := projectGit(ctx, dir, nil, migrationGitArchiveLimit(l), "cat-file", "blob", oid)
	if err != nil {
		return nil, err
	}
	archive, err := migration.ReadArchive(ctx, bytes.NewReader(raw), l)
	if err != nil {
		return nil, err
	}
	canonical, err := migration.CanonicalSnapshot(archive.Snapshot, l)
	if err != nil || migration.Digest(canonical) != digest {
		archive.Close()
		return nil, fmt.Errorf("Git migration archive digest mismatch")
	}
	return archive, nil
}

type pullRequestGitSource struct {
	source migration.Source
	dir    string
}

func (s pullRequestGitSource) Capabilities(ctx context.Context) ([]migration.Capability, error) {
	return s.source.Capabilities(ctx)
}

type observedMigrationSource struct {
	snapshot migration.Snapshot
	blobs    migration.BlobSource
}

func (s observedMigrationSource) Capabilities(context.Context) ([]migration.Capability, error) {
	return nil, nil
}
func (s observedMigrationSource) Snapshot(context.Context, migration.Selection, migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	return s.snapshot, s.blobs, nil
}

func (s pullRequestGitSource) Snapshot(ctx context.Context, sel migration.Selection, l migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	snapshot, blobs, err := s.source.Snapshot(ctx, sel, l)
	if err != nil {
		return snapshot, blobs, err
	}
	fail := func(e error) (migration.Snapshot, migration.BlobSource, error) {
		if c, ok := blobs.(io.Closer); ok {
			c.Close()
		}
		return migration.Snapshot{}, nil, e
	}
	if err := projectReadOnlySource(ctx, s.dir); err != nil {
		return fail(err)
	}
	native := migration.Snapshot{Schema: "hn.project-snapshot/1", Source: migration.Locator{Kind: "local"}, Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}
	roots := []string{}
	add := func(ref, oid, kind string) {
		native.NativeRefs = append(native.NativeRefs, migration.NativeRef{SourceRef: ref, OID: oid, Kind: kind})
		roots = append(roots, oid)
	}
	for _, ref := range sel.Refs {
		kind := migration.NativeKind(ref)
		if kind == "" || !migration.ValidRef(ref) {
			return fail(fmt.Errorf("unsupported explicit Git ref"))
		}
		oid, e := projectGit(ctx, s.dir, nil, 256, "show-ref", "--verify", "--hash", ref)
		if e != nil {
			return fail(e)
		}
		add(ref, strings.TrimSpace(string(oid)), kind)
	}
	seen := map[string]bool{}
	for _, record := range snapshot.Records {
		if record.Key.Kind != "change_request" {
			continue
		}
		pr, e := importedPullRequestRecord(ctx, blobs, record)
		if e != nil {
			return fail(e)
		}
		if pr.Number == 0 {
			continue
		}
		for _, oid := range []string{pr.Base, pr.Head, pr.Merge} {
			if seen[oid] || oid == "" {
				continue
			}
			seen[oid] = true
			if !migration.ValidOID(oid) {
				continue
			}
			out, e := projectGit(ctx, s.dir, []byte(oid+"\n"), 256, "cat-file", "--batch-check=%(objectname) %(objecttype)")
			if e != nil || string(out) != oid+" commit\n" {
				snapshot.Findings = append(snapshot.Findings, migration.Finding{ID: "pull-request/git/" + oid, Status: "missing_dependency", Path: "git", Message: "Pull request commit " + oid + " is absent from the supplied Git repository."})
				continue
			}
			add("refs/heads/hn-migration-audit/"+oid, oid, "code")
		}
	}
	if len(roots) == 0 {
		return snapshot, blobs, nil
	}
	if len(roots) > l.Refs {
		return fail(fmt.Errorf("pull request Git roots exceed migration ref budget"))
	}
	pinned, err := exportPinnedProjectSnapshot(ctx, s.dir, native, roots, l)
	if err != nil {
		return fail(err)
	}
	return ComposeMigrationSources(observedMigrationSource{snapshot, blobs}, observedMigrationSource{pinned.Snapshot, pinned}).Snapshot(ctx, sel, l)
}
