---
work_package_id: WP05
title: Signed artifact Git storage and accepted reads
dependencies:
- WP03
- WP04
requirement_refs:
- C-006
- FR-010
- FR-011
- FR-014
- FR-015
- FR-017
- FR-018
- NFR-003
- NFR-005
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
subtasks:
- T020
- T021
- T022
- T023
phase: Phase 2 - Portable artifacts
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: implementer-ivan
authoritative_surface: artifact_store.go
create_intent:
- artifact_store.go
- artifact_read.go
- artifact_store_test.go
- artifact_read_test.go
- artifact_store_commands.go
- artifact_store_commands_test.go
- internal/artifact/canonical_export.go
execution_mode: code_change
owned_files:
- artifact_store.go
- artifact_read.go
- artifact_store_test.go
- artifact_read_test.go
- artifact_store_commands.go
- artifact_store_commands_test.go
- internal/artifact/canonical_export.go
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

Signed artifact Git storage and accepted reads.

Real Git object/ref inspection plus tamper/CAS/contribution refusal controls; public CLI caller proof arrives in WP11.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: WP03, WP04. Dependencies mean approved work, not merely drafted files.
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
- Execution command after all gates: `spec-kitty agent action implement WP05 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

- `artifact_store.go` (planned new or dependency-created; create_intent)
- `artifact_read.go` (planned new or dependency-created; create_intent)
- `artifact_store_test.go` (planned new or dependency-created; create_intent)
- `artifact_read_test.go` (planned new or dependency-created; create_intent)
- `artifact_store_commands.go` (planned new or dependency-created; create_intent)
- `artifact_store_commands_test.go` (planned new or dependency-created; create_intent)
- `internal/artifact/canonical_export.go` (planned new; create_intent): two minimal existing-logic wrappers, `Canonical(raw)` and `TypeDeclaration.ValidateOpening(ctx, raw)`, immediately used by this package's live mutation CLI/store. Full unsigned opening validation precedes signer selection and prepared intent; reuse strict parsing, complete record.open payload checks, exact type_id/shape and private schema validation, with safe zero-value refusal. Preserve WP03/WP04-owned source and all limits; no fields-only API, copied parser or speculative SDK.

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T020 — Write additive actor streams with native signer mediation

**Purpose**: Write additive actor streams with native signer mediation.

**Implementation guidance**:
Use exact hn-artifact/1 ref grammar and existing local identity/key mediation. Append signed type/opening/revision objects on separate actor refs using expected-old CAS; no native or memory event kind/ref modification. Do not initialize identity on reads.

**Observable acceptance**:
Actual Git refs and signed objects verify independently; CAS failure does not replace another actor head or corrupt native history.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T021 — Load verified selected artifact histories under read budgets

**Purpose**: Load verified selected artifact histories under read budgets.

**Implementation guidance**:
Collect explicit local and selected accepted remote custom refs; validate chains/envelopes/types before projection. Enforce100000 selected refs/records,8MiB/object,256MiB aggregate and stricter envelope limits during reads. Compare source snapshots around loading.

**Observable acceptance**:
Raw quarantine and unselected refs absent from ordinary reads; invalid chain is verification failure, never empty data or partial success.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T022 — Refuse unsupported contribution writes before private/public effects

**Purpose**: Refuse unsupported contribution writes before private/public effects.

**Implementation guidance**:
Consume integrated selected-contribution-policy detection. Custom writes with selected contribution policy return unsupported_capability before signer use, private intent, object/ref mutation or publication. Native families retain beta admission paths.

**Observable acceptance**:
Selected-policy custom write has no forbidden effects; same custom mutation works without selected policy; no implicit policy selection from stored content.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T023 — Prove storage using real Git and history integrity

**Purpose**: Prove storage using real Git and history integrity.

**Implementation guidance**:
Implement the frozen type.publish and artifact open CLI slices through static WP03 registration now, routing to this shared store/read boundary. Run actual built hn processes against real actors/repos and independently inspect published refs/objects. Test signer mismatch/missing signer, CAS races, unsupported versions/missing schema and selected-policy refusal. Reuse WP01 old history hashes. No storage API is added without a current production caller.

**Observable acceptance**:
Valid writes produce verifiable objects; tampering/invalid chain controls fail; original native/memory bytes and IDs remain identical.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Real Git object/ref inspection plus tamper/CAS/contribution refusal controls; public CLI caller proof arrives in WP11.

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

A new alternate native writer, implicit key initialization or custom-policy bypass violates the mission boundary.

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
