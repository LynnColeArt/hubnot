# Independent WP14 review — cycle 2

Verdict: APPROVED after canonical rejection and correction
Reviewer: codex-independent-interop-review, resolved reviewer-renata; no implementation edits
Rejected source: 3ca117ae232c02b7c1da25bf06fffaa78d4b8974
Approved source: de5e67a646c05b440a15187c3c7a87ab9786f18b
Cycle 1 rejection event: 01M5RTRJW8ZZ0GP8DEXAP0FZ6H
Cycle 1 source-bound feedback: review-feedback-1.md, committed 93cea42d
Cycle 2 canonical invocation: a87f2e1cc13c440e8a1c948c83369f68
Scope: four owned optional internal/githubapi GraphQL/discussion files.

## R1 disposition: fixed and independently verified
Initial strictGraphQLJSON rejected exact duplicate keys but encoding/json's case-insensitive struct matching could overwrite errors or typed fields. Original independent error-envelope probe accepted a nonempty errors list followed by Errors:[]; the typed GetDiscussionByID probe accepted body plus BODY and returned manufactured body content. Both were RED at rejected source, with complete reproductions in cycle 1 feedback. The canonical seam rejected WP14 to planned and released its claim before reimplementation.

Fix de5e67a6 canonicalizes each finite Unicode SimpleFold class to its smallest rune for object-key collision detection at every nesting level, matching the decoder equivalence without quadratic pairwise comparisons. Reserved envelope data/errors/extensions additionally require canonical spelling even if supplied alone. Bounds and sanitized errors remain unchanged. New tests exercise ASCII/Unicode aliases in errors, body, IDs, answers, lock state and parent references; package code remains standard-library-only with no public API/document change. Independent original regressions are now GREEN. No remaining actionable finding.

## Completed source assessment
Full initial audit plus complete fix-delta audit covers explicit token gating for GraphQL reads/writes, validated root/enterprise endpoint derivation, inherited no-redirect credential protection, fixed headers, context cancellation and <=30-second requests. Responses are bounded to4MiB, traversal to100 pages per connection, aggregate to64MiB per public method and strict JSON to depth64. Duplicate/equivalent keys, malformed/trailing JSON, invalid UTF-8/surrogates and any nonempty GraphQL errors/partial data refuse before typed acceptance. Remote error bodies/tokens do not enter diagnostics. Every mutation method issues exactly one fixed variable-based mutation with no implicit retry/batch; returned immutable subjects and expected parent/category/content/state are checked.

Repository/categories/discussion metadata and selected full-thread methods retain opaque node IDs separately from numeric selectors. Complete thread hydration traverses independent top-comment and reply cursors with a shared byte budget, validates immutable parent/thread identities and refuses absent/contradictory selected answers without returning a partial thread. Listing intentionally returns metadata snapshots only. Nil author/edit-history state honestly represents absent or inaccessible attribution; poll/lock/reaction/history/deletion indicators support downstream explicit mapping reports. No claim of complete edit/deletion history or author impersonation is made. Native resources and provider write journaling remain downstream responsibilities.

## Verification
- Independent original package race PASS9.852s at initial source, while R1 envelope and typed-body probes each failed as expected (0.004s).
- Additional independent wrong-thread, wrong-parent, missing-answer and contradictory-answer probes plus runtime query-document matching PASS under race1.030s.
- All15 complete runtime query/mutation constants independently matched the scratch .graphql documents; external dev-only validate.cjs reported every document valid against the downloaded official GitHub public schema. Exact documents/SDK API are unchanged by the fix. Official reference: https://docs.github.com/en/graphql/overview/public-schema and downloaded schema.docs.graphql; scratch parser/schema remain outside the repository and product dependencies.
- Final complete independent overlay, including both original R1 regressions and nested/schema-document checks, PASS under race1.033s at de5e67a6.
- Final source/diff rechecked unchanged during canonical cycle 2. No reviewer production edits, live account mutations, parser install or product dependency changes.
- Orchestrator supplied exact-final full Go race PASS247.802s (githubapi11.433s), latest package race10.891s, targeted race1.028s, vet, default/nogithub builds, formatting/diff and dependency exclusion. Default generic for_review gate completed without force/skip; generic unavailable Python/JUnit coverage is not a Go regression.

## Required anti-pattern checklist
1. Dead code: PASS internal live graphQL/parser/pagination/conversion/mutation call paths. N/A deliberate public SDK seams exposed early to dependent WP11/WP12 under the approved WP split; downstream adapters must consume these APIs before full mission acceptance. No standalone user feature is claimed for unused client methods.
2. Synthetic-fixture tests: PASS. Real Client/HTTP parsing and typed methods consume local HTTP fixtures, with observed requests/results/errors; complete runtime documents are independently validated against the official schema.
3. Silent empty return: PASS after R1. Nonempty errors cannot be erased by aliases. Nil attribution/edit history and complete empty connections have explicit documented meanings; malformed/inaccessible data returns errors, no partial slices/threads.
4. FR coverage: PASS for FR-008 explicit access/redaction, SDK foundation of FR-017 stable discussion operations, NFR-001 bounded strict hostile input, NFR-002 gates and C-001 optional standard-library exclusion. Concrete native discussion transfer remains WP11.
5. Frozen surface: PASS. Only four owned new !nogithub SDK files; existing REST API/client and native signed bytes remain unchanged.
6. Locked decision: PASS after R1. No mandatory provider/core dependency, invented authority, raw secret errors, partial/error acceptance or mutation retry/idempotency claim.
7. Shared-file ownership: PASS. No root/native/shared REST implementation file edited. The parser import is standard-library unicode only; schema validator/npm files remain outside product.
8. Production fragility: PASS after R1. Uncertain/error mutations return sanitized errors; complete typed validation and explicit bounds replace ambiguous success. No internal mutation retries or execution path.

This approval closes WP14's SDK defect. Dependent root resource adapters and full expanded mission acceptance remain required.
