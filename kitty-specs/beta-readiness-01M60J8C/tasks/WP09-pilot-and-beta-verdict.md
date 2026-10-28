---
work_package_id: WP09
title: External pilot and beta readiness verdict
dependencies:
- WP04
- WP05
- WP06
- WP07
- WP08
- WP13
requirement_refs:
- FR-013
- FR-014
- NFR-004
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T027
- T028
- T029
phase: Beta readiness
history: []
agent_profile: scribe-sally
authoritative_surface: docs/
create_intent:
- docs/beta-readiness.md
execution_mode: code_change
owned_files:
- docs/beta-readiness.md
role: documentarian
tags: []
task_type: implement
tracker_refs: []
---

# WP09 — External pilot and beta readiness verdict

## Objectives & Success Criteria

### T027

Prepare independent-adopter install/collaborate/migrate/upgrade/restore and
stranger-contribution/moderation tasks and collect actual pilot outcomes. A
stranger needs no upstream push right or provider account; ordinary issue/reply
admission may be automatic under explicit project rules. Native-only/offline
exchange, scoped moderation and remaining code merge authority must be visible.

### T028

Trace each support promise and success criterion to qualification evidence and defect disposition in docs/beta-readiness.md.

### T029

Record the truthful beta verdict and remaining blockers; prepare release recommendation without publishing or silently changing the support contract.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

External user tests cannot be replaced by the implementing agent replaying its own tests. Prepare a concrete brief before asking for participant coordination. Do not contact or post to others without explicit authorization. Missing required observations yield pending/not-ready, never an invented beta pass. Do not silently narrow platform/provider claims just to get a green verdict.

## Ownership and delivery

Owned surfaces: `docs/beta-readiness.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

All beta-supported combinations require observed evidence and no unresolved release-blocking defect. Mission accept/merge and final publication remain distinct; do not tag a beta from this WP.

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
