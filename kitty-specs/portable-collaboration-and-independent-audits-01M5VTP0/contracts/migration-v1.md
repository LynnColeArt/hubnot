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
type ReceiptRef struct { IntentID, Field string }
type RemoteRef struct { ID string; Receipt *ReceiptRef }
type BoundWrite struct {
    PlanDigest, IntentID, PayloadDigest string
    Payload json.RawMessage
}
type Receipt struct {
    PlanDigest, IntentID, Kind, Action string
    Destination Locator
    RemoteID, RemoteKey, Revision, PayloadDigest string
}
type Reconciliation struct {
    Status string // exact, ambiguous, no_proof
    Receipt *Receipt
    Reason string
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
type ApplyContext interface {
    OpenBlob(context.Context, string) (io.ReadCloser, Blob, error)
    Dependency(string) (Receipt, bool)
    BoundWrite() BoundWrite
}
type Source interface {
    Capabilities(context.Context) ([]Capability, error)
    Snapshot(context.Context, Selection, Limits) (Snapshot, BlobSource, error)
}
type Destination interface {
    Capabilities(context.Context) ([]Capability, error)
    Plan(context.Context, Snapshot, Selection, Limits) ([]Intent, []Finding, error)
    Apply(context.Context, Intent, ApplyContext) (Receipt, error)
    Reconcile(context.Context, Intent, ApplyContext) (Reconciliation, error)
}
```

`Selection` contains validated locator/scope/mappings. A receipt binds the plan,
intent, destination, actual immutable remote identity, mutable lookup selector,
observed revision and bound intended-payload digest. Exact reconciliation must
return a validated matching receipt; ambiguous/no-proof outcomes cannot confirm
or authorize retry. The engine owns progress; adapters do not silently retry
creates or loop over multiple mutations inside one intended write.

`ApplyContext` is engine-owned read-only input. `OpenBlob` exposes only blobs in
the snapshot that the engine has already verified and spooled within the plan's
per-blob/aggregate budgets; it cannot open arbitrary paths/URLs. `Dependency`
returns immutable value copies of confirmed predecessor receipts only. The
context supplies fresh copies of canonical bound payload bytes. Both apply and
reconcile receive the same persisted bound write and digest, not adapter-local
hidden state. Missing/unconfirmed dependencies fail before an effect.

Plan intents remain immutable templates. `RemoteRef` is a closed exclusive union
of a literal known remote ID or a typed receipt reference. Allowed receipt fields
are `remote_id`, `remote_key`, `revision`; dependencies must be declared and
acyclic. The engine resolves these from confirmed receipts, validates the closed
actual payload and persists its canonical bytes/digest before in-flight state
and the single mutation. Resume restores those same bytes rather than recomputing
from refreshed sources or silently rewriting the saved plan. No arbitrary string
interpolation, adapter-managed parent maps or fabricated zero parent IDs.

## Closed issue/comment schemas frozen before adapter parallelism

WP04 supplies strict snapshot record schemas and WP05 supplies strict intent,
binding, receipt and reconciliation validation. Freeze their exported types and
JSON contracts before WP06/07–10 adapters; no provider may invent its own shape.

```go
type IssueContent struct {
    Schema string // hn.migration.issue/1
    Title, Body, State, Type string
    Labels, Assignees []string
}
type CommentContent struct {
    Schema string // hn.migration.comment/1
    Body string
}
type IssueWrite struct {
    Schema string // hn.migration.issue-write/1
    Target *RemoteRef // absent for create, required for update
    Title, Body, State, Type string
    Labels, Assignees []string
}
type CommentWrite struct {
    Schema string // hn.migration.comment-write/1
    Parent RemoteRef // issue/work-item identity, often a predecessor receipt
    Target *RemoteRef // absent for create
    Body string
}
```

Source authors/times/identity remain the surrounding `Record` fields. Comment
records have exactly one required `issue`/`work_item` parent link; additional
thread/reply relationships stay explicit preserved links rather than flattened
native claims. Source state/type are descriptive strings; destination state/type
choices are explicit validated mappings. Empty type is meaningful only where
that provider has no work-item type. Reject unknown/duplicate/null fields, invalid
Unicode, unbounded text/collections and schema/action mismatches. Source limits
are recorded; larger unsupported native text is preserved as bytes with a finding,
not silently truncated. API DTO translation happens after this canonical bound
input and cannot acquire a different parent or alter operator decisions.

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
local audit-authority anchors, transfer journals or unselected working files.
Exact selected committed history
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
