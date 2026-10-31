package main

// hn.contribution-artifact/1 is a local regular-file handoff, not a Git fetch
// protocol. It assumes an ordinary local filesystem: deadline checks do not
// promise cancellation of a blocking kernel/NFS operation. Framing is magic, big-endian uint64 JSON-manifest size, canonical
// manifest, then exactly one uncompressed hn.project-archive/1. Envelopes retain
// exact payload/signature bytes. Native selection is exact, never compatibility
// all. Memory and compressed audit attachments are deliberately unsupported.
//
// Receive's declared private side effects are a monotone receiver-local budget
// reservation, measured status journal, and an immutable original artifact.
// Failures and uncertainty stay charged. No policy, key, authority, native ref,
// branch, checkout, hook, pipeline, review or merge follows staging. Callers must
// separately authorize admission; portable receipts cannot spend local capacity.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	c "hubnot/internal/contribution"
	"hubnot/internal/migration"
	"hubnot/internal/safepath"
)

const contributionArtifactMagic = "hn.contribution-artifact/1\n"
const contributionJournalVersion = "hn.contribution-intake/1"
const contributionManifestMax = 4 << 20
const contributionJournalMax = 80 << 20

type contributionWireEnvelope struct {
	Payload   []byte `json:"payload"`
	Signature []byte `json:"signature"`
}

type contributionManifest struct {
	Project   string                     `json:"project"`
	Refs      []string                   `json:"refs"`
	Envelopes []contributionWireEnvelope `json:"envelopes"`
}

// ContributionNativeSource contains only independently authenticated native
// facts. It cannot be populated by envelope classifications supplied on wire.
type ContributionNativeSource struct{ facts map[string]c.Fact }

func (s ContributionNativeSource) Fact(id string) (c.Fact, error) {
	f, ok := s.facts[id]
	if !ok {
		return c.Fact{}, c.ErrMissingDependency
	}
	f.Content.Parents = append([]string{}, f.Content.Parents...)
	f.Content.References = append([]c.Reference{}, f.Content.References...)
	return f, nil
}

// ContributionArtifact owns a disposable, verified bare quarantine. Close it
// after the caller finishes raw inspection or independently authorized promotion.
type ContributionArtifact struct {
	// Intake is receiver-local provenance when inspected from a durable inbox.
	// Only pending is eligible for a future authorized intake attestation;
	// conflict/uncertain/failed never supply admission provenance.
	Intake       *ContributionIntake
	Project      string
	Refs         []migration.NativeRef
	Records      []c.Verified
	Events       []StoredEvent
	Native       ContributionNativeSource
	Quarantine   *ProjectQuarantine
	WireBytes    int64
	DecodedBytes int64
	Objects      int64
	EventCount   int64
	Depth        int64
}

func (a *ContributionArtifact) Close() error {
	if a.Quarantine != nil {
		return a.Quarantine.Close()
	}
	return nil
}

func contributionLimits(l c.Limits) (migration.Limits, error) {
	if e := l.Validate(); e != nil {
		return migration.Limits{}, e
	}
	// These implementation ceilings also bound integer conversion, allocation and
	// duration arithmetic on every host, even if a policy selects extreme values.
	if l.WireBytes > 1<<30 || l.DecodedBytes > 1<<30 || l.Objects > c.MaxGraphRecords || l.Events > c.MaxGraphRecords || l.Depth > c.MaxGraphRecords || l.Milliseconds > 3600000 {
		return migration.Limits{}, fmt.Errorf("unsupported exchange limit range")
	}
	n := int(l.Objects)
	return migration.Limits{Records: int(l.Events), Blobs: n, Refs: n, GitObjects: n, BlobBytes: l.DecodedBytes, ManifestBytes: min(l.DecodedBytes, int64(contributionManifestMax)), AttachmentBytes: l.DecodedBytes, GitObjectBytes: l.DecodedBytes, GitTotalBytes: l.DecodedBytes}, nil
}
func contributionContext(ctx context.Context, l c.Limits) (context.Context, context.CancelFunc, error) {
	if _, e := contributionLimits(l); e != nil {
		return nil, nil, e
	}
	child, cancel := context.WithTimeout(ctx, time.Duration(l.Milliseconds)*time.Millisecond)
	return child, cancel, nil
}
func contributionRefs(refs []string) ([]string, error) {
	if len(refs) == 0 || len(refs) > c.MaxGraphRecords {
		return nil, fmt.Errorf("explicit native refs required")
	}
	out := append([]string{}, refs...)
	sort.Strings(out)
	for i, r := range out {
		kind := migration.NativeKind(r)
		if !migration.ValidRef(r) || kind == "" || kind == "memory" || i > 0 && out[i-1] == r {
			return nil, fmt.Errorf("unsupported/duplicate contribution ref")
		}
	}
	return out, nil
}
func contributionSameRefs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func contributionStrict(raw []byte, v any) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8")
	}
	depth := 0
	quoted := false
	escaped := false
	for _, b := range raw {
		if quoted {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				quoted = false
			}
			continue
		}
		switch b {
		case '"':
			quoted = true
		case '{', '[':
			depth++
			if depth > 32 {
				return fmt.Errorf("JSON depth limit")
			}
		case '}', ']':
			depth--
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return e
	}
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(canonical, raw) {
		return fmt.Errorf("noncanonical exchange JSON")
	}
	return nil
}

func contributionManifestBytes(project string, refs []string, envelopes []c.Envelope, l c.Limits) ([]byte, error) {
	selected, e := contributionRefs(refs)
	if e != nil {
		return nil, e
	}
	if !migration.ValidDigest(project) || len(envelopes) == 0 || int64(len(envelopes)) > l.Events {
		return nil, fmt.Errorf("invalid envelope selection")
	}
	prior := ""
	sorted := append([]c.Envelope{}, envelopes...)
	sort.Slice(sorted, func(i, j int) bool { return c.Digest(sorted[i].Payload) < c.Digest(sorted[j].Payload) })
	for _, env := range sorted {
		v, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return nil, e
		}
		r := v.Record()
		if r.Project != project || v.ID() == prior {
			return nil, fmt.Errorf("wrong project/duplicate envelope")
		}
		prior = v.ID()
	}
	wire := make([]contributionWireEnvelope, 0, len(sorted))
	for _, env := range sorted {
		wire = append(wire, contributionWireEnvelope{env.Payload, env.Signature})
	}
	raw, e := json.Marshal(contributionManifest{project, selected, wire})
	if e != nil {
		return nil, e
	}
	if len(raw) > contributionManifestMax || int64(len(raw)) > l.DecodedBytes {
		return nil, fmt.Errorf("contribution manifest limit")
	}
	return raw, nil
}

// ExportContributionArtifact needs no project role, remote account, push rights
// or selected local authority. It exports only explicit complete native closure.
// Export and intake use the same deadline-aware restricted native profile.
func ExportContributionArtifact(ctx context.Context, gitDir, project string, refs []string, envelopes []c.Envelope, path string, l c.Limits) (string, error) {
	ctx, cancel, e := contributionContext(ctx, l)
	if e != nil {
		return "", e
	}
	defer cancel()
	ml, e := contributionLimits(l)
	if e != nil {
		return "", e
	}
	raw, e := contributionManifestBytes(project, refs, envelopes, l)
	if e != nil {
		return "", e
	}
	selected, _ := contributionRefs(refs)
	if e = contributionSafeGitDir(gitDir); e != nil {
		return "", e
	}
	p, e := exportContributionSnapshot(ctx, gitDir, selected, ml)
	if e != nil {
		return "", e
	}
	defer p.Close()
	var inner bytes.Buffer
	if e = migration.WriteArchive(ctx, &inner, p.Snapshot, p, ml); e != nil {
		return "", e
	}
	var outer bytes.Buffer
	outer.WriteString(contributionArtifactMagic)
	_ = binary.Write(&outer, binary.BigEndian, uint64(len(raw)))
	outer.Write(raw)
	outer.Write(inner.Bytes())
	if int64(outer.Len()) > l.WireBytes {
		return "", fmt.Errorf("contribution wire limit")
	}
	// Verify exactly the supported intake shape before publishing the artifact.
	a, e := readContributionArtifact(ctx, bytes.NewReader(outer.Bytes()), project, selected, l)
	if e != nil {
		return "", e
	}
	_ = a.Close()
	if e = contributionPublish(path, outer.Bytes()); e != nil {
		return "", e
	}
	return c.Digest(outer.Bytes()), nil
}
func contributionSafeGitDir(dir string) error {
	abs, e := safepath.Canonical(dir)
	if e != nil {
		return e
	}
	if e = safepath.CheckDir(abs, false); e != nil {
		return e
	}
	// Known physical Git dirs only. Alternate stores, partial clones and local
	// includes/config can run a remote helper; exported sources must be ordinary.
	if e = projectSafeEnvironment(); e != nil {
		return e
	}
	if e = safepath.CheckDir(filepath.Join(abs, "objects"), false); e != nil {
		return e
	}
	for _, name := range []string{"shallow", filepath.Join("info", "grafts")} {
		if _, e = os.Lstat(filepath.Join(abs, name)); !os.IsNotExist(e) {
			return fmt.Errorf("partial/grafted Git source refused")
		}
	}
	config, e := contributionOpenRegular(filepath.Join(abs, "config"), false)
	if e != nil {
		return e
	}
	configInfo, e := config.Stat()
	if e != nil || configInfo.Size() > 1<<20 {
		config.Close()
		return fmt.Errorf("source Git config exceeds supported bounds")
	}
	raw, e := io.ReadAll(io.LimitReader(config, 1<<20))
	_ = config.Close()
	if e != nil {
		return e
	}
	lower := strings.ToLower(string(raw))
	for _, bad := range []string{"include", "promisor", "partialclone", "alternate"} {
		if strings.Contains(lower, bad) {
			return fmt.Errorf("unsupported source Git configuration")
		}
	}
	if _, e = os.Lstat(filepath.Join(abs, "objects", "info", "alternates")); !os.IsNotExist(e) {
		return fmt.Errorf("alternate object source refused")
	}
	return nil
}
func contributionPublish(path string, raw []byte) error {
	p, e := safepath.Canonical(path)
	if e != nil {
		return e
	}
	if e = safepath.CheckDir(filepath.Dir(p), false); e != nil {
		return e
	}
	if _, e = os.Lstat(p); !os.IsNotExist(e) {
		return fmt.Errorf("output must be absent")
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".hn-contribution-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(raw); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	// Link is an atomic no-replace publish; never overwrite a raced-in file.
	if e = os.Link(f.Name(), p); e != nil {
		return e
	}
	return syncPrivateDirectory(filepath.Dir(p))
}

type contributionReader struct {
	ctx     context.Context
	r       io.Reader
	left, n int64
}

func (r *contributionReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	if r.left == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.left {
		p = p[:r.left]
	}
	n, e := r.r.Read(p)
	r.left -= int64(n)
	r.n += int64(n)
	return n, e
}

func readContributionArtifact(ctx context.Context, r io.Reader, project string, selected []string, l c.Limits) (result *ContributionArtifact, err error) {
	ml, e := contributionLimits(l)
	if e != nil {
		return nil, e
	}
	bounded := &contributionReader{ctx: ctx, r: r, left: l.WireBytes}
	header := make([]byte, len(contributionArtifactMagic)+8)
	if _, e = io.ReadFull(bounded, header); e != nil {
		return nil, e
	}
	if string(header[:len(contributionArtifactMagic)]) != contributionArtifactMagic {
		return nil, fmt.Errorf("unsupported contribution framing")
	}
	n := binary.BigEndian.Uint64(header[len(contributionArtifactMagic):])
	if n == 0 || n > contributionManifestMax || n > uint64(l.DecodedBytes) || n > uint64(bounded.left) {
		return nil, fmt.Errorf("contribution manifest budget")
	}
	raw := make([]byte, n)
	if _, e = io.ReadFull(bounded, raw); e != nil {
		return nil, e
	}
	var m contributionManifest
	if e = contributionStrict(raw, &m); e != nil {
		return nil, e
	}
	envelopes := make([]c.Envelope, 0, len(m.Envelopes))
	for _, env := range m.Envelopes {
		envelopes = append(envelopes, c.Envelope{Payload: env.Payload, Signature: env.Signature})
	}
	canonical, e := contributionManifestBytes(m.Project, m.Refs, envelopes, l)
	if e != nil || !bytes.Equal(raw, canonical) {
		return nil, fmt.Errorf("invalid canonical contribution manifest: %v", e)
	}
	if m.Project != project || !contributionSameRefs(m.Refs, selected) {
		return nil, fmt.Errorf("unselected project or refs")
	}
	a, e := migration.ReadArchive(ctx, bounded, ml)
	if e != nil {
		return nil, e
	}
	defer a.Close()
	if len(a.Snapshot.NativeRefs) != len(selected) || len(a.Snapshot.Records) != 0 || len(a.Snapshot.Blobs) != 1 || a.Snapshot.GitBundle == "" {
		return nil, fmt.Errorf("unsupported native archive shape")
	}
	for i, ref := range a.Snapshot.NativeRefs {
		if ref.SourceRef != selected[i] || ref.Kind == "memory" {
			return nil, fmt.Errorf("unselected inner ref")
		}
	}
	if a.Snapshot.Blobs[0].Digest != a.Snapshot.GitBundle {
		return nil, fmt.Errorf("unexpected attachment")
	}
	out := &ContributionArtifact{Project: project, Refs: append([]migration.NativeRef{}, a.Snapshot.NativeRefs...), WireBytes: bounded.n, DecodedBytes: int64(len(raw)) + a.Snapshot.Blobs[0].Size}
	if out.DecodedBytes > l.DecodedBytes {
		return nil, fmt.Errorf("decoded contribution limit")
	}
	for _, env := range m.Envelopes {
		v, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return nil, e
		}
		out.Records = append(out.Records, v)
	}
	actors := map[string]bool{}
	for _, v := range out.Records {
		actors[v.Record().Actor] = true
	}
	overheadObjects := int64(len(out.Records))*2 + int64(len(actors))*3
	overheadBytes := int64(len(actors))*4096 + int64(len(out.Records))*512
	if out.DecodedBytes+overheadBytes > l.DecodedBytes {
		return nil, fmt.Errorf("native plus manifest retention byte limit")
	}
	q, events, depth, e := stageContributionNative(ctx, a, ml, l, len(out.Records), overheadObjects)
	if e != nil {
		return nil, e
	}
	out.Quarantine = q
	defer func() {
		if err != nil {
			_ = out.Close()
		}
	}()
	out.Events = events
	out.Depth = depth
	out.Objects = int64(len(q.objectIDs)) + overheadObjects
	out.DecodedBytes += overheadBytes
	if out.DecodedBytes > l.DecodedBytes {
		return nil, fmt.Errorf("native plus manifest retention byte limit")
	}
	out.EventCount = int64(len(events) + len(out.Records))
	if out.Objects > l.Objects || out.EventCount > l.Events {
		return nil, fmt.Errorf("aggregate native/envelope count limit")
	}
	out.Native, e = classifyContributionFacts(events)
	if e != nil {
		return nil, e
	}
	for _, v := range out.Records {
		record := v.Record()
		if record.Submission == nil {
			continue
		}
		f, e := out.Native.Fact(record.Submission.Content.Revision)
		if e != nil {
			return nil, e
		}
		expected, _ := json.Marshal(f.Content)
		actual, _ := json.Marshal(record.Submission.Content)
		if f.Actor != record.Actor || f.Operation != record.Submission.Operation || !bytes.Equal(expected, actual) {
			return nil, fmt.Errorf("submission differs from authenticated native content")
		}
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}

// stageContributionNative deliberately uses the ordinary uncompressed object
// primitives but not StageProjectSnapshot's audit/memory native readers: those
// legacy paths include subprocesses without context and nested pack expansion.
// Signature and native relationship truth still come from the existing native
// verifiers. Every subprocess here uses projectGit's context/output boundary.
func stageContributionNative(ctx context.Context, a *migration.Archive, ml migration.Limits, l c.Limits, envelopes int, envelopeObjects int64) (result *ProjectQuarantine, events []StoredEvent, depth int64, err error) {
	r, e := a.Open(ctx, a.Snapshot.GitBundle)
	if e != nil {
		return nil, nil, 0, e
	}
	raw, e := io.ReadAll(&contributionReader{ctx: ctx, r: r, left: l.DecodedBytes + 1})
	_ = r.Close()
	if e != nil || int64(len(raw)) > l.DecodedBytes {
		return nil, nil, 0, fmt.Errorf("native decoded budget: %v", e)
	}
	count, objectDepth, e := contributionObjectPreflight(ctx, raw, l)
	if e != nil {
		return nil, nil, 0, e
	}
	if count+envelopeObjects > l.Objects {
		return nil, nil, 0, fmt.Errorf("native object count limit")
	}
	dir, e := safepath.MkdirTemp("hn-contribution-quarantine-")
	if e != nil {
		return nil, nil, 0, e
	}
	q := &ProjectQuarantine{GitDir: dir, Snapshot: a.Snapshot}
	defer func() {
		if err != nil {
			_ = q.Close()
		}
	}()
	format := "sha1"
	if len(a.Snapshot.NativeRefs[0].OID) == 64 {
		format = "sha256"
	}
	if _, e = projectGit(ctx, "", nil, 4096, "-c", "init.templateDir=", "init", "--bare", "--quiet", "--object-format="+format, dir); e != nil {
		return nil, nil, 0, e
	}
	q.objectIDs, e = projectReadObjects(ctx, dir, bytes.NewReader(raw), ml)
	if e != nil {
		return nil, nil, 0, e
	}
	if _, e = projectGit(ctx, dir, nil, 1<<20, "fsck", "--strict", "--full", "--no-reflogs", "--no-dangling"); e != nil {
		return nil, nil, 0, e
	}
	seen := map[string]bool{}
	byID := map[string]StoredEvent{}
	code := map[string]string{}
	for _, ref := range a.Snapshot.NativeRefs {
		if _, e = projectGit(ctx, dir, nil, 256, "rev-parse", "--verify", ref.OID+"^{commit}"); e != nil {
			return nil, nil, 0, e
		}
		q.roots = append(q.roots, ref.OID)
		switch ref.Kind {
		case "actor":
			chain, e := projectGit(ctx, dir, nil, l.Events*65+1, "rev-list", "--reverse", ref.OID)
			if e != nil {
				return nil, nil, 0, e
			}
			commits := strings.Fields(string(chain))
			if len(commits) == 0 || int64(len(commits)) > l.Events {
				return nil, nil, 0, fmt.Errorf("native actor event count")
			}
			previous := ""
			for _, commit := range commits {
				if e = ctx.Err(); e != nil {
					return nil, nil, 0, e
				}
				tree, e := projectGit(ctx, dir, nil, l.DecodedBytes, "ls-tree", "-z", commit)
				if e != nil {
					return nil, nil, 0, e
				}
				names := map[string]bool{}
				for _, entry := range strings.Split(string(tree), "\x00") {
					if entry == "" {
						continue
					}
					meta, name, ok := strings.Cut(entry, "\t")
					f := strings.Fields(meta)
					if !ok || len(f) != 3 || f[0] != "100644" || f[1] != "blob" || names[name] || (name != "event.json" && name != "signature") {
						return nil, nil, 0, fmt.Errorf("unsupported actor tree (audit/attachments excluded)")
					}
					names[name] = true
				}
				if len(names) != 2 {
					return nil, nil, 0, fmt.Errorf("missing signed actor material")
				}
				payload, e := projectGit(ctx, dir, nil, l.DecodedBytes, "show", commit+":event.json")
				if e != nil {
					return nil, nil, 0, e
				}
				encoded, e := projectGit(ctx, dir, nil, 256, "show", commit+":signature")
				if e != nil {
					return nil, nil, 0, e
				}
				sig, e := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
				if e != nil {
					return nil, nil, 0, e
				}
				event, id, e := verifyEvent(payload, sig)
				if e != nil {
					return nil, nil, 0, e
				}
				// Authority/CI/audit histories need contextual validators not supplied by
				// legacy staging. Refuse them explicitly rather than partially verifying.
				if !isIssueKind(event.Kind) && !isResourceKind(event.Kind) && !isProposalKind(event.Kind) && !strings.HasPrefix(event.Kind, "identity.") {
					return nil, nil, 0, fmt.Errorf("first public intake profile refuses native predecessor %s; preserve its complete chain and use trusted native migration/replication for this history", event.Kind)
				}
				if event.Actor != projectActorOwner(ref.SourceRef) {
					return nil, nil, 0, fmt.Errorf("native actor/ref mismatch")
				}
				parents, e := projectGit(ctx, dir, nil, 256, "show", "-s", "--format=%P", commit)
				if e != nil || strings.TrimSpace(string(parents)) != previous {
					return nil, nil, 0, fmt.Errorf("actor commit predecessor mismatch")
				}
				previous = commit
				if !seen[id] {
					if int64(len(events)+envelopes) >= l.Events || len(events)+envelopes >= c.MaxGraphRecords {
						return nil, nil, 0, fmt.Errorf("aggregate event count limit")
					}
					row := StoredEvent{ID: id, Commit: commit, Event: event, Payload: payload, Signature: sig, Attachments: map[string][]byte{}}
					events = append(events, row)
					byID[id] = row
					seen[id] = true
				}
			}
		case "proposal":
			id := "sha256:" + strings.TrimPrefix(ref.SourceRef, "refs/hn/proposals/")
			if _, observed, ok := parseAcceptedProposalRef(ref.SourceRef); ok {
				id = observed
			}
			if old, ok := code[id]; ok && old != ref.OID {
				return nil, nil, 0, fmt.Errorf("conflicting proposal refs")
			}
			code[id] = ref.OID
		case "code":
		default:
			return nil, nil, 0, fmt.Errorf("unsupported native ref kind %s", ref.Kind)
		}
	}
	if e = validateActorChains(events); e != nil {
		return nil, nil, 0, e
	}
	if e = validateExactEventReferenceClosure(events); e != nil {
		return nil, nil, 0, e
	}
	if _, e = validateIssueRelationships(events); e != nil {
		return nil, nil, 0, e
	}
	if _, e = validateResourceRelationships(events); e != nil {
		return nil, nil, 0, e
	}
	if _, e = ProjectIdentityContinuity(events); e != nil {
		return nil, nil, 0, e
	}
	if _, e = buildLineageIndex(events); e != nil {
		return nil, nil, 0, e
	}
	for id, head := range code {
		v, ok := byID[id]
		if !ok || !isProposalKind(v.Event.Kind) || v.Event.Head != head {
			return nil, nil, 0, fmt.Errorf("proposal ref lacks exact native binding")
		}
	}
	for _, row := range events {
		v := row.Event
		if isProposalKind(v.Kind) {
			if code[row.ID] != v.Head {
				return nil, nil, 0, fmt.Errorf("missing exact proposal code ref")
			}
			if _, e = projectGit(ctx, dir, nil, 256, "merge-base", "--is-ancestor", v.Base, v.Head); e != nil {
				return nil, nil, 0, fmt.Errorf("proposal ancestry invalid")
			}
			q.roots = append(q.roots, v.Base, v.Head)
			if v.Kind == "proposal.revise" && byID[v.Subject].Event.Actor != v.Actor {
				return nil, nil, 0, fmt.Errorf("proposal revision author mismatch")
			}
		}
	}
	ids, e := projectObjectIDs(ctx, dir, q.roots, ml)
	if e != nil {
		return nil, nil, 0, e
	}
	if !contributionSameRefs(ids, q.objectIDs) {
		return nil, nil, 0, fmt.Errorf("unselected/missing native object closure")
	}
	eventDepth, e := contributionEventDepth(ctx, events, l.Depth)
	if e != nil {
		return nil, nil, 0, e
	}
	depth = max(objectDepth, eventDepth)
	if e = ctx.Err(); e != nil {
		return nil, nil, 0, e
	}
	return q, events, depth, nil
}

func contributionObjectPreflight(ctx context.Context, raw []byte, l c.Limits) (int64, int64, error) {
	br := bufio.NewReader(bytes.NewReader(raw))
	magic, e := projectHeader(br)
	if e != nil || magic != projectGitSchema {
		return 0, 0, fmt.Errorf("unsupported native object encoding")
	}
	graph := map[string][]string{}
	var total int64
	for {
		if e = ctx.Err(); e != nil {
			return 0, 0, e
		}
		line, e := projectHeader(br)
		if e == io.EOF && line == "" {
			break
		}
		if e != nil {
			return 0, 0, e
		}
		f := strings.Fields(line)
		if len(f) != 3 || !migration.ValidOID(f[0]) {
			return 0, 0, fmt.Errorf("native object header")
		}
		n, e := strconv.ParseInt(f[2], 10, 64)
		if e != nil || n < 0 || n > l.DecodedBytes-total || int64(len(graph)) >= l.Objects {
			return 0, 0, fmt.Errorf("native predecode object budget")
		}
		if _, ok := graph[f[0]]; ok {
			return 0, 0, fmt.Errorf("duplicate native object")
		}
		total += n
		data := make([]byte, n)
		if _, e = io.ReadFull(br, data); e != nil {
			return 0, 0, e
		}
		sep, e := br.ReadByte()
		if e != nil || sep != '\n' {
			return 0, 0, fmt.Errorf("native object separator")
		}
		deps := []string{}
		switch f[1] {
		case "blob":
		case "commit":
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					break
				}
				if strings.HasPrefix(line, "parent ") || strings.HasPrefix(line, "tree ") {
					_, id, _ := strings.Cut(line, " ")
					if !migration.ValidOID(id) {
						return 0, 0, fmt.Errorf("invalid commit dependency")
					}
					deps = append(deps, id)
				}
			}
		case "tree":
			for len(data) > 0 {
				space := bytes.IndexByte(data, ' ')
				zero := bytes.IndexByte(data, 0)
				width := len(f[0]) / 2
				if space < 0 || zero <= space || zero+1+width > len(data) {
					return 0, 0, fmt.Errorf("invalid tree encoding")
				}
				name := string(data[space+1 : zero])
				if name == "audit.pack" {
					return 0, 0, fmt.Errorf("nested compressed audit material unsupported")
				}
				mode := string(data[:space])
				if mode != "160000" {
					deps = append(deps, fmt.Sprintf("%x", data[zero+1:zero+1+width]))
				}
				data = data[zero+1+width:]
			}
		default:
			return 0, 0, fmt.Errorf("unsupported native object type")
		}
		graph[f[0]] = deps
	}
	depth, e := contributionGraphDepth(ctx, graph, l.Depth)
	return int64(len(graph)), depth, e
}
func contributionGraphDepth(ctx context.Context, graph map[string][]string, limit int64) (int64, error) {
	depths := map[string]int64{}
	visiting := map[string]bool{}
	var walk func(string, int64) (int64, error)
	walk = func(id string, stack int64) (int64, error) {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		if stack > limit || visiting[id] {
			return 0, fmt.Errorf("dependency depth/cycle limit")
		}
		if d, ok := depths[id]; ok {
			return d, nil
		}
		deps, ok := graph[id]
		if !ok {
			return 0, fmt.Errorf("missing native dependency")
		}
		visiting[id] = true
		d := int64(1)
		for _, dep := range deps {
			child, e := walk(dep, stack+1)
			if e != nil {
				return 0, e
			}
			d = max(d, child+1)
		}
		delete(visiting, id)
		if d > limit {
			return 0, fmt.Errorf("dependency depth exceeded")
		}
		depths[id] = d
		return d, nil
	}
	depth := int64(0)
	for id := range graph {
		d, e := walk(id, 1)
		if e != nil {
			return 0, e
		}
		depth = max(depth, d)
	}
	return depth, nil
}
func contributionEventDepth(ctx context.Context, events []StoredEvent, limit int64) (int64, error) {
	graph := map[string][]string{}
	for _, s := range events {
		deps := replicationEventReferences(s.Event)
		if s.Event.Previous != "" {
			deps = append(deps, s.Event.Previous)
		}
		graph[s.ID] = sortedUniqueStrings(deps...)
	}
	return contributionGraphDepth(ctx, graph, limit)
}

func classifyContributionFacts(events []StoredEvent) (ContributionNativeSource, error) {
	source := ContributionNativeSource{facts: map[string]c.Fact{}}
	byID := map[string]StoredEvent{}
	for _, v := range events {
		byID[v.ID] = v
	}
	var classify func(string) (c.Fact, error)
	classify = func(id string) (c.Fact, error) {
		if f, ok := source.facts[id]; ok {
			return f, nil
		}
		v, ok := byID[id]
		if !ok {
			return c.Fact{}, c.ErrMissingDependency
		}
		e := v.Event
		content := c.Content{Root: id, Revision: id, Author: e.Actor, Parents: []string{}, References: []c.Reference{}}
		op := c.Operation("")
		add := func(kind c.ReferenceKind, id string) {
			if id != "" {
				for _, r := range content.References {
					if r.Kind == kind && r.ID == id {
						return
					}
				}
				content.References = append(content.References, c.Reference{Kind: kind, ID: id})
			}
		}
		switch e.Kind {
		case "issue.open":
			op = c.IssueCreate
		case "issue.comment":
			op = c.IssueReply
			add(c.RootReference, e.Subject)
		case "issue.revise":
			op = c.IssueEdit
			content.Root = e.Subject
			content.Parents = append([]string{}, e.Parents...)
		case "proposal.open":
			op = c.ProposalCreate
		case "proposal.revise":
			op = c.ProposalRevise
			parent, err := classify(e.Subject)
			if err != nil {
				return c.Fact{}, err
			}
			content.Root = parent.Content.Root
			content.Parents = []string{e.Subject}
		default:
			family := resourceFamily(e.Kind)
			opening := strings.HasSuffix(e.Kind, ".open")
			switch family {
			case ResourceProject:
				if opening {
					op = c.ProjectCreate
				} else {
					op = c.ProjectEdit
				}
			case ResourceProjectItem:
				if opening {
					op = c.ProjectItemCreate
				} else {
					op = c.ProjectItemEdit
				}
			case ResourceDiscussion:
				if opening {
					op = c.DiscussionCreate
				} else {
					op = c.DiscussionEdit
				}
			case ResourceDiscussionReply:
				if opening {
					op = c.DiscussionReply
				} else {
					op = c.DiscussionEdit
				}
			case ResourceWorkflow:
				if opening {
					op = c.WorkflowCreate
				} else {
					op = c.WorkflowEdit
				}
			default:
				return c.Fact{}, fmt.Errorf("native fact has no contribution content classification")
			}
			if !opening {
				content.Root = e.Subject
				content.Parents = append([]string{}, e.Parents...)
			}
			switch family {
			case ResourceProjectItem:
				var s ProjectItemState
				_ = json.Unmarshal([]byte(e.Body), &s)
				add(c.RootReference, s.ProjectID)
				add(c.RevisionReference, s.ProjectRevision)
				kind := c.RootReference
				if s.Target.Kind == "proposal" {
					kind = c.RevisionReference
				}
				add(kind, s.Target.ID)
			case ResourceDiscussion:
				var s DiscussionState
				_ = json.Unmarshal([]byte(e.Body), &s)
				add(c.RevisionReference, s.AcceptedReplyRevision)
			case ResourceDiscussionReply:
				var s DiscussionReplyState
				_ = json.Unmarshal([]byte(e.Body), &s)
				add(c.RootReference, s.DiscussionID)
				add(c.RootReference, s.ReplyTo)
			}
		}
		if e.Issue != nil {
			for _, r := range e.Issue.Relations {
				add(c.RootReference, r.Target)
			}
		}
		if content.Root != id {
			root, ok := byID[content.Root]
			if !ok {
				return c.Fact{}, c.ErrMissingDependency
			}
			content.Author = root.Event.Actor
		}
		if len(content.Parents) > c.MaxLinks || len(content.References) > c.MaxLinks {
			return c.Fact{}, fmt.Errorf("native dependency links exceed contribution profile")
		}
		c.SortIDs(content.Parents)
		c.SortReferences(content.References)
		fact := c.Fact{Actor: e.Actor, Operation: op, Content: content}
		source.facts[id] = fact
		return fact, nil
	}
	for _, v := range events {
		if strings.HasPrefix(v.Event.Kind, "identity.") {
			continue
		}
		if _, e := classify(v.ID); e != nil {
			return source, e
		}
	}
	return source, nil
}

// ContributionIntake is a local accounting receipt, never a portable project
// attestation. Reserved cost is conservative; measured fields describe work
// actually completed. Status reserved/uncertain does not imply promotion.
type ContributionIntake struct {
	Failure        string                      `json:"failure,omitempty"`
	Intent         string                      `json:"intent"`
	Artifact       string                      `json:"artifact"`
	Refs           []string                    `json:"refs"`
	Status         string                      `json:"status"`
	Reserved       c.Cost                      `json:"reserved"`
	WireBytes      int64                       `json:"wireBytes"`
	DecodedBytes   int64                       `json:"decodedBytes"`
	Objects        int64                       `json:"objects"`
	Events         int64                       `json:"events"`
	Depth          int64                       `json:"depth"`
	Milliseconds   int64                       `json:"milliseconds"`
	Accounting     string                      `json:"accounting"`
	Replayed       bool                        `json:"-"`
	ManifestWrites []ContributionManifestWrite `json:"manifestWrites,omitempty"`
}
type contributionJournal struct {
	Checksum string               `json:"checksum,omitempty"`
	GitDir   string               `json:"gitDir,omitempty"`
	Version  string               `json:"version"`
	Ledger   c.Ledger             `json:"ledger"`
	Attempts []ContributionIntake `json:"attempts"`
}

// ContributionExchange must be bound by the host to one receiver-local project
// directory regardless of keys, enable/disable state or moderation visibility.
// Policy/window are local trusted inputs, never read from an artifact. Init is
// exclusive; a missing/corrupt ledger in an existing directory is not a reset.
type ContributionExchange struct {
	root         string
	policy       c.Policy
	fault        func(string) error // deterministic crash seam, no global mutable hook
	readObserved func(int)          // instrumentation for bounded controlled-file tests
}

func OpenContributionExchange(root string, p c.Policy, window string) (*ContributionExchange, error) {
	if e := p.Validate(); e != nil {
		return nil, e
	}
	if _, e := contributionLimits(p.Limits); e != nil {
		return nil, e
	}
	abs, e := safepath.Canonical(root)
	if e != nil {
		return nil, e
	}
	if e = safepath.CheckDir(filepath.Dir(abs), false); e != nil {
		return nil, e
	}
	created := false
	if e = os.Mkdir(abs, 0700); e == nil {
		created = true
	} else if !errors.Is(e, os.ErrExist) {
		return nil, e
	}
	if e = safepath.CheckDir(abs, true); e != nil {
		return nil, e
	}
	x := &ContributionExchange{root: abs, policy: p}
	unlock, e := x.lock()
	if e != nil {
		return nil, e
	}
	defer unlock()
	if created {
		ledger := c.Ledger{Project: p.Project, Window: window}
		probe := c.Cost{WireBytes: 1, DecodedBytes: 1, Objects: 1, Events: 1, Depth: 1, Milliseconds: 1}
		if _, _, e = c.Reserve(ledger, p, "validate-initial", c.Digest([]byte("initial")), probe); e != nil {
			return nil, e
		}
		if e = x.save(contributionJournal{Version: contributionJournalVersion, Ledger: ledger, Attempts: []ContributionIntake{}}); e != nil {
			return nil, e
		}
	}
	j, e := x.load()
	if e != nil {
		return nil, e
	}
	if j.Ledger.Project != p.Project {
		return nil, fmt.Errorf("exchange project mismatch")
	}
	return x, nil
}
func (x *ContributionExchange) lock() (func(), error) {
	if e := safepath.CheckDir(x.root, true); e != nil {
		return nil, e
	}
	// A persistent lock inode plus OS advisory locking survives process death
	// without deleting another process's lock or requiring unsafe stale cleanup.
	fd, e := syscall.Open(filepath.Join(x.root, "lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "contribution lock")
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		f.Close()
		return nil, fmt.Errorf("unsafe contribution lock")
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("project intake busy: %w", e)
	}
	return func() { _ = syscall.Flock(fd, syscall.LOCK_UN); _ = f.Close() }, nil
}
func (x *ContributionExchange) save(j contributionJournal) error {
	j.Checksum = ""
	body, e := json.Marshal(j)
	if e != nil {
		return e
	}
	j.Checksum = c.Digest(body)
	raw, e := json.Marshal(j)
	if e != nil {
		return e
	}
	if len(raw) > contributionJournalMax || int64(len(raw)) > 4096+int64(len(j.Attempts))*c.LedgerEntryBytes {
		return fmt.Errorf("intake journal metadata budget")
	}
	return writePrivateFileAtomic(filepath.Join(x.root, "journal.json"), raw)
}
func (x *ContributionExchange) load() (contributionJournal, error) {
	var j contributionJournal
	raw, e := readPrivateFileBounded(filepath.Join(x.root, "journal.json"), contributionJournalMax)
	if e != nil {
		return j, e
	}
	if e = contributionStrict(raw, &j); e != nil {
		return j, e
	}
	checksum := j.Checksum
	j.Checksum = ""
	body, e := json.Marshal(j)
	if e != nil || checksum != c.Digest(body) {
		return j, fmt.Errorf("intake journal checksum mismatch")
	}
	j.Checksum = checksum
	if j.Version != contributionJournalVersion || j.Ledger.Project != x.policy.Project || len(j.Attempts) != len(j.Ledger.Reservations) || len(j.Attempts) > c.MaxGraphRecords {
		return j, fmt.Errorf("unsupported/corrupt intake journal")
	}
	if len(j.Attempts) > 0 {
		r := j.Ledger.Reservations[0]
		if _, replay, e := c.Reserve(j.Ledger, x.policy, r.Intent, r.Artifact, r.Cost); e != nil || !replay {
			return j, fmt.Errorf("corrupt intake ledger: %v", e)
		}
	} else if j.Ledger.WindowUsed != (c.Usage{}) || j.Ledger.RetainedUsed != (c.Usage{}) || len(j.Ledger.PriorWindows) != 0 {
		return j, fmt.Errorf("reset intake counters")
	}
	for i, a := range j.Attempts {
		r := j.Ledger.Reservations[i]
		if a.Intent != r.Intent || a.Artifact != r.Artifact || a.Reserved != r.Cost || !migration.ValidDigest(a.Accounting) || a.WireBytes < 0 || a.WireBytes > a.Reserved.WireBytes || a.DecodedBytes < 0 || a.DecodedBytes > a.Reserved.DecodedBytes || a.Objects < 0 || a.Objects > a.Reserved.Objects || a.Events < 0 || a.Events > a.Reserved.Events {
			return j, fmt.Errorf("corrupt intake receipt")
		}
		refs, e := contributionRefs(a.Refs)
		if e != nil || !contributionSameRefs(refs, a.Refs) {
			return j, fmt.Errorf("corrupt intake selection")
		}
		seenActors := map[string]bool{}
		var plannedObjects, plannedBytes int64
		for _, plan := range a.ManifestWrites {
			// Older durable plans were saved only after immutable objects.
			// Keep their receipts/accounting readable, but never spend their
			// unknown per-plan remainder on another immutable operation.
			legacy := plan.Material == "" && plan.Objects == 0 && plan.Bytes == 0 && !plan.ObjectsDurable
			if !validActorFingerprint(plan.Actor) || seenActors[plan.Actor] || !legacy && !migration.ValidDigest(plan.Material) || plan.Objects < 0 || plan.Bytes < 0 || plan.Objects > a.Reserved.Objects || plan.Bytes > a.Reserved.DecodedBytes || plan.Complete && !plan.ObjectsDurable && !legacy || !migration.ValidOID(plan.Head) || (plan.Expected != "absent" && !migration.ValidOID(plan.Expected)) || j.GitDir == "" || a.Status != "pending" && a.Status != "conflict" {
				return j, fmt.Errorf("corrupt raw retention intent")
			}
			seenActors[plan.Actor] = true
			plannedObjects += plan.Objects
			plannedBytes += plan.Bytes
		}
		if plannedObjects > a.Reserved.Objects || plannedBytes > a.Reserved.DecodedBytes {
			return j, fmt.Errorf("raw retention plans exceed intake reservation")
		}
		if int64(len(a.ManifestWrites))*3 > a.Reserved.Objects || len(a.Failure) > 256 {
			return j, fmt.Errorf("intake metadata exceeds reservation")
		}
		switch a.Status {
		case "reserved", "uncertain", "failed", "pending", "conflict":
		default:
			return j, fmt.Errorf("unsupported intake status")
		}
	}
	return j, nil
}
func (x *ContributionExchange) failpoint(step string) error {
	if x.fault != nil {
		return x.fault(step)
	}
	return nil
}
func contributionReservation(l c.Limits) c.Cost {
	return c.Cost{WireBytes: l.WireBytes, DecodedBytes: l.DecodedBytes, Objects: l.Objects, Events: l.Events, Depth: l.Depth, Milliseconds: l.Milliseconds}
}
func (x *ContributionExchange) artifactPath(digest string) string {
	return filepath.Join(x.root, digest[7:]+".hnc")
}

// Receive reserves all request maxima durably BEFORE opening the source. Exact
// operation/bytes/selection replay returns its durable local receipt without I/O.
// Changed source bytes cannot win by reusing an intent and declared digest.
func (x *ContributionExchange) Receive(ctx context.Context, path, digest, intent string, refs []string) (result ContributionIntake, err error) {
	if len(intent) == 0 || len(intent) > 128 || !utf8.ValidString(intent) || strings.IndexFunc(intent, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return result, fmt.Errorf("bounded operation intent required")
	}
	selected, e := contributionRefs(refs)
	if e != nil {
		return result, e
	}
	unlock, e := x.lock()
	if e != nil {
		return result, e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return result, e
	}
	for _, a := range j.Attempts {
		if a.Intent == intent {
			if a.Artifact != digest || !contributionSameRefs(a.Refs, selected) {
				return result, fmt.Errorf("conflicting operation intent")
			}
			a.Replayed = true
			if a.Status == "conflict" {
				return a, fmt.Errorf("conflicting signed actor/project intent")
			}
			return a, nil
		}
	}
	for _, a := range j.Attempts {
		if (a.Status == "pending" || a.Status == "conflict") && a.Artifact == digest && contributionSameRefs(a.Refs, selected) {
			a.Replayed = true
			if a.Status == "conflict" {
				return a, fmt.Errorf("conflicting signed actor/project intent")
			}
			return a, nil
		}
	}
	cost := contributionReservation(x.policy.Limits)
	next, _, e := c.Reserve(j.Ledger, x.policy, intent, digest, cost)
	if e != nil {
		return result, e
	}
	transition, _ := json.Marshal(next)
	result = ContributionIntake{Intent: intent, Artifact: digest, Refs: selected, Status: "reserved", Reserved: cost, Accounting: c.Digest(transition)}
	j.Ledger = next
	j.Attempts = append(j.Attempts, result)
	index := len(j.Attempts) - 1
	if e = x.save(j); e != nil {
		return result, e
	}
	if e = x.failpoint("reserved"); e != nil {
		return result, e
	}
	started := time.Now()
	ctx, cancel, e := contributionContext(ctx, x.policy.Limits)
	if e != nil {
		return result, e
	}
	defer cancel()
	// A failed attempt is retained and charged, including a caller already canceled
	// before source open. Persistence failure leaves the earlier reservation intact.
	defer func() {
		result.Milliseconds = time.Since(started).Milliseconds()
		if err != nil {
			if result.Status != "uncertain" && result.Status != "conflict" {
				result.Status = "failed"
			}
			result.Failure = strings.ToValidUTF8(err.Error(), "?")
			if len(result.Failure) > 256 {
				result.Failure = strings.ToValidUTF8(result.Failure[:256], "?")
			}
		}
		j.Attempts[index] = result
		if e := x.save(j); e != nil {
			err = errors.Join(err, e)
		}
	}()
	if e = ctx.Err(); e != nil {
		return result, e
	}
	f, e := contributionOpenRegular(path, false)
	if e != nil {
		return result, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return result, e
	}
	if info.Size() > cost.WireBytes {
		return result, fmt.Errorf("source exceeds reserved wire bytes")
	}
	reader := &contributionObservedReader{r: f, observe: x.readObserved}
	bounded := &contributionReader{ctx: ctx, r: reader, left: cost.WireBytes}
	raw, e := io.ReadAll(io.LimitReader(bounded, cost.WireBytes))
	result.WireBytes = bounded.n
	if result.WireBytes > cost.WireBytes {
		result.WireBytes = cost.WireBytes
		return result, fmt.Errorf("source wire budget exceeded")
	}
	if e != nil {
		return result, e
	}
	after, statErr := f.Stat()
	if statErr != nil || after.Size() != info.Size() || int64(len(raw)) != info.Size() || c.Digest(raw) != digest {
		return result, fmt.Errorf("source size/digest changed")
	}
	a, e := readContributionArtifact(ctx, bytes.NewReader(raw), x.policy.Project, selected, x.policy.Limits)
	if e != nil {
		return result, e
	}
	defer a.Close()
	result.DecodedBytes = a.DecodedBytes
	result.Objects = a.Objects
	result.Events = a.EventCount
	result.Depth = a.Depth
	// Conflicts are authenticated raw material. Retain both sides and mark
	// every receiver-known affected attempt, including interrupted durable input.
	conflicted, e := x.markConflicts(ctx, &j, index, a.Records)
	if e != nil {
		return result, e
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	stored := x.artifactPath(digest)
	if existing, e := contributionOpenRegular(stored, true); e == nil {
		prior, e := io.ReadAll(io.LimitReader(existing, cost.WireBytes+1))
		_ = existing.Close()
		if e != nil || !bytes.Equal(prior, raw) {
			return result, fmt.Errorf("immutable artifact conflict")
		}
	} else if errors.Is(e, os.ErrNotExist) {
		if e = contributionPublish(stored, raw); e != nil {
			return result, e
		}
	} else {
		return result, e
	}
	if e = x.failpoint("artifact-durable"); e != nil {
		result.Status = "uncertain"
		j.Attempts[index] = result
		_ = x.save(j)
		return result, e
	}
	result.Status = "pending"
	if conflicted {
		result.Status = "conflict"
		return result, fmt.Errorf("conflicting signed actor/project intent")
	}
	return result, nil
}

type contributionObservedReader struct {
	r       io.Reader
	observe func(int)
}

func (r *contributionObservedReader) Read(p []byte) (int, error) {
	n, e := r.r.Read(p)
	if r.observe != nil {
		r.observe(n)
	}
	return n, e
}
func contributionOpenRegular(path string, private bool) (*os.File, error) {
	abs, e := safepath.Canonical(path)
	if e != nil {
		return nil, e
	}
	if e = safepath.CheckDir(filepath.Dir(abs), false); e != nil {
		return nil, e
	}
	info, e := os.Lstat(abs)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || private && info.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("local regular file with safe mode required")
	}
	fd, e := syscall.Open(abs, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), abs)
	actual, e := f.Stat()
	if e != nil || !actual.Mode().IsRegular() || !os.SameFile(info, actual) {
		f.Close()
		return nil, fmt.Errorf("source changed while opening")
	}
	return f, nil
}
func contributionIntentConflict(groups ...[]c.Verified) bool {
	intents := map[string]string{}
	for _, group := range groups {
		for _, v := range group {
			r := v.Record()
			key := r.Actor + ":" + r.Intent
			if old, ok := intents[key]; ok && old != v.ID() {
				return true
			}
			intents[key] = v.ID()
		}
	}
	return false
}
func contributionCrossConflict(a, b []c.Verified) bool {
	intents := map[string]map[string]bool{}
	for _, v := range a {
		r := v.Record()
		key := r.Actor + ":" + r.Intent
		if intents[key] == nil {
			intents[key] = map[string]bool{}
		}
		intents[key][v.ID()] = true
	}
	for _, v := range b {
		r := v.Record()
		for id := range intents[r.Actor+":"+r.Intent] {
			if id != v.ID() {
				return true
			}
		}
	}
	return false
}

func (x *ContributionExchange) markConflicts(ctx context.Context, j *contributionJournal, index int, records []c.Verified) (bool, error) {
	conflicted := j.Attempts[index].Status == "conflict" || contributionIntentConflict(records)
	for i, old := range j.Attempts {
		if i == index {
			continue
		}
		if _, e := os.Lstat(x.artifactPath(old.Artifact)); errors.Is(e, os.ErrNotExist) {
			continue
		} else if e != nil {
			return false, e
		}
		prior, e := x.inspectAttempt(ctx, old)
		if e != nil {
			return false, e
		}
		conflict := contributionCrossConflict(prior.Records, records)
		_ = prior.Close()
		if conflict {
			conflicted = true
			j.Attempts[i].Status = "conflict"
			j.Attempts[i].Failure = "conflicting signed actor/project intent"
		}
	}
	return conflicted, nil
}

func (x *ContributionExchange) Pending() ([]ContributionIntake, error) {
	unlock, e := x.lock()
	if e != nil {
		return nil, e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return nil, e
	}
	out := []ContributionIntake{}
	for _, a := range j.Attempts {
		if a.Status == "pending" || a.Status == "conflict" {
			out = append(out, a)
		}
	}
	return out, nil
}

// Accounting exposes failed/uncertain reservations too; hiding or draining a
// projection cannot erase them. Returned data is detached from durable storage.
func (x *ContributionExchange) Accounting() (c.Ledger, []ContributionIntake, error) {
	unlock, e := x.lock()
	if e != nil {
		return c.Ledger{}, nil, e
	}
	defer unlock()
	j, e := x.load()
	return j.Ledger, j.Attempts, e
}
func (x *ContributionExchange) inspectAttempt(ctx context.Context, a ContributionIntake) (*ContributionArtifact, error) {
	f, e := contributionOpenRegular(x.artifactPath(a.Artifact), true)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	raw, e := io.ReadAll(&contributionReader{ctx: ctx, r: f, left: a.Reserved.WireBytes + 1})
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > a.Reserved.WireBytes || c.Digest(raw) != a.Artifact {
		return nil, fmt.Errorf("retained artifact corrupt")
	}
	l := x.policy.Limits
	l.WireBytes = a.Reserved.WireBytes
	l.DecodedBytes = a.Reserved.DecodedBytes
	l.Objects = a.Reserved.Objects
	l.Events = a.Reserved.Events
	l.Depth = a.Reserved.Depth
	l.Milliseconds = a.Reserved.Milliseconds
	verified, e := readContributionArtifact(ctx, bytes.NewReader(raw), x.policy.Project, a.Refs, l)
	if e != nil {
		return nil, e
	}
	verified.Intake = &a
	return verified, nil
}
func (x *ContributionExchange) Inspect(ctx context.Context, intent string) (*ContributionArtifact, error) {
	unlock, e := x.lock()
	if e != nil {
		return nil, e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return nil, e
	}
	for _, a := range j.Attempts {
		if a.Intent == intent {
			ctx, cancel := context.WithTimeout(ctx, time.Duration(a.Reserved.Milliseconds)*time.Millisecond)
			defer cancel()
			return x.inspectAttempt(ctx, a)
		}
	}
	return nil, fmt.Errorf("unknown intake intent")
}

// Recover reconciles only immutable, receiver-retained exact bytes. It never
// reopens a mutable foreign source or guesses that an absent artifact succeeded.
func (x *ContributionExchange) Recover(ctx context.Context, intent string) (ContributionIntake, error) {
	unlock, e := x.lock()
	if e != nil {
		return ContributionIntake{}, e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return ContributionIntake{}, e
	}
	for i, a := range j.Attempts {
		if a.Intent != intent {
			continue
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(a.Reserved.Milliseconds)*time.Millisecond)
		defer cancel()
		v, e := x.inspectAttempt(ctx, a)
		if e != nil {
			return a, fmt.Errorf("intake remains uncertain: %w", e)
		}
		conflicted, e := x.markConflicts(ctx, &j, i, v.Records)
		if e != nil {
			_ = v.Close()
			return a, e
		}
		a.Status = "pending"
		if conflicted {
			a.Status = "conflict"
			a.Failure = "conflicting signed actor/project intent"
		}
		a.WireBytes = v.WireBytes
		a.DecodedBytes = v.DecodedBytes
		a.Objects = v.Objects
		a.Events = v.EventCount
		a.Depth = v.Depth
		_ = v.Close()
		j.Attempts[i] = a
		if e = x.save(j); e != nil {
			return a, e
		}
		if conflicted {
			return a, fmt.Errorf("conflicting signed actor/project intent")
		}
		return a, nil
	}
	return ContributionIntake{}, fmt.Errorf("unknown intake intent")
}
func (x *ContributionExchange) Export(ctx context.Context, intent, path string) error {
	a, e := x.Inspect(ctx, intent)
	if e != nil {
		return e
	}
	_ = a.Close()
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return e
	}
	for _, a := range j.Attempts {
		if a.Intent != intent {
			continue
		}
		f, e := contributionOpenRegular(x.artifactPath(a.Artifact), true)
		if e != nil {
			return e
		}
		defer f.Close()
		raw, e := io.ReadAll(&contributionReader{ctx: ctx, r: f, left: a.Reserved.WireBytes + 1})
		if e != nil {
			return e
		}
		if c.Digest(raw) != a.Artifact {
			return fmt.Errorf("retained artifact changed")
		}
		return contributionPublish(path, raw)
	}
	return fmt.Errorf("unknown intake intent")
}

// Rollover requires a host-verified CURRENT maintainer decision and expected-old
// window. It never resets retained capacity or accepts a portable receipt as
// authorization. WP12 owns the selected-authority adapter that calls this seam.
func (x *ContributionExchange) Rollover(expected, next string, authorized bool) error {
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return e
	}
	ledger, e := c.Rollover(j.Ledger, x.policy, expected, next, authorized)
	if e != nil {
		return e
	}
	j.Ledger = ledger
	return x.save(j)
}

type contributionSnapshot struct {
	Snapshot migration.Snapshot
	data     []byte
}

func (p *contributionSnapshot) Close() error { return nil }
func (p *contributionSnapshot) Open(ctx context.Context, digest string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if digest != p.Snapshot.GitBundle {
		return nil, fmt.Errorf("undeclared native blob")
	}
	return io.NopCloser(bytes.NewReader(p.data)), nil
}

// This exporter avoids the legacy snapshot's non-context audit reads. Native
// code dependencies must be explicitly selected proposal refs; no ambient refs.
func exportContributionSnapshot(ctx context.Context, dir string, refs []string, l migration.Limits) (*contributionSnapshot, error) {
	s := migration.Snapshot{Schema: "hn.project-snapshot/1", Source: migration.Locator{Kind: "local"}, Records: []migration.Record{}, Blobs: []migration.Blob{}, NativeRefs: []migration.NativeRef{}, Findings: []migration.Finding{}}
	roots := []string{}
	for _, ref := range refs {
		raw, e := projectGit(ctx, dir, nil, 256, "show-ref", "--verify", "--hash", ref)
		if e != nil {
			return nil, e
		}
		id := strings.TrimSpace(string(raw))
		if !migration.ValidOID(id) {
			return nil, fmt.Errorf("invalid selected ref")
		}
		roots = append(roots, id)
		s.NativeRefs = append(s.NativeRefs, migration.NativeRef{SourceRef: ref, OID: id, Kind: migration.NativeKind(ref)})
	}
	ids, e := projectObjectIDs(ctx, dir, roots, l)
	if e != nil {
		return nil, e
	}
	var data bytes.Buffer
	data.WriteString(projectGitSchema)
	for _, id := range ids {
		meta, e := projectGit(ctx, dir, []byte(id+"\n"), 256, "cat-file", "--batch-check=%(objecttype) %(objectsize)")
		if e != nil {
			return nil, e
		}
		f := strings.Fields(string(meta))
		if len(f) != 2 {
			return nil, fmt.Errorf("missing selected object")
		}
		n, e := strconv.ParseInt(f[1], 10, 64)
		if e != nil || n < 0 || n > l.GitObjectBytes {
			return nil, fmt.Errorf("object size limit")
		}
		raw, e := projectGit(ctx, dir, nil, n, "cat-file", f[0], id)
		if e != nil || int64(len(raw)) != n {
			return nil, fmt.Errorf("object read failed")
		}
		if int64(data.Len())+n+256 > l.GitTotalBytes {
			return nil, fmt.Errorf("native framed bytes limit")
		}
		fmt.Fprintf(&data, "%s %s %d\n", id, f[0], n)
		data.Write(raw)
		data.WriteByte('\n')
	}
	digest := c.Digest(data.Bytes())
	s.GitBundle = digest
	s.Blobs = append(s.Blobs, migration.Blob{Digest: digest, Size: int64(data.Len()), MediaType: "application/vnd.hn.git-objects", Name: "selected Git object closure"})
	identity, _ := json.Marshal(s.NativeRefs)
	s.SourceID = c.Digest(identity)
	return &contributionSnapshot{Snapshot: s, data: data.Bytes()}, nil
}

const contributionRefSchema = "hn.contribution-manifest/1"

type contributionRefManifest struct {
	Schema  string   `json:"schema"`
	Project string   `json:"project"`
	Actor   string   `json:"actor"`
	Records []string `json:"records"`
}

func contributionRef(project, actor string) (string, error) {
	if !migration.ValidDigest(project) || !validActorFingerprint(actor) {
		return "", fmt.Errorf("invalid contribution namespace")
	}
	return "refs/hn/contributions/" + project[7:] + "/" + actor, nil
}

// ReadContributionManifest reads only an explicit canonical raw-retention ref.
// Its records are signature-authenticated data, not admitted project authority.
// Full commit/object history must fit the same finite bytes/count/depth bounds.
func ReadContributionManifest(ctx context.Context, gitDir, project, actor string, l c.Limits) (records []c.Verified, head string, err error) {
	ctx, cancel, e := contributionContext(ctx, l)
	if e != nil {
		return nil, "", e
	}
	defer cancel()
	if e = contributionSafeGitDir(gitDir); e != nil {
		return nil, "", e
	}
	ref, e := contributionRef(project, actor)
	if e != nil {
		return nil, "", e
	}
	raw, e := projectGit(ctx, gitDir, nil, 256, "show-ref", "--verify", "--hash", ref)
	if e != nil {
		return nil, "", e
	}
	head = strings.TrimSpace(string(raw))
	if !migration.ValidOID(head) {
		return nil, "", fmt.Errorf("invalid contribution commit ref")
	}
	if _, e = projectGit(ctx, gitDir, nil, 256, "rev-parse", "--verify", head+"^{commit}"); e != nil {
		return nil, "", e
	}
	ml, e := contributionLimits(l)
	if e != nil {
		return nil, "", e
	}
	ids, e := projectObjectIDs(ctx, gitDir, []string{head}, ml)
	if e != nil {
		return nil, "", e
	}
	// Inspect raw objects before any parser could expand nested material.
	var material bytes.Buffer
	material.WriteString(projectGitSchema)
	for _, id := range ids {
		meta, e := projectGit(ctx, gitDir, []byte(id+"\n"), 256, "cat-file", "--batch-check=%(objecttype) %(objectsize)")
		if e != nil {
			return nil, "", e
		}
		f := strings.Fields(string(meta))
		if len(f) != 2 {
			return nil, "", fmt.Errorf("missing contribution object")
		}
		n, e := strconv.ParseInt(f[1], 10, 64)
		if e != nil || n < 0 || n > l.DecodedBytes-int64(material.Len())-256 {
			return nil, "", fmt.Errorf("contribution ref byte limit")
		}
		data, e := projectGit(ctx, gitDir, nil, n, "cat-file", f[0], id)
		if e != nil {
			return nil, "", e
		}
		fmt.Fprintf(&material, "%s %s %d\n", id, f[0], n)
		material.Write(data)
		material.WriteByte('\n')
	}
	if _, _, e = contributionObjectPreflight(ctx, material.Bytes(), l); e != nil {
		return nil, "", e
	}
	raw, e = projectGit(ctx, gitDir, nil, min(l.DecodedBytes, int64(contributionManifestMax)), "show", head+":manifest.json")
	if e != nil {
		return nil, "", e
	}
	var m contributionRefManifest
	if e = contributionStrict(raw, &m); e != nil {
		return nil, "", e
	}
	if m.Schema != contributionRefSchema || m.Project != project || m.Actor != actor || len(m.Records) == 0 || int64(len(m.Records)) > l.Events {
		return nil, "", fmt.Errorf("invalid contribution ref manifest")
	}
	tree, e := projectGit(ctx, gitDir, nil, l.DecodedBytes, "ls-tree", "-z", head)
	if e != nil {
		return nil, "", e
	}
	names := map[string]bool{}
	for _, entry := range strings.Split(string(tree), "\x00") {
		if entry == "" {
			continue
		}
		meta, name, ok := strings.Cut(entry, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 || f[0] != "100644" || f[1] != "blob" || names[name] {
			return nil, "", fmt.Errorf("unsupported contribution manifest tree")
		}
		names[name] = true
	}
	if len(names) != 1+len(m.Records)*2 || !names["manifest.json"] {
		return nil, "", fmt.Errorf("unselected manifest material")
	}
	previous := ""
	for _, id := range m.Records {
		if !migration.ValidDigest(id) || id <= previous {
			return nil, "", fmt.Errorf("invalid sorted record selection")
		}
		previous = id
		payloadName := id[7:] + ".payload"
		signatureName := id[7:] + ".signature"
		if !names[payloadName] || !names[signatureName] {
			return nil, "", fmt.Errorf("missing selected contribution record")
		}
		payload, e := projectGit(ctx, gitDir, nil, c.MaxRecordBytes, "show", head+":"+payloadName)
		if e != nil {
			return nil, "", e
		}
		sig, e := projectGit(ctx, gitDir, nil, 64, "show", head+":"+signatureName)
		if e != nil {
			return nil, "", e
		}
		v, e := c.Verify(payload, sig)
		if e != nil {
			return nil, "", e
		}
		r := v.Record()
		if v.ID() != id || r.Actor != actor || r.Project != project {
			return nil, "", fmt.Errorf("contribution namespace/record mismatch")
		}
		records = append(records, v)
	}
	return records, head, nil
}

// contributionManifestObjects is prepared without writing Git objects. The
// complete deterministic result and conservative retention charge can therefore
// be journaled before any hash-object -w operation.
type contributionManifestObject struct {
	kind string
	raw  []byte
	oid  string
}
type contributionManifestObjects struct {
	head     string
	material string
	objects  []contributionManifestObject
	bytes    int64
}

func planContributionManifest(ctx context.Context, gitDir, project, actor, expected string, envelopes []c.Envelope, l c.Limits) (contributionManifestObjects, error) {
	var plan contributionManifestObjects
	if e := contributionSafeGitDir(gitDir); e != nil {
		return plan, e
	}
	if _, e := contributionRef(project, actor); e != nil {
		return plan, e
	}
	if expected != "absent" && !migration.ValidOID(expected) {
		return plan, fmt.Errorf("exact expected-old commit or absent required")
	}
	all := map[string]c.Verified{}
	priorCount := 0
	if expected != "absent" {
		prior, head, e := ReadContributionManifest(ctx, gitDir, project, actor, l)
		if e != nil {
			return plan, e
		}
		if head != expected {
			return plan, fmt.Errorf("contribution ref changed")
		}
		for _, v := range prior {
			all[v.ID()] = v
		}
		priorCount = len(all)
	}
	for _, env := range envelopes {
		v, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return plan, e
		}
		r := v.Record()
		if r.Project != project || r.Actor != actor {
			return plan, fmt.Errorf("contribution namespace mismatch")
		}
		all[v.ID()] = v
	}
	selected, e := json.Marshal(envelopes)
	if e != nil {
		return plan, e
	}
	plan.material = c.Digest(selected)
	if expected != "absent" && len(all) == priorCount {
		plan.head = expected
		return plan, nil
	}
	if len(all) == 0 || int64(len(all)) > l.Events || int64(len(all))*2+3 > l.Objects {
		return plan, fmt.Errorf("contribution manifest count limit")
	}
	ids := []string{}
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	manifest, _ := json.Marshal(contributionRefManifest{Schema: contributionRefSchema, Project: project, Actor: actor, Records: ids})
	// hash-object without -w is read-only and honors the destination's object
	// format. Build the canonical Git tree bytes in memory instead of mktree,
	// which would already persist an immutable tree before the journal save.
	add := func(kind string, raw []byte) (string, error) {
		out, e := projectGit(ctx, gitDir, raw, 256, "hash-object", "-t", kind, "--stdin")
		if e != nil {
			return "", e
		}
		oid := strings.TrimSpace(string(out))
		if !migration.ValidOID(oid) {
			return "", fmt.Errorf("invalid planned Git object")
		}
		plan.objects = append(plan.objects, contributionManifestObject{kind, raw, oid})
		// Include Git's header and a conservative zlib expansion allowance.
		// This bounds loose-object file bytes even for incompressible material.
		plan.bytes += int64(len(raw) + len(raw)/1000 + 256)
		return oid, nil
	}
	entries := map[string]string{}
	if entries["manifest.json"], e = add("blob", manifest); e != nil {
		return plan, e
	}
	for _, id := range ids {
		v := all[id]
		if entries[id[7:]+".payload"], e = add("blob", v.Payload()); e != nil {
			return plan, e
		}
		if entries[id[7:]+".signature"], e = add("blob", v.Signature()); e != nil {
			return plan, e
		}
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var tree bytes.Buffer
	for _, name := range names {
		oid, e := hex.DecodeString(entries[name])
		if e != nil {
			return plan, e
		}
		tree.WriteString("100644 " + name + "\x00")
		tree.Write(oid)
	}
	treeID, e := add("tree", tree.Bytes())
	if e != nil {
		return plan, e
	}
	commit := "tree " + treeID + "\n"
	if expected != "absent" {
		commit += "parent " + expected + "\n"
	}
	commit += "author Hubnot contribution retention <retention@hn.invalid> 0 +0000\ncommitter Hubnot contribution retention <retention@hn.invalid> 0 +0000\n\nRaw signed contribution retention\n"
	plan.head, e = add("commit", []byte(commit))
	if e != nil {
		return plan, e
	}
	if plan.bytes > l.DecodedBytes {
		return plan, fmt.Errorf("contribution manifest bytes limit")
	}
	return plan, nil
}

func writeContributionManifest(ctx context.Context, gitDir string, plan contributionManifestObjects, l c.Limits) error {
	for _, object := range plan.objects {
		oid, e := projectGit(ctx, gitDir, object.raw, 256, "hash-object", "-w", "-t", object.kind, "--stdin")
		if e != nil {
			return e
		}
		if strings.TrimSpace(string(oid)) != object.oid {
			return fmt.Errorf("stored Git object differs from durable plan")
		}
	}
	ml, _ := contributionLimits(l)
	if _, e := projectObjectIDs(ctx, gitDir, []string{plan.head}, ml); e != nil {
		return e
	}
	history, e := projectGit(ctx, gitDir, nil, l.Depth*65+1, "rev-list", plan.head)
	if e != nil || int64(len(strings.Fields(string(history))))+1 > l.Depth {
		return fmt.Errorf("contribution ref depth limit")
	}
	return nil
}

// prepareContributionManifest is the immutable-only primitive. RetainActor
// journals the complete plan before calling writeContributionManifest directly.
func prepareContributionManifest(ctx context.Context, gitDir, project, actor, expected string, envelopes []c.Envelope, l c.Limits) (string, error) {
	ctx, cancel, e := contributionContext(ctx, l)
	if e != nil {
		return "", e
	}
	defer cancel()
	plan, e := planContributionManifest(ctx, gitDir, project, actor, expected, envelopes, l)
	if e != nil {
		return "", e
	}
	if e = writeContributionManifest(ctx, gitDir, plan, l); e != nil {
		return "", e
	}
	return plan.head, nil
}

// ExportContributionRefs explicitly packages selected contribution manifests and
// their selected native closure. General hn sync/migrate do not transport these
// new envelope refs. This artifact excludes private ledger/keys/local authority.
func ExportContributionRefs(ctx context.Context, gitDir, project string, nativeRefs, actors []string, path string, l c.Limits) (string, error) {
	ctx, cancel, e := contributionContext(ctx, l)
	if e != nil {
		return "", e
	}
	defer cancel()
	if len(actors) == 0 || int64(len(actors)) > l.Objects {
		return "", fmt.Errorf("explicit contribution actor selection required")
	}
	seen := map[string]bool{}
	envs := []c.Envelope{}
	for _, actor := range actors {
		if seen[actor] {
			return "", fmt.Errorf("duplicate contribution actor")
		}
		seen[actor] = true
		records, _, e := ReadContributionManifest(ctx, gitDir, project, actor, l)
		if e != nil {
			return "", e
		}
		for _, v := range records {
			if int64(len(envs)) >= l.Events {
				return "", fmt.Errorf("aggregate envelope limit")
			}
			envs = append(envs, c.Envelope{Payload: v.Payload(), Signature: v.Signature()})
		}
	}
	return ExportContributionArtifact(ctx, gitDir, project, nativeRefs, envs, path, l)
}

// ContributionManifestWrite is a durable expected-old Git intent. An uncertain
// write is reconciled by exact planned head, never by guessing remote absence.
type ContributionManifestWrite struct {
	Actor          string `json:"actor"`
	Expected       string `json:"expected"`
	Head           string `json:"head"`
	Material       string `json:"material,omitempty"`
	Objects        int64  `json:"objects,omitempty"`
	Bytes          int64  `json:"bytes,omitempty"`
	ObjectsDurable bool   `json:"objectsDurable,omitempty"`
	Complete       bool   `json:"complete"`
}

// RetainActor explicitly installs authenticated raw envelope blobs from a
// completed raw local intake, including inspectable signed conflicts. It is
// separate from Receive and grants no admission or eligible receipt on conflicts.
// It reserves no free storage: native/envelope/manifest maxima were charged by
// the original attempt. Each attempt may write each actor at most once. The
// destination Git dir is bound once for the receiver-local project ledger.
func (x *ContributionExchange) RetainActor(ctx context.Context, gitDir, intent, actor, expected string) (string, error) {
	unlock, e := x.lock()
	if e != nil {
		return "", e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return "", e
	}
	dir, e := safepath.Canonical(gitDir)
	if e != nil {
		return "", e
	}
	if e = contributionSafeGitDir(dir); e != nil {
		return "", e
	}
	if j.GitDir != "" && j.GitDir != dir {
		return "", fmt.Errorf("receiver project already bound to another Git directory")
	}
	ref, e := contributionRef(x.policy.Project, actor)
	if e != nil {
		return "", e
	}
	if expected != "absent" && !migration.ValidOID(expected) {
		return "", fmt.Errorf("expected-old contribution head required")
	}
	for index, a := range j.Attempts {
		if a.Intent != intent {
			continue
		}
		if a.Status != "pending" && a.Status != "conflict" {
			return "", fmt.Errorf("completed bounded local intake required")
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(a.Reserved.Milliseconds)*time.Millisecond)
		defer cancel()
		finish := func(plan ContributionManifestWrite) (string, error) {
			if plan.Expected != expected {
				return "", fmt.Errorf("conflicting raw retention intent")
			}
			if plan.Complete {
				return plan.Head, nil
			}
			current, e := projectGit(ctx, dir, nil, 256, "show-ref", "--verify", "--hash", ref)
			actual := strings.TrimSpace(string(current))
			if e != nil {
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				actual = "absent"
			}
			if actual != plan.Head {
				if actual != plan.Expected {
					return "", fmt.Errorf("raw retention remains uncertain: contribution ref changed")
				}
				old := plan.Expected
				if old == "absent" {
					old = strings.Repeat("0", len(plan.Head))
				}
				if _, e = projectGit(ctx, dir, nil, 256, "-c", "core.hooksPath="+os.DevNull, "update-ref", ref, plan.Head, old); e != nil {
					return "", e
				}
			}
			if e = x.failpoint("manifest-ref-updated"); e != nil {
				return "", e
			}
			for i := range a.ManifestWrites {
				if a.ManifestWrites[i].Actor == actor {
					a.ManifestWrites[i].Complete = true
				}
			}
			j.Attempts[index] = a
			return plan.Head, x.save(j)
		}
		var existing *ContributionManifestWrite
		for i := range a.ManifestWrites {
			plan := &a.ManifestWrites[i]
			if plan.Actor == actor {
				if plan.Expected != expected {
					return "", fmt.Errorf("conflicting raw retention intent")
				}
				if plan.ObjectsDurable || plan.Material == "" {
					return finish(*plan)
				}
				existing = plan
				break
			}
		}
		verified, e := x.inspectAttempt(ctx, a)
		if e != nil {
			return "", e
		}
		defer verified.Close()
		envelopes := []c.Envelope{}
		for _, v := range verified.Records {
			if v.Record().Actor == actor {
				envelopes = append(envelopes, c.Envelope{Payload: v.Payload(), Signature: v.Signature()})
			}
		}
		if len(envelopes) == 0 {
			return "", fmt.Errorf("actor not selected in retained artifact")
		}
		current, e := projectGit(ctx, dir, nil, 256, "show-ref", "--verify", "--hash", ref)
		actual := strings.TrimSpace(string(current))
		if e != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			actual = "absent"
		}
		if actual != expected {
			return "", fmt.Errorf("contribution ref changed")
		}
		l := x.policy.Limits
		l.WireBytes = a.Reserved.WireBytes
		l.DecodedBytes = a.Reserved.DecodedBytes
		l.Objects = a.Reserved.Objects
		l.Events = a.Reserved.Events
		l.Depth = a.Reserved.Depth
		l.Milliseconds = a.Reserved.Milliseconds
		objects, e := planContributionManifest(ctx, dir, x.policy.Project, actor, expected, envelopes, l)
		if e != nil {
			return "", e
		}
		plan := ContributionManifestWrite{Actor: actor, Expected: expected, Head: objects.head, Material: objects.material, Objects: int64(len(objects.objects)), Bytes: objects.bytes}
		if existing != nil {
			if *existing != plan {
				return "", fmt.Errorf("conflicting raw retention material or charge")
			}
		} else {
			// The intake's measured totals already include an allowance for
			// every selected actor. Replace those allowances with the complete
			// plans, retaining the native/artifact baseline and all failed plans.
			actors := map[string]bool{}
			for _, record := range verified.Records {
				actors[record.Record().Actor] = true
			}
			usedObjects := verified.Objects - int64(len(verified.Records))*2 - int64(len(actors))*3
			usedBytes := verified.DecodedBytes - int64(len(actors))*4096 - int64(len(verified.Records))*512
			for _, prior := range a.ManifestWrites {
				if prior.Material == "" {
					return "", fmt.Errorf("legacy raw retention intent permits exact replay only")
				}
				usedObjects += prior.Objects
				usedBytes += prior.Bytes
			}
			if usedObjects+plan.Objects > a.Reserved.Objects || usedBytes+plan.Bytes > a.Reserved.DecodedBytes {
				return "", fmt.Errorf("raw retention exceeds durable intake reservation")
			}
			a.ManifestWrites = append(a.ManifestWrites, plan)
			j.GitDir = dir
			j.Attempts[index] = a
			if e = x.save(j); e != nil {
				return "", e
			}
			if e = x.failpoint("manifest-plan-durable"); e != nil {
				return "", e
			}
		}
		if e = writeContributionManifest(ctx, dir, objects, l); e != nil {
			return "", e
		}
		if e = x.failpoint("manifest-objects-durable"); e != nil {
			return "", e
		}
		for i := range a.ManifestWrites {
			if a.ManifestWrites[i].Actor == actor {
				a.ManifestWrites[i].ObjectsDurable = true
			}
		}
		plan.ObjectsDurable = true
		j.Attempts[index] = a
		if e = x.save(j); e != nil {
			return "", e
		}
		return finish(plan)
	}
	return "", fmt.Errorf("unknown completed intake intent")
}
