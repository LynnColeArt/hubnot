---
work_package_id: WP01
title: Local gh-style issue commands
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-009
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-github-interop-01M5RK1Z
base_commit: ea90761f5a98d0a5e0286970ff3913982107e537
created_at: '2026-10-25T04:02:18.288790+00:00'
subtasks:
- T001
- T002
- T003
- T004
history: []
agent_profile: implementer-ivan
authoritative_surface: gh_compat.go
create_intent:
- gh_compat.go
- gh_compat_test.go
execution_mode: code_change
lane: planned
owned_files:
- gh_compat.go
- gh_compat_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, this mission's spec.md and plan.md. Load implementer-ivan through the canonical profile-load skill and `spec-kitty agent profile show implementer-ivan --json`. Work only in the runtime-provided lane workspace. Use the existing native issue query/mutation and validation functions; never bypass event verification or alter signed wire schemas. Write behavior tests before implementation. This mission is explicitly authorized to completion; do not ask the user for repetitive permission. Independent review is required: do not approve your own work.

## Runtime and delivery

Use `spec-kitty next --agent <your-agent-name> --mission optional-github-interop-01M5RK1Z --json` and the generated action context. Implementation is a separate agent from review. Commit only owned source/tests/docs (and required task evidence), not unrelated generated state. Record subtasks with mark-status. Move to for_review only after relevant gates; if a synchronous gate remains running, poll it to completion rather than kill/retry. Return exact commit, commands/results, limitations and workspace. Do not merge to public main or publish GitHub issues.

## Global invariants

Core Hubnot and all local compatibility commands work offline with no credentials or gh executable. Explicit GitHub transport is standard-library only and excluded by the nogithub build tag. Imported facts use the native signer; external attribution remains source metadata. No implicit sync, timestamps-as-winner, shell execution of ticket text, or unsupported silent semantics. Existing 256 KiB native bounds apply, including 64 KiB body and namespaced metadata value <=4096 bytes. No credentials in logs/signed metadata/checkpoints. All list/history readers consume complete bounded pages, and stale snapshot errors refuse partial answers.

## Objective and implementation contract

Implement `cmdGH(args []string) error` called with arguments after hn gh. Support `issue create --title/-t --body/-b [--label/-l repeated/comma] [--assignee/-a repeated/comma]`, `issue list [--state/-s open|closed|all] [--label/-l ...] [--assignee/-a ...] [--limit/-L N] [--json FIELDS]`, `issue view ID [--comments] [--json FIELDS]`, `issue edit ID [--title/-t] [--body/-b] [--add-label ...] [--remove-label ...] [--add-assignee ...] [--remove-assignee ...]`, `issue comment ID --body/-b TEXT`, `issue close|reopen ID`. Help documents exact subset and native full IDs; no invented stable numeric issue IDs. Support useful common flags only, rejecting any unsupported command/flag, JSON field or selector rather than ignoring. Help succeeds outside a repo; reads need no identity. Keep compatibility error handling plain nonzero unless documented.

Use selected JSON fields with gh-like raw objects/arrays (not the native envelope): id,title,body,state,labels,assignees,author,comments (where meaningful), url only if truthful; define shapes in tests/docs and reject unsupported fields. State should match gh convention OPEN/CLOSED for output and open/closed/all for filtering. Labels/assignees are descriptive native values, actor author stays explicit. For native conflicting state, show conflict clearly and refuse edits/lifecycle choosing a winner. Preserve native-only fields and use exact observed head for revise. All writes discover/capture native signer then pin it through the mutation. Optional --operation can reuse native retry semantics; don't invent weaker guarantees. Pagination should yield up to --limit without silently treating one page as entire repository.

Tests may call cmdGH directly at this stage and use existing fixture/output-capture helpers. Do NOT edit main.go; WP03 will wire both top-level routes after your implementation lands. No GitHub network imports/references required for local commands. Run focused tests, go vet/build, gofmt and diff checks, with final repository race gate before review.

## Subtasks

T001 Define local compatibility lifecycle and JSON/filter behavior tests first.
T002 Implement cmdGH and strict interspersed long/short flag parsing for the documented issue subset.
T003 Translate supported reads/mutations through existing native issue functions and validation with actor pinning and conflict refusal.
T004 Validate offline behavior, precise errors, script JSON fields and preservation of existing native state.

## Review criteria

Behavior matches every referenced requirement; scoped and full required gates pass; no unrelated files, private data or binaries staged. Unsupported behavior fails explicitly. Reviewer independently probes hostile/failure cases and the standalone boundary.
