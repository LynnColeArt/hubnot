---
work_package_id: WP14
title: Bounded GraphQL and discussion provider client
dependencies:
- WP02
requirement_refs:
- FR-008
- FR-017
- NFR-001
- NFR-002
- C-001
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-optional-github-interop-01M5RK1Z
base_commit: ea90761f5a98d0a5e0286970ff3913982107e537
created_at: '2026-10-25T05:46:51.496386+00:00'
subtasks:
- T048
- T063
- T064
- T065
history: []
agent_profile: implementer-ivan
authoritative_surface: internal/githubapi/
create_intent:
- internal/githubapi/discussions.go
- internal/githubapi/discussions_test.go
- internal/githubapi/graphql.go
- internal/githubapi/graphql_test.go
execution_mode: code_change
lane: planned
owned_files:
- internal/githubapi/discussions.go
- internal/githubapi/discussions_test.go
- internal/githubapi/graphql.go
- internal/githubapi/graphql_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, primary spec/plan, and the full runtime prompt; resolve implementer-ivan canonically. Work only in runtime lane. Tests first, commit only owned source/tests, actual Go charter gates and default for_review, no self approval or public mutations. No external module/core provider dependency. No repetitive user permission question: full feature is authorized.

## Objective

Unblock all optional resource adapters with a small standard-library-only bounded GraphQL client and typed GitHub Discussion HTTP operations, independently of native migration engine. All files/tests use !nogithub. Existing REST client behavior and old DTO API remain unchanged. Do not edit root resource/command code.

## Transport contract

Add reusable package-private or exported GraphQL query/mutation method over the validated Client. GitHub.com endpoint /graphql; enterprise /api/graphql from known REST /api/v3, not /api/v3/graphql. Preserve same-origin/no-redirect credential safety and fixed headers. Explicit safe endpoint configuration only if needed; no arbitrary untrusted response URL fetch. Auth required even public GraphQL reads. Context cancellation, <=30seconds/request,4MiB response,100pages/connection,64MiB method aggregate; caller5min transfer context retained. Reject malformed/trailing/duplicate/invalid Unicode JSON and any GraphQL errors or partial data, including HTTP200. Sanitize all error messages; no raw error body/secret leak. Never auto-retry mutations. clientMutationId is correlation only, not idempotency. Expose typed operation DTOs/methods and stable API early to downstream adapters.

Connections require first/last1..100, independent cursor traversal for nested comments/replies/categories and no cursor cycles/missing endCursor or silent truncation. Empty/deleted authors become honest absent source attribution, not fake accounts. Any unsupported/inaccessible source data must be visible to caller so migration can report it before native writes.

## Discussion HTTP contract

Use fixed GraphQL documents with variables for user data. Read repository metadata/IDs, discussion bynumber/nodeID, paginated discussion listings/categories, full selected thread comments and replies with stableIDs, parentID, title/body/category/author/times/state/answer/lock indicators. Fetch enough unsupported-capability indicators (poll/reactions/lock/history limitations) to let root mapping report them honestly. Do not claim full edit/deletion history or impersonate authors.

Typed mutations: create/update discussion, add/update comment/reply, mark/unmark answer, close/reopen. Optional lock operations only if downstream supported, no invented category create. Each method represents exactly ONE remote mutation, validates returned immutable subject identity/required fields, no internal implicit batch or retry. Parent/category IDs explicit; native migration will journal every call and map source attribution. Source selectors owner/repo for repository discussions, numeric discussion selector separate from opaque nodeIDs.

Verify exact operations/input/field names against current OFFICIAL GitHub schema. Root downloaded schema and installed a DEV-ONLY parser outside repo: /home/lynn/Documents/Codex/2026-10-24/wa/work/graphql-validation/github-schema.graphql and validate.cjs. Run `node /home/lynn/Documents/Codex/2026-10-24/wa/work/graphql-validation/validate.cjs <scratch-document.graphql>...` on complete documents. No npm/GraphQL dependency enters Hubnot. Official schema source https://docs.github.com/en/graphql/overview/public-schema (download https://docs.github.com/public/fpt/schema.docs.graphql). Save exact validation evidence/query documents under work scratch or source-bound test fixtures where useful; do not merely invent fake-server query shapes.

## Tests and validation

Use httptest local provider and real Client security paths. Prove GraphQL envelope/partial-error refusal, redaction, token gating, endpoint route, cancellation, budgets, complete nested pagination and stable IDs; mutations never retry. Client unit tests against real-shaped official responses and schema-validated documents. Validate category/answer IDs, deleted-author attribution, wrong returned IDs, unknown required types/cursor cycles, malicious/overlarge responses, and no partial slice returns. No live account token is necessary.

T048 Add bounded sanitized GraphQL transport and complete nested pagination tests.
T063 Implement typed discussion/category/thread/reply reads with stable source identities and honest capability indicators.
T064 Implement one-step typed discussion/reply/answer/state mutations without automatic retries.
T065 Validate complete GraphQL documents against official schema and run scoped/full/tagged quality gates; expose API to adapters.

## Delivery

Send stable source commit/API promptly for independent read-only review while full gates run. Mark subtasks and canonical for_review after actual Go race/vet/build/format/diff/tagged exclusion checks; poll generic gate. No force/skip/self approval. Keep client package independent of native resource models and services.
