# Clean monitor shutdown must not fabricate cancellation

Root integration finding, 2026-10-31. WP17's real governed six-family raw-stdio
run at the current adapter development source reported host/cancelled, "host
validation cancelled", after valid native work completed. Evidence is retained
under external optional-extensions-WP17/native-focused-second.log. This is not a
reason to retry that suite until green or weaken its success assertion.

Approved WP16 source19cc5d1591d1148f516947730a7355562663d9ef has a matching
lifecycle race: the monitor may already be inside Recheck(sessionCtx) when normal
Run cleanup cancels sessionCtx and joins the reader/monitor. The current CLI
Recheck returns cancelled when that context is cancelled. The monitor can publish
that cleanup-induced error to abort, and a later deferred abort read can replace
an otherwise clean outcome.

Repair only WP16-owned session lifecycle code/tests. Preserve real parent/request
cancellation, revocation, child failures, diagnostics/cleanup errors and any
dispatched mutation uncertainty. Distinguish the host's own intentional shutdown
from a real runtime failure; do not blanket-ignore cancellation or other errors.
No native writer, protocol shape, grant, timeout or cleanup guarantee changes.

First capture a deterministic old-source RED with a monitor Recheck deliberately
in flight when the valid session reaches clean shutdown. Then prove clean success
and meaningful controls for genuine parent cancellation and revocation/failure.
Keep actual current CLI/capability/child cleanup controls in both fresh variants.
Reuse unchanged broader evidence only with exact source proof; no broad unrelated
stress rerun. Bind committed source, binaries and regression evidence explicitly.

Finish the normal gate and independent review. WP17 and WP18 must then normally
resume and integrate the exact newly approved WP16 source, preserving their own
in-progress work, before final qualification/review. No task ownership, dependency,
lane, limit, default workflow or final acceptance requirement is relaxed.
