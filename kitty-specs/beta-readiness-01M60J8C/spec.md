# Mission Specification: Hubnot beta readiness

**Mission Branch**: `feature/beta-readiness`  
**Created**: 2026-10-28  
**Status**: Specified; beta qualification pending  
**Input**: Package the agreed beta-readiness work as a mission and work through
it, starting with the beta support contract, then compatibility and recovery.
**Scope amendment (2026-10-28):** Add a unified native contribution and
moderation workflow for code and communications, independently optional and
disabled by default. Preserve the initial contract/compatibility/recovery order.

## Purpose

A user can install a supported Hubnot package, retain collaboration locally,
upgrade, restore a backup, and move a supported project between providers with
clear limitations and without unexplained data loss or authority changes.
This mission establishes that promise and the evidence required to make it.
Mission creation or completion of documentation does not itself publish a beta.

## User Scenarios & Testing

### User Story 1 — Know what the beta promises (Priority: P1)

An adopter can identify supported interfaces, platforms, operating limits and
experimental integrations before trusting a project to Hubnot.

**Independent Test**: Follow the support contract from installation through
support/qualification tables; every claim has scope and evidence requirements.

**Acceptance Scenarios**:

1. Given the current alpha, when the prospective beta contract is published,
   then current release status and remaining beta gates stay explicit.
2. Given a native-only install without provider credentials, when native
   collaboration runs, then no hosted service or policy opt-in is required.
3. Given an adapter, when capabilities are inspected, then preservation,
   native recreation and unavailable mappings remain distinguishable.

### User Story 2 — Upgrade without losing history (Priority: P1)

An existing user can move to the candidate reader without regenerating signed
facts or silently changing public observations.

**Independent Test**: Consume an offline corpus produced by released writers,
query public state, compare historical bytes/IDs/refs, and continue using a new
actor. Exercise incompatible data and private-state nonmutation.

**Acceptance Scenarios**:

1. Given a released-writer history, when the candidate reads it, then supported
   records retain exact payloads, signatures and IDs.
2. Given an unsupported protocol or resource version, when read, then a clear
   error occurs without accepted-history or private-state changes.
3. Given a supported archive and JSON consumer, when implementations change,
   then the declared semantic interface remains compatible.

### User Story 3 — Recover a project or a lost machine (Priority: P1)

An operator can reconstruct public history without a signer and separately
restore an exclusively controlled signer after loss of the original writer.

**Independent Test**: Follow the recovery procedures through real temporary Git
repositories and public CLI subprocesses, including unsafe/incomplete inputs.

**Acceptance Scenarios**:

1. Given code and retained collaboration/memory refs, when a keyless fresh clone
   reconstructs a project, then state agrees and no key or role appears.
2. Given a private backup and a fenced former writer, when the latest actor
   chain and signer are restored, then continuation appends without changing
   old facts. Two active writers sharing an actor remain unsupported.
3. Given lost keys without a backup, when a replacement device initializes,
   then it has a distinct actor and no inherited role; the sole-authority
   unrecoverable case is explained.
4. Given an interrupted remote write, when recovery runs, then uncertainty
   never authorizes a blind duplicate create.

### User Story 4 — Adopt a qualified beta (Priority: P2)

An adopter can distinguish observed provider/platform behavior, supported size
limits and external pilot feedback from simulated tests and proposed support.

**Independent Test**: Reproduce the documented qualification journeys and audit
an evidence matrix whose unexecuted cases remain pending.

**Acceptance Scenarios**:

1. Given disposable provider projects, when a supported provider-to-local-to-
   provider journey runs, then its mapping/loss and retry outcomes are recorded.
2. Given a package, when qualification runs, then evidence identifies the actual
   executable, OS/architecture, normal environment and outcome.
3. Given realistic histories and external adopters, when core journeys run,
   then measured limits and usability findings inform the beta verdict.

### User Story 5 — Contribute without upstream write access (Priority: P1)

A stranger can prepare and deliver a signed issue, discussion contribution or
code proposal without a provider account, upstream push access, prior native
role or permission to change a development branch. A project can inspect and
admit it through one native workflow, applying permissions appropriate to the
requested operation. This workflow is optional; existing repositories retain
their current behavior unless an authorized operator explicitly enables it.

**Independent Test**: Use contributor, maintainer and keyless observer
repositories plus ordinary local transport. Run both standard and native-only
binaries, with upstream publication denied to the contributor and no provider
credentials. Deliver contributions, inspect pending work, admit permitted
operations and reconstruct the resulting shared view from signed records.

**Acceptance Scenarios**:

1. Given no workflow configuration, when initialization, normal collaboration,
   migration, upgrade, synchronization or merge runs, then it installs no
   contribution gate, moderation policy, endpoint or worker and preserves legacy
   command behavior. Each other optional feature remains independent.
2. Given an unknown contributor without upstream push permission, when they
   submit an issue, discussion reply or code proposal using a bounded artifact
   and submission reference, then the receiving project can discover/inspect it
   in pending intake without granting branch, policy, runner or merge authority.
   A documented file/reference handoff is sufficient; hosted discovery is optional.
3. Given an enabled project policy, when the same intake mechanism evaluates
   contribution operations, then issue creation/replies may be automatically
   admitted by explicit project rules, while editing others' content, moderation
   or code changes require their configured authority. Admitting a proposal
   cannot manufacture its code review, CI, acceptance or merge evidence.
4. Given validly signed spam, oversized/truncated input, many new actor keys,
   out-of-scope refs or a replayed submission, when intake runs, then finite
   aggregate intake/retention and per-request limits are enforced before shared
   visibility, original facts remain unchanged, retries deduplicate exact intent,
   and unrelated valid pending work remains inspectable. A valid signature is
   attribution, not anti-spam proof or a scarce identity.
5. Given a moderator decision, when an authorized actor admits/rejects, hides/
   restores content or blocks/unblocks an author, then a signed project-scoped
   decision records its target and evaluated policy. Unauthorized or conflicting
   decisions cannot silently win, impersonate an author or erase history.
6. Given a fresh observer or a restored repository, when selected contribution
   and moderation records are exchanged and verified, then pending/admitted
   content and project-scoped moderation reconstruct consistently without keys,
   inherited roles, a provider or a compulsory service. Raw historical inspection
   remains distinct from the moderated shared view.

### Edge Cases

- Ordinary clone omits custom refs; a host accepts branches but not Hubnot refs.
- A backup predates the latest actor chain; the old writer is still active;
  private modes or paths are unsafe; linked worktrees locate state differently.
- New facts make binary rollback unsupported; old journals cannot be reset by
  deleting uncertain progress.
- Imported historical policy is not active authority; public archives omit
  private keys, journals and local audit authority.
- Credentials or platform runners are absent: evidence remains pending.
- Transport verification does not imply contribution admission; contribution
  admission does not imply proposal acceptance, merge or execution authority.
- A contributor changes a payload after inspection; an admission targets only
  the exact inspected revision, never a mutable locator or title.
- Existing selected replication budgets apply after Git pack receipt; they do
  not establish hard pre-download quotas for public intake. Compatibility-all
  replication must not be inherited as the public-intake discovery policy.
- Many tiny valid submissions or new keys exhaust aggregate pending storage;
  limits must apply across actors as well as to one signed request.
- An imported policy attempts to name itself authoritative; a moderator is
  revoked; concurrent decisions disagree; disabling the workflow cannot rewrite
  signed facts or falsely claim deletion from other replicas.

## Requirements

### Functional Requirements

| ID | Title | User Story | Priority | Status |
| --- | --- | --- | --- | --- |
| FR-001 | Support contract | As an adopter, I can see supported platforms, Git configurations, interfaces, limits and qualification status. | High | Open |
| FR-002 | Evolution rules | As an integrator, I know compatibility rules for facts, memory, policy, archives, journals and CLI JSON, including refusal/rollback limits. | High | Open |
| FR-003 | Historical compatibility | As an existing user, I can read released-writer data and continue without rewriting it. | High | Open |
| FR-004 | Incompatible-input refusal | As an operator, unsupported-format errors leave accepted history and private state unchanged. | High | Open |
| FR-005 | Public recovery | As an operator, I can retain and restore code and collaboration/memory refs that ordinary clone omits. | High | Open |
| FR-006 | Private recovery | As an operator, I have a tested private backup/restore procedure with freshness, fencing and exclusion rules. | High | Open |
| FR-007 | Lost-device continuity | As an operator, I can use a distinct replacement actor without inferred roles and understand sole-authority loss. | High | Open |
| FR-008 | Interrupted recovery | As an operator, I distinguish confirmed progress from uncertainty requiring reconciliation. | High | Open |
| FR-009 | Provider qualification | As a migrator, I can inspect live-account observations by provider/profile/resource/direction and explicit loss. | Medium | Open |
| FR-010 | Package qualification | As an installer, I can inspect exact package results by platform/variant and distribution limitations. | High | Open |
| FR-011 | Practical limits | As an adopter, I have measured repository/history limits and recovery costs. | Medium | Open |
| FR-012 | Boundary review | As an adopter, I can inspect credential, key, import, filesystem, authority and runner findings and disposition. | High | Open |
| FR-013 | Adoption proof | As an adopter, I can inspect independent installation, collaboration, migration, upgrade and restore findings. | Medium | Open |
| FR-014 | Truthful verdict | As an adopter, I can find current release docs and a beta verdict listing all unmet gates. | High | Open |
| FR-015 | Optional unified workflow | As an operator, I can explicitly enable one native contribution/moderation workflow without enabling it by default or coupling it to other opt-ins. | High | Open |
| FR-016 | Open submission | As an unknown contributor, I can submit signed code or communications without upstream write access, a provider account or prior project role. | High | Open |
| FR-017 | Bounded intake | As a maintainer, I can inspect pending submissions through isolated validation with finite per-request and aggregate resource limits before shared visibility. | High | Open |
| FR-018 | Operation authority | As a project, I apply common admission machinery with distinct permissions for creation, replies, edits, moderation, proposal admission and existing merge/execution. | High | Open |
| FR-019 | Native moderation | As a moderator, I can record authorized scoped admit/reject, hide/restore and author block/unblock decisions with exact targets and visible conflicts. | High | Open |
| FR-020 | Portable contribution lifecycle | As an observer, I can reconstruct pending/admitted state and moderation from immutable signed Git-backed records, preserving deduplication and interrupted recovery. | High | Open |
| FR-021 | Public-contribution proof | As an adopter, I can reproduce a provider-free stranger-to-project journey and hostile-input refusals in standard/native-only builds. | High | Open |

### Non-Functional Requirements

| ID | Title | Requirement | Category | Priority | Status |
| --- | --- | --- | --- | --- | --- |
| NFR-001 | Exact history | Every compatibility/recovery case compares all selected historical payloads, signatures and ref targets; no rewriting. | Integrity | High | Open |
| NFR-002 | Least authority | Each recovery/import case asserts absence of unintended keys, adopted authority and optional-policy activation. | Security | High | Open |
| NFR-003 | Offline local gates | Compatibility and local recovery run without credentials/services and exercise standard and native-only builds. | Portability | High | Open |
| NFR-004 | Reproducibility | Every qualification observation records source/version, environment, scenario, outcome and evidence; no unexecuted pass. | Reliability | High | Open |
| NFR-005 | Quality gates | Go changes pass formatting, full race, vet, build and whitespace checks before review; docs receive applicable consistency checks. | Quality | High | Open |
| NFR-006 | Finite public intake | Every enabled ingress and pending store has positive finite byte, object/event/count, time/cancellation and aggregate retention limits. Oversized input is stopped within those declared bounds; a post-fetch Git budget cannot be claimed as a download cap. | Resource safety | High | Open |
| NFR-007 | Moderation integrity | All decisions bind exact content/project/policy identities, validate authority and preserve original signed bytes. Replay or changed-payload tests prove zero unintended shared visibility, role grants or execution. | Security | High | Open |

### Constraints

| ID | Title | Constraint | Category | Priority | Status |
| --- | --- | --- | --- | --- | --- |
| C-001 | Independence | Git-only runtime and Go/Git-only packaging; no mandatory provider, Python, database, Docker or service. | Product | High | Open |
| C-002 | Optionality | Protected checks, automerge and audits independently disabled by default; no implicit workers. | Product | High | Open |
| C-003 | Authority | Imports, backups and continuity grant no native policy roles or execution permission. | Security | High | Open |
| C-004 | Preservation | Released tags/assets and frozen prior-mission evidence remain unchanged. | Integrity | High | Open |
| C-005 | Scope | No mandatory UI, Windows distribution, background queues, social recovery, global erasure or provider parity. | Scope | High | Open |
| C-006 | Sequence | Support contract first, compatibility/recovery next; external evidence cannot stall executable local work. | Delivery | High | Open |
| C-007 | Optional contribution defaults | Absent contribution configuration preserves existing native commands, replication selections and views; enabling requires explicit legitimate project authority and starts no endpoint, worker or network request. | Product | High | Open |

### Key Entities

`data-model.md` defines support contract, compatibility corpus, public recovery
set, private recovery set, rehearsal, qualification observation and beta verdict.
Historical provenance is distinct from current authority.
Submission, transport verification, contribution admission, proposal acceptance,
moderation decision and moderated shared view are distinct concepts in that model.

## Success Criteria

- **SC-001**: One maintained contract identifies promised stable/experimental
  surfaces and environments with qualification gates.
- **SC-002**: Released-writer corpora exercise the historical baseline offline
  with exact history preservation and continuation checks.
- **SC-003**: Unsupported-version cases demonstrate actionable refusal and zero
  accepted-history/private-state changes.
- **SC-004**: Rehearsals restore selected public code/collaboration/memory without
  a signer; exclusive private restoration safely continues the latest chain.
- **SC-005**: Recovery instructions cover stale backups, custom refs, authority
  re-adoption, journals, planned rotation and irrecoverable key loss.
- **SC-006**: Provider/platform matrices distinguish passes, limitations, failures
  and pending cases; every beta-supported combination has observed passing proof.
- **SC-007**: Boundary review, workload measurements and external-user journeys
  record findings with no unresolved release-blocking defect.
- **SC-008**: Final beta verdict traces each promise to evidence; missing external
  evidence leaves qualification pending instead of inventing a pass.
- **SC-009**: A stranger without upstream push access delivers an issue,
  discussion contribution and code proposal; a project admits operations under
  its own rules and an observer reconstructs them using Git-only native operation.
- **SC-010**: Disabled-default and authority tests prove no implicit gates,
  services, policy adoption, role grants, CI/merge evidence or execution; existing
  local collaboration remains compatible without the new opt-in.
- **SC-011**: Adversarial public-intake tests demonstrate declared transfer,
  parsing, object/event/count, time and aggregate pending-retention bounds,
  exact-target deduplication and safe interrupted retries, including signed spam
  and many-key cases.
- **SC-012**: Native moderation decisions replicate and recover with authentic
  authorship, visible conflict/revocation behavior, reversible scoped visibility
  and unchanged original facts; no global-erasure guarantee is implied.
