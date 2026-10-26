# Work Packages: Portable collaboration and independent audits

**Inputs**: `kitty-specs/portable-collaboration-and-independent-audits-01M5VTP0/spec.md`, research, substantive plan/data model/contracts and quickstart.
**Target**: `feat/portable-collaboration`; prepare a reviewable PR, without implicit public release/tag changes.
**Tests**: Required by the project charter and mission NFRs; use public CLI/real Git and bounded local API fixtures.
**Completion authority**: Plain `T###` rows are references. Record subtask completion only through `spec-kitty agent tasks mark-status`; WP lifecycle is runtime/event-log owned.

## Work Package WP01: Go-only portable release packager (Priority: P1)

**Goal**: Replace prospective release packaging with a Go/Git-only command that runs without Python or a Unix shell.
**Independent Test**: Two clean builds from one declared source/toolchain/version yield equal binary/source archives and checksums; the command cross-compiles for Windows/macOS and produces independently inspected archive contents.
**Prompt**: `tasks/WP01-go-only-release-packager.md`
**Requirement Refs**: FR-001, FR-021, FR-022, NFR-004

### Included Subtasks

T001 Specify deterministic package inputs and adversarial archive/output tests
T002 Implement the release command and archive/checksum generation
T003 Verify repeatability and portable packaging behavior

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Source selection, timestamps, modes and gzip headers can undermine reproducibility; host-only paths and shell assumptions can undermine Windows packaging. Treat host runtime qualification as evidence, never infer it from compilation.

### Parallel Opportunities

Independent starting package; may run beside other independent packages once the runtime assigns distinct lanes.

### Dependencies

None.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Source selection, timestamps, modes and gzip headers can undermine reproducibility; host-only paths and shell assumptions can undermine Windows packaging. Treat host runtime qualification as evidence, never infer it from compilation.

---

## Work Package WP02: Native observational audit protocol and admission (Priority: P1)

**Goal**: Introduce signed audit requests/results tied to inspected code and authorized rule revisions, with one authority contract for local writes, reads and replication.
**Independent Test**: A valid audit exchange replicates through a real temporary Git remote, while forged relationships, missing rule authorization and partial/dependency-invalid events refuse without accepted visibility; audit facts never count as merge evidence.
**Prompt**: `tasks/WP02-native-audit-protocol-admission.md`
**Requirement Refs**: FR-001, FR-016, FR-018, FR-019, NFR-001, NFR-002

### Included Subtasks

T004 Define audit envelope/payload and permission contracts through tests
T005 Implement canonical audit validation and fetched admission
T006 Prove replication and non-authority behavior

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Treating audit results as existing run results could accidentally satisfy merge requirements. A rule digest is provenance, not proof that its signer owns the rules; admission must validate the authorized source/owner relationship.

### Parallel Opportunities

Independent starting package; may run beside other independent packages once the runtime assigns distinct lanes.

### Dependencies

None.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Treating audit results as existing run results could accidentally satisfy merge requirements. A rule digest is provenance, not proof that its signer owns the rules; admission must validate the authorized source/owner relationship.

---

## Work Package WP03: Explicit audit execution and linked follow-up issues (Priority: P1)

**Goal**: Make independent audits usable through explicit native commands and deliberately turn stable findings into retry-safe linked issues.
**Independent Test**: The compiled CLI audits a resolved code snapshot, records a nonblocking result, creates a linked issue only on explicit request, and retries after interruption without an issue flood or unauthorized rule changes.
**Prompt**: `tasks/WP03-audit-execution-followup-issues.md`
**Requirement Refs**: FR-001, FR-017, FR-018, FR-019, FR-020, FR-022, NFR-001, NFR-003

### Included Subtasks

T007 Specify public request/execute/inspect/follow-up scenarios
T008 Implement explicit audit execution and inspection
T009 Implement deliberate follow-ups and recovery

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Do not confuse audit failure with runner transport/error state, or use an unstable timestamp/commit-only key that generates duplicate issues for the same finding. Do not claim distributed exactly-once creation or silently edit unrelated native issues.

### Parallel Opportunities

After WP02 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP02.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Do not confuse audit failure with runner transport/error state, or use an unstable timestamp/commit-only key that generates duplicate issues for the same finding. Do not claim distributed exactly-once creation or silently edit unrelated native issues.

---

## Work Package WP04: Portable project snapshots and bounded archive format (Priority: P1)

**Goal**: Define a bounded portable representation of selected Git/native histories, attributed foreign communications and verified attachment blobs.
**Independent Test**: An archive round trip retains selected Git object/signed event bytes and locally inspectable foreign topology/blobs; malformed paths, members, digests, budgets and incomplete native dependencies refuse before accepted visibility.
**Prompt**: `tasks/WP04-portable-project-snapshot-archive.md`
**Requirement Refs**: FR-002, FR-003, FR-004, FR-005, FR-014, FR-015, NFR-001, NFR-002

### Included Subtasks

T010 Specify snapshot/archive contracts and hostile input cases
T011 Implement deterministic snapshot and archive read/write
T012 Verify exact preservation and isolation

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. A bare Git archive of all refs could leak local/private or unrelated state. Foreign review observations must never be interpreted as native approvals; source workflow bytes remain inert. Git object history and attachment bytes need separate traversal/size budgets.

### Parallel Opportunities

Independent starting package; may run beside other independent packages once the runtime assigns distinct lanes.

### Dependencies

None.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. A bare Git archive of all refs could leak local/private or unrelated state. Foreign review observations must never be interpreted as native approvals; source workflow bytes remain inert. Git object history and attachment bytes need separate traversal/size budgets.

---

## Work Package WP05: Migration plan, apply/resume and local archive destinations (Priority: P1)

**Goal**: Provide one inspectable migration engine for repository/archive sources and destinations with explicit governance activation and guarded resume.
**Independent Test**: A saved plan imports a portable project into a fresh local repository with adapters/credentials absent, preserves destination governance/identity, reports every mapping/loss, and resumes interrupted progress safely.
**Prompt**: `tasks/WP05-migration-engine-local-archive.md`
**Requirement Refs**: FR-002, FR-003, FR-005, FR-007, FR-013, FR-014, FR-015, NFR-001, NFR-002

### Included Subtasks

T013 Specify plans, loss decisions and destination invariants
T014 Implement local/archive plan and guarded application
T015 Implement resume and prove offline continuation

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Importing actor histories into signer-owned refs or overwriting a destination policy can manufacture authority. Atomicity is limited across Git refs/files/external writes; expose manual reconciliation rather than guessing success after a crash.

### Parallel Opportunities

After WP02, WP04 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP02, WP04.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Importing actor histories into signer-owned refs or overwriting a destination policy can manufacture authority. Atomicity is limited across Git refs/files/external writes; expose manual reconciliation rather than guessing success after a crash.

---

## Work Package WP06: Shared provider contract and GitHub project snapshots (Priority: P1)

**Goal**: Extend optional adapters through a documented shared migration contract and deliver a real GitHub snapshot/destination implementation without breaking transfer commands.
**Independent Test**: GitHub fixtures preserve issue/comments, PR conversations/reviews, retrieved selected attachments and workflow source; supported destination issue operations recover safely; old transfer commands and nogithub retain their meaning, while an all-adapter-free build remains locally functional.
**Prompt**: `tasks/WP06-provider-contract-github-snapshots.md`
**Requirement Refs**: FR-004, FR-007, FR-008, FR-009, FR-013, FR-014, FR-015, NFR-001, NFR-002, NFR-003

### Included Subtasks

T016 Define adapter registry, capabilities and conformance contract
T017 Implement GitHub migration reader/writer and adapter exclusion
T018 Prove compatibility, bounds and recovery against fixtures

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Shared adapter code must not become a mandatory runtime service or vendor SDK. An export capability must be backed by actual writes and recovery tests; preservation-only data must not be advertised as destination-native recreation.

### Parallel Opportunities

After WP05 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP05.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Shared adapter code must not become a mandatory runtime service or vendor SDK. An export capability must be backed by actual writes and recovery tests; preservation-only data must not be advertised as destination-native recreation.

---

## Work Package WP07: GitLab migration and issue/comment transfer (Priority: P1)

**Goal**: Deliver non-vacuous GitLab Cloud/self-hosted capabilities with issue/comment read-write and attributed project communications preservation.
**Independent Test**: Versioned local GitLab fixtures exercise bounded issue/comment read/create/update, global IDs vs project-local IIDs, attached discussion topology, selected uploads/workflow source and uncertain write recovery; capabilities expose planning/export limits honestly.
**Prompt**: `tasks/WP07-gitlab-migration-adapter.md`
**Requirement Refs**: FR-004, FR-007, FR-008, FR-009, FR-013, FR-014, NFR-001, NFR-002

### Included Subtasks

T019 Write GitLab capability and API contract fixtures
T020 Implement the isolated Go GitLab adapter
T021 Run conformance, fidelity and lost-response tests

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. GitLab Discussions belong to issues/MRs and are not standalone Hubnot discussions. Namespaced paths can change; base identities and write selectors must remain distinct. Missing license/permission is not a complete empty collection.

### Parallel Opportunities

After WP06 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP06.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. GitLab Discussions belong to issues/MRs and are not standalone Hubnot discussions. Namespaced paths can change; base identities and write selectors must remain distinct. Missing license/permission is not a complete empty collection.

---

## Work Package WP08: Forgejo/Codeberg and Gitea migration adapters (Priority: P1)

**Goal**: Support separately tested Forgejo/Codeberg and Gitea profiles with issue/comment read-write, preserved PR reviews/assets and explicit capability/version differences.
**Independent Test**: Distinct Forgejo and Gitea fixtures pass shared adapter cases and their own endpoint/version/asset semantics; unavailable board APIs refuse or preserve source explicitly instead of claiming native project export.
**Prompt**: `tasks/WP08-forgejo-gitea-migration-adapters.md`
**Requirement Refs**: FR-004, FR-007, FR-008, FR-010, FR-013, FR-014, NFR-001, NFR-002

### Included Subtasks

T022 Define distinct profile and issue/comment fixtures
T023 Implement isolated adapters with truthful resource coverage
T024 Verify each profile independently

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Forgejo promises compatibility within a major, not with all Gitea releases. Codeberg instance capabilities and API availability are not universal. Shared code must not erase separate identities or inflate the advertised resource/direction matrix.

### Parallel Opportunities

After WP06 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP06.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Forgejo promises compatibility within a major, not with all Gitea releases. Codeberg instance capabilities and API availability are not universal. Shared code must not erase separate identities or inflate the advertised resource/direction matrix.

---

## Work Package WP09: Bitbucket Cloud repository and PR-conversation migration (Priority: P1)

**Goal**: Provide meaningful Bitbucket Cloud repository/PR-conversation/workflow-source support while explicitly refusing its removed native issue tracker.
**Independent Test**: Cloud fixtures preserve repository identity, PR activity/comments/inline-parent topology and pipeline source; issue selection fails before writes, opaque pagination is bounded, and no Jira or Data Center endpoint is silently substituted.
**Prompt**: `tasks/WP09-bitbucket-cloud-migration-adapter.md`
**Requirement Refs**: FR-004, FR-007, FR-008, FR-011, FR-013, FR-014, NFR-001, NFR-002

### Included Subtasks

T025 Specify currently available Bitbucket Cloud capabilities
T026 Implement Cloud snapshot and applicable destination operations
T027 Prove opaque pagination, provenance and unsupported behavior

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Removed issue APIs and obsolete authentication examples can create a superficially passing but unusable adapter. Workspace projects are repository grouping, not planning boards; source links or PR status observations grant no native authority.

### Parallel Opportunities

After WP06 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP06.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Removed issue APIs and obsolete authentication examples can create a superficially passing but unusable adapter. Workspace projects are repository grouping, not planning boards; source links or PR status observations grant no native authority.

---

## Work Package WP10: Azure DevOps mapped Work Items and PR conversations (Priority: P1)

**Goal**: Support Azure DevOps through explicit organization/project/repository and Work Item type/state mappings, with tested reads/writes and attributed PR/attachment preservation.
**Independent Test**: Azure fixtures enforce selectors/mappings, perform JSON Patch work-item creation and revision-guarded updates, transfer comments using continuation tokens, preserve selected relations/attachments and PR threads, and reject incomplete queries or guessed process mappings.
**Prompt**: `tasks/WP10-azure-devops-migration-adapter.md`
**Requirement Refs**: FR-004, FR-007, FR-008, FR-012, FR-013, FR-014, NFR-001, NFR-002

### Included Subtasks

T028 Specify Azure selectors, mappings and API-version fixtures
T029 Implement the Go Azure adapter and guarded operations
T030 Verify mapping fidelity, bounds and recovery

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Azure Work Item types/states are process-specific, not interchangeable open/closed issues. A project may contain many repositories. WIT comments use a preview-version API; endpoint versions and auth must be explicit and fixture-qualified.

### Parallel Opportunities

After WP06 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP06.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Azure Work Item types/states are process-specific, not interchangeable open/closed issues. A project may contain many repositories. WIT comments use a preview-version API; endpoint versions and auth must be explicit and fixture-qualified.

---

## Work Package WP11: Guided migration CLI and cross-provider acceptance (Priority: P1)

**Goal**: Expose one guided/noninteractive hn migrate flow that saves inspectable plans and proves a real GitLab-to-GitHub communications migration.
**Independent Test**: A real compiled CLI with local provider fixtures and temporary Git remotes captures decisions, previews without mutation, applies/resumes a saved plan, migrates code plus issues/comments between GitLab/GitHub, and preserves unsupported communications locally.
**Prompt**: `tasks/WP11-guided-migration-public-acceptance.md`
**Requirement Refs**: FR-005, FR-006, FR-007, FR-008, FR-009, FR-013, FR-015, FR-022, NFR-001, NFR-002, NFR-003

### Included Subtasks

T031 Specify interactive/noninteractive public behavior
T032 Implement guided command dispatch over the migration engine
T033 Prove cross-provider and adapter-free user journeys

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. A wizard that merely prints guessed commands or a cross-provider test that only copies Git objects is not sufficient. Its saved plan must faithfully bind observations, decisions and mutation intent; no default or EOF answer can consent to loss or authority activation.

### Parallel Opportunities

After WP05, WP07, WP08, WP09, WP10 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP05, WP07, WP08, WP09, WP10.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. A wizard that merely prints guessed commands or a cross-provider test that only copies Git objects is not sufficient. Its saved plan must faithfully bind observations, decisions and mutation intent; no default or EOF answer can consent to loss or authority activation.

---

## Work Package WP12: Integrated acceptance, documentation and release readiness (Priority: P1)

**Goal**: Integrate the independent capabilities, verify the full mission contract through public boundaries and document only shipped, tested behavior.
**Independent Test**: Ordinary/nogithub/adapter-free builds pass native/offline migration and audit acceptance; each named provider has non-vacuous fixture coverage; Go/Git-only packaging is reproducible; full charter gates and requirement coverage are recorded without changing public v0.0.1-dev artifacts.
**Prompt**: `tasks/WP12-integrated-docs-release-acceptance.md`
**Requirement Refs**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, FR-013, FR-014, FR-015, FR-016, FR-017, FR-018, FR-019, FR-020, FR-021, FR-022, NFR-001, NFR-002, NFR-003, NFR-004

### Included Subtasks

T034 Add integrated public acceptance and negative controls
T035 Synchronize operator docs, examples and capability limits
T036 Run release/readiness gates and trace all requirements

### Implementation Notes

Use the mission plan and contracts as canonical; file ownership below is intentionally narrow. Tests that each pass in isolated lanes can still fail together. Do not label unexercised hosts runtime-qualified, downgrade loss reports to success, omit failed-attempt evidence, or claim new provider functionality from capability-only stubs.

### Parallel Opportunities

After WP01, WP02, WP03, WP04, WP05, WP06, WP07, WP08, WP09, WP10, WP11 are approved, work only in this runtime-assigned lane. Provider packages may run in parallel only when finalized into distinct lanes.

### Dependencies

Depends on WP01, WP02, WP03, WP04, WP05, WP06, WP07, WP08, WP09, WP10, WP11.

### Risks & Mitigations

Retain the exact acceptance boundary and independently review this WP before approving dependents. Tests that each pass in isolated lanes can still fail together. Do not label unexercised hosts runtime-qualified, downgrade loss reports to success, omit failed-attempt evidence, or claim new provider functionality from capability-only stubs.

---

## Dependency & Execution Summary

- Independent roots: WP01 (packaging), WP02 (audit admission), WP04 (snapshot/archive).
- Audit chain: WP02 (including explicit private authority anchor) → WP03.
- Migration chain: WP02 + WP04 → WP05 → WP06 → WP07/WP08/WP09/WP10 → WP11.
- Final integration: every implementation WP → WP12. Finalized lane topology, not the orchestrator's guess, determines actual parallel dispatch.
- All WPs are required for the stated mission; a partial adapter set or preservation-only substitution must not be reported as complete native export.
- Freeze closed issue/comment schemas and verified-blob/dependency-receipt ApplyContext before parallel adapters. Engine-owned resolved payload/digest must survive uncertain resume.
- Keep review priority, role separation and charter gates. Do not duplicate a long gate on an unchanged source revision; reuse properly source-bound evidence where allowed.

## Requirements Coverage Summary

| Requirement ID | Covered By Work Package(s) |
| --- | --- |
| FR-001 | WP01, WP02, WP03, WP12 |
| FR-002 | WP04, WP05, WP12 |
| FR-003 | WP04, WP05, WP12 |
| FR-004 | WP04, WP06, WP07, WP08, WP09, WP10, WP12 |
| FR-005 | WP04, WP05, WP11, WP12 |
| FR-006 | WP11, WP12 |
| FR-007 | WP05, WP06, WP07, WP08, WP09, WP10, WP11, WP12 |
| FR-008 | WP06, WP07, WP08, WP09, WP10, WP11, WP12 |
| FR-009 | WP06, WP07, WP11, WP12 |
| FR-010 | WP08, WP12 |
| FR-011 | WP09, WP12 |
| FR-012 | WP10, WP12 |
| FR-013 | WP05, WP06, WP07, WP08, WP09, WP10, WP11, WP12 |
| FR-014 | WP04, WP05, WP06, WP07, WP08, WP09, WP10, WP12 |
| FR-015 | WP04, WP05, WP06, WP11, WP12 |
| FR-016 | WP02, WP12 |
| FR-017 | WP03, WP12 |
| FR-018 | WP02, WP03, WP12 |
| FR-019 | WP02, WP03, WP12 |
| FR-020 | WP03, WP12 |
| FR-021 | WP01, WP12 |
| FR-022 | WP01, WP03, WP11, WP12 |
| NFR-001 | WP02, WP03, WP04, WP05, WP06, WP07, WP08, WP09, WP10, WP11, WP12 |
| NFR-002 | WP02, WP04, WP05, WP06, WP07, WP08, WP09, WP10, WP11, WP12 |
| NFR-003 | WP03, WP06, WP11, WP12 |
| NFR-004 | WP01, WP12 |

## Subtask Index (Reference)

| Subtask ID | Summary | Work Package | Priority | Parallel? |
| --- | --- | --- | --- | --- |
| T001 | Specify deterministic package inputs and adversarial archive/output tests | WP01 | P1 | No; sequence within WP |
| T002 | Implement the release command and archive/checksum generation | WP01 | P1 | No; sequence within WP |
| T003 | Verify repeatability and portable packaging behavior | WP01 | P1 | No; sequence within WP |
| T004 | Define audit envelope/payload and permission contracts through tests | WP02 | P1 | No; sequence within WP |
| T005 | Implement canonical audit validation and fetched admission | WP02 | P1 | No; sequence within WP |
| T006 | Prove replication and non-authority behavior | WP02 | P1 | No; sequence within WP |
| T007 | Specify public request/execute/inspect/follow-up scenarios | WP03 | P1 | No; sequence within WP |
| T008 | Implement explicit audit execution and inspection | WP03 | P1 | No; sequence within WP |
| T009 | Implement deliberate follow-ups and recovery | WP03 | P1 | No; sequence within WP |
| T010 | Specify snapshot/archive contracts and hostile input cases | WP04 | P1 | No; sequence within WP |
| T011 | Implement deterministic snapshot and archive read/write | WP04 | P1 | No; sequence within WP |
| T012 | Verify exact preservation and isolation | WP04 | P1 | No; sequence within WP |
| T013 | Specify plans, loss decisions and destination invariants | WP05 | P1 | No; sequence within WP |
| T014 | Implement local/archive plan and guarded application | WP05 | P1 | No; sequence within WP |
| T015 | Implement resume and prove offline continuation | WP05 | P1 | No; sequence within WP |
| T016 | Define adapter registry, capabilities and conformance contract | WP06 | P1 | No; sequence within WP |
| T017 | Implement GitHub migration reader/writer and adapter exclusion | WP06 | P1 | No; sequence within WP |
| T018 | Prove compatibility, bounds and recovery against fixtures | WP06 | P1 | No; sequence within WP |
| T019 | Write GitLab capability and API contract fixtures | WP07 | P1 | No; sequence within WP |
| T020 | Implement the isolated Go GitLab adapter | WP07 | P1 | No; sequence within WP |
| T021 | Run conformance, fidelity and lost-response tests | WP07 | P1 | No; sequence within WP |
| T022 | Define distinct profile and issue/comment fixtures | WP08 | P1 | No; sequence within WP |
| T023 | Implement isolated adapters with truthful resource coverage | WP08 | P1 | No; sequence within WP |
| T024 | Verify each profile independently | WP08 | P1 | No; sequence within WP |
| T025 | Specify currently available Bitbucket Cloud capabilities | WP09 | P1 | No; sequence within WP |
| T026 | Implement Cloud snapshot and applicable destination operations | WP09 | P1 | No; sequence within WP |
| T027 | Prove opaque pagination, provenance and unsupported behavior | WP09 | P1 | No; sequence within WP |
| T028 | Specify Azure selectors, mappings and API-version fixtures | WP10 | P1 | No; sequence within WP |
| T029 | Implement the Go Azure adapter and guarded operations | WP10 | P1 | No; sequence within WP |
| T030 | Verify mapping fidelity, bounds and recovery | WP10 | P1 | No; sequence within WP |
| T031 | Specify interactive/noninteractive public behavior | WP11 | P1 | No; sequence within WP |
| T032 | Implement guided command dispatch over the migration engine | WP11 | P1 | No; sequence within WP |
| T033 | Prove cross-provider and adapter-free user journeys | WP11 | P1 | No; sequence within WP |
| T034 | Add integrated public acceptance and negative controls | WP12 | P1 | No; sequence within WP |
| T035 | Synchronize operator docs, examples and capability limits | WP12 | P1 | No; sequence within WP |
| T036 | Run release/readiness gates and trace all requirements | WP12 | P1 | No; sequence within WP |

