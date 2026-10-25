# Optional forge interoperability

Hubnot stores issues, planning projects, discussions and workflows as signed Git
facts. Those resources work without GitHub, a `gh` executable, a service or a
database. `hn gh issue` is a parallel local command interface. Network transfer
is a separate, explicitly invoked operation. GitHub is the first optional
adapter; provider-neutral transfer types and dispatch permit other adapters
without making their SDK or service a native requirement.

Native guides: [issues](issues-v1.md), [projects](projects-v1.md),
[discussions](discussions-v1.md), [workflows](workflows-v1.md).

## Build with or without GitHub

Go 1.26+ and Git 2.x are required. The product uses the Go standard library and
Git; there is no external Go module or `gh` subprocess dependency.

```sh
go build -trimpath -o hn .
go build -trimpath -tags nogithub -o hn-local .
go list -deps -tags nogithub ./...
```

The tagged dependency graph excludes `hubnot/internal/githubapi`. Both binaries
retain native issue, project/item, discussion/reply, workflow and local
`hn gh issue` operations. Previously imported facts remain readable and editable
with the native commands. `hn-local github help` explains transport exclusion;
GitHub transfer operations fail as an unavailable provider before network access.
No account or token is needed for local operation. Private native keys are
needed for new native signed facts, not ordinary reads.

## Local gh-style issue interface

This is `hn gh`, not an executable replacement for GitHub's `gh`. It never
contacts a forge, reads GitHub credentials, fetches tickets or performs sync.

| Command | Supported flags |
|---|---|
| `hn gh issue create` | Required `--title`/`-t`; `--body`/`-b`, `--label`/`-l`, `--assignee`/`-a` |
| `hn gh issue list` | `--state`/`-s open\|closed\|all`, `--label`/`-l`, `--assignee`/`-a`, `--limit`/`-L`, `--json FIELDS` |
| `hn gh issue view ID` | `--comments`, `--json FIELDS` |
| `hn gh issue edit ID` | `--title`/`-t`, `--body`/`-b`, `--add-label`, `--remove-label`, `--add-assignee`, `--remove-assignee` |
| `hn gh issue comment ID` | Required nonblank `--body`/`-b` |
| `hn gh issue close ID` / `reopen ID` | No lifecycle-specific flags |

Every supported command accepts `--help`/`-h`. Value flags accept `--flag=value`
or a separate argument. Flags may appear before or after the ID; `--` ends flag
parsing. Label and assignee values may repeat or contain comma-separated names;
they form sets. List filters require every selected label/assignee and apply
before the limit (default30, maximum100000). Conflicted issues remain visible
with unknown state. Mutations require the full native `sha256:<64-hex>` issue
ID; view accepts a unique native hexadecimal prefix. Numeric GitHub issue
numbers and `--repo` are not native identifiers.

```sh
hn gh issue create -t "Improve setup" -b "Document the first successful run" -l docs
hn gh issue list --state all --json id,title,state,conflict
hn gh issue view <full-native-issue-id> --json id,title,body,comments
hn gh issue edit <full-native-issue-id> --add-label onboarding
hn gh issue comment <full-native-issue-id> -b "A local signed comment"
hn gh issue close <full-native-issue-id>
hn gh issue reopen <full-native-issue-id>
```

Successful compatibility mutations print the full issue ID as plain text.
They observe the native head and use the same signed writer as `hn issue`;
conflicts refuse a hidden winner. Use native `hn issue` for explicit expected
heads, operation keys and conflict resolution. Unsupported flags, JSON fields,
`gh pr`, `gh run`, `gh api`, GraphQL forwarding, and other command families fail
with a nonzero exit. There is no unmodified-`gh` HTTP compatibility service.

### Compatibility JSON

`--json` is a comma-separated, duplicate-free selected field list, supported on
list and view. List emits an array; view emits one object. This is separate
from the native `hn.issue/1` envelope.

| Field | Value |
|---|---|
| `id` | Full stable native issue ID |
| `title`, `body` | Strings, or `null` for a conflicted state |
| `state` | `"OPEN"`, `"CLOSED"`, or `null` for a conflict |
| `labels` | Array of `{"name":"docs"}`, or `null` for a conflict |
| `assignees` | Array of `{"login":"name"}`, or `null` for a conflict |
| `author` | `{"login":"<native opening actor fingerprint>"}` |
| `url` | Recorded descriptive source URL, otherwise `null` |
| `conflict` | Boolean |
| `comments` | View only: array of `{id,body,author:{login},createdAt}` |

Comment authors are native signers, including for attributed imported comments;
source authors/times remain in the attributed text and metadata. Empty arrays
are complete empty results, not missing pages. Read failures and stale snapshots
refuse incomplete answers. For example:

```json
[{"id":"sha256:<64-hex>","title":"Improve setup","state":"OPEN","conflict":false}]
```

## Explicit transfer commands

```text
hn transfer capabilities --provider github [--resource issue|project|workflow|discussion]
hn transfer import --provider github --project SELECTOR [--resource RESOURCE] [--key KEY] [--endpoint REST_BASE] [--map KEY=VALUE]... [--dry-run]
hn transfer export FULL_NATIVE_ID --provider github --project SELECTOR [--resource RESOURCE] [--endpoint REST_BASE] [--map KEY=VALUE]... [--dry-run]
```

`issue` is the default resource. Export's full native ID precedes the flags.
Import `--key` selects one external record where supported; it is never a native
mutation ID. `--provider` and `--project` are explicit. Each mapping key may
appear once; at most64 nonempty `KEY=VALUE` mappings are accepted (key128 bytes,
value4096 bytes).
Unknown or unused mappings fail rather than silently changing meaning.

The installed capability report is JSON with `provider` and `capabilities`.
Each capability names `resource`, `import`, `export`, `mapped_fields` and
`unsupported`, with a `reason` when unavailable. Typed migration plans use
`resource`, `direction`, `nodes` and `findings`; finding statuses are `mapped`,
`preserved`, `requires_mapping`, `unsupported`, `missing_dependency` and
`identity_conflict`. A supported capability does not mean all foreign semantics
can be converted. Inspect the plan/error and provide only intended mappings.
Issue transfer prints a human import/export/validation summary rather than that
typed plan JSON. Dry-run reads the selected provider and validates the plan, but
creates no signed facts, export mutations or recovery checkpoint. It can still
need provider read authorization.

The convenience aliases cover **issues only**:

```sh
hn github import --repo OWNER/REPO --issue 42 --dry-run
hn github import --repo OWNER/REPO --issue 42
hn github export <full-native-issue-id> --repo OWNER/REPO --dry-run
```

Their flags are `--repo`, `--api-url`, `--dry-run`, and import-only `--issue`.
Omitting `--issue` imports the bounded issue listing. Use canonical `hn transfer`
for the other resources and repeated `--map` options.

## Supported GitHub mappings

| Resource | Selector and supported transfer | Required decisions / manual limits |
|---|---|---|
| Issue | `--project OWNER/REPO`; import `--key 42` or bounded listing. Title, body, open/closed, labels, assignee claims and attributed comments | No `--map` keys. PRs are excluded. Milestones/reactions are not migrated; attachment references remain external URLs. Native-only fields use the export representation described below. |
| Discussion | `--project OWNER/REPO`; import numeric `--key` or listing. Thread/replies, one parent level, source attribution, category, state and current answer content | `--map history=snapshot` in both directions; export `--map category=EXISTING_GLOBAL_CATEGORY_ID`. Poll/lock/vote/reaction/minimized/deleted observations require `--map losses=acknowledge`. No complete edit/deletion history, category creation, moderation transfer or automatic historical-answer replacement. |
| Project | Planning board selectors `user:LOGIN/NUMBER`, `org:LOGIN/NUMBER`, `node:GLOBAL_ID`; export may select an owner `user:LOGIN` or `org:LOGIN` for creation | Supported text/number/date/single-select fields, table/board layouts, draft or explicitly mapped items, values/order/archive. Content references and each unsupported setting/field/view omission require explicit mappings. Project automation, permissions and arbitrary schema migration are not equivalent. |
| Workflow | `--project OWNER/REPO`; import optional workflow `--key` and `--map source-ref=REF`. Read exact YAML at a resolved commit | Source is inert by default. Explicit native pipeline/generated-subset mapping is required for native execution. Export requires format, branch, path and expected blob SHA, plus explicit execution environment choices for generated Actions. Secrets, arbitrary Actions features and provider enable/disable authority are not migrated. |

See [discussion](discussions-v1.md), [project](projects-v1.md) and
[workflow](workflows-v1.md) guides for exact resource flags, JSON state examples,
per-finding mapping keys and refusal boundaries. Project board ownership is
separate from repository ownership; `OWNER/REPO` is not a Projects V2 selector.

```sh
hn transfer import --provider github --resource discussion --project OWNER/REPO \
  --key 7 --map history=snapshot --dry-run
hn transfer export <full-discussion-id> --provider github --resource discussion \
  --project OWNER/REPO --map history=snapshot --map category=<global-category-id> --dry-run
hn transfer import --provider github --resource project --project org:TEAM/3 --dry-run
hn transfer import --provider github --resource workflow --project OWNER/REPO \
  --key 12 --map source-ref=main --dry-run
```

Copying a workflow file to an explicit branch can activate provider triggers.
A successful transfer is not a claim that foreign jobs, runner isolation,
policy, secrets or permissions have become equivalent to native execution.

## Access and transport bounds

Only explicit transport commands contact the selected adapter. `HN_GITHUB_TOKEN`
is read by that adapter's factory and is never stored in signed facts, private
checkpoints or diagnostics. REST issue/workflow public reads may be anonymous;
private reads and all writes need a token with sufficient repository access.
**Every GraphQL read**, including public discussions and Projects V2, requires
authentication. The token must also have the selected resource's read/write
permissions. Missing scopes, inaccessible data and provider policy failures are
errors, not empty resources. Core commands and `hn gh` do not inspect this token.

The default REST base is `https://api.github.com`. Override it explicitly with
`--endpoint` or issue-alias `--api-url`, for example a GitHub Enterprise
`https://HOST/api/v3` base. GraphQL derives `/api/graphql` from that enterprise
base, and `/graphql` from the root base; it never appends `/graphql` to `/api/v3`.
HTTPS is required except explicit loopback-IP HTTP for local fixtures/development.
URL userinfo, query/fragment, ambiguous base paths, credential-bearing redirects
and cross-endpoint pagination are refused. No source URL from imported metadata
is fetched merely because it is present.

| Bound | Limit |
|---|---:|
| Whole transfer command deadline | 5 minutes |
| Individual HTTP request | 30 seconds |
| HTTP response | 4 MiB |
| SDK method aggregate response bytes | 64 MiB |
| REST listing / GraphQL connection | 100 pages, at most100 nodes per page |
| Provider-neutral typed graph / nodes | 8 MiB / 256 |
| Private transfer checkpoint, including future progress/results | 8 MiB |
| Individual workflow source document / workflows per read | 1 MiB / 256 |
| Native issue body/comment | 64 KiB, plus the complete256 KiB signed-event bound |

Native project/discussion/workflow limits and verified Git reader budgets are
also enforced. Attribution, source metadata, JSON escaping and publication
markers consume those bounds. A foreign record that cannot fit is rejected,
not truncated. GraphQL errors or partial data fail even with HTTP200; malformed,
ambiguous or invalid-Unicode JSON and incomplete/cyclic pagination fail closed.
Transport errors exclude remote response bodies and credentials. Mutations are
not automatically retried.

## Provenance, repeat and conflict behavior

Imports create normal native signed facts. Original provider author, timestamps,
URLs and IDs are source claims, not a native signer or policy role. Labels,
assignees, closed state, project visibility, categories and source moderator
state grant no review, merge, runner or execution authority.

Issue opening metadata uses `forge/source`; current signed observations and
managed baselines use `forge/observation` and `forge/baseline`. Typed resources
use `forge/resource-source`, `forge/resource-observation` and
`forge/resource-baseline`. Stable source identity includes provider, endpoint,
kind where applicable, and immutable provider ID; mutable lookup selectors do
not replace it. Mappings and baselines are checked against verified signed
import operations, not trusted merely because current metadata contains them.

Repeated unchanged imports do not duplicate records. Changed source can update
an unchanged native projection; unchanged source preserves local edits. Competing
native/source changes, conflicting native heads, duplicate identity claims,
missing dependencies and ambiguous publication markers refuse automatic
resolution. Imports preflight complete selected records/graphs, but multi-event
imports are resumable work, not transactions: interruption can leave valid
partial native progress, recovered by the same exact operation intent.

Issue export to a new destination preserves criteria, relations and nonreserved
metadata in a readable descriptive JSON appendix with native attribution and a
terminal structured proof. These fields do not become GitHub checklists,
relationships or authority. Returning an imported issue to its verified exact
source keeps its ordinary body; extra native-only criteria/relations/metadata
then require an explicit lossless source-body mapping, which is not implemented,
so export refuses. Verified source-origin comments are not echoed back. Source
comment edits append attributed native corrections; deletions do not erase
immutable native history. Exported comment receipts preserve remote edits or
deletions instead of recreating old comments.

A source roundtrip uses stable identity and verified provenance to avoid footer
feedback. Re-importing an exact native publication without proven mapping
refuses duplicate/remapping. Terminal markers are descriptive and forgeable;
marker text alone never grants native authority. Ordinary malformed marker-like
prose remains content. If a remote-only issue body/footer edit destroys the
association, an unchanged already-published native snapshot can be preserved
read-only; new unpublished work requires explicit manual association recovery.
That path does not adopt the external edit as a new overwrite baseline.

## Private recovery and publication

Private state lives under `hn/forge` in the repository's resolved Git directory,
usually `.git/hn/forge`: directories0700, regular files0600, atomic replacement.
It is not Git protocol authority, is not cloned, and contains no provider token.
Removing the adapter or keys does not remove imported signed native content.

Issue, discussion and project real exports require the active native signing
identity to authenticate their complete canonical private recovery journals.
Use that same identity for resume. Identity switches, unsigned old journals,
altered progress/bindings or noncanonical envelopes refuse with an explicit
recovery error. Dry-run remains private-key-free; an existing signed journal can
be checked with its public proof. The signature authenticates local export
intent and progress; it does not prove a remote write succeeded or grant any
provider permission. Workflow recovery instead binds its exact native snapshot,
explicit destination, source bytes and guarded blob-write intent; it does not
create a new native execution authorization.

Only one local transfer may hold the private `transfer.lock`. After a crash,
first establish that no transfer is running before removing a stale lock; do
not delete a journal to bypass uncertainty. Multiple publishers/clones are not
a distributed lock and cannot promise globally exactly-once creation.

Each external mutation is a separate `prepared` → `in_flight` → `confirmed`
step. Intent is persisted before the one write. On a lost response, retry the
same command and selection: recovery must find one exact positive match for
its intended identity/content. A complete negative scan is not proof that a
write never happened and does not authorize another creation. Ambiguous,
changed, deleted or inaccessible results require manual reconciliation.
`clientMutationId` is correlation, not an idempotency guarantee. A native change
made during recovery is a later export; the saved snapshot is not silently
refreshed.

Remote APIs do not provide an atomic compare-and-swap for all collaboration
fields. Baseline checks narrow the race but cannot eliminate an external edit
between read and write. Inspect manual failures, preserve the journal and
remote data, and recover deliberately. No transfer command provides background sync,
a merge policy engine, automatic protected checks, automerge, or GitHub-wide
API emulation.


## Adding another provider

The internal extension seam is `internal/forge`: an adapter supplies its bounded
identity, opaque IDs and selectors, issue methods and/or the selected typed
resource-reader interfaces. An optional registration supplies a factory, its
credential variable, explicit capabilities and resource export callbacks.
Transfer planning, signed native storage and recovery stay provider-neutral;
HTTP clients and provider DTO conversion stay inside the adapter. A new adapter
must validate its own network endpoints, implement complete bounded reads and
single-mutation writes, and report missing/lossy mappings before mutation.

This is an internal implementation boundary, not a runtime plugin loader or a
stable external SDK promise. No unspecified alternate forge or workflow format
is guessed. A provider's API or CI engine becoming unavailable cannot prevent
native Git-backed collaboration.
