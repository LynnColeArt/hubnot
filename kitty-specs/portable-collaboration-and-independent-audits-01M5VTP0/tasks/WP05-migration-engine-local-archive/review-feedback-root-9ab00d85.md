# WP05 direct independent review — changes requested

Reviewed source: `9ab00d85dee0fac114f303d5eecd2cd7e00f5cae` (lane-d).
Reviewer: root Codex, at the user's explicit request; no product edits.

## P2 — Refuse a missing reciprocal linked-worktree pointer

At `migration_native.go:196–202`, an ENOENT while reading the registered
worktree's `.git` file is accepted. The resolver then returns the pointer's
parent directory without proving that it is the selected destination worktree.
A stale registration that names an existing unrelated directory plus a missing
`.git` therefore anchors the mutating ref transaction in that unrelated directory.

The attached Go overlay exercises the actual ExportProjectSnapshot →
PrepareMigration → ApplyMigration path. It creates a linked destination whose
relative reference-transaction hook rejects writes, then changes the private
Git directory's `gitdir` backpointer to an unrelated existing folder whose
`.git` does not exist. The selected destination's own `.git` pointer is intact.
On this head:
- ApplyMigration returns nil and installs the observation ref.
- The unrelated directory's accepting hook runs twice.
- The selected destination hook does not run during migration.
- A control explicitly anchored at the selected destination runs its rejecting
  hook and refuses a fresh ref write, despite the same stale metadata.

This is a local malformed/stale-registration case, not evidence of remote control
of repository metadata. It nevertheless violates the correction's stated
fail-closed anchor verification and retains the original policy-bypass failure
under a broken linked-worktree registration.

Required correction: do not accept a missing reciprocal pointer as a verified
anchor. Fail before promotion when registration cannot be verified, or carry and
validate the explicitly selected destination path rather than reconstructing it
from unverified metadata. Keep bare, ordinary and valid linked-worktree support,
destination hook refusal, isolated configuration, bounded/context-aware Git calls
and atomic expected-old ref guards. Do not disable hooks. Add this production
boundary regression alongside the healthy layout cases.

## Verification and scope

- Inspected Kat's complete two-file correction against `3269fabbbdb526830acddc72f9450d6519062414`.
- Existing migration tests independently PASS: `go test -race -count=1 -run '^TestMigration' .` (8.521s), including bare/ordinary/linked hook refusal and TLS fixtures.
- Shared plan/progress tests independently PASS: `go test -race -count=1 ./internal/migration` (1.060s).
- New overlay FAILS as expected (0.215s); the control proves selected destination rejection is configured and active.
- Reused the already verified exact-head correction-9ab00d85 full-race/vet/build/format evidence. No unnecessary duplicate full race run.
- The existing original review's broader WP05 assessment remains useful; this review adds a concrete behavior defect in the correction. APIs deliberately consumed by later WP06/WP11 remain integration obligations; no dummy call sites or cross-WP ownership changes are requested here.
- No source, destination user project, live provider, public release/tag or other WP was changed. The fixture uses disposable local repositories; the overlay leaves product source unchanged.

Evidence: `direct-review-9ab00d85/` contains the probe, reproducible overlay,
raw failing output, focused passing output and hashes.

Verdict: **changes requested**. WP06/WP11/WP12 remain dependent on WP05 approval.
Administrative ownership rewinds in the event log are not implementation reviews.
