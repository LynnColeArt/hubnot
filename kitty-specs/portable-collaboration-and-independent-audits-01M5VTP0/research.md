# Discovery and decisions

Date: 2026-10-26. Scope follows the user's accepted portable-project vision,
the next-release Go/Git-only packaging requirement, and issue 13's native
nonblocking audit backlog. The mission target is `feat/portable-collaboration`,
not public `main`; implementation produces a reviewable PR.

## Existing boundaries

- `internal/forge/provider.go` and `internal/forge/resources.go` separate optional
  provider values, capabilities and typed readers from native authority.
  `resource_transfer.go` and `github_export.go` already provide explicit mapping
  and prepared/in-flight/confirmed recovery. Preserve those semantics and old
  transfer commands.
- Existing transfer graphs are not whole-project archives: no Git-history
  bundle, downloaded attachment blobs, project-wide selection or PR/MR
  conversations. Add a project snapshot above the resource-transfer boundary,
  not a claim that issue transfer alone migrates a project.
- `run.request`/`run.result` are proposal-bound across `event.go`, `store.go`,
  `ci.go`, `quarantine.go` and `shallow.go`. Introduce a distinct observational
  audit family and reuse low-level execution. Broadening run results could
  accidentally grant merge evidence, so it is rejected.
- `backend.go` already owns sandbox selection and the explicit unsafe-host
  flag. Audit work must use this boundary, not a new implicit command executor.
- `v0.0.1-dev`'s Python packaging script is a release asset, not source runtime.
  Replace the build process prospectively with Go tooling; preserve old assets.

## Material decisions

1. **Two preservation layers:** exact native Git/signed histories, plus canonical
   attributed foreign records and content-addressed attachments. Foreign users,
   reviews and CI data cannot become native signers or policy authority.
2. **Saved plans:** guided and automated migration use the same plan and apply/
   resume path. Capabilities, missing information and acknowledged losses are
   visible before writes. Tokens/private keys are never plan content.
3. **Capability profiles:** similarly named objects are not assumed equivalent.
   A provider may support preservation while not supporting native mapping or
   destination creation. Unread content is unavailable, not preserved.
4. **Bitbucket Cloud:** the Issue Tracker APIs were removed on 2026-08-20;
   app passwords retired on 2026-07-28. Require available repository/PR/workflow
   coverage and explicit unsupported issues. Do not substitute Jira or Data
   Center interfaces. Official source: the Bitbucket Cloud changelog below.
5. **Azure DevOps:** work items, Git repositories and planning projects are
   separate. Require explicit organization/project/repository and work-item
   type/state mappings; use revision guards where the API provides them.
6. **Rule authority vs execution:** audit target and rule revisions are recorded
   separately. Existing protected-rule approval remains authoritative. Merely
   publishing a collaborative workflow is not a rule-owner authorization.
7. **Retry limits:** positive reconciliation or manual intervention follows an
   uncertain remote write. Native operation keys are actor-scoped; neither
   transfers nor follow-up issues promise distributed exactly-once creation.
8. **Adapter-free build:** preserve `nogithub`'s current meaning and add an
   explicit all-adapters-excluded build, retaining local migrations/audits.

## Official provider evidence

| Provider | Evidence | Consequence |
| --- | --- | --- |
| GitLab | https://docs.gitlab.com/api/issues/ ; https://docs.gitlab.com/api/discussions/ ; https://docs.gitlab.com/api/boards/ ; https://docs.gitlab.com/api/project_markdown_uploads/ | Preserve global IDs and project-local IIDs, note/thread topology and original attribution. Boards are not Git repositories; support explicit self-hosted roots. |
| Forgejo | https://forgejo.org/docs/latest/user/api/usage/ ; https://codeberg.org/swagger.v1.json | API compatibility is major-version scoped; instances control availability and limits. Separate PR reviews and issue/comment assets. |
| Gitea | https://docs.gitea.com/development/api-usage/ ; https://docs.gitea.com/api/1.26/operations/tags/issue/ | Keep separate registration/conformance, preserve stable IDs/local indexes, bounded pagination and scoped credentials. |
| Bitbucket Cloud | https://developer.atlassian.com/cloud/bitbucket/changelog/ ; https://developer.atlassian.com/cloud/bitbucket/rest/api-group-pullrequests/ ; https://developer.atlassian.com/cloud/bitbucket/rest/ | Removed issue API; UUID identity vs mutable slugs; opaque bounded next links; no automatic Data Center equivalence. |
| Azure DevOps | https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items?view=azure-devops-rest-7.1 ; https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/get-comments?view=azure-devops-rest-7.1 ; https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-threads?view=azure-devops-rest-7.1 | Typed work items use explicit mapping/JSON Patch, comments have continuation tokens; PR threads are repository-scoped. |

## Risks to validate

Archive admission must reject hostile paths, symlinks, duplicate/unknown members,
digest mismatches, expansion bombs, incomplete native event closure and authority
injection before accepted visibility. Never import archived actor refs straight
into local signing-owned streams. Endpoint/attachment/pagination validation must
not forward credentials to a different origin. Existing destination branches,
working files, policy and private state need expected-old guards.

Provider acceptance uses deterministic local API fixtures and ordinary local Git
remotes. Live account mutations, credential provisioning and source deletion are
not authorized development checks. Runtime platform claims require actual
execution; cross-compilation is reported separately.

## Deferred deliberately

Audit scheduling, batching, background workers and compute-budget orchestration
follow the explicit request/result/follow-up model. A stable runtime plugin ABI,
universal CI conversion, hosted permission/secret migration and Windows CLI
release qualification are outside this mission.
