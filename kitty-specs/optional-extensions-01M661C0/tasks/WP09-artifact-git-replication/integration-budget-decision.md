# Explicit custom transport budget scope

Root decision with resolved `architect-alphonso` and design-scoped charter,
2026-10-31 UTC.

The frozen `artifact fetch|push --remote NAME --id ID` interface is an explicit
custom-artifact selection. It applies the published custom transfer ceilings:
4096 objects, 2048 signed records, ancestry depth 256, 32 MiB decoded aggregate,
16 MiB equivalent canonical exchange bytes, the 8 MiB individual read-object
ceiling, and the 30-second validation/copy stages. These are tighter than the
existing native replication defaults. Existing quarantine, bounded copying,
cancellation and expected-old promotion mechanics remain in use.

Saved `ReplicationSelection` values select native actors, proposals and memory
streams and contain budgets for that selection. Custom commands neither import
that private selector implicitly nor reinterpret its arbitrary lower limits as
custom settings. Native selection files and enforcement remain unchanged. The
custom command must not claim to honor arbitrary saved native limits; its
documentation states the fixed custom ceilings. No new budget flag, selection
schema or implicit-all custom selection is introduced.

WP09 qualification must retain native lower-budget refusal controls and verify
that using a separate explicit custom selection leaves the saved native
selection unchanged. Numeric stage-boundary controls and full valid transfers
must be distinguished honestly when an earlier, tighter limit binds first.
This is scope clarification for the approved interface, not a relaxed bound or
an ownership/lane/contract change. Independent review adjudicates the actual
live behavior against this decision and the frozen contracts.
