# Mission Specification: Pinned Actor and Operation Lookup

**Mission Branch**: `feat/pinned-actor-operation-lookup`  
**Created**: 2026-09-09  
**Status**: Specified

## Intent Summary

The user authorized autonomous prerequisite work for reliable Hubnot consumers and delegated discovery choices. A local automation client discovers its public signer, pins that actor when writing, and recovers the exact original operation after a lost response. The invariant is that an unexpected signer never publishes the requested issue mutation, and recovery reads verified immutable history without inventing authority or rewriting it. This software-dev mission is additive; defaults remain compatible. The confirmed parent handoff adds operation and timestamp to lookup output.

## User Scenarios & Testing

### User Story 1 - Pin the expected signer (Priority: P1)

A client discovers public identity and supplies that actor for its next issue mutation.

**Independent Test**: A real CLI invocation emits public identity only; an identity change followed by a pinned mutation fails and leaves all issue/actor refs unchanged.

**Acceptance Scenarios**:
1. Given an initialized identity, machine identity discovery exposes actor/name/public key and no private information.
2. Given a previously observed actor, a mutation with the same actor succeeds; a different current actor gives a stable actor_mismatch error without append.
3. Given an unpinned existing command, behavior remains compatible with the currently active identity.

### User Story 2 - Recover an original operation (Priority: P1)

A client queries one explicit actor and operation key after losing a mutation response.

**Independent Test**: After later revisions/comments, a public lookup returns the original immutable event's semantics and timestamp, not the latest issue state.

**Acceptance Scenarios**:
1. Unique actor/key returns complete original issue/event IDs, actor/key, kind, intent, timestamp, parents, requested state or body, and request digest.
2. Absent actor/key reports not_found; multiple admitted signed records report operation_conflict and never choose a winner.
3. The same key from another actor does not match; changed request reuse remains rejected by mutation replay.
4. Invalid history or resource limits remain explicit failures rather than absence.

### Edge Cases

Malformed/full actor IDs, empty/oversized/non-printable operation keys, missing/repeated/unknown flags, false machine flags, free text and delimiter handling, closed/conflicted issues, original comments with empty bodies, mutable active identity changes, and invalid or oversized history.

## Requirements

### Functional Requirements

| ID | Title | User Story | Priority | Status |
|----|-------|------------|----------|--------|
| FR-001 | Public identity | A client can request one machine-readable public actor/name/key result without private data. | High | Open |
| FR-002 | Pinned mutation | Every issue mutation accepts an optional full expected actor and refuses mismatches against the identity actually used for append, with zero writes. | High | Open |
| FR-003 | Original operation | A read-only explicit actor/key lookup returns complete original signed request semantics, operation and timestamp after subsequent edits. | High | Open |
| FR-004 | Honest errors | Missing, malformed, absent, duplicate, invalid-history and resource failures are stable typed machine responses; lookup never silently selects a duplicate. | High | Open |
| FR-006 | Complete catalog pages | A client can optionally page complete current issue states, with explicit null conflicts, without individual issue reads; continuation is bound to detail mode. | High | Open |
| FR-005 | Compatible public contract | Existing human/JSON commands retain defaults; help and protocol documentation explain new schemas, pinning and recovery limits. | High | Open |

### Non-Functional Requirements

| ID | Title | Requirement | Category | Priority | Status |
|----|-------|-------------|----------|----------|--------|
| NFR-001 | Bounded reads | Lookup uses current verified-reader caps: 100000 events/ref entries, 8 MiB object, 256 MiB aggregate; emits at most one original result without truncation. | Resource | High | Open |
| NFR-002 | Single response | Machine success/failure emits exactly one JSON document on stdout, with nonzero exit on failure. | Reliability | High | Open |
| NFR-003 | Regressions | Public subprocess regressions plus required race, vet, external build, formatting and diff gates pass. | Quality | High | Open |

### Constraints

| ID | Title | Constraint | Category | Priority | Status |
|----|-------|------------|----------|----------|--------|
| C-001 | Existing authority | No new storage authority, dependencies, services, private-key exposure or signed-event schema changes. | Technical | High | Open |
| C-002 | Honest concurrency | Actor pinning is signer identity, not a distributed issue lock or new execution authority. | Safety | High | Open |
| C-003 | Isolated delivery | Dedicated prerequisite branch, preserve existing PR and historical worktrees; no push or PR edit. | Delivery | High | Open |

### Key Entities

- PublicIdentity: actor, display name, public key; never private key or identity path.
- OperationObservation: exact accepted signed event and request semantics under explicit actor/key, plus observed repository snapshot.

## Success Criteria

- SC-001: Every mismatched pinned mutation in acceptance tests publishes zero refs.
- SC-002: Lookup recovers original state and time after later edits without consumer-local storage.
- SC-003: Duplicate/absent/invalid cases are distinguishable without reading prose.
- SC-004: Existing issue and identity compatibility tests continue to pass.
