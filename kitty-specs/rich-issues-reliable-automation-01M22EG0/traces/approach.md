# Approach Evolution

> Track how your approach changed as the mission progressed.

**Prompting questions**
- What approach did you start with (as stated in the spec or plan)?
- What changed during implementation, and why?
- What would you try differently on a similar mission?

---

## Entries

<!-- YYYY-MM-DD — 1-3 sentences: what approach was tried and what shifted. -->

2026-09-09 — Log seeded during WP03. The committed approach has four packages: signed model/projection, bounded verified reader, public commands, then independent consumer acceptance and documentation. Foundations were implemented separately and each went through independent review and a corrective cycle before CLI integration.

2026-09-09 — Independent reviewers exposed two boundary defects: sizing re-encoded JSON missed excess raw signed bytes, and parser failure could conceal a naturally failed Git child. Both fixes received failing reproductions, focused verification, full race gates and independent re-review; the public dependency interfaces stayed stable.
