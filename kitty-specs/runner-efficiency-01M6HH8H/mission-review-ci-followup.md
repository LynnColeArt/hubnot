# Runner-efficiency CI follow-up review

**Correction verdict: APPROVE. Main merge remains conditional on fresh successful Linux and macOS PR checks.**

Reviewer: independent mission-fidelity agent `runner_mission_fidelity`. Date: 2026-11-03 America/Chicago / 2026-11-04 UTC. Mission: `runner-efficiency-01M6HH8H`. PR: [#39](https://github.com/LynnColeArt/hubnot/pull/39).

- Original consolidated-source audit: `f268db842ba6e46b8d31ec875ee58b7b86120bfd`, [mission-review.md](mission-review.md), PASS WITH NOTES for source delivery.
- First PR source: `e4b98f6cd418e52bafb2935161c85ee902b030a4`.
- Reviewed correction: `68a70c4143f146efdcce982fc29fdb52a23830c9`.
- Review boundary: placement/configuration correction only; production behavior, test statements/assertions, fixtures, operation watchdogs, package/step deadlines and hosted workflows remain unchanged.

## Newly observed P2 merge blocker

**P2-CI-1: The full multi-replica runner journey exceeded the aggregate fast-test budget on macOS.**

The first actual PR portability run produced a successful Linux job `126298926751` and a failed macOS job `126298926320`. The retained macOS log shows:

```text
panic: test timed out after 2m0s
    running tests:
        TestRunnerEfficiencyPublicJourney (1m13s)
...
runner_efficiency_acceptance_test.go:173
FAIL hubnot 120.048s
Process completed with exit code 1.
```

The package timer is aggregate: the journey had consumed 73 seconds when the whole selected root process reached 120 seconds. The stack was waiting for an actual public CLI command. No earlier assertion failure appears in the supplied failure log. The evidence identifies a fast-tier capacity/placement failure; it does not prove a product assertion defect, nor justify claiming the job passed.

This was a real **pre-merge blocker** under NFR-002's bounded portability scope and the rule that observed gate failures must be resolved. Local incremental timing from the original source audit did not establish macOS aggregate timing. The original audit explicitly excluded Mac runtime qualification; its historical source-only verdict is retained, with this subsequent finding recorded separately. Neither the initial failed PR job nor the earlier incomplete monolithic suite is relabeled as a pass.

Retained failure evidence: `pr39-macos-failure.log` in the dated external mission evidence directory, SHA256 `7847542ff23983249235c8ee33657b464e6217fd008b5a84e00326a3c72b79e6`.

## Placement decision and exact source correction

Moving the complete requester/two-runner, lineage, withdrawal, merge and archive journey to the existing explicit native integration tier matches that test's actual scope. Small target-option, exact trusted-result and moderated legacy-identity controls remain in fast. The journey still runs through the full existing integration command; it has not been deleted, weakened, disabled or replaced by a narrower fixture.

The exact `e4b98f6c..68a70c41` delta contains four intended paths:

| Path | Reviewed change |
| --- | --- |
| `.hn/actions/ci:75` | Adds only `RunnerEfficiencyPublicJourney` to the existing anchored integration alternative list |
| `ci/fast-tests.txt` | Removes only `TestRunnerEfficiencyPublicJourney` |
| `docs/ci-v0.md:8` | Documents fast21/integration8 placement and unchanged budgets |
| `runner_efficiency_acceptance_test.go:16` | Changes only the tier-description comment from fast-tier to native integration |

Independent byte comparison proved that the Go file differs only by that comment and the action differs only by the added selector alternative. Every other tracked file in the fix commit is identical to baseline. In particular, no production, release asset, pipeline definition, hosted workflow, module dependency, executable test statement, assertion, fixture, native-only child build behavior or watchdog changed.

The fast package budget remains **2 minutes**, integration package budget **8 minutes**, and existing integration pipeline step **600 seconds**. The change uses the existing native integration budget; it does not widen a deadline or add a hosted deep corpus.

## Independent selection verification

Independently recomputed the before/after fast manifest and real compiled integration inventory using `go test -list`, rather than relying solely on the implementer's summary:

```sh
git diff e4b98f6cd418e52bafb2935161c85ee902b030a4..68a70c4143f146efdcce982fc29fdb52a23830c9 -- \
  .hn/actions/ci ci/fast-tests.txt docs/ci-v0.md runner_efficiency_acceptance_test.go

go test -list '^(Test(ProjectCINativePublicJourney|ArtifactFullPublicJourneyCLI|ExtensionNativeSixFamiliesCLI|ExtensionArtifactInterruptedRecoveryRawCLI|ExtensionFakeTrackerStandaloneRecovery|ExtensionCharterStandaloneJourney|InventoryStandalonePublicJourney|RunnerEfficiencyPublicJourney))$' .
```

Inventory command exit: **0**.

| Selection | Before | Corrected |
| --- | --- | --- |
| Fast | 22 roots | 21 roots |
| Integration | 7 roots | 8 roots |
| Combined distinct union | 29 roots | The exact same 29 roots |
| Omitted/added roots across union | — | Zero / zero |
| Root moved | — | Only TestRunnerEfficiencyPublicJourney |
| Cheap fast controls | Present | Preserved |

The regex is anchored by the unchanged action's `^($test_name)$` call, and the compiled inventory contains all eight intended integration roots. The full journey continues to build and run its actual native-only child using `runnerAcceptanceBinary`; it does not silently substitute the standard source-assisted executable exported for other integration roots.

Compact independent proof: `post-pr-ci-fix/independent-placement-proof.json` in the external evidence directory.

Corrected SHA256 bindings:

| File | SHA256 |
| --- | --- |
| `.hn/actions/ci` | `0cdff6784ef8f99ebf76650168718be563dde69f33e86d2ebe307c3cb78f728c` |
| `ci/fast-tests.txt` | `704dec1992d4c4f1a6f6eb1dfd4c2f11d0cc76708a6f5695281d8c6b86147a90` |
| `docs/ci-v0.md` | `190a33110de14f2e1ab9856c4ffd3506994545efe3271558ca36fc295efe8fc5` |
| `runner_efficiency_acceptance_test.go` | `0d99cac154a8286ffe71b2d80f735e2ddf1e1bcac12d0d14c3301ad7b27e436e` |

## Actual complete corrected-source gates

Read back the implementer's complete action command JSON and stdout/stderr evidence; these are full selections, not focused substitutes or timing arithmetic. Both commands ran at exact `68a70c41…`, with matching before/after HEAD and `inputs_unchanged_during_command:true`. Independent readback also matched all **1,732** recorded tracked input SHA256 values to the corrected files before the root's subsequent documentation-only companion addition.

| Command | Actual outcome |
| --- | --- |
| `env HN_COMMIT=68a70c4143f146efdcce982fc29fdb52a23830c9 ./.hn/actions/ci fast` | **PASS**, exit 0; 84.266 s wall, **7.548 s root package**; complete fast21, internal/cmd race and vet |
| `env HN_COMMIT=68a70c4143f146efdcce982fc29fdb52a23830c9 ./.hn/actions/ci integration` | **PASS**, exit 0; **123.906 s wall / 122.343 s package**, complete integration8 |
| gofmt, `sh -n .hn/actions/ci`, `git diff --check e4b98f6c..HEAD`, `go build ./...` | **PASS**, exit 0 at the same corrected source |

The integration log records Go 1.26.4 Linux/amd64, Git 2.43.0 and source-assisted standard executable SHA256 `f3580216e63cba698a3ff658517f5658b6f7398afa8a3afc47300a73aab51a86`. This is source evidence, not a release-binary certificate.

The corrected local root timings leave substantial room under the unchanged 120-second fast and 480-second integration package limits. They do not establish macOS timing; fresh hosted platform results are still required. The direct action results are not relabeled as a signed native pipeline run or as a final22-root run.

Evidence files: `post-pr-ci-fix/final-fast.json`, `final-integration.json`, `final-static.json` and adjacent logs. No redundant full suite was rerun.

## Documentation and historical binding

The root's appended “Pre-merge platform check correction” section in [delivery-decisions.md](delivery-decisions.md) was independently read and checked for accuracy/whitespace. It correctly records the initial Mac failure, exact correction, unchanged 29-root union, actual gate timings, unchanged deadlines/workflows and pending fresh public checks.

The original mission review is unchanged since its sole trailing-newline normalization before e4b98f6c. Its original 20-root native result and final22-root incremental evidence remain historical facts about those original source versions. Current placement is fast21 plus integration8. This correction does not claim that the old 20/22-root result executed the new action bytes.

No runtime done-history, approval event or historical test result was rewritten. No original checkout, old worktree, frozen Mac/release asset or public release was modified by this review.

## Final verdict and remaining merge gates

**APPROVE the correction at 68a70c4143f146efdcce982fc29fdb52a23830c9.** The P2 placement finding has a narrowly scoped, independently checked correction and actual complete fast/integration passes at fixed source. No new blocking source or security finding remains in this correction.

**Do not merge until fresh Linux and macOS required PR jobs succeed at the updated PR source.** The first macOS failure remains a failed historical run. Root owns that remaining platform-check readback and must retain exact head/job/result bindings. If either new job fails, reopen the failure rather than infer success from the local results.

The original source audit's non-blocking limits still apply: incomplete exhaustive suite and large withdrawal-history qualification remain [#38](https://github.com/LynnColeArt/hubnot/issues/38); optional simultaneous-start coordination remains [#37](https://github.com/LynnColeArt/hubnot/issues/37). No release, exhaustive coverage, Mac distribution/signing or new binary qualification is approved here.

## Retrospective follow-through

Retain the original generated retrospective and its rejection/rewind interpretation. Add this platform lesson in the companion: full multi-replica/archive journeys belong in explicit native integration, while small representative authority/result/option controls supply bounded routine portability feedback. Keep failed runs and original source bindings visible; do not manufacture successful runtime history.
