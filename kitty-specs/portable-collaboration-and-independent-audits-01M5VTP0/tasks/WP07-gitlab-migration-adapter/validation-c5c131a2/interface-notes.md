# WP07 GitLab adapter handoff

Source head: c5c131a210ee906779baca1ed1be00c1ab58ced1
Source commits: b2d36c287fd46f2fdeeae2f1d50175b06782f5ff, c5c131a210ee906779baca1ed1be00c1ab58ced1
Dependency base: dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116
Owned source: eight WP07 files only. No shared schema, provider registry, native, module or other-provider source edits.

## Public integration seams

- Self-registers gitlab in migration.Registry and the existing forgeProviders issue transfer seam. Root OpenMigrationSource/OpenMigrationDestination remain the production seam for WP11. New constructors and Capabilities are offline.
- ProviderConfig uses explicit repository numeric ID or nested namespace/path, an API root ending /api/v4, transient supplied Token, optional APIVersion=v4 and CredentialMode=private-token. Default root https://gitlab.com/api/v4. HTTP is allowed only for explicit loopback IP fixtures. Credentials use PRIVATE-TOKEN; no credential discovery, SDK/CLI, extra module or service.
- PlanningSelectors are project-local numeric board IDs, separate from repository project selectors. WorkflowRef pins the .gitlab-ci.yml source through a resolved commit. Attachments are explicit same-origin selections and must name a retrieved parent Key.
- All new source/tests have !noforge. GitLab remains compiled under nogithub. Public migration command wiring remains WP11.

## Identity, preservation and mapping

- SourceID = gitlab:<SHA256 of provider, normalized API instance and immutable global project ID>; mutable path excluded. Record Key.ProjectID is id:<global project ID>. Issues use immutable id:<global issue ID> keys and project-local IID selectors. Note keys use their immutable global ID.
- Source issue state and issue_type are retained verbatim. Original API JSON is retained as digest-addressed blobs for issues, notes, MR conversations/reviews, discussions, boards/lists and project metadata. Confidential/oversized issues become inert attachment records with unsupported findings. Project metadata is an inert attachment, never a board or native permissions.
- System/internal/thread notes remain preserved reply/inline_comment records. Attached discussion and preceding-note links retain topology. MR review/approval state remains inert descriptive JSON, never native approval. Ordinary issue notes alone become closed comment records.
- CI source is exact pinned .gitlab-ci.yml bytes and response metadata. Includes/custom external configs are reported unsupported/unretrieved; no workflow execution or authority is installed. Selected retrieved uploads retain exact bytes and verified blob digests; inaccessible/cross-origin selections yield missing_dependency findings.
- Destination mapping keys: state:<source>, type:<source>, label:<source>, assignee:<source>, target:<source Key.String()>. Empty label/assignee mappings deliberately omit values. Ordinary untyped source issues default to ordinary GitLab issue type; work_item inputs require explicit state/type mappings. GitLab source opened/issue map naturally to GitLab; other source states/types require explicit mappings.
- Destination creates support opened ordinary issues in one mutation; closed creates require an explicit opened mapping. Existing ordinary public issues can update to opened/closed. Labels reject comma/control ambiguity; assignees require explicit numeric user IDs; titles conservatively bounded to 255 bytes. Known capacity/mapping refusals occur in Plan.
- Note updates use explicit target mappings and require an explicitly mapped existing issue parent. Plan observes the note's immutable ID/revision and parent membership. Apply rechecks these before the sole PUT. Creates use confirmed receipt-bound parent IID without a private adapter map.

## Guarded effects/recovery

- gl1.<project baseline SHA256 hex>.<source key SHA256 hex> intent IDs bind normalized locator/selector plus immutable project ID and creation revision within the authenticated plan. Apply/Reconcile strictly parse and reobserve this project binding before effects. ExpectedID/ExpectedRevision retain frozen resource-update meaning.
- Receipts separate immutable RemoteID=id:<global object ID> from RemoteKey (issue IID or note lookup ID). The engine resolves/persists parent remote_key references in bound canonical payloads. Every adapter Apply performs at most one API mutation.
- Recovery requires a unique exact full issue/note projection plus deterministic plan/intent/payload marker. Negative, inaccessible, changed-parent, confidential/system/internal or duplicate observations never authorize another create. Fresh-session recovery uses only engine-bound write/dependency receipts.
- Snapshots share a positive wire budget across every API, pagination, CI and selected upload read, capped at 64 MiB. Additional record/blob/manifest/aggregate limits are enforced. Each response is capped at 4 MiB; selected uploads at 16 MiB. Exact-at-budget EOF succeeds; an extra byte refuses incomplete success. Pagination is bounded to 100 pages and validates sequential same-resource/same-origin links and duplicate identities.

## Fixtures and evidence

- newGitLabMigrationFixture(t), fixture.Config(), fixture.Server are reusable from WP11 root tests. .Issue/.Notes initialize source DTOs; .LoseIssue/.LoseNote/.HideIssue/.HideNotes/.Duplicate simulate recovery conditions. Protect mutation with fixture.Mu when driving a separately built child CLI (the fixture's HTTP handler holds this mutex).
- Fixtures exercise real HTTP+adapter+saved engine sessions, stale/replaced immutable IDs/revisions, lost creates and note updates, negative/ambiguous recovery, source type/state, private/oversized records, MR/system/thread topology, feature-tier/deleted selections, selected uploads, pinned CI, exact budget edges, shared Git companion and archive roundtrip.
- The public legacy hn transfer test imports through the actual built CLI, proves repeat import does not duplicate comments and continues with native issue comments without GitLab writes. No live provider writes or source deletion occur.
- preflight.txt records completed focused race/vet/build/format/diff checks. Final-head full race, exclusion checks and CLI evidence have separate raw logs/status entries. Never infer pass from a still-running log.

## Primary API references

https://docs.gitlab.com/api/issues/
https://docs.gitlab.com/api/notes/
https://docs.gitlab.com/api/discussions/
https://docs.gitlab.com/api/boards/
https://docs.gitlab.com/api/merge_request_approvals/
https://docs.gitlab.com/api/repository_files/
https://docs.gitlab.com/api/project_markdown_uploads/
https://docs.gitlab.com/api/rest/

## Final privacy/execution hardening

- The deprecated GitLab confidential note flag is retained alongside internal. confidential:true is nonordinary even when internal is absent/false. Shared Note.Ordinary is used in source classification, legacy reads, destination existing-note preflight, apply and exact reconciliation; write-response validation also refuses nonordinary notes. Old-v4 fixtures reproduce and protect source preservation, target refusal, and lost-create no-proof recovery.
- GitLab executes quick actions from descriptions/comments under the supplied token. ValidateBody conservatively refuses any line beginning with slash after whitespace, including CR/LF/Unicode line separators, unknown future commands and fenced examples. Plan reports unsupported and retains original source bytes without escaping/truncation; transport repeats the guard for issue create/update and note create/update, including the legacy seam. HTTP counters prove zero writes for refused text.
- Primary references: https://docs.gitlab.com/api/notes/ and https://docs.gitlab.com/user/project/quick_actions/ (automatic execution is documented in the latter's introduction).
- b2d36c28 full race ran once and timed out at the unchanged 10,000-record memory-index test after Go's default 600s package allowance. It is failed/superseded evidence, never a pass. Resource contention was not proved as the cause. Root authorized the finite20m invocation after reviewing the log, changing no assertions/skips/source/CI/config. The c5c131a2 complete full-race result must be read from its separate raw log/status after completion.
- Fresh final-head nogithub/noforge binaries ran init/create/comment/show with Git-only PATH and env -i (no ambient credentials). Fixture HOMEs/repos/private keys and binaries are retained outside the validation directory and excluded from any evidence archive. The initially rejected deletion-containing batch was unexecuted; non-deleting follow-up checks are separately recorded.
