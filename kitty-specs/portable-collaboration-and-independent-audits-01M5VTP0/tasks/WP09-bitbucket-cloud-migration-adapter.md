---
work_package_id: WP09
title: Bitbucket Cloud repository and PR-conversation migration
dependencies:
- WP06
requirement_refs:
- FR-004
- FR-007
- FR-008
- FR-011
- FR-013
- FR-014
- NFR-001
- NFR-002
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
subtasks:
- T025
- T026
- T027
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: internal/bitbucketapi/
create_intent:
- internal/bitbucketapi/client.go
- internal/bitbucketapi/client_test.go
- internal/bitbucketapi/migration.go
- internal/bitbucketapi/migration_test.go
- migration_bitbucket.go
- migration_bitbucket_test.go
execution_mode: code_change
owned_files:
- internal/bitbucketapi/client.go
- internal/bitbucketapi/client_test.go
- internal/bitbucketapi/migration.go
- internal/bitbucketapi/migration_test.go
- migration_bitbucket.go
- migration_bitbucket_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP09 — Bitbucket Cloud repository and PR-conversation migration

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Provide meaningful Bitbucket Cloud repository/PR-conversation/workflow-source support while explicitly refusing its removed native issue tracker.

**Independent acceptance**: Cloud fixtures preserve repository identity, PR activity/comments/inline-parent topology and pipeline source; issue selection fails before writes, opaque pagination is bounded, and no Jira or Data Center endpoint is silently substituted.

**Requirements**: FR-004, FR-007, FR-008, FR-011, FR-013, FR-014, NFR-001, NFR-002.

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

### T025 — Specify currently available Bitbucket Cloud capabilities

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Require PR-conversation snapshot coverage with repository UUIDs separate from workspace/slug selectors. Assert Issue Tracker unavailable, because its API was removed in August 2026. Cover current scoped API-token/OAuth authorization, not retired app passwords.
- **Files**: `internal/bitbucketapi/client.go`, `internal/bitbucketapi/client_test.go`, `internal/bitbucketapi/migration.go`, `internal/bitbucketapi/migration_test.go`, `migration_bitbucket.go`, `migration_bitbucket_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T026 — Implement Cloud snapshot and applicable destination operations

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Apply `//go:build !noforge` to network-only registration/client/test files. Use /2.0 repository/PR APIs and preserve available workflow source inertly. Represent uncreatable resources as explicit preservation-only or blocking findings. Do not invent generic issue CRUD, silently redirect to Jira, or pretend arbitrary self-hosted Data Center is Cloud-compatible.
- **Files**: `internal/bitbucketapi/client.go`, `internal/bitbucketapi/client_test.go`, `internal/bitbucketapi/migration.go`, `internal/bitbucketapi/migration_test.go`, `migration_bitbucket.go`, `migration_bitbucket_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T027 — Prove opaque pagination, provenance and unsupported behavior

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Follow valid same-origin next links without constructing assumed page numbers; reject loops, missing selected content, oversize assets and cross-origin credential forwarding. Test clear capability/loss reports and common ApplyContext/confirmed-receipt guarded write/reconciliation for any supported destination mutations.
- **Files**: `internal/bitbucketapi/client.go`, `internal/bitbucketapi/client_test.go`, `internal/bitbucketapi/migration.go`, `internal/bitbucketapi/migration_test.go`, `migration_bitbucket.go`, `migration_bitbucket_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Removed issue APIs and obsolete authentication examples can create a superficially passing but unusable adapter. Workspace projects are repository grouping, not planning boards; source links or PR status observations grant no native authority.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-004, FR-007, FR-008, FR-011, FR-013, FR-014, NFR-001, NFR-002 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T025 T026 T027 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

