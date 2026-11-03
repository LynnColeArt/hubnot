# WP25 independent formal review

Verdict: **APPROVE**. No blocking implementation or evidence finding was identified in the reviewed WP25 scope. This is an independent work-package verdict; it does not certify release publication, whole-beta acceptance, browser-download installation, WP26 adopter trials, or mission merge.

Reviewer: `wp25-independent-review`, Codex, resolved `reviewer-renata` through the installed CLI; review charter context and `spk-run-review-wp`, `spk-run-verdict-capture`, `spk-doctrine-profile-load` applied. I did not implement this work and made no source edits.

Reviewed HEAD: `da6bd2b95a71cafb4cdbc27613a266cafae64183`, tree `d7322d5ba63ad850cfa10e98582ccbdef579c17d`.

Review base: approved WP24 `4bb26359dbdb2fbc149911dc99c3a5880a24c031`. The review includes the full historical production/test qualification changes, CI integration overlay and final evidence records across this range, rather than only the generated continuation diff. The range has 96 changed paths, including 31 Go paths. The formal generated prompt was `/tmp/spec-kitty-review-prompts/nichthub-2bfda21ece60/optional-extensions-01M661C0/WP25/d6ca39f48c3c42d38d97f74ff858c454.md`.

## Authority and prerequisites

I read WP25 objectives/T100–T103, the mission requirements/plan/model/research/coverage, the relevant frozen artifact/host/default/conformance contracts, the maintenance amendments, `ci-integration-decision.md`, and the primary `wp25-review-continuation.md`. The 2026-11-03 maintainer CI decision controls change-appropriate scheduling: deep integration/stress/soak remain explicit native work, while hosted smoke is deliberately bounded. It does not authorize altered production limits or relabel historical runs.

The canonical coordination event directory is `/home/lynn/projects/nichthub/.worktrees/optional-extensions-01M661C0-coord/kitty-specs/optional-extensions-01M661C0`. Installed `specify_cli.status.event_sourced_review_result` confirms WP01 approved (cycle2/user) and WP24 approved (cycle1/codex). Their review-cycle records were inspected. The primary checkout contains historical planning snapshots and is not the event authority. WP25 was `in_review`, with T100–T103 all `done`, force_count0 and no preexisting WP25 verdict. Dependency state was not edited.

## Source review

The production diff was examined against the approved base. Its bounded optimizations retain the public contracts:

- `artifact_read.go:485` counts unique authenticated raw ancestry only after object/aggregate metadata admission and hash verification, then still performs full object/tree/signature/schema/continuity verification. The explicit-source route retains initial/final selected witnesses; discovery retains full-set final comparison. `artifact_read.go:577` owns pipe/session cleanup and Wait, with no ambient Git replacement/lazy-fetch override. `artifact_read.go:786` uses two fixed GLOBAL64 witness slots; the coordinator preserves ordered parsing/fallback/error selection and cancels/joins every launched worker.
- `artifact_exchange.go:1011` and `:1098` validate the projected accepted union before destination copy/promotion. Borrowing is restricted to the already admitted physical common object store; unsuitable newline/promisor/pack-link layouts use the exact verified inventory. This does not add incoming refs as authority or copy receiver private state. The raw blob worker, closure reader and replication callers retain exact hashes, finite byte budgets and owned lifecycle.
- `contribution_exchange.go:1919` retains enumeration/count/metadata admission before body reads, distinct sequential classic Git workers, exact repeated framing/hash checks and native policy budgets. Native stderr remains an owned discarded stream rather than acquiring the artifact transport's 8192-byte policy. `contribution_records.go:334` freshly reads/parses the exact current commit/digest policy; it does not reuse cached authority or alter the noncurrent path.
- `store.go:634` replaces copied events with sorted indexes while preserving the input slice and actor sequence/previous/conflict rules. `internal/extension/session.go:498` gives the unchanged 250ms revocation bound scheduling headroom; the bound is not weakened. `examples/extensions/fake-tracker/main.go:886` atomically publishes a completely written/closed ready URL.

I inspected the added adversarial test structure and changed fixture/refusal paths. OS config isolation now follows the actual child-selected registry and proves that detector with deliberate registration/read controls. Independent-clone checks exclude the source key while preserving the destination's own identity. Darwin temp normalization is restricted to the explicit qualification harness and verified OS aliases; production safepath policy stays unchanged. Large hostile child inputs use files rather than argv-size assumptions. Recovery/deadline controls record actual reached stages and retained state rather than accepting any exit error.

`extension_package_acceptance_test.go:38` requires the immutable package bundle and separate exact-source reference assets, verifies identities, pins supplied-binary seams, and installs/tests a compiler trap. The missing-reference subprocess must fail for its specific cause. At `:481`/`:491`, hosted smoke selects five original callbacks; all206/heavy4/remaining202 registry membership, inventory digest and callbacks are preserved. `TestProjectCINativePublicJourney` at `:1721` explicitly labels source-assisted execution. No stub, default extension launch, new module/runtime dependency, production doctrine branch, private key or generated executable was added by this diff. `git diff --check` passes.

The workflows preserve the explicit manual selector/default beta route and separate caller/workflow/candidate identity. Exact package smoke verifies native architecture, translation state, checksums, source-before/after, both variants and supplied references. Native CI pipelines are explicit, source-bound and bounded, and do not grant runner trust. Smoke exclusions remain visible; source-assisted native integration is not described as archive certification.

## Independently checked retained evidence

Detailed machine readback: `/tmp/hubnot-wp25-review-controls/evidence-readback.json`.

I independently rehashed 41 distinct manifest-referenced files, all8 current archives and executable bodies, and all12 reference executables plus their12 build-info files. All match their records. I rebound all317 frozen Go/module input hashes to actual Git blobs at `3dcbb96091c6191bc5d98cde9dbd1d2b98433f95`; recorded before/after inputs match. All seven frozen source commands exit0: full uncached race, standard/native vet and builds, formatting, diff check. Raw race JSON contains3968 named passes,0 named failures,18 package passes, and six explicitly documented helper/capture skips. The separate default run has149 named passes,0 failures/skips.

I recomputed the actual4320 CSV observations:72 groups with30 baseline and30 candidate observations each, passing all144 elapsed/RSS criteria under unchanged max(10%,5ms) and max(10%,1MiB) budgets. This does not claim new measurements at the review HEAD or retraverse all26630 raw capture files. The retained independent default audit and binding supplement were hash checked.

Only `extension_package_acceptance_test.go` differs among Go/module inputs between frozen3dc and reviewed HEAD. All150 production/module inputs are unchanged. No Go/module/action/pipeline/workflow/fast-selection input differs from tested `d5aee5d0226c2e86615c1a0910d3cc8b9592cce7` to the reviewed HEAD. The20 old/current executable identities match exactly (8 application,12 reference), as independently checked against the retained old log and current assets. This supports scoped reuse of unchanged-code evidence, not interchangeable archives, provenance, commits, trees or harnesses.

I independently parsed direct historical LinuxAMD64, LinuxARM64 and MacARM64 logs: each contains412 root PASS events, all206 canonical names exactly twice, covering six exhaustive variant rows. Their logged probe failures are expected missing-reference negative controls, not qualification-root failures. Original Intel completion remains cancelled/partial; no exhaustive Intel pass is inferred.

For current hosted run41591987505, I checked all8 raw smoke-group records and result logs against their hashes: exactd5 source identity, five expected root passes per variant,201 exclusions, no source fallback, and the expected missing-reference failure. Current archive/executable/reference hashes match. Install/runtime evidence is separate. Native signed-source records and public run-show readback bind the fast/integration results to exactd5, pipeline/request/result identities, sandbox LinuxAMD64 and101161/116154ms passes. These are isolated verification-clone results, with no new project trust role.

The69-entry limit classification inventory is byte-semantically unchanged from frozen3dc. The final records preserve dominated/private/source-only/enclosing distinctions, source-assisted fixtures and eleven historical remaining-gap entries with explicit current dispositions. Neither exact-deadline equality, an unavailable recovery-directory loader, a literal116384/116385 pair, every tail-race fault injection, nor a persisted final tracker status at an overflowing deferred update is invented as a runtime success.

## Bounded independent execution

I extracted only the current LinuxAMD64 standard executable into `/tmp/hubnot-wp25-review-controls/hn` and verified its package digest. I ran three uncached race invocations against the reviewed test source with that supplied CLI, retaining JSON/stderr:

1. Source discovery/composition/persisted-change/error-precedence, actor-chain contract/input preservation, and qualification-group controls:43 named passes,2.586s.
2. `TestArtifactExchangeRefusalsCLI/import-union/sha1` and `/sha256`, including same-wire missing-optional positive and present-wrong-kind/no-effects negative:8 named passes,1.552s.
3. Native contribution closure `real-git`, `body-stderr9000`, `metadata-object-over`, and `body-hash` controls on both object formats:11 named passes,2.678s.

All invocations passed with no fail events; the total is62 named test/subtest passes. These are focused Linux review controls, not eight new platform runs. No full suite,4320 capture,100000 corpus, ten-minute soak or hosted job was rerun. No source, frozen Mac worktree, qualification/release ref/asset, stash or unrelated WP was changed. The excluded private incident directory was not traversed.

## EXP-OBS-01 disposition

**Nonblocking evidence limitation; host-generation lifetime criterion satisfied.** I independently read the contract and implementation rather than treating the planning concern as an automatic production failure.

`host-v1.md:503–508,696` and conformance `:1871` define expiring captures and valid pages; they do not define peer receipt as the expiry boundary. The prior root decision (SHA256 `3b8c045d57d2bae1c7cdd9731be18c00618f1b54ee51b2b5c8e20a8296644ec1`) explicitly selects the existing host-generation/completed-checkpoint boundary. `extension_changes.go:585–615` checks full-precision context/time validity before rendering, after rendering, and after optional checkpoint persistence. Those checks and the600s lifetime are unchanged.

I rehashed all16 retained files of `actual-expiry-3dcbb960`. The unchanged actual run passed602.55s; retained evidence shows a fresh capture, delayed complete second page and precise expired replay in the same live session. Its host hash matches the current application executable. This plus source review supports the narrow final-host-validity inference. It is not independent black-box clock proof.

No retained response-read/publication/parent-observation timestamp establishes response1 receipt before expiry. The test does not assert that stronger property, advertised RFC3339 expiry loses subsecond precision, and the final transcript mtime cannot backdate receipt. I do not claim receipt-before-expiry proven or the missing assertion implemented. Historical failed expiry cause remains UNKNOWN. A future contract demanding receipt timing would need a new correlated retained observation; it is not the current approved contract. No new soak is justified by this evidence limitation alone.

## Remaining limitations and handoff

The known quarantined browser-download Mac Gatekeeper failure remains failed/uncertified. The maintainer explicitly accepts beta packages without Developer ID signing/notarization as a disclosed distribution limitation; signing is not a functional WP25/review/WP26 blocker. Future signed assets require new digests/provenance and actual download/launch retest. This review does not turn quarantine removal into successful original installation.

The cancelled historical Intel exhaustive run, initial incomplete hosted Mac attempts, older intermediate failures and their scopes remain visible. Current eight-row smoke does not claim206 journeys on every current platform. Stronger receipt-time proof and the narrower limit-classification gaps above remain unclaimed. These limitations do not contradict the current WP25 verification contract.

WP26's independent boundary review and separate public-guide native-user/extension-author observations remain downstream and pending. No implementation/CI replay is an adopter observation. All required WP25 qualification criteria have sufficient source-bound evidence under the maintainer decision, so I approve this exact reviewed HEAD through the normal canonical seam. The CLI result and event-sourced readback, recorded separately, determine whether formal capture succeeds.
