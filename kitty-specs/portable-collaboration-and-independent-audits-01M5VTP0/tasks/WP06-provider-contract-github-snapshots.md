---
work_package_id: WP06
title: Shared provider contract and GitHub project snapshots
dependencies:
- WP05
requirement_refs:
- FR-004
- FR-007
- FR-008
- FR-009
- FR-013
- FR-014
- FR-015
- NFR-001
- NFR-002
- NFR-003
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0
base_commit: 946a72d8c76836814a349d71323f2baf41403ebd
created_at: '2026-10-27T01:43:56.690040+00:00'
subtasks:
- T016
- T017
- T018
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: internal/migration/
create_intent:
- internal/migration/provider.go
- internal/migration/provider_test.go
- internal/migration/conformance_test.go
- migration_provider.go
- migration_provider_test.go
- migration_github.go
- migration_github_test.go
- internal/githubapi/migration.go
- internal/githubapi/migration_test.go
execution_mode: code_change
owned_files:
- internal/migration/provider.go
- internal/migration/provider_test.go
- internal/migration/conformance_test.go
- migration_provider.go
- migration_provider_test.go
- migration_github.go
- migration_github_test.go
- forge_github.go
- github_disabled.go
- github_discussions.go
- github_projects.go
- github_workflows.go
- github_discussions_test.go
- github_import_test.go
- github_projects_test.go
- github_workflows_test.go
- internal/githubapi/**
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP06 — Shared provider contract and GitHub project snapshots

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Extend optional adapters through a documented shared migration contract and deliver a real GitHub snapshot/destination implementation without breaking transfer commands.

**Independent acceptance**: GitHub fixtures preserve issue/comments, PR conversations/reviews, retrieved selected attachments and workflow source; supported destination issue operations recover safely; old transfer commands and nogithub retain their meaning, while an all-adapter-free build remains locally functional.

**Requirements**: FR-004, FR-007, FR-008, FR-009, FR-013, FR-014, FR-015, NFR-001, NFR-002, NFR-003.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

Prerequisites: WP05 must be approved; base the lane on the runtime's dependency computation.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T016 — Define adapter registry, capabilities and conformance contract

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Specify resource/direction mapping versus preservation-only capability; explicit instance identity/selectors, bounded reads, one-write intents, ApplyContext with verified blobs/confirmed dependency receipts, engine-persisted resolved payloads/digests, credential policies and positive reconciliation. Make self-registration possible in disjoint provider files and ensure all-unsupported registration cannot satisfy baseline coverage.
- **Files**: `internal/migration/provider.go`, `internal/migration/provider_test.go`, `internal/migration/conformance_test.go`, `migration_provider.go`, `migration_provider_test.go`, `migration_github.go`, `migration_github_test.go`, `forge_github.go`, `github_disabled.go`, `github_discussions.go`, `github_projects.go`, `github_workflows.go`, `github_discussions_test.go`, `github_import_test.go`, `github_projects_test.go`, `github_workflows_test.go`, `internal/githubapi/**`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T017 — Implement GitHub migration reader/writer and adapter exclusion

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Freeze the shared `internal/migration` Source/Destination/Snapshot/Record/Plan/Intent/Receipt/Reconciliation/ApplyContext contract and add scripted command/registration seams needed downstream. Reuse existing GitHub transport/typed transfer semantics and add whole-project observations rather than treating an issue dump as a full migration. Preserve PR/review topology and verified selected blobs with attributed provenance; foreign observations never become native permissions. Preserve nogithub's current GitHub-only exclusion while adding `noforge` to all GitHub-only root/client/test build constraints; disabled GitHub help must remain available with either exclusion tag. The new native-only release variant uses all-adapter exclusion, while legacy `nogithub` remains GitHub-specific.
- **Files**: `internal/migration/provider.go`, `internal/migration/provider_test.go`, `internal/migration/conformance_test.go`, `migration_provider.go`, `migration_provider_test.go`, `migration_github.go`, `migration_github_test.go`, `forge_github.go`, `github_disabled.go`, `github_discussions.go`, `github_projects.go`, `github_workflows.go`, `github_discussions_test.go`, `github_import_test.go`, `github_projects_test.go`, `github_workflows_test.go`, `internal/githubapi/**`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T018 — Prove compatibility, bounds and recovery against fixtures

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Test pagination cycles/truncation, redirects/cross-origin credential refusal, unauthorized or inaccessible blobs, stable identity vs mutable numbers, loss reports and lost-response reconciliation. Prove issue-create receipts resolve later comment targets through the engine, including lost-response resume with identical persisted resolved payload/digest; do not keep this association only inside an adapter instance. Existing native and transfer acceptance must still work; no network calls from local help/inspection/archive paths.
- **Files**: `internal/migration/provider.go`, `internal/migration/provider_test.go`, `internal/migration/conformance_test.go`, `migration_provider.go`, `migration_provider_test.go`, `migration_github.go`, `migration_github_test.go`, `forge_github.go`, `github_disabled.go`, `github_discussions.go`, `github_projects.go`, `github_workflows.go`, `github_discussions_test.go`, `github_import_test.go`, `github_projects_test.go`, `github_workflows_test.go`, `internal/githubapi/**`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Shared adapter code must not become a mandatory runtime service or vendor SDK. An export capability must be backed by actual writes and recovery tests; preservation-only data must not be advertised as destination-native recreation.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-004, FR-007, FR-008, FR-009, FR-013, FR-014, FR-015, NFR-001, NFR-002, NFR-003 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T016 T017 T018 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

