# WP05 independent final integration review

Verdict: **APPROVE**. No unresolved actionable finding at exact integrated source `4ee9ed8badcc29f4cd00aacd88534b9cf9cc2f23` in lane-e.

Reviewer: `codex-independent-interop-review`, resolved profile `reviewer-renata`, assigned tool agent `codex`. This reviewer implemented none of WP05 or its source corrections. The fresh canonical prompt was read in full: `/tmp/spec-kitty-review-prompts/nichthub-2bfda21ece60/optional-github-interop-01M5RK1Z/WP05/815ab56742c74fbbb3730050c46827d5.md`; invocation reference `815ab56742c74fbbb3730050c46827d5`. Its nominal implementer metadata does not change the resolved reviewer identity. No optional non-ULID invocation flag is used.

## Scope and method

Read primary amended WP05, spec, plan, root AGENTS, review charter/profile/skills, all 13 changed files and their source diff. Reused the independently approved dependency reviews for unchanged code. WP11/WP13 adapter reviews were performed by the other independent reviewer and their committed reports were read; this review does not claim to have authored those audits. No mission contracts directory exists. Checked lane HEAD again during canonical review: exact source unchanged, tracked implementation clean; runtime-generated untracked `.spec-kitty/` is left untouched.

Five explicit integration exceptions override the generic documentation ownership list: `main.go` sorted native-command help; `github_export.go` and `github_export_test.go` whole issue-journal authenticity/future-budget corrections; `github_projects.go` and `github_projects_test.go` populated historical-schema compatibility preflight. These were authorized in primary WP05 before final review, independently inspected, and are the only source ownership overlaps. No native Event schema, authority, signed historical bytes, SDK dependencies or unrelated command parser changed. Seven documentation files and the new public acceptance test complete the 13-file diff.

## Integration findings retained: RED, correction, GREEN

| Finding | Original concrete failure | Final correction and verification |
|---|---|---|
| Issue progress/binding forgery | Root outside `TestIndependentIssueJournalCannotSuppressNewComment` completed an export, added a signed native comment, then manufactured its private `Comments` binding to an unrelated human remote comment. Old source reported success without publication. Root recorded RED 0.179s; the earlier canonical amendment retains the initial 0.172s reproduction. | Domain-separated Ed25519 signature authenticates the entire canonical checkpoint, signer/public-key fingerprint and active identity for real export; loading refuses unsigned, altered, aliased or noncanonical state. Preview verifies the public proof without loading private keys. Original outside probe and durable same-named regression are GREEN. Additional independent probes changed native baseline, remote ID/key, IssueDone, PreserveRemote and project scope individually: every change refused before writes. |
| Whole future issue checkpoint overflow | Original outside `TestIndependentIssueFutureJournalOverflowZeroWrites` used a currently fitting receipt ledger and a legal new comment; old source made one remote comment write before the confirmed journal exceeded 8MiB. Root RED 0.480s. | Preflight reserves snapshot, all accumulated future comment bindings, worst escaped 1024-byte opaque IDs/256-byte keys, the largest pending plus confirmed result, baselines and signature envelope before initial publication, preview and resume. Receipts retain only immutable identity and exact managed fields; unbounded descriptive extras cannot inflate confirmation. Durable `TestForgeExportWholeFutureJournalPreflight`, `TestForgeExportWholeJournalGuardRunsBeforePublication` and `TestForgeExportReceiptsDropUnneededProviderExtras` are GREEN. The latter flow signs the near-limit ledger so it tests the budget rather than failing the new authentication guard. |
| Historical project item/current export schema mismatch | Original outside `TestIndependentProjectHistoricalSchemaPreflight`: item pins a number schema; a later project revision changes the field to text. Approved pre-integration project source performed 13 remote mutations and returned success. Root RED 0.331s. | Planner reads the authenticated exact pinned schema from snapshot versions and checks every populated value against both pinned and current exported fields/types/options before any write. Removed populated fields and missing selected options also refuse; compatible title/body changes remain exportable. No native revision is repinned or migrated. Original outside probe independently GREEN under race in 1.180s; durable four-case regression is GREEN. |

The issue signature authenticates local intent/progress, not remote success or provider permission. Existing exact snapshot, source witness, transmitted baseline, no-echo and positive-only recovery validation remains in place. Old unsigned private journals require explicit recovery; real issue export now needs the original active native identity, and docs disclose this deliberate boundary. No blind negative-scan retry, new authority or silent loss was introduced.

## Requirements trace

All test names below refer to actual production paths at the reviewed source. Existing per-WP independent reports supply the detailed unchanged-source audits; the final full suites execute these tests together. `PublicAcceptance` abbreviates `TestInteroperabilityPublicAcceptance` in `github_acceptance_test.go`.

| Requirement | Concrete behavior/test evidence and documentation |
|---|---|
| FR-001 local compatibility | PublicAcceptance default/nogithub create/edit/comment/close/reopen/native-show; `TestGHCompatibilityLifecycle`; interoperability guide local CLI table. |
| FR-002 scripting | `TestGHCompatibilityRejectsUnsupportedAndMalformedInput`, `TestGHCompatibilityConflictRefusesMutation`, public JSON selected fields/list and unsupported PR/repo flags; interoperability JSON table. |
| FR-003 explicit issue import | `TestGitHubImportPreflightDryRunAndPagination`, `TestListIssuesFollowsSameEndpointPaginationAndSkipsPRs`; public imported-issue fixture; interoperability transfer examples. |
| FR-004 provenance | `TestGitHubImportRepeatUpdateAndProvenance`, discussion graph claims and exact inert workflow import tests; native signer/source distinction in every guide. |
| FR-005 guarded repeat/update | `TestGitHubImportCompetingEditsAndNativeOnlyFields`, `TestForgeImportRejectsManufacturedBaseline`, exact pending-selection recovery and typed three-way tests; provenance/conflict guide. |
| FR-006 issue export | `TestForgeExportLifecycleRepeatAndNativeExtras`, same-source no-body/comment-echo and ignored-field refusals; issue export representation guide. |
| FR-007 publication recovery | Lost create/comment, negative-scan never-repeat, remote-only baseline and all new signed-journal/budget tests in `github_export_test.go`; recovery guide. |
| FR-008 explicit access | PublicAcceptance absent gh/credentials and closed local server before tagged edits; `TestRedirectsErrorsAndFailuresDoNotLeakOrRetry`, GraphQL auth/error tests; credential/transport guide. |
| FR-009 independent core | PublicAcceptance default plus nogithub binary, all native families, keyless reader and imported offline edit; complete tagged suite/dependency exclusion; optional-build guide. |
| FR-010 honest documentation | Public help discoverability assertions and exact supported flag/JSON examples; README, interoperability, issue and threat guides. No full gh/API equivalence claim. |
| FR-011 provider neutrality | `TestForgeNeutralOpaqueKeysAndCapabilityRefusal`, `TestResourceBridgeRemoteJournalNonHTTPIdentity`, `TestResourceBridgeKeepsIssueFactoryIndependent`; extension contract documented. |
| FR-012 capability/mapping honesty | `TestResourceBridgeCrossResourceMappingAndReports`, project per-finding acknowledgments, discussion history/category and workflow mapping preflight tests; resource matrices/manual limits. Public capabilities independently returned all four installed adapters. |
| FR-013 native discussions | PublicAcceptance independent nested replies and accepted original answer after reply edit; `TestResourceRepliesPinAnswerAndThread` and discussion command suite; discussions guide. |
| FR-014 native projects | PublicAcceptance typed item exact historical schema and native issue target after project revision; `TestResourceCatalogConflictAndSchemaPin`, project command suite; projects guide. |
| FR-015 native workflows | PublicAcceptance canonical materialization, inert foreign refusal and ordinary governed request without execution; `TestWorkflowRequestBindsCanonicalProposalBytesAndPreservesCI`, symlink/hardlink and disabled/conflict tests; workflows guide. |
| FR-016 integrity/portability | PublicAcceptance real bare remote/reader preserves actor commit objects and all six resource aggregates without adapter journal/private identity; `TestResourceBridgeAllFamiliesSurviveNativeGitTransport`, strict authority/replay/closure tests; resources and threat docs. |
| FR-017 discussion transfer | `TestGitHubDiscussionImportAndSameSourceReturn`, native export recovery/conflicts, deleted source reply preservation, history/category/historical-answer preflight and signed-journal regressions; discussions guide. Detailed unchanged adapter audit: WP11 independent-review-1. |
| FR-018 project transfer | Public project import/export/repeat, lost mutation, signed journal/source identity, opaque content, numeric/date/option and constructor-state tests; new historical-schema regression; projects guide. Detailed unchanged audit: WP12 independent-review-1. |
| FR-019 workflow transfer | Exact/inert import, explicit generated roundtrip, guarded PUT recovery, destination/CAS and renamed-source regressions, literal argv wrapper test; workflows guide. Detailed unchanged audit: WP13 independent-review-1. |
| FR-020 whole-resource planning | Bridge graph/reference/mapping/dry-run and missing-target tests, each adapter unsupported-zero-write tests, whole future checkpoint tests for all four exporters; interoperability/resource mapping matrices. Issue output is honestly documented as human summaries rather than typed-plan JSON. |
| NFR-001 bounded input | REST/GraphQL response/aggregate/pages/deadline/Unicode/ambiguous JSON tests; actual signer/pending/whole-checkpoint preflight and strict native states; finite bounds documented. Complete selected records/graphs preflight; interrupted multi-event imports are explicitly resumable, not transactions. |
| NFR-002 quality | Exact-source full race, tagged full suite, both vets/builds, formatting/diff/JSON-document checks below. |
| NFR-003 offline acceptance | PublicAcceptance child-only git PATH without gh, unset forge credentials, real repositories and local fake HTTP import; server closed before tagged edit. No live provider access. |
| C-001 standalone | Standard-library-only module; full nogithub/native imported-data operation and excluded SDK dependency. No mandatory service/database/Docker/gh. |
| C-002 existing authority | Core approved byte-preservation/strict authority tests, Git object identity assertions, exact policy/proposal/pipeline request path; source claims/signatures confer no policy roles. |
| C-003 bounded scope | Unsupported compatibility tests and capability/preflight refusals; docs explicitly exclude background sync, arbitrary API/Actions equivalence, moderation/permission migration, automerge and protected-check rules. |

Dependency evidence is committed under sibling task directories: WP01/WP02/WP03/WP04/WP06/WP07/WP08/WP09/WP10/WP11/WP12/WP13 `independent-review-1.md`, and WP14 `independent-review-2.md` with its preceding rejected cycle. Their original defects, corrections and source-bound gates remain in those reports; this final report does not erase or recast those REDs as initially passing behavior.

## Exact verification

Independent pre-audit: outside `work/interop-review/final-issue-overlay.json`, focused race over public acceptance, all `TestForgeExport` cases, six-field signature probes/original comment suppression and project schema regression: **PASS 19.656s**, log `/home/lynn/Documents/Codex/2026-10-24/wa/work/interop-review/wp05-independent-focused.log`. Original outside project schema overlay separately **PASS race 1.180s**. Public capabilities command returned all four resources together. No new source concern justified duplicating full suites at canonical review.

Root exact-source manifest `/home/lynn/Documents/Codex/2026-10-24/wa/work/wp05-4ee9ed8b-gates.json` was read along with both referenced complete result logs. Both entries pin reviewed lane/source and exit_code 0:

- `go test -race -count=1 ./...`: main **497.342s**, forge 1.011s, githubapi **10.855s**; wrapper 497.818s.
- `go test -tags nogithub -count=1 ./...`: main **158.833s**, forge 0.005s; wrapper 162.347s; no githubapi package.
- Both vets, default/tagged builds, SDK dependency exclusion, formatting/diff and JSON-document example checks passed in author/root exact-source evidence. Reviewer rechecked source hash, tracked cleanliness and diff whitespace.
- Default for_review gate completed without skip/force; unavailable Python/JUnit coverage authority is the known no_coverage baseline, not a Go regression. Actual required Go gates above passed.

## Required anti-pattern dispositions

1. **Dead code — PASS.** New production signature, receipt, budget and schema helpers are called from live issue/project export; main help reads the registered native commands.
2. **Synthetic fixtures — PASS.** Public binary tests use real temporary Git repositories, bare remote, signed storage and local HTTP. Budget seam construction isolates size arithmetic while signed near-limit flow verifies the real pre-write dispatch guard; it is not offered as a substitute for lifecycle coverage.
3. **Silent empty returns — PASS.** No failure is hidden as empty state. Already-matching files/results and no-op repeats are explicit; malformed journals, unavailable dependencies, unsafe mappings/bounds/conflicts fail loudly.
4. **FR coverage — PASS.** Concrete integrated and unchanged dependency assertions are traced above for every referenced FR/NFR/constraint.
5. **Frozen surface — PASS.** No frozen facts or signed payloads changed. New private signature fields are local recovery data, with old unsigned state explicitly refused.
6. **Locked decisions — PASS.** Standalone optional transport, existing authority, immutable conflicts and exact guarded runner semantics preserved; no silent foreign execution or loss.
7. **Shared ownership — PASS.** The five named source/test exceptions are explicit primary-WP amendments, independently reviewed and coordinated after dependency approval; no concurrent hidden overlap.
8. **Production fragility — PASS.** Predictable budget/schema failures precede writes, journal tampering cannot suppress publication, uncertain outcomes retain positive-only reconciliation and manual boundaries. No panic or authority fallback introduced.

Approval covers WP05 final integrated implementation/acceptance/documentation at the stated source. GitHub uses supported finite mappings: arbitrary Actions YAML stays inert; historical answer replacement, arbitrary project schema migration/automation/views/permissions and general provider equivalence remain manual or explicitly unsupported. APIs still have read/write races and one local publisher cannot provide distributed exactly-once creation. Mission acceptance/consolidation and any reviewable PR delivery remain separate workflow steps; this verdict does not authorize public-main merge or live provider publication.
