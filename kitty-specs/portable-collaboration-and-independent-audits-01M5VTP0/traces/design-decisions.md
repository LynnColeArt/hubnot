# Design Decisions

> Capture the rationale that would otherwise evaporate.

**Prompting questions**
- What decision was made?
- What alternatives were considered?
- What was the rationale — why this option over the others?

---

## Entries

<!-- YYYY-MM-DD — Decision: [what]. Alternatives: [what else]. Rationale: [why this one]. -->

- 2026-10-26 — Decision: distinct observational audit facts with an explicit private authority anchor, rather than broadening proposal CI facts. Decision: two-layer project snapshots (exact native history plus attributed foreign records). Independent review corrected authority freshness and explicit destination blob/receipt binding before implementation. Provider capabilities have non-vacuous minimums and explicit unavailable resources.

2026-10-26 · codex · Clarified the audit contract against existing shipped governance: request/import never executes target or rule-source hooks/pipelines/foreign workflow definitions, while operator-controlled destination reference-transaction hooks retain normal Git behavior (docs/protocol-v0.md304–306; governance-v0.md361–364). This preserves FR-001 defaults and avoids a process-global Git environment override or changing frozen audit foundations. The private prepared operation guard alone suppresses hooks because it publishes no fact. WP03 public tests and independent review must cover the distinction and state preservation on hook refusal.

2026-10-26 · Kat (implementer) · WP05 rework for review-cycle-1 P1 landed at lane-d 9ab00d85dee0fac114f303d5eecd2cd7e00f5cae: destination ref promotion anchored to the selected repository (bare/ordinary/linked-worktree); all 12 required fixed-source checks exit 0 including full race (1276 pass events, 0 fail); reviewer hook-probe rig now PASSES at this head; permanent 3-topology regression added in migration_native_test.go. Rehearsed evidence bundle at /tmp/wp05-checks-9ab/validation-9ab.tar.gz pending coordinator PRIMARY preservation before for_review. No gates bypassed; T013-T015 marked done after checks.

2026-10-26 · Kat (implementer) · WP05 P2 rework (review-cycle-4) landed at lane-d bc6c8db5f6292d53509c068b11302e91a80a3ce0: linked-worktree reciprocal pointer now must exist and verify back to the selected git directory; stale registration refuses before promotion, no hook runs anywhere. New TestMigrationDestinationStaleWorktreeRegression alongside healthy layouts. All fixed-source checks exit 0 at head: full race 1284 test-passes/0 fails (532.6s root pkg), focused 8.5s, internal 1.06s, nogithub/noforge variants, recovery probes, vet/build/gofmt/diff-clean. Cycle-3 open item (dead-code gate vs deferred WP06/WP11 wiring) intentionally NOT resolved; scope held to P2. Evidence: /tmp/wp05-checks-p2/validation-p2.tar.gz (19 members, SHA256SUMS verified) pending PRIMARY preservation.
