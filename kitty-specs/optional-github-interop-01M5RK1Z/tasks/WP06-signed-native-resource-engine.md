---
work_package_id: WP06
title: Signed native resource engine
dependencies:
- WP03
requirement_refs:
- FR-013
- FR-014
- FR-015
- FR-016
- NFR-001
- NFR-002
- C-001
- C-002
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-github-interop-01M5RK1Z
base_commit: ea90761f5a98d0a5e0286970ff3913982107e537
created_at: '2026-10-25T04:52:51.084687+00:00'
subtasks:
- T024
- T025
- T026
- T027
- T028
- T029
- T030
history: []
agent_profile: implementer-ivan
authoritative_surface: resource_model.go
create_intent:
- resource_model.go
- resource_engine.go
- resource_engine_test.go
- resource_replication_test.go
- docs/resources-v1.md
execution_mode: code_change
lane: planned
owned_files:
- resource_model.go
- resource_engine.go
- resource_engine_test.go
- resource_replication_test.go
- event.go
- issue_model.go
- store.go
- quarantine.go
- shallow.go
- docs/resources-v1.md
- docs/protocol-v0.md
- issue_commands.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec.md and plan.md including confirmed scope expansion, and canonical runtime-generated WP prompt. Resolve implementer-ivan through profile-load skill. Work only in runtime lane; write behavior tests first, implement, commit only owned source/tests/docs, run actual Go charter gates, mark subtasks and move to for_review. Poll synchronous gate completion. No self-approval, no public GitHub mutations or public main merge. User authorized full implementation; no repetitive permission requests.

## Invariants

Native features work with no GitHub/gh/service/Docker. Preserve existing signed bytes and policy authority. New strict native kinds use existing actor histories and verified bounded readers; all cross-resource refs participate in quarantine/shallow closure before promotion. Hostile state is bounded/strict Unicode/duplicate-key checked. Source attribution is not a native signer or authority. No timestamp winner or hidden conflict resolution. Unsupported migration is reported before writes. Read/write operations and resource scope must be explicit.

## Objective and implementation contract

Implement one shared native resource engine. Types are project, project-item, discussion, discussion-reply, workflow. Registered kind prefixes .open/.revise use versioned strict typed state JSON in Event.Body, existing Subject/Parents/Intent/Operation/Request and actor history signing. No new Event fields unless independently justified; no IssueState reuse. Open root has no subject; child open references its container via typed state (and Subject if defined consistently); revisions bind root and exact parent revision IDs. Define/validate all shape rules explicitly. Other Event identity/policy/CI fields must not acquire unintended semantics. New resource fields cannot be smuggled onto old kinds. Authenticated raw bytes capped256KiB except workflow2MiB; definition/source bounds stay below object8MiB. Strict unknown fields, duplicates, invalid Unicode, unsafe identifiers/paths, excessive collections and invalid decimals/dates reject.

ProjectState schema hn.project/1: title/body,archived, <=64 fields {id,name,type:text|number|date|single_select,options<=64}, <=16 views {id,name,layout:table|board,group_by_field?}, metadata. ItemState schema hn.project-item/1: project_id, project_revision, typed target(issue|proposal|discussion|draft|external), optional draft body/title or external URL, <=64 typed values keyed stable field ID, archived, bounded position,metadata. Exact project revision validates types/options; later schemas never reinterpret old items. Native crossrefs full IDs, explicit external URLs descriptive only. Project state schema revisions are independent from item histories.

DiscussionState hn.discussion/1: title/body,category,status open|closed,optional accepted_reply_revision,metadata. ReplyState hn.discussion-reply/1: discussion_id, optional reply_to root,body,metadata. Replies independently revisioned, nesting <=8, same thread and no cycles. Answer pins exact reply revision; later reply edits do not silently change accepted content. Source usernames or lock flags grant no native moderation authority.

WorkflowState hn.workflow/1: name/title/description,enabled,pipeline optional existing PipelineDefinition,source_documents {format,content,digest},metadata. Pipeline validated by existing validatePipeline. Foreign source stays inert; enabled runnable requires valid native pipeline. Bound source documents and verify digest. Workflow event max2MiB, pipeline limit existing1MiB subject total authenticated bound.

Engine queries use collectIssueEvents bounded accepted-history verifier (name historical but validates all). Root+revision projection validates kind/owner/container and DAG, retains multiple heads with null current state, pages stable snapshots. Mutations capture actor/sequence, pin identity, preserve exact expectations, canonical digest and <=128-byte operation keys. Actor-global key reuse across resource/issue kinds must conflict, never leak one kind through another lookup. Operation recovery inspects immutable original input before refreshing expectations. Generic native open/revise/resolve/list/show/heads/history/operation CLI can take full --input, --expect, --actor,--operation,--json. Expose stable helper API promptly to family/transfer WPs. Register default project/discussion/workflow commands through WP03 local extension map and specialization callback lookup (avoid init-order override bugs); family WPs add item/reply/materialize convenience later. Do not edit main.go if registration seam suffices.

ALL admission seams must integrate: validateEventContent/verifyEvent raw bound; validateIssueEvent explicit resource-only carveout; validateEventRelationships; replicationEventDependency/replicationEventReferences; final selectedProjectionEvents validation; guardShallowEventClosure/validateExactEventReferenceClosure. A missing project schema/reply/container ref must quarantine and be recoverable before promotion. New actor facts use existing refs and selections. Tests include invalid cross-type/owner/schema/cycle refs, conflicting heads, cross-actor operation collision, retry, and fresh clone reconstruction. Document new-kind alpha compatibility (old clients refuse unknown kinds), native authority limits and resource command/state contract in resources-v1/protocol docs. Existing Go suite must pass; do not weaken old tests.

## Subtasks

T024 Define strict native resource states and hostile/signature bounds tests.
T025 Implement canonical project/item/discussion/reply/workflow state validation.
T026 Register resource event kinds and actor-global replay request digests without weakening existing validators.
T027 Implement bounded typed projections, independent aggregate heads, conflicts and exact-head mutations/resolution.
T028 Integrate relationship/type/schema/thread dependency validation in reader, quarantine and shallow recovery.
T029 Provide reusable native query/mutation and generic CLI registration seams.
T030 Prove signed cross-actor bare-remote reconstruction, missing dependency quarantine and old-byte preservation; document protocol boundary.

## Review criteria

All referenced requirements have source-bound behavior evidence; no incomplete/unsupported behavior is silently claimed. Full actual Go charter race/vet/build/format/diff gates and relevant tagged checks pass. Independent reviewer must probe hostile, conflict, replay, quarantine and no-provider boundaries.

Integration seam refinement: issue_commands.go is owned for the narrow cross-kind actor-operation query guard. Existing issue operation lookup must not expose new resource records as issue operations; actor-global key collisions remain explicit. No other existing issue CLI behavior changes.
