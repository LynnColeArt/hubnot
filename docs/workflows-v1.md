# Native workflows and explicit definition transfer

A Hubnot workflow is a signed versioned resource. It can contain a native
`hn.pipeline/0` definition, exact foreign source documents, or both. Source
preservation alone is inert. Opening, importing or revising a workflow never
materializes files, requests a run, or executes code.

Native authoring, reads, materialization and governed request commands remain
available offline in `nogithub` builds, without GitHub, `gh` or a service.
See [native resources](resources-v1.md), [CI and runner behavior](ci-v0.md), and
the [optional transfer guide](github-interoperability.md). New signed resource
kinds require upgraded alpha clients; existing signatures and policy authority
are unchanged.

## Author a native definition

Save this as `workflow.json`:

```json
{
  "schema":"hn.workflow/1",
  "name":"check",
  "title":"Project checks",
  "description":"Run the existing check tool",
  "enabled":true,
  "pipeline":{
    "version":"hn.pipeline/0",
    "steps":[{"name":"check","command":"go","args":["test","./..."],"timeoutSeconds":300}]
  },
  "metadata":{"team/context":"continuous integration"}
}
```

```sh
hn workflow open --input workflow.json --operation workflow-check --json
hn workflow revise WORKFLOW_ID --expect WORKFLOW_REV --input workflow.json --json
hn workflow resolve WORKFLOW_ID --expect HEAD_A --expect HEAD_B --input workflow.json --json
hn workflow list --limit 50 --json
hn workflow show WORKFLOW_ID --json
hn workflow heads WORKFLOW_ID --json
hn workflow history WORKFLOW_ID --limit 50 --json
hn workflow operation --actor ACTOR --operation workflow-check --json
```

Replace uppercase IDs with full `sha256:<64-hex>` values from JSON output.
Mutations return `resource_id`, `event_id`, `replayed`, `conflict` and `head_count`
in `data`. They accept `--actor ACTOR` to pin the selected signer and
`--operation KEY` for an actor-global immutable retry. A reused key with different
input fails. Full-state `--input -` reads stdin. Ordinary revise requires the one
current head; resolve requires every current head. Conflicts retain all heads and
expose `state:null`; timestamps do not select a winner.

Read selectors accept unambiguous prefixes of at least eight hex digits. Pages
use `--limit 1..200` and `--cursor TOKEN` bound to the accepted-ref snapshot.
`show` also reports `status`, `runnable`, `native_revision`, definition digest and
source-document count. Status is `inert-source`, `missing-native-definition`,
`disabled`, `native-runnable` or `conflicted`. Reads need no private identity.

Workflow state uses `schema`, `name`, `title`, `description`, `enabled`, optional
`pipeline`, `source_documents` and namespaced `metadata`. A source document is:

```json
{"format":"github-actions-yaml","content":"exact UTF-8 source bytes","digest":"sha256:<SHA-256-of-content-64-hex>"}
```

Compute the digest from the exact content bytes; the placeholder is not a valid
digest. A foreign-only workflow must have `enabled:false`. `enabled:true`
requires a valid native pipeline. Setting it true does not execute anything.

Names use 1..64 ASCII letters/digits/dot/underscore/hyphen, excluding `.` and `..`.
Title/description limits are 1024/65536 UTF-8 bytes. A pipeline is at most 1 MiB,
with 1..64 steps, each having a nonempty `name` and executable `command`, optional
`args`, safe relative `workingDirectory`, and `timeoutSeconds` from 0..3600
(0 uses the runner default). At most 256 arguments per step, each at most 16 KiB,
are accepted; null bytes and escaping command/directory paths fail. At most eight
source documents, each at most 1 MiB, are allowed; digests must be unique and
correct. The complete authenticated workflow event, including JSON escaping and
envelope, must fit 2 MiB, so individual limits do not guarantee a state fits.
Unknown JSON fields, duplicate keys, malformed Unicode and invalid metadata fail.

## Materialize, commit, then request

```sh
hn workflow materialize WORKFLOW_ID --expect WORKFLOW_REV --json
# Inspect .hn/pipelines/check.json, then explicitly add and commit it.
hn workflow request WORKFLOW_ID --proposal PROPOSAL_ID --expect WORKFLOW_REV --actor ACTOR --json
```

Materialize writes exact canonical compact pipeline JSON at
`.hn/pipelines/<name>.json` in the working tree. It refuses symlink paths and a
differing existing file unless `--overwrite` is explicit; identical bytes are a
no-op. It makes no commit, signed event, network request or execution. It needs
no native private key. Disabled, foreign-only and conflicted workflows refuse
materialization/request.

Create a proposal whose immutable head includes those exact bytes. Request checks
the selected workflow revision, proposal/code availability, dependency closure,
base policy and pipeline definition guards, then compares the proposal's pipeline
bytes and digest with the canonical signed definition. Equal parsed JSON with
different whitespace still fails that exact byte check. On success it appends an
ordinary `run.request` with the proposal head and pipeline digest. Output reports
the selected native workflow revision; this does not add a new signed authority
field or bypass existing runner policy/isolation. Request never executes the
pipeline. Use the existing explicitly invoked runner/run commands for execution.

Signed workflow state and its referenced histories reconstruct through native Git
replication without a provider checkpoint. Materialized working-tree files are
separate: explicitly commit them when a proposal must run that definition.

## Import GitHub definitions

GitHub remains an optional transport. `--project OWNER/REPO` selects a repository;
`--key` selects one positive numeric Actions workflow ID or its exact supported
`.github/workflows/NAME.yml` / `.yaml` path. Omit it for the bounded
workflow listing. The reader resolves one immutable source commit and reads exact
UTF-8 file bytes; it does not parse arbitrary YAML into executable native steps.

```sh
hn transfer capabilities --provider github --resource workflow
hn transfer import --provider github --resource workflow --project OWNER/REPO --key 123 --dry-run
hn transfer import --provider github --resource workflow --project OWNER/REPO --key 123
hn transfer import --provider github --resource workflow --project OWNER/REPO --key 123 --map pipeline-file=pipeline.json --map enable=true
```

| Import mapping | Meaning |
| --- | --- |
| No native mapping | Preserve exact `github-actions-yaml` bytes/digest and source commit/blob/path/state claims; disabled and inert |
| `source-ref=REF` | Explicit source ref to resolve and pin before content reads |
| `pipeline-file=FILE` | Explicit validated native `hn.pipeline/0` file, regular file at most 1 MiB; requires one `--key` selection and cannot combine with `native=` |
| `native=generated` | Decode only the adapter's own exact generated Actions document/manifest; changed or arbitrary YAML refuses |
| `enable=true` or `enable=false` | Native enabled state only; true requires an explicit pipeline mapping; default false |

Source provider enabled/disabled state remains a claim; it never enables native
execution. A native pipeline mapping is an operator decision, not proof that the
foreign workflow has equivalent behavior. Unsupported or unused mappings fail.
Exact publication of an already available signed native workflow refuses duplicate
native import/remapping.

## Export a definition explicitly

Restore one preserved source document:

```sh
hn transfer export WORKFLOW_ID --provider github --resource workflow --project OWNER/REPO --map format=source --map branch=main --map path=.github/workflows/check.yml --map expected-sha=absent --dry-run
hn transfer export WORKFLOW_ID --provider github --resource workflow --project OWNER/REPO --map format=source --map branch=main --map path=.github/workflows/check.yml --map expected-sha=REMOTE_BLOB_SHA
```

`expected-sha` is `absent` for a missing destination, or its exact lowercase
40-hex blob SHA. `format=source` needs exactly one matching
`github-actions-yaml` document; select `source-digest=sha256:...` when there are
multiple documents. It restores exact bytes and does not claim native execution
equivalence.

Render a native pipeline into the constrained Actions subset:

```sh
hn transfer export WORKFLOW_ID --provider github --resource workflow --project OWNER/REPO --map format=actions --map branch=main --map path=.github/workflows/check.yml --map expected-sha=absent --map runner=ubuntu-24.04 --map trigger=workflow_dispatch --map checkout=none --map runtime=python3-posix --dry-run
```

Inspect the plan, then repeat without `--dry-run` to publish. All destination
mappings are required: safe branch, direct `.github/workflows/NAME.yml` or `.yaml`
path, and expected blob state. Native Actions rendering additionally requires a
runner label, `trigger=workflow_dispatch|push`, `runtime=python3-posix`, and
`checkout=none` or `actions/checkout@<full-40-hex-commit-SHA>`. Runtime and checkout
are explicit environment choices. The renderer transports argv as base64 JSON
and uses Python subprocess calls; it does not recreate arbitrary Actions YAML.
The conservative summed step timeout budget must be at most 350 minutes; output
must fit 1 MiB. Publishing a workflow file may activate its provider triggers,
including source-restored triggers. Dry-run reports this boundary.

| Feature | Boundary |
| --- | --- |
| Exact preserved source YAML | Supported as inert native data and exact explicit branch/path restore |
| Native pipeline to Actions | Explicit constrained renderer and exact generated-manifest roundtrip only |
| Arbitrary YAML, matrices, services, expressions, arbitrary `uses`, secrets | No general native conversion or semantic equivalence |
| Provider enable/disable, roles, policies, runner trust/isolation | Not transferred as authority or equivalent configuration |
| Native materialize/request | Separate local actions; provider import/export never invokes them |

All GitHub writes require `HN_GITHUB_TOKEN`; REST public reads can be anonymous
when repository permissions allow. Workflow export itself uses exact signed
native snapshot verification and a guarded contents write; it does not require a
new native signing event. Source/native author claims cannot impersonate the
provider writer.

Each export journals one exact contents mutation, preflighting the whole future
prepared/in-flight/confirmed checkpoint within 8 MiB. It checks the destination
blob SHA before writing; this is not a lock against all branch changes. An
uncertain PUT is not retried automatically: retry the same command for exact
positive content/blob reconciliation. A negative read, changed file content,
changed destination, or different returned identity requires explicit recovery.
Unrelated branch changes are not locked by this contents check. Complete an old
pending snapshot before considering newer native edits. After a
completed export changes natively, start a fresh explicit transfer using the new
remote `expected-sha`. A single publisher and manual conflict reconciliation
remain necessary; there is no global exactly-once guarantee.
