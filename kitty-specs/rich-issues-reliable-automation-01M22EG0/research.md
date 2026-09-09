# Research: Rich Issues and Reliable Automation

## Evidence and decisions
The prior Hubnot/Beads evaluation and direct source inspection establish that Hubnot currently has signed issue.open and issue.comment facts, Git actor streams, and no structured mutable issue catalog. Existing source at main a4d4bbfb9ba7275275b8d1bb281cf988baadeb9f is the baseline. Existing tests passed in the evaluation (go test, go vet); these are baseline observations, not acceptance evidence for this mission.

1. Evolve issue identity: the issue.open event ID remains the stable issue ID. Add optional rich fields and signed successor revisions; do not create a competing work object.
2. Preserve concurrency: revision parents form a DAG. Multiple maximal revisions are a conflict, never an automatic timestamp winner. Explicit resolution consumes all currently observed heads. Any accepted signed actor may contribute; author attribution is visible and conveys no execution authority.
3. Full-state revisions simplify audit and conflict resolution. Optional criteria, labels, assignees, relationships and namespaced metadata support ordinary work as well as automation. Open/closed is informational.
4. Idempotency is actor-scoped: a signed operation key and canonical semantic request identify a replay. Identical retry returns the recorded event; changed request fails. Actor-head Git CAS remains the write boundary. Cross-actor concurrency remains explicit; no global serializability claim.
5. Build a verified in-memory issue projection with indexes. Batch event-object reads rather than trusting a persistent materialized cache. Existing collectEvents spawns multiple Git processes per event and the previous 100-issue smoke measured about 504ms median. Performance tests must measure the new path and check exact semantic equality.
6. CLI additions are opt-in. Preserve title-only creation and existing comments. Provide JSON input/output, pagination, stable errors and full IDs. User-facing fields remain domain-general. A generic consumer fixture demonstrates inspect/update/retry/conflict behavior without importing Go Kitty.

## Boundaries and risks
No Beads replacement or Go Kitty execution adapter in this mission. No leases, approval authority, web UI, SaaS, new database or mandatory dependency. Old signed bytes and the current hn namespace remain valid. Malformed fetched data, missing parents, forged signatures, graph cycles, overlarge input, actor-ref races and pagination after ref changes require explicit tests. Existing proposal/CI/replication/memory behavior must retain its gates.
