# WP06 independent review — approved

Reviewer: codex / reviewer-renata (Reviewer Renata), independent of the implementation author.
Source: `dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116`.
Approved dependency base: `247a4e1b26295459ae159674c109bcd91aa1a954` (WP05).
Review invocation: `a2b43a1770d343bb88bce28e06878db2`; the existing canonical review claim was reused without reclaiming or rewinding.

## Verdict and scope

APPROVED: no blocking finding in the 28-file WP06 diff against the approved WP05 base. Earlier incoming work was not re-reviewed as WP06. Read the complete generated review prompt, primary spec, substantive plan, research, data model, quickstart, migration contract, root AGENTS.md, project charter and resolved reviewer profile/action-scoped governance. No product code was edited.

The shared registry is inert at registration/construction, supports disjoint provider registrations, and validates a nonempty useful adapter-owned coverage baseline. GitHub supplies actual issue/comment reads and writes, while PR/review/inline-thread, discussion/reply, separately selected planning, workflows and retrieved attachments are preservation-only. Unavailable content has explicit findings. Capabilities disclose open/untyped issue limitations, explicit state/type/label/assignee mappings, marker augmentation, no attachment upload, no author impersonation and no hosted authority migration.

Production registry callers are `init -> Registry.Register`, root `OpenMigrationSource/OpenMigrationDestination -> Lookup -> ProviderRegistration.Open -> constructors/Capabilities -> ValidateProviderCoverage`, and `MigrationProviderKinds -> Kinds`. The root Open/Kinds functions are intentional integration seams for the explicitly dependent WP11 command layer (plan IC-11), not a claimed completed public migrate command. All new API transport functions have real adapter production callers. Every internal helper is used by production paths. Public migration command reachability and GitLab-to-GitHub public CLI acceptance remain WP11/12 acceptance work; no dummy caller is requested.

## Behavioral assessment

- Source keys separate provider, normalized instance, immutable repository ID, object kind and immutable object ID from mutable issue numbers/selectors. SourceID includes the instance. Snapshot rechecks repository incarnation after observation. PR conversations remain preserved replies; review, reply-to and parent relationships remain explicit. Missing targets produce findings; original preserved records retain their raw source blobs.
- REST pagination checks bounded page counts, endpoint/query continuity, duplicate identities and page size. The operation-wide read client shares a capped wire budget across REST, GraphQL, workflows and attachments. Positive plan record/blob/aggregate/manifest budgets remain additional limits. Overflow refuses incomplete success. Selected same-origin attachment bytes are digest-addressed; redirects are not followed and foreign origins never receive the token. Inaccessible blobs are never called preserved.
- Workflow source is pinned and retained as inert bytes. Preserved actors/reviews/CI/planning data never enter native signing/permission/execution authority. Existing selected Git transport composes with the adapter instead of a second provider-specific code transfer.
- Plan produces closed issue/comment intents. Explicit source state/type mappings govern non-equivalent issue/work-item conversion. Source type names are retained; absent/null type remains empty. Work-item comments depend on the mapped issue intent. Oversized issue source bytes fall back to inert blob preservation with an unsupported finding.
- The final marker-capacity fix preflights issue and comment body plus exact deterministic marker length in Plan before any mutation; the exact positive boundary also passes. No truncation occurs.
- Existing typed targets are rejected from planning with an unsupported finding. Unexpected provider-default issue types fail full projection verification after create, cannot be reconciled as untyped success, and never trigger another create. This is an honest uncertainty/refusal boundary rather than type administration.
- Intent IDs bind normalized destination locator and immutable repository incarnation; current repository identity is re-observed before apply/reconcile. Updates check immutable issue identity and revision before their single mutation. Comments re-observe the immutable parent identified by the confirmed dependency receipt.
- Parent IDs are resolved and persisted by the shared engine in canonical bound payloads/digests. Fresh adapter/session instances resume from confirmed receipts, without an adapter-private parent map. Each apply makes at most one mutation. Recovery requires a unique exact marker plus the full intended title/body/state/type/labels/assignees projection (or exact comment body). Negative, inaccessible, changed and ambiguous observations never authorize another create.
- Every GitHub-only root/client/test build constraint includes both exclusions; disabled help builds under either. `nogithub` retains GitHub-only meaning and `noforge` excludes actual GitHub dependencies/symbols/files. Native/local/archive/audit behavior stays available with credentials and provider tools absent. No new Go module, provider CLI, service or Python product requirement was introduced.

## Required anti-pattern checklist

| Item | Result | Evidence / interpretation |
| --- | --- | --- |
| 1. Dead code | PASS | Targeted production call-site search covered new registry, API and adapter functions. Root command seams are the approved dependency-first WP11 surface described above; no unused internal helpers. |
| 2. Synthetic-fixture tests | PASS | Provider tests invoke production Open/Snapshot/Plan and PrepareMigration/ApplyMigration/OpenMigrationSession against local HTTP fixtures and real temporary Git repositories. Literal source records are inputs; production engine/adapters produce the asserted outputs/effects. |
| 3. Silent empty returns | PASS | Builder empty digest returns latch an error checked before snapshot success; absent IssueType means an explicitly untyped source; unavailable reads append missing-dependency findings. Other nil returns are success/error conventions. Marshal inputs are closed marshalable DTOs. No swallowed provider failure silently becomes preserved data. |
| 4. FR coverage | PASS | Behavior-to-assertion map below; integrated cross-provider/public CLI scope remains explicitly sequenced to dependent WPs. |
| 5. Frozen surface | PASS | Only 28 owned files changed. No WP commit touches `internal/migration/snapshot.go`, `plan.go`, `progress.go`, `archive.go`, go.mod or go.sum. Source/Destination/Locator/ApplyContext and the frozen schemas are unchanged. |
| 6. Locked decisions | PASS | No implicit execution/authority, credential discovery, source deletion, background sync, mandatory provider dependency, blind uncertain-write retry, schema expansion or duplicate transport authority. |
| 7. Shared-file ownership | PASS | Compared all 28 changed paths with primary lanes.json: every path resolves only to lane-e. This report coordinates the explicit WP11 use of the root integration seams; no other lane-owned code is modified. |
| 8. Production fragility | PASS | Single new panic is static duplicate/invalid built-in registration at init, a fail-loud programmer invariant, not a transient provider race. Runtime errors retain uncertain progress and require exact recovery. |

## Responsibility-boundary coverage

- FR-004 / FR-014: `TestGitHubMigrationSnapshotPRTopologyAttachmentsAndBudgets`, `TestGitHubMigrationWorkflowSourceIsPinnedAndInert`, `TestGitHubMigrationDiscussionAndPlanningSnapshots`, and oversized-source preservation assert actual retrieved records, relationship counts, exact verified blob content and zero workflow writes.
- FR-007: `TestGitHubMigrationLossesClosedStateMappingsAndDroppedFields`, `TestGitHubMigrationPlanReportsRecoveryMarkerCapacityBeforeWrites`, explicit work-item mapping and destination-type tests assert findings, refused intents and zero planning writes, plus refusal of a successful HTTP response with a different projection.
- FR-008: inert constructor/registry tests and `TestProviderCoverageRejectsVacuousAndDishonestProfiles` cover shared contract consumption and useful minimum coverage; unchanged old transfer implementations remain exercised in the completed full race evidence.
- FR-009 (GitHub portion): actual snapshot/issue/comment fixture HTTP paths are reached, including shared selected-Git composition; the genuine GitLab reader/cross-provider public CLI scenario belongs to WP07/WP11/WP12 and is not claimed completed by WP06.
- FR-013: lost issue/comment response tests resume with fresh sessions and destinations, inspect persisted bound parent payload/remote identity, and assert one remote issue/comment write; ambiguous/replaced-parent/stale-repository cases assert refusal without duplicate effects.
- FR-015: archived ordinary/nogithub/noforge native CLI creation/list + gh facade run with Git-only PATH, isolated HOME, empty credentials; client dependency/symbol/compiled-file proofs establish actual exclusion.
- NFR-001 / NFR-002: snapshot record/blob limits, page-cycle refusal, aggregate exact-EOF and one-byte excess checks, shared GraphQL budget, immutable namespaces and redirect/foreign-origin credential-refusal assertions exercise the responsible production boundaries.
- NFR-003: exact-head completed full race/vet/build plus tagged focused/native CLI evidence below, supplemented by independent uncached focused race execution.

## Contract walk and verification evidence

`contracts/migration-v1.md` is in scope: all shown shared value/interface/schema examples remain unchanged at the dependency base, including closed issue/work-item/comment records/writes, receipt references and reconciliation statuses. Existing frozen validators own the enumerated findings and receipt-field vocabulary; WP06 introduces no competing closed enum. Registry/adapters consume those validators. The attachment/read/preservation, one-write, exact-positive recovery, immutable destination and no-authority examples were traced through the implementation and assertions above. `contracts/audits-v1.md` and `release-v1.md` are orthogonal to WP06 changes; no contract content was edited.

Independently recomputed all 28 `validation-dac3c4ab/source-sha256.txt` entries against the clean fixed-head lane, all 8 outer SHA256SUMS entries, and every one of the 54 raw archive members against archive-members.json. Inspected run.sh, status.txt and raw logs. The completed source-bound validation ran 26 sequential checks with exit 0, including ordinary full `go test -race ./...` (hubnot 552.758s), vet/build, nogithub/noforge focused migration/native/audit compatibility, all three binaries executing offline native commands, module/dependency/file/symbol/build-info proofs, formatting, diff and tag checks. Reused this exact-head evidence rather than duplicating the full suite.

Additional independent command at unchanged source:

```text
go test -count=1 -race ./internal/githubapi ./internal/migration . -run 'Test(Migration(Pull|Attachments|SharedWire|WireBudget|ReadBudget|CanonicalOrigins|IssueType|ProviderUnknown)|Provider|Registry|GitHubMigration)'
ok  hubnot/internal/githubapi 1.037s
ok  hubnot/internal/migration 1.017s
ok  hubnot 3.462s
```

Independent `git diff --check 247a4e1b26295459ae159674c109bcd91aa1a954 dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116` passed. Frozen-file history queries were empty. No private keys/generated binaries are in the reviewed diff. Lane untracked `.spec-kitty/` and primary unrelated dossier dirt were left untouched.

Framework pre-review outcome is `no_coverage`: `tests.architectural._gate_coverage` is not importable (`No module named tests.architectural`). It completed without force/skip and is an unverified framework authority, not a Go pass or failure. This independent approval is based on the actual source review and source-bound checks, with that infrastructure limitation explicitly retained. No live provider account writes, pushes, main/tag/release changes, or other WP lifecycle changes were performed.
