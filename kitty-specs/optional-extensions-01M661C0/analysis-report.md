---
schema_version: 1
artifact_type: spec-kitty.analysis-report
command: /spec-kitty.analyze
mission_slug: optional-extensions-01M661C0
mission_id: 01M661C0T0E4DS0AXBCXG7WCAX
generated_at: '2026-11-02T02:55:43.278315+00:00'
analyzer_agent: codex
input_artifacts:
  spec.md:
    path: kitty-specs/optional-extensions-01M661C0/spec.md
    sha256: de1d9ef9995b6a84cabafdfa3c15371badb7ac6721fe389dbc20c1933c7c266f
  plan.md:
    path: kitty-specs/optional-extensions-01M661C0/plan.md
    sha256: 3bb72bbe3317867f56a9a082561212cb6649397e08776266988f1f539c28833d
  tasks.md:
    path: kitty-specs/optional-extensions-01M661C0/tasks.md
    sha256: 87f0623c0d3134aef627b04d5dad6860d6fb241d05512d0ff2d8adc2410b83c4
  charter:
    path: .kittify/charter/charter.yaml
    sha256: 404c65fe9e64919858d009c763644aefbfd369884d573d1986e3db7fa1f14bc9
verdict: ready
issue_counts:
  high: 0
  medium: 0
  low: 0
  critical: 0
  info: 0
findings: []
---

# Fresh actual analysis — optional-extensions-01M661C0

**READY for design, with zero unresolved design findings. Runtime qualification and mission acceptance remain blocked.** This independently analyzes current PRIMARY planning at `7b5b9f279919560a07b0850dc3cb0628ca1158a8`, after exact six-file metadata adoption `39de8e4ca8b6b9613ab0408bbc281130c8dad818` and normal finalizer refresh. It supplies INPUT for normal record-analysis; it neither performs that command nor authorizes implementation/reentry, approves WP25, qualifies a source/binary, accepts/merges the mission or publishes to main/release.

## Authority and method

I read repository AGENTS.md and the isolated-agent workflow in spec-kitty-implement-review. I applied the resolver-backed spk-doctrine-profile-load skill: the installed CLI resolves `analyst-annie` (builtin Analyst Annie, analyst), with analysis-mode and precision-pass defaults. Its initialization declaration is empty; no declaration, model, Op or invocation identity is invented. I applied formal acceptance, edge-case coverage and source traceability, while avoiding architecture/framework/implementation decisions or field-mapping work. The resolved handoffs are implementer-ivan/python-pedro/java-jenny and architect-alphonso for structural decisions. `charter context --action analyze --mission-type software-dev --no-mark-loaded --json` supplied locality, documented decisions, specification fidelity, meaningful public-boundary tests, immutable evidence/history, hostile-input bounds and no implicit authority. Exact commands/stdout/stderr are retained beside this report; both calls exit0 and their retained stderr is empty. No standalone dispatch/lifecycle was opened for this mission-scoped read-only assignment.

This is a fresh analysis of the CURRENT specification, all52 requirements, plan and tasks, every107 detailed task guidance/observable acceptance body, every26 frontmatter/dependency/ownership/verification surface, four normative contracts and their complete control matrices, model, research, intake, quickstart, coverage/checklists, lane/meta/acceptance records and the adopted native-correction addenda. All59 literal JSON blocks in conformance-fixtures were parsed and hash-bound; their semantic fields and expected outcomes were read back. This is not a new cryptographic verification or runtime conformance claim. The earlier fresh-analysis directory was consulted only for report/API format; no former READY verdict or requirement judgment is reused as evidence. New requirement-to-task judgments appear below and in the machine coverage.

PRIMARY is the specification/design authority. COORD's `kitty-specs/optional-extensions-01M661C0` is the canonical lifecycle/review authority and has no spec.md. Pure installed `get_all_wp_lanes` and `event_sourced_review_result` return24 approved, WP25 in_progress, WP26 planned. All24 COMPLETE reviewResult strings equal root baseline. Primary frontmatter/narrative phase labels are projections/preparation history, not status authority. The retained ROOT PENDING/draft wording preserves authored history: exact adoption, finalizer and root-authorization records establish what root has since done, without granting source authority by implication.

## Structural and ownership consistency

All52 identities (24FR,8NFR,10C,10SC),26 WPs,107 unique task definitions/index rows and26 lane assignments remain present. Each prompt's entire frontmatter equals the root baseline; authoritative surfaces are owned, create_intent is a subset of owned_files and authored owned paths do not overlap. Dependencies exist, are acyclic and precede their dependent lane's parallel group; every WP descends from WP01. WP02 alone is the planning-artifact lane. WP25 still depends on approved WP24 and has14 owned paths/13 create_intent entries; WP26 depends on WP25. No task/WP/frontmatter/owner/lane/group expansion is hidden in the appendix.

The actual detailed task bodies require a current live CLI/host/Git/process consumer, or explicitly limit a pure input-validation slice to its narrow inert claim. Some earlier objective shorthand says that broader proof arrives in a later WP; the detailed guidance explicitly requires immediate registered slices and later end-to-end proof, so it creates no permission for dormant producer APIs. The following readback includes every package; exact owned/create lists and107 complete acceptance bodies are in `wps.json`, `wp-readback.json`, `tasks.json` and `coverage.json`.

| WP | Dependencies / lane | Task IDs | Owned/create | Authoritative surface | Concrete acceptance boundary |
|---|---|---|---|---|---|
| WP01 | none / lane-a | T001, T002, T003, T004, T005, T006 | 8/7 | `replication_borrow_test.go` | Live native CLI plus original historical inputs; detector red control; healthy and corruption inventory controls; raw baseline metrics. |
| WP02 | WP01 / lane-planning | T007, T008, T009, T010, T011 | 4/1 | `kitty-specs/optional-extensions-01M661C0/contracts/artifact-v1.md` | Independent read-through can derive complete wire bytes and send handwritten valid/invalid requests without source-private assumptions. |
| WP03 | WP02 / lane-b | T012, T013, T014, T015 | 6/6 | `internal/artifact/envelope.go` | Frozen independent byte vectors and signature tamper controls, later reused through live store and public CLI. |
| WP04 | WP03 / lane-c | T016, T017, T018, T019 | 5/5 | `internal/artifact/schema.go` | Pure exported validation contracts with hostile declarative schemas, followed by WP11/WP22 live unrelated-model proof. |
| WP05 | WP03, WP04 / lane-d | T020, T021, T022, T023 | 6/6 | `artifact_store.go` | Real Git object/ref inspection plus tamper/CAS/contribution refusal controls; public CLI caller proof arrives in WP11. |
| WP06 | WP05 / lane-e | T024, T025, T026, T027 | 8/8 | `artifact_catalog.go` | Two actual actor streams, source-order permutations, stale/all-head negative controls and exact reconstructed state. |
| WP07 | WP06 / lane-f | T028, T029, T030, T031 | 4/4 | `artifact_operation.go` | Real store fault stages plus actual restart and immutable replay controls; later WP11/WP18 call the same recovery seam. |
| WP08 | WP06 / lane-g | T032, T033, T034, T035 | 4/4 | `artifact_closure.go` | Actual signed Git dependency sets, removed-schema/corrupt-object controls and private-marker export audit. |
| WP09 | WP08 / lane-h | T036, T037, T038, T039 | 4/1 | `artifact_replication.go` | Two physical clones and a real bare remote, quarantine tamper/missing-dependency controls and original native refusal suite. |
| WP10 | WP08 / lane-i | T040, T041, T042, T043 | 4/4 | `artifact_exchange.go` | Real regular files, independently initialized destination, bounded-consumption cases and no-effects snapshot checks. |
| WP11 | WP07, WP09, WP10 / lane-j | T044, T045, T046, T047 | 3/3 | `artifact_commands_test.go` | Live built hn CLI exercises model/schema/store/recovery/transport with negative controls; no registry or specialized plugin required. |
| WP12 | WP11 / lane-k | T048, T049, T050, T051 | 2/2 | `artifact_search.go` | Live plugin-free search, explicit over-budget patterns, stale selection and process-marker red controls. |
| WP13 | WP02 / lane-l | T052, T053, T054, T055 | 6/6 | `internal/extension/manifest.go` | Live extension inspect/register plus native commands, missing/malformed configuration and deliberate process/read detectors. |
| WP14 | WP13 / lane-m | T056, T057, T058, T059 | 3/3 | `internal/extension/grants.go` | Canonical repository alias cases, local lifecycle CLI, raw authorization vectors and prohibited-effect red controls. |
| WP15 | WP13 / lane-n | T060, T061, T062, T063 | 6/6 | `internal/extension/protocol.go` | Independent handwritten byte streams, allocation-bound inputs and later real process stdio conformance. |
| WP16 | WP14, WP15 / lane-o | T064, T065, T066, T067 | 7/6 | `internal/extension/session.go` | Actual hn host and external processes, observed PID exit, competing native request, timeout/oversize/secret controls. |
| WP17 | WP16, WP11 / lane-p | T068, T069, T070, T071 | 6/4 | `extension_native_adapter.go` | Raw external stdio to live hn plus real native actors/contribution policy and forbidden-effect controls per family. |
| WP18 | WP16, WP11, WP12 / lane-q | T072, T073, T074, T075 | 3/3 | `extension_artifact_adapter.go` | Live raw stdio custom model round trips, policy/grant negatives, full-state bounds and exact operation replay. |
| WP19 | WP17, WP18 / lane-r | T076, T077, T078, T079 | 2/2 | `extension_changes.go` | Raw external observer, live native mutations, policy visibility transitions, stale/expired cuts and invalid-history controls. |
| WP20 | WP19 / lane-s | T080, T081, T082, T083 | 5/5 | `examples/extensions/fake-tracker/main.go` | Independent executable round trip with lost reply, process restart, external journal and raw grant/echo/conflict controls. |
| WP21 | WP19 / lane-t | T084, T085, T086, T087 | 4/4 | `examples/extensions/charter-model/main.go` | External generic publisher plus real independent clone and plugin-free inspection; exact old selection and inert-content controls. |
| WP22 | WP19 / lane-u | T088, T089, T090, T091 | 5/5 | `examples/extensions/inventory-model/main.go` | Unrelated external executable, both real actors and clone portability, same generic caller/negative cases. |
| WP23 | WP20, WP21, WP22 / lane-v | T092, T093, T094, T095 | 5/3 | `docs/extensions.md` | Copyable actual-binary examples and requirement/capability/error inventory compared with observed contracts. |
| WP24 | WP23 / lane-w | T096, T097, T098, T099 | 9/9 | `extension_default_acceptance_test.go` | Original live native journeys, all seven configuration rows, historical byte equality and paired raw metrics with red controls. |
| WP25 | WP24 / lane-x | T100, T101, T102, T103 | 14/13 | `extension_package_acceptance_test.go` | Exact distributed-candidate binary execution and fixed-source full suite; platform rows cannot be inferred from cross compilation. |
| WP26 | WP25 / lane-y | T104, T105, T106, T107 | 4/4 | `docs/qualification/optional-extensions/independent/boundary-review.md` | Independent reviewer and actual native-user/author observations tied to exact package; no invented adopter evidence. |

## Exact amendment and four prospective source exceptions

All six current raw file hashes equal both adopted-planning-files.json and the independently reviewed proposal index. The index/patch hashes are respectively `b744c8d404e1ea4545eee366ff7cb93090cbf7bc6eeac2a41818a9b93cd1d2c3` and `d5eb35ef2ac6c8d00e1a1f5f93a656030067edb34fb2e09e4b6aa79fb1c40ea0`. The three old files retain their entire prior prefixes; the three native-correction sidecars are creates. All12 declared file/region/import bindings match actual frozen29 bytes, including test append-at-EOF frontiers. Source commit remains29ffc/treeccf0, not the metadata adoption commit. Validation and refresh each exit0; refresh reports zero WP modifications and normal commit7b5. Its retained stderr reports sync-store locked, routing failure and an event not durably queued: no remote publication, durable sync or auth-repair claim follows.

Root's authorization record explicitly confirms original WP05/WP16 inactive and no concurrent authorized source writer. Their canonical approvals and original ownership remain intact. WP25 T101–T103 carry only these bounded integration exceptions; they are not whole-file transfers. No source implementation permission is granted by this report.

| Path / original ownership | Precise prospective exception | Required preservation |
|---|---|---|
| `internal/extension/session.go` / WP16 T064–T067 | Only line498 ticker `RevocationInterval` to `RevocationInterval / 2` inside the existing active optional-session monitor | Public250ms observation, four sessions,2s TERM grace,5s exit, owner.check/Recheck order, per-request authority, cancellation/shutdown and owned close/KILL/Wait unchanged. No background monitor without active optional host. |
| `internal/extension/session_test.go` / WP16 | Append focused cadence/fresh-check/owned-child controls after original line575 and only necessary imports | Every original assertion and statement survives. Actual phase-controlled old-source RED and corrected≤250ms plus one/four-session Git/hash I/O measurements are required; nominal125ms is not success. |
| `artifact_read.go` / WP05 T021/T023 | Private admission/body factoring only in292–317 and324–460; remove ONLY ordinary locally-discovered initial witness | Exactly two private compositions. Admission copies before sorting and preserves the captured original discovery slice. Full body retains final witness, every verifier/budget/error/cleanup, final complete source comparison and Cut. Entire766–985 declared7233-byte frozen region remains exact29. |
| `artifact_read_test.go` / already WP25-owned | Append focused ordinary/explicit-route/precedence/cancellation controls after original line5270 and only necessary imports | Preserve all inherited assertions and2MiB E2BIG expression. No new registry root/test file, framework, workflow change, fixture deadline/marker adjustment or third production correction. |

B's two compositions are explicit supplied sources→unchanged admission→original initial witness→shared verified body including final witness, and actual local discovery→same admission→same verified body→fresh complete selection comparison→Cut. Exactly four external explicit call sites remain strict: artifact_exchange.go:679/:1140 and artifact_replication.go:329/:442; their signatures, arguments, both rounds, no-fetch/isolation/fallback/error/child ownership and replication's separate recheck remain unchanged. The actual caller inventory also shows ordinary locally-discovered reads inside mutation preparation, operation/query/search, host catalog/change reads, export/import current-state reads and replication. Those DO take B; “all exchange/replication unaffected” would be false. Downstream pre-effect/CAS/recheck boundaries and both import-union fixes stay exact.

No skip flag, provenance enum/string/token, caller claim, callback, environment knob, cache or third bypass route is permitted. The raw count preflight, object-type/size/hash, signatures, schemas, actor/stream/ancestry, aggregate limits, reader finish/defer, all executed worker errors and final witnesses remain binding. No broader history or fetch is introduced. The frozen worker/pair region preserves original64 primary args/caps,65 sentinel fallback, at most2 producers INCLUDING fallback, coordinator parse/error order and all-return cancel/join/release. Saving one100000-ref successful round nominally avoids1563 primary starts while retaining1563 final starts; this is a design arithmetic observation, not measured speed or RSS.

The published artifact contract requires verified selected histories and source snapshots around loading, not a third specific observation, atomic-instant Git snapshots or universal ABA exclusion. The explicit adopted supersession is needed because prior RW01C alone preserved both rounds. The reviewed nine-row matrix is compatible with those requirements:

| Case | Fresh consistency judgment |
|---|---|
| Admission syntax/OID/duplicate/repository/count failure | Same original admission checks/order/diagnosis; no exception. |
| Ref changes transiently then returns before retained final checks | Ordinary route may succeed after full verification and matching final observations; accepted captured-selection semantics, no universal ABA equivalence. |
| Omitted initial command/fallback alone transiently fails or reaches sentinel | Ordinary route may succeed when all actually executed verification/final observations succeed; never suppress an executed failure. |
| Ref movement plus missing/wrong-kind/corrupt captured object | Authentic body verification_failure may precede former initial concurrent_update; preserve actual structured tuple and no false absence. |
| Ref movement plus raw count/object/aggregate overflow | Original resource_limit may precede former initial concurrent_update; exact real count/message and later verifier remain. |
| Ref movement plus signature/actor/schema/lineage/dependency invalidity | Existing integrity/invalid_history/unsupported_schema/incomplete_dependency/actor_stream_conflict may win at its real body position. |
| Persistent final addition/removal/OID change with valid captured history | Retained final witness or full comparison MUST refuse concurrent_update, preserving the stage-specific message. |
| Cancellation at retained boundaries | Original checks and child joins remain; finishing before later expiry is legitimate, no post-parse or cleanup error override. |
| Any explicit supplied-source mixed fault | Both old rounds and their exact original diagnosis/precedence remain. |

Representative source-causal ordinary call ledgers, omitted-round-only transient faults, ref-change plus count/integrity tuples, persistent final-change refusals, unchanged outputs/Cut and strict explicit-source positives/negatives are required in actual implementation/source review. Neither general diagnostic equivalence nor complete ABA detection is claimed. A agrees with the250ms public max notice but remains an OS/I/O scheduling candidate; if unchanged fresh work cannot meet the bound, reject/redesign separately instead of adding tolerance.

All current317 inputs match29. Both `artifact_exchange.go` and `artifact_exchange_commands_test.go` are separately checked exact4357, preserving import-union corrections. Current artifact_read_test.go:4823 retains child-only `strings.Repeat("x", 2*1024*1024)`. After any implementation, at most these four existing inputs may differ and the other313/317 must remain mode/byte exact29, with the same keyset/no new dependency. That future complement is a required verification, not a future attestation already passed here.

## Fresh52-row requirements and acceptance coverage

Every row below is a current design coverage judgment, not an observed PASS. `coverage.md` supplies absolute task/line links and every107 inverse assignment; `coverage.json` preserves exact requirement statements, actual task bodies and ownership. All52 requirements and all107 tasks have meaningful trace links; no orphan, omitted criterion or unowned task was found.

| Requirement | Actual task trace | Fresh judgment |
|---|---|---|
| FR-001 | T003, T004, T006, T051, T054, T065, T096, T097, T099, T105 / WP01, WP12, WP13, WP16, WP24, WP26 | Ordinary commands retain their original contracts and guide; exact historical/native journeys and real no-loader detectors must pass before later consumer evidence can count. |
| FR-002 | T004, T015, T019, T052, T054, T055, T063, T091, T097 / WP01, WP03, WP04, WP13, WP15, WP22, WP24 | The envelope/schema/frame inspection slices explicitly limit their claims to inert input validation. Disabled manifest inspection and static dispatch cannot start, probe, initialize or fetch. |
| FR-003 | T053, T054, T055, T058, T064, T066 / WP13, WP14, WP16 | Registration instance and enablement epoch changes revoke sessions; removal uses finite owned cleanup and leaves public histories and recovery facts intact. |
| FR-004 | T009, T055, T057, T075, T092, T104 / WP02, WP13, WP14, WP18, WP23, WP26 | The fourteen-method inventory distinguishes declaration, actual grants, enablement, compatibility and observed availability; unsupported privilege never becomes availability by inference. |
| FR-005 | T009, T056, T057, T059, T061, T064, T069, T073, T104 / WP02, WP14, WP15, WP16, WP17, WP18, WP26 | Physical repository and host-selected registration bind every raw request; exact family/type/action grants and actor checks precede signer, intent and public effects. |
| FR-006 | T068, T069, T070, T071 / WP17 | Six native data families have actual raw-stdio read/write/operation proof through existing native writers. Definitions stay inert and omitted command families stay explicitly unsupported. |
| FR-007 | T007, T009, T020, T028, T029, T030, T031, T045, T062, T070, T074, T081, T082, T093 / WP02, WP05, WP07, WP11, WP15, WP17, WP18, WP20, WP23 | The actual actor/protocol/key and immutable semantic request survive acknowledgement loss. Exact lookup precedes current-head checks; changed content/schema/parents/actor cannot manufacture a new replay. |
| FR-008 | T010, T076, T077, T078, T079, T082, T083 / WP02, WP19, WP20 | Complete accepted visible captures, bounded paging and atomic replacement reconciliation handle hide/restore and invalidation. Checkpoint advancement waits for complete durable reconciliation. |
| FR-009 | T007, T008, T012, T013, T016, T044, T084, T088 / WP02, WP03, WP04, WP11, WP21, WP22 | Exact signed namespaced declarations support document/list/ticket without changing native kinds; declaration naming is descriptive and cannot shadow native commands or claim global ownership. |
| FR-010 | T008, T012, T014, T017, T018, T019, T024, T044, T072, T088 / WP02, WP03, WP04, WP06, WP11, WP18, WP22 | Exact UTF-8 text and canonical structured values remain distinct; finite schema checks label only generic validity, leaving consumer meaning and activation outside the core. |
| FR-011 | T007, T013, T020, T023, T024, T025, T027, T047, T073, T086, T089 / WP02, WP03, WP05, WP06, WP11, WP18, WP21, WP22 | Local current-head preconditions, imported immutable forks and actor-stream divergence are separate. Multi-head resolution is explicit and stale after a third head; no timestamp winner exists. |
| FR-012 | T008, T018, T026, T032, T044, T085, T089 / WP02, WP04, WP06, WP08, WP11, WP21, WP22 | Stable member IDs retain explicit order. Exact and logical links have different closure rules, with visited sets, missing-target and conflict diagnostics rather than implicit target choice. |
| FR-013 | T044, T046, T048, T049, T050, T051, T072, T090 / WP11, WP12, WP18, WP22 | Plugin-free reads/search evaluate the complete verified selection before bounded output. Regex is explicit and bounded; stale paging and oversized full state cannot silently truncate. |
| FR-014 | T021, T032, T033, T034, T035, T036, T037, T038, T039, T040, T041, T042, T043, T046, T087, T090, T094, T102 / WP05, WP08, WP09, WP10, WP11, WP21, WP22, WP23, WP25 | Both real-Git and file exchange validate selected public closure before atomic expected-old promotion. Clone reconstruction excludes private state; ordinary local-reader B and strict explicit-source union verification stay distinct. |
| FR-015 | T014, T017, T021, T024, T026, T033, T034, T042, T046, T048, T072, T094, T096 / WP03, WP04, WP05, WP06, WP08, WP10, WP11, WP12, WP18, WP23, WP24 | Opaque public preservation is separately bounded and never implies accepted semantics. Missing/unsupported/invalid custom histories are honest diagnostics and cannot poison unrelated native histories. |
| FR-016 | T016, T019, T027, T047, T086, T089 / WP04, WP06, WP11, WP21, WP22 | New schema publication and explicit conversion append signed revisions; exact old schema and charter selections retain their meaning until an explicit new selection. |
| FR-017 | T002, T022, T023, T037, T042, T057, T065, T068, T069, T071, T073, T076, T079, T096, T104 / WP01, WP05, WP09, WP10, WP14, WP16, WP17, WP18, WP19, WP24, WP26 | Native admission/moderation/retention and per-request shared locks remain authoritative. Policy-selected custom writes refuse before effects; no generic adapter grants governance or installs policy. |
| FR-018 | T007, T010, T013, T020, T028, T035, T040, T043, T068, T074, T080, T081 / WP02, WP03, WP05, WP07, WP08, WP10, WP17, WP18, WP20 | Public attribution is the actual native signer; connector associations remain untrusted descriptive fields. Public output and closure exclude private keys, credentials and recovery journals. |
| FR-019 | T009, T060, T062, T063, T064, T065, T066, T067, T071, T075, T101, T102 / WP02, WP15, WP16, WP17, WP18, WP25 | Framing, startup, request, session, stderr and cleanup bounds have live external-process controls. A reserves headroom but still requires actual notice at or below250ms and unchanged2s/5s cleanup. |
| FR-020 | T009, T010, T011, T014, T022, T025, T029, T030, T041, T045, T050, T055, T056, T060, T061, T063, T066, T070, T074, T076, T078, T093 / WP02, WP03, WP05, WP06, WP07, WP10, WP11, WP12, WP13, WP14, WP15, WP16, WP17, WP18, WP19, WP23 | Closed origin/code/recovery/operation errors distinguish failed verification from absence and dispatched uncertainty from pre-effect refusal. The B matrix explicitly limits ordinary-route precedence changes. |
| FR-021 | T080, T081, T082, T083 / WP20 | The standalone fake tracker owns external intent and reconciliation; real restart/lost-ack/duplicate/stale/disconnect cases assert no duplicate facts or echo and preserve uncertainty. |
| FR-022 | T084, T085, T086, T087 / WP21 | The external charter example stores generic doctrine-shaped documents, pack links and exact selection. It promises no upstream schema conformance, resolver, composition or execution. |
| FR-023 | T088, T089, T090, T091 / WP22 | An unrelated three-shape inventory model uses the same actual core/interface and clones without a domain switch, proving that doctrine naming is unnecessary. |
| FR-024 | T010, T011, T039, T092, T093, T094, T095, T106, T107 / WP02, WP09, WP23, WP26 | Versioned guides expose supported/unsupported capability, recovery, trust and native-only boundaries. An actual independent author trial supplies missing friction evidence before final reconciliation. |
| NFR-001 | T003, T004, T054, T065, T096, T097, T105 / WP01, WP13, WP16, WP24, WP26 | Real registry/declaration/private paths have read/start/provider/write controls including deliberate violations. No optional session means no additional cadence monitor or native setup. |
| NFR-002 | T005, T098, T099 / WP01, WP24 | The complete paired30 release measurements retain all samples and exact source/host/corpus. Current4320 observations with17 RSS failures remain blocking; diagnostics and A/B do not waive or fix the gate by assertion. |
| NFR-003 | T008, T014, T017, T018, T021, T026, T034, T041, T049, T050, T052, T060, T063, T066, T072, T075, T077, T101, T103 / WP02, WP03, WP04, WP05, WP06, WP08, WP10, WP12, WP13, WP15, WP16, WP18, WP19, WP25 | Every published finite byte/count/depth/time boundary has equality and violating consumption cases. Valid public closure remains necessary beyond isolated counter equality; B retains authentic raw count and full verification. |
| NFR-004 | T062, T064, T065, T066, T067, T079, T096, T101, T103 / WP15, WP16, WP19, WP24, WP25 | Fault children and concurrent native/second-module cases prove finite owned cleanup and honest in-flight outcomes. Faster revocation checks retain every live authority and request check. |
| NFR-005 | T001, T002, T003, T006, T015, T023, T039, T096, T099 / WP01, WP03, WP05, WP09, WP24 | Exact imported foundation, native refusal assertions and old signed bytes/IDs/refs bind baseline and final candidate independently; fixture repair cannot rewrite the production contract. |
| NFR-006 | T028, T031, T035, T040, T043, T053, T059, T067, T074, T075, T081, T095, T097, T103, T104 / WP07, WP08, WP10, WP13, WP14, WP16, WP18, WP20, WP23, WP24, WP25, WP26 | Key/credential/journal sentinels and pre/post state prove absence of private data from public capability, diagnostic, log and export surfaces, including opaque selectors. |
| NFR-007 | T005, T100, T102, T107 / WP01, WP25, WP26 | All eight Linux/macOS AMD64/ARM64 standard/combined-native rows require supplied exact candidate execution. Cross-builds and earlier-source passes cannot fill a missing row. |
| NFR-008 | T035, T038, T043, T083, T087, T090, T102, T103 / WP08, WP09, WP10, WP20, WP21, WP22, WP25 | Both consumer models reconstruct complete public IDs/types/order/links on independently initialized clones without module registration, source identity files, credentials or journals. |
| C-001 | T001, T002, T006, T099, T107 / WP01, WP24, WP26 | The beta foundation is an explicit reviewed-source prerequisite. This mission neither accepts beta/pilot/issue23 nor authorizes main or release publication. |
| C-002 | T001, T005, T064, T067, T095, T100, T103, T105 / WP01, WP16, WP23, WP25, WP26 | The native package retains Go-built standalone execution and Git; optional supplied Go child assets introduce no end-user Go/Python/Java/MCP/provider setup. |
| C-003 | T004, T017, T052, T053, T054, T058, T064, T085, T091 / WP01, WP04, WP13, WP14, WP16, WP21, WP22 | Declaration, registration, local enablement, authority selection and explicit run remain distinct; cloned or searched signed text stays inert. |
| C-004 | T084, T085, T086, T087, T088, T091, T095, T106 / WP21, WP22, WP23, WP26 | Domain interpretation belongs to external examples. The charter feasibility demonstration cannot embed or claim a Spec Kitty resolver, activation engine, runtime or service. |
| C-005 | T080, T081, T082, T083, T093, T106 / WP20, WP23, WP26 | Fake local tracker evidence suffices; enterprise API credentials, scheduling and reconciliation remain connector responsibilities. |
| C-006 | T003, T007, T013, T015, T020, T036, T039, T094 / WP01, WP02, WP03, WP05, WP09, WP23 | Additive hn-artifact/1 refs and kinds preserve native hn/0 and hn-memory/0 bytes and rejection semantics with golden and unknown-kind controls. |
| C-007 | T064, T095, T104 / WP16, WP23, WP26 | The public trust description says host grants mediate requests; same-user filesystem powers remain explicit and no hostile-code sandbox or multitenant isolation is claimed. |
| C-008 | T009, T022, T057, T061, T069, T071, T073, T075, T092, T104 / WP02, WP05, WP14, WP15, WP17, WP18, WP23, WP26 | Data grants cannot imply resolve/raw inspection or governed review/accept/merge, policy, identity, memory mutation, runner or transport-apply capabilities. |
| C-009 | T009, T060, T064, T080, T095, T100 / WP02, WP15, WP16, WP20, WP23, WP25 | One explicit registered executable and private stdio transport stays within scope; marketplace, scripting engine, embedded library transport and new GUI are absent. |
| C-010 | T003, T019, T049, T051, T087, T091, T095 / WP01, WP04, WP12, WP21, WP22, WP23 | Native memory wire/search semantics stay unchanged; generic stored guidance and matches cannot select policy or prompt authority. |
| SC-001 | T003, T006, T096, T097, T099, T105 / WP01, WP24, WP26 | Complete original native journeys across failure configurations and a true new-user guide trial establish default usability; any regression blocks mission acceptance. |
| SC-002 | T004, T005, T097, T098 / WP01, WP24 | Actual forbidden-effect detectors plus complete paired metrics jointly prove default isolation and overhead;149 controls alone cannot overcome17 failed RSS criteria. |
| SC-003 | T080, T081, T082, T083 / WP20 | A separate tracker process must complete restart/lost reply/duplicate/stale round trips with exact native identities and no echo; helper success is insufficient. |
| SC-004 | T056, T057, T059, T069, T071, T073, T075, T104 / WP14, WP17, WP18, WP26 | Handwritten denied/wrong-repository/read-only requests assert exact errors and no intent/signer/ref effects; valid controls prove detectors can see those effects. |
| SC-005 | T016, T018, T025, T027, T044, T047, T072, T084, T088, T089, T090 / WP04, WP06, WP11, WP18, WP21, WP22 | Both reference models exercise document/list/ticket create/revise/conflict/convert/search/export using the same generic public entry points and exact fields/text/order. |
| SC-006 | T032, T035, T038, T043, T087, T090, T102, T103 / WP08, WP09, WP10, WP21, WP22, WP25 | Second-clone comparisons include full history IDs, schemas, ordered collections and relations with the specialized process and private state absent. |
| SC-007 | T084, T085, T086, T087, T091 / WP21, WP22 | The charter-shaped pack and exact selected revisions survive evolution and plugin removal, while executable-looking directive content activates nothing. |
| SC-008 | T034, T041, T063, T066, T067, T075, T077, T079, T101, T103 / WP08, WP10, WP15, WP16, WP18, WP19, WP25 | Every limit and lifecycle failure requires genuine refusal/cleanup/public-state proof at the live boundary, not silent truncation, timeouts counted as success or fabricated negative controls. |
| SC-009 | T005, T055, T075, T092, T100, T102, T107 / WP01, WP13, WP18, WP23, WP25, WP26 | Capability status and exact eight-row package evidence expose unknown/missing/unsupported support. Unpublished current native29 results remain pending. |
| SC-010 | T095, T105, T106, T107 / WP23, WP26 | Two distinct true independent cold trials use exact packages and public guides; CI, replay and hypothetical observations cannot substitute for user/author friction. |

## Remaining proof and honest failure accounting

Current29 six ordinary default roots have149 named PASS and zero FAIL/SKIP, but the COMPLETE4320 observations/2160pairs/72groups/144 metric criteria yield127 PASS/17 RSS FAIL; all72 elapsed criteria pass. Therefore default performance qualification FAILS. Current29 full7 is NOT STARTED and native29 is NOT PUBLISHED. The amendment cannot erase these facts and A/B make no claim to fix RSS. The complete retained failure recomputation, original raw-sample bindings and source-after record remain authoritative for that run.

The v2 RSS attribution record is diagnostic only:84 complete observations,36 calibration PASS and1088 reaped owned task generations; root56942 terminal0/all-owned-child completion and unchanged source317/binary/corpus snapshots are root's retained reports. It found an executable-location association with final clean-file residency; it did not identify a removable product/kernel/cache mechanism or a collector bug. All84 GNU Time values matched native wait4 rusage. Low post-exec Git own-mm HWM does NOT exclude waited-task contribution: retained vfork children can carry pre-exec Go footprint into task-lifetime rusage. Final file/anonymous RSS is not historical peak decomposition, and allocation reduction is not RSS-gate proof. No new diagnostic/probe was executed here.

Historical4357 Linux H14 notice250.533675ms remains a real250ms failure despite cleanup. C844 Intel selected-ref31.273s FAIL and28.882s PASS are separate retained observations near the unchanged30s ceiling, never favorable-sample selection or new-source qualification. Pre-stage marker failures remain unresolved; no evidence establishes the intended injection stage or authorizes changing native8s/artifact2000ms fixture budgets. The returned176 child completions are partial, not whole-job coverage; the16.734s final-session leaf during the120m global watchdog does not establish a2h local deadlock. Diagnostic Mac stage work grants no third correction.

Root's additional bounded historical4357 Darwin ARM summary binds run41361822832/job125014152743 to source43574af5 and terminal failure, with two root completions5323.53/5064.93s and only the returned SHA1/SHA256 E2BIG leaf names (each twice). This is the old256KiB fixture, not a current29 result, a full native8 trace audit, or evidence against the frozen2MiB change. Old4357 Intel was last root-observed live at02:41:51UTC; this report makes no later remote-status claim and performs no remote query/cancel/retry/relabel.

A new separately authorized candidate still needs meaningful old/new causal controls, an independent clean exact-diff/source review, fresh actual root311 inherited and all317 bindings, all six unchanged defaults plus COMPLETE4320 paired observations/2160pairs/72groups/144 elapsed+RSS criteria PASS, all7 fixed-source race/static/build checks PASS, all8 actual supplied native packages at original deadlines,100000/100001 and separate99999 causal cases,16384/16385/maps/receipts/policy,45s migration, real raw traces/detectors and all three standalone consumers. Actual Mac E2BIG RED/GREEN and healthy controls remain required; Linux alone cannot qualify Mac. No measurement collector, corpus, source layout/padding, timeout, cap, environment, sample selection or threshold waiver is introduced. Old immutable source/job/alias identities remain untouched.

Formal WP25 review/approval precedes WP26 eligibility. T104 requires independent actual boundary review; T105/T106 require two true independent cold public-docs-only native-user/author trials bound to exact supplied packages/guides, with real friction and dispositions. T107 must adjudicate all52 requirement/constraint/success criteria against actual source-bound evidence. The canonical acceptance matrix remains overall pending with24 FR placeholders, every pass_fail pending and evidence null. These are unfinished bookkeeping, not acceptance proof; precise executable criteria exist in current contracts/tasks, and all24 must receive real judgments before acceptance. Missing trials or failed default/platform/security/authority/history gates block completion.

Noncircular P01–P05 criterion/preaccept gates precede P06 normal accept→feature-only merge→postmerge mission review/retrospective. P06 remains required mission work, not an already-completed prerequisite. No beta/pilot/issue23 acceptance change or main/release action is authorized. Root must normal-record this actual analysis, verify currentness and perform normal WP25 reentry before deciding separate bounded producer permission; this delegate performs none of those mutations.

## Preservation and binding

Before/after guards cover PRIMARY/LANE/COORD HEAD/tree/branch/status, every220 primary and317 lane Go/module file's hash/size/mode/Git blob, all24 complete canonical review strings,26 canonical lanes and all planning files. PRIMARY/LANE stay clean; COORD's one preexisting untracked dossier projection stays unchanged. All18 stash object IDs, including targeted `af7ae748957e5123c98cc22bd9d532e84c8cb5a2` and all17 earlier entries, remain exact. Root's retained preservation record says the78 originals were byte/mode/status and archive-verified before targeted stashing; this delegate does not read archive payloads, restore originals or manipulate stashes. Root retains the restore-after-reentry obligation.

Only this fresh-analysis directory is written. No Go/application/compiler/build/test/probe, native archive/bulk log, remote query, source/index/lifecycle write, current.json, normal recorder/reentry/move/accept/merge or sync repair was run. Hash and parser/readback checks here are static analysis tooling, not application qualification.

Normal recorder fingerprints below come from the INSTALLED `collect_input_artifact_hashes`. tasks.md uses the SDK's normalized status-cell/checkbox digest rather than its raw metadata-adoption hash. The existing normal report is stale for plan/tasks, which is why root's normal recording remains required.

| SDK input | Relative authority path | Current SDK SHA256 |
|---|---|---|
| spec.md | `kitty-specs/optional-extensions-01M661C0/spec.md` | `de1d9ef9995b6a84cabafdfa3c15371badb7ac6721fe389dbc20c1933c7c266f` |
| plan.md | `kitty-specs/optional-extensions-01M661C0/plan.md` | `3bb72bbe3317867f56a9a082561212cb6649397e08776266988f1f539c28833d` |
| tasks.md | `kitty-specs/optional-extensions-01M661C0/tasks.md` | `87f0623c0d3134aef627b04d5dad6860d6fb241d05512d0ff2d8adc2410b83c4` |
| charter | `.kittify/charter/charter.yaml` | `404c65fe9e64919858d009c763644aefbfd369884d573d1986e3db7fa1f14bc9` |

Input manifest SHA256: `60d19d492e7bbbbd4f992ca10dd183bfcf3cd69134e63e7fcb22abca5471e7b2`. Coverage SHA256: `d44bc01b4c6ccc8fcb26f161766a866889bf8b4da0b2c11bfdecc942ecf8bbf0`. Before/after evidence is in `nonmutation.json`; report/carrier and all outputs are bound by `output-manifest.json` and `summary.json`. Root must recheck these bindings before normal recording.
