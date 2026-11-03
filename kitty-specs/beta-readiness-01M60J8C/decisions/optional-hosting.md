# Optional native Hubnot hosting

Date: 2026-10-29

Status: product direction accepted by the owner; architecture, implementation
review and qualification remain required. This record does not approve or merge
Andrew's server implementation and does not add hosting to the current beta gate.

## Decision

Native self-hosted collaboration fits Hubnot's purpose. Prefer an optional
hosting capability within the Hubnot product, such as an explicitly invoked
`hn serve`, provided normal local operation and its dependency footprint remain
independent. Decide final code and package boundaries after reviewing the server;
an optional command does not justify unconditional server startup or mandatory
server dependencies.

Ordinary initialization, imports, upgrades, contribution intake, proposals and
merges must not enable a listener, install a daemon or adopt hosted authority.
No GitHub/GitLab account, external database, Docker installation or background
service becomes necessary for local use.

## Required design boundaries

- Hosting must reuse the native signed-fact and authorization semantics. A
  transport identity or permission to submit content is not permission to push
  development branches, change policy, run repository code or merge.
- Explicitly configure listener/SSH entry points, repository selection,
  identities, admission rules, quotas and protected branch rules. Public access
  is a deliberate operator choice; absence of configuration does not open it.
- Apply bounded intake and quarantine validation before promotion, including
  aggregate retained material, nested audit packs, failure accounting, time and
  decoded-object limits. Existing signature checks alone are not a resource
  bound. Reconcile the beta contribution protocol rather than inventing a second
  admission or moderation authority.
- Reevaluate protected advancement at the destination against the selected policy
  and facts available there; preserve expected-old updates and retry/conflict
  behavior. Direct code pushes must not bypass protected proposal requirements.
- Receive/quarantine environment handling, replacement objects, lazy fetching,
  object formats and hook/config execution need explicit review. Useful general
  fixes on Andrew's fork may be reviewed independently of the server feature.
- Document public facts versus private credentials, selections, authority and
  recovery journals. A hosted public backup does not recover a lost private key
  or automatically grant project authority to a new receiver.

## Sequencing and consequences

Review the smaller upstream PRs first and continue the existing beta mission.
Review Andrew's server architecture and its relationship with the new bounded
contribution exchange as separately scoped work. The current beta still needs
the local provider-free contribution journey even if no server is deployed.

The same product can support a single local repository, ordinary Git transport,
and optional self-hosted access. Published support claims must distinguish these
modes and follow actual platform, abuse-resistance and recovery qualification.

Sources: [Andrew's proposal](https://github.com/LynnColeArt/hubnot/issues/20),
[server contribution on Andrew's fork](https://github.com/acoliver/hubnot/pull/1).
Owner direction: document optional hosting separately from standing acceptance;
do not silently widen the current beta mission.
