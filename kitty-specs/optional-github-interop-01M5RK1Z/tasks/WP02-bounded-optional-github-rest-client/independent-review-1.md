# Independent WP02 review

Verdict: APPROVED after pre-handoff correction
Reviewer: codex-independent-interop-review (resolved reviewer-renata; not the implementer)
Final reviewed commit: bb36153bcdfc6e8421684ae6660cc946218b06fd
Base commit: ea90761f5a98d0a5e0286970ff3913982107e537
Canonical review invocation: 27cec7fc594a4de18c6c6ddcc49efd31
Scope: internal/githubapi/client.go and client_test.go only.

## Findings and disposition
R1, medium, accepted and fixed before canonical handoff: original commit 45180252 compared Link parameter names and registered relation values case-sensitively. Valid `rel="NEXT"` and `REL="next"` each returned successful incomplete lists (one row/request rather than two). Independent Go-overlay HTTP probes reproduced both failures. RFC 8288 requires case-insensitive registered relation comparison and parameter-name case normalization: https://www.rfc-editor.org/info/rfc8288/ . Fix ca404417 adds EqualFold comparisons and regression tests; independent probes pass on final head. The separate whitespace correction 63d69543 also prevents valid `rel = "next"` truncation and rejects empty ports/fragment-bearing links. Final additive BaseURL getter and host/slash canonicalization tests at bb36153 were inspected and independently rerun. No remaining actionable findings.

## Acceptance assessment
FR-003/FR-006 provider reads/writes, FR-008 explicit authenticated access and NFR-001 bounded hostile input have real httptest requests against production client methods. Client construction makes no requests. Repository/number validation and HTTPS-or-explicit-loopback restrictions occur before sending. Redirects are refused, pagination stays on the exact endpoint/query and sequential page, failures return no partial lists, POST/PATCH have no application retry, and error diagnostics omit response bodies/headers/tokens. Response body, aggregate listing, page count, header size and per-request time are bounded. JSON is one value with identity/shape validation. Raw malformed UTF-8, lone high surrogate and lone low surrogate were independently verified rejected before DTO decoding rather than repaired silently.

## Verification
- Independent full package run at original reviewed client: PASS (1.022s).
- Independent package race after Link fixes: PASS (10.374s); package vet PASS.
- Independent final-head overlay tests for mixed-case Link relations, malformed source Unicode plus TestCanonicalBaseURLForStableSourceKeys: PASS (0.006s).
- `go list -tags nogithub -deps .` contains no hubnot/internal/githubapi.
- Implementer gates accepted by orchestrator: full repository race PASS (hubnot 250.979s), final package race PASS (9.864s), vet/build/format/diff and transport exclusion PASS. Generic for_review gate completed without skip/force; its baseline no-JUnit/Python coverage authority is unavailable, not a Go regression.
- Rechecked final lane head and reviewed files against bb36153: no diff.

## Required anti-pattern checklist
1. Dead code: N/A at this intentionally staged transport-contract boundary. WP02 owns only the client package and explicitly forbids root routing; WP03 is the declared production coordinator dependency and must wire these exported methods before mission release. Internal request/validation/pagination helpers have production callers. This approval does not treat the isolated client as a shippable user feature.
2. Synthetic-fixture tests: PASS. Actual HTTP handlers receive requests from production methods; malformed/redirect/limit assertions would fail without the client implementation.
3. Silent empty return: PASS. Empty lists mean complete empty provider collections; invalid status/shape/pagination/limits fail with nil slices and errors.
4. FR coverage: PASS at provider-contract level; native issue translation is explicitly assigned to dependent WP03/WP04.
5. Frozen surface: PASS. Only owned new package files changed; no protocol/history/core schema changes.
6. Locked decisions: PASS. Standard library only, explicit optional invocation, no gh binary/service/database dependency, no credentials in diagnostics, nogithub exclusion.
7. Shared-file ownership: N/A. Files are exclusive to WP02; BaseURL accessor was explicitly coordinated with orchestrator to avoid duplicated source normalization.
8. Production fragility: PASS. All invalid/unavailable/transient responses fail explicitly; no swallowed errors or unbounded fallback/retry found.

## Remaining integration obligations
WP03/WP04 must validate native content limits, preserve source attribution, bind immutable remote identities and exact write results, and journal ambiguous writes. The client deliberately does not claim that a signed import is an original external-author signature or that HTTP writes have distributed exactly-once semantics. Final public documentation and executable acceptance remain WP05 requirements.
