# WP10 Azure interface handoff

Source head: `29771aed34e68503c6a4c3f660cbb56f2a335ab0`.
Approved dependency base: `dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116` (WP06).
Source commits: `a7b0d620aa839e953fa186683e8fe5a5e7ae6445`, `98dd4e94aff274850a56c56cb7dd8b4b9f469a26`, `29771aed34e68503c6a4c3f660cbb56f2a335ab0`.

## Configuration and registry

Provider kind is `azure`. Constructors and capabilities are offline; no credential/account discovery or OAuth registration occurs. Root `OpenMigrationSource` / `OpenMigrationDestination` use the existing registration seam. Public migration CLI wiring remains WP11.

`Locator.Endpoint` is an explicit normalized service/server root. `Organization` is an explicit organization/collection segment; `Project` is the separate project selector. `Repository` is optional for Boards/Work Item-only operations, and required for code refs, PR conversations and selected repository YAML. Examples: Services root `https://dev.azure.com`, Organization `example`; Server root `https://devops.example.invalid/tfs`, Organization `DefaultCollection`.

`APIVersion` must be `services-7.1` or `server-7.0`; corresponding Comments versions are `7.1-preview.4` and `7.0-preview.3`. Server qualification is Server 2022's REST 7.0 profile. `CredentialMode` must be `bearer` (supplied Entra bearer token) or `pat` (supplied PAT via Basic auth). `ProviderConfig.Token` remains transient (`json:"-"`); Azure registers no token environment variable. Credential refresh does not alter authenticated plans.

Legacy `hn transfer capabilities --provider azure` explains its pre-network refusal: that issue-only protocol cannot express explicit API/auth/process/type/state inputs. It does not guess open/closed mappings. Actual mapped Work Item/comment reads and writes use the nonvacuous migration registry.

## Mapping and observations

Required destination mapping: `process=<observed capabilities.processTemplate.templateTypeId GUID>`. Every issue/work_item input additionally needs `type:<source type>=<destination type>` and `state:<source state>=<destination state>`. The mapped type/state vocabulary is read and checked before creating intents; absent/process-invalid mappings return findings. Existing target mapping uses `target:<source Key.String()>=<Work Item ID>`. Changing a target's Work Item type is unsupported.

Supported destination fields are title, description, existing mapped type and state. Source labels/assignees require explicit empty omission mappings (`label:<value>=`, `assignee:<value>=`); arbitrary custom field or native attachment recreation is unsupported. Original custom fields, relations, attachment descriptors and attribution remain in retrieved raw blobs. Title/body plus exact recovery-marker capacity is checked during planning; no truncation or bypassRules is used.

Project/process metadata preserves the complete retrieved project JSON, including description/visibility. Work Items retain canonical IssueContent plus original raw JSON; IDs and revisions stay distinct. Comments preserve their original raw JSON/attribution; deleted or oversized comments remain inert preserved source rather than recreated comments. Unavailable collections/blobs and unknown resource kinds produce explicit findings rather than empty preservation claims.

PR lists are exhaustively bounded; each selected PR detail is read because Azure documents description truncation in list responses. PR detail, thread status/properties, diff/iteration/file context, authors and parent-comment topology remain inert attributed data. Threads and their comments have explicit PR/thread/reply links. Inaccessible or missing relationship targets are reported, with original references retained in source blobs.

Workflow selectors use `PlanningSelectors`: `yaml:/path` requires explicit repository plus exact existing SHA1 `WorkflowRef`; `definition:ID@revision` selects a classic build definition. YAML requests raw octet-stream content. Classic top/group secret-marked variable values and documented data-source authorization-header values are excluded, while names, endpoint references and nonsecret source remain inspectable. Explicit exclusion findings require decisions; malformed/duplicate/case-aliased credential declarations refuse. Blob and revision digests bind sanitized bytes only; changing excluded original values leaves the canonical snapshot unchanged. Selected committed YAML/Git is not secret-scrubbed. No source executes or adopts queues, permissions, secrets or native policy. Board configuration/backlog/iteration administration and other unqueried Azure resources are not claimed preserved.

Attachments require explicit selections and retrieved parents. Same-origin selected bytes are bounded and content-addressed; external origins and redirects receive no credentials. An attachment URL alone is only a descriptor. Download authorization failures are missing data, never preserved bytes. Destination attachment/custom-field uploads are reported unsupported before effects.

## Identity, engine and recovery

SourceID binds provider, normalized instance/organization, immutable project GUID and selected immutable repository GUID. Two repositories in one project have distinct code-observation namespaces. Key.ProjectID remains the actual project GUID; repository IDs appear in repository-scoped PR/thread keys and SourceID rather than replacing project identity.

Intent format is `az1.<baseline SHA256>.<type vocabulary SHA256>.<source key SHA256>`. The authenticated baseline binds locator, selected Services/Server API profile, immutable project/repository IDs and process GUID. Every Apply/Reconcile re-observes them; Work Item operations also re-observe mapped type/state vocabulary. Updates separately bind actual immutable Work Item identity and revision, then use JSON Patch `/rev` and type tests. No provider-name branch or schema expansion was introduced in the shared engine.

Work Item receipts use `RemoteID=work_item:<project GUID>:<numeric ID>`, `RemoteKey=<numeric ID>` and a separate revision. Comment templates reference a confirmed predecessor receipt's `remote_key`. The engine resolves and persists the exact parent payload/digest before effects, and fresh adapters resume those exact bytes. There is no private mutable parent map.

Each Apply performs at most one mutation. Recovery requires one unique exact bound marker plus the complete supported intended projection. Absent, ambiguous, inaccessible or changed observations never authorize another create. Replaced project/repository/process/API profile/type vocabulary, stale revision and changed parent-project identity refuse before effects.

WIQL uses an increasing immutable-ID keyset and pinned ASOF observation, detail batches <=200, explicit completeness/count checks and full-page exhaustion probes. Comments use bounded continuation tokens, detect duplicates/cycles/conflicting/invalid next-page identities and compare complete totals. The qualified thread endpoint documents no continuation parameter; unexpected thread cursors refuse incomplete success. All snapshot/plan/apply/reconcile operations share bounded operation read clients (64 MiB ceiling), with 4 MiB API response and additional plan record/blob/byte budgets. Exact supported budgets succeed; excess refuses.

Selected code uses the existing Git companion; no Azure-specific Git copier was added. Real local Git + API fixture migration exercises this composition and verifies archive blob bytes without tokens.

## Shared ownership exception

Root authorized only `internal/migration/plan.go` and `plan_test.go` via primary `shared-plan-scope-handoff.md` (coordination commit `1dd7f7e0`). CanonicalPlan now accepts a nonempty Project OR Repository, validates both independently and rejects missing/organization-only scope. No JSON, interface, receipt, Source/Destination/ApplyContext or write schema changed. Tests exercise project-only/repository-only scope, invalid selectors and actual Boards-only PrepareMigration+Apply without a repository sentinel. Initial commit guard warned about both shared paths but did not reject; the explicit ownership handoff is retained for review.

## Validation provenance and limits

The preliminary `a7b0d620` ordinary full race command reached its default 600-second cumulative package timeout while running existing `TestPolicyEvaluationUsesExactBaseAcrossAmendment` (1s), not an Azure failure. Complete raw output is retained separately. That source also preceded the two preservation corrections and is superseded; its partial checks do not substitute for final-head evidence.

The subsequent `98dd4e94` source passed its complete ordinary race/vet/build/tagged/native/legacy/dependency sequence, but preceded the grouped-variable/header correction and is also superseded. Final-head evidence is exclusively `29771aed`.

Final-head full validation uses the root-authorized finite 20-minute deadline for concurrent host workload, with the entire ordinary race suite followed by vet/build, applicable nogithub/noforge migration/native/audit checks, three offline native CLI variants, legacy capability checks and actual dependency/file/symbol exclusions. See raw status for executed outcomes. No assertion, test configuration, skip, mandatory module/service/CLI, live provider write, source deletion, workflow execution, public merge/push/tag/release or unrelated automation change was introduced.

## Primary API references

- [REST versioning / Server compatibility](https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rest-api-versioning?view=azure-devops)
- [Project metadata](https://learn.microsoft.com/en-us/rest/api/azure/devops/core/projects/get?view=azure-devops-rest-7.1)
- [Work Item create / JSON Patch](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/create?view=azure-devops-rest-7.1)
- [Work Item detail batches](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/list?view=azure-devops-rest-7.1)
- [WIQL / ASOF](https://learn.microsoft.com/en-us/azure/devops/boards/queries/wiql-syntax?view=azure-devops)
- [Comment pagination](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/get-comments?view=azure-devops-rest-7.1)
- [Server comment version](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/add?view=azure-devops-rest-7.0)
- [PR list limitations](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-requests?view=azure-devops-rest-7.1)
- [PR threads](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-threads/list?view=azure-devops-rest-7.1)
- [Git items / selected file](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/items/get?view=azure-devops-rest-7.1)
- [Microsoft content negotiation implementation](https://github.com/microsoft/azure-devops-python-api/blob/dev/azure-devops/azure/devops/v7_1/git/git_client_base.py)
- [Classic definition revision](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/definitions/get?view=azure-devops-rest-7.1)

## Final lifecycle and verification outcome

At unchanged final head `29771aed`, the first full race failed the unchanged 1-second memory-query p95 contract (1.077407267s). The one coordinator-authorized serialized unfiltered retry passed (`hubnot 600.198s`), with source hashes verified before/after. All distinct static, tagged, native CLI, legacy capability, dependency/file/symbol, formatting/diff and tag checks passed. No performance threshold or test configuration was changed, and the failure cause is not claimed proven. Both attempt logs are retained.

Canonical T028–T030 are done and WP10 is `for_review` (event `01M5XRZA8W13PVNRDPQNGRQTX3`). Normal pre-review invocation completed without force/skip but reported `no_coverage` because `tests.architectural._gate_coverage` is unavailable. This infrastructure limitation is explicitly unverified; it is separate from the actual passing Go gates.
