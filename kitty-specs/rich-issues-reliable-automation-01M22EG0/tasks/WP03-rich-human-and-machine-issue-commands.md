---
work_package_id: WP03
title: Rich human and machine issue commands
dependencies:
- WP01
- WP02
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-012
- NFR-001
- C-002
- C-003
planning_base_branch: feat/rich-issues-reliable-automation
merge_target_branch: feat/rich-issues-reliable-automation
branch_strategy: Planning artifacts for this mission were generated on feat/rich-issues-reliable-automation. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/rich-issues-reliable-automation unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-rich-issues-reliable-automation-01M22EG0
base_commit: 779c2ea15425b9646ca2fd21fb2b83382804ef4f
created_at: '2026-09-09T07:35:53.852673+00:00'
subtasks:
- T008
- T009
- T010
- T011
- T012
history: []
agent_profile: implementer-ivan
authoritative_surface: commands.go
create_intent:
- issue_commands.go
- issue_commands_test.go
execution_mode: code_change
owned_files:
- commands.go
- main.go
- issue_commands.go
- issue_commands_test.go
role: implementer
tags: []
tracker_refs: []
---

# WP03 — Rich human and machine issue commands

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `codex`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this work package's `task_type` and `authoritative_surface`.

---

## Objective

Expose richer signed issues through a usable human CLI and a reliable, bounded JSON interface.
Make retries, stale updates, distributed conflicts and partial reconciliation explicit and testable.
Preserve ordinary issue usage and keep descriptive work information separate from execution authority.

## Context

WP01 supplies signed state validation, canonical request semantics, revision projection and replication closure.
WP02 supplies `collectIssueEvents()` and its accepted-ref snapshot token.
Use these dependency surfaces; do not introduce a second projection or replace verification with cached JSON.
WP04 consumes this public interface for end-to-end acceptance, runnable consumer examples and documentation.
Read the current mission spec, plan and data model before deciding implementation details.
The implementation command is:

```bash
spec-kitty agent action implement WP03 --agent codex --mission rich-issues-reliable-automation-01M22EG0
```

Execute implementation only in the runtime-assigned lane after both dependencies are approved.
Never choose a lane base manually or merge sibling worktrees yourself.
Keep source edits within the four owned files and coordinate any dependency defect with its owner.
Do not modify mission manifests, another WP's tests, protocol documentation or unrelated command domains.
The user delegated product defaults; routine implementation choices do not require another discovery interview.
Preserve signed opening identity, full IDs in machine results and visible attribution for every head.
Snapshot preflight plus actor-ref CAS is the concurrency contract; it is not globally atomic issue CAS.

### Subtask T008: Establish command parsing and the strict JSON boundary

**Purpose**: Establish a consistent interface that ordinary users and subprocess consumers can invoke safely.

**Steps**:

1. Add red parsing and output tests in `issue_commands_test.go` before replacing existing issue handlers.
   Cover existing `issue open TITLE`, `open --body TEXT TITLE`, `comment`, `list` and `show` forms.
   Preserve existing non-issue commands in `commands.go` and routing behavior in `main.go`.
2. Move issue-specific implementation into `issue_commands.go` with a small dispatcher boundary.
   Preserve existing function entry points when other tests call them directly.
   Avoid duplicating old and new implementations with different issue semantics.
3. Parse opening flags before its free-form title, matching the established CLI convention.
   Parse the issue ID before flags for revise, resolve, close, reopen, comment and issue queries.
   Reject unexpected positional arguments and unsupported flag combinations with `invalid_input`.
4. Support rich opening and full-state mutations using `--input FILE` and `--input -`.
   Read at most 256 KiB plus one byte before decoding; never read an unbounded stream first.
   Return `resource_limit` for an oversized request and leave actor refs unchanged.
5. Strictly decode the documented state representation.
   Reject unknown fields, duplicate keys at every JSON object depth and trailing JSON values.
   Reject wrong types rather than converting strings, numbers, arrays or null implicitly.
   Coordinate canonical shape defaults with WP01 so signed content and request digests agree.
6. Define one versioned JSON envelope with `schema: hn.issue/1`, `ok`, and `data` or `error`.
   Errors include stable `code` and readable `message`; queries may include snapshot and continuation.
   Emit exactly one JSON document on stdout for success and failure in machine mode.
7. Integrate machine errors with `main.go` so failures produce nonzero exit status without duplicate envelopes.
   Human stderr text may remain, but sanitize hostile control characters in diagnostic interpolation.
   Never print human success prose before the JSON envelope.
8. Validate IDs, operation keys and flag syntax before loading unnecessary data or attempting a write.
   Mutations require full event IDs; read queries accept only unique short-ID matches.
   Missing and ambiguous references have distinct stable errors.

**Files**: create `issue_commands.go` and `issue_commands_test.go`; update issue routing in `commands.go` and error handling in `main.go`.
Keep parsing, rendering and mutation helpers cohesive; implementation size follows behavior rather than a line quota.

**Validation**:

- Parse stdout from real subprocess invocations as a single JSON value and assert EOF afterward.
- Exercise duplicate top-level keys, duplicate nested metadata keys, unknown fields and trailing values.
- Exercise empty input, truncated input, stdin input and the exact request-size boundary.
- Confirm invalid mutation inputs do not change any actor ref.
- Confirm an ordinary title-only issue still requires no JSON, operation key or extra setup.

### Subtask T009: Implement guarded full-state mutations and replay

**Purpose**: Append immutable issue changes while making lost-response retries reliable and stale edits visible.

**Steps**:

1. Implement open, revise and comment through a canonical mutation structure.
   Reuse WP01's field bounds, sorting, per-kind validation and semantic digest implementation.
   Preserve criteria order while normalizing only arrays declared set-like by the protocol.
2. Load the signer and capture `nextEvent` before collecting the verified issue snapshot.
   Preserve that captured sequence and predecessor throughout validation and append.
   Do not refresh the actor head after preflight and silently apply the request to different history.
3. Look up operation records scoped to the captured signer and supplied operation key.
   Operation keys contain 1..128 printable non-whitespace ASCII bytes when present.
   Absence of a key preserves ordinary append behavior, especially repeated human comments.
4. Reconstruct and validate the existing record's canonical request digest from signed fields.
   The digest binds Intent, event kind, subject, expected parents and canonical state or comment body.
   Do not trust a supplied request digest merely because the containing event has a valid signature.
5. Return the original event for an identical operation before checking current issue heads.
   Replays after intervening issue edits or unrelated actor events must still return the same event ID.
   Changed content, subject, expectations or intent under one actor/key returns `operation_conflict`.
6. Reject duplicate admitted records for the same actor/key with deterministic fail-closed diagnostics.
   Do not choose a record by timestamp, arrival order or its location in a map.
   The same operation key from another actor remains a separate operation.
7. For a fresh revise, require the exact single current head supplied with `--expect`.
   An unresolved issue returns `issue_conflict`; an out-of-date expected revision returns `stale_revision`.
   New revisions retain the opening event as their stable subject identity.
8. Validate relation targets and compare the candidate graph against existing graph state.
   Refuse self-links, missing targets and blocks/parent cycles introduced by this mutation.
   Existing concurrent cycles must remain inspectable and must not prevent edits that repair them.
9. Append with the captured actor state using the existing signer and actor-ref CAS.
   Map a competing actor-head change to retryable `concurrent_update`; never claim that append succeeded.
   A different actor advancing after the verified snapshot may produce a legitimate issue conflict.
10. Return full issue and event identities with enough mutation result data to identify a replay.
    Do not claim that assignment, closure or criteria convey permission to execute or accept work.

**Files**: mutation and replay helpers in `issue_commands.go`; targeted behavior tests in `issue_commands_test.go`.
Use existing dependency validators instead of adding overlapping validators to the owned command files.

**Validation**:

- Run create, revise and comment twice with the same operation and prove identical event IDs and unchanged ref counts.
- Repeat after later issue edits; repeat after unrelated signed actor events.
- Reuse a key with changed content, subject and expected parent and prove zero ref changes.
- Race same-actor requests and prove at most one append for a key; retry the loser deterministically.
- Race different actors and prove both facts remain visible after admission instead of silently winning.
- Confirm stale revisions and new local graph cycles leave all refs unchanged.

### Subtask T010: Add lifecycle commands and explicit conflict reconciliation

**Purpose**: Offer simple close/reopen operations and recover conflicts without hiding unconsumed heads.

**Steps**:

1. Implement `issue close ISSUE --expect REV` and `reopen` as signed full-state revisions.
   Preserve all state fields except the requested lifecycle status.
   Require one current head and apply the same stale preflight and actor CAS as ordinary revise.
2. Sign the concrete lifecycle Intent and bind it to the corresponding resulting state status.
   Before deriving current state, look for an identical prior operation using intent, subject and expected parent.
   For a matching convenience retry, reuse its recorded state and verify the canonical digest.
3. Reject changed lifecycle intent or expectations under the same actor/key.
   Closing, later reopening, then retrying the original close must replay the original event without closing again.
   Do not reconstruct a retry using the issue's newer state.
4. Implement ordinary `issue resolve ISSUE --expect REV ... --input FILE`.
   Require the complete currently observed head set, canonicalize parent order and reject duplicate expectations.
   Validate the submitted full replacement state through the same protocol surface as revise.
5. Do not infer a resolution from timestamps or silently retain one preferred head's fields.
   The submitted state is the explicit reconciliation; previous signed head states remain in history.
   Default resolution must not accept a subset and pretend the conflict is closed.
6. Implement explicit staged resolution with `--partial --snapshot TOKEN`.
   Require a matching current accepted-ref snapshot and 2..200 selected distinct current heads.
   Refuse non-head parents, stale snapshots and unsupported partial/snapshot flag combinations.
7. Append a successor for the selected heads while retaining every unconsumed head.
   Show the remaining conflict in the result rather than returning an unqualified resolved status.
   Permit repeated stages to reduce a head set larger than 200 to one head within reader budgets.
8. Preserve replay ordering for staged operations.
   An already successful retry is identified from its signed request before mutable snapshot checks.
   Snapshot guards are preflight checks; do not imply global atomicity across actors or synchronization.
9. Apply relation-cycle prevention to the actual candidate graph after consuming the selected heads.
   Retain provenance for edges from unconsumed alternatives.
   Reject newly introduced cycles without discarding already admitted conflicting facts.

**Files**: lifecycle and reconciliation handlers in `issue_commands.go`; focused tests in `issue_commands_test.go`.
Do not alter the protocol parent limit or append unvalidated special resolution events in this WP.

**Validation**:

- Round-trip close/reopen without changing title, body, criteria, labels, relations, assignees or metadata.
- Retry an old close after a later reopen and assert unchanged heads and original event identity.
- Build two sibling heads, reject incomplete default resolution, then converge through a complete resolution.
- Build more than 200 heads using signed fixtures and prove bounded staged reconciliation reaches one head.
- Prove partial resolution preserves unselected heads and rejects stale snapshot or non-current parents.
- Reverse input head order and timestamps and prove no implicit winner appears.

### Subtask T011: Expose bounded deterministic issue navigation

**Purpose**: Make issues, revision alternatives, history and graph usable without unbounded output or unstable pages.

**Steps**:

1. Build every issue query from the verified reader and WP01 catalog.
   Preserve human list/show ergonomics while displaying revised titles, lifecycle and conflict information honestly.
   Do not reconstruct current state from the most recent timestamp.
2. Provide machine list, show, heads, history and graph responses with full IDs.
   Keep `state` absent or null for conflicted issues; expose head alternatives with attribution.
   Serialize collection-valued fields as arrays, including empty results.
3. Default page size to 50 and cap it at 200; reject invalid, zero, negative or over-cap limits.
   Bound show's embedded head IDs and include total counts plus a discoverable route to remaining heads.
   Keep comments/history in explicit bounded pages rather than embedding unlimited records in show.
4. Define history item types so comments remain accessible alongside signed revision history without loss.
   Preserve topological revision ordering with event-ID tie breaks; comment ordering is timestamp then ID.
   Do not present comment timestamps as causal authority over revisions.
5. Use opaque cursors carrying version, observed snapshot, query fingerprint and validated offset.
   Bind resource identity, command and any filters into the query fingerprint.
   Validate numeric range before slicing or allocation; reject malformed cursors without panicking.
6. Return `stale_cursor` when the accepted snapshot changes between pages.
   Do not silently restart or continue on a new snapshot with omissions or duplicates.
   Unrelated code, proposal and memory refs excluded by WP02 must not invalidate issue pages.
7. Sort list results by full issue ID and head results deterministically by full revision ID.
   Sort graph entries by source, kind, target and provenance.
   If optional filters are added, keep their exact behavior in both cursor binding and CLI help.
8. Include incoming and outgoing graph relationships and distinguish signed source from displayed direction.
   Show related links bidirectionally without manufacturing a second signed fact.
   Include conflict-head provenance, ambiguity and blocks/parent cycle diagnostics.
9. Bound graph output and any diagnostic collections; do not hide truncation or invent readiness answers.
   A valid but conflicted or cyclic graph remains inspectable.
   Queries never mutate refs, create issue facts or refresh a persistent authoritative cache.

**Files**: query, rendering and cursor helpers in `issue_commands.go`; pagination tests in `issue_commands_test.go`.
Reuse the dependency snapshot token rather than computing a broader or weaker command-specific ref identity.

**Validation**:

- Traverse every page of a stable fixture and prove exactly-once coverage with deterministic ordering.
- Reuse a cursor for another issue, command or filter and reject it.
- Test malformed offsets, huge numeric offsets, truncated tokens and unsupported cursor versions.
- Append an accepted issue fact between pages and prove explicit stale continuation rejection.
- Page every head of a large conflict; verify show reports the total and does not conceal alternatives.
- Inspect incoming/outgoing relations and cycles with conflicting heads and assert provenance remains intact.

### Subtask T012: Prove the public command contract and update discoverability

**Purpose**: Supply reviewable behavior evidence and a stable interface for WP04's independent consumer trial.

**Steps**:

1. Consolidate public command tests around temporary Git repositories and synthetic test identities.
   Never use the developer's actual signing identity, accepted refs or production remote.
   Assert real exit status, stdout JSON, resulting signed event identity and ref changes.
2. Keep parser tests targeted, and use signed fixture construction only where public setup would obscure a boundary.
   Exercise public commands for every user-facing mutation and query.
   Distinguish implementation helper tests from end-to-end evidence in the handoff.
3. Cover the stable error vocabulary: invalid_input, not_found, ambiguous_id, stale_revision and issue_conflict.
   Also cover operation_conflict, concurrent_update, stale_cursor, resource_limit, invalid_history and repository_error.
   Choose deterministic boundary injection for rare failures; do not depend on timing sleeps alone.
4. Record baseline refs around rejected operations and compare afterward.
   Orphaned Git objects from a failed CAS are not published signed events; verify the actual ref boundary.
   Lost-response replay tests must prove no extra reachable event rather than matching only printed text.
5. Update issue help and top-level discovery in the owned command files.
   Describe full-ID mutation requirements, flags after issue IDs, input-file forms and conflict commands.
   Include heads pagination and explicit staged resolution; avoid consumer-specific terminology.
6. Provide WP04 a concise handoff of finalized JSON field names, commands and any justified contract refinement.
   Flag a necessary plan adjustment to the orchestrator before documenting conflicting behavior as settled.
   Do not edit WP04-owned user or protocol documentation from this WP.
7. Format changed Go files and run focused issue tests first.
   Before review, run the required repository gate: `go test -race ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`.
   Capture actual command results; do not treat a test's existence as passing evidence.
8. Commit only the owned implementation and test changes with a concise behavior-focused message.
   Report the fixed commit, commands executed and any remaining limitation for independent review.
   Let the runtime record review transitions; do not self-approve or advance directly to done.

**Files**: `issue_commands_test.go`, `issue_commands.go`, `commands.go` and `main.go` only.
Test files may be substantial because they establish an externally consumed contract; avoid assertions that merely mirror helpers.

**Validation**:

- Run all old issue behavior tests and the new machine contract tests successfully.
- Confirm non-issue command regressions are covered by the complete required gate.
- Inspect real machine output for one rich issue, one conflict, one replay and one stable error.
- Verify help documents every shipped issue subcommand and required mutation flag.
- Supply independent review with the implementation commit and complete gate evidence.

## Definition of Done

Every listed requirement has behavior evidence within this WP or a clear dependency reference.
Simple issue usage remains operational, and rich state round-trips without losing signed identity or history.
Retries append nothing, divergent operation reuse fails, and stale edits do not silently rebase.
Complete and staged reconciliation preserve all unconsumed heads and retain prior revisions.
Machine stdout is one parseable versioned response with full IDs and meaningful failure status.
All query collections are bounded, deterministic and explicit about conflicts, cycles and continuation.
The required quality gate passes for the fixed review candidate.
Record each completed subtask through the event-sourced status surface, for example:

```bash
spec-kitty agent tasks mark-status T008 --status done --mission rich-issues-reliable-automation-01M22EG0
```

Repeat for T009, T010, T011 and T012 only when their evidence exists; checkboxes are not completion records.

## Risks

- Replay after later changes can accidentally derive newer state: inspect signed intent before mutable guards.
- Same-actor races can invalidate preflight: retain the captured actor predecessor through append.
- Partial resolution can conceal work: retain unselected heads and report remaining conflicts explicitly.
- Standard JSON decoding accepts duplicates: enforce duplicate rejection before typed decoding.
- Pagination can exceed bounds through nested head/history output: page those collections independently.
- Existing graph cycles can block repair if checked indiscriminately: reject introduced cycles, not merely any cycle.
- Unrelated command refactors can regress behavior: keep the dispatch and error changes narrow.

## Reviewer Guidance

Review the fixed commit independently and exercise the public CLI, not only internal mutation helpers.
Trace one replay through canonical digest verification and prove all current-state guards occur afterward.
Inspect close/reopen replay after intervening edits and staged resolution after a consumed head disappears.
Check malformed input, response cardinality, cursor query binding and ref invariance on every failure boundary.
Verify graph ambiguity survives conflict and that assignment/status never grant execution or review authority.
Reject silent conflict winners, duplicate operation publication, unbounded nested output or unverifiable passing claims.
