# WP10 Azure validation record

Source: `29771aed34e68503c6a4c3f660cbb56f2a335ab0`.
Dependency base: approved WP06 `dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116`.

Implementation provides explicit scoped Azure Services/Server API profiles and supplied bearer/PAT authentication, actual Work Item/comment reads and creates, revision-tested updates, engine-persisted parent receipt resolution and exact-positive recovery. Project/repository/Work Item/revision identities remain distinct. Retrieved custom fields/relations, attachment bytes, PR/thread/inline topology and selected inert YAML/classic source remain inspectable. Classic service-managed credential values are excluded with explicit findings; malformed declarations refuse and preservation hashes use sanitized bytes only.

The root-authorized shared ownership exception corrects existing generic destination scope in `internal/migration/plan.go` and adds `plan_test.go` coverage. A valid explicit Project OR Repository is required, with both independently validated; actual Boards-only saved-session apply succeeds. No shared JSON/interface/receipt/write schema changed. The primary `shared-plan-scope-handoff.md` records authorization.

## Executed checks

- Final-source focused race: azureapi 1.112s, migration 1.023s, root 3.964s, exit 0.
- Distinct post-race checks all exit 0: vet, build, applicable nogithub/noforge focused race, three build variants, actual offline native issue/gh-facade CLI use with Git-only PATH/empty credentials/isolated HOME, legacy Azure capability reports, modules, actual dependency/file/symbol/build-info exclusions, formatting, diff and unchanged tags.
- First final-source full race attempt exited 1 after 752.806s. Unchanged `TestMemoryIndexTenThousandRecordsPerformanceAndRecovery` measured query p95 1.077407267s against a 1-second limit (memory_index_test.go:1119). No Azure failure or data race was reported. Raw failure is retained as race-attempt1.log/.exit. This is a performance failure, not a timeout; concurrent load is not claimed as proven cause.
- The ONE coordinator-authorized serialized, unchanged-head, unfiltered full race rerun PASSED (exit 0): hubnot 600.198s. Exact command: `go test -race -timeout 20m ./...`. Start 2026-10-27T04:00:07Z, completion recorded 2026-10-27T04:11:00Z. Source hashes checked before and after; working tree clean. Distinct passing static/tag/native checks were reused at the same head. No assertion, CI/default configuration or test filter was weakened. Attempt1 remains a real failure; this pass does not prove its cause.

The earlier a7b0d620 ordinary command hit its default cumulative 600-second package deadline in an existing policy test. Subsequent 98dd4e94 passed a full finite-20-minute ordinary/static/tagged/native sequence but preceded the credential fix. Both are retained as superseded evidence and do not stand in for final-source checks.

## Responsibility-boundary evidence

- Selectors/auth/version/process/state/type: `TestAzureMigrationOfflineRegistrationAndSelectors`, `TestExplicitVersionsCredentialsAndOfflineConstruction`, `TestAzureMigrationExplicitProcessTypeStateAndCapacity`, `TestAzureMigrationServerPATReadsAndWrites`.
- Complete bounds/hostile reads: `TestWIQLExhaustsFullPageAndRefusesIncompleteDetail`, `TestCommentsContinuationAndHonestCompleteness`, `TestRedirectAndForeignBlobNeverForwardCredentials`, `TestSharedWireExactBudgetAndExcess`, `TestAzureMigrationSnapshotBudgetsExactAndExcess`, unexpected thread cursor and missing attachment/unknown-kind findings.
- Engine and Git integration: `TestAzureMigrationSharedGitCompanionAndSecretFreeArchive`, exact lost Work Item/comment responses and persisted parent payloads, absent/ambiguous proof without duplicate effects, immutable project/repository/process/API-profile/parent replacement refusal, successful update and `/rev` conflict, exact marker capacity and excess refusal.
- Preservation/no authority: `TestAzureMigrationSnapshotPreservesCustomFieldsAttachmentsAndPRTopology`, full project metadata, inert pinned YAML/classic source, `TestAzureMigrationClassicServiceManagedCredentialsExcluded`, malformed credential-declaration refusal and precision/no-original-secret-fingerprint checks.
- Shared generic scope: `TestPlanDestinationScopeKeepsProjectAndRepositorySeparate`, real Boards-only engine apply. Legacy compatibility: explicit discoverable refusal where the old issue-only API cannot express Azure inputs.

No mandatory module, provider CLI/SDK/service/Python product dependency, live provider write, source deletion, workflow execution/adoption, public push/merge/tag/release or unrelated automation change occurred. All API effects are local HTTP fixtures; selected code uses real temporary Git remotes. Build/client registration files and tests use !noforge; nogithub retains Azure. Public migration CLI integration remains WP11.

Raw archive generation admits only flat regular text evidence and excludes all generated binaries, native fixture repositories, symlinks and private fixture HOME contents. Source hashes, outer checksums and every archive member digest are verified. See interface-notes.md and classic-schema-walk.md for exact contract and official API references.

## Canonical lifecycle

T028, T029 and T030 were marked done with the canonical command. Normal WP10 transition to `for_review` completed with event `01M5XRZA8W13PVNRDPQNGRQTX3`, agent codex / implementer-ivan. No self-approval, force or skip was used. The transition note explicitly names the root-owned shared-scope authorization `1dd7f7e0`.

The pre-review framework invocation was polled to completion and reports `no_coverage`: `tests.architectural._gate_coverage` is not importable (`No module named tests.architectural`). This is an unverified framework authority, not a Go test pass or failure. The actual source-bound Go checks above passed. Two unrelated dirty primary files were reported ignored by the tool and were not changed or investigated.
