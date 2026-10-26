---
work_package_id: WP04
title: Portable project snapshots and bounded archive format
dependencies: []
requirement_refs:
- FR-002
- FR-003
- FR-004
- FR-005
- FR-014
- FR-015
- NFR-001
- NFR-002
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0
base_commit: 946a72d8c76836814a349d71323f2baf41403ebd
created_at: '2026-10-26T10:54:05.046402+00:00'
subtasks:
- T010
- T011
- T012
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: internal/migration/
create_intent:
- internal/migration/snapshot.go
- internal/migration/archive.go
- internal/migration/archive_test.go
- project_snapshot.go
- project_snapshot_test.go
- docs/project-migration.md
execution_mode: code_change
owned_files:
- internal/migration/snapshot.go
- internal/migration/archive.go
- internal/migration/archive_test.go
- project_snapshot.go
- project_snapshot_test.go
- docs/project-migration.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP04 — Portable project snapshots and bounded archive format

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Define a bounded portable representation of selected Git/native histories, attributed foreign communications and verified attachment blobs.

**Independent acceptance**: An archive round trip retains selected Git object/signed event bytes and locally inspectable foreign topology/blobs; malformed paths, members, digests, budgets and incomplete native dependencies refuse before accepted visibility.

**Requirements**: FR-002, FR-003, FR-004, FR-005, FR-014, FR-015, NFR-001, NFR-002.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

This is an independent root; no feature implementation dependency.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T010 — Specify snapshot/archive contracts and hostile input cases

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Test explicit code/collaboration ref selection, source identity/revision and per-object provenance, source-only PR/MR/review/planning/workflow records, referenced attachments, digests and positive traversal/expansion budgets. Establish native-versus-attributed preservation semantics. Freeze closed common issue/comment record schemas with exact supported fields, typed source IDs and stable cross-links before providers implement them; use one canonical schema rather than provider-specific guesswork.
- **Files**: `internal/migration/snapshot.go`, `internal/migration/archive.go`, `internal/migration/archive_test.go`, `project_snapshot.go`, `project_snapshot_test.go`, `docs/project-migration.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T011 — Implement deterministic snapshot and archive read/write

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Keep portable data independent of provider clients. Exclude local private identities, keys, credentials, journals, unrelated refs and working-tree junk; preserve selected committed history exactly without claiming secret scrubbing. Reject duplicates, absolute/traversal paths, symlinks, unexpected members and bombs.
- **Files**: `internal/migration/snapshot.go`, `internal/migration/archive.go`, `internal/migration/archive_test.go`, `project_snapshot.go`, `project_snapshot_test.go`, `docs/project-migration.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T012 — Verify exact preservation and isolation

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Use temporary repositories and deterministic fixture blobs to prove exact signed bytes/object IDs, relationships and readable preserved records. Test corrupt/missing dependencies and wrong digests with no accepted ref/policy/identity changes; document archive structure and budgets.
- **Files**: `internal/migration/snapshot.go`, `internal/migration/archive.go`, `internal/migration/archive_test.go`, `project_snapshot.go`, `project_snapshot_test.go`, `docs/project-migration.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

A bare Git archive of all refs could leak local/private or unrelated state. Foreign review observations must never be interpreted as native approvals; source workflow bytes remain inert. Git object history and attachment bytes need separate traversal/size budgets.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-002, FR-003, FR-004, FR-005, FR-014, FR-015, NFR-001, NFR-002 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T010 T011 T012 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

