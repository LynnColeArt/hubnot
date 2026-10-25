---
work_package_id: WP09
title: Native workflow authoring and governed execution
dependencies:
- WP06
requirement_refs:
- FR-015
- FR-016
subtasks:
- T039
- T040
- T041
- T042
lane: planned
agent_profile: implementer-ivan
execution_mode: code_change
owned_files:
- workflow_commands.go
- workflow_commands_test.go
authoritative_surface: workflow_commands.go
create_intent:
- workflow_commands.go
- workflow_commands_test.go
history: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

Register workflow specialization over core engine. Workflow states author existing validated hn.pipeline/0 steps and may preserve inert foreign source docs with exact digest. No imported YAML or source prose executes. Help distinguishes native runnable definition, inert source and explicit mapping. Generic full-state input/revision/resolution remains available; show status explains missing native definition. `workflow materialize ID [--expect REV]` writes only .hn/pipelines/<validated-name>.json in current working tree, canonical deterministic bytes, refusing differing existing file unless explicit overwrite flag. Reject symlink/path escape; no automatic Git commit, run or network. Output path/digest/native revision for operator review.

`workflow request ID --proposal FULL_ID` (or equivalent documented grammar) resolves one native revision, checks proposal head exact pipeline bytes equal the canonical validated signed workflow definition/name, then uses existing cmdRunRequest path (or equivalent native function) preserving base-policy maintainer and runner semantics. Can bind workflow revision in existing evidence only if fully validated and documented; do not smuggle semantic authority through unused fields. Request creates no execution; execution remains explicit hn runner/run path. Tests prove mismatched proposal/file/version/disabled/foreign-only workflow refusal, no side effects on invalid materialization, roundtrip native bytes, and successful existing governed CI request with exact digest. Do not change CI policy/runner isolation or add autoexecution.

## Subtasks

T039 Write workflow definition/source/materialize/request safety tests first.
T040 Implement workflow open/list/show/revise/resolve and native/foreign runnable status.
T041 Implement explicit safe materialization of canonical native pipeline into repository pipeline file.
T042 Verify exact workflow/proposal pipeline digest binding before existing governed run request and preserve runner/policy limits.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.
