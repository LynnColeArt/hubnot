# Implementation status

The first audit commit replaces the CI direct write with a Settings link, reads committed pipeline and policy files, routes legacy issue reads through native state, enumerates replicated actor hints, and excludes unverified activity from the feed. The broader reconciliation of existing index rows against admission changes, ref changes and memory state remains open.

Hubnot now provides a versioned provider-kind catalog, a local archived GitHub PR projection, and migration checkpoint counts. Workbox uses these through repository Settings and MCP. The first migration UI supports imports into the current local repository. It uses the installed provider kinds and the existing CLI's common source fields. Full source/destination path discovery, provider-specific typed fields, and coverage of custom provider execution remain open. The plan's full contract remains the target.

Workbox replaced live GitHub PR reads with local imports. An explicit Hubnot import of all 24 Hubnot PRs into a disposable repository produced 23 merged and one closed. Chrome and MCP matched page sizes of 10, 10 and 4 with external browser requests blocked. Workbox also planned and applied the same import through the shared migration service and refused a mismatched confirmation.

The page size is shared by PRs, Issues, Discussions, Proposals, Explore and repository activity. The setting is `pageSize` in `workbox.settings.json` or the file selected by `WORKBOX_SETTINGS_PATH`, with a range of 1 through 50. Workbox's legacy issue tool continues returning its established list shape. Full pagination coverage, shared label and status treatment across all surfaces, provider removal, crash/resume UI tests, and release qualification remain open.

These commits stay on feature branches while those acceptance criteria are open. The current localhost installation is the previously released build.
