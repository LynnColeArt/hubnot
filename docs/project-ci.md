# Hubnot builds itself with Hubnot CI

Project development uses the native signed proposal → run request → runner →
result path. GitHub is an optional portability and package-smoke executor.
Ordinary users do not enable a runner, install Go, or configure any of this.

## Feedback tiers

| Tier | Execution | Budget and purpose |
| --- | --- | --- |
| Fast regressions | Native `test` pipeline; same action on GitHub PRs | 5-minute native step; 10-minute hosted job cap. Internal unit tests, selected core regressions, native CI protocol, vet. |
| Migration regressions | Explicit native `migration` pipeline; same action on GitHub PRs | Complete platform-specific selection in three balanced shards, each retaining the 2-minute race-test cap; 10-minute native step and hosted job caps. |
| Package smoke | GitHub PR install/runtime; explicit full-platform smoke or native Mac certification | PRs check both native variants within 2 minutes. Explicit extension qualification adds 5 focused journeys per variant with 3-minute invocation caps and a 15-minute native job cap. |
| Integration | Explicit native `integration` pipeline | 10-minute step cap. Full native collaboration, artifact recovery and three supplied reference-consumer journeys, using source-assisted binaries. |
| Stress | Explicit native `stress` pipeline | Separate bounded steps for large record/ref/byte limits, lifecycle faults and observation capacity. Run when these mechanisms change, or during a deliberate deep audit. |
| Real-clock soak | Explicit native `soak` pipeline | The unchanged ten-minute expiry test, separately requested. |

The full 206-journey registry, 100000/+1 boundaries, race assertions and real
expiry lifetime remain available. Hosted smoke has a distinct `smoke` group
with an explicit excluded inventory; it is not an exhaustive qualification
result. Existing timeout/cancelled runs remain recorded as such. No production
limit is reduced to make a check faster.

The old five-hour hosted campaign is superseded by these tiers. Mac package
certification checks the exact downloaded binaries on each claimed architecture;
it does not require repeating every OS-independent stress corpus on every Mac.
Known functional or data-integrity failures still need resolution before release.

For the published beta.2 bytes, use the [release qualification kit and recorded
results](releases/0.1.0-beta.2.md), rather than building another candidate under
a test label. Its three native supported targets passed the bounded non-race
runner. Intel Macs are outside that prerelease's support scope even though an
explicit development workflow may still include an Intel runner.

For an explicitly requested new hosted extension-smoke campaign, select
`qualification=optional-extensions` and provide the exact committed
`extension_source`. The legacy default `qualification=beta` retains the older
beta.1 candidate campaign and its build/reuse inputs; it is not the published
beta.2 artifact or the ordinary PR smoke route. Build labels and synthetic PR
merge checks do not certify released binaries.

PR package checks use `TestBetaPlatformPackageInstallAndRuntime`: exact executable
provenance, PATH installation, memory regex and literal-temp migration round trips
for both variants. The longer full collaboration journey remains unchanged in
the explicit native integration action and the full Go suite; it is not a PR
smoke result. Historical timed-out broad invocations remain incomplete.

## Native requests and signed results

The tracked definitions are `.hn/pipelines/test.json`, `migration.json`, `integration.json`, `stress.json`,
`stress-artifacts.json` (a focused stress request), and `soak.json`. They invoke
the tracked `.hn/actions/ci` entry point. For an existing signed proposal:

```sh
hn run request FULL_PROPOSAL_ID test
hn run execute FULL_REQUEST_ID
hn run show FULL_REQUEST_ID
hn run logs FULL_RESULT_ID

# Separate, explicit requests when deeper evidence is needed:
hn run request FULL_PROPOSAL_ID migration
hn run request FULL_PROPOSAL_ID integration
hn run request FULL_PROPOSAL_ID stress
hn run request FULL_PROPOSAL_ID soak
```

Use full signed `sha256:` event IDs for proposal/request inputs; abbreviated
display IDs are insufficient for requesting or executing a run. `run show`
inspects a request; `run logs` reads a result. Inspect the result's
outcome and exit code; successful publication of a failed result is not a test
pass. Results bind the exact proposed commit and pipeline digest and retain
signed attribution, backend/platform, duration and a bounded log. Existing
trusted-runner policy continues to decide which results satisfy acceptance.
This change does not edit `.hn/policy.json` or grant an agent a new role.

Linux execution defaults to Bubblewrap. On a trusted local Mac, the current
host backend requires both flags on the invocation:

```sh
hn run execute FULL_REQUEST_ID --backend host --allow-unsafe-host-execution
```

CI workers need Go and Git installed. The project action recognizes the official
read-only `/usr/local/go/bin` location when it is absent from the sandbox's fixed
PATH. This is CI setup, not an application runtime requirement; no host PATH,
private Git directory or signing key is mounted into the sandbox.

An operator can explicitly run `hn runner once|watch --accept-pipeline NAME
--accept-actor FULL_FINGERPRINT` to discover signed requests. Nothing starts
automatically and there is no GitHub account, webhook, service or database
requirement. See [runner behavior and boundaries](ci-v0.md).

## Source snapshot versus release artifacts

The native executor extracts the requested source snapshot, without exposing
the operator's Git directory or keys. Stress actions build that snapshot's CLI
once and supply it to the existing tests. Integration also builds the three
reference consumers from that snapshot. Those are **source-assisted** results;
their logs name the request commit and executable hash. They do not certify a
downloaded release archive. Package smoke checks transported checksums, build
metadata, native architecture, both variants and supplied reference executables.

Whole-history benchmarks and historical fixture captures need their separately
provisioned immutable inputs. They remain explicit local qualification tools;
the native runner does not yet provision those inputs or persist arbitrary large
artifacts. The signed run log is capped at 1 MiB. This revision does not claim
a new scheduler, artifact store, Mac sandbox or automatic issue creation.
Authorized deferred audits and explicit finding follow-up remain available via
[native audits](audits-v1.md).

## Local development

```sh
./.hn/actions/ci fast
./.hn/actions/ci migration
./.hn/actions/ci integration
./.hn/actions/ci stress-artifacts
./.hn/actions/ci stress-counts
./.hn/actions/ci stress-lifecycle
./.hn/actions/ci stress-observation
./.hn/actions/ci soak-expiry
```

The fast manifest fails if a selected test disappears. The heavier commands
remain explicit and retain their original fixture sizes; `go test ./...` is
still a full local suite, not the fast feedback command.
