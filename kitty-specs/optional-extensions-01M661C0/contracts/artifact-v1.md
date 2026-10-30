# Proposed portable artifact contract — hn-artifact/1

This is an additive planning contract. Existing hn/0 and hn-memory/0 bytes remain
unchanged. Custom declarations are data and do not start an extension.

## Generic shapes

A type revision declares a namespaced type ID, schema version, one of document/list/
ticket, exact bounded portable schema, public signer and digest identity. An instance
pins that exact signed type revision. The schema supports bounded properties,
required fields, scalar types/enums and bounded arrays; no remote references,
recursive evaluation, arbitrary code or unknown-keyword fallback.

Illustrative instance payload (digest tokens below stand for actual full IDs):
~~~json
{"shape":"document","type_id":"example.charter/directive","type_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","title":"Explicit activation","content":"Stored guidance is inert until explicitly selected.","fields":{"enforcement":"required"},"members":[],"relationships":[]}
~~~
The digest placeholder is an example of the required full-ID shape, not a resolvable
fixture. Public conformance fixtures must generate actual signed declarations and
substitute real IDs before exercising mutation.

List instances contain explicitly ordered members with stable member IDs. Ticket
instances carry schema-defined fields/status; they do not become built-in native
issues or grant code authority. Links name a relation type, target ID and target_mode
equal to logical or revision. Preserve exact content UTF-8 bytes and report its digest.

## Revision and operation invariants

An opening's signed ID is the logical record ID. A successor names that opening,
its exact schema and full expected revision parents. Each actor appends to its own
immutable signed custom stream. Ordinary revise requires one current head; resolve
requires every current head and a separate grant. Concurrent heads remain visible.
A replay carries the original actual actor/key/intent; changed type, content, target
or parents refuses. A definitive public result may be recovered; uncertain local
effects cannot be called verified absence.

Signatures prove attribution/integrity, not semantic truth, accepted policy, roles,
prompt authority or upstream namespace ownership. Public declarations identify their
exact signer/digest. Reserved Hubnot namespaces and built-in command shadowing refuse.

## Limits and portability

Envelope1MiB, type declaration128KiB/schema64KiB, nesting8, object properties64,
list/array entries1024, relationship entries256; reads/export use existing public
object/ref traversal budgets or stricter explicit artifact limits. All writes and
selected dependency closures are verified before publication, with expected-old refs.
Explicit file-exchange limits:16MiB wire,32MiB decoded,4096objects,2048signed records,
ancestry depth256 and30s per context-aware validation/copy stage. Read/catalog closure
limits:100000selected refs/records,8MiB per Git object and256MiB aggregate. These
limits are enforced while consuming material, not after an unbounded spool; preserve
the documented synchronous/kernel-I/O cancellation caveat and claim no earlier network
Git-download quota. Schema evaluation and link traversal share nesting/count budgets.
Regex search:4096UTF-8 pattern bytes,4096estimated-expanded/compiled instructions,
64Mi instruction-bytes over the complete selection before any result output.
Namespace/name grammar: each segment1..64lower-case ASCII letters/digits/dots/hyphens,
starting with a letter. Custom type declarations are explicit type.publish protocol
objects; document/list/ticket instances cannot redefine protocol envelopes or built-ins.

Incomplete closure, missing schema, malformed/unknown protocol and oversized data
produce structured diagnostics. Exact bytes can be inspected/preserved by the generic
extension-aware reader without specialized code; a declared unsupported semantic
schema version is not reported semantically validated.

Public selected export contains signed records, exact types and required public
dependencies. It excludes registration, credentials, keys, private descriptors and
application journals. Reconstruct on a second independently initialized clone using
a local bare Git remote; no source-machine private state may be copied.

## Contribution-profile boundary

Native extension adapters use the existing governed view. The first custom protocol
does not secretly join the current native contribution artifact profile. With selected
contribution policy, custom mutations refuse unsupported_capability before effects.
Without selected contribution policy, generic custom records use the documented
native attribution/conflict/transport guarantees. A future custom admission profile
requires its own explicit bounded contract and independent qualification.
