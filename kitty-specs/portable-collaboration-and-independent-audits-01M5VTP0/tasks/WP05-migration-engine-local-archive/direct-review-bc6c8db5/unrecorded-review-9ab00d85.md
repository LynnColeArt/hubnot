# WP05 independent review — changes requested

Reviewed source: `9ab00d85dee0fac114f303d5eecd2cd7e00f5cae`, lane-d.
Reviewer: Codex (independent review, no product edits).

## Blocking checklist finding: production reachability

The supplied review checklist explicitly requires every new public function/class/module to have a production caller, excluding tests, and says any FAIL blocks approval. At this head, targeted searches of all Go production files find declarations but no callers for `PrepareMigration` (migration_engine.go:26), `ApplyMigration` (:236), `InspectMigration` (:365), `ComposeMigrationDestinations` (:376), `ComposeMigrationSources` (:427), `ListLocalMigrations` (migration_local.go:499), and `DecodeReconciliation` (internal/migration/plan.go:488).

Reproduce in lane-d:

```sh
rg -n '\b(PrepareMigration|ApplyMigration|InspectMigration|ComposeMigrationSources|ComposeMigrationDestinations|DecodeReconciliation|ListLocalMigrations)\b' --glob '*.go' --glob '!**/*_test.go'
```

The engine has substantial real implementation and meaningful production-boundary tests; this finding is specifically the required live-call-site gate, not a claim that the implementation is a synthetic fixture. The architecture deliberately assigns CLI consumers to WP11 and providers to WP06 onward. Previous feedback treated those planned consumers as sufficient, but that is an exception to the explicit checklist supplied for this review, not an existing production caller.

**Required resolution:** coordinate with the mission owner to reconcile this gate with the dependency-first delivery plan: either record an explicit authorized WP-level exception for the intentionally deferred integration APIs, with the actual reachability gate retained at integration, or supply real production consumers in their assigned ownership scope before approval. Do not add dummy callers or pull other WPs' CLI/provider files into WP05. Independently remove unused helper APIs such as `DecodeReconciliation`, or connect them to a justified production consumer if they are needed. No behavior defect in the corrected hook path was found in this review.

## Prior corrections verified

The destination ref transaction is anchored with `-C` to a destination directory resolved without caller-cwd discovery. Bare, ordinary and linked-worktree refusal regressions pass. Source hooks are not executed by these cases; destination hook refusal prevents observation-ref visibility. Authenticated initial progress and pre-effect destination locator checks remain present; recovery tests pass, including exact dependent comment payload reuse and no blind repeat after missing state/negative reconciliation.

## Required checklist

1. **Dead code — FAIL:** public APIs above have no production callers at this source. Deferred WP06/WP11 wiring requires an explicit gate exception or completed integration.
2. **Synthetic-fixture test — PASS:** plan/progress tests invoke real validators; local/archive/native/recovery tests exercise actual engine and Git boundaries.
3. **Silent empty return — PASS:** examined empty returns represent missing refs, absent optional context or successful guards; uncertainty becomes no-proof/manual recovery, not retry permission.
4. **FR coverage — PASS for WP-scoped behavior assertions:** FR-002/003 audit/native byte retention and private identity/governance preservation; FR-005 local/archive/composed transport; FR-007 closed schemas and explicit loss decisions; FR-013 persisted dependent writes, lost acknowledgements, missing checkpoints and changed destinations; FR-014 inert imported content and hook isolation; FR-015 provider-neutral offline code boundaries. Actual adapter exclusion remains WP06 integration work; noforge tag compatibility is not proof of exclusion. NFR-001/002 bounded blob/native closure and hostile admission are exercised.
5. **Frozen surface — PASS:** no post-dependency changes to frozen snapshot schema/archive, snapshot tests or project-migration documentation (`git log a99daad2..HEAD -- internal/migration/snapshot.go internal/migration/archive.go project_snapshot_test.go docs/project-migration.md` is empty). No signed historical bytes or release artifacts changed by WP05.
6. **Locked decision — PASS:** previous hook-execution failure corrected; examined paths preserve explicit authority, no source deletion/implicit workflow activation, expected-old guards and uncertain-write reconciliation.
7. **Shared-file ownership — PASS:** project_snapshot.go maintenance has the explicit WP04/WP05 ownership handoff and integration advice; latest correction changes only WP05-owned native bridge/test files.
8. **Production fragility — PASS:** new Go errors refuse unsafe repository shapes and uncertain effects; no new panic or automatic retry of uncertain creates found.

## Verification and limits

- Independently passed `go test -race -run '^TestMigration' -skip '^TestMigration(AuthenticatedHTTPSGitRedactionAndFreshResumeCredentials|HTTPSIgnoresAmbientCredentialStores)$' -count=1 .` (7.056s), including hook regressions for all three repository layouts.
- Independently passed `go test -race ./internal/migration -count=1` (1.062s) and `git diff --check a99daad2..HEAD`.
- The unfiltered focused run and a subsequent run excluding the first TLS test both stopped at TLS fixtures because this sandbox denies loopback socket creation (`socket: operation not permitted`). This is an environment limitation, not a demonstrated product regression. Both TLS cases are covered in the archived successful full race run.
- Recomputed every checksum in correction-9ab00d85/SHA256SUMS and all five source hashes in source-binding.json. Verified archived race events: all four packages pass, no failure events. Reused exact-head full race/vet/build/format and nogithub evidence. No claim of independently rerunning the full network-capable suite or of actual adapter exclusion.

## Dependency notification

WP06, WP11 and WP12 owners: WP05 is returned to planned solely for the explicit production-reachability gate above. Hold consumption/approval until the gate-versus-sequencing conflict is resolved; rebase on any corrected WP05 head before proceeding. No dependent agents are active in this reviewer's agent tree, so this committed feedback is the available handoff notification. No other WP status was changed.
