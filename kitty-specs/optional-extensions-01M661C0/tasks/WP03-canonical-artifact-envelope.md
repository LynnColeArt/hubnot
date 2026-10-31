---
work_package_id: WP03
title: Canonical signed artifact envelope
dependencies:
- WP02
requirement_refs:
- C-006
- FR-009
- FR-010
- FR-015
- FR-018
- FR-020
- NFR-003
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-extensions-01M661C0
base_commit: 3a72f2aa20b124a912e9ea224333636db7b9a748
created_at: '2026-10-30T23:34:52.719451+00:00'
subtasks:
- T012
- T013
- T014
- T015
phase: Phase 2 - Portable artifacts
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: implementer-ivan
authoritative_surface: internal/artifact/envelope.go
create_intent:
- internal/artifact/envelope.go
- internal/artifact/canonical.go
- internal/artifact/envelope_test.go
- internal/artifact/canonical_test.go
- artifact_commands.go
- artifact_envelope_commands.go
execution_mode: code_change
owned_files:
- internal/artifact/envelope.go
- internal/artifact/canonical.go
- internal/artifact/envelope_test.go
- internal/artifact/canonical_test.go
- artifact_commands.go
- artifact_envelope_commands.go
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

Canonical signed artifact envelope.

Frozen independent byte vectors and signature tamper controls, later reused through live store and public CLI.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: WP02. Dependencies mean approved work, not merely drafted files.
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
- Execution command after all gates: `spec-kitty agent action implement WP03 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

- `internal/artifact/envelope.go` (planned new or dependency-created; create_intent)
- `internal/artifact/canonical.go` (planned new or dependency-created; create_intent)
- `internal/artifact/envelope_test.go` (planned new or dependency-created; create_intent)
- `internal/artifact/canonical_test.go` (planned new or dependency-created; create_intent)
- `artifact_commands.go` (planned new or dependency-created; create_intent)
- `artifact_envelope_commands.go` (planned new or dependency-created; create_intent)
- `artifact_envelope_commands_test.go` (planned new or dependency-created; create_intent)

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T012 — Implement closed envelopes and exact canonical encoding

**Purpose**: Implement closed envelopes and exact canonical encoding.

**Implementation guidance**:
Implement only WP02 frozen hn-artifact/1 types and canonical encoding. Reject duplicate keys, unknown envelope fields, malformed Unicode, invalid IDs/noncanonical encodings and malformed number forms before digest/signature interpretation. Keep original text bytes and digest distinct from structured canonical fields.

**Observable acceptance**:
Published byte vectors match exactly; a one-byte content/signature/preimage alteration fails rather than normalizing into another accepted fact.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T013 — Validate attribution and actor stream envelope continuity

**Purpose**: Validate attribution and actor stream envelope continuity.

**Implementation guidance**:
Validate actor/public-key agreement, full digest identity, signature, sequence/previous and allowed protocol/kinds using proven native primitives without modifying hn/0 or memory encoders. Keep logical opening and revision identity explicit.

**Observable acceptance**:
Forged signer, swapped actor, wrong previous, bad sequence and unknown protocol yield precise diagnostics with no writes.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T014 — Apply bounded parsing at consumption time

**Purpose**: Apply bounded parsing at consumption time.

**Implementation guidance**:
Enforce 1MiB envelope, declaration128KiB and all relevant canonical depth/count limits before unbounded allocation/spooling. Return typed limits and unsupported diagnostics consumed by later public adapters.

**Observable acceptance**:
At-bound accepted and one-over rejected for each owned limit; malformed nested input cannot bypass bounds or become a panic.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T015 — Create independent signed envelope conformance vectors

**Purpose**: Create independent signed envelope conformance vectors.

**Implementation guidance**:
Generate real signatures/IDs from test-only keys using WP02 recipes. Add static no-I/O artifact root/subcommand registration and the frozen inert envelope validate/inspect CLI slice now; exercise canonical/signature behavior through a built hn process with handwritten valid/invalid signed inputs. Expose only APIs currently called by that slice; later store capability is not claimed here.

**Observable acceptance**:
Existing native/memory golden hashes unchanged. Mutation tests include invalid-signature and altered-content controls that actually fail.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Frozen independent byte vectors and signature tamper controls, later reused through live store and public CLI.

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

Reusing native utilities must not share mutable wire format or relax existing unknown-kind validation.

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


### Prospective final-test maintenance transfer — 2026-10-31

The original implementation and independent review of `artifact_envelope_commands_test.go` remain historical evidence of this WP. Root assigns future edits to those test files exclusively to WP25 for the bounded config-path/private-state detector repairs described in `WP25-exact-package-qualification/integration-config-path-maintenance.md`. The historical ownership list in this body describes original delivery; the current frontmatter is the exclusive prospective write scope. No product source, requirement, dependency, prior verdict, signed data or test assertion is removed. WP25 must fix and independently qualify the tests before actual native platform acceptance.
