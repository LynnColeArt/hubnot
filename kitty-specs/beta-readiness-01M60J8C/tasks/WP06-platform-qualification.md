---
work_package_id: WP06
title: Candidate package and distribution qualification
dependencies:
- WP03
requirement_refs:
- FR-010
- NFR-004
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T018
- T019
- T020
phase: Beta readiness
history: []
agent_profile: scribe-sally
authoritative_surface: docs/
create_intent:
- docs/platform-qualification.md
execution_mode: code_change
owned_files:
- docs/platform-qualification.md
role: documentarian
tags: []
task_type: implement
tracker_refs: []
---

# WP06 — Candidate package and distribution qualification

## Objectives & Success Criteria

### T018

Define exact-package Linux/macOS architecture/variant installation and runtime cases, supported Git/filesystem configurations and download experience in docs/platform-qualification.md.

### T019

Run available candidate package cases using existing Go-only packaging and public package acceptance; collect actual platform/binary evidence.

### T020

Record outstanding Intel/Mac exact-byte work and a signing/notarization recommendation with owner decision prerequisites.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Do not relabel Go 1.26.6 native Mac rebuilds as execution of Go 1.26.4 published binaries. Reuse test-only package-dir/version harness flags and existing CI gates. Linux ARM host lynn@192.168.1.145 has prior explicit authorization for package testing, but verify reachability and use disposable paths. Do not message another Codex or external person without explicit authorization. No release/tag publication is implied.

## Ownership and delivery

Owned surfaces: `docs/platform-qualification.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Each claimed OS/architecture/variant gets observed normal-environment package evidence. Missing hardware is pending, not a pass. Inspect archive digests/BUILD provenance independently of runtime qualification.

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

