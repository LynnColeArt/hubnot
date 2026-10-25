# Research and decisions

Existing issue_commands.go supplies complete detailed pages, opening_metadata, guarded revise, actor pinning and operation lookup/replay. Reuse these contracts without changing signed schemas. User confirmed a parallel gh-style implementation within Hubnot, and integration/CLI scope before policy/automerge.

Primary provider references checked 2026-10-24:
- https://cli.github.com/manual/gh_issue_list (flags and selected JSON fields)
- https://cli.github.com/manual/gh_api and https://cli.github.com/manual/gh_help_environment (unmodified gh would require API compatibility; not the selected architecture)
- https://docs.github.com/en/rest/issues/issues (issue APIs also return PRs; filter pull_request)
- https://docs.github.com/en/rest/issues/comments (paginated comment APIs)

No external Go module or gh subprocess is necessary. The optional transport-excluded build proves the package boundary. Native signed provenance records what the importer observed rather than pretending source authors signed native events.
