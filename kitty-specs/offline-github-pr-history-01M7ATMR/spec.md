# Workbox migrations and offline pull request history

Workbox users can manage a migration from the browser or MCP and browse imported pull requests from local Hubnot data. Repositories can use Workbox with or without a GitHub account or remote. Native Hubnot proposals have their own tab and workflow.

## Source of truth

Git is authoritative for repository objects, refs, and history. Hubnot is authoritative for collaboration and migration records and their provenance. Workbox and MCP read those records through Hubnot and Git. Any Workbox index is a rebuildable projection of that local state.

Git providers supply source records during explicit migrations. After apply, the imported record in Hubnot is the authoritative copy for Workbox, with its provider attribution and source-reported state preserved. A later provider change becomes visible through another explicit migration. Provider availability, credentials, and remote configuration have no bearing on browsing local history.

## In-app migration workflow

1. Open **Migrations** from repository Settings. Select a migration path, source, destination, and content from the options exposed by the Hubnot installation Workbox uses. Saved local sessions can be reopened from this screen.
2. Generate a plan. Show the selected source and destination, scope, expected effects, and Hubnot findings, including any mapping or preservation decisions the operator must make. Planning leaves the destination collaboration history untouched.
3. Review the plan and explicitly apply it. Workbox enforces Hubnot's permissions and confirmation semantics in both the browser and MCP. Credentials stay outside the saved plan and are omitted from responses and logs.
4. Show progress, completion counts, skipped records, failures, and recovery instructions. Reopening the migration after a restart shows its recorded state and offers resume when Hubnot supports it. A retry must avoid duplicate records and preserve the last successful local history if a later import fails.
5. Open **Pull requests** to see the imported history. A provider is contacted only as part of an explicit migration that requires it. Browsing and MCP lookups use local data, including when the network is unavailable or the GitHub remote has been removed.

The workflow builds on Hubnot's existing migration planning, inspection, apply, and resume operations.

## Migration path discovery

- The configured Hubnot installation is authoritative for the migration catalog. Workbox discovers all supported source-to-destination paths, operations, configuration inputs, and authentication options it advertises, including paths supplied by custom providers. The browser and MCP use the same catalog and permissions. Discovery works before credentials are entered and without contacting a provider.
- The picker and MCP accept advertised paths and provider-defined inputs without a Workbox update. Show only supported combinations. A refreshed catalog reflects provider installation, removal, upgrades, and changes to the configured Hubnot installation. If discovery is unavailable, show that status and offer a refresh.
- Recheck capability availability when planning, applying, or resuming. If a saved session references a removed or incompatible provider, keep its plan, results, and recovery information inspectable. Explain which capability is missing and what the operator can do to restore it.
- Hubnot's current CLI documents capability discovery for compiled providers. Planning must identify any missing machine-readable metadata for custom paths, inputs, or operations and define the required Hubnot contract. Workbox uses that contract as its source for available paths.

## Long lists and grids

- Paginate long collections throughout Workbox. Apply this to pull requests, issues, discussions and their replies, proposals, memories, CI runs, project catalogs, migration history, and activity drill-downs. Inventory existing grids during planning and include any other collections that can grow without a fixed bound.
- Default to 10 entries per page through a Workbox settings-file value shared by browser and MCP. Missing settings use 10. Invalid values fall back to 10 with a configuration diagnostic. Apply filters and sorting before paging. Use a stable tie-breaker so adjacent pages neither repeat nor skip entries in an unchanged collection.
- Keep search, sort, filters, and the current page in the browser URL so links and Back restore context. Changing a filter, search, or sort starts at page one. Show previous and next controls and the current position. Show exact totals and direct page links where the source supports them. Cursor-based views use previous and next with the available counts. MCP returns the same effective page size, selection, and continuation data.
- Keep filters above the list and pagination below it. Use keyboard-accessible controls with visible focus and a clear selected state. Show distinct messages for an empty collection, no matches, and unavailable data. On narrow screens, prioritize titles, status, and navigation.
- Use compact text badges for lifecycle state. Color reinforces the text and meets contrast requirements. Show user-assigned labels alongside status without conflating the two. Use consistent tag styling and allow filtering by existing labels where supported. When space is limited, keep status visible and provide access to the full label list. Browser and MCP expose the same source-backed label values and filter behavior.

## Pull requests and Issues

- Both tabs default to **Open** and offer **Closed**. Sort newest first by default. Issues show **Open** or **Closed**. PRs show **Open**, **Merged**, or **Closed**, with merged and closed-unmerged records both in the Closed view. Use the shared list controls and distinguish no matches from a repository with no imported PR history.
- Preserve the source repository, original PR number, title, author attribution, state, and source link. Identify records by source repository and PR identity so repeated imports update one entry and two sources can each have a PR with the same number.
- Preserve the distinction between a source-reported GitHub merge and a native Hubnot merge. Imported records stay out of proposal lineages and do not produce native review, decision, merge, or pipeline events.
- Browser and MCP use the same local record, access rules, state filters, sort order, and pagination. The existing PR listing tool remains callable and uses local history. Clients can request the Closed view explicitly.

## Acceptance scenarios

1. With no GitHub remote, account, or network, a repository can open Issues and Proposals and can view previously imported PR history in the browser and MCP.
2. When Hubnot advertises GitHub PR import, an operator can preview, review, apply, inspect, and resume it from Workbox. The equivalent MCP operations enforce the same permissions and decisions. Reapplying the same source produces no duplicates.
3. Import a snapshot of Hubnot's 24 historical PRs and disable outbound provider access. Open shows zero. Closed shows 24, with 23 **Merged** badges and one **Closed** badge across pages of 10, 10, and 4. Browser and MCP match original numbers, titles, authors, states, links, ordering, and totals. Native proposals remain independently accessible.
4. With fixtures containing open and closed issues and open, merged, and closed-unmerged PRs, verify the default filter, readable badges, direct links to a filtered page, page-one reset on filter change, and final partial page. Change the settings-file value and verify both surfaces use the new page size.
5. Interrupt an import and resume it from its saved state. A failed import exposes a recoverable status and preserves previously available history. Browsing a repository with no import shows an empty-state link to Migrations.

6. Use two Hubnot installations with different migration catalogs. Add a custom provider with its own inputs and remove a standard provider from one installation. Without rebuilding Workbox, verify that the browser and MCP expose the same supported paths, that the custom path can be planned and applied through its advertised operations, and that a session for the removed provider stays inspectable while apply and resume explain why they are unavailable.

7. After importing history, change a PR on the provider, disconnect the provider, and rebuild the Workbox index from local Hubnot and Git state. Browser and MCP show the last successfully imported record. A subsequent explicit migration updates it while preserving its provenance.

8. Check a representative long list and grid, including a cursor-based discussion thread. Verify settings-file page size, a partial final page, filter reset, Back and deep links, stable ordering, and matching MCP continuation. At narrow widths and with keyboard navigation, verify readable status badges, access to overflow labels, and a visible filter selection.

## Scope

This release covers migration management for every path the configured Hubnot installation exposes, plus offline PR reads and shared pagination, filter, and tag behavior across long Workbox lists and grids. GitHub PR history is the reference acceptance fixture when that path is available. The preserved source history remains available for future detail views of review threads and inline comments.

## Functional requirements

| ID | Requirement |
| --- | --- |
| FR-001 | Repair the audited authority boundaries before migration work. Governed pipeline edits use Hubnot, accepted collaboration history determines visible issues and activity, and active CI inventory uses a committed revision. |
| FR-002 | Discover installed migration capabilities and support planning, applying, inspecting, and resuming advertised paths through Workbox and MCP. |
| FR-003 | Serve imported PR history from local Hubnot records with original source attribution and lifecycle, independent of provider access. |
| FR-004 | Share pagination, filters, tags, and access rules across browser and MCP for long lists and grids. |
