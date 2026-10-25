---
work_package_id: WP02
title: Explicit native rule based automerge
dependencies:
- WP01
requirement_refs:
- FR-006
- FR-007
- FR-008
- FR-009
planning_base_branch: feat/opt-in-policy-rules
merge_target_branch: feat/opt-in-policy-rules
branch_strategy: Planning artifacts for this mission were generated on feat/opt-in-policy-rules. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/opt-in-policy-rules unless the human explicitly redirects the landing branch.
subtasks:
- T006
- T007
- T008
- T009
history: []
agent_profile: implementer-ivan
authoritative_surface: automerge.go
create_intent:
- automerge.go
- automerge_test.go
execution_mode: code_change
lane: planned
owned_files:
- automerge.go
- automerge_test.go
- main.go
priority: P1
reviewer: ''
role: implementer
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Load implementer-ivan with the canonical profile load skill before implementation. Apply action-scoped charter context. Reviewers load reviewer-renata instead. Read AGENTS.md and the full spec/plan in this mission.

## Objective

Explicit native rule based automerge. Implement only this package, test first, preserve user-confirmed optionality. This is governed protocol/authority code; fail closed when opted in and preserve defaults when absent.

## Branch Strategy

Planning branch and consolidation target are feat/opt-in-policy-rules. Use only the workspace and branch allocated by spec-kitty agent action implement; lanes.json is authoritative. No direct main commits, no force pushes, no PR merge. Dependents start after approval.

## Subtasks

### T006

Write failing native rule, preview and execution tests.

### T007

Implement deterministic named-rule evaluation and read-only default preview.

### T008

Implement explicit execution using current native merge gates and expected target.

### T009

Verify stale evidence/target, author/path/actor mismatch and manual repair safety.

## Implementation guidance

Use the optional types/validation and merge entry seam supplied by WP01. No hidden role grant or auto acceptance. Command proposed: hn automerge PROPOSAL --rule NAME [--execute]. Default preview must work without a private key and leave HEAD, index, actor refs and working files unchanged. Explain disabled configuration or blockers with deterministic safe text and full exact IDs; returning a nonzero refusal for blocked preview is okay if documented. A rule independently selects branch, executing maintainers, authors and literal paths; no implicit wildcard. Explicit rule required; no fallback match. Reject empty change sets for automation.

Require Ready, Accepted, no trusted current change requests, no current rejection, active named lineage, code ref equals signed head, target checked-out branch and OID equal signed base, author allowed, every changed path allowed, exact target policy same as signed base. Block policy edits ALWAYS even without protection; block all protected changes when protection enabled even if designated approval exists. All permission from base, never proposed head. Preview lists permitted actors rather than loading signing keys; execute loads current selected identity and checks rule actor. No pipeline, shell expression, sync, push, network or daemon.

Final guard rechecks rule/candidate/current evidence, branch and expected target immediately before actual advancement. Do not trust an earlier preview. Address target races concretely using a ref lease/CAS or expected-ref mechanism; do not claim to freeze arbitrary external writers. Use narrow existing merge and manual missing-event recovery; retry after merge should refuse with existing status/repair guidance. Existing history stays byte-identical. Do not add a fake marker or new unsigned authority. Ensure command registered in main help. Tests must exercise actual successful native merge and signed event, no side effects in dry preview, config independent from protection, path rename escape, stale targets, withdrawn approvals/current failures, unauthorized actor and policy amendment. Run focused and full charter checks before handoff.

## Context and ownership

Authoritative requirements: FR-006, FR-007, FR-008, FR-009. Consult existing exact Git verification and signed event validation; avoid widening role authority. Owned files: automerge.go, automerge_test.go, main.go. New files are declared in create_intent. Coordinate a small necessary out-of-map edit with orchestrator and record rationale; do not edit another agent workspace. No Go module dependencies.

## Definition of Done

Each subtask is implemented and verified, tests demonstrate meaningful failures before code, new functions have live CLI/admission callers, old signed event bytes remain unchanged. Format changed Go with gofmt, verify git diff --check, stage only owned files, commit the implementation, mark subtasks via canonical mark-status, and transition through for_review after project checks. Do not edit lifecycle fields by hand or bypass review gates. Report commit, tests, limitations and any follow-up API seam to orchestrator. Independent reviewer records exact source-bound verdict.

## Reviewer guidance

Verify absent optional sections preserve current behavior; unknown/malformed configurations fail closed. Trace both live commands and fetched-event admission. Inspect exact base/head/actor/evidence binding and negative authority cases. Review finite output bounds, path escape/type/rename handling and side effects. Match docs/acceptance claims to actual code. No self-approval; record approve or reject through canonical review capture.
