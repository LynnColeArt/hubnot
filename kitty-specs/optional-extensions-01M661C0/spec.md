# Mission Specification: Optional extensions and custom types

**Mission**: optional-extensions-01M661C0
**Mission ID**: 01M661C0T0E4DS0AXBCXG7WCAX
**Mission branch / planning base / merge target**: feature/optional-extensions
**Created**: 2026-10-30
**Status**: Specified for separate planning; implementation has not started.
**Input**: "Perfect. Let's package the extension system as a separate mission, including custom types and the Spec Kitty use case, with preserving the default Hubnot workflow as a hard requirement."

## Intent Summary

A Hubnot user optionally enables a locally installed external application to work
with native collaboration and application-defined documents, lists and tickets.
An extension author can define a versioned information model without adding its
domain rules to Hubnot. A Spec Kitty-shaped consumer must be able to represent
directives, tactics, packs, relationships and explicitly selected charter
revisions; Spec Kitty retains composition, activation, context generation and
execution semantics. ThickTicket-style synchronization is a second reference
consumer. A user who never enables extensions keeps the established standalone
Hubnot workflow, dependencies, authority and data.

This summary consolidates the owner's explicit confirmations in the preceding
discussion, including the final instruction to create this separate mission.
No new discovery question is required for already-confirmed intent. Technical
defaults are documented in the plan; this specification does not promise an
implemented host, a Spec Kitty port or new beta qualification.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Keep ordinary Hubnot ordinary (Priority: P1)

As a native Hubnot user, I create and exchange tickets, discuss work, manage
projects, use memory and follow governed code workflows without knowing about
extensions. This is a hard release-blocking condition, not a polish objective.

**Independent Test**: Run the same baseline journeys before and after this mission
with no registry, with disabled modules, with missing/incompatible/crashing modules
and with malformed extension-local configuration. Compare native outcomes and
history, setup steps, dependencies and measured overhead.

**Acceptance Scenarios**:
1. **Given** no configured extension, **when** the user follows existing native
   commands, **then** the same inputs, output contracts, signed facts, refusals
   and recovery semantics apply without extension setup or new runtime dependencies.
2. **Given** an unavailable or failed module, **when** the user works on unrelated
   native resources, **then** its configuration is not loaded or executed by that
   path and the native journey completes normally.
3. **Given** custom data in a repository, **when** its specialized plugin is absent,
   **then** an extension-aware Hubnot client can inspect and preserve/export the
   generic records and can still use native tickets.
4. **Given** an established memory or collaboration history, **when** extensions
   are introduced, **then** old signed bytes, IDs, refs and meanings remain unchanged.

### User Story 2 — Deliberately enable an external application (Priority: P1)

As a repository operator, I inspect an installed module, see declared versus
granted capabilities, and explicitly enable only the needed actions locally.

**Independent Test**: Use a real external reference process. Inspect it disabled,
grant read access, prove an ungranted raw write refuses, grant a specific write,
disable it, and verify native use and old facts remain available.

**Acceptance Scenarios**:
1. **Given** a repository declaration or manifest, **when** it is read or inspected,
   **then** no executable starts, identity is initialized, dependency is installed,
   provider is contacted or project authority is changed.
2. **Given** a locally enabled module, **when** it submits a request, **then** its
   host-bound identity, resolved repository, version and action grants are checked
   before native dispatch; self-asserted identity cannot enlarge access.
3. **Given** a read-only grant, **when** a handwritten write or conflict-resolution
   request arrives, **then** it refuses before signing, journal creation or publication.
4. **Given** an enabled module, **when** it is disabled or fails, **then** owned
   processes are cleaned up, pending operations stay distinguishable from success,
   and signed facts are not deleted.

### User Story 3 — Define a portable information model (Priority: P1)

As an application author, I define namespaced versioned document, list and ticket
types with structured fields and text, collections and typed relationships.
People collaborate through signed revisions and explicit conflicts.

**Independent Test**: Define one document, one list and one ticket type; create,
revise and conflict their instances; transfer through a local bare Git remote to
an independently initialized clone; inspect, search and export without the authoring
plugin. Repeat with an unrelated inventory model.

**Acceptance Scenarios**:
1. **Given** a published type declaration, **when** an instance is written,
   **then** it names the exact schema revision, retains exact text/structured
   values, and is bounded and validated without running publisher-supplied code.
2. **Given** the same logical record edited by two actors, **when** histories meet,
   **then** both heads remain visible until an explicitly authorized resolution
   names them; timestamps do not select a winner.
3. **Given** a type evolves, **when** a new schema revision is introduced,
   **then** existing instances keep their original schema and meaning; conversion
   creates explicit new revisions rather than rewriting history.
4. **Given** missing type/relationship dependencies or a type unsupported by the
   reader, **when** it is inspected or transferred, **then** bounded structured
   diagnostics preserve the data and unrelated native use.
5. **Given** a collection or relationship, **when** it is edited or reconstructed,
   **then** member order and logical-versus-exact-revision targets are explicit.

### User Story 4 — Recover native operations and observations (Priority: P1)

As a connector author, I observe accepted visible changes, submit exact operations
and recover after a lost acknowledgement without generating a new intent.

**Independent Test**: A fake external tracker imports an issue/comment, loses a
native acknowledgement, restarts, recovers the original actor/operation, exports
a native edit once, and catches up after disconnected or invalidated paging.

**Acceptance Scenarios**:
1. **Given** a native operation with actor and operation identity, **when** its
   acknowledgement is lost, **then** exact lookup/replay returns the original
   result or a truthful uncertain state; changed content or parents refuse.
2. **Given** a captured visible set, **when** it is paged or resumed, **then** results
   are bounded and duplicate-recognizable; selection, authority or moderation
   changes cannot silently omit visibility transitions.
3. **Given** invalid native history, **when** changes are observed, **then**
   verification failure is distinguishable from an empty result.
4. **Given** enabled contribution policy, **when** native resources are accessed,
   **then** the same admission, visibility, retention and authority checks apply
   as direct native use. Unsupported custom-type policy combinations refuse explicitly.
5. **Given** an external API operation, **when** a connector reports it,
   **then** its own journal owns reconciliation; Hubnot does not attest unobserved writes.

### User Story 5 — Prove the Spec Kitty use case without embedding it (Priority: P2)

As a prospective Spec Kitty integrator, I use an external reference consumer to
represent representative doctrine documents, a pack and a charter selecting exact
artifact revisions through the general extension interface.

**Independent Test**: The consumer defines the representative schemas, publishes
related records, records an explicit selection, reconstructs them on another clone
and is removed. Repeat with the unrelated model to demonstrate generality.

**Acceptance Scenarios**:
1. **Given** directive/tactic documents and pack relationships, **when** the consumer
   stores them, **then** Hubnot sees generic typed data, not built-in doctrine kinds.
2. **Given** a stored or search-matched directive, **when** it is read,
   **then** it neither activates itself nor changes Hubnot policy or prompt authority.
3. **Given** a charter selection, **when** the consumer interprets it,
   **then** exact selected revisions and consumer-defined semantics are visible.
4. **Given** the consumer is absent, **when** another collaborator inspects the
   repository, **then** its documents, collections, relationships and signed history
   remain inspectable/exportable through the generic interface.

### Edge Cases

- Manifest/type namespace collision, built-in shadowing, changed local executable
  registration, symlink/path alias repositories and a request naming another repository.
- Malformed, duplicate-key, oversized, recursive or executable schema declarations;
  unsupported schema/protocol versions; missing exact type or relationship targets.
- Multiple writers, partial publication, retained uncertain operations, changed
  immutable replay input, cursor expiry, hidden/restored resources and authority changes.
- Child EOF, blocked reads, oversized output/stderr, timeout, cancellation, crash,
  unavailable dependencies and failure while another module continues working.
- Custom refs selected by an old reader, public export with local credentials nearby,
  disabled modules after data creation, and data copied without local registrations.
- Ordinary native commands with many custom records, a broken extension registry,
  absent signers for read-only inspection, and native-only packages.

## Requirements *(mandatory)*

### Functional Requirements

| ID | Title | User Story | Priority | Status |
|----|-------|------------|----------|--------|
| FR-001 | Preserve default workflow | As a native user, I complete established journeys with unchanged commands, setup and semantics without extensions. | High | Open |
| FR-002 | No implicit launch | As a reader, I inspect repository declarations and manifests without executing code, installing dependencies, fetching providers or creating identities. | High | Open |
| FR-003 | Explicit local lifecycle | As an operator, I register, inspect, enable, disable and remove a local module without changing signed facts. | High | Open |
| FR-004 | Honest capability inventory | As an operator, I distinguish declarations, grants, enabled state, compatibility and observed availability, including unsupported actions. | High | Open |
| FR-005 | Scoped host access | As an operator, I grant repository/family/type/action access enforced on every raw request with host-bound module/repository identity. | High | Open |
| FR-006 | Existing native adapters | As a consumer, I read/write supported issues, projects/items, discussions/replies and inert workflow definitions through their normal validated native paths. | High | Open |
| FR-007 | Exact operation recovery | As a consumer, I pin the actual actor, persist immutable intent, recover lost acknowledgements and reject changed replay input for every advertised replayable mutation. | High | Open |
| FR-008 | Accepted change observation | As a consumer, I catch up and incrementally page accepted visible changes with bounded checkpoints, duplicate recognition and explicit invalidation. | High | Open |
| FR-009 | Custom type declarations | As an author, I define namespaced, versioned document, list and ticket types with exact portable schemas without changing built-in kinds. | High | Open |
| FR-010 | Structured record content | As an author, I preserve exact text plus bounded structured fields and distinguish generic schema validation from application semantic validation. | High | Open |
| FR-011 | Collaborative revisions | As a collaborator, I create/revise records by stable logical identity, exact schema and expected heads, with explicit multi-head resolution and immutable history. | High | Open |
| FR-012 | Collections and relationships | As an author, I define ordered collections and typed links with explicit exact-revision or logical-target semantics. | High | Open |
| FR-013 | Generic inspection and search | As a plugin-free reader, I inspect/search/filter custom records, including explicit bounded regex, without activating content or requiring a specialized renderer. | High | Open |
| FR-014 | Native portable storage | As a collaborator, I reconstruct/export selected custom histories, types and dependency closure through Git without the original plugin, credentials or journal. | High | Open |
| FR-015 | Safe unknown-type behavior | As a reader, I retain unknown custom data and receive explicit unsupported/missing-dependency diagnostics without breaking unrelated native histories. | High | Open |
| FR-016 | Type evolution | As an author, I add new schema revisions and explicitly convert records without reinterpreting old signed data. | High | Open |
| FR-017 | Authority continuity | As an operator, I retain native admission, moderation, validation, policy and execution boundaries; unsupported combinations refuse explicitly. | High | Open |
| FR-018 | Public provenance | As a consumer, I read public identity and record descriptive connector/source associations without exporting keys, impersonating signers or manufacturing trusted provider evidence. | High | Open |
| FR-019 | Bounded external host | As an operator, I get finite requests/output, cancellation, cleanup and fault containment across modules and ordinary native use. | High | Open |
| FR-020 | Structured diagnostics | As a consumer, I distinguish denied, unsupported, incompatible, unavailable, stale, conflict, verification, limit, cancellation and uncertain outcomes. | High | Open |
| FR-021 | Fake-tracker reference | As a ThickTicket author, I run an external reference connector proving issue/comment recovery, observed edit export, conflict refusal and no echo loop. | High | Open |
| FR-022 | Charter-model reference | As a prospective Spec Kitty author, I represent directives, tactics, packs and explicit charter selections using the generic contract without embedding doctrine logic. | High | Open |
| FR-023 | Independent model proof | As another author, I create unrelated inventory/custom ticket/list types through the same interface without core source changes. | High | Open |
| FR-024 | Consumer documentation | As an extension author, I have versioned contracts, examples, recovery recipes, capability matrix and default/native-only compatibility guidance. | High | Open |

### Non-Functional Requirements

| ID | Title | Requirement | Category | Priority | Status |
|----|-------|-------------|----------|----------|--------|
| NFR-001 | Default path isolation | Baseline journeys introduce zero extra setup steps, extension process starts, provider requests or extension-local state writes; native paths do not parse the extension registry. | Usability | High | Open |
| NFR-002 | Measured default overhead | On paired fixed-host release measurements of 30 observations per command, median elapsed regression must not exceed max(10 percent, 5 ms) and median peak-RSS regression max(10 percent, 1 MiB); report all samples and investigate failures without retry-to-green. | Performance | High | Open |
| NFR-003 | Bounded consumption | Every host request/response, schema, record, relationship traversal and captured change page has an enforced finite size/count/depth/time limit published in the contract; each limit has an over-limit refusal case with no forbidden effects. | Reliability | High | Open |
| NFR-004 | Failure containment | All lifecycle cases (missing/incompatible/crash/EOF/timeout/oversized/cancelled) leave unrelated native journeys and other enabled modules usable; finite child cleanup is observed. | Reliability | High | Open |
| NFR-005 | Historical integrity | All selected existing collaboration and memory golden bytes, IDs, signatures and refs remain byte-identical; 100 percent of baseline refusal assertions remain effective. | Compatibility | High | Open |
| NFR-006 | Credential boundary | Public capability payloads, logs and exports contain zero private signing keys, provider credentials or local recovery descriptors in the exercised leak cases. | Security | High | Open |
| NFR-007 | Platform and variants | Complete claimed support cases on Linux/macOS AMD64/ARM64 for standard and combined native-only packages using exact release-candidate binaries; missing observations are pending. | Portability | High | Open |
| NFR-008 | Generic portability | Both reference consumers reconstruct their public model on an independently initialized clone with the specialized module absent and no source-machine private state. | Portability | High | Open |

### Constraints

| ID | Title | Constraint | Category | Priority | Status |
|----|-------|------------|----------|----------|--------|
| C-001 | Separate mission | This work neither changes beta acceptance nor marks beta/pilot/issue23 work complete; integrate the approved beta native contribution foundation before implementation claims against it. | Scope | High | Open |
| C-002 | Native independence | Default Hubnot keeps its Go-only packaging and Git requirement; no provider account, external language runtime, model, MCP, service database or background service becomes mandatory. | Product | High | Open |
| C-003 | Optional activation | Repository declarations and record content are inert; installation, enablement, authority selection and execution remain distinct explicit actions. | Authority | High | Open |
| C-004 | No embedded doctrine | Do not port Spec Kitty, its charter resolver, activation/composition algorithms, governance engine, Java service or runtime into Hubnot. | Scope | High | Open |
| C-005 | No enterprise connectors | Jira, Remedy, ServiceNow, Azure enterprise integration, scheduling, credentials and external reconciliation remain application-owned; a local fake tracker suffices. | Scope | High | Open |
| C-006 | Existing protocol preservation | Add explicit versioned custom storage/protocol surfaces; do not rewrite hn/0 or hn-memory/0 records or inject unknown kinds into their existing chains. | Compatibility | High | Open |
| C-007 | Honest process boundary | Host grants mediate host calls, not arbitrary same-OS-user filesystem access; no hostile-module sandbox or multi-tenant isolation claim. | Security | High | Open |
| C-008 | Narrow first capability set | Governed review/accept/merge, policy mutation, identity lifecycle, runners and execution are not granted by generic data access; advertise unsupported surfaces rather than inventing replay guarantees. | Scope | High | Open |
| C-009 | No marketplace or frontend | Automatic module distribution/updates, dynamic in-process libraries, general scripting engines, MCP transports and a new Hubnot GUI are outside this mission. | Scope | High | Open |
| C-010 | Memory remains cognition | Reuse proven storage/validation/search ideas where appropriate; do not turn recall/grep into policy activation or require a rewrite of native memory as a prerequisite. | Semantics | High | Open |

### Key Entities *(include if feature involves data)*

- **Extension declaration**: inert module identity, version, protocol and capabilities.
- **Local registration and enablement**: selected executable, repository binding and grants; not replicated authority.
- **Type revision**: exact namespaced schema identity and portable validation data.
- **Logical record and signed revision**: stable document/list/ticket identity and immutable authored content with expected revision parents.
- **Collection / relationship**: explicit membership, order, relationship type and target semantics.
- **Observation checkpoint**: bounded captured-view identity and accepted-source cut, invalidated visibly when no longer valid.
- **Operation intent / result**: actor-scoped immutable mutation identity and recovery classification.
- **Application interpretation**: consumer-owned domain semantics, distinct from generic content validity.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All baseline native journeys pass in extension-free, disabled and failure configurations with no additional setup or dependency; any default-workflow regression blocks mission acceptance.
- **SC-002**: Paired default-path measurements meet NFR-002 and show zero implicit extension startup or registry parsing.
- **SC-003**: One independently running module completes the fake-tracker round trip, lost-ack restart, duplicate observation and stale/conflict cases without duplicate native issue/comment creation.
- **SC-004**: Raw ungranted/wrong-repository/incompatible requests refuse before prohibited effects in every documented denial case.
- **SC-005**: Custom document, list and ticket types plus an unrelated inventory model are created, revised, conflicted, searched and exported through the same generic contract.
- **SC-006**: A second independently initialized clone reconstructs exact public IDs/history/types/relationships without the original module or private state.
- **SC-007**: The charter-model consumer stores representative doctrine artifacts, a pack and an explicit exact-revision selection; merely storing or searching a directive activates nothing.
- **SC-008**: All documented resource limits and lifecycle failures have meaningful refusal/cleanup evidence; no silent truncation or success-on-timeout is accepted.
- **SC-009**: Every claimed platform/variant has bound package evidence and every unsupported capability or missing observation is visible.
- **SC-010**: A new native user completes init, ticket/discussion and exchange tasks without encountering extension configuration; a separate extension-author trial uses the published guide and returns actual friction.

## Assumptions and sequencing

1. The owner requested mission packaging, not implementation or publication in this turn.
   Specification and implementation planning are authorized; work-package execution is not begun.
2. The existing beta mission remains authoritative for its own support contract and
   independent pilot. Its native contribution repairs must be integrated and
   re-inspected before extension implementation. The f8c68beb candidate is a
   research reference, not a replacement beta gate or an automatically chosen release base.
3. The Spec Kitty-shaped demonstration is a feasibility/reference consumer, not a
   production plugin, complete doctrine implementation or claim of compatibility
   with every moving upstream charter schema.
4. First capability scope covers the supported replayable native resource families,
   custom records and observation. Broader governance/execution/transport broker
   surfaces remain explicitly unsupported until separately specified.
5. NFR-002 budgets are initial engineering acceptance limits, not published latency
   promises. Relaxation requires a documented reviewed design change, never a retry to green.
