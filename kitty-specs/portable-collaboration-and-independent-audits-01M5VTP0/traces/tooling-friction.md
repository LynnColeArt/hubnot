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
- 2026-10-26 — Several attached long-running Go race commands ended with external SIGTERM (143), without a Go failure or timeout report; the cause remains unproven. Dedicated detached process groups with source, PID, JSON output and atomic exit files completed the identical required commands successfully for WP01, WP02 and WP04. Interrupted runs remain incomplete evidence; no timeout or test requirement was weakened.
- 2026-10-26 — Mandatory pre-review transitions completed for WP01, WP02 and WP04, but their framework coverage authority reported `no_coverage`: `tests.architectural._gate_coverage` is unavailable in this Go repository. The runtime reported `blocked=false` and `force_bypassed=false`; no skip or force flags were used. This is distinct from each fixed-source Go race/vet/build/format/diff pass and is recorded as unavailable framework coverage, not a successful coverage check.
