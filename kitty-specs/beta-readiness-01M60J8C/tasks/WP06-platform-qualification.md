---
work_package_id: WP06
title: Candidate package and distribution qualification
dependencies:
- WP03
- WP13
requirement_refs:
- FR-010
- FR-017
- FR-020
- NFR-002
- NFR-004
- NFR-006
- NFR-007
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-beta-readiness-01M60J8C
base_commit: 43aa2d6718fa1e65f016f82b4fb6e74e2ec1ba7c
created_at: '2026-10-29T14:59:00.841463+00:00'
subtasks:
- T018
- T019
- T020
- T046
- T047
- T048
phase: Beta readiness
history: []
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- docs/platform-qualification.md
- beta_platform_package_test.go
- beta_compatibility_test.go
- contribution_commands.go
- contribution_moderation.go
- contribution_deadline_test.go
- docs/contributions.md
execution_mode: code_change
owned_files:
- docs/platform-qualification.md
- migration_commands_test.go
- portable_collaboration_acceptance_test.go
- beta_platform_package_test.go
- beta_compatibility_test.go
- .github/workflows/portability.yml
- docs/filesystem-portability.md
- contribution_commands.go
- contribution_moderation.go
- contribution_deadline_test.go
- docs/contributions.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP06 — Candidate package and distribution qualification

## Runtime repair amendment — T046–T048, 2026-10-29

Independent contract/source assessment at
`/home/lynn/Documents/Codex/2026-10-24/wa/work/beta-readiness-WP06/deadline-contract-assessment.md`
established three late-stage deadline gaps at runtime `9a950e8f`: receipt
retained-artifact revalidation, coverage manifest planning and native object copy
pass Background into context-aware work before public promotion. Initial ingress
already has a finite timer; material and aggregate accounting remain bounded.
This is not a new whole-CLI 30-second requirement or a proven explanation of the
separate Intel 45-second test-helper timeout.

WP06 explicitly owns the two existing runtime files from WP12, one new regression
file and the existing WP13 guide for this cohesive repair. The inherited paths
also appear in create_intent because the planning target lacks dependency-created
files; inspect and modify the approved dependency contents, never create stubs.
Prior WP12/WP13 approvals remain historical. Existing lane assignments stay fixed;
source writers and commits are serialized. A separate focused implementer and
independent source reviewer cover this repair before final WP06 review.

T046 must establish original-runtime red evidence at each actual late phase,
after successful initial intake, with a positive marker/control proving the
stall was reached. T047 must propagate finite appropriate original-reservation/
parent contexts through retention, coverage, copy and receipt revalidation,
checking cancellation before later authority/signing/publication effects. Do not
recursively call a lock-taking inspection method while holding the intake lock.
Separately charged controls keep their own finite request context.

Preserve exact digest/ref/intent and reservation semantics, immutable retained
source, failure charges, authority, cryptography, defaults and disabled behavior.
No source reopening, refund, reservation extension, deadline increase, skipped
case, schema change, broad Git/store rewrite or unrelated optimization. Preserve
the documented synchronous legacy/kernel-I/O cancellation limitations.

Tests must exercise actual context-aware subprocess cancellation, not only context
presence. Assert no prohibited ref/admission/receipt publication, inspectable
retained state and charged original reservation, then successful exact retry.
Charged immutable partial object retention is not total filesystem rollback.
Use existing package-aware public helpers for real standard/native assertions;
source-assisted faults are labelled, not package/process-kill proof. T048 updates
the actual stage/request scope in the guide without concealing the gap, then
formatting, vet/build and one full uncached race run with a 30-minute harness
ceiling at a fixed clean head. Coordinate the one local execution slot with root.
Request additional ownership before editing any unlisted production surface.

Runtime changes require a new immutable candidate and requalification. H0/H2/H3
observations remain exact-head historical evidence. Workflow-only orchestration
may reuse an unchanged candidate with separately bound workflow/source/harness.

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
`docs/filesystem-portability.md`. The 2026-10-29 qualification repair also owns
`beta_compatibility_test.go` for the bounded test-fixture correction below.

### Qualification repair decision — 2026-10-29

The first candidate run, `40922372405` at `0b556b36`, failed on both Linux
architectures under Git 2.56.0: complete-repository refusal snapshots observed
temporary pack files disappearing and directory cleanup racing Git activity.
Detached automatic maintenance is a hypothesis, not a proven application defect.
The existing replication fixtures already explicitly disable automatic
maintenance to establish quiescent snapshot preconditions.

This WP may make the disposable compatibility reader/destination fixtures
quiescent before clone/fetch/continuation/import using per-command or scoped
repository maintenance settings. Do not change global Git configuration,
production defaults, the closed application environment or frozen released-writer
data. Preserve the complete before/after snapshot comparison and every traversal
or read failure; do not ignore pack files, weaken the assertion, skip cases or
retry away failures. Add bounded path-only diagnostics for differing snapshots
without printing private file contents. Preserve first-run evidence and verify
the corrected fixture on the observed Git 2.56 environments using a new fixed
candidate revision. Remaining failures require investigation rather than a
presumed maintenance explanation. Enable the existing portable CLI transcript
hook in the owned workflow for the new run.

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
