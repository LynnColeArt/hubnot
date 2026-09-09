---
work_package_id: WP04
title: Public consumer acceptance and documentation
dependencies:
- WP03
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
- FR-010
- FR-011
- FR-012
- NFR-001
- NFR-002
- NFR-003
- C-001
- C-002
- C-003
planning_base_branch: feat/rich-issues-reliable-automation
merge_target_branch: feat/rich-issues-reliable-automation
branch_strategy: Planning artifacts for this mission were generated on feat/rich-issues-reliable-automation. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/rich-issues-reliable-automation unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-rich-issues-reliable-automation-01M22EG0
base_commit: 2d5cde20d697dedb9503b2d4279ef7b73c29998e
created_at: '2026-09-09T08:17:40.630782+00:00'
subtasks:
- T013
- T014
- T015
- T016
history: []
agent_profile: implementer-ivan
authoritative_surface: issue_acceptance_test.go
create_intent:
- issue_acceptance_test.go
- examples/issue-consumer/main.go
- examples/issue-consumer/README.md
- docs/issues-v1.md
- docs/threat-model.md
execution_mode: code_change
owned_files:
- issue_acceptance_test.go
- examples/issue-consumer/**
- README.md
- docs/issues-v1.md
- docs/protocol-v0.md
- docs/threat-model.md
role: implementer
tags: []
tracker_refs: []
---

# WP04 — Public consumer acceptance and documentation

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `codex`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this work package's `task_type` and `authoritative_surface`.

---

## Objective

Prove the richer issue model through the public CLI with real temporary Git
repositories and publish usable human and automation guidance matching the code.
Ship a runnable general consumer and measured, reproducible regression evidence.

## Context

Start after WP03 approval through the runtime-managed integration lane:
`spec-kitty agent action implement WP04 --agent codex --mission rich-issues-reliable-automation-01M22EG0`.
Read the mission spec, plan and completed dependency implementation/review notes.
Load action-scoped implementation governance before edits.

This package accepts the integrated contract; it does not own protocol or CLI
implementation. Report discovered defects to the coordinator with a failing test
and exact reproduction; do not quietly patch another package's source files.
Existing `operational_acceptance_test.go` supplies public-command/build/Git
helpers that may be reused without changing that file. Capture stdout and stderr
separately where existing combined-output helpers obscure the JSON guarantee.

The opening event is stable issue identity. Revision heads express concurrent
content, while actor CAS controls only one actor's append. Assignment, criteria,
status and metadata confer no Go Kitty execution or review authority.
No Beads migration, remote publication, service or new dependency is in scope.
Tests must use synthetic identities and temporary repositories/remotes exclusively.

### Subtask T013: Ship and exercise a public-CLI consumer example

**Purpose**: Demonstrate that a small ordinary program can read, update and retry
issues without importing Hubnot internals or depending on Go Kitty.

**Steps**:
1. Create `examples/issue-consumer/main.go` using the Go standard library.
   Execute a caller-supplied `hn` executable with argument arrays, not a shell.
   Accept an explicit repository path; use no ambient production repository.
   State that the example intentionally creates and updates demonstration work.
2. Parse only documented `hn.issue/1` JSON envelopes with schema/ok checks,
   full IDs, typed data and stable error codes. Keep stderr diagnostic handling
   separate from stdout machine parsing and preserve nonzero subprocess status.
   Bound subprocess time and response reads; never interpret issue text as code.
3. Demonstrate rich creation with an operation key, read/list of the result,
   expected-head full-state revision, and replay of the identical request.
   Replay must return the same event ID. Follow with an intentionally stale
   request using a different operation key and recognize stale_revision.
4. Make demonstration output explain issue ID, original and updated revision,
   replay identity and stale rejection. Do not infer global execution readiness
   from successful mutation or issue status. Keep required example inputs small.
5. Add `examples/issue-consumer/README.md` with exact build/run commands,
   prerequisites, temporary-repository setup and expected outcome fields.
   Demonstrate the machine JSON document rather than shell text scraping.
   Do not require jq, Python, a running server or additional Go modules.
6. Invoke the example from `issue_acceptance_test.go` against the built public
   CLI and a temporary initialized repository. Read back the resulting issue
   using the CLI to validate the consumer's claims independently.

**Files**: new example main/README and new `issue_acceptance_test.go`.
Prefer one small executable over a reusable SDK or framework.

**Validation**:
- The example builds as part of `go build ./...` with no dependency changes.
- Execution uses only documented public commands and successfully parses JSON.
- Rich title/body/criteria/labels/assignees/metadata survive read-update-read.
- Replay returns exactly the original revision and appends no signed event.
- A stale write is recognized by typed error and leaves accepted refs unchanged.
- An invalid executable/repository yields a clear error without false success.

### Subtask T014: Prove human journeys and distributed conflict recovery

**Purpose**: Exercise the four user stories at the repository boundary, including
replication closure and bounded recovery when concurrent heads exceed one page.

**Steps**:
1. Add public CLI tests for title-only open, human list/show, comment and rich
   revise, close and reopen. Use full returned mutation IDs and preserve the
   opening identity throughout. Compare original signed payload/signature bytes
   using Git object inspection before and after revisions.
2. Initialize two or more clone directories with separate actor keys and a real
   temporary bare remote. Create a common issue and sync accepted histories.
   Make independent revisions from the same observed issue head before syncing.
   Use actual `hn sync` and existing selection commands, not fake catalog injection.
3. Show and page the conflicting issue's heads. Assert both authors/states remain
   inspectable and no single authoritative state is emitted. Reverse event arrival
   order in an independent run; timestamp-order invariance has WP01 unit evidence,
   while this test proves the real transport never selects a winning timestamp.
4. Resolve all observed heads through the CLI, sync again and prove both replicas
   converge on exactly one successor while preserving both predecessor revisions.
   Incomplete ordinary resolution and stale head sets must append nothing.
5. Exercise staged recovery through `heads` pages and explicit partial resolution
   with the observed snapshot. Use enough valid independent heads to exceed 200
   in a bounded dedicated fixture; inspect every head through public pagination.
   Valid signed fixture construction may avoid hundreds of expensive CLI setup
   calls, but recovery, inspection and final convergence must use public commands.
   Each partial resolution consumes selected heads and preserves unselected ones.
   A changed snapshot rejects the partial request without advancing any actor ref.
6. Cover typed graph behavior: incoming/outgoing links, conflict provenance and
   blocks/parent cycles introduced concurrently. Local cycle creation is rejected;
   valid distributed facts remain readable with explicit cycle diagnostics.
7. Exercise selection missing a root, later parent or relation supplier. Assert
   quarantine prevents promotion, names exact missing facts, and preserves selectors.
   Add required supplier selections explicitly, sync and recover the same signed facts.

**Files**: `issue_acceptance_test.go`; reuse existing helpers read-only.
Use bounded test deadlines and avoid sleeps as substitutes for deterministic setup.

**Validation**:
- All four user stories have public-command acceptance assertions.
- Both conflict heads survive transport; convergence consumes them explicitly.
- More than one page of heads remains recoverable without hidden truncation.
- Stable issue IDs and original signed records survive close/reopen and sync.
- Missing supplier recovery respects exact selection and existing admission rules.
- Test cleanup leaves no remote, key or branch in the developer's real repository.

### Subtask T015: Test failure invariants and navigation contracts

**Purpose**: Verify automation can distinguish failure, stale observations and
successful retries without guessing from prose or accidental ref movement.

**Steps**:
1. Add a helper that captures all relevant `refs/hn/*` names/OIDs before and after
   each rejected mutation. Compare exact mappings, not only reported event count.
   Check zero writes for invalid input, stale_revision and operation_conflict.
   For actor races, distinguish the competing writer's legitimate advancement
   from this request appending an extra event; do not assert no external writes.
2. Test identical-key replay after another revision or unrelated actor activity.
   Changed body, parent, target issue or convenience intent with the same key
   must fail. Exercise close/reopen replay so current-state derivation cannot
   turn an identical retry into a different semantic request.
3. Cover unknown/duplicate JSON fields, trailing documents, malformed IDs,
   ambiguous read prefixes, short mutation IDs, overlarge fields and request
   input just above 256 KiB. Test file and stdin inputs through subprocesses.
4. For JSON mode, assert exactly one stdout JSON envelope, expected schema and
   ok flag, full IDs, stable code and nonzero exit on failures. Empty collections
   are arrays. Hostile content stays data and cannot introduce terminal commands
   or human prose into machine stdout. Do not overfit volatile message wording.
5. Page list, heads, history and graph at small limits; concatenate pages and
   compare with expected stable ordering and complete unique record identities.
   Changing a relevant ref invalidates old cursors. Wrong-query, malformed and
   out-of-range cursors fail explicitly rather than returning an empty success.
   Validate default 50 and maximum 200 records without expanding output silently.
6. Consume WP02 reader evidence: 1000 issues, batch process count independent of
   event count, measured duration and equality with existing verified loading.
   Add a public-query smoke over the fixture where useful; do not invent a
   latency SLO or repeat the full benchmark for every small test case.
7. Inspect existing race/corruption tests from prior WPs and avoid duplicating
   their internals; fill remaining public boundary gaps. Defects go back to their
   owning package with exact command, expected behavior and captured evidence.

**Files**: `issue_acceptance_test.go` only.

**Validation**:
- Failed local mutations append nothing; successful replay appends nothing.
- Typed error and process exit agree for every tested public failure.
- Pagination neither skips nor duplicates records within an unchanged snapshot.
- Stale cursors and stale staged-resolution snapshots require explicit rereading.
- Read-model evidence names measured fixture size, timing and process counts.
- No test derives truth from unverified caches or treats status as permission.

### Subtask T016: Publish current documentation and integrated gate evidence

**Purpose**: Make the delivered behavior discoverable and reviewable, including
the precise limits of distributed concurrency and compatibility guarantees.

**Steps**:
1. Extend README with a short simple issue journey and a link to rich-issue docs.
   Preserve the existing namespace, experimental status and collaboration guidance.
   Update acceptance-test command examples to include the new named public suite.
2. Create `docs/issues-v1.md` documenting the actual implemented CLI and schema.
   Include minimal human usage, a complete rich JSON input, expected-head updates,
   operation replay, close/reopen, graph direction, conflict inspection/resolution,
   staged recovery and paginated navigation with all documented bounds.
3. Specify stable error codes and retry handling. Explain actor-scoped operation
   identity, signed Intent/request digest and replay-before-current-head behavior.
   Explicitly state preflight plus actor CAS is not cross-actor global CAS.
   Show readers how to inspect every conflicting state without selecting a winner.
4. Extend `docs/protocol-v0.md` field inventory and supported-kind list with the
   additive issue fields and `issue.revise`. Explain root identity, full-state
   ancestry, canonical set ordering and retained legacy bytes. Link detailed docs.
   Distinguish hn/0 signed events from hn.issue/1 CLI envelopes.
5. Create `docs/threat-model.md` scoped to issue additions and links to existing
   security documentation. Cover hostile fetched facts, admission closure, bounded
   input/ingestion, explicit conflicts and informational assignment/criteria.
   Explain that old executables may reject new event kinds; reverting a binary
   cannot erase newly signed history. No compatibility rewrite is implied.
6. Run documented examples against the candidate executable in disposable repos.
   Run gofmt for changed Go files, `go test -race ./...`, `go vet ./...`,
   `go build ./...`, and `git diff --check`. Record exact candidate commit and
   commands/results for coordinator acceptance; never cite old baseline results
   as evidence that this candidate passed. Repeat only after relevant changes.
7. Commit only owned paths and request independent review via runtime transitions.
   Do not mark the mission accepted/merged or manufacture Hubnot approval facts.

**Files**: README, docs/issues-v1.md, docs/protocol-v0.md, docs/threat-model.md,
example README and acceptance test refinements when needed.

**Validation**:
- Every documented command and field matches tested candidate behavior.
- All twelve FRs and all NFRs/constraints map to concrete tests or review evidence.
- Integrated race/vet/build/diff checks pass and carry a precise source identity.
- Limitations are explicit without implying a Beads replacement already occurred.

## Definition of Done

- Runnable consumer and public Git/CLI acceptance demonstrate all four user stories.
- Failure/ref invariants, staged conflict recovery and bounded navigation pass.
- Current issue/protocol/threat-model documentation matches implementation.
- Reader measurement and full candidate regression evidence are ready for review.
- Record completion for T013–T016 using `spec-kitty agent tasks mark-status T013 --status done --mission rich-issues-reliable-automation-01M22EG0` with the appropriate subtask ID.
  Event-sourced task records plus executable evidence establish completion.

## Risks

Subprocess tests can become slow; share executable builds and bounded fixtures.
Global working-directory helpers cannot run concurrently without isolation.
JSON tests must separate stdout from stderr and avoid asserting human wording.
Large conflict fixtures must test recoverability while remaining within budgets.
Documentation must state actual concurrency guarantees and retain alpha caveats.

## Reviewer Guidance

Run the consumer and representative public acceptance cases independently.
Trace expected-head replay and ref invariants through real subprocess results.
Verify staged recovery preserves unselected heads and rejects stale snapshots.
Compare protocol docs with signed field order and command help with test examples.
Reject acceptance claims based on mocks, prior commits, unsigned cache truth,
or informational fields being interpreted as execution or approval authority.
