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

The hosted jobs additionally use `hn-release` to build the exact checked-out
commit's standard/native-only archives for the runner's native platform. The
same public journey can consume their extracted binaries with the test-only
`-hn-package-dir` and optional `-hn-package-version` flags. It validates each
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

## Repeatable gates

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
environment. Published `0.0.2-dev` assets remain unchanged; the source fix must
be qualified and packaged in a subsequent release.
