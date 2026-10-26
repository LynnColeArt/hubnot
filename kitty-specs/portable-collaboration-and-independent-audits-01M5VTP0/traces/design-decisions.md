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
