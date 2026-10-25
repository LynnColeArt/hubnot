# Native policy rule validation

## Verified source

Runtime implementation: approved integrated `38a7200374b3da96dab5ea2763fca588b77af4f8` with protected-baseline correction `ef14e4c56ab5d1666039631a029586115b9b2d35`. Final public test/documentation source: `250224e8812141e948f950ac23619662aebe0068`. WP03 changes tests/documentation only; runtime Go source is unchanged.

## Completed verification

| Source | Verification | Result |
|---|---|---|
| WP01 ef14e4c5 | Full `go test -race ./...`; vet, build, diff | PASS; main package 438.761s |
| WP01 ef14e4c5 | Independent original baseline-reversion repro plus positive/legacy race checks | PASS 2.711s; the former exploit refuses before any target/worktree change |
| WP02 38a72003 | Full `go test -race ./...`; vet, default/nogithub builds, diff | PASS; main package 428.626s |
| WP02 38a72003 | Combined protected/automerge/merge/revision race | PASS 29.293s |
| WP02 38a72003 | Independent fresh automerge race checks | PASS 18.356s |
| WP03 250224e8 | Public CLI matrix, `go test -race -count=1 -run ^TestPolicyRulesPublicAcceptance$ .` | PASS 71.770s; default and nogithub binaries |
| WP03 fault injection | Scratch overlay removes client acceptance-readiness gates | Expected FAIL: public test detects acceptance before designated guardian approval; product files untouched |

At creation, final release suites and WP03 review were pending. They have now completed successfully as recorded below; functional evidence and global release readiness remain separately identified.

## Functional evidence

| Criterion | Verified behavior and evidence |
|---|---|
| FR-001 | Public absent/null/default and independent toggle cases; unchanged repository policy and module dependencies. Native initialization does not add these sections. |
| FR-002 | Public passing no-op lint run plus ordinary approval still refuses acceptance until designated non-author review; exact guardian/ordinary/run IDs appear in signed acceptance. Direct authority tests cover distinct/non-author actors. |
| FR-003 | Signed-tree literal path tests cover addition/deletion/rename/mode/ancestor replacement. Public candidate self-disable retains base protection and refuses acceptance; current-target baseline/policy movement requires revision. |
| FR-004 | Shared local/quarantine exact evidence validation; real bare-remote verified acceptance; withdrawal and hidden same-policy target reversion regressions; independent reproduction now refuses. Historical repair and legacy absence remain intact. |
| FR-005 | Policy inspection/amendment tests expose independent activation, designated roles/thresholds, paths and named automerge changes, including disabled/unchanged diagnostics. |
| FR-006 | Typed bounded explicit named rules; actual author/actor/full branch/literal path refusal tests. Ancestor path permission and same-name tag ambiguity cannot broaden a rule. |
| FR-007 | Public keyless full-file/ref/index snapshots remain identical after eligible preview. Existing review/run/acceptance claims are created through public commands; execution adds only the signed merge fact. |
| FR-008 | Expected-old prepared ref race, exact real signer, under-lease current evidence, trusted failures/rejections/change requests, target movement and protected/policy manual-only cases. Public stale preview and baseline regressions refuse with state preserved. |
| FR-009 | Public two-parent local merge and existing signed merge event; remote refs remain unchanged. Interrupted/missing-fact manual repair, lost acknowledgement and safe checkout rollback/ignored-file preservation tests. No automatic pipeline or publication. |
| FR-010 | Complete public matrix runs real separate native actors, bare Git transport and Git-only child PATH in both default and nogithub binaries. No direct private event constructors or provider credentials/tooling. |

## Bounds and limitations

Changed-path output is capped while reading at 8 MiB, policy/configuration reads are bounded, literal arrays/rules/actors are capped, and transaction acknowledgements/process exit have finite waits. A prepared lease does not expire during inspection/checkout. Ignored collision checking uses ordered lookups rather than a quadratic cross-product. The embedded-buffer ReaderFrom regression is tested.

Automatic operation explicitly refuses shallow/partial/promisor repositories and unfinished Git operations before object reads. Configure required checks through native pipeline policy; commit-tree creates the automatic commit and operator Git ref-transaction hooks retain their normal behavior. Protection does not semantically classify every lint change or control raw Git/host/keyholder actions outside Hubnot. Distributed event and branch state are observed snapshots, not a claimed global transaction.

## Framework evidence

Canonical generic baseline/pre-review tooling assumes Python/JUnit and reports absent tests.architectural coverage as no_coverage/unverified in this Go repository. Raw outputs are retained without modification; no dummy Python file, fabricated JUnit, skip flag or approval override was used. The actual Go commands and independent Go source reviews above supply implementation verification.

## Final release gates

All pre-consolidation release gates passed at frozen `250224e8812141e948f950ac23619662aebe0068`:

- Full `go test -race ./...`: PASS; main package 516.200s.
- Full `go test -tags nogithub ./...`: PASS; main package 248.517s; GitHub API package excluded.
- `go vet ./...`, `go build ./...`, formatting and `git diff --check`: PASS.
- Five documentation JSON examples parse successfully.
- Independent WP03 public focused race spot checks: PASS 27.789s; canonical approval recorded with no product edits.
- All three work packages have canonical independent approvals. All ten functional criterion verdicts were recorded through the canonical acceptance-verdict command.

The root verification helper hashes length-prefixed relative path, Git mode and actual blob bytes in sorted order (SHA-256), excluding governance directories from the product set. The tested-source fingerprints are:

```text
Go source plus tests and module: 6e5b784dbae8b27609898df5fe2a32dc031f8c244eff09ef4294142e1adbc29d
Runtime Go/module (no tests):  d6b968a640f2a62bea7606af04826239a87bc81f9f290705d2ff340e4855b189
Product tree with user docs:   6f2f47907158e796d53b8aa2fe33ca848371a2362672e7e0d87d478112a202c8
```

Runtime Go/module is identical to the independently reviewed implementation at `38a7200374b3da96dab5ea2763fca588b77af4f8`. Consolidation must preserve these tested product fingerprints; its result and final mission audit will be recorded after the local feature merge.

## Consolidation and final audit

Canonical acceptance passed in local mode (`7b7b924e5920cc8bac7ea78ccf532b980f68606c`) after the required real contract reference was added. Canonical lane consolidation merged into `feat/opt-in-policy-rules`, marked all three independently approved packages done, and removed the three clean execution worktrees. Public main remained `faf975be30ef29d898eea1d113bed3c3f6f7fe58`; no main merge or implicit publication occurred.

Root and independent recomputation after consolidation matched all three tested fingerprints exactly (109 Go/module files; 243 product/doc files). There was no new product code or user-documentation change to justify repeating the same full suites solely for changed Git history.

The independent post-consolidation Go mission audit in `mission-review-report.md` is PASS with all ten FRs traced and zero unresolved product/security blocker. The generic framework helper is unavailable because its Python interpreter lacks pytest; that failure remains separately and losslessly archived in `mission-review-tool-report.raw.gz` with a SHA-256 manifest/readable companion. The archive decompresses byte-for-byte to the original captured tool blob. No scanner success, test waiver, fake fixture, global dependency installation or format-check exemption was substituted. Whole-PR `git diff --check main...HEAD` passes after representing the padded raw diagnostic losslessly.

The runtime-authored retrospective was preserved. The audit explains its heuristic `force` label: a normal feedback-backed backward rejection revoked WP01 approval when the hidden target-baseline bug was found. Spec Kitty's shared wire bridge internally marks that rewind as force; this was not a forward approval override or a skipped gate. Subsequent correction, full tests and independent reapproval are all source-bound in the event history. Process lessons cover current protected target context, bounded writer ReaderFrom behavior, meaningful public negative controls and freezing before long release gates.
