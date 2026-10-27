# Saved migrations and offline continuation

`internal/migration` defines the provider-neutral plan, intent, receipt and
`Destination` contract. Root `migration_*.go` owns native admission and the local
filesystem. Provider registration and the guided/scripted CLI are separate
consumers of these boundaries. Neither planning nor importing executes workflows,
fetches dependencies, selects audit authority or reads ambient credentials.

A caller obtains a snapshot through `LocalMigrationSource` (explicit selected Git
refs), `ArchiveMigrationSource`, or an optional provider `migration.Source`.
The returned native/archive blob source owns a temporary spool and must be closed
by its caller. `PrepareMigration` takes the observed snapshot, blob source,
selection, explicit destination and decisions, and writes a new private session
directory. The parent directory must already exist. Existing session directories
are refused. For repository operations put sessions below the resolved Git
directory, for example `.git/hn/migrations/<operator-selected-name>`; keep them
outside source/worktree and archive selections. `OpenMigrationSession` opens the
same saved observation; `ApplyMigration` applies or resumes it. No source refresh
occurs during resume. `InspectMigration` reads attributed records, links and
verified blob bytes without an adapter or private signing identity.

The canonical `hn.migration-plan/1` binds snapshot digest, destination, limits,
explicit findings/decisions, dependency-ordered intents and expected destination
baselines. Plans have no credential fields and do not capture environment values,
private signer stores, audit anchors or working files. Exact selected committed
content remains exact, including any secrets someone previously committed.
Decisions identify a specific finding and use `acknowledge`, `map` or `preserve`;
all unsupported, missing-dependency, identity-conflict and requires-mapping findings
need an explicit decision. A preserved finding describes retrieved bytes, never
an inaccessible URL or unobserved record. State/type mappings remain destination
planning decisions and may not be silently rewritten during application.

## Local and archive destinations

`LocalMigrationDestination` requires an existing explicit Git repository (which
may be a fresh bare repository). It binds the physical Git directory, object
format, HEAD selection and expected-old values of every destination observation.
The local/governance finding requires an explicit decision to preserve historical
source governance. This operation installs observations and an offline archive;
it does not activate source governance. Existing working files, branches,
policies, signing streams and private audit authority remain controlled locally.
Activating governance or adopting an audit authority is a later explicit operator
operation through the ordinary native commands.

Imported actor/proposal/memory streams use deterministic `refs/hn/remotes/`
observations scoped to the immutable source ID. Code observations use the same
scope under `code/<digest-of-original-ref>`; the saved snapshot retains each
original name. Code observations are never checked out or substituted for HEAD.
All admitted objects are copied by explicit object ID before an expected-old Git
ref transaction. Imported actors do not become local signing identities. A local
signer can create new native events referencing the observed histories afterward.

Retrieved foreign records, source-only conversations, links and downloaded blobs
remain in `.git/hn/imports/<snapshot-digest>.tar`. `ReadLocalMigration` validates
and opens that immutable archive for offline inspection and re-export. Foreign
reviews, users and workflows remain descriptive/inert; they grant no native
permissions. This code imports no provider clients and also works when adapters
are excluded. Existing `nogithub` behavior is unchanged.

`ArchiveMigrationDestination` takes an explicit absolute new output path. It
writes verified canonical archive bytes to a private temporary regular file and
publishes without replacement. Archive import/export uses the same snapshot and
native quarantine validation as local migration.

## Native admission and audit integration

Maintain the canonical project snapshot bridge for read-only export and bounded
audit auxiliary admission after approved WP02/WP04 integration. Export first
copies the bounded selected ordinary Git closure into disposable inspection
staging. Importing native readers only run there. Source refs, worktree and
object database remain unchanged. Signed legacy code dependencies are then read
from the source, and final raw material passes ordinary full-history checking.

Stage checks raw hashes and ordinary full Git structure before a native reader
unpacks an audit attachment. The canonical audit pack measurement accounts for
the union of ordinary and expanded audit object IDs against saved Git object,
per-object and aggregate limits before unpacking; shared IDs count once and
ambient destination objects grant no discount. Canonical native signature, pack
binding/shape, actor-chain, role and relationship validators admit facts. Audit
packs intentionally retain snapshot/path-proof closure: missing commit parents
or unrelated proof siblings are allowed there, while incomplete ordinary history
is refused. `ProjectQuarantine.Roots()` contains ordinary traversal roots only;
`ObjectIDs()` contains the exact admitted ordinary-plus-audit union. No duplicate
audit admission implementation is introduced.

Defaults are 10,000 records, 1,024 attachments/refs, 16 MiB per attachment or
manifest, 256 MiB aggregate attachments, 100,000 Git objects, 16 MiB per Git object
and 512 MiB aggregate expanded Git bytes, plus separately bounded raw framing.
Every saved declared blob is bounded, digest-verified and spooled before effects.
`ApplyContext.OpenBlob` opens only a declared verified spool entry, not a path or
URL. Later tampering is refused before writes.

## Recovery and dependent writes

Private HMAC-authenticated plan/snapshot/progress files and a separate random
private authentication key live in the session directory. These detect altered
checkpoints; possession of the directory/key is local operator authority, not
remote or native signing authority. Regular-file/permission/path checks reject
symlinks. An exclusive lock covers the whole apply/resume operation. A crashed
process may leave `apply.lock`; inspect the owning process before manually
removing that stale lock. Resume still reconciles every in-flight write.

Each immutable intent moves through `prepared`, `inflight`, `confirmed`. Before
in-flight state, the engine resolves typed references only from confirmed
predecessor receipts. It persists canonical actual payload bytes and digest;
apply and reconciliation receive copies of that same `BoundWrite`. For example,
a comment referring to the issue-create receipt uses exactly its confirmed
`remote_id`, including after a crash. Optional providers own no hidden parent-ID
map. `RemoteRef` has exactly one present branch (`id` or `receipt`); create
`target` must be absent, update `target` required. Unknown/duplicate/null fields,
case aliases, invalid Unicode and missing required source fields are refused by
the shared strict JSON check and the sole closed write validator.

A receipt must match plan, intent, kind/action, destination, actual payload digest
and observed identity/revision. An uncertain write stays in flight. Only `exact`
reconciliation with a matching receipt confirms it. `ambiguous` and `no_proof`
require manual recovery; negative lookup, timeout or inaccessible content never
authorizes repeating a create. The engine performs no automatic mutation retry.

Git objects, imported archive files, ref transactions and independent provider
writes cannot share a global transaction. Local application preflights stale
refs before publication, saves the verified archive, copies objects and atomically
promotes refs. A failure between these steps can leave a partial archive/object
installation. Reconciliation confirms only when the exact verified archive and
all intended refs are present; otherwise it reports manual recovery. Archive
reconciliation similarly verifies the exact saved snapshot. Successful resume
needs the saved session and destination, not the original source/provider.

## Shared provider and Git composition

`ComposeMigrationSources(primary, additional...)` joins a provider communication
source with `LocalMigrationSource{GitDir: ...}` using `Selection.Refs` for explicit
already-present ordinary code history. The result is one canonical snapshot and
Close-able blob source, preserving the primary provider's immutable identity and
original record attribution. Only one child supplies native Git material. This
boundary performs no provider-specific cloning or implicit network fetch; a
caller selects its available local Git repository explicitly.

`ComposeMigrationDestinations(communications, code)` routes communication intents
to the selected adapter and `git_refs` intents to the shared
`GitMigrationDestination{Remote, Destination, Mappings}`. Mappings explicitly pair
source refs and destination branches/tags. The closed
`hn.migration.git-write/1` payload binds snapshot digest, explicit remote and
`ref`/`oid`/`expected_old` values. No provider invents a code copier or embeds
arbitrary executable URLs. Supported transports are explicit absolute local
repository paths and credential-free HTTPS endpoints; remote helpers, relative
paths, embedded credentials, SSH command forms and other protocols are refused.
No Git credentials are discovered or saved by this seam; explicit transient authentication is described below.

Preparation proves every changed destination branch is an ancestor of its selected
source head. Existing tags cannot be replaced. Before any provider side effect,
apply rechecks pending Git baselines. A single atomic Git push uses exact leases
as compare-and-swap guards after the independent fast-forward proof; divergent
history is refused. Push does not delete source/destination refs, force divergent
history, check out code or activate foreign workflows. The remote must support
atomic push. There is no transaction spanning provider writes and Git; committed
progress exposes any partial result. A lost Git acknowledgement stays in flight;
reopening the same saved plan confirms only an exact positive advertisement of
all intended remote ref IDs. Missing or different refs require manual recovery.

Git code capture requires a complete already-present ordinary local repository;
partial/promisor sources are refused before object queries to avoid lazy fetching
or source object-database mutation. Intentional partial audit proof closure is a
separate signed native format and remains supported. The shared Git remote client
uses isolated configuration with local/public HTTPS defaults. Private HTTPS
requires `GitMigrationDestination.Credentials` containing explicit
`GitHTTPSCredentials{Origin, Authorization, CAFile}`. Authorization is bounded
Basic or Bearer input bound to the exact selected HTTPS origin. These fields are
excluded from JSON and remain only in the Git child's environment/configuration,
never a saved plan/progress/receipt, URL, command argument or diagnostic. Resume
requires fresh credential input; provider API tokens are never reused implicitly.
Credential helpers and URL rewrites are absent from the isolated configuration;
redirects are disabled, including across origins. TLS verification always remains
enabled; CAFile optionally supplies an explicit trust file, including for a local
TLS fixture. Git/server failures return generic redacted errors with actionable
access/authentication guidance. Existing
operator-controlled destination Git reference-transaction hooks retain normal Git
behavior. A hook refusal is an uncertain guarded write requiring reconciliation;
source hooks and imported pipelines are never executed by capture/import.

## Explicit continuation from imported observations

`LocalMigrationSource` captures selected current Git refs only. To retain imported
foreign communications during a later local-to-archive or local-to-provider
migration, discover bounded saved IDs with `ListLocalMigrations(gitDir, limits)`,
then explicitly select `ImportedMigrationSource{GitDir, SnapshotDigests}` and join
it with the current Git source through `ComposeMigrationSources`. The imported
source reuses `ReadLocalMigration` validation and contributes attributed foreign
records and non-Git blobs only. It never reselects stale archived code or private
journals. Current code/native histories still require explicit current ref
selection. Identical record/blob observations deduplicate; conflicting revisions
or metadata require an explicit snapshot choice. Aggregate record/blob budgets
apply across selected imports. Missing selected imports refuse the observation;
unselected imports are explicitly outside observed coverage.

Every destination implements required `Locator() migration.Locator` from its
resolved configuration. Planning checks equality with the selected destination;
apply/resume checks equality with the saved destination before preflight and
before each intended write. Composed destinations expose a valid locator only
when every child agrees. A changed provider configuration cannot create an
object first and discover the mismatch only in its receipt. Adapters must also
check immutable remote project/parent identity and their expected revision guards;
matching locator text alone does not prove a remote repository was not replaced.

Preparation durably seals an authenticated empty progress checkpoint alongside
the plan and observation. Its first apply binds the actor before effects. Missing
progress is damaged recovery evidence, even if the plan/snapshot still verify;
open/apply refuse it instead of creating a new journal and repeating writes.

Git HTTP children receive a disposable private empty home/config directory and
consistent platform aliases through their own process environment. Parent
home/configuration and files are never changed. Ambient NETRC/CURL_HOME overrides
are removed from that child so libcurl cannot consult an unrelated `.netrc` or
`_netrc`; helpers/rewrites remain disabled. A real synthetic outer credential
store regression proves unauthenticated private access does not consult/send
that marker, explicit supplied authorization still works, and outer files and
parent environment remain unchanged. Certificate verification remains enabled.
