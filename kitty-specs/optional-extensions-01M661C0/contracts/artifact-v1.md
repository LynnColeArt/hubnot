# Portable artifact contract — hn-artifact/1

WP02 freezes this additive contract for implementation and independent review.
It does not change hn/0 or hn-memory/0 or claim the future implementation passes.
All records are inert data; attribution, generic validity, application semantics,
accepted visibility, project authority and execution are distinct.

## 1. JSON, signatures and identities

C(value) is the following canonical JSON encoding, with no BOM, surrounding
whitespace or final newline. Object keys sort by unsigned UTF-8 bytes; array order
is retained. Strings contain Unicode scalar values, retain exact content without
normalization, and use UTF-8 except: quotation mark/backslash escape; controls use
\b, \t, \n, \f, \r where applicable and otherwise lower-case \u00xx; less-than,
greater-than and ampersand use \u003c, \u003e, \u0026; U+2028/U+2029 use their
lower-case \u escapes. Slash does not escape. Numbers are decimal integers in
[-9007199254740991,9007199254740991], with no plus, exponent, fraction, leading
zero or negative zero. Boolean and null use JSON lower-case tokens. Reject
duplicate keys, malformed UTF-8, unpaired surrogates, unknown envelope/payload
fields and a signed payload whose bytes differ from C(parsed value).
Host request JSON uses the same value domain but need not have canonical spacing
or object ordering. This is a bounded Hubnot encoding, not general JSON Schema/JCS.

Digest ID D(bytes) = "sha256:" followed by 64 lower-case hex SHA-256 digits.
Actor = 64 lower-case hex SHA-256(raw 32-byte Ed25519 public key), reusing native
identity. Public keys and 64-byte signatures use unpadded standard Base64, with
canonical round-trip encoding. No private key enters any public artifact.
Stream = D(UTF8("hn-artifact-stream-v1") || 00 || UTF8(actor)); one stream per actor.
The signature preimage is UTF8("hn-artifact/1") || 00 || C(envelope).
The artifact ID is D(C(envelope)); signature is a separate verified object.
Git object IDs, timestamps, local paths and signature serialization are not
artifact identity. Every signature must verify with the public key bound to actor.

Every envelope has exactly these required fields, including empty strings/arrays:

| Field | Type and rule |
|---|---|
| protocol | literal "hn-artifact/1" |
| kind | type.publish, record.open, record.revise, record.resolve, record.convert |
| actor | full actor fingerprint |
| public_key | canonical public key above |
| stream | actor's deterministic stream ID |
| sequence | integer 1..9007199254740991 |
| previous | empty at sequence 1; otherwise preceding signed artifact ID in this actor stream |
| timestamp | valid Gregorian UTC RFC3339 seconds, YYYY-MM-DDTHH:MM:SSZ, year 0001..9999; descriptive only |
| operation | 1..128 printable non-whitespace ASCII bytes (0x21..0x7e) |
| request | immutable request digest below |
| record | empty for type.publish/open; logical opening ID for every successor |
| parents | strictly increasing full artifact IDs, no duplicates; at most 1024 |
| payload | the complete type or record payload below |

Request identity is (protocol, actual actor, operation). Its scope spans ALL custom
types and kinds, including type.publish; it is separate from native hn/0 keys.
Request digest = D(UTF8("hn-artifact-request-v1") || 00 || C(object containing
exactly actor, kind, operation, parents, payload, protocol, record)).
Thus time/sequence/previous can differ between separately authorized CAS attempts,
but kind, target, exact type/schema, parents, conversion, content and collection
order cannot. Request IDs from the host are not operation keys.

## 2. Type declarations and pure schema subset

Type payload has exactly type_id, schema_version, shape and schema.
type_id is namespace/name; both segments match [a-z][a-z0-9.-]{0,63}.
No slashes/aliases beyond that one separator. Namespace "hn" and "hubnot" are
reserved. A custom identifier never registers or shadows a CLI command.
schema_version is integer 1..2147483647; shape is document, list or ticket.
schema is an object with language="hn.schema/1" and root. Root is a schema node,
must be object, and validates ONLY the record's fields object.

Each node requires type and permits only the keywords in its row:

| type | Other permitted keys | Semantics |
|---|---|---|
| object | properties, required, additionalProperties | properties maps field names to nodes; required is distinct field names in properties; additionalProperties must be false |
| array | items, maxItems | one node for items; maxItems integer 0..1024 |
| string | maxBytes, enum | maxBytes integer 0..1048576; exact UTF-8 byte length; enum optional distinct strings |
| integer | minimum, maximum, enum | safe integers; inclusive bounds; minimum <= maximum |
| boolean | enum | optional distinct booleans |
| null | none | only JSON null |

For object, properties/required/additionalProperties are required; array requires
items/maxItems; string requires maxBytes. Other listed keys are optional.
An enum, when present, has 1..1024 distinct values of the declared scalar type,
each satisfying that node's other bounds. Empty required is [].
Field names match [A-Za-z_][A-Za-z0-9_.-]{0,63}; at most 64 properties in each
schema/value object. Every fields value is completely validated; unknown fields
refuse because additionalProperties is false. No implicit defaults/coercion,
remote references, schema references, regex validators, unions, recursive schemas,
code, executable validators or unknown-keyword fallback exists.

Schema and value depth count the root modeled object/array as 1 and each nested
modeled object/array as one more; maximum 8. Schema keyword maps/language wrapper
do not add modeled depth; scalar leaves add none. This permits the same eight
container levels in schema and fields. Parsing also remains bounded by this
closed finite grammar and the input byte cap, never unrestricted recursion. Type payload C bytes <=128 KiB,
including schema C bytes <=64 KiB. All arrays, including enums and required,
contain at most 1024 entries. These apply before unbounded expansion/recursion.
Unknown language/version is unsupported semantic validation, not accepted validity.

An initial type.publish has parents=[]; an evolution may name exactly one earlier
type.publish ID of the same type_id/shape with a strictly smaller schema_version.
All parent declarations must be present and verified. Different actors may publish
different declarations for the same name/version; names do not establish namespace
ownership or select a winner. Every instance pins an exact declaration artifact ID.
Publishing a new type never mutates or reinterprets existing records.

## 3. Record payload, collections and typed links

Record payload has exactly shape, type_id, type_revision, title, content, fields,
members, relationships, conversion_sources. shape/type_id must equal the exact
type.publish declaration at type_revision. title is UTF-8 1..1024 bytes with a
non-whitespace scalar; content is exact UTF-8 text, possibly empty; no binary
attachment. fields is the complete object validated by that declaration.
Envelope size is the ultimate aggregate text/field limit.

members is [] except for list shape. Each member has exactly member_id, target,
target_mode, required. member_id matches [A-Za-z0-9][A-Za-z0-9._-]{0,127};
IDs are unique within one list revision. Array position is explicit order;
revision replacement retains a member's ID when retaining that member's identity.
At most 1024 members; neither sorting by ID nor timestamps determines order.

Each relationship has exactly relation, target, target_mode, required.
relation uses the type_id grammar, including reserved-namespace refusal.
At most 256 relationships, with no duplicate (relation,target,target_mode,required)
tuple. Relationship array order is preserved without implying execution order.
For both members and relationships: target is a full custom-record ID;
target_mode is "logical" (must name an opening) or "revision" (exact opening or
successor); required is boolean. Other protocols are descriptive fields until
separately contracted, not silently followed custom dependencies.

conversion_sources is an increasing distinct array of exact parent revision IDs.
record.open has record="", parents=[], conversion_sources=[].
record.revise has one parent, keeps the logical opening and exact type_revision,
and has conversion_sources=[].
record.convert has one parent, keeps type_id/shape/logical opening, explicitly
changes type_revision, and conversion_sources=[that parent]. Content is supplied
in full and validated against the new exact schema; no converter executes.
record.resolve names every locally current head. When heads use different schemas,
the caller explicitly chooses type_revision and validates the full new payload;
conversion_sources is exactly those parents whose type_revision differs from it.
For same-schema resolution it is []. A resolution cannot change type_id or shape.

## 4. Local writes, imported concurrency and operation recovery

Before local mutation, verify the complete selected accepted custom view, actor and
signing-prefix continuity, contribution combination, schema/closure and immutable
intent. Custom writes under selected contribution policy return
unsupported_capability before creating an intent, signing or writing public objects.
No default/native command reads custom history or executable registration.

Order for an authorized custom write:
1. Validate bounded typed input and actual actor; load verified selected history.
2. Find (protocol,actor,operation) BEFORE comparing current record heads. Multiple
   signed matches are invalid_history; changed request digest is operation_conflict.
   Exact replay returns the original artifact/result identities even if later heads
   changed. It must not append again. Recheck the stored request digest.
3. For a new intent, refuse incomplete/unsupported/invalid required history. Open
   requires no parents; revise/convert require exactly the sole current head;
   revise on conflict is resource_conflict; resolve requires every current head.
   Missing/extra/stale parents are stale_revision. A new third head invalidates a
   prepared two-head resolution. Do not silently partial-resolve or choose newest.
4. Persist a private immutable actor/key/request intent before signing/publication;
   journal creation itself requires the relevant grant and supported policy.
5. Capture actor-stream expected Git head, prepare signed sequence/previous and
   bounded complete public closure, and publish with expected-old CAS. On CAS
   failure return concurrent_update; do not reload/resign/retry automatically.
6. Persist public artifact/Git-result identity in the private intent and acknowledge.

A later explicit retry uses the SAME semantic intent/key/actor. If verified absence
is established, a fresh CAS attempt may use the then-current actor sequence, but
must rerun unchanged resource-parent checks. An incomplete prior attempt is not
permission to generate a changed intent or overwrite the original descriptor.

Imported validity is different: verify signature, actor stream, sequence/previous,
declared record lineage/schema/parents and closure as immutable facts. Do NOT require
imported record parents to equal today's selected head set. Heads are verified
opening/successors minus any named as a parent by a verified successor. Two actors
revising one opening remain two heads until explicit resolution; an imported
formerly-valid two-head resolution plus a third branch leaves two current heads.
No timestamp winner. Distinct accepted copies of the same artifact deduplicate.

One actor stream may only extend by prefix. A same-actor divergent sequence or Git
history is actor_stream_conflict, not an ordinary collaborative record conflict;
preserve for diagnosis and refuse signing/accepted promotion for that stream.
Never merge actor Git chains or move a local signing root behind accepted history.

Operation lookup returns found, verified_absent or uncertain with an exact selected
source cut. found proves the original signed identity and request digest; current
head_count/conflict observations may be fresher and are labeled by that cut.
verified_absent requires a complete verified selected actor stream/closure plus
checked absence of pending local publication evidence. Limited selection,
unavailable/pruned history, an unresolved local intent, visibility restrictions or
unconfirmed ref publication yields uncertain, never absence. Bad signatures or
contradictory operation history are verification failure, not empty results.

A crash before publication retains a pending intent; after publication but before
journal/ack, recover by exact actor/key/request in verified public history. A missing
private intent does not negate a found public operation. Public absence with a
pending private intent is uncertain until its complete stage evidence proves no
publication and all writer activity has ended. No global exactly-once or remote-API
reconciliation promise follows from this.

### Private immutable intent and publication witness

Custom intent path is COMMON_GIT_DIR/hn/artifacts/intents/ACTOR/KEYHASH.json,
where KEYHASH is lower-case SHA-256(UTF8(operation)), without a digest prefix.
The key is never used as a path. Directory permissions are 0700 and file
permissions 0600, safe physical paths/no symlink following. One per-key lock plus
the actor-stream CAS protects concurrent identical/changed requests; lock failure
is explicit concurrent_update before new effects.

Intent JSON has exactly protocol="hn-artifact-intent/1",repository_id,actor,
operation,request,semantic_request,stage,expected_ref,expected_oid,candidate,result.
semantic_request is the exact seven-field canonical object used in the request
digest. Its actor/key/request binding is immutable after first persistence.
expected_ref is the exact local actor ref; expected_oid is a full Git OID or null.
candidate is null or {id:ID,commit:GitOID}; result is null or MutationResult.
stage is prepared, publishing or published:
- prepared: immutable request and captured expected-old ref are durable before
  signing; candidate/result are null.
- publishing: exact candidate identity/commit is durable BEFORE update-ref;
  public outcome is unconfirmed, result=null.
- published: verified public actor/key/request proves candidate reachable;
  immutable result identities are durable before acknowledgement.

Use bounded atomic replacement plus durability checks; a failed journal write
before publication refuses, and a failed write after publication is uncertain.
Each private intent file is <=2097152 bytes; recovery enumeration uses the
100000-entry catalog bound, and direct actor/key lookup never scans an unbounded
directory. A missing file is not proof of public absence. Querying operation status
does not create/repair an intent. Explicit exact mutation retry may reconcile it
under its actual grant/actor after verifying publication and writer settlement.
No private descriptor or pending intent is public evidence or selected export.

## 5. Git storage and selected closure

Local ref: refs/hn/artifacts/ACTOR/STREAMHEX.
Accepted remote ref: refs/hn/remotes/REMOTE/artifacts/ACTOR/STREAMHEX.
ACTOR is the full fingerprint, STREAMHEX is stream without "sha256:".
REMOTE matches [A-Za-z0-9][A-Za-z0-9._-]{0,63}, excludes "."/"..", any ".."
substring and ".lock" suffix. No arbitrary ref or filesystem path is accepted.

Each stream Git commit has exactly two regular 100644 blob entries:
artifact.json = exact C(envelope), and signature = raw-Base64 signature, no newline.
First commit has zero Git parents; every later commit has exactly the previous
stream commit as its sole parent, whose payload ID equals envelope.previous.
Commit message is "hn artifact " plus full ID plus LF; author/committer metadata
are transport metadata, never signed authority. A reader verifies object type,
tree entries/modes, signature, IDs, stream ownership and complete chain.
Repositories using SHA-1 or SHA-256 Git objects retain identical artifact IDs.

Required closure edges are stream predecessors, record opening and parents,
exact type declaration/type parents, conversion_sources, and required link/member
targets. Exact target adds that revision and its required closure. Logical target
adds its opening and ALL selected current heads; a conflicted target yields a
target_conflict diagnostic, never implicit selection of one head.
Optional domain links are preserved IDs, not required traversal; missing optional
targets produce missing_optional diagnostics and do not block source promotion.
No relation grants access or starts a process.

Use a work queue and visited full-ID set; domain logical cycles are valid and
terminate by visited-set traversal. An ancestry/stream cycle or wrong-lineage parent
is invalid_history. Required missing exact schema/target is incomplete_dependency.
Unknown schema semantics is unsupported_schema. Promotion of the complete selected
set requires verified known protocol, signatures, generic/schema validity and all
required closure. Read diagnostics distinguish integrity, completeness, schema
validity and domain target conflicts; application semantic validity is never inferred.

Fetch/copy to quarantine before expected-old atomic accepted-ref promotion. A failure
promotes none of that selected ref transaction. Temporary/quarantine refs and raw
diagnostic objects are not accepted refs or an ordinary change feed.
An unsupported/unknown/malformed object may be retained byte-for-byte only by
explicit opaque preservation, bounded like exchange, with integrity/verification
diagnostics and no semantic-validity claim. Each explicitly preserved raw public object has opaque_id =
D(UTF8(Git object type) || 00 || raw Git object body). This separate diagnostic
identity never serves as a signed artifact/record ID. The private bounded public
quarantine index is the only lookup source; no arbitrary ODB/private file access.
A diagnostic opaque export identifies every raw object, opaque_id and its status;
it never becomes an ordinary accepted import.
Known malformed signatures/history still refuse normal promotion. Opaque
preservation still requires a valid bounded exchange wrapper/object table and
correct Git object hashes; malformed wire JSON is an error, not an excuse to
invent an accepted or addressable object. Unknown custom
bytes never relax native hn/0 or hn-memory/0 validation or block unrelated native use.

Selected file exchange is a UTF-8 JSON object with exactly
protocol="hn-artifact-exchange/1", mode="accepted"|"opaque", refs and objects.
refs is an array of {ref,oid}; ref follows local/accepted grammar and oid is the
repository's full lower-case Git object ID. objects is an array of
{oid,type,data}; type is blob/tree/commit; data is canonical unpadded Base64 raw Git
object body. Recompute every object's Git hash using its type/length header.
Reject duplicate OIDs/ref names, absent closure, object-format mismatch and trailing
JSON bytes. No tag, private file, signing root, registration, grant or journal enters
accepted export. An opaque bundle is never automatically promoted by accepted import.
Git transport uses the same selected ref grammar/closure/promotion semantics;
network Git download is not claimed to have a pre-download byte quota.

## 6. Published bounds and refusals

All KiB/MiB values are powers of 1024; equality is accepted if otherwise valid.
Limits apply while consuming input, before unbounded allocation/signing/publication.

| Boundary | At limit | First over-limit | Result |
|---|---:|---:|---|
| canonical envelope | 1048576 bytes | 1048577 | resource_limit |
| private custom intent file | 2097152 bytes | 2097153 | resource_limit before publication / uncertain_operation after publication |
| type payload / included schema | 131072 / 65536 bytes | 131073 / 65537 | resource_limit |
| schema/value container depth | 8 | 9 | resource_limit |
| properties per object | 64 | 65 | resource_limit |
| array/list/parents/enum length | 1024 | 1025 | resource_limit |
| relationships | 256 | 257 | resource_limit |
| title / operation / member ID bytes | 1024 / 128 / 128 | 1025 / 129 / 129 | invalid_input |
| namespace/name / field-name bytes | 64 | 65 | invalid_input |
| schema_version / sequence | 2147483647 / 9007199254740991 | one greater | invalid_input |
| exchange wire / decoded aggregate | 16777216 / 33554432 bytes | one greater | resource_limit |
| exchange objects / signed records / ancestry depth | 4096 / 2048 / 256 | one greater | resource_limit |
| each validation or copy stage | 30 seconds | expired context | cancelled or uncertain_operation after dispatch |
| read/catalog selected refs or records | 100000 | 100001 | resource_limit |
| read/catalog Git object / aggregate | 8388608 / 268435456 bytes | one greater | resource_limit |
| regex UTF-8 pattern / expanded-compiled instructions | 4096 / 4096 | 4097 / 4097 | resource_limit |
| regex estimated selection instruction-bytes | 67108864 | 67108865 | resource_limit |

Regex follows bounded Go regexp syntax; estimate expanded and compiled instruction
counts before matching. Compute total instruction-count times selected searchable
UTF-8 byte count over the ENTIRE filtered selection before any output, with checked
integer arithmetic. Malformed regex is invalid_input; never truncate results to hide
a work-limit failure. No matching record activates policy or a module.
Synchronous/kernel I/O may outlast cancellation; do not claim an already-returned
definitive refusal while a detached writer can still publish.

## 7. Generic CLI and public JSON surface

Static command registration performs no repository/registry I/O:
- hn artifact type publish --input FILE --actor ACTOR --operation KEY --json
- hn artifact type show ID --json
- hn artifact open|revise|resolve|convert --input FILE --actor ACTOR --operation KEY --json
- hn artifact show|heads|history ID --json
- hn artifact list [--type TYPE_ID] [--shape SHAPE] --limit N --cursor TOKEN --json
- hn artifact search [--type TYPE_ID] --literal TEXT|--regex PATTERN --limit N --cursor TOKEN --json
- hn artifact operation --actor ACTOR --operation KEY --type TYPE_ID --json
- hn artifact export --id ID [--id ID] --output FILE --json
- hn artifact export --opaque --opaque-id OPAQUE_ID [--opaque-id OPAQUE_ID] --output FILE --json
- hn artifact import --input FILE [--preserve-opaque] --json
- hn artifact fetch|push --remote NAME --id ID [--id ID] --json
- hn artifact inspect --id ID --json
- hn artifact inspect --opaque-id OPAQUE_ID --opaque --json

FILE is explicit local input, "-" means stdin. Type publish input is exactly
{parents,payload}; record mutation input is exactly {record,parents,payload}, with
record=null for open. CLI command selects kind; --actor/--operation supply the
remaining host params, and duplicate/unknown input keys refuse. Exchange input
has the exact exchange object specified above, not mutation params.
Import/fetch data is {mode:"accepted"|"opaque",accepted_refs:[{ref,oid}],
imported_ids:[ID],opaque_ids:[ID],diagnostics:[Diagnostic],source_cut:Cut};
opaque mode has accepted_refs=[],imported_ids=[] and no accepted promotion.
Export/push data is {mode:"accepted"|"opaque",output_digest:ID,records:integer,
objects:integer,refs:[{ref,oid}],opaque_ids:[ID],bytes:integer}; output_digest
hashes exact wire bytes. For push it hashes C({protocol:"hn-artifact-transfer/1",\nremote:NAME,refs:[{ref,oid}],ids:[ID],mode:"accepted"}), refs sorted by ref and\nIDs increasing; transport credentials/URL/private state are excluded.
Explicit opaque export selects opaque IDs rather than treating them as signed
record IDs; --opaque requires --opaque-id ID selections. Native/custom transport
does not silently choose opaque mode. Writes never
derive revised content from a latest head. Reads use full IDs; no ambiguous prefix.
JSON success is {"ok":true,"data":VALUE,"error":null}; failure has ok=false,data=null,
and the host error object defined in host-v1.md; exit 0 or 1 respectively. Human
rendering is descriptive; machine contracts are the typed data in host-v1.md.
No registry or specialized renderer is needed for these commands.

### 7.1 Inert envelope-validation slice

The WP03 envelope utility is a read-only input-verification slice, distinct from
store-backed `hn artifact inspect --id`. Its command grammar is:

- `hn artifact envelope validate --input FILE|- --signature RAW_BASE64 [--previous-input FILE --previous-signature RAW_BASE64] [--json]`
- `hn artifact envelope inspect --input FILE|- --signature RAW_BASE64 [--previous-input FILE --previous-signature RAW_BASE64] [--json]`

RAW_BASE64 is the canonical unpadded native Base64 encoding of the 64-byte public
signature; these commands do not receive a signing key. Input is the exact bounded
canonical payload bytes. Both predecessor flags must occur together or neither;
two simultaneous stdin selections refuse invalid_input. Each payload independently
has the envelope's 1MiB bound. Existing frozen parse/signature/error rules apply.

The ordinary section7 success/error JSON wrapper and exit codes apply. Validate
success data is exactly {id:ID,actor:Actor,kind:Kind,integrity:"verified",
semantic_validation:"not_checked",continuity:Continuity}. Inspect adds the exact
decoded `envelope` object to those fields. Continuity is "genesis" for sequence1
with previous="", "predecessor_verified" after an explicitly supplied verified
adjacent same-actor/same-stream predecessor matches sequence and previous ID, and
"not_checked" when a non-genesis envelope has no supplied predecessor. Genesis
and predecessor_verified do not assert uniqueness or completeness of the whole
actor history. Full contextual schema, record lineage, conflict and closure
validation remains not_checked in this slice; integrity alone cannot admit data.

The utility works without a module or registry and performs no signing, public
publication/ref update or module startup. The existing native outer command-lock
semantics, including selected-contribution private lock bookkeeping, remain intact;
this utility does not introduce a lock bypass. Human rendering is descriptive;
--json is the stable machine surface.
