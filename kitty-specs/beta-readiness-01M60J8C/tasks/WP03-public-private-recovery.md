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
base_branch: kitty/mission-beta-readiness-01M60J8C
base_commit: 48aec71ae801f5090ca5e09a22bb54c7e67ff25c
created_at: '2026-10-28T10:39:28.356738+00:00'
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
- store.go
- memory_store.go
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

Add private disaster-restore and fresh-actor journeys: fence prior writer,
recover latest actor/memory chains, continue without history rewrite; refuse
common actor and same-actor/same-stream memory appends from missing/stale/
divergent roots when conflicting accepted history is known, with zero effects
before explicit verified reconciliation.

### T011

Verify lost sole authority, separate policy-role amendment, deliberate audit authority adoption, worktree limitations and backup exclusions; run charter gates.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Use existing Git and hn commands if they satisfy the contract; no new backup command is assumed. Inspect identity_keyring.go, docs/identity-v0.md and existing rotation/replication/migration tests. Use only disposable test keys in owner-only temp directories. Never read/copy the user's actual keyring or commit private keys. Public archives are not full private backups; ordinary clone omits custom refs. Private restore is exclusive disaster recovery of the same actor, never a way to run two active devices with one key. Latest actor chain recovery and fencing are prerequisites; do not claim the CLI can detect a disconnected second signer. A fresh device normally creates a distinct actor, with roles granted only by explicit remaining authority. Audit authority must be deliberately re-adopted; do not copy an anchor as proof. Avoid documenting blind copy/replay of migration journals against a new destination. Rehearsals must exercise the exact documented command sequence through public boundaries, not only call implementation internals. Describe safe trust assumptions for locally held backups and distinguish untrusted imports. Do not run arbitrary hooks from imported data or weaken existing refusal behavior. If a production gap appears, report it for explicit owned-surface assignment before editing sibling surfaces.

## Ownership and delivery

Owned surfaces: `beta_recovery_test.go`, `docs/recovery.md`, `store.go`,
`memory_store.go`.

Concrete production obligation: `nextEvent` currently derives its predecessor
only from the local actor ref, and common append validates only that chain.
Sync can leave it stale after accepting a newer same-actor history. Guard the
common actor append path before effects against locally accepted newer or
divergent same-actor histories. Cover missing and stale local roots, no ref/event
changes on refusal, and successful continuation after explicit verified CAS
restoration. Do not silently advance the actor root, grant roles, treat invalid
or pending data as accepted, or claim to detect unknown disconnected writers.
Inspect existing verified-reader facilities before adding another verifier.

The analogous memory risk is confirmed in `nextMemoryCommandPosition` and
`appendMemoryAtHead`, which currently consider only the local stream root.
Guard the common appendMemoryAtHead path before writes against known newer or
divergent accepted history for that same actor and stream. Compare signed
sequence/ID continuity, not just maximum sequence; other streams and accepted
older prefixes must not block valid work. No changes to memory publication,
wire formats or projection are authorized by this narrow recovery fix.

Use read-only verification primitives. `collectEvents`/`loadStoredEventAt` can
import audit packs and mutate the object database; they cannot be treated as
side-effect-free preflight. `collectIssueEvents` has a bounded batched verified
reader, but applying all of its issue-catalog limits to every actor write would
be an unrelated compatibility change: assess scope and request another narrow
ownership amendment if necessary, rather than copying a second verifier.
Memory readers validate each source but may union same-stream signed forks;
the writer guard must explicitly detect conflicting signed positions.

Public recovery tests must inspect accepted refs even if a stale memory sync
returns a publication error after successful import. Do not broaden this WP to
change publication. Prove refusal before object/ref/private-state changes, then
successful continuation after explicit verified expected-old restoration for
both actor and memory roots, preserving every prior payload/signature.
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
