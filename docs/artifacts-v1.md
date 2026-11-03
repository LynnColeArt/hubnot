# Portable typed artifacts — hn-artifact/1

Artifacts are generic signed **data**, available through `hn artifact` without a
registered executable. Document, ordered list and ticket shapes can hold custom
models while native `hn/0` and `hn-memory/0` histories remain unchanged. These
protocols are additive; pre-extension readers are not promised custom support.
No type name, signature, schema success, relationship or search match activates
doctrine, chooses policy, grants prompt/business authority or runs code.

## Public identity and wire

The normative byte grammar is the
[frozen artifact contract](https://github.com/LynnColeArt/hubnot/blob/d5aee5d0226c2e86615c1a0910d3cc8b9592cce7/kitty-specs/optional-extensions-01M661C0/contracts/artifact-v1.md).
It is a bounded Hubnot JSON encoding, not unrestricted JSON Schema or JCS.

For offline use, extract the source archive matching the package's `BUILD.json` /
`PROVENANCE.json` source revision, or use a local checkout at that revision, then
read `kitty-specs/optional-extensions-01M661C0/contracts/artifact-v1.md` as Markdown
or plain text. Binary packages omit that contract directory; their matching source
archive includes all four canonical contracts. No Go toolchain or GitHub runtime
connection is required. The [contract reference](extensions.md#contract-reference-online-or-offline)
links the same version-pinned host, artifact, fixture and default-workflow files.

Canonical JSON C has UTF-8-byte-sorted object keys, preserved array order, exact
Unicode scalar text without normalization, no BOM/whitespace/final newline.
Quotes/backslashes/control characters escape; less-than/greater-than/ampersand and
U+2028/U+2029 use the contract's lowercase Unicode escapes. Numbers are decimal
safe integers -9007199254740991..9007199254740991, with no fraction/exponent/plus,
leading zero or negative zero. Duplicate/unknown fields, malformed UTF-8/unpaired
surrogates and noncanonical signed payload bytes refuse.

D(bytes) is `sha256:` plus lowercase SHA-256 hex. Actor is SHA-256 of the raw
32-byte Ed25519 public key. Public key/signature are canonical unpadded standard
Base64. Signature preimage is UTF8(`hn-artifact/1`) || NUL || C(envelope);
artifact ID is D(C(envelope)), not a Git OID or digest of signature serialization.
Stream is D(UTF8(`hn-artifact-stream-v1`) || NUL || UTF8(actor)).

Every envelope contains exactly protocol, kind, actor, public_key, stream,
sequence, previous, timestamp, operation, request, record, parents, payload.
Kinds are type.publish, record.open, record.revise, record.resolve, record.convert.
Sequence starts at one; previous is empty only at genesis, otherwise the exact
prior signed ID in that actor stream. Timestamp is descriptive UTC RFC3339 seconds,
not causal order. Operation is 1..128 printable non-whitespace ASCII bytes.
Parents are increasing distinct full IDs, at most 1024.

Request identity is **(protocol, actual actor, operation)** across all custom
kinds/types, separate from native hn/0. Request digest is
D(UTF8(`hn-artifact-request-v1`) || NUL || C({actor,kind,operation,parents,payload,protocol,record})).
It excludes timestamp/stream position but binds exact content/order/schema/parents.
A frame correlation ID never substitutes for it.

Each actor has an append-only Git stream at
`refs/hn/artifacts/ACTOR/STREAMHEX`; accepted remote observations are
`refs/hn/remotes/REMOTE/artifacts/ACTOR/STREAMHEX`. Each commit has only two regular
100644 blobs: canonical `artifact.json` and unpadded `signature`, no newline.
The first commit has no parent; later commits have the prior stream commit as
sole parent. Actor histories extend by prefix; never merge divergent actor chains.
Readers verify Git structure, signature, identity, sequence/previous and closure.
SHA-1 and SHA-256 Git repositories retain the same artifact IDs.

## Types and pure schemas

Type payload is exactly `{type_id,schema_version,shape,schema}`. Type ID is
`namespace/name`, each segment `[a-z][a-z0-9.-]{0,63}`; hn/hubnot namespaces are
reserved. Schema version is 1..2147483647. Shape is document, list or ticket.
Schema has language=`hn.schema/1` and an object root; it validates only **fields**.

| Node type | Allowed other keywords |
|---|---|
| object | Required properties, required, additionalProperties=false |
| array | Required items and maxItems (0..1024) |
| string | Required maxBytes (0..1048576), optional enum |
| integer | Optional inclusive minimum/maximum and enum, safe integers only |
| boolean | Optional enum |
| null | None |

Required lists and enums are distinct; enums have 1..1024 same-type values.
Field names match `[A-Za-z_][A-Za-z0-9_.-]{0,63}`. At most 64 properties/object;
unknown fields refuse. There are no defaults/coercions, remote/schema references,
regex validators, unions, recursion, code or executable validators. Unknown schema
semantics is unsupported_schema, never accepted validity. Modeled object/array
depth is at most eight, counting the modeled root as one, not schema keyword maps.

Initial publication has no parents. Evolution names one verified prior declaration
of the same type/shape with strictly smaller schema_version. Different actors can
publish the same name/version; names confer no namespace ownership/winner. Records
pin an **exact declaration ID**. A new declaration never reinterprets old records.

## Record shapes, order and links

Every record payload has shape, type_id, type_revision, title, content, fields,
members, relationships, conversion_sources. Title is nonblank UTF-8 1..1024 bytes;
content preserves exact UTF-8 text, possibly empty; no binary attachment. Fields
is the full object validated by its exact declaration. Envelope size bounds the
aggregate content. Document/ticket have members=[]; ticket business statuses are
ordinary schema fields, not built-in lifecycle authority.

A list member is `{member_id,target,target_mode,required}`. IDs are unique within
the revision and match `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`. **Array position is order**,
not member ID or timestamp. Retain a member ID when retaining its identity. A
relationship is `{relation,target,target_mode,required}`; relation uses type-ID
grammar. Relationship order is preserved but is not execution order. No duplicate
relationship tuple; at most 256 relationships and 1024 members.

Targets are custom-record full IDs. `logical` names an opening and selects all
current heads; `revision` pins an exact signed opening/successor. Required links
join public closure; missing required schema/target is incomplete_dependency.
Missing optional targets produce missing_optional diagnostics. A conflicted
logical target produces target_conflict, never a chosen winner. Domain cycles
terminate through bounded visited-set traversal; ancestry/stream cycles are invalid.
Other-protocol links are descriptive fields, not silently traversed dependencies.

| Operation | Parent/schema behavior |
|---|---|
| open | record=null at CLI, parents=[], conversion_sources=[]; creates logical opening ID |
| revise | One current sole head, unchanged exact type revision, conversion_sources=[] |
| convert | One current sole head, same type/shape/opening, new exact declaration, conversion_sources=[parent]; full new data, no converter runs |
| resolve | Every current head in increasing order; explicit chosen schema/full data; conversion_sources exactly the parents with a different schema |

Conflict has multiple heads and state=null. Stale/missing/extra parents refuse
stale_revision; revise on conflict refuses resource_conflict. Do not silently
choose timestamps, update parents, partially resolve or retry changed intent.
Imported historically valid branches remain facts even when their parents are
not today's heads. A third branch after an earlier two-head resolution can still
leave a conflict. Same-actor stream divergence is actor_stream_conflict.

## CLI and exact recovery

```text
hn artifact type publish --input FILE|- --actor ACTOR --operation KEY --json
hn artifact type show FULL_ID --json
hn artifact open|revise|resolve|convert --input FILE|- --actor ACTOR --operation KEY --json
hn artifact show|heads|history FULL_ID --json
hn artifact list [--type TYPE_ID] [--shape document|list|ticket] --limit 1..200 [--cursor TOKEN] --json
hn artifact search [--type TYPE_ID] --literal TEXT|--regex PATTERN --limit 1..200 [--cursor TOKEN] --json
hn artifact operation --type TYPE_ID --actor ACTOR --operation KEY --json
hn artifact inspect --id FULL_ID --json
hn artifact export --id FULL_ID [--id FULL_ID] --output FILE --json
hn artifact import --input FILE|- [--preserve-opaque] --json
hn artifact push|fetch --remote NAME --id FULL_ID [--id FULL_ID] --json
```

Vertical alternatives describe command grammar, not literal shell commands.
Type publication input is `{parents,payload}`; record mutation input is
`{record,parents,payload}`. --actor/--operation are separate from input. Use real
returned full IDs; no prefix lookup. JSON uses `{ok,data,error}`, exits 0/1, and the
[closed error/recovery vocabulary](extensions.md#closed-errors-and-immutable-recovery).
The [copyable inventory journey](extension-consumers.md#inventory-publication-and-plugin-free-reading)
generates the IDs used for its later commands.

These exit/JSON rules describe direct `hn artifact` commands. When a publisher
uses `hn extension run`, that parent command reports session management; a
completed session can still write a business `ok:false` to its private result
file. Inspect that file separately, as explained in the
[extension lifecycle](extensions.md#explicit-lifecycle).

Persist original actor/key, target, exact payload and parents before sending.
Exact replay looks up the key **before** comparing current heads, returns original
identities and appends nothing; changed replay is operation_conflict. For a new
intent, validate actor/history/schema/closure/policy and exact heads, persist a
private immutable intent, sign, publish by expected-old CAS, durably record result,
then acknowledge. Custom writes under selected contribution policy refuse
unsupported_capability before intents/signing/publication.

After lost acknowledgement, run operation with original type/actor/key. Found
proves original request/result; verified_absent requires complete verified history
and no pending publication evidence; uncertain requires inspection/settlement and
never authorizes a new key or changed request. Pending private intent can keep a
publicly absent operation uncertain. Missing intent does not negate a found public
operation. CAS concurrency uses retry_identical, not an automatic resign/retry.
Private intent/query state is not public proof and operation lookup does not repair
or create an intent. Global exactly-once or remote API reconciliation is not promised.

## Plugin-free exchange and generic reads

Configure a real Git remote explicitly in both repositories. Push/fetch select
IDs and required public closure; ordinary `git clone` alone does not promise custom
refs. Closure includes actor predecessors, opening/parents, exact schema ancestry,
conversion sources and required typed links/members. Exact historical selections
never silently become latest; select a later revision separately when desired.

Selected file exchange is closed UTF-8 JSON:
`{protocol:"hn-artifact-exchange/1",mode:"accepted"|"opaque",refs:[{ref,oid}],objects:[{oid,type,data}]}`.
Object type is blob/tree/commit; data is canonical unpadded Base64 raw Git body.
Verify each Git hash and complete selected closure before expected-old atomic
accepted-ref promotion. Failure promotes none of that transaction. No tag/private
file/signing key/registration/grant/journal enters public export. Git network
download is not claimed to have a hard pre-download byte quota.

`inspect --id` reports signed selected closure; locate the requested exact item.
`show`, `heads`, list and search report current logical heads; history preserves
old schema/content/order. Search matches current title/content, canonical fields
and public identity metadata. CLI search has **no --shape**; CLI list does. Host
artifact.search separately accepts a shape parameter. Cursors bind query/filter
and source cut; invalidation means restart without cursor. No silent truncation.

Opaque preservation requires explicit `import --preserve-opaque`; it remains
unverified diagnostic public quarantine, never ordinary accepted data. Opaque ID
is D(UTF8(Git object type) || NUL || raw Git body), not a signed artifact ID.

```text
hn artifact inspect --opaque --opaque-id FULL_OPAQUE_ID --json
hn artifact export --opaque --opaque-id FULL_OPAQUE_ID --output FILE --json
```

Only explicitly preserved public quarantine objects are addressable; no arbitrary
ODB, working-tree, signing key or private journal access. Opaque export is not
accepted import/promotion. Diagnostics distinguish incomplete_dependency,
unsupported_schema, target_conflict, missing_optional, invalid_history and
opaque_unverified; they do not grant semantic authority.

Removing the optional module/binary leaves generic type/read/history/search/export
and native collaboration available. Transfer public history, never source private
identity/registration/grants. Native commands ignore unrelated malformed/unknown
custom histories; generic artifact reads may diagnose them.

## Bounds and inert utilities

Equality is accepted when otherwise valid; first-over refuses, not truncates.

| Boundary | Maximum |
|---|---|
| Canonical envelope / private intent file | 1 MiB / 2 MiB |
| Type payload / included schema | 128 KiB / 64 KiB |
| Modeled schema/value container depth | 8 |
| Properties/object; arrays/list/parents/enums | 64; 1024 |
| Relationships | 256 |
| Exchange wire / decoded aggregate | 16 MiB / 32 MiB |
| Exchange objects / signed records / ancestry depth | 4096 / 2048 / 256 |
| Read selected refs/records | 100000 |
| Read Git object / aggregate | 8 MiB / 256 MiB |
| Regex UTF-8 bytes / expanded-compiled instructions | 4096 / 4096 |
| Regex complete-selection instruction-bytes | 67108864 |

Regex uses bounded Go syntax and validates the whole filtered work budget before
output. Narrow type selection/simplify regex on resource_limit. One oversized
complete state cannot be fixed by reducing page size; use explicit export.
Read/copy/validation stages have finite 30-second contexts; publication uncertainty
still applies if dispatched I/O cannot promptly settle.

`hn artifact schema|record|type validate --input FILE|- --json` checks bounded
supplied values, not publication or business authority. `hn artifact envelope
validate|inspect --input FILE|- --signature RAW_BASE64 --json` verifies canonical
public bytes/signature. Optional paired --previous-input FILE and
--previous-signature RAW_BASE64 prove only that supplied adjacent predecessor;
without it a non-genesis envelope reports continuity not_checked. Inspect adds
the decoded envelope. Neither utility proves complete history/schema/closure or
accepted visibility. See the [normative limits and utility contracts](https://github.com/LynnColeArt/hubnot/blob/d5aee5d0226c2e86615c1a0910d3cc8b9592cce7/kitty-specs/optional-extensions-01M661C0/contracts/artifact-v1.md#6-published-bounds-and-refusals).
