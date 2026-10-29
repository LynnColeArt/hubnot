# Standing acceptance for explicitly scoped automerge

Date: 2026-10-29

Status: product direction accepted by the owner; protocol and implementation
review remain required. This record does not add a beta acceptance requirement
or approve the implementation on Andrew's fork.

## Decision

Support a future opt-in policy capability in which a maintainer-authorized rule
supplies standing acceptance for a narrow class of changes. A maintainer can
delegate ordinary work through policy without signing a new acceptance for every
proposal. Repositories without this capability retain their existing behavior.

Keep separate authority for proposing work, requesting CI, supplying trusted
results, reviewing, executing an automerge rule, and amending policy. Delegated
pipeline requesters are independently useful and are being reviewed in PR #18;
their inclusion does not authorize standing acceptance.

## Required design boundaries

- The selected authoritative policy must authorize the rule and its executors.
  Imported policy, a contributor, or a requester cannot grant that authority.
- Policy amendments, `.hn/` governance changes and protected-check changes keep
  the explicit protected approval path. Review must examine path matching,
  combined changes, renames and other ways a change can cross that boundary.
- Standing acceptance requires independent authorized review; an author's own
  approval is insufficient. Required CI still comes from trusted runners.
  Observational audits do not become merge evidence.
- Evaluate the exact proposal revision, selected base policy, authorized
  evidence, and applicable vetoes immediately before advancement. A hosted
  receiver must check its own facts and use an expected-old update; a stale
  client decision is insufficient. State explicitly which vetoes were knowable
  at advancement rather than promising knowledge of disconnected replicas.
- Preserve all historically valid signed facts. Andrew's proposed union of the
  legacy validation path and a strict rule-validation path is a candidate design,
  not an approved decoder patch. Review unknown-field behavior, both paths,
  older-client refusal, replication, replay, recovery and merge projection.
- Define the distinction between immutable historical evidence and a new
  advancement request evaluated against current facts. Do not retroactively
  rewrite signed facts to represent a later rejection.

## Sequencing and consequences

Review PR #18 and the independent sync optimization in PR #19 separately. Keep
the current beta mission focused on its existing compatibility, recovery,
contribution, qualification and acceptance obligations. Plan standing acceptance
as separate work with a compatibility contract and public local/hosted scenarios
before implementation is incorporated. Hosting is independently optional; its
deployment must not become a prerequisite for ordinary local Hubnot.

Scoped pipelines are a compatible future direction, but their path/branch
applicability and historical policy semantics need their own design. No new
pipeline scope feature or scheduler is authorized merely by this record.

Source: [Andrew's proposal](https://github.com/LynnColeArt/hubnot/issues/20).
Owner direction: review PRs #18/#19, merge if sound, document standing acceptance
and optional hosting separately, then continue the beta mission.
