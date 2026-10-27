package migration

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"hubnot/internal/safepath"
)

const ArchiveSchema = "hn.project-archive/1"

type archiveManifest struct {
	Schema   string   `json:"schema"`
	Snapshot Snapshot `json:"snapshot"`
}

// Archive is a verified private spool. Native material still requires native
// admission in a bare quarantine before the apply engine may promote any ref.
type Archive struct {
	Snapshot Snapshot
	root     string
	blobs    map[string]Blob
}

func (a *Archive) Close() error { return os.RemoveAll(a.root) }
func (a *Archive) Open(ctx context.Context, digest string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if _, ok := a.blobs[digest]; !ok {
		return nil, fmt.Errorf("undeclared archive blob")
	}
	return os.Open(filepath.Join(a.root, digest[7:]))
}
func blobPath(d string) string { return "blobs/sha256/" + d[7:] }

// WriteArchive verifies and spools all bytes before writing deterministic tar.
// A writer failure can leave a partial output; callers publish their file only
// after this function succeeds (for example via an atomic rename).
func WriteArchive(ctx context.Context, w io.Writer, s Snapshot, source BlobSource, l Limits) error {
	raw, e := CanonicalSnapshot(s, l)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &s); e != nil {
		return e
	}
	a, e := newArchive(s)
	if e != nil {
		return e
	}
	defer a.Close()
	for _, b := range s.Blobs {
		if source == nil {
			return fmt.Errorf("missing blob source")
		}
		r, e := source.Open(ctx, b.Digest)
		if e != nil {
			return e
		}
		e = spoolBlob(ctx, a.root, b, r)
		ce := r.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	manifest, e := json.Marshal(archiveManifest{ArchiveSchema, s})
	if e != nil {
		return e
	}
	if int64(len(manifest)) > l.ManifestBytes {
		return fmt.Errorf("archive manifest budget exceeded")
	}
	tw := tar.NewWriter(w)
	write := func(name string, size int64, r io.Reader) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if e := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: size, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); e != nil {
			return e
		}
		_, e := io.Copy(tw, r)
		return e
	}
	if e = write("manifest.json", int64(len(manifest)), bytes.NewReader(manifest)); e != nil {
		return e
	}
	for _, b := range s.Blobs {
		r, e := a.Open(ctx, b.Digest)
		if e != nil {
			return e
		}
		e = write(blobPath(b.Digest), b.Size, r)
		ce := r.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	return tw.Close()
}
func newArchive(s Snapshot) (*Archive, error) {
	root, e := safepath.MkdirTemp("hn-project-archive-")
	if e != nil {
		return nil, e
	}
	a := &Archive{Snapshot: s, root: root, blobs: map[string]Blob{}}
	for _, b := range s.Blobs {
		a.blobs[b.Digest] = b
	}
	return a, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func spoolBlob(ctx context.Context, root string, b Blob, r io.Reader) error {
	f, e := os.OpenFile(filepath.Join(root, b.Digest[7:]), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	h := newDigestWriter(f)
	n, e := io.Copy(h, io.LimitReader(contextReader{ctx, r}, b.Size+1))
	if e != nil {
		return e
	}
	if n != b.Size || h.digest() != b.Digest {
		return fmt.Errorf("blob size/digest mismatch")
	}
	return f.Close()
}

// ReadArchive accepts only the canonical regular-file tar member set. No member
// name is ever used as an extraction path. Failure removes the entire spool.
func ReadArchive(ctx context.Context, r io.Reader, l Limits) (result *Archive, err error) {
	if e := l.Validate(); e != nil {
		return nil, e
	}
	// Bound the entire wire stream as well as expanded member payloads. This
	// includes hidden tar metadata, padding and attempted extra members.
	r = &archiveBudgetReader{r: contextReader{ctx, r}, left: l.ManifestBytes + l.AttachmentBytes + GitMaterialLimit(l) + (int64(l.Blobs)+2)*1024 + 11264}
	tr := tar.NewReader(r)
	h, e := tr.Next()
	if e != nil {
		return nil, e
	}
	if h.Name != "manifest.json" || !regularHeader(h) || h.Size < 1 || h.Size > l.ManifestBytes {
		return nil, fmt.Errorf("invalid archive manifest member")
	}
	raw, e := io.ReadAll(io.LimitReader(tr, l.ManifestBytes+1))
	if e != nil {
		return nil, e
	}
	var m archiveManifest
	if e = StrictJSON(raw, &m); e != nil {
		return nil, e
	}
	if m.Schema != ArchiveSchema {
		return nil, fmt.Errorf("unsupported archive schema")
	}
	canonical, e := CanonicalSnapshot(m.Snapshot, l)
	if e != nil {
		return nil, e
	}
	_ = json.Unmarshal(canonical, &m.Snapshot)
	expected, e := json.Marshal(archiveManifest{ArchiveSchema, m.Snapshot})
	if e != nil || !bytes.Equal(raw, expected) {
		return nil, fmt.Errorf("manifest is not canonical")
	}
	a, e := newArchive(m.Snapshot)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			_ = a.Close()
		}
	}()
	seen := map[string]bool{}
	for {
		h, e = tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if !regularHeader(h) {
			return nil, fmt.Errorf("links/special archive members refused")
		}
		var b Blob
		found := false
		for _, candidate := range a.Snapshot.Blobs {
			if h.Name == blobPath(candidate.Digest) {
				b = candidate
				found = true
				break
			}
		}
		if !found || seen[h.Name] || h.Size != b.Size {
			return nil, fmt.Errorf("unexpected/duplicate/incorrect-size archive member")
		}
		seen[h.Name] = true
		if e = spoolBlob(ctx, a.root, b, tr); e != nil {
			return nil, e
		}
	}
	if len(seen) != len(a.blobs) {
		return nil, fmt.Errorf("missing archive blob")
	}
	// tar.Reader stops at its end markers; reject concatenated archives/trailing junk.
	// Remaining zero padding is bounded to one conventional blocking record.
	tail, e := io.ReadAll(io.LimitReader(contextReader{ctx, r}, 10241))
	if e != nil {
		return nil, e
	}
	if len(tail) > 10240 || len(bytes.Trim(tail, "\x00")) != 0 {
		return nil, fmt.Errorf("trailing archive data")
	}
	return a, nil
}
func regularHeader(h *tar.Header) bool {
	return (h.Typeflag == tar.TypeReg || h.Typeflag == tar.TypeRegA) && h.Linkname == "" && len(h.PAXRecords) == 0 && h.Format == tar.FormatUSTAR && h.Size >= 0
}

type archiveBudgetReader struct {
	r    io.Reader
	left int64
}

func (r *archiveBudgetReader) Read(p []byte) (int, error) {
	if r.left <= 0 {
		return 0, fmt.Errorf("archive wire budget exceeded")
	}
	if int64(len(p)) > r.left {
		p = p[:r.left]
	}
	n, e := r.r.Read(p)
	r.left -= int64(n)
	return n, e
}

type digestWriter struct {
	w io.Writer
	h hash.Hash
}

func newDigestWriter(w io.Writer) *digestWriter { return &digestWriter{w, sha256.New()} }
func (d *digestWriter) Write(p []byte) (int, error) {
	n, e := d.w.Write(p)
	_, _ = d.h.Write(p[:n])
	return n, e
}
func (d *digestWriter) digest() string { return "sha256:" + hex.EncodeToString(d.h.Sum(nil)) }
