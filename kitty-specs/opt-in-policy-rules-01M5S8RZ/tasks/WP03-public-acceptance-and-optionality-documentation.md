---
work_package_id: WP03
title: Public acceptance and optionality documentation
dependencies:
- WP01
- WP02
requirement_refs:
- FR-010
- NFR-001
- NFR-002
- C-001
- C-002
- C-003
planning_base_branch: feat/opt-in-policy-rules
merge_target_branch: feat/opt-in-policy-rules
branch_strategy: Planning artifacts for this mission were generated on feat/opt-in-policy-rules. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/opt-in-policy-rules unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-opt-in-policy-rules-01M5S8RZ
base_commit: 8634f122d04703367e11bc74283f38939080d90e
created_at: '2026-10-25T11:45:24.149742+00:00'
subtasks:
- T010
- T011
- T012
history: []
agent_profile: implementer-ivan
authoritative_surface: policy_rules_acceptance_test.go
create_intent:
- policy_rules_acceptance_test.go
execution_mode: code_change
lane: planned
owned_files:
- policy_rules_acceptance_test.go
- README.md
- docs/governance-v0.md
- docs/protocol-v0.md
- docs/threat-model.md
priority: P1
reviewer: ''
role: implementer
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Load implementer-ivan with the canonical profile load skill before implementation. Apply action-scoped charter context. Reviewers load reviewer-renata instead. Read AGENTS.md and the full spec/plan in this mission.

## Objective

Public acceptance and optionality documentation. Implement only this package, test first, preserve user-confirmed optionality. This is governed protocol/authority code; fail closed when opted in and preserve defaults when absent.

## Branch Strategy

Planning branch and consolidation target are feat/opt-in-policy-rules. Use only the workspace and branch allocated by spec-kitty agent action implement; lanes.json is authoritative. No direct main commits, no force pushes, no PR merge. Dependents start after approval.

## Subtasks

### T010

Add black-box public CLI tests for opt-out, protections and explicit automerge.

### T011

Document opt-ins, configuration, exact evidence, limitations and manual recovery.

### T012

Run default/nogithub acceptance and all release checks, recording exact evidence.

## Implementation guidance

Read spec and plan and actual implementation. Public acceptance uses built CLI and real temporary Git repositories; same test harness must run default and nogithub variants. Test absent config unchanged, only protection enabled, only auto enabled, and both; successful normal change and protected no-op lint replacement refusal; designated approval then manual acceptance/merge; exact rule accepted proposal previews without key then explicit eligible actor execution records signed merge event. Capture actor refs/HEAD before and after preview. Target advance and config deletion must fail. No remote host needed; optional local bare remote if useful. Avoid excessive duplicate full builds in tiny cases.

Document literal path semantics, automatic policy/required pipeline coverage when opted in, independent disabled defaults, no semantic lint analysis, actor trust, exact review evidence, pre-existing acceptance requirement, conservative target equality, no background process, explicit publication and ordinary merge repair. Old clients fail closed on enabled unknown sections. Never enable either feature in this repository's .hn/policy.json or rewrite frozen signed history. CLI help is WP02 ownership; request implementation correction through orchestrator instead of silently editing source.

Run go test -race ./..., go vet ./..., go build ./..., go test -tags nogithub ./..., and git diff --check. Record exact outputs and final source SHA. Additional test runs only on failures or subsequent code changes. Docs align with actual behavior; no unsupported claims about atomic distributed evidence/branch transactions or arbitrary Git writer enforcement.

## Context and ownership

Authoritative requirements: FR-010, NFR-001, NFR-002, C-001, C-002, C-003. Consult existing exact Git verification and signed event validation; avoid widening role authority. Owned files: policy_rules_acceptance_test.go, README.md, docs/governance-v0.md, docs/protocol-v0.md, docs/threat-model.md. New files are declared in create_intent. Coordinate a small necessary out-of-map edit with orchestrator and record rationale; do not edit another agent workspace. No Go module dependencies.

## Definition of Done

Each subtask is implemented and verified, tests demonstrate meaningful failures before code, new functions have live CLI/admission callers, old signed event bytes remain unchanged. Format changed Go with gofmt, verify git diff --check, stage only owned files, commit the implementation, mark subtasks via canonical mark-status, and transition through for_review after project checks. Do not edit lifecycle fields by hand or bypass review gates. Report commit, tests, limitations and any follow-up API seam to orchestrator. Independent reviewer records exact source-bound verdict.

## Reviewer guidance

Verify absent optional sections preserve current behavior; unknown/malformed configurations fail closed. Trace both live commands and fetched-event admission. Inspect exact base/head/actor/evidence binding and negative authority cases. Review finite output bounds, path escape/type/rename handling and side effects. Match docs/acceptance claims to actual code. No self-approval; record approve or reject through canonical review capture.
