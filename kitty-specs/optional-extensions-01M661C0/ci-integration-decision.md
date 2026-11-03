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
   thirteen existing product journeys per variant and a three-minute invocation
   timeout. A smoke pass is never an exhaustive qualification pass.
3. The original 206-journey registry, at/over-limit assertions, 100000-record
   fixtures and real ten-minute expiry remain intact. Deep stress and soak runs
   are explicit native requests, not mandatory repeats on every hosted platform.
   Full-suite and historical/default-performance tools remain available. Run
   affected checks when their inputs change; do not rerun a multi-hour corpus
   solely because a documentation or CI-scheduling commit changed HEAD.
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

Next work, in order:

1. Verify the integrated native fast pipeline and source/preservation attestations.
2. Reconcile WP25's package evidence into distinct functional/platform, deep
   stress and distribution-launch outcomes. Keep unobserved rows pending.
3. Resolve the actual browser-download Mac launch blocker reported in
   [issue #24](https://github.com/LynnColeArt/hubnot/issues/24#issuecomment-6595409846):
   quarantined ad-hoc-signed binaries fail Gatekeeper. The normal distribution
   path needs a signing/notarization decision and exact signed-package retest;
   removing quarantine is not evidence that the original download works.
4. Complete normal independent WP25 review, then WP26's boundary review and
   independent native-user/extension-author trials. Missing external observations
   cannot be replaced by implementer replay. Acceptance remains pending.

The original primary checkout's 78 dirty entries and 21 retained stashes remain
preserved. A generated untracked coordination dossier that collided with the
incoming tracked dossier was copied byte-for-byte before integration to
`/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-ci-integration-20261103/coord-untracked-snapshot-before-integration.json`.
