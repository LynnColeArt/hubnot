---
work_package_id: WP04
title: Journal compatibility and interrupted-operation recovery
dependencies:
- WP03
requirement_refs:
- FR-002
- FR-008
- NFR-001
- NFR-002
- NFR-005
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T012
- T013
- T014
phase: Beta readiness
history: []
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- beta_journal_recovery_test.go
- docs/operation-recovery.md
execution_mode: code_change
owned_files:
- beta_journal_recovery_test.go
- docs/operation-recovery.md
- identity_continuity.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP04 — Journal compatibility and interrupted-operation recovery

## Objectives & Success Criteria

### T012

Inventory originating-binary/session-version upgrade rules and explicit resume/reconcile commands in docs/operation-recovery.md.

### T013

Add missing observable recovery assertions for supported migration/transfer/audit/rotation/merge interruption boundaries, reusing existing tests and seams.

### T014

Prove unknown or unsafe journals refuse without resetting progress, and uncertain remote writes never trigger duplicate creation; validate charter gates.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Do not duplicate the existing comprehensive fault suites or force every historical alpha journal into a migration promise. Derive gaps from the compatibility/support contract and existing tests. Retaining the originating compatible binary to complete a session is an acceptable documented boundary when explicitly tested. Do not delete uncertainty checkpoints or use a negative lookup as authorization. Never fabricate a remote successful outcome or silently reinterpret a destination binding. If production changes are needed, obtain an updated ownership assignment from root after reporting the real failing contract.

## Ownership and delivery

Owned surfaces: `beta_journal_recovery_test.go`, `docs/operation-recovery.md`,
`identity_continuity.go`.

Concrete recovery obligation from read-only inspection: `prepareIdentityRotation`
can write reconciled progress to `rotation-command.json` before the caller
validates the referenced signed events, and can recreate a missing target record
before rejecting an unsafe or unsupported `rotation.json`. First reproduce
through the public rotate command with a valid empty-progress command journal
and matching rotation state that names a well-formed nonexistent event. Compare
complete private state, refs and objects on refusal. Also combine missing target
material with unsupported/unsafe saved rotation state. A test seeding identical
bad progress in both journals does not cover reconciliation-before-validation.
Repair only after a meaningful red result, validate saved state and signed
progress before recovery writes, and preserve the existing successful crash
recovery, exact-target material and active-pointer guarantees. The new ownership
is sequential and disjoint from other declared production surfaces; it does not
authorize format changes, fabricated progress or resetting uncertain journals.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Run targeted tests for actual gaps, then applicable full charter gates for any Go change. Documentation-only inventory must not masquerade as executed recovery evidence.

Meaningful tests assert the promised observable contract and must detect the
claimed fault; never weaken refusal assertions or add xfails to get a pass.

## Review Guidance

Check requirement coverage, truthful qualification, native independence,
disabled defaults, immutable history, private-state handling and the distinction
between provenance and authority. Review the fixed committed head. Capture a
canonical verdict through the framework, not only a prose message.

## Handoff

Commit only expected files. Record test commands, exits, evidence and remaining
obligations. Mark only this WP's subtasks done when truly completed. Move to
`for_review` with the framework and wait for the synchronous gate to finish;
do not kill/retry an in-flight gate or claim a skipped/no-coverage gate ran tests.

## Activity Log

- 2026-10-28T06:21:00Z – codex – Work package authored from the approved mission intent.
