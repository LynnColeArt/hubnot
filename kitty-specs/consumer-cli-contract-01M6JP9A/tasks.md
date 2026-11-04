# Tasks: Consumer CLI contract

All WPs are sequential in one dependency lane. Canonical runtime owns all state.

## WP01: Versioned native envelope and static discovery
**Dependencies**: None
**Requirements**: FR-006, FR-007, FR-008, NFR-001, NFR-002, C-001, C-002, C-003
- [ ] T001 Define closed native envelope/error and static discovery at real run() boundary; document exact command/capability/schema names and empty collection shapes. Parse unknown flags before effects; keep unkeyed operation null and safe recovery guidance.
- [ ] T002 Test static discovery keyless outside Git using a hostile git shim and state snapshots; audit nine contribution json flags and actual always-JSON output without changing default bytes. Test discovery invalid flags and native-only compilation.

## WP02: Exact proposal read projections
**Dependencies**: WP01
**Requirements**: FR-001, FR-002, FR-003, NFR-001, NFR-002, C-001, C-002
- [ ] T003 Add optional JSON proposal show/status only after preserving original text handlers. Export full candidate/base/head/policy, current actor-keyed reviews from operative exact candidate history, requirements (including protected checks), evidence and pipeline results.
- [ ] T004 Expose readiness, disagreement, lineage, accepted/rejected/merged and terminal eligibility distinctly. Public CLI tests must include two actors with colliding display names, latest sequence, late predecessor vote, missing review count and no inherited successor evidence; verify default text snapshots.

## WP03: Full event log and bounded prefix resolution
**Dependencies**: WP02
**Requirements**: FR-001, FR-005, FR-008, NFR-001, NFR-002, C-002, C-003
- [ ] T005 Add log --json preserving raw log boundary and all absent-JSON bytes. Add documented verified read-only event show/resolve path using existing resolveEvent; preserve moderation/pending/shallow boundaries and all mutation full-ID guards.
- [ ] T006 Test full-ID/prefix roundtrip, invalid/not-found/unknown-flag errors and distinct native ambiguous_id using deterministic bounded resolver fixtures, no hash brute force. Prove reads append no events/load no signer or migrate identity.

## WP04: Exact native review and decision receipts
**Dependencies**: WP03
**Requirements**: FR-004, FR-008, NFR-001, NFR-002, C-001, C-002, C-003
- [ ] T007 Add review/decide --json receipts at existing append success, using exact stored event ID/Policy/Evidence. Keep absent JSON text bytes and all existing write guards. No operation keys, retry safety or lost-ack recovery added.
- [ ] T008 Test created IDs and evidence roundtrip through proposal status with proposal ID plus event read/resolver for evidence IDs. Test receipts remain exact after newer history; both verdicts and refusal/unknown-flag zero-effect paths through outer public CLI, including legacy identity and native-only workflow.

## WP05: Public integration and compatibility acceptance
**Dependencies**: WP04
**Requirements**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, NFR-001, NFR-002, NFR-003, C-001, C-002, C-003, C-004
- [ ] T009 Complete a real public CLI consumer journey and documentation/contribution consistency matrix covering every listed command JSON/default, eligible-reviewer/assignment distinction, errors, exact stored evidence and zero-effect boundaries. Keep full journey in native integration without broadening fast roots or deadlines; adding integration root requires documented bounded rationale.
- [ ] T010 Run focused race and native fast/static before review, then affected native integration/migration/protocol/trust/recovery before acceptance. Record exact head/binary/source bindings and limitations; retain baseline/failed red evidence. No releases, stress/soak or original checkout changes.
