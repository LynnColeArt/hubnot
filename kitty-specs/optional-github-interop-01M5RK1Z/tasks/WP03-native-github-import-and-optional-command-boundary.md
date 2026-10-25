---
work_package_id: WP03
title: Native GitHub import and optional command boundary
dependencies:
- WP01
- WP02
requirement_refs:
- FR-003
- FR-004
- FR-005
- FR-008
- FR-009
- NFR-001
- FR-011
- FR-012
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop.
  During /spec-kitty.implement this WP may branch from a dependency-specific base,
  but completed changes must merge back into feat/optional-github-interop unless the
  human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-github-interop-01M5RK1Z
base_commit: ea90761f5a98d0a5e0286970ff3913982107e537
created_at: '2026-10-25T04:16:03.656382+00:00'
subtasks:
- T009
- T010
- T011
- T012
- T013
- T023
history: []
agent_profile: implementer-ivan
authoritative_surface: github_commands.go
create_intent:
- github_commands.go
- github_disabled.go
- github_import.go
- github_import_test.go
- internal/forge/model.go
- internal/forge/model_test.go
- forge_github.go
execution_mode: code_change
lane: planned
owned_files:
- github_commands.go
- github_disabled.go
- github_import.go
- github_import_test.go
- main.go
- internal/forge/model.go
- internal/forge/model_test.go
- forge_github.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, this mission's spec.md and plan.md. Load implementer-ivan through the canonical profile-load skill and `spec-kitty agent profile show implementer-ivan --json`. Work only in the runtime-provided lane workspace. Use the existing native issue query/mutation and validation functions; never bypass event verification or alter signed wire schemas. Write behavior tests before implementation. This mission is explicitly authorized to completion; do not ask the user for repetitive permission. Independent review is required: do not approve your own work.

## Runtime and delivery

Use `spec-kitty next --agent <your-agent-name> --mission optional-github-interop-01M5RK1Z --json` and the generated action context. Implementation is a separate agent from review. Commit only owned source/tests/docs (and required task evidence), not unrelated generated state. Record subtasks with mark-status. Move to for_review only after relevant gates; if a synchronous gate remains running, poll it to completion rather than kill/retry. Return exact commit, commands/results, limitations and workspace. Do not merge to public main or publish GitHub issues.

## Global invariants

Core Hubnot and all local compatibility commands work offline with no credentials or gh executable. Explicit GitHub transport is standard-library only and excluded by the nogithub build tag. Imported facts use the native signer; external attribution remains source metadata. No implicit sync, timestamps-as-winner, shell execution of ticket text, or unsupported silent semantics. Existing 256 KiB native bounds apply, including 64 KiB body and namespaced metadata value <=4096 bytes. No credentials in logs/signed metadata/checkpoints. All list/history readers consume complete bounded pages, and stale snapshot errors refuse partial answers.

## Objective and implementation contract

Implement `cmdGitHub(args []string) error`, `cmdGH` top-level main.go route, explicit github import command and common helpers. Transport files use !nogithub; github_disabled.go uses nogithub and returns clear error/help with no network package import. To keep this WP buildable before exporter lands, export handler should return explicit "export not installed yet" until WP04 adds it through a package-level hook or registration; document the temporary seam only in mission records, final help must not claim unsupported behavior. Prefer function variable default `githubExport = ...` with WP04 init registration, or agree a minimal typed stub-owned-file extension with orchestrator.

Commands: `hn github import --repo OWNER/REPO [--issue NUMBER] [--api-url URL] [--dry-run]` and later `hn github export ISSUE --repo OWNER/REPO [--api-url URL] [--dry-run]`. Default https://api.github.com; only HN_GITHUB_TOKEN for auth, never invoke gh or ambient GitHub credential helpers. Strict unknown arguments; import --issue positive integer; dry-run emits proposed actions and never mutates native data/ref/checkpoint. Obtain client only for explicit transport; local commands ignore credentials. Export parse routes to hook implemented WP04.

Store signed provenance in `github/source` immutable opening metadata: base API URL, repo, immutable ID/NodeID, number, URL, source author/created/updated. Use stable host+NodeID (or validated ID fallback) source key; source mutation is descriptive not authority. `github/baseline` digest tracks managed native title/body/status/labels/assignees. Preserve original source body and source assignee strings without authority; source author/times as metadata, comments with attribution text. Validate every field and comment (including attributed body) against native bounds before any mutation for that issue. No truncation. Import PRs must skip/refuse explicitly. All pages/comments are fully read/validated first for an issue.

Discover matching immutable opening metadata across all detail pages; refuse duplicate claims, malformed claims, null/conflict states. Native-only criteria/relations/unrelated metadata survive. Three-way: transformed remote digest unchanged from saved baseline => preserve any local changes; local digest unchanged => guarded remote update; both changed => conflict unless identical resulting managed content (then baseline safely advances). Validate baseline missing/tampered conservatively. Native mutation uses captured identity actor, exact original head and deterministic <=128 printable operation key; persist exact actor/input/expect/key before mutation when needed. Read-only operation lookup can recover lost responses. Do not change expected head or operation intent during uncertain replay. Source comment content edits append attributed correction using stable comment ID+content digest; repeated import is a no-op, including from a clone where local journal absent. Avoid agent identity change creating duplicate comments: inspect signed comment provenance markers or existing native history, not just actor-scoped operation lookup.

Local checkpoint/lock helpers for exporter belong here: resolve real Git directory via existing git seam, private mode atomic files with bounded JSON, no credentials; per-workflow lock with explicit stale recovery if needed. A checkpoint is recovery, never the only mapping source. Tell exporter agent exact helper API. Core must build/tests with and without tag; no wire schema changes. Unit/public tests use real temporary native repos and fake provider; fixtures can use existing helpers. Keep failure errors sanitized when remote data contains secrets/control content.

## Subtasks

T009 Specify native import/repeat/update/conflict and optional-build behavior tests first.
T010 Wire hn gh and hn github dispatch; parse strict import/export commands and optional client credentials.
T011 Implement complete native source discovery and validated attributed issue/comment imports.
T012 Implement guarded three-way updates, safe operation retries and checkpoint helpers for later export.
T013 Validate real native storage against fake paginated provider and nogithub build.

## Review criteria

Behavior matches every referenced requirement; scoped and full required gates pass; no unrelated files, private data or binaries staged. Unsupported behavior fails explicitly. Reviewer independently probes hostile/failure cases and the standalone boundary.


## Recovery contract refinement (pre-implementation review)

Export checkpoint phases are prepared/in-flight/confirmed. A complete negative marker scan after an uncertain POST is not proof that the POST never executed: refuse another POST and require manual reconciliation. Positive marker recovery must validate immutable destination identity and exact intended transmitted projection, not marker alone. Export baseline digests cover the normalized transmitted representation including generated provenance/markers; they are distinct from import managed-state digests. The first export back to an imported issue's source may use the signed imported source projection as baseline only if it matches current remote content; missing or mismatched baseline refuses overwrite. Imports validate a complete issue/comment set before the first mutation, but multiple signed events are not atomic: interrupted imports can leave valid partial work and resume through the journal/provenance checks. Do not claim transactionality.


## Binding provider-neutral amendment from user steering

Read the Provider portability amendment in primary plan.md. Generic coordination/models/checkpoints/markers/source identity must use internal/forge, not githubapi DTOs. githubapi is only the optional first adapter in forge_github.go. Canonical commands: hn transfer import/export --provider github --project ...; hn github is an alias. New signed keys use forge/source, forge/baseline, forge/observation with provider-qualified source IDs. The user wants future alternate providers and project/workflow migration. Implement neutral resource/capability reports now, explicitly reporting project/workflow conversion unsupported or mapping-required before any write; don't invent arbitrary provider CI equivalence or claim those migrations shipped. No mandatory service/provider dependency. nogithub excludes GitHub client dependency while local hn gh/native workflows remain functional.

T023 (WP03) Implement and behavior-test provider-neutral models, adapter boundary, canonical provider-selecting commands and pre-write migration resource/capability reports.

Exporter uses forge.Provider and neutral options/models. GitHub adapter validates provider-specific selectors. Native export re-import without verified mapping must refuse duplicate/remapping; source import/export keeps original body and validates structured comment provenance to avoid feedback. Document exact first delivery limits and provider extension path.
