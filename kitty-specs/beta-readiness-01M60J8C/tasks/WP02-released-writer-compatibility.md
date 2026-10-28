---
work_package_id: WP02
title: Released-writer compatibility and refusal coverage
dependencies:
- WP01
requirement_refs:
- FR-002
- FR-003
- FR-004
- NFR-001
- NFR-003
- NFR-005
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T004
- T005
- T006
- T007
phase: Beta readiness
history: []
agent_profile: implementer-ivan
authoritative_surface: testdata/compatibility/
create_intent:
- beta_compatibility_test.go
- testdata/compatibility
- docs/compatibility.md
execution_mode: code_change
owned_files:
- beta_compatibility_test.go
- testdata/compatibility/**
- docs/compatibility.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP02 — Released-writer compatibility and refusal coverage

## Objectives & Success Criteria

### T004

Create a small offline public-only compatibility corpus from released writers with source/artifact provenance, exact fixture hashes and expected semantic CLI observations.

### T005

Add beta_compatibility_test.go for standard/native-only readers, historical IDs/signatures/ref preservation, representative archives/resources/memory and new-actor continuation.

### T006

Exercise unknown protocol/resource/archive versions through public boundaries; assert actionable refusal without accepted-history/private-state mutation.

### T007

Document the tested historical baseline, corpus maintenance and supported compatibility/rollback behavior; run charter gates and preserve results.

## Context & Constraints

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Read docs/beta-support.md first. Use actual local 0.0.2-dev and 0.0.3-dev package artifacts where available (/home/lynn/Documents/Codex/2026-10-24/wa/releases), and a released 0.0.1-dev writer if supported by the contract. A source rebuild must be labeled as such, not as exact published-byte qualification. Do not generate fixtures from current encoders during tests or retain fixture private keys. Do not commit executable probe source under kitty-specs. Use bounded text fixtures/Git bundles or archives as appropriate. Reuse existing public CLI subprocess infrastructure; inspect migration_acceptance_test.go first. Assert public semantics rather than brittle whitespace/help output or a current-output SHA ratchet. Unsupported-format fixtures must be valid enough to reach the intended version gate, not fail earlier for corruption. Historical data must stay byte-exact; do not change wire versions gratuitously. Production fixes, if necessary, require reporting the affected surface and a documented failing contract before expanding ownership.

## Ownership and delivery

Owned surfaces: `beta_compatibility_test.go`, `testdata/compatibility/**`, `docs/compatibility.md`.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Test-first contract fixtures and public CLI scenarios with a credential-free environment. Focused iterations, then gofmt, go test -race -count=1 -timeout 20m ./..., go vet ./..., go build ./..., git diff --check at the final committed head. No simultaneous expensive full suites; tell root before the final run.

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
