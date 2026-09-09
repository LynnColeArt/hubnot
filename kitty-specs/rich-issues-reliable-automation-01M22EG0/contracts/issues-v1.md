# Issue protocol contract

The authoritative design is plan.md sections Signed state and projection, Reliable writes, and CLI contract. Public finalized wire and CLI examples will ship in docs/issues-v1.md with acceptance tests. The additive protocol preserves old signed bytes; issue.revise creates immutable full-state successors under a stable issue.open identity. Semantic revision heads are separate from actor stream sequence. Signature attribution and informational issue fields confer no execution authority.

Required failure guarantees: stale preflight and invalid requests append nothing; actor CAS losers never report success; equal operation replay returns exact original event; divergent key reuse fails; unresolved heads remain explicit; selective replication never promotes missing issue dependencies; bounded reads fail visibly rather than silently truncating history.
