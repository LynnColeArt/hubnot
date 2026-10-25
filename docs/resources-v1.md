# Native resources (alpha)

Projects, project items, discussions, replies and workflows are signed Git facts.
They need Git and a native identity for writes; reads need no identity, provider,
GitHub account, `gh`, HTTP service or runner. `nogithub` builds retain these
commands. Imported names, categories, lock claims and metadata remain descriptive
claims signed by the importing native actor. They grant no moderation, identity,
policy, maintainer or execution rights.

This adds `project`, `project-item`, `discussion`, `discussion-reply`, and
`workflow` `.open` / `.revise` event kinds to `hn/0`. Older alpha clients reject
these unknown kinds; upgrade clients before selecting actor histories containing
them. Existing event encoding, signatures and authority are unchanged.

## Commands and guards

Every family has this baseline, available before convenience commands:

```
hn project open --input project.json --operation create-project --json
hn project revise FULL_ID --expect FULL_REV --input project.json --json
hn project resolve FULL_ID --expect HEAD_A --expect HEAD_B --input project.json --json
hn project list --limit 50 --json
hn project show FULL_ID --json
hn project heads FULL_ID --limit 50 --json
hn project history FULL_ID --limit 50 --json
hn project operation --actor FULL_ACTOR --operation create-project --json
```

Substitute `project-item`, `discussion`, `discussion-reply` or `workflow` for the
family. `--input -` reads stdin. Mutation IDs and expectations must be full
`sha256:` IDs. Show, heads and history accept unambiguous prefixes of at least
eight hex digits. Mutations accept `--actor FULL_ACTOR` to pin the active signer.
Unknown commands and flags fail. JSON responses contain `ok`, `data` and an
`error` with code/message on failure. Exit status also reports failure.

Openings have no subject or parents, including child openings. Typed child state
contains its container. Revisions have subject = aggregate opening ID and sorted
unique exact parent IDs. Ordinary revise requires the one current head. Resolve
requires every current head (2..200); larger conflicts must be inspected rather
than silently truncated. Competing revisions stay visible as multiple heads and
`state: null`; neither timestamps nor actor names select a winner. Any actor may
append a valid collaborative revision: lineage and containers constrain facts,
without a new resource signer ACL. Reply nesting parents and containers cannot
change after opening. Item schema revisions may change explicitly.

Operation keys are optional printable non-whitespace ASCII (1..128 bytes), scoped
to the native actor across issues and every resource family. Reusing a key with
different kind, input, intent, root or expectations fails. A retry checks the
original signed request before inspecting current heads, preserving the original
result even after subsequent revisions. Issue operation queries reject resource
operations and resource queries reject operations from other families. Snapshots
bind accepted ref names/OIDs, not a repository-wide write lock; concurrent actors
can create visible conflicts. Pages use `--limit 1..200` and opaque `--cursor`
tokens bound to family, query, limit and accepted-ref snapshot. A changed snapshot
requires a fresh query.

## Typed state JSON

All inputs are objects with the exact names below. Schema strings are mandatory.
Unknown fields, duplicate object keys, malformed Unicode, null values, unsafe
identifiers and excess collections fail. Optional empty collections may be omitted.
Title/body limits are 1024 / 65536 UTF-8 bytes. Metadata is at most 64 namespaced
string keys (`tool/name`, <=128 bytes), each value <=4096 bytes. Metadata is never
a source of permissions. Stable IDs use 1..64 ASCII letters, digits, dots,
underscores or hyphens, excluding `.` and `..`.

Project (`hn.project/1`):

```json
{
  "schema": "hn.project/1", "title": "Roadmap", "body": "Planning", "archived": false,
  "fields": [
    {"id":"phase", "name":"Phase", "type":"single_select", "options":[{"id":"todo","name":"Todo"}]},
    {"id":"score", "name":"Score", "type":"number"}
  ],
  "views": [{"id":"board", "name":"Board", "layout":"board", "group_by_field":"phase"}],
  "metadata": {"team/context":"planning"}
}
```

At most 64 fields, 64 options per single-select field and 16 views. Fields are
`text`, `number`, `date` or `single_select`; only single-select has options. View
layout is `table` or `board`; optional `group_by_field` names a single-select field.
Field, option and view IDs are unique in their respective scopes. Names <=256 bytes.
Field/view order is retained. Revisions do not reinterpret earlier schema bytes.

Item (`hn.project-item/1`):

```json
{
  "schema":"hn.project-item/1", "project_id":"sha256:<64-hex>",
  "project_revision":"sha256:<64-hex>",
  "target":{"kind":"draft", "title":"Plan next release", "body":"Notes"},
  "values":{"score":{"type":"number","number":"2.5"},"phase":{"type":"single_select","option_id":"todo"}},
  "archived":false, "position":"0", "metadata":{}
}
```

`project_revision` must be an available project opening/revision of `project_id`.
Its exact field types/options validate at most 64 values. Value objects have one
type and only its applicable member: `text`, `number`, `date` or `option_id`.
An absent value is unset. Numbers and positions are precision-preserving canonical
decimal **strings**, <=64 bytes: no exponent, leading zeros, trailing fractional
zeros, plus sign, negative zero, infinity or NaN. Examples: `"0"`, `"-1.25"`,
`"100"`. Position defaults to `"0"`; it is descriptive ordering information.
Dates use exact valid `YYYY-MM-DD`. Text values <=4096 bytes.

Targets are `{kind:issue|proposal|discussion,id:FULL_ID}` (no other members),
`{kind:draft,title:TEXT,body:TEXT}` or `{kind:external,url:HTTP_OR_HTTPS_URL}`.
Issue/discussion targets name openings; proposal targets name exact proposal facts.
External URLs <=4096 bytes, contain no userinfo/control characters and grant no
native relationship authority. Draft and external target fields cannot be mixed.

Discussion (`hn.discussion/1`): `title`, `body`, `category` (nonempty descriptive
string <=256 bytes), `status` (`open` / `closed`), optional
`accepted_reply_revision` (full exact reply opening/revision ID), and `metadata`.
An accepted answer must belong to this thread. It pins that revision's content;
later reply edits do not change the accepted text.

Reply (`hn.discussion-reply/1`): `discussion_id` (thread opening), optional
`reply_to` (reply opening), nonempty `body`, and `metadata`. Replies have their own
revision histories. Nesting is at most eight levels, same thread, with no cycles.
Container and nesting-parent IDs are immutable across reply revisions.

Workflow (`hn.workflow/1`): `name` (existing pipeline name grammar), `title`,
`description` (<=65536 bytes), `enabled`, optional `pipeline` (existing validated
`hn.pipeline/0` definition, <=1 MiB), `source_documents`, and `metadata`.
Each source document is `{format:STABLE_ID,content:TEXT,digest:FULL_SHA256_ID}`;
its digest is SHA-256 of exact UTF-8 content bytes. At most eight documents, each
<=1 MiB; duplicate digests fail. Foreign content is inert. `enabled:true` requires
a native pipeline. Creation or revision never materializes, requests or runs it.
Workflow convenience commands explicitly materialize and bind native bytes to a
proposal before the existing governed run-request path.

## Integrity and limits

Resource state lives in existing `Event.body`, with existing actor-history
signatures, `subject`, `parents`, `intent`, `operation` and `request`. Unrelated
identity/policy/CI fields and `issue` state are refused. The authenticated event
ceiling is 256 KiB (2 MiB for workflows), including JSON escaping and envelope;
state satisfying individual bounds may still exceed this total. Strict new-kind
raw JSON checks do not re-encode or change old signed bytes.

Readers rebuild from verified accepted actor histories with existing budgets:
100,000 events/refs, 8 MiB objects, 256 MiB aggregate. Project schemas, item targets,
containers, nesting parents and answers participate in exact reference closure.
Missing dependencies quarantine incoming histories before promotion; selecting
the supplying actor histories allows recovery. Wrong types, unrelated lineage,
invalid schemas and cycles fail validation. A fresh clone reconstructs all state
from those signed histories without a local database or provider checkpoint.

Go consumers use `ResourceStateJSON`, `CanonicalResourceState`,
`DecodeResourceState`, `BuildResourceCatalog`, `MutateResource`, and
`ObserveResourceOperation`. Specialized commands register in
`NativeResourceCommands` and can delegate full-state commands to `CmdResource`.
