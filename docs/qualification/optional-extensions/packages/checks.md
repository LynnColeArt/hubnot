# Optional-extension package qualification

Qualification is **PENDING**. This report does not publish a release or claim beta,
issue23, main, adopter-trial or mission acceptance.

## Current qualification scope — 2026-11-03

The maintainer's [native CI integration decision](../../../../kitty-specs/optional-extensions-01M661C0/ci-integration-decision.md)
supersedes the historical exhaustive hosted scheduling described later in this
file. Native Hubnot fast/stress/soak requests and bounded hosted package smoke
replace repeated full corpora on every hosted target. Tests and production
limits remain intact. Old exhaustive results retain their original scope;
new smoke results cannot be relabeled as exhaustive passes.

The frozen Mac candidate remains `3dcbb96091c6191bc5d98cde9dbd1d2b98433f95`,
run `41465035433` attempt 1, artifact `13149409710`. Both Linux hosted jobs
and the Apple Silicon hosted job report success; the Intel Mac job was
cancelled and remains incomplete. The overall cancelled run is not a pass.
The matrix records these job observations separately from final row acceptance.

Independent Pink Menace testing of the exact Apple Silicon packages passed the
installation/runtime check, but browser-downloaded quarantined copies failed
Gatekeeper and direct launch. This is a **known distribution-launch limitation**,
accepted by the maintainer for the unsigned Mac beta on 2026-11-03. It does not
block functional qualification or independent review. See
[issue #24](https://github.com/LynnColeArt/hubnot/issues/24#issuecomment-6595409846).
Signing/notarization and the resulting exact packages require native Mac retest.
The maintainer confirmed on 2026-11-03 that no Developer ID setup is available yet.
Signing is a separate distribution follow-up; normal browser-download installation
remains uncertified. Release notes must disclose this accepted beta limitation.
No candidate bytes have been replaced to incorporate the CI changes.

The first post-integration smoke run `41589853578`, at source
`3f4c1a39a4fde59a0c2c04c0f38446de5b08a391`, passed on both Linux targets but
timed out on both Mac targets. All thirteen selected Linux journeys per variant
were independently checked from artifacts; the four Mac variant attempts remain
incomplete. The broad native journey exceeded the three-minute harness budget.
Current hosted scope is therefore install/runtime plus five focused extension
callbacks; broad cross-feature, recovery and consumer journeys execute through
the explicit native `integration` pipeline. This changes scheduling, not the
full registry, public limits or assertions. Historical timeout evidence is kept.

The corrected run
[41591987505](https://github.com/LynnColeArt/hubnot/actions/runs/41591987505),
at `d5aee5d0226c2e86615c1a0910d3cc8b9592cce7`, **passed all eight native smoke
rows** (both variants on Linux/Mac Intel/ARM). Build plus the complete matrix
took 321 seconds from run creation to the last completed job. Retained artifacts
were checked independently: exact source/tree/compiler and package/reference
hashes, all five selected roots, all 201 exclusions, no source fallback, unchanged
source, both install/runtime variants, and the expected missing-reference
negative control. Smoke invocations took 9.71–23.28 seconds each; Mac install/
runtime took 5.46 seconds on ARM and 8.99 seconds on Intel. These are functional
package observations, not notarized browser-download or exhaustive passes.

At that same corrected source, Hubnot's native Linux sandbox published passing
signed `test` and `integration` results in 101161 ms and 116154 ms respectively.
The integration run exercises broad native collaboration, artifact recovery,
six native families and all three reference consumers using explicit source-
assisted executables. Exact signed IDs and smoke evidence hashes are recorded
in `platform-matrix.json`. Qualification and beta acceptance remain pending.

The integrated coordination head `f5ea2531cf0a547a19578662589924d97ca36bdf`
passed the native `test` pipeline in the default Linux Bubblewrap sandbox. Its
signed source-bound result is recorded in `platform-matrix.json`. Execution used
an isolated clone and a verification actor; project trusted-runner roles remain
unchanged. Subsequent evidence-only documentation does not change the tested
action, pipeline, Go inputs or frozen package candidate.

## Earlier evidence and implementation history

The candidate uses Go1.26.4 and the existing Go-only `cmd/hn-release` with internal
version `0.0.0-optional-extensions-qualification.1`. Eight independent packages
cover Linux/macOS, AMD64/ARM64, standard and combined `nogithub,noforge` builds.
Twelve separate target-bound reference executables are qualification assets;
native users require the existing executable and Git.

`TestExtensionPackageQualification` verifies all archive/BUILD/provenance hashes,
source/tree/compiler/target/variant identities and all twelve reference assets
before execution. It pins every supplied-binary seam and installs a leading Go
trap after harness compilation. Its missing-reference and compiler-trap controls
must fail for their exact causes. The registered `portability.yml` caller selects
extension qualification explicitly and calls the relative reusable workflow at
the same immutable caller commit; candidate source is separately bound.

Each actual row must retain host identity (including translation checks), command,
pass/fail/skip events, binary/reference hashes, workflow/candidate/run/attempt/job/
artifact binding, available caller transcripts and raw new bound controls.
Reviewed legacy callers without stdout capture are labelled assertion-backed;
their concrete response/state assertions remain mandatory. Source-assisted signed
fixture construction and raw hostile test peers are labelled separately from
standalone consumer journeys and actual package observations.

The eight transferred fixture repairs resolve the actual `os.UserConfigDir` path,
pin each fixture environment and retain causal actual-registry controls. The
replication detector excludes the source actor's real private key while preserving
the destination's legitimate keyring bytes. They change no production Go bytes.

The authorized bounded record-count correction adds a preflight over captured
immutable tips using metadata-first bounded raw-parent reads, including shallow
and grafted views, followed by unchanged full verification. The intermediate source100000/100001 calls both cancelled and
remain historical. New development controls distinguish count admission/refusal
from subsequent verification cancellation; independent source review, final
source attestation and all eight actual package observations remain pending.
The rejected79c3 implementation traversed commits before the byte guard; its
results remain intermediate. The scoped replacement authorizes each contents
request only after exact metadata and byte checks, then verifies raw Git hashes
before parent traversal. Protocol peers provide supplemental stage evidence;
real Git and public caller cases remain required.
Production changes require a fresh complete4320-observation default/performance
capture under unchanged budgets. Historical WP24 reports remain source-specific.

A08 preparation incorrectly described the prior large-context fixture as1024
parents; it actually used30 siblings. The new direct test constructs1025 genuine
signed sibling heads, observes the over-limit refusal with unchanged protected
state, then selects the1024-head prefix and performs an exact public resolution.
The independent changes1024/1025 revision-identity cases remain BOTH positive.
The original static preparation and every failed development run remain preserved.

All published limits are reconciled against current callers. Dormant declarations,
private implementation/accounting defenses, enclosing limits and dominated caps
retain those narrower classifications. A synthetic equality is never described
as successful semantic admission. No loader, authority, wire, dependency or budget
change is authorized by this qualification.

Actual Darwin syscall instrumentation requires positive controls at the selected
OS registry path and honest process/descendant scope. Missing tracing is an
explicit blocker. A strace skip or a cross-compiled package is no native proof.

Source and package evidence, final row results and unresolved blockers will be
synchronized here before T100–T103 completion and normal independent review.

The native retained-contribution correction uses its own sequential classic
`cat-file --batch-check` and `--batch` workers. All supplied metadata/count/byte
admission and metadata EOF/Wait/reaping precede any body request. Exact repeated
headers, Git hashes, framing and final body EOF/Wait precede the unchanged native
preflight and authority/accounting steps. Native stderr is discarded through an
owned, joined drainer; it has no artifact8192-byte cap. The real9MiB historical
blob controls distinguish native policy from artifact8MiB metadata policy; they
claim neither public32MiB equality nor exchange-wire equality.

Two narrow diagnostic compatibility decisions are explicit. An admitted exact
body-phase `<OID> missing` marker retains the old logical body wrapper with
`missing contribution object` or `retained object unavailable`; its underlying
suffix/type necessarily differs from the old Git128 cause. Metadata missing still
uses its existing byte refusal, and real truncated producer failures retain the
actual ExitError. Indexed non-blob payload/signature roles alone retain the
original bounded semantic `show`, after raw finalization and all prior gates;
normal blobs use same-call offsets. Raw failure never invokes that role fallback.
No claim that every old diagnostic suffix is unchanged follows.

Protocol fault peers reexec the existing compiled Go harness. They require no
Python, runtime compiler, source build or additional installed tool. A five-second
phase witness requires a fresh nonce ACK from the same blocked peer, bound to its
actual PID/PPID/session/phase/stage, alongside the live candidate and joined
observer. A marker or PID presence alone is insufficient. Controlled descriptor
holders have explicit fixture release/exit receipts; OS init owns orphan reaping,
which is separate from the native worker's mandatory Wait. Synthetic faults and
source-assisted object mutation remain distinct from real Git/public outcomes.

The checkpoint fixture sets and reads back distinct mtimes before pruning. Its
second fresh case makes descriptor32 oldest while creation-first0 survives; the
original accounting and memory-eviction blocks remain unchanged. The historical
failure did not retain its actual old mtime/order sequence, so its cause remains
a source-supported hypothesis, not a recovered observation.

Qualification race-harness execution uses `GORACE=atexit_sleep_ms=0` to remove the
race detector's post-exit delay while retaining race detection. Product deadlines,
KILL/cleanup assertions and performance flags/math stay unchanged. Test-peer
launchers forward only the already configured outer value; unset remains unset.
The qualification root records then clears ambient `XDG_CONFIG_HOME` so isolated
legacy fixture HOME paths remain independent. Same-binary shared-config failure
and blank-config success establish that mechanism; the original CI ambient path
was not retained. Explicit OS config pins and causal registry-read controls remain.

The single unchanged tracker default-inert fixture runs at its exact top-level
name in the same compiled harness because its existing temporary-file pattern
contains `t.Name()`. Its child retains trace/binary/reference selectors, the
configured race option and actual named execution/pass/no-skip checks. Ordinary
standalone reference-consumer journeys are separate requirements.

Literal native100000 event/ref controls retain exact signed IDs, terminal-event
readback, first-over refusal, protected-state and recovery assertions. The public
combined graph16384/16385/recovery control is an all-envelope specialization with
zero native dependency facts, genuine retained registrations and authorized
rollovers. Its explicit3600000ms native-CLI policy isolates the numeric ceiling;
it makes no default/host throughput promise. Immediate command-only timings are
recorded separately from older elapsed observation spans that include subsequent
state hashing. All new-source full/default/performance/native gates remain
pending until their actual frozen results replace this pending report.
