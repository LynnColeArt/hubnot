---
work_package_id: WP07
title: Practical workload and recovery limits
dependencies:
- WP03
requirement_refs:
- FR-011
- NFR-004
- NFR-005
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T021
- T022
- T023
phase: Beta readiness
history: []
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- beta_scale_test.go
- docs/operating-limits.md
execution_mode: code_change
owned_files:
- beta_scale_test.go
- docs/operating-limits.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP07 — Practical workload and recovery limits

## Objectives & Success Criteria

### T021

Select representative actor/issue/discussion/project/memory history sizes and bounded migration/recovery workloads from the contract.

### T022

Add reproducible benchmark/scenario coverage in beta_scale_test.go and record resource/latency/recovery measurements.

### T023

Publish measured supported envelopes and budget exhaustion behavior in docs/operating-limits.md without promising unmeasured universal thresholds.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Do not turn hardware-specific timings into brittle CI gates. Bound test fixtures and distinguish microbenchmarks from practical workflows. Prefer a small useful ladder of workloads to an expensive synthetic scale system. No performance optimization may relax signature/admission validation.

## Ownership and delivery

Owned surfaces: `beta_scale_test.go`, `docs/operating-limits.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Report source, hardware, fixture dimensions, commands and measured results. Keep normal tests reasonably bounded; expensive workloads require explicit benchmark/scenario invocation. Go changes receive charter gates.

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
