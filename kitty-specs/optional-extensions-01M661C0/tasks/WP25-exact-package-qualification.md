---
work_package_id: WP25
title: Exact package platform and build qualification
dependencies:
- WP24
requirement_refs:
- C-002
- FR-014
- FR-019
- FR-024
- NFR-003
- NFR-004
- NFR-006
- NFR-007
- NFR-008
planning_base_branch: feature/optional-extensions
merge_target_branch: feature/optional-extensions
branch_strategy: Planning artifacts for this mission were generated on feature/optional-extensions. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/optional-extensions unless the human explicitly redirects the landing branch.
subtasks:
- T100
- T101
- T102
- T103
phase: Phase 5 - Documentation and hard acceptance
history:
- at: '2026-10-30T09:50:45Z'
  actor: planner-priti
  action: Drafted tasks; not finalized or committed
agent_profile: implementer-ivan
authoritative_surface: extension_package_acceptance_test.go
create_intent:
- artifact_envelope_commands_test.go
- artifact_schema_commands_test.go
- artifact_read_test.go
- artifact_replication_test.go
- artifact_search_acceptance_test.go
- extension_enablement_test.go
- extension_changes_acceptance_test.go
- extension_charter_acceptance_test.go
- extension_package_acceptance_test.go
- docs/qualification/optional-extensions/packages/manifest.json
- docs/qualification/optional-extensions/packages/platform-matrix.json
- docs/qualification/optional-extensions/packages/checks.md
- .github/workflows/optional-extensions-qualification.yml
execution_mode: code_change
owned_files:
- artifact_envelope_commands_test.go
- artifact_schema_commands_test.go
- artifact_read_test.go
- artifact_replication_test.go
- artifact_search_acceptance_test.go
- extension_enablement_test.go
- extension_changes_acceptance_test.go
- extension_charter_acceptance_test.go
- extension_package_acceptance_test.go
- docs/qualification/optional-extensions/packages/manifest.json
- docs/qualification/optional-extensions/packages/platform-matrix.json
- docs/qualification/optional-extensions/packages/checks.md
- .github/workflows/portability.yml
- .github/workflows/optional-extensions-qualification.yml
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

Exact package platform and build qualification.

Exact distributed-candidate binary execution and fixed-source full suite; platform rows cannot be inferred from cross compilation.

This work is complete only when the observable outcomes below are demonstrated and independently reviewed. A helper-only test or generated report is not equivalent to live-caller evidence.

## Context and constraints

- Mission: `kitty-specs/optional-extensions-01M661C0`; read `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md` and the three contracts.
- Required approvals: WP24. Dependencies mean approved work, not merely drafted files.
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
- Execution command after all gates: `spec-kitty agent action implement WP25 --agent codex --mission optional-extensions-01M661C0`.
- Runtime commands must always pass this mission selector.
- The required analysis-report readiness gate precedes implementation.
- This draft does not itself claim finalization, lane allocation, commit or test pass.

## Ownership and integration

- `.github/workflows/portability.yml` (existing; narrowly scoped explicit manual selector/delegation)
- `.github/workflows/optional-extensions-qualification.yml` (new reusable workflow for explicit extension qualification)
- `extension_package_acceptance_test.go` (planned new; declared in create_intent)
- `docs/qualification/optional-extensions/packages/manifest.json` (planned new; declared in create_intent)
- `docs/qualification/optional-extensions/packages/platform-matrix.json` (planned new; declared in create_intent)
- `docs/qualification/optional-extensions/packages/checks.md` (planned new; declared in create_intent)

No other WP owns these paths. Existing foundation surfaces are dependencies, not permission to recreate them. If a small out-of-map edit is indispensable, coordinate root first with exact path/rationale and preserve the no-overlap execution invariant.

## Subtasks and detailed guidance

### T100 — Build fixed-source standard and combined native-only packages

**Purpose**: Build fixed-source standard and combined native-only packages.

**Implementation guidance**:
Use approved exact release toolchain and existing release tooling without adding dependencies. Build standard and combined nogithub,noforge native-only variants; keep optional reference executables separate supplied assets.

**Observable acceptance**:
Every package has source/tree/compiler/variant/archive/executable hash identity. No Go/Python/Java/MCP/model/provider setup becomes required for native users.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T101 — Run appropriate fixed-head Go checks in serialized heavy execution

**Purpose**: Run appropriate fixed-head Go checks in serialized heavy execution.

**Implementation guidance**:
Run gofmt validation, vet/build and full uncached race suite at the fixed final source, retaining commands/results and original failure causes. Serialize heavy local builds/tests to preserve meaningful timing and resource behavior.

**Observable acceptance**:
Exact checked source recorded; known regression failures are resolved or block approval, never skipped or relabelled flakes.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T102 — Execute exact packages on each claimed target and variant

**Purpose**: Execute exact packages on each claimed target and variant.

**Implementation guidance**:
Run native/default, public artifact, optional host/reference and portability journeys on Linux/macOS AMD64/ARM64 for standard/combined native packages using actual candidate binaries, not substitute source rebuilds.

**Observable acceptance**:
Each matrix row links actual execution/host/binary evidence; unavailable hardware or package runs remain pending and block claimed full support.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

### T103 — Reconcile limits private-state and clone evidence

**Purpose**: Reconcile limits private-state and clone evidence.

**Implementation guidance**:
Audit all published size/count/depth/time limits with at-bound/over-limit live caller cases, child cleanup/failure isolation, secret sentinels and both consumers independent clone reconstruction. Check no new module/runtime dependency.

**Observable acceptance**:
No row accepted from helper-only test, silent truncation, source-assisted substitute or missing negative control; unresolved gaps have explicit owners.

**Evidence and controls**:
- Start from frozen public contracts and the actual integrated source, not assumptions about earlier branches.
- Record exact input, source/binary identity, expected outcome and observed result.
- Include a valid positive case and a negative control that fails for the asserted contract violation.
- Compare relevant signed refs, objects and private-state markers before and after any refused operation.
- Keep tests at the public boundary named in this WP; producer tests must be followed by the designated live caller.
- Do not replace error assertions with exit-status-only checks, blanket allowed errors or skipped cases.
- Record the subtask result through canonical event-sourced `mark-status`; do not add completion checkboxes.

## Verification strategy

Exact distributed-candidate binary execution and fixed-source full suite; platform rows cannot be inferred from cross compilation.

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

Missing platform evidence remains pending; this WP cannot waive support claims or publish a release.

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

## Exact-package qualification workflow ownership

Use the already registered `portability.yml` manual entrypoint at the reviewed feature ref to explicitly select extension qualification and call the new reusable workflow at the same commit. The selector defaults to existing beta behavior; extension selection suppresses all beta manual candidate validation/build/reuse/native jobs. Preserve existing PR/push behavior. No default-branch registration or main change is part of this mission. Verify actual dispatch inputs and record caller/workflow SHA separately from candidate source SHA. Execute all eight actual platform/variant candidate binaries without source-build fallback, with separately supplied exact-source target-bound reference executables. Retain source/toolchain/archive/executable/asset identities for each row. The same Go/Git journeys must remain callable outside hosted CI; no module or compiler becomes a native-user runtime dependency.


## Mandatory test-only maintenance before final qualification — 2026-10-31

Root exclusively transfers the following existing lane test files into this WP’s future maintenance scope: `artifact_envelope_commands_test.go`, `artifact_schema_commands_test.go`, `artifact_read_test.go`, `artifact_replication_test.go`, `artifact_search_acceptance_test.go`, `extension_enablement_test.go`, `extension_changes_acceptance_test.go`, `extension_charter_acceptance_test.go`. Their original authorship and independent reviews remain intact. `create_intent` permits integrating these paths absent from the primary planning checkout; it does not claim this WP originally created them. Read `WP25-exact-package-qualification/integration-config-path-maintenance.md` before editing. Fix actual OS config isolation, causal registry detectors and the stale source-identity absence assertion without changing production bytes, weakening refusals or overwriting prior evidence. Verify affected standard/combined/race source tests and all eight actual native package rows. No prior Linux result substitutes for the corrected Mac checks. These repairs are mandatory T101–T103 work, not a gate waiver.

## Bounded record-count preflight and new-source requalification — 2026-10-31

Root prospectively authorizes only the production region and separate test-only driver guard documented in `WP25-exact-package-qualification/integration-count-preflight.md` and its regions/recapture JSON. The production exception is `artifact_read.go` (WP05 approved/inactive): a bounded selected-immutable-tip unique commit-count preflight at `loadArtifactHistoryFromSources`, with safe old raw-parent fallback for shallow/grafted views and unchanged full verification, source witnesses, existing count defense and30-second stage. This supersedes only the prior production-no-edit prohibition for that exact region; no general reader refactor or ownership transfer. Original14 owned paths and create_intent remain unchanged. Existing owned `artifact_read_test.go` and `extension_package_acceptance_test.go` test the actual public count boundary and causal99999 control.

A separate TEST-ONLY exception in `extension_default_performance_test.go` (WP24 approved/inactive) adds ONLY `extension_package_acceptance_test.go` to its closed source-input union:317=311inherited+5WP24+1WP25. Preserve311 cardinality and equality of EVERY inherited hash against a NEW root-attested map. Sample selection, flags, budgets, capture math, failure retention, collectors and detector/budget controls stay exact. Any further source path/count/fix requires root coordination; no automatic scope growth.

Frozen intermediate0886fe80d8de29a5df3b880cf5e8cc6864ce3afd and its hosted run/evidence remain immutable. Its real100000/100001 calls both cancelled at30s, leaving the numeric record-count gate unqualified. The new actual100000 call must causally pass the count gate (later independent30s verification cancellation may be reported narrowly);100001 must produce exact record-count resource_limit with unchanged protected state, and a99999 threshold control must change the equality outcome. Counting grants no trust and never skips subsequent verification. Graph errors never become empty/success; multiple-parent and mixed-invalid error precedence must be independently reviewed.

Production bytes will change, invalidating any reuse claim based on production equivalence to historical WP24. Preserve WP24's original approval/review/evidence as source-specific history. Require independent source-only correction review and NEW root317-input/source attestation/freeze before fresh affected default/history/no-registry/no-effect checks and the complete unchanged paired performance gate, in addition to the new frozen-source full suite and all eight exact native package rows. Preserve every old result and absent observation; no count-bound success shortcut, longer deadline, budget/dependency/API/authority/error/data/wire/flag change, completion/verdict/lifecycle edit or release/main action is authorized here.

## Metadata-first bounded raw-parent preflight refinement — 2026-10-31

Root accepts the independent source-only P1 rejection of intermediate79c3cf1e793136a57329450a323a084c40bbeae2: rev-list may inflate/parse oversized reachable commits before the existing8MiB metadata gate. The30-second stage, count ceiling and stdout cap do not bound that earlier allocation. Preserve H79c3, all causal source/inputs/results, earlier decisions and review as immutable intermediate history; none becomes an accepted source by this amendment. Read `WP25-exact-package-qualification/integration-count-preflight-bounded-batch.md` and its regions/verification JSON.

Root refines ONLY the existing private artifact_read.go count-preflight region (WP05 approved/inactive) to metadata-first bounded raw-parent counting with ONE scoped real Git `cat-file --batch-command` session WITHOUT `--buffer`, for nonempty selection only. This expressly supersedes the earlier no-persistent-preflight-process/no-batch restriction and explicit shallow/graft bypass mechanism ONLY inside that count helper. Raw parent bytes preserve graph semantics directly even in shallow/grafted repositories; this is no new capability. artifactObjectReader, projectGit, other callers and business bodies remain unchanged. Direct necessary imports and private helper session type/methods in the same file are allowed; no new Go path, dependency, global init or ownership transfer.

Require exact info OID/type/nonnegative size and8MiB per-commit/256MiB UNIQUE-commit-subset guards BEFORE requesting contents or body allocation, then repeated contents header/OID/type/size, exact body+LF and raw hash BEFORE sole-parent interpretation. Bound headers, metadata, input/output, stderr and pending work; preserve unique captured-tip ancestry100001 ceiling, old count guard, full verifier, witnesses,30seconds, scrubbed/no-replace/no-fetch environment and exact public/private/authority semantics. ONE lifecycle owner closes/cancels/kill+Wait-reaps the exact child/pipes on EVERY return; no detached work. Unsupported Git batch capability/compatibility policy requires root coordination; record actual qualified Git versions without changing default Git2.x dependency wording or implying coverage of all2.x versions.

Existing owned artifact_read_test.go and extension_package_acceptance_test.go add8MiB/+1 and safely bounded9MiB BEFORE-BODY causal controls, real-Git protocol/hash/short-body/framing/cancellation/reaping/no-fetch cases, and the same real100000/100001/99999/removal count controls. Supplementary peer stage assertions do not substitute for real Git/public binary evidence; faults must fail for their asserted stage and no OOM probe is authorized. Original14 owned paths, all graph/tasks/contracts and the prior TEST-ONLY closed317 guard exception stay unchanged. Fresh independent source-only review and root311+317 attestation/freeze still precede unchanged affected default/history/no-registry checks, complete4,320-observation paired performance, full-race/source suite and all eight exact native package rows. No equivalence reuse, performance/platform pass, subtask/verdict/lifecycle advance, flag/budget/data/authority/error-policy change or release/main action is authorized here.

## Prospective bounded artifact I/O and qualification correction — 2026-11-01

Root adopts only the exact future integration-artifact-io.md/regions/verification records beside WP25's prior bounded-batch addendum, after complete reap of the H12f8 full source run. WP25 T101–T103 execute five narrow production exceptions under unchanged original owners: artifact_read.go/artifact_store.go (WP05), artifact_closure.go (WP08), artifact_replication.go (WP09), artifact_exchange.go (WP10). An additional TEST-ONLY exception replaces only inventoryHostShapeSearch's unordered separate-ID fetch loop in extension_inventory_acceptance_test.go (WP22) with one deterministic sorted fetch of all three exact final converted IDs; preserve every search/read/privacy assertion and product anti-rewind refusal. No ownership transfer or frontmatter mutation is authorized by this paragraph.

Keep 26 WP/lane identities,107 subtasks, WP25's14 owned paths, all13 create_intent entries, dependencies/authoritative surfaces and historical approvals exact. Full raw-reader sessions retain metadata-before-body/role+unique-byte guards, raw hash, old ls-tree/business verification, count preflight, witnesses and all30/240 budgets. Blob-only ordered ACK copying retains literal commit/tree writes, bounded owned scratch including unreachable exchange-table32MiB acceptance, exact first-detected-error diagnostics and mandatory all-return child ownership/EOF/Wait before any successful history/inventory/copy/ref advance. Qualified OS temp alias normalization stays inside the owned explicit qualification harness before any t.TempDir; arbitrary alias negative controls remain intact. No new Go path/dependency/default work or projectGit/tree-parser/adapter/lock/CLOEXEC/settlement/policy/identity/public-contract change.

The existing TEST-ONLY performance317/311 closed-union exception, numeric budgets, sample selection, ordering, math and controls remain unchanged. Require meaningful retained old/new public/causal controls, fresh independent source review, new root-attested311 hash values/all317 freeze, fresh default/control plus full4320 paired observations, full frozen source and all8 actual native packages, and normal independent WP25 review. No historical result is relabeled or reused as production-changed-source qualification. Normal finalizer planning-commit refresh and fresh analysis readiness precede normal WP25 implementation reentry; no force/skip/hand lifecycle event. No task/verdict/status completion or main/release action is recorded here.


## Prospective native retained-contribution I/O and checkpoint fixture correction — 2026-11-01

After the current H5 local full-source execution is fully reaped and its exact inputs/failure evidence retained, root may adopt only `integration-native-retention-io.md`, `integration-native-retention-io-regions.json` and `integration-native-retention-io-verification.json` beside WP25's existing addenda. This is T101–T103 work under unchanged owners, not a new work package, whole-file transfer or lifecycle verdict. Preserve all26 WP/lane identities,107 subtasks, WP25's14 owned paths,13 create_intent entries, WP24 dependency, authoritative surfaces and historical approvals.

The only new production exceptions are ReadContributionManifest's raw closure plus same-invocation payload/signature reuse in contribution_exchange.go, an adjacent native-local private helper in that existing file, and contributionRegisterRetained's raw closure in contribution_moderation.go. Use TWO SEQUENTIAL classic Git workers: `--batch-check` must admit ALL metadata and complete clean EOF/successful Wait/reap before lazy `--batch` can start. Native stderr remains `io.Discard` with retained storage0 and no cap; do not import artifact stderr limits/errors or require `--batch-command`. Preserve existing bounded enumeration/count/dedup/sort, all supplied limits/deadlines, framed headroom before body request/allocation, exact repeated OID/type/size, raw SHA1/SHA256 identity, original preflight/manifest/ls-tree/role/signature ordering and both independent plan observations. Retain registration locks/reservation/journal/coverage/ref/authority algorithms and permitted charged failure state. No change to shared projectObjectIDs/projectGit/project_snapshot.go, artifact I/O, default initialization/registry behavior, Go paths, dependencies or public protocol.

A separate TEST-ONLY exception in extension_changes_test.go (WP19 approved/inactive) replaces only the filesystem-checkpoint portion of TestExtensionChangesPrivateProofAccountingAndEviction and directly necessary local test helper. Set/read back distinct explicit mtimes and include an alternate creation-vs-mtime-order control; prove oldest-by-mtime eviction, exactly64 descriptors plus key/lock, and newest-valid. Keep the initial constructed200000-ref/128MiB-proof/256MiB accounting and later17×16MiB in-memory eviction checks unchanged. No production extension_changes.go or tie/FIFO policy change. The failed run's actual mtimes/readdir order were not retained: ordering is a source-supported hypothesis, not a recovered cause.

Only normal finalizer planning-commit refresh, fresh actual analysis/record-analysis and normal WP25 implementation reentry may activate this scope after root review. New production bytes require independent source/error/lifecycle review, a fresh root311/all317 attestation, unchanged default controls plus all4320 paired observations, full frozen race/static/build checks and all8 actual Linux/macOS standard/native package rows. Preserve current/historical results under their actual source identities; no cancellation, relabeling, deadline/cap/sample relaxation or inferred Mac pass. The separate Intel45-second migration watchdog failure remains unresolved by this design. No status/task/verdict completion, beta/main merge or release action follows from this amendment.
