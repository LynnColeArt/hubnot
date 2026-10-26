# Independent WP01 review

Verdict: **request changes** for one P1 contract/security blocker.
Reviewed source: `86dc00678c42d495c6442addfcc8da8abd983ffa`.
Base: `kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0`.
Reviewer role is independent of implementation; no product code was edited.

The six-file diff is within WP01 ownership: release CLI/library/tests,
`docs/releasing.md`, and the linker-settable development version in `main.go`.
The development default remains `0.0.1-dev`; the module/dependency set, native
command dispatch, signed facts and policy behavior are untouched.

## Finding

`Package` → `gitCommand` → `git status` executes repository-configured clean
filters while checking tracked worktree cleanliness. Disabling system/global
config and fsmonitor does not disable local filter configuration. A clean source
with a configured marker-producing clean filter was packaged successfully and
executed the marker shell command. This violates release-v1 lines 3–5, which
permit only Git and Go external executables and explicitly prohibit a shell.

See `../review-feedback-1.md`, `reproduce-clean-filter.py` and
`reproduce-clean-filter.log`. The reproducer builds a fresh packager from the
reviewed commit, records its digest, configures the filter after fixture commit,
touches only tracked README mtime, then calls the public packager CLI. Marker
creation is therefore caused by the packager's Git cleanliness invocation.
Required fix: prevent implicit executable helpers or refuse the unsafe configured
case before execution, with marker-absence/state-preservation regression tests
and retained dirty-source/reproducibility checks.

## Validation assessed

- Reviewer ran `go test -race -count=1 -v ./internal/release ./cmd/hn-release`:
  both packages pass (`focused-review.log`). Refusals cover tracked dirt,
  nonportable trees, collisions, failure/cancellation, invalid inputs and
  noninjectable historical versions; source pinning covers HEAD and replace refs.
- Reviewer reran the independent Go archive inspector (no release-library
  import). All eleven generated files match byte-for-byte across two full
  eight-target/variant builds. It verifies 591 exact source files/modes, sorted
  normalized archive metadata, all ten checksums/nine archive digests, target
  ELF/Mach-O types, packaged docs and both Linux AMD64 native CLI lifecycles
  (`archive-reinspection.log`; inspector preserved as `.go.txt`).
- Reviewer separately implemented and ran `reviewer-inspection.py`: checks exact
  commit/tree/time, closed target/variant inventory, provenance schema, equality
  of outer and packaged build metadata, executable digests and controlled
  flags/environment. Pass (`reviewer-inspection.log`). This is a review tool,
  not a dependency of Go/Git product packaging.
- Existing full race evidence was inspected rather than rerun unchanged:
  `go test -race -count=1 -json ./...`, real exit 0; source before/after identical;
  1,173 pass events, no fail events, root package 488.265 seconds. All five tested
  packages completed pass; the remaining `examples/issue-consumer` package reports
  no test files. See copied `worker-race-detached.*` and summary JSON.
- Worker source-bound vet/build/format/diff evidence is preserved; reviewer also
  checked the committed diff. No generated binaries or private identities occur
  in the six-file diff.
- Completed framework pre-review gate: `no_coverage`, missing importable
  `tests.architectural._gate_coverage`, `blocked=false`, `force_bypassed=false`.
  It is not a Go failure or Go suite substitute. Copied original gate log retained.

## Contract walk and platform boundary

Enumerated `contracts/`: release-v1 is in scope; audits-v1 and migration-v1 are
orthogonal (no audit/migration implementation or schema surfaces changed here).
Release-v1 has no concrete payload/flag examples or enumerated matrix to round-trip;
its executable boundary, explicit inputs, archive contents/provenance/checksums,
metadata normalization, native-only tags and host-qualification statements were
checked against code and observable artifacts. Only the executable boundary fails.
Actual caller is `cmd/hn-release/main.go` → `release.Package`; normal `hn` source
build still reports the development default, release binaries report injected
`0.0.2-dev`. Native-only passes `nogithub,noforge`; final all-adapter exclusion
remains WP06/WP12 integration work as planned.

Linux AMD64 host packaging and both packaged Linux AMD64 variants were executed.
Windows AMD64 and macOS AMD64/ARM64 packagers have cross-compiled PE/Mach-O file
evidence; host runtime remains unqualified. Linux ARM64/macOS release artifacts
were inspected, not executed. Publication/signing/notarization were not attempted.
Local tag inventories are empty and unchanged; the source introduces no publishing
or tag-writing path. Remote published asset bytes were not independently fetched.
Final directory promotion's documented crash partial-output limit is not a new
finding. No duplicate full suite or forced/skipped framework gate was run.

The canonical verdict will move only WP01 to planned with its feedback reference.
WP12 depends on WP01 and must refresh/rebase after remediation.
