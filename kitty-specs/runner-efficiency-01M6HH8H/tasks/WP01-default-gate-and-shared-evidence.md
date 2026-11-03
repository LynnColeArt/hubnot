---
work_package_id: WP01
title: Default workflow gate and shared pipeline evidence
dependencies: []
requirement_refs:
- C-001
- C-002
- C-003
- FR-001
- FR-003
planning_base_branch: fix/runner-efficiency
merge_target_branch: fix/runner-efficiency
branch_strategy: Planning artifacts for this mission were generated on fix/runner-efficiency. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/runner-efficiency unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
- T005
phase: Phase 1 - Default gate
history:
- at: '2026-11-03T20:32:32Z'
  actor: planner-priti
  action: Authored scoped runner-efficiency tasks after confirmed spec/plan.
agent_profile: implementer-ivan
authoritative_surface: policy.go
create_intent: []
execution_mode: code_change
owned_files:
- policy.go
- policy_test.go
- ci_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP01: Default workflow gate and shared pipeline evidence

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
Implementation command: spec-kitty agent action implement WP01 --mission runner-efficiency-01M6HH8H --agent codex --profile implementer-ivan.
Dependencies: none; approved dependencies unblock this work.

## Subtasks & Detailed Guidance

### T001 — Establish the default-workflow baseline

Run the existing native fast action before code changes and retain exact head/commands/exits. In particular TestDistributedPipelineRun must still execute with policy.Pipelines empty. Existing identity/issue/native workflow, keyless reads and sandbox opt-in behavior are the first gate; do not create new default policy, services, extension registrations or accounts.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T002 — Extract canonical pipeline result evaluation

Read evaluateProposal's full pipeline loop and its callers. Extract a narrow shared evaluator preserving current request authorization, exact proposal/pipeline/definition matching, latest-per-runner selection, trust policy, pass/fail ordering and required count. Reuse it from evaluateProposal; never change how existing decisions or merges interpret evidence.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T003 — Prove unchanged governance semantics

Run focused existing policy/requester/CI governance cases. Add a contract test only for a meaningful uncovered behavior, using observable evaluation outcomes rather than asserting internal helper structure. Include latest pass replaced by failure, matching evidence across requests, and untrusted or mismatched evidence not counted.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T004 — Verify boundary and document evidence

Demonstrate the new helper is called from live evaluation, not merely tested in isolation. Keep old signed fixtures byte-identical. Run race-focused validation, gofmt, go vet, go build and diff checks; stage only owned implementation/evidence files.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T005 — Hand off for independent review

Record exact baseline and final head, test scope/exits and any limits in the WP evidence. Mark only this WP's subtasks done through the CLI and move to for_review, polling the transition gate to completion. Do not approve your own implementation.

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

Run gofmt on changed Go files, focused race tests, the charter's full source
race gate (go test -race -timeout 20m ./...), go vet ./..., go build ./...
and git diff --check before review at each changed head. Keep that full gate
native/local, never add it to the bounded hosted workflow. Do not repeat a
successful unchanged full run; the independent reviewer verifies its exact-head
evidence and runs meaningful focused controls. Use native fast/default controls when the WP changes
their boundary. Do not launch hosted deep corpora, loosen lint/test assertions,
claim an unexecuted full suite or mutate frozen signed fixtures.

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
