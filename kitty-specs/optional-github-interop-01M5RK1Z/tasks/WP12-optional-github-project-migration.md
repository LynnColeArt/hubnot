---
work_package_id: WP12
title: Optional GitHub project migration
dependencies:
- WP07
- WP10
- WP11
requirement_refs:
- FR-004
- FR-008
- FR-011
- FR-012
- FR-014
- FR-018
- FR-020
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
subtasks:
- T053
- T054
- T055
- T056
- T057
history: []
agent_profile: implementer-ivan
authoritative_surface: github_projects.go
create_intent:
- github_projects.go
- github_projects_test.go
- internal/githubapi/projects.go
- internal/githubapi/projects_test.go
execution_mode: code_change
lane: planned
owned_files:
- github_projects.go
- github_projects_test.go
- internal/githubapi/projects.go
- internal/githubapi/projects_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

Use !nogithub, neutral bridge/native project engine and reusable GraphQL transport from WP11. Projects V2 are user/organization-owned: selectors must explicitly distinguish user:LOGIN/NUMBER or org:LOGIN/NUMBER (or validated global node ID); creation destination owner must be explicit. Never run owner/repo validator on boards. Query owner.projectV2(number)/node then independently page fields,items,fieldValues/views. Native subset text/finite-number/date/single-select and table/board maps with stable field/option IDs; remap external IDs to native schema pinned by exact revision. Richer multi-select/iteration/roadmap/group/sort/automation/permission settings must be explicitly reported manual/unsupported; no silent flattening. Project automation workflow API lacks full trigger/action mutation configuration—never claim migrated automation rules.

Mutations createProjectV2/updateProjectV2/createProjectV2Field, addProjectV2ItemById/addProjectV2DraftIssue, set/clear field values, position/archive state and supported view operations after checking current official schema. Adding item and setting fields are distinct journaled steps. Issue/PR assignees/labels/milestone/repository are underlying content properties, not ordinary custom fields. Native items target already mapped native issues/proposals/discussions, drafts or explicit external descriptive links. Missing mappings/inaccessible targets must surface before writes; use explicit --map content/issue repository mappings rather than guessing destination or granting authority. If auto-importing referenced read-only issues is supported, declare dependency plan and preserve source identity; otherwise explicit mapping refusal is acceptable and documented.

Export native project and independent items: create/schema map IDs first, then content/drafts and field values, then order/archive/view state. Prepared/in-flight/confirmed for every mutation with exact identity/content reconciliation; no negative-scan retries or blind create after uncertainty. Imported source baseline and current schema/item states retain exact refs and reject competing local/remote edits. Preserve source author/time as provenance, not native ACL. Tests fake GraphQL with actual native resources, multiple nested pages, supported value types/options, missing/unsupported preflight zero writes, duplicate mapping refusal, lost responses, repeated export and strict number/date handling. GitHub item limit may exceed local budget; fail with explicit bound instead of truncating.

## Subtasks

T053 Define explicit user/org project selectors and typed field/item mapping fixtures.
T054 Implement complete GraphQL project/field/item/value reads and supported writes.
T055 Map native project schema/items/targets and report unsupported layouts/types/automation before writes.
T056 Implement checkpointed project export/import with field/option/content ID remapping and item ordering/state.
T057 Verify clone independence, reference integrity, repeat/recovery and no false universal workflow claims.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.
