---
work_package_id: WP08
title: Independent beta boundary review
dependencies:
- WP04
- WP07
requirement_refs:
- FR-012
- NFR-002
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T024
- T025
- T026
phase: Beta readiness
history: []
agent_profile: reviewer-renata
authoritative_surface: docs/
create_intent:
- docs/beta-boundary-review.md
execution_mode: code_change
owned_files:
- docs/beta-boundary-review.md
role: reviewer
tags: []
task_type: review
tracker_refs: []
---

# WP08 — Independent beta boundary review

## Objectives & Success Criteria

### T024

Independently review credential/key handling, imports, filesystem boundaries, authority and runner isolation against the prospective beta contract.

### T025

Record actionable findings and nonfindings with evidence in docs/beta-boundary-review.md; identify required repair WPs.

### T026

Recheck disposition of release-blocking findings against reviewed fixes without claiming hardened multi-tenant isolation.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

This WP is an independent review artifact, not permission for the reviewer to edit production code. Root must add narrowly owned repair WPs for actionable findings and route them through implementation/review. Preserve existing narrow threat model and optional unsafe-host warnings. Review signed historical evidence without modifying it.

## Ownership and delivery

Owned surfaces: `docs/beta-boundary-review.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Review exact source, trust boundaries and public failure paths; use bounded probes where justified. A checklist without investigation is not review evidence. Unresolved release-blocking findings prevent qualification.

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

