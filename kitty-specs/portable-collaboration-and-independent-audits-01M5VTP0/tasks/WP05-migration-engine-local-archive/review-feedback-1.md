# WP05 review — changes requested

Reviewed product source: `ecaffbfc89affc63bf6af7289a55cd3ac403bdff` (lane-d).
Independent review used the full canonical review prompt and current PRIMARY
spec/plan/contract, including required destination `Locator()` and sealed initial
progress. The two earlier recovery defects are fixed at this source.

## P1 — Anchor destination ref promotion before executing repository hooks

`migration_native.go:126` calls `projectGit` for the destination ref transaction,
but `project_snapshot.go:396–400` supplies only `--git-dir` and leaves `Cmd.Dir`
unset. When migration runs from a source/unrelated checkout and the destination
has a relative `core.hooksPath` (for example `relative-hooks`), Git resolves that
hook path against the caller's working directory. This executes source/caller
`reference-transaction` code and bypasses the actual destination hook.

The independent production-boundary probe reproduced this with both a bare and
an ordinary worktree destination. Actual `PrepareMigration`/`ApplyMigration`
returned success, ran the SOURCE hook twice, and installed the observation ref,
although the real destination hook exits 1. A control anchored to the destination
ran the DESTINATION hook and rejected a ref write. Thus importing selected facts
can execute source code and evade operator destination policy. This contradicts
the charter hostile-input boundary, FR-014's inert imported workflows/source, and
the explicit documentation promise that source hooks never execute while
operator-controlled destination reference-transaction hooks retain behavior.

**Required correction:** anchor the final mutating Git command to the actual
selected destination's correct bare directory or worktree, including linked
worktrees, before hook discovery/execution. Keep existing expected-old atomic
promotion, bounded output, context cancellation and isolated global/system Git
configuration. Do not fix this by globally disabling hooks, adopting ambient
configuration, changing the existing audit CLI helper, or executing through the
source repository. Preserve correct destination hook refusal as an uncertain
write with exact-positive reconciliation/manual recovery semantics. Add real
boundary regressions proving source hooks never run and a destination relative
hook executes in the right directory; its refusal must prevent observation-ref
visibility. Cover bare, ordinary worktree and linked-worktree destinations.

**Evidence/reproduction:** adjacent `hook-probe-ecaffbfc/` contains the source-bound
Go overlay, reproducible launcher, failing race output, source/destination marker
bytes, positive control and hashes. Existing focused migration race tests and
`internal/migration` race tests pass; the new hostile execution probe fails.
No product source was edited for this review.

## Required checklist

1. **Dead code — PASS within WP boundary.** Internal plan/progress/native modules
have production call sites. Root engine/source/destination entry points are the
explicit downstream WP06/WP11 integration API; the mission deliberately schedules
CLI wiring in WP11. They are exercised through production boundaries in this WP.
2. **Synthetic-fixture test — PASS.** Canonical validator tests call the actual
validators; local/archive/recovery/audit/TLS cases call the actual engine/native
and Git boundaries. Literal payload inputs are test inputs, not fabricated output.
3. **Silent empty return — PASS.** Explicit empty results represent missing refs,
no imports, or successful guards. Errors propagate; missing uncertainty evidence
never becomes permission to retry. No panic/exception-swallowing path found.
4. **FR coverage — PASS for test presence, blocking behavior failure above.**
FR-002/003: audit-only private export/admission and signer/policy preservation;
FR-005: local/archive round trip plus shared composed code transport;
FR-007: closed schema/loss-decision validation; FR-013: bound comment resume,
missing checkpoint, changed destination, negative-lookup and lost-ack cases;
FR-014: attributed inert foreign records/workflow cases, with the uncovered hook
execution failure above; FR-015: provider-neutral local/imported record boundary
and dependency inventory. Actual `noforge` adapter exclusion is WP06 scope;
unknown-tag compatibility is not adapter-exclusion evidence. NFR-001/002:
verified blobs, ordinary closure, audit union bounds and hostile archive cases.
5. **Frozen surface — PASS.** `git log a99daad2..HEAD --` for the frozen WP04
snapshot/archive/schema/test/docs files is empty. No signed protocol/history or
public release/tag artifact change.
6. **Locked decision — FAIL.** Source repository hooks execute during import;
actual destination hook behavior is bypassed (P1 above).
7. **Shared-file ownership — PASS.** `project_snapshot.go` changes alone have
PRIMARY owner concurrence in `ownership-handoff-wp04-wp05.md` and
`integration-advice-wp05.md`; commit 5444f623 records the required rationale.
8. **Production fragility — PASS.** New Go errors are bounded refusal/recovery
paths, including races and uncertain effects; no unguarded new panic found.

## Validation and limits

- Independently passed `go test -race -run '^TestMigration' -count=1 -v .`
  (7.910 s) and `go test -race ./internal/migration -count=1 -v` (1.057 s).
- Independent hook overlay race run fails in both subtests (0.398 s).
- Whole-WP `git diff --check a99daad2..HEAD` passed; fixed-head implementation
  full race/vet/build/format evidence was reused after its SHA-256 checks passed.
- Earlier checkpoint/destination probes remain covered by the passing focused
  run. No provider account, public release/tag, source/product or other WP edit.

Verdict: **request changes; WP05 returns to planned**. WP06, WP11 and WP12 must
wait for correction and a new independent review before consuming this head.
