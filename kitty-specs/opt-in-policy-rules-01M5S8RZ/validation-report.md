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

Full final WP03 race/native-only release suites and independent WP03 review remain pending at this report's creation. They are global readiness gates and will be recorded below before mission acceptance; the completed functional evidence is recorded separately from those pending release gates.

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

Pending final frozen-source release commands, independent WP03 review, and consolidation source-fingerprint comparison.
