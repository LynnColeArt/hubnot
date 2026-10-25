---
work_package_id: WP11
title: Optional GitHub discussion migration
dependencies:
- WP08
- WP10
requirement_refs:
- FR-004
- FR-008
- FR-011
- FR-012
- FR-013
- FR-017
- FR-020
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
subtasks:
- T048
- T049
- T050
- T051
- T052
history: []
agent_profile: implementer-ivan
authoritative_surface: github_discussions.go
create_intent:
- github_discussions.go
- github_discussions_test.go
- internal/githubapi/discussions.go
- internal/githubapi/discussions_test.go
- internal/githubapi/graphql.go
- internal/githubapi/graphql_test.go
execution_mode: code_change
lane: planned
owned_files:
- github_discussions.go
- github_discussions_test.go
- internal/githubapi/discussions.go
- internal/githubapi/discussions_test.go
- internal/githubapi/graphql.go
- internal/githubapi/graphql_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

All GitHub-specific files/tests !nogithub. Use approved neutral bridge, native types and existing REST client's security/limits. Add reusable GraphQL transport in internal/githubapi/graphql.go: GitHub.com /graphql, enterprise /api/graphql from validated REST base /api/v3 (not /api/v3/graphql); optional explicit safe endpoint only if documented. Auth required for GraphQL. Same-origin/no redirects, <=30s/request,4MiB response,100 pages/connection,64MiB aggregate,5min transfer context. Reject errors and partial data even HTTP200, malformed/trailing/duplicate/invalid Unicode; sanitize all diagnostics. Variables only, no interpolated query data. clientMutationId is correlation, not exactly-once.

Official operations: repository.discussion(number), discussions, discussionCategories; nested comments/replies paginated independently with first<=100. Read stable global IDs, title/body/author/times/category,state,answer, reply-to, source lock/poll/reaction limitations. Mutations createDiscussion/updateDiscussion/addDiscussionComment(replyToId)/updateDiscussionComment/mark-unmark answer/close-reopen. Existing destination category must be explicitly mapped or exact supported matching reported; no category-create API claim. Preserve original source attribution as claims. Core categories/closed state do not grant imported lock/moderator authority. Unsupported polls/votes/hidden/deleted inaccessible content or lock semantics need explicit report/manual decision before destructive/lossy migration; do not impersonate authors.

Import whole selected thread or bounded listing with independently revisioned native replies, parent/answer mapping, source metadata, local/remote baseline conflicts. Export writes root then replies topologically, then answer/state; exact native accepted revision maps to correct external comment content. Every mutation uses common prepared/in-flight/confirmed checkpoint and exact positive reconciliation; no blind retry/duplicates. Source-origin replies suppressed on exact same-source return, no footer echo; new markers parsed terminal+schema/digest exactly, ordinary prose preserved. Tests fake GraphQL + real native signed resources, nested pagination beyond page1, category missing before write, cross-thread refs, lost create/reply response, repeat/competing edits, answer pin, token redaction and transport-free builds. No real GitHub mutations.

## Subtasks

T048 Add bounded sanitized GraphQL transport and complete nested pagination tests.
T049 Implement discussion/category/reply/answer/state reads and typed mutations via optional adapter.
T050 Map native threads/replies/answer revisions and require explicit destination category mapping.
T051 Implement repeatable journaled discussion export/import, conflicts and uncertain mutation recovery.
T052 Verify no implicit moderation authority, no lossy hidden fields, and nogithub exclusion.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.
