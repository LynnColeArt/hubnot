---
work_package_id: WP26
title: Independent boundaries and adopter evidence
dependencies:
- WP25
requirement_refs:
- C-001
- C-004
- C-005
- C-007
- C-008
- FR-001
- FR-004
- FR-017
- FR-024
- NFR-001
- NFR-006
- NFR-007
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
subtasks:
- T104
- T105
- T106
- T107
phase: Phase 5 - Documentation and hard acceptance
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: reviewer-renata
authoritative_surface: docs/qualification/optional-extensions/independent/boundary-review.md
create_intent:
- docs/qualification/optional-extensions/independent/boundary-review.md
- docs/qualification/optional-extensions/independent/native-user.md
- docs/qualification/optional-extensions/independent/extension-author.md
- docs/qualification/optional-extensions/independent/readiness.md
execution_mode: code_change
owned_files:
- docs/qualification/optional-extensions/independent/boundary-review.md
- docs/qualification/optional-extensions/independent/native-user.md
- docs/qualification/optional-extensions/independent/extension-author.md
- docs/qualification/optional-extensions/independent/readiness.md
priority: P1
role: reviewer
tags: []
task_type: review
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the profile specified in frontmatter, and behave according to its guidance before parsing the rest of this prompt. The canonical resolver-backed skill is `spk-doctrine-profile-load`.

- **Profile**: `reviewer-renata`
- **Role**: `reviewer`
- **Agent/tool**: `codex`
- Resolve via `spec-kitty agent profile show reviewer-renata`; load action-scoped charter context.
- No model is prescribed or invented by this prompt.

## Objectives and success criteria

Independent boundaries and adopter evidence.

Independent reviewer and actual native-user/author observations tied to exact package; no invented adopter evidence.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: WP25. Dependencies mean approved work, not merely drafted files.
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
- Execution command after all gates: `spec-kitty agent action implement WP26 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

This WP delivers repository product documentation/qualification evidence in a dedicated execution lane (`code_change` execution mode); it owns no application code. Only WP02 owns mission planning contracts. Collapsing late documentation/review outputs into the single `lane-planning` would create a false lane dependency cycle. Keep the declared role and approval boundaries unchanged.

- `docs/qualification/optional-extensions/independent/boundary-review.md` (planned new; declared in create_intent)
- `docs/qualification/optional-extensions/independent/native-user.md` (planned new; declared in create_intent)
- `docs/qualification/optional-extensions/independent/extension-author.md` (planned new; declared in create_intent)
- `docs/qualification/optional-extensions/independent/readiness.md` (planned new; declared in create_intent)

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T104 — Review host protocol and trust boundaries independently

**Purpose**: Review host protocol and trust boundaries independently.

**Implementation guidance**:
Review raw grants, session repo identity, signer pinning, per-request native gates, closure/unknown data and secret boundaries against final source/contracts. Inspect contract-to-live-caller evidence and deliberate red controls; no source change in this review WP.

**Observable acceptance**:
Every actionable finding gets an owner and re-review; unresolved authority/default/history issues block acceptance.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T105 — Observe a new native user following unchanged native guide

**Purpose**: Observe a new native user following unchanged native guide.

**Implementation guidance**:
Obtain actual independent user observation for init,ticket/discussion,exchange without extension configuration or setup. Record environment/exact package/guide/tasks/friction and consented observation facts only.

**Observable acceptance**:
Implementer replay, CI and hypothetical narrative do not substitute; absence remains pending, never a fabricated success.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T106 — Observe an independent extension author using published guide

**Purpose**: Observe an independent extension author using published guide.

**Implementation guidance**:
Have an independent author follow versioned guide with supplied optional process asset and report real contract/recovery/capability friction. Record actual binary/source/example identities and follow-up dispositions.

**Observable acceptance**:
This is an author trial, not an implementer self-check or production Spec Kitty integration; missing trial remains pending.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T107 — Reconcile final readiness without beta release or main changes

**Purpose**: Reconcile final readiness without beta release or main changes.

**Implementation guidance**:
Trace every FR/NFR/constraint/success criterion to approved WP evidence and exact source. List unresolved platform/user/security/default findings plainly. Hand back to root for canonical mission analyze/accept/merge workflow only when prerequisites/evidence really satisfy it.

**Observable acceptance**:
No beta/pilot/issue23 acceptance is amended; no automatic release/main push; work remains incomplete while any hard gate has missing or failing observations.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Independent reviewer and actual native-user/author observations tied to exact package; no invented adopter evidence.

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
- Required dependency approvals and WP01 baseline remain valid.
- No default native setup, process-start, registry-read, dependency or historical compatibility regression.
- Required live-caller and meaningful negative controls pass for their stated reasons.
- Structured error/recovery semantics and public/private separation match frozen contracts.
- No TODO stub, copied dependency implementation, weakened refusal or silent unsupported success.
- Documentation for owned observable behavior is synchronized; final consumer guide owners receive exact examples.
- Targeted expected files only; no blanket staging, unrelated runtime state or generated binary inclusion.
- Independent reviewer records a real verdict via the canonical review surface.

## Risks and mitigations

This evidence WP cannot repair code or self-approve missing external observations.

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
