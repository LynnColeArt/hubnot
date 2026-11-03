---
work_package_id: WP12
title: Native contribution commands authorization and moderation
dependencies:
- WP11
requirement_refs:
- FR-015
- FR-016
- FR-018
- FR-019
- FR-020
- NFR-002
- NFR-007
- C-001
- C-002
- C-003
- C-007
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-beta-readiness-01M60J8C
base_commit: cd6f8f975c7fb26e6c47c3e1747d02f71de735ea
created_at: '2026-10-29T10:27:14.700679+00:00'
subtasks:
- T039
- T040
- T041
- T042
phase: Optional unified contributions
history: []
agent_profile: implementer-ivan
authoritative_surface: contribution_
create_intent:
- contribution_commands.go
- contribution_moderation.go
- contribution_records.go
- contribution_integration_test.go
execution_mode: code_change
owned_files:
- contribution_commands.go
- contribution_moderation.go
- contribution_records.go
- contribution_integration_test.go
- policy.go
- policy_commands.go
- event.go
- issue_commands.go
- issue_read.go
- issue_projection.go
- resource_engine.go
- proposal.go
- commands.go
- main.go
- replication.go
- quarantine.go
- discussion_commands.go
- project_commands.go
- workflow_commands.go
- governance.go
- resource_transfer.go
- github_workflows.go
- github_discussions.go
- github_projects.go
- audit_protocol.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP12 — Native contribution commands authorization and moderation

## Objectives & Success Criteria

### T039

Wire the shared contribution model/exchange into real native CLI submission, pending-list/inspection and explicit opt-in policy paths without changing absent-configuration behavior.

### T040

Enforce operation-specific admission across code proposals and communications: automatic allowed issue/reply intake, protected edits and moderation, and unchanged native merge/execution authority.

### T041

Implement signed scoped admit/reject, hide/restore and author block/unblock commands and moderated shared views, preserving raw signed inspection and visible conflicts/revocations.

### T042

Exchange and reconstruct contribution/decision state using native Git-backed storage with exact targets and safe retries; validate direct command bypasses, disabled behavior and no implicit execution.

## Context & Constraints

Read the active charter and mission spec, plan, data model and research in
`kitty-specs/beta-readiness-01M60J8C/`. This is the owner's explicit native,
optional unified-contribution amendment. Initial support/compatibility/recovery
work remains first. Load the declared profile through the canonical CLI/API.

Read the WP10 concrete contract and the WP11 bounded exchange before editing existing entrypoints. Implement common permission evaluation and projection; avoid separate ad-hoc policy engines for code, issues and discussions. Include applicable native project/workflow operations or explicitly reject unsupported operations rather than silently bypassing the enabled contribution policy. Initialization/import/upgrade/sync/merge must not enable the workflow, start a listener/worker, fetch unselected content or adopt authority. A new optional policy section must be governed by legitimate existing project authority, with explicit historical-baseline and disable behavior. Native content remains originally signed; an admission receipt or moderator action cannot invent another author's edit, approval, CI result, acceptance, maintainer role or merge. Human review of every ordinary message is not required when explicit rules automatically admit it. The shared view respects authenticated policy/moderation; raw historical inspection remains explicit and immutable. Moderator powers are project-scoped and decisions need exact target/policy provenance and conflict handling, not last-arrival wins. Arbitrary host/Git writers remain outside Hubnot's policy enforcement; do not claim storage ACLs against an authorized host writer. Reuse existing validated readers. Files are a bounded integration ownership allowance: change only entrypoints required by the WP10 design; obtain a planning amendment before editing any undeclared shared surface, especially WP03 store.go. No duplicate source verifier or implicit SHA freezing of mutable project rules.

## Shared integration boundaries

Raw storage authenticates immutable facts; it need not apply mutable project
admission policy. Reuse one contribution graph validator/reducer after raw
signature, actor-chain and native dependency validation. Its operative resource
heads, expected revisions and proposal lineage must derive from admissible
revisions, retaining a separate exact raw-dependency index. Building a raw
catalog and merely hiding rows afterward is insufficient. A permissive rule
never admits arbitrary raw imports during reads: automatic admission must pass
the bounded intake/accounting transaction and retain exact-target evidence.

Route the actual issue/resource/proposal/workflow command seams through the
shared operation evaluator. Cover specialized discussion/project/workflow
readers and classify transfer/provider export and audit workflow-selection
consumers explicitly. Ordinary governed consumers must not bypass moderation;
raw historical inspection/export must be explicit about retaining signed
history. Admission cannot supply CI, reviews, acceptance, roles or execution
permission. An unadmitted proposal needs an explicit live merge precondition if
required by the WP10 contract, preserving all existing merge evidence checks.

The added consumer files are a bounded ownership allowance for these concrete
callers, not an instruction to edit every file. `store.go` remains outside this
WP: policy-neutral raw collection plus shared command authorization/governed
projection is a legitimate design. Do not couple cryptographic encoding to
mutable repository policy or duplicate native source verification. If a real
contract defect requires a shared-store edit, report it before editing and
obtain a canonical planning/allocation resolution; do not force lane checks.

## Ownership and delivery

Own only the declared files and this WP's evidence/status. Planning and merge
target is `feature/beta-readiness`; use the runtime-provided lane workspace.
Do not edit siblings, claim a dependent WP before approval, change existing
release assets or broaden product defaults. Additional integration ownership
requires a canonical planning amendment before edits. Keep original signed
facts and frozen prior-mission evidence byte-exact.

## Verification

Through actual public CLI paths, prove unknown-signer submission without upstream push, configured automatic and manual admission, denied foreign edits, invalid/revoked/conflicting moderation, raw-history preservation, observer reconstruction and retries. Exercise absence of configuration and independence from protected checks/automerge/audits. Imported metadata or direct native commands must not bypass enabled operation rules. Run all charter Go gates and report no_coverage honestly if the framework's Python gate is unavailable.

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
