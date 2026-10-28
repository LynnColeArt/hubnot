---
work_package_id: WP06
title: Candidate package and distribution qualification
dependencies:
- WP03
- WP13
requirement_refs:
- FR-010
- NFR-004
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T018
- T019
- T020
phase: Beta readiness
history: []
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- docs/platform-qualification.md
- beta_platform_package_test.go
execution_mode: code_change
owned_files:
- docs/platform-qualification.md
- migration_commands_test.go
- portable_collaboration_acceptance_test.go
- beta_platform_package_test.go
- .github/workflows/portability.yml
- docs/filesystem-portability.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP06 — Candidate package and distribution qualification

## Objectives & Success Criteria

### T018

Define exact-package Linux/macOS architecture/variant installation and runtime cases, supported Git/filesystem configurations and download experience in docs/platform-qualification.md.

### T019

Run available candidate package cases using existing Go-only packaging and public package acceptance; collect actual platform/binary evidence.

### T020

Record outstanding Intel/Mac exact-byte work and a signing/notarization recommendation with owner decision prerequisites.

## Context & Constraints

Qualify the integrated candidate containing the contribution workflow, not a
pre-feature package. WP13 is a dependency so the lane includes all runtime
changes before packaging. Earlier alpha/package observations remain narrow
historical evidence, not automatic qualification of the final beta candidate.

Read the mission spec, plan, research and active charter before work. Load the
frontmatter profile through the canonical profiles API/CLI. The mission is
`beta-readiness-01M60J8C`; always pass that selector to framework commands.

Do not relabel Go 1.26.6 native Mac rebuilds as execution of Go 1.26.4 published binaries. Reuse test-only package-dir/version harness flags and existing CI gates. Linux ARM host lynn@192.168.1.145 has prior explicit authorization for package testing, but verify reachability and use disposable paths. Do not message another Codex or external person without explicit authorization. No release/tag publication is implied.

## Ownership and delivery

Owned surfaces: `docs/platform-qualification.md`,
`migration_commands_test.go`, `portable_collaboration_acceptance_test.go`,
`beta_platform_package_test.go`, `.github/workflows/portability.yml`, and
`docs/filesystem-portability.md`.

Concrete package-evidence obligation: the existing package-dir/version flags
in `portableJourneyBinary` already consume extracted real packages and refuse
invalid/missing assets without rebuilding. Extend `buildMigrationCLIBinary`
only when package flags are explicitly supplied to reuse this same verifier;
preserve normal source-build behavior otherwise and avoid recursive fallback.
This makes historical compatibility, recovery and public journal subprocess
journeys reusable against actual binaries. Add bounded public installation/PATH
and native Mac literal-/tmp, /var/tmp and absent-child-TMPDIR cases through the
same package-aware helpers. Keep internal-only journal tests labelled source
proof, and preserve the intentional package-mode exclusion of the unshipped
nogithub-only variant. Inspect source tree/version/compiler/input and executable
hash observations without treating embedded Go VCS metadata as a source oracle.

Extend the existing optional portability workflow, retaining its source PR
checks and dispatch entry. Build one immutable candidate bundle from committed
source H with explicit version V and compiler G, then transport those same eight
standard/native-only binary tarballs, source tarball, PROVENANCE and checksums
to four native runtime jobs. Independently check archive, BUILD and executable
digests and run package-mode integrated, released-writer, recovery, journal,
platform and WP13 contribution journeys without rebuilding the tested app.
Use explicit available native labels, verified against current official docs;
research on 2026-10-28 found ubuntu-24.04, ubuntu-24.04-arm, macos-15 and
macos-15-intel. A latest label is not evidence for both architectures. CI remains
optional project infrastructure, not a Hubnot runtime dependency.

Record actual source H, source tree, compiler, target/variant, version, digests,
OS/Git/filesystem/temp environment, architecture/native-versus-Rosetta and
commands/exits. A PR synthetic merge SHA qualifies only that candidate; do not
silently transfer proof to another revision or separately rebuilt assets.
Documentation-only follow-up may describe the immutable tested candidate after
its source commit, provided exact product/source equivalence and provenance are
explicit; do not create an endless rebuild cycle solely to include test logs.
Runtime changes after qualification require a new candidate and appropriate
requalification. Golden Goddess can run a centrally cross-compiled test harness
if Go is unavailable, with non-race instrumentation labelled honestly.

Mac CI artifact execution does not prove browser quarantine/Gatekeeper download
experience or outside adopter feedback. Record unsigned/unnotarized status and
actual observation limits; make signing/notarization recommendations separately
from current support promises. Do not disable host protection globally or add
unpromised signing gates. Root's current Pink Menace name lookup succeeded but
SSH port 22 refused; do not claim remote Mac access or scan unrelated hosts.
Windows remains the owner's lower-priority future track, not this matrix.
Stay within this WP. If an actual defect requires an additional production
surface, report the failing contract so root can amend ownership or author a
repair WP. Do not edit sibling status or frozen past-mission evidence.

Use the runtime-provided lane worktree and prompt. Planning/merge target is
`feature/beta-readiness`; the eventual PR targets `main`.

## Verification

Each claimed OS/architecture/variant gets observed normal-environment package evidence. Missing hardware is pending, not a pass. Inspect archive digests/BUILD provenance independently of runtime qualification.

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
