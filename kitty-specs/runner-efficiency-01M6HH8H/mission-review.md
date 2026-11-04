# Mission fidelity review: Native Runner Efficiency

**Verdict: PASS WITH NOTES for source consolidation and reviewed PR/main delivery.**
No blocking product, contract, security or cross-WP finding was identified.
This verdict does not qualify a release, exhaustive test corpus, or new downloadable binary.

- Reviewer: independent mission-fidelity agent `runner_mission_fidelity`.
- Date: 2026-11-03 America/Chicago (review probes completed 2026-11-04 UTC).
- Mission: `runner-efficiency-01M6HH8H`, WP01–WP05.
- Reviewed consolidation: `fix/runner-efficiency`, `f268db842ba6e46b8d31ec875ee58b7b86120bfd`.
- Pre-mission main baseline: `6db1e658e1a586458a290729daf5319fce5b2b6c`.
- Exact independently approved WP05 source: `34ee0aa5bf778863cb10c73cd092f4b6417c01f6`.
- Scope: local consolidation has completed; public PR/main merge and issue closure are subsequent root actions.

## Review scope and source identity

Read the spec, full plan, tasks, `contracts/runner-efficiency-v1.md`, all five WP review records, the WP04 rejection/correction, acceptance/issue matrices, current event/status materializations, charter/configuration amendment, maintained product documentation, and the complete baseline-to-HEAD production diff. Inspected the new tests for real production-path causality and effects, including protocol hostility, Git/archive closure, shallow recovery, discovery, moderated legacy identity and in-flight publication.

The production diff is confined to shared policy evaluation, automatic scheduling, target observation, the explicit withdrawal writer/projection, and existing wire/storage/import boundary integration. Tests and operator/protocol/governance/threat documentation accompany these changes. Existing snapshot/archive machinery reaches the common dependency validator; a new snapshot-specific implementation was neither necessary nor omitted.

The following comparison was independently executed, exit 0, with no product/test/action/documentation difference:

```sh
git diff --exit-code 34ee0aa5bf778863cb10c73cd092f4b6417c01f6 HEAD -- \
  '*.go' go.mod go.sum .hn .github ci README.md CHANGELOG.md docs
git diff --check 6db1e658e1a586458a290729daf5319fce5b2b6c..HEAD
```

Consolidation changed reviewed governance/runtime artifacts, not approved product inputs. The newly authored uncommitted `delivery-decisions.md` companion was also read: its default/native boundaries, source bindings, failed/incomplete suite statement, incremental validation limits, issue dispositions and retrospective interpretation agree with inspected source and evidence. This companion is outside the reviewed HEAD and is documentation only.

`meta.json.baseline_merge_commit` currently points to acceptance-era `9f56b3b6…`; it is not the pre-mission product baseline. This report deliberately anchors the whole product diff to the main baseline above so no implementation disappears from review.

Selected current-source SHA256 values:

| File | SHA256 |
| --- | --- |
| `runner_eligibility.go` | `ae145c9282c8e000a51c21f3e6541c367c30d9dd3c500f564bf0d5ea2986a4dd` |
| `runner_target.go` | `9e1317fddc46f0ca7f582ccfcd4f943d91253873eaf1ec428449df1dd97241ae` |
| `run_withdrawal.go` | `03f3781714b9d7c3db6b3e17a19bcb2c211f196ff33e6bcc7578f6d3de363da9` |
| `run_withdrawal_cli.go` | `53dd5fec8f9157e9ad0b34c9547b3ac22d5188d909e5cd955f5c6067a0cca557` |
| `runner_efficiency_acceptance_test.go` | `cdce2b5d479d904c5d5ad421fbee182b308b08a1cf53ae39b86af5211140e62d` |
| `ci/fast-tests.txt` | `3be623bc144f9918f7c359c783db8ed945071ec3182b61caca28683e3a0e7fff` |

## Gate Results

The legacy review skill's Python/SaaS/Spec Kitty cross-repository examples do not describe this Go Hubnot product. The mission's explicit Go/native contract and reviewed `governance.enforcement` policy govern these results; unsupported framework checks are not reported as passed.

### Gate 1 — Contract and affected behavioral checks: PASS in declared source scopes

Fresh independent consolidated-source command, exit **0**, package **44.558 seconds**, race enabled, all 14 selected roots passed without a skip:

```sh
go test -race -count=1 -timeout=2m -v -run '^(TestPolicyPipelineEvidenceAcrossRequests|TestRunnerEfficiencyLineage|TestRunnerEfficiencyTrustedThreshold|TestRunnerEfficiencyInsufficientEvidence|TestRunnerEfficiencyUngovernedAndManual|TestRunnerEfficiencyStaleBase|TestRunnerEfficiencyMovedBaseWithoutOption|TestRunnerEfficiencyMissingTarget|TestRunnerTargetFaultRefusals|TestRunnerTargetCancellationAndDeadline|TestRunWithdrawalShape|TestRunWithdrawalLocalRefusalBeforeMutation|TestRunWithdrawalAutomaticAndHistoricalFacts|TestRunWithdrawalDoesNotCancelInFlightPublication)$' .
```

This directly exercises policy replacement, all four obsolete lineage states, rejected evidence, manual/ungoverned execution, explicit stale-base/no-option behavior, uncertain/oversized target refusals, the built-in ten-second deadline, strict new-kind shape, refusal before signing, historical evidence and late publication. It adds a fresh check at consolidated product source; it is neither the entire suite nor the native-only variant.

Additional reviewed fixed-source evidence:

| Command/scope | Actual source | Exit and interpretation |
| --- | --- | --- |
| `./.hn/actions/ci fast` | `c65e32c52dae5b4f9d8620c5dc1487cc248db4eb` | 0; 121.145 s total, 44.525 s root; original 20-root manifest plus internal/cmd race and vet |
| `./.hn/actions/ci migration` | same `c65e32c5…` | 0; 25.668 s, existing finite shards |
| `./.hn/actions/ci integration` | same `c65e32c5…` | 0; 87.284 s, existing source-assisted native journeys |
| Standard and explicit `-tags nogithub,noforge` runner/policy, withdrawal, shallow/recovery, compatibility and governance cohorts | same product inputs | 0 in each recorded cohort; native independence is evidenced by actual variants |
| `TestRunWithdrawalCLI` | same product inputs | 0; actual standard and native-only child executables, including moderation/discovery/refusal/retry |
| Independent metadata-free final-source three-root race selection | exact `34ee0aa5…` archive | 0; 65.853 s wall / 46.238 s package; PublicJourney, ModeratedLegacyRefusal, PolicyPipelineEvidenceAcrossRequests |

The last command is reproducible as:

```sh
go test -race -count=1 -timeout=2m -v -run '^(TestRunnerEfficiencyPublicJourney|TestRunnerEfficiencyModeratedLegacyRefusal|TestPolicyPipelineEvidenceAcrossRequests)$' .
```

Its exact committed archive SHA256 is `52d7407741e95cab8edd454d12d3abcb71e23737803201f80d95b9bc96073c4b`. Independent WP05 mutation evidence disabled the explicit withdrawal refusal in an external copy: PublicJourney then failed at the public execution assertion because a host result was published. This supports causality rather than an assertion-only fixture.

### Gate 2 — Architecture/default/native boundary: PASS in applicable checks

Reviewed `go vet ./...` at `34ee0aa5…`: exit 0 (1.603 s). Reviewed `go build ./...` and `go build -tags nogithub,noforge ./...` at `c65e32c5…`: exits 0 (3.460 s / 3.270 s). Product/module inputs remain identical. Current baseline whitespace check and approved-input comparison both exit 0. No dedicated Python architectural test package applies.

No service/database/provider/API dependency or implicit worker was introduced. Default sandbox/backend opt-ins and outer moderation admission remain intact. Exact static help/capability forms bypass repository/authority/lock work; malformed forms do not obtain that bypass. Existing action/pipeline and hosted workflow bytes are unchanged, apart from the explicitly extended bounded fast selection file.

### Gate 3 — Distributed/native integration and portability: PASS in declared scopes; deep release qualification OPEN

Real Git replicas, selected synchronization, archive transfer, hostile import refusal and shallow recovery are covered through production readers/validators and public CLI journeys. These provide the relevant distributed integration; no separate Spec Kitty SaaS deployment is part of Hubnot.

An actual native `.hn/pipelines/test.json` run executed source `8f0db766913381cacf20b9af624d80d0e0f1b7a6`, using fixture commit `cf8671a848a91a8b076a48ec37728b0a6708770e` whose only delta was explicit local policy configuration. The existing action/pipeline bytes were unchanged. Its signed passing result is `sha256:8dd7cf775bb606e21fe1d5c9b52325e6632bfcd626232435a68ed37c117a1e0b`, request `sha256:7024183e5c1bb07d3295cda70f9be0bc80ed4eb6b132ae00dfde61eb56a6d6da`, definition `sha256:4d89a7127e0644496f8e640b1f41f4f3c3ba1a1ca0d199a0b0e662592faa622d`; pipeline 142.507 s within its unchanged 300 s deadline. The signed log digest is `sha256:9c296d596cb720ae2db915f420339e2201ead48bcaccfa0506452b79d8b3d67f`.

That run selected **20 roots**, not final 22. The final change adds two manifest roots and a shared test builder; final three affected roots were rerun from the exact archive, while 19 prior selected inputs and production/helper inputs were proven unchanged. The final 22-root coverage is incremental. Neither the 20-root result nor the measured incremental timing arithmetic establishes a single final-22-root native run.

The monolithic `go test -race -timeout 20m ./...` at `f60d4046…` remains **failed/incomplete** after the aggregate root-package timeout. Cause/regression attribution is unknown; no complete full-suite pass exists. Reviewed charter policy explicitly separates bounded source acceptance from exhaustive native release prerequisites. Capacity/stress, transport-deadline, real-clock soak and withdrawal-heavy history qualification remain [#38](https://github.com/LynnColeArt/hubnot/issues/38). No exception is being represented as a full test pass.

### Gate 4 — Issue matrix and canonical completion: PASS

`issue-matrix.json` has two rows, #27 and #30; both are terminal `fixed`, with evidence refs describing exact delivered scope. There are zero empty/unknown/in-mission verdicts and zero deferred rows missing a handle. Public closure remains conditional on the reviewed main merge. The issues are related, not duplicates.

Current `status.json` records all five WPs done with independent approved review results. Read the event log and WP04 two-cycle history: event `01M6HV2HM729EV1TQXBEH75WRB` has `force:true` for the supported **backward rewind** from in_review to planned, explicitly carrying changes_requested and its rejection reference. It is not an approval override. WP04 was corrected at `860f675cabbd44ca6ed4c935ad859e568d0749d4` and independently approved in cycle 2. No self-approval or unresolved rejection was found.

Strict acceptance evidence records the exact direct action commands plus vet/build, exit success, no failed/skipped/blocked checks or path violations. The root's `project.path_conventions` maps colocated Go workspace/tests to `.`; it accurately describes the repository and does not replace product checks with empty directories.

## FR Coverage Matrix

ADEQUATE below means tests constrain the actual required behavior, within the source scopes stated above.

| FR | WP owner | Implementation chain | Tests and adequacy |
| --- | --- | --- | --- |
| FR-001 Preserve defaults/authority/keyless reads | WP01/02/04/05 | Existing backend/authority gates; `main.go:25`, `contribution_moderation.go:440`; automatic-only eligibility | ADEQUATE: DistributedPipelineRun, UngovernedAndManual, ExistingFilters, RunWithdrawalCLI discovery/moderation, PublicJourney, ModeratedLegacyRefusal |
| FR-002 Verified obsolete lineage skips | WP02/05 | `runner.go:112` builds existing lineage once; `runner_eligibility.go:8` handles conflict/merged/closed/superseded before execute | ADEQUATE: Lineage tests each state with empty required-pipeline policy, no-effects snapshots and genuine eligible controls; UnmergedSiblings |
| FR-003 Canonical exact trusted threshold | WP01/02/05 | `policy.go:314` shares the prior governance algorithm; signed proposal base policy and exact candidate/pipeline/definition; latest qualifying sequence per trusted actor | ADEQUATE: PolicyPipelineEvidenceAcrossRequests, TrustedThreshold, InsufficientEvidence, PublicJourney two actual runners/across requests plus manual control |
| FR-004 Requester-only bounded signed withdrawal/retry | WP03/04/05 | `run_withdrawal.go:15,32,49`; `store.go:73` validates before signing; `run_withdrawal_cli.go:99` inspects identity/replays before loading signer | ADEQUATE: Shape, LocalRefusalBeforeMutation, hostile raw JSON/ordering/key/duplicate cases, public CLI normalized retries and changed intent refusal |
| FR-005 Portable verified closure | WP03/05 | `event.go:106`, `store.go:387,565`, `quarantine.go:1095,1147`, `shallow.go:1126`; common request dependency and relationship rule | ADEQUATE: GitAndArchiveRoundTrip, RefusesHostileGitAndArchiveBeforePromotion, MissingDependencyRecovery, SelectedShallowRecovery; public archive raw payload/signature comparison |
| FR-006 Future starts blocked/history valid | WP03/04/05 | `ci.go:340,369,452,468`, `runner.go:120`; historical evaluation/validation never filters withdrawn requests' results | ADEQUATE: otherwise eligible public withdrawal, forced execution refusal, AutomaticAndHistoricalFacts, DoesNotCancelInFlightPublication, archive continuation |
| FR-007 Explicit fresh exact remote target | WP02/05 | paired option validation and `runner_target.go:256`; ls-remote exact match; uncertainty propagates before backend; no local target write | ADEQUATE: StaleBase, MovedBaseWithoutOption, MissingTarget, target authentication/fault/cancellation/deadline/inherited-pipe tests |
| FR-008 Diagnostics/discovery/compatibility | WP02/04/05 | `runner.go:121,149,156`, static help/capabilities, full request output, updated protocol/operator docs | ADEQUATE: public discovery Git marker/outside repo/held lock/corrupt authority cases; reason assertions and full IDs; explicit old-reader actor-chain boundary documented |
| FR-009 Honest concurrency limits and disposition | WP05/root | Source docs and versioned issue matrix; #37 optional claims, #38 qualification; no exclusivity implementation | ADEQUATE documentation/process evidence: source/result binding and original issue reconciliation; no unsupported concurrent-start promise |

## NFR and constraint verification

| Requirement | Result and evidence |
| --- | --- |
| NFR-001 | Meets bounded probe contract: shared total 64 KiB budget for both pipes and all subprocesses; parent-derived 10 s context; process-group cancellation and WaitDelay; no stdin/askpass; literal OpenSSH batch configuration preserves auth. Unsupported wrappers refuse. Configured credential helpers remain trusted operator code. |
| NFR-002 | Real Git/public CLI/signed bytes/backend markers with finite package and subprocess budgets. Existing hosted/action scopes are bounded; the final manifest has exactly 22 unique roots. Deep native capacity/release/Mac runtime remain unqualified, expressly tracked. |
| NFR-003 | Skips/refusals occur before backend/result append; protocol/CLI tests snapshot refs, objects, signer state and markers before positive controls. Ordinary pre-existing sync receipts are accurately excluded. WP04's outer moderation mutation defect is fixed and independently tested. |
| C-001 | Native Go/Git, explicit runner, no mandatory account/API/service/database/Docker/extension. Both native-only and standard executable evidence exists. No new dependency/module changes. |
| C-002 | Shared governance evaluator preserves existing semantics; new strict decoding applies only to run.withdraw; old signed bytes/signatures unchanged. Local/import trust and actor-chain checks remain required; no sandbox/moderation relaxation. Frozen-fixture identity is recorded and product surfaces match approved source. |
| C-003 | This review edited only its reports/evidence. Root preservation records cover the original checkout, 78 recorded files, 21 stashes, old worktrees and frozen assets; baseline diff introduces no release assets/binaries or unrelated product rewrite. No old asset was rebuilt, relabeled or certified here. |
| C-004 | Claims/leases, interruption, memory replay, standing acceptance, hosting and Mac distribution remain separate. Existing #20/#21/#24 stay open; #37/#38 capture relevant remaining work. |

## Drift findings

No blocking spec/plan/contract drift found. All nine FR chains close through live callers and behavioral evidence. Shared runner.go ownership is resolved in the final combined code: eligibility precedes the existing execution path, observed withdrawal precedes it, and explicit execution deliberately does not apply automatic threshold suppression. The full-source timeout is retained honestly under the reviewed source/release qualification distinction.

## Risk findings and accepted limits

### NOTE-1 — Large withdrawal-history validation remains unqualified

Location: `run_withdrawal.go:61`, called once per withdrawal by complete relationship validation/import. Duplicate detection scans the whole event map, yielding O(W*N) work for W withdrawals and N observed events. This creates a capacity concern for large valid or hostile histories. Bounded tests reveal no failed bound; large-history qualification is not established. Already documented in threat/operator/delivery records and [#38](https://github.com/LynnColeArt/hubnot/issues/38). Non-blocking for this expressly source-only delivery; retain as a release prerequisite rather than imply scale certification.

### NOTE-2 — Observation cannot provide exclusive execution

Location: `runner.go:102–162`, `run_withdrawal.go:71`, `runner_target.go:256`. A runner executes from one verified fact snapshot and a separate target advertisement. A simultaneous/disconnected runner or later target movement may occur after observation. Withdrawals do not interrupt in-flight effects, and later valid results remain acceptable. This is expressly the spec's boundary and [#37](https://github.com/LynnColeArt/hubnot/issues/37), not a newly discovered delivery defect.

### NOTE-3 — Qualification evidence must keep its original source/variant labels

The actual native pipeline is 8f0db766/20 roots; final34ee0aa5/22 roots is supported incrementally. No final22 native run, exhaustive pass or release-binary/Mac runtime qualification may be inferred. The failed monolithic run's cause remains unknown. These limits are clear in the implementation matrix, independent review and delivery companion; preserve them in PR/issue closeout.

## Silent failure candidates

| Path | Empty/default result | Assessment |
| --- | --- | --- |
| `runner_eligibility.go:35` | no skip when policy pipeline absent | Intentional preservation of ungoverned explicit execution, positively tested |
| `run_withdrawal.go:71` | nil when no observed withdrawal | Intentional verified-fact projection; malformed/missing dependencies refuse earlier |
| `runner_target.go:117` | empty config on Git exit 1 | Means configuration unset; all other failures propagate, then supported defaults apply |
| `runner.go:129` | silent own-result continuation | Pre-existing already-ran behavior retained; new skip conditions emit diagnostics |
| `runner_target.go:98` | ignored final process-group cleanup error | Best-effort cleanup after joined command, with prior cancel/WaitDelay/error handling; no success fabricated from failed observation |

No new hidden-error path returns an eligible/empty success after a failed target observation or invalid withdrawal.

## Security Notes

| Surface | Review outcome |
| --- | --- |
| Target subprocess/SSH construction | exec argv protects ref/remote arguments; remote option injection is rejected; full ref validated; SSH command parser refuses shell programs and re-quotes literal words; BatchMode is applied. No event/request body is interpolated into a shell program. |
| Credential/process boundary | Ambient Git repository/config/object overrides and traces are removed; built-in prompts disabled. Explicit helpers/configuration remain operator-trusted; bounded cancellation is not a sandbox for arbitrary trusted helper code. No credential replacement or unbounded HTTP client added. |
| Signed withdrawal hostility | Strict UTF-8/full ID/4096-byte reason/new-kind fields and duplicate JSON checks; exact actor/key/earlier-sequence relation; conflicting duplicate facts and unexpected tree attachments reject before promotion. |
| Execution/moderation | Outer admission is preserved, withdrawn execution refuses before legacy signer migration, eligible control still executes; requester withdrawal grants no execution or moderation role. |
| Historical integrity | Import carries immutable history, not signer or governance authority. Withdrawal projection is absent from historical result/decision/merge validation. |

No blocking normal-operation security issue was found.

## Final Verdict

**PASS WITH NOTES** for the reviewed source-only mission.

The consolidated code realizes the nine FRs, three NFRs and four constraints in their declared scope, preserves ordinary inert/native use, and has no unresolved review rejection or blocking security finding. The fresh current-source race cohort passes. Exact approved product inputs survive consolidation; bounded standard/native-only integration, protocol and recovery evidence is adequate under the explicit amended validation policy.

Root may proceed with the reviewed PR/main delivery and merge-linked #27/#30 closure, keeping #37 and #38 open and retaining all qualification limits. No release approval follows from this report. Public tracker actions, runtime verdicts and product files were not changed by this reviewer.

## Retrospective Reminder

The runtime terminus authored versioned `kitty-specs/runner-efficiency-01M6HH8H/retrospective.yaml` at f268db84; it is present and was inspected. The delivery companion correctly explains its WP04 force warning as rejection/rewind rather than approval bypass, and preserves the validation-policy lesson.

Canonical follow-through is to verify that captured record, surface `spec-kitty retrospect summary`, and inspect `spec-kitty agent retrospect synthesize --mission runner-efficiency-01M6HH8H` in its default dry-run mode before applying any proposals. Root's companion records no synthesis proposals/applications. Do not rewrite event history to suppress generated warnings. If the retrospective record becomes absent or inconsistent, escalate instead of silently proceeding.
