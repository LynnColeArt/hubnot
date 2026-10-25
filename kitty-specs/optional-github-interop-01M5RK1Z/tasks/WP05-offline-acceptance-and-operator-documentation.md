---
work_package_id: WP05
title: Offline acceptance and operator documentation
dependencies:
- WP01
- WP04
- WP07
- WP08
- WP09
- WP11
- WP12
- WP13
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-010
- NFR-001
- NFR-002
- NFR-003
- C-001
- C-002
- C-003
- FR-011
- FR-012
- FR-013
- FR-014
- FR-015
- FR-016
- FR-017
- FR-018
- FR-019
- FR-020
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-github-interop-01M5RK1Z
base_commit: ea90761f5a98d0a5e0286970ff3913982107e537
created_at: '2026-10-25T07:42:55.087358+00:00'
subtasks:
- T019
- T020
- T021
- T022
history: []
agent_profile: implementer-ivan
authoritative_surface: docs/
create_intent:
- github_acceptance_test.go
- docs/github-interoperability.md
- docs/projects-v1.md
- docs/discussions-v1.md
- docs/workflows-v1.md
execution_mode: code_change
lane: planned
owned_files:
- github_acceptance_test.go
- README.md
- docs/github-interoperability.md
- docs/issues-v1.md
- docs/threat-model.md
- docs/projects-v1.md
- docs/discussions-v1.md
- docs/workflows-v1.md
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

Write public subprocess acceptance scenarios against built hn using real temporary repositories. Prove `hn gh issue` create/list/view/edit/comment/close/reopen roundtrips with `hn issue` and signed native history while gh is absent and GitHub credential variables unset. Build tagged nogithub variant; prove same local commands and imported ticket reads/edits work, explicit github command reports exclusion, and dependency graph excludes internal/githubapi. Reuse existing CLI compile/test fixtures appropriately without adding redundant full binary rebuilds per subtest. Do not tamper with global PATH for all test processes or skip core checks. Transport end-to-end fixture coverage can reference WP03/WP04 tests, but public routing must be exercised rather than only internal functions.

Document README command overview and linked docs/github-interoperability.md with all supported flags and JSON fields/shapes, native identifier distinction, installation/build with tag, explicit import/export examples, HN_GITHUB_TOKEN optional read/required write behavior, API endpoint override and finite bounds. Clear unsupported gh PR/run/API commands, no unmodified-gh binary/service claim. Explain imported source author/time versus native signer; labels/assignees don't grant policy authority; unsupported GitHub fields, native-only fields export behavior; immutable source mapping/baselines; duplicate/competing edit refusal; marker forgery limits; checkpoint locations/modes, one exporter, lost response recovery and no global exactly-once guarantee. No live GitHub mutations needed. Update existing issue and threat docs consistently.

Run full `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, tagged tests/build/vet (where useful), gofmt and git diff --check. Record logs under mission evidence or workspace work/ without committing private keys/binaries. Independent reviewer must inspect final behavior and docs against requirements and report any implementation defect to corresponding WP rather than masking it with weaker assertions. Deliver reviewable branch/PR; no public main merge.

## Subtasks

T019 Add independent public CLI lifecycle and transport-disabled acceptance checks.
T020 Verify imported signed issues remain usable without token, gh, provider or transport build.
T021 Document exact command/flag/JSON subset, provenance, conflicts, limits and recovery constraints.
T022 Run final full gates and record actual results, fixing only owned acceptance/docs or reporting implementation defects.

## Review criteria

Behavior matches every referenced requirement; scoped and full required gates pass; no unrelated files, private data or binaries staged. Unsupported behavior fails explicitly. Reviewer independently probes hostile/failure cases and the standalone boundary.


## Binding provider-neutral amendment from user steering

Read the Provider portability amendment in primary plan.md. Generic coordination/models/checkpoints/markers/source identity must use internal/forge, not githubapi DTOs. githubapi is only the optional first adapter in forge_github.go. Canonical commands: hn transfer import/export --provider github --project ...; hn github is an alias. New signed keys use forge/source, forge/baseline, forge/observation with provider-qualified source IDs. The user wants future alternate providers and project/workflow migration. Implement neutral resource/capability reports now, explicitly reporting project/workflow conversion unsupported or mapping-required before any write; report capability status accurately at each phase and verify all expanded native/adapter WPs before final delivery; never invent arbitrary provider CI equivalence. No mandatory service/provider dependency. nogithub excludes GitHub client dependency while local hn gh/native workflows remain functional.

Reference T023 (owned by WP03): implement and behavior-test provider-neutral models, adapter boundary, canonical provider-selecting commands and pre-write migration resource/capability reports.

Exporter uses forge.Provider and neutral options/models. GitHub adapter validates provider-specific selectors. Native export re-import without verified mapping must refuse duplicate/remapping; source import/export keeps original body and validates structured comment provenance to avoid feedback. Document exact first delivery limits and provider extension path.

## Binding full-delivery acceptance expansion

User explicitly requires native discussions/projects/workflows PLUS import/export. Final public acceptance must cover all four resource families, native signed Git/bare-remote reconstruction, nogithub full native lifecycle, exact workflow materialization/governed request, project typed schema/item references and discussion threaded/answer revisions. Fake-provider tests must prove supported GitHub roundtrips, explicit mappings, no blind uncertain-write retries and zero writes on unsupported plans. Include accurate operator docs per resource and capability matrices; distinguish preserved foreign definition from runnable native workflow and GitHub Projects automation rule limits. This WP now depends on all native/provider WPs and remains final delivery acceptance. No live provider mutation needed.

## Final discoverability and checkpoint integration checks

Final default and nogithub public `hn help` must advertise registered native project/discussion/workflow commands. Authorize the minimal main.go usage-render integration needed (dynamic sorted local command registry is appropriate), with public help tests; no parser/authority behavior rewrite. Existing main.go ownership remains WP03; this is a documented final integration edit, not overlapping concurrent ownership.

For every resource exporter, verify worst-case WHOLE checkpoint (snapshot + current Pending + future accumulated bindings/results) fits the private8MiB budget BEFORE first remote mutation. Shared executor persisting a Pending is necessary but does not by itself preflight the caller's later larger states. Validate declared graph/field/ID limits and no truncated migrations.

Integration finding from the approved issue exporter: its current preflight checks each rendered comment against the unknown issue-ID maximum, but does not reserve all future `Comments` bindings or a confirmed result in the whole checkpoint. A legal bounded backlog with long opaque provider IDs can therefore fill the journal only after remote publication has started. Authorize the minimal `github_export.go` preflight helper and focused `github_export_test.go` regression needed to reject that plan before any remote mutation, including dry-run and resume. Preserve existing publication/recovery semantics. This is a documented final integration correction to WP04's source, independently reviewed with WP05; do not weaken the budget or tests.

Additional concrete issue-journal probe (RED 0.172s at WP10 lane-j source): after completing an issue export, add a new signed native comment; manufacture `Comments[newCommentID] = humanRemoteID` in the private checkpoint. Export then reports success without publishing the new comment. Reproducer is retained outside source at `work/issue-journal-probe_test.go` and `work/issue-journal-overlay.json` in the chat workspace. Authorize a focused whole-checkpoint integrity correction in the same issue-export files, analogous to the independently reviewed discussion/project envelopes: a bounded canonical domain-separated signature bound to the active native identity for real export, with key-free preview, strict loading, explicit identity-change/unsigned-journal recovery refusal and whole-envelope size preflight. Equivalent exact signed-proof reconstruction is acceptable if it closes the concrete failure without trusting editable progress/bindings. Signatures never replace exact remote positive reconciliation or confer provider permissions. Preserve RED/fix/GREEN evidence and document any real-export identity requirement.

## Confirmed historical project-schema integration correction

Final source/doc audit reproduced `TestIndependentProjectHistoricalSchemaPreflight` (RED 0.331s against approved project source `4a15be65`): a native item pins a historical number field while the current project schema changes that same field ID to text. New-destination export performs 13 remote mutations and reports success with a number value under the text field. Authorize the narrow `github_projects.go` planner preflight and `github_projects_test.go` behavior regressions necessary to reject incompatible historical field/type/selected-option mappings BEFORE any provider mutation. Use the authenticated pinned schema from the exact native snapshot, never repin or silently migrate the native item. Ordinary project title/body edits with compatible fields remain supported. Removed populated fields and missing selected options must also refuse before writes. This is a documented WP05 integration exception to WP12 ownership, with independent final review and retained RED/fix/GREEN evidence. No unrelated project API or authority changes.
