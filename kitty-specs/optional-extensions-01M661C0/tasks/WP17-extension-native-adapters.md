---
work_package_id: WP17
title: Native operation adapters with exact recovery
dependencies:
- WP16
- WP11
requirement_refs:
- C-008
- FR-004
- FR-005
- FR-006
- FR-007
- FR-017
- FR-018
- FR-020
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-extensions-01M661C0
base_commit: 8284f4d8ba6e29fcf907285c06ad607da669d674
created_at: '2026-10-31T06:42:05.987433+00:00'
subtasks:
- T068
- T069
- T070
- T071
phase: Phase 3 - Optional host
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: implementer-ivan
authoritative_surface: extension_native_adapter.go
create_intent:
- extension_native_adapter.go
- extension_native_adapter_test.go
- extension_native_acceptance_test.go
- contribution_moderation.go
execution_mode: code_change
owned_files:
- extension_native_adapter.go
- extension_native_adapter_test.go
- extension_native_acceptance_test.go
- contribution_moderation.go
- resource_engine.go
- issue_commands.go
priority: P1
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the profile specified in frontmatter, and behave according to its guidance before parsing the rest of this prompt. The canonical resolver-backed skill is `spk-doctrine-profile-load`.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `codex`
- Resolve via `spec-kitty agent profile show implementer-ivan`; load action-scoped charter context.
- No model is prescribed or invented by this prompt.

## Objectives and success criteria

Native operation adapters with exact recovery.

Raw external stdio to live hn plus real native actors/contribution policy and forbidden-effect controls per family.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: WP16, WP11. Dependencies mean approved work, not merely drafted files.
- WP01 approved default baseline is a mandatory precondition for every other WP, directly or transitively.
- WP02 freezes mechanism details; raise semantic scope changes to root/architect before implementing them.
- Consume the exact approved integrated beta foundation; do not recreate absent dependency files or introduce stubs.
- Native UX, dependencies, default overhead and historical integrity are hard acceptance gates throughout.
- Never parse the extension registry, start/probe modules or write extension state from ordinary native commands.
- Preserve hn/0 and hn-memory/0 bytes, actor/operation/ref meanings and all meaningful native refusals.
- A custom record/type is inert data; signature or search match does not grant policy, execution or prompt authority.
- Selected contribution policy: native adapters retain shared admission/visibility checks; custom writes initially refuse unsupported before effects.
- First-host privileged/governance/memory mutation/identity lifecycle/runner/implicit sync surfaces remain unsupported.
- No Spec Kitty resolver, composition/activation algorithm, runtime, Java service or doctrine-specific production core branches.
- No production enterprise connector, marketplace, scripting runtime, new frontend or extra native dependency.

## Branch strategy

- **Planning base branch**: `feature/optional-extensions`.
- **Final mission merge target**: `feature/optional-extensions`.
- **Strategy**: execution lanes, as computed by canonical finalization in `lanes.json`.
- Root finalizes tasks and resolves exact lane workspace/branch before implementation. WP01 imports its explicitly approved beta source inside that assigned lane.
- Do not invent a WP worktree, branch from main or recreate dependency code.
- Execution command after all gates: `spec-kitty agent action implement WP17 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

- `resource_engine.go` (existing; explicit Context variants and Background compatibility wrappers)
- `issue_commands.go` (existing; explicit Context variants and Background compatibility wrappers)
- `extension_native_adapter.go` (planned new or dependency-created; create_intent)
- `extension_native_adapter_test.go` (planned new or dependency-created; create_intent)
- `extension_native_acceptance_test.go` (planned new or dependency-created; create_intent)
- `contribution_moderation.go` (planned new or dependency-created; create_intent)

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T068 — Adapt public identity and supported native reads

**Purpose**: Adapt public identity and supported native reads.

**Implementation guidance**:
Expose noninitializing public identity and issue/project/item/discussion/reply/workflow-definition reads through existing verified accepted/governed projections. Derive exact inventory statuses and native errors from integrated source; keep other command families explicit unsupported.

**Observable acceptance**:
Missing identity read does not initialize; contribution-hidden content is absent and invalid history errors remain distinguishable from empty reads.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T069 — Route typed writes through shared native authorization boundaries

**Purpose**: Route typed writes through shared native authorization boundaries.

**Implementation guidance**:
Check session binding, action/family/type grants and actual signer pin before private preflight. Own the only contribution_moderation.go integration edits: factor/use contributionCommandLock (currently around516) as the common per-request repository authorization/lock seam and propagate the host parent context through contributionFinishNative (currently around862-871 background-rooted). Preserve direct CLI semantics, admission/reservation/actor checks and stricter native stage deadlines; do not merely call unlocked MutateResource. No alternate writer, arbitrary argv bridge or detached publishing goroutine after definitive cancellation.

**Observable acceptance**:
Raw read-only/wrong-repository/actor-mismatch requests refuse before intent/sign/ref effects; valid governed write preserves direct native outcome/refusal semantics.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T070 — Expose native operation lookup and immutable replay

**Purpose**: Expose native operation lookup and immutable replay.

**Implementation guidance**:
Keep original full actor/key/kind/target/parents/state and existing cross-native-family key scope. Lookup precedes stale-head logic as native contract requires; original actor lookup remains read-only even after active signer changes.

**Observable acceptance**:
Lost issue/comment and each advertised typed-family ack recovers same public result; changed replay input refuses; uncertain after dispatch is never called absent.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T071 — Prove every advertised native family through raw stdio

**Purpose**: Prove every advertised native family through raw stdio.

**Implementation guidance**:
Send handwritten framed requests from an independent external test process for each claimed read/mutation/operation. Cover stale/conflict/resolve grant, enabled-policy admission pending/refused, hidden visibility and unchanged unsafe-host refusals.

**Observable acceptance**:
All six supported families have live-caller success and meaningful denial/replay controls; test parent cancellation before and after publication and competing native command while child is idle. Privileged/memory/proposal/run/transport-apply methods explicitly unsupported.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Raw external stdio to live hn plus real native actors/contribution policy and forbidden-effect controls per family.

- Follow test-first behavior for changed contracts and retain the meaningful initial failure.
- Exercise new producer behavior via the real caller designated above before claiming that boundary accepted.
- Use real temporary Git repositories and supplied external processes where the contract crosses those boundaries.
- Inspect exact structured codes, public identities, bytes/heads and prohibited effects.
- Invalid history is not an empty result; a lost acknowledgement is not proof of absence.
- Each owned published bound has at-bound and one-over cases, with no silent truncation.
- Refusals happen before the prohibited signer/journal/ref/publication stage; assert the corresponding state.
- Missing hardware/user observation remains pending; never fabricate a platform or adopter pass.
- Run focused Go checks for changed code; serialize heavy execution and preserve exact tested source.
- Final fixed-source race/package/default qualification belongs to WP24/WP25 and cannot be inferred here.

## Definition of done

- All listed subtasks satisfy observable acceptance and have reproducible evidence.
- Reviewer checks zero-production-caller debt: every added behavior has a current live CLI/host caller in this slice; no broad unused APIs awaiting future wiring. Pure declaration/value-object code claims only its narrow inert validation capability.
- Required dependency approvals and WP01 baseline remain valid.
- No default native setup, process-start, registry-read, dependency or historical compatibility regression.
- Required live-caller and meaningful negative controls pass for their stated reasons.
- Structured error/recovery semantics and public/private separation match frozen contracts.
- No TODO stub, copied dependency implementation, weakened refusal or silent unsupported success.
- Documentation for owned observable behavior is synchronized; final consumer guide owners receive exact examples.
- Targeted expected files only; no blanket staging, unrelated runtime state or generated binary inclusion.
- Independent reviewer records a real verdict via the canonical review surface.

## Risks and mitigations

Calling an internal native helper without its integrated outer policy/lock boundary would silently bypass authority.

Preserve the source and failure evidence before changing a suspected regression. An implementation gap is a reason to fix or reject the package, not to loosen its acceptance contract. Escalate changes in signed bytes, authority, supported scope or budgets for explicit specification review.

## Reviewer guidance

- Load reviewer-renata through the canonical profile resolver and inspect actual changed code/evidence.
- Check the first native gate and this package dependency approvals rather than relying on a status label.
- Reproduce the most important positive path and intentionally broken control.
- Confirm no test passes by avoiding the production entry point or copying private source state to the destination.
- Inspect grant/refusal ordering, selected contribution behavior and changed replay where relevant.
- Check exact public IDs, source cuts, signer identity and private-marker exclusions.
- A missing platform/adopter observation stays pending even if every local automated check is green.
- Reject actionable defects with concrete file/contract, reproducer, impact and required correction.
- This mission does not change beta acceptance, issue23 independent reporting, release publication or main.

## Review feedback

Read canonical status/review references before implementation. Address every rejection with evidence before requesting another verdict. Append dated activity entries in chronological order; do not treat prose checkboxes as task state.

## Activity log

- 2026-10-30T09:50:45Z — planner-priti — Draft authored; root has not finalized tasks or authorized branch operations.

## Approved-contract ownership follow-through

O1 assigns this package the shared native writer seams in `resource_engine.go` and `issue_commands.go`, alongside `contribution_moderation.go`. Follow `contracts/host-v1.md` section 3: explicit request Context variants, Background wrappers for unchanged native callers, existing per-request command lock and actor/policy/visibility checks, and synchronous writer completion. No global context, copied writer or detached publication. `main.go` remains exclusively WP16-owned.
