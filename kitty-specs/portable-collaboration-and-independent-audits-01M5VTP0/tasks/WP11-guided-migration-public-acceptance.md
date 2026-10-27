---
work_package_id: WP11
title: Guided migration CLI and cross-provider acceptance
dependencies:
- WP05
- WP07
- WP08
- WP09
- WP10
requirement_refs:
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-013
- FR-015
- FR-022
- NFR-001
- NFR-002
- NFR-003
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0
base_commit: 946a72d8c76836814a349d71323f2baf41403ebd
created_at: '2026-10-27T04:39:46.505867+00:00'
subtasks:
- T031
- T032
- T033
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: migration_commands.go
create_intent:
- migration_commands.go
- migration_commands_test.go
- migration_acceptance_test.go
- migration_wizard.go
- migration_wizard_test.go
- docs/migration-cli.md
execution_mode: code_change
owned_files:
- migration_commands.go
- migration_commands_test.go
- migration_acceptance_test.go
- migration_wizard.go
- migration_wizard_test.go
- docs/migration-cli.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP11 — Guided migration CLI and cross-provider acceptance

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Expose one guided/noninteractive hn migrate flow that saves inspectable plans and proves a real GitLab-to-GitHub communications migration.

**Independent acceptance**: A real compiled CLI with local provider fixtures and temporary Git remotes captures decisions, previews without mutation, applies/resumes a saved plan, migrates code plus issues/comments between GitLab/GitHub, and preserves unsupported communications locally.

**Requirements**: FR-005, FR-006, FR-007, FR-008, FR-009, FR-013, FR-015, FR-022, NFR-001, NFR-002, NFR-003.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. Own the guided CLI guide in `docs/migration-cli.md`; earlier packages own their foundation guides. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

Prerequisites: WP05, WP07, WP08, WP09, WP10 must be approved; base the lane on the runtime's dependency computation.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T031 — Specify interactive/noninteractive public behavior

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Test source/destination/scope/mapping questions, saved plan identity, explicit loss/authority decisions, final apply confirmation, actionable missing-input errors and EOF never approving. Arguments/saved plans must exercise the same engine rather than a separate simpler implementation.
- **Files**: `migration_commands.go`, `migration_commands_test.go`, `migration_acceptance_test.go`, `migration_wizard.go`, `migration_wizard_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T032 — Implement guided command dispatch over the migration engine

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Self-register the native entry point, enumerate installed capabilities without network where possible and ask only necessary questions. Keep secrets out of plans/output; no implicit sync, push, source deletion, provider account provisioning or workflow activation.
- **Files**: `migration_commands.go`, `migration_commands_test.go`, `migration_acceptance_test.go`, `migration_wizard.go`, `migration_wizard_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T033 — Prove cross-provider and adapter-free user journeys

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Run genuine local-fixture GitLab-to-GitHub issue/comment/code migration, interrupted resume and hostile/lossy preflight cases. Prove archive/local inspection and continuation with all adapters absent, while legacy transfer commands and nogithub semantics remain unchanged.
- **Files**: `migration_commands.go`, `migration_commands_test.go`, `migration_acceptance_test.go`, `migration_wizard.go`, `migration_wizard_test.go`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

A wizard that merely prints guessed commands or a cross-provider test that only copies Git objects is not sufficient. Its saved plan must faithfully bind observations, decisions and mutation intent; no default or EOF answer can consent to loss or authority activation.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-005, FR-006, FR-007, FR-008, FR-009, FR-013, FR-015, FR-022, NFR-001, NFR-002, NFR-003 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T031 T032 T033 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.
