---
work_package_id: WP02
title: Useful-work scheduling and explicit stale target observation
dependencies:
- WP01
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-007
- FR-008
- NFR-001
- NFR-003
planning_base_branch: fix/runner-efficiency
merge_target_branch: fix/runner-efficiency
branch_strategy: Planning artifacts for this mission were generated on fix/runner-efficiency. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/runner-efficiency unless the human explicitly redirects the landing branch.
subtasks:
- T006
- T007
- T008
- T009
- T010
phase: Phase 2 - Native efficiency
history:
- at: '2026-11-03T20:32:32Z'
  actor: planner-priti
  action: Authored scoped runner-efficiency tasks after confirmed spec/plan.
agent_profile: implementer-ivan
authoritative_surface: runner.go
create_intent:
- runner_eligibility.go
- runner_target.go
- runner_efficiency_test.go
execution_mode: code_change
owned_files:
- runner.go
- runner_eligibility.go
- runner_target.go
- runner_efficiency_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP02: Useful-work scheduling and explicit stale target observation

## ⚡ Do This First: Load Agent Profile

Load implementer-ivan through the canonical Spec Kitty profile API/CLI before
working. Read spec.md, plan.md, research.md, data-model.md and the native runner
contract in this mission. Respect implementation/profile boundaries. The root
orchestrator owns cross-WP sequencing and independent approval.

## Objectives & Success Criteria

Deliver the specific behavior in this WP's subtasks with actual observable
acceptance evidence. Requirement references are registered through the canonical
map-requirements command. Preserve the first default-workflow gate throughout.

## Context & Constraints

Mission: runner-efficiency-01M6HH8H. Read the project charter's native protocol, immutable
evidence, least-authority and targeted-staging rules. The design reuses existing
lineage and trusted-result semantics rather than adding parallel policy logic.

## Branch Strategy

Planning/base and mission consolidation target: fix/runner-efficiency.
Final public landing: main via a reviewed PR. Work only in the runtime-returned
workspace; do not allocate a new raw Git worktree or alter old mission worktrees.
Implementation command: spec-kitty agent action implement WP02 --mission runner-efficiency-01M6HH8H --agent codex --profile implementer-ivan.
Dependencies: WP01; approved dependencies unblock this work.

## Subtasks & Detailed Guidance

### T006 — Specify scheduler outcomes with real controls

Write failing observable cases for merged/superseded/closed/conflicted lineages and threshold-complete requests. Use real Git fixtures and backend marker files plus event/ref snapshots to prove no backend start/result append. Include an eligible request and ungoverned explicit execution as positive controls.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T007 — Integrate lineage and trusted-threshold eligibility

Reuse the existing lineage index once per observed event set and WP01 shared pipeline evaluator. Honor current actor/pipeline filter and own-result suppression. Only a configured required pipeline can suppress via its qualifying trusted threshold. Latest failures and mismatched/untrusted evidence must not be treated as successful completion. Log actionable UTC skip reasons.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T008 — Add explicit stale-base runner options

Add --skip-stale-base and --target refs/heads/NAME with clear validation. No local HEAD/ref inference is allowed. Without the explicit stale option, a moved remote base alone must not change execution. Invalid or incomplete combinations must refuse before relevant effects.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T009 — Implement bounded authenticated target observation

Probe the exact configured remote ref immediately before execution using contextual Git, finite deadline <=15s and <=64KiB output. Resolve/query under one sanitized environment; retain normal configured auth, HOME/XDG and SSH agent, strip ambient object/repository/config injection, disable built-in prompts/askpass. Support standard OpenSSH batch mode without silently discarding custom identity/proxy arguments; refuse unsupported wrappers actionably. Credential helpers are intentional operator config, not a claimed hostile-code sandbox. Missing/malformed/duplicate/error/timeout/cancellation must prevent execution; no fetch, ref write or checkout mutation.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

### T010 — Validate automatic-only semantics and hand off

Prove a second runner skips after observing enough trusted evidence; threshold logic is not an exclusive claim when starts race. Use exact request/candidate/definition checks, fresh target-advancement and failed-target controls with untouched backend marker. Add explicit probe controls for authenticated configured-remote success after environment sanitization, poisoned ambient Git redirects/config injection, retained standard SSH identity/proxy semantics or actionable custom-wrapper refusal, output overflow, deadline, cancellation and built-in prompt/askpass suppression. Preserve manual run execute behavior at this WP. Run focused race tests and static/diff checks; commit and enter for_review with gate completion.

**Parallel**: No; this WP shares the computed sequential workspace and must
respect its dependency order. Check the owned-file metadata before editing.

## Validation and evidence

Before editing production code, write or select a contract-level failing test
and record the intended refusal/success outcome. Existing positive controls must
remain genuine; avoid mocks which bypass authorization or pipeline execution.

Use actual CLI/Git boundaries and finite timeouts. For skip/refusal behavior,
pre-create backend markers before invocation, snapshot relevant objects/refs/
private signer state, assert no change, and run an eligible positive control
after refusal assertions without resetting evidence.

Follow the explicit validation governance decision in plan.md and
.kittify/charter/charter.yaml: before review at each changed source, run focused
behavioral race regressions, native fast/default controls, gofmt, go vet ./...,
go build ./... and git diff --check. Record exact source, selections and exits;
any observed functional failure blocks approval. Independent review verifies
that evidence and runs meaningful focused controls without repeating unchanged
successful checks solely for handoff. Mission acceptance additionally requires
all affected integration/migration/protocol/runner/recovery/compatibility cases.
Exhaustive capacity/stress/transport-deadline/real-clock soak qualification stays
an explicit, separately tracked native release prerequisite. Preserve every
assertion, fixture and deadline. Never claim a scoped pass is exhaustive,
expand hosted deep corpora or mutate frozen signed fixtures.

Record commands, source head, test selection, exits and limitations in this WP's
evidence directory under its mission task folder. Independent reviewer reads
the fixed diff and runs meaningful checks. A failed negative control is not
evidence unless it fails for the promised reason.

## Safety and compatibility

- Application remains native Go/Git and existing sandbox/default authority.
- Do not enable policy, accounts, extensions, watchers or services implicitly.
- Withdrawals never rewrite prior signed history or invalidate earlier results.
- Keep contribution/moderation and trusted pipeline boundaries intact.
- Preserve unrelated original snapshot, 21 original stashes, old worktrees and
  all published/frozen release files. Stage specific owned files only.
- If a necessary file falls outside ownership, report it to root for a scoped
  planning amendment; do not change sibling WPs or sweep unrelated files.
- Optional framework hosted-sync warnings do not authorize login or changing
  shared configuration. Preserve local canonical events and record warnings.

## Review Feedback

If re-dispatched, read the canonical review artifact/event references first,
address every finding with causal evidence, and do not repeat a rejected
implementation unchanged. Never alter a verdict by editing rendered Markdown.

## Definition of Done

All listed subtasks are implemented or explicitly evidenced; new production
paths have real callers; declared checks pass at the committed head; original
workflow and signed evidence remain valid. Mark subtasks through
spec-kitty agent tasks mark-status, not checkboxes. Move only this WP to
for_review and poll the synchronous pre-review gate until it exits. Never
approve your own work, skip a failed guard, or move yourself to done.

## Reviewer guidance

Inspect behavior against spec, plan, contract and this WP's subtasks, including
no-effects and positive controls. Check final source identity and live call
sites; reject hidden fallback, weakened evidence/trust, fabricated pass claims
or an implicit default change. Approve/reject through canonical runtime events.
