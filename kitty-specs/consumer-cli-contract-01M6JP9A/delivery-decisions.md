# Consumer CLI contract delivery decisions

Mission **consumer-cli-contract-01M6JP9A** packages Hubnot issues #40/#41. All five WPs have independent canonical approvals and are now `done`; strict local acceptance passed and the mission merged into `feat/consumer-cli-contract`. Root owns the final PR, fresh Linux/macOS hosted checks, public main merge and tracker closure. No push, release or external tracker write occurred in this local orchestration.

## Source and validation binding

Baseline: `7a43830aec9fe75cdf2b8f9050591fe287fde801` (origin/main; baseline hosted run41748778618 green as checked by root). Reviewed/tested complete lane source: `ea505e406f25d895ec190597e3f60f878f71a695`. Strict acceptance: `6610a8c7e4b95dbdadad9a5a9e1706627778ed79`; all8FRs pass, all5WPs approved, no failed/skipped/blocked checks, optional quickstart warning only. Test records explicitly name the clean lane source/workspace; acceptance metadata commits are not test source claims. Earlier acceptance captures remain in the external evidence directory; the final capture includes exact selectors and source context.

Local merge: `f2339ad3d29ba3e315e1125b21de0a664267eaaa`. Postmerge lifecycle head: `39531cabfda28ff6b9dfed26743a1952871b9c30`. Comparison at that head found only mission/Op record differences from ea505e40: every production, test, CI and product-documentation byte is identical. Lane/mission branches and lane worktree are retained. Premerge primary mission artifacts and refs were backed up before consolidation; no recovery override was needed.

| Check at clean ea505e40 | Result |
|---|---|
| Actual default + nogithub,noforge public journey, race | PASS17.09s test /18.142s package |
| Native fast race + vet | PASS; existing21 roots unchanged |
| Explicit native integration | PASS19 roots,165.107s package /170.606s action |
| Canonical migration | PASS3 original shards,32.675s action |
|10 existing protocol/trust/lineage/shallow-recovery/identity-migration roots | PASS6.957s |
| Formatting, shell syntax, diff and live test inventory | PASS |

Integration preserves the original8 roots and adds all11 consumer roots, with exact discovery and fast-overlap guards. Existing8m package/600s pipeline deadlines, pipeline definition bytes and every pre-existing test/assertion remain unchanged. HN_COMMIT was exactly ea505e40. Source-assisted CLI SHA256: `756bc855d4d5022fcb59b8ebcbd463ed9d6456a0c5a8b9840571d80584b5d222`, built by existing -buildvcs=false/-trimpath native action. Actual journey child variants separately record their tags, binary digests and clean VCS source metadata. This is Linux source validation, including real native pipeline journeys; no released binary/macOS signing certification is claimed. No expensive stress/soak or broad full-suite rerun occurred; #38 remains separate qualification.

## Approved behavior and boundaries

The native optional hn.consumer/1 envelope reuses {ok,data,error} and the existing closed native error/recovery vocabulary. Proposal show/status and raw log export full identities; current reviews preserve full actor attribution, latest sequence for the exact candidate, and observed untrusted verdicts separately from policy qualification. Successor candidates inherit no predecessor votes. Reviewer pool/threshold deficits are not Fleet assignments; Fleet computes pending assigned reviewers itself. Readiness, disagreement, lineage and observed preconditions are separate; no field authorizes the caller or proves a fresh target merge. Writers still recheck signer, target and concurrency.

Review/decide receipts return the exact created stored ID; decision Policy/Evidence are stored values, never a later evaluation. Status still takes a proposal ID; evidence IDs resolve/read separately. Native prefix resolution uses the existing store resolver and distinct ambiguous_id, with bounded deterministic ambiguity fixtures. Every mutation and artifact full-ID rule remains intact. Ordinary event reads follow native evidence-to-candidate/request visibility; Subject is not a generic authorization parent, so visible successors remain readable after hidden predecessors. Raw log is explicitly raw signed history.

Discovery is static/keyless/outside-repository, without locks/Git IO/bootstrap. Existing non-JSON writer/read output and error inventories remain byte-identical. Nine contribution JSON parse sites/aliases are documented as already always-JSON; no incompatible wrapping transition occurred. Native core needs no plugin/service/provider. Event bytes/schema/signatures/history authority are unchanged. Reads load no signer/migrate no identity or append signed facts.

Unkeyed receipt errors use operation:null and do_not_retry; this adds no operation-key idempotency or lost-ack recovery. Existing writer ordering remains intact: early parse/ID/closure refusals prove private/public zero effects, while a later decide not-ready refusal may perform pre-existing legacy signer migration and still creates no signed facts/ref advancement. Tests/docs distinguish these cases. Memory recovery #21 remains out of scope.

## Independent approvals and process record

| WP | Reviewed source | Canonical approval event |
|---|---|---|
| WP01 | e4ffdf91c3a4eac357891f5b390872cb94de1e60 |01M6JQN5Q9HQ39DQKE178JKYSJ |
| WP02 |1b779037cfb52d7eac3c85c0b659f227f28dd3e2 |01M6JR7YQ8M8WA9FFXV4Y1HRGY |
| WP03 |72b793eefd77f32d4d00bd18ff1c53101db4c1ce |01M6JRWQZPDK1BW8E5ZB7QDTF3 |
| WP04 |5303cf932f297e94595180d8edd8c6761a954bc3 |01M6JSKDTY00ARFWM23REKJMQR |
| WP05 |ea505e406f25d895ec190597e3f60f878f71a695 |01M6JTFFFVNEEWC4EQQ9Q3VWT3 |

Reviewer reviewer-renata acted in a separate agent; detailed reports are preserved unchanged in reviews/WP01-independent.md through WP05-independent.md. One review cycle each, no rejection/arbiter/self-review fallback or forced approval. Planning review's medium ownership omissions were fixed before execution; later ownership clarifications used canonical finalize-tasks --refresh-planning-commit and independent review. Issue matrix #21/#38 uses actual existing Follow-up handles, not closures.

Framework baseline remains a pytest/JUnit no-artifact sentinel, not Go evidence. Every normal pre-review gate actually ran and separately reported unsupported Python no_coverage (tests.architectural._gate_coverage / No module named tests). No skip/gate-disable policy was used. CLI help mentioning sync-disable skipping that gate is stale; running #2801 code separates optional transport from gate controls. Optional hosted sync was logged out/unconsented and not enabled. Existing stale shared-workspace context warnings were retained without hand-editing status. Canonical subtask completion is event-sourced; static planning checkbox text was not changed as status.

Fresh independent postmerge mission fidelity review **PASS WITH NOTES** at faa0a24a: all15 requirements traced, no blocking source/security/trust/compatibility finding; independent focused cross-WP race passed10.970s. See mission-review.md. Notes are canonical metadata/scaffold presentation and honest validation limits; no runtime metadata was hand-edited. Generated retrospective verified unchanged; summary and dry-run synthesis exited0 with no proposals/applications/conflicts/events. See retrospective-verification.md. Final closeout changes are mission documentation only; the final delivery head is reported with a repeated source-tree comparison. Raw evidence, failed red attempts, clean-source results and premerge backups remain at /home/lynn/Documents/Codex/2026-11-04/hubnot-consumer-cli-contract. Original /home/lynn/projects/nichthub, its user files/modes/stashes and old worktrees, and all published beta assets were left untouched.

## Original issue closure trace

After fresh source review, supported issue-verdict commands explicitly recorded original targets #40/#41 as fixed in the local mission matrix, bound to reviewed/tested ea505e406f25d895ec190597e3f60f878f71a695 and strictaccept6610a8c7e4b95dbdadad9a5a9e1706627778ed79. Matrix now has four terminal rows: #40 fixed, #41 fixed, #21 deferred-with-followup, #38 deferred-with-followup. These are local source verdicts, not GitHub closure; root still owns PR/hostedCI/main merge and public #40/#41 closure, with #21/#38 remaining open.

Historical mission-review.md accurately observed two matrix rows at its exact reviewed faa0a24a. The later metadata-only issue trace does not change that source review, source bytes or verdict; independent readback is recorded in closure-addendum.md. Runtime/scaffold metadata fields were not hand-edited.
