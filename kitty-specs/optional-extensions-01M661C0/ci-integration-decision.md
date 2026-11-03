# Native CI integration decision — 2026-11-03

The maintainer authorized integration of `fix/native-ci-feedback` into this
mission and explicitly required preserving the Mac candidate already under test.
This decision supersedes the older WP25 appendices **only for CI scheduling,
execution location and change-appropriate revalidation**. It does not alter the
52 requirements, production limits, default-workflow contract, 24 historical
approvals, WP dependencies, or independent adopter requirements.

## Adopted source and ownership

The coordination branch `kitty/mission-optional-extensions-01M661C0` incorporates
`1c9d0dba920b509cf223fe9ada82df55f849863b`, including the three CI commits based
on the frozen candidate. This is integration, not WP25 approval or mission merge.
The final mission target remains `feature/optional-extensions`.

Root adopts a project CI maintenance overlay consisting of these exact paths:

- `.github/workflows/portability.yml`
- `.github/workflows/optional-extensions-qualification.yml`
- `.hn/actions/ci`
- `.hn/pipelines/test.json`
- `.hn/pipelines/integration.json`
- `.hn/pipelines/stress.json`
- `.hn/pipelines/stress-artifacts.json`
- `.hn/pipelines/soak.json`
- `ci/fast-tests.txt`
- `extension_package_acceptance_test.go`
- `README.md` (the project-CI documentation link only)
- `docs/ci-v0.md` (the project-CI documentation link only)
- `docs/project-ci.md`

These are coordinator-owned integration edits. They do not reopen approved WP23
or transfer its other documentation, expand extension capabilities, or authorize
edits to the frozen lane-x worktree. The native policy file is unchanged.

## Current verification contract

1. Native Hubnot CI is the primary project execution path: signed proposal,
   exact pipeline request, explicit runner, signed source-bound result. The
   shared fast action also runs in the optional GitHub portability jobs.
2. Hosted PR portability jobs have a ten-minute cap. Explicit hosted package
   smoke uses a ten-minute build cap and fifteen-minute native job caps, with
   five focused extension journeys per variant and a three-minute invocation
   timeout, plus a separate two-minute install/runtime check. A smoke pass is
   never an exhaustive qualification pass.
3. The original 206-journey registry, at/over-limit assertions, 100000-record
   fixtures and real ten-minute expiry remain intact. Deep stress and soak runs
   are explicit native requests, not mandatory repeats on every hosted platform.
   Full-suite and historical/default-performance tools remain available. Run
   affected checks when their inputs change; do not rerun a multi-hour corpus
   solely because a documentation or CI-scheduling commit changed HEAD.
   The explicit native `integration` pipeline runs the broad native collaboration,
   artifact/recovery and three reference-consumer journeys with a ten-minute
   step cap and source-assisted binaries.
4. Actual native package execution is still required for each claimed platform
   and variant. Cross-compilation is not native runtime evidence. Source-assisted
   checks and exact-package checks retain distinct labels and exact identities.
5. Missing runs, historical failures and cancelled runs remain missing, failed
   and cancelled. This decision supplies no new pass, trust role, acceptance,
   default worker, scheduled service, main-branch merge or release publication.

See [project CI](../../docs/project-ci.md) for runnable commands and boundaries.
The existing package matrix remains pending until its actual observations are
reconciled under this contract; its old exhaustive row format must not be used
to relabel smoke evidence as a 206-journey pass.

## Evidence already obtained

At `ef3d0bcbc008025cb65199a8da3627e937e0d8ca`, actual default Linux Bubblewrap
execution in an isolated verification clone published signed results:

- Fast `test`: passed, 98096 ms;
  `sha256:e163fe784d4f90cda6b1047526e815e4f6ad42526100e7273e1cbd8120730739`.
- Focused `stress-artifacts`: passed, 183300 ms;
  `sha256:fb1fc0f5bc013bd513ed5704096edaa572f11f2a3bb6e2fb93c87705a8d3a7a0`.

These use a fresh verification identity, not a newly authorized project trusted
runner. The initial sandbox failure due to Go being absent from the fixed PATH
was retained; the explicit official Go-path correction was then verified.
Both Linux/AMD64 exact-package smoke variants passed in approximately 97–98
seconds. All eight packages and twelve reference executables were built using
the Go-only tooling. No new Mac smoke execution is claimed.

The later `1c9d0dba` commit only corrects the run-show documentation and removes
duplicate hosted vet. Action, pipeline and Go inputs match the verified head.
Local raw evidence is retained at
`/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-native-ci-20261102/verification-summary.json`.

## Frozen Mac candidate and remaining work

Preserve source `3dcbb96091c6191bc5d98cde9dbd1d2b98433f95`, tree
`df57aeda7553e88ab1eb54ab2f0564a7844dee01`, lane-x, run `41465035433`
attempt 1, artifact `13149409710`, and all existing aliases/assets/results.
Do not rebuild or replace those bytes, cancel the external Mac investigation,
or reinterpret its observations as runs against the new CI source. The old
candidate's harness does not implement the new smoke selector: never dispatch
   the new smoke workflow against that old source and claim compatibility.

The CI delta changes no production Go or module input relative to the frozen
candidate. This permits scoped reuse of unchanged-code evidence with explicit
identities; it does not make commit/tree or package provenance interchangeable.

Follow-through identified at initial integration (items 1–2 are completed by the
corrected-source verification recorded below):

1. Verify the integrated native fast pipeline and source/preservation attestations.
2. Reconcile WP25's package evidence into distinct functional/platform, deep
   stress and distribution-launch outcomes. Keep unobserved rows pending.
3. Track the actual browser-download Mac launch limitation reported in
   [issue #24](https://github.com/LynnColeArt/hubnot/issues/24#issuecomment-6595409846):
   quarantined ad-hoc-signed binaries fail Gatekeeper. The maintainer accepts
   this as a disclosed unsigned-beta limitation (decision below); signing and
   exact signed-package retest remain separate distribution work. Removing
   quarantine is not evidence that the original download works.
4. Complete normal independent WP25 review, then WP26's boundary review and
   independent native-user/extension-author trials. Missing external observations
   cannot be replaced by implementer replay. Acceptance remains pending.

## Smoke scope correction after actual Mac execution

Post-integration run `41589853578` used exact source
`3f4c1a39a4fde59a0c2c04c0f38446de5b08a391`. Both Linux targets passed all
thirteen selected journeys per variant. Both Mac targets hit the unchanged
three-minute enclosing test timeout while running the broad native collaboration
journey or the next artifact journey; those four variant attempts are incomplete,
not passes. Logs and artifacts are retained. An earlier dispatch `41589800580`
had an incorrect operator-supplied candidate SHA and was cancelled; it provides
no qualification evidence.

The corrective scheduling change keeps the three-minute hosted invocation cap
and the entire 206-entry registry. Hosted extension smoke selects the five
existing registration, enablement, explicit run, inert launch and raw artifact
type-read callbacks. The existing package install/runtime test separately checks
both exact native variants within two minutes. The broader native collaboration
callback is exposed unchanged through `TestProjectCINativePublicJourney` for the
explicit native integration action, alongside the six-family, artifact recovery
and three reference-consumer journeys. The action builds and pins source-assisted
executables; it does not claim to certify a downloaded archive. This adds no
production feature, relaxed public timeout or changed stress assertion.

The corrected source `d5aee5d0226c2e86615c1a0910d3cc8b9592cce7` passed both
native sandbox pipelines: fast `test` in 101161 ms (result
`sha256:caa7e14d31f88704726e3f2d17e8dce54f072909bf16c4b4cc62a717a1f8d689`)
and `integration` in 116154 ms (result
`sha256:5c36d45af197dc1020dbfddcdec155aac8f69b3cddd87c06b86525b60c23378d`).
Both remain verification-clone results with no project trust-role change.
Hosted run [41591987505](https://github.com/LynnColeArt/hubnot/actions/runs/41591987505)
passed all eight native package-smoke rows and both install/runtime variants on
each platform. Build plus the complete matrix took 321 seconds. All eight
artifact groups were independently reconciled against source/tree/compiler,
package/reference hashes, five selected roots, 201 exclusions, no source fallback,
and the expected failing missing-reference probe. The package matrix now records
actual native smoke observations while preserving pending acceptance and the
distinct Mac browser-launch blocker. Later evidence-only edits change no tested
Go, action or pipeline input.

## Accepted unsigned Mac beta limitation

On 2026-11-03 the maintainer confirmed that no Developer ID setup is available
and explicitly accepted Mac beta packages without Developer ID signing or
notarization as a documented limitation. Signing is not a prerequisite for
functional qualification, independent WP25 review or WP26 trials. The known
Gatekeeper/browser-download launch failure stays in the evidence; that install
path remains uncertified and release notes must disclose it. This choice neither
qualifies the whole beta nor changes any historical failure into a pass.

Separate distribution follow-up can add Developer ID signing/notarization and
retest the resulting exact downloads with quarantine retained. See Apple's
[custom notarization workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow).
Account enrollment and signing identity availability are prerequisites for that
future signing work, rather than beta mission gates;
this integration does not create an account, purchase a membership or claim a
signing result. Signing will change executable/archive digests, so future signed
assets need new provenance, checksums and native runtime/launch observations.
Keep the existing unsigned candidate and its evidence intact.

The original primary checkout's 78 dirty entries and 21 retained stashes remain
preserved. A generated untracked coordination dossier that collided with the
incoming tracked dossier was copied byte-for-byte before integration to
`/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-ci-integration-20261103/coord-untracked-snapshot-before-integration.json`.
