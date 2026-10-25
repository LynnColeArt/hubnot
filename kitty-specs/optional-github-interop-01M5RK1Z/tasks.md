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

**Dependencies**: WP01, WP04

**Requirement Refs**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, NFR-001, NFR-002, NFR-003, C-001, C-002, C-003

**Prompt**: `tasks/WP05-offline-acceptance-and-operator-documentation.md`

T019 Add independent public CLI lifecycle and transport-disabled acceptance checks.
T020 Verify imported signed issues remain usable without token, gh, provider or transport build.
T021 Document exact command/flag/JSON subset, provenance, conflicts, limits and recovery constraints.
T022 Run final full gates and record actual results, fixing only owned acceptance/docs or reporting implementation defects.
