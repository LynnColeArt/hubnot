# Mission Specification: Rich Issues and Reliable Automation

**Mission Branch**: `feat/rich-issues-reliable-automation`  
**Created**: 2026-09-09  
**Status**: Specified  
**Input**: User authorized a mission to make Hubnot more useful to Go Kitty without harming its general utility, particularly through a richer work model, and delegated functionality decisions.

## User Scenarios & Testing

### User Story 1 — Keep simple issues useful as work grows (P1)
A person creates an issue with a title, then adds description, criteria, labels, relationships and assignees as needed. They edit, close and reopen it without losing its identity or earlier content.
**Independent test**: Existing title-only and comment commands still work. A rich issue round-trips through create, revise, close, reopen, show and history. Earlier signed records remain unchanged.
**Acceptance scenarios**:
1. Given a legacy issue, adding optional structure produces a new revision under the same issue ID.
2. Given a simple issue, listing or viewing it requires no schema authoring or integration setup.
3. Given assignments and completed work, the record communicates ownership and lifecycle without granting execution permission.

### User Story 2 — Automate work without guessing (P1)
A consumer lists issues, reads full IDs and current revisions, submits an expected-revision update, and safely retries after losing a response.
**Independent test**: A subprocess consumer uses only documented JSON commands to create, inspect, update and retry; the replay has the same event ID and does not append another event.
**Acceptance scenarios**:
1. Identical operation key and request returns the original result even after later unrelated changes.
2. Reusing a key for different content fails with a stable diagnostic and leaves refs unchanged.
3. A stale expected revision fails without hiding newer work. Bounded pages neither omit nor duplicate items within an unchanged snapshot.

### User Story 3 — Reconcile distributed edits (P1)
Two collaborators edit the same issue before syncing. Both changes remain visible; a person resolves them explicitly.
**Independent test**: Two actors in temporary clones produce sibling revisions, sync, inspect both heads, then resolve them into one successor. Repeat with timestamp ordering reversed.
**Acceptance scenarios**:
1. A conflict has no silently selected current state; every head is inspectable with attribution.
2. Resolution consumes the complete observed head set; incomplete or stale local resolution fails.
3. Invalid revision parents, missing issues and forged signed content cannot become valid work state.

### User Story 4 — Inspect relationships efficiently (P2)
A person or consumer asks how issues relate and obtains bounded, deterministic results from the repository.
**Independent test**: A realistic issue fixture yields identical rebuilt results, includes incoming and outgoing links, and reports dependency cycles explicitly.

### Edge Cases
Missing or ambiguous IDs; self-links; missing relation targets; invalid namespace keys; duplicate criteria IDs; empty or overlarge fields; unknown JSON fields; stale page snapshots; actor-head races; same operation key from different actors; concurrent operations with the same key; unrelated histories with equal timestamps; tampered events; dependency cycles created across replicas; closed issues with unresolved conflicts.

## Requirements

### Functional Requirements
| ID | Title | User Story | Priority | Status |
|----|-------|------------|----------|--------|
| FR-001 | Compatible simple issues | Preserve title-only creation, legacy signed issues, comments and human-readable list/show with stable opening identity. | High | Open |
| FR-002 | Optional rich state | Support title, description, criteria with stable local IDs, labels, assignees, open/closed status and namespaced string metadata; round-trip without loss. | High | Open |
| FR-003 | Immutable revisions | Edits append signed full-state revisions referencing the stable issue and predecessor revisions; retain inspectable history and author attribution. | High | Open |
| FR-004 | Conflict preservation | Compute all current heads; unresolved issues expose each head and no authoritative single state; do not use timestamps to resolve concurrent edits. | High | Open |
| FR-005 | Guarded edit and resolution | Local edits require the exact observed head; explicit resolution requires the complete observed head set. Stale requests append nothing. Distributed races remain explicit conflicts. | High | Open |
| FR-006 | Typed relationships | Support blocks, related and parent links using full issue IDs; reject self/missing targets on local writes; expose incoming/outgoing links and diagnose blocks/parent cycles, including after sync. | High | Open |
| FR-007 | Reliable operation replay | Optional actor-scoped operation IDs on create, revise/resolve and comment return the original event for identical requests; divergent key reuse fails without a write. | High | Open |
| FR-008 | Machine interface | Document JSON input and versioned JSON output for issue mutation/query, full IDs, typed stable errors and nonzero failure exit. Machine mode emits no human prose on stdout. | High | Open |
| FR-009 | Bounded navigation | List, history and graph offer deterministic bounded pages and snapshot-aware continuation; reject stale continuations rather than silently skip work. | High | Open |
| FR-010 | Verified read model | Derive the issue catalog, revision heads, comments and graph from verified signed histories; rebuild produces identical results and never depends on an authoritative cache. | High | Open |
| FR-011 | General consumer example | Ship a runnable public-CLI consumer example/test demonstrating read/update/replay/stale rejection with no Go Kitty runtime dependency. | Medium | Open |
| FR-012 | Discoverable behavior | Update CLI help and current protocol/user documentation together with tested ordinary-user and conflict-recovery examples. | High | Open |

### Non-Functional Requirements
| ID | Title | Requirement | Category | Priority | Status |
|----|-------|-------------|----------|----------|--------|
| NFR-001 | Bounded inputs and outputs | New JSON request inputs are at most 256 KiB; page size defaults to 50 and caps at 200; field, graph and traversal bounds are documented and enforced before append. | Safety | High | Open |
| NFR-002 | Efficient verified reads | Reading a 1000-issue fixture must use batched event-object access without per-event Git subprocesses; measure duration and process counts, and verify semantic equality with the existing loader. | Performance | High | Open |
| NFR-003 | Regression evidence | Formatting, go test -race ./..., go vet ./..., go build ./..., and git diff --check pass for the integrated candidate; black-box tests include two-replica conflicts and replay failure boundaries. | Quality | High | Open |

### Constraints
| ID | Title | Constraint | Category | Priority | Status |
|----|-------|------------|----------|----------|--------|
| C-001 | Repository authority | Signed Git histories remain the only durable authority; no mandatory service, database or third-party dependency. | Architecture | High | Open |
| C-002 | Consumer independence | Issue status, assignment, criteria and metadata are descriptive; no Go Kitty leases, execution authorization, review acceptance or integration authority. No Beads cutover in this mission. | Product | High | Open |
| C-003 | Compatibility and scope | Preserve hn namespaces, existing signed bytes, proposal/CI/identity/replication/memory behavior and unrelated historical work. No web UI or new sync protocol. | Compatibility | High | Open |

### Key Entities
Issue, full-state revision, current head set, criterion, typed relationship, comment, actor-scoped operation, verified projection and snapshot cursor.

## Success Criteria
- SC-001: All four user stories pass executable acceptance scenarios through the public CLI.
- SC-002: Replaying a successful operation produces zero additional signed events; stale or malformed mutations produce zero ref changes.
- SC-003: Two independent edits remain visible after sync and explicit reconciliation produces exactly one current head.
- SC-004: Simple existing issue usage requires no additional required field or setup step.
- SC-005: Every requirement has implementation and independent review evidence, with measured 1000-issue read behavior and the full regression gate passing.

## Assumptions and scope decisions
The user delegated design decisions. Accepted signed contributors may edit issues, with visible attribution; signature validity conveys no execution permission. A full state replacement is the minimal reliable machine mutation; convenience flags may compose it. The verified read model is rebuilt in memory using batched reads, so there is no disk-cache trust problem. Main remains untouched until the complete candidate is accepted; the mission lands first on its dedicated feature branch.
