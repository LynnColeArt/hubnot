# WP05 implementation validation (hook-anchoring rework)

Source: `9ab00d85dee0fac114f303d5eecd2cd7e00f5cae` (commit `9ab00d85`, lane-d) on base `3269fabbbdb526830acddc72f9450d6519062414`.
Worktree clean at this source; two files changed vs the merge base:
`migration_native.go` (+110/-1: anchored ref transaction + fail-closed
`migrationDestinationRepoDir` resolver) and `migration_native_test.go` (+128:
production-boundary regression). `migration_local.go` unchanged (read commands
never execute hooks; verified). No other lane changes.

## Checks (all exit 0 at this exact committed source)

- Full `go test -json -race ./...`: exit 0; root pkg 528.953s; Test+subtest pass
  events 1276 (prior cycle 1254 +18 merged WP03 audit tests +4 this regression).
  Zero failures, one environment skip (unchanged pre-existing skip).
- `go vet ./...` exit 0; `go build ./...` exit 0; gofmt: no violations.
- Whole-WP `git diff --check ecaffbfc..HEAD` exit 0; complete-diff exactly the
  9-file whole-feature stat expected from the two lane merges plus this fix.
- nogithub-tests (29-test focused set rebuilt from the prior cycle's own log:
  25 root + 4 internal/migration) `-race -tags nogithub`: exit 0 (19.3s).
- nogithub binary build: exit 0. noforge tag-compatibility (same focused set +
  internal/forge + internal/githubapi, `-tags noforge`): exit 0 (11.9s).
- Independent recovery probes (missing checkpoint / changed destination /
  lost-ack reconcile): exit 0.
- `internal/migration` dependency inventory: no provider-client packages.
  Current tree still lists internal/forge + internal/githubapi (WP06 later
  removes; documented, same as prior cycle).
- **Reviewer probe re-run at this head** (`hook-probe-ecaffbfc` rig, unchanged):
  `TestReviewWP05DestinationRelativeHookIsolation` now PASSES (exit 0, 10.3s;
  it failed at ecaffbfc with exit 1). bare + worktree subtests green. The
  linked-worktree scope the prior probe did not exercise is covered by the new
  permanent regression (`.../linked-worktree` green).

## Fix mechanics (response to review-cycle-1 P1)

The destination `update-ref --no-deref --stdin` transaction now runs with
`-C <destination-root>` plus the already-resolved `--git-dir`. The root is
resolved by `migrationDestinationRepoDir`: cwd-independent classification only —
`rev-parse --is-bare-repository` for bare layouts (anchor = git dir), the
`<gitdir>/gitdir` back-pointer with bidirectional verification for linked
worktrees (anchor = worktree root), the sibling `.git` directory/file
(separate-git-dir) pointing back for ordinary repositories (anchor = git-dir
parent). Refuses (`destination repository/worktree anchor invalid/unavailable`)
when no sound anchor can be proven; the mutation is never run unanchored.
Atomic expected-old guards, destination hook refusal→uncertain-write semantics,
bounded output, context cancellation and isolated Git configuration unchanged.
A key wrong-turn is recorded honestly: `rev-parse --show-toplevel` under an
explicit `--git-dir` resolves from the PROCESS CWD (returned the caller's repo
in a live probe) — it was replaced by the cwd-independent queries above; a
calling-process unit probe (`TestKatUnitProbePromoteAnchoring`, scratch-only)
demonstrated both the poisoned and corrected anchors.

## Recording notes (honest deltas from the prior evidence bundle)

- Prior cycle ran a detached supervisor recording per-check `run.json`s; this
  cycle's checks ran from a controlled kernel session at the single committed
  source with exit files per check (`.exit.json` in this bundle) and full
  stdout/stderr where produced.
- `race.stderr` is empty because the runner merged stderr into the JSON stream
  (same stream); the race JSON itself is the primary artifact.
- The prior `raw-logs.tar.gz` was extracted to a scratch dir during a
  mid-flight file-name collision check; no running check was affected — a
  redundant salvage reader was attached to the live race fd and closed without
  effect on the completed run (harmless, recorded for completeness).
- The framework's `tests.architectural` hook is unavailable in this environment
  and reports no_coverage at the `for_review` gate; it is a framework report,
  separate from the actual Go checks above (same as prior cycle).
- Canonical owned task rows T013/T014/T015 were marked done after checks
  (mark-status.json). No lane/frontmatter/status hand-edits; the implement
  claim itself was user-authorized and recorded by hermes (event
  01M5WJRMPVF66PCA1KJYDW84EV / 01M5WJRRG229BY6THTP8WZ46QB).

## Response scope statement

No provider code, workflow activation, credential adoption, source-repository
execution, global hook disabling, or process-environment mutation. The bounded
shared exporter/bridge is preserved (no duplicate exporter/admission
implementation; `project_snapshot.go` untouched by this rework). Evidence
bundle: `SHA256SUMS` covers every file in this tarball.
