# Go-only release packaging

Build the local packager with Go 1.26 or newer and Git 2.x. It uses the Go
standard library and invokes only Git and Go, using argument arrays. It runs
without Python, a Unix shell, Docker, provider CLIs or a provider account.

```text
go build -trimpath -o hn-release ./cmd/hn-release
hn-release --source . --rev COMMIT --version 0.0.2-dev --out ../release-0.0.2-dev
```

On Windows, build `hn-release.exe` and invoke `hn-release.exe` (or its full
path). No shell syntax or platform archive utility is required by the command.
The parent of the output directory must already exist; the output directory
itself must be new. Even an empty existing directory or a symbolic link is
refused. Build failures and cancellation leave no published output directory.
The final local directory transfer is not a crash-atomic filesystem transaction;
after a process or machine crash, inspect any partial directory and select a new
output directory for a fresh run.

`--rev`, `--version` and `--out` are required. The source defaults to the current
directory. A release version has the form `X.Y.Z` or `X.Y.Z-prerelease`, without
a leading `v`, path separators, whitespace or build metadata. The version is
injected into `main.version`; the normal source build retains `0.0.1-dev`.
Historical source without a linker-settable variable is refused rather than
quietly producing an executable with the wrong version.

The default targets are `linux/amd64`, `linux/arm64`, `darwin/amd64` and
`darwin/arm64`. The default variants are `standard` and `native-only`.
For a subset, use comma-separated `--targets` and `--variants`, for example:

```text
hn-release --rev COMMIT --version 0.0.2-dev --out ../release-linux --targets linux/amd64 --variants native-only
```

Unknown or duplicate targets/variants are refused. Windows CLI distribution is
outside this release matrix. `native-only` builds with `nogithub,noforge`:
`nogithub` excludes GitHub before all-adapter `noforge` support is integrated,
and remains GitHub-specific. The combined tags exclude all installed network
adapters once `noforge` is integrated. They retain native Git collaboration.

## Source and build inputs

Commit the source before packaging. Tracked worktree/index changes are refused.
Before checking cleanliness, the command inspects effective local Git
configuration (including worktree configuration and includes) without running
helpers. Configured `filter.*.clean` or `filter.*.process` commands are refused,
even for an otherwise clean checkout. Use a clean checkout without executable
filters. Filesystem monitors are disabled. Partial-clone/promisor configuration
and indexed submodules are refused before inspection can start fetch/transport
helpers or recurse into another repository's executable configuration. Source
objects must already be present locally.
Untracked and ignored files are excluded, including local binaries and private
working files. The named revision is resolved once to its commit and tree;
changing HEAD or adding `refs/replace` cannot silently substitute its contents.
The exact committed tree is read from Git blobs rather than `git archive`, so
`export-ignore` and `export-subst` do not remove or rewrite committed source.
Intentionally committed material is included, including historical/governance
files; this command is not a secret scrubber.

Links, submodules, nonportable paths, Windows-reserved names and case-colliding
paths are refused before writing output. Archive paths always use `/`.
Committed executable files use mode `0755`; other files use `0644`.
All archive member times use the source commit timestamp; owner/group names,
IDs, access/change times and gzip headers are normalized. Members and checksum
entries are sorted. Source archive members are exactly the regular committed
tree, under one versioned root directory.

Builds extract that tree into a fresh temporary directory. The command disables
CGO, workspace selection, ambient Go flags/experiments, VCS stamping and module
networking; it uses `-mod=readonly`, `-trimpath`, `-buildvcs=false` and linker
flags `-s -w -buildid= -X main.version=VERSION`. Target baselines are
`GOAMD64=v1` and `GOARM64=v8.0`. `GOTOOLCHAIN=local` prevents an implicit compiler
download. The selected Go compiler must already be installed. The ordinary Go
compiler cache may be reused; source, output and temporary build paths do not
enter archive provenance or executable bytes.

## Assets and verification

The full matrix generates eleven files:

- Eight `hubnot_VERSION_OS_ARCH[_native-only].tar.gz` binary archives.
- `hubnot_VERSION_source.tar.gz` with the exact committed source.
- `PROVENANCE.json` binding source commit/tree/timestamp, compiler/version and
  every archive digest, including each binary's build inputs/executable digest.
- `SHA256SUMS` covering every generated asset except itself.

Each binary archive includes `hn`, `LICENSE`, `README.md`, optional
`CHANGELOG.md`, the committed `docs/` tree, `INSTALL.md` and `BUILD.json` under
its asset root. `BUILD.json` records source/compiler/version/target/variant,
controlled build flags/environment and the executable SHA-256. It does not claim
a digest of the containing archive. `PROVENANCE.json` inventories those final
archives without a circular self-digest; its own digest is in `SHA256SUMS`.
Schemas are `hn.release/1` and `hn.release-build/1`.

Run the command twice using the same committed source, Go compiler, version,
target set and variant set, choosing different new output directories. Compare
every file byte-for-byte, independently inspect archive member paths, contents
and modes, and recompute the listed SHA-256 values. Reordering the requested
target/variant sets does not change outputs. A changed compiler or committed
tree is a changed input and requires new evidence.

The package tests independently read the gzip/tar archives, verify their metadata
and exact source selection, execute the Linux AMD64 fixture binary to check its
reported version, and recompute executable/archive/checksum bindings. They also
cover dirty input, replace refs, hostile filenames, duplicate/unknown inputs,
output collisions, compiler failures and cancellation.

## Platform evidence and publication boundary

Before packaging a migration change, run the [filesystem portability
gates](filesystem-portability.md) on Linux and macOS with their normal temporary
environments. The hosted PR workflow runs those same Go commands. Do not add
physical-path or TMPDIR workarounds to make acceptance pass. Case-collision
fixtures must construct committed Git trees rather than assume the checkout's
filesystem is case-sensitive. Published binary runtime qualification remains a
separate check for each target and variant; a source test is not a substitute.

| Platform | WP01 verification scope |
| --- | --- |
| Linux AMD64 packager host | Executed packaging and archive/refusal tests with Go 1.26.4. |
| Linux/macOS AMD64/ARM64 artifact targets | Cross-compilation and archive inspection; runtime execution must be reported separately for each target. |
| macOS packager host | Cross-compilation only; host runtime remains unverified. |
| Windows packager host | Cross-compilation only; host runtime remains unverified. |

Cross-compile the packager by setting `GOOS=darwin` or `GOOS=windows` and the
desired `GOARCH` in the invoking environment, then running
`go build -trimpath -o OUTPUT ./cmd/hn-release`. This is compilation evidence,
not evidence that it was executed on those hosts. Record exact source hashes,
commands, results and host observations with the review evidence.

Packaging creates local reviewable assets. Publication, tag creation, signing,
Apple notarization and runtime qualification are separate operations. No command
here publishes a release, changes tags or modifies project policy. Published
`v0.0.1-dev`, its assets and its historical Python packaging asset remain
unchanged; use this Go command prospectively for subsequent releases.
