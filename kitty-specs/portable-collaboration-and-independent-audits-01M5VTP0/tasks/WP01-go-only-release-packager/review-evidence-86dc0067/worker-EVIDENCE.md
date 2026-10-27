# WP01 source-bound implementation evidence

Source: `86dc00678c42d495c6442addfcc8da8abd983ffa`
Workspace: `/home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-a`
Host exercised: Linux AMD64. Compiler: `go1.26.4`.

Product change: deterministic Go/Git-only `cmd/hn-release` and `internal/release`;
development version is still `0.0.1-dev`; release injection is explicit.
No external module, Python, shell helper, provider CLI or publication operation.

## Acceptance

1. `go build -trimpath -o /tmp/hn-wp01-evidence/hn-release ./cmd/hn-release`
2. `/tmp/hn-wp01-evidence/hn-release --source . --rev HEAD --version 0.0.2-dev --out /tmp/hn-wp01-evidence/first`
3. `/tmp/hn-wp01-evidence/hn-release --source . --rev 86dc0067 --version 0.0.2-dev --out /tmp/hn-wp01-evidence/second-different-path --targets linux/arm64,linux/amd64,darwin/arm64,darwin/amd64 --variants native-only,standard`
4. `go run /tmp/hn-wp01-evidence/inspect.go /tmp/hn-wp01-evidence /home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-a`

All four commands passed. The independent inspector imports no release-library
code and checked all 11 generated files byte-for-byte, exact Git content/modes
for all 591 source files, normalized archive paths/order/metadata, all 10 checksum
entries, all 9 archive digests, executable provenance, packaged docs and target
ELF/Mach-O architectures. Both Linux AMD64 variants executed `version`, GitHub
help, `init`, native issue open and native issue list in fresh temporary Git
repositories. Native-only excluded GitHub. Tags before/after were identical.
See `independent-inspection.log`, `package-first.log`, `package-second.log` and
`tags-before.txt`/`tags-after.txt`.

First/second identical manifest digests:

```text
0547f98293570c66b33cbe6fb234f661a80bf7c7ec043d30888194fbd88c2ade  SHA256SUMS
fc22cfc93d1dbd4d7b07032385ec1d2e67fc4b4d1b3f3684912319293ea64dfe  PROVENANCE.json
```

`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o /tmp/hn-wp01-evidence/hn-release-windows-amd64.exe ./cmd/hn-release` passed, and analogous
darwin/amd64 + darwin/arm64 commands passed. File inspection identified PE x86-64,
Mach-O x86-64 and Mach-O ARM64 respectively. These hosts were not runtime tested.
Linux ARM64 and macOS release binaries were cross-compiled and inspected, not
runtime qualified.

`go build -trimpath -tags nogithub -o /tmp/hn-wp01-evidence/hn-nogithub .` passed;
its `version` reported `0.0.1-dev`, and GitHub help reported exclusion.

## Development checks and baseline

The package/CLI tests were initially red for missing implementation. The exact
source/replace-ref adversarial reproduction was red before replacement-object
suppression, then green. Focused package/CLI tests pass, covering repeatability,
exact source, provenance, version injection/refusal, unsafe paths, collisions,
tracked dirt, failed compiles and cancellation.

At this head `go vet ./...`, `go build ./...`, formatting and `git diff --check`
passed (their logs are empty). Two tool-owned race attempts were externally
terminated with exit 143, without a Go failure/timeout trace. They are incomplete,
not passes: `race-interrupted.log`/`race-interrupted-status.txt` and
`race-retry.jsonl`/`race-retry-tool-status.txt` preserve the evidence. The second
had 847 passing events and no failing events before termination. Neither the
implementer nor root coordinator killed these runs.

The final unchanged command, `go test -race -count=1 -json ./...`, runs in a
detached supervisor (own process group, reparented to PID 1) to avoid the apparent
tool-session lifetime limit. The Go test timeout is unchanged. `race-detached.jsonl`
and `race-detached.exit` capture the real result; exact before/after source and
start/finish records are adjacent. Final completion/status is recorded separately
in the runtime handoff.

Final race result: PASS, detached Go exit 0. The root package completed in
488.265 seconds, with 1,173 passing events across the complete run and no failing
events. Before/after source both equal `86dc00678c42d495c6442addfcc8da8abd983ffa`.

The mission's initial baseline file is a framework coverage failure, not a Go
test failure: runner `pytest`, pseudotest `<gate-coverage-junit>`, error
`no JUnit XML artifact produced by the scoped run`, total 1 failed. No Go test
case is identified by that baseline. The pre-review gate is run without bypasses
and its completed result is recorded separately.

Final canonical transition: WP01 is `for_review`, event
`01M5VZ7AB3YRM3M5E6S4DX59GZ`; T001/T002/T003 were marked done through the CLI.
The mandatory pre-review gate completed without skip flags/environment. It
reported `no_coverage` because `tests.architectural._gate_coverage` was not
importable under `/home/lynn/projects/nichthub` (`No module named
'tests.architectural'`). Its default `block_enabled` was false; `blocked` and
`force_bypassed` were false. Thus the transition succeeded, but framework
coverage authority remains unverified. This is separate from the real Go
suite/static/build/acceptance passes above. See `pre-review-gate.log`.

The committed source diff also passed `git diff --check` against the mission
base; see `committed-diff-check.log`. The implementation worktree is clean.

## Remaining limits

No Windows/macOS packager host execution; no Linux ARM64/macOS artifact runtime
qualification; no signing/notarization/publication. Native-only uses both
`nogithub,noforge` until all-adapter exclusion is integrated by WP06. Existing
`v0.0.1-dev` publication assets and tags remain untouched. A crash during final
local directory promotion can leave a partial output directory; use a new output
path after inspection rather than overwriting it.
