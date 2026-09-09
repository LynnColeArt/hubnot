# Tooling Friction Log

> Log every place the tooling fought you so it can feed the tooling-gap backlog.

**Prompting questions**
- What tooling or command did you have to work around?
- What blocked you unexpectedly, and how long did it take to unblock?
- Was this a known issue or something discovered fresh?

---

## Entries

<!-- YYYY-MM-DD — 1-3 sentences: what happened, why it slowed you down. -->

2026-09-09 — Log seeded during WP03 from contemporaneous host command logs; the earlier events below are reconstructed, not contemporaneous entries. Mission creation first hit protected-target/HEAD guards; partial attempts were preserved outside the repository and the successful mission targets a dedicated feature branch.

2026-09-09 — Runtime-inherited planning/status files triggered lane ownership hygiene on both readiness and approval. The host verified source ownership, fixed commits, tests and independent verdicts, then used the documented force exception only for that metadata check; no acceptance or independent review was bypassed.

2026-09-09 — A normal Go build produced a root executable that the runtime later auto-committed with lane deliverables. It was removed in a separate explicit cleanup commit; later build outputs go outside the checkout. Runtime review overlays were also moved outside source worktrees.

2026-09-09 — Coordination diverged from the planning branch during runtime status updates. The scoped doctor correctly refused an automatic divergent fix; an ordinary merge preserved both histories and the event-log merge driver. Hosted sync warnings did not stop local commands; no hosted passing evidence is claimed.
