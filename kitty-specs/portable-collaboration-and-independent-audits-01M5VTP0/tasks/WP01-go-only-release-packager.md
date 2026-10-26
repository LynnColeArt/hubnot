---
work_package_id: WP01
title: Go-only portable release packager
dependencies: []
requirement_refs:
- FR-001
- FR-021
- FR-022
- NFR-004
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: cmd/hn-release/
create_intent:
- cmd/hn-release/main.go
- cmd/hn-release/main_test.go
- internal/release/package.go
- internal/release/package_test.go
- docs/releasing.md
execution_mode: code_change
owned_files:
- cmd/hn-release/main.go
- cmd/hn-release/main_test.go
- internal/release/package.go
- internal/release/package_test.go
- main.go
- docs/releasing.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP01 — Go-only portable release packager

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Replace prospective release packaging with a Go/Git-only command that runs without Python or a Unix shell.

**Independent acceptance**: Two clean builds from one declared source/toolchain/version yield equal binary/source archives and checksums; the command cross-compiles for Windows/macOS and produces independently inspected archive contents.

**Requirements**: FR-001, FR-021, FR-022, NFR-004.

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

### T001 — Specify deterministic package inputs and adversarial archive/output tests

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Add tests for exact source selection, supported target/flavor names, target-independent path separators, normalized metadata/modes, output collisions, Git cleanliness/provenance, and archive digest reproducibility. Work from the exact source revision; do not mutate published tags or assets.
- **Files**: `cmd/hn-release/main.go`, `cmd/hn-release/main_test.go`, `internal/release/package.go`, `internal/release/package_test.go`, `main.go`, `docs/releasing.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T002 — Implement the release command and archive/checksum generation

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Use Go standard-library archive/gzip/tar/zip/crypto plus Git and Go subprocesses with explicit argument arrays. Build Linux/macOS AMD64/ARM64 standard and native-only artifacts, source, provenance and SHA-256 checksums. Native-only must exclude GitHub even before WP06 lands: use both `nogithub,noforge` tags (unknown future noforge alone does not exclude the current GitHub adapter). Final integration verifies that noforge excludes every installed network adapter. If linker version injection is needed, preserve the existing development default in main.go. Avoid Python, shell scripts, provider CLIs, implicit publication and unrecorded local source modifications.
- **Files**: `cmd/hn-release/main.go`, `cmd/hn-release/main_test.go`, `internal/release/package.go`, `internal/release/package_test.go`, `main.go`, `docs/releasing.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T003 — Verify repeatability and portable packaging behavior

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Run the focused package tests, two clean packaging runs, independent archive inspection and Windows/macOS packager cross-compilation. Document actual runtime-tested hosts separately from target cross-compilation; preserve v0.0.1-dev byte-for-byte.
- **Files**: `cmd/hn-release/main.go`, `cmd/hn-release/main_test.go`, `internal/release/package.go`, `internal/release/package_test.go`, `main.go`, `docs/releasing.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Source selection, timestamps, modes and gzip headers can undermine reproducibility; host-only paths and shell assumptions can undermine Windows packaging. Treat host runtime qualification as evidence, never infer it from compilation.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-001, FR-021, FR-022, NFR-004 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T001 T002 T003 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.

