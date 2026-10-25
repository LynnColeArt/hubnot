# Changelog

User-visible changes and release requirements for Hubnot. The protocol remains
experimental; alpha releases do not promise stable compatibility.

## Unreleased

### Removed

- Obsolete tracked `.nh/policy.json` and `.nh/pipelines/test.json` from the retired
  `nh` namespace. The root `.nh/` directory is now ignored. Active `.hn/`
  configuration is unchanged; earlier files remain available in Git history.

### Fixed

- Test repository fixtures now resolve physical paths consistently with Git when
  temporary directories use aliases, including macOS `/var` and `/private/var`.
  This is a test-only correction after the first release; published executable
  bytes are unchanged. ([#10](https://github.com/LynnColeArt/hubnot/pull/10))

### Required for the next release

- Replace the Python release packager with tooling that uses **Go and Git alone**
  and runs on Linux, macOS, and Windows. Release packaging must not require
  Python or a Unix-only shell. This is a packaging portability requirement;
  Windows executable distribution remains a separate scope decision.
- Keep runtime, source-build, and archive-reproduction requirements explicit.
  Running a prebuilt `hn` requires Git 2.x; ordinary source builds require Go
  and Git. The published `0.0.1-dev` artifacts remain unchanged.

## [0.0.1-dev](https://github.com/LynnColeArt/hubnot/releases/tag/v0.0.1-dev) — 2026-10-25

First packaged operational-alpha prerelease, built from
`f82b57420f0c825a9ea6f3b9f93ede680c8cf06f`. The executable reports `0.0.1-dev`.

### Added

- Git-native signed issues, projects and typed planning items, threaded
  discussions with pinned answers, and versioned workflows.
- Immutable proposals and revisions, independent reviews, CI evidence,
  policy-governed decisions and local merges, distinct device identities,
  selected quarantined replication, and deliberate signed agent memory.
- Local `hn gh issue` compatibility commands and explicit provider-neutral
  import/export with GitHub as the first optional adapter. Native operation
  requires no provider account or service.
- Independent, opt-in protected checks and rule-based automerge. Protected check
  changes require designated independent approvals; current protected baselines
  prevent hidden lint reversions. Automerge uses an explicit named rule and
  current native evidence, with a read-only preview and separate `--execute`.
  Both capabilities are disabled by default and have no GitHub dependency.
- MIT licensing and downloadable Linux AMD64/ARM64 and macOS Intel/Apple Silicon
  archives, each in standard and native-only variants. Native-only builds omit
  the GitHub adapter while retaining native commands and local compatibility.
- Source archive, product documentation, SHA-256 checksums, build provenance,
  and a reproducible packaging script. Linux binaries are statically linked;
  macOS binaries target macOS 12 or newer.

### Verification

- The release source passed the full race suite, full `nogithub` suite, vet,
  public acceptance matrix, and independent review before merge.
- All eight binary archives passed format, permissions, and provenance checks
  and were reproduced byte-for-byte with Go 1.26.4.
- Both Linux AMD64 packages passed CLI smoke checks. Both Linux ARM64 packages
  passed native resource and policy acceptance matrices on ARM64 Ubuntu 24.04;
  the standard package also passed rich-issue lifecycle and consumer, replication,
  agent-memory, and self-hosting scenarios. An initial consumer setup failure
  was resolved with a source-matched prebuilt fixture and the affected scenario
  passed on rerun; both attempts are included in the release evidence.
- Both macOS Apple Silicon packages passed startup, identity, signed-issue
  creation, and issue-list smoke checks on macOS. Intel Mac packages were
  cross-compiled and inspected; runtime testing remains outstanding.
- Public Linux downloads were checked against the tested archives and published
  SHA-256 checksums. Detailed logs and verification records accompany the release.

### Distribution limits

- Running the packaged executable requires Git 2.x. Python and Go are not
  runtime dependencies. Reproducing this release with its packaging script
  requires Python 3.12+, Git, and Go 1.26.4; removing that packaging dependency
  is required for the next release.
- macOS executables have no Apple Developer ID signature or notarization.
  Download approval may be needed in Privacy & Security. The default safe CI
  executor uses Linux bubblewrap; macOS has no bundled safe CI backend.
- No Windows executable packages are included in this release.
