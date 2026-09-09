# WP02 independent review — changes requested

Reviewer: codex-review-wp02, resolved reviewer-renata profile.
Reviewed source: 988a3f91890c79d0f75a75b5152dc2cf7d4132d4. Both owned source files are unchanged at current lane HEAD c6660b44.

## P2 — Preserve batch child failures and diagnostic causes

In issue_read.go, issueReadStoredEvents returns a readIssueBatchObject error immediately on a truncated header/body, and deferred cleanup discards cmd.Wait and stderr. The existing TestIssueReadChildFailure wrapper prints `failed child` to stderr and exits 9. An overlay-only diagnostic run on the unchanged source returns `invalid_history: malformed or truncated batch header`; both the actual exit status and diagnostic disappear. A repository/process failure therefore becomes a claim that signed history is invalid, and WP03 cannot distinguish or expose the actual operational failure.

T006 steps 5 and 7 require preserving detailed error causes and distinguishing repository errors from invalid history and resource limits. Capture the genuine child failure and bounded stderr during cleanup and return repository_error for a naturally failed child. Preserve resource_limit or malformed-response diagnoses when the reader deliberately cancels a still-running child; its induced kill must not overwrite the original reader error. Keep every child reaped and return no partial events/token.

Add a behavior assertion for the existing nonzero-child fixture that checks repository_error plus the child diagnostic/exit context. Retain parser framing and limit checks, and exercise any cleanup change on malformed and over-budget responses so the fix cannot hang or replace intentional reader errors with a cancellation error.

## Evidence

- Focused reader tests with race detector passed, including accepted refs, framing/budgets, signature/payload/actor/relationship failures, pending and corrupt admission, moving snapshot, attachments, child exit, and SHA256 Git: `go test -race -run 'TestIssueRead(EmptyAndSelectedRefs|ObjectFramingAndBudgets|RejectsCorruptionAndPending|CountAndByteLimits|SnapshotMovesDuringBatch|Attachments|ChildFailure|SHA256Git)$' -count=1 -v .` — hubnot 1.736s.
- Diagnostic-only Go overlay of existing child failure test, with no source mutation: `go test -overlay /home/lynn/Documents/Codex/2026-09-09/h/work/wp02-review/overlay.json -run '^TestIssueReadChildFailure$' -count=1 -v .` — hubnot 0.033s; error reproduced as above.
- `git diff 988a3f91 -- issue_read.go issue_read_test.go` empty. Implementation commit changes only the two owned files. `git diff --check` passed.
- Inspected full StoredEvent equality fixture and constant-count measurement; implementing-agent report supplied by root: 1000 issues old 6.018s/3002 Git calls versus batch 233ms/5 calls, plus full race 204.466s, vet/build/gofmt. Not rerun because unchanged and no additional full-suite concern.

## Required anti-pattern checklist

1. Dead code: N/A at this explicitly planned foundation boundary; production integration belongs to dependent WP03. All internal helpers are used by the foundation reader.
2. Synthetic-fixture tests: PASS. Tests exercise real reader/validator paths; framing cases call the production parser.
3. Silent empty return: PASS. Failure results return a typed error and no trusted events/token; ignored namespaces and empty repositories are intentional.
4. FR coverage: PASS for this WP's foundational FR-010, NFR-001/002, C-001/003 scope, subject to the diagnostic defect above; catalogue/CLI integration belongs to later WPs.
5. Frozen surface: PASS. Source commit touches only issue_read.go and issue_read_test.go.
6. Locked decisions: PASS. No new authority/cache/dependency, unrelated loader unchanged, signed payload validation reused.
7. Shared-file ownership: PASS. Two owned source files only. Inherited runtime-authored planning metadata is a known centrally coordinated workflow issue, not source contamination; no metadata deleted or restored.
8. Production fragility: PASS. Fail-closed rejection on invalid, denied, over-budget or moving histories is deliberate. Classification defect is captured above.

Verdict: rejected pending the P2 diagnostic fix. No source changes made by reviewer.
