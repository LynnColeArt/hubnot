# WP01 cycle 1 correction evidence

Product source: `3e6b301bb2e3142eff40727edf57a462b3e63e02`.
Previous product source: `86dc00678c42d495c6442addfcc8da8abd983ffa`.
Review/feedback preserved by commits `bb0822510cfeb87a2eb6616ad97d3515c93b7631`
and `e4417315b1961227101f8dd7f07141a7ae6737a6`; the original rejection remains
unchanged. Workspace: mission lane A under `/home/lynn/projects/nichthub/.worktrees`.
Actual host: Linux AMD64; Go 1.26.4; Git 2.43.0.

## P1 remediation

The public package and built CLI first failed the new regressions against the
previous product source: clean and process drivers in repository-local, included
and worktree configuration each created their marker and output. See
`filter-package-cli-red.log`. The harness configured helpers only after committing
clean fixture content and changed only its mtime before invocation.

Preflight now reads effective Git configuration without invoking helpers and
refuses clean/process filter configuration before Git status. It also refuses
partial-clone/promisor configuration (implicit object-fetch/transport helpers)
and indexed gitlinks before status could recurse into another repository's
configuration. The existing filesystem-monitor disable is tested with a marker.
The command retains actual Git status checks for tracked worktree and staged dirt.
No external dependency, product shell/Python helper or provider CLI was added.

The fixed tests exercise both `release.Package` and an actual built `hn-release`
process, checking error visibility, marker absence, absence of destination or
staging files, and identical source/Git metadata bytes before/after refusal.
Focused command: `go test -race -count=1 ./internal/release ./cmd/hn-release`.
Both packages passed (`focused-green.log`). The full suite below also reruns
these regressions against the committed fixed source.

## Packaging acceptance

Commands (all from the fixed source workspace):

```text
go build -trimpath -o /tmp/hn-wp01-fix-evidence/hn-release ./cmd/hn-release
/tmp/hn-wp01-fix-evidence/hn-release --source . --rev HEAD --version 0.0.2-dev --out /tmp/hn-wp01-fix-evidence/first
/tmp/hn-wp01-fix-evidence/hn-release --source . --rev 3e6b301b --version 0.0.2-dev --out /tmp/hn-wp01-fix-evidence/second-different-path --targets linux/arm64,linux/amd64,darwin/arm64,darwin/amd64 --variants native-only,standard
go run /tmp/hn-wp01-fix-evidence/inspect.go /tmp/hn-wp01-fix-evidence /home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-a
```

All passed. The inspector imports no release code. It compared all eleven assets,
checked all 591 committed source files/modes against Git, normalized archive
metadata/order, all ten checksums, all nine archive digests, executable provenance,
packaged documentation, and ELF/Mach-O target architecture. Linux AMD64 standard
and native-only artifacts actually executed version/help/init/issue-open/list;
native-only excluded GitHub. Tags before/after were identical. Inspector source
is preserved as `independent-inspector.go.txt`; logs and digests are adjacent.

The Windows AMD64 and macOS AMD64/ARM64 packagers compiled with commands of form:
`GOOS=TARGET GOARCH=ARCH CGO_ENABLED=0 go build -trimpath -o OUTPUT ./cmd/hn-release`.
`crosscompile.log` identifies the resulting PE/Mach-O architectures and
`digests.json` binds executable bytes. No Windows/macOS host execution was
available. Linux ARM64/macOS artifact runtime remains unqualified.

## Required source-bound validation

`static-checks.json` records exact commands, fixed source, actual exit 0 and empty
output for vet, build, formatting and committed mission-base diff checks.

The single full race run uses `go test -race -count=1 -json ./...`, with Go's
default timeout unchanged. A detached setsid supervisor avoids the known tool
session lifetime interruption; source before/after, PIDs, start/finish, raw JSON
and an atomically renamed exit file bind the actual result. The supervisor is
preserved as review-only evidence, not product release tooling. Raw JSON is
compressed without content changes; `race-summary.json` records its digest.

## Limits and review transition

Existing `v0.0.1-dev` assets, tags, development version, public main and other WPs
were not changed. Windows/macOS packager runtime is not qualified by cross-builds.
Publication/signing/notarization and non-Linux-AMD64 artifact execution remain
outside these checks. Local output promotion retains its documented crash limit.

The framework's earlier Python `tests.architectural` coverage-authority failure
is separate from actual Go results; the new mandatory pre-review gate is run
without skip/force or disable environment and recorded below after completion.

Full race result: PASS, exit 0; 1,186 passing events, zero failing events.
The root package completed in 464.783 seconds. Source before and after both
match `3e6b301bb2e3142eff40727edf57a462b3e63e02`. No duplicate full-race run
was started at this source. All product files remained unchanged during validation.

## Completed canonical transition

T001/T002/T003 were marked done with the canonical CLI. WP01 moved from
`in_progress` to `for_review`; event `01M5W12ZN7A36XC0VBSGGQ7C0W`.
The normal `move-task` command exited 0 and its mandatory pre-review gate
completed, without force/skip flags or disable environment. It returned
`no_coverage` because `tests.architectural._gate_coverage` is not importable
under the primary repository (`No module named tests.architectural`).
`blocked=false`, `force_bypassed=false`, `block_enabled=false`. This framework
coverage authority remains unavailable; this is not a Go test failure or a
claim of framework coverage. Raw result: `pre-review-gate.log`.

Earlier transition attempts safely refused unrelated WP02 untracked evidence,
then evidence committed on a lane, then a concurrently modified shared runtime
journal. No force or bypass was used. Review evidence was preserved on the
planning branch as `3e1f6a44`; the evidence-only lane commit was reverted as
`a382f928884fdedc0e89ee39f2eed61973d94115`. The lane's entire Git tree equals
the validated product source's tree, `2d88e80ba7db1fd22b3765fe1103bc160717a588`.
No product or test files changed after the completed validation, and no duplicate
full suite was run. Subsequent evidence/status records stay on the planning
branch, in accordance with the runtime guard. Attempt logs preserve these
workflow limitations separately from actual implementation checks.
