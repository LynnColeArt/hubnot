# Extension and artifact data model

This is a proposed versioned model. All public objects are inert signed facts;
private registration/session/grant/journal state is never part of public export.

| Entity | Identity / revision | Invariants |
|---|---|---|
| ExtensionManifest | namespaced extension ID + version + manifest digest | declares capabilities/protocol; no authority; bounded JSON; no launch on read |
| LocalRegistration | locally selected executable/argv + manifest digest | user-local installation; canonical path; replacement invalidates enablement |
| LocalEnablement | registration identity + canonical repository + grant set | enablement distinct from installation, discovery, signer or project-policy adoption |
| TypeRevision | signed exact digest, namespace, type ID/version, shape, schema | shape=document/list/ticket; closed bounded declarative schema; reserved built-in namespaces refuse |
| ArtifactOpening | full signed opening ID, actor, type revision, content | actor/keys agree; stable logical identity; no parents |
| ArtifactRevision | full signed ID + logical opening + schema + expected parents | append-only; exact old heads; visible conflicts; no timestamp resolution |
| ArtifactOperation | actual actor + caller operation key + immutable request digest | changed replay refuses; recovery distinguishes original result, verified absence and uncertainty |
| CollectionMember | member ID/order + logical/exact target | order explicit; duplicate/malformed member IDs refuse; limits apply |
| TypedRelationship | namespaced relation + logical/exact target | target mode explicit; traversal bounded; domain graph meaning remains consumer-owned |
| CapturedChanges | repo/view fingerprint + accepted cuts + bounded entry set | signed validity != visibility; no raw quarantine; deterministic presentation |
| ObservationCursor | session + source/view fingerprint + offset/limit + expiry | no global event order; changed policy/moderation/selection invalidates visibly |
| ConsumerSemanticResult | consumer/schema/version + exact input revision IDs | separate from generic validity, project policy, execution and prompt authority |
| ExternalConnectorJournal | private immutable intent, native actor/key, external identity/result | connector reconciles remote writes; no globally exactly-once promise |

## Public storage and compatibility

Plan one additive hn-artifact/1 envelope with explicit sequence/previous, actor/public key,
operation, logical record ID where applicable, schema revision, parent IDs and payload.
Use additive refs/hn/artifacts/ actor streams and corresponding selected remote namespaces;
final path grammar and signed bytes are pinned before implementation in the protocol contract.
Do not emit custom native event kinds into hn/0 or memory records into hn-memory/0.
A pre-extension reader is not promised generic protocol support; its supported native
histories/commands remain valid. An extension-aware client needs no plugin to inspect
new public records and type declarations.

Logical records can have revisions by multiple actors; an actor stream itself is
append-only/CAS and never merged. Active heads are computed from the verified selected
set. A resolution is a new revision naming every current head. Missing parents/schema
are explicit dependencies. No silent partial closure and no automatic reinterpretation.

Exact text and structured values are both data. Canonical signed envelope encoding
rejects duplicate keys, malformed Unicode, noncanonical IDs and unknown protocol fields.
Schema versions are immutable. A conversion creates new signed data and names its source.
Custom domain statuses (including a ticket's status) are schema fields, not authority.

## Lifecycle boundaries

Registration -> disabled declaration -> explicit repository enablement -> explicit host
session -> stop/disable. Registration changes require re-enablement. Host request states:
received -> validated/granted -> dispatched -> completed or refused; a lost final result
after dispatch may be uncertain and requires operation recovery, not a synthetic refusal.

Artifact lifecycle: opening -> revision(s) -> explicit resolution/new heads. Collection
membership and domain activation are versioned payloads. Merely storing/searching one
does not enforce it.

The generic schema evaluator is finite pure validation. Deeper application validation,
charter composition, source-tier precedence and interpreting required/suggested links
belong to the external consumer. No plugin-defined code is required to verify public bytes.

## Retention and private state

Public histories survive disable/uninstall. Existing private identity and contribution
state remain governed by their own contracts. Extension registration, grants, child stderr,
captured page state and recovery journals use bounded private storage; public diagnostics
select safe fields rather than dumping these files. Lock duration is per native operation,
never the entire external process session.
