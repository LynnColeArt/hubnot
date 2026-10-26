---
work_package_id: WP03
title: Explicit audit execution and linked follow-up issues
dependencies:
- WP02
requirement_refs:
- FR-001
- FR-017
- FR-018
- FR-019
- FR-020
- FR-022
- NFR-001
- NFR-003
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
subtasks:
- T007
- T008
- T009
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: audit_commands.go
create_intent:
- docs/audits-v1.md
- audit_commands.go
- audit_commands_test.go
- audit_execution.go
- audit_execution_test.go
- audit_followup.go
- audit_followup_test.go
execution_mode: code_change
owned_files:
- audit_commands.go
- audit_commands_test.go
- audit_execution.go
- audit_execution_test.go
- audit_followup.go
- audit_followup_test.go
- docs/audits-v1.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP03 — Explicit audit execution and linked follow-up issues

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Make independent audits usable through explicit native commands and deliberately turn stable findings into retry-safe linked issues.

**Independent acceptance**: The compiled CLI audits a resolved code snapshot, records a nonblocking result, creates a linked issue only on explicit request, and retries after interruption without an issue flood or unauthorized rule changes.

**Requirements**: FR-001, FR-017, FR-018, FR-019, FR-020, FR-022, NFR-001, NFR-003.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

Prerequisites: WP02 must be approved; base the lane on the runtime's dependency computation.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T007 — Specify public request/execute/inspect/follow-up scenarios

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Cover absent opt-in, unauthorized actors/rules, passing/failing findings, snapshots surviving safe code-only branch movement, refusal of stale/candidate `--policy-ref` assertions, missing/private-anchor setup, imports not choosing authority, existing required merge checks, stable finding identity and no automatic execution. Use real repositories/identities and controlled fixture pipelines.
- **Files**: `audit_commands.go`, `audit_commands_test.go`, `audit_execution.go`, `audit_execution_test.go`, `audit_followup.go`, `audit_followup_test.go`, `docs/audits-v1.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T008 — Implement explicit audit execution and inspection

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Self-register native command dispatch using the existing mechanism, including explicit authority set/show, rule authorize/revoke/list/show, request, execute, result/log inspection and follow-up commands from the audit contract. Reuse backend.go sandbox selection and per-invocation unsafe-host flag; preserve bounded logs/evidence and deterministic finding keys. Do not start daemons, run during merge/import, or grant native authority from findings.
- **Files**: `audit_commands.go`, `audit_commands_test.go`, `audit_execution.go`, `audit_execution_test.go`, `audit_followup.go`, `audit_followup_test.go`, `docs/audits-v1.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T009 — Implement deliberate follow-ups and recovery

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Use native issue writes, authenticated audit links and actor-scoped operation/recovery semantics. Make duplicate/rerun behavior inspectable, bind retries to the original finding snapshot and document concurrent-publisher limits. Test interrupted issue creation and state-preserving refusals.
- **Files**: `audit_commands.go`, `audit_commands_test.go`, `audit_execution.go`, `audit_execution_test.go`, `audit_followup.go`, `audit_followup_test.go`, `docs/audits-v1.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Do not confuse audit failure with runner transport/error state, or use an unstable timestamp/commit-only key that generates duplicate issues for the same finding. Do not claim distributed exactly-once creation or silently edit unrelated native issues.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-001, FR-017, FR-018, FR-019, FR-020, FR-022, NFR-001, NFR-003 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T007 T008 T009 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

