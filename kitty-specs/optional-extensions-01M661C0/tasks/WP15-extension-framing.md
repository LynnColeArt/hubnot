---
work_package_id: WP15
title: Strict framed stdio protocol
dependencies:
- WP13
requirement_refs:
- C-009
- FR-005
- FR-019
- FR-020
- NFR-003
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
subtasks:
- T060
- T061
- T062
- T063
phase: Phase 3 - Optional host
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: implementer-ivan
authoritative_surface: internal/extension/protocol.go
create_intent:
- internal/extension/protocol.go
- internal/extension/frame.go
- internal/extension/protocol_test.go
- internal/extension/frame_test.go
- extension_protocol_commands.go
- extension_protocol_commands_test.go
execution_mode: code_change
owned_files:
- internal/extension/protocol.go
- internal/extension/frame.go
- internal/extension/protocol_test.go
- internal/extension/frame_test.go
- extension_protocol_commands.go
- extension_protocol_commands_test.go
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

Strict framed stdio protocol.

Independent handwritten byte streams, allocation-bound inputs and later real process stdio conformance.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: WP13. Dependencies mean approved work, not merely drafted files.
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
- Execution command after all gates: `spec-kitty agent action implement WP15 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

- `internal/extension/protocol.go` (planned new or dependency-created; create_intent)
- `internal/extension/frame.go` (planned new or dependency-created; create_intent)
- `internal/extension/protocol_test.go` (planned new or dependency-created; create_intent)
- `internal/extension/frame_test.go` (planned new or dependency-created; create_intent)
- `extension_protocol_commands.go` (planned new or dependency-created; create_intent)
- `extension_protocol_commands_test.go` (planned new or dependency-created; create_intent)

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T060 — Implement finite length-delimited strict JSON frames

**Purpose**: Implement finite length-delimited strict JSON frames.

**Implementation guidance**:
Decode4-byte unsigned big-endian length and reject zero/>8MiB before allocating payload. Validate strict UTF-8 object, duplicate keys/unknown envelope fields and request ID ASCII1..128 constraints. Bound writer responses before output.

**Observable acceptance**:
Split/coalesced reads work; truncated/oversize/malformed frames produce finite structured failure without panic or oversized allocation.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T061 — Define typed method routing and closed results/errors

**Purpose**: Define typed method routing and closed results/errors.

**Implementation guidance**:
Use every WP02 typed param/result object and no arbitrary argv bridge. Preserve protocol/request correlation, nullability, session-selected binding, closed host errors and exact native error wrapper context.

**Observable acceptance**:
Unknown method/field/version refuses; request-provided extension/repository strings cannot replace host session identity.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T062 — Support one outstanding request and cancellation semantics

**Purpose**: Support one outstanding request and cancellation semantics.

**Implementation guidance**:
Implement transport state needed for one request/session and explicit cancellation/EOF contract. Keep request IDs session-only and durable actor/operation identity in mutation params; never interpret an ack loss as absent operation.

**Observable acceptance**:
Second outstanding request and duplicate/mismatched response IDs refuse deterministically; EOF unblocks readers within host watchdog contract.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T063 — Create independent raw frame conformance controls

**Purpose**: Create independent raw frame conformance controls.

**Implementation guidance**:
Expose frozen inert extension protocol/frame inspect CLI now through WP13 static dispatcher; no module launch. Execute built hn on hand-encoded prefixes/payloads from independent contract recipes, including partial-frame finite timers. Only add protocol APIs called by the current CLI slice; later child-session capability is not claimed here. Preserve vectors for real external-process WP16/17 conformance.

**Observable acceptance**:
At-bound valid frames pass; manually corrupted length/Unicode/keys and wrong-ID frames fail for precise documented reasons.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Independent handwritten byte streams, allocation-bound inputs and later real process stdio conformance.

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

Transport cancellation is not proof a dispatched mutation had no effect.

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

## Activity Log

- 2026-10-31T18:38:49Z – codex – Maintenance fix 6bd7df03 complete; focused RED/GREEN, fresh race first use, exact frozen typed/frame tests, std and combined CLI and vet pass. Normal for_review gate exited1: pre-existing committed unowned WP07 planning artifact in lane; root notified for ownership-safe cleanup. Gate no_coverage is unverified, not passed. Evidence: /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP15-lazy-validator/review-handoff.txt. No force or skip; no final RSS pass claim.
