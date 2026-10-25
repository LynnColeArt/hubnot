# Opt-in protected checks and automerge

## Intent and confirmed scope

A repository maintainer wants agents to deliver changes without quietly weakening lint checks, and wants explicitly authorized native automation to merge eligible proposals. The user approved the contract in conversation, then requested documentation followed by implementation. Both capabilities are independently opt-in; ordinary repositories retain existing behavior. Canonical terms: protected checks, protected paths, designated reviewers, automerge rule, proposal candidate. Actors are cryptographic identities; no heuristic human/agent distinction grants authority.

## User scenarios and testing

1. An existing repository upgrades and uses issues, proposals and manual merge without enabling either feature. No new rule, runner, network call or background process appears.
2. A maintainer opts into protected checks, names lint configuration and scripts, and designates reviewers. An agent replaces the required lint command with a successful no-op. Ordinary review and passing CI are insufficient; exact designated approval is required. Removal, renaming and mode/type changes receive the same treatment.
3. A designated reviewer approves that exact candidate. Existing review, CI and maintainer acceptance requirements still apply; a later revision needs new evidence. Editing or deleting the policy cannot authorize itself.
4. A maintainer independently enables a narrow automerge rule. Preview explains its matching rule, target, changes and blockers without signing events or changing Git. Explicit execution merges a currently accepted candidate as an authorized native actor and records the existing signed merge fact.
5. The target advances, evidence is withdrawn, policy changes, an actor is unauthorized, or paths fall outside a rule. Execution refuses before advancing the branch. Publication is a separate action.

## Functional requirements

| ID | Requirement | Status |
|---|---|---|
| FR-001 | Both capabilities are absent/disabled by default and independent. Initialization, upgrade and imports never enable them. | confirmed |
| FR-002 | Enabled protection requires designated, distinct, non-author approvals for changes to configured literal files/directories, the policy itself and required pipeline definitions. | confirmed |
| FR-003 | Additions, deletions, renames, content and mode/type changes are evaluated from signed base/head trees; candidate rules cannot weaken their own gate. | confirmed |
| FR-004 | Protection affects status, acceptance validation, manual merge and fetched evidence admission. Exact candidate review evidence remains signed and independently verifiable. Current withdrawal blocks subsequent enabled merge. A target with protection enabled refuses manual merges governed by a different base-policy digest, including newly opened candidates against pre-opt-in ancestors; historical signed evidence remains valid. | confirmed |
| FR-005 | Policy inspection and amendment comparison expose activation, protected paths, thresholds, reviewers and automerge rule changes. | confirmed |
| FR-006 | Explicitly configured automerge rules select a target branch, allowed executing actors, proposal authors and literal allowed paths. Only matching rules may run. | confirmed |
| FR-007 | Automerge defaults to read-only preview; execution is explicit and requires existing current approvals, passes and maintainer acceptances. It never manufactures reviews or decisions. | confirmed |
| FR-008 | Execution rechecks exact candidate, current evidence, policy, actor, branch and expected target. Stale targets, current change requests/rejections, protected changes, policy amendments and unmatched changes refuse automatic merge. | confirmed |
| FR-009 | Successful automation records the existing native merge event and retains manual merge repair behavior; neither preview nor execution implicitly syncs, pushes, starts a daemon or executes pipelines. | confirmed |
| FR-010 | Public CLI tests prove opt-out compatibility, both independent opt-ins, rejection paths and successful local native automerge without GitHub support. | confirmed |

## Non-functional requirements

| ID | Requirement | Status |
|---|---|---|
| NFR-001 | Each configuration has at most 64 protection paths/reviewers/rules and 64 actors/authors/paths per rule; each literal path is at most 4096 bytes and changed-path output is bounded to 8 MiB. | confirmed |
| NFR-002 | Native feature behavior and acceptance tests pass in ordinary and nogithub builds; no new external Go modules or mandatory runtime services. | confirmed |

## Constraints

| ID | Constraint | Status |
|---|---|---|
| C-001 | Preserve existing signed bytes, policy authority and evidence identities. Missing optional fields retain legacy semantics. | confirmed |
| C-002 | No semantic claim that arbitrary lint changes are weaker; protection gates file changes and relies on explicit reviewer authority. Raw Git writes outside Hubnot are outside enforcement. | confirmed |
| C-003 | No general expression language, daemon, queue, implicit role migration, automatic acceptance, remote publication, or GitHub dependency. | confirmed |

## Success criteria

All scenarios pass through native CLI/repository boundaries. A protected check cannot pass acceptance using only ordinary approval, nor by deleting protection. A matching accepted proposal merges only on explicit execution; previews leave branch and actor refs unchanged. A repository without optional configuration behaves as before.

## Assumptions and conservative first-version decisions

Paths are literal repository-relative files, or directories ending in `/`; there are no globs. Automerge targets must exactly equal the proposal signed base, requiring a new candidate after target movement. Automated changes to protected checks or policy take the manual path even when reviewed. Protected approvals reuse exact existing review events and count independently from ordinary reviewer membership. Automation uses an explicitly listed maintainer identity and does not create a machine account.

## Implementation boundary: automatic operation

First-version automerge requires a full, non-promisor local repository. It refuses shallow and configured partial/promisor repositories before object reads, with explicit hydration or manual-merge guidance. This preserves read-only preview and no implicit network or recovery-record writes; existing manual collaboration and selected shallow recovery are unchanged. No new shallow or partial-clone support is claimed.
