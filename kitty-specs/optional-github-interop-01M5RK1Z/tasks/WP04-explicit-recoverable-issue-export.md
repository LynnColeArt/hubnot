---
work_package_id: WP04
title: Explicit recoverable issue export
dependencies:
- WP03
requirement_refs:
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- NFR-001
- FR-011
- FR-012
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop.
  During /spec-kitty.implement this WP may branch from a dependency-specific base,
  but completed changes must merge back into feat/optional-github-interop unless the
  human explicitly redirects the landing branch.
subtasks:
- T014
- T015
- T016
- T017
- T018
history: []
agent_profile: implementer-ivan
authoritative_surface: github_export.go
create_intent:
- github_export.go
- github_export_test.go
execution_mode: code_change
lane: planned
owned_files:
- github_export.go
- github_export_test.go
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

Use !nogithub build constraints. Register export handler through WP03 hook, preserving no-core-network boundary. Read exact selected native issue and complete attributed native comments from verified query API. Refuse native conflicts and unsupported unrepresentable fields explicitly or publish a documented readable representation; never discard native criteria/relations/metadata claiming full fidelity. Preserve local native data. Include source native actor/time in exported comments, and issue ID/source actor in visible provenance. Imported comment provenance must not bounce back as duplicated comments to the exact source destination.

Use stable hidden issue marker derived from full native issue ID (and comment markers from signed comment IDs). Resolve mappings via native imported source metadata when destination matches and immutable remote identity verifies, otherwise complete bounded target issue marker scan. Reject multiple matches, PR matches, wrong IDs, corrupt checkpoints, or conflicting ownership markers. Never choose by timestamp. All write auth checked before first mutation. `--dry-run` can read remote but writes no GitHub/native/checkpoint data.

Before POST, atomically persist intent, selected native head/content/comments and marker to private Git-directory checkpoint; local lock prevents concurrent local exporters. On restart/lost response, completely reconcile bounded listings/comments before any creation; if outcome is unestablished or ambiguous refuse with actionable message instead of blind retry. Markers are forgeable descriptive data; do not grant authority. Do not auto-retry POST/PATCH in HTTP client. Persist external ID and normalized baseline after confirmed responses. For repeat update, compare current remote content with last successful baseline; unchanged remote permits local update, unchanged local preserves remote changes; both changed refuses. If baseline lost on a fresh clone, conservatively refuse overwriting externally changed ticket rather than assume ownership. Document one local exporter and that no distributed atomic/exactly-once guarantee exists.

Publish title/body/labels/assignees via API and open/closed via UpdateIssue (CreateIssue doesn't set closed state); check API returned state/identity to detect silently ignored fields before checkpointing success. Comments stable marker reconciliation handles ambiguous POST and repeated execution. Initial imported issue's remote original comment IDs should suppress feedback duplication and retain honest attribution. Persist checkpoints without tokens. Respect all HTTP/native bounds and refuse ambiguous mappings. Tests use fake HTTP provider + actual native issue events, including lost response after server creation and repeated exports, edits on both sides and checkpoint resume. Coordinate helper signatures with WP03 implementer. Do not edit root parser/routing except a documented minimal helper exposure if necessary.

## Subtasks

T014 Write explicit export, repeat/resume and remote-conflict fixture tests first.
T015 Preflight selected native issue/comments and derive stable publication identity/markers.
T016 Implement signed provenance-aware create/update export and lifecycle/comment publication.
T017 Implement bounded reconciliation/checkpoints for uncertain creation and conservative competing-edit refusal.
T018 Validate no duplicate issues/comments, no blind retries, no token leaks and actionable recovery.

## Review criteria

Behavior matches every referenced requirement; scoped and full required gates pass; no unrelated files, private data or binaries staged. Unsupported behavior fails explicitly. Reviewer independently probes hostile/failure cases and the standalone boundary.


## Recovery contract refinement (pre-implementation review)

Export checkpoint phases are prepared/in-flight/confirmed. A complete negative marker scan after an uncertain POST is not proof that the POST never executed: refuse another POST and require manual reconciliation. Positive marker recovery must validate immutable destination identity and exact intended transmitted projection, not marker alone. Export baseline digests cover the normalized transmitted representation including generated provenance/markers; they are distinct from import managed-state digests. The first export back to an imported issue's source may use the signed imported source projection as baseline only if it matches current remote content; missing or mismatched baseline refuses overwrite. Imports validate a complete issue/comment set before the first mutation, but multiple signed events are not atomic: interrupted imports can leave valid partial work and resume through the journal/provenance checks. Do not claim transactionality.


## Binding provider-neutral amendment from user steering

Read the Provider portability amendment in primary plan.md. Generic coordination/models/checkpoints/markers/source identity must use internal/forge, not githubapi DTOs. githubapi is only the optional first adapter in forge_github.go. Canonical commands: hn transfer import/export --provider github --project ...; hn github is an alias. New signed keys use forge/source, forge/baseline, forge/observation with provider-qualified source IDs. The user wants future alternate providers and project/workflow migration. Implement neutral resource/capability reports now, explicitly reporting project/workflow conversion unsupported or mapping-required before any write; don't invent arbitrary provider CI equivalence or claim those migrations shipped. No mandatory service/provider dependency. nogithub excludes GitHub client dependency while local hn gh/native workflows remain functional.

Reference T023 (owned by WP03): implement and behavior-test provider-neutral models, adapter boundary, canonical provider-selecting commands and pre-write migration resource/capability reports.

Exporter uses forge.Provider and neutral options/models. GitHub adapter validates provider-specific selectors. Native export re-import without verified mapping must refuse duplicate/remapping; source import/export keeps original body and validates structured comment provenance to avoid feedback. Document exact first delivery limits and provider extension path.
