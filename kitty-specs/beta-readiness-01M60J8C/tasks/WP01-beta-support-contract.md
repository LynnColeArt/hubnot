---
work_package_id: WP01
title: Beta support contract and current release status
dependencies: []
requirement_refs:
- C-001
- C-002
- C-004
- C-005
- C-006
- FR-001
- FR-002
- FR-014
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
phase: Beta readiness
history: []
agent_profile: scribe-sally
authoritative_surface: docs/
create_intent:
- docs/beta-support.md
execution_mode: code_change
owned_files:
- docs/beta-support.md
- README.md
- CHANGELOG.md
role: documentarian
tags: []
task_type: implement
tracker_refs: []
---

# WP01 — Beta support contract and current release status

## Objectives & Success Criteria

### T001

Write docs/beta-support.md with prospective stable surfaces, experimental boundaries, candidate platform/Git matrix, version evolution, rollback and release gates.

### T002

Update README.md and CHANGELOG.md for published 0.0.3-dev, all eleven download links and exact Mac qualification distinctions; retain previous release history and immutable assets.

### T003

Check the support promise against existing commands/contracts and published release metadata; record remaining evidence obligations and independent doc/link validation.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Do not call the current alpha beta. Define beta expectations prospectively, with current qualification distinguished. Preserve standard/native-only selection, Git-only runtime, Go/Git-only packaging and independently disabled opt-ins. No broad promise that every Git 2.x, filesystem, forge or platform combination works. Current source support can be wider than qualified beta package support. Do not introduce hard SHA pinning of normal configuration or a new mandatory CI policy. Current release notes at https://github.com/LynnColeArt/hubnot/releases/tag/v0.0.3-dev are authoritative for package testing: Linux AMD64/ARM64 actual packages passed; Go 1.26.6 Apple Silicon native rebuilds passed; exact downloadable Go 1.26.4 Mac binaries and Intel execution remain pending. The prior Mac go-ahead was sufficient to authorize publication; do not reinstate a publication hold. Existing detailed docs remain canonical; link rather than duplicate format definitions.

## Ownership and delivery

Owned surfaces: `docs/beta-support.md`, `README.md`, `CHANGELOG.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Documentation-only: verify each new download URL against the published asset list, check all changed local Markdown links, inspect commands against help/implementation, git diff --check. Do not run the full expensive Go suite for prose-only edits. No external writes, issue comments or release publication.

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

