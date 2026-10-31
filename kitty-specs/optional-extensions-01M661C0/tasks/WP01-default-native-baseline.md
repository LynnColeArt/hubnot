---
work_package_id: WP01
title: Default native baseline and integrated prerequisites
dependencies: []
requirement_refs:
- C-001
- C-002
- C-006
- C-010
- FR-001
- FR-002
- FR-017
- NFR-001
- NFR-002
- NFR-005
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
phase: Phase 0 - Mandatory native gate
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: implementer-ivan
authoritative_surface: replication_borrow_test.go
create_intent:
- extension_default_baseline_test.go
- docs/qualification/optional-extensions/baseline/manifest.json
- docs/qualification/optional-extensions/baseline/native-outcomes.json
- docs/qualification/optional-extensions/baseline/historical-hashes.json
- docs/qualification/optional-extensions/baseline/metrics.csv
- docs/qualification/optional-extensions/baseline/prerequisites.md
- beta_scale_test.go
execution_mode: code_change
owned_files:
- replication_borrow_test.go
- extension_default_baseline_test.go
- docs/qualification/optional-extensions/baseline/manifest.json
- docs/qualification/optional-extensions/baseline/native-outcomes.json
- docs/qualification/optional-extensions/baseline/historical-hashes.json
- docs/qualification/optional-extensions/baseline/metrics.csv
- docs/qualification/optional-extensions/baseline/prerequisites.md
- beta_scale_test.go
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

Default native baseline and integrated prerequisites.

Live native CLI plus original historical inputs; detector red control; healthy and corruption inventory controls; raw baseline metrics.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: none; this is the first gate. Dependencies mean approved work, not merely drafted files.
- WP01 approved default baseline is a mandatory precondition for every other WP, directly or transitively.
- WP02 freezes mechanism details; raise semantic scope changes to root/architect before implementing them.
- WP01 imports the exact approved beta foundation in its assigned lane as specified in T001; all later WPs consume that imported output. No absent dependency stubs or primary checkout mutations.
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
- Execution command after all gates: `spec-kitty agent action implement WP01 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

- `replication_borrow_test.go` (existing; inspect integrated version)
- `extension_default_baseline_test.go` (planned new or dependency-created; create_intent)
- `docs/qualification/optional-extensions/baseline/manifest.json` (planned new or dependency-created; create_intent)
- `docs/qualification/optional-extensions/baseline/native-outcomes.json` (planned new or dependency-created; create_intent)
- `docs/qualification/optional-extensions/baseline/historical-hashes.json` (planned new or dependency-created; create_intent)
- `docs/qualification/optional-extensions/baseline/metrics.csv` (planned new or dependency-created; create_intent)
- `docs/qualification/optional-extensions/baseline/prerequisites.md` (planned new or dependency-created; create_intent)
- `beta_scale_test.go` (planned new or dependency-created; create_intent)

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T001 — Verify the exact approved integrated beta prerequisite

**Purpose**: Verify the exact approved integrated beta prerequisite.

**Implementation guidance**:
In this WP01 assigned execution lane, normally merge exact approved beta source f8c68bebbaa3845d1eaaac02e3638f30dc062bbd as an external approved-source import. Do not alter primary checkout or reconstruct sibling outputs. Exclude incomplete beta WP09 ancestry. Import exact beta_scale_test.go blob 04f9587e36b571779153a8e783072d85ec22f6f3 from c2a2c271, SHA-256 60cd60b0321b837adbab9959cf6b3e082ccd02647b738d0eacf5ea5b13b29f33, without replacing the newer f8 tree with c2. Verify baseline 219 Go inputs = f8 218 plus scale; runtime105 digest 7e77be2491cf1437b9593f7a74da84f50ba59a4f7d3bc2bd9249d950460c6a46. Inspect integrated contribution admission, visible projection, operation recovery and command-lock boundaries; preserve root external foundation-audit evidence.

**Observable acceptance**:
Prerequisite manifest names exact import commit/tree/scale blob/runtime digest, toolchain, Git, host and unresolved independent beta gates. Approved source merge is an explicit import exception to authored-path ownership, not permission to rewrite dependency files. Missing/mismatched foundation stops this WP before baseline claims.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T002 — Correct the Git-dependent issue23 fixture without weakening its contract

**Purpose**: Correct the Git-dependent issue23 fixture without weakening its contract.

**Implementation guidance**:
Independently assess mainObjectsCompleteForBorrowing contract before editing replication_borrow_test.go. Replace reliance on one Git version making malformed physical inventory enumerate with error. Include healthy physical-inventory positive control, direct corrupt-ID refusal and explicit physical-inventory-corruption refusal. Retain missing-promisor/no-fetch checks. Production files are outside this repair.

**Observable acceptance**:
Run the focused original regression and new controls on the actual Git; no skip, blanket allow-error, rewritten production contract or retry-to-green. Preserve actual external macOS Git2.50/2.53 red evidence and exact f8 source-file equality. The local Git2.43 original test may pass; do not invent a local historical failure. If a controlled Git-response fixture reproduces the missing-object/no-error branch, label it source-assisted rather than actual Git2.53 execution. Obtain independent review of the repair.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T003 — Capture unchanged native journeys and historical fixtures

**Purpose**: Capture unchanged native journeys and historical fixtures.

**Implementation guidance**:
Use existing public CLI journeys for init, issue create/read/revise/comment/conflict/exchange; project/item, discussion/reply and inert workflow definitions; memory record/recall/grep/lifecycle; governed proposal/review/CI/decision/merge and unsafe-host refusals; import/export/recovery and native-only cases. Hash selected old bytes, IDs, signatures and refs.

**Observable acceptance**:
Expected outputs/refusals are recorded before extension implementation, with full input digests and historical corpus identity. An existing valid failure blocks baseline approval.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T004 — Build no-loader and forbidden-effect negative controls

**Purpose**: Build no-loader and forbidden-effect negative controls.

**Implementation guidance**:
Add a reusable public-path harness that observes child-start/provider markers, local state and registry-read instrumentation. Prove its detector fails when an explicit control deliberately reads the registry or starts the marker process. Keep test instrumentation distinct from normal startup.

**Observable acceptance**:
A native command succeeds with the deliberate trap installed and no forbidden effects; the matching deliberately violating control fails for the precise asserted cause.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T005 — Record paired release measurements and package identities

**Purpose**: Record paired release measurements and package identities.

**Implementation guidance**:
Capture 30 non-race release observations per representative native command on one physical host and corpus, including elapsed and peak RSS. Record raw samples, commands, binary/source/compiler/variant hashes, OS/CPU/Git/filesystem and measurement method. Keep the baseline executable/corpus reproducible for later paired comparisons.

**Observable acceptance**:
All 30 observations retained per case; no synthetic samples or post-hoc exclusion. Baseline is available for NFR-002 comparison and missing target evidence remains pending.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T006 — Freeze and hand off the baseline for independent approval

**Purpose**: Freeze and hand off the baseline for independent approval.

**Implementation guidance**:
Publish prerequisites.md explaining source import, issue23 disposition, integrated seams, selected golden inputs and evidence limitations. Freeze native command/corpus rows, historical golden hashes and paired-metric method. Future plugin-specific failure configurations cannot be qualified before a host exists; WP24 repeats them against final code. Run the full declared initial baseline gate at the exact imported/fixed source: git diff --check, gofmt validation, go vet, build, and full uncached race suite; serialize heavy execution, retain failures and request independent WP01 review. WP25 repeats these checks at final source and does not waive this initial gate. Prepare the review-ready packet and request for_review; the implementer does not approve this WP. Only a distinct reviewer records approval, which gates every later WP.

**Observable acceptance**:
Reviewer verifies signed goldens/refusal controls and baseline method, not just exit status. Approval is the first mandatory DAG gate for every later work package.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Live native CLI plus original historical inputs; detector red control; healthy and corruption inventory controls; raw baseline metrics.

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

Baseline captured before approved beta integration or after extension implementation would be invalid.

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
