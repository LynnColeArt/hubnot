---
work_package_id: WP01
title: Protected check policy and enforcement
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
planning_base_branch: feat/opt-in-policy-rules
merge_target_branch: feat/opt-in-policy-rules
branch_strategy: Planning artifacts for this mission were generated on feat/opt-in-policy-rules. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/opt-in-policy-rules unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
- T005
history: []
agent_profile: implementer-ivan
authoritative_surface: policy.go
create_intent:
- quarantine.go
- protected_checks.go
- protected_checks_test.go
execution_mode: code_change
lane: planned
owned_files:
- policy.go
- policy_commands.go
- governance.go
- protected_checks.go
- protected_checks_test.go
priority: P1
reviewer: ''
role: implementer
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Load implementer-ivan with the canonical profile load skill before implementation. Apply action-scoped charter context. Reviewers load reviewer-renata instead. Read AGENTS.md and the full spec/plan in this mission.

## Objective

Protected check policy and enforcement. Implement only this package, test first, preserve user-confirmed optionality. This is governed protocol/authority code; fail closed when opted in and preserve defaults when absent.

## Branch Strategy

Planning branch and consolidation target are feat/opt-in-policy-rules. Use only the workspace and branch allocated by spec-kitty agent action implement; lanes.json is authoritative. No direct main commits, no force pushes, no PR merge. Dependents start after approval.

## Subtasks

### T001

Write failing protection/legacy policy tests, including forged acceptance and withdrawal.

### T002

Add optional protectedChecks and automerge rule types and strict bounded validation.

### T003

Implement bounded literal-path diff and shared designated approval evaluation.

### T004

Integrate status, acceptance evidence/admission, and current opted-in manual merge gates.

### T005

Expose optional policy sections in inspection/comparison; verify and commit.

## Implementation guidance

Protect all required pipeline definitions and policy automatically only when protectedChecks is present. Explicit paths additionally cover lint config/scripts. Literal directory matching must cover ancestor type changes, deleting or moving a directory, and names with tabs/newlines in Git diff (NUL parsing, diagnostics safely rendered). Reject noncanonical/wildcard/control config paths. Disable renames so old and new paths appear. Bound actual diff output to 8 MiB (not only after buffering), stderr too. Use Git with external diff/textconv disabled. Rules count max64 actors/authors/paths; actors are maintainers; authors nonempty explicit IDs; paths nonempty; rule names unique, branch names valid. No defaults injected.

Use existing review events; designated actors need not belong to ordinary reviewer list. Never count the candidate author. Include exact qualifying IDs in acceptance Evidence; immutable validateDecisionEvent must reject acceptance that omits protection reviews. Current evaluateProposal reports changed protected paths and counts, and Ready includes them. Withdrawals/revisions cannot reuse current approval. Incoming acceptances are validated using exact base policy and candidate trees, not worktree.

Modify cmdMerge through a narrow reusable entry seam for WP02: cmdMerge delegates to mergeProposal(args, optionalGuard), where the guard receives current proposal/evaluation/branch/target OID and can refuse immediately before Git advancement. Recollect and reevaluate immediately before actual merge for enabled protection and guard; enabled protection requires current Ready. Keep legacy no-option manual semantics and repair tests intact. Preserve expected-head safety on advancement (Git's ref compare-and-swap plus explicit observed target guard); coordinate signature with WP02 via handoff. Avoid generic refactoring. Close the old-base bypass prospectively: if current target policy opts into protection and differs from candidate base digest, require a revision; do not invalidate historical signed decisions. quarantine.go independently validates replicated decisions: share new evidence validation there, not just local policy.go.

Tests include opt-out unchanged, independent auto configuration parse, automatic required-pipeline/policy protection, deleted/renamed/mode/type protected files, candidate policy disabling itself, designated approval distinct/non-author, withdrawal, fetched forged acceptance rejection, strict configs and bounded diff. Run focused race, vet/build and formatting before for_review; full race may run at final WP if no concern but charter gate before review still applies.

## Context and ownership

Authoritative requirements: FR-001, FR-002, FR-003, FR-004, FR-005. Consult existing exact Git verification and signed event validation; avoid widening role authority. Owned files: policy.go, policy_commands.go, governance.go, quarantine.go, protected_checks.go, protected_checks_test.go. New files are declared in create_intent. Coordinate a small necessary out-of-map edit with orchestrator and record rationale; do not edit another agent workspace. No Go module dependencies.

## Definition of Done

Each subtask is implemented and verified, tests demonstrate meaningful failures before code, new functions have live CLI/admission callers, old signed event bytes remain unchanged. Format changed Go with gofmt, verify git diff --check, stage only owned files, commit the implementation, mark subtasks via canonical mark-status, and transition through for_review after project checks. Do not edit lifecycle fields by hand or bypass review gates. Report commit, tests, limitations and any follow-up API seam to orchestrator. Independent reviewer records exact source-bound verdict.

## Reviewer guidance

Verify absent optional sections preserve current behavior; unknown/malformed configurations fail closed. Trace both live commands and fetched-event admission. Inspect exact base/head/actor/evidence binding and negative authority cases. Review finite output bounds, path escape/type/rename handling and side effects. Match docs/acceptance claims to actual code. No self-approval; record approve or reject through canonical review capture.
