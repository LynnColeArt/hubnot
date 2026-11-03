# Filesystem portability contract

Filesystem portability is exercised by ordinary Go tests on Linux and macOS.
It is part of the migration/private-state boundary, not a caller-side test
workaround. The same commands work locally without GitHub, Python or a provider
account; `.github/workflows/portability.yml` supplies optional hosted execution
on both platforms for PRs and `main`.

## Paths and temporary state

`internal/safepath` is the shared policy for migration sessions, archive
publication and every migration/project/archive temporary spool. New spool
allocation must use `safepath.MkdirTemp`, not a raw default temporary path that
will later fail private-directory checks. Session create/reopen stores an
absolute path with the same platform normalization.

On macOS only, the exact OS-owned `/var` → `/private/var` and `/tmp` →
`/private/tmp` aliases are accepted after checking their symlink owner is root
and their actual target is the expected system path. An ordinary directory at
either location needs no alias exception. The selected suffix is preserved.
All remaining components must still be real directories; a configured TMPDIR,
checkout, parent or child link is not trusted merely because it resolves to an
otherwise permitted destination. Linux gains no alias exception. This is not a
general `EvalSymlinks` pass over user-controlled paths.

Private leaf directories retain owner-only permission checks. Sealed files,
blob bounds/digests, destination identity/ref guards and worktree reciprocal
pointers retain their existing validation. Paths are rechecked at session
read/lock/write boundaries; replacing a planned session with a symlink must
refuse before any destination effect, even if it links back to the original
valid session. This policy does not grant an untrusted process authority over
the private session directory or authenticate arbitrary filesystem mutations.

The regression deliberately launches a macOS child with **no TMPDIR**, uses
literal `/tmp` and `/var/tmp` session/archive paths, and performs local import
followed by exporting imported foreign records. Do not "repair" that test by
normalizing its inputs, changing the global temp environment, or inheriting a
physical TMPDIR in the child. The existing compiled standard/`nogithub`/`noforge`
public journey also keeps its closed child environment unchanged.

The source PR jobs additionally use `hn-release` to build the exact checked-out
commit's standard/native-only archives for the runner's native platform, then
run the bounded `TestBetaPlatformPackageInstallAndRuntime` check on both variants.
The longer `TestPortableCollaborationIntegratedPublicVariants` journey remains
an explicit native integration/full-suite check. It can consume extracted
binaries with the test-only `-hn-package-dir` and optional `-hn-package-version`
flags. It validates each
BUILD source commit, architecture, variant, version and executable digest
before running the unchanged scenario. There is no fallback source rebuild for
a missing/invalid package; the separate `nogithub` subtest is explicitly skipped
because the release has no matching asset. Without these flags all three
original source-build variants continue to run. These flags affect Go tests
only, never the application's CLI or runtime behavior.

## Git trees and case sensitivity

A committed Git tree can contain names a host filesystem cannot materialize.
Tests for case collisions, reserved/trailing names and symlink modes construct
their selected commit directly using Git object plumbing. They retain a clean
ordinary working tree/index and ask the packager to inspect the hostile
revision. The case fixture proves both `case` and `CASE` entries exist and that
refusal is specifically for their collision; a directory-component collision
is tested too. No case-sensitivity probe, skip, assertion relaxation or temporary
filesystem change may substitute for constructing that input.

Fixtures asserting object-database immutability disable automatic Git
maintenance before their first commit. Otherwise a newer Git can detach a
maintenance process whose transient lock races the assertion's directory
snapshot. Keep the complete byte-for-byte assertion; do not ignore disappearing
files or weaken it to accommodate unrelated fixture background activity.

## Repeatable gates

For ordinary development and PR feedback, run the same bounded actions used by
the hosted workflow:

```sh
./.hn/actions/ci fast
./.hn/actions/ci migration
```

The migration action keeps the complete platform-specific regression selection
in three balanced shards with the original 2-minute race-test cap per shard.
See [project CI](project-ci.md) for step/job caps and deeper native tiers. The
following broader direct commands remain explicit local qualification tools;
they are not all duplicated on every hosted PR:

```sh
go test -race -count=1 -timeout 10m ./internal/safepath ./internal/release ./cmd/hn-release
go test -race -count=1 -timeout 10m -run '^(TestMigration.*|TestProjectSnapshot.*|TestPortableCollaboration.*)$' .
go vet ./...
```

To qualify locally generated archives, build a **clean committed source** with
`hn-release --rev COMMIT --version 0.0.0-portability-proof --targets OS/ARCH`,
extract its two binary archives into `PACKAGE_DIRECTORY`, then run from that
same source checkout:

```sh
go test -race -count=1 -timeout 10m -v -run '^TestPortableCollaborationIntegratedPublicVariants$' . -args -hn-package-dir PACKAGE_DIRECTORY
```

For another version, also supply `-hn-package-version VERSION`. The source
commit must equal the packaged BUILD record. Evidence logs print the exact
executable digests; private test repositories and signing keys must stay local.

Run on both Linux and macOS with their normal temporary environment. Platform
logs identify the actual architecture; `macos-latest` does not imply Intel and
Apple Silicon are both qualified. Full source validation still uses the normal
whole-repository test/race/build gates; these focused portability jobs avoid
duplicating every expensive unrelated suite on each PR.

Release qualification separately exercises the **actual standard/native-only
archives** on each claimed OS/architecture. Preserve the download/executable
hashes, macOS version, native/Rosetta distinction, unmodified environment,
command exits and evidence. A source test or workaround pass does not qualify a
published binary. A missing host stays explicitly unverified.

## Existing release limitation

[Issue #16](https://github.com/LynnColeArt/hubnot/issues/16) records the
`0.0.2-dev` migration failures and their measured workaround: explicit selected
paths must be physical, and the CLI's own environment must use a physical
temporary directory (for example `TMPDIR=/private/tmp`). Setting the outer
test's TMPDIR alone does not repair explicit alias paths or a closed child
environment. Published `0.0.2-dev` assets remain unchanged. The fix shipped in
`0.0.3-dev`; [beta.2 exact-package checks](releases/0.1.0-beta.2.md) now also pass
on native Apple Silicon with literal aliases and no child TMPDIR workaround.

## Immutable beta candidate qualification

The published [beta.2 bundle](releases/0.1.0-beta.2.md) was built once with pinned
Go 1.26.4 and version `0.1.0-beta.2`; the same bytes were transported to the three
supported native targets. It supplies a separate non-race qualification kit.
The legacy `qualification=beta` hosted dispatch still uses its historical
`0.1.0-beta.1` build label; the optional-extension dispatch has a distinct
qualification label. Neither is the published beta.2 bundle. Current dispatch
scope is documented in [project CI](project-ci.md); old full-matrix observations
remain in [historical qualification](platform-qualification.md).

Explicit package flags also route `buildMigrationCLIBinary` through the same
verifier, so released-writer, public recovery and migration-journal journeys
execute the selected assets without an application rebuild. Compiler, source
tree and actual executable build information must agree with BUILD; qualification
passes `-hn-package-compiler go1.26.4`. Native-only accepts `noforge` or combined
`nogithub,noforge` requests and always selects the shipped native-only asset.
Normal source builds retain their original tag semantics.

The package install/runtime harness tests literal `/tmp` and `/var/tmp` on
macOS with no child TMPDIR, including foreign-record local import and archive
export. It does not substitute the existing internal macOS regression, which
remains useful source evidence. Browser quarantine/Gatekeeper behavior remains
a separate manual download check.
