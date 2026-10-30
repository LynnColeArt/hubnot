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
