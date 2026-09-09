# Design Decisions

> Capture the rationale that would otherwise evaporate.

**Prompting questions**
- What decision was made?
- What alternatives were considered?
- What was the rationale — why this option over the others?

---

## Entries

<!-- YYYY-MM-DD — Decision: [what]. Alternatives: [what else]. Rationale: [why this one]. -->

2026-09-09 — Log seeded during WP03 from the committed spec and plan. The opening event remains the stable issue identity; immutable full-state revisions form a DAG, and every maximal head remains visible. This avoids clock-based conflict winners and keeps old signatures meaningful.

2026-09-09 — Rich state remains optional and descriptive. Status, assignment, criteria and metadata do not grant execution, review or integration authority; consumer-specific Go Kitty behavior remains outside Hubnot.

2026-09-09 — Operation replay is actor-scoped and binds signed intent plus canonical semantic content. Replay lookup precedes mutable head/snapshot checks so a lost response can be retried after intervening edits without appending another fact.

2026-09-09 — Complete resolution is the default, with explicit snapshot-guarded partial resolution for conflicts exceeding the 200-parent cap. Staging consumes only named current heads and preserves the remaining alternatives.

2026-09-09 — A dedicated batched reader preserves signature/admission validation while avoiding per-event object subprocesses. Its read model is rebuilt in memory; signed Git histories remain durable authority.
