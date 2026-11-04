---
work_package_id: WP04
title: Withdrawal CLI, discovery and queued execution controls
dependencies:
- WP03
requirement_refs:
- FR-001
- FR-004
- FR-006
- FR-008
- NFR-003
planning_base_branch: fix/runner-efficiency
merge_target_branch: fix/runner-efficiency
branch_strategy: Planning artifacts for this mission were generated on fix/runner-efficiency. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/runner-efficiency unless the human explicitly redirects the landing branch.
subtasks:
- T016
- T017
- T018
- T019
- T020
phase: Phase 2 - Native efficiency
history:
- at: '2026-11-03T20:32:32Z'
  actor: planner-priti
  action: Authored scoped runner-efficiency tasks after confirmed spec/plan.
agent_profile: implementer-ivan
authoritative_surface: ci.go
create_intent:
- run_withdrawal_cli.go
- run_withdrawal_cli_test.go
execution_mode: code_change
owned_files:
- ci.go
- runner.go
- main.go
- contribution_moderation.go
- run_withdrawal_cli.go
- run_withdrawal_cli_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP04: Withdrawal CLI, discovery and queued execution controls

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
Implementation command: spec-kitty agent action implement WP04 --mission runner-efficiency-01M6HH8H --agent codex --profile implementer-ivan.
Dependencies: WP03; approved dependencies unblock this work.

## Subtasks & Detailed Guidance

### T016 — Specify public commands and retry behavior

Use actual CLI tests for hn run withdraw FULL_REQUEST --reason TEXT, run help and run capabilities --json. Reuse existing test binary fixtures. Capability/help reads require no signer and cause no network/signing/backend effects. Withdrawal output binds exact request/event and reminds users publication needs normal sync.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T017 — Implement requester-only idempotent command

Validate complete history and requester authority before constructing/signing a withdrawal. Exact normalized request/reason retry returns the original withdrawal without a new event. A changed reason or different actor refuses without changing signer/object/ref state. Do not infer success from content matching on a different intention.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T018 — Expose withdrawal and preserve moderation

Extend run list/show with exact withdrawal identity/status and update root usage. Handle static help/capabilities as keyless reads under existing contribution boundary. Preserve the existing prohibition on running the runner under enabled contribution moderation; do not create a new privileged cancellation role.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T019 — Enforce future-start boundary

Check observed withdrawal before both cmdRunExecute and runner backend entry, using the shared projection. A withdrawn request cannot be forced by a new actor. Preserve already-recorded results and merge evidence. A newly signed distinct request may be explicitly executed; automatic threshold suppression may still skip that new request if existing evidence suffices.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T020 — Validate sync, retries and ordinary workflow

Exercise withdrawal on one clone then sync to a second runner; assert no backend marker or result append. Include same-actor continuation, repeated command after restart, changed reason and wrong signer refusal. Verify default workflow, ungoverned runs and static discovery. Run focused race/static/diff gates, commit and enter for_review.

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
