---
affected_files: []
cycle_number: 1
mission_slug: runner-efficiency-01M6HH8H
reproduction_command:
reviewed_at: '2026-11-03T23:14:23Z'
reviewer_agent: reviewer-renata
wp_id: WP04
---

# WP04 independent review — changes requested

Fixed source: `364939d0bd968c7935db1cfc70d25e5012f5a1dd`.
Observed review HEAD: `8423df2819edb1a0a579b5eb12eafbe0b6326c71`, identical product/test source.
Reviewer: reviewer-renata, independent of implementation.

## WP04-R1 — [P2] Refuse withdrawn execution before moderation migrates the signer

Location: `contribution_moderation.go:494`, reached by `main.go:run` before the withdrawal guard at `ci.go:452`.

With writable contribution moderation enabled, an admitted proposal/request already withdrawn by its original requester, and a valid legacy-only identity, the actual public command:

```sh
hn run execute FULL_REQUEST --backend host --allow-unsafe-host-execution --rerun
```

returns the expected withdrawal refusal but first runs `contributionRunBoundary`'s `loadIdentity()`. That loader migrates private signer state before `cmdRunExecute` can apply its new guard. The actual fixture gained these private paths:

- `/.git/hn/active`
- `/.git/hn/identities/f2fe08bc5c553a22734da63deeb772a3be4f25063abcad503e4659c54b24b54a.json`

The backend remains blocked, but the intended refusal-before-signer-work/no-effects boundary is not satisfied. The same migration can occur regardless of `--rerun`; unsafe-host opt-in does not authorize bypassing the withdrawal refusal.

Minimal fix expectation: ensure an observed withdrawn execution refuses before the outer moderation execution path performs identity migration. Preserve existing proposal admission and execution permissions for eligible requests, normal writer locking, original-requester-only withdrawal authorization, and valid in-flight/late result acceptance. Do not introduce a general identity/keyring redesign, new cancellation role, or changes to historical evidence semantics.

Add the combined enabled-moderation + legacy-identity + withdrawn-public-execution regression. Keep the actual CLI, pre-created backend marker and before/after private-state checks; retain an eligible positive execution control. The existing external test-only overlay probe can be carried into implementation as a scoped regression.

## Causal reproduction

The external overlay adds only `TestReviewModeratedWithdrawnExecutionPreservesLegacyIdentity` to the test harness; production and repository source files were unchanged. It builds/runs the actual standard CLI in its existing closed environment, establishes writable moderation and withdrawal, converts the fixture to legacy identity form, then asserts the refused execution preserves private state.

```sh
go test -overlay /home/lynn/Documents/Codex/2026-11-03/hubnot-runner-efficiency/WP04/review-moderated-legacy-overlay.json -race -count=1 -timeout=2m -v -run '^TestReviewModeratedWithdrawnExecutionPreservesLegacyIdentity$' .
```

Actual result: exit 1, package 3.795 seconds / wall 22.313 seconds. The command reached the withdrawal refusal; the assertion failed specifically because the two keyring paths were created. See `review-moderated-legacy.stdout.log`, `.result.json` and the overlay test source in that external WP04 evidence directory. No key material was printed.

## Other independent validation

The existing public CLI journeys both passed (standard and explicit nogithub,noforge), plus TestDistributedPipelineRun and TestRunWithdrawalDoesNotCancelInFlightPublication: exit 0, package 18.635 seconds / wall 19.337 seconds. Exact JSON events and head binding are in `independent-review-race.*`. All 154 production hashes in source-binding.json matched the observed source; only five owned implementation/test paths changed. No unchanged fast/full/deep campaign was rerun. Existing full-suite timeout and deep release qualification remain incomplete, without a new full-suite or Mac runtime claim.

## Required anti-pattern checklist

1. Dead code: PASS; new discovery/writer/receipt/execution helpers have live callers.
2. Synthetic-fixture test: PASS; actual CLI/Git/signed history/backend controls are used.
3. Silent empty return: PASS; exact static bypass and nonwithdrawn guard outcomes are deliberate.
4. FR coverage: FAIL for the combined moderated legacy execution-refusal boundary demonstrated by WP04-R1; existing controls cover the other scoped behaviors.
5. Frozen surface: PASS; no immutable fixture/release path changed.
6. Locked decisions: FAIL for WP04-R1's intended refusal-before-signer-work/no-effects guarantee.
7. Shared-file ownership: PASS; ci.go/main.go/contribution_moderation.go changes are owned by WP04; runner.go's approved WP02/WP03 scheduling/withdrawal behavior is reused in sequential lane-b and must remain intact.
8. Production fragility: PASS; other new refusal paths are explicit and bounded.

Verdict: REQUEST CHANGES for WP04 only. No WP05 implementation is reviewed or approved. Root handles any later dependent dispatch; unstarted WP05 should inherit the corrected approved lane tip before implementation.
