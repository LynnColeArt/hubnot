# Carry the existing session identity into actual dispatch

Root integration decision, 2026-10-31. Read-only WP19 preparation found that
Invocation carries request/registration/enablement/binding, but not the actual
host-generated SessionID already sent in hello/capabilities. Binding identifies
registration/repository/epoch and can be shared by different sessions. Change
captures must be session-bound; neither a request ID nor that binding substitutes
for session identity.

WP16 owns internal/extension/session.go and its tests. Add the existing actual
SessionID to the internal Invocation value populated by the live Run dispatcher.
It is host-selected state, never a request field, path, caller PID or authority.
No wire field, method, grant, compatibility inventory, nonce generation, deadline,
cleanup, native writer or ordinary default command changes. Existing typed
adapters may ignore the added value; WP19 will bind its transient captured state
to it. Do not invent a process-global substitute for a distinct host session.

Demonstrate through the real Run callback boundary that the value equals that
run's hello/capabilities session identity and differs for a separate session,
including otherwise identical registration/repository/epoch. A metadata-field
presence assertion may demonstrate the old missing handoff without making the
old test fail compilation. Preserve actual request/binding values and strict
rejection of request-supplied overrides; the internal value is not a new input.

Use a minimal committed-source change, focused race and actual supplied CLI
controls with fresh stamped standard/provider-free binaries. Reuse unchanged
lifecycle evidence after exact proof, finish the normal gate and independent
review. No broad stress rerun is requested. Active WP17/18 retain their source
evidence and clean work; root normally refreshes them against the new approved
value-object/dispatch handoff before their final review. This bounded existing
owner addition changes no task ownership, lane or dependency graph.
