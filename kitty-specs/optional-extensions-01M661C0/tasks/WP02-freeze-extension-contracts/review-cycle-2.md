---
affected_files: []
cycle_number: 2
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T12:24:50Z'
reviewer_agent: codex
wp_id: WP02
---

# Bounded response-completion observation rule

2026-10-31 — root, architect-alphonso. Adopted design direction, subject to the normal WP02 contract-owner update and fresh independent review before WP16 implementation.

The deterministic WP16 reproduction proves that local completion-channel publication can lag successful response writing and a conforming peer's next frame. Stdio supplies no peer read acknowledgement. Neither a flag after `Write`, a scheduling yield, nor a userspace timestamp can perfectly distinguish physical peer consumption from a premature send during a final write. The independent read-only advisory is retained at `optional-extensions-response-boundary-advisory/advisory.md`. It also identifies the analogous complete-response/clean-exit EOF race. This does not establish the cause or outcome of WP21's first, automatically removed fixture.

Keep the client's obligation to consume and validate the complete correlated response before sending another business request. Clarify the host's observable enforcement explicitly:

- Encode/validate one immutable bounded response. Settle its prefix and all but its final payload byte, then attempt that final byte separately within the same absolute deadline. The tail marker is a host observation immediately before that attempt, not proof of syscall entry, peer receipt, or a bounded time window.
- Preserve the first host-observed prefix time/phase and relevant response frontier across queued events, partial frames and later loop iterations. A frame recorded before the tail remains premature even if decoded/dequeued later. Do not claim userspace observations reveal actual peer send/read order.
- In the final-byte attempt/settlement interval, retain at most one syntactically valid business candidate. No admission, grant decision, operation reservation, signer/journal effect or second dispatch/writer occurs until the preceding response writer is successfully joined. A second candidate is a protocol failure, never another queued request. Preserve finite existing reader/channel/in-progress-payload bounds; state their total encoded-buffer envelope without claiming a process-heap cap.
- Keep exact active-request cancel, wrong/malformed controls, revocation, overflow, parent cancellation, EOF and I/O errors live while a candidate is held. Pin completion/control precedence from recorded observations rather than nondeterministic select readiness. Preserve original first-prefix time, partial-frame cap and all remaining deadlines; retention cannot extend a request budget.
- A tail-observed EOF can represent normal completion only after successful joined response settlement, verified child exit0 and no unfinished candidate. Earlier/failed writes, nonzero exit and unfinished requests remain explicit failures, with no candidate dispatch. Preserve existing native outcome and original mutation-identity classifications; transport success is not durable consumer acknowledgement.

This deliberately tolerates an otherwise indistinguishable premature peer in the tail observation interval. It preserves serial admitted operations and does not promise detection of every physically premature send. Do not broaden the allowance to the full bulk-response write or add sleeps, retries, a peer-ack field, new protocol methods, authority, numeric frame/deadline budgets, an unbounded queue or a default dependency.

WP02 must update the affected host-v1 sections/table and matching conformance prose, preserving all existing literal wire fields, signed fixtures and inventories. Fresh independent contract review precedes WP16's production change. WP16 then needs deterministic completion-gap and clean-EOF regressions, queued/partial pre-tail rejection, actual blocked-response second/cancel/timeout controls, candidate/control/error/deadline bounds and one settled writer. The adjacent completed-native-refusal response-loss priority belongs to the same repair because section 3 already requires retaining a settled definitive refusal; genuine successful-write response loss must retain uncertainty and the original key.

No file owner, lane, dependency graph, signed data, default native behavior, beta acceptance, main integration or release changes. After contract approval, refresh planning/analysis through the canonical tools without resetting historical approvals; root integrates the approved contract and later approved host repair into the current reference lanes before their final acceptance.
