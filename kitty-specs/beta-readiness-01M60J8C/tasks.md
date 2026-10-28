# Work Packages: Hubnot beta readiness

**Inputs**: `spec.md`, `plan.md`, `research.md`, `data-model.md`.

Each WP has a separate owned surface and independent review. Initial delivery is
WP01 → WP02 → WP03 → WP04, then WP10 → WP11 → WP12 → WP13 delivers optional
unified contribution/moderation before WP08 and the final beta verdict.
Later qualification cannot fabricate external evidence.
Subtask completion is recorded by the framework event log using `mark-status`.

## Work Package WP01: Beta support contract and current release status (Priority: P1)

**Goal**: Beta support contract and current release status.
**Independent Test**: Documentation-only: verify each new download URL against the published asset list, check all changed local Markdown links, inspect commands against help/implementation, git diff --check. Do not run the full expensive Go suite for prose-only edits. No external writes, issue comments or release publication.
**Prompt**: `tasks/WP01-beta-support-contract.md`
**Requirement Refs**: FR-001, FR-002, FR-014, C-001, C-002, C-004, C-005, C-006

### Included Subtasks

T001 Write docs/beta-support.md with prospective stable surfaces, experimental boundaries, candidate platform/Git matrix, version evolution, rollback and release gates.
T002 Update README.md and CHANGELOG.md for published 0.0.3-dev, all eleven download links and exact Mac qualification distinctions; retain previous release history and immutable assets.
T003 Check the support promise against existing commands/contracts and published release metadata; record remaining evidence obligations and independent doc/link validation.

### Dependencies

None.

## Work Package WP02: Released-writer compatibility and refusal coverage (Priority: P1)

**Goal**: Released-writer compatibility and refusal coverage.
**Independent Test**: Test-first contract fixtures and public CLI scenarios with a credential-free environment. Focused iterations, then gofmt, go test -race -count=1 -timeout 20m ./..., go vet ./..., go build ./..., git diff --check at the final committed head. No simultaneous expensive full suites; tell root before the final run.
**Prompt**: `tasks/WP02-released-writer-compatibility.md`
**Requirement Refs**: FR-002, FR-003, FR-004, NFR-001, NFR-003, NFR-005

### Included Subtasks

T004 Create a small offline public-only compatibility corpus from released writers with source/artifact provenance, exact fixture hashes and expected semantic CLI observations.
T005 Add beta_compatibility_test.go for standard/native-only readers, historical IDs/signatures/ref preservation, representative archives/resources/memory and new-actor continuation.
T006 Exercise unknown protocol/resource/archive versions through public boundaries; assert actionable refusal without accepted-history/private-state mutation.
T007 Document the tested historical baseline, corpus maintenance and supported compatibility/rollback behavior; run charter gates and preserve results.

### Dependencies

Depends on: WP01

## Work Package WP03: Public backup and exclusive private recovery (Priority: P1)

**Goal**: Public backup and exclusive private recovery.
**Independent Test**: Public subprocess tests in standard/native-only modes; compare all selected preexisting history and assert absent unwanted keys/authority/optional activation. Test stale public chain recovery, lost signer/fresh actor, unsafe modes and unchanged history. Then gofmt, full race, vet, build, diff check at a committed head. Coordinate full-suite resource use with root.
**Prompt**: `tasks/WP03-public-private-recovery.md`
**Requirement Refs**: FR-005, FR-006, FR-007, NFR-001, NFR-002, NFR-003, NFR-005, C-003

### Included Subtasks

T008 Write executable recovery procedures in docs/recovery.md separating public code/custom refs, private keys/local state and uncertain external journals.
T009 Add a keyless fresh-clone/public-backup rehearsal retaining selected collaboration and memory history and comparing exact historical objects/ref targets.
T010 Add private disaster-restore and fresh-actor journeys: fence prior writer, recover latest chain, continue without history rewrite; refuse common actor appends from a stale/divergent signing head when newer accepted history is already known, with zero side effects before reconciliation.
T011 Verify lost sole authority, separate policy-role amendment, deliberate audit authority adoption, worktree limitations and backup exclusions; run charter gates.

### Dependencies

Depends on: WP02

## Work Package WP04: Journal compatibility and interrupted-operation recovery (Priority: P1)

**Goal**: Journal compatibility and interrupted-operation recovery.
**Independent Test**: Run targeted tests for actual gaps, then applicable full charter gates for any Go change. Documentation-only inventory must not masquerade as executed recovery evidence.
**Prompt**: `tasks/WP04-interrupted-operation-recovery.md`
**Requirement Refs**: FR-002, FR-008, NFR-001, NFR-002, NFR-005

### Included Subtasks

T012 Inventory originating-binary/session-version upgrade rules and explicit resume/reconcile commands in docs/operation-recovery.md.
T013 Add missing observable recovery assertions for supported migration/transfer/audit/rotation/merge interruption boundaries, reusing existing tests and seams.
T014 Prove unknown or unsafe journals refuse without resetting progress, and uncertain remote writes never trigger duplicate creation; validate charter gates.

### Dependencies

Depends on: WP03

## Work Package WP05: Live provider migration qualification (Priority: P2)

**Goal**: Live provider migration qualification.
**Independent Test**: Record exact scenario/environment/outcome/evidence for every claimed live pass. Missing live access remains an unmet qualification obligation; completing a brief alone does not complete T016.
**Prompt**: `tasks/WP05-provider-qualification.md`
**Requirement Refs**: FR-009, NFR-004

### Included Subtasks

T015 Define disposable live provider/resource/direction cases and provider-to-local-to-provider mapping/loss comparisons in docs/provider-qualification.md.
T016 Execute available authorized live cases, recording account type/API profile/permissions/rate-limit/retry observations and preserving remote edits.
T017 Record unexecuted cases with concrete owner prerequisites and classify candidate beta support without claiming fake-server results as live proof.

### Dependencies

Depends on: WP03

## Work Package WP06: Candidate package and distribution qualification (Priority: P2)

**Goal**: Candidate package and distribution qualification.
**Independent Test**: Each claimed OS/architecture/variant gets observed normal-environment package evidence. Missing hardware is pending, not a pass. Inspect archive digests/BUILD provenance independently of runtime qualification.
**Prompt**: `tasks/WP06-platform-qualification.md`
**Requirement Refs**: FR-010, NFR-004

### Included Subtasks

T018 Define exact-package Linux/macOS architecture/variant installation and runtime cases, supported Git/filesystem configurations and download experience in docs/platform-qualification.md.
T019 Run available candidate package cases using existing Go-only packaging and public package acceptance; collect actual platform/binary evidence.
T020 Record outstanding Intel/Mac exact-byte work and a signing/notarization recommendation with owner decision prerequisites.

### Dependencies

Depends on: WP03

## Work Package WP07: Practical workload and recovery limits (Priority: P2)

**Goal**: Practical workload and recovery limits.
**Independent Test**: Report source, hardware, fixture dimensions, commands and measured results. Keep normal tests reasonably bounded; expensive workloads require explicit benchmark/scenario invocation. Go changes receive charter gates.
**Prompt**: `tasks/WP07-practical-operating-limits.md`
**Requirement Refs**: FR-011, NFR-004, NFR-005

### Included Subtasks

T021 Select representative actor/issue/discussion/project/memory history sizes and bounded migration/recovery workloads from the contract.
T022 Add reproducible benchmark/scenario coverage in beta_scale_test.go and record resource/latency/recovery measurements.
T023 Publish measured supported envelopes and budget exhaustion behavior in docs/operating-limits.md without promising unmeasured universal thresholds.

### Dependencies

Depends on: WP03

## Work Package WP08: Independent beta boundary review (Priority: P2)

**Goal**: Independent beta boundary review.
**Independent Test**: Review exact source, trust boundaries and public failure paths; use bounded probes where justified. A checklist without investigation is not review evidence. Unresolved release-blocking findings prevent qualification.
**Prompt**: `tasks/WP08-focused-boundary-review.md`
**Requirement Refs**: FR-012, NFR-002

### Included Subtasks

T024 Independently review credential/key handling, imports, filesystem boundaries, contribution intake/moderation, authority and runner isolation against the prospective beta contract.
T025 Record actionable findings and nonfindings with evidence in docs/beta-boundary-review.md; identify required repair WPs.
T026 Recheck disposition of release-blocking findings against reviewed fixes without claiming hardened multi-tenant isolation.

### Dependencies

Depends on: WP04, WP07, WP13

## Work Package WP09: External pilot and beta readiness verdict (Priority: P2)

**Goal**: External pilot and beta readiness verdict.
**Independent Test**: All beta-supported combinations require observed evidence and no unresolved release-blocking defect. Mission accept/merge and final publication remain distinct; do not tag a beta from this WP.
**Prompt**: `tasks/WP09-pilot-and-beta-verdict.md`
**Requirement Refs**: FR-013, FR-014, NFR-004

### Included Subtasks

T027 Prepare independent-adopter install/collaborate/migrate/upgrade/restore and stranger-contribution/moderation tasks and collect actual pilot outcomes.
T028 Trace each support promise and success criterion to qualification evidence and defect disposition in docs/beta-readiness.md.
T029 Record the truthful beta verdict and remaining blockers; prepare release recommendation without publishing or silently changing the support contract.

### Dependencies

Depends on: WP04, WP05, WP06, WP07, WP08, WP13



## Optional unified contribution and moderation work

The owner added this work on 2026-10-28. Existing defaults stay unchanged;
WP10–WP13 must pass before the boundary review and final beta verdict.

## Work Package WP10: Native contribution policy and signed record foundation (Priority: P1)

**Goal**: Native contribution policy and signed record foundation.
**Independent Test**: Exercise exported Go boundaries for strict signed formats, altered revision/signature/scope, invalid or stale authority, conflicting decisions, operation rules and absent configuration. Run charter Go gates at a committed head; foundation review must identify downstream consumers and must not count tests-only wiring as a working public workflow.
**Prompt**: `tasks/WP10-native-contribution-foundation.md`
**Requirement Refs**: FR-015, FR-018, FR-019, FR-020, NFR-007, C-001, C-002, C-003, C-007

### Included Subtasks

T030 Specify the native contribution/decision contract, authority source, concrete CLI/wire examples and existing integration seams in docs/contribution-contract.md.
T031 Implement shared typed submission identity, exact original-content/revision bindings, scoped signed decision records and strict bounded format validation in internal/contribution/.
T032 Implement one optional operation-permission evaluator distinguishing transport verification, submission admission, author edits/replies, moderation and existing code acceptance/merge.
T033 Define deterministic replay, concurrent moderation, revocation, historical-baseline and disable/re-enable semantics without rewriting original facts or granting authority.
T034 Verify model/policy/signature contracts, disabled configuration and invalid authority with independent tests; identify any additional live integration file ownership before downstream implementation.

### Dependencies

Depends on: WP04

## Work Package WP11: Bounded submission exchange and pending intake (Priority: P1)

**Goal**: Bounded submission exchange and pending intake.
**Independent Test**: Use controlled streams/local repositories and fault/cancellation seams to assert exact byte/count/disk budgets and no accepted-ref/branch/private-authority changes on refusal. Aggregate limits must survive many tiny submissions from different keys. Tests must fail for actual excess reads/retention, not only inspect configuration. Run formatting, full race, vet, build and diff checks at a fixed head.
**Prompt**: `tasks/WP11-bounded-contribution-exchange.md`
**Requirement Refs**: FR-016, FR-017, FR-020, NFR-006, NFR-007, C-001, C-007

### Included Subtasks

T035 Implement a provider-free bounded artifact/reference exchange using the WP10 contract and existing safe archive/object primitives where applicable.
T036 Isolate and validate submitted content before project admission, enforcing positive finite per-request and defined operation/window limits plus aggregate pending/admitted retention accounting that automatic admission, drained/hidden queues and new actor keys cannot reset.
T037 Implement inspectable pending state, exact-intent deduplication and interrupted/replayed intake recovery with no branch, policy, signer or workflow side effects.
T038 Prove early limit/cancellation behavior for signed spam, huge/truncated inputs, many-key aggregate saturation and unselected refs; record supported transport guarantees and exclusions.

### Dependencies

Depends on: WP10

## Work Package WP12: Native contribution commands authorization and moderation (Priority: P1)

**Goal**: Native contribution commands authorization and moderation.
**Independent Test**: Through actual public CLI paths, prove unknown-signer submission without upstream push, configured automatic and manual admission, denied foreign edits, invalid/revoked/conflicting moderation, raw-history preservation, observer reconstruction and retries. Exercise absence of configuration and independence from protected checks/automerge/audits. Imported metadata or direct native commands must not bypass enabled operation rules. Run all charter Go gates and report no_coverage honestly if the framework's Python gate is unavailable.
**Prompt**: `tasks/WP12-native-admission-and-moderation.md`
**Requirement Refs**: FR-015, FR-016, FR-018, FR-019, FR-020, NFR-002, NFR-007, C-001, C-002, C-003, C-007

### Included Subtasks

T039 Wire the shared contribution model/exchange into real native CLI submission, pending-list/inspection and explicit opt-in policy paths without changing absent-configuration behavior.
T040 Enforce operation-specific admission across code proposals and communications: automatic allowed issue/reply intake, protected edits and moderation, and unchanged native merge/execution authority.
T041 Implement signed scoped admit/reject, hide/restore and author block/unblock commands and moderated shared views, preserving raw signed inspection and visible conflicts/revocations.
T042 Exchange and reconstruct contribution/decision state using native Git-backed storage with exact targets and safe retries; validate direct command bypasses, disabled behavior and no implicit execution.

### Dependencies

Depends on: WP11

## Work Package WP13: Unified contribution public acceptance and operator guide (Priority: P1)

**Goal**: Unified contribution public acceptance and operator guide.
**Independent Test**: Run actual public black-box cases with unchanged environment boundaries; verify selected historical payload/signature/ref preservation, no unexpected keys/authority/optional activation, resource refusal before shared visibility and safe retries. Then formatting, full race, vet, build and diff gates. Validate documentation examples against actual CLI and the WP10 contract.
**Prompt**: `tasks/WP13-contribution-public-acceptance.md`
**Requirement Refs**: FR-021, FR-015, FR-016, FR-017, FR-018, FR-019, FR-020, NFR-001, NFR-003, NFR-006, NFR-007, C-007

### Included Subtasks

T043 Add standard/native-only provider-free contributor/maintainer/observer public CLI journeys for issues, discussion replies and code proposals with upstream push explicitly denied.
T044 Exercise enabled/disabled behavior, signed spam and transfer/retention bounds including automatic-admission accounting across drained queues and many keys, replay/interruption, scoped moderation/revocation/conflicts and unchanged existing merge/CI/execution gates.
T045 Write docs/contributions.md with submission/admission/moderation commands, optional policy examples, raw-view/history limits, recovery and actual resource-safety guarantees; supply pilot and boundary-review evidence.

### Dependencies

Depends on: WP12

