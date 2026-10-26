# Portable project snapshot foundation

The provider-neutral `internal/migration` package supplies a v1 portable
observation and archive library. The root `ExportProjectSnapshot` and
`StageProjectSnapshot` bridge uses the existing native signature, actor-chain,
issue/resource, proposal, identity-continuity and memory validation boundaries.
These are foundation APIs; guided migration, saved plans, destination writes,
ref promotion and resume are separate capabilities implemented by the migration
engine. An archive round trip alone does not complete a project migration.

## Snapshot and source records

`Snapshot.Schema` is `hn.project-snapshot/1`. `Source` is an explicit `Locator`
(kind, endpoint, organization, project, repository); `SourceID` records immutable
source identity. Each `Record` has a typed `Key` (provider, normalized instance,
immutable project ID, kind, immutable object ID), separate mutable selector and
observed revision, source URL, source author/times, content schema, explicit links
and attachment digests. `Key.String()` uses JSON encoding of all five components,
so delimiters inside opaque IDs cannot collide. IDs and filenames never become
filesystem paths, commands or provider requests. `NormalizeInstance` canonicalizes
provider roots; credential-bearing URLs, query strings and fragments are refused.

All JSON property names are explicit snake_case. The issue/work_item content is
exactly `schema`, `title`, `body`, `state`, `type`, `labels`, `assignees`, with
schema `hn.migration.issue/1`. Comment content is exactly `schema`, `body`, with
schema `hn.migration.comment/1`. Every field is required, including empty arrays
and an empty type where applicable. Source author/time attribution belongs to
`Record`, never a fabricated native signer. A comment has exactly one `issue` or
`work_item` parent link to an included record of that kind; thread/reply links
remain separate. Links retain exact target keys and optional observed revisions.
The schemas reject unknown, missing, duplicate, null and case-aliased fields,
invalid UTF-8, unpaired UTF-16 surrogates, oversized text and duplicate label or
assignee values. Titles are at most 1,024 bytes, bodies 65,536 bytes, state/type
256 bytes, and label/assignee collections 256 entries of 1,024 bytes each.
Larger source text must be retained as a source blob with an explicit finding,
not truncated into these typed records.

Other retrieved source records use the closed `PreservedContent` schema:
`{"schema":"hn.migration.preserved/1","summary":"...","source_blob":"sha256:..."}`.
The summary is at most 65,536 bytes. Supported kinds are `discussion`, `reply`,
`project`, `project_item`, `workflow`, `change_request`, `review`, `inline_comment`
and `attachment`. `source_blob` must be a declared, record-referenced blob of
actual retrieved bytes. Original JSON numbers, source-specific fields, inline
positions, PR/MR review states, workflow source and planning detail survive in
those bytes without floating-point conversion or invented native equivalence.
Relationships remain explicit links. A URL alone does not preserve an attachment.
A source issue exceeding the common schema can be represented as an `attachment`
source record with a finding and retrieved original bytes; it is not eligible for
typed issue creation until an explicit mapping resolves the limitation.

Foreign users, reviews, permissions, CI observations and workflow source are
inert attributed evidence. Reading or importing them grants no native identity,
approval, policy, maintainer or runner authority and executes nothing. Findings
are `mapped`, `preserved`, `requires_mapping`, `unsupported`,
`missing_dependency` or `identity_conflict`; `preserved` means bytes were retrieved.
Missing linked objects fail manifest validation, so unavailable topology must be
reported as a finding rather than silently claiming that absent data was retained.

## Deterministic uncompressed archive v1

`WriteArchive(ctx, writer, snapshot, BlobSource, limits)` first validates the
manifest and spools/verifies every declared blob privately, then writes a tar.
The caller should publish its output file with an atomic rename only after
success, because an I/O failure can leave a partial writer output.
`ReadArchive` returns an `Archive` with `Snapshot`, digest-restricted `Open`, and
`Close` to remove its private spool. Reading an archive does not admit native
facts: the caller must pass the snapshot and blob source to
`StageProjectSnapshot` before applying any native destination change.

The first member is `manifest.json`, a canonical compact JSON object containing
`schema: hn.project-archive/1` and `snapshot`. Remaining members are regular files
named `blobs/sha256/<64 lowercase hex digits>`, sorted by digest. Source filenames
are metadata only. Manifest records, refs, links, blob references and findings
are sorted deterministically. Tar ownership, timestamps and modes are fixed;
all members use USTAR regular-file headers with mode 0600. No filesystem paths
from the source are extracted. Duplicate, missing, unknown, absolute/traversal,
symlink, hardlink, special, extended-header, size-mismatched and digest-mismatched
members refuse the entire private spool. Trailing nonzero data, concatenated
archives and excessive trailing zero padding are refused. Compression is not
part of archive v1: gzip/zip and compressed Git packs are unsupported.

Defaults are explicit positive budgets: 10,000 records, 1,024 attachment blobs,
1,024 refs, 16 MiB per attachment and manifest, 256 MiB total attachment data,
100,000 Git objects, 16 MiB per Git object and 512 MiB aggregate raw Git data.
Git framing has a separately bounded allowance of 256 bytes per object plus 128
bytes. An optional dedicated native material blob is additional to the attachment
count and uses that Git budget; it cannot be referenced as an ordinary attachment
to bypass attachment limits. Invalid or exceeded budgets refuse incomplete
success. Cancellation propagates to bounded Git subprocesses and streaming I/O.

## Exact selected native history

`ExportProjectSnapshot(ctx, gitDir, refs, limits)` requires a nonempty explicit
ref list. Supported sources are `refs/heads/*`, `refs/tags/*`,
`refs/hn/actors/*`, `refs/hn/proposals/*`, and `refs/hn/memory/*`. It exports their
full reachable object histories plus code dependencies named by selected signed
facts and memory anchors. Related signed facts must come from selected actor or
memory streams; missing suppliers refuse instead of discovering unrelated refs.
Unmerged proposal heads require their exact selected proposal ref and actor fact.
Shallow or corrupt graphs cannot become a complete portable snapshot.

`Snapshot.GitBundle` is the digest of declared native material, **not** a path or
a claim of interoperability with `git bundle`. The self-identifying format starts
with `hn.git-objects/1\n`, followed by objects sorted by full Git OID. Each object
has an ASCII `<oid> <commit|tree|blob|tag> <decimal size>\n` header, exactly that
many uncompressed raw object bytes, and a newline separator. Both SHA-1 and
SHA-256 repositories retain exact object IDs; mixed object formats refuse.

`StageProjectSnapshot(ctx, snapshot, BlobSource, limits)` checks all blob bytes
and reconstructs objects with `hash-object` in an isolated temporary bare
repository. It bounds headers, object counts, per-object/aggregate expansion and
Git subprocess output before further traversal. It recomputes every Git OID,
checks exact selected/dependency closure, runs strict Git structural checks and
existing native signature/tree/relationship validation. Extra unrelated objects
and incomplete histories refuse. Successful `ProjectQuarantine` exposes its
`GitDir`, copied `Roots()` and `ObjectIDs()` for the apply engine; `Close` removes
it. There are no quarantine refs. Later admission must use expected-old guards
and designated remote-observation refs, never local signing-owned streams.
Audit-specific target/rule dependencies are integrated with the audit foundation
by the dependent engine work; opaque audit attachments do not prove full ancestry.

No source or destination checkout, accepted ref, local identity, audit authority
anchor, active policy, transfer journal, Git configuration or worktree is adopted
by these APIs. Historical policy/workflow/code objects stay available as Git
objects for verification. Active governance adoption requires an explicit later
plan decision. Managed private stores, ambient tokens, journals, unselected refs
and working files are never walked. Ambient Git object/config overrides are
refused. Deliberately selected committed history remains byte-exact, including
its historical content: export does not promise secret scrubbing.

This foundation imports no provider client, service, new Go module or provider
CLI; foreign content inspection and local archive validation need only Go and Git.

Explicitly selected previously accepted `refs/hn/remotes/<remote>/actors/*`,
`proposals/*` and `memory/*/*` observation refs are portable as well. This permits
keyless imported-history repositories to archive native observations without a
signing identity or provider client. Their original source refs remain in the
manifest. Identical facts reached through multiple selected observations are
validated once; conflicting observations still refuse. Source quarantine,
pending, private and unrelated refs remain outside the selection allowlist.
