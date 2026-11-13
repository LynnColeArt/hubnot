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
base_branch: kitty/mission-native-automatic-proposal-checks-01M7AG85
base_commit: aca27abe22e374db603b0cc1659005c275381758
created_at: '2026-11-13T13:09:18.408160+00:00'
subtasks:
- T001
- T002
- T003
history: []
authoritative_surface: proposal_checks_execution.go
execution_mode: code_change
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
