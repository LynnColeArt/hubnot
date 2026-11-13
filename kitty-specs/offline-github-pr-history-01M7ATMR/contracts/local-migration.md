# Local migration interfaces

`hn migrate catalog` returns `hn.migration-catalog/2`, including installed adapters, their non-secret input fields and allowed source/destination pairs. The Workbox browser and `workbox_migration` MCP tool both use this catalog and the same migration service.

`plan` produces a session and confirmation. `inspect` reads the durable Hubnot journal. `apply` and `resume` require confirmation for the same saved plan. Session and repository bindings are resolved server-side. The migration engine retains its existing sealed plan and recovery semantics.

`hn migrate pull-requests` reads locally applied imports, and `hn log --visible --json` selects the operative event history for reconciliation. The existing raw log remains available for audit.
