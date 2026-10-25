---
affected_files: []
cycle_number: 1
mission_slug: optional-github-interop-01M5RK1Z
reproduction_command:
reviewed_at: '2026-10-25T06:07:57Z'
reviewer_agent: codex-independent-interop-review
wp_id: WP14
---

# Independent WP14 review feedback

Verdict: REJECTED / changes required
Reviewer: codex-independent-interop-review, resolved reviewer-renata; no implementation edits
Reviewed source: 3ca117ae232c02b7c1da25bf06fffaa78d4b8974
Canonical invocation: f98f132ff3bc4b728d9b6be4656f2039

## R1 — P2: reject JSON field aliases before decoding GraphQL data
Location: internal/githubapi/graphql.go, strictGraphQLJSON object-key checks and graphQL envelope decoding (approximately lines54-65 and93-105).

The streaming parser rejects only exactly equal JSON keys, but encoding/json matches struct fields case-insensitively. A response such as `{"data":{"ok":true},"errors":[{"message":"private failure"}],"Errors":[]}` passes strictGraphQLJSON, then Errors overwrites the original nonempty errors list in the envelope struct. graphQL returns success and permits partial/error data to be treated as confirmed. The ERRORS:null variant also needs refusal. This violates the explicit HTTP200 GraphQL errors/partial-data contract and NFR-001.

The same ambiguity reaches typed data: a valid Discussion response with `"body":"body","BODY":"manufactured"` is accepted by GetDiscussionByID with Body="manufactured". Exact duplicate checks must cover decoder-equivalent field names throughout the response, including nested identity/parent/answer/state fields, and envelope reserved keys must not be bypassable by noncanonical aliases. Keep bounded depth/bytes and sanitized diagnostics. Add regression tests that exercise production transport/typed methods and show RED at this source, then GREEN with the fix; no query/SDK API or product dependency expansion is needed.

Independent reproduction overlay outside production:
- /home/lynn/Documents/Codex/2026-10-24/wa/work/interop-review/graphql-overlay.json
- /home/lynn/Documents/Codex/2026-10-24/wa/work/interop-review/graphql_review_test.go
- TestIndependentGraphQLCaseAliasErrorsRefused: FAIL0.004s (error envelope accepted).
- TestIndependentDiscussionTypedCaseAliasRefused: FAIL0.004s (manufactured body returned).
Command: go test -overlay <above overlay> -count=1 -run '^TestIndependent(GraphQLCaseAlias|DiscussionTypedCaseAlias)' ./internal/githubapi

## Completed review and other evidence
Full source audit of all four owned files is complete; no other actionable finding identified. Authentication, endpoint derivation (/graphql and enterprise /api/graphql), no redirects, response/aggregate/page/depth bounds, shared nested budget, Unicode refusal, sanitized HTTP/GraphQL errors and no mutation retries otherwise align with the contract. Full thread retrieval hydrates every paged top-level comment and each paged reply connection, validates parent/thread IDs, refuses missing or contradictory answer claims and returns no partial thread. ListDiscussions is explicitly metadata-only. Each mutation performs one typed operation and checks returned subject/content/parent/category/state as applicable.

Independent package race PASS9.852s. Additional wrong-thread, wrong-parent, absent-answer and contradictory-answer-flag probes plus exact runtime-document matching PASS under race1.030s. All15 complete runtime query/mutation documents were independently matched to scratch files and validated successfully with the external dev-only validator against the downloaded official GitHub public schema. No GraphQL parser/npm dependency enters the product. Root reports actual default/full/scoped/build/vet/tagged gates passing, but the independent R1 regressions block approval regardless of those gates. No live account mutations were made.

## Required anti-pattern checklist
1. Dead code: PASS internal helper call paths; N/A deliberate staged public SDK surface exposed to dependent WP11/WP12 under the approved split. Downstream adapters must consume those methods before mission acceptance; no standalone user feature is claimed here.
2. Synthetic-fixture tests: PASS. Local HTTP fixtures invoke real Client parsing/transport/typed methods and observable request counts; independently schema-checked documents avoid invented query shapes.
3. Silent empty return: FAIL through R1: differently cased alias can erase a nonempty GraphQL errors list and return success. Honest nil authors/edit-history pointers and empty complete connections otherwise have explicit meanings.
4. FR coverage: FAIL NFR-001/error refusal due R1. FR-008 explicit token/redaction, SDK portion of FR-017, NFR-002 gates and C-001 optional standard-library exclusion otherwise have evidence; native discussion transfer remains downstream.
5. Frozen surface: PASS. Four owned new !nogithub SDK files only; REST client/source behavior and signed native data remain unchanged.
6. Locked decision: FAIL R1 contradicts mandatory rejection of ambiguous/malformed/error GraphQL responses. No provider dependency enters core.
7. Shared-file ownership: PASS. No shared root/native/client file edited; only declared new SDK files.
8. Production fragility: FAIL R1 permits an uncertain/error mutation envelope to be reported successful. Other refusal paths are explicit and sanitized, with no automatic mutation retry.

Reimplement R1 through the canonical workflow, rerun scoped regression/race/vet/build/exclusion/schema checks, and return a fixed source for independent recheck before final exact-source full gates and approval.
