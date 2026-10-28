---
work_package_id: WP03
title: Public backup and exclusive private recovery
dependencies:
- WP02
requirement_refs:
- C-003
- FR-005
- FR-006
- FR-007
- NFR-001
- NFR-002
- NFR-003
- NFR-005
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T008
- T009
- T010
- T011
phase: Beta readiness
history: []
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- beta_recovery_test.go
- docs/recovery.md
execution_mode: code_change
owned_files:
- beta_recovery_test.go
- docs/recovery.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP03 — Public backup and exclusive private recovery

## Objectives & Success Criteria

### T008

Write executable recovery procedures in docs/recovery.md separating public code/custom refs, private keys/local state and uncertain external journals.

### T009

Add a keyless fresh-clone/public-backup rehearsal retaining selected collaboration and memory history and comparing exact historical objects/ref targets.

### T010

Add private disaster-restore and fresh-actor journeys: fence prior writer, recover latest chain, continue without history rewrite; reject unsafe private inputs and document stale-backup limits.

### T011

Verify lost sole authority, separate policy-role amendment, deliberate audit authority adoption, worktree limitations and backup exclusions; run charter gates.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Use existing Git and hn commands if they satisfy the contract; no new backup command is assumed. Inspect identity_keyring.go, docs/identity-v0.md and existing rotation/replication/migration tests. Use only disposable test keys in owner-only temp directories. Never read/copy the user's actual keyring or commit private keys. Public archives are not full private backups; ordinary clone omits custom refs. Private restore is exclusive disaster recovery of the same actor, never a way to run two active devices with one key. Latest actor chain recovery and fencing are prerequisites; do not claim the CLI can detect a disconnected second signer. A fresh device normally creates a distinct actor, with roles granted only by explicit remaining authority. Audit authority must be deliberately re-adopted; do not copy an anchor as proof. Avoid documenting blind copy/replay of migration journals against a new destination. Rehearsals must exercise the exact documented command sequence through public boundaries, not only call implementation internals. Describe safe trust assumptions for locally held backups and distinguish untrusted imports. Do not run arbitrary hooks from imported data or weaken existing refusal behavior. If a production gap appears, report it for explicit owned-surface assignment before editing sibling surfaces.

## Ownership and delivery

Owned surfaces: `beta_recovery_test.go`, `docs/recovery.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Public subprocess tests in standard/native-only modes; compare all selected preexisting history and assert absent unwanted keys/authority/optional activation. Test stale public chain recovery, lost signer/fresh actor, unsafe modes and unchanged history. Then gofmt, full race, vet, build, diff check at a committed head. Coordinate full-suite resource use with root.

Meaningful tests assert the promised observable contract and must detect the
claimed fault; never weaken refusal assertions or add xfails to get a pass.

## Review Guidance

Check requirement coverage, truthful qualification, native independence,
disabled defaults, immutable history, private-state handling and the distinction
between provenance and authority. Review the fixed committed head. Capture a
canonical verdict through the framework, not only a prose message.

## Handoff

Commit only expected files. Record test commands, exits, evidence and remaining
obligations. Mark only this WP's subtasks done when truly completed. Move to
`for_review` with the framework and wait for the synchronous gate to finish;
do not kill/retry an in-flight gate or claim a skipped/no-coverage gate ran tests.

## Activity Log

- 2026-10-28T06:21:00Z – codex – Work package authored from the approved mission intent.

