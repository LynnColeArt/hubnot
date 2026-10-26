# WP01 independent correction review

Verdict: **approve**. Original P1 is resolved; no remaining blocking finding.
Product source validated: `3e6b301bb2e3142eff40727edf57a462b3e63e02`.
Reviewed lane HEAD: `a382f928884fdedc0e89ee39f2eed61973d94115`.
Both entire trees equal `2d88e80ba7db1fd22b3765fe1103bc160717a588`.
The evidence-only commit/revert did not change product or test source. This
review edited no product files and did not repeat the unchanged full race suite.

Read the full new runtime prompt, original feedback, correction diff, full
release implementation/callers, source-bound correction evidence and release
contract. The mission spec/plan/research/data-model/quickstart, AGENTS and charter
context read in the original review remain applicable. Scope remains WP01,
FR-001/021/022 and NFR-004, including the original packaging acceptance.

## Original P1 disposition

`Package` calls `checkSourceInspection` before `git status`. The preflight reads
effective configuration with `git config --null --name-only --includes --list`,
refuses clean/process filter keys, and only then inspects the index/status.
The check is case-insensitive and covers local, conditional/included and
worktree config. It refuses promisor/partial-clone configuration and indexed
gitlinks before nested fetch/submodule helper paths; fsmonitor is disabled.
Explicit safe refusal meets the original requested remediation without adding
an executable or weakening clean-worktree detection. Documentation states these
limitations and requires complete locally available source objects.

Reviewer built a fresh CLI from the reviewed lane tree. An independent harness
rechecked the original local clean-filter reproduction, conditional-onbranch
included process filter, and worktree clean filter with a mixed-case driver.
Every invocation refused before the marker-producing helper ran; source/config/
index bytes stayed identical and no output/staging appeared. See
`filter-recheck.py` and `filter-recheck.log`. The production regression also
exercises both public `release.Package` and an actual built CLI, plus staged
dirt, promisor/submodule refusal and a filesystem-monitor execution marker.

## Evidence and verification

- Reviewer `go test -race -count=1 -v ./internal/release ./cmd/hn-release` exits 0.
  Package tests cover the complete original reproducibility, source/provenance,
  metadata, unsafe-input/refusal/recovery contract and new helper-boundary tests.
  Both packages pass; see `focused-tests.log`.
- Reviewer reran the independent Go archive inspector, which imports no release
  library. Eleven assets from two fresh full-matrix worker runs are byte-identical;
  all 591 source files/modes, normalized gzip/tar paths/order/metadata, ten checksums,
  nine archive digests, docs, ELF/Mach-O architectures and executable provenance
  pass. Linux AMD64 standard and native-only binaries actually execute version,
  GitHub help, init and issue open/list. See `archive-reinspection.log`.
- Reviewer separately ran `reviewer-inspection.py`, asserting exact source
  commit/tree/time; complete target/variant inventory; equality of packaged and
  outer build provenance; executable/archive digests; controlled flags/env;
  unchanged local tags. Pass (`reviewer-inspection.log`). Reviewer Python is an
  independent inspection harness, not a product packaging dependency.
- Reviewer decompressed durable `race-detached.jsonl.gz` and recomputed raw
  SHA-256 `3dae5884ccf8a5d2fd015ce2a2ca25c90415b81fac1dab9ce90b816770a47215`
  and 1,122,663-byte size. Parsed counts exactly match the summary: 1,186 pass,
  zero fail, real Go exit 0, root 464.783 seconds, five tested packages pass.
  The sole package skip is the example with no tests. Before/after source both
  equal the validated commit. Current entire tree matches it. See
  `source-bound-validation.json`; original compressed evidence remains in
  `../fix-cycle-1-evidence-3e6b301b/`.
- Recomputed recorded worker digests for the packager/cross-compiled executables,
  PROVENANCE and SHA256SUMS against retained files. Worker static-checks JSON
  binds vet/build/format/committed-diff checks to the exact product source, all
  exit 0 with empty output. Reviewer mission-base diff check also passes.
- The normal completed framework pre-review gate is `no_coverage` because
  `tests.architectural._gate_coverage` is unavailable; `blocked=false`,
  `force_bypassed=false`. It is separate from Go validation and does not establish
  framework coverage. No gate was forced, skipped, killed or duplicated here.

## Runtime/contract and anti-pattern checks

| Prompt check | Result | Evidence |
| --- | --- | --- |
| Dead code | PASS | CLI `run` → `release.Package` → new source-inspection preflight is live; targeted call-site search confirms production wiring. |
| Synthetic fixtures | PASS | Tests invoke the production library or actual CLI; expected archives and marker absence are observed outcomes, not constructed outputs. |
| Silent empty return | PASS | Production nil returns are success after completed checks/writes or accepted help; subprocess/config/status/build errors propagate. |
| FR coverage | PASS | FR-001 retains development/native behavior with real packaged CLI smoke; FR-021/NFR-004 assert deterministic source/binaries/checksums, matrix and host boundaries; FR-022 docs ship and are checked against the contract and packaged source. |
| Frozen surfaces | PASS | Net diff remains the six owned release surfaces; signed facts, existing public-release assets/tags, native policy and go.mod are untouched. |
| Locked decisions | PASS | No product shell/Python/provider helper, new module, implicit publication, authority or tag path. Original executable-boundary contradiction now safely refuses. |
| Shared-file ownership | N/A | WP01 owns these six files; other finalized WP ownership lists do not claim these exact surfaces. Future adapter-free integration is explicitly WP06/WP12 work. |
| Production fragility | PASS | Fail-loud refusals explain invalid/unsafe/dirty/colliding inputs; error/cancellation paths preserve output/source state. No new panic/raise path. |

Contract enumeration: release-v1 applies; audit-v1 and migration-v1 are orthogonal
to this diff. Release contract executable boundary, explicit input semantics,
metadata/archive/docs/provenance/digest bindings, source isolation and native-only
tags now pass code/public-boundary inspection. It pins no concrete payload/flag
example or closed enumerated target matrix needing a separate SSOT mirror test.
Existing source default stays `0.0.1-dev`; packaged artifacts report explicitly
injected `0.0.2-dev`. Native-only uses both `nogithub,noforge`, as WP01 requires.

## Qualification and publication limits

Linux AMD64 packager host and both packaged Linux AMD64 flavors were executed.
Windows AMD64/macOS AMD64/ARM64 packagers were cross-compiled and their PE/Mach-O
types/digests inspected; host runtime remains unqualified. Linux ARM64/macOS
artifact runtime remains unqualified. No claim substitutes compilation for
runtime qualification. Tags remain unchanged; no publication/signing/notarization
or mutation of public release assets was attempted. Remote published asset bytes
were not fetched during review. The documented crash partial-output limit remains.
All-adapter noforge verification is an existing WP06/WP12 integration requirement,
not an unfulfilled WP01 acceptance criterion.

Canonical verdict is recorded after coordination with the root to serialize
shared runtime mutations; only WP01 is transitioned.
