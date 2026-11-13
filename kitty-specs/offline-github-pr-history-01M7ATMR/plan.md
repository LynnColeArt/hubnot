# Implementation plan

## Delivery order

| Stage | Owner | Completion evidence |
| --- | --- | --- |
| 1. Audit repairs | Workbox | CI writes use the reviewed Settings flow. CI configuration is pinned to a committed revision. Legacy MCP issue tools, feed, memory, and index use Hubnot acceptance and visibility. Tests cover remote refs, rejected records, dirty checkouts, rebuilds, and browser/MCP parity. |
| 2. Migration catalog and operations | Hubnot, then Workbox | Versioned catalog exposes actual paths and typed inputs. Browser and MCP can plan, inspect, apply, and resume through the same service. Two installations demonstrate custom additions and provider removal. |
| 3. Offline PR history | Hubnot, then Workbox | Native local history read exposes preserved change requests with provenance. A 24-record import remains identical across browser and MCP with provider egress disabled. |
| 4. Shared lists | Workbox | Inventory long lists and grids. Apply shared settings, URL navigation, filters, labels, and status badges. Verify numeric pages and cursor continuation. |

## Authority and audit repairs

Use Git object reads at the resolved default revision for active pipeline and policy inventory. Replace CI direct commits with the Settings proposal flow. Old write entry points return the destination for the supported workflow. Verify a dirty checkout with unrelated staged changes stays untouched.

Route legacy issue list and detail tools through the native issue service. Preserve the old response fields and resolve number aliases before fetching authoritative state. Treat indexed event IDs as discovery hints. Use Hubnot selected history for visibility, including accepted remote refs, and rebuild or invalidate rows when ref or admission state changes. Rebuild in a transaction and preserve stable presentation aliases during upgrades. Feed and memory must carry explicit coverage and trust status. Verify an admitted replicated event appears and an unverified or refused record stays out of operative reads.

## Migration contract and workflow

The current CLI has capabilities, plan, inspect, imports, apply, and resume. Capabilities output includes human prose and lacks the full typed path and field catalog needed by a generic client. Add a versioned Hubnot machine interface backed by its provider registry. Advertise path IDs, versions, operations, record kinds, typed required and optional inputs, enum choices, credential requirements, and supported effects. Expose an installed custom provider through that same registry contract. Workbox renders known field types and reports unsupported schema versions explicitly.

Hubnot owns sealed plans, execution and recovery journals, and durable import receipts. Workbox owns the UI and shared MCP operation service. Bind opaque session IDs to the selected repository and authorized operator. Resolve session paths on the server. Keep credential values out of command arguments, logs, and durable session metadata. Use explicit confirmation bound to the reviewed plan, revalidate installed capabilities before apply and resume, and persist operation state so Workbox can reconnect after restart. Inspect saved sessions even if a provider disappears.

## Offline records

Use Hubnot's existing preserved change-request records and native import inspection as the starting point. Expose an offline, versioned projection of applied imports with source identity, original number, title, author, lifecycle, source link, and revision provenance. Retain the last applied version on partial failure. Repeated imports converge on source identity and record identity. Native proposals continue using their existing signed workflow.

Browser and MCP use the same local listing function. Keep the current PR tool callable and move it to local reads at rollout. Present missing imports and unavailable local data distinctly. Verify index deletion and rebuild without provider access, an upstream-only change, and an explicit reimport.

## List behavior and rollout

Use one Workbox setting for a default of 10 items. Apply filter and sort before page selection and preserve browser URL state and MCP continuation. Keep cursor-backed services on cursor navigation where exact totals are unavailable. Inventory Issues, Pull requests, Proposals, Discussions and replies, Memories, CI, Explore, Migrations and activity drill-downs. Reuse accessible status and label components. Validate narrow layout, keyboard navigation, filter reset, and end-of-list behavior.

Land audit repairs first. Land compatible Hubnot contracts before the Workbox consumers. Ship an honest unavailable state for older Hubnot binaries. Qualify offline migration with a disposable repository, then exercise the local browser and MCP together. Full OS qualification follows integration.

## Decisions

The specification and cross-repository plan live with the existing Hubnot mission. Workbox changes use a dedicated feature branch. The audit stage is independently reviewable. GitHub is the reference fixture. Installed Hubnot capabilities define migration coverage.
