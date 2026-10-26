package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hubnot/internal/migration"
)

type projectTestBlobs map[string][]byte

func (b projectTestBlobs) Open(_ context.Context, d string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(b[d])), nil
}
func TestProjectSnapshotExactSignedHistoryAndIsolation(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	l := migration.DefaultLimits()
	id := testIdentity(t, "Offline author")
	event := newEvent(id, "issue.open", 1, "")
	event.Title = "Retained exact signature"
	stored, e := appendEvent(event, id)
	if e != nil {
		t.Fatal(e)
	}
	gitDir := filepath.Join(root, ".git")
	head, e := gitText("rev-parse", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	branch, e := gitText("symbolic-ref", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	secret := []byte("PRIVATE-STORE-SENTINEL")
	private := filepath.Join(gitDir, "hn", "private-sentinel")
	if e = os.MkdirAll(filepath.Dir(private), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(private, secret, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "untracked-sentinel"), secret, 0600); e != nil {
		t.Fatal(e)
	}
	unrelated, e := gitInput([]byte("UNSELECTED-OBJECT-SENTINEL"), nil, "hash-object", "-w", "--stdin")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = gitOutput("update-ref", "refs/hn/private/unselected", strings.TrimSpace(string(unrelated))); e != nil {
		t.Fatal(e)
	}
	refsBefore, _ := gitOutput("show-ref")
	p, e := ExportProjectSnapshot(ctx, gitDir, []string{branch, actorRef(id.Actor)}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	var out bytes.Buffer
	if e = migration.WriteArchive(ctx, &out, p.Snapshot, p, l); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(out.Bytes(), secret) || bytes.Contains(out.Bytes(), []byte("UNSELECTED-OBJECT-SENTINEL")) {
		t.Fatal("private/unselected state archived")
	}
	a, e := migration.ReadArchive(ctx, bytes.NewReader(out.Bytes()), l)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	q, e := StageProjectSnapshot(ctx, a.Snapshot, a, l)
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	got, e := loadStoredEventAt(q.GitDir, stored.Commit)
	if e != nil {
		t.Fatal(e)
	}
	if got.ID != stored.ID || !bytes.Equal(got.Payload, stored.Payload) || !bytes.Equal(got.Signature, stored.Signature) {
		t.Fatal("signed event bytes changed")
	}
	if _, e = gitOutputAt(q.GitDir, "cat-file", "-e", head+"^{commit}"); e != nil {
		t.Fatal("selected code absent")
	}
	qrefs, _ := gitTextAt(q.GitDir, "for-each-ref")
	if qrefs != "" {
		t.Fatal("quarantine acquired refs")
	}
	after, _ := gitOutput("show-ref")
	if !bytes.Equal(refsBefore, after) {
		t.Fatal("export/staging changed source refs")
	}
	if _, e = os.Stat(filepath.Join(q.GitDir, "hn")); !os.IsNotExist(e) {
		t.Fatal("quarantine installed identities, anchors or journals")
	}
	p2, e := ExportProjectSnapshot(ctx, gitDir, []string{actorRef(id.Actor), branch}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer p2.Close()
	if p.digest != p2.digest {
		t.Fatal("native object material not deterministic")
	}
}
func TestProjectSnapshotNativeDependencyAndHostileRefRefusal(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	gitDir := filepath.Join(root, ".git")
	author := testIdentity(t, "Author")
	issue := appendIdentityTestEvent(t, author, "issue.open", func(e *Event) { e.Title = "Issue" })
	commenter := testIdentity(t, "Commenter")
	appendIdentityTestEvent(t, commenter, "issue.comment", func(e *Event) { e.Subject = issue.ID; e.Body = "Reply" })
	before, _ := gitOutput("show-ref")
	for _, refs := range [][]string{nil, {"HEAD"}, {"refs/heads/../../escape"}, {"refs/hn/private/keys"}, {actorRef(commenter.Actor)}, {actorRef(author.Actor), actorRef(author.Actor)}} {
		if p, e := ExportProjectSnapshot(ctx, gitDir, refs, migration.DefaultLimits()); e == nil {
			p.Close()
			t.Fatalf("invalid selection accepted: %v", refs)
		}
	}
	after, _ := gitOutput("show-ref")
	if !bytes.Equal(before, after) {
		t.Fatal("failed snapshot changed refs")
	}
	p, e := ExportProjectSnapshot(ctx, gitDir, []string{actorRef(author.Actor), actorRef(commenter.Actor)}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	p.Close()
}
func TestProjectSnapshotNativeBudgetAndCorruptionRefusal(t *testing.T) {
	root := inReplicationTestRepository(t)
	ctx := context.Background()
	gitDir := filepath.Join(root, ".git")
	branch, _ := gitText("symbolic-ref", "HEAD")
	l := migration.DefaultLimits()
	p, e := ExportProjectSnapshot(ctx, gitDir, []string{branch}, l)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	r, e := p.Open(ctx, p.digest)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(r)
	r.Close()
	before, _ := gitOutput("show-ref")
	for _, name := range []string{"object count", "object bytes", "total bytes", "corrupt", "missing", "bomb header", "unrelated"} {
		t.Run(name, func(t *testing.T) {
			s := p.Snapshot
			budget := l
			b := append([]byte{}, raw...)
			switch name {
			case "object count":
				budget.GitObjects = 1
			case "object bytes":
				budget.GitObjectBytes = 1
			case "total bytes":
				budget.GitTotalBytes = 1
			case "corrupt":
				b[len(b)-2] ^= 1
			case "missing":
				b = b[:len(b)-2]
			case "bomb header":
				b = []byte(projectGitSchema + strings.Repeat("a", 40) + " blob 9223372036854775807\n")
			case "unrelated":
				b = append(b, []byte(strings.Repeat("f", 40)+" blob 0\n\n")...)
			}
			d := migration.Digest(b)
			s.GitBundle = d
			s.Blobs = []migration.Blob{{Digest: d, Size: int64(len(b)), MediaType: "application/vnd.hn.git-objects", Name: "selected"}}
			if q, e := StageProjectSnapshot(ctx, s, projectTestBlobs{d: b}, budget); e == nil {
				q.Close()
				t.Fatal("hostile native material accepted")
			}
		})
	}
	after, _ := gitOutput("show-ref")
	if !bytes.Equal(before, after) {
		t.Fatal("failed native staging changed accepted refs")
	}
}
func TestProjectSnapshotPreservesCommittedHistoryIncludingHistoricalPolicy(t *testing.T) {
	root := inReplicationTestRepository(t)
	sentinel := []byte("deliberately committed content")
	if e := os.WriteFile(filepath.Join(root, "selected.txt"), sentinel, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := gitOutput("add", "selected.txt"); e != nil {
		t.Fatal(e)
	}
	if _, e := gitOutput("commit", "-qm", "selected history"); e != nil {
		t.Fatal(e)
	}
	branch, _ := gitText("symbolic-ref", "HEAD")
	p, e := ExportProjectSnapshot(context.Background(), filepath.Join(root, ".git"), []string{branch}, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	q, e := StageProjectSnapshot(context.Background(), p.Snapshot, p, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	head, _ := gitText("rev-parse", "HEAD")
	raw, e := gitOutputAt(q.GitDir, "show", head+":selected.txt")
	if e != nil || !bytes.Equal(raw, sentinel) {
		t.Fatal("selected committed history scrubbed or omitted")
	}
	manifest, _ := json.Marshal(q.Snapshot)
	if bytes.Contains(manifest, []byte("PrivateKey")) {
		t.Fatal("private identity in manifest")
	}
}

func TestProjectSnapshotRetainsUnmergedProposalAndHistoricalPolicy(t *testing.T) {
	root := inReplicationTestRepository(t)
	base, _ := gitText("rev-parse", "HEAD")
	author := testIdentity(t, "Unmerged author")
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{author.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1, TrustedReviewers: []string{}}, Pipelines: map[string]PipelinePolicy{}}
	writeTestPolicy(t, root, policy)
	if e := os.WriteFile(filepath.Join(root, "unmerged.txt"), []byte("unmerged source"), 0600); e != nil {
		t.Fatal(e)
	}
	mustGit(t, "add", ".")
	mustGit(t, "commit", "-qm", "unmerged proposal policy")
	head, _ := gitText("rev-parse", "HEAD")
	proposal := appendPolicyTestProposal(t, author, base, head, "Unmerged snapshot")
	mustGit(t, "update-ref", "refs/heads/main", base)
	mustGit(t, "reset", "--hard", "-q", base)
	refs := []string{"refs/heads/main", actorRef(author.Actor), proposalRef(proposal.ID)}
	p, e := ExportProjectSnapshot(context.Background(), filepath.Join(root, ".git"), refs, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	q, e := StageProjectSnapshot(context.Background(), p.Snapshot, p, migration.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	if _, e = gitOutputAt(q.GitDir, "show", head+":unmerged.txt"); e != nil {
		t.Fatal("unmerged code missing")
	}
	raw, e := gitOutputAt(q.GitDir, "show", head+":.hn/policy.json")
	if e != nil {
		t.Fatal("historical policy missing")
	}
	if _, _, e = parsePolicyBytes(raw); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, ".hn", "policy.json")); !os.IsNotExist(e) {
		t.Fatal("source historical governance adopted")
	}
	if p, e := ExportProjectSnapshot(context.Background(), filepath.Join(root, ".git"), refs[:2], migration.DefaultLimits()); e == nil {
		p.Close()
		t.Fatal("missing proposal code ref accepted")
	}
}
func TestProjectSnapshotRefusesAmbientObjectOverride(t *testing.T) {
	root := inReplicationTestRepository(t)
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", t.TempDir())
	if p, e := ExportProjectSnapshot(context.Background(), filepath.Join(root, ".git"), []string{"refs/heads/main"}, migration.DefaultLimits()); e == nil {
		p.Close()
		t.Fatal("ambient object override accepted")
	}
}
