# WP10 independent review

Verdict: **APPROVE**. No concrete blocking finding in the fixed source delta.

- Reviewer: `codex-azure-independent-review`, agent `codex`, profile `reviewer-renata`; independent of implementation.
- Reviewed head: `29771aed34e68503c6a4c3f660cbb56f2a335ab0` in lane-i.
- Approved dependency: `dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116` (WP06).
- Review completed: 2026-10-27 UTC.
- Scope: eight Azure-owned files plus the explicitly authorized `internal/migration/plan.go` and `plan_test.go` exception. No product/test edits made by reviewer.

Read the complete generated review prompt, root AGENTS.md, resolved Reviewer Renata profile, charter/review checklist, spec, substantive plan, research, data model, quickstart, contracts and WP10 prompt. Reviewed the actual ten-file delta and all three source commits, production paths and tests, and the retained final evidence at `validation-29771aed` (primary evidence commit `6f3272046c5eb2573c584ca18e76b65e390d84af`).

## Behavior and requirements

| Requirement | Assessment and executed assertion boundary |
| --- | --- |
| FR-004 | PASS. Snapshot HTTP paths retain Work Item original fields/relations, attributed comments, complete project JSON, PR detail and thread/inline/reply topology, exact selected attachment/YAML bytes and sanitized classic definitions. `TestAzureMigrationSnapshotPreservesCustomFieldsAttachmentsAndPRTopology`, `TestAzureMigrationSelectedProjectPreservesCompleteRetrievedMetadata`, `TestAzureMigrationSharedGitCompanionAndSecretFreeArchive`, and classic-definition tests invoke production Snapshot/archive operations. Original URLs alone are not claimed downloaded blobs. |
| FR-007 | PASS. Explicit process/type/state and omitted-label/assignee mappings precede intents; unsupported recreation, inaccessible collections/attachments, unknown kinds and missing dependencies produce findings. `TestAzureMigrationExplicitProcessTypeStateAndCapacity`, `TestAzureMigrationAttachmentAuthorizationAndUnknownKindAreHonest`, and marker capacity cases check production planning/refusal. Required loss decisions remain enforced by the existing plan engine. |
| FR-008 | PASS. Azure registers Source/Destination constructors through the approved common registry. All exported Azure client operations have actual production adapter callers; methods implement the existing engine interfaces. Legacy transfer discovery honestly refuses before network I/O because its old inputs cannot express Azure process/API/auth mapping (`TestAzureLegacyCapabilitiesExplainMappedMigrationBoundary`). Public migration command assembly is the explicitly downstream WP11 responsibility; no dummy caller was required. |
| FR-012 | PASS. Organization/collection, planning project and optional Git repository stay distinct. Services 7.1 and Server 2022 7.0 use separately qualified comment preview versions and supplied bearer/PAT modes. Actual Work Item read/create/update and comment read/create succeed through saved sessions, including a repository-free Boards destination. `TestAzureMigrationServerPATReadsAndWrites`, offline registration/selectors, successful revision-guarded update, and API-client tests assert real requests and returned identities. |
| FR-013 | PASS. Every mutation re-observes project/repository/process/API profile; Work Item writes also check type/state vocabulary. Update uses immutable identity, expected revision and JSON Patch `/rev`/type tests. Comments consume confirmed engine receipts and persisted bound parent payloads. Lost responses require one exact marker plus complete supported projection; absent/ambiguous proof cannot duplicate creates. Lost-response, immutable replacement/revision, ambiguous recovery, and changed API/parent tests exercise the actual engine across reopened sessions and fresh adapters. |
| FR-014 | PASS. Imported PR/review/workflow/project material remains preserved data. No execution, permission/queue adoption, native signer, native approval or policy authority path is introduced. Classic top/group secret variable values and data-source authorization-header values are excluded with explicit findings. Tests assert excluded-value changes leave canonical snapshot digest unchanged, sanitized-only revision/blob digests, ordinary metadata and >2^53 numeric precision retained, and malformed/duplicate/case-aliased declarations refused. This is not arbitrary YAML/Git/task-string secret detection. |
| NFR-001 | PASS for WP10. WIQL/detail/comment/PR/thread reads have finite pages, bytes and record bounds; snapshots enforce additional record/blob/per-blob/aggregate budgets. Exact-bound and excess tests exercise production reads/building/planning. A full WIQL page is exhausted rather than accepted as complete; detail batches must return all selected unique IDs. |
| NFR-002 | PASS for WP10. Endpoint/path validation, redirect refusal and exact-origin download checks prevent credential forwarding. Comment pagination rejects missing/conflicting/cyclic cursors and incomplete totals; threads reject an undocumented continuation cursor. Existing archive admission remains unchanged and the Git/archive integration test uses that production boundary. |

The source is nonvacuous: Work Item/comment reads and creates plus guarded Work Item updates execute against local API fixtures, while the Git companion uses real temporary Git repositories/remotes. Unsupported fields and unqueried planning administration are declared, rather than fabricated or claimed preserved.

## API model cross-check

Checked Microsoft's primary documentation during review: [REST versioning](https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rest-api-versioning?view=azure-devops), [comment pagination/version](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/get-comments?view=azure-devops-rest-7.1), [Server comment add](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/add?view=azure-devops-rest-7.0), [Work Item type states](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-item-types/get?view=azure-devops-rest-7.1), [JSON Patch update](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/update?view=azure-devops-rest-7.1), [PR list](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-requests?view=azure-devops-rest-7.1), [thread list](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-threads/list?view=azure-devops-rest-7.1) and [Build Definition schema](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/definitions/get?view=azure-devops-rest-7.1).

The code retrieves PR details after the documented truncated list response, uses documented comment continuation parameters, refuses unexpected thread cursors, and sanitizes the documented top/group variable and process data-source header paths. No live account/provider mutation was used for review.

## Eight prompt checklist items

1. **Dead code — PASS.** Targeted production call-site searches show `migration_azure.go` imports/calls the new Azure module, uses its operations, and registers real interface constructors. Existing registry/engine seams invoke Source/Destination methods. WP11 owns final public command wiring by the approved dependency sequence.
2. **Synthetic-fixture test — PASS.** Fixtures supply provider responses and source inputs; assertions exercise actual client HTTP requests, adapter Snapshot/Plan/Apply/Reconcile, persisted sessions and archives. Removing implementation removes or breaks these behavior paths.
3. **Silent empty return — PASS.** Empty builder output is accompanied by sticky errors returned before Snapshot succeeds; absent blob/identity errors refuse. Unavailable resources are explicit findings, unsupported mappings are explicit findings, and no-proof/ambiguous reconciliation returns an explicit outcome. Field/actor accessors have intentional optional-field semantics; required identities/fields are checked at their operation boundaries.
4. **FR coverage — PASS.** Every requirement_ref has concrete behavior assertions identified above; no all-unsupported substitute for required Work Item/comment coverage.
5. **Frozen surface — PASS.** No changed signed/native encoding, dependency manifest, policy/default/test configuration, public release artifact or tag. Frozen exported migration types/JSON/write/receipt/ApplyContext contracts are unchanged. The one generic destination scope correction changes no schema.
6. **Locked decision — PASS.** No blind retries, cross-origin credentials, OAuth registration, source deletion, arbitrary workflow execution, authority adoption, mandatory service/SDK/provider CLI/module, or parallel provider-name branches in the core were introduced.
7. **Shared-file ownership — PASS.** Primary `shared-plan-scope-handoff.md`, coordination commit `1dd7f7e0`, authorizes exactly the two shared plan files. The predicate now requires Project OR Repository, preserves independent field validation and refuses organization-only scope. The generic test and real Boards-only saved-session apply validate that exception; the for-review note records it.
8. **Production fragility — PASS.** Go error returns fail loudly for invalid/missing mappings, changed immutable baselines, revision conflicts, bounds and uncertain outcomes; they do not silently retry effects. The sole new panic guards duplicate/invalid static registration at init and performs no network I/O. Transient HTTP failures remain uncertain progress requiring positive reconciliation or intervention.

## Independent verification and retained limitations

Independently recomputed **10 outer SHA-256 checksums**, **10 source hashes against both current lane files and the fixed commit**, and **all 81 inner archive member hashes**. Archive members are unique, flat regular files; none is a link or special file. Independently inspected the source-bound race rerun script, exact command/head, before/after hash results, and raw pass/failure evidence. `git diff --check` also passed independently.

Executed independently at the fixed head:

```text
go test -race -count=1 -timeout 5m ./internal/azureapi ./internal/migration . -run 'Azure|WIQL|CommentsContinuation|WorkItemWrite|ExplicitVersions|RedirectAndForeignBlob|SharedWire|PlanDestinationScope'
ok hubnot/internal/azureapi 1.112s
ok hubnot/internal/migration 1.021s
ok hubnot 4.068s
```

Retained final-source evidence supplies the distinct passing vet/build/format/diff, ordinary/nogithub/noforge variants, actual Git-only-PATH/isolated-HOME/empty-credential native CLI checks, legacy capabilities, modules, actual dependency/file/symbol/build-info exclusions and unchanged-tag checks. Reviewed their command/result evidence rather than duplicating a full suite or the memory benchmark while another WP owns the serialized slot.

The first final-source full race genuinely failed unchanged `TestMemoryIndexTenThousandRecordsPerformanceAndRecovery`: p95 **1.077407267s > 1s**. This was a performance assertion failure, not a timeout; cause is not established. The one coordinator-authorized serialized same-head **unfiltered** `go test -race -timeout 20m ./...` then passed, root package **600.198s**, exit 0, with unchanged source hashes. Assertions/defaults/CI/filter were not relaxed, and the original failure remains retained. Earlier a7 timeout and 98 passing evidence are superseded.

The framework architectural helper remains **UNVERIFIED / no_coverage** because `tests.architectural._gate_coverage` is unavailable; this is not counted as a Go pass. This report approves the reviewed WP10 behavior and verified Go evidence within its declared capability limits, not live-host qualification or downstream WP11 CLI completion.

The lifecycle verdict must be captured by the canonical command with this report commit as approval reference and then read from durable `review_result` event authority. This report does not itself change lifecycle truth.
