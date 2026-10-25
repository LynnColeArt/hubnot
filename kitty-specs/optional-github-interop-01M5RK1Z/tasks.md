# Work Packages: Optional GitHub issue interoperability

Planning/base/merge target: `feat/optional-github-interop`.

## Work Package WP01: Local gh-style issue commands

**Dependencies**: None

**Requirement Refs**: FR-001, FR-002, FR-009

**Prompt**: `tasks/WP01-local-gh-style-issue-commands.md`

T001 Define local compatibility lifecycle and JSON/filter behavior tests first.
T002 Implement cmdGH and strict interspersed long/short flag parsing for the documented issue subset.
T003 Translate supported reads/mutations through existing native issue functions and validation with actor pinning and conflict refusal.
T004 Validate offline behavior, precise errors, script JSON fields and preservation of existing native state.

## Work Package WP02: Bounded optional GitHub REST client

**Dependencies**: None

**Requirement Refs**: FR-003, FR-006, FR-008, NFR-001

**Prompt**: `tasks/WP02-bounded-optional-github-rest-client.md`

T005 Define provider contract and HTTP fixture tests for pagination, redirects, credentials and failures first.
T006 Implement strict explicit base URL/repository validation and sanitized bounded HTTP requests without write retries.
T007 Implement issue/comment reads and writes, complete pagination and PR filtering.
T008 Verify client bounds, same-origin handling and omission from nogithub builds.

## Work Package WP03: Native GitHub import and optional command boundary

**Dependencies**: WP01, WP02

**Requirement Refs**: FR-003, FR-004, FR-005, FR-008, FR-009, FR-011, FR-012, NFR-001

**Prompt**: `tasks/WP03-native-github-import-and-optional-command-boundary.md`

T009 Specify native import/repeat/update/conflict and optional-build behavior tests first.
T010 Wire hn gh and hn github dispatch; parse strict import/export commands and optional client credentials.
T011 Implement complete native source discovery and validated attributed issue/comment imports.
T012 Implement guarded three-way updates, safe operation retries and checkpoint helpers for later export.
T013 Validate real native storage against fake paginated provider and nogithub build.
T023 Implement provider-neutral models, adapter boundary and pre-write migration capability reports.

## Work Package WP04: Explicit recoverable issue export

**Dependencies**: WP03

**Requirement Refs**: FR-004, FR-005, FR-006, FR-007, FR-008, FR-011, FR-012, NFR-001

**Prompt**: `tasks/WP04-explicit-recoverable-issue-export.md`

T014 Write explicit export, repeat/resume and remote-conflict fixture tests first.
T015 Preflight selected native issue/comments and derive stable publication identity/markers.
T016 Implement signed provenance-aware create/update export and lifecycle/comment publication.
T017 Implement bounded reconciliation/checkpoints for uncertain creation and conservative competing-edit refusal.
T018 Validate no duplicate issues/comments, no blind retries, no token leaks and actionable recovery.

## Work Package WP05: Offline acceptance and operator documentation

**Dependencies**: WP01, WP04, WP07, WP08, WP09, WP11, WP12, WP13

**Requirement Refs**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, NFR-001, NFR-002, NFR-003, C-001, C-002, C-003

**Prompt**: `tasks/WP05-offline-acceptance-and-operator-documentation.md`

T019 Add independent public CLI lifecycle and transport-disabled acceptance checks.
T020 Verify imported signed issues remain usable without token, gh, provider or transport build.
T021 Document exact command/flag/JSON subset, provenance, conflicts, limits and recovery constraints.
T022 Run final full gates and record actual results, fixing only owned acceptance/docs or reporting implementation defects.

## Work Package WP06: Signed native resource engine

**Dependencies**: WP03

**Requirement Refs**: FR-013, FR-014, FR-015, FR-016, NFR-001, NFR-002, C-001, C-002

**Prompt**: `tasks/WP06-signed-native-resource-engine.md`

T024 Define strict native resource states and hostile/signature bounds tests.
T025 Implement canonical project/item/discussion/reply/workflow state validation.
T026 Register resource event kinds and actor-global replay request digests without weakening existing validators.
T027 Implement bounded typed projections, independent aggregate heads, conflicts and exact-head mutations/resolution.
T028 Integrate relationship/type/schema/thread dependency validation in reader, quarantine and shallow recovery.
T029 Provide reusable native query/mutation and generic CLI registration seams.
T030 Prove signed cross-actor bare-remote reconstruction, missing dependency quarantine and old-byte preservation; document protocol boundary.

## Work Package WP07: Native project and work-item commands

**Dependencies**: WP06

**Requirement Refs**: FR-014, FR-016

**Prompt**: `tasks/WP07-native-project-and-work-item-commands.md`

T031 Write public/native project lifecycle and typed item tests first.
T032 Implement project open/list/show/revise/resolve and field/view input UX over engine.
T033 Implement item add/show/list/revise/resolve/archive with exact schema pinning and target validation.
T034 Verify conflicts, source provenance preservation, actor replay and offline typed values.

## Work Package WP08: Native discussion and reply commands

**Dependencies**: WP06

**Requirement Refs**: FR-013, FR-016

**Prompt**: `tasks/WP08-native-discussion-and-reply-commands.md`

T035 Write thread/reply/answer/lifecycle and conflict tests first.
T036 Implement discussion open/list/show/revise/resolve/close/reopen/category operations.
T037 Implement reply add/edit/resolve and explicit accepted-answer revision selection/clear.
T038 Verify thread bounds, same-thread refs, pinned answer content, offline identity/replay and metadata preservation.

## Work Package WP09: Native workflow authoring and governed execution

**Dependencies**: WP06

**Requirement Refs**: FR-015, FR-016

**Prompt**: `tasks/WP09-native-workflow-authoring-and-governed-execution.md`

T039 Write workflow definition/source/materialize/request safety tests first.
T040 Implement workflow open/list/show/revise/resolve and native/foreign runnable status.
T041 Implement explicit safe materialization of canonical native pipeline into repository pipeline file.
T042 Verify exact workflow/proposal pipeline digest binding before existing governed run request and preserve runner/policy limits.

## Work Package WP10: Provider-neutral native resource migration bridge

**Dependencies**: WP06, WP03, WP04

**Requirement Refs**: FR-011, FR-012, FR-016, FR-020

**Prompt**: `tasks/WP10-provider-neutral-native-resource-migration-bridge.md`

T043 Define neutral multi-resource transfer plan/reference/capability contracts and fake-provider tests.
T044 Implement typed native resource import/export graph helpers with source baselines and dependency mappings.
T045 Integrate registry dispatch by provider/resource while preserving issue path and generic dry-run/mapping reports.
T046 Implement bounded journal/reconciliation primitives shared by resource adapter mutations.
T047 Verify non-GitHub opaque identities, cross-resource remapping, unsupported/no-write and no-core-provider dependency.

## Work Package WP11: Optional GitHub discussion migration

**Dependencies**: WP08, WP10

**Requirement Refs**: FR-004, FR-008, FR-011, FR-012, FR-013, FR-017, FR-020

**Prompt**: `tasks/WP11-optional-github-discussion-migration.md`

T048 Add bounded sanitized GraphQL transport and complete nested pagination tests.
T049 Implement discussion/category/reply/answer/state reads and typed mutations via optional adapter.
T050 Map native threads/replies/answer revisions and require explicit destination category mapping.
T051 Implement repeatable journaled discussion export/import, conflicts and uncertain mutation recovery.
T052 Verify no implicit moderation authority, no lossy hidden fields, and nogithub exclusion.

## Work Package WP12: Optional GitHub project migration

**Dependencies**: WP07, WP10, WP11

**Requirement Refs**: FR-004, FR-008, FR-011, FR-012, FR-014, FR-018, FR-020

**Prompt**: `tasks/WP12-optional-github-project-migration.md`

T053 Define explicit user/org project selectors and typed field/item mapping fixtures.
T054 Implement complete GraphQL project/field/item/value reads and supported writes.
T055 Map native project schema/items/targets and report unsupported layouts/types/automation before writes.
T056 Implement checkpointed project export/import with field/option/content ID remapping and item ordering/state.
T057 Verify clone independence, reference integrity, repeat/recovery and no false universal workflow claims.

## Work Package WP13: Optional GitHub workflow migration

**Dependencies**: WP09, WP10, WP11

**Requirement Refs**: FR-004, FR-008, FR-011, FR-012, FR-015, FR-019, FR-020

**Prompt**: `tasks/WP13-optional-github-workflow-migration.md`

T058 Write exact definition/source/pinned revision and inert-import fixtures.
T059 Implement bounded Actions workflow discovery and contents read/write at explicit commit/branch with blob SHA.
T060 Map native workflow sources and require explicit runnable/environment transformation choices.
T061 Implement exact source restore and constrained native-to-Actions export with clear semantic report/manifest.
T062 Verify no autoexecution, content-CAS conflicts, recovery, roundtrip bytes and transport exclusion.
