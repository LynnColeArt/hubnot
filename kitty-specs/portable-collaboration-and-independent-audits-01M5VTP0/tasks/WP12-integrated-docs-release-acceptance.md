---
work_package_id: WP12
title: Integrated acceptance, documentation and release readiness
dependencies:
- WP01
- WP02
- WP03
- WP04
- WP05
- WP06
- WP07
- WP08
- WP09
- WP10
- WP11
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-010
- FR-011
- FR-012
- FR-013
- FR-014
- FR-015
- FR-016
- FR-017
- FR-018
- FR-019
- FR-020
- FR-021
- FR-022
- NFR-001
- NFR-002
- NFR-003
- NFR-004
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
subtasks:
- T034
- T035
- T036
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: portable_collaboration_acceptance_test.go
create_intent:
- portable_collaboration_acceptance_test.go
execution_mode: code_change
owned_files:
- portable_collaboration_acceptance_test.go
- README.md
- CHANGELOG.md
- docs/protocol-v0.md
- docs/threat-model.md
- docs/github-interoperability.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP12 — Integrated acceptance, documentation and release readiness

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Integrate the independent capabilities, verify the full mission contract through public boundaries and document only shipped, tested behavior.

**Independent acceptance**: Ordinary/nogithub/adapter-free builds pass native/offline migration and audit acceptance; each named provider has non-vacuous fixture coverage; Go/Git-only packaging is reproducible; full charter gates and requirement coverage are recorded without changing public v0.0.1-dev artifacts.

**Requirements**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, FR-013, FR-014, FR-015, FR-016, FR-017, FR-018, FR-019, FR-020, FR-021, FR-022, NFR-001, NFR-002, NFR-003, NFR-004.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. Earlier packages own their stream guides; if integration finds drift in those guides, feed a concrete correction to their owner rather than claiming overlapping final-lane ownership. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

Prerequisites: WP01, WP02, WP03, WP04, WP05, WP06, WP07, WP08, WP09, WP10, WP11 must be approved; base the lane on the runtime's dependency computation.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T034 — Add integrated public acceptance and negative controls

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Exercise fresh/local/offline projects, native audit replication, foreign preservation and archive hostility with all capabilities coexisting. Verify disabled defaults, existing merge/required-check behavior, unchanged actor/key/policy authority, explicit local audit-anchor setup and import isolation, dependent receipt/blob resolution and uncertain-resume payload binding and existing transfer compatibility across build variants.
- **Files**: `portable_collaboration_acceptance_test.go`, `README.md`, `CHANGELOG.md`, `docs/protocol-v0.md`, `docs/threat-model.md`, `docs/github-interoperability.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T035 — Synchronize operator docs, examples and capability limits

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Update README/changelog/protocol/threat and focused guides with exact commands, provider resource/direction matrices, explicit authorization/loss choices, first-run defaults, recovery/manual-reconciliation steps and Go/Git-only packaging. Clearly separate source preservation, native mapping and destination recreation.
- **Files**: `portable_collaboration_acceptance_test.go`, `README.md`, `CHANGELOG.md`, `docs/protocol-v0.md`, `docs/threat-model.md`, `docs/github-interoperability.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T036 — Run release/readiness gates and trace all requirements

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Record source-bound formatting, full race, vet, build and diff gates, the ordinary/nogithub/adapter-free acceptance matrix, deterministic package verification and actual platform execution evidence. Report unavailable host qualification honestly; preserve all original public release/tag bytes. Prepare reviewable PR/mission acceptance evidence.
- **Files**: `portable_collaboration_acceptance_test.go`, `README.md`, `CHANGELOG.md`, `docs/protocol-v0.md`, `docs/threat-model.md`, `docs/github-interoperability.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Tests that each pass in isolated lanes can still fail together. Do not label unexercised hosts runtime-qualified, downgrade loss reports to success, omit failed-attempt evidence, or claim new provider functionality from capability-only stubs.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, FR-013, FR-014, FR-015, FR-016, FR-017, FR-018, FR-019, FR-020, FR-021, FR-022, NFR-001, NFR-002, NFR-003, NFR-004 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T034 T035 T036 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

