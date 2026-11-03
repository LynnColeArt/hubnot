---
work_package_id: WP05
title: Integrated acceptance, maintained documentation and issue reconciliation
dependencies:
- WP04
requirement_refs:
- C-001
- C-002
- C-003
- C-004
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- NFR-001
- NFR-002
- NFR-003
planning_base_branch: fix/runner-efficiency
merge_target_branch: fix/runner-efficiency
branch_strategy: Planning artifacts for this mission were generated on fix/runner-efficiency. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/runner-efficiency unless the human explicitly redirects the landing branch.
subtasks:
- T021
- T022
- T023
- T024
- T025
phase: Phase 2 - Native efficiency
history:
- at: '2026-11-03T20:32:32Z'
  actor: planner-priti
  action: Authored scoped runner-efficiency tasks after confirmed spec/plan.
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- runner_efficiency_acceptance_test.go
execution_mode: code_change
owned_files:
- docs/ci-v0.md
- docs/protocol-v0.md
- docs/governance-v0.md
- docs/threat-model.md
- README.md
- CHANGELOG.md
- ci/fast-tests.txt
- .hn/actions/ci
- runner_efficiency_acceptance_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP05: Integrated acceptance, maintained documentation and issue reconciliation

## ⚡ Do This First: Load Agent Profile

Load implementer-ivan through the canonical Spec Kitty profile API/CLI before
working. Read spec.md, plan.md, research.md, data-model.md and the native runner
contract in this mission. Respect implementation/profile boundaries. The root
orchestrator owns cross-WP sequencing and independent approval.

## Objectives & Success Criteria

Deliver the specific behavior in this WP's subtasks with actual observable
acceptance evidence. Requirement references are registered through the canonical
map-requirements command. Preserve the first default-workflow gate throughout.

## Context & Constraints

Mission: runner-efficiency-01M6HH8H. Read the project charter's native protocol, immutable
evidence, least-authority and targeted-staging rules. The design reuses existing
lineage and trusted-result semantics rather than adding parallel policy logic.

## Branch Strategy

Planning/base and mission consolidation target: fix/runner-efficiency.
Final public landing: main via a reviewed PR. Work only in the runtime-returned
workspace; do not allocate a new raw Git worktree or alter old mission worktrees.
Implementation command: spec-kitty agent action implement WP05 --mission runner-efficiency-01M6HH8H --agent codex --profile implementer-ivan.
Dependencies: WP04; approved dependencies unblock this work.

## Subtasks & Detailed Guidance

### T021 — Add integrated native acceptance journey

Build a real public CLI/Git flow across requester and two runner identities, exact pipeline/results, revised/merged lineage, queued withdrawal and replica/archive continuation. Use marker/ref assertions with eligible positive controls. Add only bounded representative regressions to the native fast manifest; no hosted deep-corpus/matrix expansion.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T022 — Synchronize command, protocol and operator docs

Document command syntax, exact trust/evidence evaluation, queued-only withdrawal, manual-versus-automatic execution, old-reader rejection of new explicit kind, fresh stale-target snapshot semantics and credential/helper trust boundaries. Describe simultaneous-start limits honestly; preserve published evidence/history. Update unreleased changelog and practical native CLI examples.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T023 — Run integrated validation and preserve defaults

Run native fast/migration/integration tiers and focused standard/native-only compatibility, withdrawal and recovery tests at the final head. Include complete relevant source regression selection, vet/build/gofmt/diff checks, unchanged legacy signed fixtures and user/release preservation. Record actual scopes/exits; never call a scoped run a full-suite pass or replace a failed test with a claimed pass.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T024 — Reconcile issues and coordination follow-up

Before closing originals, tie #27/#30 to exact acceptance evidence. They are related, not duplicates. Search for an existing concurrent claims/lease item; if absent, prepare one focused native optional-coordination follow-up covering simultaneous starts, partitions, expiry, crash recovery and no exclusive-execution claim. Keep original issues in-mission until reviewed delivery; the root closes only after merge, with links. Do not close unrelated open work.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T025 — Complete independent readiness handoff

Provide a final acceptance matrix and compatibility/default-workflow evidence for root's mission review/accept gate. Resolve issue-matrix rows with actual terminal evidence when satisfied, retain review findings and exact commands. Commit owned files/evidence, mark subtasks and enter for_review; root handles mission merge and retrospective, not the implementer.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

## Validation and evidence

Before editing production code, write or select a contract-level failing test
and record the intended refusal/success outcome. Existing positive controls must
remain genuine; avoid mocks which bypass authorization or pipeline execution.

Use actual CLI/Git boundaries and finite timeouts. For skip/refusal behavior,
pre-create backend markers before invocation, snapshot relevant objects/refs/
private signer state, assert no change, and run an eligible positive control
after refusal assertions without resetting evidence.

Follow the explicit validation governance decision in plan.md and
.kittify/charter/charter.yaml: before review at each changed source, run focused
behavioral race regressions, native fast/default controls, gofmt, go vet ./...,
go build ./... and git diff --check. Record exact source, selections and exits;
any observed functional failure blocks approval. Independent review verifies
that evidence and runs meaningful focused controls without repeating unchanged
successful checks solely for handoff. Mission acceptance additionally requires
all affected integration/migration/protocol/runner/recovery/compatibility cases.
Exhaustive capacity/stress/transport-deadline/real-clock soak qualification stays
an explicit, separately tracked native release prerequisite. Preserve every
assertion, fixture and deadline. Never claim a scoped pass is exhaustive,
expand hosted deep corpora or mutate frozen signed fixtures.

Record commands, source head, test selection, exits and limitations in this WP's
evidence directory under its mission task folder. Independent reviewer reads
the fixed diff and runs meaningful checks. A failed negative control is not
evidence unless it fails for the promised reason.

## Safety and compatibility

- Application remains native Go/Git and existing sandbox/default authority.
- Do not enable policy, accounts, extensions, watchers or services implicitly.
- Withdrawals never rewrite prior signed history or invalidate earlier results.
- Keep contribution/moderation and trusted pipeline boundaries intact.
- Preserve unrelated original snapshot, 21 original stashes, old worktrees and
  all published/frozen release files. Stage specific owned files only.
- If a necessary file falls outside ownership, report it to root for a scoped
  planning amendment; do not change sibling WPs or sweep unrelated files.
- Optional framework hosted-sync warnings do not authorize login or changing
  shared configuration. Preserve local canonical events and record warnings.

## Review Feedback

If re-dispatched, read the canonical review artifact/event references first,
address every finding with causal evidence, and do not repeat a rejected
implementation unchanged. Never alter a verdict by editing rendered Markdown.

## Definition of Done

All listed subtasks are implemented or explicitly evidenced; new production
paths have real callers; declared checks pass at the committed head; original
workflow and signed evidence remain valid. Mark subtasks through
spec-kitty agent tasks mark-status, not checkboxes. Move only this WP to
for_review and poll the synchronous pre-review gate until it exits. Never
approve your own work, skip a failed guard, or move yourself to done.

## Reviewer guidance

Inspect behavior against spec, plan, contract and this WP's subtasks, including
no-effects and positive controls. Check final source identity and live call
sites; reject hidden fallback, weakened evidence/trust, fabricated pass claims
or an implicit default change. Approve/reject through canonical runtime events.
