---
affected_files: []
cycle_number: 10
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T16:54:45Z'
reviewer_agent: codex
wp_id: WP16
---

# New post-approval maintenance finding — first-byte deadline classification

Root/architect-alphonso, 2026-10-31. This is a newly observed WP24 hard-gate defect after WP16 source ae5e4f8b approval01M693G7EDMQZKZFKWN5V6FF2N/cycle9, not a rewrite of the prior successful review or an implementation edit. Prior verdict/evidence remains historical. Required independent review follows the correction.

## P2 — Preserve cancelled for a first-byte OS read deadline

Actual WP24 `TestExtensionDefaultFailureConfigurations/native-only/blocked` ran a real registered/enabled module whose startup-hang helper sleeps indefinitely without a ready frame. At10.10 seconds the supplied native-only host returned host/unavailable_module, message "module ended before ready", rather than the required pre-dispatch deadline host/cancelled. The standard blocked case passed; both paths are subject to receive scheduling. Do not widen the accepted error set or repeat the failing case until it happens to pass.

Exact retained failure: `/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP24/startup-deadline-failure-excerpt.log`, SHA256 `3ca6fd127d91570c8ab589dc2678f84e5bf944abce8bcb7188e366849f91f732`. Actual failing host SHA256 `990ebf644204a74f5b6228b09383b1c2985d1a66aa4a665427b511dd1a1a003f`; exact bytes and actual buildinfo retained under that WP24 directory's `failing-source-assets/` and `failing-source-assets.json` (SHA256 `32b2ffe32ac988d1ac84b9143ae9dccd97a5f4b7c4b1b543aedd81484aeb8c60`). `dependency-inputs.json` SHA256 `10b0bf32050674d012c45ce6f102d80bc2f87869c2b603e63ab66a8dde13320c` binds311 unchanged dependency inputs/sourcebae0e7a. The exploratory test executable predates later evidence-logging/CSV-proof-only additions to own harness checkpoint804eab1c; it is retained and is not claimed compiled from that later checkpoint. No measurements are accepted from this run.

Source inspected independently by root: `internal/extension/session.go` initial `sessionReader.read` first-byte `file.Read(prefix[:1])` returns its raw error in a frameEvent (around201). Remaining prefix/payload reads call `sessionReadError`; that helper converts `os.ErrDeadlineExceeded` to a structured cancelled error. Startup select around580 may receive the raw OS-deadline event before startup.Done; its event-error branch recognizes typed cancelled only, then emits unavailable_module for everything else. The same initial-reader classification deserves an idle/read-phase control as applicable.

Frozen host-v1 sections2/7 and diagnostic vocabulary require pre-dispatch cancellation as cancelled/do_not_retry. Genuine startup EOF/nonzero exit remains unavailable_module/incompatible_version; a dispatched unresolved mutation remains uncertain_operation with its original actor/key. This correction changes no deadline/budget, response-observation rule, wire field, method/error inventory, actor/session identity or default-native behavior.

## Required correction and regression

Implement the smallest owned host-reader correction and deterministic meaningful regression for actual first-byte OS-deadline handling, before-context-select ordering where possible. Preserve raw clean EOF identity/semantics: blindly normalizing every first-byte error through frameReadError/sessionReadError could turn a successful child EOF into a truncated-frame error. Keep genuine EOF/crash/other I/O failures distinct, and retain partial-prefix/payload behavior, clean shutdown monitor joining, actual Invocation.SessionID, response final-byte/frontier ordering, settled native-refusal priority, uncertainty/cleanup ownership, and4096-byte sanitized messages.

Demonstrate current source or exact behavioral mutant RED for this cause, fixed GREEN; no scaffolding-only red, broad allowed errors, timeout increase, retries-to-green or borrowed deadline. Add direct first-byte deadline versus genuine EOF/partial frame controls; exercise real supplied CLI startup/idle cancellation and normal clean module completion in standard/combined and relevant race variants. Retain causal response-completion/shutdown/refusal regression coverage rather than changing it. Use owned existing session/lifecycle tests; do not edit WP24's harness or unrelated core adapters. No generic SDK, new helper API or framework repair belongs here.

Root serializes one heavy slot: WP24 is finishing its already-running useful ordinary/history rows. Read/edit only until explicit grant; claim/owner dispatch will follow normal workflow, and all build/app/test/probe/gate processes must be waited before release. Build-only lane Git environment or explicit buildvcs=false with exact source maps; never global/test/app GIT_DIR/GIT_WORK_TREE. Bind exact source/test/module/binary identities and preserve every failed artifact. Normal independent review and actual canonical verdict are mandatory before root integrates the corrected session inputs into active WP24. No performance freeze until that integration is independently approved.

All311 non-owner inputs in active WP24 remain approved historical evidence; normal integration must retain later native/quarantine/adapters/changes and every consumer. No contract or ownership transfer is requested: WP16 owns session.go and its existing session/process/lifecycle tests. Ordinary native26row success is not this host correction's approval. Final Mac/whole-source/package/performance/cold-trial gates remain pending. No main/release or beta acceptance change follows.
