# Go-only packaging contract

Add `cmd/hn-release` and a testable `internal/release` library. Only Git and the
supported Go compiler may be external executables. Invoke argument arrays, never
a shell, Python, Docker or provider CLI. No external Go modules or shell helpers.

Inputs: committed source revision, validated release version, targets, variants
and output directory. Resolve the source once; use its committed tree/timestamp,
exclude generated/untracked files, and refuse silent output overwrite. Build in
isolated temporary sources with controlled flags/environment, CGO disabled,
trimpath and module networking off. An optional version variable/linker injection
must preserve the development default and match reported executable version.

Use standard Go archive libraries. Normalize entry order, ownership metadata,
timestamps, modes and gzip headers. Binary archives include `hn`, license,
installation guide, product docs and build provenance. Record exact source,
compiler, version, target, variant, flags and executable/archive digests without
circular self-digest claims. Checksums cover generated assets except themselves.
Native-only packages exclude all adapters via `noforge` once integrated; preserve
the separate compatibility meaning of `nogithub`.

No implicit publication, tag changes or policy changes. Published `v0.0.1-dev`
and its old Python asset remain unchanged; next-release guidance uses Go tooling.

Rebuild twice in different temporary paths and compare exact outputs. Inspect
archives/modes/provenance/checksums independently. Cross-compile the packager
for macOS/Windows and keep its operations free of Unix-only assumptions. Actual
host/runtime claims require execution on that host or a declared suitable test
environment; unavailable Windows/macOS runtime tests remain explicitly unverified.
Portable packaging does not imply Windows CLI releases, Apple signing/notarization
or a portable safe CI backend.
