---
work_package_id: WP08
title: Forgejo/Codeberg and Gitea migration adapters
dependencies:
- WP06
requirement_refs:
- FR-004
- FR-007
- FR-008
- FR-010
- FR-013
- FR-014
- NFR-001
- NFR-002
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0
base_commit: 946a72d8c76836814a349d71323f2baf41403ebd
created_at: '2026-10-27T02:40:51.786055+00:00'
subtasks:
- T022
- T023
- T024
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: internal/forgejoapi/
create_intent:
- internal/forgejoapi/client.go
- internal/forgejoapi/client_test.go
- internal/giteaapi/client.go
- internal/giteaapi/client_test.go
- migration_forgejo.go
- migration_forgejo_test.go
- migration_gitea.go
- migration_gitea_test.go
- forge_forgejo.go
- forge_forgejo_test.go
- forge_gitea.go
- forge_gitea_test.go
execution_mode: code_change
owned_files:
- internal/forgejoapi/client.go
- internal/forgejoapi/client_test.go
- internal/giteaapi/client.go
- internal/giteaapi/client_test.go
- migration_forgejo.go
- migration_forgejo_test.go
- migration_gitea.go
- migration_gitea_test.go
- forge_forgejo.go
- forge_forgejo_test.go
- forge_gitea.go
- forge_gitea_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP08 — Forgejo/Codeberg and Gitea migration adapters

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Support separately tested Forgejo/Codeberg and Gitea profiles with issue/comment read-write, preserved PR reviews/assets and explicit capability/version differences.

**Independent acceptance**: Distinct Forgejo and Gitea fixtures pass shared adapter cases and their own endpoint/version/asset semantics; unavailable board APIs refuse or preserve source explicitly instead of claiming native project export.

**Requirements**: FR-004, FR-007, FR-008, FR-010, FR-013, FR-014, NFR-001, NFR-002.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

Prerequisites: WP06 must be approved; base the lane on the runtime's dependency computation.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T022 — Define distinct profile and issue/comment fixtures

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Require actual issue lifecycle/comments read-write in both profiles. Cover explicit /api/v1 roots, immutable IDs versus local indexes, issue lists containing PRs, token scopes, configurable paging and separately versioned schema expectations.
- **Files**: `internal/forgejoapi/client.go`, `internal/forgejoapi/client_test.go`, `internal/giteaapi/client.go`, `internal/giteaapi/client_test.go`, `migration_forgejo.go`, `migration_forgejo_test.go`, `migration_gitea.go`, `migration_gitea_test.go`, `forge_forgejo.go`, `forge_forgejo_test.go`, `forge_gitea.go`, `forge_gitea_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T023 — Implement isolated adapters with truthful resource coverage

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Apply `//go:build !noforge` to network-only registration/client/test files; self-register disjoint provider files. Share only proven transport helpers without equating all API semantics. Preserve PR/review conversations, selected issue/comment assets and selected workflow source; validate any board capability against its tested API rather than UI availability.
- **Files**: `internal/forgejoapi/client.go`, `internal/forgejoapi/client_test.go`, `internal/giteaapi/client.go`, `internal/giteaapi/client_test.go`, `migration_forgejo.go`, `migration_forgejo_test.go`, `migration_gitea.go`, `migration_gitea_test.go`, `forge_forgejo.go`, `forge_forgejo_test.go`, `forge_gitea.go`, `forge_gitea_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T024 — Verify each profile independently

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Run conformance (including issue-create-to-comment confirmed receipt resolution where applicable) and hostile pagination/redirect/asset/recovery cases for both. Prove native-only operation after import, no automatic workflow execution or permission mapping, and positive reconciliation/manual intervention after lost writes.
- **Files**: `internal/forgejoapi/client.go`, `internal/forgejoapi/client_test.go`, `internal/giteaapi/client.go`, `internal/giteaapi/client_test.go`, `migration_forgejo.go`, `migration_forgejo_test.go`, `migration_gitea.go`, `migration_gitea_test.go`, `forge_forgejo.go`, `forge_forgejo_test.go`, `forge_gitea.go`, `forge_gitea_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Forgejo promises compatibility within a major, not with all Gitea releases. Codeberg instance capabilities and API availability are not universal. Shared code must not erase separate identities or inflate the advertised resource/direction matrix.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-004, FR-007, FR-008, FR-010, FR-013, FR-014, NFR-001, NFR-002 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T022 T023 T024 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

