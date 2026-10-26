---
work_package_id: WP05
title: Migration plan, apply/resume and local archive destinations
dependencies:
- WP02
- WP04
requirement_refs:
- FR-002
- FR-003
- FR-005
- FR-007
- FR-013
- FR-014
- FR-015
- NFR-001
- NFR-002
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0
base_commit: 946a72d8c76836814a349d71323f2baf41403ebd
created_at: '2026-10-26T12:00:13.703063+00:00'
subtasks:
- T013
- T014
- T015
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: migration_engine.go
create_intent:
- docs/migration-engine.md
- internal/migration/plan.go
- internal/migration/plan_test.go
- internal/migration/progress.go
- internal/migration/progress_test.go
- migration_engine.go
- migration_engine_test.go
- migration_native.go
- migration_native_test.go
- migration_local.go
- migration_local_test.go
- migration_journal.go
- migration_journal_test.go
execution_mode: code_change
owned_files:
- internal/migration/plan.go
- internal/migration/plan_test.go
- internal/migration/progress.go
- internal/migration/progress_test.go
- migration_engine.go
- migration_engine_test.go
- migration_native.go
- migration_native_test.go
- migration_local.go
- migration_local_test.go
- migration_journal.go
- migration_journal_test.go
- docs/migration-engine.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP05 — Migration plan, apply/resume and local archive destinations

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Provide one inspectable migration engine for repository/archive sources and destinations with explicit governance activation and guarded resume.

**Independent acceptance**: A saved plan imports a portable project into a fresh local repository with adapters/credentials absent, preserves destination governance/identity, reports every mapping/loss, and resumes interrupted progress safely.

**Requirements**: FR-002, FR-003, FR-005, FR-007, FR-013, FR-014, FR-015, NFR-001, NFR-002.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

Prerequisites: WP02, WP04 must be approved; base the lane on the runtime's dependency computation.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T013 — Specify plans, loss decisions and destination invariants

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Use frozen source observations and destination expectations. Cover mapped/preserved/requires-mapping/unsupported/missing/conflict findings, no secrets in plans, explicit loss acknowledgments, and no assumption that unread data was preserved. Include selected native audit histories from WP02, without importing any private audit-authority anchor. Cover symbolic dependent receipt references and closed canonical issue/comment schemas.
- **Files**: `internal/migration/plan.go`, `internal/migration/plan_test.go`, `internal/migration/progress.go`, `internal/migration/progress_test.go`, `migration_engine.go`, `migration_engine_test.go`, `migration_native.go`, `migration_native_test.go`, `migration_local.go`, `migration_local_test.go`, `migration_journal.go`, `migration_journal_test.go`, `docs/migration-engine.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T014 — Implement local/archive plan and guarded application

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Stage all archive/object validation before accepted visibility through existing native admission. Preserve historical policies for verification while keeping source-policy activation explicit and existing destination policy intact. Enforce expected-old refs, clean path boundaries and no imported local signer identities. Resolve dependent intents (issue creation then comment creation) only from confirmed dependency receipts; give Apply/Reconcile a bounded ApplyContext containing verified blob access and those receipts. Persist resolved payload plus digest before its external write.
- **Files**: `internal/migration/plan.go`, `internal/migration/plan_test.go`, `internal/migration/progress.go`, `internal/migration/progress_test.go`, `migration_engine.go`, `migration_engine_test.go`, `migration_native.go`, `migration_native_test.go`, `migration_local.go`, `migration_local_test.go`, `migration_journal.go`, `migration_journal_test.go`, `docs/migration-engine.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T015 — Implement resume and prove offline continuation

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Persist authenticated intent/progress and recover completed steps without repeating uncertain creates or changing the selected snapshot. Require a crash/uncertain comment resume to reuse exactly the persisted resolved payload and destination issue ID; no adapter-private mutable state or guessed IDs. Add crash/state-preservation tests and offline local read/edit scenarios, including source-only conversations/blobs, with no provider code required.
- **Files**: `internal/migration/plan.go`, `internal/migration/plan_test.go`, `internal/migration/progress.go`, `internal/migration/progress_test.go`, `migration_engine.go`, `migration_engine_test.go`, `migration_native.go`, `migration_native_test.go`, `migration_local.go`, `migration_local_test.go`, `migration_journal.go`, `migration_journal_test.go`, `docs/migration-engine.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Importing actor histories into signer-owned refs or overwriting a destination policy can manufacture authority. Atomicity is limited across Git refs/files/external writes; expose manual reconciliation rather than guessing success after a crash.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-002, FR-003, FR-005, FR-007, FR-013, FR-014, FR-015, NFR-001, NFR-002 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T013 T014 T015 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

