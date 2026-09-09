---
affected_files: []
cycle_number: 1
mission_slug: rich-issues-reliable-automation-01M22EG0
reproduction_command:
reviewed_at: '2026-09-09T07:22:28Z'
reviewer_agent: codex-review-wp01
wp_id: WP01
---

# WP01 independent review — changes requested

Reviewer: codex-review-wp01 (Reviewer Renata; independent agent)
Reviewed implementation: 74126762e4f6f1d2df1219a9a142e23adedc308e. The eight owned Go source/test paths remain byte-identical to this commit during review.

## Blocking finding

**[P2] Enforce the rich-event size bound on the actual signed payload.** `event.go:100-128` decodes the incoming signed JSON before calling content validation. `issue_model.go:224-229` measures `json.Marshal(e)`, which drops unknown fields and whitespace rather than measuring the authenticated payload. A validly signed rich `issue.open` of 262572 bytes is accepted by `verifyEvent`, despite the documented 262144-byte cap. The reproduction appends an unknown string field with 262144 bytes to an otherwise valid rich opening, signs those exact bytes with its actor key, and expects verification to reject it. Verification currently succeeds. Whitespace padding can bypass the same re-encoding measurement.

Apply the raw signed-payload cap conditionally to rich issue extensions / `issue.revise` in verification, preserving legacy issue title/body acceptance. Keep the existing pre-signing canonical encoded-size checks. Add a regression through `verifyEvent` proving signed oversized rich input is rejected and legacy oversized signed input remains readable. No wholesale decoder rewrite is requested.

## Validation and scope

- `go test -race ./... -run 'Test(RichIssue|Issue)' -count=1`: PASS, hubnot 1.455s.
- Independent temporary-overlay `TestReviewOversizeRawRichPayload`: FAIL as expected, reporting accepted 262572-byte signed payload (cap 262144). Tracked source files were not edited.
- `git diff --check`: PASS.
- Implementer reports full race suite 172.217s, vet and build passing; this review did not duplicate the full race suite.
- Reviewed appended Event field ordering and legacy fixture, all-or-neither operation metadata and semantic digest, per-kind shape checks, iterative lineage/cycle traversal, conflict heads and 205-head staged reconciliation, graph provenance, local candidate guards, generic store integration, exact dependency closure and supplier quarantine/recovery.
- `shallow.go` already calls shared `replicationEventReferences`; issue references reach its exact closure check without editing the file.
- Public catalog/candidate/GraphFor entry points are intentionally delivered for dependent WP03 CLI integration. Canonical state/digest validation and reference validation already have live production callers. CLI exact-head/snapshot checks and actor CAS remain WP03 scope.

## Required anti-pattern checklist

1. Dead code: PASS for live validation and replication wiring; N/A for explicitly staged WP03 catalog/graph/candidate API consumers, per package scope.
2. Synthetic fixture test: PASS. Model/projection fixtures invoke production validation, DAG projection and closure; replication test exercises actual signed Git histories and promotion.
3. Silent empty return: PASS. Empty catalog/edge collections are defined results; invalid inputs return errors.
4. FR coverage: FAIL for NFR-001 raw encoded signed-payload cap, as reproduced above. Other assigned domain behavior is covered at this foundation layer; public CLI guards/replay are assigned to WP03.
5. Frozen surface: PASS. Source implementation commit touches only its eight owned Go files, no historical evidence or signed fixtures.
6. Locked decision: PASS for immutable facts, explicit conflicts, descriptive state and authority boundaries; required bound failure is recorded above.
7. Shared-file ownership: PASS. Eight changed implementation/test paths belong to WP01. Inherited coordination metadata is not lane-authored source and was not restored or deleted.
8. Production fragility: PASS. New errors reject invalid signed state/dependencies; no new transient-race panic or execution authority.

Verdict: rejected; fix the raw rich-event payload cap and resubmit for independent review.
