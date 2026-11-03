# Work Packages: Native Runner Efficiency

Inputs: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md),
[data-model.md](data-model.md), [contract](contracts/runner-efficiency-v1.md).
Five sequential reviewable packages share core execution/verification surfaces.
WP02-WP04 share runner.go ownership, with withdrawal execution enforcement in
WP03; disjoint upstream/downstream lanes retain sequential dependencies.
The first gate is the existing default workflow. Final consolidation is into
fix/runner-efficiency, followed by a reviewed PR to main.

Subtasks are event-sourced reference rows, never mutable completion checkboxes.
No production work starts before the analysis report and finalized task metadata.

## Work Package WP01: Default workflow gate and shared pipeline evidence (Priority: P1)

**Goal**: Establish the default-workflow baseline and complete the cohesive contract below.
**Dependencies**: None
**Prompt**: [WP01](tasks/WP01-default-gate-and-shared-evidence.md)
**Requirement Refs**: FR-001, FR-003, C-001, C-002, C-003
**Independent Test**: Real CLI/Git/backend contracts and positive/no-effects controls described in the WP prompt.

### Included Subtasks

T001 Establish the default-workflow baseline
T002 Extract canonical pipeline result evaluation
T003 Prove unchanged governance semantics
T004 Verify boundary and document evidence
T005 Hand off for independent review

### Implementation Notes

Follow tasks/WP01-default-gate-and-shared-evidence.md. Keep one sequential computed workspace, independent review,
native/default constraints and finite test scope. No status checkboxes.

## Work Package WP02: Useful-work scheduling and explicit stale target observation (Priority: P1)

**Goal**: Specify scheduler outcomes with real controls and complete the cohesive contract below.
**Dependencies**: WP01
**Prompt**: [WP02](tasks/WP02-runner-eligibility-and-target-observation.md)
**Requirement Refs**: FR-001, FR-002, FR-003, FR-007, FR-008, NFR-001, NFR-003
**Independent Test**: Real CLI/Git/backend contracts and positive/no-effects controls described in the WP prompt.

### Included Subtasks

T006 Specify scheduler outcomes with real controls
T007 Integrate lineage and trusted-threshold eligibility
T008 Add explicit stale-base runner options
T009 Implement bounded authenticated target observation
T010 Validate automatic-only semantics and hand off

### Implementation Notes

Follow tasks/WP02-runner-eligibility-and-target-observation.md. Keep one sequential computed workspace, independent review,
native/default constraints and finite test scope. No status checkboxes.

## Work Package WP03: Signed requester withdrawal and portable dependency validation (Priority: P1)

**Goal**: Define failing signed withdrawal cases and complete the cohesive contract below.
**Dependencies**: WP02
**Prompt**: [WP03](tasks/WP03-withdrawal-protocol-and-portable-closure.md)
**Requirement Refs**: FR-004, FR-005, C-001, C-002, NFR-003
**Independent Test**: Real CLI/Git/backend contracts and positive/no-effects controls described in the WP prompt.

### Included Subtasks

T011 Define failing signed withdrawal cases
T012 Implement strict new-kind shape and relationship
T013 Wire local, quarantine and closure paths
T014 Prove portability and unchanged historical evidence
T015 Validate source variants and hand off

### Implementation Notes

Follow tasks/WP03-withdrawal-protocol-and-portable-closure.md. Keep one sequential computed workspace, independent review,
native/default constraints and finite test scope. No status checkboxes.

## Work Package WP04: Withdrawal CLI, discovery and queued execution controls (Priority: P1)

**Goal**: Specify public commands and retry behavior and complete the cohesive contract below.
**Dependencies**: WP03
**Prompt**: [WP04](tasks/WP04-withdrawal-cli-and-execution-integration.md)
**Requirement Refs**: FR-001, FR-004, FR-006, FR-008, NFR-003
**Independent Test**: Real CLI/Git/backend contracts and positive/no-effects controls described in the WP prompt.

### Included Subtasks

T016 Specify public commands and retry behavior
T017 Implement requester-only idempotent command
T018 Expose withdrawal and preserve moderation
T019 Enforce future-start boundary
T020 Validate sync, retries and ordinary workflow

### Implementation Notes

Follow tasks/WP04-withdrawal-cli-and-execution-integration.md. Keep one sequential computed workspace, independent review,
native/default constraints and finite test scope. No status checkboxes.

## Work Package WP05: Integrated acceptance, maintained documentation and issue reconciliation (Priority: P1)

**Goal**: Add integrated native acceptance journey and complete the cohesive contract below.
**Dependencies**: WP04
**Prompt**: [WP05](tasks/WP05-integration-docs-and-issue-closeout.md)
**Requirement Refs**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, NFR-001, NFR-002, NFR-003, C-001, C-002, C-003, C-004
**Independent Test**: Real CLI/Git/backend contracts and positive/no-effects controls described in the WP prompt.

### Included Subtasks

T021 Add integrated native acceptance journey
T022 Synchronize command, protocol and operator docs
T023 Run integrated validation and preserve defaults
T024 Reconcile issues and coordination follow-up
T025 Complete independent readiness handoff

### Implementation Notes

Follow tasks/WP05-integration-docs-and-issue-closeout.md. Keep one sequential computed workspace, independent review,
native/default constraints and finite test scope. No status checkboxes.
