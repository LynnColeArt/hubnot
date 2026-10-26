# Tooling Friction Log

> Log every place the tooling fought you so it can feed the tooling-gap backlog.

**Prompting questions**
- What tooling or command did you have to work around?
- What blocked you unexpectedly, and how long did it take to unblock?
- Was this a known issue or something discovered fresh?

---

## Entries

<!-- YYYY-MM-DD — 1-3 sentences: what happened, why it slowed you down. -->

- 2026-10-26 — Hosted sync warns about missing auth/locked routing, but local canonical operations work; no hosted success is claimed. Task validation exposed missing future-doc create intents and a lane dependency cycle caused by shared final-stage docs. Narrowed ownership to separate archive, engine, CLI and integration guides; canonical finalization now passes with eleven acyclic lanes. The implementation guard requires a durable analysis report before claims.
