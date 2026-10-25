---
work_package_id: WP07
title: Native project and work-item commands
dependencies:
- WP06
requirement_refs:
- FR-014
- FR-016
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
subtasks:
- T031
- T032
- T033
- T034
history: []
agent_profile: implementer-ivan
authoritative_surface: project_commands.go
create_intent:
- project_commands.go
- project_commands_test.go
execution_mode: code_change
lane: planned
owned_files:
- project_commands.go
- project_commands_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

Use approved core engine/types, no provider imports or Event/admission edits. Register project specialization over generic engine through native handler registry. Project basic full-state --input JSON remains supported; convenience open --title/--body and item add --project ID --target issue:ID or --draft-title TEXT with explicit typed --input values are acceptable. Provide help with precise grammar. List/show JSON exposes state, revision IDs, conflicts, schema references. Editing fields/views uses guarded full-state revision; stable field/option IDs, no coercing invalid number/date/select. Item add captures exactly one project head and pins it; conflicted project refuses implicit selection. Item revisions may explicitly migrate schema with validation; historical field types remain pinned. List/board ordering deterministic by descriptive position+ID, not timestamp winner. All writes use engine actor/operation guards; native-only metadata preserved by convenience edits. Tests cover fields/views, drafts/external/native targets, stale/competing schema edits, item conflicts, idempotent replay and no GitHub/network dependency. Keep core engine shared helpers unchanged unless bug is reported to core WP; small documented integration API exposure can be coordinated.

## Subtasks

T031 Write public/native project lifecycle and typed item tests first.
T032 Implement project open/list/show/revise/resolve and field/view input UX over engine.
T033 Implement item add/show/list/revise/resolve/archive with exact schema pinning and target validation.
T034 Verify conflicts, source provenance preservation, actor replay and offline typed values.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.
