---
affected_files: []
cycle_number: 1
mission_slug: portable-collaboration-and-independent-audits-01M5VTP0
reproduction_command:
reviewed_at: '2026-10-26T15:50:09Z'
reviewer_agent: user
wp_id: WP03
---

# WP03 independent review — changes requested

Reviewed source: `20923906ba24c939309540626d525a75729fbabe`.
Approved dependency: WP02 `6d687af50562e8d837915d39a97d82727e91fd71`.
Contract authority: PRIMARY `contracts/audits-v1.md`, including hook clarification `f4c8329b`.

## [P2] Validate normalized finding identities before follow-up mutations

Affected code: `audit_followup.go:70–85,102–135`.

The approved signed-result schema accepts raw-unique finding keys such as `X` and `x` (or whitespace variants). `followUpAudit` normalizes them to the same scope/rule/key identity but keeps both entries in `selected`. With different titles/bodies, the first public `hn audit follow-up RESULT --operation normalized-collision --json` creates a native issue, then returns `operation_conflict` when the second entry reuses the deterministic issue operation with different content. This is deterministic input ambiguity, reported as an interruption after a visible mutation. Retrying returns two associations for the same issue ID/identity with `multiple_associations:false`.

The independent probe creates this accepted signed result through production `AuditRequest` and `RecordAuditResult`, and uses the compiled ordinary CLI for follow-up. It observed issue count `0 → 1`, one accepted actor ref before/after with its OID changed, a first CLI exit 1, and duplicate association entries on retry. `--finding x` on the same snapshot already refuses as ambiguous before accepted-ref mutation.

Normalize and validate the complete selected finding set before `bindAuditFollowUp` or any native issue write. Refuse ambiguous normalized identities with an actionable error and unchanged accepted refs, issue counts, and private intent state. Preserve original signed finding bytes and the frozen WP02 schemas/APIs. Add a public regression using an otherwise-valid signed result containing normalized collisions, rather than only testing producer-side structured-output validation. Also retain cross-result scope/rule/key dedup, authenticated-link checks, and original-selection recovery.

Evidence: `review-20923906/public-probes-final.log` and `review-20923906/probes_test.go`. The reproduction test intentionally fails when it detects the mutation; this is new independent defect evidence, separate from the previously passing full suite.

## Verified checks and limits

- Implementation archive SHA-256 verified: `ca48a75c7df891984782028da7606b3c535f8b970dc99107732dd918ec035ae9`.
- All seven current source hashes match the archive manifest. Archived final `exit.json` binds source before/after to `20923906`, and raw race log reports `ok hubnot 524.365s`; race/vet/build/nogithub help/format/diff exit codes are 0. The unchanged full suite was not duplicated.
- Independent existing focused race checks passed: operator destination reference hook plus inert sources; invalid UTF-8 refusal; transport and mid-run authority withdrawal; client termination releasing the prepared guard; interrupted native follow-up recovery preserving operator edits (`7.765s`).
- Additional independent probe passed: a signed issue with a false request link does not suppress creation; genuine duplicate associations are exposed; identities remain separated by scope/rule.
- No process-global Git hook/config override or product edit was made. Real destination reference-transaction hooks retain Git behavior; the private uncommitted guard suppresses hooks. No guard ref is committed. Frozen WP02 implementation files were not modified by either WP03 commit.
- Shared-file ownership: `docs/audits-v1.md` deliberately extends WP02 documentation in the sequential shared lane; the other six new WP03 files are exclusively owned. Root coordination was notified before verdict.
- Runtime evidence is Linux/amd64. Host execution is explicit. The sandbox default/selection boundary is reused, but this review does not claim additional sandbox or other-platform runtime qualification. `noforge` is not yet defined in this lane and no adapter-free claim is made. Framework `no_coverage` for missing `tests.architectural` is separate from actual Go results and is not bypassed.

## Required anti-pattern checklist

1. Dead code: PASS — registered `localCommands["audit"]` reaches production command, execution and follow-up functions; helper call sites are present.
2. Synthetic-fixture test: PASS — existing public tests execute compiled CLI and real Git repositories; the new probe signs accepted facts via production writers and invokes the compiled CLI.
3. Silent empty return: PASS — empty collections represent documented no-findings/history states; read and runner errors propagate; no unexplained swallowed failure found.
4. FR coverage: FAIL — FR-020/FR-018 accepted historical-result follow-up handling has the normalized-collision defect above. Other named execution, authority, explicit opt-in, bounds and docs behavior is exercised.
5. Frozen surface: PASS — reviewed delta is exactly seven owned files; no frozen WP02 code or historical fact modification.
6. Locked decision: FAIL — deterministic ambiguous finding identity is not refused before follow-up mutation, contrary to bounded validated follow-up/refusal behavior.
7. Shared-file ownership: PASS — explicit WP02→WP03 sequential documentation extension recorded above; no uncoordinated ownership change.
8. Production fragility: PASS — explicit Go error returns preserve fail-loud runner/authority/refusal behavior; no new panic or unexplained transient exception path found. The deterministic post-write error is captured in finding 1.

Verdict: reject to `planned` with this concrete feedback; no approval claim.
