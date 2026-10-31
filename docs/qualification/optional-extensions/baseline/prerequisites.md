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

`mainObjectsCompleteForBorrowing` in `quarantine.go` independently refuses
borrowing from shallow, partial/promisor, retained-promisor-pack or alternate
stores, and answers false on inspection failure. The corruption fixture repair
does not alter that isolation boundary.

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

Measurements use the same exact non-race release binaries, physical host,
filesystem and warmed keyless `measurement-corpus-3`. The replacement CSV retains
30 observations for each of nine historical native read commands and both variants,
with explicit LF line endings. Samples alternate variant order and none are removed.
Elapsed time uses Python perf_counter_ns around subprocess.run of the native
`/usr/bin/time` wrapper and command, so wrapper launch/exec/wait overhead is included
consistently. Peak RSS is GNU Time `%M` command rusage in KiB after the small native
launcher exec/fork; Python wait4 RSS is not used. On this Linux host it reports the
command/its waited-descendant maxima, not simultaneous summed RSS or whole-machine
memory. The exact native launcher SHA256, literal version output (UNKNOWN), installed
package version and control identities are recorded in the manifest. Qualification
instrumentation adds no application or package dependency.

The new method was validated under Python parent allocations of 0, 64 and 128 MiB:
a small native child measured a median 1384 KiB in every case; a child touching 64 MiB
measured 66792/66792/66920 KiB; a waited descendant touching 64 MiB measured 65924 KiB
in every case. The identical frozen issue-read command measured 12400/12596/12340 KiB,
while each 128 MiB parent actually reached at least 128 MiB high-water. Raw observations,
exact control source/binary identities and diagnostic assertions are external and
hashed in the public manifest. These diagnostic control bounds do not change the
application regression budgets.

WP24/WP25 must use this same wrapper/scope/monotonic bracket for paired comparisons,
or separately validate a comparable mechanism. Median elapsed bound remains
max(10%,5ms), and peak-RSS bound max(10%,1MiB). No overhead pass is claimed before
comparison against final code. Raw local observations, corpus, generated keys,
executables and logs remain outside Git in the WP01 evidence directory.

Only local Linux/amd64 execution is observed here. Other platform targets,
independent native-user/extension-author experience, final host failure matrices
and final-source qualification remain pending. Python architectural coverage is
`no_coverage` for this Go change; it must not be reported as Go test coverage.
The manifest records gate outcomes only after actual completion. Reviewer
approval is a separate mandatory gate for every later WP.

## Observed validation and retained attempts

The original frozen-source Go evidence remains valid and was accepted in review:
formatting, vet, builds, focused corruption/historical/traced controls and the full
uncached race suite passed (1903 test/subtest passes, one pre-existing opt-in skip,
zero failures). All 220 Go/module inputs remain byte-identical to eb3. No Go source,
production contract, binary rebuild or full-suite rerun accompanies this evidence
correction. Current live mutation/exchange/governance/refusal test names and exact
test-source digests are in native-outcomes.json. The nine timing read commands are
separate from those current public journeys. Other platforms/adopters/future host
configuration cases remain pending.

Attempt 1 failed before sampling on a case-sensitive grep expectation. Attempt 2
retains 540 observations and binaries with invalid VCS provenance. Attempt 3 retains
another 540 exact-source observations, but formal review demonstrated that Python
wait4 peak RSS was floored by inherited launcher high-water. Attempt 3 is explicitly
superseded measurement-invalid, not a valid source for RSS budgets. Its complete
CSV, raw samples, binaries, original capture script and logs are retained in
external `superseded-rss-attempt` with an immutable digest map. Original files were
also left intact. The original committed CRLF CSV's 541 whitespace reports and
exit 2 output remain in `reviewer-diff-check.log`. No failure or sample was deleted.

Attempt 4 is the complete replacement native-time dataset, with all 540 rows retained.
The manifest records method, launcher and control identities, medians, attempt
history and artifact hashes. The R2 check covers exact approved f8 through the
corrected authored deliverables, explicitly listing the three owned Go paths and
baseline evidence directory. A committed correction-cut check is recorded in the
manifest; the final delivered-head recheck, exact input digests and raw successful
output are bound in external final-handoff-cycle2.json. A clean-working-tree check
alone does not satisfy this gate, and no whitespace rule is waived.
