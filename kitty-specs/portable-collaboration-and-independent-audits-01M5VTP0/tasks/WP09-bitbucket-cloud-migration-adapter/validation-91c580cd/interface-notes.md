# WP09 Bitbucket Cloud integration handoff

Fixed source: 91c580cda750a77493e933e086d0b535d4e7c724.
Approved dependency base: dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116.
Six owned source files only; no shared interface, registry-core, CLI or legacy-provider edits.

`bitbucket` self-registers through migrationProviders; OpenMigrationSource /
OpenMigrationDestination / MigrationProviderKinds are the production seams for
WP11. Public migration commands belong to WP11 and are not claimed by WP09.
Required nonvacuous source baseline: repository, change_request, review,
inline_comment and workflow. NewSource/NewDestination/Capabilities are offline.

Explicit ProviderConfig contract:
- Locator.Kind = bitbucket; Repository = workspace/slug; Organization/Project empty.
- Endpoint empty means https://api.bitbucket.org/2.0; that Cloud root only in
  production. Explicit loopback HTTP(S) roots support local fixtures. Data Center
  and arbitrary self-hosted roots are refused. Endpoint normalized to /2.0.
- APIVersion must be 2.0; no inferred API profile.
- CredentialMode is anonymous, oauth2, api_token_bearer or api_token_basic.
  Anonymous requires empty Token. Bearer modes require supplied token. Basic's
  opaque transient Token is email:api-token, split on first colon, validated and
  never logged/serialized. ProviderConfig.Token is the frozen json:- field.
  No environment, saved credential, SDK, provider CLI or service lookup occurs.
- Registry TokenEnv = HN_BITBUCKET_TOKEN is metadata for explicit invocation
  selection by WP11; the constructor never reads it.
- WorkflowRef must be an explicit immutable 40/64 lowercase hexadecimal commit.
  Empty selection produces a missing-dependency finding; no branch/default
  revision is silently inferred. Exact bitbucket-pipelines.yml bytes remain inert.
- PlanningSelectors and organization/project conflation are refused.
- Attachments are explicit URL/name/parent Key selections; same API origin/root,
  per-blob and aggregate wire/content budgets apply. Parent must be retrieved.

Source keys scope provider + normalized instance + repository UUID (ProjectID),
with mutable workspace/slug as selectors. SourceID excludes the mutable selector.
Repository metadata uses an inert attachment record keyed repository:<UUID>,
because frozen snapshot v1 has no repository record kind; capabilities explain
this and shared Git companion transports explicitly selected code refs.
PR record IDs are repository-scoped pr:<number>. PR comments are preserved
inline_comment (code-anchored roots) or reply (global roots/replies/tombstones), with PR and parent links, original
inline positions and raw attribution. Deleted comments retain tombstones and an
explicit missing-content finding. Missing parents preserve available reply bytes
and report missing dependency instead of fabricating a relationship target.
Activity has no universal event ID: exact raw content digest + repeated-observation
count forms an observation identity, never an asserted immutable remote event ID.
Approval/update/comment author/date attribution is projected when available.
Raw source blobs retain all unknown fields, authors, status and topology.

Pagination follows opaque same-origin, same-connection next URLs unchanged;
loops, changed connection paths, redirects, missing values, duplicate object IDs,
100-page/10000-row ceilings, 4 MiB JSON and 64 MiB aggregate wire budgets refuse
incomplete success. Selected snapshot budgets are additional ceilings. Final
UUID/creation reobservation refuses a changed repository incarnation.

Removed issue/issue-comment API, standalone discussions and planning boards are
explicit unavailable capabilities; explicit unsupported kind selection refuses
before reads. Default whole-source selection reports unsupported findings while
retrieving available PR/workflow content. Private/deleted/unavailable content is
missing_dependency, distinct from a genuinely empty successful connection.

No Bitbucket remote writes are advertised. Frozen v1 issue/comment payloads
cannot represent a PR/thread parent, repository creation or workflow source
mutation. Plan emits unsupported findings and no intents; Apply/Reconcile refuse
without network or receipt fabrication. Preserve communications to native/local
or portable archives; ComposeMigrationSources and shared Git transport carry code.
The canonical engine requires explicit decisions on unsupported/missing findings.
Fixture integration proves archive and native import from verified source bytes
after provider shutdown, including code refs, secret-free plan/archive and exact
record/blob identity. No new modules, live account mutations, source deletion,
provider sync, workflow execution or authority promotion are introduced.
