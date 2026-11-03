---
work_package_id: WP03
title: Signed requester withdrawal and portable dependency validation
dependencies:
- WP02
requirement_refs:
- C-001
- C-002
- FR-004
- FR-005
- NFR-003
planning_base_branch: fix/runner-efficiency
merge_target_branch: fix/runner-efficiency
branch_strategy: Computed mission workspace; consolidate to fix/runner-efficiency, then reviewed PR to main.
subtasks:
- T011
- T012
- T013
- T014
- T015
phase: Phase 2 - Native efficiency
history:
- at: '2026-11-03T20:32:32Z'
  actor: planner-priti
  action: Authored scoped runner-efficiency tasks after confirmed spec/plan.
agent_profile: implementer-ivan
authoritative_surface: event.go
create_intent:
- run_withdrawal.go
- run_withdrawal_protocol_test.go
execution_mode: code_change
owned_files:
- event.go
- store.go
- quarantine.go
- shallow.go
- project_snapshot.go
- run_withdrawal.go
- run_withdrawal_protocol_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP03: Signed requester withdrawal and portable dependency validation

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
Implementation command: spec-kitty agent action implement WP03 --mission runner-efficiency-01M6HH8H --agent codex --profile implementer-ivan.
Dependencies: WP02; approved dependencies unblock this work.

## Subtasks & Detailed Guidance

### T011 — Define failing signed withdrawal cases

Write test-first native facts covering valid request-owning actor withdrawal, wrong actor/key, wrong subject kind, missing request, sequence ordering, duplicate/conflicting withdrawals, extraneous fields, duplicate/unknown raw JSON keys and reason bounds. Preserve legacy fixture bytes and semantics.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T012 — Implement strict new-kind shape and relationship

Use kind run.withdraw, Subject=full request ID and Body=trimmed nonblank reason <=4096 UTF-8 bytes. Reuse standard signer metadata and shared validation helpers; do not add duplicate pipeline/head fields or relax old event readers. One relationship implementation validates original requester/key, earlier request sequence and duplicate/conflict refusal.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T013 — Wire local, quarantine and closure paths

Call the shared validator from actual local collection and quarantine dependency checks. Include withdrawal->request in replicationEventReferences and shallow scope handling. Missing dependencies return the existing recovery classification, not false absence or orphan acceptance. Demonstrate live callers through selected replication and snapshot validation.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T014 — Prove portability and unchanged historical evidence

Round-trip valid withdrawal/request/proposal dependencies through real Git sync and native archive export/import; malformed/unauthorized histories refuse before promotion. Check shallow/incomplete closures. Existing signed results/decisions/merges remain valid even when a later withdrawal exists; cancellation controls future starts only.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T015 — Validate source variants and hand off

Run focused protocol/replication/shallow/archive race tests in standard and native-only mode plus vet/build/gofmt/diff checks. Record exact source/commands/scope and no-effects evidence. Commit only owned files and this WP's evidence; mark subtasks and enter for_review, polling the gate.

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

Run gofmt on changed Go files, focused race tests, go vet ./..., go build ./...
and git diff --check. Use the native fast/default controls when the WP changes
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
