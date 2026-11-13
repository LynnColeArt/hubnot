# Workbox migrations and offline pull request history

Workbox users can manage a migration from the browser or MCP and browse imported pull requests from local Hubnot data. Repositories can use Workbox with or without a GitHub account or remote. Native Hubnot proposals have their own tab and workflow.

## In-app migration workflow

1. Open **Migrations** from repository Settings. Select a migration path, source, destination, and content from the options exposed by the Hubnot installation Workbox uses. Saved local sessions can be reopened from this screen.
2. Generate a plan. Show the selected source and destination, scope, expected effects, and Hubnot findings, including any mapping or preservation decisions the operator must make. Planning leaves the destination collaboration history untouched.
3. Review the plan and explicitly apply it. Workbox enforces Hubnot's permissions and confirmation semantics in both the browser and MCP. Credentials stay outside the saved plan and are omitted from responses and logs.
4. Show progress, completion counts, skipped records, failures, and recovery instructions. Reopening the migration after a restart shows its recorded state and offers resume when Hubnot supports it. A retry must avoid duplicate records and preserve the last successful local history if a later import fails.
5. Open **Pull requests** to see the imported history. GitHub is contacted only when the operator starts or resumes a migration that requires the provider. Browsing and MCP lookups use local data, including when the network is unavailable or the GitHub remote has been removed.

The workflow builds on Hubnot's existing migration planning, inspection, apply, and resume operations.

## Migration path discovery

- The configured Hubnot installation is authoritative for the migration catalog. Workbox discovers all supported source-to-destination paths, operations, configuration inputs, and authentication options it advertises, including paths supplied by custom providers. The browser and MCP use the same catalog and permissions. Discovery works before credentials are entered and without contacting a provider.
- The picker and MCP accept advertised paths and provider-defined inputs without a Workbox update. Show only supported combinations. A refreshed catalog reflects provider installation, removal, upgrades, and changes to the configured Hubnot installation. If discovery is unavailable, show that status and offer a refresh.
- Recheck capability availability when planning, applying, or resuming. If a saved session references a removed or incompatible provider, keep its plan, results, and recovery information inspectable. Explain which capability is missing and what the operator can do to restore it.
- Hubnot's current CLI documents capability discovery for compiled providers. Planning must identify any missing machine-readable metadata for custom paths, inputs, or operations and define the required Hubnot contract. Workbox uses that contract as its source for available paths.

## Pull requests and Issues

- Both tabs default to **Open** and offer **Closed**. The Closed PR view includes merged and closed-unmerged PRs. Each view shows its total when the local data is available and preserves the filter when paging. Opening a different filter starts at page one.
- Use small color-coded badges with readable text. Issues show **Open** or **Closed**. PRs show **Open**, **Merged**, or **Closed**. Colors supplement the labels and meet Workbox contrast requirements.
- Show 10 entries per page by default. A Workbox settings-file value configures the page size for both lists. Missing settings preserve the default. Invalid values produce a clear configuration diagnostic and fall back to 10. Use the same page size in MCP and the browser.
- Sort each filtered list by newest first with a stable tie-breaker. Pagination exposes the current page, total pages, and navigation. Empty lists explain whether the selected state has no entries or the repository has no imported PR history.
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

## Scope

This release covers migration management for every path the configured Hubnot installation exposes, plus offline PR and issue list parity. GitHub PR history is the reference acceptance fixture when that path is available. The preserved source history remains available for future detail views of review threads and inline comments.
