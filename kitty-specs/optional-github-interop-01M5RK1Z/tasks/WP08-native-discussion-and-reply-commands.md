---
work_package_id: WP08
title: Native discussion and reply commands
dependencies:
- WP06
requirement_refs:
- FR-013
- FR-016
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-github-interop-01M5RK1Z
base_commit: ea90761f5a98d0a5e0286970ff3913982107e537
created_at: '2026-10-25T05:14:14.429527+00:00'
subtasks:
- T035
- T036
- T037
- T038
history: []
agent_profile: implementer-ivan
authoritative_surface: discussion_commands.go
create_intent:
- discussion_commands.go
- discussion_commands_test.go
execution_mode: code_change
lane: planned
owned_files:
- discussion_commands.go
- discussion_commands_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

Use native engine and strict core states. Register discussion specialization, supporting basic generic full-state operations plus ergonomic open --title/--body/--category, reply add THREAD --body [--reply-to REPLY], reply edit ID --expect REV --body, answer THREAD --reply REV and clear answer. Accept exact reply REV, not moving root: later edits must not alter accepted content. Reply nesting <=8 and same thread, with no cycle/dangling crossrefs; conflicts visible and updates require exact head(s). Full-state resolve consumes all specified current heads. Category/open/closed are descriptive collaborative state; no implicit GitHub lock/moderator trust. Preserve source provenance/native metadata. Query pages contain attributed body/revision data, no identity needed. Tests cover actual engine signatures, threaded reply reconstruction, answer old-content pinning after edit, cross-thread rejection, stale/replay/actor changes and nogithub lifecycle. No provider-specific dependencies or new moderation ACL system.

## Subtasks

T035 Write thread/reply/answer/lifecycle and conflict tests first.
T036 Implement discussion open/list/show/revise/resolve/close/reopen/category operations.
T037 Implement reply add/edit/resolve and explicit accepted-answer revision selection/clear.
T038 Verify thread bounds, same-thread refs, pinned answer content, offline identity/replay and metadata preservation.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.
