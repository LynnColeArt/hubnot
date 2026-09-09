# Data model

- Issue: stable opening event ID, attributed creator, one or more current revision heads, ordered comments.
- Issue state: title, body, optional criteria (id, text), labels, assignees, open/closed status, typed relations (blocks, related, parent), namespaced string metadata. Limits are part of the protocol contract.
- Revision: signed issue.revise fact naming the issue root, sorted unique parent revision IDs (up to200 per event; explicit snapshot-guarded staged resolution handles larger head sets), and a full issue state. Root is the initial revision. Multiple heads imply conflict; state is absent when unresolved and head states remain inspectable.
- Operation: optional signed actor-scoped key on opening/revision/comment facts, with signed operation intent and a canonical request digest excluding transport sequence and timestamp. Same key and request replays the same event; divergent reuse is rejected.
- Projection: verified roots, revisions, heads, relation and comment indexes, deterministic ordering and a snapshot token derived from the observed accepted actor refs. Derived state conveys no execution permission.
- Relation: source issue state links a full target issue ID and one of blocks/related/parent. Targets must be available, self-links are invalid. Cycles in blocks/parent are diagnosed; local mutations refuse introducing them. Concurrently formed cycles remain visible and block an unqualified readiness answer.

Trust: valid signature establishes attribution only. Assignment, criteria and issue closure do not authorize execution, review acceptance or integration. Replication selection continues to determine admitted histories.
