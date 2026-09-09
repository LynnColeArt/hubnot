# Hubnot protocol v0 experiment

This document records the format implemented by the operational alpha. It is a
testable sketch, not a compatibility promise. Existing version-0 event payloads
and event IDs were not changed when identity continuity was added. Agent memory
is the separate additive `hn-memory/0` protocol documented in
[memory-v0.md](memory-v0.md); memory records are not inserted into `hn/0`
actor chains or event kinds.

`hn/0` is a breaking namespace reset, not a compatible spelling of `nh/0`.
The runtime accepts and writes only `hn` wire versions and `refs/hn/*`; legacy
wire bytes and `refs/nh/*` may remain immutable evidence but are ignored.

## Actor identity

An actor is exactly one Ed25519 key pair. Its identifier is the lowercase
hexadecimal SHA-256 digest of the 32-byte public key:

```text
actor = hex(sha256(ed25519_public_key))
```

Each actor has one append-only event chain. A device group or key lineage is a
projection over distinct actors; it is never a shared actor secret. The
raw-base64 public key remains in every event so each event can be verified from
its own signed bytes.

## Event payload and identity

An event is UTF-8 JSON emitted without insignificant whitespace. The complete
version-0 field order is:

```json
{
  "protocol": "hn/0",
  "kind": "issue.open",
  "actor": "<full-public-key-fingerprint>",
  "actorName": "Alice",
  "publicKey": "<raw-base64-Ed25519-public-key>",
  "sequence": 1,
  "timestamp": "2026-08-29T12:00:00Z",
  "previous": "<full-previous-event-id>",
  "subject": "<full-subject-event-id>",
  "relationship": "device",
  "targetActor": "<full-target-actor-fingerprint>",
  "targetKey": "<raw-base64-target-public-key>",
  "title": "An issue title",
  "body": "Optional body",
  "base": "<full-Git-commit-id>",
  "head": "<full-Git-commit-id>",
  "verdict": "approve",
  "pipeline": "test",
  "definition": "<full-pipeline-digest>",
  "commit": "<full-Git-commit-id>",
  "outcome": "passed",
  "exitCode": 0,
  "durationMs": 1,
  "log": "<full-log-digest>",
  "backend": "sandbox",
  "platform": "linux/amd64",
  "runner": "hn/<version>",
  "policy": "<full-policy-digest>",
  "evidence": ["<full-evidence-event-id>"],
  "issue": {"title": "Current work", "body": "Description", "status": "open", "criteria": [], "labels": [], "assignees": [], "relations": [], "metadata": {}},
  "parents": ["<full-prior-issue-revision-id>"],
  "operation": "actor-scoped-operation-key",
  "request": "<canonical-request-sha256-id>",
  "intent": "revise"
}
```

This is a field inventory, not a valid single event: fields unused by an event
kind are omitted. `previous` identifies the preceding event in the same actor
chain. `subject` identifies the exact event on which a dependent fact relies.
Optional string, integer-zero, and empty-list fields are omitted by the current
encoder.

The event identifier is independent of Git's configured object-hash format:

```text
event_id = "sha256:" + hex(sha256(exact_event_payload_bytes))
```

The signature is Ed25519 over those exact payload bytes. Verification does not
reserialize JSON. Every accepted event must have protocol `hn/0`, a positive
sequence, a valid RFC 3339 timestamp, a public key whose fingerprint equals
`actor`, and a valid signature. Full event IDs have the form
`sha256:<64-lowercase-hex>`.

The implemented kinds are:

```text
issue.open
issue.comment
issue.revise
proposal.open
proposal.revise
review.submit
run.request
run.result
proposal.decision
proposal.merged
identity.authorize
identity.accept
```

## Issue roots, revisions and reliable operations

The optional `issue`, `parents`, `operation`, `request`, and `intent` fields
are appended to the Event field inventory. When absent they are omitted, so
existing signed payload bytes and IDs remain unchanged. The `hn.issue/1`
versioned CLI envelope is separate from the signed `hn/0` event protocol.

`issue.open` establishes stable identity. A legacy title/body opening maps to
an open issue; a rich opening additionally signs full IssueState with matching
title/body. `issue.revise` signs `subject` equal to the opening ID, a complete
`issue` state, and sorted unique `parents` identifying 1–200 same-issue opening
or revision events. It carries no separate title/body fields. `issue.comment`
signs its opening subject and body, without state or revision parents.

`previous` is the previous event in this actor's history; `parents` are the
issue-content ancestors and may belong to other actors. The verified projection
computes all maximal issue revisions and preserves conflicts without a selected
state. Explicit reconciliation appends a successor referencing the consumed
heads; staged reconciliation preserves all unconsumed heads.

IssueState contains title, body, open/closed status, criteria with local IDs,
labels, assignees, typed relations and namespaced string metadata. Criteria
order is preserved; label/assignee/relation sets are canonically sorted and
reject duplicates. Assignment, criteria and status confer no policy or consumer
execution authority. Root/parent/relation dependencies participate in exact
replication closure and stay quarantined if suppliers are absent.

`intent` distinguishes open/revise/resolve/close/reopen/comment. When an
operation key is present, `operation` and `request` are both signed. The request
digest is SHA-256 over the canonical intent/kind/subject/parents/state/title/body
encoding; actor sequence, timestamp and operation key are excluded from that
encoding, while the operation key is separately scoped to its signer. The
reader recomputes the digest from signed semantics. Reusing a key for different
semantics fails; identical retries return the original event before checking
current issue heads. No cross-actor atomicity or shared operation authority is
implied.

See [issues-v1](issues-v1.md) for complete limits, machine schema, directed graph,
retry and staged-resolution examples. Older executables may reject the new
kind; reverting an executable does not erase newly signed facts.

## Identity continuity events

`identity.authorize` adds exactly these event-specific fields:

```json
{
  "relationship": "device",
  "targetActor": "<full-target-actor-fingerprint>",
  "targetKey": "<raw-base64-target-public-key>"
}
```

`relationship` is exactly `device` or `successor`. `targetActor` must be a full
64-hex fingerprint, must equal the fingerprint derived from `targetKey`, and
must differ from the signer. `body` may carry an explanation. Its ordinary
`previous` field is present unless it is the signer's first event.

`identity.accept` uses `subject` for the full authorization event ID. Its
signer and public key must exactly equal that authorization's target actor and
key. Its ordinary `previous` field is present unless it is the target actor's
first event.

The deterministic projection applies the following rules over verified facts:

- authorization without a matching acceptance is `pending`;
- one or more matching acceptances produce one accepted edge; duplicate exact
  claims remain inspectable but add no authority;
- an accepted `device` edge keeps both actors active;
- exactly one accepted acyclic `successor` edge retires the predecessor in the
  identity projection;
- multiple accepted outgoing successors are `competing-successors`;
- accepted successor cycles are `successor-cycle` conflicts;
- an acceptance whose authorization is absent is reported as a missing fact;
- a signer/key mismatch is a conflict, never a relationship;
- timestamps and cross-actor delivery order never pick a winner.

Edges, actors, conflicts, acceptance IDs, and event IDs are sorted for stable
inspection. Identity projection is descriptive only. It does not edit policy
and does not grant a maintainer, reviewer, runner, or decision role.

## Proposal candidates and revisions

`proposal.open` signs a title and distinct `base`/`head` Git commit IDs.
`proposal.revise` is a new immutable candidate and signs:

```text
subject   full event ID of the exact predecessor candidate
base      full resolved Git commit used as the revised diff base
head      full resolved Git commit containing the revised code
body      optional explanation
title     omitted; inherited from the lineage root
```

The predecessor must be an available `proposal.open` or `proposal.revise`
event, and the same actor must sign both. Base and head are distinct commits.
Missing predecessors, actor mismatches, and cycles fail closed.

One predecessor may have multiple successors. IDs are sorted for deterministic
projection, but no timestamp, delivery order, or actor-chain position creates a
global latest winner. Evidence always binds one exact candidate; evidence for
a predecessor or sibling cannot qualify a revision.

## Git representation and refs

Every event is a Git commit whose tree contains:

```text
event.json    exact signed payload bytes
signature     raw-base64 Ed25519 signature
```

A `run.result` tree also contains `log.txt`; the exact log bytes hash to the
signed `log` field. No other tree entry is accepted. The commit parent is the
preceding event commit for that actor. The signed `previous` event ID provides
an independent protocol chain.

Public roots are:

```text
refs/hn/actors/<full-actor-fingerprint>
refs/hn/proposals/<candidate-event-sha256-without-prefix>
```

The candidate ref points to the exact signed `head` commit, making the proposed
code reachable for transport and garbage collection. A mismatch fails closed.

Accepted remote-tracking roots are local:

```text
refs/hn/remotes/<remote>/actors/<full-actor-fingerprint>
refs/hn/remotes/<remote>/proposals/<candidate-event-sha256-without-prefix>
```

Actor chains begin at sequence 1, increase by one, and match both Git parents
and signed `previous` IDs. Different events at the same actor sequence are a
fork and are rejected by the current projection.

## Runs and governance events

A `run.request` references a candidate through `subject` and signs the
repository-local pipeline name, SHA-256 ID of the exact pipeline JSON bytes,
and proposed Git head. A `run.result` references the request and repeats all
three bindings. It also signs `passed` or `failed`, exit code, duration, exact
log digest, backend, platform, and runner. Any mismatch is rejected.

These events are attestations. Signatures protect attribution and contents;
they do not prove honest execution. The exact base policy chooses which actors'
claims count.

A `proposal.decision` references one candidate, the SHA-256 ID of its exact
base policy bytes, and an `accept` or `reject` verdict. Accept decisions include
the exact review and run-result event IDs that satisfy policy. Rejections
require an explanation.

A `proposal.merged` event binds the candidate, proposed head, resulting Git
commit, base-policy digest, and acceptance-decision IDs. It is emitted after
Git creates the merge commit; branch publication remains a separate ordinary
Git action.

## Synchronization

`hn replication select` records full per-remote actor/candidate selectors and
positive budgets below `.git/hn/`. A saved selection is authoritative. With no
saved selection, version 0 uses bounded compatibility-all: it enumerates only
advertised `refs/hn/actors/*` and `refs/hn/proposals/*`, then applies the same
quarantine, budgets, validation, and promotion transaction.

Each selected ref is fetched with its own exact refspec into a generated bare
quarantine repository. It is measured, structurally and cryptographically
validated, and checked for exact relationships before objects are copied and
accepted refs are atomically updated. Invalid, over-budget, or
dependency-missing selections do not become accepted projection roots.
Standard Git may download a selected pack before measurement, so the budgets
are hard before promotion and retention, not portable pre-download network
quotas. [Replication v0](replication-v0.md) defines the transaction and shallow
recovery boundary in detail.

Publication uses ordinary explicit Git pushes of local actor and candidate
refs. `hn sync` never publishes the repository's primary branch; use a
separate explicit `git push` for that outcome.

## Known limits

- One actor remains single-writer; disconnected concurrent writers sharing a
  private key are unsupported.
- Planned rotation requires the predecessor key. Lost-key, compromise, social,
  and organizational recovery are not implemented.
- Continuity and selection never confer policy authority.
- There is no portable hard pre-download byte quota, moderation network,
  selective global deletion, redaction guarantee, or global erasure of
  already replicated immutable facts.
- JSON canonicalization and extension negotiation are not stable
  cross-language contracts yet.
- Hosted ref retention, large-scale behavior, and providers not listed in
  [host compatibility](host-compatibility.md) remain unproven.
