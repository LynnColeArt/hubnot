---
work_package_id: WP11
title: Bounded submission exchange and pending intake
dependencies:
- WP10
requirement_refs:
- FR-016
- FR-017
- FR-020
- NFR-006
- NFR-007
- C-001
- C-007
planning_base_branch: feature/beta-readiness
merge_target_branch: feature/beta-readiness
branch_strategy: Planning artifacts for this mission were generated on feature/beta-readiness. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feature/beta-readiness unless the human explicitly redirects the landing branch.
subtasks:
- T035
- T036
- T037
- T038
phase: Optional unified contributions
history: []
agent_profile: implementer-ivan
authoritative_surface: contribution_exchange
create_intent:
- contribution_exchange.go
- contribution_exchange_test.go
execution_mode: code_change
owned_files:
- contribution_exchange.go
- contribution_exchange_test.go
role: implementer
tags: []
task_type: implement
tracker_refs: []
---

# WP11 — Bounded submission exchange and pending intake

## Objectives & Success Criteria

### T035

Implement a provider-free bounded artifact/reference exchange using the WP10 contract and existing safe archive/object primitives where applicable.

### T036

Isolate and validate submitted content before project admission, enforcing positive finite per-request bytes/object/event/count/depth/time and aggregate pending-retention limits.

### T037

Implement inspectable pending state, exact-intent deduplication and interrupted/replayed intake recovery with no branch, policy, signer or workflow side effects.

### T038

Prove early limit/cancellation behavior for signed spam, huge/truncated inputs, many-key aggregate saturation and unselected refs; record supported transport guarantees and exclusions.

## Context & Constraints

Read the active charter and mission spec, plan, data model and research in
`kitty-specs/beta-readiness-01M60J8C/`. This is the owner's explicit native,
optional unified-contribution amendment. Initial support/compatibility/recovery
work remains first. Load the declared profile through the canonical CLI/API.

Unknown contributors can prepare signed submissions with their own repositories/keys and hand off a small reference/artifact without upstream push access or a provider account. The receiving operator explicitly retrieves/stages it. File/reference handoff is sufficient for the first native public journey; no automatic notification network or hosted server is required. A submission reference is data, never a command or authorization to fetch arbitrary refs. Do not inherit compatibility-all or execute Git hooks/workflows from submitted material. Select limits before opening/contacting a source; bound streaming and decoded data and aggregate pending storage across actor keys. Existing post-fetch Git budgets cannot be presented as hard pre-download quotas. Prefer a deliberately bounded supported artifact; refuse unsupported unbounded ingress rather than promising resource safety. A signature alone cannot bypass limits. Staging is distinct from shared visibility and native authority; no imported policy/keyring/anchor is adopted. Implement idempotence from exact signed intent and durable receipts, preserving uncertain state on interruption. If a network transport is included, it needs its own early byte/time/endpoint boundaries; adding it is not needed to finish this WP.

## Ownership and delivery

Own only the declared files and this WP's evidence/status. Planning and merge
target is `feature/beta-readiness`; use the runtime-provided lane workspace.
Do not edit siblings, claim a dependent WP before approval, change existing
release assets or broaden product defaults. Additional integration ownership
requires a canonical planning amendment before edits. Keep original signed
facts and frozen prior-mission evidence byte-exact.

## Verification

Use controlled streams/local repositories and fault/cancellation seams to assert exact byte/count/disk budgets and no accepted-ref/branch/private-authority changes on refusal. Aggregate limits must survive many tiny submissions from different keys. Tests must fail for actual excess reads/retention, not only inspect configuration. Run formatting, full race, vet, build and diff checks at a fixed head.

Tests must exercise the promised behavior and detect the intended fault. Do not
add silent skips, weaken security assertions or use internal-only tests as proof
that public CLI integration works. Coordinate expensive full suites with root.

## Review Guidance

Review exact original-content/authority bindings, operation permissions,
default-off behavior, bounded intake before promotion, aggregate many-key
behavior, scoped reversible moderation, raw-history preservation and keyless
reconstruction. Distinguish signature validity, transport verification,
contribution admission and existing proposal acceptance. No missing observation
is a pass. Review the fixed committed head independently.

## Handoff

Commit only expected files and record commands/exits/evidence. Mark only this
WP's truly completed subtasks with the framework. Move to `for_review` and poll
the synchronous gate to completion; never skip/kill it or label no_coverage as a
successful Go run. No external comments, messages or release publication.

