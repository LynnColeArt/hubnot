# Runner-efficiency delivery decisions

This is a source delivery for issues [#27](https://github.com/LynnColeArt/hubnot/issues/27)
and [#30](https://github.com/LynnColeArt/hubnot/issues/30). It does not publish a
release or certify new downloadable binaries. The original requests are related,
not duplicates; both should close as completed when the reviewed PR reaches
`main`. The implementation was independently reviewed in five work packages and
consolidated locally at `f268db842ba6e46b8d31ec875ee58b7b86120bfd`.

## Product boundary

- Ordinary collaboration requires no new setup, policy, account, extension or
  worker. Runners remain explicitly started. Ungoverned explicit pipelines and
  existing execution authorization/sandbox defaults remain available.
- Automatic runners skip verified obsolete lineages and enough already-observed
  qualifying trusted evidence. Counting shares governance's latest-result-per-
  trusted-runner semantics across authorized exact requests.
- `hn run withdraw FULL_REQUEST --reason TEXT` explicitly writes a requester-only
  signed future-start withdrawal. Exact normalized retries return the original
  fact. It neither interrupts running processes nor invalidates earlier or late
  results, decisions or merge facts. Old readers may reject the new kind and
  the actor history containing it; no new fact is implicit.
- Stale-base filtering is opt-in using both `--skip-stale-base` and an explicit
  full branch `--target`. It observes the configured remote's exact advertised
  ref without fetching/updating local refs: 10-second deadline, 64 KiB combined
  output budget and process-group cancellation. Uncertainty prevents execution.
  Supported built-in Git/SSH prompts are disabled; configured credential helpers
  remain trusted operator code. Observation is a snapshot, not a branch lock.
- Two runners can still start before either observes a result or withdrawal.
  Optional native coordination remains [#37](https://github.com/LynnColeArt/hubnot/issues/37),
  with no claim of exactly-once effects or a signed event acting as a global lock.

## Validation scope and release boundary

Production, test, action and documentation inputs after consolidation match
independently approved source `34ee0aa5bf778863cb10c73cd092f4b6417c01f6`.
Charter/configuration and mission records are separate reviewed workflow changes.
Strict mission acceptance passed without skipped/blocked/failed checks or path
violations. The real Go layout is explicitly configured as the module root for
workspace/tests; no empty directories or validation bypass were used.

The initial monolithic `go test -race -timeout 20m ./...` at `f60d4046` was
**failed/incomplete** at its 1,200-second aggregate root-package timeout. All
non-root packages passed in that attempt; no earlier assertion failure was
reported. Regression attribution remains unknown. This is not a full-suite pass.

The charter was explicitly amended, reviewed and versioned to require relevant
bounded race/default/static checks per WP and affected native integration,
migration, protocol, trust and recovery checks for source acceptance. Exhaustive
capacity, transport-deadline and real-clock soak qualification remains a native
prerequisite before the next release in
[#38](https://github.com/LynnColeArt/hubnot/issues/38). No test assertion, fixture
size or deadline was reduced, and hosted CI gained no deep corpus.

Recorded validation includes:

| Scope | Source and result |
| --- | --- |
| Direct native action entry points | `c65e32c5`: fast 121.145 s wall / 44.525 s root package; migration 25.668 s; integration 87.284 s; exit 0 |
| Critical standard/native-only race cohorts | Runner/policy, withdrawal, shallow, released-writer compatibility/recovery and governance selections; all passed with explicit variant evidence |
| Actual native Hubnot pipeline | Source `8f0db766`, temporary fixture `cf8671a848a91a8b076a48ec37728b0a6708770e` changes only policy configuration; existing action/pipeline unchanged; 20-root fast manifest; signed passed result `sha256:8dd7cf775bb606e21fe1d5c9b52325e6632bfcd626232435a68ed37c117a1e0b`; pipeline 142.507 s, total 152.720 s |
| Final source additions | Exact metadata-free archive of `34ee0aa5`: public journey, moderated legacy refusal and cross-request evidence race roots passed; actual standard/native-only child binaries recorded; 19 selected old inputs unchanged |
| Final manifest | 22 selected roots; covered incrementally. No single final-22-root native pipeline run is claimed by the preceding 20-root result |
| Static checks | Relevant committed source passed gofmt, `go vet ./...`, build and diff checks; Darwin ARM64 source builds are compile checks, not Mac runtime certification |

Per-WP source/command/exit/hash records and independent verdicts were retained
under the dated runner-efficiency evidence directory. The versioned
`mission-review.md` records the fresh consolidated-source audit and any additional
current-source probes. Public PR checks supply their own exact head/run bindings.
None of these statements certifies exhaustive coverage or release binaries.
Withdrawal duplicate validation is O(W*N) in withdrawals and observed events;
withdrawal-heavy large-history capacity remains unqualified under #38.

## Preservation and tracker decisions

The original user checkout, 78 recorded file hashes/modes, 21 original stashes,
previous mission worktrees, frozen Mac candidate and published beta.2/beta.3
assets are preserved. Implementation occurred in a separate local clone.
No release tag, asset, package checksum or runtime support promise changed.

Fourteen historical issues were inspected. No additional obsolete open issue or
true duplicate was established. Standing acceptance/hosting (#20), general memory
operation recovery (#21), and Mac signing/notarization (#24) remain separate.
Only #27/#30 are completed by this delivery. #37/#38 retain the deliberately
unimplemented coordination and qualification work instead of hiding it in closed
issues. Their tracker bodies will bind the reviewed PR and final main commit.

## Retrospective interpretation

The generated `retrospective.yaml` is retained unchanged. Its WP04 `--force`
warning refers to event `01M6HV2HM729EV1TQXBEH75WRB`: the supported backward
rewind from `in_review` to `planned`, with `changes_requested` and the canonical
review-cycle reference. This records an independent rejection and correction;
it was not an override accepting failed code or bypassing a review gate.
The generated analysis-report warning also requires interpretation: the final
analysis has zero findings and a ready verdict, not unresolved product findings.

Lessons for the next mission:

- Keep source acceptance, exhaustive native qualification and exact release
  binary runtime testing distinct, with immutable source/variant bindings.
- Test outer command admission as well as inner handlers. WP04's refusal once
  migrated legacy identity before stopping; the independent regression now
  checks untouched private state and a genuine eligible control without reset.
- Make effect assertions non-vacuous: prime markers before the action, finish
  refusal assertions before controls, and record actual child binaries/tags.
- Inspect capacity-suite budgets before launching a monolithic timer. Preserve
  failed attempts and use an explicit reviewed qualification contract.
- Preserve generated coordination artifacts before supported merge recovery;
  do not manufacture runtime status, verdicts or signed product history.

Retrospective synthesis was run in its default dry-run mode: no proposals,
conflicts, applications or global doctrine mutations were produced.
