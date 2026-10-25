---
work_package_id: WP13
title: Optional GitHub workflow migration
dependencies:
- WP09
- WP10
requirement_refs:
- FR-004
- FR-008
- FR-011
- FR-012
- FR-015
- FR-019
- FR-020
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-github-interop-01M5RK1Z
base_commit: ea90761f5a98d0a5e0286970ff3913982107e537
created_at: '2026-10-25T06:22:36.788604+00:00'
subtasks:
- T058
- T059
- T060
- T061
- T062
history: []
agent_profile: implementer-ivan
authoritative_surface: github_workflows.go
create_intent:
- github_workflows.go
- github_workflows_test.go
- internal/githubapi/workflows.go
- internal/githubapi/workflows_test.go
execution_mode: code_change
lane: planned
owned_files:
- github_workflows.go
- github_workflows_test.go
- internal/githubapi/workflows.go
- internal/githubapi/workflows_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

Use !nogithub and neutral bridge. Actions workflow REST API gives metadata/path, not YAML; read source definition via contents at one pinned commit (resolve chosen ref once, record full commit). Write contents via PUT with base64 bytes, explicit destination branch, existing blob SHA on update, commit message. Validate path remains .github/workflows/<safe>.yml|yaml (native pipeline .hn/pipelines path only if explicitly selected format), no traversal/symlink assumptions. Current API permissions require Contents+Workflows write/classic repo+workflow; document, never request/store token in metadata. Respect body/page/time budgets and directory caps with explicit failure. Read/write provider file and workflow enabled state are distinct operations if supported; journal every mutation and use exact blob/content reconciliation after uncertainty.

Import arbitrary Actions YAML EXACT bytes/digest as inert native workflow source document, with runnable=false/enabled semantics requiring explicit native hn.pipeline/0 mapping. This is a first-class editable native workflow with optional source, not implicit execution of YAML. If an explicit native pipeline is supplied via mapping file, validate and record both; no blind conversion of uses/expressions/matrices/services/secrets/triggers/runner semantics. Report unmapped capabilities before any activation/execution. Original exact source may be re-exported unchanged to explicit branch with honest attribution; that is definition preservation, not native execution equivalence.

Provide useful native-authored export for a constrained explicitly mapped subset: require target runner/trigger/checkout or other execution-environment mapping before emitting provider Actions; native direct argv commands must be shell-quoted or otherwise represented without argument injection, working directory/timeouts explicit, no secrets assumed. A deterministic embedded native manifest and exact renderer validation may support roundtrip of our own generated subset without full YAML dependency. Unsupported semantics or precision changes need mapping report/refusal; do not claim general equivalence. Publishing workflow files can enable provider behavior, so dry-run/report must state intended path/branch/trigger; no actual publication in tests.

Tests fake API + native workflows: source UTF8/exact bytes including newlines, pinned source changed later, arbitrary YAML remains inert/materialize refuses, explicit mapping enables native definition, explicit destination branch/SHA mismatch, failed/lost PUT recovery without blind overwrite, repeated restore no duplicate commit if unchanged, native args with quotes/dollars/newlines safely rendered, own subset roundtrip and nogithub core execution unaffected.

## Subtasks

T058 Write exact definition/source/pinned revision and inert-import fixtures.
T059 Implement bounded Actions workflow discovery and contents read/write at explicit commit/branch with blob SHA.
T060 Map native workflow sources and require explicit runnable/environment transformation choices.
T061 Implement exact source restore and constrained native-to-Actions export with clear semantic report/manifest.
T062 Verify no autoexecution, content-CAS conflicts, recovery, roundtrip bytes and transport exclusion.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.

Dependency refinement: GraphQL transport is independently provided by WP14; project adapter uses that shared package. Workflow REST contents migration has no GraphQL dependency. Root discussion/project/workflow adapters may proceed independently once their actual native/bridge/client dependencies are approved.
