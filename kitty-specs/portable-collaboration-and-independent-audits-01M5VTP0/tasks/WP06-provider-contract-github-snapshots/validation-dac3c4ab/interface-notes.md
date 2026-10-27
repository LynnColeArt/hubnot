# WP06 interface and evidence handoff

Source head: dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116
Dependency base: 247a4e1b26295459ae159674c109bcd91aa1a954
Source commits: 09db7b52e756c8612516264286a26da3708019a0, dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116

## Registry / downstream command integration

- `internal/migration.Registry` supports disjoint self-registration. `ProviderRegistration.Open` constructs inert source/destination values, reads offline capability metadata, and validates the adapter-owned nonvacuous coverage baseline. The shared engine has no provider-name matrix.
- Root `OpenMigrationSource`, `OpenMigrationDestination`, `MigrationProviderKinds` are production integration seams for WP11. Public `hn migrate` command wiring belongs to WP11/12 and is not claimed here. Existing transfers continue through their existing registry.
- `ProviderConfig` supplies explicit Locator, ephemeral Token (`json:"-"`), APIVersion, CredentialMode, WorkflowRef, separate PlanningSelectors, and selected attachment URL/name/parent Key. GitHub accepts REST 2022-11-28 and supplied bearer tokens; no environment/CLI/service discovery occurs. Future provider files use their own API/version/credential policies.
- Frozen Source, Destination, Locator, Selection, ApplyContext, snapshot, plan, bound-write, receipt and reconciliation schemas were not changed.
- Existing ComposeMigrationSources / ComposeMigrationDestinations and GitMigrationDestination transport selected code refs. GitHub does not copy code independently. A local Git remote + real GitHub fixture destination test proves composition.

## GitHub observations / mappings

- SourceID scopes immutable numeric repository ID by provider and normalized instance, without mutable OWNER/NAME. Record keys keep Provider, Instance, immutable ProjectID, Kind and immutable ID separate from mutable selectors.
- PRs/reviews/inline comments retain raw JSON and explicit PR/review/reply links. General PR conversations are preservation replies, never fabricated ordinary issue parents.
- Discussions/replies and explicitly selected planning projects/items use existing bounded typed API readers. Planning selectors are separate from repositories. Workflow definitions retain a pinned commit and exact YAML bytes; all foreign data remains inert.
- Explicit same-origin attachments are downloaded with bounded bytes and digest descriptors. External upload hosts are reported unavailable rather than receiving API credentials. Unauthorized selected blobs are not called preserved. Oversized issue text falls back to an attributed inert source-blob record with an unsupported finding, without truncation.
- Source issue type names are observed; null/absent type stays empty. Issue create/update mapping supports open/untyped destination issues. Source closed states/types require explicit state/type mappings. Work-item inputs require explicit type and state mappings before producing issue intents. Comments can depend on either source issue or work_item parents.
- Mapping keys: `state:<source value>`, `type:<source value>`, `label:<source value>`, `assignee:<source value>`, and `target:<source Key.String()>` for an explicit existing issue number. Empty label/assignee mappings deliberately omit those values. Type administration is unsupported; existing typed destinations are reported unsupported and unexpected default types cannot be confirmed as untyped success.
- Source authors/times remain snapshot provenance rather than impersonated remote authors. Mapped findings disclose deterministic recovery-marker augmentation of API bodies. Plan checks marker capacity before effects and does not truncate.
- Retrieved unsupported records/blobs remain inspectable in the saved snapshot and are reported separately from recreated issue/comment fields. Any unsupported/missing/mapping finding needs the canonical engine's explicit decision.

## Baselines / recovery

- GitHub intent ID encoding is `gh1.<64 lowercase repository-baseline SHA256 hex>.<64 source-key SHA256 hex>`. The baseline binds normalized locator/origin and immutable repository numeric ID, node ID and creation revision. Strict parsing and current repository re-observation precede each mutation/reconciliation. Resource ExpectedID / ExpectedRevision retain their frozen meaning for updates.
- Receipts use immutable `id:<numeric object ID>` separately from RemoteKey (issue number or comment lookup ID). Comment parent templates reference the confirmed predecessor's `remote_key`; the engine resolves and persists exact canonical payload/digest. The adapter has no private parent map.
- Each Apply performs at most one mutation. Full title/body/state/label/assignee/type projection is checked. Uncertain creates reconcile only from a unique exact bound marker and full intended projection; missing or multiple matches do not confirm and never authorize a duplicate create.
- One read client shares the 64 MiB maximum wire budget across REST, GraphQL, workflow and attachment reads, in addition to plan/API/record/blob/page limits. Exact-at-budget EOF is accepted; one extra byte is refused.

## Build / evidence scope

- GitHub-only root files, client files and corresponding tests require `!nogithub && !noforge`; disabled GitHub help is available for either exclusion. `nogithub` keeps its GitHub-only meaning. `noforge` keeps native/local/archive/audit code and the registry available while excluding this optional adapter/client.
- No module dependencies, mandatory provider CLIs, Python/service requirements, live provider writes, source deletion, global sync, workflow activation, public merges/pushes/tags/releases were introduced.
- `source-sha256.txt` hashes all 28 owned changed files at the fixed source head. `status.txt` and individual command logs record actual validation results. Initial 09db evidence at `/tmp/hubnot-wp06-09db7b52-validation-Op1YZf` is partial/superseded and must not substitute for final-head checks.
