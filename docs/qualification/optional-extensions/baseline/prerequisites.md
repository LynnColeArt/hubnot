# Default native baseline prerequisite

WP01 imports approved beta commit `f8c68bebbaa3845d1eaaac02e3638f30dc062bbd`
by normal merge in its assigned execution lane. WP06 approval event
`01M65QYZ60GJ084T2GK3H55G1F` and WP08 approval event
`01M65S51K44AHFRRNHXEM9MMDH` bind this production source. The import is an
explicit approved-source ownership exception. Incomplete beta WP09 commit
`89ce723ca53e1daa5c58105e6010569d1fe73de8` is excluded from ancestry. Beta
acceptance, pilots, issue closure and release publication remain independent.

The exact approved WP07 `beta_scale_test.go` blob
`04f9587e36b571779153a8e783072d85ec22f6f3` is imported separately from
`c2a2c2717dfce6d81460ab836409a921d7552b2b`; its SHA256 is
`60cd60b0321b837adbab9959cf6b3e082ccd02647b738d0eacf5ea5b13b29f33`.
There are 218 Go/module inputs at f8, 219 with this scale test, and 220 after
adding WP01's detector test. All 105 production Go/module inputs remain
byte-identical to f8, including the compatibility generator. The canonical
sorted path/digest JSON digest is
`7e77be2491cf1437b9593f7a74da84f50ba59a4f7d3bc2bd9249d950460c6a46`.
The manifest records exact tested source and tree independently of later
evidence-only commits.

## Issue 23 disposition

The original `TestGitObjectsPresentCorruptObjectFailsClosed` passed locally on
Git 2.43.0 before editing. External issue 23 evidence reports the original
failure on macOS Git 2.50/2.53 at `ebcd023c72fa891c355e9e2ba466541837e51aca`.
`shallow.go` and the original regression file are byte-identical at ebcd and f8;
that does not make the external executable an f8 candidate. The original test
assumed a missing promised lookup always forces inventory. Production only
falls back to physical inventory when Git's batch process fails. A successful
missing result need not examine an unrelated corrupt loose object.

The repair preserves direct corrupt-ID refusal, explicitly requires physical
inventory of corrupt storage to refuse, and adds a healthy physical-inventory
positive control with storage equality and no-fetch trace. Existing promised
absence/no-fetch, uppercase-ID and replacement-ref assertions are retained.
No production change, skip, version condition or allowed-error relaxation is
introduced. Actual macOS execution of the repair remains pending. Independent
WP01 review must assess this correction; this does not close issue 23.

## Integrated seams inspected

`main.run` acquires `contributionCommandLock` around complete public commands.
The lock is omitted when contribution authority is absent; it does not grant a
future host permission to call writers outside that boundary. Direct Git writers
remain outside protocol enforcement. `contributionAdmitted` and
`contributionVisibleEvents` filter signed native histories, while native append
preflight and promotion recheck admission/authority. `ObserveResourceOperation`
and `observeIssueOperation` preserve exact actor/key identity and reject keys
belonging to another family. Resource and issue mutations pin the active actor,
validate immutable request/parents, and refuse changed replay. Existing exchange
and moderation contexts/stage deadlines remain intact. The extension host must
reuse these native boundaries under the complete command lock; WP01 adds no host.

## Reproduction and evidence boundaries

The reusable detector is in `extension_default_baseline_test.go`. Invoke
`go test -count=1 -timeout=3m -run '^TestExtensionDefaultBaseline' .
-hn-baseline-trace=/usr/bin/strace` on a host with that explicit qualification
instrument. It observes actual child process execution, registry opens, provider
connects and extension-state changes. Each detector has a deliberately violating
control that must report its exact cause. Native reads succeed with a malformed
inert registry trap installed. strace is qualification tooling, not an application
or package dependency. Without the explicit flag, the ordinary test suite checks
native success and state writes and reports tracing as unrequested.

`-hn-baseline-registry` parameterizes the provisional repository-relative trap
path. It is not a frozen production registry location. WP02/WP13 must supply real
user-local registration and repository-enablement paths, and WP24 must repeat
against those paths and the final host. Missing/incompatible/crashing/disabled
module and unrelated custom-history matrices cannot be qualified before those
features exist. No such pass is inferred from this pre-host detector.

Historical inputs are the three original released-writer corpora under
`testdata/compatibility`; `historical-hashes.json` binds their exact bytes,
original source/releases, public refs and IDs. The new public test reads all
native families through standard and combined `nogithub,noforge` executables and
compares historical object bytes (including signatures) and refs before/after.
Existing released-writer tests additionally exercise continuation and malicious
future-version refusal without weakening their assertions. Current public CLI
journeys and refusal coverage are enumerated in `native-outcomes.json`.

Measurements use non-race release binaries and 30 retained observations per
native read case and variant on one physical host/corpus/filesystem. Elapsed time
uses a monotonic clock; peak RSS uses Linux wait4 rusage for the actual process
(including waited-for Git descendants), in KiB. Samples alternate standard and
combined native-only variants and retain every observation. Neither race times
nor test timing substitute for these measurements. Baseline/candidate overhead
comparison belongs to WP24/WP25, with median elapsed bound max(10%,5ms) and peak
RSS bound max(10%,1MiB). The public manifest records binaries, compiler, host,
corpus and method. Raw local observations, reproducible corpus, generated keys,
executables and logs are retained outside Git in the WP01 evidence directory.

Only local Linux/amd64 execution is observed here. Other platform targets,
independent native-user/extension-author experience, final host failure matrices
and final-source qualification remain pending. Python architectural coverage is
`no_coverage` for this Go change; it must not be reported as Go test coverage.
The manifest records gate outcomes only after actual completion. Reviewer
approval is a separate mandatory gate for every later WP.
