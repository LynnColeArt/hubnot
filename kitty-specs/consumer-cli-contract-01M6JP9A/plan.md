# Implementation Plan: Consumer CLI contract

## Summary
Add native optional consumer projection/receipts, static discovery and verified event read resolution, preserving every default command path. No event schema/migration or policy change. All five WPs execute in one dependency lane to keep shared dispatch/projection edits coherent; independent review after each WP.

## Technical Context
Go module/package main with colocated tests. Existing main.go dispatch acquires contribution locks before most parsing. proposal.go owns textual candidate views/currentReviews; policy.go computes native evaluation and protected checks; governance.go owns exact stored decision creation; store.go owns resolver. Reuse existing envelope/error vocabulary from docs/extensions.md; contribution commands already always emit JSON.

## Components and control flow
```
static version/capabilities -> static constants -> JSON envelope (no repo/lock)
JSON proposal -> existing verified/shallow/operative selection -> evaluateProposal -> pure projection -> envelope
JSON log/event -> existing governed/raw selection appropriate to current boundary -> resolveEvent -> projection
review/decide -> existing mutation guards and signer -> appendEvent -> exact stored receipt
absent --json -> existing handler/output unchanged
```
Export full identities and latest per-actor reviews only from contribution-operative exact candidate history. Keep raw log's existing raw-history selection explicit. Protect hidden/admission evidence consistently; no more authority than existing selected history. JSON show adds evaluation and may refuse incomplete dependencies; absent JSON keeps existing shallower path.

## Contract decisions
A versioned native consumer schema advertises explicit capability IDs; documented fields include full candidate/base/head/policy, actor-keyed current reviews, reviewer pool/approval threshold/deficit, satisfied/missing requirements (including protected checks), pipelines/evidence, lineage, readiness/disagreement, acceptance/merge observations and terminal eligibility. Eligibility reflects actual known native guards and is not a merge success guarantee. Stored receipt uses stored event Policy/Evidence only. Errors use closed origin/code/message/recovery/operation, no invented vocabulary; unkeyed commands use operation:null and do_not_retry where replay safety is unavailable. Resolver ambiguity remains native ambiguous_id. Mutation commands/artifacts retain full IDs. Contribution output audit never wraps existing output.

## Work package sequence
WP01 contract/discovery/envelope -> WP02 proposal projections -> WP03 event/log/resolution -> WP04 exact review/decision receipts -> WP05 integrated compatibility/authority/docs verification.

## Verification
Each behavior is public CLI test-first with red evidence at stable entry point, then focused race, native fast/static before independent review. Cover colliding names, latest sequence, late predecessor review, missing count, protected checks, empty slices, errors/unknown flags, prefix ambiguity through bounded fixture, keyless discovery with hostile Git shim, both JSON/non-JSON, exact stored decisions after newer evidence, and outer-dispatch zero-effect refusal including legacy identity. Full integration journeys belong native integration; preserve existing 21/8 root membership and deadlines. Before acceptance run native integration plus affected migration/trust/protocol/recovery selections. Record source commit, actual binaries/tags, command exits. No broad stress/soak or release certification (#38 remains open).

## Risks
Avoid recomputing decision receipts, using display names, inheriting predecessor votes, omitting protected-check deficits, treating reviewer pool as assignment list, claiming Ready implies terminal eligibility, mapping unkeyed failures to safe retry, or acquiring locks during static discovery. No global gate relaxation: framework no_coverage unsupported Python gate stays honestly documented.
