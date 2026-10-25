---
work_package_id: WP10
title: Provider-neutral native resource migration bridge
dependencies:
- WP06
- WP03
- WP04
requirement_refs:
- FR-011
- FR-012
- FR-016
- FR-020
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
subtasks:
- T043
- T044
- T045
- T046
- T047
history: []
agent_profile: implementer-ivan
authoritative_surface: resource_transfer.go
create_intent:
- resource_transfer.go
- resource_transfer_test.go
- internal/forge/resources.go
- internal/forge/resources_test.go
execution_mode: code_change
lane: planned
owned_files:
- resource_transfer.go
- resource_transfer_test.go
- internal/forge/resources.go
- internal/forge/resources_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

Build neutral resource bridge over approved core and existing issue transfer. Resource kinds issue/project/discussion/workflow remain distinct; child replies/items included as bounded dependency graphs, not flattened opaque records. Define pure internal/forge resource DTO/interfaces or explicit handler registry contracts (narrow optional interfaces rather than mandatory methods every provider can't support), no HTTP/GraphQL dependency. Generic plan/report classifies mapped, embedded/preserved, requires mapping/manual, unsupported, missing/inaccessible dependencies and identity conflicts. Canonical transfer command --resource selects typed handler; explicit repeated --map maps provider-specific category/field/content/branch/runner choices. Unsupported kind/missing mapping fails before network mutation; no silent dropping data. Preserve issue route/aliases and public grammar.

Core helpers discover existing typed native mappings from immutable signed resource history, not only editable latest metadata. Baselines are bound to verified import observations; preserve local edits on unchanged remote, guarded update when local unchanged, reject competing changes, recover exact pending mutation without refreshing expectations. Source identity includes provider/endpoint/type/opaque stable ID; mutable project/selector isn't immutable identity. Use core operation replay and independent child roots; container/schema/reply/answer references must be remapped to exact native IDs/revisions. Preflight full bounded import graph and all states before first mutation; interrupted multi-event import is resumable, not atomic. Missing or unsupported external targets require explicit mapping or typed descriptive external reference, never guessed native authority.

Export helpers provide exact selected native snapshot/child graphs/attribution, preserving conflict refusal and native-only content. Reuse existing private checkpoint/lock IO under Git directory (one local Git-dir publisher), separate native baseline and exact transmitted remote baseline, prepared/in-flight/confirmed intents, complete positive content/identity reconciliation after uncertainty; negative scan never blindly retries creation. Each external mutation journaled, GraphQL clientMutationId isn't guaranteed dedup. Plans/journals bounded, no tokens. Expose helper API quickly to optional resource WPs; no production remote calls needed. Test a fake non-GitHub provider with nonnumeric keys, reference remapping and offline native Git roundtrip for all four families. Existing native Git transport is canonical portable migration; no provider/service dependency.

## Subtasks

T043 Define neutral multi-resource transfer plan/reference/capability contracts and fake-provider tests.
T044 Implement typed native resource import/export graph helpers with source baselines and dependency mappings.
T045 Integrate registry dispatch by provider/resource while preserving issue path and generic dry-run/mapping reports.
T046 Implement bounded journal/reconciliation primitives shared by resource adapter mutations.
T047 Verify non-GitHub opaque identities, cross-resource remapping, unsupported/no-write and no-core-provider dependency.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.
