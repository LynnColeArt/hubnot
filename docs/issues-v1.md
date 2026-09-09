# Issues for people and automation

An issue keeps its opening event ID for its lifetime. Edits append signed
full-state revisions; comments append signed discussion. Existing title-only
issues remain readable. Assignment, criteria, labels, metadata and open/closed
status describe work; they do not grant execution, review or merge authority.

## A simple issue

In an initialized repository:

```sh
hn issue open --body "The description" "An issue title"
hn issue list
hn issue show <issue-id>
hn issue comment <full-issue-id> "Here is additional context."
hn issue help
```

Human creation prints a shortened ID. Run `hn issue show <short-id> --json`
to obtain the full `data.id`, or create with `hn issue open --json TITLE` and
use its full `data.issue_id`. Use full `sha256:<64-hex>` IDs for mutations.
Read commands accept a unique lowercase hexadecimal prefix, with or without
`sha256:`. Ambiguous prefixes fail. Mutation issue IDs and expected revision
IDs must always be full. Flags follow the issue ID and precede free text;
use `--` before literal text beginning with an option spelling.

## Rich state and guarded replacement

Save this complete example as `issue.json`:

<!-- example: rich-input -->
```json
{
  "title": "Make setup easier",
  "body": "Explain the first successful action and its expected result.",
  "status": "open",
  "criteria": [{"id": "first-run", "text": "A new contributor completes the documented example."}],
  "labels": ["documentation"],
  "assignees": ["docs-team"],
  "relations": [],
  "metadata": {"example/source": "onboarding"}
}
```

```sh
hn issue open --input issue.json --operation onboarding-create --json
hn issue show <full-issue-id> --json
hn issue revise <full-issue-id> --expect <current-head-id> \
  --input issue.json --operation onboarding-edit --json
hn issue close <full-issue-id> --expect <current-head-id> --operation onboarding-close --json
hn issue reopen <full-issue-id> --expect <closed-head-id> --operation onboarding-reopen --json
```

`--input -` reads the same JSON document from stdin. Input is the state object
itself, not an envelope or patch. A replacement resets omitted optional fields
to empty and defaults omitted `status` to `open`; copy fields you want to keep.
Explicit `null`, unknown keys, duplicate keys, wrong-case field names, trailing
JSON documents, malformed UTF-8 and unpaired surrogate escapes are rejected.
A properly paired Unicode escape or intentionally supplied U+FFFD is valid.
No malformed Unicode is silently repaired before signing.

Criteria have stable IDs local to the issue state and retain their listed order.
Labels, assignees and relations are sets, stored in canonical sorted order;
duplicates are rejected. Metadata values are strings, with keys such as
`example/source`: a nonempty namespace and name separated by `/`.
A namespace need not be an Internet domain. Assignees are descriptive strings,
not a trust-policy actor list. Criteria record expected outcomes, not approval.

## Limits

Limits are inclusive and byte lengths use UTF-8 bytes.

| Surface | Limit |
|---|---:|
| JSON request input | 256 KiB |
| New signed issue payload and canonical request | 256 KiB each |
| Title | 1–1024 bytes, nonblank |
| Body or comment text | 64 KiB |
| Criteria | 64; ID 64 bytes, text 4096 bytes |
| Labels / assignees | 64 each; 256 bytes per value |
| Relations | 128 |
| Metadata | 64 pairs; key 128 bytes, value 4096 bytes |
| Parents per signed revision | 1–200 |
| Operation key | 1–128 printable non-whitespace ASCII bytes |
| Query page | Default 50, maximum 200 records |
| Verified issue reader | 100000 events, 100000 enumerated actor/remote ref entries |
| Reader object / aggregate object bytes | 8 MiB / 256 MiB |

The reader counts event payloads, encoded signatures and run-log attachments.
It verifies all selected actor histories, including non-issue facts, because
actor continuity and evidence relationships cross event kinds. Oversized
legacy history receives a resource-limit diagnostic rather than being
reclassified as a bad signature. Replication has its own separate
[selection and promotion budgets](replication-v0.md).

## Reliable retries and concurrency

An optional operation key is scoped to the signing actor across issue commands.
Its signed request digest binds intent, event kind, issue, expected parents and
canonical requested content. Repeating exactly that operation returns the
original event ID with `replayed: true`, even after later edits or comments.
Changed content, target, expected parents or close/reopen intent with the same
key returns `operation_conflict`. Use a new key for a new intentional action.
Independent actors may use the same textual key without sharing an operation.

Replay is checked before current-head guards. Close/reopen retries recover
the originally signed state, rather than deriving different content from the
issue's latest state. Signed duplicate operation-key records fail explicitly;
Hubnot does not choose an arbitrary replay winner.

An edit captures the actor's append position, verifies an observed repository
snapshot, checks the expected issue head, then appends with actor-ref CAS.
`stale_revision` means the supplied observation no longer satisfies the guard.
`concurrent_update` means a read snapshot or this actor's append position changed.
Read again and decide deliberately; Hubnot does not silently rebase your request.

This is snapshot preflight plus CAS on one actor history, not a global issue
lock. Another actor can write after your snapshot. Both valid edits remain
visible after synchronization. A mutation's `head_count` and `conflict` describe
the catalog observed by that command plus its own fact, not global consensus.

## Conflict inspection and resolution

An unresolved issue has several maximal revision heads. `show` returns
`state: null` and `conflict: true`; list summaries have empty title/status.
No timestamp or arrival order selects a preferred head.

```sh
hn issue heads <full-issue-id> --limit 50 --json
hn issue history <full-issue-id> --kind revisions --limit 50 --json
hn issue resolve <full-issue-id> --expect <head-a> --expect <head-b> \
  --input resolved.json --operation resolve-example --json
```

Inspect every attributed head, write the desired complete state, and name every
observed head for ordinary resolution. Missing or non-current heads fail
without an append. The successor retains all named parents and earlier history.
`revise`, `close` and `reopen` refuse unresolved conflicts.

If there are more than 200 heads, page through `heads` and resolve in stages:

```sh
hn issue resolve <full-issue-id> --partial --snapshot <observed-snapshot> \
  --expect <selected-head-a> --expect <selected-head-b> \
  --input resolved.json --json
```

Select 2–200 current heads per partial resolution. The snapshot must still
match; even a newly appended comment invalidates this observation. Unselected
heads stay visible. Re-read heads/snapshot after each stage. For 201 heads,
consuming 200 leaves two: the new successor and the one unconsumed original.
Resolve those two explicitly to finish. An identical operation replay still
returns its recorded event before checking an old snapshot.

## Relationships and graph pages

A relation is `{"kind":"blocks","target":"sha256:<64-hex>"}`:

- `blocks`: the source issue blocks the target.
- `parent`: the source is a child of the target parent.
- `related`: displayed from either issue, signed on its original source.

Targets must name available issues, and self-links are invalid. Local writes
reject blocks/parent cycles they would introduce. Independently valid replica
edits can create cycles after sync: their facts remain readable with explicit
cycle diagnostics. A conflicted issue contributes links from every head with
`ambiguous: true` and the exact revision/actor provenance.

```sh
hn issue graph <issue-id> --limit 50 --json
```

Graph pages contain `type: "edge"` rows with an `edge` object
`{source,target,kind,revision,actor,ambiguous}`, and `type: "cycle_member"` rows
with `{kind,component,issue_id}`. A component ID groups individual cycle members;
`cycle_count` counts components involving the queried issue. Membership rows
are paginated too, so a large cycle cannot evade the output bound.

## Machine envelope and navigation

`--json` emits exactly one `hn.issue/1` JSON envelope on stdout. Failure returns
a nonzero process exit; stderr may also contain a human diagnostic. Parse
stdout separately. `--json` accepts Go boolean spellings: true values
`1/t/T/TRUE/true/True`, false values `0/f/F/FALSE/false/False`.
The last valid actual JSON flag selects the mode; an invalid boolean reports
`invalid_input` while preserving the previous valid choice. Option values,
free text and content after `--` are not JSON flags.

Success is `{schema,ok:true,data,snapshot?,next_cursor?}`. Failure is
`{schema,ok:false,error:{code,message}}`. The signed event protocol remains
`hn/0`; it is distinct from this CLI envelope version.

| Command | `data` |
|---|---|
| Mutation | `issue_id,event_id,replayed,conflict,head_count` |
| List | `items,total,conflict,cycle_count`; items have `id,title,status,creator,conflict,head_count` |
| Show | `id,creator,state,conflict,head_ids,head_count,revision_count,comment_count` |
| Heads/history/graph | `items,total,issue_id,conflict,head_count?,cycle_count` |
| Heads item | `id,actor,state` |
| History item | `id,actor,kind,timestamp,parents,state?,body?,intent?,operation?` |

All IDs in machine results are full. Empty collections are arrays. `show`
paginates head IDs; use `heads` for complete head states and `history` for
comments/revisions. History accepts `--kind all|revisions|comments`: revisions
are topological with event-ID tie breaks; comments follow by timestamp/ID.
`all` emits revisions before comments, not one mixed chronological stream.
List and heads use stable full-ID ordering. There are currently no list filters.

```sh
hn issue list --limit 50 --json
hn issue list --limit 50 --cursor <next_cursor> --json
hn issue history <issue-id> --kind comments --limit 50 --json
```

Treat continuation tokens as opaque. Keep command, resolved issue, history kind
and page size unchanged. The snapshot binds accepted actor ref names and OIDs.
A changed snapshot returns `stale_cursor`: restart the query. Wrong-query,
changed-limit, malformed and out-of-range tokens return `invalid_input`.
`total` is the complete record count, not the current page length. Stop when
`next_cursor` is absent; never infer completion from an empty selected state.

## Error handling

| Code | Meaning / next action |
|---|---|
| `invalid_input` | Correct request, ID, field, option or cursor; no local append. |
| `not_found` | Required issue is absent from accepted history. |
| `ambiguous_id` | Use a longer unique read prefix or full ID. |
| `stale_revision` | Re-read heads/snapshot and decide on a new request. |
| `issue_conflict` | Inspect all heads and resolve explicitly. |
| `operation_conflict` | Inspect the prior operation; use a new key for new intent. |
| `concurrent_update` | A read snapshot or actor append changed; inspect/retry deliberately. |
| `stale_cursor` | Restart pagination against the new observed snapshot. |
| `resource_limit` | Request/reader budget exceeded; reduce scope/content or inspect history. |
| `invalid_history` | Signature, chain, relationship or admission verification failed; inspect the diagnostic. |
| `repository_error` | Repository or subprocess failure; preserve the original diagnostic. |

Missing replication suppliers stay quarantined. Save explicit supplier actor
selections and sync again; do not reinterpret an unaccepted object as issue
state. See [selected replication](replication-v0.md) and the
[issue threat model](threat-model.md).

The [runnable consumer](../examples/issue-consumer/README.md) demonstrates
create, show/list, guarded revision, identical retry and typed stale rejection
using only this public interface. The public acceptance suites are:

```sh
go test -count=1 -run '^TestOperationalRichIssue' -v ./...
```

## Compatibility

Optional appended event fields preserve existing signed bytes. New issue
revisions require an upgraded executable; old versions may reject the new
kind. Reverting a binary cannot erase already signed events. This remains an
experimental protocol and does not perform a Beads migration or a Go Kitty
adapter installation.
