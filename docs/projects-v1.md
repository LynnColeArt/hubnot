# Native projects and GitHub Projects V2

Hubnot projects are signed planning state carried by Git. A project defines its
fields and views; each work item has an independent signed history and pins an
exact project schema revision. Native commands need Git and a native signing
identity for writes. Reads need no private key, GitHub account, `gh`, service or
runner. All commands below remain available in a `nogithub` build.

The shared [native resource contract](resources-v1.md) defines admission,
reference closure, JSON envelopes, conflict resolution and Git reconstruction.
The [transfer guide](github-interoperability.md) covers optional provider setup,
credentials, transport bounds and recovery. New resource event kinds require
clients that understand them; older alpha clients reject unknown kinds.

## Author a project

Save this as `project.json`:

```json
{
  "schema": "hn.project/1",
  "title": "Release plan",
  "body": "Work for the next release",
  "archived": false,
  "fields": [
    {"id":"phase","name":"Phase","type":"single_select","options":[{"id":"todo","name":"Todo"},{"id":"done","name":"Done"}]},
    {"id":"score","name":"Score","type":"number"},
    {"id":"due","name":"Due","type":"date"},
    {"id":"notes","name":"Notes","type":"text"}
  ],
  "views": [{"id":"board","name":"Board","layout":"board","group_by_field":"phase"}],
  "metadata": {"team/context":"release planning"}
}
```

```sh
hn project open --input project.json --operation release-plan --json
hn project open --title "Small plan" --body "No custom fields yet" --json
hn project revise PROJECT_ID --expect PROJECT_REV --title "Updated plan" --json
hn project revise PROJECT_ID --expect PROJECT_REV --input project.json --json
hn project resolve PROJECT_ID --expect HEAD_A --expect HEAD_B --input project.json --json
```

Replace `PROJECT_ID`, `PROJECT_REV` and other uppercase ID placeholders with full
`sha256:<64-hex>` values from command output. `open` returns `resource_id` and
`event_id` inside the JSON `data` object. Convenience title/body edits preserve
fields, views and metadata. Change field definitions, options, views, archival
state or metadata through full-state `--input FILE|-`.

Projects support `text`, `number`, `date`, and `single_select` fields, at most
64 fields, 64 options per single-select field and 16 views. Views are `table` or
`board`; `group_by_field` must name a single-select field. IDs are 1..64 ASCII
letters/digits/dot/underscore/hyphen, excluding `.` and `..`. Names are at most
256 UTF-8 bytes; title/body are at most 1024/65536 bytes. Field, option and view
IDs must be unique in their scopes. See the shared contract for metadata and
whole-event limits.

## Add and revise items

`hn project item ...` is the same interface as `hn project-item ...`. Save a
field-value map as `values.json`:

```json
{
  "phase":{"type":"single_select","option_id":"todo"},
  "score":{"type":"number","number":"2.5"},
  "due":{"type":"date","date":"2026-12-01"},
  "notes":{"type":"text","text":"Discuss API migration"}
}
```

```sh
hn project item add --project PROJECT_ID --target issue:ISSUE_ID --input values.json --position 2.5 --json
hn project-item add --project PROJECT_ID --draft-title "Investigate" --draft-body "Notes" --json
hn project-item add --project PROJECT_ID --project-revision OLD_PROJECT_REV --target discussion:DISCUSSION_ID --json
hn project-item archive ITEM_ID --expect ITEM_REV --json
hn project-item revise ITEM_ID --expect ITEM_REV --input item.json --json
hn project-item resolve ITEM_ID --expect HEAD_A --expect HEAD_B --input item.json --json
```

Targets are an issue opening, exact proposal fact, discussion opening,
`external:HTTP(S)_URL`, or explicit draft title/body. Targets must exist and have
the right type. The add command pins the sole current project head unless
`--project-revision` is explicit. A retry with the same operation key retains its
original implicit schema pin even if the project later changes.

Full item state in `item.json` has this shape:

```json
{
  "schema":"hn.project-item/1",
  "project_id":"sha256:<project-opening-64-hex>",
  "project_revision":"sha256:<exact-schema-64-hex>",
  "target":{"kind":"draft","title":"Investigate","body":"Notes"},
  "values":{"score":{"type":"number","number":"2.5"}},
  "archived":false,
  "position":"2.5",
  "metadata":{}
}
```

The placeholder IDs in that shape must be replaced before use. Values validate
against the exact pinned schema, including its option IDs. Revising the project
does not reinterpret old items. Migrate an item explicitly by revising its full
state with a different `project_revision` and compatible values. Archive
preserves its other state; clearing `archived` uses a full-state revision.

Numbers and positions are canonical decimal **strings**, at most 64 bytes. Use
`"0"`, `"-1.25"` or `"100"`; exponent notation, trailing fractional zeros,
negative zero, NaN and infinity fail. Dates are exact `YYYY-MM-DD`; text values
are at most 4096 bytes. An absent field value is unset. At most 64 values fit one
item, subject to the total authenticated-event ceiling.

## Inspect, replay and resolve

```sh
hn project list --limit 50 --json
hn project show PROJECT_ID --json
hn project heads PROJECT_ID --json
hn project history PROJECT_ID --limit 50 --json
hn project-item list --project PROJECT_ID --limit 50 --json
hn project-item show ITEM_ID --json
hn project-item heads ITEM_ID --json
hn project-item history ITEM_ID --limit 50 --json
hn project operation --actor ACTOR --operation release-plan --json
hn project-item operation --actor ACTOR --operation ITEM_OPERATION --json
```

Mutation commands accept `--actor ACTOR`, `--operation KEY` and `--json`. Actor
pinning checks the selected signer. Operation keys are actor-global across issues
and resource families, and reuse with different inputs fails. Ordinary revisions
require one current exact head; conflict resolution requires every current head.
No timestamp selects a winner. Conflicts expose all heads with `state:null`.

Read commands accept unambiguous ID prefixes of at least eight hex digits.
List/history pages use `--limit 1..200` and `--cursor TOKEN`; changing the accepted
Git ref snapshot invalidates a cursor. Item lists include archived items and
conflict heads. Ordinary items sort by exact numeric position, then ID; conflicts
follow by ID. Reads remain available after removing private identities or provider
credentials. Git replication reconstructs schemas, items and their exact target
and schema dependencies from accepted signed histories, without adapter journals.

## Optional GitHub Projects V2 transfer

Projects V2 ownership is separate from repository ownership. Select an existing
board with `user:LOGIN/NUMBER`, `org:LOGIN/NUMBER`, or `node:GLOBAL_ID`. Import does
not use `--key`. Export to `user:LOGIN` or `org:LOGIN` creates a board; selecting an
existing board requires verified source/publication association rather than
adopting an unrelated board.

```sh
hn transfer capabilities --provider github --resource project
hn transfer import --provider github --resource project --project org:TEAM/7 --dry-run
hn transfer import --provider github --resource project --project org:TEAM/7
hn transfer export PROJECT_ID --provider github --resource project --project org:TEAM --dry-run
hn transfer export PROJECT_ID --provider github --resource project --project org:TEAM
```

GraphQL reads and all writes require `HN_GITHUB_TOKEN`. Real project exports also
require the selected native identity to sign their private recovery journal;
resume with the same identity. Preview needs no native private key. Source author,
time and IDs are provenance, and never grant native or provider permissions.

| Data or behavior | Implemented transfer boundary |
| --- | --- |
| Title, readme, closed/archive state | Supported, with stable source identity and guarded destination baseline |
| Text/number/date/single-select fields and values | Supported; numeric conversion refuses precision loss |
| Native item ordering and archive state | Supported as separate journaled steps after item creation |
| Draft items | Supported with explicit title/body and publication association |
| Issue/PR content | Import resolves a verified native source mapping or requires an explicit signed native dependency mapping/external-URL retention; export requires an explicit destination Issue/PR node mapping unless verified same-source content suffices |
| Table/board view name and layout | Supported; provider-created defaults are retained |
| Iteration/multi-select, source content properties, colors/descriptions, unsupported layouts/settings | Affected findings require explicit per-finding omission |
| View filter/group/sort/visible columns; native view grouping | No complete equivalent; explicit omission required where reported |
| Project automation, linked teams/repos, visibility/template settings, permissions | Not migrated as equivalent configuration or authority |
| Schema field deletion, field type/option changes, target-kind changes | Manual migration; exporter refuses before writes |
| Native historical item schema pins | Retained natively; GitHub has one mutable board schema, so reconcile incompatible historical values explicitly rather than assume equivalent pinning |

Use the exact mapping keys printed by the dry-run findings. Import dependency
keys are `ref.KIND.<sha256-of-provider-ref>` with value `NATIVE_ID`; keeping only
a descriptive content link uses `external.<hash>=true`. Loss acknowledgments use
`omit.<hash>=true` for each reported path. Export content keys are
`content.<hash>=DESTINATION_NODE_ID`; native grouping omissions also use an exact
reported `omit.<hash>=true`. Hashes are lowercase 64-hex without the `sha256:`
prefix. Unsupported or unused keys fail; there is no blanket loss-acceptance flag.

Transfer graphs are limited to 256 nodes and 8 MiB of serialized data; a project
and its items all count toward that bound. Oversized graphs fail rather than
truncate. The adapter imports one complete graph before native mutation. Private
export state binds the selected native graph, destination, mappings and progress
under a canonical signature. Whole future journal bounds are checked before the
first remote mutation. Every field/item/value/view/state mutation is a separate
step; interrupted work can therefore be valid partial progress. Retry the same
command to reconcile exact positive remote evidence. Negative scans never prove
an uncertain write did not happen. Item-add recovery requires the created item
still be unarchived, appended, with no custom values; conflicting constructor
state requires manual reconciliation. Deleted mapped items, changed remote
baselines, ambiguous bindings and identity changes refuse. A single publisher is
required because GitHub supplies no atomic compare-and-swap for the whole board.

Before export, every populated item value must fit the current exported field
schema, even when the native item pins an older revision. Removed populated
fields, changed field types, or missing selected options refuse before any
provider write. Project title/body changes with compatible fields remain
exportable. Hubnot never silently repins or migrates an item; explicitly revise
its full native state and `project_revision` when a schema migration is intended.
