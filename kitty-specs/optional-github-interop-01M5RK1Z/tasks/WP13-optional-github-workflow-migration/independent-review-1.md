# WP13 independent review

Verdict: **APPROVE**. No unresolved findings at source `1d70685b4073651d318288a5486ef0aa177a7a0e`.

Reviewer: `codex-independent-interop-review`, profile `reviewer-renata`, acting through assigned tool agent `codex`. This reviewer did not implement WP13. The reviewer resolved the Renata profile and review charter explicitly; the generated prompt's nominal implementer profile is not the reviewer identity. Canonical prompt invocation/reference: `7bd19ce08bfd491f9ac1d5cfc361cc85`; prompt `/tmp/spec-kitty-review-prompts/nichthub-2bfda21ece60/optional-github-interop-01M5RK1Z/WP13/7bd19ce08bfd491f9ac1d5cfc361cc85.md`.

## Scope and evidence method

Read the current primary WP13, specification, plan, full canonical review prompt, and all four owned files: `github_workflows.go`, `github_workflows_test.go`, `internal/githubapi/workflows.go`, and `internal/githubapi/workflows_test.go`. Inspected the narrow correction diff from `f145ae1ca1c47d6b3a0cd357b78474a1102ef777` to the approved source. Approved native workflow, transfer bridge, and client dependencies were treated as dependencies rather than reapproved here. There is no mission `contracts/` artifact.

The bounded audit used real signed native repositories, local HTTP fixtures, exact saved checkpoints and independent Go overlay probes. It made no production edits, live provider writes, public branch changes or status changes before this verdict. No full test suite was duplicated after the final root-run evidence was supplied. Runtime-generated untracked cache files are not production changes.

## Original findings, corrections and verification

All three P2 findings were reproduced against `f145ae1ca1c47d6b3a0cd357b78474a1102ef777` before corrections. They remain recorded here even though the current source passes.

| Finding | Original RED behavior | Correction in `1d70685b4073651d318288a5486ef0aa177a7a0e` | GREEN evidence |
|---|---|---|---|
| R1: mutable repository selector bypassed source protection | Import `a/b`, then export through repository alias `a/renamed` resolving the same stable workflow ID after its path moved. Targeting the original signed path containing human content returned no error and performed one PUT. Rename-only control correctly refused, isolating the moved-path branch. | Both original-path guards no longer depend on mutable `source.Project`. Stable provider workflow identity and the signed original path govern refusal; an uncertain source association cannot silently become copy mode. | `TestIndependentWorkflowMovedSourceRepoRenameCannotBecomeCopy` and rename-only control pass. Permanent `TestGitHubWorkflowReviewRegressionMovedSourceRepoRenameCannotBecomeCopy` and corresponding rename control invoke the full transfer path. |
| R2: runner label changed YAML type | Accepted explicit runner `true` emitted `runs-on: true`, a YAML boolean rather than the intended label string; the same class includes numeric/null labels. | Every accepted runner label is JSON-string encoded, producing a valid quoted YAML string. Exact renderer/manifest roundtrip remains enforced. | `TestIndependentWorkflowRunnerRemainsYAMLString` passes; permanent `TestGitHubWorkflowReviewRegressionRunnerRemainsYAMLString` checks exact quoting and generated roundtrip for reserved/numeric labels. |
| R3: newer projection blocked old exact recovery | A PUT succeeded but its response was lost. After a valid native edit removed the source document, retry computed the latest source projection first and left the old recoverable journal `in_flight` despite exact intended bytes being present remotely. No duplicate PUT occurred, but positive reconciliation was blocked. | Decode a saved checkpoint into zero state, preventing newer snapshot pointers/slices from contaminating absent older fields. Authenticate and reconcile the exact old intent before rendering newer content. A later projection error is allowed only after the old journal is confirmed. | `TestIndependentWorkflowRecoveryPrecedesNewInvalidExportProjection` passes; permanent `TestGitHubWorkflowReviewRegressionRecoveryPrecedesNewInvalidExportProjection` checks the newer projection error, confirmed old journal and exactly one PUT. |

Independent overlay evidence is retained outside the lane at `/home/lynn/Documents/Codex/2026-10-24/wa/work/interop-review/wp13_review_test.go` and `wp13-overlay.json`. Reproducer command:

```sh
go test -race -overlay /home/lynn/Documents/Codex/2026-10-24/wa/work/interop-review/wp13-overlay.json -run '^TestIndependentWorkflow' -count=1 .
```

Original RED output included `moved=true error=<nil> writes=1`, runner `"true"` emitted as a non-string scalar, and recovery `phase=in_flight writes=1`. On the corrected exact source all four independent probes passed in **1.534s** under race detection. The owned root workflow subset passed in **4.593s** and SDK workflow tests in **1.028s**. An earlier positive adversarial guard subset passed in **2.732s**, including exact CAS/destination/native selection validation, negative uncertainty without retries, clone publication proof, future result budget, and actual literal-argv script execution.

## Supported mappings and deliberate limits

Import discovers selected workflow metadata but reads definition bytes through Contents at a single resolved full source commit. Git tree traversal requires bounded regular directory/blob entries and rejects symlink traversal, unsupported paths, truncated directories and inconsistent blob identities. Exact UTF-8 bytes, newline forms and digest are retained as inert `github-actions-yaml` native source documents. Provider state is descriptive, not native execution authority.

Import mappings are `source-ref`, explicit `pipeline-file`, `native=generated`, and `enable=true|false`. A pipeline file is bounded, strictly validated native pipeline JSON and requires one selected workflow. Generated conversion accepts only the exact supported manifest/renderer output. Enabling requires an explicit native pipeline mapping. Arbitrary Actions YAML does not become runnable or materialize a native pipeline implicitly.

Export requires explicit `format=source|actions`, destination branch/path, and `expected-sha=<40 lowercase hex>|absent`. Source export restores one exact source document or requires an exact `source-digest` selection. Native Actions export additionally requires runner, `workflow_dispatch|push` trigger, `checkout=none|actions/checkout@<full SHA>`, and `runtime=python3-posix`. Commands, arguments and working directories are passed as base64 JSON to a fixed Python subprocess invocation, not interpolated shell or Actions expressions. Timeout translation is conservative and bounded; the generated job stays below the supported provider duration subset. Literal quotes, dollars, newlines and expression-looking arguments are exercised by executing the generated wrapper in a local fixture.

There is no general conversion of Actions expressions, matrices, services, arbitrary `uses`, secrets, runner isolation, policy or authority. Provider enabled/disabled state is not changed. Publishing a workflow file can activate provider triggers; dry-run/report identifies the selected branch/path and warns about that effect. Native signatures, roles and policy are not transferred as provider authority. When an original signed source path is selected but immutable source identity cannot safely be re-established, export refuses; an explicitly distinct destination remains the copy path.

Bounds include 1 MiB definition/mapping files, 256 workflows, at most 1000 entries in each traversed directory, existing 100-page/4 MiB response/64 MiB aggregate HTTP limits, the neutral 256-node/8 MiB graph limit, and an 8 MiB complete private checkpoint. The checkpoint budgets its future confirmed result before dispatch. Existing finite request and whole-transfer deadlines apply. Metadata never causes an implicit provider lookup or execution. Credentials remain in the explicitly selected client environment, not native state/checkpoints or diagnostics.

Recovery binds the explicit native ID, exact authenticated snapshot, destination, configuration, payload and blob guard. There is one external PUT per write callback. After uncertainty, only positive exact path/blob/content reconciliation can confirm the intent; a negative scan does not permit another PUT. Checkpoint edits cannot select another signed native workflow, redirect the destination/CAS or manufacture a differing result. Own exact publication proofs are checked against signed native facts, including a clone without export checkpoints. This is bounded content-CAS recovery, not a distributed exactly-once or branch-wide transaction guarantee.

## Requirements and anti-pattern checks

| Requirement | Behavior evidence |
|---|---|
| FR-004 | Native exact/inert import and pinned source fixtures preserve source IDs, URL/times, bytes and digest as attributed claims; no provider author is impersonated. |
| FR-008 | Optional explicit adapter registration, inherited sanitized/token-safe client, explicit PUT credentials, read-only dry-run and no live provider writes. |
| FR-011 | Optional `forge.WorkflowReader`, neutral graph/snapshot and exporter registry; no core dependency on GitHub DTOs or service. |
| FR-012 | Capability/plan reports distinguish exact preservation, explicit native mapping and unsupported execution semantics; mapping/preflight refusals are exercised. |
| FR-015 | Arbitrary YAML stays inert and materialize refuses; explicit validated pipeline/generated mapping is required before native enablement. |
| FR-019 | Source pin/blob/mode/path tests, exact bytes, branch/blob-CAS refusal, PUT recovery, constrained renderer, literal argv execution, generated roundtrip and source-copy conflicts. |
| FR-020 | Explicit mapping selection, full snapshot/destination reconstruction, future confirmed checkpoint bound and no-write preflight/unknown-outcome tests. |

| Canonical prompt check | Disposition |
|---|---|
| Dead code | PASS: targeted call-site search connects every exported SDK method to root import/export. Optional reader/exporter registrations are live production dispatch paths. |
| Synthetic fixture tests | PASS: HTTP/native fixtures execute production transfer, mutation, renderer and SDK paths; the RED cases fail when the corresponding guard is absent. |
| Silent empty returns | PASS: nil/success returns identify ordinary JSON scalar completion, valid validation, unrelated/native-echo nonmatches, or explicitly allowed distinct destination. Errors and missing selection/data fail loudly. |
| FR coverage | PASS: behavior assertions cover each requirement above. |
| Frozen surface | PASS: WP13 production changes are confined to its four owned files; no frozen signed facts, core authority or historical payloads are edited. |
| Locked decisions | PASS: optional stdlib transport, no `gh` subprocess/service/module requirement, no implicit YAML execution, role changes or timestamp conflict winner. |
| Shared-file ownership | PASS/N/A: no new shared-file overlap in WP13; approved dependency changes are not WP13 edits. |
| Production fragility | PASS: no panic/exception shortcut; hostile input, unsafe mapping, CAS conflict and uncertain outcome have explicit refusal/reconciliation behavior. |

## Final exact-source gates

Root-owned machine-readable evidence: `/home/lynn/Documents/Codex/2026-10-24/wa/work/wp13-1d70685b-gates.json`. Both entries pin the exact reviewed source and lane, with exit code zero. Logs were read during canonical review.

| Gate | Result |
|---|---|
| `go test -race -count=1 ./...` | PASS: root 287.316s, forge 1.021s, GitHub API 10.510s; wrapper 287.871s. |
| `go test -tags nogithub -count=1 ./...` | PASS: root 154.590s, forge 0.002s; wrapper 157.048s. |
| `go vet ./...` | PASS, author exact-source evidence. |
| Default and `nogithub` builds | PASS, author exact-source evidence. |
| Formatting and diff checks | PASS, author exact-source evidence; no production edits after the reviewed source. |
| Default pre-review gate | Completed without force/skip/bypass. Python `tests.architectural._gate_coverage` authority is unavailable; this is the known coverage/JUnit baseline artifact, not a Go test regression. Actual required Go gates passed. |

The downstream documentation/acceptance work remains responsible for integrating these precise capabilities, credentials, limitations and upgrade notes into final mission documentation. This review approves WP13 implementation against its scope; it does not approve a public merge or live publication.
