# Collaboration and optional transfer boundary

This document describes native collaboration and optional transfer additions, alongside the existing
[replication boundary](replication-v0.md), [CI execution boundary](ci-v0.md),
[governance rules](governance-v0.md) and [memory safety model](memory-safety.md).
Hubnot remains an experimental repository-native protocol, not a hardened
multi-tenant service.

## Authority and hostile data

Accepted signed Git actor histories are the durable source of issue facts.
Issue queries rebuild a verified in-memory view. There is no authoritative
issue cache, database, service, or hidden synchronization source.

Fetched payloads, signatures, object framing, actor sequences, parent/root
references, relation targets, run attachments and ref advertisements are
untrusted. The issue reader reuses signature, actor-chain and relationship
validation and rejects pending admission. Physically present objects do not
become accepted merely because they can be read from the object database.
Missing suppliers remain quarantined until explicit selected recovery succeeds.
Queries never promote or repair replication state.

JSON requests reject unknown or duplicate fields, explicit null values,
malformed raw UTF-8 and unpaired surrogate escapes. Valid text, including
intentional U+FFFD, remains data. Machine stdout is one JSON value; consumers
must keep diagnostic stderr separate and avoid interpreting issue text as
commands or instructions.

## Bounds and operational failures

New requests, state fields, revision parents and output pages have explicit
[documented bounds](issues-v1.md#limits). The reader bounds ref enumeration,
history traversal, individual object lengths and aggregate object bytes before
allocating those object bodies. A persistent Git batch process replaces
per-event object-fetch subprocesses. All selected signed records remain
verified; mixed governance histories can still require separate policy or
pipeline reads. No constant total process-count claim applies to arbitrary
non-issue governance content.

A resource-limit error does not mean a legacy signature is invalid. A naturally
failed Git batch child retains its bounded diagnostic and exit cause as a
repository error. Deliberate reader cancellation retains the original parser
or resource-limit diagnosis. These are local validation and ingestion bounds,
not universal CPU/disk/network quotas. Replication pack transfer has its own
explicit limitations documented in the replication guide.

## Concurrent actors and retries

A revision refers to its stable opening ID and exact prior issue revisions.
The actor's Previous field separately links its own append-only history.
Concurrent issue heads remain visible with authorship and full states; neither
timestamps nor arrival order grants one head authority.

The writer checks an observed snapshot and uses actor-ref compare-and-swap.
This protects one actor append, not a cross-actor global transaction. Other
actors can create valid concurrent work after that snapshot. Pagination rejects
changed snapshots; staged resolution requires the exact observed snapshot and
preserves every unconsumed head.

Operation identity is scoped to the signing actor. Signed intent and a digest
of canonical request semantics distinguish retries from new work. Replays
return the original event before checking current heads; divergent key reuse
fails. These guarantees do not merge independent actors' operations or create
an execution lease.

Assignment, lifecycle, criteria, graph edges and namespaced metadata are
informational. They do not grant Go Kitty execution, review acceptance,
integration authority, or Hubnot maintainer/reviewer/runner policy roles.
Distributed cycles are explicit graph diagnostics, not a reason to discard
otherwise valid signed facts or silently declare work ready.

## Compatibility and recovery

Existing signed records are retained byte-for-byte. Rich changes append new
facts; they do not rewrite history. Older executables may reject `issue.revise`
or new rich payload shapes. Rolling back a binary cannot remove already
published signed facts, and this feature provides no implicit history rewrite
or Beads migration. Preserve a compatible reader when retaining rich histories.

## Automation identity and historical observations

Public identity JSON explicitly selects actor, name and public key; it never
serializes the private identity record. Issue mutations may pin an expected
actor fingerprint. The comparison binds the captured identity actually used
for append, before operation replay; a mismatch produces no issue append. The
active identity file changing after capture cannot substitute another signer.
This is signer attribution, not a cross-actor lock or new execution authority.

Actor/key operation lookup runs the existing bounded verified reader and
returns one original signed event's request semantics, never current issue
state. Duplicate signed keys fail explicitly. Invalid/over-budget history
cannot be interpreted as an absent operation. Detailed list pages use the
same verified catalog and mode-bound cursors; conflicting state is null and
full state is never truncated. Detailed rows also expose the immutable opening
event metadata (empty for legacy), so clients can compare original and current
claims without separate history calls. No namespace interpretation is applied.
Neither read creates an index, bypasses trust
admission, or grants review, execution, assignment or merge authority.

Machine identity discovery reads and validates existing active or legacy identity
state without writing private keyring files or Git refs. It never performs the
legacy migration retained by default human identity commands. Invalid private
state yields a typed public error without exposing private fields or paths.


## Native typed resources

Projects/items, discussions/replies and workflows use strict type-specific
states in registered signed event kinds. Child items/replies are independently
revisioned aggregates. Project items pin exact schema revisions and accepted
answers pin exact reply revisions; a later schema or reply edit does not
reinterpret those historical facts. Cross-resource references participate in
accepted-history, quarantine and shallow-closure validation. Unknown new kinds
can be rejected by older binaries; upgrading never rewrites old signed bytes.

Native workflows separate inert foreign source from validated native pipeline
state. Materialization writes explicit pipeline bytes, while a run request must
bind the exact signed workflow revision and committed proposal pipeline bytes.
Import, materialization and request never execute source text. Existing policy,
actor and explicit runner backend checks remain the execution authority.

## Optional provider boundary

The local `hn gh issue` subset calls native issue functions and performs no
forge access. Provider-neutral transfer models contain no HTTP/SDK dependency;
network clients and GitHub registrations are excluded with `nogithub`. All
native facts remain usable without a provider, credential or service.

Only explicit transfer commands contact an explicitly selected provider.
Transport requires HTTPS except explicit loopback-IP HTTP and refuses redirects
or pagination outside the requested endpoint. Requests/responses, nested
pagination, aggregate bytes, whole-command time, native payloads and complete
future checkpoints have finite bounds. GraphQL errors/partial data, malformed
or ambiguous JSON, invalid Unicode and incomplete traversal fail before their
content is accepted. Provider errors do not echo bodies or tokens. External
source locators in signed metadata are descriptive; inspecting them does not
fetch those addresses.

Original authors, URLs, category/lock states, assignees, project visibility,
workflow enabled state and other provider observations remain claims signed by
the importing local actor. They never grant native policy roles, moderation,
runner rights or permission to execute instructions. GitHub creation uses the
authenticated provider account and cannot impersonate original authors/times.
Unsupported semantics require explicit mapping/acknowledgment or refusal;
foreign workflow preservation is not a claim of execution equivalence.

Imports preflight a complete selected issue or typed graph before publication,
but a multi-event import can leave valid partial work when interrupted. Its
recovery reuses exact frozen source content, signer, native expectations and
verified signed progress. Baselines and mappings require producer-shaped signed
import witnesses; editable latest metadata alone cannot reset overwrite authority.

Issue, discussion and project exports authenticate their complete canonical
private checkpoint with the currently selected native identity. This closes
manufactured comment/parent/field bindings or progress that could otherwise
suppress or redirect publication. Keys are not stored in the journal; a signer
switch, unsigned journal or altered/noncanonical envelope requires explicit
recovery. Preview remains private-key-free. The journal signature authenticates
local intent, not remote success or provider permissions. Workflow recovery
separately binds exact native/source bytes, explicit destination and guarded
blob intent. All exporters preflight the whole future journal, including pending
results and accumulated bindings, before the first mutation.

Private journals/lock files remain below the resolved Git directory with
0700 directories and0600 files, outside cloned signed facts. One local lock is
not a distributed lease. Every remote mutation persists prepared/in_flight
intent before one write; an uncertain outcome needs one exact positive
reconciliation. A negative complete scan is not proof of nonexecution and
cannot authorize another POST. Structured publication markers are forgeable
associations and need native/provenance/content checks; ordinary marker-like
prose is inert data. Remote-only edits are preserved or refused for manual
recovery, never silently made a new overwrite baseline. External APIs do not
supply atomic compare-and-swap for every field, so a single publisher and
explicit reconciliation remain necessary. There is no global exactly-once,
background-sync, automerge or new protected-check authority claim.
