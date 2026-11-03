# Changelog

User-visible changes and release requirements for Hubnot. The protocol remains
experimental. Beta prerelease publication does not by itself adopt the full
[prospective support contract](docs/beta-support.md).

## Unreleased

### Fixed

- Replication receipts record newly accepted objects without repeating every
  settled object closure. Full validated closure remains available for recovery,
  and transaction allocation avoids IDs belonging to retained receipts/anchors.
  Existing receipts are not compacted. Andrew's [PR #32](https://github.com/LynnColeArt/hubnot/pull/32)
  is merged on `main`; this fix is not in the published beta.2 packages.

### Documentation

- Refreshed the README, support/platform guides, packaging and feature guidance
  for the published beta.2 prerelease. Current results are distinguished from
  frozen alpha, beta.1 and extension-mission qualification records.
- Added an isolated plugin quickstart and a release checklist requiring open
  PR/issue review before candidate freeze and publication. Documented beta.2's
  receipt-growth issue and the separately reviewed, now merged fix in PR #32.

## [0.1.0-beta.2](https://github.com/LynnColeArt/hubnot/releases/tag/v0.1.0-beta.2) — 2026-11-03

First published beta prerelease, built from
`58c14768dfd094ce8dfaac747591c1e66d9a64de` with Go 1.26.4 and CGO disabled.
Both variants of the exact downloadable Linux AMD64, Linux ARM64 and Apple
Silicon Mac binaries passed bounded native smoke checks. Intel Macs are outside
this prerelease's supported platform set. **Mac Developer ID signing and
notarization remain pending.**

### Added since 0.0.3-dev

- Explicit `hn memory grep` with Go/RE2-style regex, shared trust/applicability
  filters, deterministic bounded pages, and refusal of invalid/over-complex
  patterns. Lexical recall does not automatically turn into regex search.
- Released-writer compatibility fixtures, keyless public reconstruction,
  exclusive signer-restoration and interrupted-operation recovery guidance and
  regression coverage. A prospective beta contract records the remaining
  acceptance work; publishing this prerelease does not adopt its full promise.
- Optional native contribution and moderation workflow for signed collaboration
  and code proposals without upstream push permission. Local files, explicit
  authority selection, bounded intake and governed admission remain separate
  from review, execution and merge authority; no listener or host is required.
- Generic signed `hn-artifact/1` custom document/list/ticket types, pure versioned
  schemas, explicit revision/conversion/operation recovery, bounded search and
  plugin-free public exchange. Existing collaboration and memory protocols stay
  separate.
- Optional local `hn.extension/1` stdio host with explicit registration,
  enablement/grants and run, plus fake-tracker, charter-model and inventory-model
  reference consumers. No automatic launch, default extension setup, embedded
  Spec Kitty doctrine engine, production enterprise connector or hostile-process
  sandbox is introduced. The ordinary native workflow remains the first gate.
- Native Hubnot CI tiers for fast, migration, integration, stress and real-clock
  soak work. Hosted PR checks use bounded regression/package smoke; the complete
  deep corpus remains explicitly available through native/local execution.

### Downloads

| Platform | Standard | Native-only |
| --- | --- | --- |
| Linux AMD64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/hubnot_0.1.0-beta.2_linux_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/hubnot_0.1.0-beta.2_linux_amd64_native-only.tar.gz) |
| Linux ARM64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/hubnot_0.1.0-beta.2_linux_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/hubnot_0.1.0-beta.2_linux_arm64_native-only.tar.gz) |
| macOS Apple Silicon | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/hubnot_0.1.0-beta.2_darwin_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/hubnot_0.1.0-beta.2_darwin_arm64_native-only.tar.gz) |

[Source archive](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/hubnot_0.1.0-beta.2_source.tar.gz),
[SHA256SUMS](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/SHA256SUMS)
and [build provenance](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.2/PROVENANCE.json)
bind the unchanged tested package bytes. Native-only excludes all network
provider adapters. Applications require Git, not Python or Go.

### Verification and distribution limits

- Linux AMD64 passed both direct and GitHub-download-back smoke; Linux ARM64
  passed natively on Golden Goddess (Ubuntu 24.04.4 LTS, Git 2.43.0). Apple
  Silicon passed natively on Pink Menace (macOS 26.5.1, Apple Git 2.50.1,
  translation flag 0), as reported in [#33](https://github.com/LynnColeArt/hubnot/issues/33).
- Both variants passed bundle validation, invalid-package/no-fallback controls,
  installation/runtime, memory regex and native migration, plus five extension
  callbacks each. Harnesses were non-race; 201 of 206 extension roots were
  explicitly excluded. These observations are not exhaustive or new race-suite
  passes and do not certify untested host versions.
- All eight application archives passed static architecture/build-info checks.
  All fifteen release assets matched server digests and downloaded bytes before
  publication. The unqualified Intel Mac archives remain in the complete bundle
  for provenance, outside the supported download table.
- Apple Silicon runtime used terminal-acquired, linker ad-hoc-signed binaries
  without quarantine. Browser-download/Gatekeeper testing of these exact bytes
  was not run; the earlier browser limitation remains unresolved. Signing and
  notarization are follow-up work, not inferred from a runtime pass.
- The separate qualification kit supplies native harnesses/reference executables
  and an offline public-source bundle; it needs no Go or Python installation.
  Its original brief/staging status are historical pre-publication snapshots.
- [Exact release evidence and platform decisions](docs/releases/0.1.0-beta.2.md)
  record hashes, commands, scope and outstanding limits. Windows distribution,
  broader beta acceptance and production-provider qualification are not supplied
  by these package observations. Earlier release assets and records stay intact.

## [0.0.3-dev](https://github.com/LynnColeArt/hubnot/releases/tag/v0.0.3-dev) — 2026-10-27

macOS migration portability alpha prerelease, built from
`01a152cfbb822a7be9ccbc51dac34e3caa0cf925`. The executable reports `0.0.3-dev`.
**Native Apple Silicon rebuilds passed; testing of the exact downloadable Mac
binaries is still pending for both Apple Silicon and Intel.**

### Fixed

- Migration sessions and internal project/archive/imported-record spools share
  a path policy that handles verified macOS system temporary-directory aliases
  while retaining rejection of arbitrary symlinks and unsafe private paths.
  macOS regressions exercise literal aliases and child environments without
  TMPDIR. Published `0.0.2-dev` packages retain the limitation in
  [#16](https://github.com/LynnColeArt/hubnot/issues/16).
- Packager refusal fixtures build hostile committed trees directly with Git
  plumbing, including file and directory case collisions, so the same assertions
  exercise real inputs on case-insensitive filesystems.
- Object-database immutability fixtures disable automatic Git maintenance before
  committing, avoiding transient detached-maintenance locks without relaxing
  the byte-for-byte assertions.

### Development

- Linux/macOS PR checks run filesystem, migration and packager regressions with
  normal temporary-directory environments. They build actual standard/native-only
  candidate archives and exercise them through the public acceptance journey.
  These hosted development checks add no runtime or provider dependency.

### Downloads

| Platform | Standard | Native-only |
| --- | --- | --- |
| Linux AMD64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_linux_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_linux_amd64_native-only.tar.gz) |
| Linux ARM64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_linux_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_linux_arm64_native-only.tar.gz) |
| macOS Intel | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_darwin_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_darwin_amd64_native-only.tar.gz) |
| macOS Apple Silicon | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_darwin_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_darwin_arm64_native-only.tar.gz) |

[Source archive](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/hubnot_0.0.3-dev_source.tar.gz),
[SHA256SUMS](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/SHA256SUMS)
and [build provenance](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.3-dev/PROVENANCE.json)
accompany the eight binary packages. Native-only excludes every network provider
adapter while retaining native commands and local/archive migration.

### Verification and distribution limits

- The release source passed the full local race suite and Linux/macOS CI.
  All eleven Go/Git-only outputs reproduced byte-for-byte with Go 1.26.4;
  independent source/archive/executable/provenance checks passed, and published
  downloads match the verified local bundle.
- Exact Linux AMD64/ARM64 standard/native-only binaries passed runtime smoke
  checks; Linux AMD64 packages also passed the integrated public journey.
- [Native Mac verification](https://github.com/LynnColeArt/hubnot/issues/16#issuecomment-6443074382)
  rebuilt the same source with Go 1.26.6 on macOS 26.5.1 Apple Silicon. Both
  standard and native-only packages passed the integrated journey under the
  race detector without path or TMPDIR workarounds. This qualifies those native
  rebuilds, not the downloadable Go 1.26.4 Mac binaries, whose runtime testing
  remains pending. Intel execution is also pending.
- Both Mac architectures passed cross-compilation, archive, architecture,
  checksum and provenance inspection. Downloads have no Apple Developer ID
  signature or notarization.
- The complete release package matrix ran on Linux; native Apple Silicon
  packager execution was separately demonstrated for its native target pair.
  Windows packager-host execution remains unverified, and Windows CLI
  distribution is outside this matrix.
- Prebuilt applications require Git; source builds and packaging require Go and
  Git alone. Protected checks, automerge and independent audits remain optional
  and independently disabled by default, with no implicit background worker.
- Previous release tags and assets remain unchanged. Their original
  qualification and reproduction requirements are retained below.

## [0.0.2-dev](https://github.com/LynnColeArt/hubnot/releases/tag/v0.0.2-dev) — 2026-10-27

Portable-collaboration alpha prerelease, built from
`e80880abc792e2c386182e06027fc251b134c0d2`. The executable reports `0.0.2-dev`.
**Mac runtime testing is still pending for both Intel and Apple Silicon.**

### Removed

- Obsolete tracked `.nh/policy.json` and `.nh/pipelines/test.json` from the retired
  `nh` namespace. The root `.nh/` directory is now ignored. Active `.hn/`
  configuration is unchanged; earlier files remain available in Git history.

### Fixed

- Test repository fixtures now resolve physical paths consistently with Git when
  temporary directories use aliases, including macOS `/var` and `/private/var`.
  This correction changes tests only; existing `0.0.1-dev` assets remain
  unchanged. ([#10](https://github.com/LynnColeArt/hubnot/pull/10))

### Added

- Portable project snapshots and bounded archives retain explicitly selected Git
  history, exact signed native facts, attributed foreign communications and
  retrieved attachment bytes. Saved guided/scripted `hn migrate` plans preview
  capabilities and explicit loss decisions before guarded apply/resume. Imported
  history does not install signers, activate policy or grant runner authority.
- Tested migration profiles for GitHub, GitLab including self-hosted endpoints,
  separate Forgejo and Gitea versions, Bitbucket Cloud source preservation, and
  explicitly mapped Azure DevOps Boards work items/comments. Preservation does
  not imply destination recreation; unavailable reads and removed APIs are
  reported. Legacy explicit transfer commands retain their behavior.
- `noforge` excludes all network adapters while retaining native collaboration,
  local/archive migration, foreign inspection, and independent audits.
  `nogithub` continues to exclude GitHub only.
- Independent signed audit authorizations, revocations, requests and results,
  with separate explicit local authority adoption and existing sandbox/unsafe-host
  execution boundaries. Audit outcomes never supply or block merge evidence.
  Deliberate linked follow-up issues deduplicate retries and later findings,
  preserve operator edits and expose concurrent publisher limits. No scheduler
  or automatic execution is introduced; native policy defaults remain unchanged.
- Go/Git-only `hn-release` packages an exact committed revision into deterministic
  Linux/macOS binary and source archives with normalized metadata, SHA-256
  checksums and declared provenance. Native-only packages omit all adapters.
  The packager cross-compiles for Windows/macOS; their packager-host runtime
  qualification remains outstanding. Packaging never publishes or changes tags.
  The immutable `0.0.1-dev` artifacts retain their original Python reproduction
  requirements and GitHub-only native-only meaning.

### Downloads

| Platform | Standard | Native-only |
| --- | --- | --- |
| Linux AMD64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_linux_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_linux_amd64_native-only.tar.gz) |
| Linux ARM64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_linux_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_linux_arm64_native-only.tar.gz) |
| macOS Intel | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_darwin_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_darwin_amd64_native-only.tar.gz) |
| macOS Apple Silicon | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_darwin_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_darwin_arm64_native-only.tar.gz) |

[Source archive](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/hubnot_0.0.2-dev_source.tar.gz),
[SHA256SUMS](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/SHA256SUMS)
and [build provenance](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.2-dev/PROVENANCE.json)
accompany the binary packages. Native-only excludes every network provider
adapter while retaining native commands and local/archive migration.

### Verification and distribution limits

- The release source passed the full race suite, independent mission review,
  and final pre-merge vet/build/targeted race checks.
- All eleven assets reproduced byte-for-byte with Go 1.26.4. Independent
  inspection verified exact source files, archive contents and metadata,
  executable architectures, static Linux linking, checksums and provenance.
  All eleven published downloads match the verified local files.
- Both Linux AMD64 and ARM64 standard/native-only binaries passed runtime smoke
  checks with Git-only application PATH and no provider credentials, including
  signed native issues, saved archive migration, exact native actor history
  import and local continuation. No new full ARM64 Go-suite run is claimed.
- Both macOS architectures were cross-compiled and inspected. **Mac runtime
  testing is still pending for 0.0.2-dev**; previous Apple Silicon test results
  apply only to the earlier release. The packages have no Apple Developer ID
  signature or notarization.
- Running prebuilt binaries requires Git 2.x. Source builds and the new release
  tooling require Go and Git alone; Python is not required. macOS/Windows
  packager-host execution remains unverified, and Windows CLI distribution
  remains outside this release matrix.
- Protected checks, automerge and independent audits remain optional and
  disabled by default. No scheduler, service or background worker is installed.

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
