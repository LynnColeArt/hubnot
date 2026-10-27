---
work_package_id: WP02
title: Native observational audit protocol and admission
dependencies: []
requirement_refs:
- FR-001
- FR-016
- FR-018
- FR-019
- NFR-001
- NFR-002
planning_base_branch: feat/portable-collaboration
merge_target_branch: feat/portable-collaboration
branch_strategy: Planning artifacts for this mission were generated on feat/portable-collaboration. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/portable-collaboration unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0
base_commit: 946a72d8c76836814a349d71323f2baf41403ebd
created_at: '2026-10-26T10:51:41.561442+00:00'
subtasks:
- T004
- T005
- T006
phase: Portable collaboration
history:
- at: '2026-10-26T10:30:30Z'
  actor: system
  action: Prompt drafted from canonical task template for runtime task authoring
agent_profile: implementer-ivan
authoritative_surface: audit_protocol.go
create_intent:
- audit_protocol.go
- audit_protocol_test.go
- audit_authority.go
- audit_authority_test.go
- docs/audits-v1.md
execution_mode: code_change
owned_files:
- audit_protocol.go
- audit_protocol_test.go
- audit_authority.go
- audit_authority_test.go
- event.go
- event_test.go
- store.go
- quarantine.go
- shallow.go
- policy.go
- policy_commands.go
- docs/audits-v1.md
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# Work Package Prompt: WP02 — Native observational audit protocol and admission

## Do This First: Load Agent Profile

Resolve `implementer-ivan` through the canonical Spec Kitty profile surface and follow its boundaries. Agent/tool is `codex`; no model override is prescribed. Read the runtime-generated implement prompt, current review feedback and the mission charter context before editing.

## Review Feedback

Read any runtime/event-log review reference first. Address every rejection explicitly; preserve feedback and source-bound verification. Lifecycle changes are made through Spec Kitty commands, never frontmatter edits.

## Objectives & Success Criteria

Introduce signed audit requests/results tied to inspected code and authorized rule revisions, with one authority contract for local writes, reads and replication.

**Independent acceptance**: A valid audit exchange replicates through a real temporary Git remote, while forged relationships, missing rule authorization, missing/stale authority anchor and partial/dependency-invalid events refuse without accepted visibility; audit facts never count as merge evidence.

**Requirements**: FR-001, FR-016, FR-018, FR-019, NFR-001, NFR-002.

## Context & Constraints

Read `.kittify/charter/charter.md` and this mission's `spec.md`, substantive `plan.md`, `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Use their exact schemas and command contract. If the plan's canonical filename differs from a draft ownership path, resolve that discrepancy before changing scope; do not create a parallel authority.

This is an independent root; no feature implementation dependency.
Modify only this WP's owned surfaces and its required runtime status/evidence. Other WPs' states are not yours to edit. Preserve existing signed bytes, native permissions, defaults and public release/tag artifacts. Product/release tooling uses Go standard library and Git with no Python, mandatory service, provider CLI or new module dependency.

## Branch Strategy

- **Strategy**: Spec Kitty execution lanes.
- **Planning/base/integration target**: `feat/portable-collaboration`.
- **Public publication**: a reviewable PR; no implicit merge to public main, tag replacement, source deletion or release publication.
- Use the runtime-provided workspace and branch contract, not a manually invented lane.

## Subtasks & Detailed Guidance

### T004 — Define audit envelope/payload and permission contracts through tests

- **Purpose**: Establish the observable contract before product implementation.
- **Steps**: Cover distinct audit kinds, exact code/rule revision provenance, requester/executor identities, outcomes/findings/log bounds, rule-source authorization, explicit private authority anchor set/show over a named branch, old event byte compatibility and explicit disabled defaults. Derive assertions from spec and audit contract.
- **Files**: `audit_protocol.go`, `audit_protocol_test.go`, `audit_authority.go`, `audit_authority_test.go`, `event.go`, `event_test.go`, `store.go`, `quarantine.go`, `shallow.go`, `policy.go`, `policy_commands.go`, `docs/audits-v1.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T005 — Implement canonical audit validation and fetched admission

- **Purpose**: Implement the narrow capability behind its canonical ownership boundary.
- **Steps**: Keep audit kinds observational and separate from run.request/run.result. Reuse canonical event, policy, shallow dependency and quarantine paths so remote facts cannot bypass local permission/relationship checks. Bound hostile payloads and preserve immutable prior event bytes.
- **Files**: `audit_protocol.go`, `audit_protocol_test.go`, `audit_authority.go`, `audit_authority_test.go`, `event.go`, `event_test.go`, `store.go`, `quarantine.go`, `shallow.go`, `policy.go`, `policy_commands.go`, `docs/audits-v1.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

### T006 — Prove replication and non-authority behavior

- **Purpose**: Prove the capability and its refusal/recovery behavior independently.
- **Steps**: Add local/fetched parity and state-preservation tests including imported rules, unrelated workflow edits, unknown fields, mismatched subjects, oversized logs and revoked/invalid authority. Verify proposal readiness/acceptance stays unchanged with either passing or failing audit records; document wire/authority contract.
- **Files**: `audit_protocol.go`, `audit_protocol_test.go`, `audit_authority.go`, `audit_authority_test.go`, `event.go`, `event_test.go`, `store.go`, `quarantine.go`, `shallow.go`, `policy.go`, `policy_commands.go`, `docs/audits-v1.md`.
- **Parallel?**: Sequence within this WP. Cross-WP work only follows finalized lane/dependency allocation.

## Test Strategy

Use behavior-first tests through the responsibility-appropriate public boundary. Local provider fixtures may simulate documented APIs; do not mutate live user providers or fetch arbitrary content to make tests pass. Hostile/recovery refusals must preserve externally visible state. Run focused cases during development, then required formatting, `go test -race ./...`, `go vet ./...`, `go build ./...` and `git diff --check` before review as the charter requires. Include applicable ordinary/`nogithub`/adapter-free cases without conflating their meanings.

Do not weaken lint/check rules, replace negative assertions with implementation mirrors, skip a failing gate or claim tests for unexecuted code/hosts. Poll any pre-review gate until the command completes; do not kill or duplicate a running gate. Stage only expected deliverables.

## Risks & Mitigations

Treating audit results as existing run results could accidentally satisfy merge requirements. A rule digest is provenance, not proof that its signer owns the rules. The local private authority anchor is set explicitly to a named branch; `--policy-ref` is an assertion only and cannot select arbitrary candidate/old policy authority. Imports never install or alter the anchor. Current rule projection must isolate authorizations/revocations to the selected anchor authority history; a foreign or forked policy with the same public scope/rule identifiers must not poison current rule heads. Include that malicious-history regression alongside code-only branch advancement with identical policy bytes, which remains eligible; stale/candidate policies and owner withdrawal cannot authorize new work.

## Review Guidance

Reviewers must independently assess this WP's acceptance example and FR-001, FR-016, FR-018, FR-019, NFR-001, NFR-002 against actual runtime wiring, refusal behavior and fixed source. Confirm new code is reachable, capability claims match implemented directions, bounds are enforced before mutation, and no authority or credential is silently introduced. A provider reporting every resource unsupported does not satisfy coverage. Record concrete blockers rather than speculative or cosmetic churn.

## Activity Log

- 2026-10-26T10:30:30Z — system — Draft prompt authored for task finalization.

## Updating Status

Use `spec-kitty agent tasks mark-status T004 T005 T006 --status done` only with the canonical command's accepted argument shape. Use runtime-provided WP action/transition commands for implementation and independent review. Never author lane, review-status or completion truth in this Markdown.
