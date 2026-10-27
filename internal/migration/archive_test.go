package migration

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type memoryBlobs map[string][]byte

func (m memoryBlobs) Open(_ context.Context, d string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m[d])), nil
}
func fixture(t *testing.T) (Snapshot, memoryBlobs) {
	t.Helper()
	raw := []byte(`{"id":9007199254740993,"review":"APPROVED","permissions":["admin"]}`)
	d := Digest(raw)
	issue := Key{"gitlab", "https://forge.test", "project-immutable", "issue", "42"}
	comment := Key{"gitlab", "https://forge.test", "project-immutable", "comment", "99"}
	content, _ := json.Marshal(IssueContent{"hn.migration.issue/1", "Title", "Body", "opened", "", []string{"label"}, []string{}})
	note, _ := json.Marshal(CommentContent{"hn.migration.comment/1", "Reply"})
	preserved, _ := json.Marshal(PreservedContent{"hn.migration.preserved/1", "An attributed review", d})
	return Snapshot{Schema: "hn.project-snapshot/1", Source: Locator{Kind: "gitlab", Endpoint: "https://forge.test", Project: "group/project"}, SourceID: "immutable-project", Records: []Record{{Key: issue, Revision: "rev1", Author: "source author", Schema: "hn.migration.issue/1", Content: content}, {Key: comment, Revision: "rev2", Schema: "hn.migration.comment/1", Content: note, Links: []Link{{Relation: "issue", Target: issue}}}, {Key: Key{"gitlab", "https://forge.test", "project-immutable", "review", "100"}, Revision: "rev3", Schema: "hn.migration.preserved/1", Content: preserved, Links: []Link{{Relation: "issue", Target: issue}}, Blobs: []string{d}}}, Blobs: []Blob{{d, int64(len(raw)), "application/json", "../../source-name.json"}}}, memoryBlobs{d: raw}
}
func archiveBytes(t *testing.T, s Snapshot, b BlobSource, l Limits) []byte {
	t.Helper()
	var out bytes.Buffer
	if e := WriteArchive(context.Background(), &out, s, b, l); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestArchiveRoundTripDeterministicAndAttributed(t *testing.T) {
	s, b := fixture(t)
	l := DefaultLimits()
	one := archiveBytes(t, s, b, l)
	s.Records[0], s.Records[2] = s.Records[2], s.Records[0]
	two := archiveBytes(t, s, b, l)
	if !bytes.Equal(one, two) {
		t.Fatal("archive depends on input order")
	}
	a, e := ReadArchive(context.Background(), bytes.NewReader(one), l)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	for d, want := range b {
		r, e := a.Open(context.Background(), d)
		if e != nil {
			t.Fatal(e)
		}
		got, _ := io.ReadAll(r)
		r.Close()
		if !bytes.Equal(got, want) {
			t.Fatal("retrieved review bytes/numeric precision changed")
		}
	}
	if len(a.Snapshot.Records) != 3 || a.Snapshot.Records[0].Key.Provider != "gitlab" {
		t.Fatal("attribution/topology lost")
	}
	if _, e = a.Open(context.Background(), Digest([]byte("ambient"))); e == nil {
		t.Fatal("undeclared blob opened")
	}
}
func TestClosedIssueAndCommentContent(t *testing.T) {
	good := `{"schema":"hn.migration.issue/1","title":"Title","body":"Body","state":"open","type":"","labels":[],"assignees":[]}`
	for _, raw := range []string{strings.Replace(good, `"title":"Title"`, `"title":"Title","title":"other"`, 1), strings.Replace(good, `"title"`, `"Title"`, 1), strings.Replace(good, `"title":"Title"`, `"title":null`, 1), strings.Replace(good, `"Title"`, `"\ud800"`, 1), strings.Replace(good, `"title":"Title"`, `"title":"Title","admin":true`, 1), strings.Replace(good, `"labels":[]`, `"labels":null`, 1), strings.Replace(good, `"labels":[]`, `"labels":["a","a"]`, 1), good + `{}`, strings.Replace(good, `"title":"Title",`, "", 1)} {
		if _, e := CanonicalContent("issue", []byte(raw)); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, e := CanonicalContent("issue", []byte(good)); e != nil {
		t.Fatal(e)
	}
	if _, e := CanonicalContent("comment", []byte(`{"schema":"hn.migration.comment/1","body":"\ud83d\ude00"}`)); e != nil {
		t.Fatal(e)
	}
}
func TestSnapshotRefusesDependencyDigestAndBudgets(t *testing.T) {
	for _, name := range []string{"zero", "records", "attachments", "blob size", "missing parent", "wrong parent", "duplicate record", "duplicate blob", "native attachment alias", "missing blob", "wrong schema", "credentials"} {
		t.Run(name, func(t *testing.T) {
			s, b := fixture(t)
			l := DefaultLimits()
			switch name {
			case "zero":
				l.Refs = 0
			case "records":
				l.Records = 1
			case "attachments":
				l.AttachmentBytes = 1
			case "blob size":
				l.BlobBytes = 1
			case "missing parent":
				s.Records[1].Links = nil
			case "wrong parent":
				s.Records[1].Links[0].Target.Kind = "review"
			case "duplicate record":
				s.Records = append(s.Records, s.Records[0])
			case "duplicate blob":
				s.Blobs = append(s.Blobs, s.Blobs[0])
			case "native attachment alias":
				s.GitBundle = s.Blobs[0].Digest
				s.NativeRefs = []NativeRef{{"refs/heads/main", strings.Repeat("a", 40), "code"}}
			case "missing blob":
				s.Blobs = nil
			case "wrong schema":
				s.Records[0].Schema = "hn.migration.comment/1"
			case "credentials":
				s.Source.Endpoint = "https://user:token@forge.test"
			}
			var out bytes.Buffer
			if e := WriteArchive(context.Background(), &out, s, b, l); e == nil {
				t.Fatal("invalid snapshot accepted")
			}
			if out.Len() != 0 {
				t.Fatal("invalid export published partial bytes")
			}
		})
	}
}
func rewriteArchive(t *testing.T, raw []byte, edit func(*tar.Header, []byte) (*tar.Header, []byte), extra bool) []byte {
	t.Helper()
	var out bytes.Buffer
	tr := tar.NewReader(bytes.NewReader(raw))
	tw := tar.NewWriter(&out)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		b, _ := io.ReadAll(tr)
		h, b = edit(h, b)
		if h == nil {
			continue
		}
		if e = tw.WriteHeader(h); e != nil {
			t.Fatal(e)
		}
		if _, e = tw.Write(b); e != nil {
			t.Fatal(e)
		}
		if extra && h.Name != "manifest.json" {
			if e = tw.WriteHeader(h); e != nil {
				t.Fatal(e)
			}
			_, _ = tw.Write(b)
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestArchiveHostileMemberMatrix(t *testing.T) {
	s, b := fixture(t)
	l := DefaultLimits()
	raw := archiveBytes(t, s, b, l)
	for _, name := range []string{"traversal", "absolute", "symlink", "hardlink", "unknown", "duplicate", "digest", "missing", "oversize", "trailing", "gzip"} {
		t.Run(name, func(t *testing.T) {
			bad := rewriteArchive(t, raw, func(h *tar.Header, p []byte) (*tar.Header, []byte) {
				if h.Name == "manifest.json" {
					return h, p
				}
				switch name {
				case "traversal":
					h.Name = "../escape"
				case "absolute":
					h.Name = "/escape"
				case "symlink":
					h.Typeflag = tar.TypeSymlink
					h.Linkname = "/escape"
					h.Size = 0
					p = nil
				case "hardlink":
					h.Typeflag = tar.TypeLink
					h.Linkname = "manifest.json"
					h.Size = 0
					p = nil
				case "unknown":
					h.Name = "unknown"
				case "digest":
					p[0] ^= 1
				case "missing":
					return nil, nil
				case "oversize":
					h.Size++
					p = append(p, 0)
				}
				return h, p
			}, name == "duplicate")
			if name == "trailing" {
				bad = append(bad, 1)
			}
			if name == "gzip" {
				bad = []byte{0x1f, 0x8b, 0x08, 0}
			}
			a, e := ReadArchive(context.Background(), bytes.NewReader(bad), l)
			if e == nil {
				a.Close()
				t.Fatal("hostile archive accepted")
			}
		})
	}
}
func TestArchiveVerifiesBeforeWriting(t *testing.T) {
	s, b := fixture(t)
	for d := range b {
		b[d] = []byte("corrupt")
	}
	var out bytes.Buffer
	if e := WriteArchive(context.Background(), &out, s, b, DefaultLimits()); e == nil || out.Len() != 0 {
		t.Fatal("corrupt blob produced visible archive bytes")
	}
}

func TestSnapshotPreservesSourceCommentAnchor(t *testing.T) {
	s, b := fixture(t)
	want := "https://forge.test/group/project/-/issues/42#note_99"
	s.Records[1].SourceURL = want
	a, e := ReadArchive(context.Background(), bytes.NewReader(archiveBytes(t, s, b, DefaultLimits())), DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	for _, r := range a.Snapshot.Records {
		if r.Key.Kind == "comment" && r.SourceURL != want {
			t.Fatal("source comment anchor changed")
		}
	}
	if _, e = NormalizeInstance(want); e == nil {
		t.Fatal("comment anchor accepted as immutable provider instance")
	}
}
