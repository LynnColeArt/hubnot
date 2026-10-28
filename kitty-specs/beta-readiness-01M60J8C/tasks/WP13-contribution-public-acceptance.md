---
work_package_id: WP13
title: Unified contribution public acceptance and operator guide
dependencies:
- WP12
requirement_refs:
- FR-021
- FR-015
- FR-016
- FR-017
- FR-018
- FR-019
- FR-020
- NFR-001
- NFR-003
- NFR-006
- NFR-007
- C-007
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T043
- T044
- T045
phase: Optional unified contributions
history: []
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- beta_contribution_acceptance_test.go
- docs/contributions.md
execution_mode: code_change
owned_files:
- beta_contribution_acceptance_test.go
- docs/contributions.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP13 — Unified contribution public acceptance and operator guide

## Objectives & Success Criteria

### T043

Add standard/native-only provider-free contributor/maintainer/observer public CLI journeys for issues, discussion replies and code proposals with upstream push explicitly denied.

### T044

Exercise enabled/disabled behavior, signed spam and transfer/retention bounds,
automatic-admission accounting across drained queues and many keys,
replay/interruption, scoped moderation/revocation/conflicts and unchanged
existing merge/CI/execution gates.

### T045

Write docs/contributions.md with submission/admission/moderation commands, optional policy examples, raw-view/history limits, recovery and actual resource-safety guarantees; supply pilot and boundary-review evidence.

## Context & Constraints

Read the active charter and mission spec, plan, data model and research in
`kitty-specs/beta-readiness-01M60J8C/`. This is the owner's explicit native,
optional unified-contribution amendment. Initial support/compatibility/recovery
work remains first. Load the declared profile through the canonical CLI/API.

This WP proves the complete user workflow, not only the new library API. Use real temporary repositories and ordinary local transport with credential-free closed child environments, on standard and native-only binaries. Show the contributor lacks upstream write rights by an actual rejected push, not absence of a credential variable. Contributor signs with its own ephemeral actor; shared content preserves original identities and bytes. Include unknown contributors, automatic issue/reply admission under explicit project rules, denial of unauthorized edits/moderator actions, a code proposal whose admission does not satisfy merge gates, and a fresh keyless observer reconstructing scoped decisions. Test both default-off ordinary collaboration and explicitly enabled operation rules. Early resource-bound assertions need measured reads/retained bytes/counts and cancellation, including many-key aggregate saturation. A plain Git post-download rejection is not evidence of a hard wire-size cap. Give operators a friendly file/reference path without a required service, account or server; optional external discovery may be described as a future integration, not claimed implemented. Published original facts remain immutable; hides/rejections/blocks change only project-scoped governed views and do not promise global erasure. This is the contribution acceptance prerequisite for WP08 and WP09; do not change their status or turn missing external pilot/platform evidence into a pass.

## Ownership and delivery

Own only the declared files and this WP's evidence/status. Planning and merge
target is `feature/beta-readiness`; use the runtime-provided lane workspace.
Do not edit siblings, claim a dependent WP before approval, change existing
release assets or broaden product defaults. Additional integration ownership
requires a canonical planning amendment before edits. Keep original signed
facts and frozen prior-mission evidence byte-exact.

## Package-aware evidence

Use `buildMigrationCLIBinary` or `portableJourneyBinary` for public CLI binaries,
rather than introducing an unconditional source-build helper. Preserve their
closed child environments and explicit package flags so WP06 can exercise these
same journeys against unchanged extracted candidate assets. Distinguish source/
in-process measurements from actual package runtime evidence; no silent fallback
or newly invented release variant may count as a package pass.

## Verification

Run actual public black-box cases with unchanged environment boundaries; verify selected historical payload/signature/ref preservation, no unexpected keys/authority/optional activation, resource refusal before shared visibility and safe retries. Then formatting, full race, vet, build and diff gates. Validate documentation examples against actual CLI and the WP10 contract.

Tests must exercise the promised behavior and detect the intended fault. Do not
add silent skips, weaken security assertions or use internal-only tests as proof
that public CLI integration works. Coordinate expensive full suites with root.

## Review Guidance

Review exact original-content/authority bindings, operation permissions,
default-off behavior, bounded intake before promotion, aggregate many-key
behavior, scoped reversible moderation, raw-history preservation and keyless
reconstruction. Distinguish signature validity, transport verification,
contribution admission and existing proposal acceptance. No missing observation
is a pass. Review the fixed committed head independently.

## Handoff

Commit only expected files and record commands/exits/evidence. Mark only this
WP's truly completed subtasks with the framework. Move to `for_review` and poll
the synchronous gate to completion; never skip/kill it or label no_coverage as a
successful Go run. No external comments, messages or release publication.
