---
work_package_id: WP02
title: Freeze portable artifact and host contracts
dependencies:
- WP01
requirement_refs:
- C-006
- C-008
- FR-004
- FR-005
- FR-007
- FR-008
- FR-009
- FR-010
- FR-011
- FR-012
- FR-014
- FR-015
- FR-016
- FR-019
- FR-020
- FR-024
- NFR-003
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
subtasks:
- T007
- T008
- T009
- T010
- T011
phase: Phase 1 - Contract freeze
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: architect-alphonso
authoritative_surface: kitty-specs/optional-extensions-01M661C0/contracts/artifact-v1.md
create_intent:
- kitty-specs/optional-extensions-01M661C0/contracts/conformance-fixtures.md
execution_mode: planning_artifact
owned_files:
- kitty-specs/optional-extensions-01M661C0/contracts/artifact-v1.md
- kitty-specs/optional-extensions-01M661C0/contracts/host-v1.md
- kitty-specs/optional-extensions-01M661C0/contracts/default-workflow.md
- kitty-specs/optional-extensions-01M661C0/contracts/conformance-fixtures.md
priority: P1
role: architect
tags: []
task_type: plan
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the profile specified in frontmatter, and behave according to its guidance before parsing the rest of this prompt. The canonical resolver-backed skill is `spk-doctrine-profile-load`.

- **Profile**: `architect-alphonso`
- **Role**: `architect`
- **Agent/tool**: `codex`
- Resolve via `spec-kitty agent profile show architect-alphonso`; load action-scoped charter context.
- No model is prescribed or invented by this prompt.

## Objectives and success criteria

Freeze portable artifact and host contracts.

Independent read-through can derive complete wire bytes and send handwritten valid/invalid requests without source-private assumptions.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: WP01. Dependencies mean approved work, not merely drafted files.
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
- Execution command after all gates: `spec-kitty agent action implement WP02 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

- `kitty-specs/optional-extensions-01M661C0/contracts/artifact-v1.md` (existing; inspect integrated version)
- `kitty-specs/optional-extensions-01M661C0/contracts/host-v1.md` (existing; inspect integrated version)
- `kitty-specs/optional-extensions-01M661C0/contracts/default-workflow.md` (existing; inspect integrated version)
- `kitty-specs/optional-extensions-01M661C0/contracts/conformance-fixtures.md` (planned new; declared in create_intent)

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T007 — Pin signed artifact bytes, identities and ref grammar

**Purpose**: Pin signed artifact bytes, identities and ref grammar.

**Implementation guidance**:
Specify every envelope field, allowed kind, canonical JSON/Unicode/number rule, signature preimage, actor/public-key binding, digest computation, sequence/previous semantics and exact local/accepted-remote ref grammar. Define opening, successor, type publication and conversion payloads plus required public closure. Preserve existing protocols byte-for-byte.

**Observable acceptance**:
Independent reviewer can calculate canonical bytes, signature preimage, full IDs and ref names from a written vector without consulting a future implementation.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T008 — Pin pure schemas, collections and diagnostic boundaries

**Purpose**: Pin pure schemas, collections and diagnostic boundaries.

**Implementation guidance**:
Enumerate the exact allowed schema keywords/value types and bounds, namespaced type grammar, reserved names, ordered member identity/order rules, relation target modes, unknown-version behavior, missing dependency treatment and generic-versus-semantic validity. Pin relationship and conversion-source closure rules. Explicitly distinguish opaque/quarantine preservation and diagnostic export from verified accepted promotion; define required versus optional/exact/logical dependency categories, conflicted target selection and cyclic closure rules.

**Observable acceptance**:
Document positive and one over-limit/unsupported/malformed vector for every published limit. Unknown semantics remain preserved data with explicit unsupported validity, never silently accepted semantics.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T009 — Pin every host request, response, registration and grant shape

**Purpose**: Pin every host request, response, registration and grant shape.

**Implementation guidance**:
Expand every advertised method into strict typed parameter/result objects, nullability, required/optional fields and errors; include manifest/version negotiation, canonical repository binding, executable identity/replacement, grant grammar, pending intent and recovery envelope. Pin cancellation signalling with one outstanding request, timer start for partial prefixes/payloads/startup/blocked reads, disable ownership and post-dispatch uncertainty. Specify parent-context propagation through contributionFinishNative (currently background-rooted), common per-request contributionCommandLock/actor checks and stricter reservation/stage deadlines. No detached goroutine may publish after a definitive refusal.

**Observable acceptance**:
A handwritten independent consumer can implement all methods without guessing. Closed host errors and exact integrated-beta native error examples/recovery classes are enumerated.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T010 — Pin observation, CLI and provenance conformance scenarios

**Purpose**: Pin observation, CLI and provenance conformance scenarios.

**Implementation guidance**:
Define captured/page/since result fields, view fingerprint inputs, invalidation/expiry, duplicate identities and bounded rescan. Pin either explicit visibility transitions or atomic replacement-snapshot completion/removal reconciliation, durable checkpoint advancement, interrupted rescans and hidden/restored resources without edits. Capture-capacity refusal needs honest bounded explicit partition/reselection guidance; a smaller page does not fix a >10000-entry/16MiB captured set. Pin artifact and extension command spellings/help/JSON contract; define bounded descriptive comment/source markers without forge trust. Clarify static command registration and no registry I/O on native paths.

**Observable acceptance**:
Examples include contribution visibility transitions, conflict/stale/replay results, verified absence versus uncertainty, frame-too-small full-state guidance and public/private export exclusions.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T011 — Publish implementation vectors for independent contract review

**Purpose**: Publish implementation vectors for independent contract review.

**Implementation guidance**:
Publish literal complete independent vectors, not recipes alone: real public keys/signatures, exact canonical signed bytes/signature preimages, exact computed full IDs/digests, local/remote refs and externally observable outcomes. Include actual signed non-doctrine declarations/revisions and malformed/tamper vectors. Recipes supplement these bytes and never replace them. Independent reviewer recomputes signatures/digests/ref names without using the implementation serializer. Enumerate all limits and live-caller negative controls. Resolve all P1/P2 findings in the external contract-review.md, without severity downgrades: local mutation head checks versus imported immutable concurrent forks; actor-stream CAS versus multi-actor forks; type.publish and record operation-key scope; immutable request digest; replay-before-current-head checks; verified absence versus incomplete/publication-uncertain history. Require independent signed vectors for type publish replay, two-clone fork import, stale resolution after third head, changed replay schema/parents/content, pre/post-publication interruption, partial frames, visibility removal and opaque unknown preservation. Prepare mechanism details for root/architect and independent reviewer; the author does not self-approve the contract freeze; any semantic change goes back to spec/plan review before implementation.

**Observable acceptance**:
Contract-only WP approved before model, storage, registration or framing WPs. No application production code or fabricated test pass; literal vector bytes are independently verified and complete enough for later executable conformance.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Independent read-through can derive complete wire bytes and send handwritten valid/invalid requests without source-private assumptions.

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

- All P1/P2 findings in `/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-planning-20261030/contract-review.md` are explicitly resolved and independently reviewed before affected code may start.

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

Method names and illustrative digest placeholders are insufficient contracts; mechanisms may be pinned, but scope changes require root review.

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
