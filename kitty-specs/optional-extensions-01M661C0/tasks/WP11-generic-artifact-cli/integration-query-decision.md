# Query cursor and page presentation

Root architecture/contract interpretation, 2026-10-31 UTC. Existing ownership and
signed contracts remain unchanged.

The frozen contract makes cursor contents opaque and bounded to 4096 bytes; it
does not prescribe a public encoding. WP11 may choose a private stateless encoding
bound to the actual type/shape selection, exact verified source cut and ordered
offset. A read must not create an identity, registry or private cursor store.
The token never supplies grants or authority: validate the actual selection,
cut and offset against the current verified view before producing output.

Do not bind the cursor to page size. The caller may change a valid 1..200 limit
while continuing the same selection, cut and offset. This permits a smaller page
after an oversized response. The CLI requires its explicit limit; absence of an
initial cursor selects the first page. Malformed, cross-selection, out-of-range
or stale cursors use the frozen `host/checkpoint_invalidated` error with
`rescan_required`, as specified by host-v1 section 6's cursor/checkpoint/view row.
Malformed ordinary flags, type/shape selectors or limits remain
`artifact/invalid_input`; an oversized cursor uses `resource_limit` without retry.
No new artifact or native error code is introduced.

The new list result is explicitly `Page(T)`. Host-v1 section 4 pins both its
1..200 item limit and its 8 MiB total encoded-response bound. Enforce the latter
before writing output, report `resource_limit` with smaller-page guidance, and
never silently truncate. Root initially applied the standalone-reader distinction
too broadly to list output and corrected that interpretation before implementation.
This page-output limit does not change the already approved full-history/inspect
CLI behavior or the shared reader's 100000-entry, 8 MiB-object, 256 MiB aggregate
and stage bounds. WP18 still owns host framing and host pagination.

The existing artifact storage renderer predates the new checkpoint condition.
WP11's owned query command file may render that single new frozen error/recovery
presentation and delegate ordinary success/error handling to the existing
renderer. It must not duplicate a parser, writer or general-purpose rendering
system, edit earlier owned files, or add an unused future API.

Qualification must show ordered coverage with changing page sizes, unchanged
source cuts, stale/malformed/cross-filter cursor refusal without state effects,
the exact page-output boundary, and literal closed-code/presentation agreement.
Independent review adjudicates these outcomes against the frozen contracts.
