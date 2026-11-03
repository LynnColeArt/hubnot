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
base_branch: kitty/mission-optional-extensions-01M661C0
base_commit: 39d72080478ae875924aee216cb4c5d9194ccc45
created_at: '2026-11-03T07:46:12.244677+00:00'
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

**Maintainer decision, 2026-11-03:** apply
[the native CI integration decision](../ci-integration-decision.md) before the
historical scheduling instructions below. Coordinator-owned CI maintenance uses
native fast/stress/soak tiers and bounded hosted package smoke. Keep the frozen
Mac source/packages intact, retain all assertions and actual platform requirements,
and label source-assisted, smoke and exhaustive evidence separately. This decision
does not mark T100–T103 complete or grant a WP approval.

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


## WP25 native actor-chain transient storage amendment (2026-11-01)

Root adopts the reviewed `integration-actor-chain-storage.md`, `integration-actor-chain-storage-regions.json` and `integration-actor-chain-storage-verification.json` beside WP25's existing addenda. This is narrow T101–T103 integration work, not a new task, whole-file transfer or lifecycle verdict. Preserve all26 WP/lane identities,107 subtasks, WP25's14 owned paths and13 create_intent entries, original dependencies, authoritative surfaces and historical approvals.

The only production exception is the complete existing `validateActorChains` function at store.go:634–EOF at H541. Replace full StoredEvent copies in private actor groups with immutable input indexes, and replace the sorted sequence-to-ID map with adjacent same-sequence/different-ID checks. Preserve sort.Slice, sequence-only comparison, insertion/tie behavior, unspecified cross-actor first failure, and the exact per-row conflict then expected sequence then Previous error order. Same-ID duplicates remain refused; no global conflict prescan, input mutation, cache, parser/signature/relationship/authority/recapture/retention or public-contract change. The only test exception is append-only behavioral/error/nonmutation coverage in existing event_test.go plus necessary standard-library test imports. Keep the other315 of317 Go/module inputs exact H541.

Primary-only planning sequencing refinement: reviewed metadata adoption, normal finalizer refresh and fresh actual analysis may proceed in the PRIMARY checkout while the separate exact-H541 lane-x full-source run remains active, provided its HEAD/index/clean state/all317 inputs and primary220 inputs, canonical24 approvals and26 lane assignments remain verified after each operation. This explicitly supersedes ONLY the addenda's earlier first-paragraph requirement to wait before PLANNING metadata adoption. Actual WP25 implementation reentry (which merges planning into lane HEAD) and every Go edit MUST still wait for local session14372 to naturally finish, fully reap and retain its exact source/results. Remote archived H541 jobs continue untouched and source-specific. The root ordering decision is retained externally with SHA2565016273c35ae98261e61616655480b4995b8431999ce0a7b5224bd952795c9b9.

This removes demonstrably redundant native storage, not a proved cause of the failed peak. The eight-event default fixture's static header arithmetic does not assure12KiB RSS headroom. Establish existing behavior first, retain exact diagnostics and deep input immutability, then verify the implemented reduction and freeze a new clean source for independent review/root311+317 binding. All unchanged default controls, COMPLETE4320 paired observations, full-source race/static/build and actual native8 package rows remain required. Preserve H541's actual performance FAIL and its Linux/platform results under H541 identity; no source-padding, arbitrary layout shift, flags/threshold/sample/deadline relaxation, inferred Mac pass, beta/main merge or release.


## WP25 bounded Darwin fixture portability amendment (2026-11-01)

Root adopts only the reviewed `integration-darwin-fixture-portability.md`, `integration-darwin-fixture-portability-regions.json` and `integration-darwin-fixture-portability-verification.json` beside WP25's existing addenda. This is bounded test-only T101–T103 maintenance. Preserve all26 WP identities,107 subtasks,24 historical approvals, lane assignments/groups, dependencies, authoritative surfaces, WP25's14 owned paths and13 create_intent entries. Do not change frontmatter, runtime or lifecycle state. Actor-chain source-only approval for2c372ccd3f570da43ab68856518f88cfddbb7b7c remains source-specific and does not approve this future correction; its fresh default driver never started and stays held pending normal amendment/reentry authorization.

Only two existing test paths may change against frozen2c/tree8eda4f807f170c2b8f27ed639c13ff6deaf97bf3. In owned artifact_read_test.go, replace only the isolated child fdNames helper and insert tightly colocated pre-exhaustion controls: explicitly open /dev/fd, read names with File.Readdirnames(-1), explicitly close the owned observer on every return, make read/close errors fatal, and sort the complete unfiltered names. On Darwin both transient observer descriptors remain in the read and close before return; filter no name, including File.Fd(). Prove stable successive baselines, sensitivity to an intentionally held real descriptor, and exact recovery after close. Keep the original actual childPASS/exit/watchdog/noFAIL/noSKIP boundary, cancellation/warmup, child-only RLIMIT64/free0/2/4/6, restoration, nilprocess/nonnilerror and exact cleanup sets byte-exact outside these regions.

Root expressly grants a small out-of-map test-only exception for artifact_store_commands_test.go, originally owned by WP05 with its approval preserved and no concurrent original-owner writer. This grants no whole-file ownership, new lane/task or frontmatter change. Insert real repository-local Git maintenance.auto=false and gc.auto=0 immediately after real git init and before public hn init. Add only one named t.Run("snapshot-detectors", artifactCLISnapshotDetectors) insertion immediately inside the existing TestArtifactStorePublishCLI, preserving every original statement. This registered live native root calls one appended private full-snapshot detector helper, which owns actual shared fixtures for SHA1/SHA256: assert the local config values, take a full baseline, prove lock-named addition, content-only mutation and mode-only mutation each change the appropriate complete snapshot, then prove removal restores the full baseline. Existing imports suffice and remain byte-exact. Keep snapshot/unchanged bodies byte-exact, including all repo/home/file/lock bytes, modes and error handling; retain all real fetch/refspecs and original conflict/superseded/unselected assertions. No standalone duplicate Test, qualification registry edit, ENOENT ignore, lock exclusion, retry, global/environment/production maintenance policy or search change.

All other315 Go/module inputs, including store.go/event_test.go, stay byte-exact2c; preserve the same317-path keyset, no new Go file/dependency/production change. Retain H541 run41303228685/job124838806072's completed Darwin ARM standard/native-only FAIL results and two leaf assertion classes with zero skips/outertimeouts under H541 source identity. They establish no underlying production worker/search defect. Retain the old H541 RSS FAIL, give no allocation-as-RSS credit, and monitor old Intel job124838806032 separately without assuming a45-second fix.

Only root's normal metadata adoption, finalizer refresh, fresh independent analysis/record-analysis and normal WP25 reentry may activate this scope; separate implementation authorization is still required. Preserve the original78 dirty primary files byte/mode/status exact through root's established targeted archive/stash/restoration sequence. After meaningful test-first controls, require independent clean final-source exact-diff review, fresh root311+317 binding, unchanged six default roots and COMPLETE4320 paired observations, full frozen-source race/static/build checks and all8 actual native packages. Actual amended-harness Mac evidence must show stable/positive/recovery controls, all4 free cases and real childPASS, and reach original search assertions through real fetch without retry. Linux alone cannot prove Mac RED/GREEN. No gate/budget/sample/env/deadline/collector change, historical relabeling, formal WP25 approval/acceptance/merge/main/release action is authorized here.


## WP25 bounded Intel pair-wave and fresh-authority amendment v2 (2026-11-01)

Root adopts only the reviewed v2 integration-intel-redundant-work.md/regions/verification addenda against frozen c0ccb40a02d7ef3a3cb72724de8e7e0e6bd0d388/tree6a95ccc56ab253c4b390cc6c35fbc27edefe0f14. The concept-only independent SOURCE_DESIGN_FEASIBLE_NO_BLOCKING_FINDINGS verdict does not approve these exact payloads or future code. Preserve original87bc files as immutable historical design; old packed RW01 remains held, cat-file replacement rejected and cached-value RW02A rejected.

RW01C schedules the ORIGINAL exact GLOBAL64 show-ref primaries in two fixed-slot PAIR WAVES, same argv/stdout32768 and original synchronous64-pattern/count65/stdout33280 fallback. Coordinator alone consumes original primary-error/fallback/sentinel/raw parser/global wanted bodies in order, including unexpected globally selected rows. At most2 producers INCLUDING fallback, no replacement/next pair until both prior blocks consumed/joined/released. Original parent ctx is checked at logical entry and immediately after completion before prefetched consumption; these are explicit scheduling-boundary semantics, not literal time equivalence. Keep original failed-command checks and first parser/error priority; no fallback-success/postparse/cleanup override. Every return/phase cancels and joins all launched/completed-unconsumed jobs and releases pair state. The66048 maximum returned logical payload is derived accounting, not RSS or a new public budget. No projectGit/native-cat-file helper/collector/namespace/command/cap/cardinality/deadline change. Root must explicitly confirm inactive original WP05/no concurrent writer before the bounded artifact_read.go scheduler/adjacent minimal-owner exception.

RW02B inserts only the exact commit+digest branch before Historical's original merge-base, after its unchanged fresh context/repository prefix. Explicit context check at that boundary, fresh raw-commit pending check, bounded commit-typed actual policy read, fresh parse/digest/enabled/project/maintainer checks remain per record; return fresh data. Original noncurrent/wrong-digest suffix and ALL constructor/current/caller Source/plan instances remain byte-exact. No cached value/context/state/parser/retention. Confirm inactive original beta-readiness WP12/no concurrent writer and preserve historical approval/ownership before this bounded contribution_records.go exception. Only nested live controls in the two existing registered artifact_read_test.go roots plus private EOF helpers accompany these scopes; original tests/imports/approved Darwin bytes stay exact.

All other314 Go/module inputs, all317 keys, source/default/native collector/limits/corpora/IDs/maps/receipts/deadlines, approved strictsnapshot/live detector and actor optimization remain exact C0. Preserve26WPs107tasks24reviews14owned13create, frontmatter/lanes/groups/deps/ownership and original78 user paths/current220 baseline. Root normal metadata-only adoption/finalizer/fresh independent analysis/record/reentry and separate explicit implementation permission are prerequisites; these records authorize no code/runtime action. All unchanged qualification/trial/acceptance gates bound by root714a/0dc and v2 verification remain; H5414PASS/2ARMFAIL/2IntelINCOMPLETE/RSSFAIL stays historical. No main/release, timeout/cap/sample/env/coverage waiver, retry-to-green or substitution.


## WP25 prospective import-union validation correction (2026-11-01; ROOT PENDING)

This is an external proposed amendment for existing T101–T103 integration work, pending exact root review/adoption, normal finalizer refresh, fresh independent actual analysis/record-analysis, normal WP25 implementation reentry and separate bounded producer authorization. Read `tasks/WP25-exact-package-qualification/integration-import-union.md` and its regions/verification JSON. No source, test, runtime, ownership, approval or lifecycle authority is granted by this draft. Preserve all existing planning content and all52 requirements,26WPs,107subtasks,24 historical approvals, dependencies, lane/group identities, authoritative surfaces, WP25's14 owned paths and13 create_intent entries; frontmatter and runtime state stay unchanged.

Against frozen c84402877996015cf33784bebd826e5be4981a4a/tree e118ad7d65f594ca856cce43393488c067596978, independently confirmed BR-001/P2 is source-only and unexecuted: individually valid incoming and current artifact histories can form an invalid union after accepted-ref promotion. The new inactive-original-WP10 production exception for importArtifactExchange in artifact_exchange.go and test-only exception for artifact_exchange_commands_test.go are ROOT PENDING. Prior artifact I/O exceptions do not cover this correction. Root must explicitly confirm original WP10 is inactive, no concurrent original-owner writer, exact bounded regions and unchanged historical approval/ownership; no whole-file transfer.

Follow the bound independent design: validate exact P=current.Sources overlaid by promoted destination imported refs in disposable quarantine, using the unchanged central loader and original validation context, before any receiver object/ref/signing/journal effect. Prefer import-local healthy-ODB read-only borrowing, with exact verified public-object inventory fallback only for otherwise already admitted/verified layouts. Preserve existing unsafe-layout refusals; fallback never bypasses them. Stage P witnesses with a bounded existing batch transaction using actual stage-old refs. Keep original validation context/cancellation and read/catalog limits; literal borrowMainObjects transplantation and per-ref staging subprocess loops are insufficient. Preserve source recheck, actor-prefix/rewind checks, selected-only copy/promotion, expected-old atomic CAS, lost-ack uncertainty, opaque behavior and post-promotion loader. No rollback, new clock, limit waiver, cache, packing, queue or shared-helper refactor.

Append a named live helper/subtest to existing qualification-selected TestArtifactExchangeRefusalsCLI; preserve its original body/assertions. In both Git SHA1/SHA256, real signed public CLI import must refuse a receiver-present wrong-kind optional target with invalid_history, exact full receiver no-effects and ordinary readability. The identical bundle must succeed at a receiver missing that target and report missing_optional. Meaningful authorized baseline RED, corrected GREEN and positive-control evidence must bind actual production/test/binary identities; this draft executes none. Other315 of317 Go/module inputs and all317 keys stay exact c844; no collector/workflow registration change.

C844 default144 criteria PASS, full-source7PASS and native run41349086217 attempt1 jobs LIVE at preparation are source-specific historical evidence, not corrected-source qualification or a current remote status observation. Preserve their identities without cancelling/rerunning/relabeling. Corrected source requires fresh independent exact-diff/source review, fresh root311/all317 attestation, unchanged six default roots and COMPLETE4320 paired observations, full frozen-source race/static/build and all8 actual native package rows, then normal WP25 review and later WP26 cold trials/52 adjudications. Default workflow is a hard gate throughout. No main/release action or gate/budget/sample/environment/deadline relaxation.


## WP25 prospective child-only E2BIG fixture portability amendment (2026-11-01; ROOT PENDING)

This is external proposed T101–T103 test-only maintenance, pending root's exact review/adoption of integration-e2big-fixture-portability.md and its regions/verification JSON, normal finalizer/planning refresh, fresh actual independent analysis/record-analysis, normal WP25 reentry and separate bounded producer authorization. No source or lifecycle authority is granted now. Preserve52 requirements,26WPs,107subtasks,24 historical approvals,14 owned paths/13 create_intent entries, all dependencies/lanes/groups/authoritative surfaces and frontmatter/runtime state.

Against frozen43574af5b57217df15a6ccf0b8f5c29cadc28593/tree13e20b3be58f14e0cb394a5ca999ec7ceae4437a, replace ONLY intelWitnessLowFD's existing E2BIG child-only256KiB generated environment value with2MiB, plus at most one directly adjacent explanatory comment. artifact_read_test.go is already WP25-owned; no ownership expansion is requested. Preserve every other statement/assertion/import/helper and actual verification_failure/real syscall.E2BIG/nilstdout/fullFD recovery/reaped childPASS/noFAIL/noSKIP boundary. No host-limit query, adaptive calibration, ambient environment dump, native dependency, application budget, production, collector, workflow, threshold, deadline or corpus change.

Explicitly supersede the latest import-union other315-of317 C844 freeze ONLY for artifact_read_test.go. Preserve artifact_exchange.go and artifact_exchange_commands_test.go exactly4357, leaving the combined other314 inputs exact C844. All316 inputs other than this new fixture delta remain exact4357;317-key set unchanged. Preserve historical metadata/evidence and current4357 fixes. Historical C844 Darwin ARM run41349086217 attempt1 job124975681149 has four real E2BIG leaf failures (sha1/sha256 × standard/native) with expected mapped refusal absent; this proves a fixture failure, not an underlying product defect or current4357 outcome. The supplied upstream Mac1MiB bound explains the hypothesis; actual runner getconf/sysctl was not observed.

ROOT53154 terminated exit143 at approximately00:06:04UTC: first race check INCOMPLETE with2520 namedPASS/0FAIL/6 known helper skips, no driver results or after-file; six later checks NEVER STARTED. Signal15 cause is unknown, neither testPASS nor proven infrastructure failure. Adoption/source edits require the interrupted ROOT command and all owned children fully reaped, exact manual all317 source-after/identical4357 source+tree plus partial/failure evidence preserved, and no remaining local check writer. Root reports no relevant processes and a clean working tree; the source-after capture remains a root verification obligation. Old4357 full7 stays INCOMPLETE and cannot qualify/approve; do not rerun it merely to satisfy the superseded all-seven-completed wording. After these conditions are verified, root may normal-adopt the known bounded fixture correction and separately authorize a new local candidate. OldC844/current4357 hosted jobs continue/finish independently against immutable checkouts/aliases. No alias rewrite/cancel/rerun/relabel or same-local-source overlap. Fresh new-candidate qualification still requires all unchanged7 checksPASS plus every default/native/other normal gate; no waiver or action authority now.

Retain actual old256KiB Mac RED; require corrected2MiB actual native8 GREEN with every existing genuine errno/nilstdout/fullFD/child assertion, and existing healthy selected-size/full-public-read controls with exact identities. Actual Mac RED/GREEN and healthy controls remain mandatory through preserved old/current or new source-bound observations, as well as Linux; Linux alone is insufficient. A separately authorized local size-to-zero fault injection is optional supplementary source-causal evidence, not a new mandatory Mac/native collector gate or requirement to rebuild/execute current qualification assets. Preserve source-only versus supplied-binary distinctions. Require independent clean exact-diff/source review, fresh root311/all317 binding, six unchanged default roots/collector and complete4320 observations/2160pairs/72groups/144criteria, full frozen-source race/static/build and all8 actual native rows. Default remains a hard gate; formal WP25 review, normal WP26 eligibility/two true cold trials/52 adjudications and every acceptance/merge gate remain. No historical relabeling, main or release.


## WP25 prospective native cadence/discovery-witness amendment (29ffc43c; ROOT PENDING)

Normal activation remains HOLD. Root review of this exact six-file draft, independent metadata review and root adoption are pending; design-only approval is not metadata adoption. Root must preserve current failed results and finish the authorized local RSS diagnostic: its root driver AND every owned child fully reaped, exact all317 source-after matching frozen29/source+tree, and no local source/test writer. This curation does not inspect processes or certify those conditions. WP16 and WP05 inactivity/no external writer are ROOT PENDING. Then normal preservation/adoption/finalizer, fresh independent actual analysis/record-analysis, normal WP25 implementation reentry and separate bounded producer authorization are required. No source or lifecycle edits are authorized by this draft. Old remote checkouts/jobs/aliases may finish independently under immutable original identities: no cancel, retry, alias rewrite, relabel or same-local-source overlap. Current29's17 RSS FAIL remain blocking, not erased by diagnostics or this amendment; the next candidate needs all original gates anew.

Read `tasks/WP25-exact-package-qualification/integration-native-correction.md` and its `-regions.json`/`-verification.json` before any separately authorized work. Independent review APPROVE_DESIGN_ONLY is source/metadata/runtime-unapproved.

At most four existing Go paths: internal/extension/session.go (WP16 T064–T067; one ticker expression), internal/extension/session_test.go (WP16; append focused controls and only necessary imports), artifact_read.go (WP05 T021/T023; private loader factoring only), artifact_read_test.go (already WP25 test-owned; append focused controls/necessary imports). Original ownership/authoritative surfaces and reviews remain; no whole-file transfer. WP25 T101–T103 carry the integration work, with no new task/WP. No new Go key/dependency; all other313 of317 inputs must be byte/mode exact29 after implementation. Both4357 import-union corrections and29's2MiB E2BIG expression/all inherited assertions remain exact. Preserve52 requirements/107 tasks/26 WPs/24 historical reviews/24 FRs pending,14 WP25 owned paths/13 create_intent entries, dependencies/lanes/groups/frontmatter and primary originals/modes/status/stashes.

Candidate A changes only session.go:498 to `time.NewTicker(RevocationInterval / 2)` for active optional sessions; published250ms observation/four sessions/2s TERM grace/5s exit and every owner/Recheck/per-request/shutdown check remain. Real≤250ms evidence, active-only Git/hash I/O and meaningful old-source RED are required, with no tolerance.

Explicitly supersede RW01C's initial-witness requirement ONLY on the ordinary loadArtifactHistory composition that itself just calls artifactSources. Prior RW01C does not authorize B. Keep captured discovery sources unchanged for the outer comparison: admission copies before sorting. Exactly two private compositions: explicit supplied sources → unchanged admission → original initial witness → shared verified body including final witness; actual local discovery → same admission → same body → fresh complete source comparison → Cut. No generic skip flag/token/provenance claim/callback/third entry or cache. Keep artifact_read.go:766–985 original worker/pair/primary/fallback/sentinel/parser/error/argv/caps/cancel/join bytes exact29; explicit source entry/signature and both rounds remain exact for artifact_exchange.go:679/:1140 and artifact_replication.go:329/:442. Retain replication's separate recheck and all downstream pre-effect/CAS boundaries. Ordinary calls inside mutation preparation, host catalog/change reads, export/import current-state reads and replication ALSO take B when they locally discover; only explicit supplied-source compositions are unchanged. No fetch or broader history. All admission/raw count/object/type/hash/signature/actor/schema/chain/aggregate/cleanup checks, final per-ref witnesses, final full-selection comparison and Cut remain. First/final complete captured selections are freshness boundaries, not universal ABA elimination or atomic-instant snapshots. Discovery-only omitted-round transient errors and mixed-invalid precedence may differ exactly as the independent review matrix below accepts; no universal unchanged-diagnostics claim. Every executed worker/body error remains binding; explicit-source diagnoses/order remain unchanged.

All original gates remain: independent exact-diff/source review, actual fresh311/all317 input binding, six defaults plus COMPLETE4320 observations/2160 pairs/72 groups/144 elapsed+RSS criteria PASS, full7 checks PASS, actual qualified8 native rows at original deadlines,100000/100001 and separate99999 causal controls,16384/16385/maps/receipts/policy,45s migration, real raw traces/detectors/three standalone consumers. Formal WP25 then normal WP26 eligibility/two fresh independent public-docs-only cold trials and52/24 preaccept judgments remain. Noncircular P01–P05 criterion/preaccept gates precede P06 accept→feature-only merge→postmerge review/retrospective; P06 remains mandatory mission work. No main/release, historical relabel, threshold/sample/corpus/deadline waiver. Mac native/artifact stage work is DIAGNOSTIC ONLY; no fixture request-budget, gate wait, marker or third production correction is scoped.


## WP25 prospective DD Intel readiness, recovery-fixture and qualification-method amendment (ROOT PENDING)

External draft only: root has not adopted these payloads or authorized source work. Read `WP25-exact-package-qualification/integration-dd-intel-correction.md`, `integration-dd-intel-correction-regions.json` and `integration-dd-intel-correction-verification.json` together. Independent design review recommends A/C and B initial bounded diagnostics only (report SHA256 `1ff3518f09f9db1b3f9fb8a9b4114b6b96bb7119d447ec75b50455259939eb75`); F1/P1 holds later B fixture activation. Root adoption and exact metadata review remain pending. This is bounded existing T101–T103 integration work; T100–T103 remain unfinished. No new requirement, task, WP, ownership transfer, frontmatter, dependency, lane/group identity, historical review or acceptance transition is proposed. Preserve all52 requirements,107 subtasks,26 WPs,24 historical approvals, WP25's14 owned paths/13 create_intent entries, authoritative surfaces and original user bytes/modes/status/stashes. COORD remains24 approved/WP25 in_progress/WP26 planned; PRIMARY historical status is not lifecycle authority.

Frozen source is dd906146b9055a874a9992c5a488d22fcb490a50/tree c34b4d2efaa010df68bbff33738c773f6e9501f3. This amendment builds on, and preserves, the inherited native cadence/discovery-witness, import-union, E2BIG, actor, count, A/B and RW01C/RW02B corrections. At most four existing Go paths plus the existing qualification workflow are prospective: WP20's fake-tracker main.go ready-publication region and focused main_test.go controls; WP18's extension_artifact_acceptance_test.go child/interrupted-recovery fixture regions only; WP25's extension_package_acceptance_test.go harness/registry/evidence and narrowly necessary workflow regions. Confirm WP20/WP18 inactive/no concurrent original-owner writer before separately authorizing their bounded exceptions. No whole-file transfer, new Go/module key, production artifact/host/session/store or general helper refactor. The prospective four-Go delta leaves313 protected Go/module inputs byte/mode exact DD; this is distinct from fresh311 inherited qualification identity and all317 current-input identity, both still required. Metadata adoption itself changes none of317.

A publishes the existing complete raw URL through a private0600 same-directory temporary file, checked full write and successful close, then rename, cleaning every failure and preserving listener/error behavior; no JSON URL or consumer tolerance. Strict polling/timer/over-limit/no-effect assertions stay exact. Deterministic child-only actual EFBIG controls must make old serve fail on final readiness exposure and corrected serve retain absent final readiness, plus unrestricted actual HTTP/cleanup controls. That causal OLD RED does not prove the old CI syscall schedule.

B is INITIAL BOUNDED DIAGNOSTICS ONLY: bounded16KiB phase observation/selected argv≤4KiB and actual checked selected-stage/ref evidence. Actual phase evidence, independent assessment and root's explicit decision precede any fixture or stage-budget correction. All proposed stage-driven framed cancellation/deadline decomposition, a new2000ms deadline subcase, Git-only pre-dispatch gate/expected cancelled classification and provisional20000ms request/10s stage-arrival candidate remain HELD, with no implicit permission. A controlled bounded pre-stage delay on the actual supplied host can demonstrate the original2000ms/late-stage conflation without reproducing Intel runner latency. Such evidence does not choose a new budget or establish actual Intel phase cause. Any later reviewed semantics decomposition must preserve the strict original2000ms first-byte timer, selected real-Git prepared/lost-ack evidence and exact cancellation semantics, ref/event/no-effect/replay/conflict/rotation/keyless/final-signing assertions and existing cleanup. All original host/product timers/caps remain hard. No arbitrary allowed-code union or timer-expiry substitute for framed cancellation.

C explicitly proposes two fixed serial groups (heavy4, remaining202; exact original ordered206 inventory) for both variants in the same four native jobs using the same binaries/references/tracing/isolation. Keep120m per invocation and300m per native job. If root adopts C, it supersedes ONLY the contradictory earlier internal assembly-method requirement that all206 roots complete in one aggregate120m qualification process: one row can instead use two120m invocations. It does not establish the old one-process120m result or improve product performance; full-source120m and all product timers, caps, corpora and assertions remain hard and unchanged. No matrix expansion, retry, parallel roots, hardware change, omission or cap increase. Only a validated same-identity composite of both groups may be a whole row: exact206 disjoint union, all nested runs ended successfully, no missing/duplicate/extra/overlap, fail/skip, invalid identity or control fallback, original trace and source-after evidence. Eight actual composite target/variant rows remain mandatory. The existing300m job can still fail and stays FAIL.

All historical failures remain FAIL and all incomplete results remain incomplete. Current immutable remote jobs/checkouts/aliases may finish; do not cancel/retry/rewrite/relabel. Normal root metadata-first preservation/adoption/finalizer refresh, fresh independent actual analysis/record-analysis and normal WP25 reentry precede separate bounded source authority; this draft executes none. Any changed source requires independent exact-diff/ownership/error review, fresh311/all317 and313-complement bindings, six default roots plus COMPLETE4320 observations/2160pairs/72groups/144 elapsed+RSS criteria, unchanged seven full-source checks and all eight actual revised-method native rows anew. Preserve69 limits/all206 roots,100000/100001 plus99999 causal controls,16384/16385 maps/receipts/policy,45s migration, original detectors/raw traces and three standalone consumers. Default workflow remains hard. Formal WP25 review, WP26 eligibility/two fresh independent public-docs-only cold trials/52+24 preaccept adjudications, normal accept→feature-only merge→postmerge review/retrospective remain mandatory and pending; no waiver, main or release action.


## WP25 B stage/deadline fixture activation amendment — 2026-11-02 (ROOT PENDING DRAFT)

This external draft is not adopted metadata and grants no source permission. Root's `root-B-fixture-design-maintainer-decision.json` accepts the exact targeted-reviewed DESIGN for normal activation. After exact metadata review/root adoption, normal finalizer/planning refresh, fresh independent ACTUAL analysis of this activated scope, record-analysis and normal WP25 reentry, SEPARATE exact-region source permission is still required. Neither file existence, an old READY report nor prior qualification grants that permission. Runtime F1 remains OPEN pending actual implementation/evidence; no source/native/WP approval follows.

Only after that COMPLETE sequence, including separate exact-region source permission, does this dated amendment supersede ONLY the prior B diagnostics-only/later source-activation HOLD for execution of the exact reviewed design in `WP25-exact-package-qualification/integration-b-stage-fixtures.md`. Root's design acceptance permits normal metadata/analysis activation while source execution stays held. The obsolete shared-recheck/pre-dispatch expected-cancelled proposal is rejected. Prior A/C semantics and the separate C-F1 composite-validator correction remain unchanged. WP18 remains the approved/inactive original owner; root acts as maintainer with an integration-delegate contract readback and independent review, without claiming original-author approval or transferring ownership. Existing T101–T103 carry the work; T100–T103 remain unfinished.

Separate a literal2000ms dispatch-reader deadline/no-effect leaf from prepared/lost-ack stage cancellation. The seeded first exact `[--git-dir=<actual physical private dir>,cat-file,--batch-command]` is request-only, after dispatch and before writer/intent; startup/monitor cannot select it. Require the actual gate, no cancel, peer alive/stdout open until host TERM, strict nonzero host `uncertain_operation`/`inspect_operation` with exact operation tuple, protected-state equality and exact12-field `verified_absent`/null projection, then ordinary successful mutation/found/ref-event/replay controls. Source first-prefix-byte clock plus peer writes/gate/registered-child TERM/parent monotonic brackets suffice; no inaccessible exact timestamp or kernel telemetry requirement.

Prepared/lost-ack use fixed20000ms request, checked stage arrival≤10000ms FROM HOST LAUNCH, exactly one supported framed cancel, peer alive/stdout open until TERM, and owned host launch→Wait settlement strictly<20000ms. Keep original30s stall and fixed30000ms helper/enclosing safety: any safety action is FAIL. Use finite nonqualifying helper/raw-peer fallbacks on host failure. This deliberately decomposes the old conflated2000ms fixture; it proves neither old-test PASS nor a relaxed public cap. Lost-ack's wrapper has no children: parent owns exactly ONE source-issued real-Git update-ref with exact physical dir/argv/ref/old/actual host candidate/message; frozen host environment filtered exactly like projectGit, same cwd/absolute Git/empty stdin/4096 stdout/discarded stderr; one successful Start→one Wait and joined observers on all returned paths. Checked atomic bounded requests/witnesses, no executable/alternate hooks, no replacement writer/OID/repair/retry. Publication can race failure: inspect actual ref, retain failure, never claim rollback/no effect.

The sole prospective B source surface is existing `extension_artifact_acceptance_test.go` necessary imports, raw child/helper/interrupted-recovery and tightly adjacent small fixture controls, as bounded in the new regions/verification JSON. No new Go/module/dependency, parent test name, product host/session/store/adapter/context change or seventh metadata destination. Preserve every original prepared/lost-ack assertion and protected-state control. Leaf evidence including metadata≤16KiB, selected argv≤4KiB; fixed files/counts and overflow FAIL; no environment/payload/signature/private-intent dump.

The actual preserved normal and controlled-delay packet now supports the earlier design-history diagnostic prerequisite: normal original prepared/lost-ack pass, delay reaches host uncertainty before the ORIGINAL missing-lost-ack-marker RED. It does not identify historical Intel timing or qualify the new fixtures. Planning PRIMARY is215d4311664710a420e96991eaf7258442823091; clean source anchor adc88255dbf0c9c8c43ccaaa9561a82b7f4d57f8 preserves all317 inputs in the exact initial8133 map (SHA2566a7b5abbc24b66b4828788c6dd0b33278dcc019b07f87a5685a38f966c2e892c), with four Go deltas vs DD and313 protected DD inputs. Metadata changes none of them; later source changes require fresh311/all317 and313-complement binding plus independent exact-diff/error/ownership review.

Preserve26 WPs/107 subtasks/52 requirements/24 historical reviews, WP25's14 owned paths/13 create_intent entries, frontmatter/dependencies/lanes/groups/authoritative surfaces and feature/optional-extensions target. COORD2a112042 remains24 approved/WP25 in_progress/WP26 planned; preserve original78 user files and all19 stashes. All206 parents/69 limits;240s/30s/10s/250ms/2s/5s public clocks;100000/100001/99999 and16384/16385/maps/receipts/policy;45s migration; full-source120m/seven checks, six defaults and COMPLETE4320 paired observations, eight actual native composite rows, C's fixed heavy4/remaining202 serial groups at120m/invocation/300m/job remain hard. Fresh actual analysis must assess activated B scope; no old READY or qualification/source reuse. Formal WP25 review, WP26 eligibility/two fresh independent public-docs-only cold trials,52+24 preaccept judgments, normal accept→feature-only merge→postmerge review/retrospective remain pending. Historical FAIL/incomplete stays unchanged; no lifecycle/remote/main/release action is performed.


## WP25 expiry diagnostic-only fixture amendment — 2026-11-02 (ROOT PENDING EXTERNAL DRAFT)

This appendix proposes only existing T101–T103 diagnostic maintenance. Read `tasks/WP25-exact-package-qualification/integration-expiry-diagnostics.md` and its regions/verification JSON together. Independent exact metadata/design review, root adoption, normal finalizer/planning refresh, fresh independent ACTUAL analysis/record-analysis/next, normal WP25 reentry and SEPARATE exact-region source permission remain ROOT PENDING. File existence, historical READY, source-only review and this external packet grant no source/lifecycle authority. Root independently reviews and activates; this delegate cannot adopt. Preserve original78 files/modes/status and all20 retained stashes; later activation needs a NEW21st targeted stash owned by root.

Actual frozen H b389b4dda20f6cbe419e359e5f5cbbfab5d4c497/tree64a1153bee3333be76d342a01bb39f08d3612625 full ROOT6503 exited1 after4262.878s (driver4262.701154867s). Sole named failure was response.1 absent after the unchanged15s wait in TestExtensionChangesRealTenMinuteExpiry, before callback1/final expiry assertion. Cause remains UNRESOLVED; ordinary TTL expiry refusal would still publish.1. Relevant H/DD helper/session/expiry bytes match. All3 new B leaves passed and all317 frozen inputs remained exact after failure; six later checks NEVER STARTED. H's scoped default149/performance4320/144 PASS cannot promote the full gate or qualify changed source. Old H native41434652393 attempt1 continues immutably; no cancel/rerun/alias rewrite, and any results stay old-H-only.

The sole prospective source file is already WP25-owned extension_changes_acceptance_test.go via the original eight-test maintenance transfer; WP19 remains approved/inactive. Root may authorize only its imports, changesRun Start/Wait/failure evidence seam, raw-child last-phase/known terminal observation and tightly adjacent finite controls. No whole-file transfer/new task/WP/dependency/Go key. After the complete activation sequence and separate source permission, supersede ONLY the latest B other316/four-Go/313-DD-complement freeze as needed for this exact TEST-file exception. Other316 inputs stay exact H; B fixture file, A sample fixes, C harness/validator/workflow and all production bytes stay exact. Cumulative five Go deltas vs DD leave312 protected DD inputs; fresh root311/all317 keysets/full modes still required. Metadata itself changes none of317. Earlier B ROOT PENDING preparation headers remain historical; actual prior root adoption/source permission resolved that earlier scope, not this amendment.

Preserve expiry body H770–821 byte-exact: real600s, advertised expiry−1s request, final expiry+1s refusal; known missing postresponse clock proof remains unresolved. Every assertion,15s wait,5ms poll,20s keepalive,11min child safety and public240s/30s/10s/250ms/2s/5s clocks stay hard. No retry, earlier scheduling, clock/lifetime/deadline/cap relaxation or production fix. Keep callbacks synchronous on the testing goroutine; no interruption of callback0's sleep or Fatal across an unjoined Wait. A single Wait owner records actual settlement; testing control notices already-settled command when polling resumes, with settlement and observation times distinct. Prefer ordinary CLI TERM/owned-child cleanup under existing5s exit budget; old hardKill fallback is NONQUALIFYING.

Before TempDir cleanup, retain only bounded safe terminal origin/code/recovery, actual exit/timing, response/continue-marker presence/size/digests and last peer/request/keepalive phases. Read buffers only after join; retain stderr byte count/digest, never raw text. Existing HN_WP19_EVIDENCE success transcript stays byte-exact. Failure diagnostic uses HN_WP25_EVIDENCE or bounded t.Log: fixed atomic0600 observations, per-call total including metadata/transient slots≤16KiB, explicit overflow/write failure; no payload/env/signatures/private intent/consumer-state contents/raw diagnostics. Require meaningful old real-host RED/new early-settlement precise record, unchanged15s unresponsive-host cleanup, positive paused peer, callback-Fatal, write/oversize/redaction/singleWait/no-leaked-child controls. Those controls supplement the still-required fresh actual600s expiry run; diagnostic code alone fixes no proven product defect.

Preserve26 WPs/107 subtasks/52 requirements/24 full canonical review payloads,14 owned/13 create_intent entries, frontmatter/dependencies/lanes/profiles/authoritative surfaces/feature target. COORD42c82787 remains24 approved/WP25 in_progress/WP26 planned; no transition is asserted. Every future changed-source qualification still requires independent exact-diff review; fresh311/all317/312-complement; six defaults/all149 and COMPLETE4320 observations/2160pairs/72groups/144 elapsed+RSS criteria; all7 full-source checks at original120m; all8 NEW actual native composite rows with exact206 parents/69 limits and fixed C heavy4/remaining202 serial120m invocations/300m job. Preserve100000/100001/99999,16384/16385/maps/receipts/policy,45s migration, detectors/raw traces/three standalone consumers. Formal WP25→WP26 eligibility/two fresh independent public-docs-only cold trials/52+24 preaccept judgments/normal accept/feature-only merge/target validation/postreview/retrospective remain mandatory. Historical FAIL/incomplete stays unchanged; no qualification rerun/dataset edit/main/release action here.
