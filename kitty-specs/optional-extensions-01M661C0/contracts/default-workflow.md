# Default-workflow acceptance contract

Hard owner requirement: extension support cannot make ordinary Hubnot harder to use.
Every failing default condition below blocks acceptance, even if both plugins work.

## Baseline cases

At the approved integrated beta base, capture input commands, semantic outputs,
signed bytes/IDs/refs, expected refusal causes, environment, source/toolchain and
paired release timing/RSS observations for:

1. Init, issue create/read/revise/comment/conflict and local exchange.
2. Projects/items, discussions/replies and inert workflow-definition operations.
3. Memory record/recall/grep/lifecycle with unchanged policy/trust semantics.
4. Native proposal/review/CI/decision/merge and unsafe-host/default-off refusals.
5. Existing import/export/recovery, private/public-state and native-only journeys.

Use existing baseline fixtures/goldens and meaningful public paths. Do not invent a
new easier path or waive issue23 or another valid regression to make this green.

## Configuration matrix

Run default cases with: no registry; registered disabled module; missing executable;
incompatible module; crashed module; malformed extension-local configuration; and
many unrelated custom records. Native paths start/probe zero module processes,
perform zero extension-registry parsing/state writes and require zero extra user
setup steps. Test instrumentation needs a deliberate negative control proving
forbidden loader/process effects would be detected.

An extension-aware client without the specialized module inspects/searches/exports
custom types/records and preserves native collaboration. Unsupported pre-extension
protocol support is reported honestly; no current signed native chain is polluted.

## Performance acceptance

Use30paired non-race release observations per representative command, same physical
host, Git, corpus and filesystem. Preserve raw observations and measurement method.
Median elapsed delta <= max(10percent,5ms); median peak-RSS delta <= max(10percent,1MiB).
Measure with registry absent and with large unrelated custom histories. A threshold
failure is investigated, not retried away or relaxed silently. These engineering
budgets are not a universal runtime SLA.

## Required evidence and user experience

Bind exact source/tree, input digests, compiler, package/variant/executable hashes,
OS/CPU/Git/filesystem and expected-vs-observed outcomes. Every claimed native target
and standard/native-only variant needs package execution. Missing rows stay pending.

An independent native user follows the unchanged guide without seeing extension
configuration; an independent extension author follows the new guide and reports
friction. CI/source-assisted controls and implementer replays do not replace this
user observation. No default service/provider/runtime dependency is allowed.

## Frozen first gate and final comparison

WP01 is independently approved at event 01M66767D87RQVAEF5NMB31BQK.
Its Go source is eb3f4932c265c0cf1b1f5c19939f212cded40e2c; the accepted
measurement/evidence correction is 1e7932237c7fc41a88530dcd3f5bef77a82a59a3.
Public baseline evidence is under docs/qualification/optional-extensions/baseline/.
All 220 Go/module inputs were bound; all 105 production inputs match the approved
beta f8c68bebbaa3845d1eaaac02e3638f30dc062bbd foundation. This imports no
beta acceptance or issue23 repaired-Mac claim.

The Linux baseline has 30 actual observations for nine representative historical
read commands and two build variants (540 rows), plus separately bound live
mutation/governance/refusal test evidence. Measurements use exact frozen release
binaries/corpus and GNU Time %M after a small native launcher, with monotonic
elapsed time including its wrapper overhead. Python-parent inherited RSS is NOT
an accepted workload measurement. Independent controls prove parent-memory
independence and response to actual child/waited-descendant allocation. All prior
invalid measurements and scripts remain preserved as superseded evidence.
WP24/WP25 reuse this mechanism for paired comparisons or independently establish
equivalence before comparing; thresholds and sample retention do not change.
A timing/RSS failure is not a reason to drop rows or substitute race timings.

The original pre-host registry trap was instrumentation, not a host layout.
Final no-loader/no-write tests MUST bind the actual:
- user-local os.UserConfigDir()/hubnot/extensions/registry.json;
- COMMON_GIT_DIR/hn/extensions/enabled.json, session-slot/control/capture/intent
  private paths; and
- inert repository .hn/extensions.json declaration.

Instrument reads of both absolute and repository-relative forms of those paths.
A native path must not parse even malformed files at them, start/probe a module,
connect to a provider or write extension state. Deliberate controls must trigger
each detector for its exact cause, including a relative-path registry read,
a real module child, a real provider connection and a private-state write.
The actual module disabled/missing/incompatible/crashed/oversized/EOF/blocked/
cancelled configurations and many unrelated custom records are repeated against
the final implementation; the pre-host first gate does not qualify them.

Default native behavior remains the same even when custom accepted or opaque
histories are malformed/unknown. Generic artifact reads may diagnose those histories
but never consult executable registration. Native recall/grep never activates stored
guidance or extends generic host authority. Ordinary --help/version/init/native
commands retain their existing semantics and dependencies.

## Host-specific conformance gates

Use the exact method/frame examples in host-v1.md and literal signed fixtures in
conformance-fixtures.md. At least one handwritten consumer and raw denied request
must avoid the implementation's convenience client. Required checks include:

1. Full native input/output/refusal/signed-byte/ref comparison through both build
   variants with the unchanged user guide and all final failure configurations.
2. Read-only grant versus raw write/resolve/type.publish/raw.inspect, asserting
   no signer, intent/journal or public-ref effect before refusal.
3. Idle/blocked first host while a second module and ordinary native command work;
   per-request native operation lock and actual actor/policy/visible-view rechecks.
4. Lost acknowledgement and cancellation before/after publication. A definitive
   refusal has no later publication; uncertain results preserve actor/key/request.
5. Hide/restore or authority/retention changes without resource edits; an interrupted
   replacement rescan never commits a partial checkpoint or leaves a removed resource
   falsely visible after a completed replacement.
6. Independent clone reconstruction, plugin removal, generic data access and public
   export with secret-marker files nearby. No private registration, signing key,
   raw stderr, provider credential or connector journal may leak.
7. Each documented at-limit fixture and its first-over/invalid partner, checking
   exact code, failure stage and public/private state rather than only exit status.

Native-user and extension-author trials and exact platform executables remain real
observations, never replaceable by implementer source replay. This contract and all
first-gate evidence remain release-blocking throughout the mission.
