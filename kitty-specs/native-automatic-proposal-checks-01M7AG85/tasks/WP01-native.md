---
work_package_id: WP01
title: Native scheduling and evidence
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
planning_base_branch: codex/automatic-proposal-checks
merge_target_branch: codex/automatic-proposal-checks
branch_strategy: Planning artifacts for this mission were generated on codex/automatic-proposal-checks. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into codex/automatic-proposal-checks unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
history: []
execution_mode: code_change
authoritative_surface: proposal_checks_execution.go
owned_files:
- proposal_checks_config.go
- proposal_checks_scheduler.go
- proposal_checks_execution.go
- proposal_checks_provenance_test.go
- docs/automatic-proposal-checks.md
priority: P1
role: implementer
tags: []
tracker_refs: []
---
# Native scheduling and evidence

Review the native companion implementation, target provenance regression and paired browser/MCP verification.
