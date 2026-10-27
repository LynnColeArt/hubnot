# WP05 implementation validation (P2 stale-registration rework)

Source: `bc6c8db5f6292d53509c068b11302e91a80a3ce0` (commit `bc6c8db5`, lane-d) on prior landed correction `9ab00d85`
merged with WP03 lane-b at `13922bea`. Response to `review-cycle-4.md` P2
(direct-review-9ab00d85/). Worktree clean at this source; two files changed
vs `13922bea`: `migration_native.go` (+38/-18: reciprocal pointer read must
succeed and verify back to the selected git directory; every failure refuses
before promotion) and `migration_native_test.go` (+99:
TestMigrationDestinationStaleWorktreeRegistration). No other changes.

## Checks (all exit 0 at this exact committed source)

- Full `go test -json -race ./...`: exit 0; root pkg 532.604s; Test+subtest
  pass events 1284 (prior landed-cycle 1276 + this regression); zero failures.
- Focused `go test -race -run '^TestMigration' .`: exit 0 (8.518s), including
  the new stale-registration regression and the healthy
  bare/ordinary/linked-worktree hook-anchoring regressions.
- `go test -race ./internal/migration`: exit 0 (1.059s).
- `go vet ./...` exit 0; `go build ./...` exit 0; gofmt: no violations.
- Whole-WP `git diff --check 3269fabb..HEAD`: exit 0; complete-diff unchanged
  in shape (two lane merges + the two WP05 correction commits).
- nogithub focused 29-test set + binary build: exit 0. noforge tag
  compatibility incl. internal/forge + internal/githubapi: exit 0.
- Recovery probes (missing checkpoint / changed destination / lost-ack): 0.
- internal/migration dependency inventory: no provider-client packages;
  current tree still lists forge+githubapi per plan (WP06 removes).

## The P2 correction (fail-closed reciprocal pointer)

In the linked-worktree branch, `os.ReadFile(p)` on the registered `.git`
pointer may no longer be skipped: it must succeed, carry `gitdir:` with the
selected git directory as its physical target, and the returned worktree root
must evaluate physically to the registered side's parent. Any failure —
missing pointer, unreadable, wrong/relative-unresolvable target, missing
directory — refuses `destination worktree anchor invalid` BEFORE the
ref-transaction, so a stale registration can neither install the observation
ref nor execute any hook in any directory. The regression additionally
asserts the pre-created marker file stays EMPTY (no hook anywhere) and reuses
the reviewer's unchanged anchored control proving destination hook rejection
was configured and active.

## Scope notes (honest)

- Scope held to review-cycle-4 only: the P2. The cycle-3 astra finding
  (production-reachability gate vs deferred WP06/WP11 wiring) remains an OPEN
  owner decision, NOT resolved by evidence or by this correction;
  `DecodeReconciliation` was intentionally NOT removed this cycle.
- Cycle-3's verdict was never recorded in the mission event log (review-sandbox
  event-write failure); this cycle's governing recorded review is
  review-cycle-4.md (P2). Noted to avoid double-counting verdicts.
- Reviewer's direct-review-9ab00d85 probe was independently run against the
  fixed scratch copy during rehearsal (red before, green after, no product
  file touched); final validation used only in-lane product tests above.
- Canonical task rows remain marked done from the landed cycle; no new
  frontmatter/status hand edits by implementer beyond the canonical claims.
