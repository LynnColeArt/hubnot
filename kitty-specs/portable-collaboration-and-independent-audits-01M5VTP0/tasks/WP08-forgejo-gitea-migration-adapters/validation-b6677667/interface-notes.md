# WP08 interface handoff

Source: b6677667c666148fc6cbe47d29d02a52b8bf0b5d
Approved dependency: dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116

## Profiles and integration

- Separate `forgejo` and `gitea` migration and legacy transfer registrations consume the frozen registry/Source/Destination/Locator/ApplyContext/coverage values. No shared schema, registry, module or other-provider file changed.
- Forgejo API selections `16` (default; public Codeberg Swagger qualification) and `11` (versioned v11.0.11 schema); Gitea selections `1.27` (default; current official 1.27.3 docs, versioned v1.27.0 schema) and `1.24` (v1.24.7 schema). Other families are refused. Actual server version is read during snapshot/plan and before apply/reconcile. Constructors and capabilities are offline.
- Explicit roots end in `/api/v1`, including installations at subpaths. Forgejo defaults to `https://codeberg.org/api/v1`; Gitea requires its own explicit instance. Supplied token authentication uses `Authorization: token`; no CLI/SDK, environment discovery, impersonation, Sudo or token persistence.
- The common transport in owned `internal/forgejoapi/client.go` covers only compared endpoints. Gitea owns a distinct construction/profile facade; it never assumes Forgejo version equivalence. Gitea uses `.gitea/workflows`; Forgejo observes `.forgejo/workflows` and `.gitea/workflows`. Review JSON preserves differing original/version-specific fields without reinterpretation.
- `OpenMigrationSource`, `OpenMigrationDestination` and the legacy `hn transfer --provider forgejo|gitea` are real production callers. Public `hn migrate` CLI remains WP11. Selected code refs use existing `ComposeMigrationSources`, `ComposeMigrationDestinations` and `GitMigrationDestination`, with real Git fixture acceptance.

## Source and coverage

- Stable SourceID binds provider + normalized API instance + immutable numeric repository ID, independently of mutable owner/name. Keys also retain kind and immutable object ID; issue numbers remain selectors. Snapshot rechecks repository ID/creation identity.
- Actual issue and comment read/create/update coverage for both profiles. Every retrieved ordinary issue/comment also retains exact source JSON as a digest-addressed blob. Unsupported larger issue content falls back to a preserved source blob with an explicit finding, without truncation.
- PRs use the full pull endpoint and its immutable ID; review and inline records retain explicit PR/review links plus full source JSON, diff anchors, commits, dismissal/resolver fields. General PR conversations are preserved replies, never ordinary issue comments.
- Explicitly selected same-origin asset bytes are bounded and digest-addressed. Inaccessible/foreign-origin assets are reported missing, without fabricated blobs. No attachment upload is implemented; destination planning discloses original blobs and unsupported recreation.
- Workflow resolution first obtains an immutable Git commit, then reads directory/file content at that commit. Exact YAML and raw content metadata remain inert source blobs. Symlinks, unsafe entries, unpaired Unicode, duplicate JSON keys and content/size mismatches refuse.
- No qualified project/board or standalone discussion export endpoints exist in these pinned profiles. Gitea 1.27 issue project-ID fields do not establish board export coverage. Capabilities/findings report this honestly; no UI-based native project claim.
- Imported users/reviews/CI/workflows carry no native signing, approval, runner or maintainer authority. Offline local import/inspection and native continuation are exercised.

## Mapping and guarded effects

- Mapping keys match WP06: `state:<source>`, `type:<source>`, `label:<source>`, `assignee:<source>`, `target:<source Key.String()>`. Types map explicitly to empty. Work-item conversion requires explicit source type/state mappings. Empty label/assignee mappings deliberately omit values.
- Issue creates support `open`/`closed` using a single POST with `closed` boolean. Mapped labels are existing canonical positive numeric label IDs, checked during plan and rechecked before create. Assignee values remain explicitly supplied target logins.
- Issue PATCH cannot change labels in one mutation; plan refuses different label sets. Gitea 1.27 updates require an observed positive `content_version` and send it to the API, in addition to immutable identity/full observed revision checks.
- Issue and comment API bodies append a disclosed deterministic HTML recovery marker. Planning checks conservative qualified title/body ceilings (255-byte title, 65536-byte body including marker) before effects; exact equality succeeds. No silent truncation.
- Comment creates/updates reference a mapped issue intent through the frozen receipt template. The engine persists canonical actual payload/digest from confirmed receipts; the adapter has no hidden parent-ID map. Existing comment updates also bind ID/revision and enumerate only under the immutable confirmed parent.
- Intent IDs `fv1.<64 repository-binding hex>.<64 source-key hex>` bind provider, normalized locator, selected compatibility profile, immutable repository ID and creation identity. Strict parsing/current re-observation precede each effect or reconciliation.
- Each Apply issues at most one mutation. Full intended title/body/state/label/assignee projection (or comment body) must match. A successful response with missing mapped fields remains uncertain.
- Unique exact marker + complete projection is required for recovery. Negative, inaccessible, inexact, ambiguous and replaced-parent observations never authorize a duplicate create. Fresh-session lost issue/comment tests inspect persisted parent payload and immutable ID versus local selector receipts.

## Budgets and build boundary

- Bounded 100-page listings, configurable 1..50 page size, strict next-origin/path/query/sequence continuity, duplicate immutable IDs and duplicate issue indexes, stable totals when supplied, and empty sentinel completion when instance pagination caps omit links/totals.
- Per-response 4 MiB and operation-wide 64 MiB ceilings (snapshot, plan, apply, reconcile), in addition to positive canonical record/blob/manifest/aggregate budgets. Exact-at-budget EOF succeeds; one extra byte refuses incomplete success. All redirects are refused and no foreign origin receives API credentials.
- All twelve network registration/client/test files use `!noforge`. `nogithub` retains both profiles; `noforge` excludes clients, root files, symbols and registrations. Native/local/archive/audit behavior stays available.
- No live account writes, new modules, provider CLI/SDK/Python/service requirements, public pushes/merges/tags/releases, source deletion or other WP lifecycle changes.

## Primary API evidence

- https://codeberg.org/forgejo/forgejo/raw/tag/v11.0.11/templates/swagger/v1_json.tmpl
- https://codeberg.org/swagger.v1.json (retrieved public version: 16.0.0-dev-785-465b60ef+gitea-1.22.0)
- https://raw.githubusercontent.com/go-gitea/gitea/v1.24.7/templates/swagger/v1_json.tmpl
- https://raw.githubusercontent.com/go-gitea/gitea/v1.27.0/templates/swagger/v1_json.tmpl
- https://docs.gitea.com/api/1.24/
- https://docs.gitea.com/api/1.27/
- https://docs.gitea.com/1.24/development/api-usage/

Qualification is source-schema review plus local HTTP/Git fixtures, not a claim of live account mutation or remote instance runtime qualification. Full fixed-head gate results are recorded separately in status.txt and raw logs.
