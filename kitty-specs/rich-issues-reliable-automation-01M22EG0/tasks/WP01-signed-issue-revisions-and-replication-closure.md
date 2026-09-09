---
work_package_id: WP01
title: Signed issue revisions and replication closure
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-010
- NFR-001
- C-001
- C-002
- C-003
planning_base_branch: feat/rich-issues-reliable-automation
merge_target_branch: feat/rich-issues-reliable-automation
branch_strategy: Planning artifacts for this mission were generated on feat/rich-issues-reliable-automation. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/rich-issues-reliable-automation unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-rich-issues-reliable-automation-01M22EG0
base_commit: 99ddcb9d9e2ab95226418d2fa8b572916db9e843
created_at: '2026-09-09T07:03:30.965420+00:00'
subtasks:
- T001
- T002
- T003
- T004
history: []
agent_profile: implementer-ivan
authoritative_surface: event.go
create_intent:
- issue_model.go
- issue_projection.go
- issue_model_test.go
- issue_projection_test.go
- issue_replication_test.go
execution_mode: code_change
owned_files:
- event.go
- store.go
- quarantine.go
- shallow.go
- issue_model.go
- issue_projection.go
- issue_model_test.go
- issue_projection_test.go
- issue_replication_test.go
role: implementer
tags: []
tracker_refs: []
---

# WP01 — Signed issue revisions and replication closure

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `codex`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this work package's `task_type` and `authoritative_surface`.

---

## Objective

Add optional signed issue state and immutable revisions under the opening issue ID.
Reconstruct deterministic heads, history and relationship diagnostics, and ensure
replication never admits these facts without their exact signed dependencies.

## Context

The mission is `rich-issues-reliable-automation-01M22EG0`; read its spec and plan.
Use the runtime-provided worktree, never the root checkout for implementation.
Start with `spec-kitty agent action implement WP01 --agent codex --mission rich-issues-reliable-automation-01M22EG0`.
Resolve action-scoped implementation governance before making code changes.

Current `Event` marshaling is signed and content-addressed. Preserve field order
and bytes for every existing event whose new fields are absent. Existing issues
have only `issue.open` and `issue.comment`; the opening event remains identity.
Actor chain order and issue revision ancestry are different graphs. Any admitted
signed contributor can revise an issue; proposal-author restrictions do not apply.

This package owns protocol, projection and closure, not public command parsing.
WP02 independently builds the bounded reader using existing validator signatures.
WP03 consumes this package's types, projection and canonical semantic validation.
Publish implemented interface details to the coordinator before review handoff.
Keep `validateActorChains` and `validateEventRelationships` signatures stable.
Do not edit WP02's `issue_read.go`, commands, main dispatch or documentation.

### Subtask T001: Define optional rich state and signed event validation

**Purpose**: Give issues useful structure without changing legacy signatures or
allowing unrelated event variants to smuggle issue operation data.

**Steps**:
1. Add `IssueState`, criterion and relation types in `issue_model.go`.
   State contains title, body, status, criteria, labels, assignees, relations,
   and namespaced string metadata. Keep these values informational.
2. Append these optional fields at the end of `Event` in `event.go`:
   `Issue *IssueState`, `Parents []string`, `Operation string`, `Request string`,
   and `Intent string`, all with `omitempty` JSON tags matching plan vocabulary.
   Do not reorder or retag existing fields or bump the signed protocol gratuitously.
3. Extend `validateEventContent` with `issue.revise` and issue variant checks.
   A revision requires a full root Subject, full replacement state, and 1..200
   sorted unique parent event IDs. Reject unrelated Title/Body on revisions.
   Rich opening Title/Body must match state; openings reject Parents.
   Comments reject Issue and Parents. Non-issue variants reject all new fields.
4. Preserve legacy title/body acceptance when new issue fields are absent.
   Rich state requires a nonblank title and status exactly open or closed.
   Enforce byte bounds: title 1024; body 65536; 64 criteria, IDs at most 64,
   criterion text at most 4096; 64 labels and assignees, each at most 256.
   Relations cap at 128. Metadata caps at 64 pairs, keys 128, values 4096.
   New encoded payload/request caps at 256 KiB as specified by the plan.
5. Reject duplicate criterion IDs, labels, assignees and identical links.
   Relation kinds are blocks, related and parent; targets are full issue IDs.
   Metadata keys contain a nonempty prefix and name separated by `/`.
   Canonicalize set-like fields deterministically; preserve criteria order.
   Avoid silently discarding content, accepting control-only identifiers or
   mutating a caller's state through shared slice/map references.
6. Validate optional operation metadata all-or-neither: key and request digest.
   Key bounds are 128 printable non-whitespace ASCII bytes.
   Sign Intent and check open/revise/resolve/close/reopen/comment kind mapping;
   close/reopen must agree with state status. A rich event's operation digest
   must be reconstructible from semantic content, not an unverified assertion.
   Coordinate shared canonical digest helpers with WP03; CLI replay stays there.

**Files**: `event.go`, new `issue_model.go`, new `issue_model_test.go`.
Keep the model narrowly issue-specific; no generic schema framework or dependency.

**Validation**:
- Fixed legacy payload fixture marshals byte-for-byte identically and verifies.
- Existing oversized legacy title/body remains readable; equivalent new rich
  state exceeds its documented limit and is rejected before signing/appending.
- Round-trip every optional state field without loss; test limit and limit+1.
- Table-test every new field on wrong event variants and malformed parent IDs.
- Verify signatures fail after changing state, parents, operation or intent.
- Prove canonical set ordering and unchanged criterion ordering in digest inputs.
- Test empty namespace components, duplicate IDs and unknown relation/status.

### Subtask T002: Build the pure revision and relationship projection

**Purpose**: Derive a checkable catalog from already verified facts, preserving
all concurrent edits and providing reusable local mutation guards.

**Steps**:
1. Implement `BuildIssueCatalog(events []StoredEvent) (*IssueCatalog, error)`
   in `issue_projection.go`. Index roots and issue facts by full event ID.
   Legacy roots project title/body and status=open with optional collections empty.
   Validate state and exact root/parent/target relationships before projecting.
2. Require every revision parent to be a root or revision of the same issue.
   Detect ancestry cycles deterministically without unbounded recursive stack use.
   A parent may precede or follow its child in supplied slice/timestamp order.
   Retain missing references as errors, not silent omissions or implicit roots.
3. Compute maximal heads from the entire revision DAG, independent of timestamps.
   Return full root ID, creator, attributed heads and an explicit conflict flag.
   Single-head state is available; conflicting view state is nil, never a winner.
   Keep histories and comments indexed for later bounded CLI pagination.
   Return or document deterministic order: topological history with ID tie-breaks,
   comments timestamp/ID and heads full ID order.
4. Expose relationship edges in both incoming and outgoing queries.
   Parent means source child to target parent; blocks means source blocks target.
   Related is signed on its source but visible from both endpoints.
   Preserve source issue and revision provenance, including all conflict heads.
   Mark ambiguous edges arising from conflict rather than implying readiness.
5. Detect blocks and parent cycles in the projected current graph.
   Replicated concurrent cycles are diagnostics, not grounds to drop valid facts.
   Reject local candidate self-links, missing targets and newly introduced cycles
   through a focused helper for WP03; do not reject harmless edits solely because
   an unrelated preexisting distributed cycle exists elsewhere in the catalog.
6. Provide exact-head comparison helpers or clearly documented data sufficient
   for WP03 guards. Ordinary revisions use one observed head; ordinary resolution
   consumes the entire head set. Staged resolution can consume selected heads
   without erasing unconsumed heads; command snapshot validation belongs to WP03.
   Do not introduce global CAS, leases, approval state or acceptance semantics.

**Files**: new `issue_projection.go`, new `issue_projection_test.go`;
shared pure validators may live in `issue_model.go`.

**Validation**:
- Shuffle event input and reverse sibling timestamps; catalog semantics match.
- Two sibling edits produce two attributed heads and nil selected state.
- Explicit multi-parent successor converges to one head; partial reconciliation
  keeps every unconsumed head visible, including more than 200 total heads.
- Wrong-root parent, comment-as-parent, missing parent and synthetic cycle fail.
- Incoming links and conflict provenance survive deterministic reconstruction.
- Cross-replica parent/blocks cycle yields diagnostics without invalidating facts.
- Local candidate cycle rejection leaves input projection and signed facts intact.
- Empty catalog, isolated legacy root and comment-only additions remain coherent.

### Subtask T003: Integrate relationship validation and replication closure

**Purpose**: Ensure accepted histories and selective replication enforce the same
issue dependency graph, with actionable quarantine recovery for missing suppliers.

**Steps**:
1. Extend `validateEventRelationships` in `store.go` for issue roots, revisions
   and links. Reuse pure validators without circular calls into the catalog.
   Preserve existing proposal, identity, CI and actor-chain behavior/signatures.
2. Extend `replicationEventReferences` in `quarantine.go` to enumerate issue
   Subject, every Parents entry and every relation target as appropriate.
   Rich opening links also create dependencies despite having no Subject.
   Deduplicate references deterministically without omitting any supplier.
3. Extend `replicationEventDependency` to validate referenced issue kinds and
   same-root lineage. Continue using existing dependency-missing classification,
   supplier ownership and recovery guidance. Check more than the first parent
   or first link when deciding a selected history has complete closure.
4. Inspect quarantine dependency propagation: a present fact whose supplier is
   itself quarantined must not make a dependent actor eligible for promotion.
   Preserve exact selectors and budgets; never fetch/promote unknown suppliers
   automatically or treat object presence as accepted-history authority.
5. Inspect `validateExactEventReferenceClosure` and shallow recovery in
   `shallow.go`. Reuse the shared reference enumeration so shallow accepted reads
   cannot hide missing issue ancestors or target roots. Change shallow.go only
   if required; record evidence if the shared seam fully covers it already.
6. Search closed event-kind switches across the repository and report any needed
   out-of-ownership edits to the coordinator before changing another package.
   Missing dependency is distinct from wrong-kind/malformed signed dependency.
   Do not weaken signature validation or existing pending-admission guards.

**Files**: `store.go`, `quarantine.go`, `shallow.go` when required,
new `issue_replication_test.go` with temporary Git repositories and identities.

**Validation**:
- Selecting an actor with a revision but without its root supplier quarantines it.
- Missing second parent and rich-opening relation target independently quarantine.
- Present-but-quarantined supplier transitively prevents dependent promotion.
- Selecting exact missing suppliers then syncing admits the unchanged signed
  facts and reconstructs the expected issue heads without rewriting history.
- Wrong-kind or cross-root reference is invalid, not an auto-recoverable omission.
- Shallow reads fail with full missing fact identity and existing recovery path.
- Accepted refs remain unchanged on rejected admission; selectors stay unchanged.

### Subtask T004: Prove protocol compatibility and publish integration contract

**Purpose**: Deliver an independently reviewable foundation that the reader and
command packages can consume without guessing about authority or error semantics.

**Steps**:
1. Complete focused behavioral tests across the three new test files; reuse
   existing Git/identity fixtures without altering another package's owned tests.
2. Exercise signing, storing, loading and projection together for one legacy
   issue upgraded to rich state and then revised by a second actor.
   Retain exact original opening payload, signature, event ID and Git object.
3. Test semantic operation digest validation separately from command replay.
   Altered signed request digest cannot stand in for reconstructed semantic data.
   Identical keys from different actors remain distinct; expose enough data for
   WP03 to detect duplicate same-actor operation records without arbitrary choice.
4. Run focused tests first, then formatting and charter regression gates:
   `go test -race ./...`, `go vet ./...`, `go build ./...`, `git diff --check`.
   Capture exact commands, outcomes and any preexisting failures honestly.
5. Send the coordinator signatures and semantics of public package helpers,
   structs, canonicalization and errors; explain any justified plan refinement.
   Do not change WP02/WP03 files, wps.yaml or execution-lane configuration.
6. Commit only owned implementation/test paths using targeted staging, then
   use runtime task status and review transitions. Do not self-approve this WP.

**Files**: owned implementation/test files only; no standalone documentation file.

**Validation**:
- Golden legacy bytes and all preexisting proposal/CI/identity tests pass.
- New rich events survive admitted replication with their complete dependencies.
- No signed record is edited, no new namespace/service/dependency is introduced.
- Coordinator receives a concrete helper contract before dependent work starts.

## Definition of Done

- Signed rich state, revision validation, pure projection and exact closure agree.
- Conflicts preserve all head states; replicated cycles remain explicit diagnostics.
- Legacy signed bytes and existing collaboration workflows retain regression gates.
- Full regression evidence and a fixed implementation commit are ready for review.
- Record each completed subtask with `spec-kitty agent tasks mark-status T001 --status done --mission rich-issues-reliable-automation-01M22EG0` (repeat for T002–T004).
  Event-sourced records and test evidence are completion authority, not checkboxes.

## Risks

Signature drift is mitigated by appended optional fields and fixed byte fixtures.
Dependency omission is mitigated by testing root, later parent and link suppliers.
Timestamp winner selection is prohibited by shuffled/reversed-time projection tests.
Recursive traversal and repeated graph scans must remain safe within reader budgets.
Assignment and status must never be interpreted as execution or review permission.

## Reviewer Guidance

Independently inspect signed-byte compatibility and hostile variant validation.
Trace a missing dependency through selection, quarantine, promotion and shallow read.
Check that all conflict heads and edge provenance survive order-independent rebuilds.
Distinguish snapshot-based local guards from actor CAS and global coordination.
Reject hidden timestamp winners, automatic supplier admission, rewritten history,
or integration helpers that require consumers to trust an unsigned cache.
