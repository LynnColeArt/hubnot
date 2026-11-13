# Workbox and Hubnot handoff for Mini

Workbox main contains separate Pull requests and Proposals tabs. Chrome and MCP matched 24 Hubnot GitHub PRs when checked, including 23 merged and one closed. The current PR list reads GitHub live. The new spec makes that history an explicit Hubnot migration and makes browsing local and offline.

Git owns objects, refs, and repository history. Hubnot owns collaboration, migration provenance, admission, and policy evaluation. Workbox and MCP present those facts through shared services. Workbox's database can hold local preferences and rebuildable projections.

The audit identified direct pipeline commits in the CI tab, legacy MCP issue reads from SQLite, missing accepted remote actor refs in index discovery, dashboard activity marked verified regardless of evidence, and CI inventory reading dirty checkout files. Fix these before expanding migration features.

Repository Settings will expose migration paths advertised by the installed Hubnot. Custom providers can appear and standard providers can disappear. Saved plans remain inspectable. Imported PRs keep their original number, author, source and merge state. Native proposals retain their own workflow.

Long lists share a configurable default of 10 records per page, accessible tags and filters, and browser/MCP parity. Open and Closed are the primary PR and issue views. Closed includes merged PRs with their own label.

The specification is in spec.md. Audit findings were traced in source. Existing focused tests passed, and each repair needs its own verification before release.
