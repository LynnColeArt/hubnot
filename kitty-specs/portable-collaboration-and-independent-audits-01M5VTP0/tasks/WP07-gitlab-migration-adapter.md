---
work_package_id: WP07
title: GitLab migration and issue/comment transfer
dependencies:
- WP06
requirement_refs:
- FR-004
- FR-007
- FR-008
- FR-009
- FR-013
- FR-014
- NFR-001
- NFR-002
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
subtasks:
- T019
- T020
- T021
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: internal/gitlabapi/
create_intent:
- internal/gitlabapi/client.go
- internal/gitlabapi/client_test.go
- internal/gitlabapi/migration.go
- internal/gitlabapi/migration_test.go
- migration_gitlab.go
- migration_gitlab_test.go
- forge_gitlab.go
- forge_gitlab_test.go
execution_mode: code_change
owned_files:
- internal/gitlabapi/client.go
- internal/gitlabapi/client_test.go
- internal/gitlabapi/migration.go
- internal/gitlabapi/migration_test.go
- migration_gitlab.go
- migration_gitlab_test.go
- forge_gitlab.go
- forge_gitlab_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP07 — GitLab migration and issue/comment transfer

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Deliver non-vacuous GitLab Cloud/self-hosted capabilities with issue/comment read-write and attributed project communications preservation.

**Independent acceptance**: Versioned local GitLab fixtures exercise bounded issue/comment read/create/update, global IDs vs project-local IIDs, attached discussion topology, selected uploads/workflow source and uncertain write recovery; capabilities expose planning/export limits honestly.

**Requirements**: FR-004, FR-007, FR-008, FR-009, FR-013, FR-014, NFR-001, NFR-002.

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

### T019 — Write GitLab capability and API contract fixtures

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Require working issue lifecycle/comment reads and writes, not all-unsupported registration. Cover explicit /api/v4 self-hosted roots, encoded namespace selectors, ID/IID distinction, note versus threaded/system-note semantics, boards versus repository projects and attachment authorization.
- **Files**: `internal/gitlabapi/client.go`, `internal/gitlabapi/client_test.go`, `internal/gitlabapi/migration.go`, `internal/gitlabapi/migration_test.go`, `migration_gitlab.go`, `migration_gitlab_test.go`, `forge_gitlab.go`, `forge_gitlab_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T020 — Implement the isolated Go GitLab adapter

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Apply `//go:build !noforge` to network-only registration/client/test files and use explicit credentials and shared migration contracts; preserve selected PR/MR communications, retrieved attachments, board observations and CI source without inventing native equivalence. Register applicable legacy `hn transfer` issue methods through the existing provider seam as well as the new migration contract. Route supported writes through common ApplyContext/confirmed-receipt guarded intent/progress; report feature-tier or inaccessible content before writes.
- **Files**: `internal/gitlabapi/client.go`, `internal/gitlabapi/client_test.go`, `internal/gitlabapi/migration.go`, `internal/gitlabapi/migration_test.go`, `migration_gitlab.go`, `migration_gitlab_test.go`, `forge_gitlab.go`, `forge_gitlab_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T021 — Run conformance, fidelity and lost-response tests

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Prove complete bounded pagination, stable attribution/cross-links, no native trust grants, endpoint/redirect credential isolation and no blind create retry. Provide fixture functions usable by WP11's GitLab-to-GitHub public migration scenario.
- **Files**: `internal/gitlabapi/client.go`, `internal/gitlabapi/client_test.go`, `internal/gitlabapi/migration.go`, `internal/gitlabapi/migration_test.go`, `migration_gitlab.go`, `migration_gitlab_test.go`, `forge_gitlab.go`, `forge_gitlab_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

GitLab Discussions belong to issues/MRs and are not standalone Hubnot discussions. Namespaced paths can change; base identities and write selectors must remain distinct. Missing license/permission is not a complete empty collection.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-004, FR-007, FR-008, FR-009, FR-013, FR-014, NFR-001, NFR-002 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T019 T020 T021 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

