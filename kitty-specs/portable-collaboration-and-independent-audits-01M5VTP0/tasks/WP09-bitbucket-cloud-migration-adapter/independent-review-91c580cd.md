# Independent WP09 review

Verdict: **APPROVE**. No blocking findings.

Reviewer: `codex-bitbucket-independent-review`, agent `codex`, resolved profile
`reviewer-renata`. This is independent review, not implementation or self-review.
Reviewed source: `91c580cda750a77493e933e086d0b535d4e7c724` against approved
dependency `dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116`. Only these six added
source/test files belong to the reviewed delta:

- `internal/bitbucketapi/client.go`
- `internal/bitbucketapi/client_test.go`
- `internal/bitbucketapi/migration.go`
- `internal/bitbucketapi/migration_test.go`
- `migration_bitbucket.go`
- `migration_bitbucket_test.go`

Read the full runtime review prompt
`/tmp/spec-kitty-review-prompts/nichthub-2bfda21ece60/portable-collaboration-and-independent-audits-01M5VTP0/WP09/b61f68db8aa6408f8fa13405c7f9a067.md`,
AGENTS.md, resolved reviewer profile, review charter context, charter, spec, plan,
research, data model, quickstart, WP09 and lane ownership. No source/test files
were changed by the reviewer.

## Scope and contract assessment

`contracts/migration-v1.md` applies. The implementation uses the existing
Source/Destination/ProviderConfig/Capability interfaces, closed preserved record
schema, provider/instance/repository identity keys, recognized finding statuses
and engine-owned snapshot/plan validation. Repository metadata is preserved as
an attachment record because the frozen record kinds lack `repository`;
capabilities state that selected code uses the shared Git companion.

The frozen issue/comment writes require issue/work-item parents and cannot
encode PR targets or thread positions. Consequently the destination truthfully
advertises no create/update operations, emits unsupported findings, and refuses
Apply/Reconcile without network effects or fabricated receipts. This meets the
specific Bitbucket baseline in the contract and plan; PR write support would
require a later shared contract change. No issue/Jira/Data Center compatibility
is fabricated. `release-v1.md` is orthogonal (no packager changes);
`audits-v1.md` is orthogonal (foreign observations create no audit authority).
Existing schemas and constants are reused, with no new parallel contract.

Registration has production callers through migrationProviders and its
OpenMigrationSource/OpenMigrationDestination seams. Those instantiate the
adapter and verify its required useful source coverage. The adapter calls every
new API operation; client helpers are used by those operations. Snapshot output
feeds the actual composition, PrepareMigration, ApplyMigration and local-reader
path in the offline integration test. Public migrate CLI work is owned by the
dependent WP11; this review does not claim that later integration is complete.

Current official Cloud documentation independently checked during review:
[changelog](https://developer.atlassian.com/cloud/bitbucket/changelog/) confirms
Issue Tracker endpoint removal on 2026-08-20 and API-token Bearer support alongside
Basic; [PR API](https://developer.atlassian.com/cloud/bitbucket/rest/api-group-pullrequests/)
confirms repeated state selectors, global/inline/reply comments and activity
variants. The implementation uses actual REST 2.0 reads and explicit transient
anonymous/OAuth/API-token modes; constructors/capabilities do no credential or
network discovery.

## Requirement behavior evidence

| Requirement | Observed implementation and executable assertions |
| --- | --- |
| FR-004 | `TestBitbucketRegisteredOfflineSnapshot` calls the registered source, opens returned blobs, verifies their digests and exact pipeline bytes, raw inline coordinates, author/date provenance and actual parent keys. `TestBitbucketSelectedAttachmentsStayBoundedAndOnOrigin` retrieves selected attachment content and distinguishes inaccessible content. `TestBitbucketSharedGitAndOfflineNativePreservation` archives and imports the real provider snapshot and checks canonical equality after the provider shuts down. |
| FR-007 | `TestBitbucketUnsupportedAndMissingDifferFromEmpty` proves removed issues refuse before reads and private/deleted HTTP results produce missing_dependency. `TestBitbucketEmptyPRConnectionAndMissingThreadParent` distinguishes a true empty connection, missing parent, private comments and private workflow; available orphan bytes survive without fabricated links. Destination planning returns explicit unsupported findings and zero intents. |
| FR-008 | Registry opening and required coverage execute production adapter paths. Six-file isolation leaves legacy transfer APIs untouched; fixed-source ordinary checks include existing Forge/Transfer tests, and binary smoke logs exercise transfer/GitHub help and local gh issue listing. `noforge` excludes these six files; `nogithub` retains Bitbucket. |
| FR-011 | Production Repository/PullRequests/Comments/Activity/Pipeline calls preserve repository UUID independently from workspace/slug, all PR states, content-identified activity observations, exact comment bytes/topology and pinned pipeline source. `TestBitbucketStableUUIDAcrossSelectorRename` proves mutable selectors do not change SourceID; replacement tests refuse mixed incarnations. Profile tests reject Data Center, obsolete/ambiguous auth and unpinned workflow selection. |
| FR-013 | There are no representable Bitbucket destination mutations. The registered destination test asserts Apply/Reconcile fail and request counts remain unchanged; no uncertain-write receipt is invented. Useful source bytes flow through the real saved-plan/local application engine in `TestBitbucketSharedGitAndOfflineNativePreservation`. Shared remote-write recovery is inherited unchanged from the dependency, not falsely claimed as a new Bitbucket write direction. |
| FR-014 | Workflow bytes are preserved in an inert blob, not executed; activity approval remains a preserved foreign observation. Offline local round-trip retains the same canonical snapshot, while plan/archive assertions exclude the supplied token. No code converts foreign users/reviews/CI to native signing authority. |
| NFR-001 | `TestBitbucketReplacedRepositoryAndBudgets` refuses record/blob/per-blob/manifest exhaustion. `TestCloudIdentityAndRawSource` asserts workflow and aggregate wire bounds. Transport has positive bounded response/aggregate/page/record budgets, with selected attachment bounds tested; final canonical validation prevents partial snapshot success. |
| NFR-002 | `TestHostilePaginationAndRedirects` refuses cross-origin and different-connection cursors, redirects, loops, duplicate fields/IDs, missing values, oversized responses and page exhaustion, while asserting zero requests to the other origin. Attachment hostile cases also assert no forwarding. Archive/native admission is the unchanged validated dependency; the adapter's integration actually uses that archive/local path. |

The final thread regression is substantive: the fixture has a global root,
inline root, coordinate-carrying reply, tombstone and global reply. Assertions
check each record's kind, attribution and actual parent key, beyond aggregate
counts. Only a root with actual `Inline != nil` becomes inline_comment. Global
roots/tombstones and all replies are reply records, with correct parent kinds.
The retained old-code/red proof fails on the old mapping; final-source tests
exercise the corrected production code.

## Required eight-item checklist

| Item | Result | Assessment |
| --- | --- | --- |
| 1. Dead code | PASS | Production call-site review above covers the registered adapter, API operations, helpers and standard error interface. The intentionally deferred public CLI owner is documented; no dummy caller was added. |
| 2. Synthetic-fixture test | PASS | HTTP fixtures feed real registered production readers; assertions consume returned records, blob streams, source identity, findings and local engine results. Literal fixture bodies simulate provider input rather than fabricate asserted output. |
| 3. Silent empty return | PASS | Empty results represent a successfully retrieved empty connection, parser success, or optional attribution absent from preserved original activity bytes. Builder empty-string returns carry a sticky error that Snapshot propagates before return. Unsupported/missing paths return errors or explicit findings; unknown blobs fail. |
| 4. FR coverage | PASS | Every frontmatter requirement has behavior assertions in the table above. No destination mutation or later CLI implementation is claimed. |
| 5. Frozen surface | PASS | The complete dependency-to-source delta adds only the six owned files; no pre-existing file, frozen schema/interface, signed history, native default or release artifact is changed by either source commit. Committed diff check passes. |
| 6. Locked decision | PASS | Only GET requests occur; explicit Cloud profile and supplied credentials, inert bytes and truthful unavailable capabilities preserve all relevant prohibitions. No Python product dependency, SDK/module, provider CLI, mandatory service, live mutation, source deletion, authority conversion or background work. |
| 7. Shared-file ownership | PASS | lanes.json assigns the six exact files exclusively to lane-h/WP09. No shared production file was edited, so no overlapping-file coordination exception is needed. |
| 8. Production fragility | PASS | Error returns fail loudly on hostile input, budget exhaustion, unavailable selected content or identity replacement; missing/private/deleted resources use explicit findings. The only panic is deterministic duplicate/invalid static registration in init, matching the existing registry convention; no transient request failure panics. |

## Validation and integrity

Canonical author evidence is committed at
`57905acaa465f08bfb8b62dfbaf4b5fd1bedc5f9` under `validation-91c580cd/`.
The reviewer independently checked:

- All six SHA-256 values against live lane bytes and `git show` at exact source
  commit 91c580cd; all matched. HEAD remained exact. The untracked review-lock
  directory is runtime metadata, not a source modification.
- All 11 outer SHA256SUMS entries passed.
- The archive is exactly 144 unique regular safe-path members, 1,381,830 expanded
  bytes, with no links, special files, traversal, unexpected members or oversized
  entries. All 143 inner hashes passed; the additional checksum file is an exact
  copy of the externally verified one. Its member list matches byte for byte.
- raw.tar.gz SHA-256 is
  `c41c5b9e95630a87e9a354ac37350aaf57be56cdaa4ae63db430c523a5623658`.
- The fixed-head unfiltered `go test -race -timeout 20m ./...` has a natural exit
  0 at 2026-10-27T04:23:07Z, root result `ok hubnot 627.389s`, with source HEAD
  and six hashes in the command log. No second full suite was run by the reviewer.
- Fixed-source vet/build, formatting, working/committed diff, ordinary/nogithub/
  noforge focused checks, compiled-file/dependency/symbol proofs and isolated
  env-i/Git-only-PATH native binary smoke records were inspected. All 58 status
  entries are zero; the expected old-code regression failure is separately and
  accurately marked expected exit 1, not hidden as a final-source gate.
- Independent reviewer execution:
  `go test -race -count=1 -timeout 3m -run '^(TestBitbucket|TestCloud|TestOpaquePagination|TestHostilePagination)' . ./internal/bitbucketapi`
  passed exit 0: root `2.304s`, bitbucketapi `1.283s`.

The earlier 5c full-suite result is superseded and not used as fixed-source
validation. Framework `tests.architectural._gate_coverage` is unavailable and the
canonical gate reports no_coverage/unverified; it is not a Go test PASS. Hosted
sync/auth/nonconsent and binary UTF-8 scanner warnings are retained in evidence.

The nested-worktree Go1.26.4 VCS-stamp anomaly is explicitly preserved: source
paths/hashes and symbols are lane-h/91c580cd, while embedded VCS metadata comes
from the enclosing primary checkout. This review does not assert matching
embedded revision or release provenance. No release binary is published by this
WP; later packaging uses its own isolated committed-source boundary. Actual
execution evidence is Linux amd64 only.

Review is complete at the exact source above. Record the approval with this
committed report as approval-ref through the canonical WP09 lifecycle command;
the durable review_result event remains the status authority.
