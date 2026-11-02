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

## Approved-contract integration and qualification ownership

Following independent WP02 approval, WP17 owns the shared native Context seams in `resource_engine.go`, `issue_commands.go` and `contribution_moderation.go` exactly as frozen in `contracts/host-v1.md` section 3. Existing native entry points retain Background wrappers and their behavior. Host requests reuse existing per-request native locking, actual actor/policy/visibility checks and synchronous completion. WP16 retains sole ownership of `main.go`. This resolves analysis O1 through the real shared writers, without global context or duplicate implementations.

WP05 additionally owns the new `internal/artifact/canonical_export.go` path. Its live mutation CLI/store needs the existing strict canonical parser to preserve duplicate-key, Unicode and safe-number refusals. A minimal exported wrapper in the same package reuses that parser and its unchanged limits without copying it or editing approved WP03-owned files. The wrapper must have actual current production callers; it is not a general SDK or a new canonical encoding. Signed contracts, then-existing dependency approvals, native defaults and all 26 lane identities were preserved by this ownership correction. Root records this narrow ownership correction before the new path is implemented, with normal finalization and analysis refresh. The initial two small wrappers over existing validation logic are: `Canonical(raw)` delegates to `canonicalValue`, and `TypeDeclaration.ValidateOpening(ctx, raw)` validates the complete unsigned opening payload before signer selection or prepared intent. The latter delegates to `validationValue`, `payloadEnvelope("record.open", ...)`, exact declaration `type_id`/`shape` checks and the existing private `schema.validate`; a zero-value declaration or absent schema must refuse safely. It is not a fields-only validator. Both wrappers require immediate production mutation CLI/store callers; there is no parser copy, new limit, signed-contract change or approval waiver.

WP25 owns the narrow explicit extension selector/delegation in the already registered `.github/workflows/portability.yml` and the new `.github/workflows/optional-extensions-qualification.yml` reusable workflow. Existing beta defaults and PR/push behavior remain intact; explicit extension selection suppresses beta manual candidate jobs. Manual qualification at the reviewed feature ref must verify actual caller/workflow identity separately from candidate source identity, and execute all eight Linux/macOS AMD64/ARM64 standard/combined-native packages plus separately supplied target-bound reference assets. The same Go/Git journeys remain executable outside hosted CI. Missing execution stays pending; no default-branch registration, main change or release publication follows from this mission.

WP05 accepted-history integration subsequently demonstrated that the inert utility's 16 MiB composite-input cap must not become a selected-history context cap. Root reopened WP04 through the normal canonical rework path, preserving its original approval history, to extract one private record-value core in its existing owned content.go. The existing ValidateRecord utility retains its exact parsing/input cap and calls that core. After fresh independent WP04 approval, WP05 may expose a third minimal current-reader bridge, ValidateRecordContext, in its already-owned canonical_export.go; it delegates the same core on values supplied by the bounded, independently verified history loader, without giant composite serialization or a copied validator. Individual envelope, parent-count, schema/model and time limits remain unchanged, and the bridge does not itself attest accepted visibility or complete history. The retained valid approximately 19 MiB accepted-history failure must turn green through the actual public read. This introduces no new file owner, lane, protocol limit or default behavior.

## Mutation recovery integration clarification

The current type/open mutation callers already require the frozen intent/retry contract. WP05 therefore owns one complete, currently-used shared boundary: a bounded noncreating strict eleven-field descriptor/public-history evidence classifier; a lock-owning mutation attempt that reconciles the exact request before replay and mutable-head checks; and the sole append/finish path, with the original candidate durable before CAS and the result durable before acknowledgement. Queries do not create locks, use a signer, repair state or treat errors as absence. Mutations preserve one acquired per-key lock and the original immutable intent/candidate, without a second signer or publication implementation. WP06 successors reuse that live path, and WP07 exposes status plus real interruption/restart proof through it. No dormant recovery callback or alternate writer is introduced.

WP06 additionally owns internal/artifact/revision_export.go and revision_export_test.go. Its immediately-used successor preflight delegates existing strict parsing, payloadEnvelope and private schema validation, with explicit supported successor kind, bounded/sorted parents and exact declaration checks before indexing or signing. This adds no language, protocol or authority. The read-only architecture audit found that overlapping edits by WP07 would collapse the dependency chain and renumber active lanes without a supported live migration; factoring the already-required current writer preserves all 26 lane identities and current approved work. No lifecycle/ownership guard is bypassed.


## Prospective final-test maintenance ownership — 2026-10-31

The read-only platform preflight found older test detectors using Linux-specific config paths and one obsolete source-private-identity path. Before WP25 begins, root transfers only the eight listed test files exclusively from their approved original owners to WP25, preserving historical implementation/review attribution, all production ownership, dependencies, 26 WP/lane identities, 107 subtasks and canonical lifecycle/verdict history. The exact paths, repair limits and qualification obligations are in `tasks/WP25-exact-package-qualification/integration-config-path-maintenance.md`. Naively overlapping ownership would collapse active lanes and is prohibited. Normal validation/finalization and fresh analysis must succeed with unchanged lane mapping; otherwise root uses original-owner maintenance loops. All prior executable evidence remains bound to its actual historical source. No Mac/default/performance or final acceptance pass is inferred, and no product or frozen contract changes are authorized.

## Prospective bounded artifact I/O and qualification correction — 2026-11-01

Root adopts only the exact future integration-artifact-io.md/regions/verification records beside WP25's prior bounded-batch addendum, after complete reap of the H12f8 full source run. WP25 T101–T103 execute five narrow production exceptions under unchanged original owners: artifact_read.go/artifact_store.go (WP05), artifact_closure.go (WP08), artifact_replication.go (WP09), artifact_exchange.go (WP10). An additional TEST-ONLY exception replaces only inventoryHostShapeSearch's unordered separate-ID fetch loop in extension_inventory_acceptance_test.go (WP22) with one deterministic sorted fetch of all three exact final converted IDs; preserve every search/read/privacy assertion and product anti-rewind refusal. No ownership transfer or frontmatter mutation is authorized by this paragraph.

Keep 26 WP/lane identities,107 subtasks, WP25's14 owned paths, all13 create_intent entries, dependencies/authoritative surfaces and historical approvals exact. Full raw-reader sessions retain metadata-before-body/role+unique-byte guards, raw hash, old ls-tree/business verification, count preflight, witnesses and all30/240 budgets. Blob-only ordered ACK copying retains literal commit/tree writes, bounded owned scratch including unreachable exchange-table32MiB acceptance, exact first-detected-error diagnostics and mandatory all-return child ownership/EOF/Wait before any successful history/inventory/copy/ref advance. Qualified OS temp alias normalization stays inside the owned explicit qualification harness before any t.TempDir; arbitrary alias negative controls remain intact. No new Go path/dependency/default work or projectGit/tree-parser/adapter/lock/CLOEXEC/settlement/policy/identity/public-contract change.

The existing TEST-ONLY performance317/311 closed-union exception, numeric budgets, sample selection, ordering, math and controls remain unchanged. Require meaningful retained old/new public/causal controls, fresh independent source review, new root-attested311 hash values/all317 freeze, fresh default/control plus full4320 paired observations, full frozen source and all8 actual native packages, and normal independent WP25 review. No historical result is relabeled or reused as production-changed-source qualification. Normal finalizer planning-commit refresh and fresh analysis readiness precede normal WP25 implementation reentry; no force/skip/hand lifecycle event. No task/verdict/status completion or main/release action is recorded here.


## Prospective native retained-contribution I/O and checkpoint fixture correction — 2026-11-01

After the current H5 local full-source execution is fully reaped and its exact inputs/failure evidence retained, root may adopt only `integration-native-retention-io.md`, `integration-native-retention-io-regions.json` and `integration-native-retention-io-verification.json` beside WP25's existing addenda. This is T101–T103 work under unchanged owners, not a new work package, whole-file transfer or lifecycle verdict. Preserve all26 WP/lane identities,107 subtasks, WP25's14 owned paths,13 create_intent entries, WP24 dependency, authoritative surfaces and historical approvals.

The only new production exceptions are ReadContributionManifest's raw closure plus same-invocation payload/signature reuse in contribution_exchange.go, an adjacent native-local private helper in that existing file, and contributionRegisterRetained's raw closure in contribution_moderation.go. Use TWO SEQUENTIAL classic Git workers: `--batch-check` must admit ALL metadata and complete clean EOF/successful Wait/reap before lazy `--batch` can start. Native stderr remains `io.Discard` with retained storage0 and no cap; do not import artifact stderr limits/errors or require `--batch-command`. Preserve existing bounded enumeration/count/dedup/sort, all supplied limits/deadlines, framed headroom before body request/allocation, exact repeated OID/type/size, raw SHA1/SHA256 identity, original preflight/manifest/ls-tree/role/signature ordering and both independent plan observations. Retain registration locks/reservation/journal/coverage/ref/authority algorithms and permitted charged failure state. No change to shared projectObjectIDs/projectGit/project_snapshot.go, artifact I/O, default initialization/registry behavior, Go paths, dependencies or public protocol.

A separate TEST-ONLY exception in extension_changes_test.go (WP19 approved/inactive) replaces only the filesystem-checkpoint portion of TestExtensionChangesPrivateProofAccountingAndEviction and directly necessary local test helper. Set/read back distinct explicit mtimes and include an alternate creation-vs-mtime-order control; prove oldest-by-mtime eviction, exactly64 descriptors plus key/lock, and newest-valid. Keep the initial constructed200000-ref/128MiB-proof/256MiB accounting and later17×16MiB in-memory eviction checks unchanged. No production extension_changes.go or tie/FIFO policy change. The failed run's actual mtimes/readdir order were not retained: ordering is a source-supported hypothesis, not a recovered cause.

Only normal finalizer planning-commit refresh, fresh actual analysis/record-analysis and normal WP25 implementation reentry may activate this scope after root review. New production bytes require independent source/error/lifecycle review, a fresh root311/all317 attestation, unchanged default controls plus all4320 paired observations, full frozen race/static/build checks and all8 actual Linux/macOS standard/native package rows. Preserve current/historical results under their actual source identities; no cancellation, relabeling, deadline/cap/sample relaxation or inferred Mac pass. The separate Intel45-second migration watchdog failure remains unresolved by this design. No status/task/verdict completion, beta/main merge or release action follows from this amendment.


## WP25 native actor-chain transient storage amendment (2026-11-01)

Root adopts the reviewed `integration-actor-chain-storage.md`, `integration-actor-chain-storage-regions.json` and `integration-actor-chain-storage-verification.json` beside WP25's existing addenda. This is narrow T101–T103 integration work, not a new task, whole-file transfer or lifecycle verdict. Preserve all26 WP/lane identities,107 subtasks, WP25's14 owned paths and13 create_intent entries, original dependencies, authoritative surfaces and historical approvals.

The only production exception is the complete existing `validateActorChains` function at store.go:634–EOF at H541. Replace full StoredEvent copies in private actor groups with immutable input indexes, and replace the sorted sequence-to-ID map with adjacent same-sequence/different-ID checks. Preserve sort.Slice, sequence-only comparison, insertion/tie behavior, unspecified cross-actor first failure, and the exact per-row conflict then expected sequence then Previous error order. Same-ID duplicates remain refused; no global conflict prescan, input mutation, cache, parser/signature/relationship/authority/recapture/retention or public-contract change. The only test exception is append-only behavioral/error/nonmutation coverage in existing event_test.go plus necessary standard-library test imports. Keep the other315 of317 Go/module inputs exact H541.

Primary-only planning sequencing refinement: reviewed metadata adoption, normal finalizer refresh and fresh actual analysis may proceed in the PRIMARY checkout while the separate exact-H541 lane-x full-source run remains active, provided its HEAD/index/clean state/all317 inputs and primary220 inputs, canonical24 approvals and26 lane assignments remain verified after each operation. This explicitly supersedes ONLY the addenda's earlier first-paragraph requirement to wait before PLANNING metadata adoption. Actual WP25 implementation reentry (which merges planning into lane HEAD) and every Go edit MUST still wait for local session14372 to naturally finish, fully reap and retain its exact source/results. Remote archived H541 jobs continue untouched and source-specific. The root ordering decision is retained externally with SHA2565016273c35ae98261e61616655480b4995b8431999ce0a7b5224bd952795c9b9.

This removes demonstrably redundant native storage, not a proved cause of the failed peak. The eight-event default fixture's static header arithmetic does not assure12KiB RSS headroom. Establish existing behavior first, retain exact diagnostics and deep input immutability, then verify the implemented reduction and freeze a new clean source for independent review/root311+317 binding. All unchanged default controls, COMPLETE4320 paired observations, full-source race/static/build and actual native8 package rows remain required. Preserve H541's actual performance FAIL and its Linux/platform results under H541 identity; no source-padding, arbitrary layout shift, flags/threshold/sample/deadline relaxation, inferred Mac pass, beta/main merge or release.


## WP25 bounded Darwin fixture portability amendment (2026-11-01)

Root adopts only the reviewed `integration-darwin-fixture-portability.md`, `integration-darwin-fixture-portability-regions.json` and `integration-darwin-fixture-portability-verification.json` beside WP25's existing addenda. This is bounded test-only T101–T103 maintenance. Preserve all26 WP identities,107 subtasks,24 historical approvals, lane assignments/groups, dependencies, authoritative surfaces, WP25's14 owned paths and13 create_intent entries. Do not change frontmatter, runtime or lifecycle state. Actor-chain source-only approval for2c372ccd3f570da43ab68856518f88cfddbb7b7c remains source-specific and does not approve this future correction; its fresh default driver never started and stays held pending normal amendment/reentry authorization.

Only two existing test paths may change against frozen2c/tree8eda4f807f170c2b8f27ed639c13ff6deaf97bf3. In owned artifact_read_test.go, replace only the isolated child fdNames helper and insert tightly colocated pre-exhaustion controls: explicitly open /dev/fd, read names with File.Readdirnames(-1), explicitly close the owned observer on every return, make read/close errors fatal, and sort the complete unfiltered names. On Darwin both transient observer descriptors remain in the read and close before return; filter no name, including File.Fd(). Prove stable successive baselines, sensitivity to an intentionally held real descriptor, and exact recovery after close. Keep the original actual childPASS/exit/watchdog/noFAIL/noSKIP boundary, cancellation/warmup, child-only RLIMIT64/free0/2/4/6, restoration, nilprocess/nonnilerror and exact cleanup sets byte-exact outside these regions.

Root expressly grants a small out-of-map test-only exception for artifact_store_commands_test.go, originally owned by WP05 with its approval preserved and no concurrent original-owner writer. This grants no whole-file ownership, new lane/task or frontmatter change. Insert real repository-local Git maintenance.auto=false and gc.auto=0 immediately after real git init and before public hn init. Add only one named t.Run("snapshot-detectors", artifactCLISnapshotDetectors) insertion immediately inside the existing TestArtifactStorePublishCLI, preserving every original statement. This registered live native root calls one appended private full-snapshot detector helper, which owns actual shared fixtures for SHA1/SHA256: assert the local config values, take a full baseline, prove lock-named addition, content-only mutation and mode-only mutation each change the appropriate complete snapshot, then prove removal restores the full baseline. Existing imports suffice and remain byte-exact. Keep snapshot/unchanged bodies byte-exact, including all repo/home/file/lock bytes, modes and error handling; retain all real fetch/refspecs and original conflict/superseded/unselected assertions. No standalone duplicate Test, qualification registry edit, ENOENT ignore, lock exclusion, retry, global/environment/production maintenance policy or search change.

All other315 Go/module inputs, including store.go/event_test.go, stay byte-exact2c; preserve the same317-path keyset, no new Go file/dependency/production change. Retain H541 run41303228685/job124838806072's completed Darwin ARM standard/native-only FAIL results and two leaf assertion classes with zero skips/outertimeouts under H541 source identity. They establish no underlying production worker/search defect. Retain the old H541 RSS FAIL, give no allocation-as-RSS credit, and monitor old Intel job124838806032 separately without assuming a45-second fix.

Only root's normal metadata adoption, finalizer refresh, fresh independent analysis/record-analysis and normal WP25 reentry may activate this scope; separate implementation authorization is still required. Preserve the original78 dirty primary files byte/mode/status exact through root's established targeted archive/stash/restoration sequence. After meaningful test-first controls, require independent clean final-source exact-diff review, fresh root311+317 binding, unchanged six default roots and COMPLETE4320 paired observations, full frozen-source race/static/build checks and all8 actual native packages. Actual amended-harness Mac evidence must show stable/positive/recovery controls, all4 free cases and real childPASS, and reach original search assertions through real fetch without retry. Linux alone cannot prove Mac RED/GREEN. No gate/budget/sample/env/deadline/collector change, historical relabeling, formal WP25 approval/acceptance/merge/main/release action is authorized here.


## WP25 bounded Intel pair-wave and fresh-authority amendment v2 (2026-11-01)

Root adopts only the reviewed v2 integration-intel-redundant-work.md/regions/verification addenda against frozen c0ccb40a02d7ef3a3cb72724de8e7e0e6bd0d388/tree6a95ccc56ab253c4b390cc6c35fbc27edefe0f14. The concept-only independent SOURCE_DESIGN_FEASIBLE_NO_BLOCKING_FINDINGS verdict does not approve these exact payloads or future code. Preserve original87bc files as immutable historical design; old packed RW01 remains held, cat-file replacement rejected and cached-value RW02A rejected.

RW01C schedules the ORIGINAL exact GLOBAL64 show-ref primaries in two fixed-slot PAIR WAVES, same argv/stdout32768 and original synchronous64-pattern/count65/stdout33280 fallback. Coordinator alone consumes original primary-error/fallback/sentinel/raw parser/global wanted bodies in order, including unexpected globally selected rows. At most2 producers INCLUDING fallback, no replacement/next pair until both prior blocks consumed/joined/released. Original parent ctx is checked at logical entry and immediately after completion before prefetched consumption; these are explicit scheduling-boundary semantics, not literal time equivalence. Keep original failed-command checks and first parser/error priority; no fallback-success/postparse/cleanup override. Every return/phase cancels and joins all launched/completed-unconsumed jobs and releases pair state. The66048 maximum returned logical payload is derived accounting, not RSS or a new public budget. No projectGit/native-cat-file helper/collector/namespace/command/cap/cardinality/deadline change. Root must explicitly confirm inactive original WP05/no concurrent writer before the bounded artifact_read.go scheduler/adjacent minimal-owner exception.

RW02B inserts only the exact commit+digest branch before Historical's original merge-base, after its unchanged fresh context/repository prefix. Explicit context check at that boundary, fresh raw-commit pending check, bounded commit-typed actual policy read, fresh parse/digest/enabled/project/maintainer checks remain per record; return fresh data. Original noncurrent/wrong-digest suffix and ALL constructor/current/caller Source/plan instances remain byte-exact. No cached value/context/state/parser/retention. Confirm inactive original beta-readiness WP12/no concurrent writer and preserve historical approval/ownership before this bounded contribution_records.go exception. Only nested live controls in the two existing registered artifact_read_test.go roots plus private EOF helpers accompany these scopes; original tests/imports/approved Darwin bytes stay exact.

All other314 Go/module inputs, all317 keys, source/default/native collector/limits/corpora/IDs/maps/receipts/deadlines, approved strictsnapshot/live detector and actor optimization remain exact C0. Preserve26WPs107tasks24reviews14owned13create, frontmatter/lanes/groups/deps/ownership and original78 user paths/current220 baseline. Root normal metadata-only adoption/finalizer/fresh independent analysis/record/reentry and separate explicit implementation permission are prerequisites; these records authorize no code/runtime action. All unchanged qualification/trial/acceptance gates bound by root714a/0dc and v2 verification remain; H5414PASS/2ARMFAIL/2IntelINCOMPLETE/RSSFAIL stays historical. No main/release, timeout/cap/sample/env/coverage waiver, retry-to-green or substitution.


## WP25 prospective import-union validation correction (2026-11-01; ROOT PENDING)

This is an external proposed amendment for existing T101–T103 integration work, pending exact root review/adoption, normal finalizer refresh, fresh independent actual analysis/record-analysis, normal WP25 implementation reentry and separate bounded producer authorization. Read `tasks/WP25-exact-package-qualification/integration-import-union.md` and its regions/verification JSON. No source, test, runtime, ownership, approval or lifecycle authority is granted by this draft. Preserve all existing planning content and all52 requirements,26WPs,107subtasks,24 historical approvals, dependencies, lane/group identities, authoritative surfaces, WP25's14 owned paths and13 create_intent entries; frontmatter and runtime state stay unchanged.

Against frozen c84402877996015cf33784bebd826e5be4981a4a/tree e118ad7d65f594ca856cce43393488c067596978, independently confirmed BR-001/P2 is source-only and unexecuted: individually valid incoming and current artifact histories can form an invalid union after accepted-ref promotion. The new inactive-original-WP10 production exception for importArtifactExchange in artifact_exchange.go and test-only exception for artifact_exchange_commands_test.go are ROOT PENDING. Prior artifact I/O exceptions do not cover this correction. Root must explicitly confirm original WP10 is inactive, no concurrent original-owner writer, exact bounded regions and unchanged historical approval/ownership; no whole-file transfer.

Follow the bound independent design: validate exact P=current.Sources overlaid by promoted destination imported refs in disposable quarantine, using the unchanged central loader and original validation context, before any receiver object/ref/signing/journal effect. Prefer import-local healthy-ODB read-only borrowing, with exact verified public-object inventory fallback only for otherwise already admitted/verified layouts. Preserve existing unsafe-layout refusals; fallback never bypasses them. Stage P witnesses with a bounded existing batch transaction using actual stage-old refs. Keep original validation context/cancellation and read/catalog limits; literal borrowMainObjects transplantation and per-ref staging subprocess loops are insufficient. Preserve source recheck, actor-prefix/rewind checks, selected-only copy/promotion, expected-old atomic CAS, lost-ack uncertainty, opaque behavior and post-promotion loader. No rollback, new clock, limit waiver, cache, packing, queue or shared-helper refactor.

Append a named live helper/subtest to existing qualification-selected TestArtifactExchangeRefusalsCLI; preserve its original body/assertions. In both Git SHA1/SHA256, real signed public CLI import must refuse a receiver-present wrong-kind optional target with invalid_history, exact full receiver no-effects and ordinary readability. The identical bundle must succeed at a receiver missing that target and report missing_optional. Meaningful authorized baseline RED, corrected GREEN and positive-control evidence must bind actual production/test/binary identities; this draft executes none. Other315 of317 Go/module inputs and all317 keys stay exact c844; no collector/workflow registration change.

C844 default144 criteria PASS, full-source7PASS and native run41349086217 attempt1 jobs LIVE at preparation are source-specific historical evidence, not corrected-source qualification or a current remote status observation. Preserve their identities without cancelling/rerunning/relabeling. Corrected source requires fresh independent exact-diff/source review, fresh root311/all317 attestation, unchanged six default roots and COMPLETE4320 paired observations, full frozen-source race/static/build and all8 actual native package rows, then normal WP25 review and later WP26 cold trials/52 adjudications. Default workflow is a hard gate throughout. No main/release action or gate/budget/sample/environment/deadline relaxation.


## WP25 prospective child-only E2BIG fixture portability amendment (2026-11-01; ROOT PENDING)

This is external proposed T101–T103 test-only maintenance, pending root's exact review/adoption of integration-e2big-fixture-portability.md and its regions/verification JSON, normal finalizer/planning refresh, fresh actual independent analysis/record-analysis, normal WP25 reentry and separate bounded producer authorization. No source or lifecycle authority is granted now. Preserve52 requirements,26WPs,107subtasks,24 historical approvals,14 owned paths/13 create_intent entries, all dependencies/lanes/groups/authoritative surfaces and frontmatter/runtime state.

Against frozen43574af5b57217df15a6ccf0b8f5c29cadc28593/tree13e20b3be58f14e0cb394a5ca999ec7ceae4437a, replace ONLY intelWitnessLowFD's existing E2BIG child-only256KiB generated environment value with2MiB, plus at most one directly adjacent explanatory comment. artifact_read_test.go is already WP25-owned; no ownership expansion is requested. Preserve every other statement/assertion/import/helper and actual verification_failure/real syscall.E2BIG/nilstdout/fullFD recovery/reaped childPASS/noFAIL/noSKIP boundary. No host-limit query, adaptive calibration, ambient environment dump, native dependency, application budget, production, collector, workflow, threshold, deadline or corpus change.

Explicitly supersede the latest import-union other315-of317 C844 freeze ONLY for artifact_read_test.go. Preserve artifact_exchange.go and artifact_exchange_commands_test.go exactly4357, leaving the combined other314 inputs exact C844. All316 inputs other than this new fixture delta remain exact4357;317-key set unchanged. Preserve historical metadata/evidence and current4357 fixes. Historical C844 Darwin ARM run41349086217 attempt1 job124975681149 has four real E2BIG leaf failures (sha1/sha256 × standard/native) with expected mapped refusal absent; this proves a fixture failure, not an underlying product defect or current4357 outcome. The supplied upstream Mac1MiB bound explains the hypothesis; actual runner getconf/sysctl was not observed.

ROOT53154 terminated exit143 at approximately00:06:04UTC: first race check INCOMPLETE with2520 namedPASS/0FAIL/6 known helper skips, no driver results or after-file; six later checks NEVER STARTED. Signal15 cause is unknown, neither testPASS nor proven infrastructure failure. Adoption/source edits require the interrupted ROOT command and all owned children fully reaped, exact manual all317 source-after/identical4357 source+tree plus partial/failure evidence preserved, and no remaining local check writer. Root reports no relevant processes and a clean working tree; the source-after capture remains a root verification obligation. Old4357 full7 stays INCOMPLETE and cannot qualify/approve; do not rerun it merely to satisfy the superseded all-seven-completed wording. After these conditions are verified, root may normal-adopt the known bounded fixture correction and separately authorize a new local candidate. OldC844/current4357 hosted jobs continue/finish independently against immutable checkouts/aliases. No alias rewrite/cancel/rerun/relabel or same-local-source overlap. Fresh new-candidate qualification still requires all unchanged7 checksPASS plus every default/native/other normal gate; no waiver or action authority now.

Retain actual old256KiB Mac RED; require corrected2MiB actual native8 GREEN with every existing genuine errno/nilstdout/fullFD/child assertion, and existing healthy selected-size/full-public-read controls with exact identities. Actual Mac RED/GREEN and healthy controls remain mandatory through preserved old/current or new source-bound observations, as well as Linux; Linux alone is insufficient. A separately authorized local size-to-zero fault injection is optional supplementary source-causal evidence, not a new mandatory Mac/native collector gate or requirement to rebuild/execute current qualification assets. Preserve source-only versus supplied-binary distinctions. Require independent clean exact-diff/source review, fresh root311/all317 binding, six unchanged default roots/collector and complete4320 observations/2160pairs/72groups/144criteria, full frozen-source race/static/build and all8 actual native rows. Default remains a hard gate; formal WP25 review, normal WP26 eligibility/two true cold trials/52 adjudications and every acceptance/merge gate remain. No historical relabeling, main or release.


## WP25 prospective native cadence/discovery-witness amendment (29ffc43c; ROOT PENDING)

Normal activation remains HOLD. Root review of this exact six-file draft, independent metadata review and root adoption are pending; design-only approval is not metadata adoption. Root must preserve current failed results and finish the authorized local RSS diagnostic: its root driver AND every owned child fully reaped, exact all317 source-after matching frozen29/source+tree, and no local source/test writer. This curation does not inspect processes or certify those conditions. WP16 and WP05 inactivity/no external writer are ROOT PENDING. Then normal preservation/adoption/finalizer, fresh independent actual analysis/record-analysis, normal WP25 implementation reentry and separate bounded producer authorization are required. No source or lifecycle edits are authorized by this draft. Old remote checkouts/jobs/aliases may finish independently under immutable original identities: no cancel, retry, alias rewrite, relabel or same-local-source overlap. Current29's17 RSS FAIL remain blocking, not erased by diagnostics or this amendment; the next candidate needs all original gates anew.

Read `tasks/WP25-exact-package-qualification/integration-native-correction.md` and its `-regions.json`/`-verification.json` before any separately authorized work. Independent review APPROVE_DESIGN_ONLY is source/metadata/runtime-unapproved.

At most four existing Go paths: internal/extension/session.go (WP16 T064–T067; one ticker expression), internal/extension/session_test.go (WP16; append focused controls and only necessary imports), artifact_read.go (WP05 T021/T023; private loader factoring only), artifact_read_test.go (already WP25 test-owned; append focused controls/necessary imports). Original ownership/authoritative surfaces and reviews remain; no whole-file transfer. WP25 T101–T103 carry the integration work, with no new task/WP. No new Go key/dependency; all other313 of317 inputs must be byte/mode exact29 after implementation. Both4357 import-union corrections and29's2MiB E2BIG expression/all inherited assertions remain exact. Preserve52 requirements/107 tasks/26 WPs/24 historical reviews/24 FRs pending,14 WP25 owned paths/13 create_intent entries, dependencies/lanes/groups/frontmatter and primary originals/modes/status/stashes.

Candidate A changes only session.go:498 to `time.NewTicker(RevocationInterval / 2)` for active optional sessions; published250ms observation/four sessions/2s TERM grace/5s exit and every owner/Recheck/per-request/shutdown check remain. Real≤250ms evidence, active-only Git/hash I/O and meaningful old-source RED are required, with no tolerance.

Explicitly supersede RW01C's initial-witness requirement ONLY on the ordinary loadArtifactHistory composition that itself just calls artifactSources. Prior RW01C does not authorize B. Keep captured discovery sources unchanged for the outer comparison: admission copies before sorting. Exactly two private compositions: explicit supplied sources → unchanged admission → original initial witness → shared verified body including final witness; actual local discovery → same admission → same body → fresh complete source comparison → Cut. No generic skip flag/token/provenance claim/callback/third entry or cache. Keep artifact_read.go:766–985 original worker/pair/primary/fallback/sentinel/parser/error/argv/caps/cancel/join bytes exact29; explicit source entry/signature and both rounds remain exact for artifact_exchange.go:679/:1140 and artifact_replication.go:329/:442. Retain replication's separate recheck and all downstream pre-effect/CAS boundaries. Ordinary calls inside mutation preparation, host catalog/change reads, export/import current-state reads and replication ALSO take B when they locally discover; only explicit supplied-source compositions are unchanged. No fetch or broader history. All admission/raw count/object/type/hash/signature/actor/schema/chain/aggregate/cleanup checks, final per-ref witnesses, final full-selection comparison and Cut remain. First/final complete captured selections are freshness boundaries, not universal ABA elimination or atomic-instant snapshots. Discovery-only omitted-round transient errors and mixed-invalid precedence may differ exactly as the independent review matrix below accepts; no universal unchanged-diagnostics claim. Every executed worker/body error remains binding; explicit-source diagnoses/order remain unchanged.

All original gates remain: independent exact-diff/source review, actual fresh311/all317 input binding, six defaults plus COMPLETE4320 observations/2160 pairs/72 groups/144 elapsed+RSS criteria PASS, full7 checks PASS, actual qualified8 native rows at original deadlines,100000/100001 and separate99999 causal controls,16384/16385/maps/receipts/policy,45s migration, real raw traces/detectors/three standalone consumers. Formal WP25 then normal WP26 eligibility/two fresh independent public-docs-only cold trials and52/24 preaccept judgments remain. Noncircular P01–P05 criterion/preaccept gates precede P06 accept→feature-only merge→postmerge review/retrospective; P06 remains mandatory mission work. No main/release, historical relabel, threshold/sample/corpus/deadline waiver. Mac native/artifact stage work is DIAGNOSTIC ONLY; no fixture request-budget, gate wait, marker or third production correction is scoped.


## WP25 prospective DD Intel readiness, recovery-fixture and qualification-method amendment (ROOT PENDING)

External draft only: root has not adopted these payloads or authorized source work. Read `tasks/WP25-exact-package-qualification/integration-dd-intel-correction.md`, `integration-dd-intel-correction-regions.json` and `integration-dd-intel-correction-verification.json` together. Independent design review recommends A/C and B initial bounded diagnostics only (report SHA256 `1ff3518f09f9db1b3f9fb8a9b4114b6b96bb7119d447ec75b50455259939eb75`); F1/P1 holds later B fixture activation. Root adoption and exact metadata review remain pending. This is bounded existing T101–T103 integration work; T100–T103 remain unfinished. No new requirement, task, WP, ownership transfer, frontmatter, dependency, lane/group identity, historical review or acceptance transition is proposed. Preserve all52 requirements,107 subtasks,26 WPs,24 historical approvals, WP25's14 owned paths/13 create_intent entries, authoritative surfaces and original user bytes/modes/status/stashes. COORD remains24 approved/WP25 in_progress/WP26 planned; PRIMARY historical status is not lifecycle authority.

Frozen source is dd906146b9055a874a9992c5a488d22fcb490a50/tree c34b4d2efaa010df68bbff33738c773f6e9501f3. This amendment builds on, and preserves, the inherited native cadence/discovery-witness, import-union, E2BIG, actor, count, A/B and RW01C/RW02B corrections. At most four existing Go paths plus the existing qualification workflow are prospective: WP20's fake-tracker main.go ready-publication region and focused main_test.go controls; WP18's extension_artifact_acceptance_test.go child/interrupted-recovery fixture regions only; WP25's extension_package_acceptance_test.go harness/registry/evidence and narrowly necessary workflow regions. Confirm WP20/WP18 inactive/no concurrent original-owner writer before separately authorizing their bounded exceptions. No whole-file transfer, new Go/module key, production artifact/host/session/store or general helper refactor. The prospective four-Go delta leaves313 protected Go/module inputs byte/mode exact DD; this is distinct from fresh311 inherited qualification identity and all317 current-input identity, both still required. Metadata adoption itself changes none of317.

A publishes the existing complete raw URL through a private0600 same-directory temporary file, checked full write and successful close, then rename, cleaning every failure and preserving listener/error behavior; no JSON URL or consumer tolerance. Strict polling/timer/over-limit/no-effect assertions stay exact. Deterministic child-only actual EFBIG controls must make old serve fail on final readiness exposure and corrected serve retain absent final readiness, plus unrestricted actual HTTP/cleanup controls. That causal OLD RED does not prove the old CI syscall schedule.

B is INITIAL BOUNDED DIAGNOSTICS ONLY: bounded16KiB phase observation/selected argv≤4KiB and actual checked selected-stage/ref evidence. Actual phase evidence, independent assessment and root's explicit decision precede any fixture or stage-budget correction. All proposed stage-driven framed cancellation/deadline decomposition, a new2000ms deadline subcase, Git-only pre-dispatch gate/expected cancelled classification and provisional20000ms request/10s stage-arrival candidate remain HELD, with no implicit permission. A controlled bounded pre-stage delay on the actual supplied host can demonstrate the original2000ms/late-stage conflation without reproducing Intel runner latency. Such evidence does not choose a new budget or establish actual Intel phase cause. Any later reviewed semantics decomposition must preserve the strict original2000ms first-byte timer, selected real-Git prepared/lost-ack evidence and exact cancellation semantics, ref/event/no-effect/replay/conflict/rotation/keyless/final-signing assertions and existing cleanup. All original host/product timers/caps remain hard. No arbitrary allowed-code union or timer-expiry substitute for framed cancellation.

C explicitly proposes two fixed serial groups (heavy4, remaining202; exact original ordered206 inventory) for both variants in the same four native jobs using the same binaries/references/tracing/isolation. Keep120m per invocation and300m per native job. If root adopts C, it supersedes ONLY the contradictory earlier internal assembly-method requirement that all206 roots complete in one aggregate120m qualification process: one row can instead use two120m invocations. It does not establish the old one-process120m result or improve product performance; full-source120m and all product timers, caps, corpora and assertions remain hard and unchanged. No matrix expansion, retry, parallel roots, hardware change, omission or cap increase. Only a validated same-identity composite of both groups may be a whole row: exact206 disjoint union, all nested runs ended successfully, no missing/duplicate/extra/overlap, fail/skip, invalid identity or control fallback, original trace and source-after evidence. Eight actual composite target/variant rows remain mandatory. The existing300m job can still fail and stays FAIL.

All historical failures remain FAIL and all incomplete results remain incomplete. Current immutable remote jobs/checkouts/aliases may finish; do not cancel/retry/rewrite/relabel. Normal root metadata-first preservation/adoption/finalizer refresh, fresh independent actual analysis/record-analysis and normal WP25 reentry precede separate bounded source authority; this draft executes none. Any changed source requires independent exact-diff/ownership/error review, fresh311/all317 and313-complement bindings, six default roots plus COMPLETE4320 observations/2160pairs/72groups/144 elapsed+RSS criteria, unchanged seven full-source checks and all eight actual revised-method native rows anew. Preserve69 limits/all206 roots,100000/100001 plus99999 causal controls,16384/16385 maps/receipts/policy,45s migration, original detectors/raw traces and three standalone consumers. Default workflow remains hard. Formal WP25 review, WP26 eligibility/two fresh independent public-docs-only cold trials/52+24 preaccept adjudications, normal accept→feature-only merge→postmerge review/retrospective remain mandatory and pending; no waiver, main or release action.


## WP25 B stage/deadline fixture activation amendment — 2026-11-02 (ROOT PENDING DRAFT)

This external draft is not adopted metadata and grants no source permission. Root's `root-B-fixture-design-maintainer-decision.json` accepts the exact targeted-reviewed DESIGN for normal activation. After exact metadata review/root adoption, normal finalizer/planning refresh, fresh independent ACTUAL analysis of this activated scope, record-analysis and normal WP25 reentry, SEPARATE exact-region source permission is still required. Neither file existence, an old READY report nor prior qualification grants that permission. Runtime F1 remains OPEN pending actual implementation/evidence; no source/native/WP approval follows.

Only after that COMPLETE sequence, including separate exact-region source permission, does this dated amendment supersede ONLY the prior B diagnostics-only/later source-activation HOLD for execution of the exact reviewed design in `tasks/WP25-exact-package-qualification/integration-b-stage-fixtures.md`. Root's design acceptance permits normal metadata/analysis activation while source execution stays held. The obsolete shared-recheck/pre-dispatch expected-cancelled proposal is rejected. Prior A/C semantics and the separate C-F1 composite-validator correction remain unchanged. WP18 remains the approved/inactive original owner; root acts as maintainer with an integration-delegate contract readback and independent review, without claiming original-author approval or transferring ownership. Existing T101–T103 carry the work; T100–T103 remain unfinished.

Separate a literal2000ms dispatch-reader deadline/no-effect leaf from prepared/lost-ack stage cancellation. The seeded first exact `[--git-dir=<actual physical private dir>,cat-file,--batch-command]` is request-only, after dispatch and before writer/intent; startup/monitor cannot select it. Require the actual gate, no cancel, peer alive/stdout open until host TERM, strict nonzero host `uncertain_operation`/`inspect_operation` with exact operation tuple, protected-state equality and exact12-field `verified_absent`/null projection, then ordinary successful mutation/found/ref-event/replay controls. Source first-prefix-byte clock plus peer writes/gate/registered-child TERM/parent monotonic brackets suffice; no inaccessible exact timestamp or kernel telemetry requirement.

Prepared/lost-ack use fixed20000ms request, checked stage arrival≤10000ms FROM HOST LAUNCH, exactly one supported framed cancel, peer alive/stdout open until TERM, and owned host launch→Wait settlement strictly<20000ms. Keep original30s stall and fixed30000ms helper/enclosing safety: any safety action is FAIL. Use finite nonqualifying helper/raw-peer fallbacks on host failure. This deliberately decomposes the old conflated2000ms fixture; it proves neither old-test PASS nor a relaxed public cap. Lost-ack's wrapper has no children: parent owns exactly ONE source-issued real-Git update-ref with exact physical dir/argv/ref/old/actual host candidate/message; frozen host environment filtered exactly like projectGit, same cwd/absolute Git/empty stdin/4096 stdout/discarded stderr; one successful Start→one Wait and joined observers on all returned paths. Checked atomic bounded requests/witnesses, no executable/alternate hooks, no replacement writer/OID/repair/retry. Publication can race failure: inspect actual ref, retain failure, never claim rollback/no effect.

The sole prospective B source surface is existing `extension_artifact_acceptance_test.go` necessary imports, raw child/helper/interrupted-recovery and tightly adjacent small fixture controls, as bounded in the new regions/verification JSON. No new Go/module/dependency, parent test name, product host/session/store/adapter/context change or seventh metadata destination. Preserve every original prepared/lost-ack assertion and protected-state control. Leaf evidence including metadata≤16KiB, selected argv≤4KiB; fixed files/counts and overflow FAIL; no environment/payload/signature/private-intent dump.

The actual preserved normal and controlled-delay packet now supports the earlier design-history diagnostic prerequisite: normal original prepared/lost-ack pass, delay reaches host uncertainty before the ORIGINAL missing-lost-ack-marker RED. It does not identify historical Intel timing or qualify the new fixtures. Planning PRIMARY is215d4311664710a420e96991eaf7258442823091; clean source anchor adc88255dbf0c9c8c43ccaaa9561a82b7f4d57f8 preserves all317 inputs in the exact initial8133 map (SHA2566a7b5abbc24b66b4828788c6dd0b33278dcc019b07f87a5685a38f966c2e892c), with four Go deltas vs DD and313 protected DD inputs. Metadata changes none of them; later source changes require fresh311/all317 and313-complement binding plus independent exact-diff/error/ownership review.

Preserve26 WPs/107 subtasks/52 requirements/24 historical reviews, WP25's14 owned paths/13 create_intent entries, frontmatter/dependencies/lanes/groups/authoritative surfaces and feature/optional-extensions target. COORD2a112042 remains24 approved/WP25 in_progress/WP26 planned; preserve original78 user files and all19 stashes. All206 parents/69 limits;240s/30s/10s/250ms/2s/5s public clocks;100000/100001/99999 and16384/16385/maps/receipts/policy;45s migration; full-source120m/seven checks, six defaults and COMPLETE4320 paired observations, eight actual native composite rows, C's fixed heavy4/remaining202 serial groups at120m/invocation/300m/job remain hard. Fresh actual analysis must assess activated B scope; no old READY or qualification/source reuse. Formal WP25 review, WP26 eligibility/two fresh independent public-docs-only cold trials,52+24 preaccept judgments, normal accept→feature-only merge→postmerge review/retrospective remain pending. Historical FAIL/incomplete stays unchanged; no lifecycle/remote/main/release action is performed.
