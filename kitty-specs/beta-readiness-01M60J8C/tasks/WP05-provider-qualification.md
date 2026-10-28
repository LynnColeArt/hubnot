---
work_package_id: WP05
title: Live provider migration qualification
dependencies:
- WP03
requirement_refs:
- FR-009
- NFR-004
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T015
- T016
- T017
phase: Beta readiness
history: []
agent_profile: scribe-sally
authoritative_surface: docs/
create_intent:
- docs/provider-qualification.md
execution_mode: code_change
owned_files:
- docs/provider-qualification.md
role: documentarian
tags: []
task_type: implement
tracker_refs: []
---

# WP05 — Live provider migration qualification

## Objectives & Success Criteria

### T015

Define disposable live provider/resource/direction cases and provider-to-local-to-provider mapping/loss comparisons in docs/provider-qualification.md.

### T016

Execute available authorized live cases, recording account type/API profile/permissions/rate-limit/retry observations and preserving remote edits.

### T017

Record unexecuted cases with concrete owner prerequisites and classify candidate beta support without claiming fake-server results as live proof.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Provider API compatibility is separate from custom-ref Git hosting. Reuse supported profiles in docs/github-interoperability.md. Do not assume credentials or permission to mutate unrelated projects. Prepare concrete disposable-project plans before requesting missing access. No unsupported recreation should be counted as successful lossless transfer. Preserve full local continuation without a provider.

## Ownership and delivery

Owned surfaces: `docs/provider-qualification.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Record exact scenario/environment/outcome/evidence for every claimed live pass. Missing live access remains an unmet qualification obligation; completing a brief alone does not complete T016.

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
