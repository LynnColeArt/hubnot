# Implementation Plan: Optional extensions and custom types

**Branch / planning base / mission merge target**: feature/optional-extensions
**Date**: 2026-10-30
**Spec**: [spec.md](spec.md)
**Mission**: optional-extensions-01M661C0
**State**: Planning only. No work packages, code implementation, platform execution,
beta acceptance change or publication is performed by this plan.

## Summary

Deliver two additive capabilities: a portable typed-artifact model, usable without
plugins, and an explicitly started external extension host exposing supported native
and artifact operations. Preserve the ordinary native dispatch path and existing
signed protocols. Demonstrate a fake-tracker connector and a Spec Kitty-shaped
information model outside the core; do not implement doctrine semantics in Hubnot.

Engineering alignment comes from the confirmed conversation: Go/Git native core,
optional external modules, immutable history, explicit authority, bounded operations,
default workflow as a hard contract. Mechanisms below are engineering defaults.

## Technical Context

**Language/Version**: Go 1.26 (existing go.mod); retain the approved release toolchain
and recheck its exact patch version at implementation. Reference processes are Go
executables built separately; no Go installation is required by native package users.
**Primary Dependencies**: Go standard library and Git; no new runtime or Go module
dependency planned. Use a deliberately bounded declarative schema subset rather than
a downloaded general scripting/validator framework.
**Storage**: Signed content-addressed Git objects on additive custom-artifact refs.
Local registration/grants/checkpoints/uncertain-intent state remain private local files.
**Testing**: Plain Go/Git unit and CLI contract tests, real temporary bare remotes,
external reference processes, fault controls and exact standard/native package runs.
**Target Platform**: Linux/macOS AMD64/ARM64, standard and nogithub,noforge combined
native-only variants. Windows stays a separately planned track.
**Project Type**: Existing single Go CLI plus optional external reference executables.
**Performance Goals**: Default paths do not load the registry/start modules; apply
spec NFR-002 paired 30-observation median elapsed/RSS budgets with raw samples.
**Constraints**: No existing signed-byte rewrite, default service, provider account,
memory mutation API invention, implicit execution or generic governance bypass.
**Scale/Scope**: Host request/response <=8 MiB; artifact envelope <=1 MiB;
type declaration <=128 KiB including schema <=64 KiB; schema/value nesting <=8;
<=64 object properties and <=1024 array/list entries; page limit 1..200/default20
subject to total frame bound; captured set <=10000 entries/16 MiB.
Finite host reads default30s and mutations default240s, maximum240s, startup10s, stderr64 KiB, one outstanding
request per session and four sessions per canonical repository. Lifecycle limits
and response limits are tested, not silently expanded. Existing native operation
stage limits remain authoritative and may be stricter.

## Charter Check

HN-001: Git stores public collaboration; no new mandatory service/database/provider.
HN-002: Each result binds exact record, type/schema, revision parents, actor/operation
and source/view cut. Observation is descriptive, not authority.
HN-003: Inspection is inert, raw host requests are grant-checked, schema evaluation
is bounded and non-executable, unsafe execution remains outside generic grants.
HN-004: Conflicts retain every head and require explicit successor resolution.
HN-005: New artifact/host contracts, CLI examples, threat boundaries and recovery
documentation ship together with meaningful public-boundary evidence.
DIRECTIVE_030/034/036/041: Preserve baseline/red controls, test live entry points,
run appropriate Go checks and require non-vacuous failure tests.
DIRECTIVE_024/033/044: Narrow additive surfaces, targeted commits, canonical workflow
commands. No broad conversion of native records or memory into a replacement backend.
No charter violation or exemption is proposed. Recheck after implementation design
and before acceptance; planned tests are not observed passes.

## Architecture and default-path boundary

~~~mermaid
flowchart LR
  U[Native user] --> N[Existing native commands]
  A[Explicit extension invocation] --> H[Optional local host]
  H --> G[Repository and action grants]
  G --> N
  G --> R[Generic artifact operations]
  P[External application] <-->|Versioned stdio| H
  R --> S[Signed custom Git histories]
  N --> E[Existing native Git histories]
  V[Generic inspection and export] --> S
~~~

The registry is accessed only by explicit extension commands. Artifact commands read
portable artifact data, not executable registration. Native command execution has
no mandatory dependency on either layer. Schema/type declarations and cloned
manifests never select executables or activate a module.

### Typed artifact layer

Introduce one explicit generic protocol (planned hn-artifact/1) with document/list/
ticket shapes, namespaced type IDs and exact signed schema revision references.
Keep custom revisions out of hn/0 actor event chains and hn-memory/0 streams.
The generic layer owns envelope/identity/bounds/schema/parent/closure validation;
the application owns doctrine, business meaning and specialized projections.

Use a portable, closed validation subset: bounded object properties, required fields,
scalar types, enums and bounded arrays. Disallow remote schema references, arbitrary
code, recursive schemas and executable validators. Unknown keywords/versions refuse
as unsupported rather than being silently ignored. Exact text is stored as UTF-8
content, preserving its bytes/digest; structured payload is canonical JSON.
No binary attachments in the first protocol. Generic regex uses the existing bounded
Go-regexp design: pattern4096UTF-8 bytes, expansion/compiled4096instructions and
64Mi instruction-bytes estimated work over the complete filtered set before output.
Type names use lower-case ASCII namespace/name segments of1..64characters each,
starting with a letter and continuing with letters, digits, dots or hyphens; no path aliases.

Type names are scoped descriptive identifiers bound to exact declaration digest
and signer; they are not ownership of a global DNS namespace. Reserve Hubnot's
built-in namespace and commands. Registration cannot shadow built-ins.
Record identity is its signed opening ID. Revisions name this logical ID, exact
schema and full expected parent revision IDs; multiple actors can revise the same
logical record on their own immutable streams. No timestamp winner. Explicit
resolution names all observed current heads. A schema update never migrates old
records automatically.

Collections carry explicit stable member IDs/order. Links declare logical target
or exact revision targeting. Cycles in domain links do not imply global invalidity;
generic traversal uses visited sets and finite bounds. Required missing dependencies
produce explicit incomplete diagnostics or refuse promotion, never truncated facts.
Application semantic validation is labelled separately from generic validity.

Transport is explicit selection plus bounded verified closure in quarantine before
publication, with expected-old refs and real two-clone proof. Extend native transport
selection narrowly for the new protocol; do not loosen existing unknown-kind checks.
Older clients retain existing supported native commands/histories, but are not
claimed to understand the new protocol. Extension-aware plugin-free clients support
generic inspection/search/export. Public export never copies private registration,
credentials, signing roots, local journals or grant state.

### Extension registration and local host

Register a manifest locally; canonicalize repository identity through established
physical Git/worktree checks. A repository declaration is inert discovery content.
Explicit enablement binds one registered executable/argv, module identity, protocol,
repository and action/family/type grants. A changed executable registration requires
fresh explicit enablement; manifest names and request fields cannot self-authorize.

Choose one transport: versioned length-delimited JSON over private stdio with one
host-started external process/session. Start via executable plus argv, never a shell.
Host and application exchange the typed messages in contracts/host-v1.md.
The CLI host is explicitly invoked and is not a mandatory daemon.

First adapters: public identity inspection; reliable native issue/project/item/
discussion/reply/workflow-definition reads, writes and operation lookup; generic
artifact/type/collection reads/writes/lookup/search; accepted change polling.
All other current command families appear in inventory as unsupported or unavailable
with explicit recovery class. No arbitrary CLI argv bridge or automatic migrate apply.
Type publishing, create, revise, resolve, observe and raw inspection are distinct grants.
Do not infer policy, identity lifecycle, run, review, decision, merge or audit grants.

The module request actor must match the selected actual signer; host-mediated writes
never export keys. Existing native operation functions remain the validating writers.
Acquire/release native contribution/actor locks per request; do not hold a repository
write lock for the whole host/child lifetime. Inspect beta's outer command-lock
classification and direct native entry points before adding host dispatch.

Propagate the host request context through existing context-aware native boundaries;
preserve reservation/stage deadlines and documented synchronous-call limitations.
Bound startup, frames, stderr, captured pages, deadlines and concurrency. Cancellation
returns a truthful error; terminate child then kill after2s, confirming exit within5s.
Never relabel a completed-but-unacknowledged mutation cancelled/absent: return uncertain
and recover using its immutable actor/operation identity. Local pending intents and
public results are separate evidence. Disable stops local sessions and preserves facts.
Same-user processes still have filesystem powers; this is host mediation, not a sandbox.

### Observation and recovery

Produce ordinary changes from the same verified accepted visible projection as native
readers. With contribution rules enabled include admission/moderation/retention view;
do not read raw actor chains as if every signed record were visible.

Capture a bounded source/authority/policy/selection fingerprint. Retain captured sets
for a 10-minute session lifetime or return explicit invalidation when they cannot be
validated. Source changes may invalidate pagination; moderation/authority changes
always invalidate the old visible view. Return bounded rescan guidance and let clients
deduplicate by repository/view/full-event identity. Ordering by full ID is presentation,
not global causal/time order. Invalid histories return verification failure.

Native issue/resource replay uses existing exact actor/operation recovery. Add equivalent
durable intent and public lookup semantics for newly introduced artifact mutations.
Never claim native memory writes, proposals or run requests are replay-safe by analogy.
External connector workers own their API credentials, scheduling, export journal and
remote response-loss reconciliation. Hubnot attests only the native operations it sees.

### Contribution policy combinations

Existing native adapters must preserve the beta's governed projections, locks,
reservations and authority. Custom artifacts do not automatically join the current
contribution outer profile. Initially, custom writes under selected contribution
policy are explicitly unsupported and refuse before mutation; inventory advertises this.
Generic custom collaboration works with contribution policy absent. A later explicit
bounded custom admission profile is separate scope, not a bypass hidden in this mission.
Read paths still disclose exact source/validation/view semantics.

### Two reference consumers

1. Fake tracker: separate executable using only the host contract, with its own durable
   external-intent/result journal. Prove import issue/comment, lost native ack/restart,
   export observed edit, duplicates, no echo, conflict refusal and raw grant denial.
2. Charter-model: separate executable publishes representative document/list/ticket
   schemas, directive/tactic data, typed links, a pack and an exact charter selection.
   This is a representative storage/retrieval contract proof, not a doctrine resolver,
   production Spec Kitty plugin, migrated charter runtime or semantic conformance claim.
   Include an unrelated inventory model through the same interface so core cannot pass
   by recognizing doctrine names. No Java/Python/MCP dependency in native Hubnot.

## Project Structure

### Documentation (this mission)
- spec.md, intake.md, plan.md, research.md, data-model.md, quickstart.md.
- contracts/host-v1.md, contracts/artifact-v1.md, contracts/default-workflow.md.
- checklists/requirements.md and checklists/planning.md.
- tasks.md and tasks/WP*.md are later task-authoring outputs, not created by this plan.

### Prospective implementation surfaces
- internal/artifact/: pure data model, canonical validation, schema subset and limits.
- artifact_*.go: existing-identity mediation, Git storage, catalog, CLI, search and exchange.
- internal/extension/: private registration/grants, framing and lifecycle helpers.
- extension_*.go: explicit CLI dispatch, native adapters and accepted observations.
- examples/extensions/fake-tracker/ and examples/extensions/charter-model/: external
  consumer executables and fixture-only domain schemas.
- Existing main.go, replication/quarantine selection, release packaging and public
  journey helpers only where explicit integration requires them.
- docs/extensions.md, docs/artifacts-v1.md, docs/extension-consumers.md and narrowly
  relevant protocol/recovery docs. No global memory/native record rewrite.

Actual file ownership, profiles, work-package boundaries and lane DAG are authored
only in the later tasks phase after refreshing the integrated beta base.

## Qualification and rollout

Before implementation, integrate the approved beta foundation into the mission's own
target and re-inspect its exact source. Resolve known prerequisite failures such as
issue23 through their own disposition; do not skip them or count an older main rebuild
as exact beta-candidate evidence. Record baseline source/toolchain/host/native outputs,
30 paired timings/RSS and full historical golden hashes before touching implementation.

Keep the default-path comparison release-blocking. Run it without a registry, with
disabled/missing/incompatible/crashed modules, with malformed local registry and with
large unrelated custom histories. Markers must prove a forbidden loader/start/probe
would be detected (deliberate negative control), rather than checking only exit0.
Preserve existing signed goldens, meaningful refusal assertions and default optionality.

Use appropriate focused model/host/transport/CLI tests; then gofmt, vet/build, full
uncached race suite at a fixed source, and standard/native exact package journeys
on each claimed target. Serialize local heavy execution. Native-only means the shipped
combined tags, not the unshipped nogithub-only asset. Record all actual binary/source/
compiler/variant/hash identities. Missing hardware/browser/user evidence remains pending.
Reference binaries are explicitly supplied test assets; packaging them never becomes
a native runtime requirement.

First adopter checks: a native user follows the unchanged guide without extension
configuration; an independent extension author follows the consumer guide and returns
friction. Optional hosting, standing acceptance, Windows/XP and production doctrine
integration remain separate initiatives. No beta gate or published release is amended.

## Implementation Concern Map

These are architectural concerns, not executable work packages or ownership claims.

| Concern | Purpose and requirements | Prospective surfaces | Dependencies and risk |
|---|---|---|---|
| IC-01 | Baseline/default contract: FR-001/002, NFR-001/002/005, SC-001/002/010 | Native journey helpers, historical corpora, metrics fixture | Approved beta base first; non-vacuous no-loader controls |
| IC-02 | Portable type/envelope validation: FR-009/010/012/015/016 | internal/artifact model/schema/canonical tests | IC-01; preserve exact existing protocols |
| IC-03 | Custom signed storage/closure/transport: FR-011/014/017/018 | artifact store/exchange, narrow replication integration | IC-02; stale signer, CAS, budget and unknown-ref boundaries |
| IC-04 | Generic CLI/search/recovery: FR-007/013/015 | artifact commands/catalog/search/operation | IC-03; replay and source-closure uncertainty |
| IC-05 | Inert local discovery/grants: FR-002/003/004/005 | internal/extension registration and local config | IC-01; zero native-path registry parsing |
| IC-06 | Stdio host/lifecycle: FR-005/019/020 | framing/session/child lifecycle and explicit extension dispatch | IC-05; argv/no-shell, per-request locks, finite cleanup |
| IC-07 | Native/artifact operation adapters: FR-006/007/017/018 | extension adapters and native operation boundaries | IC-04/06; actual actor pin and no parallel writer |
| IC-08 | Accepted visible changes: FR-008/017 | bounded capture/checkpoint/invalidation helpers | IC-07; no raw-history substitute for governed visibility |
| IC-09 | Fake-tracker consumer: FR-021 | examples/extensions/fake-tracker | IC-08; external journal and no echo/duplicate native facts |
| IC-10 | Charter and unrelated models: FR-022/023 | examples/extensions/charter-model and inventory fixture | IC-08; no doctrine-specific production symbols |
| IC-11 | Exact-package/default qualification: NFR-001..008, SC-001..010 | bounded CLI fixtures, Go-only packaging and host matrix | IC-09/10; all source/binary labels and missing evidence honest |
| IC-12 | Consumer docs and independent boundary review: FR-024, all constraints | contracts and maintained guides, focused review | IC-11; actionable fixes get explicit owners and independent review |

## Complexity Tracking

The extra protocol and host are justified by the two distinct user needs:
plugin-free portable custom data and optional executable consumers. A generic process
host alone cannot provide persistent custom types; forcing them into memory text
cannot provide shared document revision semantics. A broad rewrite of existing native
storage and multiple speculative transports were rejected. No charter exemption.

## Phase 1 disposition

Specification and plan are substantive. Supporting model, contracts and qualification
recipe accompany this plan. Task generation, lane finalization, implementation,
mission acceptance and release are distinct later actions. No test pass is invented.


## Execution authorization and first-gate decision — 2026-10-30

The owner explicitly requested work-package authoring and implementation, with
preserving the default Hubnot workflow as the first gate. This supersedes only
the earlier planning-only delivery stage; product requirements remain unchanged.

The first work package owns a normal content integration of the exact approved
beta runtime f8c68bebbaa3845d1eaaac02e3638f30dc062bbd into its assigned lane.
This imports an already reviewed dependency, not author-written extension code or
acceptance of the incomplete beta mission. WP09 is outside that ancestry. Include
only the exact approved WP07 beta_scale_test.go blob
04f9587e36b571779153a8e783072d85ec22f6f3 separately, not the older WP07 source tree.
The resulting baseline is219Go/module inputs;105non-testGo/module hashes must match
f8. Source import metadata/ancestry and later test-only edits are reported separately.

Issue23's missing-promised-object test assumes a Git-version-dependent physical
inventory fallback. First-gate scope includes a narrowly owned deterministic test
correction after independent contract assessment: direct corrupt-ID lookup must
refuse, direct physical inventory must refuse corruption, and healthy/no-lazy-fetch
controls remain. No version skip, blanket error allowance, source-runtime change or
waived regression is authorized by this correction. Capture the repaired baseline
and perform the full declared validation before approval unblocks extension work.

Before affected feature code, a contract-only predecessor must pin exact signed
artifact bytes/ref grammar/schema subset, host registration/grants/method shapes,
replay/localCAS-versus-imported-fork semantics, cancellation/partial-frame/native
request-lock behavior, visibility removal/rescan reconciliation and opaque dependency
preservation. Independent findings are preserved in the external planning evidence;
they are not a claim these contracts are already executable or independently approved.
