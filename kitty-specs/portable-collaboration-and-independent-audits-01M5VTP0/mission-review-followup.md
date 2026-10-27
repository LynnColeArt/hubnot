# Independent mission-review follow-up: Go evidence discovery

**Verdict: PASS WITH NOTES. RISK-1/DRIFT-1 is closed at the fixed source plus its proof commit.** The actual primary full race gate passed, and independent whole-repository checks also pass after the proof was committed. This supersedes the earlier mission-readiness FAIL for the later reviewed tree; the original failure and report remain intact.

**Reviewer:** independent Codex agent `/root/mission_fidelity_review`, resolved `reviewer-renata`. The reviewer authored neither the implementation nor this correction. **Date:** 2026-10-27. **Mission:** `portable-collaboration-and-independent-audits-01M5VTP0`, ID `01M5VTP0RY3K4D31E0N8JVMGHR`. **Branch:** `feat/portable-collaboration`; all twelve WPs remain canonically done. **Public baseline:** `76446469b23d502a436c56cd7d704511d775b4d5`.

**Fixed source tested and packaged:** `61fa1c35fbe147fcca4d50b6fce37f0ae4c238db`, tree `fe62e94dd68a69e9ebbca86c8e21f7f9770c5758`. **Evidence HEAD reviewed:** `24a2fbf985fec8ad3063f473f8fc4414409911ed`. Original failed source is `170b3254e7ca7422b90fd149c38b9cb6f7fe339d`; original report/evidence commit is `e5833894`; canonical NFR-003 failure was subsequently recorded at `73b34581e6cc93f322452780ee7125b33b991b3e`.

This follow-up supplements the frozen [original mission review](mission-review.md) and [original evidence](mission-review-evidence.json). It does not rewrite their historically correct FAIL or claim the earlier c4 run validated the failing consolidated checkout. The original report contains the complete 26-row live-code → behavioral-test trace and review-history analysis. This follow-up independently evaluates the separate correction and current repository gates; all 26 dispositions are stated below. The mission spec, plan, three actual contracts, product code and original analysis remain unchanged. No generated WP review prompt was used.

## Correction and evidence integrity

RISK-1/DRIFT-1 arose because three legitimate historical root-package Go overlay inputs were stored as `.go` files under the module. `./...` discovered them as separate packages with undefined root-private helpers/types. At the failed HEAD, unqualified `go vet ./...` and compile-only `go test -run '^$' ./...` actually exited 1.

The separate author changed only evidence representation: exactly three R100 renames to `.go.txt`, plus `postmerge-evidence-fix/` documentation, original-path mapping, preservation verifier, v2 guards and controls. No build-ignore directive, product source, real test, assertion, CI invocation, default, memory threshold or module dependency changed. Each original Go payload is still available for deliberate overlay replay through a temporary resolved mapping.

| Original overlay payload under this mission | Preserved representation | Independently verified SHA-256 |
| --- | --- | --- |
| `tasks/WP02-native-audit-protocol-admission/review-independent-d274d5c8/independent_adversarial_test.go` | Same path plus `.txt` | `26b4abc29f261e7fef10bc55a7c1563dec888b7b11d79595aad0db5f0d1a719e` |
| `tasks/WP03-audit-execution-followup-issues/review-20923906/probes_test.go` | Same path plus `.txt` | `ed5e2f4d45d37c6c7ab52c5953830711eb7cbeefd489476460332db3f9a7904d` |
| `tasks/WP03-audit-execution-followup-issues/review-53c0ead2/probes_test.go` | Same path plus `.txt` | `af91829bba357baf27e0499441bbbe52079006098860e0b5d29ef69f81306275` |

The reviewer compared all three preserved files directly with the pre-fix Git blobs, lengths, Git modes and filesystem modes. All 39 protected historical files match their original bytes; the entire old WP12 `validation-c4aaa4e8/` directory has 18 unchanged files at original paths, including all 17 literal SHA256SUMS entries. Its archives, reports, source guard v1, wrappers and receipts were not repaired or rehashed. The original mission review and evidence JSON also remain byte-identical to `e5833894`.

The v2 guard closes the false-confidence seam separately: compare **all tracked `.go`, module and workspace inputs**, including metadata directories, against approved c4; inspect untracked inputs even when Git ignores them; allow only Go's inherent hidden/underscore/vendor/testdata discovery skips for untracked paths. Tracked hidden Go is still compared. Explicit error returns avoid Bash conditional-errexit false positives. Actual package discovery must equal the approved twelve-package graph, with no evidence packages.

Archived controls demonstrate that old v1 accepted the actual bad pre-fix primary, v2 rejected its exact three added Go files, and the representation fix passed. Isolated controls additionally demonstrate a Gitignored ordinary Go file really appears in `go list ./...` and is refused; tracked normal/hidden metadata Go is refused; non-Go governance changes pass; an inherently ignored hidden directory does not create a false package. The reviewer independently reran those controls after proof commit `24a2fbf9`; they exited 0 and reached every named positive/refusal assertion. This is actual executable control evidence, not inspection of the guard alone.

## Gate Results

The skill's Spec Kitty Python FR-023/037–041/SaaS commands target a different product. The applicable Hubnot charter/Go contract, architecture, public integration and issue gates apply here; no unrelated suite was run or waived. Framework `tests.architectural._gate_coverage` remains unavailable/no_coverage and **UNVERIFIED**.

### Gate Results — Contract

**PASS.** A new unfiltered `go test -race -count=1 -timeout 20m ./...` ran from `/home/lynn/projects/nichthub` at fixed HEAD `61fa1c35`, naturally completed with exit **0**, root package **627.964s**, all eleven tested packages passing and the example package reporting no tests. There are no cached-result markers. The reviewer parsed the actual live log and exit, verified before/after pinned HEAD and v2 source checks, then verified the archived bytes equal the observed live log. Log SHA256 is `61c6d16cab7c5e9099cf224b6d3717cc00231abceb47abc4ca0aac37101d7b98`. This is fresh primary validation of the corrected checkout, not substitution of the earlier c4 lane run. No concurrent expensive Go work or tracked edits ran during it; this reviewer waited until completion to run Go checks. This reviewer did not duplicate the completed full run or separately rerun its 10k dataset.

Independent `go test -count=1 -run '^$' ./...` exited **0** both immediately after the full run at `61fa1c35` and again at proof HEAD `24a2fbf9`. This is explicitly a test-discovery/compilation check with no tests executed; it establishes that the proof packet itself does not recreate the three broken evidence packages. The full execution result above supplies actual test coverage.

### Gate Results — Architectural

**PASS for applicable Hubnot gates.** After proof commit `24a2fbf9`, the reviewer independently ran `go vet ./...`, `go build ./...`, `go test -count=1 -run '^$' ./...`, and `git diff --check 76446469b23d502a436c56cd7d704511d775b4d5...HEAD`: each exited **0**. `gofmt -l` over all 189 tracked Go files exited 0 with empty output. The committed and working Go/module input inventory is unchanged between c4, 61fa and 24a2; all 190 tracked entries (189 Go files plus `go.mod`) match, including governance paths.

Independent `bash kitty-specs/portable-collaboration-and-independent-audits-01M5VTP0/postmerge-evidence-fix/check-go-discovery-v2.sh.txt /home/lynn/projects/nichthub c4aaa4e840420b07d83617c6373d6f4088e2c077` exited 0: unqualified `go list ./...` exactly returns the approved **twelve** packages, with zero evidence packages, and both source guards pass. `guard-v2-controls.sh.txt` and `verify-preservation.py.txt` likewise exited 0. Exact argv, final exit codes, raw-log hashes and all twelve package names are recorded in the companion evidence. All these checks were repeated after the additive proof commit; none relies solely on pre-packet results.

### Gate Results — Cross-Repo E2E

The unchanged public boundaries remain the compiled ordinary/`nogithub`/`noforge` native/audit/archive/Git-replication journey, actual GitLab-reader → GitHub-writer plus Git migration with guided EOF and lost-response recovery, and useful Azure Project-only Boards apply/resume. Their real effects and assertions were traced in the original review; no fixture output substitutes for implementation. The fresh unfiltered primary full suite exercises those unchanged public tests, including the integrated test's freshly compiled three variants; source identity also retains the earlier independently executed tagged matrices and actual compile-exclusion proof. No live provider account mutation or new platform execution is claimed. Applicable integration PASS is tied to those exact boundaries, not unrelated Spec Kitty cross-repository suites.

### Gate Results — Issue Matrix

**PASS.** `issue-matrix.json` has one terminal row, #13=`fixed`, with concrete integrated audit/follow-up evidence; zero empty/unknown/invalid/in-mission verdicts and zero deferred-without-handle rows. The external issue remains open until reviewed public PR merge. At this independent review, the canonical acceptance matrix correctly still records NFR-003=`fail` and overall=`fail` pending root's review-based update. This report supplies closure evidence; it does not mutate that authority or silently relabel the original failure.

## All 26 requirement dispositions

The original report's live code and assertion references remain valid because all 189 tracked Go files plus `go.mod` match approved c4 in committed blobs/modes and working bytes. There are zero tracked metadata Go files and no new Go/module/workspace discovery input. This comparison includes governance paths; it does not repeat the original blanket metadata exclusion. The separate full-primary run establishes the actual fixed checkout's repository gate.

| Requirement | Latest disposition and behavioral basis |
| --- | --- |
| FR-001 | PASS — native defaults/old signed encodings unchanged; failed audits neither qualify nor block native merge. |
| FR-002 | PASS — explicit selected Git/native closure and attributed foreign bytes survive; private identities, anchors, journals and unrelated working data remain excluded. |
| FR-003 | PASS — signature/dependency quarantine precedes observation refs; existing destination policy and local signer authority remain separate. |
| FR-004 | PASS — real provider records retain attribution, parent/thread topology and retrieved digest-verified blobs for offline inspection. |
| FR-005 | PASS — local/archive/provider routes share saved-plan apply/resume, including operation after the original source disappears. |
| FR-006 | PASS — guided decisions/preview and scripted execution share the engine; EOF and absent approval cannot authorize effects. |
| FR-007 | PASS — useful capabilities and concrete mapping/loss/unsupported/missing findings are inspected before writes. |
| FR-008 | PASS — common registry/conformance and actual CLI callers preserve explicit legacy transfers. |
| FR-009 | PASS — actual GitHub/GitLab issue/comment read/write and compiled self-hosted GitLab → GitHub plus Git journey. |
| FR-010 | PASS — separately qualified Forgejo/Codeberg and Gitea profiles with actual issue/comment effects and recovery. |
| FR-011 | PASS — useful Bitbucket Cloud repository/PR/workflow preservation; removed issues/Jira and unsupported destination recreation are not fabricated. |
| FR-012 | PASS — explicit Azure selectors/process/type/state and actual mapped item/comment writes; Project-only Boards works without repository conflation. |
| FR-013 | PASS — sealed checkpoint, persisted bound payload and confirmed parent receipts; uncertain writes require exact positive proof or manual recovery. |
| FR-014 | PASS — foreign workflows/reviews/users/CI remain inert; imports do not grant authority; physical destination hook anchoring remains enforced. |
| FR-015 | PASS — `noforge` really excludes all adapters; `nogithub` remains GitHub-specific; native/offline variants execute without credentials. |
| FR-016 | PASS — audit request/result facts remain distinct and cannot qualify merge evidence. |
| FR-017 | PASS — only explicit audit execute reaches the existing sandbox/unsafe-host boundary. |
| FR-018 | PASS — signed provenance, bounded findings/logs and relationship/dependency validation apply to local writes, reads and real replication. |
| FR-019 | PASS — explicit current local authority anchor, roles, accepted lineage and rule heads prevent imported/candidate/stale authority poisoning. |
| FR-020 | PASS — deliberate linked follow-up uses stable identities and retry guards, preserves edits and exposes concurrent-publisher limits. |
| FR-021 | PASS — fixed-source Go/Git-only packages reproduce with exact source/tree/compiler/version/BUILD/provenance and honest host boundaries. |
| FR-022 | PASS — current product docs/contracts/flags/limits remain synchronized; correction documentation explicitly preserves the original failure. |
| NFR-001 | PASS — positive operation/attachment/native budgets and refusal of incomplete success; no default or threshold weakened. |
| NFR-002 | PASS — hostile archives/native objects and origin/pagination/input failures still refuse before accepted visibility. |
| NFR-003 | PASS — actual fresh primary full-race exit 0 / 627.964s plus independent post-proof unqualified vet/build/compile/discovery/format/diff; RISK-1/DRIFT-1 closed. |
| NFR-004 | PASS — all eleven fixed outputs reproduce, 1,097 source entries match Git, eight binaries match prior qualified bytes; unavailable hosts remain explicit. |

## Exact fixed-source package inspection

The reviewer independently parsed the actual two fixed-source package sets under `/tmp/hubnot-postmerge-artifacts-61fa1c35-one` and `-two`, using Python tar/hash parsing and Git objects only. Packaging was not rerun concurrently with the full Go gate.

All eleven output files are byte-identical between the two runs. Complete checksums, nine archive inventory entries, normalized tar/gzip headers/order/timestamps/modes, embedded BUILD equality with provenance, all maintained docs, and all 1,097 source archive files/modes match fixed Git source. All three preserved `.go.txt` payloads are present exactly and their old `.go` members are absent. Provenance SHA256 is `0cd794e5e9229f0990eef2c3a6f8cfb43b05ec0e803e5fc6e234cfc1b7d13e24`; SHA256SUMS SHA256 is `3adef2aa39457cd11b5806f74abd5234f24c39dad89da259b5896eb4e7e3d0c4`.

All eight executable byte sequences equal the approved c4 packages under the same Go 1.26.4 compiler, release version and flags. Source archives and provenance correctly change to `61fa1c35`. Identical Linux AMD64/ARM64 executable bytes inherit the existing actual standard/native-only runtime evidence; this review claims no new remote/host execution, full ARM Go suite, macOS CLI qualification, Windows CLI distribution or macOS/Windows packager-host qualification. Those unavailable host limits remain UNVERIFIED and permitted by NFR-004.

## Finding disposition and nonblocking notes

**RISK-1/DRIFT-1: CLOSED.** The exact three uncompilable evidence packages are removed from Go discovery by lossless representation, the stronger guard rejects the original bad state and meaningful independent controls, all actual Go inputs/package names match the approved tree, the corrected primary completed the full required race suite, and final unqualified gates pass after the proof archive itself was committed. No remaining blocking drift, risk or security finding was identified.

The reviewer independently verified all 12 outer postmerge SHA256SUMS entries and all 336 indexed raw proof payloads: 336 unique safe regular members, exact membership and matching hashes. Archive SHA256 is `9fc8dd1931857fa60f52605286c32b28352d7600d796e3f5393cc3f9f718b6b7`. The packet preserves original failures, old-v1 false-positive and v2 red/green controls, actual full log/exit and source bindings, public transcripts and exact package inspection without private HOME/repositories/keys or release binaries. The companion `mission-review-followup-evidence.json` records independent checks and hashes; live reviewer logs/scripts remain under `/tmp/hubnot-mission-followup-review-61fa1c35/`.

NOTE-1 remains LOW: `internal/migration/plan.go:488` `DecodeReconciliation` has no production caller, while the typed `CanonicalReconciliation` validator is live in `ApplyMigration`. A later scoped cleanup should remove or justify the unused helper while considering the frozen internal contract. No dummy caller or unreviewed API cleanup was added.

The original platform/framework limitations, historical administrative force/rejection/dispatch records, generic actor metadata qualifications, failed prior performance/timeouts and concurrency limitations remain preserved. This representation-only correction adds no product security or silent-success path. The original audit/migration/packaging analysis and prior blocker closures remain unchanged.

## Final Verdict

**PASS WITH NOTES.** All 22 FRs and all four NFRs are now adequately substantiated at the fixed source and proof tree, with the explicitly allowed host qualification gaps. RISK-1/DRIFT-1 was an actual current-repository failure and required the new primary run; lossless metadata-only code equality was not used as a shortcut around it. No blocking finding remains. The unused decoding helper, additional host qualification and preserved process/framework notes are nonblocking follow-ups. Publication/PR merge/release remains a separate authorized workflow.

This reviewer changes no canonical acceptance/lifecycle state and performs no push, public merge, tag or release action. Root must record the latest canonical NFR-003/mission outcome from this committed independent follow-up rather than alter the frozen original failure or claim retroactive validation.

## Retrospective Reminder

The existing `kitty-specs/portable-collaboration-and-independent-audits-01M5VTP0/retrospective.yaml`, created at `20d3ceb6` with `RetrospectiveCaptured` event `01M5Y160AN6G07DZ9QH7T0TFBH`, remains present and verified; no duplicate record was authored. Inspect `spec-kitty retrospect summary` and `spec-kitty agent retrospect synthesize --mission portable-collaboration-and-independent-audits-01M5VTP0` (dry-run by default) to surface the actual post-consolidation discovery failure/correction and remaining nonblocking notes. Applying proposals is separate. Preserve original failures and source-bound successes as distinct history.
