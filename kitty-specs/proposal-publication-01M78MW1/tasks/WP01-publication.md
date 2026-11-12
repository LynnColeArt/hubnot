---
work_package_id: WP01
title: Native publication and recovery
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
planning_base_branch: feat/workbox-proposal-publication
merge_target_branch: feat/workbox-proposal-publication
branch_strategy: Planning artifacts for this mission were generated on feat/workbox-proposal-publication. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/workbox-proposal-publication unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
history: []
agent_profile: node-norris
authoritative_surface: proposal_publication.go
create_intent:
- proposal_publication.go
- proposal_publication_test.go
execution_mode: code_change
owned_files:
- docs/consumer-cli-v1.md
- proposal_publication.go
- proposal_publication_test.go
- main.go
- consumer_contract.go
- event.go
- store.go
priority: P1
role: implementer
tags: []
tracker_refs: []
---

# WP01 Native publication and recovery

Read the [spec](../spec.md) and [plan](../plan.md).

- **T001** Implement and advertise exact publication with signer, operation, and base-ref checks.
- **T002** Test native races, replay, repair, policy and legacy compatibility with disposable repositories.
