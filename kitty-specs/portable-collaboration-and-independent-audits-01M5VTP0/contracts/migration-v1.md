# Migration contract v1

Canonical package: `hubnot/internal/migration`. This is an internal interface,
not a runtime plugin ABI. Existing `internal/forge` transfers remain compatible.

## Cross-package values

```go
type Locator struct { Kind, Endpoint, Organization, Project, Repository string }
type Key struct { Provider, Instance, ProjectID, Kind, ID string }
type Link struct { Relation string; Target Key; Revision string }
type Blob struct { Digest string; Size int64; MediaType, Name string }
type Record struct {
    Key Key
    Selector, Revision, SourceURL, Author, CreatedAt, UpdatedAt, Schema string
    Content json.RawMessage
    Links []Link
    Blobs []string
}
type NativeRef struct { SourceRef, OID, Kind string }
type Snapshot struct {
    Schema string // hn.project-snapshot/1
    Source Locator
    SourceID string
    Records []Record
    Blobs []Blob
    NativeRefs []NativeRef
    GitBundle string // content digest, not filesystem path
    Findings []Finding
}
type Limits struct {
    Records, Blobs, Refs, GitObjects int
    BlobBytes, ManifestBytes, AttachmentBytes, GitObjectBytes, GitTotalBytes int64
}
type Capability struct {
    Kind string
    Read, Preserve, Create, Update bool
    Limits []string
    Reason string
}
type Finding struct { ID, Status, Path, Message string }
type Decision struct { FindingID, Action string; Mapping json.RawMessage }
type Intent struct {
    ID, Action, Kind string
    Sources []Key
    Payload json.RawMessage
    ExpectedID, ExpectedRevision string
    DependsOn []string
}
type Plan struct {
    Schema string // hn.migration-plan/1
    SnapshotDigest string
    Destination Locator
    Limits Limits
    Decisions []Decision
    Intents []Intent
    Findings []Finding
}
type BlobSource interface { Open(context.Context, string) (io.ReadCloser, error) }
type Source interface {
    Capabilities(context.Context) ([]Capability, error)
    Snapshot(context.Context, Selection, Limits) (Snapshot, BlobSource, error)
}
type Destination interface {
    Capabilities(context.Context) ([]Capability, error)
    Plan(context.Context, Snapshot, Selection, Limits) ([]Intent, []Finding, error)
    Apply(context.Context, Intent) (Receipt, error)
    Reconcile(context.Context, Intent) (Reconciliation, error)
}
```

`Selection` contains validated locator/scope/mappings. `Receipt` contains observed
immutable identity/revision and intended payload digest. `Reconciliation` must
distinguish exact positive match, ambiguous/conflicting observation and no proof.
The engine owns progress and interprets reconciliation; adapters do not silently
retry creates or loop over multiple mutations inside one intended write.

JSON names are explicitly versioned, not inferred from provider DTOs. Validate
closed record schemas for issues/comments, discussions/replies, planning data,
workflow source, change requests/reviews/inline comments and attachments. A
preserved-source record can reference raw original bytes as a blob without
asserting native equivalence. Preserve numeric precision and unambiguous key
encoding; opaque IDs are never executable paths or API requests.

Findings use mapped, preserved, requires_mapping, unsupported,
missing_dependency or identity_conflict. `preserved` requires retrieved data;
an external URL is not an attachment blob. Empty results and unavailable coverage
are distinct. Specific non-equivalent/lossy findings need explicit decisions.

## Archive and native admission

`hn.project-archive/1` contains a canonical manifest and digest-addressed blobs,
including declared selected Git material. Generate member paths from digests;
source filenames are metadata. Reject duplicate/unknown members, traversal,
absolute names, links/special files, bad Unicode, mismatched sizes/digests and
expanded-budget exhaustion before accepted visibility.

Only explicitly selected refs and required dependency closure are exported.
Never capture filesystem `.git`, private identity stores, ambient credentials,
transfer journals or unselected working files. Exact selected committed history
is not implicitly secret-scrubbed. Historical governance objects are retained for
verification; active governance adoption requires an explicit plan decision.

Stage native Git data in an isolated bare quarantine. Validate signatures, actor
chains, trees and all resource/issue/proposal/audit closure before expected-old
promotion into designated remote-observation refs. Never create local signing
identities or graft imported actor histories into signing-owned streams. Worktree
or policy conflicts are explicit; no automatic reset/checkout/force update.

Foreign records and blobs remain locally inspectable under `noforge`; imported
reviews/users/CI/workflows grant no native authority or execution permission.

## Planning and recovery

Plans bind canonical source digest, destination, expected baselines, decisions
and dependency-ordered intents; contain no ambient secrets; and cannot silently
refresh during resume. Intent payloads are closed adapter inputs, not arbitrary
HTTP requests or shell commands. Planning cannot mutate native refs or providers.

Private locked progress binds plan digest and actor where native signing applies.
Persist prepared → in-flight → confirmed state around the one side effect.
After an uncertain create, exact positive evidence confirms it; absence,
inaccessibility or ambiguity requires manual recovery. A negative lookup is not
permission to repeat. Local ref lost-ack recovery observes actual refs. There is
no global atomic transaction spanning Git, files and provider APIs.

## Nonvacuous provider conformance

GitHub/GitLab/Forgejo/Gitea must implement issue+comment read/write. Azure must
implement explicitly mapped work-item+comment read/write. Bitbucket Cloud must
implement code/PR-conversation snapshots/workflow source and advertise removed
issues unavailable, without Jira substitution. Preserve applicable available
PR/MR/planning/discussion/workflow content and report destination-native creation
separately. All-unsupported registrations cannot satisfy provider coverage.

Test immutable identity versus selectors, full bounded pagination, private/deleted
objects, endpoint/version differences, cross-origin credential refusal, stale
destinations, lost responses and resumed progress. Forgejo/Gitea retain separate
profiles. Azure accepts supplied bearer/PAT and explicit Services/Server versions;
do not add deprecated OAuth registration. Native core never branches on provider
names. Existing `nogithub` remains GitHub-specific; `noforge` excludes all clients.
