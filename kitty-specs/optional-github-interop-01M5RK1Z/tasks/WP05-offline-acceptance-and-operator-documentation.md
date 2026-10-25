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
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop.
  During /spec-kitty.implement this WP may branch from a dependency-specific base,
  but completed changes must merge back into feat/optional-github-interop unless the
  human explicitly redirects the landing branch.
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
