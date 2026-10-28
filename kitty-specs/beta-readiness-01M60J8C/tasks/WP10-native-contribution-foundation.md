---
work_package_id: WP10
title: Native contribution policy and signed record foundation
dependencies:
- WP04
requirement_refs:
- FR-015
- FR-018
- FR-019
- FR-020
- NFR-007
- C-001
- C-002
- C-003
- C-007
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-beta-readiness-01M60J8C
base_commit: ab24ccff55d481705fd8291d2ed0feec025162a6
created_at: '2026-10-28T11:45:36.714681+00:00'
subtasks:
- T030
- T031
- T032
- T033
- T034
phase: Optional unified contributions
history: []
agent_profile: implementer-ivan
authoritative_surface: internal/contribution/
create_intent:
- internal/contribution/model.go
- internal/contribution/policy.go
- internal/contribution/record.go
- internal/contribution/model_test.go
- internal/contribution/policy_test.go
- internal/contribution/record_test.go
- docs/contribution-contract.md
execution_mode: code_change
owned_files:
- internal/contribution/model.go
- internal/contribution/policy.go
- internal/contribution/record.go
- internal/contribution/model_test.go
- internal/contribution/policy_test.go
- internal/contribution/record_test.go
- docs/contribution-contract.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP10 — Native contribution policy and signed record foundation

## Objectives & Success Criteria

### T030

Specify the native contribution/decision contract, authority source, concrete CLI/wire examples and existing integration seams in docs/contribution-contract.md.

### T031

Implement shared typed submission identity, exact original-content/revision bindings, scoped signed decision records and strict bounded format validation in internal/contribution/.

### T032

Implement one optional operation-permission evaluator distinguishing transport verification, submission admission, author edits/replies, moderation and existing code acceptance/merge.

### T033

Define deterministic replay, concurrent moderation, revocation, historical-baseline and disable/re-enable semantics without rewriting original facts or granting authority.

### T034

Verify model/policy/signature contracts, disabled configuration and invalid authority with independent tests; identify any additional live integration file ownership before downstream implementation.

## Context & Constraints

Read the active charter and mission spec, plan, data model and research in
`kitty-specs/beta-readiness-01M60J8C/`. This is the owner's explicit native,
optional unified-contribution amendment. Initial support/compatibility/recovery
work remains first. Load the declared profile through the canonical CLI/API.

Use existing native identities, Git content addressing and legitimate project governance. Decide the exact format/CLI surface from the existing code and record examples before downstream implementation; do not invent a second mandatory service or separate identity/account system. Model ordinary issue/reply automatic admission as an explicit permissible rule, not a compulsory approval queue. A valid signature identifies a key, not one person or spam-free content. Moderation targets exact original resource revisions, actors and project scope; decisions retain evaluated authority provenance without permanently pinning ordinary rules. Imported/source policy must never self-adopt, and changing the opt-in requires legitimate project authority. New format fields must be omitted when disabled and existing historical readers/bytes must retain the declared compatibility behavior. This WP delivers the common foundation, not a claim that the end-to-end workflow already works. Map the actual public read/write/proposal/replication callers before WP12, and obtain a canonical planning amendment for any undeclared shared source file; do not touch WP03 store.go without that assignment.

## Integration design obligation

Concretize the separation between authenticated raw facts, operation authority,
and governed admission/visibility. A shared reducer can validate contribution
relationships and authority after native raw authentication without changing
`store.go`; choose layers for correctness, not file-allocation convenience.
Derive operative heads, expected revisions and lineage from admitted revisions,
while retaining exact raw dependencies. Do not merely hide rows in an already
computed raw catalog, or automatically admit arbitrary raw imports during
reads. Automatic admission uses the same durable bounded intake accounting.
Map specialized readers, transfer/provider exports, audit workflow selection
and native proposal acceptance/merge as well as generic command handlers.
Classify explicit raw inspection/export so immutable history is not mistaken
for moderated views. Keep cryptographic encoding independent of current policy.
The downstream ownership allowance covers identified disjoint consumer files;
any actual shared-store need requires a further canonical amendment before edits.

## Ownership and delivery

Own only the declared files and this WP's evidence/status. Planning and merge
target is `feature/beta-readiness`; use the runtime-provided lane workspace.
Do not edit siblings, claim a dependent WP before approval, change existing
release assets or broaden product defaults. Additional integration ownership
requires a canonical planning amendment before edits. Keep original signed
facts and frozen prior-mission evidence byte-exact.

## Verification

Exercise exported Go boundaries for strict signed formats, altered revision/signature/scope, invalid or stale authority, conflicting decisions, operation rules and absent configuration. Run charter Go gates at a committed head; foundation review must identify downstream consumers and must not count tests-only wiring as a working public workflow.

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
